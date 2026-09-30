package store

// The TASK REGISTER — the paginated list and the single-task read. SQL, and nothing else.
//
// FOUR THINGS HOLD ACROSS EVERY METHOD IN THIS FILE, each a defect class rather than a style:
//
//  1. THE COMMUNE IS $1 IN EVERY STATEMENT, from the context (rule 1, invariants 4 and 5). It is
//     never a parameter here, so no caller can reach another commune's register.
//  2. NOTHING HERE OPENS A TRANSACTION, and nothing here WRITES. This pass is the read path; the
//     write use cases come next and will take a *store.ScopedTx so the audit entry cannot be
//     written anywhere but inside the same transaction (rule 6, invariant 3).
//  3. EVERY READ EXCLUDES SOFT-DELETED ROWS (rule 7, invariant 2) — the list and the single read
//     alike. A task removed from the register does not come back through a number somebody still
//     has written in a meeting's minutes.
//  4. EVERY FILTER VALUE IS A BOUND PARAMETER. A filter assembled as text anywhere above this line
//     is an injection point in a government register.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// NhiemVuStore is the only path to `nhiem_vu` (migration 0006).
//
// EVERY READ GOES THROUGH *store.Scoped, which binds `tenant_id` to $1 from the context. The
// commune is therefore not a parameter of any method here and cannot be made into one — a
// repository that can be built without a commune is a repository that can query across communes.
type NhiemVuStore struct {
	db *store.DB
}

func NewNhiemVuStore(db *store.DB) *NhiemVuStore { return &NhiemVuStore{db: db} }

// ErrNhiemVuKhongTonTai means no LIVE task of THIS commune carries that number.
//
// ONE ERROR FOR "no such number", "another commune's number" AND "soft deleted", and the caller
// answers 404 for all three. Telling them apart tells a caller which numbers exist in a commune
// they cannot read.
var ErrNhiemVuKhongTonTai = errors.New("nhiem_vu: không có nhiệm vụ")

// cotNhiemVu IS READ BY POSITION in the Scan below.
//
// `co_quan_chu_tri_id` AND `chuyen_vien_theo_doi_ma` ARE NOT HERE, ON PURPOSE (ADR 0065 NV5, migration
// 0025): the lead unit IS `bo_phan_id` and the monitoring officer IS `nguoi_thuc_hien_ma`. The two
// retired columns stay in the table (rule 7) and no read in this package names them.
//
// `han_xu_ly`, `han_ban_dau` AND `ngay_hoan_thanh` ARE THREE ADJACENT TIMESTAMPTZ COLUMNS and a swap
// between any two produces NO error at all — it produces a task that looks extended when it was
// not, or an on-time ratio measured against the wrong number. They are listed together on purpose,
// so the group is read as a group.
//
// `tao_luc` IS IN THIS ONE LIST, unlike the petition register's split pair, because a task has ONE
// creation instant. `phieu_phan_anh` needed two constants because `vao_so_luc` (a business fact) and
// `tao_luc` (a row-lifecycle fact) can differ by a week, and putting both in the shape every read
// returns invited a screen to render the wrong one. Here there is nothing to confuse it with.
const cotNhiemVu = `id, ma, loai, khoi, tieu_de, mo_ta, trang_thai, muc_uu_tien,
	nguon_giao, nguon_id,
	bo_phan_id, nguoi_thuc_hien_ma, lanh_dao_giao_viec_ma,
	han_xu_ly, han_ban_dau, ngay_hoan_thanh,
	tien_do, tom_tat_ket_qua, ghi_chu,
	lanh_dao_phe_duyet_hoan_thanh, cap_tren_cong_nhan_hoan_thanh,
	nguoi_tao_ma, tao_luc, nhiem_vu_cha_id,
	cap_nhat_luc`

// SapXepNhiemVu is the closed set of sorts GET /api/v1/tasks offers.
//
// `created_at` IS THE DEFAULT, DESCENDING — newest first, which is what an officer opening the
// board in the morning needs. `tao_luc` is NOT NULL and indexed (`nhiem_vu_so`, migration 0006),
// which is what page.QueryPage needs from a sort column.
//
// `code` IS OFFERED AND THE PETITION REGISTER DELIBERATELY DOES NOT OFFER ITS EQUIVALENT, and the
// difference is what the two codes ARE. `ma_tra_cuu` is random, so sorting by it is sorting by
// noise; `nhiem_vu.ma` is the commune's own register sequence — NV01, NV02… — so ordering by it is
// the Sổ theo dõi's own order (§4.3). It is NOT NULL and carries UNIQUE (tenant_id, ma), so the
// cursor has a stable total order.
//
// `due_at` IS OFFERED SINCE 28/09/2026 (the owner approved the design that day), AND IT IS NOT
// `han_xu_ly` ITSELF. The column is NULLABLE, and `(col, id) > (…)` is NULL for a NULL col, so a
// plain sort on it would make every task with no deadline vanish from page two onward — the reason
// this allowlist refused it until now. What it sorts on instead is a NOT NULL key computed per row in
// taskByDueTable: the deadline, or a SENTINEL for "no deadline" chosen per DIRECTION so those tasks
// come LAST both ways (after the latest deadline ascending, after the earliest descending). Because
// the key is never NULL, store.QueryPage's keyset stays a total order across the NULL boundary and
// keeps owning the ORDER BY, the anchor and the limit+1 — nothing here re-implements them.
//
// ONE PARAMETER, TWO ALLOWLISTS: `sort=due_at` maps to `due_sort_desc` here and to `due_sort_asc` in
// taskSortAscAllowlist. HANDLERS PARSE AGAINST SapXepNhiemVu ONLY — it is the one package-level list
// tools/apidoc reads to publish the `sort` enum (`created_at`, `code`, `due_at`) in openapi.json, so a
// second list reachable from the handler would hide the enum. DanhSach then REBUILDS an ascending
// `due_at` request against taskSortAscAllowlist (forDueDirection), carrying the limit and the
// anchor over unchanged. The cursor records both the parameter and the direction (page.Decode), so a
// cursor from one direction cannot be replayed on the other. A request whose key and direction still
// disagree after that is refused — that pairing is what puts the no-deadline tasks last.
//
// `priority` AND `title` ARE OFFERED SINCE 28/09/2026 (§4.2's sortable columns), each in the one shape
// that is not wrong:
//
//	priority  NOT the code — a code is not a rank, and alphabetically `cao` sorts above `khan`, a
//	          board that looks sorted and is not. The key is the commune's OWN catalogue order,
//	          `muc_uu_tien_nhiem_vu.thu_tu`, joined in taskByPriorityTable; a task with NO priority
//	          sorts LAST in both directions through the same two-key/sentinel scheme as `due_at`
//	          (priority_sort_asc / priority_sort_desc). The cursor carries that integer, which is
//	          no personal data.
//	title     KindRef (core/page): the cursor carries ONLY the anchor row's id and the title is
//	          looked up server-side from that row. A task title quotes a citizen's complaint often
//	          enough that putting it in `next_cursor` — a URL, an access log, a browser history —
//	          is rule 3, forbidden #4. `tieu_de` is NOT NULL (migration 0006 refuses a blank one).
//	          Order is the database collation's; if the anchor's title is edited between two pages
//	          the walk resumes from its new position (page.KindRef states the cost).
//
// WHAT IS STILL DELIBERATELY ABSENT: `han_xu_ly` as a raw column, for the NULL reason above — `due_at`
// is the only way in.
var SapXepNhiemVu = page.NewAllowlist(page.Desc,
	page.Col("created_at", "tao_luc", page.KindTime),
	page.Col("code", "ma", page.KindText),
	page.Col("due_at", dueSortDescColumn, page.KindTime),
	page.Col("priority", prioritySortDescColumn, page.KindInt),
	page.Col("title", "tieu_de", page.KindRef),
)

// taskSortAscAllowlist is SapXepNhiemVu for `order=asc`: identical but for the keys `due_at` and
// `priority` map to. Same default (created_at, descending), so a request naming no sort reads the
// same page from both.
var taskSortAscAllowlist = page.NewAllowlist(page.Desc,
	page.Col("created_at", "tao_luc", page.KindTime),
	page.Col("code", "ma", page.KindText),
	page.Col("due_at", dueSortAscColumn, page.KindTime),
	page.Col("priority", prioritySortAscColumn, page.KindInt),
	page.Col("title", "tieu_de", page.KindRef),
)

// ascendingParamOf names the params whose ascending key differs from the descending one.
var ascendingParamOf = map[string]string{
	dueSortDescColumn:      "due_at",
	prioritySortDescColumn: "priority",
}

// forDueDirection turns an ASCENDING `due_at` or `priority` request parsed against SapXepNhiemVu into
// the same request against taskSortAscAllowlist, so it sorts on the ascending key. Every other request
// is returned as it is.
//
// NOTHING IS RE-VALIDATED BY HAND: the rebuilt request goes through page.New, which decodes the
// re-encoded anchor exactly as it decodes a client's cursor. The anchor itself is the one the
// client sent, so page two continues from the same row.
func forDueDirection(yc page.Request) (page.Request, error) {
	param, twoKeys := ascendingParamOf[yc.Column().SQL]
	if !twoKeys || yc.Dir() != page.Asc {
		return yc, nil
	}
	asc, ok := columnOf(taskSortAscAllowlist, param)
	if !ok {
		return page.Request{}, ErrTaskSortDirection
	}
	cursor := ""
	if a, ok := yc.After(); ok {
		cursor = page.Encode(asc, page.Asc, a)
	}
	return page.New(taskSortAscAllowlist, param, string(page.Asc), strconv.Itoa(yc.Limit()), cursor)
}

// columnOf finds one allowlisted column by its parameter name.
func columnOf(a page.Allowlist, param string) (page.Column, bool) {
	for _, c := range a.Columns() {
		if c.Param == param {
			return c, true
		}
	}
	return page.Column{}, false
}

// The two computed sort keys and their sentinels.
//
// THE SENTINELS ARE GO VALUES AND THE SQL LITERALS ARE BUILT FROM THEM (timestamptzLiteral), because
// the anchor of a no-deadline row is read in Go (mocNhiemVu / taskAscAnchors) and compared in SQL:
// two hand-written spellings of one instant that differed by a second would re-open the exact hole
// this key closes — the first no-deadline row of page two would sort on the wrong side of the anchor.
//
//	desc       the zero time.Time, 0001-01-01 00:00:00 UTC — smaller than any deadline a commune can
//	           set, and EXACTLY what quetNhiemVu leaves in HanXuLy for a NULL
//	ascending  9999-12-31 00:00:00 UTC — larger than any deadline a commune can set
//
// A real deadline equal to a sentinel would tie with the no-deadline rows and be ordered among them
// by id — still no row lost or repeated, because (key, id) is still a total order.
const (
	dueSortAscColumn  = "due_sort_asc"
	dueSortDescColumn = "due_sort_desc"
)

var (
	dueSortAscNoDeadline  = time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC)
	dueSortDescNoDeadline = time.Time{}
)

// timestamptzLiteral spells an instant as a PostgreSQL literal. Only ever called on the two sentinels
// above — never on a value from a request.
func timestamptzLiteral(t time.Time) string {
	return "TIMESTAMPTZ '" + t.UTC().Format("2006-01-02 15:04:05") + "+00'"
}

// taskByDueTable is the relation a `sort=due_at` page is cut from: every column of `nhiem_vu`, plus
// the two NOT NULL keys. A DERIVED TABLE handed to store.QueryPage, the shape bangDeNghiChoDuyet
// already uses, so QueryPage keeps owning the whole keyset.
//
// `SELECT nhiem_vu.*` SO EVERY FILTER OF locNhiemVuThanhSQL READS THE SAME UNQUALIFIED COLUMNS it
// reads on the plain table — one predicate builder for both shapes.
//
// `tenant_id = $1` INSIDE AS WELL AS OUTSIDE: QueryPage adds the outer one; the inner one keeps the
// partition pruning and the commune bound visible in the text of the subquery itself (rule 1,
// invariant 5), rather than resting on the planner pushing the outer predicate down.
//
// ⚠ NO INDEX SERVES THIS ORDER. `nhiem_vu_han` (migration 0006) is on the raw column and excludes
// NULLs, so ordering by the COALESCE key sorts the commune's live rows in memory. That is bounded by
// one commune's register and affordable at today's sizes; an expression index on the two keys is a
// migration, which this pass does not own — reported.
var taskByDueTable = `(SELECT nhiem_vu.*, COALESCE(han_xu_ly, ` + timestamptzLiteral(dueSortAscNoDeadline) +
	`) AS ` + dueSortAscColumn + `, COALESCE(han_xu_ly, ` + timestamptzLiteral(dueSortDescNoDeadline) +
	`) AS ` + dueSortDescColumn + ` FROM nhiem_vu WHERE tenant_id = $1) AS nv`

// The two computed PRIORITY keys, same scheme as the due-date pair above: the catalogue's `thu_tu`,
// or a per-direction sentinel so a task with no priority comes LAST both ways. The sentinels are the
// int64 extremes, so no `thu_tu` a commune can type (an INT) ties with them.
const (
	prioritySortAscColumn  = "priority_sort_asc"
	prioritySortDescColumn = "priority_sort_desc"
)

// taskByPriorityTable is the relation a `sort=priority` page is cut from: every column of `nhiem_vu`
// plus the two NOT NULL keys, through a LEFT JOIN on the commune's priority catalogue.
//
//	m.tenant_id = $1          the joined table is bound to the commune too (QueryJoin's contract) —
//	                          `UNIQUE (tenant_id, ma)` then makes the join at most one row per task
//	no deleted_at on m         a priority removed from the catalogue keeps its place in the order: the
//	                          tasks still carry the code (a real FOREIGN KEY), and the order they were
//	                          given is still the commune's
//	LEFT                      a task with no priority keeps its row, with the sentinel
//
// ⚠ NO INDEX SERVES THIS ORDER; the commune's live rows are sorted in memory, as for `due_at`.
var taskByPriorityTable = `(SELECT nhiem_vu.*, COALESCE(m.thu_tu::bigint, 9223372036854775807) AS ` +
	prioritySortAscColumn + `, COALESCE(m.thu_tu::bigint, -9223372036854775808) AS ` + prioritySortDescColumn +
	` FROM nhiem_vu LEFT JOIN muc_uu_tien_nhiem_vu m ON m.tenant_id = $1 AND m.ma = nhiem_vu.muc_uu_tien` +
	` WHERE nhiem_vu.tenant_id = $1) AS nv`

// ErrTaskSortDirection means a `due_at` key reached DanhSach with the direction it was not built for —
// a caller parsed against SapXepNhiemVu while asking for `asc`, or the reverse. Refused rather than
// served: the page would put every task without a deadline FIRST, while the screen says it sorts
// them last.
var ErrTaskSortDirection = errors.New("nhiem_vu: khoá sắp xếp theo hạn không khớp chiều sắp xếp")

// TimNhiemVuToiDa bounds the free-text search. Longer than any phrase a clerk types, short enough
// that the pattern cannot become a payload.
const TimNhiemVuToiDa = 200

// ErrTimNhiemVuQuaDai — the search box was sent more than TimNhiemVuToiDa characters.
var ErrTimNhiemVuQuaDai = fmt.Errorf(
	"nhiem_vu: chuỗi tìm kiếm quá dài (tối đa %d ký tự)", TimNhiemVuToiDa)

// LocNhiemVu is the set of filters the list route accepts, already validated by the handler.
//
// A STRUCT AND NOT A STRING OF SQL: every field below becomes a BOUND PARAMETER.
//
// §3's "Sắp đến hạn" and "Liên quan đến tôi" ARE HERE SINCE 28/09/2026 (DueSoon*, Related), each
// resolved by the handler from identity BEFORE the store is reached: the threshold instant from
// ResolveDueSoonCutoff (identity walks the commune's own `sla.gio_sap_den_han` in working hours —
// no hour count lives here), and the caller's own unit(s) from StaffOrgUnits. The store sees only
// bound values.
type LocNhiemVu struct {
	TrangThai string // "" = every status
	Loai      string // "" = every task type
	Khoi      string // "" = every bloc, including none
	MucUuTien string // "" = every priority, including none
	BoPhanID  string // "" = every department, including none
	NguonGiao string // "" = every source
	Tim       string // "" = no text search

	// NguoiThucHienMa is BOTH the "Người thực hiện" picker of §3 and the whole of the
	// `Giao cho tôi` scope — one predicate, because they are one question ("whose name is on this
	// task"). The handler is what decides the value: the picker supplies a code from the request,
	// the scope supplies the CALLER'S OWN code from the session and never from the request
	// (rule 4, invariant 2 in its staff form).
	NguoiThucHienMa string

	// ChiTreHan restricts the page to tasks whose CURRENT commitment was missed.
	//
	// DERIVED IN SQL, NEVER READ FROM A COLUMN (rule 10, invariant 3). See the predicate in
	// locNhiemVuThanhSQL, and read the warning there before touching it.
	ChiTreHan bool

	// Metric restricts the page to the rows behind ONE overview figure (GET /api/v1/task-summary),
	// through the very predicate that figure is counted with — taskMetricCondition. "" = no metric.
	//
	// Period is read only when Metric is period-bound, and must then be valid; the stock figures
	// ignore it. Both are validated by the handler and again by DanhSach (validateTaskMetric).
	Metric domain.TaskMetric
	Period domain.Period

	// ParentCode restricts the page to the DIRECT children of the task carrying this register number
	// (§5.10's "Nhiệm vụ con" block). "" = no restriction.
	//
	// A CODE AND NOT AN INTERNAL id, resolved inside the statement against this commune's LIVE tasks.
	// A number that matches nothing — another commune's, a soft-deleted one, a typo — yields an empty
	// page, the same answer as a task with no children, so the filter reveals nothing about records
	// the caller cannot see.
	ParentCode string

	// DueSoonFrom and DueSoonUntil are §3's "Sắp đến hạn": `DueSoonFrom < han_xu_ly <= DueSoonUntil`,
	// unfinished work only. BOTH ZERO = no filter; the store refuses one without the other
	// (ErrTaskDueSoonWindow), because a half-open window is a different question.
	//
	// DueSoonFrom IS THE SAME `now` THE HANDLER PASSED TO identity AS asOf, and DueSoonUntil is
	// identity's answer for it — one instant for both ends, so a task cannot fall between the two.
	DueSoonFrom  time.Time
	DueSoonUntil time.Time

	// Related is §3's "Liên quan đến tôi" (`scope=related`). nil = no filter. A POINTER so the struct
	// stays comparable (the handler tests compare it whole) while carrying a list.
	Related *TaskRelatedScope
}

// TaskRelatedScope is who "tôi" is for `scope=related`, resolved by the handler from the SESSION —
// never from the request (rule 4, invariant 2 in its staff form).
//
// StaffCode is authz.Principal.Ma. OrgUnits is identity's StaffOrgUnits answer for that code: EMPTY
// IS AN ORDINARY ANSWER and means the unit clause matches nothing — never "every unit".
type TaskRelatedScope struct {
	StaffCode string
	OrgUnits  []string
}

// ErrTaskDueSoonWindow — one end of the due-soon window without the other, or an inverted window.
var ErrTaskDueSoonWindow = errors.New("nhiem_vu: cửa sổ `sắp đến hạn` thiếu một đầu hoặc bị ngược")

// ErrTaskRelatedNoStaff — `scope=related` with no staff code. Refused rather than run: with no code
// every personal clause matches nothing and the tab would show only the unit's work, silently.
var ErrTaskRelatedNoStaff = errors.New("nhiem_vu: `scope=related` không có mã cán bộ")

// validateTaskListScope refuses the two shapes of LocNhiemVu that cannot be served honestly.
func validateTaskListScope(loc LocNhiemVu) error {
	if loc.DueSoonFrom.IsZero() != loc.DueSoonUntil.IsZero() {
		return ErrTaskDueSoonWindow
	}
	// Equal ends are an EMPTY window (a zero threshold), which is a true empty page; an end BEFORE
	// the start is a broken answer from identity and is refused.
	if !loc.DueSoonFrom.IsZero() && loc.DueSoonUntil.Before(loc.DueSoonFrom) {
		return ErrTaskDueSoonWindow
	}
	if loc.Related != nil && loc.Related.StaffCode == "" {
		return ErrTaskRelatedNoStaff
	}
	return nil
}

// dieuKienTimNhiemVu is the free-text predicate, built with the placeholder already chosen.
//
// WRITTEN BY CONCATENATION RATHER THAN fmt.Sprintf for the same reason the petition register's
// equivalent is: `hooks/pii_guard` blocks a formatting call near a personal-data column name and is
// right to, so the honest move is to stop using the shape the guard watches rather than argue with
// it. §3 says the box searches "trong mã + tiêu đề" — those two columns and no others.
func dieuKienTimNhiemVu(n int) string {
	p := "$" + strconv.Itoa(n)
	return " AND (ma ILIKE " + p + " OR tieu_de ILIKE " + p + ")"
}

// locNhiemVuThanhSQL turns the validated filter struct into a predicate and its bound values.
//
// ONE FUNCTION BUILDING BOTH HALVES, because the placeholder numbers and the argument order are one
// fact: written apart, a filter added to one half and forgotten in the other produces a query that
// silently reads the wrong column's value.
func locNhiemVuThanhSQL(loc LocNhiemVu) (string, []any) {
	var (
		dieuKien string
		args     []any
	)
	them := func(mau string, gt any) {
		args = append(args, gt)
		dieuKien += fmt.Sprintf(mau, len(args)+1) // $1 is the commune
	}

	if loc.TrangThai != "" {
		them(" AND trang_thai = $%d", loc.TrangThai)
	}
	if loc.Loai != "" {
		them(" AND loai = $%d", loc.Loai)
	}
	if loc.Khoi != "" {
		them(" AND khoi = $%d", loc.Khoi)
	}
	if loc.MucUuTien != "" {
		them(" AND muc_uu_tien = $%d", loc.MucUuTien)
	}
	if loc.BoPhanID != "" {
		them(" AND bo_phan_id = $%d", loc.BoPhanID)
	}
	if loc.NguoiThucHienMa != "" {
		them(" AND nguoi_thuc_hien_ma = $%d", loc.NguoiThucHienMa)
	}
	if loc.NguonGiao != "" {
		them(" AND nguon_giao = $%d", loc.NguonGiao)
	}
	if loc.ParentCode != "" {
		// THE PARENT IS RESOLVED BY ITS REGISTER NUMBER INSIDE THE STATEMENT, against this commune's
		// LIVE rows. `UNIQUE (tenant_id, ma)` makes the scalar subquery return at most one id; a number
		// that matches nothing makes it NULL, `= NULL` matches no row, and the page is empty — the
		// same answer a childless task gets. Served by `nhiem_vu_cha` (migration 0008).
		them(" AND nhiem_vu_cha_id = (SELECT p.id FROM nhiem_vu p"+
			" WHERE p.tenant_id = $1 AND p.ma = $%d AND p.deleted_at IS NULL)", loc.ParentCode)
	}
	if loc.Tim != "" {
		// ILIKE ON TWO COLUMNS — the register number a clerk remembers and the title. A leading
		// wildcard cannot use an index, which is affordable because the scan is already bound to one
		// commune.
		args = append(args, "%"+loc.Tim+"%")
		dieuKien += dieuKienTimNhiemVu(len(args) + 1)
	}

	if loc.ChiTreHan {
		// OVERDUE IS DERIVED HERE TOO, AND THIS IS THE SECOND EXPRESSION OF domain.NhiemVu.TreHan —
		// say so rather than let somebody discover it. Go cannot run inside a WHERE clause, so a
		// register filtered on "late" has no other shape; what this comment buys is that the two are
		// read together when either changes.
		//
		// IT MIRRORS domain.NhiemVu.TreHan LINE FOR LINE:
		//
		//	no deadline  -> NOT late. Nothing was promised, so nothing can have been missed.
		//	finished     -> compare the two RECORDED instants, so work finished late STAYS late and
		//	                last quarter's figure does not change on every read.
		//	not finished -> compare the deadline with now.
		//
		// ⚠ `han_xu_ly` AND NOT `han_ban_dau`, AND THE TWO ARE NOT INTERCHANGEABLE. This filter is
		// the register's "Chỉ việc quá hạn" box: is this work late as things stand TODAY, after any
		// extension the leader granted. §11.3's ON-TIME RATIO is the other question and is measured
		// against `han_ban_dau`; swapping them here would make the box list tasks whose extension was
		// approved, which is precisely the complaint the extension was granted to answer.
		//
		// `now()` AND NOT A PARAMETER: the database's clock is the one the deadline was stored
		// against, and a `now` passed from a handler is a second clock that can disagree with it.
		dieuKien += " AND " + dieuKienTreHan
	}

	if !loc.DueSoonFrom.IsZero() {
		// §3's "Sắp đến hạn": the CURRENT commitment (`han_xu_ly`, after any approved extension —
		// the same column the late filter reads) falls inside (from, until], and the work is not
		// finished. `ngay_hoan_thanh IS NULL` is the same "not finished" dieuKienTreHan uses; a
		// `chuyen-tiep` row is unfinished and so is included.
		//
		// BOTH ENDS ARE BOUND VALUES from the handler, not now(): `from` is the asOf identity
		// computed `until` for, so the window is one instant wide at its start. The late filter
		// reads the database clock; the two differ by the app/DB clock skew, which is why a task at
		// the exact boundary is decided by `>` here and `<` there rather than both claiming it.
		args = append(args, loc.DueSoonFrom)
		from := "$" + strconv.Itoa(len(args)+1)
		args = append(args, loc.DueSoonUntil)
		until := "$" + strconv.Itoa(len(args)+1)
		dieuKien += " AND han_xu_ly IS NOT NULL AND ngay_hoan_thanh IS NULL" +
			" AND han_xu_ly > " + from + " AND han_xu_ly <= " + until
	}

	if loc.Related != nil {
		dieuKien += relatedCondition(loc.Related, &args)
	}

	if loc.Metric != "" {
		// THE FIGURE'S OWN PREDICATE, NOT A LOOK-ALIKE — see task_summary.go. ANDed with every other
		// filter, so the page equals the figure only when no other filter narrows it.
		var extra string
		extra, args = taskMetricFilter(loc.Metric, loc.Period, args)
		dieuKien += extra
	}
	return dieuKien, args
}

// relatedCondition is §3's "Liên quan đến tôi" — "tôi giao, tôi theo dõi, tôi đã xử lý, hoặc bộ
// phận tôi đang giữ" — as ONE OR over bound values, appended to args.
//
// THE CLAUSES, and where each comes from (vigov-require 0053854, tasks/repository.py `_mine_clause`,
// read against this schema):
//
//	nguoi_thuc_hien_ma       I hold it — and "tôi theo dõi": since ADR 0065 NV5 the monitoring
//	                         officer IS the assignee. require includes the assignee in "involved"
//	                         too, so the `related` tab is a SUPERSET of `mine`, never a different list.
//	lanh_dao_giao_viec_ma    "tôi giao" — the leader named as having handed it out
//	nguoi_tao_ma             "tôi giao" as require spells it (`created_by`): the officer who
//	                         entered the assignment. Both are kept; either is "I gave this out".
//	nhat_ky_nhiem_vu         "tôi đã xử lý" — I wrote an entry on its timeline (require: a progress
//	                         report of mine). Every act on a task writes one there as its actor, so
//	                         a status move or a manual log entry by me counts. The timeline is
//	                         append-only (migration 0006) — no soft-delete column to filter.
//	bo_phan_id               "bộ phận tôi đang giữ" — held by one of my live units (require matches
//	                         org_unit_id and lead_org_unit_id; the lead unit IS bo_phan_id since
//	                         ADR 0065 NV5). NO units → the clause is ABSENT, which matches nothing,
//	                         never every unit.
//
// The retired `chuyen_vien_theo_doi_ma` / `co_quan_chu_tri_id` are NOT read (migration 0025).
//
// ALL COLUMNS ARE STAFF BUSINESS CODES OR UNIT ids — no personal data (rule 3). The timeline clause is
// `id IN (…)` rather than a correlated EXISTS so the outer `id` binds to the task relation under both
// shapes DanhSach reads (`nhiem_vu` and the aliased due-sort table) without naming either.
//
// ⚠ NO INDEX SERVES `nhat_ky_nhiem_vu.nguoi_ma`; the subquery scans this commune's timeline rows.
// Bounded by one commune and affordable today; an index is a migration this pass does not own.
func relatedCondition(r *TaskRelatedScope, args *[]any) string {
	*args = append(*args, r.StaffCode)
	me := "$" + strconv.Itoa(len(*args)+1) // $1 is the commune
	clauses := []string{
		"nguoi_thuc_hien_ma = " + me,
		"lanh_dao_giao_viec_ma = " + me,
		"nguoi_tao_ma = " + me,
		"id IN (SELECT nk.nhiem_vu_id FROM nhat_ky_nhiem_vu nk WHERE nk.tenant_id = $1 AND nk.nguoi_ma = " + me + ")",
	}
	if len(r.OrgUnits) > 0 {
		var b strings.Builder
		b.WriteString("(")
		for i, u := range r.OrgUnits {
			if i > 0 {
				b.WriteString(", ")
			}
			*args = append(*args, u)
			b.WriteString("$" + strconv.Itoa(len(*args)+1))
		}
		b.WriteString(")")
		clauses = append(clauses, "bo_phan_id IN "+b.String())
	}
	return " AND (" + strings.Join(clauses, " OR ") + ")"
}

// dieuKienTreHan is THE ONE SQL SPELLING of domain.NhiemVu.TreHan. Two readers use it: the
// register's "Chỉ việc quá hạn" filter above, and the meeting register's per-conclusion late count
// (cauKetLuanKemDem). Before it was a constant it was written inline in the filter; the conclusion
// count would otherwise have been a THIRD spelling, and the one that drifts is the one on the report.
//
// THE COLUMNS ARE UNQUALIFIED ON PURPOSE, so the text is byte-identical in both places. Inside the
// meeting query it runs in the LATERAL subquery whose only FROM item is `nhiem_vu nv`; SQL resolves
// an unqualified name against the innermost FROM first, so they bind to `nv` and never to the outer
// `ket_luan_hop k`.
const dieuKienTreHan = `han_xu_ly IS NOT NULL AND (
			(ngay_hoan_thanh IS NULL AND han_xu_ly < now())
			OR (ngay_hoan_thanh IS NOT NULL AND ngay_hoan_thanh > han_xu_ly))`

// mocNhiemVu BINDS each allowlisted sort column to the way that column's cursor value is read out of
// a scanned row. store.NewMoc compares the two lists AT CONSTRUCTION, so a sort added to
// SapXepNhiemVu without a reader here is a panic at startup rather than an error on the first
// request that uses it — which would be after release.
//
// `due_at` READS THE SAME KEY THE SQL SORTS ON, sentinel included: a no-deadline task's anchor is the
// sentinel of ITS direction, which is why the two directions have two readers. A NULL deadline scans
// to the zero time.Time, which is exactly dueSortDescNoDeadline, so the descending reader is the raw
// field; the ascending one substitutes its own sentinel.
var mocNhiemVu = store.NewMoc[domain.NhiemVu](SapXepNhiemVu,
	map[string]func(domain.NhiemVu) page.Key{
		"created_at": func(n domain.NhiemVu) page.Key { return page.TimeKey(n.TaoLuc) },
		"code":       func(n domain.NhiemVu) page.Key { return page.TextKey(n.Ma) },
		"due_at":     dueDescKey,
		"priority":   priorityKey,
		"title":      titleKey,
	})

// taskAscAnchors is mocNhiemVu for taskSortAscAllowlist. Only `due_at` differs: `priority` reads the
// key the statement itself scanned for THIS direction, so one reader serves both.
var taskAscAnchors = store.NewMoc[domain.NhiemVu](taskSortAscAllowlist,
	map[string]func(domain.NhiemVu) page.Key{
		"created_at": func(n domain.NhiemVu) page.Key { return page.TimeKey(n.TaoLuc) },
		"code":       func(n domain.NhiemVu) page.Key { return page.TextKey(n.Ma) },
		"due_at":     dueAscKey,
		"priority":   priorityKey,
		"title":      titleKey,
	})

// priorityKey reads the priority key the page statement scanned into PriorityRank — the catalogue
// position, or the sentinel of the direction being read (taskByPriorityTable).
func priorityKey(n domain.NhiemVu) page.Key { return page.IntKey(n.PriorityRank) }

// titleKey is empty BY DESIGN: `title` is page.KindRef, so the cursor carries the row id alone and the
// title never leaves the server (rule 3, forbidden #4).
func titleKey(domain.NhiemVu) page.Key { return page.RefKey() }

// dueAscKey and dueDescKey are the Go spelling of the two COALESCE keys in taskByDueTable. NAMED, so
// the test that walks pages across the NULL boundary drives the very readers the cursor is built
// from rather than a copy of them.
func dueAscKey(n domain.NhiemVu) page.Key {
	if n.HanXuLy.IsZero() {
		return page.TimeKey(dueSortAscNoDeadline)
	}
	return page.TimeKey(n.HanXuLy)
}

func dueDescKey(n domain.NhiemVu) page.Key {
	if n.HanXuLy.IsZero() {
		return page.TimeKey(dueSortDescNoDeadline)
	}
	return page.TimeKey(n.HanXuLy)
}

// taskPageShape picks the relation, the anchor readers and — for `priority` — the extra key column to
// scan for one page request, and refuses a two-key sort paired with the direction it was not built
// for (ErrTaskSortDirection).
func taskPageShape(yc page.Request) (string, store.Moc[domain.NhiemVu], string, error) {
	col, dir := yc.Column().SQL, yc.Dir()
	switch col {
	case dueSortAscColumn, prioritySortAscColumn:
		if dir != page.Asc {
			return "", store.Moc[domain.NhiemVu]{}, "", ErrTaskSortDirection
		}
	case dueSortDescColumn, prioritySortDescColumn:
		if dir != page.Desc {
			return "", store.Moc[domain.NhiemVu]{}, "", ErrTaskSortDirection
		}
	}
	switch col {
	case dueSortAscColumn:
		return taskByDueTable, taskAscAnchors, "", nil
	case dueSortDescColumn:
		return taskByDueTable, mocNhiemVu, "", nil
	case prioritySortAscColumn:
		return taskByPriorityTable, taskAscAnchors, col, nil
	case prioritySortDescColumn:
		return taskByPriorityTable, mocNhiemVu, col, nil
	}
	// Every other key is a real NOT NULL column of the plain table, identical in both allowlists —
	// `title` included, which MUST page the plain table: its anchor is looked up by name there.
	return "nhiem_vu", mocNhiemVu, "", nil
}

// scanWithTail scans a row of cotNhiemVu followed by extra trailing columns — the priority key.
type scanWithTail struct {
	r    quangKiem
	tail []any
}

func (s scanWithTail) Scan(dest ...any) error { return s.r.Scan(append(dest, s.tail...)...) }

// DanhSach reads ONE PAGE of the commune's task register.
//
// PAGINATED, UNLIKE THE TWO CATALOGUES NEXT DOOR, and the difference is what the lists ARE: a
// catalogue is closed at a handful of rows, while this register grows with every week the commune
// operates — §12's sample data alone is 28 tasks for one commune in one period.
//
// THE FILTERS ARE BOUND PARAMETERS, numbered from $2 because QueryPage gives $1 to the commune.
func (s *NhiemVuStore) DanhSach(ctx context.Context, loc LocNhiemVu, yc page.Request) (
	page.Result[domain.NhiemVu], error) {

	if err := validateTaskMetric(loc); err != nil {
		return page.NewResult[domain.NhiemVu](), err
	}
	if err := validateTaskListScope(loc); err != nil {
		return page.NewResult[domain.NhiemVu](), err
	}
	// An ascending `due_at` request arrives parsed against SapXepNhiemVu (the list tools/apidoc
	// reads), naming the DESCENDING key; switch it to the ascending one before anything else.
	yc, err := forDueDirection(yc)
	if err != nil {
		return page.NewResult[domain.NhiemVu](), fmt.Errorf("nhiem_vu: dựng lại yêu cầu sắp theo hạn: %w", err)
	}
	table, anchors, rankColumn, err := taskPageShape(yc)
	if err != nil {
		return page.NewResult[domain.NhiemVu](), err
	}
	dieuKien, args := locNhiemVuThanhSQL(loc)
	columns := cotNhiemVu
	if rankColumn != "" {
		// APPENDED AT THE TAIL, like every addition to the positional scan.
		columns += ", " + rankColumn
	}

	kq, err := store.QueryPage(ctx, s.db.For(ctx), store.PageSpec{
		Columns: columns,
		Table:   table,
		// `deleted_at IS NULL` FIRST AND ALWAYS (rule 7, invariant 2). The partial index
		// `nhiem_vu_so` is built on exactly this predicate.
		Filter: `AND deleted_at IS NULL` + dieuKien,
		Args:   args,
	}, yc, anchors, func(rows *sql.Rows) (domain.NhiemVu, string, error) {
		var (
			rank int64
			src  quangKiem = rows
		)
		if rankColumn != "" {
			src = scanWithTail{r: rows, tail: []any{&rank}}
		}
		n, err := quetNhiemVu(src)
		if err != nil {
			return domain.NhiemVu{}, "", err
		}
		n.PriorityRank = rank
		return n, n.ID, nil
	})
	if err != nil {
		return kq, err
	}
	if err := s.ganNguonHop(ctx, kq.Items); err != nil {
		return page.NewResult[domain.NhiemVu](), err
	}
	// `parent` (the register number) and `child_count` for the WHOLE PAGE, two statements at most.
	if err := attachTreeFacts(ctx, scopedTreeQuery(s.db.For(ctx)), kq.Items); err != nil {
		return page.NewResult[domain.NhiemVu](), err
	}
	return kq, nil
}

// CountByStatus counts the commune's live tasks PER STATUS under the SAME filters as DanhSach —
// GET /api/v1/task-counts, the number over each Kanban column.
//
// THE SAME PREDICATE BUILDER AS THE LIST (locNhiemVuThanhSQL), NOT A LOOK-ALIKE. A column header
// counted with one spelling of the filters and filled with another would read "12" over a column of
// eleven cards the day either is edited — the drift the overview figures avoid by sharing
// taskMetricCondition, and this count shares it too, through the same builder.
//
// ONE STATEMENT, so the seven counts describe one instant of the register. A status with no row
// is ABSENT from the map; the handler answers it as 0, because every code of the closed list is a
// column on the board.
func (s *NhiemVuStore) CountByStatus(ctx context.Context, loc LocNhiemVu) (
	map[domain.TrangThaiNhiemVu]int, error) {

	if err := validateTaskMetric(loc); err != nil {
		return nil, err
	}
	if err := validateTaskListScope(loc); err != nil {
		return nil, err
	}
	dieuKien, args := locNhiemVuThanhSQL(loc)

	rows, err := s.db.For(ctx).Query(ctx, "trang_thai, count(*)", "nhiem_vu",
		`AND deleted_at IS NULL`+dieuKien+` GROUP BY trang_thai`, args...)
	if err != nil {
		return nil, fmt.Errorf("nhiem_vu: đếm theo trạng thái: %w", err)
	}
	defer rows.Close()

	counts := make(map[domain.TrangThaiNhiemVu]int, 7)
	for rows.Next() {
		var (
			status string
			n      int64
		)
		if err := rows.Scan(&status, &n); err != nil {
			return nil, fmt.Errorf("nhiem_vu: đếm theo trạng thái: đọc dòng: %w", err)
		}
		counts[domain.TrangThaiNhiemVu(status)] = int(n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("nhiem_vu: đếm theo trạng thái: %w", err)
	}
	return counts, nil
}

// cauNguonHop resolves, for a batch of conclusion ids, the meeting and ordinal each belongs to — the
// drawer's back-link. ONE statement for a whole page (never one per task).
//
//	JOIN (inner)            a conclusion whose meeting is soft-deleted, or that is soft-deleted itself,
//	                        yields NO row, so the task simply carries no link (rule 7, invariant 2).
//	                        Omitting the fields is the answer, not an error: the task is still live.
//	b.tenant_id = $1        store.Scoped.QueryJoin's contract — the joined table is bound to the
//	                        commune too, or a colliding id would name another commune's meeting.
//
// Same formatting constraint as cauKetLuanKemDem: ` FROM ` stays on the SELECT line (fake driver).
const cauNguonHop = `SELECT k.id, b.id, b.ten_cuoc_hop, k.thu_tu FROM ket_luan_hop k
	JOIN bien_ban_hop b ON b.tenant_id = $1 AND b.id = k.bien_ban_id AND b.deleted_at IS NULL
	WHERE k.tenant_id = $1 AND k.deleted_at IS NULL AND k.id IN (`

// ganNguonHop fills NhiemVu.NguonHop for every `ket-luan-hop` task in ds, in place.
//
// NO STATEMENT AT ALL when no task on the page came from a conclusion — the `IN (…)` list would be
// empty, which is not valid SQL, and most pages of most communes take this branch.
//
// THE IDS ARE PLACEHOLDERS. They come from rows just read, and still go in bound: "the values came
// from our own table" is the argument that lets a concatenated list survive review.
func (s *NhiemVuStore) ganNguonHop(ctx context.Context, ds []domain.NhiemVu) error {
	var (
		ids  []string
		thay = map[string]bool{}
	)
	for _, n := range ds {
		if n.NguonGiao != domain.NguonKetLuanHop || n.NguonID == "" || thay[n.NguonID] {
			continue
		}
		thay[n.NguonID] = true
		ids = append(ids, n.NguonID)
	}
	if len(ids) == 0 {
		return nil
	}

	var b strings.Builder
	b.WriteString(cauNguonHop)
	args := make([]any, 0, len(ids))
	for i, id := range ids {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("$" + strconv.Itoa(i+2)) // $1 is the commune
		args = append(args, id)
	}
	b.WriteString(")")

	rows, err := s.db.For(ctx).QueryJoin(ctx, b.String(), args...)
	if err != nil {
		return fmt.Errorf("nhiem_vu: đọc liên kết ngược về biên bản họp: %w", err)
	}
	defer rows.Close()

	theo := make(map[string]domain.LienKetKetLuanHop, len(ids))
	for rows.Next() {
		var (
			ketLuanID string
			ng        domain.LienKetKetLuanHop
		)
		if err := rows.Scan(&ketLuanID, &ng.BienBanID, &ng.TenCuocHop, &ng.ThuTu); err != nil {
			return fmt.Errorf("nhiem_vu: đọc dòng liên kết ngược: %w", err)
		}
		theo[ketLuanID] = ng
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("nhiem_vu: duyệt liên kết ngược: %w", err)
	}

	for i := range ds {
		if ds[i].NguonGiao != domain.NguonKetLuanHop {
			continue
		}
		if ng, ok := theo[ds[i].NguonID]; ok {
			ds[i].NguonHop = &ng
		}
	}
	return nil
}

// TheoMa reads one task by the number the commune issued it.
//
// IT TAKES THE ISSUED NUMBER AND NOT THE INTERNAL id, because that is what the URL carries, what
// the Sổ theo dõi prints and what an audit entry's subject will be. One identifier end to end is
// one identifier nobody can mix up.
//
// `deleted_at IS NULL` IS ON THIS PATH AS IT IS ON EVERY OTHER (rule 7, invariant 2).
func (s *NhiemVuStore) TheoMa(ctx context.Context, ma string) (domain.NhiemVu, error) {
	rows, err := s.db.For(ctx).Query(ctx, cotNhiemVu, "nhiem_vu",
		`AND ma = $2 AND deleted_at IS NULL`, ma)
	if err != nil {
		return domain.NhiemVu{}, fmt.Errorf("nhiem_vu: đọc theo mã: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.NhiemVu{}, fmt.Errorf("nhiem_vu: đọc theo mã: %w", err)
		}
		return domain.NhiemVu{}, ErrNhiemVuKhongTonTai
	}
	n, err := quetNhiemVu(rows)
	if err != nil {
		return domain.NhiemVu{}, err
	}
	if err := rows.Err(); err != nil {
		return domain.NhiemVu{}, fmt.Errorf("nhiem_vu: duyệt kết quả: %w", err)
	}
	// Closed before the second statement: one connection, no nested cursor held open.
	rows.Close()

	mot := []domain.NhiemVu{n}
	if err := s.ganNguonHop(ctx, mot); err != nil {
		return domain.NhiemVu{}, err
	}
	if err := attachTreeFacts(ctx, scopedTreeQuery(s.db.For(ctx)), mot); err != nil {
		return domain.NhiemVu{}, err
	}
	return mot[0], nil
}

// LiveByCode is TheoMa's row and nothing else — no meeting back-link, no tree facts — for a caller
// that needs only the task's identity and holders (the attachment acts, app/task_attachment.go). One
// statement where TheoMa runs three. The same live filter; the same ErrNhiemVuKhongTonTai.
func (s *NhiemVuStore) LiveByCode(ctx context.Context, code string) (domain.NhiemVu, error) {
	rows, err := s.db.For(ctx).Query(ctx, cotNhiemVu, "nhiem_vu",
		`AND ma = $2 AND deleted_at IS NULL`, code)
	if err != nil {
		return domain.NhiemVu{}, fmt.Errorf("nhiem_vu: đọc theo mã: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.NhiemVu{}, fmt.Errorf("nhiem_vu: đọc theo mã: %w", err)
		}
		return domain.NhiemVu{}, ErrNhiemVuKhongTonTai
	}
	n, err := quetNhiemVu(rows)
	if err != nil {
		return domain.NhiemVu{}, err
	}
	if err := rows.Err(); err != nil {
		return domain.NhiemVu{}, fmt.Errorf("nhiem_vu: duyệt kết quả: %w", err)
	}
	return n, nil
}

// quetNhiemVu reads one row of cotNhiemVu.
//
// POSITIONAL, IN LOCKSTEP WITH cotNhiemVu. database/sql binds by POSITION, so a destination inserted
// or removed anywhere but the tail silently shifts every column after it — and the columns it would
// shift are the three adjacent TIMESTAMPTZs whose confusion produces a wrong figure rather than an
// error. quetNhiemVuKhopCot in the test package asserts the two lists line up BY NAME.
func quetNhiemVu(r quangKiem) (domain.NhiemVu, error) {
	var (
		n   domain.NhiemVu
		tt  string
		ng  string
		loa string

		// EVERY NULLABLE COLUMN IS READ THROUGH AN EXPLICIT NULL TYPE. Scanning a NULL straight into
		// a string or a time.Time is a runtime error in some drivers and a zero value in others, and
		// the second is how "no deadline was ever set" quietly becomes a deadline in year 1.
		khoi, mucUuTien, moTa, nguonID    sql.NullString
		boPhan, nguoiThucHien, lanhDao    sql.NullString
		tomTat, ghiChu                    sql.NullString
		hanXuLy, hanBanDau, ngayHoanThanh sql.NullTime

		// `nhiem_vu_cha_id` — migration 0008. NULL is a ROOT TASK and is the ordinary case, so it is
		// read through a NULL type like every other nullable column here.
		nhiemVuCha sql.NullString
	)

	dich := []any{
		&n.ID, &n.Ma, &loa, &khoi, &n.TieuDe, &moTa, &tt, &mucUuTien,
		&ng, &nguonID,
		&boPhan, &nguoiThucHien, &lanhDao,
		&hanXuLy, &hanBanDau, &ngayHoanThanh,
		&n.TienDo, &tomTat, &ghiChu,
		&n.LanhDaoPheDuyetHoanThanh, &n.CapTrenCongNhanHoanThanh,
		// APPENDED AT THE TAIL, NEVER INSERTED. database/sql binds by POSITION, and a destination
		// inserted anywhere but the end silently shifts every column after it — the columns it would
		// shift here are the three adjacent TIMESTAMPTZs whose confusion produces a wrong figure
		// rather than an error.
		&n.NguoiTaoMa, &n.TaoLuc, &nhiemVuCha,
		// `cap_nhat_luc` — NOT NULL DEFAULT now() (migration 0006), the PATCH precondition token.
		// Appended at the tail for the reason above.
		&n.UpdatedAt,
	}
	if err := r.Scan(dich...); err != nil {
		return domain.NhiemVu{}, fmt.Errorf("nhiem_vu: đọc dòng: %w", err)
	}

	n.Loai = loa
	n.TrangThai = domain.TrangThaiNhiemVu(tt)
	n.NguonGiao = domain.NguonGiao(ng)
	n.Khoi = khoi.String
	n.MucUuTien = mucUuTien.String
	n.MoTa = moTa.String
	n.NguonID = nguonID.String
	n.BoPhanID = boPhan.String
	n.NguoiThucHienMa = nguoiThucHien.String
	n.LanhDaoGiaoViecMa = lanhDao.String
	n.TomTatKetQua = tomTat.String
	n.GhiChu = ghiChu.String
	n.NhiemVuChaID = nhiemVuCha.String
	// NULL -> the zero time.Time. All three zeros mean ONE thing here — "this instant was never
	// recorded" — which is the opposite of the petition register, where two adjacent NULLs mean two
	// opposite things. Say it out loud precisely because the two files sit side by side.
	n.HanXuLy = hanXuLy.Time
	n.HanBanDau = hanBanDau.Time
	n.NgayHoanThanh = ngayHoanThanh.Time
	return n, nil
}
