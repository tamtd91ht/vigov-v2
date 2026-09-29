package app

import (
	"errors"
	"testing"

	"github.com/vihat/vigov/service-identity/internal/domain"
)

// THE WORK ADDRESS IS OPTIONAL ON THE DIRECTORY ROW AND MANDATORY ON AN ACCOUNT (ADR 0059 §1, user
// decision 2026-09-29). Both halves fail silently when broken: a create that refuses a blank address
// only annoys, but an edit that clears an account holder's address locks them out of sign-in with
// no lock recorded and no screen able to say why.

// A BLANK ADDRESS ON CREATE IS NO ADDRESS: the row reaches the store with "" (which the INSERT
// writes as NULL — nullif($5,”), store/can_bo_ghi.go), the trail is written in the same
// transaction, and it commits.
func TestStaffCreateBlankEmailIsStoredAsNone(t *testing.T) {
	for _, blank := range []string{"", "   "} {
		b := dungBanThuDanhBa(t)
		req := yeuCauThemGia()
		req.Email = blank

		cb, err := b.uc.Them(ctxXa(xaThu), req, nguoiThucHienGia())
		if err != nil {
			t.Fatalf("%q: Them: %v — thư điện tử trống phải được nhận (ADR 0059 §1)", blank, err)
		}
		if cb.Email != "" || b.kho.insertedRow == nil || b.kho.insertedRow.Email != "" {
			t.Fatalf("%q: email tới kho = %+v, muốn chuỗi rỗng (lưu NULL)", blank, b.kho.insertedRow)
		}
		vet := motVet(t, b.ghi)
		if ket := b.ghi.ketThucCua(vet.tx); ket != "commit" {
			t.Fatalf("%q: giao dịch kết thúc bằng %q, muốn commit", blank, ket)
		}
	}
}

// A NON-BLANK ADDRESS IS STILL CHECKED, before any transaction opens.
func TestStaffCreateInvalidEmailStillRefused(t *testing.T) {
	b := dungBanThuDanhBa(t)
	req := yeuCauThemGia()
	req.Email = "khong-co-a-cong"

	if _, err := b.uc.Them(ctxXa(xaThu), req, nguoiThucHienGia()); !errors.Is(err, domain.ErrEmailSaiDinhDang) {
		t.Fatalf("lỗi = %v, muốn ErrEmailSaiDinhDang", err)
	}
	if n := b.ghi.soGiaoDich(); n != 0 {
		t.Fatalf("mở %d giao dịch cho một thư điện tử sai định dạng", n)
	}
}

// CLEARING THE ADDRESS OF AN ACCOUNT HOLDER IS REFUSED AND WRITES NOTHING. The fixture row has an
// account (canBoGia: CoTaiKhoan true) and an address.
//
// MUTATION THAT MUST TURN THIS RED: delete the ErrStaffEmailIsLogin check in Sua.
func TestStaffEditClearEmailWithAccountRefused(t *testing.T) {
	b := dungBanThuDanhBa(t)
	blank := ""

	_, err := b.uc.Sua(ctxXa(xaThu), idNguoiKhac, YeuCauSuaCanBo{Email: &blank}, nguoiThucHienGia())
	if !errors.Is(err, ErrStaffEmailIsLogin) {
		t.Fatalf("lỗi = %v, muốn ErrStaffEmailIsLogin", err)
	}
	khongCoGhi(t, b.ghi)
	if b.kho.updatedRow != nil {
		t.Error("bị từ chối mà kho vẫn nhận lệnh cập nhật hồ sơ")
	}
	if ket := b.ghi.ketThucCua(1); ket != "rollback" {
		t.Fatalf("giao dịch kết thúc bằng %q, muốn rollback", ket)
	}
}

// WITHOUT AN ACCOUNT, CLEARING IS AN ORDINARY EDIT: one UPDATE carrying "", one entry whose delta
// names the column with its before and after — the address unmasked, as tomTatDoiHoSo records a
// work address on every edit.
func TestStaffEditClearEmailWithoutAccountAllowed(t *testing.T) {
	b := dungBanThuDanhBa(t)
	b.kho.cb.CoTaiKhoan = false
	old := b.kho.cb.Email
	blank := "  "

	cb, err := b.uc.Sua(ctxXa(xaThu), idNguoiKhac, YeuCauSuaCanBo{Email: &blank}, nguoiThucHienGia())
	if err != nil {
		t.Fatalf("Sua: %v", err)
	}
	if cb.Email != "" || b.kho.updatedRow == nil || b.kho.updatedRow.Email != "" {
		t.Fatalf("email sau = %q, tới kho = %+v, muốn rỗng", cb.Email, b.kho.updatedRow)
	}
	vet := motVet(t, b.ghi)
	if got := chuoiArg(t, vet, viTriHanhVi); got != HanhViSuaCanBo {
		t.Errorf("action = %q, muốn %q", got, HanhViSuaCanBo)
	}
	d := deltaCua(t, vet)
	before, _ := d["truoc"].(map[string]any)
	after, _ := d["sau"].(map[string]any)
	if before["email"] != old || after["email"] != "" {
		t.Errorf("vết email: trước=%v sau=%v, muốn %q → \"\"", before["email"], after["email"], old)
	}
	if _, co := after["ho_ten"]; co {
		t.Errorf("vết mang trường không đổi: %v", after)
	}
}

// REPLACING AN ACCOUNT HOLDER'S ADDRESS WITH ANOTHER VALID ONE IS UNCHANGED BEHAVIOUR: allowed, and
// it changes their login name. Pinned so that the refusal above cannot quietly widen into it.
func TestStaffEditReplaceEmailWithAccountStillAllowed(t *testing.T) {
	b := dungBanThuDanhBa(t)
	next := "canbo.moi@example.gov.vn"

	cb, err := b.uc.Sua(ctxXa(xaThu), idNguoiKhac, YeuCauSuaCanBo{Email: &next}, nguoiThucHienGia())
	if err != nil {
		t.Fatalf("Sua: %v", err)
	}
	if cb.Email != next {
		t.Errorf("email sau = %q, muốn %q", cb.Email, next)
	}
	motVet(t, b.ghi)
}
