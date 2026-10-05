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
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// The seven Zalo channel routes on the staff harness (checkerDanhMucGia, chuTheGhi, canBoGhi). The fake
// records WHICH commune and WHICH actor reached it; the use case's transactions and entries are
// internal/app's and the store's PG tests'.
//
//	AnyAuthenticated (own link):  401 no session · 401 session of another commune · 403 non-staff · 200
//	admin.lookup (administration): 401 · 403 wrong key · 403 right key other commune · 200

type fakeZaloLinks struct {
	calls       int
	lastCommune tenant.ID
	lastActor   audit.Actor
	lastSave    domain.ZaloChannelSetting
	err         error
}

func (f *fakeZaloLinks) note(ctx context.Context, a audit.Actor) {
	f.calls++
	f.lastCommune, f.lastActor = tenant.MustFrom(ctx), a
}

func (f *fakeZaloLinks) Current(ctx context.Context, a audit.Actor) (app.ZaloLinkCurrentView, error) {
	f.note(ctx, a)
	return app.ZaloLinkCurrentView{Linked: true, LinkedAt: time.Date(2026, 10, 5, 1, 0, 0, 0, time.UTC),
		BotName: "Bot ViGov", ChatURL: "https://zalo.me/bot", ChannelEnabled: true}, f.err
}

func (f *fakeZaloLinks) LinkedStaff(ctx context.Context) ([]app.LinkedStaffView, error) {
	f.note(ctx, audit.Actor{})
	return []app.LinkedStaffView{{StaffCode: "CB-00123", StaffName: "Nguyễn Văn A",
		LinkedAt: time.Date(2026, 10, 5, 1, 0, 0, 0, time.UTC)}}, f.err
}

func (f *fakeZaloLinks) Settings(ctx context.Context) (domain.ZaloChannelSetting, error) {
	f.note(ctx, audit.Actor{})
	return domain.DefaultZaloChannelSetting(), f.err
}

func (f *fakeZaloLinks) IssuePairingCode(ctx context.Context, a audit.Actor) (app.PairingCodeView, error) {
	f.note(ctx, a)
	return app.PairingCodeView{Code: "ABCD2345", ExpiresAt: time.Date(2026, 10, 5, 1, 10, 0, 0, time.UTC),
		ChatURL: "https://zalo.me/bot"}, f.err
}

func (f *fakeZaloLinks) Unlink(ctx context.Context, a audit.Actor) error {
	f.note(ctx, a)
	return f.err
}

func (f *fakeZaloLinks) SendTestMessage(ctx context.Context, a audit.Actor) error {
	f.note(ctx, a)
	return f.err
}

func (f *fakeZaloLinks) SaveSettings(ctx context.Context, in domain.ZaloChannelSetting, a audit.Actor) (domain.ZaloChannelSetting, error) {
	f.note(ctx, a)
	f.lastSave = in
	if f.err != nil {
		return domain.ZaloChannelSetting{}, f.err
	}
	v, err := domain.NormalizeZaloChannelSetting(in)
	if err != nil {
		return domain.ZaloChannelSetting{}, fmt.Errorf("zalo: save: %w", err)
	}
	v.Saved, v.UpdatedAt, v.UpdatedBy = true, time.Date(2026, 10, 5, 2, 0, 0, 0, time.UTC), a.ID
	return v, nil
}

type zaloLinkServer struct {
	h       http.Handler
	fake    *fakeZaloLinks
	checker *checkerDanhMucGia
	keys    int
}

func newZaloLinkServer(t *testing.T) *zaloLinkServer {
	t.Helper()
	fake := &fakeZaloLinks{}
	checker := &checkerDanhMucGia{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	RegisterZaloLinks(mux, ZaloLinkDeps{Checker: checker, Reader: fake, Writer: fake, Log: log})
	var h http.Handler = mux
	h = chuTheGhi(h)
	h = idem.Middleware(moKhoIdemGia(), log)(h)
	h = httpx.TenantMiddleware(thuMucMau())(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)
	return &zaloLinkServer{h: h, fake: fake, checker: checker}
}

func (s *zaloLinkServer) grant(commune tenant.ID, perms ...authz.Perm) {
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

func (s *zaloLinkServer) call(t *testing.T, method, path, host string, p *authz.Principal, body string) *httptest.ResponseRecorder {
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
	r.Header.Set(idem.Header, fmt.Sprintf("01JIDEMZALOLINKS%010d", s.keys))
	if p != nil {
		r = r.WithContext(context.WithValue(r.Context(), khoaChuTheGhi{}, *p))
	}
	w := httptest.NewRecorder()
	s.h.ServeHTTP(w, r)
	return w
}

const zaloSettingsBody = `{"is_enabled":true,"kinds":["sap-den-han"],"quiet_start":"21:00","quiet_end":"06:00",` +
	`"overdue_start_after_days":null,"overdue_repeat_every_days":null}`

type zaloOwnRoute struct {
	name, method, path string
	ok                 int
}

func zaloOwnRoutes() []zaloOwnRoute {
	return []zaloOwnRoute{
		{"current", http.MethodGet, "/api/v1/zalo-links/current", http.StatusOK},
		{"unlink", http.MethodDelete, "/api/v1/zalo-links/current", http.StatusNoContent},
		{"pairing-code", http.MethodPost, "/api/v1/zalo-links/current/pairing-codes", http.StatusCreated},
		{"test-message", http.MethodPost, "/api/v1/zalo-links/current/test-messages", http.StatusOK},
	}
}

// zaloAdminRoutes — the wrong keys are real and adjacent: a content reader, and asset.update (a writer
// elsewhere on the same screens) must not configure the commune's channel.
func zaloAdminRoutes() []mapAssetRoute {
	return []mapAssetRoute{
		{"linked staff", http.MethodGet, "/api/v1/zalo-links", "", "admin.lookup", "content.read", http.StatusOK},
		{"get settings", http.MethodGet, "/api/v1/zalo-channel-settings", "", "admin.lookup", "content.read", http.StatusOK},
		{"put settings", http.MethodPut, "/api/v1/zalo-channel-settings", zaloSettingsBody, "admin.lookup", "asset.update", http.StatusOK},
	}
}

// --- the caller's own link: AnyAuthenticated -------------------------------------------------------

func TestZaloOwnRoutes_401NoSession(t *testing.T) {
	for _, rt := range zaloOwnRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			s := newZaloLinkServer(t)
			doiMa(t, s.call(t, rt.method, rt.path, hostA, nil, ""), http.StatusUnauthorized)
			if s.fake.calls != 0 {
				t.Error("no session and the use case ran")
			}
		})
	}
}

func TestZaloOwnRoutes_401SessionOfAnotherCommune(t *testing.T) {
	for _, rt := range zaloOwnRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			s := newZaloLinkServer(t)
			doiMa(t, s.call(t, rt.method, rt.path, hostA, canBoGhi(xaB), ""), http.StatusUnauthorized)
			if s.fake.calls != 0 {
				t.Error("a session of commune B reached commune A's link")
			}
		})
	}
}

func TestZaloOwnRoutes_403NonStaff(t *testing.T) {
	for _, rt := range zaloOwnRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			s := newZaloLinkServer(t)
			citizen := &authz.Principal{ID: "cd-opaque", Kind: "citizen", TenantID: xaA}
			doiMa(t, s.call(t, rt.method, rt.path, hostA, citizen, ""), http.StatusForbidden)
			if s.fake.calls != 0 {
				t.Error("a non-staff principal reached the use case")
			}
		})
	}
}

func TestZaloOwnRoutes_500StaffWithoutCodeNoFallback(t *testing.T) {
	for _, rt := range zaloOwnRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			s := newZaloLinkServer(t)
			p := canBoGhi(xaA)
			p.Ma = ""
			doiMa(t, s.call(t, rt.method, rt.path, hostA, p, ""), http.StatusInternalServerError)
			if s.fake.calls != 0 {
				t.Error("no business code and the use case still ran — the internal id was used?")
			}
		})
	}
}

func TestZaloOwnRoutes_200TheSessionsOwnCode(t *testing.T) {
	for _, rt := range zaloOwnRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			s := newZaloLinkServer(t)
			doiMa(t, s.call(t, rt.method, rt.path, hostA, canBoGhi(xaA), ""), rt.ok)
			if s.fake.lastCommune != xaA || s.fake.lastActor.ID != maCanBoGhi || s.fake.lastActor.IP == "" {
				t.Errorf("ran in %q as %+v, want commune %q, code %q, an IP", s.fake.lastCommune, s.fake.lastActor, xaA, maCanBoGhi)
			}
		})
	}
}

// --- the commune's administration: admin.lookup --------------------------------------------------

func TestZaloAdminRoutesAskForSeededKey(t *testing.T) {
	for _, tc := range zaloAdminRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newZaloLinkServer(t)
			s.call(t, tc.method, tc.path, hostA, canBoGhi(xaA), tc.body)
			if len(s.checker.hoiGi) == 0 || s.checker.hoiGi[0] != tc.perm {
				t.Fatalf("asked for %v, want %q (seeded at service-identity/migrations/0001_init.sql:281)", s.checker.hoiGi, tc.perm)
			}
		})
	}
}

func TestZaloAdminRoutes_401NoSession(t *testing.T) {
	for _, tc := range zaloAdminRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newZaloLinkServer(t)
			s.grant(xaA, tc.perm)
			doiMa(t, s.call(t, tc.method, tc.path, hostA, nil, tc.body), http.StatusUnauthorized)
			if s.fake.calls != 0 {
				t.Error("no session and the use case ran")
			}
		})
	}
}

func TestZaloAdminRoutes_403WrongPermission(t *testing.T) {
	for _, tc := range zaloAdminRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newZaloLinkServer(t)
			s.grant(xaA, tc.wrongPerm)
			doiMa(t, s.call(t, tc.method, tc.path, hostA, canBoGhi(xaA), tc.body), http.StatusForbidden)
			if s.fake.calls != 0 {
				t.Error("wrong permission and the use case ran")
			}
		})
	}
}

func TestZaloAdminRoutes_403RightPermissionWrongCommune(t *testing.T) {
	for _, tc := range zaloAdminRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newZaloLinkServer(t)
			s.grant(xaA, tc.perm)
			doiMa(t, s.call(t, tc.method, tc.path, hostB, canBoGhi(xaB), tc.body), http.StatusForbidden)
			if s.fake.calls != 0 {
				t.Error("a grant in another commune was enough")
			}
		})
	}
}

func TestZaloAdminRoutes_200RightPermissionRightCommune(t *testing.T) {
	for _, tc := range zaloAdminRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newZaloLinkServer(t)
			s.grant(xaA, tc.perm)
			doiMa(t, s.call(t, tc.method, tc.path, hostA, canBoGhi(xaA), tc.body), tc.ok)
			if s.fake.lastCommune != xaA {
				t.Errorf("ran in %q, want %q", s.fake.lastCommune, xaA)
			}
		})
	}
}

// --- shapes and mapping ----------------------------------------------------------------------------

func TestZaloShapesMatchTheWebContract(t *testing.T) {
	s := newZaloLinkServer(t)
	s.grant(xaA, "admin.lookup")
	checkKeys := func(path, method string, want ...string) map[string]any {
		t.Helper()
		w := s.call(t, method, path, hostA, canBoGhi(xaA), "")
		var m map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
			t.Fatalf("%s: %v — %s", path, err, w.Body.String())
		}
		for _, k := range want {
			if _, ok := m[k]; !ok {
				t.Errorf("%s: no field %q in %s", path, k, w.Body.String())
			}
		}
		if strings.Contains(w.Body.String(), "chat_id") {
			t.Errorf("%s: a chat_id field left the API", path)
		}
		return m
	}
	checkKeys("/api/v1/zalo-links/current", http.MethodGet, "linked", "linked_at", "bot_name", "chat_url", "channel_enabled")
	checkKeys("/api/v1/zalo-links/current/pairing-codes", http.MethodPost, "code", "expires_at", "chat_url")
	set := checkKeys("/api/v1/zalo-channel-settings", http.MethodGet, "is_enabled", "kinds", "quiet_start", "quiet_end",
		"overdue_start_after_days", "overdue_repeat_every_days")
	if set["quiet_start"] != "21:00" || set["quiet_end"] != "06:00" || set["is_enabled"] != false {
		t.Errorf("unsaved settings = %v, want off with 21:00–06:00", set)
	}
	if _, saved := set["updated_by"]; saved {
		t.Error("an unsaved commune carries updated_by")
	}
	list := checkKeys("/api/v1/zalo-links", http.MethodGet, "items")
	items, _ := list["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items = %v", list)
	}
	for _, k := range []string{"staff_code", "staff_name", "linked_at"} {
		if _, ok := items[0].(map[string]any)[k]; !ok {
			t.Errorf("linked staff item lacks %q", k)
		}
	}
}

func TestZaloPairingCodeIsNotCacheable(t *testing.T) {
	s := newZaloLinkServer(t)
	w := s.call(t, http.MethodPost, "/api/v1/zalo-links/current/pairing-codes", hostA, canBoGhi(xaA), "")
	doiMa(t, w, http.StatusCreated)
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q — a live credential must not be cached", w.Header().Get("Cache-Control"))
	}
}

func TestZaloSettingsPutValidation(t *testing.T) {
	s := newZaloLinkServer(t)
	s.grant(xaA, "admin.lookup")
	put := func(body string) *httptest.ResponseRecorder {
		return s.call(t, http.MethodPut, "/api/v1/zalo-channel-settings", hostA, canBoGhi(xaA), body)
	}
	doiMa(t, put(`not json`), http.StatusBadRequest)
	doiMa(t, put(`{"kinds":[],"quiet_start":"21:00","quiet_end":"06:00"}`), http.StatusBadRequest)
	w := put(`{"is_enabled":true,"kinds":["sap-den-han"],"quiet_start":"25:00","quiet_end":"06:00"}`)
	doiMa(t, w, http.StatusUnprocessableEntity)
	if loiTra(t, w).Code != "invalid_quiet_time" {
		t.Errorf("code = %q", loiTra(t, w).Code)
	}
	w = put(`{"is_enabled":true,"kinds":["qua-han"],"quiet_start":"21:00","quiet_end":"06:00"}`)
	doiMa(t, w, http.StatusUnprocessableEntity)
	if e := loiTra(t, w); e.Code != "overdue_cadence_required" || e.Message == "" {
		t.Errorf("error = %+v", e)
	}
	w = put(zaloSettingsBody)
	doiMa(t, w, http.StatusOK)
	if s.fake.lastActor.ID != maCanBoGhi {
		t.Errorf("saved as %q, want the session's business code", s.fake.lastActor.ID)
	}
	var out zaloChannelSettingsOut
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out.UpdatedBy != maCanBoGhi || out.UpdatedAt == nil || !out.IsEnabled {
		t.Errorf("reply = %+v", out)
	}
}

func TestZaloErrorMapping(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{app.ErrZaloBotNotConfigured, http.StatusConflict, "zalo_bot_not_configured"},
		{app.ErrZaloChannelOff, http.StatusConflict, "zalo_channel_disabled"},
		{app.ErrZaloNotLinked, http.StatusConflict, "zalo_not_linked"},
		{&app.ZaloSendError{Class: domain.ZaloCallRateLimited}, http.StatusBadGateway, "zalo_send_failed"},
		{fmt.Errorf("x: %w", errors.New("boom")), http.StatusInternalServerError, "internal"},
	}
	for _, c := range cases {
		s := newZaloLinkServer(t)
		s.fake.err = c.err
		w := s.call(t, http.MethodPost, "/api/v1/zalo-links/current/test-messages", hostA, canBoGhi(xaA), "")
		doiMa(t, w, c.status)
		if e := loiTra(t, w); e.Code != c.code || strings.Contains(e.Message, "boom") {
			t.Errorf("%v → %+v", c.err, e)
		}
	}
	s := newZaloLinkServer(t)
	s.grant(xaA, "admin.lookup")
	s.fake.err = fmt.Errorf("%w: down", app.ErrStaffNamesUnavailable)
	w := s.call(t, http.MethodGet, "/api/v1/zalo-links", hostA, canBoGhi(xaA), "")
	doiMa(t, w, http.StatusServiceUnavailable)
	if loiTra(t, w).Code != "staff_names_unavailable" {
		t.Errorf("code = %q", loiTra(t, w).Code)
	}
}

func TestRegisterZaloLinksRefusesIncompleteWiring(t *testing.T) {
	for name, d := range map[string]ZaloLinkDeps{
		"checker": {Reader: &fakeZaloLinks{}, Writer: &fakeZaloLinks{}},
		"reader":  {Checker: &checkerDanhMucGia{}, Writer: &fakeZaloLinks{}},
		"writer":  {Checker: &checkerDanhMucGia{}, Reader: &fakeZaloLinks{}},
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("incomplete wiring accepted")
				}
			}()
			RegisterZaloLinks(http.NewServeMux(), d)
		})
	}
}
