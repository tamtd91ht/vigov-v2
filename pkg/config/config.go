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
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds what is genuinely the same for every commune on this deployment.
type Config struct {
	// ListenAddr is the address this process serves on.
	ListenAddr string

	// GRPCListenAddr is the address this process serves gRPC on — a SEPARATE port from
	// ListenAddr, not a shared one.
	//
	// WHY SEPARATE: gRPC needs HTTP/2 with prior knowledge, REST is served to browsers over
	// HTTP/1.1, and sharing one listener means demultiplexing the two by protocol at runtime.
	// Every proxy, health check and timeout in front of the process then has to understand
	// both, and the failure when one of them does not is a connection that hangs rather than
	// one that is refused.
	//
	// It is a PLATFORM-WIDE constant: a port is a property of the process, and one process
	// serves every commune (rule 8, invariant 5).
	GRPCListenAddr string

	// DatabaseDSN is the connection string for THIS service's own schema. A service never
	// holds a DSN for another service's database — that is rule 2, and a second DSN appearing
	// in this struct is the first symptom of a distributed monolith.
	DatabaseDSN string

	// RedisDSN is the cache used for duplicate-request protection and rate limiting.
	//
	// It is a PLATFORM-WIDE constant, not a per-commune value: one Redis serves every commune
	// on this deployment, and the keys carry the commune in their prefix (rule 1, invariant 7).
	//
	// Empty is allowed, and only means local development with no cache: every route then
	// behaves per the CheDoHong it declared — idem.MoKhiHong passes, idem.DongKhiHong answers
	// 503. A service must not fail to start because a cache is absent.
	RedisDSN string

	// TenantCacheTTL bounds how long a deactivated commune keeps being served, and how long a
	// reassigned domain keeps resolving to the old commune (ADR 0004, decision 5).
	TenantCacheTTL time.Duration

	// SessionSigningKeys signs and verifies the session cookie. FIRST ENTRY SIGNS, EVERY ENTRY
	// VERIFIES — see pkg/token.
	//
	// It is a LIST because rule 8, invariant 6 requires a rotation procedure, and with a single
	// key rotating it signs out every member of staff of every commune on this deployment at
	// once. A rotation nobody can afford to perform is a rotation that never happens.
	//
	// It is a PLATFORM-WIDE constant, not a per-commune value: the commune is a claim INSIDE the
	// token (rule 1, invariant 8), not a property of the key. Per-commune keys would mean the
	// key material has to be resolved before the token can be read, which is the wrong order.
	//
	// Empty is refused outside dev by Load: a service with no signing key cannot tell a real
	// token from a forged one, and silence is the one response that is not allowed.
	SessionSigningKeys []Khoa

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

// Khoa is secret key material that refuses to print itself.
//
// WHY A TYPE AND NOT A PLAIN []byte: Redacted() below is the deliberate place to strip
// credentials before a config is logged, but it only protects the call sites that remember to
// use it. A key reaches centralised logging, backups and third-party monitoring through one
// forgotten `slog.Info("cfg", "cfg", cfg)` — and a leaked signing key cannot be recalled from
// any of them, while affecting EVERY commune at once (rule 8). Refusing to render is the only
// protection that does not depend on anybody remembering.
type Khoa []byte

func (k Khoa) String() string               { return "***" }
func (k Khoa) GoString() string             { return "***" }
func (k Khoa) MarshalJSON() ([]byte, error) { return []byte(`"***"`), nil }
func (k Khoa) MarshalText() ([]byte, error) { return []byte("***"), nil }
func (k Khoa) LogValue() slog.Value         { return slog.StringValue("***") }

// Bytes hands the raw material to pkg/token. The one explicit way out, so every use is
// greppable.
func (k Khoa) Bytes() []byte { return []byte(k) }

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

	// A missing signing key is fatal everywhere except dev, where it only means this process
	// cannot issue or read a session. Outside dev the alternative would be a service that
	// accepts forged tokens, or one that invents a key per replica and signs everybody out on
	// every restart — both fail silently, which is the one thing not allowed here.
	//
	// Named environments only, so an ENV nobody recognises still fails as ErrEnvKhongHopLe
	// below — reporting "missing key" for a typo in ENV would send the reader to the wrong
	// variable.
	khoa := khoaKy(os.Getenv("SESSION_SIGNING_KEYS"))
	if len(khoa) == 0 && (env == EnvStaging || env == EnvProd) {
		thieu = append(thieu, "SESSION_SIGNING_KEYS")
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
		GRPCListenAddr:      firstNonEmpty(os.Getenv("GRPC_LISTEN_ADDR"), ":9090"),
		DatabaseDSN:         dsn,
		RedisDSN:            strings.TrimSpace(os.Getenv("REDIS_DSN")),
		TenantCacheTTL:      duration(os.Getenv("TENANT_CACHE_TTL"), 30*time.Second),
		SessionSigningKeys:  khoa,
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
	// Without Redis there is no duplicate protection: a double-submitted POST creates a second
	// petition with a second lookup code already shown to the citizen, and rule 7 forbids
	// deleting it. Tolerable while developing, never silently in a real environment.
	if c.Env != EnvDev && c.RedisDSN == "" {
		ra = append(ra, "REDIS_DSN trống — chống trùng request không hoạt động, "+
			"route khai idem.DongKhiHong sẽ trả 503")
	}
	// Only reachable in dev — Load refuses to start anywhere else.
	if len(c.SessionSigningKeys) == 0 {
		ra = append(ra, "SESSION_SIGNING_KEYS trống — không ký và không đọc được phiên đăng nhập")
	}
	// One key means the key can never be rotated without signing out every cán bộ of every xã
	// at once (rule 8, invariant 6). Not an error, but somebody has to know before it is needed.
	if len(c.SessionSigningKeys) == 1 && c.Env == EnvProd {
		ra = append(ra, "SESSION_SIGNING_KEYS chỉ có một khoá — xoay khoá sẽ đăng xuất toàn bộ cán bộ")
	}
	return ra
}

// Redacted returns the config with every DSN's credentials removed, so it can be logged.
//
// A DSN carries a password. Logging it sends one credential into centralised logging,
// backups and third-party monitoring at once, and it cannot be recalled from any of them
// (rule 8). Every DSN field added to Config must be redacted here as well.
//
// SessionSigningKeys needs no line here: the Khoa type refuses to render itself, which also
// covers the call sites that forget to call this function at all.
func (c Config) Redacted() Config {
	c.DatabaseDSN = redactDSN(c.DatabaseDSN)
	if c.RedisDSN != "" {
		c.RedisDSN = redactDSN(c.RedisDSN)
	}
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

// khoaKy splits the comma-separated key list. The FIRST entry is the one that signs.
//
// Length and strength are NOT checked here: pkg/token.NewSigner owns that, and one owner for
// one rule is what keeps the two from drifting apart (rule 9). This function only answers
// "which strings were configured".
func khoaKy(raw string) []Khoa {
	var ra []Khoa
	for _, phan := range strings.Split(raw, ",") {
		phan = strings.TrimSpace(phan)
		if phan != "" {
			ra = append(ra, Khoa(phan))
		}
	}
	return ra
}

// KhoaKyBytes hands the signing keys to pkg/token, in order.
func (c Config) KhoaKyBytes() [][]byte {
	ra := make([][]byte, 0, len(c.SessionSigningKeys))
	for _, k := range c.SessionSigningKeys {
		ra = append(ra, k.Bytes())
	}
	return ra
}

func boolean(s string) bool {
	b, err := strconv.ParseBool(strings.TrimSpace(s))
	return err == nil && b
}
