package domain

// The custom values of a map asset, validated against THIS commune's map_field_schema rows of the
// asset's type (migration 0007; spec §8.2, §12.6).
//
// WHAT A WRITE MAY DO, decided here once for create and edit alike:
//
//	key of an ACTIVE field of the type   written, after its value passes the field's value type
//	key whose value is unchanged         accepted as it is — a client that read the row and posts the
//	                                     whole object back must not be refused for a value a schema
//	                                     change has since retired (spec §12.6 keeps such values)
//	JSON null / blank text               removes the key
//	any other key                        REFUSED: unknown, retired or disabled field (422)
//	keys not named in the request        untouched — values of an old type or retired field STAY
//
// And after the merge every ACTIVE REQUIRED field of the type must hold a value. That is the comment
// on MapFieldSchema.IsRequired coming due: an asset filed before a field became required is refused
// on its next edit until the value is supplied.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// MapAssetCustomTextMaxLen bounds a `van-ban` value — a vendor guard, not a business rule.
const MapAssetCustomTextMaxLen = 1000

// The custom-value refusals. Each is wrapped in a *CustomValueError naming the field code (a schema
// identifier, never personal data) — never the value.
var (
	ErrCustomValueUnknownField = errors.New("trường không có trong cấu hình trường bản đồ của nhóm này")
	ErrCustomValueFieldOff     = errors.New("trường đang tắt — không nhận giá trị mới")
	ErrCustomValueWrongType    = errors.New("giá trị không đúng kiểu dữ liệu của trường")
	ErrCustomValueNotAnOption  = errors.New("giá trị không nằm trong danh sách lựa chọn của trường")
	ErrCustomValueRequired     = errors.New("trường bắt buộc chưa có giá trị")
)

// CustomValueError names the field a refusal is about.
type CustomValueError struct {
	FieldCode string
	Err       error
}

func (e *CustomValueError) Error() string {
	return "custom_values." + e.FieldCode + ": " + e.Err.Error()
}
func (e *CustomValueError) Unwrap() error { return e.Err }

// ApplyCustomValues merges patch into stored under the rules above and returns the new map (never nil).
// fields are the LIVE (not soft-deleted) map_field_schema rows of the asset's type, active and disabled.
// It does not check required fields — see CheckRequiredCustomValues — because a merge is judged on the
// whole row, after every other change of the same request.
func ApplyCustomValues(fields []MapFieldSchema, stored, patch map[string]json.RawMessage) (map[string]json.RawMessage, error) {
	out := make(map[string]json.RawMessage, len(stored)+len(patch))
	for k, v := range stored {
		out[k] = v
	}
	byCode := make(map[string]MapFieldSchema, len(fields))
	for _, f := range fields {
		byCode[f.FieldCode] = f
	}
	// Sorted, so the field named in a refusal does not depend on map order.
	keys := make([]string, 0, len(patch))
	for k := range patch {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		raw := patch[k]
		if old, ok := stored[k]; ok && sameJSON(old, raw) {
			continue
		}
		if isJSONNull(raw) {
			delete(out, k)
			continue
		}
		f, ok := byCode[k]
		if !ok {
			return nil, &CustomValueError{FieldCode: safeFieldCode(k), Err: ErrCustomValueUnknownField}
		}
		if !f.IsActive {
			return nil, &CustomValueError{FieldCode: k, Err: ErrCustomValueFieldOff}
		}
		v, err := normalizeCustomValue(f, raw)
		if err != nil {
			return nil, &CustomValueError{FieldCode: k, Err: err}
		}
		if v == nil {
			delete(out, k)
			continue
		}
		out[k] = v
	}

	b, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("map_asset: mã hoá custom_values: %w", err)
	}
	if len(b) > MapAssetCustomValuesMaxBytes {
		return nil, ErrMapAssetCustomValuesTooLarge
	}
	return out, nil
}

// CheckRequiredCustomValues refuses a row missing the value of an ACTIVE REQUIRED field of its type.
// A disabled field is not shown by the form, so it cannot be demanded by it.
func CheckRequiredCustomValues(fields []MapFieldSchema, values map[string]json.RawMessage) error {
	sorted := append([]MapFieldSchema(nil), fields...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].FieldCode < sorted[j].FieldCode })
	for _, f := range sorted {
		if !f.IsActive || !f.IsRequired {
			continue
		}
		if v, ok := values[f.FieldCode]; !ok || isJSONNull(v) {
			return &CustomValueError{FieldCode: f.FieldCode, Err: ErrCustomValueRequired}
		}
	}
	return nil
}

// normalizeCustomValue checks one value against its field and returns the canonical JSON to store, or
// nil when the value clears the key (blank text).
func normalizeCustomValue(f MapFieldSchema, raw json.RawMessage) (json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, ErrCustomValueWrongType
	}
	switch f.ValueType {
	case ValueTypeText:
		s, ok := v.(string)
		if !ok {
			return nil, ErrCustomValueWrongType
		}
		s = strings.TrimSpace(s)
		if s == "" {
			return nil, nil
		}
		if len([]rune(s)) > MapAssetCustomTextMaxLen {
			return nil, ErrCustomValueWrongType
		}
		for _, r := range s {
			if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
				return nil, ErrCustomValueWrongType
			}
		}
		return json.Marshal(s)
	case ValueTypeInteger:
		n, ok := v.(json.Number)
		if !ok {
			return nil, ErrCustomValueWrongType
		}
		i, err := strconv.ParseInt(n.String(), 10, 64)
		if err != nil {
			return nil, ErrCustomValueWrongType
		}
		return json.RawMessage(strconv.FormatInt(i, 10)), nil
	case ValueTypeDecimal:
		n, ok := v.(json.Number)
		if !ok {
			return nil, ErrCustomValueWrongType
		}
		x, err := strconv.ParseFloat(n.String(), 64)
		if err != nil || math.IsNaN(x) || math.IsInf(x, 0) {
			return nil, ErrCustomValueWrongType
		}
		return json.RawMessage(strconv.FormatFloat(x, 'f', -1, 64)), nil
	case ValueTypeBoolean:
		b, ok := v.(bool)
		if !ok {
			return nil, ErrCustomValueWrongType
		}
		return json.Marshal(b)
	case ValueTypeDate:
		s, ok := v.(string)
		if !ok {
			return nil, ErrCustomValueWrongType
		}
		if _, err := time.Parse(time.DateOnly, s); err != nil {
			return nil, ErrCustomValueWrongType
		}
		return json.Marshal(s)
	case ValueTypeChoice:
		s, ok := v.(string)
		if !ok {
			return nil, ErrCustomValueWrongType
		}
		for _, o := range f.Options {
			if o.Value == s {
				return json.Marshal(s)
			}
		}
		return nil, ErrCustomValueNotAnOption
	}
	// A seventh type the CHECK does not admit: refuse rather than store something nothing can read.
	return nil, ErrCustomValueWrongType
}

func isJSONNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func sameJSON(a, b json.RawMessage) bool {
	var ca, cb bytes.Buffer
	if json.Compact(&ca, a) != nil || json.Compact(&cb, b) != nil {
		return false
	}
	return bytes.Equal(ca.Bytes(), cb.Bytes())
}

// safeFieldCode returns k when it has the shape of a field code, a placeholder otherwise: an unknown
// key is client text and goes into an error message and a log line.
func safeFieldCode(k string) string {
	if c, err := NormalizeFieldCode(k); err == nil && c == k {
		return k
	}
	return "?"
}
