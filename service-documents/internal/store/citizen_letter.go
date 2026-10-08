package store

// SỔ ĐƠN THƯ CÔNG DÂN — SQL for `citizen_letter` and `citizen_letter_log` (migration 0006), and
// nothing else. The same five properties as van_ban_den.go's header hold here:
//
//  1. THE COMMUNE IS $1 IN EVERY STATEMENT, from the context / tx.TenantID() (rule 1, invariants 4–5).
//  2. NOTHING HERE OPENS A TRANSACTION. internal/app opens it and writes the audit entry inside it.
//  3. EVERY READ EXCLUDES SOFT-DELETED LETTERS (rule 7, invariant 2) — lists, the drawer, the duplicate
//     warning and the report alike. The log has no soft-delete columns; it is only read after its
//     letter was found visible in the same transaction.
//  4. `number` AND `year` APPEAR IN NO UPDATE (rule 7, invariant 3); the trigger
//     `citizen_letter_number_immutable` refuses them underneath.
//  5. THERE IS NO DELETE of either table, and the triggers refuse one (rule 7, forbidden #1 / #5).
//
// PERSONAL DATA (rule 3): `sender_*`, `summary`, `result_summary` and the log's `content` pass
// through this file. No error message built here quotes a value; they name the operation only.

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
	"github.com/vihat/vigov/service-documents/internal/domain"
)

// CitizenLetterStore reads and writes one commune's citizen-letter register.
type CitizenLetterStore struct {
	db *store.DB
}

func NewCitizenLetterStore(db *store.DB) *CitizenLetterStore { return &CitizenLetterStore{db: db} }

// ErrCitizenLetterNotFound is "no such live letter IN THIS COMMUNE" — another commune's letter, a
// removed one and an id that never existed are one answer, because the query cannot reach the first
// two at all. The edge answers 404 with one sentence for all three.
var ErrCitizenLetterNotFound = errors.New("citizen_letter: không có đơn này trong xã")

// A ceiling was reached. Refused rather than truncated — a short log reads as complete, and a report
// missing rows reports a false figure.
var (
	ErrCitizenLetterLogTooLong     = errors.New("citizen_letter: nhật ký một đơn vượt trần")
	ErrCitizenLetterReportTooLarge = errors.New("citizen_letter: số đơn của năm vượt trần báo cáo")
)

// MaxLetterLogRows bounds one letter's log (TranLichSuChuyen's reasoning); MaxLetterReportRows bounds
// one report read (a commune books hundreds of letters a year — 50 000 is a register that is no longer
// one); MaxDuplicateScan bounds the rows the similarity is computed over.
const (
	MaxLetterLogRows    = 500
	MaxLetterReportRows = 50000
	MaxDuplicateScan    = 200
)

// letterClosedAt DERIVES the instant processing ended (domain.CitizenLetter.ClosedAt): `resolved_at`
// for the two ends of resolution; for the four processing-phase outcomes, the log row that moved the
// letter into the status it holds now. The status list is BUILT FROM domain.FinishedInProcessing, so
// the SQL holds no second copy of which statuses finish a letter.
//
// The subquery is correlated on `citizen_letter.tenant_id`, which the outer statement has bound to $1;
// `citizen_letter_log_by_letter` serves it.
var letterClosedAt = `COALESCE(resolved_at, CASE WHEN status IN (` + quotedStatuses(domain.FinishedInProcessing()) +
	`) THEN (SELECT max(lg.at) FROM citizen_letter_log lg WHERE lg.tenant_id = citizen_letter.tenant_id ` +
	`AND lg.letter_id = citizen_letter.id AND lg.to_status = citizen_letter.status) END)`

// quotedStatuses renders domain constants (never input) as a SQL list.
func quotedStatuses(ss []domain.LetterStatus) string {
	parts := make([]string, 0, len(ss))
	for _, s := range ss {
		parts = append(parts, "'"+string(s)+"'")
	}
	return strings.Join(parts, ", ")
}

// citizenLetterColumns IS READ BY POSITION in scanCitizenLetter. Two groups swap with no error: the
// three sender columns (all TEXT) and the instants (processing_due/resolution_due/accepted/resolved).
// Hence one constant, one scan.
var citizenLetterColumns = `id, number, year, received_date, letter_type, ` +
	`COALESCE(sender_name, ''), COALESCE(sender_phone, ''), COALESCE(sender_address, ''), summary, status, ` +
	`COALESCE(holding_unit_id, ''), COALESCE(assignee_code, ''), ` +
	`processing_due_at, resolution_due_at, accepted_at, resolved_at, ` + letterClosedAt + `, ` +
	`COALESCE(related_letter_id, ''), ` +
	`COALESCE(result_document_no, ''), result_document_date, COALESCE(result_signer, ''), ` +
	`COALESCE(result_issuer, ''), COALESCE(result_summary, ''), ` +
	`created_by_code, created_at, updated_at`

func scanCitizenLetter(scan func(...any) error) (domain.CitizenLetter, error) {
	var (
		l                                       domain.CitizenLetter
		letterType, status                      string
		procDue, resDue, accepted, resolved, cl sql.NullTime
		resultDate                              sql.NullTime
	)
	err := scan(&l.ID, &l.Number, &l.Year, &l.ReceivedDate, &letterType,
		&l.SenderName, &l.SenderPhone, &l.SenderAddress, &l.Summary, &status,
		&l.HoldingUnitID, &l.AssigneeCode,
		&procDue, &resDue, &accepted, &resolved, &cl,
		&l.RelatedLetterID,
		&l.ResultDocumentNo, &resultDate, &l.ResultSigner, &l.ResultIssuer, &l.ResultSummary,
		&l.CreatedByCode, &l.CreatedAt, &l.UpdatedAt)
	if err != nil {
		return domain.CitizenLetter{}, err
	}
	l.Type = domain.LetterType(letterType)
	l.Status = domain.LetterStatus(status)
	l.ProcessingDueAt = nullTime(procDue)
	l.ResolutionDueAt = nullTime(resDue)
	l.AcceptedAt = nullTime(accepted)
	l.ResolvedAt = nullTime(resolved)
	l.ClosedAt = nullTime(cl)
	l.ResultDocumentDate = nullTime(resultDate)
	return l, nil
}

func nullTime(t sql.NullTime) time.Time {
	if t.Valid {
		return t.Time
	}
	return time.Time{}
}

func timeOrNil(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

// --- the list ---------------------------------------------------------------------------------------

// SortCitizenLetters is the closed set of sorts GET /api/v1/citizen-letters offers.
//
// `created_at` DESCENDING IS THE DEFAULT — "newest first" is the booking instant. `number` is NOT the
// default, unlike the incoming register: without a `year` filter the numbers of two years interleave
// (2026 số 5 beside 2025 số 5). Neither the summary nor the sender is offered: a sort key travels in a
// URL, and both are personal data (rule 3, forbidden #4).
var SortCitizenLetters = page.NewAllowlist(page.Desc,
	page.Col("created_at", "created_at", page.KindTime),
	page.Col("number", "number", page.KindInt),
)

var cursorCitizenLetters = store.NewMoc[domain.CitizenLetter](SortCitizenLetters,
	map[string]func(domain.CitizenLetter) page.Key{
		"created_at": func(l domain.CitizenLetter) page.Key { return page.TimeKey(l.CreatedAt) },
		"number":     func(l domain.CitizenLetter) page.Key { return page.IntKey(int64(l.Number)) },
	})

// CitizenLetterFilter is the validated filter set. Every field becomes a BOUND PARAMETER.
type CitizenLetterFilter struct {
	Year          int                 // 0 = every year
	Status        domain.LetterStatus // "" = every status
	Type          domain.LetterType   // "" = every type
	HoldingUnitID string
	AssigneeCode  string
	ReceivedFrom  time.Time // zero = open
	ReceivedTo    time.Time // zero = open; inclusive
	Search        string    // "" = none; `number` exactly, or the summary of a NON-denunciation

	// MineCode is `scope=mine`: the SESSION's own staff code (never a request parameter).
	MineCode string
	// Related is `scope=related`.
	Related *CitizenLetterRelated
}

// CitizenLetterRelated is "Liên quan đến tôi": I hold it, I booked it, I wrote on its log, or one of
// my units holds it. OrgUnits is identity's StaffOrgUnits answer for the SESSION's code; empty means
// the unit clause is absent (matches nothing), never "every unit".
type CitizenLetterRelated struct {
	StaffCode string
	OrgUnits  []string
}

// List reads one page of the register, newest booking first by default.
func (s *CitizenLetterStore) List(ctx context.Context, f CitizenLetterFilter,
	req page.Request) (page.Result[domain.CitizenLetter], error) {

	cond, args := citizenLetterFilterSQL(f)
	return store.QueryPage(ctx, s.db.For(ctx), store.PageSpec{
		Columns: citizenLetterColumns,
		Table:   "citizen_letter",
		Filter:  `AND deleted_at IS NULL` + cond,
		Args:    args,
	}, req, cursorCitizenLetters, func(rows *sql.Rows) (domain.CitizenLetter, string, error) {
		l, err := scanCitizenLetter(rows.Scan)
		if err != nil {
			return domain.CitizenLetter{}, "", err
		}
		return l, l.ID, nil
	})
}

// citizenLetterFilterSQL builds the predicate and its bound values in ONE place, so placeholder
// numbers and argument order cannot drift. $1 is the commune.
func citizenLetterFilterSQL(f CitizenLetterFilter) (string, []any) {
	var (
		b    strings.Builder
		args []any
	)
	bind := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args)+1)
	}
	if f.Year != 0 {
		b.WriteString(" AND year = " + bind(f.Year))
	}
	if f.Status != "" {
		b.WriteString(" AND status = " + bind(string(f.Status)))
	}
	if f.Type != "" {
		b.WriteString(" AND letter_type = " + bind(string(f.Type)))
	}
	if f.HoldingUnitID != "" {
		b.WriteString(" AND holding_unit_id = " + bind(f.HoldingUnitID))
	}
	if f.AssigneeCode != "" {
		b.WriteString(" AND assignee_code = " + bind(f.AssigneeCode))
	}
	if !f.ReceivedFrom.IsZero() {
		b.WriteString(" AND received_date >= " + bind(f.ReceivedFrom))
	}
	if !f.ReceivedTo.IsZero() {
		b.WriteString(" AND received_date <= " + bind(f.ReceivedTo))
	}
	if f.Search != "" {
		// THE SUMMARY OF A DENUNCIATION IS NEVER SEARCHED. A hit would tell the searcher what the
		// letter says without showing it — the list withholds that summary (ADR 0078 #4), so the
		// search must not answer questions about it either.
		like := bind("%" + escapeLike(f.Search) + "%")
		denunciation := bind(string(domain.LetterTypeDenunciation))
		summary := "(letter_type <> " + denunciation + " AND summary ILIKE " + like + ")"
		if n, err := strconv.Atoi(f.Search); err == nil && n > 0 {
			b.WriteString(" AND (number = " + bind(n) + " OR " + summary + ")")
		} else {
			b.WriteString(" AND " + summary)
		}
	}
	if f.MineCode != "" {
		b.WriteString(" AND assignee_code = " + bind(f.MineCode))
	}
	if f.Related != nil {
		me := bind(f.Related.StaffCode)
		clauses := []string{
			"assignee_code = " + me,
			"created_by_code = " + me,
			"id IN (SELECT lg.letter_id FROM citizen_letter_log lg WHERE lg.tenant_id = $1 AND lg.actor_code = " + me + ")",
		}
		if len(f.Related.OrgUnits) > 0 {
			ph := make([]string, 0, len(f.Related.OrgUnits))
			for _, u := range f.Related.OrgUnits {
				ph = append(ph, bind(u))
			}
			clauses = append(clauses, "holding_unit_id IN ("+strings.Join(ph, ", ")+")")
		}
		b.WriteString(" AND (" + strings.Join(clauses, " OR ") + ")")
	}
	return b.String(), args
}

// escapeLike makes the search text literal inside ILIKE: `%` and `_` typed by a clerk are characters,
// not wildcards. Backslash is PostgreSQL's default LIKE escape.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// --- one letter -------------------------------------------------------------------------------------

func (s *CitizenLetterStore) one(ctx context.Context, tx *store.ScopedTx, id string, lock bool) (domain.CitizenLetter, error) {
	tail := `AND id = $2 AND deleted_at IS NULL`
	if lock {
		// FOR UPDATE: every write is read-decide-write (transition, result, routing). Without the lock
		// two officers moving one letter both decide against the old status.
		tail += ` FOR UPDATE`
	}
	// ScopedTx.Query writes `WHERE tenant_id = $1` itself and binds tx.TenantID() (rule 1, inv. 5).
	rows, err := tx.Query(ctx, citizenLetterColumns, "citizen_letter", tail, id)
	if err != nil {
		return domain.CitizenLetter{}, fmt.Errorf("citizen_letter: đọc đơn: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.CitizenLetter{}, fmt.Errorf("citizen_letter: đọc đơn: %w", err)
		}
		return domain.CitizenLetter{}, ErrCitizenLetterNotFound
	}
	l, err := scanCitizenLetter(rows.Scan)
	if err != nil {
		return domain.CitizenLetter{}, fmt.Errorf("citizen_letter: quét đơn: %w", err)
	}
	return l, nil
}

// ByID reads one live letter of this commune, without a lock.
func (s *CitizenLetterStore) ByID(ctx context.Context, tx *store.ScopedTx, id string) (domain.CitizenLetter, error) {
	return s.one(ctx, tx, id, false)
}

// ForUpdate reads one live letter and holds it until the transaction ends.
func (s *CitizenLetterStore) ForUpdate(ctx context.Context, tx *store.ScopedTx, id string) (domain.CitizenLetter, error) {
	return s.one(ctx, tx, id, true)
}

// --- writes -----------------------------------------------------------------------------------------

// insertCitizenLetter — `status` IS A LITERAL. A new letter is `moi-vao-so`, always; a parameter here
// is the one edit that lets a client book a letter already `da-giai-quyet`. The two due columns are
// ABSENT and stay NULL until a clerk sets one (UpdateDeadline) — never a default commitment (rule 10,
// forbidden #3).
const insertCitizenLetter = `INSERT INTO citizen_letter
	(tenant_id, id, number, year, received_date, letter_type, sender_name, sender_phone, sender_address,
	 summary, status, holding_unit_id, related_letter_id, created_by_code)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'moi-vao-so', $11, $12, $13)`

// Insert books one letter. `Number` comes from DaySoStore.CapSo in the same transaction.
func (s *CitizenLetterStore) Insert(ctx context.Context, tx *store.ScopedTx, l domain.CitizenLetter) error {
	_, err := tx.Exec(ctx, insertCitizenLetter, string(tx.TenantID()),
		l.ID, l.Number, l.Year, l.ReceivedDate, string(l.Type),
		rongThanhNil(l.SenderName), rongThanhNil(l.SenderPhone), rongThanhNil(l.SenderAddress),
		l.Summary, rongThanhNil(l.HoldingUnitID), rongThanhNil(l.RelatedLetterID), l.CreatedByCode)
	if err != nil {
		return fmt.Errorf("citizen_letter: chèn: %w", err)
	}
	return nil
}

// UpdateHolder moves the letter to a unit / officer. NOT the status: assignment is an attribute (C3).
func (s *CitizenLetterStore) UpdateHolder(ctx context.Context, tx *store.ScopedTx, id, unitID, assigneeCode, actorCode string) error {
	const stmt = `UPDATE citizen_letter
		SET holding_unit_id = $3, assignee_code = $4, updated_by_code = $5, updated_at = now()
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`
	return s.exec(ctx, tx, "chuyển đơn", stmt, id, rongThanhNil(unitID), rongThanhNil(assigneeCode), actorCode)
}

// UpdateStatus writes the status and the two instants 0006 binds to it in ONE statement.
func (s *CitizenLetterStore) UpdateStatus(ctx context.Context, tx *store.ScopedTx, l domain.CitizenLetter, actorCode string) error {
	const stmt = `UPDATE citizen_letter
		SET status = $3, accepted_at = $4, resolved_at = $5, updated_by_code = $6, updated_at = now()
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`
	return s.exec(ctx, tx, "đổi trạng thái", stmt, l.ID, string(l.Status),
		timeOrNil(l.AcceptedAt), timeOrNil(l.ResolvedAt), actorCode)
}

// UpdateResult writes C10's five result fields.
func (s *CitizenLetterStore) UpdateResult(ctx context.Context, tx *store.ScopedTx, l domain.CitizenLetter, actorCode string) error {
	const stmt = `UPDATE citizen_letter
		SET result_document_no = $3, result_document_date = $4, result_signer = $5, result_issuer = $6,
		    result_summary = $7, updated_by_code = $8, updated_at = now()
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`
	return s.exec(ctx, tx, "ghi kết quả", stmt, l.ID, rongThanhNil(l.ResultDocumentNo),
		ngayHoacNil(l.ResultDocumentDate), rongThanhNil(l.ResultSigner), rongThanhNil(l.ResultIssuer),
		rongThanhNil(l.ResultSummary), actorCode)
}

// UpdateSender writes the three sender fields — a correction, or anonymisation under Decree 13 (rule
// 7, invariant 7: replace identifying fields, keep the record).
func (s *CitizenLetterStore) UpdateSender(ctx context.Context, tx *store.ScopedTx, l domain.CitizenLetter, actorCode string) error {
	const stmt = `UPDATE citizen_letter
		SET sender_name = $3, sender_phone = $4, sender_address = $5, updated_by_code = $6, updated_at = now()
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`
	return s.exec(ctx, tx, "sửa người gửi", stmt, l.ID, rongThanhNil(l.SenderName),
		rongThanhNil(l.SenderPhone), rongThanhNil(l.SenderAddress), actorCode)
}

// UpdateDeadline writes the two due columns as the use case decided them (domain.SetActiveDue changed
// one, the other is the value read under the same row lock). A zero instant is NULL — "Không đặt".
// The value is written as given: no arithmetic here, and no now() (rule 10, invariant 2).
func (s *CitizenLetterStore) UpdateDeadline(ctx context.Context, tx *store.ScopedTx, l domain.CitizenLetter, actorCode string) error {
	const stmt = `UPDATE citizen_letter
		SET processing_due_at = $3, resolution_due_at = $4, updated_by_code = $5, updated_at = now()
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`
	return s.exec(ctx, tx, "đặt hạn xử lý", stmt, l.ID, timeOrNil(l.ProcessingDueAt),
		timeOrNil(l.ResolutionDueAt), actorCode)
}

// exec runs one UPDATE with tx.TenantID() as $1 and turns "no row touched" into not-found.
func (s *CitizenLetterStore) exec(ctx context.Context, tx *store.ScopedTx, op, stmt string, args ...any) error {
	kq, err := tx.Exec(ctx, stmt, append([]any{string(tx.TenantID())}, args...)...)
	if err != nil {
		return fmt.Errorf("citizen_letter: %s: %w", op, err)
	}
	n, err := kq.RowsAffected()
	if err != nil {
		return fmt.Errorf("citizen_letter: %s: đọc số dòng: %w", op, err)
	}
	if n == 0 {
		return ErrCitizenLetterNotFound
	}
	return nil
}

// --- the log ----------------------------------------------------------------------------------------

const insertLetterLog = `INSERT INTO citizen_letter_log
	(tenant_id, id, letter_id, at, actor_code, kind, from_status, to_status, from_unit_id, to_unit_id,
	 assignee_code, content)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

// InsertLog appends one log row, in the transaction of the act it records.
func (s *CitizenLetterStore) InsertLog(ctx context.Context, tx *store.ScopedTx, e domain.LetterLogEntry) error {
	_, err := tx.Exec(ctx, insertLetterLog, string(tx.TenantID()),
		e.ID, e.LetterID, e.At, e.ActorCode, string(e.Kind),
		rongThanhNil(string(e.FromStatus)), rongThanhNil(string(e.ToStatus)),
		rongThanhNil(e.FromUnitID), rongThanhNil(e.ToUnitID), rongThanhNil(e.AssigneeCode),
		rongThanhNil(e.Content))
	if err != nil {
		return fmt.Errorf("citizen_letter: ghi nhật ký: %w", err)
	}
	return nil
}

const letterLogColumns = `id, letter_id, at, actor_code, kind, COALESCE(from_status, ''), COALESCE(to_status, ''), ` +
	`COALESCE(from_unit_id, ''), COALESCE(to_unit_id, ''), COALESCE(assignee_code, ''), ` +
	`COALESCE(content, ''), created_at`

// Log reads one letter's log, NEWEST FIRST (`id` breaks a tie between two acts in one transaction).
// The caller has read the letter in the same transaction first.
func (s *CitizenLetterStore) Log(ctx context.Context, tx *store.ScopedTx, letterID string) ([]domain.LetterLogEntry, error) {
	// ScopedTx.Query writes `WHERE tenant_id = $1` itself and binds tx.TenantID() (rule 1, inv. 5).
	rows, err := tx.Query(ctx, letterLogColumns, "citizen_letter_log",
		`AND letter_id = $2 ORDER BY at DESC, id DESC LIMIT $3`, letterID, MaxLetterLogRows+1)
	if err != nil {
		return nil, fmt.Errorf("citizen_letter: đọc nhật ký: %w", err)
	}
	defer rows.Close()
	out := make([]domain.LetterLogEntry, 0, 8)
	for rows.Next() {
		var (
			e              domain.LetterLogEntry
			kind, from, to string
		)
		if err := rows.Scan(&e.ID, &e.LetterID, &e.At, &e.ActorCode, &kind, &from, &to,
			&e.FromUnitID, &e.ToUnitID, &e.AssigneeCode, &e.Content, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("citizen_letter: quét nhật ký: %w", err)
		}
		e.Kind, e.FromStatus, e.ToStatus = domain.LetterLogKind(kind), domain.LetterStatus(from), domain.LetterStatus(to)
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("citizen_letter: duyệt nhật ký: %w", err)
	}
	if len(out) > MaxLetterLogRows {
		return nil, ErrCitizenLetterLogTooLong
	}
	return out, nil
}

// --- the duplicate warning (C11) --------------------------------------------------------------------

// DuplicateCandidates reads the letters a new booking might duplicate: same commune, live, received on
// or after `since`, and — when a sender name was typed — the same name compared case-insensitively.
//
// `lower(sender_name) = lower($2)` BYTE FOR BYTE, with `sender_name IS NOT NULL`: the expression and
// the predicate of `citizen_letter_duplicate_by_sender`, which the planner uses only when the query
// says exactly that (0006:386-387). The name arrives in a POST body and is bound as a parameter — never
// in a URL, a log or a cache key (rule 3, forbidden #4).
//
// Without a name the window alone narrows (`citizen_letter_by_received_date`), newest first, bounded.
//
// A DENUNCIATION IS NEVER A CANDIDATE (`letter_type <> 'to-cao'`, bound). Listing one as a match on a
// typed name — even without its summary — already tells the clerk that this person filed a
// denunciation (Luật Tố cáo 2018 Đ.8; ADR 0078 #4). The cost is accepted: two denunciations from one
// sender are not flagged as duplicates. domain.RankDuplicates and the edge drop them again.
func (s *CitizenLetterStore) DuplicateCandidates(ctx context.Context, senderName string, since time.Time) ([]domain.CitizenLetter, error) {
	var (
		rows *sql.Rows
		err  error
	)
	denunciation := string(domain.LetterTypeDenunciation)
	if senderName != "" {
		rows, err = s.db.For(ctx).Query(ctx, citizenLetterColumns, "citizen_letter",
			`AND deleted_at IS NULL AND sender_name IS NOT NULL AND lower(sender_name) = lower($2) `+
				`AND received_date >= $3 AND letter_type <> $5 ORDER BY received_date DESC, id DESC LIMIT $4`,
			senderName, since, MaxDuplicateScan, denunciation)
	} else {
		rows, err = s.db.For(ctx).Query(ctx, citizenLetterColumns, "citizen_letter",
			`AND deleted_at IS NULL AND received_date >= $2 AND letter_type <> $4 `+
				`ORDER BY received_date DESC, id DESC LIMIT $3`,
			since, MaxDuplicateScan, denunciation)
	}
	if err != nil {
		return nil, fmt.Errorf("citizen_letter: đọc ứng viên trùng: %w", err)
	}
	return collectLetters(rows, "ứng viên trùng", MaxDuplicateScan, nil)
}

// --- the report -------------------------------------------------------------------------------------

// ReportRows reads the letters domain.BuildLetterReport needs for `year`: booked in it, or booked
// earlier and either still open or resolved on/after `yearStart` (the first instant of the year in
// Asia/Ho_Chi_Minh). Refused past MaxLetterReportRows rather than truncated.
func (s *CitizenLetterStore) ReportRows(ctx context.Context, year int, yearStart time.Time) ([]domain.CitizenLetter, error) {
	var finished []domain.LetterStatus
	for _, st := range domain.LetterStatuses {
		if st.Finished() {
			finished = append(finished, st)
		}
	}
	rows, err := s.db.For(ctx).Query(ctx, citizenLetterColumns, "citizen_letter",
		`AND deleted_at IS NULL AND year <= $2 AND (year = $2 OR status NOT IN (`+quotedStatuses(finished)+
			`) OR resolved_at >= $3) ORDER BY id LIMIT $4`,
		year, yearStart, MaxLetterReportRows+1)
	if err != nil {
		return nil, fmt.Errorf("citizen_letter: đọc báo cáo: %w", err)
	}
	return collectLetters(rows, "báo cáo", MaxLetterReportRows, ErrCitizenLetterReportTooLarge)
}

// collectLetters scans every row; past `max` it returns tooMany. A nil tooMany is for a read whose
// LIMIT is the bound itself.
func collectLetters(rows *sql.Rows, op string, max int, tooMany error) ([]domain.CitizenLetter, error) {
	defer rows.Close()
	var out []domain.CitizenLetter
	for rows.Next() {
		l, err := scanCitizenLetter(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("citizen_letter: quét %s: %w", op, err)
		}
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("citizen_letter: duyệt %s: %w", op, err)
	}
	if tooMany != nil && len(out) > max {
		return nil, tooMany
	}
	return out, nil
}
