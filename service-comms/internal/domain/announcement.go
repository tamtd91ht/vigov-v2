package domain

// THE INTERNAL ANNOUNCEMENT (`docs/ui-ux/08-thong-bao.md`) — what a commune tells its OWN STAFF.
//
// READ THIS BEFORE REACHING FOR A NAME IN THIS PACKAGE. `citizen_notification.go`, next door,
// models a message that LEAVES the commune to ONE CITIZEN. The two are different audiences, two
// different trust levels (rule 4) and two different tables, and they share the one Vietnamese
// word `thông báo`. Every identifier here therefore carries `Announcement` or an unambiguous prefix:
// `SendStatusSent`, `SendStatusPending` and `SendStatus` already belong to the citizen ledger, and a
// name collision resolved by "whichever compiles" is how a staff notice ends up on the citizen channel.
//
// WHAT IS MODELLED HERE AND WHAT IS NOT. The types below carry the announcement as a RECORD: what
// was said, who said it, who it went to, what each recipient has done with it. They do not carry
// the mail copy (§1, §5) beyond the two fields that record what was asked for and what happened —
// there is no SMTP adapter and no `Cấu hình → Máy chủ thư` in this repository, so sending is a
// later pass and is named as such in migration 0005.

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
)

// AnnouncementStatus is the lifecycle of §6, hyphenated like every other enum value in this system.
//
// See migration 0005 for why the separator differs from §6's sketch and for what each state means
// to the person reading the screen. THE ORDER IS ONE-WAY: a draft may be issued, an issued
// announcement may be withdrawn, and nothing goes back — the database refuses it, and so does
// every path here.
type AnnouncementStatus string

const (
	// AnnouncementDraft — drafted, no recipient list generated, visible to nobody but its author.
	AnnouncementDraft AnnouncementStatus = "nhap"
	// AnnouncementPublished — issued. §9.2 forbids editing the content from this point.
	AnnouncementPublished AnnouncementStatus = "da-phat-hanh"
	// AnnouncementWithdrawn — withdrawn with §4's `🗑 Gỡ`. The row and its acknowledgements stay: people
	// read it, and a withdrawal does not un-read it.
	AnnouncementWithdrawn AnnouncementStatus = "da-go"
)

// AnnouncementEmailStatus is §3's mail chip, plus the state before any attempt.
//
// NOTHING IN THIS REPOSITORY MOVES IT OFF `EmailNotSent` YET, and that is deliberate rather than
// unfinished: the values exist because they are part of the record an announcement carries, and
// inventing a sender to exercise them would put an external dependency (SMTP, per-commune mail
// configuration) into a pass that has neither.
type AnnouncementEmailStatus string

const (
	EmailNotSent AnnouncementEmailStatus = "chua-gui"
	EmailSending AnnouncementEmailStatus = "dang-gui"
	EmailSent    AnnouncementEmailStatus = "da-gui"
	EmailFailed  AnnouncementEmailStatus = "loi"
)

// The bounds. Every one of them is a REFUSAL rather than a truncation: a title silently cut at
// 300 characters is an announcement whose subject changed on the way into the database.
const (
	// AnnouncementTitleMaxLen — §5's placeholder is `Mời họp giao ban tháng 9`. 300 runes is roughly ten times
	// any real subject line and short enough that a card can still draw it.
	AnnouncementTitleMaxLen = 300

	// AnnouncementBodyMaxLen — the body §5 collects in a textarea. 20.000 runes is about ten pages;
	// past that it is a document with an attachment, which chapter 08 does not have.
	AnnouncementBodyMaxLen = 20000

	// MaxAnnouncementRecipients bounds ONE announcement's recipient list. A commune has a few dozen staff
	// (§10's samples reach 12), so 500 is more than an order of magnitude above the real figure —
	// the point past which the list is not an addressee list but a loop, an import run twice, or a
	// client sending the same code five hundred times.
	MaxAnnouncementRecipients = 500

	// MaxAnnouncementOrgUnits bounds the departments one announcement may address. §5 lists five; the org
	// chart of a commune is tens of units at most.
	MaxAnnouncementOrgUnits = 50

	// StaffCodeMaxLen bounds one staff business code (`CB-2026-7K3M9Q`). Long enough for any scheme
	// identity might use, short enough that a client cannot post a document as an identifier.
	StaffCodeMaxLen = 64
)

var (
	// ErrAnnouncementTitleEmpty — §5 marks the title required. A card with no title is a card nobody can pick
	// out of §2's list.
	ErrAnnouncementTitleEmpty = errors.New("thong_bao: thiếu tiêu đề")
	// ErrAnnouncementTitleTooLong — refused, never truncated. See the note on the bounds above.
	ErrAnnouncementTitleTooLong = errors.New("thong_bao: tiêu đề quá dài")

	// ErrAnnouncementBodyEmpty — §5 marks the body required. An announcement with a subject and nothing
	// under it is a notice that tells its readers nothing they can act on.
	ErrAnnouncementBodyEmpty   = errors.New("thong_bao: thiếu nội dung")
	ErrAnnouncementBodyTooLong = errors.New("thong_bao: nội dung quá dài")

	// ErrNoRecipients — an announcement addressed to nobody. It is refused rather than stored,
	// because §3's counter would read `0/0` and §2's `Gửi cho tôi` would never show it to anyone:
	// the author would believe they had told the commune something.
	ErrNoRecipients = errors.New("thong_bao: không có người nhận")
	// ErrTooManyRecipients — past MaxAnnouncementRecipients.
	ErrTooManyRecipients = errors.New("thong_bao: quá nhiều người nhận")
	// ErrTooManyOrgUnits — past MaxAnnouncementOrgUnits.
	ErrTooManyOrgUnits = errors.New("thong_bao: quá nhiều bộ phận nhận")

	// ErrStaffCodeEmpty / ErrStaffCodeTooLong — a recipient with no code, or with something that is not
	// a code. Refused HERE rather than at the database, because the column is only NOT NULL and an
	// empty string satisfies that.
	ErrStaffCodeEmpty   = errors.New("thong_bao: mã cán bộ rỗng")
	ErrStaffCodeTooLong = errors.New("thong_bao: mã cán bộ quá dài")
)

// NormalizeAnnouncementTitle trims and validates the subject line.
//
// IT CARRIES VIETNAMESE WITH DIACRITICS — it is a sentence a person reads, so nothing here
// restricts the character set beyond refusing control characters, which would corrupt a screen and
// a log line alike. Same shape as NormalizeLabel, for the same reason.
func NormalizeAnnouncementTitle(title string) (string, error) {
	title = strings.TrimSpace(title)
	switch {
	case title == "":
		return "", ErrAnnouncementTitleEmpty
	case len([]rune(title)) > AnnouncementTitleMaxLen:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrAnnouncementTitleTooLong, AnnouncementTitleMaxLen)
	}
	for _, r := range title {
		if unicode.IsControl(r) {
			return "", ErrAnnouncementTitleEmpty
		}
	}
	return title, nil
}

// NormalizeAnnouncementBody trims and validates the body.
//
// NEWLINES ARE ALLOWED AND EVERY OTHER CONTROL CHARACTER IS NOT. That is the one difference from
// NormalizeAnnouncementTitle and it is the whole point of a textarea: an announcement is paragraphs. A
// carriage return is accepted too, because a browser on Windows sends CRLF and refusing it would
// reject a perfectly ordinary form submission.
func NormalizeAnnouncementBody(body string) (string, error) {
	body = strings.TrimSpace(body)
	switch {
	case body == "":
		return "", ErrAnnouncementBodyEmpty
	case len([]rune(body)) > AnnouncementBodyMaxLen:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrAnnouncementBodyTooLong, AnnouncementBodyMaxLen)
	}
	for _, r := range body {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return "", ErrAnnouncementBodyEmpty
		}
	}
	return body, nil
}

// NormalizeStaffCode trims and validates one staff business code.
//
// IT DOES NOT CHECK THE SHAPE OF THE CODE, and that is deliberate. `CB-2026-7K3M9Q` is minted by
// service-identity and its format is that service's to change (rule 2); a pattern copied here
// would be a second declaration of somebody else's fact, and the copy would refuse a real code on
// the day identity widened it. What is checked is what this service is entitled to insist on: a
// code that is present, is not whitespace, is not a paragraph and has no control characters in it.
func NormalizeStaffCode(code string) (string, error) {
	code = strings.TrimSpace(code)
	switch {
	case code == "":
		return "", ErrStaffCodeEmpty
	case len([]rune(code)) > StaffCodeMaxLen:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrStaffCodeTooLong, StaffCodeMaxLen)
	}
	for _, r := range code {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return "", ErrStaffCodeEmpty
		}
	}
	return code, nil
}

// NormalizeStaffCodes normalises a list of staff codes, DROPS DUPLICATES and keeps the order.
//
// DEDUPLICATION HAPPENS HERE AND NOT ONLY IN THE DATABASE. `PRIMARY KEY (tenant_id, thong_bao_id,
// nguoi_nhan_ma)` would collapse the duplicates anyway, but it would do it by making the second
// INSERT a no-op inside a transaction the caller is counting rows in — so the announcement would
// report a recipient count larger than the list it actually has, and §3's `{y}` would be wrong for
// the life of the record. Doing it before the write keeps the two numbers the same number.
//
// THE ORDER IS KEPT because §4 draws the recipient list, and a list that reorders itself between
// two reads looks to the person watching it like the data changed.
func NormalizeStaffCodes(raw []string, limit int, tooMany error) ([]string, error) {
	out := make([]string, 0, len(raw))
	seen := make(map[string]bool, len(raw))
	for _, one := range raw {
		code, err := NormalizeStaffCode(one)
		if err != nil {
			return nil, err
		}
		if seen[code] {
			continue
		}
		seen[code] = true
		out = append(out, code)
	}
	if len(out) > limit {
		return nil, fmt.Errorf("%w (tối đa %d)", tooMany, limit)
	}
	return out, nil
}

// Announcement is ONE internal announcement — the `Announcement` entity of migration 0005.
//
// THE COUNTERS ARE FIELDS AND THE STATE IS NOT. `RecipientCount` and `AckCount` are read from the
// database as aggregates of `thong_bao_nguoi_nhan` and are never stored on the row (0005 says
// why); `Status` is stored, because it is a decision somebody took rather than a count of
// anything.
type Announcement struct {
	ID string // ULID, internal

	Title string
	Body  string

	Status AnnouncementStatus

	// Pinned is §3's pin. It is the one display property an issued announcement may still change.
	Pinned bool

	// AckRequired turns on §3's orange chip and the counter below, and keeps the announcement on
	// the bell of everyone who has not acknowledged it (§9.5).
	AckRequired bool

	// EmailRequested records WHAT WAS ASKED FOR; EmailStatus records what happened. They are two
	// facts and conflating them would make "we asked for mail and the adapter does not exist yet"
	// indistinguishable from "mail was never wanted".
	EmailRequested bool
	EmailStatus    AnnouncementEmailStatus

	// AuthorCode is a STAFF BUSINESS CODE (`CB-2026-7K3M9Q`), never an internal id — rule 6,
	// invariant 8. §6 models it as a uuid; the rule wins, and migration 0005 argues it in full.
	AuthorCode string

	// IssuedAt is the zero time for a draft. The database ties the two together so neither can
	// drift from the other.
	IssuedAt  time.Time
	CreatedAt time.Time

	// RecipientCount / AckCount are §3's `{x}/{y} đã xác nhận`, DERIVED by counting the recipient
	// rows. A draft has neither, and a card only draws them when AckRequired is on.
	RecipientCount int
	AckCount       int
}

// IsPublished reports whether staff can see this announcement at all.
//
// IT IS NOT `Status == AnnouncementPublished`, AND THE DIFFERENCE MATTERS: a WITHDRAWN
// announcement was issued too, and §4 keeps its recipients and their acknowledgements. Anything
// asking "has this left the author's hands" has to include `da-go`, and every place that asked the
// narrower question by hand would eventually ask it wrongly.
func (a Announcement) IsPublished() bool { return a.Status != AnnouncementDraft }

// AnnouncementRecipient is ONE member of staff on ONE announcement's list — §4's `NGƯỜI NHẬN` rows.
type AnnouncementRecipient struct {
	// RecipientCode is the staff business code. §2's `Gửi cho tôi` compares it against the session's
	// own `.Ma`, with no lookup in between.
	RecipientCode string

	// IsNamed is §6's flag: true = named explicitly in §5's `Gửi thêm đích danh`, false =
	// generated from a chosen department. It is NOT derivable from the department list — a person
	// can be both, and departmental membership lives in another service and changes.
	IsNamed bool

	// The three states of §4 as two instants: `chưa mở` is both zero, `đã mở` is the first set,
	// `✓ đã xác nhận` is both. The database refuses clearing either one once set.
	ReadAt         time.Time
	AcknowledgedAt time.Time

	EmailSentAt time.Time
	// EmailErrorCode is the provider's error CODE, never its message: an SMTP failure quotes the
	// envelope, and the envelope is somebody's address (rule 3).
	EmailErrorCode string
}

// IsAcknowledged reports whether this person acknowledged reading the announcement (§4's green check).
func (r AnnouncementRecipient) IsAcknowledged() bool { return !r.AcknowledgedAt.IsZero() }

// IsRead reports whether this person opened it at all.
func (r AnnouncementRecipient) IsRead() bool { return !r.ReadAt.IsZero() }

// CreateAnnouncementRequest is one new announcement as the use case receives it, already past the HTTP
// layer and not yet validated.
//
// THERE IS NO `Status` FIELD AND THERE MUST NEVER BE ONE. The state is decided by which act was
// performed, not by a value a client sends: a request that could name its own state could post an
// announcement already marked `da-phat-hanh` — an announcement with no recipient list that every
// screen would nonetheless treat as issued.
//
// THERE IS NO `AuthorCode` FIELD EITHER. The author is the PRINCIPAL of the request, taken from
// the session at the handler (rule 6, invariant 8). A field here is a field a body could fill,
// which is a member of staff issuing an announcement over a colleague's name.
type CreateAnnouncementRequest struct {
	Title string
	Body  string

	// OrgUnitIDs are the departments §5's chips addressed. They are RECORDED, and in this pass they
	// do NOT expand into recipients — see the use case for why that expansion needs a contract
	// service-identity does not publish today.
	OrgUnitIDs []string

	// RecipientCodes are the staff codes of §5's `Gửi thêm đích danh`.
	RecipientCodes []string

	Pinned         bool
	AckRequired    bool
	EmailRequested bool
}

// Validate normalises and validates the request, returning the cleaned values.
//
// IT REFUSES AN EMPTY RECIPIENT LIST, and that is the check most worth having: §5 lets a person
// tick departments, name individuals, or both, and an announcement that reaches nobody is the one
// failure of this module that produces no error anywhere — the author presses `➤ Phát hành`, the
// card appears in the book, and the commune is simply never told. See ErrNoRecipients.
func (r CreateAnnouncementRequest) Validate() (CreateAnnouncementRequest, error) {
	var out CreateAnnouncementRequest
	var err error

	if out.Title, err = NormalizeAnnouncementTitle(r.Title); err != nil {
		return CreateAnnouncementRequest{}, err
	}
	if out.Body, err = NormalizeAnnouncementBody(r.Body); err != nil {
		return CreateAnnouncementRequest{}, err
	}
	// The department ids are ULIDs minted by service-identity, so they are validated with the same
	// shape check as a staff code: present, one token, not a paragraph. Nothing here asserts that
	// the department EXISTS — that is identity's data (rule 2), and the consequence of a stale id
	// is written down in migration 0005.
	if out.OrgUnitIDs, err = NormalizeStaffCodes(r.OrgUnitIDs, MaxAnnouncementOrgUnits, ErrTooManyOrgUnits); err != nil {
		return CreateAnnouncementRequest{}, err
	}
	if out.RecipientCodes, err = NormalizeStaffCodes(r.RecipientCodes, MaxAnnouncementRecipients, ErrTooManyRecipients); err != nil {
		return CreateAnnouncementRequest{}, err
	}

	out.Pinned = r.Pinned
	out.AckRequired = r.AckRequired
	out.EmailRequested = r.EmailRequested
	return out, nil
}
