package store

import (
	"context"
	"errors"
	"fmt"

	corestore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-platform/internal/domain"
)

// ErrChuaCoHoSoHienThi — the commune in context has no live display profile. An ordinary state
// (a commune that has not filled it in yet), not a failure.
var ErrChuaCoHoSoHienThi = errors.New("ho_so_hien_thi_xa: xã chưa khai hồ sơ hiển thị")

// HoSoHienThiStore is the only path to `ho_so_hien_thi_xa`.
//
// UNLIKE Directory, IT IS BUILT FROM *corestore.DB AND READS ONLY THROUGH Scoped: the profile is
// a commune's own content, so the commune comes from the context and is $1 of every statement
// (rule 1, invariants 4 and 5). There is no method here that takes a commune as an argument.
type HoSoHienThiStore struct {
	db *corestore.DB
}

func NewHoSoHienThiStore(db *corestore.DB) *HoSoHienThiStore {
	return &HoSoHienThiStore{db: db}
}

// cotHoSoHienThi IS READ BY POSITION in Doc. logo_url is the one nullable column (see the
// migration), folded to "" here so the domain has one spelling of "not declared".
const cotHoSoHienThi = `dia_chi_tru_so, COALESCE(logo_url, ''), duong_day_nong, ` +
	`gio_lam_viec_hien_thi, gioi_thieu`

// duoiHoSoHienThi excludes soft-deleted rows — everywhere, always (rule 7, invariant 2).
const duoiHoSoHienThi = "AND deleted_at IS NULL"

// Doc reads the display profile of the commune in ctx. Soft-deleted rows are excluded (rule 7,
// invariant 2). Panics when ctx carries no commune — tenant.MustFrom, deliberately.
func (s *HoSoHienThiStore) Doc(ctx context.Context) (domain.HoSoHienThi, error) {
	rows, err := s.db.For(ctx).Query(ctx, cotHoSoHienThi, "ho_so_hien_thi_xa", duoiHoSoHienThi)
	if err != nil {
		return domain.HoSoHienThi{}, fmt.Errorf("ho_so_hien_thi_xa: truy vấn: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.HoSoHienThi{}, fmt.Errorf("ho_so_hien_thi_xa: đọc: %w", err)
		}
		return domain.HoSoHienThi{}, ErrChuaCoHoSoHienThi
	}
	var out domain.HoSoHienThi
	if err := rows.Scan(&out.DiaChiTruSo, &out.LogoURL, &out.DuongDayNong,
		&out.GioLamViecHienThi, &out.GioiThieu); err != nil {
		return domain.HoSoHienThi{}, fmt.Errorf("ho_so_hien_thi_xa: quét: %w", err)
	}
	// The primary key is tenant_id, so a second row is impossible; rows.Close() via defer.
	return out, nil
}
