package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// MucUuTienNhiemVuStore reads the commune's task-priority scale (`muc_uu_tien_nhiem_vu`,
// migration 0003).
//
// A SEPARATE STORE FROM LoaiNhiemVuStore, although the two tables have the same shape and the two
// queries differ in one identifier. They are two catalogues with two ceilings and two failure
// modes, and a shared reader would mean one `bang string` parameter deciding which commune
// catalogue is read — a table name arriving from the caller is the shape rule 1, forbidden #2
// warns about in its SQL form. The duplication is a dozen lines; the alternative is a query that
// can be pointed at any table in the schema.
type MucUuTienNhiemVuStore struct {
	db *store.DB
}

// NewMucUuTienNhiemVuStore takes *store.DB and nothing else — see NewLoaiNhiemVuStore.
func NewMucUuTienNhiemVuStore(db *store.DB) *MucUuTienNhiemVuStore {
	return &MucUuTienNhiemVuStore{db: db}
}

// TranDanhMucMucUuTien is the hard upper bound on one commune's priority scale.
//
// DELIBERATELY SMALLER THAN TranDanhMucLoaiNhiemVu, because this is a SCALE. The specification
// ships three levels (docs/ui-ux/14-cau-hinh.md §5), and a scale is only usable while a person can
// hold all of it in their head at once: fifty levels is not a large commune, it is a list where
// nobody can say whether one task outranks another. The ceiling marks the point past which the rows
// have stopped being a scale, not how many levels a commune might reasonably want.
const TranDanhMucMucUuTien = 50

// ErrQuaNhieuMucUuTien says the ceiling was reached. The caller answers 500 and refuses.
//
// REFUSE RATHER THAN TRUNCATE, for a reason one step worse than the type catalogue's: the levels
// arrive in rank order, so a truncated list does not merely lose an option — it loses the options
// at ONE END of the scale. Whichever end that is, the picker then offers a scale that silently
// stops short, and every task filed from it is ranked wrong.
var ErrQuaNhieuMucUuTien = errors.New("muc_uu_tien_nhiem_vu: vượt trần danh mục")

// cotMucUuTien IS READ BY POSITION in DanhSach — same note as cotLoaiNhiemVu: `ma`/`nhan` are
// adjacent TEXT columns and `la_mac_dinh`/`dang_dung` adjacent BOOLEANs, and swapping either pair
// raises no error anywhere.
const cotMucUuTien = `id, ma, nhan, la_mac_dinh, dang_dung`

// DanhSach reads the commune's whole priority scale, IN RANK ORDER.
//
// THE ORDER IS THE MEANING OF THIS LIST, not a presentation detail. "Khẩn" is urgent only relative
// to the levels around it, so a scale rendered in the wrong order is wrong in a way nobody reports
// as a bug: every screen still shows three plausible words. ORDER BY thu_tu is what produces the
// rank; the tie-break is `ma`, which carries UNIQUE (tenant_id, ma) in migration 0003 and therefore
// makes the order TOTAL — two levels sharing a `thu_tu` cannot swap places between two calls, and
// a test of this cannot accidentally compare sets instead of sequences.
//
// Not paginated, for the reasons written on LoaiNhiemVuStore.DanhSach; the bound is
// TranDanhMucMucUuTien, enforced in SQL. Rows out of use are returned, soft-deleted rows are not
// (rule 7, invariant 2).
//
// THE COMMUNE IS NOT A PARAMETER AND CANNOT BE ONE: Scoped.Query binds it to $1 from the context.
func (s *MucUuTienNhiemVuStore) DanhSach(ctx context.Context) ([]domain.MucUuTienNhiemVu, error) {
	// Ceiling PLUS ONE — a full page of exactly the ceiling is indistinguishable from a complete
	// list of that size, which is the truncation this refuses to perform.
	rows, err := s.db.For(ctx).Query(ctx, cotMucUuTien, "muc_uu_tien_nhiem_vu",
		`AND deleted_at IS NULL ORDER BY thu_tu, ma LIMIT $2`, TranDanhMucMucUuTien+1)
	if err != nil {
		return nil, fmt.Errorf("muc_uu_tien_nhiem_vu: đọc danh mục: %w", err)
	}
	defer rows.Close()

	ra := make([]domain.MucUuTienNhiemVu, 0, 8)
	for rows.Next() {
		var m domain.MucUuTienNhiemVu
		// POSITIONAL — in lockstep with cotMucUuTien.
		if err := rows.Scan(&m.ID, &m.Ma, &m.Nhan, &m.LaMacDinh, &m.DangDung); err != nil {
			return nil, fmt.Errorf("muc_uu_tien_nhiem_vu: đọc dòng: %w", err)
		}
		ra = append(ra, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("muc_uu_tien_nhiem_vu: duyệt kết quả: %w", err)
	}
	if len(ra) > TranDanhMucMucUuTien {
		// Rows already read are DROPPED rather than trimmed and returned — see the same note on
		// LoaiNhiemVuStore.DanhSach.
		return nil, ErrQuaNhieuMucUuTien
	}
	return ra, nil
}
