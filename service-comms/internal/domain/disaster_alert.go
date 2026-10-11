package domain

// DISASTER ALERTS — a warning ONE commune posts for its residents (migration 0026; owner decision
// 11/10/2026, ADR 0091 §2): storm, flood, landslide, heavy rain, epidemic, fire. Shown on the Mini App
// with the danger zone, the evacuation points and the numbers to call. Nothing is pushed by ZNS.
//
// The table, its CHECKs and every vendor choice are argued in migrations/0026_disaster_alerts.sql. The
// checks below are the SAME bounds, applied first so a refusal is a sentence; the ELEMENT SHAPE of the
// three JSONB lists is checked HERE ONLY — the database holds the array floor and the length caps.
//
// PERSONAL DATA (rule 3, 0026 PERSONAL DATA): `emergency_contacts[].name/phone` and
// `evacuation_points[].phone` may hold an officer's own mobile. Never logged, never in an error, masked in
// the audit delta (app/disaster_alert.go). Shown unmasked to residents — being shown is the purpose.

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
)

// DisasterKind is one of six VALUES (ADR 0011: Vietnamese without diacritics, kebab-case) — 0026's
// `disaster_alerts_kind_valid`. The Go names are English (rule 12); the values are not translated.
type DisasterKind string

const (
	DisasterKindStorm     DisasterKind = "bao"
	DisasterKindFlood     DisasterKind = "lu"
	DisasterKindLandslide DisasterKind = "sat-lo"
	DisasterKindHeavyRain DisasterKind = "mua-lon"
	DisasterKindEpidemic  DisasterKind = "dich-benh"
	DisasterKindFire      DisasterKind = "chay"
)

// ValidDisasterKind reports whether s is one of the six.
func ValidDisasterKind(s string) bool {
	switch DisasterKind(s) {
	case DisasterKindStorm, DisasterKindFlood, DisasterKindLandslide, DisasterKindHeavyRain,
		DisasterKindEpidemic, DisasterKindFire:
		return true
	}
	return false
}

// DangerZonePoint is one vertex of the danger zone, RELATIVE to the picture the Mini App draws: x and y
// in 0..100. Not latitude/longitude — no resident's location is stored or asked for (ADR 0091 §2).
type DangerZonePoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// EvacuationPoint is one place residents go. Capacity nil = not stated.
type EvacuationPoint struct {
	Name     string `json:"name"`
	Address  string `json:"address,omitempty"`
	Phone    string `json:"phone,omitempty"`
	Capacity *int   `json:"capacity,omitempty"`
}

// EmergencyContact is one number to call, with the role it answers for ("Trực ban PCTT xã").
type EmergencyContact struct {
	Role  string `json:"role"`
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

// DisasterAlert is one row of `disaster_alerts` as the service reads it. "" / the zero time mean NULL.
type DisasterAlert struct {
	ID       string
	Kind     DisasterKind
	Title    string
	Level    string // NULL = none stated
	Headline string // NULL = the read path shows Title
	Detail   string

	EffectiveFrom time.Time
	EffectiveTo   time.Time // zero = until ended or replaced
	EndedAt       time.Time // zero = not ended early
	EndedBy       string

	DangerZone        []DangerZonePoint
	EvacuationPoints  []EvacuationPoint
	EmergencyContacts []EmergencyContact

	CreatedAt time.Time
	CreatedBy string // staff BUSINESS code (rule 6, invariant 8)
	UpdatedAt time.Time
	UpdatedBy string
}

// InForce reports whether residents are shown the alert at `now`: not ended, started, and not past its
// end — the window [effective_from, effective_to) of 0026. Soft-deleted rows never reach here.
func (a DisasterAlert) InForce(now time.Time) bool {
	if !a.EndedAt.IsZero() || now.Before(a.EffectiveFrom) {
		return false
	}
	return a.EffectiveTo.IsZero() || now.Before(a.EffectiveTo)
}

// Ended reports whether staff ended the alert early — from then on it is a historical record (0026's
// guard refuses every edit but the soft delete).
func (a DisasterAlert) Ended() bool { return !a.EndedAt.IsZero() }

// Bounds — the CHECKs and caps of migration 0026, never retyped elsewhere. The element bounds (name,
// address, role, phone) are the vendor's, set equal to external_contacts' (0014) for the same kind of
// value.
const (
	DisasterTitleMaxLen       = 255
	DisasterHeadlineMaxLen    = 255
	DisasterLevelMaxLen       = 100
	DisasterDetailMaxLen      = 10000
	DisasterDangerZoneMax     = 200
	DisasterDangerZoneMinPts  = 3
	DisasterEvacuationMax     = 50
	DisasterContactsMax       = 50
	DisasterCoordMin          = 0
	DisasterCoordMax          = 100
	DisasterPlaceNameMaxLen   = ExternalContactNameMaxLen
	DisasterAddressMaxLen     = ExternalContactAddressMaxLen
	DisasterContactRoleMaxLen = 100
	DisasterContactNameMaxLen = ExternalContactNameMaxLen
)

var (
	ErrDisasterKindInvalid      = errors.New("disaster_alert: `kind` không thuộc sáu loại")
	ErrDisasterTitleEmpty       = errors.New("disaster_alert: thiếu `title`")
	ErrDisasterTitleTooLong     = errors.New("disaster_alert: `title` quá dài")
	ErrDisasterLevelInvalid     = errors.New("disaster_alert: `level` quá dài hoặc chứa ký tự không hợp lệ")
	ErrDisasterHeadlineInvalid  = errors.New("disaster_alert: `headline` quá dài hoặc chứa ký tự không hợp lệ")
	ErrDisasterDetailEmpty      = errors.New("disaster_alert: thiếu `detail`")
	ErrDisasterDetailInvalid    = errors.New("disaster_alert: `detail` quá dài hoặc chứa ký tự không hợp lệ")
	ErrDisasterFromMissing      = errors.New("disaster_alert: thiếu `effective_from`")
	ErrDisasterWindowInverted   = errors.New("disaster_alert: `effective_to` phải sau `effective_from`")
	ErrDisasterDangerZone       = errors.New("disaster_alert: `danger_zone` không hợp lệ")
	ErrDisasterEvacuationPoints = errors.New("disaster_alert: `evacuation_points` không hợp lệ")
	ErrDisasterContacts         = errors.New("disaster_alert: `emergency_contacts` không hợp lệ")
	ErrDisasterEnded            = errors.New("disaster_alert: cảnh báo đã kết thúc — không sửa được nữa")
)

// NormalizeDisasterAlert applies every shape check of 0026 (and the element shapes it leaves to Go) to a
// whole row, returning the cleaned row. Nil lists become empty ones: the columns are NOT NULL arrays.
//
// THE WINDOW IS STRICT — effective_to AFTER effective_from — because 0026's
// `disaster_alerts_effective_window` is `effective_to > effective_from`; an equal pair would pass here and
// fail at the database as a 500.
func NormalizeDisasterAlert(a DisasterAlert) (DisasterAlert, error) {
	if !ValidDisasterKind(string(a.Kind)) {
		return DisasterAlert{}, ErrDisasterKindInvalid
	}
	var err error
	if a.Title, err = normalizeText(a.Title, DisasterTitleMaxLen, ErrDisasterTitleEmpty, ErrDisasterTitleTooLong); err != nil {
		return DisasterAlert{}, err
	}
	if a.Level, err = optionalLine(a.Level, DisasterLevelMaxLen, ErrDisasterLevelInvalid); err != nil {
		return DisasterAlert{}, err
	}
	if a.Headline, err = optionalLine(a.Headline, DisasterHeadlineMaxLen, ErrDisasterHeadlineInvalid); err != nil {
		return DisasterAlert{}, err
	}
	if a.Detail, err = normalizeDetail(a.Detail); err != nil {
		return DisasterAlert{}, err
	}
	if a.EffectiveFrom.IsZero() {
		return DisasterAlert{}, ErrDisasterFromMissing
	}
	a.EffectiveFrom = a.EffectiveFrom.UTC()
	if !a.EffectiveTo.IsZero() {
		a.EffectiveTo = a.EffectiveTo.UTC()
		if !a.EffectiveTo.After(a.EffectiveFrom) {
			return DisasterAlert{}, ErrDisasterWindowInverted
		}
	}
	if a.DangerZone, err = checkDangerZone(a.DangerZone); err != nil {
		return DisasterAlert{}, err
	}
	if a.EvacuationPoints, err = normalizeEvacuationPoints(a.EvacuationPoints); err != nil {
		return DisasterAlert{}, err
	}
	if a.EmergencyContacts, err = normalizeEmergencyContacts(a.EmergencyContacts); err != nil {
		return DisasterAlert{}, err
	}
	return a, nil
}

// optionalLine is a nullable one-line text: blank is "" (NULL — 0026 refuses an all-blank value as a
// second spelling of NULL), otherwise bounded and free of control characters.
func optionalLine(s string, max int, errInvalid error) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	if len([]rune(s)) > max {
		return "", fmt.Errorf("%w (tối đa %d ký tự)", errInvalid, max)
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return "", errInvalid
		}
	}
	return s, nil
}

// normalizeDetail is the multi-line body: required, at most 10000 characters; newlines and tabs are
// allowed, any other control character is refused (the database allows them; a resident's screen does
// not render them).
func normalizeDetail(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ErrDisasterDetailEmpty
	}
	if len([]rune(s)) > DisasterDetailMaxLen {
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrDisasterDetailInvalid, DisasterDetailMaxLen)
	}
	for _, r := range s {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return "", ErrDisasterDetailInvalid
		}
	}
	return s, nil
}

// checkDangerZone: at most 200 points, each coordinate in 0..100, and a polygon of at least three
// points when there is one at all (two points draw a line, not an area).
func checkDangerZone(pts []DangerZonePoint) ([]DangerZonePoint, error) {
	if len(pts) == 0 {
		return []DangerZonePoint{}, nil
	}
	if len(pts) < DisasterDangerZoneMinPts {
		return nil, fmt.Errorf("%w: một vùng cần ít nhất %d điểm", ErrDisasterDangerZone, DisasterDangerZoneMinPts)
	}
	if len(pts) > DisasterDangerZoneMax {
		return nil, fmt.Errorf("%w: tối đa %d điểm", ErrDisasterDangerZone, DisasterDangerZoneMax)
	}
	out := make([]DangerZonePoint, len(pts))
	for i, p := range pts {
		// The comparisons are written so NaN fails them too (NaN compares false with everything).
		if !(p.X >= DisasterCoordMin && p.X <= DisasterCoordMax && p.Y >= DisasterCoordMin && p.Y <= DisasterCoordMax) {
			return nil, fmt.Errorf("%w: điểm thứ %d phải có x, y từ %d đến %d", ErrDisasterDangerZone, i+1,
				DisasterCoordMin, DisasterCoordMax)
		}
		out[i] = p
	}
	return out, nil
}

// normalizeEvacuationPoints: at most 50; each needs a name; address and phone optional (the phone, when
// given, is a dial string — NormalizeExternalContactPhone's rule, since the Mini App makes it a `tel:`
// link); capacity, when given, is ≥ 0. The sentence names the item's position, never its value.
func normalizeEvacuationPoints(pts []EvacuationPoint) ([]EvacuationPoint, error) {
	if len(pts) > DisasterEvacuationMax {
		return nil, fmt.Errorf("%w: tối đa %d điểm sơ tán", ErrDisasterEvacuationPoints, DisasterEvacuationMax)
	}
	out := make([]EvacuationPoint, 0, len(pts))
	for i, p := range pts {
		var err error
		bad := func(what string) error {
			return fmt.Errorf("%w: điểm sơ tán thứ %d — %s", ErrDisasterEvacuationPoints, i+1, what)
		}
		if p.Name, err = normalizeText(p.Name, DisasterPlaceNameMaxLen, errBlank, errLong); err != nil {
			return nil, bad(fmt.Sprintf("cần tên, tối đa %d ký tự", DisasterPlaceNameMaxLen))
		}
		if p.Address, err = NormalizeExternalContactAddress(p.Address); err != nil {
			return nil, bad(fmt.Sprintf("địa chỉ tối đa %d ký tự, không ký tự điều khiển", DisasterAddressMaxLen))
		}
		if strings.TrimSpace(p.Phone) == "" {
			p.Phone = ""
		} else if p.Phone, err = NormalizeExternalContactPhone(p.Phone); err != nil {
			return nil, bad("số điện thoại chỉ gồm chữ số và + . - ( ) cùng dấu cách")
		}
		if p.Capacity != nil {
			if *p.Capacity < 0 {
				return nil, bad("sức chứa phải là số không âm")
			}
			c := *p.Capacity
			p.Capacity = &c
		}
		out = append(out, p)
	}
	return out, nil
}

// normalizeEmergencyContacts: at most 50; role, name and phone all required.
func normalizeEmergencyContacts(cs []EmergencyContact) ([]EmergencyContact, error) {
	if len(cs) > DisasterContactsMax {
		return nil, fmt.Errorf("%w: tối đa %d số khẩn cấp", ErrDisasterContacts, DisasterContactsMax)
	}
	out := make([]EmergencyContact, 0, len(cs))
	for i, c := range cs {
		var err error
		bad := func(what string) error {
			return fmt.Errorf("%w: số khẩn cấp thứ %d — %s", ErrDisasterContacts, i+1, what)
		}
		if c.Role, err = normalizeText(c.Role, DisasterContactRoleMaxLen, errBlank, errLong); err != nil {
			return nil, bad(fmt.Sprintf("cần vai trò, tối đa %d ký tự", DisasterContactRoleMaxLen))
		}
		if c.Name, err = normalizeText(c.Name, DisasterContactNameMaxLen, errBlank, errLong); err != nil {
			return nil, bad(fmt.Sprintf("cần tên, tối đa %d ký tự", DisasterContactNameMaxLen))
		}
		if c.Phone, err = NormalizeExternalContactPhone(c.Phone); err != nil {
			return nil, bad("cần số điện thoại, chỉ gồm chữ số và + . - ( ) cùng dấu cách")
		}
		out = append(out, c)
	}
	return out, nil
}

// errBlank / errLong are the internal sentinels normalizeText needs for list elements; the caller turns
// either into the list's own sentence, so neither leaves this file.
var (
	errBlank = errors.New("blank")
	errLong  = errors.New("too long")
)
