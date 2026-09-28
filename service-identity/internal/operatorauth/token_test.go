package operatorauth

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/token"
)

// Fake signing keys — the text says so (rule 8, forbidden #1).
var (
	opKeyNew = secret.Secret("op-sign-key-NEW-FAKE-NOT-A-REAL-KEY-for-tests")
	opKeyOld = secret.Secret("op-sign-key-OLD-FAKE-NOT-A-REAL-KEY-for-tests")
	t0       = time.Date(2026, 9, 28, 8, 0, 0, 0, time.UTC)
)

func mustSigner(t *testing.T, keys ...secret.Secret) *TokenSigner {
	t.Helper()
	s, err := NewTokenSigner(keys)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestTokenRoundTrip(t *testing.T) {
	s := mustSigner(t, opKeyNew)
	tok, err := s.Sign(TokenClaims{SessionID: "sid-1", ExpiresAt: t0.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(tok, "op1.") || strings.Count(tok, ".") != 2 {
		t.Fatalf("unexpected shape: %s", tok)
	}
	c, err := s.Verify(tok, t0)
	if err != nil || c.SessionID != "sid-1" || !c.ExpiresAt.Equal(t0.Add(time.Hour)) {
		t.Fatalf("round trip: %+v %v", c, err)
	}
}

func TestTokenPayloadHasNoTenant(t *testing.T) {
	s := mustSigner(t, opKeyNew)
	tok, _ := s.Sign(TokenClaims{SessionID: "sid-1", ExpiresAt: t0.Add(time.Hour)})
	raw, _ := base64.RawURLEncoding.DecodeString(strings.Split(tok, ".")[1])
	if string(raw) != fmt.Sprintf(`{"realm":"operator","sid":"sid-1","exp":%d}`, t0.Add(time.Hour).Unix()) {
		t.Fatalf("payload must be exactly realm/sid/exp, got %s", raw)
	}
}

func TestTokenExpired(t *testing.T) {
	s := mustSigner(t, opKeyNew)
	tok, _ := s.Sign(TokenClaims{SessionID: "sid-1", ExpiresAt: t0})
	if _, err := s.Verify(tok, t0); !errors.Is(err, ErrExpiredToken) {
		t.Fatalf("at expiry: %v", err)
	}
	if _, err := s.Verify(tok, t0.Add(-time.Second)); err != nil {
		t.Fatalf("before expiry: %v", err)
	}
}

func TestTokenRotation(t *testing.T) {
	old := mustSigner(t, opKeyOld)
	tokOld, _ := old.Sign(TokenClaims{SessionID: "sid-1", ExpiresAt: t0.Add(time.Hour)})
	rotated := mustSigner(t, opKeyNew, opKeyOld)
	if _, err := rotated.Verify(tokOld, t0); err != nil {
		t.Fatalf("a token under the old key must verify after rotation: %v", err)
	}
	tokNew, _ := rotated.Sign(TokenClaims{SessionID: "sid-2", ExpiresAt: t0.Add(time.Hour)})
	if _, err := old.Verify(tokNew, t0); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("the new key must sign: %v", err)
	}
}

// forge builds a correctly MAC'd operator token around an arbitrary payload, to prove the checks
// AFTER the MAC (realm, unknown fields) hold on their own.
func forge(s *TokenSigner, payload string) string {
	body := TokenVersion + "." + base64.RawURLEncoding.EncodeToString([]byte(payload))
	return body + "." + base64.RawURLEncoding.EncodeToString(mac(s.keys[0], body))
}

func TestTokenRefusesBadPayloads(t *testing.T) {
	s := mustSigner(t, opKeyNew)
	exp := t0.Add(time.Hour).Unix()
	for name, p := range map[string]string{
		"wrong realm":   fmt.Sprintf(`{"realm":"staff","sid":"s","exp":%d}`, exp),
		"no realm":      fmt.Sprintf(`{"sid":"s","exp":%d}`, exp),
		"tenant field":  fmt.Sprintf(`{"realm":"operator","sid":"s","exp":%d,"tid":"01J0FAKE"}`, exp),
		"no sid":        fmt.Sprintf(`{"realm":"operator","exp":%d}`, exp),
		"no exp":        `{"realm":"operator","sid":"s"}`,
		"trailing json": fmt.Sprintf(`{"realm":"operator","sid":"s","exp":%d}{}`, exp),
		"not json":      `realm=operator`,
	} {
		if _, err := s.Verify(forge(s, p), t0); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("%s: want ErrInvalidToken, got %v", name, err)
		}
	}
}

func TestTokenRefusesTampering(t *testing.T) {
	s := mustSigner(t, opKeyNew)
	tok, _ := s.Sign(TokenClaims{SessionID: "sid-1", ExpiresAt: t0.Add(time.Hour)})
	parts := strings.Split(tok, ".")
	longer, _ := base64.RawURLEncoding.DecodeString(parts[1])
	relabelled := "op2." + parts[1] + "." + parts[2]
	swapped := parts[0] + "." + base64.RawURLEncoding.EncodeToString([]byte(strings.Replace(string(longer), "sid-1", "sid-2", 1))) + "." + parts[2]
	for name, bad := range map[string]string{
		"empty":      "",
		"no dots":    "op1",
		"relabelled": relabelled,
		"payload":    swapped,
		"mac":        tok[:len(tok)-2] + "AA",
		"no mac":     parts[0] + "." + parts[1] + ".",
		"other key": func() string {
			x, _ := mustSigner(t, opKeyOld).Sign(TokenClaims{SessionID: "sid-1", ExpiresAt: t0.Add(time.Hour)})
			return x
		}(),
	} {
		if _, err := s.Verify(bad, t0); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("%s: want ErrInvalidToken, got %v", name, err)
		}
	}
}

// ADR 0048 stop condition #6, both directions, with the SAME key bytes in both realms — the worst
// case, which core/config refuses but this layer must survive on its own.
func TestTokenCrossRealmRefusedBothWays(t *testing.T) {
	staff, err := token.NewSigner([]secret.Secret{opKeyNew})
	if err != nil {
		t.Fatal(err)
	}
	op := mustSigner(t, opKeyNew)

	staffTok, err := staff.Ky(token.Claims{TenantID: tenant.ID("01J0000000000000000000FAKE"), Sid: "sid-1", ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := op.Verify(staffTok, time.Now()); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("a staff v1 token must be refused by the operator verifier: %v", err)
	}
	// Even relabelled as op1, the staff MAC is not an operator MAC (derived key).
	relabelled := "op1" + strings.TrimPrefix(staffTok, "v1")
	if _, err := op.Verify(relabelled, time.Now()); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("a relabelled staff token must be refused: %v", err)
	}

	opTok, _ := op.Sign(TokenClaims{SessionID: "sid-1", ExpiresAt: time.Now().Add(time.Hour)})
	if _, err := staff.Giai(opTok); err == nil {
		t.Fatal("an operator op1 token must be refused by the staff verifier")
	}
	if _, err := staff.Giai("v1" + strings.TrimPrefix(opTok, "op1")); err == nil {
		t.Fatal("an operator token relabelled v1 must be refused by the staff verifier")
	}
}

func TestTokenConfiguration(t *testing.T) {
	if _, err := NewTokenSigner(nil); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("no keys: %v", err)
	}
	var none *TokenSigner
	if _, err := none.Sign(TokenClaims{SessionID: "s", ExpiresAt: t0}); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("nil signer sign: %v", err)
	}
	if _, err := none.Verify("op1.x.y", t0); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("nil signer verify: %v", err)
	}
	if _, err := NewTokenSigner([]secret.Secret{opKeyNew, secret.Secret(strings.Repeat("k", 31))}); !errors.Is(err, ErrKeyTooShort) {
		t.Fatalf("short key: %v", err)
	}
	s := mustSigner(t, opKeyNew)
	if _, err := s.Sign(TokenClaims{ExpiresAt: t0}); !errors.Is(err, ErrMissingClaim) {
		t.Fatalf("no sid: %v", err)
	}
	if _, err := s.Sign(TokenClaims{SessionID: "s"}); !errors.Is(err, ErrMissingClaim) {
		t.Fatalf("no expiry: %v", err)
	}
}

func TestTokenSignerNeverRendersKeys(t *testing.T) {
	s := mustSigner(t, opKeyNew, opKeyOld)
	for _, verb := range []string{"%v", "%+v", "%#v", "%s", "%d", "%x", "%q"} {
		for _, o := range []string{fmt.Sprintf(verb, s), fmt.Sprintf(verb, *s)} {
			if strings.Contains(o, "op-sign-key") || !strings.Contains(o, "TokenSigner(2 keys)") {
				t.Fatalf("%s: unexpected rendering %q", verb, o)
			}
		}
	}
}
