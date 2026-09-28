package store

// RestrictedPetitionAuditSubjects is the subquery core/audit.WithHiddenSubjects is wired with: the
// `audit_log.subject` values of this commune's petitions in the restricted field `can-bo` (ADR 0030,
// ADR 0054 §4). Entries about those petitions are withheld from GET /api/v1/petitions-audit-entries
// unless the reader holds `feedback.restricted` — otherwise `admin.audit` would be a second way to
// learn that a complaint against an official exists, and who touched it.
//
// WHY `ma_tra_cuu`: every petition entry this service writes names the petition by its lookup code
// (internal/app: gui_phan_anh.go, xu_ly_phan_anh.go, nhat_ky_phan_anh.go, petition_rating.go,
// petition_publication.go, xem_nguoi_gui.go — all `Subject: …MaTraCuu`). The id would match nothing.
//
// BUILT FROM restrictedFieldExclusion, NOT A SECOND SPELLING OF THE FIELD. That constant is the
// predicate the register list and every overview figure exclude with; `NOT (TRUE <it>)` is its exact
// complement — `linh_vuc IS NULL` gives TRUE inside, so an unclassified petition is NOT hidden, the
// same answer the list gives. Were the field rule ever to change, the list and this log change together.
//
// NO `deleted_at` PREDICATE, ON PURPOSE: a soft-deleted petition's entries are still in the trail
// (rule 6, invariant 4), and a report about a member of staff does not stop being one by being deleted.
//
// THE FIELD IS READ AS IT IS NOW. A petition reclassified INTO `can-bo` hides its earlier entries too,
// and one reclassified out shows them all — the trail follows what the petition is, which is what the
// list does with the petition itself.
//
// Only $1 (the commune) — WithHiddenSubjects refuses anything else at wiring time.
const RestrictedPetitionAuditSubjects = `SELECT ma_tra_cuu FROM phieu_phan_anh WHERE tenant_id = $1 AND NOT (TRUE` +
	restrictedFieldExclusion + `)`
