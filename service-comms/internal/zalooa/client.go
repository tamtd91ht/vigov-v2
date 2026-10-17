// Package zalooa calls the Zalo OA APIs a commune's OA is reached through once it has authorized the
// platform's shared Zalo App (ADR 0098, block A): OAuth v4 for OA (the permission link, the token
// exchange, the refresh), `getoa` on the Open API, and the ZBS template list / detail on the Business
// API. It is an ADAPTER: no business rule, no store, no logging.
//
// THE DISCIPLINE IS internal/zalobot's (ADR 0098 #13 "khuôn internal/zalobot"), for a different reason.
// The bot's token sits in the URL; here the access token travels in a HEADER and the app secret in the
// `secret_key` header — but the authorization `code`, the PKCE verifier and the refresh token are
// one-time credentials that a response body or an error string may echo, and a Go *url.Error prints
// the full request URL. So:
//
//   - NO METHOD RETURNS AN error. Every call answers an Outcome — a CLASS — and nothing else about a
//     failure leaves this package. The class is what the use case acts on (0031 outcome / check
//     columns); there is no string to leak because none is kept.
//   - NOTHING HERE LOGS, and no request, URL, header or body is ever rendered.
//   - REDIRECTS ARE NOT FOLLOWED. A 3xx would send the request — secret and token in the headers —
//     wherever the Location says.
//
// WHAT CROSSES TO THE THIRD PARTY (rule 3, invariant 6 — declared here, in the adapter):
//
//	permission link   app_id; the platform's callback URL; the PKCE code_challenge; `state` (CSPRNG,
//	                  opaque — it names nothing). Built here, opened by the OA admin's browser.
//	access_token      app_id; the app secret (`secret_key` header); the authorization `code` and the
//	                  code_verifier (exchange), or the refresh token (refresh). Nothing else.
//	getoa             the commune's access token (`access_token` header). Nothing else.
//	template/all      the commune's access token; offset and limit.
//	template/info     the commune's access token; the template id.
//	message/template  the commune's access token; `phone` (PERSONAL DATA — the recipient's number in
//	                  Zalo's 84… form; sending it to Zalo is ADR 0018 / ADR 0099 #6), `template_id`,
//	                  `template_data` (the values staff typed for the WHOLE campaign — no value comes from
//	                  a recipient's record, ADR 0099 #6), `tracking_id` (the recipient ROW id — opaque).
//	                  Nothing else: no name, no address.
//
// message/template IS THE ONE CALL HERE THAT CARRIES PERSONAL DATA (block D-5, the campaign sender).
// The number lives in the request body only; this package never renders, logs or returns it, and an
// error carries none of it (there is no error — an Outcome).
//
// TRANSPORT SECURITY IS NOT CONFIGURABLE DOWNWARDS (rule 13 #1): https only, certificates always
// verified, TLS 1.2 minimum, no InsecureSkipVerify anywhere. The three base URLs are platform constants
// in code, not variables: there is one Zalo.
//
// NO PROXY FROM THE ENVIRONMENT: http.ProxyFromEnvironment reads HTTPS_PROXY outside core/config
// (rule 11, forbidden #1). Egress is direct.
package zalooa

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/vihat/vigov/core/secret"
)

// The three Zalo hosts (ADR 0098 §Kết quả tra tài liệu #1, #3, #8, #9). Constants, not variables.
const (
	OAuthBaseURL    = "https://oauth.zaloapp.com/v4"
	OpenAPIBaseURL  = "https://openapi.zalo.me/v2.0"
	BusinessBaseURL = "https://business.openapi.zalo.me"
)

// DefaultTimeout bounds one whole call — a member of staff is waiting behind "Kiểm tra" / "Nạp mẫu",
// and the refresh job must not hold its lock on one commune for longer.
const DefaultTimeout = 15 * time.Second

// maxBody bounds what is read from Zalo. A template list of 100 entries is a few tens of KiB.
const maxBody = 256 << 10

// MaxTemplatePageSize is template/all's largest `limit` (ADR 0098 §Kết quả tra #9: "limit≤100").
const MaxTemplatePageSize = 100

// codeVerifierLength is the PKCE verifier length this package generates: 43 characters, RFC 7636's
// minimum, from an unreserved alphabet — the length Zalo's own PHP SDK (zaloplatform/zalo-php-sdk,
// PKCEUtil) produces.
const codeVerifierLength = 43

// Outcome is the class of one call's result. It is the whole report of a call: there is no error and
// no text beside it (see the package comment). Same six classes as internal/zalobot, so the use case
// stores them under the same 0031 / 0022 vocabulary.
type Outcome uint8

const (
	// OutcomeOK — Zalo accepted the call and answered in a shape this package reads.
	OutcomeOK Outcome = iota + 1
	// OutcomeTokenRejected — Zalo refused the credential presented: the access token (getoa, ZBS), or
	// the grant (authorization code / refresh token) at the token endpoint. Not retryable.
	OutcomeTokenRejected
	// OutcomeRateLimited — 429. RETRYABLE.
	OutcomeRateLimited
	// OutcomeUnavailable — timeout, network failure, 408, 5xx. RETRYABLE. AMBIGUOUS for a token
	// exchange: Zalo may have spent the code.
	OutcomeUnavailable
	// OutcomeRejected — any other refusal (another 4xx, a non-zero `error` outside the token classes,
	// a redirect). Final.
	OutcomeRejected
	// OutcomeMalformedResponse — Zalo answered 2xx with something this package cannot read. Final.
	OutcomeMalformedResponse
	// OutcomeRecipientRejected — message/template only: Zalo refused THIS phone number (not a Zalo user,
	// cannot receive ZNS, or not a valid number), or the number is not one this package can put in Zalo's
	// 84… form. Final for that recipient; says nothing about the OA or the template.
	OutcomeRecipientRejected
)

// Retryable — true for RateLimited and Unavailable, false for everything else, OK included.
func (o Outcome) Retryable() bool {
	return o == OutcomeRateLimited || o == OutcomeUnavailable
}

// String names the class — for a metric label or a test message. Never anything from Zalo.
func (o Outcome) String() string {
	switch o {
	case OutcomeOK:
		return "ok"
	case OutcomeTokenRejected:
		return "token_rejected"
	case OutcomeRateLimited:
		return "rate_limited"
	case OutcomeUnavailable:
		return "unavailable"
	case OutcomeRejected:
		return "rejected"
	case OutcomeMalformedResponse:
		return "malformed_response"
	case OutcomeRecipientRejected:
		return "recipient_rejected"
	}
	return "unspecified"
}

// Tokens is what the token endpoint answers. Both are secrets; the caller seals them and clears these.
// ExpiresIn is ZERO when Zalo's answer carried no readable `expires_in` — the caller applies Zalo's
// documented 25 hours (domain.AccessTokenTTL) rather than this package guessing.
type Tokens struct {
	Access    secret.Secret
	Refresh   secret.Secret
	ExpiresIn time.Duration
}

// OAInfo is what getoa answers that comms uses. Not personal data: an OA is an organisation's account.
type OAInfo struct {
	OAID string
	Name string
}

// TemplateSummary is one entry of template/all.
type TemplateSummary struct {
	ID      string
	Name    string
	Status  string // Zalo's own value, verbatim (ENABLE · PENDING_REVIEW · REJECT · DISABLE · DELETE)
	Quality string // Zalo's own value, verbatim; "" when absent
}

// TemplateDetail is template/info.
type TemplateDetail struct {
	ID      string
	Name    string
	Status  string
	Quality string
	Tag     string
	Params  []Param
}

// Param is one declared template parameter: its NAME (what ADR 0098 #9 compares), whether Zalo marks
// it required, its type and length bound.
type Param struct {
	Name      string
	Required  bool
	Type      string
	MaxLength int
}

// Client is safe for concurrent use.
type Client struct {
	oauth, openapi, business string
	http                     *http.Client
}

// New builds the production client: the three constant hosts, verified TLS, no redirects, no
// environment proxy. timeout <= 0 means DefaultTimeout.
func New(timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	tr := &http.Transport{
		Proxy:               nil,
		TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12},
		ForceAttemptHTTP2:   true,
		MaxIdleConns:        8,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	return newClient(OAuthBaseURL, OpenAPIBaseURL, BusinessBaseURL, &http.Client{Transport: tr, Timeout: timeout})
}

// newClient is New with the hosts and transport supplied — for tests against httptest.NewTLSServer.
// Redirects are refused here, whatever transport is given.
func newClient(oauth, openapi, business string, hc *http.Client) *Client {
	c := *hc
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{oauth: strings.TrimRight(oauth, "/"), openapi: strings.TrimRight(openapi, "/"),
		business: strings.TrimRight(business, "/"), http: &c}
}

// PermissionURL is the link the OA admin opens (ADR 0098 §Kết quả tra #1):
// `https://oauth.zaloapp.com/v4/oa/permission?app_id&redirect_uri&code_challenge&state`. Built from
// parts so every value is escaped; state is the caller's base64url text.
func PermissionURL(appID, callbackURL, codeChallenge, state string) string {
	q := url.Values{}
	q.Set("app_id", appID)
	q.Set("redirect_uri", callbackURL)
	q.Set("code_challenge", codeChallenge)
	q.Set("state", state)
	return OAuthBaseURL + "/oa/permission?" + q.Encode()
}

// NewPKCE generates one code verifier and its challenge (RFC 7636, S256; ADR 0098 §Kết quả tra #1 "mỗi
// yêu cầu một code verifier riêng"). The verifier is 43 characters of [A-Za-z0-9] from crypto/rand
// (rule 13 #2); the challenge is base64url(SHA-256(verifier)) without padding.
func NewPKCE() (verifier secret.Secret, challenge string, err error) {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	raw := make([]byte, codeVerifierLength)
	if _, err := rand.Read(raw); err != nil {
		return nil, "", err
	}
	v := make([]byte, codeVerifierLength)
	for i, b := range raw {
		// 62 does not divide 256: the top of the range is rejected and redrawn, so no character is more
		// likely than another. One byte per draw; a rejected byte is replaced from a fresh read.
		for b >= 248 {
			one := make([]byte, 1)
			if _, err := rand.Read(one); err != nil {
				return nil, "", err
			}
			b = one[0]
		}
		v[i] = alphabet[int(b)%len(alphabet)]
	}
	clear(raw)
	return secret.Secret(v), ChallengeOf(secret.Secret(v)), nil
}

// ChallengeOf is the S256 challenge of a verifier — exported so a test can check NewPKCE's pair.
func ChallengeOf(verifier secret.Secret) string {
	sum := sha256.Sum256(verifier.Lo())
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// ExchangeCode trades the callback's authorization code for the OA's tokens (ADR 0098 §Kết quả tra #3):
// POST oauth/v4/oa/access_token, header `secret_key`, form `app_id`, `grant_type=authorization_code`,
// `code`, `code_verifier`. One code, one try at Zalo.
func (c *Client) ExchangeCode(ctx context.Context, appID string, appSecret secret.Secret, code string,
	verifier secret.Secret) (Tokens, Outcome) {

	if appID == "" || appSecret.Rong() || code == "" || verifier.Rong() {
		return Tokens{}, OutcomeRejected // a caller fault; Zalo is not asked
	}
	form := url.Values{}
	form.Set("app_id", appID)
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("code_verifier", string(verifier.Lo()))
	return c.token(ctx, appSecret, form)
}

// Refresh trades the current refresh token for a new pair (grant_type=refresh_token). Zalo voids the
// presented refresh token and the old access token on success (ADR 0098 §Kết quả tra #4); the caller
// reseals both in ONE transaction.
func (c *Client) Refresh(ctx context.Context, appID string, appSecret, refreshToken secret.Secret) (Tokens, Outcome) {
	if appID == "" || appSecret.Rong() || refreshToken.Rong() {
		return Tokens{}, OutcomeRejected
	}
	form := url.Values{}
	form.Set("app_id", appID)
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", string(refreshToken.Lo()))
	return c.token(ctx, appSecret, form)
}

// token is the one call to the token endpoint. A non-zero `error` in a 2xx body is the GRANT being
// refused — the only inputs besides the grant are the platform's app id and secret, both constants
// validated at boot — so it is TokenRejected, which the use case turns into "needs re-authorization"
// (ADR 0098 #6). A wrong app secret therefore also reads as TokenRejected: a platform-wide fault that
// surfaces on the first refresh after the deploy, not something this package can tell apart without
// reading Zalo's text.
func (c *Client) token(ctx context.Context, appSecret secret.Secret, form url.Values) (Tokens, Outcome) {
	var raw struct {
		Access    string          `json:"access_token"`
		Refresh   string          `json:"refresh_token"`
		ExpiresIn json.RawMessage `json:"expires_in"`
		Error     json.RawMessage `json:"error"`
	}
	hdr := http.Header{"secret_key": []string{string(appSecret.Lo())}}
	o := c.call(ctx, http.MethodPost, c.oauth+"/oa/access_token", hdr, strings.NewReader(form.Encode()),
		"application/x-www-form-urlencoded", classifyTokenStatus, &raw)
	if o != OutcomeOK {
		return Tokens{}, o
	}
	if code, present := errorCode(raw.Error); present && code != 0 {
		return Tokens{}, OutcomeTokenRejected
	}
	if raw.Access == "" || raw.Refresh == "" {
		return Tokens{}, OutcomeMalformedResponse
	}
	t := Tokens{Access: secret.Secret(raw.Access), Refresh: secret.Secret(raw.Refresh)}
	// Zalo prints expires_in as a string of seconds ("90000"); a number is accepted too. Anything
	// unreadable or non-positive is reported as absent (zero), never guessed.
	if s, ok := scalarText(raw.ExpiresIn); ok {
		if n, err := strconv.ParseInt(s, 10, 64); err == nil && n > 0 {
			t.ExpiresIn = time.Duration(n) * time.Second
		}
	}
	return t, OutcomeOK
}

// GetOA asks who the access token belongs to (GET openapi/v2.0/oa/getoa, header `access_token`).
// Read-only at Zalo. The OA's id is kept as text whatever JSON type Zalo sends.
func (c *Client) GetOA(ctx context.Context, accessToken secret.Secret) (OAInfo, Outcome) {
	var raw struct {
		Error json.RawMessage `json:"error"`
		Data  struct {
			OAID json.RawMessage `json:"oa_id"`
			Name string          `json:"name"`
		} `json:"data"`
	}
	o := c.get(ctx, accessToken, c.openapi+"/oa/getoa", nil, &raw)
	if o != OutcomeOK {
		return OAInfo{}, o
	}
	if o := classifyBodyError(raw.Error); o != OutcomeOK {
		return OAInfo{}, o
	}
	id, ok := scalarText(raw.Data.OAID)
	if !ok || id == "" {
		return OAInfo{}, OutcomeMalformedResponse
	}
	return OAInfo{OAID: id, Name: raw.Data.Name}, OutcomeOK
}

// ListTemplates reads one page of the OA's ZBS templates (GET business/template/all?offset&limit).
// limit is clipped to MaxTemplatePageSize. total is Zalo's `metadata.total`; -1 when it did not say.
func (c *Client) ListTemplates(ctx context.Context, accessToken secret.Secret, offset, limit int) ([]TemplateSummary, int, Outcome) {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 || limit > MaxTemplatePageSize {
		limit = MaxTemplatePageSize
	}
	var raw struct {
		Error json.RawMessage `json:"error"`
		Data  []struct {
			ID      json.RawMessage `json:"templateId"`
			Name    string          `json:"templateName"`
			Status  string          `json:"status"`
			Quality string          `json:"templateQuality"`
		} `json:"data"`
		Metadata struct {
			Total json.RawMessage `json:"total"`
		} `json:"metadata"`
	}
	q := url.Values{}
	q.Set("offset", strconv.Itoa(offset))
	q.Set("limit", strconv.Itoa(limit))
	o := c.get(ctx, accessToken, c.business+"/template/all", q, &raw)
	if o != OutcomeOK {
		return nil, 0, o
	}
	if o := classifyBodyError(raw.Error); o != OutcomeOK {
		return nil, 0, o
	}
	out := make([]TemplateSummary, 0, len(raw.Data))
	for _, d := range raw.Data {
		id, ok := scalarText(d.ID)
		if !ok || id == "" {
			return nil, 0, OutcomeMalformedResponse
		}
		out = append(out, TemplateSummary{ID: id, Name: d.Name, Status: d.Status, Quality: d.Quality})
	}
	total := -1
	if s, ok := scalarText(raw.Metadata.Total); ok {
		if n, err := strconv.Atoi(s); err == nil && n >= 0 {
			total = n
		}
	}
	return out, total, OutcomeOK
}

// TemplateInfo reads one template's detail (GET business/template/info?template_id=), including the
// declared parameters. Zalo spells the list `listParams` with `require`; both that and the snake_case
// spellings are read, since the field names were taken from documentation pages, not from a captured
// response (stated so a reviewer knows what to verify against a live OA).
func (c *Client) TemplateInfo(ctx context.Context, accessToken secret.Secret, templateID string) (TemplateDetail, Outcome) {
	if templateID == "" {
		return TemplateDetail{}, OutcomeRejected
	}
	var raw struct {
		Error json.RawMessage `json:"error"`
		Data  struct {
			ID          json.RawMessage `json:"templateId"`
			Name        string          `json:"templateName"`
			Status      string          `json:"status"`
			Quality     string          `json:"templateQuality"`
			Tag         string          `json:"templateTag"`
			ListParams  []rawParam      `json:"listParams"`
			ListParams2 []rawParam      `json:"list_params"`
		} `json:"data"`
	}
	q := url.Values{}
	q.Set("template_id", templateID)
	o := c.get(ctx, accessToken, c.business+"/template/info", q, &raw)
	if o != OutcomeOK {
		return TemplateDetail{}, o
	}
	if o := classifyBodyError(raw.Error); o != OutcomeOK {
		return TemplateDetail{}, o
	}
	id, ok := scalarText(raw.Data.ID)
	if !ok || id == "" {
		id = templateID
	}
	params := raw.Data.ListParams
	if len(params) == 0 {
		params = raw.Data.ListParams2
	}
	d := TemplateDetail{ID: id, Name: raw.Data.Name, Status: raw.Data.Status, Quality: raw.Data.Quality,
		Tag: raw.Data.Tag, Params: make([]Param, 0, len(params))}
	for _, p := range params {
		if p.Name == "" {
			return TemplateDetail{}, OutcomeMalformedResponse
		}
		d.Params = append(d.Params, Param{Name: p.Name, Required: p.required(), Type: p.Type, MaxLength: p.maxLength()})
	}
	return d, OutcomeOK
}

// MaxTrackingIDLen bounds tracking_id. Zalo documents a string; this package only ever sends a row id
// (a ULID is 26), so anything longer is a caller fault refused here.
const MaxTrackingIDLen = 48

// msgIDPattern — what a msg_id must look like before it is handed on: printable ASCII, ≤ 128 (0032
// notice_recipient_provider_id_shape).
func validMsgID(s string) bool {
	if s == "" || len(s) > 128 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '!' || s[i] > '~' {
			return false
		}
	}
	return true
}

// Zalo's numeric errors on message/template that mean "this recipient", not "this OA" or "this template".
// UNCONFIRMED AGAINST A LIVE OA (the same caveat as tokenErrorCodes): taken from the ZNS error list —
// -108 phone number invalid, -118 Zalo account does not exist or is disabled, -119 the account cannot
// receive ZNS. A code outside this set is Rejected (final) — never retried, never blamed on the number.
var recipientErrorCodes = map[int64]bool{-108: true, -118: true, -119: true}

// zaloPhone puts a Vietnamese number into Zalo's form: "0xxxxxxxxx" → "84xxxxxxxxx"; "84…" and "+84…"
// kept (without the plus). ok=false for anything else — the caller's number is not sent.
func zaloPhone(phone string) (string, bool) {
	p := strings.TrimSpace(phone)
	p = strings.TrimPrefix(p, "+")
	switch {
	case strings.HasPrefix(p, "84"):
	case strings.HasPrefix(p, "0"):
		p = "84" + p[1:]
	default:
		return "", false
	}
	if len(p) < 10 || len(p) > 13 {
		return "", false
	}
	for i := 0; i < len(p); i++ {
		if p[i] < '0' || p[i] > '9' {
			return "", false
		}
	}
	return p, true
}

// SendTemplateMessage sends one ZBS template message (POST business.openapi.zalo.me/message/template,
// header `access_token`, JSON {phone, template_id, template_data, tracking_id}) and returns Zalo's
// msg_id on OutcomeOK. `phone` is the national number ("0…"); it is converted to Zalo's 84… form here.
// tracking_id is the recipient ROW id — opaque. THE NUMBER IS NEVER RENDERED: it is placed in the request
// body and nowhere else (see the package comment).
func (c *Client) SendTemplateMessage(ctx context.Context, accessToken secret.Secret, phone, templateID string,
	templateData map[string]string, trackingID string) (string, Outcome) {

	if accessToken.Rong() {
		return "", OutcomeTokenRejected
	}
	if templateID == "" || trackingID == "" || len(trackingID) > MaxTrackingIDLen {
		return "", OutcomeRejected // a caller fault; Zalo is not asked
	}
	p84, ok := zaloPhone(phone)
	if !ok {
		return "", OutcomeRecipientRejected
	}
	if templateData == nil {
		templateData = map[string]string{}
	}
	body, err := json.Marshal(struct {
		Phone        string            `json:"phone"`
		TemplateID   string            `json:"template_id"`
		TemplateData map[string]string `json:"template_data"`
		TrackingID   string            `json:"tracking_id"`
	}{Phone: p84, TemplateID: templateID, TemplateData: templateData, TrackingID: trackingID})
	if err != nil {
		return "", OutcomeRejected
	}
	defer clear(body)
	var raw struct {
		Error json.RawMessage `json:"error"`
		Data  struct {
			MsgID json.RawMessage `json:"msg_id"`
		} `json:"data"`
	}
	hdr := http.Header{"access_token": []string{string(accessToken.Lo())}}
	o := c.call(ctx, http.MethodPost, c.business+"/message/template", hdr, bytes.NewReader(body), "application/json",
		classifyStatus, &raw)
	if o != OutcomeOK {
		return "", o
	}
	if code, present := errorCode(raw.Error); present && code != 0 {
		switch {
		case tokenErrorCodes[code]:
			return "", OutcomeTokenRejected
		case recipientErrorCodes[code]:
			return "", OutcomeRecipientRejected
		}
		return "", OutcomeRejected
	}
	id, ok := scalarText(raw.Data.MsgID)
	if !ok || !validMsgID(id) {
		return "", OutcomeMalformedResponse
	}
	return id, OutcomeOK
}

// rawParam reads a parameter entry in either spelling.
type rawParam struct {
	Name       string          `json:"name"`
	Require    json.RawMessage `json:"require"`
	Required   json.RawMessage `json:"required"`
	Type       string          `json:"type"`
	MaxLength  json.RawMessage `json:"maxLength"`
	MaxLength2 json.RawMessage `json:"max_length"`
}

func (p rawParam) required() bool {
	for _, r := range []json.RawMessage{p.Require, p.Required} {
		if s, ok := scalarText(r); ok {
			return s == "true" || s == "1"
		}
	}
	return false
}

func (p rawParam) maxLength() int {
	for _, r := range []json.RawMessage{p.MaxLength, p.MaxLength2} {
		if s, ok := scalarText(r); ok {
			if n, err := strconv.Atoi(s); err == nil && n >= 0 {
				return n
			}
		}
	}
	return 0
}

// get is a GET with the commune's access token in the `access_token` header.
func (c *Client) get(ctx context.Context, accessToken secret.Secret, endpoint string, q url.Values, out any) Outcome {
	if accessToken.Rong() {
		return OutcomeTokenRejected
	}
	if len(q) > 0 {
		endpoint += "?" + q.Encode()
	}
	hdr := http.Header{"access_token": []string{string(accessToken.Lo())}}
	return c.call(ctx, http.MethodGet, endpoint, hdr, http.NoBody, "", classifyStatus, out)
}

// call is the one place a request is made. It returns only a class (see the package comment): the
// request, its URL, its headers and every error built on the way stay inside this function.
func (c *Client) call(ctx context.Context, method, endpoint string, hdr http.Header, body io.Reader,
	contentType string, classify func(int) Outcome, out any) Outcome {

	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return OutcomeUnavailable
	}
	for k, v := range hdr {
		req.Header[k] = v // Zalo's header names are lower-case with an underscore; canonicalising would rename them
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Accept", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		// Timeout, refused connection, TLS failure, cancelled context — the error text names the URL,
		// so it is not kept. All of them mean "try later".
		return OutcomeUnavailable
	}
	defer res.Body.Close()
	b, err := io.ReadAll(io.LimitReader(res.Body, maxBody+1))
	if err != nil {
		return OutcomeUnavailable
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return classify(res.StatusCode)
	}
	if len(b) > maxBody {
		return OutcomeMalformedResponse
	}
	b = bytes.TrimSpace(b)
	if len(b) == 0 || b[0] != '{' {
		return OutcomeMalformedResponse
	}
	if err := json.Unmarshal(b, out); err != nil {
		return OutcomeMalformedResponse
	}
	return OutcomeOK
}

// classifyStatus maps an HTTP status of the Open API / Business API onto a class.
//
//	401, 403   the access token is not accepted: TokenRejected
//	429        RateLimited (retryable)
//	408, 5xx   Unavailable (retryable)
//	3xx        a redirect, not followed: Rejected
//	other      Rejected
func classifyStatus(code int) Outcome {
	switch {
	case code == http.StatusUnauthorized || code == http.StatusForbidden:
		return OutcomeTokenRejected
	case code == http.StatusTooManyRequests:
		return OutcomeRateLimited
	case code == http.StatusRequestTimeout || (code >= 500 && code <= 599):
		return OutcomeUnavailable
	}
	return OutcomeRejected
}

// classifyTokenStatus is classifyStatus for the token endpoint, where a 400 is the grant being refused
// (an invalid or spent code, a voided refresh token) — see token().
func classifyTokenStatus(code int) Outcome {
	if code == http.StatusBadRequest {
		return OutcomeTokenRejected
	}
	return classifyStatus(code)
}

// Zalo's own numeric error codes that mean "the access token is not accepted" on the Open API and the
// Business API. UNCONFIRMED AGAINST A LIVE OA at the time of writing (no network from the build
// machine): -216 "access token invalid" and -124 "access token expired" are the codes the ZNS error
// list names, and nothing else is assumed. A non-zero code outside this set is Rejected (final), which
// is the safe side: it never marks a commune "needs re-authorization" on a code this package did not
// recognise. HTTP 401/403 classify the same way regardless of the body.
var tokenErrorCodes = map[int64]bool{-216: true, -124: true}

// classifyBodyError reads the `error` field of a 2xx Open API / Business API body: absent or 0 is OK.
func classifyBodyError(raw json.RawMessage) Outcome {
	code, present := errorCode(raw)
	switch {
	case !present || code == 0:
		return OutcomeOK
	case tokenErrorCodes[code]:
		return OutcomeTokenRejected
	}
	return OutcomeRejected
}

// errorCode reads a numeric `error` (number or numeric string). present=false when absent or unreadable.
func errorCode(raw json.RawMessage) (int64, bool) {
	s, ok := scalarText(raw)
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

// scalarText renders a JSON string, number or bool as text ("123", "abc", "true"). ok=false for
// anything else.
func scalarText(raw json.RawMessage) (string, bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return "", false
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return "", false
		}
		return s, true
	}
	if string(raw) == "true" || string(raw) == "false" {
		return string(raw), true
	}
	var n json.Number
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&n); err != nil {
		return "", false
	}
	return n.String(), true
}
