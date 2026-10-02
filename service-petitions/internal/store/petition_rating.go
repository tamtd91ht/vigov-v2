package store

// The citizen's star rating — ADR 0050 point 2. SQL, and nothing else.
//
// THE SAME FIVE PROPERTIES AS xu_ly_phan_anh.go HOLD HERE: the commune is $1 from the transaction; no
// method opens a transaction; every statement excludes soft-deleted rows; every UPDATE carries the
// expected status; and NO DEADLINE COLUMN APPEARS IN ANY UPDATE. The last one is the property the
// reopen most needs (rule 10, invariant 2; ADR 0050 — "không tính lại hạn"): the reopened petition keeps
// `han_tiep_nhan`, `han_phan_loai` and `han_xu_ly_xong` exactly as they were promised.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// CitizenPetitionForUpdate reads ONE live petition of THIS citizen, in THIS commune, inside the
// caller's transaction, and holds it until the transaction ends.
//
// BOTH AXES OF RULE 4, INVARIANT 3, AND THE LOCK OF THE STAFF PATH. It is the locking twin of
// CuaCongDanTheoMaTraCuu: `cong_dan_id = $3` is a bound parameter with no value that switches it off,
// so another citizen's code, another commune's code, a soft-deleted petition and an unknown code all
// come back as ErrPhieuKhongTonTai — the ONE answer the citizen route turns into its one 404 body.
//
// `FOR UPDATE` FOR THE REASON TheoMaTraCuuDeSua GIVES: rating is a read-decide-write. Without the lock,
// a rating racing a staff act (say, the closing) would decide against a status that has already moved.
//
// congDanID comes from the SESSION and nowhere else — see CuaCongDanTheoMaTraCuu.
func (s *PhieuPhanAnhStore) CitizenPetitionForUpdate(ctx context.Context, tx *store.ScopedTx,
	congDanID, ma string) (domain.PhieuPhanAnh, error) {

	if congDanID == "" {
		// FAIL CLOSED, BEFORE THE QUERY — the same refusal and sentinel as CuaCongDanTheoMaTraCuu.
		return domain.PhieuPhanAnh{}, ErrThieuDinhDanhCongDan
	}

	const stmt = `SELECT ` + cotPhieu + ` FROM phieu_phan_anh
		WHERE tenant_id = $1 AND ma_tra_cuu = $2 AND cong_dan_id = $3 AND deleted_at IS NULL FOR UPDATE`

	p, err := quetPhieu(tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), ma, congDanID))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.PhieuPhanAnh{}, ErrPhieuKhongTonTai
	}
	if err != nil {
		// NEITHER THE CODE NOR THE CITIZEN IDENTIFIER IS IN THE WRAPPED MESSAGE (rule 3).
		return domain.PhieuPhanAnh{}, fmt.Errorf("phieu_phan_anh: đọc phiếu của công dân để đánh giá: %w", err)
	}
	return p, nil
}

// RecordRating stores a rating that does NOT reopen the petition (3–5 stars). THE STATUS IS NOT IN THE
// SET LIST — it is only in the WHERE clause — so this statement cannot move the petition at all.
//
// A LATER RATING REPLACES AN EARLIER ONE (ADR 0050 point 2, "lần mới thay lần cũ"). The earlier values
// are kept in the audit entry's `truoc`, which is written in the same transaction by the caller.
//
// `rating_comment` "" becomes NULL — "no comment" — which migration 0017's CHECK requires rather than a
// blank string. A re-rating without a comment therefore CLEARS the earlier comment: the new verdict
// replaces the old one whole, never half of it.
func (s *PhieuPhanAnhStore) RecordRating(ctx context.Context, tx *store.ScopedTx, id string,
	tuTrangThai domain.TrangThai, stars int, comment string, at time.Time) error {

	const stmt = `UPDATE phieu_phan_anh
		SET diem_hai_long = $3, rating_comment = $4, danh_gia_luc = $5, cap_nhat_luc = now()
		WHERE tenant_id = $1 AND id = $2 AND trang_thai = $6 AND deleted_at IS NULL`

	kq, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id,
		stars, rongThanhNull(comment), at, string(tuTrangThai))
	if err != nil {
		// NOT the comment: citizen free text, and an error travels into centralised logging (rule 3).
		return fmt.Errorf("phieu_phan_anh: ghi đánh giá: %w", err)
	}
	return doiMotDongPhieu(kq, "ghi đánh giá")
}

// ReopenByRating stores a 1–2 star rating AND reopens the petition, in ONE statement.
//
// ONE STATEMENT BECAUSE IT IS ONE ACT: a window in which the low rating is recorded and the petition
// still reads "đã xử lý" — or the reverse — is a register that contradicts itself.
//
//	trang_thai      'dang-xu-ly', A LITERAL — no layer above can pass a status to this method
//	so_lan_mo_lai   + 1, and NO CAP (ADR 0050: "không trần số lần"); the counter is a monitoring
//	                figure now, and `Occurrence` on the event is read from it
//	xu_ly_xong_luc  NULL. It is the "resolved" instant domain.QuaHan compares against once the work is
//	                done; left in place, a reopened petition would stay "on time" for ever on the
//	                strength of work the citizen just said was not finished. Cleared, the petition is
//	                measured against its UNCHANGED deadline from now on — which is ADR 0050 §Cái giá
//	                ("phiếu mở lại thường đã quá hạn ngay lúc mở lại, và số liệu quá hạn … phản ánh
//	                đúng điều đó"). The next finishing step writes it again (DoiTrangThai).
//	dong_luc        NULL — the "closed" instant, as the requirement repository's service.py:821 clears
//	                `closed_at`. It is already NULL at both statuses a rating is allowed at; clearing it
//	                keeps the reopened row free of a closing that is no longer true.
//
// THE THREE DEADLINES ARE NOT IN THIS STATEMENT, and that absence is rule 10, invariant 2.
func (s *PhieuPhanAnhStore) ReopenByRating(ctx context.Context, tx *store.ScopedTx, id string,
	tuTrangThai domain.TrangThai, stars int, comment string, at time.Time) error {

	const stmt = `UPDATE phieu_phan_anh
		SET diem_hai_long = $3, rating_comment = $4, danh_gia_luc = $5,
		    trang_thai = 'dang-xu-ly', so_lan_mo_lai = so_lan_mo_lai + 1,
		    xu_ly_xong_luc = NULL, dong_luc = NULL, cap_nhat_luc = now()
		WHERE tenant_id = $1 AND id = $2 AND trang_thai = $6 AND deleted_at IS NULL`

	kq, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id,
		stars, rongThanhNull(comment), at, string(tuTrangThai))
	if err != nil {
		return fmt.Errorf("phieu_phan_anh: mở lại theo đánh giá: %w", err)
	}
	return doiMotDongPhieu(kq, "mở lại theo đánh giá")
}

// LatestReopenAtTx answers the instant of the MOST RECENT reopening of petition `petitionID` (its
// internal id), inside the caller's transaction. The zero time means the timeline holds no reopening.
//
// READ FROM THE TIMELINE, NOT FROM THE PETITION ROW, because the row has no column that holds it:
// `danh_gia_luc` is stamped by the reopening but OVERWRITTEN by any later 3–5 star rating (RecordRating
// — "lần mới thay lần cũ"), so after reopen → photo → resolve → 4 stars it would name the 4-star rating
// and make the photo look older than the reopening. The `mo-lai-theo-danh-gia` row of `nhat_ky_phan_anh`
// is written in the reopening's own transaction (app.RatePetition), `thoi_diem` is that act's instant,
// and the table is append-only (migration 0013), so the instant can never move. The index
// `nhat_ky_theo_phieu_phan_anh` (tenant_id, phieu_phan_anh_id, thoi_diem DESC, id DESC) serves it.
//
// No soft-delete predicate: the table has none (migration 0013 header).
func (s *PhieuPhanAnhStore) LatestReopenAtTx(ctx context.Context, tx *store.ScopedTx, petitionID string) (
	time.Time, error) {

	// ScopedTx.Query prefixes `WHERE tenant_id = $1` and binds the commune from the transaction.
	rows, err := tx.Query(ctx, "max(thoi_diem)", "nhat_ky_phan_anh",
		`AND phieu_phan_anh_id = $2 AND hanh_vi = $3`, petitionID, string(domain.LogActionReopenByRating))
	if err != nil {
		return time.Time{}, fmt.Errorf("nhat_ky_phan_anh: đọc lần mở lại gần nhất: %w", err)
	}
	defer rows.Close()
	var at sql.NullTime
	if rows.Next() {
		if err := rows.Scan(&at); err != nil {
			return time.Time{}, fmt.Errorf("nhat_ky_phan_anh: quét lần mở lại gần nhất: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		return time.Time{}, fmt.Errorf("nhat_ky_phan_anh: duyệt lần mở lại gần nhất: %w", err)
	}
	if !at.Valid {
		return time.Time{}, nil
	}
	return at.Time.UTC(), nil
}
