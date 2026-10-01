package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/netip"
	"time"

	"github.com/vihat/vigov/core/secret"
)

// The accessors of every grouped value. Each one calls require FIRST, so reading a value of an
// undeclared group panics instead of returning a zero that reads as "feature off" (uses.go).
// The meaning of each value is documented on its field in config.go.
//
// A new grouped field needs an accessor that calls require — TestEveryAccessorIsGuarded turns red
// on one that does not.

// ListenAddr is LISTEN_ADDR, default ":8080".
func (c Config) ListenAddr() string {
	c.require("ListenAddr", HTTPServer)
	return c.listenAddr
}

// TrustedProxies is TRUSTED_PROXY_CIDRS, parsed; nil trusts nobody.
func (c Config) TrustedProxies() []netip.Prefix {
	c.require("TrustedProxies", HTTPServer)
	return c.trustedProxies
}

// GRPCListenAddr is GRPC_LISTEN_ADDR, default ":9090".
func (c Config) GRPCListenAddr() string {
	c.require("GRPCListenAddr", GRPCServer)
	return c.grpcListenAddr
}

// GRPCCallerKey is GRPC_CALLER_KEY — read by a gRPC server and by every gRPC client.
func (c Config) GRPCCallerKey() secret.Secret {
	c.require("GRPCCallerKey", GRPCServer, PlatformClient, IdentityClient, OrgUnitOwnerClients, CommsClient)
	return c.grpcCallerKey
}

// PlatformGRPCAddr is PLATFORM_GRPC_ADDR.
func (c Config) PlatformGRPCAddr() string {
	c.require("PlatformGRPCAddr", PlatformClient)
	return c.platformGRPCAddr
}

// IdentityGRPCAddr is IDENTITY_GRPC_ADDR.
func (c Config) IdentityGRPCAddr() string {
	c.require("IdentityGRPCAddr", IdentityClient)
	return c.identityGRPCAddr
}

// PetitionsGRPCAddr is PETITIONS_GRPC_ADDR.
func (c Config) PetitionsGRPCAddr() string {
	c.require("PetitionsGRPCAddr", OrgUnitOwnerClients)
	return c.petitionsGRPCAddr
}

// DocumentsGRPCAddr is DOCUMENTS_GRPC_ADDR.
func (c Config) DocumentsGRPCAddr() string {
	c.require("DocumentsGRPCAddr", OrgUnitOwnerClients)
	return c.documentsGRPCAddr
}

// CommsGRPCAddr is COMMS_GRPC_ADDR.
func (c Config) CommsGRPCAddr() string {
	c.require("CommsGRPCAddr", CommsClient)
	return c.commsGRPCAddr
}

// TenantCacheTTL is TENANT_CACHE_TTL, default 30s.
func (c Config) TenantCacheTTL() time.Duration {
	c.require("TenantCacheTTL", TenantCache)
	return c.tenantCacheTTL
}

// RedisDSN is REDIS_DSN; empty only in dev.
func (c Config) RedisDSN() secret.DSN {
	c.require("RedisDSN", Redis)
	return c.redisDSN
}

// SessionSigningKeys is SESSION_SIGNING_KEYS, first entry signs.
func (c Config) SessionSigningKeys() []Khoa {
	c.require("SessionSigningKeys", StaffSessionSigning)
	return c.sessionSigningKeys
}

// CitizenCORSAllowedOrigins is CITIZEN_CORS_ALLOWED_ORIGINS, parsed.
func (c Config) CitizenCORSAllowedOrigins() NguonCORS {
	c.require("CitizenCORSAllowedOrigins", CitizenCORS)
	return c.citizenCORSAllowedOrigins
}

// CitizenSessionBridgeListenAddr is CITIZEN_SESSION_BRIDGE_LISTEN_ADDR.
func (c Config) CitizenSessionBridgeListenAddr() string {
	c.require("CitizenSessionBridgeListenAddr", CitizenBridge)
	return c.citizenSessionBridgeListenAddr
}

// CitizenSessionBridgeKeys is CITIZEN_SESSION_BRIDGE_KEYS.
func (c Config) CitizenSessionBridgeKeys() []secret.Secret {
	c.require("CitizenSessionBridgeKeys", CitizenBridge)
	return c.citizenSessionBridgeKeys
}

// CitizenSessionTTL is CITIZEN_SESSION_TTL, default 720h.
func (c Config) CitizenSessionTTL() time.Duration {
	c.require("CitizenSessionTTL", CitizenBridge)
	return c.citizenSessionTTL
}

// IdentityAdminSeedPassword is IDENTITY_ADMIN_SEED_PASSWORD; empty = seeding off.
func (c Config) IdentityAdminSeedPassword() secret.Secret {
	c.require("IdentityAdminSeedPassword", AdminSeed)
	return c.identityAdminSeedPassword
}

// ObjectStorage is the ADR 0052 connection for core/storage.New. PublicMediaBaseURL is filled
// only when PublicMedia is declared too; otherwise storage.PublicURL refuses (ErrNotConfigured).
func (c Config) ObjectStorage() ObjectStorage {
	c.require("ObjectStorage", ObjectStore, PublicMedia)
	return c.objectStorage
}

// MalwareScanner is the clamd connection for core/malwarescan.New.
func (c Config) MalwareScanner() MalwareScanner {
	c.require("MalwareScanner", MalwareScan)
	return c.malwareScanner
}

// SecretEncryptionKeys are the decoded KEKs of SECRET_ENCRYPTION_KEYS, first entry wraps.
func (c Config) SecretEncryptionKeys() []secret.Secret {
	c.require("SecretEncryptionKeys", SecretEncryption)
	return c.secretEncryptionKeys
}

// OperatorSessionSigningKeys is OPERATOR_SESSION_SIGNING_KEYS, first entry signs. Identity
// (OperatorRealm) signs with it; platform's operator edge (OperatorEdge) only verifies.
func (c Config) OperatorSessionSigningKeys() []secret.Secret {
	c.require("OperatorSessionSigningKeys", OperatorRealm, OperatorEdge)
	return c.operatorSessionSigningKeys
}

// OperatorHost is OPERATOR_HOST, validated; "" means the operator area is OFF and every operator
// route answers 404 (ADR 0048 #2 + #4).
func (c Config) OperatorHost() string {
	c.require("OperatorHost", OperatorEdge)
	return c.operatorHost
}

// OperatorGRPCListenAddr is OPERATOR_GRPC_LISTEN_ADDR, default ":9093" — identity's OperatorService
// listener, with nothing else registered on it.
func (c Config) OperatorGRPCListenAddr() string {
	c.require("OperatorGRPCListenAddr", OperatorRealm)
	return c.operatorGRPCListenAddr
}

// IdentityOperatorGRPCAddr is IDENTITY_OPERATOR_GRPC_ADDR — where platform dials OperatorService.
func (c Config) IdentityOperatorGRPCAddr() string {
	c.require("IdentityOperatorGRPCAddr", OperatorEdge)
	return c.identityOperatorGRPCAddr
}

// OperatorTOTPEncryptionKeys are the decoded keys of OPERATOR_TOTP_ENCRYPTION_KEY.
func (c Config) OperatorTOTPEncryptionKeys() []secret.Secret {
	c.require("OperatorTOTPEncryptionKeys", OperatorRealm)
	return c.operatorTOTPEncryptionKeys
}

// RabbitMQDSN is RABBITMQ_DSN.
func (c Config) RabbitMQDSN() secret.DSN {
	c.require("RabbitMQDSN", RabbitMQ)
	return c.rabbitMQDSN
}

// RabbitMQExchange is RABBITMQ_EXCHANGE.
func (c Config) RabbitMQExchange() string {
	c.require("RabbitMQExchange", RabbitMQ)
	return c.rabbitMQExchange
}

// ElasticsearchAddrs is ELASTICSEARCH_ADDRS, every node kept.
func (c Config) ElasticsearchAddrs() []string {
	c.require("ElasticsearchAddrs", Elasticsearch)
	return c.elasticsearchAddrs
}

// ElasticsearchAPIKey is ELASTICSEARCH_API_KEY.
func (c Config) ElasticsearchAPIKey() secret.Secret {
	c.require("ElasticsearchAPIKey", Elasticsearch)
	return c.elasticsearchAPIKey
}

// ElasticsearchIndexPrefix is ELASTICSEARCH_INDEX_PREFIX.
func (c Config) ElasticsearchIndexPrefix() string {
	c.require("ElasticsearchIndexPrefix", Elasticsearch)
	return c.elasticsearchIndexPrefix
}

// ---- rendering --------------------------------------------------------------------------

// configView is Config with every field exported, for printing only.
//
// WHY IT EXISTS: fmt and slog never call String/Format/LogValue on an UNEXPORTED field — they
// cannot reach it as an interface — so they print it by reflection. For a secret.Secret, a
// []byte, that is the key material itself, byte by byte. The moment the grouped fields became
// unexported, one `slog.Info("boot", "cfg", cfg)` would have printed every key on the
// deployment. Rendering this view instead keeps every value inside its own redacting type.
type configView struct {
	Service                        string
	Groups                         []string
	Env                            string
	DatabaseDSN                    secret.DSN
	DangerousAuthBypass            bool
	ListenAddr                     string
	TrustedProxies                 []netip.Prefix
	GRPCListenAddr                 string
	GRPCCallerKey                  secret.Secret
	PlatformGRPCAddr               string
	IdentityGRPCAddr               string
	PetitionsGRPCAddr              string
	DocumentsGRPCAddr              string
	CommsGRPCAddr                  string
	TenantCacheTTL                 time.Duration
	RedisDSN                       secret.DSN
	SessionSigningKeys             []secret.Secret
	CitizenCORSOriginCount         int // the parsed patterns have no exported fields to print
	CitizenSessionBridgeListenAddr string
	CitizenSessionBridgeKeys       []secret.Secret
	CitizenSessionTTL              time.Duration
	IdentityAdminSeedPassword      secret.Secret
	ObjectStorage                  ObjectStorage
	MalwareScanner                 MalwareScanner
	SecretEncryptionKeys           []secret.Secret
	OperatorSessionSigningKeys     []secret.Secret
	OperatorTOTPEncryptionKeys     []secret.Secret
	OperatorHost                   string
	OperatorGRPCListenAddr         string
	IdentityOperatorGRPCAddr       string
	RabbitMQDSN                    secret.DSN
	RabbitMQExchange               string
	ElasticsearchAddrs             []string
	ElasticsearchAPIKey            secret.Secret
	ElasticsearchIndexPrefix       string
}

func (c Config) view() configView {
	var groups []string
	for _, g := range c.uses.Groups() {
		groups = append(groups, g.String())
	}
	return configView{
		Service:                        c.service,
		Groups:                         groups,
		Env:                            c.Env,
		DatabaseDSN:                    c.DatabaseDSN,
		DangerousAuthBypass:            c.DangerousAuthBypass,
		ListenAddr:                     c.listenAddr,
		TrustedProxies:                 c.trustedProxies,
		GRPCListenAddr:                 c.grpcListenAddr,
		GRPCCallerKey:                  c.grpcCallerKey,
		PlatformGRPCAddr:               c.platformGRPCAddr,
		IdentityGRPCAddr:               c.identityGRPCAddr,
		PetitionsGRPCAddr:              c.petitionsGRPCAddr,
		DocumentsGRPCAddr:              c.documentsGRPCAddr,
		CommsGRPCAddr:                  c.commsGRPCAddr,
		TenantCacheTTL:                 c.tenantCacheTTL,
		RedisDSN:                       c.redisDSN,
		SessionSigningKeys:             c.sessionSigningKeys,
		CitizenCORSOriginCount:         len(c.citizenCORSAllowedOrigins),
		CitizenSessionBridgeListenAddr: c.citizenSessionBridgeListenAddr,
		CitizenSessionBridgeKeys:       c.citizenSessionBridgeKeys,
		CitizenSessionTTL:              c.citizenSessionTTL,
		IdentityAdminSeedPassword:      c.identityAdminSeedPassword,
		ObjectStorage:                  c.objectStorage,
		MalwareScanner:                 c.malwareScanner,
		SecretEncryptionKeys:           c.secretEncryptionKeys,
		OperatorSessionSigningKeys:     c.operatorSessionSigningKeys,
		OperatorTOTPEncryptionKeys:     c.operatorTOTPEncryptionKeys,
		OperatorHost:                   c.operatorHost,
		OperatorGRPCListenAddr:         c.operatorGRPCListenAddr,
		IdentityOperatorGRPCAddr:       c.identityOperatorGRPCAddr,
		RabbitMQDSN:                    c.rabbitMQDSN,
		RabbitMQExchange:               c.rabbitMQExchange,
		ElasticsearchAddrs:             c.elasticsearchAddrs,
		ElasticsearchAPIKey:            c.elasticsearchAPIKey,
		ElasticsearchIndexPrefix:       c.elasticsearchIndexPrefix,
	}
}

// Format renders the view, for every verb and flag fmt was given.
func (c Config) Format(f fmt.State, verb rune) {
	fmt.Fprintf(f, fmt.FormatString(f, verb), c.view())
}

// LogValue renders the view for slog, text and JSON handlers alike.
func (c Config) LogValue() slog.Value { return slog.AnyValue(c.view()) }

// MarshalJSON renders the view; json.Marshal would otherwise drop every grouped field silently.
func (c Config) MarshalJSON() ([]byte, error) { return json.Marshal(c.view()) }
