package http

// The four permission cases of the investment project WRITE routes, plus what each one is allowed to
// refuse (docs/ui-ux/06-giai-ngan.md §9, §8).
//
// RULE 5, INVARIANT 7 ASKS FOR FOUR CASES ON EVERY NEW ENDPOINT, and they are asserted on ALL THREE
// routes rather than on whichever one was convenient:
//
//	401  no session
//	403  a signed-in account WITHOUT the key the route declares
//	403  the RIGHT key, but held in ANOTHER commune       (rule 1 · rule 5, invariant 3)
//	2xx  the right key in the right commune
//
// THE THIRD CASE IS THE ONE THAT IS EASY TO FAKE AND THE ONE THAT MATTERS MOST. It needs a checker
// whose grants are KEYED BY COMMUNE — a flat permission set cannot express "holds budget.update, but
// in commune B" — which is why this file uses fakeCatalogueChecker and its own harness rather than
// routes_test.go's.
//
// THE PERMISSION SPLIT UNDER TEST IS THE SPECIFICATION'S OWN (06-giai-ngan.md:202): `budget.update`
// enters and corrects, and the REMOVAL takes `budget.confirm` — a choice routes.go argues in full,
// because the specification assigns the removal no key at all.

import (
	"encoding/json"
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
	"github.com/vihat/vigov/service-finance/internal/app"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"

	"context"
)

// --- fakes ---------------------------------------------------------------------------------------

// fakeInvestmentProjectWriter stands in for the write use case, RECORDING THE COMMUNE IT WAS CALLED IN — read from the
// context exactly as *store.Scoped reads it. A fake that ignored the commune would let a
// wrong-commune case pass while proving nothing.
type fakeInvestmentProjectWriter struct {
	result      domain.InvestmentProject
	allocations []domain.FundingAllocation
	err         error

	createCalls, updateCalls, deleteCalls int
	lastTenant                            tenant.ID
	lastActor                             audit.Actor
	lastCreate                            app.CreateInvestmentProjectRequest
	lastUpdate                            app.UpdateInvestmentProjectRequest
	lastID, lastReason                    string
}

func (g *fakeInvestmentProjectWriter) record(ctx context.Context, actor audit.Actor) {
	g.lastTenant = tenant.MustFrom(ctx)
	g.lastActor = actor
}

func (g *fakeInvestmentProjectWriter) CreateInvestmentProject(ctx context.Context, req app.CreateInvestmentProjectRequest,
	actor audit.Actor) (app.CreateInvestmentProjectResult, error) {
	g.createCalls++
	g.lastCreate = req
	g.record(ctx, actor)
	if g.err != nil {
		return app.CreateInvestmentProjectResult{}, g.err
	}
	return app.CreateInvestmentProjectResult{InvestmentProject: g.result, Allocations: g.allocations}, nil
}

func (g *fakeInvestmentProjectWriter) UpdateInvestmentProject(ctx context.Context, id string, req app.UpdateInvestmentProjectRequest,
	actor audit.Actor) (domain.InvestmentProject, error) {
	g.updateCalls++
	g.lastID, g.lastUpdate = id, req
	g.record(ctx, actor)
	if g.err != nil {
		return domain.InvestmentProject{}, g.err
	}
	return g.result, nil
}

func (g *fakeInvestmentProjectWriter) DeleteInvestmentProject(ctx context.Context, id, reason string, actor audit.Actor) error {
	g.deleteCalls++
	g.lastID, g.lastReason = id, reason
	g.record(ctx, actor)
	return g.err
}

func (g *fakeInvestmentProjectWriter) totalCalls() int {
	return g.createCalls + g.updateCalls + g.deleteCalls
}

// --- harness -------------------------------------------------------------------------------------

type investmentProjectServer struct {
	h       http.Handler
	writer  *fakeInvestmentProjectWriter
	checker *fakeCatalogueChecker
}

// newInvestmentProjectServer mounts the REAL routes through Register, behind the REAL edge chain in the real
// order — including idem.Middleware, which sits INSIDE TenantMiddleware because the idempotency key
// is prefixed with the commune (rule 1, invariant 7).
func newInvestmentProjectServer(t *testing.T) *investmentProjectServer {
	t.Helper()

	writer := &fakeInvestmentProjectWriter{result: domain.InvestmentProject{
		ID:                   "01JDUANMOI0000000000000000",
		Code:                 "DA-2026-be-tong-hoa-duong-ngo-xom",
		Year:                 2026,
		CategoryID:           "hm-chuyen-tiep",
		Name:                 "Bê tông hoá đường ngõ xóm tổ 6",
		PlannedAmount:        100_000_000,
		DisbursementDeadline: time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
	}}
	checker := &fakeCatalogueChecker{}
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))

	mux := http.NewServeMux()
	Register(mux, Deps{
		Checker:                   checker,
		CapitalPlanCategories:     sampleCategories(),
		CapitalPlanCategoryWriter: &fakeCategoryWriter{},
		InvestmentProjects:        sampleInvestmentProjects(),
		InvestmentProjectWriter:   writer,
		// Present so Register accepts the Deps; never called from this file.
		DisbursementVoucherWriter: &fakeVoucherWriter{},
		DelayThresholds:           sampleThresholds(),
		Budget:                    &fakeBudgetReader{},
		BudgetWriter:              &fakeBudgetWriter{},
		AuditLog:                  &auditLogFake{},
		SystemMessages:            &systemMessagesFake{},
		Now:                       func() time.Time { return atElapsed7096 },
		Log:                       quiet,

		// The catalogue's Excel import — never called here; own suite in internal/http/catalogue_import_test.go.
		CapitalPlanCategoryImports: &catalogueImportFake{},
	})

	var h http.Handler = mux
	h = injectPrincipal(h)
	h = idem.Middleware(newFakeIdemStore(), quiet)(h)
	h = httpx.TenantMiddleware(sampleDirectory())(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)

	return &investmentProjectServer{h: h, writer: writer, checker: checker}
}

func (m *investmentProjectServer) grant(tenantID tenant.ID, perm ...authz.Perm) {
	if m.checker.grants == nil {
		m.checker.grants = map[tenant.ID]map[authz.Perm]struct{}{}
	}
	if m.checker.grants[tenantID] == nil {
		m.checker.grants[tenantID] = map[authz.Perm]struct{}{}
	}
	for _, p := range perm {
		m.checker.grants[tenantID][p] = struct{}{}
	}
}

// call ALWAYS SENDS AN Idempotency-Key. POST /api/v1/investment-projects declares idem.Required, which
// refuses a request without the header BEFORE it reaches the handler — so a harness that omitted it
// would turn every POST assertion into an assertion about the header.
func (m *investmentProjectServer) call(t *testing.T, method, host, path string,
	p *authz.Principal, payload string) *httptest.ResponseRecorder {
	t.Helper()
	var body io.Reader
	if payload != "" {
		body = strings.NewReader(payload)
	}
	r := httptest.NewRequest(method, "https://"+host+path, body)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set(idem.Header, "01JIDEMPOTENCYKEYDUAN000")
	if p != nil {
		r = r.WithContext(context.WithValue(r.Context(), principalCtxKey{}, *p))
	}
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)
	return w
}

const (
	investmentProjectPath       = "/api/v1/investment-projects"
	sampleInvestmentProjectID   = "01JDUANDANGCO00000000000"
	createInvestmentProjectBody = `{"code":"DA-2026-be-tong-hoa-duong-ngo-xom","year":2026,` +
		`"category_id":"hm-chuyen-tiep","name":"Bê tông hoá đường ngõ xóm tổ 6",` +
		`"planned_amount":100000000}`
	updateInvestmentProjectBody = `{"name":"Bê tông hoá đường ngõ xóm tổ 6 (điều chỉnh)"}`
	deleteInvestmentProjectBody = `{"reason":"xã rút dự án khỏi kế hoạch vốn năm 2026"}`
)

func sampleInvestmentProjectPath() string {
	return investmentProjectPath + "/" + sampleInvestmentProjectID
}

// investmentProjectWriteRoute is one of the three write routes, with the key the specification assigns to it.
type investmentProjectWriteRoute struct {
	name   string
	method string
	path   string
	body   string
	perm   authz.Perm
	ok     int
	count  func(g *fakeInvestmentProjectWriter) int
}

// threeInvestmentProjectWriteRoutes — the four permission cases are asserted on ALL THREE rather than on whichever one
// was convenient. A route that silently lost its declaration would otherwise be reachable by every
// signed-in account, and nothing would report it (rule 5).
func threeInvestmentProjectWriteRoutes() []investmentProjectWriteRoute {
	return []investmentProjectWriteRoute{
		{
			name: "thêm dự án", method: http.MethodPost, path: investmentProjectPath, body: createInvestmentProjectBody,
			perm: "budget.update", ok: http.StatusCreated,
			count: func(g *fakeInvestmentProjectWriter) int { return g.createCalls },
		},
		{
			name: "sửa dự án", method: http.MethodPatch, path: sampleInvestmentProjectPath(), body: updateInvestmentProjectBody,
			perm: "budget.update", ok: http.StatusOK,
			count: func(g *fakeInvestmentProjectWriter) int { return g.updateCalls },
		},
		{
			// `budget.confirm` AND NOT `budget.update`. The specification assigns the removal no key;
			// routes.go argues the choice, and this case is what makes the choice testable rather than
			// a sentence in a comment.
			name: "xoá mềm dự án", method: http.MethodDelete, path: sampleInvestmentProjectPath(), body: deleteInvestmentProjectBody,
			perm: "budget.confirm", ok: http.StatusNoContent,
			count: func(g *fakeInvestmentProjectWriter) int { return g.deleteCalls },
		},
	}
}

// --- the four cases ------------------------------------------------------------------------------

func TestInvestmentProjectWritesWithoutSessionAre401(t *testing.T) {
	for _, route := range threeInvestmentProjectWriteRoutes() {
		t.Run(route.name, func(t *testing.T) {
			m := newInvestmentProjectServer(t)
			w := m.call(t, route.method, hostA, route.path, nil, route.body)
			wantStatus(t, w, http.StatusUnauthorized)
			if n := m.writer.totalCalls(); n != 0 {
				t.Errorf("use case được gọi %d lần khi chưa đăng nhập, muốn 0", n)
			}
		})
	}
}

func TestInvestmentProjectWritesWithWrongPermissionAre403(t *testing.T) {
	for _, route := range threeInvestmentProjectWriteRoutes() {
		t.Run(route.name, func(t *testing.T) {
			m := newInvestmentProjectServer(t)
			// A real key of the same group, deliberately: `budget.read` is seeded beside the two keys
			// these routes declare, so this proves the route wants THAT key rather than merely "some
			// budget permission".
			m.grant(tenantA, "budget.read")
			w := m.call(t, route.method, hostA, route.path, writerPrincipal(tenantA), route.body)
			wantStatus(t, w, http.StatusForbidden)
			if n := m.writer.totalCalls(); n != 0 {
				t.Errorf("use case được gọi %d lần khi sai quyền, muốn 0", n)
			}
		})
	}
}

// ⚠ THE CASE THAT DECIDES WHETHER THIS IS ONE SYSTEM OR A LEAK, so read what it actually sets up.
//
// The account belongs to commune B and is signed in AT COMMUNE B: nothing about the request is
// malformed, and authz's own commune comparison passes. What is wrong is the GRANT — the right to
// write projects was given in commune A. A checker that ignored the commune would answer yes here,
// and commune A's accountant would be entering projects into commune B's capital plan.
//
// That is rule 5, invariant 3 in one sentence: a permission missing its commune is cross-commune
// escalation, not a lesser bug. It answers 403 rather than 401 because the session is perfectly
// valid — it is the AUTHORITY that is absent.
func TestInvestmentProjectWritesWithPermissionGrantedInOtherCommuneAre403(t *testing.T) {
	for _, route := range threeInvestmentProjectWriteRoutes() {
		t.Run(route.name, func(t *testing.T) {
			m := newInvestmentProjectServer(t)
			m.grant(tenantA, "budget.update", "budget.confirm")

			w := m.call(t, route.method, hostB, route.path, writerPrincipal(tenantB), route.body)
			wantStatus(t, w, http.StatusForbidden)
			if n := m.writer.totalCalls(); n != 0 {
				t.Errorf("quyền cấp ở xã khác mà vẫn ghi được vào xã này (%d lần)", n)
			}
		})
	}
}

// THE OTHER SHAPE OF "wrong commune", and it is a DIFFERENT answer on purpose: a principal issued by
// commune B presented at commune A's domain. authz.RequirePermission compares the commune BEFORE the
// permission and answers 401 — rule 1, invariant 8, and the multi-tenant model's own table: "Token
// cấp cho xã A, gửi tới domain xã B → 401 + báo động". A browser does not send a cookie across hosts,
// so this is never an ordinary user error.
//
// ASSERTED SO THAT NOBODY "CORRECTS" IT TO 403 and turns a deliberate probe into an ordinary
// permission miss in the logs.
func TestInvestmentProjectWritesWithSessionOfOtherCommuneAre401(t *testing.T) {
	for _, route := range threeInvestmentProjectWriteRoutes() {
		t.Run(route.name, func(t *testing.T) {
			m := newInvestmentProjectServer(t)
			m.grant(tenantA, route.perm)
			m.grant(tenantB, route.perm)

			w := m.call(t, route.method, hostA, route.path, writerPrincipal(tenantB), route.body)
			wantStatus(t, w, http.StatusUnauthorized)
			if n := m.writer.totalCalls(); n != 0 {
				t.Errorf("phiên của xã khác mà vẫn ghi được (%d lần)", n)
			}
		})
	}
}

func TestInvestmentProjectWritesWithRightPermissionRightCommuneRun(t *testing.T) {
	for _, route := range threeInvestmentProjectWriteRoutes() {
		t.Run(route.name, func(t *testing.T) {
			m := newInvestmentProjectServer(t)
			m.grant(tenantA, route.perm)
			w := m.call(t, route.method, hostA, route.path, writerPrincipal(tenantA), route.body)
			wantStatus(t, w, route.ok)
			if n := route.count(m.writer); n != 1 {
				t.Errorf("use case được gọi %d lần, muốn 1", n)
			}
			// THE COMMUNE THE USE CASE RAN IN IS THE ONE THE HOST RESOLVED TO, never one a client named.
			// httpx.StripTenantHeaders removes any tenant header from outside; this asserts what
			// actually reached the store layer (rule 1, invariants 3 and 4).
			if m.writer.lastTenant != tenantA {
				t.Errorf("use case chạy trong xã %q, muốn %q", m.writer.lastTenant, tenantA)
			}
			// THE ACTOR IS THE STAFF BUSINESS CODE, never the internal id (rule 6, invariant 8).
			if m.writer.lastActor.ID != writerStaffCode {
				t.Errorf("chủ thể = %q, muốn mã cán bộ %q — không bao giờ id nội bộ",
					m.writer.lastActor.ID, writerStaffCode)
			}
			if m.writer.lastActor.IP == "" {
				t.Error("vết không mang IP — luật 6 bất biến 2 đòi `từ IP nào`")
			}
		})
	}
}

// --- what the handler itself refuses --------------------------------------------------------------

// Rule 7, forbidden #4 and §13 rule 8, at the edge. Both are REFUSED rather than ignored: ignoring
// would leave the client believing the change landed while every screen still showed the old value.
func TestUpdateInvestmentProjectRefusesCodeAndBudgetYearChange(t *testing.T) {
	for _, tc := range []struct {
		name, body, frag string
	}{
		{"đổi mã", `{"code":"DA-2026-khac"}`, "mã đã cấp"},
		{"đổi năm ngân sách", `{"year":2027}`, "năm ngân sách khác"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newInvestmentProjectServer(t)
			m.grant(tenantA, "budget.update")
			w := m.call(t, http.MethodPatch, hostA, sampleInvestmentProjectPath(), writerPrincipal(tenantA), tc.body)
			wantStatus(t, w, http.StatusBadRequest)
			if m.writer.updateCalls != 0 {
				t.Errorf("use case được gọi %d lần, muốn 0 — từ chối ở tầng biên", m.writer.updateCalls)
			}
			if e := errorBody(t, w); !strings.Contains(e.Message, tc.frag) {
				t.Errorf("thông điệp = %q, muốn nhắc tới %q", e.Message, tc.frag)
			}
		})
	}
}

// §9's allocation list reaches the use case intact, one line per source, so nothing between the
// screen and the transaction can drop a source silently.
func TestCreateInvestmentProjectPassesAllocationsToUseCase(t *testing.T) {
	m := newInvestmentProjectServer(t)
	m.grant(tenantA, "budget.update")

	body := `{"code":"DA-2026-moi","year":2026,"category_id":"hm-chuyen-tiep",` +
		`"name":"Dự án mới","planned_amount":100000000,` +
		`"funding_allocations":[{"funding_source_id":"nv-xa","amount":40000000},` +
		`{"funding_source_id":"nv-thanh-pho","amount":60000000}]}`

	w := m.call(t, http.MethodPost, hostA, investmentProjectPath, writerPrincipal(tenantA), body)
	wantStatus(t, w, http.StatusCreated)

	if n := len(m.writer.lastCreate.Allocations); n != 2 {
		t.Fatalf("use case nhận %d dòng phân bổ, muốn 2", n)
	}
	if got := m.writer.lastCreate.Allocations[0]; got.FundingSourceID != "nv-xa" || got.Amount != 40_000_000 {
		t.Errorf("dòng phân bổ đầu = %+v, muốn {nv-xa 40000000}", got)
	}
}

// §9's dynamic list is OPTIONAL and §11 names the state it produces (`Chưa gắn nguồn`). A route that
// required it would refuse exactly the project §9 calls ordinary.
func TestCreateInvestmentProjectWithoutFundingSourcesIsStill201(t *testing.T) {
	m := newInvestmentProjectServer(t)
	m.grant(tenantA, "budget.update")

	w := m.call(t, http.MethodPost, hostA, investmentProjectPath, writerPrincipal(tenantA), createInvestmentProjectBody)
	wantStatus(t, w, http.StatusCreated)
	if n := len(m.writer.lastCreate.Allocations); n != 0 {
		t.Errorf("use case nhận %d dòng phân bổ, muốn 0", n)
	}

	var result duAnGhiRa
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	// ⚠ `funding_allocated_total` MUST BE PRESENT AND ZERO, not absent. A field that vanished at zero
	// would make a client unable to tell "nothing allocated" from "this server does not report it" —
	// and §11's `Chưa gắn nguồn` chip is derived from exactly this figure.
	if !strings.Contains(w.Body.String(), `"funding_allocated_total"`) {
		t.Errorf("thiếu `funding_allocated_total` khi chưa gắn nguồn: %q", w.Body.String())
	}
	if result.FundingAllocatedTotal != 0 {
		t.Errorf("funding_allocated_total = %d, muốn 0", result.FundingAllocatedTotal)
	}
}

// ⚠ §9 SAYS THE MISMATCH IS A WARNING, NOT A REFUSAL. The route returns the two raw figures —
// `planned_amount` and `funding_allocated_total` — and lets the screen make §9's warning and §11's
// chip out of them. This case is what stops a future reader from turning that sentence into a 400.
func TestCreateInvestmentProjectSourcesOverPlanIsStill201AndReturnsBothFigures(t *testing.T) {
	m := newInvestmentProjectServer(t)
	m.grant(tenantA, "budget.update")
	m.writer.allocations = []domain.FundingAllocation{
		{ID: "01JPB1", FundingSourceID: "nv-xa", AllocatedAmount: 500_000_000},
	}

	body := `{"code":"DA-2026-moi","year":2026,"category_id":"hm-chuyen-tiep",` +
		`"name":"Dự án mới","planned_amount":100000000,` +
		`"funding_allocations":[{"funding_source_id":"nv-xa","amount":500000000}]}`

	w := m.call(t, http.MethodPost, hostA, investmentProjectPath, writerPrincipal(tenantA), body)
	wantStatus(t, w, http.StatusCreated)

	var result duAnGhiRa
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	if result.FundingAllocatedTotal != 500_000_000 {
		t.Errorf("funding_allocated_total = %d, muốn 500000000", result.FundingAllocatedTotal)
	}
	if result.PlannedAmount != 100_000_000 {
		t.Errorf("planned_amount = %d, muốn 100000000", result.PlannedAmount)
	}
}

// The refusals the use case raises, each mapped to the status a client can act on. 409 AND NOT 403 on
// every business refusal: the caller HOLDS the permission — what is refused is this operation on this
// record.
func TestInvestmentProjectWritesMapBusinessErrorsToStatus(t *testing.T) {
	for _, tc := range []struct {
		name    string
		failure error
		method  string
		path    string
		body    string
		perm    authz.Perm
		want    int
		field   string
	}{
		{
			name: "mã dự án đã cấp", failure: fistore.ErrInvestmentProjectCodeTaken,
			method: http.MethodPost, path: investmentProjectPath, body: createInvestmentProjectBody,
			perm: "budget.update", want: http.StatusConflict, field: "code",
		},
		{
			name: "hạng mục không có trong xã", failure: fistore.ErrCategoryNotFound,
			method: http.MethodPost, path: investmentProjectPath, body: createInvestmentProjectBody,
			perm: "budget.update", want: http.StatusNotFound, field: "category_id",
		},
		{
			name: "nguồn vốn sai năm ngân sách", failure: fistore.ErrAllocationFundingSourceNotFound,
			method: http.MethodPost, path: investmentProjectPath, body: createInvestmentProjectBody,
			perm: "budget.update", want: http.StatusNotFound, field: "funding_allocations",
		},
		{
			name: "không có dự án", failure: fistore.ErrInvestmentProjectNotFound,
			method: http.MethodPatch, path: sampleInvestmentProjectPath(), body: updateInvestmentProjectBody,
			perm: "budget.update", want: http.StatusNotFound,
		},
		{
			// ⚠ THE STOP CONDITION, AT THE EDGE. Refusing is the only direction that writes nothing,
			// and the sentence has to name the way out or the screen looks broken: the project is
			// right there and the Delete button did nothing.
			name: "dự án còn chứng từ", failure: fistore.ErrInvestmentProjectHasVouchers,
			method: http.MethodDelete, path: sampleInvestmentProjectPath(), body: deleteInvestmentProjectBody,
			perm: "budget.confirm", want: http.StatusConflict,
		},
		{
			name: "thiếu lý do xoá", failure: domain.ErrInvestmentProjectDeleteReasonMissing,
			method: http.MethodDelete, path: sampleInvestmentProjectPath(), body: `{"reason":""}`,
			perm: "budget.confirm", want: http.StatusBadRequest,
		},
		{
			name: "thiếu mã dự án — hệ thống không tự sinh", failure: domain.ErrInvestmentProjectCodeMissing,
			method: http.MethodPost, path: investmentProjectPath, body: `{"year":2026}`,
			perm: "budget.update", want: http.StatusBadRequest,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newInvestmentProjectServer(t)
			m.grant(tenantA, tc.perm)
			m.writer.err = tc.failure

			w := m.call(t, tc.method, hostA, tc.path, writerPrincipal(tenantA), tc.body)
			wantStatus(t, w, tc.want)

			e := errorBody(t, w)
			if e.Message == "" {
				t.Error("thông điệp rỗng — người dùng không biết phải làm gì")
			}
			// THE FIELD AT FAULT IS NAMED IN THE SENTENCE, backtick-quoted.
			if tc.field != "" && !strings.Contains(e.Message, "`"+tc.field+"`") {
				t.Errorf("thông điệp = %q, muốn nêu trường `%s`", e.Message, tc.field)
			}
			// ⚠ httpx.Error HAS NO `field` AT ALL — it carries `code`, `message` and `trace_id`
			// (core/httpx/edge.go:88-92) — so the FIFTH parameter of httpx.WriteError is `traceID`. A
			// field name passed there lands in `trace_id` and looks exactly like a real trace id,
			// sending an operator to search centralised logging for something that was never written.
			// The same defect shipped in service-petitions and was found on 2026-09-23.
			for _, bad := range []string{"code", "category_id", "funding_allocations", "id", "reason",
				"start_date", "completion_date", "disbursement_deadline", "year", "name"} {
				if e.TraceID == bad {
					t.Errorf("trace_id = %q — đó là TÊN TRƯỜNG chui vào chỗ traceID", e.TraceID)
				}
			}
		})
	}
}

// A malformed date must name the field that carried it, and must never reach the use case.
func TestCreateInvestmentProjectMalformedDateIs400AndNamesField(t *testing.T) {
	m := newInvestmentProjectServer(t)
	m.grant(tenantA, "budget.update")

	body := `{"code":"DA-2026-moi","year":2026,"category_id":"hm-chuyen-tiep",` +
		`"name":"Dự án mới","planned_amount":100000000,"start_date":"07/09/2026"}`

	w := m.call(t, http.MethodPost, hostA, investmentProjectPath, writerPrincipal(tenantA), body)
	wantStatus(t, w, http.StatusBadRequest)
	if m.writer.createCalls != 0 {
		t.Errorf("use case được gọi %d lần, muốn 0", m.writer.createCalls)
	}
	e := errorBody(t, w)
	if !strings.Contains(e.Message, "`start_date`") {
		t.Errorf("thông điệp = %q, muốn nêu trường `start_date`", e.Message)
	}
	// AND NOT IN `trace_id`, which is what the fifth parameter of httpx.WriteError really is.
	if e.TraceID == "start_date" {
		t.Errorf("trace_id = %q — tên trường chui vào chỗ traceID", e.TraceID)
	}
}

// The soft delete answers 204 with no body. Returning the project would invite a client to display
// one it has just taken off the screen.
func TestDeleteInvestmentProjectReturns204NoBody(t *testing.T) {
	m := newInvestmentProjectServer(t)
	m.grant(tenantA, "budget.confirm")

	w := m.call(t, http.MethodDelete, hostA, sampleInvestmentProjectPath(), writerPrincipal(tenantA), deleteInvestmentProjectBody)
	wantStatus(t, w, http.StatusNoContent)
	if w.Body.Len() != 0 {
		t.Errorf("thân = %q, muốn rỗng", w.Body.String())
	}
	// The reason reaches the use case, where it becomes `delete_reason` (rule 7, invariant 1).
	if m.writer.lastReason == "" {
		t.Error("lý do xoá không tới được use case")
	}
	if m.writer.lastID != sampleInvestmentProjectID {
		t.Errorf("id = %q, muốn %q", m.writer.lastID, sampleInvestmentProjectID)
	}
}
