package config

import (
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/secret"
)

// SEARCH_HASH_KEY — the ONE key of the keyed n-gram hashes the citizen-directory search index holds in
// place of phone numbers and e-mail addresses (ADR 0101 #8; owner 17/10/2026: one key for the whole
// system, a secret variable, never a constant in source, the SAME value in vigov-dev and vigov-prod).
//
// WHAT IT PROTECTS: Elasticsearch never holds a citizen's number or address, only HMAC-SHA256 of their
// substrings under this key (core/searchhash). Whoever reads the index without the key cannot turn a hash
// back into a number. Whoever holds the key and the index can — so it is a k8s SECRET (`identity-secrets`)
// and secret.Secret here: it never prints.
//
// FORMAT: standard base64 of AT LEAST 32 random bytes — `openssl rand -base64 32`. Shorter is refused: an
// HMAC key below the hash's output size gives away the margin the keyed hash exists for.
//
// CHANGING IT IS NOT A ROTATION, it is a full REINDEX of every commune: every stored hash was made with the
// old key, and a query hashed with the new one matches none of them. identity stores the key's fingerprint
// in the index metadata and refuses to search or index when the configured key differs (ADR 0101 #8,
// owner 17/10/2026) — two keys' hashes are never mixed silently. The reindex command is the only path that
// adopts a new key.
//
// GROUP config.SearchHash: identity only. Required in staging/prod once declared (ADR 0057). In dev, empty
// means the directory's Elasticsearch search and indexing are OFF (a startup warning; keyword search falls
// back to PostgreSQL, attribute filters answer 503), never a guessed key.

// SearchHashKeyMinLength is the shortest key accepted, in bytes — SHA-256's output size.
const SearchHashKeyMinLength = 32

// ErrSearchHashKeyInvalid is a malformed SEARCH_HASH_KEY. Load refuses, by name, never with the value.
var ErrSearchHashKeyInvalid = errors.New("config: SEARCH_HASH_KEY is invalid")

// parseSearchHashKey decodes the trimmed value. "" stays nil (off, dev only).
func parseSearchHashKey(raw string) (secret.Secret, error) {
	if raw == "" {
		return nil, nil
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		// The decoder's own error quotes a byte offset into the material — not wrapped.
		return nil, fmt.Errorf("%w: not standard base64 (generate with `openssl rand -base64 32`)", ErrSearchHashKeyInvalid)
	}
	if len(key) < SearchHashKeyMinLength {
		return nil, fmt.Errorf("%w: decodes to %d bytes, at least %d are required (generate with `openssl rand -base64 32`)",
			ErrSearchHashKeyInvalid, len(key), SearchHashKeyMinLength)
	}
	return secret.Secret(key), nil
}

// SearchHashKey is the decoded SEARCH_HASH_KEY. Empty (Rong) only in dev. Never printed.
func (c Config) SearchHashKey() secret.Secret {
	c.require("SearchHashKey", SearchHash)
	return c.searchHashKey
}
