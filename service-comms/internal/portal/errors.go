package portal

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"strconv"
)

// Error is every failure this package returns. IT CARRIES A CLASS AND NOTHING ELSE, and that is the
// redaction: the key travels in the query string (`secret_code=…`), and net/http's *url.Error quotes
// the whole URL in its message. An error that wrapped it would print the commune's key into the first
// log line, the run log or a 500 that formatted it. So the underlying error is classified and DROPPED
// — never wrapped, never kept — and what remains is safe to log, to store in
// `portal_sync_runs.error_summary` and to compare with errors.Is.
type Error struct {
	// Class is the stable, non-personal label: `connect`, `timeout`, `http-503`, `parse`… It is what
	// the run log records per category (0013: "an error CLASS").
	Class string
	// Status is the portal's HTTP status for an `http-<n>` class, 0 otherwise.
	Status int
}

func (e *Error) Error() string { return "portal: " + e.Class }

// Is compares by class, so `errors.Is(err, portal.ErrTimeout)` works on any returned value;
// ErrHTTPStatus matches every `http-<n>`.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	if t == ErrHTTPStatus {
		return e.Status != 0
	}
	return t.Class == e.Class
}

// The classes. Each sentinel is also the value returned, so the zero-allocation path and errors.Is
// agree.
var (
	ErrURLRefused        = &Error{Class: "url-refused"}        // not https / not .gov.vn / IP literal / userinfo / port
	ErrAddressRefused    = &Error{Class: "address-refused"}    // the name resolved to a non-public address
	ErrRedirectRefused   = &Error{Class: "redirect-refused"}   // a redirect target failed CheckURL or left the api_url's host
	ErrTooManyRedirects  = &Error{Class: "too-many-redirects"} // more than MaxRedirects hops
	ErrDNS               = &Error{Class: "dns"}
	ErrConnect           = &Error{Class: "connect"}
	ErrTimeout           = &Error{Class: "timeout"}
	ErrTLS               = &Error{Class: "tls"}
	ErrCanceled          = &Error{Class: "canceled"}
	ErrTooLarge          = &Error{Class: "too-large"}          // the response passed its size cap
	ErrParse             = &Error{Class: "parse"}              // not the JSON shape the portal documents
	ErrCategoryRequired  = &Error{Class: "category-required"}  // an empty category id: the whole portal (prototype: 7 001 rows)
	ErrImageForeignHost  = &Error{Class: "image-foreign-host"} // ADR 0067 §2 decision 2: dropped, never fetched
	ErrImageInvalidURL   = &Error{Class: "image-invalid-url"}  // unparseable / non-https on another host / no path
	ErrMissingCredential = &Error{Class: "missing-credential"} // no key to send
	ErrKeepInvalid       = &Error{Class: "keep-invalid"}       // Articles asked to keep < 1 or > MaxKeepPerCategory
	ErrHeldLookup        = &Error{Class: "held-lookup"}        // the caller's HeldFunc failed, or none was given
)

// IsRefusal reports whether err is a class where THIS SIDE refused an outbound destination — a URL, a
// redirect target or a resolved address the guard would not reach. Each is the security event
// `outbound_url_refused` (R2, 02/10/2026). Network failures, statuses and parse errors are not refusals.
func IsRefusal(err error) bool {
	switch ClassOf(err) {
	case ErrURLRefused.Class, ErrAddressRefused.Class, ErrRedirectRefused.Class, ErrTooManyRedirects.Class,
		ErrImageForeignHost.Class, ErrImageInvalidURL.Class:
		return true
	}
	return false
}

// ErrHTTPStatus is the sentinel `errors.Is(err, ErrHTTPStatus)` matches for ANY status.
var ErrHTTPStatus = &Error{Class: "http"}

func httpStatusError(code int) *Error {
	return &Error{Class: "http-" + strconv.Itoa(code), Status: code}
}

// IsHTTPStatus reports the portal's status when err is an `http-<n>` class.
func IsHTTPStatus(err error) (int, bool) {
	var e *Error
	if errors.As(err, &e) && e.Status != 0 {
		return e.Status, true
	}
	return 0, false
}

// ClassOf returns the class of any error this package returned; `internal` for anything else (which
// would be a defect here, and still says nothing about the URL).
func ClassOf(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Class
	}
	return "internal"
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
