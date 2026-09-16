package password

import (
	"errors"
	"strings"
	"testing"
)

// Never a real password in source, even in a test (rule 8).
const matKhauThu = "mat-khau-thu-nghiem-khong-that"

func TestBamRoiKiemTra(t *testing.T) {
	bam, err := Bam(matKhauThu)
	if err != nil {
		t.Fatalf("Bam lỗi: %v", err)
	}
	if err := KiemTra(matKhauThu, bam); err != nil {
		t.Errorf("mật khẩu đúng mà bị từ chối: %v", err)
	}
}

func TestSaiMatKhauBiTuChoi(t *testing.T) {
	bam, err := Bam(matKhauThu)
	if err != nil {
		t.Fatal(err)
	}
	if err := KiemTra(matKhauThu+"x", bam); !errors.Is(err, ErrSaiMatKhau) {
		t.Errorf("mật khẩu sai mà qua được, lỗi = %v", err)
	}
}

func TestMoiLanBamMotKhac(t *testing.T) {
	// A fresh salt each time. Identical hashes for identical passwords would tell an attacker
	// holding the table which accounts share a password — and one cracked hash would open all
	// of them at once.
	a, err := Bam(matKhauThu)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Bam(matKhauThu)
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Error("hai lần băm cùng mật khẩu cho ra cùng chuỗi — muối không ngẫu nhiên")
	}
	// Both must still verify.
	if err := KiemTra(matKhauThu, a); err != nil {
		t.Error("băm thứ nhất không kiểm được")
	}
	if err := KiemTra(matKhauThu, b); err != nil {
		t.Error("băm thứ hai không kiểm được")
	}
}

func TestMatKhauNganBiTuChoi(t *testing.T) {
	// Length beats composition rules: forcing symbols produces a sticky note, while length is
	// what actually costs an attacker time.
	if _, err := Bam("ngan"); !errors.Is(err, ErrMatKhauNgan) {
		t.Errorf("mật khẩu ngắn phải bị từ chối, lỗi = %v", err)
	}
	vuaDu := strings.Repeat("a", DaiToiThieu)
	if _, err := Bam(vuaDu); err != nil {
		t.Errorf("mật khẩu đủ dài bị từ chối: %v", err)
	}
}

func TestBamKhongHopLe(t *testing.T) {
	// A malformed hash must be an error, never an accidental pass. Returning nil here would let
	// any password through for a corrupted row.
	for ten, xau := range map[string]string{
		"rỗng":            "",
		"không đúng dạng": "khong-phai-bam",
		"thiếu phần":      "$argon2id$v=19$m=65536,t=2,p=1$abc",
		"sai thuật toán":  "$bcrypt$v=19$m=65536,t=2,p=1$YWJj$YWJj",
		"muối hỏng":       "$argon2id$v=19$m=19456,t=2,p=1$!!!$YWJj",
	} {
		t.Run(ten, func(t *testing.T) {
			if err := KiemTra(matKhauThu, xau); err == nil {
				t.Error("chuỗi băm hỏng mà vẫn cho qua")
			}
		})
	}
}

func TestCanBamLai(t *testing.T) {
	bam, err := Bam(matKhauThu)
	if err != nil {
		t.Fatal(err)
	}
	if CanBamLai(bam) {
		t.Error("băm vừa tạo bằng tham số hiện tại mà lại báo cần băm lại")
	}

	// A hash made with weaker parameters must be flagged, so it gets upgraded at the next
	// successful sign-in instead of staying weak forever.
	yeu := "$argon2id$v=19$m=4096,t=1,p=1$YWJjZGVmZ2hpamtsbW5vcA$YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NQ"
	if !CanBamLai(yeu) {
		t.Error("băm bằng tham số yếu mà không báo cần băm lại")
	}

	if !CanBamLai("khong-doc-duoc") {
		t.Error("chuỗi không đọc được thì phải băm lại")
	}
}

func TestBamKhongChuaMatKhau(t *testing.T) {
	// The encoded hash is stored in the database and may appear in a backup. It must never
	// carry the password itself.
	bam, err := Bam(matKhauThu)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(bam, matKhauThu) {
		t.Error("mật khẩu xuất hiện nguyên văn trong chuỗi băm")
	}
}
