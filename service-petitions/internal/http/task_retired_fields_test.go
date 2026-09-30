package http

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// The body keys the task register stopped accepting on 30/09/2026, and the response fields it stopped
// sending.
//
//	ADR 0065 NV3   PATCH /api/v1/tasks/{ma} refuses `code` — an issued task code is never edited.
//	ADR 0065 NV5   `lead_unit` / `monitor` are refused on the four doors that create or hand over a
//	               task, and are absent from every task response.
//
//	PROVED HERE   each refusal is 400 `invalid_request` with the decision's sentence, whatever the value
//	              (a string, "", null) and whatever the key's letter case · the use case is never
//	              reached · the task response carries neither retired key.
//	NOT PROVED    that the web stopped sending them — out of this service.

// withKey is the JSON of `body` with one extra key set to the raw JSON `value`.
func withKey(t *testing.T, body any, key, value string) string {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	m[key] = json.RawMessage(value)
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestPatchTaskCodeIs400(t *testing.T) {
	for _, c := range []struct{ key, value string }{
		{"code", `"NV45"`},
		{"code", `""`},
		{"code", `null`},
		{"Code", `"NV45"`}, // encoding/json matched this onto the old field, so it is refused too
	} {
		t.Run(c.key+"="+c.value, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, authz.Perm("task.update"))
			g := "ghi chú"
			w := m.goiGhiNVTho(t, http.MethodPatch, hostA, duongNV(maNVThu), canBoCuaXa(xaA),
				withKey(t, suaNhiemVuVao{Note: &g}, c.key, c.value))

			doiMa(t, w, http.StatusBadRequest)
			if e := loiTra(t, w); e.Code != "invalid_request" || !strings.Contains(e.Message, "Mã nhiệm vụ đã cấp không sửa được") {
				t.Errorf("lỗi = %+v", e)
			}
			if m.ghiNhiemVu.goi != 0 {
				t.Error("use case chạy dù thân còn gửi `code` — mã nhiệm vụ đã cấp là bất biến (ADR 0065 NV3)")
			}
		})
	}
}

// retiredRoleRoute is one door that creates or hands over a task, with a body it would accept.
type retiredRoleRoute struct {
	path  string
	perms []authz.Perm
	body  any
	calls func(*mayChu) int
}

func retiredRoleRoutes() map[string]retiredRoleRoute {
	return map[string]retiredRoleRoute{
		"POST /api/v1/tasks": {duongTasks, []authz.Perm{"task.create"}, thanTaoNV(),
			func(m *mayChu) int { return m.ghiNhiemVu.goi }},
		"POST /api/v1/tasks/{ma}/assignment": {taskAssignmentPath(maNVThu), []authz.Perm{"task.assign"},
			taskAssignmentIn{Unit: strPtr("bp-dia-chinh")}, func(m *mayChu) int { return m.ghiNhiemVu.goi }},
		"POST /api/v1/citizen-reports/{maTraCuu}/tasks": {petitionTaskPath(maPhieuThuong),
			[]authz.Perm{"task.create", "feedback.read"}, petitionTaskBody(),
			func(m *mayChu) int { return m.petitionTasks.calls }},
		"POST /api/v1/meetings/{id}/conclusions/{stt}/task": {duongTachNV(idBBThu, 1), []authz.Perm{"task.create"},
			thanTachNhiemVu(), func(m *mayChu) int { return m.ghiBienBan.goi }},
	}
}

func TestRetiredRoleKeysAre400OnEveryDoor(t *testing.T) {
	for name, route := range retiredRoleRoutes() {
		for _, c := range []struct{ key, value string }{
			{"lead_unit", `"bp-vpdu"`},
			{"monitor", `"CB-00412"`},
			{"monitor", `""`},
			{"lead_unit", `null`},
			{"Monitor", `"CB-00412"`},
		} {
			t.Run(name+" "+c.key+"="+c.value, func(t *testing.T) {
				m := dungMayChu(t)
				m.capQuyen(t, route.perms...)
				w := m.goiGhiNVTho(t, http.MethodPost, hostA, route.path, canBoCuaXa(xaA),
					withKey(t, route.body, c.key, c.value))

				doiMa(t, w, http.StatusBadRequest)
				if e := loiTra(t, w); e.Code != "invalid_request" || e.Message != retiredRoleSentence {
					t.Errorf("lỗi = %+v, muốn invalid_request với câu %q", e, retiredRoleSentence)
				}
				if n := route.calls(m); n != 0 {
					t.Errorf("use case chạy %d lần dù thân còn gửi %s (ADR 0065 NV5)", n, c.key)
				}
			})
		}
	}
}

// TestRetiredRoleDoorsStillAcceptTheSurvivors — the control for the table above: the same bodies
// WITHOUT the retired keys pass the decoder, so the 400s are caused by the keys and nothing else.
func TestRetiredRoleDoorsStillAcceptTheSurvivors(t *testing.T) {
	for name, route := range retiredRoleRoutes() {
		t.Run(name, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, route.perms...)
			b, err := json.Marshal(route.body)
			if err != nil {
				t.Fatal(err)
			}
			w := m.goiGhiNVTho(t, http.MethodPost, hostA, route.path, canBoCuaXa(xaA), string(b))
			if w.Code == http.StatusBadRequest {
				t.Fatalf("thân hợp lệ bị 400: %s", w.Body.String())
			}
			if n := route.calls(m); n != 1 {
				t.Errorf("use case chạy %d lần, muốn 1", n)
			}
		})
	}
}

// TestTaskResponseCarriesNoRetiredRole — the response DROPS `lead_unit` and `monitor` (no mirror of
// `unit` / `assignee`): one fact, one field.
func TestTaskResponseCarriesNoRetiredRole(t *testing.T) {
	b, err := json.Marshal(nhiemVuRaNgoai(domain.NhiemVu{
		ID: "nv-001", Ma: "NV19", BoPhanID: "bp-vpdu", NguoiThucHienMa: "CB-00311",
	}))
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"lead_unit", "monitor"} {
		if _, ok := out[k]; ok {
			t.Errorf("phản hồi nhiệm vụ còn trường %q: %s", k, b)
		}
	}
	if string(out["unit"]) != `"bp-vpdu"` || string(out["assignee"]) != `"CB-00311"` {
		t.Errorf("unit/assignee = %s/%s", out["unit"], out["assignee"])
	}
}
