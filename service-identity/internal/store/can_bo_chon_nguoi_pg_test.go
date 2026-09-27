package store

import (
	"strings"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// The picker's `permission` filter against a REAL PostgreSQL — the one place the semi-join's
// correlation, the per-commune JOINs and the placeholder numbering are checked by the engine that
// runs them in production rather than by a model of it. Skipped without VIGOV_TEST_DSN.
//
// dungXa gives every person id `nd-…` and code `CB-nd-…`, so a semi-join keyed on `nd.id` instead of
// `nd.ma` returns nobody here, exactly as it would in production.
func TestChonNguoiLocQuyenPg(t *testing.T) {
	db := moKetNoi(t)
	xaA, xaB := xaRieng(t)

	dungXa(t, db, xaA, "vt-ld", "nd-ld", []string{"task.extend", "task.read"}) // the holder
	dungXa(t, db, xaA, "vt-cv", "nd-cv", []string{"task.read"})                // pickable, not a holder
	dungXa(t, db, xaA, "vt-kh", "nd-kh", []string{"task.extend"})              // a holder, then locked
	dungXa(t, db, xaB, "vt-ld", "nd-b", []string{"task.extend"})               // same role id, commune B

	if _, err := db.Exec(`UPDATE nguoi_dung SET dang_hoat_dong = false WHERE tenant_id = $1 AND id = $2`,
		xaA, "nd-kh"); err != nil {
		t.Fatalf("khoá tài khoản mẫu: %v", err)
	}

	kho := NewCanBoStore(pkgstore.New(db))
	doc := func(xa string, loc domain.LocChonNguoi) string {
		t.Helper()
		ds, err := kho.ChonNguoi(ctxXa(xa), loc)
		if err != nil {
			t.Fatalf("ChonNguoi(%+v): %v", loc, err)
		}
		var ma []string
		for _, cb := range ds {
			ma = append(ma, cb.Ma)
		}
		return strings.Join(ma, ",")
	}

	if got := doc(xaA, domain.LocChonNguoi{}); got != "CB-nd-cv,CB-nd-ld" {
		t.Fatalf("không lọc = %q, muốn CB-nd-cv,CB-nd-ld (người bị khoá không được chọn)", got)
	}
	if got := doc(xaA, domain.LocChonNguoi{QuyenMa: "task.extend"}); got != "CB-nd-ld" {
		t.Fatalf("task.extend ở xã A = %q, muốn CB-nd-ld", got)
	}
	if got := doc(xaB, domain.LocChonNguoi{QuyenMa: "task.extend"}); got != "CB-nd-b" {
		t.Fatalf("task.extend ở xã B = %q, muốn CB-nd-b", got)
	}
	if got := doc(xaA, domain.LocChonNguoi{QuyenMa: "task.khong_ton_tai"}); got != "" {
		t.Fatalf("khoá không có trong danh mục mà nhận %q", got)
	}
}
