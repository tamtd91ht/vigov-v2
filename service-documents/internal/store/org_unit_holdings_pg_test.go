package store

import (
	"context"
	"database/sql"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
)

// Against a real PostgreSQL: CountOpenHeldByOrgUnit counts the live, unfinished incoming documents
// whose ĐANG GIỮ column is the unit — with every finished status, a soft-deleted row, another unit
// and another commune present to be wrongly counted. SKIPS WITHOUT VIGOV_TEST_DSN; the harness is
// in loai_van_ban_pg_test.go.

func insertHeldIncoming(t *testing.T, db *sql.DB, tenantID string, no int, id, status, unit string, deleted bool) {
	t.Helper()
	var holder, delAt, delBy, delReason any
	if unit != "" {
		holder = unit
	}
	if deleted {
		delAt, delBy, delReason = time.Now().UTC(), "CB-00123", "vào sổ nhầm"
	}
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO van_ban_den (tenant_id, id, so_vao_so, nam, ngay_den, co_quan_ban_hanh,
		   loai_van_ban, trich_yeu, han_xu_ly_xong, trang_thai, nguoi_tao_ma, bo_phan_dang_giu_id,
		   deleted_at, deleted_by, delete_reason)
		 VALUES ($1, $2, $3, 2026, '2026-09-01'::date, 'UBND huyện', 'cong-van', 'Trích yếu thử', $4, $5,
		   'CB-00123', $6, $7, $8, $9)`,
		tenantID, id, no, time.Date(2026, 10, 1, 3, 0, 0, 0, time.UTC), status, holder,
		delAt, delBy, delReason); err != nil {
		t.Fatalf("insert %s: %v", id, err)
	}
}

func TestPgIncomingHeldOpenByOrgUnit(t *testing.T) {
	db := moKetNoi(t)
	tenantA, tenantB := xaRieng(t)
	const unit, other = "bp-held-1", "bp-held-2"

	no := 0
	add := func(tenantID, id, status, holder string, deleted bool) {
		no++
		insertHeldIncoming(t, db, tenantID, no, id, status, holder, deleted)
	}
	// Counted: the three open statuses.
	add(tenantA, "vb-new", "moi-vao-so", unit, false)
	add(tenantA, "vb-assigned", "da-phan-cong", unit, false)
	add(tenantA, "vb-working", "dang-xu-ly", unit, false)
	// Not counted: each finished status, soft-deleted, another unit, no holder.
	add(tenantA, "vb-resolved", "da-giai-quyet", unit, false)
	add(tenantA, "vb-referred", "chuyen-cap-tren", unit, false)
	add(tenantA, "vb-filed", "luu-khong-thu-ly", unit, false)
	add(tenantA, "vb-deleted", "dang-xu-ly", unit, true)
	add(tenantA, "vb-other", "dang-xu-ly", other, false)
	add(tenantA, "vb-none", "moi-vao-so", "", false)
	// Another commune, same unit id.
	add(tenantB, "vb-b", "dang-xu-ly", unit, false)

	s := NewVanBanDenStore(pkgstore.New(db))
	n, err := s.CountOpenHeldByOrgUnit(ctxXa(tenantA), unit)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("commune A: %d, want 3", n)
	}
	if n, err := s.CountOpenHeldByOrgUnit(ctxXa(tenantB), unit); err != nil || n != 1 {
		t.Errorf("commune B: n=%d err=%v, want 1", n, err)
	}
}
