package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// WHAT THIS FILE IS FOR: GET /api/v1/task-blocs is the catalogue whose consumer lives in ANOTHER
// service — the task form in `petitions` holds the chosen code as a value (rule 2, invariant 3).
// The properties are the catalogue ones, plus one that only this route can show: the three
// reference reads are three separate stores, so reading one must not touch another.

const duongKhoiNhiemVu = "/api/v1/task-blocs"

func docKhoiNhiemVu(t *testing.T, than []byte) danhSachKhoiNhiemVuRa {
	t.Helper()
	var ra danhSachKhoiNhiemVuRa
	if err := json.Unmarshal(than, &ra); err != nil {
		t.Fatalf("thân không phải JSON: %q", string(than))
	}
	return ra
}

// --- the four cases of rule 5, invariant 7 ------------------------------------------------------

func TestKhoiNhiemVu_401KhongToken(t *testing.T) {
	m := dungMayChu(t)

	doiMa(t, m.goi(t, "GET", hostA, duongKhoiNhiemVu, "", ""), http.StatusUnauthorized)
	if m.khoiNhiemVu.goi != 0 {
		t.Error("chưa đăng nhập mà đã đọc danh mục khối nhiệm vụ")
	}
}

func TestKhoiNhiemVu_200KhongCanQuyenCauHinh(t *testing.T) {
	// THE "403 WRONG PERMISSION" CASE AS IT READS ON AN AnyAuthenticated ROUTE. The account holds
	// nothing and must still get 200: bloc labels fill the picker on the task form, the row label
	// in a task list and the filter above it, so a configuration permission here would break those
	// screens for every account that is not an administrator.
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{} // no grants at all, in any commune
	})

	w := m.goi(t, "GET", hostA, duongKhoiNhiemVu, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)
	if len(docKhoiNhiemVu(t, w.Body.Bytes()).Items) == 0 {
		t.Error("tài khoản không có quyền nào nhận danh mục rỗng — tuyến này phải trả đủ")
	}
}

func TestKhoiNhiemVu_401XaKhac(t *testing.T) {
	// Commune A's token presented at commune B's domain, refused at the token layer BEFORE any
	// store is touched. There is no permission to be right or wrong about on this route, so this is
	// what the fourth case becomes.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostB, duongKhoiNhiemVu, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusUnauthorized)
	if got := loiTra(t, w).Code; got != "tenant_mismatch" {
		t.Errorf("code = %q, muốn tenant_mismatch", got)
	}
	if m.khoiNhiemVu.goi != 0 {
		t.Error("token của xã khác mà vẫn đọc danh mục của xã này")
	}
}

func TestKhoiNhiemVu_200(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongKhoiNhiemVu, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	ra := docKhoiNhiemVu(t, w.Body.Bytes())
	if len(ra.Items) != 2 {
		t.Fatalf("nhận %d mục, muốn 2", len(ra.Items))
	}
	mot := ra.Items[0]
	// `code` IS THE VALUE ANOTHER SERVICE STORES. A task record in `petitions` holds this string,
	// not the id, and nothing rewrites those rows — which is why the migration's trigger refuses to
	// let a commune edit it, and why it is asserted here rather than left to the id.
	if mot.ID != "knv-001" || mot.Code != "khoi-uy-ban" || mot.Label != "Khối Uỷ ban" {
		t.Errorf("mục đầu sai: %+v", mot)
	}
	// Both flags, differing per row in the fixture — that is what tells "the flag is read" from
	// "the flag is always false".
	if !mot.IsDefault || !mot.Active {
		t.Errorf("mục mặc định đang dùng bị báo sai: %+v", mot)
	}
	if ra.Items[1].IsDefault || ra.Items[1].Active {
		t.Errorf("mục thứ hai phải không mặc định và đã tắt: %+v", ra.Items[1])
	}
	// The order is the store's (`thu_tu`, code breaking ties). A handler that re-sorted would
	// overrule the commune on its own configuration.
	if ra.Items[1].Code != "khoi-dang" {
		t.Errorf("thứ tự đã bị đổi: %q", ra.Items[1].Code)
	}
}

// --- one commune's catalogue never reaches another ----------------------------------------------

func TestKhoiNhiemVuKhongVuotSangXaKhac(t *testing.T) {
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostB, duongKhoiNhiemVu, "", m.tokenCho(t, xaB, sidB))
	doiMa(t, w, http.StatusOK)

	than := w.Body.String()
	if strings.Contains(than, "knv-001") || strings.Contains(than, "Khối Uỷ ban") {
		t.Fatalf("RÒ RỈ: ở xã B nhận được danh mục của xã A: %s", than)
	}
	ra := docKhoiNhiemVu(t, w.Body.Bytes())
	if len(ra.Items) != 1 || ra.Items[0].ID != "knv-b-001" {
		t.Fatalf("xã B phải nhận đúng danh mục của mình, nhận: %+v", ra.Items)
	}
}

// --- the exact field set ------------------------------------------------------------------------

func TestKhoiNhiemVuTraDungNhungTruongCuaHopDong(t *testing.T) {
	// THE EXACT FIELD SET, asserted on the raw JSON — decoding into the response struct cannot see a
	// field added or dropped, because the handler encodes from that same struct.
	//
	// EIGHT FIELDS, THE SIBLINGS' EIGHT (service-petitions loaiNhiemVuRa): `order`, `source` and
	// `tier` joined on 2026-09-24 with the write routes — the admin web detects that a catalogue is
	// writable by their presence. What stays absent: tenant_id (the dimension every row is filtered
	// by), the raw `ma_nguon_re_nhanh` (folded into `tier`), deleted_at (a soft-deleted row never
	// leaves the store).
	//
	// THIS IS A CONTRACT, NOT A SNAPSHOT. When it goes red the question is whether the ROUTE should
	// have changed.
	m := dungMayChu(t)

	w := m.goi(t, "GET", hostA, duongKhoiNhiemVu, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	var tho struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &tho); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	// NON-EMPTY FIRST: on an empty list the loop below runs zero times and the test passes having
	// checked nothing — and an empty list is the answer every commune gets today (migration 0005
	// seeds nothing), so this is the likely state, not a corner case.
	if len(tho.Items) == 0 {
		t.Fatal("dữ liệu mẫu rỗng — phép kiểm sẽ xanh mà không kiểm gì")
	}

	// `label`, NOT `name` — the field is `nhan`. Every ADR 0024 catalogue in this system answers
	// `label`; entities with a `ten` column answer `name`. `active` beside `is_default` is the same
	// pair the four sibling services ship; the asymmetry is deliberate (khoiNhiemVuRa.Active).
	muon := map[string]bool{"id": true, "code": true, "label": true, "is_default": true, "active": true,
		"order": true, "source": true, "tier": true}
	for _, mot := range tho.Items {
		for khoa := range mot {
			if !muon[khoa] {
				t.Errorf("trường ngoài hợp đồng lọt ra: %q — %s", khoa, w.Body.String())
			}
		}
		// The length check is what catches a REMOVED field: the loop above only sees keys that are
		// present.
		if len(mot) != len(muon) {
			t.Errorf("thiếu trường: có %v, muốn %v", mot, muon)
		}
	}
}

// ORDER, SOURCE AND TIER ARE READ, NOT CONSTANTS. The fixture gives the two rows different values of
// all three (tier 2 `he-thong` and tier 1 `don-vi`), so a mapping that dropped any of them shows.
func TestKhoiNhiemVuTraThuTuNguonVaTang(t *testing.T) {
	m := dungMayChu(t)
	w := m.goi(t, "GET", hostA, duongKhoiNhiemVu, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	ra := docKhoiNhiemVu(t, w.Body.Bytes())
	if a := ra.Items[0]; a.Order != 1 || a.Source != "he-thong" || a.Tier != 2 {
		t.Errorf("mục hệ thống: order=%d source=%q tier=%d, muốn 1 / he-thong / 2", a.Order, a.Source, a.Tier)
	}
	if b := ra.Items[1]; b.Order != 2 || b.Source != "don-vi" || b.Tier != 1 {
		t.Errorf("mục của xã: order=%d source=%q tier=%d, muốn 2 / don-vi / 1", b.Order, b.Source, b.Tier)
	}
}

// --- the wire name of the flag ------------------------------------------------------------------

func TestBaTuyenThamChieuTraActiveChuKhongPhaiIsActive(t *testing.T) {
	// THE ASSERTION IS ON THE RAW JSON, AND IT EXISTS BECAUSE NOTHING ELSE IN THIS SUITE CHECKS A
	// WIRE NAME. Every other test here decodes into the SAME struct the handler encodes from, so the
	// two agree no matter what the tag says: renaming `json:"active"` to anything at all left the
	// whole package green. That is a test passing for the wrong reason, and the field it hides is
	// exactly the one that just had to be renamed.
	//
	// `active` AND NOT `is_active`: five sibling catalogue routes answer `active`, and this service
	// already ships it on can_bo.go:51, a route web-admin consumes. `is_default` beside it stays
	// `is_default` — the asymmetry is shared by all of them and must not be tidied on one side.
	//
	// All three routes are checked in one test on purpose: three spellings can only drift apart if
	// something checks them together.
	m := dungMayChu(t)

	for _, duong := range []string{duongKhoiNhiemVu, duongLoaiDonViDanCu, duongThonToDanPho} {
		w := m.goi(t, "GET", hostA, duong, "", m.tokenCho(t, xaA, sidA))
		doiMa(t, w, http.StatusOK)

		than := w.Body.String()
		if !strings.Contains(than, `"active":`) {
			t.Errorf("%s: thiếu trường `active` trên dây: %s", duong, than)
		}
		if strings.Contains(than, `"is_active"`) {
			t.Errorf("%s: vẫn trả `is_active` — một khái niệm hai cách viết trên cùng một hợp đồng: %s",
				duong, than)
		}
	}
}

// --- three reads, three stores ------------------------------------------------------------------

func TestBaTuyenThamChieuDocBaKhoRieng(t *testing.T) {
	// THE THREE NARROW INTERFACES, ASSERTED RATHER THAN TRUSTED. routes.go declares three of them
	// instead of one wide `DanhMucDoc` so that a handler cannot reach a catalogue it has no
	// business reading. This is the cheap check that the wiring actually matches that intent: a
	// single shared reader, or a handler "enriching" its response from a neighbouring catalogue,
	// turns this red immediately — and nothing else in the suite would notice.
	m := dungMayChu(t)

	doiMa(t, m.goi(t, "GET", hostA, duongKhoiNhiemVu, "", m.tokenCho(t, xaA, sidA)), http.StatusOK)
	if m.khoiNhiemVu.goi != 1 {
		t.Errorf("kho khối nhiệm vụ được gọi %d lần, muốn 1", m.khoiNhiemVu.goi)
	}
	if m.loaiDonViDanCu.goi != 0 || m.thonToDanPho.goi != 0 {
		t.Errorf("tuyến khối nhiệm vụ chạm sang kho khác: loai=%d thon=%d",
			m.loaiDonViDanCu.goi, m.thonToDanPho.goi)
	}
}

// --- whole list, or refused ---------------------------------------------------------------------

func TestKhoiNhiemVuVuotTranThiTuChoiChuKhongCatBot(t *testing.T) {
	// 500 rather than the first N rows, and the cost of the alternative is paid in another service:
	// a bloc missing from the task form files the task under the wrong arm of the apparatus, and
	// the count reported upward is then simply false.
	m := dungMayChu(t)
	m.khoiNhiemVu.loi = idstore.ErrQuaNhieuKhoiNhiemVu

	w := m.goi(t, "GET", hostA, duongKhoiNhiemVu, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusInternalServerError)

	if e := loiTra(t, w); e.Code != "internal" {
		t.Errorf("code = %q, muốn internal", e.Code)
	}
	if strings.Contains(w.Body.String(), `"items"`) {
		t.Errorf("phản hồi từ chối vẫn kèm danh sách: %s", w.Body.String())
	}
}

func TestKhoiNhiemVuLoiKhoTra500VaKhongLoNoiDungLoi(t *testing.T) {
	m := dungMayChu(t)
	m.khoiNhiemVu.loi = errors.New("cơ sở dữ liệu không phản hồi")

	w := m.goi(t, "GET", hostA, duongKhoiNhiemVu, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusInternalServerError)

	if e := loiTra(t, w); strings.Contains(e.Message, "cơ sở dữ liệu không phản hồi") {
		t.Errorf("lỗi nội bộ lọt ra ngoài: %q", e.Message)
	}
}

// --- empty commune ------------------------------------------------------------------------------

func TestKhoiNhiemVuXaChuaCoMucNaoTraMangRong(t *testing.T) {
	// [] AND NOT null — the state of every commune today, because migration 0005 seeds nothing on
	// purpose and the onboarding step that would sow the first rows does not exist yet.
	m := dungMayChu(t)
	m.khoiNhiemVu.theo[xaA] = nil

	w := m.goi(t, "GET", hostA, duongKhoiNhiemVu, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	if !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Errorf("items phải tuần tự hoá thành [], nhận: %s", w.Body.String())
	}
}
