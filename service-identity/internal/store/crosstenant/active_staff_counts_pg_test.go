package crosstenant

import (
	"context"
	"reflect"
	"testing"
)

// The count behind OperatorService.ListActiveStaffCounts, on a real PostgreSQL (skips without
// VIGOV_TEST_DSN — read the skip line). What is verified lives in the SQL: the three clauses of the
// predicate, the grouping, the order, and that a commune with nobody active is ABSENT, not 0.
//
// MUTATIONS THAT MUST TURN THIS RED: drop `deleted_at IS NULL` and commune B counts 3; drop
// `co_tai_khoan` and B counts 3; drop `dang_hoat_dong` and B counts 3; drop the ORDER BY and the
// order may differ; a LEFT JOIN on some commune list and commune C appears with 0.
func TestActiveStaffCountsPg(t *testing.T) {
	db := moKetNoi(t)
	ctx := context.Background()

	const (
		communeA = "01JASC0000000000000000000A" // two active
		communeB = "01JASC0000000000000000000B" // one active, plus one of each excluded kind
		communeC = "01JASC0000000000000000000C" // nobody active: deleted, no account, locked
	)
	type row struct {
		tenant, id          string
		account, active, rm bool
	}
	rows := []row{
		// Inserted out of tenant order, so an unordered read is caught.
		{communeB, "b-active", true, true, false},
		{communeB, "b-deleted", true, true, true},
		{communeB, "b-directory-only", false, true, false},
		{communeB, "b-locked", true, false, false},
		{communeA, "a-1", true, true, false},
		{communeA, "a-2", true, true, false},
		{communeC, "c-deleted", true, true, true},
		{communeC, "c-directory-only", false, true, false},
		{communeC, "c-locked", true, false, false},
	}
	for _, r := range rows {
		hash := ""
		if r.account {
			hash = "argon2id-fake-hash-for-tests"
		}
		// deleted_at only — the soft-delete columns beside it carry no constraint the read depends on.
		_, err := db.ExecContext(ctx, `INSERT INTO nguoi_dung
			  (tenant_id, id, ma, ho_ten, co_tai_khoan, dang_hoat_dong, mat_khau_hash, deleted_at, deleted_by, delete_reason)
			VALUES ($1, $2, $3, 'Nguyen Van A', $4, $5, $6,
			        CASE WHEN $7 THEN now() END, CASE WHEN $7 THEN 'CB-00001' END, CASE WHEN $7 THEN 'test' END)`,
			r.tenant, r.id, "CB-"+r.id, r.account, r.active, hash, r.rm)
		if err != nil {
			t.Fatalf("insert %s: %v", r.id, err)
		}
	}

	got, err := NewActiveStaffCountStore(db).ActiveStaffCounts(ctx)
	if err != nil {
		t.Fatalf("ActiveStaffCounts: %v", err)
	}
	want := []CommuneStaffCount{{TenantID: communeA, Count: 2}, {TenantID: communeB, Count: 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("counts = %+v, want %+v (C absent, B counting only its active account, ordered by tenant_id)", got, want)
	}
}

func TestNewActiveStaffCountStoreRefusesNoHandle(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("a store with no handle was built")
		}
	}()
	NewActiveStaffCountStore(nil)
}
