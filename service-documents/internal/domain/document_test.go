package domain

// The rules of the two registers that hold regardless of storage. Everything here runs with no
// database and no clock of its own: a validation whose outcome depends on when the suite runs is a
// validation nobody trusts, so every function under test takes its instant as a parameter.

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var (
	testNow     = time.Date(2026, 9, 22, 8, 30, 0, 0, time.UTC)
	today       = time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	yesterday   = time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)
	tomorrow    = time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)
	sampleDue   = time.Date(2026, 9, 29, 3, 30, 0, 0, time.UTC)
	wellPastDue = time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
)

// --- overdue is DERIVED --------------------------------------------------------------------------

func TestIsOverdue_DerivedFromTheDeadlineNotAColumn(t *testing.T) {
	// RULE 10, INVARIANT 3. There is no column and no field behind this — the whole computation is
	// one comparison against the stored deadline, so it cannot go stale the way a flag set by a
	// nightly job does when the job is late, the clock skews or a holiday is added.
	d := IncomingDocument{DueAt: sampleDue, Status: IncomingStatusInProgress}

	if d.IsOverdue(testNow) {
		t.Error("còn hạn mà báo quá hạn")
	}
	if !d.IsOverdue(wellPastDue) {
		t.Error("đã quá hạn mà không báo")
	}
	// EXACTLY AT THE DEADLINE IS NOT LATE. "Trong 40 giờ làm việc" means up to and including that
	// instant; an off-by-one here turns a commitment met into a figure reported as missed.
	if d.IsOverdue(sampleDue) {
		t.Error("đúng thời điểm hạn mà đã báo quá hạn")
	}
}

func TestIsOverdue_FinishedDocumentIsNeverOverdue(t *testing.T) {
	// A document the register has finished with is not "still late": the commune did the work, and
	// whether it did so in time is a different question — one this register cannot answer yet,
	// because it does not store the instant the work finished. THAT GAP IS STATED rather than
	// papered over with a comparison against `now`, which would make every settled document drift
	// into "overdue" as time passed.
	for _, s := range []IncomingDocumentStatus{IncomingStatusResolved, IncomingStatusReferred, IncomingStatusFiledNotAdmitted} {
		d := IncomingDocument{DueAt: sampleDue, Status: s}
		if d.IsOverdue(wellPastDue) {
			t.Errorf("trạng thái %q đã kết thúc mà vẫn bị tính quá hạn", s)
		}
	}
}

func TestIsFinished_ThreeClosingCodesAndThreeOpenOnes(t *testing.T) {
	finished := map[IncomingDocumentStatus]bool{
		IncomingStatusRegistered: false, IncomingStatusAssigned: false, IncomingStatusInProgress: false,
		IncomingStatusResolved: true, IncomingStatusReferred: true, IncomingStatusFiledNotAdmitted: true,
	}
	for s, want := range finished {
		if s.IsFinished() != want {
			t.Errorf("%q.IsFinished() = %v, muốn %v", s, s.IsFinished(), want)
		}
	}
}

// --- routing only ever moves the state forward ----------------------------------------------------

func TestStatusAfterRouting_OnlyForwardNeverBack(t *testing.T) {
	// A brand-new entry becomes `da-phan-cong` — that IS what assigning it to a department means.
	if got := StatusAfterRouting(IncomingStatusRegistered); got != IncomingStatusAssigned {
		t.Errorf("mới vào sổ -> %q, muốn %q", got, IncomingStatusAssigned)
	}
	// A document already being worked on KEEPS ITS STATE. Re-routing a `dang-xu-ly` document back to
	// `da-phan-cong` would undo a fact somebody recorded, and the timeline would then show the work
	// going backwards.
	for _, s := range []IncomingDocumentStatus{IncomingStatusAssigned, IncomingStatusInProgress} {
		if got := StatusAfterRouting(s); got != s {
			t.Errorf("%q -> %q, muốn giữ nguyên", s, got)
		}
	}
}

func TestCanRoute_RefusesFinishedDocument(t *testing.T) {
	for _, s := range []IncomingDocumentStatus{IncomingStatusRegistered, IncomingStatusAssigned, IncomingStatusInProgress} {
		if err := (IncomingDocument{Status: s}).CanRoute(); err != nil {
			t.Errorf("%q: CanRoute = %v, muốn nil", s, err)
		}
	}
	for _, s := range []IncomingDocumentStatus{IncomingStatusResolved, IncomingStatusReferred, IncomingStatusFiledNotAdmitted} {
		err := (IncomingDocument{Status: s}).CanRoute()
		if !errors.Is(err, ErrDocumentFinished) {
			t.Errorf("%q: CanRoute = %v, muốn ErrDocumentFinished", s, err)
		}
		// THE SENTENCE NAMES THE STATE, because a clerk reading "không chuyển tiếp được" with no
		// reason retries the same button.
		if !strings.Contains(err.Error(), string(s)) {
			t.Errorf("thông báo không nói trạng thái hiện tại: %v", err)
		}
	}
}

// --- the dates -------------------------------------------------------------------------------------

func TestValidateReceivedDate(t *testing.T) {
	for name, tc := range map[string]struct {
		date time.Time
		want error
	}{
		"hôm nay":                   {today, nil},
		"hôm qua":                   {yesterday, nil},
		"để trống":                  {time.Time{}, ErrMissingReceivedDate},
		"ngày mai":                  {tomorrow, ErrReceivedDateInFuture},
		"quá xa trong quá khứ":      {testNow.Add(-MaxReceivedDateAge - 24*time.Hour), ErrReceivedDateTooOld},
		"vừa trong khoảng cho phép": {testNow.Add(-MaxReceivedDateAge + 24*time.Hour), nil},
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateReceivedDate(tc.date, testNow); !errors.Is(err, tc.want) {
				t.Fatalf("ValidateReceivedDate = %v, muốn %v", err, tc.want)
			}
		})
	}
}

func TestValidateReceivedDate_ComparedByDayNotByHour(t *testing.T) {
	// `ngay_den` IS A DATE and arrives as midnight UTC. Compared as an instant, a document booked at
	// 08:30 this morning would be "in the future" — which would refuse the single most ordinary
	// action in the whole register, and only in the morning.
	if err := ValidateReceivedDate(today, testNow); err != nil {
		t.Fatalf("vào sổ sáng nay cho ngày hôm nay bị từ chối: %v", err)
	}
}

func TestValidateIncomingDocumentDate_CannotBeSignedAfterArrival(t *testing.T) {
	// A document cannot arrive before it was signed. The reverse — signed long before it arrived — is
	// ordinary post and is accepted.
	early := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	if err := ValidateIncomingDocumentDate(early, today); err != nil {
		t.Errorf("ký trước, đến sau: %v, muốn nil", err)
	}
	if err := ValidateIncomingDocumentDate(time.Time{}, today); err != nil {
		t.Errorf("để trống là hợp lệ (trường tuỳ chọn): %v", err)
	}
	if err := ValidateIncomingDocumentDate(tomorrow, today); !errors.Is(err, ErrDocumentDateAfterReceived) {
		t.Errorf("ký sau ngày đến = %v, muốn ErrDocumentDateAfterReceived", err)
	}
	// SAME DAY IS FINE: a document signed and hand-delivered this morning.
	if err := ValidateIncomingDocumentDate(today, today); err != nil {
		t.Errorf("ký và đến cùng ngày: %v, muốn nil", err)
	}
}

func TestValidateOutgoingDocumentDate_RequiredAndNotInFuture(t *testing.T) {
	if err := ValidateOutgoingDocumentDate(time.Time{}, testNow); !errors.Is(err, ErrMissingDocumentDate) {
		t.Errorf("thiếu ngày = %v, muốn ErrMissingDocumentDate", err)
	}
	if err := ValidateOutgoingDocumentDate(tomorrow, testNow); !errors.Is(err, ErrDocumentDateInFuture) {
		t.Errorf("ngày tương lai = %v, muốn ErrDocumentDateInFuture", err)
	}
	if err := ValidateOutgoingDocumentDate(today, testNow); err != nil {
		t.Errorf("hôm nay = %v, muốn nil", err)
	}
}

// --- the free-text fields --------------------------------------------------------------------------

func TestNormalizeRequired_TrimsAndRefusesEmpty(t *testing.T) {
	s, err := NormalizeRequired("  Về việc rà soát hộ nghèo  ", MaxSummaryLen, ErrMissingSummary)
	if err != nil || s != "Về việc rà soát hộ nghèo" {
		t.Fatalf("NormalizeRequired = %q, %v", s, err)
	}
	if _, err := NormalizeRequired("   ", MaxSummaryLen, ErrMissingSummary); !errors.Is(err, ErrMissingSummary) {
		t.Fatalf("chuỗi toàn khoảng trắng = %v, muốn ErrMissingSummary", err)
	}
}

func TestNormalizeRequired_CountsRUNESNotBYTES(t *testing.T) {
	// A VIETNAMESE SUMMARY IS TWO TO THREE BYTES PER CHARACTER. Counted in bytes, the ceiling would
	// refuse a legitimate sentence at roughly a third of its apparent length — and the clerk would
	// see "quá dài" on a box that is visibly not full.
	long := strings.Repeat("ế", MaxSummaryLen) // 3 bytes each, exactly at the ceiling in runes
	if _, err := NormalizeRequired(long, MaxSummaryLen, ErrMissingSummary); err != nil {
		t.Fatalf("đúng trần theo ký tự mà bị từ chối: %v", err)
	}
	if _, err := NormalizeRequired(long+"ế", MaxSummaryLen, ErrMissingSummary); !errors.Is(err, ErrFieldTooLong) {
		t.Fatalf("vượt trần = %v, muốn ErrFieldTooLong", err)
	}
}

func TestNormalizeOptional_EmptyIsAnAnswer(t *testing.T) {
	// The empty value of an optional field is MEANINGFUL — "this document carries no number of its
	// own" is a statement — and it must survive as "" so the store can write NULL. One absent value
	// with two spellings in one column makes `WHERE … IS NULL` silently miss half the rows.
	s, err := NormalizeOptional("   ", MaxReferenceNoLen)
	if err != nil || s != "" {
		t.Fatalf("NormalizeOptional = %q, %v, muốn \"\", nil", s, err)
	}
}

func TestValidateUrgency(t *testing.T) {
	// The four values are the statutory list of Nghị định 30/2020/NĐ-CP, and the EMPTY one is a real
	// answer: "the commune did not record an urgency" is not the same statement as "Thường".
	for _, u := range []Urgency{UrgencyNotRecorded, UrgencyNormal, UrgencyUrgent, UrgencyVeryUrgent, UrgencyImmediate} {
		if err := ValidateUrgency(u); err != nil {
			t.Errorf("%q = %v, muốn nil", u, err)
		}
	}
	if err := ValidateUrgency("rat-khan"); !errors.Is(err, ErrInvalidUrgency) {
		t.Errorf("giá trị lạ = %v, muốn ErrInvalidUrgency", err)
	}
}

// --- the business code the trail is filed under ---------------------------------------------------

// TestDocumentCodes_TheTwoRegistersNeverShareOne PINS THE ISSUED CODE FORMAT. The strings below are
// already written into `audit_log.subject`, an append-only ledger; the English rename campaign renamed
// the functions and must never move a byte of what they produce (ADR 0061).
func TestDocumentCodes_TheTwoRegistersNeverShareOne(t *testing.T) {
	// `audit_log.subject` HOLDS THIS, and it is read years later by somebody handling a complaint or
	// an inspection (rule 6, invariant 8). Two things matter: the two registers can never collide,
	// and the strings sort the way the numbers do.
	if a, b := IncomingDocumentCode(2026, 7), OutgoingDocumentCode(2026, 7); a == b {
		t.Fatalf("số đến 7 và số đi 7 cùng một mã %q — hai văn bản khác nhau trong cùng một xã, "+
			"cùng một năm, và cả hai đều đúng", a)
	}
	if got := IncomingDocumentCode(2026, 7); got != "VB-DEN-2026-0007" {
		t.Fatalf("IncomingDocumentCode = %q, muốn VB-DEN-2026-0007", got)
	}
	if got := OutgoingDocumentCode(2026, 12); got != "VB-DI-2026-0012" {
		t.Fatalf("OutgoingDocumentCode = %q, muốn VB-DI-2026-0012", got)
	}
	// ZERO-PADDED, so số 7 sorts before số 12 in a ledger nobody can re-sort.
	if IncomingDocumentCode(2026, 7) > IncomingDocumentCode(2026, 12) {
		t.Fatal("mã không sắp đúng thứ tự số — thiếu đệm số 0")
	}
}
