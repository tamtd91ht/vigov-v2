package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// WHAT THIS FILE IS FOR: GET /api/v1/task-priorities carries everything the task-type route carries
// — the four cases of rule 5 invariant 7, the commune boundary, whole-list-or-refuse, [] not null —
// plus ONE property that is this route's alone and is the reason the entity has no `…Type` suffix:
//
//	THE ORDER OF THE LIST IS THE MEANING OF THE DATA.
//
// "Khẩn" is urgent only relative to the levels around it. A scale rendered in the wrong order is
// wrong in a way nobody reports as a bug: every screen still shows three plausible Vietnamese
// words, work is prioritised wrongly for months, and the figure that reaches leadership is simply
// false. TestMucUuTienDungThuTuThang is the test that makes that impossible to introduce quietly.

const duongMucUuTien = "/api/v1/task-priorities"

func docMucUuTien(t *testing.T, than []byte) danhSachMucUuTienRa {
	t.Helper()
	var ra danhSachMucUuTienRa
	if err := json.Unmarshal(than, &ra); err != nil {
		t.Fatalf("thân không phải JSON: %q", string(than))
	}
	return ra
}

// --- the property that belongs to this route alone ----------------------------------------------

func TestMucUuTienDungThuTuThang(t *testing.T) {
	// THE ORDER IS PINNED EXPLICITLY, position by position, and NOT as a set.
	//
	// The fixture is in rank order — khan, cao, thuong — which is deliberately neither alphabetical
	// by code (cao, khan, thuong) nor by label (Cao, Khẩn, Thường) nor by id. So a handler that
	// sorted by ANY field, or that ranged over a map on the way out, turns this red. Compare sets
	// here and the test would agree with every one of those.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongMucUuTien, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	ra := docMucUuTien(t, w.Body.Bytes())
	muon := []string{"khan", "cao", "thuong"}
	if len(ra.Items) != len(muon) {
		t.Fatalf("nhận %d mức ưu tiên, muốn %d", len(ra.Items), len(muon))
	}
	for i, ma := range muon {
		if ra.Items[i].Code != ma {
			t.Fatalf("thang ưu tiên sai thứ tự ở vị trí %d: %q, muốn %q — danh sách đủ mục nhưng sai hạng",
				i, ra.Items[i].Code, ma)
		}
	}
}

// --- the four cases -----------------------------------------------------------------------------

func TestMucUuTien_401KhongPhien(t *testing.T) {
	m := dungMayChu(t)

	doiMa(t, m.goi(t, "GET", hostA, duongMucUuTien, nil), http.StatusUnauthorized)
	if m.uuTien.goi != 0 {
		t.Error("chưa đăng nhập mà đã đọc thang ưu tiên")
	}
}

func TestMucUuTien_200KhongCanQuyenCauHinh(t *testing.T) {
	// The "403 wrong permission" case as it reads on an AnyAuthenticated route — the account holds
	// nothing at all and must still get 200. The full argument is on the task-type route's twin of
	// this test; what it protects here is the same trade-off, declared on the route.
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{} // no grants at all, in any commune
	})

	w := m.goi(t, "GET", hostA, duongMucUuTien, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	if len(docMucUuTien(t, w.Body.Bytes()).Items) == 0 {
		t.Error("tài khoản không có quyền nào nhận thang rỗng — tuyến này phải trả đủ")
	}
}

func TestMucUuTien_401XaKhac(t *testing.T) {
	// A principal issued by commune A arriving at commune B's domain, refused BEFORE any store is
	// touched. 401 and not 403 is the contract — there is no permission to fail, and the @reply
	// lines on the route declare exactly these statuses.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostB, duongMucUuTien, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusUnauthorized)
	if got := loiTra(t, w).Code; got != "unauthorized" {
		t.Errorf("code = %q, muốn unauthorized", got)
	}
	if m.uuTien.goi != 0 {
		t.Error("phiên của xã khác mà vẫn đọc thang ưu tiên của xã này")
	}
}

func TestMucUuTien_200(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongMucUuTien, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	ra := docMucUuTien(t, w.Body.Bytes())
	if ra.Items[0].ID != "uu-001" || ra.Items[0].Code != "khan" || ra.Items[0].Label != "Khẩn" {
		t.Errorf("mức đầu thang sai: %+v", ra.Items[0])
	}
	// EXACTLY ONE DEFAULT, and it is NOT the first item — the level a new task starts at is
	// ordinary, not urgent. A fixture where the default were also the first row could not tell "the
	// flag is read" from "the first row is assumed to be the default".
	var soMacDinh int
	for _, mot := range ra.Items {
		if mot.IsDefault {
			soMacDinh++
		}
	}
	if soMacDinh != 1 {
		t.Errorf("có %d mức mặc định, muốn đúng 1", soMacDinh)
	}
	if !ra.Items[2].IsDefault {
		t.Errorf("mức mặc định sai: %+v", ra.Items)
	}
}

// --- one commune's scale never reaches another --------------------------------------------------

func TestMucUuTienKhongVuotSangXaKhac(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostB, duongMucUuTien, canBoCuaXa(xaB))
	doiMa(t, w, http.StatusOK)

	than := w.Body.String()
	if strings.Contains(than, "khan") || strings.Contains(than, "uu-001") {
		t.Fatalf("RÒ RỈ: ở xã B nhận được thang ưu tiên của xã A: %s", than)
	}
	ra := docMucUuTien(t, w.Body.Bytes())
	if len(ra.Items) != 1 || ra.Items[0].ID != "uu-b-001" {
		t.Fatalf("xã B phải nhận đúng thang của mình, nhận: %+v", ra.Items)
	}
}

// --- whole list, or refused ---------------------------------------------------------------------

func TestMucUuTienVuotTranThiTuChoiChuKhongCatBot(t *testing.T) {
	// Worse here than on the type catalogue, and that is why it is asserted separately: the levels
	// arrive in rank order, so a truncated scale does not merely lose an option — it loses the
	// options at ONE END. The picker then offers a scale that silently stops short.
	m := dungMayChu(t)
	m.uuTien.loi = petstore.ErrQuaNhieuMucUuTien

	w := m.goi(t, "GET", hostA, duongMucUuTien, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusInternalServerError)

	if e := loiTra(t, w); e.Code != "internal" {
		t.Errorf("code = %q, muốn internal", e.Code)
	}
	if strings.Contains(w.Body.String(), `"items"`) {
		t.Errorf("phản hồi từ chối vẫn kèm danh sách: %s", w.Body.String())
	}
}

func TestMucUuTienLoiKhoTra500VaKhongLoNoiDungLoi(t *testing.T) {
	m := dungMayChu(t)
	m.uuTien.loi = errors.New("cơ sở dữ liệu không phản hồi")

	w := m.goi(t, "GET", hostA, duongMucUuTien, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusInternalServerError)

	if e := loiTra(t, w); strings.Contains(e.Message, "cơ sở dữ liệu không phản hồi") {
		t.Errorf("lỗi nội bộ lọt ra ngoài: %q", e.Message)
	}
}

// --- a commune with no rows ---------------------------------------------------------------------

func TestMucUuTienXaChuaCoDongNaoTraMangRong(t *testing.T) {
	// Every commune, today: migration 0003 seeds nothing. [] and never null.
	m := dungMayChu(t)
	m.uuTien.theo[xaA] = nil

	w := m.goi(t, "GET", hostA, duongMucUuTien, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	if !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Errorf("items phải tuần tự hoá thành [], nhận: %s", w.Body.String())
	}
}

// --- the contract shape -------------------------------------------------------------------------

func TestMucUuTienChiTraTruongCuaHopDong(t *testing.T) {
	// The same fields as every other ADR 0024 catalogue in this system, and the same absences — the
	// list of what is left out, and why, is on the twin of this test in loai_nhiem_vu_test.go.
	//
	// `thu_tu` USED TO BE ABSENT AND THE NOTE HERE ARGUED IT WAS THE HEAVIEST ABSENCE OF ALL: on a
	// type catalogue the sort key is a display preference, while on a SCALE it IS the rank, so
	// publishing it hands a client the numbers to re-sort by — and a priority list rendered in an
	// order that means nothing still shows plausible words on every screen.
	//
	// THE ARGUMENT WAS RIGHT AND ITS CONCLUSION IS NOW WRONG, which is worth writing out rather
	// than deleting. `order` is published because the commune EDITS it: the configuration screen
	// shows a `Thứ tự` column (docs/ui-ux/14-cau-hinh.md:158) and a screen that cannot read the
	// current rank cannot offer to change it. What the argument really established is an
	// instruction to CLIENTS — do not re-sort, the list already arrives in rank order — and that
	// instruction now lives on the field itself, where a client reading the contract will see it.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongMucUuTien, canBoCuaXa(xaA))
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
