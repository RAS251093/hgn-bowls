// Package posts manages the club's news posts: drafting, editing,
// publishing and the list the site build reads.
package posts

import (
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/ras251093/hgn-bowls/api/db"
)

var (
	// ErrNotFound means there is no live post with that ID (deleted posts count as missing).
	ErrNotFound = errors.New("post not found")
	// ErrConflict means someone else saved the post since the caller loaded it.
	ErrConflict = errors.New("post was changed by someone else")
)

// ValidationError lists what is wrong with a post, keyed by JSON field name.
type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string {
	var parts []string
	for _, k := range slices.Sorted(maps.Keys(e.Fields)) {
		parts = append(parts, k+": "+e.Fields[k])
	}
	return "invalid post: " + strings.Join(parts, "; ")
}

const (
	maxTitle       = 120
	maxDescription = 300
	maxAlt         = 300
	maxBody        = 100_000
)

// Post is a post as the admin editor sees it.
type Post struct {
	ID            string  `json:"id"`
	Slug          *string `json:"slug"`
	Title         string  `json:"title"`
	Description   string  `json:"description"`
	BodyMd        string  `json:"bodyMd"`
	CoverImageKey *string `json:"coverImageKey"`
	CoverAlt      *string `json:"coverAlt"`
	Status        string  `json:"status"`
	PublishedAt   *string `json:"publishedAt"`
	CreatedAt     string  `json:"createdAt"`
	UpdatedAt     string  `json:"updatedAt"`
	CreatedBy     string  `json:"createdBy"`
	UpdatedBy     string  `json:"updatedBy"`
}

// Draft is what an admin sends when creating or saving a post.
type Draft struct {
	Title         string  `json:"title"`
	Description   string  `json:"description"`
	BodyMd        string  `json:"bodyMd"`
	CoverImageKey *string `json:"coverImageKey"`
	CoverAlt      *string `json:"coverAlt"`
	// PublishedAt changes the shown publish date. Only used when saving;
	// leaving it out keeps the current date.
	PublishedAt *string `json:"publishedAt"`
}

// Validate checks a draft and tidies its whitespace.
func (d *Draft) Validate() error {
	fields := map[string]string{}
	d.Title = strings.TrimSpace(d.Title)
	d.Description = strings.TrimSpace(d.Description)
	d.CoverImageKey = trimmedOrNil(d.CoverImageKey)
	d.CoverAlt = trimmedOrNil(d.CoverAlt)
	d.PublishedAt = trimmedOrNil(d.PublishedAt)

	switch n := utf8.RuneCountInString(d.Title); {
	case n == 0:
		fields["title"] = "Add a title"
	case n > maxTitle:
		fields["title"] = fmt.Sprintf("Keep the title to %d characters or fewer", maxTitle)
	}
	if utf8.RuneCountInString(d.Description) > maxDescription {
		fields["description"] = fmt.Sprintf("Keep the description to %d characters or fewer", maxDescription)
	}
	if utf8.RuneCountInString(d.BodyMd) > maxBody {
		fields["bodyMd"] = "The post is too long"
	}
	if d.CoverImageKey != nil {
		key := *d.CoverImageKey
		if !strings.HasPrefix(key, "posts/") || strings.Contains(key, "..") {
			fields["coverImageKey"] = "Upload the cover image again"
		}
		if d.CoverAlt == nil {
			fields["coverAlt"] = "Describe the cover image for people who can't see it"
		}
	}
	if d.CoverAlt != nil && utf8.RuneCountInString(*d.CoverAlt) > maxAlt {
		fields["coverAlt"] = fmt.Sprintf("Keep the description to %d characters or fewer", maxAlt)
	}
	if d.CoverImageKey == nil {
		d.CoverAlt = nil
	}
	if d.PublishedAt != nil {
		t, err := time.Parse(time.RFC3339, *d.PublishedAt)
		if err != nil {
			fields["publishedAt"] = "Pick a valid date and time"
		} else {
			s := formatTime(t)
			d.PublishedAt = &s
		}
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

// timeLayout is fixed width, so stored times sort as text and two saves in
// the same second still get different versions.
const timeLayout = "2006-01-02T15:04:05.000000Z07:00"

func formatTime(t time.Time) string { return t.UTC().Format(timeLayout) }

func trimmedOrNil(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return &t
}

func fromRow(p db.Post) Post {
	return Post{
		ID:            p.ID,
		Slug:          ptr(p.Slug),
		Title:         p.Title,
		Description:   p.Description,
		BodyMd:        p.BodyMd,
		CoverImageKey: ptr(p.CoverImageKey),
		CoverAlt:      ptr(p.CoverAlt),
		Status:        p.Status,
		PublishedAt:   ptr(p.PublishedAt),
		CreatedAt:     p.CreatedAt,
		UpdatedAt:     p.UpdatedAt,
		CreatedBy:     p.CreatedBy,
		UpdatedBy:     p.UpdatedBy,
	}
}

func ptr(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	return &s.String
}

func null(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

// slugify turns a title into a URL path segment: lower case ASCII letters
// and digits separated by single hyphens, at most 80 characters.
func slugify(title string) string {
	var b strings.Builder
	hyphen := false
	for _, r := range norm.NFKD.String(strings.ToLower(title)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			hyphen = false
		case unicode.Is(unicode.Mn, r): // accents left over from NFKD
		case r == '\'' || r == '’':
		default:
			if b.Len() > 0 && !hyphen {
				b.WriteByte('-')
				hyphen = true
			}
		}
	}
	s := strings.TrimRight(b.String(), "-")
	if len(s) > 80 {
		s = strings.TrimRight(s[:80], "-")
	}
	if s == "" {
		return "post"
	}
	return s
}
