package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// MucUuTienNhiemVuStore reads the commune's task-priority scale (`muc_uu_tien_nhiem_vu`,
// migration 0003).
//
// A SEPARATE STORE FROM LoaiNhiemVuStore, although the two tables have the same shape and the two
// queries differ in one identifier. They are two catalogues with two ceilings and two failure
// modes, and a shared reader would mean one `bang string` parameter deciding which commune
// catalogue is read — a table name arriving from the caller is the shape rule 1, forbidden #2
// warns about in its SQL form. The duplication is a dozen lines; the alternative is a query that
// can be pointed at any table in the schema.
type MucUuTienNhiemVuStore struct {
	db *store.DB
}

// NewMucUuTienNhiemVuStore takes *store.DB and nothing else — see NewLoaiNhiemVuStore.
func NewMucUuTienNhiemVuStore(db *store.DB) *MucUuTienNhiemVuStore {
	return &MucUuTienNhiemVuStore{db: db}
}

// TranDanhMucMucUuTien is the hard upper bound on one commune's priority scale.
//
// DELIBERATELY SMALLER THAN TranDanhMucLoaiNhiemVu, because this is a SCALE. The specification
// ships three levels (docs/ui-ux/14-cau-hinh.md §5), and a scale is only usable while a person can
// hold all of it in their head at once: fifty levels is not a large commune, it is a list where
// nobody can say whether one task outranks another. The ceiling marks the point past which the rows
// have stopped being a scale, not how many levels a commune might reasonably want.
const TranDanhMucMucUuTien = 50

// ErrQuaNhieuMucUuTien says the ceiling was reached. The caller answers 500 and refuses.
//
// REFUSE RATHER THAN TRUNCATE, for a reason one step worse than the type catalogue's: the levels
// arrive in rank order, so a truncated list does not merely lose an option — it loses the options
// at ONE END of the scale. Whichever end that is, the picker then offers a scale that silently
// stops short, and every task filed from it is ranked wrong.
var ErrQuaNhieuMucUuTien = errors.New("muc_uu_tien_nhiem_vu: vượt trần danh mục")

// cotMucUuTien IS READ BY POSITION in DanhSach — same note as cotLoaiNhiemVu: `ma`/`nhan` are
// adjacent TEXT columns and `la_mac_dinh`/`dang_dung` adjacent BOOLEANs, and swapping either pair
// raises no error anywhere.
const cotMucUuTien = `id, ma, nhan, la_mac_dinh, dang_dung, thu_tu, nguon, ma_nguon_re_nhanh`

// DanhSach reads the commune's whole priority scale, IN RANK ORDER.
//
// THE ORDER IS THE MEANING OF THIS LIST, not a presentation detail. "Khẩn" is urgent only relative
// to the levels around it, so a scale rendered in the wrong order is wrong in a way nobody reports
// as a bug: every screen still shows three plausible words. ORDER BY thu_tu is what produces the
// rank; the tie-break is `ma`, which carries UNIQUE (tenant_id, ma) in migration 0003 and therefore
// makes the order TOTAL — two levels sharing a `thu_tu` cannot swap places between two calls, and
// a test of this cannot accidentally compare sets instead of sequences.
//
// Not paginated, for the reasons written on LoaiNhiemVuStore.DanhSach; the bound is
// TranDanhMucMucUuTien, enforced in SQL. Rows out of use are returned, soft-deleted rows are not
// (rule 7, invariant 2).
//
// THE COMMUNE IS NOT A PARAMETER AND CANNOT BE ONE: Scoped.Query binds it to $1 from the context.
func (s *MucUuTienNhiemVuStore) DanhSach(ctx context.Context) ([]domain.MucUuTienNhiemVu, error) {
	// Ceiling PLUS ONE — a full page of exactly the ceiling is indistinguishable from a complete
	// list of that size, which is the truncation this refuses to perform.
	rows, err := s.db.For(ctx).Query(ctx, cotMucUuTien, "muc_uu_tien_nhiem_vu",
		`AND deleted_at IS NULL ORDER BY thu_tu, ma LIMIT $2`, TranDanhMucMucUuTien+1)
	if err != nil {
		return nil, fmt.Errorf("muc_uu_tien_nhiem_vu: đọc danh mục: %w", err)
	}
	defer rows.Close()

	ra := make([]domain.MucUuTienNhiemVu, 0, 8)
	for rows.Next() {
		var m domain.MucUuTienNhiemVu
		// POSITIONAL — in lockstep with cotMucUuTien.
		if err := rows.Scan(&m.ID, &m.Ma, &m.Nhan, &m.LaMacDinh, &m.DangDung,
			&m.ThuTu, &m.Nguon, &m.MaNguonReNhanh); err != nil {
			return nil, fmt.Errorf("muc_uu_tien_nhiem_vu: đọc dòng: %w", err)
		}
		ra = append(ra, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("muc_uu_tien_nhiem_vu: duyệt kết quả: %w", err)
	}
	if len(ra) > TranDanhMucMucUuTien {
		// Rows already read are DROPPED rather than trimmed and returned — see the same note on
		// LoaiNhiemVuStore.DanhSach.
		return nil, ErrQuaNhieuMucUuTien
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

// docMotDong is the shared Scan of one row. Positional, in lockstep with cotMucUuTien — see the
// note there on the adjacent same-typed columns.
func docMotDongMucUuTien(quet func(...any) error) (domain.MucUuTienNhiemVu, error) {
	var muu domain.MucUuTienNhiemVu
	err := quet(&muu.ID, &muu.Ma, &muu.Nhan, &muu.DangDung, &muu.LaMacDinh,
		&muu.ThuTu, &muu.Nguon, &muu.MaNguonReNhanh)
	return muu, err
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
func (s *MucUuTienNhiemVuStore) TheoIDDeSua(ctx context.Context, tx *store.ScopedTx, id string) (domain.MucUuTienNhiemVu, error) {
	const stmt = `SELECT ` + cotMucUuTien + ` FROM muc_uu_tien_nhiem_vu ` +
		`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR UPDATE`

	muu, err := docMotDongMucUuTien(tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), id).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.MucUuTienNhiemVu{}, ErrDanhMucKhongTonTai
	}
	if err != nil {
		return domain.MucUuTienNhiemVu{}, fmt.Errorf("muc_uu_tien_nhiem_vu: đọc dòng để sửa: %w", err)
	}
	return muu, nil
}

// MaDaDung reports whether this code is already taken in this commune — INCLUDING soft-deleted rows.
//
// THE UNIQUE KEY IS THE REAL GUARD; THIS IS THE READABLE MESSAGE. Two concurrent creates of the
// same code can both pass this check and the second will then hit `UNIQUE (tenant_id, ma)` and roll
// the whole transaction back — no duplicate row, an unhelpful 500. That is the correct trade: the
// constraint never lets the duplicate exist, and this turns the ordinary case into a sentence
// somebody can act on.
func (s *MucUuTienNhiemVuStore) MaDaDung(ctx context.Context, tx *store.ScopedTx, ma string) (bool, error) {
	// `deleted_at` DELIBERATELY ABSENT FROM THE PREDICATE. See ErrMaDaTonTai: an issued code is
	// never reissued (rule 7, invariant 3), so a deleted row still owns its code.
	const stmt = `SELECT count(*) FROM muc_uu_tien_nhiem_vu WHERE tenant_id = $1 AND ma = $2`

	var n int
	if err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), ma).Scan(&n); err != nil {
		return false, fmt.Errorf("muc_uu_tien_nhiem_vu: kiểm tra mã trùng: %w", err)
	}
	return n > 0, nil
}

// DemDangSong counts the commune's live rows, for the ceiling check. Soft-deleted rows are excluded
// because DanhSach excludes them: the two numbers have to mean the same thing or the check guards
// the wrong quantity.
func (s *MucUuTienNhiemVuStore) DemDangSong(ctx context.Context, tx *store.ScopedTx) (int, error) {
	const stmt = `SELECT count(*) FROM muc_uu_tien_nhiem_vu WHERE tenant_id = $1 AND deleted_at IS NULL`

	var n int
	if err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID())).Scan(&n); err != nil {
		return 0, fmt.Errorf("muc_uu_tien_nhiem_vu: đếm dòng đang sống: %w", err)
	}
	return n, nil
}

// chenMucUuTien — `nguon` AND `ma_nguon_re_nhanh` ARE WRITTEN AS LITERALS AND ARE NOT PARAMETERS.
//
// Read that as the security property it is, not as a shortcut. There is no $n for either column, so
// there is no value a handler could pass and no field a client could fill: a commune's own row is
// tier 1, always, and the software's rows arrive by a path that does not exist in this repository
// yet (commune onboarding — see the migration header). Turning either into a parameter is the one
// edit that reopens the whole tier model, and it would look like tidying up.
const chenMucUuTien = `INSERT INTO muc_uu_tien_nhiem_vu
	(tenant_id, id, ma, nhan, thu_tu, la_mac_dinh, dang_dung, nguon, ma_nguon_re_nhanh)
	VALUES ($1, $2, $3, $4, $5, $6, $7, 'don-vi', false)`

// Chen adds one row the COMMUNE owns. There is no method here that writes a `he-thong` row.
func (s *MucUuTienNhiemVuStore) Chen(ctx context.Context, tx *store.ScopedTx, muu domain.MucUuTienNhiemVu) error {
	if _, err := tx.Exec(ctx, chenMucUuTien, string(tx.TenantID()),
		muu.ID, muu.Ma, muu.Nhan, muu.ThuTu, muu.LaMacDinh, muu.DangDung); err != nil {
		return fmt.Errorf("muc_uu_tien_nhiem_vu: chèn: %w", err)
	}
	return nil
}

// BoMacDinhKhac clears the default flag on every other live row of this commune.
//
// WHY IT IS A SEPARATE STATEMENT AND MUST RUN IN THE SAME TRANSACTION: `UNIQUE (tenant_id,
// moc_mac_dinh)` admits exactly one live default, so setting a new one without clearing the old one
// fails the constraint. Run in a different transaction, the window between them is a commune with
// NO default, and the form pre-selects nothing while it lasts.
func (s *MucUuTienNhiemVuStore) BoMacDinhKhac(ctx context.Context, tx *store.ScopedTx, trongID string) error {
	const stmt = `UPDATE muc_uu_tien_nhiem_vu SET la_mac_dinh = false, cap_nhat_luc = now() ` +
		`WHERE tenant_id = $1 AND id <> $2 AND la_mac_dinh AND deleted_at IS NULL`

	if _, err := tx.Exec(ctx, stmt, string(tx.TenantID()), trongID); err != nil {
		return fmt.Errorf("muc_uu_tien_nhiem_vu: bỏ mặc định cũ: %w", err)
	}
	return nil
}

// capNhatMucUuTien — `ma`, `nguon` AND `ma_nguon_re_nhanh` APPEAR NOWHERE IN THIS STATEMENT.
//
// `ma` because an issued code is never renumbered (rule 7, invariant 3) and business records hold
// it as a value; the other two because they decide the tier. All three are also refused by the
// trigger, and both layers are meant: the trigger is the floor that holds against every writer, and
// their absence here is what makes the floor unreachable from this service in the first place.
const capNhatMucUuTien = `UPDATE muc_uu_tien_nhiem_vu ` +
	`SET nhan = $3, thu_tu = $4, dang_dung = $5, la_mac_dinh = $6, cap_nhat_luc = now() ` +
	`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

// CapNhat writes the four fields a commune may change. The caller has already read the row with
// TheoIDDeSua and decided the change is permitted at this row's tier.
func (s *MucUuTienNhiemVuStore) CapNhat(ctx context.Context, tx *store.ScopedTx, muu domain.MucUuTienNhiemVu) error {
	kq, err := tx.Exec(ctx, capNhatMucUuTien, string(tx.TenantID()),
		muu.ID, muu.Nhan, muu.ThuTu, muu.DangDung, muu.LaMacDinh)
	if err != nil {
		return fmt.Errorf("muc_uu_tien_nhiem_vu: cập nhật: %w", err)
	}
	return doiMotDong(kq, "cập nhật")
}

// xoaMemMucUuTien writes all THREE columns rule 7, invariant 1 names — `deleted_at`, `deleted_by`
// and `delete_reason` — in one statement, so none of them can be forgotten.
//
// `AND deleted_at IS NULL` IS WHAT MAKES A SECOND DELETE A 404 rather than a silent rewrite of who
// deleted the row and why. The first deletion is the one that happened; overwriting its reason
// would be editing a historical record (rule 7, forbidden #5).
const xoaMemMucUuTien = `UPDATE muc_uu_tien_nhiem_vu ` +
	`SET deleted_at = now(), deleted_by = $3, delete_reason = $4, cap_nhat_luc = now() ` +
	`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

// XoaMem soft deletes one row. THERE IS NO HARD DELETE ANYWHERE IN THIS PACKAGE, and the trigger
// refuses one even if somebody writes it (rule 7, forbidden #1).
func (s *MucUuTienNhiemVuStore) XoaMem(ctx context.Context, tx *store.ScopedTx, id, boi, lyDo string) error {
	kq, err := tx.Exec(ctx, xoaMemMucUuTien, string(tx.TenantID()), id, boi, lyDo)
	if err != nil {
		return fmt.Errorf("muc_uu_tien_nhiem_vu: xoá mềm: %w", err)
	}
	return doiMotDong(kq, "xoá mềm")
}
