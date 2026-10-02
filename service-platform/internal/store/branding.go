package store

// The branding half of `ho_so_hien_thi_xa` (ADR 0069, migration 0017): the two columns pointing at the
// CURRENT logo and the CURRENT web-admin banner, and the staff read of them.
//
// SQL only. Which file may be pointed at, in which order things happen and what is audited are
// internal/app's (branding.go); 0017's trigger refuses pointing at a file that is not this commune's,
// not of the right purpose, not `ready` or not published, whatever this layer is handed.
//
// NO audit.Write IN THIS FILE — every write takes the caller's *corestore.ScopedTx, and internal/app
// writes the entry inside that same transaction (rule 6, invariant 3).

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	corestore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-platform/internal/domain"
)

// ErrBrandingImageUnknown — an image kind that is neither the logo nor the banner. A programming error,
// refused before any SQL runs: the column name is chosen from a closed pair, never built from input.
var ErrBrandingImageUnknown = errors.New("ho_so_hien_thi_xa: loại ảnh nhận diện không hợp lệ")

const profileForUpdateStmt = `SELECT deleted_at IS NOT NULL, COALESCE(logo_file_id, ''),
	COALESCE(web_admin_banner_file_id, '')
	FROM ho_so_hien_thi_xa WHERE tenant_id = $1 FOR UPDATE`

// ProfileForUpdate locks the commune's profile row inside the transaction. found=false: the commune has
// no row at all.
//
// IT SEES A SOFT-DELETED ROW, ON PURPOSE, and this is the one read of the table that does. It is not a
// read path that shows data (rule 7 invariant 2): it is the write path asking "may I write here", and a
// retired row must make it REFUSE (domain.ErrProfileDeleted) rather than look absent — absent would send
// the caller to EnsureProfile, whose INSERT then collides with the retired row's primary key.
func (s *HoSoHienThiStore) ProfileForUpdate(ctx context.Context, tx *corestore.ScopedTx) (
	p domain.ProfileBranding, found bool, err error) {

	row := tx.Underlying().QueryRowContext(ctx, profileForUpdateStmt, string(tx.TenantID()))
	switch err := row.Scan(&p.Deleted, &p.LogoFileID, &p.BannerFileID); {
	case errors.Is(err, sql.ErrNoRows):
		return domain.ProfileBranding{}, false, nil
	case err != nil:
		return domain.ProfileBranding{}, false, fmt.Errorf("ho_so_hien_thi_xa: khoá dòng hồ sơ: %w", err)
	}
	return p, true, nil
}

// EnsureProfile creates the commune's profile row when it has none: every text column takes its empty-string default
// (0006), logo_url stays NULL, both image columns NULL. `by` is the staff business code for tao_boi and
// cap_nhat_boi (rule 6, invariant 8). created=false: a row already existed — live OR soft-deleted — and
// nothing was written; ON CONFLICT DO NOTHING never revives a retired row.
func (s *HoSoHienThiStore) EnsureProfile(ctx context.Context, tx *corestore.ScopedTx, by string,
	at time.Time) (created bool, err error) {

	const stmt = `INSERT INTO ho_so_hien_thi_xa (tenant_id, tao_luc, tao_boi, cap_nhat_luc, cap_nhat_boi)
		VALUES ($1, $2, $3, $2, $3) ON CONFLICT (tenant_id) DO NOTHING`
	res, err := tx.Exec(ctx, stmt, string(tx.TenantID()), at, by)
	if err != nil {
		return false, fmt.Errorf("ho_so_hien_thi_xa: tạo dòng hồ sơ: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("ho_so_hien_thi_xa: tạo dòng hồ sơ, đếm dòng: %w", err)
	}
	return n == 1, nil
}

// One statement per image, chosen from the closed pair — never a column name assembled from input.
const (
	setLogoFileStmt = `UPDATE ho_so_hien_thi_xa SET logo_file_id = $2, cap_nhat_luc = $3, cap_nhat_boi = $4
		WHERE tenant_id = $1 AND deleted_at IS NULL`
	setBannerFileStmt = `UPDATE ho_so_hien_thi_xa SET web_admin_banner_file_id = $2, cap_nhat_luc = $3,
		cap_nhat_boi = $4 WHERE tenant_id = $1 AND deleted_at IS NULL`
)

// SetBrandingFile points one image column at fileID ("" = NULL, the image removed) and stamps who and
// when. Zero rows (no live row) → domain.ErrProfileDeleted: the caller locked the row first, so the only
// way to reach zero is a row that is retired.
func (s *HoSoHienThiStore) SetBrandingFile(ctx context.Context, tx *corestore.ScopedTx,
	img domain.BrandingImage, fileID, by string, at time.Time) error {

	var stmt string
	switch img {
	case domain.BrandingLogo:
		stmt = setLogoFileStmt
	case domain.BrandingBanner:
		stmt = setBannerFileStmt
	default:
		return ErrBrandingImageUnknown
	}
	var bind any
	if fileID != "" {
		bind = fileID
	}
	res, err := tx.Exec(ctx, stmt, string(tx.TenantID()), bind, at, by)
	if err != nil {
		return fmt.Errorf("ho_so_hien_thi_xa: ghi ảnh nhận diện: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("ho_so_hien_thi_xa: ghi ảnh nhận diện, đếm dòng: %w", err)
	}
	if n == 0 {
		return domain.ErrProfileDeleted
	}
	return nil
}

// brandingViewStmt is the staff settings read: the two public keys under the same rule as Doc
// (ho_so_hien_thi.go) plus who last changed the row. Read by position.
const brandingViewStmt = `SELECT COALESCE(l.public_object_key, ''), COALESCE(b.public_object_key, ''),
	h.cap_nhat_luc, h.cap_nhat_boi
	FROM ho_so_hien_thi_xa h
	LEFT JOIN stored_file l ON l.tenant_id = $1 AND l.id = h.logo_file_id
		AND l.purpose = 'tenant-logo' AND l.status = 'ready' AND l.deleted_at IS NULL
	LEFT JOIN stored_file b ON b.tenant_id = $1 AND b.id = h.web_admin_banner_file_id
		AND b.purpose = 'tenant-banner' AND b.status = 'ready' AND b.deleted_at IS NULL
	WHERE h.tenant_id = $1 AND h.deleted_at IS NULL`

// BrandingView reads the commune's current identity images. A commune with no live profile row answers
// the zero view (nothing set), not an error: the settings tab then shows the empty state.
func (s *HoSoHienThiStore) BrandingView(ctx context.Context) (domain.BrandingView, error) {
	rows, err := s.db.For(ctx).QueryJoin(ctx, brandingViewStmt)
	if err != nil {
		return domain.BrandingView{}, fmt.Errorf("ho_so_hien_thi_xa: đọc nhận diện: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.BrandingView{}, fmt.Errorf("ho_so_hien_thi_xa: đọc nhận diện: %w", err)
		}
		return domain.BrandingView{}, nil
	}
	var v domain.BrandingView
	if err := rows.Scan(&v.LogoPublicKey, &v.BannerPublicKey, &v.UpdatedAt, &v.UpdatedBy); err != nil {
		return domain.BrandingView{}, fmt.Errorf("ho_so_hien_thi_xa: quét nhận diện: %w", err)
	}
	return v, nil
}
