package domain

import (
	"errors"
	"strings"
	"testing"
)

// The shape rules of a staff record. Cheap cases, and each one is a defect that reaches a
// government directory and stays there: rule 7 forbids hard delete, so a row typed wrongly is a
// row that is corrected by an edit somebody has to notice first.

func TestChuanHoaHoTenGomKhoangTrangVaBatBuoc(t *testing.T) {
	// COLLAPSING IS NOT COSMETIC. The commune's directory arrives by Excel import
	// (12-danh-ba-can-bo.md §6), which is exactly where doubled spaces come from, and `ho_ten` is
	// what the screen searches and sorts on: "Nguyễn  Văn A" sorts and matches as a different
	// person from "Nguyễn Văn A".
	got, err := ChuanHoaHoTen("  Nguyễn\tVăn   A  ")
	if err != nil {
		t.Fatalf("lỗi: %v", err)
	}
	if got != "Nguyễn Văn A" {
		t.Errorf("= %q, muốn %q", got, "Nguyễn Văn A")
	}

	for _, tho := range []string{"", "   ", "\t\n"} {
		if _, err := ChuanHoaHoTen(tho); !errors.Is(err, ErrThieuHoTen) {
			t.Errorf("%q: lỗi = %v, muốn ErrThieuHoTen", tho, err)
		}
	}
	if _, err := ChuanHoaHoTen(strings.Repeat("A", tranHoTen+1)); !errors.Is(err, ErrHoTenQuaDai) {
		t.Errorf("tên quá dài không bị từ chối: %v", err)
	}
}

// LOWER-CASING IS LOAD-BEARING: `UNIQUE (tenant_id, email)` is case SENSITIVE, so two spellings of
// one mailbox are two rows to PostgreSQL and one person to everybody else — and the sign-in path
// looks the address up with an exact match, so which of the two can sign in would depend on how
// somebody typed it.
func TestChuanHoaEmailHaThapVaKiemHinhDang(t *testing.T) {
	got, err := ChuanHoaEmail("  Can.Bo.A@Xa.DaNang.GOV.VN ")
	if err != nil {
		t.Fatalf("lỗi: %v", err)
	}
	if got != "can.bo.a@xa.danang.gov.vn" {
		t.Errorf("= %q, muốn chữ thường đã cắt khoảng trắng", got)
	}

	for _, xau := range []string{"khong-co-a-cong", "a@b", "@xa.gov.vn", "a@", "a b@xa.gov.vn", "a@@xa.gov.vn"} {
		if _, err := ChuanHoaEmail(xau); !errors.Is(err, ErrEmailSaiDinhDang) {
			t.Errorf("%q: lỗi = %v, muốn ErrEmailSaiDinhDang", xau, err)
		}
	}
	if _, err := ChuanHoaEmail("   "); !errors.Is(err, ErrThieuEmail) {
		t.Errorf("thư điện tử rỗng: lỗi = %v, muốn ErrThieuEmail", err)
	}
}

// NO FORMAT IS IMPOSED — migration 0009 §2 says why: a real commune directory holds area codes in
// brackets, extensions and international prefixes, and a CHECK on shape can only refuse them.
// What IS refused is a character no telephone number can contain.
func TestChuanHoaSoDienThoaiNhanDangThatVaTuChoiKyTuLa(t *testing.T) {
	for _, so := range []string{"0900000000", "(0236) 3 123 456", "+84-900-000-000", "0900000000, 101"} {
		got, err := ChuanHoaSoDienThoai(" " + so + " ")
		if err != nil {
			t.Errorf("%q bị từ chối: %v", so, err)
		}
		if got != so {
			t.Errorf("%q bị sửa thành %q — chuẩn hoá chỉ được cắt khoảng trắng hai đầu", so, got)
		}
	}
	// "" is the one spelling of "no number" (migration 0009 §2), and it is legitimate on both
	// columns.
	if got, err := ChuanHoaSoDienThoai("   "); err != nil || got != "" {
		t.Errorf("số rỗng = %q, %v — muốn chuỗi rỗng không lỗi", got, err)
	}
	// The newline is EMBEDDED, not trailing: a trailing one is trimmed and the value is fine. An
	// embedded one would put a line break into a `tel:` link and into every log line that carries
	// the value.
	for _, xau := range []string{"0900000000<script>", "gọi đi", "0900\n000000"} {
		if _, err := ChuanHoaSoDienThoai(xau); !errors.Is(err, ErrSoDienThoaiSai) {
			t.Errorf("%q: lỗi = %v, muốn ErrSoDienThoaiSai", xau, err)
		}
	}
}

// NO REFUSAL EVER QUOTES WHAT IT REFUSED (rule 3, forbidden #3). Every string this file inspects is
// personal data or close to it, and an error message is the one string that reaches a log
// aggregator, a browser console and a screenshot in a support ticket.
func TestLoiKhongBaoGioNhacLaiGiaTriBiTuChoi(t *testing.T) {
	const so = "0900000000x"
	_, err := ChuanHoaSoDienThoai(so)
	if err == nil {
		t.Fatal("ký tự lạ không bị từ chối")
	}
	if strings.Contains(err.Error(), "0900000000") {
		t.Errorf("thông báo lỗi nhắc lại số điện thoại: %v", err)
	}

	_, err = ChuanHoaEmail("Nguyen Van A <nva@xa.gov.vn>")
	if err == nil {
		t.Fatal("thư điện tử sai định dạng không bị từ chối")
	}
	if strings.Contains(err.Error(), "@") || strings.Contains(err.Error(), "Nguyen") {
		t.Errorf("thông báo lỗi nhắc lại địa chỉ: %v", err)
	}
}

// An id the client supplies is BOUNDED but NOT verified here: whether the row exists is the foreign
// key's job, inside the transaction, where the answer cannot go stale between the check and the
// write. The empty string is legitimate and means "none" — both columns are nullable.
func TestKiemTraIDThamChieuChiChanDoDaiChuKhongKiemTonTai(t *testing.T) {
	for _, id := range []string{"", "bp-001", strings.Repeat("a", tranIDThamChieu)} {
		if err := KiemTraIDThamChieu(id); err != nil {
			t.Errorf("%q bị từ chối: %v", id, err)
		}
	}
	if err := KiemTraIDThamChieu(strings.Repeat("a", tranIDThamChieu+1)); !errors.Is(err, ErrIDThamChieuQuaDai) {
		t.Errorf("id quá dài không bị từ chối: %v", err)
	}
}

// THU_TU_DANH_BA: nil and 0 are both legitimate and DIFFERENT (0 is a real position, nil is "no
// explicit order"); a negative value is refused, and so is a value past the INTEGER column.
func TestKiemTraThuTuDanhBa(t *testing.T) {
	so := func(v int) *int { return &v }
	for _, ok := range []*int{nil, so(0), so(7), so(1<<31 - 1)} {
		if err := KiemTraThuTuDanhBa(ok); err != nil {
			t.Errorf("giá trị hợp lệ %v bị từ chối: %v", ok, err)
		}
	}
	if err := KiemTraThuTuDanhBa(so(-1)); !errors.Is(err, ErrThuTuDanhBaAm) {
		t.Errorf("-1: lỗi = %v, muốn ErrThuTuDanhBaAm", err)
	}
	if err := KiemTraThuTuDanhBa(so(1 << 31)); !errors.Is(err, ErrThuTuDanhBaQuaLon) {
		t.Errorf("2^31: lỗi = %v, muốn ErrThuTuDanhBaQuaLon", err)
	}
}

// THE REASON OF A SOFT DELETE IS REQUIRED, TRIMMED, AND BOUNDED IN RUNES (rule 7, invariant 1).
//
// THE BOUND IS COUNTED IN RUNES: 500 accented Vietnamese characters are ~1500 bytes, and a byte
// bound would refuse an honest Vietnamese sentence at a third of the length of an English one.
func TestChuanHoaLyDoXoaBatBuocCatKhoangTrangVaDemTheoKyTu(t *testing.T) {
	got, err := ChuanHoaLyDoXoa("  Nhập trùng với CB-2026-7K3M9Q  ")
	if err != nil {
		t.Fatalf("lỗi: %v", err)
	}
	if got != "Nhập trùng với CB-2026-7K3M9Q" {
		t.Errorf("= %q, muốn bản đã cắt khoảng trắng hai đầu", got)
	}

	for _, tho := range []string{"", "   ", "\t\n"} {
		if _, err := ChuanHoaLyDoXoa(tho); !errors.Is(err, ErrThieuLyDoXoa) {
			t.Errorf("%q: lỗi = %v, muốn ErrThieuLyDoXoa", tho, err)
		}
	}

	// Exactly at the ceiling, in multi-byte runes: accepted.
	if _, err := ChuanHoaLyDoXoa(strings.Repeat("ệ", tranLyDoXoa)); err != nil {
		t.Errorf("%d ký tự có dấu bị từ chối — trần đang đếm BYTE chứ không đếm ký tự: %v", tranLyDoXoa, err)
	}
	if _, err := ChuanHoaLyDoXoa(strings.Repeat("ệ", tranLyDoXoa+1)); !errors.Is(err, ErrLyDoXoaQuaDai) {
		t.Errorf("lý do quá dài không bị từ chối: %v", err)
	}
}
