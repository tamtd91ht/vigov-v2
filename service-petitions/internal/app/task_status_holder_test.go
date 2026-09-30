package app

import (
	"errors"
	"testing"

	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// The holder rule on the status route (vigov-require a37ec96, user decision 28/09/2026), over the
// real store on the fake driver. The fixture task is `dang-thuc-hien`, assignee CB-00311, assigner
// CB-00007, author CB-00123 (no monitor since ADR 0065 NV5; the case below plants one in the retired
// column to prove it is not read).

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
		"chỉ có task.read":                   outsiderCode,
		"chuyên viên theo dõi (cột đã nghỉ)": "CB-00412",
		"lãnh đạo giao việc":                 maLanhDao,
		"người tạo":                          "CB-00123",
	} {
		t.Run(name, func(t *testing.T) {
			k := khoNVMau()
			k.nhiemVu[idNVGoc]["chuyen_vien_theo_doi_ma"] = "CB-00412" // a pre-0025 row; never read
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

// TestStatus_AssigneeWithoutApproveCompletesDirectly (ADR 0065 NV1, 30/09/2026): the assignee — no
// `task.update`, no `task.approve` — finishes the work straight from dang-thuc-hien. The act writes
// the status with `ngay_hoan_thanh`, one timeline row and one audit entry, all in the transaction.
//
// ĐỔI CHIỀU CÓ CHỦ Ý 30/09/2026: until the user made review optional this case was refused with
// ErrKhongDuocDuyetHoanThanh.
func TestStatus_AssigneeWithoutApproveCompletesDirectly(t *testing.T) {
	k := khoNVMau() // dang-thuc-hien
	uc, ctx := dungGhiNhiemVu(t, k)
	sau, err := uc.DoiTrangThai(ctx, maNVGoc, YeuCauDoiTrangThai{TrangThai: string(domain.HoanThanh)},
		staffActor(maNguoiThucHien), false, false)
	if err != nil {
		t.Fatalf("người thực hiện hoàn thành thẳng: %v", err)
	}
	if sau.TrangThai != domain.HoanThanh || sau.NgayHoanThanh != mocThaoTacNV {
		t.Errorf("sau = %s / %v", sau.TrangThai, sau.NgayHoanThanh)
	}
	if n := len(k.cau("INSERT INTO nhat_ky_nhiem_vu")); n != 1 {
		t.Errorf("ghi %d dòng nhật ký, muốn 1", n)
	}
	vet := vetKiemToan(t, k)
	if vet.args[1] != maNguoiThucHien || vet.args[4] != HanhViChuyenTrangNhiemVu {
		t.Errorf("vết = %v / %v", vet.args[1], vet.args[4])
	}
	chiGhiTrongGiaoDich(t, k)
}

// TestStatus_AssigneeWithoutApproveCannotLeaveReview: work already AT cho-duyet waits for a holder of
// `task.approve` — neither sign-off nor return is the assignee's (ADR 0065, answer #2 of 30/09) — and
// the reopen still needs the key too.
func TestStatus_AssigneeWithoutApproveCannotLeaveReview(t *testing.T) {
	k := khoNVMau()
	k.nhiemVu[idNVGoc]["trang_thai"] = string(domain.ChoDuyet)
	uc, ctx := dungGhiNhiemVu(t, k)
	_, err := uc.DoiTrangThai(ctx, maNVGoc, YeuCauDoiTrangThai{TrangThai: string(domain.HoanThanh)},
		staffActor(maNguoiThucHien), false, false)
	if !errors.Is(err, ErrKhongDuocDuyetHoanThanh) {
		t.Fatalf("duyệt từ cho-duyet: lỗi = %v, muốn ErrKhongDuocDuyetHoanThanh", err)
	}
	khongGhiGi(t, k)

	k = khoNVMau()
	k.nhiemVu[idNVGoc]["trang_thai"] = string(domain.ChoDuyet)
	uc, ctx = dungGhiNhiemVu(t, k)
	_, err = uc.DoiTrangThai(ctx, maNVGoc,
		YeuCauDoiTrangThai{TrangThai: string(domain.DangThucHien), GhiChu: lyDoTraLaiThu},
		staffActor(maNguoiThucHien), false, false)
	if !errors.Is(err, ErrKhongDuocTraLai) {
		t.Fatalf("trả lại: lỗi = %v, muốn ErrKhongDuocTraLai", err)
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

// TestStatus_ApproverSignsOffReview: a holder of `task.approve` (here without being the assignee, via
// task.update) signs off work at cho-duyet.
func TestStatus_ApproverSignsOffReview(t *testing.T) {
	k := khoNVMau()
	k.nhiemVu[idNVGoc]["trang_thai"] = string(domain.ChoDuyet)
	uc, ctx := dungGhiNhiemVu(t, k)
	sau, err := uc.DoiTrangThai(ctx, maNVGoc, YeuCauDoiTrangThai{TrangThai: string(domain.HoanThanh)},
		staffActor(outsiderCode), true, true)
	if err != nil {
		t.Fatalf("người duyệt duyệt hoàn thành: %v", err)
	}
	if sau.TrangThai != domain.HoanThanh {
		t.Errorf("trạng thái sau = %s", sau.TrangThai)
	}
	chiGhiTrongGiaoDich(t, k)
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
