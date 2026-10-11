package http

import (
	"context"
	"encoding/json"
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
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// The four Zalo GROUP routes (ADR 0092) on the staff harness. The fake records WHICH commune and WHICH
// actor reached it; the transactions and entries are internal/app's tests.
//
//	admin.lookup: 401 no session · 403 wrong key · 403 right key other commune · 200/201/204

const testGroupUnit = "01JBPHAN00000000000000000A"

type fakeZaloGroups struct {
	calls       int
	lastCommune tenant.ID
	lastActor   audit.Actor
	lastIssue   app.GroupPairingCodeInput
	lastPatch   app.ZaloGroupPatch
	lastID      string
	lastReason  string
	err         error
}

func (f *fakeZaloGroups) note(ctx context.Context, a audit.Actor) {
	f.calls++
	f.lastCommune, f.lastActor = tenant.MustFrom(ctx), a
}

func (f *fakeZaloGroups) IssueGroupPairingCode(ctx context.Context, in app.GroupPairingCodeInput, a audit.Actor) (
	app.GroupPairingCodeView, error) {
	f.note(ctx, a)
	f.lastIssue = in
	return app.GroupPairingCodeView{Code: "ABCD2345", ExpiresAt: time.Date(2026, 10, 11, 1, 10, 0, 0, time.UTC)}, f.err
}

func (f *fakeZaloGroups) Groups(ctx context.Context) ([]app.ZaloGroupView, error) {
	f.note(ctx, audit.Actor{})
	return []app.ZaloGroupView{
		{ID: "g1", Kinds: []string{"nhiem-vu.giao-moi"}, LinkedAt: time.Date(2026, 10, 11, 1, 0, 0, 0, time.UTC),
			LinkedBy: "CB-00777", LinkedByName: "Nguyễn Văn A"},
		{ID: "g2", OrgUnitID: testGroupUnit, OrgUnitName: "Văn phòng", LinkedBy: "CB-00777"},
	}, f.err
}

func (f *fakeZaloGroups) EditGroup(ctx context.Context, id string, p app.ZaloGroupPatch, a audit.Actor) error {
	f.note(ctx, a)
	f.lastID, f.lastPatch = id, p
	return f.err
}

func (f *fakeZaloGroups) UnlinkGroup(ctx context.Context, id, reason string, a audit.Actor) error {
	f.note(ctx, a)
	f.lastID, f.lastReason = id, reason
	return f.err
}

type zaloGroupServer struct {
	h       http.Handler
	fake    *fakeZaloGroups
	checker *checkerDanhMucGia
	keys    int
}

func newZaloGroupServer(t *testing.T) *zaloGroupServer {
	t.Helper()
	fake := &fakeZaloGroups{}
	checker := &checkerDanhMucGia{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	RegisterZaloGroups(mux, ZaloGroupDeps{Checker: checker, Groups: fake, Log: log})
	var h http.Handler = mux
	h = chuTheGhi(h)
	h = idem.Middleware(moKhoIdemGia(), log)(h)
	h = httpx.TenantMiddleware(thuMucMau())(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)
	return &zaloGroupServer{h: h, fake: fake, checker: checker}
}

func (s *zaloGroupServer) grant(commune tenant.ID, perms ...authz.Perm) {
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

func (s *zaloGroupServer) call(t *testing.T, method, path, host string, p *authz.Principal, body string) *httptest.ResponseRecorder {
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
	r.Header.Set(idem.Header, fmt.Sprintf("01JIDEMZALOGROUPS%09d", s.keys))
	if p != nil {
		r = r.WithContext(context.WithValue(r.Context(), khoaChuTheGhi{}, *p))
	}
	w := httptest.NewRecorder()
	s.h.ServeHTTP(w, r)
	return w
}

const (
	groupCodeBody   = `{"org_unit_id":null,"kinds":["nhiem-vu.giao-moi"]}`
	groupPatchBody  = `{"kinds":["nhiem-vu.giao-moi"]}`
	groupUnlinkBody = `{"reason":"Nhóm giải tán"}`
)

// zaloGroupRoutes — the wrong keys are real and adjacent: a content reader, and asset.update.
func zaloGroupRoutes() []mapAssetRoute {
	return []mapAssetRoute{
		{"issue code", http.MethodPost, "/api/v1/zalo-group-pairing-codes", groupCodeBody, "admin.lookup", "content.read", http.StatusCreated},
		{"list", http.MethodGet, "/api/v1/zalo-groups", "", "admin.lookup", "content.read", http.StatusOK},
		{"edit", http.MethodPatch, "/api/v1/zalo-groups/g1", groupPatchBody, "admin.lookup", "asset.update", http.StatusNoContent},
		{"unlink", http.MethodDelete, "/api/v1/zalo-groups/g1", groupUnlinkBody, "admin.lookup", "asset.update", http.StatusNoContent},
	}
}

func TestZaloGroupRoutesAskForSeededKey(t *testing.T) {
	for _, tc := range zaloGroupRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newZaloGroupServer(t)
			s.call(t, tc.method, tc.path, hostA, canBoGhi(xaA), tc.body)
			if len(s.checker.hoiGi) == 0 || s.checker.hoiGi[0] != tc.perm {
				t.Fatalf("asked for %v, want %q", s.checker.hoiGi, tc.perm)
			}
		})
	}
}

func TestZaloGroupRoutes_401NoSession(t *testing.T) {
	for _, tc := range zaloGroupRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newZaloGroupServer(t)
			s.grant(xaA, tc.perm)
			doiMa(t, s.call(t, tc.method, tc.path, hostA, nil, tc.body), http.StatusUnauthorized)
			if s.fake.calls != 0 {
				t.Error("no session and the use case ran")
			}
		})
	}
}

func TestZaloGroupRoutes_403WrongPermission(t *testing.T) {
	for _, tc := range zaloGroupRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newZaloGroupServer(t)
			s.grant(xaA, tc.wrongPerm)
			doiMa(t, s.call(t, tc.method, tc.path, hostA, canBoGhi(xaA), tc.body), http.StatusForbidden)
			if s.fake.calls != 0 {
				t.Error("wrong permission and the use case ran")
			}
		})
	}
}

func TestZaloGroupRoutes_403RightPermissionWrongCommune(t *testing.T) {
	for _, tc := range zaloGroupRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newZaloGroupServer(t)
			s.grant(xaA, tc.perm)
			doiMa(t, s.call(t, tc.method, tc.path, hostB, canBoGhi(xaB), tc.body), http.StatusForbidden)
			if s.fake.calls != 0 {
				t.Error("a grant in another commune was enough")
			}
		})
	}
}

func TestZaloGroupRoutes_200RightPermissionRightCommune(t *testing.T) {
	for _, tc := range zaloGroupRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newZaloGroupServer(t)
			s.grant(xaA, tc.perm)
			doiMa(t, s.call(t, tc.method, tc.path, hostA, canBoGhi(xaA), tc.body), tc.ok)
			if s.fake.lastCommune != xaA {
				t.Errorf("ran in %q, want %q", s.fake.lastCommune, xaA)
			}
			if tc.method != http.MethodGet && s.fake.lastActor.ID != maCanBoGhi {
				t.Errorf("acted as %q, want the session's business code", s.fake.lastActor.ID)
			}
		})
	}
}

func TestZaloGroupShapes(t *testing.T) {
	s := newZaloGroupServer(t)
	s.grant(xaA, "admin.lookup")
	w := s.call(t, http.MethodPost, "/api/v1/zalo-group-pairing-codes", hostA, canBoGhi(xaA),
		`{"org_unit_id":"`+testGroupUnit+`","kinds":["nhiem-vu.giao-moi"]}`)
	doiMa(t, w, http.StatusCreated)
	if w.Header().Get("Cache-Control") != "no-store" || !strings.Contains(w.Body.String(), `"code":"ABCD2345"`) {
		t.Errorf("issue: %s %q", w.Body.String(), w.Header().Get("Cache-Control"))
	}
	if s.fake.lastIssue.OrgUnitID != testGroupUnit {
		t.Errorf("unit = %q", s.fake.lastIssue.OrgUnitID)
	}
	w = s.call(t, http.MethodGet, "/api/v1/zalo-groups", hostA, canBoGhi(xaA), "")
	var list struct {
		Items          []map[string]any `json:"items"`
		SupportedKinds []string         `json:"supported_kinds"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 2 || list.Items[0]["org_unit_id"] != nil || list.Items[1]["org_unit_id"] != testGroupUnit ||
		len(list.SupportedKinds) != len(domain.ZaloGroupKinds) {
		t.Fatalf("list = %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "chat_id") {
		t.Error("a chat_id field left the API")
	}
	for _, k := range []string{"id", "org_unit_name", "org_unit_removed", "kinds", "linked_at", "linked_by", "linked_by_name"} {
		if _, ok := list.Items[0][k]; !ok {
			t.Errorf("item lacks %q", k)
		}
	}
}

func TestZaloGroupPatchReadsNullAndAbsent(t *testing.T) {
	s := newZaloGroupServer(t)
	s.grant(xaA, "admin.lookup")
	patch := func(body string) *httptest.ResponseRecorder {
		return s.call(t, http.MethodPatch, "/api/v1/zalo-groups/g1", hostA, canBoGhi(xaA), body)
	}
	doiMa(t, patch(`{"org_unit_id":null}`), http.StatusNoContent)
	if !s.fake.lastPatch.OrgUnitSet || s.fake.lastPatch.OrgUnitID != "" || s.fake.lastPatch.Kinds != nil {
		t.Errorf("null → %+v, want commune-wide, kinds kept", s.fake.lastPatch)
	}
	doiMa(t, patch(groupPatchBody), http.StatusNoContent)
	if s.fake.lastPatch.OrgUnitSet || len(s.fake.lastPatch.Kinds) != 1 || s.fake.lastID != "g1" {
		t.Errorf("absent unit → %+v", s.fake.lastPatch)
	}
	doiMa(t, patch(`{"org_unit_id":""}`), http.StatusBadRequest)
	doiMa(t, patch(`{"org_unit_id":5}`), http.StatusBadRequest)
}

func TestZaloGroupErrorMapping(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{domain.ErrZaloGroupKindNotEnabled, http.StatusUnprocessableEntity, "kind_not_enabled"},
		{domain.ErrZaloGroupOrgUnitNotLive, http.StatusUnprocessableEntity, "org_unit_not_live"},
		{fmt.Errorf("x: %w", commsstore.ErrZaloGroupNotFound), http.StatusNotFound, "not_found"},
		{app.ErrZaloBotNotConfigured, http.StatusConflict, "zalo_bot_not_configured"},
		{app.ErrZaloChannelOff, http.StatusConflict, "zalo_channel_disabled"},
		{fmt.Errorf("%w: down", app.ErrOrgUnitsUnavailable), http.StatusServiceUnavailable, "org_units_unavailable"},
		{fmt.Errorf("boom"), http.StatusInternalServerError, "internal"},
	}
	for _, c := range cases {
		s := newZaloGroupServer(t)
		s.grant(xaA, "admin.lookup")
		s.fake.err = c.err
		w := s.call(t, http.MethodPatch, "/api/v1/zalo-groups/g1", hostA, canBoGhi(xaA), groupPatchBody)
		doiMa(t, w, c.status)
		if e := loiTra(t, w); e.Code != c.code || strings.Contains(e.Message, "boom") {
			t.Errorf("%v → %+v", c.err, e)
		}
	}
}

func TestRegisterZaloGroupsRefusesIncompleteWiring(t *testing.T) {
	for name, d := range map[string]ZaloGroupDeps{
		"checker": {Groups: &fakeZaloGroups{}},
		"groups":  {Checker: &checkerDanhMucGia{}},
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("incomplete wiring accepted")
				}
			}()
			RegisterZaloGroups(http.NewServeMux(), d)
		})
	}
}
