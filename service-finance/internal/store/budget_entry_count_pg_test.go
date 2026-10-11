package store

import (
	"testing"

	corestore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// Row 53 — the budget-line read's live-batch count against a real PostgreSQL: soft-deleted batches are
// not counted, and another commune's batch on a line with the SAME id is not counted either (rule 1 —
// the correlated count binds the batch side to $1, not to the line id alone).
//
// SKIPPED UNLESS VIGOV_TEST_DSN IS SET (TestMain, moKetNoi, xaRieng: hang_muc_ke_hoach_von_pg_test.go).

func TestBudgetLineReadCountsLiveEntriesOfThisCommuneOnly(t *testing.T) {
	db := moKetNoi(t)
	commune, otherCommune := xaRieng(t)

	addSheet(t, db, commune, "b-ec", "BC-EC", 2026, "chi")
	for _, line := range []string{"k-ec-1", "k-ec-2"} {
		if _, err := db.Exec(`INSERT INTO khoan_muc_ngan_sach (tenant_id, id, bang_id, ten, thu_tu)
			VALUES ($1, $2, 'b-ec', 'Khoản mục thử', 1)`, commune, line); err != nil {
			t.Fatalf("add line %s: %v", line, err)
		}
	}
	addEntry := func(tenantID, id, line string, removed bool) {
		t.Helper()
		if _, err := db.Exec(`INSERT INTO dot_thu_chi (tenant_id, id, khoan_muc_id, ngay, noi_dung, nguoi_ghi_ma)
			VALUES ($1, $2, $3, DATE '2026-03-01', 'Đợt thử', 'CB-00123')`, tenantID, id, line); err != nil {
			t.Fatalf("add entry %s: %v", id, err)
		}
		if removed {
			if _, err := db.Exec(`UPDATE dot_thu_chi SET deleted_at = now(), deleted_by = 'CB-00123',
				delete_reason = 'Ghi nhầm' WHERE tenant_id = $1 AND id = $2`, tenantID, id); err != nil {
				t.Fatalf("remove entry %s: %v", id, err)
			}
		}
	}
	addEntry(commune, "d-ec-1", "k-ec-1", false)
	addEntry(commune, "d-ec-2", "k-ec-1", false)
	addEntry(commune, "d-ec-3", "k-ec-1", true)       // soft-deleted: not counted
	addEntry(otherCommune, "d-ec-4", "k-ec-1", false) // another commune, same line id: not counted

	s := NewNganSachStore(corestore.New(db))
	lines, err := s.KhoanMucCuaBang(ctxXa(tenant.ID(commune)), "b-ec")
	if err != nil {
		t.Fatalf("read lines: %v", err)
	}
	got := map[string]int{}
	for _, k := range lines {
		got[k.ID] = k.EntryCount
	}
	if got["k-ec-1"] != 2 || got["k-ec-2"] != 0 || len(got) != 2 {
		t.Fatalf("entry counts = %v, want k-ec-1:2 k-ec-2:0", got)
	}
}
