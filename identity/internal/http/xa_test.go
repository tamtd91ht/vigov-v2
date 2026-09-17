package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
)

// WHAT THIS FILE IS FOR: GET /api/v1/commune is the only PUBLIC route in this service that
// returns data. Three things have to hold, and each of them fails silently if it stops holding:
//
//  1. it works with NO token — otherwise the sign-in screen cannot print the name of the
//     authority somebody is about to sign in to, and the route has no reason to exist;
//  2. it never returns tenant_id — the opaque identifier the whole isolation model rests on,
//     handed to anybody with curl, in exchange for nothing;
//  3. the commune comes from Host and from nothing else — never from a query parameter, never
//     from a header (rule 1, forbidden #2).

// --- (1) public ------------------------------------------------------------------------------

func TestThongTinXaKhongCanToken(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, "/api/v1/commune", "", "")
	doiMa(t, w, http.StatusOK)

	var ra thongTinXa
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	if ra.Name != "Xã Thăng Bình" {
		t.Errorf("name = %q, muốn tên xã của host A", ra.Name)
	}
	if ra.Host != hostA {
		t.Errorf("host = %q, muốn %q", ra.Host, hostA)
	}
}

func TestThongTinXaTokenHongVanPhucVu(t *testing.T) {
	// A stale cookie must not make the sign-in screen unreachable — the same property as
	// TestTokenHongThiXoaCookieVaVanPhucVuRoutePublic, asserted on the route that renders that
	// screen's branding. That is how somebody gets locked out of a government system with no way
	// back in: the page that would let them sign in again refuses to load.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, "/api/v1/commune", "", "token-hong-khong-giai-duoc")
	doiMa(t, w, http.StatusOK)
}

// --- (2) no tenant_id ------------------------------------------------------------------------

func TestThongTinXaKhongTraTenantID(t *testing.T) {
	// THE TEST THAT HOLDS THE DECISION IN PLACE. tenant.Tenant carries ID right beside Name, so
	// the next person to touch thongTinXa will have the field in front of them and a reason to
	// add it ("the web might need it"). It does not: the commune is derived from Host on every
	// request, server-side. This route is PUBLIC, so publishing that id would hand the prefix of
	// every cache key, queue message and file path (rule 1, invariant 7) to anybody with curl.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, "/api/v1/commune", "", "")
	doiMa(t, w, http.StatusOK)

	than := w.Body.String()
	if strings.Contains(than, string(xaA)) {
		t.Fatalf("phản hồi công khai chứa tenant_id: %s", than)
	}

	// Asserted as an EXACT key set, not as "no field called id". A field named tenant, xa,
	// commune_id or anything else carrying the same value is the same leak under another name.
	var ra map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân không phải JSON: %q", than)
	}
	khoa := make([]string, 0, len(ra))
	for k := range ra {
		khoa = append(khoa, k)
	}
	sort.Strings(khoa)
	muon := []string{"host", "name"}
	if strings.Join(khoa, ",") != strings.Join(muon, ",") {
		t.Fatalf("trường trả về = %v, muốn đúng %v — mỗi trường thêm vào một tuyến CÔNG KHAI "+
			"phải được cân nhắc lại từ đầu", khoa, muon)
	}
}

// --- (3) the commune comes from Host ----------------------------------------------------------

func TestThongTinXaLayTheoHostChuKhongTheoThamSo(t *testing.T) {
	// A client naming its own commune is a client granting itself access (rule 1, forbidden #2).
	// The query string and the header both name commune B; the Host is commune A; the answer must
	// be commune A. httpx.StripTenantHeaders removes the header before any handler, and nothing in
	// ThongTinXa reads the query at all — this asserts both at once.
	m := dungMayChu(t)

	r := httptest.NewRequest("GET", "https://"+hostA+"/api/v1/commune?tenant_id="+string(xaB)+"&host="+hostB, nil)
	r.Host = hostA
	r.Header.Set("X-Tenant-Id", string(xaB))
	w := m.chay(r)

	doiMa(t, w, http.StatusOK)
	var ra thongTinXa
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	if ra.Name != "Xã Thăng Bình" || ra.Host != hostA {
		t.Fatalf("RÒ RỈ: tham số của client quyết định xã — nhận %+v", ra)
	}
}

func TestThongTinXaMoiHostTraXaCuaChinhNo(t *testing.T) {
	m := dungMayChu(t)

	for host, muon := range map[string]string{
		hostA: "Xã Thăng Bình",
		hostB: "Xã Bình Dương",
	} {
		w := m.goi(t, "GET", host, "/api/v1/commune", "", "")
		doiMa(t, w, http.StatusOK)
		var ra thongTinXa
		if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
			t.Fatalf("thân không phải JSON: %q", w.Body.String())
		}
		if ra.Name != muon {
			t.Errorf("host %s trả tên xã %q, muốn %q", host, ra.Name, muon)
		}
	}
}

func TestThongTinXaHostKhongThuocXaNaoTra404(t *testing.T) {
	// Rule 1, invariant 3: cannot resolve the commune -> 404, never a default commune and never a
	// different error. Asserted on the PUBLIC route specifically: it is the one an unauthenticated
	// prober reaches, and a 400 or a 500 here would answer questions a 404 does not.
	m := dungMayChu(t)

	doiMa(t, m.goi(t, "GET", "khong-ai-biet.example.gov.vn", "/api/v1/commune", "", ""),
		http.StatusNotFound)
}

// --- failing closed ---------------------------------------------------------------------------

func TestThongTinXaKhongDocDuocThi503(t *testing.T) {
	// The platform service has gone away between the edge's lookup and the handler's. There is no
	// cached name to fall back on and no name may be guessed, so nothing is served.
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) { d.Xa = thuMucGia{} })

	w := m.goi(t, "GET", hostA, "/api/v1/commune", "", "")
	doiMa(t, w, http.StatusServiceUnavailable)
	if got := loiTra(t, w).Code; got != "tenant_unavailable" {
		t.Errorf("code = %q, muốn tenant_unavailable", got)
	}
}

func TestThongTinXaPhanGiaiRaXaKhacThiTuChoi(t *testing.T) {
	// THE CASE WORTH THE WHOLE MECHANISM. The handler resolves Host a second time, so the two
	// lookups could in principle disagree — a domain reassigned between them, or a normalisation
	// that drifted away from the edge's. Serving the name that came back would print commune B's
	// name on commune A's domain: a breach between two authorities, on a public page.
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Xa = thuMucGia{hostA: {ID: xaB, Host: hostB, Name: "Xã Bình Dương", Active: true}}
	})

	w := m.goi(t, "GET", hostA, "/api/v1/commune", "", "")
	doiMa(t, w, http.StatusServiceUnavailable)
	if strings.Contains(w.Body.String(), "Bình Dương") {
		t.Fatalf("RÒ RỈ: trả tên xã khác trên tên miền của xã này: %s", w.Body.String())
	}
}

func TestThongTinXaChuanHoaHostGiongBien(t *testing.T) {
	// httptest sends a Host with no port, so this is the case the normalisation copy exists for:
	// an upper-cased Host with a port must reach the same commune at the edge AND in the handler.
	// If the two ever normalise differently the answer is 503, never another commune's name — but
	// a 503 on an ordinary request is still a broken sign-in screen.
	m := dungMayChu(t)

	r := httptest.NewRequest("GET", "https://"+hostA+"/api/v1/commune", nil)
	r.Host = strings.ToUpper(hostA) + ":8443"
	w := m.chay(r)

	doiMa(t, w, http.StatusOK)
}
