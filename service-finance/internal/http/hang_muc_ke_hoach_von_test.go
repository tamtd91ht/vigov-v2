package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"testing"

	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// WHAT THIS FILE IS FOR: GET /api/v1/capital-plan-categories is the FIRST route in this service.
// Five things have to hold, and each fails silently if it stops holding:
//
//  1. the four cases of rule 5, invariant 7, read for an AnyAuthenticated route — see the note on
//     TestHangMuc_200KhongCanQuyenCauHinh for what the "403 wrong permission" case becomes here;
//  2. one commune's catalogue never reaches another commune's caller;
//  3. the list is returned WHOLE and in the store's order, or refused — never trimmed;
//  4. a commune with no rows serialises as [] and not null — which is EVERY commune today;
//  5. the response carries the five fields of the contract and nothing more.

const duongHangMuc = "/api/v1/capital-plan-categories"

func docHangMuc(t *testing.T, than []byte) danhSachHangMucRa {
	t.Helper()
	var ra danhSachHangMucRa
	if err := json.Unmarshal(than, &ra); err != nil {
		t.Fatalf("thân không phải JSON: %q", string(than))
	}
	return ra
}

// --- (1) the four cases -------------------------------------------------------------------------

func TestHangMuc_401KhongCoPhien(t *testing.T) {
	m := dungMayChu(t)

	doiMa(t, m.goi(t, "GET", hostA, duongHangMuc, nil), http.StatusUnauthorized)
	if m.hangMuc.goi != 0 {
		t.Error("chưa đăng nhập mà đã đọc danh mục hạng mục kế hoạch vốn")
	}
}

func TestHangMuc_200KhongCanQuyenCauHinh(t *testing.T) {
	// THE "403 WRONG PERMISSION" CASE, AS IT READS ON AN AnyAuthenticated ROUTE — and it is the
	// decision this route was declared with, so it is asserted rather than assumed.
	//
	// checkerGia grants NOTHING, to anybody, in any commune. The account must still get 200. That
	// is what AnyAuthenticated means here, and it is the whole reason the route is declared that
	// way: category names fill the classifier on a capital plan line and every filter beside it, so
	// a configuration permission on this list would empty those boxes for everybody who is not an
	// administrator.
	//
	// If somebody later "tightens" this to RequirePermission, this test goes red — which is the
	// point. The trade-off it protects is stated on the route: the catalogue of ONE commune is
	// readable by every signed-in account OF THAT COMMUNE.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongHangMuc, canBoCua(xaA))
	doiMa(t, w, http.StatusOK)
	if len(docHangMuc(t, w.Body.Bytes()).Items) == 0 {
		t.Error("tài khoản không có quyền nào nhận danh sách rỗng — tuyến này phải trả đủ")
	}
}

func TestHangMuc_401XaKhac(t *testing.T) {
	// THE "RIGHT PERMISSION, WRONG COMMUNE" CASE, as it reads here: there is no permission to be
	// right or wrong about, so what remains is an account of commune A presenting itself at commune
	// B's domain. authz.AnyAuthenticated compares the principal's commune with the one resolved
	// from Host and refuses — BEFORE the store is touched, which is what makes another commune's
	// catalogue unreachable rather than merely unrequested.
	//
	// THE ERROR `code` IS DELIBERATELY NOT ASSERTED. Today the refusal comes from authz, which
	// answers "unauthorized". When this service gets its own token layer, the refusal will move
	// earlier and answer "tenant_mismatch" the way the identity service already does — a correct
	// improvement that an assertion here would make look like a regression. What must not change is
	// the pair below: refused, and nothing read.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostB, duongHangMuc, canBoCua(xaA))
	doiMa(t, w, http.StatusUnauthorized)
	if m.hangMuc.goi != 0 {
		t.Error("tài khoản của xã khác mà vẫn đọc danh mục của xã này")
	}
}

func TestHangMuc_200(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongHangMuc, canBoCua(xaA))
	doiMa(t, w, http.StatusOK)

	ra := docHangMuc(t, w.Body.Bytes())
	if len(ra.Items) != 3 {
		t.Fatalf("nhận %d hạng mục, muốn 3", len(ra.Items))
	}
	if ra.Items[0] != (hangMucRa{
		ID: "hm-001", Code: "xay-dung-moi", Label: "Xây dựng mới", IsDefault: true, Active: true,
	}) {
		t.Errorf("hạng mục đầu sai: %+v", ra.Items[0])
	}

	// THE TWO BOOLEANS ARE ADJACENT AND A SWAP BETWEEN THEM IS INVISIBLE on any row where they
	// agree. hm-002 is the row where they do not: swapped, it would report a row the commune took
	// out of use as the one the form pre-selects, and nothing on any screen would say so.
	if ra.Items[1].IsDefault || !ra.Items[1].Active {
		t.Errorf("hm-002: is_default/active đã bị hoán đổi: %+v", ra.Items[1])
	}
}

// --- (2) one commune's catalogue never reaches another -------------------------------------------

func TestHangMucKhongVuotSangXaKhac(t *testing.T) {
	// The same person, signed in properly at commune B. Commune A's catalogue must not travel with
	// them. Nothing about the request is malformed — this is the shape a leak actually takes, and
	// the route being readable by EVERY signed-in account is exactly why it is asserted here.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostB, duongHangMuc, canBoCua(xaB))
	doiMa(t, w, http.StatusOK)

	than := w.Body.String()
	if strings.Contains(than, "xay-dung-moi") || strings.Contains(than, "hm-001") {
		t.Fatalf("RÒ RỈ: ở xã B nhận được danh mục của xã A: %s", than)
	}
	ra := docHangMuc(t, w.Body.Bytes())
	if len(ra.Items) != 1 || ra.Items[0].ID != "hm-b-001" {
		t.Fatalf("xã B phải nhận đúng danh mục của mình, nhận: %+v", ra.Items)
	}
}

// --- (3) whole list, store's order, or refused ---------------------------------------------------

func TestHangMucGiuNguyenThuTuCuaKho(t *testing.T) {
	// `thu_tu` IS THE ORDER THE COMMUNE ARRANGED ITS OWN CATALOGUE IN. The store sorts by it; a
	// handler that re-sorted — alphabetically, by id, by anything — would silently overrule the
	// commune on its own catalogue. The fixture is deliberately NOT in alphabetical order, by code
	// or by label, so any re-sort turns this red.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongHangMuc, canBoCua(xaA))
	doiMa(t, w, http.StatusOK)

	ra := docHangMuc(t, w.Body.Bytes())
	nhan := []string{ra.Items[0].Code, ra.Items[1].Code, ra.Items[2].Code}
	muon := []string{"xay-dung-moi", "cai-tao-nang-cap", "tra-no"}
	for i := range muon {
		if nhan[i] != muon[i] {
			t.Fatalf("thứ tự đã bị đổi: %v, muốn %v", nhan, muon)
		}
	}
	// Named explicitly so the failure above cannot be read as "the fixture happened to be sorted".
	sap := append([]string(nil), nhan...)
	sort.Strings(sap)
	if sap[0] == nhan[0] && sap[1] == nhan[1] && sap[2] == nhan[2] {
		t.Fatal("fixture đang xếp theo bảng chữ cái — ca này không còn phân biệt được handler có xếp lại hay không")
	}
}

func TestHangMucTatVanNamTrongDanhSach(t *testing.T) {
	// A ROW OUT OF USE IS STILL RETURNED, carrying active:false. Two reasons, and the second is the
	// expensive one: the catalogue screen lists it with a "Đã tắt" chip, and a capital plan line
	// recorded in an earlier budget year still holds that code AS A VALUE — a reader that never saw
	// the row would render an existing figure with no category name at all.
	//
	// What must NOT be returned is a SOFT-DELETED row, and that is a different question, settled in
	// SQL (rule 7, invariant 2). This route never sees one.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongHangMuc, canBoCua(xaA))
	doiMa(t, w, http.StatusOK)

	ra := docHangMuc(t, w.Body.Bytes())
	var thay bool
	for _, mot := range ra.Items {
		if mot.ID == "hm-003" {
			thay = true
			if mot.Active {
				t.Errorf("hm-003 đã tắt mà trả về active:true: %+v", mot)
			}
		}
	}
	if !thay {
		t.Error("hạng mục đã tắt bị loại khỏi danh sách — màn hình danh mục mất dòng, và dòng kế hoạch cũ mất tên hạng mục")
	}
}

func TestHangMucVuotTranThiTuChoiChuKhongCatBot(t *testing.T) {
	// THE DECISION THIS PINS, and it is the one that looks wrong at first glance: the route answers
	// 500 rather than returning the first N categories.
	//
	// This list fills the classifier on a capital plan line, and those figures are totalled and
	// reported upward. A silently short list is a category that has disappeared from that picker —
	// the line is filed under the wrong heading or under none, and every screen looks entirely
	// normal. A refusal breaks one commune's screen loudly and names itself in the log.
	//
	// The ceiling itself lives in the store (fistore.TranDanhMucHangMuc) because only the store
	// knows the LIMIT; what is asserted here is that the handler does not quietly render the error
	// away.
	m := dungMayChu(t)
	m.hangMuc.loi = fistore.ErrQuaNhieuHangMuc

	w := m.goi(t, "GET", hostA, duongHangMuc, canBoCua(xaA))
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

func TestHangMucLoiKhoTra500VaKhongLoNoiDungLoi(t *testing.T) {
	m := dungMayChu(t)
	m.hangMuc.loi = errors.New("cơ sở dữ liệu không phản hồi")

	w := m.goi(t, "GET", hostA, duongHangMuc, canBoCua(xaA))
	doiMa(t, w, http.StatusInternalServerError)

	if e := loiTra(t, w); strings.Contains(e.Message, "cơ sở dữ liệu không phản hồi") {
		t.Errorf("lỗi nội bộ lọt ra ngoài: %q", e.Message)
	}
}

// --- (4) the empty catalogue — the ordinary case today -------------------------------------------

func TestHangMucXaChuaCoDongNaoTraMangRong(t *testing.T) {
	// [] AND NOT null, AND THIS IS NOT AN EDGE CASE: the table ships empty for every commune, on
	// purpose (0003_danh_muc_hang_muc_ke_hoach_von.sql). Until commune onboarding sows the rows,
	// this is what the route returns for everybody, so it is the shape most likely to reach a real
	// client — and a client that has to handle both [] and null handles one of them wrong.
	m := dungMayChu(t)
	m.hangMuc.theo[xaA] = nil

	w := m.goi(t, "GET", hostA, duongHangMuc, canBoCua(xaA))
	doiMa(t, w, http.StatusOK)

	if !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Errorf("items phải tuần tự hoá thành [], nhận: %s", w.Body.String())
	}
}

// --- (5) the contract shape ----------------------------------------------------------------------

func TestHangMucChiTraNamTruongCuaHopDong(t *testing.T) {
	// THE FIELDS THAT ARE ABSENT ARE THE DESIGN, so their absence is asserted rather than assumed.
	//
	//	tenant_id    never leaves this service — it is not data, it is the dimension every row is
	//	             already filtered by (rule 1, invariant 4)
	//	thu_tu       the sort key, not data. Exposing it invites a client to re-sort, which is a
	//	             client overruling the commune on its own catalogue
	//	nguon,       they answer "what may be DONE to this row" — the three tiers of ADR 0024 §6.
	//	ma_nguon_    Only a configuration surface asks that, and open question #21 has not settled
	//	re_nhanh     who may do anything at all. Publishing the tier now would describe buttons
	//	             nobody has decided to allow
	//	deleted_at   a soft-deleted row never leaves the store, so no reader needs to ask
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongHangMuc, canBoCua(xaA))
	doiMa(t, w, http.StatusOK)

	var tho struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &tho); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	// `label`, NOT `name` — the field is `nhan`, and ADR 0017 names a contract field after what the
	// data IS. Every ADR 0024 catalogue in this system answers `label`; entities with a `ten` column
	// answer `name`. This literal is where a drift back to `name` turns red.
	muon := map[string]bool{"id": true, "code": true, "label": true, "is_default": true, "active": true}
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
