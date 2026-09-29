package http

// The four-case permission suite rule 5, invariant 7 requires — 401 · 403 wrong permission · 403
// right permission WRONG COMMUNE · 2xx both correct — for both announcement routes, plus the
// refusals that are specific to this module.
//
// THE ROUTES ARE MOUNTED THROUGH THE REAL Register, BEHIND THE REAL EDGE CHAIN in the real order.
// A test mux would prove that a handler works and nothing about the declaration this service
// actually ships — and the declaration is where rule 5's defects live: an endpoint with no
// permission is callable by every staff role, and nothing turns red.

import (
	"context"
	"encoding/json"
	"errors"
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
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

const announcementsPath = "/api/v1/announcements"

// --- fakes ---------------------------------------------------------------------------------

// fakeAnnouncementReader is the READ half. It records the commune it was asked in, which is the one
// thing a handler test can assert about isolation: the store binds `tenant_id` from the context, so
// what is provable here is that the context reaching it carries the commune of the Host.
type fakeAnnouncementReader struct {
	calls  int
	tenant tenant.ID
	req    page.Request
	out    page.Result[domain.Announcement]
	err    error
	called bool
}

func (s *fakeAnnouncementReader) List(ctx context.Context, req page.Request) (
	page.Result[domain.Announcement], error) {
	s.calls++
	s.called = true
	s.tenant = tenant.MustFrom(ctx)
	s.req = req
	if s.err != nil {
		return page.NewResult[domain.Announcement](), s.err
	}
	return s.out, nil
}

// fakeAnnouncementWriter is the WRITE half. It records the request AND THE ACTOR, because the actor
// is what rule 6, invariant 8 is about: the trail must carry `CB-…` and never an internal id, and the
// only place a handler can get that wrong is here.
type fakeAnnouncementWriter struct {
	calls   int
	tenant  tenant.ID
	lastReq domain.CreateAnnouncementRequest
	actor   audit.Actor
	out     domain.Announcement
	err     error
}

func (g *fakeAnnouncementWriter) Publish(ctx context.Context, req domain.CreateAnnouncementRequest,
	actor audit.Actor) (domain.Announcement, error) {
	g.calls++
	g.tenant = tenant.MustFrom(ctx)
	g.lastReq = req
	g.actor = actor
	if g.err != nil {
		return domain.Announcement{}, g.err
	}
	return g.out, nil
}

// memoryIdemStore is an in-memory idempotency store.
//
// WHY THE TEST NEEDS ONE AT ALL, when the catalogue's write suite runs with a nil store: the POST
// route below declares idem.Required(DongKhiHong), which answers 503 when no store is configured —
// correctly, and that is the whole point of the declaration. A nil store here would turn every
// assertion in this file into an assertion about Redis being absent.
//
// It is not a Redis emulator: Claim is compare-and-set, Get reads, Complete overwrites, Release
// removes. That is the entire contract idem.Store states, and nothing in these tests exercises a
// TTL.
type memoryIdemStore struct {
	mu sync.Mutex
	m  map[string]string
}

func newMemoryIdemStore() *memoryIdemStore { return &memoryIdemStore{m: map[string]string{}} }

func (k *memoryIdemStore) Claim(_ context.Context, key string, _ time.Duration) (bool, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if _, taken := k.m[key]; taken {
		return false, nil
	}
	k.m[key] = ""
	return true, nil
}

func (k *memoryIdemStore) Get(_ context.Context, key string) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.m[key], nil
}

func (k *memoryIdemStore) Complete(_ context.Context, key, value string, _ time.Duration) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.m[key] = value
	return nil
}

func (k *memoryIdemStore) Release(_ context.Context, key string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.m, key)
	return nil
}

// --- harness -------------------------------------------------------------------------------

type announcementServer struct {
	h       http.Handler
	reader  *fakeAnnouncementReader
	writer  *fakeAnnouncementWriter
	checker *perCommuneChecker
}

func newAnnouncementServer(t *testing.T) *announcementServer {
	t.Helper()

	reader := &fakeAnnouncementReader{out: page.NewResult[domain.Announcement]()}
	writer := &fakeAnnouncementWriter{}
	checker := &perCommuneChecker{}
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))

	mux := http.NewServeMux()
	Register(mux, Deps{
		Checker:            checker,
		MapAssetTypes:      sampleMapAssetTypes(),
		WriteMapAssetTypes: &fakeMapAssetTypeWriter{},
		Announcements:      reader,
		WriteAnnouncements: writer,
		// Same again for the four Mini App content dependencies — see content_item_test.go.
		ContentItems:           &fakeContentItemReader{},
		WriteContentItems:      &fakeContentItemWriter{},
		ContentCategories:      &fakeContentCategoryReader{},
		WriteContentCategories: &fakeContentCategoryWriter{},
		// And the map field schema — see map_field_schema_test.go.
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
		Log:             quiet,
	})

	// The real edge chain in the real order. idem.Middleware sits INSIDE TenantMiddleware because
	// the idempotency key is prefixed with the commune (rule 1, invariant 7) — outside it, two
	// communes sending the same key would share one key space.
	var h http.Handler = mux
	h = injectPrincipal(h)
	h = idem.Middleware(newMemoryIdemStore(), quiet)(h)
	h = httpx.TenantMiddleware(sampleDirectory())(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)

	return &announcementServer{h: h, reader: reader, writer: writer, checker: checker}
}

func (m *announcementServer) grant(t tenant.ID, perms ...authz.Perm) { m.checker.grantIn(t, perms...) }

// call sends one request. It ALWAYS attaches an Idempotency-Key on a POST: the route declares
// idem.Required, which refuses a request without the header BEFORE it reaches the handler, so a
// harness that omitted it would turn every POST assertion into an assertion about the header.
func (m *announcementServer) call(t *testing.T, method, host, body string, p *authz.Principal) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	r := httptest.NewRequest(method, "https://"+host+announcementsPath, reader)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	if method == http.MethodPost {
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set(idem.Header, "01JTHONGBAOKEY00000000000")
	}
	if p != nil {
		r = r.WithContext(authz.Into(r.Context(), *p))
	}
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)
	return w
}

// staffOf builds a staff principal of one commune. `Ma` IS SET AND IS NOT THE ID: the handler copies
// it into audit.Actor.ID, which is the column an inspection reads years later (rule 6, invariant 8).
func staffOf(t tenant.ID) *authz.Principal {
	return &authz.Principal{ID: "01JCANBONOIBO000000000000", Ma: "CB-2026-7K3M9Q", Kind: "staff", TenantID: t}
}

// --- rule 5, invariant 7: the four cases, on the READ route ---------------------------------

func TestListAnnouncementsNoSessionIs401(t *testing.T) {
	m := newAnnouncementServer(t)
	expectStatus(t, m.call(t, http.MethodGet, hostA, "", nil), http.StatusUnauthorized)
	if m.reader.called {
		t.Error("không có phiên mà vẫn đọc sổ thông báo")
	}
}

func TestListAnnouncementsWrongPermissionIs403(t *testing.T) {
	m := newAnnouncementServer(t)
	// The account is signed in and holds a DIFFERENT permission of the same service. This is the
	// case a missing declaration would let through, and nothing would report it.
	m.grant(tenantA, "admin.lookup")
	expectStatus(t, m.call(t, http.MethodGet, hostA, "", staffOf(tenantA)), http.StatusForbidden)
	if m.reader.called {
		t.Error("thiếu quyền mà vẫn đọc sổ thông báo")
	}
}

func TestListAnnouncementsRightPermissionWrongCommuneIs403(t *testing.T) {
	// THE CASE THAT IS ONLY EXPRESSIBLE WITH TWO COMMUNES. The permission is granted in commune A;
	// the request arrives at commune B's domain with a principal of commune B. Granting per commune
	// is what stops one commune's administrator from opening another commune's book.
	m := newAnnouncementServer(t)
	m.grant(tenantA, PermAnnouncement)
	expectStatus(t, m.call(t, http.MethodGet, hostB, "", staffOf(tenantB)), http.StatusForbidden)
	if m.reader.called {
		t.Error("quyền cấp ở xã A mà đọc được sổ của xã B")
	}
}

func TestListAnnouncementsWithPermissionIs200InTheRightCommune(t *testing.T) {
	m := newAnnouncementServer(t)
	m.grant(tenantB, PermAnnouncement)
	w := m.call(t, http.MethodGet, hostB, "", staffOf(tenantB))
	expectStatus(t, w, http.StatusOK)

	if m.reader.tenant != tenantB {
		t.Fatalf("kho được gọi với xã %q, muốn %q — xã phải suy từ Host, không từ yêu cầu", m.reader.tenant, tenantB)
	}
	// `items` MUST BE [] AND NEVER null on a commune that has issued nothing — which is every
	// commune today. A client that has to handle both shapes handles one of them wrong.
	if !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Errorf("danh sách rỗng phải là [] chứ không phải null — thân: %s", w.Body.String())
	}
}

// --- rule 5, invariant 7: the four cases, on the WRITE route ---------------------------------

const validPublishBody = `{"title":"Mời họp giao ban tháng 9","body":"Kính mời các đồng chí dự họp.","recipient_codes":["CB-2026-AAAA11"]}`

func TestPublishAnnouncementNoSessionIs401(t *testing.T) {
	m := newAnnouncementServer(t)
	expectStatus(t, m.call(t, http.MethodPost, hostA, validPublishBody, nil), http.StatusUnauthorized)
	if m.writer.calls != 0 {
		t.Error("không có phiên mà vẫn phát hành thông báo")
	}
}

func TestPublishAnnouncementWrongPermissionIs403(t *testing.T) {
	m := newAnnouncementServer(t)
	m.grant(tenantA, "admin.lookup")
	expectStatus(t, m.call(t, http.MethodPost, hostA, validPublishBody, staffOf(tenantA)), http.StatusForbidden)
	if m.writer.calls != 0 {
		t.Error("thiếu quyền mà vẫn phát hành thông báo")
	}
}

func TestPublishAnnouncementRightPermissionWrongCommuneIs403(t *testing.T) {
	m := newAnnouncementServer(t)
	m.grant(tenantA, PermAnnouncement)
	expectStatus(t, m.call(t, http.MethodPost, hostB, validPublishBody, staffOf(tenantB)), http.StatusForbidden)
	if m.writer.calls != 0 {
		t.Error("quyền cấp ở xã A mà phát hành được ở xã B")
	}
}

func TestPublishAnnouncementWithPermissionIs201AndActorIsStaffCode(t *testing.T) {
	m := newAnnouncementServer(t)
	m.grant(tenantA, PermAnnouncement)
	m.writer.out = domain.Announcement{
		ID: "01JTHONGBAOMOI0000000000", Title: "Mời họp giao ban tháng 9",
		Body: "Kính mời các đồng chí dự họp.", Status: domain.AnnouncementPublished,
		EmailStatus: domain.EmailNotSent, AuthorCode: "CB-2026-7K3M9Q",
		IssuedAt:       time.Date(2026, 9, 23, 8, 30, 0, 0, time.UTC),
		CreatedAt:      time.Date(2026, 9, 23, 8, 30, 0, 0, time.UTC),
		RecipientCount: 1,
	}

	w := m.call(t, http.MethodPost, hostA, validPublishBody, staffOf(tenantA))
	expectStatus(t, w, http.StatusCreated)

	if m.writer.tenant != tenantA {
		t.Fatalf("use case được gọi với xã %q, muốn %q", m.writer.tenant, tenantA)
	}
	// RULE 6, INVARIANT 8, AND THIS IS THE ASSERTION THAT WOULD HAVE CAUGHT THE SIX DEFECTS OF
	// 2026-09-22: the actor is the BUSINESS CODE, never `Principal.ID`. A ULID in `actor_id` names
	// nobody to the person reading the trail years later, and the two are indistinguishable on sight.
	if m.writer.actor.ID != "CB-2026-7K3M9Q" {
		t.Errorf("chủ thể vết kiểm toán = %q, muốn mã cán bộ CB-2026-7K3M9Q", m.writer.actor.ID)
	}
	if m.writer.actor.IP == "" {
		t.Error("vết kiểm toán thiếu địa chỉ IP (luật 6, bất biến 2)")
	}
	// The body is DECODED INTO THE REQUEST, not passed through: a handler that dropped
	// `recipient_codes` would issue an announcement to nobody and nothing else here would notice.
	if len(m.writer.lastReq.RecipientCodes) != 1 || m.writer.lastReq.RecipientCodes[0] != "CB-2026-AAAA11" {
		t.Errorf("người nhận không tới được use case: %#v", m.writer.lastReq.RecipientCodes)
	}

	var out thongBaoRa
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("thân trả về không phải JSON: %v", err)
	}
	if out.Status != "da-phat-hanh" || out.RecipientCount != 1 {
		t.Errorf("phản hồi = %+v, muốn trạng thái da-phat-hanh và 1 người nhận", out)
	}
	if out.IssuedAt == nil {
		t.Error("thông báo đã phát hành phải có issued_at")
	}
	if out.EmailStatus != "chua-gui" {
		t.Errorf("trạng thái thư = %q, muốn chua-gui — kho này chưa có bộ gửi thư", out.EmailStatus)
	}
}

// --- the refusals that belong to this module -------------------------------------------------

func TestPublishToOrgUnitsIs501AndSendsToNobody(t *testing.T) {
	// THE REFUSAL THIS PASS EXISTS TO MAKE LOUD. Expanding a department into its staff needs an RPC
	// service-identity does not publish, and the alternative — record the departments, deliver to
	// nobody — produces no error anywhere while the commune is never told.
	m := newAnnouncementServer(t)
	m.grant(tenantA, PermAnnouncement)
	m.writer.err = app.ErrOrgUnitDeliveryUnavailable

	w := m.call(t, http.MethodPost, hostA,
		`{"title":"Mời họp","body":"Nội dung","org_unit_ids":["01JBOPHAN0000000000000000"]}`,
		staffOf(tenantA))
	expectStatus(t, w, http.StatusNotImplemented)

	e := decodeError(t, w)
	if e.Code != "not_implemented" {
		t.Errorf("mã lỗi = %q, muốn not_implemented", e.Code)
	}
	// THE SENTENCE HAS TO NAME THE WORKAROUND. An error a person cannot act on is an error they
	// report to somebody else, and this one has a real answer: name the recipients.
	if !strings.Contains(e.Message, "đích danh") {
		t.Errorf("thông điệp không chỉ ra cách làm được: %q", e.Message)
	}
}

func TestPublishWithoutRecipientsIs400(t *testing.T) {
	m := newAnnouncementServer(t)
	m.grant(tenantA, PermAnnouncement)
	m.writer.err = domain.ErrNoRecipients

	w := m.call(t, http.MethodPost, hostA, `{"title":"Mời họp","body":"Nội dung"}`, staffOf(tenantA))
	expectStatus(t, w, http.StatusBadRequest)
	if decodeError(t, w).Code != "invalid_request" {
		t.Errorf("mã lỗi = %q, muốn invalid_request", decodeError(t, w).Code)
	}
}

func TestPublishStoreErrorDoesNotLeakContent(t *testing.T) {
	// Rule 3, forbidden #3: an announcement body is free text a colleague typed and can quote a
	// case. A store failure must not carry any of it back to the client.
	m := newAnnouncementServer(t)
	m.grant(tenantA, PermAnnouncement)
	m.writer.err = errors.New("pq: duplicate key value violates unique constraint on noi_dung bí mật")

	w := m.call(t, http.MethodPost, hostA, validPublishBody, staffOf(tenantA))
	expectStatus(t, w, http.StatusInternalServerError)
	if strings.Contains(w.Body.String(), "bí mật") || strings.Contains(w.Body.String(), "pq:") {
		t.Errorf("lỗi kho lọt ra client: %s", w.Body.String())
	}
}

func TestListAnnouncementsBadCursorIs400(t *testing.T) {
	m := newAnnouncementServer(t)
	m.grant(tenantA, PermAnnouncement)

	r := httptest.NewRequest(http.MethodGet, "https://"+hostA+announcementsPath+"?cursor=khong-phai-con-tro", nil)
	r.Host = hostA
	r.RemoteAddr = "10.0.0.7:51000"
	r = r.WithContext(authz.Into(r.Context(), *staffOf(tenantA)))
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)

	expectStatus(t, w, http.StatusBadRequest)
	if m.reader.called {
		t.Error("con trỏ hỏng mà vẫn chạy truy vấn — page.Parse phải chặn TRƯỚC khi chạm CSDL")
	}
}

func TestListAnnouncementsDefaultSortIsNewestFirst(t *testing.T) {
	// §2: "mới nhất ở trên". The default sort is part of the contract: a client that sends no
	// `sort` must get the newest first, and a silent change of default reorders every screen.
	m := newAnnouncementServer(t)
	m.grant(tenantA, PermAnnouncement)
	expectStatus(t, m.call(t, http.MethodGet, hostA, "", staffOf(tenantA)), http.StatusOK)

	if m.reader.req.Column().SQL != "tao_luc" {
		t.Errorf("cột sắp xếp mặc định = %q, muốn tao_luc", m.reader.req.Column().SQL)
	}
	if m.reader.req.Dir() != page.Desc {
		t.Errorf("chiều sắp xếp mặc định = %q, muốn desc", m.reader.req.Dir())
	}
}
