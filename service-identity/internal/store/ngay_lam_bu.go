package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// NgayLamBuStore reads the dates this commune works although the week says otherwise — the Prime
// Minister's annual swap days (migration 0006).
//
// NO WRITE PATH, same reason as the other two calendar stores. The write path, when it lands, owes
// the same overlap rejection the weekly calendar does: `ngay_lam_bu` carries only UNIQUE
// (tenant_id, ngay, bat_dau), so two sessions can overlap on one swap day exactly as they can on a
// Monday (migration 0006:283).
type NgayLamBuStore struct {
	db *store.DB
}

func NewNgayLamBuStore(db *store.DB) *NgayLamBuStore { return &NgayLamBuStore{db: db} }

// TranNgayLamBuMotNam is the hard upper bound on one commune's swap-day SESSIONS in one year.
//
// BY YEAR, for the same reason as TranNgayNghiLeMotNam: this table grows with time, not with how
// the commune is organised, so a route returning all of it would grow without limit.
//
// 200 IS ABOUT TWENTY TIMES THE REAL FIGURE. An announcement produces a handful of swap days a
// year, each one or two sessions — call it ten rows. The ceiling is higher than the plausible
// figure and lower than a year's worth of days times sessions, which is the range in which it can
// still mean "this stopped being a list of swap days".
//
// IT IS NOT 366 LIKE THE HOLIDAYS, AND THE ASYMMETRY IS THE SCHEMA'S: a holiday is one row per
// date, a swap day is one row per SESSION, so no per-date unique key bounds this table at the
// number of days in a year.
const TranNgayLamBuMotNam = 200

// ErrQuaNhieuNgayLamBu says the ceiling was reached. The caller answers 500 and refuses.
//
// REFUSE RATHER THAN TRUNCATE, and the direction of the error is the mirror of the holidays': a
// silently short list of swap days counts a day the office WAS open as closed, which lengthens
// every deadline crossing it. The commune then looks slower than it was, on the days of the year
// when its backlog is largest.
var ErrQuaNhieuNgayLamBu = errors.New("ngay_lam_bu: vượt trần ngày làm bù trong một năm")

// truyVanNgayLamBu reads one commune's swap-day sessions for one year, AND FLAGS EACH DATE THAT IS
// ALSO A HOLIDAY.
//
// THE MIRROR OF truyVanNgayNghiLe, AND BOTH HALVES EXIST ON PURPOSE. The conflict — a date that is
// both a closure and a working day — is a property of the PAIR of tables, so refusing on only one
// side would leave the other screen looking healthy and the commune fixing nothing. Refusing on
// both is also what "do not pick a winner" means in practice (migration 0006:254;
// domain.LoiNgayVuaNghiVuaLamBu).
//
// NO DISTINCT ON THIS SIDE, and that asymmetry is the schema's rather than an oversight:
// `ngay_nghi_le` carries UNIQUE (tenant_id, ngay), so at most one holiday row can match a date and
// the join cannot multiply rows. The other direction needs DISTINCT because a swap day with a
// lunch break is two rows on one date.
//
// The commune constraint, the soft-delete predicate and the year range carry the same reasoning as
// truyVanNgayNghiLe; the argument is written out once, there.
//
// ORDER BY lb.ngay, lb.bat_dau — the order the screen shows and the order a day is walked in. It is
// TOTAL: UNIQUE (tenant_id, ngay, bat_dau) means two sessions cannot share a date and a start
// (migration 0006:283).
const truyVanNgayLamBu = `
SELECT lb.id,
       to_char(lb.ngay, 'YYYY-MM-DD') AS ngay,
       date_part('epoch', lb.bat_dau)::int AS bat_dau,
       date_part('epoch', lb.ket_thuc)::int AS ket_thuc,
       lb.ten,
       (nl.ngay IS NOT NULL) AS trung_nghi_le
FROM ngay_lam_bu lb
LEFT JOIN (SELECT tenant_id, ngay
             FROM ngay_nghi_le
            WHERE tenant_id = $1
              AND deleted_at IS NULL) nl
       ON nl.tenant_id = lb.tenant_id
      AND nl.ngay = lb.ngay
WHERE lb.tenant_id = $1
  AND lb.deleted_at IS NULL
  AND lb.ngay >= make_date($2, 1, 1)
  AND lb.ngay < make_date($2 + 1, 1, 1)
ORDER BY lb.ngay, lb.bat_dau
LIMIT $3`

// TheoNam reads the commune's swap-day sessions for one calendar year, ordered.
//
// IT REFUSES A CALENDAR THAT CONTRADICTS ITSELF, exactly as NgayNghiLeStore.TheoNam does, and
// returns no rows when it does.
//
// AN EMPTY YEAR IS ORDINARY HERE, unlike an empty weekly calendar: most years, most communes work
// no swap days at all. Nothing downstream may read "no swap days" as "calendar not configured".
//
// THE COMMUNE IS NOT A PARAMETER AND CANNOT BE ONE: Scoped.QueryJoin binds it to $1 from the
// context (rule 1, invariants 4 and 5).
func (s *NgayLamBuStore) TheoNam(ctx context.Context, nam int) ([]domain.CaLamBu, error) {
	// Ceiling PLUS ONE — see TranNgayLamBuMotNam.
	rows, err := s.db.For(ctx).QueryJoin(ctx, truyVanNgayLamBu, nam, TranNgayLamBuMotNam+1)
	if err != nil {
		return nil, fmt.Errorf("ngay_lam_bu: đọc ngày làm bù năm %d: %w", nam, err)
	}
	defer rows.Close()

	ra := make([]domain.CaLamBu, 0, 16)
	var xungDot []string
	for rows.Next() {
		var c domain.CaLamBu
		var batDau, ketThuc int
		var trungNghiLe bool
		// POSITIONAL — in lockstep with truyVanNgayLamBu. TWO PAIRS OF ADJACENT SAME-TYPED VALUES
		// sit in this list: `bat_dau` / `ket_thuc` (both int) and `ngay` / `ten` (both text).
		// Swapping either pair produces no error at all — the first makes a swap day run from
		// closing time to opening time, the second puts a date where the announcement's title
		// belongs on the commune's own screen.
		if err := rows.Scan(&c.ID, &c.Ngay, &batDau, &ketThuc, &c.Ten, &trungNghiLe); err != nil {
			return nil, fmt.Errorf("ngay_lam_bu: đọc dòng: %w", err)
		}
		c.BatDau = domain.GioTrongNgay(batDau)
		c.KetThuc = domain.GioTrongNgay(ketThuc)
		if trungNghiLe && chuaCo(xungDot, c.Ngay) {
			// ONE ENTRY PER DATE, not per session: a swap day with a lunch break is two rows and
			// would otherwise name the same date twice in the refusal.
			xungDot = append(xungDot, c.Ngay)
		}
		ra = append(ra, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ngay_lam_bu: duyệt kết quả: %w", err)
	}
	if len(xungDot) > 0 {
		return nil, &domain.LoiNgayVuaNghiVuaLamBu{Ngay: xungDot}
	}
	if len(ra) > TranNgayLamBuMotNam {
		// Rows already read are DROPPED rather than trimmed and returned.
		return nil, ErrQuaNhieuNgayLamBu
	}
	return ra, nil
}

// chuaCo reports whether a value is absent from a short list. A linear scan over a list bounded by
// TranNgayLamBuMotNam, walked only for rows that already carry a conflict — a map would be a data
// structure for a handful of strings that also loses the statement's ascending order.
func chuaCo(ds []string, v string) bool {
	for _, d := range ds {
		if d == v {
			return false
		}
	}
	return true
}
