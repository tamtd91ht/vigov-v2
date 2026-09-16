package app

import (
	"errors"
	"strings"
	"testing"
)

// THE CHANNEL THIS FILE CLOSES. One `nguoi_dung` table holds two populations (migration 0003):
// the public staff directory, and the people who actually have a sign-in account. Before the
// sign-in query filtered `co_tai_khoan`, a directory row came back from the database like any
// other and went on to password.KiemTra — with an EMPTY hash, which fails to parse and returns
// at once, while a real account spends tens of milliseconds inside argon2.
//
// That difference is not a performance detail, it is a READ of data nobody was granted: time the
// responses of an UNAUTHENTICATED endpoint against a list of names and the commune's staff
// directory comes back, split into who has an account and who does not. Nothing is logged,
// nothing fails, no rate limit is tripped by a request that answers correctly.
//
// WHAT IS ASSERTED, AND WHY NOT TIME. Measuring durations in a test is a flaky test, and a flaky
// test gets deleted. The property that actually erases the channel is stronger and exact: the
// row NEVER CROSSES INTO GO, so password.KiemTra is not reached at all — the hash is the only
// input it could take and it stays in the database. That is counted, not timed.
//
// The directory row here carries the hash of the password being submitted (bamGia). So if the
// filter were ever dropped, this would not fail on a subtle timing assertion: a person who was
// never given an account would SIGN IN, and the assertions below would report a session.

// boThoiGian strips slog's time attribute, which differs between two runs by construction. What
// is under test is that the REST of the line is identical.
func boThoiGian(ra string) string {
	var dong []string
	for _, d := range strings.Split(strings.TrimSpace(ra), "\n") {
		if i := strings.Index(d, " level="); i >= 0 {
			d = d[i+1:]
		}
		dong = append(dong, d)
	}
	return strings.Join(dong, "\n")
}

func TestNguoiChiCoTrongDanhBaKhongPhanBietDuocVoiEmailKhongTonTai(t *testing.T) {
	// Case A: the address belongs to somebody in the public staff directory who has no account.
	// NOT LOCKED — dangHoatDong stays true, as it does on every directory row — so this test can
	// only pass on the co_tai_khoan predicate.
	danhBa := dungBanThu(t)
	danhBa.ghi.coTaiKhoan = false
	if !danhBa.ghi.dangHoatDong {
		t.Fatal("dựng sai cảnh: người này phải CHƯA bị khoá, chỉ là không có tài khoản")
	}
	_, errDanhBa := danhBa.dangNhap.Chay(ctxXa(xaThu), yeuCauDung())

	// Case B: the address does not exist in this commune at all.
	khongCo := dungBanThu(t)
	khongCo.ghi.khongCoNguoiDung = true
	_, errKhongCo := khongCo.dangNhap.Chay(ctxXa(xaThu), yeuCauDung())

	// 1. The same error, and the one that says nothing about which case it was.
	if !errors.Is(errDanhBa, ErrDangNhapThatBai) {
		t.Fatalf("người chỉ có trong danh bạ: muốn ErrDangNhapThatBai, nhận %v", errDanhBa)
	}
	if !errors.Is(errKhongCo, ErrDangNhapThatBai) {
		t.Fatalf("email không tồn tại: muốn ErrDangNhapThatBai, nhận %v", errKhongCo)
	}
	if errDanhBa.Error() != errKhongCo.Error() {
		t.Errorf("hai trường hợp trả hai lỗi khác nhau:\n  danh bạ:    %q\n  không có:   %q",
			errDanhBa.Error(), errKhongCo.Error())
	}

	// 2. The same single log line. Two different sentences here would hand the distinction
	// straight back, in the log pipeline, readable across every commune at once.
	if a, b := boThoiGian(danhBa.log.String()), boThoiGian(khongCo.log.String()); a != b {
		t.Errorf("hai trường hợp ghi hai dòng log khác nhau:\n  danh bạ:  %s\n  không có: %s", a, b)
	}

	// 3. THE ASSERTION THAT ERASES THE CHANNEL: not one staff row was handed to Go, so
	// password.KiemTra never ran — it has no hash to run on. The filter is in the SQL, which is
	// the only place it removes the difference instead of hiding it.
	if n := danhBa.ghi.soHangCanBoDaTra(); n != 0 {
		t.Errorf("CSDL trả %d dòng cho người chỉ có trong danh bạ — dòng đó đi tiếp tới "+
			"password.KiemTra và thời gian đáp lộ ra ai có tài khoản, ai không", n)
	}
	if n := khongCo.ghi.soHangCanBoDaTra(); n != 0 {
		t.Errorf("email không tồn tại mà CSDL trả %d dòng", n)
	}

	// 4. And nothing was written. The submitted password MATCHES the hash on the directory row,
	// so a session here would mean the row reached KiemTra and verified — a person who was never
	// given an account, signed in.
	for ten, b := range map[string]*banThu{"danh bạ": danhBa, "email không tồn tại": khongCo} {
		if n := b.ghi.soGiaoDich(); n != 0 {
			t.Errorf("%s: mở %d giao dịch cho một lần đăng nhập thất bại", ten, n)
		}
		if b.ghi.tim("INSERT INTO phien") != nil {
			t.Errorf("%s: MỞ ĐƯỢC PHIÊN — người không có tài khoản đăng nhập vào được", ten)
		}
		if b.ghi.tim("INSERT INTO audit_log") != nil {
			t.Errorf("%s: có vết cho một lần đăng nhập không thành", ten)
		}
	}
}

func TestBanThuPhanBietDuocTaiKhoanThatVoiDongDanhBa(t *testing.T) {
	// The control. Without it the test above could pass because the harness refuses every
	// sign-in, which would prove nothing at all. Same request, same password, the one difference
	// being that this person HAS an account: the row is delivered, the session opens.
	b := dungBanThu(t)

	kq, err := b.dangNhap.Chay(ctxXa(xaThu), yeuCauDung())
	if err != nil {
		t.Fatalf("tài khoản thật mà đăng nhập hỏng: %v", err)
	}
	if kq.Sid == "" {
		t.Fatal("không có sid")
	}
	if !kq.CanBo.CoTaiKhoan || !kq.CanBo.DangHoatDong {
		t.Errorf("đọc ra (CoTaiKhoan=%v, DangHoatDong=%v), muốn cả hai true",
			kq.CanBo.CoTaiKhoan, kq.CanBo.DangHoatDong)
	}
	if n := b.ghi.soHangCanBoDaTra(); n != 1 {
		t.Fatalf("CSDL trả %d dòng cho một tài khoản thật, muốn 1 — bàn thử không phân biệt "+
			"được hai trường hợp thì test kia không chứng minh điều gì", n)
	}
}

func TestTaiKhoanBiKhoaVanBiTuChoiRieng(t *testing.T) {
	// The twin case, kept separate on purpose. `co_tai_khoan` was ADDED to the sign-in predicate,
	// not swapped in for `dang_hoat_dong`: dropping the latter would let every locked-out former
	// employee back in, and the new filter would not notice, because they do have an account.
	b := dungBanThu(t)
	b.ghi.dangHoatDong = false
	if !b.ghi.coTaiKhoan {
		t.Fatal("dựng sai cảnh: người này phải CÓ tài khoản, chỉ là đã bị khoá")
	}

	_, err := b.dangNhap.Chay(ctxXa(xaThu), yeuCauDung())
	if !errors.Is(err, ErrDangNhapThatBai) {
		t.Fatalf("tài khoản đã khoá: muốn ErrDangNhapThatBai, nhận %v", err)
	}
	if b.ghi.tim("INSERT INTO phien") != nil {
		t.Error("tài khoản đã khoá mà vẫn mở được phiên")
	}
	if n := b.ghi.soHangCanBoDaTra(); n != 0 {
		t.Errorf("CSDL trả %d dòng cho tài khoản đã khoá, muốn 0", n)
	}
}
