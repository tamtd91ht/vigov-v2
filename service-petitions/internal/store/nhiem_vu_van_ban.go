package store

// `nhiem_vu_van_ban` (migration 0009) — the three lists of referenced documents of §5.4 / §7.2.
// SQL, and nothing else.
//
// FIVE THINGS HOLD IN EVERY STATEMENT IN THIS FILE, each a defect class rather than a style:
//
//  1. THE COMMUNE IS $1 IN EVERY STATEMENT, from the context or from tx.TenantID() (rule 1,
//     invariants 4 and 5). It is never a parameter here, so no caller can reach another commune's
//     task.
//  2. NOTHING HERE OPENS A TRANSACTION. Every write takes *store.ScopedTx — the caller opens it and
//     writes the audit entry inside it (rule 6, invariant 3). That is what makes "change the record
//     now, record the trail afterwards if it works" impossible to express.
//  3. EVERY READ EXCLUDES SOFT-DELETED ROWS (rule 7, invariant 2) — WITH ONE DELIBERATE EXCEPTION,
//     ThuTuVanBanLonNhat, which counts them on purpose. Its own comment says why.
//  4. THERE IS NO HARD REMOVAL STATEMENT IN THIS FILE AND THERE MUST NEVER BE ONE. A line is part of
//     a task, and a task is an administrative record (rule 7); migration 0009's
//     `nhiem_vu_van_ban_cam_xoa` trigger refuses it underneath in any case.
//  5. `nhom` AND `thu_tu` APPEAR IN NO UPDATE HERE. A line does not move between groups and does not
//     change position — see domain.ErrDoiNhomVanBan and the `thu_tu` block of migration 0009. Their
//     absence from SuaVanBan is what keeps that floor unreachable from Go.
//
// THE METHODS SIT ON NhiemVuStore, not on a store of their own, and the split is the one
// `nhat_ky_nhiem_vu` already made: this table has no lifecycle of its own and is only ever read and
// written as part of a task. `de_nghi_lui_han` got its own store because it IS a record with its own
// lifecycle — filed, then decided, by a different person under a different permission.

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// cotVanBanNhiemVu IS READ BY POSITION in quetVanBanNhiemVu below.
//
// `deleted_at` / `deleted_by` ARE NOT IN IT, deliberately: every read here filters them out, so a
// row that came back carrying them would mean the filter was dropped — and a column nobody scans
// cannot hide that. `tao_luc` is not in it either: it is a ROW-LIFECYCLE instant, not a business
// fact any screen renders, and putting it in the shape every read returns would invite one.
const cotVanBanNhiemVu = `id, nhiem_vu_id, nhom, so_ky_hieu, ngay_van_ban, trich_yeu, thu_tu`

// thuTuVanBan is the order §5.4 draws the block in: group by group, and inside a group by the
// position that was issued. Written once because three statements below need the same ordering and
// three copies would drift.
const thuTuVanBan = ` ORDER BY nhom, thu_tu`

// VanBanCuaNhiemVu reads the whole block of ONE task, for the detail screen.
//
// # IT IS A SEPARATE CALL FROM TheoMa AND NOT A JOIN, AND THAT IS THE POINT
//
// The register LIST must not pay for this: §4's card does not draw the block, and a join would put
// three lists of free text onto every row of every page. Keeping it a second call means the surface
// that needs it asks for it, and `domain.NhiemVu.VanBan` stays nil everywhere else — which is the
// distinction the response type carries onto the wire.
//
// IT RUNS OUTSIDE A TRANSACTION, like every other read path in this service. The task and its lines
// are therefore read at two instants, and a concurrent edit could land between them. What that costs
// on a read-only drawer is one line appearing or disappearing a moment early; what a transaction
// would cost is a read path that takes locks on a government register. The write path does it the
// other way round and holds the task row — see VanBanCuaNhiemVuDeSua.
func (s *NhiemVuStore) VanBanCuaNhiemVu(ctx context.Context, nhiemVuID string) (
	[]domain.NhiemVuVanBan, error) {

	rows, err := s.db.For(ctx).Query(ctx, cotVanBanNhiemVu, "nhiem_vu_van_ban",
		`AND nhiem_vu_id = $2 AND deleted_at IS NULL`+thuTuVanBan, nhiemVuID)
	if err != nil {
		return nil, fmt.Errorf("nhiem_vu_van_ban: đọc danh sách văn bản: %w", err)
	}
	defer rows.Close()

	return quetDanhSachVanBan(rows)
}

// VanBanCuaNhiemVuDeSua reads the same block INSIDE the caller's transaction.
//
// # THERE IS NO `FOR UPDATE` HERE, AND THE ABSENCE IS DELIBERATE RATHER THAN FORGOTTEN
//
// Every caller has already read the TASK with `TheoMaDeSua`, which holds that row until the
// transaction ends. Two officers editing one task's document block are therefore serialised on the
// task, which is the row both acts have in common — exactly the way appending a conclusion is
// serialised on its meeting (`BienBanTheoIDDeSua`, migration 0007). Locking the lines as well would
// add rows to the lock set without closing any window the parent lock leaves open, and a NEW line
// has no row to lock in the first place.
//
// ⚠ IT IS THE SAME COLUMNS AND THE SAME FILTER AS THE READ PATH. A write path that saw rows the read
// path hides would decide against a block nobody can see, and a soft-deleted line pulled into the
// comparison would come back as "the client removed it" on every single save.
func (s *NhiemVuStore) VanBanCuaNhiemVuDeSua(ctx context.Context, tx *store.ScopedTx,
	nhiemVuID string) ([]domain.NhiemVuVanBan, error) {

	const stmt = `SELECT ` + cotVanBanNhiemVu + ` FROM nhiem_vu_van_ban
		WHERE tenant_id = $1 AND nhiem_vu_id = $2 AND deleted_at IS NULL` + thuTuVanBan

	rows, err := tx.Underlying().QueryContext(ctx, stmt, string(tx.TenantID()), nhiemVuID)
	if err != nil {
		return nil, fmt.Errorf("nhiem_vu_van_ban: đọc danh sách văn bản để sửa: %w", err)
	}
	defer rows.Close()

	return quetDanhSachVanBan(rows)
}

// ThuTuVanBanLonNhat reads the HIGHEST position ever issued in ONE group of ONE task.
//
// # IT DELIBERATELY DOES NOT FILTER `deleted_at`, AND THAT IS THE WHOLE POINT
//
// A removed line keeps its number: `UNIQUE (tenant_id, nhiem_vu_id, nhom, thu_tu)` counts
// soft-deleted rows (migration 0009 says why in full). Counting only live rows would mint a position
// the removed row still holds, and the INSERT would be refused by the unique key INSIDE the business
// transaction — which, because the audit entry shares it (rule 6, invariant 3), rolls the whole edit
// back. A gap after a removal is the correct outcome.
//
// ZERO MEANS THE GROUP HAS NEVER HELD A LINE, which domain.ThuTuVanBanTiepTheo turns into 1.
func (s *NhiemVuStore) ThuTuVanBanLonNhat(ctx context.Context, tx *store.ScopedTx,
	nhiemVuID string, nhom domain.NhomVanBanNhiemVu) (int, error) {

	const stmt = `SELECT COALESCE(MAX(thu_tu), 0) FROM nhiem_vu_van_ban
		WHERE tenant_id = $1 AND nhiem_vu_id = $2 AND nhom = $3`

	var so int
	err := tx.Underlying().QueryRowContext(ctx, stmt,
		string(tx.TenantID()), nhiemVuID, string(nhom)).Scan(&so)
	if err != nil {
		return 0, fmt.Errorf("nhiem_vu_van_ban: đọc số thứ tự lớn nhất: %w", err)
	}
	return so, nil
}

// ThemVanBan appends ONE line INSIDE the caller's transaction.
//
// THE POSITION ARRIVES ALREADY MINTED, from domain.ThuTuVanBanTiepTheo over ThuTuVanBanLonNhat, read
// under the task's lock. It is not computed here: a store that minted it would be a second place the
// numbering rule lives, and the unique key is what actually guarantees it either way.
//
// AN EMPTY `so_ky_hieu` AND A ZERO DATE BECOME SQL NULL. Both are the ordinary case today — §7.2
// offers one textarea and does not split them out — and NULL is the value that means "not recorded",
// while an empty string would satisfy the column while naming no document at all.
func (s *NhiemVuStore) ThemVanBan(ctx context.Context, tx *store.ScopedTx,
	v domain.NhiemVuVanBan) error {

	const stmt = `INSERT INTO nhiem_vu_van_ban (
		tenant_id, id, nhiem_vu_id, nhom, so_ky_hieu, ngay_van_ban, trich_yeu, thu_tu)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`

	_, err := tx.Exec(ctx, stmt, string(tx.TenantID()), v.ID, v.NhiemVuID, string(v.Nhom),
		rongThanhNull(v.SoKyHieu), khongThanhNull(v.NgayVanBan), v.TrichYeu, v.ThuTu)
	if err != nil {
		// NOT the line's text: a document's subject line routinely names a citizen's case, and an
		// INSERT error can quote the whole row on some drivers (rule 3, forbidden #1).
		return fmt.Errorf("nhiem_vu_van_ban: ghi dòng văn bản: %w", err)
	}
	return nil
}

// SuaVanBan writes the three content fields of ONE existing line.
//
// `nhom` AND `thu_tu` ARE NOT IN THE SET LIST, and `nhiem_vu_id` is not either. There is no value
// any caller could pass that would move a line between groups, change its position, or hand it to
// another task — which is the floor under domain.SoSanhVanBan rather than a duplicate of it.
//
// THE WHERE CLAUSE CARRIES `nhiem_vu_id`, not only the line id. The line id alone would be enough to
// find the row; matching the task as well is what stops a line of task A being edited through task
// B's URL, and it is the same discipline `DeNghiLuiHanStore.TheoIDDeSua` applies to a request.
func (s *NhiemVuStore) SuaVanBan(ctx context.Context, tx *store.ScopedTx,
	v domain.NhiemVuVanBan) error {

	const stmt = `UPDATE nhiem_vu_van_ban
		SET so_ky_hieu = $4, ngay_van_ban = $5, trich_yeu = $6, cap_nhat_luc = now()
		WHERE tenant_id = $1 AND id = $2 AND nhiem_vu_id = $3 AND deleted_at IS NULL`

	kq, err := tx.Exec(ctx, stmt, string(tx.TenantID()), v.ID, v.NhiemVuID,
		rongThanhNull(v.SoKyHieu), khongThanhNull(v.NgayVanBan), v.TrichYeu)
	if err != nil {
		return fmt.Errorf("nhiem_vu_van_ban: sửa dòng văn bản: %w", err)
	}
	return doiMotDongVanBan(kq, "sửa dòng văn bản")
}

// XoaMemVanBan removes ONE line from every read path and keeps the row (rule 7, invariant 1).
//
// TWO COLUMNS AND NOT THREE. Migration 0009 sets out why there is no `delete_reason` on this table:
// removing a line is an EDIT of the task, and the reason for an edit lives on the audit entry of the
// act, not on each line the act touched. `nguoiMa` is a STAFF BUSINESS CODE (rule 6, invariant 8) —
// read years later by somebody handling a complaint, where a ULID would name nobody.
//
// `AND deleted_at IS NULL` IS WHAT MAKES A SECOND REMOVAL HARMLESS: it cannot overwrite who removed
// the line or when, so the same request sent twice leaves the first answer standing.
func (s *NhiemVuStore) XoaMemVanBan(ctx context.Context, tx *store.ScopedTx,
	nhiemVuID, id, nguoiMa string, luc time.Time) error {

	const stmt = `UPDATE nhiem_vu_van_ban
		SET deleted_at = $4, deleted_by = $5, cap_nhat_luc = now()
		WHERE tenant_id = $1 AND id = $2 AND nhiem_vu_id = $3 AND deleted_at IS NULL`

	kq, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id, nhiemVuID, luc, nguoiMa)
	if err != nil {
		return fmt.Errorf("nhiem_vu_van_ban: gỡ dòng văn bản: %w", err)
	}
	return doiMotDongVanBan(kq, "gỡ dòng văn bản")
}

// doiMotDongVanBan turns "no row matched" into the sentinel the caller answers 409 for.
//
// IT REUSES ErrNhiemVuDaChuyenTrang's MEANING RATHER THAN A SENTINEL OF ITS OWN, and the shared
// sentence is the honest one: the caller read this line under the task's lock a moment ago, so zero
// rows means somebody edited the same task's block through another session between the two
// statements. "Tải lại rồi thao tác lại" is exactly what that officer has to do.
func doiMotDongVanBan(kq sql.Result, viec string) error {
	n, err := kq.RowsAffected()
	if err != nil {
		return fmt.Errorf("nhiem_vu_van_ban: %s, đếm dòng: %w", viec, err)
	}
	if n == 0 {
		return ErrNhiemVuDaChuyenTrang
	}
	return nil
}

// quetDanhSachVanBan reads every row of a block.
//
// THE SLICE IS NEVER nil ON SUCCESS, and that is load-bearing rather than tidy: nil is the value
// `domain.NhiemVu.VanBan` uses for "not loaded on this surface", so a read that returned nil for an
// empty block would be indistinguishable from a read that never ran — and a screen would report an
// empty document list for a task nobody had asked about.
func quetDanhSachVanBan(rows *sql.Rows) ([]domain.NhiemVuVanBan, error) {
	ra := make([]domain.NhiemVuVanBan, 0, 3)
	for rows.Next() {
		v, err := quetVanBanNhiemVu(rows)
		if err != nil {
			return nil, err
		}
		ra = append(ra, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("nhiem_vu_van_ban: duyệt danh sách văn bản: %w", err)
	}
	return ra, nil
}

// quetVanBanNhiemVu reads one row of cotVanBanNhiemVu.
//
// POSITIONAL, IN LOCKSTEP WITH cotVanBanNhiemVu — database/sql binds by POSITION, so a destination
// inserted or removed anywhere but the tail silently shifts every column after it.
//
// THE TWO NULLABLE COLUMNS ARE READ THROUGH EXPLICIT NULL TYPES. Scanning a NULL straight into a
// string or a time.Time is a runtime error in some drivers and a zero value in others, and the
// second is how "this line records no document date" quietly becomes a date in year 1.
func quetVanBanNhiemVu(r quangKiem) (domain.NhiemVuVanBan, error) {
	var (
		v        domain.NhiemVuVanBan
		nhom     string
		soKyHieu sql.NullString
		ngay     sql.NullTime
	)
	if err := r.Scan(&v.ID, &v.NhiemVuID, &nhom, &soKyHieu, &ngay, &v.TrichYeu, &v.ThuTu); err != nil {
		// NOT the row's contents (rule 3, forbidden #3).
		return domain.NhiemVuVanBan{}, fmt.Errorf("nhiem_vu_van_ban: đọc dòng: %w", err)
	}
	v.Nhom = domain.NhomVanBanNhiemVu(nhom)
	v.SoKyHieu = soKyHieu.String
	// NULL -> the zero time.Time, which means "no document date was recorded" and is rendered as an
	// empty string on the wire. It must never reach a screen as a date in year 1.
	v.NgayVanBan = ngay.Time
	return v, nil
}
