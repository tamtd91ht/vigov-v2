package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/privacy"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// WHAT THIS FILE IS FOR: owner decision 2026-09-28 on #12 (ledger service-identity/
// che-so-di-dong-can-bo). A consent to publish was given FOR ONE NUMBER and FOR A PERSON IN POST, so
//
//	PATCH that changes `di_dong_ca_nhan` of a published person  → unpublish + clear marks + audit
//	lock of a published person                                   → unpublish + clear marks + audit
//	unlock                                                       → never republishes
//
// all in the transaction of the write that caused it. Every defect here is SILENT: the edit
// succeeds, the lock succeeds, and a number nobody consented to sits on a public channel.
//
// Same fake driver and fake store as danh_ba_can_bo_test.go; the fixture helpers daCongKhai and so
// come from danh_ba_mini_app_test.go.

// hanhViCacVet returns the action of every audit entry written, in order.
func hanhViCacVet(t *testing.T, b *banThuDanhBa) []string {
	t.Helper()
	var ra []string
	for _, l := range vetDaGhi(b.ghi) {
		ra = append(ra, chuoiArg(t, l, viTriHanhVi))
	}
	return ra
}

// daRutHet asserts the row carries no publication and no consent mark — what migration 0010 §3
// demands of an unpublished row, and what makes the next publication ask again.
func daRutHet(t *testing.T, ten string, cb domain.CanBoTomTat) {
	t.Helper()
	if cb.HienTrenMiniApp || cb.DongYCongKhaiLuc != nil || cb.DongYCongKhaiGhiBoi != "" {
		t.Errorf("%s: còn công khai hoặc còn dấu đồng ý (hien=%v, luc=%v, ghi_boi=%q)",
			ten, cb.HienTrenMiniApp, cb.DongYCongKhaiLuc, cb.DongYCongKhaiGhiBoi)
	}
}

// --- PATCH: the personal mobile --------------------------------------------------------------------

// THE CENTRAL CASE. Changed mobile on a published person: the profile UPDATE, the unpublish UPDATE
// and BOTH audit entries land in ONE committed transaction, the second entry carrying the editor's
// staff code, the reason code, and only masked numbers.
//
// MUTATION THAT MUST TURN THIS RED: delete the `truoc.HienTrenMiniApp && truoc.DiDongCaNhan !=
// sau.DiDongCaNhan` branch in Sua, or move rutCongKhaiTuDong into a second Tx.
func TestSuaDiDongNguoiDangCongKhaiThiRutVaXoaDauCungGiaoDich(t *testing.T) {
	b := dungBanThuDanhBa(t)
	daCongKhai(b, so(2))
	moi := diDongGiaSau

	cb, err := b.uc.Sua(ctxXa(xaThu), idNguoiKhac, YeuCauSuaCanBo{DiDongCaNhan: &moi}, nguoiThucHienGia())
	if err != nil {
		t.Fatalf("Sua: %v", err)
	}
	if cb.DiDongCaNhan != diDongGiaSau {
		t.Errorf("số di động không được sửa")
	}
	daRutHet(t, "kết quả trả về", cb)
	ghi := b.kho.congKhaiCuoi
	if ghi == nil {
		t.Fatal("đổi số di động của người đang công khai mà kho không được yêu cầu rút công khai")
	}
	daRutHet(t, "dòng ghi xuống", *ghi)
	if ghi.ThuTuDanhBa == nil || *ghi.ThuTuDanhBa != 2 {
		t.Errorf("thứ tự danh bạ bị đổi khi rút tự động: %v", ghi.ThuTuDanhBa)
	}

	hoSo, congKhai := b.ghi.tim("SET ho-so"), b.ghi.tim("SET cong-khai")
	if hoSo == nil || congKhai == nil {
		t.Fatal("thiếu câu UPDATE hồ sơ hoặc câu UPDATE công khai")
	}
	got := hanhViCacVet(t, b)
	if len(got) != 2 || got[0] != HanhViSuaCanBo || got[1] != HanhViRutCongKhaiMiniApp {
		t.Fatalf("vết = %v, muốn [%s %s]", got, HanhViSuaCanBo, HanhViRutCongKhaiMiniApp)
	}
	vets := vetDaGhi(b.ghi)
	for _, l := range append(vets, *congKhai) {
		if l.tx == 0 || l.tx != hoSo.tx {
			t.Fatalf("câu %q ở giao dịch %d, câu sửa hồ sơ ở %d — phải CÙNG một giao dịch (luật 6 bất biến 3)",
				l.sql, l.tx, hoSo.tx)
		}
	}
	if ket := b.ghi.ketThucCua(hoSo.tx); ket != "commit" {
		t.Fatalf("giao dịch kết thúc bằng %q, muốn commit", ket)
	}

	rut := vets[1]
	if a := chuoiArg(t, rut, viTriActor); a != maCanBo {
		t.Errorf("actor của vết rút tự động = %q, muốn mã cán bộ người sửa %q", a, maCanBo)
	}
	if s := chuoiArg(t, rut, viTriChuThe); s != maNguoiKhac {
		t.Errorf("subject = %q, muốn %q", s, maNguoiKhac)
	}
	for _, l := range vets {
		tho := string(l.args[viTriDelta].([]byte))
		if strings.Contains(tho, diDongGia) || strings.Contains(tho, diDongGiaSau) {
			t.Errorf("vết mang SỐ DI ĐỘNG TRẦN: %s", tho)
		}
	}
	d := deltaCua(t, rut)
	if d["ly_do_tu_dong"] != lyDoRutDoiDiDong {
		t.Errorf("ly_do_tu_dong = %v, muốn %q", d["ly_do_tu_dong"], lyDoRutDoiDiDong)
	}
	truoc, _ := d["truoc"].(map[string]any)
	sau, _ := d["sau"].(map[string]any)
	if truoc["hien_tren_mini_app"] != true || sau["hien_tren_mini_app"] != false {
		t.Errorf("vết không ghi trước/sau của cờ công khai: %v -> %v", truoc, sau)
	}
	if truoc["dong_y_cong_khai_ghi_boi"] != "CB-2026-GHIGOC" || sau["dong_y_cong_khai_ghi_boi"] != "" {
		t.Errorf("vết không ghi dấu đồng ý bị xoá: %v -> %v", truoc, sau)
	}
	// "truoc" names the OLD number — the one that was on the public channel — masked.
	if truoc["di_dong_ca_nhan"] != privacy.MaskPhone(diDongGia) {
		t.Errorf("vết không nêu số CŨ đã che: %v", truoc["di_dong_ca_nhan"])
	}
}

// Sending the mobile the row already has is not a change of the published data.
func TestSuaCungSoDiDongThiVanCongKhai(t *testing.T) {
	b := dungBanThuDanhBa(t)
	daCongKhai(b, nil)
	cu, ten := diDongGia, "Trần Thị C"

	cb, err := b.uc.Sua(ctxXa(xaThu), idNguoiKhac,
		YeuCauSuaCanBo{DiDongCaNhan: &cu, HoTen: &ten}, nguoiThucHienGia())
	if err != nil {
		t.Fatalf("Sua: %v", err)
	}
	if !cb.HienTrenMiniApp || cb.DongYCongKhaiGhiBoi != "CB-2026-GHIGOC" {
		t.Errorf("gửi lại đúng số di động đang có mà bị rút công khai: %+v", cb)
	}
	if b.kho.congKhaiCuoi != nil {
		t.Error("kho được yêu cầu ghi trạng thái công khai")
	}
	if got := hanhViCacVet(t, b); len(got) != 1 || got[0] != HanhViSuaCanBo {
		t.Errorf("vết = %v, muốn đúng [%s]", got, HanhViSuaCanBo)
	}
}

// OTHER FIELDS DO NOT TRIGGER IT — including the office line and co_zalo, which ARE on the public
// route. That is the scope the owner named ("Sửa số di động"); widening it is reported, not decided.
// If the owner widens it, THIS test is the one to change.
func TestSuaTruongKhacThiVanCongKhai(t *testing.T) {
	b := dungBanThuDanhBa(t)
	daCongKhai(b, nil)
	ten, cq, zalo := "Trần Thị C", "0236000000", true

	cb, err := b.uc.Sua(ctxXa(xaThu), idNguoiKhac,
		YeuCauSuaCanBo{HoTen: &ten, DienThoaiCoQuan: &cq, CoZalo: &zalo}, nguoiThucHienGia())
	if err != nil {
		t.Fatalf("Sua: %v", err)
	}
	if !cb.HienTrenMiniApp {
		t.Error("sửa trường khác số di động mà bị rút công khai")
	}
	if b.kho.congKhaiCuoi != nil {
		t.Error("kho được yêu cầu ghi trạng thái công khai")
	}
	if got := hanhViCacVet(t, b); len(got) != 1 || got[0] != HanhViSuaCanBo {
		t.Errorf("vết = %v, muốn đúng [%s]", got, HanhViSuaCanBo)
	}
}

// Not published: a changed mobile is an ordinary profile edit — no unpublish write, no entry for one.
func TestSuaDiDongNguoiChuaCongKhaiThiKhongGhiVetRut(t *testing.T) {
	b := dungBanThuDanhBa(t)
	moi := diDongGiaSau

	if _, err := b.uc.Sua(ctxXa(xaThu), idNguoiKhac, YeuCauSuaCanBo{DiDongCaNhan: &moi}, nguoiThucHienGia()); err != nil {
		t.Fatalf("Sua: %v", err)
	}
	if b.kho.congKhaiCuoi != nil {
		t.Error("người chưa công khai mà kho vẫn được yêu cầu rút công khai")
	}
	if got := hanhViCacVet(t, b); len(got) != 1 || got[0] != HanhViSuaCanBo {
		t.Errorf("vết = %v, muốn đúng [%s]", got, HanhViSuaCanBo)
	}
}

// --- lock / unlock ---------------------------------------------------------------------------------

// MUTATION THAT MUST TURN THIS RED: delete the `truoc.HienTrenMiniApp` branch in DatKhoa.
func TestKhoaNguoiDangCongKhaiThiRutVaXoaDauCungGiaoDich(t *testing.T) {
	b := dungBanThuDanhBa(t)
	daCongKhai(b, so(1))

	cb, err := b.uc.DatKhoa(ctxXa(xaThu), idNguoiKhac, true, nguoiThucHienGia())
	if err != nil {
		t.Fatalf("DatKhoa: %v", err)
	}
	if cb.DangHoatDong {
		t.Error("đã khoá mà dang_hoat_dong vẫn true")
	}
	daRutHet(t, "kết quả trả về", cb)
	if b.kho.congKhaiCuoi == nil {
		t.Fatal("khoá người đang công khai mà kho không được yêu cầu rút công khai")
	}
	daRutHet(t, "dòng ghi xuống", *b.kho.congKhaiCuoi)

	got := hanhViCacVet(t, b)
	if len(got) != 2 || got[0] != HanhViKhoaCanBo || got[1] != HanhViRutCongKhaiMiniApp {
		t.Fatalf("vết = %v, muốn [%s %s]", got, HanhViKhoaCanBo, HanhViRutCongKhaiMiniApp)
	}
	khoa, congKhai := b.ghi.tim("SET dang_hoat_dong"), b.ghi.tim("SET cong-khai")
	if khoa == nil || congKhai == nil {
		t.Fatal("thiếu câu khoá hoặc câu rút công khai")
	}
	for _, l := range append(vetDaGhi(b.ghi), *congKhai) {
		if l.tx == 0 || l.tx != khoa.tx {
			t.Fatalf("câu %q ở giao dịch %d, câu khoá ở %d — phải CÙNG một giao dịch", l.sql, l.tx, khoa.tx)
		}
	}
	if ket := b.ghi.ketThucCua(khoa.tx); ket != "commit" {
		t.Fatalf("giao dịch kết thúc bằng %q, muốn commit", ket)
	}
	rut := vetDaGhi(b.ghi)[1]
	if a := chuoiArg(t, rut, viTriActor); a != maCanBo {
		t.Errorf("actor = %q, muốn %q", a, maCanBo)
	}
	if d := deltaCua(t, rut); d["ly_do_tu_dong"] != lyDoRutKhoaTaiKhoan {
		t.Errorf("ly_do_tu_dong = %v, muốn %q", d["ly_do_tu_dong"], lyDoRutKhoaTaiKhoan)
	}
}

// UNLOCK NEVER REPUBLISHES. After a lock under the new rule the row carries nothing to republish
// from, so unlocking writes the unlock and nothing else.
func TestMoKhoaSauKhiKhoaThiVanKhongCongKhai(t *testing.T) {
	b := dungBanThuDanhBa(t)
	daCongKhai(b, nil)

	daKhoa, err := b.uc.DatKhoa(ctxXa(xaThu), idNguoiKhac, true, nguoiThucHienGia())
	if err != nil {
		t.Fatalf("khoá: %v", err)
	}
	b.kho.cb = daKhoa // the row as the lock left it
	b.kho.congKhaiCuoi = nil
	truocMo := len(vetDaGhi(b.ghi))

	cb, err := b.uc.DatKhoa(ctxXa(xaThu), idNguoiKhac, false, nguoiThucHienGia())
	if err != nil {
		t.Fatalf("mở khoá: %v", err)
	}
	if !cb.DangHoatDong {
		t.Error("mở khoá mà dang_hoat_dong vẫn false")
	}
	daRutHet(t, "sau khi mở khoá", cb)
	if b.kho.congKhaiCuoi != nil {
		t.Error("mở khoá mà kho được yêu cầu ghi trạng thái công khai")
	}
	got := hanhViCacVet(t, b)[truocMo:]
	if len(got) != 1 || got[0] != HanhViMoKhoaCanBo {
		t.Errorf("vết của lần mở khoá = %v, muốn đúng [%s]", got, HanhViMoKhoaCanBo)
	}
}

// A row that is LOCKED AND STILL PUBLISHED can only predate this rule (or have been published while
// locked). Unlocking it must not put it back on the public channel: it is unpublished in the same
// transaction as the unlock.
func TestMoKhoaDongCuConCongKhaiThiRut(t *testing.T) {
	b := dungBanThuDanhBa(t)
	daCongKhai(b, nil)
	b.kho.cb.DangHoatDong = false

	cb, err := b.uc.DatKhoa(ctxXa(xaThu), idNguoiKhac, false, nguoiThucHienGia())
	if err != nil {
		t.Fatalf("mở khoá: %v", err)
	}
	daRutHet(t, "kết quả trả về", cb)
	got := hanhViCacVet(t, b)
	if len(got) != 2 || got[0] != HanhViMoKhoaCanBo || got[1] != HanhViRutCongKhaiMiniApp {
		t.Fatalf("vết = %v, muốn [%s %s]", got, HanhViMoKhoaCanBo, HanhViRutCongKhaiMiniApp)
	}
	if d := deltaCua(t, vetDaGhi(b.ghi)[1]); d["ly_do_tu_dong"] != lyDoRutMoKhoaTaiKhoan {
		t.Errorf("ly_do_tu_dong = %v, muốn %q", d["ly_do_tu_dong"], lyDoRutMoKhoaTaiKhoan)
	}
}

// A REFUSED lock (#13) of a published person unpublishes nothing either: the refusal is before any
// write, and the rollback takes the whole transaction.
func TestKhoaBiTuChoiThiKhongRutCongKhai(t *testing.T) {
	b := dungBanThuDanhBa(t)
	daCongKhai(b, nil)
	b.kho.quanTri = []string{idNguoiKhac}

	_, err := b.uc.DatKhoa(ctxXa(xaThu), idNguoiKhac, true, nguoiThucHienGia())
	if !errors.Is(err, ErrQuanTriCuoiCung) {
		t.Fatalf("lỗi = %v, muốn ErrQuanTriCuoiCung", err)
	}
	khongCoGhi(t, b.ghi)
	if b.kho.congKhaiCuoi != nil {
		t.Error("khoá bị từ chối mà kho vẫn được yêu cầu rút công khai")
	}
}
