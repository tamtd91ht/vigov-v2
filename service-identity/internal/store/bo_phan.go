package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// BoPhanStore reads the commune's organisational chart. Its write paths are in bo_phan_ghi.go.
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

// truyVanBoPhanKemSoCanBo reads the commune's whole org chart WITH the staff count of each unit.
//
// ONE STATEMENT, ONE ROUND TRIP (skills/load-data-once) — the shape truyVanVaiTroKemSoCanBo in
// quyen.go already uses for roles, and every trap it names applies here:
//
//   - count(nd.id) AND NOT count(*): this is a LEFT JOIN, so a unit nobody sits in still produces
//     one row with every `nd.*` NULL. count(*) would print "1 cán bộ" against every empty unit.
//   - THE STAFF CONDITIONS SIT IN THE JOIN, NOT IN THE WHERE. In the WHERE they would drop the UNIT
//     whenever it has no live member — an empty unit would vanish from the org chart and from every
//     assignment box, which is the silent truncation this route refuses everywhere else.
//   - THE JOIN REPEATS THE COMMUNE (`nd.tenant_id = bp.tenant_id`), not merely the unit id. Joining
//     on id alone would count another commune's staff wherever ids collide (rule 1) — a leak no
//     single-commune test shows.
//
// WHO IS COUNTED is argued on domain.BoPhan.SoCanBo: not soft-deleted, not locked; having a sign-in
// account is not a condition. Nothing about a person is selected, so nothing personal can reach the
// response (rule 3).
//
// COLUMNS ARE READ BY POSITION in DanhSach. `ma` and `ten` are adjacent TEXT columns, `thu_tu` and
// the count adjacent integers: swapping either pair produces no error at all.
const truyVanBoPhanKemSoCanBo = `
SELECT bp.id, bp.ma, bp.ten, coalesce(bp.cha_id,''), bp.thu_tu,
       count(nd.id) AS so_can_bo
FROM bo_phan bp
LEFT JOIN nguoi_dung nd
       ON nd.tenant_id  = bp.tenant_id
      AND nd.bo_phan_id = bp.id
      AND nd.deleted_at IS NULL
      AND nd.dang_hoat_dong
WHERE bp.tenant_id = $1
  AND bp.deleted_at IS NULL
GROUP BY bp.id, bp.ma, bp.ten, bp.cha_id, bp.thu_tu
ORDER BY bp.thu_tu, bp.ten
LIMIT $2`

// DanhSach reads the commune's whole org chart, ordered, with each unit's staff count.
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
// THE COMMUNE IS NOT A PARAMETER AND CANNOT BE ONE: QueryJoin binds it to $1 from the context, so
// this can only ever read the org chart of the commune the request arrived in (rule 1).
func (s *BoPhanStore) DanhSach(ctx context.Context) ([]domain.BoPhan, error) {
	// LIMIT is the ceiling PLUS ONE, which is what makes "there are too many" detectable at all.
	// Selecting exactly the ceiling would return a full page that is indistinguishable from a
	// complete list of exactly that size — the truncation this route refuses to perform, performed
	// by the bound meant to prevent it.
	rows, err := s.db.For(ctx).QueryJoin(ctx, truyVanBoPhanKemSoCanBo, TranDanhMucBoPhan+1)
	if err != nil {
		return nil, fmt.Errorf("bo_phan: đọc danh mục: %w", err)
	}
	defer rows.Close()

	ra := make([]domain.BoPhan, 0, 16)
	for rows.Next() {
		var bp domain.BoPhan
		// POSITIONAL — in lockstep with truyVanBoPhanKemSoCanBo.
		if err := rows.Scan(&bp.ID, &bp.Ma, &bp.Ten, &bp.ChaID, &bp.ThuTu, &bp.SoCanBo); err != nil {
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
