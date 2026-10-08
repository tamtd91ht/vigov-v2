package domain

import (
	"errors"
	"fmt"
	"strings"
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

	// Nothing resolved → nil, never 100%.
	if BuildLetterReport(2026, rows[2:4], now).OnTimePercent != nil {
		t.Fatal("không có đơn nào đã giải quyết mà tỷ lệ đúng hạn khác nil")
	}
}

// ADR 0084 #4 (reverses C12): a resolved letter with NO deadline counts as ON TIME, over every letter
// resolved in the year; a unit's rate follows the same rule and is nil only when it resolved nothing;
// the average counts `da-giai-quyet` only (the prototype's PROCESSED), still inclusive (C16/C17);
// ByType and ByUnit are sorted by total, largest first.
func TestBuildLetterReportAfterADR0084(t *testing.T) {
	now := time.Date(2026, 10, 7, 3, 0, 0, 0, time.UTC)
	rec := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	booked := time.Date(2026, 3, 2, 2, 0, 0, 0, time.UTC)
	at := func(m time.Month, d int) time.Time { return time.Date(2026, m, d, 2, 0, 0, 0, time.UTC) }
	due := at(time.April, 1)
	resolved := func(typ LetterType, st LetterStatus, unit string, end, dueAt time.Time) CitizenLetter {
		return CitizenLetter{Year: 2026, Type: typ, Status: st, ReceivedDate: rec, CreatedAt: booked, AcceptedAt: booked,
			ResolvedAt: end, ClosedAt: end, ResolutionDueAt: dueAt, HoldingUnitID: unit}
	}
	open := CitizenLetter{Year: 2026, Type: LetterTypeRequest, Status: LetterStatusScreening, ReceivedDate: rec,
		CreatedAt: booked, HoldingUnitID: "bp-3"}
	rows := []CitizenLetter{
		// bp-1: one on time with a deadline, one with none (on time), one late.
		resolved(LetterTypeComplaint, LetterStatusResolved, "bp-1", at(time.March, 10), due),
		resolved(LetterTypeFeedback, LetterStatusResolved, "bp-1", at(time.March, 20), time.Time{}),
		resolved(LetterTypeComplaint, LetterStatusResolved, "bp-1", at(time.May, 1), due),
		// bp-2: discontinued with no deadline — resolved and on time, but NOT in the average.
		resolved(LetterTypeRequest, LetterStatusDiscontinued, "bp-2", at(time.June, 30), time.Time{}),
		// bp-3: resolved nothing.
		open, open, open,
	}
	r := BuildLetterReport(2026, rows, now)
	if r.Resolved != 4 || r.OnTimePercent == nil || *r.OnTimePercent != 75 {
		t.Fatalf("đã giải quyết %d, đúng hạn %v — muốn 4 và 75 (đơn không hạn tính là đúng hạn)", r.Resolved, r.OnTimePercent)
	}
	// da-giai-quyet only: 10, 20 and 62 days inclusive → 30.7; the dinh-chi letter (122 days) is out.
	if r.AverageDays == nil || *r.AverageDays != 30.7 {
		t.Fatalf("số ngày trung bình = %v, muốn 30.7 (chỉ đơn đã giải quyết, đếm cả hai đầu)", r.AverageDays)
	}
	units := map[string]LetterReportUnitRow{}
	for _, u := range r.ByUnit {
		units[u.UnitID] = u
	}
	if p := units["bp-1"].OnTimePercent; p == nil || *p != 66.7 {
		t.Fatalf("bp-1 đúng hạn = %v, muốn 66.7", p)
	}
	if p := units["bp-2"].OnTimePercent; p == nil || *p != 100 {
		t.Fatalf("bp-2 (đình chỉ, không hạn) đúng hạn = %v, muốn 100", p)
	}
	if units["bp-3"].OnTimePercent != nil {
		t.Fatal("bộ phận chưa giải quyết đơn nào phải là nil (web in —)")
	}
	if r.ByUnit[0].Total != 3 || r.ByUnit[len(r.ByUnit)-1].UnitID != "bp-2" {
		t.Fatalf("theo bộ phận chưa sắp theo tổng giảm dần: %+v", r.ByUnit)
	}
	// By type: de-nghi 4, khieu-nai 2, kien-nghi-phan-anh 1 — total desc, not C4 order.
	if len(r.ByType) != 3 || r.ByType[0].Type != LetterTypeRequest || r.ByType[1].Type != LetterTypeComplaint ||
		r.ByType[2].Type != LetterTypeFeedback {
		t.Fatalf("theo loại chưa sắp theo tổng giảm dần: %+v", r.ByType)
	}
	// A tie keeps C4 order.
	tie := BuildLetterReport(2026, []CitizenLetter{rows[3], rows[0]}, now)
	if tie.ByType[0].Type != LetterTypeComplaint || tie.ByType[1].Type != LetterTypeRequest {
		t.Fatalf("hoà tổng phải giữ thứ tự C4: %+v", tie.ByType)
	}
}

// ADR 0084 §2 table: every (status, held?) pair belongs to exactly one group, and the derived group is
// the one the filter rule selects.
func TestLetterGroupsPartitionEveryStatus(t *testing.T) {
	for _, st := range LetterStatuses {
		for _, unit := range []string{"", "bp-1"} {
			l := CitizenLetter{Status: st, HoldingUnitID: unit}
			hits := 0
			for _, g := range LetterStatusGroups() {
				rule, ok := g.Rule()
				if !ok {
					t.Fatalf("nhóm %q không có quy tắc", g)
				}
				inStatuses := false
				for _, s := range rule.Statuses {
					inStatuses = inStatuses || s == st
				}
				holds := rule.Holding == HoldingAny || (rule.Holding == HoldingNone) == (unit == "")
				if inStatuses && holds {
					hits++
					if l.StatusGroup() != g {
						t.Fatalf("%s/%q: nhóm suy ra %q, quy tắc chọn %q", st, unit, l.StatusGroup(), g)
					}
				}
			}
			if hits != 1 {
				t.Fatalf("%s/%q thuộc %d nhóm, muốn đúng 1", st, unit, hits)
			}
		}
	}
	want := map[LetterStatus]LetterStatusGroup{
		LetterStatusScreening: LetterGroupInProgress, LetterStatusAdmitted: LetterGroupInProgress,
		LetterStatusResolving: LetterGroupInProgress, LetterStatusResolved: LetterGroupResolved,
		LetterStatusForwarded: LetterGroupForwarded, LetterStatusNotAdmitted: LetterGroupNotProcessed,
		LetterStatusGuided: LetterGroupNotProcessed, LetterStatusFiled: LetterGroupNotProcessed,
		LetterStatusDiscontinued: LetterGroupNotProcessed,
	}
	for st, g := range want {
		if got := (CitizenLetter{Status: st}).StatusGroup(); got != g {
			t.Fatalf("%s → %q, muốn %q", st, got, g)
		}
	}
	if (CitizenLetter{Status: LetterStatusNew}).StatusGroup() != LetterGroupNew ||
		(CitizenLetter{Status: LetterStatusNew, HoldingUnitID: "bp-1"}).StatusGroup() != LetterGroupAssigned {
		t.Fatal("moi-vao-so: chưa có bộ phận là Mới vào sổ, có bộ phận là Đã phân công")
	}
	if LetterStatusGroup("cho-phan-cong").Valid() {
		t.Fatal("Chờ phân công không phải một nhóm (ADR 0084 #5)")
	}
	if got := LetterStatusGroups(); len(got) != 6 {
		t.Fatalf("có %d nhóm, muốn 6", len(got))
	}
}

func TestHasResultAndWithResultFollowTheType(t *testing.T) {
	doc := LetterResult{DocumentNo: "12/QĐ-UBND", DocumentDate: time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC),
		Signer: "Chủ tịch UBND xã", Issuer: "UBND xã", Summary: "Giữ nguyên quyết định"}
	for _, typ := range LetterTypes {
		empty := CitizenLetter{Type: typ}
		if empty.HasResult() == typ.RequiresIssuedResult() {
			t.Fatalf("%s: HasResult khi chưa ghi gì = %v", typ, empty.HasResult())
		}
		full, err := empty.WithResult(doc)
		if err != nil || !full.HasResult() {
			t.Fatalf("%s: kết quả đủ năm trường: %v", typ, err)
		}
		_, err = empty.WithResult(LetterResult{Summary: "Đã trả lời"})
		if typ.RequiresIssuedResult() != errors.Is(err, ErrLetterResultIncomplete) {
			t.Fatalf("%s: chỉ trả lời → %v", typ, err)
		}
	}
	if !LetterTypeComplaint.RequiresIssuedResult() || !LetterTypeDenunciation.RequiresIssuedResult() ||
		LetterTypeFeedback.RequiresIssuedResult() || LetterTypeRequest.RequiresIssuedResult() {
		t.Fatal("chỉ khiếu nại và tố cáo bắt buộc văn bản kết quả (ADR 0084 #2)")
	}
}

func TestLetterSourcesAreMigration0008s(t *testing.T) {
	want := []LetterSource{"nhap-tay", "nhap-excel", "mini-app", "thu-dien-tu"}
	if len(LetterSources) != len(want) {
		t.Fatalf("có %d nguồn", len(LetterSources))
	}
	for i, s := range want {
		if LetterSources[i] != s || !s.Valid() {
			t.Fatalf("nguồn %d = %q, muốn %q", i, LetterSources[i], s)
		}
	}
	if LetterSource("").Valid() || LetterSource("zalo").Valid() {
		t.Fatal("nguồn rỗng hoặc lạ được nhận")
	}
}

// The contract current_due_at: resolution deadline once admitted, processing deadline before — and
// NOT blank on a finished letter, unlike ActiveDueAt.
func TestCurrentStageDueAt(t *testing.T) {
	p, r := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC), time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	before := CitizenLetter{Status: LetterStatusScreening, ProcessingDueAt: p}
	if !before.CurrentStageDueAt().Equal(p) {
		t.Fatal("trước thụ lý phải là hạn xử lý đơn")
	}
	after := CitizenLetter{Status: LetterStatusResolved, ProcessingDueAt: p, ResolutionDueAt: r, AcceptedAt: p}
	if !after.CurrentStageDueAt().Equal(r) || !after.ActiveDueAt().IsZero() {
		t.Fatal("sau thụ lý phải là hạn giải quyết, kể cả khi đơn đã kết thúc")
	}
	if !(CitizenLetter{Status: LetterStatusAdmitted, AcceptedAt: p, ProcessingDueAt: p}).CurrentStageDueAt().IsZero() {
		t.Fatal("đã thụ lý mà chưa đặt hạn giải quyết thì không có hạn — không lùi về hạn xử lý đơn")
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

// THE CLERK'S DEADLINE (ADR 0079 lô 5 Q18), EXHAUSTIVELY over the ten statuses: an open letter takes it
// on the column ActiveDueAt reads for its phase, stored to the nanosecond as given; a finished one is
// refused; zero clears. A status that gained an arrow without a phase here would turn its cell red.
func TestSetActiveDueWritesTheCurrentPhaseColumnAsGiven(t *testing.T) {
	due := time.Date(2026, 10, 20, 16, 45, 30, 123, time.FixedZone("ICT", 7*3600))
	for _, s := range LetterStatuses {
		l := CitizenLetter{Status: s}
		got, err := l.SetActiveDue(due)
		if s.Finished() {
			if !errors.Is(err, ErrLetterDueOnFinished) || LetterDueColumn(s) != "" {
				t.Errorf("%s: đơn đã kết thúc mà đặt được hạn (%v)", s, err)
			}
			continue
		}
		if err != nil || !got.ActiveDueAt().Equal(due) || got.ActiveDueAt().Nanosecond() != 123 {
			t.Errorf("%s: hạn hiện hành %v, lỗi %v — muốn đúng %v", s, got.ActiveDueAt(), err, due)
		}
		other := got.ResolutionDueAt
		if LetterDueColumn(s) == "resolution_due_at" {
			other = got.ProcessingDueAt
		}
		if !other.IsZero() {
			t.Errorf("%s: ghi cả cột của giai đoạn khác", s)
		}
		cleared, err := got.SetActiveDue(time.Time{})
		if err != nil || !cleared.ActiveDueAt().IsZero() {
			t.Errorf("%s: bỏ hạn không xoá: %v %v", s, cleared.ActiveDueAt(), err)
		}
	}
	if _, err := (CitizenLetter{Status: LetterStatusNew}).SetActiveDue(time.Date(26, 1, 1, 0, 0, 0, 0, time.UTC)); !errors.Is(err, ErrLetterDueTooFar) {
		t.Errorf("năm 0026 phải bị từ chối: %v", err)
	}
}

// The incoming wrappers are the register methods, byte for byte — the documents' keys, titles and
// links did not move when letters joined (a moved key re-delivers every notice already sent that day).
func TestIncomingNoticesUnchangedAndLetterKeysNamespaced(t *testing.T) {
	r := AutomationRecord{ID: "x", Code: "VB-DEN-2026-0001", Deadline: time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)}
	if n := OverdueNotice(r, "2026-09-29", []string{"CB-1"}); n.Key != "sla_reminders:overdue:van-ban-den:x:2026-09-29" ||
		n.Title != "Văn bản đến VB-DEN-2026-0001 đã quá hạn xử lý" || n.Link != "/van-ban?metric=overdue" {
		t.Errorf("văn bản đến đổi khoá/câu: %+v", n)
	}
	r.Code = "DT-2026-0001"
	n := LetterNotices.Overdue(r, "2026-09-29", []string{"CB-1"})
	if n.Key != "sla_reminders:overdue:don-thu:x:2026-09-29" || n.Title != "Đơn thư DT-2026-0001 đã quá hạn xử lý" {
		t.Errorf("đơn thư: %+v", n)
	}
	for _, k := range []string{DueSoonKey(LetterNotices.Kind, "d", "CB-1"), EscalationKey(LetterNotices.Kind, "x", EscalationChairman),
		WeeklyDigestKey(LetterNotices.Kind, "2026-W40"), UnassignedKey(LetterNotices.Kind, "x", r.Deadline)} {
		if !ValidNoticeKey(k) || !strings.Contains(k, ":don-thu:") {
			t.Errorf("khoá đơn thư %q không nằm trong không gian don-thu", k)
		}
	}
}
