package domain

import (
	"net"
	"strings"
)

// ValidCommuneHost reports whether s has the SHAPE of a commune's domain as the Mini App sends it — the
// domain printed on the commune's QR, which the accountless routes take as `host` (ADR 0083 row 2). It
// says nothing about existence: only platform ResolveHost knows which commune, if any, holds a domain.
//
// THE THIRD COPY, CHARACTER FOR CHARACTER, NOT AN IMPORT: service-identity/internal/domain/ten_mien_xa.go
// and service-comms/internal/domain/ten_mien_xa.go (HopLeTenMienXa) hold the other two. Rule 2 forbids
// importing another service's internal/. Three copies can drift — the day they do, the QR that opens a
// commune's notice board (comms) is refused here with 400. The right home is core/, next to
// platformclient.XaTheoHost; that move is reported, not made here. Until then, change all three or none.
//
// STRICT, AND NEVER A REPAIR: no scheme, port, path, query, fragment or userinfo; lowercase ASCII only;
// no leading, trailing or doubled dot; at least two labels; not an IP literal.
func ValidCommuneHost(s string) bool {
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
