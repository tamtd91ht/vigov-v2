package app

import (
	"errors"
	"testing"

	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// The holder rule on the status route (vigov-require a37ec96, user decision 28/09/2026), over the
// real store on the fake driver. The fixture task is `dang-thuc-hien`, assignee CB-00311, monitor
// CB-00412, assigner CB-00007, author CB-00123.

// TestStatus_AssigneeWithoutTaskUpdateMayMove — the case a37ec96 was written for: the specialist
// handed the task moves it without the commune-wide edit key.
func TestStatus_AssigneeWithoutTaskUpdateMayMove(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)

	sau, err := uc.DoiTrangThai(ctx, maNVGoc, YeuCauDoiTrangThai{TrangThai: string(domain.ChoDuyet)},
		staffActor(maNguoiThucHien), false, false)
	if err != nil {
		t.Fatalf("người thực hiện chuyển trạng thái: %v", err)
	}
	if sau.TrangThai != domain.ChoDuyet {
		t.Errorf("trạng thái sau = %s", sau.TrangThai)
	}
	chiGhiTrongGiaoDich(t, k)
}

// TestStatus_NonHolderRefused: an officer who only reads the register, and each "log-only" related
// person, is refused with nothing written.
func TestStatus_NonHolderRefused(t *testing.T) {
	for name, code := range map[string]string{
		"chỉ có task.read":     outsiderCode,
		"chuyên viên theo dõi": "CB-00412",
		"lãnh đạo giao việc":   maLanhDao,
		"người tạo":            "CB-00123",
	} {
		t.Run(name, func(t *testing.T) {
			k := khoNVMau()
			uc, ctx := dungGhiNhiemVu(t, k)
			_, err := uc.DoiTrangThai(ctx, maNVGoc, YeuCauDoiTrangThai{TrangThai: string(domain.ChoDuyet)},
				staffActor(code), true, false)
			if !errors.Is(err, domain.ErrStatusNeedsHolder) {
				t.Fatalf("lỗi = %v, muốn ErrStatusNeedsHolder", err)
			}
			khongGhiGi(t, k)
		})
	}
}

// TestStatus_TaskUpdateHolderMayMoveAnyTask: the commune-wide key is a37ec96's other "full" case.
func TestStatus_TaskUpdateHolderMayMoveAnyTask(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	if _, err := uc.DoiTrangThai(ctx, maNVGoc, YeuCauDoiTrangThai{TrangThai: string(domain.ChoDuyet)},
		staffActor(outsiderCode), false, true); err != nil {
		t.Fatalf("cán bộ có task.update chuyển trạng thái: %v", err)
	}
}

// TestStatus_AssigneeWithoutApproveCannotComplete: the holder rule does not replace `task.approve` —
// completing (even straight from dang-thuc-hien), reopening and returning still need it.
func TestStatus_AssigneeWithoutApproveCannotComplete(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	_, err := uc.DoiTrangThai(ctx, maNVGoc, YeuCauDoiTrangThai{TrangThai: string(domain.HoanThanh)},
		staffActor(maNguoiThucHien), false, false)
	if !errors.Is(err, ErrKhongDuocDuyetHoanThanh) {
		t.Fatalf("hoàn thành: lỗi = %v, muốn ErrKhongDuocDuyetHoanThanh", err)
	}
	khongGhiGi(t, k)

	k = khoNVHoanThanh()
	uc, ctx = dungGhiNhiemVu(t, k)
	_, err = uc.DoiTrangThai(ctx, maNVGoc, YeuCauDoiTrangThai{TrangThai: string(domain.DangThucHien)},
		staffActor(maNguoiThucHien), false, false)
	if !errors.Is(err, ErrReopenNeedsApproval) {
		t.Fatalf("mở lại: lỗi = %v, muốn ErrReopenNeedsApproval", err)
	}
	khongGhiGi(t, k)
}

// TestStatus_HolderCheckedBeforeShape: a caller who may not move the task is told so, not told which
// moves exist.
func TestStatus_HolderCheckedBeforeShape(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)
	_, err := uc.DoiTrangThai(ctx, maNVGoc, YeuCauDoiTrangThai{TrangThai: string(domain.MoiGiao)},
		staffActor(outsiderCode), true, false)
	if !errors.Is(err, domain.ErrStatusNeedsHolder) {
		t.Fatalf("lỗi = %v, muốn ErrStatusNeedsHolder trước lỗi vòng đời", err)
	}
}
