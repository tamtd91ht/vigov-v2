package domain

// TYPED / IMPORTED RECIPIENTS of a notice campaign (migration 0033; ADR 0099 §Sửa đổi 17/10/2026, S1–S7):
// numbers staff typed ('danh-sach-so', ≤ 1 000) or the rows of a file the browser read ('tep-nhap',
// ≤ 10 000). The pure rules live here — standard library only (this package's rule): the bounds, the
// {{variable_i}} rendering of a shared SMS text, the mask of a number that did not normalise, the scrub of
// a provider's error message, the per-number history cursor. The phone NORMALISATION is core/phone (shared
// across services, rule 2); the sealing is internal/app (core/crypto).

import (
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Bounds — the owner's S1 limits and 0032 / 0033's CHECKs.
const (
	// MaxTypedNoticeRecipients — "nhập số … ≤ 1.000" (S1).
	MaxTypedNoticeRecipients = 1000
	// MaxImportedNoticeRecipients — "nhập tệp … ≤ 10.000 dòng" (S1).
	MaxImportedNoticeRecipients = 10000
	// MaxNoticeSmsContentRunes — one SMS text, shared or per row (0032 notice_campaign_template_content_length).
	MaxNoticeSmsContentRunes = 1000
	// maxNoticeRawPhoneRunes — what one typed cell may hold before it is not even worth reading.
	maxNoticeRawPhoneRunes = 64
	// maxNoticeRowValues — values one row may carry (columns mapped to variables / parameters).
	maxNoticeRowValues = 50
	// MaxNoticeErrorMessageRunes — 0033 notice_recipient_error_message_shape.
	MaxNoticeErrorMessageRunes = 500
	// NoticeRecipientPageSize — one page of the per-number history.
	NoticeRecipientPageSize = 100
)

var (
	ErrNoticeRecipients = fmt.Errorf("%w: recipients: required and non-empty for danh-sach-so / tep-nhap (at most %d / %d), "+
		"forbidden for a directory audience", ErrNoticeInvalid, MaxTypedNoticeRecipients, MaxImportedNoticeRecipients)
	ErrNoticeContentMode = fmt.Errorf("%w: sms_content_mode: cot-trong-tep or noi-dung-chung for SMS to typed numbers, "+
		"absent otherwise; no template for SMS to typed numbers", ErrNoticeInvalid)
	ErrNoticeContent = fmt.Errorf("%w: content: required for noi-dung-chung, at most %d characters, absent otherwise",
		ErrNoticeInvalid, MaxNoticeSmsContentRunes)
	ErrNoticeRowContent = fmt.Errorf("%w: a row's message is empty or longer than %d characters", ErrNoticeInvalid,
		MaxNoticeSmsContentRunes)
	ErrNoticeRowValues = fmt.Errorf("%w: a row's values: at most %d, keys and values printable, values at most %d characters",
		ErrNoticeInvalid, maxNoticeRowValues, MaxNoticeParamValueRune)
)

// NoticeMissingVariablesError — a shared SMS text names {{variable_i}} that no row provides (400 at the
// send, listed in the preview). Missing names the variables — never a value.
type NoticeMissingVariablesError struct{ Missing []string }

func (e *NoticeMissingVariablesError) Error() string {
	return "notice_campaign: variables no row provides: " + strings.Join(e.Missing, ", ")
}

// Unwrap places it in the 400 family.
func (e *NoticeMissingVariablesError) Unwrap() error { return ErrNoticeInvalid }

// NoticeRowContentError — row Row (1-based, in the order the rows were sent) has an empty SMS text, or one
// longer than 1 000 characters once its variables are filled (400; ErrNoticeRowContent's family). The row
// number names the line — never its content.
type NoticeRowContentError struct{ Row int }

func (e *NoticeRowContentError) Error() string {
	return fmt.Sprintf("%v (row %d)", ErrNoticeRowContent, e.Row)
}

// Unwrap places it under ErrNoticeRowContent (and so ErrNoticeInvalid).
func (e *NoticeRowContentError) Unwrap() error { return ErrNoticeRowContent }

// MaxTypedRecipientsFor is the row ceiling of a typed audience kind (0 for any other kind).
func MaxTypedRecipientsFor(kind string) int {
	switch kind {
	case NoticeAudienceTypedList:
		return MaxTypedNoticeRecipients
	case NoticeAudienceImportedFile:
		return MaxImportedNoticeRecipients
	}
	return 0
}

// ValidateTypedSmsSource — SMS to typed numbers: an eSMS account and NO template (S3), and one of the two
// content modes.
func ValidateTypedSmsSource(oaRowID, smsAccountID, templateRef, mode string) error {
	if smsAccountID == "" || oaRowID != "" || !validOpaqueID(smsAccountID) {
		return ErrNoticeSource
	}
	if templateRef != "" || (mode != SmsContentPerRow && mode != SmsContentShared) {
		return ErrNoticeContentMode
	}
	return nil
}

// NormalizeNoticeMessageText checks one SMS text (shared, or one row's): trimmed, non-empty, ≤ 1 000
// characters, valid UTF-8, no control character other than a line break or a tab — an SMS may run over
// several lines.
func NormalizeNoticeMessageText(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" || !utf8.ValidString(s) || utf8.RuneCountInString(s) > MaxNoticeSmsContentRunes {
		return "", false
	}
	for _, r := range s {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return "", false
		}
	}
	return s, true
}

// NormalizeNoticeRowValues checks one row's values (≤ 50, keys printable ≤ 64, values ≤ 500 characters with
// no control character) and trims each value. An EMPTY value is kept: "an empty cell renders as empty".
func NormalizeNoticeRowValues(in map[string]string) (map[string]string, error) {
	if len(in) > maxNoticeRowValues {
		return nil, ErrNoticeRowValues
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		if k == "" || !utf8.ValidString(k) || utf8.RuneCountInString(k) > maxNoticeParamKeyRunes || hasControl(k) {
			return nil, ErrNoticeRowValues
		}
		v = strings.TrimSpace(v)
		if !utf8.ValidString(v) || utf8.RuneCountInString(v) > MaxNoticeParamValueRune || hasControl(v) {
			return nil, ErrNoticeRowValues
		}
		out[k] = v
	}
	return out, nil
}

// RawPhoneTooLong — a typed cell longer than any number could be. It is still recorded (masked) as an
// invalid number, never refused: S5 keeps every number in the history.
func RawPhoneTooLong(raw string) bool { return utf8.RuneCountInString(raw) > maxNoticeRawPhoneRunes }

// ---- {{variable_i}} -------------------------------------------------------------------------------------

// noticeVariable is the one variable spelling of S3: {{variable_<n>}}, nothing else. Any other {{…}} is
// ordinary text and goes out as typed.
var noticeVariable = regexp.MustCompile(`\{\{variable_([0-9]{1,3})\}\}`)

// NoticeContentVariables lists the variables a shared text names ("variable_1", …), each once, in order of
// first appearance.
func NoticeContentVariables(content string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, m := range noticeVariable.FindAllStringSubmatch(content, -1) {
		name := "variable_" + m[1]
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}

// RenderNoticeContent fills every {{variable_i}} of content with values["variable_i"] — "" when the row has
// no such value (an empty cell renders as empty). keep names variables to leave AS WRITTEN (the preview's
// missing ones), so the staff member sees what is not mapped yet.
func RenderNoticeContent(content string, values map[string]string, keep map[string]bool) string {
	return noticeVariable.ReplaceAllStringFunc(content, func(v string) string {
		name := strings.TrimSuffix(strings.TrimPrefix(v, "{{"), "}}")
		if keep[name] {
			return v
		}
		return values[name]
	})
}

// MissingNoticeVariables — the variables content names that NO row's values carries as a key. A key that is
// present with an empty value is provided (an empty cell), not missing.
func MissingNoticeVariables(content string, rows []map[string]string) []string {
	missing := []string{}
	for _, name := range NoticeContentVariables(content) {
		found := false
		for _, r := range rows {
			if _, ok := r[name]; ok {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, name)
		}
	}
	return missing
}

// ---- masks ----------------------------------------------------------------------------------------------

// MaskInvalidNoticePhone masks a typed value that did NOT normalise to a mobile number, so the history can
// still show "which line it was" while 0032's notice_recipient_phone_must_be_masked holds: a '*' present,
// at most six digits, at most 20 characters. Only the DIGITS of the input are considered (letters and
// separators say nothing a person could be found by, and they are dropped rather than echoed):
//
//	no digit             "*"
//	1–7 digits           one '*' per digit (MaskPhone's rule for a short value)
//	8+ digits            first 2 + '*'… + last 4, the stars capped so the whole is ≤ 20
func MaskInvalidNoticePhone(raw string) string {
	var d []byte
	for i := 0; i < len(raw); i++ {
		if raw[i] >= '0' && raw[i] <= '9' {
			d = append(d, raw[i])
		}
	}
	switch {
	case len(d) == 0:
		return "*"
	case len(d) < 8:
		return strings.Repeat("*", len(d))
	}
	stars := min(len(d)-6, 14)
	return string(d[:2]) + strings.Repeat("*", stars) + string(d[len(d)-4:])
}

// longDigitRun — seven or more digits in a row: a phone number, a national id, an account number. A
// provider's message may echo the number it refused; none of that reaches the history (rule 3).
var longDigitRun = regexp.MustCompile(`[0-9]{7,}`)

// ScrubProviderMessage prepares a provider's error message for notice_recipient.error_message (S5): control
// characters become spaces, whitespace collapses, every run of 7+ digits becomes "***", the result is cut
// to 500 characters. "" when nothing readable is left. The input is never logged by any caller.
func ScrubProviderMessage(s string) string {
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, " ")
	}
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	s = longDigitRun.ReplaceAllString(s, "***")
	if utf8.RuneCountInString(s) > MaxNoticeErrorMessageRunes {
		s = strings.TrimSpace(string([]rune(s)[:MaxNoticeErrorMessageRunes]))
	}
	return s
}

// ---- the per-number history ---------------------------------------------------------------------------------

// NoticeRecipientView is one row of GET notice-campaigns/{id}/recipients: the MASKED number and the outcome.
// Never the sealed columns, never the ref (a directory ref is identity's key, not the screen's).
type NoticeRecipientView struct {
	ID           string
	PhoneMasked  string
	Status       string
	FailureClass string
	ErrorMessage string
	SentAt       time.Time
	CreatedAt    time.Time
}

// ErrNoticeRecipientCursor — the history cursor could not be read (400).
var ErrNoticeRecipientCursor = errors.New("notice_recipient: unreadable cursor")

// EncodeNoticeRecipientCursor — opaque to the client: base64url of the last row id of the page (rows of one
// campaign share created_at, so the id alone orders them).
func EncodeNoticeRecipientCursor(lastID string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(lastID))
}

// DecodeNoticeRecipientCursor reads EncodeNoticeRecipientCursor's output; "" is the first page.
func DecodeNoticeRecipientCursor(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil || len(b) == 0 || !validOpaqueID(string(b)) {
		return "", fmt.Errorf("%w: %w", ErrNoticeCursor, ErrNoticeRecipientCursor)
	}
	return string(b), nil
}

// ---- the sealed payload -------------------------------------------------------------------------------------

// NoticePayload is what a typed row's payload_sealed holds (JSON, sealed): the final SMS text, or the ZNS
// parameters merged for this row. Never logged; cleared after the provider call.
type NoticePayload struct {
	Text   string            `json:"text,omitempty"`
	Params map[string]string `json:"params,omitempty"`
}

// ---- the column mapping of an imported file ---------------------------------------------------------------

// Bounds of the mapping (owner 17/10/2026: "lịch sử nên lưu thông tin map biến").
const (
	maxNoticeMappingHeaderRunes = 100
	maxNoticeMappingEntries     = 50
)

// ErrNoticeMapping — the mapping is absent for tep-nhap, present for another kind, or malformed / not
// matching the content or the template (400).
var ErrNoticeMapping = fmt.Errorf("%w: mapping: required for tep-nhap (phone_column at least), forbidden otherwise; "+
	"column headers ≤ %d characters, ≤ %d entries; content_column for SMS cot-trong-tep, variables for SMS "+
	"noi-dung-chung (exactly the content's variables), params for ZNS (template parameters only)",
	ErrNoticeInvalid, maxNoticeMappingHeaderRunes, maxNoticeMappingEntries)

// NoticeRecipientMapping is which column of the imported file (its row-1 HEADER, as written) fed what: the
// number, each row's own SMS text, each {{variable_i}}, each ZNS parameter. Stored on the campaign
// (notice_campaign.recipient_mapping, 0033) so the history shows what was mapped into what — the per-row
// VALUES are not kept anywhere beyond the sealed payload, wiped at the outcome (S6).
type NoticeRecipientMapping struct {
	PhoneColumn   string            `json:"phone_column"`
	ContentColumn string            `json:"content_column,omitempty"`
	Variables     map[string]string `json:"variables,omitempty"`
	Params        map[string]string `json:"params,omitempty"`
}

func validMappingHeader(s string) (string, bool) {
	s = strings.TrimSpace(s)
	return s, s != "" && utf8.ValidString(s) && utf8.RuneCountInString(s) <= maxNoticeMappingHeaderRunes && !hasControl(s)
}

func normalizeMappingEntries(in map[string]string) (map[string]string, bool) {
	if len(in) == 0 {
		return nil, true
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		if k == "" || !utf8.ValidString(k) || utf8.RuneCountInString(k) > maxNoticeParamKeyRunes || hasControl(k) {
			return nil, false
		}
		h, ok := validMappingHeader(v)
		if !ok {
			return nil, false
		}
		out[k] = h
	}
	return out, true
}

// NormalizeNoticeMapping checks a mapping against the audience kind, the channel and the content mode, and
// returns it trimmed. kind != tep-nhap: m must be nil. sharedContent is the SMS shared text (its variables
// must be exactly the mapped ones); declared are the ZNS template's parameters (params keys must be among
// them). A shared text naming a variable the mapping lacks is *NoticeMissingVariablesError.
func NormalizeNoticeMapping(m *NoticeRecipientMapping, kind, channel, smsMode, sharedContent string,
	declared []TemplateParam) (*NoticeRecipientMapping, error) {

	if kind != NoticeAudienceImportedFile {
		if m != nil {
			return nil, ErrNoticeMapping
		}
		return nil, nil
	}
	if m == nil {
		return nil, ErrNoticeMapping
	}
	phoneCol, ok := validMappingHeader(m.PhoneColumn)
	if !ok {
		return nil, ErrNoticeMapping
	}
	out := &NoticeRecipientMapping{PhoneColumn: phoneCol}
	vars, ok1 := normalizeMappingEntries(m.Variables)
	params, ok2 := normalizeMappingEntries(m.Params)
	if !ok1 || !ok2 || len(vars)+len(params) > maxNoticeMappingEntries {
		return nil, ErrNoticeMapping
	}
	switch {
	case channel == NoticeChannelSMS && smsMode == SmsContentPerRow:
		col, ok := validMappingHeader(m.ContentColumn)
		if !ok || len(vars) > 0 || len(params) > 0 {
			return nil, ErrNoticeMapping
		}
		out.ContentColumn = col
	case channel == NoticeChannelSMS && smsMode == SmsContentShared:
		if strings.TrimSpace(m.ContentColumn) != "" || len(params) > 0 {
			return nil, ErrNoticeMapping
		}
		named := NoticeContentVariables(sharedContent)
		inContent := make(map[string]bool, len(named))
		missing := []string{}
		for _, v := range named {
			inContent[v] = true
			if _, ok := vars[v]; !ok {
				missing = append(missing, v)
			}
		}
		for v := range vars {
			if !inContent[v] {
				return nil, ErrNoticeMapping // a mapped variable the text never uses
			}
		}
		if len(missing) > 0 {
			return nil, &NoticeMissingVariablesError{Missing: missing}
		}
		out.Variables = vars
	case channel == NoticeChannelZNS:
		if strings.TrimSpace(m.ContentColumn) != "" || len(vars) > 0 {
			return nil, ErrNoticeMapping
		}
		known := make(map[string]bool, len(declared))
		for _, p := range declared {
			known[p.Name] = true
		}
		for k := range params {
			if !known[k] {
				return nil, ErrNoticeMapping
			}
		}
		out.Params = params
	default:
		return nil, ErrNoticeMapping
	}
	return out, nil
}
