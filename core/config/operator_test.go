package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// Fake key material — the text says so (rule 8, forbidden #1).
const (
	operatorSigningKeyFake = "operator-signing-key-FAKE-NOT-A-REAL-KEY-for-tests"
	operatorSigningKeyOld  = "operator-signing-key-OLD-FAKE-NOT-A-REAL-KEY-tests"
)

// totpKeyFake is base64 of 32 fake bytes: "totp-key-FAKE-NOT-REAL-32-bytes!".
var totpKeyFake = base64.StdEncoding.EncodeToString([]byte("totp-key-FAKE-NOT-REAL-32-bytes!"))
var totpKeyOld = base64.StdEncoding.EncodeToString([]byte("totp-key-OLD-FAKE-NOT-REAL-32byt"))

func operatorBase() map[string]string {
	return map[string]string{"DATABASE_DSN": dsnGia, "ENV": EnvDev}
}

func TestOperatorAbsentIsNotAnError(t *testing.T) {
	datMoiTruong(t, operatorBase())
	cfg, err := Load("identity", Uses(OperatorRealm))
	if err != nil {
		t.Fatalf("absent operator variables must not stop the service: %v", err)
	}
	if cfg.OperatorRealmConfigured() {
		t.Fatal("nothing set, yet OperatorRealmConfigured is true")
	}
	for _, w := range cfg.CanhBao() {
		if strings.Contains(w, "OPERATOR_") {
			t.Fatalf("neither variable set must stay silent, got %q", w)
		}
	}
}

func TestOperatorBothSetLoadsInOrder(t *testing.T) {
	env := operatorBase()
	env["OPERATOR_SESSION_SIGNING_KEYS"] = " " + operatorSigningKeyFake + " , " + operatorSigningKeyOld + ","
	env["OPERATOR_TOTP_ENCRYPTION_KEY"] = totpKeyFake + "," + totpKeyOld
	datMoiTruong(t, env)

	cfg, err := Load("identity", Uses(OperatorRealm))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.OperatorRealmConfigured() {
		t.Fatal("both set, yet OperatorRealmConfigured is false")
	}
	if len(cfg.OperatorSessionSigningKeys()) != 2 ||
		string(cfg.OperatorSessionSigningKeys()[0].Lo()) != operatorSigningKeyFake {
		t.Fatal("signing keys: order or trimming lost — the FIRST entry must be the one that signs")
	}
	if len(cfg.OperatorTOTPEncryptionKeys()) != 2 ||
		string(cfg.OperatorTOTPEncryptionKeys()[0].Lo()) != "totp-key-FAKE-NOT-REAL-32-bytes!" {
		t.Fatal("TOTP keys must be held DECODED, in order")
	}
}

func TestOperatorHalfConfiguredIsReported(t *testing.T) {
	for _, tc := range []struct{ set, value, missing string }{
		{"OPERATOR_SESSION_SIGNING_KEYS", operatorSigningKeyFake, "OPERATOR_TOTP_ENCRYPTION_KEY"},
		{"OPERATOR_TOTP_ENCRYPTION_KEY", totpKeyFake, "OPERATOR_SESSION_SIGNING_KEYS"},
	} {
		t.Run(tc.set, func(t *testing.T) {
			env := operatorBase()
			env[tc.set] = tc.value
			datMoiTruong(t, env)
			cfg, err := Load("identity", Uses(OperatorRealm))
			if err != nil {
				t.Fatalf("half-configured is a warning, not a refusal: %v", err)
			}
			if cfg.OperatorRealmConfigured() {
				t.Fatal("half-configured must not count as configured")
			}
			found := false
			for _, w := range cfg.CanhBao() {
				if strings.Contains(w, tc.missing) {
					found = true
				}
			}
			if !found {
				t.Fatalf("CanhBao must name the missing %s", tc.missing)
			}
		})
	}
}

func TestOperatorSigningKeySharedWithStaffIsRefused(t *testing.T) {
	env := operatorBase()
	env["SESSION_SIGNING_KEYS"] = khoaGia
	env["OPERATOR_SESSION_SIGNING_KEYS"] = operatorSigningKeyFake + "," + khoaGia
	datMoiTruong(t, env)

	_, err := Load("identity", Uses(OperatorRealm))
	if !errors.Is(err, ErrOperatorSigningKeysInvalid) {
		t.Fatalf("a key shared between realms must be refused, got %v", err)
	}
	if !strings.Contains(err.Error(), "entry 2") {
		t.Fatalf("the error must point at the entry: %v", err)
	}
	if strings.Contains(err.Error(), khoaGia) {
		t.Fatal("the error printed key material (rule 8)")
	}
}

func TestOperatorTOTPKeyMalformedIsRefused(t *testing.T) {
	short := base64.StdEncoding.EncodeToString([]byte("only-sixteen-byt"))
	long := base64.StdEncoding.EncodeToString([]byte("thirty-three-bytes-FAKE-NOT-REAL!"))
	for name, raw := range map[string]string{
		"not base64":  "not*base64*at*all",
		"raw 32 text": "totp-key-FAKE-NOT-REAL-32-bytes!", // pasted undecoded
		"16 bytes":    short,
		"33 bytes":    long,
		"duplicate":   totpKeyFake + "," + totpKeyFake,
	} {
		t.Run(name, func(t *testing.T) {
			env := operatorBase()
			env["OPERATOR_TOTP_ENCRYPTION_KEY"] = raw
			datMoiTruong(t, env)
			_, err := Load("identity", Uses(OperatorRealm))
			if !errors.Is(err, ErrOperatorTOTPKeyInvalid) {
				t.Fatalf("want ErrOperatorTOTPKeyInvalid, got %v", err)
			}
			if strings.Contains(err.Error(), raw) {
				t.Fatal("the error printed key material (rule 8)")
			}
		})
	}
}

func TestOperatorKeysNeverRender(t *testing.T) {
	env := operatorBase()
	env["OPERATOR_SESSION_SIGNING_KEYS"] = operatorSigningKeyFake
	env["OPERATOR_TOTP_ENCRYPTION_KEY"] = totpKeyFake
	datMoiTruong(t, env)
	cfg, err := Load("identity", Uses(OperatorRealm))
	if err != nil {
		t.Fatal(err)
	}
	for _, verb := range []string{"%v", "%+v", "%#v", "%s", "%d", "%x"} {
		out := fmt.Sprintf(verb, cfg)
		if strings.Contains(out, operatorSigningKeyFake) || strings.Contains(out, "totp-key-FAKE") {
			t.Fatalf("%s printed operator key material", verb)
		}
	}
}
