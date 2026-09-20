package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// LoaiTaiNguyenBanDoStore reads the commune's map-asset-type catalogue.
//
// It is built from *store.DB and reaches the database only through Scoped — there is no field
// here holding a *sql.DB, and adding one would reopen the hole core/store exists to close
// (rule 1, invariant 5).
type LoaiTaiNguyenBanDoStore struct {
	db *store.DB
}

func NewLoaiTaiNguyenBanDoStore(db *store.DB) *LoaiTaiNguyenBanDoStore {
	return &LoaiTaiNguyenBanDoStore{db: db}
}

// TranDanhMucLoaiTaiNguyen is the hard upper bound on one commune's map-asset-type catalogue.
//
// WHY A BOUND AT ALL WHEN THE LIST IS NOT PAGINATED. One process serves 200+ communes, so every
// list route is a shared resource and an unbounded one is forbidden (skills/rest-api-design §5,
// FORBIDDEN #4). This route returns the WHOLE list on purpose — see DanhSach — so the bound cannot
// come from a `limit` parameter; it has to be a ceiling the commune's data is checked against.
//
// THE SAME FIGURE AS identity's TranDanhMucBoPhan, DELIBERATELY. The real catalogue is about a
// dozen rows (migration 0003, question 1: "around a dozen rows per commune"), so 500 is roughly
// forty times it — the point past which the content is no longer a catalogue but a loop that
// inserted rows, an import run twice, or a test fixture on a live database. Two ceilings in one
// system carrying two different numbers would invite the next reader to look for a meaning in the
// difference that is not there.
const TranDanhMucLoaiTaiNguyen = 500

// ErrQuaNhieuLoaiTaiNguyen says the ceiling was reached. The caller answers 500 and refuses.
//
// WHY REFUSE RATHER THAN TRUNCATE, stated where the cost is paid: this list fills the group
// selector on the economic map and the filter beside it. A silently short list is a group that has
// quietly disappeared from the selector — assets get filed under the wrong group, or cannot be
// filed at all, and the screen looks entirely normal. A refusal breaks the screen loudly for ONE
// commune and names itself in the log. Between a wrong answer nobody notices and no answer
// somebody fixes, this system chooses the second (fail closed).
var ErrQuaNhieuLoaiTaiNguyen = errors.New("loai_tai_nguyen_ban_do: vượt trần danh mục")

// cotLoaiTaiNguyen IS READ BY POSITION in DanhSach. `ma` and `nhan` are adjacent TEXT columns, and
// `la_mac_dinh` and `dang_dung` are adjacent BOOLEANs: swapping either pair here — or there —
// produces no error at all. The first swap shows slugs where labels belong; the second opens the
// map on a group that was taken out of use.
const cotLoaiTaiNguyen = `id, ma, nhan, thu_tu, la_mac_dinh, dang_dung`

// DanhSach reads the commune's whole map-asset-type catalogue, ordered.
//
// NOT PAGINATED, AND THAT IS A DECISION, not an omission. The same three reasons hold as for
// identity's org chart, and the migration's own figures back the first one:
//
//   - it is a CLOSED REFERENCE LIST of about a dozen rows, not a register that grows with use;
//   - its consumers need the WHOLE list to be correct at all — a selector showing the first page of
//     groups is a selector missing the group somebody needs, with nothing on the screen saying so;
//   - a client that must follow cursors to fill a dropdown will not, and the one that forgets
//     produces exactly that silent truncation.
//
// The bound pagination would have provided is provided instead by TranDanhMucLoaiTaiNguyen.
//
// `deleted_at IS NULL` AND NOTHING ELSE IN THE PREDICATE. Rows with `dang_dung = false` are
// deliberately returned: the catalogue screen shows them with a "Đã tắt" chip, and an asset already
// filed under a retired group still has to render that group's name. Only soft-deleted rows drop
// out, and they drop out here — everywhere, always (rule 7, invariant 2).
//
// ORDER BY thu_tu, ma: `thu_tu` is the order the commune arranged its own groups in, and `ma`
// breaks ties. The tie-break is `ma` rather than the label because UNIQUE (tenant_id, ma) makes it
// a TOTAL order — two rows can share a label, and an order that is not total lets two calls return
// the same rows in a different sequence, which makes a client-side diff flicker and makes any test
// of this compare sets by accident.
//
// THE COMMUNE IS NOT A PARAMETER AND CANNOT BE ONE: Scoped.Query binds it to $1 from the context,
// so this can only ever read the catalogue of the commune the request arrived in (rule 1,
// invariants 4 and 5).
//
// AN EMPTY RESULT IS THE EXPECTED ANSWER TODAY, not a fault to work around. The table ships empty
// for every commune: the shipped code list is contradicted by its own specification (11 groups at
// docs/ui-ux/10-ban-do-kinh-te-so.md:37 versus 8 at :53, in two different spellings), and the step
// that sows a commune's system rows does not exist. Nothing here invents a row to fill the gap.
func (s *LoaiTaiNguyenBanDoStore) DanhSach(ctx context.Context) ([]domain.LoaiTaiNguyenBanDo, error) {
	// LIMIT is the ceiling PLUS ONE, which is what makes "there are too many" detectable at all.
	// Selecting exactly the ceiling would return a full page indistinguishable from a complete list
	// of exactly that size — the truncation this route refuses to perform, performed by the bound
	// meant to prevent it.
	rows, err := s.db.For(ctx).Query(ctx, cotLoaiTaiNguyen, "loai_tai_nguyen_ban_do",
		`AND deleted_at IS NULL ORDER BY thu_tu, ma LIMIT $2`, TranDanhMucLoaiTaiNguyen+1)
	if err != nil {
		return nil, fmt.Errorf("loai_tai_nguyen_ban_do: đọc danh mục: %w", err)
	}
	defer rows.Close()

	ra := make([]domain.LoaiTaiNguyenBanDo, 0, 16)
	for rows.Next() {
		var lt domain.LoaiTaiNguyenBanDo
		// POSITIONAL — in lockstep with cotLoaiTaiNguyen. See the note there on the two adjacent
		// pairs of same-typed columns.
		if err := rows.Scan(&lt.ID, &lt.Ma, &lt.Nhan, &lt.ThuTu, &lt.LaMacDinh, &lt.DangDung); err != nil {
			return nil, fmt.Errorf("loai_tai_nguyen_ban_do: đọc dòng: %w", err)
		}
		ra = append(ra, lt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("loai_tai_nguyen_ban_do: duyệt kết quả: %w", err)
	}
	if len(ra) > TranDanhMucLoaiTaiNguyen {
		// The rows already read are DROPPED rather than trimmed and returned. Handing back a list the
		// caller might render anyway is how a refusal turns back into a silent truncation, one
		// careless `if err != nil { log }` later.
		return nil, ErrQuaNhieuLoaiTaiNguyen
	}
	return ra, nil
}
