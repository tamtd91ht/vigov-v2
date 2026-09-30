package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-documents/internal/domain"
)

// LoaiVanBanStore reads and writes the commune's document-type catalogue.
//
// Built from *store.DB, which only hands out commune-scoped access: there is no constructor here
// taking a raw *sql.DB, because a repository that can be built without a commune is a repository
// that can query across communes (rule 1, invariant 5).
//
// EVERY MUTATING METHOD TAKES A *store.ScopedTx AND NONE OF THEM OPENS ONE. The caller opens the
// transaction and writes the audit entry inside it (rule 6, invariant 3) — the same split
// service-identity's PhienStore makes, and for the same reason: this layer does not know the
// business fact. "The row changed" is a technical detail; what the trail has to answer is what a
// person DID.
type LoaiVanBanStore struct {
	db *store.DB
}

func NewLoaiVanBanStore(db *store.DB) *LoaiVanBanStore { return &LoaiVanBanStore{db: db} }

// TranDanhMucLoaiVanBan is the hard upper bound on one commune's document-type catalogue.
//
// WHY A BOUND AT ALL WHEN THE LIST IS NOT PAGINATED. One process serves 200+ communes, so every
// list route is a shared resource and an unbounded one is forbidden (skills/rest-api-design §5,
// forbidden #4). This route deliberately returns the WHOLE list — see DanhSach — so the bound
// cannot come from a `limit` parameter; it has to be a ceiling the commune's data is checked
// against.
//
// 200 IS ABOUT THIRTY TIMES THE FIGURE THE SPECIFICATION LISTS. docs/ui-ux/14-cau-hinh.md:169
// names seven shipped codes, and a commune adds a handful of its own at most. The number is not an
// estimate of how many there might be; it is the point past which the rows are no longer a
// catalogue — an import run twice, a loop that inserted rows, a test fixture on a live database.
const TranDanhMucLoaiVanBan = 200

// ErrQuaNhieuLoaiVanBan says the ceiling was reached. The caller answers 500 and refuses.
//
// WHY REFUSE RATHER THAN TRUNCATE, stated where the cost is paid: this list is what a document is
// registered under, and numbering follows the type (ADR 0024). A silently short list is a type
// that has quietly disappeared from the registration form — the document is filed under the wrong
// type, it takes a number from the wrong series, and the screen looks entirely normal. A refusal
// breaks the screen loudly for ONE commune and names itself in the log. Between a wrong answer
// nobody notices and no answer somebody fixes, this system chooses the second (fail closed).
var ErrQuaNhieuLoaiVanBan = errors.New("loai_van_ban: vượt trần danh mục")

// cotLoaiVanBan IS READ BY POSITION in DanhSach. `ma` and `nhan` are adjacent TEXT columns and
// `dang_dung` and `la_mac_dinh` are adjacent BOOLEANs: swapping either pair here — or there —
// produces no error at all, and the screen shows slugs where labels belong, or pre-selects a type
// the commune has taken out of use.
const cotLoaiVanBan = `id, ma, nhan, dang_dung, la_mac_dinh, thu_tu, nguon, ma_nguon_re_nhanh`

// DanhSach reads the commune's whole document-type catalogue, ordered.
//
// NOT PAGINATED, AND THAT IS A DECISION rather than an omission — the same one
// service-identity's BoPhanStore.DanhSach argues for the org chart, and for the same three
// reasons:
//
//   - it is a CLOSED REFERENCE LIST of a handful of rows, not a register that grows with use.
//     Its size is a property of how the commune classifies its paperwork, not of how long the
//     commune has been using the system — unlike `van_ban_den`, which grows without limit;
//   - its consumers need the WHOLE list to be correct at all. It fills the type box on the
//     registration form and the filter on every document list. A dropdown showing the first page
//     of types is a dropdown missing the type somebody needs, and nothing on the screen says so;
//   - a client that must follow cursors to fill a dropdown will not, and the one that forgets
//     produces exactly the silent truncation above.
//
// The bound the pagination would have provided is provided instead by TranDanhMucLoaiVanBan,
// enforced in SQL — one row over the ceiling and this refuses rather than trimming.
//
// `deleted_at IS NULL` IS THE ONLY VISIBILITY FILTER, and `dang_dung` is deliberately NOT one:
// rows the commune has taken out of use are still returned, carrying their flag, because the
// configuration screen lists them with a "Đã tắt" chip and an old document still has to render the
// label of the type it was registered under. Soft-deleted rows drop out — everywhere, always
// (rule 7, invariant 2). The partial index `loai_van_ban_danh_sach` is built on exactly this
// predicate and this order.
//
// ORDER BY thu_tu, nhan: `thu_tu` is the order the commune arranged its own catalogue in, and
// `nhan` breaks ties so two rows sharing a rank do not swap places between two calls — an unstable
// order makes a client-side diff flicker on every reload and makes any test of this compare sets
// by accident.
//
// THE COMMUNE IS NOT A PARAMETER AND CANNOT BE ONE: Scoped.Query binds it to $1 from the context,
// so this can only ever read the catalogue of the commune the request arrived in (rule 1).
func (s *LoaiVanBanStore) DanhSach(ctx context.Context) ([]domain.LoaiVanBan, error) {
	// LIMIT is the ceiling PLUS ONE, which is what makes "there are too many" detectable at all.
	// Selecting exactly the ceiling would return a full page indistinguishable from a complete
	// list of exactly that size — the truncation this route refuses to perform, performed by the
	// bound meant to prevent it.
	rows, err := s.db.For(ctx).Query(ctx, cotLoaiVanBan, "loai_van_ban",
		`AND deleted_at IS NULL ORDER BY thu_tu, nhan LIMIT $2`, TranDanhMucLoaiVanBan+1)
	if err != nil {
		return nil, fmt.Errorf("loai_van_ban: đọc danh mục: %w", err)
	}
	defer rows.Close()

	ra := make([]domain.LoaiVanBan, 0, 16)
	for rows.Next() {
		var lvb domain.LoaiVanBan
		// POSITIONAL — in lockstep with cotLoaiVanBan. See the note there on the two adjacent
		// pairs of same-typed columns.
		if err := rows.Scan(&lvb.ID, &lvb.Ma, &lvb.Nhan, &lvb.DangDung, &lvb.LaMacDinh,
			&lvb.ThuTu, &lvb.Nguon, &lvb.MaNguonReNhanh); err != nil {
			return nil, fmt.Errorf("loai_van_ban: đọc dòng: %w", err)
		}
		ra = append(ra, lvb)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("loai_van_ban: duyệt kết quả: %w", err)
	}
	if len(ra) > TranDanhMucLoaiVanBan {
		// The rows already read are DROPPED rather than trimmed and returned. Handing back a list
		// the caller might render anyway is how a refusal turns back into a silent truncation one
		// careless `if err != nil { log }` later.
		return nil, ErrQuaNhieuLoaiVanBan
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

// docMotDong is the shared Scan of one row. Positional, in lockstep with cotLoaiVanBan — see the
// note there on the adjacent same-typed columns.
func docMotDongLoaiVanBan(quet func(...any) error) (domain.LoaiVanBan, error) {
	var lvb domain.LoaiVanBan
	err := quet(&lvb.ID, &lvb.Ma, &lvb.Nhan, &lvb.DangDung, &lvb.LaMacDinh,
		&lvb.ThuTu, &lvb.Nguon, &lvb.MaNguonReNhanh)
	return lvb, err
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
func (s *LoaiVanBanStore) TheoIDDeSua(ctx context.Context, tx *store.ScopedTx, id string) (domain.LoaiVanBan, error) {
	const stmt = `SELECT ` + cotLoaiVanBan + ` FROM loai_van_ban ` +
		`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR UPDATE`

	lvb, err := docMotDongLoaiVanBan(tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), id).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.LoaiVanBan{}, ErrDanhMucKhongTonTai
	}
	if err != nil {
		return domain.LoaiVanBan{}, fmt.Errorf("loai_van_ban: đọc dòng để sửa: %w", err)
	}
	return lvb, nil
}

// MaDaDung reports whether this code is already taken in this commune — INCLUDING soft-deleted rows.
//
// THE UNIQUE KEY IS THE REAL GUARD; THIS IS THE READABLE MESSAGE. Two concurrent creates of the
// same code can both pass this check and the second will then hit `UNIQUE (tenant_id, ma)` and roll
// the whole transaction back — no duplicate row, an unhelpful 500. That is the correct trade: the
// constraint never lets the duplicate exist, and this turns the ordinary case into a sentence
// somebody can act on.
func (s *LoaiVanBanStore) MaDaDung(ctx context.Context, tx *store.ScopedTx, ma string) (bool, error) {
	// `deleted_at` DELIBERATELY ABSENT FROM THE PREDICATE. See ErrMaDaTonTai: an issued code is
	// never reissued (rule 7, invariant 3), so a deleted row still owns its code.
	const stmt = `SELECT count(*) FROM loai_van_ban WHERE tenant_id = $1 AND ma = $2`

	var n int
	if err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), ma).Scan(&n); err != nil {
		return false, fmt.Errorf("loai_van_ban: kiểm tra mã trùng: %w", err)
	}
	return n > 0, nil
}

// DemDangSong counts the commune's live rows, for the ceiling check. Soft-deleted rows are excluded
// because DanhSach excludes them: the two numbers have to mean the same thing or the check guards
// the wrong quantity.
func (s *LoaiVanBanStore) DemDangSong(ctx context.Context, tx *store.ScopedTx) (int, error) {
	const stmt = `SELECT count(*) FROM loai_van_ban WHERE tenant_id = $1 AND deleted_at IS NULL`

	var n int
	if err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID())).Scan(&n); err != nil {
		return 0, fmt.Errorf("loai_van_ban: đếm dòng đang sống: %w", err)
	}
	return n, nil
}

// chenLoaiVanBan — `nguon` AND `ma_nguon_re_nhanh` ARE WRITTEN AS LITERALS AND ARE NOT PARAMETERS.
//
// Read that as the security property it is, not as a shortcut. There is no $n for either column, so
// there is no value a handler could pass and no field a client could fill: a commune's own row is
// tier 1, always, and the software's rows arrive by a path that does not exist in this repository
// yet (commune onboarding — see the migration header). Turning either into a parameter is the one
// edit that reopens the whole tier model, and it would look like tidying up.
const chenLoaiVanBan = `INSERT INTO loai_van_ban
	(tenant_id, id, ma, nhan, thu_tu, la_mac_dinh, dang_dung, nguon, ma_nguon_re_nhanh)
	VALUES ($1, $2, $3, $4, $5, $6, $7, 'don-vi', false)`

// Chen adds one row the COMMUNE owns. There is no method here that writes a `he-thong` row.
func (s *LoaiVanBanStore) Chen(ctx context.Context, tx *store.ScopedTx, lvb domain.LoaiVanBan) error {
	if _, err := tx.Exec(ctx, chenLoaiVanBan, string(tx.TenantID()),
		lvb.ID, lvb.Ma, lvb.Nhan, lvb.ThuTu, lvb.LaMacDinh, lvb.DangDung); err != nil {
		return fmt.Errorf("loai_van_ban: chèn: %w", err)
	}
	return nil
}

// BoMacDinhKhac clears the default flag on every other live row of this commune.
//
// WHY IT IS A SEPARATE STATEMENT AND MUST RUN IN THE SAME TRANSACTION: `UNIQUE (tenant_id,
// moc_mac_dinh)` admits exactly one live default, so setting a new one without clearing the old one
// fails the constraint. Run in a different transaction, the window between them is a commune with
// NO default, and the form pre-selects nothing while it lasts.
func (s *LoaiVanBanStore) BoMacDinhKhac(ctx context.Context, tx *store.ScopedTx, trongID string) error {
	const stmt = `UPDATE loai_van_ban SET la_mac_dinh = false, cap_nhat_luc = now() ` +
		`WHERE tenant_id = $1 AND id <> $2 AND la_mac_dinh AND deleted_at IS NULL`

	if _, err := tx.Exec(ctx, stmt, string(tx.TenantID()), trongID); err != nil {
		return fmt.Errorf("loai_van_ban: bỏ mặc định cũ: %w", err)
	}
	return nil
}

// capNhatLoaiVanBan — `ma`, `nguon` AND `ma_nguon_re_nhanh` APPEAR NOWHERE IN THIS STATEMENT.
//
// `ma` because an issued code is never renumbered (rule 7, invariant 3) and business records hold
// it as a value; the other two because they decide the tier. All three are also refused by the
// trigger, and both layers are meant: the trigger is the floor that holds against every writer, and
// their absence here is what makes the floor unreachable from this service in the first place.
const capNhatLoaiVanBan = `UPDATE loai_van_ban ` +
	`SET nhan = $3, thu_tu = $4, dang_dung = $5, la_mac_dinh = $6, cap_nhat_luc = now() ` +
	`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

// CapNhat writes the four fields a commune may change. The caller has already read the row with
// TheoIDDeSua and decided the change is permitted at this row's tier.
func (s *LoaiVanBanStore) CapNhat(ctx context.Context, tx *store.ScopedTx, lvb domain.LoaiVanBan) error {
	kq, err := tx.Exec(ctx, capNhatLoaiVanBan, string(tx.TenantID()),
		lvb.ID, lvb.Nhan, lvb.ThuTu, lvb.DangDung, lvb.LaMacDinh)
	if err != nil {
		return fmt.Errorf("loai_van_ban: cập nhật: %w", err)
	}
	return doiMotDong(kq, "cập nhật")
}

// xoaMemLoaiVanBan writes all THREE columns rule 7, invariant 1 names — `deleted_at`, `deleted_by`
// and `delete_reason` — in one statement, so none of them can be forgotten.
//
// `AND deleted_at IS NULL` IS WHAT MAKES A SECOND DELETE A 404 rather than a silent rewrite of who
// deleted the row and why. The first deletion is the one that happened; overwriting its reason
// would be editing a historical record (rule 7, forbidden #5).
const xoaMemLoaiVanBan = `UPDATE loai_van_ban ` +
	`SET deleted_at = now(), deleted_by = $3, delete_reason = $4, cap_nhat_luc = now() ` +
	`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

// XoaMem soft deletes one row. THERE IS NO HARD DELETE ANYWHERE IN THIS PACKAGE, and the trigger
// refuses one even if somebody writes it (rule 7, forbidden #1).
func (s *LoaiVanBanStore) XoaMem(ctx context.Context, tx *store.ScopedTx, id, boi, lyDo string) error {
	kq, err := tx.Exec(ctx, xoaMemLoaiVanBan, string(tx.TenantID()), id, boi, lyDo)
	if err != nil {
		return fmt.Errorf("loai_van_ban: xoá mềm: %w", err)
	}
	return doiMotDong(kq, "xoá mềm")
}
