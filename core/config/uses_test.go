package config

// config.Uses — each service loads only the groups it declares (owner's decision, 2026-09-29).

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// allGroupVars is every variable read by a group, forced empty by clean so the machine's own
// environment cannot decide a result.
var allGroupVars = []string{
	"LISTEN_ADDR", "TRUSTED_PROXY_CIDRS", "GRPC_LISTEN_ADDR", "GRPC_CALLER_KEY",
	"PLATFORM_GRPC_ADDR", "IDENTITY_GRPC_ADDR", "PETITIONS_GRPC_ADDR", "DOCUMENTS_GRPC_ADDR",
	"COMMS_GRPC_ADDR", "TENANT_CACHE_TTL", "REDIS_DSN", "SESSION_SIGNING_KEYS", "CITIZEN_CORS_ALLOWED_ORIGINS",
	"CITIZEN_SESSION_BRIDGE_LISTEN_ADDR", "CITIZEN_SESSION_BRIDGE_KEYS", "CITIZEN_SESSION_TTL",
	"IDENTITY_ADMIN_SEED_PASSWORD",
	"OBJECT_STORAGE_ENDPOINT", "OBJECT_STORAGE_PUBLIC_ENDPOINT", "OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL",
	"OBJECT_STORAGE_ACCESS_KEY", "OBJECT_STORAGE_SECRET_KEY", "OBJECT_STORAGE_REGION",
	"OBJECT_STORAGE_BUCKET_PREFIX", "MALWARE_SCANNER_ADDRESS", "SECRET_ENCRYPTION_KEYS",
	"OPERATOR_SESSION_SIGNING_KEYS", "OPERATOR_TOTP_ENCRYPTION_KEY", "OPERATOR_HOST",
	"RABBITMQ_DSN", "RABBITMQ_EXCHANGE", "ELASTICSEARCH_ADDRS", "ELASTICSEARCH_API_KEY",
	"ELASTICSEARCH_INDEX_PREFIX", "DANGEROUS_AUTH_BYPASS",
}

func clean(t *testing.T, env string, set map[string]string) {
	t.Helper()
	for _, k := range allGroupVars {
		t.Setenv(k, "")
	}
	t.Setenv("DATABASE_DSN", dsnGia)
	t.Setenv("ENV", env)
	for k, v := range set {
		t.Setenv(k, v)
	}
}

// prodRequired is the decision per group, written out as the specification: what staging and
// prod refuse when empty. A variable NOT listed for its group has a stated default or is off by
// design (the reason sits beside its r.read call in Load).
var prodRequired = map[Group][]string{
	HTTPServer:          {"TRUSTED_PROXY_CIDRS"},
	GRPCServer:          {"GRPC_CALLER_KEY"},
	PlatformClient:      {"GRPC_CALLER_KEY", "PLATFORM_GRPC_ADDR"},
	IdentityClient:      {"GRPC_CALLER_KEY", "IDENTITY_GRPC_ADDR"},
	OrgUnitOwnerClients: {"GRPC_CALLER_KEY", "PETITIONS_GRPC_ADDR", "DOCUMENTS_GRPC_ADDR"},
	TenantCache:         nil,
	Redis:               {"REDIS_DSN"},
	StaffSessionSigning: {"SESSION_SIGNING_KEYS"},
	CitizenCORS:         {"CITIZEN_CORS_ALLOWED_ORIGINS"},
	CitizenBridge:       {"CITIZEN_SESSION_BRIDGE_LISTEN_ADDR", "CITIZEN_SESSION_BRIDGE_KEYS"},
	AdminSeed:           nil,
	ObjectStore: {"OBJECT_STORAGE_ENDPOINT", "OBJECT_STORAGE_PUBLIC_ENDPOINT", "OBJECT_STORAGE_ACCESS_KEY",
		"OBJECT_STORAGE_SECRET_KEY", "OBJECT_STORAGE_BUCKET_PREFIX"},
	PublicMedia:      {"OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL"},
	MalwareScan:      {"MALWARE_SCANNER_ADDRESS"},
	SecretEncryption: {"SECRET_ENCRYPTION_KEYS"},
	OperatorRealm:    {"OPERATOR_SESSION_SIGNING_KEYS", "OPERATOR_TOTP_ENCRYPTION_KEY"},
	RabbitMQ:         {"RABBITMQ_DSN", "RABBITMQ_EXCHANGE"},
	Elasticsearch:    {"ELASTICSEARCH_ADDRS", "ELASTICSEARCH_API_KEY", "ELASTICSEARCH_INDEX_PREFIX"},
	CommsClient:      {"GRPC_CALLER_KEY", "COMMS_GRPC_ADDR"},
	// OPERATOR_HOST is deliberately absent: it is the feature switch (ADR 0048 #2 + #4).
	OperatorEdge: {"OPERATOR_SESSION_SIGNING_KEYS"},
}

func TestEveryGroupHasAProdDecision(t *testing.T) {
	for g := Group(1); g < groupEnd; g++ {
		if _, ok := prodRequired[g]; !ok {
			t.Errorf("%s has no entry in prodRequired — decide which of its variables prod requires", g)
		}
	}
}

// Declared + missing in staging/prod: refused, and EVERY missing variable is named. Staging is
// held to prod's rule so the refusal is met there first.
func TestDeclaredMissingInProdIsRefusedByName(t *testing.T) {
	for g, want := range prodRequired {
		for _, env := range []string{EnvStaging, EnvProd} {
			t.Run(g.String()+"/"+env, func(t *testing.T) {
				clean(t, env, nil)
				_, err := Load("svc-test", Uses(g))
				if len(want) == 0 {
					if err != nil {
						t.Fatalf("%s has no prod-required variable, yet Load refused: %v", g, err)
					}
					return
				}
				if !errors.Is(err, ErrThieuBienMoiTruong) {
					t.Fatalf("want ErrThieuBienMoiTruong, got %v", err)
				}
				for _, v := range want {
					if !strings.Contains(err.Error(), v) {
						t.Errorf("refusal does not name %s: %v", v, err)
					}
				}
				if !strings.Contains(err.Error(), "svc-test") {
					t.Errorf("refusal does not name the service: %v", err)
				}
			})
		}
	}
}

// Variables with a stated default, or off by design, are never named by the refusal.
func TestDefaultsAreNotRequiredInProd(t *testing.T) {
	clean(t, EnvProd, nil)
	_, err := Load("svc-test", Uses(HTTPServer, GRPCServer, TenantCache, CitizenBridge, AdminSeed, ObjectStore))
	if err == nil {
		t.Fatal("expected a refusal for the required variables")
	}
	named := map[string]bool{}
	msg := err.Error()
	list := msg[:strings.Index(msg, " (service")]
	for _, n := range strings.Split(list[strings.LastIndex(list, ": ")+2:], ", ") {
		named[n] = true
	}
	for _, v := range []string{"LISTEN_ADDR", "GRPC_LISTEN_ADDR", "TENANT_CACHE_TTL", "CITIZEN_SESSION_TTL",
		"IDENTITY_ADMIN_SEED_PASSWORD", "OBJECT_STORAGE_REGION"} {
		if named[v] {
			t.Errorf("%s has a default / is off by design, yet the refusal names it: %v", v, err)
		}
	}
	if !named["TRUSTED_PROXY_CIDRS"] {
		t.Errorf("the parse of the refusal is broken — TRUSTED_PROXY_CIDRS is required: %v", err)
	}
}

// Declared + missing in dev: allowed (feature off, fail closed), except GRPC_CALLER_KEY, which ADR
// 0025 requires in every environment.
func TestDeclaredMissingInDevIsAllowed(t *testing.T) {
	var all []Group
	for g := Group(1); g < groupEnd; g++ {
		all = append(all, g)
	}
	clean(t, EnvDev, map[string]string{"GRPC_CALLER_KEY": khoaGoiNoiBoGia})
	cfg, err := Load("svc-test", Uses(all...))
	if err != nil {
		t.Fatalf("dev with every group declared and empty must start: %v", err)
	}
	if cfg.RedisDSN() != "" || cfg.ObjectStorage().Configured() || cfg.CauPhienBat() {
		t.Error("an empty declared variable must read as off, never as a guess")
	}

	clean(t, EnvDev, nil)
	_, err = Load("svc-test", Uses(GRPCServer))
	if !errors.Is(err, ErrThieuBienMoiTruong) || !strings.Contains(err.Error(), "GRPC_CALLER_KEY") {
		t.Errorf("GRPC_CALLER_KEY must be refused in dev too, got %v", err)
	}
}

// An UNDECLARED group is not parsed: a petitions-only malformed value in the shared ConfigMap
// must not stop platform, and must not make platform warn.
func TestUndeclaredMalformedValueIsIgnored(t *testing.T) {
	clean(t, EnvProd, map[string]string{
		"TRUSTED_PROXY_CIDRS":          "10.42.0.0/16",
		"GRPC_CALLER_KEY":              khoaGoiNoiBoGia,
		"OBJECT_STORAGE_ENDPOINT":      "https://a.example:9000,https://b.example:9000", // refused if read
		"OBJECT_STORAGE_ACCESS_KEY":    fakeAccessKey,                                   // half-configured
		"MALWARE_SCANNER_ADDRESS":      "tcp://clamd:3310",
		"SECRET_ENCRYPTION_KEYS":       "not*base64",
		"CITIZEN_CORS_ALLOWED_ORIGINS": "*",
		"CITIZEN_SESSION_BRIDGE_KEYS":  khoaCauGia, // half a bridge
		"CITIZEN_SESSION_TTL":          "30d",
		"OPERATOR_TOTP_ENCRYPTION_KEY": "not*base64",
	})
	cfg, err := Load("platform", Uses(HTTPServer, GRPCServer, TenantCache))
	if err != nil {
		t.Fatalf("an undeclared group's malformed value stopped platform: %v", err)
	}
	if w := cfg.CanhBao(); len(w) != 0 {
		t.Errorf("platform must not warn about groups it does not use: %v", w)
	}

	// ...and the same value DOES stop the service that declares it.
	if _, err := Load("petitions", Uses(ObjectStore)); !errors.Is(err, ErrObjectStorageInvalid) &&
		!errors.Is(err, ErrThieuBienMoiTruong) {
		t.Errorf("declared ObjectStore with a malformed endpoint must be refused, got %v", err)
	}
}

// SESSION_SIGNING_KEYS is identity's alone: a service that does not issue staff sessions starts
// in prod without it; identity does not.
func TestSessionSigningKeysOnlyForIdentity(t *testing.T) {
	documents := map[string]string{
		"TRUSTED_PROXY_CIDRS": "10.42.0.0/16",
		"GRPC_CALLER_KEY":     khoaGoiNoiBoGia,
		"PLATFORM_GRPC_ADDR":  "platform:9090",
		"IDENTITY_GRPC_ADDR":  "identity:9090",
		"REDIS_DSN":           redisGia,
	}
	clean(t, EnvProd, documents)
	if _, err := Load("documents",
		Uses(HTTPServer, GRPCServer, PlatformClient, IdentityClient, TenantCache, Redis)); err != nil {
		t.Fatalf("documents does not sign staff sessions, yet refused without SESSION_SIGNING_KEYS: %v", err)
	}

	clean(t, EnvProd, nil)
	_, err := Load("identity", Uses(StaffSessionSigning))
	if !errors.Is(err, ErrThieuBienMoiTruong) || !strings.Contains(err.Error(), "SESSION_SIGNING_KEYS") {
		t.Errorf("identity without SESSION_SIGNING_KEYS in prod must be refused by name, got %v", err)
	}
}

// Reporting has no gRPC server and no gRPC client, so it never reads GRPC_CALLER_KEY.
func TestServiceWithoutGRPCNeedsNoCallerKey(t *testing.T) {
	clean(t, EnvProd, map[string]string{"TRUSTED_PROXY_CIDRS": "10.42.0.0/16"})
	if _, err := Load("reporting", Uses(HTTPServer)); err != nil {
		t.Fatalf("reporting refused without GRPC_CALLER_KEY: %v", err)
	}
}

// IDENTITY_ADMIN_SEED_PASSWORD is a one-off bootstrap switch, removed once every commune has
// changed its admin password. Declared, it is still never required — not even in prod.
func TestAdminSeedIsNeverRequired(t *testing.T) {
	clean(t, EnvProd, nil)
	cfg, err := Load("identity", Uses(AdminSeed))
	if err != nil {
		t.Fatalf("prod refused without IDENTITY_ADMIN_SEED_PASSWORD: %v", err)
	}
	if !cfg.IdentityAdminSeedPassword().Rong() {
		t.Error("empty must mean off")
	}
}

// DANGEROUS_AUTH_BYPASS is refused in prod whatever the service declared.
func TestDangerousFlagRefusedInProdForEveryDeclaration(t *testing.T) {
	clean(t, EnvProd, map[string]string{"DANGEROUS_AUTH_BYPASS": "true"})
	if _, err := Load("reporting", Uses()); !errors.Is(err, ErrCoBienNguyHiem) {
		t.Errorf("want ErrCoBienNguyHiem, got %v", err)
	}
}

// Reading a value of an undeclared group is loud, never a silent zero.
func TestUndeclaredAccessorPanics(t *testing.T) {
	clean(t, EnvDev, map[string]string{"REDIS_DSN": redisGia})
	cfg, err := Load("platform", Uses(HTTPServer))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		p := recover()
		u, ok := p.(undeclaredRead)
		if !ok {
			t.Fatalf("want an undeclaredRead panic, got %v", p)
		}
		if msg := u.Error(); !strings.Contains(msg, "config.Redis") || !strings.Contains(msg, "platform") {
			t.Errorf("the panic must name the group and the service: %s", msg)
		}
	}()
	_ = cfg.RedisDSN()
}

// unguarded are the zero-argument Config methods that read no group of their own.
var unguarded = map[string]bool{"CanhBao": true, "Redacted": true, "LogValue": true, "MarshalJSON": true}

// Every other zero-argument method must refuse on a Config that declares nothing, and must answer
// for each group it names. An accessor added without require() is red here.
func TestEveryAccessorIsGuarded(t *testing.T) {
	typ := reflect.TypeOf(Config{})
	for i := 0; i < typ.NumMethod(); i++ {
		m := typ.Method(i)
		if m.Type.NumIn() != 1 || unguarded[m.Name] {
			continue
		}
		groups, guarded := accessorGroups(m.Name)
		if !guarded {
			t.Errorf("Config.%s() reads without require() — add its group guard, or list it in unguarded", m.Name)
			continue
		}
		for _, g := range groups {
			func() {
				defer func() {
					if p := recover(); p != nil {
						t.Errorf("Config.%s() names %s but panics when it is declared: %v", m.Name, g, p)
					}
				}()
				reflect.ValueOf(Config{uses: Uses(g)}).MethodByName(m.Name).Call(nil)
			}()
		}
	}
}

func TestUsesRejectsANonGroup(t *testing.T) {
	for _, g := range []Group{0, groupEnd} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("Uses(%d) must panic", g)
				}
			}()
			Uses(g)
		}()
	}
}

// CheckUses must go red on an omission AND on a surplus.
func TestCheckUsesFindsOmissionAndSurplus(t *testing.T) {
	dir := t.TempDir()
	src := "package x\n\nimport \"github.com/vihat/vigov/core/config\"\n\n" +
		"func f(cfg config.Config) {\n\t_ = cfg.RedisDSN()\n\t_ = cfg.GRPCCallerKey()\n\t_ = cfg.Env\n\t_ = cfg.CanhBao()\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	// A _test.go file is not the binary: a read there must not count.
	if err := os.WriteFile(filepath.Join(dir, "x_test.go"),
		[]byte("package x\n\nfunc g(cfg config.Config) { _ = cfg.ObjectStorage() }\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if p, err := CheckUses(dir, Uses(Redis, PlatformClient)); err != nil || len(p) != 0 {
		t.Fatalf("matching declaration: problems %v, err %v", p, err)
	}

	p, err := CheckUses(dir, Uses(PlatformClient))
	if err != nil {
		t.Fatal(err)
	}
	if len(p) != 1 || !strings.Contains(p[0], "RedisDSN") || !strings.Contains(p[0], "config.Redis") {
		t.Errorf("omission not reported by name: %v", p)
	}

	p, err = CheckUses(dir, Uses(Redis, PlatformClient, MalwareScan))
	if err != nil {
		t.Fatal(err)
	}
	if len(p) != 1 || !strings.Contains(p[0], "config.MalwareScan") {
		t.Errorf("surplus not reported: %v", p)
	}
}
