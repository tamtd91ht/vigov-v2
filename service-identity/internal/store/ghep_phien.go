package store

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vihat/vigov/core/store"
)

// GhepPhienStore writes the pairing CODE — ghep_phien (@entity SessionPairing, migration 0004,
// ADR 0019, ADR 0023 §4).
//
// THE ENTITY IS THE CODE, NOT A SECOND KIND OF SESSION. What comes out of redeeming one is an
// ordinary row in phien_cong_dan with nguon = 'ghep' (PhienCongDanStore.Tao). Keeping that
// distinction is what stops "screen session" from becoming a parallel session model with its
// own revocation rules and its own gaps.
//
// # THREE PROPERTIES THE SCHEMA ENFORCES, AND NOTHING HERE WORKS AROUND
//
//  1. THE CODE IS STORED HASHED. SHA-256 hex of the code, never the code — the same shape the
//     staff registry uses for refresh tokens (bamRefresh) and the citizen registry for bearer
//     tokens. The code is a bearer secret for two minutes: whoever can read it can have a
//     shared screen issued a session in somebody else's name, so storing it raw would put that
//     capability into every database backup and every read-only reporting account. It needs no
//     salt and no work factor because the input is 256 bits of randomness, not a password.
//
//  2. SINGLE USE IS ENFORCED BY THE DATABASE. Dung updates under the condition that dung_luc
//     is still NULL, so two screens racing for one code serialise on the row lock and the
//     loser matches zero rows. It is not a check-then-write in Go, which is the shape that
//     loses that race.
//
//  3. THE STATE IS DERIVED FROM THREE TIMESTAMPS. There is no trang_thai column and none may
//     be added; domain.TrangThaiGhep is the one place the derivation is written. "Expired" in
//     particular is a comparison against a clock (rule 10, invariant 3).
//
// # EVERY METHOD TAKES THE CALLER'S TRANSACTION, AND NONE WRITES THE AUDIT ENTRY
//
// Creating, redeeming and cancelling a pairing are all business writes: ADR 0019, invariant 6
// requires each pairing to record who, which device, which IP, which commune and when. The
// entry is written by the USE CASE, in the transaction it hands in here (rule 6, invariant 3),
// because only the use case knows the business fact — this layer sees rows moving, not a
// person authorising a screen.
//
// THE COMMUNE IS NEVER A PARAMETER: it comes from the ScopedTx, which got it from the context.
// A code therefore cannot be created in one commune by a caller holding another, and the
// foreign key (tenant_id, phien_id) makes the database itself refuse a pairing whose session
// belongs to a different commune — ADR 0019, invariant 8 backed by a constraint and not only
// by a check in Go.
type GhepPhienStore struct {
	db *store.DB
}

func NewGhepPhienStore(db *store.DB) *GhepPhienStore { return &GhepPhienStore{db: db} }

// ThoiHanMaGhep is the life of a pairing code. ADR 0019, invariant 1 fixes it at 120 seconds
// and CONSTRAINT ghep_phien_ttl_toi_da_120s enforces it in the schema.
//
// IT IS A CONSTANT AND NOT A CALLER ARGUMENT — the opposite of PhienMoi.ThoiHan, and the
// difference is that this number IS decided. The TTL of the SESSION a pairing produces is
// still open (ADR 0019, §CÒN MỞ #1: nobody has decided what the shared screen is), so that one
// belongs to the caller; this one is written in the ADR, so letting a caller choose it would
// only create a way to get it wrong.
const ThoiHanMaGhep = 120 * time.Second

// chuCaiDoiChieu is Crockford's base32 alphabet — no I, L, O or U.
//
// THE FOUR CHARACTERS ARE COMPARED BY A HUMAN EYE, on two screens standing side by side at a
// one-stop counter (ADR 0019, invariant 4), which is the whole reason the confusable letters
// are out: a citizen who cannot tell O from 0 cannot perform the one check this step exists
// for. 32 characters divide 256 exactly, so a byte masked with 31 is uniform — no modulo bias,
// and no rejection loop.
const chuCaiDoiChieu = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

var (
	ErrThieuMaGhep       = errors.New("ghep_phien: thiếu định danh mã ghép")
	ErrThieuPhienRa      = errors.New("ghep_phien: dùng mã phải kèm công dân và phiên vừa phát")
	ErrThieuLyDoHuy      = errors.New("ghep_phien: huỷ phải có lý do")
	ErrThieuGiaoDichGhep = errors.New("ghep_phien: cần giao dịch của lời gọi để ghi vết đi cùng")

	// ErrMaGhepKhongDungDuoc means the redeem matched no row: the code was already used, or
	// cancelled, or has expired, or belongs to another commune, or never existed.
	//
	// THE FIVE CASES ARE NOT DISTINGUISHED HERE, ON PURPOSE. Telling them apart from a single
	// statement would mean reading the row first and then writing, which is exactly the
	// check-then-write that loses the race two screens can create. The caller that needs the
	// distinction — ADR 0019 step 5 wants a commune mismatch to raise 401 AND AN ALERT — gets
	// it from crosstenant.GhepPhienStore.TheoMa BEFORE calling this, and this remains the
	// authority on whether the code was actually spent.
	ErrMaGhepKhongDungDuoc = errors.New("ghep_phien: mã không còn dùng được")

	// ErrMaGhepKhongHuyDuoc means the cancel matched no row: already used, already cancelled,
	// or not this commune's code.
	//
	// IT IS AN ERROR AND NOT A SILENT NO-OP, and the case that decides it is "already used": a
	// screen that has ALREADY been issued a session is not un-paired by cancelling the code,
	// and a caller told "cancelled" would report to the citizen that a screen was logged out
	// when it was not. Ending that session is PhienCongDanStore.ThuHoi, a different act.
	ErrMaGhepKhongHuyDuoc = errors.New("ghep_phien: mã không huỷ được")
)

// MaGhepMoi is the diagnostic context of one pairing request. There is no commune field and no
// TTL field — see the type comment and ThoiHanMaGhep.
type MaGhepMoi struct {
	// IP and ThietBi describe the SCREEN that asked, never a person. ADR 0019, invariant 6
	// requires both in the trail. Never personal data (rule 3).
	IP      string
	ThietBi string
}

// MaGhepDaTao is what the screen is handed. The raw code appears exactly here and nowhere else
// in the system: it is never stored, never logged, never put in an error.
type MaGhepDaTao struct {
	// ID identifies the pairing row so the screen can poll or cancel it. It is unguessable on
	// purpose, but it is NOT the secret — holding it does not let anyone redeem the pairing.
	ID string

	// Ma is the code the QR carries. Returned once, in plaintext; only its hash is stored.
	Ma string

	// KyTuDoiChieu is shown on BOTH devices for the citizen to compare by eye. Not a secret:
	// an attacker holding these four characters still cannot pair, because they do not hold Ma.
	KyTuDoiChieu string

	// HetHanLuc is the expiry the DATABASE stored, read back rather than computed in Go, so
	// the screen counts down against the same clock the redeem is compared with. A value
	// computed here would be wrong by whatever the two clocks differ by.
	HetHanLuc time.Time
}

// chenGhepPhien writes one pairing code.
//
// het_han_luc IS COMPUTED BY THE DATABASE, not passed in. tao_luc defaults to now() and now()
// is the transaction's own timestamp, so het_han_luc is exactly tao_luc + the TTL and
// CONSTRAINT ghep_phien_ttl_toi_da_120s can never be violated by a clock that drifted. Passing
// a Go-side timestamp would make an app server running a few seconds fast unable to create a
// pairing at all — and only on that one server.
const chenGhepPhien = `
INSERT INTO ghep_phien
    (tenant_id, id, bam_ma, ky_tu_doi_chieu, het_han_luc, ip_tao, thiet_bi)
VALUES ($1, $2, $3, $4, now() + make_interval(secs => $5), $6, $7)
RETURNING het_han_luc`

// Tao creates a pairing code for the commune of the transaction, and returns the raw code once.
func (s *GhepPhienStore) Tao(ctx context.Context, tx *store.ScopedTx, m MaGhepMoi) (MaGhepDaTao, error) {
	if tx == nil {
		return MaGhepDaTao{}, ErrThieuGiaoDichGhep
	}

	id, err := maNgauNhien()
	if err != nil {
		return MaGhepDaTao{}, err
	}
	// 256 bits of randomness, where ADR 0019, invariant 1 asks for at least 128. The same
	// generator the session tokens use — one source of randomness in this package, so there is
	// only one place to get it wrong.
	ma, err := maNgauNhien()
	if err != nil {
		return MaGhepDaTao{}, err
	}
	doiChieu, err := sinhKyTuDoiChieu()
	if err != nil {
		return MaGhepDaTao{}, err
	}

	var hetHan time.Time
	err = tx.Underlying().QueryRowContext(ctx, chenGhepPhien,
		string(tx.TenantID()), id, bamRefresh(ma), doiChieu,
		ThoiHanMaGhep.Seconds(), catNgan(m.IP, 45), catNgan(m.ThietBi, 200)).Scan(&hetHan)
	if err != nil {
		// Neither the code nor its hash is in the message. An error travels into logs, and for
		// the next two minutes this value stands in for a citizen's session (rule 3).
		return MaGhepDaTao{}, fmt.Errorf("ghep_phien: tạo: %w", err)
	}
	return MaGhepDaTao{ID: id, Ma: ma, KyTuDoiChieu: doiChieu, HetHanLuc: hetHan}, nil
}

// dungGhepPhien spends a pairing code — all three facts together, exactly once.
//
// THE PREDICATE IS THE SINGLE-USE GUARANTEE, and every clause of it is load-bearing:
//
//	tenant_id = $1        the commune of the ScopedTx, never a parameter
//	dung_luc IS NULL      the row can be spent once; the second of two racing transactions
//	                      waits on the lock and then matches nothing
//	huy_luc IS NULL       a cancelled code is not redeemable
//	het_han_luc > now()   compared against the DATABASE clock, the same one that wrote it —
//	                      never a timestamp from the caller, and never a stored flag
//
// CONSTRAINT ghep_phien_dung_thi_du_ba_thu then refuses a row that records the time without the
// citizen and the session, so "redeemed by nobody" is not a state that can exist.
const dungGhepPhien = `
UPDATE ghep_phien SET dung_luc = now(), cong_dan_id = $3, phien_id = $4
WHERE tenant_id = $1 AND id = $2
  AND dung_luc IS NULL AND huy_luc IS NULL AND het_han_luc > now()`

// Dung redeems a pairing code for the session just issued to the screen.
//
// THE ORDER OF OPERATIONS THE CALLER MUST KEEP: issue the screen's session first
// (PhienCongDanStore.Tao with NguonGhep), then spend the code with that session's sid, both in
// ONE transaction with the audit entry. Spending the code first would leave a code marked used
// against a session that the rest of the transaction may never create.
//
// IT DOES NOT CHECK THE COMMUNE OF THE CITIZEN'S SESSION — that comparison is ADR 0019,
// invariant 8 and belongs to the use case, because the answer is 401 PLUS AN ALERT and this
// layer cannot raise one. What this does guarantee is narrower and mechanical: the code it
// spends is a code of THIS commune, and the foreign key refuses a phien_id from another.
func (s *GhepPhienStore) Dung(ctx context.Context, tx *store.ScopedTx, id, congDanID, phienID string) error {
	if strings.TrimSpace(id) == "" {
		return ErrThieuMaGhep
	}
	if strings.TrimSpace(congDanID) == "" || strings.TrimSpace(phienID) == "" {
		return ErrThieuPhienRa
	}
	if tx == nil {
		return ErrThieuGiaoDichGhep
	}

	kq, err := tx.Exec(ctx, dungGhepPhien, string(tx.TenantID()), id, congDanID, phienID)
	if err != nil {
		return fmt.Errorf("ghep_phien: dùng mã: %w", err)
	}
	return motDongDuyNhat(kq, ErrMaGhepKhongDungDuoc, "dùng mã")
}

// huyGhepPhien abandons a pairing before it is redeemed.
//
// `dung_luc IS NULL` IS IN THE PREDICATE, NOT ONLY `huy_luc IS NULL`. Without it a code that
// has already produced a session could be marked cancelled, and the row — which ADR 0019,
// invariant 6 keeps as evidence that a citizen authorised a specific screen at a specific time
// — would say the pairing never happened. CONSTRAINT ghep_phien_khong_vua_dung_vua_huy refuses
// that row anyway; this clause turns a constraint violation into a clean "no".
//
// There is no expiry clause: cancelling an expired-but-unused code is harmless, and refusing it
// would only force the caller to distinguish two states that mean the same thing to a screen.
const huyGhepPhien = `
UPDATE ghep_phien SET huy_luc = now(), huy_ly_do = $3
WHERE tenant_id = $1 AND id = $2 AND huy_luc IS NULL AND dung_luc IS NULL`

// Huy cancels a pairing code — the screen or the citizen walking away before it is redeemed.
//
// THE REASON IS MANDATORY, the same discipline as PhienCongDanStore.ThuHoi: the audit entry
// the caller writes beside this is only as good as what it can say about why, and "cancelled,
// no reason given" is an entry nobody can account for six months later. The schema allows NULL
// here; this layer does not.
func (s *GhepPhienStore) Huy(ctx context.Context, tx *store.ScopedTx, id, lyDo string) error {
	if strings.TrimSpace(id) == "" {
		return ErrThieuMaGhep
	}
	if strings.TrimSpace(lyDo) == "" {
		return ErrThieuLyDoHuy
	}
	if tx == nil {
		return ErrThieuGiaoDichGhep
	}

	kq, err := tx.Exec(ctx, huyGhepPhien, string(tx.TenantID()), id, catNgan(lyDo, 500))
	if err != nil {
		return fmt.Errorf("ghep_phien: huỷ mã: %w", err)
	}
	return motDongDuyNhat(kq, ErrMaGhepKhongHuyDuoc, "huỷ mã")
}

// motDongDuyNhat turns "how many rows did that touch" into an answer.
//
// ZERO ROWS IS THE SENTINEL, because on these two statements it is never an infrastructure
// failure: the predicate is what refused, and the caller has to be able to tell that apart
// from a database that fell over.
//
// MORE THAN ONE ROW IS A DEFECT AND IT FAILS LOUDLY. Both statements filter on the primary key
// (tenant_id, id), so two rows is impossible unless the key has changed — and a pairing code
// that can be spent twice in one statement is the one thing this table exists to prevent. The
// transaction the caller holds is rolled back by the error, so nothing is committed on the
// strength of a count nobody can explain.
func motDongDuyNhat(kq interface{ RowsAffected() (int64, error) }, khongCo error, viec string) error {
	n, err := kq.RowsAffected()
	if err != nil {
		return fmt.Errorf("ghep_phien: %s: %w", viec, err)
	}
	switch {
	case n == 0:
		return khongCo
	case n > 1:
		return fmt.Errorf("ghep_phien: %s: %d dòng bị đổi, khoá chính đáng lẽ chỉ cho 1", viec, n)
	}
	return nil
}

// sinhKyTuDoiChieu returns the four characters the citizen compares by eye.
func sinhKyTuDoiChieu() (string, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("ghep_phien: sinh ký tự đối chiếu: %w", err)
	}
	out := make([]byte, len(b))
	for i, v := range b {
		out[i] = chuCaiDoiChieu[v&31]
	}
	return string(out), nil
}
