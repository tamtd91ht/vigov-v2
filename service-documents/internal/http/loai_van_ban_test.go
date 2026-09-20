package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	docstore "github.com/vihat/vigov/service-documents/internal/store"
)

// WHAT THIS FILE IS FOR: GET /api/v1/document-types is the FIRST route of the documents service,
// and it is AnyAuthenticated. Four things have to hold, and each fails silently if it stops
// holding:
//
//  1. the four cases of rule 5, invariant 7, read for an AnyAuthenticated route — see the note on
//     TestLoaiVanBan_200KhongCanQuyenCauHinh for what the "403 wrong permission" case becomes here;
//  2. one commune's catalogue never reaches another commune's caller;
//  3. the list is returned WHOLE and in the store's order, or refused — never trimmed;
//  4. a commune with no rows serialises as [] and not null. TODAY THAT IS EVERY COMMUNE: the table
//     ships empty on purpose and nothing sows it yet.

const duongLoaiVanBan = "/api/v1/document-types"

func docLoaiVanBan(t *testing.T, than []byte) danhSachLoaiVanBanRa {
	t.Helper()
	var ra danhSachLoaiVanBanRa
	if err := json.Unmarshal(than, &ra); err != nil {
		t.Fatalf("thân không phải JSON: %q", string(than))
	}
	return ra
}

// --- (1) the four cases ---------------------------------------------------------------------------

func TestLoaiVanBan_401KhongPhien(t *testing.T) {
	m := dungMayChu(t)

	doiMa(t, m.goi(t, "GET", hostA, duongLoaiVanBan, nil), http.StatusUnauthorized)
	if m.loai.goi != 0 {
		t.Error("chưa đăng nhập mà đã đọc danh mục loại văn bản")
	}
}

func TestLoaiVanBan_200KhongCanQuyenCauHinh(t *testing.T) {
	// THE "403 WRONG PERMISSION" CASE, AS IT READS ON AN AnyAuthenticated ROUTE — and it is the
	// decision this route was declared with, so it is asserted rather than assumed.
	//
	// The harness's checker REFUSES every permission in every commune. The account must still get
	// 200. That is what AnyAuthenticated means here, and it is the whole reason the route is
	// declared that way: type names fill the registration form, every document list's filter and
	// the label on every document already registered, so a configuration permission on this list
	// would empty those screens for everybody who is not an administrator.
	//
	// The second assertion is the stronger one: the checker was never even ASKED. A route that
	// happened to pass because the caller held something would go green here too; one that
	// consults no permission at all is the only thing that leaves the counter at zero.
	//
	// If somebody later "tightens" this to RequirePermission, both assertions go red — which is the
	// point. The trade-off they protect is stated on the route: the catalogue of ONE commune is
	// readable by every signed-in account OF THAT COMMUNE.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongLoaiVanBan, canBoCua(xaA))
	doiMa(t, w, http.StatusOK)
	if len(docLoaiVanBan(t, w.Body.Bytes()).Items) == 0 {
		t.Error("tài khoản không có quyền nào nhận danh sách rỗng — tuyến này phải trả đủ")
	}
	if m.checker.goi != 0 {
		t.Errorf("tuyến AnyAuthenticated vẫn hỏi Checker %d lần — nó không được phụ thuộc vào quyền nào",
			m.checker.goi)
	}
}

func TestLoaiVanBan_401XaKhac(t *testing.T) {
	// THE "RIGHT PERMISSION, WRONG COMMUNE" CASE, as it reads here: there is no permission to be
	// right or wrong about, so what remains is a principal issued by commune A presented at commune
	// B's domain. authz.AnyAuthenticated compares the commune in the principal with the commune
	// resolved from Host and refuses BEFORE the handler runs — which is also what makes one
	// commune's catalogue unreachable from another, rather than merely unrequested.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostB, duongLoaiVanBan, canBoCua(xaA))
	doiMa(t, w, http.StatusUnauthorized)
	if m.loai.goi != 0 {
		t.Error("phiên của xã khác mà vẫn đọc danh mục của xã này")
	}
}

func TestLoaiVanBan_200(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongLoaiVanBan, canBoCua(xaA))
	doiMa(t, w, http.StatusOK)

	ra := docLoaiVanBan(t, w.Body.Bytes())
	if len(ra.Items) != 2 {
		t.Fatalf("nhận %d loại văn bản, muốn 2", len(ra.Items))
	}
	if ra.Items[0].ID != "lvb-001" || ra.Items[0].Code != "quyet-dinh" || ra.Items[0].Label != "Quyết định" {
		t.Errorf("loại đầu sai: %+v", ra.Items[0])
	}
	if !ra.Items[0].IsDefault || !ra.Items[0].Active {
		t.Errorf("hàng mặc định đang dùng lại ra %+v — ô chọn sẽ không chọn sẵn được gì", ra.Items[0])
	}
	// A row the commune has taken out of use IS returned, carrying active:false. It is what the
	// configuration screen shows with a "Đã tắt" chip, and what lets a document registered under a
	// retired type still render its own label. Dropping it here would look tidy and break both.
	if ra.Items[1].Code != "cong-van" || ra.Items[1].Active {
		t.Errorf("hàng đã tắt sai: %+v", ra.Items[1])
	}
	if ra.Items[1].IsDefault {
		t.Error("hàng thứ hai báo là mặc định — mỗi xã chỉ có ĐÚNG MỘT mặc định còn sống")
	}
}

// --- (2) one commune's catalogue never reaches another ----------------------------------------------

func TestLoaiVanBanKhongVuotSangXaKhac(t *testing.T) {
	// The same account, signed in properly at commune B. Commune A's catalogue must not travel with
	// the person. Nothing about the request is malformed — this is the shape a leak actually takes,
	// and the route being readable by EVERY signed-in account is exactly why it is asserted here.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostB, duongLoaiVanBan, canBoCua(xaB))
	doiMa(t, w, http.StatusOK)

	than := w.Body.String()
	if strings.Contains(than, "quyet-dinh") || strings.Contains(than, "lvb-001") {
		t.Fatalf("RÒ RỈ: ở xã B nhận được danh mục của xã A: %s", than)
	}
	ra := docLoaiVanBan(t, w.Body.Bytes())
	if len(ra.Items) != 1 || ra.Items[0].ID != "lvb-b-001" {
		t.Fatalf("xã B phải nhận đúng danh mục của mình, nhận: %+v", ra.Items)
	}
}

// --- (3) whole list, store's order, or refused ------------------------------------------------------

func TestLoaiVanBanGiuNguyenThuTuCuaKho(t *testing.T) {
	// `thu_tu` IS THE ORDER THE COMMUNE ARRANGED ITS OWN CATALOGUE IN. The store sorts by it; a
	// handler that re-sorted — alphabetically, by code, by anything — would silently overrule the
	// commune on its own list. The fixture is deliberately NOT in alphabetical order ("Quyết định"
	// before "Công văn"), so any re-sort turns this red.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongLoaiVanBan, canBoCua(xaA))
	doiMa(t, w, http.StatusOK)

	ra := docLoaiVanBan(t, w.Body.Bytes())
	if ra.Items[0].Code != "quyet-dinh" || ra.Items[1].Code != "cong-van" {
		t.Fatalf("thứ tự đã bị đổi: %v, %v", ra.Items[0].Code, ra.Items[1].Code)
	}
}

func TestLoaiVanBanVuotTranThiTuChoiChuKhongCatBot(t *testing.T) {
	// THE DECISION THIS PINS, and it is the one that looks wrong at first glance: the route answers
	// 500 rather than returning the first N types.
	//
	// This list is what a document is registered under, and numbering follows the type (ADR 0024).
	// A silently short list files the document under the wrong type, in the wrong number series —
	// and an issued number is the one thing rule 7 never lets us renumber. A refusal breaks one
	// commune's screen loudly and names itself in the log.
	//
	// The ceiling itself lives in the store (docstore.TranDanhMucLoaiVanBan) because only the store
	// knows the LIMIT; what is asserted here is that the handler does not quietly render the error
	// away.
	m := dungMayChu(t)
	m.loai.loi = docstore.ErrQuaNhieuLoaiVanBan

	w := m.goi(t, "GET", hostA, duongLoaiVanBan, canBoCua(xaA))
	doiMa(t, w, http.StatusInternalServerError)

	if e := loiTra(t, w); e.Code != "internal" {
		t.Errorf("code = %q, muốn internal", e.Code)
	}
	// No partial list came back with the error. A body carrying items alongside a 500 is how a
	// refusal turns back into a truncation on a client that reads the body anyway.
	if strings.Contains(w.Body.String(), `"items"`) {
		t.Errorf("phản hồi từ chối vẫn kèm danh sách: %s", w.Body.String())
	}
}

func TestLoaiVanBanLoiKhoTra500VaKhongLoNoiDungLoi(t *testing.T) {
	m := dungMayChu(t)
	m.loai.loi = errors.New("cơ sở dữ liệu không phản hồi")

	w := m.goi(t, "GET", hostA, duongLoaiVanBan, canBoCua(xaA))
	doiMa(t, w, http.StatusInternalServerError)

	if e := loiTra(t, w); strings.Contains(e.Message, "cơ sở dữ liệu không phản hồi") {
		t.Errorf("lỗi nội bộ lọt ra ngoài: %q", e.Message)
	}
}

// --- (4) a commune with no rows ---------------------------------------------------------------------

func TestLoaiVanBanXaChuaCoDongNaoTraMangRong(t *testing.T) {
	// [] AND NOT null, AND THIS IS THE ORDINARY PATH RATHER THAN AN EDGE CASE: the table ships empty
	// for every commune and the onboarding step that would sow it does not exist yet
	// (migrations/0003_danh_muc_loai_van_ban.sql, §WHERE THE he-thong ROWS COME FROM). An empty list
	// is the CORRECT answer here, not a failure — and a client that has to handle both [] and null
	// handles one of them wrong.
	m := dungMayChu(t)
	m.loai.theo[xaA] = nil

	w := m.goi(t, "GET", hostA, duongLoaiVanBan, canBoCua(xaA))
	doiMa(t, w, http.StatusOK)

	if !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Errorf("items phải tuần tự hoá thành [], nhận: %s", w.Body.String())
	}
}

// --- the host is still what decides the commune -----------------------------------------------------

func TestLoaiVanBanTenMienLa404TruocMoiThu(t *testing.T) {
	// A Host belonging to no commune is refused by httpx.TenantMiddleware with 404, before the
	// permission declaration and before any store is touched. Asserted here because this is the
	// service's first route, so it is the first time the chain in dungLai is exercised at all: a
	// route mounted outside that chain would answer 401 (no principal) or panic instead, and both
	// would be a commune resolved by something other than the edge (rule 1, invariant 3).
	m := dungMayChu(t)

	w := m.goi(t, "GET", "khong-thuoc-xa-nao.example.vn", duongLoaiVanBan, canBoCua(xaA))
	doiMa(t, w, http.StatusNotFound)
	if m.loai.goi != 0 {
		t.Error("tên miền không thuộc xã nào mà vẫn đọc danh mục")
	}
}
