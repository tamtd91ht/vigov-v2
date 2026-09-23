package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// Tests for the WRITE rules of the meeting-minutes register.
//
//	PROVED HERE   §7.2's numbering counts from the highest number EVER issued, so a removal leaves a
//	              gap rather than a second ② · a conclusion may not be empty · the meeting day is
//	              kept as a CALENDAR DAY and a missing one is refused · every refusal of what the
//	              client sent is recognised by LaLoiDauVaoBienBan, so none of them reaches a client
//	              as a 500.

func TestThuTuKetLuanTiepTheo_NoiTiepSoLonNhatDaCap(t *testing.T) {
	for _, ca := range []struct {
		ten     string
		lonNhat int
		muon    int
	}{
		{"biên bản chưa có kết luận nào", 0, 1},
		{"nối tiếp ③", 3, 4},
		// THE CASE THE WHOLE RULE EXISTS FOR: ② was removed, so two conclusions are live and the
		// highest number issued is still 3. The next one is ④ — a gap, exactly as §7.2 asks.
		{"đã xoá mềm một kết luận ở giữa", 3, 4},
	} {
		t.Run(ca.ten, func(t *testing.T) {
			if got := ThuTuKetLuanTiepTheo(ca.lonNhat); got != ca.muon {
				t.Errorf("số thứ tự kế tiếp = %d, muốn %d", got, ca.muon)
			}
		})
	}
}

// TestThuTuKetLuanTiepTheo_KhongBaoGioTraSoKhongHoacAm — ① is the first number, and the schema's
// `ket_luan_hop_thu_tu_tu_mot` refuses anything below it. A zero here would reach the database as a
// constraint error instead of a circle nobody can read.
func TestThuTuKetLuanTiepTheo_KhongBaoGioTraSoKhongHoacAm(t *testing.T) {
	for _, lonNhat := range []int{-1, -100} {
		if got := ThuTuKetLuanTiepTheo(lonNhat); got != 1 {
			t.Errorf("với số lớn nhất %d, số kế tiếp = %d, muốn 1", lonNhat, got)
		}
	}
}

func TestKiemTenCuocHop(t *testing.T) {
	if _, err := KiemTenCuocHop("   "); !errors.Is(err, ErrThieuTenCuocHop) {
		t.Errorf("tên toàn khoảng trắng: lỗi = %v, muốn ErrThieuTenCuocHop", err)
	}
	if _, err := KiemTenCuocHop(strings.Repeat("a", TenCuocHopToiDa+1)); !errors.Is(err, ErrTenCuocHopQuaDai) {
		t.Errorf("tên quá dài: lỗi = %v", err)
	}
	// MEASURED IN RUNES: a title at the ceiling in Vietnamese must pass, or the bound cuts Vietnamese
	// at a third of the length it cuts English.
	dai := strings.Repeat("ữ", TenCuocHopToiDa)
	got, err := KiemTenCuocHop("  " + dai + "  ")
	if err != nil {
		t.Fatalf("tên tiếng Việt đúng bằng trần bị từ chối: %v", err)
	}
	if got != dai {
		t.Error("tên không được cắt khoảng trắng hai đầu")
	}
}

func TestKiemNoiDungKetLuan(t *testing.T) {
	if _, err := KiemNoiDungKetLuan(" \n "); !errors.Is(err, ErrThieuNoiDungKetLuan) {
		t.Errorf("kết luận rỗng: lỗi = %v, muốn ErrThieuNoiDungKetLuan", err)
	}
	if _, err := KiemNoiDungKetLuan(strings.Repeat("x", NoiDungKetLuanToiDa+1)); !errors.Is(err, ErrNoiDungKetLuanQuaDai) {
		t.Errorf("kết luận quá dài: lỗi = %v", err)
	}
	got, err := KiemNoiDungKetLuan("  Giao Địa chính rà soát tiến độ tuyến đường.  ")
	if err != nil || got != "Giao Địa chính rà soát tiến độ tuyến đường." {
		t.Errorf("nội dung = %q, lỗi = %v", got, err)
	}
}

// TestChuanHoaNgayHop_GiuNgayLichCatGio is the difference between this column and the task
// register's two deadlines: nothing counts from a meeting day, and a time of day nobody recorded
// would decide which DAY the card shows for a reader one time zone away.
func TestChuanHoaNgayHop_GiuNgayLichCatGio(t *testing.T) {
	if _, err := ChuanHoaNgayHop(time.Time{}); !errors.Is(err, ErrThieuNgayHop) {
		t.Errorf("ngày họp rỗng: lỗi = %v, muốn ErrThieuNgayHop", err)
	}

	// EARLY MORNING IN A ZONE AHEAD OF UTC — the case that separates "read the calendar day" from
	// "convert to UTC, then read it". `01:00 +07` on the fifth is `18:00 UTC` on the FOURTH, so a
	// conversion first would file minutes for a meeting held the day before it was.
	dongDuong := time.FixedZone("ICT", 7*3600)
	ra, err := ChuanHoaNgayHop(time.Date(2026, 8, 5, 1, 0, 0, 0, dongDuong))
	if err != nil {
		t.Fatalf("chuẩn hoá ngày họp: %v", err)
	}
	if !ra.Equal(time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("ngày họp = %v, muốn 2026-08-05 nửa đêm UTC", ra)
	}
	if ra.Hour() != 0 || ra.Minute() != 0 || ra.Second() != 0 || ra.Nanosecond() != 0 {
		t.Errorf("ngày họp còn giữ giờ: %v", ra)
	}

	// The ordinary path: the wire format already produces UTC midnight, and it must come back
	// unchanged.
	ngay, _ := time.Parse("2006-01-02", "2026-09-06")
	if ra, err := ChuanHoaNgayHop(ngay); err != nil || !ra.Equal(ngay) {
		t.Errorf("ngày lịch từ dây đổi thành %v (lỗi %v)", ra, err)
	}
}

func TestKiemThanhPhan(t *testing.T) {
	// A BLANK ROW IS DROPPED, not refused: §4's control leaves them behind on its own.
	ra, err := KiemThanhPhan([]string{" CB-00007 ", "", "   ", "Đại diện Mặt trận Tổ quốc xã"})
	if err != nil {
		t.Fatalf("thành phần hợp lệ bị từ chối: %v", err)
	}
	if len(ra) != 2 || ra[0] != "CB-00007" || ra[1] != "Đại diện Mặt trận Tổ quốc xã" {
		t.Errorf("thành phần = %#v", ra)
	}

	// NEVER nil — the column is NOT NULL DEFAULT '[]'.
	if ra := mustThanhPhan(t, nil); ra == nil {
		t.Error("danh sách rỗng trả nil — cột là NOT NULL DEFAULT '[]'")
	}

	if _, err := KiemThanhPhan(make([]string, ThanhPhanToiDa+1)); !errors.Is(err, ErrQuaNhieuThanhPhan) {
		t.Errorf("quá nhiều dòng: lỗi = %v", err)
	}
	if _, err := KiemThanhPhan([]string{strings.Repeat("x", ThanhPhanMotDongToiDa+1)}); !errors.Is(err, ErrThanhPhanQuaDai) {
		t.Errorf("một dòng quá dài: lỗi = %v", err)
	}
}

func mustThanhPhan(t *testing.T, ds []string) []string {
	t.Helper()
	ra, err := KiemThanhPhan(ds)
	if err != nil {
		t.Fatalf("thành phần: %v", err)
	}
	return ra
}

// TestLaLoiDauVaoBienBan_NhanHetMoiLoiNguoiDungGui is what keeps a refusal of the FORM from reaching
// a client as a 500 — and, the other way round, a store failure from reaching it as a 400. A
// sentinel added above and forgotten here is a 500 on a typo.
func TestLaLoiDauVaoBienBan_NhanHetMoiLoiNguoiDungGui(t *testing.T) {
	for _, err := range []error{
		ErrThieuTenCuocHop, ErrTenCuocHopQuaDai, ErrThieuNgayHop, ErrNgayHopKhongDocDuoc,
		ErrSoHieuBienBanQuaDai, ErrDiaDiemBienBanQuaDai, ErrNoiDungBienBanQuaDai, ErrChuTriQuaDai,
		ErrThieuNoiDungKetLuan, ErrNoiDungKetLuanQuaDai, ErrQuaNhieuKetLuan,
		ErrQuaNhieuThanhPhan, ErrThanhPhanQuaDai,
	} {
		if !LaLoiDauVaoBienBan(err) {
			t.Errorf("lỗi đầu vào %v không được nhận — client sẽ nhận 500 cho một lỗi gõ phím", err)
		}
	}
	if LaLoiDauVaoBienBan(errors.New("kết nối cơ sở dữ liệu hỏng")) {
		t.Error("lỗi hệ thống bị nhận là lỗi đầu vào — client sẽ thử lại mãi với dữ liệu khác")
	}
	// A refusal of another register must not be swept in either: the two lists answer different
	// questions and one shared list would blur them.
	if LaLoiDauVaoBienBan(ErrThieuTieuDeNhiemVu) {
		t.Error("lỗi của sổ nhiệm vụ bị nhận là lỗi của sổ biên bản")
	}
}
