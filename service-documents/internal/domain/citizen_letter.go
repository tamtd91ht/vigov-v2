package domain

// SỔ ĐƠN THƯ CÔNG DÂN — the citizen-letter register (`don_thu`, URL `citizen-letters`), ADR 0039 and
// ADR 0078 #2–#4, on top of migration 0006.
//
// WHAT LIVES HERE is what is true regardless of storage: the four letter types (C4), the ten statuses
// of TT 05/2021 and the arrows between them (C3), what a whistleblower letter may show and to whom
// (ADR 0078 #4), the duplicate-similarity measure (C11), the "số ngày xử lý" count (C16/C17) and the
// report figures (C12, as ADR 0084 #4 reversed it), the display groups and the closing-result rule by
// type (ADR 0084 #2). Standard library only.
//
// THE DATABASE CLOSES THE SET, THIS FILE DRAWS THE ARROWS. 0006 refuses an unknown status and binds
// `accepted_at` / `resolved_at` to the statuses; it does not encode which move is admissible. That is
// CheckLetterTransition below, and nothing else decides it.
//
// NAMES (rule 12): identifiers are English, enum VALUES are the Vietnamese codes 0006 stores (ADR
// 0011). `thu-ly` is "admission" per kb/00-foundation/ubiquitous-language.md:154. The glossary names no
// English word for the four letter types or for the other statuses; the ones below were chosen here
// and are reported in the hand-over rather than presented as settled.

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// --- refusals ---------------------------------------------------------------------------------------

// LetterRefusal says which kind of "no" a LetterError is, so the edge maps it to ONE status code with
// one switch instead of a list of sentinels that drifts.
type LetterRefusal int

const (
	// RefusalInput — what the client sent is wrong (400).
	RefusalInput LetterRefusal = iota + 1
	// RefusalState — the caller may do this, but not to this letter in its current state (409).
	RefusalState
	// RefusalNotPermitted — the caller holds `petition.read` but is neither the assignee nor a holder
	// of `petition.create` (403). C13/C14.
	RefusalNotPermitted
)

// LetterError is every refusal this register makes. Msg IS THE SENTENCE THE CLIENT READS: Vietnamese,
// names the field and the rule, carries NO personal data and no internal detail (rule 3, forbidden
// #3). The edge returns Msg — never err.Error() of the wrapped chain, which carries the commune id and
// the operation prefix.
type LetterError struct {
	Refusal LetterRefusal
	Msg     string
	base    *LetterError
}

func (e *LetterError) Error() string { return e.Msg }

// Is lets a refinement (a too-long field with its ceiling appended) still match its sentinel.
func (e *LetterError) Is(target error) bool {
	t, ok := target.(*LetterError)
	return ok && (t == e || (e.base != nil && t == e.base))
}

func (e *LetterError) withDetail(detail string) *LetterError {
	return &LetterError{Refusal: e.Refusal, Msg: e.Msg + " " + detail, base: e}
}

func inputRefusal(msg string) *LetterError { return &LetterError{Refusal: RefusalInput, Msg: msg} }
func stateRefusal(msg string) *LetterError { return &LetterError{Refusal: RefusalState, Msg: msg} }

var (
	ErrLetterReceivedDateMissing = inputRefusal("Đơn thư: thiếu ngày nhận đơn.")
	ErrLetterReceivedDateFuture  = inputRefusal("Đơn thư: ngày nhận đơn ở tương lai.")
	ErrLetterDateTooOld          = inputRefusal("Đơn thư: ngày quá xa trong quá khứ.")
	ErrLetterTypeInvalid         = inputRefusal("Đơn thư: loại đơn phải là một trong kien-nghi-phan-anh, khieu-nai, to-cao, de-nghi.")
	ErrLetterSummaryMissing      = inputRefusal("Đơn thư: thiếu trích yếu nội dung đơn.")
	ErrLetterFieldTooLong        = inputRefusal("Đơn thư: nội dung một trường vượt độ dài cho phép.")
	ErrLetterPhoneInvalid        = inputRefusal("Đơn thư: số điện thoại người gửi chỉ gồm chữ số, dấu cách, dấu +, -, . và ngoặc.")
	ErrLetterStatusInvalid       = inputRefusal("Đơn thư: trạng thái không thuộc bộ trạng thái của sổ đơn thư.")
	ErrLetterUnitMissing         = inputRefusal("Chuyển đơn: chưa chọn bộ phận nhận.")
	ErrLetterReasonMissing       = inputRefusal("Chuyển đơn: thiếu lý do chuyển.")
	ErrLetterNoteMissing         = inputRefusal("Nhật ký đơn thư: thiếu nội dung ghi chú.")
	ErrLetterUnitNotLive         = inputRefusal("Bộ phận này không nhận được việc: không có trong sơ đồ tổ chức đang dùng của xã.")
	ErrLetterAssigneeNotLive     = inputRefusal("Cán bộ này không nhận được việc: không có tài khoản đang hoạt động ở xã.")
	ErrLetterRelatedNotFound     = inputRefusal("Đơn liên quan không có trong sổ đơn thư của xã.")
	ErrLetterResultIncomplete    = inputRefusal("Kết quả giải quyết đơn khiếu nại / tố cáo cần đủ số văn bản, ngày văn bản, người ký, cơ quan ban hành và tóm tắt kết quả.")
	ErrLetterReplyMissing        = inputRefusal("Kết quả giải quyết: thiếu nội dung trả lời.")
	ErrLetterResultDocumentPair  = inputRefusal("Kết quả giải quyết: số văn bản và ngày văn bản phải nhập cùng nhau.")
	ErrLetterSourceInvalid       = inputRefusal("Đơn thư: nguồn vào sổ phải là một trong nhap-tay, nhap-excel, mini-app, thu-dien-tu.")
	ErrLetterSourceFromClient    = inputRefusal("Đơn thư: nguồn vào sổ do hệ thống ghi theo đường vào sổ, không nhận từ client.")
	ErrLetterResultDateFuture    = inputRefusal("Kết quả giải quyết: ngày văn bản ở tương lai.")
	ErrLetterSenderEmpty         = inputRefusal("Sửa người gửi: không có trường nào để sửa.")

	// The three facts a client may never state on a booking — refused, not ignored, the line the
	// incoming register draws (ErrSoDoTuClient and its two siblings).
	ErrLetterNumberFromClient = inputRefusal("Đơn thư: số vào sổ do hệ thống cấp, không nhận từ client.")
	ErrLetterStatusFromClient = inputRefusal("Đơn thư: trạng thái do luồng xử lý quyết định, không nhận từ client.")
	ErrLetterDueFromClient    = inputRefusal("Đơn thư: hạn xử lý không đặt lúc vào sổ — vào sổ xong, đặt hạn bằng thao tác Đặt hạn xử lý của đơn.")

	ErrLetterDueInvalid = inputRefusal("Hạn xử lý: phải là một thời điểm dạng RFC 3339 (ví dụ 2026-10-20T17:00:00+07:00), hoặc null để bỏ hạn.")
	ErrLetterDueTooFar  = inputRefusal("Hạn xử lý: năm phải từ 2000 đến 2200.")

	ErrLetterTransitionRefused = stateRefusal("Không chuyển được đơn từ trạng thái hiện tại sang trạng thái đã chọn.")
	ErrLetterNeedsResult       = stateRefusal("Đơn khiếu nại / tố cáo chưa ghi kết quả giải quyết (số, ngày, người ký, cơ quan ban hành văn bản kết quả và tóm tắt) nên chưa chuyển được sang Đã giải quyết. Hãy ghi kết quả trước.")
	ErrLetterResultNotAllowed  = stateRefusal("Chỉ ghi kết quả giải quyết khi đơn đang ở Thụ lý hoặc Đang giải quyết.")
	ErrLetterFinished          = stateRefusal("Đơn đã kết thúc xử lý nên không chuyển tiếp được.")
	ErrLetterDueOnFinished     = stateRefusal("Đơn đã kết thúc xử lý nên không đặt hay bỏ hạn xử lý được.")

	ErrLetterNotPermitted = &LetterError{Refusal: RefusalNotPermitted,
		Msg: "Chỉ cán bộ được giao xử lý đơn này hoặc người có quyền Tiếp nhận đơn thư mới thực hiện được thao tác này."}
)

// --- types ------------------------------------------------------------------------------------------

// LetterType is C4's four types. `phan-anh` alone is NOT one: that word belongs to service-petitions'
// citizen reports, and conflating the two applies the wrong statutory procedure (0006:192-193).
type LetterType string

const (
	LetterTypeFeedback     LetterType = "kien-nghi-phan-anh"
	LetterTypeComplaint    LetterType = "khieu-nai"
	LetterTypeDenunciation LetterType = "to-cao"
	LetterTypeRequest      LetterType = "de-nghi"
)

// LetterTypes is the closed set, in the order a form lists them.
var LetterTypes = []LetterType{LetterTypeFeedback, LetterTypeComplaint, LetterTypeDenunciation, LetterTypeRequest}

func (t LetterType) Valid() bool {
	for _, v := range LetterTypes {
		if t == v {
			return true
		}
	}
	return false
}

// ProtectsIdentity is Luật Tố cáo 2018 Đ.8 in one line: a denunciation's sender is protected, so no
// surface that lists letters shows who sent it, nor what it says (ADR 0078 #4).
func (t LetterType) ProtectsIdentity() bool { return t == LetterTypeDenunciation }

// RequiresIssuedResult is ADR 0084 #2 (C10 narrowed): a complaint or a denunciation is resolved by an
// ISSUED decision/conclusion (Luật Khiếu nại 2011, Luật Tố cáo 2018), so it cannot reach
// `da-giai-quyet` without the document's number, date, signer, issuing body and the summary. A
// feedback letter or a request is answered by a reply the clerk may leave unwritten. Migration 0008's
// `citizen_letter_resolved_has_result` is the same rule as a CHECK.
func (t LetterType) RequiresIssuedResult() bool {
	return t == LetterTypeComplaint || t == LetterTypeDenunciation
}

// LetterSource is HOW a letter entered the register — the "nguồn vào sổ" chip (ADR 0084 #7, migration
// 0008). A FACT OF THE BOOKING ACT: written once by the path that booked the letter, never by a client
// and never by a later edit. No UPDATE in store/citizen_letter.go names the column, and no method here
// changes it — a letter that changed source would show a chip that lies about the record's origin.
type LetterSource string

const (
	LetterSourceManual  LetterSource = "nhap-tay"    // typed in by a clerk — the booking route
	LetterSourceExcel   LetterSource = "nhap-excel"  // the Excel import (TASK-07)
	LetterSourceMiniApp LetterSource = "mini-app"    // arrived through the Zalo Mini App
	LetterSourceEmail   LetterSource = "thu-dien-tu" // arrived by e-mail
)

// LetterSources is the closed set — 0008's `citizen_letter_source_valid`.
var LetterSources = []LetterSource{LetterSourceManual, LetterSourceExcel, LetterSourceMiniApp, LetterSourceEmail}

func (s LetterSource) Valid() bool {
	for _, v := range LetterSources {
		if s == v {
			return true
		}
	}
	return false
}

// LetterStatus is TT 05/2021's set, C3.
type LetterStatus string

const (
	LetterStatusNew          LetterStatus = "moi-vao-so"
	LetterStatusScreening    LetterStatus = "dang-xu-ly-don"
	LetterStatusAdmitted     LetterStatus = "thu-ly"
	LetterStatusNotAdmitted  LetterStatus = "khong-thu-ly"
	LetterStatusGuided       LetterStatus = "huong-dan"
	LetterStatusForwarded    LetterStatus = "chuyen-don"
	LetterStatusFiled        LetterStatus = "luu-don"
	LetterStatusResolving    LetterStatus = "dang-giai-quyet"
	LetterStatusResolved     LetterStatus = "da-giai-quyet"
	LetterStatusDiscontinued LetterStatus = "dinh-chi"
)

// LetterStatuses is the closed set — the same ten codes as 0006's three CHECKs.
var LetterStatuses = []LetterStatus{
	LetterStatusNew, LetterStatusScreening, LetterStatusAdmitted, LetterStatusNotAdmitted,
	LetterStatusGuided, LetterStatusForwarded, LetterStatusFiled, LetterStatusResolving,
	LetterStatusResolved, LetterStatusDiscontinued,
}

func (s LetterStatus) Valid() bool {
	for _, v := range LetterStatuses {
		if s == v {
			return true
		}
	}
	return false
}

// letterTransitions IS C3, AND THE ONLY COPY OF IT. Assignment is an ATTRIBUTE, not a status: routing
// a letter moves `holding_unit_id` / `assignee_code` and never appears here.
var letterTransitions = map[LetterStatus][]LetterStatus{
	LetterStatusNew:       {LetterStatusScreening},
	LetterStatusScreening: {LetterStatusAdmitted, LetterStatusNotAdmitted, LetterStatusGuided, LetterStatusForwarded, LetterStatusFiled},
	LetterStatusAdmitted:  {LetterStatusResolving},
	LetterStatusResolving: {LetterStatusResolved, LetterStatusDiscontinued},
}

// NextStatuses is where a letter in `s` may move. Empty for every finishing status.
func (s LetterStatus) NextStatuses() []LetterStatus {
	return append([]LetterStatus(nil), letterTransitions[s]...)
}

// Finished reports whether processing has ended: the four processing-phase outcomes and the two ends
// of resolution. DERIVED from the transition table, so a status that gains an arrow stops being
// "finished" in the same edit.
func (s LetterStatus) Finished() bool { return s.Valid() && len(letterTransitions[s]) == 0 }

// EndsResolution reports the two statuses 0006 binds `resolved_at` to.
func (s LetterStatus) EndsResolution() bool {
	return s == LetterStatusResolved || s == LetterStatusDiscontinued
}

// FinishedInProcessing is the four processing-phase outcomes — finished, but with no `resolved_at`.
func FinishedInProcessing() []LetterStatus {
	var out []LetterStatus
	for _, s := range LetterStatuses {
		if s.Finished() && !s.EndsResolution() {
			out = append(out, s)
		}
	}
	return out
}

// LetterTransitionError is a refused arrow. It names the two CODES and nothing about the letter.
type LetterTransitionError struct{ From, To LetterStatus }

func (e *LetterTransitionError) Error() string {
	return fmt.Sprintf("%s (hiện tại: %s, muốn chuyển sang: %s)", ErrLetterTransitionRefused.Msg, e.From, e.To)
}
func (e *LetterTransitionError) Is(target error) bool { return target == ErrLetterTransitionRefused }

// CheckLetterTransition refuses any move C3 does not draw, including staying where it is.
func CheckLetterTransition(from, to LetterStatus) error {
	if !to.Valid() {
		return ErrLetterStatusInvalid
	}
	for _, next := range letterTransitions[from] {
		if next == to {
			return nil
		}
	}
	return &LetterTransitionError{From: from, To: to}
}

// --- display groups (ADR 0084 #2) -------------------------------------------------------------------

// LetterStatusGroup is one of the prototype's DISPLAY groups — the status filter and the 4-step bar.
// DERIVED on read from the TT 05/2021 status and whether a unit holds the letter; NEVER STORED: the
// data keeps C3's ten statuses, because folding them into groups would erase whether a letter was
// admitted, guided or filed — editing archival records (rule 7, forbidden #5; ADR 0084 "Vì sao không
// gộp trạng thái").
//
// The VALUES are the codes the web filter sends (Vietnamese, ADR 0011). There is deliberately no
// "Chờ phân công" group (ADR 0084 #5).
type LetterStatusGroup string

const (
	LetterGroupNew          LetterStatusGroup = "moi-vao-so"       // Mới vào sổ
	LetterGroupAssigned     LetterStatusGroup = "da-phan-cong"     // Đã phân công
	LetterGroupInProgress   LetterStatusGroup = "dang-xu-ly"       // Đang xử lý
	LetterGroupResolved     LetterStatusGroup = "da-giai-quyet"    // Đã giải quyết
	LetterGroupForwarded    LetterStatusGroup = "chuyen-cap-tren"  // Chuyển cấp trên (branch)
	LetterGroupNotProcessed LetterStatusGroup = "luu-khong-thu-ly" // Lưu, không thụ lý (branch)
)

// LetterHolding is the second axis of a group: whether a unit must, must not, or may hold the letter.
type LetterHolding int

const (
	HoldingAny LetterHolding = iota
	HoldingNone
	HoldingSome
)

// LetterGroupRule is one row of ADR 0084 §2's table.
type LetterGroupRule struct {
	Group    LetterStatusGroup
	Statuses []LetterStatus
	Holding  LetterHolding
}

// letterGroupRules IS ADR 0084 §2's TABLE, AND THE ONLY COPY OF IT — the filter (store), the derived
// field (GroupOf) and any future count by group read this, so two figures cannot drift ("Hệ quả" #2).
// Every (status, held?) pair falls in exactly one row; TestLetterGroupsPartitionEveryStatus holds it.
var letterGroupRules = []LetterGroupRule{
	{LetterGroupNew, []LetterStatus{LetterStatusNew}, HoldingNone},
	{LetterGroupAssigned, []LetterStatus{LetterStatusNew}, HoldingSome},
	{LetterGroupInProgress, []LetterStatus{LetterStatusScreening, LetterStatusAdmitted, LetterStatusResolving}, HoldingAny},
	{LetterGroupResolved, []LetterStatus{LetterStatusResolved}, HoldingAny},
	{LetterGroupForwarded, []LetterStatus{LetterStatusForwarded}, HoldingAny},
	{LetterGroupNotProcessed, []LetterStatus{LetterStatusNotAdmitted, LetterStatusGuided, LetterStatusFiled, LetterStatusDiscontinued}, HoldingAny},
}

// LetterStatusGroups is the closed set, in the order the filter lists them.
func LetterStatusGroups() []LetterStatusGroup {
	out := make([]LetterStatusGroup, 0, len(letterGroupRules))
	for _, r := range letterGroupRules {
		out = append(out, r.Group)
	}
	return out
}

// Rule returns the group's row of the table; ok is false for a value outside the set.
func (g LetterStatusGroup) Rule() (LetterGroupRule, bool) {
	for _, r := range letterGroupRules {
		if r.Group == g {
			return LetterGroupRule{Group: r.Group, Statuses: append([]LetterStatus(nil), r.Statuses...), Holding: r.Holding}, true
		}
	}
	return LetterGroupRule{}, false
}

func (g LetterStatusGroup) Valid() bool {
	_, ok := g.Rule()
	return ok
}

// StatusGroup derives the letter's display group. "" only for a status outside C3's set, which the
// database refuses.
func (l CitizenLetter) StatusGroup() LetterStatusGroup {
	held := l.HoldingUnitID != ""
	for _, r := range letterGroupRules {
		if r.Holding == HoldingNone && held || r.Holding == HoldingSome && !held {
			continue
		}
		for _, s := range r.Statuses {
			if s == l.Status {
				return r.Group
			}
		}
	}
	return ""
}

// LetterLogKind is the kind of one log row — 0006's `citizen_letter_log_kind_valid`.
type LetterLogKind string

const (
	LetterLogStatusChange     LetterLogKind = "chuyen-trang-thai"
	LetterLogRouting          LetterLogKind = "luan-chuyen"
	LetterLogNote             LetterLogKind = "ghi-chu"
	LetterLogResult           LetterLogKind = "ket-qua"
	LetterLogSenderCorrection LetterLogKind = "sua-nguoi-gui"
)

// --- the record -------------------------------------------------------------------------------------

// CitizenLetter is one row of the register.
//
// THE TWO DUE INSTANTS ARE STORED, NEVER COMPUTED HERE (rule 10, invariant 2). Each is the instant a
// clerk SET, as set, or zero for "Không đặt" (ADR 0079 lô 5 Q18; SetActiveDue). THERE IS NO OVERDUE
// FIELD: IsOverdue derives it (rule 10, invariant 3).
//
// ClosedAt IS DERIVED ON READ by the store — `resolved_at` for the two ends of resolution, the instant
// of the log row that moved the letter into one of the four processing-phase outcomes otherwise. Not a
// column: a second column for one fact drifts.
type CitizenLetter struct {
	ID           string
	Number       int
	Year         int
	ReceivedDate time.Time
	Type         LetterType
	Source       LetterSource // fixed at booking, never changed (LetterSource)

	// ⚠ PERSONAL DATA (rule 3). All optional (C7).
	SenderName    string
	SenderPhone   string
	SenderAddress string
	// ⚠ May hold personal data — it quotes the letter.
	Summary string

	Status        LetterStatus
	HoldingUnitID string
	AssigneeCode  string // a STAFF BUSINESS CODE (rule 6, invariant 8)

	ProcessingDueAt time.Time
	ResolutionDueAt time.Time
	AcceptedAt      time.Time
	ResolvedAt      time.Time
	ClosedAt        time.Time

	RelatedLetterID string

	ResultDocumentNo   string
	ResultDocumentDate time.Time
	ResultSigner       string
	ResultIssuer       string
	ResultSummary      string // ⚠ may hold personal data

	CreatedByCode string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// LetterLogEntry is one row of `citizen_letter_log`. Append-only; a correction is a new row.
type LetterLogEntry struct {
	ID           string
	LetterID     string
	At           time.Time
	ActorCode    string
	Kind         LetterLogKind
	FromStatus   LetterStatus
	ToStatus     LetterStatus
	FromUnitID   string
	ToUnitID     string
	AssigneeCode string
	Content      string // ⚠ free text on `ghi-chu` / `luan-chuyen` / `chuyen-trang-thai`
	CreatedAt    time.Time
}

// LetterAuditSubject is the business code a letter is filed under in the audit trail — rule 6,
// invariant 8's reason applies to the subject too: "DT-2026-0007" names a register line an inspector
// can open, a ULID names nothing. `DT` cannot collide with `VB-DEN` / `VB-DI`. STAFF-ONLY: it is never
// a lookup code handed to a citizen (that would be rule 4, forbidden #3 — this is a register number).
func LetterAuditSubject(year, number int) string {
	return "DT-" + strconv.Itoa(year) + "-" + fmt.Sprintf("%04d", number)
}

// HasResult reports whether the result THE LETTER'S TYPE requires before `da-giai-quyet` is recorded
// (ADR 0084 #2, migration 0008): for a complaint or a denunciation, the issued document (number, date,
// signer, issuing body) and the summary; for a feedback letter or a request, nothing — the reply is
// optional, so such a letter always "has" what it needs.
func (l CitizenLetter) HasResult() bool {
	if !l.Type.RequiresIssuedResult() {
		return true
	}
	return l.ResultDocumentNo != "" && !l.ResultDocumentDate.IsZero() && l.ResultSigner != "" &&
		l.ResultIssuer != "" && l.ResultSummary != ""
}

// LetterResult is the result as a clerk recorded it, already trimmed and length-checked.
type LetterResult struct {
	DocumentNo   string
	DocumentDate time.Time
	Signer       string
	Issuer       string
	Summary      string
}

// WithResult checks `r` against the rule of the letter's type and returns the letter carrying it.
//
//	khieu-nai, to-cao              all five fields (C10 as 0006 had it) — ErrLetterResultIncomplete
//	kien-nghi-phan-anh, de-nghi    the reply (`Summary`, the prototype's one textarea) is required on
//	                               THIS act — recording an empty result records nothing; the document
//	                               fields stay optional, but number and date travel together (0006's
//	                               `citizen_letter_result_document_pair`)
//
// Recording a result is optional for the second pair; closing without one is Move's business.
func (l CitizenLetter) WithResult(r LetterResult) (CitizenLetter, error) {
	if l.Type.RequiresIssuedResult() {
		if r.DocumentNo == "" || r.DocumentDate.IsZero() || r.Signer == "" || r.Issuer == "" || r.Summary == "" {
			return l, ErrLetterResultIncomplete
		}
	} else {
		if r.Summary == "" {
			return l, ErrLetterReplyMissing
		}
		if (r.DocumentNo == "") != r.DocumentDate.IsZero() {
			return l, ErrLetterResultDocumentPair
		}
	}
	l.ResultDocumentNo, l.ResultDocumentDate = r.DocumentNo, r.DocumentDate
	l.ResultSigner, l.ResultIssuer, l.ResultSummary = r.Signer, r.Issuer, r.Summary
	return l, nil
}

// CurrentStageDueAt is the deadline the letter shows right now, as ResolveCitizenLetterForTask's
// contract defines it (documents.proto, CitizenLetterTaskSource.current_due_at): the resolution
// deadline once the letter was admitted (`accepted_at` set), the processing deadline before. Unlike
// ActiveDueAt it does not go blank on a finished letter — the contract converts a letter in any status
// (ADR 0085 A) and the task inherits what the letter showed. Zero = no deadline.
func (l CitizenLetter) CurrentStageDueAt() time.Time {
	if !l.AcceptedAt.IsZero() {
		return l.ResolutionDueAt
	}
	return l.ProcessingDueAt
}

// SenderUnknown is C7's "Không rõ người gửi" — DERIVED from the three columns, never a flag.
func (l CitizenLetter) SenderUnknown() bool {
	return l.SenderName == "" && l.SenderPhone == "" && l.SenderAddress == ""
}

// ActiveDueAt is the commitment that applies to the letter's CURRENT phase: the processing deadline
// before admission, the resolution deadline after it, none once finished. Zero = none set.
func (l CitizenLetter) ActiveDueAt() time.Time {
	switch LetterDueColumn(l.Status) {
	case "processing_due_at":
		return l.ProcessingDueAt
	case "resolution_due_at":
		return l.ResolutionDueAt
	}
	return time.Time{}
}

// LetterDueColumn names the column ActiveDueAt reads for a status — the one SetActiveDue writes. ""
// for a finished letter, which has no active deadline.
func LetterDueColumn(s LetterStatus) string {
	switch s {
	case LetterStatusNew, LetterStatusScreening:
		return "processing_due_at"
	case LetterStatusAdmitted, LetterStatusResolving:
		return "resolution_due_at"
	}
	return ""
}

// SetActiveDue is the clerk's "Hạn xử lý" (ADR 0079 lô 5 Q18, the prototype's PetitionDetailDrawer
// :519-523): ONE deadline, typed by a clerk, optional. It is written to the deadline of the letter's
// CURRENT phase — the column ActiveDueAt reads — so what the drawer shows, what the reminders compare
// and what the report counts are one value:
//
//	moi-vao-so · dang-xu-ly-don      processing_due_at
//	thu-ly · dang-giai-quyet         resolution_due_at (0006 binds it to `accepted_at`, set in these)
//	any finishing status             refused — a closed letter has no deadline to keep
//
// `due` is stored EXACTLY as given (rule 10, invariant 2: fixed at the act, never recomputed); zero
// clears it ("Không đặt"). Nothing here adds an hour or reads a calendar: the clerk's instant IS the
// commitment, which is why ADR 0064's working-day questions are not decided by this path.
//
// AFTER `thu-ly` THE PROCESSING DEADLINE STOPS APPLYING and the resolution one is "Không đặt" until a
// clerk sets it — TT 05/2021's two phases (C8), which 0006 already drew as two columns.
func (l CitizenLetter) SetActiveDue(due time.Time) (CitizenLetter, error) {
	if !due.IsZero() && (due.Year() < 2000 || due.Year() > 2200) {
		return l, ErrLetterDueTooFar
	}
	switch LetterDueColumn(l.Status) {
	case "processing_due_at":
		l.ProcessingDueAt = due
	case "resolution_due_at":
		l.ResolutionDueAt = due
	default:
		return l, ErrLetterDueOnFinished
	}
	return l, nil
}

// IsOverdue DERIVES lateness (rule 10, invariant 3). A letter with no deadline is never late — a clerk
// may leave the deadline unset ("Không đặt"), and no default stands in for it.
func (l CitizenLetter) IsOverdue(now time.Time) bool {
	due := l.ActiveDueAt()
	return !due.IsZero() && now.After(due)
}

// DaysOpen is C16/C17's "số ngày xử lý": calendar days from the day the letter was RECEIVED, that day
// counted, through the day it was closed — or today while it is open. ONE count for the list column
// and the report (C17). It measures time already spent and fixes no commitment, so rule 10's
// working-hour rule does not apply. Days are read in Asia/Ho_Chi_Minh (AutomationZone).
func (l CitizenLetter) DaysOpen(now time.Time) int {
	end := now
	if !l.ClosedAt.IsZero() {
		end = l.ClosedAt
	}
	return CalendarDaysInclusive(l.ReceivedDate, end.In(AutomationZone))
}

// CalendarDaysInclusive counts the calendar days from the date `from` carries to the date `to`
// carries, both counted, each read in its own location. Never below 1.
func CalendarDaysInclusive(from, to time.Time) int {
	n := int(dayNumber(to)-dayNumber(from)) + 1
	if n < 1 {
		return 1
	}
	return n
}

// dayNumber is the count of days since 1970-01-01 of the date t carries in its own location.
func dayNumber(t time.Time) int64 {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix() / 86400
}

// --- whistleblower protection and masking -----------------------------------------------------------

// LetterDisclosure is what ONE viewer may see of ONE letter's identity and summary. The edge masks
// from it and from nothing else, so the rule has one source.
type LetterDisclosure struct {
	Identity bool // sender name in full. The phone is ALWAYS masked and the address ALWAYS withheld
	Summary  bool // summary and result summary
}

// ListDisclosure is the rule for every list (register, duplicate warning): a denunciation carries
// neither identity nor summary, whoever is looking (ADR 0078 #4).
func ListDisclosure(t LetterType) LetterDisclosure {
	if t.ProtectsIdentity() {
		return LetterDisclosure{}
	}
	return LetterDisclosure{Identity: true, Summary: true}
}

// DetailDisclosure is the detail drawer's rule.
//
// A DENUNCIATION'S SENDER IS SHOWN TO ITS ASSIGNEE ALONE. The ledger also names holders of a NEW
// "xem danh tính tố cáo" key; that key is not seeded (rule 5, invariant 3c), so it does not exist here
// and nobody else sees the sender — fail closed.
//
// ITS SUMMARY IS SHOWN TO THE ASSIGNEE AND TO HOLDERS OF `petition.create`. This session's choice: the
// clerk who booked the letter typed the summary and must be able to check what was booked, and
// `petition.create` is the key that books and routes letters (C13). Every other `petition.read` holder
// sees number, dates, status and holder only. The caller audits every read that discloses either
// (rule 6, invariant 7).
func DetailDisclosure(l CitizenLetter, viewerCode string, viewerCanBook bool) LetterDisclosure {
	if !l.Type.ProtectsIdentity() {
		return LetterDisclosure{Identity: true, Summary: true}
	}
	isAssignee := viewerCode != "" && viewerCode == l.AssigneeCode
	return LetterDisclosure{Identity: isAssignee, Summary: isAssignee || viewerCanBook}
}

// --- validation -------------------------------------------------------------------------------------

// Length ceilings — 0006's CHECKs, counted in runes like their char_length.
const (
	MaxSenderName    = 200
	MaxSenderPhone   = 20
	MaxSenderAddress = 500
	MaxLetterSummary = 2000
	MaxLetterUnitID  = 64
	MaxStaffCode     = 64
	MaxLetterReason  = 2000
	MaxLetterNote    = 2000
	MaxResultNo      = 100
	MaxResultSigner  = 200
	MaxResultIssuer  = 300
	MaxResultSummary = 2000
	MaxLetterIDLen   = 64
	MaxLetterSearch  = 200
)

// TrimRequired trims a required field and refuses it empty or over its ceiling.
func TrimRequired(raw string, max int, whenEmpty *LetterError) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", whenEmpty
	}
	if utf8.RuneCountInString(s) > max {
		return "", ErrLetterFieldTooLong.withDetail(fmt.Sprintf("(tối đa %d ký tự)", max))
	}
	return s, nil
}

// TrimOptional trims an optional field; empty stays empty and the store writes it as NULL (0006
// refuses ” on every optional column, so `IS NULL` finds every absent value).
func TrimOptional(raw string, max int) (string, error) {
	s := strings.TrimSpace(raw)
	if utf8.RuneCountInString(s) > max {
		return "", ErrLetterFieldTooLong.withDetail(fmt.Sprintf("(tối đa %d ký tự)", max))
	}
	return s, nil
}

// TrimPhone is TrimOptional plus a character check. NO FORMAT IS IMPOSED BEYOND THE CHARACTERS: a
// letter may carry a landline, an extension or a number from abroad, and refusing those would push
// clerks to type them into the summary instead — where nothing masks them.
func TrimPhone(raw string) (string, error) {
	s, err := TrimOptional(raw, MaxSenderPhone)
	if err != nil {
		return "", err
	}
	for _, r := range s {
		if !(r >= '0' && r <= '9') && !strings.ContainsRune(" +-.()", r) {
			return "", ErrLetterPhoneInvalid
		}
	}
	return s, nil
}

// CheckReceivedDate refuses a received date the register cannot mean: missing, after today in
// Asia/Ho_Chi_Minh, or further back than the incoming register's bound (NgayDenSomNhat — a mistyped
// century). Compared by DAY: the column is a DATE.
func CheckReceivedDate(day, now time.Time) error {
	if day.IsZero() {
		return ErrLetterReceivedDateMissing
	}
	if dayNumber(day) > dayNumber(now.In(AutomationZone)) {
		return ErrLetterReceivedDateFuture
	}
	if day.Before(now.Add(-NgayDenSomNhat)) {
		return ErrLetterDateTooOld
	}
	return nil
}

// CheckResultDocumentDate refuses a result document dated after today, or absurdly far back.
func CheckResultDocumentDate(day, now time.Time) error {
	if day.IsZero() {
		return ErrLetterResultIncomplete
	}
	if dayNumber(day) > dayNumber(now.In(AutomationZone)) {
		return ErrLetterResultDateFuture
	}
	if day.Before(now.Add(-NgayDenSomNhat)) {
		return ErrLetterDateTooOld
	}
	return nil
}

// --- duplicate warning (C11) ------------------------------------------------------------------------

// DuplicateSimilarityThreshold is the summary similarity at or above which a candidate is shown.
//
// ⚠ DEBT: C11 says "ngưỡng là cấu hình" — the threshold is per-commune configuration. No such setting
// exists yet, so the prototype's figure (vigov-require router.py:315-334, pg_trgm 0.45) is a constant
// here. It only decides which candidates a WARNING lists — the server never links two letters on its
// own — so a wrong value costs a noisier or quieter warning, never a wrong record.
const DuplicateSimilarityThreshold = 0.45

// DuplicateMaxCandidates bounds the warning — the prototype's five.
const DuplicateMaxCandidates = 5

// SummarySimilarity approximates PostgreSQL pg_trgm's similarity(): lower-case, split into words of
// letters/digits/marks, pad each word with two spaces in front and one behind, take the SET of 3-rune
// grams, divide the shared grams by the union. pg_trgm is installed nowhere in this repository (0006
// header), so the measure runs here over the candidates the sender/date index narrowed to — kept close
// to pg_trgm so the prototype's 0.45 keeps its meaning.
//
// KNOWN LIMIT: no Unicode normalisation (the standard library has none), so a summary typed in NFD
// scores lower against the same text in NFC. Vietnamese keyboards emit NFC.
func SummarySimilarity(a, b string) float64 {
	ga, gb := trigrams(a), trigrams(b)
	if len(ga) == 0 || len(gb) == 0 {
		return 0
	}
	shared := 0
	for g := range ga {
		if _, ok := gb[g]; ok {
			shared++
		}
	}
	return float64(shared) / float64(len(ga)+len(gb)-shared)
}

func trigrams(s string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r))
	}) {
		rs := append([]rune("  "+w), ' ')
		for i := 0; i+3 <= len(rs); i++ {
			out[string(rs[i:i+3])] = struct{}{}
		}
	}
	return out
}

// DuplicateCandidate is one letter the warning lists.
type DuplicateCandidate struct {
	Letter     CitizenLetter
	Similarity float64
}

// RankDuplicates scores every candidate against the summary typed, keeps those at or above the
// threshold, and returns at most DuplicateMaxCandidates, most similar first (newer year/number breaks
// a tie, so two reads of one state answer in one order).
//
// A DENUNCIATION IS NEVER A CANDIDATE, whatever the type of the letter being booked: a match listed on
// a typed name already discloses that this person filed one (Luật Tố cáo 2018 Đ.8; ADR 0078 #4). The
// store excludes them in SQL; this is the second wall.
func RankDuplicates(summary string, candidates []CitizenLetter) []DuplicateCandidate {
	var out []DuplicateCandidate
	for _, c := range candidates {
		if c.Type.ProtectsIdentity() {
			continue
		}
		s := SummarySimilarity(summary, c.Summary)
		if s >= DuplicateSimilarityThreshold {
			out = append(out, DuplicateCandidate{Letter: c, Similarity: roundHundredth(s)})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Similarity != out[j].Similarity {
			return out[i].Similarity > out[j].Similarity
		}
		if out[i].Letter.Year != out[j].Letter.Year {
			return out[i].Letter.Year > out[j].Letter.Year
		}
		return out[i].Letter.Number > out[j].Letter.Number
	})
	if len(out) > DuplicateMaxCandidates {
		out = out[:DuplicateMaxCandidates]
	}
	return out
}

// --- report (Báo cáo tab) ---------------------------------------------------------------------------

// LetterReport is the year's figures, computed in Go from the rows with ONE definition per figure —
// the prototype's argument (vigov-require service.py:546-550): four aggregate queries would be four
// definitions of "on time" that drift apart.
//
// DEFINITIONS, stated once:
//
//	Received            booked in the year (the series year — the year of the booking act).
//	Resolved            `resolved_at` falls in the year: `da-giai-quyet` or `dinh-chi`.
//	ClosedInProcessing  booked in the year and ended in one of the four processing-phase outcomes —
//	                    neither resolved nor in progress; reported so the figures reconcile.
//	InProgress          not finished NOW, booked in the year or carried over from an earlier year.
//	PastDue             not finished, a deadline exists and is past now. DERIVED (rule 10). A letter
//	                    whose clerk set no deadline is never counted.
//	OnTimePercent       ADR 0084 #4 (REVERSES C12): among ALL letters resolved in the year, the share
//	                    resolved at or before their resolution deadline — A LETTER WITH NO DEADLINE
//	                    COUNTS AS ON TIME (prototype service.py:605-608). nil only when nothing was
//	                    resolved. The owner accepted that this inflates the rate in a commune that sets
//	                    no deadlines; figures before 08/10/2026 are not comparable.
//	AverageDays         C16: mean DaysOpen of letters that reached `da-giai-quyet` in the year — the
//	                    prototype averages its PROCESSED letters only, so `dinh-chi` is left out here.
//	                    Days counted INCLUSIVE (C16/C17, not reversed). nil when none.
//
// ByType and ByUnit are sorted by Total, largest first (ADR 0084 #4); a tie keeps C4's type order and
// the unit id order respectively. A unit's OnTimePercent follows the same rule over the letters it
// resolved, nil only when it resolved none (the web prints "—").
//
// A denunciation is COUNTED and never NAMED: the report carries no identity and no summary at all.
type LetterReport struct {
	Year               int
	Received           int
	Resolved           int
	ClosedInProcessing int
	InProgress         int
	PastDue            int
	OnTimePercent      *float64
	AverageDays        *float64
	ByType             []LetterReportTypeRow
	ByUnit             []LetterReportUnitRow
	ByMonth            []LetterReportMonthRow
}

type LetterReportTypeRow struct {
	Type       LetterType
	Total      int
	Resolved   int
	InProgress int
	PastDue    int
}

type LetterReportUnitRow struct {
	UnitID        string // "" = held by no unit
	Total         int
	InProgress    int
	Resolved      int
	PastDue       int
	OnTimePercent *float64
	onTime        int
}

type LetterReportMonthRow struct {
	Month    int
	Received int
	Resolved int
}

// BuildLetterReport computes the figures from the rows the store selected for the year: booked in
// it, or booked earlier and still open or resolved in it. Months are read in Asia/Ho_Chi_Minh.
func BuildLetterReport(year int, rows []CitizenLetter, now time.Time) LetterReport {
	r := LetterReport{Year: year}
	byType := map[LetterType]*LetterReportTypeRow{}
	byUnit := map[string]*LetterReportUnitRow{}
	var unitOrder []string
	months := make([]LetterReportMonthRow, 12)
	for i := range months {
		months[i].Month = i + 1
	}
	onTime, daysSum, daysN := 0, 0, 0

	for _, l := range rows {
		resolvedInYear := !l.ResolvedAt.IsZero() && l.ResolvedAt.In(AutomationZone).Year() == year
		bookedInYear := l.Year == year
		finished := l.Status.Finished()
		late := !finished && l.IsOverdue(now)

		t := byType[l.Type]
		if t == nil {
			t = &LetterReportTypeRow{Type: l.Type}
			byType[l.Type] = t
		}
		u := byUnit[l.HoldingUnitID]
		if u == nil {
			u = &LetterReportUnitRow{UnitID: l.HoldingUnitID}
			byUnit[l.HoldingUnitID] = u
			unitOrder = append(unitOrder, l.HoldingUnitID)
		}
		t.Total++
		u.Total++

		if bookedInYear {
			r.Received++
			months[int(l.CreatedAt.In(AutomationZone).Month())-1].Received++
			if finished && !l.Status.EndsResolution() {
				r.ClosedInProcessing++
			}
		}
		if resolvedInYear {
			r.Resolved++
			t.Resolved++
			u.Resolved++
			months[int(l.ResolvedAt.In(AutomationZone).Month())-1].Resolved++
			if l.Status == LetterStatusResolved {
				daysSum += l.DaysOpen(now)
				daysN++
			}
			if l.ResolutionDueAt.IsZero() || !l.ResolvedAt.After(l.ResolutionDueAt) {
				onTime++
				u.onTime++
			}
		}
		if !finished {
			r.InProgress++
			t.InProgress++
			u.InProgress++
		}
		if late {
			r.PastDue++
			t.PastDue++
			u.PastDue++
		}
	}

	r.OnTimePercent = percent(onTime, r.Resolved)
	if daysN > 0 {
		avg := roundTenth(float64(daysSum) / float64(daysN))
		r.AverageDays = &avg
	}
	for _, lt := range LetterTypes {
		if t := byType[lt]; t != nil {
			r.ByType = append(r.ByType, *t)
		}
	}
	sort.SliceStable(r.ByType, func(i, j int) bool { return r.ByType[i].Total > r.ByType[j].Total })
	sort.SliceStable(unitOrder, func(i, j int) bool {
		a, b := byUnit[unitOrder[i]], byUnit[unitOrder[j]]
		if a.Total != b.Total {
			return a.Total > b.Total
		}
		return a.UnitID < b.UnitID
	})
	for _, id := range unitOrder {
		u := byUnit[id]
		u.OnTimePercent = percent(u.onTime, u.Resolved)
		r.ByUnit = append(r.ByUnit, *u)
	}
	r.ByMonth = months
	return r
}

func percent(part, whole int) *float64 {
	if whole == 0 {
		return nil
	}
	p := roundTenth(float64(part) * 100 / float64(whole))
	return &p
}

func roundTenth(f float64) float64     { return float64(int64(f*10+0.5)) / 10 }
func roundHundredth(f float64) float64 { return float64(int64(f*100+0.5)) / 100 }
