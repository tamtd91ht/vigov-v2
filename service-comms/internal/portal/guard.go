// Package portal is the OUTBOUND adapter to a commune's e-government portal (Cổng thông tin điện
// tử) — today only the Đà Nẵng shared portal's `apichiase` API (ADR 0067 §2, provider
// `cttdt-danang`). It is the first place this service fetches a URL a member of staff typed, so it is
// an SSRF surface before it is anything else.
//
// THE BOUNDARY IS TWO CHECKS, AND BOTH ARE NEEDED:
//
//  1. CheckURL, on every URL before a request leaves — the saved api_url, every redirect target, every
//     image: https only, the host a DNS name ending in `.gov.vn`, no IP literal, no userinfo, port 443
//     only. A name check alone is not enough: `x.gov.vn` can be made to resolve anywhere.
//  2. the guarded dialer, at CONNECT time: the name is resolved HERE, every address it resolves to must
//     be public, and the connection is made to that checked address — never to the name again. So a
//     DNS answer that changes between a check and a connect (rebinding) cannot reach the cluster, a
//     loopback, a link-local metadata endpoint or a private range.
//
// THE NETWORKPOLICY IS NOT THE BOUNDARY. deploy/base/mang/netpol.yaml opens comms 443 to 0.0.0.0/0
// (the portal publishes no address range); these two checks are what keep that rule from being an
// open door. Weakening either for a test is weakening production: tests inject a resolver and a
// dialer (Options) and leave both checks exactly as they are.
package portal

import (
	"context"
	"net"
	"net/netip"
	"net/url"
	"strings"
)

// AllowedHostSuffix is ADR 0067 §2 decision 1: the portal host is a government host. A `.gov.vn`
// host is NOT necessarily THIS commune's (the ADR's accepted risk) — what keeps the commune's key from
// following a typo is the "api_url change requires the key again" rule, not this suffix.
const AllowedHostSuffix = ".gov.vn"

// httpsPort is the only port a request may use. DECIDED HERE AND STATED: the schema's CHECK admits any
// port, this refuses everything but 443 — an e-government portal on a non-default port is not a case
// we have met, and a free port is how an SSRF reaches a service that is not a web server.
const httpsPort = "443"

// CheckURL is check 1 of the package doc. The error never quotes the URL: it may carry the key.
func CheckURL(u *url.URL) error {
	if u == nil || u.Opaque != "" || !strings.EqualFold(u.Scheme, "https") {
		return ErrURLRefused
	}
	if u.User != nil {
		return ErrURLRefused
	}
	if p := u.Port(); p != "" && p != httpsPort {
		return ErrURLRefused
	}
	return checkHostName(u.Hostname())
}

// checkHostName accepts a DNS name ending in AllowedHostSuffix with at least one label before it, made
// of LDH labels (punycode `xn--` included). An IP literal, a trailing dot, a zone, an empty label or
// anything outside [a-z0-9-] is refused.
func checkHostName(host string) error {
	h := strings.ToLower(host)
	if h == "" || strings.ContainsAny(h, "%[]") || strings.HasSuffix(h, ".") {
		return ErrURLRefused
	}
	if _, err := netip.ParseAddr(h); err == nil {
		return ErrURLRefused
	}
	if !strings.HasSuffix(h, AllowedHostSuffix) || len(h) <= len(AllowedHostSuffix) {
		return ErrURLRefused
	}
	for _, label := range strings.Split(h, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return ErrURLRefused
		}
		for i := 0; i < len(label); i++ {
			c := label[i]
			if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
				return ErrURLRefused
			}
		}
	}
	return nil
}

// blockedPrefixes are the non-public ranges netip's predicates do not cover. Each is a way back into
// a network that is not the public Internet: carrier NAT (some clouds put metadata there —
// 100.100.100.200), the "this network" block, benchmark and documentation ranges, the reserved class
// E, and the IPv6 transition prefixes that EMBED an IPv4 address (NAT64, 6to4, Teredo, and — added
// 02/10/2026, R4 — the deprecated IPv4-compatible ::/96 and the SIIT IPv4-translated ::ffff:0:0:0/96),
// which would otherwise let a v6 answer name a private v4 host. IPv4-MAPPED (::ffff:0:0/96) needs no
// row: AddrAllowed unmaps it and judges the IPv4 it carries.
var blockedPrefixes = func() []netip.Prefix {
	var out []netip.Prefix
	for _, s := range []string{
		"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15",
		"198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "255.255.255.255/32",
		"64:ff9b::/96", "64:ff9b:1::/48", "100::/64", "2001::/32", "2001:db8::/32", "2002::/16",
		"fec0::/10", "::/96", "::ffff:0:0:0/96",
	} {
		out = append(out, netip.MustParsePrefix(s))
	}
	return out
}()

// AddrAllowed reports whether a resolved address is public. Loopback, private (RFC 1918, ULA),
// link-local (169.254.0.0/16 holds the cloud metadata endpoint), multicast, unspecified and every
// blocked prefix are refused. An IPv4-mapped IPv6 address is judged as the IPv4 it carries.
func AddrAllowed(a netip.Addr) bool {
	a = a.Unmap()
	if !a.IsValid() || a.Zone() != "" || a.IsLoopback() || a.IsPrivate() || a.IsUnspecified() ||
		a.IsLinkLocalUnicast() || a.IsLinkLocalMulticast() || a.IsInterfaceLocalMulticast() ||
		a.IsMulticast() {
		return false
	}
	for _, p := range blockedPrefixes {
		if p.Contains(a) {
			return false
		}
	}
	return true
}

// Resolver is the part of *net.Resolver the dialer uses — injectable so a test can answer for a
// `.gov.vn` name without a real DNS. Production uses net.DefaultResolver.
type Resolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

// DialFunc opens the TCP connection to an address the guard has ALREADY checked (`ip:443`).
type DialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// guardedDialer is check 2 of the package doc.
type guardedDialer struct {
	resolver Resolver
	dial     DialFunc
}

// DialContext resolves host, refuses the connection when ANY answer is not public — stricter than
// "pick a good one", because a name that resolves to a private address is not a portal we should be
// talking to at all — and connects to the checked addresses only, in order.
func (d *guardedDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil || port != httpsPort {
		return nil, ErrAddressRefused
	}
	if err := checkHostName(host); err != nil {
		return nil, ErrAddressRefused
	}
	addrs, err := d.resolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		if ctx.Err() != nil {
			return nil, classifyContext(ctx.Err())
		}
		return nil, ErrDNS
	}
	if len(addrs) == 0 {
		return nil, ErrDNS
	}
	for _, a := range addrs {
		if !AddrAllowed(a) {
			return nil, ErrAddressRefused
		}
	}
	var last error = ErrConnect
	for _, a := range addrs {
		conn, err := d.dial(ctx, network, net.JoinHostPort(a.Unmap().String(), port))
		if err == nil {
			return conn, nil
		}
		last = err
	}
	return nil, last
}
