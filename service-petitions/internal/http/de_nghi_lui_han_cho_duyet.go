package http

// The APPROVAL QUEUE of extension requests (§5.8).
//
//	GET /api/v1/task-extensions               task.read   every PENDING request of the commune
//	GET /api/v1/task-extensions?approver=me   task.read   only those whose task names ME as leader
//
// READ ONLY, AND IT GRANTS NOTHING. Who may DECIDE a request is unchanged: `task.extend` at the gate
// of POST /api/v1/tasks/{ma}/extensions/{deNghiID}/decision and ADR 0038's named-leader rule inside
// it. A request shown here to somebody who is not its leader is a request they can read and cannot
// approve.
//
// THIS IS A NEW READ SURFACE, NOT A COPY OF ONE: before this route no GET returned
// `de_nghi_lui_han.ly_do` to anybody — only the requester saw it, in its own POST reply. Opening it to
// every `task.read` holder of the commune was the project owner's decision (27/09/2026), on the ground
// that `task.read` already exposes the same kind of free text (title, description, note). If the
// reason text ever needs a narrower audience, this route is where it leaks first.
//
// A TOP-LEVEL RESOURCE, like task-types and task-priorities, and not a sub-route of one task: the
// queue spans every task of the commune. The per-task routes …/tasks/{ma}/extensions stay as they are.

import (
	"errors"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// deNghiChoDuyetRa is one row of the queue as it leaves the API to a member of staff — what §5.8's
// approve/reject block needs, so the screen renders a page without one request per row.
//
// EVERY PERSON NAMED HERE IS A MEMBER OF STAFF, BY BUSINESS CODE (rule 6, invariant 8). `reason` and
// `task_title` are staff-internal free text; they are returned to staff and never logged (rule 3).
type deNghiChoDuyetRa struct {
	// ID is the request's id — EXACTLY the `{deNghiID}` the decision route takes, together with
	// TaskCode as its `{ma}`.
	ID string `json:"id"`

	// TaskCode is the commune's register number (`NV19`) — the `{ma}` of every task route.
	TaskCode  string `json:"task_code"`
	TaskTitle string `json:"task_title"`

	// TaskDueAt is the task's CURRENT deadline (`han_xu_ly`) — the one approving would move. null
	// when the task has none (no request can be filed on such a task; the wire does not assume it).
	TaskDueAt *time.Time `json:"task_due_at"`

	// TaskAssigner is the leader named on the task (`lanh_dao_giao_viec_ma`) — under ADR 0038 the ONLY
	// person who may decide this request. "" when the task names nobody, and then NOBODY can decide it
	// until one is recorded (ADR 0038's open question, failing closed). Same name as nhiemVuRa's
	// `assigner`, prefixed like the other task fields here.
	TaskAssigner string `json:"task_assigner"`

	NewDueAt    time.Time `json:"new_due_at"`
	Reason      string    `json:"reason"`
	RequestedBy string    `json:"requested_by"`
	RequestedAt time.Time `json:"requested_at"`
}

func deNghiChoDuyetRaNgoai(d domain.DeNghiLuiHanChoDuyet) deNghiChoDuyetRa {
	ra := deNghiChoDuyetRa{
		ID:           d.DeNghi.ID,
		TaskCode:     d.NhiemVuMa,
		TaskTitle:    d.NhiemVuTieuDe,
		TaskAssigner: d.LanhDaoGiaoViecMa,
		NewDueAt:     d.DeNghi.HanMoi,
		Reason:       d.DeNghi.LyDo,
		RequestedBy:  d.DeNghi.NguoiDeNghiMa,
		RequestedAt:  d.DeNghi.ThoiDiem,
	}
	// A ZERO time.Time BECOMES JSON null, here and in one place — never `0001-01-01T00:00:00Z`.
	if !d.HanXuLyHienTai.IsZero() {
		t := d.HanXuLyHienTai
		ra.TaskDueAt = &t
	}
	return ra
}

// errNguoiDuyetKhongHopLe — `approver` carried something other than `me`.
//
// NAMING ANOTHER OFFICER IS REFUSED, not served. Unlike the task register's `assignee` picker, this
// filter is phrased as an identity ("the requests I must decide"), and a value that could name
// somebody else would make it a claim the client makes about who it is. The value is not echoed.
var errNguoiDuyetKhongHopLe = errors.New(
	"`approver` chỉ nhận `me`; bỏ hẳn tham số để xem mọi đề nghị đang chờ duyệt của xã")

// DanhSachDeNghiLuiHan serves one page of the commune's pending extension requests, oldest first.
// GET /api/v1/task-extensions
//
// NO AUDIT ENTRY, for the reason DocNhiemVu gives: no citizen personal data, no cross-commune read.
func (h *Handler) DanhSachDeNghiLuiHan(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	thamSo := r.URL.Query()

	// Both parsed BEFORE any store is touched: a rejected request runs no statement at all.
	yc, err := page.Parse(thamSo, petstore.SapXepDeNghiChoDuyet)
	if err != nil {
		status, maLoi, thongBao := page.HTTPError(err)
		httpx.WriteError(w, status, maLoi, thongBao, "")
		return
	}

	var (
		loc        petstore.LocDeNghiChoDuyet
		nguoiDuyet string
	)
	// Read by map index, as locNhiemVuTuQuery reads its filters.
	if v := thamSo["approver"]; len(v) > 0 {
		nguoiDuyet = v[0]
	}
	switch nguoiDuyet {
	case "":
	case "me":
		// "ME" IS THE SESSION'S STAFF CODE AND NOTHING ELSE. FAIL CLOSED on an empty one: the honest
		// answers are "refuse" and "return the whole commune", and the second is a screen labelled
		// `Chờ tôi duyệt` listing everybody's requests. An empty `.Ma` on a staff principal is a wiring
		// fault, answered 500 as `scope=mine` on GET /api/v1/tasks answers it.
		principal, ok := authz.From(ctx)
		if !ok || principal.Ma == "" {
			h.d.Log.Error("bộ lọc `approver=me` chạy mà chủ thể không có mã cán bộ — SAI CẤU HÌNH ROUTE",
				"xa", string(tenant.MustFrom(ctx)), "duong", r.URL.Path)
			httpx.WriteError(w, http.StatusInternalServerError, "internal",
				"Đã xảy ra lỗi. Vui lòng thử lại.", "")
			return
		}
		loc.LanhDaoGiaoViecMa = principal.Ma
	default:
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", errNguoiDuyetKhongHopLe.Error(), "")
		return
	}

	kq, err := h.d.DeNghiChoDuyet.ChoDuyet(ctx, loc, yc)
	if err != nil {
		// The wrapped error carries the store failure — never a reason text or a title (rule 3).
		h.d.Log.Error("hàng chờ duyệt lùi hạn: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	// make(..., 0, ...) so an empty queue marshals as [] and never as null.
	ra := page.Result[deNghiChoDuyetRa]{
		Items:      make([]deNghiChoDuyetRa, 0, len(kq.Items)),
		NextCursor: kq.NextCursor,
		HasMore:    kq.HasMore,
	}
	for _, d := range kq.Items {
		ra.Items = append(ra.Items, deNghiChoDuyetRaNgoai(d))
	}
	vietJSON(w, http.StatusOK, ra)
}
