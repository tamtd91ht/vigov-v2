package domain

// Commune sentences — "Xã tự thêm" on "Lời hệ thống" (ADR 0079 Q2, Q5a, Q5b; migration 0033 §B).
//
// STORED AND MANAGED ONLY. ADR 0079 Q5b ("Chỉ lưu và quản lý, chưa hiện ra đâu"): nothing in this
// repository resolves, sends or shows one of these sentences — not a refusal branch, not ZNS, not the
// Mini App. They are listed, added, edited, switched and soft deleted on the configuration screen and
// nowhere else. The first caller that DOES show one (a later card) decides where, and if a citizen
// reads it, that is a rule 4 question at that time — not something to wire in passing.
//
// A different entity from MessageOverride on purpose (0033 §B): an override's key is closed by a CHECK
// that agrees with the code; a commune key is open by definition.

import (
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// CustomMessage is one sentence a commune added, as stored. Live rows only reach callers — a
// soft-deleted row is history (rule 7, invariant 2).
type CustomMessage struct {
	ID          string
	Group       string
	Key         string
	Text        string
	Description string // "" = none (NULL)
	Active      bool
	CreatedAt   time.Time
	CreatedBy   string // staff business code (rule 6, invariant 8)
	UpdatedAt   time.Time
	UpdatedBy   string
}

// customGroups is the closed set this service stores commune sentences under, mirrored by the CHECK
// `custom_system_message_group_known` in migration 0033. "Giải ngân" is service-finance's; "Báo cáo
// điều hành" takes no commune sentences at all (ADR 0079 Q5a).
var customGroups = []string{GroupFeedback, GroupShared}

// CustomMessageKeyMax mirrors the CHECK `custom_system_message_key_shape` (0033).
const CustomMessageKeyMax = 100

// CustomMessageDeleteReasonMax mirrors the CHECK `custom_system_message_delete_complete` (0033).
const CustomMessageDeleteReasonMax = 200

// customKeyShape is the CHECK's regular expression, verbatim: lowercase slug segments joined by `.`
// or `-`.
var customKeyShape = regexp.MustCompile(`^[a-z0-9]+([.-][a-z0-9]+)*$`)

var (
	ErrCustomGroupUnknown = errors.New("system_message: nhóm câu không nhận câu xã tự thêm — chọn Phản ánh hoặc Dùng chung")
	ErrCustomKeyShape     = errors.New("system_message: mã câu chỉ gồm chữ thường không dấu, số, dấu chấm hoặc gạch nối, tối đa 100 ký tự")
	ErrCustomKeyPrefix    = errors.New("system_message: mã câu phải bắt đầu bằng mã nhóm và dấu chấm, ví dụ chung.loi-chao")

	// ErrMessageCodeTaken — the key is a shipped key, or a commune sentence of this commune already
	// holds it, SOFT-DELETED ROWS INCLUDED: a deleted key is never reused (rule 7, invariant 3).
	ErrMessageCodeTaken = errors.New("system_message: mã câu đã được dùng")

	// ErrCustomMessageNotFound — no live commune sentence with that key in this commune. Same answer
	// as an unknown key; it does not say whether another commune has one.
	ErrCustomMessageNotFound = errors.New("system_message: không có câu xã tự thêm mang mã này")

	// ErrShippedMessageNotDeletable — DELETE on a shipped key. It can be reworded and restored, never
	// removed: removed, a refusal would have nothing to say.
	ErrShippedMessageNotDeletable = errors.New("system_message: câu đi kèm phần mềm không xoá được")

	// ErrShippedMessageNotCustom — PATCH of a commune sentence's fields aimed at a shipped key.
	ErrShippedMessageNotCustom = errors.New("system_message: câu đi kèm phần mềm sửa lời bằng tuyến lời của xã")

	// ErrCustomCatalogueFull — the commune is at TranCustomMessages live sentences.
	ErrCustomCatalogueFull = errors.New("system_message: số câu xã tự thêm đã đạt tối đa")

	ErrCustomDeleteReasonEmpty   = errors.New("system_message: phải ghi lý do xoá")
	ErrCustomDeleteReasonTooLong = errors.New("system_message: lý do xoá quá dài (tối đa 200 ký tự)")
	ErrCustomDeleteReasonControl = errors.New("system_message: lý do xoá chứa ký tự điều khiển hoặc ký tự vô hình")
	ErrDescriptionInvalid        = errors.New("system_message: mô tả chứa ký tự điều khiển, ký tự vô hình, dấu < hoặc >, hoặc quá 1000 ký tự")
)

// TranCustomMessages bounds one commune's live commune sentences in THIS service. The list route
// returns the whole catalogue (shipped + commune) unpaginated, so it needs a ceiling the data is
// measured against; 200 is far past a screen of sentences and short of a loop that inserted rows.
// An assumption of this card, stated: neither the ADR nor the prototype names a number.
const TranCustomMessages = 200

// NormalizeCustomKey validates a commune key against its group. The group is checked first so the
// prefix error can name a group that exists.
func NormalizeCustomKey(group, key string) (string, string, error) {
	group = strings.TrimSpace(group)
	known := false
	for _, g := range customGroups {
		if g == group {
			known = true
			break
		}
	}
	if !known {
		return "", "", ErrCustomGroupUnknown
	}
	key = strings.TrimSpace(key)
	if len(key) > CustomMessageKeyMax || !customKeyShape.MatchString(key) {
		return "", "", ErrCustomKeyShape
	}
	if !strings.HasPrefix(key, group+".") {
		return "", "", ErrCustomKeyPrefix
	}
	// A shipped key cannot pass the prefix rule today (`feedback.` is no group), but the readable
	// refusal must not depend on that staying true.
	if _, shipped := LookupShippedMessage(key); shipped {
		return "", "", ErrMessageCodeTaken
	}
	return group, key, nil
}

// NormalizeDescription trims an optional description. "" means none (NULL); otherwise the same floor
// as a wording — no control or format character, no `<` / `>`, at most MessageTextMax runes.
func NormalizeDescription(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	if len([]rune(s)) > MessageTextMax {
		return "", ErrDescriptionInvalid
	}
	for _, r := range s {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == '<' || r == '>' {
			return "", ErrDescriptionInvalid
		}
	}
	return s, nil
}

// NormalizeCustomDeleteReason trims and bounds the reason rule 7, invariant 1 requires.
func NormalizeCustomDeleteReason(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ErrCustomDeleteReasonEmpty
	}
	if len([]rune(s)) > CustomMessageDeleteReasonMax {
		return "", ErrCustomDeleteReasonTooLong
	}
	for _, r := range s {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return "", ErrCustomDeleteReasonControl
		}
	}
	return s, nil
}

// CustomToMessage is how a commune sentence is listed beside the shipped ones.
func CustomToMessage(c CustomMessage) SystemMessage {
	at := c.UpdatedAt
	return SystemMessage{
		Key: c.Key, Group: c.Group, Origin: OriginCommune, Description: c.Description,
		CurrentText: c.Text, Active: c.Active, UpdatedAt: &at, UpdatedBy: c.UpdatedBy,
	}
}

// IsCustomMessageInputError reports a refusal of what the client sent for a commune sentence (400).
func IsCustomMessageInputError(err error) bool {
	for _, e := range []error{ErrCustomGroupUnknown, ErrCustomKeyShape, ErrCustomKeyPrefix,
		ErrCustomDeleteReasonEmpty, ErrCustomDeleteReasonTooLong, ErrCustomDeleteReasonControl, ErrDescriptionInvalid} {
		if errors.Is(err, e) {
			return true
		}
	}
	return IsMessageInputError(err)
}
