package domain

// System messages — the sentences this service says, which a commune may reword
// (14-cau-hinh §7 "Lời hệ thống", ADR 0024 §`loi_he_thong`).
//
// THE SHIPPED DEFAULT LIVES HERE, IN CODE, AND NOWHERE ELSE. The table
// `system_message_override` holds only a commune's own wording, and only once it has reworded a
// key (migration 0010). So a commune that never touched the screen reads exactly the sentence
// below, and a default improved in a release reaches every commune that has not reworded it.
//
// WHICH KEYS BELONG HERE: the ones THIS service raises. ADR 0024's test is "delete the branch of
// code and the sentence becomes garbage" — `budget.scope_notice` travels with the disbursement
// figures this service computes. The six `feedback.*` keys are service-petitions'; the 32
// `report.*` keys have no owner yet and are NOT built anywhere.

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
)

// KeyBudgetScopeNotice is the product-boundary sentence shown to staff at the top of the budget
// screen and meant to accompany every disbursement figure (docs/ui-ux/README.md:65, 14-cau-hinh §7).
const KeyBudgetScopeNotice = "budget.scope_notice"

// ShippedMessage is one sentence the software ships. Key and DefaultText are the contract; the
// description is what the configuration card prints under the key.
type ShippedMessage struct {
	Key         string
	Description string
	DefaultText string
}

// shippedMessages is the closed set. It MUST agree with the CHECK `system_message_override_key_known`
// in migration 0010: a key here and not there is a key the database refuses to store; a key there and
// not here is a row nothing reads. Order is the order the configuration screen lists them in.
var shippedMessages = []ShippedMessage{
	{
		Key:         KeyBudgetScopeNotice,
		Description: "Dòng ghi rõ ranh giới sản phẩm, hiện ở đầu màn hình theo dõi giải ngân và đi kèm mọi số liệu API trả về",
		DefaultText: "Hệ thống là công cụ theo dõi và điều hành, không phải phần mềm kế toán. Số liệu phục vụ chỉ đạo, không thay thế sổ sách kế toán và không đối chiếu với Kho bạc.",
	},
}

// ShippedMessages returns a copy of the catalogue, so no caller can edit the defaults in place.
func ShippedMessages() []ShippedMessage {
	out := make([]ShippedMessage, len(shippedMessages))
	copy(out, shippedMessages)
	return out
}

// LookupShippedMessage finds one key. A key outside the catalogue answers false — the caller turns
// that into 404, never into a new row.
func LookupShippedMessage(key string) (ShippedMessage, bool) {
	for _, m := range shippedMessages {
		if m.Key == key {
			return m, true
		}
	}
	return ShippedMessage{}, false
}

// MessageOverride is a commune's live wording of one key, as stored.
type MessageOverride struct {
	ID        string
	Key       string
	Text      string
	UpdatedAt time.Time
	UpdatedBy string // staff business code, never the internal id (rule 6, invariant 8)

	// Inactive is "Tắt" (migration 0017 §A, ADR 0079 Q2): the wording is kept and NOT used — the shipped
	// sentence is in force until "Bật lại". Named for the off state so the zero value is the column's
	// default (`is_active` true): an override built without thinking about the switch is in force,
	// exactly as every override was before that migration.
	//
	// Text "" with Inactive true is a sentence switched off WITHOUT a commune wording (migration 0019:
	// `message_text` NULL). The database refuses "" as a wording, so "" never means anything else.
	Inactive bool
}

// GroupDisbursement is the group every shipped key of this service is listed under on "Lời hệ thống" — the
// web's group code (ADR 0011 value).
const GroupDisbursement = "giai-ngan"

// Origin says who wrote a sentence: the software ("shipped") or the commune ("commune"). Technical
// values the web branches on — which buttons a card has — not business vocabulary.
const (
	OriginShipped = "shipped"
	OriginCommune = "commune"
)

// SystemMessage is what a reader gets: the default, the text in force, and whether the commune
// has reworded it.
//
// THREE STATES OF A SHIPPED KEY, and how they read:
//
//	no override          CurrentText = DefaultText, Overridden false, Active true
//	override, on         CurrentText = OverrideText, Overridden true,  Active true
//	override, "Tắt"      CurrentText = DefaultText, Overridden true,  Active false — OverrideText
//	                     still carries the commune's words, so "Bật lại" shows what comes back
//	"Tắt", no wording    CurrentText = DefaultText, Overridden false, Active false — a sentence the
//	                     commune switched off without rewording it (user decision 09/10/2026,
//	                     migration 0019)
//
// WHAT A CONSUMER READS (user decision 09/10/2026): a switched-off sentence is HIDDEN where it is used;
// a consumer that must say something falls back to the software's sentence. So CurrentText is always
// the text a must-say consumer prints (the default while off), and Active is what a consumer that can
// stay silent checks first. Which kind each consumer is, is decided AT the consumer, never here.
type SystemMessage struct {
	Key          string
	Group        string
	Origin       string
	Description  string
	DefaultText  string
	CurrentText  string
	OverrideText string
	Overridden   bool
	Active       bool
	UpdatedAt    *time.Time
	UpdatedBy    string
}

// ResolveMessage is THE fallback rule, in one place: no live override means the default, and an
// override for another key is a programming error, not a wording. Every reader of these sentences
// — the configuration list today, any route that emits the sentence tomorrow — goes through here,
// so "what does this commune see" has exactly one answer.
//
// A SWITCHED-OFF OVERRIDE RESOLVES TO THE DEFAULT (migration 0017 §A). This is the one place that decides it,
// so every reader agrees; a reader that took o.Text without asking here would keep printing a wording
// the commune switched off.
func ResolveMessage(m ShippedMessage, o *MessageOverride) SystemMessage {
	out := SystemMessage{
		Key:         m.Key,
		Group:       GroupDisbursement,
		Origin:      OriginShipped,
		Description: m.Description,
		DefaultText: m.DefaultText,
		CurrentText: m.DefaultText,
		Active:      true,
	}
	if o == nil || o.Key != m.Key {
		return out
	}
	at := o.UpdatedAt
	out.Active = !o.Inactive
	out.UpdatedAt = &at
	out.UpdatedBy = o.UpdatedBy
	if o.Text == "" {
		// Switched off without ever being reworded: the default stays the fallback text, and nothing
		// is "overridden" — there is no commune wording for "Bật" to bring back.
		return out
	}
	out.OverrideText = o.Text
	out.Overridden = true
	if out.Active {
		out.CurrentText = o.Text
	}
	return out
}

// MessageTextMax is the bound in characters (runes), mirrored by the CHECK in migration 0010. A
// system sentence is one or two lines on a screen; 1000 is far past that and far short of a
// document somebody pasted by mistake.
const MessageTextMax = 1000

var (
	ErrUnknownMessageKey = errors.New("system_message: khoá câu không thuộc phân hệ này")

	// ErrMessageTextEmpty — an empty wording is refused rather than stored: "no sentence" is not a
	// state this screen offers. Going back to the software's sentence is the revert route.
	ErrMessageTextEmpty    = errors.New("system_message: nội dung câu trống — muốn dùng lại câu mặc định thì bấm Khôi phục câu mặc định")
	ErrMessageTextTooLong  = errors.New("system_message: nội dung câu quá dài")
	ErrMessageTextControl  = errors.New("system_message: nội dung câu chứa ký tự điều khiển hoặc ký tự vô hình")
	ErrMessageTextMarkup   = errors.New("system_message: nội dung câu không được chứa dấu < hoặc >")
	ErrMessageActorMissing = errors.New("system_message: thiếu mã cán bộ thực hiện")
)

// NormalizeMessageText trims and validates one wording.
//
// CONTROL (Cc) AND FORMAT (Cf) CHARACTERS ARE BOTH REFUSED. Cc covers line breaks and escape
// sequences; Cf covers zero-width and bidirectional-override characters, which make a sentence
// read differently from what it contains — on the one line a public authority prints above its
// own figures.
//
// `<` AND `>` ARE REFUSED so the stored sentence is text by construction; no renderer has to be
// the only defence (rule 13, invariant 3).
func NormalizeMessageText(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ErrMessageTextEmpty
	}
	if n := len([]rune(s)); n > MessageTextMax {
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrMessageTextTooLong, MessageTextMax)
	}
	for _, r := range s {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return "", ErrMessageTextControl
		}
		if r == '<' || r == '>' {
			return "", ErrMessageTextMarkup
		}
	}
	return s, nil
}

// IsMessageInputError reports whether err is a refusal of what the client sent (400), as opposed
// to a failure. Listed explicitly: a default of "unknown means client error" turns an outage into
// a 400 that a client retries with different input forever.
func IsMessageInputError(err error) bool {
	for _, e := range []error{ErrMessageTextEmpty, ErrMessageTextTooLong, ErrMessageTextControl, ErrMessageTextMarkup} {
		if errors.Is(err, e) {
			return true
		}
	}
	return false
}
