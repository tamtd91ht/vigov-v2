package app

import (
	"errors"
	"strings"
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

// TWO DISTINCT ROLES ARE BOTH ASSIGNED (owner decision 13/10/2026, reversing 08/10): the whole set
// reaches the store in the insert's transaction, the first is the primary role, and the entry records
// the set — actor = STAFF CODE (rule 6, invariant 8).
//
// MUTATIONS THAT MUST TURN THIS RED: trim to the first role; put back the "one role" refusal; drop
// `vai_tro_ids` from the delta.
func TestCreateStaffWithTwoRolesAssignsBoth(t *testing.T) {
	b := dungBanThuDanhBa(t)
	yc := yeuCauThemGia()
	yc.RoleIDs = []string{vaiTroManh, vaiTroYeu, vaiTroManh}

	cb, err := b.uc.Them(ctxXa(xaThu), yc, nguoiThucHienGia())
	if err != nil {
		t.Fatalf("Them: %v", err)
	}
	if got := strings.Join(b.kho.setRoles, ","); got != vaiTroManh+","+vaiTroYeu {
		t.Fatalf("tập vai trò ghi = %q, muốn hai vai trò theo thứ tự gửi", got)
	}
	if b.kho.setRolesBy != maCanBo {
		t.Errorf("created_by = %q, muốn MÃ CÁN BỘ %q", b.kho.setRolesBy, maCanBo)
	}
	if cb.VaiTroID != vaiTroManh || strings.Join(cb.RoleIDs, ",") != vaiTroManh+","+vaiTroYeu {
		t.Errorf("trả về vai trò chính %q, tập %v", cb.VaiTroID, cb.RoleIDs)
	}
	insert, role := b.ghi.tim("dau-hieu-chen"), b.ghi.tim("SET vai_tro_id")
	if insert == nil || role == nil || insert.tx != role.tx {
		t.Fatal("người mới và các vai trò không ghi trong CÙNG một giao dịch")
	}
	entries := vetDaGhi(b.ghi)
	if len(entries) != 2 {
		t.Fatalf("có %d vết, muốn 2", len(entries))
	}
	sau, _ := deltaCua(t, entries[1])["sau"].(map[string]any)
	ids, _ := sau["vai_tro_ids"].([]any)
	if len(ids) != 2 || ids[0] != vaiTroManh || ids[1] != vaiTroYeu || sau["vai_tro_id"] != vaiTroManh {
		t.Errorf("delta sau = %v, muốn vai_tro_id chính + vai_tro_ids đủ hai", sau)
	}
}

// #14 ON EVERY ROLE OF THE SET: a weak role first and a strong one second is still a grant of the
// strong one. Nothing is written.
//
// MUTATION THAT MUST TURN THIS RED: check only the first (or primary) role of the set.
func TestCreateStaffEscalationThroughSecondRoleIsRefused(t *testing.T) {
	b := dungBanThuDanhBa(t)
	b.kho.quyenToi = []string{"admin.user", "document.read"}
	yc := yeuCauThemGia()
	yc.RoleIDs = []string{vaiTroYeu, vaiTroManh}

	_, err := b.uc.Them(ctxXa(xaThu), yc, nguoiThucHienGia())
	var missing *LoiTraoQuyenKhongCam
	if !errors.As(err, &missing) || strings.Join(missing.Thieu, ",") != "budget.confirm" {
		t.Fatalf("lỗi = %v, muốn LoiTraoQuyenKhongCam[budget.confirm]", err)
	}
	khongCoGhi(t, b.ghi)
}

// A SET LONGER THAN ANY CATALOGUE is refused before any transaction.
func TestCreateStaffWithTooManyRolesOpensNoTransaction(t *testing.T) {
	b := dungBanThuDanhBa(t)
	yc := yeuCauThemGia()
	for i := 0; i <= domain.MaxStaffRoles; i++ {
		yc.RoleIDs = append(yc.RoleIDs, "vt-"+strings.Repeat("x", i%20)+string(rune('a'+i%26)))
	}
	if _, err := b.uc.Them(ctxXa(xaThu), yc, nguoiThucHienGia()); !errors.Is(err, domain.ErrTooManyStaffRoles) {
		t.Fatalf("lỗi = %v, muốn ErrTooManyStaffRoles", err)
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
