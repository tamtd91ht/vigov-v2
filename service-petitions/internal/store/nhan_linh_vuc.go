package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// NhanLinhVucStore reads the commune's OVERRIDES for the petition field labels — tier 2 of
// ADR 0026 (`nhan_linh_vuc`, migration 0004).
//
// IT READS ONLY, and the missing write path is a decision recorded elsewhere rather than an
// unfinished job: writing a row means checking that `ma` names a real tier-1 code, and the
// tier-1 set lives in service `platform`. How `petitions` reads it — a live gRPC call or a
// replica fed by events — is an ENGINEERING decision that ADR 0026 §Bổ sung says needs its own
// ADR the day `petitions` first reads the set, and writing one without it is that ADR's stop
// condition #2.
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

const cotNhanLinhVuc = `id, ma, nhan`

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
		var n domain.NhanLinhVuc
		if err := rows.Scan(&n.ID, &n.Ma, &n.Nhan); err != nil {
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
