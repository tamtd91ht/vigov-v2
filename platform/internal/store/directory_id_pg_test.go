package store

// Integration tests for Directory.ByID against a real PostgreSQL.
//
// A separate file from directory_pg_test.go, reusing its harness (moKetNoi, chayMigration,
// themXa, themHost). What is checked here is behaviour the database owns and a mock would only
// agree with: the LEFT JOIN that picks the ONE canonical host, and the fact that a deactivated
// commune still comes back.
//
// Skipped unless VIGOV_TEST_DSN is set, so a machine with no database stays green.

import (
	"context"
	"errors"
	"testing"

	"github.com/vihat/vigov/core/tenant"
)

func TestPgByIDTraVeHostChinh(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Thăng Bình", true)
	// Two hosts, as after a merger: the old address must keep resolving while people learn the
	// new one (rule 7). Exactly one of them is canonical, and that is the one a link uses.
	themHost(t, db, "cu.vigov.vn", ulidA, false)
	themHost(t, db, "thangbinh.vigov.vn", ulidA, true)

	got, err := NewDirectory(db).ByID(context.Background(), tenant.ID(ulidA))
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if got.ID != tenant.ID(ulidA) || got.Name != "Xã Thăng Bình" {
		t.Errorf("xã trả về sai: %+v", got)
	}
	if got.Host != "thangbinh.vigov.vn" {
		t.Errorf("Host = %q, muốn host chính", got.Host)
	}
	if !got.Active {
		t.Error("Active = false với xã đang hoạt động")
	}
}

// A merged commune keeps its data and its codes (rule 7), so it must stay describable — an
// archival record referring to it still has to be able to render a name. Active=false is the
// answer; "không có xã" is not.
func TestPgByIDXaNgungHoatDongVanTraVe(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidB, "Xã Đã Sáp Nhập", false)

	got, err := NewDirectory(db).ByID(context.Background(), tenant.ID(ulidB))
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if got.Active {
		t.Error("Active = true với xã đã ngừng hoạt động")
	}
	// No domain row at all: a commune configured without a host is a configuration state, not
	// a missing commune.
	if got.Host != "" {
		t.Errorf("Host = %q, muốn rỗng", got.Host)
	}
}

func TestPgByIDKhongTonTai(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)

	_, err := NewDirectory(db).ByID(context.Background(), tenant.ID(ulidA))
	if !errors.Is(err, ErrKhongCoXa) {
		t.Fatalf("err = %v, muốn ErrKhongCoXa", err)
	}
}

// Checked before the query runs: an identifier that is not a ULID means the caller is using an
// administrative code or a name, which rule 1 invariant 2 forbids.
func TestPgByIDIdKhongPhaiUlid(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)

	_, err := NewDirectory(db).ByID(context.Background(), tenant.ID("26734"))
	if err == nil {
		t.Fatal("id không phải ULID lại được chấp nhận")
	}
	if errors.Is(err, ErrKhongCoXa) {
		t.Fatalf("err = %v — id sai định dạng không phải là 'không có xã'", err)
	}
}
