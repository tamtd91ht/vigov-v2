package domain

// The two document registers a commune office keeps every day: SỔ VĂN BẢN ĐẾN and SỔ VĂN BẢN ĐI.
//
// THIS PACKAGE IMPORTS NOTHING BUT THE STANDARD LIBRARY (rule 4 of the service pattern). What lives
// here is the part of the register that is TRUE REGARDLESS OF STORAGE: what a valid entry looks
// like, which states exist and which moves between them are admissible, and how an entry is named
// in an audit trail.
//
// THE ONE RULE THIS FILE EXISTS TO PROTECT: an issued register number is never reissued and never
// renumbered (rule 7, invariant 3 and forbidden #4). Nothing here can allocate a number — the
// counter and its row lock are in store/ — but everything here refuses to let one be moved.

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// --- states -----------------------------------------------------------------------------------

// IncomingDocumentStatus is where an incoming document stands.
//
// THE SIX CODES ARE §3.2's, AND THE REASON THIS REGISTER MAY USE THEM IS §2: the incoming-document
// half of that screen "dùng chung khung sổ" with the citizen-letter half. THE SPECIFICATION ITSELF
// LISTS NO CODES FOR `van_ban_den` — §5 says "enum" and stops. That gap is reported to the user
// rather than closed quietly here, because these strings end up in an archival register: changing
// one later is a data migration, not a rename (the same line ADR 0027 draws for petitions).
type IncomingDocumentStatus string

const (
	IncomingStatusRegistered       IncomingDocumentStatus = "moi-vao-so"
	IncomingStatusAssigned         IncomingDocumentStatus = "da-phan-cong"
	IncomingStatusInProgress       IncomingDocumentStatus = "dang-xu-ly"
	IncomingStatusResolved         IncomingDocumentStatus = "da-giai-quyet"
	IncomingStatusReferred         IncomingDocumentStatus = "chuyen-cap-tren"
	IncomingStatusFiledNotAdmitted IncomingDocumentStatus = "luu-khong-thu-ly"
)

// IsFinished reports whether the register has finished with this document.
//
// THE THREE CLOSING CODES ARE §3.2's OWN GROUPING: `da-giai-quyet` ends the flow, `chuyen-cap-tren`
// and `luu-khong-thu-ly` branch out of it. All three mean the commune is no longer processing the
// document, which is what CanRoute below needs to know.
func (s IncomingDocumentStatus) IsFinished() bool {
	switch s {
	case IncomingStatusResolved, IncomingStatusReferred, IncomingStatusFiledNotAdmitted:
		return true
	default:
		return false
	}
}

// --- urgency ----------------------------------------------------------------------------------

// Urgency is `do_khan`. §5 of the specification says "enum" and never lists the values; these four
// are the statutory list of Nghị định 30/2020/NĐ-CP. REPORTED AS AN ASSUMPTION, not decided here.
//
// THE EMPTY STRING IS A REAL ANSWER and reaches the database as NULL: "the commune did not record
// an urgency" is not the same statement as "Thường". A default of `thuong` would put a classification
// on a government document that nobody chose.
type Urgency string

const (
	UrgencyNotRecorded Urgency = ""
	UrgencyNormal      Urgency = "thuong"
	UrgencyUrgent      Urgency = "khan"
	UrgencyVeryUrgent  Urgency = "thuong-khan"
	UrgencyImmediate   Urgency = "hoa-toc"
)

// --- the records ------------------------------------------------------------------------------

// IncomingDocument is one entry in the incoming register.
//
// `DueAt` IS AN INSTANT THAT WAS FIXED ONCE, at the act that booked the document, from this
// commune's SLA row and this commune's calendar (rule 10, invariant 2; ADR 0007). NOTHING IN THIS
// PACKAGE COMPUTES IT and nothing recomputes it on read: identity owns that arithmetic, it is asked
// over gRPC by the use case, and the answer is stored. Moving this value later silently moves a
// commitment the authority has already made.
//
// THERE IS NO OVERDUE FIELD AND THERE MUST NEVER BE ONE — rule 10, invariant 3 and forbidden #1.
// Overdue is DERIVED, by IsOverdue below, from the stored deadline against the current instant. A
// stored flag is a second copy of a derivable fact, and the stale copy is the one that reaches the
// figure reported upward.
type IncomingDocument struct {
	ID string

	// ArrivalNo and Year are the ISSUED NUMBER. Allocated once, under a row lock, and immutable
	// afterwards — the database refuses to change either (migration 0004, `so_van_ban_bat_bien`).
	ArrivalNo int
	Year      int

	ReceivedDate     time.Time // the day it arrived at the commune
	ReferenceNo      string    // "1742-CV/BTCTU" — the issuing body's own number, optional
	DocumentDate     time.Time // the day the issuing body signed it, optional
	IssuingBody      string
	DocumentType     string // the catalogue CODE, e.g. "cong-van"
	Summary          string
	Urgency          Urgency
	HoldingOrgUnitID string // identity's `bo_phan.id`, optional — not validated here (rule 2)
	AssigneeCode     string // a STAFF BUSINESS CODE, never an internal id (rule 6, invariant 8)
	DueAt            time.Time
	Status           IncomingDocumentStatus
	CreatedBy        string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// OutgoingDocument is one entry in the outgoing register.
//
// ⚠ NO SPECIFICATION EXISTS FOR THIS RECORD. docs/ui-ux/05-van-ban-don-thu.md describes the
// incoming register and the citizen-letter register and says nothing at all about outgoing
// documents; the only written source is kb/00-foundation/ubiquitous-language.md:47 (the concept),
// :143 (the URL resource `outgoing-documents`) and :48 (numbering per body, restarting at 01 each
// year). EVERY FIELD BELOW IS THIS SESSION'S READING of what a commune's outgoing register holds,
// and it is reported as such rather than presented as the customer's answer.
//
// THERE IS NO STATE FIELD. An outgoing document is issued when it takes its number; inventing
// `nhap` -> `da-ky` -> `da-phat-hanh` would be inventing a workflow a commune then has to follow.
type OutgoingDocument struct {
	ID string

	// IssuedNo and Year are the ISSUED NUMBER, and this one is on paper outside the commune.
	IssuedNo int
	Year     int

	DocumentDate time.Time
	DocumentType string
	Summary      string
	Recipient    string
	Signer       string // the name under the seal, as free text — see the migration
	CreatedBy    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// DocumentRouting is one line of "Dòng thời gian chuyển tiếp" (§3.5).
//
// IT IS A HISTORICAL RECORD. Once written it is never edited and never removed — the database
// refuses both (rule 7, forbidden #5). A correction is a NEW routing whose reason says so.
type DocumentRouting struct {
	ID                 string
	IncomingDocumentID string
	RoutedAt           time.Time
	RoutedBy           string // a STAFF BUSINESS CODE
	StatusAtTime       IncomingDocumentStatus
	FromOrgUnitID      string // empty for the first routing: nobody was holding the file
	ToOrgUnitID        string
	AssigneeCode       string // optional — "— Để bộ phận tự phân công —"
	Instruction        string

	// CreatedAt is read back only. The INSERT leaves it to the column default, so on the write path
	// it is zero; RoutedAt is the instant of the act.
	CreatedAt time.Time
}

// --- derived facts ----------------------------------------------------------------------------

// IsOverdue DERIVES whether the commitment on this document has been missed.
//
// A FUNCTION AND NOT A COLUMN (rule 10, invariant 3). A stored `is_overdue` is wrong the moment a
// nightly job is late, the clock skews or a holiday is added, and nothing on the screen says so.
// Derived, it cannot drift: there is one deadline and one comparison.
//
// A DOCUMENT THE REGISTER HAS FINISHED WITH IS NEVER OVERDUE. `da-giai-quyet` means the commune did
// the work; whether it did so in time is a different question, answered by comparing the deadline
// with the moment it was settled — which this register does not yet store, and which is reported
// rather than guessed at here.
func (d IncomingDocument) IsOverdue(now time.Time) bool {
	if d.Status.IsFinished() {
		return false
	}
	return now.After(d.DueAt)
}

// IncomingDocumentCode is the business code this entry is filed under in the audit trail.
//
// WHY A COMPOSED STRING AND NOT THE ULID: rule 6, invariant 8. `audit_log.subject` is read years
// later, by somebody handling a complaint or an inspection, and "VB-DEN-2026-0007" names a row of a
// register they can open. A ULID names nothing, and the two are indistinguishable on sight.
//
// THE FORMAT IS THIS SESSION'S — no source in this repository names one — and it is chosen so that
// the two registers can never collide in the same column, which is why `DEN` and `DI` are in it.
// Zero-padded to four digits so the strings sort the way the numbers do.
//
// THE FORMAT STRING IS AN ISSUED CODE ALREADY WRITTEN INTO `audit_log.subject`, and it survives the
// English rename campaign unchanged (ADR 0061): TestDocumentCodes_TheTwoRegistersNeverShareOne pins
// the exact output for a fixed input.
func IncomingDocumentCode(year, no int) string { return fmt.Sprintf("VB-DEN-%d-%04d", year, no) }

// OutgoingDocumentCode is the same for the outgoing register.
func OutgoingDocumentCode(year, no int) string { return fmt.Sprintf("VB-DI-%d-%04d", year, no) }

// --- refusals ---------------------------------------------------------------------------------

// The input refusals. EACH ONE IS A SENTENCE A CLERK CAN ACT ON, in Vietnamese, naming the field
// and what is wrong with it — the handler returns these verbatim (400) rather than writing a second
// copy that would drift.
var (
	ErrMissingReceivedDate       = errors.New("văn bản đến: thiếu ngày đến")
	ErrReceivedDateInFuture      = errors.New("văn bản đến: ngày đến ở tương lai")
	ErrReceivedDateTooOld        = errors.New("văn bản đến: ngày đến quá xa trong quá khứ")
	ErrDocumentDateAfterReceived = errors.New("văn bản đến: ngày ký văn bản sau ngày đến")
	ErrMissingDocumentDate       = errors.New("văn bản đi: thiếu ngày văn bản")
	ErrDocumentDateInFuture      = errors.New("văn bản đi: ngày văn bản ở tương lai")

	ErrMissingIssuingBody  = errors.New("văn bản đến: thiếu cơ quan ban hành")
	ErrMissingRecipient    = errors.New("văn bản đi: thiếu nơi nhận")
	ErrMissingDocumentType = errors.New("văn bản: thiếu loại văn bản")
	ErrMissingSummary      = errors.New("văn bản: thiếu trích yếu")
	ErrInvalidUrgency      = errors.New("văn bản đến: độ khẩn không hợp lệ")

	ErrMissingTargetOrgUnit = errors.New("chuyển văn bản: chưa chọn bộ phận nhận")
	ErrMissingRoutingReason = errors.New("chuyển văn bản: thiếu lý do chuyển")
	ErrMissingRemovalReason = errors.New("văn bản: thiếu lý do gỡ")

	ErrFieldTooLong = errors.New("văn bản: nội dung một trường vượt độ dài cho phép")

	// ErrClientSuppliedNumber — a request that named its own register number.
	//
	// IT IS REFUSED, NOT IGNORED, and that is the difference between a clerk learning that the
	// number is the register's to give and a clerk watching the number they typed disappear. A
	// client that could set it could reissue a number that is already on a sealed document.
	ErrClientSuppliedNumber = errors.New("văn bản: số vào sổ / số đi do hệ thống cấp, không nhận từ client")

	// ErrClientSuppliedStatus — a request that named its own state. The state moves by routing and
	// by nothing else on this surface.
	ErrClientSuppliedStatus = errors.New("văn bản đến: trạng thái do luồng xử lý quyết định, không nhận từ client")

	// ErrClientSuppliedDeadline — a request that named its own deadline. THE COMMITMENT IS THE
	// COMMUNE'S SLA AND ITS CALENDAR, computed once by identity (rule 10, invariant 2 and forbidden
	// #3). A client choosing its own deadline is a client choosing how long the authority may take.
	ErrClientSuppliedDeadline = errors.New("văn bản đến: hạn xử lý tính theo cấu hình thời hạn của xã, không nhận từ client")

	// ErrDocumentFinished — routing a document the register has finished with.
	//
	// 409 AND NOT 403: the caller holds `document.route` and is allowed to route documents. What is
	// refused is this act on THIS document, because of the state it is in.
	ErrDocumentFinished = errors.New("văn bản đến: văn bản đã kết thúc xử lý nên không chuyển tiếp được")
)

// --- validation -------------------------------------------------------------------------------

// The length ceilings. THEY ARE BOUNDS ON A SHARED RESOURCE, not opinions about how much somebody
// should type: one process serves 200+ communes, and an unbounded TEXT field is memory a client
// chooses. Counted in RUNES, not bytes — a Vietnamese summary is two to three bytes per character,
// and a ceiling counted in bytes would refuse a legitimate sentence at a third of its apparent
// length.
const (
	MaxSummaryLen       = 2000
	MaxIssuingBodyLen   = 300
	MaxReferenceNoLen   = 100
	MaxDocumentTypeLen  = 100
	MaxRecipientLen     = 500
	MaxSignerLen        = 200
	MaxRoutingReasonLen = 1000
	MaxRemovalReasonLen = 500
	MaxOrgUnitIDLen     = 64
	MaxStaffCodeLen     = 64
)

// MaxReceivedDateAge bounds how far back a document may be booked.
//
// WHY A BOUND AT ALL: `ngay_den` is typed by a person and a mistyped year ("2006" for "2026") puts
// a document twenty years back in a register ordered by number and filtered by year. Ten years is
// past any plausible correction and far short of a typo of the century digit.
const MaxReceivedDateAge = 10 * 365 * 24 * time.Hour

// NormalizeRequired trims a required free-text field and refuses it empty or over its ceiling.
func NormalizeRequired(raw string, max int, errEmpty error) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", errEmpty
	}
	if utf8.RuneCountInString(s) > max {
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrFieldTooLong, max)
	}
	return s, nil
}

// NormalizeOptional is the same for an OPTIONAL field: empty is a legitimate answer and stays empty,
// which the store writes as NULL rather than as ”. One absent value with two spellings in one
// column makes `WHERE ... IS NULL` silently miss half the rows.
func NormalizeOptional(raw string, max int) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", nil
	}
	if utf8.RuneCountInString(s) > max {
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrFieldTooLong, max)
	}
	return s, nil
}

// ValidateUrgency refuses an urgency outside the statutory list. The empty string passes: see Urgency.
func ValidateUrgency(u Urgency) error {
	switch u {
	case UrgencyNotRecorded, UrgencyNormal, UrgencyUrgent, UrgencyVeryUrgent, UrgencyImmediate:
		return nil
	default:
		return ErrInvalidUrgency
	}
}

// ValidateReceivedDate refuses a date the register cannot mean.
//
// A FUTURE ARRIVAL DATE IS REFUSED, and the reason is the deadline rather than tidiness: a document
// booked with tomorrow's date takes a commitment counted from today, so the two disagree on the
// screen from the first minute. `now` is a parameter rather than time.Now() so a test can pin
// the day — a validation whose outcome depends on when the suite runs is a validation nobody trusts.
func ValidateReceivedDate(date, now time.Time) error {
	if date.IsZero() {
		return ErrMissingReceivedDate
	}
	// Compared by DAY, not by instant: `ngay_den` is a DATE and arrives as midnight UTC, so an
	// instant comparison would refuse a document booked this morning in Vietnam.
	if date.After(endOfDay(now)) {
		return ErrReceivedDateInFuture
	}
	if date.Before(now.Add(-MaxReceivedDateAge)) {
		return ErrReceivedDateTooOld
	}
	return nil
}

// ValidateIncomingDocumentDate bounds the OPTIONAL date on the document itself. A document cannot
// arrive before it was signed; the reverse — signed long before it arrived — is ordinary post.
func ValidateIncomingDocumentDate(documentDate, receivedDate time.Time) error {
	if documentDate.IsZero() {
		return nil
	}
	if documentDate.After(endOfDay(receivedDate)) {
		return ErrDocumentDateAfterReceived
	}
	return nil
}

// ValidateOutgoingDocumentDate bounds the date an outgoing document carries. It is REQUIRED — an
// issued document without a date is not a document — and it may not be in the future: the number is
// allocated at the moment of issue.
func ValidateOutgoingDocumentDate(date, now time.Time) error {
	if date.IsZero() {
		return ErrMissingDocumentDate
	}
	if date.After(endOfDay(now)) {
		return ErrDocumentDateInFuture
	}
	if date.Before(now.Add(-MaxReceivedDateAge)) {
		return ErrReceivedDateTooOld
	}
	return nil
}

// endOfDay is the last instant of the day `t` falls in, UTC. It is what makes the comparisons above
// about DAYS rather than about the hour the request happened to arrive.
func endOfDay(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 23, 59, 59, int(time.Second-1), time.UTC)
}

// CanRoute reports whether this document may be routed to a department.
//
// THE REFUSAL IS ABOUT THE DOCUMENT, NOT ABOUT THE PERSON — hence 409 rather than 403 at the edge.
// A document the commune has settled (`da-giai-quyet`), passed upward (`chuyen-cap-tren`) or filed
// without taking up (`luu-khong-thu-ly`) is not routed further: doing so would reopen a closed
// record by a side effect of a routing form.
func (d IncomingDocument) CanRoute() error {
	if d.Status.IsFinished() {
		return fmt.Errorf("%w (trạng thái hiện tại: %s)", ErrDocumentFinished, d.Status)
	}
	return nil
}

// StatusAfterRouting is the state a routing moves the document INTO.
//
// IT ONLY EVER MOVES FORWARD. A brand-new entry becomes `da-phan-cong` — that is what assigning it
// to a department means. A document already being worked on KEEPS ITS STATE: re-routing a
// `dang-xu-ly` document back to `da-phan-cong` would undo a fact somebody recorded, and the
// timeline would then show the work going backwards.
//
// THE CLOSING CODES NEVER REACH HERE — CanRoute refuses them first.
func StatusAfterRouting(current IncomingDocumentStatus) IncomingDocumentStatus {
	if current == IncomingStatusRegistered {
		return IncomingStatusAssigned
	}
	return current
}
