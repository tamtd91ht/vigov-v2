package http

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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
)

// WHAT THIS FILE IS FOR: the SIX write routes of the disbursement voucher register — the first
// routes in this repository that move money.
//
// FIVE THINGS, each of which fails silently if it stops holding:
//
//  1. the four cases of rule 5, invariant 7, on EVERY one of the six routes — and the third case is
//     the one that is easy to fake, see TestVoucherWrites_403RightPermissionWrongCommune;
//  2. each route asks for the key the SPECIFICATION assigns to that act (06-giai-ngan.md:202), and
//     that key exists in the `quyen` table. A key no migration seeds is a route that answers 403 to
//     every account forever while every test stays green (rule 5, invariant 3c);
//  3. `status` and `project_id` are REFUSED, not ignored. One decides the lifecycle, the other moves
//     money between two reported totals;
//  4. a refusal reaches the caller as a SENTENCE AN ACCOUNTANT CAN ACT ON — §13 rule 3's own words,
//     with 409 rather than 403, because the caller holds the permission and the ROW is what refuses;
//  5. the commune and the acting person reach the use case, because they are what the audit entry is
//     filed under (rule 6, invariant 2).

// --- fakes ---------------------------------------------------------------------------------------

// fakeVoucherWriter stands in for the write use case, RECORDING THE COMMUNE IT WAS CALLED IN — read from
// the context exactly as *store.Scoped reads it. A fake that ignored the commune would let a
// wrong-commune case pass while proving nothing.
type fakeVoucherWriter struct {
	result domain.DisbursementVoucher
	err    error

	createCalls, updateCalls, removeCalls int
	confirmCalls, lockCalls, unlockCalls  int
	lastTenant                            tenant.ID
	lastActor                             audit.Actor
	lastCreate                            app.CreateDisbursementVoucherRequest
	lastUpdate                            app.UpdateDisbursementVoucherRequest
	lastID, lastReason                    string
}

func (g *fakeVoucherWriter) record(ctx context.Context, actor audit.Actor) {
	g.lastTenant = tenant.MustFrom(ctx)
	g.lastActor = actor
}

func (g *fakeVoucherWriter) CreateVoucher(ctx context.Context, req app.CreateDisbursementVoucherRequest,
	actor audit.Actor) (domain.DisbursementVoucher, error) {
	g.createCalls++
	g.lastCreate = req
	g.record(ctx, actor)
	if g.err != nil {
		return domain.DisbursementVoucher{}, g.err
	}
	return g.result, nil
}

func (g *fakeVoucherWriter) UpdateVoucher(ctx context.Context, id string, req app.UpdateDisbursementVoucherRequest,
	actor audit.Actor) (domain.DisbursementVoucher, error) {
	g.updateCalls++
	g.lastID, g.lastUpdate = id, req
	g.record(ctx, actor)
	if g.err != nil {
		return domain.DisbursementVoucher{}, g.err
	}
	return g.result, nil
}

func (g *fakeVoucherWriter) Remove(ctx context.Context, id, reason string, actor audit.Actor) error {
	g.removeCalls++
	g.lastID, g.lastReason = id, reason
	g.record(ctx, actor)
	return g.err
}

func (g *fakeVoucherWriter) Confirm(ctx context.Context, id string,
	actor audit.Actor) (domain.DisbursementVoucher, error) {
	g.confirmCalls++
	g.lastID = id
	g.record(ctx, actor)
	if g.err != nil {
		return domain.DisbursementVoucher{}, g.err
	}
	return g.result, nil
}

func (g *fakeVoucherWriter) Lock(ctx context.Context, id string,
	actor audit.Actor) (domain.DisbursementVoucher, error) {
	g.lockCalls++
	g.lastID = id
	g.record(ctx, actor)
	if g.err != nil {
		return domain.DisbursementVoucher{}, g.err
	}
	return g.result, nil
}

func (g *fakeVoucherWriter) Unlock(ctx context.Context, id, reason string,
	actor audit.Actor) (domain.DisbursementVoucher, error) {
	g.unlockCalls++
	g.lastID, g.lastReason = id, reason
	g.record(ctx, actor)
	if g.err != nil {
		return domain.DisbursementVoucher{}, g.err
	}
	return g.result, nil
}

func (g *fakeVoucherWriter) totalCalls() int {
	return g.createCalls + g.updateCalls + g.removeCalls + g.confirmCalls + g.lockCalls + g.unlockCalls
}

// fakeThresholds answers the threshold, KEYED BY COMMUNE, reading the commune from the context exactly as
// *store.Scoped does. Keyed any other way, the cross-commune case would pass while proving nothing —
// and this value decides whether a commune's KPI card reads "29 dự án chậm" or "0".
type fakeThresholds struct {
	byTenant map[tenant.ID]domain.DelayThreshold
	err      error
	calls    int
	lastYear int
}

func (n *fakeThresholds) DelayThreshold(ctx context.Context, year int) (domain.DelayThreshold, error) {
	n.calls++
	n.lastYear = year
	if n.err != nil {
		return domain.DelayThreshold{}, n.err
	}
	if v, ok := n.byTenant[tenant.MustFrom(ctx)]; ok {
		return v, nil
	}
	// NO ROW IS THE ORDINARY STATE of every commune today — `cau_hinh_giai_ngan` has no write path
	// yet — and the store answers it with the software's default, MARKED AS THE SOFTWARE'S.
	return domain.SoftwareDefaultThreshold(), nil
}

// sampleThresholds gives commune A a figure IT CHOSE and leaves commune B on the default, so that the two
// provenances are both reachable and cannot be confused with each other.
//
// 15 POINTS AND NOT 10, deliberately: a commune that had chosen exactly the software's default would
// make "the commune chose this" and "nobody has chosen anything" produce the same number, and the
// whole point of the provenance field is that those two are different statements.
func sampleThresholds() *fakeThresholds {
	return &fakeThresholds{byTenant: map[tenant.ID]domain.DelayThreshold{
		tenantA: domain.CommuneThreshold(15 * domain.OnePercent),
	}}
}

// defaultThresholds leaves EVERY commune on the software's default — the state of every commune today,
// because nothing writes `cau_hinh_giai_ngan` yet. It is what routes_test.go's harness uses, so the
// disbursement read assertions there stay about the projects rather than about a threshold fixture.
func defaultThresholds() *fakeThresholds { return &fakeThresholds{} }

// fakeIdemStore is an in-memory idempotency store — a map, and the TTLs are ignored.
//
// WHY THIS FILE NEEDS ONE WHEN NO OTHER SUITE DOES: `POST /api/v1/disbursements` declares
// idem.Required(idem.DongKhiHong), so with NO store it answers 503 to everything — correctly, and
// that is asserted on its own below. Every other assertion about that route is about what happens
// AFTER the duplicate check passes, and a nil store would make all of them assertions about Redis
// being absent.
//
// The catalogue suite next door has no such need: its POST declares MoKhiHong, which lets a request
// through when the cache is down. The difference between the two declarations is exactly what money
// costs, and it shows up here as a fixture.
type fakeIdemStore struct {
	mu      sync.Mutex
	entries map[string]string
}

func newFakeIdemStore() *fakeIdemStore { return &fakeIdemStore{entries: map[string]string{}} }

func (k *fakeIdemStore) Claim(_ context.Context, key string, _ time.Duration) (bool, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if _, ok := k.entries[key]; ok {
		return false, nil
	}
	k.entries[key] = ""
	return true, nil
}

func (k *fakeIdemStore) Get(_ context.Context, key string) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.entries[key], nil
}

func (k *fakeIdemStore) Complete(_ context.Context, key, value string, _ time.Duration) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.entries[key] = value
	return nil
}

func (k *fakeIdemStore) Release(_ context.Context, key string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.entries, key)
	return nil
}

// --- harness -------------------------------------------------------------------------------------
//
// ITS OWN, and not routes_test.go's, for the same reason capital_plan_category_write_test.go has one:
// the "right permission, wrong commune" case needs a checker whose grants are KEYED BY COMMUNE,
// which a flat permission set cannot express.

type voucherServer struct {
	h       http.Handler
	writer  *fakeVoucherWriter
	checker *fakeCatalogueChecker
}

// newVoucherServer mounts the REAL routes through Register, behind the REAL edge chain in the real
// order — including idem.Middleware with a NIL store, which is a valid deployment (local development
// with no Redis) and is what makes each route's declared CheDoHong the thing under test rather than
// Redis behaviour. It sits INSIDE TenantMiddleware because the idempotency key is prefixed with the
// commune (rule 1, invariant 7).
func newVoucherServer(t *testing.T) *voucherServer {
	t.Helper()
	return newVoucherServerWith(t, newFakeIdemStore())
}

// newVoucherServerWith takes the idempotency store, so that one case can pass NIL and assert what
// DongKhiHong actually does when the cache is gone.
func newVoucherServerWith(t *testing.T, idemStore idem.Store) *voucherServer {
	t.Helper()

	writer := &fakeVoucherWriter{result: domain.DisbursementVoucher{
		ID: "01JCHUNGTUMOI000000000000", InvestmentProjectID: "01JDUANCUAXAA000000000000",
		PaymentDate: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC),
		Amount:      30_000_000, Description: "Thanh toán đợt 3", Counterparty: "Công ty ABC",
		Status: domain.VoucherEntered, EnteredByID: writerStaffCode,
	}}
	checker := &fakeCatalogueChecker{}
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))

	mux := http.NewServeMux()
	Register(mux, Deps{
		Checker:                   checker,
		CapitalPlanCategories:     sampleCategories(),
		CapitalPlanCategoryWriter: &fakeCategoryWriter{},
		InvestmentProjects:        sampleInvestmentProjects(),
		InvestmentProjectWriter:   &fakeInvestmentProjectWriter{},
		DisbursementVoucherWriter: writer,
		DelayThresholds:           sampleThresholds(),
		// Present so Register accepts the Deps; never called from this file. See routes_test.go.
		Budget:         &fakeBudgetReader{},
		BudgetWriter:   &fakeBudgetWriter{},
		AuditLog:       &auditLogFake{},
		SystemMessages: &systemMessagesFake{},
		Now:            func() time.Time { return atElapsed7096 },
		Log:            quiet,

		// The catalogue's Excel import — never called here; own suite in internal/http/catalogue_import_test.go.
		CapitalPlanCategoryImports: &catalogueImportFake{},
	})

	var h http.Handler = mux
	h = injectPrincipal(h)
	h = idem.Middleware(idemStore, quiet)(h)
	h = httpx.TenantMiddleware(sampleDirectory())(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)

	return &voucherServer{h: h, writer: writer, checker: checker}
}

func (m *voucherServer) grant(tenantID tenant.ID, perm ...authz.Perm) {
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

// call ALWAYS SENDS AN Idempotency-Key. POST /api/v1/disbursements declares idem.Required, which
// refuses a request without the header BEFORE it reaches the handler — so a harness that omitted it
// would turn every POST assertion into an assertion about the header.
func (m *voucherServer) call(t *testing.T, method, host, path string,
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
	r.Header.Set(idem.Header, "01JIDEMPOTENCYKEYCUATEST")
	if p != nil {
		r = r.WithContext(context.WithValue(r.Context(), principalCtxKey{}, *p))
	}
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)
	return w
}

const (
	voucherPath     = "/api/v1/disbursements"
	sampleVoucherID = "01JCHUNGTUDANGCO000000000"
)

func voucherPathFor(suffix string) string {
	return voucherPath + "/" + sampleVoucherID + suffix
}

const createVoucherBody = `{"project_id":"01JDUANCUAXAA000000000000","payment_date":"2026-09-07",` +
	`"amount":30000000,"description":"Thanh toán đợt 3","counterparty":"Công ty ABC"}`

// voucherWriteRoute is one of the six write routes, with the key the specification assigns to it.
type voucherWriteRoute struct {
	name   string
	method string
	path   string
	body   string
	perm   authz.Perm
	ok     int
	count  func(g *fakeVoucherWriter) int
}

// sixVoucherWriteRoutes — the four permission cases are asserted on ALL SIX rather than on whichever
// one was written first, and the `perm` column is what makes the suite able to tell the two keys
// apart: a route that quietly asked for `budget.update` where the specification says
// `budget.confirm` would let a clerk freeze a figure.
func sixVoucherWriteRoutes() []voucherWriteRoute {
	return []voucherWriteRoute{
		{"POST", http.MethodPost, voucherPath, createVoucherBody,
			"budget.update", http.StatusCreated, func(g *fakeVoucherWriter) int { return g.createCalls }},
		{"PATCH", http.MethodPatch, voucherPathFor(""), `{"description":"Thanh toán đợt 4"}`,
			"budget.update", http.StatusOK, func(g *fakeVoucherWriter) int { return g.updateCalls }},
		{"DELETE", http.MethodDelete, voucherPathFor(""), `{"reason":"nhập trùng"}`,
			"budget.confirm", http.StatusNoContent, func(g *fakeVoucherWriter) int { return g.removeCalls }},
		{"POST confirmation", http.MethodPost, voucherPathFor("/confirmation"), "",
			"budget.confirm", http.StatusOK, func(g *fakeVoucherWriter) int { return g.confirmCalls }},
		{"POST lockout", http.MethodPost, voucherPathFor("/lockout"), "",
			"budget.confirm", http.StatusOK, func(g *fakeVoucherWriter) int { return g.lockCalls }},
		{"DELETE lockout", http.MethodDelete, voucherPathFor("/lockout"),
			`{"reason":"kho bạc trả lại chứng từ"}`,
			"budget.confirm", http.StatusOK, func(g *fakeVoucherWriter) int { return g.unlockCalls }},
	}
}

// --- (2) the permission keys themselves ------------------------------------------------------------

func TestVoucherPermissionKeysMatchPermissionTable(t *testing.T) {
	// THE KEY EACH ROUTE ACTUALLY ASKS FOR, COMPARED AGAINST A LITERAL — and that is the whole point.
	//
	// Every other assertion in this file grants the key and then expects it to be accepted, so
	// changing a route's key to ANY string leaves them all green: the fake checker grants whatever it
	// was handed. That is not hypothetical — this repository carried three invented keys
	// (`finance.read`, `map.read`, `document.approve`) through several sessions with every suite
	// green, because a route holding a key the `quyen` table lacks answers 403 to EVERY account,
	// forever, and nothing says so (rule 5, invariant 3c).
	//
	// `budget.update` and `budget.confirm` are seeded at
	// service-identity/migrations/0001_init.sql:282-284 and assigned to these exact acts at
	// docs/ui-ux/06-giai-ngan.md:202. tools/check_quyen.py scans the whole repository against that
	// table on every `make check` and is the guard a fixture cannot fool; this is the cheap half that
	// turns red in `go test` too.
	for _, tc := range sixVoucherWriteRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			m := newVoucherServer(t)
			m.call(t, tc.method, hostA, tc.path, writerPrincipal(tenantA), tc.body)

			if got := m.checker.lastAskedKey(); got != tc.perm {
				t.Fatalf("tuyến hỏi khoá %q, muốn %q — một khoá bảng `quyen` không có là một tuyến "+
					"trả 403 với MỌI tài khoản, mãi mãi, và không phép kiểm nào đỏ", got, tc.perm)
			}
		})
	}
}

// --- (1) the four cases of rule 5, invariant 7 -------------------------------------------------------

func TestVoucherWrites_401WithoutSession(t *testing.T) {
	for _, tc := range sixVoucherWriteRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			m := newVoucherServer(t)
			m.grant(tenantA, "budget.update", "budget.confirm") // granted, and still refused
			wantStatus(t, m.call(t, tc.method, hostA, tc.path, nil, tc.body), http.StatusUnauthorized)
			if m.writer.totalCalls() != 0 {
				t.Error("chưa đăng nhập mà use case ghi đã chạy")
			}
		})
	}
}

func TestVoucherWrites_403WrongPermission(t *testing.T) {
	// A signed-in account of the right commune holding a DIFFERENT permission. `budget.read` is
	// deliberately a REAL key of this very subsystem — the failure being guarded against is not "an
	// account with nothing", it is somebody who may LOOK at the commune's disbursement figures being
	// able to change them.
	for _, tc := range sixVoucherWriteRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			m := newVoucherServer(t)
			m.grant(tenantA, "budget.read")

			wantStatus(t, m.call(t, tc.method, hostA, tc.path, writerPrincipal(tenantA), tc.body),
				http.StatusForbidden)
			if m.writer.totalCalls() != 0 {
				t.Error("sai quyền mà use case ghi vẫn chạy")
			}
		})
	}
}

func TestVoucherWrites_403EntererCannotLock(t *testing.T) {
	// THE SPLIT THE SPECIFICATION DRAWS, ASSERTED AS A SEPARATE CASE because it is the one an
	// ordinary "grant the module's permissions" fixture would paper over: an accountant holding
	// `budget.update` may enter and correct vouchers and may NOT confirm, freeze or remove one
	// (06-giai-ngan.md:202). Freezing a figure and taking one out of a reported total are acts of
	// a different weight, and this is where that is enforced.
	m := newVoucherServer(t)
	m.grant(tenantA, "budget.update")

	for _, tc := range sixVoucherWriteRoutes() {
		if tc.perm != "budget.confirm" {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			wantStatus(t, m.call(t, tc.method, hostA, tc.path, writerPrincipal(tenantA), tc.body),
				http.StatusForbidden)
		})
	}
	if m.writer.totalCalls() != 0 {
		t.Errorf("use case chạy %d lần với tài khoản chỉ có `budget.update`", m.writer.totalCalls())
	}
}

func TestVoucherWrites_403RightPermissionWrongCommune(t *testing.T) {
	// THE CASE THAT IS EASIEST TO FAKE AND HARDEST TO GET RIGHT, so read what it actually sets up.
	//
	// The account belongs to commune B and is signed in AT COMMUNE B: nothing about the request is
	// malformed, and authz's own commune comparison passes. What is wrong is the GRANT — the right
	// to write vouchers was given in commune A. A checker that ignored the commune would answer yes
	// here, and commune A's accountant would be recording payments in commune B's register.
	//
	// That is rule 5, invariant 3 in one sentence: a permission missing its commune is cross-commune
	// escalation, not a lesser bug. And it answers 403 rather than 401 because the session is
	// perfectly valid — it is the authority that is absent.
	for _, tc := range sixVoucherWriteRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			m := newVoucherServer(t)
			m.grant(tenantA, "budget.update", "budget.confirm")

			wantStatus(t, m.call(t, tc.method, hostB, tc.path, writerPrincipal(tenantB), tc.body),
				http.StatusForbidden)
			if m.writer.totalCalls() != 0 {
				t.Error("quyền cấp ở xã khác mà vẫn ghi được vào xã này")
			}
		})
	}
}

func TestVoucherWrites_401SessionOfOtherCommune(t *testing.T) {
	// The other shape of "wrong commune": a principal issued by commune A presented at commune B's
	// domain. authz.RequirePermission compares the commune BEFORE the permission and answers 401,
	// not 403 — a browser does not send a cookie across hosts, so this is never an ordinary user
	// error. Asserted so that nobody "corrects" it to 403 and turns a deliberate probe into
	// something that reads like a permissions problem.
	for _, tc := range sixVoucherWriteRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			m := newVoucherServer(t)
			m.grant(tenantA, "budget.update", "budget.confirm")
			m.grant(tenantB, "budget.update", "budget.confirm")

			wantStatus(t, m.call(t, tc.method, hostB, tc.path, writerPrincipal(tenantA), tc.body),
				http.StatusUnauthorized)
			if m.writer.totalCalls() != 0 {
				t.Error("phiên của xã khác mà vẫn ghi được")
			}
		})
	}
}

func TestVoucherWrites_RightPermissionRightCommune(t *testing.T) {
	for _, tc := range sixVoucherWriteRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			m := newVoucherServer(t)
			m.grant(tenantA, "budget.update", "budget.confirm")

			wantStatus(t, m.call(t, tc.method, hostA, tc.path, writerPrincipal(tenantA), tc.body), tc.ok)
			if got := tc.count(m.writer); got != 1 {
				t.Fatalf("use case chạy %d lần, muốn 1", got)
			}
			// Rule 6, invariant 2: WHO, and IN WHICH COMMUNE. Both have to reach the layer that
			// writes the entry, or the trail cannot answer the only question it exists for.
			if m.writer.lastTenant != tenantA {
				t.Errorf("use case chạy trong xã %q, muốn %q", m.writer.lastTenant, tenantA)
			}
			// THE TRAIL CARRIES THE BUSINESS CODE, AND THE SECOND CHECK NAMES THE WRONG VALUE
			// OUTRIGHT. Asserting only "equals the code" would stay green the day somebody made the
			// two constants the same string.
			if m.writer.lastActor.ID != writerStaffCode || m.writer.lastActor.Kind != "staff" {
				t.Errorf("chủ thể vết = %+v, muốn MÃ CÁN BỘ %q", m.writer.lastActor, writerStaffCode)
			}
			if m.writer.lastActor.ID == writerInternalID {
				t.Errorf("vết mang ID NỘI BỘ %q — luật 6 bất biến 8 đòi mã nghiệp vụ", writerInternalID)
			}
			// The IP is taken from this process's own socket, never from X-Forwarded-For.
			if m.writer.lastActor.IP != "10.0.0.7" {
				t.Errorf("IP trong vết = %q, muốn 10.0.0.7", m.writer.lastActor.IP)
			}
		})
	}
}

// --- (3) `status` and `project_id` are refused, not ignored ------------------------------------------

func TestCreateVoucherRefusesStatusFromClient(t *testing.T) {
	// THE ONE THAT MUST NEVER GO GREEN BY ACCIDENT. The state decides whether a figure is frozen and
	// who signed it. A client that could set it could create a voucher already `Đã khoá` — nobody
	// confirmed, nobody can edit it, and its money is inside the commune's "đã giải ngân".
	//
	// BOTH DIRECTIONS ARE SENT. `da-khoa` is the one that would buy something; `ke-toan-nhap` is the
	// one that looks harmless and is refused just as firmly, because the rule is "not from the
	// client", not "not that value".
	for name, body := range map[string]string{
		"POST status da-khoa": `{"project_id":"p","payment_date":"2026-09-07","amount":1,` +
			`"description":"x","status":"da-khoa"}`,
		"POST status ke-toan-nhap": `{"project_id":"p","payment_date":"2026-09-07","amount":1,` +
			`"description":"x","status":"ke-toan-nhap"}`,
	} {
		t.Run(name, func(t *testing.T) {
			m := newVoucherServer(t)
			m.grant(tenantA, "budget.update")

			w := m.call(t, http.MethodPost, hostA, voucherPath, writerPrincipal(tenantA), body)
			wantStatus(t, w, http.StatusBadRequest)
			if m.writer.createCalls != 0 {
				t.Error("thân mang `status` mà use case vẫn chạy")
			}
		})
	}
}

func TestUpdateVoucherRefusesInvestmentProjectChange(t *testing.T) {
	// Moving a voucher between projects moves money between two totals that have already been read
	// off a screen, and it leaves ONE entry that reads as an ordinary edit. The operation is to
	// remove it with a reason and enter it again — two entries, each naming the project it belongs to.
	m := newVoucherServer(t)
	m.grant(tenantA, "budget.update")

	w := m.call(t, http.MethodPatch, hostA, voucherPathFor(""), writerPrincipal(tenantA),
		`{"project_id":"01JDUANKHAC00000000000000"}`)
	wantStatus(t, w, http.StatusBadRequest)
	if m.writer.updateCalls != 0 {
		t.Error("thân mang `project_id` mà use case vẫn chạy")
	}
}

// --- (4) a refusal reaches the client as a sentence somebody can act on ------------------------------

func TestUpdateLockedVoucher_VietnameseSentenceReachesClient(t *testing.T) {
	// §13 rule 3, ALL THE WAY OUT TO THE BODY. The `chung_tu_da_khoa` trigger refuses the same UPDATE
	// underneath with an English exception naming a constraint; what an accountant in a commune needs
	// is the sentence naming the operation and the way out. If that sentence stops arriving, the
	// screen simply looks broken: the row is right there and the Save button did nothing.
	//
	// 409 AND NOT 403: the caller HOLDS `budget.update`. What refuses is the ROW.
	m := newVoucherServer(t)
	m.grant(tenantA, "budget.update")
	m.writer.err = domain.ErrVoucherLocked

	w := m.call(t, http.MethodPatch, hostA, voucherPathFor(""), writerPrincipal(tenantA),
		`{"amount":12000000}`)
	wantStatus(t, w, http.StatusConflict)

	e := errorBody(t, w)
	for _, frag := range []string{"đã khoá", "mở khoá", "budget.confirm"} {
		if !strings.Contains(e.Message, frag) {
			t.Errorf("thông báo thiếu %q — người dùng không biết phải làm gì: %q", frag, e.Message)
		}
	}
	// AND NOTHING FROM THE DRIVER. A PostgreSQL exception carries the constraint name and English
	// wording, neither of which an accountant can act on (rule 3, forbidden #3 on the same path).
	if strings.Contains(strings.ToLower(e.Message), "constraint") ||
		strings.Contains(e.Message, "chung_tu_giai_ngan_") {
		t.Errorf("câu lỗi của CSDL lọt ra client: %q", e.Message)
	}
}

func TestUnlock_NoReasonIsRefused400(t *testing.T) {
	// The use case refuses a blank reason before the transaction opens (proved in internal/app); this
	// asserts the REFUSAL REACHES THE CALLER as a 400 naming what to add, rather than as a 500.
	m := newVoucherServer(t)
	m.grant(tenantA, "budget.confirm")
	m.writer.err = domain.ErrUnlockReasonMissing

	w := m.call(t, http.MethodDelete, hostA, voucherPathFor("/lockout"), writerPrincipal(tenantA), `{}`)
	wantStatus(t, w, http.StatusBadRequest)
	if !strings.Contains(errorBody(t, w).Message, "lý do") {
		t.Errorf("thông báo không nói thiếu lý do: %q", errorBody(t, w).Message)
	}
	// THE REASON IT SENT IS THE EMPTY ONE, not something the handler invented on the way through.
	if m.writer.lastReason != "" {
		t.Errorf("handler tự bịa lý do %q", m.writer.lastReason)
	}
}

func TestUnlock_LockerReopening_409Not403(t *testing.T) {
	// 409 AND NOT 403, and the distinction is the reason this has its own test. The caller HOLDS
	// `budget.confirm` and is allowed to unlock vouchers; what is refused is this PERSON against THIS
	// row. A 403 would send them to the Phân quyền screen to be granted a permission they already
	// have, where nothing they could do would help.
	m := newVoucherServer(t)
	m.grant(tenantA, "budget.confirm")
	m.writer.err = domain.ErrSelfUnlockAfterOwnLock

	w := m.call(t, http.MethodDelete, hostA, voucherPathFor("/lockout"), writerPrincipal(tenantA),
		`{"reason":"sai số tiền"}`)
	wantStatus(t, w, http.StatusConflict)
	if !strings.Contains(errorBody(t, w).Message, "cán bộ khác") {
		t.Errorf("thông báo không nói cần người khác: %q", errorBody(t, w).Message)
	}
}

func TestCreateVoucher_NonPositiveAmountIs400(t *testing.T) {
	m := newVoucherServer(t)
	m.grant(tenantA, "budget.update")
	m.writer.err = domain.ErrAmountNotPositive

	w := m.call(t, http.MethodPost, hostA, voucherPath, writerPrincipal(tenantA),
		`{"project_id":"p","payment_date":"2026-09-07","amount":0,"description":"x"}`)
	wantStatus(t, w, http.StatusBadRequest)
}

func TestCreateVoucher_MissingInvestmentProjectIs404(t *testing.T) {
	// A project of another commune is indistinguishable from one that does not exist, because the
	// query cannot reach it at all (rule 4, forbidden #2, applied between communes).
	m := newVoucherServer(t)
	m.grant(tenantA, "budget.update")
	m.writer.err = fistore.ErrVoucherInvestmentProjectNotFound

	w := m.call(t, http.MethodPost, hostA, voucherPath, writerPrincipal(tenantA), createVoucherBody)
	wantStatus(t, w, http.StatusNotFound)
}

// --- (5) the body the client actually gets ------------------------------------------------------------

func TestCreateVoucher_201ReturnsWrittenVoucher(t *testing.T) {
	m := newVoucherServer(t)
	m.grant(tenantA, "budget.update")

	w := m.call(t, http.MethodPost, hostA, voucherPath, writerPrincipal(tenantA), createVoucherBody)
	wantStatus(t, w, http.StatusCreated)

	var result chungTuRa
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	if result.Amount != 30_000_000 {
		t.Errorf("amount = %d, muốn 30000000 — số ĐỒNG, không phải chuỗi đã định dạng", result.Amount)
	}
	if result.Status != string(domain.VoucherEntered) {
		t.Errorf("status = %q, muốn %q", result.Status, domain.VoucherEntered)
	}
	if result.PaymentDate != "2026-09-07" {
		t.Errorf("payment_date = %q, muốn 2026-09-07", result.PaymentDate)
	}
	// `unlock_count` MUST SURVIVE AT ZERO. A voucher nobody has reopened has been reopened zero
	// times, which is a fact rather than an absence — a field that vanished would make a client
	// unable to tell that from "this server does not report it".
	if !strings.Contains(w.Body.String(), `"unlock_count":0`) {
		t.Errorf("thiếu `unlock_count` khi bằng 0: %s", w.Body.String())
	}
	// The use case received exactly what the body said, parsed — not a value the handler invented.
	if m.writer.lastCreate.Amount != 30_000_000 || m.writer.lastCreate.Description != "Thanh toán đợt 3" {
		t.Errorf("use case nhận %+v", m.writer.lastCreate)
	}
	if !m.writer.lastCreate.PaymentDate.Equal(time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("ngày chi = %v", m.writer.lastCreate.PaymentDate)
	}
}

// --- the funding source of a payment (§8.2's `NGUỒN VỐN` column, migration 0007) ---------------------
//
// WHAT THIS LAYER CAN PROVE AND THE app LAYER CANNOT: that the field survives the JSON boundary in
// both directions, and above all that PATCH's THREE ANSWERS stay three. The use case distinguishes
// "leave alone" from "detach" by a nil pointer; a handler that dereferenced it, or that dropped the
// field, would collapse the two at the last layer able to tell them apart — and every test in
// internal/app would stay green.

const fundingSourceIDHTTP = "01JNGUONVONNGANSACHXA0000"

func TestCreateVoucher_FundingSourceGoesThroughBodyAndBackInResponse(t *testing.T) {
	m := newVoucherServer(t)
	m.grant(tenantA, "budget.update")
	m.writer.result.FundingSourceID = fundingSourceIDHTTP

	w := m.call(t, http.MethodPost, hostA, voucherPath, writerPrincipal(tenantA),
		`{"project_id":"01JDUANCUAXAA000000000000","payment_date":"2026-09-07","amount":30000000,`+
			`"description":"Thanh toán đợt 3","funding_source_id":"`+fundingSourceIDHTTP+`"}`)
	wantStatus(t, w, http.StatusCreated)

	if m.writer.lastCreate.FundingSourceID != fundingSourceIDHTTP {
		t.Errorf("use case nhận nguồn vốn %q, muốn %q", m.writer.lastCreate.FundingSourceID, fundingSourceIDHTTP)
	}
	var result chungTuRa
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	if result.FundingSourceID != fundingSourceIDHTTP {
		t.Errorf("`funding_source_id` = %q, muốn %q", result.FundingSourceID, fundingSourceIDHTTP)
	}
}

func TestCreateVoucher_NoSourceIsStillAccepted(t *testing.T) {
	// §13 rule 6 IS A STATE, NOT A GAP. A voucher with no source counts toward "đã giải ngân" and is
	// reported at §6 as "đã chi nhưng chưa ghi rút từ nguồn nào". A route that required the field would
	// refuse a payment that has ALREADY LEFT THE COMMUNE'S ACCOUNT — and the accountant would have
	// nothing to type.
	m := newVoucherServer(t)
	m.grant(tenantA, "budget.update")

	w := m.call(t, http.MethodPost, hostA, voucherPath, writerPrincipal(tenantA), createVoucherBody)
	wantStatus(t, w, http.StatusCreated)
	if m.writer.lastCreate.FundingSourceID != "" {
		t.Errorf("handler tự bịa nguồn vốn %q", m.writer.lastCreate.FundingSourceID)
	}
	// AND THE REPLY DOES NOT CARRY THE FIELD AT ALL. `—` is what §8.2's own sample rows show in that
	// column; `omitempty` is what makes the body say so.
	if strings.Contains(w.Body.String(), `"funding_source_id"`) {
		t.Errorf("chứng từ chưa gắn nguồn mà đáp án vẫn mang `funding_source_id`: %s", w.Body.String())
	}
}

func TestUpdateVoucher_ThreeFundingSourceAnswersStayDistinct(t *testing.T) {
	// THE THREE ANSWERS, EACH SENT AS THE CLIENT WOULD SEND IT:
	//
	//	field absent   nil   -> the use case leaves the voucher's source exactly as it is
	//	`""`           ptr   -> DETACH, back into §6's "đã chi nhưng chưa ghi rút từ nguồn nào"
	//	an id          ptr   -> attach
	//
	// A HANDLER THAT DROPPED THE MIDDLE ONE would leave a commune unable to say "not this source"
	// about a payment it attributed wrongly — short of removing the voucher entirely, which is a
	// different act with a different permission and a different entry in the ledger.
	for _, tc := range []struct {
		name       string
		body       string
		hasPointer bool
		value      string
	}{
		{"không nhắc tới thì để nguyên", `{"description":"Thanh toán đợt 4"}`, false, ""},
		{"chuỗi rỗng là GỠ khỏi nguồn", `{"funding_source_id":""}`, true, ""},
		{"mã thật là gắn vào nguồn", `{"funding_source_id":"` + fundingSourceIDHTTP + `"}`, true,
			fundingSourceIDHTTP},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newVoucherServer(t)
			m.grant(tenantA, "budget.update")

			wantStatus(t, m.call(t, http.MethodPatch, hostA, voucherPathFor(""), writerPrincipal(tenantA), tc.body),
				http.StatusOK)

			got := m.writer.lastUpdate.FundingSourceID
			if !tc.hasPointer {
				if got != nil {
					t.Fatalf("thân không nhắc `funding_source_id` mà use case nhận con trỏ tới %q — "+
						"\"để nguyên\" và \"gỡ khỏi nguồn\" đã bị gộp làm một", *got)
				}
				return
			}
			if got == nil {
				t.Fatalf("thân có `funding_source_id` mà use case nhận nil")
			}
			if *got != tc.value {
				t.Fatalf("use case nhận %q, muốn %q", *got, tc.value)
			}
		})
	}
}

func TestVoucher_FundingSourceNotInTenantIs404(t *testing.T) {
	// 404 ON BOTH ROUTES THAT CARRY THE FIELD, and the same answer for "no such source" and "another
	// commune's source": the query binds tenant_id = $1, so this service cannot tell them apart and
	// must not appear to (rule 1). A different status for the two would let a caller probe which
	// commune holds which sources.
	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"POST", http.MethodPost, voucherPath, createVoucherBody},
		{"PATCH", http.MethodPatch, voucherPathFor(""),
			`{"funding_source_id":"01JNGUONVONCUAXAKHAC00000"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newVoucherServer(t)
			m.grant(tenantA, "budget.update")
			m.writer.err = fistore.ErrVoucherFundingSourceNotFound

			w := m.call(t, tc.method, hostA, tc.path, writerPrincipal(tenantA), tc.body)
			wantStatus(t, w, http.StatusNotFound)
			if e := errorBody(t, w); !strings.Contains(e.Message, "nguồn vốn") {
				t.Errorf("thông báo không nói về nguồn vốn: %q", e.Message)
			}
		})
	}
}

func TestCreateVoucher_NoRedisIs503_FailClosed(t *testing.T) {
	// THE DECLARATION, EXERCISED. `POST /api/v1/disbursements` is the only route in this service
	// that declares idem.Required(idem.DongKhiHong), and the reason is written at the route: there
	// is NO uniqueness constraint that could tell a double-submitted form from two genuine payments
	// to the same company, on the same day, for the same amount. So with no cache reachable, a double
	// click is a voucher counted twice inside "đã giải ngân" — money on a figure a decision quotes.
	//
	// 503 IS THE CORRECT ANSWER and this test is what stops somebody "fixing" it to MoKhiHong to make
	// local development quieter. The permission is granted and the body is valid: nothing else is
	// wrong with this request.
	m := newVoucherServerWith(t, nil)
	m.grant(tenantA, "budget.update")

	wantStatus(t, m.call(t, http.MethodPost, hostA, voucherPath, writerPrincipal(tenantA), createVoucherBody),
		http.StatusServiceUnavailable)
	if m.writer.createCalls != 0 {
		t.Error("không bảo đảm được chống trùng mà vẫn ghi chứng từ")
	}
}

func TestCreateVoucher_MissingIdempotencyKeyIsRefused(t *testing.T) {
	// The header is REQUIRED on this route, so a client that never sends one is told at once rather
	// than discovering it the day a retry duplicates a payment.
	m := newVoucherServer(t)
	m.grant(tenantA, "budget.update")

	r := httptest.NewRequest(http.MethodPost, "https://"+hostA+voucherPath,
		strings.NewReader(createVoucherBody))
	r.Host = hostA
	r.RemoteAddr = "10.0.0.7:51000"
	r.Header.Set("Content-Type", "application/json")
	r = r.WithContext(context.WithValue(r.Context(), principalCtxKey{}, *writerPrincipal(tenantA)))
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)

	if w.Code == http.StatusCreated {
		t.Fatalf("thiếu %s mà vẫn ghi được chứng từ", idem.Header)
	}
	if m.writer.createCalls != 0 {
		t.Error("thiếu khoá chống trùng mà use case vẫn chạy")
	}
}

func TestCreateVoucher_MalformedDateIs400(t *testing.T) {
	m := newVoucherServer(t)
	m.grant(tenantA, "budget.update")

	w := m.call(t, http.MethodPost, hostA, voucherPath, writerPrincipal(tenantA),
		`{"project_id":"p","payment_date":"07/09/2026","amount":1,"description":"x"}`)
	wantStatus(t, w, http.StatusBadRequest)
	if m.writer.createCalls != 0 {
		t.Error("ngày sai định dạng mà use case vẫn chạy")
	}
}

func TestRemoveVoucher_ReasonComesFromBodyNotQueryString(t *testing.T) {
	// The reason is mandatory (rule 7, invariant 1) and travels in the BODY: a query string would
	// put free text about a public authority's spending into every access log and proxy cache.
	m := newVoucherServer(t)
	m.grant(tenantA, "budget.confirm")

	const reason = "kế toán nhập trùng hai lần"
	wantStatus(t, m.call(t, http.MethodDelete, hostA, voucherPathFor(""), writerPrincipal(tenantA),
		`{"reason":"`+reason+`"}`), http.StatusNoContent)
	if m.writer.lastReason != reason {
		t.Errorf("lý do tới use case = %q, muốn %q", m.writer.lastReason, reason)
	}
	if m.writer.lastID != sampleVoucherID {
		t.Errorf("id tới use case = %q, muốn %q", m.writer.lastID, sampleVoucherID)
	}
}

// --- the threshold now comes from the commune's own configuration -------------------------------------

func TestListInvestmentProjects_ThresholdReadFromCommuneSettings(t *testing.T) {
	// THE FIX MIGRATION 0005 EXISTS FOR. Until it, `nguong_canh_bao_cham` was a constant in the
	// vendor's source deciding whether a commune's KPI card reads "29 dự án chậm" or "0" — rule 1,
	// invariant 10 read backwards, and open question #31.
	//
	// AND THE REPLY SAYS WHO CHOSE IT. A figure with no provenance cannot tell a commune that has
	// chosen 10 points from one that has chosen nothing, and only the second needs to be told.
	m := newVoucherServer(t)
	m.grant(tenantA, "budget.read")

	w := m.call(t, http.MethodGet, hostA, "/api/v1/investment-projects?year=2026", writerPrincipal(tenantA), "")
	wantStatus(t, w, http.StatusOK)

	var result danhSachDuAnRa
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	if result.DelayThreshold != int64(15*domain.OnePercent) {
		t.Errorf("delay_threshold = %d, muốn 1500 — ngưỡng xã A tự đặt, không phải mặc định 1000",
			result.DelayThreshold)
	}
	if result.DelayThresholdSource != string(domain.ThresholdFromCommune) {
		t.Errorf("delay_threshold_source = %q, muốn %q", result.DelayThresholdSource, domain.ThresholdFromCommune)
	}
}

func TestListInvestmentProjects_UnsetCommuneSaysDefault(t *testing.T) {
	// THE ORDINARY STATE OF EVERY COMMUNE TODAY, because no screen writes `cau_hinh_giai_ngan` yet.
	// The honest answer is the software's 10 points LABELLED AS THE SOFTWARE'S — 0004's header named
	// the failure otherwise: an API that "reads as 'already configurable' while every commune
	// silently gets the default".
	m := newVoucherServer(t)
	m.grant(tenantB, "budget.read")

	w := m.call(t, http.MethodGet, hostB, "/api/v1/investment-projects?year=2026", writerPrincipal(tenantB), "")
	wantStatus(t, w, http.StatusOK)

	var result danhSachDuAnRa
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	if result.DelayThreshold != int64(domain.DefaultDelayThreshold) {
		t.Errorf("delay_threshold = %d, muốn %d", result.DelayThreshold, domain.DefaultDelayThreshold)
	}
	if result.DelayThresholdSource != string(domain.ThresholdFromDefault) {
		t.Errorf("delay_threshold_source = %q, muốn %q — xã B chưa khai gì",
			result.DelayThresholdSource, domain.ThresholdFromDefault)
	}
}

func TestPATCHReturnsStatusFromUseCase_resetToEnteredShowsInBody(t *testing.T) {
	// THE RULE ITSELF IS PROVED IN internal/app (TestUpdateConfirmedVoucherResetsToEnteredAndClearsConfirmer),
	// where the SQL and the transaction are. WHAT IS PROVED HERE is the half that layer cannot see:
	// the state and the (now empty) confirmer reach the CLIENT.
	//
	// It matters because the screen draws its buttons from this body. A handler that kept sending
	// `da-xac-nhan` — or that let `confirmed_by` survive through `omitempty` on the wrong field —
	// would leave the accountant looking at a voucher the server considers a draft while the screen
	// offers `Khoá`, and every test in internal/app would stay green.
	m := newVoucherServer(t)
	m.grant(tenantA, "budget.update")
	m.writer.result = domain.DisbursementVoucher{
		ID: sampleVoucherID, InvestmentProjectID: "01JDUANCUAXAA000000000000",
		PaymentDate: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC),
		Amount:      31_000_000, Description: "Thanh toán đợt 3",
		// What app.DisbursementVoucherService.UpdateVoucher returns for a voucher that WAS `Đã xác nhận` and has just been corrected.
		Status: domain.VoucherEntered, EnteredByID: writerStaffCode, ConfirmedByID: "",
	}

	w := m.call(t, http.MethodPatch, hostA, voucherPathFor(""), writerPrincipal(tenantA), `{"amount":31000000}`)
	wantStatus(t, w, http.StatusOK)

	var result chungTuRa
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("thân không phải JSON: %v", err)
	}
	if result.Status != string(domain.VoucherEntered) {
		t.Fatalf("`status` = %q, muốn %q — sửa một chứng từ đã xác nhận thì nó VỀ NHÁP",
			result.Status, domain.VoucherEntered)
	}
	if result.ConfirmedBy != "" {
		t.Fatalf("`confirmed_by` = %q, muốn rỗng — dòng không được khai một lãnh đạo đã duyệt "+
			"những con số họ chưa từng thấy", result.ConfirmedBy)
	}
	if strings.Contains(w.Body.String(), "da-xac-nhan") {
		t.Fatalf("thân vẫn mang `da-xac-nhan`: %s", w.Body.String())
	}
}
