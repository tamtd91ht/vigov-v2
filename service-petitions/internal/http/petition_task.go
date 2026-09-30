package http

// POST /api/v1/citizen-reports/{maTraCuu}/tasks — "Tạo nhiệm vụ" on the petition drawer (user decision
// 30/09/2026; docs/ui-ux/09 §13, 02 §57 "Từ phản ánh"). The act is app.PetitionTaskCreation.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// PetitionTaskCreator books a task whose source is the petition in the path. *app.PetitionTaskCreation
// satisfies it. A field of its own on Deps, for the reason every write pair there gives: it opens a
// transaction and audits inside it.
type PetitionTaskCreator interface {
	CreateTask(ctx context.Context, code string, yc app.YeuCauTaoNhiemVu, actor audit.Actor,
		restricted app.QuyenXemHanChe) (domain.NhiemVu, error)
}

// petitionTaskIn is the body — the "Giao việc mới" form (02 §7) pre-filled from the petition and
// confirmed by a person: exactly taoNhiemVuVao's fields MINUS `source` and `source_id`.
//
// # THE SOURCE IS NOT ON THE WIRE, AND ONE THAT IS SENT IS REFUSED
//
// The server sets `nguon_giao = phan-anh` and `nguon_id` from the petition named in the PATH. A body
// naming its own source could point the task at another petition — another commune's, or a `can-bo`
// report the caller cannot see (commit 2d34eba4). The meeting split simply leaves the pair off its
// body; this route goes one step further and answers 400 when a client sends either key, the refusal
// POST /api/v1/tasks gives for a client-owned source: a client that believes it chose the source must
// be told it did not, rather than have its value silently dropped.
//
// `lead_unit` AND `monitor` ARE REFUSED THE SAME WAY (ADR 0065 NV5, retiredRoleKeys): one role now.
type petitionTaskIn struct {
	Code     string `json:"code,omitempty"`
	AutoCode bool   `json:"auto_code"`

	Type     string `json:"type"`
	Bloc     string `json:"bloc,omitempty"`
	Title    string `json:"title"`
	Body     string `json:"description,omitempty"`
	Priority string `json:"priority,omitempty"`
	Note     string `json:"note,omitempty"`

	Unit     string `json:"unit,omitempty"`
	Assignee string `json:"assignee,omitempty"`
	Assigner string `json:"assigner,omitempty"`

	// DueAt is "Hạn hoàn thành" — a pointer, so "no deadline" stays expressible, as on POST /api/v1/tasks.
	// The petition's own deadlines are NOT copied into it: a task's deadline is what the leader typed.
	DueAt *time.Time `json:"due_at,omitempty"`

	// Parent: the parent task's REGISTER NUMBER (`NV19`), as on POST /api/v1/tasks.
	Parent string `json:"parent,omitempty"`

	Documents []vanBanNhiemVuVao `json:"documents,omitempty"`
}

// sourceKeysRefused are the two body keys this route refuses by NAME, whatever their value.
var sourceKeysRefused = []string{"source", "source_id"}

// CreateTaskFromPetition books one task from a petition. POST /api/v1/citizen-reports/{maTraCuu}/tasks
//
// IT ANSWERS ONE QUESTION AND DECIDES NOTHING: whether the caller holds `feedback.restricted`. Whether
// this petition may give rise to a task is app.PetitionTaskCreation's, on the row read FOR UPDATE.
func (h *Handler) CreateTaskFromPetition(w http.ResponseWriter, r *http.Request) {
	var raw json.RawMessage
	if !docThan(w, r, &raw) {
		return
	}
	var keys map[string]json.RawMessage
	var in petitionTaskIn
	if json.Unmarshal(raw, &keys) != nil || json.Unmarshal(raw, &in) != nil {
		// The decoder's message quotes the input; it never reaches the client (docThan's reason).
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Nội dung gửi lên không phải JSON hợp lệ hoặc quá lớn.", "")
		return
	}
	for _, k := range sourceKeysRefused {
		if _, sent := keys[k]; sent {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
				"Nhiệm vụ tạo từ phiếu phản ánh luôn lấy nguồn là chính phiếu trên đường dẫn — "+
					"không gửi `source` hay `source_id`.", "")
			return
		}
	}
	if sentence := refusedKeySent(keys, retiredRoleKeys); sentence != "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", sentence, "")
		return
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheNhiemVu(w, r)
		return
	}
	// REFUSED BEFORE THE USE CASE, so a malformed document date opens no transaction.
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
		// NguonGiao AND NguonID ARE NOT SET HERE — the use case sets both from the petition it reads.
	}
	if in.DueAt != nil {
		yc.HanXuLy = *in.DueAt
	}

	// The commune is the context's (tenant middleware, from Host); the use case reads through the
	// scoped store (rule 1, invariant 5), and nothing here names a commune.
	ctx := r.Context()
	n, err := h.d.PetitionTasks.CreateTask(ctx, r.PathValue("maTraCuu"), yc, actor, h.coQuyenHanChe(ctx))
	if err != nil {
		switch {
		case errors.Is(err, petstore.ErrPhieuKhongTonTai), errors.Is(err, app.ErrPhieuHanChe):
			// ONE 404 for unknown, another commune's, soft-deleted and `can-bo` without the key — the
			// read route's answer (Handler.khongTimThay).
			h.khongTimThay(w)
		// THE THREE 409s: the caller holds both keys; what is refused is this act on THIS petition. Fixed
		// sentences, never err.Error() — it arrives wrapped with the commune id (app.bocNhiemVu). Three
		// NAMED codes, because each asks the officer for a different next step.
		case errors.Is(err, domain.ErrRestrictedFieldNoTask):
			// Reached only by a `feedback.restricted` holder; without the key it is the 404 above.
			httpx.WriteError(w, http.StatusConflict, "restricted_field_no_task",
				"Phiếu thuộc lĩnh vực phản ánh về cán bộ không tạo nhiệm vụ. "+
					"Lãnh đạo xử lý trực tiếp trên phiếu phản ánh.", "")
		case errors.Is(err, domain.ErrPetitionNotClassifiedForTask):
			httpx.WriteError(w, http.StatusConflict, "petition_not_classified",
				"Phiếu chưa được phân loại và chuyển xử lý nên chưa tạo nhiệm vụ được. "+
					"Hãy phân loại và chuyển xử lý phiếu trước.", "")
		case errors.Is(err, domain.ErrPetitionClosedForTask):
			httpx.WriteError(w, http.StatusConflict, "petition_state",
				"Phiếu đã đóng hoặc đã kết thúc nên không tạo nhiệm vụ mới từ phiếu này được.", "")
		default:
			// Every other refusal is the TASK register's, mapped by its own function, so one act answers
			// one way whichever door it came through.
			h.traLoiLoiNhiemVu(w, r, "tạo nhiệm vụ từ phiếu phản ánh", err)
		}
		return
	}
	vietJSON(w, http.StatusCreated, nhiemVuRaNgoai(n))
}
