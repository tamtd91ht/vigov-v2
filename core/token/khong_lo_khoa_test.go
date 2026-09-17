package token

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/secret"
)

// THE DEFECT CLASS THIS FILE CLOSES: a Signer printed into a log line.
//
//	sig, _ := NewSigner(cfg.KhoaKyBytes())
//	log.Info("signer sẵn sàng", "signer", sig)
//
// Without String/GoString/LogValue, fmt and slog walk the unexported field by reflection and
// write the WHOLE PLATFORM'S session signing keys, as raw bytes, into centralised logging,
// backups and third-party monitoring. A secret that reaches any of those cannot be recalled
// (rule 8, invariant 1), and one leaked signing key forges a session in EVERY commune.
//
// Nothing here turns red on its own: the sign-in flow keeps working perfectly while the keys
// stream out. That is exactly why it needs a test.

// khoaBiMat is fake key material whose bytes are easy to spot in any output. Never a real key in
// source (rule 8, forbidden #1).
var khoaBiMat = secret.Secret("khoa-ky-gia-KHONG-PHAI-KHOA-THAT-bimat")

// loKhoa reports whether an output leaks the key, in any of the shapes fmt and slog produce it:
// the string itself, or the byte slice printed as numbers.
func loKhoa(t *testing.T, ra string) bool {
	t.Helper()
	if strings.Contains(ra, string(khoaBiMat)) {
		return true
	}
	// [107 104 111 97 ...] — what %v does to a []byte inside a struct.
	var so []string
	for _, b := range khoaBiMat[:6] {
		so = append(so, fmt.Sprint(b))
	}
	return strings.Contains(ra, strings.Join(so, " "))
}

func signerThu(t *testing.T) *Signer {
	t.Helper()
	s, err := NewSigner([]secret.Secret{khoaBiMat, khoaCu})
	if err != nil {
		t.Fatalf("NewSigner lỗi: %v", err)
	}
	return s
}

func TestInSignerKhongBaoGioLoKhoaKy(t *testing.T) {
	s := signerThu(t)

	// Every verb somebody reaches for while debugging, on the pointer AND on a dereferenced
	// copy — a value receiver is what makes the second case safe.
	cases := map[string]string{
		"%v con trỏ":   fmt.Sprintf("%v", s),
		"%+v con trỏ":  fmt.Sprintf("%+v", s),
		"%#v con trỏ":  fmt.Sprintf("%#v", s),
		"%s con trỏ":   fmt.Sprintf("%s", s),
		"%v giá trị":   fmt.Sprintf("%v", *s),
		"%+v giá trị":  fmt.Sprintf("%+v", *s),
		"%#v giá trị":  fmt.Sprintf("%#v", *s),
		"Sprint":       fmt.Sprint(s),
		"String() gọi": s.String(),
	}
	for ten, ra := range cases {
		if loKhoa(t, ra) {
			t.Errorf("%s LÀM LỘ KHOÁ KÝ: %s", ten, ra)
		}
		if !strings.Contains(ra, "token.Signer(2 khoá)") {
			t.Errorf("%s = %q, muốn \"token.Signer(2 khoá)\" — số khoá là thứ DUY NHẤT được nói ra", ten, ra)
		}
	}
}

func TestSignerNamTrongStructKhacInRaCungKhongLoKhoa(t *testing.T) {
	// The real shape of the accident: a wiring struct printed whole while debugging. fmt reaches
	// the field by reflection, and the Stringer on the field is the only thing between the key
	// and the log pipeline.
	type deps struct {
		Ten    string
		Signer *Signer
	}
	d := deps{Ten: "identity", Signer: signerThu(t)}

	for _, ra := range []string{fmt.Sprintf("%v", d), fmt.Sprintf("%+v", d)} {
		if loKhoa(t, ra) {
			t.Fatalf("in cả struct làm lộ khoá ký: %s", ra)
		}
		if !strings.Contains(ra, "token.Signer(") {
			t.Errorf("trường Signer in ra %q — không đi qua String()", ra)
		}
	}
}

func TestSlogKhongLoKhoaKy(t *testing.T) {
	// slog is the path that actually reaches centralised logging. LogValue is checked before
	// reflection, so it is the one that has to refuse.
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	s := signerThu(t)

	log.Info("signer sẵn sàng", "signer", s)
	log.Info("signer sẵn sàng (giá trị)", "signer", *s)
	log.Error("hỏng lúc ký", "signer", s, "so_khoa", s.SoKhoa())

	ra := buf.String()
	if loKhoa(t, ra) {
		t.Fatalf("log LÀM LỘ KHOÁ KÝ:\n%s", ra)
	}
	if !strings.Contains(ra, "token.Signer(2 khoá)") {
		t.Errorf("log không mang được số khoá — dòng khởi động mất thông tin cần thiết:\n%s", ra)
	}
}

func TestVanKyVaGiaiDuocSauKhiBitDuongIn(t *testing.T) {
	// The refusal must not be a refusal to work: the keys are still there, they are merely not
	// printable.
	s := signerThu(t)
	tok := kyDuoc(t, s, claimMau())
	if _, err := s.Giai(tok); err != nil {
		t.Fatalf("Giai lỗi sau khi thêm String/LogValue: %v", err)
	}
	if s.SoKhoa() != 2 {
		t.Errorf("SoKhoa = %d, muốn 2", s.SoKhoa())
	}
	if strings.Contains(s.String(), tok) {
		t.Error("String() nhắc lại token")
	}
}
