package grpc

// What these tests defend: the commune's OWN `gio_sap_den_han` decides the answer, a commune that
// configured nothing is refused rather than given 72, and each fault gets the code that sends the
// right person to the right place. The walk itself is defended in internal/domain.

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// taskRow is a default `nhiem-vu` row whose five hour columns all DIFFER, so reading the wrong
// column produces a different instant.
func taskRow(dueSoon int) domain.DongSLA {
	return domain.DongSLA{
		ID: "sla-nv", LoaiViec: domain.LoaiViecNhiemVu,
		GioTiepNhan: 2, GioXuLyXong: 16, GioSapDenHan: dueSoon, GioBaoLanhDao: 5, GioBaoChuTich: 7,
	}
}

func dueSoonReq(t *testing.T, asOf string) *identityv1.ResolveDueSoonCutoffRequest {
	t.Helper()
	return &identityv1.ResolveDueSoonCutoffRequest{
		WorkKind: identityv1.WorkKind_WORK_KIND_NHIEM_VU,
		AsOf:     timestamppb.New(mocVN(t, asOf)),
	}
}

// The commune's number, from the right column, with the LATEST-instant rule: 3 working hours from
// 08:30 Monday end at 11:30 exactly, so the cutoff is the afternoon opening, 13:30. (Column 2 would
// give 10:30; column 16 a day later.)
func TestDueSoonUsesCommuneThresholdColumn(t *testing.T) {
	s, _ := may(t, func(d *Deps) { d.SLA = &slaGia{ds: []domain.DongSLA{taskRow(3)}} })

	ra, err := s.ResolveDueSoonCutoff(ctxXa(xaA), dueSoonReq(t, "2026-09-21 08:30"))
	if err != nil {
		t.Fatalf("ResolveDueSoonCutoff: %v", err)
	}
	if got, want := ra.GetDueSoonUntil().AsTime(), mocVN(t, "2026-09-21 13:30"); !got.Equal(want) {
		t.Fatalf("due_soon_until = %s, muốn %s", got, want)
	}
}

// TODAY'S STATE OF EVERY COMMUNE: nothing configured. Refused — never 72, never an empty field.
func TestDueSoonEmptySLAIsFailedPrecondition(t *testing.T) {
	s, _ := may(t, func(d *Deps) { d.SLA = &slaGia{} })
	ra, err := s.ResolveDueSoonCutoff(ctxXa(xaA), dueSoonReq(t, "2026-09-21 08:30"))
	if status.Code(err) != codes.FailedPrecondition || ra != nil {
		t.Fatalf("mã = %v, phản hồi = %v; muốn FailedPrecondition và không phản hồi", status.Code(err), ra)
	}
}

// Another kind of work being configured lends `nhiem-vu` nothing.
func TestDueSoonDoesNotBorrowAnotherWorkKind(t *testing.T) {
	petitionRow := dongSLA("sla-pa", domain.LoaiViecPhanAnh, "", 2, 16)
	s, _ := may(t, func(d *Deps) { d.SLA = &slaGia{ds: []domain.DongSLA{petitionRow}} })
	_, err := s.ResolveDueSoonCutoff(ctxXa(xaA), dueSoonReq(t, "2026-09-21 08:30"))
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("mã = %v, muốn FailedPrecondition", status.Code(err))
	}
}

// The number came from the commune's table, so a bad one is the commune's to fix.
func TestDueSoonBadThresholdIsFailedPrecondition(t *testing.T) {
	for _, h := range []int{0, -1, TranGioMotMoc + 1} {
		s, _ := may(t, func(d *Deps) { d.SLA = &slaGia{ds: []domain.DongSLA{taskRow(h)}} })
		_, err := s.ResolveDueSoonCutoff(ctxXa(xaA), dueSoonReq(t, "2026-09-21 08:30"))
		if status.Code(err) != codes.FailedPrecondition {
			t.Errorf("%d giờ: mã = %v, muốn FailedPrecondition", h, status.Code(err))
		}
	}
}

// Caller faults are INVALID_ARGUMENT and read nothing.
func TestDueSoonCallerFaultsReadNothing(t *testing.T) {
	cases := map[string]*identityv1.ResolveDueSoonCutoffRequest{
		"thiếu work_kind": {AsOf: timestamppb.New(mocVN(t, "2026-09-21 08:30"))},
		"thiếu as_of":     {WorkKind: identityv1.WorkKind_WORK_KIND_NHIEM_VU},
		"as_of hỏng": {WorkKind: identityv1.WorkKind_WORK_KIND_NHIEM_VU,
			AsOf: &timestamppb.Timestamp{Seconds: 1, Nanos: -1}},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			sla := &slaGia{ds: []domain.DongSLA{taskRow(3)}}
			lich := &lichGia{cas: tuanGia()}
			s, _ := may(t, func(d *Deps) { d.SLA = sla; d.Lich = lich })
			_, err := s.ResolveDueSoonCutoff(ctxXa(xaA), req)
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("mã = %v, muốn InvalidArgument", status.Code(err))
			}
			if sla.soLanGoi != 0 || lich.soLanGoi != 0 {
				t.Errorf("đã đọc kho (sla %d, lịch %d) cho một lời gọi sai hình dạng", sla.soLanGoi, lich.soLanGoi)
			}
		})
	}
}

// A calendar that cannot count is the commune's configuration, not an outage.
func TestDueSoonEmptyCalendarIsFailedPrecondition(t *testing.T) {
	s, _ := may(t, func(d *Deps) {
		d.SLA = &slaGia{ds: []domain.DongSLA{taskRow(3)}}
		d.Lich = &lichGia{}
	})
	_, err := s.ResolveDueSoonCutoff(ctxXa(xaA), dueSoonReq(t, "2026-09-21 08:30"))
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("mã = %v, muốn FailedPrecondition", status.Code(err))
	}
}

// An outage is NOT "nothing is due soon".
func TestDueSoonStoreFailureIsInternal(t *testing.T) {
	s, _ := may(t, func(d *Deps) { d.SLA = &slaGia{err: errors.New("mất kết nối")} })
	ra, err := s.ResolveDueSoonCutoff(ctxXa(xaA), dueSoonReq(t, "2026-09-21 08:30"))
	if status.Code(err) != codes.Internal || ra != nil {
		t.Fatalf("mã = %v, phản hồi = %v; muốn Internal", status.Code(err), ra)
	}
}

func TestDueSoonWithoutCommuneIsInternalNotPanic(t *testing.T) {
	s, _ := may(t, func(d *Deps) { d.SLA = &slaGia{ds: []domain.DongSLA{taskRow(3)}} })
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("handler panic khi thiếu xã: %v", r)
		}
	}()
	_, err := s.ResolveDueSoonCutoff(context.Background(), dueSoonReq(t, "2026-09-21 08:30"))
	if status.Code(err) != codes.Internal {
		t.Fatalf("mã = %v, muốn Internal", status.Code(err))
	}
}
