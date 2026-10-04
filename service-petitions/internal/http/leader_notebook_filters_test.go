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
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// Tests for the Sổ tay lãnh đạo's server-side filters (ADR 0071): `scope=assigned-by-me` and
// `incomplete=true` on GET /api/v1/tasks · /task-counts, and the new GET /api/v1/task-extension-counts.
//
//	PROVED HERE   rule 5 invariant 7 for the new route and the changed reads — 401 no session · 403
//	              without `task.read` · right key WRONG COMMUNE refused (401 in this package, see
//	              task_counts_test.go) · 200 — with no store touched in the first three · "me" is the
//	              SESSION's code, and no query parameter (`assigner`, `created_by`, `assigned_by`)
//	              can name anybody · a principal with no code is 500 before any read · malformed values
//	              are 400 before any read · the list and the count receive the SAME filter · the badge
//	              equals the queue length under the same filter, in the request's commune.
//
//	NOT PROVED    the SQL — store/leader_notebook_filters_test.go (fake driver) and
//	              store/leader_notebook_filters_pg_test.go (PostgreSQL; SKIPS without VIGOV_TEST_DSN).

const taskExtensionCountsPath = "/api/v1/task-extension-counts"

func leaderNotebookReads(m *mayChu) map[string]func() int {
	return map[string]func() int{
		"/api/v1/tasks?scope=assigned-by-me&incomplete=true":     func() int { return m.nhiemVu.goi },
		taskCountsPath + "?scope=assigned-by-me&incomplete=true": func() int { return m.nhiemVu.goi },
		taskExtensionCountsPath:                                  func() int { return m.deNghiCho.goi },
		taskExtensionCountsPath + "?approver=me":                 func() int { return m.deNghiCho.goi },
	}
}

// --- rule 5, invariant 7 ------------------------------------------------------------------------

func TestLeaderNotebookReadsNoSessionIs401(t *testing.T) {
	m := dungMayChu(t)
	for path, reads := range leaderNotebookReads(m) {
		t.Run(path, func(t *testing.T) {
			before := reads()
			doiMa(t, m.goi(t, http.MethodGet, hostA, path, nil), http.StatusUnauthorized)
			if reads() != before {
				t.Error("đã chạm kho dù chưa có phiên")
			}
		})
	}
}

// 403 without `task.read`, holding the extension, approval and report keys — so a route guarded by
// any of those instead would pass here and fail.
func TestLeaderNotebookReadsWrongPermissionIs403(t *testing.T) {
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
			xaA: {idCanBo: {authz.Perm("task.extend"): true, authz.Perm("task.approve"): true,
				authz.Perm("task.update"): true, authz.Perm("report.read"): true}},
		}}
	})
	for path, reads := range leaderNotebookReads(m) {
		t.Run(path, func(t *testing.T) {
			before := reads()
			doiMa(t, m.goi(t, http.MethodGet, hostA, path, canBoCuaXa(xaA)), http.StatusForbidden)
			if reads() != before {
				t.Error("đã chạm kho dù thiếu `task.read`")
			}
		})
	}
}

func TestLeaderNotebookReadsRightPermissionWrongCommuneIsRefused(t *testing.T) {
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
			xaA: {idCanBo: {authz.Perm("task.read"): true}},
			xaB: {idCanBo: {authz.Perm("task.read"): true}},
		}}
	})
	for path, reads := range leaderNotebookReads(m) {
		t.Run(path, func(t *testing.T) {
			before := reads()
			doiMa(t, m.goi(t, http.MethodGet, hostA, path, canBoCuaXa(xaB)), http.StatusUnauthorized)
			if reads() != before {
				t.Error("đã chạm kho của xã A bằng phiên của xã B")
			}
		})
	}
}

func TestLeaderNotebookReadsRightPermissionRightCommuneIs200(t *testing.T) {
	m := dungMayChu(t)
	for path := range leaderNotebookReads(m) {
		t.Run(path, func(t *testing.T) {
			doiMa(t, m.goi(t, http.MethodGet, hostA, path, canBoCuaXa(xaA)), http.StatusOK)
		})
	}
}

// --- scope=assigned-by-me · incomplete=true --------------------------------------------------------

// "Me" is the SESSION's code. Every parameter a client might try to name somebody with is ignored —
// none of them is read into the field — and the result is the same filter as without them.
func TestAssignedByMeTakesTheSessionCodeOnly(t *testing.T) {
	m := dungMayChu(t)
	doiMa(t, m.goi(t, http.MethodGet, hostA,
		"/api/v1/tasks?scope=assigned-by-me&incomplete=true"+
			"&assigner=CB-99999&created_by=CB-99999&assigned_by=CB-99999&creator=CB-99999",
		canBoCuaXa(xaA)), http.StatusOK)

	want := petstore.LocNhiemVu{AssignedByStaffCode: maCanBo, Incomplete: true}
	if m.nhiemVu.locCuoi != want {
		t.Errorf("bộ lọc xuống kho = %+v,\nmuốn %+v — mã của CHÍNH phiên, không gì khác", m.nhiemVu.locCuoi, want)
	}
}

// The badge over "Việc tôi đã giao" is counted under the filter its rows are read with.
func TestAssignedByMeListAndCountsReceiveTheSameFilter(t *testing.T) {
	const q = "?scope=assigned-by-me&incomplete=true&type=theo-van-ban"
	m := dungMayChu(t)
	doiMa(t, m.goi(t, http.MethodGet, hostA, "/api/v1/tasks"+q, canBoCuaXa(xaA)), http.StatusOK)
	list := m.nhiemVu.locCuoi
	doiMa(t, m.goi(t, http.MethodGet, hostA, taskCountsPath+q, canBoCuaXa(xaA)), http.StatusOK)
	if m.nhiemVu.locCuoi != list {
		t.Errorf("đếm = %+v,\ndanh sách = %+v — hai bộ lọc phải là một", m.nhiemVu.locCuoi, list)
	}
	if list.AssignedByStaffCode != maCanBo || !list.Incomplete || list.Loai != "theo-van-ban" {
		t.Errorf("bộ lọc = %+v", list)
	}
}

func TestAssignedByMeWithoutStaffCodeIs500BeforeAnyRead(t *testing.T) {
	for _, path := range []string{"/api/v1/tasks?scope=assigned-by-me", taskCountsPath + "?scope=assigned-by-me"} {
		m := dungMayChu(t)
		p := canBoCuaXa(xaA)
		p.Ma = ""
		doiMa(t, m.goi(t, http.MethodGet, hostA, path, p), http.StatusInternalServerError)
		if m.nhiemVu.goi != 0 {
			t.Errorf("%s: đã đọc kho với chủ thể không mã — cột sẽ hiện việc của cả xã", path)
		}
	}
}

func TestLeaderNotebookTaskFiltersRefuseMalformedValues(t *testing.T) {
	for _, q := range []string{"?incomplete=1", "?incomplete=false", "?incomplete=TRUE",
		"?scope=assigned", "?scope=assigned-by-CB-00123", "?scope=Assigned-By-Me"} {
		for _, base := range []string{"/api/v1/tasks", taskCountsPath} {
			t.Run(base+q, func(t *testing.T) {
				m := dungMayChu(t)
				w := m.goi(t, http.MethodGet, hostA, base+q, canBoCuaXa(xaA))
				doiMa(t, w, http.StatusBadRequest)
				if m.nhiemVu.goi != 0 {
					t.Error("đã đọc kho dù bộ lọc bị từ chối")
				}
				if strings.Contains(w.Body.String(), "CB-00123") {
					t.Error("thân lỗi dội lại giá trị client gửi")
				}
			})
		}
	}
}

// --- GET /api/v1/task-extension-counts -------------------------------------------------------------

func TestTaskExtensionCountEqualsQueueLength(t *testing.T) {
	for _, q := range []string{"", "?approver=me"} {
		t.Run(q, func(t *testing.T) {
			m := dungMayChu(t)

			wq := m.goi(t, http.MethodGet, hostA, duongHangChoLuiHan+q, canBoCuaXa(xaA))
			doiMa(t, wq, http.StatusOK)
			var queue page.Result[deNghiChoDuyetRa]
			if err := json.Unmarshal(wq.Body.Bytes(), &queue); err != nil {
				t.Fatalf("hàng chờ không phải JSON: %s", wq.Body.String())
			}
			queueLoc := m.deNghiCho.loc

			wc := m.goi(t, http.MethodGet, hostA, taskExtensionCountsPath+q, canBoCuaXa(xaA))
			doiMa(t, wc, http.StatusOK)
			var out taskExtensionCountOut
			if err := json.Unmarshal(wc.Body.Bytes(), &out); err != nil {
				t.Fatalf("thân không phải JSON: %s", wc.Body.String())
			}
			if out.Count != len(queue.Items) {
				t.Errorf("count = %d, hàng chờ có %d dòng", out.Count, len(queue.Items))
			}
			if m.deNghiCho.loc != queueLoc || m.deNghiCho.xa != xaA {
				t.Errorf("bộ lọc đếm = %+v ở xã %q, muốn %+v ở xã A", m.deNghiCho.loc, m.deNghiCho.xa, queueLoc)
			}
			// Commune B also holds a request whose leader is maCanBo: it must not be counted.
			want := 2
			if q == "?approver=me" {
				want = 1
			}
			if out.Count != want {
				t.Errorf("count = %d, muốn %d", out.Count, want)
			}
			// EXACTLY one field — no rows, no reason text.
			var raw map[string]any
			_ = json.Unmarshal(wc.Body.Bytes(), &raw)
			if len(raw) != 1 {
				t.Errorf("phản hồi = %v, muốn đúng một trường `count`", raw)
			}
		})
	}
}

func TestTaskExtensionCountApproverIsSessionOnly(t *testing.T) {
	m := dungMayChu(t)
	doiMa(t, m.goi(t, http.MethodGet, hostA, taskExtensionCountsPath+"?approver=me", canBoCuaXa(xaA)), http.StatusOK)
	if m.deNghiCho.loc.LanhDaoGiaoViecMa != maCanBo {
		t.Errorf("lọc theo lãnh đạo = %q, muốn mã của CHÍNH phiên", m.deNghiCho.loc.LanhDaoGiaoViecMa)
	}
	for _, q := range []string{"?approver=CB-00999", "?approver=" + maCanBo, "?approver=ME", "?approver=all"} {
		t.Run(q, func(t *testing.T) {
			m := dungMayChu(t)
			w := m.goi(t, http.MethodGet, hostA, taskExtensionCountsPath+q, canBoCuaXa(xaA))
			doiMa(t, w, http.StatusBadRequest)
			if m.deNghiCho.goi != 0 || strings.Contains(w.Body.String(), "CB-00999") {
				t.Error("đã đếm, hoặc thân lỗi dội lại giá trị client gửi")
			}
		})
	}
}

func TestTaskExtensionCountWithoutStaffCodeIs500BeforeAnyRead(t *testing.T) {
	m := dungMayChu(t)
	p := &authz.Principal{ID: idCanBo, Kind: "staff", TenantID: xaA}
	doiMa(t, m.goi(t, http.MethodGet, hostA, taskExtensionCountsPath+"?approver=me", p), http.StatusInternalServerError)
	if m.deNghiCho.goi != 0 {
		t.Errorf("đã đếm %d lần — lẽ ra từ chối TRƯỚC khi truy vấn", m.deNghiCho.goi)
	}
}

func TestTaskExtensionCountStoreFailureIs500WithoutDetail(t *testing.T) {
	m := dungMayChu(t)
	m.deNghiCho.loi = errors.New("pg: connection refused on host db-07")
	w := m.goi(t, http.MethodGet, hostA, taskExtensionCountsPath, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusInternalServerError)
	if strings.Contains(w.Body.String(), "db-07") {
		t.Errorf("chi tiết hạ tầng lọt ra phản hồi: %s", w.Body.String())
	}
}
