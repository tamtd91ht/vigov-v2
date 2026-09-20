package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// NgayNghiLeStore reads the dates this commune does not work (migration 0006).
//
// NO WRITE PATH, same reason as LichLamViecStore: who may edit a commune's calendar has not been
// asked.
type NgayNghiLeStore struct {
	db *store.DB
}

func NewNgayNghiLeStore(db *store.DB) *NgayNghiLeStore { return &NgayNghiLeStore{db: db} }

// TranNgayNghiLeMotNam is the hard upper bound on one commune's holidays IN ONE YEAR.
//
// READ BY YEAR AND NOT WHOLE, AND THAT IS THE DIFFERENCE FROM EVERY CATALOGUE IN THIS SERVICE. A
// catalogue's size is a property of how the commune is organised; this table's size is a property
// of HOW LONG the commune has been using the system — fifteen-odd rows a year, for ever. A route
// returning "all of it" would grow without limit, which is exactly the shape
// skills/rest-api-design §5 forbids on a process shared by 200+ communes. So the window is a year:
// the unit the configuration screen shows, and the unit the Prime Minister's announcement comes in.
//
// 366 IS THE NUMBER OF DAYS IN A LEAP YEAR, and the ceiling is therefore UNREACHABLE while the
// year filter works — UNIQUE (tenant_id, ngay) allows at most one row per date (migration
// 0006:215). That is deliberate rather than a wasted bound: it can only trip if the window itself
// is broken, and a refusal is precisely the right answer then. The bound still does its real job,
// which is keeping one commune's data from being read unbounded into the memory of a process that
// serves every commune.
const TranNgayNghiLeMotNam = 366

// ErrQuaNhieuNgayNghiLe says the ceiling was reached. The caller answers 500 and refuses.
//
// REFUSE RATHER THAN TRUNCATE: a silently short list of holidays is a day the office was closed
// being counted as a working day, which shortens every deadline crossing it. The commitment told
// to the citizen and the commitment the system counts then differ, and nothing on any screen says
// so.
var ErrQuaNhieuNgayNghiLe = errors.New("ngay_nghi_le: vượt trần ngày nghỉ lễ trong một năm")

// truyVanNgayNghiLe reads one commune's holidays for one year, AND FLAGS EACH DATE THAT IS ALSO A
// SWAP WORKING DAY.
//
// THE JOIN IS NOT DECORATION — IT IS THE REFUSAL MIGRATION 0006:254 ASKS FOR. A date present in
// both `ngay_nghi_le` and `ngay_lam_bu` says the commune is closed and working on the same day.
// Neither table may quietly win; both read paths refuse. Detecting it in the SAME statement rather
// than in a second query is what makes the two halves come from one instant — two queries could
// straddle a write and report a conflict that had just been fixed, or miss one that had just been
// created.
//
// EACH CLAUSE OF THE JOIN IS LOAD-BEARING:
//
//	DISTINCT               `ngay_lam_bu` holds one row per SESSION, so a swap day with a lunch
//	                       break is two rows on one date. Without DISTINCT a plain join would
//	                       return the holiday TWICE, and the commune's list of holidays would grow
//	                       rows nobody entered. `ngay_nghi_le` needs no such guard on the other
//	                       side — UNIQUE (tenant_id, ngay) means one row per date there.
//	tenant_id = $1 inside  the derived table is constrained to this commune BEFORE the join, so no
//	                       partition of another commune is ever scanned.
//	lb.tenant_id = nl.tenant_id
//	                       AND THE JOIN ITSELF IS CONSTRAINED TO THE SAME COMMUNE. Joining on the
//	                       date alone would let another commune's swap day flag this commune's
//	                       holiday, and 02/09 collides in every commune in the country. That is the
//	                       leak rule 1 exists to prevent, and no single-commune test would show it.
//	deleted_at IS NULL     in BOTH places: a soft-deleted swap day is not a swap day (rule 7,
//	                       invariant 2). Without it, deleting the offending row would not clear the
//	                       conflict and the commune could never make its calendar readable again.
//
// THE YEAR IS A RANGE, NOT `EXTRACT(YEAR FROM ngay) = $2`. A function on the column cannot use the
// index migration 0006:231 creates, and the range form reads the same rows.
//
// ORDER BY nl.ngay — dates ascending, which is what the screen shows. The order is TOTAL because
// the date is unique per commune.
const truyVanNgayNghiLe = `
SELECT nl.id,
       to_char(nl.ngay, 'YYYY-MM-DD') AS ngay,
       nl.ten,
       (lb.ngay IS NOT NULL) AS trung_lam_bu
FROM ngay_nghi_le nl
LEFT JOIN (SELECT DISTINCT tenant_id, ngay
             FROM ngay_lam_bu
            WHERE tenant_id = $1
              AND deleted_at IS NULL) lb
       ON lb.tenant_id = nl.tenant_id
      AND lb.ngay = nl.ngay
WHERE nl.tenant_id = $1
  AND nl.deleted_at IS NULL
  AND nl.ngay >= make_date($2, 1, 1)
  AND nl.ngay < make_date($2 + 1, 1, 1)
ORDER BY nl.ngay
LIMIT $3`

// TheoNam reads the commune's holidays for one calendar year, ordered.
//
// IT REFUSES A CALENDAR THAT CONTRADICTS ITSELF. When any date in the window is also a swap
// working day, this returns *domain.LoiNgayVuaNghiVuaLamBu naming every such date and NO rows —
// handing back the list alongside the error is how a refusal turns into a caller picking a winner
// after all.
//
// THE COMMUNE IS NOT A PARAMETER AND CANNOT BE ONE: Scoped.QueryJoin binds it to $1 from the
// context (rule 1, invariants 4 and 5). `nam` is the only thing the caller chooses, and it chooses
// a window, never a commune.
func (s *NgayNghiLeStore) TheoNam(ctx context.Context, nam int) ([]domain.NgayNghiLe, error) {
	// Ceiling PLUS ONE — a full list of exactly the ceiling is indistinguishable from a complete
	// one of that size.
	rows, err := s.db.For(ctx).QueryJoin(ctx, truyVanNgayNghiLe, nam, TranNgayNghiLeMotNam+1)
	if err != nil {
		return nil, fmt.Errorf("ngay_nghi_le: đọc ngày nghỉ lễ năm %d: %w", nam, err)
	}
	defer rows.Close()

	ra := make([]domain.NgayNghiLe, 0, 32)
	var xungDot []string
	for rows.Next() {
		var n domain.NgayNghiLe
		var trungLamBu bool
		// POSITIONAL — in lockstep with truyVanNgayNghiLe. `ngay` and `ten` are two adjacent TEXT
		// values (the date is rendered as text on purpose — domain.NgayNghiLe.Ngay), so swapping
		// them produces no error at all: the screen would show "2026-09-02" where the holiday's
		// name belongs and a name where the date belongs.
		if err := rows.Scan(&n.ID, &n.Ngay, &n.Ten, &trungLamBu); err != nil {
			return nil, fmt.Errorf("ngay_nghi_le: đọc dòng: %w", err)
		}
		if trungLamBu {
			xungDot = append(xungDot, n.Ngay)
		}
		ra = append(ra, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ngay_nghi_le: duyệt kết quả: %w", err)
	}
	if len(xungDot) > 0 {
		// EVERY conflicting date, in the statement's own ascending order, so whoever fixes the
		// configuration is not sent back for a second round after correcting the first one.
		return nil, &domain.LoiNgayVuaNghiVuaLamBu{Ngay: xungDot}
	}
	if len(ra) > TranNgayNghiLeMotNam {
		// Rows already read are DROPPED rather than trimmed and returned.
		return nil, ErrQuaNhieuNgayNghiLe
	}
	return ra, nil
}
