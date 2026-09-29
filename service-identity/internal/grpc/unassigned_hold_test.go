package grpc

// What these tests defend: ResolveUnassignedHoldInstants' status table and its one piece of arithmetic
// — `unassigned_hold_hours` counted FORWARD through the commune's calendar from the hold start — and,
// above all, that NULL answers "disabled" and never a number.

import (
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// holdRow is a default `don-thu` row whose `unassigned_hold_hours` is `hours` (0 = NULL), every other
// column DIFFERENT from the values tests use, so reading the wrong column shows up as a wrong instant.
func holdRow(hours int) domain.DongSLA {
	return domain.DongSLA{ID: "sla-dt", LoaiViec: domain.LoaiViecDonThu,
		GioTiepNhan: 8, GioXuLyXong: 40, GioSapDenHan: 24, GioBaoLanhDao: 5, GioBaoChuTich: 10,
		UnassignedHoldHours: hours}
}

func holdReq(t *testing.T, starts ...string) *identityv1.ResolveUnassignedHoldInstantsRequest {
	t.Helper()
	req := &identityv1.ResolveUnassignedHoldInstantsRequest{WorkKind: identityv1.WorkKind_WORK_KIND_DON_THU}
	for _, m := range starts {
		req.HoldStartedAt = append(req.HoldStartedAt, timestamppb.New(mocVN(t, m)))
	}
	return req
}

// Held from Friday 2026-10-02 16:00, 3 working hours; Monday 2026-10-05 is a holiday.
//
//	Fri 16:00–17:00 = 1h, weekend and Monday skipped, Tue 07:30 + 2h = 09:30
//
// Wall-clock arithmetic would say Fri 19:00 — the answer rule 10 forbids.
func TestUnassignedHoldCountsWorkingHoursAcrossWeekendAndHoliday(t *testing.T) {
	s, _ := may(t, func(d *Deps) {
		d.SLA = &slaGia{ds: []domain.DongSLA{holdRow(3)}}
		d.NghiLe = &nghiLeGia{ds: []domain.NgayNghiLe{{ID: "nl-1", Ngay: "2026-10-05"}}}
	})
	ra, err := s.ResolveUnassignedHoldInstants(ctxXa(xaA), holdReq(t, "2026-10-02 16:00", "2026-10-02 16:00"))
	if err != nil {
		t.Fatalf("ResolveUnassignedHoldInstants: %v", err)
	}
	if ra.GetReportingDisabled() {
		t.Fatal("reporting_disabled = true for a row holding 3 hours")
	}
	if len(ra.GetItems()) != 1 {
		t.Fatalf("%d items, want 1 — duplicates collapse", len(ra.GetItems()))
	}
	it := ra.GetItems()[0]
	if !it.GetHoldStartedAt().AsTime().Equal(mocVN(t, "2026-10-02 16:00")) {
		t.Errorf("hold_started_at not echoed: %v", it.GetHoldStartedAt().AsTime())
	}
	if got, want := it.GetUnassignedReportDueAt().AsTime(), mocVN(t, "2026-10-06 09:30"); !got.Equal(want) {
		t.Errorf("unassigned_report_due_at = %s, want %s", got.In(muiDoiChungVN), want)
	}
}

// NULL IS "DO NOT REPORT": OK, disabled, no items — and no calendar read, so an empty calendar (which
// would be FAILED_PRECONDITION if counted) does not turn "disabled" into a failed run.
//
// MUTATION THAT MUST TURN THIS RED: substitute any default hour count for 0.
func TestUnassignedHoldNullIsDisabledNeverANumber(t *testing.T) {
	lich := &lichGia{}
	s, _ := may(t, func(d *Deps) { d.SLA = &slaGia{ds: []domain.DongSLA{holdRow(0)}}; d.Lich = lich })
	ra, err := s.ResolveUnassignedHoldInstants(ctxXa(xaA), holdReq(t, "2026-10-02 16:00"))
	if err != nil {
		t.Fatalf("NULL threshold: %v — must be OK, disabled", err)
	}
	if !ra.GetReportingDisabled() || len(ra.GetItems()) != 0 {
		t.Errorf("disabled = %v, items = %d; want true and 0", ra.GetReportingDisabled(), len(ra.GetItems()))
	}
	if lich.soLanGoi != 0 {
		t.Error("calendar read for a disabled threshold")
	}
}

// THE FALLBACK IS BY ROW, NOT BY COLUMN: a field row holding NULL stays disabled even though the
// default row holds a number.
func TestUnassignedHoldFieldRowNullDoesNotBorrowDefaultRow(t *testing.T) {
	field := holdRow(0)
	field.ID, field.LinhVuc = "sla-dt-f", "dat-dai"
	s, _ := may(t, func(d *Deps) { d.SLA = &slaGia{ds: []domain.DongSLA{holdRow(3), field}} })
	req := holdReq(t, "2026-10-02 16:00")
	req.LinhVuc = "dat-dai"
	ra, err := s.ResolveUnassignedHoldInstants(ctxXa(xaA), req)
	if err != nil || !ra.GetReportingDisabled() {
		t.Fatalf("field row NULL: %v %v — want OK, disabled (never the default row's 3 hours)", ra, err)
	}
}

func TestUnassignedHoldCallerFaultsAreInvalidArgumentAndReadNothing(t *testing.T) {
	tooMany := holdReq(t)
	for i := 0; i <= MaxHoldStarts; i++ {
		tooMany.HoldStartedAt = append(tooMany.HoldStartedAt, timestamppb.New(mocVN(t, "2026-10-02 16:00").Add(time.Duration(i)*time.Minute)))
	}
	cases := map[string]*identityv1.ResolveUnassignedHoldInstantsRequest{
		"no work_kind":      {HoldStartedAt: []*timestamppb.Timestamp{timestamppb.New(mocVN(t, "2026-10-02 16:00"))}},
		"unknown work_kind": {WorkKind: 99, HoldStartedAt: []*timestamppb.Timestamp{timestamppb.New(mocVN(t, "2026-10-02 16:00"))}},
		"empty":             holdReq(t),
		"over 500":          tooMany,
		"nil entry":         {WorkKind: identityv1.WorkKind_WORK_KIND_DON_THU, HoldStartedAt: []*timestamppb.Timestamp{nil}},
		"invalid":           {WorkKind: identityv1.WorkKind_WORK_KIND_DON_THU, HoldStartedAt: []*timestamppb.Timestamp{{Seconds: 1, Nanos: -1}}},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			// A NULL row: a malformed call is refused even when the answer would be "disabled".
			sla := &slaGia{ds: []domain.DongSLA{holdRow(0)}}
			s, _ := may(t, func(d *Deps) { d.SLA = sla })
			if _, err := s.ResolveUnassignedHoldInstants(ctxXa(xaA), req); status.Code(err) != codes.InvalidArgument {
				t.Fatalf("code = %v, want InvalidArgument", status.Code(err))
			}
			if sla.soLanGoi != 0 {
				t.Error("store read for a misshapen call")
			}
		})
	}
}

// Nothing configured, a bad value, a calendar that cannot count: FAILED_PRECONDITION — never a default.
// A store outage is Internal — never "disabled" or "nothing to report".
func TestUnassignedHoldConfigurationFaultsAreFailedPrecondition(t *testing.T) {
	for name, sua := range map[string]func(*Deps){
		"empty sla":      func(d *Deps) { d.SLA = &slaGia{} },
		"bad value":      func(d *Deps) { d.SLA = &slaGia{ds: []domain.DongSLA{holdRow(TranGioMotMoc + 1)}} },
		"empty calendar": func(d *Deps) { d.SLA = &slaGia{ds: []domain.DongSLA{holdRow(3)}}; d.Lich = &lichGia{} },
	} {
		s, _ := may(t, sua)
		if _, err := s.ResolveUnassignedHoldInstants(ctxXa(xaA), holdReq(t, "2026-10-02 16:00")); status.Code(err) != codes.FailedPrecondition {
			t.Errorf("%s: code = %v, want FailedPrecondition", name, status.Code(err))
		}
	}
	s, _ := may(t, func(d *Deps) { d.SLA = &slaGia{err: loiKho} })
	if _, err := s.ResolveUnassignedHoldInstants(ctxXa(xaA), holdReq(t, "2026-10-02 16:00")); status.Code(err) != codes.Internal {
		t.Errorf("store outage: code = %v, want Internal", status.Code(err))
	}
}
