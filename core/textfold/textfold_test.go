package textfold

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestFoldFindsTheUnaccentedForm(t *testing.T) {
	for in, want := range map[string]string{
		"Nhà văn hoá":           "nha van hoa",
		"NHÀ VĂN HOÁ":           "nha van hoa",
		"Đường Đỗ Quang Đẩu":    "duong do quang dau",
		"Ðường":                 "duong", // U+00D0, the legacy look-alike of Đ
		"Thuỷ lợi, Thủy lợi":    "thuy loi, thuy loi",
		"Nguyễn Thị Ánh Tuyết":  "nguyen thi anh tuyet",
		"nha van hoa":           "nha van hoa",
		"NV19 — 50% (đã xử lý)": "nv19 — 50% (da xu ly)",
		"Nhà văn hoá":        "nha van hoa", // decomposed input
		"Ở ư̛":                 "o u",
	} {
		if got := Fold(in); got != want {
			t.Errorf("Fold(%q) = %q, want %q", in, got, want)
		}
	}
}

// Every letter of the table folds to ASCII, in both cases — a missing capital would leave "Ầ" unfound
// on a cluster whose lower() does not know it.
func TestEveryVietnameseLetterFoldsToASCII(t *testing.T) {
	for _, g := range bases {
		for _, r := range g.marked {
			got := Fold(string(r))
			if len(got) != 1 || got[0] >= utf8.RuneSelf {
				t.Errorf("Fold(%q) = %q, want one ASCII letter", r, got)
			}
		}
	}
	// 66 marked vowels (a 17 · e 11 · i 5 · o 17 · u 11 · y 5) + đ, in both cases, + Ð.
	if n := utf8.RuneCountInString(fromLetters); n != 2*67+1 || n != utf8.RuneCountInString(toLetters) {
		t.Errorf("translate() arguments: %d and %d letters", n, utf8.RuneCountInString(toLetters))
	}
}

// The SQL text is pinned by migrations; this states its shape so an accidental edit is visible here
// first. No invisible character: the marks travel as escapes.
func TestSQLShape(t *testing.T) {
	s := SQL("tieu_de")
	if !strings.HasPrefix(s, "lower(translate(translate(tieu_de, U&'\\0300\\0301\\0302\\0303\\0306\\0309\\031B\\0323', ''), '") {
		t.Errorf("SQL = %s", s)
	}
	for _, m := range marks {
		if strings.ContainsRune(s, m) {
			t.Errorf("SQL carries the raw combining mark U+%04X", m)
		}
	}
	if SQL("$3") == s || !strings.Contains(SQL("$3"), "translate(translate($3,") {
		t.Errorf("SQL($3) = %s", SQL("$3"))
	}
}
