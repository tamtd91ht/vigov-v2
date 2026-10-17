package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// The petitions side of the petition ↔ citizen-directory link (ADR 0102, migration 0046). Three reads
// and one write over `sender_phone_normalized`:
//
//	SenderMatchKey                the stored key of ONE petition (the "Đồng bộ danh bạ" match)
//	ContactPetitions / Count...   one person's petitions, by key (the directory's "Xem phản ánh")
//	SenderPhoneBackfillBatch +    the per-commune backfill of rows written before 0046
//	SetSenderPhoneMatchKey
//
// ⚠ PERSONAL DATA (rule 3): the key IS the citizen's number. It is a bind parameter here and nothing
// else — never in an error, never in a log line. Every error below names the operation only.

// SenderMatchKey reads the stored match key of one LIVE petition of this commune, by internal id. ""
// when the column is NULL (no usable number, or a row the backfill has not reached). The caller has
// already read the petition through TheoMaTraCuu, so the commune and soft-delete checks were applied;
// they are repeated here because this is its own statement.
func (s *PhieuPhanAnhStore) SenderMatchKey(ctx context.Context, petitionID string) (string, error) {
	rows, err := s.db.For(ctx).Query(ctx, "sender_phone_normalized", "phieu_phan_anh",
		`AND id = $2 AND deleted_at IS NULL`, petitionID)
	if err != nil {
		return "", fmt.Errorf("phieu_phan_anh: đọc khoá đối chiếu danh bạ: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return "", fmt.Errorf("phieu_phan_anh: đọc khoá đối chiếu danh bạ: %w", err)
		}
		return "", ErrPhieuKhongTonTai
	}
	var key sql.NullString
	if err := rows.Scan(&key); err != nil {
		return "", fmt.Errorf("phieu_phan_anh: đọc khoá đối chiếu danh bạ: %w", err)
	}
	return key.String, rows.Err()
}

// contactPetitionFilter is the predicate migration 0046's partial index serves, TEXT-IDENTICAL to its
// header: `sender_phone_normalized = $2 AND an_danh = false AND deleted_at IS NULL`. A different spelling
// of the same predicate stays correct and silently stops using the index.
//
// `an_danh = false` IS ADR 0102 #4: an anonymous petition never appears under anybody's directory row.
// Merged petitions (`merged_into`, 0037) ARE listed: a merged petition is still one this person sent.
const contactPetitionFilter = `AND sender_phone_normalized = $2 AND an_danh = false AND deleted_at IS NULL`

// ContactPetitions is one page of the petitions whose sender's match key is matchPhone, newest booked
// first. `includeRestricted` is the caller's answer to "does this account hold `feedback.restricted`";
// false excludes `can-bo` petitions in the WHERE, exactly as the register list does.
//
// LIMIT/OFFSET and not the keyset QueryPage: the web pages this list by page NUMBER with a total (the
// contract of TASK-04), and one person's petitions are a handful — the offset skips few rows.
func (s *PhieuPhanAnhStore) ContactPetitions(ctx context.Context, matchPhone string, includeRestricted bool,
	limit, offset int) ([]domain.ContactPetition, error) {
	if matchPhone == "" {
		return nil, errors.New("phieu_phan_anh: danh sách phiếu theo danh bạ: thiếu khoá đối chiếu")
	}
	if limit < 1 || offset < 0 {
		return nil, errors.New("phieu_phan_anh: danh sách phiếu theo danh bạ: trang không hợp lệ")
	}
	tail := contactPetitionFilter
	if !includeRestricted {
		tail += restrictedFieldExclusion
	}
	tail += ` ORDER BY vao_so_luc DESC, id DESC LIMIT $3 OFFSET $4`

	rows, err := s.db.For(ctx).Query(ctx, "ma_tra_cuu, noi_dung, trang_thai, vao_so_luc", "phieu_phan_anh",
		tail, matchPhone, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("phieu_phan_anh: danh sách phiếu theo danh bạ: %w", err)
	}
	defer rows.Close()
	out := []domain.ContactPetition{}
	for rows.Next() {
		var p domain.ContactPetition
		var status string
		if err := rows.Scan(&p.LookupCode, &p.Content, &status, &p.ReceivedAt); err != nil {
			return nil, fmt.Errorf("phieu_phan_anh: danh sách phiếu theo danh bạ: đọc dòng: %w", err)
		}
		p.Status = domain.TrangThai(status)
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("phieu_phan_anh: danh sách phiếu theo danh bạ: %w", err)
	}
	return out, nil
}

// CountContactPetitions is the total under the SAME predicate as ContactPetitions.
func (s *PhieuPhanAnhStore) CountContactPetitions(ctx context.Context, matchPhone string, includeRestricted bool) (int, error) {
	if matchPhone == "" {
		return 0, errors.New("phieu_phan_anh: đếm phiếu theo danh bạ: thiếu khoá đối chiếu")
	}
	tail := contactPetitionFilter
	if !includeRestricted {
		tail += restrictedFieldExclusion
	}
	rows, err := s.db.For(ctx).Query(ctx, "count(*) AS total", "phieu_phan_anh", tail, matchPhone)
	if err != nil {
		return 0, fmt.Errorf("phieu_phan_anh: đếm phiếu theo danh bạ: %w", err)
	}
	defer rows.Close()
	var n int
	if rows.Next() {
		if err := rows.Scan(&n); err != nil {
			return 0, fmt.Errorf("phieu_phan_anh: đếm phiếu theo danh bạ: %w", err)
		}
	}
	return n, rows.Err()
}

// SenderPhoneBackfillRow is one row the backfill still owes a key. RawPhone is PERSONAL DATA held in
// memory for one batch — never logged; the backfill logs counts and the commune id only.
type SenderPhoneBackfillRow struct {
	ID         string
	LookupCode string
	RawPhone   string
}

// SenderPhoneBackfillBatch reads up to `limit` LIVE rows of this commune with a typed number and no key,
// after `afterID` in id order. The id cursor is what makes ONE run finish: a number that does not
// normalise keeps its NULL and would otherwise be read again forever.
//
// SOFT-DELETED ROWS ARE NOT BACKFILLED (rule 7, invariant 2: background jobs exclude them too); no read
// path shows a deleted petition, so its key would never be compared.
func (s *PhieuPhanAnhStore) SenderPhoneBackfillBatch(ctx context.Context, afterID string, limit int) (
	[]SenderPhoneBackfillRow, error) {
	if limit < 1 {
		return nil, errors.New("phieu_phan_anh: điền khoá đối chiếu: cỡ lô không hợp lệ")
	}
	rows, err := s.db.For(ctx).Query(ctx, "id, ma_tra_cuu, nguoi_gui_dien_thoai", "phieu_phan_anh",
		`AND sender_phone_normalized IS NULL AND nguoi_gui_dien_thoai <> '' AND deleted_at IS NULL
		 AND id > $2 ORDER BY id LIMIT $3`, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("phieu_phan_anh: điền khoá đối chiếu: đọc lô: %w", err)
	}
	defer rows.Close()
	var out []SenderPhoneBackfillRow
	for rows.Next() {
		var r SenderPhoneBackfillRow
		if err := rows.Scan(&r.ID, &r.LookupCode, &r.RawPhone); err != nil {
			return nil, fmt.Errorf("phieu_phan_anh: điền khoá đối chiếu: đọc dòng: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("phieu_phan_anh: điền khoá đối chiếu: đọc lô: %w", err)
	}
	return out, nil
}

// SetSenderPhoneMatchKey writes the key of ONE row inside the caller's transaction (its audit entry is
// written in the same one). A SINGLE-COLUMN UPDATE: migration 0046 §5 states it passes every trigger on
// the table; `cap_nhat_luc` is not touched — a derived key is not a change to the record.
//
// The WHERE carries `sender_phone_normalized IS NULL` AND the raw number the key was derived from, so a
// row keyed meanwhile (by a newer intake build, or a second run) or whose number changed is left alone:
// false, nothing written, and the caller writes no audit entry for it. That is what makes a re-run free.
func (s *PhieuPhanAnhStore) SetSenderPhoneMatchKey(ctx context.Context, tx *store.ScopedTx,
	id, rawPhone, key string) (bool, error) {
	if key == "" {
		return false, errors.New("phieu_phan_anh: điền khoá đối chiếu: khoá rỗng — dùng NULL, không bao giờ ''")
	}
	const stmt = `UPDATE phieu_phan_anh SET sender_phone_normalized = $3
		WHERE tenant_id = $1 AND id = $2 AND sender_phone_normalized IS NULL
		  AND nguoi_gui_dien_thoai = $4 AND deleted_at IS NULL`
	kq, err := tx.Exec(ctx, stmt, string(tx.TenantID()), id, key, rawPhone)
	if err != nil {
		return false, fmt.Errorf("phieu_phan_anh: điền khoá đối chiếu: ghi: %w", err)
	}
	n, err := kq.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("phieu_phan_anh: điền khoá đối chiếu: đếm dòng: %w", err)
	}
	return n == 1, nil
}
