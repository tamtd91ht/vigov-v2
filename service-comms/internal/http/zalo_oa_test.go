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

// The seven routes of the commune's Zalo OAs (ADR 0099 #1, #10) on the staff harness (checkerDanhMucGia,
// chuTheGhi, canBoGhi). The fake records WHICH commune, WHICH actor and WHICH id reached it, and answers
// as the scoped use case does: an id that is not the commune's is "missing". The transactions, the sealing
// and the entries are internal/app's tests; the scoping of the read itself is internal/store's.
//
//	admin.zalo: 401 no session · 403 wrong key · 403 right key other commune · 200 · 404 another commune's id

// testOALink is a fixture, not a credential: the shape of Zalo's permission link with a fake state.
const testOALink = "https://oauth.zaloapp.com/v4/oa/permission?app_id=1&state=FAKE-STATE-NOT-REAL"

var oaTestAt = time.Date(2026, 10, 16, 2, 0, 0, 0, time.UTC)

const (
	oaRowA  = "01JOAROW000000000000000001" // commune A's OA
	oaRowA2 = "01JOAROW000000000000000002" // commune A's second OA
	oaRowB  = "01JOAROWXAB000000000000001" // commune B's OA
	tplA    = "01JTPL0000000000000000001"  // a template of A's OA
	tplB    = "01JTPLXAB0000000000000001"  // a template of B's OA
)

type fakeCommuneOAs struct {
	calls       int
	lastCommune tenant.ID
	lastActor   audit.Actor
	lastReason  string
	lastID      string
	lastMile    *string
	milePassed  bool
	err         error
	noOA        bool
}

func (f *fakeCommuneOAs) note(ctx context.Context, a audit.Actor) {
	f.calls++
	f.lastCommune, f.lastActor = tenant.MustFrom(ctx), a
}

// ownsOA / ownsTemplate answer as the scoped store does: only the context's commune's rows exist.
func ownsOA(ctx context.Context, id string) bool {
	switch tenant.MustFrom(ctx) {
	case xaA:
		return id == oaRowA || id == oaRowA2
	case xaB:
		return id == oaRowB
	}
	return false
}

func ownsTemplate(ctx context.Context, id string) bool {
	switch tenant.MustFrom(ctx) {
	case xaA:
		return id == tplA
	case xaB:
		return id == tplB
	}
	return false
}

func sampleOA(id, oaID string) domain.CommuneZaloOA {
	return domain.CommuneZaloOA{ID: id, OAID: oaID, OAName: "UBND Xã A",
		AccessExpiresAt: oaTestAt.Add(20 * time.Hour), RefreshIssuedAt: oaTestAt, TokenRefreshedAt: oaTestAt,
		AuthorizedAt: oaTestAt, AuthorizedBy: "CB-00099", LastCheckAt: oaTestAt, LastCheckResult: domain.ZaloCallOK}
}

func sampleTemplate(attached bool) domain.ZaloOATemplate {
	t := domain.ZaloOATemplate{ID: tplA, OARowID: oaRowA, TemplateID: "1001",
		TemplateName: "Mẫu tiếp nhận", ZaloStatus: domain.ZaloTemplateStatusEnabled, ZaloQuality: "HIGH", ZaloTag: "TRANSACTION",
		Params: []domain.TemplateParam{{Name: "ma_tra_cuu", Required: true, Type: "STRING", MaxLength: 30}}, SyncedAt: oaTestAt}
	if attached {
		t.Milestone, t.MilestoneSetAt, t.MilestoneSetBy = domain.MilestoneReceived, oaTestAt, "CB-00099"
	}
	return t
}

func (f *fakeCommuneOAs) List(ctx context.Context) (app.CommuneZaloOAList, error) {
	f.note(ctx, audit.Actor{})
	if f.noOA {
		return app.CommuneZaloOAList{Configured: true}, f.err
	}
	second := sampleOA(oaRowA2, "222333444")
	second.AuthorizedBy = "CB-00100"
	second.NeedsReauthorizationSince, second.NeedsReauthorizationReason = oaTestAt, domain.ReauthorizationRefreshTokenRejected
	return app.CommuneZaloOAList{Configured: true, Items: []app.CommuneZaloOAItem{
		{OA: sampleOA(oaRowA, "579745863508352884"), TemplateCount: 3, AttachedCount: 1},
		{OA: second, TemplateCount: 0, AttachedCount: 0},
	}}, f.err
}

func (f *fakeCommuneOAs) StartAuthorization(ctx context.Context, a audit.Actor) (app.AuthorizationLink, error) {
	f.note(ctx, a)
	if f.err != nil {
		return app.AuthorizationLink{}, f.err
	}
	return app.AuthorizationLink{URL: testOALink, ExpiresAt: oaTestAt.Add(10 * time.Minute)}, nil
}

func (f *fakeCommuneOAs) Check(ctx context.Context, id string, a audit.Actor) (app.ZaloOACheckOutput, error) {
	f.note(ctx, a)
	f.lastID = id
	if !ownsOA(ctx, id) {
		return app.ZaloOACheckOutput{}, app.ErrCommuneZaloOAMissing
	}
	return app.ZaloOACheckOutput{Outcome: domain.ZaloCallOK, CheckedAt: oaTestAt, OAName: "UBND Xã A"}, f.err
}

func (f *fakeCommuneOAs) Disconnect(ctx context.Context, id, reason string, a audit.Actor) (app.DisconnectZaloOAOutput, error) {
	f.note(ctx, a)
	f.lastID, f.lastReason = id, reason
	if !ownsOA(ctx, id) {
		return app.DisconnectZaloOAOutput{}, app.ErrCommuneZaloOAMissing
	}
	if f.err != nil {
		return app.DisconnectZaloOAOutput{}, f.err
	}
	return app.DisconnectZaloOAOutput{Retired: true, DetachedTemplateCount: 2, RevokeNotice: domain.CommuneZaloOARevokeNotice}, nil
}

func (f *fakeCommuneOAs) ListTemplates(ctx context.Context, id string) ([]domain.ZaloOATemplate, error) {
	f.note(ctx, audit.Actor{})
	f.lastID = id
	if !ownsOA(ctx, id) {
		return nil, app.ErrCommuneZaloOAMissing
	}
	return []domain.ZaloOATemplate{sampleTemplate(true), sampleTemplate(false)}, f.err
}

func (f *fakeCommuneOAs) SyncTemplates(ctx context.Context, id string, a audit.Actor) (app.TemplateSyncResult, error) {
	f.note(ctx, a)
	f.lastID = id
	if !ownsOA(ctx, id) {
		return app.TemplateSyncResult{}, app.ErrCommuneZaloOAMissing
	}
	return app.TemplateSyncResult{Loaded: 4, Enabled: 3, Matching: 2, ChangedTemplateIDs: []string{"1001", "1002"}, SyncedAt: oaTestAt}, f.err
}

func (f *fakeCommuneOAs) SetMilestone(ctx context.Context, id string, m *string, a audit.Actor) (domain.ZaloOATemplate, error) {
	f.note(ctx, a)
	f.lastID, f.lastMile, f.milePassed = id, m, true
	if !ownsTemplate(ctx, id) {
		return domain.ZaloOATemplate{}, app.ErrZaloOATemplateMissing
	}
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
	fake := &fakeCommuneOAs{}
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
	communeOAsPath = "/api/v1/zalo-oas"
	disconnectBody = `{"reason":"Xã đổi sang OA mới"}`
	attachBody     = `{"milestone":"da-tiep-nhan"}`
)

func oaPath(id string) string         { return communeOAsPath + "/" + id }
func oaTemplatePath(id string) string { return "/api/v1/zalo-oa-templates/" + id }

// communeOARoutes — the wrong keys are real and adjacent: a content reader, asset.update, the audit
// reader and the old catalogue key must not configure the commune's OAs.
func communeOARoutes() []mapAssetRoute {
	return []mapAssetRoute{
		{"list", http.MethodGet, communeOAsPath, "", "admin.zalo", "content.read", http.StatusOK},
		{"link", http.MethodPost, communeOAsPath + "/authorization-links", "", "admin.zalo", "asset.update", http.StatusCreated},
		{"delete", http.MethodDelete, oaPath(oaRowA), disconnectBody, "admin.zalo", "admin.audit", http.StatusOK},
		{"check", http.MethodPost, oaPath(oaRowA) + "/check", "", "admin.zalo", "content.read", http.StatusOK},
		{"templates", http.MethodGet, oaPath(oaRowA) + "/templates", "", "admin.zalo", "content.read", http.StatusOK},
		{"sync", http.MethodPost, oaPath(oaRowA) + "/templates/sync", "", "admin.zalo", "asset.update", http.StatusOK},
		{"patch", http.MethodPatch, oaTemplatePath(tplA), attachBody, "admin.zalo", "admin.lookup", http.StatusOK},
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

// An {id} of ANOTHER commune, asked with full rights in one's own: 404 not_found, with the very sentence
// an unknown id gets — the reply never tells "exists elsewhere" from "does not exist" (rule 1).
func TestCommuneOARoutes_404OtherCommunesID(t *testing.T) {
	const noOA, noTpl = "01JNOSUCHOA000000000000001", "01JNOSUCHTPL0000000000001"
	routes := []struct {
		name, method, path, body, unknown, id string
	}{
		{"delete", http.MethodDelete, oaPath(oaRowB), disconnectBody, oaPath(noOA), oaRowB},
		{"check", http.MethodPost, oaPath(oaRowB) + "/check", "", oaPath(noOA) + "/check", oaRowB},
		{"templates", http.MethodGet, oaPath(oaRowB) + "/templates", "", oaPath(noOA) + "/templates", oaRowB},
		{"sync", http.MethodPost, oaPath(oaRowB) + "/templates/sync", "", oaPath(noOA) + "/templates/sync", oaRowB},
		{"patch", http.MethodPatch, oaTemplatePath(tplB), attachBody, oaTemplatePath(noTpl), tplB},
	}
	for _, tc := range routes {
		t.Run(tc.name, func(t *testing.T) {
			s := newCommuneOAServer(t)
			s.grant(xaA, "admin.zalo")
			w := s.call(t, tc.method, tc.path, hostA, canBoGhi(xaA), tc.body)
			doiMa(t, w, http.StatusNotFound)
			other := loiTra(t, w)
			if other.Code != "not_found" || s.fake.lastCommune != xaA {
				t.Fatalf("code %q, commune %q", other.Code, s.fake.lastCommune)
			}
			if s.fake.lastID != tc.id {
				t.Errorf("the path id did not reach the use case: %q, want %q", s.fake.lastID, tc.id)
			}
			w = s.call(t, tc.method, tc.unknown, hostA, canBoGhi(xaA), tc.body)
			doiMa(t, w, http.StatusNotFound)
			if unknown := loiTra(t, w); unknown.Message != other.Message || strings.Contains(other.Message, string(xaB)) {
				t.Errorf("another commune's id %q vs unknown id %q — the two must read the same", other.Message, unknown.Message)
			}
		})
	}
}

// --- shapes (the contract web-admin D-6 builds on) -----------------------------------------------------

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

func TestCommuneOAListShapeCarriesEveryOAAndNoToken(t *testing.T) {
	s := newCommuneOAServer(t)
	s.grant(xaA, "admin.zalo")
	w := s.call(t, http.MethodGet, communeOAsPath, hostA, canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	top := keysOf(t, w.Body.Bytes())
	wantKeys(t, "list", top, "configured", "items")
	var items []map[string]json.RawMessage
	if err := json.Unmarshal(top["items"], &items); err != nil || len(items) != 2 {
		t.Fatalf("items = %s (%v)", top["items"], err)
	}
	for i, it := range items {
		wantKeys(t, fmt.Sprintf("item %d", i), it, "id", "oa_id", "oa_name", "authorized_at", "authorized_by",
			"authorized_by_name", "access_expires_at", "token_refreshed_at", "needs_reauthorization", "last_check",
			"template_count", "attached_count")
	}
	if string(items[0]["id"]) != `"`+oaRowA+`"` || string(items[0]["authorized_by_name"]) != `"Cán bộ CB-00099"` ||
		string(items[0]["needs_reauthorization"]) != "null" || string(items[0]["template_count"]) != "3" ||
		string(items[0]["attached_count"]) != "1" {
		t.Errorf("item 0 = %v", items[0])
	}
	if string(items[1]["authorized_by_name"]) != `"Cán bộ CB-00100"` ||
		!strings.Contains(string(items[1]["needs_reauthorization"]), domain.ReauthorizationRefreshTokenRejected) {
		t.Errorf("item 1 = %v", items[1])
	}
	// Both officers' names in ONE identity question.
	if len(s.names.asked) != 2 {
		t.Errorf("asked identity for %v", s.names.asked)
	}
	for _, bad := range []string{"sealed", "access_token\"", "refresh_token\"", "verifier"} {
		if strings.Contains(w.Body.String(), bad) {
			t.Fatalf("GET carries %q: %s", bad, w.Body.String())
		}
	}
	s.fake.noOA = true
	w = s.call(t, http.MethodGet, communeOAsPath, hostA, canBoGhi(xaA), "")
	if !strings.Contains(w.Body.String(), `"items":[]`) || !strings.Contains(w.Body.String(), `"configured":true`) {
		t.Errorf("no OA = %s", w.Body.String())
	}
}

// Identity down: names are display only — the read still answers, the code stands alone.
func TestCommuneOANamesDegradeToEmpty(t *testing.T) {
	s := newCommuneOAServer(t)
	s.grant(xaA, "admin.zalo")
	s.names.err = errors.New("identity down")
	w := s.call(t, http.MethodGet, communeOAsPath, hostA, canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"authorized_by_name":""`) || !strings.Contains(w.Body.String(), `"authorized_by":"CB-00099"`) {
		t.Errorf("reply = %s", w.Body.String())
	}
}

func TestCommuneOALinkIs201UncachedWithURL(t *testing.T) {
	s := newCommuneOAServer(t)
	s.grant(xaA, "admin.zalo")
	w := s.call(t, http.MethodPost, communeOAsPath+"/authorization-links", hostA, canBoGhi(xaA), "")
	doiMa(t, w, http.StatusCreated)
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q — the URL carries a one-time state", w.Header().Get("Cache-Control"))
	}
	wantKeys(t, "link", keysOf(t, w.Body.Bytes()), "url", "expires_at")
	if !strings.Contains(w.Body.String(), "FAKE-STATE-NOT-REAL") {
		t.Errorf("reply = %s", w.Body.String())
	}
}

func TestCommuneOACheckSyncDeleteShapesNameTheOA(t *testing.T) {
	s := newCommuneOAServer(t)
	s.grant(xaA, "admin.zalo")

	w := s.call(t, http.MethodPost, oaPath(oaRowA2)+"/check", hostA, canBoGhi(xaA), "")
	wantKeys(t, "check", keysOf(t, w.Body.Bytes()), "result", "checked_at", "oa_name")
	if s.fake.lastID != oaRowA2 {
		t.Errorf("check reached OA %q, want %q", s.fake.lastID, oaRowA2)
	}

	w = s.call(t, http.MethodPost, oaPath(oaRowA2)+"/templates/sync", hostA, canBoGhi(xaA), "")
	m := keysOf(t, w.Body.Bytes())
	wantKeys(t, "sync", m, "loaded", "enabled", "matching", "changed", "synced_at")
	if string(m["changed"]) != "2" || s.fake.lastID != oaRowA2 {
		t.Errorf("changed = %s, OA %q", m["changed"], s.fake.lastID)
	}

	w = s.call(t, http.MethodDelete, oaPath(oaRowA2), hostA, canBoGhi(xaA), disconnectBody)
	m = keysOf(t, w.Body.Bytes())
	wantKeys(t, "delete", m, "disconnected", "detached_template_count", "revoke_notice")
	if s.fake.lastReason != "Xã đổi sang OA mới" || s.fake.lastID != oaRowA2 || !strings.Contains(w.Body.String(), "OA Manager") ||
		string(m["disconnected"]) != "true" {
		t.Errorf("reason %q OA %q reply %s", s.fake.lastReason, s.fake.lastID, w.Body.String())
	}
}

func TestCommuneOATemplatesShape(t *testing.T) {
	s := newCommuneOAServer(t)
	s.grant(xaA, "admin.zalo")
	w := s.call(t, http.MethodGet, oaPath(oaRowA)+"/templates", hostA, canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	var out struct {
		Items    []map[string]json.RawMessage `json:"items"`
		SyncedAt *time.Time                   `json:"synced_at"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 2 || out.SyncedAt == nil || !out.SyncedAt.Equal(oaTestAt) || s.fake.lastID != oaRowA {
		t.Fatalf("reply = %s", w.Body.String())
	}
	keys := []string{"id", "oa_row_id", "template_id", "template_name", "zalo_status", "zalo_quality", "zalo_tag", "params",
		"params_match", "missing_params", "milestone", "milestone_set_at", "milestone_set_by", "milestone_set_by_name"}
	wantKeys(t, "attached", out.Items[0], keys...)
	wantKeys(t, "detached", out.Items[1], keys...)
	if string(out.Items[0]["milestone"]) != `"da-tiep-nhan"` || string(out.Items[0]["milestone_set_by_name"]) != `"Cán bộ CB-00099"` ||
		string(out.Items[0]["oa_row_id"]) != `"`+oaRowA+`"` {
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

	doiMa(t, s.call(t, http.MethodPatch, oaTemplatePath(tplA), hostA, canBoGhi(xaA), `{}`), http.StatusBadRequest)
	doiMa(t, s.call(t, http.MethodPatch, oaTemplatePath(tplA), hostA, canBoGhi(xaA), `{"milestone":"da-dong","x":1}`), http.StatusBadRequest)
	if s.fake.milePassed {
		t.Fatal("an absent or polluted body reached the use case")
	}

	w := s.call(t, http.MethodPatch, oaTemplatePath(tplA), hostA, canBoGhi(xaA), `{"milestone":null}`)
	doiMa(t, w, http.StatusOK)
	if !s.fake.milePassed || s.fake.lastMile != nil || s.fake.lastID != tplA {
		t.Fatalf("detach passed id %q milestone %v", s.fake.lastID, s.fake.lastMile)
	}
	w = s.call(t, http.MethodPatch, oaTemplatePath(tplA), hostA, canBoGhi(xaA), attachBody)
	if s.fake.lastMile == nil || *s.fake.lastMile != domain.MilestoneReceived || !strings.Contains(w.Body.String(), `"milestone":"da-tiep-nhan"`) {
		t.Fatalf("attach = %s", w.Body.String())
	}
}

// The one-OA routes are gone (ADR 0099 #10): nothing answers them any more.
func TestCommuneOACurrentRoutesAreGone(t *testing.T) {
	s := newCommuneOAServer(t)
	s.grant(xaA, "admin.zalo")
	for _, c := range []struct{ method, path string }{
		{http.MethodGet, communeOAsPath + "/current"},
		{http.MethodPost, communeOAsPath + "/current/authorization-links"},
		{http.MethodGet, communeOAsPath + "/current/templates"},
		{http.MethodPatch, communeOAsPath + "/current/templates/" + tplA},
	} {
		w := s.call(t, c.method, c.path, hostA, canBoGhi(xaA), attachBody)
		// GET zalo-oas/current/templates does match {id}/templates — with the id "current", which the use
		// case answers as an unknown OA. Every other old path matches no pattern at all.
		if w.Code == http.StatusOK || w.Code == http.StatusCreated {
			t.Errorf("%s %s still answers %d", c.method, c.path, w.Code)
		}
	}
}

func TestCommuneOAErrorMapping(t *testing.T) {
	check := oaPath(oaRowA) + "/check"
	sync := oaPath(oaRowA) + "/templates/sync"
	cases := []struct {
		name   string
		method string
		path   string
		body   string
		err    error
		status int
		code   string
	}{
		{"not configured", http.MethodPost, communeOAsPath + "/authorization-links", "", app.ErrZaloOANotConfigured, 503, "zalo_oa_not_configured"},
		{"no KEK", http.MethodPost, communeOAsPath + "/authorization-links", "", crypto.ErrNotConfigured, 503, "encryption_not_configured"},
		{"no OA", http.MethodPost, check, "", app.ErrCommuneZaloOAMissing, 404, "not_found"},
		{"reauth", http.MethodPost, check, "", app.ErrZaloOANeedsReauthorization, 409, "zalo_oa_needs_reauthorization"},
		{"changed", http.MethodPost, check, "", fmt.Errorf("x: %w", app.ErrCommuneZaloOAChanged), 409, "zalo_oa_changed"},
		{"refresh down", http.MethodPost, check, "", &app.ZaloOACallError{Step: "refresh", Class: domain.ZaloCallUnavailable}, 502, "zalo_unavailable"},
		{"token rejected", http.MethodPost, sync, "", &app.ZaloOACallError{Step: "template/all", Class: domain.ZaloCallTokenRejected}, 422, "zalo_token_rejected"},
		{"zalo down", http.MethodPost, sync, "", &app.ZaloOACallError{Step: "template/all", Class: domain.ZaloCallUnavailable}, 502, "zalo_unavailable"},
		{"taken", http.MethodPatch, oaTemplatePath(tplA), attachBody, commsstore.ErrMilestoneTaken, 409, "milestone_taken"},
		{"not enabled", http.MethodPatch, oaTemplatePath(tplA), attachBody, domain.ErrZaloOATemplateNotEnabled, 422, "template_not_attachable"},
		{"params", http.MethodPatch, oaTemplatePath(tplA), attachBody, &domain.TemplateParamsMissingError{Missing: []string{"moc_nhan"}}, 422, "template_not_attachable"},
		{"no template", http.MethodPatch, oaTemplatePath(tplA), attachBody, app.ErrZaloOATemplateMissing, 404, "not_found"},
		{"bad milestone", http.MethodPatch, oaTemplatePath(tplA), attachBody, domain.ErrZaloOAMilestoneUnknown, 400, "invalid_request"},
		{"bad reason", http.MethodDelete, oaPath(oaRowA), disconnectBody, domain.ErrCommuneZaloOARetireReason, 400, "invalid_request"},
		{"broken", http.MethodPost, check, "", errors.New("boom: " + string(xaB)), 500, "internal"},
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
