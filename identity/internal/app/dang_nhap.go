// Package app holds the use cases of the identity service.
package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/password"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/token"
	"github.com/vihat/vigov/identity/internal/domain"
	idstore "github.com/vihat/vigov/identity/internal/store"
)

// KyToken signs the session token. An interface, declared at the point of use (see doc.go).
//
// WHY THE USE CASE SIGNS AND NOT THE HANDLER: signing has to happen INSIDE the transaction that
// writes the session and its audit entry. See Chay.
type KyToken interface {
	Ky(c token.Claims) (string, error)
}

// DangNhap signs a staff member in.
type DangNhap struct {
	db    *store.DB
	canBo *idstore.CanBoStore
	phien *idstore.PhienStore
	ky    KyToken
	log   *slog.Logger
}

func NewDangNhap(db *store.DB, canBo *idstore.CanBoStore, phien *idstore.PhienStore,
	ky KyToken, log *slog.Logger) *DangNhap {
	return &DangNhap{db: db, canBo: canBo, phien: phien, ky: ky, log: log}
}

// YeuCauDangNhap is what the handler passes in. IP and device are recorded on the session.
type YeuCauDangNhap struct {
	Email   string
	MatKhau string
	IP      string
	ThietBi string
}

// KetQuaDangNhap carries the signed session token. The refresh token is returned once and never
// again.
type KetQuaDangNhap struct {
	Sid string

	// Token is the signed cookie value. It is produced INSIDE the transaction below, so a
	// signing failure takes the session and the audit entry down with it.
	Token string

	Refresh   string
	HetHanLuc time.Time
	CanBo     domain.CanBo
}

// ErrDangNhapThatBai is returned for a wrong email AND for a wrong password.
//
// ONE ERROR FOR BOTH ON PURPOSE: distinguishing them tells an attacker which email addresses
// exist on this commune's domain, which is a staff directory they should not have.
//
// AND ONE LOG LINE FOR BOTH, for the same reason. The HTTP layer goes to some trouble to answer
// identically; writing two different sentences here hands the distinction straight back, in the
// log pipeline, which is readable by everybody with access to centralised logging — across every
// commune at once. Counting two kinds of line is all it takes to rebuild the directory.
var ErrDangNhapThatBai = errors.New("dang_nhap: email hoặc mật khẩu không đúng")

// vanTay is a short, one-way fingerprint, so repeated attempts can be correlated in the log
// without the log holding the value itself.
//
// WHY THE EMAIL IS NEVER LOGGED RAW, even though a staff work address is not citizen personal
// data: on this route it is an UNAUTHENTICATED, CLIENT-SUPPLIED STRING. Anyone can write
// anything into it at Info level, without limit — including a citizen who mistypes their own
// address into the staff screen, which puts a citizen's email (Decree 13/2023, Article 9) in
// centralised logging, backups and a monitoring vendor, from where it cannot be recalled.
//
// Lower-cased and trimmed first, so the same address typed two ways correlates. It is the
// correlation that has to work; the value itself must not come back.
//
// NOTE: identity/internal/http/middleware.go has the same helper, unexported, for the
// sid. Sharing one would mean a new home in pkg/ — a decision wider than this change.
func vanTay(s string) string {
	tong := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(s))))
	return hex.EncodeToString(tong[:])[:12]
}

// Chay signs the staff member in and opens a session.
func (uc *DangNhap) Chay(ctx context.Context, yc YeuCauDangNhap) (KetQuaDangNhap, error) {
	// The commune is read once, here, and used on every line below. One process serves 200+
	// communes into one log stream: a line without it cannot be acted on (rule 6, invariant 2).
	xa := tenant.MustFrom(ctx)

	cb, err := uc.canBo.TheoEmail(ctx, yc.Email)
	if err != nil {
		if errors.Is(err, idstore.ErrCanBoKhongTonTai) {
			uc.thatBai(xa, yc)
			return KetQuaDangNhap{}, ErrDangNhapThatBai
		}
		return KetQuaDangNhap{}, fmt.Errorf("dang_nhap: tra cứu cán bộ: %w", err)
	}

	if err := password.KiemTra(yc.MatKhau, cb.MatKhauHash); err != nil {
		uc.thatBai(xa, yc)
		return KetQuaDangNhap{}, ErrDangNhapThatBai
	}

	var kq KetQuaDangNhap
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		sid, refresh, err := uc.phien.Tao(ctx, tx, cb.ID, yc.IP, yc.ThietBi)
		if err != nil {
			return err
		}

		hetHan := time.Now().UTC().Add(idstore.ThoiHanPhien)

		// THE TOKEN IS SIGNED INSIDE THE TRANSACTION, and that placement is the whole point.
		//
		// Signed afterwards — which is where it used to live, in the HTTP handler — a signing
		// failure arrives with the transaction ALREADY COMMITTED. The archive of a public
		// authority then states "staff member X signed in at T" while no cookie was ever issued
		// and that person never signed in at all. An audit entry is append-only (rule 6,
		// invariant 4), so the false statement cannot be corrected afterwards; the only moment
		// it can be prevented is before the commit.
		//
		// token.Ky is a pure function — no database, no network — so it costs this transaction
		// nothing but the microseconds of an HMAC.
		tok, err := uc.ky.Ky(token.Claims{TenantID: xa, Sid: sid, ExpiresAt: hetHan})
		if err != nil {
			return fmt.Errorf("ký token phiên: %w", err)
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
			Token:     tok,
			Refresh:   refresh,
			HetHanLuc: hetHan,
			CanBo:     cb,
		}
		return nil
	})
	if err != nil {
		// Nothing was committed: no session, no trail, no token. The three states agree.
		return KetQuaDangNhap{}, fmt.Errorf("dang_nhap: mở phiên: %w", err)
	}

	// Raising the argon2 cost later must not lock anyone out: a successful sign-in is the only
	// moment the plaintext is available to rehash with.
	if password.CanBamLai(cb.MatKhauHash) {
		uc.namLaiMatKhau(ctx, xa, cb.ID, yc.MatKhau)
	}

	return kq, nil
}

// thatBai writes THE ONE LINE a failed sign-in produces, whatever the reason.
//
// No account code, no email, no hint of which of the two cases it was: whoever reads centralised
// logging must not be able to tell "this address does not exist here" from "this password is
// wrong". The fingerprint still lets an administrator correlate repeated attempts, and the IP
// still says where they came from.
func (uc *DangNhap) thatBai(xa tenant.ID, yc YeuCauDangNhap) {
	uc.log.Info("đăng nhập thất bại",
		"email_van_tay", vanTay(yc.Email), "ip", yc.IP, "xa", string(xa))
}

// namLaiMatKhau upgrades a hash made with weaker parameters.
//
// A failure here is logged and swallowed: the sign-in already succeeded, and refusing it
// because a background upgrade failed would lock out the very accounts being protected.
func (uc *DangNhap) namLaiMatKhau(ctx context.Context, xa tenant.ID, canBoID, matKhau string) {
	bam, err := password.Bam(matKhau)
	if err != nil {
		uc.log.Warn("không băm lại được mật khẩu",
			"can_bo_id", canBoID, "xa", string(xa), "err", err)
		return
	}
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		return uc.canBo.DatMatKhauHash(ctx, tx, canBoID, bam)
	})
	if err != nil {
		uc.log.Warn("không lưu được mật khẩu băm lại",
			"can_bo_id", canBoID, "xa", string(xa), "err", err)
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
