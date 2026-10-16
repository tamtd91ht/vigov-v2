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
	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// The seven routes of the commune's Zalo OA on the staff harness (checkerDanhMucGia, chuTheGhi, canBoGhi).
// The fake records WHICH commune and WHICH actor reached it; the transactions, the sealing and the entries
// are internal/app's tests.
//
//	admin.zalo: 401 no session · 403 wrong key · 403 right key other commune · 200

// testOALink is a fixture, not a credential: the shape of Zalo's permission link with a fake state.
const testOALink = "https://oauth.zaloapp.com/v4/oa/permission?app_id=1&state=FAKE-STATE-NOT-REAL"

var oaTestAt = time.Date(2026, 10, 16, 2, 0, 0, 0, time.UTC)

type fakeCommuneOAs struct {
	calls       int
	lastCommune tenant.ID
	lastActor   audit.Actor
	lastReason  string
	lastID      string
	lastMile    *string
	milePassed  bool
	err         error
	hasOA       bool
	retired     bool
}

func (f *fakeCommuneOAs) note(ctx context.Context, a audit.Actor) {
	f.calls++
	f.lastCommune, f.lastActor = tenant.MustFrom(ctx), a
}

func sampleOA() domain.CommuneZaloOA {
	return domain.CommuneZaloOA{ID: "01JOAROW000000000000000001", OAID: "579745863508352884", OAName: "UBND Xã A",
		AccessExpiresAt: oaTestAt.Add(20 * time.Hour), RefreshIssuedAt: oaTestAt, TokenRefreshedAt: oaTestAt,
		AuthorizedAt: oaTestAt, AuthorizedBy: "CB-00099", LastCheckAt: oaTestAt, LastCheckResult: domain.ZaloCallOK}
}

func sampleTemplate(attached bool) domain.ZaloOATemplate {
	t := domain.ZaloOATemplate{ID: "01JTPL0000000000000000001", OARowID: "01JOAROW000000000000000001", TemplateID: "1001",
		TemplateName: "Mẫu tiếp nhận", ZaloStatus: domain.ZaloTemplateStatusEnabled, ZaloQuality: "HIGH", ZaloTag: "TRANSACTION",
		Params: []domain.TemplateParam{{Name: "ma_tra_cuu", Required: true, Type: "STRING", MaxLength: 30}}, SyncedAt: oaTestAt}
	if attached {
		t.Milestone, t.MilestoneSetAt, t.MilestoneSetBy = domain.MilestoneReceived, oaTestAt, "CB-00099"
	}
	return t
}

func (f *fakeCommuneOAs) Current(ctx context.Context) (app.CommuneZaloOAView, error) {
	f.note(ctx, audit.Actor{})
	if !f.hasOA {
		return app.CommuneZaloOAView{Configured: true}, f.err
	}
	return app.CommuneZaloOAView{HasOA: true, OA: sampleOA(), TemplateCount: 3, AttachedCount: 1, Configured: true}, f.err
}

func (f *fakeCommuneOAs) StartAuthorization(ctx context.Context, a audit.Actor) (app.AuthorizationLink, error) {
	f.note(ctx, a)
	if f.err != nil {
		return app.AuthorizationLink{}, f.err
	}
	return app.AuthorizationLink{URL: testOALink, ExpiresAt: oaTestAt.Add(10 * time.Minute)}, nil
}

func (f *fakeCommuneOAs) Check(ctx context.Context, a audit.Actor) (app.ZaloOACheckOutput, error) {
	f.note(ctx, a)
	return app.ZaloOACheckOutput{Outcome: domain.ZaloCallOK, CheckedAt: oaTestAt, OAName: "UBND Xã A"}, f.err
}

func (f *fakeCommuneOAs) Disconnect(ctx context.Context, reason string, a audit.Actor) (app.DisconnectZaloOAOutput, error) {
	f.note(ctx, a)
	f.lastReason = reason
	if !f.retired {
		return app.DisconnectZaloOAOutput{}, f.err
	}
	return app.DisconnectZaloOAOutput{Retired: true, DetachedTemplateCount: 2, RevokeNotice: domain.CommuneZaloOARevokeNotice}, f.err
}

func (f *fakeCommuneOAs) ListTemplates(ctx context.Context) ([]domain.ZaloOATemplate, error) {
	f.note(ctx, audit.Actor{})
	return []domain.ZaloOATemplate{sampleTemplate(true), sampleTemplate(false)}, f.err
}

func (f *fakeCommuneOAs) SyncTemplates(ctx context.Context, a audit.Actor) (app.TemplateSyncResult, error) {
	f.note(ctx, a)
	return app.TemplateSyncResult{Loaded: 4, Enabled: 3, Matching: 2, ChangedTemplateIDs: []string{"1001", "1002"}, SyncedAt: oaTestAt}, f.err
}

func (f *fakeCommuneOAs) SetMilestone(ctx context.Context, id string, m *string, a audit.Actor) (domain.ZaloOATemplate, error) {
	f.note(ctx, a)
	f.lastID, f.lastMile, f.milePassed = id, m, true
	if f.err != nil {
		return domain.ZaloOATemplate{}, f.err
	}
	return sampleTemplate(m != nil), nil
}

type fakeStaffNames struct {
	err   error
	asked []string
}

// vi-name-ok: the method of the existing core/identityclient.Client that app.StaffNames mirrors and this fake must match.
func (n *fakeStaffNames) TenCanBoTheoMa(_ context.Context, codes []string) (map[string]identityclient.TenCanBo, error) {
	n.asked = append(n.asked, codes...)
	if n.err != nil {
		return nil, n.err
	}
	out := map[string]identityclient.TenCanBo{}
	for _, c := range codes {
		out[c] = identityclient.TenCanBo{HoTen: "Cán bộ " + c}
	}
	return out, nil
}

type communeOAServer struct {
	h       http.Handler
	fake    *fakeCommuneOAs
	names   *fakeStaffNames
	checker *checkerDanhMucGia
	keys    int
}

func newCommuneOAServer(t *testing.T) *communeOAServer {
	t.Helper()
	fake := &fakeCommuneOAs{hasOA: true, retired: true}
	names := &fakeStaffNames{}
	checker := &checkerDanhMucGia{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	RegisterZaloCommuneOA(mux, ZaloCommuneOADeps{Checker: checker, OAs: fake, Names: names, Log: log})
	var h http.Handler = mux
	h = chuTheGhi(h)
	h = idem.Middleware(moKhoIdemGia(), log)(h)
	h = httpx.TenantMiddleware(thuMucMau())(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)
	return &communeOAServer{h: h, fake: fake, names: names, checker: checker}
}

func (s *communeOAServer) grant(commune tenant.ID, perms ...authz.Perm) {
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

func (s *communeOAServer) call(t *testing.T, method, path, host string, p *authz.Principal, body string) *httptest.ResponseRecorder {
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
	r.Header.Set(idem.Header, fmt.Sprintf("01JIDEMCOMMUNEOA%010d", s.keys))
	if p != nil {
		r = r.WithContext(context.WithValue(r.Context(), khoaChuTheGhi{}, *p))
	}
	w := httptest.NewRecorder()
	s.h.ServeHTTP(w, r)
	return w
}

const (
	communeOAPath  = "/api/v1/zalo-oas/current"
	disconnectBody = `{"reason":"Xã đổi sang OA mới"}`
	attachBody     = `{"milestone":"da-tiep-nhan"}`
	oaTemplatePath = communeOAPath + "/templates/01JTPL0000000000000000001"
)

// communeOARoutes — the wrong keys are real and adjacent: a content reader, asset.update, the audit
// reader and the old catalogue key must not configure the commune's OA.
func communeOARoutes() []mapAssetRoute {
	return []mapAssetRoute{
		{"get", http.MethodGet, communeOAPath, "", "admin.zalo", "content.read", http.StatusOK},
		{"delete", http.MethodDelete, communeOAPath, disconnectBody, "admin.zalo", "admin.audit", http.StatusOK},
		{"link", http.MethodPost, communeOAPath + "/authorization-links", "", "admin.zalo", "asset.update", http.StatusCreated},
		{"check", http.MethodPost, communeOAPath + "/check", "", "admin.zalo", "content.read", http.StatusOK},
		{"templates", http.MethodGet, communeOAPath + "/templates", "", "admin.zalo", "content.read", http.StatusOK},
		{"sync", http.MethodPost, communeOAPath + "/templates/sync", "", "admin.zalo", "asset.update", http.StatusOK},
		{"patch", http.MethodPatch, oaTemplatePath, attachBody, "admin.zalo", "admin.lookup", http.StatusOK},
	}
}

func TestCommuneOARoutesAskForSeededKey(t *testing.T) {
	for _, tc := range communeOARoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newCommuneOAServer(t)
			s.call(t, tc.method, tc.path, hostA, canBoGhi(xaA), tc.body)
			// Compared with a LITERAL: a fake checker grants any string (rule 5, invariant 3c).
			if len(s.checker.hoiGi) == 0 || s.checker.hoiGi[0] != "admin.zalo" {
				t.Fatalf("asked for %v, want admin.zalo (seeded at service-identity/migrations/0032_split_configuration_permissions.sql)", s.checker.hoiGi)
			}
		})
	}
}

func TestCommuneOARoutes_401NoSession(t *testing.T) {
	for _, tc := range communeOARoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newCommuneOAServer(t)
			s.grant(xaA, tc.perm)
			doiMa(t, s.call(t, tc.method, tc.path, hostA, nil, tc.body), http.StatusUnauthorized)
			if s.fake.calls != 0 {
				t.Error("no session and the use case ran")
			}
		})
	}
}

func TestCommuneOARoutes_403WrongPermission(t *testing.T) {
	for _, tc := range communeOARoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newCommuneOAServer(t)
			s.grant(xaA, tc.wrongPerm)
			doiMa(t, s.call(t, tc.method, tc.path, hostA, canBoGhi(xaA), tc.body), http.StatusForbidden)
			if s.fake.calls != 0 {
				t.Error("wrong permission and the use case ran")
			}
		})
	}
}

func TestCommuneOARoutes_403RightPermissionWrongCommune(t *testing.T) {
	// Signed in at B as a member of B; the grant is in A. A checker ignoring the commune would let A's
	// administrator connect, or disconnect, B's OA — the sender B's citizens see.
	for _, tc := range communeOARoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newCommuneOAServer(t)
			s.grant(xaA, tc.perm)
			doiMa(t, s.call(t, tc.method, tc.path, hostB, canBoGhi(xaB), tc.body), http.StatusForbidden)
			if s.fake.calls != 0 {
				t.Error("a grant in another commune was enough")
			}
		})
	}
}

func TestCommuneOARoutes_200RightPermissionRightCommune(t *testing.T) {
	for _, tc := range communeOARoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newCommuneOAServer(t)
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

// --- shapes (web-admin/src/lib/api/zalo-oa.ts) --------------------------------------------------------

func keysOf(t *testing.T, raw []byte) map[string]json.RawMessage {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("%v: %s", err, raw)
	}
	return m
}

func wantKeys(t *testing.T, what string, m map[string]json.RawMessage, keys ...string) {
	t.Helper()
	for _, k := range keys {
		if _, ok := m[k]; !ok {
			t.Errorf("%s lacks %q", what, k)
		}
	}
	if len(m) != len(keys) {
		t.Errorf("%s has %d keys, want exactly %v: %v", what, len(m), keys, m)
	}
}

func TestCommuneOAGetMatchesWebContractAndCarriesNoToken(t *testing.T) {
	s := newCommuneOAServer(t)
	s.grant(xaA, "admin.zalo")
	w := s.call(t, http.MethodGet, communeOAPath, hostA, canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	top := keysOf(t, w.Body.Bytes())
	wantKeys(t, "current", top, "configured", "has_oa", "oa", "template_count", "attached_count")
	oa := keysOf(t, top["oa"])
	wantKeys(t, "oa", oa, "oa_id", "oa_name", "authorized_at", "authorized_by", "authorized_by_name", "access_expires_at",
		"token_refreshed_at", "needs_reauthorization", "last_check")
	if string(oa["authorized_by_name"]) != `"Cán bộ CB-00099"` || string(oa["needs_reauthorization"]) != "null" {
		t.Errorf("oa = %s", top["oa"])
	}
	for _, bad := range []string{"sealed", "access_token\"", "refresh_token\"", "verifier"} {
		if strings.Contains(w.Body.String(), bad) {
			t.Fatalf("GET carries %q: %s", bad, w.Body.String())
		}
	}
	s.fake.hasOA = false
	w = s.call(t, http.MethodGet, communeOAPath, hostA, canBoGhi(xaA), "")
	if !strings.Contains(w.Body.String(), `"has_oa":false`) || !strings.Contains(w.Body.String(), `"oa":null`) {
		t.Errorf("no OA = %s", w.Body.String())
	}
}

// Identity down: names are display only — the read still answers, the code stands alone.
func TestCommuneOANamesDegradeToEmpty(t *testing.T) {
	s := newCommuneOAServer(t)
	s.grant(xaA, "admin.zalo")
	s.names.err = errors.New("identity down")
	w := s.call(t, http.MethodGet, communeOAPath, hostA, canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"authorized_by_name":""`) || !strings.Contains(w.Body.String(), `"authorized_by":"CB-00099"`) {
		t.Errorf("reply = %s", w.Body.String())
	}
}

func TestCommuneOALinkIs201UncachedWithURL(t *testing.T) {
	s := newCommuneOAServer(t)
	s.grant(xaA, "admin.zalo")
	w := s.call(t, http.MethodPost, communeOAPath+"/authorization-links", hostA, canBoGhi(xaA), "")
	doiMa(t, w, http.StatusCreated)
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q — the URL carries a one-time state", w.Header().Get("Cache-Control"))
	}
	wantKeys(t, "link", keysOf(t, w.Body.Bytes()), "url", "expires_at")
	if !strings.Contains(w.Body.String(), "FAKE-STATE-NOT-REAL") {
		t.Errorf("reply = %s", w.Body.String())
	}
}

func TestCommuneOACheckSyncDeleteShapes(t *testing.T) {
	s := newCommuneOAServer(t)
	s.grant(xaA, "admin.zalo")

	w := s.call(t, http.MethodPost, communeOAPath+"/check", hostA, canBoGhi(xaA), "")
	wantKeys(t, "check", keysOf(t, w.Body.Bytes()), "result", "checked_at", "oa_name")

	w = s.call(t, http.MethodPost, communeOAPath+"/templates/sync", hostA, canBoGhi(xaA), "")
	m := keysOf(t, w.Body.Bytes())
	wantKeys(t, "sync", m, "loaded", "enabled", "matching", "changed", "synced_at")
	if string(m["changed"]) != "2" {
		t.Errorf("changed = %s", m["changed"])
	}

	w = s.call(t, http.MethodDelete, communeOAPath, hostA, canBoGhi(xaA), disconnectBody)
	m = keysOf(t, w.Body.Bytes())
	wantKeys(t, "delete", m, "disconnected", "detached_template_count", "revoke_notice")
	if s.fake.lastReason != "Xã đổi sang OA mới" || !strings.Contains(w.Body.String(), "OA Manager") {
		t.Errorf("reason %q reply %s", s.fake.lastReason, w.Body.String())
	}
	s.fake.retired = false
	w = s.call(t, http.MethodDelete, communeOAPath, hostA, canBoGhi(xaA), disconnectBody)
	if !strings.Contains(w.Body.String(), `"disconnected":false`) || !strings.Contains(w.Body.String(), `"revoke_notice":""`) {
		t.Errorf("nothing retired = %s", w.Body.String())
	}
}

func TestCommuneOATemplatesShape(t *testing.T) {
	s := newCommuneOAServer(t)
	s.grant(xaA, "admin.zalo")
	w := s.call(t, http.MethodGet, communeOAPath+"/templates", hostA, canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	var out struct {
		Items    []map[string]json.RawMessage `json:"items"`
		SyncedAt *time.Time                   `json:"synced_at"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 2 || out.SyncedAt == nil || !out.SyncedAt.Equal(oaTestAt) {
		t.Fatalf("reply = %s", w.Body.String())
	}
	keys := []string{"id", "template_id", "template_name", "zalo_status", "zalo_quality", "zalo_tag", "params", "params_match",
		"missing_params", "milestone", "milestone_set_at", "milestone_set_by", "milestone_set_by_name"}
	wantKeys(t, "attached", out.Items[0], keys...)
	wantKeys(t, "detached", out.Items[1], keys...)
	if string(out.Items[0]["milestone"]) != `"da-tiep-nhan"` || string(out.Items[0]["milestone_set_by_name"]) != `"Cán bộ CB-00099"` {
		t.Errorf("attached = %v", out.Items[0])
	}
	if string(out.Items[1]["milestone"]) != "null" || string(out.Items[1]["milestone_set_by_name"]) != "null" {
		t.Errorf("detached = %v", out.Items[1])
	}
	if string(out.Items[0]["missing_params"]) != `["moc_nhan","viec_tiep_theo"]` {
		t.Errorf("missing_params = %s", out.Items[0]["missing_params"])
	}
}

func TestCommuneOAPatchNeedsMilestoneKey(t *testing.T) {
	s := newCommuneOAServer(t)
	s.grant(xaA, "admin.zalo")

	doiMa(t, s.call(t, http.MethodPatch, oaTemplatePath, hostA, canBoGhi(xaA), `{}`), http.StatusBadRequest)
	doiMa(t, s.call(t, http.MethodPatch, oaTemplatePath, hostA, canBoGhi(xaA), `{"milestone":"da-dong","x":1}`), http.StatusBadRequest)
	if s.fake.milePassed {
		t.Fatal("an absent or polluted body reached the use case")
	}

	w := s.call(t, http.MethodPatch, oaTemplatePath, hostA, canBoGhi(xaA), `{"milestone":null}`)
	doiMa(t, w, http.StatusOK)
	if !s.fake.milePassed || s.fake.lastMile != nil || s.fake.lastID != "01JTPL0000000000000000001" {
		t.Fatalf("detach passed id %q milestone %v", s.fake.lastID, s.fake.lastMile)
	}
	w = s.call(t, http.MethodPatch, oaTemplatePath, hostA, canBoGhi(xaA), attachBody)
	if s.fake.lastMile == nil || *s.fake.lastMile != domain.MilestoneReceived || !strings.Contains(w.Body.String(), `"milestone":"da-tiep-nhan"`) {
		t.Fatalf("attach = %s", w.Body.String())
	}
}

func TestCommuneOAErrorMapping(t *testing.T) {
	cases := []struct {
		name   string
		method string
		path   string
		body   string
		err    error
		status int
		code   string
	}{
		{"not configured", http.MethodPost, communeOAPath + "/authorization-links", "", app.ErrZaloOANotConfigured, 503, "zalo_oa_not_configured"},
		{"no KEK", http.MethodPost, communeOAPath + "/authorization-links", "", crypto.ErrNotConfigured, 503, "encryption_not_configured"},
		{"no OA", http.MethodPost, communeOAPath + "/check", "", app.ErrCommuneZaloOAMissing, 409, "zalo_oa_missing"},
		{"reauth", http.MethodPost, communeOAPath + "/check", "", app.ErrZaloOANeedsReauthorization, 409, "zalo_oa_needs_reauthorization"},
		{"changed", http.MethodPost, communeOAPath + "/check", "", fmt.Errorf("x: %w", app.ErrCommuneZaloOAChanged), 409, "zalo_oa_changed"},
		{"refresh down", http.MethodPost, communeOAPath + "/check", "", &app.ZaloOACallError{Step: "refresh", Class: domain.ZaloCallUnavailable}, 502, "zalo_unavailable"},
		{"token rejected", http.MethodPost, communeOAPath + "/templates/sync", "", &app.ZaloOACallError{Step: "template/all", Class: domain.ZaloCallTokenRejected}, 422, "zalo_token_rejected"},
		{"zalo down", http.MethodPost, communeOAPath + "/templates/sync", "", &app.ZaloOACallError{Step: "template/all", Class: domain.ZaloCallUnavailable}, 502, "zalo_unavailable"},
		{"taken", http.MethodPatch, oaTemplatePath, attachBody, commsstore.ErrMilestoneTaken, 409, "milestone_taken"},
		{"not enabled", http.MethodPatch, oaTemplatePath, attachBody, domain.ErrZaloOATemplateNotEnabled, 422, "template_not_attachable"},
		{"params", http.MethodPatch, oaTemplatePath, attachBody, &domain.TemplateParamsMissingError{Missing: []string{"moc_nhan"}}, 422, "template_not_attachable"},
		{"no template", http.MethodPatch, oaTemplatePath, attachBody, app.ErrZaloOATemplateMissing, 404, "not_found"},
		{"bad milestone", http.MethodPatch, oaTemplatePath, attachBody, domain.ErrZaloOAMilestoneUnknown, 400, "invalid_request"},
		{"bad reason", http.MethodDelete, communeOAPath, disconnectBody, domain.ErrCommuneZaloOARetireReason, 400, "invalid_request"},
		{"broken", http.MethodPost, communeOAPath + "/check", "", errors.New("boom: " + string(xaB)), 500, "internal"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newCommuneOAServer(t)
			s.grant(xaA, "admin.zalo")
			s.fake.err = c.err
			w := s.call(t, c.method, c.path, hostA, canBoGhi(xaA), c.body)
			doiMa(t, w, c.status)
			e := loiTra(t, w)
			if e.Code != c.code {
				t.Errorf("code = %q, want %q", e.Code, c.code)
			}
			for _, bad := range []string{string(xaB), "boom", "zalo_oa_xa:", "template/all"} {
				if strings.Contains(e.Message, bad) {
					t.Errorf("message leaks %q: %q", bad, e.Message)
				}
			}
			if c.name == "params" && !strings.Contains(e.Message, "moc_nhan") {
				t.Errorf("the missing parameter is not named: %q", e.Message)
			}
		})
	}
}

func TestRegisterZaloCommuneOARefusesIncompleteWiring(t *testing.T) {
	for name, d := range map[string]ZaloCommuneOADeps{
		"checker": {OAs: &fakeCommuneOAs{}, Names: &fakeStaffNames{}},
		"oas":     {Checker: &checkerDanhMucGia{}, Names: &fakeStaffNames{}},
		"names":   {Checker: &checkerDanhMucGia{}, OAs: &fakeCommuneOAs{}},
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("incomplete wiring accepted")
				}
			}()
			RegisterZaloCommuneOA(http.NewServeMux(), d)
		})
	}
}
