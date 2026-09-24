package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// The read half of the `entries` mode: which source a leaf's figure comes from, and that everything
// above it — parents, the summary row, the indicators — picks the batch sum up.

// bangTheoDot is a chi sheet: `Tổng số` (marked) is a PARENT of two leaves — one `manual`, one
// `entries`. The `entries` leaf ALSO carries an old typed figure, which must never be shown.
func bangTheoDot() BangDayDu {
	return BangDayDu{
		Bang: BangNganSach{ID: "b", Ma: "NS-2026-CHI-01", Nam: 2026, Loai: BangChi},
		Cot: []CotNganSach{
			{ID: "c-dt", Ten: "Dự toán năm", ThuTu: 1, Kieu: CotSo, VaiTro: VaiTroDuToanNam},
			{ID: "c-chi", Ten: "Chi ngân sách", ThuTu: 2, Kieu: CotSo, VaiTro: VaiTroChiNganSach},
		},
		KhoanMuc: []KhoanMucNganSach{
			{ID: "tong", Ten: "Tổng số", ThuTu: 1, CachTinh: TinhTheoCon, LaDongTong: true},
			{ID: "tay", ChaID: "tong", Ten: "Chi thường xuyên", ThuTu: 2, CachTinh: TinhTay, Cap: 1},
			{ID: "dot", ChaID: "tong", Ten: "Chi sự nghiệp", ThuTu: 3, CachTinh: TinhTheoDot, Cap: 1},
		},
		Gia: map[string]map[string]Dong{
			"tay": {"c-dt": 1_000, "c-chi": 400},
			"dot": {"c-dt": 9_999_999, "c-chi": 8_888_888}, // stale typed figures — locked, not shown
		},
		GiaDot: map[string]map[string]Dong{
			"dot": {"c-chi": 600},                    // c-dt: no live batch stated it -> EMPTY
			"tay": {"c-dt": 77_777, "c-chi": 77_777}, // batches on a MANUAL leaf count nowhere
		},
	}
}

// giaVaCo reads one cell as (figure, present) and FAILS the test when the cell is unavailable — so a
// test written for a figure cannot pass on an unavailable cell by reading its zero value.
func giaVaCo(t *testing.T, b BangDayDu, khoanMucID, cotID string) (Dong, bool) {
	t.Helper()
	s := b.GiaTri(khoanMucID, cotID)
	if s.LyDo != "" {
		t.Fatalf("ô %s/%s không tính được: %s", khoanMucID, cotID, s.LyDo)
	}
	return s.Gia, s.Co
}

func TestGiaTri_LaTheoDotDocTongDotKhongDocOGoTay(t *testing.T) {
	b := bangTheoDot()
	if g, co := giaVaCo(t, b, "dot", "c-chi"); !co || g != 600 {
		t.Fatalf("lá `entries` = %d,%v — muốn tổng đợt 600", g, co)
	}
	// EMPTY, NOT 0 AND NOT THE STALE 9 999 999 (§9 rule 4, §9.1).
	if g, co := giaVaCo(t, b, "dot", "c-dt"); co {
		t.Fatalf("lá `entries` không có đợt nào ở c-dt mà vẫn ra %d", g)
	}
}

func TestGiaTri_LaNhapTayBoQuaTongDot(t *testing.T) {
	b := bangTheoDot()
	if g, co := giaVaCo(t, b, "tay", "c-chi"); !co || g != 400 {
		t.Fatalf("lá `manual` = %d,%v — muốn số gõ tay 400, không phải tổng đợt", g, co)
	}
}

func TestGiaTri_ChaCongCaLaTheoDot(t *testing.T) {
	b := bangTheoDot()
	if g, co := giaVaCo(t, b, "tong", "c-chi"); !co || g != 1_000 {
		t.Fatalf("cha = %d,%v — muốn 400 (tay) + 600 (tổng đợt) = 1000", g, co)
	}
	// c-dt: the manual leaf's 1000 + the entries leaf's EMPTY = 1000. Not 1000 + 9 999 999.
	if g, co := giaVaCo(t, b, "tong", "c-dt"); !co || g != 1_000 {
		t.Fatalf("cha c-dt = %d,%v — muốn 1000; số gõ tay đã khoá của lá `entries` lọt vào tổng", g, co)
	}
}

func TestChiSo_DungSoCuaLaTheoDot(t *testing.T) {
	// Chi đạt dự toán = Chi ngân sách / Dự toán năm on the marked row = 1000 / 1000 = 100,00%.
	// Had the stale typed figures of the entries leaf been read, it would be 8 889 288 / 10 000 999.
	b := bangTheoDot()
	_, ty := ChiSoDatDuToan(b)
	if !ty.Co || ty.Gia != 10_000 {
		t.Fatalf("Chi đạt dự toán = %+v, muốn 10000 phần vạn", ty)
	}
	g, co, err := b.SoTong(VaiTroChiNganSach)
	if err != nil || !co || g != 1_000 {
		t.Fatalf("số tổng Chi ngân sách = %d,%v,%v — muốn 1000", g, co, err)
	}
}

func TestGiaTri_KhoanMucCoConThiCongConDuCachTinhLuu(t *testing.T) {
	// A line whose stored mode says `entries` but which HAS children sums its children: the tree wins.
	// The write path makes this state unreachable; the read path must not trust it anyway.
	b := bangTheoDot()
	b.KhoanMuc[0].CachTinh = TinhTheoDot
	b.GiaDot["tong"] = map[string]Dong{"c-chi": 5}
	if g, _ := giaVaCo(t, b, "tong", "c-chi"); g != 1_000 {
		t.Fatalf("cha mang nhãn `entries` = %d — muốn tổng con 1000", g)
	}
}

// --- figures that cannot be computed: a reason, never a wrapped or clamped number ---------------------

func TestTranGiaTriLaSoNguyenAnToanCuaTrinhDuyet(t *testing.T) {
	// Pinned as a literal: 2^53 − 1 is what JSON.parse reads exactly. A value past it arrives in the
	// browser rounded, so the ceiling moving up again would re-open a one-đồng-off display.
	if GiaTriToiDa != 9_007_199_254_740_991 || int64(GiaTriToiDa) != 1<<53-1 {
		t.Fatalf("GiaTriToiDa = %d, muốn 2^53-1", int64(GiaTriToiDa))
	}
	if err := KiemTraGiaTri(GiaTriToiDa); err != nil {
		t.Fatalf("đúng trần bị từ chối: %v", err)
	}
	if err := KiemTraGiaTri(-GiaTriToiDa); err != nil {
		t.Fatalf("đúng trần âm bị từ chối: %v", err)
	}
	// The old ceiling (10^17) is now a refusal on write.
	if err := KiemTraGiaTri(100_000_000_000_000_000); !errors.Is(err, ErrGiaTriQuaLon) {
		t.Fatalf("10^17 = %v, muốn ErrGiaTriQuaLon", err)
	}
}

func TestGiaTri_ChaVuotTranThiKhongTinhDuocChuKhongRaSoAm(t *testing.T) {
	// Two children each AT the ceiling: each is a legal figure, their sum is not one the browser can
	// read. The old `tong += g` returned a plausible figure here (and wrapped to a NEGATIVE one once
	// the operands were large enough); now the cell carries a reason.
	b := bangTheoDot()
	b.Gia["tay"]["c-chi"] = GiaTriToiDa
	b.GiaDot["dot"]["c-chi"] = GiaTriToiDa

	s := b.GiaTri("tong", "c-chi")
	if s.Co || s.Gia != 0 {
		t.Fatalf("cha vượt trần = %+v, muốn KHÔNG có số", s)
	}
	if !strings.Contains(s.LyDo, ErrTongVuotMuc.Error()) || !strings.Contains(s.LyDo, "Tổng số") {
		t.Fatalf("lý do = %q, muốn câu ErrTongVuotMuc kèm tên dòng", s.LyDo)
	}
	// ISOLATED: the children themselves and the other column still read.
	if g, co := giaVaCo(t, b, "tay", "c-chi"); !co || g != GiaTriToiDa {
		t.Fatalf("lá tay = %d,%v", g, co)
	}
	if g, co := giaVaCo(t, b, "tong", "c-dt"); !co || g != 1_000 {
		t.Fatalf("cột khác của cha = %d,%v, muốn 1000", g, co)
	}
	// Everything built on the unavailable figure is unavailable with the same sentence — never a
	// ratio of a wrong number.
	if _, _, err := b.SoTong(VaiTroChiNganSach); !errors.Is(err, ErrTongVuotMuc) {
		t.Fatalf("SoTong = %v, muốn ErrTongVuotMuc", err)
	}
	if _, ty := ChiSoDatDuToan(b); ty.Co || !strings.Contains(ty.LyDo, ErrTongVuotMuc.Error()) {
		t.Fatalf("chỉ số = %+v, muốn không tính được", ty)
	}
}

func TestGiaTri_TongAmDuongQuaTranGiuaChungVanDung(t *testing.T) {
	// A large positive and a large negative child: the partial sum passes the ceiling, the total does
	// not. Checking per step would refuse a correct figure.
	b := bangTheoDot()
	b.Gia["tay"]["c-chi"] = GiaTriToiDa
	b.GiaDot["dot"]["c-chi"] = -GiaTriToiDa + 5
	b.KhoanMuc = append(b.KhoanMuc, KhoanMucNganSach{ID: "them", ChaID: "tong", Ten: "Chi khác", ThuTu: 4, Cap: 1})
	b.Gia["them"] = map[string]Dong{"c-chi": 10}
	if g, co := giaVaCo(t, b, "tong", "c-chi"); !co || g != 15 {
		t.Fatalf("cha = %d,%v, muốn 15", g, co)
	}
}

func TestCongKiemTraBaoTranInt64(t *testing.T) {
	const max = Dong(1<<63 - 1)
	if _, tran := congKiemTra(max, 1); !tran {
		t.Fatal("max+1 không báo tràn")
	}
	if _, tran := congKiemTra(-max-1, -1); !tran {
		t.Fatal("min-1 không báo tràn")
	}
	if s, tran := congKiemTra(max, -max); tran || s != 0 {
		t.Fatalf("max-max = %d,%v", s, tran)
	}
}

func TestGiaTri_TongDotKhongVuaInt64ChiKhoaDongDo(t *testing.T) {
	// The store could not fit the SUM into int64 and flagged it. That line and its ancestors in that
	// column are unavailable with the batch sentence; the other lines and columns are untouched.
	b := bangTheoDot()
	delete(b.GiaDot["dot"], "c-chi")
	b.GiaDotVuotMuc = map[string]map[string]bool{"dot": {"c-chi": true}}

	for _, id := range []string{"dot", "tong"} {
		s := b.GiaTri(id, "c-chi")
		if s.Co || !strings.Contains(s.LyDo, ErrTongDotVuotMuc.Error()) || !strings.Contains(s.LyDo, "Chi sự nghiệp") {
			t.Fatalf("%s = %+v, muốn lý do tổng đợt kèm tên dòng lá", id, s)
		}
	}
	if g, co := giaVaCo(t, b, "tay", "c-chi"); !co || g != 400 {
		t.Fatalf("dòng nhập tay bên cạnh = %d,%v, muốn 400", g, co)
	}
	// A sum that fits int64 but is past the ceiling is the same sentence.
	b.GiaDotVuotMuc = nil
	b.GiaDot["dot"]["c-chi"] = GiaTriToiDa + 1
	if s := b.GiaTri("dot", "c-chi"); s.Co || !strings.Contains(s.LyDo, ErrTongDotVuotMuc.Error()) {
		t.Fatalf("tổng đợt vượt trần = %+v", s)
	}
}

func TestGiaTri_SoDaLuuVuotTranMoiVanDocDuocVaBaoLyDo(t *testing.T) {
	// A cell stored under the old 10^17 ceiling. The sheet READS; that cell is flagged.
	b := bangTheoDot()
	b.Gia["tay"]["c-chi"] = 50_000_000_000_000_000
	s := b.GiaTri("tay", "c-chi")
	if s.Co || !strings.Contains(s.LyDo, ErrGiaTriDaLuuVuotMuc.Error()) {
		t.Fatalf("ô cũ vượt trần = %+v, muốn lý do ErrGiaTriDaLuuVuotMuc", s)
	}
	if err := KiemTraGiaTriDaLuu(-50_000_000_000_000_000); !errors.Is(err, ErrGiaTriDaLuuVuotMuc) {
		t.Fatalf("KiemTraGiaTriDaLuu âm = %v", err)
	}
	if err := KiemTraGiaTriDaLuu(GiaTriToiDa); err != nil {
		t.Fatalf("KiemTraGiaTriDaLuu đúng trần = %v", err)
	}
}

func TestCanDoiVuotTranThiKhongTinhDuoc(t *testing.T) {
	// Each side within the ceiling, the difference up to twice it — not a figure the browser reads.
	cot := func(id string, v VaiTroCot) CotNganSach { return CotNganSach{ID: id, Ten: id, Kieu: CotSo, VaiTro: v} }
	mot := func(loai LoaiBang, c CotNganSach, g Dong) BangDayDu {
		return BangDayDu{
			Bang:     BangNganSach{Loai: loai},
			Cot:      []CotNganSach{c},
			KhoanMuc: []KhoanMucNganSach{{ID: "t", Ten: "Tổng", LaDongTong: true}},
			Gia:      map[string]map[string]Dong{"t": {c.ID: g}},
		}
	}
	thu := mot(BangThu, cot("xh", VaiTroThuXaHuong), GiaTriToiDa)
	chi := mot(BangChi, cot("cn", VaiTroChiNganSach), -GiaTriToiDa)
	if s := CanDoiThuChi(thu, chi); s.Co || !strings.Contains(s.LyDo, ErrTongVuotMuc.Error()) {
		t.Fatalf("cân đối vượt trần = %+v", s)
	}
	chi = mot(BangChi, cot("cn", VaiTroChiNganSach), 1)
	if s := CanDoiThuChi(thu, chi); !s.Co || s.Gia != GiaTriToiDa-1 {
		t.Fatalf("cân đối bình thường = %+v", s)
	}
}

func TestKiemTraCachTinhChon(t *testing.T) {
	for _, c := range []CachTinh{TinhTay, TinhTheoDot} {
		if err := KiemTraCachTinhChon(c); err != nil {
			t.Errorf("%q bị từ chối: %v", c, err)
		}
	}
	for _, c := range []CachTinh{TinhTheoCon, "", "MANUAL", "entry"} {
		if err := KiemTraCachTinhChon(c); !errors.Is(err, ErrCachTinhDoTuClient) {
			t.Errorf("%q = %v, muốn ErrCachTinhDoTuClient", c, err)
		}
	}
}

func TestTranKyTuDotDemTheoRune(t *testing.T) {
	// `ệ` is 3 bytes: a byte count would refuse text a Vietnamese commune types every day.
	if _, err := ChuanHoaNoiDungDot(strings.Repeat("ệ", NoiDungChungTuToiDa)); err != nil {
		t.Fatalf("đúng trần theo ký tự bị từ chối: %v", err)
	}
	if _, err := ChuanHoaNoiDungDot(strings.Repeat("ệ", NoiDungChungTuToiDa+1)); !errors.Is(err, ErrNoiDungDotQuaDai) {
		t.Fatalf("vượt trần = %v", err)
	}
	if s, err := ChuanHoaDonViCaNhan("   "); err != nil || s != "" {
		t.Fatalf("đơn vị trống = %q,%v — muốn \"\" (lưu NULL)", s, err)
	}
	if _, err := ChuanHoaDonViCaNhan(strings.Repeat("ễ", DoiTacToiDa+1)); !errors.Is(err, ErrDonViCaNhanQuaDai) {
		t.Fatalf("đơn vị vượt trần = %v", err)
	}
	if _, err := ChuanHoaSoChungTuDot(strings.Repeat("ố", SoChungTuToiDa+1)); !errors.Is(err, ErrSoChungTuDotQuaDai) {
		t.Fatalf("số chứng từ vượt trần = %v", err)
	}
}

func TestNgayDotTheoKhoangCua0008(t *testing.T) {
	for _, tc := range []struct {
		ngay time.Time
		muon error
	}{
		{time.Time{}, ErrThieuNgayDot},
		{time.Date(1999, 12, 31, 0, 0, 0, 0, time.UTC), ErrNgayDotNgoaiLich},
		{time.Date(2101, 1, 1, 0, 0, 0, 0, time.UTC), ErrNgayDotNgoaiLich},
		{time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), nil},
		{time.Date(2100, 12, 31, 0, 0, 0, 0, time.UTC), nil},
	} {
		if err := KiemTraNgayDot(tc.ngay); !errors.Is(err, tc.muon) {
			t.Errorf("%v = %v, muốn %v", tc.ngay, err, tc.muon)
		}
	}
}
