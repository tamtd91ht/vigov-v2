package store

// The SỔ VĂN BẢN ĐẾN register — SQL, and nothing else.
//
// FIVE THINGS HOLD ACROSS EVERY METHOD BELOW, and each one is a defect class rather than a style:
//
//  1. THE COMMUNE IS $1 IN EVERY STATEMENT, taken from tx.TenantID() / the context (rule 1,
//     invariants 4 and 5). It is never a parameter of any method here, so a caller cannot reach
//     another commune's register even by mistake.
//  2. NOTHING HERE OPENS A TRANSACTION. The caller opens it and writes the audit entry inside it
//     (rule 6, invariant 3) — internal/app is the only layer that may. That is why the advisory
//     audit hook has nothing to find in this file and should not: an audit entry written HERE would
//     be an entry that knows the row changed but not what a person DID.
//  3. EVERY READ EXCLUDES SOFT-DELETED ROWS (rule 7, invariant 2). There is no method here that can
//     see one — and that is exactly why the NUMBER of a removed document is still taken: the unique
//     key counts the row this package can no longer read.
//  4. `so_vao_so` AND `nam` APPEAR IN NO UPDATE. They are the issued number; the trigger
//     `so_van_ban_bat_bien` refuses to change them underneath, and their absence here is what makes
//     that floor unreachable from this service in the first place.
//  5. THERE IS NO HARD DELETE, and `ho_so_luu_tru_cam_xoa_cung` refuses one even if somebody writes
//     it (rule 7, forbidden #1).

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-documents/internal/domain"
)

// IncomingDocumentStore reads and writes one commune's incoming-document register.
type IncomingDocumentStore struct {
	db *store.DB
}

func NewIncomingDocumentStore(db *store.DB) *IncomingDocumentStore {
	return &IncomingDocumentStore{db: db}
}

var (
	// ErrIncomingDocumentNotFound is "no such live entry IN THIS COMMUNE", and the two halves are one
	// answer on purpose. An entry of another commune is indistinguishable from one that does not
	// exist, because the query cannot reach it at all; the caller answers 404 either way and so
	// leaks nothing about what another authority holds.
	ErrIncomingDocumentNotFound = errors.New("van_ban_den: không có văn bản này trong xã")

	// ErrDocumentTypeNotInUse — the document type code names no live row of this commune's
	// catalogue.
	//
	// WHY A DOCUMENT MUST NAME A LIVE TYPE: the type is what the register is filtered and reported
	// by, and it is what the type column of every list renders. A code matching nothing is a
	// document that disappears from every filtered view while sitting in the register looking
	// healthy. There is no foreign key (the code is stored as a VALUE — see migration 0004), so
	// this check is the only one there is.
	ErrDocumentTypeNotInUse = errors.New("van_ban_den: loại văn bản không có trong danh mục của xã")
)

// IncomingDocumentSorts is the closed set of sorts GET /api/v1/incoming-documents offers. Declared
// here, next to the SQL, because it names real columns — only store/ knows column names.
//
// `number` IS THE DEFAULT, DESCENDING, which is what §3.1 shows ("7, 6, 5… giảm dần"). It is the
// value a clerk quotes on the telephone, it is unique within (commune, year) and it is NOT NULL —
// the three things page.QueryPage needs from a sort column.
//
// WHAT IS DELIBERATELY ABSENT: `co_quan_ban_hanh` and `trich_yeu`. A sort key travels in a URL, an
// access log and a browser history, and a summary of a document is free text somebody typed about a
// government matter. `han_xu_ly_xong` is absent for a different reason — it is the column an
// overdue report groups by, and offering it as a page sort invites a client to page the whole
// register looking for late documents instead of asking for them.
var IncomingDocumentSorts = page.NewAllowlist(page.Desc,
	page.Col("number", "so_vao_so", page.KindInt),
	page.Col("created_at", "tao_luc", page.KindTime),
)

// IncomingDocumentFilter is the set of filters the list route accepts, already validated by the
// handler.
//
// A STRUCT AND NOT A STRING OF SQL: every field below becomes a BOUND PARAMETER. A filter assembled
// as text anywhere above this line is an injection point in a government register.
type IncomingDocumentFilter struct {
	Year         int    // 0 = every year
	Status       string // "" = every state
	DocumentType string // "" = every type
	OrgUnitID    string // "" = every department, including none
	Search       string // "" = no text search

	// Metric narrows the list to exactly one dashboard figure's rows (zero Metric = no such filter).
	// It is built by metricPredicate — the SAME function CountIncomingSummary counts with — so the
	// drill-down behind a figure has as many rows as the figure says.
	Metric IncomingMetricFilter
}

// incomingDocumentColumns IS READ BY POSITION in the scans below, and the list has one group that
// swaps with no error whatsoever:
//
//	ngay_den, ngay_van_ban    two DATE columns. Swapped, a document signed in January and received
//	                          in March reads as received in January — which is the pair
//	                          domain.ValidateIncomingDocumentDate compares, so the validation would
//	                          still "work" and would be about the wrong two dates.
//
// That is why this is a constant rather than a column list written inline at each call site.
//
// COALESCE ON EVERY NULLABLE TEXT COLUMN, so the domain struct carries "" rather than needing a
// sql.NullString per field. `ngay_van_ban` cannot be COALESCEd to anything honest — there is no
// "zero" date that is not also a real one — so it is scanned through sql.NullTime.
const incomingDocumentColumns = `id, so_vao_so, nam, ngay_den, COALESCE(so_ky_hieu, ''), ngay_van_ban, ` +
	`co_quan_ban_hanh, loai_van_ban, trich_yeu, COALESCE(do_khan, ''), ` +
	`COALESCE(bo_phan_dang_giu_id, ''), COALESCE(can_bo_xu_ly_ma, ''), ` +
	`han_xu_ly_xong, trang_thai, nguoi_tao_ma, tao_luc, cap_nhat_luc`

// scanIncomingDocument is the shared Scan of one row. Positional, in lockstep with
// incomingDocumentColumns — see the note there on the two adjacent DATE columns.
func scanIncomingDocument(scan func(...any) error) (domain.IncomingDocument, error) {
	var (
		d            domain.IncomingDocument
		documentDate sql.NullTime
		urgency      string
		status       string
	)
	err := scan(&d.ID, &d.ArrivalNo, &d.Year, &d.ReceivedDate, &d.ReferenceNo, &documentDate,
		&d.IssuingBody, &d.DocumentType, &d.Summary, &urgency,
		&d.HoldingOrgUnitID, &d.AssigneeCode,
		&d.DueAt, &status, &d.CreatedBy, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return domain.IncomingDocument{}, err
	}
	if documentDate.Valid {
		d.DocumentDate = documentDate.Time
	}
	d.Urgency = domain.Urgency(urgency)
	d.Status = domain.IncomingDocumentStatus(status)
	return d, nil
}

// List reads ONE PAGE of the commune's incoming register.
//
// PAGINATED, UNLIKE THE DOCUMENT-TYPE CATALOGUE, and the difference is what the two lists ARE: a
// catalogue is a closed reference list whose size depends on how the commune classifies its
// paperwork, while this register grows with every week the commune operates. An unpaginated read of
// it is a query whose cost rises for ever (skills/rest-api-design §5, forbidden #4).
//
// THE FILTERS ARE BOUND PARAMETERS, numbered from $2 because QueryPage gives $1 to the commune.
// Their ORDER here must match the order they are appended to `args` below; they are built together,
// in one loop, so the two cannot drift.
func (s *IncomingDocumentStore) List(ctx context.Context, filter IncomingDocumentFilter,
	req page.Request) (page.Result[domain.IncomingDocument], error) {

	where, args, err := incomingFilterSQL(filter)
	if err != nil {
		return page.NewResult[domain.IncomingDocument](), err
	}

	return store.QueryPage(ctx, s.db.For(ctx), store.PageSpec{
		Columns: incomingDocumentColumns,
		Table:   "van_ban_den",
		// `deleted_at IS NULL` FIRST AND ALWAYS (rule 7, invariant 2). The partial index
		// `van_ban_den_so_theo_nam` is built on exactly this predicate.
		Filter: `AND deleted_at IS NULL` + where,
		Args:   args,
	}, req, incomingDocumentCursor, func(rows *sql.Rows) (domain.IncomingDocument, string, error) {
		d, err := scanIncomingDocument(rows.Scan)
		if err != nil {
			return domain.IncomingDocument{}, "", err
		}
		return d, d.ID, nil
	})
}

// incomingFilterSQL turns the validated filter struct into a predicate and its bound values.
//
// ONE FUNCTION BUILDING BOTH HALVES, because the placeholder numbers and the argument order are one
// fact: written apart, a filter added to one half and forgotten in the other produces a query that
// silently reads the wrong column's value.
func incomingFilterSQL(filter IncomingDocumentFilter) (string, []any, error) {
	var (
		where string
		args  []any
	)
	add := func(format string, value any) {
		args = append(args, value)
		where += fmt.Sprintf(format, len(args)+1) // $1 is the commune
	}
	if filter.Year != 0 {
		add(" AND nam = $%d", filter.Year)
	}
	if filter.Status != "" {
		add(" AND trang_thai = $%d", filter.Status)
	}
	if filter.DocumentType != "" {
		add(" AND loai_van_ban = $%d", filter.DocumentType)
	}
	if filter.OrgUnitID != "" {
		add(" AND bo_phan_dang_giu_id = $%d", filter.OrgUnitID)
	}
	if filter.Search != "" {
		// ILIKE ON TWO COLUMNS AND NO MORE. `trich_yeu` is what a clerk remembers and `so_ky_hieu`
		// is what they were read over the telephone. A leading wildcard cannot use an index, which
		// is affordable HERE and only here: the scan is already bounded to one commune and — in the
		// ordinary case — one year, which is a few thousand rows.
		args = append(args, "%"+filter.Search+"%")
		where += fmt.Sprintf(" AND (trich_yeu ILIKE $%d OR COALESCE(so_ky_hieu,'') ILIKE $%d)",
			len(args)+1, len(args)+1)
	}
	if filter.Metric.Metric != "" {
		// The same binder numbering as `add` above ($1 is the commune), and the same predicate the
		// figure is counted with.
		pred, err := metricPredicate(filter.Metric, newBinder(&args))
		if err != nil {
			return "", nil, err
		}
		where += " AND (" + pred + ")"
	}
	return where, args, nil
}

// incomingDocumentCursor BINDS each allowlisted sort column to the way that column's cursor value is
// read out of a scanned row. store.NewMoc compares the two lists AT CONSTRUCTION, so a sort added to
// IncomingDocumentSorts without a reader here is a panic at startup rather than an error on the
// first request that uses it — which would be after release.
var incomingDocumentCursor = store.NewMoc[domain.IncomingDocument](IncomingDocumentSorts,
	map[string]func(domain.IncomingDocument) page.Key{
		"number":     func(d domain.IncomingDocument) page.Key { return page.IntKey(int64(d.ArrivalNo)) },
		"created_at": func(d domain.IncomingDocument) page.Key { return page.TimeKey(d.CreatedAt) },
	})

// --- the detail drawer ------------------------------------------------------------------------

// MaxRoutingHistory is the hard upper bound on one document's routing timeline.
//
// A CEILING AND NOT PAGINATION, for the reason MaxDocumentTypes gives: the drawer needs the
// WHOLE timeline to be correct at all — a history missing its first page is a history that says
// the document started somewhere it did not — so the bound cannot be a `limit` parameter. One
// process serves every commune, so an unbounded read is still forbidden (skills/rest-api-design
// §5, forbidden #4).
//
// 500 IS NOT AN ESTIMATE. A document is routed a handful of times; five hundred routings of ONE
// document is a loop or an import gone wrong, and past that point the rows are not a timeline.
const MaxRoutingHistory = 500

// ErrTooMuchRoutingHistory says the ceiling was reached. The caller answers 500 and refuses rather
// than returning a truncated history — a short timeline reads as a complete one, and it is the
// record of who was made responsible for a document.
var ErrTooMuchRoutingHistory = errors.New("van_ban_den: lịch sử chuyển vượt trần")

// ByID reads one live entry for display.
//
// NOT `FOR UPDATE` — nothing is decided on it, so locking would only queue a routing behind a
// clerk opening a drawer. `deleted_at IS NULL` IS in the predicate (rule 7, invariant 2), and the
// commune is $1: a removed entry, another commune's entry and an id that never existed are one
// answer, ErrIncomingDocumentNotFound, because the query cannot tell them apart.
func (s *IncomingDocumentStore) ByID(ctx context.Context, tx *store.ScopedTx,
	id string) (domain.IncomingDocument, error) {

	const stmt = `SELECT ` + incomingDocumentColumns + ` FROM van_ban_den ` +
		`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

	d, err := scanIncomingDocument(
		tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), id).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.IncomingDocument{}, ErrIncomingDocumentNotFound
	}
	if err != nil {
		return domain.IncomingDocument{}, fmt.Errorf("van_ban_den: đọc văn bản: %w", err)
	}
	return d, nil
}

// routingColumns IS READ BY POSITION in RoutingHistory. `tu_bo_phan_id` and `den_bo_phan_id` are
// adjacent TEXT columns: swapped, the timeline says the file travelled backwards and nothing
// errors.
const routingColumns = `id, van_ban_den_id, thoi_diem, nguoi_ma, trang_thai_tai_thoi_diem, ` +
	`COALESCE(tu_bo_phan_id, ''), den_bo_phan_id, COALESCE(can_bo_xu_ly_ma, ''), noi_dung, tao_luc`

// RoutingHistory reads one document's routing timeline, OLDEST FIRST.
//
// IT DOES NOT CHECK THE DOCUMENT. The caller reads it with ByID in the same transaction first;
// this table has no soft-delete columns and no foreign key, so on its own it would hand back the
// timeline of a removed document.
//
// ORDER BY thoi_diem, then `id`: two routings in the same instant must not swap places between two
// reads of an append-only history. The index `lich_su_chuyen_theo_van_ban` is on
// (tenant_id, van_ban_den_id, thoi_diem DESC); PostgreSQL scans it backwards for ASC.
func (s *IncomingDocumentStore) RoutingHistory(ctx context.Context, tx *store.ScopedTx,
	incomingDocumentID string) ([]domain.DocumentRouting, error) {

	// LIMIT is the ceiling PLUS ONE, so "too many" is detectable rather than a silent cut.
	const stmt = `SELECT ` + routingColumns + ` FROM lich_su_chuyen_van_ban ` +
		`WHERE tenant_id = $1 AND van_ban_den_id = $2 ORDER BY thoi_diem ASC, id ASC LIMIT $3`

	rows, err := tx.Underlying().QueryContext(ctx, stmt, string(tx.TenantID()), incomingDocumentID,
		MaxRoutingHistory+1)
	if err != nil {
		return nil, fmt.Errorf("van_ban_den: đọc lịch sử chuyển: %w", err)
	}
	defer rows.Close()

	out := make([]domain.DocumentRouting, 0, 8)
	for rows.Next() {
		var (
			r      domain.DocumentRouting
			status string
		)
		if err := rows.Scan(&r.ID, &r.IncomingDocumentID, &r.RoutedAt, &r.RoutedBy, &status,
			&r.FromOrgUnitID, &r.ToOrgUnitID, &r.AssigneeCode, &r.Instruction, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("van_ban_den: đọc dòng lịch sử chuyển: %w", err)
		}
		r.StatusAtTime = domain.IncomingDocumentStatus(status)
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("van_ban_den: duyệt lịch sử chuyển: %w", err)
	}
	if len(out) > MaxRoutingHistory {
		return nil, ErrTooMuchRoutingHistory
	}
	return out, nil
}

// --- the write path ---------------------------------------------------------------------------

// ByIDForUpdate reads one live entry inside the transaction and holds it until the transaction ends.
//
// `FOR UPDATE` IS THE POINT OF THIS METHOD AND NOT AN OPTIMISATION. Every write below is a
// read-decide-write: read the entry, work out whether the register admits the operation, refuse or
// apply. Without the lock two clerks acting on the same document both read the old state and both
// decide against it — and the pair that actually costs something is routing racing routing, where
// the second write leaves the document in one department while the timeline says another.
//
// IT ALSO EXCLUDES SOFT-DELETED ROWS, so "edit a document somebody removed a second ago" is a 404
// rather than a resurrection.
func (s *IncomingDocumentStore) ByIDForUpdate(ctx context.Context, tx *store.ScopedTx,
	id string) (domain.IncomingDocument, error) {

	const stmt = `SELECT ` + incomingDocumentColumns + ` FROM van_ban_den ` +
		`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR UPDATE`

	d, err := scanIncomingDocument(
		tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), id).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.IncomingDocument{}, ErrIncomingDocumentNotFound
	}
	if err != nil {
		return domain.IncomingDocument{}, fmt.Errorf("van_ban_den: đọc văn bản để sửa: %w", err)
	}
	return d, nil
}

// DocumentTypeInUse reports whether this catalogue code names a live, in-use type of this commune.
//
// `dang_dung` IS PART OF THE PREDICATE and `deleted_at IS NULL` is too, and the two are different
// refusals with one answer: a type the commune has retired must not be chosen for a NEW document,
// while a document already registered under it keeps rendering its label (which is why the code is
// stored as a value and this check is only on the write path).
//
// NOT `FOR UPDATE`: this reads a fact about a row nothing here writes. Locking the catalogue row
// for every booking would serialise the whole register behind one reference row.
func (s *IncomingDocumentStore) DocumentTypeInUse(ctx context.Context, tx *store.ScopedTx, code string) error {
	const stmt = `SELECT 1 FROM loai_van_ban
		WHERE tenant_id = $1 AND ma = $2 AND dang_dung AND deleted_at IS NULL`

	var one int
	err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), code).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrDocumentTypeNotInUse
	}
	if err != nil {
		return fmt.Errorf("van_ban_den: kiểm loại văn bản: %w", err)
	}
	return nil
}

// insertIncomingDocument — `trang_thai` IS A LITERAL AND NOT A PARAMETER.
//
// Read that as the property it is, not as a shortcut. A newly booked document is `Mới vào sổ`,
// always: there is no $n for the state, so there is no value any layer above could pass and no
// field a client could fill. Turning it into a parameter is the one edit that would let a client
// create a document already `Đã giải quyết` — a record closed by nobody, counted as done in every
// figure the commune reports.
//
// `so_vao_so` IS A PARAMETER AND MUST BE: it comes from NumberSeriesStore.IssueNumber, in this same
// transaction, under the counter's row lock. There is no other caller and no other source.
const insertIncomingDocument = `INSERT INTO van_ban_den
	(tenant_id, id, so_vao_so, nam, ngay_den, so_ky_hieu, ngay_van_ban, co_quan_ban_hanh,
	 loai_van_ban, trich_yeu, do_khan, bo_phan_dang_giu_id, can_bo_xu_ly_ma,
	 han_xu_ly_xong, trang_thai, nguoi_tao_ma)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, 'moi-vao-so', $15)`

// Insert books one document, in the first state of the register.
func (s *IncomingDocumentStore) Insert(ctx context.Context, tx *store.ScopedTx, d domain.IncomingDocument) error {
	_, err := tx.Exec(ctx, insertIncomingDocument, string(tx.TenantID()),
		d.ID, d.ArrivalNo, d.Year, d.ReceivedDate, emptyToNil(d.ReferenceNo), dateOrNil(d.DocumentDate),
		d.IssuingBody, d.DocumentType, d.Summary, emptyToNil(string(d.Urgency)),
		emptyToNil(d.HoldingOrgUnitID), emptyToNil(d.AssigneeCode),
		d.DueAt, d.CreatedBy)
	if err != nil {
		return fmt.Errorf("van_ban_den: chèn: %w", err)
	}
	return nil
}

// updateIncomingDocument — `so_vao_so`, `nam`, `trang_thai`, `han_xu_ly_xong`, `bo_phan_dang_giu_id`
// and `can_bo_xu_ly_ma` APPEAR NOWHERE IN THIS STATEMENT, and each absence is a different rule:
//
//	so_vao_so, nam        an issued number is never renumbered (rule 7, invariant 3). The trigger
//	                      refuses it too; their absence here is what keeps the trigger unreachable.
//	trang_thai            the state moves by ROUTING and by nothing else on this surface, so an edit
//	                      and a state change can never be one event — the same split
//	                      service-finance's voucher register makes, for the same reason: a record of
//	                      what changed is only readable if the two changes are two events.
//	han_xu_ly_xong        the commitment was fixed at the act that set it (rule 10, invariant 2).
//	                      An UPDATE here silently moves a deadline the authority already committed
//	                      to, and nothing on any screen would say so.
//	bo_phan, can_bo       who is holding the file is the routing history's business, and it is
//	                      written by RouteToOrgUnit below together with the timeline entry.
const updateIncomingDocument = `UPDATE van_ban_den
	SET ngay_den = $3, so_ky_hieu = $4, ngay_van_ban = $5, co_quan_ban_hanh = $6,
	    loai_van_ban = $7, trich_yeu = $8, do_khan = $9, cap_nhat_luc = now()
	WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

// Update writes the seven fields a clerk may correct. The caller has already read the row with
// ByIDForUpdate.
func (s *IncomingDocumentStore) Update(ctx context.Context, tx *store.ScopedTx, d domain.IncomingDocument) error {
	res, err := tx.Exec(ctx, updateIncomingDocument, string(tx.TenantID()),
		d.ID, d.ReceivedDate, emptyToNil(d.ReferenceNo), dateOrNil(d.DocumentDate), d.IssuingBody,
		d.DocumentType, d.Summary, emptyToNil(string(d.Urgency)))
	if err != nil {
		return fmt.Errorf("van_ban_den: cập nhật: %w", err)
	}
	return requireOneIncomingRow(res, "cập nhật")
}

// routeIncomingDocument moves the file to another department and records the state in ONE statement.
//
// ONE STATEMENT AND NOT TWO: `trang_thai` and `bo_phan_dang_giu_id` are what the register's own
// screen shows side by side, and a window in which the document has moved but the state has not is
// a window where the "Đang giữ" column and the status chip disagree. The state VALUE comes from the
// caller — domain.StatusAfterRouting — rather than being a literal here, because the rule (a
// routing only ever moves the state forward) is reasoned about in the domain, and a literal here
// would be a second, silent copy of that reasoning in SQL.
const routeIncomingDocument = `UPDATE van_ban_den
	SET bo_phan_dang_giu_id = $3, can_bo_xu_ly_ma = $4, trang_thai = $5, cap_nhat_luc = now()
	WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

func (s *IncomingDocumentStore) RouteToOrgUnit(ctx context.Context, tx *store.ScopedTx,
	id, toOrgUnitID, assigneeCode string, status domain.IncomingDocumentStatus) error {

	res, err := tx.Exec(ctx, routeIncomingDocument, string(tx.TenantID()),
		id, toOrgUnitID, emptyToNil(assigneeCode), string(status))
	if err != nil {
		return fmt.Errorf("van_ban_den: chuyển bộ phận: %w", err)
	}
	return requireOneIncomingRow(res, "chuyển bộ phận")
}

// insertDocumentRouting writes one line of the timeline.
//
// IT IS INSERTED IN THE SAME TRANSACTION as the UPDATE above, and that is not a convenience: the
// document's current department and the history of how it got there are one fact recorded twice,
// and a window where only one of them exists is a register that cannot say who sent the file where.
// The table is append-only underneath (rule 7, forbidden #5), so there is no repair afterwards.
const insertDocumentRouting = `INSERT INTO lich_su_chuyen_van_ban
	(tenant_id, id, van_ban_den_id, thoi_diem, nguoi_ma, trang_thai_tai_thoi_diem,
	 tu_bo_phan_id, den_bo_phan_id, can_bo_xu_ly_ma, noi_dung)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

func (s *IncomingDocumentStore) InsertRouting(ctx context.Context, tx *store.ScopedTx,
	r domain.DocumentRouting) error {

	_, err := tx.Exec(ctx, insertDocumentRouting, string(tx.TenantID()),
		r.ID, r.IncomingDocumentID, r.RoutedAt, r.RoutedBy, string(r.StatusAtTime),
		emptyToNil(r.FromOrgUnitID), r.ToOrgUnitID, emptyToNil(r.AssigneeCode), r.Instruction)
	if err != nil {
		return fmt.Errorf("van_ban_den: ghi lịch sử chuyển: %w", err)
	}
	return nil
}

// softDeleteIncomingDocument writes all THREE columns rule 7, invariant 1 names — `deleted_at`,
// `deleted_by` and `delete_reason` — in one statement, so none of them can be forgotten. The CHECK
// constraint `van_ban_den_xoa_mem_du_cot` refuses two out of three underneath.
//
// `AND deleted_at IS NULL` IS WHAT MAKES A SECOND REMOVAL A 404 rather than a silent rewrite of who
// removed the document and why. The first removal is the one that happened; overwriting its reason
// would be editing a historical record (rule 7, forbidden #5).
//
// THE NUMBER IS NOT RETURNED TO THE SERIES. Nothing here touches `day_so_van_ban`, and the unique
// key still counts this row — so the next document takes the NEXT number and the removed one leaves
// a visible gap, which is exactly what an archival register is supposed to show.
const softDeleteIncomingDocument = `UPDATE van_ban_den
	SET deleted_at = now(), deleted_by = $3, delete_reason = $4, cap_nhat_luc = now()
	WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

func (s *IncomingDocumentStore) SoftDelete(ctx context.Context, tx *store.ScopedTx, id, by, reason string) error {
	res, err := tx.Exec(ctx, softDeleteIncomingDocument, string(tx.TenantID()), id, by, reason)
	if err != nil {
		return fmt.Errorf("van_ban_den: xoá mềm: %w", err)
	}
	return requireOneIncomingRow(res, "xoá mềm")
}

// --- shared between the two registers ------------------------------------------------------------

// requireOneIncomingRow turns "nothing was updated" into ErrIncomingDocumentNotFound.
//
// WHY IT IS CHECKED AT ALL WHEN THE CALLER ALREADY READ THE ROW: the read and the write are two
// statements, and a method used without the read — a future caller, a retry path — would otherwise
// report success for a row that does not exist. An UPDATE touching zero rows is not an error to
// PostgreSQL; it is only an error to us.
func requireOneIncomingRow(res sql.Result, op string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("van_ban: %s: đọc số dòng: %w", op, err)
	}
	if n == 0 {
		return ErrIncomingDocumentNotFound
	}
	return nil
}

// emptyToNil writes an empty optional TEXT column as NULL rather than as ”.
//
// WHY IT MATTERS ON THESE TABLES SPECIFICALLY: `so_ky_hieu`, `do_khan`, `bo_phan_dang_giu_id` and
// `can_bo_xu_ly_ma` are all optional and all read back through COALESCE. Storing ” as well would
// give one absent value two spellings in one column, so `WHERE bo_phan_dang_giu_id IS NULL` — the
// obvious way anybody would later look for documents nobody is holding — would silently miss half
// of them.
func emptyToNil(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// dateOrNil is the same for an optional DATE. The zero time.Time is not a date the register can
// mean; written as-is it would reach the column as the year 1 and sort before every real document.
func dateOrNil(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}
