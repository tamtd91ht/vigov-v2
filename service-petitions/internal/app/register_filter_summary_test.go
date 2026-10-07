package app

import (
	"testing"

	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// The Sổ theo dõi export's trail records the Sổ tay filters (ADR 0071) by NAME: the scope is the
// actor's own, so the code behind it is not repeated, and `incomplete` is recorded as set.
func TestRegisterFilterSummaryRecordsLeaderNotebookFilters(t *testing.T) {
	out := registerFilterSummary(petstore.LocNhiemVu{AssignedByStaffCode: "CB-00123", Incomplete: true})

	if out["scope"] != "assigned-by-me" || out["incomplete"] != true {
		t.Errorf("tóm tắt bộ lọc = %v, muốn scope=assigned-by-me và incomplete=true", out)
	}
	for k, v := range out {
		if v == "CB-00123" {
			t.Errorf("trường %q mang mã cán bộ của phạm vi — vết đã có người thực hiện", k)
		}
	}
	if len(out) != 2 {
		t.Errorf("tóm tắt bộ lọc = %v, muốn đúng hai khoá", out)
	}
}

// `roots=true` narrows the exported file, so the trail says it was set; absent, no key at all.
func TestRegisterFilterSummaryRecordsRoots(t *testing.T) {
	if out := registerFilterSummary(petstore.LocNhiemVu{Roots: true}); out["roots"] != true || len(out) != 1 {
		t.Errorf("tóm tắt bộ lọc = %v, muốn đúng roots=true", out)
	}
	if out := registerFilterSummary(petstore.LocNhiemVu{}); len(out) != 0 {
		t.Errorf("tóm tắt bộ lọc khi không lọc = %v, muốn rỗng", out)
	}
}
