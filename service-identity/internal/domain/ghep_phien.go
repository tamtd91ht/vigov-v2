package domain

import "time"

// The state of a pairing code — @entity SessionPairing, table ghep_phien (ADR 0019, ADR 0023
// §4, migration 0004).
//
// # THE STATE IS DERIVED, AND THERE IS NO COLUMN HOLDING IT
//
// ghep_phien has no `trang_thai` column. The state is computed from three timestamps, here, as
// a pure function of them plus the clock:
//
//	ChoGhep   het_han_luc > now()  AND dung_luc IS NULL AND huy_luc IS NULL
//	DaGhep    dung_luc IS NOT NULL
//	DaHuy     huy_luc  IS NOT NULL
//	HetHan    het_han_luc <= now() AND dung_luc IS NULL AND huy_luc IS NULL
//
// "Expired" in particular must never become a column: it is a comparison against a clock
// (rule 10, invariant 3), and a column holding it is wrong for as long as no job has run —
// which on a two-minute TTL is most of the time. A derived value cannot drift from the facts
// it is derived from; a stored one drifts the first time a job is late.
//
// WHY IT LIVES IN domain/ AND NOT NEXT TO THE SQL: it is a business rule with no
// infrastructure in it, so it is testable without a database and there is exactly ONE place
// the rule is written. The redeem statement in store/ enforces the same conditions in its
// WHERE clause — that is the database refusing, not a second copy of the rule to be kept in
// step: the statement can only ever refuse MORE than this function reports, never less.

// TrangThaiMaGhep is the derived state of one pairing code.
type TrangThaiMaGhep string

const (
	// ChoGhep is the only state in which a code can be redeemed.
	ChoGhep TrangThaiMaGhep = "cho_ghep"
	DaGhep  TrangThaiMaGhep = "da_ghep"
	DaHuy   TrangThaiMaGhep = "da_huy"
	HetHan  TrangThaiMaGhep = "het_han"
)

// TrangThaiGhep derives the state of a pairing code from its three timestamps.
//
// `bayGio` IS AN ARGUMENT, NOT time.Now() READ INSIDE. A rule that reads the clock itself can
// only be tested by waiting, so the boundary case — the instant the code expires — is the one
// case that never gets a test. It is also the one that decides whether a citizen standing at
// the counter is refused.
//
// EXPIRY IS `het_han_luc <= bayGio`, NOT `<`. The database stores `het_han_luc = tao_luc +
// 120s` and ADR 0019 invariant 1 caps the life of the code AT 120 seconds; at exactly the
// boundary the code has lived its full span, and the redeem statement uses `het_han_luc >
// now()`, which refuses there. Choosing the other comparison here would make this function
// report ChoGhep for a code the database will not let anyone redeem.
//
// USED WINS OVER CANCELLED, and the schema makes the case impossible anyway (CONSTRAINT
// ghep_phien_khong_vua_dung_vua_huy). If a row ever showed both, a session HAS been issued to
// a screen, and reporting "cancelled" would tell an operator the opposite of what happened.
func TrangThaiGhep(hetHanLuc time.Time, dungLuc, huyLuc *time.Time, bayGio time.Time) TrangThaiMaGhep {
	switch {
	case dungLuc != nil:
		return DaGhep
	case huyLuc != nil:
		return DaHuy
	case !hetHanLuc.After(bayGio):
		return HetHan
	default:
		return ChoGhep
	}
}
