package app

import (
	"errors"
	"testing"

	"github.com/vihat/vigov/service-identity/internal/domain"
)

// DanhBaCanBo.Them with `role_ids` (owner decision 10/10/2026, bug sheet row 41) — over the fake
// driver of danh_ba_can_bo_test.go, so "the role lands in the same transaction as the person" is an
// assertion about which statements ran in which transaction.
//
// Every refusal here fails SILENTLY if its guard is removed: the person is created holding a role,
// and the trail records an ordinary creation.

// THE ROLE IS ASSIGNED IN THE SAME TRANSACTION AS THE INSERT, AND AUDITED AS ITS OWN ACT — the verb
// and the delta of PUT /api/v1/staff/{id}/role — with the STAFF CODE as actor (rule 6, invariant 8).
//
// MUTATIONS THAT MUST TURN THIS RED:
//   - assign the role in a second transaction, or after the commit
//   - drop the doi_vai_tro_can_bo entry (rule 5, invariant 5)
func TestCreateStaffWithRoleAssignsInSameTransaction(t *testing.T) {
	b := dungBanThuDanhBa(t)
	yc := yeuCauThemGia()
	yc.RoleIDs = []string{vaiTroYeu}

	cb, err := b.uc.Them(ctxXa(xaThu), yc, nguoiThucHienGia())
	if err != nil {
		t.Fatalf("Them: %v", err)
	}
	if cb.VaiTroID != vaiTroYeu {
		t.Errorf("vai trò trả về = %q, muốn %q", cb.VaiTroID, vaiTroYeu)
	}

	insert := b.ghi.tim("dau-hieu-chen")
	role := b.ghi.tim("SET vai_tro_id")
	if insert == nil || role == nil || insert.tx != role.tx {
		t.Fatal("người mới và vai trò không ghi trong CÙNG một giao dịch")
	}
	entries := vetDaGhi(b.ghi)
	if len(entries) != 2 {
		t.Fatalf("có %d vết, muốn 2 (them_can_bo + doi_vai_tro_can_bo)", len(entries))
	}
	for i, want := range []string{HanhViThemCanBo, HanhViDoiVaiTro} {
		e := entries[i]
		if e.tx != insert.tx {
			t.Errorf("vết %s ở giao dịch %d, bản ghi ở %d — luật 6 bất biến 3", want, e.tx, insert.tx)
		}
		if got := chuoiArg(t, e, viTriHanhVi); got != want {
			t.Errorf("vết %d: action = %q, muốn %q", i, got, want)
		}
		if got := chuoiArg(t, e, viTriActor); got != maCanBo {
			t.Errorf("vết %d: actor_id = %q, muốn MÃ CÁN BỘ %q", i, got, maCanBo)
		}
		if got := chuoiArg(t, e, viTriChuThe); got != cb.Ma {
			t.Errorf("vết %d: subject = %q, muốn %q", i, got, cb.Ma)
		}
	}
	sau, _ := deltaCua(t, entries[1])["sau"].(map[string]any)
	if sau["vai_tro_id"] != vaiTroYeu {
		t.Errorf("delta đổi vai trò = %v", sau)
	}
	if ket := b.ghi.ketThucCua(insert.tx); ket != "commit" {
		t.Fatalf("giao dịch kết thúc bằng %q, muốn commit", ket)
	}
}

// NO ROLE = THE OLD BEHAVIOUR: one entry, no UPDATE of vai_tro_id.
func TestCreateStaffWithoutRoleWritesNoRole(t *testing.T) {
	b := dungBanThuDanhBa(t)
	yc := yeuCauThemGia()
	yc.RoleIDs = []string{""}

	cb, err := b.uc.Them(ctxXa(xaThu), yc, nguoiThucHienGia())
	if err != nil {
		t.Fatalf("Them: %v", err)
	}
	if cb.VaiTroID != "" || b.ghi.tim("SET vai_tro_id") != nil {
		t.Error("không chọn vai trò mà vẫn gán")
	}
	motVet(t, b.ghi)
}

// A ROLE THAT IS NOT THIS COMMUNE'S — another commune's id, an invented one, a soft-deleted one — is
// one refusal, and NOTHING is written: not the person, not the role, not an entry.
//
// MUTATION THAT MUST TURN THIS RED: skip the `exists` check — the FK would refuse another commune's
// role, but a soft-deleted one of this commune would be assigned and grant nothing.
func TestCreateStaffWithUnknownRoleWritesNothing(t *testing.T) {
	b := dungBanThuDanhBa(t)
	yc := yeuCauThemGia()
	yc.RoleIDs = []string{"vt-xa-khac"}

	if _, err := b.uc.Them(ctxXa(xaThu), yc, nguoiThucHienGia()); !errors.Is(err, ErrVaiTroKhongTonTai) {
		t.Fatalf("lỗi = %v, muốn ErrVaiTroKhongTonTai", err)
	}
	khongCoGhi(t, b.ghi)
}

// #14, SECOND CONSTRAINT, ON THE CREATE FORM TOO: a role carrying a key the actor does not hold is
// refused, and nothing is written. Without it the create form is the way around PUT .../role.
//
// MUTATION THAT MUST TURN THIS RED: delete the khongCam check in Them.
func TestCreateStaffWithStrongerRoleIsRefused(t *testing.T) {
	b := dungBanThuDanhBa(t)
	b.kho.quyenToi = []string{"admin.user", "document.read"}
	yc := yeuCauThemGia()
	yc.RoleIDs = []string{vaiTroManh}

	_, err := b.uc.Them(ctxXa(xaThu), yc, nguoiThucHienGia())
	var missing *LoiTraoQuyenKhongCam
	if !errors.As(err, &missing) || len(missing.Thieu) != 1 || missing.Thieu[0] != "budget.confirm" {
		t.Fatalf("lỗi = %v, muốn LoiTraoQuyenKhongCam[budget.confirm]", err)
	}
	khongCoGhi(t, b.ghi)
}

// TWO DISTINCT ROLES ARE REFUSED BEFORE ANY TRANSACTION — one role per person.
func TestCreateStaffWithTwoRolesOpensNoTransaction(t *testing.T) {
	b := dungBanThuDanhBa(t)
	yc := yeuCauThemGia()
	yc.RoleIDs = []string{vaiTroYeu, vaiTroManh}

	if _, err := b.uc.Them(ctxXa(xaThu), yc, nguoiThucHienGia()); !errors.Is(err, domain.ErrOneRolePerStaff) {
		t.Fatalf("lỗi = %v, muốn ErrOneRolePerStaff", err)
	}
	if n := b.ghi.soGiaoDich(); n != 0 {
		t.Errorf("mở %d giao dịch", n)
	}
}

// THE PERSONAL MOBILE IS AT MOST TEN DIGITS (owner decision 10/10/2026) on the create form AND the
// edit, refused before any transaction.
func TestStaffMobileOverTenDigitsRefused(t *testing.T) {
	b := dungBanThuDanhBa(t)
	yc := yeuCauThemGia()
	yc.DiDongCaNhan = "09000000001"
	if _, err := b.uc.Them(ctxXa(xaThu), yc, nguoiThucHienGia()); !errors.Is(err, domain.ErrMobileTooManyDigits) {
		t.Fatalf("thêm: lỗi = %v, muốn ErrMobileTooManyDigits", err)
	}

	mobile := "+84 900 000 0001"
	if _, err := b.uc.Sua(ctxXa(xaThu), idNguoiKhac, YeuCauSuaCanBo{DiDongCaNhan: &mobile}, nguoiThucHienGia()); !errors.Is(err, domain.ErrMobileTooManyDigits) {
		t.Fatalf("sửa: lỗi = %v, muốn ErrMobileTooManyDigits", err)
	}
	if n := b.ghi.soGiaoDich(); n != 0 {
		t.Errorf("mở %d giao dịch cho một số di động sai", n)
	}
}
