package crosstenant

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
)

// TaiKhoanZaloStore is the Zalo account behind a Mini App session (migration 0011, ADR 0045
// §Phiên chưa có số). SHAPE 1 of doc.go: a table with NO tenant_id, because a decision put it above
// the commune — it is what answers "which commune" for the main app.
//
// IT HOLDS A RAW *sql.DB FOR EXACTLY ONE READ, and the reason cannot be designed away: the bridge
// must know the account's remembered commune BEFORE it can choose the commune whose scoped
// transaction it then opens (core/store.For(ctx) needs that commune). Every WRITE takes the caller's
// transaction, so the audit entry shares it (rule 6, invariant 3). The handle never leaves.
//
// THE ZALO USER ID NEVER REACHES THE DATABASE RAW. Every method hashes it first; no struct here can
// carry it back out. It is personal data (an online identifier, Decree 13/2023), and the hash is
// enough because nothing ever needs to show it.
type TaiKhoanZaloStore struct {
	raw *sql.DB
}

func NewTaiKhoanZaloStore(db *sql.DB) *TaiKhoanZaloStore {
	if db == nil {
		panic("crosstenant: NewTaiKhoanZaloStore cần kết nối — cầu phiên đọc xã đã nhớ trước giao dịch")
	}
	return &TaiKhoanZaloStore{raw: db}
}

// TaiKhoanZalo is one account. NO Zalo id and no phone number: the struct cannot leak either.
type TaiKhoanZalo struct {
	// ID is the opaque ULID — the audit "who" before a phone is verified.
	ID string

	// CongDanID is the verified citizen identity this account points at, "" when none yet.
	CongDanID string

	// XaDaNho is the commune the citizen last confirmed in the main app, "" when none. It is READ
	// here and compared against the registry by the caller — never copied into a session unchecked.
	XaDaNho tenant.ID
}

var (
	ErrThieuAppID        = errors.New("tài khoản Zalo: thiếu app_id")
	ErrThieuMaZalo       = errors.New("tài khoản Zalo: thiếu mã tài khoản Zalo")
	ErrKhoaZaloQuaDai    = errors.New("tài khoản Zalo: app_id hoặc mã Zalo dài bất thường")
	ErrThieuTaiKhoan     = errors.New("tài khoản Zalo: thiếu định danh tài khoản")
	ErrThieuDinhDanh     = errors.New("tài khoản Zalo: thiếu định danh công dân để liên kết")
	ErrKhongCoTaiKhoan   = errors.New("tài khoản Zalo: không có tài khoản để cập nhật")
	ErrThieuGiaoDichZalo = errors.New("tài khoản Zalo: cần giao dịch của lời gọi để ghi vết đi cùng")
)

// doDaiKhoaToiDa bounds two strings that arrive from outside ViGov (the bridge caller). A Zalo Mini
// App id and a Zalo user id are both short digit strings; the margin is for whatever the platform
// changes next, not for a payload.
const doDaiKhoaToiDa = 128

// khoaZalo checks both halves of the key and returns the hash of the Zalo id. Checked BEFORE any
// database is touched, so these refusals are testable with none. Neither value goes into an error.
func khoaZalo(appID, maZalo string) (string, string, error) {
	appID = strings.TrimSpace(appID)
	maZalo = strings.TrimSpace(maZalo)
	switch {
	case appID == "":
		return "", "", ErrThieuAppID
	case maZalo == "":
		return "", "", ErrThieuMaZalo
	case len(appID) > doDaiKhoaToiDa || len(maZalo) > doDaiKhoaToiDa:
		return "", "", ErrKhoaZaloQuaDai
	}
	return appID, bamMaZalo(maZalo), nil
}

// bamMaZalo is SHA-256 hex — the shape CHECK tai_khoan_zalo_bam_la_sha256 enforces. Unsalted on
// purpose, because the lookup must be deterministic; migration 0011 states what that does not buy.
func bamMaZalo(maZalo string) string {
	tong := sha256.Sum256([]byte(maZalo))
	return hex.EncodeToString(tong[:])
}

const cotTaiKhoanZalo = `id, cong_dan_id, xa_da_nho`

func quetTaiKhoan(sc interface{ Scan(...any) error }) (TaiKhoanZalo, error) {
	var tk TaiKhoanZalo
	var congDan, xa sql.NullString
	if err := sc.Scan(&tk.ID, &congDan, &xa); err != nil {
		return TaiKhoanZalo{}, err
	}
	tk.CongDanID = congDan.String
	tk.XaDaNho = tenant.ID(xa.String)
	return tk, nil
}

// Doc reads one account WITHOUT a transaction — the pre-transaction read described on the type.
// ok=false: the account has never been seen. It writes nothing: an account seen for the first time
// on an open that ends with NO commune is not recorded at all, because there is no commune's trail
// to record it in (open question #25's no-commune table is not built).
func (s *TaiKhoanZaloStore) Doc(ctx context.Context, appID, maZalo string) (TaiKhoanZalo, bool, error) {
	app, bam, err := khoaZalo(appID, maZalo)
	if err != nil {
		return TaiKhoanZalo{}, false, err
	}
	// @cross-tenant: tai_khoan_zalo không có tenant_id (migration 0011, ADR 0045) — nó là thứ trả lời
	// "xã nào" cho app chính. Tìm đúng MỘT dòng theo khoá (app_id, băm mã Zalo) mà bên cầu đã xác minh
	// với Zalo; không phải thứ người gọi liệt kê được.
	tk, err := quetTaiKhoan(s.raw.QueryRowContext(ctx,
		`SELECT `+cotTaiKhoanZalo+` FROM tai_khoan_zalo WHERE app_id = $1 AND bam_zalo_user_id = $2`, app, bam))
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return TaiKhoanZalo{}, false, nil
	case err != nil:
		return TaiKhoanZalo{}, false, fmt.Errorf("tài khoản Zalo: đọc: %w", err)
	}
	return tk, true, nil
}

// TimHoacTao returns the account, creating it the first time, and LOCKS ITS ROW for the rest of the
// caller's transaction. vuaTao reports whether this call created it.
//
// THE LOCK IS THE POINT. Two opens of one account at once — a double tap, a retry after a timeout —
// both read the remembered commune, both decide, both write. FOR UPDATE (and ON CONFLICT DO UPDATE,
// which locks the same way) serialises them, so the second sees the first one's commune as "before"
// and the audit entry says what really changed.
//
// The race on first creation and what it costs are the ones DinhDanhStore.TimHoacTao states: the
// loser's INSERT waits, ON CONFLICT returns the winner's row, and both report vuaTao=true.
func (s *TaiKhoanZaloStore) TimHoacTao(ctx context.Context, tx *sql.Tx, appID, maZalo string) (TaiKhoanZalo, bool, error) {
	app, bam, err := khoaZalo(appID, maZalo)
	if err != nil {
		return TaiKhoanZalo{}, false, err
	}
	if tx == nil {
		return TaiKhoanZalo{}, false, ErrThieuGiaoDichZalo
	}

	// @cross-tenant: cùng lý do với Doc — bảng tài khoản Zalo không thuộc xã nào. Một dòng, theo khoá
	// đã xác minh, khoá dòng tới hết giao dịch của lời gọi.
	tk, err := quetTaiKhoan(tx.QueryRowContext(ctx,
		`SELECT `+cotTaiKhoanZalo+` FROM tai_khoan_zalo WHERE app_id = $1 AND bam_zalo_user_id = $2 FOR UPDATE`,
		app, bam))
	switch {
	case err == nil:
		return tk, false, nil
	case !errors.Is(err, sql.ErrNoRows):
		return TaiKhoanZalo{}, false, fmt.Errorf("tài khoản Zalo: tìm: %w", err)
	}

	moi, err := maULID()
	if err != nil {
		return TaiKhoanZalo{}, false, err
	}
	// @cross-tenant: ghi một dòng tài khoản Zalo — bảng không có tenant_id (migration 0011). No-op
	// DO UPDATE, not DO NOTHING, for the reason DinhDanhStore.TimHoacTao gives.
	tk, err = quetTaiKhoan(tx.QueryRowContext(ctx,
		`INSERT INTO tai_khoan_zalo (id, app_id, bam_zalo_user_id) VALUES ($1, $2, $3)
		 ON CONFLICT (app_id, bam_zalo_user_id)
		 DO UPDATE SET cap_nhat_luc = tai_khoan_zalo.cap_nhat_luc
		 RETURNING `+cotTaiKhoanZalo, moi, app, bam))
	if err != nil {
		return TaiKhoanZalo{}, false, fmt.Errorf("tài khoản Zalo: tạo: %w", err)
	}
	return tk, true, nil
}

// TroToiDinhDanh points the account at a verified citizen identity — the first time a phone is
// verified through it, or when a later phone token names a different number. It does NOT merge
// identities and does NOT move records (ADR 0020 CÒN MỞ #1). The caller audits the move.
func (s *TaiKhoanZaloStore) TroToiDinhDanh(ctx context.Context, tx *sql.Tx, taiKhoanID, congDanID string) error {
	switch {
	case strings.TrimSpace(taiKhoanID) == "":
		return ErrThieuTaiKhoan
	case strings.TrimSpace(congDanID) == "":
		return ErrThieuDinhDanh
	case tx == nil:
		return ErrThieuGiaoDichZalo
	}
	// @cross-tenant: cập nhật đúng MỘT tài khoản Zalo theo khoá chính — bảng không có tenant_id.
	kq, err := tx.ExecContext(ctx,
		`UPDATE tai_khoan_zalo SET cong_dan_id = $2, lien_ket_luc = now(), cap_nhat_luc = now()
		 WHERE id = $1`, taiKhoanID, congDanID)
	if err != nil {
		return fmt.Errorf("tài khoản Zalo: liên kết định danh: %w", err)
	}
	return motDong(kq)
}

// NhoXaCuaGiaoDich records THE COMMUNE OF THE CALLER'S TRANSACTION as the account's remembered
// commune. The commune is not a parameter (rule 1, invariant 4): it is the commune the session is
// being opened in, taken from the scoped transaction that opens it, so the two cannot differ.
func (s *TaiKhoanZaloStore) NhoXaCuaGiaoDich(ctx context.Context, tx *store.ScopedTx, taiKhoanID string) error {
	if strings.TrimSpace(taiKhoanID) == "" {
		return ErrThieuTaiKhoan
	}
	if tx == nil {
		return ErrThieuGiaoDichZalo
	}
	// @cross-tenant: cập nhật đúng MỘT tài khoản Zalo theo khoá chính — bảng không có tenant_id; giá
	// trị ghi vào là xã của CHÍNH giao dịch này, không phải thứ người gọi đưa vào.
	kq, err := tx.Underlying().ExecContext(ctx,
		`UPDATE tai_khoan_zalo SET xa_da_nho = $2, xa_da_nho_luc = now(), cap_nhat_luc = now()
		 WHERE id = $1`, taiKhoanID, string(tx.TenantID()))
	if err != nil {
		return fmt.Errorf("tài khoản Zalo: nhớ xã: %w", err)
	}
	return motDong(kq)
}

func motDong(kq sql.Result) error {
	n, err := kq.RowsAffected()
	if err != nil {
		return fmt.Errorf("tài khoản Zalo: đếm dòng: %w", err)
	}
	if n != 1 {
		return ErrKhongCoTaiKhoan
	}
	return nil
}
