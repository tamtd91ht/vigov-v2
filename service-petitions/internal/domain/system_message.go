package domain

// System messages — the sentences this service says, which a commune may reword
// (14-cau-hinh §7 "Lời hệ thống", ADR 0024 §`loi_he_thong`).
//
// A COPY OF service-finance/internal/domain/system_message.go, NOT AN IMPORT (rule 2, forbidden #1).
// Only the catalogue differs: these are the six `feedback.*` keys, the ones ADR 0024 assigns to this
// service because a refusal branch HERE raises each of them.
//
// THE SHIPPED DEFAULT LIVES HERE, IN CODE, AND NOWHERE ELSE. The table `system_message_override`
// holds only a commune's own wording, and only once it has reworded a key (migration 0020). So a
// commune that never touched the screen reads exactly the sentence below, and a default improved in
// a release reaches every commune that has not reworded it.

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
)

// The six keys. Their spelling is the requirement's (../vigov-require/apps/api/app/modules/admin/
// messages.py) and ADR 0024's, and it is a contract with the configuration screen and the database
// CHECK — never renamed.
const (
	KeyFeedbackAfterPhotoRequired = "feedback.after_photo_required"
	KeyFeedbackReasonRequired     = "feedback.reason_required"
	KeyFeedbackInvalidTransition  = "feedback.invalid_transition"
	KeyFeedbackNeverPublic        = "feedback.never_public"
	KeyFeedbackUnknownField       = "feedback.unknown_field"
	KeyFeedbackAssignmentRequired = "feedback.assignment_required"
)

// ShippedMessage is one sentence the software ships. Key and DefaultText are the contract; the
// description is what the configuration card prints under the key.
type ShippedMessage struct {
	Key         string
	Description string
	DefaultText string
}

// shippedMessages is the closed set. It MUST agree with the CHECK `system_message_override_key_known`
// in migration 0020, in the same order (TestCatalogueAgreesWithMigrationCheck). Order is the order the
// configuration screen lists them in — the requirement's.
//
// THE DEFAULTS ARE THE REQUIREMENT'S SENTENCES WITH v2's TERMS: "Không tiếp nhận" where the
// requirement said "Từ chối" (the v2 status is `khong-tiep-nhan`), and "thái độ, tác phong" for the
// staff-conduct field.
var shippedMessages = []ShippedMessage{
	{
		Key:         KeyFeedbackAfterPhotoRequired,
		Description: "Hiện khi cán bộ đóng phiếu mà chưa đính kèm ảnh sau xử lý — chỉ ở xã bật bắt buộc ảnh nghiệm thu",
		DefaultText: "Chưa thể đóng phiếu: phải có ít nhất một ảnh sau xử lý để người dân đối chiếu với ảnh trước khi xử lý.",
	},
	{
		Key:         KeyFeedbackReasonRequired,
		Description: "Hiện khi cán bộ không tiếp nhận hoặc chuyển phiếu lên cấp trên mà bỏ trống lý do",
		DefaultText: "Không tiếp nhận hoặc chuyển phiếu lên cấp trên thì phải ghi rõ lý do để trả lời người dân.",
	},
	{
		Key:         KeyFeedbackInvalidTransition,
		Description: "Hiện khi thao tác chuyển bước không hợp lệ với trạng thái hiện tại của phiếu",
		DefaultText: "Không thể chuyển phiếu sang trạng thái này từ trạng thái hiện tại.",
	},
	{
		Key:         KeyFeedbackNeverPublic,
		Description: "Hiện khi cán bộ cho hiện công khai một phiếu thuộc lĩnh vực thái độ, tác phong cán bộ",
		DefaultText: "Phản ánh về thái độ, tác phong cán bộ không được hiển thị công khai.",
	},
	{
		Key:         KeyFeedbackUnknownField,
		Description: "Hiện khi phiếu gửi lên hoặc cán bộ nhập hộ với lĩnh vực không còn trong danh mục của xã",
		DefaultText: "Lĩnh vực đã chọn không còn sử dụng. Vui lòng chọn lại lĩnh vực phản ánh.",
	},
	{
		Key:         KeyFeedbackAssignmentRequired,
		Description: "Hiện khi cán bộ chuyển xử lý mà chưa chọn bộ phận nhận việc",
		DefaultText: "Phiếu phải được giao cho một bộ phận xử lý; cán bộ phụ trách chọn thêm nếu cần.",
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

	// Inactive is "Tắt" (migration 0033 §A, ADR 0079 Q2): the wording is kept and NOT used — the
	// shipped sentence is in force until "Bật lại". Named for the off state so the zero value is the
	// column's default (`is_active` true): an override built without thinking about the switch is in
	// force, exactly as every override was before 0033.
	Inactive bool
}

// The group a sentence is listed under on "Lời hệ thống" — the web's group codes (ADR 0011 values).
// Every shipped key of this service is a petitions refusal, so it is listed under "Phản ánh"; a
// commune may add sentences to "Phản ánh" and to "Dùng chung" (ADR 0079 Q5a: "Dùng chung" lives here).
const (
	GroupFeedback = "phan-anh"
	GroupShared   = "chung"
)

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
//
// A commune sentence (Origin commune) has no DefaultText; CurrentText is its own text and Active its
// switch — see CustomMessage.
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
// override for another key is a programming error, not a wording. The configuration list and every
// refusal branch that emits one of these sentences go through here, so "what does this commune see"
// has exactly one answer.
//
// A SWITCHED-OFF OVERRIDE RESOLVES TO THE DEFAULT (migration 0033 §A, question 4). This is the one place
// that decides it, so the list, Text and therefore every refusal branch agree; a reader that took
// o.Text without asking here would keep sending a wording the commune switched off.
func ResolveMessage(m ShippedMessage, o *MessageOverride) SystemMessage {
	out := SystemMessage{
		Key:         m.Key,
		Group:       GroupFeedback,
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
	out.OverrideText = o.Text
	out.Overridden = true
	out.Active = !o.Inactive
	if out.Active {
		out.CurrentText = o.Text
	}
	out.UpdatedAt = &at
	out.UpdatedBy = o.UpdatedBy
	return out
}

// MessageTextMax is the bound in characters (runes), mirrored by the CHECK in migration 0020.
const MessageTextMax = 1000

var (
	ErrUnknownMessageKey = errors.New("system_message: khoá câu không thuộc phân hệ này")

	// ErrMessageTextEmpty — an empty wording is refused rather than stored: "no sentence" is not a
	// state this screen offers, and an empty refusal is a refusal the officer cannot act on. Going back
	// to the software's sentence is the revert route.
	ErrMessageTextEmpty    = errors.New("system_message: nội dung câu trống — muốn dùng lại câu mặc định thì bấm Khôi phục câu mặc định")
	ErrMessageTextTooLong  = errors.New("system_message: nội dung câu quá dài")
	ErrMessageTextControl  = errors.New("system_message: nội dung câu chứa ký tự điều khiển hoặc ký tự vô hình")
	ErrMessageTextMarkup   = errors.New("system_message: nội dung câu không được chứa dấu < hoặc >")
	ErrMessageActorMissing = errors.New("system_message: thiếu mã cán bộ thực hiện")
)

// NormalizeMessageText trims and validates one wording.
//
// CONTROL (Cc) AND FORMAT (Cf) CHARACTERS ARE BOTH REFUSED. Cc covers line breaks and escape
// sequences; Cf covers zero-width and bidirectional-override characters, which make a sentence read
// differently from what it contains — on a refusal a citizen or an officer reads as the authority's.
//
// `<` AND `>` ARE REFUSED so the stored sentence is text by construction; no renderer has to be the
// only defence (rule 13, invariant 3).
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

// IsMessageInputError reports whether err is a refusal of what the client sent (400), as opposed to
// a failure. Listed explicitly: a default of "unknown means client error" turns an outage into a 400
// that a client retries with different input forever.
func IsMessageInputError(err error) bool {
	for _, e := range []error{ErrMessageTextEmpty, ErrMessageTextTooLong, ErrMessageTextControl, ErrMessageTextMarkup} {
		if errors.Is(err, e) {
			return true
		}
	}
	return false
}
