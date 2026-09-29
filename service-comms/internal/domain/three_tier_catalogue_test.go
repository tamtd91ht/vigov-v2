package domain

import (
	"errors"
	"strings"
	"testing"
)

// The pure rules of a reference catalogue. No database, no HTTP — this package imports nothing but
// the standard library (doc.go), which is exactly what makes these testable at all.

func TestTierOfMatchesThreeTierTable(t *testing.T) {
	// THE TABLE IS COPIED FROM THE MIGRATION, not restated from memory —
	// migrations/0003_danh_muc_loai_tai_nguyen_ban_do.sql:75-77. A tier read wrongly here offers a button the
	// database will refuse, or hides one that would have worked.
	for name, tc := range map[string]struct {
		source   string
		branched bool
		want     Tier
	}{
		"xã tự thêm":          {SourceCommune, false, TierCommune},
		"hệ thống cấp":        {SourceSystem, false, TierSystem},
		"hệ thống + rẽ nhánh": {SourceSystem, true, TierBranched},
		// The schema's CHECK refuses this combination outright (`..._re_nhanh_thi_he_thong`), so it
		// cannot come out of the database. It is asserted anyway, and asserted as the STRICTER
		// reading: if a row ever held it, the safe answer is "do not touch it".
		"đơn vị + rẽ nhánh (lược đồ cấm)": {SourceCommune, true, TierBranched},
		// An unknown `nguon` also cannot come out of the database, and the safe reading here is the
		// OPPOSITE one: treat it as the commune's own row rather than as untouchable, because a row
		// nobody can reach is a row an administrator cannot clean up.
		"nguồn lạ": {"khong-biet", false, TierCommune},
	} {
		t.Run(name, func(t *testing.T) {
			if got := TierOf(tc.source, tc.branched); got != tc.want {
				t.Errorf("TierOf(%q, %v) = %d, muốn %d", tc.source, tc.branched, got, tc.want)
			}
		})
	}
}

func TestAllowSoftDeleteTier1Only(t *testing.T) {
	if err := TierCommune.AllowSoftDelete(); err != nil {
		t.Errorf("tầng 1 phải xoá mềm được: %v", err)
	}
	for _, tier := range []Tier{TierSystem, TierBranched} {
		if err := tier.AllowSoftDelete(); !errors.Is(err, ErrSystemRowNotDeletable) {
			t.Errorf("tầng %d: lỗi = %v, muốn ErrSystemRowNotDeletable", tier, err)
		}
	}
}

func TestAllowDisableEveryTierButTier3(t *testing.T) {
	for _, tier := range []Tier{TierCommune, TierSystem} {
		if err := tier.AllowDisable(); err != nil {
			t.Errorf("tầng %d phải tắt được: %v", tier, err)
		}
	}
	if err := TierBranched.AllowDisable(); !errors.Is(err, ErrBranchedRowNotDisableable) {
		t.Errorf("tầng 3: lỗi = %v, muốn ErrBranchedRowNotDisableable", err)
	}
}

func TestNormalizeCodeAcceptsOnlyKebabWithoutDiacritics(t *testing.T) {
	// ADR 0011: catalogue VALUES stay Vietnamese without diacritics while the surrounding contract
	// is English. The code goes straight into business records as a value nothing ever rewrites, so
	// this is the last moment it can be refused.
	for _, good := range []string{"cong-van", "quyet-dinh", "bao-cao-2026", "a1"} {
		if got, err := NormalizeCode(good); err != nil || got != good {
			t.Errorf("NormalizeCode(%q) = %q, %v — phải nhận", good, got, err)
		}
	}
	// Whitespace is trimmed rather than refused: it is invisible on a form, and a code that differs
	// from another only by a trailing space is the kind of duplicate nobody can see.
	if got, err := NormalizeCode("  cong-van  "); err != nil || got != "cong-van" {
		t.Errorf("NormalizeCode cắt khoảng trắng: %q, %v", got, err)
	}
	for name, bad := range map[string]string{
		"chữ hoa":        "Cong-Van",
		"dấu tiếng Việt": "công-văn",
		"gạch dưới":      "cong_van",
		"khoảng trắng":   "cong van",
		"gạch đầu":       "-cong-van",
		"gạch cuối":      "cong-van-",
		"gạch đôi":       "cong--van",
		"rỗng":           "   ",
		"quá dài":        strings.Repeat("a", CodeMaxLen+1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NormalizeCode(bad); err == nil {
				t.Errorf("NormalizeCode(%q) phải từ chối", bad)
			}
		})
	}
}

func TestNormalizeCodeDoesNotLowerCase(t *testing.T) {
	// NO CASE FOLDING, and it is asserted rather than left to the refusal above: lower-casing what
	// the person typed means the code stored is not the code they saw, on the one column rule 7
	// never lets us correct afterwards.
	if _, err := NormalizeCode("Cong-Van"); !errors.Is(err, ErrCodeShape) {
		t.Errorf("lỗi = %v, muốn ErrCodeShape — không được tự hạ chữ", err)
	}
}

func TestNormalizeLabelKeepsVietnameseDiacritics(t *testing.T) {
	// The label is a sentence a person reads, so it carries diacritics — the opposite rule to the
	// code. It is also the ONE field every tier may change.
	if got, err := NormalizeLabel("  Quyết định  "); err != nil || got != "Quyết định" {
		t.Errorf("NormalizeLabel = %q, %v", got, err)
	}
	if _, err := NormalizeLabel("   "); !errors.Is(err, ErrLabelEmpty) {
		t.Errorf("nhãn rỗng: lỗi = %v, muốn ErrLabelEmpty", err)
	}
	if _, err := NormalizeLabel(strings.Repeat("a", LabelMaxLen+1)); !errors.Is(err, ErrLabelTooLong) {
		t.Error("nhãn quá dài phải bị từ chối")
	}
	// A control character corrupts a screen and a log line alike.
	if _, err := NormalizeLabel("Công\nvăn"); err == nil {
		t.Error("nhãn chứa ký tự điều khiển phải bị từ chối")
	}
	// Counted in RUNES, not bytes: "Quyết định" is 10 characters and 14 bytes, and a byte bound
	// would cut a Vietnamese label off at two thirds of the length an English one gets.
	if _, err := NormalizeLabel(strings.Repeat("ế", LabelMaxLen)); err != nil {
		t.Errorf("nhãn %d ký tự tiếng Việt bị từ chối — giới hạn đang đếm byte: %v", LabelMaxLen, err)
	}
}

func TestValidateSortOrderRefusesNegativeAndTooLarge(t *testing.T) {
	// NEGATIVE IS REFUSED, NOT CLAMPED. A client sending -1 to mean "first" works until a second
	// client sends -2, and the order a commune arranged its own catalogue in then depends on who
	// edited last.
	for _, good := range []int{0, 1, SortOrderMax} {
		if err := ValidateSortOrder(good); err != nil {
			t.Errorf("thứ tự %d phải hợp lệ: %v", good, err)
		}
	}
	for _, bad := range []int{-1, SortOrderMax + 1} {
		if err := ValidateSortOrder(bad); !errors.Is(err, ErrSortOrderOutOfRange) {
			t.Errorf("thứ tự %d: lỗi = %v, muốn ErrSortOrderOutOfRange", bad, err)
		}
	}
}

func TestNormalizeDeleteReasonRequired(t *testing.T) {
	// Rule 7, invariant 1 names `delete_reason` beside `deleted_at` and `deleted_by`. A row that
	// vanished from every screen with no reason attached is a row nobody can explain — and the row
	// is still there, so the question will be asked.
	if got, err := NormalizeDeleteReason("  gộp vào loại khác "); err != nil || got != "gộp vào loại khác" {
		t.Errorf("NormalizeDeleteReason = %q, %v", got, err)
	}
	if _, err := NormalizeDeleteReason(" "); !errors.Is(err, ErrDeleteReasonMissing) {
		t.Errorf("lỗi = %v, muốn ErrDeleteReasonMissing", err)
	}
	if _, err := NormalizeDeleteReason(strings.Repeat("a", DeleteReasonMaxLen+1)); !errors.Is(err, ErrDeleteReasonTooLong) {
		t.Error("lý do quá dài phải bị từ chối")
	}
}

func TestRowTierReadsBothColumns(t *testing.T) {
	// The method on the row, rather than the free function, because that is what the HTTP layer
	// calls. A method reading only `nguon` would report tier 2 for a row the source code branches
	// on — and the screen would then offer `Tắt` on the one row that must never be disabled.
	l := MapAssetType{Source: SourceSystem, BranchedInSource: true}
	if l.Tier() != TierBranched {
		t.Errorf("Tier() = %d, muốn %d", l.Tier(), TierBranched)
	}
}
