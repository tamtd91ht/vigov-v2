package app

import (
	"errors"
	"strings"
	"testing"
)

// Two defect classes, both silent, both in the sign-in path.
//
//  1. THE TOKEN SIGNED AFTER THE COMMIT. A signing failure then leaves the session row and its
//     audit entry committed: the archive of a public authority states that a member of staff
//     signed in at a moment when no cookie was issued and nobody signed in. Rule 6 measures the
//     trail by whether it is TRUE, not by whether the discrepancy is harmless — and an entry is
//     append-only, so it cannot be corrected afterwards.
//
//  2. TWO DIFFERENT LOG LINES FOR TWO KINDS OF FAILURE. The HTTP layer answers identically for a
//     wrong email and a wrong password so as not to hand over a directory of which addresses
//     exist on this commune's domain. Two distinguishable log lines hand it straight back, in
//     centralised logging, for every commune at once.

// --- the token is signed inside the transaction ------------------------------------------------

func TestKyTokenHongThiKhongCoPhienVaKhongCoVet(t *testing.T) {
	b := dungBanThu(t)
	b.ky.loi = errors.New("khoá ký không dùng được")

	kq, err := b.dangNhap.Chay(ctxXa(xaThu), yeuCauDung())
	if err == nil {
		t.Fatal("ký token hỏng mà đăng nhập vẫn báo thành công")
	}
	if kq.Token != "" || kq.Sid != "" {
		t.Errorf("trả về kết quả dù không ký được: %+v", kq)
	}

	if b.ghi.tim("INSERT INTO audit_log") != nil {
		t.Fatal("CÓ VẾT cho một lần đăng nhập không xảy ra — hồ sơ lưu trữ khẳng định một sự việc " +
			"không có thật, và vết là append-only nên không sửa lại được (luật 6)")
	}
	phien := b.ghi.tim("INSERT INTO phien")
	if phien == nil {
		t.Fatal("không có câu lệnh tạo phiên để kiểm")
	}
	if got := b.ghi.ketThucCua(phien.tx); got != "rollback" {
		t.Fatalf("giao dịch kết thúc bằng %q, muốn rollback — không ký được thì không có phiên", got)
	}
}

func TestKyTokenChayBenTrongGiaoDichCuaPhien(t *testing.T) {
	// The property itself, not just its consequence: signing happens while the transaction is
	// open. Signed outside it, an error has nothing left to roll back.
	b := dungBanThu(t)

	if _, err := b.dangNhap.Chay(ctxXa(xaThu), yeuCauDung()); err != nil {
		t.Fatal(err)
	}
	phien := b.ghi.tim("INSERT INTO phien")
	if phien == nil {
		t.Fatal("không tạo phiên")
	}
	if b.ky.goi != 1 {
		t.Fatalf("Ky gọi %d lần, muốn 1", b.ky.goi)
	}
	if b.ky.tx == 0 {
		t.Fatal("ký token chạy NGOÀI mọi giao dịch — ký hỏng thì vết đã commit mất rồi")
	}
	if b.ky.tx != phien.tx {
		t.Fatalf("ký ở giao dịch %d, phiên ở giao dịch %d — phải cùng một giao dịch",
			b.ky.tx, phien.tx)
	}
}

func TestDangNhapThanhCongTraVeTokenDaKy(t *testing.T) {
	b := dungBanThu(t)

	kq, err := b.dangNhap.Chay(ctxXa(xaThu), yeuCauDung())
	if err != nil {
		t.Fatal(err)
	}
	if kq.Token == "" {
		t.Fatal("không có token — tầng HTTP không còn ký nữa, không có gì đặt vào cookie")
	}
	// The claims are the ones this request is entitled to: this commune, this session.
	if !strings.Contains(kq.Token, string(xaThu)) || !strings.Contains(kq.Token, kq.Sid) {
		t.Errorf("token không mang đúng xã và sid: %q", kq.Token)
	}
	if strings.Contains(kq.Token, string(xaKhac)) {
		t.Error("token mang xã khác")
	}
	if kq.HetHanLuc.IsZero() {
		t.Error("thiếu hạn phiên")
	}
}

// --- one line for every failed sign-in ---------------------------------------------------------

// dongLog strips the timestamp so two lines can be compared for everything else.
func dongLog(t *testing.T, ra string) string {
	t.Helper()
	ra = strings.TrimSpace(ra)
	if ra == "" {
		t.Fatal("không ghi dòng log nào cho một lần đăng nhập thất bại — không điều tra được")
	}
	if i := strings.Index(ra, "level="); i >= 0 {
		ra = ra[i:]
	}
	return ra
}

func TestHaiKieuDangNhapThatBaiChoRaCungMotDongLog(t *testing.T) {
	// Wrong password and unknown email must be INDISTINGUISHABLE in the log, exactly as they are
	// in the response. Otherwise counting two kinds of line rebuilds the staff directory of every
	// commune from centralised logging.
	saiMatKhau := dungBanThu(t)
	yc := yeuCauDung()
	yc.MatKhau = "mat-khau-sai-hoan-toan"
	if _, err := saiMatKhau.dangNhap.Chay(ctxXa(xaThu), yc); !errors.Is(err, ErrDangNhapThatBai) {
		t.Fatalf("muốn ErrDangNhapThatBai, nhận %v", err)
	}

	khongCoEmail := dungBanThu(t)
	khongCoEmail.ghi.khongCoNguoiDung = true
	if _, err := khongCoEmail.dangNhap.Chay(ctxXa(xaThu), yeuCauDung()); !errors.Is(err, ErrDangNhapThatBai) {
		t.Fatalf("muốn ErrDangNhapThatBai, nhận %v", err)
	}

	a := dongLog(t, saiMatKhau.log.String())
	b := dongLog(t, khongCoEmail.log.String())
	if a != b {
		t.Fatalf("hai kiểu đăng nhập thất bại ghi hai dòng KHÁC NHAU — đếm hai loại dòng là dò được "+
			"danh bạ cán bộ:\n  sai mật khẩu: %s\n  không có email: %s", a, b)
	}
	if strings.Contains(a, "mật khẩu") || strings.Contains(a, "tài khoản") {
		t.Errorf("dòng log tự nói ra là trường hợp nào: %s", a)
	}
}

func TestLogDangNhapThatBaiKhongChuaEmailThoNhungCoVanTayVaXa(t *testing.T) {
	// The email on this route is an unauthenticated, client-supplied string: anybody can write
	// anything into the log at Info level, and a citizen mistyping their own address into the
	// staff screen leaves their email there (Decree 13/2023, Article 9). The fingerprint keeps
	// what an administrator actually needs — correlating repeated attempts.
	b := dungBanThu(t)
	yc := yeuCauDung()
	yc.MatKhau = "mat-khau-sai-hoan-toan"
	if _, err := b.dangNhap.Chay(ctxXa(xaThu), yc); err == nil {
		t.Fatal("muốn lỗi")
	}

	ra := b.log.String()
	if strings.Contains(ra, emailCB) {
		t.Errorf("email thô nằm trong log: %s", ra)
	}
	if strings.Contains(ra, maCanBo) {
		t.Errorf("mã cán bộ nằm trong log — nói ra rằng tài khoản CÓ TỒN TẠI: %s", ra)
	}
	if !strings.Contains(ra, "email_van_tay="+vanTay(emailCB)) {
		t.Errorf("thiếu vân tay email — không tương quan được nhiều lần thử: %s", ra)
	}
	if !strings.Contains(ra, "xa="+string(xaThu)) {
		t.Errorf("dòng log thiếu xã — một tiến trình phục vụ 200+ xã ghi chung một luồng log: %s", ra)
	}
	if !strings.Contains(ra, "ip="+ipGia) {
		t.Errorf("dòng log thiếu IP: %s", ra)
	}
}

func TestVanTayKhongPhatLaiDuocVaTuongQuanDuoc(t *testing.T) {
	if vanTay(emailCB) == emailCB || strings.Contains(vanTay(emailCB), "@") {
		t.Error("vân tay vẫn đọc ra được địa chỉ")
	}
	if vanTay(emailCB) != vanTay(strings.ToUpper(emailCB)+"  ") {
		t.Error("cùng một địa chỉ gõ khác kiểu cho hai vân tay — không tương quan được")
	}
	if vanTay(emailCB) == vanTay("nguoi.khac@example.gov.vn") {
		t.Error("hai địa chỉ khác nhau cho cùng một vân tay")
	}
}
