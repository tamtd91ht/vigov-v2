package store

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/vihat/vigov/service-identity/internal/domain"
)

// The read behind GET /api/v1/staff-directory — the FIFTH staff read in this package, and the only
// one whose question is "whom may I hand work to".
//
// IT MUST NOT BE MERGED WITH ANY OF THE OTHER FOUR, because its predicate is its own:
//
//	DanhSach / ChiTiet   the register     not deleted
//	TheoEmail / TheoID   sign-in          not deleted, has an account, not locked
//	TenTheoNhieuMa       archival names   (commune only — deleted rows on purpose)
//	this one             the picker       not deleted, has an account, not locked
//
// It shares sign-in's three conditions but not its columns: sign-in reads the password hash, and
// this runs for every account of the commune. Two statements, two column lists.

// TranDanhBaChonNguoi is the hard upper bound on one commune's picker list.
//
// NOT PAGINATED, FOR THE REASON TranDanhMucBoPhan GIVES: an assignee box showing the first page of
// colleagues is a box missing the colleague somebody needs, and nothing on the screen says so. A
// commune's directory is a few dozen people (the customer's own lists 26, 12-danh-ba-can-bo.md §7);
// 500 is the point past which the data is wrong, not large.
const TranDanhBaChonNguoi = 500

// ErrQuaNhieuCanBoChonNguoi says the ceiling was reached. The caller answers 500 and REFUSES —
// a silently short picker sends work to the wrong person or to nobody (fail closed).
var ErrQuaNhieuCanBoChonNguoi = errors.New("can_bo: danh bạ chọn người vượt trần")

// cotChonNguoi — POSITIONAL, in lockstep with the Scan in chonNguoi. `ho_ten` and `chuc_vu` are
// adjacent TEXT columns; a swap errors nowhere and prints every position as a name.
//
// WHAT IS NOT SELECTED IS THE CONTRACT: no id, no email, no telephone number, no account or lock
// flag, no password hash. A value never read cannot leak through a handler that forgot to drop it.
const cotChonNguoi = `ma, ho_ten, chuc_vu, coalesce(bo_phan_id,'')`

// locChonNguoi is the picker's predicate. EACH CONDITION IS A DECISION, stated where it is paid:
//
//	deleted_at IS NULL  a soft-deleted row is a duplicate kept for the trail (#10, rule 7 inv 2).
//	co_tai_khoan        a directory-only person can never ACT on the work. Every holder check
//	                    compares the assignee's code with a signed-in Principal.Ma
//	                    (service-petitions app/xu_ly_phan_anh.go), and without an account there
//	                    is never such a principal — the work would sit with nobody, forever.
//	dang_hoat_dong      a locked account is somebody retired or moved (#10). They stay on old
//	                    records, but must not be offered as a NEW assignee, for the same reason:
//	                    they can no longer sign in to act.
//
// Adding a person to the picker is therefore giving them an account, or unlocking one — both
// audited acts on the register, never a change here.
const locChonNguoi = `AND deleted_at IS NULL AND co_tai_khoan AND dang_hoat_dong`

// thuTuChonNguoi — a picker is scanned by name, and `ma` (unique within the commune) breaks ties so
// two people of the same name never swap places between two loads.
const thuTuChonNguoi = ` ORDER BY ho_ten ASC, ma ASC`

// locGiuQuyen narrows the picker to people who HOLD one permission key, in this commune, today —
// the `permission` filter of GET /api/v1/staff-directory (owner decision 27/09/2026: the "Lãnh đạo
// giao việc" box offers only holders of `task.extend`). The key is the placeholder %s.
//
// THE PREDICATE IS store.Checker's, NOT A SECOND SPELLING OF IT. The join (noiGiuQuyen) and the
// conditions (dieuKienGiuQuyen) are truyVanQuyenGoc's own, shared by constant — so "offered as somebody who holds X" and
// "allowed to do X" cannot drift. Looser here, and the box offers a person the extension route then
// answers 403 for; stricter, and a real leader is missing from the box with nothing on screen
// saying why.
//
// THE SEMI-JOIN IS ON `ma`, BOTH SIDES, AND THAT IS THE TRAP THIS CLAUSE EXISTS TO AVOID. The
// Checker's query matches `nd.id` because a Principal carries the internal id; the picker's row is
// keyed by the business code, and `id` and `ma` never hold the same value. `ma IN (SELECT nd.id …)`
// compiles, runs, and matches nobody — an empty "Lãnh đạo" box in every commune, with every test
// that uses equal ids green. `(tenant_id, ma)` is unique and deliberately NOT partial (migration
// 0009 §2), so one code is one person, soft-deleted rows included.
//
// EVERY TABLE IS BOUND TO THE COMMUNE: `nd.tenant_id = $1` (the same $1 Scoped.Query binds for the
// outer row) and each JOIN's ON clause repeats it — role ids are per commune, and a join on id alone
// would read commune B's grant for a role id that happens to exist in both.
//
// AN UNKNOWN KEY MATCHES NOBODY, AND THAT IS THE ANSWER, NOT A FALLBACK: `vai_tro_quyen.quyen_ma`
// REFERENCES `quyen(ma)`, so a key outside the catalogue is held by no one. Nothing here reads
// `quyen` to turn that into an error — see the handler for why.
const locGiuQuyen = ` AND ma IN (SELECT nd.ma` + noiGiuQuyen + dieuKienGiuQuyen + `
  AND vq.quyen_ma = %s)`

// ChonNguoi reads the commune's whole picker list, optionally narrowed (domain.LocChonNguoi).
//
// THE COMMUNE IS NOT A PARAMETER AND CANNOT BE ONE: Scoped.Query binds it to $1 from the context
// (rule 1, invariant 5). Both filters reach the statement only as bound parameters; "" means "no
// such filter", never "any".
func (s *CanBoStore) ChonNguoi(ctx context.Context, loc domain.LocChonNguoi) ([]domain.CanBoChonNguoi, error) {
	return s.chonNguoi(ctx, loc, TranDanhBaChonNguoi)
}

// chonNguoi takes the ceiling as a parameter only so a test can reach the refusal with a handful
// of rows; production has exactly one caller, with TranDanhBaChonNguoi.
func (s *CanBoStore) chonNguoi(ctx context.Context, loc domain.LocChonNguoi, tran int) ([]domain.CanBoChonNguoi, error) {
	tail := locChonNguoi
	var args []any
	if loc.BoPhanID != "" {
		args = append(args, loc.BoPhanID)
		tail += " AND bo_phan_id = $" + strconv.Itoa(len(args)+1)
	}
	if loc.QuyenMa != "" {
		args = append(args, loc.QuyenMa)
		tail += fmt.Sprintf(locGiuQuyen, "$"+strconv.Itoa(len(args)+1))
	}
	// LIMIT is the ceiling PLUS ONE — the only way "too many" is distinguishable from "exactly the
	// ceiling". See BoPhanStore.DanhSach.
	args = append(args, tran+1)
	tail += thuTuChonNguoi + " LIMIT $" + strconv.Itoa(len(args)+1)

	rows, err := s.db.For(ctx).Query(ctx, cotChonNguoi, "nguoi_dung", tail, args...)
	if err != nil {
		return nil, fmt.Errorf("can_bo: đọc danh bạ chọn người: %w", err)
	}
	defer rows.Close()

	ra := make([]domain.CanBoChonNguoi, 0, 32)
	for rows.Next() {
		var cb domain.CanBoChonNguoi
		// NEVER LOG A SCANNED ROW: HoTen is a person's name (rule 3, forbidden #1).
		if err := rows.Scan(&cb.Ma, &cb.HoTen, &cb.ChucVu, &cb.BoPhanID); err != nil {
			return nil, fmt.Errorf("can_bo: đọc dòng danh bạ chọn người: %w", err)
		}
		ra = append(ra, cb)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("can_bo: duyệt danh bạ chọn người: %w", err)
	}
	if len(ra) > tran {
		// DROPPED, not trimmed: a partial list handed back is a refusal one careless caller away
		// from being rendered.
		return nil, ErrQuaNhieuCanBoChonNguoi
	}
	return ra, nil
}
