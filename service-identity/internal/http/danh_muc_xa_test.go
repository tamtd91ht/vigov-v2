package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// GET /api/v1/communes?host= — the Mini App's "Làm việc với xã X?" lookup. PUBLIC since 2026-09-27.
//
// Driven through the route as RegisterCongKhai mounts it. The chain in front of it (CORS, header
// stripping, no session layer) is cmd/server's, and cmd/server/bien_cong_dan_test.go drives that.

const (
	hostQR        = "xa-qr.vigov.vn"
	hostNgung     = "xa-cu.vigov.vn"
	hostDanhRieng = "admin.vigov.vn"
	hostKhongCo   = "khong-ai-co.vigov.vn"
	hostXaB       = "xa-b.vigov.vn"
)

// xaQR is a ULID that must never appear in any reply of the public surface.
var xaQR = tenant.ID("01JQ" + strings.Repeat("Q", 22))

// nenTangXaGia is the platform registry. `goi` counts calls: a malformed host must cost none.
type nenTangXaGia struct {
	chet bool
	goi  int
}

func (n *nenTangXaGia) XaTheoHost(_ context.Context, host string) (tenant.Tenant, bool, error) {
	n.goi++
	if n.chet {
		return tenant.Tenant{}, false, errors.New("rpc error: code = Unavailable")
	}
	switch host {
	case hostQR:
		return tenant.Tenant{ID: xaQR, Host: hostQR, Name: "Xã Quế Sơn", Province: "Thành phố Đà Nẵng", Active: true}, true, nil
	case hostXaB:
		return tenant.Tenant{ID: xaB, Host: hostXaB, Name: "Xã B", Active: true}, true, nil
	case hostNgung:
		// A merged commune: the registry still knows it (rule 7), it is simply not active.
		return tenant.Tenant{ID: xaA, Host: hostNgung, Name: "Xã đã sáp nhập", Active: false}, true, nil
	}
	// Unknown AND reserved: service-platform answers both with NotFound, which XaTheoHost maps here.
	return tenant.Tenant{}, false, nil
}

// Launch codes the fake registry knows. Every one is a well-formed 12-hex code, so a 200 `[]` proves
// the HANDLER folded the case, not the shape check.
const (
	codeQR           = "a1b2c3d4e5f6"
	codeInactive     = "0000000000aa"
	codeReservedHost = "0000000000bb"
	codeBadHost      = "0000000000cc"
	codeUnknown      = "ffffffffffff"
)

func (n *nenTangXaGia) CommuneByLaunchCode(_ context.Context, code string) (tenant.Tenant, bool, error) {
	n.goi++
	if n.chet {
		return tenant.Tenant{}, false, errors.New("rpc error: code = Unavailable")
	}
	switch code {
	case codeQR:
		return tenant.Tenant{ID: xaQR, Host: hostQR, Name: "Xã Quế Sơn", Province: "Thành phố Đà Nẵng", Active: true}, true, nil
	case codeInactive:
		return tenant.Tenant{ID: xaA, Host: hostNgung, Name: "Xã đã sáp nhập", Active: false}, true, nil
	case codeReservedHost:
		// service-platform blanks a primary host that is reserved for the platform.
		return tenant.Tenant{ID: xaB, Host: "", Name: "Xã B", Active: true}, true, nil
	case codeBadHost:
		return tenant.Tenant{ID: xaB, Host: "https://" + hostXaB, Name: "Xã B", Active: true}, true, nil
	}
	return tenant.Tenant{}, false, nil
}

// danhBaCongKhaiGia is the published-directory store, keyed by the commune IN THE CONTEXT — so a handler that
// put the wrong commune there, or none, is caught by what comes back (or by the panic).
type danhBaCongKhaiGia struct {
	theoXa map[tenant.ID][]domain.CanBoCongKhai
	loi    error
	goi    int
}

func (d *danhBaCongKhaiGia) DanhBaCongKhai(ctx context.Context) ([]domain.CanBoCongKhai, error) {
	d.goi++
	xa := tenant.MustFrom(ctx) // no commune in the context → panic, exactly like store.For
	if d.loi != nil {
		return nil, d.loi
	}
	return d.theoXa[xa], nil
}

func dungChuoiCongKhai(t *testing.T, nt *nenTangXaGia, db *danhBaCongKhaiGia) http.Handler {
	t.Helper()
	if db == nil {
		db = &danhBaCongKhaiGia{}
	}
	mux := http.NewServeMux()
	RegisterCongKhai(mux, DepsCongKhai{Xa: nt, DanhBa: db, Profile: &profileReaderFake{}, CitizenSessions: &signInFake{}})
	return mux
}

func goiCongKhai(h http.Handler, path, query string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", "https://identity.api.vigov.vn"+path+query, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func goiDanhMucXa(h http.Handler, query string) *httptest.ResponseRecorder {
	return goiCongKhai(h, MauDanhMucXa, query)
}

func qHost(h string) string { return "?host=" + url.QueryEscape(h) }

// hostSaiHinhDang is every malformed `host` the public surface must refuse with 400, before any RPC.
var hostSaiHinhDang = []string{
	"",                                     // no host at all
	"?host=",                               // empty
	qHost(hostQR) + "&host=" + hostKhongCo, // repeated
	qHost("Xa-QR.vigov.vn"),                // not lowercase
	qHost("https://" + hostQR),             // scheme
	qHost(hostQR + ":443"),                 // port
	qHost(hostQR + "/x"),                   // path
	qHost("canbo@" + hostQR),               // userinfo
	qHost(hostQR + "."),                    // trailing dot
	qHost("current"),                       // single label — the staff selector
	qHost("-xa.vigov.vn"),                  // label starts with a hyphen
	qHost("xa..vigov.vn"),                  // empty label
	qHost("xa_qr.vigov.vn"),                // underscore
	qHost("xã.vigov.vn"),                   // non-ASCII
	qHost(strings.Repeat("a", 64) + ".vn"), // label over 63
	qHost("10.0.0.1"),                      // IP literal — the shape the bridge refuses too
}

// --- 200 with no session at all ------------------------------------------------------------------

func TestDanhMucXaKhongCanPhien(t *testing.T) {
	// Owner decision 2026-09-27: the confirmation screen names the commune BEFORE any session exists.
	h := dungChuoiCongKhai(t, &nenTangXaGia{}, nil)
	doiMa(t, goiDanhMucXa(h, qHost(hostQR)), http.StatusOK)
}

func TestDanhMucXaHostDungTraTenVaTinhKhongTraMaXa(t *testing.T) {
	h := dungChuoiCongKhai(t, &nenTangXaGia{}, nil)
	w := goiDanhMucXa(h, qHost(hostQR))
	doiMa(t, w, http.StatusOK)

	than := w.Body.String()
	if strings.Contains(than, string(xaQR)) {
		t.Fatalf("phản hồi chứa tenant_id: %s", than)
	}
	var ra struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân không phải JSON: %q", than)
	}
	if len(ra.Items) != 1 {
		t.Fatalf("items = %v, muốn đúng một xã", ra.Items)
	}
	// EXACT key set: a field named id, tenant, commune_id or host is a new question to answer, not
	// a free addition.
	var khoa []string
	for k := range ra.Items[0] {
		khoa = append(khoa, k)
	}
	sort.Strings(khoa)
	if strings.Join(khoa, ",") != "name,province" {
		t.Fatalf("trường trả về = %v, muốn đúng [name province]", khoa)
	}
	if ra.Items[0]["name"] != "Xã Quế Sơn" || ra.Items[0]["province"] != "Thành phố Đà Nẵng" {
		t.Fatalf("xã trả về sai: %v", ra.Items[0])
	}
}

// --- one answer for every negative -------------------------------------------------------------

func TestDanhMucXaKhongCoDanhRiengNgungHoatDongTraCungMotCauTraLoi(t *testing.T) {
	h := dungChuoiCongKhai(t, &nenTangXaGia{}, nil)

	var mau *httptest.ResponseRecorder
	for _, host := range []string{hostKhongCo, hostDanhRieng, hostNgung} {
		w := goiDanhMucXa(h, qHost(host))
		if mau == nil {
			mau = w
			doiMa(t, w, http.StatusOK)
			if strings.TrimSpace(w.Body.String()) != `{"items":[]}` {
				t.Fatalf("câu trả lời phủ định = %s, muốn {\"items\":[]}", w.Body.String())
			}
			continue
		}
		if w.Code != mau.Code || w.Body.String() != mau.Body.String() {
			t.Fatalf("%s trả %d %s, khác %d %s — người gọi phân biệt được tên miền nào từng là một xã",
				host, w.Code, w.Body.String(), mau.Code, mau.Body.String())
		}
	}
}

// --- 400 before any RPC -------------------------------------------------------------------------

func TestDanhMucXaHostSaiHinhDangLa400VaKhongHoiNenTang(t *testing.T) {
	nt := &nenTangXaGia{}
	h := dungChuoiCongKhai(t, nt, nil)
	for _, q := range hostSaiHinhDang {
		w := goiDanhMucXa(h, q)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%q: mã = %d, muốn 400 — thân: %s", q, w.Code, w.Body.String())
		}
	}
	if nt.goi != 0 {
		t.Fatalf("nền tảng bị hỏi %d lần cho tên miền sai hình dạng", nt.goi)
	}
}

// --- 503 ----------------------------------------------------------------------------------------

func TestDanhMucXaNenTangChetLa503KhongPhaiDanhSachRong(t *testing.T) {
	// An empty list here would tell the citizen the QR names no commune, during what is only an
	// outage; a remembered commune would be a default on the isolation path (rule 1, forbidden #1).
	h := dungChuoiCongKhai(t, &nenTangXaGia{chet: true}, nil)
	w := goiDanhMucXa(h, qHost(hostQR))
	doiMa(t, w, http.StatusServiceUnavailable)
	if strings.Contains(w.Body.String(), "items") {
		t.Fatalf("503 mang hình dạng danh sách: %s", w.Body.String())
	}
}

// --- the literal routes and the exported constants agree ---------------------------------------

func TestMauCongKhaiKhopTuyenDaDangKy(t *testing.T) {
	// cmd/server routes these constants to the public chain; routes_cong_dan.go registers literals.
	// If they drift, the outer mux sends the route to the staff chain and the reserved API host 404s.
	mux := http.NewServeMux()
	RegisterCongKhai(mux, DepsCongKhai{Xa: &nenTangXaGia{}, DanhBa: &danhBaCongKhaiGia{}, Profile: &profileReaderFake{}, CitizenSessions: &signInFake{}})
	for _, p := range []string{MauDanhMucXa, MauDanhBaCongKhai, CommuneProfilesPath} {
		_, mau := mux.Handler(httptest.NewRequest("GET", p, nil))
		if mau != "GET "+p {
			t.Errorf("mẫu khớp = %q, muốn %q", mau, "GET "+p)
		}
	}
}

func TestRegisterCongKhaiThieuKhoThiPanic(t *testing.T) {
	for ten, d := range map[string]DepsCongKhai{
		"thiếu kho tra xã":     {DanhBa: &danhBaCongKhaiGia{}, Profile: &profileReaderFake{}, CitizenSessions: &signInFake{}},
		"thiếu kho danh bạ":    {Xa: &nenTangXaGia{}, Profile: &profileReaderFake{}, CitizenSessions: &signInFake{}},
		"thiếu hồ sơ hiển thị": {Xa: &nenTangXaGia{}, DanhBa: &danhBaCongKhaiGia{}, CitizenSessions: &signInFake{}},
		"missing sign-in":      {Xa: &nenTangXaGia{}, DanhBa: &danhBaCongKhaiGia{}, Profile: &profileReaderFake{}},
		"thiếu cả ba kho đọc":  {},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: dựng tuyến công khai vẫn chạy — lỗi sẽ hiện lúc người dân mở Mini App", ten)
				}
			}()
			RegisterCongKhai(http.NewServeMux(), d)
		}()
	}
}

// --- ?code= — the commune a QR's launch code names (owner decision 2026-10-12) ------------------

func qCode(c string) string { return "?code=" + url.QueryEscape(c) }

// itemKeys decodes a 200 answer and returns its items and their key sets, sorted and comma-joined.
func itemKeys(t *testing.T, w *httptest.ResponseRecorder) ([]map[string]any, []string) {
	t.Helper()
	var ra struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("body is not JSON: %q", w.Body.String())
	}
	var keys []string
	for _, it := range ra.Items {
		var k []string
		for name := range it {
			k = append(k, name)
		}
		sort.Strings(k)
		keys = append(keys, strings.Join(k, ","))
	}
	return ra.Items, keys
}

func TestCommunesByCodeReturnsNameProvinceAndPrimaryHostOnly(t *testing.T) {
	h := dungChuoiCongKhai(t, &nenTangXaGia{}, nil)
	w := goiDanhMucXa(h, qCode(codeQR))
	doiMa(t, w, http.StatusOK)
	if strings.Contains(w.Body.String(), string(xaQR)) {
		t.Fatalf("reply carries the tenant_id: %s", w.Body.String())
	}
	items, keys := itemKeys(t, w)
	if len(items) != 1 || keys[0] != "host,name,province" {
		t.Fatalf("items = %v, want exactly one with keys [host name province]", items)
	}
	if items[0]["host"] != hostQR || items[0]["name"] != "Xã Quế Sơn" || items[0]["province"] != "Thành phố Đà Nẵng" {
		t.Fatalf("commune = %v", items[0])
	}
}

func TestCommunesByHostStillOmitsHostAndLaunchCode(t *testing.T) {
	// The `?host=` answer is unchanged byte-wise: the new Host field is omitempty and never set there.
	// launch_code is NOT added — proto Tenant has no such field (a contract change, not this service's).
	h := dungChuoiCongKhai(t, &nenTangXaGia{}, nil)
	w := goiDanhMucXa(h, qHost(hostQR))
	doiMa(t, w, http.StatusOK)
	want := `{"items":[{"name":"Xã Quế Sơn","province":"Thành phố Đà Nẵng"}]}`
	if strings.TrimSpace(w.Body.String()) != want {
		t.Fatalf("body = %s, want %s", w.Body.String(), want)
	}
}

func TestCommunesByCodeEveryNegativeIsTheSameEmptyList(t *testing.T) {
	// Unknown, inactive, reserved primary host (platform answers ""), and a primary host the bridge
	// would refuse — all one answer, identical to the `?host=` negative.
	h := dungChuoiCongKhai(t, &nenTangXaGia{}, nil)
	for _, c := range []string{codeUnknown, codeInactive, codeReservedHost, codeBadHost} {
		w := goiDanhMucXa(h, qCode(c))
		doiMa(t, w, http.StatusOK)
		if strings.TrimSpace(w.Body.String()) != `{"items":[]}` {
			t.Errorf("%s: body = %s, want {\"items\":[]}", c, w.Body.String())
		}
	}
	byHost := goiDanhMucXa(h, qHost(hostKhongCo))
	byCode := goiDanhMucXa(h, qCode(codeUnknown))
	if byHost.Body.String() != byCode.Body.String() {
		t.Fatalf("negative by host %q differs from negative by code %q", byHost.Body.String(), byCode.Body.String())
	}
}

func TestCommunesBadCodeIs400WithoutAskingThePlatform(t *testing.T) {
	nt := &nenTangXaGia{}
	h := dungChuoiCongKhai(t, nt, nil)
	for _, q := range []string{
		"?code=",                               // empty
		qCode(codeQR) + "&code=" + codeUnknown, // repeated
		qCode("A1B2C3D4E5F6"),                  // uppercase
		qCode("a1b2c3d4e5f"),                   // 11 chars
		qCode("a1b2c3d4e5f6a"),                 // 13 chars
		qCode("g1b2c3d4e5f6"),                  // not hex
		qCode(string(xaQR)),                    // a tenant id is not a launch code
		qCode(hostQR),                          // nor is a domain
	} {
		w := goiDanhMucXa(h, q)
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), `"invalid_code"`) {
			t.Errorf("%q: %d %s, want 400 invalid_code", q, w.Code, w.Body.String())
		}
	}
	if nt.goi != 0 {
		t.Fatalf("platform asked %d times for a malformed code", nt.goi)
	}
}

func TestCommunesHostAndCodeTogetherIs400(t *testing.T) {
	nt := &nenTangXaGia{}
	h := dungChuoiCongKhai(t, nt, nil)
	for _, q := range []string{
		qHost(hostQR) + "&code=" + codeQR,
		qCode(codeQR) + "&host=" + hostQR,
		"?host=&code=", // both present, both empty — still both
	} {
		w := goiDanhMucXa(h, q)
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), `"invalid_query"`) {
			t.Errorf("%q: %d %s, want 400 invalid_query", q, w.Code, w.Body.String())
		}
	}
	if nt.goi != 0 {
		t.Fatalf("platform asked %d times when both parameters were sent", nt.goi)
	}
}

func TestCommunesNeitherParameterIs400(t *testing.T) {
	nt := &nenTangXaGia{}
	h := dungChuoiCongKhai(t, nt, nil)
	for _, q := range []string{"", "?other=x"} {
		w := goiDanhMucXa(h, q)
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), `"invalid_host"`) {
			t.Errorf("%q: %d %s, want 400 invalid_host (the answer for no filter, unchanged)", q, w.Code, w.Body.String())
		}
	}
	if nt.goi != 0 {
		t.Fatalf("platform asked %d times with no filter", nt.goi)
	}
}

func TestCommunesByCodePlatformDownIs503(t *testing.T) {
	h := dungChuoiCongKhai(t, &nenTangXaGia{chet: true}, nil)
	w := goiDanhMucXa(h, qCode(codeQR))
	doiMa(t, w, http.StatusServiceUnavailable)
	if strings.Contains(w.Body.String(), "items") || !strings.Contains(w.Body.String(), `"platform_unavailable"`) {
		t.Fatalf("503 body = %s, want platform_unavailable and no list", w.Body.String())
	}
}
