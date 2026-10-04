package store

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/service-platform/internal/domain"
)

// OperatorLog is the READ side of the operator log screen (ADR 0073 #2): every act a Vihat operator
// made through this service, newest first, from the two trails THIS service owns —
//
//	audit_log            rows with actor_kind = 'operator' — an operator's act ON a commune, filed
//	                     under that commune (ADR 0048 §Thiết kế #6). Withheld from the commune's own
//	                     screen (core/audit.Log.Read, ADR 0070 bổ sung #7); shown here.
//	platform_audit_log   changes to platform-wide configuration (upload limits) — no commune by
//	                     definition (migration 0008). Every row, 'system' migrations included: they are
//	                     the history of the values an operator is about to change.
//
// NOTHING ELSE. A commune staff member's entries (actor_kind staff — branding edits) never appear:
// the screen is the operator log, and those belong to the commune's own screen.
//
// A RAW HANDLE, like OperatorRegistry: the general read spans every commune by design, which no
// scoped handle can express.
//
// THE READ IS ITSELF TRAILED (rule 6 invariant 7: reading across communes is audited — no exception
// was decided for this screen, unlike the commune list of ADR 0048 §30/09 #5). One platform_audit_log
// row per page, `operator_log.read`, in the SAME transaction as the read: a page whose entry did not
// commit is not returned (ADR 0054 §5's rule, applied here). Those rows are never listed back — the
// log would otherwise fill with its own views.
type OperatorLog struct {
	db *sql.DB
}

// NewOperatorLog binds the reader to this service's own database (rule 2 invariant 2).
func NewOperatorLog(db *sql.DB) *OperatorLog { return &OperatorLog{db: db} }

// OperatorLogOrder is the one order the log is read in: newest first. No sort is taken from the client.
var OperatorLogOrder = page.NewAllowlist(page.Desc, page.Col("at", "at", page.KindTime))

// ActionOperatorLogRead is the platform_audit_log verb of one page read.
const ActionOperatorLogRead = "operator_log.read"

// OperatorLogQuery is a validated request. CommuneID empty = every commune AND the platform-wide
// entries; set = that commune's entries only (a platform-wide change belongs to no commune).
type OperatorLogQuery struct {
	CommuneID string
	From, To  time.Time // zero = unbounded; half-open [From, To)
	Page      page.Request
}

// The cursor's tie-break id is "<source>:<row id>" — two tables, two BIGSERIALs, so the id alone is
// not unique across the merged stream. Source 'c' = audit_log, 'p' = platform_audit_log; the order
// (at DESC, src DESC, id DESC) is total.
func parseLogAnchor(id string) (src string, n int64, err error) {
	s, num, ok := strings.Cut(id, ":")
	if !ok || (s != "c" && s != "p") {
		return "", 0, fmt.Errorf("%w: khoá phá hoà không đúng dạng", page.ErrCursor)
	}
	if n, err = strconv.ParseInt(num, 10, 64); err != nil || n < 1 {
		return "", 0, fmt.Errorf("%w: khoá phá hoà không phải số", page.ErrCursor)
	}
	return s, n, nil
}

// Read returns one page and records the read, in one transaction. reader is the operator (VH- code
// and address); an empty code refuses before any statement.
func (l *OperatorLog) Read(ctx context.Context, q OperatorLogQuery, reader domain.OperatorActor) (
	res page.Result[domain.OperatorLogEntry], err error) {
	res = page.NewResult[domain.OperatorLogEntry]()
	if err := reader.Validate(); err != nil {
		return res, err
	}
	var (
		afterAt  any
		afterSrc string
		afterID  int64
	)
	if a, ok := q.Page.After(); ok {
		if afterSrc, afterID, err = parseLogAnchor(a.ID); err != nil {
			return res, err
		}
		afterAt = a.Key.Time()
	}
	var from, to any
	if !q.From.IsZero() {
		from = q.From
	}
	if !q.To.IsZero() {
		to = q.To
	}
	var commune any
	if q.CommuneID != "" {
		commune = q.CommuneID
	}

	// BEFORE/AFTER, uniformly: an audit_log delta with a truoc/sau pair gives both; one without (a
	// creation, an attachment, a secret forward) is "after" whole, minus the reason, which has its own
	// field. platform_audit_log has the three columns already.
	const stmt = `
	SELECT src, id, at, actor, action, commune_id, commune_name, subject, before, after, reason FROM (
		SELECT 'c'::text AS src, a.id, a.at, a.actor_id AS actor, a.action, a.tenant_id AS commune_id,
		       COALESCE(t.ten, '') AS commune_name, a.subject,
		       CASE WHEN a.delta->'truoc' IS NOT NULL OR a.delta->'sau' IS NOT NULL THEN a.delta->'truoc' END AS before,
		       CASE WHEN a.delta->'truoc' IS NOT NULL OR a.delta->'sau' IS NOT NULL THEN a.delta->'sau'
		            ELSE a.delta - 'ly_do' END AS after,
		       COALESCE(a.delta->>'ly_do', '') AS reason
		  FROM audit_log a LEFT JOIN tenant t ON t.id = a.tenant_id
		 WHERE a.actor_kind = 'operator' AND ($1::text IS NULL OR a.tenant_id = $1)
		UNION ALL
		SELECT 'p'::text, p.id, p.occurred_at, p.actor, p.action, ''::text, ''::text, p.subject,
		       p.before, p.after, COALESCE(p.reason, '')
		  FROM platform_audit_log p
		 WHERE $1::text IS NULL AND p.action <> $2
	) e
	WHERE ($3::timestamptz IS NULL OR at >= $3)
	  AND ($4::timestamptz IS NULL OR at < $4)
	  AND ($5::timestamptz IS NULL OR (at, src, id) < ($5::timestamptz, $6::text, $7::bigint))
	ORDER BY at DESC, src DESC, id DESC
	LIMIT $8`

	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return res, fmt.Errorf("operator log: begin: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// @cross-tenant: the operator log (ADR 0073 #2) — operator acts on EVERY commune, read by a Vihat
	// operator on OPERATOR_HOST only (never mounted on a commune host). Metadata of operator writes
	// only; the read itself is trailed below in this transaction (rule 6 invariant 7).
	rows, err := tx.QueryContext(ctx, stmt, commune, ActionOperatorLogRead, from, to,
		afterAt, afterSrc, afterID, q.Page.Limit()+1)
	if err != nil {
		return res, fmt.Errorf("operator log: read: %w", err)
	}
	var lastSrc string
	var lastID int64
	for rows.Next() {
		if len(res.Items) == q.Page.Limit() {
			res.HasMore = true
			break
		}
		var (
			e             domain.OperatorLogEntry
			src           string
			id            int64
			before, after []byte
		)
		if err = rows.Scan(&src, &id, &e.At, &e.ActorCode, &e.Action, &e.CommuneID, &e.CommuneName,
			&e.Subject, &before, &after, &e.Reason); err != nil {
			rows.Close()
			return page.NewResult[domain.OperatorLogEntry](), fmt.Errorf("operator log: scan: %w", err)
		}
		e.Before, e.After = nullJSON(before), nullJSON(after)
		res.Items = append(res.Items, e)
		lastSrc, lastID = src, id
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return page.NewResult[domain.OperatorLogEntry](), fmt.Errorf("operator log: read: %w", err)
	}
	if err = rows.Close(); err != nil {
		return page.NewResult[domain.OperatorLogEntry](), fmt.Errorf("operator log: read: %w", err)
	}
	if res.HasMore {
		last := res.Items[len(res.Items)-1]
		res.NextCursor = page.Encode(q.Page.Column(), q.Page.Dir(),
			page.Anchor{Key: page.TimeKey(last.At), ID: lastSrc + ":" + strconv.FormatInt(lastID, 10)})
	}

	// The read's own entry: what was asked and how many came back — never a returned row.
	summary := map[string]any{"cursor": afterAt != nil, "returned": len(res.Items)}
	if q.CommuneID != "" {
		summary["commune_id"] = q.CommuneID
	}
	if !q.From.IsZero() {
		summary["from"] = q.From.UTC().Format(time.RFC3339Nano)
	}
	if !q.To.IsZero() {
		summary["to"] = q.To.UTC().Format(time.RFC3339Nano)
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO platform_audit_log (actor, actor_ip, action, subject, before, after)
		VALUES ($1, $2, $3, $1, NULL, $4::jsonb)`,
		reader.Code, reader.IP, ActionOperatorLogRead, string(delta(summary))); err != nil {
		return page.NewResult[domain.OperatorLogEntry](), fmt.Errorf("operator log: trail of the read: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return page.NewResult[domain.OperatorLogEntry](), fmt.Errorf("operator log: commit: %w", err)
	}
	return res, nil
}

// nullJSON maps SQL NULL and JSON null to nil, so "absent" has one spelling on the wire.
func nullJSON(b []byte) []byte {
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	return b
}
