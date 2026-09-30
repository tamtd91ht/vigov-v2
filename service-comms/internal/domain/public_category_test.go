package domain

import (
	"strings"
	"testing"
)

func idsOf(cs []DanhMucMiniApp) string {
	var ids []string
	for _, c := range cs {
		ids = append(ids, c.ID)
	}
	return strings.Join(ids, ",")
}

// tree:
//
//	root-a (no item)       ─ child-a1 (item) ─ grand-a1x (no item)
//	                       ─ child-a2 (no item)
//	root-b (item)
//	root-c (no item)       ─ child-c1 (no item) ─ grand-c1x (item)
//	orphan (item, parent soft-deleted → absent from `live`) ─ orphan-kid (item)
func sampleTree() []DanhMucMiniApp {
	return []DanhMucMiniApp{
		{ID: "root-a"}, {ID: "child-a1", ChaID: "root-a"}, {ID: "grand-a1x", ChaID: "child-a1"},
		{ID: "child-a2", ChaID: "root-a"},
		{ID: "root-b"},
		{ID: "root-c"}, {ID: "child-c1", ChaID: "root-c"}, {ID: "grand-c1x", ChaID: "child-c1"},
		{ID: "orphan", ChaID: "deleted-parent"}, {ID: "orphan-kid", ChaID: "orphan"},
	}
}

func TestCategoriesWithPublishedItemsKeepsAncestorsOfUsedAndHidesEmpty(t *testing.T) {
	got := CategoriesWithPublishedItems(sampleTree(),
		[]string{"child-a1", "root-b", "grand-c1x", "orphan", "orphan-kid", "not-live"})
	// MUTATIONS THAT MUST TURN THIS RED: parents not marked (root-a, root-c, child-c1 vanish); empty
	// categories kept (grand-a1x, child-a2); orphans promoted (orphan, orphan-kid appear); an id absent
	// from the live tree (soft-deleted) resurrected.
	if want := "root-a,child-a1,root-b,root-c,child-c1,grand-c1x"; idsOf(got) != want {
		t.Fatalf("chip = %s, muốn %s", idsOf(got), want)
	}
}

func TestCategoriesWithPublishedItemsNothingUsedIsEmptyNotNil(t *testing.T) {
	got := CategoriesWithPublishedItems(sampleTree(), nil)
	if got == nil || len(got) != 0 {
		t.Fatalf("không mục nào có tin: %#v, muốn slice rỗng", got)
	}
}

func TestCategoriesWithPublishedItemsSurvivesACycle(t *testing.T) {
	// Not producible today (create-only writes), but a walk that loops forever is a hung request.
	live := []DanhMucMiniApp{{ID: "x", ChaID: "y"}, {ID: "y", ChaID: "x"}, {ID: "r"}}
	if got := CategoriesWithPublishedItems(live, []string{"x", "r"}); idsOf(got) != "r" {
		t.Fatalf("chu trình: chip = %s, muốn r", idsOf(got))
	}
}

func TestValidPublicCategoryID(t *testing.T) {
	for _, ok := range []string{"01JDM0000000000000000000AA", "dm-a", "a_b", strings.Repeat("x", 64)} {
		if !ValidPublicCategoryID(ok) {
			t.Errorf("%q bị từ chối", ok)
		}
	}
	for _, bad := range []string{"", strings.Repeat("x", 65), "a b", "a'--", "a/b", "đm", "a\x00", "a%27"} {
		if ValidPublicCategoryID(bad) {
			t.Errorf("%q được nhận", bad)
		}
	}
}
