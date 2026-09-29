package domain

import "testing"

// WHAT THIS FILE PROVES: the chip rule of §11 and the three bars of §6, against the
// specification's OWN sample figures. Where a case uses a number from docs/ui-ux/06-giai-ngan.md it
// says so — a test that invents its own expected value only proves the code agrees with itself.
//
// WHAT IT DOES NOT PROVE: nothing here touches a database, a route or a permission. The commune
// does not appear at all; isolation is not a property this package can hold or break.

// --- §11, the chip on a project row ---------------------------------------------------------------

func TestFundingStateOfNoRowsIsUnfunded(t *testing.T) {
	// §11: "không có bản ghi ⇒ Chưa gắn nguồn". §9 says that state is NORMAL — "Xã theo dõi kế
	// hoạch vốn theo hạng mục thì để trống cũng được" — so it must be its own state and not folded
	// into "Chưa đủ", which reads on screen as something left unfinished.
	status := FundingStateOf(7_500_000_000, nil)
	if status.Status != FundingStatusUnfunded {
		t.Fatalf("trạng thái = %q, muốn %q", status.Status, FundingStatusUnfunded)
	}
	if status.SourceCount != 0 || status.AllocatedTotal != 0 {
		t.Fatalf("số nguồn = %d, tổng phân bổ = %d — cả hai phải là 0", status.SourceCount, status.AllocatedTotal)
	}
}

func TestFundingStateOfIsFundedWhenTotalEqualsPlan(t *testing.T) {
	// §11 writes the condition with `≥`. A project allocated EXACTLY its plan is the ordinary case,
	// and reading it as "Chưa đủ" would put an orange chip on the most normal row in the commune.
	// The boundary is the only value that tells `>=` from `>`.
	status := FundingStateOf(100_000_000, []FundingAllocation{
		{FundingSourceID: "nv-xa", AllocatedAmount: 60_000_000},
		{FundingSourceID: "nv-tp", AllocatedAmount: 40_000_000},
	})
	if status.Status != FundingStatusFunded {
		t.Fatalf("trạng thái = %q, muốn %q (§11: SUM ≥ kế hoạch)", status.Status, FundingStatusFunded)
	}
	if status.SourceCount != 2 {
		t.Fatalf("số nguồn = %d, muốn 2 — chip §11 in ra 'Đủ · N nguồn'", status.SourceCount)
	}
}

func TestFundingStateOfOneDongShortIsUnderfunded(t *testing.T) {
	// One đồng below the plan. The pair with the case above is what makes the comparison itself
	// tested rather than one side of it.
	status := FundingStateOf(100_000_000, []FundingAllocation{{FundingSourceID: "nv-xa", AllocatedAmount: 99_999_999}})
	if status.Status != FundingStatusUnderfunded {
		t.Fatalf("trạng thái = %q, muốn %q", status.Status, FundingStatusUnderfunded)
	}
	if status.AllocatedTotal != 99_999_999 {
		t.Fatalf("tổng phân bổ = %d, muốn 99999999", status.AllocatedTotal)
	}
}

func TestFundingStateOfCountsDISTINCTSourcesNotRows(t *testing.T) {
	// THE CASE THAT EXISTS BECAUSE OF A SCHEMA DECISION, not because of a screen. Migration 0007
	// deliberately leaves `UNIQUE (tenant_id, du_an_id, nguon_von_id)` undeclared — it is an open
	// question for the customer — so two rows CAN name one source. The chip must then read "2
	// nguồn", not "3": a count of rows would tell a commune it has funding from three places when
	// it has two, and nothing on the screen would say otherwise.
	status := FundingStateOf(100_000_000, []FundingAllocation{
		{FundingSourceID: "nv-xa", AllocatedAmount: 30_000_000},
		{FundingSourceID: "nv-xa", AllocatedAmount: 30_000_000},
		{FundingSourceID: "nv-tp", AllocatedAmount: 40_000_000},
	})
	if status.SourceCount != 2 {
		t.Fatalf("số nguồn = %d, muốn 2 — đếm DÒNG thay vì đếm nguồn riêng biệt", status.SourceCount)
	}
	// The MONEY, unlike the count, is the sum of every row: two allocations from one source are two
	// real amounts.
	if status.AllocatedTotal != 100_000_000 {
		t.Fatalf("tổng phân bổ = %d, muốn 100000000", status.AllocatedTotal)
	}
}

// --- §6, the card of one funding source ------------------------------------------------------------

// communeBudget is the first row of §6's sample table, with the figures its card prints:
// tổng nguồn 9,2 tỷ · đã phân bổ 70 triệu · đã giải ngân 13,2 triệu · 2 dự án.
var communeBudget = FundingSourceProgress{
	FundingSource:          FundingSource{ID: "nv-xa", Name: "Ngân sách xã, phường", Year: 2026, TotalAmount: 9_200_000_000},
	AllocatedAmount:        70_000_000,
	DisbursedAmount:        13_200_000,
	InvestmentProjectCount: 2,
}

func TestFundingSourceProgressMatchesSpecExample(t *testing.T) {
	// §6's card, all three bars plus the remainder line "còn 9,1 tỷ chưa phân bổ".
	if unallocated := communeBudget.UnallocatedAmount(); unallocated != 9_130_000_000 {
		t.Fatalf("còn chưa phân bổ = %d, muốn 9130000000 (§6: 'còn 9,1 tỷ')", unallocated)
	}

	if ratio, ok := communeBudget.AllocatedRatio(); !ok || ratio != 76 {
		t.Fatalf("đã phân bổ/tổng nguồn = %d phần vạn (ok=%v), muốn 76 (0,76%%)", ratio, ok)
	}
	// 13,2 triệu / 70 triệu — the figure §6 prints as 18,9%.
	if ratio, ok := communeBudget.DisbursedOfAllocatedRatio(); !ok || ratio != 1885 {
		t.Fatalf("giải ngân/đã phân bổ = %d phần vạn (ok=%v), muốn 1885 (§6: 18,9%%)", ratio, ok)
	}
	// 13,2 triệu / 9,2 tỷ — the figure §6 prints as 0,1%.
	if ratio, ok := communeBudget.DisbursedOfTotalRatio(); !ok || ratio != 14 {
		t.Fatalf("giải ngân/tổng nguồn = %d phần vạn (ok=%v), muốn 14 (§6: 0,1%%)", ratio, ok)
	}
}

func TestFundingSourceProgressUnallocatedHasNoMiddleBarRatio(t *testing.T) {
	// §6's third sample row: "Chương trình mục tiêu quốc gia, 10,4 tỷ, đã phân bổ 0 đ, 0 dự án".
	//
	// THE MIDDLE BAR HAS NO DENOMINATOR AND THEREFORE NO VALUE. `0%` and "không tính được" are
	// different statements: printing 0% would report a source the commune has not committed
	// anywhere as its worst-performing one. The other two bars still compute — their denominator is
	// the ceiling, which exists.
	unallocatedSource := FundingSourceProgress{
		FundingSource: FundingSource{ID: "nv-ctmt", Name: "Chương trình mục tiêu quốc gia", Year: 2026,
			TotalAmount: 10_400_000_000},
	}
	if _, ok := unallocatedSource.DisbursedOfAllocatedRatio(); ok {
		t.Fatal("chưa phân bổ đồng nào mà vẫn trả về một tỷ lệ — màn hình sẽ vẽ 0%")
	}
	if ratio, ok := unallocatedSource.AllocatedRatio(); !ok || ratio != 0 {
		t.Fatalf("đã phân bổ/tổng nguồn = %d (ok=%v), muốn 0 và tính được", ratio, ok)
	}
	if ratio, ok := unallocatedSource.DisbursedOfTotalRatio(); !ok || ratio != 0 {
		t.Fatalf("giải ngân/tổng nguồn = %d (ok=%v), muốn 0 và tính được", ratio, ok)
	}
}

func TestFundingSourceProgressOverAllocationIsNotClamped(t *testing.T) {
	// Over-allocation is a real state a commune can type itself into, and it is exactly the state
	// somebody has to resolve. Clamping the remainder to zero would hide it on every screen while
	// the rows underneath stayed wrong — the same reason §13 rule 2 refuses to cap the disbursement
	// ratio at 100%.
	overAllocatedSource := FundingSourceProgress{
		FundingSource:   FundingSource{ID: "nv-xhh", Name: "Nguồn xã hội hoá", Year: 2026, TotalAmount: 1_000_000},
		AllocatedAmount: 1_500_000,
	}
	if unallocated := overAllocatedSource.UnallocatedAmount(); unallocated != -500_000 {
		t.Fatalf("còn chưa phân bổ = %d, muốn -500000 — kẹp về 0 là giấu mất khoản vượt", unallocated)
	}
	if ratio, ok := overAllocatedSource.AllocatedRatio(); !ok || ratio != 15000 {
		t.Fatalf("đã phân bổ/tổng nguồn = %d phần vạn (ok=%v), muốn 15000 (150%%)", ratio, ok)
	}
}

func TestFundingSourceProgressZeroTotalHasNoRatio(t *testing.T) {
	// A source entered before its ceiling is decided. Both bars measured against the ceiling must
	// say "không tính được"; neither may return a plausible 0%.
	overAllocatedSource := FundingSourceProgress{
		FundingSource:   FundingSource{ID: "nv-moi", Name: "Nguồn mới", Year: 2026},
		AllocatedAmount: 5_000_000,
		DisbursedAmount: 1_000_000,
	}
	if _, ok := overAllocatedSource.AllocatedRatio(); ok {
		t.Error("tổng nguồn = 0 mà vẫn trả tỷ lệ đã phân bổ")
	}
	if _, ok := overAllocatedSource.DisbursedOfTotalRatio(); ok {
		t.Error("tổng nguồn = 0 mà vẫn trả tỷ lệ giải ngân trên tổng nguồn")
	}
	// The middle bar DOES have a denominator here, and it is the one figure that can still be
	// stated: 1 triệu / 5 triệu.
	if ratio, ok := overAllocatedSource.DisbursedOfAllocatedRatio(); !ok || ratio != 2000 {
		t.Fatalf("giải ngân/đã phân bổ = %d phần vạn (ok=%v), muốn 2000 (20%%)", ratio, ok)
	}
}
