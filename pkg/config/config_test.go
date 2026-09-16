package config

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// dsnGia is a fake DSN. Never a real credential in source (rule 8, forbidden #1).
const dsnGia = "postgres://vigov:khong-phai-mat-khau-that@localhost:5432/vigov_test"

// redisGia is the same fake credential on the cache DSN.
const redisGia = "redis://vigov:khong-phai-mat-khau-that@localhost:6379/0"

func datMoiTruong(t *testing.T, cap map[string]string) {
	t.Helper()
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
	an := cfg.Redacted().DatabaseDSN

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
			c := Config{DatabaseDSN: dsn}
			got := c.Redacted().DatabaseDSN
			if strings.Contains(got, "khong-phai-mat-khau-that") {
				t.Errorf("rò rỉ: %q", got)
			}
		})
	}
}

func TestRedisDSNTuyChon(t *testing.T) {
	// Empty is a valid deployment: local development with no cache. A service must not fail to
	// start because a cache is absent — the route's own idem declaration decides what happens.
	datMoiTruong(t, map[string]string{"DATABASE_DSN": dsnGia, "ENV": EnvDev})
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
	datMoiTruong(t, map[string]string{"DATABASE_DSN": dsnGia, "ENV": EnvStaging})
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
	an := cfg.Redacted().RedisDSN
	if strings.Contains(an, "khong-phai-mat-khau-that") {
		t.Errorf("mật khẩu Redis lọt ra sau khi che: %q", an)
	}
	if !strings.Contains(an, "localhost:6379") {
		t.Errorf("che quá tay, mất thông tin chẩn đoán: %q", an)
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
