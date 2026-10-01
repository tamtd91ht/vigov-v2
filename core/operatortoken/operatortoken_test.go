package operatortoken

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/secret"
)

// Fake key material — the text says so in full (rule 8, forbidden #1).
var keyFake = secret.Secret("operator-signing-key-FAKE-0123456789abcdef")

// goldenToken is what service-identity/internal/operatorauth.TokenSigner produced for keyFake and
// goldenClaims on 2026-10-01 (run through `go test -overlay` against identity's package, so nothing
// was written into service-identity). THIS IS THE CROSS-SERVICE CONTRACT IN ONE STRING: if this
// package's MAC derivation, payload order or encoding drifts from identity's, every token identity
// issues is refused by platform, and the operator area is down with green tests everywhere else.
const goldenToken = "op1.eyJyZWFsbSI6Im9wZXJhdG9yIiwic2lkIjoic2lkLUZBS0UtMDAwMSIsImV4cCI6MTg5MzU1MzQ0NX0." +
	"FcVpku_Youzpxe_uGg7b26fXcW8ex49afK7Y7n-Dv7o"

var goldenClaims = Claims{SessionID: "sid-FAKE-0001", ExpiresAt: time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)}

var t0 = time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)

func signer(t *testing.T, keys ...secret.Secret) *Signer {
	t.Helper()
	s, err := NewSigner(keys)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestGoldenToken(t *testing.T) {
	s := signer(t, keyFake)
	got, err := s.Sign(goldenClaims)
	if err != nil {
		t.Fatal(err)
	}
	if got != goldenToken {
		t.Fatalf("Sign drifted from identity's format:\n got %s\nwant %s", got, goldenToken)
	}
	c, err := s.Verify(goldenToken, t0)
	if err != nil || c != goldenClaims {
		t.Fatalf("Verify(golden) = %+v, %v", c, err)
	}
}

func TestExpiredIsSeparateFromInvalid(t *testing.T) {
	s := signer(t, keyFake)
	tok, _ := s.Sign(Claims{SessionID: "s", ExpiresAt: t0})
	if _, err := s.Verify(tok, t0); !errors.Is(err, ErrExpired) {
		t.Fatalf("at expiry: err = %v, want ErrExpired", err)
	}
}

// A staff `v1.` token, a forged MAC, another key, an extra field (`tid`) and garbage are all ONE
// answer — and none of them reaches the payload parser before the MAC is checked.
func TestRefusals(t *testing.T) {
	s := signer(t, keyFake)
	good, _ := s.Sign(Claims{SessionID: "s", ExpiresAt: t0.Add(time.Hour)})
	other := signer(t, secret.Secret("another-operator-key-FAKE-0123456789abcd"))
	foreign, _ := other.Sign(Claims{SessionID: "s", ExpiresAt: t0.Add(time.Hour)})

	body := Version + "." + base64.RawURLEncoding.EncodeToString(
		[]byte(fmt.Sprintf(`{"realm":"operator","sid":"s","exp":%d,"tid":"01JD8ZQK9M3NPXR7TVWYB2C4EF"}`, t0.Add(time.Hour).Unix())))
	withTenant := body + "." + base64.RawURLEncoding.EncodeToString(mac(s.keys[0], body))

	for name, tok := range map[string]string{
		"staff v1":      "v1." + strings.TrimPrefix(good, "op1."),
		"tampered MAC":  good[:len(good)-2] + "AA",
		"other key":     foreign,
		"tenant field":  withTenant,
		"garbage":       "op1.x.y",
		"empty":         "",
		"no signature":  "op1.abc",
		"bad base64":    "op1.@@@.@@@",
		"version only":  "op1.",
		"other version": "op2." + strings.TrimPrefix(good, "op1."),
	} {
		if _, err := s.Verify(tok, t0); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
}

func TestRotationVerifiesEveryKeySignsWithFirst(t *testing.T) {
	old := signer(t, keyFake)
	tok, _ := old.Sign(Claims{SessionID: "s", ExpiresAt: t0.Add(time.Hour)})
	rotated := signer(t, secret.Secret("new-operator-signing-key-FAKE-0123456789"), keyFake)
	if _, err := rotated.Verify(tok, t0); err != nil {
		t.Fatalf("old-key token refused after rotation: %v", err)
	}
}

func TestConfiguration(t *testing.T) {
	if _, err := NewSigner(nil); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("no keys: %v", err)
	}
	if _, err := NewSigner([]secret.Secret{secret.Secret("short")}); !errors.Is(err, ErrKeyTooShort) {
		t.Errorf("short key: %v", err)
	}
	var none *Signer
	if _, err := none.Verify(goldenToken, t0); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("nil signer: %v", err)
	}
	if _, err := signer(t, keyFake).Sign(Claims{}); !errors.Is(err, ErrMissingClaim) {
		t.Errorf("empty claims: %v", err)
	}
}

func TestSignerNeverRendersKeys(t *testing.T) {
	s := signer(t, keyFake)
	for _, verb := range []string{"%v", "%+v", "%#v", "%s", "%d", "%x", "%q"} {
		out := fmt.Sprintf(verb, *s)
		if strings.Contains(out, "[") || strings.Contains(out, string(keyFake)) {
			t.Errorf("%s renders key material: %s", verb, out)
		}
	}
}
