package httpx

import (
	"context"
	"net/http"
	"net/netip"
	"strings"
)

// khoaClientIP is the context key ClientIPTuProxyTinCay stores the client address under.
// Unexported, so nothing outside this package can put a value there — a handler setting its own
// "client address" would be the forged header again, one layer further in.
type khoaClientIP struct{}

// ClientIPTuProxyTinCay computes the client address ONCE per request, crossing only the proxies
// the deployment configured (config.Config.TrustedProxies), and puts it where ClientIP reads it.
//
// MOUNT IT OUTERMOST, before StripTenantHeaders, TenantMiddleware and authentication: the cross-
// commune alert, the login trail and every audited write must name the SAME address for one
// request. Computed in two places, two layers could disagree about where one person acted.
//
// THE CHAIN IN PRODUCTION: client -> ingress-nginx -> web-admin -> this pod. Each proxy APPENDS the
// peer it saw to X-Forwarded-For, so the header's RIGHT end is written by hops we trust and its
// LEFT end by whoever sent the request. The walk therefore goes right to left:
//
//	peer ∉ trusted (or nothing configured)  -> peer; X-Forwarded-For ignored entirely
//	entry not an address                     -> stop; the last trusted hop reached. Nothing to the
//	                                            left of garbage was written by anyone we trust
//	entry ∈ trusted                          -> keep walking
//	first entry ∉ trusted                    -> that is the client
//	header exhausted, every entry trusted    -> the leftmost trusted hop reached
//
// Taking the LEFTMOST entry instead — the usual shortcut — takes the one value the client wrote
// itself: `X-Forwarded-For: <anyone's address>` would then be recorded as fact.
//
// X-Real-IP and Forwarded are NEVER read. Nothing in this deployment is configured to write them,
// so whatever arrives in them came from the client.
func ClientIPTuProxyTinCay(tinCay []netip.Prefix) func(http.Handler) http.Handler {
	// Copied so a caller mutating its slice after wiring cannot widen the boundary at runtime.
	ds := append([]netip.Prefix(nil), tinCay...)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := diaChiKhach(r, ds)
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), khoaClientIP{}, ip)))
		})
	}
}

// diaChiKhach is the walk described on ClientIPTuProxyTinCay.
func diaChiKhach(r *http.Request, tinCay []netip.Prefix) string {
	socket := diaChiSocket(r)
	if len(tinCay) == 0 {
		// Exactly the behaviour before a boundary could be configured.
		return socket
	}
	peer, err := netip.ParseAddr(socket)
	if err != nil {
		// Not an IP (an in-process test server, a unix socket): nothing to compare, so nothing
		// is trusted and the header is ignored.
		return socket
	}
	hop := chuanHoa(peer)
	if !trongDanhSach(hop, tinCay) {
		return hop.String()
	}

	// Every header line, in order: RFC 9110 §5.3 makes several lines equivalent to one line
	// joined by commas, and a proxy may add its own line rather than extend the existing one.
	var muc []string
	for _, dong := range r.Header.Values("X-Forwarded-For") {
		muc = append(muc, strings.Split(dong, ",")...)
	}
	for i := len(muc) - 1; i >= 0; i-- {
		a, err := netip.ParseAddr(strings.TrimSpace(muc[i]))
		if err != nil {
			return hop.String()
		}
		a = chuanHoa(a)
		if !trongDanhSach(a, tinCay) {
			return a.String()
		}
		hop = a
	}
	return hop.String()
}

// chuanHoa unmaps "::ffff:192.0.2.1" to "192.0.2.1" and drops any zone, so one host is one
// string on the trail and an IPv4 prefix matches it (netip does not match a mapped address
// against an IPv4 prefix).
func chuanHoa(a netip.Addr) netip.Addr { return a.Unmap().WithZone("") }

func trongDanhSach(a netip.Addr, ds []netip.Prefix) bool {
	for _, p := range ds {
		if p.Contains(a) {
			return true
		}
	}
	return false
}
