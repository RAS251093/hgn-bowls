package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/ras251093/hgn-bowls/api/db"
)

// hygraphPost is a post as the old site's Hygraph schema holds it. The
// old site showed the first asset as the cover, with the first alt text.
type hygraphPost struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Slug        string `json:"slug"`
	PubDate     string `json:"pubDate"`
	Body        struct {
		Markdown string `json:"markdown"`
	} `json:"body"`
	AltText []string `json:"altText"`
	Assets  []struct {
		ID       string `json:"id"`
		FileName string `json:"fileName"`
		URL      string `json:"url"`
	} `json:"assets"`
}

// Hygraph returns 10 entries unless asked for more; the club has far fewer than 1000.
const postsQuery = `{
  posts(first: 1000, orderBy: pubDate_ASC) {
    title
    description
    slug
    pubDate
    body { markdown }
    altText
    assets { id fileName url }
  }
}`

// fetchPosts reads every published post from Hygraph's content API. token
// may be empty when the API allows public reads.
func fetchPosts(ctx context.Context, client *http.Client, endpoint, token string) ([]hygraphPost, error) {
	body, err := json.Marshal(map[string]string{"query": postsQuery})
	if err != nil {
		return nil, err
	}
	// The endpoint is the operator's own setting, not user input.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body)) //nolint:gosec // G704
	if err != nil {
		return nil, fmt.Errorf("fetch posts: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := client.Do(req) //nolint:gosec // G704, as above
	if err != nil {
		return nil, fmt.Errorf("fetch posts: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return nil, fmt.Errorf("fetch posts: Hygraph answered %s: %s", res.Status, bytes.TrimSpace(msg))
	}
	var out struct {
		Data struct {
			Posts []hygraphPost `json:"posts"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("fetch posts: decode: %w", err)
	}
	if len(out.Errors) > 0 {
		msgs := make([]string, len(out.Errors))
		for i, e := range out.Errors {
			msgs[i] = e.Message
		}
		return nil, fmt.Errorf("fetch posts: Hygraph: %s", strings.Join(msgs, "; "))
	}
	return out.Data.Posts, nil
}

// store is where imported posts go (*db.Queries in production).
type store interface {
	SlugTaken(ctx context.Context, slug sql.NullString) (bool, error)
	ImportPost(ctx context.Context, arg db.ImportPostParams) (int64, error)
}

// bucket is where cover images go (Cloud Storage, or a folder for a trial run).
type bucket interface {
	Put(ctx context.Context, key, contentType string, data []byte) error
}

type importer struct {
	store  store
	images bucket
	client *http.Client
	now    func() time.Time
	newID  func() string
	dryRun bool
	log    io.Writer
}

type report struct {
	imported, skipped, warnings []string
}

const (
	importedBy = "import:hygraph"
	// Same layout as the posts package, so imported times sort with the rest.
	timeLayout = "2006-01-02T15:04:05.000000Z07:00"
	maxImage   = 25 << 20
)

var (
	slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	// Markdown images whose address is on Hygraph's CDN, which goes when Hygraph does.
	oldImage   = regexp.MustCompile(`!\[[^\]]*\]\((https?://[^)\s]+)`)
	extensions = map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp"}
	london     = mustLoadLocation("Europe/London")
)

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

// run imports posts in order, skipping any whose slug is already taken, so
// it can be run again after a failure. It stops at the first bad post.
func (im *importer) run(ctx context.Context, posts []hygraphPost) (report, error) {
	var rep report
	for _, p := range posts {
		taken, err := im.store.SlugTaken(ctx, sql.NullString{String: p.Slug, Valid: true})
		if err != nil {
			return rep, fmt.Errorf("%s: check slug: %w", p.Slug, err)
		}
		if taken {
			fmt.Fprintf(im.log, "skip %s: already imported\n", p.Slug)
			rep.skipped = append(rep.skipped, p.Slug)
			continue
		}
		warnings, err := im.importPost(ctx, p)
		for _, w := range warnings {
			rep.warnings = append(rep.warnings, p.Slug+": "+w)
		}
		if err != nil {
			return rep, fmt.Errorf("%s: %w", p.Slug, err)
		}
		rep.imported = append(rep.imported, p.Slug)
	}
	return rep, nil
}

func (im *importer) importPost(ctx context.Context, p hygraphPost) (warnings []string, err error) {
	if !slugPattern.MatchString(p.Slug) {
		return nil, fmt.Errorf("slug %q can't be used in an address", p.Slug)
	}
	title := strings.TrimSpace(p.Title)
	if title == "" {
		return nil, errors.New("no title")
	}
	published, err := parsePubDate(p.PubDate)
	if err != nil {
		return nil, err
	}
	if extra := len(p.Assets) - 1; extra > 0 {
		warnings = append(warnings, fmt.Sprintf("%d more image the old site never showed; not imported", extra))
	}
	for _, m := range oldImage.FindAllStringSubmatch(p.Body.Markdown, -1) {
		warnings = append(warnings, "body links to an image on the old site: "+m[1])
	}

	id := im.newID()
	var coverKey, coverAlt sql.NullString
	if len(p.Assets) > 0 {
		if im.dryRun {
			coverKey = sql.NullString{String: "posts/" + id, Valid: true}
		} else {
			data, contentType, err := im.download(ctx, p.Assets[0].URL)
			if err != nil {
				return warnings, fmt.Errorf("cover: %w", err)
			}
			key := "posts/" + id + "." + extensions[contentType]
			if err := im.images.Put(ctx, key, contentType, data); err != nil {
				return warnings, fmt.Errorf("cover: upload: %w", err)
			}
			coverKey = sql.NullString{String: key, Valid: true}
		}
		alt := ""
		if len(p.AltText) > 0 {
			alt = strings.TrimSpace(p.AltText[0])
		}
		if alt == "" {
			warnings = append(warnings, "cover has no alt text; add one in the editor")
		}
		coverAlt = sql.NullString{String: alt, Valid: true}
	}

	if im.dryRun {
		fmt.Fprintf(im.log, "would import %s (%s), published %s, cover %t\n",
			p.Slug, title, published, coverKey.Valid)
		return warnings, nil
	}
	n, err := im.store.ImportPost(ctx, db.ImportPostParams{
		ID: id, Slug: sql.NullString{String: p.Slug, Valid: true},
		Title: title, Description: strings.TrimSpace(p.Description), BodyMd: p.Body.Markdown,
		CoverImageKey: coverKey, CoverAlt: coverAlt,
		PublishedAt: sql.NullString{String: published, Valid: true},
		Now:         im.now().UTC().Format(timeLayout), UserID: importedBy,
	})
	if err != nil {
		return warnings, fmt.Errorf("save: %w", err)
	}
	if n == 0 {
		return warnings, errors.New("save: slug was taken while importing")
	}
	fmt.Fprintf(im.log, "imported %s (%s)\n", p.Slug, title)
	return warnings, nil
}

// parsePubDate reads Hygraph's Date ("2025-04-05", taken as midnight UK
// time) or DateTime value.
func parsePubDate(s string) (string, error) {
	if t, err := time.ParseInLocation(time.DateOnly, s, london); err == nil {
		return t.UTC().Format(timeLayout), nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC().Format(timeLayout), nil
	}
	return "", fmt.Errorf("pubDate %q is not a date", s)
}

// download fetches an image, trusting its bytes rather than its headers.
func (im *importer) download(ctx context.Context, url string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	res, err := im.client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("GET %s: %s", url, res.Status)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, maxImage+1))
	if err != nil {
		return nil, "", fmt.Errorf("GET %s: %w", url, err)
	}
	if len(data) > maxImage {
		return nil, "", fmt.Errorf("GET %s: larger than %d MB", url, maxImage>>20)
	}
	contentType := http.DetectContentType(data)
	if _, ok := extensions[contentType]; !ok {
		return nil, "", fmt.Errorf("GET %s: %s is not a JPEG, PNG or WebP image", url, contentType)
	}
	return data, contentType, nil
}
