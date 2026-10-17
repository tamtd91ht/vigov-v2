package domain

// The RECIPIENT ATTRIBUTES a notice campaign may map a template parameter to (ADR 0101 #12, owner
// 17/10/2026): which codes exist, which a request may ask for, and how a stored value becomes the text a
// message carries. The contract is on RevealNoticeRecipientPhones / ListNoticeAttributes in identity.proto.
//
// STANDARD LIBRARY ONLY (doc.go). Values handled here ARE personal data (a name, an address, an attribute a
// commune chose to record): no refusal quotes one, and nothing here logs.

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// The static attribute codes — columns of `citizen_contact` (0033, 0036). Underscored, so most cannot
// collide with a dynamic code; `address` (7 characters) could under 0036's 6–8 rule, and the static one wins (identity.proto) — new codes are 8–12 (0037).
const (
	NoticeAttrFullName      = "full_name"
	NoticeAttrAddress       = "address"
	NoticeAttrEmail         = "email"
	NoticeAttrHouseholdCode = "household_code"
)

// Bounds of the contract.
const (
	// MaxNoticeAttributeCodes bounds attribute_codes, counted as SENT (identity.proto).
	MaxNoticeAttributeCodes = 20
	// MaxNoticeAttributeValue bounds one formatted value. Longer text is cut: the longest slot a provider
	// template allows is far shorter, and the caller cuts again to the slot.
	MaxNoticeAttributeValue = 500
)

// NoticeAttribute is one attribute a parameter may be mapped to: its code and its display label.
type NoticeAttribute struct {
	Code  string
	Label string
}

// noticeStaticAttributes — the static attributes in the order the composer lists them. The labels are the
// Vietnamese the screen shows (rule 12, invariant 2).
var noticeStaticAttributes = []NoticeAttribute{
	{NoticeAttrFullName, "Họ và tên"},
	{NoticeAttrAddress, "Địa chỉ"},
	{NoticeAttrEmail, "Email"},
	{NoticeAttrHouseholdCode, "Mã hộ"},
}

// IsNoticeStaticAttribute reports whether code is one of the static attributes.
func IsNoticeStaticAttribute(code string) bool {
	for _, a := range noticeStaticAttributes {
		if a.Code == code {
			return true
		}
	}
	return false
}

// ErrNoticeAttributeInvalid marks every attribute_codes shape the contract answers INVALID_ARGUMENT. The
// reason may name a CODE (configuration), never a value.
var ErrNoticeAttributeInvalid = errors.New("thuộc tính cho đợt gửi không hợp lệ")

// NoticeAttributesFor lists what a campaign of `kind` may map to: STAFF `full_name` only; CITIZEN the
// static attributes then every LIVE definition in `defs` order, minus a dynamic code a static one owns.
func NoticeAttributesFor(kind NoticeRecipientKind, defs []CitizenField) []NoticeAttribute {
	if kind == NoticeRecipientStaff {
		return []NoticeAttribute{noticeStaticAttributes[0]}
	}
	out := append([]NoticeAttribute{}, noticeStaticAttributes...)
	for _, d := range defs {
		if d.Live() && !IsNoticeStaticAttribute(d.Code) {
			out = append(out, NoticeAttribute{Code: d.Code, Label: d.Label})
		}
	}
	return out
}

// CheckNoticeAttributeCodes applies the contract to attribute_codes and returns them collapsed (blank and
// duplicates dropped, order kept). STAFF: `full_name` only. CITIZEN: a static code, or a code defined in
// `defs` LIVE OR RETIRED — the mapping was fixed at POST, so a field retired since still answers.
func CheckNoticeAttributeCodes(kind NoticeRecipientKind, codes []string, defs []CitizenField) ([]string, error) {
	if len(codes) > MaxNoticeAttributeCodes {
		return nil, errors.Join(ErrNoticeAttributeInvalid, errors.New("quá 20 mã thuộc tính trong một lần mở"))
	}
	defined := make(map[string]bool, len(defs))
	for _, d := range defs {
		defined[d.Code] = true
	}
	out := make([]string, 0, len(codes))
	seen := make(map[string]bool, len(codes))
	for _, c := range codes {
		if strings.TrimSpace(c) == "" || seen[c] {
			continue
		}
		seen[c] = true
		switch {
		case kind == NoticeRecipientStaff && c != NoticeAttrFullName:
			return nil, errors.Join(ErrNoticeAttributeInvalid, errors.New("cán bộ chỉ có thuộc tính full_name"))
		case kind != NoticeRecipientStaff && !IsNoticeStaticAttribute(c) && !defined[c]:
			// The code is configuration, not personal data: naming it tells the caller which mapping is stale.
			return nil, errors.Join(ErrNoticeAttributeInvalid, errors.New("mã thuộc tính không có trong cấu hình của xã: "+c))
		}
		out = append(out, c)
	}
	return out, nil
}

// NoticeAttributeSource is what one recipient's row holds for the attributes: the static columns by code
// and the stored dynamic values (`citizen_contact.attributes`). Staff rows carry `full_name` only.
type NoticeAttributeSource struct {
	Static  map[string]string
	Dynamic map[string]json.RawMessage
}

// FormatNoticeAttributes renders every code in `codes` (already checked) for one recipient: one entry per
// code, "" when absent. A dynamic code is formatted by its definition in `defs` (live or retired); `loc`
// is the commune's zone for dateTime (MuiGio).
func FormatNoticeAttributes(codes []string, defs map[string]CitizenField, src NoticeAttributeSource, loc *time.Location) map[string]string {
	out := make(map[string]string, len(codes))
	for _, c := range codes {
		if IsNoticeStaticAttribute(c) {
			out[c] = cutRunes(strings.TrimSpace(src.Static[c]), MaxNoticeAttributeValue)
			continue
		}
		d, ok := defs[c]
		if !ok {
			out[c] = ""
			continue
		}
		out[c] = FormatCitizenAttributeValue(d, src.Dynamic[c], loc)
	}
	return out
}

// FormatCitizenAttributeValue renders one stored value as message text, by the field's type:
//
//	singleText, multiText   as stored, trimmed
//	singleBox               the option's LABEL (a retired option still has one)
//	combobox                the options' labels in stored order, joined ", "
//	number                  as stored (canonical decimal text, '.' separator)
//	dateTime                epoch millis in `loc`, per the field's date pattern
//
// "" for no value, an unreadable one, or an option the definition no longer knows. Cut to 500 characters.
func FormatCitizenAttributeValue(d CitizenField, raw json.RawMessage, loc *time.Location) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	switch d.Type {
	case FieldSingleText, FieldMultiText:
		if json.Unmarshal(raw, &s) != nil {
			return ""
		}
		s = strings.TrimSpace(s)
	case FieldSingleBox:
		var v string
		if json.Unmarshal(raw, &v) != nil {
			return ""
		}
		s = optionLabel(d, v)
	case FieldCombobox:
		var vs []string
		if json.Unmarshal(raw, &vs) != nil {
			return ""
		}
		labels := make([]string, 0, len(vs))
		for _, v := range vs {
			if l := optionLabel(d, v); l != "" {
				labels = append(labels, l)
			}
		}
		s = strings.Join(labels, ", ")
	case FieldNumber:
		n, ok := jsonNumber(raw)
		if !ok {
			return ""
		}
		s = n
	case FieldDateTime:
		n, ok := jsonNumber(raw)
		if !ok || !integerShape.MatchString(n) || loc == nil {
			return ""
		}
		ms, err := strconv.ParseInt(n, 10, 64)
		if err != nil {
			return ""
		}
		s = time.UnixMilli(ms).In(loc).Format(goDateLayout(d.DatePattern))
	}
	return cutRunes(s, MaxNoticeAttributeValue)
}

// goDateLayout maps the fixed display patterns (CitizenFieldDatePatterns) onto Go layouts. An unknown or
// empty pattern renders as a date — the first pattern — never as raw millis.
func goDateLayout(p string) string {
	switch p {
	case "dd/MM/yyyy HH:mm":
		return "02/01/2006 15:04"
	case "MM/yyyy":
		return "01/2006"
	case "yyyy":
		return "2006"
	}
	return "02/01/2006"
}

func optionLabel(d CitizenField, v string) string {
	for _, o := range d.Options {
		if o.Value == v {
			return o.Label
		}
	}
	return ""
}

func cutRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return strings.TrimSpace(string([]rune(s)[:n]))
}
