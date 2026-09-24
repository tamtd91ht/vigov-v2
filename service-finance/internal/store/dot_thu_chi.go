package store

// The batches of revenue/expenditure against one budget line — reads and writes (migration 0008,
// docs/ui-ux/07-thu-chi-ngan-sach.md §5). The five properties at the top of thu_chi_ngan_sach.go hold
// here unchanged; three more are specific to these two tables:
//
//  1. EVERY SUM AND EVERY LIST JOINS `dot_thu_chi.deleted_at IS NULL`. `gia_tri_dot` has no soft-delete
//     columns of its own (0008 says why), so a read that forgets the join counts removed batches.
//  2. `gia_tri_dot` IS ONLY EVER INSERTED INTO. Its trigger refuses UPDATE and DELETE; no statement
//     here tries either.
//  3. `don_vi_ca_nhan` MAY BE A PERSON'S NAME. It is read and written as a bound parameter and never
//     appears in an error built here (rule 3, forbidden #3).

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// ErrQuaNhieuDot — one line has more live batches than TranDotMotKhoanMuc. REFUSED, NEVER TRUNCATED:
// the list is what an accountant reconciles against receipts, and a silently short list is a batch
// that looks missing while it is still counted in the line's figure.
var ErrQuaNhieuDot = errors.New("ngan_sach: khoản mục vượt trần số đợt thu chi")

// TranDotMotKhoanMuc — the domain's ceiling (domain.TranDotMotKhoanMuc), named here too because the
// list read below enforces it. One value: the write refuses at the same number the read does.
const TranDotMotKhoanMuc = domain.TranDotMotKhoanMuc

// tongDotCuaBang sums the LIVE batches of every live LEAF line in `entries` mode of one sheet, per
// line and column.
//
// ONLY `entries` LEAVES ($3 = 'entries', no live child): those are the only lines whose displayed
// figure is the batch sum (BangDayDu.GiaTri). Summing every line's batches cost a scan for figures
// nobody reads — and, before the sum stopped failing the read, let one oversized sum on a `manual`
// line lock the whole sheet over a number that counts nowhere.
//
// `d.deleted_at IS NULL` IS THE CLAUSE THAT CAUSES INCIDENTS WHEN MISSING (0008, question 4a).
// `g.gia_tri IS NOT NULL` + GROUP BY means a (line, column) with no stated amount produces NO ROW, so
// the domain reads it as empty (`—`), never 0 (§9 rule 4).
//
// `::text` BECAUSE SUM(BIGINT) IS NUMERIC in PostgreSQL. The text is parsed with strconv.ParseInt, so a
// sum that does not fit int64 is detected (strconv.ErrRange) rather than whatever a driver does with an
// oversize numeric — and it becomes THAT cell's reason, not the sheet's failure (quetTongDot).
//
// tenant_id IS BOUND ON EVERY TABLE OF THE JOIN (rule 1): joining on ids alone would match another
// commune's row wherever ids collide.
const tongDotCuaBang = `SELECT d.khoan_muc_id, g.cot_id, SUM(g.gia_tri)::text
	FROM gia_tri_dot g
	JOIN dot_thu_chi d
	  ON d.tenant_id = g.tenant_id AND d.id = g.dot_id
	JOIN khoan_muc_ngan_sach k
	  ON k.tenant_id = d.tenant_id AND k.id = d.khoan_muc_id
	WHERE g.tenant_id = $1 AND k.bang_id = $2
	  AND k.deleted_at IS NULL AND d.deleted_at IS NULL AND g.gia_tri IS NOT NULL
	  AND k.cach_tinh = $3
	  AND NOT EXISTS (SELECT 1 FROM khoan_muc_ngan_sach c
	                   WHERE c.tenant_id = k.tenant_id AND c.cha_id = k.id AND c.deleted_at IS NULL)
	GROUP BY d.khoan_muc_id, g.cot_id`

// quetTongDot returns the sums that fit int64, and SEPARATELY the (line, column) pairs whose sum does
// not. An oversized sum is NOT an error of the read: failing here failed the whole sheet, which locked
// every write — GoDot included, the one act that removes the offending batch. The domain turns the
// flag into that one cell's reason (domain.ErrTongDotVuotMuc). Any OTHER parse failure is not a size
// and still fails: a sum column that is not a number is a defect, not a figure.
func quetTongDot(rows *sql.Rows) (map[string]map[string]domain.Dong, map[string]map[string]bool, error) {
	defer rows.Close()
	ra := map[string]map[string]domain.Dong{}
	vuot := map[string]map[string]bool{}
	for rows.Next() {
		var khoanMucID, cotID, tong string
		if err := rows.Scan(&khoanMucID, &cotID, &tong); err != nil {
			return nil, nil, fmt.Errorf("ngan_sach: đọc dòng tổng đợt: %w", err)
		}
		g, err := strconv.ParseInt(tong, 10, 64)
		if errors.Is(err, strconv.ErrRange) {
			if vuot[khoanMucID] == nil {
				vuot[khoanMucID] = map[string]bool{}
			}
			vuot[khoanMucID][cotID] = true
			continue
		}
		if err != nil {
			// The text is not quoted: it is a figure from the commune's budget.
			return nil, nil, fmt.Errorf("ngan_sach: tổng đợt không phải số nguyên: %w", strconv.ErrSyntax)
		}
		if ra[khoanMucID] == nil {
			ra[khoanMucID] = map[string]domain.Dong{}
		}
		ra[khoanMucID][cotID] = domain.Dong(g)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("ngan_sach: duyệt tổng đợt: %w", err)
	}
	return ra, vuot, nil
}

// --- reads outside a transaction (GET /api/v1/budget-lines/{id}/entries) ---------------------------

// khoanMucSongTrongBangSong reads one live line whose SHEET is live too. A line of a removed sheet
// is off every screen (thu_chi_ngan_sach.go, xoaMemBang), so its batches must be too.
const khoanMucSongTrongBangSong = `SELECT k.id, k.bang_id, COALESCE(k.cha_id, ''), k.tt, k.ten,
	k.thu_tu, k.cach_tinh, k.cap, k.is_headline
	FROM khoan_muc_ngan_sach k
	JOIN bang_ngan_sach b
	  ON b.tenant_id = k.tenant_id AND b.id = k.bang_id
	WHERE k.tenant_id = $1 AND k.id = $2 AND k.deleted_at IS NULL AND b.deleted_at IS NULL`

const cotDot = `id, khoan_muc_id, ngay, noi_dung, COALESCE(don_vi_ca_nhan, ''), ` +
	`COALESCE(so_chung_tu, ''), nguoi_ghi_ma, tao_luc`

// soTienDotCuaKhoanMuc — every stated amount of one line's LIVE batches.
const soTienDotCuaKhoanMuc = `SELECT g.dot_id, g.cot_id, g.gia_tri
	FROM gia_tri_dot g
	JOIN dot_thu_chi d
	  ON d.tenant_id = g.tenant_id AND d.id = g.dot_id
	WHERE g.tenant_id = $1 AND d.khoan_muc_id = $2
	  AND d.deleted_at IS NULL AND g.gia_tri IS NOT NULL`

// DotCuaKhoanMuc reads what the `⇄` dialog shows: the line, its sheet's live columns, and its live
// batches NEWEST FIRST (`ngay DESC, tao_luc DESC`, then `id` so the order is total).
//
// A line that does not exist, was removed, sits in a removed sheet, or belongs to another commune is
// ONE answer — domain.ErrKhongThayKhoanMuc — because the query reaches none of them.
func (s *NganSachStore) DotCuaKhoanMuc(ctx context.Context,
	khoanMucID string) (domain.DotCuaKhoanMuc, error) {

	rows, err := s.db.For(ctx).QueryJoin(ctx, khoanMucSongTrongBangSong, khoanMucID)
	if err != nil {
		return domain.DotCuaKhoanMuc{}, fmt.Errorf("ngan_sach: đọc khoản mục của đợt: %w", err)
	}
	k, thay, err := quetMotKhoanMuc(rows)
	if err != nil {
		return domain.DotCuaKhoanMuc{}, err
	}
	if !thay {
		return domain.DotCuaKhoanMuc{}, domain.ErrKhongThayKhoanMuc
	}

	cot, err := s.CotCuaBang(ctx, k.BangID)
	if err != nil {
		return domain.DotCuaKhoanMuc{}, err
	}

	// LIMIT IS THE CEILING PLUS ONE — the same reason KhoanMucCuaBang gives: selecting exactly the
	// ceiling cannot tell a full list from a truncated one.
	rows, err = s.db.For(ctx).Query(ctx, cotDot, "dot_thu_chi",
		`AND deleted_at IS NULL AND khoan_muc_id = $2 ORDER BY ngay DESC, tao_luc DESC, id DESC LIMIT $3`,
		khoanMucID, TranDotMotKhoanMuc+1)
	if err != nil {
		return domain.DotCuaKhoanMuc{}, fmt.Errorf("ngan_sach: đọc đợt: %w", err)
	}
	dot, err := quetDot(rows)
	if err != nil {
		return domain.DotCuaKhoanMuc{}, err
	}
	if len(dot) > TranDotMotKhoanMuc {
		return domain.DotCuaKhoanMuc{}, ErrQuaNhieuDot
	}

	rows, err = s.db.For(ctx).QueryJoin(ctx, soTienDotCuaKhoanMuc, khoanMucID)
	if err != nil {
		return domain.DotCuaKhoanMuc{}, fmt.Errorf("ngan_sach: đọc số tiền đợt: %w", err)
	}
	if err := ganSoTien(rows, dot); err != nil {
		return domain.DotCuaKhoanMuc{}, err
	}
	return domain.DotCuaKhoanMuc{KhoanMuc: k, Cot: cot, Dot: dot}, nil
}

func quetMotKhoanMuc(rows *sql.Rows) (domain.KhoanMucNganSach, bool, error) {
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.KhoanMucNganSach{}, false, fmt.Errorf("ngan_sach: đọc khoản mục: %w", err)
		}
		return domain.KhoanMucNganSach{}, false, nil
	}
	var (
		k        domain.KhoanMucNganSach
		cachTinh string
	)
	err := rows.Scan(&k.ID, &k.BangID, &k.ChaID, &k.TT, &k.Ten, &k.ThuTu, &cachTinh, &k.Cap, &k.LaDongTong)
	if err != nil {
		return domain.KhoanMucNganSach{}, false, fmt.Errorf("ngan_sach: đọc dòng khoản mục: %w", err)
	}
	k.CachTinh = domain.CachTinh(cachTinh)
	return k, true, nil
}

func quetDot(rows *sql.Rows) ([]domain.DotThuChi, error) {
	defer rows.Close()
	ra := make([]domain.DotThuChi, 0, 16)
	for rows.Next() {
		var d domain.DotThuChi
		err := rows.Scan(&d.ID, &d.KhoanMucID, &d.Ngay, &d.NoiDung, &d.DonViCaNhan,
			&d.SoChungTu, &d.NguoiGhiMa, &d.TaoLuc)
		if err != nil {
			return nil, fmt.Errorf("ngan_sach: đọc dòng đợt: %w", err)
		}
		d.GiaTri = map[string]domain.Dong{}
		ra = append(ra, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ngan_sach: duyệt đợt: %w", err)
	}
	return ra, nil
}

// ganSoTien attaches amounts to the batches already read. An amount whose batch is not in the list
// cannot occur (both reads bind the same line and `deleted_at IS NULL`) and is skipped rather than
// invented into a batch.
func ganSoTien(rows *sql.Rows, dot []domain.DotThuChi) error {
	defer rows.Close()
	theoID := make(map[string]int, len(dot))
	for i := range dot {
		theoID[dot[i].ID] = i
	}
	for rows.Next() {
		var dotID, cotID string
		var gia int64
		if err := rows.Scan(&dotID, &cotID, &gia); err != nil {
			return fmt.Errorf("ngan_sach: đọc dòng số tiền đợt: %w", err)
		}
		if i, co := theoID[dotID]; co {
			dot[i].GiaTri[cotID] = domain.Dong(gia)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("ngan_sach: duyệt số tiền đợt: %w", err)
	}
	return nil
}

// --- reads INSIDE a transaction ----------------------------------------------------------------------

const dotSongTheoID = `SELECT ` + cotDot + ` FROM dot_thu_chi ` +
	`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

const soTienCuaMotDot = `SELECT g.dot_id, g.cot_id, g.gia_tri
	FROM gia_tri_dot g
	WHERE g.tenant_id = $1 AND g.dot_id = $2 AND g.gia_tri IS NOT NULL`

// DotTheoIDTrongGiaoDich reads one LIVE batch with its amounts, inside the transaction that is about
// to remove it. The caller then locks the SHEET (the serialisation point for every write on this
// board) and the soft delete's own `AND deleted_at IS NULL` catches a concurrent removal.
func (s *NganSachStore) DotTheoIDTrongGiaoDich(ctx context.Context, tx *store.ScopedTx,
	id string) (domain.DotThuChi, error) {

	rows, err := tx.Underlying().QueryContext(ctx, dotSongTheoID, string(tx.TenantID()), id)
	if err != nil {
		return domain.DotThuChi{}, fmt.Errorf("ngan_sach: đọc đợt để gỡ: %w", err)
	}
	dot, err := quetDot(rows)
	if err != nil {
		return domain.DotThuChi{}, err
	}
	if len(dot) == 0 {
		return domain.DotThuChi{}, domain.ErrKhongThayDot
	}

	rows, err = tx.Underlying().QueryContext(ctx, soTienCuaMotDot, string(tx.TenantID()), id)
	if err != nil {
		return domain.DotThuChi{}, fmt.Errorf("ngan_sach: đọc số tiền đợt để gỡ: %w", err)
	}
	if err := ganSoTien(rows, dot[:1]); err != nil {
		return domain.DotThuChi{}, err
	}
	return dot[0], nil
}

const coDotConSong = `SELECT EXISTS (SELECT 1 FROM dot_thu_chi
	WHERE tenant_id = $1 AND khoan_muc_id = $2 AND deleted_at IS NULL)`

// CoDotConSong reports whether one line still has a LIVE batch — the question the child-add refusal
// turns on (domain.ErrKhoanMucTheoDotConDot).
func (s *NganSachStore) CoDotConSong(ctx context.Context, tx *store.ScopedTx,
	khoanMucID string) (bool, error) {

	var co bool
	err := tx.Underlying().QueryRowContext(ctx, coDotConSong, string(tx.TenantID()), khoanMucID).Scan(&co)
	if err != nil {
		return false, fmt.Errorf("ngan_sach: kiểm đợt còn sống: %w", err)
	}
	return co, nil
}

const demDotSong = `SELECT count(*) FROM dot_thu_chi
	WHERE tenant_id = $1 AND khoan_muc_id = $2 AND deleted_at IS NULL`

// DemDotSong counts one line's LIVE batches, inside the write transaction — the number the batch
// ceiling (domain.TranDotMotKhoanMuc) is checked against before an insert. It runs AFTER the sheet's
// FOR UPDATE lock (every batch write and removal takes that lock first), so two concurrent writes
// cannot both see 1999 and both insert.
func (s *NganSachStore) DemDotSong(ctx context.Context, tx *store.ScopedTx, khoanMucID string) (int, error) {
	var n int
	err := tx.Underlying().QueryRowContext(ctx, demDotSong, string(tx.TenantID()), khoanMucID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("ngan_sach: đếm đợt còn sống: %w", err)
	}
	return n, nil
}

// --- writes --------------------------------------------------------------------------------------------

// chenDot — `tao_luc` takes its default and `deleted_*` are absent: a batch is born live. Empty
// optional text goes in as NULL, never ” (0008 refuses ” so "not stated" has one spelling).
const chenDot = `INSERT INTO dot_thu_chi
	(tenant_id, id, khoan_muc_id, ngay, noi_dung, don_vi_ca_nhan, so_chung_tu, nguoi_ghi_ma)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

// ChenDot records one batch row. Its amounts follow through ChenSoTienDot in the same transaction.
func (s *NganSachStore) ChenDot(ctx context.Context, tx *store.ScopedTx, d domain.DotThuChi) error {
	_, err := tx.Exec(ctx, chenDot, string(tx.TenantID()),
		d.ID, d.KhoanMucID, d.Ngay, d.NoiDung,
		rongThanhNil(d.DonViCaNhan), rongThanhNil(d.SoChungTu), d.NguoiGhiMa)
	if err != nil {
		return fmt.Errorf("ngan_sach: chèn đợt: %w", err)
	}
	return nil
}

const chenSoTienDot = `INSERT INTO gia_tri_dot (tenant_id, dot_id, cot_id, gia_tri) VALUES ($1, $2, $3, $4)`

// ChenSoTienDot records one amount of one batch. Plain INSERT, no upsert: an amount is written once
// and the primary key (tenant_id, dot_id, cot_id) refuses a second one for the same column.
func (s *NganSachStore) ChenSoTienDot(ctx context.Context, tx *store.ScopedTx,
	dotID, cotID string, gia domain.Dong) error {

	if _, err := tx.Exec(ctx, chenSoTienDot, string(tx.TenantID()), dotID, cotID, int64(gia)); err != nil {
		return fmt.Errorf("ngan_sach: chèn số tiền đợt: %w", err)
	}
	return nil
}

// xoaMemDot writes rule 7 invariant 1's three columns in one statement — the ONE update 0008's
// `dot_thu_chi_bat_bien` admits. `AND deleted_at IS NULL` makes a second removal a not-found (404)
// rather than a statement the trigger would refuse with a 500.
const xoaMemDot = `UPDATE dot_thu_chi
	SET deleted_at = now(), deleted_by = $3, delete_reason = $4
	WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

func (s *NganSachStore) XoaMemDot(ctx context.Context, tx *store.ScopedTx, id, boi, lyDo string) error {
	kq, err := tx.Exec(ctx, xoaMemDot, string(tx.TenantID()), id, boi, lyDo)
	if err != nil {
		return fmt.Errorf("ngan_sach: xoá mềm đợt: %w", err)
	}
	return doiMotDongNganSach(kq, "xoá mềm đợt", domain.ErrKhongThayDot)
}
