package domain

// Which funding source a disbursement voucher may name, given the project it is filed against
// (user decision 06/10/2026, following the prototype: vigov-require
// apps/api/app/modules/budget/service.py:877-899 and DisbursementForm.tsx).
//
//	the project has ≥1 live allocation line   a source is REQUIRED (source_required) and must be one
//	                                          of those lines' sources (source_not_allocated)
//	the project has no allocation line        NO source may be named (source_not_allocated) — the
//	                                          prototype's form shows no select at all
//
// WHY "REQUIRED" ONCE A PROJECT HAS SOURCES — the prototype's own sentence: *"khi đã khai, bỏ trống là
// làm hỏng đúng con số mà việc khai nguồn sinh ra để trả lời: tiền tỉnh đã tiêu tới đâu."* A voucher
// with no source on such a project lands in §6's "đã chi nhưng chưa ghi rút từ nguồn nào" warning,
// which is a figure about missing data, not a state to create on purpose.
//
// WHY A PROJECT WITH NO LINES STILL HAS VOUCHERS WITH NO SOURCE: §13 rule 6 keeps that state legal.
// A commune that tracks by category only enters payments normally; nothing here forces it to declare
// sources first. ⚠ The prototype is looser on this one side — it accepts any source on such a project
// (service.py:889-890 returns before looking at the source). The user decided the stricter reading on
// 06/10/2026: a source the project does not draw on is money on a card the project is not part of.

import "errors"

var (
	// ErrSourceRequired — the project draws on named sources and the voucher names none.
	ErrSourceRequired = errors.New("chung_tu: dự án đã khai nguồn vốn — chứng từ phải ghi rút từ nguồn nào")

	// ErrSourceNotAllocated — the voucher names a source this project has no live allocation line for
	// (including: the project has no line at all).
	ErrSourceNotAllocated = errors.New("chung_tu: nguồn vốn này không được phân bổ cho dự án của chứng từ")
)

// CheckVoucherSource decides whether `source` (already trimmed; "" = no source) is admissible on a
// project whose live allocation lines name `allocated`.
//
// THE CALLER MUST READ `allocated` UNDER A LOCK THAT CONFLICTS WITH AN ALLOCATION EDIT, inside the
// transaction that writes the voucher (store.ChungTuGiaiNganStore.ProjectForVoucherWrite takes the
// project row FOR SHARE). Read without it, a concurrent edit dropping a source can commit between this
// check and the voucher's INSERT.
func CheckVoucherSource(allocated []string, source string) error {
	if len(allocated) == 0 {
		if source != "" {
			return ErrSourceNotAllocated
		}
		return nil
	}
	if source == "" {
		return ErrSourceRequired
	}
	for _, a := range allocated {
		if a == source {
			return nil
		}
	}
	return ErrSourceNotAllocated
}

// ProjectVoucher is one voucher as the project's voucher list returns it: the row plus the NAME of
// the source it draws on, so the screen's `NGUỒN VỐN` column needs no second read. FundingSourceName
// is "" when the voucher names no source.
type ProjectVoucher struct {
	Voucher           ChungTuGiaiNgan
	FundingSourceName string
}
