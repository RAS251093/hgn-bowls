// Command import-hygraph copies the old site's posts from Hygraph into the
// posts table, keeping each post's slug and date so old /blog/ links still
// find it, and uploads each cover image to Cloud Storage. Posts whose slug
// is already in the table are skipped, so it is safe to run again.
//
//	HYGRAPH_ENDPOINT=… TURSO_URL=… TURSO_TOKEN=… \
//	  go run ./cmd/import-hygraph -images gs://hgn-bowls-images [-dry-run]
//
// For a trial, point TURSO_URL at a file (file:trial.db) and -images at a folder.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/oklog/ulid/v2"

	"github.com/ras251093/hgn-bowls/api/db"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "import-hygraph:", err)
		os.Exit(1)
	}
}

func run() error {
	images := flag.String("images", "", "where cover images go: gs://bucket, or a local folder")
	dryRun := flag.Bool("dry-run", false, "read Hygraph and report what would happen, writing nothing")
	flag.Parse()

	endpoint, tursoURL := os.Getenv("HYGRAPH_ENDPOINT"), os.Getenv("TURSO_URL")
	if endpoint == "" || tursoURL == "" || *images == "" {
		return errors.New("need HYGRAPH_ENDPOINT, TURSO_URL and -images")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	conn, err := db.Open(tursoURL, os.Getenv("TURSO_TOKEN"))
	if err != nil {
		return err
	}
	defer conn.Close()
	if strings.HasPrefix(tursoURL, "file:") {
		// A trial database starts empty.
		if err := db.Migrate(ctx, conn); err != nil {
			return err
		}
	}

	var dest bucket
	if name, ok := strings.CutPrefix(*images, "gs://"); ok {
		// Uses your gcloud application default credentials.
		gcs, err := storage.NewClient(ctx)
		if err != nil {
			return fmt.Errorf("storage client: %w", err)
		}
		defer gcs.Close()
		dest = gcsBucket{gcs.Bucket(name)}
	} else {
		dest = dirBucket(*images)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	posts, err := fetchPosts(ctx, client, endpoint, os.Getenv("HYGRAPH_TOKEN"))
	if err != nil {
		return err
	}
	fmt.Printf("Hygraph has %d posts\n", len(posts))

	im := &importer{
		store:  db.New(conn),
		images: dest,
		client: client,
		now:    time.Now,
		newID:  func() string { return ulid.Make().String() },
		dryRun: *dryRun,
		log:    os.Stdout,
	}
	rep, runErr := im.run(ctx, posts)
	for _, w := range rep.warnings {
		fmt.Println("warning:", w)
	}
	verb := "imported"
	if *dryRun {
		verb = "would import"
	}
	fmt.Printf("%s %d, skipped %d\n", verb, len(rep.imported), len(rep.skipped))
	if runErr == nil && !*dryRun && len(rep.imported) > 0 {
		fmt.Println("Rebuild the site to show them: retry the latest Pages deployment, or publish from admin.")
	}
	return runErr
}

// gcsBucket uploads to Cloud Storage.
type gcsBucket struct{ h *storage.BucketHandle }

func (b gcsBucket) Put(ctx context.Context, key, contentType string, data []byte) error {
	w := b.h.Object(key).If(storage.Conditions{DoesNotExist: true}).NewWriter(ctx)
	w.ContentType = contentType
	if _, err := w.Write(data); err != nil {
		_ = w.Close()
		return err
	}
	return w.Close()
}

// dirBucket writes to a local folder, for trial runs.
type dirBucket string

func (d dirBucket) Put(_ context.Context, key, _ string, data []byte) error {
	path := filepath.Join(string(d), filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
