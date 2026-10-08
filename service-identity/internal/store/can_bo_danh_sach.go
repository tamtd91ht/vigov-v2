package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// The READ paths behind GET /api/v1/staff and GET /api/v1/staff/{id}.
//
// WHY THESE ARE NOT TheoID / TheoEmail, AND MUST NOT BE MERGED WITH THEM. Those two serve the
// SIGN-IN path and filter three conditions — not deleted, has an account, not locked. These two
// serve the STAFF REGISTER and filter one: not deleted. The predicates differ because the
// questions differ, and unifying them breaks whichever side loses its condition:
//
//   - widen the sign-in query and a public directory entry, or a locked-out former employee,
//     signs in;
//   - narrow the register and the 26 people of the commune's directory (12-danh-ba-can-bo.md §7)
//     disappear from the screen that exists to list them.
//
// Neither failure is loud. Two functions with two stated predicates is the cheap way to keep
// them from drifting into one.

// SapXepCanBo is the closed set of sorts GET /api/v1/staff and POST /api/v1/staff/searches offer.
// Declared here, next to the SQL, because it names real columns of `nguoi_dung` — only store/ knows
// column names. tools/apidoc reads it (`@page idstore.SapXepCanBo`) to publish the `sort` enum.
//
// `code` IS THE DEFAULT, ascending: it is stable, unique within the commune (UNIQUE (tenant_id,
// ma), migration 0001), and it is the value the audit trail quotes, so "the third row on page 2"
// means the same thing to the screen and to whoever later reads the trail.
//
// THE SIX SCREEN COLUMNS ARE SORTABLE SINCE 08/10/2026 (owner decision; prototype UserTable.tsx). This
// list used to refuse all six, for two reasons that core/page states; each is now answered by a shape
// that removes the reason rather than ignoring it:
//
//	full_name, position, phone   PERSONAL DATA MUST NOT TRAVEL IN A CURSOR (rule 3, forbidden #4) — a
//	  (ho_ten, chuc_vu,          cursor is a URL, an access log and a browser history. page.KindRef:
//	   dien_thoai_co_quan)       the cursor carries the anchor row's ID ONLY, and QueryPage looks the
//	                             key up from that row server-side. All three are NOT NULL columns of the
//	                             plain table, which KindRef requires. `phone` is the OFFICE number — the
//	                             prototype's single "Điện thoại" column; the cost KindRef states (an
//	                             anchor edited between two pages resumes from its new position) applies.
//	department, last_login_at    NULLABLE: `(col, id) > (…)` is NULL for a NULL col, so the people with
//	                             no department or who never signed in would vanish from page two on —
//	                             exactly the rows an administrator opens this screen to find. They sort
//	                             on a NOT NULL key COMPUTED in a derived relation (staffSortRelation)
//	                             whose empty value is a per-DIRECTION sentinel, so empties come LAST
//	                             both ways and the keyset stays a total order across the boundary.
//	status                       a boolean: the owner chose to sort by it anyway, ties by id. Folded
//	                             to 0/1 (core/page's note on booleans names the cost: the id does most
//	                             of the ordering — which is what "ties → id" asks for).
//
// WHAT STAYS ABSENT: `email` and `mobile` (di_dong_ca_nhan). Both are personal data, the owner did not
// list them, and the prototype has no column for either; a sort key is a promise about the whole column.
// `co_tai_khoan` stays absent too — it is not one of the six columns.
//
// THE ANCHOR OF A COMPUTED KEY IS THE SCANNED KEY, never re-derived in Go: the statement selects the
// computed column at the tail and staffPageRow carries it, so the sentinel in the cursor is the very
// value the SQL compared. One spelling of each sentinel, not two that could drift by a second.
var SapXepCanBo = page.NewAllowlist(page.Asc,
	page.Col("code", "ma", page.KindText),
	page.Col("created_at", "tao_luc", page.KindTime),
	page.Col("full_name", "ho_ten", page.KindRef),
	page.Col("position", "chuc_vu", page.KindRef),
	page.Col("department", staffDepartmentSortColumn, page.KindText),
	page.Col("phone", "dien_thoai_co_quan", page.KindRef),
	page.Col("last_login_at", staffLastLoginSortColumn, page.KindTime),
	page.Col("status", staffStatusSortColumn, page.KindInt),
)

// The three computed sort columns of the derived relations.
const (
	staffDepartmentSortColumn = "department_sort"
	staffLastLoginSortColumn  = "last_login_sort"
	staffStatusSortColumn     = "status_sort"
)

// The "never signed in" sentinels, one per direction: later than any sign-in ascending, earlier than
// any descending, so those people come LAST both ways. The SQL literals are built FROM these values
// (staffTimestamptzLiteral) — the same discipline as service-petitions' due-date key.
var (
	staffNeverSignedInAsc  = time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC)
	staffNeverSignedInDesc = time.Time{} // 0001-01-01 00:00:00 UTC
)

// staffTimestamptzLiteral spells one of the two sentinels above as a PostgreSQL literal. Never called
// on a value from a request.
func staffTimestamptzLiteral(t time.Time) string {
	return "TIMESTAMPTZ '" + t.UTC().Format("2006-01-02 15:04:05") + "+00'"
}

// staffSortRelation is the relation one page is cut from, and the computed key column to scan at the
// tail ("" when the sort reads a plain column).
//
// THE PLAIN TABLE for `code`, `created_at` and the three KindRef sorts — KindRef MUST page the plain
// table, because QueryPage looks the anchor's key up there by name.
//
// A DERIVED RELATION for the other three: every column of `nguoi_dung` plus one NOT NULL key.
//
//	ALIASED `nguoi_dung`        so every predicate of menhDeLocCanBo — including the department
//	                            search's correlated `nguoi_dung.bo_phan_id` — reads the same names on
//	                            both shapes. One predicate builder, no second copy.
//	`nguoi_dung.tenant_id = $1` INSIDE as well as outside: QueryPage binds only the outer relation; the
//	                            inner one keeps the commune bound in the subquery's own text (rule 1,
//	                            invariant 5) and the partition pruned.
//	department                  LEFT JOIN bo_phan bound to the SAME commune in its ON clause (rule 1) and
//	                            to LIVE units only: the screen resolves names from the live unit list
//	                            and shows "—" for a deleted one, so a deleted unit sorts as "no
//	                            department" — the same reading the name search makes. The key is a
//	                            one-character PREFIX + the name, NOT a sentinel string: no text value is
//	                            reliably "greater than every name" under a linguistic collation, but a
//	                            leading '0' vs '1' decides the comparison at its first character under
//	                            any collation, and an equal prefix leaves the names' own order intact.
//	                            Ascending: '0'||name, none = '1'. Descending: '1'||name, none = '0'.
//	                            Never empty, which a KindText cursor key must not be (page.parseKey).
//	                            A department name is not personal data, so it may ride in the cursor.
//	last_login_at               COALESCE with the direction's sentinel.
//	status                      dang_hoat_dong as 0/1 — one key for both directions.
//
// `nguoi_dung.*` NEVER LEAVES THE STATEMENT: the outer SELECT is cotTomTat plus the key, so the
// credential column the derived relation passes through is never selected out.
//
// ⚠ NO INDEX SERVES THESE ORDERS; one commune's register (tens to a few hundred rows) is sorted in
// memory — the same bound menhDeLocCanBo states for its scans.
func staffSortRelation(param string, dir page.Dir) (relation, keyColumn string) {
	const from = ` FROM nguoi_dung`
	const where = ` WHERE nguoi_dung.tenant_id = $1) AS nguoi_dung`
	switch param {
	case "department":
		present, absent := "'0'", "'1'"
		if dir == page.Desc {
			present, absent = "'1'", "'0'"
		}
		return `(SELECT nguoi_dung.*, CASE WHEN b.id IS NULL THEN ` + absent + ` ELSE ` + present +
			` || b.ten END AS ` + staffDepartmentSortColumn + from +
			` LEFT JOIN bo_phan b ON b.tenant_id = $1 AND b.id = nguoi_dung.bo_phan_id AND b.deleted_at IS NULL` +
			where, staffDepartmentSortColumn
	case "last_login_at":
		never := staffNeverSignedInAsc
		if dir == page.Desc {
			never = staffNeverSignedInDesc
		}
		return `(SELECT nguoi_dung.*, COALESCE(nguoi_dung.dang_nhap_gan_nhat, ` + staffTimestamptzLiteral(never) +
			`) AS ` + staffLastLoginSortColumn + from + where, staffLastLoginSortColumn
	case "status":
		return `(SELECT nguoi_dung.*, (CASE WHEN nguoi_dung.dang_hoat_dong THEN 1 ELSE 0 END)::bigint AS ` +
			staffStatusSortColumn + from + where, staffStatusSortColumn
	}
	return "nguoi_dung", ""
}

// staffPageRow is one register row as the PAGE reads it: the person plus the computed key the
// statement scanned for this sort (zero when the sort reads a plain column). Only mocCanBo reads the
// key; DanhSach hands out the person alone.
type staffPageRow struct {
	staff    domain.CanBoTomTat
	sortText string
	sortTime time.Time
	sortInt  int64
}

// cotTomTat IS READ BY POSITION in motTomTat, exactly like cotCanBo. Same trap, same discipline:
// `co_tai_khoan` and `dang_hoat_dong` are adjacent BOOLEANs and swapping them produces no error
// anywhere — the driver scans a bool into a bool and the two values trade places for good.
//
// mat_khau_hash IS NOT IN THIS LIST, and leaving it out is the point: a credential that is never
// selected cannot leak through a handler that forgot to drop it. domain.CanBoTomTat has no field
// to put it in either, so adding it here would not even compile.
//
// `dien_thoai_co_quan` AND `di_dong_ca_nhan` ARE ADJACENT TEXT COLUMNS AND ARE THE SECOND TRAP IN THIS LIST,
// added by migration 0009 §2. A swap between them is exactly as silent as the one above — two
// strings into two strings — and it is WORSE in consequence, because the two are different kinds
// of data in law (#16): `dien_thoai_co_quan` is the office landline, duty information; `di_dong_ca_nhan` is the
// person's own mobile, personal data under Decree 13/2023/NĐ-CP. Traded over, every masking rule
// then applies to the wrong one — the switchboard number is redacted from the commune's own
// directory while the personal mobile goes out in an unmasked export. Pinned by a test that
// gives the two fixture rows DIFFERENT numbers; equal values cannot tell a swap from a correct
// read.
//
// THE FIVE MINI APP COLUMNS OF MIGRATION 0010 GO AT THE END, and the two new BOOLEANs are
// deliberately NOT adjacent: `thu_tu_danh_ba` sits between `co_zalo` and `hien_tren_mini_app`.
// Swapped, those two would publish a person because they have Zalo — the exact unconsented
// publication open question #12 forbids — so the list is laid out to make that swap a visible
// edit rather than a neighbour transposition, and the test gives the fixture rows opposite pairs
// (true,false) / (false,true) so it cannot survive either way.
//
// `coalesce(email,”)`: since migration 0019 no address is NULL; the Go side keeps its one spelling of
// "none", the empty string, so no response shape moves.
const cotTomTat = `id, ma, ho_ten, coalesce(email,''), chuc_vu,
                   coalesce(bo_phan_id,''), coalesce(vai_tro_id,''),
                   dien_thoai_co_quan, di_dong_ca_nhan, co_tai_khoan, dang_hoat_dong,
                   dang_nhap_gan_nhat, tao_luc,
                   co_zalo, thu_tu_danh_ba, hien_tren_mini_app,
                   dong_y_cong_khai_luc, dong_y_cong_khai_ghi_boi,
                   sign_in_locked_until`

// locTomTat is the ONE predicate both read paths share.
//
// deleted_at IS NULL, and NOT `da_xoa = false`: rows predate any flag column, and a direct
// equality comparison silently drops all of them. A soft-deleted person is kept for the audit
// trail (rule 7, invariant 1) and must not appear on a screen; both halves of that sentence are
// this clause.
const locTomTat = `AND deleted_at IS NULL`

// DanhSach reads ONE page of the commune's staff register.
//
// BOTH KINDS OF RECORD COME BACK — people who can sign in (co_tai_khoan) and people who exist
// only in the public directory. One `nguoi_dung` table serves two screens (migration 0003), and
// `co_tai_khoan` is returned as a FIELD so each screen decides for itself. Filtering here would
// answer, on behalf of one screen, a question the customer has not been asked.
//
// `loc` IS THE ONLY WAY A CLIENT NARROWS THE PAGE, and every value in it reaches the statement as
// a BOUND PARAMETER — see menhDeLocCanBo. The zero LocCanBo is the whole register, which is what
// GET /api/v1/staff with no filter has always returned.
//
// THE CURSOR DOES NOT ENCODE THE FILTER, following core/page (its cursor carries sort, direction,
// key and id, nothing else) and service-comms' filtered list. A cursor replayed under a different
// filter set is therefore NOT refused: it anchors at the same (sort key, id) position and returns
// the rows after it that match the NEW filter. Nothing is repeated or skipped within that filter,
// because the order is the same total order whatever the filter; what the client loses is only the
// rows before the anchor, which is what "continue from here" means.
func (s *CanBoStore) DanhSach(ctx context.Context, loc domain.LocCanBo, yc page.Request) (page.Result[domain.CanBoTomTat], error) {
	menhDe, thamSo := menhDeLocCanBo(loc)
	relation, keyColumn := staffSortRelation(yc.Column().Param, yc.Dir())
	columns := cotTomTat
	if keyColumn != "" {
		// APPENDED AT THE TAIL, like every addition to the positional scan.
		columns += ", " + keyColumn
	}
	// Cột sắp xếp KHÔNG còn được đọc ở đây. `mocCanBo` đã buộc sẵn mỗi cột với cách đọc mốc
	// của nó, và QueryPage chọn hàm đúng theo cột nó đang sắp xếp — nên callback quét không
	// cần biết gì về cột nữa, và cũng không còn cách nào chọn nhầm.
	kq, err := store.QueryPage(ctx, s.db.For(ctx), store.PageSpec{
		Columns: columns,
		Table:   relation,
		Filter:  locTomTat + menhDe,
		Args:    thamSo,
	}, yc, mocCanBo, func(rows *sql.Rows) (staffPageRow, string, error) {
		// quetTomTat, NOT motTomTat: QueryPage has already advanced the cursor to this row, and a
		// second Next() here would hand back every other row and drop the ones in between.
		var r staffPageRow
		targets := dichQuetTomTat(&r.staff)
		switch keyColumn {
		case staffDepartmentSortColumn:
			targets = append(targets, &r.sortText)
		case staffLastLoginSortColumn:
			targets = append(targets, &r.sortTime)
		case staffStatusSortColumn:
			targets = append(targets, &r.sortInt)
		}
		if err := rows.Scan(targets...); err != nil {
			return staffPageRow{}, "", fmt.Errorf("can_bo: đọc dòng: %w", err)
		}
		return r, r.staff.ID, nil
	})
	if err != nil {
		return page.NewResult[domain.CanBoTomTat](), err
	}
	out := page.Result[domain.CanBoTomTat]{
		Items:      make([]domain.CanBoTomTat, 0, len(kq.Items)),
		NextCursor: kq.NextCursor,
		HasMore:    kq.HasMore,
	}
	for _, r := range kq.Items {
		out.Items = append(out.Items, r.staff)
	}
	return out, nil
}

// CountMatching counts the rows DanhSach would walk under `loc` — the total over the register's
// current search (POST /api/v1/staff-count-queries).
//
// THE LIST'S OWN PREDICATE, NOT A COPY: locTomTat (soft-deleted excluded, rule 7) + menhDeLocCanBo,
// so a total and the list it heads cannot disagree about what a filter means. A separate method
// rather than a `total` on page.Result: core/page refuses one there on purpose.
//
// THE COMMUNE IS NOT A PARAMETER: Scoped.Query binds it to $1 from the context. A count, so nothing
// personal is read; `loc` is never logged by any caller (it may hold the typed search text).
func (s *CanBoStore) CountMatching(ctx context.Context, loc domain.LocCanBo) (int, error) {
	menhDe, thamSo := menhDeLocCanBo(loc)
	rows, err := s.db.For(ctx).Query(ctx, `count(*) AS matching`, "nguoi_dung", locTomTat+menhDe, thamSo...)
	if err != nil {
		return 0, fmt.Errorf("can_bo: đếm theo bộ lọc: %w", err)
	}
	defer rows.Close()
	var n int64
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, fmt.Errorf("can_bo: đếm theo bộ lọc: %w", err)
		}
		return 0, fmt.Errorf("can_bo: đếm theo bộ lọc: không có dòng kết quả")
	}
	if err := rows.Scan(&n); err != nil {
		return 0, fmt.Errorf("can_bo: đếm theo bộ lọc: đọc: %w", err)
	}
	return int(n), rows.Err()
}

// thoatLike escapes the three characters LIKE treats specially, so the administrator's text is
// matched LITERALLY: "50%" finds "50%", not every value starting with "50", and "_" is an
// underscore rather than "any one character". PostgreSQL's default LIKE escape character is `\`,
// so no ESCAPE clause is needed — and `\` itself is escaped first, or a trailing backslash typed
// by somebody would escape the closing `%` this code adds.
//
// A COPY OF service-comms' thoatLike (internal/store/noi_dung_mini_app.go), NOT AN IMPORT: rule 2
// forbids importing another service's internal/, and core/ is outside this change.
var thoatLike = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// menhDeLocCanBo builds the extra predicate of one register read, and its arguments.
//
// PLACEHOLDERS START AT $2: $1 is the commune, always, bound by Scoped.Query from the context. No
// value from `loc` is ever concatenated into the statement text — only the placeholder numbers are.
//
// THE TEXT SEARCH covers name, position, office number, personal mobile and — since 08/10/2026 (owner
// decision) — email and the DEPARTMENT NAME, with ILIKE on one escaped, bound pattern. When the text
// looks like a telephone number (domain.ChuSoTimSoDienThoai) it ALSO compares its digits against each
// number's digits, so "0900 000 001" finds "0900000001" whatever spacing either side was typed with.
// Shared by the register, /danh-ba and /mini-app (all POST /api/v1/staff/searches) — intended. NOT by
// GET /api/v1/staff-directory, whose own predicate must never match on the email it masks (ADR 0082):
// a search that hits on a hidden value is an oracle for it.
//
//	email            NULL since migration 0019 for a person with no address; `NULL ILIKE …` is NULL,
//	                 which an OR with the other columns treats as "this column did not match".
//	department name  an EXISTS on `bo_phan` that binds `bp.tenant_id = $1` ITSELF — Scoped.Query binds
//	                 only the outer relation, and ids alone collide across communes. LIVE units only:
//	                 the screen resolves names from the live unit list and shows "—" for a deleted one,
//	                 so a hit on a name the row does not show would look like a wrong result; the
//	                 department sort reads a deleted unit as "none" for the same reason. Correlated on
//	                 `nguoi_dung.bo_phan_id`, a name both the plain table and the derived relations of
//	                 staffSortRelation answer to.
//
// NO INDEX SERVES ANY OF THIS, and that is stated rather than hoped about: `ILIKE '%x%'` and a
// `regexp_replace` on the column both force a scan. The scan is of ONE commune's register — the
// partition and `tenant_id = $1` bound it — which is tens to a few hundred rows (the customer's own
// directory lists 26, 12-danh-ba-can-bo.md §7). If a commune ever holds thousands, the answer is a
// trigram index added by a migration, not a change here.
//
// ACCENT-INSENSITIVE SEARCH IS NOT PROVIDED: "nguyen" does not find "Nguyễn". That needs the
// `unaccent` extension, which is an infrastructure decision (ADR 0010) and was not asked for.
func menhDeLocCanBo(loc domain.LocCanBo) (string, []any) {
	var b strings.Builder
	var args []any
	so := func(v any) int { // appends v and returns its placeholder number
		args = append(args, v)
		return len(args) + 1
	}

	if loc.BoPhanID != "" {
		fmt.Fprintf(&b, " AND bo_phan_id = $%d", so(loc.BoPhanID))
	}
	if loc.CongKhai != nil {
		fmt.Fprintf(&b, " AND hien_tren_mini_app = $%d", so(*loc.CongKhai))
	}
	if loc.TuKhoa != "" {
		p := so("%" + thoatLike.Replace(loc.TuKhoa) + "%")
		fmt.Fprintf(&b, " AND (ho_ten ILIKE $%d OR chuc_vu ILIKE $%d"+
			" OR dien_thoai_co_quan ILIKE $%d OR di_dong_ca_nhan ILIKE $%d OR email ILIKE $%d"+
			" OR EXISTS (SELECT 1 FROM bo_phan bp WHERE bp.tenant_id = $1 AND bp.id = nguoi_dung.bo_phan_id"+
			" AND bp.deleted_at IS NULL AND bp.ten ILIKE $%d)", p, p, p, p, p, p)
		if chuSo := domain.ChuSoTimSoDienThoai(loc.TuKhoa); chuSo != "" {
			// Digits only, so there is nothing for LIKE to misread and nothing to escape.
			c := so("%" + chuSo + "%")
			fmt.Fprintf(&b, " OR regexp_replace(dien_thoai_co_quan, '[^0-9]', '', 'g') LIKE $%d"+
				" OR regexp_replace(di_dong_ca_nhan, '[^0-9]', '', 'g') LIKE $%d", c, c)
		}
		b.WriteString(")")
	}
	return b.String(), args
}

// ChiTiet reads one staff record by internal id.
//
// THE COMMUNE IS NOT A PARAMETER AND CANNOT BE ONE: Scoped.Query binds it to $1 from the context.
// An id belonging to another commune therefore matches no row and comes back as
// ErrCanBoKhongTonTai — which the handler answers with 404, not 403. A 403 would confirm that the
// record exists somewhere, and the existence of another authority's record is itself information
// (rule 4, forbidden #2).
func (s *CanBoStore) ChiTiet(ctx context.Context, id string) (domain.CanBoTomTat, error) {
	rows, err := s.db.For(ctx).Query(ctx, cotTomTat, "nguoi_dung", locTomTat+` AND id = $2`, id)
	if err != nil {
		return domain.CanBoTomTat{}, fmt.Errorf("can_bo: đọc chi tiết: %w", err)
	}
	defer rows.Close()
	return motTomTat(rows)
}

// mocCanBo BUỘC từng cột trong SapXepCanBo với cách đọc mốc của cột ấy.
//
// Bản trước là một `switch` trên `col.SQL` mà bên gọi tự phân nhánh, kèm một nhánh `default`
// trả lỗi để phòng trường hợp danh sách trắng và switch lệch nhau. Nó phòng được đúng một
// nửa: thêm một cột vào SapXepCanBo mà quên thêm nhánh thì lỗi nổ ở YÊU CẦU ĐẦU TIÊN dùng cột
// đó — nghĩa là sau khi đã phát hành.
//
// `store.NewMoc` đối chiếu hai danh sách ngay lúc dựng, nên cùng sai sót ấy nay là một panic
// lúc khởi động. Và vì hàm đọc mốc được chọn THEO CHÍNH cột đang sắp xếp, việc lẫn hai cột
// cùng kiểu — thứ không phép kiểm nào bắt được trước đây — không còn dựng được nữa.
//
// THE THREE KindRef SORTS READ NOTHING: their cursor carries the id alone (page.KindRef), which is the
// whole point — the name, position or office number never reaches `next_cursor`. THE THREE COMPUTED
// SORTS READ THE KEY THE STATEMENT SCANNED (staffPageRow), sentinel included, so the anchor is the very
// value the SQL compared.
var mocCanBo = store.NewMoc[staffPageRow](SapXepCanBo,
	map[string]func(staffPageRow) page.Key{
		"code":          func(r staffPageRow) page.Key { return page.TextKey(r.staff.Ma) },
		"created_at":    func(r staffPageRow) page.Key { return page.TimeKey(r.staff.TaoLuc) },
		"full_name":     staffRefKey,
		"position":      staffRefKey,
		"phone":         staffRefKey,
		"department":    func(r staffPageRow) page.Key { return page.TextKey(r.sortText) },
		"last_login_at": func(r staffPageRow) page.Key { return page.TimeKey(r.sortTime) },
		"status":        func(r staffPageRow) page.Key { return page.IntKey(r.sortInt) },
	})

// staffRefKey is empty BY DESIGN — see page.KindRef.
func staffRefKey(staffPageRow) page.Key { return page.RefKey() }

// motTomTat reads the FIRST row of a single-row read, or reports that there is none.
func motTomTat(rows quetDuoc) (domain.CanBoTomTat, error) {
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.CanBoTomTat{}, fmt.Errorf("can_bo: đọc: %w", err)
		}
		return domain.CanBoTomTat{}, ErrCanBoKhongTonTai
	}
	return quetTomTat(rows)
}

// quetTomTat scans the row the cursor is ALREADY on. POSITIONAL — this list of Scan targets must
// stay in lockstep with cotTomTat. See the note there on the two adjacent booleans.
//
// &cb.DangNhapGanNhat is a **time.Time, which is how database/sql is told that SQL NULL is a
// legitimate answer: it sets the pointer to nil instead of failing the scan. A plain time.Time
// target would make every person who has never signed in an error, and the register would stop
// listing precisely the accounts an administrator opened it to find.
func quetTomTat(rows quetDuoc) (domain.CanBoTomTat, error) {
	var cb domain.CanBoTomTat
	if err := rows.Scan(dichQuetTomTat(&cb)...); err != nil {
		return domain.CanBoTomTat{}, fmt.Errorf("can_bo: đọc dòng: %w", err)
	}
	return cb, nil
}

// dichQuetTomTat is THE ONE list of Scan targets for cotTomTat, shared by quetTomTat (the register)
// and quetMotDong (the row a write decides on).
//
// ONE LIST BECAUSE THERE USED TO BE TWO, each a positional copy of the other, and migration 0010
// adds five columns to both — two copies of eighteen positions is two places for the same silent
// swap. &cb.ThuTuDanhBa and &cb.DongYCongKhaiLuc are pointer-to-pointer for the same reason as
// &cb.DangNhapGanNhat: SQL NULL is a legitimate answer ("no explicit order", "no consent").
func dichQuetTomTat(cb *domain.CanBoTomTat) []any {
	return []any{&cb.ID, &cb.Ma, &cb.HoTen, &cb.Email, &cb.ChucVu,
		&cb.BoPhanID, &cb.VaiTroID, &cb.DienThoaiCoQuan, &cb.DiDongCaNhan,
		&cb.CoTaiKhoan, &cb.DangHoatDong,
		&cb.DangNhapGanNhat, &cb.TaoLuc,
		&cb.CoZalo, &cb.ThuTuDanhBa, &cb.HienTrenMiniApp,
		&cb.DongYCongKhaiLuc, &cb.DongYCongKhaiGhiBoi,
		// Migration 0020. **time.Time: NULL is "never auto-locked". Only the stored instant is read;
		// whether it is in force is derived at the edge (domain.CanBoTomTat.SignInLockedAt).
		&cb.SignInLockedUntil}
}
