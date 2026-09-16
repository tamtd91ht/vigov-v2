package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/pkg/store"
)

// PhienStore is the session registry.
//
// WHY SESSIONS ARE A TABLE AND NOT ONLY A SIGNED TOKEN: a merely signed token cannot be taken
// back. Locking an account, changing a role or changing a password must end every open session
// at once (skills/session-and-token, required #7); with a stateless token a former employee
// keeps working until it expires on its own.
//
// WHERE THE AUDIT ENTRY IS WRITTEN — read this before adding a method here.
//
// Every mutating method takes a *store.ScopedTx rather than opening its own transaction, and
// the CALLER writes the audit entry inside that same transaction. Rule 6, invariant 3: the
// entry and the change must succeed or fail together.
//
// This layer does not write the entry itself because it does not know the business fact. The
// same ThuHoiCuaCanBo call is "khoá tài khoản", "đổi vai trò" or "đổi mật khẩu" depending on
// why the use case invoked it, and "revoked 3 sessions" is a technical detail — the audit
// trail has to answer what a person DID (rule 6, invariant 2), not which rows moved.
//
// The two read-only methods, KiemTra and GhiNhanDung, write no business fact and are
// deliberately unaudited: auditing a session lookup would add one row per request and bury the
// entries that matter.
type PhienStore struct {
	db *store.DB
}

func NewPhienStore(db *store.DB) *PhienStore { return &PhienStore{db: db} }

// Phien is one open session.
type Phien struct {
	ID          string
	NguoiDungID string
	TaoLuc      time.Time
	HetHanLuc   time.Time
	DungGanNhat *time.Time
}

var (
	ErrPhienKhongTonTai = errors.New("phien: không tồn tại hoặc đã bị thu hồi")
	ErrPhienHetHan      = errors.New("phien: đã hết hạn")
)

// ThoiHanPhien is how long a session may live without being refreshed.
const ThoiHanPhien = 12 * time.Hour

// Tao opens a session and returns the sid plus the refresh token.
//
// The refresh token is returned ONCE, in plaintext, and only its SHA-256 is stored: a leaked
// backup must not hand over working sessions. SHA-256 is right here and argon2 is not — this
// is a 256-bit random value, not a human-chosen password, so there is nothing to brute-force
// and the check sits on the hot path.
func (s *PhienStore) Tao(ctx context.Context, tx *store.ScopedTx, nguoiDungID, ip, thietBi string) (sid, refresh string, err error) {
	sid, err = maNgauNhien()
	if err != nil {
		return "", "", err
	}
	refresh, err = maNgauNhien()
	if err != nil {
		return "", "", err
	}

	// User agent is truncated: it is a diagnostic, and an unbounded client-supplied string in a
	// government database is a liability, not a feature.
	if len(thietBi) > 200 {
		thietBi = thietBi[:200]
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO phien (tenant_id, id, nguoi_dung_id, refresh_hash, het_han_luc, ip_tao, thiet_bi)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		string(tx.TenantID()), sid, nguoiDungID, bamRefresh(refresh),
		time.Now().UTC().Add(ThoiHanPhien), ip, thietBi)
	if err != nil {
		return "", "", fmt.Errorf("phien: tạo: %w", err)
	}
	return sid, refresh, nil
}

// KiemTra looks up a sid and reports whether it is still usable.
//
// This runs on EVERY authenticated request (skills/session-and-token, required #3), which is
// the price of being able to revoke a token at all. The partial index phien_con_hieu_luc
// exists for exactly this query.
func (s *PhienStore) KiemTra(ctx context.Context, sid string) (Phien, error) {
	rows, err := s.db.For(ctx).Query(ctx,
		"id, nguoi_dung_id, tao_luc, het_han_luc, dung_gan_nhat", "phien",
		`AND id = $2 AND thu_hoi_luc IS NULL`, sid)
	if err != nil {
		return Phien{}, fmt.Errorf("phien: kiểm tra: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return Phien{}, fmt.Errorf("phien: đọc: %w", err)
		}
		return Phien{}, ErrPhienKhongTonTai
	}

	var p Phien
	if err := rows.Scan(&p.ID, &p.NguoiDungID, &p.TaoLuc, &p.HetHanLuc, &p.DungGanNhat); err != nil {
		return Phien{}, fmt.Errorf("phien: đọc dòng: %w", err)
	}
	if time.Now().UTC().After(p.HetHanLuc) {
		return Phien{}, ErrPhienHetHan
	}
	return p, nil
}

const dongThuHoi = `UPDATE phien SET thu_hoi_luc = now(), thu_hoi_ly_do = $3
                    WHERE tenant_id = $1 AND thu_hoi_luc IS NULL AND `

// ThuHoi ends one session. Used at sign-out.
func (s *PhienStore) ThuHoi(ctx context.Context, tx *store.ScopedTx, sid, lyDo string) error {
	if _, err := tx.Exec(ctx, dongThuHoi+`id = $2`,
		string(tx.TenantID()), sid, lyDo); err != nil {
		return fmt.Errorf("phien: thu hồi: %w", err)
	}
	return nil
}

// ThuHoiCuaCanBo ends EVERY open session of one staff member.
//
// Required #7: locking an account, changing a role and changing a password must all call this.
// Leaving a session open after a role change means the old permissions keep working until the
// token expires — which is the same as not having changed the role at all.
func (s *PhienStore) ThuHoiCuaCanBo(ctx context.Context, tx *store.ScopedTx, nguoiDungID, lyDo string) error {
	if _, err := tx.Exec(ctx, dongThuHoi+`nguoi_dung_id = $2`,
		string(tx.TenantID()), nguoiDungID, lyDo); err != nil {
		return fmt.Errorf("phien: thu hồi theo cán bộ: %w", err)
	}
	return nil
}

// GhiNhanDung records that a session was used, for the "last active" column.
//
// Deliberately NOT part of the request's transaction: this is a diagnostic, and failing a
// business write because a timestamp could not be updated would trade a real operation for a
// nicety.
func (s *PhienStore) GhiNhanDung(ctx context.Context, sid string) {
	_ = s.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		_, err := tx.Exec(ctx,
			`UPDATE phien SET dung_gan_nhat = now()
			 WHERE tenant_id = $1 AND id = $2 AND thu_hoi_luc IS NULL`,
			string(tx.TenantID()), sid)
		return err
	})
}

func bamRefresh(token string) string {
	tong := sha256.Sum256([]byte(token))
	return hex.EncodeToString(tong[:])
}

// maNgauNhien returns 256 bits of randomness, URL-safe.
//
// A session id must not be guessable: a guessable sid is a session anyone can take over, and
// with it every permission its owner holds.
func maNgauNhien() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("phien: sinh mã ngẫu nhiên: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
