package domain

import (
	"errors"
	"testing"
)

func TestCategoryNameKeyFoldsCaseSpacingAndComposition(t *testing.T) {
	want := CategoryNameKey("Đất đai")
	for _, s := range []string{"ĐẤT ĐAI", "đất đai", "  Đất \t đai ", "Đất đai", "Đất\u200b đai\ufeff"} {
		if got := CategoryNameKey(s); got != want {
			t.Errorf("CategoryNameKey(%q) = %q, want %q", s, got, want)
		}
	}
	if CategoryNameKey("Đất đai") == CategoryNameKey("Dat dai") {
		t.Error("the diacritics were folded away — 'Đất đai' and 'Dat dai' are different names (owner 13/10/2026)")
	}
}

// One name per PARENT (owner 13/10/2026): siblings collide, cousins do not.
func TestCheckCategoryNameFree(t *testing.T) {
	live := []CategoryName{
		{ID: "a", Ten: "Y tế"},
		{ID: "b", Ten: "Giáo dục"},
		{ID: "c", Ten: "Chuyển đổi số", ChaID: "a"},
	}
	if err := CheckCategoryNameFree("Văn hoá", "", live, ""); err != nil {
		t.Errorf("free name refused: %v", err)
	}
	err := CheckCategoryNameFree("y TẾ", "", live, "")
	var taken CategoryNameTakenError
	if !errors.As(err, &taken) || taken.Existing != "Y tế" || !errors.Is(err, ErrCategoryNameTaken) {
		t.Errorf("root collision: err = %v, want the collision with “Y tế”", err)
	}
	if err := CheckCategoryNameFree("chuyển đổi số", "a", live, ""); !errors.Is(err, ErrCategoryNameTaken) {
		t.Errorf("sibling collision under a: err = %v", err)
	}
	if err := CheckCategoryNameFree("Chuyển đổi số", "b", live, ""); err != nil {
		t.Errorf("the same name under ANOTHER parent is allowed: %v", err)
	}
	if err := CheckCategoryNameFree("Chuyển đổi số", "", live, ""); err != nil {
		t.Errorf("a root may share a child's name: %v", err)
	}
	if err := CheckCategoryNameFree("y tế", "", live, "a"); err != nil {
		t.Errorf("its own name refused on a rename: %v", err)
	}
}
