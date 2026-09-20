package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// HangMucKeHoachVonStore reads the commune's capital plan category catalogue.
//
// It holds *store.DB and never a *sql.DB. The commune is taken from the context on every call
// through db.For(ctx), so there is no constructor, no field and no method here that could produce
// a query without one (rule 1, invariant 5).
type HangMucKeHoachVonStore struct {
	db *store.DB
}

func NewHangMucKeHoachVonStore(db *store.DB) *HangMucKeHoachVonStore {
	return &HangMucKeHoachVonStore{db: db}
}

// TranDanhMucHangMuc is the hard upper bound on one commune's category catalogue.
//
// WHY THERE IS A BOUND AT ALL WHEN THE LIST IS NOT PAGINATED. One process serves 200+ communes, so
// every list route is a shared resource and an unbounded one is forbidden
// (skills/rest-api-design §5, FORBIDDEN #4). This route deliberately returns the WHOLE list — see
// DanhSach — so the bound cannot come from a `limit` parameter; it has to be a ceiling the
// commune's data is checked against.
//
// 200 IS ROUGHLY TWENTY-FIVE TIMES THE REAL FIGURE. These categories come from budget regulation
// and a commune carries a handful of them — the migration that creates the table says as much
// (0003_danh_muc_hang_muc_ke_hoach_von.sql, migration question 1). The number is not an estimate of
// how many there might be; it is the point past which the data is no longer a catalogue: an import
// run twice, a loop that inserted rows, a test fixture on a live database.
const TranDanhMucHangMuc = 200

// ErrQuaNhieuHangMuc says the ceiling was reached. The caller answers 500 and refuses.
//
// WHY REFUSE RATHER THAN TRUNCATE, stated where the cost is paid: this list fills the classifier on
// a capital plan line. A silently short list is a category that has quietly disappeared from that
// picker — the line gets classified under the wrong heading, or under none, and the screen looks
// entirely normal. Those figures are totalled and reported upward. A refusal breaks one commune's
// screen loudly and names itself in the log. Between a wrong answer nobody notices and no answer
// somebody fixes, this system chooses the second (fail closed).
var ErrQuaNhieuHangMuc = errors.New("hang_muc_ke_hoach_von: vượt trần danh mục")

// cotHangMuc IS READ BY POSITION in DanhSach, and this list has two adjacent pairs that swap
// without any error at all:
//
//	ma, nhan                 two TEXT columns — a swap shows slugs where labels belong. Visible.
//	la_mac_dinh, dang_dung   two BOOLEAN columns — a swap makes a row the commune TOOK OUT OF USE
//	                         the one the form pre-selects, and nothing anywhere says so. Invisible.
//
// The second pair is why this constant exists instead of the column list being written inline.
const cotHangMuc = `id, ma, nhan, la_mac_dinh, dang_dung`

// DanhSach reads the commune's whole capital plan category catalogue, ordered.
//
// NOT PAGINATED, AND THAT IS A DECISION, not an omission. The reasoning is the same one
// idstore.BoPhanStore.DanhSach sets out for the org chart, and it holds here for the same reasons:
//
//   - it is a CLOSED REFERENCE LIST of a handful of rows, not a growing register. Its size follows
//     budget regulation, not how long the commune has been using the system;
//   - its consumers need the WHOLE list to be correct at all — a picker showing the first page of
//     categories is a picker missing the category somebody needs, with nothing on the screen
//     saying so;
//   - a client that must follow cursors to fill a dropdown will not, and the one that forgets
//     produces exactly that silent truncation.
//
// The bound pagination would have provided is provided instead by TranDanhMucHangMuc, enforced in
// SQL — one row over the ceiling and this refuses rather than trimming.
//
// ROWS OUT OF USE ARE INCLUDED, SOFT-DELETED ROWS ARE NOT, and the two are different questions.
// `dang_dung = false` is a row the commune took out of use; the catalogue screen still lists it
// with a "Đã tắt" chip, and a plan line from an earlier budget year still holds its `ma`, so
// dropping it here would leave an existing figure with no category name. `deleted_at IS NOT NULL`
// is a row that must not appear on ANY read path — lists, statistics, search, reports, background
// jobs (rule 7, invariant 2). The predicate below filters the second and deliberately not the
// first; domain.HangMucKeHoachVon.DangDung is how the caller tells them apart.
//
// ORDER BY thu_tu, ma: `thu_tu` is the order the commune arranged its own catalogue in, and `ma`
// breaks ties. The tie-break is `ma` rather than `nhan` on purpose — UNIQUE (tenant_id, ma) makes
// the ordering TOTAL, while two rows may perfectly well carry the same label, and a tie left
// unresolved lets two rows swap places between two calls. An unstable order makes a client-side
// diff flicker on every reload and makes any test of this compare sets by accident.
//
// THE COMMUNE IS NOT A PARAMETER AND CANNOT BE ONE: Scoped.Query binds it to $1 from the context,
// so this can only ever read the catalogue of the commune the request arrived in (rule 1).
func (s *HangMucKeHoachVonStore) DanhSach(ctx context.Context) ([]domain.HangMucKeHoachVon, error) {
	// LIMIT is the ceiling PLUS ONE, which is what makes "there are too many" detectable at all.
	// Selecting exactly the ceiling would return a full page indistinguishable from a complete list
	// of exactly that size — the truncation this route refuses to perform, performed by the bound
	// meant to prevent it.
	rows, err := s.db.For(ctx).Query(ctx, cotHangMuc, "hang_muc_ke_hoach_von",
		`AND deleted_at IS NULL ORDER BY thu_tu, ma LIMIT $2`, TranDanhMucHangMuc+1)
	if err != nil {
		return nil, fmt.Errorf("hang_muc_ke_hoach_von: đọc danh mục: %w", err)
	}
	defer rows.Close()

	ra := make([]domain.HangMucKeHoachVon, 0, 16)
	for rows.Next() {
		var hm domain.HangMucKeHoachVon
		// POSITIONAL — in lockstep with cotHangMuc. See the note there on the two adjacent pairs.
		if err := rows.Scan(&hm.ID, &hm.Ma, &hm.Nhan, &hm.LaMacDinh, &hm.DangDung); err != nil {
			return nil, fmt.Errorf("hang_muc_ke_hoach_von: đọc dòng: %w", err)
		}
		ra = append(ra, hm)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("hang_muc_ke_hoach_von: duyệt kết quả: %w", err)
	}
	if len(ra) > TranDanhMucHangMuc {
		// The rows already read are DROPPED rather than trimmed and returned. Handing back a list
		// the caller might render anyway is how a refusal turns back into a silent truncation one
		// careless `if err != nil { log }` later.
		return nil, ErrQuaNhieuHangMuc
	}
	return ra, nil
}
