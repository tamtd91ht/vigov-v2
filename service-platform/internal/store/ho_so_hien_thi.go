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
//
// The branding writes (ADR 0069) are in branding.go, on this same type: one table, one store.
type HoSoHienThiStore struct {
	db *corestore.DB
}

func NewHoSoHienThiStore(db *corestore.DB) *HoSoHienThiStore {
	return &HoSoHienThiStore{db: db}
}

// profileReadStmt IS READ BY POSITION in Doc. logo_url is the one nullable text column (see the
// migration), folded to "" here so the domain has one spelling of "not declared".
//
// THE TWO IMAGE KEYS (ADR 0069, migration 0017) come from a LEFT JOIN, so a profile with no image
// still reads. A key is returned only when the referenced file is `ready`, live, of the right purpose
// and published — the trigger of 0017 already refuses pointing at anything else, and this repeats it
// on the read so a file soft-deleted or purged later can never be served. NEVER logo_url as a fallback
// for the logo: logo_url is a typed URL to another system's image (platform.proto, TenantProfile).
//
// Every joined table is constrained to $1 in its ON clause (corestore.QueryJoin's contract): joining on
// id alone would let a file id of another commune complete the row.
const profileReadStmt = `SELECT h.dia_chi_tru_so, COALESCE(h.logo_url, ''), h.duong_day_nong,
	h.gio_lam_viec_hien_thi, h.gioi_thieu,
	COALESCE(l.public_object_key, ''), COALESCE(b.public_object_key, '')
	FROM ho_so_hien_thi_xa h
	LEFT JOIN stored_file l ON l.tenant_id = $1 AND l.id = h.logo_file_id
		AND l.purpose = 'tenant-logo' AND l.status = 'ready' AND l.deleted_at IS NULL
	LEFT JOIN stored_file b ON b.tenant_id = $1 AND b.id = h.web_admin_banner_file_id
		AND b.purpose = 'tenant-banner' AND b.status = 'ready' AND b.deleted_at IS NULL
	WHERE h.tenant_id = $1 AND h.deleted_at IS NULL`

// Doc reads the display profile of the commune in ctx. Soft-deleted rows are excluded (rule 7,
// invariant 2). Panics when ctx carries no commune — tenant.MustFrom, deliberately.
//
// A profile row that carries ONLY branding (every text column the empty string) is returned like any other: the
// upload path creates such a row for a commune that had none (branding.go EnsureProfile).
func (s *HoSoHienThiStore) Doc(ctx context.Context) (domain.HoSoHienThi, error) {
	rows, err := s.db.For(ctx).QueryJoin(ctx, profileReadStmt)
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
		&out.GioLamViecHienThi, &out.GioiThieu, &out.LogoPublicKey, &out.WebAdminBannerPublicKey); err != nil {
		return domain.HoSoHienThi{}, fmt.Errorf("ho_so_hien_thi_xa: quét: %w", err)
	}
	// The primary key is tenant_id, so a second row is impossible; rows.Close() via defer.
	return out, nil
}
