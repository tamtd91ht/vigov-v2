package http

import (
	"context"
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
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// The six portal sync routes. Harness pieces are the catalogue write suite's (checkerDanhMucGia,
// chuTheGhi, canBoGhi), because "right permission, wrong commune" needs a checker keyed by commune.

// testPortalKey is a fixture, not a credential: no portal accepts it.
const testPortalKey = "fixture-portal-key-9KQ2"

// fakePortalSync stands in for app.PortalSyncAdmin, RECORDING THE COMMUNE it was called in and what it
// received.
type fakePortalSync struct {
	view app.PortalSyncSettingsView
	tree app.PortalCategoryTree
	runs page.Result[domain.PortalSyncRun]
	run  domain.PortalSyncRun
	cats []domain.PortalCategory
	err  error

	calls       int
	lastCommune tenant.ID
	lastActor   audit.Actor
	lastSave    app.SavePortalSyncSettingsRequest
	lastKey     string // copied before the handler clears the secret
	lastSel     []domain.PortalCategorySelection
}

func (f *fakePortalSync) note(ctx context.Context) {
	f.calls++
	f.lastCommune = tenant.MustFrom(ctx)
}

func (f *fakePortalSync) Settings(ctx context.Context) (app.PortalSyncSettingsView, error) {
	f.note(ctx)
	return f.view, f.err
}

func (f *fakePortalSync) CategoryTree(ctx context.Context) (app.PortalCategoryTree, error) {
	f.note(ctx)
	return f.tree, f.err
}

func (f *fakePortalSync) Runs(ctx context.Context, _ page.Request) (page.Result[domain.PortalSyncRun], error) {
	f.note(ctx)
	return f.runs, f.err
}

func (f *fakePortalSync) SaveSettings(ctx context.Context, req app.SavePortalSyncSettingsRequest,
	actor audit.Actor) (app.PortalSyncSettingsView, error) {
	f.note(ctx)
	f.lastSave, f.lastActor, f.lastKey = req, actor, string(req.APIKey.Lo())
	return f.view, f.err
}

func (f *fakePortalSync) SaveCategories(ctx context.Context, sel []domain.PortalCategorySelection,
	actor audit.Actor) ([]domain.PortalCategory, error) {
	f.note(ctx)
	f.lastSel, f.lastActor = sel, actor
	return f.cats, f.err
}

func (f *fakePortalSync) StartRun(ctx context.Context, actor audit.Actor) (domain.PortalSyncRun, error) {
	f.note(ctx)
	f.lastActor = actor
	return f.run, f.err
}

type portalServer struct {
	h       http.Handler
	fake    *fakePortalSync
	checker *checkerDanhMucGia
}

func newPortalServer(t *testing.T) *portalServer {
	t.Helper()
	started := time.Date(2026, 10, 1, 3, 0, 0, 0, time.UTC)
	fake := &fakePortalSync{
		view: app.PortalSyncSettingsView{Settings: domain.PortalSyncSettings{
			Provider: domain.PortalProviderCityShared, APIURL: "https://portal.example.gov.vn/api",
			PublishMode: domain.PortalPublishReview, IntervalHours: 6, WindowDays: 90, MaxItemsPerRun: 100,
			KeepSourceCredit: true, IsEnabled: true, APIKeySet: true, UpdatedBy: maCanBoGhi,
		}, Configured: true, EncryptionConfigured: true},
		tree: app.PortalCategoryTree{Items: []app.PortalCategoryNode{
			{ExternalID: "120", Name: "Tin tức", ParentName: "Tin", IsSelected: true, TargetKind: "tin-tuc"}}},
		runs: page.Result[domain.PortalSyncRun]{Items: []domain.PortalSyncRun{{ID: "01JRUN", TriggerKind: "theo-lich",
			Actor: "system", StartedAt: started, FinishedAt: started.Add(time.Minute), Outcome: "mot-phan",
			Counts: domain.PortalRunCounts{Fetched: 9, Imported: 3, SkippedExisting: 2, SkippedDeleted: 1, Failed: 1},
			Errors: []domain.PortalRunError{{CategoryExternalID: "144", CategoryName: "Chuyển đổi số", Error: "http-503", Count: 1}}}}},
		run:  domain.PortalSyncRun{ID: "01JRUNMANUAL", TriggerKind: "chay-tay", Actor: maCanBoGhi, StartedAt: started},
		cats: []domain.PortalCategory{{ID: "01JCAT", ExternalID: "120", Name: "Tin tức", TargetKind: "tin-tuc", IsSelected: true}},
	}
	checker := &checkerDanhMucGia{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	Register(mux, Deps{
		Checker:              checker,
		LoaiTaiNguyen:        danhMucMau(),
		GhiLoaiTaiNguyen:     &ghiDanhMucGia{},
		ThongBao:             &soThongBaoGia{},
		GhiThongBao:          &ghiThongBaoGia{},
		NoiDung:              &soNoiDungGia{},
		GhiNoiDung:           &ghiNoiDungGia{},
		DanhMucNoiDung:       &soDanhMucNDGia{},
		GhiDanhMucNoiDung:    &ghiDanhMucNDGia{},
		ContentCovers:        &fakeCovers{},
		ContentAudio:         &fakeAudio{},
		MapFieldSchemas:      &fakeMapFieldSchemas{},
		WriteMapFieldSchemas: &fakeMapFieldSchemas{},
		MailSettings:         &fakeMailSettings{},
		WriteMailSettings:    &fakeMailSettings{},
		AuditLog:             &auditLogFake{},
		StaffInbox:           &fakeInbox{},
		WriteStaffInbox:      &fakeInbox{},
		PortalSync:           fake,
		WritePortalSync:      fake,
		Log:                  log,
	})
	var h http.Handler = mux
	h = chuTheGhi(h)
	h = idem.Middleware(nil, log)(h)
	h = httpx.TenantMiddleware(thuMucMau())(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)
	return &portalServer{h: h, fake: fake, checker: checker}
}

func (s *portalServer) grant(commune tenant.ID, perms ...authz.Perm) {
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

func (s *portalServer) call(t *testing.T, method, host, path string, p *authz.Principal, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	r := httptest.NewRequest(method, "https://"+host+path, rd)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set(idem.Header, "01JIDEMPOTENCYKEYPORTALSY")
	if p != nil {
		r = r.WithContext(context.WithValue(r.Context(), khoaChuTheGhi{}, *p))
	}
	w := httptest.NewRecorder()
	s.h.ServeHTTP(w, r)
	return w
}

const (
	portalSettingsPath   = "/api/v1/portal-sync/settings"
	portalCategoriesPath = "/api/v1/portal-sync/categories"
	portalRunsPath       = "/api/v1/portal-sync/runs"
)

var putPortalBody = `{"api_url":"https://portal.example.gov.vn/api","api_key":"` + testPortalKey +
	`","publish_mode":"cho-duyet","interval_hours":6,"window_days":90,"max_items_per_run":100,"keep_source_credit":true,"is_enabled":true}`

const putCategoriesBody = `{"categories":[{"external_id":"120","name":"Tin tức","target_kind":"tin-tuc","is_selected":true}]}`

type portalRoute struct {
	name, method, path, body string
	key, wrongKey            authz.Perm
	ok                       int
}

// sixRoutes — every permission case runs on every route. The wrong key is real and adjacent: a reader
// of the content register (`content.read`) must not change the sync, and an administrator of
// catalogues (`admin.lookup`) must not read it.
func sixRoutes() []portalRoute {
	return []portalRoute{
		{"GET settings", http.MethodGet, portalSettingsPath, "", "content.read", "admin.lookup", http.StatusOK},
		{"PUT settings", http.MethodPut, portalSettingsPath, putPortalBody, "content.update", "content.read", http.StatusOK},
		{"GET categories", http.MethodGet, portalCategoriesPath, "", "content.read", "admin.lookup", http.StatusOK},
		{"PUT categories", http.MethodPut, portalCategoriesPath, putCategoriesBody, "content.update", "content.read", http.StatusOK},
		{"GET runs", http.MethodGet, portalRunsPath, "", "content.read", "admin.lookup", http.StatusOK},
		{"POST runs", http.MethodPost, portalRunsPath, "", "content.update", "content.read", http.StatusAccepted},
	}
}

func TestPortalRoutesAskForSeededKey(t *testing.T) {
	// Compared with a LITERAL: a fake checker grants any string (rule 5, invariant 3c).
	for _, tc := range sixRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newPortalServer(t)
			s.call(t, tc.method, hostA, tc.path, canBoGhi(xaA), tc.body)
			if got := s.checker.hoiKhoaCuoi(); got != tc.key {
				t.Fatalf("route asked for %q, want %q", got, tc.key)
			}
		})
	}
}

func TestPortalRoutes_401NoSession(t *testing.T) {
	for _, tc := range sixRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newPortalServer(t)
			s.grant(xaA, tc.key)
			doiMa(t, s.call(t, tc.method, hostA, tc.path, nil, tc.body), http.StatusUnauthorized)
			if s.fake.calls != 0 {
				t.Error("no session and the use case still ran")
			}
		})
	}
}

func TestPortalRoutes_403WrongPermission(t *testing.T) {
	for _, tc := range sixRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newPortalServer(t)
			s.grant(xaA, tc.wrongKey)
			doiMa(t, s.call(t, tc.method, hostA, tc.path, canBoGhi(xaA), tc.body), http.StatusForbidden)
			if s.fake.calls != 0 {
				t.Error("wrong permission and the use case still ran")
			}
		})
	}
}

func TestPortalRoutes_403RightPermissionWrongCommune(t *testing.T) {
	// Signed in at B as a member of B; the grant is in A. A checker ignoring the commune would let A's
	// editor point B's sync at another portal — or read B's key status.
	for _, tc := range sixRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newPortalServer(t)
			s.grant(xaA, tc.key)
			doiMa(t, s.call(t, tc.method, hostB, tc.path, canBoGhi(xaB), tc.body), http.StatusForbidden)
			if s.fake.calls != 0 {
				t.Error("a grant in another commune was enough")
			}
		})
	}
}

func TestPortalRoutes_401SessionOfAnotherCommune(t *testing.T) {
	for _, tc := range sixRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newPortalServer(t)
			s.grant(xaA, tc.key)
			s.grant(xaB, tc.key)
			doiMa(t, s.call(t, tc.method, hostB, tc.path, canBoGhi(xaA), tc.body), http.StatusUnauthorized)
			if s.fake.calls != 0 {
				t.Error("a session of another commune still reached the use case")
			}
		})
	}
}

func TestPortalRoutes_RightPermissionRightCommune(t *testing.T) {
	for _, tc := range sixRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newPortalServer(t)
			s.grant(xaA, tc.key)
			doiMa(t, s.call(t, tc.method, hostA, tc.path, canBoGhi(xaA), tc.body), tc.ok)
			if s.fake.calls != 1 || s.fake.lastCommune != xaA {
				t.Fatalf("use case ran %d times in commune %q, want once in %q", s.fake.calls, s.fake.lastCommune, xaA)
			}
			if tc.method == http.MethodGet {
				return
			}
			// Rule 6, invariant 8: the BUSINESS CODE, never the internal id.
			if s.fake.lastActor.ID != maCanBoGhi || s.fake.lastActor.IP != "10.0.0.7" {
				t.Errorf("actor = %+v, want business code %q from the socket address", s.fake.lastActor, maCanBoGhi)
			}
		})
	}
}

func TestGetPortalSettingsNeverCarriesTheKey(t *testing.T) {
	s := newPortalServer(t)
	s.grant(xaA, "content.read")
	w := s.call(t, http.MethodGet, hostA, portalSettingsPath, canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	body := w.Body.String()
	if !strings.Contains(body, `"api_key_set":true`) {
		t.Errorf("api_key_set missing: %s", body)
	}
	if strings.Contains(body, `"api_key":`) || strings.Contains(body, testPortalKey) {
		t.Fatalf("GET carries a key field: %s", body)
	}
}

func TestPutPortalSettingsPassesKeyAsSecretAndDoesNotEchoIt(t *testing.T) {
	s := newPortalServer(t)
	s.grant(xaA, "content.update")
	w := s.call(t, http.MethodPut, hostA, portalSettingsPath, canBoGhi(xaA), putPortalBody)
	doiMa(t, w, http.StatusOK)
	if s.fake.lastKey != testPortalKey {
		t.Error("use case did not receive the typed key")
	}
	in := s.fake.lastSave.Input
	if in.APIURL != "https://portal.example.gov.vn/api" || in.IntervalHours == nil || *in.IntervalHours != 6 {
		t.Errorf("input = %+v", in)
	}
	if strings.Contains(fmt.Sprintf("%v %+v", s.fake.lastSave, s.fake.lastSave), testPortalKey) {
		t.Error("the request formats with the key in it")
	}
	if strings.Contains(w.Body.String(), testPortalKey) {
		t.Error("PUT echoes the key")
	}
}

func TestPortalErrorsMapToStatuses(t *testing.T) {
	for name, tc := range map[string]struct {
		method, path, body string
		err                error
		code               int
		key                string
	}{
		"url change, no key": {http.MethodPut, portalSettingsPath, putPortalBody, app.ErrPortalKeyRequiredForNewURL, http.StatusUnprocessableEntity, "api_key_required_for_new_url"},
		"first save, no key": {http.MethodPut, portalSettingsPath, putPortalBody, app.ErrPortalKeyRequired, http.StatusUnprocessableEntity, "api_key_required"},
		"no KEK":             {http.MethodPut, portalSettingsPath, putPortalBody, crypto.ErrNotConfigured, http.StatusServiceUnavailable, "encryption_not_configured"},
		"bad url":            {http.MethodPut, portalSettingsPath, putPortalBody, domain.ErrPortalAPIURLInvalid, http.StatusBadRequest, "invalid_request"},
		"bad kind":           {http.MethodPut, portalCategoriesPath, putCategoriesBody, domain.ErrPortalSelectionKind, http.StatusBadRequest, "invalid_request"},
		"not configured":     {http.MethodGet, portalCategoriesPath, "", commsstore.ErrPortalSyncSettingsNotFound, http.StatusConflict, "portal_sync_not_configured"},
		"portal timeout":     {http.MethodGet, portalCategoriesPath, "", &app.PortalCallError{Class: "timeout"}, http.StatusBadGateway, "portal_timeout"},
		"portal 401":         {http.MethodGet, portalCategoriesPath, "", &app.PortalCallError{Class: "http-401"}, http.StatusBadGateway, "portal_http_401"},
		"portal odd status":  {http.MethodGet, portalCategoriesPath, "", &app.PortalCallError{Class: "http-418"}, http.StatusBadGateway, "portal_http_418"},
		"run in progress":    {http.MethodPost, portalRunsPath, "", app.ErrPortalRunInProgress, http.StatusConflict, "portal_sync_in_progress"},
		"runner stopped":     {http.MethodPost, portalRunsPath, "", app.ErrPortalRunnerStopped, http.StatusServiceUnavailable, "portal_sync_unavailable"},
		"store broken":       {http.MethodGet, portalRunsPath, "", errors.New("cơ sở dữ liệu không phản hồi secret_code=x"), http.StatusInternalServerError, "internal"},
	} {
		t.Run(name, func(t *testing.T) {
			s := newPortalServer(t)
			s.grant(xaA, "content.read", "content.update")
			s.fake.err = tc.err
			w := s.call(t, tc.method, hostA, tc.path, canBoGhi(xaA), tc.body)
			doiMa(t, w, tc.code)
			e := loiTra(t, w)
			if e.Code != tc.key {
				t.Errorf("code = %q, want %q", e.Code, tc.key)
			}
			if strings.Contains(e.Message, "secret_code") || strings.Contains(e.Message, "không phản hồi") ||
				strings.Contains(e.Message, string(xaA)) {
				t.Errorf("message leaks the underlying error: %q", e.Message)
			}
		})
	}
}

func TestPortalRefusalsAnswerFixedSentences(t *testing.T) {
	for _, row := range portalSyncRefusals {
		t.Run(row.err.Error(), func(t *testing.T) {
			s := newPortalServer(t)
			s.grant(xaA, "content.update")
			s.fake.err = fmt.Errorf("đồng bộ: lưu cho xã %s: %w", xaA, row.err)
			w := s.call(t, http.MethodPut, hostA, portalSettingsPath, canBoGhi(xaA), putPortalBody)
			doiMa(t, w, http.StatusBadRequest)
			if e := loiTra(t, w); e.Message != row.message {
				t.Fatalf("got %q, want %q", e.Message, row.message)
			}
		})
	}
}

func TestPortalRunsListShape(t *testing.T) {
	s := newPortalServer(t)
	s.grant(xaA, "content.read")
	w := s.call(t, http.MethodGet, hostA, portalRunsPath, canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	body := w.Body.String()
	for _, want := range []string{`"outcome":"mot-phan"`, `"imported_count":3`, `"skipped_deleted_count":1`,
		`"error":"http-503"`, `"category_external_id":"144"`, `"has_more":false`} {
		if !strings.Contains(body, want) {
			t.Errorf("runs body lacks %s: %s", want, body)
		}
	}
}

func TestStartPortalRunNeedsIdempotencyKey(t *testing.T) {
	s := newPortalServer(t)
	s.grant(xaA, "content.update")
	r := httptest.NewRequest(http.MethodPost, "https://"+hostA+portalRunsPath, nil)
	r.Host = hostA
	r.RemoteAddr = "10.0.0.7:51000"
	r = r.WithContext(context.WithValue(r.Context(), khoaChuTheGhi{}, *canBoGhi(xaA)))
	w := httptest.NewRecorder()
	s.h.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest || s.fake.calls != 0 {
		t.Fatalf("no idempotency key: status %d, calls %d", w.Code, s.fake.calls)
	}
}
