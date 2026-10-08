package store

import (
	"errors"
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

// The full-email read (ADR 0082 §3) and the picker's masked address, on the real engine.
func TestEmailForRevealPg(t *testing.T) {
	db := moKetNoi(t)
	xaA, xaB := xaRieng(t)

	dungXa(t, db, xaA, "vt-ld", "nd-ld", []string{"task.read"})
	dungXa(t, db, xaA, "vt-kh", "nd-kh", nil) // locked below — still readable
	dungXa(t, db, xaA, "vt-ne", "nd-ne", nil) // no address below
	dungXa(t, db, xaB, "vt-ld", "nd-b", nil)  // commune B
	if _, err := db.Exec(`UPDATE nguoi_dung SET dang_hoat_dong = false WHERE tenant_id = $1 AND id = $2`,
		xaA, "nd-kh"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE nguoi_dung SET email = NULL WHERE tenant_id = $1 AND id = $2`,
		xaA, "nd-ne"); err != nil {
		t.Fatal(err)
	}

	db2 := pkgstore.New(db)
	staff := NewCanBoStore(db2)
	reveal := func(xa, code string) (string, error) {
		t.Helper()
		var email string
		err := db2.For(ctxXa(xa)).Tx(ctxXa(xa), func(tx *pkgstore.ScopedTx) error {
			var err error
			email, err = staff.EmailForReveal(ctxXa(xa), tx, code)
			return err
		})
		return email, err
	}

	if got, err := reveal(xaA, "CB-nd-ld"); err != nil || got != "nd-ld@xa.danang.gov.vn" {
		t.Fatalf("CB-nd-ld = %q, %v", got, err)
	}
	if got, err := reveal(xaA, "CB-nd-kh"); err != nil || got != "nd-kh@xa.danang.gov.vn" {
		t.Fatalf("a locked assignee's address must stay readable: %q, %v", got, err)
	}
	for _, code := range []string{"CB-nd-ne", "CB-nd-b", "CB-khong-co"} {
		if _, err := reveal(xaA, code); !errors.Is(err, ErrCanBoKhongTonTai) {
			t.Errorf("%s in commune A: err = %v, want ErrCanBoKhongTonTai", code, err)
		}
	}

	ds, err := staff.ChonNguoi(ctxXa(xaA), domain.LocChonNguoi{})
	if err != nil {
		t.Fatal(err)
	}
	masked := map[string]string{}
	for _, cb := range ds {
		masked[cb.Ma] = cb.EmailMasked
	}
	if masked["CB-nd-ld"] != "n***@xa.danang.gov.vn" || masked["CB-nd-ne"] != "" {
		t.Fatalf("masked addresses = %v", masked)
	}
}
