package store

import (
	"database/sql"
	"testing"
	"time"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// Against a real PostgreSQL: CountOpenHeldByOrgUnit counts exactly the rows the contract says HOLD
// the unit and are OPEN — every excluded status present to be wrongly counted, a soft-deleted row,
// a row of another commune naming the same unit, and tasks naming the unit ONLY in the retired
// `co_quan_chu_tri_id` (ADR 0065 NV5: not read, so not counted). The helper still WRITES that column
// with raw SQL: it is how a pre-0025 row sits on disk, and the point is that the store ignores it.
//
// ⚠ SKIPS WITHOUT VIGOV_TEST_DSN, and the package still prints `ok`. The harness (TestMain,
// moKetNoi, xaRieng) lives in danh_muc_nhiem_vu_pg_test.go.

func insertHeldPetition(t *testing.T, db *sql.DB, tenantID, id, status, unit string, deleted bool) {
	t.Helper()
	origin := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	var delAt, delBy, delReason any
	if deleted {
		delAt, delBy, delReason = time.Now().UTC(), "CB-00123", "vào sổ nhầm"
	}
	if _, err := db.Exec(
		`INSERT INTO phieu_phan_anh
		 (tenant_id, id, ma_tra_cuu, kenh_tiep_nhan, noi_dung, trang_thai, bo_phan_id,
		  goc_dem_han, vao_so_luc, deleted_at, deleted_by, delete_reason)
		 VALUES ($1,$2,$3,'web-xa',$4,$5,$6,$7,$7,$8,$9,$10)`,
		tenantID, id, "PA-HELD-"+id, "Nội dung phản ánh dùng cho phép kiểm.", status, nullHoac(unit),
		origin, delAt, delBy, delReason); err != nil {
		t.Fatalf("insert petition %s: %v", id, err)
	}
}

func insertHeldTask(t *testing.T, db *sql.DB, tenantID, id, status, unit, retiredLeadUnit string, deleted bool) {
	t.Helper()
	var doneAt, delAt, delBy, delReason any
	if status == "hoan-thanh" {
		doneAt = mocXongPg // the CHECK nhiem_vu_hoan_thanh_co_ngay ties the date to the status
	}
	if deleted {
		delAt, delBy, delReason = time.Now().UTC(), "CB-00123", "giao nhầm"
	}
	if _, err := db.Exec(
		`INSERT INTO nhiem_vu
		 (tenant_id, id, ma, loai, tieu_de, trang_thai, nguon_giao, han_xu_ly, han_ban_dau,
		  ngay_hoan_thanh, tien_do, nguoi_tao_ma, bo_phan_id, co_quan_chu_tri_id,
		  deleted_at, deleted_by, delete_reason)
		 VALUES ($1,$2,$3,'theo-van-ban','Nhiệm vụ dùng cho phép kiểm.',$4,'truc-tiep',$5,$5,
		  $6,0,'CB-00123',$7,$8,$9,$10,$11)`,
		tenantID, id, "NV-HELD-"+id, status, mocHanGocPg, doneAt, nullHoac(unit), nullHoac(retiredLeadUnit),
		delAt, delBy, delReason); err != nil {
		t.Fatalf("insert task %s: %v", id, err)
	}
}

func TestPgPetitionsHeldOpenByOrgUnit(t *testing.T) {
	db := moKetNoi(t)
	tenantA, tenantB := xaRieng(t)
	const unit, other = "bp-held-1", "bp-held-2"

	// OPEN, held — counted: every non-final status, including the two "done, not closed".
	for _, s := range []string{"da-tiep-nhan", "dang-phan-loai", "da-chuyen-xu-ly", "dang-xu-ly",
		"da-xu-ly", "cho-dan-xac-nhan"} {
		insertHeldPetition(t, db, tenantA, "pa-open-"+s, s, unit, false)
	}
	// NOT counted: each of the three endings, a soft-deleted open row, another unit, no unit.
	insertHeldPetition(t, db, tenantA, "pa-closed", "da-dong", unit, false)
	insertHeldPetition(t, db, tenantA, "pa-declined", "khong-tiep-nhan", unit, false)
	insertHeldPetition(t, db, tenantA, "pa-referred", "chuyen-cap-tren", unit, false)
	insertHeldPetition(t, db, tenantA, "pa-deleted", "dang-xu-ly", unit, true)
	insertHeldPetition(t, db, tenantA, "pa-other", "dang-xu-ly", other, false)
	insertHeldPetition(t, db, tenantA, "pa-none", "da-tiep-nhan", "", false)
	// Another commune, same unit id, open: counted in A only if isolation is broken.
	insertHeldPetition(t, db, tenantB, "pa-b", "dang-xu-ly", unit, false)

	s := NewPhieuPhanAnhStore(pkgstore.New(db))
	n, err := s.CountOpenHeldByOrgUnit(ctxXa(tenant.ID(tenantA)), unit)
	if err != nil {
		t.Fatal(err)
	}
	if n != 6 {
		t.Errorf("xã A: %d phiếu, muốn 6", n)
	}
	if n, err := s.CountOpenHeldByOrgUnit(ctxXa(tenant.ID(tenantB)), unit); err != nil || n != 1 {
		t.Errorf("xã B: n=%d err=%v, muốn 1", n, err)
	}
	if n, err := s.CountOpenHeldByOrgUnit(ctxXa(tenant.ID(tenantA)), "bp-unknown"); err != nil || n != 0 {
		t.Errorf("mã lạ: n=%d err=%v, muốn 0", n, err)
	}
}

func TestPgTasksHeldOpenByOrgUnit(t *testing.T) {
	db := moKetNoi(t)
	tenantA, tenantB := xaRieng(t)
	danhMucChoXa(t, db, tenantA)
	danhMucChoXa(t, db, tenantB)
	const unit, other = "bp-held-1", "bp-held-2"

	// Counted: open, held through bo_phan_id — once, whatever the retired column says.
	insertHeldTask(t, db, tenantA, "nv-unit", "dang-thuc-hien", unit, "", false)
	insertHeldTask(t, db, tenantA, "nv-both", "da-tiep-nhan", unit, unit, false)
	insertHeldTask(t, db, tenantA, "nv-paused", "tam-dung", unit, "", false)
	insertHeldTask(t, db, tenantA, "nv-review", "cho-duyet", unit, "", false)
	// Not counted: the unit ONLY in the retired lead-unit column (ADR 0065 NV5), finished, soft-deleted,
	// another unit only.
	insertHeldTask(t, db, tenantA, "nv-lead", "moi-giao", other, unit, false)
	insertHeldTask(t, db, tenantA, "nv-forwarded", "chuyen-tiep", "", unit, false)
	insertHeldTask(t, db, tenantA, "nv-done", "hoan-thanh", unit, "", false)
	insertHeldTask(t, db, tenantA, "nv-done-lead", "hoan-thanh", "", unit, false)
	insertHeldTask(t, db, tenantA, "nv-deleted", "dang-thuc-hien", unit, unit, true)
	insertHeldTask(t, db, tenantA, "nv-other", "dang-thuc-hien", other, other, false)
	// Another commune, same unit id.
	insertHeldTask(t, db, tenantB, "nv-b", "dang-thuc-hien", unit, "", false)

	s := NewNhiemVuStore(pkgstore.New(db))
	n, err := s.CountOpenHeldByOrgUnit(ctxXa(tenant.ID(tenantA)), unit)
	if err != nil {
		t.Fatal(err)
	}
	if n != 4 {
		t.Errorf("xã A: %d nhiệm vụ, muốn 4 (chỉ bo_phan_id; co_quan_chu_tri_id đã nghỉ, không đếm)", n)
	}
	if n, err := s.CountOpenHeldByOrgUnit(ctxXa(tenant.ID(tenantB)), unit); err != nil || n != 1 {
		t.Errorf("xã B: n=%d err=%v, muốn 1", n, err)
	}
}
