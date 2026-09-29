package domain

import (
	"errors"
	"strings"
)

// MiniAppMode is the mode a registered Zalo Mini App runs in (ADR 0044). The two values are the
// database's own (`mini_app.mode`), and a third one is a decision, not an edit.
type MiniAppMode string

const (
	MiniAppModeMain    MiniAppMode = "chinh" // the ViHAT-owned main app; never bound to a commune
	MiniAppModeCommune MiniAppMode = "rieng" // a commune's dedicated app, bound to exactly one commune
)

// MiniApp is one registered, ACTIVE, not-deleted app as the registry answers it.
//
// Tenant is set exactly when Mode is MiniAppModeCommune, and it carries the bound commune WHATEVER ITS STATE:
// deciding that an inactive commune means "refuse" is the caller's job, and the answer has to let
// it tell the two apart. It is never followed through tenant_succession (ADR 0045 §Chế độ).
type MiniApp struct {
	AppID  string
	Mode   MiniAppMode
	Tenant *Tenant
}

// ErrAppIDEmpty — an empty App ID is a malformed question, not an unknown app.
var ErrAppIDEmpty = errors.New("mini_app: app_id không được để trống")

// ValidateAppID refuses an App ID that cannot name anything.
//
// IT DOES NOT NORMALISE. An App ID is an exact string issued by Zalo; trimming or lower-casing it
// here would make two different strings resolve to one app. Surrounding whitespace is refused
// rather than stripped, matching the CHECK on the column.
func ValidateAppID(appID string) error {
	if appID == "" || strings.TrimSpace(appID) != appID {
		return ErrAppIDEmpty
	}
	return nil
}

// CommuneProfile is a commune's display profile (ADR 0045, decision 5). "" means "not declared".
//
// OfficeHoursText is DISPLAY TEXT. The working-hours calendar that deadlines count against is
// service-identity's (ADR 0007); nothing may parse this field into hours.
type CommuneProfile struct {
	OfficeAddress   string
	LogoURL         string
	Hotline         string // official office number — public-service information (open question #16)
	OfficeHoursText string
	Introduction    string
}
