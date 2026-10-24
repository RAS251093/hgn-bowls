package posts

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func str(s string) *string { return &s }

func TestDraftValidate(t *testing.T) {
	tests := []struct {
		name       string
		draft      Draft
		wantFields []string // empty means valid
	}{
		{name: "minimal", draft: Draft{Title: "Open day"}},
		{name: "with cover", draft: Draft{Title: "t", CoverImageKey: str("posts/2026/10/abc.jpg"), CoverAlt: str("Bowlers on the green")}},
		{name: "missing title", draft: Draft{Title: "   "}, wantFields: []string{"title"}},
		{name: "title at limit", draft: Draft{Title: strings.Repeat("é", 120)}},
		{name: "title too long", draft: Draft{Title: strings.Repeat("a", 121)}, wantFields: []string{"title"}},
		{name: "description too long", draft: Draft{Title: "t", Description: strings.Repeat("a", 301)}, wantFields: []string{"description"}},
		{name: "body too long", draft: Draft{Title: "t", BodyMd: strings.Repeat("a", 100_001)}, wantFields: []string{"bodyMd"}},
		{name: "cover without alt", draft: Draft{Title: "t", CoverImageKey: str("posts/a.jpg"), CoverAlt: str(" ")}, wantFields: []string{"coverAlt"}},
		{name: "cover outside posts/", draft: Draft{Title: "t", CoverImageKey: str("secret/a.jpg"), CoverAlt: str("a")}, wantFields: []string{"coverImageKey"}},
		{name: "cover path traversal", draft: Draft{Title: "t", CoverImageKey: str("posts/../secret.jpg"), CoverAlt: str("a")}, wantFields: []string{"coverImageKey"}},
		{name: "bad publish date", draft: Draft{Title: "t", PublishedAt: str("next tuesday")}, wantFields: []string{"publishedAt"}},
		{name: "everything wrong", draft: Draft{CoverImageKey: str("x"), PublishedAt: str("x")}, wantFields: []string{"coverAlt", "coverImageKey", "publishedAt", "title"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.draft.Validate()
			if len(tt.wantFields) == 0 {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			var invalid *ValidationError
			if !errors.As(err, &invalid) {
				t.Fatalf("err = %v, want a *ValidationError", err)
			}
			var got []string
			for k := range invalid.Fields {
				got = append(got, k)
			}
			if diff := cmp.Diff(tt.wantFields, got, sortStrings); diff != "" {
				t.Errorf("fields (-want +got):\n%s", diff)
			}
		})
	}
}

var sortStrings = cmpopts.SortSlices(func(a, b string) bool { return a < b })

func TestDraftValidateTidies(t *testing.T) {
	d := Draft{
		Title: "  Open day  ", Description: " d ", CoverImageKey: nil, CoverAlt: str("orphan alt"),
		PublishedAt: str("2026-05-01T10:00:00+01:00"),
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	want := Draft{Title: "Open day", Description: "d", PublishedAt: str("2026-05-01T09:00:00.000000Z")}
	if diff := cmp.Diff(want, d); diff != "" {
		t.Errorf("tidied draft (-want +got):\n%s", diff)
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct{ title, want string }{
		{"Open Day 2026", "open-day-2026"},
		{"  Summer -- League!  ", "summer-league"},
		{"Café crème", "cafe-creme"},
		{"Members' night", "members-night"},
		{"Members’ night", "members-night"},
		{"!!!", "post"},
		{"日本語", "post"},
		{strings.Repeat("ab ", 50), strings.TrimRight(strings.Repeat("ab-", 27), "-")},
	}
	for _, tt := range tests {
		if got := slugify(tt.title); got != tt.want {
			t.Errorf("slugify(%q) = %q, want %q", tt.title, got, tt.want)
		}
	}
}
