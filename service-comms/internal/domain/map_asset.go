package domain

// MAP ASSET — one place on ONE commune's economic map (migrations/0015_map_asset.sql; spec
// docs/ui-ux/10-ban-do-kinh-te-so.md §8, §10, §12; ADR 0072). The table, its CHECKs and every vendor
// choice are argued in the migration; the checks below are the SAME bounds, applied first so a refusal
// is a sentence rather than a constraint name (the two-layer split of danh_muc_ba_tang.go).
//
// PERSONAL DATA (rule 3, 0015 §PERSONAL DATA): `representative`, `phone`, `address`, `tax_code` (possibly
// a citizen ID number) and the coordinates of a household business. No error below ever quotes a value.

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode"
)

// The three statuses — VALUES in Vietnamese without diacritics (ADR 0011), the CHECK
// `map_asset_status_known` admits no fourth (spec §10).
const (
	MapAssetStatusActive    = "dang-hoat-dong"
	MapAssetStatusSuspended = "tam-ngung"
	MapAssetStatusDissolved = "da-giai-the"
)

// ValidMapAssetStatus reports whether s is one of the three statuses.
func ValidMapAssetStatus(s string) bool {
	switch s {
	case MapAssetStatusActive, MapAssetStatusSuspended, MapAssetStatusDissolved:
		return true
	}
	return false
}

// MapAsset is one row of `map_asset` as the service reads it. "" is NULL for every optional text;
// EmployeeCount nil is NULL; EstablishedOn is "YYYY-MM-DD" or "".
type MapAsset struct {
	ID                string
	AssetTypeCode     string
	Name              string
	Address           string
	ResidentialUnitID string
	Lat, Lng          float64
	Representative    string
	Phone             string
	Status            string

	Verified   bool
	VerifiedAt *time.Time
	VerifiedBy string // business code `CB-…` (rule 6, invariant 8)

	TaxCode       string
	IndustryCode  string
	EmployeeCount *int
	EstablishedOn string
	Description   string

	// CustomValues is {"<map_field_schema.field_code>": <JSON value>}. Never nil on a row the store read.
	// Values under a retired or disabled field STAY (spec §12.6); the schema decides what the form
	// shows, never what the row keeps.
	CustomValues map[string]json.RawMessage

	CreatedAt time.Time
	UpdatedAt time.Time
}

// MapAssetPoint is the light shape the map draws — no personal field (rule 3).
type MapAssetPoint struct {
	ID            string
	AssetTypeCode string
	Name          string
	Status        string
	Verified      bool
	Lat, Lng      float64
}

// MapAssetFilter is the filter shared by the map points and the `Sổ địa điểm` list. Zero values mean
// "no condition".
type MapAssetFilter struct {
	AssetTypeCodes    []string
	Status            string
	Verified          *bool
	IndustryCode      string
	ResidentialUnitID string
	Query             string // matched against name and address, ILIKE inside the commune
}

// MapAssetTypeCount is the live count of one group.
type MapAssetTypeCount struct {
	AssetTypeCode string
	Count         int
	Verified      int
}

// MapAssetSummary is the header meta line (spec §1 "26 đối tượng · 42.3% đã xác minh") and the layer
// counts (§4.1). Live rows only.
type MapAssetSummary struct {
	ByType   []MapAssetTypeCount
	Total    int
	Verified int
}

// VerifiedRatio is count(verified) / count(*) (spec §12.3), 0 for an empty register, rounded to four
// decimals so "42.3%" is renderable and the wire value is stable.
func (s MapAssetSummary) VerifiedRatio() float64 {
	if s.Total == 0 {
		return 0
	}
	return math.Round(float64(s.Verified)/float64(s.Total)*10000) / 10000
}

// Bounds — the CHECKs of migration 0015, never retyped elsewhere.
const (
	MapAssetNameMaxLen           = 255
	MapAssetAddressMaxLen        = 500
	MapAssetRepresentativeMaxLen = 255
	MapAssetDescriptionMaxLen    = 4000
	MapAssetTaxCodeMaxLen        = 20
	MapAssetCustomValuesMaxBytes = 65536

	// MapAssetCoordinateDecimals is numeric(10,6) — about 0.1 m (ADR 0072 §5).
	MapAssetCoordinateDecimals = 6

	// Not in the migration — vendor guards against a bogus value, in the same spirit as its bounds.
	MapAssetResidentialUnitIDMaxLen = 64
	MapAssetEmployeeCountMax        = math.MaxInt32
	MapAssetFilterQueryMaxLen       = 100
	MapAssetFilterTypesMax          = 20
)

// Input refusals — each one a 400 naming the JSON field it is about, never its value.
var (
	ErrMapAssetTypeEmpty              = errors.New("map_asset: thiếu `asset_type_code`")
	ErrMapAssetNameEmpty              = errors.New("map_asset: thiếu `name`")
	ErrMapAssetNameTooLong            = errors.New("map_asset: `name` quá dài")
	ErrMapAssetAddressInvalid         = errors.New("map_asset: `address` chứa ký tự không hợp lệ")
	ErrMapAssetAddressTooLong         = errors.New("map_asset: `address` quá dài")
	ErrMapAssetResidentialUnitInvalid = errors.New("map_asset: `residential_unit_id` không đúng dạng")
	ErrMapAssetLocationMissing        = errors.New("map_asset: thiếu `lat` hoặc `lng` — vị trí là bắt buộc")
	ErrMapAssetLocationOutOfRange     = errors.New("map_asset: toạ độ ngoài phạm vi — `lat` từ -90 đến 90, `lng` từ -180 đến 180")
	ErrMapAssetRepresentativeInvalid  = errors.New("map_asset: `representative` chứa ký tự không hợp lệ")
	ErrMapAssetRepresentativeTooLong  = errors.New("map_asset: `representative` quá dài")
	ErrMapAssetPhoneShape             = errors.New("map_asset: `phone` không đúng dạng")
	ErrMapAssetStatusUnknown          = errors.New("map_asset: `status` phải là dang-hoat-dong, tam-ngung hoặc da-giai-the")
	ErrMapAssetTaxCodeShape           = errors.New("map_asset: `tax_code` chỉ gồm chữ số, có thể một dấu gạch nối, tối đa 20 ký tự")
	ErrMapAssetIndustryCodeShape      = errors.New("map_asset: `industry_code` là mã ngành VSIC cấp 2, đúng hai chữ số")
	ErrMapAssetEmployeeCountInvalid   = errors.New("map_asset: `employee_count` phải là số nguyên không âm")
	ErrMapAssetEstablishedOnInvalid   = errors.New("map_asset: `established_on` phải là ngày dạng YYYY-MM-DD")
	ErrMapAssetDescriptionInvalid     = errors.New("map_asset: `description` chứa ký tự không hợp lệ")
	ErrMapAssetDescriptionTooLong     = errors.New("map_asset: `description` quá dài")
	ErrMapAssetCustomValuesTooLarge   = errors.New("map_asset: `custom_values` quá lớn")
	ErrMapAssetFilterInvalid          = errors.New("map_asset: bộ lọc không hợp lệ")
)

// normalizeOptionalText trims an OPTIONAL text: blank is "" (stored NULL — 0015 refuses an all-blank
// value as a second spelling of NULL), otherwise bounded with no control character. allowNewline
// keeps line breaks for the one multi-line field (description).
func normalizeOptionalText(s string, maxLen int, allowNewline bool, errInvalid, errTooLong error) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	if len([]rune(s)) > maxLen {
		return "", fmt.Errorf("%w (tối đa %d ký tự)", errTooLong, maxLen)
	}
	for _, r := range s {
		if unicode.IsControl(r) && !(allowNewline && (r == '\n' || r == '\r' || r == '\t')) {
			return "", errInvalid
		}
	}
	return s, nil
}

// NormalizeMapAssetLocation rounds a REQUIRED pin to the column's six decimals (spec §12.1: no
// coordinate, no map). The world's range and not Vietnam's — petitions' NormaliseSceneLocation argues
// why a national box would be a business rule nobody stated. Rounded HERE so the value returned is
// byte-equal to the value stored.
func NormalizeMapAssetLocation(lat, lng *float64) (float64, float64, error) {
	if lat == nil || lng == nil {
		return 0, 0, ErrMapAssetLocationMissing
	}
	if !coordinateWithin(*lat, 90) || !coordinateWithin(*lng, 180) {
		return 0, 0, ErrMapAssetLocationOutOfRange
	}
	return roundCoordinate(*lat), roundCoordinate(*lng), nil
}

func coordinateWithin(v, bound float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= -bound && v <= bound
}

// roundCoordinate folds -0 to 0 so the wire never carries "-0".
func roundCoordinate(v float64) float64 {
	p := math.Pow10(MapAssetCoordinateDecimals)
	r := math.Round(v*p) / p
	if r == 0 {
		return 0
	}
	return r
}

// NormalizeMapAssetPhone checks an OPTIONAL dial string with external_contacts' shape (0015 reuses
// 0014's CHECK): the screen turns it into a `tel:` link and a letter breaks the link silently.
func NormalizeMapAssetPhone(s string) (string, error) {
	if strings.TrimSpace(s) == "" {
		return "", nil
	}
	p, err := NormalizeExternalContactPhone(s)
	if err != nil {
		return "", ErrMapAssetPhoneShape
	}
	return p, nil
}

// NormalizeMapAssetTaxCode checks an OPTIONAL tax code: digits with at most one `-` group, ≤ 20
// characters (0015 — deliberately loose: a household may be filed under its owner's 12-digit ID).
func NormalizeMapAssetTaxCode(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	if len(s) > MapAssetTaxCodeMaxLen {
		return "", ErrMapAssetTaxCodeShape
	}
	dash := -1
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c >= '0' && c <= '9':
		case c == '-' && dash < 0:
			dash = i
		default:
			return "", ErrMapAssetTaxCodeShape
		}
	}
	if dash == 0 || dash == len(s)-1 {
		return "", ErrMapAssetTaxCodeShape
	}
	return s, nil
}

// NormalizeMapAssetIndustryCode checks an OPTIONAL VSIC level-2 code: exactly two digits (0015's
// CHECK). Membership in the spec's 20-code list (§5) is NOT checked: that list is a picker for the
// screen, VSIC level 2 has more codes, and narrowing it is a business rule nobody stated.
func NormalizeMapAssetIndustryCode(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	if len(s) != 2 || s[0] < '0' || s[0] > '9' || s[1] < '0' || s[1] > '9' {
		return "", ErrMapAssetIndustryCodeShape
	}
	return s, nil
}

// NormalizeMapAssetDate checks an OPTIONAL calendar date.
func NormalizeMapAssetDate(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	if _, err := time.Parse(time.DateOnly, s); err != nil {
		return "", ErrMapAssetEstablishedOnInvalid
	}
	return s, nil
}

// NormalizeResidentialUnitID checks an OPTIONAL identity id held as a VALUE (no foreign key: another
// service's row, rule 2). Its EXISTENCE is not checked here — that is a read of identity's data. Only
// the shape: an opaque id, ASCII letters, digits, `-` and `_`.
func NormalizeResidentialUnitID(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	if len(s) > MapAssetResidentialUnitIDMaxLen {
		return "", ErrMapAssetResidentialUnitInvalid
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return "", ErrMapAssetResidentialUnitInvalid
		}
	}
	return s, nil
}

// NormalizeMapAsset applies every shape check of migration 0015 to a whole row — the custom values
// excepted, which depend on the commune's configured schema (custom_value.go). The type code is only
// checked for presence and slug shape; whether it is in THIS commune's catalogue is the store's
// question.
func NormalizeMapAsset(a MapAsset) (MapAsset, error) {
	var err error
	if strings.TrimSpace(a.AssetTypeCode) == "" {
		return MapAsset{}, ErrMapAssetTypeEmpty
	}
	if a.AssetTypeCode, err = ChuanHoaMa(a.AssetTypeCode); err != nil {
		return MapAsset{}, ErrMapAssetTypeEmpty
	}
	if a.Name, err = normalizeText(a.Name, MapAssetNameMaxLen, ErrMapAssetNameEmpty, ErrMapAssetNameTooLong); err != nil {
		return MapAsset{}, err
	}
	if a.Address, err = normalizeOptionalText(a.Address, MapAssetAddressMaxLen, false,
		ErrMapAssetAddressInvalid, ErrMapAssetAddressTooLong); err != nil {
		return MapAsset{}, err
	}
	if a.ResidentialUnitID, err = NormalizeResidentialUnitID(a.ResidentialUnitID); err != nil {
		return MapAsset{}, err
	}
	if a.Lat, a.Lng, err = NormalizeMapAssetLocation(&a.Lat, &a.Lng); err != nil {
		return MapAsset{}, err
	}
	if a.Representative, err = normalizeOptionalText(a.Representative, MapAssetRepresentativeMaxLen, false,
		ErrMapAssetRepresentativeInvalid, ErrMapAssetRepresentativeTooLong); err != nil {
		return MapAsset{}, err
	}
	if a.Phone, err = NormalizeMapAssetPhone(a.Phone); err != nil {
		return MapAsset{}, err
	}
	if !ValidMapAssetStatus(a.Status) {
		return MapAsset{}, ErrMapAssetStatusUnknown
	}
	if a.TaxCode, err = NormalizeMapAssetTaxCode(a.TaxCode); err != nil {
		return MapAsset{}, err
	}
	if a.IndustryCode, err = NormalizeMapAssetIndustryCode(a.IndustryCode); err != nil {
		return MapAsset{}, err
	}
	if a.EmployeeCount != nil && (*a.EmployeeCount < 0 || *a.EmployeeCount > MapAssetEmployeeCountMax) {
		return MapAsset{}, ErrMapAssetEmployeeCountInvalid
	}
	if a.EstablishedOn, err = NormalizeMapAssetDate(a.EstablishedOn); err != nil {
		return MapAsset{}, err
	}
	if a.Description, err = normalizeOptionalText(a.Description, MapAssetDescriptionMaxLen, true,
		ErrMapAssetDescriptionInvalid, ErrMapAssetDescriptionTooLong); err != nil {
		return MapAsset{}, err
	}
	return a, nil
}

// NormalizeMapAssetFilter validates a filter BEFORE any statement runs. Every refusal is the one
// sentinel ErrMapAssetFilterInvalid — the reason never quotes the input (`q` may be a person's name).
func NormalizeMapAssetFilter(f MapAssetFilter) (MapAssetFilter, error) {
	out := MapAssetFilter{Verified: f.Verified}
	seen := map[string]bool{}
	for _, c := range f.AssetTypeCodes {
		for _, part := range strings.Split(c, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			code, err := ChuanHoaMa(part)
			if err != nil {
				return MapAssetFilter{}, fmt.Errorf("%w: asset_type_code", ErrMapAssetFilterInvalid)
			}
			if !seen[code] {
				seen[code] = true
				out.AssetTypeCodes = append(out.AssetTypeCodes, code)
			}
		}
	}
	if len(out.AssetTypeCodes) > MapAssetFilterTypesMax {
		return MapAssetFilter{}, fmt.Errorf("%w: asset_type_code quá nhiều", ErrMapAssetFilterInvalid)
	}
	if s := strings.TrimSpace(f.Status); s != "" {
		if !ValidMapAssetStatus(s) {
			return MapAssetFilter{}, fmt.Errorf("%w: status", ErrMapAssetFilterInvalid)
		}
		out.Status = s
	}
	var err error
	if out.IndustryCode, err = NormalizeMapAssetIndustryCode(f.IndustryCode); err != nil {
		return MapAssetFilter{}, fmt.Errorf("%w: industry_code", ErrMapAssetFilterInvalid)
	}
	if out.ResidentialUnitID, err = NormalizeResidentialUnitID(f.ResidentialUnitID); err != nil {
		return MapAssetFilter{}, fmt.Errorf("%w: residential_unit_id", ErrMapAssetFilterInvalid)
	}
	if out.Query, err = normalizeOptionalText(f.Query, MapAssetFilterQueryMaxLen, false,
		ErrMapAssetFilterInvalid, ErrMapAssetFilterInvalid); err != nil {
		return MapAssetFilter{}, fmt.Errorf("%w: q", ErrMapAssetFilterInvalid)
	}
	return out, nil
}
