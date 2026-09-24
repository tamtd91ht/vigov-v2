package app

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/privacy"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// WHAT THIS FILE IS FOR: DatCongKhai — open question #12, the publication of a staff member's
// personal mobile to the public Zalo Mini App directory. Every defect here is SILENT: a publication
// without consent succeeds, a recorder written as an internal id is a valid string, a stale consent
// left on an unpublished row looks perfectly consented. The database CHECKs of migration 0010 §3
// back the last one up; the rest can only be caught here.
//
// Same fake driver and fake store as danh_ba_can_bo_test.go.

// mocDongY is the pinned clock of dungBanThuDanhBa — the value the consent time must carry.
var mocDongY = time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)

func so(v int) *int { return &v }

// daCongKhai puts the fake row into the published state, with consent recorded by SOMEBODY ELSE
// at an EARLIER time — so an assertion can tell the original marks from freshly written ones.
func daCongKhai(b *banThuDanhBa, thuTu *int) time.Time {
	luc := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	b.kho.cb.HienTrenMiniApp = true
	b.kho.cb.DongYCongKhaiLuc = &luc
	b.kho.cb.DongYCongKhaiGhiBoi = "CB-2026-GHIGOC"
	b.kho.cb.ThuTuDanhBa = thuTu
	return luc
}

// NO CONSENT CONFIRMATION, NO PUBLICATION — AND NOTHING WRITTEN, NOT EVEN A TRANSACTION OPENED.
//
// MUTATION THAT MUST TURN THIS RED: delete the `yc.CongKhai && !yc.DaXacNhanDongY` check.
func TestCongKhaiKhongXacNhanDongYBiTuChoiVaKhongGhiGi(t *testing.T) {
	b := dungBanThuDanhBa(t)

	_, err := b.uc.DatCongKhai(ctxXa(xaThu), idNguoiKhac,
		YeuCauCongKhai{CongKhai: true, DaXacNhanDongY: false}, nguoiThucHienGia())
	if !errors.Is(err, ErrChuaXacNhanDongY) {
		t.Fatalf("lỗi = %v, muốn ErrChuaXacNhanDongY (#12)", err)
	}
	khongCoGhi(t, b.ghi)
	if n := b.ghi.soGiaoDich(); n != 0 {
		t.Errorf("mở %d giao dịch cho một yêu cầu thiếu xác nhận đồng ý", n)
	}
	if b.kho.congKhaiCuoi != nil {
		t.Error("kho đã được yêu cầu ghi trạng thái công khai")
	}
}

// PUBLISHING WRITES BOTH CONSENT MARKS — when (this service's clock) and who (the recorder's STAFF
// CODE) — and the audit entry shares the transaction.
//
// MUTATION THAT MUST TURN THIS RED: write nguoi.ID into DongYCongKhaiGhiBoi.
func TestCongKhaiGhiDauDongYBangMaCanBoVaVetCungGiaoDich(t *testing.T) {
	b := dungBanThuDanhBa(t)

	cb, err := b.uc.DatCongKhai(ctxXa(xaThu), idNguoiKhac,
		YeuCauCongKhai{CongKhai: true, DaXacNhanDongY: true, ThuTu: so(4)}, nguoiThucHienGia())
	if err != nil {
		t.Fatalf("DatCongKhai: %v", err)
	}

	ghi := b.kho.congKhaiCuoi
	if ghi == nil {
		t.Fatal("kho không được yêu cầu ghi")
	}
	for ten, v := range map[string]domain.CanBoTomTat{"kết quả trả về": cb, "dòng ghi xuống": *ghi} {
		if !v.HienTrenMiniApp {
			t.Errorf("%s: HienTrenMiniApp = false", ten)
		}
		if v.DongYCongKhaiLuc == nil || !v.DongYCongKhaiLuc.Equal(mocDongY) {
			t.Errorf("%s: DongYCongKhaiLuc = %v, muốn %v", ten, v.DongYCongKhaiLuc, mocDongY)
		}
		if v.DongYCongKhaiGhiBoi != maCanBo {
			t.Errorf("%s: người ghi đồng ý = %q, muốn MÃ CÁN BỘ %q (luật 6 bất biến 8)",
				ten, v.DongYCongKhaiGhiBoi, maCanBo)
		}
		if v.DongYCongKhaiGhiBoi == idNoiBo {
			t.Errorf("%s: người ghi đồng ý đang là ĐỊNH DANH NỘI BỘ", ten)
		}
		if v.ThuTuDanhBa == nil || *v.ThuTuDanhBa != 4 {
			t.Errorf("%s: ThuTuDanhBa = %v, muốn 4", ten, v.ThuTuDanhBa)
		}
	}

	capNhat := b.ghi.tim("SET cong-khai")
	vet := motVet(t, b.ghi)
	if capNhat == nil || vet.tx == 0 || vet.tx != capNhat.tx {
		t.Fatalf("vết và câu UPDATE không cùng một giao dịch (luật 6 bất biến 3)")
	}
	if ket := b.ghi.ketThucCua(vet.tx); ket != "commit" {
		t.Fatalf("giao dịch kết thúc bằng %q, muốn commit", ket)
	}
	if got := chuoiArg(t, vet, viTriActor); got != maCanBo {
		t.Errorf("actor_id = %q, muốn %q", got, maCanBo)
	}
	if got := chuoiArg(t, vet, viTriHanhVi); got != HanhViCongKhaiMiniApp {
		t.Errorf("action = %q, muốn %q", got, HanhViCongKhaiMiniApp)
	}
	if got := chuoiArg(t, vet, viTriChuThe); got != maNguoiKhac {
		t.Errorf("subject = %q, muốn mã của người được công khai %q", got, maNguoiKhac)
	}

	d := deltaCua(t, vet)
	truoc, _ := d["truoc"].(map[string]any)
	sau, _ := d["sau"].(map[string]any)
	if truoc["hien_tren_mini_app"] != false || sau["hien_tren_mini_app"] != true {
		t.Errorf("vết không ghi trước/sau của cờ công khai: %v -> %v", truoc, sau)
	}
	if sau["dong_y_cong_khai_ghi_boi"] != maCanBo {
		t.Errorf("vết: người ghi đồng ý = %v, muốn %q", sau["dong_y_cong_khai_ghi_boi"], maCanBo)
	}
	if sau["dong_y_cong_khai_luc"] != mocDongY.Format(time.RFC3339) {
		t.Errorf("vết: lúc đồng ý = %v, muốn %s", sau["dong_y_cong_khai_luc"], mocDongY.Format(time.RFC3339))
	}
	tho := string(vet.args[viTriDelta].([]byte))
	if strings.Contains(tho, diDongGia) {
		t.Errorf("vết mang SỐ DI ĐỘNG TRẦN: %s", tho)
	}
	if sau["di_dong_ca_nhan"] != privacy.MaskPhone(diDongGia) {
		t.Errorf("vết không nêu số di động ĐÃ CHE: %v", sau["di_dong_ca_nhan"])
	}
}

// UNPUBLISHING CLEARS BOTH MARKS IN THE SAME WRITE, and ignores whatever consent flag was sent. The
// next publication must therefore ask again (#12, user decision 2026-09-24).
func TestRutCongKhaiXoaCaHaiDauDongY(t *testing.T) {
	b := dungBanThuDanhBa(t)
	daCongKhai(b, so(2))

	cb, err := b.uc.DatCongKhai(ctxXa(xaThu), idNguoiKhac,
		YeuCauCongKhai{CongKhai: false, DaXacNhanDongY: true, ThuTu: so(2)}, nguoiThucHienGia())
	if err != nil {
		t.Fatalf("DatCongKhai: %v", err)
	}
	for ten, v := range map[string]domain.CanBoTomTat{"kết quả trả về": cb, "dòng ghi xuống": *b.kho.congKhaiCuoi} {
		if v.HienTrenMiniApp || v.DongYCongKhaiLuc != nil || v.DongYCongKhaiGhiBoi != "" {
			t.Errorf("%s: thôi công khai mà còn dấu (hien=%v, luc=%v, ghi_boi=%q) — CHECK 0010 §3 sẽ từ chối, "+
				"và dấu cũ sẽ bị dùng lại cho lần công khai sau", ten, v.HienTrenMiniApp, v.DongYCongKhaiLuc, v.DongYCongKhaiGhiBoi)
		}
	}
	vet := motVet(t, b.ghi)
	if got := chuoiArg(t, vet, viTriHanhVi); got != HanhViRutCongKhaiMiniApp {
		t.Errorf("action = %q, muốn %q", got, HanhViRutCongKhaiMiniApp)
	}
	truoc, _ := deltaCua(t, vet)["truoc"].(map[string]any)
	if truoc["dong_y_cong_khai_ghi_boi"] != "CB-2026-GHIGOC" {
		t.Errorf("vết mất bằng chứng đồng ý TRƯỚC khi rút: %v", truoc)
	}
}

// PUBLISHING SOMEBODY ALREADY PUBLISHED, SAME ORDER, WRITES NOTHING — and the original consent
// marks survive. That is the property `idem.KhongCan` on the route claims.
func TestCongKhaiNguoiDaCongKhaiGiuDauGocVaKhongGhi(t *testing.T) {
	b := dungBanThuDanhBa(t)
	lucGoc := daCongKhai(b, so(2))

	cb, err := b.uc.DatCongKhai(ctxXa(xaThu), idNguoiKhac,
		YeuCauCongKhai{CongKhai: true, DaXacNhanDongY: true, ThuTu: so(2)}, nguoiThucHienGia())
	if err != nil {
		t.Fatalf("DatCongKhai: %v", err)
	}
	if b.kho.congKhaiCuoi != nil || len(vetDaGhi(b.ghi)) != 0 {
		t.Error("không có gì đổi mà vẫn ghi dòng hoặc vết")
	}
	if cb.DongYCongKhaiGhiBoi != "CB-2026-GHIGOC" || !cb.DongYCongKhaiLuc.Equal(lucGoc) {
		t.Errorf("dấu đồng ý gốc bị ghi đè: %q %v", cb.DongYCongKhaiGhiBoi, cb.DongYCongKhaiLuc)
	}
}

// Moving a published person in the list is its own verb, and keeps the consent marks.
func TestDoiThuTuKhiDangCongKhai(t *testing.T) {
	b := dungBanThuDanhBa(t)
	daCongKhai(b, nil)

	if _, err := b.uc.DatCongKhai(ctxXa(xaThu), idNguoiKhac,
		YeuCauCongKhai{CongKhai: true, DaXacNhanDongY: true, ThuTu: so(0)}, nguoiThucHienGia()); err != nil {
		t.Fatalf("DatCongKhai: %v", err)
	}
	vet := motVet(t, b.ghi)
	if got := chuoiArg(t, vet, viTriHanhVi); got != HanhViDoiThuTuDanhBa {
		t.Errorf("action = %q, muốn %q", got, HanhViDoiThuTuDanhBa)
	}
	if g := b.kho.congKhaiCuoi; g == nil || g.DongYCongKhaiGhiBoi != "CB-2026-GHIGOC" ||
		g.ThuTuDanhBa == nil || *g.ThuTuDanhBa != 0 {
		t.Errorf("dòng ghi xuống sai: %+v", g)
	}
}

// AN ACTOR WITH NO STAFF CODE IS REFUSED — never replaced by the internal id — and nothing is written.
func TestCongKhaiThieuMaNguoiThucHienBiTuChoi(t *testing.T) {
	b := dungBanThuDanhBa(t)
	nguoi := nguoiThucHienGia()
	nguoi.Vet.ID = ""

	if _, err := b.uc.DatCongKhai(ctxXa(xaThu), idNguoiKhac,
		YeuCauCongKhai{CongKhai: true, DaXacNhanDongY: true}, nguoi); err == nil {
		t.Fatal("thiếu mã cán bộ của người thực hiện mà vẫn công khai")
	}
	khongCoGhi(t, b.ghi)
}

func TestCongKhaiThuTuAmBiTuChoiTruocGiaoDich(t *testing.T) {
	b := dungBanThuDanhBa(t)

	_, err := b.uc.DatCongKhai(ctxXa(xaThu), idNguoiKhac,
		YeuCauCongKhai{CongKhai: false, ThuTu: so(-1)}, nguoiThucHienGia())
	if !errors.Is(err, domain.ErrThuTuDanhBaAm) {
		t.Fatalf("lỗi = %v, muốn ErrThuTuDanhBaAm", err)
	}
	if n := b.ghi.soGiaoDich(); n != 0 {
		t.Errorf("mở %d giao dịch cho một thứ tự âm", n)
	}
}

// Another commune's id, a soft-deleted person and an invented id are one answer (TheoIDDeGhi is
// scoped and excludes deleted rows), and nothing is written.
func TestCongKhaiNguoiKhongTonTaiThiKhongGhi(t *testing.T) {
	b := dungBanThuDanhBa(t)
	b.kho.coDong = false

	_, err := b.uc.DatCongKhai(ctxXa(xaThu), idNguoiKhac,
		YeuCauCongKhai{CongKhai: true, DaXacNhanDongY: true}, nguoiThucHienGia())
	if !errors.Is(err, idstore.ErrCanBoKhongTonTai) {
		t.Fatalf("lỗi = %v, muốn ErrCanBoKhongTonTai", err)
	}
	khongCoGhi(t, b.ghi)
}

// --- PATCH has_zalo ----------------------------------------------------------------------------

func TestSuaCoZaloGhiDongVaVet(t *testing.T) {
	b := dungBanThuDanhBa(t)
	co := true

	cb, err := b.uc.Sua(ctxXa(xaThu), idNguoiKhac, YeuCauSuaCanBo{CoZalo: &co}, nguoiThucHienGia())
	if err != nil {
		t.Fatalf("Sua: %v", err)
	}
	if !cb.CoZalo {
		t.Error("CoZalo không được đặt")
	}
	if b.ghi.tim("SET ho-so") == nil {
		t.Error("đổi Có Zalo mà không chạy UPDATE")
	}
	sau, _ := deltaCua(t, motVet(t, b.ghi))["sau"].(map[string]any)
	if sau["co_zalo"] != true {
		t.Errorf("vết không ghi co_zalo: %v", sau)
	}
	// And nil means "unchanged": a second edit that does not mention it leaves it alone.
	ten := "Trần Thị C"
	b.kho.cb = cb
	cb2, err := b.uc.Sua(ctxXa(xaThu), idNguoiKhac, YeuCauSuaCanBo{HoTen: &ten}, nguoiThucHienGia())
	if err != nil {
		t.Fatalf("Sua lần hai: %v", err)
	}
	if !cb2.CoZalo {
		t.Error("sửa trường khác mà Có Zalo bị xoá")
	}
}
