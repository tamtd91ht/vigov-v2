package app

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// Xoa — the soft delete of a DUPLICATED directory row (#10; user decision 2026-09-24).
//
// Every refusal below fails SILENTLY if its guard is removed: the request succeeds, the row leaves
// every screen, and the trail records an ordinary delete. What the fake driver can prove is which
// statements ran inside which transaction and how it ended — which is what "a refusal writes
// nothing" and "the entry shares the transaction" turn on. What only a real PostgreSQL can prove
// (the CHECKs of 0010 accepting the row, the read paths hiding it, the code staying taken) is in
// danh_ba_can_bo_xoa_pg_test.go.

const lyDoXoaGia = "Nhập trùng với CB-2026-7K3M9Q do nhập Excel hai lần"

// banThuXoa is the default fixture turned into what the route exists for: a DIRECTORY-ONLY row,
// with no account, that is a duplicate.
func banThuXoa(t *testing.T) *banThuDanhBa {
	t.Helper()
	b := dungBanThuDanhBa(t)
	b.kho.cb.CoTaiKhoan = false
	b.kho.cb.VaiTroID = ""
	return b
}

// THE WRITE AND ITS ENTRY SHARE ONE TRANSACTION, WHICH COMMITS; deleted_by AND THE ACTOR ARE BOTH
// THE STAFF CODE; THE SUBJECT IS THE DELETED ROW'S CODE.
//
// MUTATIONS THAT MUST TURN THIS RED:
//   - pass nguoi.ID to XoaMem instead of nguoi.Vet.ID (deleted_by = internal id)
//   - move audit.Write out of the Tx closure, or drop it
func TestXoaCanBoNhapTrungGhiVetCungGiaoDichVaNguoiXoaLaMaCanBo(t *testing.T) {
	b := banThuXoa(t)

	if err := b.uc.Xoa(ctxXa(xaThu), idNguoiKhac, "  "+lyDoXoaGia+"  ", nguoiThucHienGia()); err != nil {
		t.Fatalf("Xoa: %v", err)
	}

	x := b.kho.xoa
	if x == nil {
		t.Fatal("XoaMem không được gọi")
	}
	if x.xoaBoi != maCanBo {
		t.Errorf("deleted_by = %q, muốn MÃ CÁN BỘ %q (luật 6 bất biến 8)", x.xoaBoi, maCanBo)
	}
	if x.xoaBoi == idNoiBo {
		t.Error("deleted_by đang mang ĐỊNH DANH NỘI BỘ")
	}
	if x.lyDo != lyDoXoaGia {
		t.Errorf("delete_reason = %q, muốn bản đã cắt khoảng trắng %q", x.lyDo, lyDoXoaGia)
	}
	if !x.luc.Equal(time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)) {
		t.Errorf("deleted_at = %v, muốn đồng hồ của use case", x.luc)
	}
	if x.id != idNguoiKhac {
		t.Errorf("xoá dòng %q, muốn %q", x.id, idNguoiKhac)
	}

	ghi := b.ghi.tim("SET xoa-mem")
	vet := motVet(t, b.ghi)
	if ghi == nil || vet.tx == 0 || vet.tx != ghi.tx {
		t.Fatalf("vết và câu xoá mềm không cùng một giao dịch (luật 6 bất biến 3)")
	}
	if ket := b.ghi.ketThucCua(vet.tx); ket != "commit" {
		t.Fatalf("giao dịch kết thúc bằng %q, muốn commit", ket)
	}

	if got := chuoiArg(t, vet, viTriActor); got != maCanBo {
		t.Errorf("actor_id = %q, muốn %q", got, maCanBo)
	}
	if got := chuoiArg(t, vet, viTriHanhVi); got != HanhViXoaCanBoNhapTrung {
		t.Errorf("action = %q, muốn %q", got, HanhViXoaCanBoNhapTrung)
	}
	if got := chuoiArg(t, vet, viTriChuThe); got != maNguoiKhac {
		t.Errorf("subject = %q, muốn mã của dòng bị xoá %q", got, maNguoiKhac)
	}
	if got := chuoiArg(t, vet, 0); got != string(xaThu) {
		t.Errorf("tenant_id của vết = %q, muốn %q", got, string(xaThu))
	}
	if got := chuoiArg(t, vet, 3); got != ipGia {
		t.Errorf("actor_ip = %q, muốn %q", got, ipGia)
	}
}

// THE ENTRY IS MASKED AND DOES NOT CARRY THE REASON TEXT — only its length (rule 6, forbidden #4;
// the convention service-petitions' task delete set). The row it describes is named by Subject.
func TestXoaCanBoVetCheDuLieuCaNhanVaKhongChepLyDo(t *testing.T) {
	b := banThuXoa(t)

	if err := b.uc.Xoa(ctxXa(xaThu), idNguoiKhac, lyDoXoaGia, nguoiThucHienGia()); err != nil {
		t.Fatalf("Xoa: %v", err)
	}
	vet := motVet(t, b.ghi)
	tho := string(vet.args[viTriDelta].([]byte))

	for _, cam := range []string{diDongGia, dienThoaiGia, "Trần Thị B", lyDoXoaGia} {
		if strings.Contains(tho, cam) {
			t.Errorf("vết mang giá trị thô %q: %s", cam, tho)
		}
	}
	d := deltaCua(t, vet)
	if n, _ := d["do_dai_ly_do"].(float64); int(n) != len([]rune(lyDoXoaGia)) {
		t.Errorf("do_dai_ly_do = %v, muốn %d", d["do_dai_ly_do"], len([]rune(lyDoXoaGia)))
	}
	truoc, _ := d["truoc"].(map[string]any)
	sau, _ := d["sau"].(map[string]any)
	if truoc["da_xoa"] != false || sau["da_xoa"] != true {
		t.Errorf("vết không ghi trước/sau của việc xoá: %v -> %v", truoc, sau)
	}
	if _, co := truoc["ho_ten"]; !co {
		t.Errorf("vết không nêu (đã che) họ tên của dòng bị xoá: %v", truoc)
	}
}

// A PUBLISHED ROW IS UNPUBLISHED BY THE DELETE, AND THE ENTRY SAYS SO. The UPDATE's literals are
// proven in store/can_bo_xoa_test.go; this is the trail recording the transition, so "a deleted row
// was taken off the public channel" is answerable from the ledger.
func TestXoaCanBoDangCongKhaiThiVetGhiRutCongKhai(t *testing.T) {
	b := banThuXoa(t)
	luc := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	b.kho.cb.HienTrenMiniApp = true
	b.kho.cb.DongYCongKhaiLuc = &luc
	b.kho.cb.DongYCongKhaiGhiBoi = "CB-2026-GHI001"

	if err := b.uc.Xoa(ctxXa(xaThu), idNguoiKhac, lyDoXoaGia, nguoiThucHienGia()); err != nil {
		t.Fatalf("Xoa: %v", err)
	}
	d := deltaCua(t, motVet(t, b.ghi))
	truoc, _ := d["truoc"].(map[string]any)
	sau, _ := d["sau"].(map[string]any)
	if truoc["hien_tren_mini_app"] != true || truoc["dong_y_cong_khai_ghi_boi"] != "CB-2026-GHI001" {
		t.Errorf("vết không ghi trạng thái công khai TRƯỚC khi xoá: %v", truoc)
	}
	if sau["hien_tren_mini_app"] != false || sau["dong_y_cong_khai_luc"] != nil || sau["dong_y_cong_khai_ghi_boi"] != "" {
		t.Errorf("vết không ghi việc gỡ công khai và xoá dấu đồng ý: %v", sau)
	}
}

// A ROW WITH A SIGN-IN ACCOUNT IS REFUSED, AND NOTHING IS WRITTEN (user decision 2026-09-24).
//
// MUTATION THAT MUST TURN THIS RED: delete the `truoc.CoTaiKhoan` check in Xoa. The request then
// succeeds and a credential stays attached to a row no screen shows.
func TestXoaCanBoCoTaiKhoanBiTuChoiVaKhongGhiGi(t *testing.T) {
	b := banThuXoa(t)
	b.kho.cb.CoTaiKhoan = true

	err := b.uc.Xoa(ctxXa(xaThu), idNguoiKhac, lyDoXoaGia, nguoiThucHienGia())
	if !errors.Is(err, ErrCanBoCoTaiKhoan) {
		t.Fatalf("lỗi = %v, muốn ErrCanBoCoTaiKhoan", err)
	}
	if b.kho.xoa != nil {
		t.Error("dòng có tài khoản vẫn bị xoá mềm")
	}
	khongCoGhi(t, b.ghi)
	if ket := b.ghi.ketThucCua(1); ket != "rollback" {
		t.Fatalf("giao dịch kết thúc bằng %q, muốn rollback", ket)
	}
}

// A LOCKED account is still an account: locking is the answer to retirement, not a licence to
// delete. The refusal reads `co_tai_khoan`, not `dang_hoat_dong`.
func TestXoaCanBoTaiKhoanDaKhoaVanBiTuChoi(t *testing.T) {
	b := banThuXoa(t)
	b.kho.cb.CoTaiKhoan = true
	b.kho.cb.DangHoatDong = false

	if err := b.uc.Xoa(ctxXa(xaThu), idNguoiKhac, lyDoXoaGia, nguoiThucHienGia()); !errors.Is(err, ErrCanBoCoTaiKhoan) {
		t.Fatalf("lỗi = %v, muốn ErrCanBoCoTaiKhoan", err)
	}
	khongCoGhi(t, b.ghi)
}

// #13, DEFENSIVE. Unreachable while `dieuKienGiuQuyen` requires an account — this fixture builds
// the inconsistent state on purpose (an account-less row inside the administrator set) to prove the
// line is really there rather than assumed.
//
// MUTATION THAT MUST TURN THIS RED: delete the laNguoiQuanTriCuoiCung check in Xoa.
func TestXoaNguoiQuanTriCuoiCungBiTuChoiDuKhongCoTaiKhoan(t *testing.T) {
	b := banThuXoa(t)
	b.kho.quanTri = []string{idNguoiKhac}

	err := b.uc.Xoa(ctxXa(xaThu), idNguoiKhac, lyDoXoaGia, nguoiThucHienGia())
	if !errors.Is(err, ErrQuanTriCuoiCung) {
		t.Fatalf("lỗi = %v, muốn ErrQuanTriCuoiCung (câu #13)", err)
	}
	khongCoGhi(t, b.ghi)
}

// #14 — nobody deletes their own row. Refused before any read.
func TestXoaChinhMinhBiTuChoiTruocMoiLuotDoc(t *testing.T) {
	b := banThuXoa(t)
	b.kho.cb.ID = idNoiBo

	if err := b.uc.Xoa(ctxXa(xaThu), idNoiBo, lyDoXoaGia, nguoiThucHienGia()); !errors.Is(err, ErrTuThaoTacChinhMinh) {
		t.Fatalf("lỗi = %v, muốn ErrTuThaoTacChinhMinh", err)
	}
	if n := b.ghi.soGiaoDich(); n != 0 {
		t.Errorf("mở %d giao dịch cho một thao tác bị chặn trước mọi lượt đọc", n)
	}
}

// A MISSING, BLANK OR OVER-LONG REASON OPENS NO TRANSACTION.
func TestXoaLyDoThieuHoacQuaDaiThiKhongMoGiaoDich(t *testing.T) {
	for ten, c := range map[string]struct {
		lyDo string
		muon error
	}{
		"rỗng":       {"", domain.ErrThieuLyDoXoa},
		"toàn trắng": {"   \t ", domain.ErrThieuLyDoXoa},
		"quá dài":    {strings.Repeat("ệ", 501), domain.ErrLyDoXoaQuaDai},
	} {
		b := banThuXoa(t)
		if err := b.uc.Xoa(ctxXa(xaThu), idNguoiKhac, c.lyDo, nguoiThucHienGia()); !errors.Is(err, c.muon) {
			t.Errorf("%s: lỗi = %v, muốn %v", ten, err, c.muon)
		}
		if n := b.ghi.soGiaoDich(); n != 0 {
			t.Errorf("%s: mở %d giao dịch cho một lý do không hợp lệ", ten, n)
		}
	}
}

// ALREADY DELETED, ANOTHER COMMUNE'S, OR INVENTED — one answer, nothing written. The fake answers
// "not there" the way the scoped `deleted_at IS NULL` read does for all three.
func TestXoaDongKhongConThiKhongTimThayVaKhongGhiGi(t *testing.T) {
	b := banThuXoa(t)
	b.kho.coDong = false

	if err := b.uc.Xoa(ctxXa(xaThu), idNguoiKhac, lyDoXoaGia, nguoiThucHienGia()); !errors.Is(err, idstore.ErrCanBoKhongTonTai) {
		t.Fatalf("lỗi = %v, muốn ErrCanBoKhongTonTai", err)
	}
	khongCoGhi(t, b.ghi)
}

// AN ACTOR WITH NO STAFF CODE REFUSES THE WRITE — deleted_by would otherwise have nothing honest to
// hold, and there is no fallback to the internal id.
func TestXoaThieuMaNguoiThucHienThiTuChoi(t *testing.T) {
	b := banThuXoa(t)
	nguoi := nguoiThucHienGia()
	nguoi.Vet.ID = ""

	if err := b.uc.Xoa(ctxXa(xaThu), idNguoiKhac, lyDoXoaGia, nguoi); err == nil {
		t.Fatal("thiếu mã cán bộ của người thực hiện mà vẫn xoá")
	}
	if b.kho.xoa != nil {
		t.Error("XoaMem đã chạy với người thực hiện không có mã")
	}
	khongCoGhi(t, b.ghi)
}
