package events

import "testing"

func TestPlainText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain text unchanged", "Bowls provided.", "Bowls provided."},
		{"tags removed", "<b>Bring</b> flat <i>shoes</i>", "Bring flat shoes"},
		{"line breaks kept", "Line one<br>Line two<br/>Line three", "Line one\nLine two\nLine three"},
		{"paragraphs become lines", "<p>First</p><p>Second</p>", "First\nSecond"},
		{"links keep their address", `Book <a href="https://example.org/book">here</a>.`, "Book here (https://example.org/book)."},
		{"bare link not repeated", `<a href="https://example.org">https://example.org</a>`, "https://example.org"},
		{"entities decoded", "Fish &amp; chips", "Fish & chips"},
		{"scripts dropped", "Hi<script>alert(1)</script>", "Hi"},
		{"surrounding space trimmed", "  <p> Hello </p>  ", "Hello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := plainText(tt.in); got != tt.want {
				t.Errorf("plainText(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
