package backup

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/ras251093/hgn-bowls/api/db"
)

func openDB(t *testing.T) *db.Queries {
	t.Helper()
	return db.New(openConn(t))
}

func openConn(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := db.Open("file:"+filepath.Join(t.TempDir(), "test.db"), "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(t.Context(), conn); err != nil {
		t.Fatal(err)
	}
	return conn
}

func str(s string) sql.NullString { return sql.NullString{String: s, Valid: true} }

// A published post with a cover, a draft, and a deleted post.
func seed(t *testing.T, q *db.Queries) {
	t.Helper()
	rows := []db.RestorePostRowParams{
		{
			ID: "01A", Slug: str("opening-day"), Title: "Opening day", Description: "The green opens.",
			BodyMd: "We **opened**.", CoverImageKey: str("posts/a.jpg"), CoverAlt: str("The green"),
			Status: "published", PublishedAt: str("2025-04-05T09:00:00.000000Z"),
			CreatedAt: "2025-04-01T09:00:00.000000Z", UpdatedAt: "2025-04-05T09:00:00.000000Z",
			CreatedBy: "auth0|rob", UpdatedBy: "auth0|rob",
		},
		{
			ID: "01B", Title: "AGM notice (draft)", Status: "draft",
			CreatedAt: "2026-10-01T09:00:00.000000Z", UpdatedAt: "2026-10-02T09:00:00.000000Z",
			CreatedBy: "auth0|rob", UpdatedBy: "auth0|ann",
		},
		{
			ID: "01C", Slug: str("old-news"), Title: "Old news", Status: "draft",
			PublishedAt: str("2024-01-01T09:00:00.000000Z"),
			CreatedAt:   "2024-01-01T08:00:00.000000Z", UpdatedAt: "2026-10-03T09:00:00.000000Z",
			CreatedBy: "import:hygraph", UpdatedBy: "auth0|rob", DeletedAt: str("2026-10-03T09:00:00.000000Z"),
		},
	}
	for _, r := range rows {
		if err := q.RestorePostRow(t.Context(), r); err != nil {
			t.Fatal(err)
		}
	}
}

var now = time.Date(2026, 11, 1, 3, 17, 0, 0, time.UTC)

func TestExportWritesEveryPostAsJSON(t *testing.T) {
	q := openDB(t)
	seed(t, q)
	var buf bytes.Buffer
	if err := Export(t.Context(), q, &buf, now); err != nil {
		t.Fatal(err)
	}

	var got File
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("export isn't valid JSON: %v\n%s", err, buf.String())
	}
	if got.Version != 1 || got.ExportedAt != "2026-11-01T03:17:00Z" {
		t.Errorf("header = version %d, exported %q", got.Version, got.ExportedAt)
	}
	ids := make([]string, len(got.Posts))
	for i, p := range got.Posts {
		ids[i] = p.ID
	}
	// Drafts and deleted posts too: a backup is for getting everything back.
	if diff := cmp.Diff([]string{"01C", "01A", "01B"}, ids); diff != "" {
		t.Errorf("posts, oldest first (-want +got):\n%s", diff)
	}
	opening := got.Posts[1]
	if opening.Slug == nil || *opening.Slug != "opening-day" || opening.CoverAlt == nil || *opening.CoverAlt != "The green" {
		t.Errorf("published post = %+v", opening)
	}
	if got.Posts[0].DeletedAt == nil {
		t.Error("the deleted post lost its deleted_at")
	}
	// Plain values, not sql.NullString's {"String":…,"Valid":…}.
	if strings.Contains(buf.String(), `"Valid"`) {
		t.Errorf("export leaks sql.NullString:\n%s", buf.String())
	}
}

func TestRestoreRebuildsTheSameRowsInAFreshDatabase(t *testing.T) {
	from := openDB(t)
	seed(t, from)
	var buf bytes.Buffer
	if err := Export(t.Context(), from, &buf, now); err != nil {
		t.Fatal(err)
	}

	conn := openConn(t)
	to := db.New(conn)
	n, err := Restore(t.Context(), conn, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("restored %d posts, want 3", n)
	}
	want, _ := from.AllPosts(t.Context())
	got, _ := to.AllPosts(t.Context())
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("restored rows differ (-original +restored):\n%s", diff)
	}
}

func TestRestoreRefusesADatabaseWithPosts(t *testing.T) {
	conn := openConn(t)
	q := db.New(conn)
	seed(t, q)
	var buf bytes.Buffer
	if err := Export(t.Context(), q, &buf, now); err != nil {
		t.Fatal(err)
	}
	if _, err := Restore(t.Context(), conn, &buf); err == nil {
		t.Fatal("restored over existing posts; want an error")
	}
}

func TestRestoreRefusesFilesItDoesntUnderstand(t *testing.T) {
	for name, body := range map[string]string{
		"not JSON":        "posts,title\n",
		"another version": `{"version":2,"posts":[]}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Restore(t.Context(), openConn(t), strings.NewReader(body)); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestObjectNameSortsByTime(t *testing.T) {
	if got := ObjectName(now); got != "posts/2026-11-01T031700Z.json" {
		t.Errorf("ObjectName = %q", got)
	}
}
