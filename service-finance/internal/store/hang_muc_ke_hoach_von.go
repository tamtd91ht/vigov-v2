package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// HangMucKeHoachVonStore reads the commune's capital plan category catalogue.
//
// It holds *store.DB and never a *sql.DB. The commune is taken from the context on every call
// through db.For(ctx), so there is no constructor, no field and no method here that could produce
// a query without one (rule 1, invariant 5).
type HangMucKeHoachVonStore struct {
	db *store.DB
}

func NewHangMucKeHoachVonStore(db *store.DB) *HangMucKeHoachVonStore {
	return &HangMucKeHoachVonStore{db: db}
}

// TranDanhMucHangMuc is the hard upper bound on one commune's category catalogue.
//
// WHY THERE IS A BOUND AT ALL WHEN THE LIST IS NOT PAGINATED. One process serves 200+ communes, so
// every list route is a shared resource and an unbounded one is forbidden
// (skills/rest-api-design §5, FORBIDDEN #4). This route deliberately returns the WHOLE list — see
// DanhSach — so the bound cannot come from a `limit` parameter; it has to be a ceiling the
// commune's data is checked against.
//
// 200 IS ROUGHLY TWENTY-FIVE TIMES THE REAL FIGURE. These categories come from budget regulation
// and a commune carries a handful of them — the migration that creates the table says as much
// (0003_danh_muc_hang_muc_ke_hoach_von.sql, migration question 1). The number is not an estimate of
// how many there might be; it is the point past which the data is no longer a catalogue: an import
// run twice, a loop that inserted rows, a test fixture on a live database.
const TranDanhMucHangMuc = 200

// ErrQuaNhieuHangMuc says the ceiling was reached. The caller answers 500 and refuses.
//
// WHY REFUSE RATHER THAN TRUNCATE, stated where the cost is paid: this list fills the classifier on
// a capital plan line. A silently short list is a category that has quietly disappeared from that
// picker — the line gets classified under the wrong heading, or under none, and the screen looks
// entirely normal. Those figures are totalled and reported upward. A refusal breaks one commune's
// screen loudly and names itself in the log. Between a wrong answer nobody notices and no answer
// somebody fixes, this system chooses the second (fail closed).
var ErrQuaNhieuHangMuc = errors.New("hang_muc_ke_hoach_von: vượt trần danh mục")

// cotHangMuc IS READ BY POSITION in DanhSach, and this list has two adjacent pairs that swap
// without any error at all:
//
//	ma, nhan                 two TEXT columns — a swap shows slugs where labels belong. Visible.
//	la_mac_dinh, dang_dung   two BOOLEAN columns — a swap makes a row the commune TOOK OUT OF USE
//	                         the one the form pre-selects, and nothing anywhere says so. Invisible.
//
// The second pair is why this constant exists instead of the column list being written inline.
const cotHangMuc = `id, ma, nhan, la_mac_dinh, dang_dung, thu_tu, nguon, ma_nguon_re_nhanh`

// DanhSach reads the commune's whole capital plan category catalogue, ordered.
//
// NOT PAGINATED, AND THAT IS A DECISION, not an omission. The reasoning is the same one
// idstore.BoPhanStore.DanhSach sets out for the org chart, and it holds here for the same reasons:
//
//   - it is a CLOSED REFERENCE LIST of a handful of rows, not a growing register. Its size follows
//     budget regulation, not how long the commune has been using the system;
//   - its consumers need the WHOLE list to be correct at all — a picker showing the first page of
//     categories is a picker missing the category somebody needs, with nothing on the screen
//     saying so;
//   - a client that must follow cursors to fill a dropdown will not, and the one that forgets
//     produces exactly that silent truncation.
//
// The bound pagination would have provided is provided instead by TranDanhMucHangMuc, enforced in
// SQL — one row over the ceiling and this refuses rather than trimming.
//
// ROWS OUT OF USE ARE INCLUDED, SOFT-DELETED ROWS ARE NOT, and the two are different questions.
// `dang_dung = false` is a row the commune took out of use; the catalogue screen still lists it
// with a "Đã tắt" chip, and a plan line from an earlier budget year still holds its `ma`, so
// dropping it here would leave an existing figure with no category name. `deleted_at IS NOT NULL`
// is a row that must not appear on ANY read path — lists, statistics, search, reports, background
// jobs (rule 7, invariant 2). The predicate below filters the second and deliberately not the
// first; domain.HangMucKeHoachVon.DangDung is how the caller tells them apart.
//
// ORDER BY thu_tu, ma: `thu_tu` is the order the commune arranged its own catalogue in, and `ma`
// breaks ties. The tie-break is `ma` rather than `nhan` on purpose — UNIQUE (tenant_id, ma) makes
// the ordering TOTAL, while two rows may perfectly well carry the same label, and a tie left
// unresolved lets two rows swap places between two calls. An unstable order makes a client-side
// diff flicker on every reload and makes any test of this compare sets by accident.
//
// THE COMMUNE IS NOT A PARAMETER AND CANNOT BE ONE: Scoped.Query binds it to $1 from the context,
// so this can only ever read the catalogue of the commune the request arrived in (rule 1).
func (s *HangMucKeHoachVonStore) DanhSach(ctx context.Context) ([]domain.HangMucKeHoachVon, error) {
	// LIMIT is the ceiling PLUS ONE, which is what makes "there are too many" detectable at all.
	// Selecting exactly the ceiling would return a full page indistinguishable from a complete list
	// of exactly that size — the truncation this route refuses to perform, performed by the bound
	// meant to prevent it.
	rows, err := s.db.For(ctx).Query(ctx, cotHangMuc, "hang_muc_ke_hoach_von",
		`AND deleted_at IS NULL ORDER BY thu_tu, ma LIMIT $2`, TranDanhMucHangMuc+1)
	if err != nil {
		return nil, fmt.Errorf("hang_muc_ke_hoach_von: đọc danh mục: %w", err)
	}
	defer rows.Close()

	ra := make([]domain.HangMucKeHoachVon, 0, 16)
	for rows.Next() {
		var hm domain.HangMucKeHoachVon
		// POSITIONAL — in lockstep with cotHangMuc. See the note there on the two adjacent pairs.
		if err := rows.Scan(&hm.ID, &hm.Ma, &hm.Nhan, &hm.LaMacDinh, &hm.DangDung,
			&hm.ThuTu, &hm.Nguon, &hm.MaNguonReNhanh); err != nil {
			return nil, fmt.Errorf("hang_muc_ke_hoach_von: đọc dòng: %w", err)
		}
		ra = append(ra, hm)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("hang_muc_ke_hoach_von: duyệt kết quả: %w", err)
	}
	if len(ra) > TranDanhMucHangMuc {
		// The rows already read are DROPPED rather than trimmed and returned. Handing back a list
		// the caller might render anyway is how a refusal turns back into a silent truncation one
		// careless `if err != nil { log }` later.
		return nil, ErrQuaNhieuHangMuc
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

// docMotDong is the shared Scan of one row. Positional, in lockstep with cotHangMuc — see the
// note there on the adjacent same-typed columns.
func docMotDongHangMuc(quet func(...any) error) (domain.HangMucKeHoachVon, error) {
	var hm domain.HangMucKeHoachVon
	err := quet(&hm.ID, &hm.Ma, &hm.Nhan, &hm.DangDung, &hm.LaMacDinh,
		&hm.ThuTu, &hm.Nguon, &hm.MaNguonReNhanh)
	return hm, err
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
func (s *HangMucKeHoachVonStore) TheoIDDeSua(ctx context.Context, tx *store.ScopedTx, id string) (domain.HangMucKeHoachVon, error) {
	const stmt = `SELECT ` + cotHangMuc + ` FROM hang_muc_ke_hoach_von ` +
		`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR UPDATE`

	hm, err := docMotDongHangMuc(tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), id).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.HangMucKeHoachVon{}, ErrDanhMucKhongTonTai
	}
	if err != nil {
		return domain.HangMucKeHoachVon{}, fmt.Errorf("hang_muc_ke_hoach_von: đọc dòng để sửa: %w", err)
	}
	return hm, nil
}

// MaDaDung reports whether this code is already taken in this commune — INCLUDING soft-deleted rows.
//
// THE UNIQUE KEY IS THE REAL GUARD; THIS IS THE READABLE MESSAGE. Two concurrent creates of the
// same code can both pass this check and the second will then hit `UNIQUE (tenant_id, ma)` and roll
// the whole transaction back — no duplicate row, an unhelpful 500. That is the correct trade: the
// constraint never lets the duplicate exist, and this turns the ordinary case into a sentence
// somebody can act on.
func (s *HangMucKeHoachVonStore) MaDaDung(ctx context.Context, tx *store.ScopedTx, ma string) (bool, error) {
	// `deleted_at` DELIBERATELY ABSENT FROM THE PREDICATE. See ErrMaDaTonTai: an issued code is
	// never reissued (rule 7, invariant 3), so a deleted row still owns its code.
	const stmt = `SELECT count(*) FROM hang_muc_ke_hoach_von WHERE tenant_id = $1 AND ma = $2`

	var n int
	if err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), ma).Scan(&n); err != nil {
		return false, fmt.Errorf("hang_muc_ke_hoach_von: kiểm tra mã trùng: %w", err)
	}
	return n > 0, nil
}

// DemDangSong counts the commune's live rows, for the ceiling check. Soft-deleted rows are excluded
// because DanhSach excludes them: the two numbers have to mean the same thing or the check guards
// the wrong quantity.
func (s *HangMucKeHoachVonStore) DemDangSong(ctx context.Context, tx *store.ScopedTx) (int, error) {
	const stmt = `SELECT count(*) FROM hang_muc_ke_hoach_von WHERE tenant_id = $1 AND deleted_at IS NULL`

	var n int
	if err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID())).Scan(&n); err != nil {
		return 0, fmt.Errorf("hang_muc_ke_hoach_von: đếm dòng đang sống: %w", err)
	}
	return n, nil
}

// chenHangMuc — `nguon` AND `ma_nguon_re_nhanh` ARE WRITTEN AS LITERALS AND ARE NOT PARAMETERS.
//
// Read that as the security property it is, not as a shortcut. There is no $n for either column, so
// there is no value a handler could pass and no field a client could fill: a commune's own row is
// tier 1, always, and the software's rows arrive by a path that does not exist in this repository
// yet (commune onboarding — see the migration header). Turning either into a parameter is the one
// edit that reopens the whole tier model, and it would look like tidying up.
const chenHangMuc = `INSERT INTO hang_muc_ke_hoach_von
	(tenant_id, id, ma, nhan, thu_tu, la_mac_dinh, dang_dung, nguon, ma_nguon_re_nhanh)
	VALUES ($1, $2, $3, $4, $5, $6, $7, 'don-vi', false)`

// Chen adds one row the COMMUNE owns. There is no method here that writes a `he-thong` row.
func (s *HangMucKeHoachVonStore) Chen(ctx context.Context, tx *store.ScopedTx, hm domain.HangMucKeHoachVon) error {
	if _, err := tx.Exec(ctx, chenHangMuc, string(tx.TenantID()),
		hm.ID, hm.Ma, hm.Nhan, hm.ThuTu, hm.LaMacDinh, hm.DangDung); err != nil {
		return fmt.Errorf("hang_muc_ke_hoach_von: chèn: %w", err)
	}
	return nil
}

// BoMacDinhKhac clears the default flag on every other live row of this commune.
//
// WHY IT IS A SEPARATE STATEMENT AND MUST RUN IN THE SAME TRANSACTION: `UNIQUE (tenant_id,
// moc_mac_dinh)` admits exactly one live default, so setting a new one without clearing the old one
// fails the constraint. Run in a different transaction, the window between them is a commune with
// NO default, and the form pre-selects nothing while it lasts.
func (s *HangMucKeHoachVonStore) BoMacDinhKhac(ctx context.Context, tx *store.ScopedTx, trongID string) error {
	const stmt = `UPDATE hang_muc_ke_hoach_von SET la_mac_dinh = false, cap_nhat_luc = now() ` +
		`WHERE tenant_id = $1 AND id <> $2 AND la_mac_dinh AND deleted_at IS NULL`

	if _, err := tx.Exec(ctx, stmt, string(tx.TenantID()), trongID); err != nil {
		return fmt.Errorf("hang_muc_ke_hoach_von: bỏ mặc định cũ: %w", err)
	}
	return nil
}

// capNhatHangMuc — `ma`, `nguon` AND `ma_nguon_re_nhanh` APPEAR NOWHERE IN THIS STATEMENT.
//
// `ma` because an issued code is never renumbered (rule 7, invariant 3) and business records hold
// it as a value; the other two because they decide the tier. All three are also refused by the
// trigger, and both layers are meant: the trigger is the floor that holds against every writer, and
// their absence here is what makes the floor unreachable from this service in the first place.
const capNhatHangMuc = `UPDATE hang_muc_ke_hoach_von ` +
	`SET nhan = $3, thu_tu = $4, dang_dung = $5, la_mac_dinh = $6, cap_nhat_luc = now() ` +
	`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

// CapNhat writes the four fields a commune may change. The caller has already read the row with
// TheoIDDeSua and decided the change is permitted at this row's tier.
func (s *HangMucKeHoachVonStore) CapNhat(ctx context.Context, tx *store.ScopedTx, hm domain.HangMucKeHoachVon) error {
	kq, err := tx.Exec(ctx, capNhatHangMuc, string(tx.TenantID()),
		hm.ID, hm.Nhan, hm.ThuTu, hm.DangDung, hm.LaMacDinh)
	if err != nil {
		return fmt.Errorf("hang_muc_ke_hoach_von: cập nhật: %w", err)
	}
	return doiMotDong(kq, "cập nhật")
}

// xoaMemHangMuc writes all THREE columns rule 7, invariant 1 names — `deleted_at`, `deleted_by`
// and `delete_reason` — in one statement, so none of them can be forgotten.
//
// `AND deleted_at IS NULL` IS WHAT MAKES A SECOND DELETE A 404 rather than a silent rewrite of who
// deleted the row and why. The first deletion is the one that happened; overwriting its reason
// would be editing a historical record (rule 7, forbidden #5).
const xoaMemHangMuc = `UPDATE hang_muc_ke_hoach_von ` +
	`SET deleted_at = now(), deleted_by = $3, delete_reason = $4, cap_nhat_luc = now() ` +
	`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

// XoaMem soft deletes one row. THERE IS NO HARD DELETE ANYWHERE IN THIS PACKAGE, and the trigger
// refuses one even if somebody writes it (rule 7, forbidden #1).
func (s *HangMucKeHoachVonStore) XoaMem(ctx context.Context, tx *store.ScopedTx, id, boi, lyDo string) error {
	kq, err := tx.Exec(ctx, xoaMemHangMuc, string(tx.TenantID()), id, boi, lyDo)
	if err != nil {
		return fmt.Errorf("hang_muc_ke_hoach_von: xoá mềm: %w", err)
	}
	return doiMotDong(kq, "xoá mềm")
}
