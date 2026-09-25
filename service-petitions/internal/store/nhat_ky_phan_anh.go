package store

// The petition processing logbook `nhat_ky_phan_anh` (migration 0013). SQL, and nothing else.
//
// ONE WRITE METHOD, NO UPDATE AND NO DELETE: the table is APPEND-ONLY and migration 0013 enforces it
// with a trigger (rule 7, forbidden #5). There is no signature here that could edit an entry, so
// "correct the timeline" can only ever mean writing another entry that carries the correction —
// the same shape as the task logbook (nhiem_vu_ghi.go, GhiNhatKy).
//
// ⚠ `noi_dung` MAY HOLD CITIZEN PERSONAL DATA (rule 3). No error built here quotes it, and no error
// built here quotes the lookup code either.

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// GhiNhatKy appends one timeline entry INSIDE the caller's transaction — the transaction of the act
// it describes. A status change committed without its row would leave a gap an officer reads as
// "nothing happened here".
//
// `dinh_kem` IS NOT WRITTEN: it defaults to '[]' and there is no file store yet (migration 0013).
func (s *PhieuPhanAnhStore) GhiNhatKy(ctx context.Context, tx *store.ScopedTx,
	e domain.NhatKyPhanAnh) error {

	const stmt = `INSERT INTO nhat_ky_phan_anh (
		tenant_id, id, phieu_phan_anh_id, thoi_diem, nguoi_ma, hanh_vi, trang_thai_tai_thoi_diem,
		bo_phan_id, can_bo_xu_ly_ma, noi_dung)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`

	_, err := tx.Exec(ctx, stmt, string(tx.TenantID()), e.ID, e.PhieuPhanAnhID, e.ThoiDiem,
		e.NguoiMa, string(e.HanhVi), string(e.TrangThai),
		rongThanhNull(e.BoPhanID), rongThanhNull(e.CanBoXuLyMa), rongThanhNull(e.NoiDung))
	if err != nil {
		// NOT the note: an INSERT error can quote the whole row on some drivers (rule 3).
		return fmt.Errorf("nhat_ky_phan_anh: ghi nhật ký: %w", err)
	}
	return nil
}

// SapXepNhatKyPhieu is the ONE order the timeline offers: newest first, `id` as the tie-break
// QueryPage always appends — two acts of one transaction share a `thoi_diem`. The index
// `nhat_ky_theo_phieu_phan_anh` (migration 0013) is built on exactly (tenant_id, phieu_phan_anh_id,
// thoi_diem DESC, id DESC).
var SapXepNhatKyPhieu = page.NewAllowlist(page.Desc,
	page.Col("at", "thoi_diem", page.KindTime),
)

var mocNhatKyPhieu = store.NewMoc[domain.NhatKyPhanAnh](SapXepNhatKyPhieu,
	map[string]func(domain.NhatKyPhanAnh) page.Key{
		"at": func(e domain.NhatKyPhanAnh) page.Key { return page.TimeKey(e.ThoiDiem) },
	})

const cotNhatKyPhieu = `id, phieu_phan_anh_id, thoi_diem, nguoi_ma, hanh_vi,
	trang_thai_tai_thoi_diem, bo_phan_id, can_bo_xu_ly_ma, noi_dung`

// NhatKyCuaPhieu reads ONE PAGE of one petition's timeline, by the petition's INTERNAL id — which
// the caller has from the row it just read, after the soft-delete and restricted-field checks.
//
// THE COMMUNE IS $1 FROM THE CONTEXT (Scoped.Query), so another commune's petition id matches no row
// even if a caller had one. No soft-delete predicate: the table has none (migration 0013 header).
func (s *PhieuPhanAnhStore) NhatKyCuaPhieu(ctx context.Context, phieuID string, yc page.Request) (
	page.Result[domain.NhatKyPhanAnh], error) {

	return store.QueryPage(ctx, s.db.For(ctx), store.PageSpec{
		Columns: cotNhatKyPhieu,
		Table:   "nhat_ky_phan_anh",
		Filter:  `AND phieu_phan_anh_id = $2`,
		Args:    []any{phieuID},
	}, yc, mocNhatKyPhieu, func(rows *sql.Rows) (domain.NhatKyPhanAnh, string, error) {
		var (
			e                      domain.NhatKyPhanAnh
			hanhVi, trangThai      string
			boPhan, canBo, noiDung sql.NullString
		)
		if err := rows.Scan(&e.ID, &e.PhieuPhanAnhID, &e.ThoiDiem, &e.NguoiMa, &hanhVi,
			&trangThai, &boPhan, &canBo, &noiDung); err != nil {
			return domain.NhatKyPhanAnh{}, "", fmt.Errorf("nhat_ky_phan_anh: quét dòng: %w", err)
		}
		e.HanhVi = domain.HanhViNhatKy(hanhVi)
		e.TrangThai = domain.TrangThai(trangThai)
		e.BoPhanID, e.CanBoXuLyMa, e.NoiDung = boPhan.String, canBo.String, noiDung.String
		return e, e.ID, nil
	})
}
