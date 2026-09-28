package operatorauth

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"regexp"
	"testing"
)

var recoveryShape = regexp.MustCompile(`^[A-Z2-7]{4}-[A-Z2-7]{4}-[A-Z2-7]{4}-[A-Z2-7]{4}$`)

func TestGenerateRecoveryCodes(t *testing.T) {
	codes, hashes, err := GenerateRecoveryCodes(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(codes) != 10 || len(hashes) != 10 {
		t.Fatalf("want 10 codes and 10 hashes, got %d/%d", len(codes), len(hashes))
	}
	seen := map[string]bool{}
	for i, c := range codes {
		plain := string(c.Lo())
		if !recoveryShape.MatchString(plain) {
			t.Fatalf("code %d has the wrong shape", i)
		}
		if seen[plain] {
			t.Fatal("duplicate code in one batch")
		}
		seen[plain] = true
		if !bytes.Equal(hashes[i], HashRecoveryCode(plain)) {
			t.Fatalf("hash %d does not match its code", i)
		}
		if len(hashes[i]) != sha256.Size {
			t.Fatal("hash must be SHA-256")
		}
	}
	if _, _, err := GenerateRecoveryCodes(0); !errors.Is(err, ErrRecoveryCount) {
		t.Fatalf("n=0: %v", err)
	}
}

func TestHashRecoveryCodeNormalises(t *testing.T) {
	want := HashRecoveryCode("ABCD-EFGH-IJKL-MNOP")
	for _, in := range []string{"abcd-efgh-ijkl-mnop", "ABCDEFGHIJKLMNOP", " abcd efgh\tijkl-mnop ", "Abcd--Efgh-Ijkl-Mnop", "A8CD-EFGH-1JKL-MN0P"} {
		if !bytes.Equal(HashRecoveryCode(in), want) {
			t.Errorf("%q must hash like the canonical form", in)
		}
	}
	if bytes.Equal(HashRecoveryCode("ABCD-EFGH-IJKL-MNOQ"), want) {
		t.Error("a different code hashed the same")
	}
	sum := sha256.Sum256([]byte("ABCDEFGHIJKLMNOP"))
	if !bytes.Equal(want, sum[:]) {
		t.Error("stored form must be plain SHA-256 of the normalised code")
	}
}

func TestRecoveryCodesNeverRender(t *testing.T) {
	codes, _, _ := GenerateRecoveryCodes(1)
	if codes[0].String() != "***" {
		t.Fatal("a plaintext recovery code must not render")
	}
}
