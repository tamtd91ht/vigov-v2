package app

import "testing"

// SoDoToChuc.Them WITHOUT a rank (owner decision 10/10/2026: the add-child popup drops "Thứ tự"): the
// server places the new unit right after the last LIVE sibling of THIS commune under the same parent.
// Over the real store and the fake driver, so the rank asserted is the one the INSERT carried.
//
// MUTATIONS THAT MUST TURN THIS RED:
//   - default to 0 again (every new unit jumps to the top of its branch)
//   - count the soft-deleted unit (rank 9) or the other commune's unit
//   - override a rank the client DID send
func TestCreateOrgUnitDefaultsOrderAfterLastSibling(t *testing.T) {
	zero := 0
	for _, c := range []struct {
		name   string
		parent string
		order  *int
		want   int
	}{
		{"root: after LÃNH ĐẠO (1), the deleted unit (9) and commune B ignored", "", nil, 2},
		{"under bp-a: after VĂN PHÒNG (2)", "bp-a", nil, 3},
		{"under a leaf: no sibling", "bp-c", nil, 0},
		{"a rank that is sent is used as sent", "bp-a", &zero, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			k := &khoBoPhanGia{hang: cayBaTang()}
			uc, ctx := dungUseCaseBoPhan(t, k)

			bp, err := uc.Them(ctx, YeuCauThemBoPhan{Ten: "TỔ MỚI", ChaID: c.parent, ThuTu: c.order}, nguoiBoPhan())
			if err != nil {
				t.Fatalf("Them: %v", err)
			}
			if bp.ThuTu != c.want {
				t.Errorf("thứ tự trả về = %d, muốn %d", bp.ThuTu, c.want)
			}
			ins := k.cau("INSERT INTO bo_phan")
			if len(ins) != 1 {
				t.Fatalf("có %d câu INSERT, muốn 1", len(ins))
			}
			if got := ins[0].args[5]; got != int64(c.want) {
				t.Errorf("thu_tu ghi xuống = %v, muốn %d", got, c.want)
			}
			// The rank is part of the creation's trail, in the same transaction.
			sau, _ := motVetBoPhan(t, k, HanhViThemBoPhan, bp.Ma)["sau"].(map[string]any)
			if n, _ := sau["thu_tu"].(float64); int(n) != c.want {
				t.Errorf("vết thu_tu = %v, muốn %d", sau["thu_tu"], c.want)
			}
		})
	}
}
