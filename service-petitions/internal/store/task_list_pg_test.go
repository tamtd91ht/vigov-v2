package store

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/page"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// Integration tests for the TASK-01 additions (28/09/2026) against a REAL PostgreSQL.
//
// ⚠ THEY SKIP UNLESS VIGOV_TEST_DSN IS SET, AND THE PACKAGE STILL PRINTS `ok`. No PostgreSQL was
// reachable when these were written (DSN unset, Docker daemon down), so NONE of the assertions below
// has been executed. What the fake-driver suite (task_list_test.go) proves is the statement TEXT and
// the Go-side cursor; what only these can prove is that PostgreSQL accepts the derived table, the
// COALESCE keys and the scalar subquery, and pages them exactly.

// TestPgDueSortWalkAcrossNullBoundary pages a commune's register by `due_at`, two rows at a time, in
// both directions, and requires every task exactly once with the no-deadline tasks LAST — the NULL
// boundary that made this sort unshippable before the NOT NULL keys.
func TestPgDueSortWalkAcrossNullBoundary(t *testing.T) {
	db := moKetNoi(t)
	commune, _ := xaRieng(t)
	danhMucChoXa(t, db, commune)

	d1 := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	for _, r := range []struct {
		id, code string
		due      any
	}{
		{"t1", "NV01", d1}, {"t2", "NV02", nil}, {"t3", "NV03", d2},
		{"t4", "NV04", nil}, {"t5", "NV05", d1}, {"t6", "NV06", nil},
	} {
		if err := themNhiemVu(db, commune, commune[:10]+r.id, r.code, "theo-van-ban", "moi-giao",
			r.due, r.due, nil); err != nil {
			t.Fatalf("thêm %s: %v", r.code, err)
		}
	}
	s := NewNhiemVuStore(pkgstore.New(db))
	ctx := ctxXa(tenant.ID(commune))

	for order, want := range map[string]string{
		"asc":  "NV01,NV05,NV03,NV02,NV04,NV06",
		"desc": "NV03,NV05,NV01,NV06,NV04,NV02",
	} {
		t.Run(order, func(t *testing.T) {
			var (
				got    []string
				cursor string
			)
			for guard := 0; guard < 10; guard++ {
				q := url.Values{"sort": {"due_at"}, "order": {order}, "limit": {"2"}}
				if cursor != "" {
					q.Set("cursor", cursor)
				}
				yc, err := page.Parse(q, SapXepNhiemVu) // the handler's list; DanhSach switches the key
				if err != nil {
					t.Fatalf("page.Parse: %v", err)
				}
				kq, err := s.DanhSach(ctx, LocNhiemVu{}, yc)
				if err != nil {
					t.Fatalf("DanhSach: %v", err)
				}
				for _, n := range kq.Items {
					got = append(got, n.Ma)
				}
				if !kq.HasMore {
					break
				}
				cursor = kq.NextCursor
			}
			// Ties (NV01/NV05 on d1; the three no-deadline tasks) are broken by the internal id, which
			// the fixture orders the same way as the codes.
			if strings.Join(got, ",") != want {
				t.Errorf("qua các trang = %v, muốn %s", got, want)
			}
		})
	}
}

// TestPgTreeFactsAndParentFilter — `parent=` lists the LIVE direct children, `child_count` counts
// only those, and `parent` comes back as the register number; a soft-deleted child is in neither,
// and another commune's number resolves to nothing.
func TestPgTreeFactsAndParentFilter(t *testing.T) {
	db := moKetNoi(t)
	commune, otherCommune := xaRieng(t)
	danhMucChoXa(t, db, commune)
	danhMucChoXa(t, db, otherCommune)

	rowID := func(c, s string) string { return c[:10] + s }
	for _, r := range []struct{ commune, id, code string }{
		{commune, "p", "NV10"}, {commune, "c1", "NV11"}, {commune, "c2", "NV12"}, {commune, "c3", "NV13"},
		{otherCommune, "p", "NV10"},
	} {
		if err := themNhiemVu(db, r.commune, rowID(r.commune, r.id), r.code, "theo-van-ban", "moi-giao",
			nil, nil, nil); err != nil {
			t.Fatalf("thêm %s: %v", r.code, err)
		}
	}
	for _, c := range []string{"c1", "c2", "c3"} {
		if _, err := db.Exec(`UPDATE nhiem_vu SET nhiem_vu_cha_id = $1 WHERE tenant_id = $2 AND id = $3`,
			rowID(commune, "p"), commune, rowID(commune, c)); err != nil {
			t.Fatalf("gắn cha: %v", err)
		}
	}
	if _, err := db.Exec(`UPDATE nhiem_vu SET deleted_at = now(), deleted_by = 'CB-00123',
		delete_reason = 'kiểm thử' WHERE tenant_id = $1 AND id = $2`, commune, rowID(commune, "c3")); err != nil {
		t.Fatalf("xoá mềm: %v", err)
	}

	s := NewNhiemVuStore(pkgstore.New(db))
	ctx := ctxXa(tenant.ID(commune))
	yc, _ := page.Parse(url.Values{"sort": {"code"}, "order": {"asc"}}, SapXepNhiemVu)

	kq, err := s.DanhSach(ctx, LocNhiemVu{ParentCode: "NV10"}, yc)
	if err != nil {
		t.Fatal(err)
	}
	if len(kq.Items) != 2 || kq.Items[0].Ma != "NV11" || kq.Items[1].Ma != "NV12" {
		t.Fatalf("việc con của NV10 = %+v, muốn NV11, NV12 (NV13 đã xoá mềm)", kq.Items)
	}
	for _, n := range kq.Items {
		if n.ParentCode != "NV10" {
			t.Errorf("%s: parent = %q, muốn NV10", n.Ma, n.ParentCode)
		}
	}
	p, err := s.TheoMa(ctx, "NV10")
	if err != nil {
		t.Fatal(err)
	}
	if p.ChildCount != 2 {
		t.Errorf("child_count của NV10 = %d, muốn 2 (việc con đã xoá mềm không tính)", p.ChildCount)
	}

	counts, err := s.CountByStatus(ctx, LocNhiemVu{ParentCode: "NV10"})
	if err != nil {
		t.Fatal(err)
	}
	if counts[domain.MoiGiao] != 2 {
		t.Errorf("đếm theo trạng thái dưới NV10 = %v, muốn moi-giao: 2", counts)
	}

	// The other commune has its own NV10 with no children: its register does not see ours.
	kq, err = s.DanhSach(ctxXa(tenant.ID(otherCommune)), LocNhiemVu{ParentCode: "NV10"}, yc)
	if err != nil {
		t.Fatal(err)
	}
	if len(kq.Items) != 0 {
		t.Errorf("xã khác thấy %d việc con của NV10 xã này", len(kq.Items))
	}
}

// TestPgDeletedParentIsNeitherAFilterNorAName — the two soft-delete edges of the tree facts.
//
//	?parent=<a soft-deleted task's number>   an EMPTY page, even though live rows still point at it:
//	                                         the number resolves among LIVE tasks only (rule 7 inv. 2)
//	a live child under a deleted parent      `parent: ""`, never the removed task's number (the state
//	                                         ADR 0037 decision 3 keeps the write path from creating,
//	                                         written here straight into the table)
func TestPgDeletedParentIsNeitherAFilterNorAName(t *testing.T) {
	db := moKetNoi(t)
	commune, _ := xaRieng(t)
	danhMucChoXa(t, db, commune)

	rowID := func(s string) string { return commune[:10] + s }
	for _, r := range []struct{ id, code string }{{"p", "NV20"}, {"c", "NV21"}} {
		if err := themNhiemVu(db, commune, rowID(r.id), r.code, "theo-van-ban", "moi-giao", nil, nil, nil); err != nil {
			t.Fatalf("thêm %s: %v", r.code, err)
		}
	}
	if _, err := db.Exec(`UPDATE nhiem_vu SET nhiem_vu_cha_id = $1 WHERE tenant_id = $2 AND id = $3`,
		rowID("p"), commune, rowID("c")); err != nil {
		t.Fatalf("gắn cha: %v", err)
	}
	if _, err := db.Exec(`UPDATE nhiem_vu SET deleted_at = now(), deleted_by = 'CB-00123',
		delete_reason = 'kiểm thử' WHERE tenant_id = $1 AND id = $2`, commune, rowID("p")); err != nil {
		t.Fatalf("xoá mềm cha: %v", err)
	}

	s := NewNhiemVuStore(pkgstore.New(db))
	ctx := ctxXa(tenant.ID(commune))
	yc, _ := page.Parse(url.Values{}, SapXepNhiemVu)

	kq, err := s.DanhSach(ctx, LocNhiemVu{ParentCode: "NV20"}, yc)
	if err != nil {
		t.Fatal(err)
	}
	if len(kq.Items) != 0 {
		t.Errorf("?parent= mã của việc ĐÃ XOÁ trả %d dòng, muốn trang rỗng", len(kq.Items))
	}

	child, err := s.TheoMa(ctx, "NV21")
	if err != nil {
		t.Fatal(err)
	}
	if child.ParentCode != "" {
		t.Errorf("việc con của cha đã xoá mang parent = %q, muốn rỗng", child.ParentCode)
	}
}

// TestPgQueueTaskFilterStaysInItsCommune — `?task=` with a number that exists ONLY in another commune
// is an empty page, not that commune's pending request: the joined task is bound to $1.
func TestPgQueueTaskFilterStaysInItsCommune(t *testing.T) {
	db := moKetNoi(t)
	commune, otherCommune := xaRieng(t)
	danhMucChoXa(t, db, commune)
	danhMucChoXa(t, db, otherCommune)

	due := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	taskID := otherCommune[:10] + "t"
	if err := themNhiemVu(db, otherCommune, taskID, "NV30", "theo-van-ban", "dang-thuc-hien", due, due, nil); err != nil {
		t.Fatalf("thêm nhiệm vụ xã khác: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO de_nghi_lui_han
		 (tenant_id, id, nhiem_vu_id, nguoi_de_nghi_ma, han_moi, ly_do, trang_thai, thoi_diem)
		 VALUES ($1, $2, $3, 'CB-00311', $4, 'Chờ số liệu.', 'cho-duyet', now())`,
		otherCommune, otherCommune[:10]+"dn", taskID, due.Add(7*24*time.Hour)); err != nil {
		t.Fatalf("thêm đề nghị xã khác: %v", err)
	}

	s := NewDeNghiLuiHanStore(pkgstore.New(db))
	yc, _ := page.Parse(url.Values{}, SapXepDeNghiChoDuyet)

	kq, err := s.ChoDuyet(ctxXa(tenant.ID(commune)), LocDeNghiChoDuyet{TaskCode: "NV30"}, yc)
	if err != nil {
		t.Fatal(err)
	}
	if len(kq.Items) != 0 {
		t.Errorf("?task= mã của xã khác trả %d đề nghị — rò rỉ giữa hai xã", len(kq.Items))
	}
	// The other direction, so the case above cannot pass on an empty table: the owning commune sees it.
	kq, err = s.ChoDuyet(ctxXa(tenant.ID(otherCommune)), LocDeNghiChoDuyet{TaskCode: "NV30"}, yc)
	if err != nil {
		t.Fatal(err)
	}
	if len(kq.Items) != 1 {
		t.Errorf("xã sở hữu thấy %d đề nghị của NV30, muốn 1", len(kq.Items))
	}
}
