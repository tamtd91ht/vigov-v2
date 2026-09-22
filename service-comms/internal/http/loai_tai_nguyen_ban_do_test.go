package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// WHAT THIS FILE IS FOR: GET /api/v1/map-asset-types is the FIRST real route in this service, and
// five things have to hold. Each of them fails silently if it stops holding:
//
//  1. the four cases of rule 5, invariant 7, read for an AnyAuthenticated route — see the note on
//     TestLoaiTaiNguyen_200KhongCanQuyenCauHinh for what the "403 wrong permission" case becomes
//     here, and on TestLoaiTaiNguyen_401XaKhac for why the "wrong commune" case is a 401;
//  2. one commune's catalogue never reaches another commune's caller;
//  3. the list comes back WHOLE and in the store's order, or is refused — never trimmed;
//  4. an empty catalogue serialises as [] and not null. This is not an edge case today: the table
//     ships empty for EVERY commune, so it is the answer the whole system currently gets;
//  5. a group taken out of use is still in the list, carrying `active: false`;
//  6. the response carries EXACTLY the five contract fields and nothing else — see
//     TestLoaiTaiNguyenChiTraNamTruongCuaHopDong for why the absent ones matter more than the
//     present ones.

const duongLoaiTaiNguyen = "/api/v1/map-asset-types"

func docDanhMuc(t *testing.T, than []byte) danhSachLoaiTaiNguyenRa {
	t.Helper()
	var ra danhSachLoaiTaiNguyenRa
	if err := json.Unmarshal(than, &ra); err != nil {
		t.Fatalf("thân không phải JSON: %q", string(than))
	}
	return ra
}

// --- (1) the four cases -------------------------------------------------------------------------

func TestLoaiTaiNguyen_401KhongCoPhien(t *testing.T) {
	m := dungMayChu(t) // no daDangNhap: no principal at all

	w := m.goi(t, "GET", hostA, duongLoaiTaiNguyen)
	doiMa(t, w, http.StatusUnauthorized)
	if m.danhMuc.goi != 0 {
		t.Error("chưa đăng nhập mà đã đọc danh mục")
	}
}

func TestLoaiTaiNguyen_200KhongCanQuyenCauHinh(t *testing.T) {
	// THE "403 WRONG PERMISSION" CASE, AS IT READS ON AN AnyAuthenticated ROUTE — and it is the
	// decision this route was declared with, so it is asserted rather than assumed.
	//
	// The account holds NOTHING: checkerGia denies every permission in every commune. It must still
	// get 200, and the checker must not even be consulted. That is what AnyAuthenticated means
	// here, and it is the whole reason the route is declared that way: group names fill the selector
	// on the economic map, the filter beside it and the label of every asset already filed, so a
	// configuration permission on this list would empty those for everybody who is not an
	// administrator.
	//
	// If somebody later "tightens" this to RequirePermission, this test goes red — which is the
	// point. The trade-off it protects is stated on the route: the catalogue of ONE commune is
	// readable by every signed-in account OF THAT COMMUNE.
	m := dungMayChu(t)
	m.daDangNhap(xaA)

	w := m.goi(t, "GET", hostA, duongLoaiTaiNguyen)
	doiMa(t, w, http.StatusOK)
	if len(docDanhMuc(t, w.Body.Bytes()).Items) == 0 {
		t.Error("tài khoản không có quyền nào nhận danh sách rỗng — tuyến này phải trả đủ")
	}
	if m.checker.goi != 0 {
		t.Errorf("Checker được hỏi %d lần trên một tuyến AnyAuthenticated — khai báo và hành vi đã lệch nhau", m.checker.goi)
	}
}

func TestLoaiTaiNguyen_401XaKhac(t *testing.T) {
	// THE "RIGHT PERMISSION, WRONG COMMUNE" CASE, as it reads here — and the answer is 401, not 403,
	// which is a fact about core/authz rather than a choice made in this service: AnyAuthenticated
	// compares the principal's commune with the commune resolved from Host (authz.xacNhanXa) and
	// answers 401 for a mismatch, the same code it gives a caller with no session at all. There is
	// no permission to be right or wrong about, so 403 is not reachable on this route and the
	// @reply block does not claim it.
	//
	// The refusal happens BEFORE the store is touched, which is what makes another commune's
	// catalogue unreachable rather than merely unrequested.
	m := dungMayChu(t)
	m.daDangNhap(xaA)

	w := m.goi(t, "GET", hostB, duongLoaiTaiNguyen)
	doiMa(t, w, http.StatusUnauthorized)
	if m.danhMuc.goi != 0 {
		t.Error("phiên của xã khác mà vẫn đọc danh mục của xã này")
	}
}

func TestLoaiTaiNguyen_200(t *testing.T) {
	m := dungMayChu(t)
	m.daDangNhap(xaA)

	w := m.goi(t, "GET", hostA, duongLoaiTaiNguyen)
	doiMa(t, w, http.StatusOK)

	ra := docDanhMuc(t, w.Body.Bytes())
	if len(ra.Items) != 3 {
		t.Fatalf("nhận %d nhóm, muốn 3", len(ra.Items))
	}
	if ra.Items[0].ID != "ltn-001" || ra.Items[0].Code != "mau-mot" || ra.Items[0].Label != "Nhóm mẫu một" {
		t.Errorf("nhóm đầu sai: %+v", ra.Items[0])
	}
	// EXACTLY ONE DEFAULT. The database guarantees at most one live default per commune
	// (UNIQUE (tenant_id, moc_mac_dinh) over the generated column); what is asserted here is that
	// the flag survives the trip out, because the map's selector opens on it.
	var soMacDinh int
	for _, mot := range ra.Items {
		if mot.IsDefault {
			soMacDinh++
		}
	}
	if soMacDinh != 1 || !ra.Items[0].IsDefault {
		t.Errorf("mốc mặc định sai: %d nhóm mặc định, %+v", soMacDinh, ra.Items)
	}
}

// --- (2) one commune's catalogue never reaches another -------------------------------------------

func TestLoaiTaiNguyenKhongVuotSangXaKhac(t *testing.T) {
	// The same account, signed in properly at commune B. Commune A's catalogue must not travel with
	// the person. Nothing about the request is malformed — this is the shape a leak actually takes,
	// and the route being readable by EVERY signed-in account is exactly why it is asserted here.
	m := dungMayChu(t)
	m.daDangNhap(xaB)

	w := m.goi(t, "GET", hostB, duongLoaiTaiNguyen)
	doiMa(t, w, http.StatusOK)

	than := w.Body.String()
	if strings.Contains(than, "mau-mot") || strings.Contains(than, "ltn-001") {
		t.Fatalf("RÒ RỈ: ở xã B nhận được danh mục của xã A: %s", than)
	}
	ra := docDanhMuc(t, w.Body.Bytes())
	if len(ra.Items) != 1 || ra.Items[0].ID != "ltn-b-001" {
		t.Fatalf("xã B phải nhận đúng danh mục của mình, nhận: %+v", ra.Items)
	}
}

// --- (3) whole list, store's order, or refused ---------------------------------------------------

func TestLoaiTaiNguyenGiuNguyenThuTuCuaKho(t *testing.T) {
	// `thu_tu` IS THE ORDER THE COMMUNE ARRANGED ITS OWN GROUPS IN. The store sorts by it; a handler
	// that re-sorted — alphabetically, by id, by anything — would silently overrule the commune on
	// its own catalogue. The fixture is deliberately NOT in alphabetical order of the codes
	// (`mau-ba` sorts first and comes last), so any re-sort turns this red.
	m := dungMayChu(t)
	m.daDangNhap(xaA)

	w := m.goi(t, "GET", hostA, duongLoaiTaiNguyen)
	doiMa(t, w, http.StatusOK)

	ra := docDanhMuc(t, w.Body.Bytes())
	got := []string{ra.Items[0].Code, ra.Items[1].Code, ra.Items[2].Code}
	muon := []string{"mau-mot", "mau-hai", "mau-ba"}
	for i := range muon {
		if got[i] != muon[i] {
			t.Fatalf("thứ tự đã bị đổi: %v, muốn %v", got, muon)
		}
	}
}

func TestLoaiTaiNguyenVuotTranThiTuChoiChuKhongCatBot(t *testing.T) {
	// THE DECISION THIS PINS, and it is the one that looks wrong at first glance: the route answers
	// 500 rather than returning the first N groups.
	//
	// This list fills the selector an asset is filed under. A silently short list is a group that
	// has disappeared from that selector — the asset is filed under the wrong group or cannot be
	// filed at all, and every screen looks entirely normal. A refusal breaks one commune's screen
	// loudly and names itself in the log.
	//
	// The ceiling itself lives in the store (commsstore.TranDanhMucLoaiTaiNguyen) because only the
	// store knows the LIMIT; what is asserted here is that the handler does not quietly render the
	// error away.
	m := dungMayChu(t)
	m.daDangNhap(xaA)
	m.danhMuc.loi = commsstore.ErrQuaNhieuLoaiTaiNguyen

	w := m.goi(t, "GET", hostA, duongLoaiTaiNguyen)
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

func TestLoaiTaiNguyenLoiKhoTra500VaKhongLoNoiDungLoi(t *testing.T) {
	m := dungMayChu(t)
	m.daDangNhap(xaA)
	m.danhMuc.loi = errors.New("cơ sở dữ liệu không phản hồi")

	w := m.goi(t, "GET", hostA, duongLoaiTaiNguyen)
	doiMa(t, w, http.StatusInternalServerError)

	if e := loiTra(t, w); strings.Contains(e.Message, "cơ sở dữ liệu không phản hồi") {
		t.Errorf("lỗi nội bộ lọt ra ngoài: %q", e.Message)
	}
}

// --- (4) the empty catalogue — today's answer for EVERY commune ----------------------------------

func TestLoaiTaiNguyenDanhMucRongTraMangRong(t *testing.T) {
	// [] AND NOT null, and this is not a corner case: migration 0003 creates the table and seeds
	// NOTHING, for two reasons that are both still open (no commune-onboarding step, and a
	// specification that contradicts itself about the code list). So every commune is in this state
	// right now, and `null` here would be what the entire admin web receives.
	m := dungMayChu(t)
	m.daDangNhap(xaA)
	m.danhMuc.theo[xaA] = nil

	w := m.goi(t, "GET", hostA, duongLoaiTaiNguyen)
	doiMa(t, w, http.StatusOK)

	if !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Errorf("items phải tuần tự hoá thành [], nhận: %s", w.Body.String())
	}
}

// --- (5) a group taken out of use is still in the list -------------------------------------------

func TestLoaiTaiNguyenNhomDaTatVanTraVeKemCoActiveFalse(t *testing.T) {
	// A retired group is NOT filtered out here. Two consumers need it: the catalogue screen, which
	// shows it with a "Đã tắt" chip, and every asset already filed under it, which still has to
	// render its group's name. Only soft-deleted rows drop out, and that happens in the store.
	//
	// A picker filters on `active` itself — which it can only do if the flag actually travels.
	m := dungMayChu(t)
	m.daDangNhap(xaA)

	w := m.goi(t, "GET", hostA, duongLoaiTaiNguyen)
	doiMa(t, w, http.StatusOK)

	ra := docDanhMuc(t, w.Body.Bytes())
	if len(ra.Items) != 3 {
		t.Fatalf("nhóm đã tắt bị loại khỏi danh sách: %+v", ra.Items)
	}
	if ra.Items[2].Code != "mau-ba" || ra.Items[2].Active {
		t.Errorf("nhóm đã tắt phải mang active=false: %+v", ra.Items[2])
	}
	if !ra.Items[0].Active || !ra.Items[1].Active {
		t.Errorf("hai nhóm đang dùng phải mang active=true: %+v", ra.Items[:2])
	}
}

// --- (6) the contract shape ----------------------------------------------------------------------

func TestLoaiTaiNguyenChiTraTruongCuaHopDong(t *testing.T) {
	// THE FIELDS THAT ARE ABSENT ARE THE DESIGN, so their absence is asserted rather than assumed.
	// Unmarshalling into the typed struct cannot show this: an extra field added to loaiTaiNguyenRa
	// would ship on every response and every existing test would stay green.
	//
	//	tenant_id    never leaves this service — it is not data, it is the dimension every row is
	//	             already filtered by (rule 1, invariant 4)
	//	deleted_at   a soft-deleted row never leaves the store, so no reader needs to ask
	//
	// `order`, `source` AND `tier` ARE NOW IN THE CONTRACT, and the comment that used to stand here
	// said the opposite: that `thu_tu` invites a client to re-sort, and that `nguon` /
	// `ma_nguon_re_nhanh` describe buttons open question #21 had not settled. THAT READING IS OUT
	// OF DATE — #21 is about the TASK STATUS catalogue, whose codes a fixed state machine walks;
	// this catalogue's own answer is in the schema and is enforced by a trigger
	// (0003_danh_muc_loai_tai_nguyen_ban_do.sql:89-91). The write routes exist, and the
	// configuration screen has to know which buttons it may draw: `Tắt` is refused at tier 3, `Xoá`
	// at tiers 2 and 3.
	//
	// The re-sort argument still stands as an instruction to CLIENTS and is written on the field
	// itself; it was never an argument for hiding the value from a screen that edits it.
	//
	// It also pins the ONE name this field has. `label` and not `name`: the column is `nhan`, and a
	// catalogue row carries a label while a unit or a role carries a name (ADR 0017 — the argument
	// is on loaiTaiNguyenRa). A rename back turns this red instead of silently reshaping a contract.
	m := dungMayChu(t)
	m.daDangNhap(xaA)

	w := m.goi(t, "GET", hostA, duongLoaiTaiNguyen)
	doiMa(t, w, http.StatusOK)

	var tho struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &tho); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	if len(tho.Items) == 0 {
		t.Fatal("không có dòng nào để kiểm hình dạng hợp đồng")
	}
	muon := map[string]bool{
		"id": true, "code": true, "label": true, "is_default": true, "active": true,
		"order": true, "source": true, "tier": true,
	}
	for _, mot := range tho.Items {
		for khoa := range mot {
			if !muon[khoa] {
				t.Errorf("trường ngoài hợp đồng lọt ra: %q — %s", khoa, w.Body.String())
			}
		}
		if len(mot) != len(muon) {
			t.Errorf("thiếu trường: có %v, muốn %v", mot, muon)
		}
	}
}
