package domain

import (
	"html"
	"strings"
	"unicode"
)

// PlainTextForCitizen turns a stored article field into PLAIN TEXT for the public Mini App read
// (GET /api/v1/commune-news, owner decision 2026-09-27: "no HTML ever reaches the citizen").
//
// WHY THE SERVER AND NOT THE MINI APP. The body is stored AS GIVEN (see NormalizeLongText — §8 says
// HTML, and a member of staff with content.update can put script into it). A client asked to "render it
// safely" is one client update away from innerHTML; a server that never sends markup cannot be undone
// by any client. This is a STRIPPER, not a sanitiser: nothing it returns is meant to be interpreted as
// HTML at all, so it does not have to decide which markup is safe — it keeps none.
//
// WHAT IT DOES, in order:
//
//  1. drops every tag, and drops the WHOLE CONTENT of elements whose content is not prose (script,
//     style, iframe, svg, …); a block-level tag becomes a paragraph break, `<br>` a line break;
//  2. decodes character references (`&amp;` → `&`), so the citizen reads text, not entities;
//  3. drops tags AGAIN. Markup that arrived entity-encoded (`&lt;script&gt;`) is markup after step 2,
//     and a Mini App that ever did render as HTML would run it. Fail closed: a tag-shaped sequence
//     never survives, whichever spelling it came in. A lone `<` that starts no tag ("giá < 5 triệu")
//     is text and is kept;
//  4. normalises whitespace: runs of spaces collapse, lines are trimmed, paragraphs are separated by
//     exactly one blank line ("\n\n"), and control characters other than the newline are removed.
//
// AN UNTERMINATED TAG OR ELEMENT DROPS EVERYTHING AFTER IT. `<script>alert(1)` with no `</script>` is
// not an invitation to guess where the script ends; losing the tail of a malformed article is the
// fail-closed side, and the staff screen still shows the stored original.
func PlainTextForCitizen(s string) string {
	t := stripTags(s)
	t = html.UnescapeString(t)
	t = stripTags(t)
	return normalizeWhitespace(t)
}

// dropContentTags are the elements whose CONTENT is not prose: printing it would print code, styles or
// a document the article embeds. Lower case; the match is case-insensitive.
var dropContentTags = map[string]bool{
	"script": true, "style": true, "noscript": true, "template": true, "iframe": true,
	"object": true, "embed": true, "svg": true, "math": true, "head": true, "title": true,
	"textarea": true, "select": true, "xmp": true, "noembed": true, "noframes": true, "canvas": true,
}

// blockTags are the block-level elements that end a paragraph. Everything else is inline and simply
// disappears, so `<b>xã</b>` reads `xã`.
var blockTags = map[string]bool{
	"p": true, "div": true, "li": true, "ul": true, "ol": true, "h1": true, "h2": true, "h3": true,
	"h4": true, "h5": true, "h6": true, "tr": true, "table": true, "blockquote": true, "section": true,
	"article": true, "header": true, "footer": true, "hr": true, "pre": true, "dd": true, "dt": true,
	"dl": true, "figure": true, "figcaption": true, "address": true, "main": true, "nav": true,
	"aside": true,
}

// stripTags removes tags from s. See PlainTextForCitizen for the policy.
func stripTags(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		c := s[i]
		if c != '<' || !startsTag(s, i) {
			b.WriteByte(c)
			i++
			continue
		}
		// An HTML comment runs to `-->`, whatever it contains.
		if strings.HasPrefix(s[i:], "<!--") {
			end := strings.Index(s[i+4:], "-->")
			if end < 0 {
				return b.String()
			}
			i += 4 + end + 3
			continue
		}
		name, closing := tagName(s, i)
		end := tagEnd(s, i)
		if end < 0 {
			return b.String() // unterminated tag: drop the rest
		}
		i = end + 1
		switch {
		case !closing && dropContentTags[name] && s[end-1] != '/':
			// Skip to the end of the matching closing tag.
			j := findClosingTag(s, i, name)
			if j < 0 {
				return b.String() // unterminated element: drop the rest
			}
			i = j
		case name == "br":
			b.WriteByte('\n')
		case blockTags[name]:
			b.WriteString("\n\n")
		case name == "td" || name == "th":
			b.WriteByte(' ')
		}
	}
	return b.String()
}

// startsTag reports whether the `<` at i starts something tag-shaped: `<x`, `</`, `<!`, `<?`.
func startsTag(s string, i int) bool {
	if i+1 >= len(s) {
		return false
	}
	n := s[i+1]
	return n == '/' || n == '!' || n == '?' || (n >= 'a' && n <= 'z') || (n >= 'A' && n <= 'Z')
}

// tagName reads the lower-cased element name after `<` or `</`, and whether it is a closing tag.
func tagName(s string, i int) (string, bool) {
	j := i + 1
	closing := false
	if j < len(s) && s[j] == '/' {
		closing = true
		j++
	}
	k := j
	for k < len(s) {
		c := s[k]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			k++
			continue
		}
		break
	}
	return strings.ToLower(s[j:k]), closing
}

// tagEnd finds the `>` that closes the tag starting at i, skipping `>` inside quoted attribute
// values. -1 when there is none.
func tagEnd(s string, i int) int {
	var quote byte
	for k := i + 1; k < len(s); k++ {
		c := s[k]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '>':
			return k
		}
	}
	return -1
}

// findClosingTag finds the position just after the `>` of the first `</name` at or after i,
// case-insensitively. -1 when there is none.
func findClosingTag(s string, i int, name string) int {
	lower := strings.ToLower(s[i:])
	pattern := "</" + name
	for from := 0; ; {
		k := strings.Index(lower[from:], pattern)
		if k < 0 {
			return -1
		}
		k += from
		after := k + len(pattern)
		// `</scriptx` is not `</script`: the name must end here.
		if after < len(lower) {
			if c := lower[after]; (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
				from = after
				continue
			}
		}
		end := tagEnd(s, i+k)
		if end < 0 {
			return -1
		}
		return end + 1
	}
}

// normalizeWhitespace is step 4 of PlainTextForCitizen.
func normalizeWhitespace(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	var lines []string
	for _, l := range strings.Split(s, "\n") {
		var b strings.Builder
		space := false
		for _, r := range l {
			switch {
			case r == '\t' || r == ' ' || unicode.IsSpace(r):
				space = true
			case unicode.IsControl(r):
				// dropped: corrupts a screen and a log line alike
			default:
				if space && b.Len() > 0 {
					b.WriteByte(' ')
				}
				space = false
				b.WriteRune(r)
			}
		}
		lines = append(lines, b.String())
	}

	// One blank line between paragraphs, whatever produced it; none at either end.
	var out []string
	blank := 0
	for _, l := range lines {
		if l == "" {
			blank++
			continue
		}
		if len(out) > 0 {
			if blank > 0 {
				out = append(out, "")
			}
		}
		blank = 0
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}
