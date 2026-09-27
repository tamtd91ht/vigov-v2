package store

// The READ of the task progress log `nhat_ky_nhiem_vu` (migration 0006, §5.9). SQL, and nothing else.
//
// ITS WRITE LIVES IN nhiem_vu_ghi.go (GhiNhatKy), inside the transaction of the act it describes, and
// so does NhatKyGanNhat — the narrow read the resume rule takes under the row lock. This file is the
// page a screen renders, shaped like the petition logbook's read (nhat_ky_phan_anh.go,
// NhatKyCuaPhieu) so the two timelines page, order and scan alike.
//
// ⚠ `noi_dung` IS FREE TEXT AN OFFICER TYPED ABOUT THE WORK (rule 3). No error built here quotes it,
// and none quotes the task number either.

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// SapXepNhatKyNhiemVu is the ONE order the timeline offers: newest first, `id` as the tie-break
// QueryPage always appends — two rows of one transaction share a `thoi_diem`. The same order, and the
// same single param `at`, as SapXepNhatKyPhieu: a screen reading both logs reads them the same way.
//
// The index `nhat_ky_theo_nhiem_vu` (migration 0006) is (tenant_id, nhiem_vu_id, thoi_diem DESC) —
// WITHOUT the `id DESC` that the petition logbook's index carries. The tie-break is therefore settled
// by a sort over the rows sharing one instant — a handful per task. Reported, not migrated here.
var SapXepNhatKyNhiemVu = page.NewAllowlist(page.Desc,
	page.Col("at", "thoi_diem", page.KindTime),
)

var mocNhatKyNhiemVu = store.NewMoc[domain.NhatKyNhiemVu](SapXepNhatKyNhiemVu,
	map[string]func(domain.NhatKyNhiemVu) page.Key{
		"at": func(e domain.NhatKyNhiemVu) page.Key { return page.TimeKey(e.ThoiDiem) },
	})

// `dinh_kem` IS NOT READ: nothing writes it (no file store yet, migration 0006), and the petition
// logbook leaves its twin off the wire for the reason nhatKyPhieuRa gives.
const cotNhatKyNhiemVu = `id, nhiem_vu_id, thoi_diem, nguoi_ma, trang_thai_tai_thoi_diem,
	bo_phan_id, nguoi_phu_trach_ma, noi_dung`

// NhatKyCuaNhiemVu reads ONE PAGE of one task's timeline, by the task's INTERNAL id — which the
// caller has from the row TheoMa just read, after the soft-delete check. ONE STATEMENT PER PAGE.
//
// THE COMMUNE IS $1 FROM THE CONTEXT (Scoped.Query), so another commune's task id matches no row even
// if a caller had one. No soft-delete predicate: the table has none, it is append-only (migration
// 0006) — the soft-delete of the TASK is enforced by the read that produced the id.
func (s *NhiemVuStore) NhatKyCuaNhiemVu(ctx context.Context, nhiemVuID string, yc page.Request) (
	page.Result[domain.NhatKyNhiemVu], error) {

	return store.QueryPage(ctx, s.db.For(ctx), store.PageSpec{
		Columns: cotNhatKyNhiemVu,
		Table:   "nhat_ky_nhiem_vu",
		Filter:  `AND nhiem_vu_id = $2`,
		Args:    []any{nhiemVuID},
	}, yc, mocNhatKyNhiemVu, func(rows *sql.Rows) (domain.NhatKyNhiemVu, string, error) {
		var (
			e                     domain.NhatKyNhiemVu
			trangThai             string
			boPhan, nguoiPhuTrach sql.NullString
		)
		if err := rows.Scan(&e.ID, &e.NhiemVuID, &e.ThoiDiem, &e.NguoiMa, &trangThai,
			&boPhan, &nguoiPhuTrach, &e.NoiDung); err != nil {
			return domain.NhatKyNhiemVu{}, "", fmt.Errorf("nhat_ky_nhiem_vu: quét dòng: %w", err)
		}
		e.TrangThaiTaiThoiDiem = domain.TrangThaiNhiemVu(trangThai)
		e.BoPhanID, e.NguoiPhuTrachMa = boPhan.String, nguoiPhuTrach.String
		return e, e.ID, nil
	})
}
