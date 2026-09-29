package store

import (
	"context"
	"errors"
	"fmt"

	corestore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-platform/internal/domain"
)

// ErrCommuneProfileNotDeclared — the commune in context has no live display profile. An ordinary state
// (a commune that has not filled it in yet), not a failure.
var ErrCommuneProfileNotDeclared = errors.New("ho_so_hien_thi_xa: xã chưa khai hồ sơ hiển thị")

// CommuneProfileStore is the only path to `ho_so_hien_thi_xa`.
//
// UNLIKE Directory, IT IS BUILT FROM *corestore.DB AND READS ONLY THROUGH Scoped: the profile is
// a commune's own content, so the commune comes from the context and is $1 of every statement
// (rule 1, invariants 4 and 5). There is no method here that takes a commune as an argument.
type CommuneProfileStore struct {
	db *corestore.DB
}

func NewCommuneProfileStore(db *corestore.DB) *CommuneProfileStore {
	return &CommuneProfileStore{db: db}
}

// communeProfileColumns IS READ BY POSITION in Read. logo_url is the one nullable column (see the
// migration), folded to "" here so the domain has one spelling of "not declared".
const communeProfileColumns = `dia_chi_tru_so, COALESCE(logo_url, ''), duong_day_nong, ` +
	`gio_lam_viec_hien_thi, gioi_thieu`

// communeProfileFilter excludes soft-deleted rows — everywhere, always (rule 7, invariant 2).
const communeProfileFilter = "AND deleted_at IS NULL"

// Read reads the display profile of the commune in ctx. Soft-deleted rows are excluded (rule 7,
// invariant 2). Panics when ctx carries no commune — tenant.MustFrom, deliberately.
func (s *CommuneProfileStore) Read(ctx context.Context) (domain.CommuneProfile, error) {
	rows, err := s.db.For(ctx).Query(ctx, communeProfileColumns, "ho_so_hien_thi_xa", communeProfileFilter)
	if err != nil {
		return domain.CommuneProfile{}, fmt.Errorf("ho_so_hien_thi_xa: truy vấn: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.CommuneProfile{}, fmt.Errorf("ho_so_hien_thi_xa: đọc: %w", err)
		}
		return domain.CommuneProfile{}, ErrCommuneProfileNotDeclared
	}
	var out domain.CommuneProfile
	if err := rows.Scan(&out.OfficeAddress, &out.LogoURL, &out.Hotline,
		&out.OfficeHoursText, &out.Introduction); err != nil {
		return domain.CommuneProfile{}, fmt.Errorf("ho_so_hien_thi_xa: quét: %w", err)
	}
	// The primary key is tenant_id, so a second row is impossible; rows.Close() via defer.
	return out, nil
}
