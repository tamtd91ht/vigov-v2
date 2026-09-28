// Package operatorauth holds the cryptographic building blocks of the ViHAT OPERATOR realm
// (ADR 0048, owner's decisions of 2026-09-28 #2+4 — the realm token — and #10 — MFA required; the
// parameters are in §"Chốt bước 1 — 28/09/2026", rows "Token và bí mật" and "Tham số"): the `op1.` session token, TOTP, the
// AES-256-GCM sealer for TOTP secrets at rest, and one-time recovery codes.
//
// PURE FUNCTIONS AND VALUE TYPES ONLY: no database, no transport, no clock of its own (every
// time-dependent call takes `now`). The use cases that store, look up and audit are built on top.
//
// WHY THIS IS NOT core/token: the operator realm has NO commune. A staff token carries tenant_id
// and is compared against the Host on every request (rule 1, invariant 8); an operator token must
// carry no tenant field at all, and must never be accepted where a staff token is — nor the
// reverse (ADR 0048 stop condition #6). Separate format, separate key list, separate MAC key
// derivation: three independent reasons a token of one realm fails in the other.
//
// STANDARD LIBRARY ONLY, no new dependency: HMAC-SHA1 (TOTP, RFC 6238), HMAC-SHA256, AES-GCM and
// SHA-256 are all in crypto/*.
//
// SECRETS NEVER REACH AN ERROR STRING OR A LOG LINE. Material travels as secret.Secret; the types
// that hold keys (TokenSigner, Sealer) close every fmt/slog rendering path themselves, because
// their unexported fields are exactly what fmt reaches by reflection.
package operatorauth

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
)

// ErrNotConfigured means the operator realm has no key material. The caller REFUSES the operator
// sign-in (fail closed); it never falls back to anything. The service still serves every other
// request — both variables are optional (core/config operator.go).
var ErrNotConfigured = errors.New("operatorauth: operator realm is not configured")

// redacted renders a key-holding type as its name and key count, on every fmt verb.
//
// Format is load-bearing: fmt consults String only for the string-shaped verbs, so without it %d
// prints the unexported [][]byte field one byte at a time (the hole core/secret documents).
func redacted(f fmt.State, verb rune, s string) {
	if verb == 'q' {
		_, _ = io.WriteString(f, strconv.Quote(s))
		return
	}
	_, _ = io.WriteString(f, s)
}

func redactedLog(s string) slog.Value { return slog.StringValue(s) }
