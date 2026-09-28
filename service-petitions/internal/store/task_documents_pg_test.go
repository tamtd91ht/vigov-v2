package store

import (
	"testing"

	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// Integration test for DocumentsForTasks against the real migration 0009 schema.
//
// ⚠ SKIPS unless VIGOV_TEST_DSN is set — see nhiem_vu_van_ban_pg_test.go's header. What it pins, and
// no fake driver can: the IN list and ORDER BY are valid PostgreSQL, a soft-deleted line is excluded,
// and another commune's line on a task with the SAME internal id never joins the page.
func TestPgDocumentsForTasks(t *testing.T) {
	db := moKetNoi(t)
	xa, other := xaRieng(t)
	nhiemVuChoVanBan(t, db, xa, "nv-dt-1", "NV61") // sows xa's catalogue once
	if err := themNhiemVu(db, xa, "nv-dt-2", "NV62", "theo-van-ban", "dang-thuc-hien",
		mocHanGocPg, mocHanGocPg, nil); err != nil {
		t.Fatalf("thêm nhiệm vụ thứ hai: %v", err)
	}
	// The OTHER commune reuses the same internal id on purpose — the collision rule 1 guards.
	nhiemVuChoVanBan(t, db, other, "nv-dt-1", "NV61")

	for _, l := range []struct {
		xa, id, task, group string
		pos                 int
	}{
		{xa, "vb-dt-2", "nv-dt-1", "san-pham-dau-ra", 1},
		{xa, "vb-dt-1", "nv-dt-1", "cap-tren-giao", 1},
		{xa, "vb-dt-gone", "nv-dt-1", "cap-tren-giao", 2},
		{other, "vb-dt-x", "nv-dt-1", "cap-tren-giao", 1},
	} {
		if err := themVanBanNhiemVu(db, l.xa, l.id, l.task, l.group, "Một văn bản", l.pos, nil, nil); err != nil {
			t.Fatalf("thêm dòng %s: %v", l.id, err)
		}
	}
	if _, err := db.Exec(`UPDATE nhiem_vu_van_ban SET deleted_at = now(), deleted_by = 'CB-00001'
		WHERE tenant_id = $1 AND id = $2`, xa, "vb-dt-gone"); err != nil {
		t.Fatalf("gỡ mềm dòng: %v", err)
	}

	s := NewNhiemVuStore(pkgstore.New(db))
	got, err := s.DocumentsForTasks(ctxXa(tenant.ID(xa)), []string{"nv-dt-1", "nv-dt-2"})
	if err != nil {
		t.Fatalf("DocumentsForTasks: %v", err)
	}
	b := got["nv-dt-1"]
	if len(b) != 2 || b[0].ID != "vb-dt-1" || b[1].ID != "vb-dt-2" {
		t.Errorf("nv-dt-1 = %+v, muốn vb-dt-1 (cap-tren-giao) rồi vb-dt-2 — không dòng đã gỡ, không dòng xã khác", b)
	}
	if b2, ok := got["nv-dt-2"]; !ok || b2 == nil || len(b2) != 0 {
		t.Errorf("nv-dt-2 = %#v, muốn slice rỗng khác nil", b2)
	}
}
