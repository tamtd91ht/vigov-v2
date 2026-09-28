package operatorauth

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/vihat/vigov/core/secret"
)

// Recovery codes: one-time codes an operator uses when the authenticator app is lost (ADR 0048
// #10; `operator_recovery_code` in the ubiquitous language).
//
// 16 CHARACTERS OF BASE32 = 80 BITS, shown as XXXX-XXXX-XXXX-XXXX. The owner fixed storage as
// SHA-256 only — a FAST hash, no salt, no work factor. That is sound only if the code itself is
// too large to brute-force from a stolen table: at 50 bits one GPU recovers a code in about a day;
// at 80 bits it is out of reach. The entropy is what the missing work factor is paid with.
//
// The alphabet is RFC 4648 base32 (A–Z, 2–7): it has no 0, 1 or 8, so an operator who types one
// of those off paper meant O, I or B, and HashRecoveryCode reads it that way.
//
// HOW MANY codes a batch holds is NOT decided here: domain.RecoveryCodeCount is the one owner of
// that figure (the store refuses any other batch size), and the caller passes it in. A second
// constant here would be a second copy of an owner's decision, free to drift.
const (
	recoveryCodeChars = 16
	recoveryGroupSize = 4
	recoveryAlphabet  = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
)

// ErrRecoveryCount is a request for a non-positive number of codes.
var ErrRecoveryCount = errors.New("operatorauth: recovery code count must be positive")

// GenerateRecoveryCodes returns n plaintext codes — to be shown ONCE, never stored — and their
// SHA-256 hashes, in the same order, which are what the store keeps.
func GenerateRecoveryCodes(n int) (codes []secret.Secret, hashes [][]byte, err error) {
	if n <= 0 {
		return nil, nil, ErrRecoveryCount
	}
	seen := make(map[string]bool, n)
	for len(codes) < n {
		raw := make([]byte, recoveryCodeChars)
		if _, err := rand.Read(raw); err != nil {
			return nil, nil, fmt.Errorf("operatorauth: generate recovery code: %w", err)
		}
		// 256 is a multiple of 32, so b&31 is uniform — no modulo bias.
		var sb strings.Builder
		for i, b := range raw {
			if i > 0 && i%recoveryGroupSize == 0 {
				sb.WriteByte('-')
			}
			sb.WriteByte(recoveryAlphabet[b&31])
		}
		code := sb.String()
		h := HashRecoveryCode(code)
		if seen[string(h)] {
			continue // 2^-80 per pair; checked because a duplicate would be one code counted twice
		}
		seen[string(h)] = true
		codes = append(codes, secret.Secret(code))
		hashes = append(hashes, h)
	}
	return codes, hashes, nil
}

// HashRecoveryCode normalises what the operator typed — upper case, dashes and whitespace
// removed, 0/1/8 read as O/I/B — and returns its SHA-256. `abcd efgh-ijkl mnop` and
// `ABCD-EFGH-IJKL-MNOP` hash alike.
func HashRecoveryCode(input string) []byte {
	var sb strings.Builder
	for _, r := range input {
		if r == '-' || unicode.IsSpace(r) {
			continue
		}
		switch r = unicode.ToUpper(r); r {
		case '0':
			r = 'O'
		case '1':
			r = 'I'
		case '8':
			r = 'B'
		}
		sb.WriteRune(r)
	}
	sum := sha256.Sum256([]byte(sb.String()))
	return sum[:]
}
