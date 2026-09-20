package domain

import (
	"testing"
	"time"
)

// WHAT THIS FILE PROVES: the arithmetic of §3 and §13, against the specification's OWN worked
// figures. Where a case uses a number from docs/ui-ux/06-giai-ngan.md it says so, because a test
// that invents its own expected value only proves the code agrees with itself.
//
// WHAT IT DOES NOT PROVE: nothing here touches a database, a route or a permission. The commune
// does not appear at all — this package imports the standard library and nothing else (rule 4 for
// services), so isolation is not a property it can hold or break.

// vnTime is the commune's clock. Every date in this file is built in it, because
// PhanTramThoiGianDaQua compares against year boundaries in the location of the time it is given
// and a UTC clock would move those boundaries by seven hours.
var vnTime = time.FixedZone("ICT", 7*3600)

func ngay(nam int, thang time.Month, ngay int) time.Time {
	return time.Date(nam, thang, ngay, 12, 0, 0, 0, vnTime)
}

// lucDaQua7096 is the instant at which the 2026 budget year is 70,96% elapsed — the figure §3
// uses in its worked examples ("thời gian đã qua 70,96%", "chậm 70,96 điểm"). It is a datetime and
// not a date because 70,96% of a 365-day year falls partway through 17/09; a date alone cannot
// reproduce the specification's own number, and rounding one to make it fit would be testing the
// test rather than the code.
var lucDaQua7096 = time.Date(2026, time.September, 17, 0, 6, 0, 0, vnTime)

func TestTyLeGiaiNganTheoViDuCuaDacTa(t *testing.T) {
	// §3, the KPI card: 3.433.990.000 out of 33.230.000.000 is reported as 10,33%.
	tien := TienDoDuAn{
		DuAn:       DuAn{Nam: 2026, KeHoachVonNam: 33_230_000_000},
		DaGiaiNgan: 3_433_990_000,
	}
	ty, ok := tien.TyLeGiaiNgan()
	if !ok {
		t.Fatal("tỷ lệ phải tính được khi có kế hoạch vốn")
	}
	if ty != 1033 {
		t.Fatalf("tỷ lệ = %d phần vạn, muốn 1033 (10,33%% — §3)", ty)
	}
}

func TestTyLeGiaiNganVuotMotTramKhongBiChan(t *testing.T) {
	// §13 rule 2: the ratio may exceed 100% and must be shown, not blocked.
	tien := TienDoDuAn{
		DuAn:       DuAn{Nam: 2026, KeHoachVonNam: 100_000_000},
		DaGiaiNgan: 176_500_000,
	}
	ty, ok := tien.TyLeGiaiNgan()
	if !ok || ty != 17650 {
		t.Fatalf("tỷ lệ = %d phần vạn (ok=%v), muốn 17650 (176,5%% — §13 quy tắc 2)", ty, ok)
	}
	// And the remainder is negative, not clamped: an over-disbursement must be visible.
	if con := tien.ConPhaiGiaiNgan(); con != -76_500_000 {
		t.Fatalf("còn phải giải ngân = %d, muốn -76500000 — kẹp về 0 là giấu mất khoản vượt", con)
	}
}

func TestKhongCoKeHoachThiKhongCoTyLe(t *testing.T) {
	// 0% and "no ratio" are different statements. A project nobody has allocated money to must not
	// be reported as the worst performer in the commune.
	tien := TienDoDuAn{DuAn: DuAn{Nam: 2026, KeHoachVonNam: 0}, DaGiaiNgan: 0}
	if _, ok := tien.TyLeGiaiNgan(); ok {
		t.Fatal("kế hoạch vốn 0 phải trả ok=false, không phải 0%")
	}
	if _, ok := DiemCham(tien, ngay(2026, time.September, 20)); ok {
		t.Fatal("không có mẫu số thì không có điểm chậm")
	}
	if LaCham(tien, ngay(2026, time.September, 20), NguongCanhBaoChamMacDinh) {
		t.Fatal("dự án chưa bố trí vốn KHÔNG bị đếm là chậm — xem chú thích trên LaCham")
	}
}

func TestPhanTramThoiGianDaQuaOHaiDauNam(t *testing.T) {
	cases := []struct {
		ten  string
		luc  time.Time
		muon PhanVan
	}{
		{"đầu ngày 01/01", time.Date(2026, time.January, 1, 0, 0, 0, 0, vnTime), 0},
		{"trước năm", ngay(2025, time.December, 31), 0},
		// 2026 is not a leap year: 01/07 00:00 is day 181 of 365.
		{"giữa năm", time.Date(2026, time.July, 1, 0, 0, 0, 0, vnTime), 4958},
		{"đúng 01/01 năm sau", time.Date(2027, time.January, 1, 0, 0, 0, 0, vnTime), 10000},
		// The last day of the year must NOT read as more than 100% elapsed — that is what the
		// literal "31/12 − 01/01" denominator of §3 would produce.
		{"ngày cuối năm", time.Date(2026, time.December, 31, 23, 0, 0, 0, vnTime), 9998},
	}
	for _, c := range cases {
		t.Run(c.ten, func(t *testing.T) {
			if got := PhanTramThoiGianDaQua(c.luc, 2026); got != c.muon {
				t.Fatalf("= %d phần vạn, muốn %d", got, c.muon)
			}
		})
	}
}

func TestDiemChamBangDungPhanTramThoiGianKhiGiaiNganKhong(t *testing.T) {
	// §3, stated explicitly: "Dự án giải ngân 0% giữa năm ⇒ chậm 70,96 điểm (bằng đúng % thời gian
	// đã qua)". See lucDaQua7096 for why that share of 2026 falls partway through 17/09.
	luc := lucDaQua7096
	if daQua := PhanTramThoiGianDaQua(luc, 2026); daQua != 7096 {
		t.Fatalf("thời gian đã qua = %d phần vạn, muốn 7096 — mốc lấy từ §3", daQua)
	}

	tien := TienDoDuAn{DuAn: DuAn{Nam: 2026, KeHoachVonNam: 2_130_000_000}, DaGiaiNgan: 0}
	diem, ok := DiemCham(tien, luc)
	if !ok || diem != 7096 {
		t.Fatalf("điểm chậm = %d (ok=%v), muốn 7096", diem, ok)
	}
	if !LaCham(tien, luc, NguongCanhBaoChamMacDinh) {
		t.Fatal("chậm 70,96 điểm vượt ngưỡng 10 điểm — phải bị đánh dấu chậm")
	}
}

func TestDuAnDiTruocLichThiDiemChamAm(t *testing.T) {
	luc := lucDaQua7096 // 70,96% đã qua
	tien := TienDoDuAn{
		DuAn:       DuAn{Nam: 2026, KeHoachVonNam: 100_000_000},
		DaGiaiNgan: 90_000_000, // §8, dự án mẫu: 90%
	}
	diem, ok := DiemCham(tien, luc)
	if !ok || diem != 7096-9000 {
		t.Fatalf("điểm chậm = %d (ok=%v), muốn %d", diem, ok, 7096-9000)
	}
	if LaCham(tien, luc, NguongCanhBaoChamMacDinh) {
		t.Fatal("dự án đi trước lịch không phải dự án chậm")
	}
}

func TestNguongSoSanhLaLonHonHAN(t *testing.T) {
	// §3: "dự án CHẬM khi diem_cham > nguong". Exactly on the threshold is NOT behind — the
	// difference is invisible until a commune sets the threshold to 0.
	luc := lucDaQua7096 // 7096
	tien := TienDoDuAn{
		DuAn:       DuAn{Nam: 2026, KeHoachVonNam: 10_000_000},
		DaGiaiNgan: 6_096_000, // 60,96% -> điểm chậm đúng 1000 = 10 điểm
	}
	diem, _ := DiemCham(tien, luc)
	if diem != NguongCanhBaoChamMacDinh {
		t.Fatalf("dựng sai ca: điểm chậm = %d, cần đúng bằng ngưỡng %d", diem, NguongCanhBaoChamMacDinh)
	}
	if LaCham(tien, luc, NguongCanhBaoChamMacDinh) {
		t.Fatal("đúng BẰNG ngưỡng thì chưa chậm — so sánh phải là > chứ không phải >=")
	}
}

func TestTongMucDuocDuyetBoTrongThiLayKeHoachNam(t *testing.T) {
	// §9: "Để trống thì lấy bằng số tiền bố trí năm nay."
	if got := (DuAn{KeHoachVonNam: 7_500_000_000}).TongMucHieuLuc(); got != 7_500_000_000 {
		t.Fatalf("= %d, muốn 7500000000", got)
	}
	if got := (DuAn{KeHoachVonNam: 100, TongMucDuocDuyet: 250}).TongMucHieuLuc(); got != 250 {
		t.Fatalf("= %d, muốn 250 — số đã khai phải thắng mặc định", got)
	}
}

func TestLuyKeCongDon(t *testing.T) {
	got := LuyKe([]Dong{100, 0, 250, 50})
	muon := []Dong{100, 100, 350, 400}
	for i := range muon {
		if got[i] != muon[i] {
			t.Fatalf("luỹ kế[%d] = %d, muốn %d (cả dãy: %v)", i, got[i], muon[i], got)
		}
	}
	if len(LuyKe(nil)) != 0 {
		t.Fatal("luỹ kế của dãy rỗng phải rỗng")
	}
}

func TestKeHoachTuyenTinhKetThucDUNGBangKeHoach(t *testing.T) {
	// 33.230.000.000 across 12 months does not divide evenly. The last point must still be the
	// plan exactly: a planning line stopping a few đồng short raises the question of which figure
	// is wrong, every time somebody looks at the chart.
	duong := KeHoachTuyenTinh(33_230_000_000, 12)
	if len(duong) != 12 {
		t.Fatalf("số mốc = %d, muốn 12", len(duong))
	}
	if duong[11] != 33_230_000_000 {
		t.Fatalf("mốc cuối = %d, muốn 33230000000 ĐÚNG BẰNG kế hoạch", duong[11])
	}
	for i := 1; i < len(duong); i++ {
		if duong[i] < duong[i-1] {
			t.Fatalf("đường kế hoạch giảm tại mốc %d: %v", i, duong)
		}
	}
	if KeHoachTuyenTinh(100, 0) != nil {
		t.Fatal("0 mốc phải trả nil, không phải một dãy rỗng giả vờ là biểu đồ")
	}
}
