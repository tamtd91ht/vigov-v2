package http

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-finance/internal/domain"
)

// The percentage columns on the wire (TASK-02): the operand ids on each column, the server-computed
// `percent_basis_points` on each line, the reason beside every null, and the create body's operand
// indexes. No new route and no new permission — the four RBAC cases stay in thu_chi_ngan_sach_test.go.

func TestSheetReadCarriesOperandsAndServerComputedPercent(t *testing.T) {
	m := dungMayChuNganSach(t)
	m.capQuyen(xaA, "budget.read")

	b := bangChiCuaXaA()
	b.Cot[2].NumeratorColumnID, b.Cot[2].DenominatorColumnID = "c-chi", "c-dt"
	b.Gia["k-a"]["c-dt"] = 0 // zero denominator on the second line
	m.doc.theo[xaA][khoaBang(2026, domain.BangChi)] = b

	var ra bangDayDuRa
	docJSON(t, m.goi(t, http.MethodGet, hostA, duongBang+"?year=2026&kind=chi", canBoGhi(xaA), ""), &ra)

	col := ra.Columns[2]
	if col.NumeratorColumnID == nil || *col.NumeratorColumnID != "c-chi" ||
		col.DenominatorColumnID == nil || *col.DenominatorColumnID != "c-dt" {
		t.Fatalf("%% column operands = %v / %v", col.NumeratorColumnID, col.DenominatorColumnID)
	}
	if ra.Columns[0].NumeratorColumnID != nil || ra.Columns[0].DenominatorColumnID != nil {
		t.Fatal("a `so` column must carry null operands")
	}

	lines := map[string]dongRa{}
	for _, d := range ra.Lines {
		lines[d.ID] = d
	}
	// §3.1's own figure: 3.463.459,2 / 3.794.740,0 = 91,27%.
	total := lines[idDongChi]
	if v := total.PercentBasisPoints["c-ty"]; v == nil || *v != 9127 {
		t.Fatalf("total row %% = %v, want 9127", v)
	}
	if _, has := total.UnavailableReasons["c-ty"]; has {
		t.Fatal("a computed percentage must not carry a reason")
	}
	// Still not in `values`: a % column has no stored figure.
	if _, has := total.Values["c-ty"]; has {
		t.Fatal("percentage column leaked into `values`")
	}

	zero := lines["k-a"]
	if v, has := zero.PercentBasisPoints["c-ty"]; !has || v != nil {
		t.Fatalf("zero denominator %% = %v (present %v), want null", v, has)
	}
	if !strings.Contains(zero.UnavailableReasons["c-ty"], "mẫu số bằng 0") {
		t.Fatalf("zero denominator reason = %q", zero.UnavailableReasons["c-ty"])
	}
}

func TestLegacyPercentColumnReadsAsUnresolvedWithNullOperands(t *testing.T) {
	// The fixture's `c-ty` has NO operands — a formula migration 0011 could not resolve.
	m := dungMayChuNganSach(t)
	m.capQuyen(xaA, "budget.read")

	w := m.goi(t, http.MethodGet, hostA, duongBang+"?year=2026&kind=chi", canBoGhi(xaA), "")
	var raw struct {
		Columns []map[string]json.RawMessage `json:"columns"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, f := range []string{"numerator_column_id", "denominator_column_id"} {
		if v, has := raw.Columns[2][f]; !has || string(v) != "null" {
			t.Fatalf("%s = %s (present %v), want an explicit null", f, v, has)
		}
	}

	var ra bangDayDuRa
	docJSON(t, w, &ra)
	for _, d := range ra.Lines {
		if v, has := d.PercentBasisPoints["c-ty"]; !has || v != nil {
			t.Fatalf("line %s legacy %% = %v, want null", d.ID, v)
		}
		if !strings.Contains(d.UnavailableReasons["c-ty"], "không đoán từ công thức") {
			t.Fatalf("line %s reason = %q", d.ID, d.UnavailableReasons["c-ty"])
		}
	}
}

func TestCreateSheetPassesOperandIndexesToTheUseCase(t *testing.T) {
	m := dungMayChuNganSach(t)
	m.capQuyen(xaA, "budget.update")

	than := `{"year":2027,"kind":"chi","title":"BÁO CÁO CHI 2027","unit":"trieu-dong","columns":[` +
		`{"name":"Dự toán năm","order":1,"type":"so","role":"du-toan-nam"},` +
		`{"name":"Chi ngân sách","order":2,"type":"so","role":"chi-ngan-sach"},` +
		`{"name":"So sánh (%)","order":3,"type":"phan_tram","numerator_index":1,"denominator_index":0}]}`
	doiMa(t, m.goi(t, http.MethodPost, hostA, duongBang, canBoGhi(xaA), than), http.StatusCreated)

	cols := m.ghi.taoCuoi.Cot
	if len(cols) != 3 {
		t.Fatalf("%d columns reached the use case, want 3", len(cols))
	}
	op := cols[2].Operands
	if op.Numerator == nil || *op.Numerator != 1 || op.Denominator == nil || *op.Denominator != 0 {
		t.Fatalf("operands = %v / %v, want 1 / 0", op.Numerator, op.Denominator)
	}
	if cols[0].Operands.Numerator != nil || cols[0].Operands.Denominator != nil {
		t.Fatal("a column that sent no indexes must reach the use case with none")
	}
	if m.ghi.xaCuoi != xaA {
		t.Fatalf("use case ran in commune %q, want %q", m.ghi.xaCuoi, xaA)
	}
}

func TestCreateSheetOperandRefusalsAre400(t *testing.T) {
	for _, loi := range []error{
		domain.ErrPercentOperandsMissing, domain.ErrOperandsOnNumberColumn,
		domain.ErrOperandIndexOutOfRange, domain.ErrOperandsSameColumn,
		domain.ErrOperandNotNumberColumn, domain.ErrOperandNotInSheet,
	} {
		t.Run(loi.Error(), func(t *testing.T) {
			m := dungMayChuNganSach(t)
			m.capQuyen(xaA, "budget.update")
			m.ghi.loi = loi
			w := m.goi(t, http.MethodPost, hostA, duongBang, canBoGhi(xaA), thanTaoBang)
			doiMa(t, w, http.StatusBadRequest)
			if e := loiTra(t, w); e.Message != loi.Error() {
				t.Fatalf("message = %q, want the domain's sentence", e.Message)
			}
		})
	}
}
