package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// Tests for the TASK-01 surface (28/09/2026): GET /api/v1/task-counts, and the new parameters and
// fields of GET /api/v1/tasks, GET /api/v1/tasks/{ma} and GET /api/v1/task-extensions.
//
// RULE 5 INVARIANT 7 FOR EVERY NEW OR CHANGED READ — 401 no session · 403 without `task.read` ·
// right key WRONG COMMUNE · 200. The wrong-commune case answers 401 in this package, not 403: authz
// compares the principal's commune with the Host's BEFORE it reads any key (see the header of
// nhiem_vu_test.go), so the property asserted is the one that matters — NO STORE IS TOUCHED.

const taskCountsPath = "/api/v1/task-counts"

// guardedReads are the new/changed URLs whose four cases are run below, each with the counter that
// proves no read happened.
func guardedReads(m *mayChu) map[string]func() int {
	return map[string]func() int{
		taskCountsPath:                           func() int { return m.nhiemVu.goi },
		taskCountsPath + "?scope=mine&late=true": func() int { return m.nhiemVu.goi },
		"/api/v1/tasks?parent=NV19":              func() int { return m.nhiemVu.goi },
		"/api/v1/tasks?sort=due_at&order=asc":    func() int { return m.nhiemVu.goi },
		duongNhiemVu(maNhiemVuA):                 func() int { return m.nhiemVu.goi },
		duongHangChoLuiHan + "?task=NV19":        func() int { return m.deNghiCho.goi },
	}
}

func TestTask01ReadsNoSessionIs401(t *testing.T) {
	m := dungMayChu(t)
	for path, reads := range guardedReads(m) {
		t.Run(path, func(t *testing.T) {
			before := reads()
			doiMa(t, m.goi(t, http.MethodGet, hostA, path, nil), http.StatusUnauthorized)
			if reads() != before {
				t.Error("đã chạm kho dù chưa có phiên")
			}
		})
	}
}

// 403 WITHOUT `task.read`, even holding every neighbouring task key and `report.read` — so a route
// guarded by any of those instead would pass here and fail.
func TestTask01ReadsWrongPermissionIs403(t *testing.T) {
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
			xaA: {idCanBo: {authz.Perm("task.update"): true, authz.Perm("task.extend"): true,
				authz.Perm("task.create"): true, authz.Perm("report.read"): true}},
		}}
	})
	for path, reads := range guardedReads(m) {
		t.Run(path, func(t *testing.T) {
			before := reads()
			doiMa(t, m.goi(t, http.MethodGet, hostA, path, canBoCuaXa(xaA)), http.StatusForbidden)
			if reads() != before {
				t.Error("đã chạm kho dù thiếu `task.read`")
			}
		})
	}
}

// Right key, WRONG COMMUNE: a session of commune B holding `task.read` in BOTH communes, at commune
// A's host. Refused before any read — the count of one commune never reaches another.
func TestTask01ReadsRightPermissionWrongCommuneIsRefused(t *testing.T) {
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
			xaA: {idCanBo: {authz.Perm("task.read"): true}},
			xaB: {idCanBo: {authz.Perm("task.read"): true}},
		}}
	})
	for path, reads := range guardedReads(m) {
		t.Run(path, func(t *testing.T) {
			before := reads()
			doiMa(t, m.goi(t, http.MethodGet, hostA, path, canBoCuaXa(xaB)), http.StatusUnauthorized)
			if reads() != before {
				t.Error("đã chạm kho của xã A bằng phiên của xã B")
			}
		})
	}
}

func TestTask01ReadsRightPermissionRightCommuneIs200(t *testing.T) {
	m := dungMayChu(t)
	for path := range guardedReads(m) {
		t.Run(path, func(t *testing.T) {
			doiMa(t, m.goi(t, http.MethodGet, hostA, path, canBoCuaXa(xaA)), http.StatusOK)
		})
	}
}

// --- GET /api/v1/task-counts -------------------------------------------------------------------------

func TestTaskCountsAllSevenStatusesZerosIncluded(t *testing.T) {
	m := dungMayChu(t)
	w := m.goi(t, http.MethodGet, hostA, taskCountsPath, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	var out taskCountsOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("thân không phải JSON: %s", w.Body.String())
	}
	defaults := domain.MacDinhTrangThaiNhiemVu()
	if len(out.ByStatus) != len(defaults) {
		t.Fatalf("trả %d trạng thái, muốn đủ %d — mỗi mã là một cột Kanban", len(out.ByStatus), len(defaults))
	}
	for i, d := range defaults {
		if out.ByStatus[i].Status != string(d.Ma) {
			t.Errorf("vị trí %d = %q, muốn %q (thứ tự mặc định của bảy mã)", i, out.ByStatus[i].Status, d.Ma)
		}
	}
	// Commune A's fixtures: one `dang-thuc-hien`, one `hoan-thanh`. Commune B's `moi-giao` must NOT
	// be counted here, and every other status is an explicit 0.
	want := map[string]int{string(domain.DangThucHien): 1, string(domain.HoanThanh): 1}
	for _, c := range out.ByStatus {
		if c.Count != want[c.Status] {
			t.Errorf("%s = %d, muốn %d", c.Status, c.Count, want[c.Status])
		}
	}
	if !strings.Contains(w.Body.String(), `"count":0`) {
		t.Error("trạng thái không có việc nào phải là 0 tường minh, không vắng mặt")
	}
}

// THE SAME FILTERS AS THE LIST, THROUGH THE SAME PARSER — including `scope=mine` resolved from the
// SESSION and never from the URL.
func TestTaskCountsTakesTheListFilters(t *testing.T) {
	m := dungMayChu(t)
	w := m.goi(t, http.MethodGet, hostA,
		taskCountsPath+"?scope=mine&assignee=CB-99999&type=theo-van-ban&q=H%C3%A0+Lam&late=true&parent=NV19"+
			"&limit=5&sort=code", // paging parameters are ignored, not refused
		canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	want := petstore.LocNhiemVu{
		Loai: "theo-van-ban", Tim: "Hà Lam", ChiTreHan: true, ParentCode: "NV19",
		NguoiThucHienMa: maCanBo, // the SESSION's code, not CB-99999
	}
	if m.nhiemVu.locCuoi != want {
		t.Errorf("bộ lọc xuống kho = %+v,\nmuốn %+v", m.nhiemVu.locCuoi, want)
	}
}

func TestTaskCountsRefusesWhatTheListRefuses(t *testing.T) {
	for _, q := range []string{"?status=chua-thuc-hien", "?scope=toan-quoc", "?soon=1", "?late=1",
		"?metric=completed", "?parent=" + strings.Repeat("N", domain.MaNhiemVuToiDa+1)} {
		t.Run(q, func(t *testing.T) {
			m := dungMayChu(t)
			doiMa(t, m.goi(t, http.MethodGet, hostA, taskCountsPath+q, canBoCuaXa(xaA)), http.StatusBadRequest)
			if m.nhiemVu.goi != 0 {
				t.Error("chạy câu đếm dù bộ lọc bị từ chối")
			}
		})
	}
}

func TestTaskCountsStoreFailureIs500WithoutDetail(t *testing.T) {
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.DanhSachNhiemVu = &nhiemVuGia{loi: errors.New("pg: connection refused on host db-07")}
	})
	w := m.goi(t, http.MethodGet, hostA, taskCountsPath, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusInternalServerError)
	if strings.Contains(w.Body.String(), "db-07") {
		t.Errorf("chi tiết hạ tầng lọt ra phản hồi: %s", w.Body.String())
	}
}

// --- GET /api/v1/tasks and /tasks/{ma}: parent, child_count, sort=due_at --------------------------------

func TestTaskResponseCarriesParentCodeAndChildCount(t *testing.T) {
	m := dungMayChu(t)
	w := m.goi(t, http.MethodGet, hostA, "/api/v1/tasks", canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	ra := docTrangNhiemVu(t, w.Body.Bytes())
	byCode := map[string]nhiemVuRa{}
	for _, it := range ra.Items {
		byCode[it.Code] = it
	}
	if byCode[maNhiemVuA].ChildCount != 3 || byCode[maNhiemVuA].Parent != "" {
		t.Errorf("%s: child_count=%d parent=%q, muốn 3 và rỗng", maNhiemVuA, byCode[maNhiemVuA].ChildCount,
			byCode[maNhiemVuA].Parent)
	}
	// THE PARENT IS THE REGISTER NUMBER. The internal id `nv-001` must be in no response field (the
	// fake returns no cursor here, so the raw body check below is exact).
	if byCode[maNhiemVuXo].Parent != maNhiemVuA {
		t.Errorf("%s: parent = %q, muốn mã sổ %q", maNhiemVuXo, byCode[maNhiemVuXo].Parent, maNhiemVuA)
	}
	if strings.Contains(w.Body.String(), "nv-001") {
		t.Error("id nội bộ của nhiệm vụ lọt ra phản hồi")
	}
	// `child_count` is always present, 0 included.
	var raw struct {
		Items []map[string]any `json:"items"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &raw)
	for _, it := range raw.Items {
		if _, ok := it["child_count"]; !ok {
			t.Errorf("dòng %v thiếu child_count", it["code"])
		}
	}

	// And on the DETAIL route.
	w = m.goi(t, http.MethodGet, hostA, duongNhiemVu(maNhiemVuXo), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	if d := docNhiemVu(t, w.Body.Bytes()); d.Parent != maNhiemVuA || d.ChildCount != 0 {
		t.Errorf("chi tiết: parent=%q child_count=%d, muốn %q và 0", d.Parent, d.ChildCount, maNhiemVuA)
	}
}

func TestTaskListParentFilterReachesTheStore(t *testing.T) {
	m := dungMayChu(t)
	doiMa(t, m.goi(t, http.MethodGet, hostA, "/api/v1/tasks?parent=NV19", canBoCuaXa(xaA)), http.StatusOK)
	if m.nhiemVu.locCuoi.ParentCode != "NV19" {
		t.Errorf("parent xuống kho = %q, muốn NV19", m.nhiemVu.locCuoi.ParentCode)
	}

	// Longer than an issued number can be: 400, nothing read, the value not echoed.
	m = dungMayChu(t)
	long := strings.Repeat("X", domain.MaNhiemVuToiDa+1)
	w := m.goi(t, http.MethodGet, hostA, "/api/v1/tasks?parent="+long, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusBadRequest)
	if m.nhiemVu.goi != 0 || strings.Contains(w.Body.String(), long) {
		t.Error("bộ lọc cha quá dài vẫn chạm kho, hoặc bị dội lại")
	}
}

// `sort=due_at` reaches the store as the `due_at` sort in the requested direction, parsed against the
// ONE list tools/apidoc publishes (petstore.SapXepNhiemVu). Which NOT NULL key serves the ascending
// direction is the store's switch (petstore.DanhSach) and is tested there. Default direction is the
// allowlist's (desc).
func TestTaskListDueSortReachesTheStoreWithItsDirection(t *testing.T) {
	for q, want := range map[string]page.Dir{
		"?sort=due_at&order=asc":  page.Asc,
		"?sort=due_at&order=desc": page.Desc,
		"?sort=due_at":            page.Desc,
	} {
		t.Run(q, func(t *testing.T) {
			m := dungMayChu(t)
			doiMa(t, m.goi(t, http.MethodGet, hostA, "/api/v1/tasks"+q, canBoCuaXa(xaA)), http.StatusOK)
			if got := m.nhiemVu.lastPage; got.Column().Param != "due_at" || got.Dir() != want {
				t.Errorf("kho nhận %s %s, muốn due_at %s", got.Column().Param, got.Dir(), want)
			}
		})
	}
}

// Still refused: the raw nullable column, the title (PII in a URL), the priority code, and a bad order.
func TestTaskListSortsStillRefused(t *testing.T) {
	for _, q := range []string{"?sort=han_xu_ly", "?sort=title", "?sort=priority", "?sort=due_at&order=ngang"} {
		t.Run(q, func(t *testing.T) {
			m := dungMayChu(t)
			doiMa(t, m.goi(t, http.MethodGet, hostA, "/api/v1/tasks"+q, canBoCuaXa(xaA)), http.StatusBadRequest)
			if m.nhiemVu.goi != 0 {
				t.Error("chạy truy vấn dù tiêu chí sắp xếp bị từ chối")
			}
		})
	}
}

// --- GET /api/v1/task-extensions?task= ------------------------------------------------------------------

func TestQueueTaskFilterReachesTheStore(t *testing.T) {
	m := dungMayChu(t)
	doiMa(t, m.goi(t, http.MethodGet, hostA, duongHangChoLuiHan+"?task=NV19&approver=me", canBoCuaXa(xaA)),
		http.StatusOK)
	if m.deNghiCho.loc.TaskCode != "NV19" || m.deNghiCho.loc.LanhDaoGiaoViecMa != maCanBo {
		t.Errorf("bộ lọc xuống kho = %+v, muốn task NV19 + lãnh đạo là chính người gọi", m.deNghiCho.loc)
	}

	m = dungMayChu(t)
	doiMa(t, m.goi(t, http.MethodGet, hostA, duongHangChoLuiHan, canBoCuaXa(xaA)), http.StatusOK)
	if m.deNghiCho.loc.TaskCode != "" {
		t.Errorf("không có `task` mà vẫn lọc theo %q", m.deNghiCho.loc.TaskCode)
	}

	m = dungMayChu(t)
	long := strings.Repeat("X", domain.MaNhiemVuToiDa+1)
	w := m.goi(t, http.MethodGet, hostA, duongHangChoLuiHan+"?task="+long, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusBadRequest)
	if m.deNghiCho.goi != 0 || strings.Contains(w.Body.String(), long) {
		t.Error("`task` quá dài vẫn chạm kho, hoặc bị dội lại")
	}
}
