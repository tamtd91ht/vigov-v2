package http

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/ratelimit"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	"github.com/vihat/vigov/service-comms/internal/zalobot"
)

// THE WEBHOOK. What these tests defend (ADR 0074 #5):
//
//  1. a missing or wrong secret is 403 AND THE BODY IS NOT READ;
//  2. the rate limit runs BEFORE the secret is looked at, and fails closed;
//  3. every authenticated update is 200 — ignored events and malformed payloads included;
//  4. the reply goes out AFTER Zalo is answered, to the chat the update came from.

type fakeUpdates struct {
	secret    string
	authErr   error
	authCalls int
	handled   []zalobot.Update
	reply     string
	replies   []string
	repliedTo []string
	ip        string
}

func (f *fakeUpdates) Authenticate(_ context.Context, presented string) (bool, error) {
	f.authCalls++
	return presented != "" && presented == f.secret, f.authErr
}

func (f *fakeUpdates) Handle(_ context.Context, u zalobot.Update, ip string) string {
	f.handled = append(f.handled, u)
	f.ip = ip
	return f.reply
}

func (f *fakeUpdates) Reply(chatID, text string) {
	f.repliedTo = append(f.repliedTo, chatID)
	f.replies = append(f.replies, text)
}

// readCounter records whether anybody read the body.
type readCounter struct {
	r    io.Reader
	read bool
}

func (b *readCounter) Read(p []byte) (int, error) {
	b.read = true
	return b.r.Read(p)
}

// fakeCommuneUpdates is the commune bot's use case, RECORDING THE COMMUNE each call ran in. Its secret is
// per commune, as the real one is.
type fakeCommuneUpdates struct {
	secrets   map[tenant.ID]string
	authCalls int
	authIn    []tenant.ID
	handledIn []tenant.ID
	repliedIn []tenant.ID
	handled   []zalobot.Update
	// lostAuth: Handle or Reply ran without Authenticate's context — the authenticated bot was dropped.
	lostAuth bool
}

// authMark is what the fake's Authenticate puts in the context; Handle and Reply must see it.
type authMark struct{}

func (f *fakeCommuneUpdates) Authenticate(ctx context.Context, presented string) (context.Context, bool, error) {
	f.authCalls++
	xa := tenant.MustFrom(ctx)
	f.authIn = append(f.authIn, xa)
	if presented == "" || presented != f.secrets[xa] {
		return ctx, false, nil
	}
	return context.WithValue(ctx, authMark{}, "bot-of-"+string(xa)), true, nil
}

func (f *fakeCommuneUpdates) Handle(ctx context.Context, u zalobot.Update, _ string) string {
	if ctx.Value(authMark{}) == nil {
		f.lostAuth = true
	}
	f.handledIn = append(f.handledIn, tenant.MustFrom(ctx))
	f.handled = append(f.handled, u)
	return domain.ZaloReplyHelp
}

func (f *fakeCommuneUpdates) Reply(ctx context.Context, _, _ string) {
	if ctx.Value(authMark{}) == nil {
		f.lostAuth = true
	}
	f.repliedIn = append(f.repliedIn, tenant.MustFrom(ctx))
}

type webhookServer struct {
	h       http.Handler
	commune http.Handler // the SAME mux behind TenantMiddleware — cmd/server's commune chain
	fake    *fakeUpdates
	cfake   *fakeCommuneUpdates
	counter *memCounter
}

func newWebhookServer(t *testing.T) *webhookServer {
	t.Helper()
	fake := &fakeUpdates{secret: "s3cret-of-forty-three-characters-XXXXXXXXXX", reply: domain.ZaloReplyHelp}
	cfake := &fakeCommuneUpdates{secrets: map[tenant.ID]string{xaA: "secret-of-commune-A", xaB: "secret-of-commune-B"}}
	counter := &memCounter{}
	lim, err := ratelimit.New(counter, ratelimit.ZaloBotWebhook)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	RegisterZaloBotUpdates(mux, ZaloBotUpdateDeps{Updates: fake, CommuneUpdates: cfake, Limiter: lim,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	var commune http.Handler = mux
	commune = httpx.TenantMiddleware(thuMucMau())(commune)
	commune = httpx.StripTenantHeaders(commune)
	return &webhookServer{h: mux, commune: commune, fake: fake, cfake: cfake, counter: counter}
}

func (s *webhookServer) postCommune(t *testing.T, host, secret string, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "https://"+host+ZaloBotUpdatesPath, body)
	r.Host = host
	r.RemoteAddr = "203.0.113.9:443"
	if secret != "" {
		r.Header.Set(ZaloSecretHeader, secret)
	}
	w := httptest.NewRecorder()
	s.commune.ServeHTTP(w, r)
	return w
}

const flatUpdate = `{"event_name":"message.text.received","message":{"chat":{"id":"chat-1"},"text":"/trogiup","message_id":"m1"}}`

func (s *webhookServer) post(t *testing.T, secret string, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "https://bot.api.example.vn"+ZaloBotUpdatesPath, body)
	r.RemoteAddr = "203.0.113.9:443"
	if secret != "" {
		r.Header.Set(ZaloSecretHeader, secret)
	}
	w := httptest.NewRecorder()
	s.h.ServeHTTP(w, r)
	return w
}

func TestZaloWebhookPathAgreesWithTheRouteAndTheRegisteredURL(t *testing.T) {
	if ZaloBotUpdatesPath != domain.ZaloWebhookPath {
		t.Fatalf("%q ≠ %q — Zalo would be told one URL and served another", ZaloBotUpdatesPath, domain.ZaloWebhookPath)
	}
	s := newWebhookServer(t)
	if w := s.post(t, s.fake.secret, strings.NewReader(flatUpdate)); w.Code != http.StatusOK {
		t.Fatalf("the literal route does not serve ZaloBotUpdatesPath: %d", w.Code)
	}
}

func TestZaloWebhookWrongOrMissingSecretIs403AndTheBodyIsNotRead(t *testing.T) {
	for name, secret := range map[string]string{"missing": "", "wrong": "not-the-secret"} {
		t.Run(name, func(t *testing.T) {
			s := newWebhookServer(t)
			body := &readCounter{r: strings.NewReader(flatUpdate)}
			doiMa(t, s.post(t, secret, body), http.StatusForbidden)
			if body.read {
				t.Error("the body was read before the secret was accepted")
			}
			if len(s.fake.handled) != 0 || len(s.fake.replies) != 0 {
				t.Error("an unauthenticated update was acted on")
			}
		})
	}
}

func TestZaloWebhookSecretStoreDownIs503BodyUnread(t *testing.T) {
	s := newWebhookServer(t)
	s.fake.authErr = errors.New("db down")
	body := &readCounter{r: strings.NewReader(flatUpdate)}
	doiMa(t, s.post(t, s.fake.secret, body), http.StatusServiceUnavailable)
	if body.read || len(s.fake.handled) != 0 {
		t.Error("acted without a working secret check")
	}
}

func TestZaloWebhookAuthenticatedIs200ThenReplies(t *testing.T) {
	s := newWebhookServer(t)
	w := s.post(t, s.fake.secret, strings.NewReader(flatUpdate))
	doiMa(t, w, http.StatusOK)
	if len(s.fake.handled) != 1 || s.fake.handled[0].ChatID != "chat-1" || s.fake.handled[0].Text != "/trogiup" {
		t.Fatalf("handled = %+v", s.fake.handled)
	}
	if s.fake.ip != "203.0.113.9" {
		t.Errorf("ip = %q, want the edge's client address", s.fake.ip)
	}
	if len(s.fake.repliedTo) != 1 || s.fake.repliedTo[0] != "chat-1" || s.fake.replies[0] != domain.ZaloReplyHelp {
		t.Errorf("reply to %v: %v", s.fake.repliedTo, s.fake.replies)
	}
	// The wrapped shape is read too.
	s2 := newWebhookServer(t)
	doiMa(t, s2.post(t, s2.fake.secret, strings.NewReader(`{"ok":true,"result":`+flatUpdate+`}`)), http.StatusOK)
	if len(s2.fake.handled) != 1 {
		t.Error("the result-wrapped shape was not read")
	}
}

func TestZaloWebhookMalformedOrOversizedIs200AndIgnored(t *testing.T) {
	for name, body := range map[string]string{
		"not json":   "{{{",
		"no chat":    `{"event_name":"message.text.received","message":{"text":"x"}}`,
		"oversized":  `{"event_name":"x","pad":"` + strings.Repeat("a", maxZaloUpdateBytes) + `"}`,
		"empty body": "",
	} {
		t.Run(name, func(t *testing.T) {
			s := newWebhookServer(t)
			doiMa(t, s.post(t, s.fake.secret, strings.NewReader(body)), http.StatusOK)
			if len(s.fake.handled) != 0 || len(s.fake.replies) != 0 {
				t.Error("a payload that could not be read was acted on")
			}
		})
	}
}

func TestZaloWebhookRateLimitBeforeTheSecretAndClosed(t *testing.T) {
	s := newWebhookServer(t)
	for i := 0; i < ratelimit.ZaloBotWebhookLimit; i++ {
		s.post(t, "wrong", strings.NewReader(flatUpdate))
	}
	before := s.fake.authCalls
	w := s.post(t, s.fake.secret, strings.NewReader(flatUpdate))
	doiMa(t, w, http.StatusTooManyRequests)
	if s.fake.authCalls != before {
		t.Error("the secret was compared for a request over the limit")
	}
	s2 := newWebhookServer(t)
	s2.counter.fail = errors.New("redis down")
	doiMa(t, s2.post(t, s2.fake.secret, strings.NewReader(flatUpdate)), http.StatusServiceUnavailable)
	if s2.fake.authCalls != 0 {
		t.Error("the webhook served with its rate limit down — it must fail closed")
	}
}

func TestRegisterZaloBotUpdatesRefusesIncompleteWiring(t *testing.T) {
	lim, _ := ratelimit.New(&memCounter{}, ratelimit.ZaloBotWebhook)
	for name, d := range map[string]ZaloBotUpdateDeps{
		"updates": {Limiter: lim, CommuneUpdates: &fakeCommuneUpdates{}},
		"commune": {Limiter: lim, Updates: &fakeUpdates{}},
		"limiter": {Updates: &fakeUpdates{}, CommuneUpdates: &fakeCommuneUpdates{}},
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("incomplete wiring accepted")
				}
			}()
			RegisterZaloBotUpdates(http.NewServeMux(), d)
		})
	}
}

// --- a commune's own bot: the same route on the commune's domain (ADR 0079 Q1 #2) ---------------------

func TestCommuneWebhookRunsInTheHostsCommuneOnly(t *testing.T) {
	s := newWebhookServer(t)
	doiMa(t, s.postCommune(t, hostA, "secret-of-commune-A", strings.NewReader(flatUpdate)), http.StatusOK)
	if len(s.cfake.handledIn) != 1 || s.cfake.handledIn[0] != xaA || len(s.cfake.repliedIn) != 1 || s.cfake.repliedIn[0] != xaA {
		t.Fatalf("handled in %v, replied in %v — want commune A from Host", s.cfake.handledIn, s.cfake.repliedIn)
	}
	if len(s.fake.handled) != 0 || s.fake.authCalls != 0 {
		t.Fatal("a commune host reached the SHARED bot's use case")
	}
	if s.cfake.lostAuth {
		t.Fatal("Handle or Reply ran without the authenticated bot's context")
	}
	// Commune A's secret presented on commune B's host is wrong there: 403, body unread.
	body := &readCounter{r: strings.NewReader(flatUpdate)}
	doiMa(t, s.postCommune(t, hostB, "secret-of-commune-A", body), http.StatusForbidden)
	if body.read || len(s.cfake.handled) != 1 {
		t.Fatal("another commune's secret was accepted, or the body was read")
	}
	// A client-named commune header changes nothing: Host decides.
	r := httptest.NewRequest(http.MethodPost, "https://"+hostA+ZaloBotUpdatesPath, strings.NewReader(flatUpdate))
	r.Host = hostA
	r.Header.Set("X-Tenant-Id", string(xaB))
	r.Header.Set(ZaloSecretHeader, "secret-of-commune-A")
	w := httptest.NewRecorder()
	s.commune.ServeHTTP(w, r)
	doiMa(t, w, http.StatusOK)
	if last := s.cfake.handledIn[len(s.cfake.handledIn)-1]; last != xaA {
		t.Fatalf("ran in %q, want the Host's commune", last)
	}
}

func TestCommuneWebhookUnknownHostIs404BeforeAnything(t *testing.T) {
	s := newWebhookServer(t)
	doiMa(t, s.postCommune(t, "khong-co.example.gov.vn", "secret-of-commune-A", strings.NewReader(flatUpdate)),
		http.StatusNotFound)
	if s.cfake.authCalls != 0 || len(s.counter.keys) != 0 {
		t.Fatal("an unknown host reached the limiter or the secret check")
	}
}

func TestCommuneWebhookRateLimitIsPerCommuneAndBeforeTheSecret(t *testing.T) {
	s := newWebhookServer(t)
	for i := 0; i < ratelimit.ZaloBotWebhookLimit; i++ {
		s.postCommune(t, hostA, "wrong", strings.NewReader(flatUpdate))
	}
	before := s.cfake.authCalls
	doiMa(t, s.postCommune(t, hostA, "secret-of-commune-A", strings.NewReader(flatUpdate)), http.StatusTooManyRequests)
	if s.cfake.authCalls != before {
		t.Error("the secret was compared for a request over the limit")
	}
	// Commune A's spent budget is not commune B's: the key carries the commune.
	doiMa(t, s.postCommune(t, hostB, "secret-of-commune-B", strings.NewReader(flatUpdate)), http.StatusOK)
	for _, k := range s.counter.keys {
		if !strings.HasPrefix(k, "t:") {
			t.Fatalf("a commune webhook counted under an unscoped key %q", k)
		}
	}
	s2 := newWebhookServer(t)
	s2.counter.fail = errors.New("redis down")
	doiMa(t, s2.postCommune(t, hostA, "secret-of-commune-A", strings.NewReader(flatUpdate)), http.StatusServiceUnavailable)
	if s2.cfake.authCalls != 0 {
		t.Error("the commune webhook served with its rate limit down — it must fail closed")
	}
}
