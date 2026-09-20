package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// LoaiNhiemVuStore reads the commune's task-type catalogue (`loai_nhiem_vu`, migration 0003).
//
// IT READS ONLY. Creating, relabelling, disabling and soft-deleting a catalogue row are the
// configuration screen's operations, and open question #21 — may a commune edit the CODE LIST of a
// task catalogue, or only the labels and the order — is still OPEN. A half-written write path here
// would look like somebody had answered it.
type LoaiNhiemVuStore struct {
	db *store.DB
}

// NewLoaiNhiemVuStore takes *store.DB and nothing else. There is deliberately no constructor from a
// raw *sql.DB: a repository that can be built without a commune is a repository that can query
// across communes (rule 1, invariant 5).
func NewLoaiNhiemVuStore(db *store.DB) *LoaiNhiemVuStore { return &LoaiNhiemVuStore{db: db} }

// TranDanhMucLoaiNhiemVu is the hard upper bound on one commune's task-type catalogue.
//
// WHY A BOUND AT ALL WHEN THE LIST IS NOT PAGINATED: one process serves 200+ communes, so every
// list route is a shared resource and an unbounded one is forbidden. This route returns the WHOLE
// list by design — see DanhSach — so the bound cannot come from a `limit` parameter; it has to be a
// ceiling the commune's own data is measured against.
//
// 100 IS ABOUT FIFTY TIMES THE FIGURE IN THE SPECIFICATION, which lists two types
// (docs/ui-ux/14-cau-hinh.md §5). The number is not an estimate of how many types a commune might
// legitimately want; it is the point past which the rows have stopped being a catalogue — an import
// run twice, a loop that inserted rows, a test fixture on a live database.
const TranDanhMucLoaiNhiemVu = 100

// ErrQuaNhieuLoaiNhiemVu says the ceiling was reached. The caller answers 500 and refuses.
//
// WHY REFUSE RATHER THAN TRUNCATE, stated where the cost is paid: this list fills the type picker
// on the task form and the type filter on every task list. A silently short list is a type that has
// quietly disappeared from the picker — the task is filed under the wrong type, or the filter hides
// tasks that do exist, and every screen looks entirely normal. A refusal breaks ONE commune's
// screen loudly and names itself in the log. Between a wrong answer nobody notices and no answer
// somebody fixes, this system chooses the second (fail closed).
var ErrQuaNhieuLoaiNhiemVu = errors.New("loai_nhiem_vu: vượt trần danh mục")

// cotLoaiNhiemVu IS READ BY POSITION in DanhSach. `ma` and `nhan` are adjacent TEXT columns and
// `la_mac_dinh` / `dang_dung` adjacent BOOLEANs: swapping either pair — here or in the Scan —
// produces no error at all. The first pair shows slugs where names belong; the second pre-selects a
// disabled row on a form.
const cotLoaiNhiemVu = `id, ma, nhan, la_mac_dinh, dang_dung`

// DanhSach reads the commune's whole task-type catalogue, in the commune's own order.
//
// NOT PAGINATED, AND THAT IS A DECISION WITH A REASON, not an omission — the same one
// service-identity states on its org-chart and role catalogues:
//
//   - it is a CLOSED REFERENCE LIST of a handful of rows, not a register that grows with use;
//   - its consumers need the WHOLE list to be correct at all. A picker showing the first page of a
//     catalogue is a picker missing the entry somebody needs, and nothing on the screen says so;
//   - a client that must follow cursors to fill a dropdown will not, and the one that forgets
//     produces exactly that silent truncation.
//
// The bound pagination would have given is TranDanhMucLoaiNhiemVu, enforced in SQL.
//
// ORDER BY thu_tu, ma: `thu_tu` is the order the commune arranged its own catalogue in. The
// tie-break is `ma` and NOT `nhan`, which is where this deviates from service-identity's
// catalogues: `ma` carries UNIQUE (tenant_id, ma) in migration 0003, so the order is TOTAL —
// two rows can never compare equal and swap places between two calls. Labels carry no unique key
// and two rows may legitimately share one.
//
// ROWS OUT OF USE ARE RETURNED, soft-deleted rows are NOT (rule 7, invariant 2) — the split is
// argued on domain.LoaiNhiemVu.DangDung, and the partial index
// `loai_nhiem_vu_danh_sach (tenant_id, thu_tu) WHERE deleted_at IS NULL` exists for exactly this
// query.
//
// THE COMMUNE IS NOT A PARAMETER AND CANNOT BE ONE: Scoped.Query binds it to $1 from the context,
// so this can only ever read the catalogue of the commune the request arrived in (rule 1,
// invariant 4).
func (s *LoaiNhiemVuStore) DanhSach(ctx context.Context) ([]domain.LoaiNhiemVu, error) {
	// LIMIT is the ceiling PLUS ONE, which is what makes "there are too many" detectable at all.
	// Selecting exactly the ceiling returns a full page indistinguishable from a complete list of
	// that size — the truncation this route refuses to perform, performed by the bound meant to
	// prevent it.
	rows, err := s.db.For(ctx).Query(ctx, cotLoaiNhiemVu, "loai_nhiem_vu",
		`AND deleted_at IS NULL ORDER BY thu_tu, ma LIMIT $2`, TranDanhMucLoaiNhiemVu+1)
	if err != nil {
		return nil, fmt.Errorf("loai_nhiem_vu: đọc danh mục: %w", err)
	}
	defer rows.Close()

	ra := make([]domain.LoaiNhiemVu, 0, 8)
	for rows.Next() {
		var l domain.LoaiNhiemVu
		// POSITIONAL — in lockstep with cotLoaiNhiemVu. See the note there on the adjacent pairs.
		if err := rows.Scan(&l.ID, &l.Ma, &l.Nhan, &l.LaMacDinh, &l.DangDung); err != nil {
			return nil, fmt.Errorf("loai_nhiem_vu: đọc dòng: %w", err)
		}
		ra = append(ra, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("loai_nhiem_vu: duyệt kết quả: %w", err)
	}
	if len(ra) > TranDanhMucLoaiNhiemVu {
		// The rows already read are DROPPED rather than trimmed and returned. Handing back a list
		// the caller might render anyway is how a refusal turns back into a silent truncation, one
		// careless `if err != nil { log }` later.
		return nil, ErrQuaNhieuLoaiNhiemVu
	}
	return ra, nil
}
