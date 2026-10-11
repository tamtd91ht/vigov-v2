package config

// config.Basemap — BASEMAP_URL (ADR 0072 §"Sửa đổi 11/10/2026 (Mini App)" M1).

import (
	"errors"
	"strings"
	"testing"
)

// Optional even in prod: the archive waits for a signed sovereignty check, and comms must start without it.
func TestBasemapURLOptionalInProd(t *testing.T) {
	clean(t, EnvProd, nil)
	cfg, err := Load("comms", Uses(Basemap))
	if err != nil {
		t.Fatalf("prod refused without BASEMAP_URL: %v", err)
	}
	if cfg.BasemapURL() != "" {
		t.Fatal("empty must read as off, never as a guess")
	}
}

func TestBasemapURLAcceptedAndNormalised(t *testing.T) {
	for raw, want := range map[string]string{
		"https://minio.internal:9000/vigov-prod-public/config/map/20261011":     "https://minio.internal:9000/vigov-prod-public/config/map/20261011/",
		" https://minio.internal:9000/vigov-prod-public/config/map/20261011/\n": "https://minio.internal:9000/vigov-prod-public/config/map/20261011/",
		"https://minio.internal": "https://minio.internal/",
	} {
		clean(t, EnvProd, map[string]string{"BASEMAP_URL": raw})
		cfg, err := Load("comms", Uses(Basemap))
		if err != nil {
			t.Errorf("%q refused: %v", raw, err)
			continue
		}
		if got := cfg.BasemapURL(); got != want {
			t.Errorf("%q loaded as %q, want %q", raw, got, want)
		}
	}
}

// Malformed is FATAL in every environment, and the refusal never echoes the value (it may hold a password).
func TestBasemapURLRefused(t *testing.T) {
	for _, raw := range []string{
		"http://minio.internal:9000/p",                  // plain http
		"https://user:not-a-real-pass@minio.internal/p", // credentials
		"https://minio.internal/p?X-Amz-Signature=abc",  // query
		"https://minio.internal/p?",                     // empty query
		"https://minio.internal/p#frag",                 // fragment
		"minio.internal/p",                              // no scheme
		"https:///p",                                    // no host
		"https://%zz",                                   // not a URL
	} {
		clean(t, EnvDev, map[string]string{"BASEMAP_URL": raw})
		_, err := Load("comms", Uses(Basemap))
		if !errors.Is(err, ErrBasemapURLInvalid) {
			t.Errorf("%q: want ErrBasemapURLInvalid, got %v", raw, err)
			continue
		}
		if strings.Contains(err.Error(), "not-a-real-pass") || strings.Contains(err.Error(), "X-Amz") {
			t.Errorf("%q: the refusal echoes the value: %v", raw, err)
		}
		if !strings.Contains(err.Error(), "comms") {
			t.Errorf("%q: the refusal does not name the service: %v", raw, err)
		}
	}
}

// Undeclared: a malformed shared value stops nobody else, and nobody else can read it.
func TestBasemapURLIgnoredWhenUndeclared(t *testing.T) {
	clean(t, EnvProd, map[string]string{"BASEMAP_URL": "http://x", "TRUSTED_PROXY_CIDRS": "10.42.0.0/16"})
	cfg, err := Load("platform", Uses(HTTPServer))
	if err != nil {
		t.Fatalf("an undeclared BASEMAP_URL stopped platform: %v", err)
	}
	defer func() {
		if recover() == nil {
			t.Error("platform read BASEMAP_URL without declaring Basemap")
		}
	}()
	_ = cfg.BasemapURL()
}
