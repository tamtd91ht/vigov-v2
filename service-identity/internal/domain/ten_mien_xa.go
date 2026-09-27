package domain

import (
	"net"
	"strings"
)

// HopLeTenMienXa reports whether s has the SHAPE of a commune's domain as a citizen channel sends
// it: the QR / URL parameter of the shared Mini App (ADR 0047). It says nothing about existence —
// only platform ResolveHost knows which commune, if any, holds a domain.
//
// ONE RULE, TWO DOORS. It is the shape `commune_host_hint` fixes in
// proto/vigov/identity/v1/citizen_session_bridge.proto, and both doors that take such a domain —
// GET /api/v1/communes?host= (internal/http/danh_muc_xa.go) and the session bridge
// (internal/app/cau_phien_cong_dan.go) — call THIS function. Two validators would drift, and the
// day they do, the confirmation screen names a commune for a value the bridge then refuses, or the
// other way round.
//
// STRICT, AND NEVER A REPAIR. The caller trims surrounding whitespace; nothing else is normalised.
// No scheme, port, path, query, fragment or userinfo; lowercase ASCII only (an internationalised
// name arrives as "xn--"); no leading, trailing or doubled dot. Folding case or stripping a port
// here is how two different inputs come to name one commune — ResolveHost's own normalisation must
// never be what makes them equal.
//
// AT LEAST TWO LABELS: no domain a commune is reached on is a single label, and it also keeps the
// literal `current` — the staff selector at /api/v1/communes/current — from ever being valid.
//
// NOT AN IP LITERAL: an address is not a commune's name. With ':' refused only IPv4 can reach the
// check; net.ParseIP is asked anyway so the rule does not depend on that reasoning staying true.
func HopLeTenMienXa(s string) bool {
	if len(s) == 0 || len(s) > 253 || !strings.Contains(s, ".") {
		return false
	}
	for _, nhan := range strings.Split(s, ".") {
		if len(nhan) == 0 || len(nhan) > 63 || nhan[0] == '-' || nhan[len(nhan)-1] == '-' {
			return false
		}
		for i := 0; i < len(nhan); i++ {
			c := nhan[i]
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	return net.ParseIP(s) == nil
}
