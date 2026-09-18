package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// QuyenStore reads the Phân quyền matrix: the software's permission catalogue, this commune's
// roles, and which of those roles hold which key.
//
// IT DECIDES NOTHING, exactly like VaiTroStore. Access decisions belong to Checker, one key at a
// time, against `vai_tro_quyen`, on every request. This store DESCRIBES the grants so an
// administrator can see them on a screen. The two must not merge: a caller that read the matrix
// here and branched on it would be deciding access from a description, which is rule 5,
// forbidden #3 — and it would decide from a snapshot, while Checker re-reads on every request so a
// withdrawn grant takes effect immediately (skills/session-and-token).
type QuyenStore struct {
	db *store.DB
}

func NewQuyenStore(db *store.DB) *QuyenStore { return &QuyenStore{db: db} }

// --- ceilings -----------------------------------------------------------------------------------
//
// REFUSE, NEVER TRUNCATE — the same discipline as TranDanhMucBoPhan and TranDanhMucVaiTro, and on
// this screen the cost of getting it wrong is one step worse than on either of those. A silently
// short list here is a permission key that has VANISHED FROM THE MATRIX: the administrator sees a
// role without that row, concludes the role does not hold the right, and the screen looks entirely
// normal. Nothing turns red, and the conclusion drawn from it is wrong in the direction that
// matters — somebody is granted, or refused, a right on a government system on the strength of a
// screen that was quietly incomplete.
//
// The role ceiling is NOT redefined here: it is TranDanhMucVaiTro, the same catalogue read by the
// same commune. A second number for one bound is two numbers that will drift apart.

// TranDanhMucQuyen is the hard upper bound on the software's permission catalogue.
//
// The catalogue ships with the code — it can only grow through a migration — so this bound is not
// guarding against a commune's data, it is guarding one shared process against a catalogue that has
// stopped being a catalogue. The specification lists 43 keys and the migration seeds 33
// (0001_init.sql:262). 300 is roughly seven times the specified figure: past it, the matrix is not
// a screen anybody can read, and the likely cause is an import run twice rather than a system that
// grew ten times.
const TranDanhMucQuyen = 300

// TranCapQuyen is the hard upper bound on one commune's ticked cells.
//
// The real figure is small: eight roles times forty-three keys is under 350 (spec §4.1 and §4.2),
// and that is the whole matrix ticked, which never happens. 5000 is about fourteen times that, and
// still above what a commune with a large-but-plausible role catalogue could legitimately produce.
// Past it the data is not a grant set — it is a loop that inserted rows, or a fixture on a live
// database.
const TranCapQuyen = 5000

// ErrQuaNhieuQuyen says the permission catalogue exceeded its ceiling. The caller answers 500.
var ErrQuaNhieuQuyen = errors.New("quyen: vượt trần danh mục quyền")

// ErrQuaNhieuCapQuyen says the commune's grant set exceeded its ceiling. The caller answers 500.
var ErrQuaNhieuCapQuyen = errors.New("quyen: vượt trần số ô đã cấp")

// --- the catalogue ------------------------------------------------------------------------------

// truyVanDanhMucQuyen reads the WHOLE permission catalogue, granted or not.
//
// THIS IS THE ONE QUERY IN THIS SERVICE THAT DOES NOT FILTER BY COMMUNE, AND IT IS NOT A HOLE IN
// RULE 1. `quyen` has no `tenant_id` column to filter on: the catalogue is the set of rights THE
// SOFTWARE can enforce, identical for every commune, seeded by the migration that ships with the
// binary (0001_init.sql:44 states the same thing at the schema). Nothing in it describes a commune,
// names a person, or reveals that another commune exists — three keys and a Vietnamese label are
// the same three strings in all 200+ communes. What IS a commune's data is the GRANT, and that is
// read by truyVanCapQuyen below, scoped, always.
//
// THE `WHERE` CLAUSE IS NOT A FILTER AND IS NOT DECORATION. Scoped.QueryJoin binds the commune
// to $1 whatever the statement does with it, and a $1 that appears nowhere makes PostgreSQL refuse
// the query outright ("could not determine data type of parameter $1"). Consuming it as an
// assertion keeps the read on the ONE sanctioned path — the commune must already have been resolved
// for *Scoped to exist at all (store.DB.For panics otherwise) — instead of asking core/store for an
// unscoped escape hatch that every future query could then reach for. The predicate is always true
// in practice, and if a commune ever were empty this returns nothing, which fails closed.
//
// ORDER BY thu_tu, ma: `thu_tu` is the order the specification lists the keys in, and `ma` breaks
// ties so two keys sharing a rank cannot swap places between two calls — an unstable order makes
// the matrix's rows reshuffle on every reload.
const truyVanDanhMucQuyen = `
SELECT q.ma, q.nhom, q.nhan
FROM quyen q
WHERE $1::text <> ''
ORDER BY q.thu_tu, q.ma
LIMIT $2`

// --- the columns --------------------------------------------------------------------------------

// truyVanVaiTroKemSoCanBo reads the commune's roles WITH TWO counts of the staff holding each.
//
// BOTH COUNTS COME FROM ONE PASS OVER ONE JOIN, and that is the difference between one round trip
// and reading a commune's entire staff register into memory to length a few slices
// (skills/load-data-once). A second query for the second number would also be a second instant:
// the two figures on one column header could then disagree with each other. It keeps personal data
// out of this path entirely as well — nothing about a person is ever selected, so nothing about a
// person can accidentally reach the response (rule 3).
//
// count(nd.id) AND NOT count(*) — THE ONE LINE THAT IS WRONG IN A WAY NOTHING REPORTS. This is a
// LEFT JOIN, so a role nobody holds still produces one row, with every `nd.*` column NULL.
// count(*) counts that row and answers 1; count(nd.id) skips NULLs and answers 0. The screen would
// show "1 cán bộ" against every empty role, and it looks entirely plausible.
//
// THE SECOND COUNT USES `FILTER`, AND THE FILTER MUST STAY ON THE AGGREGATE. Moved into the JOIN's
// ON clause it would narrow the FIRST count too — the register figure would silently stop matching
// the Người dùng tab. Moved into the WHERE it would drop the whole ROLE whenever no holder has a
// live account, so the column with the most to say ("3 cán bộ · 0 đang hiệu lực") is exactly the
// one that would vanish from the matrix. Same failure as `nd.deleted_at`, one step further along.
//
// ITS PREDICATE IS THE ONE store.Checker ENFORCES — `co_tai_khoan AND dang_hoat_dong` — and it is
// written here rather than referenced, which means two copies that can drift. The direction of that
// drift is stated so it is not mistaken for a bug when it is read: looser here and the header
// OVERSTATES how far a tick reaches; stricter and it understates. Neither changes who may do what:
// this is a description, and checker.go remains the only thing that decides (rule 5, forbidden #3).
//
// THE JOIN IS CONSTRAINED TO THE SAME COMMUNE, not merely to the matching role id. Joining on id
// alone would count another commune's staff wherever ids collide — the leak rule 1 exists to
// prevent, and one no single-commune test would ever show. Same discipline as checker.go.
//
// `nd.deleted_at IS NULL` SITS IN THE JOIN, NOT IN THE WHERE. In the WHERE it would drop the ROLE
// along with its deleted staff, so a role whose only holder was removed would disappear from the
// matrix — and a role missing from the matrix cannot be granted anything. In the join it leaves the
// role and counts zero, which is the truth. It stays in the JOIN rather than moving into both
// FILTERs because a deleted person is not a holder of the role at all, on either count.
const truyVanVaiTroKemSoCanBo = `
SELECT vt.id, vt.ma, vt.ten, vt.la_lanh_dao,
       count(nd.id)                                                          AS so_can_bo,
       count(nd.id) FILTER (WHERE nd.co_tai_khoan AND nd.dang_hoat_dong)     AS so_tai_khoan_hoat_dong
FROM vai_tro vt
LEFT JOIN nguoi_dung nd
       ON nd.tenant_id  = vt.tenant_id
      AND nd.vai_tro_id = vt.id
      AND nd.deleted_at IS NULL
WHERE vt.tenant_id = $1
  AND vt.deleted_at IS NULL
GROUP BY vt.id, vt.ma, vt.ten, vt.la_lanh_dao, vt.thu_tu
ORDER BY vt.thu_tu, vt.ten
LIMIT $2`

// --- the ticked cells ---------------------------------------------------------------------------

// truyVanCapQuyen reads the commune's grants — every ticked cell of the matrix.
//
// SCOPED, AND THIS IS THE PART THAT REALLY IS A COMMUNE'S DATA: which role in THIS commune holds
// which right. `vai_tro_quyen.tenant_id` is what stops commune A's grant from applying in commune B
// (migration 0001_init.sql:125), and the join to `vai_tro` repeats the commune rather than matching
// on the role id alone, for the reason stated on the query above.
//
// THE JOIN TO `vai_tro` IS NOT DECORATION EITHER: it drops grants belonging to a soft-deleted role.
// Rule 7 keeps those rows, so they are still there; without this the response would carry cells
// naming a role absent from `Cot`, and a client mapping cells by role id would silently drop them —
// or, worse, draw a column for a role the commune has retired.
//
// ORDER BY so two calls return the same order: an unordered list makes every client-side comparison
// and every test compare sets by accident.
const truyVanCapQuyen = `
SELECT vq.vai_tro_id, vq.quyen_ma
FROM vai_tro_quyen vq
JOIN vai_tro vt
  ON vt.tenant_id  = vq.tenant_id
 AND vt.id         = vq.vai_tro_id
 AND vt.deleted_at IS NULL
WHERE vq.tenant_id = $1
ORDER BY vq.vai_tro_id, vq.quyen_ma
LIMIT $2`

// MaTran reads everything the Phân quyền tab needs, in the commune carried by ctx.
//
// THREE QUERIES AND NOT ONE, deliberately. A single statement would have to produce the catalogue,
// the roles and the grants as one rectangle, which means repeating the catalogue once per role —
// 33 keys times 8 roles of mostly-empty rows — to answer a question that is three small lists. Three
// round trips on a screen an administrator opens occasionally is the cheaper side of that trade;
// what skills/load-data-once forbids is a query PER ROW, and there is none here.
//
// NOT PAGINATED, same decision and same reasons as BoPhanStore.DanhSach: three closed reference
// lists whose consumer needs them whole — a matrix missing rows is a matrix that lies. The bound
// pagination would have given is the three ceilings above, enforced in SQL.
//
// THE COMMUNE IS NOT A PARAMETER AND CANNOT BE ONE: Scoped binds it to $1 from the context, so this
// can only ever read the roles and grants of the commune the request arrived in (rule 1).
func (s *QuyenStore) MaTran(ctx context.Context) (domain.MaTranQuyen, error) {
	danhMuc, err := s.danhMucQuyen(ctx)
	if err != nil {
		return domain.MaTranQuyen{}, err
	}
	cot, err := s.vaiTroKemSoCanBo(ctx)
	if err != nil {
		return domain.MaTranQuyen{}, err
	}
	daCap, err := s.capQuyen(ctx)
	if err != nil {
		return domain.MaTranQuyen{}, err
	}
	return domain.MaTranQuyen{DanhMuc: danhMuc, Cot: cot, DaCap: daCap}, nil
}

func (s *QuyenStore) danhMucQuyen(ctx context.Context) ([]domain.Quyen, error) {
	// Ceiling PLUS ONE, and that is what makes "there are too many" detectable at all: selecting
	// exactly the ceiling returns a full page indistinguishable from a complete list of that size —
	// the truncation this refuses to perform, performed by the bound meant to prevent it.
	rows, err := s.db.For(ctx).QueryJoin(ctx, truyVanDanhMucQuyen, TranDanhMucQuyen+1)
	if err != nil {
		return nil, fmt.Errorf("quyen: đọc danh mục quyền: %w", err)
	}
	defer rows.Close()

	ra := make([]domain.Quyen, 0, 64)
	for rows.Next() {
		var q domain.Quyen
		// POSITIONAL — in lockstep with the SELECT list. `nhom` and `nhan` are adjacent TEXT
		// columns: swapping them produces no error at all, and the matrix shows group headings
		// where row labels belong.
		if err := rows.Scan(&q.Ma, &q.Nhom, &q.Nhan); err != nil {
			return nil, fmt.Errorf("quyen: đọc dòng quyền: %w", err)
		}
		ra = append(ra, q)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("quyen: duyệt danh mục quyền: %w", err)
	}
	if len(ra) > TranDanhMucQuyen {
		// The rows already read are DROPPED rather than trimmed and returned. Handing back a list
		// the caller might render anyway is how a refusal turns back into a silent truncation, one
		// careless `if err != nil { log }` later.
		return nil, ErrQuaNhieuQuyen
	}
	return ra, nil
}

func (s *QuyenStore) vaiTroKemSoCanBo(ctx context.Context) ([]domain.VaiTroCot, error) {
	rows, err := s.db.For(ctx).QueryJoin(ctx, truyVanVaiTroKemSoCanBo, TranDanhMucVaiTro+1)
	if err != nil {
		return nil, fmt.Errorf("quyen: đọc vai trò kèm số cán bộ: %w", err)
	}
	defer rows.Close()

	ra := make([]domain.VaiTroCot, 0, 16)
	for rows.Next() {
		var c domain.VaiTroCot
		// POSITIONAL — `ma` and `ten` are adjacent TEXT columns; the note on cotVaiTroMuc applies
		// unchanged.
		if err := rows.Scan(&c.ID, &c.Ma, &c.Ten, &c.LaLanhDao,
			&c.SoCanBo, &c.SoTaiKhoanDangHoatDong); err != nil {
			return nil, fmt.Errorf("quyen: đọc dòng vai trò: %w", err)
		}
		ra = append(ra, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("quyen: duyệt vai trò: %w", err)
	}
	if len(ra) > TranDanhMucVaiTro {
		// ErrQuaNhieuVaiTro and not an error of this file's own: it is the same catalogue, over the
		// same ceiling, and the caller already knows how to answer it.
		return nil, ErrQuaNhieuVaiTro
	}
	return ra, nil
}

func (s *QuyenStore) capQuyen(ctx context.Context) ([]domain.CapQuyen, error) {
	rows, err := s.db.For(ctx).QueryJoin(ctx, truyVanCapQuyen, TranCapQuyen+1)
	if err != nil {
		return nil, fmt.Errorf("quyen: đọc ô đã cấp: %w", err)
	}
	defer rows.Close()

	ra := make([]domain.CapQuyen, 0, 256)
	for rows.Next() {
		var c domain.CapQuyen
		if err := rows.Scan(&c.VaiTroID, &c.QuyenMa); err != nil {
			return nil, fmt.Errorf("quyen: đọc dòng ô đã cấp: %w", err)
		}
		ra = append(ra, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("quyen: duyệt ô đã cấp: %w", err)
	}
	if len(ra) > TranCapQuyen {
		return nil, ErrQuaNhieuCapQuyen
	}
	return ra, nil
}
