package domain

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// The rules for ADDING and REMOVING a field's own deadline row — ADR 0079 lô 2 Q4 ("Theo prototype"):
// a commune may add a row of its own for a field of any kind of work, remove one with a reason, and
// never remove the default row. No approval step.
//
// WHAT A "FIELD" IS DEPENDS ON THE KIND OF WORK, and that is the whole reason the per-kind check is
// not here: `phan-anh` rows name a tier-1 petition field (platform, ADR 0026/0060), `van-ban-den`
// rows a document type (service-documents), `nhiem-vu` rows a task priority (service-petitions).
// Whether a code is real is a question for the service that owns the list (rule 2), asked by the use
// case. This file holds only what is true of every field code whatever list it comes from.
//
// `don-thu` HAS NO FIELD ROWS — the default row only (ADR 0079, "Đơn thư trong bảng thời hạn").

var (
	// ErrSLAFieldMissing — an empty field code. The empty code IS the default row (LaDongMacDinh), and
	// the default row is never created through this path: it comes from seeding (GieoMacDinh).
	ErrSLAFieldMissing = errors.New("sla: thiếu mã lĩnh vực của dòng thời hạn riêng")

	// ErrSLAFieldMalformed — a code too long, or carrying a control character or a space. Codes in
	// every list a field can come from are short ASCII slugs; anything else is a mistake, and it would
	// be stored as a VALUE nothing can correct afterwards.
	ErrSLAFieldMalformed = errors.New("sla: mã lĩnh vực không hợp lệ")

	// ErrSLAKindHasNoFieldRows — `don-thu`: the default row only (ADR 0079, main-session note in lô 3).
	ErrSLAKindHasNoFieldRows = errors.New("sla: loại việc này chỉ có dòng thời hạn mặc định, không thêm dòng riêng")

	// ErrSLAKindUnknown — a kind of work the table does not admit.
	ErrSLAKindUnknown = errors.New("sla: loại việc không hợp lệ")

	// ErrSLADefaultRowNotRemovable — every field without a row of its own falls back to the default row
	// (DongTheoLinhVuc); removing it would leave the commune unable to compute any deadline of that kind.
	ErrSLADefaultRowNotRemovable = errors.New("sla: không xoá được dòng thời hạn mặc định")

	// ErrSLADeleteReasonTooLong — a reason that IS typed, past SLADeleteReasonMaxLen. A blank one is
	// no longer refused — see SLAFieldRowDeleteDefaultReason.
	ErrSLADeleteReasonTooLong = errors.New("sla: lý do xoá quá dài")
)

// SLAFieldRowDeleteDefaultReason is `delete_reason` when the person removing a field's own deadline
// row types none.
//
// OPTIONAL ON THE WAY IN (owner decision 10/10/2026: the screen drops the reason box), NEVER EMPTY IN
// THE ROW — rule 7, invariant 1 names `delete_reason` beside `deleted_at` and `deleted_by`; the shape
// of ADR 0075 #4a and ADR 0077 #3. The sentence says what removing the row DOES: that field falls
// back to the default row (DongTheoLinhVuc), which is what a reader of the record needs to know.
const SLAFieldRowDeleteDefaultReason = "Xoá dòng thời hạn riêng của lĩnh vực, lĩnh vực quay về dùng dòng thời hạn mặc định (người xoá không nhập lý do)"

// SLAFieldMaxLen and SLADeleteReasonMaxLen bound what a person types. The field bound is the catalogue
// code bound (MaDanhMucToiDa); the reason bound is the one every soft delete in this service uses.
const (
	SLAFieldMaxLen        = 64
	SLADeleteReasonMaxLen = 500
)

// NormalizeSLAField trims and shape-checks a field code. It does NOT say the code exists — that is
// the owning service's answer, asked by the caller.
func NormalizeSLAField(field string) (string, error) {
	field = strings.TrimSpace(field)
	switch {
	case field == "":
		return "", ErrSLAFieldMissing
	case len(field) > SLAFieldMaxLen:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrSLAFieldMalformed, SLAFieldMaxLen)
	}
	for _, r := range field {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return "", ErrSLAFieldMalformed
		}
	}
	return field, nil
}

// CheckSLAKindTakesFieldRows refuses a kind of work that cannot carry a field's own row.
func CheckSLAKindTakesFieldRows(kind LoaiViec) error {
	switch {
	case !kind.HopLe():
		return ErrSLAKindUnknown
	case kind == LoaiViecDonThu:
		return ErrSLAKindHasNoFieldRows
	}
	return nil
}

// CheckSLARowRemovable refuses removing the default row. Every other live row may be removed.
func CheckSLARowRemovable(d DongSLA) error {
	if d.LaDongMacDinh() {
		return ErrSLADefaultRowNotRemovable
	}
	return nil
}

// NormalizeSLADeleteReason trims and bounds the reason recorded beside a soft delete; a blank one
// becomes SLAFieldRowDeleteDefaultReason.
func NormalizeSLADeleteReason(reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	switch {
	case reason == "":
		return SLAFieldRowDeleteDefaultReason, nil
	case len([]rune(reason)) > SLADeleteReasonMaxLen:
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrSLADeleteReasonTooLong, SLADeleteReasonMaxLen)
	}
	return reason, nil
}
