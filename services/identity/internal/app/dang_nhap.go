// Package app holds the use cases of the identity service.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/vihat/vigov/pkg/audit"
	"github.com/vihat/vigov/pkg/password"
	"github.com/vihat/vigov/pkg/store"
	"github.com/vihat/vigov/services/identity/internal/domain"
	idstore "github.com/vihat/vigov/services/identity/internal/store"
)

// DangNhap signs a staff member in.
type DangNhap struct {
	db    *store.DB
	canBo *idstore.CanBoStore
	phien *idstore.PhienStore
	log   *slog.Logger
}

func NewDangNhap(db *store.DB, canBo *idstore.CanBoStore, phien *idstore.PhienStore, log *slog.Logger) *DangNhap {
	return &DangNhap{db: db, canBo: canBo, phien: phien, log: log}
}

// YeuCauDangNhap is what the handler passes in. IP and device are recorded on the session.
type YeuCauDangNhap struct {
	Email   string
	MatKhau string
	IP      string
	ThietBi string
}

// KetQuaDangNhap carries the two tokens. The refresh token is returned once and never again.
type KetQuaDangNhap struct {
	Sid       string
	Refresh   string
	HetHanLuc time.Time
	CanBo     domain.CanBo
}

// ErrDangNhapThatBai is returned for a wrong email AND for a wrong password.
//
// ONE ERROR FOR BOTH ON PURPOSE: distinguishing them tells an attacker which email addresses
// exist on this commune's domain, which is a staff directory they should not have. The log
// below records which case it actually was, because an administrator investigating a locked-out
// colleague does need to know.
var ErrDangNhapThatBai = errors.New("dang_nhap: email hoặc mật khẩu không đúng")

// Chay signs the staff member in and opens a session.
func (uc *DangNhap) Chay(ctx context.Context, yc YeuCauDangNhap) (KetQuaDangNhap, error) {
	cb, err := uc.canBo.TheoEmail(ctx, yc.Email)
	if err != nil {
		if errors.Is(err, idstore.ErrCanBoKhongTonTai) {
			// Logged with the email because this is a staff work address, not citizen personal
			// data, and an administrator investigating a sign-in problem needs it. Never the
			// password, and never the reason in the response.
			uc.log.Info("đăng nhập thất bại: không có tài khoản", "email", yc.Email, "ip", yc.IP)
			return KetQuaDangNhap{}, ErrDangNhapThatBai
		}
		return KetQuaDangNhap{}, fmt.Errorf("dang_nhap: tra cứu cán bộ: %w", err)
	}

	if err := password.KiemTra(yc.MatKhau, cb.MatKhauHash); err != nil {
		uc.log.Info("đăng nhập thất bại: sai mật khẩu", "can_bo", cb.Ma, "ip", yc.IP)
		return KetQuaDangNhap{}, ErrDangNhapThatBai
	}

	var kq KetQuaDangNhap
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		sid, refresh, err := uc.phien.Tao(ctx, tx, cb.ID, yc.IP, yc.ThietBi)
		if err != nil {
			return err
		}

		if err := uc.canBo.GhiNhanDangNhap(ctx, tx, cb.ID); err != nil {
			return err
		}

		// The audit entry shares this transaction with the session insert (rule 6, invariant 3).
		// A session that exists without a trail is a sign-in nobody can account for.
		if err := audit.Write(ctx, tx, audit.Entry{
			Actor:   audit.Actor{ID: cb.Ma, Kind: "staff", IP: yc.IP},
			Action:  "dang_nhap",
			Subject: cb.Ma, // business code, never the internal id
		}); err != nil {
			return err
		}

		kq = KetQuaDangNhap{
			Sid:       sid,
			Refresh:   refresh,
			HetHanLuc: time.Now().UTC().Add(idstore.ThoiHanPhien),
			CanBo:     cb,
		}
		return nil
	})
	if err != nil {
		return KetQuaDangNhap{}, fmt.Errorf("dang_nhap: mở phiên: %w", err)
	}

	// Raising the argon2 cost later must not lock anyone out: a successful sign-in is the only
	// moment the plaintext is available to rehash with.
	if password.CanBamLai(cb.MatKhauHash) {
		uc.namLaiMatKhau(ctx, cb.ID, yc.MatKhau)
	}

	return kq, nil
}

// namLaiMatKhau upgrades a hash made with weaker parameters.
//
// A failure here is logged and swallowed: the sign-in already succeeded, and refusing it
// because a background upgrade failed would lock out the very accounts being protected.
func (uc *DangNhap) namLaiMatKhau(ctx context.Context, canBoID, matKhau string) {
	bam, err := password.Bam(matKhau)
	if err != nil {
		uc.log.Warn("không băm lại được mật khẩu", "can_bo_id", canBoID, "err", err)
		return
	}
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		return uc.canBo.DatMatKhauHash(ctx, tx, canBoID, bam)
	})
	if err != nil {
		uc.log.Warn("không lưu được mật khẩu băm lại", "can_bo_id", canBoID, "err", err)
	}
}

// DangXuat ends one session.
type DangXuat struct {
	db    *store.DB
	phien *idstore.PhienStore
}

func NewDangXuat(db *store.DB, phien *idstore.PhienStore) *DangXuat {
	return &DangXuat{db: db, phien: phien}
}

func (uc *DangXuat) Chay(ctx context.Context, sid, maCanBo, ip string) error {
	return uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		if err := uc.phien.ThuHoi(ctx, tx, sid, "đăng xuất"); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   audit.Actor{ID: maCanBo, Kind: "staff", IP: ip},
			Action:  "dang_xuat",
			Subject: maCanBo,
		})
	})
}
