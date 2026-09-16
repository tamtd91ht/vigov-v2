package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/vihat/vigov/pkg/page"
)

// PageSpec is the part of a paginated read the CALLER owns: what to select, from where, and any
// extra filter. Everything that decides the ORDER and the BOUND is owned by QueryPage, because
// those are the two things that cannot be got wrong quietly.
//
// Filter keeps the contract of Scoped.Query: placeholders start at $2, and $1 is always the
// commune. QueryPage appends its own placeholders after the caller's.
type PageSpec struct {
	Columns string // SELECT list — must include the sort column and id
	Table   string
	Filter  string // extra predicate, e.g. `AND deleted_at IS NULL AND trang_thai = $2`
	Args    []any  // the values for the caller's own placeholders, in order
}

// QueryPage runs ONE page of a cursor-paginated read, through Scoped.Query and nowhere else.
//
// WHY IT IS A FUNCTION AND NOT A SECOND ROUTE TO THE DATABASE: rule 1, invariant 5 — every query
// goes through a scoped repository. This builds a tail and hands it to Scoped.Query, which is the
// method that owns `WHERE tenant_id = $1`. There is no path here that reaches *sql.DB, and adding
// one would reopen exactly the hole pkg/store exists to close.
//
// THE STATEMENT IT BUILDS, and why each piece is there:
//
//		SELECT <cols> FROM <table> WHERE tenant_id = $1 <filter>
//		  AND (<sort>, id) > ($n, $n+1)      -- the anchor; omitted on the first page
//		 ORDER BY <sort> ASC, id ASC          -- TOTAL order: the id breaks ties
//		 LIMIT $n+2                           -- limit+1: one row more than is returned
//
//	  - The row-value comparison `(sort, id) > (…)` is exactly "everything after this anchor" in the
//	    ORDER BY's own order, and PostgreSQL can serve it from a composite index on (tenant_id,
//	    sort, id). It is also the only shape that stays correct when sort keys repeat.
//	  - `, id` in the ORDER BY is the load-bearing character of this file. Two rows with the same
//	    ngay_tao and no tie-break give an order the database is free to change between two queries,
//	    so the next page repeats one row and loses another — with no error and no test failing
//	    unless the test uses duplicate keys on purpose.
//	  - limit+1 is how has_more is known WITHOUT a COUNT(*) (§5 #4). The extra row is a signal only;
//	    it is never returned, and never encoded into the cursor.
//
// WHAT IT DOES NOT DO YET, so nobody has to find out by reading the code: it pages ONE table, the
// way Scoped.Query does. A joined list needs the sort column qualified with its table's alias and
// every joined table constrained to $1 in its ON clause — that is QueryJoin's contract, and
// bolting it on here would mean this function no longer owns the whole statement, which is what
// makes the ordering safe. The tie-break column is `id`, and the sort column must be NOT NULL:
// `(col, id) > (…)` is NULL for a NULL col, so those rows vanish from every page after the first.
//
// scan converts one row into an item AND states that row's anchor. Stating it is the caller's job
// because only the caller knows which scanned field is the sort key — and an anchor built from the
// wrong field is a cursor that walks the list in an order nobody asked for. The kind is checked
// against the column, so a mismatch is an error at the first page, not a corrupt cursor later.
func QueryPage[T any](
	ctx context.Context,
	s *Scoped,
	spec PageSpec,
	req page.Request,
	scan func(*sql.Rows) (T, page.Anchor, error),
) (page.Result[T], error) {
	out := page.NewResult[T]()

	if err := spec.hopLe(); err != nil {
		return out, err
	}
	col := req.Column()
	if col.SQL == "" {
		return out, fmt.Errorf("store: yêu cầu phân trang chưa qua page.Parse — không có cột sắp xếp")
	}

	var tail strings.Builder
	tail.WriteString(spec.Filter)

	args := make([]any, 0, len(spec.Args)+3)
	args = append(args, spec.Args...)
	next := len(args) + 2 // $1 belongs to the commune, always

	op, dir := ">", "ASC"
	if req.Dir() == page.Desc {
		op, dir = "<", "DESC"
	}

	if a, ok := req.After(); ok {
		if a.Key.Kind() != col.Kind {
			return out, fmt.Errorf("store: mốc phân trang kiểu %q, cột %q kiểu %q",
				a.Key.Kind(), col.SQL, col.Kind)
		}
		if col.SQL == "id" {
			// Sorting BY the tie-break: the id alone is already a total order, so a row comparison
			// against itself would only be noise.
			fmt.Fprintf(&tail, " AND id %s $%d", op, next)
			args = append(args, a.ID)
			next++
		} else {
			fmt.Fprintf(&tail, " AND (%s, id) %s ($%d, $%d)", col.SQL, op, next, next+1)
			args = append(args, a.Key.Value(), a.ID)
			next += 2
		}
	}

	if col.SQL == "id" {
		fmt.Fprintf(&tail, " ORDER BY id %s LIMIT $%d", dir, next)
	} else {
		fmt.Fprintf(&tail, " ORDER BY %s %s, id %s LIMIT $%d", col.SQL, dir, dir, next)
	}
	args = append(args, req.Limit()+1)

	rows, err := s.Query(ctx, spec.Columns, spec.Table, tail.String(), args...)
	if err != nil {
		return out, fmt.Errorf("store: đọc trang %s: %w", spec.Table, err)
	}
	defer rows.Close()

	var cuoi page.Anchor
	for rows.Next() {
		if len(out.Items) == req.Limit() {
			// The limit+1-th row exists, which is all it was fetched for. It is NOT scanned into the
			// response: returning it would hand the client one more row than it asked for and make
			// the next page start one row late.
			out.HasMore = true
			break
		}
		item, moc, err := scan(rows)
		if err != nil {
			return page.NewResult[T](), fmt.Errorf("store: quét dòng %s: %w", spec.Table, err)
		}
		out.Items = append(out.Items, item)
		cuoi = moc
	}
	if err := rows.Err(); err != nil {
		return page.NewResult[T](), fmt.Errorf("store: đọc trang %s: %w", spec.Table, err)
	}

	if out.HasMore {
		if cuoi.ID == "" {
			return page.NewResult[T](), fmt.Errorf("store: quét không trả mốc cho %s — con trỏ kế tiếp sẽ sai", spec.Table)
		}
		if cuoi.Key.Kind() != col.Kind {
			return page.NewResult[T](), fmt.Errorf("store: mốc quét được kiểu %q, cột %q kiểu %q",
				cuoi.Key.Kind(), col.SQL, col.Kind)
		}
		out.NextCursor = page.Encode(col, req.Dir(), cuoi)
	}
	return out, nil
}

// hopLe refuses a spec that would take the ordering back off QueryPage.
//
// A caller-supplied ORDER BY does not fail — it produces a statement with two of them, or one
// that silently reorders the keyset walk, and the result is the skipped/repeated rows this whole
// file exists to prevent. A second LIMIT contradicts the bound. Both are cheap to catch here and
// expensive to notice in production.
func (p PageSpec) hopLe() error {
	if strings.TrimSpace(p.Columns) == "" {
		return fmt.Errorf("store: PageSpec thiếu danh sách cột")
	}
	if strings.TrimSpace(p.Table) == "" {
		return fmt.Errorf("store: PageSpec thiếu bảng")
	}
	thap := strings.ToLower(p.Filter)
	for _, cam := range []string{"order by", "limit", "offset", ";"} {
		if strings.Contains(thap, cam) {
			return fmt.Errorf("store: PageSpec.Filter chứa %q — thứ tự và giới hạn do QueryPage quyết định", cam)
		}
	}
	return nil
}
