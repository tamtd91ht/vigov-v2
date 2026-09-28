package config

import "errors"

// SECRET_ENCRYPTION_KEYS — the key-encryption keys (KEK) of ADR 0009 (owner's approval of
// 2026-09-28, card C4). core/crypto wraps each commune's data key (DEK) with them; the DEK then
// encrypts that commune's secrets (SMTP password, portal API key, OA key) inside the owning
// service's own database.
//
// FORMAT: exactly OPERATOR_TOTP_ENCRYPTION_KEY's — a comma-separated list, each entry standard
// base64 of EXACTLY 32 bytes, newest FIRST. The first entry wraps new DEKs; every entry unwraps.
// One format for both on purpose: an operator who has rotated one of them already knows the other.
//
// OPTIONAL AT Load, decided per variable (rule 11 invariant 8, stop condition #2): only a service
// that stores per-commune secrets reads it — comms first — and every other service serves every
// request without it. A service that needs it and lacks it REFUSES the operation by name
// (crypto.ErrNotConfigured), never falls back to storing a secret in the clear (ADR 0009 #3).
// Making it required would stop all eight services on every machine to protect one feature.
//
// k8s SECRET, secret.Secret: the bytes are the key that opens every commune's secrets.
//
// WHY THE BACKUP IS NOT OPTIONAL even though the variable is: losing every entry here makes every
// wrapped DEK — and so every commune's stored secret — permanently unreadable (ADR 0009 §Hệ quả).
// The value must be backed up SEPARATELY from the database backup before the first secret is
// written: a backup that holds both is a backup that holds the plaintext.

// ErrSecretEncryptionKeysInvalid is a malformed SECRET_ENCRYPTION_KEYS. Load refuses.
var ErrSecretEncryptionKeysInvalid = errors.New("config: SECRET_ENCRYPTION_KEYS is invalid")

// SecretEncryptionConfigured reports whether at least one KEK is configured.
func (c Config) SecretEncryptionConfigured() bool { return len(c.SecretEncryptionKeys) > 0 }
