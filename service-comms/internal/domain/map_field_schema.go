package domain

// MapFieldSchema — one configured field of one asset type's form, in one commune. The entity
// declared on migrations/0007_map_field_schema.sql (docs/ui-ux/14-cau-hinh.md §6).
//
// WHAT THIS DESCRIBES DOES NOT EXIST YET. There is no asset register (`doi_tuong_ban_do`) in this
// repository, so these rows define the form of values nobody can store. Every rule below is
// written for the day that register exists — the day relaxing them would need a data migration
// on archival records. The migration header lists the four rules; this file restates them only
// where a refusal has to be made in a sentence staff can act on.
//
// THE SAME TWO-LAYER SPLIT AS three_tier_catalogue.go: the trigger `map_field_schema_guard` is the
// floor; this file refuses first, in Vietnamese. A drift between the two costs a worse error
// message, never a hole — the trigger runs last and the transaction rolls back with its audit
// entry.

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// The six value types. VALUES in Vietnamese without diacritics (ADR 0011) — they are data, stored
// in every row and read by every client. The labels the screen shows are
// Văn bản · Số nguyên · Số thập phân · Đúng/Sai · Ngày · Chọn trong danh sách
// (docs/ui-ux/10-ban-do-kinh-te-so.md:221). The CHECK constraint on the table admits no seventh.
const (
	ValueTypeText    = "van-ban"
	ValueTypeInteger = "so-nguyen"
	ValueTypeDecimal = "so-thap-phan"
	ValueTypeBoolean = "dung-sai"
	ValueTypeDate    = "ngay"
	ValueTypeChoice  = "chon"
)

// FieldOption is one entry of a `chon` field's list. Assets will store Value; Label is what the
// commune may re-word.
type FieldOption struct {
	Value string
	Label string
}

// MapFieldSchema is one row.
type MapFieldSchema struct {
	ID string

	// AssetTypeCode is the `ma` of a live row of THIS commune's loai_tai_nguyen_ban_do at create
	// time. Immutable, like FieldCode: together they are the address an asset stores a value under.
	AssetTypeCode string
	FieldCode     string

	Label string

	// ValueType is immutable after create: stored values were written as this type.
	ValueType string

	// Options is non-empty exactly when ValueType is ValueTypeChoice. Never nil on a row the store
	// read, so it marshals as [] rather than null.
	Options []FieldOption

	// IsRequired false -> true is allowed and WILL MATTER once assets exist: every asset filed
	// without the value becomes invalid on its next edit. Nothing can check that today, because
	// there is nothing to count.
	IsRequired bool
	SortOrder  int

	// IsActive false is `Tắt`: the row stays in the list and can be re-enabled. `Xoá` is a soft
	// delete — the row leaves every read path. Stored values survive both.
	IsActive bool
}

// Subject is the business code the audit trail files this row under: the address an asset will
// store a value at. Never the internal id (core/audit.Entry.Subject).
func (m MapFieldSchema) Subject() string { return m.AssetTypeCode + "/" + m.FieldCode }

// Bounds. Not business rules — the point past which a value is a mistake or an attack.
const (
	FieldCodeMaxLen        = 64
	FieldLabelMaxLen       = 255
	FieldSortOrderMax      = 9999
	FieldOptionsMax        = 200
	FieldOptionValueMaxLen = 64
)

// Input refusals — each one a 400, each naming the JSON field it is about.
var (
	ErrFieldCodeEmpty     = errors.New("truong_ban_do: thiếu `field_code`")
	ErrFieldCodeShape     = errors.New("truong_ban_do: `field_code` phải bắt đầu bằng chữ thường a-z, chỉ gồm a-z, số và dấu gạch dưới, ví dụ `legal_form`")
	ErrFieldCodeTooLong   = errors.New("truong_ban_do: `field_code` quá dài")
	ErrFieldLabelEmpty    = errors.New("truong_ban_do: thiếu `label`")
	ErrFieldLabelTooLong  = errors.New("truong_ban_do: `label` quá dài")
	ErrValueTypeUnknown   = errors.New("truong_ban_do: `value_type` phải là một trong: van-ban, so-nguyen, so-thap-phan, dung-sai, ngay, chon")
	ErrSortOrderRange     = errors.New("truong_ban_do: `sort_order` ngoài khoảng cho phép")
	ErrOptionsRequired    = errors.New("truong_ban_do: kiểu `chon` cần ít nhất một lựa chọn trong `options`")
	ErrOptionsNotAllowed  = errors.New("truong_ban_do: chỉ kiểu `chon` mới có `options`")
	ErrOptionsTooMany     = errors.New("truong_ban_do: `options` quá nhiều lựa chọn")
	ErrOptionValueEmpty   = errors.New("truong_ban_do: một lựa chọn trong `options` thiếu `value`")
	ErrOptionValueTooLong = errors.New("truong_ban_do: `value` của một lựa chọn quá dài")
	ErrOptionLabelEmpty   = errors.New("truong_ban_do: một lựa chọn trong `options` thiếu `label`")
	ErrOptionLabelTooLong = errors.New("truong_ban_do: `label` của một lựa chọn quá dài")
	ErrOptionValueDup     = errors.New("truong_ban_do: hai lựa chọn trong `options` trùng `value`")

	// Refused on PATCH whenever the body NAMES the field, even with the value it already has — a
	// client that sends it back is told it could not have changed it, rather than left to assume.
	ErrAssetTypeImmutable = errors.New("truong_ban_do: `asset_type_code` không đổi được — thêm trường mới ở nhóm khác")
	ErrFieldCodeImmutable = errors.New("truong_ban_do: `field_code` không đổi được — sửa `label`, hoặc thêm trường mới")
	ErrValueTypeImmutable = errors.New("truong_ban_do: `value_type` không đổi được — dữ liệu đã ghi theo kiểu cũ sẽ không đọc được; hãy thêm trường mới")
)

// ErrOptionRemoved — a PATCH whose `options` drops a value the row has. A rule about the ROW, not
// the request shape, so it maps to 409.
//
// REFUSED OUTRIGHT, NOT "WHEN UNUSED": assets store the option's value, and whether any asset uses
// it cannot be known — the register does not exist. The day it does, this may become a check
// against it; until then refusing is the only answer that cannot be wrong.
var ErrOptionRemoved = errors.New("truong_ban_do: không bỏ được một lựa chọn đã có — có thể đổi nhãn hoặc thêm lựa chọn mới")

// NormalizeFieldCode trims and validates a field key: `^[a-z][a-z0-9_]*$`, 1–64.
//
// NO CASE FOLDING — `Legal_Form` is refused, not lowered: the key stored must be the key the
// person typed, on a column nothing ever rewrites.
func NormalizeFieldCode(key string) (string, error) {
	key = strings.TrimSpace(key)
	switch {
	case key == "":
		return "", ErrFieldCodeEmpty
	case len(key) > FieldCodeMaxLen:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrFieldCodeTooLong, FieldCodeMaxLen)
	case key[0] < 'a' || key[0] > 'z':
		return "", ErrFieldCodeShape
	}
	for i := 1; i < len(key); i++ {
		c := key[i]
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_') {
			return "", ErrFieldCodeShape
		}
	}
	return key, nil
}

// normalizeText trims and bounds one human-readable string, refusing control characters (they
// corrupt a screen and a log line alike).
func normalizeText(s string, maxLen int, errEmpty, errTooLong error) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", errEmpty
	case len([]rune(s)) > maxLen:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", errTooLong, maxLen)
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return "", errEmpty
		}
	}
	return s, nil
}

// NormalizeFieldLabel trims and validates the label staff see, 1–255 characters.
func NormalizeFieldLabel(label string) (string, error) {
	return normalizeText(label, FieldLabelMaxLen, ErrFieldLabelEmpty, ErrFieldLabelTooLong)
}

// ValidValueType reports whether v is one of the six value types.
func ValidValueType(v string) bool {
	switch v {
	case ValueTypeText, ValueTypeInteger, ValueTypeDecimal, ValueTypeBoolean, ValueTypeDate, ValueTypeChoice:
		return true
	}
	return false
}

// ValidateFieldSortOrder bounds the display order. Negative is refused, not clamped — same
// argument as ValidateSortOrder.
func ValidateFieldSortOrder(n int) error {
	if n < 0 || n > FieldSortOrderMax {
		return fmt.Errorf("%w (0..%d)", ErrSortOrderRange, FieldSortOrderMax)
	}
	return nil
}

// NormalizeOptions validates the option list against the value type and returns it trimmed.
//
// `chon` needs at least one option with unique values; every other type takes none. The result is
// never nil, so the store writes `[]` and the CHECK constraint sees an array.
func NormalizeOptions(valueType string, opts []FieldOption) ([]FieldOption, error) {
	if valueType != ValueTypeChoice {
		if len(opts) > 0 {
			return nil, ErrOptionsNotAllowed
		}
		return []FieldOption{}, nil
	}
	if len(opts) == 0 {
		return nil, ErrOptionsRequired
	}
	if len(opts) > FieldOptionsMax {
		return nil, fmt.Errorf("%w (tối đa %d)", ErrOptionsTooMany, FieldOptionsMax)
	}
	out := make([]FieldOption, 0, len(opts))
	seen := make(map[string]struct{}, len(opts))
	for _, o := range opts {
		v, err := normalizeText(o.Value, FieldOptionValueMaxLen, ErrOptionValueEmpty, ErrOptionValueTooLong)
		if err != nil {
			return nil, err
		}
		l, err := normalizeText(o.Label, FieldLabelMaxLen, ErrOptionLabelEmpty, ErrOptionLabelTooLong)
		if err != nil {
			return nil, err
		}
		if _, dup := seen[v]; dup {
			return nil, ErrOptionValueDup
		}
		seen[v] = struct{}{}
		out = append(out, FieldOption{Value: v, Label: l})
	}
	return out, nil
}

// CheckOptionsKept refuses an edit whose new list drops any value the old list had. Relabelling
// (same value, new label) and appending are the only edits a `chon` list admits.
func CheckOptionsKept(before, after []FieldOption) error {
	kept := make(map[string]struct{}, len(after))
	for _, o := range after {
		kept[o.Value] = struct{}{}
	}
	for _, o := range before {
		if _, ok := kept[o.Value]; !ok {
			return ErrOptionRemoved
		}
	}
	return nil
}

// SameOptions reports whether two lists are identical, order included. Order is part of what the
// form shows, so a reorder is a change and is audited.
func SameOptions(a, b []FieldOption) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
