package storage

import (
	"net/url"
	"strings"
	"testing"
)

func TestSanitizeFilename(t *testing.T) {
	cases := map[string]struct{ in, want string }{
		"vietnamese kept":     {"Biên bản họp tổ dân phố.pdf", "Biên bản họp tổ dân phố.pdf"},
		"unix path":           {"../../etc/passwd", "passwd"},
		"windows path":        {`C:\Users\a\Desktop\scan.pdf`, "scan.pdf"},
		"crlf injection":      {"a.pdf\r\nSet-Cookie: x=1", "a.pdfSet-Cookie x=1"},
		"quotes":              {`say "hi".pdf`, "say hi.pdf"},
		"semicolon":           {`a.pdf; filename="evil.exe"`, "a.pdf filename=evil.exe"},
		"nul and del":         {"a\x00b\x7fc.pdf", "abc.pdf"},
		"bidi override":       {"invoice\u202Efdp.exe", "invoicefdp.exe"},
		"leading dots":        {"...hidden.pdf", "hidden.pdf"},
		"whitespace collapse": {"  a \t\t b  .pdf ", "a b .pdf"},
		"only junk":           {"\r\n\"\\/", ""},
		"invalid utf8":        {"a\xffb.pdf", "ab.pdf"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := SanitizeFilename(c.in); got != c.want {
				t.Fatalf("SanitizeFilename(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestSanitizeFilenameCapsLength(t *testing.T) {
	got := SanitizeFilename(strings.Repeat("ệ", 400) + ".pdf")
	if n := len([]rune(got)); n > maxFilenameRunes {
		t.Fatalf("length %d > %d", n, maxFilenameRunes)
	}
}

func TestContentDisposition(t *testing.T) {
	got := ContentDisposition("Đơn đề nghị \"gấp\"\r\n.pdf", MIMEPDF)
	if strings.ContainsAny(got, "\r\n") {
		t.Fatalf("header injection: %q", got)
	}
	if !strings.HasPrefix(got, `attachment; filename="`) {
		t.Errorf("PDF must be an attachment: %q", got)
	}
	// ASCII fallback: quoted, non-ASCII replaced, no stray quote inside.
	fallback := strings.SplitN(got, `filename="`, 2)[1]
	fallback = fallback[:strings.Index(fallback, `"`)]
	if strings.Count(got, `"`) != 2 {
		t.Errorf("exactly one quoted string expected: %q", got)
	}
	if strings.ContainsFunc(fallback, func(r rune) bool { return r > 0x7E }) {
		t.Errorf("fallback not ASCII: %q", fallback)
	}
	// RFC 5987 part decodes back to the sanitised name.
	enc := strings.SplitN(got, "filename*=UTF-8''", 2)
	if len(enc) != 2 {
		t.Fatalf("no filename* parameter: %q", got)
	}
	dec, err := url.PathUnescape(enc[1])
	if err != nil {
		t.Fatalf("filename* not percent-encoded: %v", err)
	}
	if dec != "Đơn đề nghị gấp.pdf" {
		t.Errorf("filename* decodes to %q", dec)
	}
	for i := 0; i < len(enc[1]); i++ {
		if c := enc[1][i]; c != '%' && !isAttrChar(c) {
			t.Errorf("filename* has an unencoded byte %q: %q", c, enc[1])
		}
	}
}

func TestContentDispositionMediaInline(t *testing.T) {
	for _, mime := range []string{MIMEJPEG, MIMEPNG, MIMEWebP, MIMEHEIC, MIMEMP4, MIMEQuickTime} {
		if got := ContentDisposition("a", mime); !strings.HasPrefix(got, "inline;") {
			t.Errorf("%s: %q", mime, got)
		}
	}
}

func TestContentDispositionEmptyFallsBack(t *testing.T) {
	got := ContentDisposition("\r\n", MIMEPDF)
	if !strings.Contains(got, `filename="download.pdf"`) {
		t.Fatalf("got %q", got)
	}
}
