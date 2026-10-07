package store

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// The two extension facts (task_extension_facts.go), driven through a fake treeQuery.
//
//	PROVED HERE   ONE statement per page whatever its size, one placeholder per DISTINCT task · an empty
//	              page runs nothing · the commune is $1 · the approved item is EXACTLY migration 0016's
//	              predicate (no `deleted_at`) · the pending item excludes soft-deleted rows · each row gets
//	              its own answer, absent = 0 / false · a failure is returned wrapped, never zeros ·
//	              attachTaskFacts runs the tree facts and then this, three statements for a page.
//	NOT PROVED    PostgreSQL's FILTER semantics — task_extension_facts_pg_test.go (skips without
//	              VIGOV_TEST_DSN).

// fakeExtensionQuery answers the extension statement from task id -> (approved, pending) and the two
// tree statements with nothing.
func fakeExtensionQuery(calls *[]treeCall, facts map[string][2]int64, fail error) treeQuery {
	return func(_ context.Context, stmt string, args ...any) (treeRows, error) {
		*calls = append(*calls, treeCall{stmt, args})
		if fail != nil {
			return nil, fail
		}
		out := &fakeTreeRows{}
		if !strings.Contains(stmt, "FROM de_nghi_lui_han") {
			return out, nil
		}
		for _, a := range args {
			id := a.(string)
			if f, ok := facts[id]; ok {
				out.rows = append(out.rows, []any{id, f[0], f[1]})
			}
		}
		return out, nil
	}
}

func TestAttachExtensionFactsIsOneStatementForAWholePage(t *testing.T) {
	ds := []domain.NhiemVu{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "a"}} // `a` twice: one placeholder
	var calls []treeCall
	q := fakeExtensionQuery(&calls, map[string][2]int64{"a": {2, 0}, "b": {0, 1}}, nil) // c has none

	if err := attachExtensionFacts(context.Background(), q, ds); err != nil {
		t.Fatalf("attachExtensionFacts: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("chạy %d câu cho 4 dòng, muốn 1 (không N+1)", len(calls))
	}
	c := calls[0]
	if !strings.Contains(c.stmt, "nhiem_vu_id IN ($2, $3, $4) GROUP BY nhiem_vu_id") || len(c.args) != 3 {
		t.Errorf("câu = %q %v, muốn 3 id KHÁC NHAU", c.stmt, c.args)
	}
	if !strings.Contains(c.stmt, "FROM de_nghi_lui_han WHERE tenant_id = $1 AND") {
		t.Errorf("câu không buộc xã: %q", c.stmt)
	}

	want := map[string]struct {
		n       int
		pending bool
	}{"a": {2, false}, "b": {0, true}, "c": {0, false}}
	for _, n := range ds {
		if w := want[n.ID]; n.ExtensionCount != w.n || n.PendingExtension != w.pending {
			t.Errorf("%s: extension_count=%d pending=%v, muốn %d %v", n.ID, n.ExtensionCount, n.PendingExtension, w.n, w.pending)
		}
	}
}

// THE APPROVED COUNT IS MIGRATION 0016'S PREDICATE, AND THE TWO MUST NEVER DISAGREE: `trang_thai =
// 'da-duyet'` with NO `deleted_at` — in the item and in the outer WHERE alike. The pending item is the
// `moc_cho_duyet` slot: `cho-duyet` AND live.
func TestExtensionFactsPredicatesMatchTheTriggerAndTheSlot(t *testing.T) {
	if extensionApprovedItem != "count(*) FILTER (WHERE trang_thai = 'da-duyet')" {
		t.Errorf("mục đã duyệt = %q — phải đúng điều kiện của trigger 0016", extensionApprovedItem)
	}
	if extensionPendingItem != "count(*) FILTER (WHERE trang_thai = 'cho-duyet' AND deleted_at IS NULL)" {
		t.Errorf("mục đang chờ = %q", extensionPendingItem)
	}
	where := extensionFactsQuery[strings.Index(extensionFactsQuery, " FROM "):]
	if strings.Contains(where, "deleted_at") {
		t.Errorf("WHERE ngoài lọc xoá mềm — số lần lùi hạn sẽ bỏ sót đề nghị đã duyệt rồi xoá: %q", where)
	}
	// The fake drivers split the SELECT list on commas: an item carrying one would desync every row.
	for _, item := range []string{extensionApprovedItem, extensionPendingItem} {
		if strings.Contains(item, ",") || strings.Contains(item, " FROM ") {
			t.Errorf("mục %q mang dấu phẩy hoặc FROM", item)
		}
	}
}

func TestAttachExtensionFactsSkipsAnEmptyPage(t *testing.T) {
	var calls []treeCall
	q := fakeExtensionQuery(&calls, nil, nil)
	if err := attachExtensionFacts(context.Background(), q, nil); err != nil || len(calls) != 0 {
		t.Errorf("trang rỗng: %d câu, lỗi %v — muốn 0 câu", len(calls), err)
	}
	if err := attachExtensionFacts(context.Background(), q, []domain.NhiemVu{{ID: ""}}); err != nil || len(calls) != 0 {
		t.Errorf("dòng không id: %d câu, lỗi %v — muốn 0 câu", len(calls), err)
	}
}

// A failure is RETURNED, never answered with zeros — `pending_extension: false` on a task with a
// request on the desk would offer the officer a second request.
func TestAttachExtensionFactsReturnsAFailure(t *testing.T) {
	var calls []treeCall
	ds := []domain.NhiemVu{{ID: "a", ExtensionCount: 9}}
	err := attachExtensionFacts(context.Background(), fakeExtensionQuery(&calls, nil, errors.New("pg: down")), ds)
	if err == nil || !strings.Contains(err.Error(), "pg: down") {
		t.Errorf("lỗi = %v, muốn lỗi gốc được bọc", err)
	}
}

// attachTaskFacts = the two tree facts, then the extension facts: three statements for a page with a
// parent, in that order, all bound to the commune.
func TestAttachTaskFactsRunsTreeThenExtension(t *testing.T) {
	var calls []treeCall
	ds := []domain.NhiemVu{{ID: "a"}, {ID: "b", NhiemVuChaID: "a"}}
	if err := attachTaskFacts(context.Background(),
		fakeExtensionQuery(&calls, map[string][2]int64{"b": {1, 1}}, nil), ds); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 3 || !strings.Contains(calls[0].stmt, "GROUP BY nhiem_vu_cha_id") ||
		!strings.Contains(calls[2].stmt, "FROM de_nghi_lui_han") {
		t.Fatalf("thứ tự câu = %d câu", len(calls))
	}
	if ds[1].ExtensionCount != 1 || !ds[1].PendingExtension || ds[0].ExtensionCount != 0 {
		t.Errorf("dữ kiện = %+v", ds)
	}
}
