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

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
)

// GET /api/v1/communes?host= — the Mini App's "Làm việc với xã X?" lookup.
//
// Driven through the REAL citizen chain (CitizenEdge + CitizenPrincipal + the route's own two
// declarations), not the handler alone: the properties that matter — 401 without a session, a
// session without a phone still accepted, no commune in the context — live in the chain.

const (
	hostQR        = "xa-qr.vigov.vn"
	hostNgung     = "xa-cu.vigov.vn"
	hostDanhRieng = "admin.vigov.vn"
	hostKhongCo   = "khong-ai-co.vigov.vn"

	tokCoSo   = "tok-phien-co-so"
	tokChuaSo = "tok-phien-chua-so"
	tokChuaXa = "tok-phien-chua-chon-xa"
)

// xaQR is a ULID that must never appear in any reply of this route.
var xaQR = tenant.ID("01JQ" + strings.Repeat("Q", 22))

type soPhienCongDanGia map[string]httpx.CitizenSession

func (s soPhienCongDanGia) TraCuu(_ context.Context, tok string) (httpx.CitizenSession, bool) {
	p, ok := s[tok]
	return p, ok
}

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
	case hostNgung:
		// A merged commune: the registry still knows it (rule 7), it is simply not active.
		return tenant.Tenant{ID: xaB, Host: hostNgung, Name: "Xã đã sáp nhập", Active: false}, true, nil
	}
	// Unknown AND reserved: service-platform answers both with NotFound, which XaTheoHost maps here.
	return tenant.Tenant{}, false, nil
}

func dungChuoiCongDan(t *testing.T, nt *nenTangXaGia) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	RegisterCongDan(mux, DepsCongDan{Xa: nt})
	so := soPhienCongDanGia{
		tokCoSo:   {ID: "sid-1", CitizenID: "cd-1", TenantID: xaA},
		tokChuaSo: {ID: "sid-2", CitizenID: "", TenantID: xaA}, // ADR 0045: phone not verified yet
		tokChuaXa: {ID: "sid-3", CitizenID: "cd-3", TenantID: ""},
	}
	var c http.Handler = mux
	c = authz.CitizenPrincipal()(c)
	c = httpx.CitizenEdge(so)(c)
	return c
}

func goiDanhMucXa(h http.Handler, query, tok string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", "https://identity.api.vigov.vn"+MauDanhMucXa+query, nil)
	if tok != "" {
		r.Header.Set("Authorization", "Bearer "+tok)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func qHost(h string) string { return "?host=" + url.QueryEscape(h) }

// --- 401 --------------------------------------------------------------------------------------

func TestDanhMucXaKhongCoPhienLa401VaKhongHoiNenTang(t *testing.T) {
	nt := &nenTangXaGia{}
	h := dungChuoiCongDan(t, nt)
	for _, tok := range []string{"", "tok-khong-ai-phat"} {
		doiMa(t, goiDanhMucXa(h, qHost(hostQR), tok), http.StatusUnauthorized)
	}
	if nt.goi != 0 {
		t.Fatalf("nền tảng bị hỏi %d lần cho yêu cầu không có phiên — tuyến thành máy tra tên miền công khai", nt.goi)
	}
}

// --- 200 --------------------------------------------------------------------------------------

func TestDanhMucXaHostDungTraTenVaTinhKhongTraMaXa(t *testing.T) {
	h := dungChuoiCongDan(t, &nenTangXaGia{})
	w := goiDanhMucXa(h, qHost(hostQR), tokCoSo)
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

func TestDanhMucXaPhienChuaCoSoVanXemDuoc(t *testing.T) {
	// ADR 0045: a session without a verified phone may VIEW. This route only views public registry
	// metadata; refusing it would block the very first screen of every Mini App launch.
	h := dungChuoiCongDan(t, &nenTangXaGia{})
	doiMa(t, goiDanhMucXa(h, qHost(hostQR), tokChuaSo), http.StatusOK)
}

func TestDanhMucXaPhienChuaChonXaVanXemDuoc(t *testing.T) {
	// The KhongThuocXa class: the commune being asked about is NOT the session's. A session that has
	// no commune yet is the normal caller, not an edge case.
	h := dungChuoiCongDan(t, &nenTangXaGia{})
	doiMa(t, goiDanhMucXa(h, qHost(hostQR), tokChuaXa), http.StatusOK)
}

// --- one answer for every negative -------------------------------------------------------------

func TestDanhMucXaKhongCoDanhRiengNgungHoatDongTraCungMotCauTraLoi(t *testing.T) {
	h := dungChuoiCongDan(t, &nenTangXaGia{})

	var mau *httptest.ResponseRecorder
	for _, host := range []string{hostKhongCo, hostDanhRieng, hostNgung} {
		w := goiDanhMucXa(h, qHost(host), tokCoSo)
		if mau == nil {
			mau = w
			doiMa(t, w, http.StatusOK)
			if strings.TrimSpace(w.Body.String()) != `{"items":[]}` {
				t.Fatalf("câu trả lời phủ định = %s, muốn {\"items\":[]}", w.Body.String())
			}
			continue
		}
		if w.Code != mau.Code || w.Body.String() != mau.Body.String() {
			t.Fatalf("%s trả %d %s, khác %d %s — người có phiên phân biệt được tên miền nào từng là một xã",
				host, w.Code, w.Body.String(), mau.Code, mau.Body.String())
		}
	}
}

// --- 400 before any RPC -------------------------------------------------------------------------

func TestDanhMucXaHostSaiHinhDangLa400VaKhongHoiNenTang(t *testing.T) {
	nt := &nenTangXaGia{}
	h := dungChuoiCongDan(t, nt)
	for _, q := range []string{
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
	} {
		w := goiDanhMucXa(h, q, tokCoSo)
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
	h := dungChuoiCongDan(t, &nenTangXaGia{chet: true})
	w := goiDanhMucXa(h, qHost(hostQR), tokCoSo)
	doiMa(t, w, http.StatusServiceUnavailable)
	if strings.Contains(w.Body.String(), "items") {
		t.Fatalf("503 mang hình dạng danh sách: %s", w.Body.String())
	}
}

// --- the literal route and the exported constant agree ------------------------------------------

func TestMauDanhMucXaKhopTuyenDaDangKy(t *testing.T) {
	// cmd/server routes MauDanhMucXa to the citizen chain; routes_cong_dan.go registers a literal.
	// If the two drift, the outer mux sends the route to the staff chain and every citizen gets 404.
	mux := http.NewServeMux()
	RegisterCongDan(mux, DepsCongDan{Xa: &nenTangXaGia{}})
	_, mau := mux.Handler(httptest.NewRequest("GET", MauDanhMucXa, nil))
	if mau != "GET "+MauDanhMucXa {
		t.Fatalf("mẫu khớp = %q, muốn %q", mau, "GET "+MauDanhMucXa)
	}
}

func TestRegisterCongDanThieuKhoTraXaThiPanic(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("dựng tuyến công dân mà không có kho tra xã vẫn chạy — lỗi sẽ hiện lúc công dân quét QR")
		}
	}()
	RegisterCongDan(http.NewServeMux(), DepsCongDan{})
}
