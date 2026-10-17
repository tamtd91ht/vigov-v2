package domain

// NOTICE CAMPAIGNS (migration 0032; ADR 0099 #4–#9) — "Gửi tin ZNS / SMS": one campaign is one message
// template sent, through ONE source the staff member picked (an OA of the commune for ZNS, an eSMS account
// for SMS), to every recipient of one audience. The campaign row and every recipient row are written in
// ONE transaction with the audit entry; a background sender then sends the rows in batches.
//
// THE FULL PHONE NUMBER IS NEVER A FIELD HERE. A recipient carries the directory row code (`Ref`) and the
// MASKED number; the full number is asked of identity per batch, at send time, and lives only in the
// sender's local variables (ADR 0099 #5). Standard library only (this package's rule): the mapping onto
// identity's audience enum and onto the providers' outcome classes lives in internal/app, which may import
// both.

import (
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// The channels — 0032 notice_campaign_channel_known. 'zalo-zns' is 0004's spelling of the ZNS channel.
const (
	NoticeChannelZNS = "zalo-zns"
	NoticeChannelSMS = "sms"
)

// The audience kinds — 0032 notice_campaign_audience_kind_known (ADR 0011 values, decided 17/10/2026).
const (
	NoticeAudienceCitizenAll   = "nguoi-dan-tat-ca"
	NoticeAudienceCitizenGroup = "nguoi-dan-theo-nhom"
	NoticeAudienceCitizenTag   = "nguoi-dan-theo-nhan"
	NoticeAudienceStaffAll     = "can-bo-tat-ca"
	NoticeAudienceStaffUnit    = "can-bo-theo-bo-phan"
)

// The recipient kinds — 0032 notice_recipient_kind_known.
const (
	NoticeRecipientCitizen = "nguoi-dan"
	NoticeRecipientStaff   = "can-bo"
)

// The campaign statuses — 0032 notice_campaign_status_known. Forward only: dang-gui → one terminal value.
const (
	NoticeCampaignSending    = "dang-gui"
	NoticeCampaignSent       = "da-gui"
	NoticeCampaignPartlySent = "gui-mot-phan"
	NoticeCampaignFailed     = "that-bai"
)

// The recipient statuses — 0032 notice_recipient_status_known.
const (
	NoticeRecipientPending = "cho-gui"
	NoticeRecipientSent    = "da-gui"
	NoticeRecipientFailed  = "that-bai"
	NoticeRecipientSkipped = "bo-qua"
)

// THE FAILURE CLASSES of a recipient row — the CLOSED list 0032 left "OWED BY GO" (its CHECK is a shape
// check only). ADR 0011 values. A class, never a provider's message: the message may echo a number or a
// key, the class cannot. The three sms_account check classes are reused verbatim where they mean the same
// thing, so one word names one fault on both screens.
const (
	NoticeFailKeyRejected       = SmsCheckKeyRejected        // eSMS refused ApiKey / SecretKey
	NoticeFailBrandnameInactive = SmsCheckBrandnameInactive  // eSMS: Brandname not active on the account
	NoticeFailTemplateRejected  = "sai-mau"                  // the provider matched no approved template (eSMS 146), or the ZBS template is no longer ENABLE
	NoticeFailUnavailable       = SmsCheckUnavailable        // retries ran out on a timeout / 5xx / network fault
	NoticeFailRateLimited       = "gioi-han-tan-suat"        // retries ran out on 429
	NoticeFailRejected          = SmsCheckRejected           // any other refusal (balance, IP, locked account, …)
	NoticeFailMalformed         = SmsCheckMalformed          // the provider answered something unreadable
	NoticeFailInvalidNumber     = "so-khong-hop-le"          // the provider refused THIS number
	NoticeFailOANeedsReauth     = "oa-can-uy-quyen-lai"      // the OA's token was refused; the OA admin must authorize again
	NoticeFailSourceRetired     = "nguon-ngung-dung"         // the OA was disconnected / the account retired after the campaign was written
	NoticeFailRecipientInactive = "nguoi-nhan-ngung-lien-he" // identity no longer delivers this ref (switched off, removed, locked, no number)
)

// NoticeFailureClasses is the closed list, in the order above — for the test that keeps it inside 0032's
// shape CHECK, and for the migration that will close that CHECK.
func NoticeFailureClasses() []string {
	return []string{NoticeFailKeyRejected, NoticeFailBrandnameInactive, NoticeFailTemplateRejected,
		NoticeFailUnavailable, NoticeFailRateLimited, NoticeFailRejected, NoticeFailMalformed,
		NoticeFailInvalidNumber, NoticeFailOANeedsReauth, NoticeFailSourceRetired, NoticeFailRecipientInactive}
}

// The commune audit_log verbs (values, ADR 0011).
const (
	// ActionSendNoticeCampaign — a member of staff sent a campaign: the campaign row and every recipient
	// row, in the same transaction.
	ActionSendNoticeCampaign = "gui_dot_tin"
	// ActionNoticeSendBatch — the background sender (system principal, rule 6 invariant 6) settled one
	// batch: each row's outcome and every campaign it finished.
	ActionNoticeSendBatch = "ket_qua_gui_dot_tin"
)

// Bounds — 0032's CHECKs and the identity contract's ceilings.
const (
	MaxNoticeTitleRunes     = 200
	maxNoticeTemplateRef    = 64
	MaxNoticeAudienceIDs    = 50 // identity.proto: at most 50 group / tag / unit ids
	maxNoticeAudienceID     = 64
	maxNoticeTagRunes       = 100
	maxNoticeParams         = 50
	maxNoticeParamKeyRunes  = 64
	MaxNoticeParamValueRune = 500
	// NoticeListPageSize — the "Đã gửi" tab's page.
	NoticeListPageSize = 50
	// MaxNoticeSendAttempts — after this many attempts that ended in a RETRYABLE fault the row is
	// that-bai with the fault's class (ADR 0099 #8; transaction-boundaries gui_dot_tin_zns_sms (b)).
	MaxNoticeSendAttempts = 5
)

// ErrNoticeInvalid is the family of request faults of the campaign routes. Each error below wraps it; the
// HTTP layer answers with its own fixed sentence.
var ErrNoticeInvalid = errors.New("notice_campaign: invalid")

var (
	ErrNoticeChannel   = fmt.Errorf("%w: channel must be zalo-zns or sms", ErrNoticeInvalid)
	ErrNoticeSource    = fmt.Errorf("%w: zalo-zns needs oa_id and no sms_account_id; sms the reverse", ErrNoticeInvalid)
	ErrNoticeTemplate  = fmt.Errorf("%w: template_ref required, at most %d characters", ErrNoticeInvalid, maxNoticeTemplateRef)
	ErrNoticeAudience  = fmt.Errorf("%w: unknown audience kind, or ids/tags not matching the kind", ErrNoticeInvalid)
	ErrNoticeTitle     = fmt.Errorf("%w: title required, at most %d characters, no control characters", ErrNoticeInvalid, MaxNoticeTitleRunes)
	ErrNoticeParams    = fmt.Errorf("%w: params: at most %d, keys and values printable, values at most %d characters", ErrNoticeInvalid, maxNoticeParams, MaxNoticeParamValueRune)
	ErrNoticeParamLong = fmt.Errorf("%w: a parameter value is longer than the template allows", ErrNoticeInvalid)
	ErrNoticeCursor    = fmt.Errorf("%w: unreadable cursor", ErrNoticeInvalid)
)

// NoticeAudience is the "Gửi cho ai" choice as stored: the kind and the chosen group / tag / unit ids
// (0032 audience_ids — empty exactly for the two "all" kinds).
type NoticeAudience struct {
	Kind string
	IDs  []string
}

// RecipientKind is the directory the audience's refs point into.
func (a NoticeAudience) RecipientKind() string {
	if a.Kind == NoticeAudienceStaffAll || a.Kind == NoticeAudienceStaffUnit {
		return NoticeRecipientStaff
	}
	return NoticeRecipientCitizen
}

// NoticeCampaign is one notice_campaign row.
type NoticeCampaign struct {
	ID           string
	Title        string
	Channel      string
	OARowID      string // ZNS only
	SmsAccountID string // SMS only
	TemplateRef  string // ZBS template_id (ZNS) or sms_template.id (SMS)
	TemplateName string
	// TemplateContent — SMS: the template text with its `{P1,20}` slots, as it stood at send. ZNS: "".
	TemplateContent string
	// Params — what staff typed for the whole campaign (ADR 0099 #6): ZNS keyed by Zalo's param name,
	// SMS keyed by slot position ("1", "2", … — names may repeat, see SmsTemplateParam).
	Params         map[string]string
	Audience       NoticeAudience
	RecipientCount int
	SkippedCount   int
	// UnitPrice / EstimatedCost — decimal text ("200.00") or both "" (0032 notice_campaign_estimate_pair).
	UnitPrice     string
	EstimatedCost string
	Status        string
	SentCount     int
	FailedCount   int
	CreatedAt     time.Time
	CreatedBy     string // staff business code (rule 6 invariant 8)
	FinishedAt    time.Time
}

// Subject is the audit subject. A campaign has no business code of its own; its row id is the identity
// (the sms-account precedent).
func (c NoticeCampaign) Subject() string { return "notice-campaign/" + c.ID }

// Finished — the campaign left dang-gui.
func (c NoticeCampaign) Finished() bool { return c.Status != NoticeCampaignSending }

// NoticeRecipient is one notice_recipient row to be written: the ref and the MASKED number only.
type NoticeRecipient struct {
	ID          string
	Ref         string
	Kind        string
	PhoneMasked string
}

// DueNoticeRecipient is one row the sender claimed.
type DueNoticeRecipient struct {
	ID         string
	CampaignID string
	Ref        string
	Kind       string
	Attempts   int // AFTER the claim counted this attempt
}

// ---- validation -----------------------------------------------------------------------------------------

// NormalizeNoticeTitle — trimmed, required, ≤ 200 characters (0032 notice_campaign_title_shape), no control
// characters.
func NormalizeNoticeTitle(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" || !utf8.ValidString(s) || utf8.RuneCountInString(s) > MaxNoticeTitleRunes || hasControl(s) {
		return "", ErrNoticeTitle
	}
	return s, nil
}

// ValidateNoticeSource checks the channel and that exactly the source of that channel is named (0032
// notice_campaign_source_matches_channel), and the template reference's shape.
func ValidateNoticeSource(channel, oaRowID, smsAccountID, templateRef string) error {
	switch channel {
	case NoticeChannelZNS:
		if oaRowID == "" || smsAccountID != "" {
			return ErrNoticeSource
		}
	case NoticeChannelSMS:
		if smsAccountID == "" || oaRowID != "" {
			return ErrNoticeSource
		}
	default:
		return ErrNoticeChannel
	}
	if !validOpaqueID(oaRowID) || !validOpaqueID(smsAccountID) {
		return ErrNoticeSource
	}
	t := strings.TrimSpace(templateRef)
	if t == "" || t != templateRef || len(t) > maxNoticeTemplateRef || hasControl(t) {
		return ErrNoticeTemplate
	}
	return nil
}

// validOpaqueID — empty, or a printable ASCII token of at most 64 bytes (a row id, a group id).
func validOpaqueID(s string) bool {
	if len(s) > maxNoticeAudienceID {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '!' || s[i] > '~' {
			return false
		}
	}
	return true
}

// NormalizeNoticeAudience checks the kind and that exactly the list the kind uses is non-empty (ids for
// groups and units, tags for tags), ≤ 50 entries, deduplicated, order kept.
func NormalizeNoticeAudience(kind string, ids, tags []string) (NoticeAudience, error) {
	var list []string
	switch kind {
	case NoticeAudienceCitizenAll, NoticeAudienceStaffAll:
		if len(ids) > 0 || len(tags) > 0 {
			return NoticeAudience{}, ErrNoticeAudience
		}
		return NoticeAudience{Kind: kind, IDs: []string{}}, nil
	case NoticeAudienceCitizenGroup, NoticeAudienceStaffUnit:
		if len(tags) > 0 {
			return NoticeAudience{}, ErrNoticeAudience
		}
		list = ids
	case NoticeAudienceCitizenTag:
		if len(ids) > 0 {
			return NoticeAudience{}, ErrNoticeAudience
		}
		list = tags
	default:
		return NoticeAudience{}, ErrNoticeAudience
	}
	out := make([]string, 0, len(list))
	seen := map[string]bool{}
	for _, v := range list {
		v = strings.TrimSpace(v)
		switch {
		case v == "":
			return NoticeAudience{}, ErrNoticeAudience
		case kind == NoticeAudienceCitizenTag && (!utf8.ValidString(v) || utf8.RuneCountInString(v) > maxNoticeTagRunes || hasControl(v)):
			return NoticeAudience{}, ErrNoticeAudience
		case kind != NoticeAudienceCitizenTag && !validOpaqueID(v):
			return NoticeAudience{}, ErrNoticeAudience
		}
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	if len(out) == 0 || len(out) > MaxNoticeAudienceIDs {
		return NoticeAudience{}, ErrNoticeAudience
	}
	return NoticeAudience{Kind: kind, IDs: out}, nil
}

// NormalizeNoticeParams trims every value and drops empty ones (an empty value is a MISSING one — the
// preview lists it, the send refuses it). Keys and values are printable, ≤ 50 pairs.
func NormalizeNoticeParams(in map[string]string) (map[string]string, error) {
	if len(in) > maxNoticeParams {
		return nil, ErrNoticeParams
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		if k == "" || !utf8.ValidString(k) || utf8.RuneCountInString(k) > maxNoticeParamKeyRunes || hasControl(k) {
			return nil, ErrNoticeParams
		}
		v = strings.TrimSpace(v)
		if !utf8.ValidString(v) || utf8.RuneCountInString(v) > MaxNoticeParamValueRune || hasControl(v) {
			return nil, ErrNoticeParams
		}
		if v != "" {
			out[k] = v
		}
	}
	return out, nil
}

// maskedPhonePattern is 0032 notice_recipient_phone_must_be_masked, in Go: the mask character present, at
// most six digits left, at most 20 characters.
var maskedPhonePattern = regexp.MustCompile(`^[0-9+*]{1,20}$`)

// ValidMaskedPhone — what identity returned as the masked number satisfies 0032's CHECK. A value that
// does not is a broken contract, refused before the transaction rather than by it.
func ValidMaskedPhone(s string) bool {
	if !maskedPhonePattern.MatchString(s) || !strings.Contains(s, "*") {
		return false
	}
	digits := 0
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			digits++
		}
	}
	return digits <= 6
}

// ---- parameters and content --------------------------------------------------------------------------

// SmsSlotKey is the params key of the i-th slot (0-based i → "1", "2", …). SLOTS ARE KEYED BY POSITION,
// not by name, because eSMS templates repeat a name with different values (SmsTemplateParam).
func SmsSlotKey(i int) string { return strconv.Itoa(i + 1) }

// PreviewSmsContent fills the slots of an SMS template that have a value and leaves the others exactly as
// written (`{P2,20}`), returning the keys still missing in order. A value longer than its slot, or one
// carrying a brace, is ErrNoticeParamLong / ErrNoticeParams — never silently cut.
func PreviewSmsContent(content string, params map[string]string) (string, []string, error) {
	slots, err := ParseSmsTemplateParams(content)
	if err != nil {
		return "", nil, err
	}
	missing := []string{}
	values := make([]string, len(slots))
	for i, s := range slots {
		v, ok := params[SmsSlotKey(i)]
		if !ok {
			missing = append(missing, SmsSlotKey(i))
			continue
		}
		if strings.ContainsAny(v, "{}") {
			return "", nil, ErrNoticeParams
		}
		if utf8.RuneCountInString(v) > s.MaxLen {
			return "", nil, ErrNoticeParamLong
		}
		values[i] = v
	}
	i := 0
	out := smsPlaceholder.ReplaceAllStringFunc(content, func(slot string) string {
		v := values[i]
		i++
		if v == "" {
			return slot
		}
		return v
	})
	return out, missing, nil
}

// SmsSlotValues is the ordered value list FillSmsTemplate takes, from params keyed by position.
func SmsSlotValues(content string, params map[string]string) ([]string, error) {
	slots, err := ParseSmsTemplateParams(content)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(slots))
	for i := range slots {
		out[i] = params[SmsSlotKey(i)]
	}
	return out, nil
}

// MissingZnsParams returns the template's declared parameters (in Zalo's order) with no value, and checks
// each given value against its declared length. EVERY declared parameter is treated as required: Zalo
// refuses a template message with a declared parameter absent, and a campaign has one value per parameter
// for everyone (ADR 0099 #6). Keys that are not parameters of the template are ignored (and not stored —
// see ZnsTemplateData).
func MissingZnsParams(declared []TemplateParam, params map[string]string) ([]string, error) {
	missing := []string{}
	for _, p := range declared {
		v, ok := params[p.Name]
		if !ok {
			missing = append(missing, p.Name)
			continue
		}
		if p.MaxLength > 0 && utf8.RuneCountInString(v) > p.MaxLength {
			return nil, ErrNoticeParamLong
		}
	}
	return missing, nil
}

// ZnsTemplateData keeps only the declared parameters' values — what is stored and what crosses to Zalo
// (rule 3, invariant 6: only the fields actually needed).
func ZnsTemplateData(declared []TemplateParam, params map[string]string) map[string]string {
	out := make(map[string]string, len(declared))
	for _, p := range declared {
		if v, ok := params[p.Name]; ok {
			out[p.Name] = v
		}
	}
	return out
}

// SmsTemplateData keeps only the slots' values (keys "1".."n").
func SmsTemplateData(content string, params map[string]string) map[string]string {
	slots, err := ParseSmsTemplateParams(content)
	if err != nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(slots))
	for i := range slots {
		if v, ok := params[SmsSlotKey(i)]; ok {
			out[SmsSlotKey(i)] = v
		}
	}
	return out
}

// ---- cost ---------------------------------------------------------------------------------------------

// THE ZNS PRICE TABLE — ESTIMATE ONLY (ADR 0099 #7). Source: Zalo's ZBS price list as measured on
// 16/10/2026, VAT EXCLUDED: 200 ₫ per message for an ordinary template (transaction, customer care,
// promotion), 300 ₫ for the authentication / payment-request / voucher kinds. Zalo bills, not this
// system; a change of Zalo's price list changes these two constants, and an estimate that is off is
// shown as an estimate — never a gate (no approval, no quota: owner 17/10/2026).
const (
	znsPriceStandard = "200.00"
	znsPricePremium  = "300.00"
)

// znsPremiumTags — Zalo's templateTag values for the 300 ₫ kinds, as far as the documentation names
// them. UNCONFIRMED AGAINST A LIVE OA (the tag spellings were read from documentation pages): a tag not
// recognised here is priced at 200 ₫ — the ordinary rate — which can only UNDER-estimate a premium
// template, and is said to be an estimate on the screen.
var znsPremiumTags = map[string]bool{"OTP": true, "AUTHENTICATION": true, "PAYMENT_REQUEST": true, "VOUCHER": true}

// ZnsUnitPrice is the estimated price of one ZNS message of a template with Zalo tag `tag`.
func ZnsUnitPrice(tag string) string {
	if znsPremiumTags[strings.ToUpper(strings.TrimSpace(tag))] {
		return znsPricePremium
	}
	return znsPriceStandard
}

// moneyPattern — a non-negative decimal with at most 12 integer digits and 2 decimals in the result
// (NUMERIC(14, 2)).
var moneyPattern = regexp.MustCompile(`^([0-9]{1,12})(?:\.([0-9]+))?$`)

// ParseMoney reads a non-negative decimal amount into hundredths ("350" → 35000, "350.5" → 35050). More
// than two decimals are refused rather than rounded: a price eSMS quotes in fractions of a đồng is not one
// this column can hold exactly.
func ParseMoney(s string) (int64, bool) {
	m := moneyPattern.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, false
	}
	whole, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return 0, false
	}
	frac := m[2]
	if len(frac) > 2 {
		if strings.Trim(frac[2:], "0") != "" {
			return 0, false
		}
		frac = frac[:2]
	}
	for len(frac) < 2 {
		frac += "0"
	}
	f, _ := strconv.ParseInt(frac, 10, 64)
	return whole*100 + f, true
}

// FormatMoney renders hundredths as decimal text with two decimals.
func FormatMoney(hundredths int64) string {
	return fmt.Sprintf("%d.%02d", hundredths/100, hundredths%100)
}

// maxMoneyHundredths — NUMERIC(14, 2) holds 12 integer digits.
const maxMoneyHundredths = 99_999_999_999_999

// EstimateCost is unit × count, both as decimal text; ok=false when the unit is unreadable or the product
// overflows the column — the caller then stores NO estimate (an estimate is never a gate, ADR 0099 #7).
func EstimateCost(unit string, count int) (string, string, bool) {
	u, ok := ParseMoney(unit)
	if !ok || count < 0 {
		return "", "", false
	}
	if u != 0 && int64(count) > maxMoneyHundredths/u {
		return "", "", false
	}
	return FormatMoney(u), FormatMoney(u * int64(count)), true
}

// ---- the sender's decisions --------------------------------------------------------------------------

// NoticeRetryBackoff is how long a row waits after its n-th attempt ended in a retryable fault: 1, 5,
// then 15 minutes (ADR 0099 #8 card). A VENDOR choice — minutes, because a campaign is not urgent and the
// provider's own rate limit is what a fast retry would hit.
func NoticeRetryBackoff(attempts int) time.Duration {
	switch {
	case attempts <= 1:
		return time.Minute
	case attempts == 2:
		return 5 * time.Minute
	}
	return 15 * time.Minute
}

// NoticeCampaignOutcome is the campaign's status from its rows' counts: dang-gui while any row is owed;
// then da-gui when every row was accepted, that-bai when none was, gui-mot-phan otherwise.
func NoticeCampaignOutcome(total, pending, sent int) string {
	switch {
	case pending > 0:
		return NoticeCampaignSending
	case sent >= total:
		return NoticeCampaignSent
	case sent == 0:
		return NoticeCampaignFailed
	}
	return NoticeCampaignPartlySent
}

// DuplicateRequestMessageID is what a row records as its provider id when eSMS answers 124 (RequestId
// already held — the earlier, ambiguous attempt reached it) and so returns NO SMSID. 0032 demands a
// provider id on a 'da-gui' row; this value says honestly which message it is — the request this row
// made, known to eSMS by RequestId = the row id — and cannot be mistaken for an SMSID.
func DuplicateRequestMessageID(rowID string) string { return "request:" + rowID }

// ---- the list cursor ------------------------------------------------------------------------------------

// NoticeCursor is the keyset position of the "Đã gửi" list: (created_at, id), newest first.
type NoticeCursor struct {
	CreatedAt time.Time
	ID        string
}

// Encode is opaque to the client: base64url of "<unix micro>.<id>".
func (c NoticeCursor) Encode() string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(c.CreatedAt.UnixMicro(), 10) + "." + c.ID))
}

// DecodeNoticeCursor reads Encode's output; "" is the first page (zero cursor, ok).
func DecodeNoticeCursor(s string) (NoticeCursor, error) {
	if s == "" {
		return NoticeCursor{}, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil || len(b) > 128 {
		return NoticeCursor{}, ErrNoticeCursor
	}
	micro, id, ok := strings.Cut(string(b), ".")
	if !ok || id == "" || !validOpaqueID(id) {
		return NoticeCursor{}, ErrNoticeCursor
	}
	n, err := strconv.ParseInt(micro, 10, 64)
	if err != nil || n <= 0 {
		return NoticeCursor{}, ErrNoticeCursor
	}
	return NoticeCursor{CreatedAt: time.UnixMicro(n).UTC(), ID: id}, nil
}
