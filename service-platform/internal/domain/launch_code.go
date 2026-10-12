package domain

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
)

// A commune's LAUNCH CODE is the opaque public code its QR / launch link carries (`?x=<code>&src=qr`)
// in place of its domain (owner decision 12/10/2026; column tenant.launch_code, migration 0024).
//
// It steers which commune's interface opens and grants nothing (ADR 0005 · 0019 · 0022): the commune
// enters a session only by the citizen's explicit act. That is why 48 random bits are enough, and why
// the code may sit in audit entries and on printed paper. It is not personal data.
//
// The shape is the one 0024's CHECK (tenant_launch_code_shape) and platform.proto's
// ResolveHostRequest.launch_code declare. The three spellings must move together: a code this file
// mints that the CHECK refuses fails every commune creation, and a code the CHECK admits that this
// file refuses is a commune ResolveHost can never find.
var launchCodeShape = regexp.MustCompile(`^[0-9a-f]{12}$`)

// launchCodeBytes is 6 bytes = 12 hex characters.
const launchCodeBytes = 6

// ErrLaunchCodeInvalid — not exactly 12 lowercase hex characters.
var ErrLaunchCodeInvalid = errors.New("tenant: mã mở app của xã không đúng dạng (12 ký tự hex thường)")

// NewLaunchCode draws a fresh code from crypto/rand.
//
// NOT DERIVED FROM ANYTHING (owner 12/10/2026): a code computed from the name, the domain or the
// tenant id is a code anyone can compute for another commune, and one that changes when the commune
// is renamed or merged. crypto/rand only — rule 13 forbids math/rand. Uniqueness is the database's
// (tenant_launch_code_unique); the caller retries on that violation.
func NewLaunchCode() (string, error) {
	b := make([]byte, launchCodeBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("tenant: draw launch code: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// ValidLaunchCode reports whether s has the launch code shape. No normalisation: an upper-case or
// padded code is refused, never repaired — the stored codes are lower-case, so a repaired one would
// only ever match by accident of the repair.
func ValidLaunchCode(s string) bool {
	return launchCodeShape.MatchString(s)
}
