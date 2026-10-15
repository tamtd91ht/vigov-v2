package store

// Integration test for the ORDER of GET /api/v1/investment-projects against a real PostgreSQL.
//
// SKIPPED UNLESS VIGOV_TEST_DSN IS SET — a green run without it means the code COMPILES, nothing more.

import (
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// Newest project first (owner decision 15/10/2026), and two projects created at the SAME instant keep
// one fixed order — id descending — so the list is a total order. The codes are chosen so that code
// order (a, c, b) differs from the expected one (c, b, a): a list still sorted by code turns this red.
func TestPgProjectListNewestFirst(t *testing.T) {
	db := moKetNoi(t)
	commune, _ := xaRieng(t)
	themDuAnToiThieu(t, db, commune, "da-a", "DA01", 2026, 1)
	themDuAnToiThieu(t, db, commune, "da-b", "DA03", 2026, 1)
	themDuAnToiThieu(t, db, commune, "da-c", "DA02", 2026, 1)

	setCreated := func(id, at string) {
		t.Helper()
		if _, err := db.Exec(`UPDATE du_an SET tao_luc = $3::timestamptz WHERE tenant_id = $1 AND id = $2`,
			commune, id, at); err != nil {
			t.Fatalf("set tao_luc %s: %v", id, err)
		}
	}
	setCreated("da-a", "2026-01-01T00:00:00Z")
	setCreated("da-b", "2026-02-01T00:00:00Z") // same instant as da-c: id breaks the tie
	setCreated("da-c", "2026-02-01T00:00:00Z")

	list, err := NewDuAnStore(pkgstore.New(db)).DanhSach(ctxXa(tenant.ID(commune)), LocDuAn{Nam: 2026})
	if err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	var got []string
	for _, p := range list {
		got = append(got, p.DuAn.ID)
	}
	want := []string{"da-c", "da-b", "da-a"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v (tao_luc DESC, id DESC)", got, want)
		}
	}
}
