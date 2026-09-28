package domain

import "errors"

// A petition's PUBLICATION STATUS — staff moderation of the public page, ADR 0050 point 8 (round 4,
// 28/09/2026), column `publication_status` (migration 0017). THIS FILE IMPORTS THE STANDARD LIBRARY
// AND NOTHING ELSE.
//
//	cho-duyet   no member of staff has decided yet — the default a petition is born with
//	cong-khai   a member of staff approved it for the public page
//	an          hidden by staff, or never publishable (staff conduct, `can-bo`)
//
// SEPARATE FROM `trang_thai`, AND A MODERATION NEVER MOVES IT (user decision 28/09/2026). The
// requirement repository moves RECEIVED -> SCREENING as a side effect of moderating
// (`service.py:789-790`); that edge is NOT copied: deadlines are fixed only at classification
// (rule 10, invariant 2), and a status the citizen is notified about must not change as the by-product
// of a decision about a web page.
//
// THE VALUES ARE THE USER'S (migration 0017 header), Vietnamese without diacritics like every enum
// value (ADR 0011). The migration's CHECK `phieu_phan_anh_publication_status_valid` is the floor under
// the closed list; this file refuses first, in a sentence.

// PublicationStatus is one of the three values of `phieu_phan_anh.publication_status`.
type PublicationStatus string

const (
	PublicationPending PublicationStatus = "cho-duyet"
	PublicationPublic  PublicationStatus = "cong-khai"
	PublicationHidden  PublicationStatus = "an"
)

// Valid reports whether s is one of the three stored values.
func (s PublicationStatus) Valid() bool {
	return s == PublicationPending || s == PublicationPublic || s == PublicationHidden
}

var (
	// ErrPublicationStatusInvalid — the body named something other than `cong-khai` or `an`.
	//
	// `cho-duyet` IS REFUSED TOO. It means "nobody has decided", so a staff act writing it would record
	// a decision that says no decision was taken. The requirement repository's screen offers exactly
	// two buttons, "Cho hiện công khai" and "Ẩn khỏi trang công khai"
	// (`FeedbackDetailDrawer.tsx:270-306`); its API schema admits `pending` as well, but nothing there
	// ever sends it.
	ErrPublicationStatusInvalid = errors.New(
		"phan_anh: `status` chỉ nhận `cong-khai` hoặc `an`")

	// ErrNeverPublic — publishing a petition in the staff-conduct field `can-bo`. A refusal about the
	// RECORD (409), never about what was typed. Requirement: `service.py:776-782`, code `never_public`;
	// migration 0017's CHECK `phieu_phan_anh_staff_conduct_never_public` is the floor under it.
	ErrNeverPublic = errors.New(
		"phan_anh: phiếu thuộc lĩnh vực tác phong cán bộ không bao giờ được hiển thị công khai")
)

// CheckPublicationRequest validates the target a member of staff asked for. FAIL CLOSED: anything but
// the two acts is refused, and the input is not echoed.
func CheckPublicationRequest(raw string) (PublicationStatus, error) {
	switch s := PublicationStatus(raw); s {
	case PublicationPublic, PublicationHidden:
		return s, nil
	default:
		return "", ErrPublicationStatusInvalid
	}
}

// PublicationAllowed decides whether petition p may be put into status `to`, on the row read under
// the lock. Only one refusal exists, the requirement's: a `can-bo` petition is never `cong-khai`.
//
// NO LIFECYCLE STATUS IS REFUSED, following the requirement (`service.py:764-793` checks none): whether
// a petition may be shown in public is a decision ABOUT the record, and a `da-dong` or
// `khong-tiep-nhan` petition is as much a record as a `dang-xu-ly` one.
func PublicationAllowed(p PhieuPhanAnh, to PublicationStatus) error {
	if to == PublicationPublic && p.LinhVuc == LinhVucHanChe {
		return ErrNeverPublic
	}
	return nil
}

// InitialPublicationStatus is the status a petition is BORN with: `an` in the staff-conduct field,
// `cho-duyet` otherwise — the requirement's `service.py:301-307` ("skips moderation and stays hidden,
// so nobody can approve a conduct report onto a public board by accident"). An unclassified petition
// (field "") is in no field yet, so it waits like any other.
func InitialPublicationStatus(field string) PublicationStatus {
	if field == LinhVucHanChe {
		return PublicationHidden
	}
	return PublicationPending
}

// PublicationAfterClassification is the status once the field is settled to `field`. Classifying INTO
// `can-bo` hides the petition in the SAME statement that settles the field — without it, a petition
// published while unclassified and then classified as staff conduct would fail migration 0017's CHECK
// (a 500), or, worse, the rule would depend on the CHECK alone. Every other field leaves the staff
// decision already taken exactly where it was.
func PublicationAfterClassification(current PublicationStatus, field string) PublicationStatus {
	if field == LinhVucHanChe {
		return PublicationHidden
	}
	return current
}
