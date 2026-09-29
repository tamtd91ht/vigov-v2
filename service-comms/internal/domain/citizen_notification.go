package domain

// The citizen notification ledger — the `CitizenNotification` entity declared on
// migrations/0004_thong_bao_gui_cong_dan.sql.
//
// WHY THIS TYPE IS IN domain/ AND NOT IN store/: the two rules that decide whether a
// notification is worth sending at all are pure — what the message must contain, and when two
// notifications are the same notification. Both have to be testable without a database, and
// both have to be impossible to bypass by writing a row directly, which is why the store takes
// these types rather than loose strings.
//
// domain/ IMPORTS NOTHING BUT THE STANDARD LIBRARY (rule 4 of the service pattern). encoding/json
// is standard library; core/privacy is not, which is why the masking of the recipient happens at
// the edge of the adapter and arrives here already masked.

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// SubjectType says which kind of business record a notification is about.
//
// ONE VALUE TODAY, and the CHECK constraint in migration 0004 lists exactly this one. A petition
// is the only object in this system a citizen creates and then watches (rule 10), so it is the
// only object that currently owes anybody a notification.
type SubjectType string

const SubjectCitizenReport SubjectType = "phieu-phan-anh"

// Channel is the channel the message goes out through.
//
// `zalo-zns` AND NOT `zalo`: ADR 0018 splits two roles of a Zalo Official Account that used to be
// one concept, and only one of them is a send path. The OA that authenticates the Mini App is a
// single platform-wide account; the OA that sends a citizen a message about their own record is
// the commune's own, and it reaches them through ZNS, by phone number, with no requirement that
// the citizen follows anything. Naming the channel after the product rather than the mechanism
// would lose that distinction exactly where somebody would later need it.
type Channel string

const ChannelZaloZNS Channel = "zalo-zns"

// SendStatus is where one notification stands. The four values are the CHECK constraint in
// migration 0004; the reason there are four rather than three is on that constraint.
type SendStatus string

const (
	// SendStatusPending — the obligation is recorded, the message has not gone out. This is the state a row
	// is born in, and it is born BEFORE any send is attempted: ADR 0006 consequence 4 makes
	// sending eventually consistent with the business write, so a petition is never refused
	// because a message could not leave.
	SendStatusPending SendStatus = "cho-gui"

	// SendStatusSent — the channel accepted it. Terminal, and the database refuses to move a row back out
	// of it: the citizen already has the message.
	SendStatusSent SendStatus = "da-gui"

	// SendStatusFailed — the channel refused it and the retries are exhausted. Staff must be able to see
	// this; a commitment that silently failed is indistinguishable from one nobody made.
	SendStatusFailed SendStatus = "that-bai"

	// SendStatusChannelNotConfigured — THE COMMUNE HAS NO OA CONFIGURED. A newly onboarded commune can take
	// petitions before its OA is approved (ADR 0018, §getPhoneNumber không đi qua OA), so its
	// citizens are owed notifications nobody can send yet. ADR 0006 consequence 3 requires that
	// to degrade VISIBLY rather than silently, and folding it into SendStatusFailed would put it in a
	// retry loop that can never succeed.
	SendStatusChannelNotConfigured SendStatus = "chua-cau-hinh-kenh"
)

// NotificationParams is EXACTLY what leaves this system inside a message, and the list is closed.
//
// RULE 3, INVARIANT 6: when data goes to an external service, only the fields actually needed
// travel, and they are declared in the adapter. This struct is that declaration. It is a struct
// and not a map deliberately — a map accepts a key somebody adds during an incident, and the
// field that gets added under pressure is the one carrying the citizen's name or the text of
// their petition, both of which are personal data under Decree 13/2023.
//
// NONE OF THE THREE IS PERSONAL DATA, and that is checkable rather than asserted:
//
//	LookupCode   the citizen's own lookup code. It identifies a record, not a person, and it
//	             is the one handle the citizen has on their case (rule 10, invariant 1)
//	StatusLabel  what changed, in words — "Đã chuyển xử lý". A status label, same for everyone
//	NextStep     what happens next. A sentence about the procedure, not about the person
//
// The Go names are the wire names of `vigov.petitions.v1.CitizenMessage` (`status_label`,
// `next_step`); the JSON keys are the `tham_so` contract of migration 0004 and the approved ZNS
// template, and do NOT follow the Go names.
type NotificationParams struct {
	LookupCode  string `json:"ma_tra_cuu"`
	StatusLabel string `json:"moc_nhan"`
	NextStep    string `json:"viec_tiep_theo"`
}

var (
	// ErrMissingLookupCode — a notification with no lookup code is a notification the citizen cannot
	// act on: it is the only handle they have on their case (rule 10, invariant 1, and
	// `skills/accessibility-elderly` §Submitting a petition #5).
	ErrMissingLookupCode = errors.New("thong_bao: thiếu mã tra cứu")

	// ErrMissingStatusLabel — nothing says what changed.
	ErrMissingStatusLabel = errors.New("thong_bao: thiếu mô tả việc đã đổi")

	// ErrMissingNextStep — nothing says what happens next.
	//
	// THIS IS THE ONE THAT WILL LOOK LIKE OVER-STRICTNESS, so the reason is written where the
	// refusal is. Rule 10, invariant 6 and `skills/petition-lifecycle` §Notifying the citizen both
	// refuse a bare "Đã xử lý": it names a state and no consequence, so the citizen learns nothing
	// they can do anything with — not whether to expect a visit, not whether the matter is over,
	// not who to ask. A channel that answers like that is a channel people stop reading, and a
	// channel nobody reads stops delivering the reports the commune actually needs.
	ErrMissingNextStep = errors.New("thong_bao: thiếu việc tiếp theo")

	// ErrNextStepRepeatsLabel — "what happens next" merely repeats "what changed".
	//
	// Without this, the previous refusal is trivially satisfied by copying one field into the
	// other, which produces "Đã xử lý. Đã xử lý." — worse than the message it was meant to
	// replace, and it would pass every test that only checked for emptiness.
	ErrNextStepRepeatsLabel = errors.New("thong_bao: việc tiếp theo chỉ lặp lại mốc")
)

// Validate refuses a message that does not carry the three things a citizen needs.
//
// Whitespace-only counts as absent: a template parameter of " " renders as an empty line, which
// is exactly the message this function exists to stop, wearing a value.
func (p NotificationParams) Validate() error {
	code := strings.TrimSpace(p.LookupCode)
	label := strings.TrimSpace(p.StatusLabel)
	next := strings.TrimSpace(p.NextStep)

	switch {
	case code == "":
		return ErrMissingLookupCode
	case label == "":
		return ErrMissingStatusLabel
	case next == "":
		return ErrMissingNextStep
	}
	// Compared case-insensitively and without surrounding space, because the copy that gets made
	// is a copy with the capitalisation changed.
	if strings.EqualFold(label, next) {
		return ErrNextStepRepeatsLabel
	}
	return nil
}

// JSON renders the parameters for the `tham_so` column.
//
// It validates FIRST and returns the error rather than an empty object: a caller that ignored a
// validation error somewhere upstream must not be handed a serialisable value it can still write.
// The CHECK constraint on the column catches the missing keys too, but a constraint violation
// arrives as a database error at the bottom of a transaction, with no explanation of which rule
// was broken or why it exists.
func (p NotificationParams) JSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	b, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("thong_bao: mã hoá tham số: %w", err)
	}
	return b, nil
}

// ErrKeyHasSeparator says a component of the deduplication key contains the separator.
var ErrKeyHasSeparator = errors.New("thong_bao: thành phần khoá chứa dấu phân cách")

// keySeparator separates the parts of the deduplication key.
//
// `|` is chosen because none of the value sets that go into the key can legitimately contain it:
// the codes are kebab-case Vietnamese without diacritics (ADR 0011), lookup codes avoid easily
// confused characters, and identity ids are ULIDs. SendKey refuses a component that does
// contain it rather than escaping — see there.
const keySeparator = "|"

// SendKey builds the deduplication key for one notification.
//
// IT IS A KEY OF THE FACT, NOT OF THE MESSAGE, and this is the decision worth arguing with.
//
// The obvious key is the id of the message that carried the fact. That deduplicates a
// REDELIVERY — the same message arriving twice, which queues guarantee will happen (rule 2,
// invariant 5) — and nothing else. It does not deduplicate a producer that published, crashed
// before recording that it had, and published again under a fresh id: two different messages,
// one fact, and the citizen gets told twice. On a channel a person reads, "told twice" is not a
// cosmetic defect; it is the commune looking like it does not know what it has already said.
//
// So the key is built from the business fact:
//
//	doi_tuong_loai | doi_tuong_ma | moc | lan | nguoi_nhan_ma | kenh
//
// THE RECIPE IS STORED DATA. Every key already written to `khoa_lan_gui` was produced by it, so a
// change to the order, the separator or the rendering of `lan` makes the next redelivery of an old
// fact look new — a duplicate message to a real person. TestSendKeyRecipeIsPinned fixes the exact
// output for one input.
//
// WHY `lan` IS IN IT, and dropping it is the subtle way to break this. Without it, the key says
// "this record reached this status", which is true more than once: ADR 0008 lets a citizen reopen
// a closed petition, so a petition can be closed, reopened and closed again. The second closing
// is a DIFFERENT commitment being met and owes a DIFFERENT notification — and a key without
// `lan` would silently swallow it, leaving a citizen who was told about the first closing and
// never about the second. That failure looks exactly like correct deduplication from the inside.
//
// WHY `nguoi_nhan_ma` IS IN IT: the same transition may owe messages to more than one recipient
// the day a petition can be watched by somebody other than its author (rule 4, stop condition 1
// is open). Leaving the recipient out would make the second recipient's message a duplicate of
// the first's.
//
// WHY `kenh` IS IN IT: a second channel is a second delivery, not a repeat of the first.
//
// IT REFUSES rather than escapes a component containing the separator. Escaping is the
// reasonable-looking choice and it is worse here: two different escaping conventions produce two
// different keys for one fact, and the drift shows up as duplicate messages to real people,
// months later, with the escaping function nowhere near the symptom. A refusal stops the write
// and names the component.
func SendKey(subjectType SubjectType, subjectCode, milestone string, round int,
	recipientCode string, channel Channel) (string, error) {

	parts := []string{string(subjectType), subjectCode, milestone, fmt.Sprintf("%d", round), recipientCode, string(channel)}
	for _, p := range parts {
		if strings.Contains(p, keySeparator) {
			return "", fmt.Errorf("%w: %q", ErrKeyHasSeparator, p)
		}
	}
	return strings.Join(parts, keySeparator), nil
}

// CitizenNotification is one row of the ledger.
//
// THE FIELDS THAT ARE ABSENT ARE THE DESIGN. There is no recipient phone number and no rendered
// message text, and migration 0004's PERSONAL DATA block says at length what it costs to add
// either. MaskedRecipient is the masked number and nothing else; RecipientCode is opaque.
type CitizenNotification struct {
	ID string // ULID, internal

	// SendKey is what makes a redelivery a no-op rather than a second message. Built by
	// SendKey, never assembled by hand at a call site.
	SendKey string

	SubjectType SubjectType
	SubjectCode string // the BUSINESS code — the citizen's lookup code, never an internal id

	// Milestone is the transition, stored as an OPAQUE value. `petitions` owns the list of statuses
	// (ADR 0027, nine values, closed); this service validates nothing against it, because a copy
	// of that list here would be a second source for one fact across a service boundary.
	Milestone string

	// Round is which occurrence of that transition on that record — see SendKey.
	Round int

	Channel Channel

	// RecipientCode is the OPAQUE citizen identity id. It identifies the recipient exactly and
	// means nothing to whoever reads a backup.
	RecipientCode string

	// MaskedRecipient is the MASKED number (core/privacy.MaskPhone), empty until a send is attempted
	// — the fact is not known before then. The database refuses any value with no mask character
	// in it, so an unmasked number cannot be stored here even by a statement typed by hand.
	MaskedRecipient string

	TemplateCode string
	Params       NotificationParams

	Status       SendStatus
	AttemptCount int

	// SentAt is when the channel accepted the message. Zero until then, and the database refuses
	// the combination Status == SendStatusSent with no time: "sent" with no date answers nothing.
	SentAt time.Time

	// ErrorCode is the PROVIDER'S ERROR CODE, never its message. A gateway's message quotes the
	// request back, and the request carries the recipient's number (rule 3, forbidden #3).
	ErrorCode string

	CreatedAt time.Time
}

var (
	// ErrResultStatusNotTerminal says the outcome being recorded is not an outcome.
	// `cho-gui` is where a row starts; writing it back as a RESULT would erase a real one.
	ErrResultStatusNotTerminal = errors.New("thong_bao: trạng thái không phải kết quả gửi")

	// ErrResultMissingSentAt says a delivery is being recorded with no time on it. "Sent" with no
	// date answers nothing when an inspection asks when the citizen was told, and the database
	// refuses the same combination.
	ErrResultMissingSentAt = errors.New("thong_bao: đã gửi nhưng thiếu mốc giờ")

	// ErrRecipientNotMasked says a recipient value arrived without a mask in it.
	//
	// THE SAME RULE IS IN THE DATABASE (migration 0004, thong_bao_gui_cong_dan_nguoi_nhan_phai_che)
	// AND THAT IS NOT A DUPLICATE WORTH REMOVING. The constraint is the one that holds against a
	// statement typed at a psql prompt; this one is the one that holds against a caller, names
	// the rule, and fails before a transaction is open. Removing either leaves the other covering
	// a different half. What they must never do is disagree, which is why both test for exactly
	// the mask character and nothing cleverer.
	ErrRecipientNotMasked = errors.New("thong_bao: số người nhận chưa được che")
)

// maskChar is the character core/privacy.MaskPhone puts in place of every hidden digit.
//
// domain/ MAY NOT IMPORT core/privacy (rule 4 of the service pattern: nothing but the standard
// library), so this is a restatement rather than a reference — and it is the narrowest possible
// one. It does not reimplement masking; it only asserts that masking HAPPENED. A raw Vietnamese
// mobile number contains no such character, so the check catches exactly the mistake it exists
// for and nothing else.
const maskChar = "*"

// SendResult is the outcome of one attempt to deliver a notification.
//
// It is a separate type from CitizenNotification because these are the ONLY fields a send may
// change: everything else about a notification is fixed when the obligation arises, and the
// database enforces that with a trigger. A single update type carrying every column would make
// "which fields may move" a question answered by whichever call site was written last.
type SendResult struct {
	// Status must be one of the three terminal states. SendStatusPending is refused — see
	// ErrResultStatusNotTerminal.
	Status SendStatus

	// SentAt is required when Status is SendStatusSent, and ignored otherwise.
	SentAt time.Time

	// MaskedRecipient is the MASKED number the message actually went to. Empty is allowed: a send
	// that never got as far as resolving a number — a commune with no OA — has no number to show.
	MaskedRecipient string

	// ErrorCode is the provider's error CODE. Never its message (rule 3, forbidden #3).
	ErrorCode string

	// AttemptCount is how many attempts have now been made. It is carried rather than incremented in
	// SQL so that a redelivery of the same outcome writes the same value instead of counting the
	// queue's retries as the channel's.
	AttemptCount int
}

// Validate refuses an outcome that is not an outcome.
func (r SendResult) Validate() error {
	switch r.Status {
	case SendStatusSent, SendStatusFailed, SendStatusChannelNotConfigured:
	default:
		return fmt.Errorf("%w: %q", ErrResultStatusNotTerminal, r.Status)
	}
	if r.Status == SendStatusSent && r.SentAt.IsZero() {
		return ErrResultMissingSentAt
	}
	if r.MaskedRecipient != "" && !strings.Contains(r.MaskedRecipient, maskChar) {
		// The value itself is NOT in the error: it is the very thing suspected of being an
		// unmasked phone number, and an error message travels into logs (rule 3, forbidden #3).
		return ErrRecipientNotMasked
	}
	if r.AttemptCount < 0 {
		return fmt.Errorf("thong_bao: số lần thử âm: %d", r.AttemptCount)
	}
	return nil
}
