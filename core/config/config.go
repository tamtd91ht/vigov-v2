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
//
// # WHERE EACH VARIABLE COMES FROM IN THE CLUSTER
//
// Every infrastructure dependency is read HERE and nowhere else: no host, no port and no
// credential is read from another package, and none is hard-coded. The values arrive as
// environment variables, and the cluster fills them from one of exactly two objects. Which one
// is not a deployment detail — it is the difference between a value that may sit in a git-
// tracked manifest and one that may not (rule 8, invariants 1 and 2):
//
//	k8s Secret     anything credentialed: a password, a token, an API key, or a DSN with a
//	               password inside it. The Go type is secret.Secret or secret.DSN, so it cannot
//	               reach a log line by accident
//	k8s ConfigMap  anything not credentialed: a host, a port, an address list, an exchange
//	               name, an index prefix, a database number, a TTL. Plain Go types
//
//	Variable                    Object      Refusal
//	------------------------------------------------------------------------------------
//	DATABASE_DSN                Secret      REQUIRED — Load refuses, by name
//	GRPC_CALLER_KEY             Secret      REQUIRED — Load refuses, by name (ADR 0025)
//	SESSION_SIGNING_KEYS        Secret      REQUIRED outside dev — Load refuses, by name
//	ENV                         ConfigMap   REQUIRED — Load refuses, by name
//	REDIS_DSN                   Secret      optional — empty means no cache; CanhBao reports it
//	PLATFORM_GRPC_ADDR          ConfigMap   optional here, refused by name at Dial
//	IDENTITY_GRPC_ADDR          ConfigMap   optional here, refused by name at Dial
//	LISTEN_ADDR                 ConfigMap   optional — default :8080
//	GRPC_LISTEN_ADDR            ConfigMap   optional — default :9090
//	TENANT_CACHE_TTL            —           optional — default 30s, not set in the cluster
//	RABBITMQ_DSN                Secret      optional — refused by name at connect time
//	RABBITMQ_EXCHANGE           ConfigMap   optional — refused by name at connect time
//	ELASTICSEARCH_ADDRS         ConfigMap   optional — refused by name at connect time
//	ELASTICSEARCH_API_KEY       Secret      optional — refused by name at connect time
//	ELASTICSEARCH_INDEX_PREFIX  ConfigMap   optional — refused by name at connect time
//	DANGEROUS_AUTH_BYPASS       ConfigMap   optional — refused outright when ENV=prod
//	TRUSTED_PROXY_CIDRS         ConfigMap   optional — empty trusts nobody; malformed refused by Load
//	CITIZEN_SESSION_BRIDGE_LISTEN_ADDR  ConfigMap  optional — both bridge vars or neither; one alone refused by Load
//	CITIZEN_SESSION_BRIDGE_KEYS         Secret     optional — both bridge vars or neither; one alone refused by Load
//	CITIZEN_SESSION_TTL                 ConfigMap  optional — default 720h (30 days); malformed refused by Load
//	IDENTITY_ADMIN_SEED_PASSWORD        Secret     optional — empty = off; shorter than password.DaiToiThieu = off, reported by identity at startup
//
// THE TABLE IS HERE AND NOT IN A MANIFEST because the manifests are not in this repository's
// gift and a classification that lives only in deploy/ is one nobody reading the config layer
// can check. What deploy/ decides is which object holds the value; what this file decides is
// which object is ALLOWED to.
//
// WHY POSTGRESQL AND REDIS HAVE ONE VARIABLE EACH AND NOT FIVE: a DSN already carries the host,
// the port, the user, the password and the Redis database number, and every consumer in this
// repository takes a DSN (sql.Open, redis.ParseURL). Splitting it into PG_HOST + PG_PORT +
// PG_USER + PG_PASSWORD + REDIS_DB would create a second source for facts the DSN already
// states, and two sources for one fact drift (rule 9, invariant 2) — here the drift is a
// service pointed at one database while the startup line names another.
package config

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/vihat/vigov/core/secret"
)

// Config holds what is genuinely the same for every commune on this deployment.
type Config struct {
	// ListenAddr is the address this process serves on: LISTEN_ADDR, or ":8080" when unset.
	//
	// ONE DEFAULT FOR EVERY SERVICE, decided by the owner on 2026-09-25 — and it reverses
	// commit 0645284, which gave each service its own default (:8083, :8084, :8087, …) through
	// a ListenAddrHoac(default) helper. That rationale was "distinct ports matter when several
	// services share one developer machine"; it made the cluster pay for a laptop convenience.
	// Under k8s every pod has its own IP, and the Service targetPort, the httpGet probes and the
	// NetworkPolicy ingress rule all assume 8080. A per-service default meant that deleting one
	// "redundant-looking" LISTEN_ADDR line from a manifest silently broke both the probe and the
	// NetworkPolicy at once. With one default, the manifest line is optional and a pod that
	// omits it still listens where everything in front of it expects.
	//
	// Running several services on one machine: set LISTEN_ADDR per process (.env.example lists
	// suggested ports). Collisions there are loud (bind: address already in use) and cheap.
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

	// RabbitMQDSN is the broker for DELAYED background work — deadline reminders, escalation,
	// and the five automation jobs. ADR 0010 fixes what it is and is not for: background tasks
	// with a delay, NEVER inter-service events, which are Kafka's. Putting an inter-service
	// event on this connection is rule 2, invariant 3 broken, not a shortcut.
	//
	// k8s SECRET: an amqp URL carries the broker password. Same type and same reason as
	// DatabaseDSN — secret.DSN redacts the password on every rendering path while keeping the
	// host readable, so a startup line still says which broker this process opened.
	//
	// OPTIONAL, AND THAT IS THE WHOLE ARGUMENT OF THIS FIELD. Nothing in this repository
	// publishes or consumes a task yet. Making it required at Load would stop all eight services
	// from starting on any machine that has not set it — refusing to run working code in order
	// to protect code that does not exist. Required belongs to what is actually used:
	// DATABASE_DSN, because every service opens a database in its first thirty lines.
	//
	// IT IS THE IdentityGRPCAddr PRECEDENT, NOT A THIRD PATTERN: empty is allowed here, and the
	// first thing that genuinely needs a broker refuses BY NAME at connect time, so the operator
	// reads "thiếu RABBITMQ_DSN" rather than watching reminders silently never fire.
	RabbitMQDSN secret.DSN

	// RabbitMQExchange is the exchange the background-task publisher binds to.
	//
	// k8s CONFIGMAP: a name is not a credential. It belongs in a manifest a reviewer can read.
	//
	// NO DEFAULT, on purpose, and this one is not the usual "a default on the isolation path"
	// argument — it is that the exchange topology has not been decided. ADR 0010 chose RabbitMQ
	// and named what it is for; it did not name an exchange, a queue or a routing key. Writing
	// "vigov.tasks" here would record that decision in a source file instead of in an ADR, and
	// the next person would read it as settled. Empty until whoever wires the first job names it.
	RabbitMQExchange string

	// ElasticsearchAddrs are the search cluster nodes, comma-separated in the environment.
	//
	// k8s CONFIGMAP: hosts and ports, no credential.
	//
	// ADR 0010 CONSTRAINS THIS HARDER THAN THE OTHERS, and the constraint is a business one:
	// Elasticsearch serves FULL-SYSTEM SEARCH ONLY and must never produce a reported figure.
	// It is near-real-time — refresh is a second by default, longer during a reindex — and the
	// disbursement totals and overdue-petition counts go to leadership. A figure that is off is
	// an incident somebody answers for, not a display bug. Reporting figures come from the
	// PostgreSQL read model.
	//
	// OPTIONAL, and more clearly so than RabbitMQ: ADR 0010 itself says to consider DEFERRING
	// Elastic, on the grounds that pg_trgm + unaccent is probably enough at ~10,000 records per
	// commune, and that it should be added when search is measured slow rather than assumed so.
	// A deployment with this empty is the deployment ADR 0010 expects today.
	ElasticsearchAddrs []string

	// ElasticsearchAPIKey authenticates to the search cluster.
	//
	// k8s SECRET: the bytes ARE the credential, so the type has to refuse to render them — same
	// reason as GRPCCallerKey. An API key differs from a DSN in that there is no host inside it
	// worth keeping readable, so it collapses to *** on every path.
	//
	// OPTIONAL, like the addresses. A cluster reachable without authentication is a legitimate
	// choice inside a closed network and a bad one for an index holding petition contents
	// (rule 3), so it is not refused here — it is REPORTED at every startup outside dev by
	// CanhBao, which is where a choice somebody has to own belongs.
	ElasticsearchAPIKey secret.Secret

	// ElasticsearchIndexPrefix namespaces this deployment's indices.
	//
	// k8s CONFIGMAP: a name, not a credential.
	//
	// IT IS THE DEPLOYMENT'S PREFIX AND NOTHING MORE. The per-commune part of an index name is
	// NOT decided here: rule 1, invariant 7 requires every index, room, cache key and queue to
	// carry t:<tenant_id>, and that belongs to whoever writes the indexer, where the tenant is
	// in the context. A prefix read from the environment cannot be per-commune anyway — one
	// process serves every commune, so an environment variable holding a commune's name gives
	// every commune whichever one was deployed last (rule 8, invariant 5).
	ElasticsearchIndexPrefix string

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

	// IdentityGRPCAddr is where the identity service answers ResolveStaffPrincipal — the RPC that
	// turns a staff member's session cookie into the principal every guard needs
	// (proto/vigov/identity/v1/identity.proto).
	//
	// EXACTLY THE SHAPE OF PlatformGRPCAddr ABOVE, AND FOR THE SAME REASON: no default, empty
	// allowed here, refused by name at Dial. A guessed address would not fail closed in an obvious
	// way — it would make every staff request answer 503 while identity was healthy, which reads as
	// "identity is down" and sends somebody to inspect the wrong service for an afternoon.
	//
	// Empty is allowed HERE because two services do not need it: identity itself builds its
	// principal from its own session registry (its XacThuc), and platform holds the registry and
	// calls nobody. Every service that guards a staff route refuses to start without it, and says
	// so by name — identityclient.Dial("").
	//
	// CLUSTER-INTERNAL ADDRESS ONLY. A WORKING SESSION TOKEN travels on this hop and there is no
	// TLS on it (ADR 0025); an address that leaves the cluster puts every staff session on the wire
	// in the clear.
	IdentityGRPCAddr string

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

	// TrustedProxies are the hops allowed to tell this process who the client was, through
	// X-Forwarded-For. Read from TRUSTED_PROXY_CIDRS: comma-separated CIDRs or bare IPs (a bare IP
	// is one host, /32 or /128). httpx.ClientIPTuProxyTinCay is the one consumer.
	//
	// k8s CONFIGMAP: an address range is not a credential. The value is the pod CIDR that
	// ingress-nginx and web-admin run in, and it belongs to whoever operates the cluster — this
	// repository cannot know it, so it has no default.
	//
	// OPTIONAL, AND EMPTY MEANS TRUST NOBODY — the behaviour before this field existed. A process
	// with no configured proxy records the address on its own socket, which behind web-admin is
	// web-admin's pod IP: wrong for an investigator, but never FORGED. That is the right failure
	// for an audit trail (rule 6, invariant 2), and it is why blank is tolerated where a
	// malformed value is not.
	//
	// MALFORMED IS FATAL, NOT SKIPPED. Skipping a typo'd entry silently trusts fewer hops than
	// the operator wrote, and a "fixed" parser that fell back to trusting everything would let any
	// client write its own address into a government record. Either way the trail looks normal and
	// is wrong, so Load refuses and names the variable and the entry.
	//
	// 0.0.0.0/0 AND ::/0 ARE REFUSED BY NAME: trusting the whole internet as a proxy is exactly
	// trusting a forged X-Forwarded-For from anyone, which is the thing the boundary exists to stop.
	TrustedProxies []netip.Prefix

	// CitizenSessionBridgeListenAddr is the address service-identity serves the citizen-session
	// bridge on (ADR 0045 §Tin cậy): a SECOND gRPC listener, serving CitizenSessionBridgeService and
	// nothing else, whose only caller is the vihat-miniapp backend.
	//
	// A SEPARATE PORT FROM GRPCListenAddr, AND THAT IS THE SECURITY PROPERTY, not tidiness. The
	// bridge caller is not a ViGov service and never holds GRPC_CALLER_KEY (that key opens every RPC
	// of every service, ADR 0025). "The bridge key opens exactly one RPC" holds because this
	// listener registers exactly one service — a property of the wiring, not a list somebody keeps.
	//
	// k8s CONFIGMAP: a port is not a credential. NO DEFAULT, on purpose: the port has to match the
	// NetworkPolicy rule that admits only the vihat-miniapp pods, and a default the code knows but
	// the manifest does not is exactly the drift that opened the 8080 incident (ListenAddr above).
	//
	// OPTIONAL, AND ONLY TOGETHER WITH THE KEYS. Without the bridge identity still serves every
	// staff request, so this is "cannot serve one flow", not "cannot serve one request" (rule 11,
	// invariant 8). Both empty = bridge not started. Exactly one set = Load refuses: an address with
	// no key is a port that would have to choose between answering nobody and answering everybody,
	// and keys with no address are credentials sitting in a process that never uses them — the
	// usual cause of both is a typo'd key in the manifest (ADR 0045 §Cấu hình).
	CitizenSessionBridgeListenAddr string

	// CitizenSessionBridgeKeys authenticate the bridge caller. ANY entry in the list is accepted;
	// vihat-miniapp sends ONE.
	//
	// A LIST, UNLIKE GRPC_CALLER_KEY, because the two ends deploy independently (ADR 0045 §Xoay
	// khoá): with a single value, rotating it would need both repositories released at the same
	// instant, and in the gap every citizen sign-in fails. Rotate by adding the new key here,
	// switching vihat-miniapp to it, then removing the old one. HOW OFTEN is not decided — ADR 0045
	// CÒN MỞ #5, and rule 8 invariant 6 still owes that number.
	//
	// k8s SECRET, secret.Secret: the bytes are the credential (rule 8). Strength is checked where
	// the key is USED — the bridge interceptor refuses a short key at construction — for the reason
	// khoaKy gives: one owner per rule.
	CitizenSessionBridgeKeys []secret.Secret

	// CitizenSessionTTL is how long a citizen session issued through the bridge lives.
	//
	// A PLATFORM-WIDE CONSTANT BY THE OWNER'S DECISION (ADR 0045, answer to CÒN MỞ #4): an
	// environment variable, default ONE MONTH. It is product policy, not a commune's value, so rule
	// 1 invariant 10 does not apply. Not the 7 days of vihat-miniapp's own session — that is that
	// repository's policy (ADR 0045 CÒN MỞ #4 records why the two are separate).
	//
	// OPTIONAL WITH THAT DEFAULT, because the owner stated the default; a deployment that says
	// nothing gets the decided number, not a guessed one. A MALFORMED OR NON-POSITIVE VALUE IS
	// REFUSED rather than falling back like TENANT_CACHE_TTL does: silently turning a typo into 30
	// days would hide that the operator meant something else for the lifetime of a credential.
	// Go duration syntax — "720h", not "30d".
	CitizenSessionTTL time.Duration

	// IdentityAdminSeedPassword is the password of the default administrator identity creates in
	// a commune at that commune's FIRST sign-in as `admin` (owner's decision, 2026-09-26; ledger
	// item `xa-moi-khong-co-vai-tro-va-quyen`). Read by identity only.
	//
	// OPTIONAL, AND EMPTY MEANS THE FEATURE IS OFF — rule 11 invariant 8. Every service serves
	// every request without it; identity merely stops seeding. Making it required would stop
	// every service on every machine that has not set it, for a bootstrap step that is meant to
	// be removed once every commune has changed its admin password.
	//
	// k8s SECRET (`bi-mat-identity`), secret.Secret: it is a live credential that opens the
	// highest-privileged account of every commune not yet seeded (rule 8, rule 11 invariant 7).
	// Its strength is judged where it is USED — identity refuses to seed with one shorter than
	// password.DaiToiThieu — for the same one-owner-per-rule reason CitizenSessionBridgeKeys gives.
	//
	// Trimmed: a trailing newline pasted into a Secret would otherwise make the typed password
	// never match, and the refusal would read as "wrong password" with nothing pointing here.
	IdentityAdminSeedPassword secret.Secret
}

// ThoiHanPhienCongDanMacDinh is CITIZEN_SESSION_TTL when the variable is unset — 30 days, the
// owner's decision of 2026-09-25 (ADR 0045, answer to CÒN MỞ #4).
const ThoiHanPhienCongDanMacDinh = 30 * 24 * time.Hour

// CauPhienBat reports whether the citizen-session bridge listener is configured. Load guarantees
// the address and the keys are either both present or both absent.
func (c Config) CauPhienBat() bool {
	return c.CitizenSessionBridgeListenAddr != "" && len(c.CitizenSessionBridgeKeys) > 0
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
	ErrProxyTinCayHong    = errors.New("config: TRUSTED_PROXY_CIDRS không hợp lệ")
	ErrCauPhienNuaVoi     = errors.New("config: cầu phiên công dân cấu hình nửa vời")
	ErrThoiHanPhienHong   = errors.New("config: CITIZEN_SESSION_TTL không hợp lệ")
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

	proxy, err := proxyTinCay(os.Getenv("TRUSTED_PROXY_CIDRS"))
	if err != nil {
		return Config{}, err
	}

	// The citizen-session bridge: both halves or neither (see the fields). Named in the error,
	// never with a value — the keys are credentials (rule 8).
	diaChiCau := strings.TrimSpace(os.Getenv("CITIZEN_SESSION_BRIDGE_LISTEN_ADDR"))
	var khoaCau []secret.Secret
	for _, phan := range danhSach(os.Getenv("CITIZEN_SESSION_BRIDGE_KEYS")) {
		khoaCau = append(khoaCau, secret.Secret(phan))
	}
	switch {
	case diaChiCau != "" && len(khoaCau) == 0:
		return Config{}, fmt.Errorf("%w: CITIZEN_SESSION_BRIDGE_LISTEN_ADDR có giá trị nhưng "+
			"CITIZEN_SESSION_BRIDGE_KEYS trống — cổng cầu không có khoá (service %s)", ErrCauPhienNuaVoi, serviceName)
	case diaChiCau == "" && len(khoaCau) > 0:
		return Config{}, fmt.Errorf("%w: CITIZEN_SESSION_BRIDGE_KEYS có giá trị nhưng "+
			"CITIZEN_SESSION_BRIDGE_LISTEN_ADDR trống — khoá nằm trong một tiến trình không mở cổng cầu (service %s)",
			ErrCauPhienNuaVoi, serviceName)
	}

	thoiHanPhien, err := thoiHanPhienCongDan(os.Getenv("CITIZEN_SESSION_TTL"))
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		TrustedProxies:                 proxy,
		CitizenSessionBridgeListenAddr: diaChiCau,
		CitizenSessionBridgeKeys:       khoaCau,
		CitizenSessionTTL:              thoiHanPhien,
		IdentityAdminSeedPassword:      secret.Secret(strings.TrimSpace(os.Getenv("IDENTITY_ADMIN_SEED_PASSWORD"))),
		// Trimmed: a trailing newline pasted into a ConfigMap would otherwise reach net.Listen
		// as part of the port. The five services that used to read it per-service trimmed too.
		ListenAddr:       firstNonEmpty(strings.TrimSpace(os.Getenv("LISTEN_ADDR")), ":8080"),
		GRPCListenAddr:   firstNonEmpty(os.Getenv("GRPC_LISTEN_ADDR"), ":9090"),
		DatabaseDSN:      secret.DSN(dsn),
		RedisDSN:         secret.DSN(strings.TrimSpace(os.Getenv("REDIS_DSN"))),
		PlatformGRPCAddr: strings.TrimSpace(os.Getenv("PLATFORM_GRPC_ADDR")),
		IdentityGRPCAddr: strings.TrimSpace(os.Getenv("IDENTITY_GRPC_ADDR")),
		// NONE OF THE FOLLOWING FIVE APPEARS IN `thieu` ABOVE, and that is a decision rather
		// than an omission. Nothing in this repository connects to RabbitMQ or Elasticsearch
		// yet; a variable made required at Load stops all eight services from starting until
		// four more values are set, in order to protect code that does not exist. Each is
		// refused BY NAME by the first thing that genuinely needs it — the IDENTITY_GRPC_ADDR
		// precedent — so an absent broker is a named refusal at connect time, never a reminder
		// that silently never fires.
		//
		// Trimmed for the same reason as GRPC_CALLER_KEY below: a trailing newline pasted out
		// of a k8s Secret or ConfigMap turns into a connection error that names the wrong cause.
		RabbitMQDSN:              secret.DSN(strings.TrimSpace(os.Getenv("RABBITMQ_DSN"))),
		RabbitMQExchange:         strings.TrimSpace(os.Getenv("RABBITMQ_EXCHANGE")),
		ElasticsearchAddrs:       danhSach(os.Getenv("ELASTICSEARCH_ADDRS")),
		ElasticsearchAPIKey:      secret.Secret(strings.TrimSpace(os.Getenv("ELASTICSEARCH_API_KEY"))),
		ElasticsearchIndexPrefix: strings.TrimSpace(os.Getenv("ELASTICSEARCH_INDEX_PREFIX")),
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
	// A HALF-CONFIGURED DEPENDENCY, WHICH IS THE ONE STATE NOBODY MEANS TO BE IN. Both pairs
	// below report a name that was set while the thing it names has nowhere to go — and the
	// usual cause is a typo'd key in the ConfigMap or the Secret, which leaves the intended
	// variable empty while a plausible-looking one is present.
	//
	// DELIBERATELY SILENT WHEN BOTH ARE EMPTY. Nothing uses RabbitMQ or Elasticsearch yet, so a
	// warning on every startup of all eight services would be noise in every environment — and
	// a CanhBao list that is never empty is a list operators learn to scroll past, which costs
	// the DANGEROUS_AUTH_BYPASS line above its only reader.
	if c.RabbitMQExchange != "" && c.RabbitMQDSN == "" {
		ra = append(ra, "RABBITMQ_EXCHANGE có giá trị nhưng RABBITMQ_DSN trống — "+
			"tác vụ nền không có chỗ để gửi; kiểm lại tên khoá trong k8s Secret")
	}
	if !c.ElasticsearchAPIKey.Rong() && len(c.ElasticsearchAddrs) == 0 {
		ra = append(ra, "ELASTICSEARCH_API_KEY có giá trị nhưng ELASTICSEARCH_ADDRS trống — "+
			"một khoá đang nằm trong môi trường của tiến trình không bao giờ dùng tới nó")
	}
	// An index holding petition contents is an index holding citizen personal data (rule 3).
	// Reachable without authentication is a choice somebody may legitimately make inside a
	// closed network — it is not refused, it is said out loud so it is owned. Dev is exempt:
	// a local single-node Elastic has no credential to configure.
	if c.Env != EnvDev && len(c.ElasticsearchAddrs) > 0 && c.ElasticsearchAPIKey.Rong() {
		ra = append(ra, "ELASTICSEARCH_ADDRS có giá trị nhưng ELASTICSEARCH_API_KEY trống — "+
			"cụm tìm kiếm chứa nội dung phản ánh đang mở, không xác thực")
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
	// Every DSN field, not merely the two that existed when this function was written. A DSN
	// added to the struct and forgotten here is a value that still leaks through .Lo() to
	// anything holding a "redacted" copy.
	c.RabbitMQDSN = secret.DSN(c.RabbitMQDSN.String())
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

// thoiHanPhienCongDan parses CITIZEN_SESSION_TTL. Empty is the owner's default; anything else
// must be a positive Go duration.
//
// NOT duration() above, whose silent fallback is right for a cache TTL and wrong for the lifetime
// of a credential: "30d" (not Go syntax) would quietly become the default, and the operator who
// wrote "7 days" would never learn that citizens hold sessions four times longer.
func thoiHanPhienCongDan(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ThoiHanPhienCongDanMacDinh, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%w: %q không phải khoảng thời gian Go (ví dụ 720h)", ErrThoiHanPhienHong, raw)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%w: %q phải lớn hơn 0", ErrThoiHanPhienHong, raw)
	}
	return d, nil
}

// khoaKy splits the comma-separated key list. The FIRST entry is the one that signs.
//
// Length and strength are NOT checked here: pkg/token.NewSigner owns that, and one owner for
// one rule is what keeps the two from drifting apart (rule 9). This function only answers
// "which strings were configured".
func khoaKy(raw string) []Khoa {
	var ra []Khoa
	for _, phan := range danhSach(raw) {
		ra = append(ra, Khoa(phan))
	}
	return ra
}

// danhSach splits a comma-separated variable, trims each entry and drops the empty ones.
//
// ONE SPLITTER, NOT TWO. Trailing commas and stray whitespace arrive from every hand-edited
// manifest, and a second copy of this loop is a second place for the trimming to be forgotten —
// which shows up as a cluster node addressed with a leading space and a dial error that names
// the wrong cause. Order is preserved: SESSION_SIGNING_KEYS depends on it, because the first
// key signs.
func danhSach(raw string) []string {
	var ra []string
	for _, phan := range strings.Split(raw, ",") {
		if phan = strings.TrimSpace(phan); phan != "" {
			ra = append(ra, phan)
		}
	}
	return ra
}

// proxyTinCay parses TRUSTED_PROXY_CIDRS. Blank returns nil — trust nobody.
//
// The index in an error is the 1-based position in the RAW comma-split value, empty entries
// counted, so it points at the same place an operator sees in the ConfigMap. danhSach is not used
// here for that reason: it drops empty entries, which would shift the number off the line.
func proxyTinCay(raw string) ([]netip.Prefix, error) {
	var ra []netip.Prefix
	for i, phan := range strings.Split(raw, ",") {
		phan = strings.TrimSpace(phan)
		if phan == "" {
			continue
		}
		var p netip.Prefix
		if strings.Contains(phan, "/") {
			pp, err := netip.ParsePrefix(phan)
			if err != nil {
				return nil, fmt.Errorf("%w: mục thứ %d (%q): %w", ErrProxyTinCayHong, i+1, phan, err)
			}
			p = pp.Masked()
		} else {
			a, err := netip.ParseAddr(phan)
			if err != nil {
				return nil, fmt.Errorf("%w: mục thứ %d (%q): %w", ErrProxyTinCayHong, i+1, phan, err)
			}
			// Unmapped so "::ffff:10.0.0.5" means the same host as "10.0.0.5": the request side
			// unmaps too, and a mapped prefix would otherwise never match anything.
			a = a.Unmap().WithZone("")
			p = netip.PrefixFrom(a, a.BitLen())
		}
		if !p.IsValid() {
			return nil, fmt.Errorf("%w: mục thứ %d (%q)", ErrProxyTinCayHong, i+1, phan)
		}
		if p.Bits() == 0 {
			return nil, fmt.Errorf("%w: mục thứ %d (%q) tin mọi địa chỉ — "+
				"tức là tin X-Forwarded-For giả của bất kỳ ai", ErrProxyTinCayHong, i+1, phan)
		}
		ra = append(ra, p)
	}
	return ra, nil
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
