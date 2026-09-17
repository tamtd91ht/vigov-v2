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
	if ra.Province != "Thành phố Đà Nẵng" {
		t.Errorf("province = %q, muốn %q", ra.Province, "Thành phố Đà Nẵng")
	}
}

func TestThongTinXaChuaKhaiTinhThanhTraChuoiRongChuKhongBiaMotTinh(t *testing.T) {
	// THE DECISION THIS PINS: a commune that has not declared a province gets "", and the screen
	// renders nothing on that line. What must never happen is a default appearing here — the
	// registry's answer for commune A is one line above in the same fixture, and a handler that
	// fell back to "some province" would make both communes print the same one.
	//
	// The key must still be PRESENT in the JSON, not omitted: a client that receives the field
	// only sometimes has to handle two shapes, and it will handle one of them wrong.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostB, "/api/v1/commune", "", "")
	doiMa(t, w, http.StatusOK)

	var tho map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &tho); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	got, co := tho["province"]
	if !co {
		t.Fatal("thiếu hẳn khoá province — client phải xử lý hai hình dạng phản hồi")
	}
	if got != "" {
		t.Fatalf("province = %v, muốn chuỗi rỗng — không được bịa một tỉnh mặc định", got)
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
	// `province` was added deliberately and passed the question this assertion asks of every
	// field: it is not personal data, it is not an identifier anything is keyed by, and it says
	// nothing about the commune's internal operation. `id` is still absent and still the point.
	muon := []string{"host", "name", "province"}
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

// --- một nguồn phân giải ------------------------------------------------------------------

// TestThongTinXaLayTenTuChinhBienPhanGiai thay cho hai bài đã xoá cùng cơ chế chúng kiểm.
//
// HAI BÀI CŨ KIỂM GÌ: handler phân giải `Host` LẦN THỨ HAI, nên hai lần có thể lệch nhau —
// một bài dựng ca "không đọc được lần hai" (503), một bài dựng ca "lần hai ra xã khác" và
// khẳng định tên xã B không lọt lên tên miền xã A.
//
// VÌ SAO CHÚNG BIẾN MẤT: biên nay mang cả `tenant.Tenant` vào context, nên chỉ còn MỘT lần
// phân giải. Hai ca kia không còn dựng được — không phải vì đã sửa cho chúng không xảy ra,
// mà vì cái sinh ra chúng không còn tồn tại. Giữ lại hai bài đó sẽ là giữ hai bài xanh vĩnh
// viễn mà không kiểm gì, đúng thứ tệ hơn không có test.
//
// CÁI CÒN PHẢI GIỮ là kết luận của chúng: tên phục vụ ra PHẢI là tên của xã mà biên đã phân
// giải, không bao giờ của xã khác. Bài này ghim đúng câu đó, và nó đỏ nếu ai đó lại thêm một
// nguồn phân giải thứ hai vào handler.
func TestThongTinXaLayTenTuChinhBienPhanGiai(t *testing.T) {
	m := dungMayChu(t)

	for _, tr := range []struct {
		host, ten string
	}{
		{hostA, m.thuMuc[hostA].Name},
		{hostB, m.thuMuc[hostB].Name},
	} {
		w := m.goi(t, "GET", tr.host, "/api/v1/commune", "", "")
		doiMa(t, w, http.StatusOK)

		var ra thongTinXa
		if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
			t.Fatalf("%s: %v", tr.host, err)
		}
		if ra.Name != tr.ten {
			t.Errorf("%s: name = %q, muốn %q — tên phải đến từ chính lần phân giải của biên",
				tr.host, ra.Name, tr.ten)
		}
	}
}

func TestThongTinXaChuanHoaHostGiongBien(t *testing.T) {
	// Phép chuẩn hoá Host nay chỉ còn MỘT bản, ở biên (core/httpx/edge.go). Bài này vì thế
	// không còn kiểm "hai bản có khớp nhau không" mà kiểm chính bản duy nhất ấy: một Host viết
	// hoa kèm cổng vẫn phải tới đúng xã. Nếu biên thôi chuẩn hoá, mọi người gõ tên miền viết
	// hoa sẽ nhận 404 — một màn hình đăng nhập hỏng mà không có gì đỏ.
	m := dungMayChu(t)

	r := httptest.NewRequest("GET", "https://"+hostA+"/api/v1/commune", nil)
	r.Host = strings.ToUpper(hostA) + ":8443"
	w := m.chay(r)

	doiMa(t, w, http.StatusOK)
}
