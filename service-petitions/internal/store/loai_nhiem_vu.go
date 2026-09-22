package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// LoaiNhiemVuStore reads the commune's task-type catalogue (`loai_nhiem_vu`, migration 0003).
//
// IT READS ONLY. Creating, relabelling, disabling and soft-deleting a catalogue row are the
// configuration screen's operations, and open question #21 — may a commune edit the CODE LIST of a
// task catalogue, or only the labels and the order — is still OPEN. A half-written write path here
// would look like somebody had answered it.
type LoaiNhiemVuStore struct {
	db *store.DB
}

// NewLoaiNhiemVuStore takes *store.DB and nothing else. There is deliberately no constructor from a
// raw *sql.DB: a repository that can be built without a commune is a repository that can query
// across communes (rule 1, invariant 5).
func NewLoaiNhiemVuStore(db *store.DB) *LoaiNhiemVuStore { return &LoaiNhiemVuStore{db: db} }

// TranDanhMucLoaiNhiemVu is the hard upper bound on one commune's task-type catalogue.
//
// WHY A BOUND AT ALL WHEN THE LIST IS NOT PAGINATED: one process serves 200+ communes, so every
// list route is a shared resource and an unbounded one is forbidden. This route returns the WHOLE
// list by design — see DanhSach — so the bound cannot come from a `limit` parameter; it has to be a
// ceiling the commune's own data is measured against.
//
// 100 IS ABOUT FIFTY TIMES THE FIGURE IN THE SPECIFICATION, which lists two types
// (docs/ui-ux/14-cau-hinh.md §5). The number is not an estimate of how many types a commune might
// legitimately want; it is the point past which the rows have stopped being a catalogue — an import
// run twice, a loop that inserted rows, a test fixture on a live database.
const TranDanhMucLoaiNhiemVu = 100

// ErrQuaNhieuLoaiNhiemVu says the ceiling was reached. The caller answers 500 and refuses.
//
// WHY REFUSE RATHER THAN TRUNCATE, stated where the cost is paid: this list fills the type picker
// on the task form and the type filter on every task list. A silently short list is a type that has
// quietly disappeared from the picker — the task is filed under the wrong type, or the filter hides
// tasks that do exist, and every screen looks entirely normal. A refusal breaks ONE commune's
// screen loudly and names itself in the log. Between a wrong answer nobody notices and no answer
// somebody fixes, this system chooses the second (fail closed).
var ErrQuaNhieuLoaiNhiemVu = errors.New("loai_nhiem_vu: vượt trần danh mục")

// cotLoaiNhiemVu IS READ BY POSITION in DanhSach. `ma` and `nhan` are adjacent TEXT columns and
// `la_mac_dinh` / `dang_dung` adjacent BOOLEANs: swapping either pair — here or in the Scan —
// produces no error at all. The first pair shows slugs where names belong; the second pre-selects a
// disabled row on a form.
const cotLoaiNhiemVu = `id, ma, nhan, la_mac_dinh, dang_dung, thu_tu, nguon, ma_nguon_re_nhanh`

// DanhSach reads the commune's whole task-type catalogue, in the commune's own order.
//
// NOT PAGINATED, AND THAT IS A DECISION WITH A REASON, not an omission — the same one
// service-identity states on its org-chart and role catalogues:
//
//   - it is a CLOSED REFERENCE LIST of a handful of rows, not a register that grows with use;
//   - its consumers need the WHOLE list to be correct at all. A picker showing the first page of a
//     catalogue is a picker missing the entry somebody needs, and nothing on the screen says so;
//   - a client that must follow cursors to fill a dropdown will not, and the one that forgets
//     produces exactly that silent truncation.
//
// The bound pagination would have given is TranDanhMucLoaiNhiemVu, enforced in SQL.
//
// ORDER BY thu_tu, ma: `thu_tu` is the order the commune arranged its own catalogue in. The
// tie-break is `ma` and NOT `nhan`, which is where this deviates from service-identity's
// catalogues: `ma` carries UNIQUE (tenant_id, ma) in migration 0003, so the order is TOTAL —
// two rows can never compare equal and swap places between two calls. Labels carry no unique key
// and two rows may legitimately share one.
//
// ROWS OUT OF USE ARE RETURNED, soft-deleted rows are NOT (rule 7, invariant 2) — the split is
// argued on domain.LoaiNhiemVu.DangDung, and the partial index
// `loai_nhiem_vu_danh_sach (tenant_id, thu_tu) WHERE deleted_at IS NULL` exists for exactly this
// query.
//
// THE COMMUNE IS NOT A PARAMETER AND CANNOT BE ONE: Scoped.Query binds it to $1 from the context,
// so this can only ever read the catalogue of the commune the request arrived in (rule 1,
// invariant 4).
func (s *LoaiNhiemVuStore) DanhSach(ctx context.Context) ([]domain.LoaiNhiemVu, error) {
	// LIMIT is the ceiling PLUS ONE, which is what makes "there are too many" detectable at all.
	// Selecting exactly the ceiling returns a full page indistinguishable from a complete list of
	// that size — the truncation this route refuses to perform, performed by the bound meant to
	// prevent it.
	rows, err := s.db.For(ctx).Query(ctx, cotLoaiNhiemVu, "loai_nhiem_vu",
		`AND deleted_at IS NULL ORDER BY thu_tu, ma LIMIT $2`, TranDanhMucLoaiNhiemVu+1)
	if err != nil {
		return nil, fmt.Errorf("loai_nhiem_vu: đọc danh mục: %w", err)
	}
	defer rows.Close()

	ra := make([]domain.LoaiNhiemVu, 0, 8)
	for rows.Next() {
		var l domain.LoaiNhiemVu
		// POSITIONAL — in lockstep with cotLoaiNhiemVu. See the note there on the adjacent pairs.
		if err := rows.Scan(&l.ID, &l.Ma, &l.Nhan, &l.LaMacDinh, &l.DangDung,
			&l.ThuTu, &l.Nguon, &l.MaNguonReNhanh); err != nil {
			return nil, fmt.Errorf("loai_nhiem_vu: đọc dòng: %w", err)
		}
		ra = append(ra, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("loai_nhiem_vu: duyệt kết quả: %w", err)
	}
	if len(ra) > TranDanhMucLoaiNhiemVu {
		// The rows already read are DROPPED rather than trimmed and returned. Handing back a list
		// the caller might render anyway is how a refusal turns back into a silent truncation, one
		// careless `if err != nil { log }` later.
		return nil, ErrQuaNhieuLoaiNhiemVu
	}
	return ra, nil
}

// --- the write path -------------------------------------------------------------------------------
//
// FOUR THINGS HOLD ACROSS EVERY METHOD BELOW, and each one is a defect class rather than a style:
//
//  1. THE COMMUNE IS $1 IN EVERY STATEMENT, taken from tx.TenantID() which took it from the
//     context (rule 1, invariants 4 and 5). It is never a parameter of any method here, so a
//     caller cannot name another commune's row even by mistake.
//  2. `nguon` AND `ma_nguon_re_nhanh` ARE LITERALS IN THE INSERT AND APPEAR IN NO UPDATE. They
//     decide which tier a row is in; a bound parameter for either is a value that can come from a
//     client, and the migration says what follows — "every guard below could be stepped around by
//     setting nguon = 'don-vi' first".
//  3. EVERY READ EXCLUDES SOFT-DELETED ROWS except MaDaDung, which exists precisely to see them.
//  4. NOTHING HERE OPENS A TRANSACTION. The caller opens it and writes the audit entry inside it.

// docMotDong is the shared Scan of one row. Positional, in lockstep with cotLoaiNhiemVu — see the
// note there on the adjacent same-typed columns.
func docMotDongLoaiNhiemVu(quet func(...any) error) (domain.LoaiNhiemVu, error) {
	var lnv domain.LoaiNhiemVu
	err := quet(&lnv.ID, &lnv.Ma, &lnv.Nhan, &lnv.DangDung, &lnv.LaMacDinh,
		&lnv.ThuTu, &lnv.Nguon, &lnv.MaNguonReNhanh)
	return lnv, err
}

// TheoIDDeSua reads one live row inside the transaction and holds it until the transaction ends.
//
// `FOR UPDATE` IS THE POINT OF THIS METHOD AND NOT AN OPTIMISATION. Every write below is a
// read-decide-write: read the row, work out its tier, refuse or apply. Without the lock, two
// members of staff editing the same catalogue row both read the old state, both decide against it,
// and the second write silently overwrites the first — including the case where one of them was
// clearing `la_mac_dinh` and the other was setting it, which leaves the commune with a default
// nobody chose.
//
// IT ALSO EXCLUDES SOFT-DELETED ROWS, so "edit a row somebody deleted a second ago" is a 404 rather
// than a resurrection.
func (s *LoaiNhiemVuStore) TheoIDDeSua(ctx context.Context, tx *store.ScopedTx, id string) (domain.LoaiNhiemVu, error) {
	const stmt = `SELECT ` + cotLoaiNhiemVu + ` FROM loai_nhiem_vu ` +
		`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR UPDATE`

	lnv, err := docMotDongLoaiNhiemVu(tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), id).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.LoaiNhiemVu{}, ErrDanhMucKhongTonTai
	}
	if err != nil {
		return domain.LoaiNhiemVu{}, fmt.Errorf("loai_nhiem_vu: đọc dòng để sửa: %w", err)
	}
	return lnv, nil
}

// MaDaDung reports whether this code is already taken in this commune — INCLUDING soft-deleted rows.
//
// THE UNIQUE KEY IS THE REAL GUARD; THIS IS THE READABLE MESSAGE. Two concurrent creates of the
// same code can both pass this check and the second will then hit `UNIQUE (tenant_id, ma)` and roll
// the whole transaction back — no duplicate row, an unhelpful 500. That is the correct trade: the
// constraint never lets the duplicate exist, and this turns the ordinary case into a sentence
// somebody can act on.
func (s *LoaiNhiemVuStore) MaDaDung(ctx context.Context, tx *store.ScopedTx, ma string) (bool, error) {
	// `deleted_at` DELIBERATELY ABSENT FROM THE PREDICATE. See ErrMaDaTonTai: an issued code is
	// never reissued (rule 7, invariant 3), so a deleted row still owns its code.
	const stmt = `SELECT count(*) FROM loai_nhiem_vu WHERE tenant_id = $1 AND ma = $2`

	var n int
	if err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), ma).Scan(&n); err != nil {
		return false, fmt.Errorf("loai_nhiem_vu: kiểm tra mã trùng: %w", err)
	}
	return n > 0, nil
}

// DemDangSong counts the commune's live rows, for the ceiling check. Soft-deleted rows are excluded
// because DanhSach excludes them: the two numbers have to mean the same thing or the check guards
// the wrong quantity.
func (s *LoaiNhiemVuStore) DemDangSong(ctx context.Context, tx *store.ScopedTx) (int, error) {
	const stmt = `SELECT count(*) FROM loai_nhiem_vu WHERE tenant_id = $1 AND deleted_at IS NULL`

	var n int
	if err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID())).Scan(&n); err != nil {
		return 0, fmt.Errorf("loai_nhiem_vu: đếm dòng đang sống: %w", err)
	}
	return n, nil
}

// chenLoaiNhiemVu — `nguon` AND `ma_nguon_re_nhanh` ARE WRITTEN AS LITERALS AND ARE NOT PARAMETERS.
//
// Read that as the security property it is, not as a shortcut. There is no $n for either column, so
// there is no value a handler could pass and no field a client could fill: a commune's own row is
// tier 1, always, and the software's rows arrive by a path that does not exist in this repository
// yet (commune onboarding — see the migration header). Turning either into a parameter is the one
// edit that reopens the whole tier model, and it would look like tidying up.
const chenLoaiNhiemVu = `INSERT INTO loai_nhiem_vu
	(tenant_id, id, ma, nhan, thu_tu, la_mac_dinh, dang_dung, nguon, ma_nguon_re_nhanh)
	VALUES ($1, $2, $3, $4, $5, $6, $7, 'don-vi', false)`

// Chen adds one row the COMMUNE owns. There is no method here that writes a `he-thong` row.
func (s *LoaiNhiemVuStore) Chen(ctx context.Context, tx *store.ScopedTx, lnv domain.LoaiNhiemVu) error {
	if _, err := tx.Exec(ctx, chenLoaiNhiemVu, string(tx.TenantID()),
		lnv.ID, lnv.Ma, lnv.Nhan, lnv.ThuTu, lnv.LaMacDinh, lnv.DangDung); err != nil {
		return fmt.Errorf("loai_nhiem_vu: chèn: %w", err)
	}
	return nil
}

// BoMacDinhKhac clears the default flag on every other live row of this commune.
//
// WHY IT IS A SEPARATE STATEMENT AND MUST RUN IN THE SAME TRANSACTION: `UNIQUE (tenant_id,
// moc_mac_dinh)` admits exactly one live default, so setting a new one without clearing the old one
// fails the constraint. Run in a different transaction, the window between them is a commune with
// NO default, and the form pre-selects nothing while it lasts.
func (s *LoaiNhiemVuStore) BoMacDinhKhac(ctx context.Context, tx *store.ScopedTx, trongID string) error {
	const stmt = `UPDATE loai_nhiem_vu SET la_mac_dinh = false, cap_nhat_luc = now() ` +
		`WHERE tenant_id = $1 AND id <> $2 AND la_mac_dinh AND deleted_at IS NULL`

	if _, err := tx.Exec(ctx, stmt, string(tx.TenantID()), trongID); err != nil {
		return fmt.Errorf("loai_nhiem_vu: bỏ mặc định cũ: %w", err)
	}
	return nil
}

// capNhatLoaiNhiemVu — `ma`, `nguon` AND `ma_nguon_re_nhanh` APPEAR NOWHERE IN THIS STATEMENT.
//
// `ma` because an issued code is never renumbered (rule 7, invariant 3) and business records hold
// it as a value; the other two because they decide the tier. All three are also refused by the
// trigger, and both layers are meant: the trigger is the floor that holds against every writer, and
// their absence here is what makes the floor unreachable from this service in the first place.
const capNhatLoaiNhiemVu = `UPDATE loai_nhiem_vu ` +
	`SET nhan = $3, thu_tu = $4, dang_dung = $5, la_mac_dinh = $6, cap_nhat_luc = now() ` +
	`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

// CapNhat writes the four fields a commune may change. The caller has already read the row with
// TheoIDDeSua and decided the change is permitted at this row's tier.
func (s *LoaiNhiemVuStore) CapNhat(ctx context.Context, tx *store.ScopedTx, lnv domain.LoaiNhiemVu) error {
	kq, err := tx.Exec(ctx, capNhatLoaiNhiemVu, string(tx.TenantID()),
		lnv.ID, lnv.Nhan, lnv.ThuTu, lnv.DangDung, lnv.LaMacDinh)
	if err != nil {
		return fmt.Errorf("loai_nhiem_vu: cập nhật: %w", err)
	}
	return doiMotDong(kq, "cập nhật")
}

// xoaMemLoaiNhiemVu writes all THREE columns rule 7, invariant 1 names — `deleted_at`, `deleted_by`
// and `delete_reason` — in one statement, so none of them can be forgotten.
//
// `AND deleted_at IS NULL` IS WHAT MAKES A SECOND DELETE A 404 rather than a silent rewrite of who
// deleted the row and why. The first deletion is the one that happened; overwriting its reason
// would be editing a historical record (rule 7, forbidden #5).
const xoaMemLoaiNhiemVu = `UPDATE loai_nhiem_vu ` +
	`SET deleted_at = now(), deleted_by = $3, delete_reason = $4, cap_nhat_luc = now() ` +
	`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

// XoaMem soft deletes one row. THERE IS NO HARD DELETE ANYWHERE IN THIS PACKAGE, and the trigger
// refuses one even if somebody writes it (rule 7, forbidden #1).
func (s *LoaiNhiemVuStore) XoaMem(ctx context.Context, tx *store.ScopedTx, id, boi, lyDo string) error {
	kq, err := tx.Exec(ctx, xoaMemLoaiNhiemVu, string(tx.TenantID()), id, boi, lyDo)
	if err != nil {
		return fmt.Errorf("loai_nhiem_vu: xoá mềm: %w", err)
	}
	return doiMotDong(kq, "xoá mềm")
}
