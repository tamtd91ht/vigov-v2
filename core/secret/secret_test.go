package secret

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"testing"
)

// THE DEFECT CLASS THIS FILE CLOSES: a secret reaching a log line through a rendering path
// nobody thought of. The paths are ENUMERATED rather than sampled, because one missed path is
// the whole leak — the service keeps working perfectly while the key streams out.
//
// khoaThu and matKhauThu are fake. The text says so, so a scanner and a reviewer can both tell
// at a glance (rule 8, forbidden #1).
const (
	khoaThu    = "khoa-ky-gia-KHONG-PHAI-KHOA-THAT-de-kiem-tra-in"
	matKhauThu = "khong-phai-mat-khau-that"
	dsnThu     = "postgres://vigov:" + matKhauThu + "@db.noi-bo:5432/vigov?sslmode=require"
)

// loKhoa reports whether an output leaks the material, in EITHER shape it can take: the text
// itself, or the bytes printed as numbers by a numeric verb.
func loKhoa(ra string) bool {
	if strings.Contains(ra, khoaThu) {
		return true
	}
	// "[107 104 111 ..." — what %d does to a []byte with no Formatter.
	var so []string
	// "[k h o a ..." — what %c does to the same bytes. A key printed one character at a time is
	// still the key: a grep over the log pipeline for the material would miss it, a person
	// reading the line would not.
	var ky []string
	for _, b := range []byte(khoaThu)[:6] {
		so = append(so, fmt.Sprint(b))
		ky = append(ky, string(rune(b)))
	}
	return strings.Contains(ra, strings.Join(so, " ")) || strings.Contains(ra, strings.Join(ky, " "))
}

// --- Secret: every verb ---------------------------------------------------------------------

func TestSecretKhongLoQuaBatKyVerbNao(t *testing.T) {
	// %d %c %U %b %o ARE THE POINT OF THIS TEST. fmt never consults String() for a numeric
	// verb, so before fmt.Formatter existed on this type each of them printed the key one byte
	// at a time while the type advertised itself as "refuses to render".
	k := Secret(khoaThu)
	conTro := &k

	// The assertion is EQUALITY, not "does not contain the key". Every verb has to come out of
	// Format, so every verb produces exactly the placeholder — and a verb that renders anything
	// else at all is a verb that bypassed Format, whether or not this particular fake key
	// happens to be visible in it.
	verbs := []string{"%v", "%+v", "%#v", "%s", "%q", "%x", "%X", "%d", "%c", "%U", "%b", "%o", "%08b", "%e", "%p"}
	for _, verb := range verbs {
		muon := Che
		switch verb {
		case "%q":
			muon = strconv.Quote(Che)
		case "%p":
			// fmt documents TWO verbs it never routes through Formatter: %p and %T. Neither can
			// leak the material — one prints an address, the other a type name — so the equality
			// check is skipped here while the leak check below still runs.
			muon = ""
		}
		t.Run("giá trị "+verb, func(t *testing.T) {
			ra := fmt.Sprintf(verb, k)
			if loKhoa(ra) {
				t.Fatalf("%s làm lộ khoá: %q", verb, ra)
			}
			if muon != "" && ra != muon {
				t.Fatalf("%s = %q, muốn %q — verb này không đi qua Format", verb, ra, muon)
			}
		})
		t.Run("con trỏ "+verb, func(t *testing.T) {
			// A pointer taken while debugging must be covered too — that is what the VALUE
			// receiver on Format buys.
			ra := fmt.Sprintf(verb, conTro)
			if loKhoa(ra) {
				t.Fatalf("%s trên con trỏ làm lộ khoá: %q", verb, ra)
			}
			if muon != "" && ra != muon {
				t.Fatalf("%s trên con trỏ = %q, muốn %q", verb, ra, muon)
			}
		})
	}
}

func TestSecretKhongLoQuaMoiHinhDangChuaNo(t *testing.T) {
	k := Secret(khoaThu)

	type capHinh struct {
		Ten       string
		KhoaKy    Secret
		DanhSach  []Secret
		ConTroKey *Secret
	}
	cfg := capHinh{Ten: "identity", KhoaKy: k, DanhSach: []Secret{k, k}, ConTroKey: &k}

	thoJSON, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	thoKhoa, err := json.Marshal(k)
	if err != nil {
		t.Fatalf("json.Marshal(Secret): %v", err)
	}
	vanBan, err := k.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText: %v", err)
	}

	cases := map[string]string{
		"struct %v":        fmt.Sprintf("%v", cfg),
		"struct %+v":       fmt.Sprintf("%+v", cfg),
		"struct %#v":       fmt.Sprintf("%#v", cfg),
		"con trỏ struct":   fmt.Sprintf("%+v", &cfg),
		"slice %v":         fmt.Sprintf("%v", cfg.DanhSach),
		"slice %d":         fmt.Sprintf("%d", cfg.DanhSach),
		"map":              fmt.Sprintf("%v", map[string]Secret{"session": k}),
		"interface":        fmt.Sprintf("%v", any(k)),
		"Sprint":           fmt.Sprint(k),
		"Sprintln":         fmt.Sprintln(k),
		"fmt.Errorf":       fmt.Errorf("mở phiên với khoá %v: hỏng", k).Error(),
		"fmt.Errorf %d":    fmt.Errorf("mở phiên với khoá %d: hỏng", k).Error(),
		"json cả struct":   string(thoJSON),
		"json riêng khoá":  string(thoKhoa),
		"MarshalText":      string(vanBan),
		"String":           k.String(),
		"GoString":         k.GoString(),
		"nối chuỗi Sprint": fmt.Sprint("khoá=", k),
	}
	for ten, ra := range cases {
		if loKhoa(ra) {
			t.Errorf("%s làm lộ khoá: %q", ten, ra)
		}
	}
}

func TestSecretKhongLoQuaSlog(t *testing.T) {
	// slog is the logger this system actually uses, and it has its own rendering order:
	// LogValuer first, then the handler's own formatting. Both handlers, and every shape a
	// call site plausibly hands over.
	k := Secret(khoaThu)
	type capHinh struct {
		Ten    string
		KhoaKy Secret
	}
	cfg := capHinh{Ten: "identity", KhoaKy: k}

	for _, ten := range []string{"text", "json"} {
		t.Run(ten, func(t *testing.T) {
			var buf bytes.Buffer
			var h slog.Handler
			if ten == "text" {
				h = slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
			} else {
				h = slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
			}
			log := slog.New(h)

			log.Info("khoá", "k", k)
			log.Info("con trỏ khoá", "k", &k)
			log.Info("danh sách", "keys", []Secret{k})
			log.Info("cả cấu hình", "cfg", cfg)
			log.Info("cả cấu hình con trỏ", "cfg", &cfg)
			log.Info("nhóm", slog.Group("boot", "cfg", cfg, "k", k))
			log.With("cfg", cfg).With("k", k).Info("kèm sẵn")
			log.Info("dưới dạng Any", slog.Any("k", k), slog.Any("cfg", cfg))
			log.Error("hỏng lúc ký", "err", fmt.Errorf("ký bằng %v: hỏng", k))

			if loKhoa(buf.String()) {
				t.Fatalf("khoá lọt vào log %s:\n%s", ten, buf.String())
			}
		})
	}
}

func TestSecretVanLayRaDuocDeKy(t *testing.T) {
	// The protection must not have been achieved by destroying the material: pkg/token still
	// needs the real bytes, through the one exit whose name says what it does.
	k := Secret(khoaThu)

	if string(k.Lo()) != khoaThu {
		t.Fatal("Lo() không trả về khoá thật — token sẽ ký bằng ***")
	}
	if k.Rong() {
		t.Fatal("Rong() nói là rỗng trong khi có khoá")
	}
	if !Secret(nil).Rong() {
		t.Fatal("Secret rỗng phải báo Rong()")
	}
	// Lo() must hand over the material, not a copy of the placeholder.
	if len(k.Lo()) != len(khoaThu) {
		t.Fatalf("Lo() dài %d byte, muốn %d", len(k.Lo()), len(khoaThu))
	}
}

// --- DSN: the password goes, the address stays ----------------------------------------------

func TestDsnCheMatKhauNhungGiuDuocDiaChi(t *testing.T) {
	// BOTH HALVES MATTER. Leaking the password is rule 8; redacting the whole string is how a
	// service pointed at the wrong database by a bad deploy becomes undiagnosable from its own
	// startup line.
	d := DSN(dsnThu)

	for _, ra := range []string{
		fmt.Sprintf("%v", d), fmt.Sprintf("%+v", d), fmt.Sprintf("%#v", d),
		fmt.Sprintf("%s", d), fmt.Sprintf("%q", d), fmt.Sprintf("%d", d), fmt.Sprintf("%x", d),
		fmt.Sprint(d), d.String(), d.GoString(), fmt.Errorf("mở CSDL %v: hỏng", d).Error(),
	} {
		if strings.Contains(ra, matKhauThu) {
			t.Fatalf("mật khẩu lọt ra: %q", ra)
		}
	}

	an := d.String()
	if !strings.Contains(an, "db.noi-bo:5432") {
		t.Errorf("che quá tay, mất thông tin chẩn đoán: %q", an)
	}
	if !strings.Contains(an, "postgres://vigov:") {
		t.Errorf("scheme và tên người dùng phải giữ lại: %q", an)
	}
	if d.Lo() != dsnThu {
		t.Error("Lo() phải trả về DSN thật — sql.Open sẽ không nối được")
	}
}

func TestDsnKhongLoQuaJsonVaSlog(t *testing.T) {
	type capHinh struct {
		DatabaseDSN DSN
		RedisDSN    DSN
	}
	cfg := capHinh{DatabaseDSN: DSN(dsnThu), RedisDSN: DSN("redis://vigov:" + matKhauThu + "@cache:6379/0")}

	tho, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(tho), matKhauThu) {
		t.Fatalf("mật khẩu lọt ra qua JSON: %s", tho)
	}

	for _, ten := range []string{"text", "json"} {
		var buf bytes.Buffer
		var h slog.Handler
		if ten == "text" {
			h = slog.NewTextHandler(&buf, nil)
		} else {
			h = slog.NewJSONHandler(&buf, nil)
		}
		log := slog.New(h)
		log.Info("khởi động", "cfg", cfg)
		log.Info("khởi động", "dsn", cfg.DatabaseDSN)
		log.With("cfg", cfg).Info("kèm sẵn")

		if strings.Contains(buf.String(), matKhauThu) {
			t.Fatalf("mật khẩu lọt vào log %s:\n%s", ten, buf.String())
		}
		// And the host still has to be readable, or the line is worthless.
		if !strings.Contains(buf.String(), "db.noi-bo") {
			t.Errorf("log %s mất địa chỉ CSDL:\n%s", ten, buf.String())
		}
	}
}

func TestDsnKhongPhaiUrlThiCheCaChuoi(t *testing.T) {
	// An unparseable string cannot be split into "address" and "credential", and guessing wrong
	// prints the password. Refusing to guess is the only safe reading.
	cases := map[string]string{
		"không phải URL":               "day-khong-phai-dsn-" + matKhauThu,
		"không có thông tin đăng nhập": "postgres://localhost:5432/vigov",
		"rỗng":                         "",
	}
	for ten, raw := range cases {
		t.Run(ten, func(t *testing.T) {
			got := DSN(raw).String()
			if strings.Contains(got, matKhauThu) {
				t.Errorf("rò rỉ: %q", got)
			}
		})
	}
	if got := DSN("postgres://localhost:5432/vigov").String(); got != "postgres://localhost:5432/vigov" {
		t.Errorf("DSN không có mật khẩu phải giữ nguyên, nhận %q", got)
	}
	if got := DSN("").String(); got != "" {
		t.Errorf("DSN rỗng phải in ra rỗng, nhận %q", got)
	}
}
