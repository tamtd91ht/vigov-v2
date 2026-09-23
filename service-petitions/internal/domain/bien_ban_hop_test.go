package domain

import (
	"testing"
	"time"
)

// The two counting rules of the Biên bản card, and the one distinction the screen makes that a
// single number cannot carry.
//
//	PROVED HERE   the header badge sums over CONCLUSIONS (§7.5) · a meeting with no conclusions is
//	              (0,0) and not an error · "chưa tách" is a different state from "0 of n done" ·
//	              "everything done" is not derivable from `x == y` alone.
//
//	NOT PROVED    which rows reach these structs. That is the store's job — the counts arrive
//	              already aggregated, over LIVE tasks only, and internal/store asserts the predicate.

var mocTaoBB = time.Date(2026, 8, 5, 3, 30, 0, 0, time.UTC)

// bienBanBaKetLuan is §8's sample meeting, with §2's exact figures: three conclusions, one of them
// finished. THE NUMBERS ARE THE SPECIFICATION'S OWN — `0/1`, `0/1`, `1/1`, badge `1/3` — so a
// change to the aggregation rule shows up as a disagreement with the screen rather than with a
// number this test invented.
func bienBanBaKetLuan() BienBanHop {
	return BienBanHop{
		ID:         "bb-001",
		TenCuocHop: "Giao ban Uỷ ban nhân dân xã tháng 8 năm 2026",
		NgayHop:    time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC),
		SoHieu:     "31/BB-UBND",
		DiaDiem:    "Phòng họp UBND xã",
		ChuTriMa:   "CB-00007",
		NguoiTaoMa: "CB-00123",
		TaoLuc:     mocTaoBB,
		KetLuan: []KetLuanHop{
			{ID: "kl-1", BienBanID: "bb-001", ThuTu: 1, NoiDung: "Rà soát tiến độ tuyến đường.",
				SoNhiemVu: 1, SoNhiemVuXong: 0, TaoLuc: mocTaoBB},
			{ID: "kl-2", BienBanID: "bb-001", ThuTu: 2, NoiDung: "Hoàn tất hồ sơ hỗ trợ sinh kế đợt 3.",
				SoNhiemVu: 1, SoNhiemVuXong: 0, TaoLuc: mocTaoBB},
			{ID: "kl-3", BienBanID: "bb-001", ThuTu: 3, NoiDung: "Đối chiếu số liệu giải ngân sáu tháng.",
				SoNhiemVu: 1, SoNhiemVuXong: 1, TaoLuc: mocTaoBB},
		},
	}
}

func TestSoKetLuanDemDungSoDongTrenThe(t *testing.T) {
	if got := bienBanBaKetLuan().SoKetLuan(); got != 3 {
		t.Errorf("SoKetLuan() = %d, muốn 3 — badge của §2 ghi `3 kết luận`", got)
	}
}

// TestTienDoNhiemVuCongDonTrenMoiKetLuan is §7.5 stated as an assertion: the header's `x/y` is the
// SUM over every conclusion, not a figure about the meeting itself.
//
// THE THREE CONCLUSIONS CARRY DIFFERENT PAIRS ON PURPOSE. With `1/1` three times, a function that
// returned the FIRST conclusion's pair, or the LAST one's, would pass — and both are plausible
// mistakes for somebody "simplifying" the loop.
func TestTienDoNhiemVuCongDonTrenMoiKetLuan(t *testing.T) {
	xong, tong := bienBanBaKetLuan().TienDoNhiemVu()
	if xong != 1 || tong != 3 {
		t.Errorf("TienDoNhiemVu() = %d/%d, muốn 1/3 — đúng con số §2 vẽ trên badge", xong, tong)
	}
}

// TestTienDoNhiemVuNhieuNhiemVuMotKetLuan is §3's sentence enforced: "Một kết luận có thể tách
// thành NHIỀU nhiệm vụ (đếm `y` trong `x/y`)".
//
// §2's per-conclusion line reads `0/1` and looks like a one-to-one link; §3 says it is not. Nothing
// in the schema or in this file caps the number, and this case is what stops somebody "tidying" the
// sum into a count of conclusions.
func TestTienDoNhiemVuNhieuNhiemVuMotKetLuan(t *testing.T) {
	b := BienBanHop{KetLuan: []KetLuanHop{
		{ID: "kl-1", ThuTu: 1, SoNhiemVu: 4, SoNhiemVuXong: 3},
		{ID: "kl-2", ThuTu: 2, SoNhiemVu: 2, SoNhiemVuXong: 0},
	}}
	if xong, tong := b.TienDoNhiemVu(); xong != 3 || tong != 6 {
		t.Errorf("TienDoNhiemVu() = %d/%d, muốn 3/6", xong, tong)
	}
}

// TestBienBanKhongCoKetLuanVanDemDuoc is §7.3: minutes with no conclusions are still saved, so
// every figure about them must have an answer. (0,0) is that answer; a panic or a divide is not.
func TestBienBanKhongCoKetLuanVanDemDuoc(t *testing.T) {
	var b BienBanHop
	if b.SoKetLuan() != 0 {
		t.Errorf("SoKetLuan() = %d, muốn 0", b.SoKetLuan())
	}
	if xong, tong := b.TienDoNhiemVu(); xong != 0 || tong != 0 {
		t.Errorf("TienDoNhiemVu() = %d/%d, muốn 0/0", xong, tong)
	}
}

// TestChuaTachKhacVoiChuaXong is the distinction §2 draws on the screen and the reason the response
// carries two numbers instead of a percentage.
//
//	`Chưa tách thành nhiệm vụ nào`  the conclusion is still only a sentence in the minutes
//	`0/3 nhiệm vụ đã hoàn thành`    three people were told to do something and none has finished
//
// Collapsing them hides exactly the conclusions somebody still has to act on.
func TestChuaTachKhacVoiChuaXong(t *testing.T) {
	chuaTach := KetLuanHop{ThuTu: 1, SoNhiemVu: 0, SoNhiemVuXong: 0}
	chuaXong := KetLuanHop{ThuTu: 2, SoNhiemVu: 3, SoNhiemVuXong: 0}

	if !chuaTach.ChuaTachNhiemVu() {
		t.Error("kết luận chưa có nhiệm vụ nào lại không báo `chưa tách`")
	}
	if chuaXong.ChuaTachNhiemVu() {
		t.Error("kết luận đã tách 3 nhiệm vụ lại báo `chưa tách` — dòng phụ trên màn hình sẽ sai")
	}
}

// TestDaXongToanBoKhongGopChuaTachVaoDaXong is the trap `0 == 0` sets in Go and not in a commune.
//
// A conclusion nobody has split is NOT "everything asked for was completed". The second return
// value is what makes the two impossible to confuse, and this case is why it exists.
func TestDaXongToanBoKhongGopChuaTachVaoDaXong(t *testing.T) {
	for _, tr := range []struct {
		ten          string
		k            KetLuanHop
		xong, daTach bool
	}{
		{"chưa tách nhiệm vụ nào", KetLuanHop{SoNhiemVu: 0, SoNhiemVuXong: 0}, false, false},
		{"tách rồi, chưa xong", KetLuanHop{SoNhiemVu: 3, SoNhiemVuXong: 1}, false, true},
		{"tách rồi, xong hết", KetLuanHop{SoNhiemVu: 3, SoNhiemVuXong: 3}, true, true},
		{"một nhiệm vụ, xong", KetLuanHop{SoNhiemVu: 1, SoNhiemVuXong: 1}, true, true},
	} {
		t.Run(tr.ten, func(t *testing.T) {
			xong, daTach := tr.k.DaXongToanBo()
			if xong != tr.xong || daTach != tr.daTach {
				t.Errorf("DaXongToanBo() = (%v, %v), muốn (%v, %v)", xong, daTach, tr.xong, tr.daTach)
			}
		})
	}
}
