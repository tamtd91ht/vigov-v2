package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// POST /api/v1/sla and DELETE /api/v1/sla/{id} (ADR 0079 lô 2 Q4). The four permission cases run in
// sla_test.go through moiTuyenSLA, which lists both routes. What is asserted here is what the HANDLER
// owns: the body mapping, and which status and code each refusal of the use case becomes.

func errorCode(t *testing.T, body []byte) string {
	t.Helper()
	var e struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(body, &e); err != nil {
		t.Fatalf("thân lỗi không phải JSON: %s", body)
	}
	return e.Code
}

// THE SIX FIGURES REACH THE USE CASE IN THE RIGHT FIELDS — all different, so a transposition shows.
func TestAddSLAFieldRowMapsBody(t *testing.T) {
	m := dungMayChuSLA(t)
	body := `{"work_kind":"phan-anh","field":"an-ninh","acknowledge_hours":2,"resolve_hours":12,` +
		`"due_soon_hours":4,"escalate_leader_hours":8,"escalate_president_hours":17,"unassigned_hold_hours":6}`
	w := m.goiIdem(t, "POST", hostA, "/api/v1/sla", body, m.tokenCho(t, xaA, sidA))
	if w.Code != http.StatusCreated {
		t.Fatalf("mã = %d — thân: %s", w.Code, w.Body.String())
	}
	got := m.ghiSLA.addLast
	want := app.AddFieldRuleRequest{Kind: domain.LoaiViecPhanAnh, Field: "an-ninh",
		AcknowledgeHours: 2, ResolveHours: 12, DueSoonHours: 4, EscalateLeaderHours: 8,
		EscalatePresidentHours: 17, UnassignedHoldHours: 6}
	if got != want {
		t.Errorf("yêu cầu tới use case = %+v, muốn %+v", got, want)
	}
	var row dongSLARa
	if err := json.Unmarshal(w.Body.Bytes(), &row); err != nil || row.ID != idDongSLAThu {
		t.Errorf("thân trả về = %s, muốn dòng %s", w.Body.String(), idDongSLAThu)
	}
}

// A typed 0 for the optional threshold is refused, not read as "do not report" — same rule as PATCH.
func TestAddSLAFieldRowZeroUnassignedHoldIs400(t *testing.T) {
	m := dungMayChuSLA(t)
	body := `{"work_kind":"phan-anh","field":"an-ninh","acknowledge_hours":2,"resolve_hours":12,` +
		`"due_soon_hours":4,"escalate_leader_hours":8,"escalate_president_hours":17,"unassigned_hold_hours":0}`
	w := m.goiIdem(t, "POST", hostA, "/api/v1/sla", body, m.tokenCho(t, xaA, sidA))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("mã = %d, muốn 400 — thân: %s", w.Code, w.Body.String())
	}
	if n := m.ghiSLA.soLanGoi(); n != 0 {
		t.Errorf("use case chạy %d lần", n)
	}
}

func TestAddSLAFieldRowRefusalsMapToStatus(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{app.ErrSLAFieldRowExists, http.StatusConflict, "sla_rule_exists"},
		{app.ErrSLAFieldUnverifiable, http.StatusBadRequest, "sla_field_unverifiable"},
		{fmt.Errorf("%w: x", app.ErrSLAFieldNotInList), http.StatusBadRequest, "sla_field_unknown"},
		{fmt.Errorf("%w: x", app.ErrSLAFieldListUnavailable), http.StatusServiceUnavailable, "sla_field_check_unavailable"},
		{domain.ErrSLAKindHasNoFieldRows, http.StatusBadRequest, "invalid_request"},
		{domain.ErrSLAFieldMissing, http.StatusBadRequest, "invalid_request"},
		{domain.ErrChairmanBeforeUnitHead, http.StatusBadRequest, "invalid_request"},
		{fmt.Errorf("db down"), http.StatusInternalServerError, "internal"},
	}
	for _, c := range cases {
		m := dungMayChuSLA(t)
		m.ghiSLA.loi = c.err
		w := m.goiIdem(t, "POST", hostA, "/api/v1/sla", addSLAFieldRowBody, m.tokenCho(t, xaA, sidA))
		if w.Code != c.status || errorCode(t, w.Body.Bytes()) != c.code {
			t.Errorf("%v: mã = %d %s, muốn %d %s", c.err, w.Code, w.Body.String(), c.status, c.code)
		}
	}
}

// The exact sentence the card fixes for the duplicate — the screen shows it as is.
func TestAddSLAFieldRowExistsSentence(t *testing.T) {
	m := dungMayChuSLA(t)
	m.ghiSLA.loi = app.ErrSLAFieldRowExists
	w := m.goiIdem(t, "POST", hostA, "/api/v1/sla", addSLAFieldRowBody, m.tokenCho(t, xaA, sidA))
	var e struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &e)
	if e.Message != "Lĩnh vực này đã có thời hạn riêng — hãy sửa dòng sẵn có." {
		t.Errorf("câu = %q", e.Message)
	}
}

// POST WITHOUT AN Idempotency-Key IS REFUSED before the use case.
func TestAddSLAFieldRowNeedsIdempotencyKey(t *testing.T) {
	m := dungMayChuSLA(t)
	w := m.goi(t, "POST", hostA, "/api/v1/sla", addSLAFieldRowBody, m.tokenCho(t, xaA, sidA))
	if w.Code < 400 || m.ghiSLA.soLanGoi() != 0 {
		t.Errorf("thiếu Idempotency-Key: mã = %d, use case %d lần", w.Code, m.ghiSLA.soLanGoi())
	}
}

func TestRemoveSLAFieldRowPassesIDAndReason(t *testing.T) {
	m := dungMayChuSLA(t)
	w := m.goi(t, "DELETE", hostA, "/api/v1/sla/"+idDongSLAThu, `{"reason":"Gộp vào dòng mặc định"}`,
		m.tokenCho(t, xaA, sidA))
	if w.Code != http.StatusNoContent || w.Body.Len() != 0 {
		t.Fatalf("mã = %d, thân %q — muốn 204 rỗng", w.Code, w.Body.String())
	}
	if m.ghiSLA.idCuoi != idDongSLAThu || m.ghiSLA.reasonLast != "Gộp vào dòng mặc định" {
		t.Errorf("use case nhận id %q lý do %q", m.ghiSLA.idCuoi, m.ghiSLA.reasonLast)
	}
}

// NO BODY, `{}` OR A BLANK REASON IS A DELETE WITHOUT A REASON (owner decision 10/10/2026): 204, and
// the use case receives "" — the domain turns it into the fixed sentence.
func TestRemoveSLAFieldRowWithoutReasonReachesUseCase(t *testing.T) {
	for _, body := range []string{"", "{}", `{"reason":""}`} {
		m := dungMayChuSLA(t)
		w := m.goi(t, "DELETE", hostA, "/api/v1/sla/"+idDongSLAThu, body, m.tokenCho(t, xaA, sidA))
		if w.Code != http.StatusNoContent {
			t.Errorf("thân %q: mã = %d — %s", body, w.Code, w.Body.String())
		}
		if m.ghiSLA.idCuoi != idDongSLAThu || m.ghiSLA.reasonLast != "" {
			t.Errorf("thân %q: use case nhận id %q lý do %q", body, m.ghiSLA.idCuoi, m.ghiSLA.reasonLast)
		}
	}
}

func TestRemoveSLAFieldRowRefusalsMapToStatus(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{idstore.ErrDongSLAKhongTonTai, http.StatusNotFound, "sla_row_not_found"},
		{domain.ErrSLADefaultRowNotRemovable, http.StatusConflict, "default_sla_rule"},
		{domain.ErrSLADeleteReasonTooLong, http.StatusBadRequest, "invalid_request"},
	}
	for _, c := range cases {
		m := dungMayChuSLA(t)
		m.ghiSLA.loi = c.err
		w := m.goi(t, "DELETE", hostA, "/api/v1/sla/"+idDongSLAThu, `{"reason":"x"}`, m.tokenCho(t, xaA, sidA))
		if w.Code != c.status || errorCode(t, w.Body.Bytes()) != c.code {
			t.Errorf("%v: mã = %d %s, muốn %d %s", c.err, w.Code, w.Body.String(), c.status, c.code)
		}
	}
}
