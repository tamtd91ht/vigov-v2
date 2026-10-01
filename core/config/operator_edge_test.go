package config

// config.OperatorEdge — OPERATOR_HOST and the signing keys platform's operator edge verifies with
// (ADR 0048 §"Chốt của chủ dự án — 01/10/2026" #2).

import (
	"errors"
	"strings"
	"testing"
)

// Absent OPERATOR_HOST is the designed OFF state — in prod too. Only the keys are required there.
func TestOperatorHostIsNeverRequired(t *testing.T) {
	clean(t, EnvProd, map[string]string{"OPERATOR_SESSION_SIGNING_KEYS": operatorSigningKeyFake})
	cfg, err := Load("platform", Uses(OperatorEdge))
	if err != nil {
		t.Fatalf("prod refused without OPERATOR_HOST — it is the feature switch, never required: %v", err)
	}
	if cfg.OperatorHost() != "" {
		t.Fatalf("unset must read as off, got %q", cfg.OperatorHost())
	}
}

// The edge needs the signing keys and NOT the TOTP key: declaring OperatorEdge must never demand,
// or even read, the key that decrypts every operator's second factor.
func TestOperatorEdgeDoesNotReadTheTOTPKey(t *testing.T) {
	clean(t, EnvProd, map[string]string{
		"OPERATOR_SESSION_SIGNING_KEYS": operatorSigningKeyFake,
		"OPERATOR_TOTP_ENCRYPTION_KEY":  "not*base64", // fatal if it were read
		"OPERATOR_HOST":                 "admin.vigov.vn",
	})
	cfg, err := Load("platform", Uses(OperatorEdge))
	if err != nil {
		t.Fatalf("OperatorEdge read OPERATOR_TOTP_ENCRYPTION_KEY: %v", err)
	}
	if got := cfg.OperatorHost(); got != "admin.vigov.vn" {
		t.Fatalf("OperatorHost = %q", got)
	}
	if len(cfg.OperatorSessionSigningKeys()) != 1 {
		t.Fatal("the edge must see the signing keys")
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("OperatorTOTPEncryptionKeys must panic on a service that declared only OperatorEdge")
			}
		}()
		_ = cfg.OperatorTOTPEncryptionKeys()
	}()
}

// The prod refusal names the signing keys, and only them.
func TestOperatorEdgeWithoutKeysRefusedInProd(t *testing.T) {
	clean(t, EnvProd, map[string]string{"OPERATOR_HOST": "admin.vigov.vn"})
	_, err := Load("platform", Uses(OperatorEdge))
	if !errors.Is(err, ErrThieuBienMoiTruong) || !strings.Contains(err.Error(), "OPERATOR_SESSION_SIGNING_KEYS") {
		t.Fatalf("want a refusal naming OPERATOR_SESSION_SIGNING_KEYS, got %v", err)
	}
	if first := strings.SplitN(err.Error(), "\n", 2)[0]; strings.Contains(first, "OPERATOR_TOTP") || strings.Contains(first, "OPERATOR_HOST") {
		t.Errorf("the refusal names a variable the edge does not require: %s", first)
	}
}

// The overlap rule holds for the edge too: platform must not accept a staff key as an operator key.
func TestOperatorEdgeSigningKeySharedWithStaffIsRefused(t *testing.T) {
	clean(t, EnvDev, map[string]string{
		"SESSION_SIGNING_KEYS":          khoaGia,
		"OPERATOR_SESSION_SIGNING_KEYS": khoaGia,
	})
	if _, err := Load("platform", Uses(OperatorEdge)); !errors.Is(err, ErrOperatorSigningKeysInvalid) {
		t.Fatalf("want ErrOperatorSigningKeysInvalid, got %v", err)
	}
}

func TestOperatorHostAccepted(t *testing.T) {
	for _, h := range []string{
		"admin.vigov.vn",
		"admin-stg.vigov.vn",
		"admin.stg.vigov.vn",
		" admin.vigov.vn\n",     // trimmed like every variable
		"ops.vihat.example.com", // outside vigov.vn: not judged
	} {
		clean(t, EnvDev, map[string]string{"OPERATOR_HOST": h})
		cfg, err := Load("platform", Uses(OperatorEdge))
		if err != nil {
			t.Errorf("%q refused: %v", h, err)
			continue
		}
		if cfg.OperatorHost() != strings.TrimSpace(h) {
			t.Errorf("%q loaded as %q", h, cfg.OperatorHost())
		}
	}
}

// Every malformed or commune-shaped value is FATAL, never repaired into shape.
func TestOperatorHostRefused(t *testing.T) {
	for _, h := range []string{
		"   ",                       // set but blank
		"https://admin.vigov.vn",    // scheme
		"admin.vigov.vn/van-hanh",   // path
		"admin.vigov.vn:443",        // port
		"Admin.vigov.vn",            // upper-case
		"admin.vigov.vn.",           // trailing dot
		"10.0.0.5",                  // IP literal
		"[::1]",                     // IPv6 literal
		"localhost",                 // not qualified
		"*.vigov.vn",                // wildcard
		"-admin.vigov.vn",           // bad label
		"admin..vigov.vn",           // empty label
		"vigov.vn",                  // the root
		"stg.vigov.vn",              // the staging root
		"thangbinh-danang.vigov.vn", // a commune's web host
		"thangbinh.stg.vigov.vn",    // a commune's staging host
		"platform.api.vigov.vn",     // a service API host
		"www.vigov.vn",              // reserved, but not the operator console
		"api.vigov.vn",
	} {
		clean(t, EnvDev, map[string]string{"OPERATOR_HOST": h})
		_, err := Load("platform", Uses(OperatorEdge))
		if !errors.Is(err, ErrOperatorHostInvalid) {
			t.Errorf("%q: want ErrOperatorHostInvalid, got %v", h, err)
			continue
		}
		if !strings.Contains(err.Error(), "platform") {
			t.Errorf("%q: the refusal does not name the service: %v", h, err)
		}
	}
}

// An undeclared OperatorEdge never parses OPERATOR_HOST: a malformed value in common-config must
// not stop identity, documents, … — the usual "set but unused" state (uses.go).
func TestOperatorHostIgnoredWhenUndeclared(t *testing.T) {
	clean(t, EnvDev, map[string]string{"OPERATOR_HOST": "https://Thang-Binh.vigov.vn:443/"})
	if _, err := Load("identity", Uses(OperatorRealm)); err != nil {
		t.Fatalf("an undeclared OPERATOR_HOST stopped a service: %v", err)
	}
}

// Dev only: the area switched on with nothing to verify a token with is said out loud.
func TestOperatorEdgeHalfConfiguredWarns(t *testing.T) {
	clean(t, EnvDev, map[string]string{"OPERATOR_HOST": "admin.vigov.vn"})
	cfg, err := Load("platform", Uses(OperatorEdge))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, w := range cfg.CanhBao() {
		if strings.Contains(w, "OPERATOR_SESSION_SIGNING_KEYS") {
			found = true
		}
	}
	if !found {
		t.Errorf("OPERATOR_HOST without signing keys must warn, got %v", cfg.CanhBao())
	}
}
