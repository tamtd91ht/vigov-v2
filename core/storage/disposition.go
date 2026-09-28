package storage

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// maxFilenameRunes bounds the name put into Content-Disposition. Browsers and file systems cap
// names near 255 bytes; 150 runes of Vietnamese stays under that after UTF-8 encoding in most
// cases, and a longer name is a pasted paragraph, not a file name.
const maxFilenameRunes = 150

// ContentDisposition builds a Content-Disposition value for a download: `inline` for media,
// `attachment` for everything else (ADR 0052 §4), with the user-supplied file name sanitised
// and encoded per RFC 6266 / RFC 5987 — an ASCII `filename="…"` fallback plus the exact name,
// percent-encoded, in the `filename*` parameter with the UTF-8 charset.
//
// The name is USER INPUT and may carry personal data (rule 3): it is never logged, and a
// header-injection attempt (CR/LF), a quote, or a path is neutralised here rather than trusted
// to the store. An empty result falls back to `download.<ext>`.
func ContentDisposition(filename, mime string) string {
	disposition := "attachment"
	if isMedia(mime) {
		disposition = "inline"
	}
	name := SanitizeFilename(filename)
	if name == "" {
		name = "download"
		if ext, ok := extByMIME[mime]; ok {
			name += "." + ext
		}
	}
	return disposition + `; filename="` + asciiFallback(name) + `"; filename*=UTF-8''` + rfc5987(name)
}

// SanitizeFilename keeps the last path segment of a user-supplied name, drops control and
// format characters, quotes and backslashes, collapses whitespace, trims dots and spaces from
// both ends, and caps the length. It returns "" when nothing usable is left.
func SanitizeFilename(s string) string {
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, "")
	}
	// Last segment of either separator: "../../etc/passwd" and "C:\x\y.pdf" both lose their path.
	if i := strings.LastIndexAny(s, `/\`); i >= 0 {
		s = s[i+1:]
	}
	var b strings.Builder
	space := false
	for _, r := range s {
		switch {
		case unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == utf8.RuneError:
			continue // CR, LF, NUL, DEL, bidi overrides (a "gpj.exe" trick) and friends
		case r == '"' || r == '\\' || r == ';' || r == ':' || r == '*' || r == '?' || r == '<' || r == '>' || r == '|':
			continue
		case unicode.IsSpace(r):
			if !space {
				b.WriteByte(' ')
			}
			space = true
			continue
		}
		space = false
		b.WriteRune(r)
	}
	out := strings.Trim(b.String(), " .")
	if utf8.RuneCountInString(out) > maxFilenameRunes {
		out = strings.TrimRight(string([]rune(out)[:maxFilenameRunes]), " .")
	}
	return out
}

// asciiFallback replaces everything outside printable ASCII — and `%`, which some browsers
// percent-decode inside the quoted form — with `_`. Quotes and backslashes are already gone.
func asciiFallback(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r < 0x20 || r > 0x7E || r == '%' || r == '"' || r == '\\' {
			b.WriteByte('_')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// rfc5987 percent-encodes every byte that is not an RFC 5987 attr-char.
func rfc5987(s string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if isAttrChar(c) {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(hex[c>>4])
		b.WriteByte(hex[c&0x0F])
	}
	return b.String()
}

// isAttrChar is RFC 5987's attr-char: ALPHA / DIGIT / "!" / "#" / "$" / "&" / "+" / "-" / "."
// / "^" / "_" / "`" / "|" / "~".
func isAttrChar(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	}
	return strings.IndexByte("!#$&+-.^_`|~", c) >= 0
}
