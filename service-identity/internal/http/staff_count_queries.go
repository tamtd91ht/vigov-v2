package http

import (
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
)

// POST /api/v1/staff-count-queries — how many people the register's CURRENT search matches (owner
// decision 08/10/2026), so the screen can print "Hiển thị 20 / 57" instead of "20+".
//
// WHY A ROUTE OF ITS OWN AND NOT A `total` ON THE PAGE: core/page refuses one on purpose ("a total that
// somebody needs is a separate route"), the shape service-petitions' GET /api/v1/task-counts already
// takes. GET /api/v1/staff-counts stays as it is: it answers the commune-wide KPI cards, with no filter.
//
// WHY A POST FOR A READ, AND WHY THIS NOUN: the filter carries the search text, which is usually a name
// or a number, and a URL is written into access logs, proxies and browser history (rule 3, forbidden
// #4) — the reason POST /api/v1/staff/searches gives. Each POST describes one count query and creates
// nothing; `staff-count-queries` is the nominalised noun, top-level for the same reason `staff-counts`
// is (`staff/{id}` owns the second segment).
//
// THE SAME FILTER, THROUGH THE SAME CODE: staffSearchFilter (validation, normalisation) and
// store.CountMatching (locTomTat + menhDeLocCanBo, the list's own predicate). Paging fields are not
// read: they choose which rows of a set are shown, not which rows are in it.
//
// NO AUDIT ENTRY: a count of one commune's register, no personal data in the answer, no cross-commune
// read — rule 6 invariant 7 does not reach it, exactly as for GET /api/v1/staff-counts.

// staffCountQueryIn is the whole body: the three filter fields of timCanBoVao, with the SAME
// validation, minus paging and sort.
//
//	q          REQUIRED, 1..200 characters after normalisation — as on the search
//	unit       optional department id → `bo_phan_id`
//	published  optional → `hien_tren_mini_app`. null/absent = both
type staffCountQueryIn struct {
	Q         string `json:"q"`
	Unit      string `json:"unit"`
	Published *bool  `json:"published"`
}

// staffCountQueryOut is the reply: the number of live (not soft-deleted) people of the commune the
// filter matches.
type staffCountQueryOut struct {
	Total int `json:"total"`
}

// CountStaffMatching serves one count. POST /api/v1/staff-count-queries
//
// `q` IS NEVER LOGGED: the one log line on this path carries the commune and the store error only
// (rule 3, invariant 1).
func (h *Handler) CountStaffMatching(w http.ResponseWriter, r *http.Request) {
	var in staffCountQueryIn
	if !docThanCanBo(w, r, &in) {
		return
	}
	loc, ok := staffSearchFilter(w, in.Q, in.Unit, in.Published)
	if !ok {
		return
	}

	ctx := r.Context()
	total, err := h.d.DanhBa.CountMatching(ctx, loc)
	if err != nil {
		// The wrapped error names a statement, never a person; `loc` is NOT logged — it holds the text.
		h.d.Log.Error("đếm cán bộ theo tìm kiếm: lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	vietJSON(w, http.StatusOK, staffCountQueryOut{Total: total})
}
