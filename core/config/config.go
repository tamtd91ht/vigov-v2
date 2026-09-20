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

	"github.com/vihat/vigov/core/secret"
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
	//
	// THE TYPE IS WHAT KEEPS THE PASSWORD OUT OF THE LOGS. As a plain string, one
	// `slog.Info("boot", "cfg", cfg)` written while debugging put the database password of the
	// whole deployment into centralised logging, backups and third-party monitoring — from
	// where it cannot be recalled (rule 8, invariants 1 and 3), while Redacted() below only
	// ever protected the call sites that remembered to call it. secret.DSN redacts on EVERY
	// rendering path and keeps the host readable, so the startup line still says which
	// database this process opened. sql.Open takes .Lo().
	DatabaseDSN secret.DSN

	// RedisDSN is the cache used for duplicate-request protection and rate limiting.
	//
	// It is a PLATFORM-WIDE constant, not a per-commune value: one Redis serves every commune
	// on this deployment, and the keys carry the commune in their prefix (rule 1, invariant 7).
	//
	// Empty is allowed, and only means local development with no cache: every route then
	// behaves per the CheDoHong it declared — idem.MoKhiHong passes, idem.DongKhiHong answers
	// 503. A service must not fail to start because a cache is absent.
	//
	// Same type, same reason as DatabaseDSN: it carries a credential.
	RedisDSN secret.DSN

	// PlatformGRPCAddr is where the platform service answers ResolveHost — the RPC that maps an
	// incoming Host to a commune (proto/vigov/platform/v1/platform.proto).
	//
	// IT HAS NO DEFAULT, deliberately. Every service edge resolves its commune through this
	// address, so guessing it wrong means either refusing every request or — far worse —
	// resolving communes against something that is not the registry. Rule 1, forbidden #1 is
	// about defaults on the isolation path, and the address of the registry is on it.
	//
	// Empty is allowed HERE because the platform service itself does not call anybody: it holds
	// the registry. Every other service refuses to start without it, and says so by name.
	PlatformGRPCAddr string

	// GRPCCallerKey authenticates the CALLER on the inter-service gRPC port. ONE key, shared by
	// every service on the deployment, read from GRPC_CALLER_KEY and sourced from a k8s secret.
	//
	// IT IS HALF OF A TWO-LAYER DEFENCE, and reading it as the whole one overestimates it
	// badly: the other half is the gRPC port being confined to the cluster's internal network.
	// What the shape does and does not buy — no caller attribution, no blast-radius limit
	// inside the cluster, no graceful rotation — is written out in core/grpcx's package doc and
	// recorded in ADR 0025. Read it before reasoning about this value.
	//
	// IT IS A PLATFORM-WIDE CONSTANT, not a per-commune value: it says nothing about which
	// commune a call is for (rule 8, invariant 5). The commune travels separately, in metadata.
	//
	// REQUIRED, IN EVERY ENVIRONMENT, WITH NO DEV EXEMPTION (ADR 0025, invariant 3). Not the
	// shape of PlatformGRPCAddr above, which is allowed to be empty here: an address that is
	// missing produces a service that answers nothing, while a KEY that is missing would
	// produce a server that answers EVERYTHING. "Runs without the key" is the one configuration
	// that must not be reachable, so it is refused at the earliest point that can name the
	// variable — Load — rather than at a stack trace further in.
	//
	// The interceptors refuse an empty key again, at construction. That is not a second answer
	// to the same question: this function answers "is the deployment configured", and the
	// interceptor answers "is this server wired", and a direct caller of the interceptor never
	// passes through here at all.
	//
	// Same type, same reason as the signing keys: the bytes ARE the credential, so the type has
	// to refuse to render them (rule 8, invariant 1).
	GRPCCallerKey secret.Secret

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
// IT IS AN ALIAS, NOT A SECOND TYPE. This package used to carry its own implementation of
// "refuses to render", and so did pkg/token, and the DSNs carried none at all — three
// answers to one question, which is how the fourth one gets forgotten. There is now exactly
// one implementation, in pkg/secret, and this name only says what the material is FOR.
//
// An alias rather than a defined type on purpose: `[]Khoa` and `[]secret.Secret` have to be
// the same type, or every hand-over to pkg/token would need a conversion loop, and a
// conversion loop is a place to write `[]byte(k)` by accident.
//
// The raw material comes out through secret.Secret.Lo(), and nowhere else.
type Khoa = secret.Secret

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

	// Required everywhere, dev included. A missing database DSN produces a service that cannot
	// answer; a missing caller key would produce a gRPC port that answers ANYTHING that reaches
	// it, silently and on every RPC. There is no environment in which that is a tolerable
	// default (ADR 0025, invariant 3).
	khoaGoi := strings.TrimSpace(os.Getenv("GRPC_CALLER_KEY"))
	if khoaGoi == "" {
		thieu = append(thieu, "GRPC_CALLER_KEY")
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
		ListenAddr:       firstNonEmpty(os.Getenv("LISTEN_ADDR"), ":8080"),
		GRPCListenAddr:   firstNonEmpty(os.Getenv("GRPC_LISTEN_ADDR"), ":9090"),
		DatabaseDSN:      secret.DSN(dsn),
		RedisDSN:         secret.DSN(strings.TrimSpace(os.Getenv("REDIS_DSN"))),
		PlatformGRPCAddr: strings.TrimSpace(os.Getenv("PLATFORM_GRPC_ADDR")),
		// Trimmed above: a trailing newline pasted out of a k8s secret would make the key
		// compare unequal at the far end, and the refusal it produces says "unauthenticated" —
		// which sends the reader looking for a missing variable rather than for an invisible
		// character.
		GRPCCallerKey:       secret.Secret(khoaGoi),
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

// Redacted returns a config whose DSN fields hold the redacted text itself, not merely a type
// that redacts when printed.
//
// IT IS NO LONGER THE PROTECTION, and that is the point of keeping it. Every field that can
// carry a credential now refuses to render on its own (secret.Secret, secret.DSN), so a config
// logged WITHOUT calling this function is already safe — which is the case Redacted() could
// never cover, because it depended on somebody remembering. What this still buys is a value
// that cannot leak even through .Lo(): hand THIS to anything that has to keep a copy.
func (c Config) Redacted() Config {
	c.DatabaseDSN = secret.DSN(c.DatabaseDSN.String())
	c.RedisDSN = secret.DSN(c.RedisDSN.String())
	return c
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
//
// IT RETURNS THE PROTECTED TYPE, NOT [][]byte. Returning bare byte slices ended the protection
// at this function's boundary: `log.Info("khoá", "k", cfg.KhoaKyBytes())` printed the whole
// platform's signing keys as numbers, and nothing on the way there was a Khoa any more. The
// keys become raw bytes at exactly one place now — inside token.NewSigner, through Lo().
func (c Config) KhoaKyBytes() []secret.Secret {
	ra := make([]secret.Secret, 0, len(c.SessionSigningKeys))
	ra = append(ra, c.SessionSigningKeys...)
	return ra
}

func boolean(s string) bool {
	b, err := strconv.ParseBool(strings.TrimSpace(s))
	return err == nil && b
}
