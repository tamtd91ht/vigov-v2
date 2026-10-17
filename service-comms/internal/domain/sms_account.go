package domain

// A COMMUNE'S SMS ACCOUNTS (migration 0032; ADR 0099 #2, #3): the eSMS accounts a commune sends SMS
// through — several per commune, one may be the default the campaign picker preselects, chosen per
// campaign — and the CSKH templates eSMS lists for each (`sms_template`). Configured in web-admin
// Cấu hình › Kênh SMS under `admin.sms`. Standard library only.
//
// THE ApiKey AND SecretKey ARE NOT FIELDS HERE. They are sealed under the commune's data key and travel
// only as sealed bytes (store) or as a secret.Secret on their way to eSMS (app) — never in a value that
// could be returned, logged or compared. The additional data binding them is built in internal/app,
// where the commune is read from the context (rule 1, invariant 4).

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// SmsProviderEsms is sms_account.provider for eSMS (0032 sms_account_provider_known).
const SmsProviderEsms = "esms"

// The classes of "Kiểm tra kết nối" — 0032's sms_account_check_result_known, verbatim (ADR 0011 values).
// Never eSMS's text.
const (
	SmsCheckOK                = "thanh-cong"
	SmsCheckKeyRejected       = "sai-khoa"
	SmsCheckBrandnameInactive = "brandname-chua-kich-hoat"
	SmsCheckUnavailable       = "khong-kha-dung"
	SmsCheckRejected          = "bi-tu-choi"
	SmsCheckMalformed         = "phan-hoi-sai-dang"
)

// SmsAccount is one sms_account row as metadata.
type SmsAccount struct {
	ID        string // ULID, internal
	Provider  string // SmsProviderEsms
	Label     string // the commune's name for it ("eSMS chính")
	Brandname string // the sender name eSMS sends under — registered with the networks, not secret
	IsDefault bool
	// LastCheckAt / LastCheckResult: both zero or both set (0032 sms_account_check_pair); the result is a
	// SmsCheck* class.
	LastCheckAt     time.Time
	LastCheckResult string
	// LastBalance is eSMS's balance as decimal text ("36693.00"); "" unless the last check succeeded
	// (0032 sms_account_balance_from_success).
	LastBalance string
	// SetAt / SetBy: when and by whom (staff CODE, rule 6 invariant 8) the keys / brandname were set.
	SetAt     time.Time
	SetBy     string
	CreatedAt time.Time
}

// Subject is the audit subject of an act on this account. An account has no business code of its own;
// its row id is the identity that outlives a change of label, keys or brandname (the zalo-oa-authorization
// precedent).
func (a SmsAccount) Subject() string { return "sms-account/" + a.ID }

// The commune audit_log verbs of the SMS accounts (values, ADR 0011).
const (
	ActionCreateSmsAccount = "them_tai_khoan_sms"
	ActionUpdateSmsAccount = "sua_tai_khoan_sms"
	ActionRetireSmsAccount = "ngung_dung_tai_khoan_sms"
	ActionCheckSmsAccount  = "kiem_tra_tai_khoan_sms"
	ActionSyncSmsTemplates = "nap_mau_tin_sms"
)

// Bounds from 0032's CHECKs.
const (
	MaxSmsAccountLabelRunes  = 100
	maxSmsRetireReasonRunes  = 200
	MaxSmsTemplateContent    = 1000
	maxSmsKeyBytes           = 128
	maxSmsTemplateNetworkID  = 32767 // SMALLINT
	maxSmsTemplateParamCount = 50
)

// ErrSmsAccountInvalid is the family of request faults of the SMS account routes. Each error below wraps
// it; its text is for the log — the HTTP layer answers with its own fixed sentence.
var ErrSmsAccountInvalid = errors.New("sms_account: invalid")

var (
	ErrSmsAccountLabel = fmt.Errorf("%w: label required, at most %d characters, no control characters",
		ErrSmsAccountInvalid, MaxSmsAccountLabelRunes)
	ErrSmsBrandname = fmt.Errorf("%w: brandname must be letters, digits, space . _ -, starting with a letter or digit, at most 32",
		ErrSmsAccountInvalid)
	ErrSmsKeyShape = fmt.Errorf("%w: ApiKey / SecretKey must be printable ASCII without spaces, at most %d bytes",
		ErrSmsAccountInvalid, maxSmsKeyBytes)
	ErrSmsKeysRequired  = fmt.Errorf("%w: a new account needs both ApiKey and SecretKey", ErrSmsAccountInvalid)
	ErrSmsRetireReason  = fmt.Errorf("%w: reason required, at most %d characters", ErrSmsAccountInvalid, maxSmsRetireReasonRunes)
	ErrSmsNothingToSave = fmt.Errorf("%w: nothing to change", ErrSmsAccountInvalid)
)

// NormalizeSmsAccountLabel — trimmed, required, ≤ 100 characters (0032 sms_account_label_shape), no
// control characters.
func NormalizeSmsAccountLabel(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" || !utf8.ValidString(s) || utf8.RuneCountInString(s) > MaxSmsAccountLabelRunes || hasControl(s) {
		return "", ErrSmsAccountLabel
	}
	return s, nil
}

// brandnamePattern is 0032's sms_account_brandname_shape.
var brandnamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 ._-]{0,31}$`)

// NormalizeSmsBrandname — trimmed, then 0032's shape. Case is kept: eSMS matches the registered name.
func NormalizeSmsBrandname(s string) (string, error) {
	s = strings.TrimSpace(s)
	if !brandnamePattern.MatchString(s) {
		return "", ErrSmsBrandname
	}
	return s, nil
}

// ValidSmsKey — an ApiKey / SecretKey as eSMS issues them: printable ASCII, no space, ≤ 128 bytes. The
// check runs on the bytes and never quotes them.
func ValidSmsKey(b []byte) bool {
	if len(b) == 0 || len(b) > maxSmsKeyBytes {
		return false
	}
	for _, c := range b {
		if c < '!' || c > '~' {
			return false
		}
	}
	return true
}

// NormalizeSmsRetireReason — required, trimmed, ≤ 200 characters, no control characters (rule 7: a soft
// delete carries its reason; 0032 sms_account_retire_reason_length).
func NormalizeSmsRetireReason(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" || !utf8.ValidString(s) || utf8.RuneCountInString(s) > maxSmsRetireReasonRunes || hasControl(s) {
		return "", ErrSmsRetireReason
	}
	return s, nil
}

// ---- templates ----------------------------------------------------------------------------------------

// SmsTemplate is one sms_template row: one eSMS CSKH template on one network, under one account.
type SmsTemplate struct {
	ID                 string // ULID, internal
	AccountID          string
	ProviderTemplateID string // eSMS's TempId, verbatim
	NetworkID          int    // eSMS's NetworkID
	Content            string // eSMS's TempContent, placeholders included
	Params             []SmsTemplateParam
	SyncedAt           time.Time
	VanishedAt         time.Time // zero while eSMS still lists it
}

// SmsTemplateParam is one placeholder of a template, IN ORDER OF APPEARANCE: `{P1,20}` is Name "P1",
// MaxLen 20.
//
// A PLACEHOLDER IS A SLOT, NOT A NAMED VARIABLE. eSMS's own documented sample registers
// "… {P2,20} VND luc {P2,20} cho don hang {P1,20}" and "{P2,50} … {P2,50} coupon {P2,20}": the same name
// appears several times, with DIFFERENT lengths. So the name cannot be what identifies a value; each
// occurrence is filled separately, by position (FillSmsTemplate). The name is kept because it is what
// the commune registered and what the screen can show beside the slot.
type SmsTemplateParam struct {
	Name   string
	MaxLen int
}

// smsPlaceholder matches one `{Pn,len}` slot. Spaces after the comma are tolerated; nothing else is.
var smsPlaceholder = regexp.MustCompile(`\{(P[0-9]{1,3}),\s*([0-9]{1,4})\}`)

// ErrSmsTemplateShape — a template eSMS returned that 0032 cannot store or this package cannot read.
var ErrSmsTemplateShape = errors.New("sms_template: unreadable template")

// providerTemplateIDPattern is 0032's sms_template_provider_template_id_shape.
var providerTemplateIDPattern = regexp.MustCompile(`^[0-9A-Za-z_-]{1,64}$`)

// ValidateSmsTemplate checks one template from eSMS against 0032's CHECKs before it is written: TempId
// shape, a positive SMALLINT network, content 1..1000 characters.
func ValidateSmsTemplate(providerTemplateID string, networkID int, content string) error {
	if !providerTemplateIDPattern.MatchString(providerTemplateID) || networkID <= 0 || networkID > maxSmsTemplateNetworkID ||
		strings.TrimSpace(content) == "" || !utf8.ValidString(content) || utf8.RuneCountInString(content) > MaxSmsTemplateContent {
		return ErrSmsTemplateShape
	}
	return nil
}

// ParseSmsTemplateParams reads the slots out of a template's content, in order.
func ParseSmsTemplateParams(content string) ([]SmsTemplateParam, error) {
	matches := smsPlaceholder.FindAllStringSubmatch(content, -1)
	if len(matches) > maxSmsTemplateParamCount {
		return nil, ErrSmsTemplateShape
	}
	out := make([]SmsTemplateParam, 0, len(matches))
	for _, m := range matches {
		n, err := strconv.Atoi(m[2])
		if err != nil || n <= 0 {
			return nil, ErrSmsTemplateShape
		}
		out = append(out, SmsTemplateParam{Name: m[1], MaxLen: n})
	}
	return out, nil
}

// ErrSmsTemplateValue is the family of faults in the values staff typed for a template's slots.
var ErrSmsTemplateValue = errors.New("sms_template: invalid value")

var (
	ErrSmsTemplateValueCount = fmt.Errorf("%w: one value per slot, in order", ErrSmsTemplateValue)
	ErrSmsTemplateValueEmpty = fmt.Errorf("%w: every slot must be filled", ErrSmsTemplateValue)
	ErrSmsTemplateValueLong  = fmt.Errorf("%w: a value is longer than its slot allows", ErrSmsTemplateValue)
	ErrSmsTemplateValueShape = fmt.Errorf("%w: a value carries a line break, a control character or a slot", ErrSmsTemplateValue)
)

// FillSmsTemplate replaces each `{Pn,len}` slot of content, in order, with values[i]. Each value is
// trimmed, required, at most its slot's length in characters, without control characters, and must not
// itself look like a slot — the result is checked to hold none, so a message never leaves with a hole
// or a raw placeholder in it (the operator would see eSMS's "sai mẫu" instead, or worse, the citizen
// would read "{P1,20}").
func FillSmsTemplate(content string, values []string) (string, error) {
	params, err := ParseSmsTemplateParams(content)
	if err != nil {
		return "", err
	}
	if len(values) != len(params) {
		return "", ErrSmsTemplateValueCount
	}
	clean := make([]string, len(values))
	for i, v := range values {
		v = strings.TrimSpace(v)
		switch {
		case v == "":
			return "", ErrSmsTemplateValueEmpty
		case !utf8.ValidString(v) || hasControl(v) || strings.ContainsAny(v, "{}"):
			return "", ErrSmsTemplateValueShape
		case utf8.RuneCountInString(v) > params[i].MaxLen:
			return "", ErrSmsTemplateValueLong
		}
		clean[i] = v
	}
	i := 0
	out := smsPlaceholder.ReplaceAllStringFunc(content, func(string) string {
		v := clean[i]
		i++
		return v
	})
	if smsPlaceholder.MatchString(out) {
		return "", ErrSmsTemplateValueShape
	}
	return out, nil
}

// SmsContentIsUnicode — eSMS's IsUnicode: 1 when the text carries anything outside 7-bit ASCII
// (Vietnamese with diacritics), 0 otherwise.
func SmsContentIsUnicode(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return true
		}
	}
	return false
}
