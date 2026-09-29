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
//	Variable                              Object     Group                When empty
//	-------------------------------------------------------------------------------------------------
//	ENV                                   ConfigMap  (base, every service)  refused everywhere
//	DATABASE_DSN                          Secret     (base, every service)  refused everywhere
//	DANGEROUS_AUTH_BYPASS                 ConfigMap  (base, every service)  off; "true" refused when ENV=prod
//	LISTEN_ADDR                           ConfigMap  HTTPServer             default :8080
//	TRUSTED_PROXY_CIDRS                   ConfigMap  HTTPServer             refused in staging/prod; dev trusts nobody
//	GRPC_LISTEN_ADDR                      ConfigMap  GRPCServer             default :9090
//	GRPC_CALLER_KEY                       Secret     GRPCServer + clients   refused everywhere (ADR 0025)
//	PLATFORM_GRPC_ADDR                    ConfigMap  PlatformClient         refused in staging/prod; dev: refused at Dial
//	IDENTITY_GRPC_ADDR                    ConfigMap  IdentityClient         refused in staging/prod; dev: refused at Dial
//	PETITIONS_GRPC_ADDR                   ConfigMap  OrgUnitOwnerClients    refused in staging/prod; dev: delete answers 503
//	DOCUMENTS_GRPC_ADDR                   ConfigMap  OrgUnitOwnerClients    as above
//	COMMS_GRPC_ADDR                       ConfigMap  CommsClient            refused in staging/prod; dev: automation runner off
//	TENANT_CACHE_TTL                      —          TenantCache            default 30s, not set in the cluster
//	REDIS_DSN                             Secret     Redis                  refused in staging/prod; dev: no cache
//	SESSION_SIGNING_KEYS                  Secret     StaffSessionSigning    refused in staging/prod; dev: no sessions
//	CITIZEN_CORS_ALLOWED_ORIGINS          ConfigMap  CitizenCORS            refused in staging/prod; dev: no CORS
//	CITIZEN_SESSION_BRIDGE_LISTEN_ADDR    ConfigMap  CitizenBridge          refused in staging/prod; dev: both or neither
//	CITIZEN_SESSION_BRIDGE_KEYS           Secret     CitizenBridge          as above
//	CITIZEN_SESSION_TTL                   ConfigMap  CitizenBridge          default 720h; malformed refused
//	IDENTITY_ADMIN_SEED_PASSWORD          Secret     AdminSeed              off, in EVERY environment (bootstrap switch)
//	OBJECT_STORAGE_ENDPOINT               ConfigMap  ObjectStore            refused in staging/prod; dev: uploads refused
//	OBJECT_STORAGE_PUBLIC_ENDPOINT        ConfigMap  ObjectStore            as above
//	OBJECT_STORAGE_ACCESS_KEY             Secret     ObjectStore            as above; one pair per service (ADR 0052 §3)
//	OBJECT_STORAGE_SECRET_KEY             Secret     ObjectStore            as above
//	OBJECT_STORAGE_BUCKET_PREFIX          ConfigMap  ObjectStore            as above
//	OBJECT_STORAGE_REGION                 ConfigMap  ObjectStore            default us-east-1
//	OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL  ConfigMap  PublicMedia            refused in staging/prod; dev: PublicURL refused
//	MALWARE_SCANNER_ADDRESS               ConfigMap  MalwareScan            refused in staging/prod; dev: uploads refused
//	SECRET_ENCRYPTION_KEYS                Secret     SecretEncryption       refused in staging/prod; dev: those operations refused
//	OPERATOR_SESSION_SIGNING_KEYS         Secret     OperatorRealm          refused in staging/prod; dev: operator sign-in refused
//	OPERATOR_TOTP_ENCRYPTION_KEY          Secret     OperatorRealm          as above
//	RABBITMQ_DSN, RABBITMQ_EXCHANGE       Secret/CM  RabbitMQ               refused in staging/prod once declared
//	ELASTICSEARCH_ADDRS / _API_KEY / _INDEX_PREFIX   Elasticsearch          refused in staging/prod once declared
//
// An UNDECLARED group is not read at all, in any environment — see Group (uses.go). Which service
// declares which group is each main's `configUses`; a malformed value of a declared group is
// refused in every environment, by name.
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
//
// TWO KINDS OF FIELD. The base values every caller of Load reads — Env, DatabaseDSN,
// DangerousAuthBypass — are plain exported fields. Every other value belongs to a Group (uses.go)
// and is held unexported behind an accessor of the same name (cfg.RedisDSN()), which panics when
// the service did not declare that group: an undeclared group is never loaded, and its zero value
// would read as "feature off" with nothing pointing at the missing declaration.
//
// The field comments below keep the accessor's name, because that is the name every caller reads.
type Config struct {
	// service and uses are what Load was called with; the accessors check uses.
	service string
	uses    Usage

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
	listenAddr string

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
	grpcListenAddr string

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
	// Group Redis. REQUIRED IN STAGING/PROD for a service that declares it (since 2026-09-29; it
	// used to be a warning): without it a double-submitted POST creates a second permanent record.
	// Empty in dev means no cache: every route then behaves per the CheDoHong it declared —
	// idem.MoKhiHong passes, idem.DongKhiHong answers 503.
	//
	// Same type, same reason as DatabaseDSN: it carries a credential.
	redisDSN secret.DSN

	// RabbitMQDSN is the broker for DELAYED background work — deadline reminders, escalation,
	// and the five automation jobs. ADR 0010 fixes what it is and is not for: background tasks
	// with a delay, NEVER inter-service events, which are Kafka's. Putting an inter-service
	// event on this connection is rule 2, invariant 3 broken, not a shortcut.
	//
	// k8s SECRET: an amqp URL carries the broker password. Same type and same reason as
	// DatabaseDSN — secret.DSN redacts the password on every rendering path while keeping the
	// host readable, so a startup line still says which broker this process opened.
	//
	// Group RabbitMQ, WHICH NO SERVICE DECLARES YET: nothing in this repository publishes or
	// consumes a task. Required belongs to what is actually used, and config.Uses is how "used"
	// is said — the first service that wires a job declares the group, and from then on staging
	// and prod refuse to start it without the broker, by name.
	rabbitMQDSN secret.DSN

	// RabbitMQExchange is the exchange the background-task publisher binds to.
	//
	// k8s CONFIGMAP: a name is not a credential. It belongs in a manifest a reviewer can read.
	//
	// NO DEFAULT, on purpose, and this one is not the usual "a default on the isolation path"
	// argument — it is that the exchange topology has not been decided. ADR 0010 chose RabbitMQ
	// and named what it is for; it did not name an exchange, a queue or a routing key. Writing
	// "vigov.tasks" here would record that decision in a source file instead of in an ADR, and
	// the next person would read it as settled. Empty until whoever wires the first job names it.
	rabbitMQExchange string

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
	// Group Elasticsearch, which no service declares: ADR 0010 itself says to consider DEFERRING
	// Elastic, on the grounds that pg_trgm + unaccent is probably enough at ~10,000 records per
	// commune, and that it should be added when search is measured slow rather than assumed so.
	elasticsearchAddrs []string

	// ElasticsearchAPIKey authenticates to the search cluster.
	//
	// k8s SECRET: the bytes ARE the credential, so the type has to refuse to render them — same
	// reason as GRPCCallerKey. An API key differs from a DSN in that there is no host inside it
	// worth keeping readable, so it collapses to *** on every path.
	//
	// Required in staging/prod with the rest of the group: an index holding petition contents
	// holds citizen personal data (rule 3), and "reachable without authentication" is not a
	// default anybody should inherit. It used to be a CanhBao warning.
	elasticsearchAPIKey secret.Secret

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
	elasticsearchIndexPrefix string

	// PlatformGRPCAddr is where the platform service answers ResolveHost — the RPC that maps an
	// incoming Host to a commune (proto/vigov/platform/v1/platform.proto).
	//
	// IT HAS NO DEFAULT, deliberately. Every service edge resolves its commune through this
	// address, so guessing it wrong means either refusing every request or — far worse —
	// resolving communes against something that is not the registry. Rule 1, forbidden #1 is
	// about defaults on the isolation path, and the address of the registry is on it.
	//
	// Group PlatformClient, which platform itself never declares: it holds the registry and calls
	// nobody. A declaring service is refused in staging/prod without it; in dev, platformclient.Dial
	// refuses the empty address by name.
	platformGRPCAddr string

	// IdentityGRPCAddr is where the identity service answers ResolveStaffPrincipal — the RPC that
	// turns a staff member's session cookie into the principal every guard needs
	// (proto/vigov/identity/v1/identity.proto).
	//
	// EXACTLY THE SHAPE OF PlatformGRPCAddr ABOVE, AND FOR THE SAME REASON: no default, refused at
	// Load in staging/prod, refused by name at Dial in dev. A guessed address would not fail closed in an obvious
	// way — it would make every staff request answer 503 while identity was healthy, which reads as
	// "identity is down" and sends somebody to inspect the wrong service for an afternoon.
	//
	// Group IdentityClient, which two services never declare: identity itself builds its principal
	// from its own session registry (its XacThuc), and platform holds the registry and calls nobody.
	//
	// CLUSTER-INTERNAL ADDRESS ONLY. A WORKING SESSION TOKEN travels on this hop and there is no
	// TLS on it (ADR 0025); an address that leaves the cluster puts every staff session on the wire
	// in the clear.
	identityGRPCAddr string

	// PetitionsGRPCAddr and DocumentsGRPCAddr are where petitions and documents answer
	// CountOrgUnitHoldings — the question identity asks before it soft-deletes an org unit (menu
	// Cấu hình §12.4; proto/vigov/{petitions,documents}/v1).
	//
	// Group OrgUnitOwnerClients, identity only. THE SHAPE OF IdentityGRPCAddr: no default, refused
	// at Load in staging/prod; in dev, empty makes org-unit delete answer 503. A guessed address would not fail open — an unreachable owner refuses the
	// delete (fail closed, core/petitionsclient) — but it would refuse EVERY org-unit delete with a
	// "try again" message while both services are healthy, which sends somebody to inspect the
	// wrong service.
	//
	// CLUSTER-INTERNAL ADDRESS ONLY. The hop is plaintext (ADR 0025; tools/security_debt.json). What
	// travels on it is one org-unit id and two counts — no session token, no personal data — but the
	// caller key does, and an address that leaves the cluster puts that key on the wire.
	petitionsGRPCAddr string
	documentsGRPCAddr string

	// CommsGRPCAddr is where comms answers DeliverStaffNotifications — the bell inbox the
	// automation jobs write into (ADR 0058 §3; proto/vigov/comms/v1).
	//
	// Group CommsClient, declared by the services that RUN automation jobs. THE SHAPE OF
	// PetitionsGRPCAddr: no default, refused at Load in staging/prod; in dev, empty turns the runner
	// off with a warning. A guessed address would not fail open — a run whose delivery fails records
	// DEPENDENCY_UNAVAILABLE and the next run retries under the same idempotency keys — but every
	// commune's reminders would stop while comms is healthy.
	//
	// CLUSTER-INTERNAL ADDRESS ONLY. Plaintext hop (ADR 0025); what travels is staff codes and
	// Vietnamese sentences that by contract carry no citizen personal data — and the caller key.
	commsGRPCAddr string

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
	// REQUIRED, IN EVERY ENVIRONMENT, WITH NO DEV EXEMPTION (ADR 0025, invariant 3), for every
	// service declaring a gRPC group (GRPCServer or a client); a service with neither — reporting —
	// never reads it. Not the shape of PlatformGRPCAddr above, which dev allows empty: an address that is
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
	grpcCallerKey secret.Secret

	// TenantCacheTTL bounds how long a deactivated commune keeps being served, and how long a
	// reassigned domain keeps resolving to the old commune (ADR 0004, decision 5).
	tenantCacheTTL time.Duration

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
	// Group StaffSessionSigning, IDENTITY ONLY: identity is the one service that issues and reads
	// the token; every other service asks identity (ResolveStaffPrincipal). Empty is refused in
	// staging/prod: a service with no signing key cannot tell a real token from a forged one.
	// LEAST PRIVILEGE: whoever holds this value can forge a staff token for every commune, so it
	// belongs in identity's Secret and nowhere else (deploy/cau-hinh/README.md).
	sessionSigningKeys []Khoa

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
	// Group HTTPServer. REQUIRED IN STAGING/PROD (2026-09-29). Empty means TRUST NOBODY: the
	// process records the address on its own socket, which behind ingress and web-admin is a pod
	// IP — never FORGED, but wrong on EVERY audit entry in a cluster (rule 6, invariant 2), which
	// is why a cluster must set it. In dev, empty is still that safe failure.
	//
	// MALFORMED IS FATAL, NOT SKIPPED. Skipping a typo'd entry silently trusts fewer hops than
	// the operator wrote, and a "fixed" parser that fell back to trusting everything would let any
	// client write its own address into a government record. Either way the trail looks normal and
	// is wrong, so Load refuses and names the variable and the entry.
	//
	// 0.0.0.0/0 AND ::/0 ARE REFUSED BY NAME: trusting the whole internet as a proxy is exactly
	// trusting a forged X-Forwarded-For from anyone, which is the thing the boundary exists to stop.
	trustedProxies []netip.Prefix

	// CitizenCORSAllowedOrigins are the browser origins allowed to call the CITIZEN edge
	// cross-origin — the Zalo Mini App webview, which is served from a Zalo domain and calls
	// `<dịch vụ>.api.vigov.vn` (ADR 0046). Read from CITIZEN_CORS_ALLOWED_ORIGINS, comma-separated:
	// exact https origins or `https://*.<suffix>`. The rule is on NguonCORS (cors.go).
	//
	// CITIZEN EDGE ONLY, NEVER STAFF. Staff reach the services same-origin through web-admin
	// (ADR 0043) with a host-only cookie; a CORS grant there would be a way for another page to
	// drive a staff session. httpx.CORSCongDan is mounted on the citizen chain and nowhere else.
	//
	// k8s CONFIGMAP: an origin list is not a credential. PLATFORM-WIDE: one Mini App serves every
	// commune, so the origin it runs from is the same for all of them (rule 8, invariant 5).
	//
	// Group CitizenCORS (identity, petitions, comms). EMPTY MEANS NO CORS HEADERS AT ALL — the
	// browser then blocks the Mini App, the fail-closed direction. Refused in staging/prod for a
	// declaring service: a citizen edge the Mini App cannot reach is not a working deployment of it.
	//
	// `*` AND ANY NON-https ENTRY ARE REFUSED BY Load, never skipped: allow-all is the one value
	// that must not be reachable, and a skipped typo would read as "CORS is broken" for an afternoon.
	citizenCORSAllowedOrigins NguonCORS

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
	// Group CitizenBridge, identity only. REQUIRED IN STAGING/PROD with the keys (identity declares
	// the bridge, so a deployment without it is a Mini App nobody can sign in to). In dev, ONLY
	// TOGETHER WITH THE KEYS: both empty = bridge not started. Exactly one set = Load refuses: an address with
	// no key is a port that would have to choose between answering nobody and answering everybody,
	// and keys with no address are credentials sitting in a process that never uses them — the
	// usual cause of both is a typo'd key in the manifest (ADR 0045 §Cấu hình).
	citizenSessionBridgeListenAddr string

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
	citizenSessionBridgeKeys []secret.Secret

	// CitizenSessionTTL is how long a citizen session issued through the bridge lives.
	//
	// A PLATFORM-WIDE CONSTANT BY THE OWNER'S DECISION (ADR 0045, answer to CÒN MỞ #4): an
	// environment variable, default ONE MONTH. It is product policy, not a commune's value, so rule
	// 1 invariant 10 does not apply. Not the 7 days of vihat-miniapp's own session — that is that
	// repository's policy (ADR 0045 CÒN MỞ #4 records why the two are separate).
	//
	// NEVER REQUIRED — IT HAS THAT DEFAULT, because the owner stated it; a deployment that says
	// nothing gets the decided number, not a guessed one. A MALFORMED OR NON-POSITIVE VALUE IS
	// REFUSED rather than falling back like TENANT_CACHE_TTL does: silently turning a typo into 30
	// days would hide that the operator meant something else for the lifetime of a credential.
	// Go duration syntax — "720h", not "30d".
	citizenSessionTTL time.Duration

	// IdentityAdminSeedPassword is the password of the default administrator identity creates in
	// a commune at that commune's FIRST sign-in as `admin` (owner's decision, 2026-09-26; ledger
	// item `xa-moi-khong-co-vai-tro-va-quyen`). Read by identity only.
	//
	// Group AdminSeed. NEVER REQUIRED, NOT EVEN IN PROD, AND EMPTY MEANS THE FEATURE IS OFF — rule
	// 11 invariant 8. It is a one-off bootstrap switch, meant to be REMOVED from the Secret once
	// every commune has changed its admin password; requiring it would force a live credential for
	// the highest-privileged account to stay in the cluster forever.
	//
	// k8s SECRET (`bi-mat-identity`), secret.Secret: it is a live credential that opens the
	// highest-privileged account of every commune not yet seeded (rule 8, rule 11 invariant 7).
	// Its strength is judged where it is USED — identity refuses to seed with one shorter than
	// password.DaiToiThieu — for the same one-owner-per-rule reason CitizenSessionBridgeKeys gives.
	//
	// Trimmed: a trailing newline pasted into a Secret would otherwise make the typed password
	// never match, and the refusal would read as "wrong password" with nothing pointing here.
	identityAdminSeedPassword secret.Secret

	// ObjectStorage is the MinIO/S3 connection of ADR 0052, consumed by core/storage.New. Groups
	// ObjectStore and PublicMedia; what each field requires is on the type (object_storage.go).
	objectStorage ObjectStorage

	// MalwareScanner is the clamd connection of ADR 0052 §9, consumed by core/malwarescan.New.
	// Group MalwareScan; the reasons are on the type (malware_scanner.go).
	malwareScanner MalwareScanner

	// OperatorSessionSigningKeys sign and verify the OPERATOR realm token (`op1.`, ADR 0048
	// owner's decision #2) — a SEPARATE list from SessionSigningKeys, refused by Load if any entry
	// is shared with it. First entry signs, every entry verifies. Read by identity only.
	//
	// Group OperatorRealm, which no service declares yet (ADR 0048 is not wired): absent means
	// operator sign-in is refused (operatorauth.ErrNotConfigured). See operator.go. k8s SECRET,
	// secret.Secret (rule 8). Minimum length is owned by operatorauth.NewTokenSigner.
	operatorSessionSigningKeys []secret.Secret

	// OperatorTOTPEncryptionKeys encrypt operator TOTP secrets at rest (AES-256-GCM, ADR 0048
	// owner's decision #10). Read from OPERATOR_TOTP_ENCRYPTION_KEY — singular name, but a
	// comma-separated LIST so the key can rotate: first entry encrypts, every entry decrypts.
	// Each entry is standard base64 of exactly 32 bytes; the value held here is the DECODED key.
	//
	// Group OperatorRealm, same reason and same refusal as OperatorSessionSigningKeys. k8s SECRET.
	operatorTOTPEncryptionKeys []secret.Secret

	// SecretEncryptionKeys are the KEKs of ADR 0009, consumed by core/crypto.NewKeyring. Read from
	// SECRET_ENCRYPTION_KEYS, same format as OPERATOR_TOTP_ENCRYPTION_KEY; the value held here is
	// the DECODED key. Group SecretEncryption (comms); why the backup is not optional:
	// secret_encryption.go.
	secretEncryptionKeys []secret.Secret
}

// ThoiHanPhienCongDanMacDinh is CITIZEN_SESSION_TTL when the variable is unset — 30 days, the
// owner's decision of 2026-09-25 (ADR 0045, answer to CÒN MỞ #4).
const ThoiHanPhienCongDanMacDinh = 30 * 24 * time.Hour

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

// Load reads the configuration for one service: the base variables, plus the groups in uses.
//
// It FAILS instead of defaulting. A service that starts with a guessed database or a guessed
// listen address is a service that will be debugged at the wrong layer — and in this system a
// wrong guess on the isolation path is a data breach, not an inconvenience.
//
// WHAT uses CHANGES (uses.go). An undeclared group is not read at all: a malformed petitions-only
// OBJECT_STORAGE_ENDPOINT in the shared ConfigMap cannot stop platform. A declared group is held
// to its requirement: in staging and prod, every variable of it without a stated default must be
// set, and the refusal names each missing variable.
func Load(serviceName string, uses Usage) (Config, error) {
	// ENV first: every other requirement depends on it. An ENV nobody recognises fails as
	// ErrEnvKhongHopLe before anything else — reporting "missing REDIS_DSN" for a typo in ENV
	// would send the reader to the wrong variable.
	env := strings.ToLower(strings.TrimSpace(os.Getenv("ENV")))
	switch env {
	case "", EnvDev, EnvStaging, EnvProd:
	default:
		return Config{}, fmt.Errorf("%w: %q", ErrEnvKhongHopLe, env)
	}
	r := &reader{uses: uses, strict: env == EnvStaging || env == EnvProd}

	// ---- base: every caller of Load -------------------------------------------------------
	r.read("ENV", env, requiredEverywhere)
	dsn := r.read("DATABASE_DSN", os.Getenv("DATABASE_DSN"), requiredEverywhere)
	// Optional: empty is off, the only safe state; "on" is refused below when ENV=prod.
	bypass := r.read("DANGEROUS_AUTH_BYPASS", os.Getenv("DANGEROUS_AUTH_BYPASS"), optional)

	// ---- HTTPServer -----------------------------------------------------------------------
	// Optional: the default :8080 is what the Service, the probes and the NetworkPolicy assume.
	listen := r.read("LISTEN_ADDR", os.Getenv("LISTEN_ADDR"), optional, HTTPServer)
	// Required in prod: in a cluster every request arrives through ingress, so "trust nobody"
	// records the ingress pod's IP on EVERY audit entry — an audit trail that is wrong for every
	// commune at once (rule 6, invariant 2). In dev, empty still means trust nobody.
	proxyRaw := r.read("TRUSTED_PROXY_CIDRS", os.Getenv("TRUSTED_PROXY_CIDRS"), requiredInProd, HTTPServer)

	// ---- the inter-service port and its clients -------------------------------------------
	// Optional: the default :9090 is the port every gRPC Service in the cluster targets.
	grpcListen := r.read("GRPC_LISTEN_ADDR", os.Getenv("GRPC_LISTEN_ADDR"), optional, GRPCServer)
	// Required EVERYWHERE, dev included, for any process on the gRPC surface (ADR 0025, invariant
	// 3): a missing key would produce a port that answers ANYTHING that reaches it, silently and on
	// every RPC. A service with no gRPC server and no gRPC client (reporting) never reads it.
	callerKey := r.read("GRPC_CALLER_KEY", os.Getenv("GRPC_CALLER_KEY"), requiredEverywhere,
		GRPCServer, PlatformClient, IdentityClient, OrgUnitOwnerClients, CommsClient)
	// Required in prod; in dev empty is allowed here and refused by name at Dial.
	platformAddr := r.read("PLATFORM_GRPC_ADDR", os.Getenv("PLATFORM_GRPC_ADDR"), requiredInProd, PlatformClient)
	identityAddr := r.read("IDENTITY_GRPC_ADDR", os.Getenv("IDENTITY_GRPC_ADDR"), requiredInProd, IdentityClient)
	// Required in prod; in dev empty means org-unit delete answers 503 (fail closed).
	petitionsAddr := r.read("PETITIONS_GRPC_ADDR", os.Getenv("PETITIONS_GRPC_ADDR"), requiredInProd, OrgUnitOwnerClients)
	documentsAddr := r.read("DOCUMENTS_GRPC_ADDR", os.Getenv("DOCUMENTS_GRPC_ADDR"), requiredInProd, OrgUnitOwnerClients)
	// Required in prod: a runner that cannot deliver sends no commune any reminder. In dev empty
	// means the automation runner does not start (fail closed: nothing claimed, nothing sent).
	commsAddr := r.read("COMMS_GRPC_ADDR", os.Getenv("COMMS_GRPC_ADDR"), requiredInProd, CommsClient)

	// ---- TenantCache ----------------------------------------------------------------------
	// Optional: the default 30s is the decided value (ADR 0004 decision 5), deliberately not in
	// the cluster.
	tenantTTL := r.read("TENANT_CACHE_TTL", os.Getenv("TENANT_CACHE_TTL"), optional, TenantCache)

	// ---- Redis ----------------------------------------------------------------------------
	// Required in prod: without it a double-submitted POST creates a second permanent record,
	// and rule 7 forbids deleting it. In dev, empty = no cache; each route's idem mode decides.
	redis := r.read("REDIS_DSN", os.Getenv("REDIS_DSN"), requiredInProd, Redis)

	// ---- StaffSessionSigning (identity only) ----------------------------------------------
	// Required in prod: a service that issues staff sessions with no key either accepts forged
	// tokens or invents a key per replica and signs everybody out on every restart. In dev,
	// empty = this process cannot issue or read a session, reported by CanhBao.
	signingRaw := r.read("SESSION_SIGNING_KEYS", os.Getenv("SESSION_SIGNING_KEYS"), requiredInProd, StaffSessionSigning)

	// ---- CitizenCORS ----------------------------------------------------------------------
	// Required in prod: empty sends no CORS headers and the browser blocks the Mini App on this
	// edge. In dev, empty is that fail-closed state.
	corsRaw := r.read("CITIZEN_CORS_ALLOWED_ORIGINS", os.Getenv("CITIZEN_CORS_ALLOWED_ORIGINS"), requiredInProd, CitizenCORS)

	// ---- CitizenBridge (identity only) ----------------------------------------------------
	// Both required in prod: a declared bridge that is not started is a Mini App nobody can sign
	// in to. In dev, both-or-neither (below).
	bridgeAddr := r.read("CITIZEN_SESSION_BRIDGE_LISTEN_ADDR", os.Getenv("CITIZEN_SESSION_BRIDGE_LISTEN_ADDR"), requiredInProd, CitizenBridge)
	bridgeKeysRaw := r.read("CITIZEN_SESSION_BRIDGE_KEYS", os.Getenv("CITIZEN_SESSION_BRIDGE_KEYS"), requiredInProd, CitizenBridge)
	// Optional: the default 720h is the owner's decision (ADR 0045, CÒN MỞ #4).
	sessionTTLRaw := r.read("CITIZEN_SESSION_TTL", os.Getenv("CITIZEN_SESSION_TTL"), optional, CitizenBridge)

	// ---- AdminSeed (identity only) --------------------------------------------------------
	// Optional IN EVERY ENVIRONMENT, prod included: a one-off bootstrap switch, meant to be
	// REMOVED once every commune has changed its admin password. Empty = off, by design.
	seed := r.read("IDENTITY_ADMIN_SEED_PASSWORD", os.Getenv("IDENTITY_ADMIN_SEED_PASSWORD"), optional, AdminSeed)

	// ---- ObjectStore / PublicMedia / MalwareScan (ADR 0052) -------------------------------
	// Required in prod: a declared store that is absent refuses every upload. In dev, absent
	// makes storage.New answer ErrNotConfigured (fail closed).
	osEndpoint := r.read("OBJECT_STORAGE_ENDPOINT", os.Getenv("OBJECT_STORAGE_ENDPOINT"), requiredInProd, ObjectStore)
	osPublic := r.read("OBJECT_STORAGE_PUBLIC_ENDPOINT", os.Getenv("OBJECT_STORAGE_PUBLIC_ENDPOINT"), requiredInProd, ObjectStore)
	osAccess := r.read("OBJECT_STORAGE_ACCESS_KEY", os.Getenv("OBJECT_STORAGE_ACCESS_KEY"), requiredInProd, ObjectStore)
	osSecret := r.read("OBJECT_STORAGE_SECRET_KEY", os.Getenv("OBJECT_STORAGE_SECRET_KEY"), requiredInProd, ObjectStore)
	osPrefix := r.read("OBJECT_STORAGE_BUCKET_PREFIX", os.Getenv("OBJECT_STORAGE_BUCKET_PREFIX"), requiredInProd, ObjectStore)
	// Optional: the default us-east-1 is MinIO's own (see ObjectStorage.Region).
	osRegion := r.read("OBJECT_STORAGE_REGION", os.Getenv("OBJECT_STORAGE_REGION"), optional, ObjectStore)
	// Required in prod for a service that publishes public media; no service declares it yet.
	osMedia := r.read("OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL", os.Getenv("OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL"), requiredInProd, PublicMedia)
	// Required in prod: without a scanner every upload is refused (ADR 0052 §9, fail closed).
	scannerRaw := r.read("MALWARE_SCANNER_ADDRESS", os.Getenv("MALWARE_SCANNER_ADDRESS"), requiredInProd, MalwareScan)

	// ---- SecretEncryption (ADR 0009) ------------------------------------------------------
	// Required in prod: without it every save/read of a commune secret is refused.
	secretKeysRaw := r.read("SECRET_ENCRYPTION_KEYS", os.Getenv("SECRET_ENCRYPTION_KEYS"), requiredInProd, SecretEncryption)

	// ---- OperatorRealm (ADR 0048; no service declares it yet) -----------------------------
	// Required in prod once declared: a declared operator area that cannot sign anybody in.
	opSigningRaw := r.read("OPERATOR_SESSION_SIGNING_KEYS", os.Getenv("OPERATOR_SESSION_SIGNING_KEYS"), requiredInProd, OperatorRealm)
	opTOTPRaw := r.read("OPERATOR_TOTP_ENCRYPTION_KEY", os.Getenv("OPERATOR_TOTP_ENCRYPTION_KEY"), requiredInProd, OperatorRealm)

	// ---- RabbitMQ / Elasticsearch (ADR 0010; no service declares them yet) ----------------
	// Required in prod once declared — the service that declares one is the one that connects to
	// it. An Elasticsearch without an API key would hold petition contents unguarded (rule 3).
	rabbitDSN := r.read("RABBITMQ_DSN", os.Getenv("RABBITMQ_DSN"), requiredInProd, RabbitMQ)
	rabbitExchange := r.read("RABBITMQ_EXCHANGE", os.Getenv("RABBITMQ_EXCHANGE"), requiredInProd, RabbitMQ)
	esAddrs := r.read("ELASTICSEARCH_ADDRS", os.Getenv("ELASTICSEARCH_ADDRS"), requiredInProd, Elasticsearch)
	esKey := r.read("ELASTICSEARCH_API_KEY", os.Getenv("ELASTICSEARCH_API_KEY"), requiredInProd, Elasticsearch)
	esPrefix := r.read("ELASTICSEARCH_INDEX_PREFIX", os.Getenv("ELASTICSEARCH_INDEX_PREFIX"), requiredInProd, Elasticsearch)

	if len(r.missing) > 0 {
		return Config{}, fmt.Errorf("%w: %s (service %s, ENV=%s)",
			ErrThieuBienMoiTruong, strings.Join(r.missing, ", "), serviceName, env)
	}

	// ---- parse what was read; a MALFORMED value of a declared group is fatal ---------------
	proxy, err := proxyTinCay(proxyRaw)
	if err != nil {
		return Config{}, err
	}

	nguonCORS, err := PhanTichNguonCORS(corsRaw)
	if err != nil {
		return Config{}, err
	}

	// The citizen-session bridge: both halves or neither (see the fields). Named in the error,
	// never with a value — the keys are credentials (rule 8).
	var khoaCau []secret.Secret
	for _, phan := range danhSach(bridgeKeysRaw) {
		khoaCau = append(khoaCau, secret.Secret(phan))
	}
	switch {
	case bridgeAddr != "" && len(khoaCau) == 0:
		return Config{}, fmt.Errorf("%w: CITIZEN_SESSION_BRIDGE_LISTEN_ADDR có giá trị nhưng "+
			"CITIZEN_SESSION_BRIDGE_KEYS trống — cổng cầu không có khoá (service %s)", ErrCauPhienNuaVoi, serviceName)
	case bridgeAddr == "" && len(khoaCau) > 0:
		return Config{}, fmt.Errorf("%w: CITIZEN_SESSION_BRIDGE_KEYS có giá trị nhưng "+
			"CITIZEN_SESSION_BRIDGE_LISTEN_ADDR trống — khoá nằm trong một tiến trình không mở cổng cầu (service %s)",
			ErrCauPhienNuaVoi, serviceName)
	}

	var thoiHanPhien time.Duration
	if uses.has(CitizenBridge) {
		if thoiHanPhien, err = thoiHanPhienCongDan(sessionTTLRaw); err != nil {
			return Config{}, err
		}
	}

	var objectStorage ObjectStorage
	if uses.has(ObjectStore) || uses.has(PublicMedia) {
		objectStorage, err = parseObjectStorage(osEndpoint, osPublic, osMedia, osAccess, osSecret, osRegion, osPrefix)
		if err != nil {
			return Config{}, fmt.Errorf("%w (service %s)", err, serviceName)
		}
	}

	scannerAddresses, err := ParseMalwareScannerAddresses(scannerRaw)
	if err != nil {
		return Config{}, fmt.Errorf("%w (service %s)", err, serviceName)
	}

	// The overlap check compares against the staff keys as SET, not as declared: "the operator
	// realm shares no key material with staff sessions" is a property of the deployment's bytes,
	// and a service declaring OperatorRealm without StaffSessionSigning must not skip it.
	operatorSigningKeys, err := parseOperatorSigningKeys(opSigningRaw, khoaKy(os.Getenv("SESSION_SIGNING_KEYS")))
	if err != nil {
		return Config{}, fmt.Errorf("%w (service %s)", err, serviceName)
	}
	operatorTOTPKeys, err := parseOperatorTOTPKeys(opTOTPRaw)
	if err != nil {
		return Config{}, fmt.Errorf("%w (service %s)", err, serviceName)
	}

	// Malformed is fatal: a skipped entry is a KEK that silently cannot unwrap the DEKs it wrapped.
	secretEncryptionKeys, err := parseAES256KeyList(secretKeysRaw, ErrSecretEncryptionKeysInvalid)
	if err != nil {
		return Config{}, fmt.Errorf("%w (service %s)", err, serviceName)
	}

	cfg := Config{
		service:             serviceName,
		uses:                uses,
		DatabaseDSN:         secret.DSN(dsn),
		Env:                 env,
		DangerousAuthBypass: boolean(bypass),

		listenAddr:     firstNonEmpty(listen, ":8080"),
		trustedProxies: proxy,

		grpcListenAddr:    firstNonEmpty(grpcListen, ":9090"),
		grpcCallerKey:     secret.Secret(callerKey),
		platformGRPCAddr:  platformAddr,
		identityGRPCAddr:  identityAddr,
		petitionsGRPCAddr: petitionsAddr,
		documentsGRPCAddr: documentsAddr,
		commsGRPCAddr:     commsAddr,
		tenantCacheTTL:    duration(tenantTTL, 30*time.Second),

		redisDSN:           secret.DSN(redis),
		sessionSigningKeys: khoaKy(signingRaw),

		citizenCORSAllowedOrigins:      nguonCORS,
		citizenSessionBridgeListenAddr: bridgeAddr,
		citizenSessionBridgeKeys:       khoaCau,
		citizenSessionTTL:              thoiHanPhien,
		identityAdminSeedPassword:      secret.Secret(seed),

		objectStorage:              objectStorage,
		malwareScanner:             MalwareScanner{Addresses: scannerAddresses},
		secretEncryptionKeys:       secretEncryptionKeys,
		operatorSessionSigningKeys: operatorSigningKeys,
		operatorTOTPEncryptionKeys: operatorTOTPKeys,

		rabbitMQDSN:              secret.DSN(rabbitDSN),
		rabbitMQExchange:         rabbitExchange,
		elasticsearchAddrs:       danhSach(esAddrs),
		elasticsearchAPIKey:      secret.Secret(esKey),
		elasticsearchIndexPrefix: esPrefix,
	}

	// A dangerous flag left on in production is the failure mode rule 8 invariant 7 exists
	// for: it is switched on for a local afternoon and nobody remembers to switch it off.
	// Refusing to start is the only response that cannot be ignored. Every service, whatever it
	// declared.
	if cfg.Env == EnvProd && cfg.DangerousAuthBypass {
		return Config{}, fmt.Errorf("%w: DANGEROUS_AUTH_BYPASS", ErrCoBienNguyHiem)
	}

	return cfg, nil
}

// CanhBao lists the dangerous settings currently in force, for logging at startup.
// Empty in a correctly configured deployment.
//
// ONLY DECLARED GROUPS SPEAK. A pod that never uploads says nothing about OBJECT_STORAGE_*, and
// one that holds no cache says nothing about REDIS_DSN: the shared ConfigMap hands every pod every
// key, and a list that warns about somebody else's dependency is a list operators learn to scroll
// past — which costs the DANGEROUS_AUTH_BYPASS line its only reader.
//
// What staging/prod already REFUSE (a declared variable left empty) has no line here: Load never
// returns such a Config, so a warning for it would be dead code that reads like a safety net.
func (c Config) CanhBao() []string {
	var ra []string
	if c.DangerousAuthBypass {
		ra = append(ra, "DANGEROUS_AUTH_BYPASS đang BẬT — mọi kiểm tra xác thực bị bỏ qua")
	}
	if c.uses.has(TenantCache) && c.Env != EnvProd && c.tenantCacheTTL > time.Minute {
		ra = append(ra, "TENANT_CACHE_TTL dài hơn 1 phút — thay đổi tên miền sẽ chậm có hiệu lực")
	}
	// A HALF-CONFIGURED DEPENDENCY, WHICH IS THE ONE STATE NOBODY MEANS TO BE IN — reachable in
	// dev only, where an empty declared variable is allowed. The usual cause is a typo'd key,
	// which leaves the intended variable empty while a plausible-looking one is present.
	if c.uses.has(RabbitMQ) && c.rabbitMQExchange != "" && c.rabbitMQDSN == "" {
		ra = append(ra, "RABBITMQ_EXCHANGE có giá trị nhưng RABBITMQ_DSN trống — "+
			"tác vụ nền không có chỗ để gửi; kiểm lại tên khoá trong k8s Secret")
	}
	if c.uses.has(Elasticsearch) && !c.elasticsearchAPIKey.Rong() && len(c.elasticsearchAddrs) == 0 {
		ra = append(ra, "ELASTICSEARCH_API_KEY có giá trị nhưng ELASTICSEARCH_ADDRS trống — "+
			"một khoá đang nằm trong môi trường của tiến trình không bao giờ dùng tới nó")
	}
	if c.uses.has(ObjectStore) {
		// Object storage half-configured: the same "typo'd key" shape. Silent when nothing is set.
		if o := c.objectStorage; !o.untouched() && !o.Configured() {
			ra = append(ra, "OBJECT_STORAGE_* cấu hình nửa vời — thiếu "+strings.Join(o.Missing(), ", ")+
				"; mọi lần tải tệp lên sẽ bị từ chối")
		}
		// A presigned URL is a bearer credential (ADR 0052 §Cái giá). Over plain http it travels
		// in the clear from the browser to the store. Set, so not refused — but said out loud.
		if c.Env != EnvDev && strings.HasPrefix(c.objectStorage.PublicEndpoint, "http://") {
			ra = append(ra, "OBJECT_STORAGE_PUBLIC_ENDPOINT dùng http:// — presigned URL đi qua mạng không mã hoá")
		}
		// Storage without a scanner: every upload is refused at complete (ADR 0052 §9, fail
		// closed). Nobody configures storage meaning "refuse every upload", so it is said at
		// startup rather than at the first citizen — including when MalwareScan is not declared.
		if c.objectStorage.Configured() && !(c.uses.has(MalwareScan) && c.malwareScanner.Configured()) {
			ra = append(ra, "OBJECT_STORAGE_* đã cấu hình nhưng MALWARE_SCANNER_ADDRESS trống — "+
				"mọi lần tải tệp lên sẽ bị từ chối vì không quét được mã độc")
		}
	}
	if c.uses.has(OperatorRealm) {
		ra = append(ra, c.operatorWarnings()...)
	}
	if c.uses.has(StaffSessionSigning) {
		// Only reachable in dev — Load refuses to start anywhere else.
		if len(c.sessionSigningKeys) == 0 {
			ra = append(ra, "SESSION_SIGNING_KEYS trống — không ký và không đọc được phiên đăng nhập")
		}
		// One key means the key can never be rotated without signing out every cán bộ of every
		// xã at once (rule 8, invariant 6). Not an error, but somebody has to know before it is needed.
		if len(c.sessionSigningKeys) == 1 && c.Env == EnvProd {
			ra = append(ra, "SESSION_SIGNING_KEYS chỉ có một khoá — xoay khoá sẽ đăng xuất toàn bộ cán bộ")
		}
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
	c.redisDSN = secret.DSN(c.redisDSN.String())
	// Every DSN field, not merely the two that existed when this function was written. A DSN
	// added to the struct and forgotten here is a value that still leaks through .Lo() to
	// anything holding a "redacted" copy.
	c.rabbitMQDSN = secret.DSN(c.rabbitMQDSN.String())
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
func (c Config) KhoaKyBytes() []secret.Secret { // vi-name-ok: existing exported name, only its body changed (rule 12 invariant 3)
	c.require("KhoaKyBytes", StaffSessionSigning)
	ra := make([]secret.Secret, 0, len(c.sessionSigningKeys))
	ra = append(ra, c.sessionSigningKeys...)
	return ra
}

// CauPhienBat reports whether the citizen-session bridge listener is configured. Load guarantees
// the address and the keys are either both present or both absent.
func (c Config) CauPhienBat() bool { // vi-name-ok: existing exported name, only its body changed (rule 12 invariant 3)
	c.require("CauPhienBat", CitizenBridge)
	return c.citizenSessionBridgeListenAddr != "" && len(c.citizenSessionBridgeKeys) > 0
}

func boolean(s string) bool {
	b, err := strconv.ParseBool(strings.TrimSpace(s))
	return err == nil && b
}
