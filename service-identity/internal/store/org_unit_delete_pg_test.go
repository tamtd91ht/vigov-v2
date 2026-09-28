package store

import (
	"errors"
	"strings"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
)

// The delete's three statements. The first test runs everywhere and pins the text that decides who
// is counted; the second needs PostgreSQL (VIGOV_TEST_DSN) and skips without it.

// ONE PREDICATE FOR "sits in the unit": the org chart's count and the delete refusal must use the
// same one, or the screen says "0 cán bộ" against a unit the delete refuses.
func TestOrgUnitDeleteCountsShareTheListPredicate(t *testing.T) {
	if !strings.Contains(truyVanBoPhanKemSoCanBo, staffCountedInUnit) {
		t.Error("câu đếm của sơ đồ tổ chức không dùng staffCountedInUnit")
	}
	for _, want := range []string{
		"nd.tenant_id = bp.tenant_id",  // the staff subquery repeats the commune (rule 1)
		"con.tenant_id = bp.tenant_id", // so does the child subquery
		"con.deleted_at IS NULL",       // a soft-deleted child holds nothing
		"AND " + staffCountedInUnit,    // the shared predicate
	} {
		if !strings.Contains(localHoldingsCols, want) {
			t.Errorf("localHoldingsCols thiếu %q", want)
		}
	}
}

func TestPgOrgUnitDelete(t *testing.T) {
	db := moKetNoi(t)
	commune, otherCommune := xaRieng(t)
	kho := pkgstore.New(db)
	s := NewBoPhanStore(kho)
	ctx := ctxXa(commune)

	themBoPhanPg(t, db, commune, "bp-1", "van-phong", "", false)
	themBoPhanPg(t, db, commune, "bp-2", "to-con", "bp-1", false)
	themBoPhanPg(t, db, commune, "bp-3", "to-con-cu", "bp-1", true) // deleted child: not counted
	themNguoiBoPhanPg(t, db, commune, "nd-1", "bp-1", true, false)
	themNguoiBoPhanPg(t, db, commune, "nd-khoa", "bp-1", false, false) // locked: not counted
	themBoPhanPg(t, db, otherCommune, "bp-1", "van-phong", "", false)
	themNguoiBoPhanPg(t, db, otherCommune, "nd-k", "bp-1", true, false) // other commune: not counted

	bp, h, err := s.LiveForDelete(ctx, "bp-1")
	if err != nil || bp.Ma != "van-phong" || h.Staff != 1 || h.ChildUnits != 1 {
		t.Fatalf("LiveForDelete = %+v %+v %v — muốn 1 cán bộ, 1 bộ phận con", bp, h, err)
	}
	if _, _, err := s.LiveForDelete(ctx, "bp-3"); !errors.Is(err, ErrKhongTimThayBoPhan) {
		t.Errorf("bộ phận đã xoá: %v", err)
	}

	err = kho.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		again, err := s.LocalHoldings(ctx, tx, "bp-1")
		if err != nil || again.Staff != 1 || again.ChildUnits != 1 {
			t.Errorf("LocalHoldings = %+v %v", again, err)
		}
		// bp-2 is a leaf with nobody in it.
		if err := s.SoftDelete(ctx, tx, "bp-2", "CB-0077", "sáp nhập"); err != nil {
			return err
		}
		// The second delete cannot overwrite who removed it or why.
		if err := s.SoftDelete(ctx, tx, "bp-2", "CB-0099", "khác"); !errors.Is(err, ErrKhongTimThayBoPhan) {
			t.Errorf("xoá lần hai: %v, muốn ErrKhongTimThayBoPhan", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var by, reason string
	if err := db.QueryRow(`SELECT deleted_by, delete_reason FROM bo_phan WHERE tenant_id = $1 AND id = 'bp-2' AND deleted_at IS NOT NULL`,
		commune).Scan(&by, &reason); err != nil || by != "CB-0077" || reason != "sáp nhập" {
		t.Errorf("dòng đã xoá: by=%q reason=%q err=%v", by, reason, err)
	}
	// The other commune's bp-1 is untouched by anything above.
	if _, h, err := s.LiveForDelete(ctxXa(otherCommune), "bp-1"); err != nil || h.Staff != 1 || h.ChildUnits != 0 {
		t.Errorf("xã khác: %+v %v", h, err)
	}
}
