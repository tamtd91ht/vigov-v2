package domain

// ATTRIBUTE MAPPING of a notice campaign (migration 0034; ADR 0101 #12, owner 17/10/2026): a campaign to a
// DIRECTORY audience may fill a template parameter from each recipient's own attribute. The mapping is
// {param key → attribute code}; param keys are what `params` is keyed by (ZNS parameter name, SMS slot
// position). Values are NEVER stored: identity formats them per batch, the sender renders in memory.
//
// Standard library only (this package's rule). An attribute VALUE is personal data: nothing here logs, and no
// refusal carries one.

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// MaxNoticeAttributeMappings — 0034 notice_campaign_attribute_mapping_shape, and identity's ceiling on
	// attribute_codes in one reveal.
	MaxNoticeAttributeMappings = 20
	// NoticeAttrFullName — the one attribute a staff audience may map (identity.proto ListNoticeAttributes).
	NoticeAttrFullName = "full_name"
)

// noticeAttributeCode is 0034's value shape: lower-case, digits, underscore — every static name and every
// dynamic code (`[a-z][a-z0-9]{5,7}`) fits.
var noticeAttributeCode = regexp.MustCompile(`^[a-z_][a-z0-9_]{1,31}$`)

var (
	// ErrNoticeAttributeMapping — the mapping is malformed, names a key the template does not have, or was sent
	// for typed / imported numbers (which map file columns instead — recipient_mapping).
	ErrNoticeAttributeMapping = fmt.Errorf("%w: attribute_mapping: directory audiences only, at most %d entries, "+
		"keys are the template's parameters, values attribute codes", ErrNoticeInvalid, MaxNoticeAttributeMappings)
	// ErrNoticeAttributeStaff — a staff audience mapped an attribute other than full_name.
	ErrNoticeAttributeStaff = fmt.Errorf("%w: attribute_mapping: a staff audience maps full_name only", ErrNoticeInvalid)
	// ErrNoticeAttributeUnknown — a code the commune does not offer (unknown, or a retired dynamic field).
	ErrNoticeAttributeUnknown = fmt.Errorf("%w: attribute_mapping: unknown or retired attribute", ErrNoticeInvalid)
)

// NormalizeNoticeAttributeMapping checks a mapping's shape against the audience: nil/empty answers nil; typed
// audiences refuse any entry; ≤ 20 entries; keys printable ≤ 64; values code-shaped; a staff audience maps
// full_name only. Whether a key is one of the template's and a code one the commune offers is checked by the
// caller, which has both.
func NormalizeNoticeAttributeMapping(m map[string]string, a NoticeAudience) (map[string]string, error) {
	if len(m) == 0 {
		return nil, nil
	}
	if a.Typed() || len(m) > MaxNoticeAttributeMappings {
		return nil, ErrNoticeAttributeMapping
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		if k == "" || !utf8.ValidString(k) || utf8.RuneCountInString(k) > maxNoticeParamKeyRunes || hasControl(k) ||
			!noticeAttributeCode.MatchString(v) {
			return nil, ErrNoticeAttributeMapping
		}
		if a.RecipientKind() == NoticeRecipientStaff && v != NoticeAttrFullName {
			return nil, ErrNoticeAttributeStaff
		}
		out[k] = v
	}
	return out, nil
}

// NoticeAttributeCodes is the mapping's distinct codes, sorted — the attribute_codes of a reveal.
func NoticeAttributeCodes(m map[string]string) []string {
	seen := make(map[string]bool, len(m))
	out := make([]string, 0, len(m))
	for _, v := range m {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}

// FitAttributeValue makes one formatted attribute value safe for a template slot: control characters and
// line breaks become spaces, whitespace collapses, braces are dropped (an SMS slot must not look like a
// slot), and the result is CUT to max characters (max ≤ 0: MaxNoticeParamValueRune). truncated reports a cut —
// the eSMS `{P1,20}` and the ZBS max_length are hard limits a provider refuses past, so a long address is
// shortened rather than the message lost; the batch's audit entry names the rows that were.
func FitAttributeValue(v string, max int) (string, bool) {
	if max <= 0 || max > MaxNoticeParamValueRune {
		max = MaxNoticeParamValueRune
	}
	v = strings.Map(func(r rune) rune {
		switch {
		case r == '{' || r == '}':
			return -1
		case unicode.IsControl(r):
			return ' '
		}
		return r
	}, v)
	v = strings.Join(strings.Fields(v), " ")
	if utf8.RuneCountInString(v) <= max {
		return v, false
	}
	return strings.TrimSpace(string([]rune(v)[:max])), true
}

// mappedSlotLabel is how the SMS preview shows a mapped slot: the attribute's label in brackets — no value is
// revealed at preview (ADR 0101 #12: the preview discloses nobody's data).
func mappedSlotLabel(label string) string { return "[" + label + "]" }

// PreviewSmsContentMapped is PreviewSmsContent with mapped slots shown as "[<label>]" (labels by attribute
// code). A mapped slot is never missing and is not length-checked here — its value is cut to the slot at send.
func PreviewSmsContentMapped(content string, params, mapping, labels map[string]string) (string, []string, error) {
	slots, err := ParseSmsTemplateParams(content)
	if err != nil {
		return "", nil, err
	}
	missing := []string{}
	values := make([]string, len(slots))
	for i, s := range slots {
		key := SmsSlotKey(i)
		if code, ok := mapping[key]; ok {
			l := labels[code]
			if l == "" {
				l = code
			}
			values[i] = mappedSlotLabel(l)
			continue
		}
		v, ok := params[key]
		if !ok {
			missing = append(missing, key)
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

// RenderSmsRow fills an SMS template for ONE recipient: fixed slots from params, mapped slots from the
// recipient's attribute values (by code), each fitted to its slot. A mapped slot whose value is empty is left
// empty — the provider decides on that row (eSMS answers "sai mẫu" with its message), as an empty file cell
// does for imported numbers. A fixed slot must hold a value (the send refused a missing one). The result is
// checked to hold no raw slot.
func RenderSmsRow(content string, params, mapping, attrs map[string]string) (string, bool, error) {
	slots, err := ParseSmsTemplateParams(content)
	if err != nil {
		return "", false, err
	}
	values := make([]string, len(slots))
	truncated := false
	for i, s := range slots {
		key := SmsSlotKey(i)
		if code, ok := mapping[key]; ok {
			v, cut := FitAttributeValue(attrs[code], s.MaxLen)
			values[i], truncated = v, truncated || cut
			continue
		}
		v := strings.TrimSpace(params[key])
		switch {
		case v == "":
			return "", false, ErrSmsTemplateValueEmpty
		case !utf8.ValidString(v) || hasControl(v) || strings.ContainsAny(v, "{}"):
			return "", false, ErrSmsTemplateValueShape
		case utf8.RuneCountInString(v) > s.MaxLen:
			return "", false, ErrSmsTemplateValueLong
		}
		values[i] = v
	}
	i := 0
	out := smsPlaceholder.ReplaceAllStringFunc(content, func(string) string {
		v := values[i]
		i++
		return v
	})
	if smsPlaceholder.MatchString(out) {
		return "", false, ErrSmsTemplateValueShape
	}
	return out, truncated, nil
}

// ZnsRowParams merges the campaign's fixed parameters with ONE recipient's mapped attribute values, each
// fitted to the parameter's declared max_length. A mapped parameter whose value is empty is left OUT — Zalo
// refuses that row with its own message (S5), as for an empty file cell. Only declared parameters cross.
func ZnsRowParams(declared []TemplateParam, params, mapping, attrs map[string]string) (map[string]string, bool) {
	out := make(map[string]string, len(declared))
	truncated := false
	for _, p := range declared {
		if code, ok := mapping[p.Name]; ok {
			v, cut := FitAttributeValue(attrs[code], p.MaxLength)
			truncated = truncated || cut
			if v != "" {
				out[p.Name] = v
			}
			continue
		}
		if v, ok := params[p.Name]; ok {
			out[p.Name] = v
		}
	}
	return out, truncated
}
