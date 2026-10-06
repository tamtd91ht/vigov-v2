package domain

import (
	"errors"
	"math"
	"testing"
)

func TestFundingStatusCarriesShortfall(t *testing.T) {
	for _, tc := range []struct {
		name      string
		plan      Dong
		lines     []ProjectAllocation
		status    TrangThaiGanNguon
		count     int
		allocated Dong
		shortfall Dong
	}{
		{"no line", 100, nil, ChuaGanNguon, 0, 0, 100},
		{"short", 100, []ProjectAllocation{{FundingSourceID: "a", Amount: 30}, {FundingSourceID: "b", Amount: 20}},
			ChuaDuNguon, 2, 50, 50},
		{"full", 100, []ProjectAllocation{{FundingSourceID: "a", Amount: 100}}, DuNguon, 1, 100, 0},
		// A legacy row allocated above its plan reads as full with no NEGATIVE shortfall.
		{"legacy over plan", 100, []ProjectAllocation{{FundingSourceID: "a", Amount: 150}}, DuNguon, 1, 150, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := FundingStatusOfAllocations(tc.plan, tc.lines)
			if got.TrangThai != tc.status || got.SoNguon != tc.count || got.TongPhanBo != tc.allocated ||
				got.Shortfall != tc.shortfall {
				t.Errorf("got %+v, want status=%s count=%d allocated=%d shortfall=%d",
					got, tc.status, tc.count, tc.allocated, tc.shortfall)
			}
		})
	}
}

func TestCheckAllocationWithinPlan(t *testing.T) {
	if err := CheckAllocationWithinPlan(100, 100); err != nil {
		t.Errorf("equal to plan refused: %v", err)
	}
	if err := CheckAllocationWithinPlan(100, 0); err != nil {
		t.Errorf("under-allocation refused: %v", err)
	}
	err := CheckAllocationWithinPlan(100_000_000, 120_500_000)
	var over *AllocationExceedsPlanError
	if !errors.As(err, &over) || !errors.Is(err, ErrAllocationExceedsPlan) {
		t.Fatalf("err = %v, want AllocationExceedsPlanError", err)
	}
	if over.Overrun() != 20_500_000 {
		t.Errorf("overrun = %d", over.Overrun())
	}
	want := "`funding_allocations`: tổng các nguồn vốn (120.500.000 đ) vượt kế hoạch vốn năm của dự án " +
		"(100.000.000 đ) 20.500.000 đ. Hãy giảm số phân bổ hoặc tăng kế hoạch vốn năm trước."
	if over.Sentence() != want {
		t.Errorf("sentence = %q\nwant       %q", over.Sentence(), want)
	}
}

// 200 lines of SoTienToiDa overflow int64. A wrapped (negative) total would PASS the plan check.
func TestSumAllocationsSaturatesInsteadOfWrapping(t *testing.T) {
	amounts := make([]Dong, 200)
	for i := range amounts {
		amounts[i] = SoTienToiDa
	}
	if got := SumAllocations(amounts...); got != Dong(math.MaxInt64) {
		t.Fatalf("sum = %d, want saturation at MaxInt64", got)
	}
	if CheckAllocationWithinPlan(SoTienToiDa, SumAllocations(amounts...)) == nil {
		t.Fatal("overflowing total passed the plan check")
	}
}

func TestFormatDong(t *testing.T) {
	for in, want := range map[Dong]string{0: "0 đ", 999: "999 đ", 1000: "1.000 đ", 1234567: "1.234.567 đ", -5000: "-5.000 đ"} {
		if got := FormatDong(in); got != want {
			t.Errorf("FormatDong(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestPlanAllocationReplacement(t *testing.T) {
	stored := []StoredAllocationLine{
		{PhanBoNguonVon: PhanBoNguonVon{ID: "1", NguonVonID: "keep-same", SoTien: 10}},
		{PhanBoNguonVon: PhanBoNguonVon{ID: "2", NguonVonID: "keep-move", SoTien: 10}},
		{PhanBoNguonVon: PhanBoNguonVon{ID: "3", NguonVonID: "drop", SoTien: 10}},
		{PhanBoNguonVon: PhanBoNguonVon{ID: "4", NguonVonID: "revive", SoTien: 10}, Removed: true},
		{PhanBoNguonVon: PhanBoNguonVon{ID: "5", NguonVonID: "stay-removed", SoTien: 10}, Removed: true},
	}
	r := PlanAllocationReplacement(stored, []DongPhanBoMoi{
		{NguonVonID: "keep-same", SoTien: 10},
		{NguonVonID: "keep-move", SoTien: 25},
		{NguonVonID: "revive", SoTien: 7},
		{NguonVonID: "new", SoTien: 3},
	})
	if len(r.Update) != 1 || r.Update[0].ID != "2" || r.Update[0].SoTien != 25 {
		t.Errorf("Update = %+v", r.Update)
	}
	if len(r.Revive) != 1 || r.Revive[0].ID != "4" || r.Revive[0].SoTien != 7 {
		t.Errorf("Revive = %+v", r.Revive)
	}
	if len(r.Insert) != 1 || r.Insert[0].NguonVonID != "new" {
		t.Errorf("Insert = %+v", r.Insert)
	}
	// A removed row the request does not name stays removed — it is not "removed again".
	if len(r.Remove) != 1 || r.Remove[0].ID != "3" {
		t.Errorf("Remove = %+v", r.Remove)
	}
	if PlanAllocationReplacement(stored, []DongPhanBoMoi{
		{NguonVonID: "keep-move", SoTien: 10}, {NguonVonID: "drop", SoTien: 10}, {NguonVonID: "keep-same", SoTien: 10},
	}).Empty() != true {
		t.Error("the live set in another order is not a no-op")
	}
}

func TestProjectAllocationDisbursedRatio(t *testing.T) {
	if _, ok := (ProjectAllocation{Amount: 0, Disbursed: 5}).DisbursedRatio(); ok {
		t.Error("a ratio over a zero allocation must have no value")
	}
	if v, ok := (ProjectAllocation{Amount: 200, Disbursed: 250}).DisbursedRatio(); !ok || v != 12500 {
		t.Errorf("ratio = %d,%v, want 12500 (not clamped)", v, ok)
	}
}
