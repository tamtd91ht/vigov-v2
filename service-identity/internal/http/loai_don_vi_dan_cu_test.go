package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// WHAT THIS FILE IS FOR: GET /api/v1/residential-unit-types is a reference catalogue read, and the
// five things that have to hold are the ones that fail silently:
//
//  1. the four cases of rule 5, invariant 7, read for an AnyAuthenticated route;
//  2. one commune's catalogue never reaches another commune's caller;
//  3. the two flags a form depends on — is_default and active — really travel;
//  4. the list is returned WHOLE and in the store's order, or refused — never trimmed;
//  5. an empty commune serialises as [] and not null, which is EVERY commune today because
//     migration 0005 seeds nothing.

const duongLoaiDonViDanCu = "/api/v1/residential-unit-types"

func docLoaiDonViDanCu(t *testing.T, than []byte) danhSachLoaiDonViDanCuRa {
	t.Helper()
	var ra danhSachLoaiDonViDanCuRa
	if err := json.Unmarshal(than, &ra); err != nil {
		t.Fatalf("thân không phải JSON: %q", string(than))
	}
	return ra
}

// --- (1) the four cases -------------------------------------------------------------------------

func TestLoaiDonViDanCu_401KhongToken(t *testing.T) {
	m := dungMayChu(t)

	doiMa(t, m.goi(t, "GET", hostA, duongLoaiDonViDanCu, "", ""), http.StatusUnauthorized)
	if m.loaiDonViDanCu.goi != 0 {
		t.Error("chưa đăng nhập mà đã đọc danh mục loại đơn vị dân cư")
	}
}

func TestLoaiDonViDanCu_200KhongCanQuyenCauHinh(t *testing.T) {
	// THE "403 WRONG PERMISSION" CASE AS IT READS ON AN AnyAuthenticated ROUTE — asserted rather
	// than assumed, because it IS the decision the route was declared with. The account holds no
	// permission at all and must still get 200: these labels fill the `Loại` column of the
	// residential-unit list and the picker on its form, so a configuration permission here would
	// blank that column for everybody who is not an administrator.
	//
	// A later "tightening" to RequirePermission turns this red, which is the point.
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{} // no grants at all, in any commune
	})

	w := m.goi(t, "GET", hostA, duongLoaiDonViDanCu, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)
	if len(docLoaiDonViDanCu(t, w.Body.Bytes()).Items) == 0 {
		t.Error("tài khoản không có quyền nào nhận danh mục rỗng — tuyến này phải trả đủ")
	}
}

func TestLoaiDonViDanCu_401XaKhac(t *testing.T) {
	// "RIGHT PERMISSION, WRONG COMMUNE" as it reads here: there is no permission to be right or
	// wrong about, so what remains is commune A's token presented at commune B's domain. Refused at
	// the token layer, BEFORE any store is touched.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostB, duongLoaiDonViDanCu, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusUnauthorized)
	if got := loiTra(t, w).Code; got != "tenant_mismatch" {
		t.Errorf("code = %q, muốn tenant_mismatch", got)
	}
	if m.loaiDonViDanCu.goi != 0 {
		t.Error("token của xã khác mà vẫn đọc danh mục của xã này")
	}
}

func TestLoaiDonViDanCu_200(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongLoaiDonViDanCu, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	ra := docLoaiDonViDanCu(t, w.Body.Bytes())
	if len(ra.Items) != 2 {
		t.Fatalf("nhận %d mục, muốn 2", len(ra.Items))
	}
	mot := ra.Items[0]
	if mot.ID != "ldv-001" || mot.Code != "thon" || mot.Label != "Thôn" {
		t.Errorf("mục đầu sai: %+v", mot)
	}
	// THE ORDER IS THE STORE'S — `thu_tu`, the order the commune arranged its own catalogue in,
	// with the code breaking ties so it is total. A handler that re-sorted would overrule the
	// commune on its own configuration.
	if ra.Items[1].Code != "to-dan-pho" {
		t.Errorf("thứ tự đã bị đổi: %q", ra.Items[1].Code)
	}
}

// --- (2) one commune's catalogue never reaches another ------------------------------------------

func TestLoaiDonViDanCuKhongVuotSangXaKhac(t *testing.T) {
	// The same account, signed in properly at commune B. Nothing about the request is malformed —
	// this is the shape a leak actually takes.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostB, duongLoaiDonViDanCu, "", m.tokenCho(t, xaB, sidB))
	doiMa(t, w, http.StatusOK)

	than := w.Body.String()
	if strings.Contains(than, "ldv-001") || strings.Contains(than, `"label":"Thôn"`) {
		t.Fatalf("RÒ RỈ: ở xã B nhận được danh mục của xã A: %s", than)
	}
	ra := docLoaiDonViDanCu(t, w.Body.Bytes())
	if len(ra.Items) != 1 || ra.Items[0].ID != "ldv-b-001" {
		t.Fatalf("xã B phải nhận đúng danh mục của mình, nhận: %+v", ra.Items)
	}
}

// --- (3) the two flags a form depends on --------------------------------------------------------

func TestLoaiDonViDanCuMangCoMacDinhVaDangDung(t *testing.T) {
	// BOTH FLAGS, AND THEY DIFFER PER ROW IN THE FIXTURE, so this can tell "the flag is read" from
	// "the flag is always false".
	//
	// is_default: the schema spends a generated column and a unique key guaranteeing at most one
	// default per commune (migration 0005:244). If no read path carried it out, that guarantee
	// would be unreachable and a form would pre-select whichever row sorted first.
	//
	// active: an out-of-use row is RETURNED, not filtered — the catalogue screen must show it
	// with its "Đã tắt" chip while a picker must not offer it. Drop the flag and a disabled option
	// lands in a form with nothing on the screen to show it.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongLoaiDonViDanCu, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	ra := docLoaiDonViDanCu(t, w.Body.Bytes())
	if !ra.Items[0].IsDefault || !ra.Items[0].Active {
		t.Errorf("mục mặc định đang dùng bị báo sai: %+v", ra.Items[0])
	}
	if ra.Items[1].IsDefault {
		t.Errorf("hai mục cùng là mặc định: %+v", ra.Items[1])
	}
	if ra.Items[1].Active {
		t.Errorf("mục đã tắt bị báo là đang dùng: %+v", ra.Items[1])
	}
}

func TestLoaiDonViDanCuKhongPhoiRaNguonVaCoReNhanh(t *testing.T) {
	// `nguon` and `ma_nguon_re_nhanh` answer "what may be DONE to this row", and nothing may be
	// done to it through this API — there is no write route (open question #21). Publishing them
	// would describe a write surface that does not exist, and the first client to grey out a button
	// from them would be enforcing in the browser a rule the server is the one enforcing.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongLoaiDonViDanCu, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	than := w.Body.String()
	for _, cam := range []string{"nguon", "source", "re_nhanh", "ma_nguon", "he-thong", "don-vi"} {
		if strings.Contains(than, cam) {
			t.Errorf("phản hồi danh mục chứa %q: %s", cam, than)
		}
	}
}

func TestLoaiDonViDanCuTraDungNhungTruongCuaHopDong(t *testing.T) {
	// THE FIELDS THAT ARE ABSENT ARE THE DESIGN, asserted rather than assumed. The test one function
	// up forbids a few substrings; this one pins the WHOLE set, which is what catches a field nobody
	// thought to forbid — and a field REMOVED, which no substring check can see.
	//
	//	tenant_id    never leaves this service — not data, but the dimension every row is already
	//	             filtered by (rule 1, invariant 4)
	//	thu_tu       the sort key, not data. `items` already carries the order; the number is only
	//	             of use to a screen that edits it, and exposing it invites a client to re-sort
	//	             and overrule the commune on its own catalogue
	//	nguon,       they answer "what may be DONE to this row" — the three tiers of ADR 0024 §6.
	//	ma_nguon_    There is no write route (open question #21), so publishing the tier would
	//	re_nhanh     describe buttons nobody has decided to allow
	//	deleted_at   a soft-deleted row never leaves the store, so no reader needs to ask
	//
	// THIS IS A CONTRACT, NOT A SNAPSHOT: when it goes red, the question is whether the ROUTE should
	// have changed.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongLoaiDonViDanCu, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	var tho struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &tho); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	// NON-EMPTY FIRST: on an empty list the loop below checks nothing and passes — and empty is what
	// every commune answers today, because migration 0005 seeds nothing.
	if len(tho.Items) == 0 {
		t.Fatal("dữ liệu mẫu rỗng — phép kiểm sẽ xanh mà không kiểm gì")
	}

	// The same five as the four sibling services' catalogues: `label` not `name` (the column is
	// `nhan`), and `active` beside `is_default` — the asymmetry is shared on purpose.
	muon := map[string]bool{"id": true, "code": true, "label": true, "is_default": true, "active": true}
	for _, mot := range tho.Items {
		for khoa := range mot {
			if !muon[khoa] {
				t.Errorf("trường ngoài hợp đồng lọt ra: %q — %s", khoa, w.Body.String())
			}
		}
		// The length check is what catches a REMOVED field.
		if len(mot) != len(muon) {
			t.Errorf("thiếu trường: có %v, muốn %v", mot, muon)
		}
	}
}

// --- (4) whole list, or refused -----------------------------------------------------------------

func TestLoaiDonViDanCuVuotTranThiTuChoiChuKhongCatBot(t *testing.T) {
	// 500 rather than the first N rows. A silently short catalogue is a classification missing from
	// the picker, so the next unit created is classified wrongly — and thon_to_dan_pho then carries
	// that wrong code onward to every screen that groups by it.
	m := dungMayChu(t)
	m.loaiDonViDanCu.loi = idstore.ErrQuaNhieuLoaiDonViDanCu

	w := m.goi(t, "GET", hostA, duongLoaiDonViDanCu, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusInternalServerError)

	if e := loiTra(t, w); e.Code != "internal" {
		t.Errorf("code = %q, muốn internal", e.Code)
	}
	if strings.Contains(w.Body.String(), `"items"`) {
		t.Errorf("phản hồi từ chối vẫn kèm danh sách: %s", w.Body.String())
	}
}

func TestLoaiDonViDanCuLoiKhoTra500VaKhongLoNoiDungLoi(t *testing.T) {
	m := dungMayChu(t)
	m.loaiDonViDanCu.loi = errors.New("cơ sở dữ liệu không phản hồi")

	w := m.goi(t, "GET", hostA, duongLoaiDonViDanCu, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusInternalServerError)

	if e := loiTra(t, w); strings.Contains(e.Message, "cơ sở dữ liệu không phản hồi") {
		t.Errorf("lỗi nội bộ lọt ra ngoài: %q", e.Message)
	}
}

// --- (5) empty commune --------------------------------------------------------------------------

func TestLoaiDonViDanCuXaChuaCoMucNaoTraMangRong(t *testing.T) {
	// [] AND NOT null, and this is the state of EVERY commune today: migration 0005 creates the
	// table and seeds nothing, deliberately, because a seeded row would belong to one named commune
	// and the migration runner has no commune in it (0005:38). An empty list is the correct answer
	// here, not a degraded one.
	m := dungMayChu(t)
	m.loaiDonViDanCu.theo[xaA] = nil

	w := m.goi(t, "GET", hostA, duongLoaiDonViDanCu, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	if !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Errorf("items phải tuần tự hoá thành [], nhận: %s", w.Body.String())
	}
}
