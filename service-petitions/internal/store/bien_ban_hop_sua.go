package store

// The LIFECYCLE writes of the meeting-minutes register (user decisions 25/09/2026, migration 0012):
// edit a draft, record the conclusion notice after signing, sign, soft delete, edit / soft delete one
// conclusion, set / clear the "không phát sinh nhiệm vụ" mark — and the locking reads and counts the
// use case decides on. SQL, and nothing else.
//
// THE FIVE THINGS bien_ban_hop_ghi.go's header lists hold here too (commune is $1 from the
// transaction, nothing opens a transaction, reads exclude soft-deleted rows, `thu_tu` is never
// recomputed). Two more, particular to UPDATEs on archival records:
//
//  6. EVERY UPDATE CARRIES THE STATE IT MAY RUN IN in its WHERE clause (`trang_thai = 'du-thao'`,
//     `deleted_at IS NULL`, the mark's current value), and checks it touched exactly one row. The use
//     case has already decided under the parent's `FOR UPDATE` lock, so a zero here means the two
//     layers disagree — it is refused, never read as success.
//  7. THE STATUS CODES ARE BOUND PARAMETERS carrying the domain constants, not literals — the
//     reason cauKetLuanKemDem gives: a typo in a literal is not an error, it is a statement that
//     matches nothing.
//
// A TRIGGER REFUSAL (`RAISE EXCEPTION`, SQLSTATE P0001, migration 0012's two `…_da_ky_bat_bien`
// functions) is translated into domain.ErrBienBanDaKy / domain.ErrDaCoThongBao by boiTuChoiHoSoDaKy,
// so if one ever slips past the use case's checks the client reads 409 and not 500.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// cotKetLuanSua is cotKetLuan plus the mark — what the conclusion write acts decide on. Read by
// position in quetKetLuanSua.
const cotKetLuanSua = cotKetLuan + `, khong_phat_sinh`

// --- the locking reads ------------------------------------------------------------------------------

// BienBanDayDuDeSua reads one live meeting WITH its body and attendees, `FOR UPDATE`, inside the
// caller's transaction. It is the read PATCH needs: the no-op comparison and the before/after lengths
// of the audit entry are made against the row as it is under the lock, never against a copy read
// earlier.
//
// Conclusions are NOT loaded: an edit of the minutes touches none of them.
func (s *BienBanHopStore) BienBanDayDuDeSua(ctx context.Context, tx *store.ScopedTx, id string) (
	domain.BienBanHop, error) {

	const stmt = `SELECT ` + cotBienBanChiTiet + ` FROM bien_ban_hop
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR UPDATE`

	b, err := quetBienBanDoc(tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), id), true)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.BienBanHop{}, ErrBienBanKhongTonTai
	}
	if err != nil {
		// NOT the id and NOT any column: this register's columns hold minutes (rule 3).
		return domain.BienBanHop{}, fmt.Errorf("bien_ban_hop: đọc biên bản để sửa: %w", err)
	}
	return b, nil
}

// KetLuanTheoThuTuDeSua reads ONE live conclusion by the pair `(bien_ban_id, thu_tu)`, `FOR UPDATE`,
// inside the caller's transaction — WITH the no-task mark, which the write acts decide on.
//
// THE CALLER HOLDS THE PARENT'S LOCK FIRST (BienBanTheoIDDeSua). That order is what serialises every
// act on one meeting's conclusions — edit, delete, the mark, and the split (app.kiemNguonKetLuan) —
// on the one row they all have in common.
func (s *BienBanHopStore) KetLuanTheoThuTuDeSua(ctx context.Context, tx *store.ScopedTx,
	bienBanID string, thuTu int) (domain.KetLuanHop, error) {

	const stmt = `SELECT ` + cotKetLuanSua + ` FROM ket_luan_hop
		WHERE tenant_id = $1 AND bien_ban_id = $2 AND thu_tu = $3 AND deleted_at IS NULL FOR UPDATE`

	var k domain.KetLuanHop
	err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), bienBanID, thuTu).
		Scan(&k.ID, &k.BienBanID, &k.ThuTu, &k.NoiDung, &k.TaoLuc, &k.KhongPhatSinh)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.KetLuanHop{}, ErrKetLuanKhongTonTai
	}
	if err != nil {
		return domain.KetLuanHop{}, fmt.Errorf("ket_luan_hop: đọc kết luận để sửa: %w", err)
	}
	return k, nil
}

// SoNhiemVuSongCuaKetLuan counts the LIVE tasks split from one conclusion — decision 3's lock and
// decision 4's precondition. By the blurred pair, source code bound as a parameter.
func (s *BienBanHopStore) SoNhiemVuSongCuaKetLuan(ctx context.Context, tx *store.ScopedTx,
	ketLuanID string) (int, error) {

	const stmt = `SELECT count(*) FROM nhiem_vu
		WHERE tenant_id = $1 AND nguon_giao = $2 AND nguon_id = $3 AND deleted_at IS NULL`

	var n int
	err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()),
		string(domain.NguonKetLuanHop), ketLuanID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("nhiem_vu: đếm nhiệm vụ của kết luận: %w", err)
	}
	return n, nil
}

// SoNhiemVuSongCuaBienBan counts the LIVE tasks pointing at ANY conclusion of one meeting — the
// precondition of removing the minutes.
//
// SOFT-DELETED CONCLUSIONS ARE COUNTED TOO, on purpose and conservatively: a conclusion can only be
// removed while it has no live task, but a task can reach one by a path this register does not
// guard (POST /api/v1/tasks accepts a `ket-luan-hop` source — reported), and a meeting removed from
// under a live task is a back-link to nothing.
//
// BOTH TABLES BOUND TO $1 — the subquery included — so no other commune's conclusion id can match.
func (s *BienBanHopStore) SoNhiemVuSongCuaBienBan(ctx context.Context, tx *store.ScopedTx,
	bienBanID string) (int, error) {

	const stmt = `SELECT count(*) FROM nhiem_vu
		WHERE tenant_id = $1 AND nguon_giao = $2 AND deleted_at IS NULL
		  AND nguon_id IN (SELECT id FROM ket_luan_hop WHERE tenant_id = $1 AND bien_ban_id = $3)`

	var n int
	err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()),
		string(domain.NguonKetLuanHop), bienBanID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("nhiem_vu: đếm nhiệm vụ của biên bản: %w", err)
	}
	return n, nil
}

// --- the minutes ------------------------------------------------------------------------------------

// SuaBienBan writes every editable column of a DRAFT from `b` — the row the use case read under the
// lock with the request applied. Notice included: on a draft it may still be typed or corrected.
//
// A WHOLE-ROW WRITE OF THE EDITABLE SET, not a dynamic SET list: one statement text for every edit is
// one statement to review, and the use case has already decided nothing changed means no call at
// all. Absent optional values go in as NULL (rongThanhNull) — ” is a broken reference to the CHECKs.
func (s *BienBanHopStore) SuaBienBan(ctx context.Context, tx *store.ScopedTx, b domain.BienBanHop) error {
	const stmt = `UPDATE bien_ban_hop SET
		ten_cuoc_hop = $3, ngay_hop = $4, so_hieu = $5, dia_diem = $6, chu_tri_ma = $7,
		thu_ky_ma = $8, thanh_phan = $9::jsonb, noi_dung = $10, tb_so_ky_hieu = $11, tb_ngay = $12,
		cap_nhat_luc = now()
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL AND trang_thai = $13`

	thanhPhan, err := jsonDanhSach(b.ThanhPhan)
	if err != nil {
		return err
	}
	kq, err := tx.Exec(ctx, stmt, string(tx.TenantID()), b.ID,
		b.TenCuocHop, b.NgayHop, rongThanhNull(b.SoHieu), rongThanhNull(b.DiaDiem),
		rongThanhNull(b.ChuTriMa), rongThanhNull(b.ThuKyMa), thanhPhan, rongThanhNull(b.NoiDung),
		rongThanhNull(b.TbSoKyHieu), ngayHoacNull(b.TbNgay), domain.TrangThaiBienBanDuThao)
	if err != nil {
		return boiTuChoiHoSoDaKy(fmt.Errorf("bien_ban_hop: sửa biên bản: %w", err))
	}
	return motDong(kq, domain.ErrBienBanDaKy, "bien_ban_hop: sửa biên bản")
}

// GhiThongBao records the conclusion notice on SIGNED minutes — the one write migration 0012's
// trigger still allows after signing, and only once (NULL -> value). The WHERE clause carries both
// conditions, so a second write matches no row and is refused as ErrDaCoThongBao.
func (s *BienBanHopStore) GhiThongBao(ctx context.Context, tx *store.ScopedTx, id, so string,
	ngay time.Time) error {

	const stmt = `UPDATE bien_ban_hop SET tb_so_ky_hieu = $3, tb_ngay = $4, cap_nhat_luc = now()
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL AND trang_thai = $5
		  AND tb_so_ky_hieu IS NULL AND tb_ngay IS NULL`

	kq, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id, so, ngay, domain.TrangThaiBienBanDaKy)
	if err != nil {
		return boiTuChoiHoSoDaKy(fmt.Errorf("bien_ban_hop: ghi thông báo kết luận: %w", err))
	}
	return motDong(kq, domain.ErrDaCoThongBao, "bien_ban_hop: ghi thông báo kết luận")
}

// KyBienBan moves a DRAFT to `da-ky`, stamping the instant and the signer's STAFF BUSINESS CODE (rule
// 6, invariant 8 — the CHECK `bien_ban_hop_ky_du_truong` holds the three together). The notice may
// ride along: an empty `tbSo` leaves the notice columns as they are (COALESCE over NULL parameters).
//
// THE WHERE CLAUSE CARRIES `trang_thai = 'du-thao'`, so signing twice matches no row: the second
// request is refused (ErrBienBanDaKy), it never records a second signature.
func (s *BienBanHopStore) KyBienBan(ctx context.Context, tx *store.ScopedTx, id string, luc time.Time,
	boiMa, tbSo string, tbNgay time.Time) error {

	const stmt = `UPDATE bien_ban_hop SET trang_thai = $3, ky_luc = $4, ky_boi_ma = $5,
		tb_so_ky_hieu = COALESCE($6, tb_so_ky_hieu), tb_ngay = COALESCE($7, tb_ngay),
		cap_nhat_luc = now()
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL AND trang_thai = $8`

	kq, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id, domain.TrangThaiBienBanDaKy, luc, boiMa,
		rongThanhNull(tbSo), ngayHoacNull(tbNgay), domain.TrangThaiBienBanDuThao)
	if err != nil {
		return boiTuChoiHoSoDaKy(fmt.Errorf("bien_ban_hop: ký biên bản: %w", err))
	}
	return motDong(kq, domain.ErrBienBanDaKy, "bien_ban_hop: ký biên bản")
}

// XoaMemBienBan soft deletes a DRAFT (rule 7, invariant 1: all three columns, CHECK-enforced). A
// signed record is refused here AND by the trigger. Its conclusions are NOT touched: every read of
// them joins the live meeting (cauKetLuanSong) or starts from live meetings, so they leave every
// screen with it, and their rows stay as they were typed.
func (s *BienBanHopStore) XoaMemBienBan(ctx context.Context, tx *store.ScopedTx, id, boiMa,
	lyDo string, luc time.Time) error {

	const stmt = `UPDATE bien_ban_hop SET deleted_at = $3, deleted_by = $4, delete_reason = $5,
		cap_nhat_luc = now()
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL AND trang_thai = $6`

	kq, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id, luc, boiMa, lyDo,
		domain.TrangThaiBienBanDuThao)
	if err != nil {
		return boiTuChoiHoSoDaKy(fmt.Errorf("bien_ban_hop: xoá mềm biên bản: %w", err))
	}
	return motDong(kq, domain.ErrBienBanDaKy, "bien_ban_hop: xoá mềm biên bản")
}

// --- one conclusion ---------------------------------------------------------------------------------

// SuaKetLuan rewrites the text of one live conclusion. The parent's state is the use case's check
// and the trigger's; the "no live task" rule is the use case's (decision 3, app-enforced — 0012).
func (s *BienBanHopStore) SuaKetLuan(ctx context.Context, tx *store.ScopedTx, id, noiDung string) error {
	const stmt = `UPDATE ket_luan_hop SET noi_dung = $3, cap_nhat_luc = now()
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

	kq, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id, noiDung)
	if err != nil {
		// NOT the content: a conclusion routinely quotes a case.
		return boiTuChoiHoSoDaKy(fmt.Errorf("ket_luan_hop: sửa kết luận: %w", err))
	}
	return motDong(kq, ErrKetLuanKhongTonTai, "ket_luan_hop: sửa kết luận")
}

// XoaMemKetLuan soft deletes one live conclusion. ITS ORDINAL STAYS TAKEN FOR EVER: ThuTuLonNhat
// counts deleted rows, so the next conclusion continues past it (rule 7, invariant 3).
func (s *BienBanHopStore) XoaMemKetLuan(ctx context.Context, tx *store.ScopedTx, id, boiMa,
	lyDo string, luc time.Time) error {

	const stmt = `UPDATE ket_luan_hop SET deleted_at = $3, deleted_by = $4, delete_reason = $5,
		cap_nhat_luc = now()
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

	kq, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id, luc, boiMa, lyDo)
	if err != nil {
		return boiTuChoiHoSoDaKy(fmt.Errorf("ket_luan_hop: xoá mềm kết luận: %w", err))
	}
	return motDong(kq, ErrKetLuanKhongTonTai, "ket_luan_hop: xoá mềm kết luận")
}

// DatKhongPhatSinh sets the mark with its who/when (CHECK `ket_luan_hop_khong_phat_sinh_du_truong`).
// `AND NOT khong_phat_sinh` — the use case treats an already-set mark as a no-op and never calls this.
func (s *BienBanHopStore) DatKhongPhatSinh(ctx context.Context, tx *store.ScopedTx, id string,
	luc time.Time, boiMa string) error {

	const stmt = `UPDATE ket_luan_hop SET khong_phat_sinh = true, khong_phat_sinh_luc = $3,
		khong_phat_sinh_boi_ma = $4, cap_nhat_luc = now()
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL AND NOT khong_phat_sinh`

	kq, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id, luc, boiMa)
	if err != nil {
		return boiTuChoiHoSoDaKy(fmt.Errorf("ket_luan_hop: đánh dấu không phát sinh: %w", err))
	}
	return motDong(kq, ErrKetLuanKhongTonTai, "ket_luan_hop: đánh dấu không phát sinh")
}

// BoKhongPhatSinh clears the mark AND its who/when together (the same CHECK). The trail of both acts
// lives in the audit ledger, not in these columns.
func (s *BienBanHopStore) BoKhongPhatSinh(ctx context.Context, tx *store.ScopedTx, id string) error {
	const stmt = `UPDATE ket_luan_hop SET khong_phat_sinh = false, khong_phat_sinh_luc = NULL,
		khong_phat_sinh_boi_ma = NULL, cap_nhat_luc = now()
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL AND khong_phat_sinh`

	kq, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id)
	if err != nil {
		return boiTuChoiHoSoDaKy(fmt.Errorf("ket_luan_hop: bỏ dấu không phát sinh: %w", err))
	}
	return motDong(kq, ErrKetLuanKhongTonTai, "ket_luan_hop: bỏ dấu không phát sinh")
}

// --- shared --------------------------------------------------------------------------------------------

// motDong refuses an UPDATE that did not touch exactly one row — invariant 6 of the header.
func motDong(kq sql.Result, khiKhong error, viec string) error {
	n, err := kq.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: đọc số dòng: %w", viec, err)
	}
	if n != 1 {
		return khiKhong
	}
	return nil
}

// ngayHoacNull binds a calendar day, or NULL for the zero value.
func ngayHoacNull(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

// sqlStateLoi is the one method a driver error needs for this file to read its SQLSTATE.
// pgconn.PgError has it; declaring the interface here keeps the driver out of the store's imports.
type sqlStateLoi interface{ SQLState() string }

// boiTuChoiHoSoDaKy translates migration 0012's trigger refusals (SQLSTATE P0001, `RAISE EXCEPTION`)
// into the domain's 409 sentinels, keeping the driver error in the chain for the operator's log.
//
// THE MESSAGE IS OURS (migration 0012 wrote it), which is why matching a fragment of it is not
// guessing: "recorded once" is the notice trigger, everything else on these two tables is the
// signed-record lock. Any other error passes through unchanged and answers 500, as it should.
func boiTuChoiHoSoDaKy(err error) error {
	var e sqlStateLoi
	if !errors.As(err, &e) || e.SQLState() != "P0001" {
		return err
	}
	if strings.Contains(err.Error(), "recorded once") {
		return fmt.Errorf("%w: %w", domain.ErrDaCoThongBao, err)
	}
	return fmt.Errorf("%w: %w", domain.ErrBienBanDaKy, err)
}
