package http

import (
	"context"
	"encoding/json"
	"errors"
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
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-reporting/internal/app"
	"github.com/vihat/vigov/service-reporting/internal/domain"
)

// "Lời hệ thống", reporting half — the rule 5 invariant 7 set on all three routes, plus what the
// handler owns: which key and text reach the use case, who the actor is, and how each refusal is
// answered. The fallback rule and the transaction are app/system_message_test.go's.
//
// THE GUARD IS REAL (authz.RequirePermission). The principal is injected directly in place of
// core/staffauth — that middleware is driven end to end in cmd/server/main_test.go.

const (
	hostA = "xa-a.example.gov.vn"
	hostB = "xa-b.example.gov.vn"

	// The internal staff id and the business code of the same person, kept SEPARATE: authorisation
	// joins on the id, the trail records the code (rule 6, invariant 8). One value for both is a test
	// that cannot tell them apart.
	staffInternalID = "nd-01JINTERNALIDCUACANBO"
	staffCode       = "CB-00123"

	systemMessagesPath = "/api/v1/reporting-system-messages"
	titleKey           = "report.title"
)

var (
	communeA = tenant.ID("01JA" + strings.Repeat("A", 22))
	communeB = tenant.ID("01JB" + strings.Repeat("B", 22))
)

func overridePath(key string) string { return systemMessagesPath + "/" + key + "/override" }

func staffOf(c tenant.ID) *authz.Principal {
	return &authz.Principal{ID: staffInternalID, Ma: staffCode, Kind: "staff", TenantID: c}
}

// staffWithoutCode is a principal from an identity older than the `ma` field.
func staffWithoutCode(c tenant.ID) *authz.Principal {
	return &authz.Principal{ID: staffInternalID, Kind: "staff", TenantID: c}
}

type directoryFake map[string]tenant.Tenant

func (m directoryFake) ByHost(_ context.Context, host string) (tenant.Tenant, bool) {
	t, ok := m[host]
	return t, ok
}

type principalKey struct{}

// injectPrincipal puts the principal into the context in place of the real authentication edge, and
// deliberately does NOT copy the Host's commune onto it — that would make every request
// self-consistent and the wrong-commune case untestable.
func injectPrincipal(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := r.Context().Value(principalKey{}).(authz.Principal)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r.WithContext(authz.Into(r.Context(), p)))
	})
}

// checkerFake keys its grants BY COMMUNE, read from the CONTEXT — never from the principal, which
// would answer about the commune the token claims rather than the one the request arrived at.
type checkerFake struct {
	grants map[tenant.ID]map[authz.Perm]struct{}
	asked  []authz.Perm
}

func (c *checkerFake) Allows(ctx context.Context, p authz.Principal, perm authz.Perm) bool {
	c.asked = append(c.asked, perm)
	if p.TenantID != tenant.MustFrom(ctx) {
		return false
	}
	_, ok := c.grants[tenant.MustFrom(ctx)][perm]
	return ok
}

func (c *checkerFake) lastAsked() authz.Perm {
	if len(c.asked) == 0 {
		return ""
	}
	return c.asked[len(c.asked)-1]
}

// systemMessagesFake records the commune read from the context, as *store.Scoped reads it.
type systemMessagesFake struct {
	calls   int
	commune tenant.ID
	actor   audit.Actor
	key     string
	text    string
	op      string
	err     error
	list    []domain.SystemMessage
	result  domain.SystemMessage
	active  *bool
}

func (f *systemMessagesFake) record(ctx context.Context, op string) {
	f.calls++
	f.op = op
	f.commune = tenant.MustFrom(ctx)
}

func (f *systemMessagesFake) Messages(ctx context.Context) ([]domain.SystemMessage, error) {
	f.record(ctx, "list")
	return f.list, f.err
}

func (f *systemMessagesFake) Reword(ctx context.Context, key, text string, actor audit.Actor) (domain.SystemMessage, error) {
	f.record(ctx, "reword")
	f.key, f.text, f.actor = key, text, actor
	return f.result, f.err
}

func (f *systemMessagesFake) Restore(ctx context.Context, key string, actor audit.Actor) (domain.SystemMessage, error) {
	f.record(ctx, "restore")
	f.key, f.actor = key, actor
	return f.result, f.err
}

func (f *systemMessagesFake) SetActive(ctx context.Context, key string, active bool, actor audit.Actor) (domain.SystemMessage, error) {
	f.record(ctx, "switch")
	f.key, f.actor, f.active = key, actor, &active
	return f.result, f.err
}

type harness struct {
	h       http.Handler
	svc     *systemMessagesFake
	checker *checkerFake
}

func newHarnessWith(t *testing.T, svc SystemMessageService) (http.Handler, *checkerFake) {
	t.Helper()
	checker := &checkerFake{}
	lg := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	Register(mux, Deps{Checker: checker, SystemMessages: svc, Log: lg})
	var h http.Handler = mux
	h = injectPrincipal(h)
	h = idem.Middleware(nil, lg)(h)
	h = httpx.TenantMiddleware(directoryFake{
		hostA: {ID: communeA, Host: hostA, Active: true},
		hostB: {ID: communeB, Host: hostB, Active: true},
	})(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)
	return h, checker
}

func newHarness(t *testing.T) *harness {
	svc := &systemMessagesFake{}
	h, checker := newHarnessWith(t, svc)
	return &harness{h: h, svc: svc, checker: checker}
}

func (c *checkerFake) grant(commune tenant.ID, perm ...authz.Perm) {
	if c.grants == nil {
		c.grants = map[tenant.ID]map[authz.Perm]struct{}{}
	}
	if c.grants[commune] == nil {
		c.grants[commune] = map[authz.Perm]struct{}{}
	}
	for _, p := range perm {
		c.grants[commune][p] = struct{}{}
	}
}

func call(h http.Handler, method, host, path string, p *authz.Principal, body string) *httptest.ResponseRecorder {
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	r := httptest.NewRequest(method, "https://"+host+path, rd)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	r.Header.Set("Content-Type", "application/json")
	if p != nil {
		r = r.WithContext(context.WithValue(r.Context(), principalKey{}, *p))
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func wantStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("status = %d, want %d — body: %s", w.Code, want, w.Body.String())
	}
}

func errorBody(t *testing.T, w *httptest.ResponseRecorder) httpx.Error {
	t.Helper()
	var e httpx.Error
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatalf("error body is not JSON: %q", w.Body.String())
	}
	return e
}

type systemMessageRoute struct {
	name, method, path, body string
	ok                       int
}

func systemMessageRoutes() []systemMessageRoute {
	return []systemMessageRoute{
		{"GET", http.MethodGet, systemMessagesPath, "", http.StatusOK},
		{"PUT", http.MethodPut, overridePath(titleKey), `{"text":"Báo cáo của xã."}`, http.StatusOK},
		{"DELETE", http.MethodDelete, overridePath(titleKey), "", http.StatusNoContent},
		// ADR 0079 Q2 (migration 0004): "Tắt / Bật lại".
		{"PATCH (switch)", http.MethodPatch, overridePath(titleKey), `{"is_active":false}`, http.StatusOK},
	}
}

// --- rule 5, invariant 7 on all three routes ------------------------------------------------------

func TestSystemMessages_401WithoutSession(t *testing.T) {
	for _, rt := range systemMessageRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			s := newHarness(t)
			s.checker.grant(communeA, "admin.lookup")
			wantStatus(t, call(s.h, rt.method, hostA, rt.path, nil, rt.body), http.StatusUnauthorized)
			if s.svc.calls != 0 {
				t.Error("use case ran with no session")
			}
		})
	}
}

func TestSystemMessages_403WrongPermission(t *testing.T) {
	// `report.read` and `admin.audit` are real keys and deliberately not enough: reading a report,
	// or reading the trail, is not administering the commune's configuration.
	for _, rt := range systemMessageRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			s := newHarness(t)
			s.checker.grant(communeA, "report.read", "report.export", "admin.audit")
			wantStatus(t, call(s.h, rt.method, hostA, rt.path, staffOf(communeA), rt.body), http.StatusForbidden)
			if s.svc.calls != 0 {
				t.Error("use case ran without admin.lookup")
			}
			// The key the ROUTE asked for, against a literal: a fake checker grants any string, so
			// only this catches a key the `quyen` table lacks (rule 5, invariant 3c).
			if got := s.checker.lastAsked(); got != "admin.lookup" {
				t.Errorf("route asked for %q, want admin.lookup", got)
			}
		})
	}
}

func TestSystemMessages_403RightPermissionWrongCommune(t *testing.T) {
	// Granted in commune A; the account is commune B's, signed in at commune B.
	for _, rt := range systemMessageRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			s := newHarness(t)
			s.checker.grant(communeA, "admin.lookup")
			wantStatus(t, call(s.h, rt.method, hostB, rt.path, staffOf(communeB), rt.body), http.StatusForbidden)
			if s.svc.calls != 0 {
				t.Error("commune A's grant let commune B configure its sentences")
			}
		})
	}
}

func TestSystemMessages_TokenOfAnotherCommuneIsRefused(t *testing.T) {
	// Commune A's account presented at commune B's Host: authz's own commune comparison refuses it
	// before any permission is consulted (rule 1, invariant 8).
	for _, rt := range systemMessageRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			s := newHarness(t)
			s.checker.grant(communeA, "admin.lookup")
			s.checker.grant(communeB, "admin.lookup")
			w := call(s.h, rt.method, hostB, rt.path, staffOf(communeA), rt.body)
			if w.Code != http.StatusUnauthorized && w.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 401 or 403", w.Code)
			}
			if s.svc.calls != 0 {
				t.Error("a commune A account configured commune B")
			}
		})
	}
}

func TestSystemMessages_200RightPermissionRightCommune(t *testing.T) {
	for _, rt := range systemMessageRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			s := newHarness(t)
			s.checker.grant(communeA, "admin.lookup")
			wantStatus(t, call(s.h, rt.method, hostA, rt.path, staffOf(communeA), rt.body), rt.ok)
			if s.svc.calls != 1 || s.svc.commune != communeA {
				t.Fatalf("calls=%d commune=%q, want one call in the Host's commune", s.svc.calls, s.svc.commune)
			}
			if rt.method != http.MethodGet {
				// Rule 6, invariant 8: the trail names the BUSINESS CODE, never the internal id.
				if s.svc.actor.ID != staffCode || s.svc.actor.IP != "10.0.0.7" || s.svc.actor.Kind != "staff" {
					t.Errorf("actor = %+v, want business code %s and socket IP", s.svc.actor, staffCode)
				}
				if s.svc.key != titleKey {
					t.Errorf("key = %q", s.svc.key)
				}
			}
		})
	}
}

// --- what each route passes on and answers -------------------------------------------------------

func TestListSystemMessagesAll38WithDefaultsThroughTheRealUseCase(t *testing.T) {
	// The REAL use case over a store holding nothing — every commune today. What the web tab
	// receives is the whole catalogue, each key on its shipped default.
	h, checker := newHarnessWith(t, app.NewSystemMessages(pkgstore.New(nil), emptyOverrideStore{}))
	checker.grant(communeA, "admin.lookup")
	w := call(h, http.MethodGet, hostA, systemMessagesPath, staffOf(communeA), "")
	wantStatus(t, w, http.StatusOK)

	var out struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	shipped := domain.ShippedMessages()
	if len(out.Items) != 38 {
		t.Fatalf("listed %d keys, want 38", len(out.Items))
	}
	for i, it := range out.Items {
		m := shipped[i]
		if it["code"] != m.Key || it["default_text"] != m.DefaultText || it["current_text"] != m.DefaultText ||
			it["description"] != m.Description || it["overridden"] != false {
			t.Errorf("item %d = %v, want %s on its default", i, it, m.Key)
		}
		if _, ok := it["updated_by"]; ok {
			t.Errorf("%s names somebody though nobody changed it", m.Key)
		}
	}
}

func TestListSystemMessagesShape(t *testing.T) {
	s := newHarness(t)
	s.checker.grant(communeA, "admin.lookup")
	at := time.Date(2026, 9, 29, 3, 0, 0, 0, time.UTC)
	s.svc.list = []domain.SystemMessage{{
		Key: titleKey, Description: "d", DefaultText: "Báo cáo điều hành",
		CurrentText: "Báo cáo của xã.", Overridden: true, UpdatedAt: &at, UpdatedBy: "CB-00777",
	}}
	w := call(s.h, http.MethodGet, hostA, systemMessagesPath+"?tenant_id=01JOTHERCOMMUNE", staffOf(communeA), "")
	wantStatus(t, w, http.StatusOK)
	if s.svc.commune != communeA {
		t.Errorf("read in %q — the commune is the Host's, never a query parameter", s.svc.commune)
	}
	var out struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || len(out.Items) != 1 {
		t.Fatalf("body %s", w.Body.String())
	}
	for k, want := range map[string]any{
		"code": titleKey, "default_text": "Báo cáo điều hành", "current_text": "Báo cáo của xã.",
		"overridden": true, "updated_by": "CB-00777", "updated_at": "2026-09-29T03:00:00Z",
	} {
		if out.Items[0][k] != want {
			t.Errorf("%s = %v, want %v", k, out.Items[0][k], want)
		}
	}
}

func TestRewordSystemMessagePassesTextAndReturnsMessage(t *testing.T) {
	s := newHarness(t)
	s.checker.grant(communeA, "admin.lookup")
	s.svc.result = domain.SystemMessage{Key: titleKey, CurrentText: "Báo cáo của xã.", Overridden: true}
	w := call(s.h, http.MethodPut, hostA, overridePath(titleKey), staffOf(communeA), `{"text":"Báo cáo của xã."}`)
	wantStatus(t, w, http.StatusOK)
	if s.svc.text != "Báo cáo của xã." || s.svc.op != "reword" {
		t.Errorf("text=%q op=%q", s.svc.text, s.svc.op)
	}
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out["code"] != titleKey || out["current_text"] != "Báo cáo của xã." || out["overridden"] != true {
		t.Errorf("body %s", w.Body.String())
	}
}

func TestRestoreSystemMessageIs204AndCallsRestore(t *testing.T) {
	s := newHarness(t)
	s.checker.grant(communeA, "admin.lookup")
	w := call(s.h, http.MethodDelete, hostA, overridePath("report.metric.fiscal.balance"), staffOf(communeA), "")
	wantStatus(t, w, http.StatusNoContent)
	if s.svc.op != "restore" || s.svc.key != "report.metric.fiscal.balance" || w.Body.Len() != 0 {
		t.Errorf("op=%q key=%q body=%q", s.svc.op, s.svc.key, w.Body.String())
	}
}

func TestSystemMessageRefusalsMapped(t *testing.T) {
	for name, c := range map[string]struct {
		err  error
		want int
		code string
	}{
		"unknown key": {domain.ErrUnknownMessageKey, http.StatusNotFound, "not_found"},
		"empty text":  {domain.ErrMessageTextEmpty, http.StatusBadRequest, "invalid_request"},
		"markup":      {domain.ErrMessageTextMarkup, http.StatusBadRequest, "invalid_request"},
		"store down":  {errors.New("connection reset"), http.StatusInternalServerError, "internal"},
	} {
		t.Run(name, func(t *testing.T) {
			s := newHarness(t)
			s.checker.grant(communeA, "admin.lookup")
			s.svc.err = c.err
			w := call(s.h, http.MethodPut, hostA, overridePath(titleKey), staffOf(communeA), `{"text":""}`)
			wantStatus(t, w, c.want)
			e := errorBody(t, w)
			if e.Code != c.code || strings.Contains(e.Message, "connection reset") {
				t.Errorf("error = %+v", e)
			}
		})
	}
}

func TestEmptyTextNamesTheRestoreAction(t *testing.T) {
	s := newHarness(t)
	s.checker.grant(communeA, "admin.lookup")
	s.svc.err = domain.ErrMessageTextEmpty
	w := call(s.h, http.MethodPut, hostA, overridePath(titleKey), staffOf(communeA), `{"text":"  "}`)
	wantStatus(t, w, http.StatusBadRequest)
	if !strings.Contains(errorBody(t, w).Message, "Khôi phục câu mặc định") {
		t.Errorf("message %q does not point to the restore action", errorBody(t, w).Message)
	}
}

func TestUnknownKeyIs404OnBothWritesThroughTheRealUseCase(t *testing.T) {
	// The REAL use case: another service's key is refused before any transaction, so the nil
	// *store.DB behind it is never touched.
	for _, key := range []string{"budget.scope_notice", "feedback.reason_required", "zalo.header", "report.nope"} {
		for _, method := range []string{http.MethodPut, http.MethodDelete} {
			t.Run(method+" "+key, func(t *testing.T) {
				h, checker := newHarnessWith(t, app.NewSystemMessages(pkgstore.New(nil), emptyOverrideStore{}))
				checker.grant(communeA, "admin.lookup")
				body := ""
				if method == http.MethodPut {
					body = `{"text":"x"}`
				}
				w := call(h, method, hostA, overridePath(key), staffOf(communeA), body)
				wantStatus(t, w, http.StatusNotFound)
				if errorBody(t, w).Code != "not_found" {
					t.Errorf("body %s", w.Body.String())
				}
			})
		}
	}
}

func TestRewordBadJSONIs400AndCallsNothing(t *testing.T) {
	s := newHarness(t)
	s.checker.grant(communeA, "admin.lookup")
	wantStatus(t, call(s.h, http.MethodPut, hostA, overridePath(titleKey), staffOf(communeA), `{"text":`),
		http.StatusBadRequest)
	if s.svc.calls != 0 {
		t.Error("malformed body reached the use case")
	}
}

func TestSystemMessageWriteWithoutBusinessCodeIs500(t *testing.T) {
	// No `Ma`: the trail could not name who acted, so nothing is written.
	for _, rt := range systemMessageRoutes()[1:] {
		t.Run(rt.name, func(t *testing.T) {
			s := newHarness(t)
			s.checker.grant(communeA, "admin.lookup")
			wantStatus(t, call(s.h, rt.method, hostA, rt.path, staffWithoutCode(communeA), rt.body),
				http.StatusInternalServerError)
			if s.svc.calls != 0 {
				t.Error("write ran for a principal with no business code")
			}
		})
	}
}

func TestRegisterRefusesIncompleteWiring(t *testing.T) {
	for name, d := range map[string]Deps{
		"no checker":         {SystemMessages: &systemMessagesFake{}},
		"no system messages": {Checker: &checkerFake{}},
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("Register accepted incomplete wiring")
				}
			}()
			Register(http.NewServeMux(), d)
		})
	}
}

// emptyOverrideStore is the override table of a commune that reworded nothing. The write methods are
// never reached from this file: the only real-use-case writes here are refused before a transaction.
type emptyOverrideStore struct{}

var errNoWritePath = errors.New("emptyOverrideStore: no write path in this file")

func (emptyOverrideStore) ListLive(context.Context) ([]domain.MessageOverride, error) {
	return nil, nil
}

func (emptyOverrideStore) LiveForUpdate(context.Context, *pkgstore.ScopedTx, string) (*domain.MessageOverride, error) {
	return nil, errNoWritePath
}

func (emptyOverrideStore) AddOverride(context.Context, *pkgstore.ScopedTx, domain.MessageOverride) error {
	return errNoWritePath
}

func (emptyOverrideStore) UpdateText(context.Context, *pkgstore.ScopedTx, domain.MessageOverride) error {
	return errNoWritePath
}

func (emptyOverrideStore) SoftDelete(context.Context, *pkgstore.ScopedTx, string, string, string, time.Time) error {
	return errNoWritePath
}

func (emptyOverrideStore) SetActive(context.Context, *pkgstore.ScopedTx, string, bool, string, time.Time) error {
	return errNoWritePath
}

// --- "Tắt / Bật lại" (ADR 0079 Q2, migration 0004) -----------------------------------------------

func TestSwitchPassesStateRequiresItAndMapsRefusal(t *testing.T) {
	s := newHarness(t)
	s.checker.grant(communeA, "admin.lookup")
	w := call(s.h, http.MethodPatch, hostA, overridePath(titleKey), staffOf(communeA), `{"is_active":true}`)
	wantStatus(t, w, http.StatusOK)
	if s.svc.active == nil || !*s.svc.active || s.svc.op != "switch" || s.svc.key != titleKey {
		t.Errorf("active=%v op=%q key=%q", s.svc.active, s.svc.op, s.svc.key)
	}

	s = newHarness(t)
	s.checker.grant(communeA, "admin.lookup")
	wantStatus(t, call(s.h, http.MethodPatch, hostA, overridePath(titleKey), staffOf(communeA), `{}`), http.StatusBadRequest)
	if s.svc.calls != 0 {
		t.Error("a PATCH with no state reached the use case")
	}
}

func TestListCarriesGroupOriginAndSwitch(t *testing.T) {
	s := newHarness(t)
	s.checker.grant(communeA, "admin.lookup")
	s.svc.list = []domain.SystemMessage{{Key: titleKey, Group: domain.GroupReport, Origin: domain.OriginShipped,
		DefaultText: "M.", CurrentText: "M.", OverrideText: "Của xã.", Overridden: true, Active: false}}
	w := call(s.h, http.MethodGet, hostA, systemMessagesPath, staffOf(communeA), "")
	wantStatus(t, w, http.StatusOK)
	for _, frag := range []string{`"group_code":"bao-cao"`, `"origin":"shipped"`, `"is_active":false`, `"override_text":"Của xã."`} {
		if !strings.Contains(w.Body.String(), frag) {
			t.Errorf("body lacks %s: %s", frag, w.Body.String())
		}
	}
}
