package domain

import (
	"errors"
	"strings"
)

// The commune of a citizen session opened through the Mini App bridge — ADR 0045 §Chế độ và xã
// của phiên, as one pure function so every row of that table is a test that needs no database and
// no platform.
//
// THE TABLE, restated only as far as the code needs it (the ADR owns the reasons):
//
//	app mode   condition                       commune of the session        remembered commune
//	--------   -----------------------------   ---------------------------   ------------------
//	(unknown)  app_id not registered           REFUSE                        —
//	dedicated  bound commune active            that commune; hint IGNORED    not used
//	dedicated  bound commune not active        REFUSE (no auto-succession)   —
//	main       hint + citizen CONFIRMED        hint, must be active, else    becomes hint
//	                                           REFUSE
//	main       not confirmed (hint or not)     remembered if still active,   unchanged
//	                                           else NO COMMUNE
//
// Whether a commune is ACTIVE is a platform read, so this function returns a candidate plus the
// rule for an inactive answer; the use case asks the registry and applies it.
//
// Communes are plain strings here: domain/ imports nothing but the standard library, and the
// ULID check on the hint happens in the use case (core/tenant owns it).

// CheDoApp is the mode of the Mini App the session is opened through (ADR 0044).
type CheDoApp int

const (
	CheDoAppKhongRo CheDoApp = iota
	CheDoAppChinh
	CheDoAppRieng
)

// NguonXa says HOW the session's commune was decided. It goes into the audit entry verbatim — the
// three values are the ones ADR 0045 §Ghi vết names.
type NguonXa string

const (
	NguonXaAppRieng  NguonXa = "app_rieng"
	NguonXaQRXacNhan NguonXa = "qr_xac_nhan"
	NguonXaNhoLai    NguonXa = "nho_lai"
	NguonXaKhongCoXa NguonXa = ""
)

// DauVaoChonXa is everything the table above reads.
type DauVaoChonXa struct {
	CheDo CheDoApp

	// XaCuaAppRieng is the commune bound to a dedicated app, as the platform answered it: present
	// only when the binding exists and the commune is active. "" under CheDoAppRieng = not active.
	XaCuaAppRieng string

	// GoiY is the QR's `t=`, verbatim. Client-supplied: on its own it enters nothing.
	GoiY string

	// DaXacNhan is the citizen's explicit act on the confirmation screen.
	DaXacNhan bool

	// XaDaNho is the account's remembered commune (main app), "" when none.
	XaDaNho string
}

// UngVienXa is the table's answer before the registry has been asked whether the commune is active.
type UngVienXa struct {
	// Xa is the candidate commune, "" for "no commune" (main app, nothing confirmed or remembered).
	Xa    string
	Nguon NguonXa

	// TuChoiNeuNgung: an inactive candidate REFUSES the whole call (dedicated app, confirmed QR).
	// false: an inactive candidate degrades to NO COMMUNE (the remembered commune of the main app,
	// which a merger may have deactivated — the citizen still opens the app, and a new QR is how
	// they enter the new commune).
	TuChoiNeuNgung bool

	// NhoXa: when the session is issued, the account's remembered commune becomes Xa.
	NhoXa bool
}

var (
	// ErrCheDoAppKhongRo — a mode this code does not know. A third mode is a decision (ADR 0044),
	// not something to guess the meaning of.
	ErrCheDoAppKhongRo = errors.New("cầu phiên: chế độ app không rõ")

	// ErrAppRiengKhongCoXa — a dedicated app whose bound commune is not active. Refused, and the
	// bridge does NOT follow tenant_succession on its own (ADR 0045: re-binding an app after a
	// merger is an operator's act, with a trail).
	ErrAppRiengKhongCoXa = errors.New("cầu phiên: app riêng không gắn với xã nào đang hoạt động")

	// ErrXacNhanKhongCoGoiY — commune_confirmed without tenant_hint: a wiring fault in the caller.
	ErrXacNhanKhongCoGoiY = errors.New("cầu phiên: có xác nhận xã mà không có tham số xã của QR")
)

// ChonXa applies the table.
func ChonXa(v DauVaoChonXa) (UngVienXa, error) {
	switch v.CheDo {
	case CheDoAppRieng:
		// tenant_hint is IGNORED in this mode, confirmed or not: the app IS the commune.
		if strings.TrimSpace(v.XaCuaAppRieng) == "" {
			return UngVienXa{}, ErrAppRiengKhongCoXa
		}
		return UngVienXa{Xa: v.XaCuaAppRieng, Nguon: NguonXaAppRieng, TuChoiNeuNgung: true}, nil

	case CheDoAppChinh:
		goiY := strings.TrimSpace(v.GoiY)
		if v.DaXacNhan {
			if goiY == "" {
				return UngVienXa{}, ErrXacNhanKhongCoGoiY
			}
			return UngVienXa{Xa: goiY, Nguon: NguonXaQRXacNhan, TuChoiNeuNgung: true, NhoXa: true}, nil
		}
		// NOT CONFIRMED — the hint, if any, is ignored. "A QR does not put anybody into any commune"
		// (ADR 0045): the first QR open therefore costs two calls, and that is the price.
		if nho := strings.TrimSpace(v.XaDaNho); nho != "" {
			return UngVienXa{Xa: nho, Nguon: NguonXaNhoLai}, nil
		}
		return UngVienXa{Nguon: NguonXaKhongCoXa}, nil
	}
	return UngVienXa{}, ErrCheDoAppKhongRo
}
