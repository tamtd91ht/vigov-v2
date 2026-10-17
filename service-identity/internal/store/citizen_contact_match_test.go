package store

import (
	"context"
	"database/sql/driver"
	"strings"
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
)

// The SHAPE of the two link reads (ADR 0102) on the fake driver of kenh_cong_dan_kho_gia_test.go — so the
// commune and live predicates are checked on every `go test`, not only where VIGOV_TEST_DSN is set. The
// behaviour against real rows is citizen_contact_match_pg_test.go.

const matchCommune = "01J0000000000000000000000A"

func TestLiveIDByPhoneIsScopedLiveAndExact(t *testing.T) {
	k := khoMoi()
	k.hang = []map[string]driver.Value{{"id": "CT01JCONTACTSAMPLE000000000"}}
	s := NewCitizenDirectoryStore(dbGia(k))

	id, found, err := s.LiveIDByPhone(ctxXa(matchCommune), "0900000000")
	if err != nil || !found || id != "CT01JCONTACTSAMPLE000000000" {
		t.Fatalf("LiveIDByPhone = %q, %v, %v", id, found, err)
	}
	q := k.cuoi()
	for _, want := range []string{"FROM citizen_contact", "tenant_id = $1", "deleted_at IS NULL", "phone = $2"} {
		if !strings.Contains(q.sql, want) {
			t.Errorf("statement lacks %q: %s", want, q.sql)
		}
	}
	// "ngừng liên hệ" rows are found: the statement must not filter on is_active.
	if strings.Contains(q.sql, "is_active") {
		t.Errorf("statement filters is_active: %s", q.sql)
	}
	if q.args[0] != matchCommune || q.args[1] != "0900000000" {
		t.Errorf("args = %v", q.args)
	}
}

func TestLiveIDByPhoneEmptyNeverQueries(t *testing.T) {
	k := khoMoi()
	s := NewCitizenDirectoryStore(dbGia(k))
	if id, found, err := s.LiveIDByPhone(ctxXa(matchCommune), ""); err != nil || found || id != "" {
		t.Fatalf("got %q, %v, %v", id, found, err)
	}
	if len(k.lenh) != 0 {
		t.Errorf("an empty number reached SQL: %v", k.lenh)
	}
}

func TestLiveStaffCodeExistsExcludesRemovedRows(t *testing.T) {
	k := khoMoi()
	k.hang = []map[string]driver.Value{{"1": int64(1)}}
	db := dbGia(k)
	s := NewCanBoStore(db)

	ctx := ctxXa(matchCommune)
	err := db.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		ok, err := s.LiveStaffCodeExists(ctx, tx, "CB-00123")
		if err != nil || !ok {
			t.Errorf("LiveStaffCodeExists = %v, %v", ok, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	q := k.cuoi()
	for _, want := range []string{"FROM nguoi_dung", "tenant_id = $1", "deleted_at IS NULL", "ma = $2"} {
		if !strings.Contains(q.sql, want) {
			t.Errorf("statement lacks %q: %s", want, q.sql)
		}
	}
	if q.tx == 0 {
		t.Error("the actor check ran outside the caller's transaction")
	}
	if _, err := s.LiveStaffCodeExists(context.Background(), nil, "CB-00123"); err == nil {
		t.Error("a nil transaction was accepted")
	}
}
