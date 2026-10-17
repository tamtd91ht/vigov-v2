package domain

// A COMMUNE'S ZALO OA, AUTHORIZED TO THE PLATFORM'S SHARED ZALO APP (migration 0031; ADR 0098, block A).
// The commune's OA is the sender a citizen sees (ADR 0006, 0018): its templates, its quota, its name.
// Standard library only.
//
// THE TOKENS AND THE CODE VERIFIER ARE NOT FIELDS HERE. They are sealed under the commune's data key and
// travel only as sealed bytes (store) or as a secret.Secret on their way to Zalo (app) — never in a value
// that could be returned, logged or compared (ADR 0098 #5, #13).

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// The lifetimes Zalo documents (ADR 0098 §Kết quả tra tài liệu, 16/10/2026, evidence HIGH) and the one
// lead ADR 0098 #6 fixes. Platform-wide facts about ONE third party — not per-commune configuration.
const (
	// AccessTokenTTL — "Access token 25 giờ". Used only when Zalo's answer carries no `expires_in`.
	AccessTokenTTL = 25 * time.Hour
	// RefreshTokenTTL — "Refresh token 3 tháng, dùng một lần". Counted from refresh_issued_at; a token
	// older than this is not even presented to Zalo (ADR 0098 #6 → `refresh-token-het-han`).
	RefreshTokenTTL = 90 * 24 * time.Hour
	// AccessRefreshLead — the refresh job acts while LESS than this remains (ADR 0098 #6, "< 2 giờ").
	AccessRefreshLead = 2 * time.Hour
	// AuthorizationTTL — an authorization link lives as long as Zalo's authorization code: 10 minutes.
	AuthorizationTTL = 10 * time.Minute
)

// CommuneZaloOA is a zalo_commune_oa row as metadata — one of the commune's live OAs (several at once,
// ADR 0099 #1), or a retired one as history. Zero times mean NULL.
type CommuneZaloOA struct {
	ID     string // ULID, internal
	OAID   string // Zalo's oa_id — from the callback and from getoa, never typed by a person
	OAName string
	// AccessExpiresAt: when the access token stops working. RefreshIssuedAt: when the CURRENT refresh
	// token was issued. TokenRefreshedAt: when the sealed columns were last written (0031's guard demands
	// it moves on every reseal — ADR 0098 §"Hai cột thời điểm").
	AccessExpiresAt  time.Time
	RefreshIssuedAt  time.Time
	TokenRefreshedAt time.Time
	// AuthorizedAt / AuthorizedBy: who connected this OA (staff CODE, rule 6 invariant 8), immutable.
	AuthorizedAt time.Time
	AuthorizedBy string
	// LastCheckAt / LastCheckResult: both zero, or both set; the result is a ZaloCall* value.
	LastCheckAt     time.Time
	LastCheckResult string
	// NeedsReauthorizationSince / Reason: both zero, or both set (0031 reauthorization_pair). The row is
	// still live, but nothing may send through it (ADR 0098 #6).
	NeedsReauthorizationSince  time.Time
	NeedsReauthorizationReason string
	// RetiredAt / RetiredBy / RetireReason: 0031's soft-delete trio. Zero on the live row.
	RetiredAt    time.Time
	RetiredBy    string
	RetireReason string
}

// Subject is the audit subject of an act on this OA: Zalo's oa_id, the identity that outlives the row id
// (a re-authorization of the same OA is a new row with the same subject).
func (o CommuneZaloOA) Subject() string { return "zalo-oa/" + o.OAID }

// NeedsReauthorization — the refresh token was refused or aged out; the OA admin must authorize again.
func (o CommuneZaloOA) NeedsReauthorization() bool { return !o.NeedsReauthorizationSince.IsZero() }

// AccessExpired — nothing sends with this access token (0031 question 4: "refresh first, never send
// with it").
func (o CommuneZaloOA) AccessExpired(now time.Time) bool { return !now.Before(o.AccessExpiresAt) }

// AccessRefreshDue — less than AccessRefreshLead remains, or none (ADR 0098 #6).
func (o CommuneZaloOA) AccessRefreshDue(now time.Time) bool {
	return o.AccessExpiresAt.Sub(now) < AccessRefreshLead
}

// RefreshTokenExpired — the refresh token is older than Zalo's 3 months and is not presented to Zalo.
func (o CommuneZaloOA) RefreshTokenExpired(now time.Time) bool {
	return !now.Before(o.RefreshIssuedAt.Add(RefreshTokenTTL))
}

// ValidZaloOAID mirrors 0031's zalo_commune_oa_oa_id_shape (`^[0-9]{1,32}$`): digits only at Zalo.
// Checked on what the callback and getoa answered BEFORE anything is sealed or written.
func ValidZaloOAID(s string) bool {
	if len(s) == 0 || len(s) > 32 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// MaxZaloOANameRunes is 0031's zalo_commune_oa_oa_name_length. A longer name from getoa is clipped,
// never refused: the name is display only.
const MaxZaloOANameRunes = 255

// ClipZaloOAName fits a name Zalo answered into the column: trimmed, control characters removed,
// at most MaxZaloOANameRunes characters, invalid UTF-8 replaced.
func ClipZaloOAName(s string) string {
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, "�")
	}
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= MaxZaloOANameRunes {
		return s
	}
	n := 0
	for i := range s {
		if n == MaxZaloOANameRunes {
			return s[:i]
		}
		n++
	}
	return s
}

// The reasons a live OA needs re-authorization (0031 zalo_commune_oa_reauthorization_reason_known).
const (
	ReauthorizationRefreshTokenRejected = "refresh-token-bi-tu-choi"
	ReauthorizationRefreshTokenExpired  = "refresh-token-het-han"
)

// ---- the authorization request ---------------------------------------------------------------------

// ZaloOAAuthorization is a zalo_oa_authorization row as metadata: the state hash and the sealed verifier
// are not here. UsedAt zero = still open (whatever the outcome column later says, a used row is spent —
// 0031 question 4).
type ZaloOAAuthorization struct {
	ID          string
	RequestedAt time.Time
	RequestedBy string // staff CODE — the trail's "who" for the callback (ADR 0098 #12)
	ExpiresAt   time.Time
	UsedAt      time.Time
	Outcome     string
	OARowID     string
}

// Open — not yet used and not yet past its expiry.
func (a ZaloOAAuthorization) Open(now time.Time) bool {
	return a.UsedAt.IsZero() && now.Before(a.ExpiresAt)
}

// Subject is the audit subject of an act on this request: its own id is the only stable handle it has
// before an OA is known; prefixed so a reader tells it from an OA subject.
func (a ZaloOAAuthorization) Subject() string { return "zalo-oa-authorization/" + a.ID }

// The outcomes of an authorization request (0031 zalo_oa_authorization_outcome_known). OUR values,
// kebab-case Vietnamese (ADR 0011).
const (
	AuthorizationSucceeded         = "thanh-cong"
	AuthorizationZaloRefused       = "zalo-tu-choi"
	AuthorizationZaloUnavailable   = "khong-kha-dung"
	AuthorizationMalformedResponse = "phan-hoi-sai-dang"
	AuthorizationExpired           = "het-han"
	AuthorizationOALiveElsewhere   = "oa-dang-o-xa-khac"
	AuthorizationMalformedCallback = "sai-khuon"
)

// AuthorizationOutcomeOf maps a Zalo call class (ZaloCall*) onto the authorization outcome written when
// the token exchange did not succeed: a refusal is final, an outage is final too for THIS code (one
// authorization code, one try — 0031 question 4), and an unreadable answer is its own class.
func AuthorizationOutcomeOf(callClass string) string {
	switch callClass {
	case ZaloCallOK:
		return AuthorizationSucceeded
	case ZaloCallTokenRejected, ZaloCallRejected:
		return AuthorizationZaloRefused
	case ZaloCallRateLimited, ZaloCallUnavailable:
		return AuthorizationZaloUnavailable
	}
	return AuthorizationMalformedResponse
}

// ---- the ZBS templates ---------------------------------------------------------------------------------

// TemplateParam is one parameter Zalo declared on a template (`template/info`), as 0031 stores it in
// `params`: {name, required, type, max_length}. The NAMES are what ADR 0098 #9 compares.
type TemplateParam struct {
	Name      string
	Required  bool
	Type      string
	MaxLength int
}

// ZaloOATemplate is a zalo_oa_template row. Everything Zalo owns is overwritten by the next sync;
// Milestone* is the commune's and survives syncs.
type ZaloOATemplate struct {
	ID           string // ULID, internal
	OARowID      string
	TemplateID   string // Zalo's template id
	TemplateName string
	// ZaloStatus / ZaloQuality / ZaloTag hold ZALO'S OWN VALUES verbatim (0031: the one named
	// exception to ADR 0011).
	ZaloStatus  string
	ZaloQuality string
	ZaloTag     string
	Params      []TemplateParam
	// ParamsMatch is Go's verdict on Params (ADR 0098 #9), recomputed at every sync.
	ParamsMatch bool
	// Milestone: the petition milestone this template announces; "" = not attached. Trio all or none.
	Milestone      string
	MilestoneSetAt time.Time
	MilestoneSetBy string
	SyncedAt       time.Time
}

// Subject is the audit subject of an act on this template: Zalo's template id, under the OA.
func (t ZaloOATemplate) Subject() string { return "zalo-oa-template/" + t.TemplateID }

// Attached — a milestone is set.
func (t ZaloOATemplate) Attached() bool { return t.Milestone != "" }

// Attachable — may take a milestone (ADR 0098 #8, #9): ENABLE at Zalo and the three parameters present.
// Also what MauTinXa requires at read time (0031 question 4: "an attached template is not a sendable
// template").
func (t ZaloOATemplate) Attachable() bool {
	return t.ZaloStatus == ZaloTemplateStatusEnabled && t.ParamsMatch
}

// Zalo's template statuses (0031 zalo_oa_template_zalo_status_known) and qualities, VERBATIM — the named
// exception to ADR 0011: compared against Zalo's API on every sync.
const (
	ZaloTemplateStatusEnabled       = "ENABLE"
	ZaloTemplateStatusPendingReview = "PENDING_REVIEW"
	ZaloTemplateStatusRejected      = "REJECT"
	ZaloTemplateStatusDisabled      = "DISABLE"
	ZaloTemplateStatusDeleted       = "DELETE"

	ZaloTemplateQualityUndefined = "UNDEFINED"
)

// ValidZaloTemplateStatus mirrors 0031's CHECK. A value outside it is a template this sync cannot
// record — skipped and reported, never translated.
func ValidZaloTemplateStatus(s string) bool {
	switch s {
	case ZaloTemplateStatusEnabled, ZaloTemplateStatusPendingReview, ZaloTemplateStatusRejected,
		ZaloTemplateStatusDisabled, ZaloTemplateStatusDeleted:
		return true
	}
	return false
}

// ValidZaloTemplateQuality mirrors 0031's CHECK.
func ValidZaloTemplateQuality(s string) bool {
	switch s {
	case "HIGH", "MEDIUM", "LOW", ZaloTemplateQualityUndefined:
		return true
	}
	return false
}

// MaxZaloTemplateTagLen is 0031's zalo_oa_template_zalo_tag_length.
const MaxZaloTemplateTagLen = 32

// ValidZaloTemplateID mirrors 0031's zalo_oa_template_template_id_shape (`^[0-9A-Za-z_-]{1,64}$`).
func ValidZaloTemplateID(s string) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		b := s[i]
		switch {
		case b >= '0' && b <= '9', b >= 'A' && b <= 'Z', b >= 'a' && b <= 'z', b == '_', b == '-':
		default:
			return false
		}
	}
	return true
}

// The petition milestones a template may announce (0031 zalo_oa_template_milestone_known; ADR 0098 #8):
// ADR 0041's target statuses plus ADR 0087's merge / split. VALUES, Vietnamese without diacritics.
const (
	MilestoneReceived        = "da-tiep-nhan"
	MilestoneForwarded       = "da-chuyen-xu-ly"
	MilestoneAwaitingCitizen = "cho-dan-xac-nhan"
	MilestoneClosed          = "da-dong"
	MilestoneRefused         = "khong-tiep-nhan"
	MilestoneEscalated       = "chuyen-cap-tren"
	MilestonePetitionMerged  = "gop-phieu"
	MilestonePetitionSplit   = "tach-phieu"
)

// Milestones lists the eight, in 0031's order — for a screen's select and for tests.
func Milestones() []string {
	return []string{MilestoneReceived, MilestoneForwarded, MilestoneAwaitingCitizen, MilestoneClosed,
		MilestoneRefused, MilestoneEscalated, MilestonePetitionMerged, MilestonePetitionSplit}
}

// ValidMilestone reports whether s is one of the eight 0031 admits.
func ValidMilestone(s string) bool {
	for _, m := range Milestones() {
		if s == m {
			return true
		}
	}
	return false
}

// RequiredTemplateParams are the three parameter names a template MUST declare, BY NAME (ADR 0098 #9,
// owner 16/10/2026: "bắt buộc đúng tên, không có bảng ánh xạ"). They are the JSON keys of
// ThamSoThongBao — the only three things that ever leave this system inside a message (rule 3 inv 6).
var RequiredTemplateParams = []string{"ma_tra_cuu", "moc_nhan", "viec_tiep_theo"}

// MissingTemplateParams returns, in RequiredTemplateParams' order, the names params does not declare.
// Empty = the template matches. Case-sensitive and exact: `MaTraCuu` or `ma-tra-cuu` is missing.
func MissingTemplateParams(params []TemplateParam) []string {
	have := make(map[string]bool, len(params))
	for _, p := range params {
		have[p.Name] = true
	}
	var missing []string
	for _, name := range RequiredTemplateParams {
		if !have[name] {
			missing = append(missing, name)
		}
	}
	return missing
}

// ---- audit verbs, fixed sentences, errors ------------------------------------------------------------

// The commune audit_log verbs of the commune OA (values, ADR 0011; the commune bot's shape).
const (
	ActionCreateZaloOAAuthorizationLink = "tao_link_uy_quyen_oa_zalo"
	ActionConnectZaloOA                 = "noi_oa_zalo"
	ActionZaloOAAuthorizationFailed     = "uy_quyen_oa_zalo_that_bai"
	ActionZaloOAAuthorizationExpired    = "uy_quyen_oa_zalo_het_han"
	ActionCheckZaloOA                   = "kiem_tra_oa_zalo"
	ActionRefreshZaloOAToken            = "lam_moi_token_oa_zalo"
	ActionZaloOANeedsReauthorization    = "oa_zalo_can_uy_quyen_lai"
	ActionSyncZaloOATemplates           = "nap_mau_oa_zalo"
	ActionAttachZaloOATemplateMilestone = "gan_moc_mau_oa_zalo"
	ActionDetachZaloOATemplateMilestone = "go_moc_mau_oa_zalo"
	ActionDisconnectZaloOA              = "ngat_oa_zalo"
)

// CommuneZaloOAReauthorizedReason is retire_reason when the commune authorizes an OA that is ALREADY live
// in it: the old row is retired and a new one inserted (ADR 0098 #11, kept for the same OA by ADR 0099
// #1). Connecting a DIFFERENT OA retires nothing any more — a commune holds several (ADR 0099 #1). A fixed
// sentence (0031 CHECK ≤ 200). "Ngắt OA" carries the staff member's own reason.
const CommuneZaloOAReauthorizedReason = "Xã uỷ quyền lại OA này"

// CommuneZaloOARevokeNotice is said in the reply of every act that retires an OA row (0031 "OWED BY GO",
// ADR 0098 #11): retiring here revokes nothing at Zalo — the app keeps the OA's grant until the OA admin
// revokes it in OA Manager.
const CommuneZaloOARevokeNotice = "Ngắt ở đây không thu hồi quyền ở phía Zalo. Hãy vào OA Manager (oa.zalo.me) " +
	"› Quản lý ứng dụng và thu hồi quyền của ứng dụng ViGov trên OA này."

// ErrCommuneZaloOAInvalid is the family of request faults of the commune OA routes. Each error below
// wraps it; its text is the sentence the screen shows.
var ErrCommuneZaloOAInvalid = errors.New("zalo_oa_xa: invalid")

var (
	ErrCommuneZaloOARetireReason = fmt.Errorf("%w: Cần nhập lý do ngắt OA, tối đa %d ký tự.",
		ErrCommuneZaloOAInvalid, maxCommuneOARetireReasonRunes)
	ErrZaloOAMilestoneUnknown   = fmt.Errorf("%w: Mốc không hợp lệ.", ErrCommuneZaloOAInvalid)
	ErrZaloOATemplateNotEnabled = fmt.Errorf("%w: Chỉ mẫu đang ở trạng thái ENABLE trên Zalo mới gắn được mốc.",
		ErrCommuneZaloOAInvalid)
)

// TemplateParamsMissingError — the template cannot take a milestone because it lacks these parameter
// names (ADR 0098 #9: "nói rõ thiếu gì"). Wraps ErrCommuneZaloOAInvalid. The names are parameter NAMES
// of a template, not a person's data.
type TemplateParamsMissingError struct{ Missing []string }

func (e *TemplateParamsMissingError) Error() string {
	return ErrCommuneZaloOAInvalid.Error() + ": Mẫu thiếu tham số bắt buộc: " + strings.Join(e.Missing, ", ") + "."
}

func (e *TemplateParamsMissingError) Unwrap() error { return ErrCommuneZaloOAInvalid }

// maxCommuneOARetireReasonRunes is 0031's zalo_commune_oa_retire_reason_length.
const maxCommuneOARetireReasonRunes = 200

// NormalizeCommuneZaloOARetireReason — required, trimmed, ≤ 200 characters, no control characters
// (rule 7: a soft delete carries its reason).
func NormalizeCommuneZaloOARetireReason(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" || !utf8.ValidString(s) || utf8.RuneCountInString(s) > maxCommuneOARetireReasonRunes {
		return "", ErrCommuneZaloOARetireReason
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return "", ErrCommuneZaloOARetireReason
		}
	}
	return s, nil
}
