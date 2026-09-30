package domain

import (
	"errors"
	"strings"
)

// CheDoMiniApp is the mode a registered Zalo Mini App runs in (ADR 0044). The two values are the
// database's own (`mini_app.che_do`), and a third one is a decision, not an edit.
type CheDoMiniApp string

const (
	CheDoChinh CheDoMiniApp = "chinh" // the ViHAT-owned main app; never bound to a commune
	CheDoRieng CheDoMiniApp = "rieng" // a commune's dedicated app, bound to exactly one commune
)

// MiniApp is one registered, ACTIVE, not-deleted app as the registry answers it.
//
// Xa is set exactly when CheDo is CheDoRieng, and it carries the bound commune WHATEVER ITS STATE:
// deciding that an inactive commune means "refuse" is the caller's job, and the answer has to let
// it tell the two apart. It is never followed through tenant_succession (ADR 0045 §Chế độ).
type MiniApp struct {
	AppID string
	CheDo CheDoMiniApp
	Xa    *Tenant
}

// ErrAppIDTrong — an empty App ID is a malformed question, not an unknown app.
var ErrAppIDTrong = errors.New("mini_app: app_id không được để trống")

// KiemAppID refuses an App ID that cannot name anything.
//
// IT DOES NOT NORMALISE. An App ID is an exact string issued by Zalo; trimming or lower-casing it
// here would make two different strings resolve to one app. Surrounding whitespace is refused
// rather than stripped, matching the CHECK on the column.
func KiemAppID(appID string) error {
	if appID == "" || strings.TrimSpace(appID) != appID {
		return ErrAppIDTrong
	}
	return nil
}

// HoSoHienThi is a commune's display profile (ADR 0045, decision 5). "" means "not declared".
//
// GioLamViecHienThi is DISPLAY TEXT. The working-hours calendar that deadlines count against is
// service-identity's (ADR 0007); nothing may parse this field into hours.
type HoSoHienThi struct {
	DiaChiTruSo       string
	LogoURL           string
	DuongDayNong      string // official office number — public-service information (open question #16)
	GioLamViecHienThi string
	GioiThieu         string
}
