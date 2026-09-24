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

func TestGiaTri_LaTheoDotDocTongDotKhongDocOGoTay(t *testing.T) {
	b := bangTheoDot()
	if g, co := b.GiaTri("dot", "c-chi"); !co || g != 600 {
		t.Fatalf("lá `entries` = %d,%v — muốn tổng đợt 600", g, co)
	}
	// EMPTY, NOT 0 AND NOT THE STALE 9 999 999 (§9 rule 4, §9.1).
	if g, co := b.GiaTri("dot", "c-dt"); co {
		t.Fatalf("lá `entries` không có đợt nào ở c-dt mà vẫn ra %d", g)
	}
}

func TestGiaTri_LaNhapTayBoQuaTongDot(t *testing.T) {
	b := bangTheoDot()
	if g, co := b.GiaTri("tay", "c-chi"); !co || g != 400 {
		t.Fatalf("lá `manual` = %d,%v — muốn số gõ tay 400, không phải tổng đợt", g, co)
	}
}

func TestGiaTri_ChaCongCaLaTheoDot(t *testing.T) {
	b := bangTheoDot()
	if g, co := b.GiaTri("tong", "c-chi"); !co || g != 1_000 {
		t.Fatalf("cha = %d,%v — muốn 400 (tay) + 600 (tổng đợt) = 1000", g, co)
	}
	// c-dt: the manual leaf's 1000 + the entries leaf's EMPTY = 1000. Not 1000 + 9 999 999.
	if g, co := b.GiaTri("tong", "c-dt"); !co || g != 1_000 {
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
	if g, _ := b.GiaTri("tong", "c-chi"); g != 1_000 {
		t.Fatalf("cha mang nhãn `entries` = %d — muốn tổng con 1000", g)
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
