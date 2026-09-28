package store

// The two TREE FACTS every task response carries (ADR 0037): the parent's REGISTER NUMBER and the
// number of live direct children. SQL, and nothing else.
//
// # ONE STATEMENT PER FACT PER PAGE, NEVER ONE PER ROW (skills/load-data-once)
//
// A page of 100 tasks costs exactly two statements here, whatever it holds: one GROUP BY over the
// children of every task on the page, one read of every distinct parent. A per-row lookup would be
// invisible at 20 rows and 201 round trips at a commune that pages by 100.
//
// # WHY THE PARENT'S CODE AND NOT ITS INTERNAL id
//
// `parent` on the wire is the register number, in BOTH directions: the create/update bodies take a
// code (app.GhiNhiemVu resolves it inside the transaction), so the read side must emit the same
// thing, or a client that sends back what it read would be sending an id to a field that now takes a
// code. No response FIELD carries a task's internal id. (The opaque `next_cursor` does embed the
// last row's id as its tie-break — core/page — but that is a token to hand back, not a value any
// request field accepts.)
//
// # THE THREE GUARANTEES OF THIS PACKAGE HOLD HERE TOO
//
//	tenant_id = $1       every statement, from the context or the transaction (rule 1, invariant 5)
//	deleted_at IS NULL   a soft-deleted child is not counted, a soft-deleted parent is not named
//	                     (rule 7, invariant 2). ADR 0037 decision 3 refuses to delete a task while it
//	                     has live children, so a live child under a deleted parent is unreachable by
//	                     the write path; should one exist, its `parent` is "" rather than a number
//	                     that 404s.
//	bound placeholders   the ids come from rows just read and still go in bound
//
// ⚠ FORMATTING CONSTRAINT (fake drivers, driver_gia_test.go and the app package's): the SELECT list
// is read between the first `SELECT ` and the first ` FROM `, split on commas. Neither statement
// below may put a comma inside a select item.

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// treeRows is the part of *sql.Rows the two reads use. An interface so the batching itself — how
// many statements, which ids, how the answers are mapped back — is testable without a driver.
type treeRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close() error
}

// treeQuery runs ONE full statement whose $1 is the commune; args are the placeholders from $2 on.
type treeQuery func(ctx context.Context, stmt string, args ...any) (treeRows, error)

// scopedTreeQuery is the request-scoped reader: store.Scoped.QueryJoin binds the commune from the
// context as $1.
func scopedTreeQuery(s *store.Scoped) treeQuery {
	return func(ctx context.Context, stmt string, args ...any) (treeRows, error) {
		r, err := s.QueryJoin(ctx, stmt, args...)
		if err != nil {
			// Returned as a nil INTERFACE, not a typed nil *sql.Rows wrapped in one — a caller testing
			// `rows == nil` would otherwise see a non-nil value.
			return nil, err
		}
		return r, nil
	}
}

// txTreeQuery is the same reader inside a write transaction, with the commune from the transaction.
func txTreeQuery(tx *store.ScopedTx) treeQuery {
	return func(ctx context.Context, stmt string, args ...any) (treeRows, error) {
		r, err := tx.Underlying().QueryContext(ctx, stmt, append([]any{string(tx.TenantID())}, args...)...)
		if err != nil {
			return nil, err
		}
		return r, nil
	}
}

// childCountQuery counts the live direct children of a set of tasks. Served by the partial index
// `nhiem_vu_cha` (tenant_id, nhiem_vu_cha_id) of migration 0008.
const childCountQuery = `SELECT nhiem_vu_cha_id, count(*) FROM nhiem_vu
	WHERE tenant_id = $1 AND deleted_at IS NULL AND nhiem_vu_cha_id IN (`

// parentCodeQuery reads the register number of a set of parents.
const parentCodeQuery = `SELECT id, ma FROM nhiem_vu
	WHERE tenant_id = $1 AND deleted_at IS NULL AND id IN (`

// writePlaceholders writes `$2, $3, …)` for n values — $1 is the commune.
func writePlaceholders(b *strings.Builder, n int) {
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("$" + strconv.Itoa(i+2))
	}
	b.WriteString(")")
}

// attachTreeFacts fills ParentCode and ChildCount on every task in ds, in place.
//
// NO STATEMENT FOR AN EMPTY PAGE, and no parent statement when no task on the page has a parent —
// the `IN (…)` list would be empty, which is not valid SQL.
//
// A FAILURE IS RETURNED, NEVER PAPERED OVER WITH ZEROS. `child_count: 0` on a task with three
// children is a statement about the record made from a failure to read it — the same line the detail
// route draws for the document block.
func attachTreeFacts(ctx context.Context, q treeQuery, ds []domain.NhiemVu) error {
	if len(ds) == 0 {
		return nil
	}

	// --- 1. the children of every task on the page ------------------------------------------------
	taskIDs := make([]any, 0, len(ds))
	seen := make(map[string]bool, len(ds))
	for _, n := range ds {
		if n.ID == "" || seen[n.ID] {
			continue
		}
		seen[n.ID] = true
		taskIDs = append(taskIDs, n.ID)
	}
	counts := make(map[string]int, len(taskIDs))
	if len(taskIDs) > 0 {
		var b strings.Builder
		b.WriteString(childCountQuery)
		writePlaceholders(&b, len(taskIDs))
		b.WriteString(" GROUP BY nhiem_vu_cha_id")
		if err := eachRow(ctx, q, b.String(), taskIDs, func(r treeRows) error {
			var (
				parentID string
				n        int64
			)
			if err := r.Scan(&parentID, &n); err != nil {
				return err
			}
			counts[parentID] = int(n)
			return nil
		}); err != nil {
			return fmt.Errorf("nhiem_vu: đếm việc con: %w", err)
		}
	}

	// --- 2. the register number of every distinct parent ------------------------------------------
	var parentIDs []any
	seenParent := map[string]bool{}
	for _, n := range ds {
		if n.NhiemVuChaID == "" || seenParent[n.NhiemVuChaID] {
			continue
		}
		seenParent[n.NhiemVuChaID] = true
		parentIDs = append(parentIDs, n.NhiemVuChaID)
	}
	parentCodes := make(map[string]string, len(parentIDs))
	if len(parentIDs) > 0 {
		var b strings.Builder
		b.WriteString(parentCodeQuery)
		writePlaceholders(&b, len(parentIDs))
		if err := eachRow(ctx, q, b.String(), parentIDs, func(r treeRows) error {
			var id, code string
			if err := r.Scan(&id, &code); err != nil {
				return err
			}
			parentCodes[id] = code
			return nil
		}); err != nil {
			return fmt.Errorf("nhiem_vu: đọc mã việc cha: %w", err)
		}
	}

	for i := range ds {
		ds[i].ChildCount = counts[ds[i].ID]
		ds[i].ParentCode = parentCodes[ds[i].NhiemVuChaID]
	}
	return nil
}

// eachRow runs one statement and hands every row to f. The rows are closed before it returns, so the
// second statement of attachTreeFacts never runs while the first still holds the connection.
func eachRow(ctx context.Context, q treeQuery, stmt string, args []any, f func(treeRows) error) error {
	rows, err := q(ctx, stmt, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := f(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

// AttachTreeFactsTx is attachTreeFacts inside a write transaction — for the reply of an act that
// changed the task (PATCH, status), so the reply carries the same two facts the reads do.
//
// INSIDE THE TRANSACTION, like the document block the same replies re-read: the reply describes the
// state the act committed, and a read after the commit could describe a later one.
func (s *NhiemVuStore) AttachTreeFactsTx(ctx context.Context, tx *store.ScopedTx, ds []domain.NhiemVu) error {
	return attachTreeFacts(ctx, txTreeQuery(tx), ds)
}

// Compile-time check that *sql.Rows is what the two readers hand back.
var _ treeRows = (*sql.Rows)(nil)
