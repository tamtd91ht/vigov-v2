package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// WHAT THIS FILE IS FOR: GET /api/v1/org-units is the first AnyAuthenticated route in this service
// that returns DATA belonging to the commune rather than to the caller. Four things have to hold,
// and each fails silently if it stops holding:
//
//  1. the four cases of rule 5, invariant 7, read for an AnyAuthenticated route — see the note on
//     TestBoPhan_200KhongCanQuyenCauHinh for what the "403 wrong permission" case becomes here;
//  2. one commune's org chart never reaches another commune's caller;
//  3. the list is returned WHOLE and in the store's order, or refused — never trimmed;
//  4. an empty commune serialises as [] and not null.

const duongBoPhan = "/api/v1/org-units"

func docBoPhan(t *testing.T, than []byte) danhSachBoPhanRa {
	t.Helper()
	var ra danhSachBoPhanRa
	if err := json.Unmarshal(than, &ra); err != nil {
		t.Fatalf("thân không phải JSON: %q", string(than))
	}
	return ra
}

// --- (1) the four cases -------------------------------------------------------------------------

func TestBoPhan_401KhongToken(t *testing.T) {
	m := dungMayChu(t)

	doiMa(t, m.goi(t, "GET", hostA, duongBoPhan, "", ""), http.StatusUnauthorized)
	if m.boPhan.goi != 0 {
		t.Error("chưa đăng nhập mà đã đọc danh mục bộ phận")
	}
}

func TestBoPhan_200KhongCanQuyenCauHinh(t *testing.T) {
	// THE "403 WRONG PERMISSION" CASE, AS IT READS ON AN AnyAuthenticated ROUTE — and it is the
	// decision this route was declared with, so it is asserted rather than assumed.
	//
	// The account is rebuilt holding NOTHING: not admin.user, not anything. It must still get 200.
	// That is what AnyAuthenticated means here, and it is the whole reason the route is declared
	// that way: unit names fill the assignment box, document routing, the directory and every
	// filter, so a configuration permission on this list would empty those boxes for everybody who
	// is not an administrator.
	//
	// If somebody later "tightens" this to RequirePermission, this test goes red — which is the
	// point. The trade-off it protects is stated on the route: the org chart of ONE commune is
	// readable by every signed-in account OF THAT COMMUNE.
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{} // no grants at all, in any commune
	})

	w := m.goi(t, "GET", hostA, duongBoPhan, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)
	if len(docBoPhan(t, w.Body.Bytes()).Items) == 0 {
		t.Error("tài khoản không có quyền nào nhận danh sách rỗng — tuyến này phải trả đủ")
	}
}

func TestBoPhan_401XaKhac(t *testing.T) {
	// THE "RIGHT PERMISSION, WRONG COMMUNE" CASE, as it reads here: there is no permission to be
	// right or wrong about, so what remains is a token issued by commune A presented at commune B's
	// domain. It is refused at the token layer, BEFORE any store is touched — which is also what
	// makes the org chart unreachable across communes rather than merely unrequested.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostB, duongBoPhan, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusUnauthorized)
	if got := loiTra(t, w).Code; got != "tenant_mismatch" {
		t.Errorf("code = %q, muốn tenant_mismatch", got)
	}
	if m.boPhan.goi != 0 {
		t.Error("token của xã khác mà vẫn đọc danh mục của xã này")
	}
}

func TestBoPhan_200(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongBoPhan, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	ra := docBoPhan(t, w.Body.Bytes())
	if len(ra.Items) != 2 {
		t.Fatalf("nhận %d bộ phận, muốn 2", len(ra.Items))
	}
	if ra.Items[0].ID != "bp-001" || ra.Items[0].Code != "van-phong-dang-uy" ||
		ra.Items[0].Name != "VĂN PHÒNG ĐẢNG ỦY" {
		t.Errorf("bộ phận đầu sai: %+v", ra.Items[0])
	}
	// The root node's parent is "" — the flat-list-with-parent-ids decision, asserted so that
	// switching to a nested tree cannot happen without this going red.
	if ra.Items[0].ParentID != "" {
		t.Errorf("parent_id của nút gốc = %q, muốn rỗng", ra.Items[0].ParentID)
	}
	if ra.Items[1].ParentID != "bp-001" {
		t.Errorf("parent_id = %q, muốn bp-001 — cây tổ chức mất quan hệ cha con", ra.Items[1].ParentID)
	}
}

// --- (2) one commune's chart never reaches another ----------------------------------------------

func TestBoPhanKhongVuotSangXaKhac(t *testing.T) {
	// The same account, signed in properly at commune B. Commune A's org chart must not travel with
	// the person. Nothing about the request is malformed — this is the shape a leak actually takes,
	// and the route being readable by EVERY signed-in account is exactly why it is asserted here.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostB, duongBoPhan, "", m.tokenCho(t, xaB, sidB))
	doiMa(t, w, http.StatusOK)

	than := w.Body.String()
	if strings.Contains(than, "VĂN PHÒNG ĐẢNG ỦY") || strings.Contains(than, "bp-001") {
		t.Fatalf("RÒ RỈ: ở xã B nhận được sơ đồ tổ chức của xã A: %s", than)
	}
	ra := docBoPhan(t, w.Body.Bytes())
	if len(ra.Items) != 1 || ra.Items[0].ID != "bp-b-001" {
		t.Fatalf("xã B phải nhận đúng bộ phận của mình, nhận: %+v", ra.Items)
	}
}

// --- (3) whole list, store's order, or refused --------------------------------------------------

func TestBoPhanGiuNguyenThuTuCuaKho(t *testing.T) {
	// `thu_tu` IS THE ORDER THE COMMUNE ARRANGED ITS OWN UNITS IN. The store sorts by it; a handler
	// that re-sorted — alphabetically, by id, by anything — would silently overrule the commune on
	// its own org chart. The fixture is deliberately NOT in alphabetical order ("VĂN PHÒNG…" before
	// "TỔ MỘT CỬA"), so any re-sort turns this red.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongBoPhan, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	ra := docBoPhan(t, w.Body.Bytes())
	if ra.Items[0].Code != "van-phong-dang-uy" || ra.Items[1].Code != "to-mot-cua" {
		t.Fatalf("thứ tự đã bị đổi: %v, %v", ra.Items[0].Code, ra.Items[1].Code)
	}
}

func TestBoPhanVuotTranThiTuChoiChuKhongCatBot(t *testing.T) {
	// THE DECISION THIS PINS, and it is the one that looks wrong at first glance: the route answers
	// 500 rather than returning the first N units.
	//
	// This list fills the box work is assigned in. A silently short list is a unit that has
	// disappeared from that box — the work goes to the wrong unit or to nobody, and every screen
	// looks entirely normal. A refusal breaks one commune's screen loudly and names itself in the
	// log. Between a wrong answer nobody notices and no answer somebody fixes, this chooses the
	// second.
	//
	// The ceiling itself lives in the store (idstore.TranDanhMucBoPhan) because only the store knows
	// the LIMIT; what is asserted here is that the handler does not quietly render the error away.
	m := dungMayChu(t)
	m.boPhan.loi = idstore.ErrQuaNhieuBoPhan

	w := m.goi(t, "GET", hostA, duongBoPhan, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusInternalServerError)

	e := loiTra(t, w)
	if e.Code != "internal" {
		t.Errorf("code = %q, muốn internal", e.Code)
	}
	// No partial list came back with the error. A body carrying items alongside a 500 is how a
	// refusal turns back into a truncation on a client that reads the body anyway.
	if strings.Contains(w.Body.String(), `"items"`) {
		t.Errorf("phản hồi từ chối vẫn kèm danh sách: %s", w.Body.String())
	}
}

func TestBoPhanLoiKhoTra500VaKhongLoNoiDungLoi(t *testing.T) {
	m := dungMayChu(t)
	m.boPhan.loi = errors.New("cơ sở dữ liệu không phản hồi")

	w := m.goi(t, "GET", hostA, duongBoPhan, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusInternalServerError)

	if e := loiTra(t, w); strings.Contains(e.Message, "cơ sở dữ liệu không phản hồi") {
		t.Errorf("lỗi nội bộ lọt ra ngoài: %q", e.Message)
	}
}

// --- (4) empty commune ---------------------------------------------------------------------------

func TestBoPhanXaChuaCauHinhTraMangRong(t *testing.T) {
	// [] AND NOT null. A newly onboarded commune has no org chart yet — an ordinary state, not an
	// error — and a client that has to handle both shapes handles one of them wrong.
	m := dungMayChu(t)
	m.boPhan.theo[xaA] = nil

	w := m.goi(t, "GET", hostA, duongBoPhan, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	if !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Errorf("items phải tuần tự hoá thành [], nhận: %s", w.Body.String())
	}
}
