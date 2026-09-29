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

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// Harness for the comms routes. Nothing here is a PostgreSQL: the properties under test are
// ordering and isolation, and a test that needs infrastructure is a test that stops being run —
// this repository's integration suites skip themselves silently without VIGOV_TEST_DSN.
//
// WHAT THIS HARNESS STILL FAKES, said plainly so nobody reads more into a green run than it proves:
// `fakeAuth` below puts a principal into the context directly, so the tests here prove what
// authz.AnyAuthenticated and the handler do WITH a principal, and nothing about how one is obtained.
//
// The real edge exists now — core/staffauth.Middleware, asking identity over gRPC, mounted by
// cmd/server.buildEdge — and it is exercised where it is wired, in cmd/server/main_test.go. Keeping
// the injection here is deliberate rather than leftover: the route's own properties (ordering,
// isolation, refusal) must stay testable without a resolver, and a harness that had to stand up a
// fake identity to assert a sort order is a harness that gets bypassed.

// --- fixtures ---------------------------------------------------------------------------

const (
	hostA = "xa-a.example.gov.vn"
	hostB = "xa-b.example.gov.vn"

	// The internal staff id, the value identity's Checker matches on. Never the business code.
	staffID = "nd-01JINTERNALIDCUACANBO"
)

var (
	tenantA = tenant.ID("01JA" + strings.Repeat("A", 22))
	tenantB = tenant.ID("01JB" + strings.Repeat("B", 22))
)

type fakeDirectory map[string]tenant.Tenant

func (m fakeDirectory) ByHost(_ context.Context, host string) (tenant.Tenant, bool) {
	t, ok := m[host]
	return t, ok
}

// sampleDirectory is the platform registry for the two test communes. A NEW MAP PER CALL, so a test
// that rewrites one copy cannot change another's.
func sampleDirectory() fakeDirectory {
	return fakeDirectory{
		hostA: {ID: tenantA, Host: hostA, Name: "Xã Thăng Bình", Province: "Thành phố Đà Nẵng", Active: true},
		hostB: {ID: tenantB, Host: hostB, Name: "Xã Bình Dương", Active: true},
	}
}

// fakeMapAssetTypes is the map-asset-type catalogue, KEYED BY COMMUNE, reading the commune from the
// context exactly as *store.Scoped does. Keyed any other way, the isolation case in
// map_asset_type_test.go would pass while proving nothing.
type fakeMapAssetTypes struct {
	byTenant map[tenant.ID][]domain.MapAssetType
	err      error
	calls    int
}

func (d *fakeMapAssetTypes) List(ctx context.Context) ([]domain.MapAssetType, error) {
	d.calls++
	if d.err != nil {
		return nil, d.err
	}
	return d.byTenant[tenant.MustFrom(ctx)], nil
}

// sampleMapAssetTypes gives commune A three groups and commune B one with a DIFFERENT name. Two
// communes whose catalogues were spelled the same could not show a leak.
//
// THE CODES ARE DELIBERATELY NOT ANY OF THE SPECIFICATION'S. docs/ui-ux/10-ban-do-kinh-te-so.md
// contradicts itself about this list — eleven groups at :37 against eight at :53, spelled
// `enterprise` in one place and `doanh-nghiep` in the other — and the table ships EMPTY until the
// user settles both the count and the spelling (migration 0003, REASON TWO). A fixture spelling
// either version would read like a side being taken.
//
// The order is the one the store returns (ORDER BY thu_tu, ma) and is deliberately NOT the
// alphabetical order of the codes: `mau-ba` sorts first alphabetically and last by `thu_tu`, so any
// re-sort in the handler turns the order test red.
//
// The third row is `IsActive: false` — a group taken out of use. It is part of the list on purpose;
// see the note on domain.MapAssetType.IsActive.
func sampleMapAssetTypes() *fakeMapAssetTypes {
	return &fakeMapAssetTypes{byTenant: map[tenant.ID][]domain.MapAssetType{
		tenantA: {
			{ID: "ltn-001", Code: "mau-mot", Label: "Nhóm mẫu một", SortOrder: 1, IsDefault: true, IsActive: true},
			{ID: "ltn-002", Code: "mau-hai", Label: "Nhóm mẫu hai", SortOrder: 2, IsActive: true},
			{ID: "ltn-003", Code: "mau-ba", Label: "Nhóm mẫu ba", SortOrder: 3, IsActive: false},
		},
		tenantB: {
			{ID: "ltn-b-001", Code: "mau-cua-xa-b", Label: "Nhóm mẫu của xã B", SortOrder: 1, IsActive: true},
		},
	}}
}

// denyAllChecker grants NOTHING, in any commune, and counts every call.
//
// Both halves are the assertion. The route is AnyAuthenticated, so a checker that denies everything
// must not change the outcome — and the count proves the route does not consult it at all rather
// than consulting it and ignoring the answer.
type denyAllChecker struct{ calls int }

func (c *denyAllChecker) Allows(context.Context, authz.Principal, authz.Perm) bool {
	c.calls++
	return false
}

// --- harness ----------------------------------------------------------------------------

type server struct {
	h         http.Handler
	d         Deps
	directory fakeDirectory
	catalogue *fakeMapAssetTypes
	checker   *denyAllChecker

	// session is the principal this harness pretends an authentication edge resolved. nil means no
	// session at all — the 401 case. It is read PER REQUEST, so a test changes it without rebuilding
	// the chain.
	session *authz.Principal
}

func newServer(t *testing.T) *server {
	t.Helper()

	catalogue := sampleMapAssetTypes()
	checker := &denyAllChecker{}

	m := &server{
		directory: sampleDirectory(),
		catalogue: catalogue,
		checker:   checker,
		d: Deps{
			Checker:       checker,
			MapAssetTypes: catalogue,
			// The write use case, so Register accepts the Deps. NOTHING IN THIS FILE CALLS IT: the
			// three write routes have their own four-case suite in
			// map_asset_type_write_test.go, with a fake that records the commune and the
			// acting person. Register refuses a nil dependency at construction, so it has to be
			// present — and a fake that is never invoked cannot answer anything wrongly.
			WriteMapAssetTypes: &fakeMapAssetTypeWriter{},
			// Same argument for the two announcement dependencies: Register refuses a nil one at
			// construction, and NOTHING IN THIS FILE CALLS EITHER — the announcement routes have
			// their own four-case permission suite in announcement_test.go.
			Announcements:      &fakeAnnouncementReader{},
			WriteAnnouncements: &fakeAnnouncementWriter{},
			// And the same for the four Mini App content dependencies: present because Register
			// refuses a nil one at construction, called by nothing here — their four-case permission
			// suite is in content_item_test.go.
			ContentItems:           &fakeContentItemReader{},
			WriteContentItems:      &fakeContentItemWriter{},
			ContentCategories:      &fakeContentCategoryReader{},
			WriteContentCategories: &fakeContentCategoryWriter{},
			// The map field schema: present because Register refuses a nil one; its four-case
			// suite is map_field_schema_test.go.
			MapFieldSchemas:      &fakeMapFieldSchemas{},
			WriteMapFieldSchemas: &fakeMapFieldSchemas{},
			// The mail server: present because Register refuses a nil one; its suite is mail_settings_test.go.
			MailSettings:      &fakeMailSettings{},
			WriteMailSettings: &fakeMailSettings{},
			// The audit-log reader: present because Register refuses a nil one; its suite is audit_entries_test.go.
			AuditLog: &auditLogFake{},
			// The header bell: present because Register refuses a nil one; its suite is staff_notification_test.go.
			StaffInbox:      &fakeInbox{},
			WriteStaffInbox: &fakeInbox{},
			Log:             slog.New(slog.NewTextHandler(io.Discard, nil)),
		},
	}

	mux := http.NewServeMux()
	Register(mux, m.d)

	// The real edge chain minus the authentication middleware this service does not have. The order
	// of what IS here is the real one and is not negotiable: strip client-supplied commune headers,
	// recover panics into a traceable 500, resolve Host -> commune (404 when it resolves to none).
	var h http.Handler = mux
	h = m.fakeAuth(h)
	h = httpx.TenantMiddleware(m.directory)(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)
	m.h = h
	return m
}

func (m *server) fakeAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.session != nil {
			r = r.WithContext(authz.Into(r.Context(), *m.session))
		}
		next.ServeHTTP(w, r)
	})
}

// signIn makes the next requests carry a session issued BY THE NAMED COMMUNE. The commune is part of
// the principal, never of the request, which is what makes "signed in at A, calling B" expressible at
// all.
func (m *server) signIn(t tenant.ID) {
	m.session = &authz.Principal{ID: staffID, Kind: "staff", TenantID: t}
}

func (m *server) call(t *testing.T, method, host, path string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, "https://"+host+path, nil)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)
	return w
}

func expectStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("mã trạng thái = %d, muốn %d — thân: %s", w.Code, want, w.Body.String())
	}
}

func decodeError(t *testing.T, w *httptest.ResponseRecorder) httpx.Error {
	t.Helper()
	var e httpx.Error
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatalf("thân lỗi không phải JSON: %q", w.Body.String())
	}
	return e
}

// --- the edge -----------------------------------------------------------------------------

func TestUnknownHostIs404(t *testing.T) {
	// Rule 1, invariant 3: cannot resolve the commune -> 404, never a default commune. 404 also
	// reveals nothing about which communes exist on the platform.
	m := newServer(t)
	m.signIn(tenantA)

	expectStatus(t, m.call(t, "GET", "khong-ai-biet.example.gov.vn", mapAssetTypesPath), http.StatusNotFound)
	if m.catalogue.calls != 0 {
		t.Error("tên miền không thuộc xã nào mà vẫn đọc danh mục")
	}
}

func TestRegisterPanicsOnMissingStore(t *testing.T) {
	// Refusing incomplete wiring at CONSTRUCTION, not at request time. Without this, the route is
	// mounted over a nil interface and every caller gets a recovered 500 — a broken screen whose
	// cause is nowhere in the message.
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Register không panic khi thiếu kho danh mục")
		}
	}()
	Register(http.NewServeMux(), Deps{})
}
