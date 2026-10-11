// Package textfold is the ONE definition of accent-insensitive Vietnamese search: "nha van hoa" finds
// "Nhà văn hoá", and so does "NHÀ VĂN HOÁ".
//
// # WHY IT IS IN core AND NOT IN A SERVICE
//
// A search box that folds "đ" to "d" in one register and not in the next is a search the officer
// cannot trust in either. Three services already carry a private copy of the diacritic table for slugs
// (service-documents' vietnameseBase says so); search is the case where the copies must AGREE, so the
// table lives here once. The slug copies are left alone — moving them is its own change.
//
// # WHY THERE IS AN SQL HALF, AND WHY `unaccent` IS NOT USED
//
// PostgreSQL's `unaccent` is an extension, and installing one is an infrastructure decision (ADR 0010).
// SQL builds the same fold from `lower()` and `translate()`, both IMMUTABLE, so it can back a STORED
// GENERATED column. SQL renders that expression FROM THE TABLE BELOW, so the Go and the SQL fold cannot
// drift apart: a migration that pins the expression is checked against SQL by a test in that service.
//
// # WHICH SIDE FOLDS THE SEARCH TERM
//
// THE DATABASE, with the very same expression — never Go. `lower()` folds non-ASCII letters only under
// a Unicode-aware LC_CTYPE; under `C` it folds ASCII alone. Folding the column in SQL and the term in
// Go would agree on Vietnamese and disagree on any other non-ASCII capital, silently, depending on how
// the cluster was initialised. Folding both through one SQL expression agrees by construction.
//
// # WHAT IT DOES, IN ORDER
//
//  1. Deletes the eight COMBINING marks Vietnamese decomposed input uses (Unikey's "Unicode tổ hợp",
//     macOS file names): grave, acute, circumflex, tilde, breve, hook above, horn, dot below. Without
//     this, "a" + U+0300 never matches "à".
//  2. Maps every precomposed toned or hatted vowel, BOTH CASES, and đ/Đ to its base letter. Both cases,
//     so the fold does not depend on `lower()` knowing Vietnamese capitals.
//  3. Lower-cases.
package textfold

import "strings"

// Marks are the combining diacritics Vietnamese text uses, as SQL Unicode escapes and as runes.
// U+0300 grave · U+0301 acute · U+0302 circumflex · U+0303 tilde · U+0306 breve · U+0309 hook above ·
// U+031B horn · U+0323 dot below.
var marks = []rune{0x0300, 0x0301, 0x0302, 0x0303, 0x0306, 0x0309, 0x031B, 0x0323}

// bases is the precomposed table: each base letter and every letter that folds onto it.
//
// `Ð` (U+00D0) is listed with `Đ` (U+0110) on purpose: the two are indistinguishable on screen and
// legacy Vietnamese fonts typed the first for the second, so a register imported from old files holds
// both.
var bases = []struct {
	base   rune
	marked string
}{
	{'a', "àáảãạăằắẳẵặâầấẩẫậ"},
	{'A', "ÀÁẢÃẠĂẰẮẲẴẶÂẦẤẨẪẬ"},
	{'e', "èéẻẽẹêềếểễệ"},
	{'E', "ÈÉẺẼẸÊỀẾỂỄỆ"},
	{'i', "ìíỉĩị"},
	{'I', "ÌÍỈĨỊ"},
	{'o', "òóỏõọôồốổỗộơờớởỡợ"},
	{'O', "ÒÓỎÕỌÔỒỐỔỖỘƠỜỚỞỠỢ"},
	{'u', "ùúủũụưừứửữự"},
	{'U', "ÙÚỦŨỤƯỪỨỬỮỰ"},
	{'y', "ỳýỷỹỵ"},
	{'Y', "ỲÝỶỸỴ"},
	{'d', "đ"},
	{'D', "ĐÐ"},
}

var (
	// fromLetters / toLetters are translate()'s two arguments, position for position.
	fromLetters, toLetters = func() (string, string) {
		var from, to strings.Builder
		for _, g := range bases {
			for _, r := range g.marked {
				from.WriteRune(r)
				to.WriteRune(g.base)
			}
		}
		return from.String(), to.String()
	}()

	// replacer is the Go fold's steps 1 and 2 in one pass.
	replacer = func() *strings.Replacer {
		var pairs []string
		for _, m := range marks {
			pairs = append(pairs, string(m), "")
		}
		for _, g := range bases {
			for _, r := range g.marked {
				pairs = append(pairs, string(r), string(g.base))
			}
		}
		return strings.NewReplacer(pairs...)
	}()
)

// Fold is the Go fold: marks deleted, letters mapped to their base, lower-cased.
//
// IT AGREES WITH SQL ON VIETNAMESE AND ASCII TEXT, which is what a commune's registers hold. Beyond
// that, PostgreSQL's `lower()` depends on the cluster's LC_CTYPE (package comment), so a DATABASE
// search must fold its term with SQL, not with this. Use Fold where both sides are compared in Go.
func Fold(s string) string { return strings.ToLower(replacer.Replace(s)) }

// SQL returns the SQL fold of `expr` — a column name or a placeholder such as `$3`. It is IMMUTABLE
// (lower, translate, constant arguments), so it may back a STORED GENERATED column or an index.
//
// THE TEXT IS STABLE BYTE FOR BYTE. A migration that stores it is checked against this function by
// a test; changing the table above therefore fails that test, which is the point: the stored column
// was computed with the OLD table, and a new table needs a new migration that recomputes it.
//
// U&'…' carries the combining marks as escapes so no invisible character sits in a migration file.
// It requires `standard_conforming_strings = on`, PostgreSQL's default since 9.1; off, the statement
// fails loudly rather than folding differently.
func SQL(expr string) string {
	var esc strings.Builder
	for _, m := range marks {
		esc.WriteString(`\`)
		esc.WriteString(strings.ToUpper(hex4(m)))
	}
	return "lower(translate(translate(" + expr + ", U&'" + esc.String() + "', ''), '" +
		fromLetters + "', '" + toLetters + "'))"
}

// hex4 is r as four hexadecimal digits, the form U&'\XXXX' takes.
func hex4(r rune) string {
	const digits = "0123456789abcdef"
	return string([]byte{digits[r>>12&0xF], digits[r>>8&0xF], digits[r>>4&0xF], digits[r&0xF]})
}
