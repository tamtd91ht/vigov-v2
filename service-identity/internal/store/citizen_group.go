package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/textfold"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// CitizenGroupStore reads and writes `citizen_group` (migration 0033) — the named groups a commune
// defines over its citizen contact directory (ADR 0097 D4). A plain table of a few dozen rows per
// commune (ADR 0096 #3). The same four properties as CitizenDirectoryStore.
//
// NAME UNIQUENESS IS ON THE FOLDED NAME, live rows only (`citizen_group_name_live_unique`): "Hộ
// nghèo" and "Ho ngheo" are one group. The fold is computed by the DATABASE with the expression
// core/textfold renders — never by Go (textfold package comment) — so NameTaken and IDsByNames take
// the name AS TYPED and fold it in SQL.
type CitizenGroupStore struct {
	db *store.DB
}

func NewCitizenGroupStore(db *store.DB) *CitizenGroupStore {
	return &CitizenGroupStore{db: db}
}

// MaxCitizenGroups bounds the group list. A commune defines a few dozen groups; 500 is the point past
// which the rows have stopped being a catalogue. Refused rather than truncated: the list fills the
// group picker on every contact, and a short picker is a group nobody can be put in.
const MaxCitizenGroups = 500

var (
	// ErrCitizenGroupNotFound — no live group of THIS commune has this id (one answer for invented,
	// removed and other-commune ids — rule 4, forbidden #2), or a membership write named one.
	ErrCitizenGroupNotFound = errors.New("danh bạ người dân: không tìm thấy nhóm")

	// ErrCitizenGroupNameTaken — the live-unique key refused a second live group with this folded
	// name.
	ErrCitizenGroupNameTaken = errors.New("danh bạ người dân: xã đã có nhóm mang tên này")

	// ErrTooManyCitizenGroups — the list ceiling was reached.
	ErrTooManyCitizenGroups = errors.New("danh bạ người dân: vượt trần số nhóm")
)

// citizenGroupCols is read BY POSITION in scanCitizenGroup. Live reads only, so the soft-delete
// columns are not selected.
const citizenGroupCols = `id, name, coalesce(description, ''), sort_order, created_at, updated_at`

func scanCitizenGroup(scan func(...any) error) (domain.CitizenGroup, error) {
	var g domain.CitizenGroup
	if err := scan(&g.ID, &g.Name, &g.Description, &g.SortOrder, &g.CreatedAt, &g.UpdatedAt); err != nil {
		return domain.CitizenGroup{}, err
	}
	return g, nil
}

// List reads this commune's live groups in the commune's own order: sort_order, then the folded name,
// then id — total, so two groups sharing a rank and a name never swap places between two calls
// (index citizen_group_list serves the first two). Ceiling PLUS ONE, refused rather than truncated.
func (s *CitizenGroupStore) List(ctx context.Context) ([]domain.CitizenGroup, error) {
	rows, err := s.db.For(ctx).Query(ctx, citizenGroupCols, "citizen_group",
		`AND deleted_at IS NULL ORDER BY sort_order, name_folded, id LIMIT $2`, MaxCitizenGroups+1)
	if err != nil {
		return nil, fmt.Errorf("danh bạ người dân: đọc danh sách nhóm: %w", err)
	}
	defer rows.Close()
	out := make([]domain.CitizenGroup, 0, 16)
	for rows.Next() {
		g, err := scanCitizenGroup(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("danh bạ người dân: đọc dòng nhóm: %w", err)
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("danh bạ người dân: duyệt danh sách nhóm: %w", err)
	}
	if len(out) > MaxCitizenGroups {
		return nil, ErrTooManyCitizenGroups
	}
	return out, nil
}

// Get reads one LIVE group of this commune. found == false for an invented id, a removed row and
// another commune's row alike.
func (s *CitizenGroupStore) Get(ctx context.Context, id string) (domain.CitizenGroup, bool, error) {
	if id == "" {
		return domain.CitizenGroup{}, false, nil
	}
	rows, err := s.db.For(ctx).Query(ctx, citizenGroupCols, "citizen_group", `AND id = $2 AND deleted_at IS NULL`, id)
	if err != nil {
		return domain.CitizenGroup{}, false, fmt.Errorf("danh bạ người dân: đọc nhóm: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.CitizenGroup{}, false, fmt.Errorf("danh bạ người dân: đọc nhóm: %w", err)
		}
		return domain.CitizenGroup{}, false, nil
	}
	g, err := scanCitizenGroup(rows.Scan)
	if err != nil {
		return domain.CitizenGroup{}, false, fmt.Errorf("danh bạ người dân: đọc dòng nhóm: %w", err)
	}
	return g, true, rows.Err()
}

// NameTaken reports whether a LIVE group of this commune other than `exceptID` has the same FOLDED
// name as `name` (as typed — the fold is SQL's). Inside the transaction, so the check and the write
// see the same rows; the live-unique key is the backstop (ErrCitizenGroupNameTaken).
func (s *CitizenGroupStore) NameTaken(ctx context.Context, tx *store.ScopedTx, name, exceptID string) (bool, error) {
	if tx == nil {
		return false, ErrThieuGiaoDich
	}
	if strings.TrimSpace(name) == "" {
		return false, nil
	}
	// ScopedTx.Query binds tenant_id = $1 from the transaction's commune (rule 1, invariant 5).
	rows, err := tx.Query(ctx, `1`, "citizen_group",
		`AND deleted_at IS NULL AND id <> $3 AND name_folded = `+textfold.SQL("$2")+` LIMIT 1`, name, exceptID)
	if err != nil {
		return false, fmt.Errorf("danh bạ người dân: kiểm tên nhóm đã có: %w", err)
	}
	defer rows.Close()
	taken := rows.Next()
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("danh bạ người dân: kiểm tên nhóm đã có: %w", err)
	}
	return taken, nil
}

// IDsByNames resolves group names AS TYPED (the Excel "Nhóm" cell) to the ids of this commune's live
// groups, in ONE statement, keyed by the name exactly as it was asked. A name with no live group is
// absent — the caller names the row and the column, never the cell (domain import rules).
//
// The asked names are unnested and each is folded by SQL (textfold) against `name_folded`, so the
// match is the same accent-insensitive one the unique key enforces.
func (s *CitizenGroupStore) IDsByNames(ctx context.Context, names []string) (map[string]string, error) {
	out := map[string]string{}
	ask := make([]string, 0, len(names))
	seen := make(map[string]bool, len(names))
	for _, n := range names {
		if strings.TrimSpace(n) == "" || seen[n] {
			continue
		}
		seen[n] = true
		ask = append(ask, n)
	}
	if len(ask) == 0 {
		return out, nil
	}
	stmt := `SELECT u.name, g.id FROM unnest($2::text[]) AS u(name)` +
		` JOIN citizen_group g ON g.tenant_id = $1 AND g.deleted_at IS NULL` +
		` AND g.name_folded = ` + textfold.SQL("u.name")
	rows, err := s.db.For(ctx).QueryJoin(ctx, stmt, ask)
	if err != nil {
		return nil, fmt.Errorf("danh bạ người dân: tra nhóm theo tên: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name, id string
		if err := rows.Scan(&name, &id); err != nil {
			return nil, fmt.Errorf("danh bạ người dân: đọc dòng tra nhóm: %w", err)
		}
		out[name] = id
	}
	// Checked: a truncated answer refuses a real group as "chưa lập".
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("danh bạ người dân: duyệt tra nhóm: %w", err)
	}
	return out, nil
}

// Insert writes one group. The id and the instant (CreatedAt) are the caller's; UpdatedAt =
// CreatedAt. No ON CONFLICT: a duplicate name arrives as ErrCitizenGroupNameTaken.
func (s *CitizenGroupStore) Insert(ctx context.Context, tx *store.ScopedTx, g domain.CitizenGroup) error {
	if tx == nil {
		return ErrThieuGiaoDich
	}
	if g.ID == "" {
		return ErrCitizenGroupNotFound
	}
	if g.CreatedAt.IsZero() {
		return ErrCitizenDirectoryInstantMissing
	}
	const stmt = `INSERT INTO citizen_group (tenant_id, id, name, description, sort_order, created_at, updated_at)
		VALUES ($1, $2, $3, nullif($4, ''), $5, $6, $6)`
	_, err := tx.Exec(ctx, stmt, string(tx.TenantID()), g.ID, g.Name, g.Description, g.SortOrder, g.CreatedAt)
	if err != nil {
		return fmt.Errorf("danh bạ người dân: chèn nhóm: %w", translateCitizenDirectoryWriteError(err))
	}
	return nil
}

// Update rewrites name, description and sort_order of one LIVE group, and `updated_at` from
// g.UpdatedAt.
func (s *CitizenGroupStore) Update(ctx context.Context, tx *store.ScopedTx, g domain.CitizenGroup) error {
	if tx == nil {
		return ErrThieuGiaoDich
	}
	if g.ID == "" {
		return ErrCitizenGroupNotFound
	}
	if g.UpdatedAt.IsZero() {
		return ErrCitizenDirectoryInstantMissing
	}
	const stmt = `UPDATE citizen_group
			SET name = $3, description = nullif($4, ''), sort_order = $5, updated_at = $6
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`
	res, err := tx.Exec(ctx, stmt, string(tx.TenantID()), g.ID, g.Name, g.Description, g.SortOrder, g.UpdatedAt)
	if err != nil {
		return fmt.Errorf("danh bạ người dân: sửa nhóm: %w", translateCitizenDirectoryWriteError(err))
	}
	return exactlyOneCitizenGroup(res, "sửa nhóm")
}

// SoftDelete writes the soft-delete columns on one LIVE group. `by` is the remover's STAFF CODE;
// `reason` is OPTIONAL for a group (ADR 0097 D7 — removing a group only tidies a catalogue): "" is
// stored as NULL, which the 0033 CHECK admits.
//
// MEMBERSHIPS ARE LEFT AS THEY ARE, for the reason SoftDelete on a contact gives; every read of them
// requires the group to be live (GroupIDsOf), so a removed group names nobody.
func (s *CitizenGroupStore) SoftDelete(ctx context.Context, tx *store.ScopedTx, id, by, reason string, at time.Time) error {
	if tx == nil {
		return ErrThieuGiaoDich
	}
	if id == "" {
		return ErrCitizenGroupNotFound
	}
	if strings.TrimSpace(by) == "" {
		return ErrCitizenDirectoryActorMissing
	}
	if at.IsZero() {
		return ErrCitizenDirectoryInstantMissing
	}
	const stmt = `UPDATE citizen_group
			SET deleted_at = $3, deleted_by = $4, delete_reason = nullif($5, ''), updated_at = $3
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`
	res, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id, at, by, strings.TrimSpace(reason))
	if err != nil {
		return fmt.Errorf("danh bạ người dân: xoá mềm nhóm: %w", err)
	}
	return exactlyOneCitizenGroup(res, "xoá mềm nhóm")
}

func exactlyOneCitizenGroup(res sql.Result, what string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("danh bạ người dân: %s: đếm dòng đã ghi: %w", what, err)
	}
	if n == 0 {
		return ErrCitizenGroupNotFound
	}
	return nil
}
