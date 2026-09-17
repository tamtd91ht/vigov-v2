package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// BoPhanStore reads the commune's organisational chart.
type BoPhanStore struct {
	db *store.DB
}

func NewBoPhanStore(db *store.DB) *BoPhanStore { return &BoPhanStore{db: db} }

// TranDanhMucBoPhan is the hard upper bound on one commune's org chart.
//
// WHY THERE IS A BOUND AT ALL WHEN THE LIST IS NOT PAGINATED. One process serves 200+ communes, so
// every list route is a shared resource (skills/rest-api-design §5) and an unbounded one is
// forbidden. This route deliberately returns the WHOLE list — see DanhSach — which means the bound
// cannot come from a `limit` parameter; it has to be a ceiling the commune's data is checked
// against.
//
// 500 IS ABOUT FIFTY TIMES THE REAL FIGURE. A commune runs roughly ten units. The number is not an
// estimate of how many there might be; it is the point past which the data is no longer an org
// chart — a loop that inserted rows, an import run twice, a test fixture on a live database.
const TranDanhMucBoPhan = 500

// ErrQuaNhieuBoPhan says the ceiling was reached. The caller answers 500 and refuses.
//
// WHY REFUSE RATHER THAN TRUNCATE, stated where the cost is paid: this list feeds the boxes a
// member of staff assigns work in. A silently truncated list is a unit that has quietly disappeared
// from the assignment box — the work goes to the wrong unit, or to nobody, and the screen looks
// completely normal. A refusal breaks the screen loudly for ONE commune and names itself in the
// log. Between a wrong answer nobody notices and no answer somebody fixes, this system chooses the
// second (fail closed).
var ErrQuaNhieuBoPhan = errors.New("bo_phan: vượt trần danh mục")

// cotBoPhan IS READ BY POSITION in DanhSach. `ma` and `ten` are adjacent TEXT columns: swapping
// them here — or there — produces no error at all, and the screen shows slugs where names belong.
const cotBoPhan = `id, ma, ten, coalesce(cha_id,'')`

// DanhSach reads the commune's whole org chart, ordered.
//
// NOT PAGINATED, AND THAT IS A DECISION WITH A REASON, not an omission. Every other list in this
// system is cursor-paginated by default and this one deliberately is not:
//
//   - it is a CLOSED REFERENCE LIST of about ten rows, not a growing register. Its size is a
//     property of the commune's organisation, not of how long the commune has been using the
//     system — unlike petitions or documents, which grow without limit;
//   - its consumers need the WHOLE list to be correct at all. It fills assignment boxes, document
//     routing, the directory and filter dropdowns. A dropdown showing the first page of departments
//     is a dropdown missing the department somebody needs, and nothing on the screen says so;
//   - a client that must follow cursors to fill a dropdown will not, and the one that forgets
//     produces exactly the silent truncation above.
//
// The bound the pagination would have provided is provided instead by TranDanhMucBoPhan, enforced
// in SQL — one row over the ceiling and this refuses rather than trimming.
//
// ORDER BY thu_tu, ten: `thu_tu` is the order the commune arranged its own units in, and `ten`
// breaks ties so two units sharing a rank do not swap places between two calls — an unstable order
// makes a client-side diff flicker on every reload and makes any test of this compare sets by
// accident.
//
// THE COMMUNE IS NOT A PARAMETER AND CANNOT BE ONE: Scoped.Query binds it to $1 from the context,
// so this can only ever read the org chart of the commune the request arrived in (rule 1).
func (s *BoPhanStore) DanhSach(ctx context.Context) ([]domain.BoPhan, error) {
	// LIMIT is the ceiling PLUS ONE, which is what makes "there are too many" detectable at all.
	// Selecting exactly the ceiling would return a full page that is indistinguishable from a
	// complete list of exactly that size — the truncation this route refuses to perform, performed
	// by the bound meant to prevent it.
	rows, err := s.db.For(ctx).Query(ctx, cotBoPhan, "bo_phan",
		`AND deleted_at IS NULL ORDER BY thu_tu, ten LIMIT $2`, TranDanhMucBoPhan+1)
	if err != nil {
		return nil, fmt.Errorf("bo_phan: đọc danh mục: %w", err)
	}
	defer rows.Close()

	ra := make([]domain.BoPhan, 0, 16)
	for rows.Next() {
		var bp domain.BoPhan
		// POSITIONAL — in lockstep with cotBoPhan. See the note there on the two adjacent TEXT
		// columns.
		if err := rows.Scan(&bp.ID, &bp.Ma, &bp.Ten, &bp.ChaID); err != nil {
			return nil, fmt.Errorf("bo_phan: đọc dòng: %w", err)
		}
		ra = append(ra, bp)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("bo_phan: duyệt kết quả: %w", err)
	}
	if len(ra) > TranDanhMucBoPhan {
		// The rows already read are DROPPED rather than trimmed and returned. Handing back a list
		// the caller might render anyway is how a refusal turns back into a silent truncation one
		// careless `if err != nil { log }` later.
		return nil, ErrQuaNhieuBoPhan
	}
	return ra, nil
}
