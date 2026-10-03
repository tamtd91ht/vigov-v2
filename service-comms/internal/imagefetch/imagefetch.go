// Package imagefetch is the OUTBOUND fetch of an image whose https link an officer pasted into an
// article body (ADR 0067 §Sửa đổi 03/10/2026, H5, K6). It is the second place this service fetches a
// URL a member of staff typed — and, unlike the portal, to ANY host: officers paste from newspapers, so
// a `.gov.vn` rule would make H1 useless (K6). That makes the dial-time check the whole boundary.
//
// THE BOUNDARY IS internal/portal's TWO CHECKS, the second one SHARED, not copied:
//
//  1. CheckURL, on the pasted URL before any network AND on every redirect target: https only, port
//     empty or 443, no userinfo, the host a DNS name (no IP literal, at least two labels), ≤ MaxURLLen.
//  2. the guarded dialer, at CONNECT time: the name is resolved HERE, EVERY answer must pass
//     portal.AddrAllowed — the exact predicate (and blocked-prefix table) the portal sync uses — and the
//     connection goes to that checked address, never to the name again. DNS rebinding, a redirect to a
//     name that resolves to the cluster, a loopback, 169.254.169.254: all refused at this point.
//
// Weakening either for a test is weakening production: tests inject a resolver, a socket and a trust
// pool (Options) and leave both checks exactly as they are — the portal tests' discipline.
//
// WHAT AN ERROR CARRIES: a class and nothing else (Error). A pasted URL can carry a token or a person's
// data in its query string (rule 3), and net/http's *url.Error quotes the whole URL; so the underlying
// error is classified and DROPPED, never wrapped.
package imagefetch

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/vihat/vigov/service-comms/internal/portal"
)

// MaxURLLen bounds the pasted link. 2048 is the de-facto browser/CDN ceiling; a longer "image link" is
// not one an officer copied from a page.
const MaxURLLen = 2048

// MaxRedirects is the card's (and the portal's, ADR 0067 Còn mở #5) three hops; every target is checked
// again by CheckURL and, when dialled, by the guarded dialer. ANY host is allowed, unlike the portal.
const MaxRedirects = 3

// UserAgent names ViGov to the site the image is fetched from — a fixed string, never the officer's.
const UserAgent = "ViGov-comms/1 (body-image fetch)"

// The bounds. Vendor numbers, chosen and stated, not customer figures.
const (
	connectTimeout        = 10 * time.Second
	tlsHandshakeTimeout   = 10 * time.Second
	responseHeaderTimeout = 20 * time.Second
	// fetchTimeout bounds the WHOLE call — DNS, every redirect, the body. The officer is waiting on the
	// request; a site slower than this is a site whose image they should download and upload instead.
	fetchTimeout = 30 * time.Second
	// maxResponseHeaderBytes bounds the status line and headers of every response (redirects included).
	// net/http's default is 1 MiB per response, OUTSIDE the body cap the caller passes: a site answering
	// with a megabyte of headers on each of MaxRedirects+1 hops, times the caller's concurrent fetches,
	// would hold memory the stated budget (bodyImageFetchSlots × the body cap) never counted. 64 KiB is
	// far above any real image response's headers. Over it, the fetch fails (`connect` class, a 502).
	maxResponseHeaderBytes = 64 << 10
)

const httpsPort = "443"

// Error is every failure this package returns: a class, nothing else (package doc).
type Error struct{ Class string }

func (e *Error) Error() string { return "imagefetch: " + e.Class }

// Is compares by class, so errors.Is(err, ErrTimeout) works on any returned value.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && t.Class == e.Class
}

// The classes. `http-<n>` is built per status.
var (
	ErrURLRefused       = &Error{Class: "url-refused"}        // the pasted URL failed CheckURL — the caller's 400
	ErrAddressRefused   = &Error{Class: "address-refused"}    // the name resolved to a non-public address
	ErrRedirectRefused  = &Error{Class: "redirect-refused"}   // a redirect target failed CheckURL
	ErrTooManyRedirects = &Error{Class: "too-many-redirects"} // more than MaxRedirects hops
	ErrDNS              = &Error{Class: "dns"}
	ErrConnect          = &Error{Class: "connect"}
	ErrTimeout          = &Error{Class: "timeout"}
	ErrTLS              = &Error{Class: "tls"}
	ErrCanceled         = &Error{Class: "canceled"}
	ErrTooLarge         = &Error{Class: "too-large"} // the body passed the caller's cap
)

// ClassOf returns the class of an error this package returned; `internal` for anything else.
func ClassOf(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Class
	}
	return "internal"
}

// IsRefusal reports whether THIS SIDE refused a destination (a redirect target or a resolved address) —
// the security event `outbound_url_refused`, as in the portal sync (R2, 02/10/2026).
func IsRefusal(err error) bool {
	switch ClassOf(err) {
	case ErrURLRefused.Class, ErrAddressRefused.Class, ErrRedirectRefused.Class, ErrTooManyRedirects.Class:
		return true
	}
	return false
}

// ParseURL parses and checks a pasted link (check 1). The fragment is dropped (it is never sent). The
// error never quotes the input.
func ParseURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > MaxURLLen || strings.ContainsAny(raw, " \t\r\n\\") {
		return nil, ErrURLRefused
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, ErrURLRefused
	}
	if err := CheckURL(u); err != nil {
		return nil, err
	}
	u.Fragment, u.RawFragment = "", ""
	return u, nil
}

// CheckURL is check 1 of the package doc, applied to the pasted URL and to every redirect target.
func CheckURL(u *url.URL) error {
	if u == nil || u.Opaque != "" || !strings.EqualFold(u.Scheme, "https") || u.User != nil {
		return ErrURLRefused
	}
	if p := u.Port(); p != "" && p != httpsPort {
		return ErrURLRefused
	}
	if len(u.String()) > MaxURLLen {
		return ErrURLRefused
	}
	return checkHostName(u.Hostname())
}

// checkHostName accepts a DNS name of at least two LDH labels (punycode `xn--` included) whose last label
// is not all digits. An IP literal, a single label (a cluster search domain would complete `minio` into
// an internal service), a trailing dot, a zone, or anything outside [a-z0-9-] is refused.
func checkHostName(host string) error {
	h := strings.ToLower(host)
	if h == "" || len(h) > 253 || strings.ContainsAny(h, "%[]:") || strings.HasSuffix(h, ".") {
		return ErrURLRefused
	}
	if _, err := netip.ParseAddr(h); err == nil {
		return ErrURLRefused
	}
	labels := strings.Split(h, ".")
	if len(labels) < 2 {
		return ErrURLRefused
	}
	for _, label := range labels {
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
	if _, err := strconv.Atoi(labels[len(labels)-1]); err == nil {
		return ErrURLRefused // `1.2.3.4`-shaped names that netip did not parse (`127.1`, `0177.1`)
	}
	return nil
}

// Options configures a Client. Every field has a production default; the injectable ones exist for
// tests, which replace the resolver, the socket and the trust pool — never CheckURL or AddrAllowed.
type Options struct {
	Resolver portal.Resolver // default net.DefaultResolver
	Dial     portal.DialFunc // default a net.Dialer with connectTimeout; receives a CHECKED ip:443
	RootCAs  *x509.CertPool  // default the system pool
}

// Client fetches pasted image links. Safe for concurrent use; one per process.
type Client struct {
	hc *http.Client
}

// New builds the client: a transport whose ONLY way to a socket is the guarded dialer, no proxy (an
// HTTPS_PROXY would move the connect check onto the proxy's address), TLS 1.2+ verified against the host
// name (rule 13, invariant 1), no cookie jar, and the redirect policy of check 1.
func New(o Options) *Client {
	if o.Resolver == nil {
		o.Resolver = net.DefaultResolver
	}
	if o.Dial == nil {
		d := &net.Dialer{Timeout: connectTimeout, KeepAlive: 30 * time.Second}
		o.Dial = d.DialContext
	}
	gd := &guardedDialer{resolver: o.Resolver, dial: o.Dial}
	return &Client{hc: &http.Client{
		Transport: &http.Transport{
			Proxy:                  nil,
			DialContext:            gd.DialContext,
			TLSClientConfig:        &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: o.RootCAs},
			TLSHandshakeTimeout:    tlsHandshakeTimeout,
			ResponseHeaderTimeout:  responseHeaderTimeout,
			MaxResponseHeaderBytes: maxResponseHeaderBytes,
			MaxIdleConns:           8,
			MaxIdleConnsPerHost:    2,
			IdleConnTimeout:        30 * time.Second,
			ForceAttemptHTTP2:      true,
		},
		CheckRedirect: checkRedirect,
		Jar:           nil, // no cookies, ever: nothing a site sets follows the officer's next paste
	}}
}

// checkRedirect allows at most MaxRedirects hops, each passing CheckURL. The Referer net/http would add
// is removed: it is the previous URL, query string included, handed to another host.
func checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) > MaxRedirects {
		return ErrTooManyRedirects
	}
	if CheckURL(req.URL) != nil {
		return ErrRedirectRefused
	}
	req.Header.Del("Referer")
	return nil
}

// Fetch GETs u (already through ParseURL) and returns at most max bytes of a 200 body. The bytes are
// NOT trusted: the caller sniffs, scans and re-encodes them. Over max → ErrTooLarge, read no further.
func (c *Client) Fetch(ctx context.Context, u *url.URL, max int64) ([]byte, error) {
	if max <= 0 {
		return nil, ErrTooLarge
	}
	if err := CheckURL(u); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, ErrURLRefused
	}
	req.Header.Set("Accept", "image/avif,image/webp,image/png,image/jpeg,image/*;q=0.8")
	req.Header.Set("User-Agent", UserAgent)

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, classify(err) // the *url.Error — which quotes the URL — ends here
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// Never read into anything: an error page may quote the request.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		return nil, &Error{Class: "http-" + strconv.Itoa(resp.StatusCode)}
	}
	if resp.ContentLength > max {
		return nil, ErrTooLarge
	}
	// The transport decompresses gzip BEFORE this reader, so the cap is on the bytes held in memory.
	b, err := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if err != nil {
		return nil, classify(err)
	}
	if int64(len(b)) > max {
		return nil, ErrTooLarge
	}
	return b, nil
}

// guardedDialer is check 2 of the package doc.
type guardedDialer struct {
	resolver portal.Resolver
	dial     portal.DialFunc
}

// DialContext resolves host, refuses the connection when ANY answer is not public (portal.AddrAllowed),
// and connects to the checked addresses only, in order.
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
		if !portal.AddrAllowed(a) {
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
	return nil, classify(last)
}

// classify maps whatever net/http returned onto a class. The input is discarded by every caller.
func classify(err error) *Error {
	var own *Error
	if errors.As(err, &own) {
		return own
	}
	if errors.Is(err, context.Canceled) {
		return ErrCanceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrTimeout
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return ErrTimeout
	}
	var (
		certErr    *tls.CertificateVerificationError
		unknownCA  x509.UnknownAuthorityError
		hostErr    x509.HostnameError
		invalidErr x509.CertificateInvalidError
		recordErr  tls.RecordHeaderError
		alertErr   tls.AlertError
	)
	switch {
	case errors.As(err, &certErr), errors.As(err, &unknownCA), errors.As(err, &hostErr),
		errors.As(err, &invalidErr), errors.As(err, &recordErr), errors.As(err, &alertErr):
		return ErrTLS
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return ErrDNS
	}
	return ErrConnect
}

func classifyContext(err error) *Error {
	if errors.Is(err, context.Canceled) {
		return ErrCanceled
	}
	return ErrTimeout
}
