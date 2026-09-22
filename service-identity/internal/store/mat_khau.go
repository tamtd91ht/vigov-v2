package store

// The three WRITE paths that touch a staff credential, and the one read they all decide on.
//
// WHY THREE STATEMENTS AND NOT ONE WITH PARAMETERS. Each writes `mat_khau_hash` together with a
// DIFFERENT value of `phai_doi_mat_khau`, and that pairing is the whole of open question #9:
//
//	CapTaiKhoan          co_tai_khoan = true  · hash = temporary · phai_doi_mat_khau = true
//	DatMatKhauTam        (#17, administrator resets for somebody) · hash = temporary · true
//	DoiMatKhauChinhMinh  the person themselves · hash = their own · phai_doi_mat_khau = FALSE
//
// A single method with a bool would have to be given a value by `DatMatKhauHash` in can_bo.go too —
// the silent argon2 rehash after a successful sign-in — and there is no value that method could
// pass that is not a lie: `true` orders a change nobody asked for, `false` quietly releases an
// account still carrying the password an administrator typed. That comment is already written at
// DatMatKhauHash; these three are its other half.
//
// THE FLAG IS WRITTEN IN THE SAME STATEMENT AS THE HASH, NEVER AFTERWARDS. Two statements can
// commit one and lose the other, and the direction that loses is the dangerous one: a hash written
// with the flag left false is a temporary password that is valid forever, working perfectly, with
// nothing to report it (migration 0009 §1).
//
// FIVE THINGS HOLD HERE EXACTLY AS THEY DO IN can_bo_ghi.go, and for the same reasons: the commune
// is $1 from tx.TenantID() and never a parameter; nothing opens its own transaction; every read
// excludes soft-deleted rows; the read the guards decide on takes `FOR UPDATE`; and `ma` is in no
// UPDATE, because an issued staff code is never renumbered.
//
// NOTHING IN THIS FILE EVER SEES A PLAINTEXT PASSWORD. Every parameter named `bam` is already an
// argon2id encoding produced by core/password.Bam. A store that accepted the plaintext would be a
// store one careless `fmt.Errorf("%s", ...)` away from putting a working credential into a log
// line (rule 3, forbidden #1; rule 8).

import (
	"context"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// TheoIDDeGhiMatKhau reads one staff row inside the transaction, credential included, and holds it
// until the transaction ends.
//
// IT DELIBERATELY DOES NOT FILTER `co_tai_khoan` OR `dang_hoat_dong`, which is the one way it
// differs from TheoEmail and TheoID. The three callers want three different answers about those two
// columns — issuing an account requires `co_tai_khoan` to be FALSE, resetting requires it TRUE,
// and a person changing their own password must be both — so a predicate here could only be right
// for one of them. The row is handed over as it is and each use case refuses for its own reason,
// with a sentence that says which reason.
//
// IT DOES FILTER `deleted_at`, like every other read on this table: a person kept for the audit
// trail (rule 7) is not a person anybody issues a credential to.
//
// `FOR UPDATE` IS THE POINT, not an optimisation — the same argument as TheoIDDeGhi. Every write
// below is read-decide-write, and two administrators resetting one person at the same instant would
// otherwise both read the old row, both mint a temporary password, and both read one out: the
// second write wins silently and one of the two people is dictating a password that no longer
// works.
//
// IT RETURNS domain.CanBo — the SAME type and the SAME column list (cotCanBo) as the sign-in read.
// A second column list would be a second positional Scan to keep in lockstep, and cotCanBo already
// carries the warning about `phai_doi_mat_khau` sitting between two TEXT columns so a positional
// slip fails loudly instead of silently.
func (s *CanBoStore) TheoIDDeGhiMatKhau(ctx context.Context, tx *store.ScopedTx, id string) (domain.CanBo, error) {
	if id == "" {
		// Fail closed rather than compare against the empty string, which matches whatever row
		// happens to carry it. Same reading as TheoIDDeGhi.
		return domain.CanBo{}, ErrCanBoKhongTonTai
	}

	const stmt = `SELECT ` + cotCanBo + ` FROM nguoi_dung ` +
		`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR UPDATE`

	rows, err := tx.Underlying().QueryContext(ctx, stmt, string(tx.TenantID()), id)
	if err != nil {
		return domain.CanBo{}, fmt.Errorf("can_bo: đọc dòng để ghi mật khẩu: %w", err)
	}
	defer rows.Close()
	return motCanBo(rows)
}

// capTaiKhoanCanBo turns a directory row into an account.
//
// `AND NOT co_tai_khoan` IS IN THE WHERE CLAUSE, and it is not a duplicate of the use case's own
// check. It is what makes this statement unable to overwrite the password of an account that
// already exists, whatever a caller believes it read a moment ago — the one shape in which
// "issuing an account" can never become "silently replacing somebody's credential". With the
// FOR UPDATE read above it is also what the two halves of a race collapse onto: the second
// transaction matches no row and the write reports it instead of winning.
//
// `phai_doi_mat_khau = true` IS WRITTEN EXPLICITLY although the column DEFAULTs to true. The
// default protects an INSERT; this is an UPDATE on a row that has existed since the directory entry
// was created, so the value sitting there is whatever that row has carried all along. Relying on a
// default here would be relying on a value nothing in this statement sets.
const capTaiKhoanCanBo = `UPDATE nguoi_dung
	SET co_tai_khoan = true, mat_khau_hash = $3, phai_doi_mat_khau = true, cap_nhat_luc = now()
	WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL AND NOT co_tai_khoan`

// CapTaiKhoan gives a directory row a sign-in account carrying a temporary password (#9).
func (s *CanBoStore) CapTaiKhoan(ctx context.Context, tx *store.ScopedTx, id, bam string) error {
	if bam == "" {
		// The CHECK constraint `nguoi_dung_co_tai_khoan_co_mat_khau` (migration 0009 §3) refuses
		// this too, and refusing here as well is deliberate: the server's message names a
		// constraint on a partition, which tells nobody which flow produced an account with no
		// credential.
		return errors.New("can_bo: cấp tài khoản với chuỗi băm rỗng")
	}
	kq, err := tx.Exec(ctx, capTaiKhoanCanBo, string(tx.TenantID()), id, bam)
	if err != nil {
		return dichLoiGhiCanBo("cấp tài khoản", err)
	}
	return doiMotDong(kq, "cấp tài khoản")
}

// datMatKhauTamCanBo replaces the credential of an EXISTING account and marks it as not the
// person's own (#17 — the administrator resets for them, because there is no self-service path).
//
// `AND co_tai_khoan` IS THE MIRROR OF THE PREDICATE ABOVE: this statement cannot create an account,
// only replace the password of one that exists. Between the two, neither route can do the other's
// job however its handler is called.
const datMatKhauTamCanBo = `UPDATE nguoi_dung
	SET mat_khau_hash = $3, phai_doi_mat_khau = true, cap_nhat_luc = now()
	WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL AND co_tai_khoan`

// DatMatKhauTam is the administrator resetting somebody else's password (#17).
func (s *CanBoStore) DatMatKhauTam(ctx context.Context, tx *store.ScopedTx, id, bam string) error {
	if bam == "" {
		return errors.New("can_bo: đặt lại mật khẩu với chuỗi băm rỗng")
	}
	kq, err := tx.Exec(ctx, datMatKhauTamCanBo, string(tx.TenantID()), id, bam)
	if err != nil {
		return dichLoiGhiCanBo("đặt lại mật khẩu", err)
	}
	return doiMotDong(kq, "đặt lại mật khẩu")
}

// doiMatKhauChinhMinh is the ONLY statement in this repository that writes
// `phai_doi_mat_khau = false`.
//
// THAT EXCLUSIVITY IS THE INVARIANT, not a coincidence of who happened to need it: migration 0009
// §1 says the flag is "cleared ONLY by the person themselves changing it". Every other write path
// either sets it true or does not name the column at all (DatMatKhauHash, the silent rehash). If a
// second statement ever clears it, an administrator has gained a way to mint a password that is
// valid forever — which is precisely the state #9 exists to end.
//
// `AND co_tai_khoan AND dang_hoat_dong` — the same two conditions the sign-in read applies. A
// person whose account was withdrawn or locked mid-session must not be able to write a new
// credential on the way out; the middleware already stops building a principal for them, and this
// is the second half of that, at the statement.
const doiMatKhauChinhMinh = `UPDATE nguoi_dung
	SET mat_khau_hash = $3, phai_doi_mat_khau = false, cap_nhat_luc = now()
	WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL AND co_tai_khoan AND dang_hoat_dong`

// DoiMatKhauChinhMinh is the person setting their own password — the one act that clears the flag.
func (s *CanBoStore) DoiMatKhauChinhMinh(ctx context.Context, tx *store.ScopedTx, id, bam string) error {
	if bam == "" {
		return errors.New("can_bo: đổi mật khẩu với chuỗi băm rỗng")
	}
	kq, err := tx.Exec(ctx, doiMatKhauChinhMinh, string(tx.TenantID()), id, bam)
	if err != nil {
		return dichLoiGhiCanBo("đổi mật khẩu", err)
	}
	return doiMotDong(kq, "đổi mật khẩu")
}
