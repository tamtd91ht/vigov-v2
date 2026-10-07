package http

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/tenant"
)

// Tests for the menu nhiem-vu additions of 07/10/2026: `extension_count` / `pending_extension` on every
// task reply, the `roots=true` filter, and `requires_directive` on the task types.
//
//	PROVED HERE   rule 5 invariant 7 for `roots=true` on the list and the counts (401 · 403 · right key
//	              wrong commune refused — 401 in this package, see task_counts_test.go · 200), with no
//	              read on a refusal · both extension fields are ALWAYS on the wire, `0` / `false` when
//	              the store found none, on the list and the detail · `roots=true` reaches the list, the
//	              counts and the register export as ONE filter · any other spelling is 400 before any
//	              read · `requires_directive` is true for `theo-van-ban` only.
//
//	NOT PROVED    the SQL — store/task_extension_facts_test.go, store/task_list_test.go (fake driver)
//	              and store/task_extension_facts_pg_test.go (PostgreSQL; SKIPS without VIGOV_TEST_DSN).

func rootsReads(m *mayChu) map[string]func() int {
	return map[string]func() int{
		"/api/v1/tasks?roots=true":     func() int { return m.nhiemVu.goi },
		taskCountsPath + "?roots=true": func() int { return m.nhiemVu.goi },
	}
}

// --- rule 5, invariant 7 ------------------------------------------------------------------------

func TestRootsFilterNoSessionIs401(t *testing.T) {
	m := dungMayChu(t)
	for path, reads := range rootsReads(m) {
		t.Run(path, func(t *testing.T) {
			before := reads()
			doiMa(t, m.goi(t, http.MethodGet, hostA, path, nil), http.StatusUnauthorized)
			if reads() != before {
				t.Error("đã chạm kho dù chưa có phiên")
			}
		})
	}
}

// 403 without `task.read`, holding the neighbouring task keys — so a route guarded by any of those
// instead would pass here and fail.
func TestRootsFilterWrongPermissionIs403(t *testing.T) {
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
			xaA: {idCanBo: {authz.Perm("task.extend"): true, authz.Perm("task.update"): true,
				authz.Perm("task.create"): true}},
		}}
	})
	for path, reads := range rootsReads(m) {
		t.Run(path, func(t *testing.T) {
			before := reads()
			doiMa(t, m.goi(t, http.MethodGet, hostA, path, canBoCuaXa(xaA)), http.StatusForbidden)
			if reads() != before {
				t.Error("đã chạm kho dù thiếu `task.read`")
			}
		})
	}
}

func TestRootsFilterRightPermissionWrongCommuneIsRefused(t *testing.T) {
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
			xaA: {idCanBo: {authz.Perm("task.read"): true}},
			xaB: {idCanBo: {authz.Perm("task.read"): true}},
		}}
	})
	for path, reads := range rootsReads(m) {
		t.Run(path, func(t *testing.T) {
			before := reads()
			doiMa(t, m.goi(t, http.MethodGet, hostA, path, canBoCuaXa(xaB)), http.StatusUnauthorized)
			if reads() != before {
				t.Error("đã chạm kho của xã A bằng phiên của xã B")
			}
		})
	}
}

func TestRootsFilterRightPermissionRightCommuneIs200(t *testing.T) {
	m := dungMayChu(t)
	for path := range rootsReads(m) {
		t.Run(path, func(t *testing.T) {
			doiMa(t, m.goi(t, http.MethodGet, hostA, path, canBoCuaXa(xaA)), http.StatusOK)
		})
	}
}

// --- roots=true ----------------------------------------------------------------------------------

// ONE FILTER for the list, the Kanban counts and the export file: the same query string reaches all
// three as the same LocNhiemVu, `Roots` set and nothing else changed.
func TestRootsFilterIsSharedByListCountsAndExport(t *testing.T) {
	const q = "?roots=true&type=theo-van-ban"
	m := dungMayChu(t)
	doiMa(t, m.goi(t, http.MethodGet, hostA, "/api/v1/tasks"+q, canBoCuaXa(xaA)), http.StatusOK)
	list := m.nhiemVu.locCuoi
	if !list.Roots || list.Loai != "theo-van-ban" {
		t.Errorf("bộ lọc danh sách = %+v, muốn Roots và loại", list)
	}
	doiMa(t, m.goi(t, http.MethodGet, hostA, taskCountsPath+q, canBoCuaXa(xaA)), http.StatusOK)
	if m.nhiemVu.locCuoi != list {
		t.Errorf("đếm = %+v,\ndanh sách = %+v — hai bộ lọc phải là một", m.nhiemVu.locCuoi, list)
	}
	doiMa(t, m.goi(t, http.MethodGet, hostA, registerExportPath+q, canBoCuaXa(xaA)), http.StatusOK)
	if m.registerExport.req.Filter != list {
		t.Errorf("xuất sổ = %+v,\ndanh sách = %+v — tệp phải đúng bộ lọc của danh sách", m.registerExport.req.Filter, list)
	}

	// Absent = every task: the default is unchanged.
	doiMa(t, m.goi(t, http.MethodGet, hostA, "/api/v1/tasks", canBoCuaXa(xaA)), http.StatusOK)
	if m.nhiemVu.locCuoi.Roots {
		t.Error("không gửi `roots` mà bộ lọc vẫn chỉ lấy việc gốc")
	}
}

func TestRootsFilterRefusesMalformedValues(t *testing.T) {
	for _, q := range []string{"?roots=1", "?roots=false", "?roots=TRUE", "?roots=yes"} {
		for _, base := range []string{"/api/v1/tasks", taskCountsPath} {
			t.Run(base+q, func(t *testing.T) {
				m := dungMayChu(t)
				w := m.goi(t, http.MethodGet, hostA, base+q, canBoCuaXa(xaA))
				doiMa(t, w, http.StatusBadRequest)
				if m.nhiemVu.goi != 0 {
					t.Error("đã đọc kho dù bộ lọc bị từ chối")
				}
			})
		}
	}
}

// --- extension_count · pending_extension ----------------------------------------------------------

// rawTaskKeys decodes one task object keeping every key, so "absent" and "false" are told apart.
func rawTaskKeys(t *testing.T, b []byte) map[string]json.RawMessage {
	t.Helper()
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("thân không phải JSON: %s", b)
	}
	return raw
}

func TestTaskWireCarriesExtensionFactsOnListAndDetail(t *testing.T) {
	m := dungMayChu(t)
	// What the store attaches (store.attachExtensionFacts): NV19 extended twice with a request pending;
	// the other task has none, which the store leaves at 0 / false.
	m.nhiemVu.theo[xaA][0].ExtensionCount = 2
	m.nhiemVu.theo[xaA][0].PendingExtension = true

	w := m.goi(t, http.MethodGet, hostA, "/api/v1/tasks", canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	var page struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil || len(page.Items) != 2 {
		t.Fatalf("trang = %s", w.Body.String())
	}
	want := map[string][2]string{
		`"` + maNhiemVuA + `"`:  {"2", "true"},
		`"` + maNhiemVuXo + `"`: {"0", "false"},
	}
	for _, item := range page.Items {
		exp, ok := want[string(item["code"])]
		if !ok {
			t.Fatalf("mã lạ: %s", item["code"])
		}
		// PRESENT AS 0 / false, never omitted: a client reads `pending_extension` to decide whether to
		// offer a request, and an absent field is a question it should not have to ask.
		if string(item["extension_count"]) != exp[0] || string(item["pending_extension"]) != exp[1] {
			t.Errorf("%s: extension_count=%s pending_extension=%s, muốn %s %s", item["code"],
				item["extension_count"], item["pending_extension"], exp[0], exp[1])
		}
	}

	w = m.goi(t, http.MethodGet, hostA, duongNhiemVu(maNhiemVuA), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	raw := rawTaskKeys(t, w.Body.Bytes())
	if string(raw["extension_count"]) != "2" || string(raw["pending_extension"]) != "true" {
		t.Errorf("chi tiết: extension_count=%s pending_extension=%s, muốn 2 true", raw["extension_count"], raw["pending_extension"])
	}

	w = m.goi(t, http.MethodGet, hostA, duongNhiemVu(maNhiemVuXo), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	raw = rawTaskKeys(t, w.Body.Bytes())
	if string(raw["extension_count"]) != "0" || string(raw["pending_extension"]) != "false" {
		t.Errorf("chi tiết không lùi hạn: extension_count=%s pending_extension=%s, muốn 0 false",
			raw["extension_count"], raw["pending_extension"])
	}
}

// --- requires_directive ---------------------------------------------------------------------------

func TestTaskTypesCarryRequiresDirective(t *testing.T) {
	m := dungMayChu(t)
	w := m.goi(t, http.MethodGet, hostA, duongLoaiNhiemVu, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	var out danhSachLoaiNhiemVuRa
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || len(out.Items) == 0 {
		t.Fatalf("thân = %s", w.Body.String())
	}
	sawDirective := false
	for _, it := range out.Items {
		if it.RequiresDirective != (it.Code == "theo-van-ban") {
			t.Errorf("%s: requires_directive=%v", it.Code, it.RequiresDirective)
		}
		sawDirective = sawDirective || it.RequiresDirective
	}
	if !sawDirective {
		t.Error("fixture không có `theo-van-ban` — test này không khẳng định gì")
	}
}
