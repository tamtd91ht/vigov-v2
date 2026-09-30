package http

// POST /api/v1/tasks/{ma}/assignment — the assignment act (owner decision 28/09/2026), `task.assign`.
// The rules are in internal/domain/task_assignment.go and the transaction in
// internal/app/task_assignment.go; this file translates HTTP and nothing else.

import (
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// taskAssignmentIn is the body.
//
// THE FIELD NAMES ARE THE ONES GET /api/v1/tasks/{ma} RETURNS (`unit`, `assignee`), so a drawer sends
// back the shape it read.
//
// NO `lead_unit`, NO `monitor` (ADR 0065 NV5): changing the monitoring officer IS changing `assignee`,
// here. A body that still sends either key is refused 400 (retiredRoleKeys), never silently ignored.
//
// EVERY FIELD IS A POINTER: absent means "leave it", "" means "clear it". `unit` may not be cleared
// (400); `assignee` may. SEND ONLY WHAT CHANGES — every staff code sent is checked with identity.
//
// THERE IS NO `status` AND NO `due_at`: the status is the server's consequence of the act, and a
// deadline moves only through an approved extension (ADR 0038).
type taskAssignmentIn struct {
	Unit     *string `json:"unit,omitempty"`
	Assignee *string `json:"assignee,omitempty"`
	// Note is the officer's optional line; it goes on the timeline under the generated from→to
	// sentence, and only its LENGTH reaches the audit entry (it may name a citizen's case).
	Note string `json:"note,omitempty"`
}

// ReassignTask hands the task over. POST /api/v1/tasks/{ma}/assignment
func (h *Handler) ReassignTask(w http.ResponseWriter, r *http.Request) {
	var in taskAssignmentIn
	if !decodeRefusingKeys(w, r, &in, retiredRoleKeys) {
		return
	}
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheNhiemVu(w, r)
		return
	}

	n, err := h.d.GhiNhiemVu.Reassign(r.Context(), r.PathValue("ma"), app.TaskAssignmentRequest{
		Change: domain.TaskAssignmentChange{
			Unit: in.Unit, Assignee: in.Assignee,
		},
		Note: in.Note,
	}, nguoi)
	if err != nil {
		h.writeAssignmentError(w, r, err)
		return
	}
	vietJSON(w, http.StatusOK, nhiemVuRaNgoai(n))
}

// writeAssignmentError maps the refusals only this act raises, then falls through to the register's
// shared mapping (404, 409 `task_state` on a race, 400 domain input, 500).
func (h *Handler) writeAssignmentError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, app.ErrAssignmentStaffInvalid):
		// ONE SENTENCE for unknown, another commune, locked, no account — the code is not echoed, so the
		// answer is identical whichever it was (rule 1: no existence leak across communes).
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Cán bộ được chọn không nhận được việc. Hãy chọn người khác trong danh sách.", "")
	case errors.Is(err, app.ErrAssignmentStaffUnchecked):
		h.d.Log.Warn("CẢNH BÁO: từ chối giao lại nhiệm vụ vì chưa kiểm được cán bộ nhận việc",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "assignee_check_unavailable",
			"Chưa kiểm tra được cán bộ nhận việc nên nhiệm vụ CHƯA được giao lại. "+
				"Vui lòng thử lại sau ít phút.", "")
	case errors.Is(err, domain.ErrAssignmentNoChange):
		httpx.WriteError(w, http.StatusConflict, "no_change",
			cauTuChoi(err, domain.ErrAssignmentNoChange), "")
	case errors.Is(err, domain.ErrTaskClosedForAssignment):
		httpx.WriteError(w, http.StatusConflict, "task_state",
			cauTuChoi(err, domain.ErrTaskClosedForAssignment), "")
	default:
		h.traLoiLoiNhiemVu(w, r, "giao lại nhiệm vụ", err)
	}
}
