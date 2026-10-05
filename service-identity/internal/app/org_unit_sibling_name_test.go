package app

import (
	"errors"
	"testing"

	"github.com/vihat/vigov/service-identity/internal/domain"
)

// TC-02 (tester report 05/10/2026): the form refuses a name a LIVE sibling already carries, compared
// the way the Excel import compares it — trimmed, case-folded. Over the real store and the fake
// driver, which answers the sibling read only for the commune bound at $1.

func TestCreateOrgUnitSiblingNameTakenIsRefused(t *testing.T) {
	k := &khoBoPhanGia{hang: cayBaTang()}
	uc, ctx := dungUseCaseBoPhan(t, k)

	// `VĂN PHÒNG` (bp-b) already sits under bp-a; case and spaces differ, the folded name does not.
	_, err := uc.Them(ctx, YeuCauThemBoPhan{Ten: "  văn phòng ", ChaID: "bp-a", Ma: "vp-trung"}, nguoiBoPhan())
	if !errors.Is(err, domain.ErrOrgUnitNameTaken) {
		t.Fatalf("lỗi = %v, muốn ErrOrgUnitNameTaken", err)
	}
	if len(k.cau("INSERT")) != 0 || k.daCommit != 0 {
		t.Error("bị từ chối mà vẫn ghi hoặc commit")
	}
}

func TestCreateOrgUnitSiblingNameAllowedCases(t *testing.T) {
	for _, tc := range []struct {
		name string
		req  YeuCauThemBoPhan
	}{
		// The same name under ANOTHER parent is a different unit (bp-b is under bp-a, not the root).
		{"other parent", YeuCauThemBoPhan{Ten: "VĂN PHÒNG", Ma: "vp-goc"}},
		// A soft-deleted sibling does not hold its name — only its code.
		{"soft-deleted sibling", YeuCauThemBoPhan{Ten: "ĐÃ XOÁ", Ma: "da-xoa-moi"}},
		// Another commune's root unit is invisible: the read is scoped to the commune (rule 1).
		{"other commune", YeuCauThemBoPhan{Ten: "VĂN PHÒNG HĐND XÃ B", Ma: "vp-hdnd"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k := &khoBoPhanGia{hang: cayBaTang()}
			uc, ctx := dungUseCaseBoPhan(t, k)
			if _, err := uc.Them(ctx, tc.req, nguoiBoPhan()); err != nil {
				t.Fatalf("Them: %v", err)
			}
		})
	}
}

// siblingsUnderB adds a second unit under bp-b next to `TỔ MỘT CỬA` (bp-c), plus a root unit whose
// name collides with bp-c's, for the move case.
func siblingsUnderB() []hangBoPhan {
	return append(cayBaTang(),
		hangBoPhan{xa: xaBoPhan, id: "bp-d", ma: "to-ke-toan", ten: "TỔ KẾ TOÁN", cha: "bp-b", thuTu: 4},
		hangBoPhan{xa: xaBoPhan, id: "bp-g", ma: "to-mot-cua-goc", ten: "Tổ Một Cửa", thuTu: 5},
	)
}

func TestUpdateOrgUnitSiblingName(t *testing.T) {
	t.Run("rename onto a sibling's name is refused", func(t *testing.T) {
		k := &khoBoPhanGia{hang: siblingsUnderB()}
		uc, ctx := dungUseCaseBoPhan(t, k)
		_, err := uc.Sua(ctx, "bp-d", YeuCauSuaBoPhan{Ten: chuoi("tổ một cửa")}, nguoiBoPhan())
		if !errors.Is(err, domain.ErrOrgUnitNameTaken) {
			t.Fatalf("lỗi = %v, muốn ErrOrgUnitNameTaken", err)
		}
		if len(k.cau("UPDATE bo_phan")) != 0 || k.daCommit != 0 {
			t.Error("bị từ chối mà vẫn ghi hoặc commit")
		}
	})

	t.Run("rename to its own name in another case passes", func(t *testing.T) {
		k := &khoBoPhanGia{hang: siblingsUnderB()}
		uc, ctx := dungUseCaseBoPhan(t, k)
		if _, err := uc.Sua(ctx, "bp-c", YeuCauSuaBoPhan{Ten: chuoi("Tổ một cửa")}, nguoiBoPhan()); err != nil {
			t.Fatalf("Sua: %v", err)
		}
		if len(k.cau("UPDATE bo_phan")) != 1 {
			t.Error("đổi chữ hoa/thường của chính tên mình mà không ghi")
		}
	})

	t.Run("move under a parent holding the same name is refused", func(t *testing.T) {
		k := &khoBoPhanGia{hang: siblingsUnderB()}
		uc, ctx := dungUseCaseBoPhan(t, k)
		_, err := uc.Sua(ctx, "bp-c", YeuCauSuaBoPhan{ChaID: chuoi("")}, nguoiBoPhan())
		if !errors.Is(err, domain.ErrOrgUnitNameTaken) {
			t.Fatalf("lỗi = %v, muốn ErrOrgUnitNameTaken", err)
		}
	})

	// Two siblings that already share a name (written before this check) can still be re-ranked.
	t.Run("rank-only change on legacy duplicates passes", func(t *testing.T) {
		rows := append(cayBaTang(),
			hangBoPhan{xa: xaBoPhan, id: "bp-dup", ma: "to-mot-cua-2", ten: "TỔ MỘT CỬA", cha: "bp-b", thuTu: 6})
		k := &khoBoPhanGia{hang: rows}
		uc, ctx := dungUseCaseBoPhan(t, k)
		if _, err := uc.Sua(ctx, "bp-dup", YeuCauSuaBoPhan{ThuTu: so(7)}, nguoiBoPhan()); err != nil {
			t.Fatalf("Sua thứ tự: %v", err)
		}
		if len(k.cau("cha_id IS NOT DISTINCT FROM")) != 0 {
			t.Error("chỉ đổi thứ tự mà vẫn kiểm tên cùng cấp")
		}
	})
}
