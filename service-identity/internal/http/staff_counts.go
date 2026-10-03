package http

import (
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
)

// GET /api/v1/staff-counts — the numbers over the Danh bạ cán bộ screen (12-danh-ba-can-bo.md §2
// KPI cards, §3 the unit filter's `{đang hiện}/{tổng}`). What each number counts is argued on
// idstore.CanBoStore.StaffCounts: `total` is the register's own predicate, `published` the public
// directory's, both shared rather than retyped.

// staffCountsOut is the reply. `total`/`published` are the KPI cards.
//
// PEOPLE IN NO DEPARTMENT ARE `no_department`, NOT A `departments` ENTRY WITH id "". The dropdown is
// keyed by department id, and "" is the value a filter control conventionally uses for "all units";
// an entry with that id would be read as the commune-wide figure by the first client that looks it up
// by key. A separate field cannot be confused with a department, and keeps
// total = no_department.total + Σ departments[].total checkable by anyone.
//
// A department with nobody in it is ABSENT from `departments`: read it as 0/0.
type staffCountsOut struct {
	Total        int                  `json:"total"`
	Published    int                  `json:"published"`
	NoDepartment staffTallyOut        `json:"no_department"`
	Departments  []departmentCountOut `json:"departments"`
}

type staffTallyOut struct {
	Total     int `json:"total"`
	Published int `json:"published"`
}

// departmentCountOut is one unit. `id` is `bo_phan.id` — the value GET /api/v1/staff takes as `unit`.
type departmentCountOut struct {
	ID        string `json:"id"`
	Total     int    `json:"total"`
	Published int    `json:"published"`
}

// StaffCounts serves the register's counts. GET /api/v1/staff-counts
//
// NO QUERY PARAMETER IS READ: the commune comes from the context the edge fixed from Host, never from
// the client (rule 1, forbidden #2), and the route has no filters.
func (h *Handler) StaffCounts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	counts, err := h.d.DanhBa.StaffCounts(ctx)
	if err != nil {
		// The wrapped error names a statement, never a person; it does not reach the client.
		h.d.Log.Error("đếm danh bạ cán bộ: lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	out := staffCountsOut{
		Total:        counts.Total,
		Published:    counts.Published,
		NoDepartment: staffTallyOut{Total: counts.NoDepartment.Total, Published: counts.NoDepartment.Published},
		// Never nil: `departments` marshals as [] for a commune with nobody in a unit.
		Departments: make([]departmentCountOut, 0, len(counts.Departments)),
	}
	for _, d := range counts.Departments {
		out.Departments = append(out.Departments, departmentCountOut{
			ID: d.DepartmentID, Total: d.Total, Published: d.Published,
		})
	}
	vietJSON(w, http.StatusOK, out)
}
