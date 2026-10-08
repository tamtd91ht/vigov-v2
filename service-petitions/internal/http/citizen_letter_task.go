package http

// POST /api/v1/citizen-letter-tasks — "⇄ Chuyển thành nhiệm vụ" on the citizen-letter drawer (docs/ui-ux/05
// §3.5; ADR 0085 A; ADR 0084 #6). The act is app.CitizenLetterTaskCreation.

import (
	"context"
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// CitizenLetterTaskCreator books a task whose source is a citizen letter. *app.CitizenLetterTaskCreation
// satisfies it. A Deps field of its own: it calls another service, opens a transaction and audits in it.
type CitizenLetterTaskCreator interface {
	CreateTask(ctx context.Context, letterID string, yc app.YeuCauTaoNhiemVu, actor audit.Actor) (domain.NhiemVu, error)
}

// citizenLetterTaskIn is the dialog — petitionTaskIn's form plus `letter_id`, MINUS `due_at`.
//
// # NO SOURCE, NO DEADLINE ON THE WIRE — AND ONE THAT IS SENT IS REFUSED
//
// The server sets `nguon_giao = don-thu`, `nguon_id` from documents' answer, and the deadline from the
// letter's (ADR 0085 A5, A6: "Client không gửi hạn"). A client sending `source`, `source_id` or `due_at`
// believes it chose them; it is told it did not (400), rather than having its value silently dropped —
// the refusal POST /api/v1/citizen-reports/{maTraCuu}/tasks gives.
//
// `title` IS TYPED, NOT DERIVED (A7): the screen pre-fills it from the summary, a person confirms it.
// The server never copies the summary.
type citizenLetterTaskIn struct {
	LetterID string `json:"letter_id"`

	Code     string `json:"code,omitempty"`
	AutoCode bool   `json:"auto_code"`

	Type     string `json:"type"`
	Bloc     string `json:"bloc,omitempty"`
	Title    string `json:"title"`
	Body     string `json:"description,omitempty"`
	Priority string `json:"priority,omitempty"`
	Note     string `json:"note,omitempty"`

	// Unit / Assignee: optional. BOTH empty → the letter's holding unit and assignee (ADR 0085 §Trả lời #6);
	// none there either → 422 `assignment_required`.
	Unit     string `json:"unit,omitempty"`
	Assignee string `json:"assignee,omitempty"`
	Assigner string `json:"assigner,omitempty"`

	// Parent: the parent task's REGISTER NUMBER (`NV19`), as on POST /api/v1/tasks.
	Parent string `json:"parent,omitempty"`

	Documents []vanBanNhiemVuVao `json:"documents,omitempty"`
}

const citizenLetterSourceSentence = "Nhiệm vụ chuyển từ đơn thư luôn lấy nguồn là chính đơn thư `letter_id` — " +
	"không gửi `source` hay `source_id`."

// citizenLetterTaskRefusedKeys are refused by NAME, whatever their value.
var citizenLetterTaskRefusedKeys = append([]refusedKey{
	{"source", citizenLetterSourceSentence},
	{"source_id", citizenLetterSourceSentence},
	{"due_at", "Hạn nhiệm vụ lấy theo hạn hiện tại của đơn thư (ngày hạn, 17:00) — không gửi `due_at`. " +
		"Đơn không đặt hạn thì nhiệm vụ cũng không có hạn; sửa hạn sau trên nhiệm vụ."},
}, retiredRoleKeys...)

// CreateTaskFromCitizenLetter books one task from a citizen letter. POST /api/v1/citizen-letter-tasks
func (h *Handler) CreateTaskFromCitizenLetter(w http.ResponseWriter, r *http.Request) {
	var in citizenLetterTaskIn
	if !decodeRefusingKeys(w, r, &in, citizenLetterTaskRefusedKeys) {
		return
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheNhiemVu(w, r)
		return
	}
	// REFUSED BEFORE documents IS ASKED: a malformed id costs no round trip and opens nothing.
	if err := domain.CheckCitizenLetterID(in.LetterID); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Thiếu `letter_id` hoặc mã đơn thư không hợp lệ. Hãy chọn đơn trên sổ đơn thư.", "")
		return
	}
	documents, err := vanBanVaoTrong(in.Documents)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
		return
	}

	yc := app.YeuCauTaoNhiemVu{
		Ma:                in.Code,
		TuSinhMa:          in.AutoCode,
		Loai:              in.Type,
		Khoi:              in.Bloc,
		TieuDe:            in.Title,
		MoTa:              in.Body,
		MucUuTien:         in.Priority,
		GhiChu:            in.Note,
		BoPhanID:          in.Unit,
		NguoiThucHienMa:   in.Assignee,
		LanhDaoGiaoViecMa: in.Assigner,
		ParentCode:        in.Parent,
		VanBan:            documents,
		// NguonGiao, NguonID AND HanXuLy ARE NOT SET HERE — the use case sets all three from documents.
	}

	// The commune is the context's (tenant middleware, from Host); documents is asked in it (metadata),
	// and the task is written through the scoped store. Nothing here names a commune.
	n, err := h.d.CitizenLetterTasks.CreateTask(r.Context(), in.LetterID, yc, actor)
	if err != nil {
		h.writeCitizenLetterTaskError(w, r, err)
		return
	}
	// A retry with the same Idempotency-Key is told the task's REGISTER NUMBER (transaction-boundaries
	// `tao_nhiem_vu_tu_don_thu`: "trả lại đúng nhiệm vụ đã tạo"), not left with a bare 409 while its task
	// sits in the register. The code, never the body (core/idem.RecordCode).
	idem.RecordCode(r.Context(), n.Ma)
	vietJSON(w, http.StatusCreated, nhiemVuRaNgoai(n))
}

// writeCitizenLetterTaskError maps the refusals only this door raises, then hands every other one to the
// task register's own mapping — so one act answers one way whichever door it came through. Fixed
// sentences, never err.Error(): the chain may carry the commune id and documents' status text.
func (h *Handler) writeCitizenLetterTaskError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, app.ErrCitizenLetterNotFound):
		// ONE 404 for unknown, soft-deleted and another commune's letter (rule 1) — documents answered so.
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy đơn thư.", "")
	case errors.Is(err, app.ErrCitizenLetterDenunciation):
		// C9, ADR 0084 #6. Nothing about the letter is echoed or logged.
		httpx.WriteError(w, http.StatusUnprocessableEntity, "denunciation_no_task",
			"Đơn tố cáo không chuyển thành nhiệm vụ để giữ bí mật người tố cáo.", "")
	case errors.Is(err, domain.ErrCitizenLetterAssignmentRequired):
		httpx.WriteError(w, http.StatusUnprocessableEntity, "assignment_required",
			domain.CitizenLetterAssignmentRequiredSentence, "")
	case errors.Is(err, domain.ErrCitizenLetterIDInvalid):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Thiếu `letter_id` hoặc mã đơn thư không hợp lệ. Hãy chọn đơn trên sổ đơn thư.", "")
	case errors.Is(err, app.ErrCitizenLetterUnchecked):
		// 503: documents was not asked, or its answer was refused. Nothing was written.
		h.d.Log.Warn("CẢNH BÁO: từ chối chuyển đơn thư thành nhiệm vụ vì chưa hỏi được sổ đơn thư",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "citizen_letter_check_unavailable",
			"Chưa kiểm tra được đơn thư nên nhiệm vụ CHƯA được tạo. Vui lòng thử lại sau ít phút.", "")
	case errors.Is(err, app.ErrAssignmentStaffInvalid):
		// One sentence for unknown, locked, another commune — the code is not echoed (rule 1).
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Cán bộ thực hiện không nhận được việc. Hãy chọn người khác trong danh sách.", "")
	case errors.Is(err, app.ErrAssignmentStaffUnchecked):
		h.d.Log.Warn("CẢNH BÁO: từ chối chuyển đơn thư thành nhiệm vụ vì chưa kiểm được cán bộ nhận việc",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "assignee_check_unavailable",
			"Chưa kiểm tra được cán bộ nhận việc nên nhiệm vụ CHƯA được tạo. Vui lòng thử lại sau ít phút.", "")
	default:
		h.traLoiLoiNhiemVu(w, r, "chuyển đơn thư thành nhiệm vụ", err)
	}
}
