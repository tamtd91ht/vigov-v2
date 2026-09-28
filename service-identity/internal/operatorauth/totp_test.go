package operatorauth

import (
	"bytes"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/secret"
)

// rfcKey is the published test key of RFC 4226 appendix D and RFC 6238 appendix B (SHA-1).
// Not a credential: it is printed in both RFCs.
var rfcKey = secret.Secret("12345678901234567890")

func TestHOTPRFC4226Vectors(t *testing.T) {
	want := []string{"755224", "287082", "359152", "969429", "338314",
		"254676", "287922", "162583", "399871", "520489"}
	for counter, w := range want {
		if got := hotp(rfcKey.Lo(), uint64(counter), 6); got != w {
			t.Errorf("counter %d: got %s want %s", counter, got, w)
		}
	}
}

// RFC 6238 appendix B, SHA-1 column (8 digits) — exercises the time-step computation too.
func TestTOTPRFC6238Vectors(t *testing.T) {
	for _, tc := range []struct {
		unix int64
		want string
	}{
		{59, "94287082"}, {1111111109, "07081804"}, {1111111111, "14050471"},
		{1234567890, "89005924"}, {2000000000, "69279037"}, {20000000000, "65353130"},
	} {
		got := hotp(rfcKey.Lo(), uint64(TimeStep(time.Unix(tc.unix, 0))), 8)
		if got != tc.want {
			t.Errorf("T=%d: got %s want %s", tc.unix, got, tc.want)
		}
	}
}

func TestVerifyWindowAndMatchedStep(t *testing.T) {
	// Step 1 (t in [30,60)) → "287082".
	for _, tc := range []struct {
		unix     int64
		ok       bool
		wantStep int64
	}{
		{29, true, 1},  // step 0, code from step +1 (clock behind)
		{45, true, 1},  // exact step
		{89, true, 1},  // step 2, code from step -1
		{90, false, 0}, // step 3: outside ±1
		{0, true, 1},   // still step 0
	} {
		step, ok := Verify(rfcKey, "287082", time.Unix(tc.unix, 0))
		if ok != tc.ok || step != tc.wantStep {
			t.Errorf("t=%d: got (%d,%v) want (%d,%v)", tc.unix, step, ok, tc.wantStep, tc.ok)
		}
	}
}

// The caller refuses replay by comparing the matched step with the last one used. Verify must
// report the SAME step for the same code within the window, or that comparison cannot work.
func TestVerifyStepSupportsReplayRefusal(t *testing.T) {
	s1, ok1 := Verify(rfcKey, "287082", time.Unix(40, 0))
	s2, ok2 := Verify(rfcKey, "287082", time.Unix(70, 0))
	if !ok1 || !ok2 || s1 != s2 {
		t.Fatalf("same code, same step expected: (%d,%v) (%d,%v)", s1, ok1, s2, ok2)
	}
	lastUsed := s1
	if s2 > lastUsed {
		t.Fatal("a replayed code must not have a step greater than the one already used")
	}
}

func TestVerifyRejectsMalformed(t *testing.T) {
	now := time.Unix(45, 0)
	for _, code := range []string{"", "28708", "2870820", " 287082", "28708a", "２８７０８２", "287-82"} {
		if _, ok := Verify(rfcKey, code, now); ok {
			t.Errorf("%q must be refused", code)
		}
	}
	if _, ok := Verify(secret.Secret("short"), "287082", now); ok {
		t.Error("a secret of the wrong size must be refused")
	}
	if _, ok := Verify(rfcKey, "000000", now); ok {
		t.Error("wrong code accepted")
	}
}

func TestGenerateSecret(t *testing.T) {
	a, err := GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := GenerateSecret()
	if len(a) != TOTPSecretSize || bytes.Equal(a.Lo(), b.Lo()) {
		t.Fatal("secret must be 20 random bytes")
	}
	enc := string(EncodeSecret(a).Lo())
	if strings.Contains(enc, "=") || len(enc) != 32 {
		t.Fatalf("base32 without padding of 20 bytes is 32 chars, got %d", len(enc))
	}
}

func TestProvisioningURI(t *testing.T) {
	uri, err := ProvisioningURI("ViGov", "VH-00001", rfcKey)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(string(uri.Lo()))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if u.Scheme != "otpauth" || u.Host != "totp" || u.Path != "/ViGov:VH-00001" {
		t.Fatalf("unexpected URI shape: %s://%s%s", u.Scheme, u.Host, u.Path)
	}
	if q.Get("secret") != "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ" || q.Get("issuer") != "ViGov" ||
		q.Get("digits") != "6" || q.Get("period") != "30" || q.Get("algorithm") != "SHA1" {
		t.Fatalf("unexpected parameters: %v", q)
	}
	if uri.String() != secret.Che {
		t.Fatal("the URI carries the secret and must not render")
	}
}

func TestProvisioningURIRefusesPIIAndBadInput(t *testing.T) {
	for _, label := range []string{"", "  ", "someone@example.invalid", "a:b"} {
		if _, err := ProvisioningURI("ViGov", label, rfcKey); err != ErrInvalidLabel {
			t.Errorf("label %q: want ErrInvalidLabel, got %v", label, err)
		}
	}
	if _, err := ProvisioningURI("", "VH-00001", rfcKey); err != ErrInvalidIssuer {
		t.Errorf("empty issuer: got %v", err)
	}
	if _, err := ProvisioningURI("ViGov", "VH-00001", secret.Secret("short")); err != ErrInvalidSecret {
		t.Errorf("short secret: got %v", err)
	}
}

func TestCodeAtIsWhatVerifyAccepts(t *testing.T) {
	s, err := GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 28, 8, 0, 0, 0, time.UTC)
	code := CodeAt(s, now)
	step, ok := Verify(s, code, now)
	if !ok || step != TimeStep(now) {
		t.Fatalf("CodeAt output refused by Verify: ok=%v step=%d", ok, step)
	}
	if CodeAt(secret.Secret("short"), now) != "" {
		t.Fatal("a malformed secret must yield no code")
	}
}
