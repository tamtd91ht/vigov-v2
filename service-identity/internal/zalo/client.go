// Package zalo exchanges the tokens a Zalo Mini App hands its back end: an accessToken for
// the per-app Zalo account id, and an accessToken + phoneToken pair for the citizen's phone
// number.
//
// WHERE THIS COMES FROM: this is ADR 0066's port of vihat-miniapp's internal/zalo (client.go,
// wire.go, ma_tai_khoan.go). Since ADR 0066 a commune's OWN Mini App signs in through ViGov
// service-identity, so the exchange signed with that App ID's secret has to live here (which
// repo owns a Zalo surface is decided by which secret signs it — ADR 0032). The SHARED ViHAT app
// keeps vihat-miniapp's copy. Two copies of one protocol can drift: a change to the wire shape
// in either repo must be made in both, and wire.go here is the only file in this repo that
// describes it.
//
// ORDER — AccountID FIRST, THEN Phone. The phoneToken that getPhoneNumber() returns may be
// single-use and short-lived (Zalo does not document it; the location token of the same
// family is reported as one-shot). The account-id call takes no secret and no phoneToken, so
// it can be repeated freely, and it rejects a bad accessToken before anything is spent. Call
// Phone first and a transient failure of the account-id call afterwards leaves the citizen
// with a burnt phoneToken: the retry needs a fresh consent prompt, and a silent re-open of the
// app cannot produce one.
//
// WHAT "APP ID VERIFIED" MEANS, AND WHAT IT RESTS ON. Zalo gives no way to ask "which app is
// this accessToken for": the token is opaque, and /v2.0/me takes no secret and returns no app.
// Only /v2.0/me/info takes a secret_key. So "App ID verified" means exactly this: Phone,
// called with THAT App ID's secret, succeeded in the same request. That is only a proof if
// Zalo refuses app X's tokens when exchanged with app Y's secret — which has NOT been measured
// under control (ADR 0045 UNKNOWN #1; evidence points both ways, see vihat-miniapp
// internal/httpapi/app_zalo.go:22-28). If Zalo does not check, somebody naming commune B's App
// ID only reaches commune B — what B's public QR already offers — and reads nobody else's
// records; the consequence is bounded there (ADR 0045), not here.
//
// PERSONAL DATA: the account id and the phone number are personal data (rule 3); the tokens
// and the app secret are credentials. None of them is ever logged, put in an error, or wrapped
// from Zalo's response. Errors carry only an HTTP status or Zalo's NUMERIC error code; Zalo's
// "message" text is not even decoded, because it is a string this system does not control.
package zalo

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/vihat/vigov/core/secret"
)

// Exactly two error classes, because the HTTP layer answers two different codes and tells the
// citizen two different things:
//
//	ErrTokenInvalid    -> 401, "open the app again and sign in"  (the session's side)
//	ErrZaloUnreachable -> 502, "try again in a few minutes"      (the infrastructure's side)
//
// Merging them tells a citizen to sign in again while the fault is ours — they retry forever
// and never get in.
var (
	ErrTokenInvalid    = errors.New("zalo: token invalid or expired")
	ErrZaloUnreachable = errors.New("zalo: Zalo service unreachable")
)

const (
	// defaultTimeout bounds one whole call, connect to last byte.
	defaultTimeout = 8 * time.Second
	// bodyLimit stops a huge response (or an impostor server) from eating the memory.
	bodyLimit = 64 << 10
)

// Client calls Zalo's Graph API. It holds NO app secret: one ViGov process serves the own apps
// of many communes, so the secret of the right App ID is passed per call to Phone.
type Client struct {
	baseURL string
	hc      *http.Client

	// sendAppSecretProof — see UNKNOWN #1 in wire.go. OFF BY DEFAULT: there is no evidence the
	// Mini App flow needs it, and an extra header signed the wrong way could break a call that
	// works today. It is a switch to flip when testing against real Zalo, not a decision.
	sendAppSecretProof bool
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient replaces the http.Client (tests, or a proxy). The replacement's Timeout is
// the overall bound; the 6 s bound of AccountID still applies on top of it.
func WithHTTPClient(hc *http.Client) Option { return func(c *Client) { c.hc = hc } }

// WithAppSecretProof turns on the appsecret_proof header on Phone. Default off — see the
// field comment and UNKNOWN #1 in wire.go.
func WithAppSecretProof() Option { return func(c *Client) { c.sendAppSecretProof = true } }

// New builds a client. An empty baseURL means the real server (DefaultBaseURL); tests inject
// an httptest server.
func New(baseURL string, opts ...Option) *Client {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	c := &Client{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		hc:      &http.Client{Timeout: defaultTimeout},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Phone exchanges phoneToken, using the secret of the App ID the token belongs to, and returns
// the number normalised to 84xxxxxxxxx.
//
// Success here is what "App ID verified" means — see the package comment for what that rests
// on. Call AccountID first (package comment, ORDER).
//
// Parameters and result are NEVER logged, and the response body never enters an error: it
// holds the phone number, and an error carrying it is personal data in central logging.
func (c *Client) Phone(ctx context.Context, accessToken, phoneToken string, appSecret secret.Secret) (string, error) {
	if accessToken == "" || phoneToken == "" {
		return "", fmt.Errorf("zalo: missing accessToken or phoneToken: %w", ErrTokenInvalid)
	}
	if appSecret.Rong() {
		// Our configuration, not the citizen's token: sending an empty secret_key would come
		// back as error != 0 and be reported to the citizen as "sign in again" — forever.
		return "", fmt.Errorf("zalo: no app secret configured: %w", ErrZaloUnreachable)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+InfoPath, nil)
	if err != nil {
		return "", fmt.Errorf("zalo: build request: %w", ErrZaloUnreachable)
	}
	req.Header.Set(HeaderAccessToken, accessToken)
	req.Header.Set(HeaderCode, phoneToken)
	req.Header.Set(HeaderSecretKey, string(appSecret.Lo()))
	if c.sendAppSecretProof {
		req.Header.Set(HeaderAppSecretProof, AppSecretProof(accessToken, appSecret))
	}

	body, err := c.do(req)
	if err != nil {
		return "", err
	}

	var env infoEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		// The body is deliberately NOT attached: it may hold the number.
		return "", fmt.Errorf("zalo: response is not the expected JSON: %w", ErrZaloUnreachable)
	}
	if env.Error != 0 {
		// Only the numeric code; Zalo's message is a string we do not control.
		return "", fmt.Errorf("zalo: error code %d: %w", env.Error, ErrTokenInvalid)
	}
	var raw string
	if env.Data != nil {
		raw = env.Data.Number
	}
	number, err := normalizePhone(raw)
	if err != nil {
		// error == 0 but no usable number: we misread the response, or Zalo changed its shape.
		// Infrastructure side — never tell the citizen to sign in again.
		return "", fmt.Errorf("zalo: %w: %w", err, ErrZaloUnreachable)
	}
	return number, nil
}

// do sends req and returns the (capped) body of a 200 response. Every failure is
// ErrZaloUnreachable; only the HTTP status, or whether it was a timeout, is kept.
func (c *Client) do(req *http.Request) ([]byte, error) {
	resp, err := c.hc.Do(req)
	if err != nil {
		// The transport error is NOT wrapped: it is reduced to "timeout or not". It carries the
		// URL (base + path + fields=id — no token: tokens travel in headers precisely so that
		// they never appear here), but there is nothing in it the caller needs beyond this.
		if errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
			return nil, fmt.Errorf("zalo: call timed out: %w", ErrZaloUnreachable)
		}
		return nil, fmt.Errorf("zalo: call failed: %w", ErrZaloUnreachable)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		// A 4xx at the HTTP layer is still "we called wrong / Zalo refused", not something
		// the citizen can fix -> 502. Only error != 0 in a 200 body means a bad token.
		return nil, fmt.Errorf("zalo: HTTP %d: %w", resp.StatusCode, ErrZaloUnreachable)
	}

	// Read one byte past the cap so an oversized body is REFUSED rather than silently cut to a
	// prefix that happens to be valid JSON.
	body, err := io.ReadAll(io.LimitReader(resp.Body, bodyLimit+1))
	if err != nil {
		return nil, fmt.Errorf("zalo: read response: %w", ErrZaloUnreachable)
	}
	if len(body) > bodyLimit {
		return nil, fmt.Errorf("zalo: response larger than %d bytes: %w", bodyLimit, ErrZaloUnreachable)
	}
	return body, nil
}

func isTimeout(err error) bool {
	var t interface{ Timeout() bool }
	return errors.As(err, &t) && t.Timeout()
}

// AppSecretProof = hex(HMAC-SHA256(key = app secret, message = accessToken)).
//
// EVIDENCE LEVEL: LOW — the familiar shape of the Graph API family; NOT confirmed that Zalo
// signs exactly this string. Do not enable WithAppSecretProof before testing on real Zalo.
func AppSecretProof(accessToken string, appSecret secret.Secret) string {
	mac := hmac.New(sha256.New, appSecret.Lo())
	mac.Write([]byte(accessToken))
	return hex.EncodeToString(mac.Sum(nil))
}
