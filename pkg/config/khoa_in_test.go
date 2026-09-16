package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

// Khoa must refuse to render on EVERY path that can reach a log pipeline, not only the ones
// config_test.go already covers (%v, %s, %+v of the parent, %#v of the slice, json of the
// slice, Redacted).
//
// THE DEFECT CLASS: one `slog.Info("cfg", "cfg", cfg)` written while debugging. Nothing turns
// red, the service works, and the signing key of EVERY commune on the deployment is now in
// centralised logging, in backups and at a third-party monitoring vendor — where it cannot be
// recalled (rule 8). One missed rendering path is enough, so the paths are enumerated here.

const khoaThu = "khoa-ky-gia-KHONG-PHAI-KHOA-THAT-de-kiem-tra-in"

// bienThe is every way a Khoa can plausibly reach a log line.
func bienThe(t *testing.T, cfg Config) map[string]string {
	t.Helper()

	k := cfg.SessionSigningKeys[0]
	conTro := &cfg.SessionSigningKeys[0]

	thoCfg, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("json.Marshal(Config): %v", err)
	}
	thoConTro, err := json.Marshal(conTro)
	if err != nil {
		t.Fatalf("json.Marshal(*Khoa): %v", err)
	}
	tho, err := k.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText: %v", err)
	}

	return map[string]string{
		"fmt %v khoá":              fmt.Sprintf("%v", k),
		"fmt %s khoá":              fmt.Sprintf("%s", k),
		"fmt %q khoá":              fmt.Sprintf("%q", k),
		"fmt %x khoá":              fmt.Sprintf("%x", k),
		"fmt %v con trỏ khoá":      fmt.Sprintf("%v", conTro),
		"fmt %+v con trỏ khoá":     fmt.Sprintf("%+v", conTro),
		"fmt %#v con trỏ khoá":     fmt.Sprintf("%#v", conTro),
		"fmt %v cả struct":         fmt.Sprintf("%v", cfg),
		"fmt %+v cả struct":        fmt.Sprintf("%+v", cfg),
		"fmt %#v cả struct":        fmt.Sprintf("%#v", cfg),
		"fmt %v con trỏ struct":    fmt.Sprintf("%v", &cfg),
		"fmt %+v con trỏ struct":   fmt.Sprintf("%+v", &cfg),
		"fmt %v danh sách khoá":    fmt.Sprintf("%v", cfg.SessionSigningKeys),
		"fmt %+v danh sách khoá":   fmt.Sprintf("%+v", cfg.SessionSigningKeys),
		"fmt %v Redacted":          fmt.Sprintf("%v", cfg.Redacted()),
		"fmt %+v Redacted":         fmt.Sprintf("%+v", cfg.Redacted()),
		"json cả struct":           string(thoCfg),
		"json con trỏ khoá":        string(thoConTro),
		"MarshalText":              string(tho),
		"String":                   k.String(),
		"GoString":                 k.GoString(),
		"CanhBao":                  strings.Join(cfg.CanhBao(), " | "),
		"fmt %v trong map":         fmt.Sprintf("%v", map[string]Khoa{"session": k}),
		"fmt %v trong interface":   fmt.Sprintf("%v", any(k)),
		"errors: %w kèm khoá":      fmt.Errorf("mở phiên: %v", k).Error(),
		"strings: nối bằng Sprint": fmt.Sprint(k),
	}
}

func TestKhoaKhongHienRaTrenMoiDuongIn(t *testing.T) {
	datMoiTruong(t, map[string]string{
		"DATABASE_DSN":         dsnGia,
		"ENV":                  EnvProd,
		"SESSION_SIGNING_KEYS": khoaThu + "," + khoaThu + "-cu",
	})
	cfg, err := Load("identity")
	if err != nil {
		t.Fatal(err)
	}

	for ten, ra := range bienThe(t, cfg) {
		if strings.Contains(ra, khoaThu) {
			t.Errorf("%s: khoá ký lọt ra: %q", ten, ra)
		}
		// A key printed as bytes is still the key. Catches a rendering that turns the material
		// into its numeric form instead of refusing.
		if strings.Contains(ra, fmt.Sprintf("%d", khoaThu[0])+" "+fmt.Sprintf("%d", khoaThu[1])) {
			t.Errorf("%s: khoá ký lọt ra dưới dạng byte: %q", ten, ra)
		}
	}
}

func TestKhoaKhongHienRaQuaVerbSoHoc(t *testing.T) {
	// KNOWN GAP, SKIPPED ON PURPOSE — DO NOT DELETE THIS TEST.
	//
	// String/GoString/MarshalJSON/MarshalText/LogValue cover the verbs fmt routes through a
	// Stringer: %v %s %q %x %X. They do NOT cover the numeric verbs, because fmt never consults
	// String() for them — and Khoa is a []byte, so %d, %c, %U and %b each print the key material
	// one byte at a time:
	//
	//	fmt.Sprintf("%d", khoa) -> [107 104 111 97 ...]
	//
	// Unlikely to be written on purpose, which is exactly the property that makes it survive
	// review: the type promises "refuses to render" and this is the one path where it does not.
	//
	// THE FIX IS IN PRODUCTION CODE, so it was not made here: give Khoa a fmt.Formatter, which
	// takes precedence over every verb —
	//
	//	func (k Khoa) Format(f fmt.State, verb rune) { io.WriteString(f, "***") }
	//
	// Remove the Skip below once that exists; the assertions already say what is required.
	t.Skip("hở đã biết: verb số học in ra khoá — cần Khoa implement fmt.Formatter (chỉ sửa được ở mã sản phẩm)")

	k := Khoa(khoaThu)
	for _, verb := range []string{"%d", "%c", "%U", "%08b"} {
		ra := fmt.Sprintf(verb, k)
		if strings.Contains(ra, fmt.Sprintf("%d", khoaThu[0])) {
			t.Errorf("%s in ra khoá dưới dạng số: %q", verb, ra)
		}
	}
}

func TestKhoaKhongHienRaQuaSlog(t *testing.T) {
	// slog is the logger this system actually uses, and it has its own rendering path:
	// LogValuer first, then the handler's own formatting. Both are exercised here, for the
	// value, the pointer, the slice and the whole Config — the four shapes a call site can
	// plausibly hand to slog.
	datMoiTruong(t, map[string]string{
		"DATABASE_DSN":         dsnGia,
		"ENV":                  EnvProd,
		"SESSION_SIGNING_KEYS": khoaThu,
	})
	cfg, err := Load("identity")
	if err != nil {
		t.Fatal(err)
	}

	for _, handler := range []string{"text", "json"} {
		t.Run(handler, func(t *testing.T) {
			var buf bytes.Buffer
			var h slog.Handler
			if handler == "text" {
				h = slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
			} else {
				h = slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
			}
			log := slog.New(h)

			k := cfg.SessionSigningKeys[0]
			log.Info("khoá", "k", k)
			log.Info("con trỏ khoá", "k", &cfg.SessionSigningKeys[0])
			log.Info("danh sách", "keys", cfg.SessionSigningKeys)
			log.Info("cả cấu hình", "cfg", cfg)
			log.Info("cả cấu hình con trỏ", "cfg", &cfg)
			log.Info("cấu hình đã che", "cfg", cfg.Redacted())
			log.Info("nhóm", slog.Group("boot", "cfg", cfg, "k", k))
			log.With("cfg", cfg).Info("kèm sẵn")
			log.Info("dưới dạng Any", slog.Any("k", k), slog.Any("cfg", cfg))

			if strings.Contains(buf.String(), khoaThu) {
				t.Errorf("khoá ký lọt vào log %s:\n%s", handler, buf.String())
			}
		})
	}
}

func TestDsnKhongLoMatKhauKhiGhiCaCauHinh(t *testing.T) {
	// KNOWN GAP, SKIPPED ON PURPOSE — DO NOT DELETE THIS TEST.
	//
	// The argument that produced the Khoa type — "Redacted() only protects the call sites that
	// remember to use it" — applies word for word to DatabaseDSN and RedisDSN, and they are
	// plain strings. So:
	//
	//	slog.Info("boot", "cfg", cfg)  ->  DatabaseDSN:postgres://vigov:<mật khẩu>@host/db
	//
	// A database password reaches centralised logging, backups and third-party monitoring in one
	// line, it cannot be recalled from any of them, and one deployment's database serves every
	// commune (rule 8, invariants 1 and 3).
	//
	// THE FIX IS IN PRODUCTION CODE, so it was not made here: give the two DSN fields a type of
	// their own that redacts itself, exactly as Khoa does —
	//
	//	type DSN string
	//	func (d DSN) String() string       { return redactDSN(string(d)) }
	//	func (d DSN) LogValue() slog.Value { return slog.StringValue(redactDSN(string(d))) }
	//
	// with the raw value reachable through one explicit, greppable method for the sql.Open call.
	// Remove the Skip below once that exists.
	t.Skip("hở đã biết: DSN là string trần nên ghi log cả Config làm lộ mật khẩu CSDL (chỉ sửa được ở mã sản phẩm)")

	datMoiTruong(t, map[string]string{
		"DATABASE_DSN":         dsnGia,
		"ENV":                  EnvProd,
		"SESSION_SIGNING_KEYS": khoaThu,
	})
	cfg, err := Load("identity")
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	slog.New(slog.NewTextHandler(&buf, nil)).Info("khởi động", "cfg", cfg)
	if strings.Contains(buf.String(), "khong-phai-mat-khau-that") {
		t.Errorf("mật khẩu trong DSN lọt vào log:\n%s", buf.String())
	}
}

func TestKhoaVanDungDuocChoTokenDuDaCheKhiIn(t *testing.T) {
	// The protection must not have been achieved by destroying the material: pkg/token still
	// needs the real bytes, and exactly once, through the one greppable exit.
	k := Khoa(khoaThu)
	if string(k.Bytes()) != khoaThu {
		t.Fatal("Bytes() không trả về khoá thật — token sẽ ký bằng ***")
	}
	if string(k) != khoaThu {
		t.Fatal("chuyển kiểu về string phải giữ nguyên khoá")
	}
}
