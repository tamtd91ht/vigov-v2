package store

import (
	"context"
	"database/sql"
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
//
// `color` (migration 0020) is NULLABLE and read through sql.NullString: NULL is "no colour chosen".
const cotLoaiTaiNguyen = `id, ma, nhan, thu_tu, la_mac_dinh, dang_dung, nguon, ma_nguon_re_nhanh, color`

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
		// POSITIONAL — in lockstep with cotLoaiTaiNguyen. See the note there on the two adjacent
		// pairs of same-typed columns.
		lt, err := docMotDongLoaiTaiNguyen(rows.Scan)
		if err != nil {
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

// docMotDong is the shared Scan of one row. Positional, in lockstep with cotLoaiTaiNguyen — see the
// note there on the adjacent same-typed columns.
//
// FIXED 2026-09-29: this scanned `dang_dung, la_mac_dinh, thu_tu` where cotLoaiTaiNguyen selects
// `thu_tu, la_mac_dinh, dang_dung`. On PostgreSQL an INT into a bool and a bool into an int fail the
// Scan, so every PATCH and DELETE of /api/v1/map-asset-types answered 500. The app suite's fake
// driver mirrored the SAME wrong order, so it stayed green; it now mirrors cotLoaiTaiNguyen.
func docMotDongLoaiTaiNguyen(quet func(...any) error) (domain.LoaiTaiNguyenBanDo, error) {
	var ltn domain.LoaiTaiNguyenBanDo
	var color sql.NullString
	err := quet(&ltn.ID, &ltn.Ma, &ltn.Nhan, &ltn.ThuTu, &ltn.LaMacDinh,
		&ltn.DangDung, &ltn.Nguon, &ltn.MaNguonReNhanh, &color)
	ltn.Color = color.String
	return ltn, err
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
func (s *LoaiTaiNguyenBanDoStore) TheoIDDeSua(ctx context.Context, tx *store.ScopedTx, id string) (domain.LoaiTaiNguyenBanDo, error) {
	const stmt = `SELECT ` + cotLoaiTaiNguyen + ` FROM loai_tai_nguyen_ban_do ` +
		`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR UPDATE`

	ltn, err := docMotDongLoaiTaiNguyen(tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), id).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.LoaiTaiNguyenBanDo{}, ErrDanhMucKhongTonTai
	}
	if err != nil {
		return domain.LoaiTaiNguyenBanDo{}, fmt.Errorf("loai_tai_nguyen_ban_do: đọc dòng để sửa: %w", err)
	}
	return ltn, nil
}

// MaDaDung reports whether this code is already taken in this commune — INCLUDING soft-deleted rows.
//
// THE UNIQUE KEY IS THE REAL GUARD; THIS IS THE READABLE MESSAGE. Two concurrent creates of the
// same code can both pass this check and the second will then hit `UNIQUE (tenant_id, ma)` and roll
// the whole transaction back — no duplicate row, an unhelpful 500. That is the correct trade: the
// constraint never lets the duplicate exist, and this turns the ordinary case into a sentence
// somebody can act on.
func (s *LoaiTaiNguyenBanDoStore) MaDaDung(ctx context.Context, tx *store.ScopedTx, ma string) (bool, error) {
	// `deleted_at` DELIBERATELY ABSENT FROM THE PREDICATE. See ErrMaDaTonTai: an issued code is
	// never reissued (rule 7, invariant 3), so a deleted row still owns its code.
	const stmt = `SELECT count(*) FROM loai_tai_nguyen_ban_do WHERE tenant_id = $1 AND ma = $2`

	var n int
	if err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), ma).Scan(&n); err != nil {
		return false, fmt.Errorf("loai_tai_nguyen_ban_do: kiểm tra mã trùng: %w", err)
	}
	return n > 0, nil
}

// DemDangSong counts the commune's live rows, for the ceiling check. Soft-deleted rows are excluded
// because DanhSach excludes them: the two numbers have to mean the same thing or the check guards
// the wrong quantity.
func (s *LoaiTaiNguyenBanDoStore) DemDangSong(ctx context.Context, tx *store.ScopedTx) (int, error) {
	const stmt = `SELECT count(*) FROM loai_tai_nguyen_ban_do WHERE tenant_id = $1 AND deleted_at IS NULL`

	var n int
	if err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID())).Scan(&n); err != nil {
		return 0, fmt.Errorf("loai_tai_nguyen_ban_do: đếm dòng đang sống: %w", err)
	}
	return n, nil
}

// chenLoaiTaiNguyen — `nguon` AND `ma_nguon_re_nhanh` ARE WRITTEN AS LITERALS AND ARE NOT PARAMETERS.
//
// Read that as the security property it is, not as a shortcut. There is no $n for either column, so
// there is no value a handler could pass and no field a client could fill: a commune's own row is
// tier 1, always, and the software's rows arrive by a path that does not exist in this repository
// yet (commune onboarding — see the migration header). Turning either into a parameter is the one
// edit that reopens the whole tier model, and it would look like tidying up.
const chenLoaiTaiNguyen = `INSERT INTO loai_tai_nguyen_ban_do
	(tenant_id, id, ma, nhan, thu_tu, la_mac_dinh, dang_dung, nguon, ma_nguon_re_nhanh, color)
	VALUES ($1, $2, $3, $4, $5, $6, $7, 'don-vi', false, NULLIF($8, ''))`

// Chen adds one row the COMMUNE owns. There is no method here that writes a `he-thong` row. An empty
// Color is stored as NULL ("no colour chosen", 0020).
func (s *LoaiTaiNguyenBanDoStore) Chen(ctx context.Context, tx *store.ScopedTx, ltn domain.LoaiTaiNguyenBanDo) error {
	if _, err := tx.Exec(ctx, chenLoaiTaiNguyen, string(tx.TenantID()),
		ltn.ID, ltn.Ma, ltn.Nhan, ltn.ThuTu, ltn.LaMacDinh, ltn.DangDung, ltn.Color); err != nil {
		return fmt.Errorf("loai_tai_nguyen_ban_do: chèn: %w", err)
	}
	return nil
}

// BoMacDinhKhac clears the default flag on every other live row of this commune.
//
// WHY IT IS A SEPARATE STATEMENT AND MUST RUN IN THE SAME TRANSACTION: `UNIQUE (tenant_id,
// moc_mac_dinh)` admits exactly one live default, so setting a new one without clearing the old one
// fails the constraint. Run in a different transaction, the window between them is a commune with
// NO default, and the form pre-selects nothing while it lasts.
func (s *LoaiTaiNguyenBanDoStore) BoMacDinhKhac(ctx context.Context, tx *store.ScopedTx, trongID string) error {
	const stmt = `UPDATE loai_tai_nguyen_ban_do SET la_mac_dinh = false, cap_nhat_luc = now() ` +
		`WHERE tenant_id = $1 AND id <> $2 AND la_mac_dinh AND deleted_at IS NULL`

	if _, err := tx.Exec(ctx, stmt, string(tx.TenantID()), trongID); err != nil {
		return fmt.Errorf("loai_tai_nguyen_ban_do: bỏ mặc định cũ: %w", err)
	}
	return nil
}

// capNhatLoaiTaiNguyen — `ma`, `nguon` AND `ma_nguon_re_nhanh` APPEAR NOWHERE IN THIS STATEMENT.
//
// `ma` because an issued code is never renumbered (rule 7, invariant 3) and business records hold
// it as a value; the other two because they decide the tier. All three are also refused by the
// trigger, and both layers are meant: the trigger is the floor that holds against every writer, and
// their absence here is what makes the floor unreachable from this service in the first place.
const capNhatLoaiTaiNguyen = `UPDATE loai_tai_nguyen_ban_do ` +
	`SET nhan = $3, thu_tu = $4, dang_dung = $5, la_mac_dinh = $6, color = NULLIF($7, ''), cap_nhat_luc = now() ` +
	`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

// CapNhat writes the five fields a commune may change (the colour since 0020 — presentation, every
// tier). The caller has already read the row with TheoIDDeSua and decided the change is permitted at
// this row's tier.
func (s *LoaiTaiNguyenBanDoStore) CapNhat(ctx context.Context, tx *store.ScopedTx, ltn domain.LoaiTaiNguyenBanDo) error {
	kq, err := tx.Exec(ctx, capNhatLoaiTaiNguyen, string(tx.TenantID()),
		ltn.ID, ltn.Nhan, ltn.ThuTu, ltn.DangDung, ltn.LaMacDinh, ltn.Color)
	if err != nil {
		return fmt.Errorf("loai_tai_nguyen_ban_do: cập nhật: %w", err)
	}
	return doiMotDong(kq, "cập nhật")
}

// xoaMemLoaiTaiNguyen writes all THREE columns rule 7, invariant 1 names — `deleted_at`, `deleted_by`
// and `delete_reason` — in one statement, so none of them can be forgotten.
//
// `AND deleted_at IS NULL` IS WHAT MAKES A SECOND DELETE A 404 rather than a silent rewrite of who
// deleted the row and why. The first deletion is the one that happened; overwriting its reason
// would be editing a historical record (rule 7, forbidden #5).
const xoaMemLoaiTaiNguyen = `UPDATE loai_tai_nguyen_ban_do ` +
	`SET deleted_at = now(), deleted_by = $3, delete_reason = $4, cap_nhat_luc = now() ` +
	`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

// MaxImportSnapshotRows bounds the import snapshot: live rows are capped at TranDanhMucLoaiTaiNguyen,
// and ten times that leaves room for a long history of soft-deleted rows. Past it the import REFUSES
// — a snapshot missing rows would miss a taken code, and the plan would promise a row the unique key
// then refuses.
const MaxImportSnapshotRows = 10 * TranDanhMucLoaiTaiNguyen

// ErrImportSnapshotTooLarge — the commune's catalogue history exceeds MaxImportSnapshotRows.
var ErrImportSnapshotTooLarge = errors.New("loai_tai_nguyen_ban_do: ảnh chụp danh mục vượt trần")

// ImportSnapshot reads EVERY row of this commune's catalogue — soft-deleted ones INCLUDED and flagged —
// inside the import's transaction. Deleted rows are there because their codes are still taken (rule
// 7, invariant 3); the planner never treats them as live for the label check.
func (s *LoaiTaiNguyenBanDoStore) ImportSnapshot(ctx context.Context, tx *store.ScopedTx) (
	[]domain.ExistingMapAssetType, error) {

	const stmt = `SELECT ma, nhan, deleted_at IS NOT NULL FROM loai_tai_nguyen_ban_do ` +
		`WHERE tenant_id = $1 ORDER BY ma LIMIT $2`
	rows, err := tx.Underlying().QueryContext(ctx, stmt, string(tx.TenantID()), MaxImportSnapshotRows+1)
	if err != nil {
		return nil, fmt.Errorf("loai_tai_nguyen_ban_do: đọc ảnh chụp để nhập: %w", err)
	}
	defer rows.Close()
	out := make([]domain.ExistingMapAssetType, 0, 16)
	for rows.Next() {
		var e domain.ExistingMapAssetType
		if err := rows.Scan(&e.Code, &e.Label, &e.Deleted); err != nil {
			return nil, fmt.Errorf("loai_tai_nguyen_ban_do: đọc dòng ảnh chụp: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("loai_tai_nguyen_ban_do: duyệt ảnh chụp: %w", err)
	}
	if len(out) > MaxImportSnapshotRows {
		return nil, ErrImportSnapshotTooLarge
	}
	return out, nil
}

// XoaMem soft deletes one row. THERE IS NO HARD DELETE ANYWHERE IN THIS PACKAGE, and the trigger
// refuses one even if somebody writes it (rule 7, forbidden #1).
func (s *LoaiTaiNguyenBanDoStore) XoaMem(ctx context.Context, tx *store.ScopedTx, id, boi, lyDo string) error {
	kq, err := tx.Exec(ctx, xoaMemLoaiTaiNguyen, string(tx.TenantID()), id, boi, lyDo)
	if err != nil {
		return fmt.Errorf("loai_tai_nguyen_ban_do: xoá mềm: %w", err)
	}
	return doiMotDong(kq, "xoá mềm")
}
