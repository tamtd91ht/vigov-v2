package domain

// The WRITE half of the funding source catalogue (§6 "Quản lý nguồn vốn") and the per-source project
// breakdown behind §6's card (prototype SourceItemsDialog). Migration 0013 settled the shape; the
// user decisions of 06/10/2026 settled the rules this file enforces:
//
//	a source NAME is unique within a commune      checked in the store (it counts soft-deleted rows)
//	a source has NO remove and NO rename          there is no function here that could do either
//	the granted amount is PER YEAR, optional on   FundingSourceAnnualAmount; "blank" = no row = 0
//	  create
//
// Nothing here is personal data: a source is how a public authority labels public money.

import (
	"errors"
	"fmt"
	"strings"
)

// FundingSourceNameMax bounds a source name. §6's longest sample is "Chương trình mục tiêu quốc gia"
// (30 characters); the bound is not a format, it is the point past which a name box holds a pasted
// document.
const FundingSourceNameMax = 255

var (
	ErrFundingSourceNameMissing = errors.New("nguon_von: thiếu `name`")

	// The bound is IN THE SENTINEL'S OWN TEXT, not added by a wrapping %w: the handler answers with
	// the sentinel's text and never with the wrapped chain, which on the transaction path carries the
	// commune id (app.wrapFundingSource).
	ErrFundingSourceNameTooLong = fmt.Errorf("nguon_von: `name` quá dài (tối đa %d ký tự)", FundingSourceNameMax)

	// A control character makes a second, invisible spelling of a name the unique key compares
	// EXACTLY (0013's header) — and corrupts a screen, an export and a log line alike.
	ErrFundingSourceNameInvalid = errors.New("nguon_von: `name` chứa ký tự không hợp lệ (xuống dòng, tab…)")

	ErrFundingSourceYearMissing    = errors.New("nguon_von: thiếu `year`")
	ErrFundingSourceYearOutOfRange = errors.New("nguon_von: `year` ngoài khoảng năm hợp lệ (2000..2100)")

	// ErrGrantedAmountMissing — PUT .../annual-amounts/{year} without `granted_amount`. Refused rather
	// than read as 0: "the client forgot the field" and "the commune was granted nothing" are two
	// different statements, and only the second may land in a figure reported upward.
	ErrGrantedAmountMissing = errors.New("nguon_von: thiếu `granted_amount`")

	// Zero is a real state (the source is known before its figure is decided — 0013's CHECK); negative
	// is a sign error that would drag the year's capital below the truth.
	ErrGrantedAmountNegative = errors.New("nguon_von: `granted_amount` không được âm")
	ErrGrantedAmountTooLarge = fmt.Errorf("nguon_von: `granted_amount` vượt mức có thể có (tối đa %d đồng)", int64(SoTienToiDa))
)

// NormaliseFundingSourceName trims and validates a source name typed by a member of staff.
//
// TRIMMED HERE, because 0013's `nguon_von_name_trimmed` refuses an untrimmed name at the floor: a
// trailing space would otherwise be a second, invisible copy of a name the unique key compares exactly.
// Case is NOT folded — "Ngân sách xã" and "ngân sách xã" are two names to the key (0013's header), and
// folding here would store a name that differs from the one on the commune's decision.
func NormaliseFundingSourceName(s string) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", ErrFundingSourceNameMissing
	case len([]rune(s)) > FundingSourceNameMax:
		return "", ErrFundingSourceNameTooLong
	case coKyTuDieuKhien(s):
		return "", ErrFundingSourceNameInvalid
	}
	return s, nil
}

// CheckFundingSourceYear bounds the budget year a granted amount is recorded for — the window of
// 0013's `funding_source_annual_amounts_year_valid`, which is du_an.nam's (0004).
func CheckFundingSourceYear(year int) error {
	switch {
	case year == 0:
		return ErrFundingSourceYearMissing
	case year < NamDuAnSom || year > NamDuAnMuon:
		return ErrFundingSourceYearOutOfRange
	}
	return nil
}

// CheckGrantedAmount refuses a negative or absurd granted amount. The ceiling is SoTienToiDa, the
// same typo guard a voucher, a project plan and an allocation line use: all four are đồng typed by the
// same accountant, and a ceiling that differed would let a figure through one box and not another.
func CheckGrantedAmount(amount Dong) error {
	switch {
	case amount < 0:
		return ErrGrantedAmountNegative
	case amount > SoTienToiDa:
		return ErrGrantedAmountTooLarge
	}
	return nil
}

// FundingSourceAnnualAmount is one row of `funding_source_annual_amounts` (migration 0013): the amount
// granted to one source of one commune for one budget year. A CEILING, not a balance — nothing
// decrements it as money is allocated or spent.
type FundingSourceAnnualAmount struct {
	ID              string // ULID
	FundingSourceID string
	Year            int
	GrantedAmount   Dong
}

// FundingSourceProject is one project of one budget year drawing on one source: THIS SOURCE'S share of
// the project, not the project's whole plan. A project funded from three sources contributes only its
// line from this one, so the rows of the breakdown add up to the card's "đã phân bổ" (prototype
// SourceItemsDialog's own rule).
type FundingSourceProject struct {
	ProjectID     string
	Code          string // `du_an.ma`
	Name          string
	PlannedAmount Dong // the project's whole year plan, for context

	// AllocatedAmount is this source's allocation line on the project (`phan_bo_nguon_von`).
	AllocatedAmount Dong

	// DisbursedAmount is the project's LIVE vouchers drawn FROM THIS SOURCE, every state — the same
	// rule as the card's "đã giải ngân" (store.tongChungTuTheoNguon).
	DisbursedAmount Dong
}

// DisbursedRatio is disbursed / allocated for this source on this project, in PhanVan.
//
// ok = false WHEN NOTHING IS ALLOCATED (a line of 0 đồng is admitted by 0007's CHECK), for the reason
// TienDoNguonVon's ratios give: "0%" there would report a line nobody funded as the worst performer.
// NOT CLAMPED — above 100% is shown, not hidden (§13 rule 2).
func (p FundingSourceProject) DisbursedRatio() (PhanVan, bool) {
	if p.AllocatedAmount <= 0 {
		return 0, false
	}
	return PhanVan(int64(p.DisbursedAmount) * 10000 / int64(p.AllocatedAmount)), true
}

// FundingSourceProjects is the breakdown of one source's card for one budget year.
type FundingSourceProjects struct {
	// Source is the catalogue row. Its Nam is the year asked; its TongNguon is NOT read (0) — the
	// breakdown is about the projects, and the card already carries the granted figure.
	Source NguonVon

	Projects []FundingSourceProject

	// DisbursedWithoutAllocation is money paid FROM this source, in this year, by projects that hold
	// NO allocation line for it. It is in the card's "đã giải ngân" (vouchers are counted by the
	// source they name) and in no row of Projects (rows are projects WITH a line) — so without this
	// figure the breakdown's disbursed column would not add up to the card, with nothing on the screen
	// saying why.
	DisbursedWithoutAllocation Dong
}

// UnallocatedAmount is "còn … chưa phân bổ" of §6, NEVER NEGATIVE. An over-allocation is reported
// separately by OverallocatedAmount rather than as a negative remainder: a source over-committed by
// 4,8 tỷ read as "còn 0 đ" looks like "fully committed, all is well" (prototype list_sources).
//
// ConChuaPhanBo, the signed figure, stays: nothing is lost — the two clamped halves together say what
// the signed one says.
func (t TienDoNguonVon) UnallocatedAmount() Dong {
	if d := t.NguonVon.TongNguon - t.DaPhanBo; d > 0 {
		return d
	}
	return 0
}

// OverallocatedAmount is how much MORE has been allocated from the source than was granted for the
// year — reported ONLY WHEN SOMETHING WAS GRANTED (user decision 06/10/2026, the prototype's rule).
//
// WHY NOT WHEN THE GRANTED AMOUNT IS 0: a source with no figure entered for the year is "not yet
// entered", not "granted nothing" (0013: blank = 0). Reporting its whole allocation as an overrun would
// flag every source whose figure nobody has typed yet. The allocated figure is still on the card, and
// TyLeDaPhanBo answers "no value" for that card, which is the honest state.
func (t TienDoNguonVon) OverallocatedAmount() Dong {
	if t.NguonVon.TongNguon <= 0 {
		return 0
	}
	if d := t.DaPhanBo - t.NguonVon.TongNguon; d > 0 {
		return d
	}
	return 0
}
