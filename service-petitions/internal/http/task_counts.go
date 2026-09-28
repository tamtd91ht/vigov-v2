package http

// The PER-STATUS COUNT of the task register (§4.1) — the real number over each Kanban column.
//
//	GET /api/v1/task-counts   task.read   same filters as GET /api/v1/tasks
//
// # WHY A ROUTE OF ITS OWN AND NOT A `total` ON THE LIST
//
// page.Result carries no total, deliberately (core/page: a keyset read never computes one, and a
// COUNT(*) on every list request is §5 #4 of skills/rest-api-design). "A total that somebody needs is
// a separate route" — this is that route, and it answers the one question the board asks: how many
// tasks sit in each of the seven columns under the filters on screen. Without it the header could
// only print `20+`, which a leader reads as `20`.
//
// # WHY A SEPARATE COLLECTION AND NOT `tasks/counts`
//
// `tasks/{ma}` owns the second segment: `GET /api/v1/tasks/counts` would be a task numbered
// `counts`. A hyphenated top-level noun is the shape task-types, task-statuses, task-extensions and
// task-summary already use for the same reason.
//
// # THE SAME FILTERS, THROUGH THE SAME CODE
//
// taskFilterFromRequest and store.locNhiemVuThanhSQL are shared with the list, so a count and the
// column it heads cannot disagree about what `scope=mine`, `q`, `late`, `parent` or `metric` mean.
// Paging parameters (`limit`, `cursor`, `sort`, `order`) are NOT READ: they choose which rows of a
// set are shown, not which rows are in it, so the board can send the list's query string unchanged.
//
// NO AUDIT ENTRY (rule 6, invariant 7): counts of one commune, no personal data, no cross-commune read.

import (
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// taskCountsOut is the reply: one entry per status of the closed list, ALWAYS ALL SEVEN.
//
// A LIST AND NOT A MAP KEYED BY CODE, so the contract generator can type it and a client iterates it
// without guessing keys. THE ORDER IS THE DEFAULT ORDER OF THE SEVEN CODES and nothing more: the
// column order a commune chose belongs to GET /api/v1/task-statuses, and a second copy of it here
// would be a second source for one fact (rule 9).
//
// A STATUS WITH NO TASK IS `0`, NOT ABSENT: every code is a column on the board, and an absent entry
// would leave the client to decide whether it meant zero or "not counted".
type taskCountsOut struct {
	ByStatus []taskStatusCountOut `json:"by_status"`
}

// taskStatusCountOut is one column's number.
type taskStatusCountOut struct {
	// Status is one of the seven codes of §6 (`moi-giao`, `da-tiep-nhan`, …) — Vietnamese without
	// diacritics, like every enum value on this API (ADR 0011).
	Status string `json:"status"`
	Count  int    `json:"count"`
}

// TaskCounts serves the per-status counts. GET /api/v1/task-counts
func (h *Handler) TaskCounts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// THE LIST'S OWN FILTER PARSING, refusals included: an unknown status, a malformed `soon`, a
	// malformed metric period — each is the same 400 with the same sentence the list gives, before
	// any statement runs; and `soon=true` / `scope=related` ask identity exactly as the list does
	// (409 / 503 on the same conditions).
	loc, ok := h.taskFilterFromRequest(w, r, r.URL.Query())
	if !ok {
		return
	}

	counts, err := h.d.DanhSachNhiemVu.CountByStatus(ctx, loc)
	if err != nil {
		h.d.Log.Error("đếm nhiệm vụ theo trạng thái: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	defaults := domain.MacDinhTrangThaiNhiemVu()
	out := taskCountsOut{ByStatus: make([]taskStatusCountOut, 0, len(defaults))}
	for _, d := range defaults {
		out.ByStatus = append(out.ByStatus, taskStatusCountOut{
			Status: string(d.Ma),
			Count:  counts[d.Ma],
		})
	}
	vietJSON(w, http.StatusOK, out)
}
