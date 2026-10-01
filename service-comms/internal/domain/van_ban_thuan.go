package domain

import (
	"html"
	"strings"
	"unicode"
)

// VanBanThuanChoDan turns a stored article field into PLAIN TEXT for the public Mini App read
// (GET /api/v1/commune-news): the title, the summary, the event place, category names, and the `body`
// field older app builds read.
//
// THE 27/09/2026 DECISION THIS COMMENT USED TO STATE ("no HTML ever reaches the citizen") WAS REPLACED
// ON 01/10/2026 BY ADR 0067 §1: the body now reaches residents WITH FORMATTING — but as STRUCTURE
// (`body_blocks`, built by internal/richtext from the allow-list-sanitised HTML), never as markup. What
// still holds from 27/09: no field of the public answer is HTML, and no client is ever asked to render
// markup safely — a client asked that is one update away from innerHTML. This function keeps serving
// every plain-text field, `body` included (ADR 0067 §1 decision 4), unchanged.
//
// This is a STRIPPER, not a sanitiser: nothing it returns is meant to be interpreted as HTML at all, so
// it does not have to decide which markup is safe — it keeps none. The sanitiser is internal/richtext.
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
func VanBanThuanChoDan(s string) string {
	t := boThe(s)
	t = html.UnescapeString(t)
	t = boThe(t)
	return chuanHoaKhoangTrang(t)
}

// theBoCaNoiDung are the elements whose CONTENT is not prose: printing it would print code, styles or
// a document the article embeds. Lower case; the match is case-insensitive.
var theBoCaNoiDung = map[string]bool{
	"script": true, "style": true, "noscript": true, "template": true, "iframe": true,
	"object": true, "embed": true, "svg": true, "math": true, "head": true, "title": true,
	"textarea": true, "select": true, "xmp": true, "noembed": true, "noframes": true, "canvas": true,
}

// theKhoi are the block-level elements that end a paragraph. Everything else is inline and simply
// disappears, so `<b>xã</b>` reads `xã`.
var theKhoi = map[string]bool{
	"p": true, "div": true, "li": true, "ul": true, "ol": true, "h1": true, "h2": true, "h3": true,
	"h4": true, "h5": true, "h6": true, "tr": true, "table": true, "blockquote": true, "section": true,
	"article": true, "header": true, "footer": true, "hr": true, "pre": true, "dd": true, "dt": true,
	"dl": true, "figure": true, "figcaption": true, "address": true, "main": true, "nav": true,
	"aside": true,
}

// boThe removes tags from s. See VanBanThuanChoDan for the policy.
func boThe(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		c := s[i]
		if c != '<' || !batDauThe(s, i) {
			b.WriteByte(c)
			i++
			continue
		}
		// An HTML comment runs to `-->`, whatever it contains.
		if strings.HasPrefix(s[i:], "<!--") {
			het := strings.Index(s[i+4:], "-->")
			if het < 0 {
				return b.String()
			}
			i += 4 + het + 3
			continue
		}
		ten, dong := tenThe(s, i)
		ket := cuoiThe(s, i)
		if ket < 0 {
			return b.String() // unterminated tag: drop the rest
		}
		i = ket + 1
		switch {
		case !dong && theBoCaNoiDung[ten] && s[ket-1] != '/':
			// Skip to the end of the matching closing tag.
			j := timTheDong(s, i, ten)
			if j < 0 {
				return b.String() // unterminated element: drop the rest
			}
			i = j
		case ten == "br":
			b.WriteByte('\n')
		case theKhoi[ten]:
			b.WriteString("\n\n")
		case ten == "td" || ten == "th":
			b.WriteByte(' ')
		}
	}
	return b.String()
}

// batDauThe reports whether the `<` at i starts something tag-shaped: `<x`, `</`, `<!`, `<?`.
func batDauThe(s string, i int) bool {
	if i+1 >= len(s) {
		return false
	}
	n := s[i+1]
	return n == '/' || n == '!' || n == '?' || (n >= 'a' && n <= 'z') || (n >= 'A' && n <= 'Z')
}

// tenThe reads the lower-cased element name after `<` or `</`, and whether it is a closing tag.
func tenThe(s string, i int) (string, bool) {
	j := i + 1
	dong := false
	if j < len(s) && s[j] == '/' {
		dong = true
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
	return strings.ToLower(s[j:k]), dong
}

// cuoiThe finds the `>` that closes the tag starting at i, skipping `>` inside quoted attribute
// values. -1 when there is none.
func cuoiThe(s string, i int) int {
	var nhay byte
	for k := i + 1; k < len(s); k++ {
		c := s[k]
		switch {
		case nhay != 0:
			if c == nhay {
				nhay = 0
			}
		case c == '"' || c == '\'':
			nhay = c
		case c == '>':
			return k
		}
	}
	return -1
}

// timTheDong finds the position just after the `>` of the first `</ten` at or after i,
// case-insensitively. -1 when there is none.
func timTheDong(s string, i int, ten string) int {
	thap := strings.ToLower(s[i:])
	mau := "</" + ten
	for tu := 0; ; {
		k := strings.Index(thap[tu:], mau)
		if k < 0 {
			return -1
		}
		k += tu
		sau := k + len(mau)
		// `</scriptx` is not `</script`: the name must end here.
		if sau < len(thap) {
			if c := thap[sau]; (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
				tu = sau
				continue
			}
		}
		ket := cuoiThe(s, i+k)
		if ket < 0 {
			return -1
		}
		return ket + 1
	}
}

// chuanHoaKhoangTrang is step 4 of VanBanThuanChoDan.
func chuanHoaKhoangTrang(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	var dong []string
	for _, l := range strings.Split(s, "\n") {
		var b strings.Builder
		khoang := false
		for _, r := range l {
			switch {
			case r == '\t' || r == ' ' || unicode.IsSpace(r):
				khoang = true
			case unicode.IsControl(r):
				// dropped: corrupts a screen and a log line alike
			default:
				if khoang && b.Len() > 0 {
					b.WriteByte(' ')
				}
				khoang = false
				b.WriteRune(r)
			}
		}
		dong = append(dong, b.String())
	}

	// One blank line between paragraphs, whatever produced it; none at either end.
	var ra []string
	trong := 0
	for _, l := range dong {
		if l == "" {
			trong++
			continue
		}
		if len(ra) > 0 {
			if trong > 0 {
				ra = append(ra, "")
			}
		}
		trong = 0
		ra = append(ra, l)
	}
	return strings.Join(ra, "\n")
}
