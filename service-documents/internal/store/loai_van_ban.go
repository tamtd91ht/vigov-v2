package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-documents/internal/domain"
)

// LoaiVanBanStore reads the commune's document-type catalogue.
//
// Built from *store.DB, which only hands out commune-scoped access: there is no constructor here
// taking a raw *sql.DB, because a repository that can be built without a commune is a repository
// that can query across communes (rule 1, invariant 5).
type LoaiVanBanStore struct {
	db *store.DB
}

func NewLoaiVanBanStore(db *store.DB) *LoaiVanBanStore { return &LoaiVanBanStore{db: db} }

// TranDanhMucLoaiVanBan is the hard upper bound on one commune's document-type catalogue.
//
// WHY A BOUND AT ALL WHEN THE LIST IS NOT PAGINATED. One process serves 200+ communes, so every
// list route is a shared resource and an unbounded one is forbidden (skills/rest-api-design §5,
// forbidden #4). This route deliberately returns the WHOLE list — see DanhSach — so the bound
// cannot come from a `limit` parameter; it has to be a ceiling the commune's data is checked
// against.
//
// 200 IS ABOUT THIRTY TIMES THE FIGURE THE SPECIFICATION LISTS. docs/ui-ux/14-cau-hinh.md:169
// names seven shipped codes, and a commune adds a handful of its own at most. The number is not an
// estimate of how many there might be; it is the point past which the rows are no longer a
// catalogue — an import run twice, a loop that inserted rows, a test fixture on a live database.
const TranDanhMucLoaiVanBan = 200

// ErrQuaNhieuLoaiVanBan says the ceiling was reached. The caller answers 500 and refuses.
//
// WHY REFUSE RATHER THAN TRUNCATE, stated where the cost is paid: this list is what a document is
// registered under, and numbering follows the type (ADR 0024). A silently short list is a type
// that has quietly disappeared from the registration form — the document is filed under the wrong
// type, it takes a number from the wrong series, and the screen looks entirely normal. A refusal
// breaks the screen loudly for ONE commune and names itself in the log. Between a wrong answer
// nobody notices and no answer somebody fixes, this system chooses the second (fail closed).
var ErrQuaNhieuLoaiVanBan = errors.New("loai_van_ban: vượt trần danh mục")

// cotLoaiVanBan IS READ BY POSITION in DanhSach. `ma` and `nhan` are adjacent TEXT columns and
// `dang_dung` and `la_mac_dinh` are adjacent BOOLEANs: swapping either pair here — or there —
// produces no error at all, and the screen shows slugs where labels belong, or pre-selects a type
// the commune has taken out of use.
const cotLoaiVanBan = `id, ma, nhan, dang_dung, la_mac_dinh`

// DanhSach reads the commune's whole document-type catalogue, ordered.
//
// NOT PAGINATED, AND THAT IS A DECISION rather than an omission — the same one
// service-identity's BoPhanStore.DanhSach argues for the org chart, and for the same three
// reasons:
//
//   - it is a CLOSED REFERENCE LIST of a handful of rows, not a register that grows with use.
//     Its size is a property of how the commune classifies its paperwork, not of how long the
//     commune has been using the system — unlike `van_ban_den`, which grows without limit;
//   - its consumers need the WHOLE list to be correct at all. It fills the type box on the
//     registration form and the filter on every document list. A dropdown showing the first page
//     of types is a dropdown missing the type somebody needs, and nothing on the screen says so;
//   - a client that must follow cursors to fill a dropdown will not, and the one that forgets
//     produces exactly the silent truncation above.
//
// The bound the pagination would have provided is provided instead by TranDanhMucLoaiVanBan,
// enforced in SQL — one row over the ceiling and this refuses rather than trimming.
//
// `deleted_at IS NULL` IS THE ONLY VISIBILITY FILTER, and `dang_dung` is deliberately NOT one:
// rows the commune has taken out of use are still returned, carrying their flag, because the
// configuration screen lists them with a "Đã tắt" chip and an old document still has to render the
// label of the type it was registered under. Soft-deleted rows drop out — everywhere, always
// (rule 7, invariant 2). The partial index `loai_van_ban_danh_sach` is built on exactly this
// predicate and this order.
//
// ORDER BY thu_tu, nhan: `thu_tu` is the order the commune arranged its own catalogue in, and
// `nhan` breaks ties so two rows sharing a rank do not swap places between two calls — an unstable
// order makes a client-side diff flicker on every reload and makes any test of this compare sets
// by accident.
//
// THE COMMUNE IS NOT A PARAMETER AND CANNOT BE ONE: Scoped.Query binds it to $1 from the context,
// so this can only ever read the catalogue of the commune the request arrived in (rule 1).
func (s *LoaiVanBanStore) DanhSach(ctx context.Context) ([]domain.LoaiVanBan, error) {
	// LIMIT is the ceiling PLUS ONE, which is what makes "there are too many" detectable at all.
	// Selecting exactly the ceiling would return a full page indistinguishable from a complete
	// list of exactly that size — the truncation this route refuses to perform, performed by the
	// bound meant to prevent it.
	rows, err := s.db.For(ctx).Query(ctx, cotLoaiVanBan, "loai_van_ban",
		`AND deleted_at IS NULL ORDER BY thu_tu, nhan LIMIT $2`, TranDanhMucLoaiVanBan+1)
	if err != nil {
		return nil, fmt.Errorf("loai_van_ban: đọc danh mục: %w", err)
	}
	defer rows.Close()

	ra := make([]domain.LoaiVanBan, 0, 16)
	for rows.Next() {
		var lvb domain.LoaiVanBan
		// POSITIONAL — in lockstep with cotLoaiVanBan. See the note there on the two adjacent
		// pairs of same-typed columns.
		if err := rows.Scan(&lvb.ID, &lvb.Ma, &lvb.Nhan, &lvb.DangDung, &lvb.LaMacDinh); err != nil {
			return nil, fmt.Errorf("loai_van_ban: đọc dòng: %w", err)
		}
		ra = append(ra, lvb)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("loai_van_ban: duyệt kết quả: %w", err)
	}
	if len(ra) > TranDanhMucLoaiVanBan {
		// The rows already read are DROPPED rather than trimmed and returned. Handing back a list
		// the caller might render anyway is how a refusal turns back into a silent truncation one
		// careless `if err != nil { log }` later.
		return nil, ErrQuaNhieuLoaiVanBan
	}
	return ra, nil
}
