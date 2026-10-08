package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/crypto"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// The five routes of the commune's own bot on the staff harness (checkerDanhMucGia, chuTheGhi, canBoGhi).
// The fake records WHICH commune and WHICH actor reached it; the transactions, the sealing and the entries
// are internal/app's tests.
//
//	admin.lookup: 401 no session · 403 wrong key · 403 right key other commune · 200

// testBotToken is a fixture, not a credential.
const testBotToken = "123456789:FAKE-commune-bot-token-NOT-REAL"

type fakeCommuneBots struct {
	calls       int
	lastCommune tenant.ID
	lastActor   audit.Actor
	lastSet     app.SetCommuneZaloBotInput
	lastToken   string
	lastReason  string
	err         error
	secret      string
	setOut      app.SetCommuneZaloBotOutput
	retired     bool
	hasOwnBot   bool
}

func (f *fakeCommuneBots) note(ctx context.Context, a audit.Actor) {
	f.calls++
	f.lastCommune, f.lastActor = tenant.MustFrom(ctx), a
}

func sampleBot() domain.CommuneZaloBot {
	at := time.Date(2026, 10, 8, 2, 0, 0, 0, time.UTC)
	return domain.CommuneZaloBot{ID: "01JBOTROW000000000000000001", BotAccountID: "bot-111", BotName: "Bot Xã A",
		ChatURL: "https://zalo.me/123", SetAt: at, SetBy: "CB-00099", LastCheckAt: at, LastCheckResult: domain.ZaloCallOK}
}

func (f *fakeCommuneBots) Current(ctx context.Context) (app.CommuneZaloBotView, error) {
	f.note(ctx, audit.Actor{})
	if !f.hasOwnBot {
		return app.CommuneZaloBotView{LiveLinkCount: 3}, f.err
	}
	return app.CommuneZaloBotView{HasOwnBot: true, Bot: sampleBot(), LiveLinkCount: 3}, f.err
}

func (f *fakeCommuneBots) Set(ctx context.Context, in app.SetCommuneZaloBotInput, a audit.Actor) (app.SetCommuneZaloBotOutput, error) {
	f.note(ctx, a)
	f.lastSet, f.lastToken = in, string(in.Token.Lo())
	if f.err != nil {
		return app.SetCommuneZaloBotOutput{}, f.err
	}
	out := f.setOut
	out.Bot = sampleBot()
	return out, nil
}

func (f *fakeCommuneBots) Check(ctx context.Context, a audit.Actor) (app.CheckOutput, error) {
	f.note(ctx, a)
	return app.CheckOutput{Outcome: domain.ZaloCallOK, CheckedAt: time.Date(2026, 10, 8, 3, 0, 0, 0, time.UTC),
		AccountName: "Bot Xã A"}, f.err
}

func (f *fakeCommuneBots) RegisterWebhook(ctx context.Context, a audit.Actor) (app.CommuneWebhookOutput, error) {
	f.note(ctx, a)
	if f.err != nil {
		return app.CommuneWebhookOutput{}, f.err
	}
	return app.CommuneWebhookOutput{Outcome: domain.ZaloCallOK, URL: "https://" + hostA + ZaloBotUpdatesPath,
		SetAt: time.Date(2026, 10, 8, 3, 0, 0, 0, time.UTC), SetBy: a.ID, Secret: secret.Secret(f.secret)}, nil
}

func (f *fakeCommuneBots) Retire(ctx context.Context, reason string, a audit.Actor) (app.RetireCommuneZaloBotOutput, error) {
	f.note(ctx, a)
	f.lastReason = reason
	return app.RetireCommuneZaloBotOutput{Retired: f.retired, EndedLinkCount: 2}, f.err
}

type communeBotServer struct {
	h       http.Handler
	fake    *fakeCommuneBots
	checker *checkerDanhMucGia
	keys    int
}

func newCommuneBotServer(t *testing.T) *communeBotServer {
	t.Helper()
	fake := &fakeCommuneBots{secret: "generated-webhook-secret-shown-once-XXXXXXX", hasOwnBot: true, retired: true}
	checker := &checkerDanhMucGia{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	RegisterZaloCommuneBot(mux, ZaloCommuneBotDeps{Checker: checker, Bots: fake, Log: log})
	var h http.Handler = mux
	h = chuTheGhi(h)
	h = idem.Middleware(moKhoIdemGia(), log)(h)
	h = httpx.TenantMiddleware(thuMucMau())(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)
	return &communeBotServer{h: h, fake: fake, checker: checker}
}

func (s *communeBotServer) grant(commune tenant.ID, perms ...authz.Perm) {
	if s.checker.co == nil {
		s.checker.co = map[tenant.ID]map[authz.Perm]struct{}{}
	}
	if s.checker.co[commune] == nil {
		s.checker.co[commune] = map[authz.Perm]struct{}{}
	}
	for _, p := range perms {
		s.checker.co[commune][p] = struct{}{}
	}
}

func (s *communeBotServer) call(t *testing.T, method, path, host string, p *authz.Principal, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	r := httptest.NewRequest(method, "https://"+host+path, rd)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	r.Header.Set("Content-Type", "application/json")
	s.keys++
	r.Header.Set(idem.Header, fmt.Sprintf("01JIDEMCOMMUNEBOT%09d", s.keys))
	if p != nil {
		r = r.WithContext(context.WithValue(r.Context(), khoaChuTheGhi{}, *p))
	}
	w := httptest.NewRecorder()
	s.h.ServeHTTP(w, r)
	return w
}

const (
	communeBotPath = "/api/v1/zalo-bots/current"
	putBotBody     = `{"bot_token":"` + testBotToken + `","bot_name":"Bot Xã A","chat_url":"https://zalo.me/123"}`
	retireBotBody  = `{"reason":"Xã không dùng bot riêng nữa"}`
)

// communeBotRoutes — the wrong keys are real and adjacent: a content reader, and asset.update (a writer
// elsewhere on the same screens) must not configure the commune's bot.
func communeBotRoutes() []mapAssetRoute {
	return []mapAssetRoute{
		{"get", http.MethodGet, communeBotPath, "", "admin.lookup", "content.read", http.StatusOK},
		{"put", http.MethodPut, communeBotPath, putBotBody, "admin.lookup", "asset.update", http.StatusOK},
		{"check", http.MethodPost, communeBotPath + "/check", "", "admin.lookup", "content.read", http.StatusOK},
		{"webhook", http.MethodPost, communeBotPath + "/webhook", "", "admin.lookup", "asset.update", http.StatusOK},
		{"delete", http.MethodDelete, communeBotPath, retireBotBody, "admin.lookup", "admin.audit", http.StatusOK},
	}
}

func TestCommuneBotRoutesAskForSeededKey(t *testing.T) {
	for _, tc := range communeBotRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newCommuneBotServer(t)
			s.call(t, tc.method, tc.path, hostA, canBoGhi(xaA), tc.body)
			// Compared with a LITERAL: a fake checker grants any string (rule 5, invariant 3c).
			if len(s.checker.hoiGi) == 0 || s.checker.hoiGi[0] != "admin.lookup" {
				t.Fatalf("asked for %v, want admin.lookup (seeded at service-identity/migrations/0001_init.sql:281)", s.checker.hoiGi)
			}
		})
	}
}

func TestCommuneBotRoutes_401NoSession(t *testing.T) {
	for _, tc := range communeBotRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newCommuneBotServer(t)
			s.grant(xaA, tc.perm)
			doiMa(t, s.call(t, tc.method, tc.path, hostA, nil, tc.body), http.StatusUnauthorized)
			if s.fake.calls != 0 {
				t.Error("no session and the use case ran")
			}
		})
	}
}

func TestCommuneBotRoutes_403WrongPermission(t *testing.T) {
	for _, tc := range communeBotRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newCommuneBotServer(t)
			s.grant(xaA, tc.wrongPerm)
			doiMa(t, s.call(t, tc.method, tc.path, hostA, canBoGhi(xaA), tc.body), http.StatusForbidden)
			if s.fake.calls != 0 {
				t.Error("wrong permission and the use case ran")
			}
		})
	}
}

func TestCommuneBotRoutes_403RightPermissionWrongCommune(t *testing.T) {
	// Signed in at B as a member of B; the grant is in A. A checker ignoring the commune would let A's
	// administrator replace B's bot — and take its staff's chats.
	for _, tc := range communeBotRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newCommuneBotServer(t)
			s.grant(xaA, tc.perm)
			doiMa(t, s.call(t, tc.method, tc.path, hostB, canBoGhi(xaB), tc.body), http.StatusForbidden)
			if s.fake.calls != 0 {
				t.Error("a grant in another commune was enough")
			}
		})
	}
}

func TestCommuneBotRoutes_401SessionOfAnotherCommune(t *testing.T) {
	for _, tc := range communeBotRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newCommuneBotServer(t)
			s.grant(xaA, tc.perm)
			s.grant(xaB, tc.perm)
			doiMa(t, s.call(t, tc.method, tc.path, hostB, canBoGhi(xaA), tc.body), http.StatusUnauthorized)
			if s.fake.calls != 0 {
				t.Error("a session of another commune still reached the use case")
			}
		})
	}
}

func TestCommuneBotRoutes_200RightPermissionRightCommune(t *testing.T) {
	for _, tc := range communeBotRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newCommuneBotServer(t)
			s.grant(xaA, tc.perm)
			doiMa(t, s.call(t, tc.method, tc.path, hostA, canBoGhi(xaA), tc.body), tc.ok)
			if s.fake.calls != 1 || s.fake.lastCommune != xaA {
				t.Fatalf("use case ran %d times in %q, want once in %q", s.fake.calls, s.fake.lastCommune, xaA)
			}
			if tc.method != http.MethodGet && (s.fake.lastActor.ID != maCanBoGhi || s.fake.lastActor.IP != "10.0.0.7") {
				// Rule 6, invariant 8: the BUSINESS CODE, never the internal id.
				t.Errorf("actor = %+v, want business code %q", s.fake.lastActor, maCanBoGhi)
			}
		})
	}
}

// --- shapes ------------------------------------------------------------------------------------------

func TestCommuneBotGetCarriesNoSecretAndTheContractFields(t *testing.T) {
	s := newCommuneBotServer(t)
	s.grant(xaA, "admin.lookup")
	w := s.call(t, http.MethodGet, communeBotPath, hostA, canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	body := w.Body.String()
	for _, bad := range []string{"token", "secret\"", "sealed"} {
		if strings.Contains(body, bad) {
			t.Fatalf("GET carries %q: %s", bad, body)
		}
	}
	var out struct {
		HasOwnBot     bool                       `json:"has_own_bot"`
		Bot           map[string]json.RawMessage `json:"bot"`
		LiveLinkCount int                        `json:"live_link_count"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"bot_account_id", "bot_name", "chat_url", "set_at", "set_by", "webhook_set_at",
		"webhook_set_by", "webhook_pending", "last_check"} {
		if _, ok := out.Bot[k]; !ok {
			t.Errorf("bot lacks %q: %s", k, body)
		}
	}
	if !out.HasOwnBot || out.LiveLinkCount != 3 {
		t.Errorf("reply = %s", body)
	}
	s.fake.hasOwnBot = false
	w = s.call(t, http.MethodGet, communeBotPath, hostA, canBoGhi(xaA), "")
	if !strings.Contains(w.Body.String(), `"has_own_bot":false`) || !strings.Contains(w.Body.String(), `"bot":null`) {
		t.Errorf("no own bot = %s", w.Body.String())
	}
}

func TestCommuneBotPutPassesTheTokenAsASecretAndNeverEchoesIt(t *testing.T) {
	s := newCommuneBotServer(t)
	s.grant(xaA, "admin.lookup")
	s.fake.setOut = app.SetCommuneZaloBotOutput{Adopted: true, EndedLinkCount: 4, RetiredPrevious: true}
	w := s.call(t, http.MethodPut, communeBotPath, hostA, canBoGhi(xaA), putBotBody)
	doiMa(t, w, http.StatusOK)
	if s.fake.lastToken != testBotToken || s.fake.lastSet.BotName != "Bot Xã A" || s.fake.lastSet.ChatURL != "https://zalo.me/123" {
		t.Errorf("use case received %+v", s.fake.lastSet)
	}
	if strings.Contains(fmt.Sprintf("%v %+v", s.fake.lastSet, s.fake.lastSet), testBotToken) {
		t.Error("the request formats with the token in it")
	}
	if strings.Contains(w.Body.String(), testBotToken) {
		t.Fatal("PUT echoes the token")
	}
	for _, want := range []string{`"adopted":true`, `"ended_link_count":4`, `"revoke_notice"`} {
		if !strings.Contains(w.Body.String(), want) {
			t.Errorf("reply lacks %s: %s", want, w.Body.String())
		}
	}
}

func TestCommuneBotWebhookShowsTheSecretOnceUncached(t *testing.T) {
	s := newCommuneBotServer(t)
	s.grant(xaA, "admin.lookup")
	w := s.call(t, http.MethodPost, communeBotPath+"/webhook", hostA, canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q — a credential must not be cached", w.Header().Get("Cache-Control"))
	}
	if !strings.Contains(w.Body.String(), `"secret":"`+s.fake.secret+`"`) || !strings.Contains(w.Body.String(), `"url":"https://`+hostA) {
		t.Errorf("reply = %s", w.Body.String())
	}
	s.fake.secret = "" // a reused pending secret is not shown again
	w = s.call(t, http.MethodPost, communeBotPath+"/webhook", hostA, canBoGhi(xaA), "")
	if strings.Contains(w.Body.String(), `"secret"`) {
		t.Errorf("an empty secret is still a field: %s", w.Body.String())
	}
}

func TestCommuneBotDeleteCarriesTheReasonAndTheRevokeNotice(t *testing.T) {
	s := newCommuneBotServer(t)
	s.grant(xaA, "admin.lookup")
	w := s.call(t, http.MethodDelete, communeBotPath, hostA, canBoGhi(xaA), retireBotBody)
	doiMa(t, w, http.StatusOK)
	if s.fake.lastReason != "Xã không dùng bot riêng nữa" {
		t.Errorf("reason = %q", s.fake.lastReason)
	}
	if !strings.Contains(w.Body.String(), `"retired":true`) || !strings.Contains(w.Body.String(), "Zalo Bot Creator") {
		t.Errorf("reply = %s", w.Body.String())
	}
	s.fake.retired = false
	w = s.call(t, http.MethodDelete, communeBotPath, hostA, canBoGhi(xaA), retireBotBody)
	if strings.Contains(w.Body.String(), "revoke_notice") {
		t.Errorf("nothing retired, yet a revoke notice: %s", w.Body.String())
	}
}

func TestCommuneBotErrorMapping(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"no KEK", crypto.ErrNotConfigured, http.StatusServiceUnavailable, "encryption_not_configured"},
		{"token rejected", &app.ZaloTokenCheckError{Class: domain.ZaloCallTokenRejected}, http.StatusUnprocessableEntity, "zalo_token_rejected"},
		{"zalo down", &app.ZaloTokenCheckError{Class: domain.ZaloCallUnavailable}, http.StatusBadGateway, "zalo_unavailable"},
		{"taken elsewhere", fmt.Errorf("save: %w", commsstore.ErrCommuneZaloBotAccountTaken), http.StatusConflict, "zalo_bot_in_use"},
		{"changed", app.ErrCommuneZaloBotChanged, http.StatusConflict, "zalo_bot_changed"},
		{"no own bot", app.ErrCommuneZaloBotMissing, http.StatusConflict, "zalo_own_bot_missing"},
		{"no host", app.ErrCommuneHostUnknown, http.StatusServiceUnavailable, "commune_host_unavailable"},
		{"bad name", fmt.Errorf("x: %w", domain.ErrCommuneZaloBotName), http.StatusBadRequest, "invalid_request"},
		{"no token", domain.ErrCommuneZaloBotTokenRequired, http.StatusBadRequest, "invalid_request"},
		{"broken", errors.New("boom: " + string(xaB)), http.StatusInternalServerError, "internal"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newCommuneBotServer(t)
			s.grant(xaA, "admin.lookup")
			s.fake.err = c.err
			w := s.call(t, http.MethodPut, communeBotPath, hostA, canBoGhi(xaA), putBotBody)
			doiMa(t, w, c.status)
			e := loiTra(t, w)
			if e.Code != c.code {
				t.Errorf("code = %q, want %q", e.Code, c.code)
			}
			// Never another commune, never the wrapped chain, never the token.
			for _, bad := range []string{string(xaB), "boom", testBotToken, "zalo_bot_rieng:"} {
				if strings.Contains(e.Message, bad) {
					t.Errorf("message leaks %q: %q", bad, e.Message)
				}
			}
		})
	}
}

func TestRegisterZaloCommuneBotRefusesIncompleteWiring(t *testing.T) {
	for name, d := range map[string]ZaloCommuneBotDeps{
		"checker": {Bots: &fakeCommuneBots{}},
		"bots":    {Checker: &checkerDanhMucGia{}},
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("incomplete wiring accepted")
				}
			}()
			RegisterZaloCommuneBot(http.NewServeMux(), d)
		})
	}
}
