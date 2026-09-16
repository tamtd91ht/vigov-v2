package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/vihat/vigov/pkg/store"
	"github.com/vihat/vigov/services/identity/internal/domain"
)

// CanBoStore reads and writes staff accounts.
//
// As with PhienStore, the mutating methods take a *store.ScopedTx and leave the audit entry to
// the caller: this layer knows which rows changed, the use case knows what a person did, and
// rule 6 asks for the second one.
type CanBoStore struct {
	db *store.DB
}

func NewCanBoStore(db *store.DB) *CanBoStore { return &CanBoStore{db: db} }

var ErrCanBoKhongTonTai = errors.New("can_bo: không tồn tại")

const cotCanBo = `id, ma, ho_ten, email, chuc_vu,
                  coalesce(bo_phan_id,''), coalesce(vai_tro_id,''),
                  dien_thoai, mat_khau_hash, dang_hoat_dong`

// TheoEmail looks a staff member up for sign-in.
//
// Only active, non-deleted accounts are returned: a locked account must not sign in, and a
// soft-deleted one is kept for the audit trail (rule 7), not for logging in.
func (s *CanBoStore) TheoEmail(ctx context.Context, email string) (domain.CanBo, error) {
	rows, err := s.db.For(ctx).Query(ctx, cotCanBo, "nguoi_dung",
		`AND email = $2 AND deleted_at IS NULL AND dang_hoat_dong`, email)
	if err != nil {
		return domain.CanBo{}, fmt.Errorf("can_bo: tra theo email: %w", err)
	}
	defer rows.Close()
	return motCanBo(rows)
}

// TheoID looks a staff member up by internal id, used when rebuilding a request's principal
// from its session.
func (s *CanBoStore) TheoID(ctx context.Context, id string) (domain.CanBo, error) {
	rows, err := s.db.For(ctx).Query(ctx, cotCanBo, "nguoi_dung",
		`AND id = $2 AND deleted_at IS NULL AND dang_hoat_dong`, id)
	if err != nil {
		return domain.CanBo{}, fmt.Errorf("can_bo: tra theo id: %w", err)
	}
	defer rows.Close()
	return motCanBo(rows)
}

// GhiNhanDangNhap stamps the last sign-in, shown on Cấu hình → Người dùng.
func (s *CanBoStore) GhiNhanDangNhap(ctx context.Context, tx *store.ScopedTx, id string) error {
	_, err := tx.Exec(ctx,
		`UPDATE nguoi_dung SET dang_nhap_gan_nhat = now(), cap_nhat_luc = now()
		 WHERE tenant_id = $1 AND id = $2`,
		string(tx.TenantID()), id)
	if err != nil {
		return fmt.Errorf("can_bo: ghi nhận đăng nhập: %w", err)
	}
	return nil
}

// DatMatKhauHash stores a new hash.
//
// CALLER MUST ALSO revoke every open session when a person changes their password
// (skills/session-and-token, required #7) — a password change that leaves old sessions working
// does not actually lock anybody out. The one exception is a silent rehash after a successful
// sign-in, where the password did not change.
func (s *CanBoStore) DatMatKhauHash(ctx context.Context, tx *store.ScopedTx, id, bam string) error {
	_, err := tx.Exec(ctx,
		`UPDATE nguoi_dung SET mat_khau_hash = $3, cap_nhat_luc = now()
		 WHERE tenant_id = $1 AND id = $2`,
		string(tx.TenantID()), id, bam)
	if err != nil {
		return fmt.Errorf("can_bo: đặt mật khẩu: %w", err)
	}
	return nil
}

type quetDuoc interface {
	Next() bool
	Scan(...any) error
	Err() error
}

func motCanBo(rows quetDuoc) (domain.CanBo, error) {
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.CanBo{}, fmt.Errorf("can_bo: đọc: %w", err)
		}
		return domain.CanBo{}, ErrCanBoKhongTonTai
	}
	var cb domain.CanBo
	err := rows.Scan(&cb.ID, &cb.Ma, &cb.HoTen, &cb.Email, &cb.ChucVu,
		&cb.BoPhanID, &cb.VaiTroID, &cb.DienThoai, &cb.MatKhauHash, &cb.DangHoatDong)
	if err != nil {
		return domain.CanBo{}, fmt.Errorf("can_bo: đọc dòng: %w", err)
	}
	return cb, nil
}
