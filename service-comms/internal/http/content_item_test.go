package http

// The four-case permission suite rule 5, invariant 7 requires — 401 · 403 wrong permission · 403
// right permission WRONG COMMUNE · 2xx both correct — for ALL SIX Mini App content routes, plus the
// refusals and the contract details that are specific to this module.
//
// THE ROUTES ARE MOUNTED THROUGH THE REAL Register, BEHIND THE REAL EDGE CHAIN in the real order. A
// test mux would prove that a handler works and nothing about the declaration this service actually
// ships — and the declaration is where rule 5's defects live: an endpoint with no permission is
// callable by every staff role, and nothing turns red.
//
// THE "WRONG PERMISSION" CASE GRANTS THE OTHER KEY OF THIS SAME MODULE. `content.read` against a
// write route and `content.update` against a read route is a sharper test than an unrelated key:
// §10.5 divides this screen into exactly those two rights, and a route that accepted either would
// let anybody who may LOOK at the commune's news also PUBLISH it.

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
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

const (
	contentItemsPath      = "/api/v1/content-items"
	contentCategoriesPath = "/api/v1/content-categories"
)

// --- fakes ---------------------------------------------------------------------------------

// fakeContentItemReader is the content READ half. It records the commune it was asked in, which is
// the one thing a handler test can assert about isolation: the store binds `tenant_id` from the
// context, so what is provable here is that the context reaching it carries the commune of the Host.
type fakeContentItemReader struct {
	listCalls, byIDCalls int
	tenant               tenant.ID
	lastFilter           commsstore.ContentItemFilter
	lastReq              page.Request
	out                  page.Result[domain.ContentItem]
	one                  domain.ContentItem
	err                  error
}

func (s *fakeContentItemReader) List(ctx context.Context, filter commsstore.ContentItemFilter, req page.Request) (
	page.Result[domain.ContentItem], error) {
	s.listCalls++
	s.tenant = tenant.MustFrom(ctx)
	s.lastFilter = filter
	s.lastReq = req
	if s.err != nil {
		return page.NewResult[domain.ContentItem](), s.err
	}
	return s.out, nil
}

func (s *fakeContentItemReader) ByID(ctx context.Context, id string) (domain.ContentItem, error) {
	s.byIDCalls++
	s.tenant = tenant.MustFrom(ctx)
	if s.err != nil {
		return domain.ContentItem{}, s.err
	}
	return s.one, nil
}

// fakeContentItemWriter is the content WRITE half. It records the request AND THE ACTOR, because the
// actor is what rule 6, invariant 8 is about: the trail must carry `CB-…` and never an internal id,
// and the only place a handler can get that wrong is here.
type fakeContentItemWriter struct {
	createCalls, updateCalls int
	tenant                   tenant.ID
	actor                    audit.Actor
	lastCreate               domain.CreateContentItemRequest
	lastUpdate               domain.UpdateContentItemRequest
	lastID                   string
	out                      domain.ContentItem
	err                      error
}

func (g *fakeContentItemWriter) Create(ctx context.Context, req domain.CreateContentItemRequest, actor audit.Actor) (
	domain.ContentItem, error) {
	g.createCalls++
	g.tenant = tenant.MustFrom(ctx)
	g.lastCreate = req
	g.actor = actor
	if g.err != nil {
		return domain.ContentItem{}, g.err
	}
	return g.out, nil
}

func (g *fakeContentItemWriter) Update(ctx context.Context, id string, req domain.UpdateContentItemRequest,
	actor audit.Actor) (domain.ContentItem, error) {
	g.updateCalls++
	g.tenant = tenant.MustFrom(ctx)
	g.lastID = id
	g.lastUpdate = req
	g.actor = actor
	if g.err != nil {
		return domain.ContentItem{}, g.err
	}
	return g.out, nil
}

func (g *fakeContentItemWriter) total() int { return g.createCalls + g.updateCalls }

// fakeContentCategoryReader is the category READ half.
type fakeContentCategoryReader struct {
	calls  int
	tenant tenant.ID
	out    []domain.ContentCategory
	err    error
}

func (s *fakeContentCategoryReader) List(ctx context.Context) ([]domain.ContentCategory, error) {
	s.calls++
	s.tenant = tenant.MustFrom(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return s.out, nil
}

// fakeContentCategoryWriter is the category WRITE half.
type fakeContentCategoryWriter struct {
	calls   int
	tenant  tenant.ID
	actor   audit.Actor
	lastReq domain.CreateContentCategoryRequest
	out     domain.ContentCategory
	err     error
}

func (g *fakeContentCategoryWriter) Create(ctx context.Context, req domain.CreateContentCategoryRequest, actor audit.Actor) (
	domain.ContentCategory, error) {
	g.calls++
	g.tenant = tenant.MustFrom(ctx)
	g.lastReq = req
	g.actor = actor
	if g.err != nil {
		return domain.ContentCategory{}, g.err
	}
	return g.out, nil
}

// --- harness -------------------------------------------------------------------------------

type contentServer struct {
	h              http.Handler
	items          *fakeContentItemReader
	itemWriter     *fakeContentItemWriter
	categories     *fakeContentCategoryReader
	categoryWriter *fakeContentCategoryWriter
	checker        *perCommuneChecker
}

func newContentServer(t *testing.T) *contentServer {
	t.Helper()

	items := &fakeContentItemReader{out: page.NewResult[domain.ContentItem]()}
	itemWriter := &fakeContentItemWriter{out: sampleContentItem()}
	categories := &fakeContentCategoryReader{}
	categoryWriter := &fakeContentCategoryWriter{out: domain.ContentCategory{
		ID: "dm-moi", Name: "Chuyển đổi số", Slug: "chuyen-doi-so", SortOrder: 2, CreatedAt: sampleContentTime,
	}}
	checker := &perCommuneChecker{}
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))

	mux := http.NewServeMux()
	Register(mux, Deps{
		Checker:                checker,
		MapAssetTypes:          sampleMapAssetTypes(),
		WriteMapAssetTypes:     &fakeMapAssetTypeWriter{},
		Announcements:          &fakeAnnouncementReader{},
		WriteAnnouncements:     &fakeAnnouncementWriter{},
		ContentItems:           items,
		WriteContentItems:      itemWriter,
		ContentCategories:      categories,
		WriteContentCategories: categoryWriter,
		// Present because Register refuses a nil one — see map_field_schema_test.go.
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

	// The real edge chain in the real order. idem.Middleware sits INSIDE TenantMiddleware because the
	// idempotency key is prefixed with the commune (rule 1, invariant 7) — outside it, two communes
	// sending the same key would share one key space.
	var h http.Handler = mux
	h = injectPrincipal(h)
	h = idem.Middleware(newMemoryIdemStore(), quiet)(h)
	h = httpx.TenantMiddleware(sampleDirectory())(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)

	return &contentServer{h: h, items: items, itemWriter: itemWriter, categories: categories,
		categoryWriter: categoryWriter, checker: checker}
}

func (m *contentServer) grant(t tenant.ID, perms ...authz.Perm) { m.checker.grantIn(t, perms...) }

// call sends one request. It ALWAYS attaches an Idempotency-Key on a POST: both POST routes declare
// idem.Required, which refuses a request without the header BEFORE it reaches the handler, so a
// harness that omitted it would turn every POST assertion into an assertion about the header.
func (m *contentServer) call(t *testing.T, method, host, path, body string, p *authz.Principal) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	r := httptest.NewRequest(method, "https://"+host+path, reader)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	if method == http.MethodPost || method == http.MethodPatch {
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set(idem.Header, "01JNOIDUNGKEY0000000000000")
	}
	if p != nil {
		r = r.WithContext(authz.Into(r.Context(), *p))
	}
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)
	return w
}

var sampleContentTime = time.Date(2026, 9, 14, 8, 9, 0, 0, time.UTC)

func sampleContentItem() domain.ContentItem {
	return domain.ContentItem{
		ID: "01JNOIDUNGMOI00000000000", Type: domain.ContentTypeNews,
		CategoryID: "dm-001", Title: "Xã Thăng Bình khai giảng năm học mới",
		Summary: "Sáng nay…", Body: "<p>Toàn văn</p>",
		ImageURL:    "https://x/a.png",
		PublishedOn: time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC),
		ViewCount:   0, Status: domain.ContentStatusVisible,
		Source: domain.ContentSourcePortalSync, SourceURL: "https://cong/a", SourceRef: "cong-42",
		HandEdited: true, AuthorCode: "CB-2026-7K3M9Q",
		CreatedAt: sampleContentTime, UpdatedAt: sampleContentTime,
	}
}

const (
	validCreateContentBody  = `{"type":"tin-tuc","title":"Xã Thăng Bình khai giảng năm học mới","summary":"Sáng nay…","publish":true}`
	validUpdateContentBody  = `{"title":"Tiêu đề đã sửa"}`
	validCreateCategoryBody = `{"name":"Chuyển đổi số","slug":"chuyen-doi-so","order":2}`
)

// --- rule 5, invariant 7: the four cases, on all six routes ----------------------------------

// contentRoute is one route under the permission suite.
//
// TABLE-DRIVEN, AND THE TABLE IS THE POINT: six routes times four cases is twenty-four assertions,
// and twenty-four hand-written functions is where one of them quietly stops asserting the commune
// axis. `reached` is what turns "the response was 403" into "and the handler was never reached" —
// without it, a route that answered 403 AFTER doing the work would pass.
type contentRoute struct {
	name     string
	method   string
	path     string
	perm     authz.Perm // the key this route declares
	other    authz.Perm // the OTHER key of this module — the "wrong permission" case
	body     string
	okStatus int
	reached  func(*contentServer) bool
}

func contentRoutes() []contentRoute {
	read, update := PermContentRead, PermContentUpdate
	return []contentRoute{
		{"đọc sổ nội dung", http.MethodGet, contentItemsPath, read, update, "", http.StatusOK,
			func(m *contentServer) bool { return m.items.listCalls > 0 }},
		{"đọc một mục", http.MethodGet, contentItemsPath + "/nd-001", read, update, "", http.StatusOK,
			func(m *contentServer) bool { return m.items.byIDCalls > 0 }},
		{"thêm nội dung", http.MethodPost, contentItemsPath, update, read, validCreateContentBody, http.StatusCreated,
			func(m *contentServer) bool { return m.itemWriter.createCalls > 0 }},
		{"sửa nội dung", http.MethodPatch, contentItemsPath + "/nd-001", update, read, validUpdateContentBody, http.StatusOK,
			func(m *contentServer) bool { return m.itemWriter.updateCalls > 0 }},
		{"đọc danh mục", http.MethodGet, contentCategoriesPath, read, update, "", http.StatusOK,
			func(m *contentServer) bool { return m.categories.calls > 0 }},
		{"thêm danh mục", http.MethodPost, contentCategoriesPath, update, read, validCreateCategoryBody, http.StatusCreated,
			func(m *contentServer) bool { return m.categoryWriter.calls > 0 }},
	}
}

func TestContentRoutesNoSessionIs401(t *testing.T) {
	for _, tc := range contentRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			m := newContentServer(t)
			expectStatus(t, m.call(t, tc.method, hostA, tc.path, tc.body, nil), http.StatusUnauthorized)
			if tc.reached(m) {
				t.Error("không có phiên mà vẫn chạm tới nghiệp vụ")
			}
		})
	}
}

func TestContentRoutesWrongPermissionIs403(t *testing.T) {
	for _, tc := range contentRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			// THE ACCOUNT HOLDS THE OTHER KEY OF THIS SAME MODULE. §10.5 divides this screen into
			// `content.read` and `content.update`, and a route that accepted either would let anybody
			// who may look at the commune's news also publish it.
			m := newContentServer(t)
			m.grant(tenantA, tc.other)
			expectStatus(t, m.call(t, tc.method, hostA, tc.path, tc.body, staffOf(tenantA)), http.StatusForbidden)
			if tc.reached(m) {
				t.Error("thiếu quyền mà vẫn chạm tới nghiệp vụ")
			}
		})
	}
}

func TestContentRoutesRightPermissionWrongCommuneIs403(t *testing.T) {
	for _, tc := range contentRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			// THE CASE THAT IS ONLY EXPRESSIBLE WITH TWO COMMUNES. The permission is granted in
			// commune A; the request arrives at commune B's domain with a principal of commune B.
			// Granting per commune is what stops one commune's administrator from opening another
			// commune's register.
			m := newContentServer(t)
			m.grant(tenantA, tc.perm)
			expectStatus(t, m.call(t, tc.method, hostB, tc.path, tc.body, staffOf(tenantB)), http.StatusForbidden)
			if tc.reached(m) {
				t.Error("quyền cấp ở xã A mà làm được ở xã B")
			}
		})
	}
}

func TestContentRoutesWithPermissionIs2xxInTheRightCommune(t *testing.T) {
	for _, tc := range contentRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			m := newContentServer(t)
			m.grant(tenantB, tc.perm)
			w := m.call(t, tc.method, hostB, tc.path, tc.body, staffOf(tenantB))
			expectStatus(t, w, tc.okStatus)
			if !tc.reached(m) {
				t.Fatal("đủ quyền mà nghiệp vụ không được gọi")
			}
			// THE COMMUNE COMES FROM Host AND FROM NOWHERE ELSE (rule 1, invariant 3). The store binds
			// it to $1 from the context, so what is provable here is that the context reaching the
			// business layer carries the commune the request arrived at.
			for _, got := range []tenant.ID{m.items.tenant, m.itemWriter.tenant, m.categories.tenant, m.categoryWriter.tenant} {
				if got != "" && got != tenantB {
					t.Errorf("nghiệp vụ được gọi với xã %q, muốn %q", got, tenantB)
				}
			}
		})
	}
}

// --- the contract details of this module ------------------------------------------------------

func TestListContentItemsEmptyIsArrayNotNull(t *testing.T) {
	// `items` MUST BE [] AND NEVER null on a commune that has published nothing — which is every
	// commune today. A client that has to handle both shapes handles one of them wrong.
	m := newContentServer(t)
	m.grant(tenantA, PermContentRead)
	w := m.call(t, http.MethodGet, hostA, contentItemsPath, "", staffOf(tenantA))
	expectStatus(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Errorf("danh sách rỗng phải là [] chứ không phải null — thân: %s", w.Body.String())
	}
}

func TestListContentItemsOmitsBodyButDetailCarriesIt(t *testing.T) {
	// THE CONTRACT DIFFERENCE BETWEEN THE TWO READS, asserted on the wire rather than assumed. `body`
	// is ABSENT from a list item and PRESENT on the detail: "" would otherwise mean two different
	// things — "this article has no body" and "you asked for a page, which does not carry bodies" —
	// and a client rendering the second as the first shows an empty article with nothing saying so.
	m := newContentServer(t)
	m.grant(tenantA, PermContentRead)
	m.items.out = page.Result[domain.ContentItem]{Items: []domain.ContentItem{sampleContentItem()}}
	m.items.one = sampleContentItem()

	w := m.call(t, http.MethodGet, hostA, contentItemsPath, "", staffOf(tenantA))
	expectStatus(t, w, http.StatusOK)
	if strings.Contains(w.Body.String(), `"body"`) {
		t.Errorf("trang danh sách không được mang `body`: %s", w.Body.String())
	}

	w = m.call(t, http.MethodGet, hostA, contentItemsPath+"/nd-001", "", staffOf(tenantA))
	expectStatus(t, w, http.StatusOK)
	var detail noiDungRa
	if err := json.Unmarshal(w.Body.Bytes(), &detail); err != nil {
		t.Fatalf("thân chi tiết không phải JSON: %v", err)
	}
	if detail.Body == nil || *detail.Body != "<p>Toàn văn</p>" {
		t.Errorf("tuyến chi tiết phải mang toàn văn: %#v", detail.Body)
	}
	// §6's `Tệp đính kèm` column, derived rather than stored.
	if !detail.HasImage {
		t.Error("has_image phải suy ra từ image_url")
	}
	// §10.4's flag is on the wire because it is the only place the screen learns that a correction is
	// now protected from the next synchronisation.
	if !detail.HandEdited || detail.Source != "dong-bo-cong" {
		t.Errorf("xuất xứ không ra tới hợp đồng: source=%q hand_edited=%v",
			detail.Source, detail.HandEdited)
	}
	// A DATE, not a timestamp: an article carried over from the portal was published on a day.
	if detail.PublishedOn != "2026-09-14" {
		t.Errorf("published_on = %q, muốn 2026-09-14", detail.PublishedOn)
	}
}

func TestListContentItemsPassesThreeFiltersToStore(t *testing.T) {
	// §6's filter bar reaching the store. A handler that dropped one of the three would show the
	// wrong tab's rows under the right tab's header, with no error anywhere.
	m := newContentServer(t)
	m.grant(tenantA, PermContentRead)

	w := m.call(t, http.MethodGet, hostA,
		contentItemsPath+"?type=banner&category=dm-9&q=khai+gi%E1%BA%A3ng", "", staffOf(tenantA))
	expectStatus(t, w, http.StatusOK)

	if m.items.lastFilter.Type != "banner" || m.items.lastFilter.CategoryID != "dm-9" ||
		m.items.lastFilter.Search != "khai giảng" {
		t.Errorf("bộ lọc tới kho = %+v", m.items.lastFilter)
	}
}

func TestListContentItemsUnknownTypeIs400(t *testing.T) {
	// AN UNKNOWN `type` IS REFUSED RATHER THAN PASSED THROUGH. Passed through it matches nothing, and
	// the screen shows an empty tab with no way to tell "this commune has no banners" from "the
	// client sent a code that does not exist".
	m := newContentServer(t)
	m.grant(tenantA, PermContentRead)

	w := m.call(t, http.MethodGet, hostA, contentItemsPath+"?type=podcast", "", staffOf(tenantA))
	expectStatus(t, w, http.StatusBadRequest)
	if m.items.listCalls != 0 {
		t.Error("loại lạ mà vẫn chạy truy vấn")
	}
	if decodeError(t, w).Code != "invalid_request" {
		t.Errorf("mã lỗi = %q, muốn invalid_request", decodeError(t, w).Code)
	}
}

func TestListContentItemsBadCursorIs400(t *testing.T) {
	m := newContentServer(t)
	m.grant(tenantA, PermContentRead)

	w := m.call(t, http.MethodGet, hostA, contentItemsPath+"?cursor=khong-phai-con-tro", "", staffOf(tenantA))
	expectStatus(t, w, http.StatusBadRequest)
	if m.items.listCalls != 0 {
		t.Error("con trỏ hỏng mà vẫn chạy truy vấn — page.Parse phải chặn TRƯỚC khi chạm CSDL")
	}
}

func TestListContentItemsDefaultSortIsNewestFirst(t *testing.T) {
	// The default sort is part of the contract: a client that sends no `sort` must get the newest
	// first, and a silent change of default reorders every screen.
	m := newContentServer(t)
	m.grant(tenantA, PermContentRead)
	expectStatus(t, m.call(t, http.MethodGet, hostA, contentItemsPath, "", staffOf(tenantA)), http.StatusOK)

	if m.items.lastReq.Column().SQL != "tao_luc" {
		t.Errorf("cột sắp xếp mặc định = %q, muốn tao_luc", m.items.lastReq.Column().SQL)
	}
	if m.items.lastReq.Dir() != page.Desc {
		t.Errorf("chiều sắp xếp mặc định = %q, muốn desc", m.items.lastReq.Dir())
	}
}

func TestCreateContentItemActorIsStaffCodeAndBodyReachesUseCase(t *testing.T) {
	m := newContentServer(t)
	m.grant(tenantA, PermContentUpdate)

	w := m.call(t, http.MethodPost, hostA, contentItemsPath, validCreateContentBody, staffOf(tenantA))
	expectStatus(t, w, http.StatusCreated)

	// RULE 6, INVARIANT 8, AND THIS IS THE ASSERTION THAT WOULD HAVE CAUGHT THE SIX DEFECTS OF
	// 2026-09-22: the actor is the BUSINESS CODE, never `Principal.ID`. A ULID in `actor_id` names
	// nobody to the person reading the trail years later, and the two are indistinguishable on sight.
	if m.itemWriter.actor.ID != "CB-2026-7K3M9Q" {
		t.Errorf("chủ thể vết kiểm toán = %q, muốn mã cán bộ CB-2026-7K3M9Q", m.itemWriter.actor.ID)
	}
	if m.itemWriter.actor.IP == "" {
		t.Error("vết kiểm toán thiếu địa chỉ IP (luật 6, bất biến 2)")
	}
	// The body is DECODED INTO THE REQUEST, not passed through: a handler that dropped `publish`
	// would compose an article nobody can see and nothing else here would notice.
	if m.itemWriter.lastCreate.Type != "tin-tuc" || !m.itemWriter.lastCreate.Publish {
		t.Errorf("thân không tới được use case: %+v", m.itemWriter.lastCreate)
	}
}

func TestUpdateContentItemTellsUnmentionedFromClearedOnTheWire(t *testing.T) {
	// THE WHOLE REASON THE PATCH BODY IS A STRUCT OF POINTERS, asserted where a client actually hits
	// it: a screen editing only the title must not clear the summary and unpublish the article.
	m := newContentServer(t)
	m.grant(tenantA, PermContentUpdate)

	expectStatus(t, m.call(t, http.MethodPatch, hostA, contentItemsPath+"/nd-001",
		`{"title":"Tiêu đề đã sửa"}`, staffOf(tenantA)), http.StatusOK)
	if m.itemWriter.lastUpdate.Title == nil || *m.itemWriter.lastUpdate.Title != "Tiêu đề đã sửa" {
		t.Errorf("tiêu đề không tới được use case: %#v", m.itemWriter.lastUpdate.Title)
	}
	for name, field := range map[string]any{
		"summary":     m.itemWriter.lastUpdate.Summary,
		"body":        m.itemWriter.lastUpdate.Body,
		"category_id": m.itemWriter.lastUpdate.CategoryID,
		"image_url":   m.itemWriter.lastUpdate.ImageURL,
		"type":        m.itemWriter.lastUpdate.Type,
	} {
		if v, ok := field.(*string); ok && v != nil {
			t.Errorf("trường %q không được nhắc mà vẫn thành con trỏ khác nil: %q", name, *v)
		}
	}
	if m.itemWriter.lastUpdate.Publish != nil {
		t.Error("`publish` không được nhắc mà vẫn thành con trỏ khác nil — sẽ gỡ bài khỏi Mini App")
	}
	if m.itemWriter.lastID != "nd-001" {
		t.Errorf("id trên đường dẫn không tới được use case: %q", m.itemWriter.lastID)
	}

	// MENTIONED AND EMPTY IS A REAL REQUEST — `— Chưa xếp danh mục —`.
	m2 := newContentServer(t)
	m2.grant(tenantA, PermContentUpdate)
	expectStatus(t, m2.call(t, http.MethodPatch, hostA, contentItemsPath+"/nd-001",
		`{"category_id":"","publish":false}`, staffOf(tenantA)), http.StatusOK)
	if m2.itemWriter.lastUpdate.CategoryID == nil || *m2.itemWriter.lastUpdate.CategoryID != "" {
		t.Errorf("`category_id: \"\"` mất nghĩa 'bỏ khỏi danh mục': %#v", m2.itemWriter.lastUpdate.CategoryID)
	}
	if m2.itemWriter.lastUpdate.Publish == nil || *m2.itemWriter.lastUpdate.Publish {
		t.Errorf("`publish: false` mất nghĩa 'gỡ khỏi Mini App': %#v", m2.itemWriter.lastUpdate.Publish)
	}
}

func TestContentItemNotFoundIs404(t *testing.T) {
	m := newContentServer(t)
	m.grant(tenantA, PermContentRead, PermContentUpdate)
	m.items.err = commsstore.ErrContentItemNotFound
	m.itemWriter.err = commsstore.ErrContentItemNotFound

	expectStatus(t, m.call(t, http.MethodGet, hostA, contentItemsPath+"/nd-cua-xa-khac", "", staffOf(tenantA)),
		http.StatusNotFound)
	expectStatus(t, m.call(t, http.MethodPatch, hostA, contentItemsPath+"/nd-cua-xa-khac",
		validUpdateContentBody, staffOf(tenantA)), http.StatusNotFound)
}

func TestCreateContentItemMissingCategoryIs409(t *testing.T) {
	// 409 AND NOT 400: the body is well-formed and the caller holds the permission. What is refused
	// is this value against the state of the data — usually a screen left open while a colleague
	// retired the category.
	m := newContentServer(t)
	m.grant(tenantA, PermContentUpdate)
	m.itemWriter.err = commsstore.ErrContentCategoryNotFound

	w := m.call(t, http.MethodPost, hostA, contentItemsPath, validCreateContentBody, staffOf(tenantA))
	expectStatus(t, w, http.StatusConflict)
	if decodeError(t, w).Code != "category_missing" {
		t.Errorf("mã lỗi = %q, muốn category_missing", decodeError(t, w).Code)
	}
}

func TestCreateCategoryTakenSlugIs409AndSaysWhy(t *testing.T) {
	m := newContentServer(t)
	m.grant(tenantA, PermContentUpdate)
	m.categoryWriter.err = commsstore.ErrCategorySlugTaken

	w := m.call(t, http.MethodPost, hostA, contentCategoriesPath, validCreateCategoryBody, staffOf(tenantA))
	expectStatus(t, w, http.StatusConflict)
	e := decodeError(t, w)
	if e.Code != "code_taken" {
		t.Errorf("mã lỗi = %q, muốn code_taken", e.Code)
	}
	// THE SENTENCE HAS TO SAY WHY a slug that is nowhere on the screen is nonetheless taken: a
	// soft-deleted row keeps its code forever. Without that, this reads as a bug.
	if !strings.Contains(e.Message, "xoá") {
		t.Errorf("thông điệp không giải thích vì sao slug bị chiếm: %q", e.Message)
	}
}

func TestCreateCategoryMissingParentIs409(t *testing.T) {
	m := newContentServer(t)
	m.grant(tenantA, PermContentUpdate)
	m.categoryWriter.err = app.ErrParentCategoryNotFound

	w := m.call(t, http.MethodPost, hostA, contentCategoriesPath,
		`{"name":"Mục con","slug":"muc-con","parent_id":"dm-da-xoa"}`, staffOf(tenantA))
	expectStatus(t, w, http.StatusConflict)
	if decodeError(t, w).Code != "parent_missing" {
		t.Errorf("mã lỗi = %q, muốn parent_missing", decodeError(t, w).Code)
	}
}

func TestContentItemBadShapeIs400(t *testing.T) {
	m := newContentServer(t)
	m.grant(tenantA, PermContentUpdate)
	m.itemWriter.err = domain.ErrInvalidURL

	w := m.call(t, http.MethodPost, hostA, contentItemsPath,
		`{"type":"tin-tuc","title":"T","image_url":"javascript:alert(1)"}`, staffOf(tenantA))
	expectStatus(t, w, http.StatusBadRequest)
	if decodeError(t, w).Code != "invalid_request" {
		t.Errorf("mã lỗi = %q, muốn invalid_request", decodeError(t, w).Code)
	}
}

func TestContentItemStoreErrorDoesNotLeakTitleOrBody(t *testing.T) {
	// Rule 3, forbidden #3: an article's title and body are free text that a commune's news routinely
	// spends on residents. A store failure must not carry any of it back to the client.
	m := newContentServer(t)
	m.grant(tenantA, PermContentUpdate)
	m.itemWriter.err = errors.New("pq: duplicate key on tieu_de 'Trao quà cho gia đình ông Nguyễn Văn A'")

	w := m.call(t, http.MethodPost, hostA, contentItemsPath, validCreateContentBody, staffOf(tenantA))
	expectStatus(t, w, http.StatusInternalServerError)
	for _, banned := range []string{"Nguyễn Văn A", "pq:", "tieu_de"} {
		if strings.Contains(w.Body.String(), banned) {
			t.Errorf("lỗi kho lọt ra client (%q): %s", banned, w.Body.String())
		}
	}
}

func TestListContentCategoriesReturnsFlatTreeWithNameField(t *testing.T) {
	m := newContentServer(t)
	m.grant(tenantA, PermContentRead)
	m.categories.out = []domain.ContentCategory{
		{ID: "dm-goc", Name: "Danh mục", Slug: "danh-muc", SortOrder: 1, CreatedAt: sampleContentTime},
		{ID: "dm-001", Name: "Chuyển đổi số", Slug: "chuyen-doi-so", ParentID: "dm-goc", SortOrder: 2, CreatedAt: sampleContentTime},
	}

	w := m.call(t, http.MethodGet, hostA, contentCategoriesPath, "", staffOf(tenantA))
	expectStatus(t, w, http.StatusOK)

	var out danhSachDanhMucRa
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("thân không phải JSON: %v", err)
	}
	if len(out.Items) != 2 {
		t.Fatalf("số danh mục = %d, muốn 2", len(out.Items))
	}
	// THE FIELD IS `name` AND NOT `label`, and the line is drawn at the SCHEMA: the column is `ten`,
	// a NAME the commune gave a category of its own, which is the `bo_phan.ten` side of that line.
	if !strings.Contains(w.Body.String(), `"name"`) || strings.Contains(w.Body.String(), `"label"`) {
		t.Errorf("hợp đồng danh mục phải trả `name`, không phải `label`: %s", w.Body.String())
	}
	// THE TREE IS FLAT, with `parent_id` on each row: §7 draws a select and §3 draws an indented
	// list, and the two want different shapes of the same rows.
	if out.Items[1].ParentID != "dm-goc" || out.Items[0].ParentID != "" {
		t.Errorf("cây phải trả phẳng kèm parent_id: %+v", out.Items)
	}
}

func TestListContentCategoriesOverCeilingIs500NotTruncated(t *testing.T) {
	// REFUSED RATHER THAN TRUNCATED. A silently short tree is a category that has disappeared from
	// §7's select, so articles get filed under the wrong one and the screen looks entirely normal.
	m := newContentServer(t)
	m.grant(tenantA, PermContentRead)
	m.categories.err = commsstore.ErrTooManyContentCategories

	expectStatus(t, m.call(t, http.MethodGet, hostA, contentCategoriesPath, "", staffOf(tenantA)),
		http.StatusInternalServerError)
}

func TestCreateContentItemWithoutIdempotencyKeyIsRefused(t *testing.T) {
	// BOTH POST ROUTES DECLARE idem.Required, so the header is part of the contract rather than a
	// suggestion. This is the only case in this file that sends a POST WITHOUT it.
	m := newContentServer(t)
	m.grant(tenantA, PermContentUpdate)

	r := httptest.NewRequest(http.MethodPost, "https://"+hostA+contentItemsPath,
		strings.NewReader(validCreateContentBody))
	r.Host = hostA
	r.RemoteAddr = "10.0.0.7:51000"
	r.Header.Set("Content-Type", "application/json")
	r = r.WithContext(authz.Into(r.Context(), *staffOf(tenantA)))
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)

	if w.Code == http.StatusCreated {
		t.Fatalf("thiếu Idempotency-Key mà vẫn tạo được: %d", w.Code)
	}
	if m.itemWriter.total() != 0 {
		t.Error("thiếu Idempotency-Key mà vẫn chạm tới use case")
	}
}
