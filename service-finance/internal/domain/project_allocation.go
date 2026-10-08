package domain

// A project's funding allocation lines as the project screens read them, and the rules for EDITING
// them (user decisions 06/10/2026, following the prototype: vigov-require
// apps/api/app/modules/budget/service.py:541-610).
//
// THE DECISIONS THIS FILE CARRIES, each of which replaced an earlier "warning only" reading of §9:
//
//	1. allocated total > the project's year plan      REFUSED (allocation_exceeds_plan). Under-allocation
//	                                                   is allowed — a commune declares sources one decision
//	                                                   at a time (prototype NT-1).
//	2. the same source twice in one project            REFUSED (duplicate_source) — ErrPhanBoTrungNguon.
//	3. allocations editable after creation             a FULL REPLACEMENT set; a removed source that already
//	                                                   has vouchers on this project is REFUSED
//	                                                   (source_has_disbursements).
//	4. the chip                                        status + numbers from the server; the words are the
//	                                                   screen's.

import (
	"errors"
	"fmt"
	"math"
	"strconv"
)

// ProjectAllocation is one allocation line of one project as a READ returns it: the source, its name,
// the amount allocated from it and — on the detail read only — what this project's vouchers have
// actually drawn from it.
//
// Disbursed IS DERIVED, by the counting rule of the project's own "đã giải ngân" (store/du_an.go
// tongChungTu: every state, soft-deleted vouchers excluded), restricted to vouchers naming this source.
// Stored nowhere.
type ProjectAllocation struct {
	FundingSourceID string
	SourceName      string
	Amount          Dong
	Disbursed       Dong
}

// DisbursedRatio is disbursed / allocated for this source on this project, in PhanVan. ok = false when
// nothing is allocated — the reason FundingSourceProject.DisbursedRatio gives. Not clamped (§13 rule 2).
// Rounded half away from zero by RatioOf (ADR 0080 #1).
func (a ProjectAllocation) DisbursedRatio() (PhanVan, bool) {
	return RatioOf(a.Disbursed, a.Amount)
}

// FundingStatusOfAllocations is GanNguon over read lines — the ONE rule for the chip, whichever shape
// the lines arrive in.
func FundingStatusOfAllocations(plan Dong, lines []ProjectAllocation) TinhTrangGanNguon {
	pb := make([]PhanBoNguonVon, 0, len(lines))
	for _, l := range lines {
		pb = append(pb, PhanBoNguonVon{NguonVonID: l.FundingSourceID, SoTien: l.Amount})
	}
	return GanNguon(plan, pb)
}

// --- refusals -----------------------------------------------------------------------------------

var (
	// ErrAllocationExceedsPlan — the allocated total would exceed the project's year plan.
	//
	// WHY A REFUSAL (decision 06/10/2026, prototype service.py:589-595): allocating more than the
	// project was given is a figure that does not exist, and it flows straight into §6's "đã phân bổ"
	// per source. Allocating LESS is allowed: the remainder is the chip's shortfall.
	//
	// Raised as *AllocationExceedsPlanError, which carries the two figures so the sentence can name the
	// overrun; errors.Is against this sentinel still matches.
	ErrAllocationExceedsPlan = errors.New("du_an: tổng phân bổ nguồn vốn vượt kế hoạch vốn năm của dự án")

	// ErrSourceHasDisbursements — an edit removes a source from a project while live vouchers of THIS
	// project still name that source (prototype service.py:563-575). Removing it would leave those
	// vouchers pointing at a source the project no longer draws on: §6 would count them on a card the
	// project is no longer part of. The way out is to move or remove those vouchers first.
	ErrSourceHasDisbursements = errors.New("du_an: nguồn vốn này đã có chứng từ giải ngân của dự án nên chưa gỡ được")
)

// AllocationExceedsPlanError is ErrAllocationExceedsPlan with its two figures.
type AllocationExceedsPlanError struct {
	Plan      Dong
	Allocated Dong
}

func (e *AllocationExceedsPlanError) Error() string {
	return fmt.Sprintf("%s (phân bổ %d, kế hoạch %d)", ErrAllocationExceedsPlan.Error(),
		int64(e.Allocated), int64(e.Plan))
}

func (e *AllocationExceedsPlanError) Unwrap() error { return ErrAllocationExceedsPlan }

// Overrun is by how much the allocation exceeds the plan — always positive on a raised error.
func (e *AllocationExceedsPlanError) Overrun() Dong { return e.Allocated - e.Plan }

// Sentence is the clerk-facing refusal, naming the overrun. It carries amounts of public money only —
// no personal data — and no commune id, so it is safe to return as it is.
func (e *AllocationExceedsPlanError) Sentence() string {
	return "`funding_allocations`: tổng các nguồn vốn (" + FormatDong(e.Allocated) +
		") vượt kế hoạch vốn năm của dự án (" + FormatDong(e.Plan) + ") " + FormatDong(e.Overrun()) +
		". Hãy giảm số phân bổ hoặc tăng kế hoạch vốn năm trước."
}

// SumAllocations totals allocation amounts, SATURATING at MaxInt64 rather than wrapping. Each line is
// bounded by SoTienToiDa (10^17) and a request may carry 200 lines, so a plain sum can overflow int64
// and wrap negative — which would pass the plan check it exists to fail.
func SumAllocations(amounts ...Dong) Dong {
	var total Dong
	for _, a := range amounts {
		if a > 0 && total > Dong(math.MaxInt64)-a {
			return Dong(math.MaxInt64)
		}
		total += a
	}
	return total
}

// CheckAllocationWithinPlan refuses an allocated total above the plan. EQUAL IS ACCEPTED: a project
// allocated exactly its plan is the ordinary fully-funded project (§11's `≥`).
func CheckAllocationWithinPlan(plan, allocated Dong) error {
	if allocated > plan {
		return &AllocationExceedsPlanError{Plan: plan, Allocated: allocated}
	}
	return nil
}

// FormatDong renders an amount the way the screens print it: "1.234.567 đ".
func FormatDong(d Dong) string {
	neg := d < 0
	v := int64(d)
	if neg {
		v = -v
	}
	digits := strconv.FormatInt(v, 10)
	out := make([]byte, 0, len(digits)+len(digits)/3+4)
	for i := 0; i < len(digits); i++ {
		if i > 0 && (len(digits)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, digits[i])
	}
	s := string(out) + " đ"
	if neg {
		return "-" + s
	}
	return s
}

// --- replacing a project's allocation set --------------------------------------------------------

// StoredAllocationLine is one row of `phan_bo_nguon_von` as the EDIT path reads it — INCLUDING
// soft-deleted rows, which is the one place they are read, and why: `UNIQUE (tenant_id, du_an_id,
// nguon_von_id)` (0013) counts them, so re-adding a source the project once dropped must REVIVE that
// row; an INSERT would hit the key.
type StoredAllocationLine struct {
	PhanBoNguonVon
	Removed bool
}

// AllocationReplacement is what turning the stored lines into the wanted set takes, row by row.
//
// NEVER "soft delete everything, insert the new set": that reinserts every kept source, and the full
// unique key refuses the second row of each (decision 06/10/2026). Kept sources are updated IN PLACE.
type AllocationReplacement struct {
	Update []PhanBoNguonVon // live line kept, amount moved — new amount
	Revive []PhanBoNguonVon // soft-deleted line of a re-added source — new amount
	Insert []DongPhanBoMoi  // source this project never had a line for
	Remove []PhanBoNguonVon // live line whose source is not in the wanted set
}

// Empty reports whether the wanted set equals the live set — nothing to write, nothing to audit.
func (r AllocationReplacement) Empty() bool {
	return len(r.Update) == 0 && len(r.Revive) == 0 && len(r.Insert) == 0 && len(r.Remove) == 0
}

// PlanAllocationReplacement diffs the stored lines (live and soft-deleted) against the wanted set.
// `wanted` has already been through ChuanHoaPhanBoMoi, so it names each source at most once.
func PlanAllocationReplacement(stored []StoredAllocationLine, wanted []DongPhanBoMoi) AllocationReplacement {
	bySource := make(map[string]StoredAllocationLine, len(stored))
	for _, s := range stored {
		bySource[s.NguonVonID] = s
	}
	var r AllocationReplacement
	keep := make(map[string]struct{}, len(wanted))
	for _, w := range wanted {
		keep[w.NguonVonID] = struct{}{}
		s, ok := bySource[w.NguonVonID]
		switch {
		case !ok:
			r.Insert = append(r.Insert, w)
		case s.Removed:
			line := s.PhanBoNguonVon
			line.SoTien = w.SoTien
			r.Revive = append(r.Revive, line)
		case s.SoTien != w.SoTien:
			line := s.PhanBoNguonVon
			line.SoTien = w.SoTien
			r.Update = append(r.Update, line)
		}
	}
	for _, s := range stored {
		if s.Removed {
			continue
		}
		if _, ok := keep[s.NguonVonID]; !ok {
			r.Remove = append(r.Remove, s.PhanBoNguonVon)
		}
	}
	return r
}

// LiveAllocationLines returns the stored lines that are not soft-deleted.
func LiveAllocationLines(stored []StoredAllocationLine) []PhanBoNguonVon {
	out := make([]PhanBoNguonVon, 0, len(stored))
	for _, s := range stored {
		if !s.Removed {
			out = append(out, s.PhanBoNguonVon)
		}
	}
	return out
}

// TotalOfLines is SumAllocations over lines.
func TotalOfLines(lines []PhanBoNguonVon) Dong {
	amounts := make([]Dong, 0, len(lines))
	for _, l := range lines {
		amounts = append(amounts, l.SoTien)
	}
	return SumAllocations(amounts...)
}
