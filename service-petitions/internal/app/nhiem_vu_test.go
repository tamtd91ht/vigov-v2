package app

import (
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// Tests for the six STAFF acts on a task, over the REAL stores on a fake driver.
//
//	PROVED HERE   the business write, the timeline row and the audit entry share ONE transaction ·
//	              every refusal commits NOTHING · the rows a rule decides against are read
//	              `FOR UPDATE` · ADR 0037 decision 4 walks the WHOLE TREE (a grandchild blocks a
//	              parent) · the fifth rule finds a cycle across THREE rows · ADR 0038's two layers,
//	              including the empty-column refusal · approving an extension moves `han_xu_ly` and
//	              names `han_ban_dau` nowhere · a sub-task is created with NO deadline copied from
//	              its parent · the audit subject is the register number and the actor is the STAFF
//	              BUSINESS CODE.
//
//	NOT PROVED    anything PostgreSQL does with these statements — see the header of
//	              driver_gia_nhiem_vu_test.go for the full list. The CHECK constraints and the three
//	              triggers of migration 0006 are the FLOOR under everything here.

// --- helpers --------------------------------------------------------------------------------------

// chiGhiTrongGiaoDich asserts that every write statement ran INSIDE a transaction and that the
// transaction committed exactly once.
//
// THE `trongGiaoDich` FLAG IS THE WHOLE POINT OF THE FAKE DRIVER. A business write outside a
// transaction and one inside it are indistinguishable in every other respect, and rule 6, forbidden
// #2 is precisely that difference.
func chiGhiTrongGiaoDich(t *testing.T, k *khoNhiemVuGia) {
	t.Helper()
	for _, l := range k.lenh {
		if !strings.HasPrefix(l.sql, "INSERT") && !strings.HasPrefix(l.sql, "UPDATE") {
			continue
		}
		if !l.trongGiaoDich {
			t.Errorf("câu ghi chạy NGOÀI giao dịch: %q", l.sql)
		}
	}
	if k.daCommit != 1 {
		t.Errorf("commit %d lần, muốn 1", k.daCommit)
	}
}

// khongGhiGi asserts that a refusal left nothing behind: no business row, no timeline entry and no
// audit entry. A rule that refuses AFTER writing is a rule that did not refuse.
func khongGhiGi(t *testing.T, k *khoNhiemVuGia) {
	t.Helper()
	for _, l := range k.lenh {
		if strings.HasPrefix(l.sql, "INSERT") || strings.HasPrefix(l.sql, "UPDATE") {
			t.Errorf("đã ghi dù bị từ chối: %q", l.sql)
		}
	}
	if k.daCommit != 0 {
		t.Errorf("commit %d lần dù bị từ chối, muốn 0", k.daCommit)
	}
}

// vetKiemToan returns the one audit entry, failing when there is not exactly one.
func vetKiemToan(t *testing.T, k *khoNhiemVuGia) lenhPhieu {
	t.Helper()
	l := k.cau("INSERT INTO audit_log")
	if len(l) != 1 {
		t.Fatalf("ghi %d vết kiểm toán, muốn 1", len(l))
	}
	return l[0]
}

func taoMau() YeuCauTaoNhiemVu {
	return YeuCauTaoNhiemVu{
		TuSinhMa:  true,
		Loai:      "theo-van-ban",
		TieuDe:    "Rà soát tiến độ tuyến đường Hà Lam – Bình Trị",
		MucUuTien: "cao",
		HanXuLy:   mocHanNV,
	}
}

// --- 1. giao việc mới -------------------------------------------------------------------------------

func TestTaoNhiemVu_BaCauGhiTrongMotGiaoDich(t *testing.T) {
	k := khoNVMau()
	k.soLonNhat = 18
	uc, ctx := dungGhiNhiemVu(t, k)

	n, err := uc.Tao(ctx, taoMau(), canBoThu())
	if err != nil {
		t.Fatalf("giao việc mới: %v", err)
	}

	// THE THREE WRITES THAT MUST STAND OR FALL TOGETHER (rule 6, invariant 3).
	for _, tu := range []string{"INSERT INTO nhiem_vu", "INSERT INTO nhat_ky_nhiem_vu",
		"INSERT INTO audit_log"} {
		if !k.coCau(tu) {
			t.Errorf("thiếu câu %q", tu)
		}
	}
	chiGhiTrongGiaoDich(t, k)

	// THE NUMBER IS MINTED FROM THE COMMUNE'S OWN SERIES, inside that same transaction.
	if n.Ma != "NV19" {
		t.Errorf("mã nhiệm vụ = %q, muốn NV19 (số lớn nhất đã cấp là 18)", n.Ma)
	}
	if !k.coCau("MAX(") {
		t.Error("không đọc dãy số đã cấp — mã có thể trùng hoặc bắt đầu lại từ đầu")
	}
}

// TestTaoNhiemVu_ChuTheVaDoiTuongCuaVetLaMaNghiepVu pins rule 6, invariant 8 at the boundary it was
// broken at on 2026-09-22: `actor_id` holds the STAFF BUSINESS CODE, and the subject is the
// REGISTER NUMBER — not two internal ULIDs that name nobody a year later.
func TestTaoNhiemVu_ChuTheVaDoiTuongCuaVetLaMaNghiepVu(t *testing.T) {
	k := khoNVMau()
	k.soLonNhat = 18
	uc, ctx := dungGhiNhiemVu(t, k)

	if _, err := uc.Tao(ctx, taoMau(), canBoThu()); err != nil {
		t.Fatalf("giao việc mới: %v", err)
	}

	vet := vetKiemToan(t, k)
	if vet.args[1] != maCanBoThu {
		t.Errorf("chủ thể vết = %v, muốn mã cán bộ %q", vet.args[1], maCanBoThu)
	}
	if vet.args[4] != HanhViTaoNhiemVu {
		t.Errorf("hành vi = %v, muốn %q", vet.args[4], HanhViTaoNhiemVu)
	}
	if vet.args[5] != "NV19" {
		t.Errorf("đối tượng vết = %v, muốn mã nhiệm vụ NV19", vet.args[5])
	}
}

// TestTaoViecCon_KhongChepHanCuaCha IS ADR 0037 DECISION 2, and the thing it proves is an ABSENCE:
// no line in the use case reads the parent's deadline.
//
// The parent carries one; the child's form carried none. A child that came out with the parent's
// date would be a commitment nobody made, and §5.10 says the opposite in the specification's own
// words: "Việc con có hạn riêng".
func TestTaoViecCon_KhongChepHanCuaCha(t *testing.T) {
	k := khoNVMau() // the root task has han_xu_ly = mocHanNV
	k.soLonNhat = 19
	uc, ctx := dungGhiNhiemVu(t, k)

	yc := taoMau()
	yc.NhiemVuChaID = idNVGoc
	yc.HanXuLy = time.Time{} // the form left "Hạn hoàn thành" empty

	if _, err := uc.Tao(ctx, yc, canBoThu()); err != nil {
		t.Fatalf("giao việc con: %v", err)
	}

	chen := k.cau("INSERT INTO nhiem_vu")
	if len(chen) != 1 {
		t.Fatalf("ghi %d dòng nhiệm vụ, muốn 1", len(chen))
	}
	// $17 and $18 are `han_xu_ly` and `han_ban_dau` — the two columns the schema requires to arrive
	// together. Both must be NULL: the child was given no deadline of its own.
	if chen[0].args[16] != nil || chen[0].args[17] != nil {
		t.Fatalf("việc con nhận hạn %v / %v — ADR 0037 quyết định 2: con có hạn RIÊNG, không thừa kế",
			chen[0].args[16], chen[0].args[17])
	}
	// The parent WAS read and locked, which is what stops it being soft-deleted in the window.
	if !k.coCau("FOR UPDATE") {
		t.Error("không khoá dòng cha — cha có thể bị xoá mềm giữa lúc kiểm và lúc ghi")
	}
}

func TestTaoNhiemVu_HaiHanBangNhauKhiFormCoHan(t *testing.T) {
	k := khoNVMau()
	k.soLonNhat = 18
	uc, ctx := dungGhiNhiemVu(t, k)

	if _, err := uc.Tao(ctx, taoMau(), canBoThu()); err != nil {
		t.Fatalf("giao việc mới: %v", err)
	}
	chen := k.cau("INSERT INTO nhiem_vu")[0]
	// ONE VALUE, TWO COLUMNS. `han_ban_dau` is the denominator of §11.3 for ever after this
	// statement — the trigger refuses every later change to it.
	if chen.args[16] != mocHanNV || chen.args[17] != mocHanNV {
		t.Fatalf("hạn xử lý / hạn ban đầu = %v / %v, muốn cả hai là %v",
			chen.args[16], chen.args[17], mocHanNV)
	}
}

func TestTaoNhiemVu_ChaKhongTonTaiThiTuChoiVaKhongGhiGi(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)

	yc := taoMau()
	yc.NhiemVuChaID = "01JKHONGCOTRONGSONHIEMVU0"

	_, err := uc.Tao(ctx, yc, canBoThu())
	if !errors.Is(err, domain.ErrChaKhongTonTai) {
		t.Fatalf("lỗi = %v, muốn ErrChaKhongTonTai", err)
	}
	khongGhiGi(t, k)
}

func TestTaoNhiemVu_MaTuNhapDaDungThiTuChoi(t *testing.T) {
	k := khoNVMau()
	k.maDaDung = 1
	uc, ctx := dungGhiNhiemVu(t, k)

	yc := taoMau()
	yc.TuSinhMa = false
	yc.Ma = "NV19"

	_, err := uc.Tao(ctx, yc, canBoThu())
	if !errors.Is(err, petstore.ErrMaNhiemVuDaTonTai) {
		t.Fatalf("lỗi = %v, muốn ErrMaNhiemVuDaTonTai", err)
	}
	khongGhiGi(t, k)
}

func TestTaoNhiemVu_ThieuChuTheThiKhongMoGiaoDich(t *testing.T) {
	// Rule 6 does not permit a business write whose trail cannot name who made it — and refusing
	// BEFORE the transaction opens keeps a government register from being locked for a request that
	// was never going to be written.
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)

	if _, err := uc.Tao(ctx, taoMau(), audit.Actor{}); err == nil {
		t.Fatal("ghi được nhiệm vụ mà không có chủ thể")
	}
	if k.batDau != 0 {
		t.Errorf("mở %d giao dịch dù không có chủ thể, muốn 0", k.batDau)
	}
}

// --- 2. sửa, và LUẬT THỨ NĂM: chặn chu trình ---------------------------------------------------------

func TestSuaNhiemVu_KhongDoiGiThiKhongGhiGi(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)

	tieuDe := "Báo cáo tổng kết việc thực hiện chủ trương về công tác cán bộ" // the value already on the row
	if _, err := uc.Sua(ctx, maNVGoc, petstore.SuaNhiemVu{TieuDe: &tieuDe}, canBoThu()); err != nil {
		t.Fatalf("sửa nhiệm vụ: %v", err)
	}
	for _, l := range k.lenh {
		if strings.HasPrefix(l.sql, "UPDATE") || strings.Contains(l.sql, "INSERT INTO audit_log") {
			t.Errorf("ghi dù không có gì đổi: %q", l.sql)
		}
	}
}

// TestSuaNhiemVu_ChuTrinhBaTangBiChan IS THE FIFTH RULE — the one in no specification.
//
// The tree is `A → B → C`. Making A a child of C closes `A → B → C → A`, and NOTHING IN THE SCHEMA
// CAN SEE IT: migration 0008's CHECK compares one row's `nhiem_vu_cha_id` with its own `id`, so it
// catches only the one-hop case. Finding this one takes THREE HOPS UPWARD, inside the transaction.
//
// A cycle here is not an aesthetic problem: it makes every later tree walk — the completion check
// of decision 4 above all — run for ever while holding row locks on a government register.
func TestSuaNhiemVu_ChuTrinhBaTangBiChan(t *testing.T) {
	k := khoNVMau()
	k.themCon(idNVCon, maNVCon, idNVGoc, domain.DangThucHien)   // B under A
	k.themCon(idNVChau, maNVChau, idNVCon, domain.DangThucHien) // C under B
	uc, ctx := dungGhiNhiemVu(t, k)

	chau := idNVChau
	_, err := uc.Sua(ctx, maNVGoc, petstore.SuaNhiemVu{NhiemVuChaID: &chau}, canBoThu())

	if !errors.Is(err, domain.ErrChuTrinhCayNhiemVu) {
		t.Fatalf("lỗi = %v, muốn ErrChuTrinhCayNhiemVu — "+
			"A → B → C → A biểu diễn được và một CHECK không thấy được nó", err)
	}
	khongGhiGi(t, k)
}

// TestSuaNhiemVu_TuLamChaCuaMinhBiChan is the one-hop case. The schema catches it too, and this
// layer catches it FIRST, in a sentence somebody can read.
func TestSuaNhiemVu_TuLamChaCuaMinhBiChan(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)

	minh := idNVGoc
	_, err := uc.Sua(ctx, maNVGoc, petstore.SuaNhiemVu{NhiemVuChaID: &minh}, canBoThu())
	if !errors.Is(err, domain.ErrChuTrinhCayNhiemVu) {
		t.Fatalf("lỗi = %v, muốn ErrChuTrinhCayNhiemVu", err)
	}
	khongGhiGi(t, k)
}

// TestSuaNhiemVu_ChuyenSangNhanhKhacVanDuoc is the other half: without it, the two cases above
// would also pass against a rule that refused EVERY re-parenting.
func TestSuaNhiemVu_ChuyenSangNhanhKhacVanDuoc(t *testing.T) {
	k := khoNVMau()
	k.themCon(idNVCon, maNVCon, idNVGoc, domain.DangThucHien)
	// A second root, unrelated to A: moving C under it closes nothing.
	k.nhiemVu[idNVChau] = dongNhiemVuGia(idNVChau, maNVChau, nil)
	uc, ctx := dungGhiNhiemVu(t, k)

	goc2 := idNVChau
	if _, err := uc.Sua(ctx, maNVCon, petstore.SuaNhiemVu{NhiemVuChaID: &goc2}, canBoThu()); err != nil {
		t.Fatalf("chuyển việc con sang nhánh khác bị từ chối: %v", err)
	}
	if !k.coCau("UPDATE nhiem_vu") {
		t.Error("không ghi gì dù đổi cha hợp lệ")
	}
	chiGhiTrongGiaoDich(t, k)
}

// --- 3. đổi trạng thái, và ADR 0037 quyết định 4 -------------------------------------------------------

// TestHoanThanh_ChauChuaXongThiChan IS ADR 0037 DECISION 4, at the depth that separates a correct
// recursion from a plausible one.
//
// The tree is `A → B → C`. B is FINISHED and C is not. A one-level check looks at B, finds it
// finished, and completes A — which is exactly the defect migration 0008 warns about in its own
// words: "DUYỆT ĐỆ QUY CẢ CÂY chứ không chỉ một tầng con". The completion rate that reaches
// leadership would then count a parent whose grandchild is still open.
func TestHoanThanh_ChauChuaXongThiChan(t *testing.T) {
	k := khoNVMau()
	k.nhiemVu[idNVGoc]["trang_thai"] = string(domain.ChoDuyet)
	k.themCon(idNVCon, maNVCon, idNVGoc, domain.HoanThanh)      // the CHILD is finished
	k.themCon(idNVChau, maNVChau, idNVCon, domain.DangThucHien) // the GRANDCHILD is not
	uc, ctx := dungGhiNhiemVu(t, k)

	_, err := uc.DoiTrangThai(ctx, maNVGoc,
		YeuCauDoiTrangThai{TrangThai: string(domain.HoanThanh)}, canBoThu(), true)

	if !errors.Is(err, domain.ErrConChuaXong) {
		t.Fatalf("lỗi = %v, muốn ErrConChuaXong — duyệt một tầng con là chưa đủ", err)
	}
	// THE REFUSAL NAMES THE WORK THAT IS IN THE WAY, which is decision 4's own wording.
	if !strings.Contains(err.Error(), maNVChau) {
		t.Errorf("câu từ chối không nêu việc con còn lại (%s): %v", maNVChau, err)
	}
	khongGhiGi(t, k)
}

func TestHoanThanh_CaCayXongThiQua(t *testing.T) {
	k := khoNVMau()
	k.nhiemVu[idNVGoc]["trang_thai"] = string(domain.ChoDuyet)
	k.themCon(idNVCon, maNVCon, idNVGoc, domain.HoanThanh)
	k.themCon(idNVChau, maNVChau, idNVCon, domain.HoanThanh)
	uc, ctx := dungGhiNhiemVu(t, k)

	sau, err := uc.DoiTrangThai(ctx, maNVGoc,
		YeuCauDoiTrangThai{TrangThai: string(domain.HoanThanh)}, canBoThu(), true)
	if err != nil {
		t.Fatalf("hoàn thành khi cả cây đã xong bị từ chối: %v", err)
	}
	if sau.NgayHoanThanh != mocThaoTacNV {
		t.Errorf("ngày hoàn thành = %v, muốn %v", sau.NgayHoanThanh, mocThaoTacNV)
	}
	// §11.3's ratio cannot classify a finished task with no instant, and the schema ties the two
	// together with a biconditional — so the UPDATE has to carry it.
	doi := k.cau("UPDATE nhiem_vu")
	if len(doi) != 1 || doi[0].args[3] != mocThaoTacNV {
		t.Fatalf("câu đổi trạng thái không ghi ngày hoàn thành: %+v", doi)
	}
	chiGhiTrongGiaoDich(t, k)
}

// TestHoanThanh_ThieuQuyenDuyetThiTuChoi pins the second key: `task.update` moves work along,
// `task.approve` declares it finished (§6). Rule 5, invariant 3b — these are not a Cartesian
// product.
func TestHoanThanh_ThieuQuyenDuyetThiTuChoi(t *testing.T) {
	k := khoNVMau()
	k.nhiemVu[idNVGoc]["trang_thai"] = string(domain.ChoDuyet)
	uc, ctx := dungGhiNhiemVu(t, k)

	_, err := uc.DoiTrangThai(ctx, maNVGoc,
		YeuCauDoiTrangThai{TrangThai: string(domain.HoanThanh)}, canBoThu(), false)

	if !errors.Is(err, ErrKhongDuocDuyetHoanThanh) {
		t.Fatalf("lỗi = %v, muốn ErrKhongDuocDuyetHoanThanh", err)
	}
	khongGhiGi(t, k)
	// AND THE REFUSAL CAME BEFORE THE TREE WAS READ: a caller who may not complete the task is not
	// told which of its sub-tasks are still open.
	if k.coCau("nhiem_vu_cha_id = $2") {
		t.Error("đã duyệt cây dù người gọi không có quyền duyệt hoàn thành — lộ thông tin điều hành")
	}
}

func TestDoiTrangThai_GhiNhatKyCungGiaoDichVaMangTrangThaiSau(t *testing.T) {
	k := khoNVMau() // dang-thuc-hien
	uc, ctx := dungGhiNhiemVu(t, k)

	if _, err := uc.DoiTrangThai(ctx, maNVGoc,
		YeuCauDoiTrangThai{TrangThai: string(domain.ChoDuyet)}, canBoThu(), false); err != nil {
		t.Fatalf("đổi trạng thái: %v", err)
	}

	nk := k.cau("INSERT INTO nhat_ky_nhiem_vu")
	if len(nk) != 1 {
		t.Fatalf("ghi %d dòng nhật ký, muốn 1", len(nk))
	}
	// $6 is `trang_thai_tai_thoi_diem`. IT CARRIES THE STATE THE ACT LANDED THE TASK IN — which is
	// what the resume rule reads back. Recording the state BEFORE the act would make every resume
	// read one row too far back.
	if nk[0].args[5] != string(domain.ChoDuyet) {
		t.Errorf("trạng thái tại thời điểm = %v, muốn %q", nk[0].args[5], domain.ChoDuyet)
	}
	chiGhiTrongGiaoDich(t, k)
}

// TestTamDung_TiepTucVeDungTrangThaiTruocDocTuNhatKy is §6's "(trạng thái trước)".
//
// THERE IS NO `trang_thai_truoc` COLUMN AND THERE MUST NOT BE ONE: the fact is already in
// `nhat_ky_nhiem_vu`, and a second copy is what rule 9's one-line test forbids.
func TestTamDung_TiepTucVeDungTrangThaiTruocDocTuNhatKy(t *testing.T) {
	k := khoNVMau()
	k.nhiemVu[idNVGoc]["trang_thai"] = string(domain.TamDung)
	// NEWEST FIRST: paused, and before that the task was `da-tiep-nhan`.
	k.nhatKy[idNVGoc] = []string{string(domain.TamDung), string(domain.DaTiepNhanNV)}
	uc, ctx := dungGhiNhiemVu(t, k)

	if _, err := uc.DoiTrangThai(ctx, maNVGoc,
		YeuCauDoiTrangThai{TrangThai: string(domain.DaTiepNhanNV)}, canBoThu(), false); err != nil {
		t.Fatalf("tiếp tục về trạng thái trước bị từ chối: %v", err)
	}
	if !k.coCau("FROM nhat_ky_nhiem_vu") {
		t.Error("không đọc nhật ký — trạng thái trước phải SUY RA, không được đoán")
	}
}

func TestTamDung_TiepTucSaiTrangThaiThiTuChoi(t *testing.T) {
	k := khoNVMau()
	k.nhiemVu[idNVGoc]["trang_thai"] = string(domain.TamDung)
	k.nhatKy[idNVGoc] = []string{string(domain.TamDung), string(domain.DaTiepNhanNV)}
	uc, ctx := dungGhiNhiemVu(t, k)

	// It was paused from `da-tiep-nhan`; resuming into `dang-thuc-hien` would skip a step nobody
	// took.
	_, err := uc.DoiTrangThai(ctx, maNVGoc,
		YeuCauDoiTrangThai{TrangThai: string(domain.DangThucHien)}, canBoThu(), false)
	if !errors.Is(err, domain.ErrChuyenTrangThaiNhiemVuSaiLuc) {
		t.Fatalf("lỗi = %v, muốn ErrChuyenTrangThaiNhiemVuSaiLuc", err)
	}
	khongGhiGi(t, k)
}

func TestTamDung_NhatKyKhongNoiDuocThiTuChoiChuKhongDoan(t *testing.T) {
	k := khoNVMau()
	k.nhiemVu[idNVGoc]["trang_thai"] = string(domain.TamDung)
	// No timeline at all — the log cannot say what it was paused from.
	uc, ctx := dungGhiNhiemVu(t, k)

	_, err := uc.DoiTrangThai(ctx, maNVGoc,
		YeuCauDoiTrangThai{TrangThai: string(domain.DangThucHien)}, canBoThu(), false)
	if !errors.Is(err, domain.ErrTiepTucKhongBietTrangThaiTruoc) {
		t.Fatalf("lỗi = %v, muốn ErrTiepTucKhongBietTrangThaiTruoc", err)
	}
	khongGhiGi(t, k)
}

// --- 4. xoá mềm, ADR 0037 quyết định 3 -----------------------------------------------------------------

func TestXoa_ConChuaXoaThiTuChoiKemSoLuong(t *testing.T) {
	k := khoNVMau()
	k.themCon(idNVCon, maNVCon, idNVGoc, domain.DangThucHien)
	k.themCon(idNVChau, maNVChau, idNVGoc, domain.HoanThanh)
	uc, ctx := dungGhiNhiemVu(t, k)

	err := uc.Xoa(ctx, maNVGoc, "Trùng với nhiệm vụ NV05.", canBoThu())

	if !errors.Is(err, domain.ErrConChuaXoa) {
		t.Fatalf("lỗi = %v, muốn ErrConChuaXoa", err)
	}
	// "kèm câu nói rõ còn mấy việc con" — ADR 0037 decision 3's own wording.
	if !strings.Contains(err.Error(), "2") {
		t.Errorf("câu từ chối không nói còn mấy việc con: %v", err)
	}
	khongGhiGi(t, k)
}

func TestXoa_KhongConConThiXoaMemVaGhiVet(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)

	if err := uc.Xoa(ctx, maNVGoc, "Trùng với nhiệm vụ NV05.", canBoThu()); err != nil {
		t.Fatalf("xoá nhiệm vụ: %v", err)
	}

	doi := k.cau("UPDATE nhiem_vu")
	if len(doi) != 1 {
		t.Fatalf("chạy %d câu xoá, muốn 1", len(doi))
	}
	// IT IS A SOFT DELETE: the statement sets the three columns and there is no DELETE anywhere.
	if !strings.Contains(doi[0].sql, "deleted_at") || !strings.Contains(doi[0].sql, "delete_reason") {
		t.Errorf("câu xoá không phải xoá mềm: %q", doi[0].sql)
	}
	for _, l := range k.lenh {
		if strings.HasPrefix(l.sql, "DELETE") {
			t.Errorf("có câu xoá cứng trên hồ sơ lưu trữ: %q", l.sql)
		}
	}
	// `deleted_by` IS THE STAFF BUSINESS CODE — read years later by somebody handling a complaint.
	if doi[0].args[3] != maCanBoThu {
		t.Errorf("người xoá = %v, muốn mã cán bộ %q", doi[0].args[3], maCanBoThu)
	}
	chiGhiTrongGiaoDich(t, k)
}

// --- 5 & 6. lùi hạn, ADR 0038 -------------------------------------------------------------------------

func TestDeNghiLuiHan_GhiDeNghiVaVetTrongMotGiaoDich(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)

	_, err := uc.DeNghiLuiHan(ctx, maNVGoc,
		YeuCauDeNghiLuiHan{HanMoi: mocHanMoiNV, LyDo: "Chờ số liệu từ ba chi bộ."}, canBoThu())
	if err != nil {
		t.Fatalf("đề nghị lùi hạn: %v", err)
	}
	if !k.coCau("INSERT INTO de_nghi_lui_han") {
		t.Error("không ghi đề nghị")
	}
	chiGhiTrongGiaoDich(t, k)

	// THE REQUESTER IS THE ACTING PRINCIPAL, never a field of the request.
	dn := k.cau("INSERT INTO de_nghi_lui_han")[0]
	if dn.args[3] != maCanBoThu {
		t.Errorf("người đề nghị = %v, muốn mã cán bộ %q", dn.args[3], maCanBoThu)
	}
}

func TestDeNghiLuiHan_DaCoDeNghiChoDuyetThiTuChoi(t *testing.T) {
	k := khoNVMau()
	k.deNghi[idDeNghi] = dongDeNghiGia(nil) // one already pending
	uc, ctx := dungGhiNhiemVu(t, k)

	_, err := uc.DeNghiLuiHan(ctx, maNVGoc,
		YeuCauDeNghiLuiHan{HanMoi: mocHanMoiNV, LyDo: "Lý do khác."}, canBoThu())
	if !errors.Is(err, domain.ErrDaCoDeNghiChoDuyet) {
		t.Fatalf("lỗi = %v, muốn ErrDaCoDeNghiChoDuyet", err)
	}
	khongGhiGi(t, k)
}

func TestDeNghiLuiHan_HanMoiKhongMuonHonThiTuChoi(t *testing.T) {
	k := khoNVMau()
	uc, ctx := dungGhiNhiemVu(t, k)

	_, err := uc.DeNghiLuiHan(ctx, maNVGoc,
		YeuCauDeNghiLuiHan{HanMoi: mocHanNV.Add(-time.Hour), LyDo: "Lý do."}, canBoThu())
	if !errors.Is(err, domain.ErrHanMoiKhongLui) {
		t.Fatalf("lỗi = %v, muốn ErrHanMoiKhongLui", err)
	}
	khongGhiGi(t, k)
}

// TestDuyetLuiHan_ChiLanhDaoGhiTrenBanGhi IS ADR 0038'S SECOND LAYER, and it is the case the
// permission key cannot express: rule 5 checks `(tenant_id, role, permission)` and has no "which
// record" dimension.
//
// The caller here HOLDS `task.extend` — that is the route's gate and it has already passed. What is
// refused is deciding an extension on somebody else's task.
func TestDuyetLuiHan_ChiLanhDaoGhiTrenBanGhi(t *testing.T) {
	k := khoNVMau()
	k.deNghi[idDeNghi] = dongDeNghiGia(nil)
	uc, ctx := dungGhiNhiemVu(t, k)

	// `maCanBoThu` is not `maLanhDao` — a DIFFERENT member of staff holding the same key.
	_, err := uc.QuyetDinhLuiHan(ctx, maNVGoc, idDeNghi,
		YeuCauQuyetDinhLuiHan{Duyet: true}, canBoThu())

	if !errors.Is(err, domain.ErrKhongPhaiLanhDaoGiaoViec) {
		t.Fatalf("lỗi = %v, muốn ErrKhongPhaiLanhDaoGiaoViec — "+
			"cầm `task.extend` KHÔNG có nghĩa duyệt được việc của người khác", err)
	}
	khongGhiGi(t, k)
}

func TestDuyetLuiHan_DungLanhDaoThiQua(t *testing.T) {
	k := khoNVMau()
	k.deNghi[idDeNghi] = dongDeNghiGia(nil)
	uc, ctx := dungGhiNhiemVu(t, k)

	lanhDao := audit.Actor{ID: maLanhDao, Kind: "staff", IP: "10.0.0.8"}
	sau, err := uc.QuyetDinhLuiHan(ctx, maNVGoc, idDeNghi,
		YeuCauQuyetDinhLuiHan{Duyet: true}, lanhDao)
	if err != nil {
		t.Fatalf("lãnh đạo giao việc duyệt bị từ chối: %v", err)
	}
	if sau.TrangThai != domain.DaDuyetLuiHan || sau.NguoiDuyetMa != maLanhDao {
		t.Fatalf("đề nghị sau quyết định = %+v", sau)
	}
	chiGhiTrongGiaoDich(t, k)
}

// TestDuyetLuiHan_ChuaGhiLanhDaoThiTuChoi is ADR 0038's OPEN QUESTION, failing CLOSED — and the
// sentence has to name what is missing, because that is the only thing the commune can act on.
//
// ⚠ IT MUST NOT FALL BACK TO `nguoi_tao_ma`. The fixture's creator is `CB-00123`, which IS the
// acting officer here: a fallback would make this call SUCCEED, and the clerk who typed the row on
// somebody else's behalf would be deciding an extension.
func TestDuyetLuiHan_ChuaGhiLanhDaoThiTuChoi(t *testing.T) {
	k := khoNVMau()
	k.nhiemVu[idNVGoc]["lanh_dao_giao_viec_ma"] = nil
	k.nhiemVu[idNVGoc]["nguoi_tao_ma"] = maCanBoThu
	k.deNghi[idDeNghi] = dongDeNghiGia(nil)
	uc, ctx := dungGhiNhiemVu(t, k)

	_, err := uc.QuyetDinhLuiHan(ctx, maNVGoc, idDeNghi,
		YeuCauQuyetDinhLuiHan{Duyet: true}, canBoThu())

	if !errors.Is(err, domain.ErrChuaGhiLanhDaoGiaoViec) {
		t.Fatalf("lỗi = %v, muốn ErrChuaGhiLanhDaoGiaoViec — "+
			"KHÔNG được rơi về người tạo: đó là trao quyền duyệt cho văn thư đã nhập hộ", err)
	}
	if !strings.Contains(err.Error(), "lãnh đạo giao việc") {
		t.Errorf("câu từ chối không nói rõ thiếu gì: %v", err)
	}
	khongGhiGi(t, k)
}

// TestDuyetLuiHan_KhongTuDuyetDeNghiCuaMinh is ADR 0038's SECOND, INDEPENDENT rule. It bites even
// when the person IS the named leader — the case the first rule cannot catch.
func TestDuyetLuiHan_KhongTuDuyetDeNghiCuaMinh(t *testing.T) {
	k := khoNVMau()
	k.deNghi[idDeNghi] = dongDeNghiGia(map[string]driver.Value{"nguoi_de_nghi_ma": maLanhDao})
	uc, ctx := dungGhiNhiemVu(t, k)

	lanhDao := audit.Actor{ID: maLanhDao, Kind: "staff", IP: "10.0.0.8"}
	_, err := uc.QuyetDinhLuiHan(ctx, maNVGoc, idDeNghi,
		YeuCauQuyetDinhLuiHan{Duyet: true}, lanhDao)

	if !errors.Is(err, domain.ErrTuDuyetDeNghiCuaMinh) {
		t.Fatalf("lỗi = %v, muốn ErrTuDuyetDeNghiCuaMinh", err)
	}
	khongGhiGi(t, k)
}

// TestDuyetLuiHan_DoiHanXuLyChuKhongDoiHanBanDau is §5.8's promise made true: "Hạn gốc vẫn được giữ
// lại để báo cáo đúng hạn không bị lùi theo".
//
// THE ASSERTION IS ON THE STATEMENT, not on a struct. `han_ban_dau` is the denominator of §11.3, and
// an UPDATE that moved it alongside `han_xu_ly` would make every granted extension read as a
// deadline met — retroactively, in a figure that goes upward.
func TestDuyetLuiHan_DoiHanXuLyChuKhongDoiHanBanDau(t *testing.T) {
	k := khoNVMau()
	k.deNghi[idDeNghi] = dongDeNghiGia(nil)
	uc, ctx := dungGhiNhiemVu(t, k)

	lanhDao := audit.Actor{ID: maLanhDao, Kind: "staff", IP: "10.0.0.8"}
	if _, err := uc.QuyetDinhLuiHan(ctx, maNVGoc, idDeNghi,
		YeuCauQuyetDinhLuiHan{Duyet: true}, lanhDao); err != nil {
		t.Fatalf("duyệt lùi hạn: %v", err)
	}

	doi := k.cau("UPDATE nhiem_vu")
	if len(doi) != 1 {
		t.Fatalf("chạy %d câu đổi hạn, muốn 1", len(doi))
	}
	if strings.Contains(doi[0].sql, "han_ban_dau") {
		t.Fatalf("câu lùi hạn chạm tới `han_ban_dau`: %q", doi[0].sql)
	}
	if doi[0].args[2] != mocHanMoiNV {
		t.Errorf("hạn mới = %v, muốn %v", doi[0].args[2], mocHanMoiNV)
	}
	// The WHERE clause carries the deadline the approver saw, so a second approval elsewhere cannot
	// move the commitment twice.
	if doi[0].args[3] != mocHanNV {
		t.Errorf("câu lùi hạn không so với hạn đang có: %v", doi[0].args[3])
	}
}

func TestTuChoiLuiHan_KhongDongToiHanCuaNhiemVu(t *testing.T) {
	k := khoNVMau()
	k.deNghi[idDeNghi] = dongDeNghiGia(nil)
	uc, ctx := dungGhiNhiemVu(t, k)

	lanhDao := audit.Actor{ID: maLanhDao, Kind: "staff", IP: "10.0.0.8"}
	sau, err := uc.QuyetDinhLuiHan(ctx, maNVGoc, idDeNghi,
		YeuCauQuyetDinhLuiHan{Duyet: false}, lanhDao)
	if err != nil {
		t.Fatalf("từ chối đề nghị: %v", err)
	}
	if sau.TrangThai != domain.TuChoiLuiHan {
		t.Errorf("trạng thái đề nghị = %q, muốn %q", sau.TrangThai, domain.TuChoiLuiHan)
	}
	if k.coCau("UPDATE nhiem_vu") {
		t.Error("từ chối mà vẫn đổi hạn của nhiệm vụ")
	}
	if !k.coCau("UPDATE de_nghi_lui_han") {
		t.Error("không ghi quyết định từ chối")
	}
	chiGhiTrongGiaoDich(t, k)
}

// TestDuyetLuiHan_DeNghiCuaNhiemVuKhacThiKhongThay closes the path where a request filed on task A
// is decided through task B's URL — where the leader named on B would answer for A.
func TestDuyetLuiHan_DeNghiCuaNhiemVuKhacThiKhongThay(t *testing.T) {
	k := khoNVMau()
	k.deNghi[idDeNghi] = dongDeNghiGia(map[string]driver.Value{"nhiem_vu_id": "01JNHIEMVUKHACHOANTOAN00"})
	uc, ctx := dungGhiNhiemVu(t, k)

	lanhDao := audit.Actor{ID: maLanhDao, Kind: "staff", IP: "10.0.0.8"}
	_, err := uc.QuyetDinhLuiHan(ctx, maNVGoc, idDeNghi,
		YeuCauQuyetDinhLuiHan{Duyet: true}, lanhDao)

	if !errors.Is(err, petstore.ErrDeNghiKhongTonTai) {
		t.Fatalf("lỗi = %v, muốn ErrDeNghiKhongTonTai", err)
	}
	khongGhiGi(t, k)
}
