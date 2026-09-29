package http

import (
	"context"
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
	"github.com/vihat/vigov/service-documents/internal/app"
	"github.com/vihat/vigov/service-documents/internal/domain"
)

// The harness for this package's route tests. Nothing here is a PostgreSQL or a Redis: the
// properties under test are ordering, isolation and refusal, and a test that needs infrastructure
// is a test that stops being run.

// --- fixtures -----------------------------------------------------------------------------------

const (
	hostA = "xa-a.example.gov.vn"
	hostB = "xa-b.example.gov.vn"

	staffID   = "nd-01JINTERNALIDCUACANBO"
	staffCode = "CB-00123"
)

var (
	tenantA = tenant.ID("01JA" + strings.Repeat("A", 22))
	tenantB = tenant.ID("01JB" + strings.Repeat("B", 22))
)

// staffOf builds the principal the authentication edge would have built for a signed-in member of
// staff of that commune.
//
// Roles IS LEFT EMPTY, exactly as service-identity's XacThuc leaves it: permissions are read from
// the database on every request, never carried on the principal, so a role change takes effect on
// the next request rather than when the session ends.
func staffOf(tenantID tenant.ID) *authz.Principal {
	return &authz.Principal{ID: staffID, Ma: staffCode, Kind: "staff", TenantID: tenantID}
}

// --- fakes --------------------------------------------------------------------------------------

type fakeDirectory map[string]tenant.Tenant

func (m fakeDirectory) ByHost(_ context.Context, host string) (tenant.Tenant, bool) {
	t, ok := m[host]
	return t, ok
}

// sampleDirectory is the platform registry for the two test communes. A NEW MAP PER CALL, so a test
// that rewrites one copy cannot change the other.
func sampleDirectory() fakeDirectory {
	return fakeDirectory{
		hostA: {ID: tenantA, Host: hostA, Name: "Xã Thăng Bình", Province: "Thành phố Đà Nẵng", Active: true},
		hostB: {ID: tenantB, Host: hostB, Name: "Xã Bình Dương", Active: true},
	}
}

// fakeDocumentTypes is the document-type catalogue, KEYED BY COMMUNE, reading the commune from the
// context exactly as *store.Scoped reads it. Keyed any other way — by nothing, or by a field set
// at construction — the isolation test below would pass while proving nothing at all.
type fakeDocumentTypes struct {
	byTenant map[tenant.ID][]domain.DocumentType
	err      error
	calls    int
}

func (l *fakeDocumentTypes) List(ctx context.Context) ([]domain.DocumentType, error) {
	l.calls++
	if l.err != nil {
		return nil, l.err
	}
	return l.byTenant[tenant.MustFrom(ctx)], nil
}

// sampleDocumentTypes gives commune A two types and commune B one with a DIFFERENT code and label.
// Two communes whose catalogues were named the same could not show a leak.
//
// The order is the one the store returns (ORDER BY thu_tu, nhan) and is deliberately NOT
// alphabetical by label ("Quyết định" before "Công văn"): any re-sort in the handler turns the
// order test red.
//
// Commune A's second row is `dang_dung: false` and its first is the default. A fixture where every
// row looked the same could not tell "the flag is read" from "the flag is always true".
func sampleDocumentTypes() *fakeDocumentTypes {
	return &fakeDocumentTypes{byTenant: map[tenant.ID][]domain.DocumentType{
		tenantA: {
			{ID: "lvb-001", Code: "quyet-dinh", Label: "Quyết định", IsActive: true, IsDefault: true},
			{ID: "lvb-002", Code: "cong-van", Label: "Công văn", IsActive: false},
		},
		tenantB: {
			{ID: "lvb-b-001", Code: "to-trinh", Label: "Tờ trình xã B", IsActive: true},
		},
	}}
}

// fakeChecker answers permissions and COUNTS the times it was asked.
//
// BOTH HALVES MATTER. The default set is empty, which is what makes "an account holding nothing
// still gets 200 on the read route" a real assertion; the count is what shows that route consults
// no permission at all, rather than happening to hold one.
//
// IT COMPARES THE COMMUNE, and that is not decoration. A checker that ignored the commune would
// grant one commune's roles inside another (rule 5, invariant 3), and the "right permission, wrong
// commune" case of invariant 7 would pass while proving nothing. The real one —
// service-identity/internal/store.Checker — reads the grants scoped to the commune in the context;
// this reproduces that one property and nothing else.
// THE GRANTS ARE KEYED BY COMMUNE, and that is the whole reason this fake is not a bool.
//
// Rule 5, invariant 7 asks for a "right permission, WRONG COMMUNE" case. With a flat permission set
// that case cannot be expressed at all: whoever holds the key holds it everywhere, so the test
// would assert something else and go green. Keyed by commune, a grant made in commune A is silent
// in commune B — which is exactly what cross-commune escalation looks like, and what
// service-identity/internal/store.Checker prevents by reading the grants scoped to the commune in
// the context.
type fakeChecker struct {
	grants map[tenant.ID]map[authz.Perm]struct{}
	calls  int
	asked  []authz.Perm
}

func (c *fakeChecker) Allows(ctx context.Context, p authz.Principal, perm authz.Perm) bool {
	c.calls++
	c.asked = append(c.asked, perm)
	// The commune comes from the CONTEXT, never from the principal — a checker reading p.TenantID
	// would answer about the commune the token claims rather than the one the request arrived at.
	if p.TenantID != tenant.MustFrom(ctx) {
		return false
	}
	_, ok := c.grants[tenant.MustFrom(ctx)][perm]
	return ok
}

func (c *fakeChecker) lastAsked() authz.Perm {
	if len(c.asked) == 0 {
		return ""
	}
	return c.asked[len(c.asked)-1]
}

// fakeDocumentTypeWriter stands in for the write use case.
//
// IT RECORDS THE COMMUNE IT WAS CALLED IN, read from the context exactly as *store.Scoped reads it.
// A fake that ignored the commune would let a wrong-commune test pass while proving nothing — the
// same trap fakeDocumentTypes avoids by keying its rows by commune.
type fakeDocumentTypeWriter struct {
	out domain.DocumentType
	err error

	createCalls, updateCalls, deleteCalls int
	lastTenant                            tenant.ID
	lastActor                             audit.Actor
	lastCreate                            app.CreateDocumentTypeRequest
	lastUpdate                            app.UpdateDocumentTypeRequest
	lastID, lastReason                    string
}

func (g *fakeDocumentTypeWriter) note(ctx context.Context, actor audit.Actor) {
	g.lastTenant = tenant.MustFrom(ctx)
	g.lastActor = actor
}

func (g *fakeDocumentTypeWriter) Create(ctx context.Context, req app.CreateDocumentTypeRequest,
	actor audit.Actor) (domain.DocumentType, error) {
	g.createCalls++
	g.lastCreate = req
	g.note(ctx, actor)
	if g.err != nil {
		return domain.DocumentType{}, g.err
	}
	return g.out, nil
}

func (g *fakeDocumentTypeWriter) Update(ctx context.Context, id string, req app.UpdateDocumentTypeRequest,
	actor audit.Actor) (domain.DocumentType, error) {
	g.updateCalls++
	g.lastID, g.lastUpdate = id, req
	g.note(ctx, actor)
	if g.err != nil {
		return domain.DocumentType{}, g.err
	}
	return g.out, nil
}

func (g *fakeDocumentTypeWriter) SoftDelete(ctx context.Context, id, reason string, actor audit.Actor) error {
	g.deleteCalls++
	g.lastID, g.lastReason = id, reason
	g.note(ctx, actor)
	return g.err
}

func (g *fakeDocumentTypeWriter) totalCalls() int {
	return g.createCalls + g.updateCalls + g.deleteCalls
}

// --- the edge -------------------------------------------------------------------------------------

type principalKey struct{}

// injectPrincipal INJECTS A PRINCIPAL DIRECTLY, in place of the real authentication edge.
//
// THE EDGE EXISTS NOW: core/staffauth.Middleware asks identity over gRPC, and cmd/server.buildEdge
// mounts it — cmd/server/main_test.go drives that chain end to end, including the wrong-commune
// and identity-down cases. Keeping the injection here is deliberate rather than leftover: the
// properties this package owns (ordering, isolation, refusal) must stay testable without standing
// up a fake identity service, and a harness that needed one to assert a sort order is a harness
// that gets bypassed.
//
// What this stand-in reproduces is the ONE property the routes depend on: a principal carrying its
// OWN commune, put into the context INSIDE httpx.TenantMiddleware. It deliberately does NOT copy
// the commune resolved from Host onto the principal — that would quietly make every request
// self-consistent and the wrong-commune case below untestable.
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

type testServer struct {
	h         http.Handler
	d         Deps // kept so a test can rebuild the chain with ONE dependency swapped — see rebuild
	directory fakeDirectory
	types     *fakeDocumentTypes
	writer    *fakeDocumentTypeWriter
	checker   *fakeChecker

	// The two registers — see fake_document_test.go.
	incoming       *fakeIncomingLister
	incomingWriter *fakeIncomingWriter
	incomingReader *fakeIncomingReader
	outgoing       *fakeOutgoingLister
	outgoingWriter *fakeOutgoingWriter

	// The dashboard block — see incoming_dashboard_test.go.
	summary *fakeSummaryReader
	queue   *fakeQueueReader

	// The audit-log reader — see audit_entries_test.go.
	auditLog *auditLogFake

	// idemStore is nil BY DEFAULT, and that is deliberate: a nil store is a valid deployment (local
	// development with no Redis) and it is what makes the CheDoHong each route DECLARED the thing
	// under test rather than Redis behaviour. The two routes that ISSUE A NUMBER declare
	// DongKhiHong, so with nil they answer 503 — see TestDocuments_NoRedisClosesNumberIssuingRoutes,
	// which pins exactly that. Every other assertion about those routes uses newTestServerWithIdem.
	idemStore idem.Store
}

// newTestServer builds the edge chain in the real order, with the stand-in above where the
// authentication middleware will sit.
func newTestServer(t *testing.T) *testServer {
	t.Helper()

	types := sampleDocumentTypes()
	checker := &fakeChecker{}
	writer := &fakeDocumentTypeWriter{out: domain.DocumentType{
		ID: "lvb-moi", Code: "bao-cao", Label: "Báo cáo", SortOrder: 5,
		IsActive: true, Source: domain.SourceCommune,
	}}
	incoming := sampleIncomingLister()
	incomingReader := sampleIncomingReader()
	outgoing := sampleOutgoingLister()
	// The rows the write use cases hand back. They carry a NUMBER AND A YEAR the request never sent,
	// which is what lets a test assert that the register issues them rather than echoing the client.
	incomingWriter := &fakeIncomingWriter{out: domain.IncomingDocument{
		ID: "vbd-moi", ArrivalNo: 8, Year: 2026, IssuingBody: "Huyện uỷ",
		DocumentType: "cong-van", Summary: "Về việc rà soát hộ nghèo",
		Status: domain.IncomingStatusRegistered,
	}}
	outgoingWriter := &fakeOutgoingWriter{out: domain.OutgoingDocument{
		ID: "vbdi-moi", IssuedNo: 12, Year: 2026, DocumentType: "cong-van",
		Summary: "Trả lời đơn của công dân", Recipient: "UBND huyện",
	}}
	summary := sampleSummaryReader()
	queue := sampleQueueReader()
	auditLog := &auditLogFake{}

	m := &testServer{
		directory:      sampleDirectory(),
		types:          types,
		writer:         writer,
		checker:        checker,
		incoming:       incoming,
		incomingWriter: incomingWriter,
		incomingReader: incomingReader,
		outgoing:       outgoing,
		outgoingWriter: outgoingWriter,
		d: Deps{
			Checker:                checker,
			DocumentTypes:          types,
			DocumentTypeWriter:     writer,
			IncomingDocuments:      incoming,
			IncomingDocumentWriter: incomingWriter,
			IncomingDocumentReader: incomingReader,
			OutgoingDocuments:      outgoing,
			OutgoingDocumentWriter: outgoingWriter,
			IncomingSummary:        summary,
			OverdueQueue:           queue,
			AuditLog:               auditLog,
			Clock:                  func() time.Time { return testNow },
			Log:                    slog.New(slog.NewTextHandler(io.Discard, nil)),
		},
		summary:  summary,
		queue:    queue,
		auditLog: auditLog,
	}
	m.rebuild(t, nil)
	return m
}

// grant grants permissions INSIDE ONE COMMUNE. It rebuilds nothing: fakeChecker reads the map on
// each call, so a test may grant mid-way through.
func (m *testServer) grant(tenantID tenant.ID, perm ...authz.Perm) {
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

func (m *testServer) rebuild(t *testing.T, change func(d *Deps)) {
	t.Helper()
	if change != nil {
		change(&m.d)
	}

	mux := http.NewServeMux()
	Register(mux, m.d)

	var h http.Handler = mux
	h = injectPrincipal(h)
	// THE SAME ORDER cmd/server.buildEdge BUILDS. The store is m.idemStore — nil unless a test asked
	// for one, see the field. Inside TenantMiddleware because the idempotency key is prefixed with
	// the commune (rule 1, invariant 7).
	h = idem.Middleware(m.idemStore, slog.New(slog.NewTextHandler(io.Discard, nil)))(h)
	h = httpx.TenantMiddleware(m.directory)(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)
	m.h = h
}

// call issues one request with no body. A nil principal is a request that carries no session at all.
func (m *testServer) call(t *testing.T, method, host, path string, p *authz.Principal) *httptest.ResponseRecorder {
	t.Helper()
	return m.callWithBody(t, method, host, path, p, "")
}

// callWithBody issues one request carrying a JSON body.
//
// IT ALWAYS SENDS AN Idempotency-Key. POST declares idem.Required, which refuses a request without
// the header BEFORE it reaches the handler — so a harness that omitted it would turn every POST
// assertion into an assertion about the header. The one test that cares about the absent header
// builds its own request.
func (m *testServer) callWithBody(t *testing.T, method, host, path string, p *authz.Principal, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	r := httptest.NewRequest(method, "https://"+host+path, reader)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set(idem.Header, "01JIDEMPOTENCYKEYCUATEST")
	if p != nil {
		r = r.WithContext(context.WithValue(r.Context(), principalKey{}, *p))
	}
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)
	return w
}

func wantStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("mã trạng thái = %d, muốn %d — thân: %s", w.Code, want, w.Body.String())
	}
}

func errorBody(t *testing.T, w *httptest.ResponseRecorder) httpx.Error {
	t.Helper()
	var e httpx.Error
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatalf("thân lỗi không phải JSON: %q", w.Body.String())
	}
	return e
}

// --- Register refuses incomplete wiring -----------------------------------------------------------

func TestRegisterRefusesDepsWithNoStores(t *testing.T) {
	// AT CONSTRUCTION, NOT AT REQUEST TIME. A route mounted without its store would answer every
	// call with a panic turned into a 500, and the first person to see it would be a member of
	// staff trying to register a document — long after the deployment that caused it.
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Register nhận Deps thiếu kho mà không panic — tuyến sẽ chết lúc có người gọi")
		}
	}()
	Register(http.NewServeMux(), Deps{})
}

func TestRegisterRefusesDepsMissingAnyOneDependency(t *testing.T) {
	// EACH DEPENDENCY ON ITS OWN, and every other one present — which is the whole point of the
	// shape below. An earlier version of this test passed a Deps carrying only ONE or TWO fields, so
	// when the two registers were added every case went on passing while proving something else
	// entirely: the panic it observed was about the missing register, not about the field the case
	// was named after. A complete Deps minus one field cannot drift that way.
	//
	// A nil Checker is the dangerous one: authz.RequirePermission would meet a nil interface at
	// request time, which is a panic turned into a 500 on the very screen a member of staff uses —
	// long after the deployment that caused it.
	for name, drop := range map[string]func(d *Deps){
		"thiếu use case ghi danh mục":             func(d *Deps) { d.DocumentTypeWriter = nil },
		"thiếu kho danh mục":                      func(d *Deps) { d.DocumentTypes = nil },
		"thiếu kho sổ văn bản đến":                func(d *Deps) { d.IncomingDocuments = nil },
		"thiếu use case ghi văn bản đến":          func(d *Deps) { d.IncomingDocumentWriter = nil },
		"thiếu use case đọc chi tiết văn bản đến": func(d *Deps) { d.IncomingDocumentReader = nil },
		"thiếu kho sổ văn bản đi":                 func(d *Deps) { d.OutgoingDocuments = nil },
		"thiếu use case ghi văn bản đi":           func(d *Deps) { d.OutgoingDocumentWriter = nil },
		"thiếu Checker":                           func(d *Deps) { d.Checker = nil },
		"missing incoming summary reader":         func(d *Deps) { d.IncomingSummary = nil },
		"missing overdue queue":                   func(d *Deps) { d.OverdueQueue = nil },
		"missing audit log reader":                func(d *Deps) { d.AuditLog = nil },
	} {
		t.Run(name, func(t *testing.T) {
			d := Deps{
				Checker:                &fakeChecker{},
				DocumentTypes:          sampleDocumentTypes(),
				DocumentTypeWriter:     &fakeDocumentTypeWriter{},
				IncomingDocuments:      sampleIncomingLister(),
				IncomingDocumentWriter: &fakeIncomingWriter{},
				IncomingDocumentReader: sampleIncomingReader(),
				OutgoingDocuments:      sampleOutgoingLister(),
				OutgoingDocumentWriter: &fakeOutgoingWriter{},
				IncomingSummary:        sampleSummaryReader(),
				OverdueQueue:           sampleQueueReader(),
				AuditLog:               &auditLogFake{},
			}
			drop(&d)
			defer func() {
				if r := recover(); r == nil {
					t.Fatalf("Register nhận Deps %s mà không panic", name)
				}
			}()
			Register(http.NewServeMux(), d)
		})
	}
}
