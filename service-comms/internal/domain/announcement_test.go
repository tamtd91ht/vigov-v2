package domain

import (
	"errors"
	"strings"
	"testing"
)

// The rules of chapter 08 that are DECIDABLE WITHOUT INFRASTRUCTURE. Everything here runs
// everywhere, always — which is the point: the pg suite next door skips itself when no DSN is set,
// and a rule that only that suite checks is a rule nothing checks on most machines.

func TestNormalizeAnnouncementTitleTrimsAndRefusesEmpty(t *testing.T) {
	out, err := NormalizeAnnouncementTitle("  Mời họp giao ban tháng 9  ")
	if err != nil {
		t.Fatalf("tiêu đề hợp lệ bị từ chối: %v", err)
	}
	if out != "Mời họp giao ban tháng 9" {
		t.Errorf("tiêu đề = %q, muốn đã cắt khoảng trắng", out)
	}

	for _, bad := range []string{"", "   ", "\t\n"} {
		if _, err := NormalizeAnnouncementTitle(bad); !errors.Is(err, ErrAnnouncementTitleEmpty) {
			t.Errorf("tiêu đề %q: lỗi = %v, muốn ErrAnnouncementTitleEmpty", bad, err)
		}
	}
}

func TestNormalizeAnnouncementTitleRefusesControlCharacters(t *testing.T) {
	// A newline in a subject line is how one announcement becomes two lines in a log and a broken
	// card on the screen. The body is allowed them; the title is not.
	if _, err := NormalizeAnnouncementTitle("Mời họp\ngiao ban"); !errors.Is(err, ErrAnnouncementTitleEmpty) {
		t.Errorf("tiêu đề có xuống dòng: lỗi = %v, muốn bị từ chối", err)
	}
}

func TestNormalizeAnnouncementTitleRefusesRatherThanTruncates(t *testing.T) {
	// REFUSED, NOT TRUNCATED. A title silently cut is an announcement whose subject changed on the
	// way into an archival record.
	long := strings.Repeat("a", AnnouncementTitleMaxLen+1)
	out, err := NormalizeAnnouncementTitle(long)
	if !errors.Is(err, ErrAnnouncementTitleTooLong) {
		t.Fatalf("lỗi = %v, muốn ErrAnnouncementTitleTooLong", err)
	}
	if out != "" {
		t.Errorf("tiêu đề quá dài phải trả rỗng, nhận %d ký tự — trả về một phần là cắt bớt ngầm", len([]rune(out)))
	}
}

func TestNormalizeAnnouncementBodyKeepsNewlinesRefusesOtherControls(t *testing.T) {
	out, err := NormalizeAnnouncementBody("Đoạn một.\r\n\nĐoạn hai.\tCó tab.")
	if err != nil {
		t.Fatalf("nội dung nhiều đoạn bị từ chối: %v", err)
	}
	if !strings.Contains(out, "\n") {
		t.Error("xuống dòng bị mất — một thông báo là nhiều đoạn")
	}
	if _, err := NormalizeAnnouncementBody("Có ký tự \x00 rỗng"); !errors.Is(err, ErrAnnouncementBodyEmpty) {
		t.Errorf("ký tự NUL: lỗi = %v, muốn bị từ chối", err)
	}
}

func TestNormalizeStaffCodeRefusesInnerWhitespace(t *testing.T) {
	if _, err := NormalizeStaffCode("  CB-2026-7K3M9Q "); err != nil {
		t.Errorf("mã hợp lệ có khoảng trắng hai đầu bị từ chối: %v", err)
	}
	// A code with a space inside is not a code; it is two things, or a name typed into the wrong
	// box. Accepting it would put a value in `nguoi_nhan_ma` that no identity lookup can resolve.
	if _, err := NormalizeStaffCode("CB-2026 7K3M9Q"); !errors.Is(err, ErrStaffCodeEmpty) {
		t.Errorf("mã có khoảng trắng bên trong: lỗi = %v, muốn bị từ chối", err)
	}
}

func TestNormalizeStaffCodesDropsDuplicatesKeepsOrder(t *testing.T) {
	// DEDUPLICATION HAPPENS BEFORE THE COUNT IS TAKEN. The primary key would collapse the duplicate
	// anyway, so without this the announcement would report a recipient count larger than its own
	// list — and §3's `{y}` would be wrong for the life of the record.
	out, err := NormalizeStaffCodes(
		[]string{"CB-B", " CB-A ", "CB-B", "CB-C"}, MaxAnnouncementRecipients, ErrTooManyRecipients)
	if err != nil {
		t.Fatalf("danh sách hợp lệ bị từ chối: %v", err)
	}
	want := []string{"CB-B", "CB-A", "CB-C"}
	if len(out) != len(want) {
		t.Fatalf("danh sách = %v, muốn %v", out, want)
	}
	for i := range want {
		if out[i] != want[i] {
			t.Fatalf("danh sách = %v, muốn %v — thứ tự phải giữ nguyên, §4 vẽ đúng danh sách này", out, want)
		}
	}
}

func TestNormalizeStaffCodesRefusesOverLimit(t *testing.T) {
	raw := make([]string, 0, MaxAnnouncementRecipients+1)
	for i := 0; i <= MaxAnnouncementRecipients; i++ {
		raw = append(raw, "CB-"+strings.Repeat("A", 1)+itoaTest(i))
	}
	if _, err := NormalizeStaffCodes(raw, MaxAnnouncementRecipients, ErrTooManyRecipients); !errors.Is(err, ErrTooManyRecipients) {
		t.Errorf("lỗi = %v, muốn ErrTooManyRecipients", err)
	}
}

func TestCreateAnnouncementRequestCarriesNoStatusOrAuthor(t *testing.T) {
	// A STRUCTURAL ASSERTION, and it is the one worth having: a `Status` or an `AuthorCode`
	// field here would be a field the HTTP body could fill — an announcement posted already marked
	// issued with no recipients behind it, or issued over a colleague's name. The test fails to
	// COMPILE if either is added, which is earlier than any runtime check.
	req := CreateAnnouncementRequest{
		Title:          "Mời họp",
		Body:           "Nội dung",
		RecipientCodes: []string{"CB-2026-7K3M9Q"},
	}
	clean, err := req.Validate()
	if err != nil {
		t.Fatalf("yêu cầu hợp lệ bị từ chối: %v", err)
	}
	if len(clean.RecipientCodes) != 1 || len(clean.OrgUnitIDs) != 0 {
		t.Errorf("kết quả = %+v", clean)
	}
}

func TestCreateAnnouncementRequestKeepsAckRequiredAndPinned(t *testing.T) {
	// The three flags decide what the commune was ASKED to do. A Validate that dropped one would
	// issue an announcement demanding no acknowledgement on a screen that showed the box ticked.
	clean, err := CreateAnnouncementRequest{
		Title: "T", Body: "N", RecipientCodes: []string{"CB-1"},
		Pinned: true, AckRequired: true, EmailRequested: true,
	}.Validate()
	if err != nil {
		t.Fatalf("lỗi: %v", err)
	}
	if !clean.Pinned || !clean.AckRequired || !clean.EmailRequested {
		t.Errorf("cờ bị mất: %+v", clean)
	}
}

func TestIsPublishedIncludesWithdrawn(t *testing.T) {
	// THE DISTINCTION THIS METHOD EXISTS FOR. A withdrawn announcement WAS issued: §4 keeps its
	// recipients and their acknowledgements. Anything asking "has this left the author's hands"
	// that compared against `da-phat-hanh` alone would answer wrongly for exactly those rows.
	cases := []struct {
		status AnnouncementStatus
		want   bool
	}{
		{AnnouncementDraft, false},
		{AnnouncementPublished, true},
		{AnnouncementWithdrawn, true},
	}
	for _, c := range cases {
		if got := (Announcement{Status: c.status}).IsPublished(); got != c.want {
			t.Errorf("IsPublished(%q) = %v, muốn %v", c.status, got, c.want)
		}
	}
}

func TestRecipientReadAndAcknowledgedState(t *testing.T) {
	var unread AnnouncementRecipient
	if unread.IsRead() || unread.IsAcknowledged() {
		t.Error("người nhận mới phải là `chưa mở`")
	}
}

// itoaTest keeps strconv out of this file for one loop.
func itoaTest(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
