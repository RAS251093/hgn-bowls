package events

import (
	"strings"

	"golang.org/x/net/html"
)

// plainText turns a Google Calendar description, which may contain HTML,
// into plain text. Line breaks are kept and links keep their address.
func plainText(s string) string {
	z := html.NewTokenizer(strings.NewReader(s))
	var b strings.Builder
	var href, linkText string
	inLink, skip := false, 0

	for {
		switch z.Next() {
		case html.ErrorToken:
			return tidy(b.String())

		case html.TextToken:
			if skip > 0 {
				continue
			}
			text := string(z.Text())
			if inLink {
				linkText += text
			}
			b.WriteString(text)

		case html.StartTagToken, html.SelfClosingTagToken:
			name, hasAttr := z.TagName()
			switch string(name) {
			case "script", "style":
				skip++
			case "br":
				b.WriteString("\n")
			case "p", "div", "li":
				b.WriteString("\n")
			case "a":
				inLink, href, linkText = true, "", ""
				for hasAttr {
					var key, val []byte
					key, val, hasAttr = z.TagAttr()
					if string(key) == "href" {
						href = string(val)
					}
				}
			}

		case html.EndTagToken:
			name, _ := z.TagName()
			switch string(name) {
			case "script", "style":
				skip = max(skip-1, 0)
			case "p", "div", "li":
				b.WriteString("\n")
			case "a":
				if href != "" && strings.TrimSpace(linkText) != href {
					b.WriteString(" (" + href + ")")
				}
				inLink = false
			}
		}
	}
}

// tidy trims each line and collapses blank lines left behind by block tags.
func tidy(s string) string {
	var lines []string
	for line := range strings.SplitSeq(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}
