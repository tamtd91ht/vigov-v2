package domain

import (
	"errors"
	"testing"
)

func TestCategoryNameKeyFoldsCaseSpacingAndComposition(t *testing.T) {
	want := CategoryNameKey("Đất đai")
	for _, s := range []string{"ĐẤT ĐAI", "đất đai", "  Đất \t đai ", "Đất đai"} {
		if got := CategoryNameKey(s); got != want {
			t.Errorf("CategoryNameKey(%q) = %q, want %q", s, got, want)
		}
	}
	if CategoryNameKey("Đất đai") == CategoryNameKey("Dat dai") {
		t.Error("the diacritics were folded away — 'Đất đai' and 'Dat dai' are different names")
	}
}

func TestCheckCategoryNameFree(t *testing.T) {
	live := []CategoryName{{ID: "a", Ten: "Y tế"}, {ID: "b", Ten: "Giáo dục"}}
	if err := CheckCategoryNameFree("Văn hoá", live, ""); err != nil {
		t.Errorf("free name refused: %v", err)
	}
	err := CheckCategoryNameFree("y TẾ", live, "")
	var taken CategoryNameTakenError
	if !errors.As(err, &taken) || taken.Existing != "Y tế" || !errors.Is(err, ErrCategoryNameTaken) {
		t.Errorf("err = %v, want the collision with “Y tế”", err)
	}
	if err := CheckCategoryNameFree("y tế", live, "a"); err != nil {
		t.Errorf("its own name refused on a rename: %v", err)
	}
}
