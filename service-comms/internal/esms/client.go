// Package esms calls the eSMS (ViHAT) REST API a commune's SMS account is reached through (ADR 0099 #2,
// #6): get balance (the connection check), the CSKH template list, the CSKH price estimate, and the CSKH
// send. It is an ADAPTER: no business rule, no store, no logging. Read from developers.esms.vn on
// 17/10/2026 (ADR 0099 §eSMS); every endpoint, field name and code below is from those pages, not from a
// captured response — stated so a reviewer knows what to verify against a live account.
//
// THE DISCIPLINE IS internal/zalooa's, for a sharper reason: EVERY eSMS REQUEST CARRIES THE ACCOUNT'S
// ApiKey AND SecretKey IN CLEAR — in the JSON body (balance, templates, send) or, for the price estimate,
// in the QUERY STRING (eSMS documents that call as GET only). A Go *url.Error prints the full URL, and a
// request body dumped while debugging prints both keys. So:
//
//   - NO METHOD RETURNS AN error. Every call answers an Outcome — a CLASS — and nothing else about a
//     failure leaves this package: not eSMS's ErrorMessage, not a status line, not a URL.
//   - NOTHING HERE LOGS, and no request, URL, header or body is ever rendered. Message refuses to print
//     its phone number and content (Format / LogValue).
//   - REDIRECTS ARE NOT FOLLOWED. A 3xx would resend the body — both keys and a citizen's number —
//     wherever the Location says.
//
// WHAT CROSSES TO THE THIRD PARTY (rule 3, invariant 6 — declared here, in the adapter):
//
//	GetBalance_json                 ApiKey, SecretKey. Nothing else.
//	GetTemplate                     ApiKey, SecretKey, SmsType=2, Brandname.
//	SummaryMultipleMessage_V4_get   ApiKey, SecretKey, SmsType=2, Brandname, Content (the template text
//	                                with the values staff typed for the whole campaign), IsUnicode, and
//	                                Phone = EstimateProbePhone — the agreed FAKE number, never a recipient's:
//	                                this call is a GET, and a real number in a URL is rule 3, forbidden #4.
//	SendMultipleMessage_V4_post_json  ApiKey, SecretKey, SmsType=2, Brandname, Phone (PERSONAL DATA — the
//	                                owner approved sending it to eSMS on 17/10/2026, ADR 0099 #6, rule 3
//	                                stop condition #2), Content (the template filled with the values staff
//	                                typed for the WHOLE campaign — no value comes from a recipient's record,
//	                                ADR 0099 #6), IsUnicode, RequestId (the recipient ROW id — opaque),
//	                                campaignid (the campaign ROW id — never its title, which is free text),
//	                                Sandbox. Never CallbackUrl (ADR 0099 #9: the delivery callback is a later
//	                                round) and never SendDate.
//
// TRANSPORT SECURITY IS NOT CONFIGURABLE DOWNWARDS (rule 13 #1): https only, certificates always
// verified, TLS 1.2 minimum. The base URL is a platform constant in code: there is one eSMS.
//
// NO PROXY FROM THE ENVIRONMENT: http.ProxyFromEnvironment reads HTTPS_PROXY outside core/config
// (rule 11, forbidden #1). Egress is direct.
package esms

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/vihat/vigov/core/secret"
)

// BaseURL is eSMS's REST host (ADR 0099 §eSMS). A constant, not a variable.
const BaseURL = "https://rest.esms.vn/MainService.svc/json"

// DefaultTimeout bounds one whole call — a member of staff is waiting behind "Kiểm tra" / "Nạp mẫu",
// and the campaign sender must not hold its lease on one commune for longer.
const DefaultTimeout = 15 * time.Second

// maxBody bounds what is read from eSMS. A template list of a few hundred entries is tens of KiB.
const maxBody = 256 << 10

// smsTypeCSKH is eSMS's SmsType for a customer-care (CSKH) message — the only type this package sends.
const smsTypeCSKH = "2"

// MaxRequestIDLen is eSMS's bound on RequestId ("Độ dài tối đa 50 ký tự"). A ULID is 26.
const MaxRequestIDLen = 50

// maxCampaignIDLen bounds campaignid. eSMS documents no limit; this package only ever sends a row id.
const maxCampaignIDLen = 64

// EstimateProbePhone is the number the price estimate is asked for: the AGREED FAKE NUMBER of this
// repository (rule 3, invariant 5), on a MobiFone prefix. eSMS's estimate is a GET that wants a Phone in
// the query string; a recipient's real number there would be personal data in a URL (rule 3, forbidden
// #4), so the estimate is per message for one probe recipient and the caller multiplies. The price eSMS
// quotes for another network may differ — an ESTIMATE, which is all ADR 0099 #7 asks for.
const EstimateProbePhone = "0900000000"

// Outcome is the class of one call's result. It is the whole report of a call: there is no error and
// no text beside it (see the package comment).
type Outcome uint8

const (
	// OutcomeOK — CodeResult 100: eSMS ACCEPTED the request. For a send this is "eSMS received it", not
	// "the handset received it" (ADR 0099 #9).
	OutcomeOK Outcome = iota + 1
	// OutcomeKeyRejected — CodeResult 101: ApiKey / SecretKey refused. Final.
	OutcomeKeyRejected
	// OutcomeBrandnameInactive — CodeResult 104: the Brandname does not exist on the account or is not
	// active. Final.
	OutcomeBrandnameInactive
	// OutcomeDuplicateRequest — CodeResult 124: eSMS already holds a request with this RequestId (its
	// duplicate block lasts 24 hours). THE CALLER TREATS IT AS ACCEPTED (Accepted() is true): RequestId is
	// the recipient row's id and is only ever re-sent after an AMBIGUOUS attempt (Unavailable), so the
	// earlier attempt is the one that reached eSMS. No SMSID comes back with it. Final.
	OutcomeDuplicateRequest
	// OutcomeTemplateMismatch — CodeResult 146: the content matches no CSKH template registered for the
	// Brandname. Final.
	OutcomeTemplateMismatch
	// OutcomeInvalidRequest — CodeResult 99, or a body `{"error": …}` without a code (the send page's "99"
	// tab). eSMS's two pages disagree on what 99 means — "Invalid request" on the send page, "the number is
	// blocked" in the error table — and both are final, so they share this class.
	OutcomeInvalidRequest
	// OutcomeRejected — any other CodeResult (102 no price list, 103 balance too low, 140 IP not
	// whitelisted, 143 account locked, 177 network not registered, 201 blocked keyword, …), a non-2xx
	// status outside the retryable set, or a redirect. Final.
	OutcomeRejected
	// OutcomeUnavailable — timeout, network failure, 408, 429, 5xx. RETRYABLE. AMBIGUOUS for a send:
	// eSMS may have taken it — which is what RequestId is for.
	OutcomeUnavailable
	// OutcomeMalformedResponse — eSMS answered 2xx with something this package cannot read. Final.
	OutcomeMalformedResponse
)

// Retryable — true only for Unavailable.
func (o Outcome) Retryable() bool { return o == OutcomeUnavailable }

// Accepted — true when eSMS holds the request: OK, or the duplicate-RequestId answer (see
// OutcomeDuplicateRequest for why that one counts).
func (o Outcome) Accepted() bool { return o == OutcomeOK || o == OutcomeDuplicateRequest }

// String names the class — for a metric label or a test message. Never anything from eSMS.
func (o Outcome) String() string {
	switch o {
	case OutcomeOK:
		return "ok"
	case OutcomeKeyRejected:
		return "key_rejected"
	case OutcomeBrandnameInactive:
		return "brandname_inactive"
	case OutcomeDuplicateRequest:
		return "duplicate_request"
	case OutcomeTemplateMismatch:
		return "template_mismatch"
	case OutcomeInvalidRequest:
		return "invalid_request"
	case OutcomeRejected:
		return "rejected"
	case OutcomeUnavailable:
		return "unavailable"
	case OutcomeMalformedResponse:
		return "malformed_response"
	}
	return "unspecified"
}

// Credentials are one eSMS account's keys. Both are secrets: they render as a mask and are cleared by
// the caller after the call.
type Credentials struct {
	APIKey    secret.Secret
	SecretKey secret.Secret
}

func (c Credentials) empty() bool { return c.APIKey.Rong() || c.SecretKey.Rong() }

// Template is one entry of GetTemplate's BrandnameTemplates, verbatim: eSMS's TempId on one network,
// and the registered text with its `{P1,20}` placeholders.
type Template struct {
	ID        string // TempId, as text whatever JSON type eSMS sends
	NetworkID int    // 1 Viettel · 2 Mobi · 3 Vinaphone · 4 Vietnamobile · 5 Gtel · 6 Itel · 7 Reddi (eSMS's list)
	Content   string
}

// Message is one CSKH SMS to one number. Phone and Content never render (Format / LogValue): the
// first is personal data, the second may carry what staff typed.
type Message struct {
	Phone      string
	Content    string
	Brandname  string
	IsUnicode  bool
	RequestID  string // the recipient row id: eSMS refuses a second request with it for 24 hours
	CampaignID string // the campaign row id — never its title
}

// Format prints the message without its phone number and content, whatever the verb.
func (m Message) Format(f fmt.State, _ rune) {
	_, _ = io.WriteString(f, "esms.Message{Brandname:"+m.Brandname+" RequestID:"+m.RequestID+" Phone:"+
		secret.Che+" Content:"+secret.Che+"}")
}

// LogValue keeps the phone number and content out of a structured log line.
func (m Message) LogValue() slog.Value {
	return slog.GroupValue(slog.String("brandname", m.Brandname), slog.String("request_id", m.RequestID))
}

// Client is safe for concurrent use.
type Client struct {
	base string
	http *http.Client
	// sandbox sends every message with Sandbox=1 (eSMS: "không về tin, không trừ tiền"). ALWAYS false from
	// New: whether a dev deployment should send in sandbox is an open decision (rule 11, stop condition
	// #1 — it would be a new variable), not one this package takes.
	sandbox bool
}

// New builds the production client: the constant host, verified TLS, no redirects, no environment
// proxy, sandbox off. timeout <= 0 means DefaultTimeout.
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
	return newClient(BaseURL, &http.Client{Transport: tr, Timeout: timeout})
}

// newClient is New with the host and transport supplied — for tests against httptest.NewTLSServer.
// Redirects are refused here, whatever transport is given.
func newClient(base string, hc *http.Client) *Client {
	c := *hc
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{base: strings.TrimRight(base, "/"), http: &c}
}

// balancePattern is what a balance must look like before it is handed on: a decimal amount that fits
// 0032's NUMERIC(16, 2) (14 integer digits). eSMS's account may be post-paid, so a sign is allowed.
var balancePattern = regexp.MustCompile(`^-?[0-9]{1,14}(\.[0-9]+)?$`)

// GetBalance asks eSMS for the account's main balance (POST GetBalance_json) — the connection check
// (ADR 0099 #2). The balance is returned as eSMS's decimal text ("36693"), never a float. Note the field:
// this endpoint answers `CodeResponse`, not `CodeResult`.
func (c *Client) GetBalance(ctx context.Context, creds Credentials) (string, Outcome) {
	if creds.empty() {
		return "", OutcomeKeyRejected // nothing to present; eSMS is not asked
	}
	body := map[string]string{"ApiKey": string(creds.APIKey.Lo()), "SecretKey": string(creds.SecretKey.Lo())}
	var raw struct {
		Code    json.RawMessage `json:"CodeResponse"`
		Code2   json.RawMessage `json:"CodeResult"`
		Error   json.RawMessage `json:"error"`
		Balance json.RawMessage `json:"Balance"`
	}
	o := c.post(ctx, "/GetBalance_json", body, &raw)
	clearMap(body)
	if o != OutcomeOK {
		return "", o
	}
	code := raw.Code
	if len(bytes.TrimSpace(code)) == 0 {
		code = raw.Code2
	}
	if o := classifyCode(code, raw.Error); o != OutcomeOK {
		return "", o
	}
	b, ok := scalarText(raw.Balance)
	if !ok || !balancePattern.MatchString(b) {
		return "", OutcomeMalformedResponse
	}
	return b, OutcomeOK
}

// ListCSKHTemplates reads the CSKH templates registered for brandname on the account (POST GetTemplate,
// SmsType=2) — one entry per (TempId, NetworkID), exactly as eSMS lists them.
func (c *Client) ListCSKHTemplates(ctx context.Context, creds Credentials, brandname string) ([]Template, Outcome) {
	if creds.empty() {
		return nil, OutcomeKeyRejected
	}
	if brandname == "" {
		return nil, OutcomeBrandnameInactive
	}
	// eSMS spells the field `Brandname` in the curl sample and `BrandName` in the table; JSON keys are
	// case-sensitive in .NET's WCF binder only by configuration, so the sample's spelling is sent.
	body := map[string]string{"ApiKey": string(creds.APIKey.Lo()), "SecretKey": string(creds.SecretKey.Lo()),
		"SmsType": smsTypeCSKH, "Brandname": brandname}
	var raw struct {
		Code      json.RawMessage `json:"CodeResult"`
		Error     json.RawMessage `json:"error"`
		Templates []struct {
			NetworkID json.RawMessage `json:"NetworkID"`
			Content   string          `json:"TempContent"`
			ID        json.RawMessage `json:"TempId"`
		} `json:"BrandnameTemplates"`
	}
	o := c.post(ctx, "/GetTemplate/", body, &raw)
	clearMap(body)
	if o != OutcomeOK {
		return nil, o
	}
	if o := classifyCode(raw.Code, raw.Error); o != OutcomeOK {
		return nil, o
	}
	out := make([]Template, 0, len(raw.Templates))
	for _, t := range raw.Templates {
		id, ok := scalarText(t.ID)
		if !ok || id == "" {
			return nil, OutcomeMalformedResponse
		}
		ns, ok := scalarText(t.NetworkID)
		if !ok {
			return nil, OutcomeMalformedResponse
		}
		n, err := strconv.Atoi(ns)
		if err != nil || n <= 0 {
			return nil, OutcomeMalformedResponse
		}
		out = append(out, Template{ID: id, NetworkID: n, Content: t.Content})
	}
	return out, OutcomeOK
}

// EstimateCSKH asks eSMS what ONE message of content would cost for EstimateProbePhone (GET
// SummaryMultipleMessage_V4_get) and returns eSMS's TotalPrice as decimal text. The caller multiplies by
// the number of recipients. THE KEYS TRAVEL IN THE QUERY STRING — eSMS documents no POST form — so the
// URL built here is never rendered anywhere (call()).
func (c *Client) EstimateCSKH(ctx context.Context, creds Credentials, brandname, content string, isUnicode bool) (string, Outcome) {
	if creds.empty() {
		return "", OutcomeKeyRejected
	}
	if brandname == "" || content == "" {
		return "", OutcomeInvalidRequest
	}
	q := url.Values{}
	q.Set("Phone", EstimateProbePhone)
	q.Set("Content", content)
	q.Set("ApiKey", string(creds.APIKey.Lo()))
	q.Set("SecretKey", string(creds.SecretKey.Lo()))
	q.Set("SmsType", smsTypeCSKH)
	q.Set("BrandName", brandname)
	q.Set("IsUnicode", flag(isUnicode))
	var raw struct {
		Code  json.RawMessage `json:"CodeResult"`
		Error json.RawMessage `json:"error"`
		Price json.RawMessage `json:"TotalPrice"`
	}
	endpoint := c.base + "/SummaryMultipleMessage_V4_get?" + q.Encode()
	o := c.call(ctx, http.MethodGet, endpoint, nil, &raw)
	if o != OutcomeOK {
		return "", o
	}
	if o := classifyCode(raw.Code, raw.Error); o != OutcomeOK {
		return "", o
	}
	p, ok := scalarText(raw.Price)
	if !ok || !balancePattern.MatchString(p) || strings.HasPrefix(p, "-") {
		return "", OutcomeMalformedResponse
	}
	return p, OutcomeOK
}

// SendCSKH sends one CSKH message (POST SendMultipleMessage_V4_post_json) and returns eSMS's SMSID on
// OutcomeOK ("" on OutcomeDuplicateRequest, which carries none). A message missing what eSMS requires is
// refused here without asking eSMS (OutcomeInvalidRequest) — a caller fault.
func (c *Client) SendCSKH(ctx context.Context, creds Credentials, m Message) (string, Outcome) {
	if creds.empty() {
		return "", OutcomeKeyRejected
	}
	if m.Phone == "" || m.Content == "" || m.Brandname == "" || m.RequestID == "" || len(m.RequestID) > MaxRequestIDLen ||
		len(m.CampaignID) > maxCampaignIDLen {
		return "", OutcomeInvalidRequest
	}
	body := map[string]string{
		"ApiKey": string(creds.APIKey.Lo()), "SecretKey": string(creds.SecretKey.Lo()),
		"Phone": m.Phone, "Content": m.Content, "Brandname": m.Brandname, "SmsType": smsTypeCSKH,
		"IsUnicode": flag(m.IsUnicode), "RequestId": m.RequestID, "Sandbox": flag(c.sandbox),
	}
	if m.CampaignID != "" {
		body["campaignid"] = m.CampaignID
	}
	var raw struct {
		Code  json.RawMessage `json:"CodeResult"`
		Error json.RawMessage `json:"error"`
		SMSID json.RawMessage `json:"SMSID"`
	}
	o := c.post(ctx, "/SendMultipleMessage_V4_post_json/", body, &raw)
	clearMap(body)
	if o != OutcomeOK {
		return "", o
	}
	if o := classifyCode(raw.Code, raw.Error); o != OutcomeOK {
		return "", o
	}
	id, ok := scalarText(raw.SMSID)
	if !ok || id == "" {
		// 100 without the message id it promises: not the documented shape, so it is reported as such
		// rather than guessed. eSMS may nonetheless hold the request — a repeat with the SAME RequestId
		// then answers 124 (OutcomeDuplicateRequest), never a second message.
		return "", OutcomeMalformedResponse
	}
	return id, OutcomeOK
}

func flag(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// clearMap drops the request map's values once the body is built and sent. The strings themselves are
// immutable in Go and cannot be wiped; this only shortens how long they stay reachable.
func clearMap(m map[string]string) {
	for k := range m {
		delete(m, k)
	}
}

// post is a JSON POST of body to path.
func (c *Client) post(ctx context.Context, path string, body map[string]string, out any) Outcome {
	b, err := json.Marshal(body)
	if err != nil {
		return OutcomeInvalidRequest
	}
	defer clear(b)
	return c.call(ctx, http.MethodPost, c.base+path, b, out)
}

// call is the one place a request is made. It returns only a class (see the package comment): the
// request, its URL, its body and every error built on the way stay inside this function.
func (c *Client) call(ctx context.Context, method, endpoint string, body []byte, out any) Outcome {
	var rd io.Reader = http.NoBody
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, rd)
	if err != nil {
		return OutcomeInvalidRequest
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		// Timeout, refused connection, TLS failure, cancelled context — the error text names the URL (and,
		// for the estimate, both keys in its query), so it is not kept. All of them mean "try later".
		return OutcomeUnavailable
	}
	defer res.Body.Close()
	b, err := io.ReadAll(io.LimitReader(res.Body, maxBody+1))
	if err != nil {
		return OutcomeUnavailable
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return classifyStatus(res.StatusCode)
	}
	if len(b) > maxBody {
		return OutcomeMalformedResponse
	}
	b = bytes.TrimPrefix(bytes.TrimSpace(b), []byte("\xef\xbb\xbf")) // a .NET service may prefix a BOM
	if len(b) == 0 || b[0] != '{' {
		return OutcomeMalformedResponse
	}
	if err := json.Unmarshal(b, out); err != nil {
		return OutcomeMalformedResponse
	}
	return OutcomeOK
}

// classifyStatus maps a non-2xx HTTP status onto a class.
//
//	408, 429, 5xx   Unavailable (retryable)
//	3xx             a redirect, not followed: Rejected
//	other           Rejected
func classifyStatus(code int) Outcome {
	switch {
	case code == http.StatusRequestTimeout || code == http.StatusTooManyRequests || (code >= 500 && code <= 599):
		return OutcomeUnavailable
	}
	return OutcomeRejected
}

// classifyCode maps eSMS's result code (string or number) onto a class. No code but an `error` field is
// the send page's "99" shape; no code and no error is a body this package cannot read.
func classifyCode(code, errField json.RawMessage) Outcome {
	s, ok := scalarText(code)
	if !ok {
		if _, isErr := scalarText(errField); isErr {
			return OutcomeInvalidRequest
		}
		return OutcomeMalformedResponse
	}
	switch strings.TrimSpace(s) {
	case "100":
		return OutcomeOK
	case "101":
		return OutcomeKeyRejected
	case "104":
		return OutcomeBrandnameInactive
	case "124":
		return OutcomeDuplicateRequest
	case "146":
		return OutcomeTemplateMismatch
	case "99":
		return OutcomeInvalidRequest
	}
	return OutcomeRejected
}

// scalarText renders a JSON string or number as text ("123", "abc"). ok=false for anything else.
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
	var n json.Number
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&n); err != nil {
		return "", false
	}
	return n.String(), true
}
