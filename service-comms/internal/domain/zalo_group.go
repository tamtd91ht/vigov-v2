package domain

// ZALO GROUP CHATS (ADR 0092; migrations/0029_zalo_group_link.sql; comms.proto StaffNotification fields 8–9):
// the group pairing code a member of staff issues in web-admin, the group link it produces, which notices a
// linked group receives, and the bot's replies about groups. Standard library only.
//
// A GROUP IS NOT A PERSON. Its scope is one org unit (`bo_phan`) or the whole commune, its kinds are its own
// subset of the commune's, and a notice reaches it by the RECORD's holding unit — never by a recipient's
// (comms.proto field 8). chat_id of a group is personal data like a private chat's (ADR 0092 §Hệ quả): no
// type here carries it.

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// ZaloGroupKinds are the kinds a group may receive — 0029's list, in ZaloReminderKinds' order. Every
// other kind is per-person, a digest, legacy, bell-only or not decided (0029 header), and NEVER reaches a
// group. The migration's three CHECKs name the same fourteen; migrations/zalo_group_link_test.go and
// zalo_group_test.go keep the two in step.
var ZaloGroupKinds = []string{
	ZaloKindTaskOverdue, ZaloKindTaskUnassigned, ZaloKindTaskEscalation,
	ZaloKindDocumentOverdue, ZaloKindDocumentUnassigned, ZaloKindDocumentEscalation,
	ZaloKindPetitionOverdue, ZaloKindPetitionUnassigned, ZaloKindPetitionEscalation,
	ZaloKindTaskAssigned, ZaloKindTaskApprovalRequested, ZaloKindDocumentAssigned,
	ZaloKindPetitionAssigned, ZaloKindPetitionReopened,
}

// IsZaloGroupKind reports whether a notice of kind may ever be posted to a group.
func IsZaloGroupKind(kind string) bool { return slices.Contains(ZaloGroupKinds, kind) }

// GroupPosting is comms.proto GroupPosting — whether the PRODUCER allows a notice into a group. The zero
// value is Unspecified, which is NOT allowed (fail closed: what every older producer sends).
type GroupPosting int

const (
	GroupPostingUnspecified GroupPosting = iota
	GroupPostingAllowed
	GroupPostingRestricted
)

// ValidOrgUnitID is 0029's org_unit_shape CHECK: 26 characters of upper-case Crockford base32 (a ULID).
func ValidOrgUnitID(s string) bool {
	if len(s) != 26 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
		case c >= 'A' && c <= 'Z' && c != 'I' && c != 'L' && c != 'O' && c != 'U':
		default:
			return false
		}
	}
	return true
}

// NormalizeOrgUnitID reads an org-unit id from a client or a producer: upper-cased (Crockford is
// case-insensitive, the column is upper-case), then 0029's shape. ok=false: not a ULID.
func NormalizeOrgUnitID(s string) (string, bool) {
	s = strings.ToUpper(s)
	return s, ValidOrgUnitID(s)
}

// ZaloGroupTarget is one live group as the enqueue reads it: which group, its scope, its kinds.
type ZaloGroupTarget struct {
	ID string
	// OrgUnitID "" = the whole commune.
	OrgUnitID string
	Kinds     []string
}

// GroupNotice is one notice as the group enqueue sees it.
type GroupNotice struct {
	IdempotencyKey   string
	Kind             string
	HoldingOrgUnitID string // "" = unknown / none: commune-wide groups only
	Posting          GroupPosting
	Title, Body      string
	Link             string
}

// ZaloGroupMatches returns the ids of the groups a notice is posted to (comms.proto fields 8–9, (a)–(c)):
// group posting ALLOWED, a group kind the group selected, and the group commune-wide or the holding unit
// non-empty and equal to the group's. Empty is never "every unit".
func ZaloGroupMatches(n GroupNotice, groups []ZaloGroupTarget) []string {
	if n.Posting != GroupPostingAllowed || !IsZaloGroupKind(n.Kind) {
		return nil
	}
	var out []string
	for _, g := range groups {
		if !slices.Contains(g.Kinds, n.Kind) {
			continue
		}
		if g.OrgUnitID == "" || (n.HoldingOrgUnitID != "" && g.OrgUnitID == n.HoldingOrgUnitID) {
			out = append(out, g.ID)
		}
	}
	return out
}

// ZaloGroupLink is one `zalo_group_link` row as the staff side reads it. NO chat_id (ADR 0092 §Hệ quả).
type ZaloGroupLink struct {
	ID        string
	BotRef    string
	OrgUnitID string // "" = the whole commune
	Kinds     []string
	LinkedAt  time.Time
	LinkedBy  string // the code ISSUER's business code (rule 6 inv 8)
}

// ---- validation ------------------------------------------------------------------------------------

// ZaloGroupError is a refusal of a group code or a group edit: a machine code and ONE Vietnamese sentence the
// screen shows as is (the ZaloSettingError shape).
type ZaloGroupError struct {
	Code    string
	Message string
}

func (e *ZaloGroupError) Error() string { return "zalo_group: " + e.Code }

// ErrZaloGroupInvalid is matched by errors.Is on every *ZaloGroupError.
var ErrZaloGroupInvalid = errors.New("zalo_group: invalid")

func (e *ZaloGroupError) Is(target error) bool { return target == ErrZaloGroupInvalid }

func groupErr(code, msg string) error { return &ZaloGroupError{Code: code, Message: msg} }

// The refusals of the group routes.
var (
	ErrZaloGroupOrgUnitShape = groupErr("org_unit_invalid", "Mã bộ phận không hợp lệ.")
	// ErrZaloGroupOrgUnitNotLive — unknown, removed and another commune's unit are ONE answer
	// (identityclient.LiveOrgUnits).
	ErrZaloGroupOrgUnitNotLive = groupErr("org_unit_not_live",
		"Bộ phận này không còn trong sơ đồ tổ chức của xã. Hãy chọn bộ phận khác hoặc để trống (cả xã).")
	ErrZaloGroupKindsRequired = groupErr("kinds_required", "Cần chọn ít nhất một loại tin cho nhóm.")
	ErrZaloGroupKindUnknown   = groupErr("unknown_kind",
		"Loại tin này không gửi vào nhóm được. Chỉ chọn trong các loại tin dành cho nhóm.")
	ErrZaloGroupKindNotEnabled = groupErr("kind_not_enabled",
		"Loại tin này xã chưa bật ở Kênh Zalo. Hãy bật loại tin ấy trong cấu hình kênh trước.")
	ErrZaloGroupUnlinkReason = groupErr("reason_required",
		fmt.Sprintf("Cần nhập lý do gỡ nhóm, tối đa %d ký tự.", maxZaloGroupUnlinkReasonRunes))
	ErrZaloGroupNothingToChange = groupErr("nothing_to_change",
		"Cần gửi ít nhất một trong hai trường: bộ phận hoặc loại tin.")
)

// maxZaloGroupUnlinkReasonRunes is 0029's zalo_group_link_unlink_reason_length.
const maxZaloGroupUnlinkReasonRunes = 200

// NormalizeZaloGroupKinds validates a group's kinds against the commune's settings NOW: non-empty, each a
// group kind, each switched on for the commune (ADR 0092 #2 — a subset of the commune's). Duplicates
// collapse, the order is ZaloGroupKinds'. A commune whose channel is off has no kind switched on.
func NormalizeZaloGroupKinds(kinds []string, s ZaloChannelSetting) ([]string, error) {
	if len(kinds) == 0 {
		return nil, ErrZaloGroupKindsRequired
	}
	want := map[string]bool{}
	for _, k := range kinds {
		if !IsZaloGroupKind(k) {
			return nil, ErrZaloGroupKindUnknown
		}
		if !s.Saved || !s.IsEnabled || !s.KindEnabled(k) {
			return nil, ErrZaloGroupKindNotEnabled
		}
		want[k] = true
	}
	out := make([]string, 0, len(want))
	for _, k := range ZaloGroupKinds {
		if want[k] {
			out = append(out, k)
		}
	}
	return out, nil
}

// NormalizeZaloGroupUnlinkReason — required, trimmed, ≤ 200 characters, no control characters (rule 7).
func NormalizeZaloGroupUnlinkReason(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" || !utf8.ValidString(s) || utf8.RuneCountInString(s) > maxZaloGroupUnlinkReasonRunes {
		return "", ErrZaloGroupUnlinkReason
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return "", ErrZaloGroupUnlinkReason
		}
	}
	return s, nil
}

// ---- trail and fixed sentences ------------------------------------------------------------------------

// The commune audit_log verbs of group chats (values, ADR 0011).
const (
	ActionIssueZaloGroupPairingCode = "tao_ma_gan_nhom_zalo"
	ActionLinkZaloGroup             = "gan_nhom_zalo"
	ActionEditZaloGroup             = "sua_nhom_zalo"
	ActionEndZaloGroupLink          = "ket_thuc_nhom_zalo"
)

// ZaloGroupSubject is the audit subject of a group link — it has no business code, so the locator is its id.
func ZaloGroupSubject(linkID string) string { return "nhom-zalo/" + linkID }

// ZaloGroupEndedByBotChange is zalo_group_link.unlink_reason when a switch of bot ended the link (ADR 0079
// Q1 #4; 0018 account change) — a fixed sentence, ≤ 200 characters.
const ZaloGroupEndedByBotChange = "Đổi bot Zalo — nhóm không còn nhận tin qua bot cũ, cần gắn lại bằng mã mới"

// The bot's replies about groups (ADR 0092; fixed in code like ADR 0074 #8). None names a person, a code
// or another commune.
const (
	// ZaloReplyGroupCodeRefused is ONE sentence for every refusal of a code-shaped text in a group that is not
	// an open group code of a single commune — wrong, expired, used, ambiguous, through the wrong bot — AND
	// for a personal code typed there: the personal table is never read from a group (ADR 0092 #4), so the
	// two cannot be told apart, and the sentence carries both instructions.
	ZaloReplyGroupCodeRefused = "Mã gắn nhóm không đúng hoặc đã hết hạn. Nếu đây là mã ghép nối cá nhân, " +
		"hãy nhắn riêng cho bot."
	// ZaloReplyGroupAlreadyLinked — the chat already has a live group link, in this commune or any other
	// (ADR 0092 #9; re-linking in the same commune is an edit on ViGov). The code is NOT consumed.
	ZaloReplyGroupAlreadyLinked = "Nhóm này đang được gắn với ViGov. Muốn gắn lại, cần gỡ nhóm trên ViGov trước " +
		"(Cấu hình → Kênh Zalo)."
	// ZaloReplyGroupPairedPersonally — the group chat is paired to one person (a pairing made before the
	// chat type was read, 0029 zalo_chat_one_link_kind). That person unlinks on ViGov first.
	ZaloReplyGroupPairedPersonally = "Cuộc trò chuyện này đang ghép nối với tài khoản cá nhân của một cán bộ nên " +
		"chưa gắn nhóm được. Cán bộ đó cần gỡ ghép nối Zalo trên ViGov (trang Cá nhân) trước."
	// ZaloReplyPrivateGroupCode — a GROUP code typed in a private chat: guidance, nothing consumed.
	ZaloReplyPrivateGroupCode = "Đây là mã gắn nhóm. Hãy thêm bot vào nhóm Zalo cần gắn rồi gửi mã này trong nhóm " +
		"(có thể kèm @tên bot)."
	// zaloReplyGroupLinkedFormat names the scope: "bộ phận <tên>" or "toàn xã".
	zaloReplyGroupLinkedFormat = "Đã gắn nhóm với ViGov — nhận nhắc việc của %s."
	// ZaloGroupScopeCommune / ZaloGroupScopeUnknownUnit are the scope words of the confirmation.
	ZaloGroupScopeCommune     = "toàn xã"
	ZaloGroupScopeUnknownUnit = "bộ phận đã chọn"
)

// ZaloReplyGroupLinked is the confirmation posted in the group: unitName "" = the whole commune; a unit
// whose name could not be read is "bộ phận đã chọn" — never a guessed name.
func ZaloReplyGroupLinked(commune bool, unitName string) string {
	switch {
	case commune:
		return fmt.Sprintf(zaloReplyGroupLinkedFormat, ZaloGroupScopeCommune)
	case strings.TrimSpace(unitName) == "":
		return fmt.Sprintf(zaloReplyGroupLinkedFormat, ZaloGroupScopeUnknownUnit)
	}
	return fmt.Sprintf(zaloReplyGroupLinkedFormat, "bộ phận "+strings.TrimSpace(unitName))
}
