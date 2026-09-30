package app

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/service-finance/internal/domain"
)

// WHAT THIS FILE IS FOR: the batches (migration 0008) and the `entries` calculation mode, through the
// REAL store on the fake driver (driver_gia_ngan_sach_test.go says what that does and does not prove).
//
// THE PROPERTIES, each a defect that fails silently:
//
//  1. a batch, its amounts and its audit entry are ONE transaction; a failed entry takes all down;
//  2. `don_vi_ca_nhan` (may be a citizen's name) NEVER enters the audit delta — only presence/length;
//  3. every refusal — non-leaf, % column, foreign column, over-long text — writes nothing;
//  4. writing a batch does NOT switch the mode;
//  5. every batch sum joins `dot_thu_chi.deleted_at IS NULL`;
//  6. entries -> manual copies the sums into the cells (NULL where empty), same transaction, audited;
//     manual -> entries leaves the typed cells alone;
//  7. a line in `entries` mode with live batches cannot gain a child; without batches it flips.

// donViMau is a counterparty that looks like what a commune really types. The assertion is that
// these exact characters never reach the audit ledger.
const donViMau = "Hộ ông Nguyễn Văn Mẫu"

func ngayDot() time.Time { return time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC) }

func dong(g int64) *domain.Dong { d := domain.Dong(g); return &d }

func yeuCauDotMau() YeuCauGhiDot {
	return YeuCauGhiDot{
		KhoanMucID: idDongKia, Ngay: ngayDot(), NoiDung: "Thu tiền sử dụng đất đợt 2",
		DonViCaNhan: donViMau, SoChungTu: "PT-0042",
		GiaTri: map[string]*domain.Dong{"c-chi": dong(1_500_000), "c-dt": nil},
	}
}

// --- (1)(2)(4) the write ------------------------------------------------------------------------------

func TestGhiDotMotGiaoDichVaVetKhongMangDonViCaNhan(t *testing.T) {
	k := khoMau()
	uc, ctx := dungUseCaseNganSach(t, k)

	moi, err := uc.GhiDot(ctx, yeuCauDotMau(), nguoiGhi())
	if err != nil {
		t.Fatalf("GhiDot lỗi: %v", err)
	}
	if k.batDau != 1 || k.daCommit != 1 || k.daRollback != 0 {
		t.Fatalf("giao dịch: mở %d commit %d rollback %d, muốn 1/1/0", k.batDau, k.daCommit, k.daRollback)
	}
	chen := k.cau("INSERT INTO dot_thu_chi")
	if len(chen) != 1 {
		t.Fatalf("có %d câu chèn đợt, muốn 1", len(chen))
	}
	// nguoi_ghi_ma is the STAFF CODE (rule 6, invariant 8), and the counterparty goes in as a bound
	// parameter — the one place it may live.
	if !coGiaTri(chen[0], maCanBo) || !coGiaTri(chen[0], donViMau) {
		t.Fatalf("câu chèn đợt thiếu mã cán bộ hoặc đơn vị: %v", chen[0].args)
	}
	// A nil amount writes NO row: "a column with no row is empty too" (0008).
	so := k.cau("INSERT INTO gia_tri_dot")
	if len(so) != 1 || !coGiaTri(so[0], int64(1_500_000)) || !coGiaTri(so[0], "c-chi") {
		t.Fatalf("số tiền đợt ghi sai: %v", so)
	}
	if moi.GiaTri["c-chi"] != 1_500_000 || len(moi.GiaTri) != 1 {
		t.Fatalf("đợt trả về mang số tiền %v", moi.GiaTri)
	}

	vet := k.cau("INSERT INTO audit_log")
	if len(vet) != 1 {
		t.Fatalf("có %d vết, muốn 1", len(vet))
	}
	if k.thuTuCua("INSERT INTO gia_tri_dot") > k.thuTuCua("INSERT INTO audit_log") {
		t.Fatal("vết ghi trước số tiền đợt")
	}
	if !coGiaTri(vet[0], "NS-2026-CHI-01") || !coGiaTri(vet[0], maCanBo) || !coGiaTri(vet[0], HanhViGhiDotThuChi) {
		t.Fatalf("vết không mang mã bảng / mã cán bộ / hành vi: %v", vet[0].args)
	}
	// RULE 6 FORBIDDEN #4 / RULE 3: the ledger records THAT a counterparty was stated, never what.
	if coChuoiTrongDelta(vet[0], donViMau) || coChuoiTrongDelta(vet[0], "Nguyễn Văn") {
		t.Fatal("delta vết chứa nguyên văn `đơn vị, cá nhân` — sổ kiểm toán thành kho dữ liệu cá nhân")
	}
	for _, muon := range []string{`"co_don_vi_ca_nhan":true`, `"do_dai_don_vi_ca_nhan":21`, "PT-0042", "2026-08-20"} {
		if !coChuoiTrongDelta(vet[0], muon) {
			t.Errorf("delta thiếu %q", muon)
		}
	}
	// (4) NO AUTO-SWITCH: the user chooses the mode (§4.2, decision 25/09/2026).
	if k.coCau("SET cach_tinh = $3") {
		t.Fatal("ghi đợt mà tự đổi cách tính của khoản mục")
	}
}

func TestVetHongThiDotVaSoTienCungKhongCon(t *testing.T) {
	k := khoMau()
	k.loiSau = "INSERT INTO audit_log"
	uc, ctx := dungUseCaseNganSach(t, k)

	if _, err := uc.GhiDot(ctx, yeuCauDotMau(), nguoiGhi()); err == nil {
		t.Fatal("vết hỏng mà GhiDot vẫn báo thành công")
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Fatalf("giao dịch: commit %d rollback %d, muốn 0/1", k.daCommit, k.daRollback)
	}
}

// --- (3) refusals write nothing ----------------------------------------------------------------------

func TestGhiDotVaoKhoanMucCoConBiTuChoi(t *testing.T) {
	k := khoMau()
	k.khoanMuc = append(k.khoanMuc, domain.KhoanMucNganSach{
		ID: "k-con", BangID: idBangMau, ChaID: idDongKia, Ten: "Chi đầu tư", ThuTu: 3, Cap: 1,
	})
	uc, ctx := dungUseCaseNganSach(t, k)

	_, err := uc.GhiDot(ctx, yeuCauDotMau(), nguoiGhi())
	if !errors.Is(err, domain.ErrDotChiGhiVaoLa) {
		t.Fatalf("= %v, muốn ErrDotChiGhiVaoLa", err)
	}
	if k.coCau("INSERT INTO dot_thu_chi") || k.coCau("INSERT INTO audit_log") || k.daCommit != 0 {
		t.Fatal("bị từ chối mà vẫn ghi")
	}
}

func TestGhiDotVaoCotPhanTramHoacCotBangKhacBiTuChoi(t *testing.T) {
	for ten, tc := range map[string]struct {
		cot  string
		muon error
	}{
		"cột phần trăm": {"c-ty", domain.ErrCotKhongPhaiCotSo},
		"cột bảng khác": {"c-cua-bang-khac", domain.ErrCotKhongThuocBang},
	} {
		t.Run(ten, func(t *testing.T) {
			k := khoMau()
			uc, ctx := dungUseCaseNganSach(t, k)
			yc := yeuCauDotMau()
			yc.GiaTri[tc.cot] = dong(10)
			if _, err := uc.GhiDot(ctx, yc, nguoiGhi()); !errors.Is(err, tc.muon) {
				t.Fatalf("= %v, muốn %v", err, tc.muon)
			}
			if k.coCau("INSERT INTO dot_thu_chi") || k.coCau("INSERT INTO gia_tri_dot") {
				t.Fatal("bị từ chối mà vẫn chèn")
			}
		})
	}
}

func TestGhiDotTranKyTuDemTheoRuneVaKhongMoGiaoDich(t *testing.T) {
	// `ệ` is THREE bytes. 1000 of them is 3000 bytes and must be ACCEPTED — a byte count would refuse
	// it, which is the bug a Vietnamese commune meets first.
	vua := strings.Repeat("ệ", domain.NoiDungChungTuToiDa)
	k := khoMau()
	uc, ctx := dungUseCaseNganSach(t, k)
	yc := yeuCauDotMau()
	yc.NoiDung = vua
	if _, err := uc.GhiDot(ctx, yc, nguoiGhi()); err != nil {
		t.Fatalf("nội dung đúng %d ký tự (3 byte mỗi ký tự) bị từ chối: %v", domain.NoiDungChungTuToiDa, err)
	}

	for ten, tc := range map[string]struct {
		sua  func(*YeuCauGhiDot)
		muon error
	}{
		"content quá dài":  {func(y *YeuCauGhiDot) { y.NoiDung = vua + "ệ" }, domain.ErrNoiDungDotQuaDai},
		"content rỗng":     {func(y *YeuCauGhiDot) { y.NoiDung = "   " }, domain.ErrThieuNoiDungDot},
		"counterparty dài": {func(y *YeuCauGhiDot) { y.DonViCaNhan = strings.Repeat("ễ", domain.DoiTacToiDa+1) }, domain.ErrDonViCaNhanQuaDai},
		"document_no dài":  {func(y *YeuCauGhiDot) { y.SoChungTu = strings.Repeat("ố", domain.SoChungTuToiDa+1) }, domain.ErrSoChungTuDotQuaDai},
		"thiếu ngày":       {func(y *YeuCauGhiDot) { y.Ngay = time.Time{} }, domain.ErrThieuNgayDot},
		"năm 1999":         {func(y *YeuCauGhiDot) { y.Ngay = time.Date(1999, 12, 31, 0, 0, 0, 0, time.UTC) }, domain.ErrNgayDotNgoaiLich},
		"không có số nào":  {func(y *YeuCauGhiDot) { y.GiaTri = map[string]*domain.Dong{"c-chi": nil} }, domain.ErrDotKhongCoSoTienNao},
		"số vượt chặn gõ nhầm": {func(y *YeuCauGhiDot) {
			y.GiaTri = map[string]*domain.Dong{"c-chi": dong(int64(domain.GiaTriToiDa) + 1)}
		}, domain.ErrGiaTriQuaLon},
	} {
		t.Run(ten, func(t *testing.T) {
			k := khoMau()
			uc, ctx := dungUseCaseNganSach(t, k)
			yc := yeuCauDotMau()
			tc.sua(&yc)
			_, err := uc.GhiDot(ctx, yc, nguoiGhi())
			if !errors.Is(err, tc.muon) {
				t.Fatalf("= %v, muốn %v", err, tc.muon)
			}
			if k.batDau != 0 {
				t.Fatalf("mở %d giao dịch cho yêu cầu sai hình dạng, muốn 0", k.batDau)
			}
			// The refusal names the field and the limit — never the value (rule 3, forbidden #3).
			if strings.Contains(err.Error(), donViMau) {
				t.Fatal("câu lỗi chứa nguyên văn `đơn vị, cá nhân`")
			}
		})
	}
}

// --- removal --------------------------------------------------------------------------------------------

func dotSong() *domain.DotThuChi {
	return &domain.DotThuChi{
		ID: "01JDOTTHUCHIMAU00000000000", KhoanMucID: idDongKia, Ngay: ngayDot(),
		NoiDung: "Thu tiền sử dụng đất đợt 2", DonViCaNhan: donViMau, SoChungTu: "PT-0042",
		NguoiGhiMa: "CB-00007", TaoLuc: time.Date(2026, 8, 20, 9, 0, 0, 0, time.UTC),
		GiaTri: map[string]domain.Dong{"c-chi": 1_500_000},
	}
}

func TestGoDotGhiDuBaCotVaVetKhongMangDonViCaNhan(t *testing.T) {
	k := khoMau()
	k.dot = dotSong()
	uc, ctx := dungUseCaseNganSach(t, k)

	if err := uc.GoDot(ctx, k.dot.ID, "ghi nhầm số chứng từ", nguoiGhi()); err != nil {
		t.Fatalf("GoDot lỗi: %v", err)
	}
	xoa := k.cau("UPDATE dot_thu_chi")
	if len(xoa) != 1 {
		t.Fatalf("có %d câu xoá mềm đợt, muốn 1", len(xoa))
	}
	for _, manh := range []string{"deleted_at = now()", "deleted_by = $3", "delete_reason = $4",
		"deleted_at IS NULL", "tenant_id = $1"} {
		if !strings.Contains(xoa[0].sql, manh) {
			t.Errorf("câu xoá mềm đợt thiếu %q: %s", manh, xoa[0].sql)
		}
	}
	if !coGiaTri(xoa[0], maCanBo) {
		t.Fatalf("deleted_by không phải mã cán bộ: %v", xoa[0].args)
	}
	if k.coCau("DELETE ") || k.coCau("UPDATE gia_tri_dot") {
		t.Fatal("gỡ đợt mà đụng tới số tiền hoặc xoá cứng")
	}
	vet := k.cau("INSERT INTO audit_log")
	if len(vet) != 1 || !coGiaTri(vet[0], HanhViGoDotThuChi) || !coGiaTri(vet[0], "NS-2026-CHI-01") {
		t.Fatalf("vết gỡ đợt sai: %v", vet)
	}
	if coChuoiTrongDelta(vet[0], donViMau) {
		t.Fatal("delta vết gỡ chứa nguyên văn `đơn vị, cá nhân`")
	}
	if !coChuoiTrongDelta(vet[0], "ghi nhầm số chứng từ") || !coChuoiTrongDelta(vet[0], "1500000") {
		t.Fatal("vết gỡ thiếu lý do hoặc số tiền trước khi gỡ")
	}
	if k.daCommit != 1 {
		t.Fatalf("commit %d, muốn 1", k.daCommit)
	}
}

func TestGoDotKhongCoHoacKhoanMucDaGoThiKhongGhiGi(t *testing.T) {
	for ten, sua := range map[string]func(*khoNSGia){
		"không có đợt sống": func(k *khoNSGia) { k.dot = nil },
		"khoản mục đã gỡ":   func(k *khoNSGia) { k.dot = dotSong(); k.bangIDCuaKhoanMuc = "" },
	} {
		t.Run(ten, func(t *testing.T) {
			k := khoMau()
			sua(k)
			uc, ctx := dungUseCaseNganSach(t, k)
			err := uc.GoDot(ctx, "01JDOTTHUCHIMAU00000000000", "x", nguoiGhi())
			if !errors.Is(err, domain.ErrKhongThayDot) {
				t.Fatalf("= %v, muốn ErrKhongThayDot", err)
			}
			if k.coCau("UPDATE dot_thu_chi") || k.coCau("INSERT INTO audit_log") {
				t.Fatal("không có đợt mà vẫn ghi")
			}
		})
	}
}

// --- (5) the sum counts live batches only -----------------------------------------------------------------

func TestTongDotChiCongDotConSong(t *testing.T) {
	// The fake cannot evaluate SQL, so the property is asserted where it lives: the statement text.
	// 0008 question 4a: "a sum that forgets the join counts removed batches".
	k := khoMau()
	uc, ctx := dungUseCaseNganSach(t, k)
	if _, err := uc.GhiDot(ctx, yeuCauDotMau(), nguoiGhi()); err != nil {
		t.Fatalf("GhiDot lỗi: %v", err)
	}
	tong := k.cau("SUM(g.gia_tri)")
	if len(tong) != 1 {
		t.Fatalf("có %d câu tổng đợt, muốn 1 (đọc dưới khoá bảng)", len(tong))
	}
	for _, manh := range []string{"d.deleted_at IS NULL", "k.deleted_at IS NULL", "g.gia_tri IS NOT NULL",
		"g.tenant_id = $1", "d.tenant_id = g.tenant_id", "k.tenant_id = d.tenant_id", "GROUP BY"} {
		if !strings.Contains(tong[0].sql, manh) {
			t.Errorf("câu tổng đợt thiếu %q", manh)
		}
	}
}

// --- (6) switching the mode ---------------------------------------------------------------------------------

func cachTinh(c domain.CachTinh) *domain.CachTinh { return &c }

func TestDoiTheoDotSangNhapTayChepTongVaoOTrongCungGiaoDich(t *testing.T) {
	k := khoMau()
	k.khoanMuc[1].CachTinh = domain.TinhTheoDot
	// An OLD typed figure in `c-dt`, from before the entries period, and a batch sum in `c-chi` only.
	k.gia[idDongKia] = map[string]domain.Dong{"c-dt": 999}
	k.giaDot = map[string]map[string]domain.Dong{idDongKia: {"c-chi": 7_000}}
	uc, ctx := dungUseCaseNganSach(t, k)

	sau, err := uc.SuaKhoanMuc(ctx, idDongKia, YeuCauSuaKhoanMuc{CachTinh: cachTinh(domain.TinhTay)}, nguoiGhi())
	if err != nil {
		t.Fatalf("SuaKhoanMuc lỗi: %v", err)
	}
	if sau.CachTinh != domain.TinhTay {
		t.Fatalf("cách tính trả về %q", sau.CachTinh)
	}
	ghi := k.cau("INSERT INTO gia_tri_khoan_muc")
	if len(ghi) != 2 {
		t.Fatalf("có %d câu ghi ô, muốn 2 (c-dt về NULL, c-chi nhận tổng)", len(ghi))
	}
	var coTong, coNull bool
	for _, g := range ghi {
		if coGiaTri(g, "c-chi") && coGiaTri(g, int64(7_000)) {
			coTong = true
		}
		if coGiaTri(g, "c-dt") && g.args[3] == nil {
			coNull = true
		}
	}
	if !coTong {
		t.Fatal("ô c-chi không nhận tổng các đợt (§9.1)")
	}
	// The screen showed `—` in c-dt; the old typed 999 must NOT come back as the starting value.
	if !coNull {
		t.Fatal("ô c-dt giữ số gõ tay cũ thay vì trống như màn hình vừa hiện")
	}
	dat := k.cau("SET cach_tinh = $3")
	if len(dat) != 1 || !coGiaTri(dat[0], string(domain.TinhTay)) {
		t.Fatalf("câu đặt cách tính sai: %v", dat)
	}
	if k.batDau != 1 || k.daCommit != 1 {
		t.Fatalf("giao dịch: mở %d commit %d, muốn 1/1", k.batDau, k.daCommit)
	}
	vet := k.cau("INSERT INTO audit_log")
	if len(vet) != 1 {
		t.Fatalf("có %d vết, muốn 1", len(vet))
	}
	for _, muon := range []string{`"cach_tinh"`, `"entries"`, `"manual"`, `"o_lay_tu_tong_dot"`, "7000", "999"} {
		if !coChuoiTrongDelta(vet[0], muon) {
			t.Errorf("delta thiếu %q", muon)
		}
	}
}

func TestDoiNhapTaySangTheoDotGiuNguyenOGoTay(t *testing.T) {
	k := khoMau()
	uc, ctx := dungUseCaseNganSach(t, k)

	if _, err := uc.SuaKhoanMuc(ctx, idDongMau, YeuCauSuaKhoanMuc{CachTinh: cachTinh(domain.TinhTheoDot)}, nguoiGhi()); err != nil {
		t.Fatalf("SuaKhoanMuc lỗi: %v", err)
	}
	if k.coCau("INSERT INTO gia_tri_khoan_muc") {
		t.Fatal("manual -> entries mà ghi đè ô gõ tay — ô phải giữ nguyên, chỉ thôi hiển thị")
	}
	dat := k.cau("SET cach_tinh = $3")
	if len(dat) != 1 || !coGiaTri(dat[0], string(domain.TinhTheoDot)) {
		t.Fatalf("câu đặt cách tính sai: %v", dat)
	}
	if !coChuoiTrongDelta(k.cau("INSERT INTO audit_log")[0], `"entries"`) {
		t.Fatal("vết không ghi cách tính mới")
	}
}

func TestDoiCachTinhBiTuChoiKhongGhiGi(t *testing.T) {
	for ten, tc := range map[string]struct {
		sua  func(*khoNSGia)
		id   string
		yc   YeuCauSuaKhoanMuc
		muon error
	}{
		"children từ client": {func(*khoNSGia) {}, idDongMau,
			YeuCauSuaKhoanMuc{CachTinh: cachTinh(domain.TinhTheoCon)}, domain.ErrCachTinhDoTuClient},
		"khoản mục có con": {func(k *khoNSGia) {
			k.khoanMuc[1].CachTinh = domain.TinhTheoCon
			k.khoanMuc = append(k.khoanMuc, domain.KhoanMucNganSach{
				ID: "k-con", BangID: idBangMau, ChaID: idDongKia, Ten: "Chi đầu tư", ThuTu: 3, Cap: 1})
		}, idDongKia, YeuCauSuaKhoanMuc{CachTinh: cachTinh(domain.TinhTheoDot)}, domain.ErrKhoanMucChaKhongDoiCachTinh},
		"gõ số vào khoản mục theo đợt": {func(k *khoNSGia) { k.khoanMuc[1].CachTinh = domain.TinhTheoDot },
			idDongKia, YeuCauSuaKhoanMuc{GiaTri: map[string]*domain.Dong{"c-chi": dong(1)}},
			domain.ErrKhoanMucTheoDotKhongGoThang},
		"đổi sang theo đợt kèm gõ số": {func(*khoNSGia) {}, idDongMau,
			YeuCauSuaKhoanMuc{CachTinh: cachTinh(domain.TinhTheoDot), GiaTri: map[string]*domain.Dong{"c-chi": dong(1)}},
			domain.ErrKhoanMucTheoDotKhongGoThang},
	} {
		t.Run(ten, func(t *testing.T) {
			k := khoMau()
			tc.sua(k)
			uc, ctx := dungUseCaseNganSach(t, k)
			if _, err := uc.SuaKhoanMuc(ctx, tc.id, tc.yc, nguoiGhi()); !errors.Is(err, tc.muon) {
				t.Fatalf("= %v, muốn %v", err, tc.muon)
			}
			if k.coCau("SET cach_tinh = $3") || k.coCau("INSERT INTO gia_tri_khoan_muc") ||
				k.coCau("INSERT INTO audit_log") {
				t.Fatal("bị từ chối mà vẫn ghi")
			}
		})
	}
}

func TestDoiSangNhapTayRoiGoSoTrongCungYeuCau(t *testing.T) {
	// "Switch to manual and type a figure" in one PATCH: the hand-over happens first, then the typed
	// figure overrides the copied sum — and it is compared against the copied sum, not the old cell.
	k := khoMau()
	k.khoanMuc[1].CachTinh = domain.TinhTheoDot
	k.giaDot = map[string]map[string]domain.Dong{idDongKia: {"c-chi": 7_000}}
	uc, ctx := dungUseCaseNganSach(t, k)

	if _, err := uc.SuaKhoanMuc(ctx, idDongKia, YeuCauSuaKhoanMuc{
		CachTinh: cachTinh(domain.TinhTay), GiaTri: map[string]*domain.Dong{"c-chi": dong(8_000)},
	}, nguoiGhi()); err != nil {
		t.Fatalf("SuaKhoanMuc lỗi: %v", err)
	}
	ghi := k.cau("INSERT INTO gia_tri_khoan_muc")
	if len(ghi) != 2 || !coGiaTri(ghi[0], int64(7_000)) || !coGiaTri(ghi[1], int64(8_000)) {
		t.Fatalf("thứ tự ghi ô sai — muốn tổng đợt rồi số gõ: %v", ghi)
	}
}

// --- (7) adding a child under an `entries` line ----------------------------------------------------------------

func TestThemConVaoKhoanMucTheoDotConDotBiTuChoi(t *testing.T) {
	k := khoMau()
	k.khoanMuc[1].CachTinh = domain.TinhTheoDot
	k.dotConSong = true
	uc, ctx := dungUseCaseNganSach(t, k)

	_, err := uc.ThemKhoanMuc(ctx, YeuCauThemKhoanMuc{
		BangID: idBangMau, ChaID: idDongKia, TT: "1", Ten: "Chi con", ThuTu: 3,
	}, nguoiGhi())
	if !errors.Is(err, domain.ErrKhoanMucTheoDotConDot) {
		t.Fatalf("= %v, muốn ErrKhoanMucTheoDotConDot", err)
	}
	if k.coCau("INSERT INTO khoan_muc_ngan_sach") || k.coCau("SET cach_tinh = $3") {
		t.Fatal("bị từ chối mà vẫn chèn / đổi cách tính")
	}
	// The refusal tells the user the two ways out.
	if !strings.Contains(err.Error(), "gỡ các đợt") || !strings.Contains(err.Error(), "manual") {
		t.Fatalf("câu từ chối không nói cách gỡ: %q", err.Error())
	}
	// The existence check is tenant-bound and counts LIVE batches only.
	kiem := k.cau("SELECT EXISTS (SELECT 1 FROM dot_thu_chi")
	if len(kiem) != 1 || !strings.Contains(kiem[0].sql, "deleted_at IS NULL") ||
		!strings.Contains(kiem[0].sql, "tenant_id = $1") {
		t.Fatalf("câu kiểm đợt còn sống sai: %v", kiem)
	}
}

func TestThemConVaoKhoanMucTheoDotKhongConDotThiThanhChildren(t *testing.T) {
	k := khoMau()
	k.khoanMuc[1].CachTinh = domain.TinhTheoDot
	uc, ctx := dungUseCaseNganSach(t, k)

	if _, err := uc.ThemKhoanMuc(ctx, YeuCauThemKhoanMuc{
		BangID: idBangMau, ChaID: idDongKia, TT: "1", Ten: "Chi con", ThuTu: 3,
	}, nguoiGhi()); err != nil {
		t.Fatalf("ThemKhoanMuc lỗi: %v", err)
	}
	dat := k.cau("SET cach_tinh = $3")
	if len(dat) != 1 || !coGiaTri(dat[0], string(domain.TinhTheoCon)) {
		t.Fatalf("cha không chuyển sang `children`: %v", dat)
	}
	if !coChuoiTrongDelta(k.cau("INSERT INTO audit_log")[0], `"cach_tinh_truoc":"entries"`) {
		t.Fatal("vết không ghi cha trước đó là `entries`")
	}
}
