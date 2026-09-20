package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// LichLamViecStore reads the commune's ordinary working week (migration 0006).
//
// NOT A CATALOGUE STORE, and it deliberately does not go through docDanhMuc: this table has no
// `ma`, no `nhan`, no `thu_tu` and none of the tier columns. It holds a weekday and two times.
//
// NO WRITE PATH. Who may edit a commune's working calendar has not been asked — it is the sibling
// of open question #21, and the configuration screen that would ask it does not exist. A
// half-written write path looks like a decision somebody made. The write path, when it lands, owes
// one thing this read cannot provide: it must REJECT an overlapping session, because the database
// does not (see domain.TimChongNhau and migration 0006:109).
type LichLamViecStore struct {
	db *store.DB
}

func NewLichLamViecStore(db *store.DB) *LichLamViecStore { return &LichLamViecStore{db: db} }

// TranLichLamViec is the hard upper bound on one commune's weekly calendar.
//
// Same argument as TranDanhMucBoPhan: one process serves 200+ communes, so every list route is a
// shared resource and an unbounded one is forbidden (skills/rest-api-design §5). This route returns
// the WHOLE week — a deadline cannot be computed from part of a calendar — so the bound cannot come
// from a `limit` parameter and has to be a ceiling the commune's data is checked against.
//
// 100 IS ABOUT FIVE TIMES THE OUTER PLAUSIBLE SHAPE. Seven weekdays times three sessions is
// twenty-one, and migration 0006:31 puts the real figure at about ten. The number is not an
// estimate of how many sessions a commune might have; it is the point past which the rows have
// stopped being a working week — an import run twice, a loop that inserted rows, a fixture on a
// live database.
const TranLichLamViec = 100

// ErrQuaNhieuCaLamViec says the ceiling was reached. The caller answers 500 and refuses.
//
// REFUSE RATHER THAN TRUNCATE, and here the cost of truncating is heavier than on any catalogue:
// a silently short calendar is a working session that has disappeared from the week, so every
// deadline computed afterwards is LONGER than the commitment the commune actually made — and the
// citizen who was told "within 3 days" is the one who finds out. A refusal breaks one commune's
// configuration screen loudly and names itself in the log.
var ErrQuaNhieuCaLamViec = errors.New("lich_lam_viec: vượt trần lịch làm việc")

// cotLichLamViec IS READ BY POSITION in DanhSach.
//
// `bat_dau` AND `ket_thuc` ARE TWO ADJACENT COLUMNS OF ONE TYPE — swapping them here, or in the
// Scan, produces no error at all: the week simply runs from closing time to opening time, the
// CHECK constraint that would have caught it lives in the database and is never consulted on a
// read, and every deadline computed from it is wrong in a direction nobody can see.
//
// THE TWO TIMES ARE CONVERTED TO SECONDS SINCE MIDNIGHT IN SQL, and that is not a formatting
// preference — see domain.GioTrongNgay for the three reasons. `date_part` and NOT `EXTRACT(EPOCH
// FROM …)`, which is the same function: EXTRACT's syntax puts the word FROM inside the SELECT list,
// and every tool that reads a statement's column list — including the fake driver these stores are
// tested with — finds the end of that list by looking for FROM.
//
// THE ALIASES ARE LOAD-BEARING: without `AS bat_dau` PostgreSQL names the output column
// `date_part`, twice, and a reader comparing the column list against the Scan order has nothing to
// compare.
const cotLichLamViec = `id, thu, date_part('epoch', bat_dau)::int AS bat_dau, ` +
	`date_part('epoch', ket_thuc)::int AS ket_thuc, ghi_chu`

// DanhSach reads the commune's whole working week, ordered.
//
// NOT PAGINATED, and for a stronger reason than the catalogues: a calendar is only correct WHOLE.
// A page of it is a week with sessions missing, and nothing downstream can tell that apart from a
// commune that genuinely does not work those hours. The bound pagination would have provided is
// TranLichLamViec, enforced in SQL.
//
// AN EMPTY RESULT IS NOT AN ERROR HERE, AND IT IS NOT A DEFAULT EITHER. It is returned as an empty
// list because the configuration screen must be able to show "chưa cấu hình" — and today that is
// EVERY commune, since migration 0006 seeds nothing (0006:48). What must never happen is a caller
// treating the empty list as ordinary working hours: domain.VanDeCuaLich turns it into a named
// problem, and anything computing a deadline must refuse on it (rule 10; migration 0006:56).
//
// ORDER BY thu, bat_dau — the order the commune's own screen shows, and the order the deadline
// function walks a day in. It is TOTAL, so two calls cannot return the same rows in a different
// sequence: UNIQUE (tenant_id, thu, bat_dau) makes the pair unique per commune
// (migration 0006:149). It is also exactly the index (0006:170).
//
// THE COMMUNE IS NOT A PARAMETER AND CANNOT BE ONE: Scoped.Query binds it to $1 from the context
// (rule 1, invariants 4 and 5), so this can only ever read the calendar of the commune the request
// arrived in.
func (s *LichLamViecStore) DanhSach(ctx context.Context) ([]domain.CaLamViec, error) {
	// LIMIT is the ceiling PLUS ONE, which is what makes "there are too many" detectable at all.
	// Selecting exactly the ceiling returns a full list indistinguishable from a complete one of
	// that size — the truncation this refuses to perform, performed by the bound meant to prevent it.
	rows, err := s.db.For(ctx).Query(ctx, cotLichLamViec, "lich_lam_viec",
		`AND deleted_at IS NULL ORDER BY thu, bat_dau LIMIT $2`, TranLichLamViec+1)
	if err != nil {
		return nil, fmt.Errorf("lich_lam_viec: đọc lịch: %w", err)
	}
	defer rows.Close()

	ra := make([]domain.CaLamViec, 0, 16)
	for rows.Next() {
		var c domain.CaLamViec
		var batDau, ketThuc int
		// POSITIONAL — in lockstep with cotLichLamViec. See the note there on the two adjacent
		// time columns.
		if err := rows.Scan(&c.ID, &c.Thu, &batDau, &ketThuc, &c.GhiChu); err != nil {
			return nil, fmt.Errorf("lich_lam_viec: đọc dòng: %w", err)
		}
		c.BatDau = domain.GioTrongNgay(batDau)
		c.KetThuc = domain.GioTrongNgay(ketThuc)
		ra = append(ra, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("lich_lam_viec: duyệt kết quả: %w", err)
	}
	if len(ra) > TranLichLamViec {
		// The rows already read are DROPPED rather than trimmed and returned. Handing back a list
		// the caller might render anyway is how a refusal turns back into a silent truncation one
		// careless `if err != nil { log }` later.
		return nil, ErrQuaNhieuCaLamViec
	}
	return ra, nil
}
