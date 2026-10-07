package http

// The STAFF WRITE surface of the task register (docs/ui-ux/02-nhiem-vu.md §5, §6, §7).
//
//	POST   /api/v1/tasks                                task.create
//	PATCH  /api/v1/tasks/{ma}                           task.update
//	POST   /api/v1/tasks/{ma}/status                    task.update   + task.approve for `hoan-thanh`
//	                                                                  and for the return to `dang-thuc-hien`
//	DELETE /api/v1/tasks/{ma}                           task.delete
//	POST   /api/v1/tasks/{ma}/extensions                task.update
//	POST   /api/v1/tasks/{ma}/extensions/{id}/decision  task.extend   + ADR 0038's second layer
//	POST   /api/v1/tasks/{ma}/assignment                task.assign   (task_assignment.go)
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
// `task.assign` (0001_init.sql:307) GUARDS ITS OWN ROUTE SINCE 28/09/2026 — …/assignment, in
// task_assignment.go. PATCH still deliberately cannot move `bo_phan_id` or `nguoi_thuc_hien_ma`:
// folding assignment into the edit route would hand it to every holder of `task.update`.
//
// # TWO ROUTES CONSULT A SECOND KEY INSIDE THE HANDLER, AND NO STATUS CODE SHOWS IT
//
//	…/status  with `status = hoan-thanh`   ALSO needs `task.approve` (§6: "Duyệt hoàn thành")
//	…/status  `cho-duyet` → `dang-thuc-hien` ALSO needs `task.approve` and a `note` (the reason)
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
	"encoding/json"
	"errors"
	"net/http"
	"strings"
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

	// Note is the same `ghi_chu` column PATCH's `note` edits, bounded by the same rule
	// (app.chuanHoaTaoNhiemVu reuses PATCH's check). OMITEMPTY IS LOAD-BEARING: tools/apidoc reads a
	// field without it as REQUIRED, and a required field added to this published contract would break
	// every existing caller (rule 2, invariant 4). Absent and "" both mean "no note".
	Note string `json:"note,omitempty"`

	Source   string `json:"source,omitempty"`
	SourceID string `json:"source_id,omitempty"`

	// Unit and Assignee are the task's ONE holder pair. There is no `lead_unit` and no `monitor`
	// (ADR 0065 NV5): the lead unit IS `unit`, the monitoring officer IS `assignee`, and a body that
	// still sends either key is refused (retiredRoleKeys).
	Unit     string `json:"unit,omitempty"`
	Assignee string `json:"assignee,omitempty"`
	Assigner string `json:"assigner,omitempty"`

	// DueAt is "Hạn hoàn thành". A POINTER, so "no deadline" is expressible: §7.1 does not mark the
	// field required, and §4.1 renders such a task as `Hạn —`.
	//
	// `han_ban_dau` takes the same instant. Since 28/09/2026 PATCH `due_at` can CORRECT it later (and
	// give a deadline to a task created without one); `han_ban_dau` follows only while no extension
	// was ever approved (migration 0016).
	DueAt *time.Time `json:"due_at,omitempty"`

	// Parent makes this a sub-task (§5.10). It carries the parent's REGISTER NUMBER (`NV19`) — the
	// `code` the drawer already holds for the task it is open on.
	//
	// A NUMBER SINCE 28/09/2026 (the owner chose this way out of the two recorded on 23/09): it used
	// to take the parent's INTERNAL id, which no response FIELD has ever carried (the opaque
	// `next_cursor` embeds one as its tie-break, but it is not a field), so §5.10 "Thêm việc con"
	// could not form a request at all. The number is resolved in THIS commune, among LIVE tasks,
	// inside the transaction (app.nhanCha); an unknown number, another commune's number and a
	// soft-deleted task all get the one 409 `task_tree` answer an invalid parent always got, so the
	// field reveals nothing about records the caller cannot see. `loai` already travels as a code
	// the same way.
	//
	// ⚠ AN INTERNAL id SENT HERE IS NOW JUST A NUMBER THAT MATCHES NOTHING — 409, never a silent
	// root task. No caller in this repository sent one (checked 28/09/2026: web-admin never sets it,
	// the Go tests set it only through the use case).
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
// WHAT IS NOT HERE IS petstore.SuaNhiemVu's list and its reasoning. `code` is NOT here since ADR 0065
// NV3 (user decision 30/09/2026): an issued task code is never edited, and a body that still sends the
// key is refused 400 rather than silently ignored (editRefusedKeys). The two that matter most:
// `due_at` IS here since 28/09/2026 as a CORRECTION (see DueAt), and `assigner` is absent because that column IS the approver under ADR 0038 — a `task.update` holder
// able to rewrite it could name themselves the approver of their own extension requests.
type suaNhiemVuVao struct {
	// DueAt CORRECTS "Hạn hoàn thành" (vigov-require 93cff7f, user decision 28/09/2026) — an RFC 3339
	// instant, stored as sent; the server adds and defaults no hour. A correction, NOT an extension:
	// no leader approves it. While the task has never had an approved extension, `original_due_at`
	// follows it; after one, only `due_at` moves. `null`/absent leaves the deadline alone — a deadline
	// cannot be cleared, and the zero instant is refused (400).
	DueAt *time.Time `json:"due_at,omitempty"`

	Bloc          *string `json:"bloc,omitempty"`
	Title         *string `json:"title,omitempty"`
	Body          *string `json:"description,omitempty"`
	Priority      *string `json:"priority,omitempty"`
	Progress      *int    `json:"progress,omitempty"`
	ResultSummary *string `json:"result_summary,omitempty"`
	Note          *string `json:"note,omitempty"`

	LeaderApproved       *bool `json:"leader_approved,omitempty"`
	SuperiorAcknowledged *bool `json:"superior_acknowledged,omitempty"`

	// Parent re-parents the task under the task carrying this REGISTER NUMBER (`NV19`); an empty
	// string detaches it into a root task. The same kind of value GET returns in `parent`, so a
	// drawer can send back exactly what it read. THIS IS THE ONE FIELD THAT CAN CLOSE A CYCLE — see
	// app.kiemChuTrinh; naming the task itself is a cycle of one and is refused the same way.
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

	// ExpectedUpdatedAt is the OPTIONAL precondition (optimistic locking, 28/09/2026): send back the
	// `updated_at` of the task as this screen read it, and the edit is refused with 409 `task_changed`
	// if anybody wrote the task since. Absent keeps today's behaviour — this is a published contract
	// (rule 2, invariant 4), so it can only be ADDED as optional.
	//
	// A BODY FIELD AND NOT AN `If-Match` HEADER, deliberately: the contract generator (tools/apidoc)
	// publishes body fields into kb/20-contracts/openapi.json and web-admin's generated types, but has
	// no way to declare a custom request header — so a header would be a precondition no client could
	// discover from the contract, which is the "screen built by guessing" failure
	// skills/rest-api-design names. The semantics are If-Match's; only the carrier differs.
	ExpectedUpdatedAt *time.Time `json:"expected_updated_at,omitempty"`
}

// doiTrangThaiVao is the body of POST /api/v1/tasks/{ma}/status.
//
// THE TARGET IS ON THE WIRE HERE AND IS NOT ON THE PETITION PATH, and the difference is the shape of
// the two lifecycles: §6's task lifecycle BRANCHES — `tam-dung` leaves all four working states — so
// there is no single "next" for the server to choose. What the server still owns is the MAP: a move
// the diagram does not draw is refused. `chuyen-tiep` is refused with 400 since 28/09/2026: forwarding
// is the assignment act (POST …/assignment), and the refusal says so.
type doiTrangThaiVao struct {
	Status string `json:"status"`
	// Note becomes the timeline entry (§5.9). Optional: when it is empty the entry carries a
	// generated sentence naming the two statuses, because the timeline may not have a gap.
	//
	// ⚠ REQUIRED BY THE SERVER ON ONE MOVE ONLY — `cho-duyet` → `dang-thuc-hien`, "Trả lại để làm
	// tiếp" (owner decision 2026-09-27), where it is the reason; empty or whitespace answers 400. It
	// STAYS omitempty on the wire: whether it is required depends on the task's CURRENT status, which
	// no schema of this body can express, and marking it required would break every other move of a
	// published contract (rule 2, invariant 4).
	Note string `json:"note,omitempty"`

	// Handover is the OPTIONAL "Cập nhật và giao việc" (user decision 07/10/2026): THE SAME SHAPE as the
	// body of POST …/assignment (`unit`, `assignee`, `note`; absent = leave, "" = clear, `unit` may not be
	// cleared; `lead_unit` / `monitor` refused 400 inside it too). Applied in the same transaction, and
	// the task lands in `status`, NOT `moi-giao`. Allowed to a holder of `task.assign` or the task's
	// current assignee; anybody else 403. Absent = no handover, the move as before (rule 2, invariant 4).
	Handover *taskAssignmentIn `json:"handover,omitempty"`

	// Attachments are OPTIONAL ids returned by POST /api/v1/tasks/{ma}/attachments and COMPLETED, by the
	// same officer, for this task — the rules of the log entry's `attachments`. Linked to this move's
	// timeline row in its transaction; they can never be added to that row later.
	Attachments []string `json:"attachments,omitempty"`
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

	// DecisionNote is the decider's note (migration 0031) — null when there is none: on a pending
	// request, on a decision taken without a note, and on every decision taken before 0031 (their note,
	// if any, is only in the task's log-entries). ADDED, OPTIONAL-READ (rule 2, invariant 4): the two
	// POST replies carry it too — null on filing, the note just written on deciding.
	DecisionNote *string `json:"decision_note"`
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
	// "" IS "NO NOTE" and travels as null, never as "" — the column holds NULL for it.
	if d.DecisionNote != "" {
		n := d.DecisionNote
		ra.DecisionNote = &n
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

// --- keys a body may no longer carry --------------------------------------------------------------

// refusedKey is one body key refused BY NAME, whatever its value — `null` and `""` included. The
// struct no longer has the field, and encoding/json ignores unknown keys, so without this check a
// client that still sends it would believe the server acted on it.
type refusedKey struct {
	key      string
	sentence string
}

// retiredRoleSentence is ADR 0065 NV5's refusal, one sentence for every door that creates or hands
// over a task.
const retiredRoleSentence = "Vai chủ trì đã gộp vào người thực hiện: không gửi `lead_unit` hay `monitor` nữa. " +
	"Cơ quan chủ trì là `unit`, chuyên viên theo dõi là `assignee`."

// retiredRoleKeys are refused on POST /api/v1/tasks, POST /api/v1/tasks/{ma}/assignment,
// POST /api/v1/citizen-reports/{maTraCuu}/tasks and POST /api/v1/meetings/{id}/conclusions/{stt}/task.
var retiredRoleKeys = []refusedKey{
	{"lead_unit", retiredRoleSentence},
	{"monitor", retiredRoleSentence},
}

// editRefusedKeys are refused on PATCH /api/v1/tasks/{ma} (ADR 0065 NV3).
var editRefusedKeys = []refusedKey{
	{"code", "Mã nhiệm vụ đã cấp không sửa được — không gửi `code` khi sửa nhiệm vụ."},
}

// refusedKeySent answers the sentence of the first refused key the object carries, or "". Keys are
// matched CASE-INSENSITIVELY, because that is how encoding/json matched them onto the field that used
// to exist: `"Lead_Unit"` was accepted then and must be refused now.
func refusedKeySent(keys map[string]json.RawMessage, refused []refusedKey) string {
	for sent := range keys {
		for _, k := range refused {
			if strings.EqualFold(sent, k.key) {
				return k.sentence
			}
		}
	}
	return ""
}

// decodeRefusingKeys is docThan plus the refusal of named keys: 400 `invalid_request` when the body
// is not a JSON object this struct decodes, or when it carries a refused key. Answers the response
// itself; false means "stop".
func decodeRefusingKeys(w http.ResponseWriter, r *http.Request, vao any, refused []refusedKey) bool {
	var raw json.RawMessage
	if !docThan(w, r, &raw) {
		return false
	}
	var keys map[string]json.RawMessage
	if json.Unmarshal(raw, &keys) != nil || json.Unmarshal(raw, vao) != nil {
		// The decoder's message quotes the input; it never reaches the client (docThan's reason).
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Nội dung gửi lên không phải JSON hợp lệ hoặc quá lớn.", "")
		return false
	}
	if sentence := refusedKeySent(keys, refused); sentence != "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", sentence, "")
		return false
	}
	return true
}

// --- the six handlers ----------------------------------------------------------------------------

// TaoNhiemVu books one task. POST /api/v1/tasks
func (h *Handler) TaoNhiemVu(w http.ResponseWriter, r *http.Request) {
	var vao taoNhiemVuVao
	if !decodeRefusingKeys(w, r, &vao, retiredRoleKeys) {
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
	// A PETITION SOURCE LIKEWISE (user decision 28/09/2026, ErrPetitionSourceNotDirect): nothing here
	// can check that `source_id` names a live petition of THIS commune.
	if domain.NguonGiao(vao.Source) == domain.NguonPhanAnh {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			domain.ErrPetitionSourceNotDirect.Error(), "")
		return
	}

	// REFUSED BEFORE THE USE CASE IS REACHED, so a malformed date opens no transaction at all.
	vanBan, err := vanBanVaoTrong(vao.Documents)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
		return
	}

	yc := app.YeuCauTaoNhiemVu{
		Ma:                vao.Code,
		TuSinhMa:          vao.AutoCode,
		Loai:              vao.Type,
		Khoi:              vao.Bloc,
		TieuDe:            vao.Title,
		MoTa:              vao.Body,
		MucUuTien:         vao.Priority,
		GhiChu:            vao.Note,
		NguonGiao:         vao.Source,
		NguonID:           vao.SourceID,
		BoPhanID:          vao.Unit,
		NguoiThucHienMa:   vao.Assignee,
		LanhDaoGiaoViecMa: vao.Assigner,
		ParentCode:        vao.Parent,
		VanBan:            vanBan,
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
	if !decodeRefusingKeys(w, r, &vao, editRefusedKeys) {
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
		DueAt:                    vao.DueAt,
		Khoi:                     vao.Bloc,
		TieuDe:                   vao.Title,
		MoTa:                     vao.Body,
		MucUuTien:                vao.Priority,
		TienDo:                   vao.Progress,
		TomTatKetQua:             vao.ResultSummary,
		GhiChu:                   vao.Note,
		LanhDaoPheDuyetHoanThanh: vao.LeaderApproved,
		CapTrenCongNhanHoanThanh: vao.SuperiorAcknowledged,
		ParentCode:               vao.Parent,
		VanBan:                   vanBan,
		ExpectedUpdatedAt:        vao.ExpectedUpdatedAt,
	}, nguoi)
	if err != nil {
		h.traLoiLoiNhiemVu(w, r, "sửa nhiệm vụ", err)
		return
	}
	vietJSON(w, http.StatusOK, nhiemVuRaNgoai(n))
}

// DoiTrangThaiNhiemVu moves the task along §6. POST /api/v1/tasks/{ma}/status
//
// THIS HANDLER ANSWERS TWO QUESTIONS AND DECIDES NOTHING. The route's gate refused anyone without
// `task.read` (vigov-require a37ec96). What is read here is whether the caller holds `task.update`
// (with being the assignee, who may move at all) and `task.approve` (which moves need it); both facts
// are handed down, and the decisions are app.DoiTrangThai's, inside the transaction.
func (h *Handler) DoiTrangThaiNhiemVu(w http.ResponseWriter, r *http.Request) {
	var raw json.RawMessage
	if !docThan(w, r, &raw) {
		return
	}
	var vao doiTrangThaiVao
	if !decodeStatusBody(w, raw, &vao) {
		return
	}
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheNhiemVu(w, r)
		return
	}

	yc := app.YeuCauDoiTrangThai{TrangThai: vao.Status, GhiChu: vao.Note}
	if vao.Handover != nil {
		yc.Handover = &app.TaskAssignmentRequest{
			Change: domain.TaskAssignmentChange{Unit: vao.Handover.Unit, Assignee: vao.Handover.Assignee},
			Note:   vao.Handover.Note,
		}
		// Asked ONLY when a handover is present, so a plain move consults nothing it did not before.
		yc.AssignRight = h.hasTaskAssign(r)
	}
	if len(vao.Attachments) > 0 {
		yc.Attachments = vao.Attachments
	}

	n, err := h.d.GhiNhiemVu.DoiTrangThai(r.Context(), r.PathValue("ma"), yc,
		nguoi, h.coQuyenDuyetHoanThanh(r), h.hasTaskUpdate(r))
	if err != nil {
		h.writeStatusError(w, r, err)
		return
	}
	vietJSON(w, http.StatusOK, nhiemVuRaNgoai(n))
}

// decodeStatusBody decodes the status body and refuses `lead_unit` / `monitor` INSIDE `handover`, the
// way POST …/assignment refuses them at its top level — the same shape must not accept, and silently
// ignore, keys its twin refuses. Answers the response itself; false means "stop".
func decodeStatusBody(w http.ResponseWriter, raw json.RawMessage, vao *doiTrangThaiVao) bool {
	var top struct {
		Handover json.RawMessage `json:"handover"`
	}
	if json.Unmarshal(raw, vao) != nil || json.Unmarshal(raw, &top) != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Nội dung gửi lên không phải JSON hợp lệ hoặc quá lớn.", "")
		return false
	}
	if vao.Handover == nil {
		return true
	}
	var keys map[string]json.RawMessage
	if json.Unmarshal(top.Handover, &keys) != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Nội dung gửi lên không phải JSON hợp lệ hoặc quá lớn.", "")
		return false
	}
	if sentence := refusedKeySent(keys, retiredRoleKeys); sentence != "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", sentence, "")
		return false
	}
	return true
}

// writeStatusError maps the refusals only the optional handover / attachments raise, then hands every
// other refusal to the attachment mapper, which ends in traLoiLoiNhiemVu — so a move without the new
// fields answers exactly as before.
func (h *Handler) writeStatusError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrHandoverNotAllowed):
		httpx.WriteError(w, http.StatusForbidden, "forbidden",
			cauTuChoi(err, domain.ErrHandoverNotAllowed), "")
	case errors.Is(err, app.ErrAssignmentStaffInvalid), errors.Is(err, app.ErrAssignmentStaffUnchecked),
		errors.Is(err, domain.ErrTaskClosedForAssignment):
		h.writeAssignmentError(w, r, err)
	default:
		h.answerTaskAttachmentError(w, r, "chuyển trạng thái", err)
	}
}

// hasTaskAssign: does this account hold `task.assign`? FAIL CLOSED — no principal is `false`.
func (h *Handler) hasTaskAssign(r *http.Request) app.TaskAssignRight {
	principal, ok := authz.From(r.Context())
	if !ok {
		return false
	}
	return app.TaskAssignRight(h.d.Checker.Allows(r.Context(), principal, PermTaskAssign))
}

// PermTaskAssign is the commune-wide "Giao nhiệm vụ" key (service-identity/migrations/0001_init.sql:307),
// asked with Checker.Allows on the status route when it carries a handover. A constant for the reason
// PermTaskUpdate is one.
const PermTaskAssign authz.Perm = "task.assign"

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
		// Since ADR 0065 NV1 this fires only from `cho-duyet` — the sentence says so, because the same
		// officer may complete a task straight from `dang-thuc-hien`.
		httpx.WriteError(w, http.StatusForbidden, "forbidden",
			"Nhiệm vụ đã gửi chờ duyệt chỉ người có quyền duyệt hoàn thành mới duyệt được. Tài khoản "+
				"của bạn chưa có quyền này.", "")
	case errors.Is(err, app.ErrKhongDuocTraLai):
		httpx.WriteError(w, http.StatusForbidden, "forbidden",
			"Trả lại nhiệm vụ đang chờ duyệt để làm tiếp cần quyền duyệt hoàn thành. Tài khoản của "+
				"bạn mới có quyền cập nhật tiến độ.", "")
	case errors.Is(err, domain.ErrStatusNeedsHolder):
		// vigov-require a37ec96 `status_needs_holder`: the gate let a `task.read` holder in; only the
		// assignee or a holder of `task.update` moves the status.
		httpx.WriteError(w, http.StatusForbidden, "forbidden",
			cauTuChoi(err, domain.ErrStatusNeedsHolder), "")
	case errors.Is(err, domain.ErrNotTaskParticipant):
		// vigov-require a37ec96: the gate let a `task.read` holder in; the row says they are neither
		// the assignee nor related to this task. The domain's own sentence names who may write.
		httpx.WriteError(w, http.StatusForbidden, "forbidden",
			cauTuChoi(err, domain.ErrNotTaskParticipant), "")
	case errors.Is(err, app.ErrReopenNeedsApproval):
		httpx.WriteError(w, http.StatusForbidden, "forbidden",
			"Mở lại nhiệm vụ đã hoàn thành cần quyền duyệt hoàn thành. Tài khoản của bạn mới có "+
				"quyền cập nhật tiến độ.", "")
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
	case errors.Is(err, domain.ErrReopenParentCompleted):
		// ADR 0065 NV2. ITS OWN CODE, not `task_tree`: the client can offer the one act that unblocks
		// it — open the parent named in the sentence and reopen that first.
		httpx.WriteError(w, http.StatusConflict, "parent_completed",
			cauTuChoi(err, domain.ErrReopenParentCompleted), "")
	case errors.Is(err, domain.ErrChuTrinhCayNhiemVu), errors.Is(err, domain.ErrChaKhongTonTai),
		errors.Is(err, domain.ErrCayNhiemVuQuaLon):
		httpx.WriteError(w, http.StatusConflict, "task_tree", cauTuChoi(err,
			domain.ErrChuTrinhCayNhiemVu, domain.ErrChaKhongTonTai, domain.ErrCayNhiemVuQuaLon), "")
	case errors.Is(err, petstore.ErrMaNhiemVuDaTonTai):
		httpx.WriteError(w, http.StatusConflict, "code_taken",
			// Raised by CREATE only since ADR 0065 NV3 (a typed code already issued); "đã đổi sang mã khác"
			// stays true of the tasks renamed while the edit path was live (28/09 → 30/09/2026).
			"Mã nhiệm vụ này đã được dùng trong xã — kể cả khi nhiệm vụ mang mã đó đã bị xoá "+
				"hoặc đã đổi sang mã khác. Mã đã cấp thì không cấp lại.", "")
	case errors.Is(err, app.ErrTaskEditConflict):
		// Optimistic locking (28/09/2026). Its OWN code, not `task_state`: the client knows exactly what
		// happened — its copy is stale — and can reload and show the officer the newer version.
		httpx.WriteError(w, http.StatusConflict, "task_changed",
			"Nhiệm vụ đã được người khác sửa sau khi bạn mở. Hãy tải lại để xem bản mới nhất rồi sửa lại.", "")
	case errors.Is(err, petstore.ErrNhiemVuDaChuyenTrang):
		httpx.WriteError(w, http.StatusConflict, "task_state",
			"Nhiệm vụ đã thay đổi trong lúc bạn đang mở màn hình. Hãy tải lại rồi thao tác lại.", "")
	case errors.Is(err, domain.ErrChuyenTrangThaiNhiemVuSaiLuc):
		httpx.WriteError(w, http.StatusConflict, "task_state", cauTuChoi(err,
			domain.ErrChuyenTrangThaiNhiemVuSaiLuc), "")
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

	// --- the assigner check (owner decision 2026-09-27) ------------------------------------------------
	case errors.Is(err, app.ErrLanhDaoGiaoViecKhongHopLe):
		// 400 `invalid_request`, THE STATUS AND CODE the petition path answers for an assignee identity
		// did not accept (xu_ly_phan_anh.go, ErrCanBoKhongNhanDuocViec). ONE SENTENCE FOR FIVE REASONS
		// and the code is not echoed: splitting "another commune" from "unknown" leaks existence across
		// communes (rule 1), and "locked" publishes an employment fact.
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Người được chọn làm lãnh đạo giao việc không hợp lệ. Hãy chọn người khác trong danh sách.", "")
	case errors.Is(err, app.ErrChuaKiemDuocLanhDaoGiaoViec):
		// 503 AND NOT 400 OR 500: the check did not happen, which is neither a bad assigner nor a fault
		// in this service. The same code the petition path uses for its assignee check, so a client
		// retries both the same way. No staff code in the log line — the wrapped chain carries only
		// the gRPC status core/identityclient already logged.
		h.d.Log.Warn("CẢNH BÁO: từ chối giao việc vì chưa kiểm được lãnh đạo giao việc",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "assignee_check_unavailable",
			"Chưa kiểm tra được lãnh đạo giao việc nên nhiệm vụ CHƯA được tạo. "+
				"Vui lòng thử lại sau ít phút.", "")

	// --- the unit check (ResolveLiveOrgUnits, user decision 28/09/2026) -----------------------------
	case errors.Is(err, app.ErrOrgUnitNotLive):
		// ONE SENTENCE for unknown, removed, another commune; the id is not echoed (rule 1).
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Bộ phận được chọn không nhận được việc. Hãy chọn bộ phận khác trong danh sách.", "")
	case errors.Is(err, app.ErrOrgUnitUnchecked):
		// 503: the check did not happen, and nothing was written. The same code the staff checks use,
		// so a client retries all of them the same way.
		h.d.Log.Warn("CẢNH BÁO: từ chối ghi nhiệm vụ vì chưa kiểm được bộ phận nhận việc",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "assignee_check_unavailable",
			"Chưa kiểm tra được bộ phận nhận việc nên nhiệm vụ CHƯA được ghi. "+
				"Vui lòng thử lại sau ít phút.", "")

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
