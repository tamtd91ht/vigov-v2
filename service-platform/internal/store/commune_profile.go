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
var ErrCommuneProfileNotDeclared = errors.New("commune_profile: xã chưa khai hồ sơ hiển thị")

// CommuneProfileStore is the only path to `commune_profile`.
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
const communeProfileColumns = `office_address, COALESCE(logo_url, ''), hotline, ` +
	`office_hours_text, introduction`

// communeProfileFilter excludes soft-deleted rows — everywhere, always (rule 7, invariant 2).
const communeProfileFilter = "AND deleted_at IS NULL"

// Read reads the display profile of the commune in ctx. Soft-deleted rows are excluded (rule 7,
// invariant 2). Panics when ctx carries no commune — tenant.MustFrom, deliberately.
func (s *CommuneProfileStore) Read(ctx context.Context) (domain.CommuneProfile, error) {
	rows, err := s.db.For(ctx).Query(ctx, communeProfileColumns, "commune_profile", communeProfileFilter)
	if err != nil {
		return domain.CommuneProfile{}, fmt.Errorf("commune_profile: truy vấn: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.CommuneProfile{}, fmt.Errorf("commune_profile: đọc: %w", err)
		}
		return domain.CommuneProfile{}, ErrCommuneProfileNotDeclared
	}
	var out domain.CommuneProfile
	if err := rows.Scan(&out.OfficeAddress, &out.LogoURL, &out.Hotline,
		&out.OfficeHoursText, &out.Introduction); err != nil {
		return domain.CommuneProfile{}, fmt.Errorf("commune_profile: quét: %w", err)
	}
	// The primary key is tenant_id, so a second row is impossible; rows.Close() via defer.
	return out, nil
}
