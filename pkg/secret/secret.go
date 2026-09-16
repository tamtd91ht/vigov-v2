// Package secret holds the values that must never reach a log line, and makes refusing to
// print them a property of the TYPE rather than a habit of the call sites.
//
// WHY A SHARED PACKAGE AND NOT A TYPE PER CALL SITE: the same defect has now been patched four
// times — config.Khoa, token.Signer, and then the database/Redis DSNs and the raw [][]byte
// handed to the signer. Each patch was correct and each one covered exactly one type, so the
// fifth secret added to this system would start again from "plain string". One type, used
// everywhere, is the only version of this that ends.
//
// WHAT IT COSTS TO GET WRONG: a process serves 200+ communes. A signing key or a database
// password written into one log line reaches centralised logging, backups and a third-party
// monitoring vendor at once, and a secret that reaches any of those cannot be recalled
// (rule 8, invariant 1). One leaked session key forges a staff session in EVERY commune.
//
// THIS PACKAGE IMPORTS NOTHING BUT THE STANDARD LIBRARY, on purpose. It has to sit BELOW
// pkg/config and pkg/token — both of which hold secrets — and any import in the other
// direction would be a cycle.
package secret

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
)

// Che is what every rendering path produces instead of the material.
const Che = "***"

// Secret is opaque key material: session signing keys, API keys, anything whose bytes are the
// credential itself.
//
// EVERY RENDERING PATH IS CLOSED, and the list is not decoration — each entry is a separate
// route by which fmt or slog can reach the bytes:
//
//	Format       every verb, INCLUDING the numeric ones. See below; this is the one that matters
//	String       fmt.Stringer, and callers that print .String() themselves
//	GoString     the %#v verb, which does NOT consult String
//	MarshalJSON  encoding/json
//	MarshalText  encoding/json's fallback, and anything else taking a TextMarshaler
//	LogValue     slog, which checks LogValuer before it falls back to reflection
//
// WHY Format IS LOAD-BEARING AND String() ALONE IS NOT: fmt only consults String for the verbs
// it routes through a Stringer — v, s, q, x, X. It never consults it for the NUMERIC verbs, and
// the underlying type here is []byte, so without Format the verbs d, c, U, b and o each print
// the material one byte at a time — the key as a list of numbers, or as a list of characters.
// That is a real hole that was measured on this code, not a hypothetical: a type promising
// "refuses to render" while one family of verbs renders it in full is worse than a plain
// []byte, because it stops anybody looking. fmt.Formatter takes precedence over every verb, so
// it is the only construction that can make the promise true.
//
// THE RECEIVERS ARE VALUES, NOT POINTERS, ON PURPOSE: a value receiver puts these methods in
// the method set of BOTH Secret and *Secret, so a dereferenced copy and a pointer taken while
// debugging are both covered. A pointer receiver would leave a printed VALUE showing the bytes.
type Secret []byte

// Format closes every verb. The verb is deliberately ignored: there is no verb for which
// printing key material is the right answer, so there is no switch here to get wrong later.
func (s Secret) Format(f fmt.State, verb rune) { viet(f, verb, Che) }

func (s Secret) String() string { return Che }

func (s Secret) GoString() string { return Che }

func (s Secret) MarshalJSON() ([]byte, error) { return []byte(`"` + Che + `"`), nil }

func (s Secret) MarshalText() ([]byte, error) { return []byte(Che), nil }

func (s Secret) LogValue() slog.Value { return slog.StringValue(Che) }

// Lo hands over the raw material. THE NAME IS THE WARNING — "lộ" is what calling it does.
//
// CALLING THIS LEAVES THE PROTECTION. Use it only at the FINAL point of consumption — the
// argument of token.NewSigner, of sql.Open — and never assign the result to a variable that
// then travels: a []byte in a local, in a struct field or on a channel is a plain byte slice
// again, and the next person to print it gets the key.
//
// One exit, one name, so searching for `.Lo()` is the complete list of places where the
// material is in the open.
func (s Secret) Lo() []byte { return []byte(s) }

// Rong reports whether there is no material at all. Exists so a caller can ask that question
// without reaching for Lo().
func (s Secret) Rong() bool { return len(s) == 0 }

// DSN is a driver connection string: it carries a password, and it is otherwise an ordinary
// string that everybody prints.
//
// WHY IT DOES NOT COLLAPSE TO "***" LIKE Secret: the startup line has to say WHICH database
// this process opened. A service pointed at the wrong host by a bad deploy is diagnosed from
// that line, and a DSN rendered as "***" turns a two-second diagnosis into an afternoon. So
// the scheme, the user and the host survive and ONLY the password is removed — the same
// behaviour config.Redacted() had, now applied on every rendering path instead of only on the
// call sites that remembered to ask for it.
type DSN string

func (d DSN) Format(f fmt.State, verb rune) { viet(f, verb, d.String()) }

// String is the redaction. Everything else here routes through it, so there is one definition
// of "what may be shown".
func (d DSN) String() string { return che(string(d)) }

func (d DSN) GoString() string { return d.String() }

func (d DSN) MarshalJSON() ([]byte, error) { return json.Marshal(d.String()) }

func (d DSN) MarshalText() ([]byte, error) { return []byte(d.String()), nil }

func (d DSN) LogValue() slog.Value { return slog.StringValue(d.String()) }

// Lo hands over the connection string with its password intact. Same rule as Secret.Lo: the
// argument of sql.Open or redis.ParseURL, and nowhere else.
func (d DSN) Lo() string { return string(d) }

// viet writes the replacement, quoting it for the %q verb so a quoted verb still produces a
// quoted value and a log line stays parseable.
func viet(f fmt.State, verb rune, s string) {
	if verb == 'q' {
		_, _ = io.WriteString(f, strconv.Quote(s))
		return
	}
	_, _ = io.WriteString(f, s)
}

// che removes the password from a driver URL, keeping the scheme, the user and the host so a
// misconfigured target is still recognisable.
//
// A string that is not a URL at all is replaced WHOLE: it cannot be parsed, so there is no way
// to tell which part of it is the credential, and guessing wrong here prints the password.
func che(dsn string) string {
	if dsn == "" {
		return ""
	}
	i := strings.Index(dsn, "://")
	if i < 0 {
		return Che
	}
	rest := dsn[i+3:]
	at := strings.LastIndex(rest, "@")
	if at < 0 {
		return dsn // no credentials present
	}
	cred := rest[:at]
	if colon := strings.Index(cred, ":"); colon >= 0 {
		cred = cred[:colon] + ":" + Che
	}
	return dsn[:i+3] + cred + rest[at:]
}
