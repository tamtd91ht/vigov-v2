package domain

import (
	"strings"
	"time"
	"unicode/utf8"
)

// The petition ↔ citizen-directory link (ADR 0102): the link is DERIVED at read time from
// (tenant_id, sender_phone_normalized); nothing here stores a directory id.

// ContactPetition is one row of the directory's "Xem phản ánh" — the petitions one person sent. ADR 0102
// #8 fixes its four facts: lookup code, title, status, date. NO REPORTER FIELD AND NO NUMBER, ever: the
// list is reached by a directory row id, and the person it names is already on the staff's screen.
type ContactPetition struct {
	LookupCode string
	// Content is `noi_dung`, the source of the row's title (ContactPetitionTitle). Citizen free text:
	// never logged (rule 3).
	Content    string
	Status     TrangThai
	ReceivedAt time.Time // `vao_so_luc` — the register's "booked at", the list's sort key
}

// The page bounds of "Xem phản ánh". 10 is the web's page (ADR 0102 request: "phân trang"); 50 caps a
// client asking for more, so one request never reads a person's whole history.
const (
	ContactPetitionPageSizeDefault = 10
	ContactPetitionPageSizeMax     = 50
)

// ContactPetitionTitleMaxRunes bounds the title. A petition has no title column: the register shows its
// content, so the title is the content's opening, whitespace folded, cut on a rune boundary.
const ContactPetitionTitleMaxRunes = 120

// ContactPetitionTitle is the title a "Xem phản ánh" row shows for one petition's content.
func ContactPetitionTitle(content string) string {
	folded := strings.Join(strings.Fields(content), " ")
	if utf8.RuneCountInString(folded) <= ContactPetitionTitleMaxRunes {
		return folded
	}
	r := []rune(folded)
	return strings.TrimSpace(string(r[:ContactPetitionTitleMaxRunes])) + "…"
}
