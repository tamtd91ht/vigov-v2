package store

// The WRITE path of the disbursement voucher register (`chung_tu_giai_ngan`).
//
// FIVE THINGS HOLD ACROSS EVERY METHOD BELOW, and each one is a defect class rather than a style:
//
//  1. THE COMMUNE IS $1 IN EVERY STATEMENT, taken from tx.TenantID() which took it from the
//     context (rule 1, invariants 4 and 5). It is never a parameter of any method here, so a
//     caller cannot name another commune's row even by mistake.
//  2. NOTHING HERE OPENS A TRANSACTION. The caller opens it and writes the audit entry inside it
//     (rule 6, invariant 3) — internal/app is the only layer that may.
//  3. EVERY READ EXCLUDES SOFT-DELETED ROWS. There is no method here that can see one.
//  4. THERE IS NO HARD DELETE, and `ho_so_luu_tru_cam_xoa_cung` (0004:325-328) refuses one even
//     if somebody writes it (rule 7, forbidden #1).
//  5. `so_lan_mo_khoa` IS NEVER READ INTO Go AND WRITTEN BACK. It is incremented in SQL, so a
//     count cannot be lost by a read-modify-write even on the day somebody removes the row lock.
//
// THE FIVE UNLOCK-LEDGER COLUMNS (migration 0005) ARE MENTIONED IN EXACTLY ONE STATEMENT — the
// unlock UPDATE. That is load-bearing rather than tidy: 0005:114-121 states plainly that the
// `chung_tu_da_khoa` trigger does NOT guard those columns (it cannot, because an unlock has to
// write them while the row is still `da-khoa`), so what keeps a reason from being rewritten
// afterwards is that no other statement in this package names them.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// DisbursementVoucherStore writes one commune's disbursement vouchers.
//
// It holds *store.DB and never a *sql.DB. The commune is taken from the context on every call, so
// there is no constructor, no field and no method here that could produce a statement without one
// (rule 1, invariant 5).
type DisbursementVoucherStore struct {
	db *store.DB
}

func NewDisbursementVoucherStore(db *store.DB) *DisbursementVoucherStore {
	return &DisbursementVoucherStore{db: db}
}

var (
	// ErrVoucherNotFound is "no such live voucher IN THIS COMMUNE" — and the two halves are one
	// answer on purpose. A voucher of another commune is indistinguishable from one that does not
	// exist, because the query cannot reach it at all; the caller answers 404 either way and so
	// leaks nothing about what another authority holds.
	ErrVoucherNotFound = errors.New("chung_tu_giai_ngan: không có chứng từ này trong xã")

	// ErrVoucherInvestmentProjectNotFound — the project a voucher is being filed against does not exist in
	// this commune.
	//
	// WHY A VOUCHER MUST NAME A LIVE PROJECT OF THIS COMMUNE: `da_giai_ngan` is SUM(so_tien) over
	// the vouchers joined to a project (investment_project.go, voucherTotals). Money filed against a project id
	// that matches nothing is money that counts toward NO project's total while sitting in the
	// register looking healthy — the commune's own figures then disagree with the sum of its own
	// vouchers, and no row looks wrong. There is no foreign key in the database to catch it
	// (0004:182-190 says why), so this check is the only one there is.
	ErrVoucherInvestmentProjectNotFound = errors.New("chung_tu_giai_ngan: không có dự án này trong xã")

	// ErrVoucherFundingSourceNotFound — the funding source a voucher is being attached to does not
	// exist in this commune, or has been removed from its catalogue.
	//
	// WHY THE APPLICATION HAS TO ASK AT ALL: migration 0007 declares NO foreign key from
	// `chung_tu_giai_ngan.nguon_von_id` to `nguon_von`, and it says so outright rather than leaving
	// the absence to be discovered — a real FK between two HASH-partitioned tables is accepted by
	// PostgreSQL 13, but no PostgreSQL is reachable from this repository's build environment and a
	// migration that fails at startup stops the service (0007:102-113, following 0004:182-190).
	//
	// SO THIS CHECK IS THE ONLY ONE THERE IS, AND IT GUARDS TWO DIFFERENT FAILURES AT ONCE:
	//
	//	an id that matches nothing   the voucher's money would belong to no card on §6 while the row
	//	                             looks filled in — and it would ALSO be missing from the "đã chi
	//	                             nhưng chưa ghi rút từ nguồn nào" warning, because that warning
	//	                             counts NULL. Money gone from both sides of the screen.
	//	a source of ANOTHER commune  rule 1. The statement binds tenant_id = $1, so such an id is
	//	                             indistinguishable from one that does not exist — which is the
	//	                             answer this error carries, and it leaks nothing about what
	//	                             another authority holds.
	ErrVoucherFundingSourceNotFound = errors.New("chung_tu_giai_ngan: không có nguồn vốn này trong xã")
)

// voucherColumns IS READ BY POSITION in the scans below, and the list has two groups that swap with no
// error whatsoever:
//
//	nguoi_nhap_id, nguoi_xac_nhan_id,   four staff codes in a row. A swap makes the person who
//	  nguoi_khoa_id, nguoi_mo_khoa_id   locked a voucher read as the person who unlocked it —
//	                                    which is the exact comparison decision (2) of 0005 is made
//	                                    of, so the rule would still "work" and would answer about
//	                                    the wrong people.
//	thoi_diem_khoa, thoi_diem_mo_khoa   two timestamps. A swap reads as a voucher unlocked before
//	                                    it was locked, which nothing validates.
//
// That is why this is a constant rather than a column list written inline at each call site.
//
// COALESCE ON EVERY NULLABLE TEXT COLUMN, so the domain struct carries "" rather than needing a
// sql.NullString per field. The two timestamps cannot be COALESCEd to anything honest — there is
// no "zero" instant — so they are scanned through sql.NullTime.
const voucherColumns = `id, du_an_id, ngay_chi, so_tien, noi_dung, ` +
	`COALESCE(doi_tac, ''), COALESCE(so_chung_tu, ''), COALESCE(nguon_von_id, ''), trang_thai, ` +
	`nguoi_nhap_id, COALESCE(nguoi_xac_nhan_id, ''), ` +
	`COALESCE(nguoi_khoa_id, ''), COALESCE(nguoi_mo_khoa_id, ''), ` +
	`thoi_diem_khoa, thoi_diem_mo_khoa, ` +
	`COALESCE(ly_do_mo_khoa, ''), so_lan_mo_khoa`

// scanVoucher is the shared Scan of one row. Positional, in lockstep with voucherColumns — see
// the note there on the two groups of same-typed columns.
func scanVoucher(scan func(...any) error) (domain.DisbursementVoucher, error) {
	var (
		voucher    domain.DisbursementVoucher
		amount     int64
		lockedAt   sql.NullTime
		unlockedAt sql.NullTime
		status     string
	)
	err := scan(&voucher.ID, &voucher.InvestmentProjectID, &voucher.PaymentDate, &amount, &voucher.Description,
		&voucher.Counterparty, &voucher.VoucherNo, &voucher.FundingSourceID, &status,
		&voucher.EnteredByID, &voucher.ConfirmedByID,
		&voucher.LockedByID, &voucher.UnlockedByID,
		&lockedAt, &unlockedAt,
		&voucher.UnlockReason, &voucher.UnlockCount)
	if err != nil {
		return domain.DisbursementVoucher{}, err
	}
	voucher.Amount = domain.Dong(amount)
	voucher.Status = domain.VoucherStatus(status)
	if lockedAt.Valid {
		voucher.LockedAt = lockedAt.Time
	}
	if unlockedAt.Valid {
		voucher.UnlockedAt = unlockedAt.Time
	}
	return voucher, nil
}

// ByIDForUpdate reads one live voucher inside the transaction and holds it until the transaction ends.
//
// `FOR UPDATE` IS THE POINT OF THIS METHOD AND NOT AN OPTIMISATION. Every write below is a
// read-decide-write: read the voucher, work out whether the lifecycle admits the operation, refuse
// or apply. Without the lock two members of staff acting on the same voucher both read the old
// state and both decide against it — and the pair that actually costs something is
// confirm+lock racing unlock, where the second write silently produces a locked voucher whose
// `nguoi_khoa_id` names somebody who never locked this version of it.
//
// IT IS ALSO WHAT MAKES THE UNLOCK RULE ENFORCEABLE AT ALL. Decision (2) of migration 0005 compares
// the unlocker against `nguoi_khoa_id`; read without a lock, two people could both read the same
// `nguoi_khoa_id`, both find themselves to be somebody else, and both unlock — the second one
// overwriting the first one's reason.
func (s *DisbursementVoucherStore) ByIDForUpdate(ctx context.Context, tx *store.ScopedTx,
	id string) (domain.DisbursementVoucher, error) {

	const stmt = `SELECT ` + voucherColumns + ` FROM chung_tu_giai_ngan ` +
		`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR UPDATE`

	voucher, err := scanVoucher(
		tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), id).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.DisbursementVoucher{}, ErrVoucherNotFound
	}
	if err != nil {
		return domain.DisbursementVoucher{}, fmt.Errorf("chung_tu_giai_ngan: đọc chứng từ để sửa: %w", err)
	}
	return voucher, nil
}

// LiveInvestmentProjectCode returns the BUSINESS CODE of one live project of this commune.
//
// IT SERVES TWO PURPOSES AT ONCE AND BOTH ARE LOAD-BEARING:
//
//	the existence check   a voucher filed against a project this commune does not have is money
//	                      that totals nowhere — see ErrVoucherInvestmentProjectNotFound.
//	the audit subject     `audit_log.subject` holds a BUSINESS code, never an internal id
//	                      (rule 6, invariant 8). A voucher has no code of its own — there is no
//	                      `ma` column on `chung_tu_giai_ngan` — so the project's code is the
//	                      business identifier an inspection can actually look up, and the voucher
//	                      itself is named inside the delta.
//
// NOT `FOR UPDATE`: this reads a fact about a row nothing here writes. Locking the project row for
// every voucher write would serialise all of one project's data entry behind one clerk.
func (s *DisbursementVoucherStore) LiveInvestmentProjectCode(ctx context.Context, tx *store.ScopedTx,
	investmentProjectID string) (string, error) {

	const stmt = `SELECT ma FROM du_an ` +
		`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

	var code string
	err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), investmentProjectID).Scan(&code)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrVoucherInvestmentProjectNotFound
	}
	if err != nil {
		return "", fmt.Errorf("chung_tu_giai_ngan: đọc mã dự án: %w", err)
	}
	return code, nil
}

// CheckLiveFundingSource refuses a funding source that is not a live source OF THIS COMMUNE.
//
// IT LIVES HERE AND NOT ON FundingSourceStore, AND THE REASON IS THE TRANSACTION. Every method of this
// file takes the caller's *store.ScopedTx; FundingSourceStore reads through db.For(ctx), which is a
// connection of its own. Checking through that one would ask the question OUTSIDE the transaction
// holding the voucher's row lock — so the answer could already be stale by the time the UPDATE runs,
// and a voucher could be attached to a source removed in between. One transaction, one answer
// (rule 6, invariant 3 applied to the check that decides the write).
//
// NOT `FOR UPDATE`, for the same reason LiveInvestmentProjectCode is not: this reads a fact about a row nothing
// here writes, and locking the catalogue row would serialise every commune's voucher entry behind
// whoever last touched a funding source.
//
// IT RETURNS ONLY AN ERROR. There is no `ma` column on `nguon_von` (§11 gives it a name and no
// code), so unlike LiveInvestmentProjectCode there is no business code to hand back for the audit subject — the
// subject stays the PROJECT's code and the source id is named inside the delta.
func (s *DisbursementVoucherStore) CheckLiveFundingSource(ctx context.Context, tx *store.ScopedTx,
	fundingSourceID string) error {

	const stmt = `SELECT 1 FROM nguon_von ` +
		`WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

	var one int
	err := tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), fundingSourceID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrVoucherFundingSourceNotFound
	}
	if err != nil {
		return fmt.Errorf("chung_tu_giai_ngan: kiểm nguồn vốn: %w", err)
	}
	return nil
}

// insertVoucher — `trang_thai` IS A LITERAL AND NOT A PARAMETER.
//
// Read that as the property it is, not as a shortcut. A new voucher is `Kế toán nhập`, always:
// there is no $n for the state, so there is no value a handler could pass and no field a client
// could fill. Turning it into a parameter is the one edit that would let a client create a voucher
// already `Đã khoá` — a figure nobody confirmed, frozen against editing, counting toward the
// commune's disbursement total.
//
// `tep_dinh_kem` IS ABSENT AND TAKES ITS DEFAULT `[]`. Attachments are not part of this write path
// and the column is in the trigger's frozen list (0004:154).
//
// `so_lan_mo_khoa` IS ABSENT AND TAKES ITS DEFAULT 0. A voucher that has never been unlocked has
// been unlocked zero times, which is a fact rather than an absence (0005:145-148).
//
// `nguon_von_id` IS BOUND THROUGH emptyToNil LIKE THE OTHER TWO OPTIONAL COLUMNS, and here that is
// not tidiness: `CHECK (nguon_von_id IS NULL OR btrim(nguon_von_id) <> ”)` (0007:274-277) REFUSES an
// empty string outright, so a voucher entered with no source would fail the INSERT — and with it the
// audit entry sharing the transaction. NULL is the value §6's "đã chi nhưng chưa ghi rút từ nguồn
// nào" counts.
const insertVoucher = `INSERT INTO chung_tu_giai_ngan
	(tenant_id, id, du_an_id, ngay_chi, so_tien, noi_dung, doi_tac, so_chung_tu,
	 nguon_von_id, trang_thai, nguoi_nhap_id)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'ke-toan-nhap', $10)`

// InsertVoucher adds one voucher, in the first state of the lifecycle.
//
// `nguoi_nhap_id` HOLDS THE STAFF BUSINESS CODE (`CB-2026-7K3M9Q`), never the internal ULID —
// migration 0005:52-59 states the convention for all four `nguoi_*_id` columns, and it is the same
// value `audit_log.actor_id` holds (rule 6, invariant 8).
func (s *DisbursementVoucherStore) InsertVoucher(ctx context.Context, tx *store.ScopedTx,
	voucher domain.DisbursementVoucher) error {

	_, err := tx.Exec(ctx, insertVoucher, string(tx.TenantID()),
		voucher.ID, voucher.InvestmentProjectID, voucher.PaymentDate, int64(voucher.Amount), voucher.Description,
		emptyToNil(voucher.Counterparty), emptyToNil(voucher.VoucherNo), emptyToNil(voucher.FundingSourceID),
		voucher.EnteredByID)
	if err != nil {
		return fmt.Errorf("chung_tu_giai_ngan: chèn: %w", err)
	}
	return nil
}

// updateVoucher — `trang_thai`, `du_an_id` AND EVERY `nguoi_*_id` APPEAR NOWHERE IN THIS STATEMENT.
//
// `trang_thai` and the four staff codes because the lifecycle moves them, each through its own
// statement below, so that an edit and a state change can never be one event — which is exactly
// what 0004:132-135 requires: "one UPDATE can do both at once — unlock AND rewrite the amount".
//
// `du_an_id` because moving a voucher between projects moves money between two reported totals
// with nothing on either screen saying so. If a voucher was filed against the wrong project, the
// operation is to remove it with a reason and enter it again — two events, both audited.
//
// THE TRIGGER REFUSES THE SAME FIELDS ON A LOCKED ROW UNDERNEATH (0004:148-159), and both layers
// are meant: the trigger is the floor that holds against every writer, and the application refuses
// first with a sentence an accountant can act on.
//
// `nguon_von_id` IS IN THIS STATEMENT AND IS THE SIXTH EDITABLE FIELD. Moving a payment from one
// source to another is a correction a commune really makes — the accountant typed it before the
// source list was agreed — and it is also the change an inspection asks about by name, which is why
// the audit delta carries it. A LOCKED voucher never reaches here: CanUpdate refuses first, and 0007
// added this very column to the `chung_tu_da_khoa` trigger's frozen list underneath (0007:329), so
// the floor refuses it too.
const updateVoucher = `UPDATE chung_tu_giai_ngan
	SET ngay_chi = $3, so_tien = $4, noi_dung = $5, doi_tac = $6, so_chung_tu = $7,
	    nguon_von_id = $8, cap_nhat_luc = now()
	WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

// UpdateVoucher writes the six fields a member of staff may correct on an UNLOCKED voucher. The caller
// has already read the row with ByIDForUpdate and asked domain.DisbursementVoucher.CanUpdate.
func (s *DisbursementVoucherStore) UpdateVoucher(ctx context.Context, tx *store.ScopedTx,
	voucher domain.DisbursementVoucher) error {

	res, err := tx.Exec(ctx, updateVoucher, string(tx.TenantID()),
		voucher.ID, voucher.PaymentDate, int64(voucher.Amount), voucher.Description,
		emptyToNil(voucher.Counterparty), emptyToNil(voucher.VoucherNo), emptyToNil(voucher.FundingSourceID))
	if err != nil {
		return fmt.Errorf("chung_tu_giai_ngan: cập nhật: %w", err)
	}
	return expectOneVoucherRow(res, "cập nhật")
}

// resetToEnteredSQL sends a CONFIRMED voucher back to `Kế toán nhập` because its figures just changed
// — *"lãnh đạo xác nhận những con số kia, không phải những con số này"* (ADR-less customer rule,
// `../vigov-require` commit `c3f4d6a`; see domain.StatusAfterUpdate).
//
// A SECOND STATEMENT AND NOT A COLUMN ON updateVoucher, and the reason is the one 0004:132-135
// gives for the trigger's allowlist: one UPDATE that moved both the amount and the state would be a
// single row change out of which nobody could read WHICH of the two happened. Two statements in one
// transaction are two facts, and the audit entry names both.
//
// `nguoi_xac_nhan_id = NULL` IS PART OF IT AND IS NOT ERASING HISTORY. Leaving the code there would
// leave the row asserting that a named officer confirmed figures they have never seen — which is
// precisely the claim this rule exists to stop. Who HAD confirmed it, and when it was undone, are in
// the append-only entry that shares this transaction; the column describes the row's CURRENT state,
// and the row is currently unconfirmed.
//
// THE ROW IS NEVER LOCKED WHEN THIS RUNS: domain.CanUpdate refuses a locked voucher before any UPDATE
// is attempted, and `chung_tu_da_khoa` refuses it underneath. So this statement cannot be the way a
// frozen figure is quietly reopened.
const resetToEnteredSQL = `UPDATE chung_tu_giai_ngan
	SET trang_thai = 'ke-toan-nhap', nguoi_xac_nhan_id = NULL, cap_nhat_luc = now()
	WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

// ResetToEnteredAfterUpdate returns one corrected voucher to the first state of the lifecycle.
func (s *DisbursementVoucherStore) ResetToEnteredAfterUpdate(ctx context.Context, tx *store.ScopedTx,
	id string) error {

	res, err := tx.Exec(ctx, resetToEnteredSQL, string(tx.TenantID()), id)
	if err != nil {
		return fmt.Errorf("chung_tu_giai_ngan: về nháp sau khi sửa: %w", err)
	}
	return expectOneVoucherRow(res, "về nháp sau khi sửa")
}

// Confirm moves a voucher from `Kế toán nhập` to `Đã xác nhận`, recording who.
const confirmVoucher = `UPDATE chung_tu_giai_ngan
	SET trang_thai = 'da-xac-nhan', nguoi_xac_nhan_id = $3, cap_nhat_luc = now()
	WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

func (s *DisbursementVoucherStore) Confirm(ctx context.Context, tx *store.ScopedTx,
	id, staffCode string) error {

	res, err := tx.Exec(ctx, confirmVoucher, string(tx.TenantID()), id, staffCode)
	if err != nil {
		return fmt.Errorf("chung_tu_giai_ngan: xác nhận: %w", err)
	}
	return expectOneVoucherRow(res, "xác nhận")
}

// lockVoucher freezes a voucher, recording who and when.
//
// BOTH COLUMNS IN ONE STATEMENT, AND THAT IS WHAT THE DATABASE REQUIRES: `Đã khoá` with no
// timestamp violates `chung_tu_giai_ngan_khoa_co_thoi_diem` (0004:308) and `Đã khoá` with nobody
// attached violates `chung_tu_giai_ngan_khoa_co_nguoi` (0005:158-161). Written as two statements,
// the first one would simply be refused.
const lockVoucher = `UPDATE chung_tu_giai_ngan
	SET trang_thai = 'da-khoa', nguoi_khoa_id = $3, thoi_diem_khoa = $4, cap_nhat_luc = now()
	WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

func (s *DisbursementVoucherStore) Lock(ctx context.Context, tx *store.ScopedTx,
	id, staffCode string, at time.Time) error {

	res, err := tx.Exec(ctx, lockVoucher, string(tx.TenantID()), id, staffCode, at)
	if err != nil {
		return fmt.Errorf("chung_tu_giai_ngan: khoá: %w", err)
	}
	return expectOneVoucherRow(res, "khoá")
}

// unlockVoucher reopens a voucher and records the whole unlock in ONE statement.
//
// ONE STATEMENT IS THE DESIGN AND NOT A CONVENIENCE — 0005:100-112 sets it out. The state moves
// while `OLD.trang_thai` is still `da-khoa`, which is what unlocking means; recording the reason in
// a second statement would mean the state had already moved before the reason existed, and a second
// statement can fail. A voucher unlocked with no reason recorded is the precise state decision (1)
// of that migration exists to prevent, and `chung_tu_giai_ngan_mo_khoa_du_vet` refuses it outright.
//
// `so_lan_mo_khoa + 1` IS COMPUTED IN SQL. Under ByIDForUpdate's row lock a read-modify-write in Go
// would be equally correct today; in SQL it stays correct on the day somebody removes the lock, and
// a counter that silently stops counting is exactly the shape decision (3) — "no ceiling, but every
// one is counted" — cannot survive.
//
// THE STATE IT RETURNS TO IS THE CALLER'S, from domain.StatusAfterUnlock. It is a parameter
// rather than a literal here because the rule ("back to `Đã xác nhận`, because that is provably
// where it was") is reasoned about in the domain, and a literal here would be a second, silent copy
// of that reasoning in SQL.
const unlockVoucher = `UPDATE chung_tu_giai_ngan
	SET trang_thai = $3,
	    nguoi_mo_khoa_id = $4, thoi_diem_mo_khoa = $5, ly_do_mo_khoa = $6,
	    so_lan_mo_khoa = so_lan_mo_khoa + 1,
	    cap_nhat_luc = now()
	WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

func (s *DisbursementVoucherStore) Unlock(ctx context.Context, tx *store.ScopedTx,
	id string, statusBack domain.VoucherStatus, staffCode, reason string, at time.Time) error {

	res, err := tx.Exec(ctx, unlockVoucher, string(tx.TenantID()), id,
		string(statusBack), staffCode, at, reason)
	if err != nil {
		return fmt.Errorf("chung_tu_giai_ngan: mở khoá: %w", err)
	}
	return expectOneVoucherRow(res, "mở khoá")
}

// softDeleteVoucher writes all THREE columns rule 7, invariant 1 names — `deleted_at`, `deleted_by` and
// `delete_reason` — in one statement, so none of them can be forgotten.
//
// `AND deleted_at IS NULL` IS WHAT MAKES A SECOND REMOVAL A 404 rather than a silent rewrite of who
// removed the voucher and why. The first removal is the one that happened; overwriting its reason
// would be editing a historical record (rule 7, forbidden #5).
//
// THE ROW STAYS. `ho_so_luu_tru_cam_xoa_cung` refuses a hard DELETE on this table (0004:325-328),
// and the voucher's money drops out of `da_giai_ngan` because every read path excludes soft-deleted
// rows (rule 7, invariant 2) — not because anything was destroyed.
const softDeleteVoucher = `UPDATE chung_tu_giai_ngan
	SET deleted_at = now(), deleted_by = $3, delete_reason = $4, cap_nhat_luc = now()
	WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`

func (s *DisbursementVoucherStore) SoftDelete(ctx context.Context, tx *store.ScopedTx,
	id, by, reason string) error {

	res, err := tx.Exec(ctx, softDeleteVoucher, string(tx.TenantID()), id, by, reason)
	if err != nil {
		return fmt.Errorf("chung_tu_giai_ngan: xoá mềm: %w", err)
	}
	return expectOneVoucherRow(res, "xoá mềm")
}

// expectOneVoucherRow turns "nothing was updated" into ErrVoucherNotFound.
//
// IT IS A SECOND COPY OF expectOneRow (catalogue_write.go) ON PURPOSE, and the difference is the error
// it returns: that one answers ErrCatalogueRowNotFound, which a handler maps to "Không tìm thấy mục
// danh mục này." Sharing it would put a sentence about a catalogue row in front of somebody who was
// removing a payment voucher.
func expectOneVoucherRow(res sql.Result, op string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("chung_tu_giai_ngan: %s: đọc số dòng: %w", op, err)
	}
	if n == 0 {
		return ErrVoucherNotFound
	}
	return nil
}

// emptyToNil writes an empty optional TEXT column as NULL rather than as ”.
//
// WHY IT MATTERS ON THIS TABLE SPECIFICALLY: `doi_tac` and `so_chung_tu` are both optional (§8.2's
// own sample rows show `—`), and the reads above COALESCE them to "". Storing ” as well would give
// one absent value two spellings in one column, so `WHERE so_chung_tu IS NULL` — the obvious way
// anybody would later look for vouchers with no treasury number — would silently miss half of them.
func emptyToNil(s string) any {
	if s == "" {
		return nil
	}
	return s
}
