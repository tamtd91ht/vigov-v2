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

// cotCanBo IS READ BY POSITION IN motCanBo. `co_tai_khoan` and `dang_hoat_dong` are two
// adjacent BOOLEANs, so swapping them here — or there — produces NO ERROR ANYWHERE: the driver
// scans two bools into two bools and the two values quietly trade places. The result is a
// directory-only person reported as an account and a locked account reported as open. The two
// lists are kept in the same order and pinned by a test that asserts an asymmetric pair
// (co_tai_khoan = false, dang_hoat_dong = true) comes back the right way round — a symmetric
// pair cannot tell a swap from a correct read.
const cotCanBo = `id, ma, ho_ten, email, chuc_vu,
                  coalesce(bo_phan_id,''), coalesce(vai_tro_id,''),
                  dien_thoai, mat_khau_hash, co_tai_khoan, dang_hoat_dong`

// TheoEmail looks a staff member up for sign-in.
//
// THREE conditions, and each one is load-bearing. Only rows that are not soft-deleted (kept for
// the audit trail, rule 7, not for signing in), that HAVE an account at all (`co_tai_khoan` —
// the public staff directory shares this table with the accounts), and whose account is not
// locked (`dang_hoat_dong`) are returned.
//
// THE FILTER IS IN THE SQL, NOT IN GO, AND THAT IS A SECURITY PROPERTY, not a style choice:
//
//  1. A directory-only person becomes INDISTINGUISHABLE from an address that does not exist
//     here — same ErrCanBoKhongTonTai, same ErrDangNhapThatBai, same single log line. That is
//     already the behaviour this code deliberately chose for an unknown email.
//  2. Let the row through and it reaches password.KiemTra with an EMPTY hash, which fails to
//     parse and returns IMMEDIATELY, while a real account spends tens of milliseconds in
//     argon2. Timing the response then rebuilds the commune's staff directory — who has an
//     account and who does not — from an unauthenticated endpoint. Filtering in SQL erases the
//     difference instead of hiding it.
//  3. It is also what makes the partial index `nguoi_dung_dang_nhap` (migration 0003) usable:
//     its predicate is exactly these three conditions.
func (s *CanBoStore) TheoEmail(ctx context.Context, email string) (domain.CanBo, error) {
	rows, err := s.db.For(ctx).Query(ctx, cotCanBo, "nguoi_dung",
		`AND email = $2 AND deleted_at IS NULL AND co_tai_khoan AND dang_hoat_dong`, email)
	if err != nil {
		return domain.CanBo{}, fmt.Errorf("can_bo: tra theo email: %w", err)
	}
	defer rows.Close()
	return motCanBo(rows)
}

// TheoID looks a staff member up by internal id, used when rebuilding a request's principal
// from its session.
//
// SAME THREE CONDITIONS AS TheoEmail, and `co_tai_khoan` matters here for a reason of its own:
// this runs on EVERY request that carries a session, so it is where an account withdrawn while
// somebody was signed in actually takes effect. Without it, revoking the account leaves every
// open session working until it expires — the middleware would keep rebuilding a principal for
// a person who no longer has an account. With it, the next request has none.
func (s *CanBoStore) TheoID(ctx context.Context, id string) (domain.CanBo, error) {
	rows, err := s.db.For(ctx).Query(ctx, cotCanBo, "nguoi_dung",
		`AND id = $2 AND deleted_at IS NULL AND co_tai_khoan AND dang_hoat_dong`, id)
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
	// POSITIONAL — this list must stay in lockstep with cotCanBo. See the note there on the two
	// adjacent bools.
	err := rows.Scan(&cb.ID, &cb.Ma, &cb.HoTen, &cb.Email, &cb.ChucVu,
		&cb.BoPhanID, &cb.VaiTroID, &cb.DienThoai, &cb.MatKhauHash,
		&cb.CoTaiKhoan, &cb.DangHoatDong)
	if err != nil {
		return domain.CanBo{}, fmt.Errorf("can_bo: đọc dòng: %w", err)
	}
	return cb, nil
}
