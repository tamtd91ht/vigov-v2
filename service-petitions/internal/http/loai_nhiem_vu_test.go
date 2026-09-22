package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// WHAT THIS FILE IS FOR: GET /api/v1/task-types is one of the two FIRST real routes in this
// service. Four things have to hold, and each fails silently if it stops holding:
//
//  1. the four cases of rule 5, invariant 7, read for an AnyAuthenticated route — see the note on
//     TestLoaiNhiemVu_200KhongCanQuyenCauHinh for what the "403 wrong permission" case becomes;
//  2. one commune's catalogue never reaches another commune's caller;
//  3. the list is returned WHOLE and in the store's order, or refused — never trimmed;
//  4. a commune with no rows serialises as [] and not null — which is EVERY commune today, because
//     migration 0003 ships both catalogues empty on purpose.

const duongLoaiNhiemVu = "/api/v1/task-types"

func docLoaiNhiemVu(t *testing.T, than []byte) danhSachLoaiNhiemVuRa {
	t.Helper()
	var ra danhSachLoaiNhiemVuRa
	if err := json.Unmarshal(than, &ra); err != nil {
		t.Fatalf("thân không phải JSON: %q", string(than))
	}
	return ra
}

// --- (1) the four cases -------------------------------------------------------------------------

func TestLoaiNhiemVu_401KhongPhien(t *testing.T) {
	m := dungMayChu(t)

	doiMa(t, m.goi(t, "GET", hostA, duongLoaiNhiemVu, nil), http.StatusUnauthorized)
	if m.loai.goi != 0 {
		t.Error("chưa đăng nhập mà đã đọc danh mục loại nhiệm vụ")
	}
}

func TestLoaiNhiemVu_200KhongCanQuyenCauHinh(t *testing.T) {
	// THE "403 WRONG PERMISSION" CASE, AS IT READS ON AN AnyAuthenticated ROUTE — and it is the
	// decision this route was declared with, so it is asserted rather than assumed.
	//
	// The account holds NOTHING: not a configuration permission, not anything. It must still get
	// 200. That is what AnyAuthenticated means here, and it is the whole reason for the
	// declaration: these names fill the type picker on the task form and the filter bar of nearly
	// every task screen, so a configuration permission on this list would empty those boxes for
	// everybody who is not an administrator.
	//
	// If somebody later "tightens" this to RequirePermission, this test goes red — which is the
	// point.
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{} // no grants at all, in any commune
	})

	w := m.goi(t, "GET", hostA, duongLoaiNhiemVu, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	if len(docLoaiNhiemVu(t, w.Body.Bytes()).Items) == 0 {
		t.Error("tài khoản không có quyền nào nhận danh sách rỗng — tuyến này phải trả đủ")
	}
}

func TestLoaiNhiemVu_401XaKhac(t *testing.T) {
	// THE "RIGHT PERMISSION, WRONG COMMUNE" CASE, as it reads here: there is no permission to be
	// right or wrong about, so what remains is a principal issued by commune A arriving at commune
	// B's domain. authz.AnyAuthenticated refuses it with 401 BEFORE any store is touched — which is
	// what makes the catalogue unreachable across communes rather than merely unrequested.
	//
	// 401 AND NOT 403 IS THE CONTRACT, and it is what the route's @reply lines declare: a browser
	// does not send a cookie across hosts, so a mismatch is never an ordinary user error.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostB, duongLoaiNhiemVu, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusUnauthorized)
	if got := loiTra(t, w).Code; got != "unauthorized" {
		t.Errorf("code = %q, muốn unauthorized", got)
	}
	if m.loai.goi != 0 {
		t.Error("phiên của xã khác mà vẫn đọc danh mục của xã này")
	}
}

func TestLoaiNhiemVu_200(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongLoaiNhiemVu, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	ra := docLoaiNhiemVu(t, w.Body.Bytes())
	if len(ra.Items) != 3 {
		t.Fatalf("nhận %d loại nhiệm vụ, muốn 3", len(ra.Items))
	}
	if ra.Items[0].ID != "lnv-001" || ra.Items[0].Code != "theo-van-ban" ||
		ra.Items[0].Label != "Theo văn bản" {
		t.Errorf("dòng đầu sai: %+v", ra.Items[0])
	}
	// EXACTLY ONE DEFAULT. The database holds that (UNIQUE (tenant_id, moc_mac_dinh)), and the
	// response must carry it through unchanged: a form that finds two pre-selects whichever it
	// reads first, which is to say at random, with nothing on the screen showing that it did.
	var soMacDinh int
	for _, mot := range ra.Items {
		if mot.IsDefault {
			soMacDinh++
		}
	}
	if soMacDinh != 1 {
		t.Errorf("có %d dòng mặc định, muốn đúng 1", soMacDinh)
	}
	if !ra.Items[0].IsDefault {
		t.Error("dòng mặc định không phải theo-van-ban — biểu mẫu sẽ chọn sẵn sai mục")
	}
}

// --- (2) one commune's catalogue never reaches another ------------------------------------------

func TestLoaiNhiemVuKhongVuotSangXaKhac(t *testing.T) {
	// The same account, signed in properly at commune B. Commune A's catalogue must not travel with
	// the person. Nothing about the request is malformed — this is the shape a leak actually takes,
	// and the route being readable by EVERY signed-in account is exactly why it is asserted here.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostB, duongLoaiNhiemVu, canBoCuaXa(xaB))
	doiMa(t, w, http.StatusOK)

	than := w.Body.String()
	if strings.Contains(than, "theo-van-ban") || strings.Contains(than, "lnv-001") {
		t.Fatalf("RÒ RỈ: ở xã B nhận được danh mục của xã A: %s", than)
	}
	ra := docLoaiNhiemVu(t, w.Body.Bytes())
	if len(ra.Items) != 1 || ra.Items[0].ID != "lnv-b-001" {
		t.Fatalf("xã B phải nhận đúng danh mục của mình, nhận: %+v", ra.Items)
	}
}

// --- (3) whole list, store's order, or refused --------------------------------------------------

func TestLoaiNhiemVuGiuNguyenThuTuCuaKho(t *testing.T) {
	// `thu_tu` IS THE ORDER THE COMMUNE ARRANGED ITS OWN CATALOGUE IN. The store sorts by it; a
	// handler that re-sorted — by code, by label, by id — would silently overrule the commune. The
	// fixture is deliberately neither alphabetical by code nor by label, so any re-sort turns this
	// red.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongLoaiNhiemVu, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	ra := docLoaiNhiemVu(t, w.Body.Bytes())
	muon := []string{"theo-van-ban", "co-ban", "viec-cu"}
	for i, ma := range muon {
		if ra.Items[i].Code != ma {
			t.Fatalf("thứ tự đã bị đổi ở vị trí %d: %q, muốn %q", i, ra.Items[i].Code, ma)
		}
	}
}

func TestLoaiNhiemVuGiuCaDongDaTat(t *testing.T) {
	// A row taken out of use STAYS IN THE LIST, flagged. A task recorded last year may hold that
	// code, and a list that dropped the row would leave that task showing a raw code with no label
	// — the display failure ADR 0024 cites as already sitting in the specification. Filtering for a
	// picker of NEW choices is the client's job, and `active` is what it filters on.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongLoaiNhiemVu, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	ra := docLoaiNhiemVu(t, w.Body.Bytes())
	if ra.Items[2].Code != "viec-cu" {
		t.Fatalf("dòng đã tắt biến mất khỏi danh sách: %+v", ra.Items)
	}
	if ra.Items[2].Active {
		t.Error("active = true cho dòng đã tắt — ô chọn sẽ mời người dùng chọn một mục xã đã ngừng dùng")
	}
	if !ra.Items[0].Active {
		t.Error("active = false cho dòng đang dùng — cả trường này chỉ là hằng số thì không kiểm được gì")
	}
}

func TestLoaiNhiemVuVuotTranThiTuChoiChuKhongCatBot(t *testing.T) {
	// THE DECISION THIS PINS, and it is the one that looks wrong at first glance: the route answers
	// 500 rather than returning the first N rows.
	//
	// This list fills the type picker on the task form. A silently short list files work under the
	// wrong type, or hides tasks behind a filter that no longer offers their type, and every screen
	// looks entirely normal. A refusal breaks ONE commune's screen loudly and names itself in the
	// log.
	//
	// The ceiling itself lives in the store (petstore.TranDanhMucLoaiNhiemVu) because only the
	// store knows the LIMIT; what is asserted here is that the handler does not quietly render the
	// error away.
	m := dungMayChu(t)
	m.loai.loi = petstore.ErrQuaNhieuLoaiNhiemVu

	w := m.goi(t, "GET", hostA, duongLoaiNhiemVu, canBoCuaXa(xaA))
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

func TestLoaiNhiemVuLoiKhoTra500VaKhongLoNoiDungLoi(t *testing.T) {
	m := dungMayChu(t)
	m.loai.loi = errors.New("cơ sở dữ liệu không phản hồi")

	w := m.goi(t, "GET", hostA, duongLoaiNhiemVu, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusInternalServerError)

	if e := loiTra(t, w); strings.Contains(e.Message, "cơ sở dữ liệu không phản hồi") {
		t.Errorf("lỗi nội bộ lọt ra ngoài: %q", e.Message)
	}
}

// --- (4) a commune with no rows -----------------------------------------------------------------

func TestLoaiNhiemVuXaChuaCoDongNaoTraMangRong(t *testing.T) {
	// [] AND NOT null — and this is not an edge case today, it is EVERY commune: migration 0003
	// creates both catalogues and seeds nothing, because a catalogue row carries tenant_id and the
	// step that sows a commune's first rows does not exist in this repository yet. An empty list is
	// the correct answer; a client that has to handle both [] and null handles one of them wrong.
	m := dungMayChu(t)
	m.loai.theo[xaA] = nil

	w := m.goi(t, "GET", hostA, duongLoaiNhiemVu, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	if !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Errorf("items phải tuần tự hoá thành [], nhận: %s", w.Body.String())
	}
}

// --- (5) the contract shape ---------------------------------------------------------------------

func TestLoaiNhiemVuChiTraTruongCuaHopDong(t *testing.T) {
	// THE FIELDS THAT ARE ABSENT ARE THE DESIGN, so their absence is asserted rather than assumed.
	// The struct is decoded into a map here precisely so a field ADDED to loaiNhiemVuRa turns this
	// red instead of quietly shipping:
	//
	//	tenant_id    never leaves this service — it is not data, it is the dimension every row is
	//	             already filtered by (rule 1, invariant 4)
	//	deleted_at   a soft-deleted row never leaves the store, so no reader needs to ask
	//
	// `order`, `source` AND `tier` ARE NOW IN THE CONTRACT, and the comment that used to stand here
	// said the opposite: that `thu_tu` invites a client to re-sort, and that `nguon` /
	// `ma_nguon_re_nhanh` describe buttons open question #21 had not settled. THE SECOND HALF OF
	// THAT READING IS OUT OF DATE FOR THESE TWO CATALOGUES, and the distinction matters: #21 is
	// about the TASK STATUS catalogue (`trang_thai_nhiem_vu`), whose codes a fixed state machine
	// walks — adding one makes a status with no way in and no way out, and disabling `hoan-thanh`
	// stops every task in the commune from ever finishing. `loai_nhiem_vu` and
	// `muc_uu_tien_nhiem_vu` have no state machine, and their own answer is in the schema, enforced
	// by a trigger (0003_danh_muc_nhiem_vu.sql). The write routes exist, and the configuration
	// screen has to know which buttons it may draw: `Tắt` is refused at tier 3, `Xoá` at tiers 2
	// and 3.
	//
	// The re-sort argument still stands as an instruction to CLIENTS and is written on the field
	// itself; it was never an argument for hiding the value from a screen that edits it.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongLoaiNhiemVu, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	var tho struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &tho); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	if len(tho.Items) == 0 {
		t.Fatal("không có mục nào để kiểm hình dạng — fixture rỗng thì test này không khẳng định gì")
	}
	// `label`, NOT `name` — the field is `nhan`, and ADR 0017 names a contract field after what the
	// data IS. Every ADR 0024 catalogue in this system answers `label`; entities with a `ten` column
	// answer `name`. This literal is where a drift back to `name` turns red, and it is also where
	// `is_active` would.
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
