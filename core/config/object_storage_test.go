package config

// Object storage settings (ADR 0052). Helpers datMoiTruong and nenChay come from config_test.go
// and ha_tang_test.go — same package.

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// Fake credentials, labelled as such (rule 8, forbidden #1).
const (
	fakeAccessKey = "FAKE-ACCESS-KEY-NOT-REAL"
	fakeSecretKey = "fake-secret-key-NOT-A-REAL-SECRET"
)

var objectStorageVars = []string{
	"OBJECT_STORAGE_ENDPOINT", "OBJECT_STORAGE_PUBLIC_ENDPOINT", "OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL",
	"OBJECT_STORAGE_ACCESS_KEY", "OBJECT_STORAGE_SECRET_KEY", "OBJECT_STORAGE_REGION",
	"OBJECT_STORAGE_BUCKET_PREFIX",
	// Not object storage, but CanhBao pairs the two (malware_scanner.go): forced empty too so the
	// machine's own environment cannot decide whether a warning appears.
	"MALWARE_SCANNER_ADDRESS",
}

// withObjectStorage sets the baseline plus the given object-storage values; every other
// object-storage variable is forced empty so the machine's own environment cannot leak in.
func withObjectStorage(t *testing.T, env string, vals map[string]string) {
	t.Helper()
	m := nenChay(env)
	for _, k := range objectStorageVars {
		m[k] = ""
	}
	for k, v := range vals {
		m[k] = v
	}
	datMoiTruong(t, m)
}

func fullObjectStorage() map[string]string {
	return map[string]string{
		"OBJECT_STORAGE_ENDPOINT":              "http://minio.internal.example:9000",
		"OBJECT_STORAGE_PUBLIC_ENDPOINT":       "https://files.example.test",
		"OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL": "https://cdn.example.test/vigov-test-public/",
		"OBJECT_STORAGE_ACCESS_KEY":            fakeAccessKey,
		"OBJECT_STORAGE_SECRET_KEY":            fakeSecretKey + "\n",
		"OBJECT_STORAGE_BUCKET_PREFIX":         "vigov-test",
		// A complete upload configuration includes the scanner (ADR 0052 §9).
		"MALWARE_SCANNER_ADDRESS": "clamd.internal.example:3310",
	}
}

// uploadUses is what a service storing files declares, plus PublicMedia so the media base URL is
// read too (only a publisher declares that one).
var uploadUses = Uses(ObjectStore, PublicMedia, MalwareScan)

// Declared and absent starts ONLY in dev — staging and prod refuse it (uses_test.go). Undeclared,
// it starts silently everywhere, which is every service but petitions.
func TestObjectStorageAbsentIsNotConfiguredAndStarts(t *testing.T) {
	withObjectStorage(t, EnvDev, nil)
	cfg, err := Load("petitions", uploadUses)
	if err != nil {
		t.Fatalf("absent object storage must not stop the service in dev: %v", err)
	}
	if cfg.ObjectStorage().Configured() {
		t.Fatal("Configured() = true with nothing set")
	}
	if len(cfg.ObjectStorage().Missing()) != 5 {
		t.Errorf("Missing() = %v", cfg.ObjectStorage().Missing())
	}
	for _, w := range cfg.CanhBao() {
		if strings.Contains(w, "OBJECT_STORAGE") {
			t.Errorf("nothing set must be silent, got %q", w)
		}
	}

	for _, env := range []string{EnvDev, EnvStaging, EnvProd} {
		t.Run("undeclared "+env, func(t *testing.T) {
			withObjectStorage(t, env, nil)
			cfg, err := Load("platform", Uses())
			if err != nil {
				t.Fatalf("undeclared object storage must not stop the service: %v", err)
			}
			for _, w := range cfg.CanhBao() {
				if strings.Contains(w, "OBJECT_STORAGE") {
					t.Errorf("undeclared must be silent, got %q", w)
				}
			}
		})
	}
}

func TestObjectStorageFullLoads(t *testing.T) {
	withObjectStorage(t, EnvProd, fullObjectStorage())
	cfg, err := Load("petitions", uploadUses)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	o := cfg.ObjectStorage()
	if !o.Configured() {
		t.Fatalf("Configured() = false, missing %v", o.Missing())
	}
	if o.Endpoint != "http://minio.internal.example:9000" || o.PublicEndpoint != "https://files.example.test" {
		t.Errorf("endpoints = %q, %q", o.Endpoint, o.PublicEndpoint)
	}
	if o.PublicMediaBaseURL != "https://cdn.example.test/vigov-test-public" {
		t.Errorf("media base = %q (trailing slash must go)", o.PublicMediaBaseURL)
	}
	if o.Region != ObjectStorageDefaultRegion {
		t.Errorf("region = %q, want default", o.Region)
	}
	if string(o.SecretKey.Lo()) != fakeSecretKey {
		t.Error("secret key not trimmed")
	}
	for _, w := range cfg.CanhBao() {
		if strings.Contains(w, "OBJECT_STORAGE") {
			t.Errorf("complete https config must be silent, got %q", w)
		}
	}
}

func TestObjectStorageSecretsNeverRender(t *testing.T) {
	withObjectStorage(t, EnvDev, fullObjectStorage())
	cfg, err := Load("petitions", uploadUses)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, out := range []string{
		cfg.ObjectStorage().String(),
		fmt.Sprintf("%v", cfg.ObjectStorage()),
		fmt.Sprintf("%+v", cfg),
		fmt.Sprintf("%#v", cfg),
	} {
		if strings.Contains(out, fakeSecretKey) || strings.Contains(out, fakeAccessKey) {
			t.Fatalf("credential rendered: %s", out)
		}
	}
}

func TestObjectStorageMultipleHostsRefused(t *testing.T) {
	for _, name := range []string{"OBJECT_STORAGE_ENDPOINT", "OBJECT_STORAGE_PUBLIC_ENDPOINT"} {
		t.Run(name, func(t *testing.T) {
			vals := fullObjectStorage()
			vals[name] = "https://minio-a.example:9000,https://minio-b.example:9000"
			withObjectStorage(t, EnvDev, vals)
			_, err := Load("petitions", uploadUses)
			if !errors.Is(err, ErrObjectStorageInvalid) {
				t.Fatalf("err = %v, want ErrObjectStorageInvalid", err)
			}
			if !strings.Contains(err.Error(), name) || !strings.Contains(err.Error(), "ONE endpoint") {
				t.Errorf("error must name the variable and the rule: %v", err)
			}
		})
	}
}

func TestObjectStorageMalformedRefused(t *testing.T) {
	cases := map[string][2]string{
		"no scheme":        {"OBJECT_STORAGE_ENDPOINT", "minio.internal:9000"},
		"ftp scheme":       {"OBJECT_STORAGE_ENDPOINT", "ftp://minio.internal:9000"},
		"path":             {"OBJECT_STORAGE_PUBLIC_ENDPOINT", "https://files.example.test/s3"},
		"credentials":      {"OBJECT_STORAGE_ENDPOINT", "https://user:pw@minio.internal:9000"},
		"query":            {"OBJECT_STORAGE_ENDPOINT", "https://minio.internal:9000?x=1"},
		"prefix uppercase": {"OBJECT_STORAGE_BUCKET_PREFIX", "Vigov-Prod"},
		"prefix colon":     {"OBJECT_STORAGE_BUCKET_PREFIX", "vigov:prod"},
		"prefix too long":  {"OBJECT_STORAGE_BUCKET_PREFIX", strings.Repeat("a", 56)},
		"media list":       {"OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL", "https://a.test,https://b.test"},
		"media query":      {"OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL", "https://a.test/x?y=1"},
	}
	for label, c := range cases {
		t.Run(label, func(t *testing.T) {
			vals := fullObjectStorage()
			vals[c[0]] = c[1]
			withObjectStorage(t, EnvDev, vals)
			if _, err := Load("petitions", uploadUses); !errors.Is(err, ErrObjectStorageInvalid) {
				t.Fatalf("err = %v, want ErrObjectStorageInvalid", err)
			}
		})
	}
}

func TestObjectStorageHalfConfiguredWarns(t *testing.T) {
	withObjectStorage(t, EnvDev, map[string]string{
		"OBJECT_STORAGE_ENDPOINT":   "http://minio.internal.example:9000",
		"OBJECT_STORAGE_ACCESS_KEY": fakeAccessKey,
	})
	cfg, err := Load("petitions", uploadUses)
	if err != nil {
		t.Fatalf("half configuration must not stop the service: %v", err)
	}
	var found string
	for _, w := range cfg.CanhBao() {
		if strings.Contains(w, "OBJECT_STORAGE_") {
			found = w
		}
	}
	for _, want := range []string{"OBJECT_STORAGE_PUBLIC_ENDPOINT", "OBJECT_STORAGE_SECRET_KEY", "OBJECT_STORAGE_BUCKET_PREFIX"} {
		if !strings.Contains(found, want) {
			t.Errorf("warning %q does not name %s", found, want)
		}
	}
}

func TestObjectStoragePlainHTTPPublicEndpointWarnsOutsideDev(t *testing.T) {
	vals := fullObjectStorage()
	vals["OBJECT_STORAGE_PUBLIC_ENDPOINT"] = "http://files.example.test"
	withObjectStorage(t, EnvStaging, vals)
	cfg, err := Load("petitions", uploadUses)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !strings.Contains(strings.Join(cfg.CanhBao(), "\n"), "OBJECT_STORAGE_PUBLIC_ENDPOINT") {
		t.Error("http public endpoint outside dev must be reported")
	}
}

func TestObjectStorageRegionOverride(t *testing.T) {
	vals := fullObjectStorage()
	vals["OBJECT_STORAGE_REGION"] = " ap-southeast-1 "
	withObjectStorage(t, EnvDev, vals)
	cfg, err := Load("petitions", uploadUses)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ObjectStorage().Region != "ap-southeast-1" {
		t.Errorf("region = %q", cfg.ObjectStorage().Region)
	}
}
