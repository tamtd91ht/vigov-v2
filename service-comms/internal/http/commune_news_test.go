package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// GET /api/v1/commune-news and /api/v1/commune-news/{id} — the public Mini App reads.
//
// WHICH ROWS the store returns (only `dang-hien`, only this commune, not soft-deleted) is the store's
// predicate, proven in internal/store/content_item_public_test.go and _pg_test.go. The fakes here are
// deliberately RAW — they hand back whatever the commune holds, drafts included — so what is proven here
// is the handler's own half: the commune the host resolves to is the only one read, the second wall
// (IsVisibleToCitizens) drops anything unpublished, no markup leaves, every negative answers one shape.
//
// Rule 5 invariant 7 adapted to Public: no principal, so no 401/403; "right permission, wrong commune"
// becomes "commune A's host never returns commune B's items, and B's id under A's host is a 404".

const (
	newsHostA        = "xa-a.vigov.vn"
	newsHostB        = "xa-b.vigov.vn"
	newsHostInactive = "xa-cu.vigov.vn"
	newsHostUnknown  = "khong-ai-co.vigov.vn"
	newsHostReserved = "admin.vigov.vn"
)

var newsDay = time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)

type fakePlatform struct {
	down  bool
	calls int
}

func (n *fakePlatform) XaTheoHost(_ context.Context, host string) (tenant.Tenant, bool, error) {
	n.calls++
	if n.down {
		return tenant.Tenant{}, false, errors.New("rpc error: code = Unavailable")
	}
	switch host {
	case newsHostA:
		return tenant.Tenant{ID: tenantA, Host: host, Name: "Xã A", Active: true}, true, nil
	case newsHostB:
		return tenant.Tenant{ID: tenantB, Host: host, Name: "Xã B", Active: true}, true, nil
	case newsHostInactive:
		// Merged: still known to the registry (rule 7), not active. Its id is B's on purpose — reading
		// it would publish a live commune's news under a domain that no longer serves it.
		return tenant.Tenant{ID: tenantB, Host: host, Active: false}, true, nil
	}
	return tenant.Tenant{}, false, nil
}

// fakePublicContent holds each commune's items RAW (every state), keyed by the commune IN THE CONTEXT.
type fakePublicContent struct {
	byTenant map[tenant.ID][]domain.ContentItem
	err      error
	calls    int

	// lastType is the `type` the handler passed on the last list read. The fake applies it (like the
	// store's `loai = $3`) but NOT the state predicate — the second wall stays the handler's to prove.
	lastType domain.ContentType
}

func (k *fakePublicContent) ListPublic(ctx context.Context, itemType domain.ContentType, _ page.Request) (page.Result[domain.ContentItem], error) {
	k.calls++
	k.lastType = itemType
	t := tenant.MustFrom(ctx)
	if k.err != nil {
		return page.Result[domain.ContentItem]{}, k.err
	}
	var items []domain.ContentItem
	for _, n := range k.byTenant[t] {
		if itemType == "" || n.Type == itemType {
			items = append(items, n)
		}
	}
	if len(items) == 0 {
		return page.NewResult[domain.ContentItem](), nil
	}
	return page.Result[domain.ContentItem]{Items: items, NextCursor: "con-tro-tiep", HasMore: true}, nil
}

func (k *fakePublicContent) PublicByID(ctx context.Context, id string) (domain.ContentItem, error) {
	k.calls++
	t := tenant.MustFrom(ctx)
	if k.err != nil {
		return domain.ContentItem{}, k.err
	}
	for _, n := range k.byTenant[t] {
		if n.ID == id {
			return n, nil
		}
	}
	return domain.ContentItem{}, commsstore.ErrContentItemNotFound
}

type fakePublicCategories struct {
	byTenant map[tenant.ID][]domain.ContentCategory
}

func (d *fakePublicCategories) List(ctx context.Context) ([]domain.ContentCategory, error) {
	return d.byTenant[tenant.MustFrom(ctx)], nil
}

const newsScript = `<p>Lịch tiêm chủng</p><script>alert("xã")</script><p>Bà con &amp; các cháu</p>` +
	`<img src=x onerror=alert(1)>&lt;script&gt;alert(2)&lt;/script&gt;`

func newsFixture() (*fakePublicContent, *fakePublicCategories) {
	items := &fakePublicContent{byTenant: map[tenant.ID][]domain.ContentItem{
		tenantA: {
			{ID: "nd-a-1", Type: domain.ContentTypeNews, Title: "<b>Tiêm chủng</b> tháng 10", Summary: "Tóm <i>tắt</i>",
				Body: newsScript, CategoryID: "dm-a", PublishedOn: newsDay, Status: domain.ContentStatusVisible,
				Source: domain.ContentSourceManual, AuthorCode: "CB-2026-7K3M9Q", ViewCount: 9, ImageURL: "https://x/a.png"},
			{ID: "nd-a-nhap", Title: "Bản nháp của xã A", Status: domain.ContentStatusHidden, PublishedOn: newsDay},
			{ID: "nd-a-cho", Title: "Chờ duyệt của xã A", Status: domain.ContentStatusPending, PublishedOn: newsDay},
		},
		tenantB: {
			{ID: "nd-b-1", Title: "Tin của xã B", Body: "Toàn văn xã B", PublishedOn: newsDay,
				Status: domain.ContentStatusVisible},
		},
	}}
	categories := &fakePublicCategories{byTenant: map[tenant.ID][]domain.ContentCategory{
		tenantA: {{ID: "dm-a", Name: "Y tế"}},
		// The SAME id in B with another name: a lookup that crossed communes would print it.
		tenantB: {{ID: "dm-a", Name: "DANH MỤC CỦA XÃ B"}},
	}}
	return items, categories
}

func newNewsServer(t *testing.T, platform *fakePlatform, items *fakePublicContent, categories *fakePublicCategories, log *slog.Logger) http.Handler {
	t.Helper()
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	mux := http.NewServeMux()
	RegisterPublic(mux, PublicDeps{Tenants: platform, ContentItems: items, Categories: categories, Log: log})
	return mux
}

func callNews(h http.Handler, path, host string, extra ...string) *httptest.ResponseRecorder {
	q := ""
	if host != "" {
		q = "?host=" + url.QueryEscape(host)
	}
	for _, x := range extra {
		q += x
	}
	r := httptest.NewRequest(http.MethodGet, "https://comms.api.vigov.vn"+path+q, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

type newsPage struct {
	Items      []map[string]any `json:"items"`
	NextCursor string           `json:"next_cursor"`
	HasMore    bool             `json:"has_more"`
}

func decodeNewsPage(t *testing.T, w *httptest.ResponseRecorder) newsPage {
	t.Helper()
	var out newsPage
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	return out
}

// assertNoHTML is the property every public answer must hold.
func assertNoHTML(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	// Decode first: encoding/json escapes `<` as <, so the raw bytes would hide a tag.
	var v any
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("mã hoá lại JSON: %v", err)
	}
	s := strings.NewReplacer(`<`, "<", `>`, ">", `&`, "&").Replace(string(b))
	for _, banned := range []string{"<script", "</script", "<p>", "<b>", "<i>", "<img", "onerror", "&lt;", "&amp;"} {
		if strings.Contains(strings.ToLower(s), banned) {
			t.Fatalf("phản hồi công khai còn %q: %s", banned, s)
		}
	}
}

// --- list: 200 --------------------------------------------------------------------------------------

func TestCommuneNewsOnlyPublishedOfTheRightCommune(t *testing.T) {
	items, categories := newsFixture()
	h := newNewsServer(t, &fakePlatform{}, items, categories, nil)

	w := callNews(h, CommuneNewsPath, newsHostA)
	expectStatus(t, w, http.StatusOK)
	out := decodeNewsPage(t, w)
	if len(out.Items) != 1 || out.Items[0]["id"] != "nd-a-1" {
		t.Fatalf("xã A nhận %v, muốn đúng [nd-a-1] — bản nháp và chờ duyệt không bao giờ ra ngoài", out.Items)
	}
	if out.NextCursor != "con-tro-tiep" || !out.HasMore {
		t.Fatalf("con trỏ bị đánh rơi: %+v", out)
	}
	body := w.Body.String()
	for _, banned := range []string{"Bản nháp", "Chờ duyệt", "Tin của xã B", "DANH MỤC CỦA XÃ B", string(tenantA), string(tenantB)} {
		if strings.Contains(body, banned) {
			t.Fatalf("phản hồi xã A chứa %q: %s", banned, body)
		}
	}

	wb := callNews(h, CommuneNewsPath, newsHostB)
	expectStatus(t, wb, http.StatusOK)
	if pb := decodeNewsPage(t, wb); len(pb.Items) != 1 || pb.Items[0]["id"] != "nd-b-1" {
		t.Fatalf("xã B nhận %v, muốn đúng [nd-b-1]", pb.Items)
	}
}

func TestCommuneNewsReturnsOnlyContractFieldsAsPlainText(t *testing.T) {
	items, categories := newsFixture()
	h := newNewsServer(t, &fakePlatform{}, items, categories, nil)

	w := callNews(h, CommuneNewsPath, newsHostA)
	expectStatus(t, w, http.StatusOK)
	assertNoHTML(t, w)
	it := decodeNewsPage(t, w).Items[0]

	var keys []string
	for k := range it {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	// No body on the list; no status, author, portal id, view count, image or ids of anything else.
	// `source_url` is absent because the row has none.
	if got := strings.Join(keys, ","); got != "category_name,id,published_on,source,summary,title,type" {
		t.Fatalf("trường của trang = %s", got)
	}
	if it["title"] != "Tiêm chủng tháng 10" || it["summary"] != "Tóm tắt" || it["type"] != "tin-tuc" ||
		it["source"] != "thu-cong" || it["category_name"] != "Y tế" || it["published_on"] != "2026-09-27" {
		t.Fatalf("dòng = %v", it)
	}
}

// --- detail -------------------------------------------------------------------------------------

func TestCommuneNewsDetailBodyIsPlainText(t *testing.T) {
	items, categories := newsFixture()
	h := newNewsServer(t, &fakePlatform{}, items, categories, nil)

	w := callNews(h, CommuneNewsPath+"/nd-a-1", newsHostA)
	expectStatus(t, w, http.StatusOK)
	assertNoHTML(t, w)
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	if out["body"] != "Lịch tiêm chủng\n\nBà con & các cháu" {
		t.Fatalf("toàn văn = %q", out["body"])
	}
	var keys []string
	for k := range out {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if got := strings.Join(keys, ","); got != "body,category_name,id,published_on,source,summary,title,type" {
		t.Fatalf("trường của chi tiết = %s", got)
	}
}

// --- the `type` filter and the provenance link (added 2026-09-29, SRS M6.1.4) ----------------------

// newsByType is commune A holding every case the filter must separate, and commune B holding the same
// type, so a filter that crossed communes shows up.
func newsByType() *fakePublicContent {
	return &fakePublicContent{byTenant: map[tenant.ID][]domain.ContentItem{
		tenantA: {
			{ID: "a-news", Type: domain.ContentTypeNews, Title: "Tin tức A", PublishedOn: newsDay, Status: domain.ContentStatusVisible},
			{ID: "a-event", Type: domain.ContentTypeEvent, Title: "Sự kiện A", PublishedOn: newsDay, Status: domain.ContentStatusVisible},
			{ID: "a-event-draft", Type: domain.ContentTypeEvent, Title: "Sự kiện nháp A", PublishedOn: newsDay, Status: domain.ContentStatusHidden},
			{ID: "a-event-pending", Type: domain.ContentTypeEvent, Title: "Sự kiện chờ A", PublishedOn: newsDay, Status: domain.ContentStatusPending},
		},
		tenantB: {
			{ID: "b-event", Type: domain.ContentTypeEvent, Title: "Sự kiện B", PublishedOn: newsDay, Status: domain.ContentStatusVisible},
		},
	}}
}

func TestPublicNewsTypeFilterReturnsOnlyPublishedOfThatTypeInThatCommune(t *testing.T) {
	items := newsByType()
	h := newNewsServer(t, &fakePlatform{}, items, &fakePublicCategories{}, nil)

	w := callNews(h, CommuneNewsPath, newsHostA, "&type=su-kien")
	expectStatus(t, w, http.StatusOK)
	if items.lastType != domain.ContentTypeEvent {
		t.Fatalf("kho nhận type = %q, muốn su-kien", items.lastType)
	}
	got := decodeNewsPage(t, w).Items
	if len(got) != 1 || got[0]["id"] != "a-event" || got[0]["type"] != "su-kien" {
		t.Fatalf("xã A type=su-kien nhận %v, muốn đúng [a-event]", got)
	}
	body := w.Body.String()
	for _, banned := range []string{"Tin tức A", "nháp", "chờ", "Sự kiện B", string(tenantB)} {
		if strings.Contains(body, banned) {
			t.Fatalf("phản hồi lọc loại chứa %q: %s", banned, body)
		}
	}

	// No filter, or an empty one, is every type — and the store is told so with "".
	for _, extra := range []string{"", "&type="} {
		w := callNews(h, CommuneNewsPath, newsHostA, extra)
		expectStatus(t, w, http.StatusOK)
		if items.lastType != "" || len(decodeNewsPage(t, w).Items) != 2 {
			t.Fatalf("%q: type = %q, %d mục — muốn không lọc, hai mục đã đăng", extra, items.lastType,
				len(decodeNewsPage(t, w).Items))
		}
	}
}

func TestPublicNewsUnknownTypeIs400BeforeAnyLookup(t *testing.T) {
	platform := &fakePlatform{}
	items := newsByType()
	h := newNewsServer(t, platform, items, &fakePublicCategories{}, nil)

	var first *httptest.ResponseRecorder
	for _, host := range []string{newsHostA, newsHostUnknown} {
		for _, v := range []string{"abc", "Tin-Tuc", "tin_tuc", "tin-tuc%27--"} {
			w := callNews(h, CommuneNewsPath, host, "&type="+v)
			expectStatus(t, w, http.StatusBadRequest)
			if strings.Contains(w.Body.String(), "abc") || strings.Contains(w.Body.String(), "tin_tuc") {
				t.Fatalf("400 lặp lại giá trị đã gửi: %s", w.Body.String())
			}
			// One body whatever the domain — the 400/200 split must not say which domains are communes.
			if first == nil {
				first = w
			} else if w.Body.String() != first.Body.String() {
				t.Fatalf("%s type=%s: thân 400 khác: %s / %s", host, v, w.Body.String(), first.Body.String())
			}
		}
	}
	if platform.calls != 0 || items.calls != 0 {
		t.Fatalf("type lạ: nền tảng bị hỏi %d lần, kho bị đọc %d lần", platform.calls, items.calls)
	}
}

func TestPublicNewsSourceURLOnlyWhenHTTPLink(t *testing.T) {
	items := &fakePublicContent{byTenant: map[tenant.ID][]domain.ContentItem{
		tenantA: {
			{ID: "synced", Type: domain.ContentTypeNews, Title: "Từ Cổng", PublishedOn: newsDay, Status: domain.ContentStatusVisible,
				Source: domain.ContentSourcePortalSync, SourceRef: "cong-1", SourceURL: "https://cong.xa-a.gov.vn/tin/1"},
			{ID: "bad-link", Type: domain.ContentTypeNews, Title: "Liên kết xấu", PublishedOn: newsDay, Status: domain.ContentStatusVisible,
				Source: domain.ContentSourcePortalSync, SourceRef: "cong-2", SourceURL: "javascript:alert(1)"},
		},
	}}
	h := newNewsServer(t, &fakePlatform{}, items, &fakePublicCategories{}, nil)

	for id, want := range map[string]any{"synced": "https://cong.xa-a.gov.vn/tin/1", "bad-link": nil} {
		w := callNews(h, CommuneNewsPath+"/"+id, newsHostA)
		expectStatus(t, w, http.StatusOK)
		var got map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("thân không phải JSON: %q", w.Body.String())
		}
		if got["source_url"] != want || got["source"] != "dong-bo-cong" {
			t.Fatalf("%s: source_url = %v, source = %v; muốn %v, dong-bo-cong", id, got["source_url"], got["source"], want)
		}
		// The portal's own id is internal and never leaves.
		if _, ok := got["source_ref"]; ok || strings.Contains(w.Body.String(), "cong-1") {
			t.Fatalf("%s: lộ mã bài trên Cổng: %s", id, w.Body.String())
		}
	}
}

func TestCommuneNewsDetailOne404ForEveryAbsentCase(t *testing.T) {
	// Another commune's id, a draft, awaiting approval, nonexistent, an overlong id, and a domain no
	// active commune holds: ONE answer, byte for byte.
	items, categories := newsFixture()
	h := newNewsServer(t, &fakePlatform{}, items, categories, nil)

	var reference *httptest.ResponseRecorder
	for _, c := range []struct{ id, host string }{
		{"nd-b-1", newsHostA},    // commune B's published item under A's domain
		{"nd-a-nhap", newsHostA}, // draft
		{"nd-a-cho", newsHostA},  // awaiting approval
		{"khong-co", newsHostA},
		{strings.Repeat("x", 65), newsHostA},
		{"nd-b-1", newsHostInactive}, // inactive commune whose id is B's
		{"nd-a-1", newsHostUnknown},
		{"nd-a-1", newsHostReserved},
	} {
		w := callNews(h, CommuneNewsPath+"/"+c.id, c.host)
		if reference == nil {
			reference = w
			expectStatus(t, w, http.StatusNotFound)
			continue
		}
		if w.Code != reference.Code || w.Body.String() != reference.Body.String() {
			t.Fatalf("%s @ %s trả %d %s, khác %d %s", c.id, c.host, w.Code, w.Body.String(), reference.Code, reference.Body.String())
		}
	}
}

// --- one shape for every negative on the list ------------------------------------------------------

func TestCommuneNewsUnknownReservedOrInactiveDomainLooksLikeEmptyCommune(t *testing.T) {
	// Commune A is ACTIVE and has published nothing: the byte-identical reference. If the three
	// negatives differ from it, the route answers "is this domain a commune".
	items := &fakePublicContent{byTenant: map[tenant.ID][]domain.ContentItem{}}
	h := newNewsServer(t, &fakePlatform{}, items, &fakePublicCategories{}, nil)

	reference := callNews(h, CommuneNewsPath, newsHostA)
	expectStatus(t, reference, http.StatusOK)
	if strings.TrimSpace(reference.Body.String()) != `{"items":[],"next_cursor":"","has_more":false}` {
		t.Fatalf("trang rỗng = %s", reference.Body.String())
	}
	callsBefore := items.calls
	for _, host := range []string{newsHostUnknown, newsHostReserved, newsHostInactive} {
		w := callNews(h, CommuneNewsPath, host)
		if w.Code != reference.Code || w.Body.String() != reference.Body.String() {
			t.Fatalf("%s trả %d %s, khác %d %s", host, w.Code, w.Body.String(), reference.Code, reference.Body.String())
		}
	}
	if items.calls != callsBefore {
		t.Fatalf("kho nội dung bị đọc %d lần cho tên miền không thuộc xã đang hoạt động nào", items.calls-callsBefore)
	}
}

// --- 400 / 503 before any read ----------------------------------------------------------------------

func TestCommuneNewsMalformedHostIs400WithoutAskingPlatformOrStore(t *testing.T) {
	platform := &fakePlatform{}
	items, categories := newsFixture()
	h := newNewsServer(t, platform, items, categories, nil)
	for _, path := range []string{CommuneNewsPath, CommuneNewsPath + "/nd-a-1"} {
		for _, q := range []string{"", "Xa-A.vigov.vn", "https://xa-a.vigov.vn", "xa-a.vigov.vn:443",
			"current", "xa_a.vigov.vn", "xã.vigov.vn", "10.0.0.1"} {
			if w := callNews(h, path, q); w.Code != http.StatusBadRequest {
				t.Errorf("%s host=%q: mã = %d, muốn 400", path, q, w.Code)
			}
		}
		if w := callNews(h, path, newsHostA, "&host="+newsHostB); w.Code != http.StatusBadRequest {
			t.Errorf("%s host lặp: mã = %d, muốn 400", path, w.Code)
		}
	}
	if platform.calls != 0 || items.calls != 0 {
		t.Fatalf("host sai hình dạng: nền tảng bị hỏi %d lần, kho bị đọc %d lần", platform.calls, items.calls)
	}
}

func TestCommuneNewsBadCursorIs400BeforeAskingPlatform(t *testing.T) {
	// A bad cursor is a 400 WHATEVER the domain — asked before the platform, so the 400/200 split says
	// nothing about which domains are communes.
	platform := &fakePlatform{}
	items, categories := newsFixture()
	h := newNewsServer(t, platform, items, categories, nil)
	for _, host := range []string{newsHostA, newsHostUnknown} {
		if w := callNews(h, CommuneNewsPath, host, "&cursor=khong-phai-con-tro"); w.Code != http.StatusBadRequest {
			t.Fatalf("%s: con trỏ hỏng mã = %d, muốn 400", host, w.Code)
		}
	}
	if platform.calls != 0 {
		t.Fatalf("con trỏ hỏng mà nền tảng vẫn bị hỏi %d lần", platform.calls)
	}
}

func TestCommuneNewsPlatformDownIs503WithoutReadingStore(t *testing.T) {
	items, categories := newsFixture()
	h := newNewsServer(t, &fakePlatform{down: true}, items, categories, nil)
	for _, path := range []string{CommuneNewsPath, CommuneNewsPath + "/nd-a-1"} {
		w := callNews(h, path, newsHostA)
		expectStatus(t, w, http.StatusServiceUnavailable)
		if strings.Contains(w.Body.String(), "items") {
			t.Fatalf("%s: 503 mang hình dạng danh sách: %s", path, w.Body.String())
		}
	}
	if items.calls != 0 {
		t.Fatalf("nền tảng chết mà kho vẫn bị đọc %d lần", items.calls)
	}
}

// --- 500, and nothing from an article in the log ----------------------------------------------------

func TestCommuneNewsStoreErrorIs500WithoutLeaking(t *testing.T) {
	var logged strings.Builder
	log := slog.New(slog.NewJSONHandler(&logged, nil))
	h := newNewsServer(t, &fakePlatform{}, &fakePublicContent{err: errors.New("pq: connection reset")}, &fakePublicCategories{}, log)

	for _, path := range []string{CommuneNewsPath, CommuneNewsPath + "/nd-a-1"} {
		w := callNews(h, path, newsHostA)
		expectStatus(t, w, http.StatusInternalServerError)
		if strings.Contains(w.Body.String(), "pq:") {
			t.Fatalf("%s: thân 500 lộ chi tiết: %s", path, w.Body.String())
		}
	}
	if logged.Len() == 0 {
		t.Fatal("lỗi kho không để lại dòng nhật ký nào")
	}
}

func TestCommuneNewsSecondWallDropsUnpublishedAndLogs(t *testing.T) {
	// The raw fake returns drafts; that the list above shows none is the second wall. Here: it logs,
	// and the log names no title.
	var logged strings.Builder
	log := slog.New(slog.NewJSONHandler(&logged, nil))
	items, categories := newsFixture()
	h := newNewsServer(t, &fakePlatform{}, items, categories, log)

	expectStatus(t, callNews(h, CommuneNewsPath, newsHostA), http.StatusOK)
	if !strings.Contains(logged.String(), "CHƯA ĐĂNG") {
		t.Fatalf("mục chưa đăng bị bỏ mà không báo: %s", logged.String())
	}
	if strings.Contains(logged.String(), "Bản nháp") {
		t.Fatalf("nhật ký chứa tiêu đề bài: %s", logged.String())
	}
}

// --- the literal routes and the exported constant agree ---------------------------------------------

func TestCommuneNewsPathMatchesRegisteredRoutes(t *testing.T) {
	items, categories := newsFixture()
	mux := http.NewServeMux()
	RegisterPublic(mux, PublicDeps{Tenants: &fakePlatform{}, ContentItems: items, Categories: categories})
	for p, want := range map[string]string{
		CommuneNewsPath:          "GET " + CommuneNewsPath,
		CommuneNewsPath + "/abc": "GET " + CommuneNewsPath + "/{id}",
	} {
		if _, pattern := mux.Handler(httptest.NewRequest(http.MethodGet, p, nil)); pattern != want {
			t.Errorf("%s khớp %q, muốn %q", p, pattern, want)
		}
	}
}

func TestRegisterPublicPanicsOnMissingStore(t *testing.T) {
	items, categories := newsFixture()
	for name, d := range map[string]PublicDeps{
		"thiếu nền tảng": {ContentItems: items, Categories: categories},
		"thiếu nội dung": {Tenants: &fakePlatform{}, Categories: categories},
		"thiếu danh mục": {Tenants: &fakePlatform{}, ContentItems: items},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: dựng tuyến công khai vẫn chạy", name)
				}
			}()
			RegisterPublic(http.NewServeMux(), d)
		}()
	}
}
