package http

// The STAFF READ of a task's progress log — "Nhật ký & Trao đổi" (migration 0006; docs/ui-ux/02 §5.9) —
// and its one manual write.
//
//	GET  /api/v1/tasks/{ma}/log-entries   task.read
//	POST /api/v1/tasks/{ma}/log-entries   task.read at the gate; assignee / related / task.update in
//	                                      the use case (vigov-require a37ec96, domain/task_participant.go)
//
// MODELLED ON THE PETITION LOGBOOK'S READ (nhat_ky_phan_anh.go, DocNhatKyPhieu) so the two timelines
// read alike: same URL noun, same newest-first order, same core/page cursor, same field names where
// the two tables hold the same fact.
//
// # WHERE THE ROW SHAPE DIFFERS FROM THE PETITION LOGBOOK'S, AND WHY
//
//	no `action`   `nhat_ky_nhiem_vu` has no act column (the petition logbook's `hanh_vi` is migration
//	              0013's). Inventing one from the text or from a status change would be a fact nobody
//	              recorded.
//	`note` always non-empty  the column is NOT NULL with a non-blank CHECK (0006): every task entry
//	              carries text, a generated sentence when the officer typed none.
//
// NO `actor_name` ON THE WIRE, for the reason the petition logbook's header gives: nothing in this
// service resolves staff names through identity, and adding that dependency to a read path is a
// decision reported, not made.

import (
	"errors"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// nhatKyNhiemVuRa is one timeline row as it leaves the API to a member of staff.
type nhatKyNhiemVuRa struct {
	// ID is the row's internal id — the tie-break of the page order, and what a screen keys a list
	// item on. It names nothing outside this service. (The petition logbook exposes its row id too.)
	ID string `json:"id"`

	// At is the instant of the act, from the same clock as the act itself.
	At time.Time `json:"at"`

	// ActorCode is the STAFF BUSINESS CODE of who wrote the entry (`CB-00123`), never an internal id
	// (rule 6, invariant 8).
	ActorCode string `json:"actor_code"`

	// Status is the state the task stood in at this moment — AFTER the act, for an act that moved it
	// (domain.NhatKyNhiemVu.TrangThaiTaiThoiDiem). One of the seven codes of §6.
	Status string `json:"status"`

	// Unit and Assignee are who the task was handed to AT THIS STEP (`bo_phan_id`,
	// `nguoi_phu_trach_ma`) — "" on an entry that changed no assignment. Unit is identity's
	// department id; Assignee a staff business code. The same names nhiemVuRa uses for the CURRENT
	// holder, and the petition logbook for its step.
	Unit     string `json:"unit"`
	Assignee string `json:"assignee"`

	// Note is the entry text. Staff-internal free text about the work.
	Note string `json:"note"`

	// NO `attachments` FIELD, ON PURPOSE — `dinh_kem` exists but nothing writes it (no file store),
	// and an untyped array on the wire is refused by web-admin's type generator. The petition
	// logbook leaves its twin off for the same reason; the day the store exists it is added as an
	// OPTIONAL field.
}

func nhatKyNhiemVuRaNgoai(e domain.NhatKyNhiemVu) nhatKyNhiemVuRa {
	return nhatKyNhiemVuRa{
		ID: e.ID, At: e.ThoiDiem, ActorCode: e.NguoiMa, Status: string(e.TrangThaiTaiThoiDiem),
		Unit: e.BoPhanID, Assignee: e.NguoiPhuTrachMa, Note: e.NoiDung,
	}
}

// DocNhatKyNhiemVu serves one page of one task's timeline, newest first.
// GET /api/v1/tasks/{ma}/log-entries
//
// THE TASK IS READ FIRST, THROUGH THE SAME READER AS GET /api/v1/tasks/{ma}, and that is the check:
// the log is keyed by the task's internal id, which this handler has only after that read — so a
// soft-deleted task, another commune's number and an unknown number all stop at the one 404
// (khongTimThayNhiemVu), and the log is never touched for any of them.
//
// NO AUDIT ENTRY, for the reason DocNhiemVu gives: no citizen personal data, no cross-commune read.
func (h *Handler) DocNhatKyNhiemVu(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ma := r.PathValue("ma")
	if ma == "" {
		h.khongTimThayNhiemVu(w)
		return
	}

	// Parsed BEFORE any store is touched: a rejected page request runs no statement at all.
	yc, err := page.Parse(r.URL.Query(), petstore.SapXepNhatKyNhiemVu)
	if err != nil {
		status, maLoi, thongBao := page.HTTPError(err)
		httpx.WriteError(w, status, maLoi, thongBao, "")
		return
	}

	n, err := h.d.NhiemVu.TheoMa(ctx, ma)
	if err != nil {
		if errors.Is(err, petstore.ErrNhiemVuKhongTonTai) {
			h.khongTimThayNhiemVu(w)
			return
		}
		h.d.Log.Error("đọc nhiệm vụ cho nhật ký: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	kq, err := h.d.NhiemVu.NhatKyCuaNhiemVu(ctx, n.ID, yc)
	if err != nil {
		// The wrapped error carries the store failure — never an entry text (rule 3).
		h.d.Log.Error("đọc nhật ký nhiệm vụ: lỗi hệ thống",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	// make(..., 0, ...) so an empty timeline marshals as [] and never as null.
	ra := page.Result[nhatKyNhiemVuRa]{
		Items:      make([]nhatKyNhiemVuRa, 0, len(kq.Items)),
		NextCursor: kq.NextCursor,
		HasMore:    kq.HasMore,
	}
	for _, e := range kq.Items {
		ra.Items = append(ra.Items, nhatKyNhiemVuRaNgoai(e))
	}
	vietJSON(w, http.StatusOK, ra)
}

// taskLogEntryIn is the body of POST /api/v1/tasks/{ma}/log-entries. `note` is MANDATORY (no
// omitempty): migration 0006 refuses a blank entry, and an entry in an append-only table can never be
// corrected. The same field name the petition note (ghiChuPhieuVao) and the timeline row use.
//
// ⚠ STAFF FREE TEXT — it may quote a citizen's case (rule 3). At most domain.NoiDungNhatKyToiDa
// characters.
type taskLogEntryIn struct {
	Note string `json:"note"`
}

// PermTaskUpdate is the commune-wide edit key — a37ec96's first "full" case. A constant because it is
// used with Checker.Allows, not in a route declaration (tools/apidoc reads only literals there).
const PermTaskUpdate authz.Perm = "task.update"

// AddTaskLogEntry appends one manual entry. POST /api/v1/tasks/{ma}/log-entries
//
// THIS HANDLER ANSWERS ONE QUESTION AND DECIDES NOTHING — the shape of GhiChuPhieu. It reads whether
// the caller holds `task.update`; whether the entry is allowed (assignee, related person, or that key)
// is app.AddLogEntry's, on the row read FOR UPDATE.
func (h *Handler) AddTaskLogEntry(w http.ResponseWriter, r *http.Request) {
	var in taskLogEntryIn
	if !docThan(w, r, &in) {
		return
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheNhiemVu(w, r)
		return
	}
	ctx := r.Context()
	row, err := h.d.GhiNhiemVu.AddLogEntry(ctx, r.PathValue("ma"), in.Note, actor, h.hasTaskUpdate(r))
	if err != nil {
		h.traLoiLoiNhiemVu(w, r, "ghi nhật ký", err)
		return
	}
	// What a retry carrying the same Idempotency-Key is told: the row's id, never the text.
	idem.RecordCode(ctx, row.ID)
	vietJSON(w, http.StatusCreated, nhatKyNhiemVuRaNgoai(row))
}

// hasTaskUpdate: does this account hold `task.update`? FAIL CLOSED — no principal is `false`.
func (h *Handler) hasTaskUpdate(r *http.Request) app.TaskUpdateRight {
	principal, ok := authz.From(r.Context())
	if !ok {
		return false
	}
	return app.TaskUpdateRight(h.d.Checker.Allows(r.Context(), principal, PermTaskUpdate))
}
