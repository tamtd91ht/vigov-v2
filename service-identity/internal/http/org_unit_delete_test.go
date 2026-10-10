package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// DELETE /api/v1/org-units/{id}. The rule 5 invariant 7 set (401 · 403 wrong key · 403 right key
// wrong commune · 2xx) runs in bo_phan_ghi_test.go over moiTuyenSoDo, which lists this route. What
// is asserted here is what only this handler owns: what reaches the use case, and the wire shape of
// the three answers only the delete has.

func callDeleteOrgUnit(t *testing.T, m *mayChu, body string) (int, string) {
	t.Helper()
	w := m.goiSoDo(t, tuyenSoDo{method: "DELETE", duong: duongBoPhan + "/bp-002", than: body},
		hostA, m.tokenCho(t, xaA, sidA))
	return w.Code, w.Body.String()
}

func TestDeleteOrgUnitForwardsIDReasonAndCommune(t *testing.T) {
	m := dungMayChuSoDo(t)
	code, body := callDeleteOrgUnit(t, m, `{"reason":"  sáp nhập vào Văn phòng "}`)
	if code != http.StatusNoContent || body != "" {
		t.Fatalf("mã = %d, thân = %q — muốn 204 không thân", code, body)
	}
	g := m.ghiBoPhan
	if g.idCuoi != "bp-002" || g.xaCuoi != xaA {
		t.Errorf("use case nhận id=%q xã=%q", g.idCuoi, g.xaCuoi)
	}
	// Passed as sent; trimming is the domain's (one owner).
	if g.lastReason != "  sáp nhập vào Văn phòng " {
		t.Errorf("lý do tới use case = %q", g.lastReason)
	}
}

// NO BODY, `{}` OR A BLANK REASON IS A DELETE WITHOUT A REASON (owner decision 10/10/2026): 204, and
// the use case receives "" — the domain turns it into the fixed sentence.
func TestDeleteOrgUnitWithoutReasonReachesUseCase(t *testing.T) {
	for _, body := range []string{"", "{}", `{"reason":""}`} {
		m := dungMayChuSoDo(t)
		if code, out := callDeleteOrgUnit(t, m, body); code != http.StatusNoContent {
			t.Errorf("thân %q: mã = %d — %s", body, code, out)
		}
		if m.ghiBoPhan.goi != 1 || m.ghiBoPhan.lastReason != "" {
			t.Errorf("thân %q: use case chạy %d lần, lý do %q", body, m.ghiBoPhan.goi, m.ghiBoPhan.lastReason)
		}
	}
}

// 409 CARRIES EVERY KIND, zeros included, and the sentence §12.4 asks for.
func TestDeleteOrgUnitInUseBodyShape(t *testing.T) {
	m := dungMayChuSoDo(t)
	m.ghiBoPhan.loi = &app.OrgUnitInUseError{Holdings: domain.OrgUnitHoldings{Staff: 3, OpenTasks: 2}}
	code, body := callDeleteOrgUnit(t, m, `{"reason":"sáp nhập"}`)
	if code != http.StatusConflict {
		t.Fatalf("mã = %d, muốn 409 — %s", code, body)
	}
	var out struct {
		Code     string         `json:"code"`
		Message  string         `json:"message"`
		Holdings map[string]int `json:"holdings"`
	}
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("thân không đọc được: %v — %s", err, body)
	}
	if out.Code != "org_unit_in_use" {
		t.Errorf("code = %q", out.Code)
	}
	if !strings.Contains(out.Message, "3 cán bộ") || !strings.Contains(out.Message, "chuyển trước khi xoá") {
		t.Errorf("message = %q", out.Message)
	}
	want := map[string]int{"staff": 3, "child_units": 0, "open_petitions": 0, "open_tasks": 2, "open_incoming_documents": 0}
	if len(out.Holdings) != len(want) {
		t.Errorf("holdings có %d trường, muốn %d (số 0 cũng phải in): %v", len(out.Holdings), len(want), out.Holdings)
	}
	for k, v := range want {
		if n, ok := out.Holdings[k]; !ok || n != v {
			t.Errorf("holdings.%s = %d (có=%v), muốn %d", k, n, ok, v)
		}
	}
}

func TestDeleteOrgUnitErrorMapping(t *testing.T) {
	for _, c := range []struct {
		name string
		err  error
		code int
		key  string
	}{
		{"owner down", fmt.Errorf("%w: petitions: rpc unavailable", app.ErrOrgUnitHoldingsUnavailable), http.StatusServiceUnavailable, "org_unit_delete_unavailable"},
		{"not configured", app.ErrOrgUnitDeleteNotConfigured, http.StatusServiceUnavailable, "org_unit_delete_not_configured"},
		{"not found", idstore.ErrKhongTimThayBoPhan, http.StatusNotFound, "org_unit_not_found"},
		{"reason too long", domain.ErrOrgUnitDeleteReasonTooLong, http.StatusBadRequest, "invalid_request"},
		{"store failure", fmt.Errorf("bo_phan: xoá mềm bộ phận: boom"), http.StatusInternalServerError, "internal"},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := dungMayChuSoDo(t)
			m.ghiBoPhan.loi = c.err
			code, body := callDeleteOrgUnit(t, m, `{"reason":"sáp nhập"}`)
			if code != c.code || !strings.Contains(body, `"code":"`+c.key+`"`) {
				t.Errorf("mã = %d, thân = %s — muốn %d %s", code, body, c.code, c.key)
			}
			// The wrapped internal error never reaches the client (rule 3, forbidden #3).
			if strings.Contains(body, "rpc unavailable") || strings.Contains(body, "boom") {
				t.Errorf("lỗi nội bộ lọt ra thân: %s", body)
			}
		})
	}
}

func TestDeleteOrgUnitMalformedBodyRefusedBeforeUseCase(t *testing.T) {
	m := dungMayChuSoDo(t)
	if code, body := callDeleteOrgUnit(t, m, `{"reason":`); code != http.StatusBadRequest {
		t.Errorf("mã = %d — %s", code, body)
	}
	if m.ghiBoPhan.goi != 0 {
		t.Errorf("thân hỏng mà use case vẫn chạy")
	}
}
