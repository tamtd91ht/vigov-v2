package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// CitizenLetterDeadlineRuleStore reads and writes `citizen_letter_deadline_rule` (migration 0027) —
// a commune's deadline rule per letter type and deadline kind.
//
// THE SAME FOUR PROPERTIES AS THE `sla` WRITE PATHS (sla_ghi.go), for the same reasons:
//
//  1. THE COMMUNE IS $1 IN EVERY STATEMENT, from the context (rule 1, invariants 4 and 5). No method
//     takes a commune, so no caller can name another commune's row.
//  2. NOTHING HERE OPENS A TRANSACTION. Every write takes the *store.ScopedTx its audit entry is
//     written in (rule 6, invariant 3).
//  3. NO UPDATE NAMES `letter_type` OR `deadline_kind`. What a rule applies to is fixed at creation.
//  4. NO HARD DELETE (rule 7).
//
// THIS STORE DOES NOT VALIDATE THE UNIT LOCK. A row is returned as stored; the domain decides whether
// it may be used (domain.CheckCitizenLetterDeadlineRule) — the gRPC read refuses it, the screen shows it.
type CitizenLetterDeadlineRuleStore struct {
	db *store.DB
}

func NewCitizenLetterDeadlineRuleStore(db *store.DB) *CitizenLetterDeadlineRuleStore {
	return &CitizenLetterDeadlineRuleStore{db: db}
}

// MaxCitizenLetterDeadlineRules bounds the list read. A commune has at most SIX live rules (four
// processing + two resolution, the 0027 live-unique key); 64 is the point past which the rows have
// stopped being a rule table. Refused rather than truncated, as TranSLA.
const MaxCitizenLetterDeadlineRules = 64

var (
	// ErrCitizenLetterDeadlineRuleNotFound — no live rule of THIS commune has this id. One answer for
	// an invented id, a removed rule and another commune's rule (rule 4, forbidden #2).
	ErrCitizenLetterDeadlineRuleNotFound = errors.New("hạn đơn thư: không tìm thấy quy tắc")

	// ErrCitizenLetterDeadlineRuleExists — the live-unique key refused a second live rule for the same
	// (letter type, deadline kind).
	ErrCitizenLetterDeadlineRuleExists = errors.New("hạn đơn thư: loại đơn này đã có quy tắc cho loại hạn này")

	// ErrTooManyCitizenLetterDeadlineRules — the list ceiling was reached.
	ErrTooManyCitizenLetterDeadlineRules = errors.New("hạn đơn thư: vượt trần số quy tắc")
)

// citizenLetterDeadlineRuleCols is read BY POSITION in scanCitizenLetterDeadlineRule. One list, one
// scan, for every read.
const citizenLetterDeadlineRuleCols = `id, letter_type, deadline_kind, amount, unit`

func scanCitizenLetterDeadlineRule(scan func(...any) error) (domain.CitizenLetterDeadlineRule, error) {
	var r domain.CitizenLetterDeadlineRule
	var letterType, kind, unit string
	if err := scan(&r.ID, &letterType, &kind, &r.Amount, &unit); err != nil {
		return domain.CitizenLetterDeadlineRule{}, err
	}
	r.LetterType = domain.CitizenLetterType(letterType)
	r.Kind = domain.CitizenLetterDeadlineKind(kind)
	r.Unit = domain.CitizenLetterDeadlineUnit(unit)
	return r, nil
}

// List reads this commune's live rules, ordered by (letter_type, deadline_kind) — total, because the
// live-unique key makes the pair unique among live rows.
func (s *CitizenLetterDeadlineRuleStore) List(ctx context.Context) ([]domain.CitizenLetterDeadlineRule, error) {
	rows, err := s.db.For(ctx).Query(ctx, citizenLetterDeadlineRuleCols, "citizen_letter_deadline_rule",
		`AND deleted_at IS NULL ORDER BY letter_type, deadline_kind LIMIT $2`, MaxCitizenLetterDeadlineRules+1)
	if err != nil {
		return nil, fmt.Errorf("hạn đơn thư: đọc quy tắc: %w", err)
	}
	defer rows.Close()

	out := make([]domain.CitizenLetterDeadlineRule, 0, 6)
	for rows.Next() {
		r, err := scanCitizenLetterDeadlineRule(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("hạn đơn thư: đọc dòng quy tắc: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("hạn đơn thư: duyệt quy tắc: %w", err)
	}
	if len(out) > MaxCitizenLetterDeadlineRules {
		return nil, ErrTooManyCitizenLetterDeadlineRules
	}
	return out, nil
}

// Live reads THE live rule for one (letter type, deadline kind). found == false is "not configured" —
// an answer, not an error (ADR 0085 B3). The live-unique key guarantees at most one row; LIMIT 2 makes
// a second one (a database whose constraint is gone) a refusal rather than an arbitrary pick.
func (s *CitizenLetterDeadlineRuleStore) Live(ctx context.Context, letterType domain.CitizenLetterType,
	kind domain.CitizenLetterDeadlineKind) (domain.CitizenLetterDeadlineRule, bool, error) {

	rows, err := s.db.For(ctx).Query(ctx, citizenLetterDeadlineRuleCols, "citizen_letter_deadline_rule",
		`AND letter_type = $2 AND deadline_kind = $3 AND deleted_at IS NULL LIMIT 2`,
		string(letterType), string(kind))
	if err != nil {
		return domain.CitizenLetterDeadlineRule{}, false, fmt.Errorf("hạn đơn thư: đọc quy tắc: %w", err)
	}
	defer rows.Close()

	var found []domain.CitizenLetterDeadlineRule
	for rows.Next() {
		r, err := scanCitizenLetterDeadlineRule(rows.Scan)
		if err != nil {
			return domain.CitizenLetterDeadlineRule{}, false, fmt.Errorf("hạn đơn thư: đọc dòng quy tắc: %w", err)
		}
		found = append(found, r)
	}
	if err := rows.Err(); err != nil {
		return domain.CitizenLetterDeadlineRule{}, false, fmt.Errorf("hạn đơn thư: duyệt quy tắc: %w", err)
	}
	switch len(found) {
	case 0:
		return domain.CitizenLetterDeadlineRule{}, false, nil
	case 1:
		return found[0], true, nil
	}
	return domain.CitizenLetterDeadlineRule{}, false,
		errors.New("hạn đơn thư: hai quy tắc còn hiệu lực cho một loại đơn và loại hạn — khoá duy nhất không còn")
}

// GetForUpdate reads one live rule inside the transaction and locks it, so an edit and a removal of
// the same rule cannot both act on the state before the other.
func (s *CitizenLetterDeadlineRuleStore) GetForUpdate(ctx context.Context, tx *store.ScopedTx,
	id string) (domain.CitizenLetterDeadlineRule, error) {

	if id == "" {
		return domain.CitizenLetterDeadlineRule{}, ErrCitizenLetterDeadlineRuleNotFound
	}
	const stmt = `SELECT ` + citizenLetterDeadlineRuleCols + ` FROM citizen_letter_deadline_rule ` +
		`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR UPDATE`
	r, err := scanCitizenLetterDeadlineRule(
		tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), id).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.CitizenLetterDeadlineRule{}, ErrCitizenLetterDeadlineRuleNotFound
	}
	if err != nil {
		return domain.CitizenLetterDeadlineRule{}, fmt.Errorf("hạn đơn thư: đọc quy tắc để ghi: %w", err)
	}
	return r, nil
}

// Insert writes one rule. The id is minted by the caller. No ON CONFLICT: a duplicate arrives as
// ErrCitizenLetterDeadlineRuleExists and the use case decides.
func (s *CitizenLetterDeadlineRuleStore) Insert(ctx context.Context, tx *store.ScopedTx,
	r domain.CitizenLetterDeadlineRule) error {

	const stmt = `INSERT INTO citizen_letter_deadline_rule
			(tenant_id, id, letter_type, deadline_kind, amount, unit)
		VALUES ($1,$2,$3,$4,$5,$6)`
	_, err := tx.Exec(ctx, stmt, string(tx.TenantID()), r.ID, string(r.LetterType), string(r.Kind),
		r.Amount, string(r.Unit))
	if err != nil {
		return fmt.Errorf("hạn đơn thư: chèn quy tắc: %w", translateCitizenLetterRuleWriteError(err))
	}
	return nil
}

// UpdateAmountUnit writes `amount` and `unit` of one live rule — and nothing else.
func (s *CitizenLetterDeadlineRuleStore) UpdateAmountUnit(ctx context.Context, tx *store.ScopedTx,
	r domain.CitizenLetterDeadlineRule) error {

	if r.ID == "" {
		return ErrCitizenLetterDeadlineRuleNotFound
	}
	const stmt = `UPDATE citizen_letter_deadline_rule SET amount = $3, unit = $4, updated_at = now()
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`
	res, err := tx.Exec(ctx, stmt, string(tx.TenantID()), r.ID, r.Amount, string(r.Unit))
	if err != nil {
		return fmt.Errorf("hạn đơn thư: sửa quy tắc: %w", err)
	}
	return exactlyOneCitizenLetterRule(res, "sửa quy tắc")
}

// SoftDelete writes the three columns rule 7 names. `by` is the remover's STAFF CODE (rule 6,
// invariant 8). `deleted_at IS NULL` makes a second removal not-found rather than a rewrite of who
// removed it and why.
func (s *CitizenLetterDeadlineRuleStore) SoftDelete(ctx context.Context, tx *store.ScopedTx,
	id, by, reason string) error {

	if id == "" {
		return ErrCitizenLetterDeadlineRuleNotFound
	}
	const stmt = `UPDATE citizen_letter_deadline_rule
			SET deleted_at = now(), deleted_by = $3, delete_reason = $4, updated_at = now()
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`
	res, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id, by, reason)
	if err != nil {
		return fmt.Errorf("hạn đơn thư: xoá mềm quy tắc: %w", err)
	}
	return exactlyOneCitizenLetterRule(res, "xoá mềm quy tắc")
}

func exactlyOneCitizenLetterRule(res sql.Result, what string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("hạn đơn thư: %s: đếm dòng đã ghi: %w", what, err)
	}
	if n == 0 {
		return ErrCitizenLetterDeadlineRuleNotFound
	}
	return nil
}

// translateCitizenLetterRuleWriteError turns the live-unique violation into a sentinel.
//
// A SUBSTRING MATCH, for the reason dichLoiGhiSLA gives: the table is PARTITION BY HASH, so PostgreSQL
// reports the PARTITION's index (`citizen_letter_deadline_rule_p07_tenant_id_letter_type_deadline_kind_live_key_key`
// — truncated to 63 bytes) rather than the parent's constraint name. Both spellings are matched; an
// unrecognised error is returned unchanged.
func translateCitizenLetterRuleWriteError(err error) error {
	msg := err.Error()
	if strings.Contains(msg, "citizen_letter_deadline_rule_live_unique") ||
		(strings.Contains(msg, "citizen_letter_deadline_rule_p") && strings.Contains(msg, "duplicate key")) {
		return ErrCitizenLetterDeadlineRuleExists
	}
	return err
}
