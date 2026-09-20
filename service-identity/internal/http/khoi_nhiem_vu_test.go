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
	if !mot.IsDefault || !mot.IsActive {
		t.Errorf("mục mặc định đang dùng bị báo sai: %+v", mot)
	}
	if ra.Items[1].IsDefault || ra.Items[1].IsActive {
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
