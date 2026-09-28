package store

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// Tests for the task register additions of 28/09/2026 (TASK-01): `sort=due_at` across the NULL
// boundary, `?parent=`, the per-status count, the two tree facts, and the queue's `task` filter.
//
//	PROVED HERE   the due-date sort never names the nullable column, puts no-deadline tasks LAST in
//	              both directions, and its cursor survives the wire round trip ON THE SENTINEL rows ·
//	              a key/direction mismatch is refused before any statement · page two across the
//	              boundary binds the sentinel as the anchor · the tree facts are two statements for a
//	              whole page, bound to the commune, mapped back correctly, and a failure is returned ·
//	              `parent` and `task` are bound parameters resolved inside the statement · the count
//	              uses the SAME predicate builder as the list.
//
//	NOT PROVED    what PostgreSQL does with the COALESCE key, the derived table and the scalar
//	              subquery. That is TestPgDueSortWalkAcrossNullBoundary and
//	              TestPgTreeFactsAndParentFilter in task_list_pg_test.go, which SKIP without
//	              VIGOV_TEST_DSN — and no DSN was reachable when this was written.

// --- sort=due_at ------------------------------------------------------------------------------------

func dueTask(id string, due time.Time) domain.NhiemVu {
	return domain.NhiemVu{ID: id, Ma: strings.ToUpper(id), HanXuLy: due}
}

// TestDueSortWalkAcrossNullBoundary walks a register of seven tasks — three without a deadline, two
// sharing one — two rows per page, in BOTH directions, with the SAME anchor readers the store builds
// its cursor from, and every anchor passed through the real cursor encoding (page.Encode/Decode).
//
// What it pins: every task appears exactly once, the tasks without a deadline are all at the END,
// and the anchor of a no-deadline row comes back from the wire as the very sentinel the SQL uses.
// The row comparison is the one QueryPage writes, `(key, id) > (…)` ascending and `<` descending —
// over a key that is never NULL, which is the whole point.
func TestDueSortWalkAcrossNullBoundary(t *testing.T) {
	d1 := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	d3 := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	rows := []domain.NhiemVu{
		dueTask("t1", d1), dueTask("t2", time.Time{}), dueTask("t3", d2), dueTask("t4", time.Time{}),
		dueTask("t5", d1), dueTask("t6", time.Time{}), dueTask("t7", d3),
	}

	for _, tc := range []struct {
		order string
		dir   page.Dir
		key   func(domain.NhiemVu) page.Key
		want  []string
	}{
		{"asc", page.Asc, dueAscKey, []string{"t1", "t5", "t3", "t7", "t2", "t4", "t6"}},
		{"desc", page.Desc, dueDescKey, []string{"t7", "t3", "t5", "t1", "t6", "t4", "t2"}},
	} {
		t.Run(tc.order, func(t *testing.T) {
			wantColumn := map[string]string{"asc": dueSortAscColumn, "desc": dueSortDescColumn}[tc.order]

			before := func(a, b domain.NhiemVu) bool { // a sorts before b
				ka, kb := tc.key(a).Time(), tc.key(b).Time()
				if !ka.Equal(kb) {
					if tc.dir == page.Asc {
						return ka.Before(kb)
					}
					return ka.After(kb)
				}
				if tc.dir == page.Asc {
					return a.ID < b.ID
				}
				return a.ID > b.ID
			}
			ordered := append([]domain.NhiemVu(nil), rows...)
			sort.Slice(ordered, func(i, j int) bool { return before(ordered[i], ordered[j]) })

			var (
				got    []string
				cursor string
			)
			for guard := 0; guard < 10; guard++ {
				// THE PRODUCTION PATH, END TO END: the handler parses against SapXepNhiemVu (the list
				// tools/apidoc publishes), DanhSach rebuilds the request for its direction, and the
				// next cursor is what QueryPage would encode from the rebuilt column.
				q := url.Values{"sort": {"due_at"}, "order": {tc.order}}
				if cursor != "" {
					q.Set("cursor", cursor)
				}
				parsed, err := page.Parse(q, SapXepNhiemVu)
				if err != nil {
					t.Fatalf("page.Parse (trang %d): %v — con trỏ trang trước không đọc lại được", guard+1, err)
				}
				yc, err := forDueDirection(parsed)
				if err != nil {
					t.Fatalf("forDueDirection: %v", err)
				}
				if yc.Column().SQL != wantColumn || yc.Dir() != tc.dir {
					t.Fatalf("khoá sắp xếp = %s %s, muốn %s %s", yc.Column().SQL, yc.Dir(), wantColumn, tc.dir)
				}
				var anchor *page.Anchor
				if a, ok := yc.After(); ok {
					anchor = &a
				}

				var pageRows []domain.NhiemVu
				for _, r := range ordered {
					if anchor != nil && !before(domain.NhiemVu{ID: anchor.ID, HanXuLy: keyToDeadline(anchor.Key.Time(), tc.dir)}, r) {
						continue
					}
					pageRows = append(pageRows, r)
					if len(pageRows) == 2 {
						break
					}
				}
				if len(pageRows) == 0 {
					break
				}
				for _, r := range pageRows {
					got = append(got, r.ID)
				}
				last := pageRows[len(pageRows)-1]
				// THROUGH THE WIRE: the next loop decodes this, via page.Parse, exactly as a client's
				// replayed cursor is decoded.
				cursor = page.Encode(yc.Column(), tc.dir, page.Anchor{Key: tc.key(last), ID: last.ID})
			}

			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("thứ tự qua các trang = %v, muốn %v (việc không có hạn phải ở CUỐI, không mất, không lặp)",
					got, tc.want)
			}
		})
	}
}

// keyToDeadline turns a decoded anchor key back into the deadline a row would carry, so the walk
// above can reuse its own comparator: the sentinel means "no deadline".
func keyToDeadline(k time.Time, dir page.Dir) time.Time {
	if (dir == page.Asc && k.Equal(dueSortAscNoDeadline)) || (dir == page.Desc && k.Equal(dueSortDescNoDeadline)) {
		return time.Time{}
	}
	return k
}

// TestDueSortSentinelsMatchTheSQL — the Go sentinel and the SQL literal are one instant. A second off
// would put the first no-deadline row of page two on the wrong side of the anchor.
func TestDueSortSentinelsMatchTheSQL(t *testing.T) {
	for _, s := range []time.Time{dueSortAscNoDeadline, dueSortDescNoDeadline} {
		lit := timestamptzLiteral(s)
		if !strings.Contains(taskByDueTable, lit) {
			t.Errorf("bảng sắp theo hạn không chứa mốc %s", lit)
		}
		raw := strings.TrimSuffix(strings.TrimPrefix(lit, "TIMESTAMPTZ '"), "+00'")
		back, err := time.Parse("2006-01-02 15:04:05", raw)
		if err != nil || !back.Equal(s) {
			t.Errorf("mốc %v viết thành %q, đọc lại %v (%v)", s, lit, back, err)
		}
	}
	// THE DESCENDING SENTINEL IS THE ZERO time.Time, which is what a NULL deadline scans to — so the
	// reader needs no branch there, and a NULL cannot be told from "no deadline" by accident.
	if !dueSortDescNoDeadline.IsZero() {
		t.Error("mốc giảm dần không phải time.Time rỗng")
	}
	// Both are outside any deadline a commune can set.
	someDeadline := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	if !dueSortAscNoDeadline.After(someDeadline.AddDate(1000, 0, 0)) ||
		!dueSortDescNoDeadline.Before(someDeadline.AddDate(-1000, 0, 0)) {
		t.Error("mốc lọt vào khoảng hạn có thật")
	}
}

func runDueSortPage(t *testing.T, q url.Values) lenhGia {
	t.Helper()
	k := &khoGia{hangTheoCot: []map[string]driver.Value{dongNhiemVu(nil)}}
	s := NewNhiemVuStore(store.New(moKhoGia(k)))
	// SapXepNhiemVu, as the handler parses: the switch to the ascending key is DanhSach's.
	yc, err := page.Parse(q, SapXepNhiemVu)
	if err != nil {
		t.Fatalf("page.Parse: %v", err)
	}
	if _, err := s.DanhSach(ctxXa(xaThu), LocNhiemVu{}, yc); err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	return k.lenh[0]
}

func TestDueSortStatementUsesTheNotNullKey(t *testing.T) {
	for order, want := range map[string]string{
		"asc":  "ORDER BY due_sort_asc ASC, id ASC",
		"desc": "ORDER BY due_sort_desc DESC, id DESC",
		"":     "ORDER BY due_sort_desc DESC, id DESC", // the allowlist's default direction
	} {
		t.Run("order="+order, func(t *testing.T) {
			q := url.Values{"sort": {"due_at"}}
			if order != "" {
				q.Set("order", order)
			}
			l := runDueSortPage(t, q)
			if !strings.Contains(l.sql, want) {
				t.Errorf("thiếu %q: %q", want, l.sql)
			}
			// The raw column is never the ORDER BY key.
			if strings.Contains(l.sql, "ORDER BY han_xu_ly") {
				t.Errorf("sắp thẳng theo cột NULL được: %q", l.sql)
			}
			// The commune is bound INSIDE the derived table as well as outside, and removed rows stay out.
			if !strings.Contains(l.sql, "FROM nhiem_vu WHERE tenant_id = $1) AS nv WHERE tenant_id = $1") ||
				!strings.Contains(l.sql, "deleted_at IS NULL") {
				t.Errorf("bảng dẫn xuất không buộc xã ở cả hai tầng hoặc thiếu loại dòng xoá: %q", l.sql)
			}
			if l.args[0] != string(xaThu) {
				t.Errorf("$1 = %v, muốn xã từ context", l.args[0])
			}
		})
	}
}

// TestDueSortPageTwoFromANoDeadlineAnchor — the cursor of a page that ENDED ON a task without a
// deadline anchors on the sentinel, bound as a parameter, so page two continues with the remaining
// no-deadline tasks by id instead of losing them.
func TestDueSortPageTwoFromANoDeadlineAnchor(t *testing.T) {
	for _, tc := range []struct {
		order    string
		dir      page.Dir
		sentinel time.Time
		op       string
	}{
		{"asc", page.Asc, dueSortAscNoDeadline, "(due_sort_asc, id) > ("},
		{"desc", page.Desc, dueSortDescNoDeadline, "(due_sort_desc, id) < ("},
	} {
		t.Run(tc.order, func(t *testing.T) {
			yc0, _ := page.Parse(url.Values{"sort": {"due_at"}, "order": {tc.order}}, SapXepNhiemVu)
			cursor := page.Encode(yc0.Column(), tc.dir, page.Anchor{Key: page.TimeKey(tc.sentinel), ID: "nv-042"})

			l := runDueSortPage(t, url.Values{"sort": {"due_at"}, "order": {tc.order}, "cursor": {cursor}})
			if !strings.Contains(l.sql, tc.op) {
				t.Fatalf("thiếu so sánh mốc %q: %q", tc.op, l.sql)
			}
			var (
				sawSentinel bool
				sawID       bool
			)
			for _, a := range l.args {
				if tm, ok := a.(time.Time); ok && tm.Equal(tc.sentinel) {
					sawSentinel = true
				}
				if a == "nv-042" {
					sawID = true
				}
			}
			if !sawSentinel || !sawID {
				t.Errorf("mốc trang 2 không mang (mốc thay NULL, id): %v", l.args)
			}
		})
	}
}

// TestDueSortRefusesAKeyOfTheOtherDirection — the ascending key driven DESCENDING would put every
// task without a deadline FIRST. No handler can build that request (they parse against
// SapXepNhiemVu, and forDueDirection only ever moves asc onto the asc key), so this is the floor
// under a future caller: refused before any statement runs.
func TestDueSortRefusesAKeyOfTheOtherDirection(t *testing.T) {
	for _, tc := range []struct {
		list  page.Allowlist
		order string
	}{
		{taskSortAscAllowlist, "desc"},
	} {
		k := &khoGia{hangTheoCot: []map[string]driver.Value{dongNhiemVu(nil)}}
		s := NewNhiemVuStore(store.New(moKhoGia(k)))
		yc, err := page.Parse(url.Values{"sort": {"due_at"}, "order": {tc.order}}, tc.list)
		if err != nil {
			t.Fatalf("page.Parse: %v", err)
		}
		if _, err := s.DanhSach(ctxXa(xaThu), LocNhiemVu{}, yc); !errors.Is(err, ErrTaskSortDirection) {
			t.Errorf("order=%s trên danh sách sai chiều: lỗi = %v, muốn ErrTaskSortDirection", tc.order, err)
		}
		if len(k.lenh) != 0 {
			t.Errorf("chạy %d câu lệnh dù khoá sắp xếp sai chiều", len(k.lenh))
		}
	}
}

// TestNonDueSortsKeepThePlainTable — only `due_at` goes through the derived table; the two sorts that
// were always there read `nhiem_vu` exactly as before, in both allowlists.
func TestNonDueSortsKeepThePlainTable(t *testing.T) {
	for _, q := range []url.Values{{}, {"sort": {"code"}, "order": {"asc"}}, {"sort": {"created_at"}}} {
		l := runDueSortPage(t, q)
		if !strings.Contains(l.sql, " FROM nhiem_vu WHERE tenant_id = $1") || strings.Contains(l.sql, "AS nv") {
			t.Errorf("%v: sai bảng: %q", q, l.sql)
		}
	}
}

// --- ?parent= ----------------------------------------------------------------------------------------

func TestParentFilterIsResolvedInsideTheStatement(t *testing.T) {
	l := chayDanhSachNhiemVu(t, LocNhiemVu{ParentCode: "NV19"})

	// THE CODE IS A BOUND PARAMETER, and the subquery that resolves it is bound to the commune and to
	// live rows — a number of another commune, or of a removed task, resolves to nothing.
	if strings.Contains(l.sql, "NV19") {
		t.Errorf("mã cha nằm TRONG câu lệnh: %q", l.sql)
	}
	if !coThamSoChuoi(l.args, "NV19") {
		t.Errorf("mã cha không có trong tham số: %v", l.args)
	}
	for _, want := range []string{"nhiem_vu_cha_id = (SELECT p.id FROM nhiem_vu p",
		"p.tenant_id = $1", "p.deleted_at IS NULL", "p.ma = $2"} {
		if !strings.Contains(l.sql, want) {
			t.Errorf("thiếu %q: %q", want, l.sql)
		}
	}
	// Absent, it adds nothing.
	if l := chayDanhSachNhiemVu(t, LocNhiemVu{}); strings.Contains(l.sql, "nhiem_vu_cha_id =") {
		t.Errorf("bộ lọc cha rỗng vẫn sinh điều kiện: %q", l.sql)
	}
}

// --- the per-status count --------------------------------------------------------------------------

func TestCountByStatusSharesTheListPredicate(t *testing.T) {
	loc := LocNhiemVu{Loai: "theo-van-ban", ParentCode: "NV19", ChiTreHan: true, Tim: "Hà Lam"}

	k := &khoGia{hangTheoCot: []map[string]driver.Value{
		{"trang_thai": "moi-giao", "count(*)": int64(3)},
		{"trang_thai": "dang-thuc-hien", "count(*)": int64(5)},
	}}
	s := NewNhiemVuStore(store.New(moKhoGia(k)))
	counts, err := s.CountByStatus(ctxXa(xaThu), loc)
	if err != nil {
		t.Fatalf("CountByStatus: %v", err)
	}
	if counts[domain.MoiGiao] != 3 || counts[domain.DangThucHien] != 5 || len(counts) != 2 {
		t.Errorf("số đếm = %v", counts)
	}
	if len(k.lenh) != 1 {
		t.Fatalf("chạy %d câu lệnh, muốn 1 — bảy con số phải cùng một thời điểm", len(k.lenh))
	}
	l := k.lenh[0]

	// THE SAME FILTER TEXT AND THE SAME BOUND VALUES AS THE LIST. Built once from the same function,
	// so the predicate part of the count statement must CONTAIN the list's predicate verbatim.
	listPredicate, listArgs := locNhiemVuThanhSQL(loc)
	if !strings.Contains(l.sql, "AND deleted_at IS NULL"+listPredicate+" GROUP BY trang_thai") {
		t.Errorf("câu đếm không dùng đúng điều kiện của danh sách:\n%q\nmuốn chứa %q", l.sql, listPredicate)
	}
	if l.args[0] != string(xaThu) || len(l.args) != len(listArgs)+1 {
		t.Errorf("tham số = %v, muốn [xã, %v]", l.args, listArgs)
	}
	if !strings.Contains(l.sql, "WHERE tenant_id = $1") {
		t.Errorf("câu đếm không buộc xã: %q", l.sql)
	}
}

func TestCountByStatusRefusesAnInvalidMetric(t *testing.T) {
	k := &khoGia{}
	s := NewNhiemVuStore(store.New(moKhoGia(k)))
	if _, err := s.CountByStatus(ctxXa(xaThu), LocNhiemVu{Metric: "khong-co"}); !errors.Is(err, ErrTaskMetricInvalid) {
		t.Errorf("lỗi = %v, muốn ErrTaskMetricInvalid", err)
	}
	if len(k.lenh) != 0 {
		t.Error("chạy câu đếm dù chỉ số không hợp lệ")
	}
}

// --- the two tree facts ------------------------------------------------------------------------------

// fakeTreeRows is a slice-backed treeRows.
type fakeTreeRows struct {
	rows [][]any
	i    int
}

func (r *fakeTreeRows) Next() bool { r.i++; return r.i <= len(r.rows) }
func (r *fakeTreeRows) Err() error { return nil }
func (r *fakeTreeRows) Close() error {
	return nil
}
func (r *fakeTreeRows) Scan(dest ...any) error {
	row := r.rows[r.i-1]
	for i, d := range dest {
		switch p := d.(type) {
		case *string:
			*p = row[i].(string)
		case *int64:
			*p = row[i].(int64)
		default:
			return fmt.Errorf("fake: kiểu đích %T", d)
		}
	}
	return nil
}

type treeCall struct {
	stmt string
	args []any
}

// fakeTree answers the two statements from a tiny register: parent id -> live child count, and
// id -> register number.
func fakeTree(calls *[]treeCall, children map[string]int64, codes map[string]string, fail error) treeQuery {
	return func(_ context.Context, stmt string, args ...any) (treeRows, error) {
		*calls = append(*calls, treeCall{stmt, args})
		if fail != nil {
			return nil, fail
		}
		out := &fakeTreeRows{}
		for _, a := range args {
			id := a.(string)
			if strings.Contains(stmt, "GROUP BY nhiem_vu_cha_id") {
				if n, ok := children[id]; ok {
					out.rows = append(out.rows, []any{id, n})
				}
			} else if code, ok := codes[id]; ok {
				out.rows = append(out.rows, []any{id, code})
			}
		}
		return out, nil
	}
}

// TestAttachTreeFactsIsTwoStatementsForAWholePage — the child-count batching (skills/load-data-once):
// a page of five tasks, three of them under two distinct parents, costs exactly TWO statements, each
// with one placeholder per DISTINCT value, and every row gets its own answer back.
func TestAttachTreeFactsIsTwoStatementsForAWholePage(t *testing.T) {
	ds := []domain.NhiemVu{
		{ID: "a", Ma: "NV01"},
		{ID: "b", Ma: "NV02", NhiemVuChaID: "a"},
		{ID: "c", Ma: "NV03", NhiemVuChaID: "a"},
		{ID: "d", Ma: "NV04", NhiemVuChaID: "p-off-page"},
		{ID: "e", Ma: "NV05"},
	}
	var calls []treeCall
	q := fakeTree(&calls,
		map[string]int64{"a": 2, "b": 1},                          // e has none, and is absent
		map[string]string{"a": "NV01", "p-off-page": "NV77"}, nil) // a parent NOT on the page

	if err := attachTreeFacts(context.Background(), q, ds); err != nil {
		t.Fatalf("attachTreeFacts: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("chạy %d câu lệnh cho 5 nhiệm vụ, muốn 2 (không N+1)", len(calls))
	}
	count, parent := calls[0], calls[1]
	if !strings.Contains(count.stmt, "nhiem_vu_cha_id IN ($2, $3, $4, $5, $6) GROUP BY nhiem_vu_cha_id") ||
		len(count.args) != 5 {
		t.Errorf("câu đếm việc con = %q %v", count.stmt, count.args)
	}
	if !strings.Contains(parent.stmt, "id IN ($2, $3)") || len(parent.args) != 2 {
		t.Errorf("câu mã việc cha phải mang đúng 2 cha KHÁC NHAU: %q %v", parent.stmt, parent.args)
	}
	for _, c := range calls {
		for _, want := range []string{"tenant_id = $1", "deleted_at IS NULL"} {
			if !strings.Contains(c.stmt, want) {
				t.Errorf("thiếu %q: %q", want, c.stmt)
			}
		}
	}

	want := map[string]struct {
		count  int
		parent string
	}{
		"a": {2, ""}, "b": {1, "NV01"}, "c": {0, "NV01"}, "d": {0, "NV77"}, "e": {0, ""},
	}
	for _, n := range ds {
		if w := want[n.ID]; n.ChildCount != w.count || n.ParentCode != w.parent {
			t.Errorf("%s: child_count=%d parent=%q, muốn %d %q", n.ID, n.ChildCount, n.ParentCode, w.count, w.parent)
		}
	}
}

func TestAttachTreeFactsSkipsWhatItNeedNotAsk(t *testing.T) {
	var calls []treeCall
	q := fakeTree(&calls, nil, nil, nil)

	if err := attachTreeFacts(context.Background(), q, nil); err != nil || len(calls) != 0 {
		t.Errorf("trang rỗng: %d câu, lỗi %v — muốn 0 câu", len(calls), err)
	}
	// Roots only: the count is asked, the parent statement is not (an empty IN list is not SQL).
	if err := attachTreeFacts(context.Background(), q, []domain.NhiemVu{{ID: "a"}, {ID: "b"}}); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || !strings.Contains(calls[0].stmt, "GROUP BY") {
		t.Errorf("toàn việc gốc: %d câu, muốn đúng 1 câu đếm", len(calls))
	}
}

// A failure is RETURNED, never answered with zeros — `child_count: 0` on a task with children would
// be a statement about the record made from a failure to read it.
func TestAttachTreeFactsReturnsAFailure(t *testing.T) {
	var calls []treeCall
	ds := []domain.NhiemVu{{ID: "a", NhiemVuChaID: "p"}}
	err := attachTreeFacts(context.Background(), fakeTree(&calls, nil, nil, errors.New("pg: down")), ds)
	if err == nil || !strings.Contains(err.Error(), "pg: down") {
		t.Errorf("lỗi = %v, muốn lỗi gốc được bọc", err)
	}
}

// --- the queue's `task` filter -------------------------------------------------------------------------

func TestQueueTaskFilterIsBoundAfterWhatIsAlreadyBound(t *testing.T) {
	yc, _ := page.Parse(url.Values{}, SapXepDeNghiChoDuyet)
	for _, tc := range []struct {
		loc       LocDeNghiChoDuyet
		want      string
		wantArgAt int
	}{
		{LocDeNghiChoDuyet{TaskCode: "NV19"}, "AND nhiem_vu_ma = $3", 2},
		{LocDeNghiChoDuyet{LanhDaoGiaoViecMa: "CB-00123", TaskCode: "NV19"}, "AND nhiem_vu_ma = $4", 3},
	} {
		k := &khoGia{}
		s := NewDeNghiLuiHanStore(store.New(moKhoGia(k)))
		if _, err := s.ChoDuyet(ctxXa(xaThu), tc.loc, yc); err != nil {
			t.Fatalf("ChoDuyet: %v", err)
		}
		l := k.lenh[0]
		if !strings.Contains(l.sql, tc.want) {
			t.Errorf("thiếu %q: %q", tc.want, l.sql)
		}
		if strings.Contains(l.sql, "NV19") || l.args[tc.wantArgAt] != "NV19" {
			t.Errorf("mã nhiệm vụ không phải tham số ở vị trí %d: %v — %q", tc.wantArgAt+1, l.args, l.sql)
		}
		// The joined task is still bound to the commune in the ON clause.
		if !strings.Contains(l.sql, "n.tenant_id = $1") {
			t.Errorf("phép nối nhiệm vụ không buộc xã: %q", l.sql)
		}
	}
}
