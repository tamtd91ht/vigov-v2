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
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// "Lời hệ thống", finance half — the rule 5 invariant 7 set on all three routes, plus what the
// handler owns: which key and text reach the use case, who the actor is, and how each refusal is
// answered. The fallback rule and the transaction are app/system_message_test.go's.
//
// ITS OWN HARNESS, for the reason audit_entries_test.go gives: the "right permission, wrong
// commune" case needs fakeCatalogueChecker, whose grants are keyed by commune.

const systemMessagesPath = "/api/v1/finance-system-messages"

func overridePath(key string) string { return systemMessagesPath + "/" + key + "/override" }

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

	// Text is recorded apart from `calls`: the investment-project reads call it on every request,
	// and the three routes above count their own calls.
	textCalls   int
	textKey     string
	textCommune tenant.ID
	textOut     string // "" answers the shipped default, which is what a commune that reworded nothing reads
	textErr     error
}

func (f *systemMessagesFake) Text(ctx context.Context, key string) (string, error) {
	f.textCalls++
	f.textKey, f.textCommune = key, tenant.MustFrom(ctx)
	if f.textErr != nil {
		return "", f.textErr
	}
	if f.textOut != "" {
		return f.textOut, nil
	}
	m, ok := domain.LookupShippedMessage(key)
	if !ok {
		return "", domain.ErrUnknownMessageKey
	}
	return m.DefaultText, nil
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

type systemMessagesHarness struct {
	h       http.Handler
	svc     *systemMessagesFake
	checker *fakeCatalogueChecker
}

func newSystemMessagesHarness(t *testing.T) *systemMessagesHarness {
	t.Helper()
	svc := &systemMessagesFake{}
	checker := &fakeCatalogueChecker{}
	lg := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	Register(mux, Deps{
		Checker:                   checker,
		CapitalPlanCategories:     sampleCategories(),
		CapitalPlanCategoryWriter: &fakeCategoryWriter{},
		InvestmentProjects:        sampleInvestmentProjects(),
		InvestmentProjectWriter:   &fakeInvestmentProjectWriter{},
		DisbursementVoucherWriter: &fakeVoucherWriter{},
		DelayThresholds:           defaultThresholds(),
		Budget:                    sampleBudget(),
		BudgetWriter:              &fakeBudgetWriter{},
		AuditLog:                  &auditLogFake{},
		SystemMessages:            svc,
		Now:                       func() time.Time { return atElapsed7096 },
		Log:                       lg,

		// The catalogue's Excel import — never called here; own suite in internal/http/catalogue_import_test.go.
		CapitalPlanCategoryImports: &catalogueImportFake{},
	})
	var h http.Handler = mux
	h = injectPrincipal(h)
	h = idem.Middleware(nil, lg)(h)
	h = httpx.TenantMiddleware(sampleDirectory())(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)
	return &systemMessagesHarness{h: h, svc: svc, checker: checker}
}

func (s *systemMessagesHarness) grant(tenantID tenant.ID, perm ...authz.Perm) {
	if s.checker.grants == nil {
		s.checker.grants = map[tenant.ID]map[authz.Perm]struct{}{}
	}
	if s.checker.grants[tenantID] == nil {
		s.checker.grants[tenantID] = map[authz.Perm]struct{}{}
	}
	for _, p := range perm {
		s.checker.grants[tenantID][p] = struct{}{}
	}
}

func (s *systemMessagesHarness) call(method, host, path string, p *authz.Principal, body string) *httptest.ResponseRecorder {
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	r := httptest.NewRequest(method, "https://"+host+path, rd)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	r.Header.Set("Content-Type", "application/json")
	if p != nil {
		r = r.WithContext(context.WithValue(r.Context(), principalCtxKey{}, *p))
	}
	w := httptest.NewRecorder()
	s.h.ServeHTTP(w, r)
	return w
}

type systemMessageRoute struct {
	name, method, path, body string
	ok                       int
}

func systemMessageRoutes() []systemMessageRoute {
	return []systemMessageRoute{
		{"GET", http.MethodGet, systemMessagesPath, "", http.StatusOK},
		{"PUT", http.MethodPut, overridePath(domain.KeyBudgetScopeNotice), `{"text":"Câu của xã."}`, http.StatusOK},
		{"DELETE", http.MethodDelete, overridePath(domain.KeyBudgetScopeNotice), "", http.StatusNoContent},
	}
}

// --- rule 5, invariant 7 on all three routes ------------------------------------------------------

func TestSystemMessages_401WithoutSession(t *testing.T) {
	for _, rt := range systemMessageRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			s := newSystemMessagesHarness(t)
			s.grant(tenantA, "admin.lookup")
			wantStatus(t, s.call(rt.method, hostA, rt.path, nil, rt.body), http.StatusUnauthorized)
			if s.svc.calls != 0 {
				t.Error("use case ran with no session")
			}
		})
	}
}

func TestSystemMessages_403WrongPermission(t *testing.T) {
	// `budget.read` and `admin.audit` are real keys and deliberately not enough: reading the
	// budget, or reading the trail, is not administering the commune's configuration.
	for _, rt := range systemMessageRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			s := newSystemMessagesHarness(t)
			s.grant(tenantA, "budget.read", "budget.confirm", "admin.audit")
			wantStatus(t, s.call(rt.method, hostA, rt.path, writerPrincipal(tenantA), rt.body), http.StatusForbidden)
			if s.svc.calls != 0 {
				t.Error("use case ran without admin.lookup")
			}
			// The key the ROUTE asked for, against a literal: a fake checker grants any string, so
			// only this catches a key the `quyen` table lacks (rule 5, invariant 3c).
			if got := s.checker.lastAskedKey(); got != "admin.lookup" {
				t.Errorf("route asked for %q, want admin.lookup", got)
			}
		})
	}
}

func TestSystemMessages_403RightPermissionWrongCommune(t *testing.T) {
	// Granted in commune A; the account is commune B's, signed in at commune B.
	for _, rt := range systemMessageRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			s := newSystemMessagesHarness(t)
			s.grant(tenantA, "admin.lookup")
			wantStatus(t, s.call(rt.method, hostB, rt.path, writerPrincipal(tenantB), rt.body), http.StatusForbidden)
			if s.svc.calls != 0 {
				t.Error("commune A's grant let commune B configure its sentences")
			}
		})
	}
}

func TestSystemMessages_200RightPermissionRightCommune(t *testing.T) {
	for _, rt := range systemMessageRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			s := newSystemMessagesHarness(t)
			s.grant(tenantA, "admin.lookup")
			wantStatus(t, s.call(rt.method, hostA, rt.path, writerPrincipal(tenantA), rt.body), rt.ok)
			if s.svc.calls != 1 || s.svc.commune != tenantA {
				t.Fatalf("calls=%d commune=%q, want one call in the Host's commune", s.svc.calls, s.svc.commune)
			}
			if rt.method != http.MethodGet {
				// Rule 6, invariant 8: the trail names the BUSINESS CODE, never the internal id.
				if s.svc.actor.ID != writerStaffCode || s.svc.actor.ID == writerInternalID || s.svc.actor.IP != "10.0.0.7" {
					t.Errorf("actor = %+v, want business code %s and socket IP", s.svc.actor, writerStaffCode)
				}
				if s.svc.key != domain.KeyBudgetScopeNotice {
					t.Errorf("key = %q", s.svc.key)
				}
			}
		})
	}
}

// --- what each route passes on and answers -------------------------------------------------------

func TestListSystemMessagesShape(t *testing.T) {
	s := newSystemMessagesHarness(t)
	s.grant(tenantA, "admin.lookup")
	at := time.Date(2026, 9, 28, 3, 0, 0, 0, time.UTC)
	s.svc.list = []domain.SystemMessage{{
		Key: domain.KeyBudgetScopeNotice, Description: "d", DefaultText: "Mặc định.",
		CurrentText: "Câu của xã.", Overridden: true, UpdatedAt: &at, UpdatedBy: "CB-00777",
	}}
	w := s.call(http.MethodGet, hostA, systemMessagesPath+"?tenant_id=01JOTHERCOMMUNE", writerPrincipal(tenantA), "")
	wantStatus(t, w, http.StatusOK)
	if s.svc.commune != tenantA {
		t.Errorf("read in %q — the commune is the Host's, never a query parameter", s.svc.commune)
	}
	var out struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || len(out.Items) != 1 {
		t.Fatalf("body %s", w.Body.String())
	}
	it := out.Items[0]
	for k, want := range map[string]any{
		"code": domain.KeyBudgetScopeNotice, "default_text": "Mặc định.", "current_text": "Câu của xã.",
		"overridden": true, "updated_by": "CB-00777", "updated_at": "2026-09-28T03:00:00Z",
	} {
		if it[k] != want {
			t.Errorf("%s = %v, want %v", k, it[k], want)
		}
	}
}

func TestListSystemMessagesOnDefaultOmitsWhoAndWhen(t *testing.T) {
	s := newSystemMessagesHarness(t)
	s.grant(tenantA, "admin.lookup")
	s.svc.list = []domain.SystemMessage{{Key: domain.KeyBudgetScopeNotice, DefaultText: "M.", CurrentText: "M."}}
	w := s.call(http.MethodGet, hostA, systemMessagesPath, writerPrincipal(tenantA), "")
	wantStatus(t, w, http.StatusOK)
	if strings.Contains(w.Body.String(), "updated_by") || strings.Contains(w.Body.String(), "updated_at") {
		t.Errorf("a key nobody changed names somebody: %s", w.Body.String())
	}
}

func TestRewordSystemMessagePassesTextAndReturnsMessage(t *testing.T) {
	s := newSystemMessagesHarness(t)
	s.grant(tenantA, "admin.lookup")
	s.svc.result = domain.SystemMessage{Key: domain.KeyBudgetScopeNotice, CurrentText: "Câu của xã.", Overridden: true}
	w := s.call(http.MethodPut, hostA, overridePath(domain.KeyBudgetScopeNotice), writerPrincipal(tenantA), `{"text":"Câu của xã."}`)
	wantStatus(t, w, http.StatusOK)
	if s.svc.text != "Câu của xã." || s.svc.op != "reword" {
		t.Errorf("text=%q op=%q", s.svc.text, s.svc.op)
	}
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out["current_text"] != "Câu của xã." || out["overridden"] != true {
		t.Errorf("body %s", w.Body.String())
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
			s := newSystemMessagesHarness(t)
			s.grant(tenantA, "admin.lookup")
			s.svc.err = c.err
			w := s.call(http.MethodPut, hostA, overridePath(domain.KeyBudgetScopeNotice), writerPrincipal(tenantA), `{"text":""}`)
			wantStatus(t, w, c.want)
			e := errorBody(t, w)
			if e.Code != c.code || strings.Contains(e.Message, "connection reset") {
				t.Errorf("error = %+v", e)
			}
		})
	}
}

func TestEmptyTextNamesTheRestoreAction(t *testing.T) {
	// 400 on empty text must tell the administrator what to do instead.
	s := newSystemMessagesHarness(t)
	s.grant(tenantA, "admin.lookup")
	s.svc.err = domain.ErrMessageTextEmpty
	w := s.call(http.MethodPut, hostA, overridePath(domain.KeyBudgetScopeNotice), writerPrincipal(tenantA), `{"text":"  "}`)
	wantStatus(t, w, http.StatusBadRequest)
	if !strings.Contains(errorBody(t, w).Message, "Khôi phục câu mặc định") {
		t.Errorf("message %q does not point to the restore action", errorBody(t, w).Message)
	}
}

func TestUnknownKeyIs404OnBothWrites(t *testing.T) {
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			s := newSystemMessagesHarness(t)
			s.grant(tenantA, "admin.lookup")
			s.svc.err = domain.ErrUnknownMessageKey
			body := ""
			if method == http.MethodPut {
				body = `{"text":"x"}`
			}
			w := s.call(method, hostA, overridePath("feedback.reason_required"), writerPrincipal(tenantA), body)
			wantStatus(t, w, http.StatusNotFound)
			if s.svc.key != "feedback.reason_required" {
				t.Errorf("key reaching use case = %q", s.svc.key)
			}
		})
	}
}

func TestRewordBadJSONIs400AndCallsNothing(t *testing.T) {
	s := newSystemMessagesHarness(t)
	s.grant(tenantA, "admin.lookup")
	wantStatus(t, s.call(http.MethodPut, hostA, overridePath(domain.KeyBudgetScopeNotice), writerPrincipal(tenantA), `{"text":`),
		http.StatusBadRequest)
	if s.svc.calls != 0 {
		t.Error("malformed body reached the use case")
	}
}

func TestSystemMessageWriteWithoutBusinessCodeIs500(t *testing.T) {
	// staffOf carries no `Ma`: the trail could not name who acted, so nothing is written.
	s := newSystemMessagesHarness(t)
	s.grant(tenantA, "admin.lookup")
	wantStatus(t, s.call(http.MethodDelete, hostA, overridePath(domain.KeyBudgetScopeNotice), staffOf(tenantA), ""),
		http.StatusInternalServerError)
	if s.svc.calls != 0 {
		t.Error("write ran for a principal with no business code")
	}
}

func TestSystemMessageWritesNeedNoIdempotencyKey(t *testing.T) {
	// Both writes declare idem.KhongCan; the harness sends no Idempotency-Key at all, so a 200/204
	// here is the declaration under test. Making them Required would break the Lưu button.
	s := newSystemMessagesHarness(t)
	s.grant(tenantA, "admin.lookup")
	wantStatus(t, s.call(http.MethodPut, hostA, overridePath(domain.KeyBudgetScopeNotice), writerPrincipal(tenantA), `{"text":"x"}`), http.StatusOK)
	wantStatus(t, s.call(http.MethodDelete, hostA, overridePath(domain.KeyBudgetScopeNotice), writerPrincipal(tenantA), ""), http.StatusNoContent)
}
