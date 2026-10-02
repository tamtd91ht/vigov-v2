package grpc

// What these tests defend: THE CODE EACH FAULT GETS on MeasureWorkingHours, and that a measured figure
// round-trips with AdvanceWorkingHours on the wire. The arithmetic itself is defended in
// internal/domain (measure_working_time_test.go).

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

func spanVN(t *testing.T, start, end string) *identityv1.WorkingTimeSpan {
	t.Helper()
	return &identityv1.WorkingTimeSpan{Start: timestamppb.New(mocVN(t, start)), End: timestamppb.New(mocVN(t, end))}
}

func measureReq(spans ...*identityv1.WorkingTimeSpan) *identityv1.MeasureWorkingHoursRequest {
	return &identityv1.MeasureWorkingHoursRequest{Spans: spans}
}

// Mỗi lỗi dưới đây là lỗi của BÊN GỌI → INVALID_ARGUMENT, và KHÔNG ĐƯỢC ĐỌC LỊCH.
func TestMeasureCallerFaultsAreInvalidArgumentAndReadNothing(t *testing.T) {
	ok := spanVN(t, "2026-09-21 08:00", "2026-09-21 10:00")
	tooMany := make([]*identityv1.WorkingTimeSpan, MaxMeasuredSpans+1)
	for i := range tooMany {
		tooMany[i] = ok // all duplicates: the ceiling is on what was SENT
	}
	cases := []struct {
		name string
		req  *identityv1.MeasureWorkingHoursRequest
	}{
		{"không có khoảng nào", measureReq()},
		{"quá 500 khoảng dù trùng hết", measureReq(tooMany...)},
		{"mục trống", measureReq(ok, nil)},
		{"thiếu start", measureReq(&identityv1.WorkingTimeSpan{End: ok.GetEnd()})},
		{"thiếu end", measureReq(&identityv1.WorkingTimeSpan{Start: ok.GetStart()})},
		{"start không hợp lệ", measureReq(&identityv1.WorkingTimeSpan{
			Start: &timestamppb.Timestamp{Seconds: 1, Nanos: -1}, End: ok.GetEnd()})},
		{"end không hợp lệ", measureReq(&identityv1.WorkingTimeSpan{
			Start: ok.GetStart(), End: &timestamppb.Timestamp{Seconds: 1 << 62}})},
		{"end trước start", measureReq(spanVN(t, "2026-09-21 10:00", "2026-09-21 08:00"))},
		{"cả lời gọi rộng quá 10 năm", measureReq(ok, spanVN(t, "2036-09-21 09:00", "2036-09-21 10:00"))},
		{"mốc năm 1 (cột chưa đặt)", measureReq(&identityv1.WorkingTimeSpan{
			Start: timestamppb.New(time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)), End: ok.GetEnd()})},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			lich := &lichGia{cas: tuanGia()}
			s, _ := may(t, func(d *Deps) { d.Lich = lich })
			_, err := s.MeasureWorkingHours(ctxXa(xaA), c.req)
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("mã = %v, muốn InvalidArgument (lỗi: %v)", status.Code(err), err)
			}
			if lich.soLanGoi != 0 {
				t.Errorf("đã đọc lịch %d lần cho một lời gọi sai hình dạng — phải từ chối trước", lich.soLanGoi)
			}
		})
	}
}

// Exactly ten calendar years is still inside the bound.
func TestMeasureExactlyTenYearsIsAccepted(t *testing.T) {
	s, _ := may(t, nil)
	if _, err := s.MeasureWorkingHours(ctxXa(xaA), measureReq(spanVN(t, "2026-09-21 08:00", "2036-09-21 08:00"))); err != nil {
		t.Fatalf("10 năm tròn bị từ chối: %v", err)
	}
}

// LỊCH TRỐNG LÀ FAILED_PRECONDITION, KHÔNG BAO GIỜ LÀ 0 — 0 đọc thành "không trễ" trên mọi dòng.
func TestMeasureEmptyCalendarIsFailedPreconditionNotZero(t *testing.T) {
	s, _ := may(t, func(d *Deps) { d.Lich = &lichGia{} })
	ra, err := s.MeasureWorkingHours(ctxXa(xaA), measureReq(spanVN(t, "2026-09-21 08:00", "2026-09-22 08:00")))
	if err == nil {
		t.Fatalf("lịch trống mà vẫn trả OK: %+v", ra.GetItems())
	}
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("mã = %v, muốn FailedPrecondition (lỗi: %v)", status.Code(err), err)
	}
}

func TestMeasureCalendarConflictIsFailedPrecondition(t *testing.T) {
	s, _ := may(t, func(d *Deps) {
		d.NghiLe = &nghiLeGia{err: &domain.LoiNgayVuaNghiVuaLamBu{Ngay: []string{"2026-09-19"}}}
	})
	_, err := s.MeasureWorkingHours(ctxXa(xaA), measureReq(spanVN(t, "2026-09-18 22:00", "2026-09-21 09:00")))
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("mã = %v, muốn FailedPrecondition (lỗi: %v)", status.Code(err), err)
	}
}

func TestMeasureStoreFailureIsInternal(t *testing.T) {
	for name, fix := range map[string]func(*Deps){
		"kho lịch làm việc hỏng": func(d *Deps) { d.Lich = &lichGia{err: loiKho} },
		"kho ngày nghỉ lễ hỏng":  func(d *Deps) { d.NghiLe = &nghiLeGia{err: loiKho} },
		"kho ngày làm bù hỏng":   func(d *Deps) { d.LamBu = &lamBuGia{err: loiKho} },
	} {
		t.Run(name, func(t *testing.T) {
			s, _ := may(t, fix)
			ra, err := s.MeasureWorkingHours(ctxXa(xaA), measureReq(spanVN(t, "2026-09-21 08:00", "2026-09-21 10:00")))
			if status.Code(err) != codes.Internal {
				t.Fatalf("mã = %v, muốn Internal (lỗi: %v, trả: %+v)", status.Code(err), err, ra.GetItems())
			}
			if msg := status.Convert(err).Message(); msg != "lỗi nội bộ, vui lòng thử lại" {
				t.Errorf("thông điệp = %q — không được mang nguyên nhân qua ranh giới", msg)
			}
		})
	}
}

func TestMeasureWithoutCommuneIsInternalNotPanic(t *testing.T) {
	s, _ := may(t, nil)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("handler panic khi thiếu xã: %v", r)
		}
	}()
	_, err := s.MeasureWorkingHours(context.Background(), measureReq(spanVN(t, "2026-09-21 08:00", "2026-09-21 10:00")))
	if status.Code(err) != codes.Internal {
		t.Fatalf("mã = %v, muốn Internal (lỗi: %v)", status.Code(err), err)
	}
}

// One item per DISTINCT span, each echoing its span; whole seconds truncated; start == end and a span
// inside a weekend are real zeros; ONE calendar read for the whole call.
func TestMeasureAnswersEveryDistinctSpanOnce(t *testing.T) {
	lich := &lichGia{cas: tuanGia()}
	s, _ := may(t, func(d *Deps) { d.Lich = lich })

	frac := &identityv1.WorkingTimeSpan{
		Start: timestamppb.New(mocVN(t, "2026-09-21 08:00")),
		End:   timestamppb.New(mocVN(t, "2026-09-21 08:00").Add(90*time.Second + 999*time.Millisecond)),
	}
	friday := spanVN(t, "2026-09-18 22:00", "2026-09-21 09:30")
	ra, err := s.MeasureWorkingHours(ctxXa(xaA), measureReq(
		friday,
		spanVN(t, "2026-09-21 09:00", "2026-09-21 09:00"), // start == end
		spanVN(t, "2026-09-19 08:00", "2026-09-20 20:00"), // inside a weekend
		frac,
		spanVN(t, "2026-09-18 22:00", "2026-09-21 09:30"), // duplicate of friday
	))
	if err != nil {
		t.Fatalf("MeasureWorkingHours: %v", err)
	}
	if len(ra.GetItems()) != 4 {
		t.Fatalf("số item = %d, muốn 4 (trùng phải gộp)", len(ra.GetItems()))
	}
	got := map[[2]time.Time]uint64{}
	for _, it := range ra.GetItems() {
		got[[2]time.Time{it.GetSpan().GetStart().AsTime(), it.GetSpan().GetEnd().AsTime()}] = it.GetWorkingSeconds()
	}
	want := map[*identityv1.WorkingTimeSpan]uint64{
		friday: 2 * 3600,
		spanVN(t, "2026-09-21 09:00", "2026-09-21 09:00"): 0,
		spanVN(t, "2026-09-19 08:00", "2026-09-20 20:00"): 0,
		frac: 90,
	}
	for sp, w := range want {
		k := [2]time.Time{sp.GetStart().AsTime(), sp.GetEnd().AsTime()}
		if g, ok := got[k]; !ok || g != w {
			t.Errorf("span %s → %s: %d giây (có=%v), muốn %d", k[0], k[1], g, ok, w)
		}
	}
	if lich.soLanGoi != 1 {
		t.Errorf("đọc lịch %d lần, muốn đúng 1 cho cả lời gọi", lich.soLanGoi)
	}
}

// THE INVERSE PROPERTY ON THE WIRE: measure(t, AdvanceWorkingHours(t, h)) == h × 3600.
func TestMeasureRoundTripsWithAdvance(t *testing.T) {
	holiday := func(d *Deps) {
		d.NghiLe = &nghiLeGia{ds: []domain.NgayNghiLe{{ID: "le-1", Ngay: "2026-09-21", Ten: "Lễ giả định"}}}
	}
	swap := func(d *Deps) {
		d.LamBu = &lamBuGia{ds: []domain.CaLamBu{{ID: "bu-1", Ngay: "2026-09-19",
			BatDau: 7*3600 + 30*60, KetThuc: 11*3600 + 30*60, Ten: "Làm bù giả định"}}}
	}
	cases := []struct {
		name  string
		from  string
		hours uint32
		fix   func(*Deps)
	}{
		{"qua cuối tuần", "2026-09-18 22:00", 2, nil},
		{"thứ Hai nghỉ lễ", "2026-09-18 22:00", 16, holiday},
		{"thứ Bảy làm bù", "2026-09-18 22:00", 2, swap},
		{"hết đúng 11:30", "2026-09-21 08:30", 3, nil},
		{"168 giờ", "2026-09-21 07:30", 168, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, _ := may(t, c.fix)
			adv, err := s.AdvanceWorkingHours(ctxXa(xaA), yeuCau(t, c.from, c.hours))
			if err != nil {
				t.Fatalf("AdvanceWorkingHours: %v", err)
			}
			m, err := s.MeasureWorkingHours(ctxXa(xaA), measureReq(&identityv1.WorkingTimeSpan{
				Start: timestamppb.New(mocVN(t, c.from)), End: adv.GetItems()[0].GetReachedAt()}))
			if err != nil {
				t.Fatalf("MeasureWorkingHours: %v", err)
			}
			if g := m.GetItems()[0].GetWorkingSeconds(); g != uint64(c.hours)*3600 {
				t.Errorf("đo lại được %d giây, muốn %d", g, uint64(c.hours)*3600)
			}
		})
	}
}

// MÚI GIỜ LÀ CỦA XÃ, KHÔNG PHẢI CỦA TIẾN TRÌNH.
func TestMeasureIndependentOfProcessTZ(t *testing.T) {
	orig := time.Local
	t.Cleanup(func() { time.Local = orig })
	for _, z := range []*time.Location{time.UTC, time.FixedZone("GIA-TAY", -8*3600)} {
		time.Local = z
		s, _ := may(t, nil)
		ra, err := s.MeasureWorkingHours(ctxXa(xaA), measureReq(spanVN(t, "2026-09-21 08:00", "2026-09-21 15:00")))
		if err != nil {
			t.Fatalf("TZ=%s: %v", z, err)
		}
		if g := ra.GetItems()[0].GetWorkingSeconds(); g != 5*3600 {
			t.Errorf("TZ=%s: %d giây, muốn %d", z, g, 5*3600)
		}
	}
}
