package config

import (
	"fmt"
	"strings"
)

// Group is one set of variables a service reads because it USES one dependency or one feature.
//
// WHY GROUPS EXIST (owner's decision, 2026-09-29). Before them, Load read every variable for every
// service: petitions' object storage was parsed in platform, identity's signing keys were demanded
// from reporting, and a CanhBao line about a half-configured bucket appeared in pods that never
// upload. Worse, "required" could only mean "required in EVERY service", so almost everything had
// to be optional — including variables whose absence in production is a silent defect (no Redis =
// no duplicate protection; no TRUSTED_PROXY_CIDRS = every audit entry records the ingress IP).
//
// Each main now declares what it uses, and only those groups are read, parsed and validated. An
// UNDECLARED group is ignored entirely — not refused, not warned: the shared ConfigMap
// `common-config` hands every pod every non-secret key, so "set but unused" is the normal state.
// A DECLARED group is held to its rule: in staging and prod every variable of it must be set,
// except the ones with a stated default (see the r.read calls in Load, where each decision sits
// beside its variable, rule 11 invariant 8).
//
// Reading a value of an undeclared group PANICS (Config.require). A zero value there would look
// exactly like "feature off", and the omission would surface as a route answering 503 in
// production. CheckUses turns the same omission into a red test before it reaches a pod.
type Group uint8

// The groups. Base variables (ENV, DATABASE_DSN, DANGEROUS_AUTH_BYPASS) belong to no group: every
// caller of Load reads them. The variables of each group are the r.read calls naming it in Load.
const (
	// HTTPServer: LISTEN_ADDR, TRUSTED_PROXY_CIDRS — every process serving the REST edge.
	HTTPServer Group = iota + 1
	// GRPCServer: GRPC_LISTEN_ADDR, GRPC_CALLER_KEY — a process serving the inter-service port.
	GRPCServer
	// PlatformClient: PLATFORM_GRPC_ADDR, GRPC_CALLER_KEY — resolving Host to a commune.
	PlatformClient
	// IdentityClient: IDENTITY_GRPC_ADDR, GRPC_CALLER_KEY — resolving a staff principal.
	IdentityClient
	// OrgUnitOwnerClients: PETITIONS_GRPC_ADDR, DOCUMENTS_GRPC_ADDR, GRPC_CALLER_KEY — identity
	// asking the owners before it soft-deletes an org unit.
	OrgUnitOwnerClients
	// TenantCache: TENANT_CACHE_TTL — the cached commune directory.
	TenantCache
	// Redis: REDIS_DSN — duplicate-request protection and rate limiting.
	Redis
	// StaffSessionSigning: SESSION_SIGNING_KEYS — ISSUING and verifying the staff session token.
	// Identity only: every other service asks identity (IdentityClient), and a copy of the key in
	// any other pod is one more place a forger can read it from.
	StaffSessionSigning
	// CitizenCORS: CITIZEN_CORS_ALLOWED_ORIGINS — a citizen edge the Mini App calls cross-origin.
	CitizenCORS
	// CitizenBridge: CITIZEN_SESSION_BRIDGE_LISTEN_ADDR, CITIZEN_SESSION_BRIDGE_KEYS,
	// CITIZEN_SESSION_TTL — identity's citizen-session bridge (ADR 0045).
	CitizenBridge
	// AdminSeed: IDENTITY_ADMIN_SEED_PASSWORD — the one-off default administrator of a new commune.
	AdminSeed
	// ObjectStore: OBJECT_STORAGE_* except PUBLIC_MEDIA_BASE_URL (ADR 0052).
	ObjectStore
	// PublicMedia: OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL — only a service publishing public media.
	PublicMedia
	// MalwareScan: MALWARE_SCANNER_ADDRESS (ADR 0052 §9).
	MalwareScan
	// SecretEncryption: SECRET_ENCRYPTION_KEYS — per-commune secrets at rest (ADR 0009).
	SecretEncryption
	// OperatorRealm: OPERATOR_SESSION_SIGNING_KEYS, OPERATOR_TOTP_ENCRYPTION_KEY (ADR 0048) — the
	// service that ISSUES operator sessions (identity). The edge that only verifies them declares
	// OperatorEdge instead.
	OperatorRealm
	// RabbitMQ: RABBITMQ_DSN, RABBITMQ_EXCHANGE (ADR 0010).
	RabbitMQ
	// Elasticsearch: ELASTICSEARCH_ADDRS, ELASTICSEARCH_API_KEY, ELASTICSEARCH_INDEX_PREFIX.
	Elasticsearch
	// CommsClient: COMMS_GRPC_ADDR, GRPC_CALLER_KEY — an automation runner delivering staff
	// notices into comms' bell inbox (ADR 0058 §3). APPENDED, never inserted: a Group is a bit
	// position, and renumbering the ones above changes nothing on disk but reads badly in a diff.
	CommsClient
	// OperatorEdge: OPERATOR_HOST, OPERATOR_SESSION_SIGNING_KEYS — the operator area's HTTP edge,
	// service-platform only (ADR 0048 §"Chốt của chủ dự án — 01/10/2026" #2). A SEPARATE GROUP FROM
	// OperatorRealm because the edge needs the signing keys to check `op1.` signatures and nothing
	// else: declaring OperatorRealm there would also demand OPERATOR_TOTP_ENCRYPTION_KEY, putting the
	// key that decrypts every operator's second factor into a pod that never decrypts one.
	OperatorEdge

	groupEnd // not a group: the bound Uses checks against
)

var groupNames = map[Group]string{
	HTTPServer:          "HTTPServer",
	GRPCServer:          "GRPCServer",
	PlatformClient:      "PlatformClient",
	IdentityClient:      "IdentityClient",
	OrgUnitOwnerClients: "OrgUnitOwnerClients",
	TenantCache:         "TenantCache",
	Redis:               "Redis",
	StaffSessionSigning: "StaffSessionSigning",
	CitizenCORS:         "CitizenCORS",
	CitizenBridge:       "CitizenBridge",
	AdminSeed:           "AdminSeed",
	ObjectStore:         "ObjectStore",
	PublicMedia:         "PublicMedia",
	MalwareScan:         "MalwareScan",
	SecretEncryption:    "SecretEncryption",
	OperatorRealm:       "OperatorRealm",
	RabbitMQ:            "RabbitMQ",
	Elasticsearch:       "Elasticsearch",
	CommsClient:         "CommsClient",
	OperatorEdge:        "OperatorEdge",
}

// String is the identifier a main writes (`config.Redis`), so a message names what to add.
func (g Group) String() string {
	if n, ok := groupNames[g]; ok {
		return "config." + n
	}
	return fmt.Sprintf("config.Group(%d)", uint8(g))
}

// Usage is the set of groups one service declared. The zero value declares nothing: only the base
// variables are read.
type Usage struct{ bits uint64 }

// Uses builds the declaration a main passes to Load.
//
// An out-of-range Group panics: it can only come from a conversion like Group(99), which is a
// programming error that must not silently declare nothing.
func Uses(groups ...Group) Usage {
	var u Usage
	for _, g := range groups {
		if g == 0 || g >= groupEnd {
			panic(fmt.Sprintf("config.Uses: %d is not a group", uint8(g)))
		}
		u.bits |= 1 << g
	}
	return u
}

func (u Usage) has(g Group) bool { return u.bits&(1<<g) != 0 }

func (u Usage) hasAny(groups []Group) bool {
	for _, g := range groups {
		if u.has(g) {
			return true
		}
	}
	return false
}

// Groups lists the declared groups in declaration order of the constants.
func (u Usage) Groups() []Group {
	var out []Group
	for g := Group(1); g < groupEnd; g++ {
		if u.has(g) {
			out = append(out, g)
		}
	}
	return out
}

func joinGroups(groups []Group) string {
	s := make([]string, len(groups))
	for i, g := range groups {
		s[i] = g.String()
	}
	return strings.Join(s, " or ")
}

// requirement is how strictly Load treats an EMPTY value of a variable whose group is declared.
type requirement int

const (
	// requiredEverywhere: empty refuses in every environment, dev included.
	requiredEverywhere requirement = iota
	// requiredInProd: empty refuses in staging and prod; in dev it means the feature is off.
	requiredInProd
	// optional: empty is never refused — the variable has a stated default, or empty is the
	// designed "off" state. The reason is written beside each such r.read call.
	optional
)

// reader applies the declaration and the requirement to one raw value. It is the only path from
// os.Getenv to a Config field inside Load, so "declared" and "required" cannot be forgotten for a
// variable added later: a call without its requirement does not compile.
type reader struct {
	uses Usage
	// strict is staging or prod. STAGING IS HELD TO PROD'S RULE on purpose: staging is where a
	// missing variable must be discovered, and a staging that tolerates what prod refuses only
	// moves the first refusal to the production rollout.
	strict  bool
	missing []string
}

// read returns the trimmed value of one variable, or "" when none of its groups is declared — in
// which case the value is ignored entirely, even if malformed. No groups means a base variable.
//
// Trimmed for every variable: a trailing newline pasted into a ConfigMap or Secret turns into a
// connection error, a listen error or a signature mismatch that names the wrong cause.
func (r *reader) read(name, raw string, req requirement, groups ...Group) string {
	if len(groups) > 0 && !r.uses.hasAny(groups) {
		return ""
	}
	v := strings.TrimSpace(raw)
	if v == "" && (req == requiredEverywhere || (req == requiredInProd && r.strict)) {
		r.missing = append(r.missing, name)
	}
	return v
}

// undeclaredRead is the panic value of an accessor whose group the service did not declare.
type undeclaredRead struct {
	service string
	method  string
	groups  []Group
}

func (e undeclaredRead) Error() string {
	return fmt.Sprintf("config: service %q read Config.%s() without declaring %s in config.Uses — "+
		"an undeclared group is never loaded, so the value would be a silent zero",
		e.service, e.method, joinGroups(e.groups))
}

// require panics unless the service declared one of groups. Every accessor of a grouped value
// calls it first. See Group for why a panic and not a zero value.
func (c Config) require(method string, groups ...Group) {
	if !c.uses.hasAny(groups) {
		panic(undeclaredRead{service: c.service, method: method, groups: groups})
	}
}
