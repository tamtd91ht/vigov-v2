package domain

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

// C3, EXHAUSTIVELY: every (from, to) pair of the ten statuses is either one of the nine arrows the
// user decided on 24/09/2026 or refused. A table edited by accident turns exactly one cell red.
func TestLetterTransitionTableIsC3(t *testing.T) {
	allowed := map[[2]LetterStatus]bool{
		{LetterStatusNew, LetterStatusScreening}:          true,
		{LetterStatusScreening, LetterStatusAdmitted}:     true,
		{LetterStatusScreening, LetterStatusNotAdmitted}:  true,
		{LetterStatusScreening, LetterStatusGuided}:       true,
		{LetterStatusScreening, LetterStatusForwarded}:    true,
		{LetterStatusScreening, LetterStatusFiled}:        true,
		{LetterStatusAdmitted, LetterStatusResolving}:     true,
		{LetterStatusResolving, LetterStatusResolved}:     true,
		{LetterStatusResolving, LetterStatusDiscontinued}: true,
	}
	for _, from := range LetterStatuses {
		for _, to := range LetterStatuses {
			err := CheckLetterTransition(from, to)
			if allowed[[2]LetterStatus{from, to}] {
				if err != nil {
					t.Errorf("%s → %s bị từ chối, C3 cho phép: %v", from, to, err)
				}
				continue
			}
			if !errors.Is(err, ErrLetterTransitionRefused) {
				t.Errorf("%s → %s được nhận, C3 không vẽ mũi tên này", from, to)
			}
		}
	}
}

func TestLetterTransitionRefusalNamesBothCodes(t *testing.T) {
	err := CheckLetterTransition(LetterStatusNew, LetterStatusAdmitted)
	var te *LetterTransitionError
	if !errors.As(err, &te) || te.From != LetterStatusNew || te.To != LetterStatusAdmitted {
		t.Fatalf("lỗi chuyển trạng thái không mang hai mã: %v", err)
	}
	if !errors.Is(CheckLetterTransition(LetterStatusNew, "da-phan-cong"), ErrLetterStatusInvalid) {
		t.Fatal("một mã ngoài bộ mười phải là lỗi đầu vào, không phải lỗi chuyển")
	}
}

func TestLetterStatusFinishedSets(t *testing.T) {
	finished := map[LetterStatus]bool{}
	for _, s := range LetterStatuses {
		if s.Finished() {
			finished[s] = true
		}
	}
	for _, s := range []LetterStatus{LetterStatusNotAdmitted, LetterStatusGuided, LetterStatusForwarded,
		LetterStatusFiled, LetterStatusResolved, LetterStatusDiscontinued} {
		if !finished[s] {
			t.Errorf("%s phải là trạng thái kết thúc", s)
		}
	}
	if len(finished) != 6 || len(FinishedInProcessing()) != 4 {
		t.Fatalf("kết thúc = %d, kết thúc ở giai đoạn xử lý đơn = %d; muốn 6 và 4", len(finished), len(FinishedInProcessing()))
	}
}

func TestLetterTypesAreC4(t *testing.T) {
	for _, s := range []string{"kien-nghi-phan-anh", "khieu-nai", "to-cao", "de-nghi"} {
		if !LetterType(s).Valid() {
			t.Errorf("%s phải hợp lệ", s)
		}
	}
	// `phan-anh` alone is service-petitions' word, not a letter type (0006:192-193).
	if LetterType("phan-anh").Valid() {
		t.Fatal("`phan-anh` không phải loại đơn thư")
	}
}

func TestDaysOpenCountsTheReceivedDayInVietnam(t *testing.T) {
	received := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC) // a DATE, as PostgreSQL hands it back
	l := CitizenLetter{ReceivedDate: received}
	cases := []struct {
		now  time.Time
		want int
	}{
		{time.Date(2026, 9, 20, 3, 0, 0, 0, time.UTC), 1},  // same day
		{time.Date(2026, 9, 22, 16, 0, 0, 0, time.UTC), 3}, // 23:00 on the 22nd in Vietnam
		{time.Date(2026, 9, 22, 18, 0, 0, 0, time.UTC), 4}, // 01:00 on the 23rd in Vietnam
		{time.Date(2026, 9, 19, 10, 0, 0, 0, time.UTC), 1}, // never below one
	}
	for _, c := range cases {
		if got := l.DaysOpen(c.now); got != c.want {
			t.Errorf("DaysOpen(%s) = %d, muốn %d", c.now, got, c.want)
		}
	}
	l.ClosedAt = time.Date(2026, 9, 24, 2, 0, 0, 0, time.UTC)
	if got := l.DaysOpen(time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)); got != 5 {
		t.Fatalf("đơn đã đóng: DaysOpen = %d, muốn 5 (tới ngày đóng, không tới hôm nay)", got)
	}
}

func TestNoDeadlineIsNeverOverdue(t *testing.T) {
	l := CitizenLetter{Status: LetterStatusScreening}
	if l.IsOverdue(time.Now().Add(1e6 * time.Second)) {
		t.Fatal("đơn không có hạn bị tính quá hạn — hai hạn để trống theo ADR 0078 #3")
	}
	l.ProcessingDueAt = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if !l.IsOverdue(time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("đơn có hạn xử lý đã qua phải quá hạn")
	}
	l.Status = LetterStatusFiled
	if l.IsOverdue(time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("đơn đã kết thúc không bao giờ quá hạn")
	}
}

func TestDisclosure(t *testing.T) {
	ordinary := CitizenLetter{Type: LetterTypeComplaint, AssigneeCode: "CB-1"}
	if (ListDisclosure(ordinary.Type) != LetterDisclosure{Identity: true, Summary: true}) {
		t.Fatal("đơn thường phải hiện trên danh sách")
	}
	if (ListDisclosure(LetterTypeDenunciation) != LetterDisclosure{}) {
		t.Fatal("đơn tố cáo không được mang danh tính hay trích yếu trên danh sách")
	}
	den := CitizenLetter{Type: LetterTypeDenunciation, AssigneeCode: "CB-1"}
	cases := []struct {
		viewer  string
		canBook bool
		want    LetterDisclosure
	}{
		{"CB-2", false, LetterDisclosure{}},
		{"CB-2", true, LetterDisclosure{Summary: true}},
		{"CB-1", false, LetterDisclosure{Identity: true, Summary: true}},
		{"", false, LetterDisclosure{}},
	}
	for _, c := range cases {
		if got := DetailDisclosure(den, c.viewer, c.canBook); got != c.want {
			t.Errorf("người xem %q (tiếp nhận=%v): %+v, muốn %+v", c.viewer, c.canBook, got, c.want)
		}
	}
	// An unassigned denunciation discloses its sender to NOBODY — "" must never equal "".
	if DetailDisclosure(CitizenLetter{Type: LetterTypeDenunciation}, "", true).Identity {
		t.Fatal("đơn tố cáo chưa giao lộ danh tính cho người có mã rỗng")
	}
}

func TestSummarySimilarity(t *testing.T) {
	a := "Đề nghị sửa đường liên thôn bị sạt lở"
	if s := SummarySimilarity(a, a); s != 1 {
		t.Fatalf("hai trích yếu giống hệt: %.2f, muốn 1", s)
	}
	if s := SummarySimilarity(a, "đề nghị SỬA đường liên thôn bị sạt lở!"); s != 1 {
		t.Fatalf("khác hoa thường và dấu câu: %.2f, muốn 1", s)
	}
	if s := SummarySimilarity(a, "Khiếu nại quyết định thu hồi đất nông nghiệp"); s >= DuplicateSimilarityThreshold {
		t.Fatalf("hai trích yếu khác hẳn mà giống %.2f", s)
	}
	if s := SummarySimilarity(a, "Đề nghị sửa đường liên thôn sạt lở"); s < DuplicateSimilarityThreshold {
		t.Fatalf("gần như cùng một đơn mà chỉ giống %.2f", s)
	}
	if SummarySimilarity("", a) != 0 {
		t.Fatal("trích yếu rỗng không giống gì")
	}
}

func TestRankDuplicatesKeepsFiveMostSimilar(t *testing.T) {
	base := "Đề nghị sửa đường liên thôn bị sạt lở"
	var rows []CitizenLetter
	for i := 1; i <= 7; i++ {
		rows = append(rows, CitizenLetter{ID: fmt.Sprint(i), Year: 2026, Number: i, Summary: base})
	}
	rows = append(rows, CitizenLetter{ID: "x", Year: 2026, Number: 99, Summary: "Xin cấp giấy xác nhận cư trú"})
	// An identical denunciation with the highest number: it must still never be a candidate.
	rows = append(rows, CitizenLetter{ID: "tc", Year: 2026, Number: 100, Type: LetterTypeDenunciation, Summary: base})
	got := RankDuplicates(base, rows)
	if len(got) != DuplicateMaxCandidates {
		t.Fatalf("%d ứng viên, muốn %d", len(got), DuplicateMaxCandidates)
	}
	if got[0].Letter.Number != 7 {
		t.Fatalf("bằng điểm thì số mới hơn trước: %d", got[0].Letter.Number)
	}
	for _, c := range got {
		if c.Letter.ID == "x" {
			t.Fatal("ứng viên dưới ngưỡng vẫn được trả")
		}
		if c.Letter.ID == "tc" {
			t.Fatal("đơn tố cáo là ứng viên trùng — lộ người tố cáo (ADR 0078 #4)")
		}
	}
}

func TestBuildLetterReport(t *testing.T) {
	now := time.Date(2026, 10, 7, 3, 0, 0, 0, time.UTC)
	booked := time.Date(2026, 3, 2, 2, 0, 0, 0, time.UTC)
	rec := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	due := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	rows := []CitizenLetter{
		{Year: 2026, Type: LetterTypeComplaint, Status: LetterStatusResolved, ReceivedDate: rec, CreatedAt: booked,
			AcceptedAt: booked, ResolvedAt: time.Date(2026, 3, 10, 2, 0, 0, 0, time.UTC), ClosedAt: time.Date(2026, 3, 10, 2, 0, 0, 0, time.UTC),
			ResolutionDueAt: due, HoldingUnitID: "bp-1"},
		{Year: 2026, Type: LetterTypeComplaint, Status: LetterStatusResolved, ReceivedDate: rec, CreatedAt: booked,
			AcceptedAt: booked, ResolvedAt: time.Date(2026, 5, 1, 2, 0, 0, 0, time.UTC), ClosedAt: time.Date(2026, 5, 1, 2, 0, 0, 0, time.UTC),
			ResolutionDueAt: due, HoldingUnitID: "bp-1"},
		{Year: 2026, Type: LetterTypeDenunciation, Status: LetterStatusScreening, ReceivedDate: rec, CreatedAt: booked},
		{Year: 2026, Type: LetterTypeRequest, Status: LetterStatusFiled, ReceivedDate: rec, CreatedAt: booked},
		// carried over from 2025, still open
		{Year: 2025, Type: LetterTypeFeedback, Status: LetterStatusAdmitted, ReceivedDate: rec.AddDate(-1, 0, 0),
			CreatedAt: booked.AddDate(-1, 0, 0), AcceptedAt: booked.AddDate(-1, 0, 0), ResolutionDueAt: due.AddDate(-1, 0, 0)},
	}
	r := BuildLetterReport(2026, rows, now)
	if r.Received != 4 || r.Resolved != 2 || r.InProgress != 2 || r.ClosedInProcessing != 1 || r.PastDue != 1 {
		t.Fatalf("nhận %d · giải quyết %d · đang xử lý %d · kết thúc ở xử lý đơn %d · quá hạn %d",
			r.Received, r.Resolved, r.InProgress, r.ClosedInProcessing, r.PastDue)
	}
	if r.OnTimePercent == nil || *r.OnTimePercent != 50 {
		t.Fatalf("tỷ lệ đúng hạn = %v, muốn 50", r.OnTimePercent)
	}
	// 10 days (1/3 → 10/3) and 62 days (1/3 → 1/5), both counted inclusive.
	if r.AverageDays == nil || *r.AverageDays != 36 {
		t.Fatalf("số ngày trung bình = %v, muốn 36", r.AverageDays)
	}
	if r.ByMonth[2].Received != 4 || r.ByMonth[2].Resolved != 1 || r.ByMonth[4].Resolved != 1 {
		t.Fatalf("theo tháng sai: %+v", r.ByMonth)
	}
	if r.ByUnit[0].UnitID != "" && r.ByUnit[0].UnitID != "bp-1" {
		t.Fatalf("theo bộ phận sai: %+v", r.ByUnit)
	}

	// C12: no letter with a deadline → nil, never 100%.
	if BuildLetterReport(2026, rows[2:4], now).OnTimePercent != nil {
		t.Fatal("không có đơn nào có hạn mà tỷ lệ đúng hạn khác nil")
	}
}

func TestCheckReceivedDate(t *testing.T) {
	now := time.Date(2026, 10, 7, 18, 0, 0, 0, time.UTC) // 01:00 on the 8th in Vietnam
	if err := CheckReceivedDate(time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC), now); err != nil {
		t.Fatalf("hôm nay theo giờ Việt Nam bị coi là tương lai: %v", err)
	}
	if !errors.Is(CheckReceivedDate(time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC), now), ErrLetterReceivedDateFuture) {
		t.Fatal("ngày mai phải bị từ chối")
	}
	if !errors.Is(CheckReceivedDate(time.Time{}, now), ErrLetterReceivedDateMissing) {
		t.Fatal("thiếu ngày nhận phải bị từ chối")
	}
	if !errors.Is(CheckReceivedDate(time.Date(2006, 10, 7, 0, 0, 0, 0, time.UTC), now), ErrLetterDateTooOld) {
		t.Fatal("ngày gõ nhầm thế kỷ phải bị từ chối")
	}
}

func TestTrimRefinementStillMatchesItsSentinel(t *testing.T) {
	// Counted in RUNES: ten Vietnamese letters with tone marks are ten, not twenty-odd bytes.
	if _, err := TrimRequired("ệệệệệệệệệệ", 10, ErrLetterSummaryMissing); err != nil {
		t.Fatalf("10 ký tự có dấu bị coi là quá 10: %v", err)
	}
	_, err := TrimRequired("ệệệệệệệệệệệ", 10, ErrLetterSummaryMissing)
	var le *LetterError
	if !errors.Is(err, ErrLetterFieldTooLong) || !errors.As(err, &le) || le.Refusal != RefusalInput {
		t.Fatalf("lỗi quá dài không còn khớp mẫu gốc: %v", err)
	}
	if _, err := TrimRequired("   ", 10, ErrLetterSummaryMissing); !errors.Is(err, ErrLetterSummaryMissing) {
		t.Fatal("chuỗi toàn dấu cách phải là thiếu")
	}
	if _, err := TrimPhone("0900 000 000"); err != nil {
		t.Fatalf("số có dấu cách bị từ chối: %v", err)
	}
	if _, err := TrimPhone("09000abc"); !errors.Is(err, ErrLetterPhoneInvalid) {
		t.Fatal("chữ cái trong số điện thoại phải bị từ chối")
	}
}

func TestLetterAuditSubject(t *testing.T) {
	if got := LetterAuditSubject(2026, 7); got != "DT-2026-0007" {
		t.Fatalf("mã đối tượng = %q", got)
	}
}
