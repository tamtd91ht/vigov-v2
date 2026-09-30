package app

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-finance/internal/domain"
)

// Creating a sheet whose percentage columns name their operands (migration 0011, TASK-02), through the
// REAL store on the fake driver: what reaches `INSERT INTO cot_ngan_sach`, what reaches the audit entry,
// and that every refusal writes nothing and opens no transaction.

func intPtr(i int) *int { return &i }

func numberColumn(name string, order int) NewColumn {
	return NewColumn{CotNganSach: domain.CotNganSach{Ten: name, ThuTu: order, Kieu: domain.CotSo}}
}

func percentColumn(name string, order int, formula string, num, den *int) NewColumn {
	return NewColumn{
		CotNganSach: domain.CotNganSach{Ten: name, ThuTu: order, Kieu: domain.CotPhanTram, CongThuc: formula},
		Operands:    domain.OperandIndexes{Numerator: num, Denominator: den},
	}
}

func sheetRequest(cols ...NewColumn) YeuCauTaoBang {
	return YeuCauTaoBang{Nam: 2027, Loai: domain.BangChi, TieuDe: "BÁO CÁO CHI 2027",
		DonViTinh: "trieu-dong", Cot: cols}
}

// argText reports whether any argument, read as text, contains sub. The audit delta is JSON.
func argText(l lenhGhi, sub string) bool {
	for _, a := range l.args {
		var s string
		switch v := a.(type) {
		case string:
			s = v
		case []byte:
			s = string(v)
		default:
			s = fmt.Sprint(v)
		}
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func TestCreateSheetStoresOperandsAsIdsOfColumnsInTheSameRequest(t *testing.T) {
	k := &khoNSGia{lanKeTiep: 1}
	uc, ctx := dungUseCaseNganSach(t, k)

	// The % column sits BEFORE its numerator: the reference is forward, so ids must be issued for the
	// whole set before any operand is resolved. No formula is sent — the server writes one.
	_, err := uc.TaoBang(ctx, sheetRequest(
		numberColumn("Dự toán năm", 1),
		percentColumn("So sánh TH/DT (%)", 2, "", intPtr(2), intPtr(0)),
		numberColumn("Chi ngân sách", 3),
	), nguoiGhi())
	if err != nil {
		t.Fatalf("TaoBang: %v", err)
	}

	// sinhID: the sheet first, then the columns in request order.
	idPlan, idPercent, idSpent := idMoiNganSach+"-2", idMoiNganSach+"-3", idMoiNganSach+"-4"

	inserts := k.cau("INSERT INTO cot_ngan_sach")
	if len(inserts) != 3 {
		t.Fatalf("%d column inserts, want 3", len(inserts))
	}
	p := inserts[1]
	// $1 tenant · $2 id · $3 bang_id · … · $7 cong_thuc · $8 vai_tro · $9 numerator · $10 denominator
	if p.args[0] != string(xaA) || p.args[1] != idPercent || p.args[2] != idMoiNganSach {
		t.Fatalf("%% column row (tenant, id, sheet) = %v, %v, %v", p.args[0], p.args[1], p.args[2])
	}
	if p.args[8] != idSpent || p.args[9] != idPlan {
		t.Fatalf("operands = (%v, %v), want (%s, %s)", p.args[8], p.args[9], idSpent, idPlan)
	}
	if p.args[6] != "Chi ngân sách / Dự toán năm × 100" {
		t.Fatalf("generated formula = %v", p.args[6])
	}
	// A `so` column's operands go in as NULL, never '' (0011's not-blank CHECK would refuse '').
	for _, i := range []int{0, 2} {
		if inserts[i].args[8] != nil || inserts[i].args[9] != nil {
			t.Fatalf("number column %d operands = (%v, %v), want NULL", i, inserts[i].args[8], inserts[i].args[9])
		}
	}

	// The audit entry records the operands: after TASK-02 this pair is the only statement of which
	// columns were meant (0011's reversal note).
	entries := k.cau("INSERT INTO audit_log")
	if len(entries) != 1 {
		t.Fatalf("%d audit entries, want 1", len(entries))
	}
	for _, want := range []string{`"numerator_column_id":"` + idSpent + `"`,
		`"denominator_column_id":"` + idPlan + `"`} {
		if !argText(entries[0], want) {
			t.Errorf("audit delta lacks %s: %v", want, entries[0].args)
		}
	}
}

func TestCreateSheetKeepsTheClientFormulaAsDisplayText(t *testing.T) {
	k := &khoNSGia{lanKeTiep: 1}
	uc, ctx := dungUseCaseNganSach(t, k)

	if _, err := uc.TaoBang(ctx, sheetRequest(
		numberColumn("Dự toán năm", 1),
		numberColumn("Chi ngân sách", 2),
		percentColumn("So sánh TH/DT (%)", 3, "  TH / DT  ", intPtr(1), intPtr(0)),
	), nguoiGhi()); err != nil {
		t.Fatalf("TaoBang: %v", err)
	}
	if got := k.cau("INSERT INTO cot_ngan_sach")[2].args[6]; got != "TH / DT" {
		t.Fatalf("formula = %v, want the client's text, trimmed", got)
	}
}

func TestCreateSheetRefusesBadOperandsBeforeAnyTransaction(t *testing.T) {
	for _, tc := range []struct {
		name string
		cols []NewColumn
		want error
	}{
		{"numerator is the % column itself", []NewColumn{
			numberColumn("A", 1), percentColumn("P", 2, "", intPtr(1), intPtr(0)),
		}, domain.ErrOperandNotNumberColumn},
		{"operand is another % column", []NewColumn{
			numberColumn("A", 1), numberColumn("B", 2),
			percentColumn("P1", 3, "", intPtr(1), intPtr(0)),
			percentColumn("P2", 4, "", intPtr(2), intPtr(0)),
		}, domain.ErrOperandNotNumberColumn},
		{"same column twice", []NewColumn{
			numberColumn("A", 1), percentColumn("P", 2, "", intPtr(0), intPtr(0)),
		}, domain.ErrOperandsSameColumn},
		{"index past the end", []NewColumn{
			numberColumn("A", 1), percentColumn("P", 2, "", intPtr(5), intPtr(0)),
		}, domain.ErrOperandIndexOutOfRange},
		{"negative index", []NewColumn{
			numberColumn("A", 1), percentColumn("P", 2, "", intPtr(0), intPtr(-1)),
		}, domain.ErrOperandIndexOutOfRange},
		{"only a numerator", []NewColumn{
			numberColumn("A", 1), numberColumn("B", 2), percentColumn("P", 3, "", intPtr(1), nil),
		}, domain.ErrPercentOperandsMissing},
		{"a new % column with no operands", []NewColumn{
			numberColumn("A", 1), percentColumn("P", 2, "col_1 / col_1 * 100", nil, nil),
		}, domain.ErrPercentOperandsMissing},
		{"operands on a number column", []NewColumn{
			numberColumn("A", 1),
			{CotNganSach: domain.CotNganSach{Ten: "B", ThuTu: 2, Kieu: domain.CotSo},
				Operands: domain.OperandIndexes{Numerator: intPtr(0), Denominator: intPtr(0)}},
		}, domain.ErrOperandsOnNumberColumn},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k := &khoNSGia{lanKeTiep: 1}
			uc, ctx := dungUseCaseNganSach(t, k)
			_, err := uc.TaoBang(ctx, sheetRequest(tc.cols...), nguoiGhi())
			if !errors.Is(err, tc.want) {
				t.Fatalf("= %v, want %v", err, tc.want)
			}
			if k.batDau != 0 || len(k.lenh) != 0 {
				t.Fatalf("refused request opened %d transactions and ran %d statements, want 0/0",
					k.batDau, len(k.lenh))
			}
		})
	}
}

func TestCreateSheetIgnoresClientSuppliedIds(t *testing.T) {
	// The embedded column's ID, BangID and operand ids are the system's. A caller that set them — for
	// example to a column of another sheet or another commune — is overridden, so no operand can point
	// outside the sheet being created.
	k := &khoNSGia{lanKeTiep: 1}
	uc, ctx := dungUseCaseNganSach(t, k)

	foreign := percentColumn("P", 3, "", intPtr(1), intPtr(0))
	foreign.ID, foreign.BangID = "column-of-other-commune", "sheet-of-other-commune"
	foreign.NumeratorColumnID, foreign.DenominatorColumnID = "foreign-1", "foreign-2"
	if _, err := uc.TaoBang(ctx, sheetRequest(numberColumn("A", 1), numberColumn("B", 2), foreign),
		nguoiGhi()); err != nil {
		t.Fatalf("TaoBang: %v", err)
	}
	p := k.cau("INSERT INTO cot_ngan_sach")[2]
	for _, bad := range []string{"column-of-other-commune", "sheet-of-other-commune", "foreign-1", "foreign-2"} {
		if coGiaTri(p, bad) {
			t.Fatalf("caller-supplied id %q reached the insert: %v", bad, p.args)
		}
	}
	if p.args[0] != string(xaA) || p.args[8] != idMoiNganSach+"-3" || p.args[9] != idMoiNganSach+"-2" {
		t.Fatalf("tenant/operands = (%v, %v, %v)", p.args[0], p.args[8], p.args[9])
	}
}
