package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// ThonToDanPhoStore reads the commune's residential units. GET /api/v1/residential-units
//
// NOT A CATALOGUE STORE, and it deliberately does not go through docDanhMuc: this table holds data
// of its own (the two counts), has no `thu_tu`, none of the tier columns, and needs a join the
// catalogues do not. Migration 0005:363 draws the same line.
//
// NO WRITE PATH: open question #21 is unanswered, and a half-written write path looks like a
// decision somebody made.
type ThonToDanPhoStore struct {
	db *store.DB
}

func NewThonToDanPhoStore(db *store.DB) *ThonToDanPhoStore { return &ThonToDanPhoStore{db: db} }

// TranDanhSachThonToDanPho is the hard upper bound on one commune's residential units.
//
// Same argument as TranDanhMucBoPhan — an unbounded list route on a process serving 200+ communes
// is forbidden, and this route returns the WHOLE list, so the bound has to be a ceiling rather than
// a `limit`.
//
// 500 IS ABOUT TWENTY TIMES THE REAL FIGURE. Migration 0005:21 puts a commune at "a few dozen"
// residential units, and the 2025 mergers made communes larger rather than smaller. The number is
// the point past which the data has stopped being a list of thôn — an import run twice, a fixture
// on a live database — not an estimate of how many a commune might legitimately have.
//
// IT IS HIGHER THAN THE TWO CATALOGUE CEILINGS ON PURPOSE: this is a register of real places on the
// ground, not a closed list of classifications, so the honest bound is a different one.
const TranDanhSachThonToDanPho = 500

// ErrQuaNhieuThonToDanPho says the ceiling was reached. The caller answers 500 and refuses.
//
// REFUSE RATHER THAN TRUNCATE. This list fills the address picker on a petition, the "which thôn"
// field on a household record and every filter that groups the commune's work by area. A silently
// short list is a hamlet that has disappeared from the picker: the petition is filed against the
// wrong place or against none, and every screen looks entirely normal. A refusal breaks one
// commune's screen loudly and names itself in the log.
var ErrQuaNhieuThonToDanPho = errors.New("thon_to_dan_pho: vượt trần danh sách")

// truyVanThonToDanPho reads one commune's residential units with the LABEL of each unit's type.
//
// A LEFT JOIN, AND EACH OF ITS THREE CLAUSES IS LOAD-BEARING:
//
//	LEFT and not INNER     `loai` is NULLABLE — the specification's own column list shows `—` as a
//	                       legitimate value (migration 0005:376). An inner join would silently drop
//	                       every unit whose classification has not been entered yet, which is a
//	                       hamlet missing from the commune's own list with nothing on the screen to
//	                       say so — the same failure the ceiling above refuses to cause.
//	l.tenant_id = tt.tenant_id
//	                       THE JOIN IS CONSTRAINED TO THE SAME COMMUNE, not merely to the matching
//	                       code. Joining on `ma` alone would label this commune's unit with ANOTHER
//	                       commune's type wherever codes collide — and `thon` collides in every
//	                       commune in the country. That is the leak rule 1 exists to prevent, and no
//	                       single-commune test would ever show it. Same discipline as checker.go and
//	                       vai_tro.go.
//	l.deleted_at IS NULL   IN THE JOIN, NOT IN THE WHERE. In the WHERE it would filter out the UNIT
//	                       along with the deleted type, turning untidy catalogue data into "this
//	                       hamlet does not exist". In the join it leaves the unit and empties the
//	                       label — the caller still gets `loai`, and the screen shows a raw code
//	                       rather than losing a row. Same reading as vai_tro.go's left join.
//
// `l.dang_dung` IS DELIBERATELY NOT IN THE JOIN. A type taken out of use still names what the units
// already classified under it ARE; blanking their label because nobody may pick that type again
// would rewrite history to match today's configuration.
//
// ONE QUERY, NOT ONE LOOKUP PER ROW. The type of each unit is read in the same statement — the N+1
// skills/load-data-once exists to prevent. The catalogue is two or three rows, so the join costs
// nothing; a loop over units would cost one round trip each and would be invisible in every test.
//
// ORDER BY tt.ten, tt.ma — by name, because that is what the commune's own list screen shows and
// what migration 0005:415 indexed (tenant_id, ten). `ma` breaks ties and is UNIQUE per commune
// (migration 0005:401), which makes the order TOTAL: two hamlets sharing a name can never swap
// places between two calls. There is no `thu_tu` on this table to sort by.
const truyVanThonToDanPho = `
SELECT tt.id, tt.ma, tt.ten, coalesce(tt.loai, ''), coalesce(l.nhan, ''),
       tt.so_ho, tt.nhan_khau, tt.dang_dung
FROM thon_to_dan_pho tt
LEFT JOIN loai_don_vi_dan_cu l
       ON l.tenant_id  = tt.tenant_id
      AND l.ma         = tt.loai
      AND l.deleted_at IS NULL
WHERE tt.tenant_id = $1
  AND tt.deleted_at IS NULL
ORDER BY tt.ten, tt.ma
LIMIT $2`

// DanhSach reads the commune's whole list of residential units, ordered, with each unit's type
// label.
//
// NOT PAGINATED, same decision and same reasons as BoPhanStore.DanhSach: the consumers need the
// WHOLE list to be correct at all — an address picker showing the first page of hamlets is an
// address picker missing the one somebody needs — and the bound pagination would have given is
// TranDanhSachThonToDanPho, enforced in SQL.
//
// THE COMMUNE IS NOT A PARAMETER AND CANNOT BE ONE: Scoped.QueryJoin binds it to $1 from the
// context (rule 1, invariant 5), so this can only ever read the units of the commune the request
// arrived in.
func (s *ThonToDanPhoStore) DanhSach(ctx context.Context) ([]domain.ThonToDanPho, error) {
	// Ceiling PLUS ONE — a full page of exactly the ceiling is indistinguishable from a complete
	// list of that size, which is the truncation this refuses to perform.
	rows, err := s.db.For(ctx).QueryJoin(ctx, truyVanThonToDanPho, TranDanhSachThonToDanPho+1)
	if err != nil {
		return nil, fmt.Errorf("thon_to_dan_pho: đọc danh sách: %w", err)
	}
	defer rows.Close()

	ra := make([]domain.ThonToDanPho, 0, 32)
	for rows.Next() {
		var tt domain.ThonToDanPho
		// NULLABLE TARGETS FOR THE TWO COUNTS, and that is the whole reason they are pointers in
		// the domain type: a plain int here would turn "not entered" into a counted zero at the
		// first read, and a zero travels onward into a report as a number.
		var soHo, nhanKhau sql.NullInt64
		// POSITIONAL — in lockstep with truyVanThonToDanPho. `ma` / `ten` / `loai` / the type label
		// are four adjacent TEXT columns: any swap among them produces no error at all and puts
		// slugs where names belong on the commune's own screen.
		if err := rows.Scan(&tt.ID, &tt.Ma, &tt.Ten, &tt.LoaiMa, &tt.LoaiNhan,
			&soHo, &nhanKhau, &tt.DangDung); err != nil {
			return nil, fmt.Errorf("thon_to_dan_pho: đọc dòng: %w", err)
		}
		if soHo.Valid {
			n := int(soHo.Int64)
			tt.SoHo = &n
		}
		if nhanKhau.Valid {
			n := int(nhanKhau.Int64)
			tt.NhanKhau = &n
		}
		ra = append(ra, tt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("thon_to_dan_pho: duyệt kết quả: %w", err)
	}
	if len(ra) > TranDanhSachThonToDanPho {
		// Rows already read are DROPPED rather than trimmed and returned: handing back a list the
		// caller might render anyway is how a refusal turns back into a silent truncation, one
		// careless `if err != nil { log }` later.
		return nil, ErrQuaNhieuThonToDanPho
	}
	return ra, nil
}
