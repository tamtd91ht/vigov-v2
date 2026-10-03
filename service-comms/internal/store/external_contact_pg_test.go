package store

import (
	"context"
	"errors"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// external_contacts against a REAL PostgreSQL — the half the fake driver in
// internal/app/external_contact_test.go cannot see: migration 0014's columns as the store names them,
// NULLS LAST in the order, the soft-delete predicate evaluated, and the guard trigger. SKIPPED WITHOUT
// VIGOV_TEST_DSN (shared harness: loai_tai_nguyen_ban_do_pg_test.go) — a green run without the DSN
// means this file COMPILES, nothing more.
//
// ROWS (xã 1), EACH MAKING ONE DEFECT VISIBLE:
//
//	o1        display_order 1               must come first
//	o5        display_order 5               must come second
//	none-a    NULL order, id "none-a"       NULLS FIRST would put it first
//	none-b    NULL order, id "none-b"       the tie-break drops → order unstable
//	gone      display_order 0, soft-deleted drop `deleted_at IS NULL` → appears first
//	xã 2's x  display_order 0               drop the commune → appears in xã 1
func TestPgExternalContactsListLiveOrderedScoped(t *testing.T) {
	db := moKetNoi(t)
	c1, c2 := xaRieng(t)
	for _, r := range []struct {
		tenant, id string
		order      any
	}{
		{c1, "o5", 5}, {c1, "none-b", nil}, {c1, "o1", 1}, {c1, "none-a", nil}, {c1, "gone", 0}, {c2, "x", 0},
	} {
		if _, err := db.Exec(`INSERT INTO external_contacts
			(tenant_id, id, name, category, phone, display_order, created_by, updated_by)
			VALUES ($1, $2, 'Tên '||$2, 'Nhóm', '0900000000', $3, 'CB-TEST', 'CB-TEST')`,
			r.tenant, r.id, r.order); err != nil {
			t.Fatalf("insert %s: %v", r.id, err)
		}
	}
	if _, err := db.Exec(`UPDATE external_contacts SET deleted_at = now(), deleted_by = 'CB-TEST',
		delete_reason = 'thử' WHERE tenant_id = $1 AND id = 'gone'`, c1); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	got, err := NewExternalContactStore(pkgstore.New(db)).List(ctxXa(tenant.ID(c1)))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	var ids []string
	for _, c := range got {
		ids = append(ids, c.ID)
	}
	want := []string{"o1", "o5", "none-a", "none-b"}
	if len(ids) != len(want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("ids = %v, want %v", ids, want)
		}
	}
	if got[2].DisplayOrder != nil || got[0].DisplayOrder == nil || *got[0].DisplayOrder != 1 || got[0].Address != "" {
		t.Errorf("scanned = %+v", got[0])
	}
}

// The write path end to end on a real server: insert, edit under lock, soft delete — and the trigger
// refusing an edit of the deleted row and a hard DELETE.
func TestPgExternalContactWritesAndGuard(t *testing.T) {
	db := moKetNoi(t)
	c1, _ := xaRieng(t)
	handle := pkgstore.New(db)
	s := NewExternalContactStore(handle)
	ctx := ctxXa(tenant.ID(c1))
	order := 3

	run := func(fn func(tx *pkgstore.ScopedTx) error) error {
		return handle.For(ctx).Tx(ctx, fn)
	}
	if err := run(func(tx *pkgstore.ScopedTx) error {
		return s.Insert(ctx, tx, domain.ExternalContact{ID: "w1", Name: "Điện lực", Category: "Điện",
			Phone: "1900 1006", Address: "Thôn 3", DisplayOrder: &order}, "CB-00123")
	}); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if err := run(func(tx *pkgstore.ScopedTx) error {
		c, err := s.ByIDForUpdate(ctx, tx, "w1")
		if err != nil {
			return err
		}
		c.Address, c.Phone = "", "19001006"
		return s.Update(ctx, tx, c, "CB-00456")
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	var address any
	var updatedBy, createdBy string
	if err := db.QueryRow(`SELECT address, updated_by, created_by FROM external_contacts
		WHERE tenant_id = $1 AND id = 'w1'`, c1).Scan(&address, &updatedBy, &createdBy); err != nil {
		t.Fatal(err)
	}
	if address != nil || updatedBy != "CB-00456" || createdBy != "CB-00123" {
		t.Errorf("after update: address=%v updated_by=%q created_by=%q", address, updatedBy, createdBy)
	}

	if err := run(func(tx *pkgstore.ScopedTx) error { return s.SoftDelete(ctx, tx, "w1", "CB-00456", "trùng") }); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}
	if err := run(func(tx *pkgstore.ScopedTx) error { return s.SoftDelete(ctx, tx, "w1", "CB-00789", "lần hai") }); !errors.Is(err, ErrExternalContactNotFound) {
		t.Errorf("second delete: %v, want ErrExternalContactNotFound", err)
	}
	if _, err := db.ExecContext(context.Background(),
		`UPDATE external_contacts SET name = 'sửa lén' WHERE tenant_id = $1 AND id = 'w1'`, c1); err == nil {
		t.Error("the trigger let a deleted row be edited")
	}
	if _, err := db.ExecContext(context.Background(),
		`DELETE FROM external_contacts WHERE tenant_id = $1 AND id = 'w1'`, c1); err == nil {
		t.Error("the trigger let a hard DELETE through")
	}
}
