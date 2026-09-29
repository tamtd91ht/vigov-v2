package domain

// The refusals of the investment project write path (docs/ui-ux/06-giai-ngan.md §9, §13).
//
// EVERY CASE HERE IS ABOUT THE SENTENCE, NOT THE ENFORCEMENT. The floor is in migration 0004 and
// 0007 — `UNIQUE (tenant_id, ma)`, `CHECK (ke_hoach_von_nam >= 0)`, `CHECK (so_tien_phan_bo >= 0)` —
// and it holds against every writer. What this package adds is a refusal an accountant in a commune
// can act on, arriving first and in Vietnamese.

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNormalizeInvestmentProjectCode(t *testing.T) {
	for _, tc := range []struct {
		name, in, result string
		wantErr          error
	}{
		// §11's own sample, IN CAPITALS. NormalizeCode in three_tier_catalogue.go admits lower case only,
		// because a catalogue code is a slug this system mints; a PROJECT code is a value the commune
		// types off a paper decision, and folding the case would store something different from it.
		{name: "mã của đặc tả, giữ nguyên chữ hoa",
			in: "DA-2026-be-tong-hoa-duong-ngo-xo-2", result: "DA-2026-be-tong-hoa-duong-ngo-xo-2"},
		// §9's other spelling. Both formats have to pass, because the specification uses both and
		// this package is not the place that decides between them.
		{name: "mã dãy số của §9", in: "DA01", result: "DA01"},
		{name: "cắt khoảng trắng hai đầu", in: "  DA01  ", result: "DA01"},

		// ⚠ THE REFUSAL THAT IS A FINDING, NOT A BUG. §9 offers `☑ Tự sinh mã`; this service does not
		// generate one, because the specification gives two incompatible formats and no scope for the
		// sequence, and a project code is an ISSUED code that rule 7 forbids renumbering.
		{name: "trống thì từ chối, KHÔNG tự sinh", in: "", wantErr: ErrInvestmentProjectCodeMissing},
		{name: "chỉ khoảng trắng cũng là trống", in: "   ", wantErr: ErrInvestmentProjectCodeMissing},

		{name: "gạch nối đầu", in: "-DA01", wantErr: ErrInvestmentProjectCodeInvalidFormat},
		{name: "gạch nối cuối", in: "DA01-", wantErr: ErrInvestmentProjectCodeInvalidFormat},
		// `DA--01` reads as one code and sorts as another. Refused while it is still a typo rather
		// than after it is the code on an archival record.
		{name: "hai gạch nối liền", in: "DA--01", wantErr: ErrInvestmentProjectCodeInvalidFormat},
		{name: "khoảng trắng giữa", in: "DA 01", wantErr: ErrInvestmentProjectCodeInvalidFormat},
		// A code carrying diacritics would be a code nobody can type twice the same way, and it lands
		// in file names and export columns.
		{name: "dấu tiếng Việt", in: "DA-bê-tông", wantErr: ErrInvestmentProjectCodeInvalidFormat},
		{name: "quá dài", in: strings.Repeat("A", InvestmentProjectCodeMax+1), wantErr: ErrInvestmentProjectCodeTooLong},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := NormalizeInvestmentProjectCode(tc.in)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("lỗi = %v, muốn %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("lỗi bất ngờ: %v", err)
			}
			if result != tc.result {
				t.Errorf("= %q, muốn %q", result, tc.result)
			}
		})
	}
}

// Zero is a REAL STATE (a project entered before its allocation is decided, 0004:224-229); negative
// is a sign error that would drag the commune's headline "KẾ HOẠCH VỐN NĂM" below the truth with no
// row looking wrong.
func TestValidatePlannedAmount(t *testing.T) {
	for _, tc := range []struct {
		name    string
		amount  Dong
		wantErr error
	}{
		{name: "không đồng là trạng thái thật", amount: 0},
		{name: "số thường", amount: 100_000_000},
		{name: "âm thì từ chối", amount: -1, wantErr: ErrPlannedAmountNegative},
		// The typo guard, shared with a voucher on purpose: both numbers are đồng typed by the same
		// accountant on the same screen, and a ceiling that differed would let a figure through in one
		// box that is refused in the other.
		{name: "vượt trần chống gõ nhầm", amount: AmountMax + 1, wantErr: ErrPlannedAmountTooLarge},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePlannedAmount(tc.amount)
			if tc.wantErr == nil && err != nil {
				t.Fatalf("lỗi bất ngờ: %v", err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("lỗi = %v, muốn %v", err, tc.wantErr)
			}
		})
	}
}

// §11's "mặc định 31/12". A FUNCTION rather than a database default, because the schema does not
// know the year: a `now()`-derived default would stamp a 2026 project with a 2027 deadline on
// 02/01/2027.
func TestDefaultDisbursementDeadlineIsDec31OfBudgetYear(t *testing.T) {
	got := DefaultDisbursementDeadline(2026)
	want := time.Date(2026, time.December, 31, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("= %v, muốn %v", got, want)
	}
	// UTC BECAUSE THE COLUMN IS A `DATE`. A date has no timezone; taking a caller's location would
	// let a clock in UTC+7 produce 30/12 for the same year.
	if got.Location() != time.UTC {
		t.Errorf("múi giờ = %v, muốn UTC", got.Location())
	}
}

// §9 makes the allocation list optional and §11 names the state it produces (`Chưa gắn nguồn`). A
// nil list is a NORMAL project, not an incomplete one.
func TestNormalizeNewAllocationsNoneDeclaredIsNoError(t *testing.T) {
	result, err := NormalizeNewAllocations(nil)
	if err != nil {
		t.Fatalf("lỗi bất ngờ: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("= %v, muốn rỗng", result)
	}
}

func TestNormalizeNewAllocations(t *testing.T) {
	for _, tc := range []struct {
		name    string
		in      []NewAllocationLine
		wantErr error
	}{
		{
			name: "hai nguồn khác nhau",
			in:   []NewAllocationLine{{FundingSourceID: "nv-xa", Amount: 40}, {FundingSourceID: "nv-tp", Amount: 60}},
		},
		{
			// Zero is admitted for the same reason `CHECK (so_tien_phan_bo >= 0)` admits it (0007): a
			// source attached before its figure is agreed is a real intermediate state a commune types.
			name: "số tiền bằng không là trạng thái thật",
			in:   []NewAllocationLine{{FundingSourceID: "nv-xa", Amount: 0}},
		},
		{
			// ⚠ THIS DOES NOT ANSWER MIGRATION 0007's OPEN QUESTION (b). What is refused is one REQUEST
			// naming a source twice — a malformed body — not the business state "a project holds two
			// lines for one source", which 0007 says outright is the customer's call. No constraint was
			// added to the schema.
			name:    "một nguồn khai hai dòng trong CÙNG một lần tạo",
			in:      []NewAllocationLine{{FundingSourceID: "nv-xa", Amount: 40}, {FundingSourceID: "nv-xa", Amount: 60}},
			wantErr: ErrAllocationDuplicateSource,
		},
		{
			// '' IS NOT "no source" ON THIS TABLE, IT IS A BROKEN REFERENCE — unlike
			// `chung_tu_giai_ngan.nguon_von_id`, where a blank is the meaningful state §13 rule 6
			// defines. `phan_bo_nguon_von.nguon_von_id` is NOT NULL with `CHECK (btrim(...) <> '')`.
			name:    "dòng không nêu nguồn nào",
			in:      []NewAllocationLine{{FundingSourceID: "  ", Amount: 40}},
			wantErr: ErrAllocationFundingSourceMissing,
		},
		{
			name:    "số tiền âm",
			in:      []NewAllocationLine{{FundingSourceID: "nv-xa", Amount: -1}},
			wantErr: ErrAllocationNegative,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NormalizeNewAllocations(tc.in)
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("lỗi bất ngờ: %v", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("lỗi = %v, muốn %v", err, tc.wantErr)
			}
		})
	}
}

// ⚠ THE CASE THAT KEEPS §9's WARNING FROM BECOMING A CONSTRAINT. §9: the system "đối chiếu tổng các
// nguồn với số ấy và CẢNH BÁO khi thiếu hoặc vượt". This function validates lines and MUST NOT look
// at the project's plan at all — it does not even receive it, and that signature is the guarantee.
func TestNormalizeNewAllocationsDoesNotCompareWithPlannedAmount(t *testing.T) {
	// Far above and far below any plausible plan. Both must pass: the comparison belongs to the
	// screen, which draws §9's warning and §11's chip from the two raw figures.
	for _, amount := range []Dong{1, AmountMax} {
		if _, err := NormalizeNewAllocations([]NewAllocationLine{{FundingSourceID: "nv-xa", Amount: amount}}); err != nil {
			t.Fatalf("số tiền %d bị từ chối — §9 là CẢNH BÁO chứ không phải ràng buộc: %v", amount, err)
		}
	}
}

// §13 rule 8 lives in the store (the year is bound into the source lookup); what the domain owns is
// refusing a year that is a typo — one that would make the project invisible on every year-filtered
// screen while it sits in the table looking healthy.
func TestValidateInvestmentProjectYear(t *testing.T) {
	for _, tc := range []struct {
		name    string
		year    int
		wantErr error
	}{
		{name: "năm thường", year: 2026},
		{name: "biên dưới", year: InvestmentProjectYearMin},
		{name: "biên trên", year: InvestmentProjectYearMax},
		{name: "không có năm", year: 0, wantErr: ErrInvestmentProjectYearMissing},
		{name: "gõ nhầm 1026", year: 1026, wantErr: ErrInvestmentProjectYearOutOfRange},
		{name: "gõ nhầm 20226", year: 20226, wantErr: ErrInvestmentProjectYearOutOfRange},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateInvestmentProjectYear(tc.year)
			if tc.wantErr == nil && err != nil {
				t.Fatalf("lỗi bất ngờ: %v", err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("lỗi = %v, muốn %v", err, tc.wantErr)
			}
		})
	}
}

// Rule 7, invariant 1 names `delete_reason` and it is not optional.
func TestNormalizeInvestmentProjectDeleteReasonIsRequired(t *testing.T) {
	if _, err := NormalizeInvestmentProjectDeleteReason("   "); !errors.Is(err, ErrInvestmentProjectDeleteReasonMissing) {
		t.Fatalf("lỗi = %v, muốn ErrThieuLyDoXoaDuAn", err)
	}
	result, err := NormalizeInvestmentProjectDeleteReason("  xã rút khỏi kế hoạch vốn  ")
	if err != nil {
		t.Fatalf("lỗi bất ngờ: %v", err)
	}
	if result != "xã rút khỏi kế hoạch vốn" {
		t.Errorf("= %q, muốn đã cắt khoảng trắng", result)
	}
}

// §9: "Để trống thì lấy bằng số tiền bố trí năm nay." The rule lives in ONE place so three copies of
// a default cannot drift — and the drift would be invisible, because a plausible number still
// appears.
func TestEffectiveApprovedAmountAppliesSection9Rule(t *testing.T) {
	d := InvestmentProject{PlannedAmount: 100_000_000}
	if got := d.EffectiveApprovedAmount(); got != 100_000_000 {
		t.Errorf("để trống: = %d, muốn bằng kế hoạch vốn năm", got)
	}
	d.ApprovedAmount = 250_000_000
	if got := d.EffectiveApprovedAmount(); got != 250_000_000 {
		t.Errorf("có khai: = %d, muốn 250000000", got)
	}
}
