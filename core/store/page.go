package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/vihat/vigov/core/page"
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

// Moc binds each allowlisted sort column to the function that reads that column's value out of
// one scanned row.
//
// VÌ SAO NÓ TỒN TẠI. Trước đây `scan` phải tự khai mốc: nó trả về `(item, page.Anchor, error)`,
// và mỗi bên gọi tự viết một `switch` trên cột đang sắp xếp. Hai vấn đề, cả hai đều im lặng:
//
//   - `scan` KHÔNG BIẾT cột nào đang được sắp xếp trừ khi bên gọi tự truyền vào và tự phân
//     nhánh đúng. Hai cột cùng kiểu (`tao_luc` và `cap_nhat_luc`, cùng KindTime) lẫn nhau thì
//     không gì bắt được: phép kiểm duy nhất là so KIỂU, mà hai cột ấy cùng kiểu. Con trỏ khi
//     đó đi theo một thứ tự không ai đặt hàng, và trang sau vừa lặp vừa sót.
//   - Danh sách trắng và cách lấy mốc là HAI khai báo ở hai chỗ. Thêm một cột vào danh sách
//     trắng mà quên thêm nhánh `switch` thì cột ấy sắp xếp được nhưng mốc sai.
//
// Nay hàm đọc mốc được CHỌN THEO CHÍNH CỘT ĐANG SẮP XẾP, nên lẫn cột là chuyện không dựng
// được nữa. Và `NewMoc` đối chiếu hai danh sách lúc dựng, nên thiếu hay thừa một cột là panic
// lúc khởi động — chỗ rẻ nhất để phát hiện.
type Moc[T any] struct {
	lay map[string]func(T) page.Key
}

// NewMoc binds an allowlist to its anchor readers, and panics when the two do not match.
//
// PANIC CHỨ KHÔNG TRẢ LỖI, cùng lý do với page.Col: hàm này chạy lúc dựng, từ hằng số trong mã
// nguồn. Một lệch ở đây không phải dữ liệu xấu từ người dùng mà là mã sai, và nó phải chết ở
// lần khởi động đầu tiên chứ không phải ở trang thứ hai của một danh sách nào đó, sáu tháng sau.
func NewMoc[T any](ds page.Allowlist, lay map[string]func(T) page.Key) Moc[T] {
	cols := ds.Columns()
	co := make(map[string]bool, len(cols))
	for _, c := range cols {
		co[c.Param] = true
		if lay[c.Param] == nil {
			panic(fmt.Sprintf("store: cột %q được phép sắp xếp nhưng không khai cách đọc mốc "+
				"của nó — con trỏ sẽ đi sai thứ tự mà không báo lỗi", c.Param))
		}
	}
	for param := range lay {
		if !co[param] {
			panic(fmt.Sprintf("store: khai cách đọc mốc cho cột %q không có trong danh sách "+
				"trắng — hoặc gõ nhầm tên, hoặc cột đã bị gỡ", param))
		}
	}
	ra := make(map[string]func(T) page.Key, len(lay))
	for k, v := range lay {
		ra[k] = v
	}
	return Moc[T]{lay: ra}
}

// khoa reads the anchor value of `item` for the column currently being sorted on.
func (m Moc[T]) khoa(col page.Column, item T) (page.Key, error) {
	f, ok := m.lay[col.Param]
	if !ok || f == nil {
		return page.Key{}, fmt.Errorf("store: không có cách đọc mốc cho cột %q", col.Param)
	}
	k := f(item)
	if k.Kind() != col.Kind {
		return page.Key{}, fmt.Errorf("store: mốc đọc được kiểu %q, cột %q kiểu %q",
			k.Kind(), col.SQL, col.Kind)
	}
	return k, nil
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
// scan converts one row into an item and its `id`. It does NOT state the anchor any more: the
// anchor is read by `moc`, which is bound to the allowlist at wiring time, so the reader used is
// always the one belonging to the column actually being sorted on. See Moc for what that removes.
func QueryPage[T any](
	ctx context.Context,
	s *Scoped,
	spec PageSpec,
	req page.Request,
	moc Moc[T],
	scan func(*sql.Rows) (T, string, error),
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
		item, id, err := scan(rows)
		if err != nil {
			return page.NewResult[T](), fmt.Errorf("store: quét dòng %s: %w", spec.Table, err)
		}
		khoa, err := moc.khoa(col, item)
		if err != nil {
			return page.NewResult[T](), fmt.Errorf("store: trang %s: %w", spec.Table, err)
		}
		if id == "" {
			return page.NewResult[T](), fmt.Errorf(
				"store: quét dòng %s không trả id — con trỏ kế tiếp sẽ sai", spec.Table)
		}
		out.Items = append(out.Items, item)
		cuoi = page.Anchor{Key: khoa, ID: id}
	}
	if err := rows.Err(); err != nil {
		return page.NewResult[T](), fmt.Errorf("store: đọc trang %s: %w", spec.Table, err)
	}

	// KIỂM MỐC NAY CHẠY TRÊN MỌI TRANG, không chỉ khi còn trang sau.
	//
	// Trước đây cả hai phép kiểm nằm trong nhánh `if out.HasMore`, nên một danh sách vừa đúng
	// một trang KHÔNG được kiểm gì cả. Mốc sai ở đó không gây ra triệu chứng nào — cho tới
	// ngày dữ liệu vượt một trang, và khi ấy nó là một lỗi phân trang ở một hệ thống đang
	// chạy thật, xa hẳn thay đổi đã gây ra nó. Nay `moc.khoa` kiểm ngay tại vòng quét ở trên.
	if out.HasMore {
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
