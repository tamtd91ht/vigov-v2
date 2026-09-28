package app

import (
	"context"
	"errors"
	"testing"

	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// Tests for the unit check (ResolveLiveOrgUnits, user decision 28/09/2026) on the two acts that write
// a unit id: task creation and the assignment act.
//
//	PROVED HERE   both acts ask identity about exactly the non-empty ids they would write, in the
//	              context's commune, BEFORE any transaction · an id identity does not answer live is
//	              ONE refusal with nothing written · identity down / not wired is "unchecked" (503),
//	              never a silent write · an act naming no unit asks nothing.

type orgUnitsFake struct {
	allLive bool
	live    map[string]struct{}
	err     error

	calls int
	asked [][]string
	xa    []tenant.ID
}

func (f *orgUnitsFake) LiveOrgUnits(ctx context.Context, ids []string) (map[string]struct{}, error) {
	f.calls++
	f.asked = append(f.asked, append([]string(nil), ids...))
	f.xa = append(f.xa, tenant.MustFrom(ctx))
	if f.err != nil {
		return nil, f.err
	}
	out := map[string]struct{}{}
	for _, id := range ids {
		if _, ok := f.live[id]; ok || f.allLive {
			out[id] = struct{}{}
		}
	}
	return out, nil
}

func TestCreateTask_ChecksUnitAndLeadUnitBeforeTransaction(t *testing.T) {
	k := khoNVMau()
	k.soLonNhat = 18
	uc, ctx := dungGhiNhiemVu(t, k)
	f := &orgUnitsFake{allLive: true}
	uc.orgUnits = f

	yc := taoMau()
	yc.BoPhanID, yc.CoQuanChuTriID = "bp-dia-chinh", "bp-vpdu"
	if _, err := uc.Tao(ctx, yc, canBoThu()); err != nil {
		t.Fatalf("giao việc: %v", err)
	}
	if f.calls != 1 || len(f.asked[0]) != 2 || f.asked[0][0] != "bp-dia-chinh" || f.asked[0][1] != "bp-vpdu" ||
		f.xa[0] != xaThu {
		t.Errorf("identity được hỏi %v trong xã %v", f.asked, f.xa)
	}
}

func TestCreateTask_UnitNotLiveRefusedNothingOpened(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	uc.orgUnits = &orgUnitsFake{live: map[string]struct{}{"bp-vpdu": {}}}

	yc := taoMau()
	yc.BoPhanID, yc.CoQuanChuTriID = "bp-cua-xa-khac", "bp-vpdu"
	_, err := uc.Tao(ctx, yc, canBoThu())
	if !errors.Is(err, ErrOrgUnitNotLive) {
		t.Fatalf("lỗi = %v, muốn ErrOrgUnitNotLive", err)
	}
	if k.batDau != 0 {
		t.Errorf("mở %d giao dịch cho một bộ phận bị từ chối", k.batDau)
	}
	khongGhiGi(t, k)
}

func TestCreateTask_IdentityDownIsUnchecked(t *testing.T) {
	for name, f := range map[string]*orgUnitsFake{
		"identity lỗi": {err: errors.New("unavailable")},
	} {
		t.Run(name, func(t *testing.T) {
			k := khoNVMau()
			uc, ctx := dungGhiNhiemVu(t, k)
			uc.orgUnits = f
			yc := taoMau()
			yc.BoPhanID = "bp-dia-chinh"
			if _, err := uc.Tao(ctx, yc, canBoThu()); !errors.Is(err, ErrOrgUnitUnchecked) {
				t.Fatalf("lỗi = %v, muốn ErrOrgUnitUnchecked", err)
			}
			khongGhiGi(t, k)
		})
	}
	// NOT WIRED fails closed too.
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	uc.orgUnits = nil
	yc := taoMau()
	yc.BoPhanID = "bp-dia-chinh"
	if _, err := uc.Tao(ctx, yc, canBoThu()); !errors.Is(err, ErrOrgUnitUnchecked) {
		t.Fatalf("chưa nối dây: lỗi = %v, muốn ErrOrgUnitUnchecked", err)
	}
	khongGhiGi(t, k)
}

func TestCreateTask_NoUnitAsksNothing(t *testing.T) {
	k := khoNVMau()
	k.soLonNhat = 18
	uc, ctx := dungGhiNhiemVu(t, k)
	f := &orgUnitsFake{}
	uc.orgUnits = f
	yc := taoMau()
	yc.BoPhanID, yc.CoQuanChuTriID = "", ""
	if _, err := uc.Tao(ctx, yc, canBoThu()); err != nil {
		t.Fatalf("giao việc không bộ phận: %v", err)
	}
	if f.calls != 0 {
		t.Errorf("hỏi identity %d lần dù không có bộ phận nào", f.calls)
	}
}

func TestReassign_UnitNotLiveRefusedWritesNothing(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	f := &orgUnitsFake{}
	uc.orgUnits = f

	_, err := uc.Reassign(ctx, maNVGoc, reassignReq(domain.TaskAssignmentChange{
		Unit: unitPtr("bp-cua-xa-khac"), LeadUnit: unitPtr("bp-khac")}), canBoThu())
	if !errors.Is(err, ErrOrgUnitNotLive) {
		t.Fatalf("lỗi = %v, muốn ErrOrgUnitNotLive", err)
	}
	if f.calls != 1 || len(f.asked[0]) != 2 {
		t.Errorf("identity được hỏi %v", f.asked)
	}
	if k.batDau != 0 {
		t.Errorf("mở %d giao dịch", k.batDau)
	}
	khongGhiGi(t, k)
}

func TestReassign_IdentityDownIsUnchecked(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	uc.orgUnits = &orgUnitsFake{err: errors.New("unavailable")}

	_, err := uc.Reassign(ctx, maNVGoc, reassignReq(domain.TaskAssignmentChange{Unit: unitPtr("bp-dia-chinh")}),
		canBoThu())
	if !errors.Is(err, ErrOrgUnitUnchecked) {
		t.Fatalf("lỗi = %v, muốn ErrOrgUnitUnchecked", err)
	}
	khongGhiGi(t, k)
}

// TestReassign_MonitorOnlyAsksNoUnit: a change naming no unit column does not call identity for units.
func TestReassign_MonitorOnlyAsksNoUnit(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	uc.giaoViec = &giaoViecGia{duocTatCa: true}
	f := &orgUnitsFake{}
	uc.orgUnits = f

	if _, err := uc.Reassign(ctx, maNVGoc, reassignReq(domain.TaskAssignmentChange{Monitor: unitPtr("CB-00500")}),
		canBoThu()); err != nil {
		t.Fatalf("đổi chuyên viên theo dõi: %v", err)
	}
	if f.calls != 0 {
		t.Errorf("hỏi identity về bộ phận %d lần dù không đổi bộ phận", f.calls)
	}
}
