package app

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// WHAT THIS FILE IS FOR: the SQL and the transaction boundaries of the budget board, asserted
// through the REAL store on a fake `database/sql` driver — see driver_gia_ngan_sach_test.go for why
// this shape and what it still does not prove.
//
// THE PROPERTIES, and each is a defect that fails silently:
//
//  1. the audit entry is in the SAME transaction as the business write (rule 6, invariant 3), and a
//     failure on the entry takes the write down with it;
//  2. a REFUSAL writes nothing at all — no UPDATE, no INSERT, no entry;
//  3. marking a row as the total CLEARS every other row of the sheet, in the same transaction;
//  4. removing a line writes all three soft-delete columns AND releases the star;
//  5. clearing a cell writes NULL and never a DELETE;
//  6. `cach_tinh` is never written from anything a request could reach — it follows the tree;
//  7. the actor written into `deleted_by` is the STAFF CODE, never a ULID (rule 6, invariant 8).

const (
	idBangMau = "01JBANGCHIMAU0000000000000"
	idDongMau = "01JKHOANMUCMAU000000000000"
	idDongKia = "01JKHOANMUCKIA000000000000"
	maCanBo   = "CB-00123"
)

func nguoiGhi() audit.Actor { return audit.Actor{ID: maCanBo, Kind: "staff", IP: "10.0.0.7"} }

// khoMau is a chi sheet with two top-level lines, the first of them MARKED. It is the shape §10
// describes for the chi tab — `Tổng số` beside `A. CHI NGÂN SÁCH NHÀ NƯỚC` — which is the shape that
// makes "take the first row" and "sum the roots" both wrong.
func khoMau() *khoNSGia {
	return &khoNSGia{
		bang: &hangBang{id: idBangMau, ma: "NS-2026-CHI-01", nam: 2026, loai: "chi", lan: 1,
			tieuDe: "BÁO CÁO CHI NGÂN SÁCH NHÀ NƯỚC XÃ THĂNG BÌNH NĂM 2026", donViTinh: "Triệu đồng"},
		cot: []domain.CotNganSach{
			{ID: "c-dt", BangID: idBangMau, Ten: "Dự toán năm", ThuTu: 1,
				Kieu: domain.CotSo, VaiTro: domain.VaiTroDuToanNam},
			{ID: "c-chi", BangID: idBangMau, Ten: "Chi ngân sách", ThuTu: 2,
				Kieu: domain.CotSo, VaiTro: domain.VaiTroChiNganSach},
			{ID: "c-ty", BangID: idBangMau, Ten: "So sánh TH/DT (%)", ThuTu: 3,
				Kieu: domain.CotPhanTram, CongThuc: "col_2 / col_1 * 100"},
		},
		khoanMuc: []domain.KhoanMucNganSach{
			{ID: idDongMau, BangID: idBangMau, Ten: "Tổng số", ThuTu: 1,
				CachTinh: domain.TinhTay, LaDongTong: true},
			{ID: idDongKia, BangID: idBangMau, TT: "A", Ten: "CHI NGÂN SÁCH NHÀ NƯỚC", ThuTu: 2,
				CachTinh: domain.TinhTay},
		},
		gia:               map[string]map[string]domain.Dong{idDongMau: {"c-chi": 5_000_000}},
		bangIDCuaKhoanMuc: idBangMau,
	}
}

// --- (1) the entry shares the transaction ------------------------------------------------------------

func TestThemKhoanMucGhiVetTrongCUNGGiaoDich(t *testing.T) {
	k := khoMau()
	uc, ctx := dungUseCaseNganSach(t, k)

	if _, err := uc.ThemKhoanMuc(ctx, YeuCauThemKhoanMuc{
		BangID: idBangMau, ChaID: idDongKia, TT: "I", Ten: "Chi đầu tư phát triển", ThuTu: 3,
	}, nguoiGhi()); err != nil {
		t.Fatalf("ThemKhoanMuc lỗi: %v", err)
	}

	if k.batDau != 1 || k.daCommit != 1 || k.daRollback != 0 {
		t.Fatalf("giao dịch: mở %d commit %d rollback %d, muốn 1/1/0",
			k.batDau, k.daCommit, k.daRollback)
	}
	if !k.coCau("INSERT INTO khoan_muc_ngan_sach") {
		t.Fatal("không có câu chèn khoản mục")
	}
	if !k.coCau("INSERT INTO audit_log") {
		t.Fatal("không có vết kiểm toán")
	}
	// THE SUBJECT IS THE SHEET'S BUSINESS CODE, not an internal id. A budget LINE has no code of its
	// own, so `NS-2026-CHI-01` is the handle an inspection can look up years later; a ULID names
	// nobody (rule 6, invariant 8).
	vet := k.cau("INSERT INTO audit_log")[0]
	if !coGiaTri(vet, "NS-2026-CHI-01") {
		t.Fatalf("subject của vết không phải mã bảng: %v", vet.args)
	}
	if !coGiaTri(vet, maCanBo) {
		t.Fatalf("chủ thể của vết không phải mã cán bộ %q: %v", maCanBo, vet.args)
	}
	if coGiaTri(vet, "nd-01JINTERNALIDCUACANBO") {
		t.Fatal("chủ thể của vết là id nội bộ — rule 6 bất biến 8")
	}
}

func TestVetHongThiKhoanMucCungKhongCon(t *testing.T) {
	// Rule 6, invariant 3 stated as its consequence: the entry is the LAST statement of the
	// transaction, so a failure there must take the line with it. If the two could be committed
	// separately, the register would hold a line nobody can attribute.
	k := khoMau()
	k.loiSau = "INSERT INTO audit_log"
	uc, ctx := dungUseCaseNganSach(t, k)

	if _, err := uc.ThemKhoanMuc(ctx, YeuCauThemKhoanMuc{
		BangID: idBangMau, Ten: "Chi quốc phòng", ThuTu: 3,
	}, nguoiGhi()); err == nil {
		t.Fatal("vết hỏng mà ThemKhoanMuc vẫn báo thành công")
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Fatalf("giao dịch: commit %d rollback %d, muốn 0/1", k.daCommit, k.daRollback)
	}
}

// --- (2) a refusal writes nothing ------------------------------------------------------------------------

func TestGoThangVaoKhoanMucChaKhongGhiGiNua(t *testing.T) {
	// The customer's decision of 06/09/2026 (anh Hà), enforced in the business layer. `idDongKia` is
	// about to be given a child, so it becomes a parent; typing a figure into it is refused.
	k := khoMau()
	k.khoanMuc = append(k.khoanMuc, domain.KhoanMucNganSach{
		ID: "k-con", BangID: idBangMau, ChaID: idDongKia, TT: "I", Ten: "Chi đầu tư", ThuTu: 3, Cap: 1,
	})
	k.bangIDCuaKhoanMuc = idBangMau
	uc, ctx := dungUseCaseNganSach(t, k)

	mot := domain.Dong(9_000_000)
	_, err := uc.SuaKhoanMuc(ctx, idDongKia, YeuCauSuaKhoanMuc{
		GiaTri: map[string]*domain.Dong{"c-chi": &mot},
	}, nguoiGhi())
	if !errors.Is(err, domain.ErrKhoanMucChaKhongGoThang) {
		t.Fatalf("= %v, muốn ErrKhoanMucChaKhongGoThang", err)
	}
	if k.coCau("INSERT INTO gia_tri_khoan_muc") {
		t.Fatal("bị từ chối mà vẫn ghi giá trị")
	}
	if k.coCau("INSERT INTO audit_log") {
		t.Fatal("bị từ chối mà vẫn ghi vết")
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Fatalf("giao dịch: commit %d rollback %d, muốn 0/1", k.daCommit, k.daRollback)
	}
}

func TestGoKhoanMucConDongConThiTuChoi(t *testing.T) {
	// Refused rather than cascaded: a cascade takes a branch off every total in one click, and these
	// rows are archival — "undo" is not a button, it is re-entering them.
	k := khoMau()
	k.khoanMuc = append(k.khoanMuc, domain.KhoanMucNganSach{
		ID: "k-con", BangID: idBangMau, ChaID: idDongKia, Ten: "Chi đầu tư", ThuTu: 3, Cap: 1,
	})
	uc, ctx := dungUseCaseNganSach(t, k)

	err := uc.GoKhoanMuc(ctx, idDongKia, "nhập trùng", nguoiGhi())
	if !errors.Is(err, domain.ErrConGiuKhoanMucCon) {
		t.Fatalf("= %v, muốn ErrConGiuKhoanMucCon", err)
	}
	if k.coCau("deleted_at = now()") {
		t.Fatal("bị từ chối mà vẫn xoá mềm")
	}
}

// --- (3) the star is a radio ----------------------------------------------------------------------------

func TestDatDongTongBoDanhDauMoiDongKhacTrongCungGiaoDich(t *testing.T) {
	// THE PROPERTY THAT HAS NO FLOOR IN THE DATABASE. Migration 0006 states why it carries no partial
	// unique index for "at most one marked row", so these two statements under the sheet's row lock
	// ARE the enforcement. Two marked rows is the double count `is_headline` was introduced to end.
	k := khoMau()
	uc, ctx := dungUseCaseNganSach(t, k)

	if _, err := uc.DatDongTong(ctx, idDongKia, nguoiGhi()); err != nil {
		t.Fatalf("DatDongTong lỗi: %v", err)
	}

	bo := k.cau("is_headline = false")
	dat := k.cau("is_headline = true")
	if len(bo) != 1 || len(dat) != 1 {
		t.Fatalf("câu bỏ đánh dấu %d, câu đánh dấu %d — muốn 1 và 1", len(bo), len(dat))
	}
	// THE CLEAR MUST COME FIRST. Reversed, the sheet would momentarily have two marked rows and then
	// none, and a reader inside the transaction would see a sheet with no total at all.
	if k.thuTuCua("is_headline = false") > k.thuTuCua("is_headline = true") {
		t.Fatal("đánh dấu TRƯỚC khi bỏ đánh dấu — giữa hai câu bảng có hai dòng tổng")
	}
	// The clear is scoped to the SHEET and excludes the row being marked.
	if !strings.Contains(bo[0].sql, "bang_id = $2") || !strings.Contains(bo[0].sql, "id <> $3") {
		t.Fatalf("câu bỏ đánh dấu không giới hạn đúng phạm vi: %s", bo[0].sql)
	}
	if !strings.Contains(bo[0].sql, "tenant_id = $1") {
		t.Fatalf("câu bỏ đánh dấu không mang tenant_id: %s", bo[0].sql)
	}
	if k.batDau != 1 || k.daCommit != 1 {
		t.Fatalf("giao dịch: mở %d commit %d, muốn 1/1", k.batDau, k.daCommit)
	}
	// The trail records WHICH ROW HELD IT BEFORE. After the write that answer is gone from the table,
	// so this entry is the only place "the reported total moved from here to there" survives.
	vet := k.cau("INSERT INTO audit_log")[0]
	if !coChuoiTrongDelta(vet, idDongMau) {
		t.Fatalf("vết không ghi dòng tổng CŨ: %v", vet.args)
	}
}

func TestDatDongTongLenDongKhongCoThiKhongGhiGi(t *testing.T) {
	k := khoMau()
	k.bangIDCuaKhoanMuc = "" // no such live line
	uc, ctx := dungUseCaseNganSach(t, k)

	_, err := uc.DatDongTong(ctx, "khong-co", nguoiGhi())
	if !errors.Is(err, domain.ErrKhongThayKhoanMuc) {
		t.Fatalf("= %v, muốn ErrKhongThayKhoanMuc", err)
	}
	if k.coCau("is_headline = true") || k.coCau("INSERT INTO audit_log") {
		t.Fatal("dòng không tồn tại mà vẫn ghi")
	}
}

// --- (4) removal -----------------------------------------------------------------------------------------

func TestGoKhoanMucGhiDuBaCotVaTraSaoVe(t *testing.T) {
	// rule 7, invariant 1's three columns in ONE statement so none can be forgotten — plus
	// `is_headline = false`, which is what lets the commune star another row and get its summary back.
	// Without it the flag would sit on a dead row: the total would be gone and nothing would say why.
	k := khoMau()
	uc, ctx := dungUseCaseNganSach(t, k)

	if err := uc.GoKhoanMuc(ctx, idDongMau, "kế toán nhập trùng dòng này", nguoiGhi()); err != nil {
		t.Fatalf("GoKhoanMuc lỗi: %v", err)
	}
	xoa := k.cau("deleted_at = now()")
	if len(xoa) != 1 {
		t.Fatalf("có %d câu xoá mềm, muốn 1", len(xoa))
	}
	for _, manh := range []string{"deleted_at = now()", "deleted_by = $3", "delete_reason = $4",
		"is_headline = false", "deleted_at IS NULL"} {
		if !strings.Contains(xoa[0].sql, manh) {
			t.Errorf("câu xoá mềm thiếu %q: %s", manh, xoa[0].sql)
		}
	}
	// NO HARD DELETE ANYWHERE. `ho_so_luu_tru_cam_xoa_cung` refuses one underneath; this is the
	// assertion that the application never even writes one.
	if k.coCau("DELETE FROM") {
		t.Fatal("có câu DELETE FROM trên dữ liệu nghiệp vụ")
	}
	// `deleted_by` holds the STAFF CODE, the same value the entry's actor holds. Two kinds of
	// identifier in one column is a column nobody can query.
	if !coGiaTri(xoa[0], maCanBo) {
		t.Fatalf("deleted_by không phải mã cán bộ: %v", xoa[0].args)
	}
}

func TestGoDongConCuoiCungThiChaVeNhapTayVaGiuSoVuaTinh(t *testing.T) {
	// §9 rule 1's second half: "đổi ngược lại thì giữ giá trị vừa tính làm giá trị khởi đầu". Without
	// it the parent goes blank, and blank on this screen means "nobody has entered this" — which is
	// not what happened.
	k := khoMau()
	k.khoanMuc = []domain.KhoanMucNganSach{
		{ID: idDongKia, BangID: idBangMau, TT: "A", Ten: "CHI NGÂN SÁCH NHÀ NƯỚC", ThuTu: 1,
			CachTinh: domain.TinhTheoCon},
		{ID: "k-con", BangID: idBangMau, ChaID: idDongKia, TT: "I", Ten: "Chi đầu tư", ThuTu: 2,
			CachTinh: domain.TinhTay, Cap: 1},
	}
	k.gia = map[string]map[string]domain.Dong{"k-con": {"c-chi": 7_777_000}}
	uc, ctx := dungUseCaseNganSach(t, k)

	if err := uc.GoKhoanMuc(ctx, "k-con", "gộp vào dòng khác", nguoiGhi()); err != nil {
		t.Fatalf("GoKhoanMuc lỗi: %v", err)
	}
	dat := k.cau("SET cach_tinh = $3")
	if len(dat) != 1 {
		t.Fatalf("có %d câu đặt cách tính, muốn 1", len(dat))
	}
	if !coGiaTri(dat[0], string(domain.TinhTay)) {
		t.Fatalf("cha không về `manual`: %v", dat[0].args)
	}
	ghi := k.cau("INSERT INTO gia_tri_khoan_muc")
	if len(ghi) != 1 {
		t.Fatalf("có %d câu ghi giá trị trả lại, muốn 1", len(ghi))
	}
	if !coGiaTri(ghi[0], int64(7_777_000)) {
		t.Fatalf("giá trị trả lại cho cha không phải số vừa tính: %v", ghi[0].args)
	}
}

// --- (5) cells ---------------------------------------------------------------------------------------------

func TestXoaOGhiNULLChuKhongXoaDong(t *testing.T) {
	// §9 rule 4 and rule 7, forbidden #1 at once: an empty cell is a STATE the screen draws as `—`,
	// and the row stays. A DELETE here would be a hard delete on business data.
	k := khoMau()
	uc, ctx := dungUseCaseNganSach(t, k)

	if _, err := uc.SuaKhoanMuc(ctx, idDongMau, YeuCauSuaKhoanMuc{
		GiaTri: map[string]*domain.Dong{"c-chi": nil},
	}, nguoiGhi()); err != nil {
		t.Fatalf("SuaKhoanMuc lỗi: %v", err)
	}
	ghi := k.cau("INSERT INTO gia_tri_khoan_muc")
	if len(ghi) != 1 {
		t.Fatalf("có %d câu ghi ô, muốn 1", len(ghi))
	}
	if !strings.Contains(ghi[0].sql, "ON CONFLICT") {
		t.Fatalf("câu ghi ô không phải upsert: %s", ghi[0].sql)
	}
	if ghi[0].args[3] != nil {
		t.Fatalf("xoá ô mà ghi %v thay vì NULL", ghi[0].args[3])
	}
	if k.coCau("DELETE FROM") {
		t.Fatal("xoá ô bằng DELETE — luật 7 cấm #1")
	}
}

func TestSuaKhongDoiGiThiKhongGhiVaKhongCoVet(t *testing.T) {
	// A no-op is not an event. Recording it would fill a public authority's ledger with entries saying
	// nothing changed, and those bury the entries that carry legal weight. It is also what makes the
	// route's `idem.KhongCan` declaration true rather than hopeful.
	k := khoMau()
	uc, ctx := dungUseCaseNganSach(t, k)

	cu := domain.Dong(5_000_000) // exactly what the fixture already holds
	ten := "Tổng số"
	if _, err := uc.SuaKhoanMuc(ctx, idDongMau, YeuCauSuaKhoanMuc{
		Ten: &ten, GiaTri: map[string]*domain.Dong{"c-chi": &cu},
	}, nguoiGhi()); err != nil {
		t.Fatalf("SuaKhoanMuc lỗi: %v", err)
	}
	if k.coCau("INSERT INTO gia_tri_khoan_muc") || k.coCau("SET tt = $3") ||
		k.coCau("INSERT INTO audit_log") {
		t.Fatal("không có gì đổi mà vẫn ghi")
	}
	if k.daCommit != 1 {
		t.Fatalf("commit %d, muốn 1 — không ghi gì vẫn phải đóng giao dịch sạch", k.daCommit)
	}
}

func TestGhiVaoCotPhanTramBiTuChoi(t *testing.T) {
	// §9 rule 3: a percentage is computed at render and never stored. A stored one is a second home
	// for a derivable number, and the stale copy is the one that reaches the report.
	k := khoMau()
	uc, ctx := dungUseCaseNganSach(t, k)

	mot := domain.Dong(9127)
	_, err := uc.SuaKhoanMuc(ctx, idDongMau, YeuCauSuaKhoanMuc{
		GiaTri: map[string]*domain.Dong{"c-ty": &mot},
	}, nguoiGhi())
	if !errors.Is(err, domain.ErrCotKhongPhaiCotSo) {
		t.Fatalf("= %v, muốn ErrCotKhongPhaiCotSo", err)
	}
	if k.coCau("INSERT INTO gia_tri_khoan_muc") {
		t.Fatal("bị từ chối mà vẫn ghi ô")
	}
}

func TestGhiVaoCotCuaBangKhacBiTuChoi(t *testing.T) {
	// A figure filed against another sheet's column is a figure on no screen, sitting in the table
	// looking healthy. Nothing in the database catches it — `gia_tri_khoan_muc` has no foreign key.
	k := khoMau()
	uc, ctx := dungUseCaseNganSach(t, k)

	mot := domain.Dong(1_000)
	_, err := uc.SuaKhoanMuc(ctx, idDongMau, YeuCauSuaKhoanMuc{
		GiaTri: map[string]*domain.Dong{"c-cua-bang-khac": &mot},
	}, nguoiGhi())
	if !errors.Is(err, domain.ErrCotKhongThuocBang) {
		t.Fatalf("= %v, muốn ErrCotKhongThuocBang", err)
	}
	if k.coCau("INSERT INTO gia_tri_khoan_muc") {
		t.Fatal("bị từ chối mà vẫn ghi ô")
	}
}

// --- (6) `cach_tinh` follows the tree ------------------------------------------------------------------------

func TestThemConDauTienThiChaChuyenSangCongTuDongCon(t *testing.T) {
	// The customer's rule as a PROPERTY rather than as a field somebody has to keep consistent: a leaf
	// that gains its first child stops being typed into and starts summing, in the SAME transaction.
	k := khoMau()
	uc, ctx := dungUseCaseNganSach(t, k)

	if _, err := uc.ThemKhoanMuc(ctx, YeuCauThemKhoanMuc{
		BangID: idBangMau, ChaID: idDongKia, TT: "I", Ten: "Chi đầu tư phát triển", ThuTu: 3,
	}, nguoiGhi()); err != nil {
		t.Fatalf("ThemKhoanMuc lỗi: %v", err)
	}
	dat := k.cau("SET cach_tinh = $3")
	if len(dat) != 1 {
		t.Fatalf("có %d câu đặt cách tính cho cha, muốn 1", len(dat))
	}
	if !coGiaTri(dat[0], string(domain.TinhTheoCon)) {
		t.Fatalf("cha không chuyển sang `children`: %v", dat[0].args)
	}
	// The new line is a LEAF: `manual`, depth = parent + 1, and `is_headline` is not in the INSERT at
	// all — it takes the column default, which is what stops a client creating a second total row.
	chen := k.cau("INSERT INTO khoan_muc_ngan_sach")[0]
	if !coGiaTri(chen, string(domain.TinhTay)) {
		t.Fatalf("dòng mới không phải `manual`: %v", chen.args)
	}
	if strings.Contains(chen.sql, "is_headline") {
		t.Fatalf("câu chèn có nhắc `is_headline` — dòng mới không được là dòng tổng: %s", chen.sql)
	}
	if !coGiaTri(chen, int64(1)) {
		t.Fatalf("cấp của dòng mới không phải cấp cha + 1: %v", chen.args)
	}
}

func TestThemConVaoChaDaCongTuConThiKhongGhiLaiCachTinh(t *testing.T) {
	// The second child must not rewrite a column that already holds the right value: a no-op UPDATE
	// on a sheet with 59 rows is 59 writes nobody asked for, and it would show in the trail as a
	// change that did not happen.
	k := khoMau()
	k.khoanMuc[1].CachTinh = domain.TinhTheoCon
	k.khoanMuc = append(k.khoanMuc, domain.KhoanMucNganSach{
		ID: "k-con", BangID: idBangMau, ChaID: idDongKia, Ten: "Chi đầu tư", ThuTu: 3, Cap: 1,
	})
	uc, ctx := dungUseCaseNganSach(t, k)

	if _, err := uc.ThemKhoanMuc(ctx, YeuCauThemKhoanMuc{
		BangID: idBangMau, ChaID: idDongKia, Ten: "Chi an ninh", ThuTu: 4,
	}, nguoiGhi()); err != nil {
		t.Fatalf("ThemKhoanMuc lỗi: %v", err)
	}
	if k.coCau("SET cach_tinh = $3") {
		t.Fatal("cha đã `children` mà vẫn ghi lại cách tính")
	}
}

func TestChaNgoaiBangBiTuChoi(t *testing.T) {
	// Accepting it would put one year's line under another year's tree and total them together.
	k := khoMau()
	uc, ctx := dungUseCaseNganSach(t, k)

	_, err := uc.ThemKhoanMuc(ctx, YeuCauThemKhoanMuc{
		BangID: idBangMau, ChaID: "k-cua-bang-khac", Ten: "X", ThuTu: 9,
	}, nguoiGhi())
	if !errors.Is(err, domain.ErrChaKhongCungBang) {
		t.Fatalf("= %v, muốn ErrChaKhongCungBang", err)
	}
	if k.coCau("INSERT INTO khoan_muc_ngan_sach") {
		t.Fatal("cha ngoài bảng mà vẫn chèn")
	}
}

// --- (7) creating a sheet ---------------------------------------------------------------------------------------

func TestTaoBangDatMaTheoLanVaChenDuCotTrongMotGiaoDich(t *testing.T) {
	k := &khoNSGia{lanKeTiep: 2} // this year+kind has been loaded and removed once before
	uc, ctx := dungUseCaseNganSach(t, k)

	moi, err := uc.TaoBang(ctx, YeuCauTaoBang{
		Nam: 2026, Loai: domain.BangChi,
		TieuDe: "BÁO CÁO CHI NGÂN SÁCH NHÀ NƯỚC XÃ THĂNG BÌNH NĂM 2026", DonViTinh: "trieu-dong",
		Cot: []domain.CotNganSach{
			{Ten: "Dự toán năm", ThuTu: 1, Kieu: domain.CotSo, VaiTro: domain.VaiTroDuToanNam},
			{Ten: "Chi ngân sách", ThuTu: 2, Kieu: domain.CotSo, VaiTro: domain.VaiTroChiNganSach},
			{Ten: "So sánh TH/DT (%)", ThuTu: 3, Kieu: domain.CotPhanTram, CongThuc: "col_2 / col_1 * 100"},
		},
	}, nguoiGhi())
	if err != nil {
		t.Fatalf("TaoBang lỗi: %v", err)
	}
	// `NS-2026-CHI-02`, NOT `-01`: the previous load's code is taken forever, even though its sheet is
	// soft-deleted (rule 7, invariant 3). Reissuing it would put one code on two sheets in a table
	// where the old one is still there.
	if moi.Ma != "NS-2026-CHI-02" {
		t.Fatalf("mã bảng = %q, muốn NS-2026-CHI-02", moi.Ma)
	}
	if len(k.cau("INSERT INTO cot_ngan_sach")) != 3 {
		t.Fatalf("chèn %d cột, muốn 3", len(k.cau("INSERT INTO cot_ngan_sach")))
	}
	if k.batDau != 1 || k.daCommit != 1 {
		t.Fatalf("giao dịch: mở %d commit %d, muốn 1/1 — bảng và cột phải cùng một giao dịch",
			k.batDau, k.daCommit)
	}
	// An ordinary column's role goes in as NULL and not ''. `UNIQUE (tenant_id, bang_id, vai_tro)`
	// lets any number of NULLs coexist; two '' would COLLIDE, and every sheet would then be limited
	// to one column without a role — which is every sheet.
	// THE COLUMN STORES THE LABEL, not the wire code — the same bytes as migration 0006's default, so
	// legacy and new rows are one shape (domain.DonViTinh).
	chenBang := k.cau("INSERT INTO bang_ngan_sach")[0]
	if !coGiaTri(chenBang, "Triệu đồng") || coGiaTri(chenBang, "trieu-dong") {
		t.Fatalf("don_vi_tinh lưu không phải nhãn 'Triệu đồng': %v", chenBang.args)
	}
	phanTram := k.cau("INSERT INTO cot_ngan_sach")[2]
	if phanTram.args[7] != nil {
		t.Fatalf("vai trò của cột thường ghi %v thay vì NULL", phanTram.args[7])
	}
}

func TestDaCoBangConSongThiTuChoiVaKhongChenGi(t *testing.T) {
	// §6's `🗑 Gỡ` is how a year's sheet is replaced. A second live sheet would give the year two
	// answers with nothing on either screen saying which the report was built from.
	k := &khoNSGia{daCoBangConSong: true, lanKeTiep: 2}
	uc, ctx := dungUseCaseNganSach(t, k)

	_, err := uc.TaoBang(ctx, YeuCauTaoBang{
		Nam: 2026, Loai: domain.BangChi, TieuDe: "X", DonViTinh: "trieu-dong",
		Cot: []domain.CotNganSach{{Ten: "Chi ngân sách", ThuTu: 1, Kieu: domain.CotSo}},
	}, nguoiGhi())
	if !errors.Is(err, fistore.ErrBangDaTonTai) {
		t.Fatalf("= %v, muốn ErrBangDaTonTai", err)
	}
	if k.coCau("INSERT INTO bang_ngan_sach") || k.coCau("INSERT INTO audit_log") {
		t.Fatal("đã có bảng còn sống mà vẫn chèn")
	}
}

func TestHaiCotCungVaiTroBiTuChoiTruocKhiMoGiaoDich(t *testing.T) {
	// The shape is validated BEFORE the transaction opens: a request that fails its shape must never
	// hold a row lock while doing so, and the caller needs the reason rather than a rollback.
	k := &khoNSGia{lanKeTiep: 1}
	uc, ctx := dungUseCaseNganSach(t, k)

	_, err := uc.TaoBang(ctx, YeuCauTaoBang{
		Nam: 2026, Loai: domain.BangThu, TieuDe: "X", DonViTinh: "trieu-dong",
		Cot: []domain.CotNganSach{
			{Ten: "Thu xã hưởng", ThuTu: 1, Kieu: domain.CotSo, VaiTro: domain.VaiTroThuXaHuong},
			{Ten: "Thu xã hưởng (điều chỉnh)", ThuTu: 2, Kieu: domain.CotSo, VaiTro: domain.VaiTroThuXaHuong},
		},
	}, nguoiGhi())
	if !errors.Is(err, domain.ErrVaiTroTrungTrongBang) {
		t.Fatalf("= %v, muốn ErrVaiTroTrungTrongBang", err)
	}
	if k.batDau != 0 {
		t.Fatalf("mở %d giao dịch cho một yêu cầu sai hình dạng, muốn 0", k.batDau)
	}
}

func TestGoBangGhiDuBaCotVaVetMangMaBang(t *testing.T) {
	k := khoMau()
	uc, ctx := dungUseCaseNganSach(t, k)

	if err := uc.GoBang(ctx, idBangMau, "nạp lại từ tệp Phòng Tài chính đã sửa", nguoiGhi()); err != nil {
		t.Fatalf("GoBang lỗi: %v", err)
	}
	xoa := k.cau("UPDATE bang_ngan_sach")
	if len(xoa) != 1 {
		t.Fatalf("có %d câu xoá mềm bảng, muốn 1", len(xoa))
	}
	for _, manh := range []string{"deleted_at = now()", "deleted_by = $3", "delete_reason = $4"} {
		if !strings.Contains(xoa[0].sql, manh) {
			t.Errorf("câu xoá mềm bảng thiếu %q", manh)
		}
	}
	// THE TREE IS NOT TOUCHED. Every read reaches the lines THROUGH the sheet, so a removed sheet
	// takes its whole tree off every screen without a single extra row being written.
	if k.coCau("UPDATE khoan_muc_ngan_sach") {
		t.Fatal("gỡ bảng mà vẫn ghi vào khoản mục — một hành vi, một lần ghi")
	}
	if !coGiaTri(k.cau("INSERT INTO audit_log")[0], "NS-2026-CHI-01") {
		t.Fatal("vết gỡ bảng không mang mã bảng")
	}
}

func TestThieuNguoiThucHienThiKhongMoGiaoDich(t *testing.T) {
	// Rule 6 does not permit a business write whose trail cannot name its author. Refusing BEFORE the
	// transaction opens keeps the row's `deleted_by` and the entry telling the same story, and avoids
	// a rollback whose cause is a missing principal rather than anything about the budget.
	k := khoMau()
	uc, ctx := dungUseCaseNganSach(t, k)

	if _, err := uc.ThemKhoanMuc(ctx, YeuCauThemKhoanMuc{
		BangID: idBangMau, Ten: "X", ThuTu: 1,
	}, audit.Actor{}); err == nil {
		t.Fatal("không có chủ thể mà vẫn ghi được")
	}
	if k.batDau != 0 {
		t.Fatalf("mở %d giao dịch, muốn 0", k.batDau)
	}
}

// --- (8) editing a sheet's header: title, display unit, cut-off date -------------------------------------------

func chuoi(s string) *string { return &s }

func TestSuaBangGhiVaVetTrongCUNGGiaoDich(t *testing.T) {
	k := khoMau()
	uc, ctx := dungUseCaseNganSach(t, k)

	luyKe := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	sau, err := uc.SuaBang(ctx, idBangMau, YeuCauSuaBang{
		TieuDe: chuoi("BÁO CÁO CHI (đã sửa)"), DonViTinh: chuoi("nghin-dong"), LuyKeDen: &luyKe,
	}, nguoiGhi())
	if err != nil {
		t.Fatalf("SuaBang lỗi: %v", err)
	}
	if k.batDau != 1 || k.daCommit != 1 || k.daRollback != 0 {
		t.Fatalf("giao dịch: mở %d commit %d rollback %d, muốn 1/1/0", k.batDau, k.daCommit, k.daRollback)
	}
	sua := k.cau("UPDATE bang_ngan_sach")
	if len(sua) != 1 {
		t.Fatalf("có %d câu cập nhật bảng, muốn 1", len(sua))
	}
	for _, manh := range []string{"tenant_id = $1", "deleted_at IS NULL"} {
		if !strings.Contains(sua[0].sql, manh) {
			t.Errorf("câu cập nhật bảng thiếu %q: %s", manh, sua[0].sql)
		}
	}
	// THE LABEL IS STORED, the code is what the client sent.
	if !coGiaTri(sua[0], "Nghìn đồng") || !coGiaTri(sua[0], "BÁO CÁO CHI (đã sửa)") {
		t.Fatalf("giá trị cập nhật sai: %v", sua[0].args)
	}
	if sau.DonViTinh != "Nghìn đồng" || !sau.LuyKeDen.Equal(luyKe) {
		t.Fatalf("bảng trả về: đơn vị %q, luỹ kế %v", sau.DonViTinh, sau.LuyKeDen)
	}
	// The locked read excludes removed sheets and is tenant-scoped.
	doc := k.cau("FOR UPDATE")
	if len(doc) == 0 || !strings.Contains(doc[0].sql, "deleted_at IS NULL") ||
		!strings.Contains(doc[0].sql, "tenant_id = $1") {
		t.Fatalf("câu đọc khoá bảng không loại bảng đã gỡ / không mang tenant_id: %v", doc)
	}

	vet := k.cau("INSERT INTO audit_log")
	if len(vet) != 1 {
		t.Fatalf("có %d vết, muốn 1", len(vet))
	}
	if k.thuTuCua("UPDATE bang_ngan_sach") > k.thuTuCua("INSERT INTO audit_log") {
		t.Fatal("vết ghi trước câu cập nhật")
	}
	if !coGiaTri(vet[0], "NS-2026-CHI-01") || !coGiaTri(vet[0], maCanBo) || !coGiaTri(vet[0], HanhViSuaBangNganSach) {
		t.Fatalf("vết không mang mã bảng / mã cán bộ / hành vi: %v", vet[0].args)
	}
	// Before AND after of the unit, as the column holds it.
	for _, muon := range []string{`"truoc"`, `"sau"`, "Triệu đồng", "Nghìn đồng", "2026-09-30"} {
		if !coChuoiTrongDelta(vet[0], muon) {
			t.Errorf("delta thiếu %q", muon)
		}
	}
}

func TestSuaBangVetHongThiKhongGiCon(t *testing.T) {
	k := khoMau()
	k.loiSau = "INSERT INTO audit_log"
	uc, ctx := dungUseCaseNganSach(t, k)

	if _, err := uc.SuaBang(ctx, idBangMau, YeuCauSuaBang{TieuDe: chuoi("X")}, nguoiGhi()); err == nil {
		t.Fatal("vết hỏng mà SuaBang vẫn báo thành công")
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Fatalf("giao dịch: commit %d rollback %d, muốn 0/1", k.daCommit, k.daRollback)
	}
}

func TestSuaBangKhongDoiGiThiKhongGhiVaKhongCoVet(t *testing.T) {
	// The same values the sheet already holds — including `trieu-dong` against a stored
	// "Triệu đồng" — and an empty request: neither is an event.
	for ten, yc := range map[string]YeuCauSuaBang{
		"cùng giá trị": {TieuDe: chuoi("BÁO CÁO CHI NGÂN SÁCH NHÀ NƯỚC XÃ THĂNG BÌNH NĂM 2026"),
			DonViTinh: chuoi("trieu-dong"), LuyKeDen: &time.Time{}},
		"rỗng": {},
	} {
		t.Run(ten, func(t *testing.T) {
			k := khoMau()
			uc, ctx := dungUseCaseNganSach(t, k)
			if _, err := uc.SuaBang(ctx, idBangMau, yc, nguoiGhi()); err != nil {
				t.Fatalf("SuaBang lỗi: %v", err)
			}
			if k.coCau("UPDATE bang_ngan_sach") || k.coCau("INSERT INTO audit_log") {
				t.Fatal("không có gì đổi mà vẫn ghi")
			}
			if k.daCommit != 1 {
				t.Fatalf("commit %d, muốn 1", k.daCommit)
			}
		})
	}
}

func TestSuaBangDonViSaiHoacTieuDeRongKhongMoGiaoDich(t *testing.T) {
	for ten, tc := range map[string]struct {
		yc   YeuCauSuaBang
		muon error
	}{
		"nhãn thay vì mã": {YeuCauSuaBang{DonViTinh: chuoi("Triệu đồng")}, domain.ErrDonViTinhSai},
		"tỷ đồng":         {YeuCauSuaBang{DonViTinh: chuoi("ty-dong")}, domain.ErrDonViTinhSai},
		"tiêu đề rỗng":    {YeuCauSuaBang{TieuDe: chuoi("  ")}, domain.ErrThieuTieuDe},
	} {
		t.Run(ten, func(t *testing.T) {
			k := khoMau()
			uc, ctx := dungUseCaseNganSach(t, k)
			if _, err := uc.SuaBang(ctx, idBangMau, tc.yc, nguoiGhi()); !errors.Is(err, tc.muon) {
				t.Fatalf("= %v, muốn %v", err, tc.muon)
			}
			if k.batDau != 0 {
				t.Fatalf("mở %d giao dịch cho yêu cầu sai hình dạng, muốn 0", k.batDau)
			}
		})
	}
}

func TestSuaBangDaGoHoacKhongCoThiKhongGhiGi(t *testing.T) {
	k := khoMau()
	k.bang = nil // the locked read found no LIVE sheet of this commune
	uc, ctx := dungUseCaseNganSach(t, k)

	_, err := uc.SuaBang(ctx, idBangMau, YeuCauSuaBang{TieuDe: chuoi("X")}, nguoiGhi())
	if !errors.Is(err, fistore.ErrKhongThayBangNganSach) {
		t.Fatalf("= %v, muốn ErrKhongThayBangNganSach", err)
	}
	if k.coCau("UPDATE bang_ngan_sach") || k.coCau("INSERT INTO audit_log") {
		t.Fatal("bảng không tồn tại mà vẫn ghi")
	}
}

func TestSuaBangChiTieuDeThiGiuDonViVaLuyKe(t *testing.T) {
	k := khoMau()
	uc, ctx := dungUseCaseNganSach(t, k)

	if _, err := uc.SuaBang(ctx, idBangMau, YeuCauSuaBang{TieuDe: chuoi("TIÊU ĐỀ MỚI")}, nguoiGhi()); err != nil {
		t.Fatalf("SuaBang lỗi: %v", err)
	}
	sua := k.cau("UPDATE bang_ngan_sach")
	if len(sua) != 1 {
		t.Fatalf("có %d câu cập nhật, muốn 1", len(sua))
	}
	// args: tenant, id, tieu_de, don_vi_tinh, luy_ke_den
	if sua[0].args[3] != "Triệu đồng" || sua[0].args[4] != nil {
		t.Fatalf("sửa tiêu đề mà đơn vị/luỹ kế đổi theo: %v", sua[0].args)
	}
}

func TestSuaBangChuTuDoCuSangMaThiGhiVaGiuChuCuTrongVet(t *testing.T) {
	// A legacy sheet holding free text set to a code DOES change — its column becomes the label and
	// the screen prints differently from that moment — so it is written and the old text recorded.
	k := khoMau()
	k.bang.donViTinh = "tr.đồng"
	uc, ctx := dungUseCaseNganSach(t, k)

	if _, err := uc.SuaBang(ctx, idBangMau, YeuCauSuaBang{DonViTinh: chuoi("trieu-dong")}, nguoiGhi()); err != nil {
		t.Fatalf("SuaBang lỗi: %v", err)
	}
	if !k.coCau("UPDATE bang_ngan_sach") {
		t.Fatal("chữ cũ đổi sang nhãn chuẩn mà không ghi")
	}
	if !coChuoiTrongDelta(k.cau("INSERT INTO audit_log")[0], "tr.đồng") {
		t.Fatal("vết không giữ chữ đơn vị cũ")
	}
}

// --- helpers ------------------------------------------------------------------------------------------------

// thuTuCua is the index of the first recorded statement containing `tu`, or -1.
func (k *khoNSGia) thuTuCua(tu string) int {
	k.mu.Lock()
	defer k.mu.Unlock()
	for i, l := range k.lenh {
		if strings.Contains(l.sql, tu) {
			return i
		}
	}
	return -1
}

func coGiaTri(l lenhGhi, muon any) bool {
	for _, a := range l.args {
		if a == muon {
			return true
		}
	}
	return false
}

// coChuoiTrongDelta looks inside the audit entry's JSON delta, which arrives as a []byte argument.
func coChuoiTrongDelta(l lenhGhi, muon string) bool {
	for _, a := range l.args {
		b, ok := a.([]byte)
		if ok && strings.Contains(string(b), muon) {
			return true
		}
	}
	return false
}
