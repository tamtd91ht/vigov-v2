package app

import (
	"errors"
	"testing"

	"github.com/vihat/vigov/service-comms/internal/domain"
)

// One name per category (owner 13/10/2026): a create or a rename to the name of another LIVE category of
// the commune — any case, any spacing — is refused before anything is written.

func TestCreateCategoryWithTakenNameIsRefusedWhateverTheCase(t *testing.T) {
	for _, typed := range []string{"Nông nghiệp", "nông nghiệp", "NÔNG NGHIỆP", "  nông   nghiệp ", "nông nghiệp"} {
		k := &khoNDGia{liveNames: []domain.CategoryName{{ID: "dm-1", Ten: "Nông nghiệp"}}}
		_, ucDM, ctx := dungUseCaseNoiDung(t, k)
		_, err := ucDM.Them(ctx, domain.YeuCauThemDanhMuc{Ten: typed}, nguoiSoanND())
		var taken domain.CategoryNameTakenError
		if !errors.As(err, &taken) || taken.Existing != "Nông nghiệp" {
			t.Errorf("%q: err = %v, want CategoryNameTakenError naming “Nông nghiệp”", typed, err)
		}
		if k.coCau("INSERT INTO danh_muc_mini_app") || k.coCau("INSERT INTO audit_log") {
			t.Errorf("%q: a taken name still wrote", typed)
		}
	}
}

func TestCreateCategoryWithFreeNameStillWorks(t *testing.T) {
	k := &khoNDGia{liveNames: []domain.CategoryName{{ID: "dm-1", Ten: "Nông nghiệp"}}}
	_, ucDM, ctx := dungUseCaseNoiDung(t, k)
	if _, err := ucDM.Them(ctx, domain.YeuCauThemDanhMuc{Ten: "Nông nghiệp sạch"}, nguoiSoanND()); err != nil {
		t.Fatalf("create: %v", err)
	}
	if !k.coCau("INSERT INTO danh_muc_mini_app") {
		t.Error("a free name was not inserted")
	}
}

func TestRenameCategoryToAnotherCategorysNameIsRefused(t *testing.T) {
	k := &khoNDGia{category: categoryNow(), liveNames: []domain.CategoryName{
		{ID: "dm-1", Ten: categoryNow().Ten}, {ID: "dm-2", Ten: "Giáo dục"},
	}}
	_, ucDM, ctx := dungUseCaseNoiDung(t, k)
	name := "giáo dục"
	_, err := ucDM.Update(ctx, "dm-1", domain.ContentCategoryUpdate{Ten: &name}, nguoiSoanND())
	if !errors.Is(err, domain.ErrCategoryNameTaken) {
		t.Fatalf("err = %v, want ErrCategoryNameTaken", err)
	}
	if k.coCau("UPDATE danh_muc_mini_app") || k.coCau("INSERT INTO audit_log") || k.daCommit != 0 {
		t.Error("a refused rename still wrote")
	}
}

// Changing only the case of its OWN name is an edit, not a collision with itself.
func TestRenameCategoryToADifferentCaseOfItsOwnName(t *testing.T) {
	k := &khoNDGia{category: categoryNow(), liveNames: []domain.CategoryName{{ID: "dm-1", Ten: categoryNow().Ten}}}
	_, ucDM, ctx := dungUseCaseNoiDung(t, k)
	name := domain.CategoryNameKey(categoryNow().Ten)
	if name == categoryNow().Ten {
		t.Fatalf("fixture name %q is already lower case — the test proves nothing", name)
	}
	if _, err := ucDM.Update(ctx, "dm-1", domain.ContentCategoryUpdate{Ten: &name}, nguoiSoanND()); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if !k.coCau("UPDATE danh_muc_mini_app") {
		t.Error("the case change was not written")
	}
}
