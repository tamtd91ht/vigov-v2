package http

// Funding allocations on the investment project routes (user decisions 06/10/2026, following the
// prototype): the chip and source names on the list, the per-source lines on the detail, the PATCH
// body's replacement set, and the three refusals as 409s a client can act on.

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// withAllocations gives commune A's da-001 (plan 100.000.000) two lines, and commune B's da-001 —
// THE SAME ID — one line of its own. A read that keyed lines by project id alone would hand A's
// sources to B.
func withAllocations(d *duAnGia) {
	d.allocations = map[tenant.ID]map[string][]domain.ProjectAllocation{
		xaA: {"da-001": {
			{FundingSourceID: "nv-xa", SourceName: "Ngân sách xã", Amount: 40_000_000, Disbursed: 50_000_000},
			{FundingSourceID: "nv-tinh", SourceName: "Ngân sách tỉnh", Amount: 30_000_000, Disbursed: 0},
		}},
		xaB: {"da-001": {
			{FundingSourceID: "nv-b", SourceName: "Nguồn của xã B", Amount: 9_000_000_000},
		}},
	}
}

func TestProjectListCarriesFundingChipFromOneRead(t *testing.T) {
	m := dungMayChuVoi(t, coQuyen("budget.read"))
	withAllocations(m.duAn)

	w := m.goi(t, http.MethodGet, hostA, duongDanDuAn, canBoCua(xaA))
	doiMa(t, w, http.StatusOK)
	var ra danhSachDuAnRa
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatal(err)
	}
	// ONE allocation read for the whole page, never one per project (load-data-once).
	if m.duAn.allocationReads != 1 {
		t.Errorf("allocation reads = %d, want 1 for the whole page", m.duAn.allocationReads)
	}
	byCode := map[string]duAnRa{}
	for _, it := range ra.Items {
		byCode[it.Code] = it
	}

	short := byCode["DA-2026-be-tong-hoa-duong-ngo-xo-2"]
	if short.FundingStatus == nil {
		t.Fatal("funding_status missing")
	}
	want := fundingStatusOut{Status: "chua-du", SourceCount: 2, AllocatedTotal: 70_000_000, ShortfallAmount: 30_000_000}
	if *short.FundingStatus != want {
		t.Errorf("funding_status = %+v, want %+v", *short.FundingStatus, want)
	}
	if strings.Join(short.FundingSourceNames, "|") != "Ngân sách xã|Ngân sách tỉnh" {
		t.Errorf("funding_source_names = %v, want the store's order", short.FundingSourceNames)
	}
	// LIST ITEMS DO NOT CARRY THE DETAIL'S LINES.
	if short.FundingAllocations != nil || short.UnallocatedPlanAmount != nil {
		t.Error("list item carries detail-only fields")
	}

	none := byCode["DA-2026-nong-thon-moi"]
	if none.FundingStatus == nil || none.FundingStatus.Status != "chua-gan-nguon" ||
		none.FundingStatus.ShortfallAmount != 25_000_000_000 {
		t.Errorf("project with no line: funding_status = %+v", none.FundingStatus)
	}
	if len(none.FundingSourceNames) != 0 {
		t.Errorf("project with no line lists sources %v", none.FundingSourceNames)
	}
}

func TestProjectDetailCarriesAllocationLines(t *testing.T) {
	m := dungMayChuVoi(t, coQuyen("budget.read"))
	withAllocations(m.duAn)

	w := m.goi(t, http.MethodGet, hostA, "/api/v1/investment-projects/da-001", canBoCua(xaA))
	doiMa(t, w, http.StatusOK)
	var ra duAnRa
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatal(err)
	}
	if len(ra.FundingAllocations) != 2 {
		t.Fatalf("funding_allocations = %+v, want 2 lines", ra.FundingAllocations)
	}
	first := ra.FundingAllocations[0]
	if first.FundingSourceID != "nv-xa" || first.Name != "Ngân sách xã" || first.Amount != 40_000_000 ||
		first.DisbursedAmount != 50_000_000 || first.DisbursedRatio == nil || *first.DisbursedRatio != 12500 {
		t.Errorf("first line = %+v (ratio %v), want nv-xa 40M/50M at 12500, not clamped", first, first.DisbursedRatio)
	}
	if second := ra.FundingAllocations[1]; second.DisbursedRatio == nil || *second.DisbursedRatio != 0 {
		t.Errorf("second line ratio = %v, want 0 (allocated, nothing paid)", second.DisbursedRatio)
	}
	if ra.UnallocatedPlanAmount == nil || *ra.UnallocatedPlanAmount != 30_000_000 {
		t.Errorf("unallocated_plan_amount = %v, want 30000000", ra.UnallocatedPlanAmount)
	}
	if ra.FundingStatus == nil || ra.FundingStatus.Status != "chua-du" {
		t.Errorf("funding_status = %+v", ra.FundingStatus)
	}
}

// A PRESENT ZERO, not an absent field: "nothing left unallocated" is a fact the screen reads.
func TestProjectDetailUnallocatedZeroIsPresent(t *testing.T) {
	m := dungMayChuVoi(t, coQuyen("budget.read"))
	m.duAn.allocations = map[tenant.ID]map[string][]domain.ProjectAllocation{
		xaA: {"da-001": {{FundingSourceID: "nv-xa", SourceName: "Ngân sách xã", Amount: 100_000_000}}},
	}
	w := m.goi(t, http.MethodGet, hostA, "/api/v1/investment-projects/da-001", canBoCua(xaA))
	doiMa(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"unallocated_plan_amount":0`) {
		t.Errorf("body lacks unallocated_plan_amount:0: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"status":"du"`) {
		t.Errorf("fully allocated project not `du`: %s", w.Body.String())
	}
}

// Rule 1: commune B's project shares commune A's id. B's caller sees B's source, never A's.
func TestProjectAllocationReadsStayInsideTheCommune(t *testing.T) {
	for _, path := range []string{duongDanDuAn, "/api/v1/investment-projects/da-001"} {
		m := dungMayChuVoi(t, coQuyen("budget.read"))
		withAllocations(m.duAn)
		w := m.goi(t, http.MethodGet, hostB, path, canBoCua(xaB))
		doiMa(t, w, http.StatusOK)
		body := w.Body.String()
		if strings.Contains(body, "Ngân sách xã") || strings.Contains(body, "nv-tinh") {
			t.Errorf("%s: commune B's reply carries commune A's sources: %s", path, body)
		}
		if !strings.Contains(body, "Nguồn của xã B") {
			t.Errorf("%s: commune B's own source missing: %s", path, body)
		}
	}
}

// A failed allocation read is a 500 — never a page of "Chưa gắn nguồn" chips that reports every
// project as unfunded.
func TestProjectReadsRefuseWhenAllocationsCannotBeRead(t *testing.T) {
	for _, path := range []string{duongDanDuAn, "/api/v1/investment-projects/da-001"} {
		m := dungMayChuVoi(t, coQuyen("budget.read"))
		m.duAn.allocationErr = errors.New("kết nối kho hỏng")
		w := m.goi(t, http.MethodGet, hostA, path, canBoCua(xaA))
		doiMa(t, w, http.StatusInternalServerError)
		if strings.Contains(w.Body.String(), "kết nối kho") {
			t.Errorf("%s: store failure leaked to the client", path)
		}
	}
}

// --- PATCH funding_allocations --------------------------------------------------------------------

func TestPatchFundingAllocationsReachesUseCase(t *testing.T) {
	for _, tc := range []struct {
		name  string
		body  string
		isNil bool
		lines int
	}{
		{"absent leaves the lines alone", `{"name":"Tên mới"}`, true, 0},
		{"null leaves the lines alone", `{"funding_allocations":null}`, true, 0},
		{"empty removes every line", `{"funding_allocations":[]}`, false, 0},
		{"lines replace the set", `{"funding_allocations":[{"funding_source_id":"nv-xa","amount":40000000},` +
			`{"funding_source_id":"nv-tinh","amount":60000000}]}`, false, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := dungMayChuDuAnGhi(t)
			m.capQuyen(xaA, "budget.update")
			w := m.goi(t, http.MethodPatch, hostA, duongDuAnMot(), canBoGhi(xaA), tc.body)
			doiMa(t, w, http.StatusOK)
			got := m.ghi.suaCuoi.PhanBo
			if (got == nil) != tc.isNil {
				t.Fatalf("PhanBo nil = %v, want %v", got == nil, tc.isNil)
			}
			if got != nil && len(*got) != tc.lines {
				t.Errorf("lines = %d, want %d", len(*got), tc.lines)
			}
		})
	}
}

// The PATCH reply echoes the lines the project draws on after the edit, with their total.
func TestPatchReplyCarriesAllocationsAfterEdit(t *testing.T) {
	m := dungMayChuDuAnGhi(t)
	m.capQuyen(xaA, "budget.update")
	m.ghi.phanBo = []domain.PhanBoNguonVon{
		{ID: "pb-1", NguonVonID: "nv-xa", SoTien: 40_000_000},
		{ID: "pb-2", NguonVonID: "nv-tinh", SoTien: 30_000_000},
	}
	w := m.goi(t, http.MethodPatch, hostA, duongDuAnMot(), canBoGhi(xaA), thanSuaDA)
	doiMa(t, w, http.StatusOK)
	var ra duAnGhiRa
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatal(err)
	}
	if len(ra.FundingAllocations) != 2 || ra.FundingAllocatedTotal != 70_000_000 {
		t.Errorf("reply lines=%d total=%d, want 2 / 70000000", len(ra.FundingAllocations), ra.FundingAllocatedTotal)
	}
}

// The three allocation refusals, on both write routes, as 409 with their own codes. On the edit
// path the use case wraps them with the commune id (bocDuAn) — the sentence must never carry it.
func TestAllocationRefusalsAre409WithTheirCodes(t *testing.T) {
	wrap := func(err error) error { return fmt.Errorf("du_an: sửa cho xã %s: %w", xaA, err) }
	over := &domain.AllocationExceedsPlanError{Plan: 100_000_000, Allocated: 120_000_000}
	for _, tc := range []struct {
		name, method, path, body, code, mustSay string
		err                                     error
	}{
		{"create over plan", http.MethodPost, duongDuAn, thanThemDA, "allocation_exceeds_plan", "20.000.000 đ", over},
		{"edit over plan", http.MethodPatch, duongDuAnMot(), thanSuaDA, "allocation_exceeds_plan", "20.000.000 đ", wrap(over)},
		{"create duplicate source", http.MethodPost, duongDuAn, thanThemDA, "duplicate_source", "`funding_allocations`", domain.ErrPhanBoTrungNguon},
		{"edit duplicate source", http.MethodPatch, duongDuAnMot(), thanSuaDA, "duplicate_source", "`funding_allocations`", domain.ErrPhanBoTrungNguon},
		{"edit drops a source with vouchers", http.MethodPatch, duongDuAnMot(), thanSuaDA, "source_has_disbursements",
			"chứng từ", wrap(domain.ErrSourceHasDisbursements)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := dungMayChuDuAnGhi(t)
			m.capQuyen(xaA, "budget.update")
			m.ghi.loi = tc.err
			w := m.goi(t, tc.method, hostA, tc.path, canBoGhi(xaA), tc.body)
			doiMa(t, w, http.StatusConflict)
			e := loiTra(t, w)
			if e.Code != tc.code {
				t.Errorf("code = %q, want %q", e.Code, tc.code)
			}
			if !strings.Contains(e.Message, tc.mustSay) {
				t.Errorf("message = %q, want it to say %q", e.Message, tc.mustSay)
			}
			if strings.Contains(e.Message, "du_an:") || strings.Contains(e.Message, string(xaA)) {
				t.Errorf("message = %q leaks technical text or the commune id", e.Message)
			}
		})
	}
}
