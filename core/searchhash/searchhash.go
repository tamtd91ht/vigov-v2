// Package searchhash turns a phone number or an e-mail address into the KEYED N-GRAM HASHES a search
// index may hold in their place (ADR 0101 #8, owner 17/10/2026).
//
// WHY: the citizen directory is searched on Elasticsearch, and ES must never hold a citizen's number or
// address (rule 3). It holds, instead, HMAC-SHA256 under one system-wide key (SEARCH_HASH_KEY,
// core/config) of every contiguous substring of length 3..10 of the value. A query is hashed the same way
// and compared for equality, so "any part of the number" still finds the person, and whoever reads the
// index without the key reads no number.
//
// THE RISK THE OWNER ACCEPTED, stated where the code is: a 3-digit substring has only 1,000 values, so the
// hashes do not hide how OFTEN a substring occurs within one commune — frequency analysis inside a commune
// is possible; reversing a hash without the key is not.
//
// ONE KEY FOR EVERY COMMUNE (owner, 17/10/2026): the same phone in two communes yields the same hashes, so
// the TENANT FILTER on every query is the only thing separating them (rule 1). This package cannot enforce
// that; the index does.
//
// DOMAIN SEPARATION: each hash covers a short label ("phone", "email", "email-exact") before the value, so
// a digit-only e-mail local part can never collide with a phone substring, and a gram never equals a whole
// address. Changing a label is changing the key: every index built with the old label must be rebuilt.
//
// NOTHING HERE LOGS, and no error quotes the value: every input is personal data.
package searchhash

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/vihat/vigov/core/secret"
)

const (
	// MinGram and MaxGram bound the substring length (ADR 0101 #8: "dài 3…10").
	MinGram = 3
	MaxGram = 10
	// EmailLocalPrefix is how much of the part before '@' is cut into grams (ADR 0101 #8: "10 ký tự đầu").
	EmailLocalPrefix = 10
	// hashHexLen is the stored length: the first 128 bits of the HMAC, in hex. Enough to make a collision
	// between two different substrings of one commune negligible, half the bytes in the index.
	hashHexLen = 32
	// MinKeyLength mirrors config.SearchHashKeyMinLength.
	MinKeyLength = 32

	labelPhone      = "phone"
	labelEmail      = "email"
	labelEmailExact = "email-exact"
	// labelFingerprint names the key, never a value (Fingerprint).
	fingerprintMessage = "vigov-search-key-fingerprint-v1"
)

// ErrKeyTooShort is a key shorter than MinKeyLength. config.Load refuses it first; this is the backstop for
// a hand-built value.
var ErrKeyTooShort = errors.New("searchhash: key shorter than 32 bytes")

// Key is the HMAC key. The zero value is unusable (Valid false).
type Key struct {
	k []byte
}

// NewKey wraps the decoded SEARCH_HASH_KEY.
func NewKey(s secret.Secret) (Key, error) {
	b := s.Lo()
	if len(b) < MinKeyLength {
		return Key{}, ErrKeyTooShort
	}
	return Key{k: append([]byte(nil), b...)}, nil
}

// Valid reports whether the key can hash.
func (k Key) Valid() bool { return len(k.k) >= MinKeyLength }

func (k Key) sum(label, value string) string {
	m := hmac.New(sha256.New, k.k)
	m.Write([]byte(label))
	m.Write([]byte{0})
	m.Write([]byte(value))
	return hex.EncodeToString(m.Sum(nil))[:hashHexLen]
}

// Fingerprint names the key without revealing it: HMAC-SHA256(key, "vigov-search-key-fingerprint-v1"), the
// first 16 hex characters. Stored in the index metadata, compared at startup, so a changed key is noticed
// before any hash made with it reaches the index (owner, 17/10/2026).
func (k Key) Fingerprint() string {
	m := hmac.New(sha256.New, k.k)
	m.Write([]byte(fingerprintMessage))
	return hex.EncodeToString(m.Sum(nil))[:16]
}

// Substrings returns every contiguous substring of s (counted in runes) whose length is MinGram..MaxGram,
// each once, in first-seen order. Shorter than MinGram: none.
func Substrings(s string) []string {
	r := []rune(s)
	seen := map[string]bool{}
	var out []string
	for n := MinGram; n <= MaxGram && n <= len(r); n++ {
		for i := 0; i+n <= len(r); i++ {
			g := string(r[i : i+n])
			if !seen[g] {
				seen[g] = true
				out = append(out, g)
			}
		}
	}
	return out
}

func (k Key) hashAll(label string, grams []string) []string {
	out := make([]string, 0, len(grams))
	for _, g := range grams {
		out = append(out, k.sum(label, g))
	}
	return out
}

// PhoneGrams hashes every substring of a NORMALISED phone (digits only, the directory's national form).
// Anything but digits is refused with nil — the caller normalises first.
func (k Key) PhoneGrams(phone string) []string {
	if !k.Valid() || !allDigits(phone) {
		return nil
	}
	return k.hashAll(labelPhone, Substrings(phone))
}

// PhoneQuery hashes a typed phone fragment: 3..10 digits → one hash, ok. Anything else → ok false (the
// caller does not add a phone clause).
func (k Key) PhoneQuery(digits string) (string, bool) {
	if !k.Valid() || !allDigits(digits) || len(digits) < MinGram || len(digits) > MaxGram {
		return "", false
	}
	return k.sum(labelPhone, digits), true
}

// NormalizeEmail is the one spelling both sides hash: trimmed, lower case.
func NormalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

// EmailGrams hashes the substrings of the first EmailLocalPrefix runes of the part before '@', and the
// WHOLE address (exact). "" in, nothing out.
func (k Key) EmailGrams(email string) (grams []string, exact string) {
	e := NormalizeEmail(email)
	if !k.Valid() || e == "" {
		return nil, ""
	}
	local := e
	if i := strings.IndexByte(e, '@'); i >= 0 {
		local = e[:i]
	}
	if utf8.RuneCountInString(local) > EmailLocalPrefix {
		local = string([]rune(local)[:EmailLocalPrefix])
	}
	return k.hashAll(labelEmail, Substrings(local)), k.sum(labelEmailExact, e)
}

// EmailQuery hashes a typed e-mail fragment. Returned: the gram hash when the input is 3..10 runes (it may
// be part of the local part), and the exact hash when it is longer than 10 runes or carries '@' (it may be
// the whole address). Either may be "".
func (k Key) EmailQuery(input string) (gram, exact string) {
	q := NormalizeEmail(input)
	if !k.Valid() || q == "" || strings.ContainsAny(q, " \t") {
		return "", ""
	}
	n := utf8.RuneCountInString(q)
	if n >= MinGram && n <= MaxGram && !strings.Contains(q, "@") {
		gram = k.sum(labelEmail, q)
	}
	if n > MaxGram || strings.Contains(q, "@") {
		exact = k.sum(labelEmailExact, q)
	}
	return gram, exact
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
