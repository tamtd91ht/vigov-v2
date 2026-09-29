package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// NhanLinhVucStore reads and writes the commune's OVERRIDES for the petition field catalogue — tier 2
// of ADR 0026 (`nhan_linh_vuc`, migrations 0004 and 0022): label, display order, and the switch that
// hides a code from the citizen's new-submission form.
//
// THE WRITE PATH EXISTS SINCE ADR 0060 settled how tier 1 is read (gRPC to `platform`). This file
// does not check that `ma` is a real tier-1 code — it knows no network; app.PetitionFieldCatalogue
// checks it against the platform's set BEFORE the transaction opens.
//
// WHAT A MISSING ROW MEANS: the commune has not re-worded that code, so the screen shows the
// PLATFORM'S DEFAULT LABEL. A missing row is never "unknown field" — the code is valid whether
// or not anybody re-worded it, and a reader that treated absence as invalidity would blank out
// eleven of the twelve fields for every commune that changed one.
type NhanLinhVucStore struct {
	db *store.DB
}

func NewNhanLinhVucStore(db *store.DB) *NhanLinhVucStore { return &NhanLinhVucStore{db: db} }

// TranNhanLinhVuc is the hard upper bound on one commune's overrides.
//
// WHY A BOUND WHEN THE LIST IS NOT PAGINATED: one process serves 200+ communes, so every list
// read is a shared resource and an unbounded one is forbidden. This route returns the whole set
// by design, so the bound cannot come from a `limit` parameter.
//
// 100 IS ABOUT EIGHT TIMES THE SIZE OF THE CLOSED CODE SET — twelve codes today
// (docs/ui-ux/09 §5), and a commune cannot add a thirteenth (ADR 0026). The number is not a
// guess at how many a commune might want; it is the point past which the rows have stopped
// being overrides: an import run twice, a loop, a fixture on a live database.
const TranNhanLinhVuc = 100

// ErrQuaNhieuNhanLinhVuc says the ceiling was reached. The caller refuses rather than truncating:
// a silently short set of overrides shows the platform's wording on a field the commune has
// deliberately renamed, on a government screen, with nothing to indicate it.
var ErrQuaNhieuNhanLinhVuc = errors.New("nhan_linh_vuc: vượt trần danh mục")

// cotNhanLinhVuc IS READ BY POSITION (scanFieldOverride). `nhan` is NULL when the commune inherits
// the tier-1 label (migration 0022); it is scanned through sql.NullString and surfaces as "".
const cotNhanLinhVuc = `id, ma, nhan, thu_tu, enabled`

func scanFieldOverride(scan func(...any) error) (domain.NhanLinhVuc, error) {
	var n domain.NhanLinhVuc
	var label sql.NullString
	if err := scan(&n.ID, &n.Ma, &label, &n.SortOrder, &n.Enabled); err != nil {
		return domain.NhanLinhVuc{}, err
	}
	n.Nhan = label.String
	return n, nil
}

// DanhSach reads the commune's whole set of overrides, in the commune's own display order.
//
// THE COMMUNE IS NOT A PARAMETER AND CANNOT BE ONE: Scoped.Query binds it to $1 from the
// context, so this can only ever read the overrides of the commune the request arrived in
// (rule 1, invariant 5).
//
// ORDER BY thu_tu, ma — the tie-break is `ma` and not `nhan`, because `ma` carries
// UNIQUE (tenant_id, ma) so the order is TOTAL: two rows can never compare equal and swap places
// between two calls. Labels carry no unique key.
func (s *NhanLinhVucStore) DanhSach(ctx context.Context) ([]domain.NhanLinhVuc, error) {
	// LIMIT is the ceiling PLUS ONE, which is what makes "there are too many" detectable at all.
	// Selecting exactly the ceiling returns a full page indistinguishable from a complete list of
	// that size — the truncation this method refuses to perform, performed by the bound meant to
	// prevent it.
	rows, err := s.db.For(ctx).Query(ctx, cotNhanLinhVuc, "nhan_linh_vuc",
		`AND deleted_at IS NULL ORDER BY thu_tu, ma LIMIT $2`, TranNhanLinhVuc+1)
	if err != nil {
		return nil, fmt.Errorf("nhan_linh_vuc: đọc danh mục: %w", err)
	}
	defer rows.Close()

	ra := make([]domain.NhanLinhVuc, 0, 12)
	for rows.Next() {
		n, err := scanFieldOverride(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("nhan_linh_vuc: đọc dòng: %w", err)
		}
		ra = append(ra, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("nhan_linh_vuc: duyệt kết quả: %w", err)
	}
	if len(ra) > TranNhanLinhVuc {
		// The rows already read are DROPPED rather than trimmed and returned. Handing back a list
		// the caller might render anyway is how a refusal turns back into a silent truncation.
		return nil, ErrQuaNhieuNhanLinhVuc
	}
	return ra, nil
}

// ErrFieldOverrideSoftDeleted says the row this write would overwrite is soft-deleted. REFUSED,
// never revived: UNIQUE (tenant_id, ma) counts soft-deleted rows (migration 0004), so an upsert would
// otherwise bring a deleted row back to life with nothing in the trail saying it was ever deleted.
// No path in this service soft-deletes a row here, so reaching this means a hand edit in the database.
var ErrFieldOverrideSoftDeleted = errors.New("nhan_linh_vuc: dòng ghi đè đã bị xoá mềm — không ghi đè lên")

// ForUpdate reads this commune's override for one code inside the transaction and locks it. ok=false:
// the commune has no row for the code (it inherits everything). A SOFT-DELETED row is read too — so
// the refusal above can happen — and answered with ErrFieldOverrideSoftDeleted.
//
// ⚠ FOR UPDATE LOCKS ONLY A ROW THAT EXISTS: two first-ever writes of one code both see "no row" and
// the second upsert becomes an update, so the table ends correct but the second audit entry's "before"
// is the default rather than the first writer's value. Accepted for display configuration, the same
// trade NhanTrangThaiNhiemVuStore.TheoMaDeSua states.
func (s *NhanLinhVucStore) ForUpdate(ctx context.Context, tx *store.ScopedTx, code string) (
	domain.NhanLinhVuc, bool, error) {

	const stmt = `SELECT ` + cotNhanLinhVuc + `, deleted_at IS NOT NULL FROM nhan_linh_vuc ` +
		`WHERE tenant_id = $1 AND ma = $2 FOR UPDATE`

	var deleted bool
	n, err := scanFieldOverride(func(dest ...any) error {
		return tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), code).
			Scan(append(dest, &deleted)...)
	})
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return domain.NhanLinhVuc{}, false, nil
	case err != nil:
		return domain.NhanLinhVuc{}, false, fmt.Errorf("nhan_linh_vuc: đọc dòng để sửa: %w", err)
	case deleted:
		return domain.NhanLinhVuc{}, false, ErrFieldOverrideSoftDeleted
	}
	return n, true, nil
}

// upsertFieldOverride writes all three override columns every time — they are absolute values, so the
// same request twice leaves one row. `tenant_id` and `ma` appear in no SET clause: a row never moves
// to another commune or another code. The WHERE on the conflict branch is what turns "overwrite a
// soft-deleted row" into zero affected rows instead of a silent revival.
const upsertFieldOverride = `INSERT INTO nhan_linh_vuc (tenant_id, id, ma, nhan, thu_tu, enabled)
	VALUES ($1, $2, $3, $4, $5, $6)
	ON CONFLICT (tenant_id, ma) DO UPDATE
	SET nhan = EXCLUDED.nhan, thu_tu = EXCLUDED.thu_tu, enabled = EXCLUDED.enabled, cap_nhat_luc = now()
	WHERE nhan_linh_vuc.deleted_at IS NULL`

// Upsert stores the commune's override for n.Ma. n.Nhan "" is written as NULL (inherit the tier-1
// label). The commune comes from the transaction, never from n.
func (s *NhanLinhVucStore) Upsert(ctx context.Context, tx *store.ScopedTx, n domain.NhanLinhVuc) error {
	id, err := ulid.Moi()
	if err != nil {
		return fmt.Errorf("nhan_linh_vuc: sinh id: %w", err)
	}
	label := sql.NullString{String: n.Nhan, Valid: n.Nhan != ""}
	res, err := tx.Underlying().ExecContext(ctx, upsertFieldOverride,
		string(tx.TenantID()), id, n.Ma, label, n.SortOrder, n.Enabled)
	if err != nil {
		return fmt.Errorf("nhan_linh_vuc: ghi: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("nhan_linh_vuc: đếm dòng đã ghi: %w", err)
	}
	if affected != 1 {
		return ErrFieldOverrideSoftDeleted
	}
	return nil
}
