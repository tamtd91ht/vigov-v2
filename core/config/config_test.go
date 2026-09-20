package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/secret"
)

// dsnGia is a fake DSN. Never a real credential in source (rule 8, forbidden #1).
const dsnGia = "postgres://vigov:khong-phai-mat-khau-that@localhost:5432/vigov_test"

// redisGia is the same fake credential on the cache DSN.
const redisGia = "redis://vigov:khong-phai-mat-khau-that@localhost:6379/0"

// khoaGia is fake signing key material. The text says so on purpose: a scanner and a reviewer
// must both be able to tell at a glance that this is not a real key (rule 8, forbidden #1).
const khoaGia = "khoa-ky-gia-KHONG-PHAI-KHOA-THAT-cho-test"

// nenBatBuoc is the set of variables Load requires that most tests here are NOT about.
//
// WHY A BASELINE AND NOT A LINE IN EVERY MAP: GRPC_CALLER_KEY became required for every service
// in every environment (ADR 0025, invariant 3), and twenty tests that are about DSNs, TTLs and
// signing keys would each have to carry it. The copies drift, and the drift shows up as a test
// failing for a reason that has nothing to do with what it is testing.
//
// IT DOES NOT WEAKEN ANYTHING. A test that needs the variable ABSENT clears it with t.Setenv
// after this helper runs — the last Setenv wins — and the refusal is asserted by name in
// TestLoadThieuBienThiHong below. Nothing here is asserted true because the baseline hid it.
var nenBatBuoc = map[string]string{"GRPC_CALLER_KEY": khoaGoiNoiBoGia}

// khoaGoiNoiBoGia is fake key material for the inter-service caller key (rule 8, forbidden #1).
const khoaGoiNoiBoGia = "khoa-goi-noi-bo-GIA-KHONG-PHAI-KHOA-THAT"

func datMoiTruong(t *testing.T, cap map[string]string) {
	t.Helper()
	for k, v := range nenBatBuoc {
		if _, co := cap[k]; !co {
			t.Setenv(k, v)
		}
	}
	for k, v := range cap {
		t.Setenv(k, v)
	}
}

func TestLoadDayDu(t *testing.T) {
	datMoiTruong(t, map[string]string{
		"DATABASE_DSN":     dsnGia,
		"ENV":              EnvDev,
		"LISTEN_ADDR":      ":8081",
		"TENANT_CACHE_TTL": "15s",
	})

	cfg, err := Load("platform")
	if err != nil {
		t.Fatalf("Load lỗi: %v", err)
	}
	if cfg.ListenAddr != ":8081" {
		t.Errorf("ListenAddr = %q", cfg.ListenAddr)
	}
	if cfg.TenantCacheTTL != 15*time.Second {
		t.Errorf("TenantCacheTTL = %v", cfg.TenantCacheTTL)
	}
	if cfg.Env != EnvDev {
		t.Errorf("Env = %q", cfg.Env)
	}
}

func TestLoadThieuBienThiHong(t *testing.T) {
	// Failing beats defaulting. A service that starts against a guessed database gets debugged
	// at the wrong layer, and on the isolation path a wrong guess is a data breach.
	cases := []struct {
		ten string
		moi map[string]string
	}{
		{"thiếu DSN", map[string]string{"ENV": EnvDev}},
		{"thiếu ENV", map[string]string{"DATABASE_DSN": dsnGia}},
		{"thiếu cả hai", map[string]string{}},
		// Required in EVERY environment, dev included: a missing DSN produces a service that
		// cannot answer, a missing caller key would produce a gRPC port that answers anything
		// reaching it (ADR 0025, invariant 3).
		{"thiếu khoá gọi nội bộ", map[string]string{
			"DATABASE_DSN": dsnGia, "ENV": EnvDev, "GRPC_CALLER_KEY": ""}},
	}
	for _, c := range cases {
		t.Run(c.ten, func(t *testing.T) {
			t.Setenv("DATABASE_DSN", "")
			t.Setenv("ENV", "")
			datMoiTruong(t, c.moi)

			_, err := Load("platform")
			if !errors.Is(err, ErrThieuBienMoiTruong) {
				t.Errorf("muốn ErrThieuBienMoiTruong, nhận %v", err)
			}
		})
	}
}

func TestLoadEnvKhongHopLe(t *testing.T) {
	datMoiTruong(t, map[string]string{"DATABASE_DSN": dsnGia, "ENV": "production"})

	if _, err := Load("platform"); !errors.Is(err, ErrEnvKhongHopLe) {
		t.Errorf("ENV=production phải bị từ chối, nhận %v", err)
	}
}

func TestCoNguyHiemBiChanOProd(t *testing.T) {
	// Rule 8, invariant 7: a dangerous flag is switched on for a local afternoon and nobody
	// remembers to switch it off. Refusing to start is the only response that cannot be ignored.
	datMoiTruong(t, map[string]string{
		"DATABASE_DSN":          dsnGia,
		"ENV":                   EnvProd,
		"SESSION_SIGNING_KEYS":  khoaGia,
		"DANGEROUS_AUTH_BYPASS": "true",
	})

	if _, err := Load("platform"); !errors.Is(err, ErrCoBienNguyHiem) {
		t.Fatalf("cờ nguy hiểm ở prod phải chặn khởi động, nhận %v", err)
	}
}

func TestCoNguyHiemChoPhepODev(t *testing.T) {
	datMoiTruong(t, map[string]string{
		"DATABASE_DSN":          dsnGia,
		"ENV":                   EnvDev,
		"DANGEROUS_AUTH_BYPASS": "true",
	})

	cfg, err := Load("platform")
	if err != nil {
		t.Fatalf("dev phải khởi động được: %v", err)
	}
	// Allowed, but never silently: it must be reported at every startup.
	canhBao := cfg.CanhBao()
	if len(canhBao) == 0 {
		t.Error("cờ nguy hiểm đang bật mà không có cảnh báo nào")
	}
}

func TestRedactedGiauMatKhau(t *testing.T) {
	// Logging a DSN sends one credential into centralised logging, backups and third-party
	// monitoring at once, and it cannot be recalled from any of them.
	datMoiTruong(t, map[string]string{"DATABASE_DSN": dsnGia, "ENV": EnvDev})

	cfg, err := Load("platform")
	if err != nil {
		t.Fatal(err)
	}
	// .String() is the redaction itself now — the type refuses to render the password on every
	// path, and Redacted() only freezes that into the value.
	an := cfg.Redacted().DatabaseDSN.String()

	if strings.Contains(an, "khong-phai-mat-khau-that") {
		t.Errorf("mật khẩu lọt ra sau khi che: %q", an)
	}
	// The host must survive, or a misconfigured target is unrecognisable in a log.
	if !strings.Contains(an, "localhost:5432") {
		t.Errorf("che quá tay, mất thông tin chẩn đoán: %q", an)
	}
	if !strings.Contains(an, "vigov:") {
		t.Errorf("tên người dùng phải giữ lại: %q", an)
	}
}

func TestRedactedKhiKhongCoThongTinDangNhap(t *testing.T) {
	cases := map[string]string{
		"không có thông tin đăng nhập": "postgres://localhost:5432/vigov",
		"không phải URL":               "day-khong-phai-dsn",
	}
	for ten, dsn := range cases {
		t.Run(ten, func(t *testing.T) {
			c := Config{DatabaseDSN: secret.DSN(dsn)}
			got := c.Redacted().DatabaseDSN.String()
			if strings.Contains(got, "khong-phai-mat-khau-that") {
				t.Errorf("rò rỉ: %q", got)
			}
		})
	}
}

func TestRedisDSNTuyChon(t *testing.T) {
	// Empty is a valid deployment: local development with no cache. A service must not fail to
	// start because a cache is absent — the route's own idem declaration decides what happens.
	datMoiTruong(t, map[string]string{
		"DATABASE_DSN":         dsnGia,
		"ENV":                  EnvDev,
		"SESSION_SIGNING_KEYS": khoaGia,
	})
	t.Setenv("REDIS_DSN", "")

	cfg, err := Load("petitions")
	if err != nil {
		t.Fatalf("REDIS_DSN trống phải khởi động được: %v", err)
	}
	if cfg.RedisDSN != "" {
		t.Errorf("RedisDSN = %q, muốn rỗng", cfg.RedisDSN)
	}
	if len(cfg.CanhBao()) != 0 {
		t.Errorf("ở dev, thiếu Redis chưa cần cảnh báo: %v", cfg.CanhBao())
	}

	// Outside dev it must never be silent: no Redis means no duplicate protection, and a
	// duplicated petition cannot be deleted afterwards (rule 7).
	datMoiTruong(t, map[string]string{
		"DATABASE_DSN":         dsnGia,
		"ENV":                  EnvStaging,
		"SESSION_SIGNING_KEYS": khoaGia,
	})
	cfg, err = Load("petitions")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.CanhBao()) == 0 {
		t.Error("thiếu REDIS_DSN ngoài dev phải được cảnh báo")
	}
}

func TestRedisDSNCungBiCheMatKhau(t *testing.T) {
	// Every DSN in Config carries a credential. One of them logged is one credential in
	// centralised logging, backups and third-party monitoring at once (rule 8).
	datMoiTruong(t, map[string]string{
		"DATABASE_DSN": dsnGia,
		"ENV":          EnvDev,
		"REDIS_DSN":    redisGia,
	})

	cfg, err := Load("petitions")
	if err != nil {
		t.Fatal(err)
	}
	an := cfg.Redacted().RedisDSN.String()
	if strings.Contains(an, "khong-phai-mat-khau-that") {
		t.Errorf("mật khẩu Redis lọt ra sau khi che: %q", an)
	}
	if !strings.Contains(an, "localhost:6379") {
		t.Errorf("che quá tay, mất thông tin chẩn đoán: %q", an)
	}
}

func TestKhoaKyThieuThiChanKhoiDongNgoaiDev(t *testing.T) {
	// Fail closed. A service with no signing key cannot tell a real token from a forged one,
	// and one forged token is a staff account in somebody else's commune (rule 1, invariant 8).
	for _, env := range []string{EnvStaging, EnvProd} {
		t.Run(env, func(t *testing.T) {
			datMoiTruong(t, map[string]string{"DATABASE_DSN": dsnGia, "ENV": env})
			t.Setenv("SESSION_SIGNING_KEYS", "")

			_, err := Load("identity")
			if !errors.Is(err, ErrThieuBienMoiTruong) {
				t.Fatalf("muốn ErrThieuBienMoiTruong, nhận %v", err)
			}
			if !strings.Contains(err.Error(), "SESSION_SIGNING_KEYS") {
				t.Errorf("thông báo phải nói rõ thiếu biến nào: %v", err)
			}
		})
	}
}

func TestKhoaKyThieuODevThiChayNhungCoCanhBao(t *testing.T) {
	datMoiTruong(t, map[string]string{"DATABASE_DSN": dsnGia, "ENV": EnvDev})
	t.Setenv("SESSION_SIGNING_KEYS", "")

	cfg, err := Load("identity")
	if err != nil {
		t.Fatalf("dev phải khởi động được: %v", err)
	}
	if len(cfg.CanhBao()) == 0 {
		t.Error("không có khoá ký mà không cảnh báo — đăng nhập sẽ hỏng mà không ai biết vì sao")
	}
}

func TestKhoaKyDocTheoThuTu(t *testing.T) {
	// Order is the whole rotation procedure: the FIRST key signs, every key verifies. Reordering
	// silently would sign with a key being retired.
	datMoiTruong(t, map[string]string{
		"DATABASE_DSN":         dsnGia,
		"ENV":                  EnvProd,
		"SESSION_SIGNING_KEYS": khoaGia + "-moi , " + khoaGia + "-cu ,,",
	})

	cfg, err := Load("identity")
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.KhoaKyBytes()
	if len(got) != 2 {
		t.Fatalf("đọc được %d khoá, muốn 2 (bỏ phần tử rỗng)", len(got))
	}
	if string(got[0]) != khoaGia+"-moi" || string(got[1]) != khoaGia+"-cu" {
		t.Errorf("sai thứ tự hoặc còn khoảng trắng: %q", got)
	}
	// One key in prod is legal but must be said out loud: rotating it signs everybody out.
	datMoiTruong(t, map[string]string{"SESSION_SIGNING_KEYS": khoaGia})
	cfg, err = Load("identity")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.CanhBao()) == 0 {
		t.Error("một khoá duy nhất ở prod phải được cảnh báo")
	}
}

func TestKhoaKyKhongTuHienRaKhiGhiLog(t *testing.T) {
	// A leaked signing key cannot be recalled from centralised logging, backups or third-party
	// monitoring, and it affects EVERY commune at once. Redacted() only protects the call sites
	// that remember it; the type has to protect the ones that do not.
	datMoiTruong(t, map[string]string{
		"DATABASE_DSN":         dsnGia,
		"ENV":                  EnvProd,
		"SESSION_SIGNING_KEYS": khoaGia,
	})

	cfg, err := Load("identity")
	if err != nil {
		t.Fatal(err)
	}

	tho, err := json.Marshal(cfg.SessionSigningKeys)
	if err != nil {
		t.Fatal(err)
	}
	renders := []string{
		fmt.Sprintf("%v", cfg.SessionSigningKeys),
		fmt.Sprintf("%s", cfg.SessionSigningKeys),
		fmt.Sprintf("%+v", cfg),
		fmt.Sprintf("%#v", cfg.SessionSigningKeys),
		fmt.Sprintf("%v", cfg.Redacted()),
		string(tho),
	}
	for _, r := range renders {
		if strings.Contains(r, khoaGia) {
			t.Errorf("khoá ký lọt ra khi in: %q", r)
		}
	}

	// ...but the key itself must still be reachable for pkg/token.
	if string(cfg.KhoaKyBytes()[0]) != khoaGia {
		t.Error("KhoaKyBytes không trả về khoá thật")
	}
}

func TestTtlMacDinhKhiSai(t *testing.T) {
	// A malformed TTL must not become zero: zero disables the cache, and the directory lookup
	// runs on every request of every commune.
	datMoiTruong(t, map[string]string{
		"DATABASE_DSN":     dsnGia,
		"ENV":              EnvDev,
		"TENANT_CACHE_TTL": "khong-phai-thoi-gian",
	})

	cfg, err := Load("platform")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TenantCacheTTL != 30*time.Second {
		t.Errorf("TTL sai định dạng phải về mặc định 30s, nhận %v", cfg.TenantCacheTTL)
	}
}
