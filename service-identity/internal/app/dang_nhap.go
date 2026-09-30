// Package app holds the use cases of the identity service.
package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
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

	// gieo is the default-administrator seed (gieo_quan_tri.go). nil = off, which is the state
	// NewDangNhap leaves it in; only BatGieoQuanTri switches it on.
	gieo *gieoQuanTri

	// now is the clock the automatic lockout (#39) is decided on. time.Now in production; a test pins
	// it to walk twelve hours without waiting for them.
	now func() time.Time
}

func NewDangNhap(db *store.DB, canBo *idstore.CanBoStore, phien *idstore.PhienStore,
	ky KyToken, log *slog.Logger) *DangNhap {
	return &DangNhap{db: db, canBo: canBo, phien: phien, ky: ky, log: log, now: time.Now}
}

func (uc *DangNhap) clock() time.Time { return uc.now().UTC() }

// ActionSignInAutoLocked is the audit verb written when the fifth consecutive failed sign-in locks a
// staff account (open question #39). A DIFFERENT verb from `khoa_tai_khoan_can_bo`, the manual lock
// of #10: that one says an administrator retired somebody, this one says the system stopped a run of
// guesses. An inspection reading the trail must never have to open the delta to tell them apart.
const ActionSignInAutoLocked = "khoa_tu_dong_do_dang_nhap_sai"

// signInAutoLockReason travels in the delta of that entry. The actor is the SYSTEM (rule 6, invariant
// 6): the person guessing did not choose to lock anything, and naming the account's owner as the
// actor would record them locking themselves.
const signInAutoLockReason = "tự động: đăng nhập sai liên tiếp đạt ngưỡng"

// countFailure records one failed sign-in on a live, unlocked account and, when it is the one that
// locks, writes the lock's audit entry IN THE SAME TRANSACTION.
//
// THE LOCK DOES NOT REVOKE THE ACCOUNT'S OPEN SESSIONS, and that is a choice with a reason: anybody
// who knows a staff member's work address can cause this lock from the sign-in screen. Revoking would
// hand them a button that signs the chairman out at will. The lock stops NEW sign-ins, which is what
// counting guesses is for — the same answer the owner gave for the operator realm (operator_auth.go,
// registerFailure, decision 28/09/2026).
func (uc *DangNhap) countFailure(ctx context.Context, tx *store.ScopedTx, xa tenant.ID, cb domain.CanBo,
	lock domain.SignInLock, ip string, now time.Time) error {

	next, lockedNow := lock.AfterFailure(now)
	if err := uc.canBo.SetSignInLock(ctx, tx, cb.ID, next); err != nil {
		return err
	}
	if !lockedNow {
		return nil
	}
	until := next.LockedUntil.UTC().Format(time.RFC3339)
	delta, err := json.Marshal(map[string]any{
		"truoc": signInLockDelta(lock),
		"sau":   signInLockDelta(next),
		"ly_do": signInAutoLockReason,
	})
	if err != nil {
		return fmt.Errorf("dang_nhap: delta khoá tự động: %w", err)
	}
	if err := audit.Write(ctx, tx, audit.Entry{
		Actor:   audit.Actor{ID: audit.SystemActor, Kind: "system", IP: ip},
		Action:  ActionSignInAutoLocked,
		Subject: cb.Ma,
		Delta:   delta,
	}); err != nil {
		return err
	}
	// The security log. The BUSINESS CODE, not the email fingerprint: the account is known here and
	// the trail already names it; an alert on this line is what tells somebody an account is under
	// attack. Written before the commit — a commit failure then leaves a line for a lock that did not
	// land, which says "somebody tried five times" and is still true.
	uc.log.Warn("tài khoản cán bộ bị khoá tự động do đăng nhập sai",
		"event", "staff.sign_in_locked", "outcome", "locked",
		"actor", audit.SystemActor, "subject", cb.Ma, "xa", string(xa), "ip", ip,
		"locked_until", until)
	return nil
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

	// The default-administrator seed (gieo_quan_tri.go) — reached ONLY when no live account
	// matched AND the typed pair is `admin` + the configured value. Every other request skips
	// this block without a statement or a hash, so it answers exactly as it did before the seed
	// existed. After a seed — or after finding that one is not due — the account is simply read
	// again and the ordinary path below decides, with its own password check, session and trail.
	if errors.Is(err, idstore.ErrCanBoKhongTonTai) && uc.coTheGieo(yc) {
		if gErr := uc.gieoQuanTriMacDinh(ctx, yc.IP); gErr != nil && !errors.Is(gErr, errKhongGieo) {
			// Reached only by whoever typed the configured value, so this line tells a log reader
			// nothing about which addresses exist. The error names a statement, never a value.
			uc.log.Warn("gieo quản trị mặc định thất bại", "xa", string(xa), "err", gErr)
			uc.thatBai(xa, yc)
			return KetQuaDangNhap{}, ErrDangNhapThatBai
		}
		cb, err = uc.canBo.TheoEmail(ctx, yc.Email)
	}

	if err != nil {
		if errors.Is(err, idstore.ErrCanBoKhongTonTai) {
			uc.thatBai(xa, yc)
			return KetQuaDangNhap{}, ErrDangNhapThatBai
		}
		return KetQuaDangNhap{}, fmt.Errorf("dang_nhap: tra cứu cán bộ: %w", err)
	}

	// THE PASSWORD IS VERIFIED BEFORE THE LOCK IS LOOKED AT, AND ALWAYS. A locked account that skipped
	// argon2 would answer in a microsecond instead of tens of milliseconds, and the response time would
	// then say "locked" — i.e. "this address has an account and somebody has been guessing it". The
	// answer to a locked account is the same error and the same log line as a wrong password, whether
	// or not the password typed was right (open question #39; TestLockedAccountRefusedEvenWithRightPassword).
	passwordOK := password.KiemTra(yc.MatKhau, cb.MatKhauHash) == nil
	now := uc.clock()

	var (
		kq      KetQuaDangNhap
		refused bool
	)
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		kq, refused = KetQuaDangNhap{}, true

		// The automatic lockout (#39), read and HELD for this transaction, so parallel attempts on one
		// account queue here and each sees the count the previous one committed.
		//
		// A REFUSAL RETURNS nil, NOT AN ERROR, AND THAT IS THE POINT: the failure count must COMMIT
		// although the sign-in fails. Returning the refusal as the closure's error would roll the
		// counter back and make the lockout a decoration (the operator realm's shape, operator_auth.go).
		lock, err := uc.canBo.SignInLockForUpdate(ctx, tx, cb.ID)
		if err != nil {
			return err
		}
		if lock.LockedAt(now) {
			// Refused and NOT counted: a lock must not be extended by the guesses it stops, or whoever
			// guesses keeps the real owner out for as long as they like. Nothing is written.
			return nil
		}
		if !passwordOK {
			return uc.countFailure(ctx, tx, xa, cb, lock, yc.IP, now)
		}
		// A success restarts the count (#39: "LIÊN TIẾP" — five in a row, not five in total). Written
		// only when there is something to clear, so an ordinary sign-in costs no extra statement.
		if !lock.IsClear() {
			if err := uc.canBo.SetSignInLock(ctx, tx, cb.ID, lock.Clear()); err != nil {
				return err
			}
		}
		refused = false

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
	if refused {
		// Wrong password, or an account under the automatic lock: ONE error, ONE log line, whichever.
		uc.thatBai(xa, yc)
		return KetQuaDangNhap{}, ErrDangNhapThatBai
	}

	// Raising the argon2 cost later must not lock anyone out: a successful sign-in is the only
	// moment the plaintext is available to rehash with.
	if password.CanBamLai(cb.MatKhauHash) {
		uc.namLaiMatKhau(ctx, xa, cb.ID, yc.MatKhau)
	}

	return kq, nil
}

// signInLockDelta is one side of the before/after of a lock or an unlock. Two numbers — no personal
// data (rule 6, invariant 5; rule 3).
func signInLockDelta(l domain.SignInLock) map[string]any {
	var until any
	if l.LockedUntil != nil {
		until = l.LockedUntil.UTC().Format(time.RFC3339)
	}
	return map[string]any{"failed_sign_in_count": l.FailedCount, "sign_in_locked_until": until}
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
