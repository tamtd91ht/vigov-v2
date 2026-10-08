package domain

import (
	"strings"
	"time"
)

// WHO OWNS A CITIZEN-CHANNEL PETITION — ADR 0080.
//
// Before ADR 0080 a petition had two shapes: filed by a verified citizen (`cong_dan_id`) or booked by
// staff (no owner). ADR 0080 decision 2 adds a third: sent from a Mini App session WITHOUT a verified
// phone, owned by the session's Zalo account (`zalo_account_id`, migration 0032). The typed name and
// phone on such a petition are CONTACT DETAILS, never an identity.
//
// THE OWNER IS A TYPE WITH A KIND, NOT A STRING. A bare id cannot say which column it belongs in, and
// the failure is silent both ways: a Zalo account id written into `cong_dan_id` makes a petition owned
// by a citizen who does not exist; a citizen id compared with `zalo_account_id` matches nothing and
// looks like "not found". Every reader and writer switches on Kind, and an unknown Kind is refused.

// PetitionOwnerKind names which owner column a citizen-channel petition is keyed by.
//
// THE VALUES EQUAL authz.KindCitizen / authz.KindZaloAccount, but nothing converts one into the other
// by string: the handler maps the principal's Kind explicitly (http/gui_phan_anh.go, channelOwner),
// so a third principal kind cannot reach a store by spelling coincidence.
type PetitionOwnerKind string

const (
	// OwnerCitizen: a verified citizen — `cong_dan_id`, the opaque citizen id.
	OwnerCitizen PetitionOwnerKind = "citizen"
	// OwnerZaloAccount: a Mini App session with no verified phone — `zalo_account_id`, i.e.
	// `tai_khoan_zalo.id` in service-identity (ADR 0080 decisions 2 and 9).
	OwnerZaloAccount PetitionOwnerKind = "zalo-account"
)

// PetitionOwner is the owner of a citizen-channel petition, as the SESSION names it (rule 4,
// invariant 2). Never built from a request value.
type PetitionOwner struct {
	Kind PetitionOwnerKind
	ID   string
}

// Valid reports whether the owner names one of the two kinds AND a non-blank id. An invalid owner is
// a wiring fault in the caller — refused, never treated as "no filter".
func (o PetitionOwner) Valid() bool {
	if strings.TrimSpace(o.ID) == "" {
		return false
	}
	return o.Kind == OwnerCitizen || o.Kind == OwnerZaloAccount
}

// ContactUnverified reports that the petition's contact details were typed by hand and never verified
// — the petition is owned by a Zalo account (ADR 0080 decision 5). DERIVED from the owner column on
// every read, never stored beside it: a flag next to the column would be a second copy of one fact
// (rule 9), and the day they disagree the label lies to staff about whether a number was verified.
func (p PhieuPhanAnh) ContactUnverified() bool { return p.ZaloAccountID != "" }

// UnverifiedDailyCeiling is how many UNVERIFIED petitions (ADR 0080) one Zalo account may send in one
// commune in one day.
//
// A SECURITY THRESHOLD CHOSEN BY THE OWNER — ADR 0080 decision 7 (08/10/2026), rule 13. A NAMED
// CONSTANT AND NOT CONFIGURATION, on purpose: loosening or tightening it is rule 13's stop condition
// and ADR 0080 stop condition #5, so it must change through a reviewed code change, never through a
// value somebody edits on a screen. It is the compensation ADR 0080 cost #1 names for the weaker
// accountability of a petition traceable only to a Zalo account.
const UnverifiedDailyCeiling = 10

// UnverifiedCountingDayStart is the start of the day — 00:00 in Asia/Ho_Chi_Minh — that `now` falls
// in, the lower bound of the UnverifiedDailyCeiling count.
//
// WHY ASIA/HO_CHI_MINH AND NOT A PER-COMMUNE ZONE: no commune configuration in this system carries a
// time zone, and every commune is in Viet Nam. This service already reads "the day" in this zone for
// its date keys and windows (AutomationZone, a FIXED +07:00 — Viet Nam has no daylight saving and a
// fixed zone needs no tzdata in the image). Reused here rather than a second constant for one fact.
//
// NOT A DEADLINE. It bounds an abuse counter, and no commitment to a citizen is computed from it, so
// working hours (ADR 0007) do not apply. Built with time.Date on the local date, so no wall-clock
// arithmetic is done at all.
func UnverifiedCountingDayStart(now time.Time) time.Time {
	l := now.In(AutomationZone)
	return time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, AutomationZone)
}
