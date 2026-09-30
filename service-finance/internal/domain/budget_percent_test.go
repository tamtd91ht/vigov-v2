package domain

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

// The per-line percentage (BangDayDu.PercentCell), its arithmetic (PercentBasisPoints), and the
// write-side operand checks (AssignOperandsByIndex + the column-set check).

func intRef(i int) *int { return &i }

// percentSheet is bangChiMau with its % column's operands resolved: `Chi ngân sách` / `Dự toán năm`.
func percentSheet() BangDayDu {
	b := bangChiMau()
	b.Cot[2].NumeratorColumnID, b.Cot[2].DenominatorColumnID = "c-chi", "c-dt"
	return b
}

func TestPercentCellIsTheSpecificationsOwnFigure(t *testing.T) {
	// §3.1 prints 91,3% for 3.463.459,2 / 3.794.740,0 on the total row: 9127 phần vạn (91,27%).
	p := percentSheet().PercentCell("k-tong", "c-ty")
	if !p.Co || p.Gia != 9127 {
		t.Fatalf("= %+v, want 9127", p)
	}
	// Same arithmetic as the indicator card, so the two never disagree in the last digit.
	if want := TyLeDong(trieu(chiNganSachSo), trieu(chiDuToanNam)); p.Gia != want {
		t.Fatalf("cell %d vs indicator %d", p.Gia, want)
	}
}

func TestPercentCellOfAParentIsSumOverSum(t *testing.T) {
	b := percentSheet()
	b.KhoanMuc = append(b.KhoanMuc,
		KhoanMucNganSach{ID: "k-a1", BangID: "b-chi", ChaID: "k-a", Ten: "1", ThuTu: 1, CachTinh: TinhTay},
		KhoanMucNganSach{ID: "k-a2", BangID: "b-chi", ChaID: "k-a", Ten: "2", ThuTu: 2, CachTinh: TinhTay})
	b.Gia["k-a1"] = map[string]Dong{"c-dt": 100, "c-chi": 100} // 100%
	b.Gia["k-a2"] = map[string]Dong{"c-dt": 300, "c-chi": 0}   // 0%
	// (100 + 0) / (100 + 300) = 25%, NOT (100% + 0%) / 2 = 50% and NOT the parent's locked figures.
	if p := b.PercentCell("k-a", "c-ty"); !p.Co || p.Gia != 2500 {
		t.Fatalf("parent = %+v, want 2500", p)
	}
}

func TestPercentCellReasons(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(b *BangDayDu)
		want  error
	}{
		{"zero denominator", func(b *BangDayDu) { b.Gia["k-a"]["c-dt"] = 0 }, ErrMauSoBangKhong},
		{"empty denominator", func(b *BangDayDu) { delete(b.Gia["k-a"], "c-dt") }, ErrPercentDenominatorEmpty},
		{"empty numerator", func(b *BangDayDu) { delete(b.Gia["k-a"], "c-chi") }, ErrPercentNumeratorEmpty},
		{"legacy unresolved", func(b *BangDayDu) {
			b.Cot[2].NumeratorColumnID, b.Cot[2].DenominatorColumnID = "", ""
		}, ErrPercentOperandsUnresolved},
		{"operand column gone (soft-deleted)", func(b *BangDayDu) { b.Cot = b.Cot[1:] }, ErrOperandNotInSheet},
		{"operand is a % column", func(b *BangDayDu) { b.Cot[2].DenominatorColumnID = "c-ty" }, ErrOperandNotNumberColumn},
		{"operand figure unavailable", func(b *BangDayDu) { b.Gia["k-a"]["c-chi"] = GiaTriToiDa + 1 }, ErrGiaTriDaLuuVuotMuc},
		{"ratio past the exact range", func(b *BangDayDu) {
			b.Gia["k-a"]["c-chi"], b.Gia["k-a"]["c-dt"] = GiaTriToiDa, 1
		}, ErrPercentOutOfRange},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := percentSheet()
			tc.setup(&b)
			p := b.PercentCell("k-a", "c-ty")
			if p.Co || p.Gia != 0 {
				t.Fatalf("= %+v, want no figure", p)
			}
			if p.LyDo == "" || !strings.Contains(p.LyDo, strings.TrimPrefix(tc.want.Error(), "ngan_sach: ")) {
				t.Fatalf("reason = %q, want %q", p.LyDo, tc.want)
			}
		})
	}
}

func TestPercentCellOnANumberColumnIsRefused(t *testing.T) {
	if p := percentSheet().PercentCell("k-a", "c-dt"); p.Co || p.LyDo == "" {
		t.Fatalf("= %+v, want a reason", p)
	}
}

func TestPercentBasisPoints(t *testing.T) {
	for _, tc := range []struct {
		num, den Dong
		want     PhanVan
	}{
		{1, 2, 5000},
		{2, 3, 6667},    // 66,666…% -> 66,67%
		{1, 3, 3333},    // 33,333…% -> 33,33%
		{1, 8, 1250},    // exact
		{1, 20000, 1},   // 0,5 phần vạn — half rounds AWAY from zero
		{-1, 20000, -1}, // …in both directions
		{1, -20000, -1},
		{-1, -20000, 1},
		{1, 20001, 0}, // just under half
		{-2, 3, -6667},
		{0, 5, 0},
		{0, -5, 0},
		{-50, 200, -2500},
		// At the ceiling: num × 10^4 is ≈ 9 × 10^19, past int64. TyLeDong scales here; this is exact.
		{GiaTriToiDa, GiaTriToiDa, 10_000},
		{GiaTriToiDa, 10_000, PhanVan(GiaTriToiDa)},
		// Where TyLeDong's scaling answers 0 (the scaled denominator becomes 0): exact here.
		{10_000_000_000_000, 500_000, 200_000_000_000},
	} {
		got, err := PercentBasisPoints(tc.num, tc.den)
		if err != nil || got != tc.want {
			t.Errorf("%d / %d = %d, %v — want %d", tc.num, tc.den, got, err, tc.want)
		}
		// Consistent with the indicators wherever TyLeDong does not scale (|num| ≤ 9 × 10^12).
		if tc.num <= 9_000_000_000_000 && tc.num >= -9_000_000_000_000 {
			if ind := TyLeDong(tc.num, tc.den); ind != got {
				t.Errorf("%d / %d: cell %d vs indicator %d — the two roundings drifted", tc.num, tc.den, got, ind)
			}
		}
	}
	if _, err := PercentBasisPoints(5, 0); !errors.Is(err, ErrMauSoBangKhong) {
		t.Errorf("x / 0 = %v, want ErrMauSoBangKhong", err)
	}
	// Past ±(2^53 − 1) phần vạn: unavailable, never clamped.
	for _, tc := range [][2]Dong{{GiaTriToiDa, 1}, {-GiaTriToiDa, 1}, {GiaTriToiDa, 9_999}} {
		if got, err := PercentBasisPoints(tc[0], tc[1]); !errors.Is(err, ErrPercentOutOfRange) {
			t.Errorf("%d / %d = %d, %v — want ErrPercentOutOfRange", tc[0], tc[1], got, err)
		}
	}
}

// --- the write side -----------------------------------------------------------------------------------

func newColumns() []CotNganSach {
	return []CotNganSach{
		{ID: "a", BangID: "s", Ten: "Dự toán năm", Kieu: CotSo},
		{ID: "b", BangID: "s", Ten: "Chi ngân sách", Kieu: CotSo},
		{ID: "p", BangID: "s", Ten: "Tỷ lệ", Kieu: CotPhanTram, CongThuc: "b / a"},
	}
}

func TestAssignOperandsByIndexThenColumnSetCheck(t *testing.T) {
	cols := newColumns()
	if err := AssignOperandsByIndex(cols, []OperandIndexes{{}, {}, {intRef(1), intRef(0)}}); err != nil {
		t.Fatalf("assign: %v", err)
	}
	if cols[2].NumeratorColumnID != "b" || cols[2].DenominatorColumnID != "a" {
		t.Fatalf("operands = %q / %q", cols[2].NumeratorColumnID, cols[2].DenominatorColumnID)
	}
	if err := KiemTraBoCot(BangChi, cols); err != nil {
		t.Fatalf("column-set check: %v", err)
	}
}

func TestColumnSetCheckRefusesOperandsOutsideTheSet(t *testing.T) {
	for _, tc := range []struct {
		name     string
		num, den string
		want     error
	}{
		{"none", "", "", ErrPercentOperandsMissing},
		{"one", "b", "", ErrPercentOperandsMissing},
		{"same", "a", "a", ErrOperandsSameColumn},
		{"itself", "p", "a", ErrOperandNotNumberColumn},
		{"unknown id", "zzz", "a", ErrOperandNotInSheet},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cols := newColumns()
			cols[2].NumeratorColumnID, cols[2].DenominatorColumnID = tc.num, tc.den
			if err := KiemTraBoCot(BangChi, cols); !errors.Is(err, tc.want) {
				t.Fatalf("= %v, want %v", err, tc.want)
			}
		})
	}
	// A column of ANOTHER sheet with a matching id is not an operand.
	cols := newColumns()
	cols[1].BangID = "other-sheet"
	cols[2].NumeratorColumnID, cols[2].DenominatorColumnID = "b", "a"
	if err := KiemTraBoCot(BangChi, cols); !errors.Is(err, ErrOperandNotInSheet) {
		t.Fatalf("other sheet = %v, want ErrOperandNotInSheet", err)
	}
	// A number column carrying operands.
	cols = newColumns()
	cols[2].NumeratorColumnID, cols[2].DenominatorColumnID = "b", "a"
	cols[0].NumeratorColumnID = "b"
	if err := KiemTraBoCot(BangChi, cols); !errors.Is(err, ErrOperandsOnNumberColumn) {
		t.Fatalf("number column = %v, want ErrOperandsOnNumberColumn", err)
	}
}

func TestAssignOperandsByIndexRefusals(t *testing.T) {
	for _, tc := range []struct {
		name string
		refs []OperandIndexes
		want error
	}{
		{"past the end", []OperandIndexes{{}, {}, {intRef(3), intRef(0)}}, ErrOperandIndexOutOfRange},
		{"negative", []OperandIndexes{{}, {}, {intRef(1), intRef(-1)}}, ErrOperandIndexOutOfRange},
		{"only one", []OperandIndexes{{}, {}, {nil, intRef(0)}}, ErrPercentOperandsMissing},
		{"on a number column", []OperandIndexes{{intRef(1), intRef(2)}, {}, {intRef(1), intRef(0)}}, ErrOperandsOnNumberColumn},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := AssignOperandsByIndex(newColumns(), tc.refs); !errors.Is(err, tc.want) {
				t.Fatalf("= %v, want %v", err, tc.want)
			}
		})
	}
}

func TestPercentFormulaFitsTheLimit(t *testing.T) {
	if got := PercentFormula("Chi ngân sách", "Dự toán năm"); got != "Chi ngân sách / Dự toán năm × 100" {
		t.Fatalf("= %q", got)
	}
	long := strings.Repeat("Ư", TenCotToiDa)
	for _, tc := range [][2]string{{long, long}, {long, "Dự toán"}, {"Chi", long}} {
		got := PercentFormula(tc[0], tc[1])
		if n := utf8.RuneCountInString(got); n > CongThucToiDa {
			t.Errorf("%d runes, max %d", n, CongThucToiDa)
		}
		if _, err := ChuanHoaCongThuc(got); err != nil {
			t.Errorf("generated formula refused by its own validator: %v", err)
		}
		if !strings.HasSuffix(got, " × 100") {
			t.Errorf("= %q", got)
		}
	}
	// The short name is kept whole.
	if got := PercentFormula(long, "Dự toán"); !strings.Contains(got, " / Dự toán × 100") {
		t.Errorf("short denominator cut: %q", got)
	}
}
