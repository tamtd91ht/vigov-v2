// Package config reads PLATFORM-WIDE constants from the environment.
//
// THE ONE DISTINCTION THIS PACKAGE EXISTS TO ENFORCE:
//
//	Platform-wide constant  -> environment variable  -> this package
//	Per-commune value       -> database, read at RUNTIME -> NOT this package
//
// A process serves N communes at once. An environment variable is a constant of the PROCESS,
// so the moment a commune-specific value is read from one, every commune gets whichever
// commune was deployed last. That failure is silent: the service starts, requests succeed,
// and one commune's name, SLA or hotline appears under another commune's domain.
//
// Rule 1, invariant 10. There is deliberately no Get("...") helper here that business code
// could reach for — the absence of that function is the enforcement.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds what is genuinely the same for every commune on this deployment.
type Config struct {
	// ListenAddr is the address this process serves on.
	ListenAddr string

	// DatabaseDSN is the connection string for THIS service's own schema. A service never
	// holds a DSN for another service's database — that is rule 2, and a second DSN appearing
	// in this struct is the first symptom of a distributed monolith.
	DatabaseDSN string

	// TenantCacheTTL bounds how long a deactivated commune keeps being served, and how long a
	// reassigned domain keeps resolving to the old commune (ADR 0004, decision 5).
	TenantCacheTTL time.Duration

	// Env is "dev" | "staging" | "prod". It decides nothing about business behaviour; it is
	// used for log verbosity and for refusing dangerous flags in production.
	Env string

	// DangerousAuthBypass disables authentication. It exists for local development only and
	// is reported at every startup (rule 8, invariant 7).
	DangerousAuthBypass bool
}

const (
	EnvDev     = "dev"
	EnvStaging = "staging"
	EnvProd    = "prod"
)

var (
	ErrThieuBienMoiTruong = errors.New("config: thiếu biến môi trường bắt buộc")
	ErrCoBienNguyHiem     = errors.New("config: cờ nguy hiểm đang bật trong môi trường thật")
	ErrEnvKhongHopLe      = errors.New("config: ENV phải là dev, staging hoặc prod")
)

// Load reads the configuration for one service.
//
// It FAILS instead of defaulting. A service that starts with a guessed database or a guessed
// listen address is a service that will be debugged at the wrong layer — and in this system a
// wrong guess on the isolation path is a data breach, not an inconvenience.
func Load(serviceName string) (Config, error) {
	var thieu []string

	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		thieu = append(thieu, "DATABASE_DSN")
	}

	env := strings.ToLower(strings.TrimSpace(os.Getenv("ENV")))
	if env == "" {
		thieu = append(thieu, "ENV")
	}

	if len(thieu) > 0 {
		return Config{}, fmt.Errorf("%w: %s (service %s)",
			ErrThieuBienMoiTruong, strings.Join(thieu, ", "), serviceName)
	}

	switch env {
	case EnvDev, EnvStaging, EnvProd:
	default:
		return Config{}, fmt.Errorf("%w: %q", ErrEnvKhongHopLe, env)
	}

	cfg := Config{
		ListenAddr:          firstNonEmpty(os.Getenv("LISTEN_ADDR"), ":8080"),
		DatabaseDSN:         dsn,
		TenantCacheTTL:      duration(os.Getenv("TENANT_CACHE_TTL"), 30*time.Second),
		Env:                 env,
		DangerousAuthBypass: boolean(os.Getenv("DANGEROUS_AUTH_BYPASS")),
	}

	// A dangerous flag left on in production is the failure mode rule 8 invariant 7 exists
	// for: it is switched on for a local afternoon and nobody remembers to switch it off.
	// Refusing to start is the only response that cannot be ignored.
	if cfg.Env == EnvProd && cfg.DangerousAuthBypass {
		return Config{}, fmt.Errorf("%w: DANGEROUS_AUTH_BYPASS", ErrCoBienNguyHiem)
	}

	return cfg, nil
}

// CanhBao lists the dangerous settings currently in force, for logging at startup.
// Empty in a correctly configured deployment.
func (c Config) CanhBao() []string {
	var ra []string
	if c.DangerousAuthBypass {
		ra = append(ra, "DANGEROUS_AUTH_BYPASS đang BẬT — mọi kiểm tra xác thực bị bỏ qua")
	}
	if c.Env != EnvProd && c.TenantCacheTTL > time.Minute {
		ra = append(ra, "TENANT_CACHE_TTL dài hơn 1 phút — thay đổi tên miền sẽ chậm có hiệu lực")
	}
	return ra
}

// Redacted returns the config with the DSN's credentials removed, so it can be logged.
//
// The DSN carries a password. Logging it sends one credential into centralised logging,
// backups and third-party monitoring at once, and it cannot be recalled from any of them
// (rule 8).
func (c Config) Redacted() Config {
	c.DatabaseDSN = redactDSN(c.DatabaseDSN)
	return c
}

// redactDSN replaces the password inside a driver URL with three asterisks, keeping the
// scheme, the user and the host so a misconfigured target is still recognisable in a log.
func redactDSN(dsn string) string {
	i := strings.Index(dsn, "://")
	if i < 0 {
		return "***"
	}
	rest := dsn[i+3:]
	at := strings.LastIndex(rest, "@")
	if at < 0 {
		return dsn // no credentials present
	}
	cred := rest[:at]
	if colon := strings.Index(cred, ":"); colon >= 0 {
		cred = cred[:colon] + ":***"
	}
	return dsn[:i+3] + cred + rest[at:]
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func duration(s string, mac time.Duration) time.Duration {
	if s == "" {
		return mac
	}
	d, err := time.ParseDuration(s)
	if err != nil || d < 0 {
		return mac
	}
	return d
}

func boolean(s string) bool {
	b, err := strconv.ParseBool(strings.TrimSpace(s))
	return err == nil && b
}
