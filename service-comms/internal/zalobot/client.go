// Package zalobot calls the Zalo Bot API (`https://bot-api.zaloplatforms.com/bot<TOKEN>/<method>`,
// ADR 0074) — getMe, setWebhook, getWebhookInfo, sendMessage — and reads the update payload Zalo posts
// to the webhook (update.go). It is an ADAPTER: no business rule, no store, no logging.
//
// THE TOKEN IS IN THE URL PATH. That single fact shapes the whole package (ADR 0074 §Bối cảnh):
//
//   - NO METHOD RETURNS AN error. A Go *url.Error prints the full URL, and so the token; a transport
//     error string may too; a Zalo error body may echo it. Every call answers an Outcome — a CLASS —
//     and nothing else about a failure leaves this package. That is a deliberate redaction, not a
//     swallowed error: the class is what the operator and the dispatcher act on, and there is no
//     string to leak because none is kept.
//   - NOTHING HERE LOGS, and no request or URL is ever rendered. The *http.Request is built and
//     dropped inside one function.
//   - REDIRECTS ARE NOT FOLLOWED. A 3xx would send the request — token in the path — wherever the
//     Location says.
//
// WHAT CROSSES TO THE THIRD PARTY (rule 3, invariant 6 — declared here, in the adapter):
//
//	getMe            the bot token (URL path). Nothing else.
//	getWebhookInfo   the bot token. Nothing else.
//	setWebhook       the bot token; `url` (comms' own public webhook URL); `secret_token` (comms'
//	                 CSPRNG output, which Zalo echoes in X-Bot-Api-Secret-Token).
//	sendMessage      the bot token; `chat_id` (a member of staff's chat with the bot — personal data,
//	                 ADR 0074); `text` (the message, clipped to 2000 characters). The CALLER decides
//	                 what the text says; ADR 0074 #7 fixes that wave 1 sends only a bell notice's title
//	                 and body, which comms.proto keeps free of citizens' personal data.
//
// TRANSPORT SECURITY IS NOT CONFIGURABLE DOWNWARDS (rule 13 #1): https only, certificates always
// verified, TLS 1.2 minimum, no InsecureSkipVerify anywhere. The base URL is a platform constant in
// code, not a variable: there is one Zalo.
//
// NO PROXY FROM THE ENVIRONMENT: http.ProxyFromEnvironment reads HTTPS_PROXY outside core/config
// (rule 11, forbidden #1). Egress is direct to bot-api.zaloplatforms.com:443 (ADR 0074 §Hệ quả).
package zalobot

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/vihat/vigov/core/secret"
)

// BaseURL is the Zalo Bot API. A constant, not a variable: there is one Zalo (ADR 0074).
const BaseURL = "https://bot-api.zaloplatforms.com"

// DefaultTimeout bounds one whole call. An operator is waiting on the other side of the button, and
// the dispatcher must not hold a delivery for longer than this.
const DefaultTimeout = 15 * time.Second

// MaxTextRunes is sendMessage's upper bound ("1–2000 ký tự", ../vigov-require 08-tich-hop-ngoai.md).
const MaxTextRunes = 2000

// maxBody bounds what is read from Zalo. Every answer here is a few hundred bytes.
const maxBody = 64 << 10

// MaxTokenBytes is the longest token accepted (zalo_bot_operator.proto, SetSharedZaloBotRequest.token).
const MaxTokenBytes = 256

// Outcome is the class of one call's result. It is the whole report of a call: there is no error and
// no text beside it (see the package comment). The values map one-to-one onto the contract enum
// ZaloBotCallOutcome, minus NOT_CONFIGURED, which is the caller's to say (no token was there to call
// with).
type Outcome uint8

const (
	// OutcomeOK — Zalo accepted the call and answered in a shape this package reads.
	OutcomeOK Outcome = iota + 1
	// OutcomeTokenRejected — Zalo refused the token (401, 404 on the bot path, or `ok:false` with one
	// of those codes). Set a new token.
	OutcomeTokenRejected
	// OutcomeRateLimited — 429. RETRYABLE.
	OutcomeRateLimited
	// OutcomeUnavailable — timeout, network failure, 408, 5xx. RETRYABLE. AMBIGUOUS for a write: Zalo
	// may have applied it.
	OutcomeUnavailable
	// OutcomeRejected — any other refusal (another 4xx, `ok:false`, a redirect). Final.
	OutcomeRejected
	// OutcomeMalformedResponse — Zalo answered 2xx with something that is neither the flat nor the
	// `result`-wrapped shape. Final, and ambiguous for a write.
	OutcomeMalformedResponse
)

// Retryable is the dispatcher's policy (ADR 0074: "429/408/5xx thử lại có lùi, lỗi khác dừng"): true
// for RateLimited and Unavailable, false for everything else, OK included.
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
	}
	return "unspecified"
}

// BotInfo is what getMe answers that comms uses.
type BotInfo struct {
	// AccountID is the bot's own id — what tells "a new token of the same bot" from "a different bot"
	// (owner, 05/10/2026). Kept as text whatever JSON type Zalo sends.
	AccountID string
	// AccountName is the bot's public display name. Not personal data.
	AccountName string
}

// WebhookInfo is what getWebhookInfo answers.
type WebhookInfo struct {
	// URL is the webhook Zalo has on file; "" means none.
	URL string
	// UpdatedAt is Zalo's own "last updated" instant, ZERO when the answer carries none this package
	// can read unambiguously (zalo_bot_operator.proto: "never a value it inferred").
	UpdatedAt time.Time
}

// Client is safe for concurrent use.
type Client struct {
	base string
	http *http.Client
}

// New builds the production client: BaseURL, verified TLS, no redirects, no environment proxy.
// timeout <= 0 means DefaultTimeout.
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

// newClient is New with the base and transport supplied — for tests against httptest.NewTLSServer.
// Redirects are refused here, whatever transport is given.
func newClient(base string, hc *http.Client) *Client {
	c := *hc
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{base: strings.TrimRight(base, "/"), http: &c}
}

// ValidToken reports whether token may be spliced into the URL path: 1-256 bytes of printable ASCII,
// no whitespace, none of `/ ? # %` (zalo_bot_operator.proto, SetSharedZaloBotRequest.token). Any of
// those would change WHICH URL is called.
func ValidToken(token []byte) bool {
	if len(token) == 0 || len(token) > MaxTokenBytes {
		return false
	}
	for _, b := range token {
		if b <= ' ' || b > '~' {
			return false
		}
		switch b {
		case '/', '?', '#', '%':
			return false
		}
	}
	return true
}

// GetMe asks Zalo who the token belongs to. Read-only at Zalo.
func (c *Client) GetMe(ctx context.Context, token secret.Secret) (BotInfo, Outcome) {
	var raw struct {
		ID          json.RawMessage `json:"id"`
		AccountName string          `json:"account_name"`
	}
	if o := c.call(ctx, token, "getMe", nil, &raw); o != OutcomeOK {
		return BotInfo{}, o
	}
	id, ok := scalarText(raw.ID)
	if !ok || id == "" || len(id) > 128 {
		// No account id = nothing to tell bots apart with. Not something to store a guess for.
		return BotInfo{}, OutcomeMalformedResponse
	}
	return BotInfo{AccountID: id, AccountName: raw.AccountName}, OutcomeOK
}

// GetWebhookInfo asks Zalo where it posts updates. Read-only at Zalo.
func (c *Client) GetWebhookInfo(ctx context.Context, token secret.Secret) (WebhookInfo, Outcome) {
	var raw struct {
		URL       *string         `json:"url"`
		UpdatedAt json.RawMessage `json:"updated_at"`
	}
	if o := c.call(ctx, token, "getWebhookInfo", nil, &raw); o != OutcomeOK {
		return WebhookInfo{}, o
	}
	info := WebhookInfo{}
	if raw.URL != nil {
		info.URL = *raw.URL
	}
	info.UpdatedAt = epochTime(raw.UpdatedAt)
	return info, OutcomeOK
}

// SetWebhook points the bot at webhookURL, signed with secretToken (8-256 characters at Zalo; comms
// sends more than 32 bytes of CSPRNG output). Ambiguous on Unavailable / MalformedResponse: Zalo may
// have applied it.
func (c *Client) SetWebhook(ctx context.Context, token secret.Secret, webhookURL string, secretToken secret.Secret) Outcome {
	if webhookURL == "" || len(secretToken) < 8 || len(secretToken) > 256 {
		return OutcomeRejected // a caller fault; Zalo is not asked
	}
	body := struct {
		URL         string `json:"url"`
		SecretToken string `json:"secret_token"`
	}{URL: webhookURL, SecretToken: string(secretToken.Lo())}
	var ignored json.RawMessage
	return c.call(ctx, token, "setWebhook", body, &ignored)
}

// SendMessage sends text to chatID. text is clipped to MaxTextRunes characters; an empty text or chat
// id is a caller fault, answered Rejected without calling Zalo.
func (c *Client) SendMessage(ctx context.Context, token secret.Secret, chatID, text string) Outcome {
	text = ClipText(text)
	if chatID == "" || text == "" {
		return OutcomeRejected
	}
	body := struct {
		ChatID string `json:"chat_id"`
		Text   string `json:"text"`
	}{ChatID: chatID, Text: text}
	var ignored json.RawMessage
	return c.call(ctx, token, "sendMessage", body, &ignored)
}

// ClipText cuts text to at most MaxTextRunes characters, never inside a character. Invalid UTF-8 is
// replaced rather than sent.
func ClipText(text string) string {
	if !utf8.ValidString(text) {
		text = strings.ToValidUTF8(text, "�")
	}
	if utf8.RuneCountInString(text) <= MaxTextRunes {
		return text
	}
	n := 0
	for i := range text {
		if n == MaxTextRunes {
			return text[:i]
		}
		n++
	}
	return text
}

// call is the one place a request is made. It returns only a class (see the package comment): the
// request, its URL and every error built on the way stay inside this function.
func (c *Client) call(ctx context.Context, token secret.Secret, method string, in any, out any) Outcome {
	if !ValidToken(token) {
		return OutcomeTokenRejected
	}
	// Built from parts, never parsed from a string: url.URL.String escapes the path, and there is no
	// parse error to carry the token anywhere.
	u, err := url.Parse(c.base)
	if err != nil {
		return OutcomeUnavailable
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/bot" + string(token.Lo()) + "/" + method

	var reqBody io.Reader = http.NoBody
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return OutcomeRejected
		}
		reqBody = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), reqBody)
	if err != nil {
		return OutcomeUnavailable
	}
	req.Header.Set("Content-Type", "application/json")
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
		return classifyStatus(res.StatusCode)
	}
	if len(b) > maxBody {
		return OutcomeMalformedResponse
	}
	return decodeEnvelope(b, out)
}

// classifyStatus maps an HTTP status — or the error_code of an `ok:false` body — onto a class.
//
//	401, 404   the token (the bot path) is not known: TokenRejected
//	429        RateLimited (retryable)
//	408, 5xx   Unavailable (retryable)
//	3xx        a redirect, not followed: Rejected
//	other      Rejected
func classifyStatus(code int) Outcome {
	switch {
	case code == http.StatusUnauthorized || code == http.StatusNotFound:
		return OutcomeTokenRejected
	case code == http.StatusTooManyRequests:
		return OutcomeRateLimited
	case code == http.StatusRequestTimeout || (code >= 500 && code <= 599):
		return OutcomeUnavailable
	}
	return OutcomeRejected
}

// decodeEnvelope reads the BOTH shapes ADR 0074 names: wrapped `{"ok":…, "result":{…}}` and flat
// `{…}` (the result object itself). `ok:false` is classified by its error_code. Anything else is
// MalformedResponse.
func decodeEnvelope(b []byte, out any) Outcome {
	var env struct {
		OK        *bool           `json:"ok"`
		Result    json.RawMessage `json:"result"`
		ErrorCode json.RawMessage `json:"error_code"`
	}
	if err := json.Unmarshal(b, &env); err != nil {
		return OutcomeMalformedResponse
	}
	if env.OK != nil && !*env.OK {
		code, ok := scalarText(env.ErrorCode)
		if !ok {
			return OutcomeRejected
		}
		n, err := strconv.Atoi(code)
		if err != nil {
			return OutcomeRejected
		}
		return classifyStatus(n)
	}
	payload := b
	if len(env.Result) > 0 && string(env.Result) != "null" {
		payload = env.Result
	}
	// setWebhook may answer `{"ok":true,"result":true}`: a bare value is a valid result there.
	if raw, isRaw := out.(*json.RawMessage); isRaw {
		*raw = append((*raw)[:0], payload...)
		return OutcomeOK
	}
	if len(payload) == 0 || payload[0] != '{' {
		return OutcomeMalformedResponse
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return OutcomeMalformedResponse
	}
	return OutcomeOK
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

// epochTime reads Zalo's `updated_at` ONLY when its unit is unambiguous from its magnitude:
// milliseconds since 1970 (13 digits, 2001-2286) or seconds (10 digits, 2001-2286). Anything else —
// a string, a fraction, a number in neither range — is ZERO: reported as absent, never guessed.
func epochTime(raw json.RawMessage) time.Time {
	s, ok := scalarText(raw)
	if !ok || len(raw) == 0 || raw[0] == '"' {
		return time.Time{}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}
	}
	switch {
	case n >= 1_000_000_000_000 && n < 10_000_000_000_000:
		return time.UnixMilli(n).UTC()
	case n >= 1_000_000_000 && n < 10_000_000_000:
		return time.Unix(n, 0).UTC()
	}
	return time.Time{}
}
