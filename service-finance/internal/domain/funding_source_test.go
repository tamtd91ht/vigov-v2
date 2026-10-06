package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestNormaliseFundingSourceNameTrimsAndRefuses(t *testing.T) {
	// TRIMMED, because 0013's CHECK refuses an untrimmed name and a trailing space would be a second,
	// invisible copy of a name the unique key compares exactly.
	got, err := NormaliseFundingSourceName("  Ngân sách xã, phường \t")
	if err != nil || got != "Ngân sách xã, phường" {
		t.Fatalf("= %q, %v — muốn tên đã cắt khoảng trắng", got, err)
	}
	// Case is kept: folding it would store a name different from the commune's decision.
	if got, _ := NormaliseFundingSourceName("NGUỒN Xã hội hoá"); got != "NGUỒN Xã hội hoá" {
		t.Fatalf("chữ hoa bị đổi: %q", got)
	}

	for ten, c := range map[string]struct {
		in   string
		want error
	}{
		"rỗng":            {"", ErrFundingSourceNameMissing},
		"toàn khoảng":     {"   ", ErrFundingSourceNameMissing},
		"quá dài":         {strings.Repeat("ệ", FundingSourceNameMax+1), ErrFundingSourceNameTooLong},
		"xuống dòng giữa": {"Ngân sách\nxã", ErrFundingSourceNameInvalid},
	} {
		t.Run(ten, func(t *testing.T) {
			if _, err := NormaliseFundingSourceName(c.in); !errors.Is(err, c.want) {
				t.Fatalf("lỗi = %v, muốn %v", err, c.want)
			}
		})
	}
	// Exactly the bound is admitted — counted in characters, not bytes (Vietnamese is multi-byte).
	if _, err := NormaliseFundingSourceName(strings.Repeat("ệ", FundingSourceNameMax)); err != nil {
		t.Fatalf("tên đúng %d ký tự bị từ chối: %v", FundingSourceNameMax, err)
	}
}

func TestCheckFundingSourceYearWindow(t *testing.T) {
	for year, want := range map[int]error{
		0: ErrFundingSourceYearMissing, 1999: ErrFundingSourceYearOutOfRange,
		2101: ErrFundingSourceYearOutOfRange, 2000: nil, 2026: nil, 2100: nil,
	} {
		if err := CheckFundingSourceYear(year); !errors.Is(err, want) {
			t.Errorf("năm %d: lỗi = %v, muốn %v", year, err, want)
		}
	}
}

func TestCheckGrantedAmount(t *testing.T) {
	for amount, want := range map[Dong]error{
		-1: ErrGrantedAmountNegative, 0: nil, 9_200_000_000: nil,
		SoTienToiDa: nil, SoTienToiDa + 1: ErrGrantedAmountTooLarge,
	} {
		if err := CheckGrantedAmount(amount); !errors.Is(err, want) {
			t.Errorf("%d: lỗi = %v, muốn %v", amount, err, want)
		}
	}
}

func TestUnallocatedAndOverallocatedNeverNegative(t *testing.T) {
	the := func(granted, allocated Dong) TienDoNguonVon {
		return TienDoNguonVon{NguonVon: NguonVon{TongNguon: granted}, DaPhanBo: allocated}
	}
	for ten, c := range map[string]struct {
		card        TienDoNguonVon
		under, over Dong
	}{
		// §6's first card: 9,2 tỷ granted, 70 triệu allocated -> "còn 9,13 tỷ chưa phân bổ".
		"còn chưa phân bổ": {the(9_200_000_000, 70_000_000), 9_130_000_000, 0},
		"vừa đủ":           {the(100, 100), 0, 0},
		// Over-committed: never "còn 0 đ" alone — the overrun is its own figure.
		"phân bổ vượt": {the(1_000, 5_800), 0, 4_800},
		// Nothing granted for the year (no row = 0): NOT reported as an overrun (user decision).
		"chưa nhập vốn giao": {the(0, 70_000_000), 0, 0},
	} {
		t.Run(ten, func(t *testing.T) {
			if u := c.card.UnallocatedAmount(); u != c.under {
				t.Errorf("chưa phân bổ = %d, muốn %d", u, c.under)
			}
			if o := c.card.OverallocatedAmount(); o != c.over {
				t.Errorf("phân bổ vượt = %d, muốn %d", o, c.over)
			}
		})
	}
}

func TestFundingSourceProjectRatioNotClampedAndAbsentWithoutAllocation(t *testing.T) {
	p := FundingSourceProject{AllocatedAmount: 20_000_000, DisbursedAmount: 35_300_000}
	if r, ok := p.DisbursedRatio(); !ok || r != 17650 {
		t.Fatalf("tỷ lệ = %d, %v — muốn 17650 (176,5%%), không cắt về 100%%", r, ok)
	}
	if _, ok := (FundingSourceProject{AllocatedAmount: 0, DisbursedAmount: 5}).DisbursedRatio(); ok {
		t.Fatal("dòng phân bổ 0 đồng mà vẫn có tỷ lệ — phải là \"không có giá trị\", không phải 0%")
	}
}
