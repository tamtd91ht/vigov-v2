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
