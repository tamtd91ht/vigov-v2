package zalobot

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/secret"
)

// testToken is FAKE — the text says so (rule 8, forbidden #1). Shaped like Zalo's `123456789:abc-xyz`.
const testToken = "123456789:FAKE-zalo-bot-token-NOT-REAL"

// recorder is a fake Zalo: it records what reached it and answers what the test says.
type recorder struct {
	mu     sync.Mutex
	paths  []string
	bodies [][]byte
	answer func(w http.ResponseWriter, r *http.Request)
}

func (rc *recorder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	rc.mu.Lock()
	rc.paths = append(rc.paths, r.URL.Path)
	rc.bodies = append(rc.bodies, b)
	rc.mu.Unlock()
	rc.answer(w, r)
}

func start(t *testing.T, answer func(w http.ResponseWriter, r *http.Request)) (*Client, *recorder, *httptest.Server) {
	t.Helper()
	rc := &recorder{answer: answer}
	srv := httptest.NewTLSServer(rc)
	t.Cleanup(srv.Close)
	return newClient(srv.URL, srv.Client()), rc, srv
}

func jsonAnswer(status int, body string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

// captureLogs routes slog's default logger AND the standard log package into one buffer for the test.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prevSlog := slog.Default()
	prevOut := log.Writer()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	log.SetOutput(&buf)
	t.Cleanup(func() {
		slog.SetDefault(prevSlog)
		log.SetOutput(prevOut)
	})
	return &buf
}

func TestGetMeReadsBothShapesAndPutsTheTokenOnlyInThePath(t *testing.T) {
	for name, body := range map[string]string{
		"wrapped, numeric id": `{"ok":true,"result":{"id":987654321,"account_name":"Bot ViGov"}}`,
		"flat, string id":     `{"id":"987654321","account_name":"Bot ViGov"}`,
	} {
		t.Run(name, func(t *testing.T) {
			c, rc, _ := start(t, jsonAnswer(200, body))
			info, o := c.GetMe(context.Background(), secret.Secret(testToken))
			if o != OutcomeOK || info.AccountID != "987654321" || info.AccountName != "Bot ViGov" {
				t.Fatalf("= %+v, %v", info, o)
			}
			if rc.paths[0] != "/bot"+testToken+"/getMe" {
				t.Fatalf("path = %q", rc.paths[0])
			}
		})
	}
}

func TestGetMeWithoutAnAccountIDIsMalformed(t *testing.T) {
	c, _, _ := start(t, jsonAnswer(200, `{"ok":true,"result":{"account_name":"Bot"}}`))
	if _, o := c.GetMe(context.Background(), secret.Secret(testToken)); o != OutcomeMalformedResponse {
		t.Fatalf("= %v", o)
	}
}

func TestStatusClasses(t *testing.T) {
	cases := map[int]Outcome{
		401: OutcomeTokenRejected, 404: OutcomeTokenRejected,
		429: OutcomeRateLimited,
		408: OutcomeUnavailable, 500: OutcomeUnavailable, 502: OutcomeUnavailable, 503: OutcomeUnavailable,
		400: OutcomeRejected, 403: OutcomeRejected, 409: OutcomeRejected,
	}
	for status, want := range cases {
		c, _, _ := start(t, jsonAnswer(status, `{"ok":false,"error_code":`+strconv.Itoa(status)+`,"description":"`+testToken+`"}`))
		if _, o := c.GetMe(context.Background(), secret.Secret(testToken)); o != want {
			t.Errorf("HTTP %d: = %v, want %v", status, o, want)
		}
	}
}

func TestOKFalseBodyIsClassifiedByErrorCode(t *testing.T) {
	cases := map[string]Outcome{
		`{"ok":false,"error_code":401,"description":"Unauthorized"}`: OutcomeTokenRejected,
		`{"ok":false,"error_code":"429"}`:                            OutcomeRateLimited,
		`{"ok":false,"error_code":500}`:                              OutcomeUnavailable,
		`{"ok":false,"error_code":400}`:                              OutcomeRejected,
		`{"ok":false}`:                                               OutcomeRejected,
		`<html>gateway</html>`:                                       OutcomeMalformedResponse,
		`[]`:                                                         OutcomeMalformedResponse,
	}
	for body, want := range cases {
		c, _, _ := start(t, jsonAnswer(200, body))
		if _, o := c.GetMe(context.Background(), secret.Secret(testToken)); o != want {
			t.Errorf("%s: = %v, want %v", body, o, want)
		}
	}
}

func TestRetryPolicy(t *testing.T) {
	for o, want := range map[Outcome]bool{
		OutcomeOK: false, OutcomeTokenRejected: false, OutcomeRateLimited: true, OutcomeUnavailable: true,
		OutcomeRejected: false, OutcomeMalformedResponse: false,
	} {
		if o.Retryable() != want {
			t.Errorf("%v.Retryable() = %v", o, !want)
		}
	}
}

// A redirect is never followed: the request carries the token in its path.
func TestRedirectIsNotFollowed(t *testing.T) {
	var hit bool
	other := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hit = true }))
	defer other.Close()
	c, _, _ := start(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+r.URL.Path, http.StatusTemporaryRedirect)
	})
	if _, o := c.GetMe(context.Background(), secret.Secret(testToken)); o != OutcomeRejected {
		t.Fatalf("= %v", o)
	}
	if hit {
		t.Fatal("the redirect was followed — the token went to another host")
	}
}

// THE PROPERTY THE PACKAGE EXISTS FOR: whatever fails, nothing carrying the token comes out — no
// error (there is none to return), no log line from this package or from net/http, no Outcome text.
func TestTokenNeverLeavesThroughAFailure(t *testing.T) {
	buf := captureLogs(t)

	// 1. A server that is gone: the transport error (*url.Error) names the full URL.
	c, _, srv := start(t, jsonAnswer(200, `{}`))
	srv.Close()
	if _, o := c.GetMe(context.Background(), secret.Secret(testToken)); o != OutcomeUnavailable {
		t.Fatalf("closed server: = %v", o)
	}

	// 2. A timeout.
	slow, _, _ := start(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	})
	slow.http.Timeout = 50 * time.Millisecond
	if o := slow.SetWebhook(context.Background(), secret.Secret(testToken), "https://bot.api.example/x",
		secret.Secret("FAKE-secret-token-0123456789abcdef")); o != OutcomeUnavailable {
		t.Fatalf("timeout: = %v", o)
	}

	// 3. A cancelled context.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ok, _, _ := start(t, jsonAnswer(200, `{"ok":true,"result":{"id":1}}`))
	if _, o := ok.GetMe(ctx, secret.Secret(testToken)); o != OutcomeUnavailable {
		t.Fatalf("cancelled: = %v", o)
	}

	// 4. Zalo echoing the token back in an error body.
	echo, _, _ := start(t, jsonAnswer(400, `{"ok":false,"error_code":400,"description":"bad bot`+testToken+`"}`))
	o := echo.SendMessage(context.Background(), secret.Secret(testToken), "chat-1", "xin chào")
	if o != OutcomeRejected {
		t.Fatalf("echo: = %v", o)
	}

	for _, s := range []string{buf.String(), o.String(), OutcomeUnavailable.String()} {
		if strings.Contains(s, testToken) || strings.Contains(s, "FAKE-zalo") {
			t.Fatalf("the token leaked: %q", s)
		}
	}
	if buf.Len() != 0 {
		t.Fatalf("the adapter (or net/http) logged: %q", buf.String())
	}
}

func TestInvalidTokenNeverReachesTheNetwork(t *testing.T) {
	c, rc, _ := start(t, jsonAnswer(200, `{"id":1}`))
	for _, tok := range []string{"", "a/b", "a?b", "a#b", "a%2Fb", "a b", "a\nb", "ẽ", strings.Repeat("x", 257)} {
		if _, o := c.GetMe(context.Background(), secret.Secret(tok)); o != OutcomeTokenRejected {
			t.Errorf("%q: = %v", tok, o)
		}
		if ValidToken([]byte(tok)) {
			t.Errorf("ValidToken(%q) = true", tok)
		}
	}
	if len(rc.paths) != 0 {
		t.Fatalf("an invalid token was sent: %d calls", len(rc.paths))
	}
	if !ValidToken([]byte(testToken)) {
		t.Fatal("Zalo's own token shape refused")
	}
}

func TestSetWebhookSendsURLAndSecret(t *testing.T) {
	c, rc, _ := start(t, jsonAnswer(200, `{"ok":true,"result":true}`))
	const u = "https://bot.api.vigov.vn/api/v1/zalo-bot-updates"
	sec := secret.Secret("FAKE-secret-token-0123456789abcdef")
	if o := c.SetWebhook(context.Background(), secret.Secret(testToken), u, sec); o != OutcomeOK {
		t.Fatalf("= %v", o)
	}
	var got map[string]string
	if err := json.Unmarshal(rc.bodies[0], &got); err != nil {
		t.Fatal(err)
	}
	if got["url"] != u || got["secret_token"] != string(sec) || rc.paths[0] != "/bot"+testToken+"/setWebhook" {
		t.Fatalf("sent %v to %q", got, rc.paths[0])
	}
	if o := c.SetWebhook(context.Background(), secret.Secret(testToken), u, secret.Secret("short")); o != OutcomeRejected {
		t.Fatalf("a secret shorter than Zalo's 8 must be refused before the call: %v", o)
	}
}

func TestSendMessageClipsTo2000Characters(t *testing.T) {
	c, rc, _ := start(t, jsonAnswer(200, `{"ok":true,"result":{"message_id":"m1"}}`))
	long := strings.Repeat("ệ", MaxTextRunes+5)
	if o := c.SendMessage(context.Background(), secret.Secret(testToken), "chat-1", long); o != OutcomeOK {
		t.Fatalf("= %v", o)
	}
	var got map[string]string
	if err := json.Unmarshal(rc.bodies[0], &got); err != nil {
		t.Fatal(err)
	}
	if n := len([]rune(got["text"])); n != MaxTextRunes || got["chat_id"] != "chat-1" {
		t.Fatalf("sent %d characters to %q", n, got["chat_id"])
	}
	if o := c.SendMessage(context.Background(), secret.Secret(testToken), "", "x"); o != OutcomeRejected {
		t.Fatalf("empty chat id: %v", o)
	}
	if o := c.SendMessage(context.Background(), secret.Secret(testToken), "chat-1", ""); o != OutcomeRejected {
		t.Fatalf("empty text: %v", o)
	}
	if len(rc.paths) != 1 {
		t.Fatalf("a caller fault reached Zalo: %d calls", len(rc.paths))
	}
}

func TestGetWebhookInfoReadsOnlyAnUnambiguousTime(t *testing.T) {
	cases := map[string]time.Time{
		`{"ok":true,"result":{"url":"https://a.example/x","updated_at":1749538250568}}`: time.UnixMilli(1749538250568).UTC(),
		`{"url":"https://a.example/x","updated_at":1749538250}`:                         time.Unix(1749538250, 0).UTC(),
		`{"url":"https://a.example/x","updated_at":"2026-10-05"}`:                       {},
		`{"url":"https://a.example/x","updated_at":12345}`:                              {},
		`{"url":"https://a.example/x"}`:                                                 {},
	}
	for body, want := range cases {
		c, _, _ := start(t, jsonAnswer(200, body))
		info, o := c.GetWebhookInfo(context.Background(), secret.Secret(testToken))
		if o != OutcomeOK || info.URL != "https://a.example/x" || !info.UpdatedAt.Equal(want) {
			t.Errorf("%s: = %+v, %v", body, info, o)
		}
	}
	c, _, _ := start(t, jsonAnswer(200, `{"ok":true,"result":{"url":""}}`))
	if info, o := c.GetWebhookInfo(context.Background(), secret.Secret(testToken)); o != OutcomeOK || info.URL != "" {
		t.Errorf("no webhook: = %+v, %v", info, o)
	}
}

func TestProductionClientIsHTTPSOnly(t *testing.T) {
	c := New(0)
	if c.base != "https://bot-api.zaloplatforms.com" || c.http.Timeout != DefaultTimeout {
		t.Fatalf("base %q timeout %v", c.base, c.http.Timeout)
	}
	tr := c.http.Transport.(*http.Transport)
	if tr.Proxy != nil || tr.TLSClientConfig.InsecureSkipVerify || tr.TLSClientConfig.MinVersion < 0x0303 {
		t.Fatal("transport security lowered")
	}
}
