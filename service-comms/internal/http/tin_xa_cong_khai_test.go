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
}

func (k *ckNoiDung) DanhSachCongKhai(ctx context.Context, _ page.Request) (page.Result[domain.NoiDungMiniApp], error) {
	k.goi++
	xa := tenant.MustFrom(ctx)
	if k.loi != nil {
		return page.Result[domain.NoiDungMiniApp]{}, k.loi
	}
	if len(k.theoXa[xa]) == 0 {
		return page.NewResult[domain.NoiDungMiniApp](), nil
	}
	return page.Result[domain.NoiDungMiniApp]{Items: k.theoXa[xa], NextCursor: "con-tro-tiep", HasMore: true}, nil
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

type ckDanhMuc struct {
	theoXa map[tenant.ID][]domain.DanhMucMiniApp
}

func (d *ckDanhMuc) DanhSach(ctx context.Context) ([]domain.DanhMucMiniApp, error) {
	return d.theoXa[tenant.MustFrom(ctx)], nil
}

const ckScript = `<p>Lịch tiêm chủng</p><script>alert("xã")</script><p>Bà con &amp; các cháu</p>` +
	`<img src=x onerror=alert(1)>&lt;script&gt;alert(2)&lt;/script&gt;`

func ckDuLieu() (*ckNoiDung, *ckDanhMuc) {
	nd := &ckNoiDung{theoXa: map[tenant.ID][]domain.NoiDungMiniApp{
		xaA: {
			{ID: "nd-a-1", TieuDe: "<b>Tiêm chủng</b> tháng 10", TomTat: "Tóm <i>tắt</i>", NoiDung: ckScript,
				DanhMucID: "dm-a", NgayDang: ckNgay, TrangThai: domain.TrangThaiDangHien,
				NguoiTaoMa: "CB-2026-7K3M9Q", LuotXem: 9, AnhDaiDienURL: "https://x/a.png"},
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
	RegisterCongKhai(mux, DepsCongKhai{Xa: nt, NoiDung: nd, DanhMuc: dm, Log: log})
	return mux
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
	// No body on the list; no status, author, provenance, view count, image or ids of anything else.
	if got := strings.Join(khoa, ","); got != "category_name,id,published_on,summary,title" {
		t.Fatalf("trường của trang = %s", got)
	}
	if it["title"] != "Tiêm chủng tháng 10" || it["summary"] != "Tóm tắt" ||
		it["category_name"] != "Y tế" || it["published_on"] != "2026-09-27" {
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
	if got := strings.Join(khoa, ","); got != "body,category_name,id,published_on,summary,title" {
		t.Fatalf("trường của chi tiết = %s", got)
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
	RegisterCongKhai(mux, DepsCongKhai{Xa: &ckNenTang{}, NoiDung: nd, DanhMuc: dm})
	for p, muon := range map[string]string{
		MauTinXa:          "GET " + MauTinXa,
		MauTinXa + "/abc": "GET " + MauTinXa + "/{id}",
	} {
		if _, mau := mux.Handler(httptest.NewRequest(http.MethodGet, p, nil)); mau != muon {
			t.Errorf("%s khớp %q, muốn %q", p, mau, muon)
		}
	}
}

func TestRegisterCongKhaiThieuKhoThiPanic(t *testing.T) {
	nd, dm := ckDuLieu()
	for ten, d := range map[string]DepsCongKhai{
		"thiếu nền tảng": {NoiDung: nd, DanhMuc: dm},
		"thiếu nội dung": {Xa: &ckNenTang{}, DanhMuc: dm},
		"thiếu danh mục": {Xa: &ckNenTang{}, NoiDung: nd},
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
