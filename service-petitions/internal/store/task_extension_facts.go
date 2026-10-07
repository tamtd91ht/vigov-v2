package store

// The two EXTENSION FACTS every task response carries (menu nhiem-vu, owner 07/10/2026, following the
// prototype vigov-require tasks/service.py:176 and repository.py:373): how many extension requests
// were ever approved (`extension_count`) and whether one is awaiting a decision (`pending_extension`).
// SQL, and nothing else.
//
// # ONE STATEMENT PER PAGE, NEVER ONE PER ROW (skills/load-data-once)
//
// Both facts come from ONE GROUP BY over `de_nghi_lui_han` for every task on the page, served by
// `de_nghi_lui_han_theo_nhiem_vu` (tenant_id, nhiem_vu_id, …) of migration 0006 for the live rows.
// An empty page runs no statement — an empty `IN (…)` list is not SQL.
//
// # A FAILURE IS RETURNED, NEVER PAPERED OVER WITH ZEROS
//
// `pending_extension: false` on a task with a request on the leader's desk would tell the drawer to
// offer "Đề nghị lùi hạn" again — a statement about the record made from a failure to read it, the
// same line task_tree_facts.go draws for `child_count`.
//
// # THE TWO PREDICATES, AND WHY THEY DIFFER ON `deleted_at`
//
//	extension_count    trang_thai = 'da-duyet', SOFT-DELETED ROWS INCLUDED. EXACTLY the predicate of
//	                   the `nhiem_vu_bat_bien` trigger (migration 0016:93-95) and of
//	                   DeNghiLuiHanStore.ApprovedCount. That trigger decides whether `han_ban_dau` is
//	                   still free to follow a correction; this figure is what the screen shows next to
//	                   it. If the two disagreed, a drawer could read "chưa lùi hạn lần nào" while the
//	                   database refuses to let the original deadline follow a correction — or the
//	                   reverse. An approval that moved the deadline moved it, whatever happened to the
//	                   request row afterwards (0016:29-33).
//	pending_extension  trang_thai = 'cho-duyet' AND deleted_at IS NULL — the `moc_cho_duyet` slot of
//	                   migration 0006:632-637 and DeNghiLuiHanStore.DangChoDuyet: a withdrawn request is
//	                   on nobody's desk.
//
// `tenant_id = $1` on every statement, from the context or the transaction (rule 1, invariant 5); the
// task ids are bound placeholders even though they come from rows just read.
//
// ⚠ FORMATTING CONSTRAINT (fake drivers): the SELECT list is read between the first `SELECT ` and the
// first ` FROM ` and split on commas. No select item below may carry a comma or the word ` FROM `.

import (
	"context"
	"fmt"
	"strings"

	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// The two aggregate items, named so the fake drivers can key a fixture row by the exact text the
// statement selects. The status literals come from the domain constants — one spelling of each code.
const (
	extensionApprovedItem = `count(*) FILTER (WHERE trang_thai = '` + string(domain.DaDuyetLuiHan) + `')`
	extensionPendingItem  = `count(*) FILTER (WHERE trang_thai = '` + string(domain.ChoDuyetLuiHan) +
		`' AND deleted_at IS NULL)`
)

// extensionFactsQuery is the statement up to the opening of the id list. NO `deleted_at` IN THE OUTER
// WHERE, on purpose: the approved count must see soft-deleted rows (see the header); the pending item
// excludes them inside its own FILTER.
const extensionFactsQuery = `SELECT nhiem_vu_id, ` + extensionApprovedItem + `, ` + extensionPendingItem +
	` FROM de_nghi_lui_han WHERE tenant_id = $1 AND nhiem_vu_id IN (`

// attachExtensionFacts fills ExtensionCount and PendingExtension on every task in ds, in place.
// A task with no request is absent from the answer and reads 0 / false — that IS its true value.
func attachExtensionFacts(ctx context.Context, q treeQuery, ds []domain.NhiemVu) error {
	ids := make([]any, 0, len(ds))
	seen := make(map[string]bool, len(ds))
	for _, n := range ds {
		if n.ID == "" || seen[n.ID] {
			continue
		}
		seen[n.ID] = true
		ids = append(ids, n.ID)
	}
	if len(ids) == 0 {
		return nil
	}

	type facts struct {
		approved int
		pending  bool
	}
	byTask := make(map[string]facts, len(ids))

	var b strings.Builder
	b.WriteString(extensionFactsQuery)
	writePlaceholders(&b, len(ids))
	b.WriteString(" GROUP BY nhiem_vu_id")
	if err := eachRow(ctx, q, b.String(), ids, func(r treeRows) error {
		var (
			taskID            string
			approved, pending int64
		)
		if err := r.Scan(&taskID, &approved, &pending); err != nil {
			return err
		}
		byTask[taskID] = facts{approved: int(approved), pending: pending > 0}
		return nil
	}); err != nil {
		return fmt.Errorf("de_nghi_lui_han: đọc số lần lùi hạn của trang nhiệm vụ: %w", err)
	}

	for i := range ds {
		f := byTask[ds[i].ID]
		ds[i].ExtensionCount = f.approved
		ds[i].PendingExtension = f.pending
	}
	return nil
}

// attachTaskFacts is every per-page fact a task response carries beyond its own row: the two tree
// facts (task_tree_facts.go) and the two extension facts above — three statements at most for a page
// of any size. EVERY READER OF THE nhiemVuRa SHAPE GOES THROUGH HERE, so a fact added to one read is
// on all of them: the register list, the single read, the meeting's task list and the write replies.
func attachTaskFacts(ctx context.Context, q treeQuery, ds []domain.NhiemVu) error {
	if err := attachTreeFacts(ctx, q, ds); err != nil {
		return err
	}
	return attachExtensionFacts(ctx, q, ds)
}
