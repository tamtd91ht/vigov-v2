package store

import (
	"net/url"
	"testing"
	"time"

	"github.com/vihat/vigov/core/page"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// Integration tests for `extension_count` / `pending_extension` and `roots=true` against a REAL
// PostgreSQL.
//
// ⚠ THESE SKIP UNLESS VIGOV_TEST_DSN IS SET, and the package still prints `ok` — see the header of
// nhiem_vu_pg_test.go. None of the assertions below has been executed in the environment they were
// written in. task_extension_facts_test.go proves the statement text and the mapping; only these prove
// PostgreSQL accepts the FILTER items and that the count agrees with ApprovedCount (migration 0016).

// fileExtension inserts one request against a task, decided when status is not pending, soft-deleted
// when asked. Every value is invented (rule 3, invariant 5).
func fileExtension(t *testing.T, commune, id, taskID, status string, deleted bool) {
	t.Helper()
	db := moKetNoi(t)
	due := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	if status == "cho-duyet" {
		if _, err := db.Exec(`INSERT INTO de_nghi_lui_han
			 (tenant_id, id, nhiem_vu_id, nguoi_de_nghi_ma, han_moi, ly_do, trang_thai, thoi_diem)
			 VALUES ($1, $2, $3, 'CB-00311', $4, 'Chờ số liệu.', 'cho-duyet', now())`,
			commune, id, taskID, due); err != nil {
			t.Fatalf("thêm đề nghị chờ: %v", err)
		}
	} else {
		if _, err := db.Exec(`INSERT INTO de_nghi_lui_han
			 (tenant_id, id, nhiem_vu_id, nguoi_de_nghi_ma, nguoi_duyet_ma, han_moi, ly_do, trang_thai,
			  thoi_diem, duyet_luc)
			 VALUES ($1, $2, $3, 'CB-00311', 'CB-00007', $4, 'Chờ số liệu.', $5, now(), now())`,
			commune, id, taskID, due, status); err != nil {
			t.Fatalf("thêm đề nghị %s: %v", status, err)
		}
	}
	if deleted {
		if _, err := db.Exec(`UPDATE de_nghi_lui_han SET deleted_at = now(), deleted_by = 'CB-00123',
			delete_reason = 'kiểm thử' WHERE tenant_id = $1 AND id = $2`, commune, id); err != nil {
			t.Fatalf("xoá mềm đề nghị: %v", err)
		}
	}
}

// TestPgExtensionFactsAgreeWithTheTrigger — a task with a live approval, a SOFT-DELETED approval, a
// rejection and a live pending request reads extension_count = 2 (= ApprovedCount, the trigger's
// predicate) and pending = true; a withdrawn pending request is not pending; another commune's
// requests on a colliding task id are not counted.
func TestPgExtensionFactsAgreeWithTheTrigger(t *testing.T) {
	db := moKetNoi(t)
	commune, otherCommune := xaRieng(t)
	danhMucChoXa(t, db, commune)
	danhMucChoXa(t, db, otherCommune)

	due := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	id := func(c, s string) string { return c[:10] + s }
	for _, r := range []struct{ commune, id, code string }{
		{commune, "t1", "NV40"}, {commune, "t2", "NV41"}, {commune, "t3", "NV42"}, {otherCommune, "t1", "NV40"},
	} {
		if err := themNhiemVu(db, r.commune, id(r.commune, r.id), r.code, "theo-van-ban", "dang-thuc-hien",
			due, due, nil); err != nil {
			t.Fatalf("thêm %s: %v", r.code, err)
		}
	}
	t1 := id(commune, "t1")
	fileExtension(t, commune, id(commune, "e1"), t1, "da-duyet", false)
	fileExtension(t, commune, id(commune, "e2"), t1, "da-duyet", true)
	fileExtension(t, commune, id(commune, "e3"), t1, "tu-choi", false)
	fileExtension(t, commune, id(commune, "e4"), t1, "cho-duyet", false)
	fileExtension(t, commune, id(commune, "e5"), id(commune, "t2"), "cho-duyet", true) // withdrawn
	// The other commune: approvals on ITS t1, whose id differs — and must never reach ours.
	fileExtension(t, otherCommune, id(otherCommune, "e1"), id(otherCommune, "t1"), "da-duyet", false)

	kho := pkgstore.New(db)
	s := NewNhiemVuStore(kho)
	ctx := ctxXa(tenant.ID(commune))
	yc, _ := page.Parse(url.Values{"sort": {"code"}, "order": {"asc"}}, SapXepNhiemVu)
	kq, err := s.DanhSach(ctx, LocNhiemVu{}, yc)
	if err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	want := map[string]struct {
		n       int
		pending bool
	}{"NV40": {2, true}, "NV41": {0, false}, "NV42": {0, false}}
	if len(kq.Items) != 3 {
		t.Fatalf("trang = %d dòng, muốn 3", len(kq.Items))
	}
	for _, n := range kq.Items {
		if w := want[n.Ma]; n.ExtensionCount != w.n || n.PendingExtension != w.pending {
			t.Errorf("%s: extension_count=%d pending=%v, muốn %d %v", n.Ma, n.ExtensionCount, n.PendingExtension, w.n, w.pending)
		}
	}

	// The single read carries the same, and agrees with ApprovedCount — the trigger's own predicate.
	one, err := s.TheoMa(ctx, "NV40")
	if err != nil {
		t.Fatal(err)
	}
	if err := kho.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		approved, err := NewDeNghiLuiHanStore(kho).ApprovedCount(ctx, tx, t1)
		if err != nil {
			return err
		}
		if approved != one.ExtensionCount {
			t.Errorf("extension_count = %d, ApprovedCount = %d — hai con số của một sự thật lệch nhau",
				one.ExtensionCount, approved)
		}
		// And the write-reply path, inside the transaction (AttachTreeFactsTx).
		ds := []domain.NhiemVu{{ID: t1}}
		if err := s.AttachTreeFactsTx(ctx, tx, ds); err != nil {
			return err
		}
		if ds[0].ExtensionCount != 2 || !ds[0].PendingExtension {
			t.Errorf("phản hồi ghi: extension_count=%d pending=%v, muốn 2 true", ds[0].ExtensionCount, ds[0].PendingExtension)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// TestPgRootsFilter — `roots=true` lists the tasks with no parent, in the list and the per-status count.
func TestPgRootsFilter(t *testing.T) {
	db := moKetNoi(t)
	commune, _ := xaRieng(t)
	danhMucChoXa(t, db, commune)

	id := func(s string) string { return commune[:10] + s }
	for _, r := range []struct{ id, code string }{{"p", "NV50"}, {"c", "NV51"}, {"r", "NV52"}} {
		if err := themNhiemVu(db, commune, id(r.id), r.code, "theo-van-ban", "moi-giao", nil, nil, nil); err != nil {
			t.Fatalf("thêm %s: %v", r.code, err)
		}
	}
	if _, err := db.Exec(`UPDATE nhiem_vu SET nhiem_vu_cha_id = $1 WHERE tenant_id = $2 AND id = $3`,
		id("p"), commune, id("c")); err != nil {
		t.Fatalf("gắn cha: %v", err)
	}

	s := NewNhiemVuStore(pkgstore.New(db))
	ctx := ctxXa(tenant.ID(commune))
	yc, _ := page.Parse(url.Values{"sort": {"code"}, "order": {"asc"}}, SapXepNhiemVu)
	kq, err := s.DanhSach(ctx, LocNhiemVu{Roots: true}, yc)
	if err != nil {
		t.Fatal(err)
	}
	if len(kq.Items) != 2 || kq.Items[0].Ma != "NV50" || kq.Items[1].Ma != "NV52" {
		t.Errorf("việc gốc = %+v, muốn NV50, NV52", kq.Items)
	}
	counts, err := s.CountByStatus(ctx, LocNhiemVu{Roots: true})
	if err != nil {
		t.Fatal(err)
	}
	if counts[domain.MoiGiao] != 2 {
		t.Errorf("đếm việc gốc = %v, muốn moi-giao: 2", counts)
	}
	// Due-sort shape (the aliased derived table) reads the same unqualified column.
	ycDue, _ := page.Parse(url.Values{"sort": {"due_at"}}, SapXepNhiemVu)
	if kq, err := s.DanhSach(ctx, LocNhiemVu{Roots: true}, ycDue); err != nil || len(kq.Items) != 2 {
		t.Errorf("việc gốc theo hạn = %d dòng, lỗi %v — muốn 2", len(kq.Items), err)
	}
}
