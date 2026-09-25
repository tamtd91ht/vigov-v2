package http

// The STAFF WRITE surface of the task register (docs/ui-ux/02-nhiem-vu.md §5, §6, §7).
//
//	POST   /api/v1/tasks                                task.create
//	PATCH  /api/v1/tasks/{ma}                           task.update
//	POST   /api/v1/tasks/{ma}/status                    task.update   + task.approve for `hoan-thanh`
//	DELETE /api/v1/tasks/{ma}                           task.delete
//	POST   /api/v1/tasks/{ma}/extensions                task.update
//	POST   /api/v1/tasks/{ma}/extensions/{id}/decision  task.extend   + ADR 0038's second layer
//
// # EVERY KEY ALREADY EXISTS IN `quyen`, AND ALL SIX WERE CHECKED AGAINST THE TABLE FIRST
//
// service-identity/migrations/0001_init.sql seeds `task.approve`:299, `task.assign`:300,
// `task.create`:301, `task.delete`:302, `task.extend`:303, `task.read`:304 and `task.update`:305 —
// the full list §6 names. NO KEY WAS INVENTED (rule 5, invariant 3c): a key no migration seeds is a
// right no administrator can grant, so the route would answer 403 to every account for ever while
// the tests stayed green, because a fake checker grants any string. `tools/check_quyen.py` scans the
// whole repository against the table on every `make check`.
//
// `task.assign` IS SEEDED AND IS USED BY NO ROUTE HERE, and that is this pass's boundary rather than
// an oversight: §10's `giao-viec` route is not built, and PATCH deliberately cannot move
// `bo_phan_id` or `nguoi_thuc_hien_ma` — folding assignment into the edit route would hand it to
// every holder of `task.update`. Reported as a finding.
//
// # TWO ROUTES CONSULT A SECOND KEY INSIDE THE HANDLER, AND NO STATUS CODE SHOWS IT
//
//	…/status  with `status = hoan-thanh`   ALSO needs `task.approve` (§6: "Duyệt hoàn thành")
//	…/decision                             ALSO needs to BE the leader named on the record (ADR 0038)
//
// Both are read here as a FACT and decided in internal/app, inside the transaction, on the row read
// under the lock. Deciding either here would mean deciding against a row this layer never locked.
//
// # THE URL NOUNS, AND WHERE EACH COMES FROM
//
//	tasks        SETTLED — kb/00-foundation/ubiquitous-language.md:144 maps `nhiem_vu` to `tasks`.
//	status       a noun, and the state of the resource. The same segment the petition path already
//	             serves, so two registers of one service do not spell one concept two ways.
//	extensions   } NOT in that table, and NOT translated on the spot. `de_nghi_lui_han` has no row
//	decision     } there, and ADR 0011 says ASK rather than translate — so both are raised as a
//	             FINDING for the owner of that table, exactly as `classification` and `assignment`
//	             were for the petition path. Changing them is free today and stops being free the
//	             day a commune runs.

import (
	"errors"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// QuyenDuyetHoanThanhNhiemVu is the key §6 names for the step that declares work finished.
//
// It is a constant because it is used with Checker.Allows and not inside authz.RequirePermission:
// tools/apidoc reads the ROUTE declarations and refuses anything there that is not a string
// literal, which is why routes.go spells its keys out and this does not.
const QuyenDuyetHoanThanhNhiemVu authz.Perm = "task.approve"

// coQuyenDuyetHoanThanh answers the ONE question the status use case asks about the caller.
//
// IT ANSWERS AND DECIDES NOTHING. Which moves that fact refuses is app.duocHoanThanh's, inside the
// transaction — the rule is about the TASK's sub-tree as well as the account, and the tree is a
// property of rows this layer has not read and cannot lock.
//
// FAIL CLOSED: no principal in the context means `false`, never `true`. There is always one behind
// authz.RequirePermission, so this is a precondition rather than a case — but the safe value of a
// permission fact is the one that grants nothing.
func (h *Handler) coQuyenDuyetHoanThanh(r *http.Request) app.QuyenDuyetHoanThanh {
	principal, ok := authz.From(r.Context())
	if !ok {
		return false
	}
	return app.QuyenDuyetHoanThanh(h.d.Checker.Allows(r.Context(), principal,
		QuyenDuyetHoanThanhNhiemVu))
}

// --- request bodies -----------------------------------------------------------------------------

// taoNhiemVuVao is the body of POST /api/v1/tasks — §7's "Giao việc mới".
//
// THE FIELD NAMES SAY WHAT THE DATA IS, NOT WHAT THE SCREEN CALLS IT (ADR 0017), and they are the
// same names GET /api/v1/tasks/{ma} already returns: `assigner` is the leader who handed the work
// out, which the drawer labels "Lãnh đạo giao việc".
//
// # THE ABSENCES ARE REFUSALS RATHER THAN OMISSIONS
//
// There is no `status`: a new task starts at `moi-giao` and nowhere else. There is no
// `original_due_at`: it is written from `due_at` by the INSERT, once, and the trigger refuses every
// later change. There is no `created_by`: that is the session's own principal, and a request that
// could name its author is a request that can forge the trail.
type taoNhiemVuVao struct {
	// Code is §7.1's "Mã nhiệm vụ", used only when AutoCode is false. AutoCode IS THE CHECKBOX and
	// the form ships with it ON, so the ordinary request carries neither.
	Code     string `json:"code,omitempty"`
	AutoCode bool   `json:"auto_code"`

	Type     string `json:"type"`
	Bloc     string `json:"bloc,omitempty"`
	Title    string `json:"title"`
	Body     string `json:"description,omitempty"`
	Priority string `json:"priority,omitempty"`

	Source   string `json:"source,omitempty"`
	SourceID string `json:"source_id,omitempty"`

	Unit     string `json:"unit,omitempty"`
	Assignee string `json:"assignee,omitempty"`
	Assigner string `json:"assigner,omitempty"`
	LeadUnit string `json:"lead_unit,omitempty"`
	Monitor  string `json:"monitor,omitempty"`

	// DueAt is "Hạn hoàn thành". A POINTER, so "no deadline" is expressible: §7.1 does not mark the
	// field required, and §4.1 renders such a task as `Hạn —`.
	//
	// ⚠ IT IS THE ONLY MOMENT THIS VALUE CAN EVER BE SET. `han_ban_dau` takes the same instant and
	// migration 0006's trigger refuses every later change to it, so a task created without a
	// deadline can never be given one. That consequence is reported rather than worked around.
	DueAt *time.Time `json:"due_at,omitempty"`

	// Parent makes this a sub-task (§5.10). It carries the parent's INTERNAL id.
	//
	// ⚠ AND NO CLIENT CAN SUPPLY IT TODAY. This comment used to end "…which is what the drawer
	// already holds for the task it is open on"; that was measured false on 23/09/2026. `nhiemVuRa`
	// (`nhiem_vu.go:73-165`) emits `code` and `parent` and NEVER its own `id`, so a drawer open on
	// NV19 knows NV19's parent and never NV19 itself. §5.10 "Thêm việc con" therefore cannot form a
	// request at all, and the admin web reports that on screen rather than drawing a dead button.
	//
	// Two ways out, and neither may be picked here: emit `id` on `nhiemVuRa`, or let `parent` take
	// the business code — the same shape `loai` already uses, which the `loai_nhiem_vu` foreign key
	// settles in favour of codes. It changes a published contract, so it is the owner's call.
	Parent string `json:"parent,omitempty"`

	// Documents is §7.2's three dynamic lists. OPTIONAL, and its absence is the ordinary request:
	// §7.3 removes the whole block for a `co-ban` task.
	//
	// ⚠ IT IS OPTIONAL BECAUSE IT HAS TO BE (rule 2, invariant 4). `taoNhiemVuVao` is a PUBLISHED
	// contract with a real client behind it (`web-admin/src/lib/api/nhiem-vu.ts`), and a required
	// field added to a published shape breaks every caller that was already correct. A client that
	// sends nothing here gets a task with an empty block, exactly as before this pass.
	Documents []vanBanNhiemVuVao `json:"documents,omitempty"`
}

// vanBanNhiemVuVao is ONE line of the block on the way in — §7.2's `+ Thêm văn bản` and, on PATCH,
// also the lines that are being kept.
//
// # THERE IS NO `position` FIELD, AND THAT IS THE RULE RATHER THAN AN OVERSIGHT
//
// A line's position is MINTED BY THE REGISTER (max + 1 within its group, counting removed lines) and
// never sent by a client. A request that could carry it could place a line at a number another line
// already holds — and on this table that is a constraint error inside the business transaction,
// which rolls the whole edit back. Migration 0009 spends a block on why this is the expensive one.
//
// THERE IS NO `task` FIELD EITHER: the task is the `{ma}` in the path. A body that could name a
// different one is a body that can write into a record the caller never opened.
type vanBanNhiemVuVao struct {
	// ID names an EXISTING line. EMPTY MEANS A NEW ONE — that is the whole of `+ Thêm văn bản`.
	//
	// ⚠ ON PATCH, A LINE THE BODY DOES NOT NAME IS REMOVED. That is how `✕` reaches the server: the
	// block is sent WHOLE, and the line simply stops being in it. A client that sends only the line
	// it just added deletes the other two.
	ID string `json:"id,omitempty"`

	// Group is one of §5.4's three codes. REQUIRED on a new line; on an existing one it may be
	// omitted, and if it is sent it must MATCH the stored group — a line does not move between the
	// three boxes (domain.ErrDoiNhomVanBan).
	Group string `json:"group"`

	// Reference (`1742-CV/BTCTU`) and Date are optional, and both are absent from §7.2's form today:
	// it offers ONE textarea, whose content is `summary`. They are on the contract so a later form
	// that does collect them needs no migration and no new field.
	Reference string `json:"reference,omitempty"`

	// Date is `YYYY-MM-DD` — the day printed on the document, with no time and no zone. An RFC 3339
	// instant is REFUSED rather than truncated: a browser's zone would otherwise decide which DAY a
	// document was signed.
	Date string `json:"date,omitempty"`

	// Summary is the line's text and is MANDATORY. It is where §7.2's textarea lands.
	Summary string `json:"summary"`
}

// suaNhiemVuVao is the body of PATCH /api/v1/tasks/{ma}.
//
// PATCH AND NOT PUT, AND EVERY FIELD IS A POINTER: four of them have a meaningful zero — a cleared
// note, a progress of 0, an unticked box, a detached parent — so a full replacement could not tell
// "not mentioned" from "set to zero".
//
// WHAT IS NOT HERE IS petstore.SuaNhiemVu's list and its reasoning. The two that matter most:
// `due_at` is absent because a deadline moves through the extension flow and nowhere else, and
// `assigner` is absent because that column IS the approver under ADR 0038 — a `task.update` holder
// able to rewrite it could name themselves the approver of their own extension requests.
type suaNhiemVuVao struct {
	Bloc          *string `json:"bloc,omitempty"`
	Title         *string `json:"title,omitempty"`
	Body          *string `json:"description,omitempty"`
	Priority      *string `json:"priority,omitempty"`
	Progress      *int    `json:"progress,omitempty"`
	ResultSummary *string `json:"result_summary,omitempty"`
	Note          *string `json:"note,omitempty"`

	LeaderApproved       *bool `json:"leader_approved,omitempty"`
	SuperiorAcknowledged *bool `json:"superior_acknowledged,omitempty"`

	// Parent re-parents the task; an empty string detaches it into a root task. THIS IS THE ONE
	// FIELD THAT CAN CLOSE A CYCLE — see app.kiemChuTrinh.
	Parent *string `json:"parent,omitempty"`

	// Documents replaces §5.4's document block WHOLE. A POINTER TO A SLICE, and the double
	// indirection carries a distinction a plain slice cannot:
	//
	//	absent / null   the block is not being edited. It is left exactly as it is.
	//	[]              the block is being EMPTIED. Three `✕` clicks then Save, and it has to be
	//	                expressible — folded into "absent" it would silently keep lines a member of
	//	                staff deleted.
	//
	// ⚠ REPLACE-BY-SET, NOT APPEND. Every line that is to survive must be sent back, WITH ITS `id`.
	// §5.4 draws one `✎ Sửa` over the whole block, so one Save is one act over the whole block —
	// and "the lines you did not send are gone" is the only reading under which `✕` works at all.
	//
	// OPTIONAL, like every field of this body and for the reason `taoNhiemVuVao.Documents` states:
	// this is a published contract with a real client behind it (rule 2, invariant 4).
	Documents *[]vanBanNhiemVuVao `json:"documents,omitempty"`
}

// doiTrangThaiVao is the body of POST /api/v1/tasks/{ma}/status.
//
// THE TARGET IS ON THE WIRE HERE AND IS NOT ON THE PETITION PATH, and the difference is the shape of
// the two lifecycles: §6's task lifecycle BRANCHES at every state — `tam-dung` and `chuyen-tiep`
// leave all four working states — so there is no single "next" for the server to choose. What the
// server still owns is the MAP: a move the diagram does not draw is refused.
type doiTrangThaiVao struct {
	Status string `json:"status"`
	// Note becomes the timeline entry (§5.9). Optional: when it is empty the entry carries a
	// generated sentence naming the two statuses, because the timeline may not have a gap.
	Note string `json:"note,omitempty"`
}

// xoaNhiemVuVao is the body of DELETE /api/v1/tasks/{ma}.
//
// A BODY ON A DELETE, AND THE ALTERNATIVE WAS WORSE: the reason is mandatory (rule 7, invariant 1),
// and the query string would put free text about a government record into every access log and
// proxy cache.
type xoaNhiemVuVao struct {
	Reason string `json:"reason"`
}

// deNghiLuiHanVao is the body of POST /api/v1/tasks/{ma}/extensions — §5.8's two fields.
type deNghiLuiHanVao struct {
	NewDueAt time.Time `json:"new_due_at"`
	Reason   string    `json:"reason"`
}

// quyetDinhLuiHanVao is the body of the decision route.
//
// TWO OUTCOMES, AND THE WIRE CARRIES EXACTLY TWO. A `status` string would let a client send
// `cho-duyet` and file a decision that decides nothing, or invent a fourth code.
type quyetDinhLuiHanVao struct {
	// Decision is `approve` or `reject`. ANY OTHER VALUE IS REFUSED rather than read as a rejection:
	// a typo that silently rejected an extension would move nothing and tell nobody why.
	Decision string `json:"decision"`
	Note     string `json:"note,omitempty"`
}

const (
	quyetDinhDuyet  = "approve"
	quyetDinhTuChoi = "reject"
)

var errQuyetDinhKhongHopLe = errors.New(
	"`decision` chỉ nhận `approve` hoặc `reject`")

// deNghiLuiHanRa is one extension request as it leaves the API.
//
// THE REASON TEXT IS ON THE WIRE and the audit entry holds only its length — those are two different
// stores with two different lifetimes. The screen needs the sentence to show the leader what is
// being asked; `audit_log` is append-only and never deleted, so a copy there would be permanent.
type deNghiLuiHanRa struct {
	ID string `json:"id"`

	// RequestedBy and DecidedBy are STAFF BUSINESS CODES (`CB-00123`), never internal ids (rule 6,
	// invariant 8). DecidedBy is empty until somebody decides.
	RequestedBy string `json:"requested_by"`
	DecidedBy   string `json:"decided_by,omitempty"`

	NewDueAt time.Time `json:"new_due_at"`
	Reason   string    `json:"reason"`
	Status   string    `json:"status"`

	RequestedAt time.Time  `json:"requested_at"`
	DecidedAt   *time.Time `json:"decided_at"`
}

func deNghiRaNgoai(d domain.DeNghiLuiHan) deNghiLuiHanRa {
	ra := deNghiLuiHanRa{
		ID:          d.ID,
		RequestedBy: d.NguoiDeNghiMa,
		DecidedBy:   d.NguoiDuyetMa,
		NewDueAt:    d.HanMoi,
		Reason:      d.LyDo,
		Status:      string(d.TrangThai),
		RequestedAt: d.ThoiDiem,
	}
	// A ZERO time.Time BECOMES JSON null, HERE AND IN ONE PLACE. Marshalled straight it would travel
	// as `0001-01-01T00:00:00Z` — a date the screen would happily render as a decision instant.
	if !d.DuyetLuc.IsZero() {
		t := d.DuyetLuc
		ra.DecidedAt = &t
	}
	return ra
}

// --- §5.4's document block, on the way in -----------------------------------------------------------

// errNgayVanBanKhongDocDuoc — the date on a document line did not parse.
//
// THE CLIENT'S STRING IS NOT ECHOED BACK. What it typed is free text about an administrative
// document and an error message travels into centralised logging (rule 3, forbidden #3); what the
// caller can act on is the FORMAT, which the sentence names.
var errNgayVanBanKhongDocDuoc = errors.New(
	"`date` của một dòng văn bản phải theo dạng YYYY-MM-DD (ngày ghi trên văn bản, không có giờ)")

// ngayVanBanVao reads a document date off the wire. IT IS THE INVERSE OF ngayVanBanRa.
//
// AN EMPTY STRING IS A LEGAL ANSWER and becomes the zero time.Time, which the store writes as SQL
// NULL. That is the ordinary case: §7.2 offers one textarea and collects no date at all, so almost
// every line arrives without one. This is exactly where it differs from `ngayHopVao` next door,
// which refuses "" because §4 marks the meeting day required.
//
// `2006-01-02` AND NOTHING ELSE. Accepting an RFC 3339 instant here would let a browser's zone
// decide which DAY a document was signed, and `ngay_van_ban` is a `DATE` column precisely because
// that question has no answer.
func ngayVanBanVao(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(dinhDangNgay, s)
	if err != nil {
		return time.Time{}, errNgayVanBanKhongDocDuoc
	}
	return t, nil
}

// vanBanVaoTrong turns the request block into the domain's shape.
//
// IT VALIDATES NOTHING BUT THE DATE, and that asymmetry is deliberate: a date is a WIRE FORMAT
// question and can only be answered here, while "is this group one of the three", "is the text
// present" and "are there too many lines" are BUSINESS rules and belong to domain.
// KiemDanhSachVanBanNhiemVu, which the use case calls before it opens a transaction. Two layers
// checking the same thing would be two sentences for one refusal, and the one that drifts is
// whichever is edited second.
func vanBanVaoTrong(ds []vanBanNhiemVuVao) ([]domain.VanBanNhiemVuVao, error) {
	ra := make([]domain.VanBanNhiemVuVao, 0, len(ds))
	for _, v := range ds {
		ngay, err := ngayVanBanVao(v.Date)
		if err != nil {
			return nil, err
		}
		ra = append(ra, domain.VanBanNhiemVuVao{
			ID:         v.ID,
			Nhom:       domain.NhomVanBanNhiemVu(v.Group),
			SoKyHieu:   v.Reference,
			NgayVanBan: ngay,
			TrichYeu:   v.Summary,
		})
	}
	return ra, nil
}

// --- the six handlers ----------------------------------------------------------------------------

// TaoNhiemVu books one task. POST /api/v1/tasks
func (h *Handler) TaoNhiemVu(w http.ResponseWriter, r *http.Request) {
	var vao taoNhiemVuVao
	if !docThan(w, r, &vao) {
		return
	}
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheNhiemVu(w, r)
		return
	}

	// A MEETING-CONCLUSION SOURCE HAS ITS OWN DOOR, and this is not it. Only the split route
	// (POST /api/v1/meetings/{id}/conclusions/{stt}/task) checks the conclusion exists, is live and is
	// not marked "không phát sinh nhiệm vụ", under the meeting's lock; accepting the code here took
	// any `source_id` and checked none of it. Refused before the use case, so nothing is opened.
	if domain.NguonGiao(vao.Source) == domain.NguonKetLuanHop {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			domain.ErrNguonKetLuanPhaiTach.Error(), "")
		return
	}

	// REFUSED BEFORE THE USE CASE IS REACHED, so a malformed date opens no transaction at all.
	vanBan, err := vanBanVaoTrong(vao.Documents)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
		return
	}

	yc := app.YeuCauTaoNhiemVu{
		Ma:                  vao.Code,
		TuSinhMa:            vao.AutoCode,
		Loai:                vao.Type,
		Khoi:                vao.Bloc,
		TieuDe:              vao.Title,
		MoTa:                vao.Body,
		MucUuTien:           vao.Priority,
		NguonGiao:           vao.Source,
		NguonID:             vao.SourceID,
		BoPhanID:            vao.Unit,
		NguoiThucHienMa:     vao.Assignee,
		LanhDaoGiaoViecMa:   vao.Assigner,
		CoQuanChuTriID:      vao.LeadUnit,
		ChuyenVienTheoDoiMa: vao.Monitor,
		NhiemVuChaID:        vao.Parent,
		VanBan:              vanBan,
	}
	if vao.DueAt != nil {
		yc.HanXuLy = *vao.DueAt
	}

	n, err := h.d.GhiNhiemVu.Tao(r.Context(), yc, nguoi)
	if err != nil {
		h.traLoiLoiNhiemVu(w, r, "giao việc mới", err)
		return
	}
	// 201 AND THE RECORD, because the caller's next act is on the same row and the MINTED NUMBER is
	// the one thing they cannot have known before asking.
	vietJSON(w, http.StatusCreated, nhiemVuRaNgoai(n))
}

// SuaNhiemVu edits the descriptive fields. PATCH /api/v1/tasks/{ma}
func (h *Handler) SuaNhiemVu(w http.ResponseWriter, r *http.Request) {
	var vao suaNhiemVuVao
	if !docThan(w, r, &vao) {
		return
	}
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheNhiemVu(w, r)
		return
	}

	// THE POINTER IS CARRIED THROUGH, NOT THE SLICE. `nil` means the block was not mentioned and
	// must be left alone; a non-nil pointer to an EMPTY slice means the block is being emptied. A
	// conversion that returned a plain slice would collapse the two — and the one it would lose is
	// the one that deletes lines a member of staff removed.
	var vanBan *[]domain.VanBanNhiemVuVao
	if vao.Documents != nil {
		ds, err := vanBanVaoTrong(*vao.Documents)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
			return
		}
		vanBan = &ds
	}

	n, err := h.d.GhiNhiemVu.Sua(r.Context(), r.PathValue("ma"), petstore.SuaNhiemVu{
		Khoi:                     vao.Bloc,
		TieuDe:                   vao.Title,
		MoTa:                     vao.Body,
		MucUuTien:                vao.Priority,
		TienDo:                   vao.Progress,
		TomTatKetQua:             vao.ResultSummary,
		GhiChu:                   vao.Note,
		LanhDaoPheDuyetHoanThanh: vao.LeaderApproved,
		CapTrenCongNhanHoanThanh: vao.SuperiorAcknowledged,
		NhiemVuChaID:             vao.Parent,
		VanBan:                   vanBan,
	}, nguoi)
	if err != nil {
		h.traLoiLoiNhiemVu(w, r, "sửa nhiệm vụ", err)
		return
	}
	vietJSON(w, http.StatusOK, nhiemVuRaNgoai(n))
}

// DoiTrangThaiNhiemVu moves the task along §6. POST /api/v1/tasks/{ma}/status
//
// THIS HANDLER ANSWERS ONE EXTRA QUESTION AND DECIDES NOTHING. The route's gate already refused
// anyone without `task.update`. What is read here is whether the caller ALSO holds `task.approve`,
// and that single fact is handed down; whether the move is permitted — the permission AND the whole
// sub-tree being finished — is app.duocHoanThanh's, inside the transaction.
func (h *Handler) DoiTrangThaiNhiemVu(w http.ResponseWriter, r *http.Request) {
	var vao doiTrangThaiVao
	if !docThan(w, r, &vao) {
		return
	}
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheNhiemVu(w, r)
		return
	}

	n, err := h.d.GhiNhiemVu.DoiTrangThai(r.Context(), r.PathValue("ma"),
		app.YeuCauDoiTrangThai{TrangThai: vao.Status, GhiChu: vao.Note},
		nguoi, h.coQuyenDuyetHoanThanh(r))
	if err != nil {
		h.traLoiLoiNhiemVu(w, r, "chuyển trạng thái", err)
		return
	}
	vietJSON(w, http.StatusOK, nhiemVuRaNgoai(n))
}

// XoaNhiemVu soft-deletes the task. DELETE /api/v1/tasks/{ma}
//
// THE METHOD IS THE ONLY THING THAT SAYS OTHERWISE. The row stays, carrying `deleted_at`,
// `deleted_by` and `delete_reason` (rule 7, invariant 1), and its number stays taken for ever.
// DELETE is still the right method: the resource is gone from every read path, which is what the
// caller is asking for.
func (h *Handler) XoaNhiemVu(w http.ResponseWriter, r *http.Request) {
	var vao xoaNhiemVuVao
	if !docThan(w, r, &vao) {
		return
	}
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheNhiemVu(w, r)
		return
	}

	if err := h.d.GhiNhiemVu.Xoa(r.Context(), r.PathValue("ma"), vao.Reason, nguoi); err != nil {
		h.traLoiLoiNhiemVu(w, r, "xoá nhiệm vụ", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DeNghiLuiHanNhiemVu files an extension request. POST /api/v1/tasks/{ma}/extensions
func (h *Handler) DeNghiLuiHanNhiemVu(w http.ResponseWriter, r *http.Request) {
	var vao deNghiLuiHanVao
	if !docThan(w, r, &vao) {
		return
	}
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheNhiemVu(w, r)
		return
	}

	dn, err := h.d.GhiNhiemVu.DeNghiLuiHan(r.Context(), r.PathValue("ma"),
		app.YeuCauDeNghiLuiHan{HanMoi: vao.NewDueAt, LyDo: vao.Reason}, nguoi)
	if err != nil {
		h.traLoiLoiNhiemVu(w, r, "đề nghị lùi hạn", err)
		return
	}
	vietJSON(w, http.StatusCreated, deNghiRaNgoai(dn))
}

// QuyetDinhLuiHanNhiemVu answers a request.
// POST /api/v1/tasks/{ma}/extensions/{deNghiID}/decision
//
// ⚠ THE ROUTE'S KEY IS NOT THE WHOLE ANSWER, AND THAT IS ADR 0038. `task.extend` says this account
// may touch extensions at all; whether it may decide THIS one is a question about the record — is
// this the leader named on it — and rule 5 checks `(tenant_id, role, permission)` with no "which
// record" dimension. The second layer lives in domain.DuocDuyetLuiHan and runs inside the
// transaction, on the task row read under the lock.
func (h *Handler) QuyetDinhLuiHanNhiemVu(w http.ResponseWriter, r *http.Request) {
	var vao quyetDinhLuiHanVao
	if !docThan(w, r, &vao) {
		return
	}
	// REFUSED RATHER THAN READ AS A REJECTION. A typo that silently rejected an extension would move
	// nothing, tell nobody why, and leave the officer waiting on a request that was answered.
	if vao.Decision != quyetDinhDuyet && vao.Decision != quyetDinhTuChoi {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			errQuyetDinhKhongHopLe.Error(), "")
		return
	}
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheNhiemVu(w, r)
		return
	}

	dn, err := h.d.GhiNhiemVu.QuyetDinhLuiHan(r.Context(), r.PathValue("ma"),
		r.PathValue("deNghiID"),
		app.YeuCauQuyetDinhLuiHan{Duyet: vao.Decision == quyetDinhDuyet, GhiChu: vao.Note}, nguoi)
	if err != nil {
		h.traLoiLoiNhiemVu(w, r, "quyết định lùi hạn", err)
		return
	}
	vietJSON(w, http.StatusOK, deNghiRaNgoai(dn))
}

// --- shared ---------------------------------------------------------------------------------------

// traLoiLoiNhiemVu maps one use-case failure onto a status and a sentence.
//
// ONE FUNCTION FOR ALL SIX ROUTES, because six copies of this mapping would drift and the copy that
// drifts is the one answering 500 where it meant 409 — which reads to an operator as a broken server
// rather than as a rule doing its job.
//
// # THE 403 / 409 LINE IS THE ONE WORTH READING TWICE
//
//	403  the refusal is about WHO IS ACTING. A 409 here would tell an officer to reload a task they
//	     were never allowed to move, hiding a permission problem behind a sentence about state.
//	409  the caller HOLDS the right and is allowed to perform the act; what is refused is this act on
//	     THIS record, because of the state it is in. A 403 would send them to the Phân quyền screen
//	     to be granted a right they already have.
//
// ⚠ THE LAST ARGUMENT OF httpx.WriteError IS THE TRACE ID, NOT A FIELD NAME. Every call below passes
// "" for it. That is worth saying because the petition path next door passes field names there
// (internal/http/xu_ly_phan_anh.go, `"unit"` and `"field"`), which lands them in `trace_id` on the
// wire — httpx.Error has three members and none of them names a field. Copying that shape here would
// have doubled a defect rather than reported it; it is raised as a finding instead.
func (h *Handler) traLoiLoiNhiemVu(w http.ResponseWriter, r *http.Request, viec string, err error) {
	switch {
	case errors.Is(err, petstore.ErrNhiemVuKhongTonTai):
		// THE SAME ANSWER AS AN UNKNOWN NUMBER, ANOTHER COMMUNE'S NUMBER AND A SOFT-DELETED TASK.
		// Telling them apart tells a caller which numbers exist in a register they are not reading.
		h.khongTimThayNhiemVu(w)
	case errors.Is(err, petstore.ErrDeNghiKhongTonTai):
		httpx.WriteError(w, http.StatusNotFound, "not_found",
			"Không tìm thấy đề nghị lùi hạn này trên nhiệm vụ.", "")

	// --- 403: about the person ------------------------------------------------------------------
	case errors.Is(err, app.ErrKhongDuocDuyetHoanThanh):
		httpx.WriteError(w, http.StatusForbidden, "forbidden",
			"Hoàn thành nhiệm vụ cần quyền duyệt hoàn thành. Tài khoản của bạn mới có quyền "+
				"cập nhật tiến độ.", "")
	case domain.LaLoiThamQuyenLuiHan(err):
		// ADR 0038's two layers, plus the open question failing CLOSED. The sentence is the domain's
		// own: it names what is missing — the leader on the record — which is the only thing the
		// commune can act on.
		httpx.WriteError(w, http.StatusForbidden, "forbidden",
			cauTuChoi(err, domain.LoiThamQuyenLuiHanGoc(err)), "")

	// --- 409: about the record ------------------------------------------------------------------
	//
	// THE DOMAIN'S OWN SENTENCE (cauTuChoi), NEVER err.Error(): a refusal raised inside the
	// transaction arrives wrapped by app.bocNhiemVu with the operation AND THE COMMUNE ID, which is
	// the operator's detail — the same leak the minutes register closed (traLoiLoiBienBan).
	case errors.Is(err, domain.ErrConChuaXoa), errors.Is(err, domain.ErrConChuaXong):
		// ADR 0037 decisions 3 and 4. The sentence carries the count and the register numbers,
		// because "you may not" without "what is in the way" sends an officer hunting.
		httpx.WriteError(w, http.StatusConflict, "task_tree",
			cauTuChoi(err, domain.ErrConChuaXoa, domain.ErrConChuaXong), "")
	case errors.Is(err, domain.ErrChuTrinhCayNhiemVu), errors.Is(err, domain.ErrChaKhongTonTai),
		errors.Is(err, domain.ErrCayNhiemVuQuaLon):
		httpx.WriteError(w, http.StatusConflict, "task_tree", cauTuChoi(err,
			domain.ErrChuTrinhCayNhiemVu, domain.ErrChaKhongTonTai, domain.ErrCayNhiemVuQuaLon), "")
	case errors.Is(err, petstore.ErrMaNhiemVuDaTonTai):
		httpx.WriteError(w, http.StatusConflict, "code_taken",
			"Mã nhiệm vụ này đã được dùng trong xã — kể cả khi nhiệm vụ mang mã đó đã bị xoá. "+
				"Mã đã cấp thì không cấp lại.", "")
	case errors.Is(err, petstore.ErrNhiemVuDaChuyenTrang):
		httpx.WriteError(w, http.StatusConflict, "task_state",
			"Nhiệm vụ đã thay đổi trong lúc bạn đang mở màn hình. Hãy tải lại rồi thao tác lại.", "")
	case errors.Is(err, domain.ErrChuyenTrangThaiNhiemVuSaiLuc),
		errors.Is(err, domain.ErrTiepTucKhongBietTrangThaiTruoc):
		httpx.WriteError(w, http.StatusConflict, "task_state", cauTuChoi(err,
			domain.ErrChuyenTrangThaiNhiemVuSaiLuc, domain.ErrTiepTucKhongBietTrangThaiTruoc), "")
	case errors.Is(err, domain.ErrDaCoDeNghiChoDuyet), errors.Is(err, domain.ErrDeNghiDaQuyetDinh),
		errors.Is(err, domain.ErrNhiemVuChuaCoHan):
		httpx.WriteError(w, http.StatusConflict, "task_state", cauTuChoi(err,
			domain.ErrDaCoDeNghiChoDuyet, domain.ErrDeNghiDaQuyetDinh, domain.ErrNhiemVuChuaCoHan), "")
	case errors.Is(err, domain.ErrVanBanKhongThuocNhiemVu), errors.Is(err, domain.ErrDoiNhomVanBan):
		// 409 AND NOT 400, AND THE LINE IS THE ONE DRAWN ABOVE. The caller holds the right and the
		// body is well formed; what is refused is this act on THIS record — the line is not on the
		// task any more, or it sits in a group the request disagrees with. Both are states an
		// officer fixes by reloading the drawer, which is what the sentences say.
		httpx.WriteError(w, http.StatusConflict, "task_document",
			cauTuChoi(err, domain.ErrVanBanKhongThuocNhiemVu, domain.ErrDoiNhomVanBan), "")

	// --- 400: about what was sent, document block --------------------------------------------------
	case domain.LaLoiDauVaoVanBanNhiemVu(err):
		// The domain's own sentence: it names the field and the rule, holds no personal data and no
		// internal detail. A second sentence written here would drift from it.
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			cauTuChoi(err, domain.LoiDauVaoVanBanNhiemVuGoc(err)), "")

	// --- 400: about what was sent ----------------------------------------------------------------
	case domain.LaLoiDauVaoNhiemVu(err):
		// The domain's own sentence is returned: it names the field and the rule, holds no personal
		// data and no internal detail, and a second sentence written here would drift from it. Some of
		// these are raised inside the transaction (ErrHanMoiKhongLui, ErrTrangThaiNhiemVuKhongBiet)
		// and arrive wrapped with the commune — hence cauTuChoi, not err.Error().
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			cauTuChoi(err, domain.LoiDauVaoNhiemVuGoc(err)), "")

	default:
		// The wrapped error carries the store failure and never reaches the client (rule 3,
		// forbidden #3). The commune is logged because it is the only thing an operator can act on.
		h.d.Log.Error("ghi nhiệm vụ: "+viec+" lỗi hệ thống",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}

// cauTuChoi returns the sentence a domain refusal carries ON ITS OWN: the detailed refusal built on
// the first matching sentinel (domain.LoiKemChiTiet — the count, the register numbers), or that
// sentinel's sentence. Never err.Error(), which carries app.bocNhiemVu's operation and commune id.
//
// A NIL OR UNMATCHED LIST ANSWERS THE GENERIC SENTENCE rather than falling back to err.Error(): the
// callers only reach here after errors.Is matched, so this branch is a mapping bug, and a mapping
// bug must not be the one path that puts the commune id back on the wire.
func cauTuChoi(err error, cacGoc ...error) string {
	for _, goc := range cacGoc {
		if goc == nil || !errors.Is(err, goc) {
			continue
		}
		var chiTiet *domain.LoiKemChiTiet
		if errors.As(err, &chiTiet) && errors.Is(chiTiet, goc) {
			return chiTiet.Error()
		}
		return goc.Error()
	}
	return "Yêu cầu bị từ chối. Vui lòng tải lại rồi thao tác lại."
}

// thieuChuTheNhiemVu answers a request that reached a guarded write route with no principal, or with
// one carrying no business code.
//
// A 500 AND NOT AN ANONYMOUS WRITE. These routes sit behind authz.RequirePermission, so there is
// always a principal; arriving here without one means the route was mounted wrong, or identity is
// older than the `ma` field. Rule 6 does not permit a business write whose trail cannot name who
// made it, and a fallback to the internal id would put two kinds of identifier into
// `audit_log.actor_id` one deployment window at a time, with every test green (rule 6, invariant 8 —
// measured on 2026-09-22).
func (h *Handler) thieuChuTheNhiemVu(w http.ResponseWriter, r *http.Request) {
	h.d.Log.Error("tuyến ghi nhiệm vụ chạy mà không có chủ thể — SAI CẤU HÌNH ROUTE",
		"xa", string(tenant.MustFrom(r.Context())), "duong", r.URL.Path)
	httpx.WriteError(w, http.StatusInternalServerError, "internal",
		"Đã xảy ra lỗi. Vui lòng thử lại.", "")
}
