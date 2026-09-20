package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// WHAT THIS FILE IS FOR: GET /api/v1/residential-units is the first route in this service that
// returns a JOINED shape — each unit carries the LABEL of its type, read from a second table. Six
// things have to hold, and every one of them fails silently if it stops holding:
//
//  1. the four cases of rule 5, invariant 7, read for an AnyAuthenticated route — see the note on
//     TestThonToDanPho_200KhongCanQuyenCauHinh for what the "403 wrong permission" case becomes;
//  2. one commune's hamlets never reach another commune's caller;
//  3. the type label travels with the unit, and a unit whose type is missing STILL COMES BACK;
//  4. `null` and `0` stay apart on the two counts, all the way into the JSON;
//  5. the list is returned WHOLE and in the store's order, or refused — never trimmed;
//  6. an empty commune serialises as [] and not null — which is EVERY commune today, because
//     migration 0005 seeds nothing.

const duongThonToDanPho = "/api/v1/residential-units"

func docThonToDanPho(t *testing.T, than []byte) danhSachThonToDanPhoRa {
	t.Helper()
	var ra danhSachThonToDanPhoRa
	if err := json.Unmarshal(than, &ra); err != nil {
		t.Fatalf("thân không phải JSON: %q", string(than))
	}
	return ra
}

// --- (1) the four cases -------------------------------------------------------------------------

func TestThonToDanPho_401KhongToken(t *testing.T) {
	m := dungMayChu(t)

	doiMa(t, m.goi(t, "GET", hostA, duongThonToDanPho, "", ""), http.StatusUnauthorized)
	if m.thonToDanPho.goi != 0 {
		t.Error("chưa đăng nhập mà đã đọc danh sách thôn/tổ dân phố")
	}
}

func TestThonToDanPho_200KhongCanQuyenCauHinh(t *testing.T) {
	// THE "403 WRONG PERMISSION" CASE, AS IT READS ON AN AnyAuthenticated ROUTE — and it is the
	// decision the route was declared with, so it is asserted rather than assumed.
	//
	// The account is rebuilt holding NOTHING: not admin.user, not anything. It must still get 200.
	// That is what AnyAuthenticated means here: hamlet names fill the address picker on a petition,
	// the household record and every filter that groups work by area, so a configuration permission
	// on this list would empty those boxes for everybody who is not an administrator.
	//
	// If somebody later "tightens" this to RequirePermission, this test goes red — which is the
	// point. The trade-off it protects is stated on the route: the list is readable by every
	// signed-in account OF THAT COMMUNE.
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{} // no grants at all, in any commune
	})

	w := m.goi(t, "GET", hostA, duongThonToDanPho, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)
	if len(docThonToDanPho(t, w.Body.Bytes()).Items) == 0 {
		t.Error("tài khoản không có quyền nào nhận danh sách rỗng — tuyến này phải trả đủ")
	}
}

func TestThonToDanPho_401XaKhac(t *testing.T) {
	// THE "RIGHT PERMISSION, WRONG COMMUNE" CASE, as it reads here: there is no permission to be
	// right or wrong about, so what remains is a token issued by commune A presented at commune B's
	// domain. It is refused at the token layer, BEFORE any store is touched — which is what makes
	// the list unreachable across communes rather than merely unrequested.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostB, duongThonToDanPho, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusUnauthorized)
	if got := loiTra(t, w).Code; got != "tenant_mismatch" {
		t.Errorf("code = %q, muốn tenant_mismatch", got)
	}
	if m.thonToDanPho.goi != 0 {
		t.Error("token của xã khác mà vẫn đọc danh sách của xã này")
	}
}

func TestThonToDanPho_200(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongThonToDanPho, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	ra := docThonToDanPho(t, w.Body.Bytes())
	if len(ra.Items) != 3 {
		t.Fatalf("nhận %d thôn/tổ dân phố, muốn 3", len(ra.Items))
	}

	mot := ra.Items[0]
	if mot.ID != "tt-001" || mot.Code != "thon-binh-an" || mot.Name != "Thôn Bình An" {
		t.Errorf("bản ghi đầu sai: %+v", mot)
	}
	// THE JOINED FIELD. Without it the list screen has to call the type catalogue as well and join
	// the two in the browser — the cost the response shape was chosen to avoid.
	if mot.TypeCode != "thon" || mot.TypeLabel != "Thôn" {
		t.Errorf("loại đơn vị không đi kèm: type_code=%q type_label=%q", mot.TypeCode, mot.TypeLabel)
	}
	if mot.HouseholdCount == nil || *mot.HouseholdCount != 284 {
		t.Errorf("số hộ sai: %v", mot.HouseholdCount)
	}
	if mot.PopulationCount == nil || *mot.PopulationCount != 1132 {
		t.Errorf("nhân khẩu sai: %v", mot.PopulationCount)
	}
	if !mot.Active {
		t.Error("active = false với đơn vị đang dùng")
	}
	// The unit the commune has taken out of use is RETURNED, with the flag telling a picker to
	// leave it out. Filtering it away server-side would leave the list screen unable to show what
	// it manages.
	if ra.Items[2].Active {
		t.Errorf("active = true với đơn vị đã tắt: %+v", ra.Items[2])
	}
}

// --- (2) one commune's hamlets never reach another ----------------------------------------------

func TestThonToDanPhoKhongVuotSangXaKhac(t *testing.T) {
	// The same account, signed in properly at commune B. Commune A's hamlets must not travel with
	// the person. Nothing about the request is malformed — this is the shape a leak actually takes,
	// and the route being readable by EVERY signed-in account is exactly why it is asserted here.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostB, duongThonToDanPho, "", m.tokenCho(t, xaB, sidB))
	doiMa(t, w, http.StatusOK)

	than := w.Body.String()
	if strings.Contains(than, "Thôn Bình An") || strings.Contains(than, "tt-001") {
		t.Fatalf("RÒ RỈ: ở xã B nhận được thôn của xã A: %s", than)
	}
	ra := docThonToDanPho(t, w.Body.Bytes())
	if len(ra.Items) != 1 || ra.Items[0].ID != "tt-b-001" {
		t.Fatalf("xã B phải nhận đúng địa bàn của mình, nhận: %+v", ra.Items)
	}
}

// --- (3) the type label, and the unit that outlives its type ------------------------------------

func TestThonToDanPhoChuaPhanLoaiVanTraVe(t *testing.T) {
	// `loai` IS NULLABLE and `—` is a legitimate value on the commune's own screen
	// (14-cau-hinh.md:54). An inner join in the store would drop this row entirely, and a hamlet
	// missing from the list is exactly the failure the ceiling below refuses to cause — except
	// nothing would log it.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongThonToDanPho, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	ra := docThonToDanPho(t, w.Body.Bytes())
	var thay bool
	for _, mot := range ra.Items {
		if mot.ID == "tt-002" {
			thay = true
			if mot.TypeCode != "" || mot.TypeLabel != "" {
				t.Errorf("đơn vị chưa phân loại lại có loại: %+v", mot)
			}
		}
	}
	if !thay {
		t.Fatal("đơn vị chưa phân loại biến mất khỏi danh sách")
	}
}

func TestThonToDanPhoGiuMaLoaiKhiNhanKhongCon(t *testing.T) {
	// THE PAIR A CLIENT MUST BE ABLE TO RENDER: a type code with no label, because the catalogue row
	// was soft-deleted while units still carry its code.
	//
	// Two things are asserted at once and both matter. The unit SURVIVES — in the store that is
	// `deleted_at IS NULL` sitting in the JOIN rather than in the WHERE, and in the WHERE it would
	// turn untidy catalogue data into "this hamlet does not exist". And the CODE survives with it,
	// so the screen can fall back to showing the code instead of a blank cell.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongThonToDanPho, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	ra := docThonToDanPho(t, w.Body.Bytes())
	var thay bool
	for _, mot := range ra.Items {
		if mot.ID == "tt-003" {
			thay = true
			if mot.TypeCode != "to-dan-pho" {
				t.Errorf("mã loại mất theo nhãn: %+v", mot)
			}
			if mot.TypeLabel != "" {
				t.Errorf("nhãn loại đã xoá mềm vẫn hiện: %q", mot.TypeLabel)
			}
		}
	}
	if !thay {
		t.Fatal("đơn vị mang mã loại đã xoá mềm bị rơi khỏi danh sách")
	}
}

// --- (4) null is not zero -----------------------------------------------------------------------

func TestThonToDanPhoKhongNhapSoLieuKhacVoiSoKhong(t *testing.T) {
	// THE ASSERTION IS ON THE RAW JSON, not on the decoded struct, because this is a fact about the
	// wire: 0 is a statement about a unit ("no households"), null is the absence of one ("not
	// entered"). A commune that imported a spreadsheet without those columns must not have zeros
	// published on its behalf — a zero travels onward into a report as a number and nothing
	// downstream can tell it from a counted zero.
	//
	// tt-002 entered neither figure; tt-003 entered a real zero. Both shapes must appear.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongThonToDanPho, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	than := w.Body.String()
	if !strings.Contains(than, `"household_count":null`) {
		t.Errorf("số hộ chưa nhập phải là null: %s", than)
	}
	if !strings.Contains(than, `"household_count":0`) {
		t.Errorf("số hộ bằng 0 phải là 0: %s", than)
	}

	ra := docThonToDanPho(t, w.Body.Bytes())
	for _, mot := range ra.Items {
		switch mot.ID {
		case "tt-002":
			if mot.HouseholdCount != nil || mot.PopulationCount != nil {
				t.Errorf("chưa nhập mà có số: %+v", mot)
			}
		case "tt-003":
			if mot.HouseholdCount == nil || *mot.HouseholdCount != 0 {
				t.Errorf("số 0 đã nhập bị mất: %v", mot.HouseholdCount)
			}
		}
	}
}

func TestThonToDanPhoTraDungNhungTruongCuaHopDong(t *testing.T) {
	// THE FIELDS THAT ARE ABSENT ARE THE DESIGN, asserted rather than assumed — and this route needs
	// it more than the two catalogues do, because its shape is a JOIN and a join is where an extra
	// column arrives without anybody deciding to publish it.
	//
	//	tenant_id        never leaves this service — not data, but the dimension every row is
	//	                 already filtered by (rule 1, invariant 4)
	//	deleted_at,      a soft-deleted unit never leaves the store, so no reader needs to ask who
	//	deleted_by,      removed it or why. Those three are the commune's internal record of an
	//	delete_reason    administrative act, not part of a list of places
	//	the type's own   `type_code` + `type_label` and NOTHING ELSE of loai_don_vi_dan_cu. No `id`,
	//	fields           no `is_default`, no `active` of the TYPE: that is the embedded-object shape
	//	                 argued against on thonToDanPhoRa, and it would make this response change
	//	                 whenever the catalogue's shape changes
	//
	// THE SET IS THIS ROUTE'S OWN AND DELIBERATELY NOT THE CATALOGUES': `name` rather than `label`,
	// because a unit HAS a name (`ten`) while a catalogue row carries a label put on a code.
	//
	// THIS IS A CONTRACT, NOT A SNAPSHOT: when it goes red, the question is whether the ROUTE should
	// have changed — `make kb` regenerates the OpenAPI contract from these types, and web-admin's
	// types from that.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongThonToDanPho, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	var tho struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &tho); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	// NON-EMPTY FIRST: on an empty list the loop below runs zero times and passes having checked
	// nothing — and empty is what every commune answers today (migration 0005 seeds nothing).
	if len(tho.Items) == 0 {
		t.Fatal("dữ liệu mẫu rỗng — phép kiểm sẽ xanh mà không kiểm gì")
	}

	muon := map[string]bool{
		"id": true, "code": true, "name": true,
		"type_code": true, "type_label": true,
		// PRESENT EVEN WHEN null: the two counts are pointers so "not entered" survives as null
		// rather than as a counted zero, and a key that disappeared when the value was absent would
		// make a client read "not entered" and "field gone" the same way.
		"household_count": true, "population_count": true,
		"active": true,
	}
	for _, mot := range tho.Items {
		for khoa := range mot {
			if !muon[khoa] {
				t.Errorf("trường ngoài hợp đồng lọt ra: %q — %s", khoa, w.Body.String())
			}
		}
		// The length check is what catches a REMOVED field: the loop above only sees keys that are
		// present. It also catches `omitempty` creeping onto the two nullable counts.
		if len(mot) != len(muon) {
			t.Errorf("thiếu trường: có %v, muốn %v", mot, muon)
		}
	}
}

// --- (5) whole list, store's order, or refused --------------------------------------------------

func TestThonToDanPhoGiuNguyenThuTuCuaKho(t *testing.T) {
	// The store orders by name, with the code breaking ties so the order is TOTAL. A handler that
	// re-sorted — by id, by type, by anything — would overrule it silently, and a list that changes
	// order between two reloads makes every client-side diff flicker. The fixture is deliberately
	// not in id order.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongThonToDanPho, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	ra := docThonToDanPho(t, w.Body.Bytes())
	muon := []string{"thon-binh-an", "thon-chua-phan-loai", "to-dan-pho-so-1"}
	for i, ma := range muon {
		if ra.Items[i].Code != ma {
			t.Fatalf("thứ tự đã bị đổi ở vị trí %d: %q, muốn %q", i, ra.Items[i].Code, ma)
		}
	}
}

func TestThonToDanPhoVuotTranThiTuChoiChuKhongCatBot(t *testing.T) {
	// THE DECISION THIS PINS, and it is the one that looks wrong at first glance: the route answers
	// 500 rather than returning the first N units.
	//
	// This list fills the address picker on a petition. A silently short list is a hamlet that has
	// disappeared from that picker — the report is filed against the wrong place or against none,
	// and every screen looks entirely normal. A refusal breaks one commune's screen loudly and
	// names itself in the log.
	//
	// The ceiling itself lives in the store (idstore.TranDanhSachThonToDanPho) because only the
	// store knows the LIMIT; what is asserted here is that the handler does not render the error
	// away.
	m := dungMayChu(t)
	m.thonToDanPho.loi = idstore.ErrQuaNhieuThonToDanPho

	w := m.goi(t, "GET", hostA, duongThonToDanPho, "", m.tokenCho(t, xaA, sidA))
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

func TestThonToDanPhoLoiKhoTra500VaKhongLoNoiDungLoi(t *testing.T) {
	m := dungMayChu(t)
	m.thonToDanPho.loi = errors.New("cơ sở dữ liệu không phản hồi")

	w := m.goi(t, "GET", hostA, duongThonToDanPho, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusInternalServerError)

	if e := loiTra(t, w); strings.Contains(e.Message, "cơ sở dữ liệu không phản hồi") {
		t.Errorf("lỗi nội bộ lọt ra ngoài: %q", e.Message)
	}
}

// --- (6) empty commune --------------------------------------------------------------------------

func TestThonToDanPhoXaChuaNhapTraMangRong(t *testing.T) {
	// [] AND NOT null. This is not an edge case: migration 0005 creates the table and seeds nothing
	// on purpose, and the onboarding step that would sow a commune's first rows does not exist in
	// this repository yet — so an empty list is the ONLY answer any commune gets today. A client
	// that has to handle both [] and null handles one of them wrong.
	m := dungMayChu(t)
	m.thonToDanPho.theo[xaA] = nil

	w := m.goi(t, "GET", hostA, duongThonToDanPho, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	if !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Errorf("items phải tuần tự hoá thành [], nhận: %s", w.Body.String())
	}
}
