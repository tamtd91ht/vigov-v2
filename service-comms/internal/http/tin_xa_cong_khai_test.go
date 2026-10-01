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
// predicate, proven in internal/store/noi_dung_cong_khai_test.go and _pg_test.go. The fakes here are
// deliberately RAW — they hand back whatever the commune holds, drafts included — so what is proven here
// is the handler's own half: the commune the host resolves to is the only one read, the second wall
// (HienChoDan) drops anything unpublished, no markup leaves, every negative answers one shape.
//
// Rule 5 invariant 7 adapted to Public: no principal, so no 401/403; "right permission, wrong commune"
// becomes "commune A's host never returns commune B's items, and B's id under A's host is a 404".

const (
	ckHostA       = "xa-a.vigov.vn"
	ckHostB       = "xa-b.vigov.vn"
	ckHostNgung   = "xa-cu.vigov.vn"
	ckHostKhongCo = "khong-ai-co.vigov.vn"
	ckHostRieng   = "admin.vigov.vn"
)

var ckNgay = time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)

type ckNenTang struct {
	chet bool
	goi  int
}

func (n *ckNenTang) XaTheoHost(_ context.Context, host string) (tenant.Tenant, bool, error) {
	n.goi++
	if n.chet {
		return tenant.Tenant{}, false, errors.New("rpc error: code = Unavailable")
	}
	switch host {
	case ckHostA:
		return tenant.Tenant{ID: xaA, Host: host, Name: "Xã A", Active: true}, true, nil
	case ckHostB:
		return tenant.Tenant{ID: xaB, Host: host, Name: "Xã B", Active: true}, true, nil
	case ckHostNgung:
		// Merged: still known to the registry (rule 7), not active. Its id is B's on purpose — reading
		// it would publish a live commune's news under a domain that no longer serves it.
		return tenant.Tenant{ID: xaB, Host: host, Active: false}, true, nil
	}
	return tenant.Tenant{}, false, nil
}

// ckNoiDung holds each commune's items RAW (every state), keyed by the commune IN THE CONTEXT.
type ckNoiDung struct {
	theoXa map[tenant.ID][]domain.NoiDungMiniApp
	loi    error
	goi    int

	// lastType is the `type` the handler passed on the last list read. The fake applies it (like the
	// store's `loai = $3`) but NOT the state predicate — the second wall stays the handler's to prove.
	lastType domain.LoaiNoiDung

	// lastCategory is the `category` passed on the last list read. The fake applies it the way the
	// store's recursive CTE does — the category and its live descendants IN THE CONTEXT'S COMMUNE, read
	// from `tree` — so "parent includes children" and "another commune's id is empty" are visible here.
	lastCategory string
	tree         *ckDanhMuc

	// bannerLimit is the limit the handler passed on the last banner-strip read.
	bannerLimit int
}

// subtree is the fake's publicCategorySubtree: ids reachable from root through the commune's live tree.
func (k *ckNoiDung) subtree(xa tenant.ID, root string) map[string]bool {
	in := map[string]bool{}
	if k.tree == nil {
		return in
	}
	live := k.tree.theoXa[xa]
	for _, c := range live {
		if c.ID == root {
			in[root] = true
		}
	}
	for grew := true; grew; {
		grew = false
		for _, c := range live {
			if in[c.ChaID] && !in[c.ID] {
				in[c.ID], grew = true, true
			}
		}
	}
	return in
}

func (k *ckNoiDung) DanhSachCongKhai(ctx context.Context, itemType domain.LoaiNoiDung, categoryID string, _ page.Request) (page.Result[domain.NoiDungMiniApp], error) {
	k.goi++
	k.lastType = itemType
	k.lastCategory = categoryID
	xa := tenant.MustFrom(ctx)
	if k.loi != nil {
		return page.Result[domain.NoiDungMiniApp]{}, k.loi
	}
	var in map[string]bool
	if categoryID != "" {
		in = k.subtree(xa, categoryID)
	}
	var items []domain.NoiDungMiniApp
	for _, n := range k.theoXa[xa] {
		// The store's type predicate: "" = every type but banner (ADR 0067 §5 decision 5).
		typeOK := n.Loai == itemType || (itemType == "" && n.Loai != domain.LoaiBanner)
		if typeOK && (in == nil || in[n.DanhMucID]) {
			items = append(items, n)
		}
	}
	if len(items) == 0 {
		return page.NewResult[domain.NoiDungMiniApp](), nil
	}
	return page.Result[domain.NoiDungMiniApp]{Items: items, NextCursor: "con-tro-tiep", HasMore: true}, nil
}

func (k *ckNoiDung) CongKhaiTheoID(ctx context.Context, id string) (domain.NoiDungMiniApp, error) {
	k.goi++
	xa := tenant.MustFrom(ctx)
	if k.loi != nil {
		return domain.NoiDungMiniApp{}, k.loi
	}
	for _, n := range k.theoXa[xa] {
		if n.ID == id {
			return n, nil
		}
	}
	return domain.NoiDungMiniApp{}, commsstore.ErrNoiDungKhongTonTai
}

// PublishedCategoryIDs applies the STORE's predicate (dang-hien, filed under a live category of this
// commune, type when given): unlike the list there is no second wall for it in the handler.
func (k *ckNoiDung) PublishedCategoryIDs(ctx context.Context, itemType domain.LoaiNoiDung) ([]string, error) {
	k.goi++
	k.lastType = itemType
	xa := tenant.MustFrom(ctx)
	if k.loi != nil {
		return nil, k.loi
	}
	live := map[string]bool{}
	if k.tree != nil {
		for _, c := range k.tree.theoXa[xa] {
			live[c.ID] = true
		}
	}
	seen := map[string]bool{}
	var ids []string
	for _, n := range k.theoXa[xa] {
		if n.TrangThai == domain.TrangThaiDangHien && live[n.DanhMucID] && !seen[n.DanhMucID] &&
			(n.Loai == itemType || (itemType == "" && n.Loai != domain.LoaiBanner)) {
			seen[n.DanhMucID] = true
			ids = append(ids, n.DanhMucID)
		}
	}
	return ids, nil
}

// PublicBanners returns the commune's `banner` rows RAW — every state, with or without a cover — in the
// STORE's order (display_order ASC NULLS LAST, id). The state and cover predicates are the store's
// (proven in internal/store); the handler's own walls — published only, a published picture only — are
// what the tests over this fake prove.
func (k *ckNoiDung) PublicBanners(ctx context.Context, limit int) ([]domain.NoiDungMiniApp, error) {
	k.goi++
	k.bannerLimit = limit
	if k.loi != nil {
		return nil, k.loi
	}
	var out []domain.NoiDungMiniApp
	for _, n := range k.theoXa[tenant.MustFrom(ctx)] {
		if n.Loai == domain.LoaiBanner {
			out = append(out, n)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].DisplayOrder, out[j].DisplayOrder
		switch {
		case a == nil && b == nil:
			return out[i].ID < out[j].ID
		case a == nil:
			return false
		case b == nil:
			return true
		case *a != *b:
			return *a < *b
		}
		return out[i].ID < out[j].ID
	})
	if len(out) > limit+1 {
		out = out[:limit+1]
	}
	return out, nil
}

type ckDanhMuc struct {
	theoXa map[tenant.ID][]domain.DanhMucMiniApp
	loi    error
	goi    int
}

func (d *ckDanhMuc) DanhSach(ctx context.Context) ([]domain.DanhMucMiniApp, error) {
	d.goi++
	if d.loi != nil {
		return nil, d.loi
	}
	return d.theoXa[tenant.MustFrom(ctx)], nil
}

const ckScript = `<p>Lịch tiêm chủng</p><script>alert("xã")</script><p>Bà con &amp; các cháu</p>` +
	`<img src=x onerror=alert(1)>&lt;script&gt;alert(2)&lt;/script&gt;`

func ckDuLieu() (*ckNoiDung, *ckDanhMuc) {
	nd := &ckNoiDung{theoXa: map[tenant.ID][]domain.NoiDungMiniApp{
		xaA: {
			{ID: "nd-a-1", Loai: domain.LoaiTinTuc, TieuDe: "<b>Tiêm chủng</b> tháng 10", TomTat: "Tóm <i>tắt</i>",
				NoiDung: ckScript, DanhMucID: "dm-a", NgayDang: ckNgay, TrangThai: domain.TrangThaiDangHien,
				Nguon: domain.NguonThuCong, NguoiTaoMa: "CB-2026-7K3M9Q", LuotXem: 9, AnhDaiDienURL: "https://x/a.png"},
			{ID: "nd-a-nhap", TieuDe: "Bản nháp của xã A", TrangThai: domain.TrangThaiAn, NgayDang: ckNgay},
			{ID: "nd-a-cho", TieuDe: "Chờ duyệt của xã A", TrangThai: domain.TrangThaiChoDuyet, NgayDang: ckNgay},
		},
		xaB: {
			{ID: "nd-b-1", TieuDe: "Tin của xã B", NoiDung: "Toàn văn xã B", NgayDang: ckNgay,
				TrangThai: domain.TrangThaiDangHien},
		},
	}}
	dm := &ckDanhMuc{theoXa: map[tenant.ID][]domain.DanhMucMiniApp{
		xaA: {{ID: "dm-a", Ten: "Y tế"}},
		// The SAME id in B with another name: a lookup that crossed communes would print it.
		xaB: {{ID: "dm-a", Ten: "DANH MỤC CỦA XÃ B"}},
	}}
	return nd, dm
}

func ckMayChu(t *testing.T, nt *ckNenTang, nd *ckNoiDung, dm *ckDanhMuc, log *slog.Logger) http.Handler {
	t.Helper()
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	mux := http.NewServeMux()
	RegisterCongKhai(mux, DepsCongKhai{Limiter: ckLimiter(), Xa: nt, NoiDung: nd, DanhMuc: dm, CoverImages: &fakePublicCovers{}, Audio: &fakePublicAudio{}, Log: log})
	return mux
}

// fakePublicCovers is the public cover-image read: file id → public URL, per commune, recording what
// it was asked for so a test can prove only PUBLISHED items' covers are resolved.
type fakePublicCovers struct {
	byTenant map[tenant.ID]map[string]string
	asked    []string
	tenantID tenant.ID
	err      error
}

func (a *fakePublicCovers) PublicImageURLs(ctx context.Context, ids []string) (map[string]string, error) {
	a.tenantID = tenant.MustFrom(ctx)
	a.asked = append(a.asked, ids...)
	if a.err != nil {
		return nil, a.err
	}
	out := map[string]string{}
	for _, id := range ids {
		if u, ok := a.byTenant[a.tenantID][id]; ok {
			out[id] = u
		}
	}
	return out, nil
}

func ckGoi(h http.Handler, path, host string, them ...string) *httptest.ResponseRecorder {
	q := ""
	if host != "" {
		q = "?host=" + url.QueryEscape(host)
	}
	for _, x := range them {
		q += x
	}
	r := httptest.NewRequest(http.MethodGet, "https://comms.api.vigov.vn"+path+q, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

type ckTrang struct {
	Items      []map[string]any `json:"items"`
	NextCursor string           `json:"next_cursor"`
	HasMore    bool             `json:"has_more"`
}

func ckDocTrang(t *testing.T, w *httptest.ResponseRecorder) ckTrang {
	t.Helper()
	var ra ckTrang
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	return ra
}

// ckKhongCoHTML is the property every public answer must hold.
func ckKhongCoHTML(t *testing.T, w *httptest.ResponseRecorder) {
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
	for _, cam := range []string{"<script", "</script", "<p>", "<b>", "<i>", "<img", "onerror", "&lt;", "&amp;"} {
		if strings.Contains(strings.ToLower(s), cam) {
			t.Fatalf("phản hồi công khai còn %q: %s", cam, s)
		}
	}
}

// --- list: 200 --------------------------------------------------------------------------------------

func TestTinXaChiDangHienCuaDungXa(t *testing.T) {
	nd, dm := ckDuLieu()
	h := ckMayChu(t, &ckNenTang{}, nd, dm, nil)

	w := ckGoi(h, MauTinXa, ckHostA)
	doiMa(t, w, http.StatusOK)
	ra := ckDocTrang(t, w)
	if len(ra.Items) != 1 || ra.Items[0]["id"] != "nd-a-1" {
		t.Fatalf("xã A nhận %v, muốn đúng [nd-a-1] — bản nháp và chờ duyệt không bao giờ ra ngoài", ra.Items)
	}
	if ra.NextCursor != "con-tro-tiep" || !ra.HasMore {
		t.Fatalf("con trỏ bị đánh rơi: %+v", ra)
	}
	than := w.Body.String()
	for _, cam := range []string{"Bản nháp", "Chờ duyệt", "Tin của xã B", "DANH MỤC CỦA XÃ B", string(xaA), string(xaB)} {
		if strings.Contains(than, cam) {
			t.Fatalf("phản hồi xã A chứa %q: %s", cam, than)
		}
	}

	wb := ckGoi(h, MauTinXa, ckHostB)
	doiMa(t, wb, http.StatusOK)
	if rb := ckDocTrang(t, wb); len(rb.Items) != 1 || rb.Items[0]["id"] != "nd-b-1" {
		t.Fatalf("xã B nhận %v, muốn đúng [nd-b-1]", rb.Items)
	}
}

func TestTinXaChiTraDungCacTruongVaVanBanThuan(t *testing.T) {
	nd, dm := ckDuLieu()
	h := ckMayChu(t, &ckNenTang{}, nd, dm, nil)

	w := ckGoi(h, MauTinXa, ckHostA)
	doiMa(t, w, http.StatusOK)
	ckKhongCoHTML(t, w)
	it := ckDocTrang(t, w).Items[0]

	var khoa []string
	for k := range it {
		khoa = append(khoa, k)
	}
	sort.Strings(khoa)
	// No body on the list; no status, author, portal id, view count, image or ids of anything else.
	// `source_url` is absent because the row has none.
	if got := strings.Join(khoa, ","); got != "category_name,id,published_on,source,summary,title,type" {
		t.Fatalf("trường của trang = %s", got)
	}
	if it["title"] != "Tiêm chủng tháng 10" || it["summary"] != "Tóm tắt" || it["type"] != "tin-tuc" ||
		it["source"] != "thu-cong" || it["category_name"] != "Y tế" || it["published_on"] != "2026-09-27" {
		t.Fatalf("dòng = %v", it)
	}
}

// --- detail -------------------------------------------------------------------------------------

func TestMotTinXaToanVanLaVanBanThuan(t *testing.T) {
	nd, dm := ckDuLieu()
	h := ckMayChu(t, &ckNenTang{}, nd, dm, nil)

	w := ckGoi(h, MauTinXa+"/nd-a-1", ckHostA)
	doiMa(t, w, http.StatusOK)
	ckKhongCoHTML(t, w)
	var ra map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	if ra["body"] != "Lịch tiêm chủng\n\nBà con & các cháu" {
		t.Fatalf("toàn văn = %q", ra["body"])
	}
	var khoa []string
	for k := range ra {
		khoa = append(khoa, k)
	}
	sort.Strings(khoa)
	if got := strings.Join(khoa, ","); got != "body,body_blocks,category_name,id,published_on,source,summary,title,type" {
		t.Fatalf("trường của chi tiết = %s", got)
	}
	// ADR 0067 §1: the same legacy (unsanitised) row as structure — sanitised on THIS read, the script,
	// the image and the entity-encoded markup gone (as in `body`), the two paragraphs kept.
	// Re-encoded WITHOUT HTML escaping, so the comparison reads the text a client reads.
	var buf strings.Builder
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(ra["body_blocks"]); err != nil {
		t.Fatalf("mã hoá body_blocks: %v", err)
	}
	blocks := strings.TrimSpace(buf.String())
	want := `[{"kind":"paragraph","runs":[{"text":"Lịch tiêm chủng"}]},` +
		`{"kind":"paragraph","runs":[{"text":"Bà con & các cháu"}]}]`
	if string(blocks) != want {
		t.Fatalf("body_blocks =\n%s\nmuốn\n%s", blocks, want)
	}
}

// --- the `type` filter and the provenance link (added 2026-09-29, SRS M6.1.4) ----------------------

// newsByType is commune A holding every case the filter must separate, and commune B holding the same
// type, so a filter that crossed communes shows up.
func newsByType() *ckNoiDung {
	return &ckNoiDung{theoXa: map[tenant.ID][]domain.NoiDungMiniApp{
		xaA: {
			{ID: "a-news", Loai: domain.LoaiTinTuc, TieuDe: "Tin tức A", NgayDang: ckNgay, TrangThai: domain.TrangThaiDangHien},
			{ID: "a-event", Loai: domain.LoaiSuKien, TieuDe: "Sự kiện A", NgayDang: ckNgay, TrangThai: domain.TrangThaiDangHien},
			{ID: "a-event-draft", Loai: domain.LoaiSuKien, TieuDe: "Sự kiện nháp A", NgayDang: ckNgay, TrangThai: domain.TrangThaiAn},
			{ID: "a-event-pending", Loai: domain.LoaiSuKien, TieuDe: "Sự kiện chờ A", NgayDang: ckNgay, TrangThai: domain.TrangThaiChoDuyet},
		},
		xaB: {
			{ID: "b-event", Loai: domain.LoaiSuKien, TieuDe: "Sự kiện B", NgayDang: ckNgay, TrangThai: domain.TrangThaiDangHien},
		},
	}}
}

func TestPublicNewsTypeFilterReturnsOnlyPublishedOfThatTypeInThatCommune(t *testing.T) {
	nd := newsByType()
	h := ckMayChu(t, &ckNenTang{}, nd, &ckDanhMuc{}, nil)

	w := ckGoi(h, MauTinXa, ckHostA, "&type=su-kien")
	doiMa(t, w, http.StatusOK)
	if nd.lastType != domain.LoaiSuKien {
		t.Fatalf("kho nhận type = %q, muốn su-kien", nd.lastType)
	}
	items := ckDocTrang(t, w).Items
	if len(items) != 1 || items[0]["id"] != "a-event" || items[0]["type"] != "su-kien" {
		t.Fatalf("xã A type=su-kien nhận %v, muốn đúng [a-event]", items)
	}
	body := w.Body.String()
	for _, banned := range []string{"Tin tức A", "nháp", "chờ", "Sự kiện B", string(xaB)} {
		if strings.Contains(body, banned) {
			t.Fatalf("phản hồi lọc loại chứa %q: %s", banned, body)
		}
	}

	// No filter, or an empty one, is every type — and the store is told so with "".
	for _, extra := range []string{"", "&type="} {
		w := ckGoi(h, MauTinXa, ckHostA, extra)
		doiMa(t, w, http.StatusOK)
		if nd.lastType != "" || len(ckDocTrang(t, w).Items) != 2 {
			t.Fatalf("%q: type = %q, %d mục — muốn không lọc, hai mục đã đăng", extra, nd.lastType,
				len(ckDocTrang(t, w).Items))
		}
	}
}

func TestPublicNewsUnknownTypeIs400BeforeAnyLookup(t *testing.T) {
	nt := &ckNenTang{}
	nd := newsByType()
	h := ckMayChu(t, nt, nd, &ckDanhMuc{}, nil)

	var first *httptest.ResponseRecorder
	for _, host := range []string{ckHostA, ckHostKhongCo} {
		for _, v := range []string{"abc", "Tin-Tuc", "tin_tuc", "tin-tuc%27--"} {
			w := ckGoi(h, MauTinXa, host, "&type="+v)
			doiMa(t, w, http.StatusBadRequest)
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
	if nt.goi != 0 || nd.goi != 0 {
		t.Fatalf("type lạ: nền tảng bị hỏi %d lần, kho bị đọc %d lần", nt.goi, nd.goi)
	}
}

func TestPublicNewsSourceURLOnlyWhenHTTPLink(t *testing.T) {
	nd := &ckNoiDung{theoXa: map[tenant.ID][]domain.NoiDungMiniApp{
		xaA: {
			{ID: "synced", Loai: domain.LoaiTinTuc, TieuDe: "Từ Cổng", NgayDang: ckNgay, TrangThai: domain.TrangThaiDangHien,
				Nguon: domain.NguonDongBoCong, NguonIDNgoai: "cong-1", NguonURL: "https://cong.xa-a.gov.vn/tin/1"},
			{ID: "bad-link", Loai: domain.LoaiTinTuc, TieuDe: "Liên kết xấu", NgayDang: ckNgay, TrangThai: domain.TrangThaiDangHien,
				Nguon: domain.NguonDongBoCong, NguonIDNgoai: "cong-2", NguonURL: "javascript:alert(1)"},
		},
	}}
	h := ckMayChu(t, &ckNenTang{}, nd, &ckDanhMuc{}, nil)

	for id, want := range map[string]any{"synced": "https://cong.xa-a.gov.vn/tin/1", "bad-link": nil} {
		w := ckGoi(h, MauTinXa+"/"+id, ckHostA)
		doiMa(t, w, http.StatusOK)
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

func TestMotTinXaMotCau404ChoMoiTruongHopKhongCo(t *testing.T) {
	// Another commune's id, a draft, awaiting approval, nonexistent, an overlong id, and a domain no
	// active commune holds: ONE answer, byte for byte.
	nd, dm := ckDuLieu()
	h := ckMayChu(t, &ckNenTang{}, nd, dm, nil)

	var mau *httptest.ResponseRecorder
	for _, c := range []struct{ id, host string }{
		{"nd-b-1", ckHostA},    // commune B's published item under A's domain
		{"nd-a-nhap", ckHostA}, // draft
		{"nd-a-cho", ckHostA},  // awaiting approval
		{"khong-co", ckHostA},
		{strings.Repeat("x", 65), ckHostA},
		{"nd-b-1", ckHostNgung}, // inactive commune whose id is B's
		{"nd-a-1", ckHostKhongCo},
		{"nd-a-1", ckHostRieng},
	} {
		w := ckGoi(h, MauTinXa+"/"+c.id, c.host)
		if mau == nil {
			mau = w
			doiMa(t, w, http.StatusNotFound)
			continue
		}
		if w.Code != mau.Code || w.Body.String() != mau.Body.String() {
			t.Fatalf("%s @ %s trả %d %s, khác %d %s", c.id, c.host, w.Code, w.Body.String(), mau.Code, mau.Body.String())
		}
	}
}

// --- one shape for every negative on the list ------------------------------------------------------

func TestTinXaTenMienKhongCoDanhRiengNgungHoatDongTraTrangRongGiongXaChuaDangGi(t *testing.T) {
	// Commune A is ACTIVE and has published nothing: the byte-identical reference. If the three
	// negatives differ from it, the route answers "is this domain a commune".
	nd := &ckNoiDung{theoXa: map[tenant.ID][]domain.NoiDungMiniApp{}}
	h := ckMayChu(t, &ckNenTang{}, nd, &ckDanhMuc{}, nil)

	mau := ckGoi(h, MauTinXa, ckHostA)
	doiMa(t, mau, http.StatusOK)
	if strings.TrimSpace(mau.Body.String()) != `{"items":[],"next_cursor":"","has_more":false}` {
		t.Fatalf("trang rỗng = %s", mau.Body.String())
	}
	goiTruoc := nd.goi
	for _, host := range []string{ckHostKhongCo, ckHostRieng, ckHostNgung} {
		w := ckGoi(h, MauTinXa, host)
		if w.Code != mau.Code || w.Body.String() != mau.Body.String() {
			t.Fatalf("%s trả %d %s, khác %d %s", host, w.Code, w.Body.String(), mau.Code, mau.Body.String())
		}
	}
	if nd.goi != goiTruoc {
		t.Fatalf("kho nội dung bị đọc %d lần cho tên miền không thuộc xã đang hoạt động nào", nd.goi-goiTruoc)
	}
}

// --- 400 / 503 before any read ----------------------------------------------------------------------

func TestTinXaHostSaiHinhDangLa400KhongHoiNenTangKhongDocKho(t *testing.T) {
	nt := &ckNenTang{}
	nd, dm := ckDuLieu()
	h := ckMayChu(t, nt, nd, dm, nil)
	for _, path := range []string{MauTinXa, MauTinXa + "/nd-a-1"} {
		for _, q := range []string{"", "Xa-A.vigov.vn", "https://xa-a.vigov.vn", "xa-a.vigov.vn:443",
			"current", "xa_a.vigov.vn", "xã.vigov.vn", "10.0.0.1"} {
			if w := ckGoi(h, path, q); w.Code != http.StatusBadRequest {
				t.Errorf("%s host=%q: mã = %d, muốn 400", path, q, w.Code)
			}
		}
		if w := ckGoi(h, path, ckHostA, "&host="+ckHostB); w.Code != http.StatusBadRequest {
			t.Errorf("%s host lặp: mã = %d, muốn 400", path, w.Code)
		}
	}
	if nt.goi != 0 || nd.goi != 0 {
		t.Fatalf("host sai hình dạng: nền tảng bị hỏi %d lần, kho bị đọc %d lần", nt.goi, nd.goi)
	}
}

func TestTinXaConTroHongLa400TruocKhiHoiNenTang(t *testing.T) {
	// A bad cursor is a 400 WHATEVER the domain — asked before the platform, so the 400/200 split says
	// nothing about which domains are communes.
	nt := &ckNenTang{}
	nd, dm := ckDuLieu()
	h := ckMayChu(t, nt, nd, dm, nil)
	for _, host := range []string{ckHostA, ckHostKhongCo} {
		if w := ckGoi(h, MauTinXa, host, "&cursor=khong-phai-con-tro"); w.Code != http.StatusBadRequest {
			t.Fatalf("%s: con trỏ hỏng mã = %d, muốn 400", host, w.Code)
		}
	}
	if nt.goi != 0 {
		t.Fatalf("con trỏ hỏng mà nền tảng vẫn bị hỏi %d lần", nt.goi)
	}
}

func TestTinXaNenTangChetLa503KhongDocKho(t *testing.T) {
	nd, dm := ckDuLieu()
	h := ckMayChu(t, &ckNenTang{chet: true}, nd, dm, nil)
	for _, path := range []string{MauTinXa, MauTinXa + "/nd-a-1"} {
		w := ckGoi(h, path, ckHostA)
		doiMa(t, w, http.StatusServiceUnavailable)
		if strings.Contains(w.Body.String(), "items") {
			t.Fatalf("%s: 503 mang hình dạng danh sách: %s", path, w.Body.String())
		}
	}
	if nd.goi != 0 {
		t.Fatalf("nền tảng chết mà kho vẫn bị đọc %d lần", nd.goi)
	}
}

// --- 500, and nothing from an article in the log ----------------------------------------------------

func TestTinXaLoiKhoLa500KhongLoNoiDung(t *testing.T) {
	var nhatKy strings.Builder
	log := slog.New(slog.NewJSONHandler(&nhatKy, nil))
	h := ckMayChu(t, &ckNenTang{}, &ckNoiDung{loi: errors.New("pq: connection reset")}, &ckDanhMuc{}, log)

	for _, path := range []string{MauTinXa, MauTinXa + "/nd-a-1"} {
		w := ckGoi(h, path, ckHostA)
		doiMa(t, w, http.StatusInternalServerError)
		if strings.Contains(w.Body.String(), "pq:") {
			t.Fatalf("%s: thân 500 lộ chi tiết: %s", path, w.Body.String())
		}
	}
	if nhatKy.Len() == 0 {
		t.Fatal("lỗi kho không để lại dòng nhật ký nào")
	}
}

func TestTinXaTuongThuHaiBoMucChuaDangVaGhiNhatKy(t *testing.T) {
	// The raw fake returns drafts; that the list above shows none is the second wall. Here: it logs,
	// and the log names no title.
	var nhatKy strings.Builder
	log := slog.New(slog.NewJSONHandler(&nhatKy, nil))
	nd, dm := ckDuLieu()
	h := ckMayChu(t, &ckNenTang{}, nd, dm, log)

	doiMa(t, ckGoi(h, MauTinXa, ckHostA), http.StatusOK)
	if !strings.Contains(nhatKy.String(), "CHƯA ĐĂNG") {
		t.Fatalf("mục chưa đăng bị bỏ mà không báo: %s", nhatKy.String())
	}
	if strings.Contains(nhatKy.String(), "Bản nháp") {
		t.Fatalf("nhật ký chứa tiêu đề bài: %s", nhatKy.String())
	}
}

// --- the literal routes and the exported constant agree ---------------------------------------------

func TestMauTinXaKhopTuyenDaDangKy(t *testing.T) {
	nd, dm := ckDuLieu()
	mux := http.NewServeMux()
	RegisterCongKhai(mux, DepsCongKhai{Limiter: ckLimiter(), Xa: &ckNenTang{}, NoiDung: nd, DanhMuc: dm, CoverImages: &fakePublicCovers{}, Audio: &fakePublicAudio{}})
	for p, muon := range map[string]string{
		MauTinXa:                 "GET " + MauTinXa,
		MauTinXa + "/abc":        "GET " + MauTinXa + "/{id}",
		MauTinXa + "/categories": "GET " + MauTinXa + "/categories",
	} {
		if _, mau := mux.Handler(httptest.NewRequest(http.MethodGet, p, nil)); mau != muon {
			t.Errorf("%s khớp %q, muốn %q", p, mau, muon)
		}
	}
}

func TestRegisterCongKhaiThieuKhoThiPanic(t *testing.T) {
	nd, dm := ckDuLieu()
	for ten, d := range map[string]DepsCongKhai{
		"thiếu nền tảng": {Limiter: ckLimiter(), NoiDung: nd, DanhMuc: dm, CoverImages: &fakePublicCovers{}, Audio: &fakePublicAudio{}},
		"thiếu nội dung": {Limiter: ckLimiter(), Xa: &ckNenTang{}, DanhMuc: dm, CoverImages: &fakePublicCovers{}, Audio: &fakePublicAudio{}},
		"thiếu danh mục": {Limiter: ckLimiter(), Xa: &ckNenTang{}, NoiDung: nd, CoverImages: &fakePublicCovers{}, Audio: &fakePublicAudio{}},
		"thiếu ảnh bìa":  {Limiter: ckLimiter(), Xa: &ckNenTang{}, NoiDung: nd, DanhMuc: dm},
		"thiếu âm thanh": {Limiter: ckLimiter(), Xa: &ckNenTang{}, NoiDung: nd, DanhMuc: dm, CoverImages: &fakePublicCovers{}},
		// Rule 13 invariant 7: no public route is mounted without its rate limit.
		"thiếu giới hạn tần suất": {Xa: &ckNenTang{}, NoiDung: nd, DanhMuc: dm, CoverImages: &fakePublicCovers{}, Audio: &fakePublicAudio{}},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: dựng tuyến công khai vẫn chạy", ten)
				}
			}()
			RegisterCongKhai(http.NewServeMux(), d)
		}()
	}
}

// --- the category chips (user decision 2026-09-30) -------------------------------------------------

// chipData is commune A holding every case the chip row must separate, and commune B holding the SAME
// category id under another name, so anything that crossed communes shows up.
//
//	A: r-health (no item of its own) ─ c-vacc (a-vacc, tin-tuc) ─ g-kids (a-kids, su-kien)
//	   r-econ   (only a DRAFT)                                    → hidden
//	   r-html   ("<b>Văn hoá</b>", a-culture)                     → name as plain text
//	   items filed under r-gone (soft-deleted: absent from the live tree) → never a chip, empty list
//	B: r-health ("CỦA XÃ B", b-health)
func chipData() (*ckNoiDung, *ckDanhMuc) {
	dm := &ckDanhMuc{theoXa: map[tenant.ID][]domain.DanhMucMiniApp{
		xaA: {
			{ID: "r-health", Ten: "Y tế", Slug: "y-te", ThuTu: 1},
			{ID: "c-vacc", Ten: "Tiêm chủng", Slug: "tiem-chung", ChaID: "r-health", ThuTu: 1},
			{ID: "g-kids", Ten: "Trẻ em", Slug: "tre-em", ChaID: "c-vacc", ThuTu: 2},
			{ID: "r-econ", Ten: "Kinh tế", Slug: "kinh-te", ThuTu: 2},
			{ID: "r-html", Ten: "<b>Văn hoá</b>", Slug: "van-hoa", ThuTu: 3},
		},
		xaB: {{ID: "r-health", Ten: "CỦA XÃ B", Slug: "y-te"}},
	}}
	pub := func(id, cat string, typ domain.LoaiNoiDung) domain.NoiDungMiniApp {
		return domain.NoiDungMiniApp{ID: id, Loai: typ, DanhMucID: cat, TieuDe: "Tiêu đề " + id,
			NgayDang: ckNgay, TrangThai: domain.TrangThaiDangHien}
	}
	draft := pub("a-econ-draft", "r-econ", domain.LoaiTinTuc)
	draft.TrangThai = domain.TrangThaiAn
	nd := &ckNoiDung{tree: dm, theoXa: map[tenant.ID][]domain.NoiDungMiniApp{
		xaA: {
			pub("a-vacc", "c-vacc", domain.LoaiTinTuc),
			pub("a-kids", "g-kids", domain.LoaiSuKien),
			draft,
			pub("a-culture", "r-html", domain.LoaiTinTuc),
			pub("a-gone", "r-gone", domain.LoaiTinTuc),
			pub("a-nowhere", "", domain.LoaiTinTuc),
		},
		xaB: {pub("b-health", "r-health", domain.LoaiTinTuc)},
	}}
	return nd, dm
}

const chipPath = MauTinXa + "/categories"

func TestPublicNewsCategoriesShowsOnlyCategoriesWithPublishedItems(t *testing.T) {
	nd, dm := chipData()
	h := ckMayChu(t, &ckNenTang{}, nd, dm, nil)

	w := ckGoi(h, chipPath, ckHostA)
	doiMa(t, w, http.StatusOK)
	ckKhongCoHTML(t, w)
	// MUTATIONS THAT MUST TURN THIS RED: a parent with no item of its own dropped (r-health); an empty
	// category kept (r-econ, draft only); a soft-deleted category resurrected (r-gone); the name sent
	// with markup; `parent_id` sent on a root; the slug or the commune leaking.
	want := `{"items":[` +
		`{"id":"r-health","name":"Y tế","order":1},` +
		`{"id":"c-vacc","name":"Tiêm chủng","parent_id":"r-health","order":1},` +
		`{"id":"g-kids","name":"Trẻ em","parent_id":"c-vacc","order":2},` +
		`{"id":"r-html","name":"Văn hoá","order":3}]}`
	if got := strings.TrimSpace(w.Body.String()); got != want {
		t.Fatalf("chip xã A =\n%s\nmuốn\n%s", got, want)
	}
	for _, banned := range []string{"CỦA XÃ B", "y-te", "r-econ", "r-gone", string(xaA), string(xaB)} {
		if strings.Contains(w.Body.String(), banned) {
			t.Fatalf("chip xã A chứa %q: %s", banned, w.Body.String())
		}
	}

	// Commune B's host sees only B's tree.
	wb := ckGoi(h, chipPath, ckHostB)
	doiMa(t, wb, http.StatusOK)
	if got := strings.TrimSpace(wb.Body.String()); got != `{"items":[{"id":"r-health","name":"CỦA XÃ B","order":0}]}` {
		t.Fatalf("chip xã B = %s", got)
	}
}

func TestPublicNewsCategoriesTypeFilter(t *testing.T) {
	nd, dm := chipData()
	h := ckMayChu(t, &ckNenTang{}, nd, dm, nil)

	w := ckGoi(h, chipPath, ckHostA, "&type=su-kien")
	doiMa(t, w, http.StatusOK)
	if nd.lastType != domain.LoaiSuKien {
		t.Fatalf("kho nhận type = %q", nd.lastType)
	}
	var ra struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	var ids []string
	for _, it := range ra.Items {
		ids = append(ids, it["id"].(string))
	}
	// Only g-kids holds a su-kien; its ancestors come with it. r-html (tin-tuc only) is hidden.
	if got := strings.Join(ids, ","); got != "r-health,c-vacc,g-kids" {
		t.Fatalf("chip su-kien = %s", got)
	}

	// A type nothing is published under: an empty row, and the tree is not even read.
	goiTruoc := dm.goi
	w = ckGoi(h, chipPath, ckHostA, "&type=video")
	doiMa(t, w, http.StatusOK)
	if strings.TrimSpace(w.Body.String()) != `{"items":[]}` || dm.goi != goiTruoc {
		t.Fatalf("type=video: %s, cây bị đọc %d lần", w.Body.String(), dm.goi-goiTruoc)
	}
}

func TestPublicNewsCategoryFilterIncludesDescendantsAndAndsWithType(t *testing.T) {
	nd, dm := chipData()
	h := ckMayChu(t, &ckNenTang{}, nd, dm, nil)

	ids := func(extra string) string {
		t.Helper()
		w := ckGoi(h, MauTinXa, ckHostA, extra)
		doiMa(t, w, http.StatusOK)
		var out []string
		for _, it := range ckDocTrang(t, w).Items {
			out = append(out, it["id"].(string))
		}
		sort.Strings(out)
		return strings.Join(out, ",")
	}
	for extra, want := range map[string]string{
		"&category=r-health":              "a-kids,a-vacc", // the parent includes child and grandchild
		"&category=c-vacc":                "a-kids,a-vacc",
		"&category=g-kids":                "a-kids",
		"&category=r-health&type=tin-tuc": "a-vacc", // AND, not OR
		"&category=r-health&type=su-kien": "a-kids",
		"&category=g-kids&type=tin-tuc":   "",
		"&category=":                      "a-culture,a-gone,a-kids,a-nowhere,a-vacc", // empty = no filter
	} {
		if got := ids(extra); got != want {
			t.Errorf("%s: %q, muốn %q", extra, got, want)
		}
	}
	if ids("&category=g-kids"); nd.lastCategory != "g-kids" {
		t.Fatalf("kho nhận category = %q", nd.lastCategory)
	}
}

func TestPublicNewsCategoryUnknownDeletedOrOtherCommuneIsTheEmptyPage(t *testing.T) {
	nd, dm := chipData()
	h := ckMayChu(t, &ckNenTang{}, nd, dm, nil)

	const empty = `{"items":[],"next_cursor":"","has_more":false}`
	// A live category holding only a DRAFT: no item (the raw fake returns the draft with a cursor, so only
	// the items are compared here — the store's own predicate is the pg suite's).
	if items := ckDocTrang(t, ckGoi(h, MauTinXa, ckHostA, "&category=r-econ")).Items; len(items) != 0 {
		t.Fatalf("danh mục chỉ có bản nháp: %v", items)
	}
	for _, c := range []struct{ host, extra string }{
		{ckHostA, "&category=r-gone"},       // soft-deleted
		{ckHostA, "&category=khong-co"},     // no such id
		{ckHostB, "&category=c-vacc"},       // A's category under B's host
		{ckHostKhongCo, "&category=c-vacc"}, // no commune at all
	} {
		w := ckGoi(h, MauTinXa, c.host, c.extra)
		if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != empty {
			t.Errorf("%s %s: %d %s — muốn 200 %s", c.host, c.extra, w.Code, w.Body.String(), empty)
		}
	}
}

func TestPublicNewsCategoryParamsAre400BeforeAnyLookup(t *testing.T) {
	nt := &ckNenTang{}
	nd, dm := chipData()
	h := ckMayChu(t, nt, nd, dm, nil)

	for _, host := range []string{ckHostA, ckHostKhongCo} {
		for _, v := range []string{"a%20b", "x%3B--", "a%2Fb", "%C4%91m", strings.Repeat("x", 65)} {
			w := ckGoi(h, MauTinXa, host, "&category="+v)
			doiMa(t, w, http.StatusBadRequest)
			if strings.Contains(w.Body.String(), "--") || strings.Contains(w.Body.String(), "xxxx") {
				t.Fatalf("400 lặp lại giá trị đã gửi: %s", w.Body.String())
			}
		}
		// The categories route: a bad `type` is a 400 before the platform too.
		doiMa(t, ckGoi(h, chipPath, host, "&type=abc"), http.StatusBadRequest)
	}
	for _, q := range []string{"", "Xa-A.vigov.vn", "xa-a.vigov.vn:443", "10.0.0.1"} {
		doiMa(t, ckGoi(h, chipPath, q), http.StatusBadRequest)
	}
	doiMa(t, ckGoi(h, chipPath, ckHostA, "&host="+ckHostB), http.StatusBadRequest)
	if nt.goi != 0 || nd.goi != 0 || dm.goi != 0 {
		t.Fatalf("tham số sai: nền tảng %d, kho nội dung %d, kho danh mục %d lần", nt.goi, nd.goi, dm.goi)
	}
}

func TestPublicNewsCategoriesUnknownDomainIsByteIdenticalToEmptyCommune(t *testing.T) {
	// A is active with nothing filed: the reference.
	nd := &ckNoiDung{theoXa: map[tenant.ID][]domain.NoiDungMiniApp{}, tree: &ckDanhMuc{}}
	dm := &ckDanhMuc{}
	h := ckMayChu(t, &ckNenTang{}, nd, dm, nil)

	ref := ckGoi(h, chipPath, ckHostA)
	doiMa(t, ref, http.StatusOK)
	if strings.TrimSpace(ref.Body.String()) != `{"items":[]}` {
		t.Fatalf("hàng chip rỗng = %s", ref.Body.String())
	}
	before := nd.goi
	for _, host := range []string{ckHostKhongCo, ckHostRieng, ckHostNgung} {
		w := ckGoi(h, chipPath, host)
		if w.Code != ref.Code || w.Body.String() != ref.Body.String() {
			t.Fatalf("%s: %d %s, khác %d %s", host, w.Code, w.Body.String(), ref.Code, ref.Body.String())
		}
	}
	if nd.goi != before || dm.goi != 0 {
		t.Fatalf("tên miền không thuộc xã nào mà kho vẫn bị đọc (nội dung %d, danh mục %d)", nd.goi-before, dm.goi)
	}
}

func TestPublicNewsCategoriesPlatformDownIs503(t *testing.T) {
	nd, dm := chipData()
	h := ckMayChu(t, &ckNenTang{chet: true}, nd, dm, nil)
	w := ckGoi(h, chipPath, ckHostA)
	doiMa(t, w, http.StatusServiceUnavailable)
	if strings.Contains(w.Body.String(), "items") || nd.goi != 0 || dm.goi != 0 {
		t.Fatalf("503: %s, kho đọc %d/%d", w.Body.String(), nd.goi, dm.goi)
	}
}

func TestPublicNewsCategoriesStoreFailuresAre500(t *testing.T) {
	for name, set := range map[string]func(*ckNoiDung, *ckDanhMuc){
		"content store":  func(nd *ckNoiDung, _ *ckDanhMuc) { nd.loi = errors.New("pq: connection reset") },
		"category store": func(_ *ckNoiDung, dm *ckDanhMuc) { dm.loi = errors.New("pq: connection reset") },
		"over the cap":   func(_ *ckNoiDung, dm *ckDanhMuc) { dm.loi = commsstore.ErrQuaNhieuDanhMucMiniApp },
	} {
		nd, dm := chipData()
		set(nd, dm)
		h := ckMayChu(t, &ckNenTang{}, nd, dm, nil)
		w := ckGoi(h, chipPath, ckHostA)
		if w.Code != http.StatusInternalServerError || strings.Contains(w.Body.String(), "pq:") ||
			strings.Contains(w.Body.String(), "items") {
			t.Errorf("%s: %d %s — muốn 500 không lộ chi tiết, không phải danh sách cụt", name, w.Code, w.Body.String())
		}
	}
}

func TestPublicNewsCategoriesNotSwallowedByID(t *testing.T) {
	// Go 1.22 ServeMux: the literal `categories` segment is more specific than `{id}`. Were it swallowed,
	// the detail handler would look up an item named "categories" and answer the one 404.
	nd, dm := chipData()
	h := ckMayChu(t, &ckNenTang{}, nd, dm, nil)
	w := ckGoi(h, chipPath, ckHostA)
	doiMa(t, w, http.StatusOK)
	if !strings.HasPrefix(w.Body.String(), `{"items":[`) {
		t.Fatalf("/categories không tới trình xử lý danh mục: %s", w.Body.String())
	}
	// And `{id}` still serves an id — the new literal took nothing else from it.
	doiMa(t, ckGoi(h, MauTinXa+"/a-vacc", ckHostA), http.StatusOK)
	doiMa(t, ckGoi(h, MauTinXa+"/categoriesx", ckHostA), http.StatusNotFound)
}

// --- migration 0011: published_at, event and video fields (added 2026-09-30) ----------------------

func newsWithMedia() *ckNoiDung {
	published := time.Date(2026, 9, 30, 2, 15, 0, 0, time.UTC)
	starts := time.Date(2026, 10, 5, 1, 0, 0, 0, time.UTC)
	ends := time.Date(2026, 10, 5, 3, 0, 0, 0, time.UTC)
	return &ckNoiDung{theoXa: map[tenant.ID][]domain.NoiDungMiniApp{
		xaA: {
			{ID: "event", Loai: domain.LoaiSuKien, TieuDe: "Hội thi", NgayDang: ckNgay,
				TrangThai: domain.TrangThaiDangHien, PublishedAt: published,
				EventStartsAt: starts, EventEndsAt: ends, EventPlace: "<b>Nhà văn hoá</b> thôn 3"},
			// The CHECKs make this row impossible; the handler must still not print it.
			{ID: "news-stray", Loai: domain.LoaiTinTuc, TieuDe: "Tin", NgayDang: ckNgay,
				TrangThai: domain.TrangThaiDangHien, EventStartsAt: starts, EventPlace: "Sân xã",
				VideoURL: "https://v.example/stray"},
			{ID: "video", Loai: domain.LoaiVideo, TieuDe: "Video", NgayDang: ckNgay,
				TrangThai: domain.TrangThaiDangHien, VideoURL: "https://v.example/watch?v=1"},
			{ID: "video-bad", Loai: domain.LoaiVideo, TieuDe: "Video xấu", NgayDang: ckNgay,
				TrangThai: domain.TrangThaiDangHien, VideoURL: "javascript:alert(1)"},
		},
	}}
}

func TestPublicNewsNewFieldsAreTypeGatedAndAbsentWhenUnset(t *testing.T) {
	h := ckMayChu(t, &ckNenTang{}, newsWithMedia(), &ckDanhMuc{}, nil)
	want := map[string]map[string]any{
		"event": {
			"published_at":    "2026-09-30T02:15:00Z",
			"event_starts_at": "2026-10-05T01:00:00Z",
			"event_ends_at":   "2026-10-05T03:00:00Z",
			"event_place":     "Nhà văn hoá thôn 3", // plain text, markup dropped
		},
		"news-stray": {},
		"video":      {"video_url": "https://v.example/watch?v=1"},
		"video-bad":  {}, // a non-http link is dropped, the article still shows
	}
	newKeys := []string{"published_at", "event_starts_at", "event_ends_at", "event_place", "video_url"}

	check := func(where string, item map[string]any) {
		id, _ := item["id"].(string)
		exp, ok := want[id]
		if !ok {
			t.Fatalf("%s: unexpected item %q", where, id)
		}
		for _, key := range newKeys {
			got, present := item[key]
			w, expected := exp[key]
			if present != expected || (expected && got != w) {
				t.Errorf("%s %s: %s = %v (present=%v), want %v (present=%v)", where, id, key, got, present, w, expected)
			}
		}
	}

	w := ckGoi(h, MauTinXa, ckHostA)
	doiMa(t, w, http.StatusOK)
	ckKhongCoHTML(t, w)
	items := ckDocTrang(t, w).Items
	if len(items) != len(want) {
		t.Fatalf("list has %d items, want %d", len(items), len(want))
	}
	for _, it := range items {
		check("list", it)
	}
	if strings.Contains(w.Body.String(), "javascript") || strings.Contains(w.Body.String(), "v.example/stray") ||
		strings.Contains(w.Body.String(), "Sân xã") {
		t.Fatalf("a dropped value reached the wire: %s", w.Body.String())
	}

	for id := range want {
		w := ckGoi(h, MauTinXa+"/"+id, ckHostA)
		doiMa(t, w, http.StatusOK)
		ckKhongCoHTML(t, w)
		var item map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &item); err != nil {
			t.Fatalf("thân không phải JSON: %q", w.Body.String())
		}
		check("detail", item)
	}
}
