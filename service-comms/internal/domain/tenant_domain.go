package domain

import (
	"net"
	"strings"
)

// IsValidTenantDomain reports whether s has the SHAPE of a commune's domain as the Mini App sends it in
// `?host=` (GET /api/v1/commune-news). It says nothing about existence — only platform ResolveHost
// knows which commune, if any, holds a domain.
//
// A COPY OF service-identity/internal/domain/ten_mien_xa.go, CHARACTER FOR CHARACTER, NOT AN IMPORT:
// rule 2 forbids importing another service's internal/, and core/ was outside this change. Two copies
// of one validator are two copies that can drift — the day they do, the confirmation screen names a
// commune (identity) whose news this service then refuses with 400, or the reverse. The right home is
// core/, next to platformclient.XaTheoHost; that move is reported, not made here. Until then, change
// both or neither.
//
// STRICT, AND NEVER A REPAIR: no scheme, port, path, query, fragment or userinfo; lowercase ASCII only;
// no leading, trailing or doubled dot; at least two labels; not an IP literal.
func IsValidTenantDomain(s string) bool {
	if len(s) == 0 || len(s) > 253 || !strings.Contains(s, ".") {
		return false
	}
	for _, label := range strings.Split(s, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for i := 0; i < len(label); i++ {
			c := label[i]
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	return net.ParseIP(s) == nil
}
