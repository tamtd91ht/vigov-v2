package http

// The EXTENSION HISTORY of one task (§5.8, user decision 07/10/2026):
//
//	GET /api/v1/tasks/{ma}/extensions   task.read   every request of THIS task, newest first
//
// READ ONLY, AND IT GRANTS NOTHING. Who may decide a request is unchanged (`task.extend` at the
// decision route's gate plus ADR 0038's named-leader rule inside it).
//
// WHAT IT ADDS OVER THE QUEUE (GET /api/v1/task-extensions?task=NV19): decided requests too, and the
// decider's note (`decision_note`, migration 0031). The queue and its count are NOT touched — they
// keep their own predicate builder (store/de_nghi_lui_han_cho_duyet.go).
//
// `reason` and `decision_note` are staff-internal free text, returned to `task.read` holders of the
// commune — the audience the queue already opened `reason` to (project owner, 27/09/2026) — and never
// logged (rule 3).

import (
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// TaskExtensionHistory serves one page of one task's extension requests, newest first.
// GET /api/v1/tasks/{ma}/extensions
//
// THE TASK IS READ FIRST, THROUGH THE SAME READER AS GET /api/v1/tasks/{ma}, and that is the check:
// the history is keyed by the task's internal id, which this handler has only after that read — so a
// soft-deleted task, another commune's number and an unknown number all stop at the one 404
// (khongTimThayNhiemVu), and the request table is never touched for any of them.
//
// NO AUDIT ENTRY, for the reason DocNhiemVu gives: no citizen personal data, no cross-commune read.
func (h *Handler) TaskExtensionHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	taskCode := r.PathValue("ma")
	if taskCode == "" {
		h.khongTimThayNhiemVu(w)
		return
	}

	// Parsed BEFORE any store is touched: a rejected page request runs no statement at all.
	req, err := page.Parse(r.URL.Query(), petstore.TaskExtensionHistorySort)
	if err != nil {
		status, code, message := page.HTTPError(err)
		httpx.WriteError(w, status, code, message, "")
		return
	}

	task, err := h.d.NhiemVu.TheoMa(ctx, taskCode)
	if err != nil {
		if errors.Is(err, petstore.ErrNhiemVuKhongTonTai) {
			h.khongTimThayNhiemVu(w)
			return
		}
		h.d.Log.Error("đọc nhiệm vụ cho lịch sử lùi hạn: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	result, err := h.d.DeNghiChoDuyet.TaskHistory(ctx, task.ID, req)
	if err != nil {
		// The wrapped error carries the store failure — never a reason or a note text (rule 3).
		h.d.Log.Error("đọc lịch sử lùi hạn: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	// make(..., 0, ...) so an empty history marshals as [] and never as null.
	out := page.Result[deNghiLuiHanRa]{
		Items:      make([]deNghiLuiHanRa, 0, len(result.Items)),
		NextCursor: result.NextCursor,
		HasMore:    result.HasMore,
	}
	for _, d := range result.Items {
		out.Items = append(out.Items, deNghiRaNgoai(d))
	}
	vietJSON(w, http.StatusOK, out)
}
