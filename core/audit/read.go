package audit

// THE READ SIDE of the audit trail — the "Xem nhật ký hệ thống" screen (ADR 0054).
//
// WHY IT LIVES HERE AND NOT IN EACH SERVICE: five services own an `audit_log` with the same columns
// (ADR 0054 §1), and each must answer the same filters, in the same order, with the same fields, and
// must audit the read the same way. Five copies of that are five definitions of "the next page" and
// five places for the one mistake nobody notices — an entry for a citizen that shows their IP, or a
// read that leaves no trail. Each service keeps only what is genuinely its own: the route, the
// permission declaration and the handler that names the query parameters (tools/apidoc reads them
// from the service's own package, so they cannot move here without leaving the contract).
//
// WHAT THIS DELIBERATELY DOES NOT DO:
//
//   - Read another service's `audit_log`. There is no parameter naming a database; the handle is the
//     service's own *store.DB (rule 2, invariant 2; ADR 0054 stop condition #1).
//   - Re-mask `delta`. It is returned exactly as stored — masked at write time (rule 6, invariant 5).
//     A delta found holding raw personal data is a rule 3 incident AT THE WRITE SITE (ADR 0054 §4).
//   - Edit, delete or hide the content of any stored entry (ADR 0054 stop condition #4). What is
//     withheld is withheld from the RESPONSE, never from storage: two columns of a citizen's entries,
//     and every entry whose actor_kind is KindOperator — a Vihat operator's act on the commune is
//     shown in the operator log only, by the owner's decision (ADR 0070 §"Bổ sung 02/10/2026" #7).
//     The entry still exists, in the same transaction as the change (rule 6, invariant 1).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
)

// ActionReadLog is the verb of the entry every read of the audit log leaves (ADR 0054 §5). Same shape
// as `xem_day_du_nguoi_gui` in service-petitions.
const ActionReadLog = "xem_nhat_ky_he_thong"

// actorKindCitizen is the actor_kind whose code and IP are never returned (ADR 0054 §4).
const actorKindCitizen = "citizen"

// maxFilterLen bounds the three exact-match text filters. A business code is a few dozen bytes; a
// filter of kilobytes matches nothing and would be copied whole into the read's own audit entry.
const maxFilterLen = 128

var (
	// ErrRange means from/to did not parse as RFC 3339, or from >= to. 400.
	ErrRange = errors.New("audit: khoảng thời gian không hợp lệ")
	// ErrFilter means actor/action/subject exceeded maxFilterLen. 400.
	ErrFilter = errors.New("audit: giá trị lọc không hợp lệ")
	// ErrNoReader means the read has nobody to attribute it to. Never answered as 4xx: the route sits
	// behind authz.RequirePermission, so reaching here without a business code is wiring gone wrong.
	ErrNoReader = errors.New("audit: không có người đọc — lượt đọc sổ vết phải ghi được là của ai")
)

// Query is one request to read the audit log, AS THE CLIENT SENT IT — raw query-string values.
//
// RAW ON PURPOSE: parsing happens inside Log.Read, so there is one place that validates, and a
// request rejected there still leaves its entry (ADR 0054 §5 "Ghi vết lỗi"). The commune is not a
// field and must never become one: it comes from the context (rule 1, forbidden #2).
type Query struct {
	From, To string // RFC 3339, half-open [From, To) on `at`
	Actor    string // staff business code (CB-…) or "system"; never matches a citizen's entry
	Action   string // exact verb; ActionReadLog shows the read entries, which are hidden otherwise
	Subject  string // exact business code
	Limit    string // core/page
	Cursor   string // core/page — opaque, carries no commune

	// SeeHidden lifts the WithHiddenSubjects exclusion for this request. The caller decides it from
	// a permission it has checked (petitions: `feedback.restricted`, ADR 0030); it is never read
	// from the client.
	SeeHidden bool
}

// EntryView is one audit entry as the screen receives it — the fields ADR 0054 §4 lists, and no
// others. No `id`: the cursor already carries it and the screen has no use for it.
type EntryView struct {
	At        time.Time `json:"at"`
	ActorKind string    `json:"actor_kind"`
	// ActorCode is the staff code or "system". EMPTY for a citizen: the column holds an internal id
	// that names nobody to staff but LINKS every act of one citizen together (ADR 0054 §4).
	ActorCode string `json:"actor_code"`
	// ActorIP is EMPTY for a citizen: showing it to `admin.audit` would be showing personal data to a
	// new role — rule 3 stop condition #1, not asked of the customer (ADR 0054 §Còn mở #2).
	ActorIP string `json:"actor_ip"`
	Action  string `json:"action"`
	Subject string `json:"subject"`
	// Delta is the stored JSON EXACTLY AS STORED (a json.RawMessage inside), null when the entry has
	// none. Typed `any` and not json.RawMessage because the shape differs per verb and the published
	// contract must say "any JSON" — tools/apidoc refuses a type it cannot see into, rightly.
	Delta any `json:"delta"`
}

// Log reads one service's own `audit_log`.
type Log struct {
	db     *store.DB
	hidden string // WithHiddenSubjects — a subquery of subjects, or ""
}

// Option configures a Log.
type Option func(*Log)

// WithHiddenSubjects withholds every entry whose subject is returned by `subquery`, unless the
// request sets Query.SeeHidden.
//
// FOR petitions (ADR 0054 §4): entries about a petition in the restricted field must not become a
// second way to learn that a complaint against an official exists (ADR 0030). The subquery is a
// CONSTANT from the service's store package — it is SQL, and only a store knows SQL — and it may
// reference exactly one placeholder, $1, the commune. It is wrapped so a NULL it returns cannot turn
// `NOT IN` into "hide everything".
//
// Panics at wiring time on anything that is not a single SELECT with only $1: this runs once, from
// a constant, and a client string reaching it would be an injection route.
func WithHiddenSubjects(subquery string) Option {
	s := strings.TrimSpace(subquery)
	low := strings.ToLower(s)
	if !strings.HasPrefix(low, "select ") || strings.Contains(s, ";") {
		panic("audit: WithHiddenSubjects cần đúng một câu SELECT")
	}
	for i := 2; i <= 9; i++ {
		if strings.Contains(s, "$"+strconv.Itoa(i)) {
			panic("audit: WithHiddenSubjects chỉ được dùng $1 (mã xã)")
		}
	}
	return func(l *Log) { l.hidden = s }
}

// NewLog binds the reader to the service's own database handle.
func NewLog(db *store.DB, opts ...Option) *Log {
	if db == nil {
		panic("audit: NewLog thiếu *store.DB")
	}
	l := &Log{db: db}
	for _, o := range opts {
		o(l)
	}
	return l
}

// readOrder is the ONE order the audit log is read in (ADR 0054 §4): `at` descending, ties broken by
// `id` descending. It exists so the cursor has the same shape as every other list's; no `sort` or
// `order` is ever taken from the client.
var readOrder = page.NewAllowlist(page.Desc, page.Col("at", "at", page.KindTime))

// filter is a validated Query.
type filter struct {
	from, to               time.Time
	hasFrom, hasTo         bool
	actor, action, subject string
	cursor, seeHidden      bool
	req                    page.Request
	lastID                 int64 // the anchor's id, when req carries one
}

func parseQuery(q Query) (filter, error) {
	f := filter{
		actor: q.Actor, action: q.Action, subject: q.Subject,
		cursor: q.Cursor != "", seeHidden: q.SeeHidden,
	}
	var err error
	if q.From != "" {
		if f.from, err = time.Parse(time.RFC3339, q.From); err != nil {
			return f, fmt.Errorf("%w: from", ErrRange)
		}
		f.hasFrom = true
	}
	if q.To != "" {
		if f.to, err = time.Parse(time.RFC3339, q.To); err != nil {
			return f, fmt.Errorf("%w: to", ErrRange)
		}
		f.hasTo = true
	}
	if f.hasFrom && f.hasTo && !f.from.Before(f.to) {
		return f, fmt.Errorf("%w: from >= to", ErrRange)
	}
	for _, v := range []string{q.Actor, q.Action, q.Subject} {
		if len(v) > maxFilterLen {
			return f, fmt.Errorf("%w: quá %d byte", ErrFilter, maxFilterLen)
		}
	}
	if f.req, err = page.New(readOrder, "", "", q.Limit, q.Cursor); err != nil {
		return f, err
	}
	if a, ok := f.req.After(); ok {
		// `audit_log.id` is BIGSERIAL. A cursor id that is not an integer is not one this package
		// wrote; refusing it here makes it a 400 instead of a type error from the database.
		if f.lastID, err = strconv.ParseInt(a.ID, 10, 64); err != nil || f.lastID < 1 {
			return f, fmt.Errorf("%w: khoá phá hoà không phải số", page.ErrCursor)
		}
	}
	return f, nil
}

// Read returns one page of the commune's audit log, and records the read itself.
//
// THE READ AND ITS ENTRY SHARE ONE TRANSACTION (ADR 0054 §5): a page handed out whose entry did not
// commit is a read of the audit log that left no trail — stop condition #5. So when the entry
// cannot be written, the page is NOT returned; the error is.
//
// A REQUEST THAT FAILS STILL LEAVES ITS ENTRY ("Ghi vết lỗi"): a rejected filter, or a read the
// database refused, is recorded in a transaction of its own, with the outcome. Only if THAT write
// also fails does the request end with no entry — and it then ends as an error, never as data.
//
// `reader` is the principal's business code (rule 6, invariant 8) — also the entry's subject, the
// same pattern `dang_nhap` uses. The commune is the context's (rule 1, invariant 4).
func (l *Log) Read(ctx context.Context, reader Actor, q Query) (page.Result[EntryView], error) {
	out := page.NewResult[EntryView]()
	if reader.ID == "" {
		return out, ErrNoReader // before any statement: a read nobody can be named for does not run
	}
	s := l.db.For(ctx)

	f, perr := parseQuery(q)
	if perr != nil {
		delta := readDelta{Outcome: "rejected", Reason: reasonCode(perr), Cursor: q.Cursor != ""}
		if err := s.Tx(ctx, func(tx *store.ScopedTx) error {
			return l.writeReadEntry(ctx, tx, reader, delta)
		}); err != nil {
			return out, fmt.Errorf("audit: không ghi được vết của lượt đọc bị từ chối: %w", err)
		}
		return out, perr
	}

	err := s.Tx(ctx, func(tx *store.ScopedTx) error {
		var err error
		if out, err = l.readPage(ctx, tx, f); err != nil {
			return err
		}
		d := l.summary(f, "ok")
		d.Returned = len(out.Items)
		return l.writeReadEntry(ctx, tx, reader, d)
	})
	if err != nil {
		// The transaction rolled back, the page with it. Record that the read was attempted.
		if werr := s.Tx(ctx, func(tx *store.ScopedTx) error {
			return l.writeReadEntry(ctx, tx, reader, l.summary(f, "error"))
		}); werr != nil {
			err = fmt.Errorf("%w (và không ghi được vết của lượt đọc lỗi: %v)", err, werr)
		}
		return page.NewResult[EntryView](), err
	}
	return out, nil
}

// readPage is the keyset read. It is not store.QueryPage for two reasons, both about this table:
// it must run on the TRANSACTION that also writes the read's entry, and the tie-break `id` is a
// BIGSERIAL, so the anchor is bound as an int64 — QueryPage binds it as text, which suits the ULID
// ids of every business table. The ORDER BY and the limit+1 have exactly QueryPage's shape.
func (l *Log) readPage(ctx context.Context, tx *store.ScopedTx, f filter) (page.Result[EntryView], error) {
	out := page.NewResult[EntryView]()

	var tail strings.Builder
	args := make([]any, 0, 8)
	next := 2 // $1 is the commune, bound by ScopedTx.Query
	add := func(clause string, v any) {
		fmt.Fprintf(&tail, clause, next)
		args = append(args, v)
		next++
	}
	// UNCONDITIONAL, before every filter: no actor, action or subject a client sends can bring an
	// operator's entry back onto the commune's screen (ADR 0070 §"Bổ sung 02/10/2026" #7). A constant
	// literal, not a placeholder, so the numbering of every client filter is unchanged.
	tail.WriteString(" AND actor_kind <> '" + KindOperator + "'")
	if f.hasFrom {
		add(" AND at >= $%d", f.from)
	}
	if f.hasTo {
		add(" AND at < $%d", f.to)
	}
	if f.actor != "" {
		// A citizen's actor_id is an internal id this route never returns; matching on it would let a
		// guessed id confirm a citizen's activity.
		add(" AND actor_id = $%d AND actor_kind <> '"+actorKindCitizen+"'", f.actor)
	}
	if f.action != "" {
		add(" AND action = $%d", f.action)
	} else {
		// Hidden by default (ADR 0054 §5): otherwise every opening of the screen pushes itself to the
		// top, and the log fills with views instead of acts.
		add(" AND action <> $%d", ActionReadLog)
	}
	if f.subject != "" {
		add(" AND subject = $%d", f.subject)
	}
	if l.hidden != "" && !f.seeHidden {
		hiddenSet := "(SELECT h.s FROM (" + l.hidden + ") AS h(s) WHERE h.s IS NOT NULL)"
		tail.WriteString(" AND subject NOT IN " + hiddenSet)
		// A READ ENTRY ECHOES ITS FILTER. Before readDelta stopped recording the subject (see
		// Log.summary), a read entry carried the searched subject in delta->>'subject' — for petitions
		// a can-bo lookup code — and `returned` told whether it matched. Listing action=ActionReadLog
		// would then hand a reader without the right the very code the clause above withholds, and
		// confirm the complaint exists (ADR 0030; ADR 0054 §4). So read entries naming a hidden
		// subject are withheld too; this covers every entry already written in the old shape.
		// ONLY `subject` can carry such a code: `actor` is a staff code, `action` a verb, from/to
		// instants. A read entry without the key (delta NULL, or the new shape) is kept — the IS NULL
		// arm stops `NOT IN` over a NULL from hiding it. Needs `delta` to be JSONB, which it is in the
		// one service configured with hidden subjects (service-petitions migrations/0001_init.sql).
		fmt.Fprintf(&tail, " AND (action <> $%d OR delta->>'subject' IS NULL OR delta->>'subject' NOT IN ", next)
		tail.WriteString(hiddenSet + ")")
		args = append(args, ActionReadLog)
		next++
	}
	if a, ok := f.req.After(); ok {
		fmt.Fprintf(&tail, " AND (at, id) < ($%d, $%d)", next, next+1)
		args = append(args, a.Key.Time(), f.lastID)
		next += 2
	}
	fmt.Fprintf(&tail, " ORDER BY at DESC, id DESC LIMIT $%d", next)
	args = append(args, f.req.Limit()+1)

	rows, err := tx.Query(ctx,
		"id, at, actor_kind, actor_id, actor_ip, action, subject, delta", "audit_log",
		tail.String(), args...)
	if err != nil {
		return out, fmt.Errorf("audit: đọc nhật ký: %w", err)
	}
	defer rows.Close()

	var last page.Anchor
	for rows.Next() {
		if len(out.Items) == f.req.Limit() {
			out.HasMore = true // the limit+1-th row: a signal only, never returned
			break
		}
		var (
			id    int64
			v     EntryView
			delta []byte
		)
		if err := rows.Scan(&id, &v.At, &v.ActorKind, &v.ActorCode, &v.ActorIP,
			&v.Action, &v.Subject, &delta); err != nil {
			return page.NewResult[EntryView](), fmt.Errorf("audit: quét dòng nhật ký: %w", err)
		}
		if v.ActorKind == actorKindCitizen {
			v.ActorCode, v.ActorIP = "", ""
		}
		if len(delta) > 0 {
			v.Delta = json.RawMessage(delta)
		}
		out.Items = append(out.Items, v)
		last = page.Anchor{Key: page.TimeKey(v.At), ID: strconv.FormatInt(id, 10)}
	}
	if err := rows.Err(); err != nil {
		return page.NewResult[EntryView](), fmt.Errorf("audit: đọc nhật ký: %w", err)
	}
	if out.HasMore {
		out.NextCursor = page.Encode(f.req.Column(), f.req.Dir(), last)
	}
	return out, nil
}

// readDelta is the filter summary the read's own entry carries (ADR 0054 §5). No entry that was
// returned is copied into it — only what was asked, and how many came back.
type readDelta struct {
	From    string `json:"from,omitempty"`
	To      string `json:"to,omitempty"`
	Actor   string `json:"actor,omitempty"`
	Action  string `json:"action,omitempty"`
	Subject string `json:"subject,omitempty"`
	// SubjectFiltered replaces Subject when the Log hides subjects (WithHiddenSubjects): the entry
	// records THAT a subject was searched, never which one — see Log.summary.
	SubjectFiltered bool `json:"subject_filtered,omitempty"`
	Cursor          bool `json:"cursor"`
	Returned        int  `json:"returned"`
	// Outcome is "ok", "rejected" (the filter was refused, nothing read) or "error" (the read or its
	// entry failed, nothing returned). Reason is set for "rejected" only, as a code — the rejected
	// raw values are not copied, since nothing was read with them.
	Outcome string `json:"outcome"`
	Reason  string `json:"reason,omitempty"`
}

// summary is the read's own entry for a validated filter.
//
// WHEN THE LOG HIDES SUBJECTS, THE SEARCHED SUBJECT IS NOT RECORDED — only that there was one. The
// entry is read later by anybody holding the log-reading right, and for petitions that right does not
// include the restricted field (ADR 0030): a stored subject would be a can-bo lookup code, and
// `returned` would confirm it exists. readPage also withholds old entries that carry it; not writing
// it at all means no future reader, filter or export can learn it. This holds even when THIS reader
// may see hidden subjects: the entry outlives the request, and its next reader may not.
// The cost: an auditor cannot tell which subject a colleague searched. Logs without hidden subjects
// (identity, documents, finance, comms) record the subject as before.
func (l *Log) summary(f filter, outcome string) readDelta {
	d := readDelta{
		Actor: f.actor, Action: f.action, Subject: f.subject,
		Cursor: f.cursor, Outcome: outcome,
	}
	if l.hidden != "" {
		d.Subject, d.SubjectFiltered = "", f.subject != ""
	}
	if f.hasFrom {
		d.From = f.from.UTC().Format(time.RFC3339Nano)
	}
	if f.hasTo {
		d.To = f.to.UTC().Format(time.RFC3339Nano)
	}
	return d
}

func (l *Log) writeReadEntry(ctx context.Context, tx *store.ScopedTx, reader Actor, d readDelta) error {
	b, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("audit: mã hoá tóm tắt lượt đọc: %w", err)
	}
	return Write(ctx, tx, Entry{
		Actor:   reader,
		Action:  ActionReadLog,
		Subject: reader.ID,
		Delta:   b,
	})
}

func reasonCode(err error) string {
	switch {
	case errors.Is(err, ErrRange):
		return "invalid_range"
	case errors.Is(err, ErrFilter):
		return "invalid_filter"
	case errors.Is(err, page.ErrCursor):
		return "invalid_cursor"
	case errors.Is(err, page.ErrLimit):
		return "invalid_limit"
	default:
		return "invalid_request"
	}
}

// HTTPError maps a Read failure onto the single error shape of core/httpx: status, code, message.
// `code` is an identifier; `message` is read by a person and is Vietnamese. Nothing the client sent
// is echoed back. Anything unrecognised is a 500 — including ErrNoReader, which is wiring gone wrong.
func HTTPError(err error) (status int, code, message string) {
	switch {
	case errors.Is(err, ErrRange):
		return http.StatusBadRequest, "invalid_range",
			"Khoảng thời gian không hợp lệ: from và to theo RFC 3339, from phải trước to."
	case errors.Is(err, ErrFilter):
		return http.StatusBadRequest, "invalid_filter", "Giá trị lọc quá dài."
	case errors.Is(err, page.ErrCursor), errors.Is(err, page.ErrLimit), errors.Is(err, page.ErrSort):
		return page.HTTPError(err)
	default:
		return http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại."
	}
}
