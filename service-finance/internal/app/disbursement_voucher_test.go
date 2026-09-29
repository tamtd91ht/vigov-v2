package app

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// WHAT THIS FILE IS FOR: the six write use cases of the disbursement voucher register, asserted
// through the REAL store over a fake driver (fake_driver_voucher_test.go), so that every property
// below is a property of the SQL and the transaction boundaries rather than of a mock's call log.
//
// SIX THINGS, each of which fails SILENTLY if it stops holding:
//
//  1. the business write and its audit entry share ONE transaction (rule 6, invariant 3);
//  2. every refusal leaves NOTHING committed — a rolled-back transaction, no row, no entry;
//  3. the trail's actor is the STAFF BUSINESS CODE, never an internal id (rule 6, invariant 8);
//  4. the unlock rules of migration 0005 — a mandatory reason, and never the person who locked it;
//  5. `so_lan_mo_khoa` is incremented IN SQL, so no retry and no lost lock can drop a count;
//  6. `CHECK (so_tien > 0)` is not worked around anywhere, in either direction (open question #30).

// The two acting people. SEPARATE CONSTANTS, and the whole self-unlock rule is the difference
// between them: a test that used one code for both could not tell "somebody else unlocked it" from
// "the rule is not being checked".
const (
	accountantCode              = "CB-00123"
	leaderCode                  = "CB-00999"
	sampleInvestmentProjectCode = "DA-2026-be-tong-hoa-duong-ngo-xo-2"
	voucherID                   = "01JCHUNGTUDANGCO000000000"
	sampleInvestmentProjectID   = "01JDUANCUAXAA000000000000"
)

func voucherStaff(code string) audit.Actor {
	return audit.Actor{ID: code, Kind: "staff", IP: "10.0.0.7"}
}

var samplePaymentDate = time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)

func sampleCreateVoucher() CreateDisbursementVoucherRequest {
	return CreateDisbursementVoucherRequest{
		InvestmentProjectID: sampleInvestmentProjectID,
		PaymentDate:         samplePaymentDate,
		Amount:              30_000_000,
		Description:         "Thanh toán đợt 3",
		Counterparty:        "Công ty ABC",
		VoucherNo:           "CT-2026-119",
	}
}

// lockedVoucherRow returns a voucher already frozen by `by`.
func lockedVoucherRow(by string) *voucherRow {
	lockedAt := fixedTime.Add(-24 * time.Hour)
	return &voucherRow{
		id: voucherID, investmentProjectID: sampleInvestmentProjectID, paymentDate: samplePaymentDate, amount: 30_000_000,
		description: "Thanh toán đợt 3", counterparty: "Công ty ABC", voucherNo: "CT-2026-119",
		status:    string(domain.VoucherLocked),
		enteredBy: accountantCode, confirmedBy: by, lockedBy: by,
		lockedAt: &lockedAt,
	}
}

func voucherRowInStatus(status domain.VoucherStatus) *voucherRow {
	return &voucherRow{
		id: voucherID, investmentProjectID: sampleInvestmentProjectID, paymentDate: samplePaymentDate, amount: 30_000_000,
		description: "Thanh toán đợt 3", counterparty: "Công ty ABC", voucherNo: "CT-2026-119",
		status: string(status), enteredBy: accountantCode,
	}
}

// --- (1) the write and the trail share one transaction -------------------------------------------

func TestCreateVoucher_WriteAndAuditInOneTransaction(t *testing.T) {
	// THE INVARIANT THIS WHOLE LAYER EXISTS FOR (rule 6, invariant 3). The previous system had no
	// transactions anywhere, so "every write leaves a trail" could not actually hold: there was
	// always a window where the money had moved and the trail had not.
	k := &fakeVoucherStore{investmentProjectCode: sampleInvestmentProjectCode}
	uc, ctx := newVoucherUseCase(t, k)

	next, err := uc.CreateVoucher(ctx, sampleCreateVoucher(), voucherStaff(accountantCode))
	if err != nil {
		t.Fatalf("Them lỗi: %v", err)
	}
	if next.ID != newVoucherID {
		t.Errorf("id = %q, muốn %q", next.ID, newVoucherID)
	}
	if k.begins != 1 || k.commits != 1 || k.rollbacks != 0 {
		t.Fatalf("giao dịch: mở %d, commit %d, rollback %d — muốn 1/1/0",
			k.begins, k.commits, k.rollbacks)
	}
	if !k.hasStmt("INSERT INTO chung_tu_giai_ngan") {
		t.Error("không có câu chèn chứng từ")
	}
	if !k.hasStmt("INSERT INTO audit_log") {
		t.Error("không có dòng vết kiểm toán — luật 6 bất biến 1")
	}
}

func TestCreateVoucher_FailedAuditLeavesNoVoucher(t *testing.T) {
	// THE ONE ORDERING THAT MATTERS. The audit entry is the LAST statement of the transaction, so a
	// failure there has to take the voucher down with it. If it did not, the register would hold a
	// payment nobody can attribute — a state the records rules do not permit (rule 2, invariant 6).
	k := &fakeVoucherStore{investmentProjectCode: sampleInvestmentProjectCode, failOnSQL: "INSERT INTO audit_log"}
	uc, ctx := newVoucherUseCase(t, k)

	if _, err := uc.CreateVoucher(ctx, sampleCreateVoucher(), voucherStaff(accountantCode)); err == nil {
		t.Fatal("vết hỏng mà Them vẫn báo thành công")
	}
	if k.commits != 0 || k.rollbacks != 1 {
		t.Fatalf("commit %d, rollback %d — muốn 0/1", k.commits, k.rollbacks)
	}
}

func TestAuditCarriesStaffCodeNotInternalID(t *testing.T) {
	// RULE 6, INVARIANT 8. `audit_log.actor_id` is read years later by somebody handling an
	// inspection: `CB-00123` names a person with no lookup still alive, a ULID names nobody — and
	// the two are indistinguishable on sight, so a column holding both is a column nobody can query.
	// Six write paths in this repository put the internal id there on 2026-09-22 and no test turned
	// red.
	k := &fakeVoucherStore{investmentProjectCode: sampleInvestmentProjectCode}
	uc, ctx := newVoucherUseCase(t, k)

	if _, err := uc.CreateVoucher(ctx, sampleCreateVoucher(), voucherStaff(accountantCode)); err != nil {
		t.Fatalf("Them lỗi: %v", err)
	}
	audit := k.stmtsContaining("INSERT INTO audit_log")
	if len(audit) != 1 {
		t.Fatalf("có %d dòng vết, muốn 1", len(audit))
	}
	// audit.Write binds (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta).
	if got := audit[0].args[1]; got != accountantCode {
		t.Errorf("actor_id = %v, muốn MÃ CÁN BỘ %q", got, accountantCode)
	}
	if got := audit[0].args[4]; got != ActionCreateDisbursementVoucher {
		t.Errorf("action = %v, muốn %q", got, ActionCreateDisbursementVoucher)
	}
	// THE SUBJECT IS A BUSINESS CODE. A voucher has no `ma` column of its own, so it is the
	// project's code — the identifier an inspection can actually look up — and the voucher is named
	// inside the delta.
	if got := audit[0].args[5]; got != sampleInvestmentProjectCode {
		t.Errorf("subject = %v, muốn mã dự án %q", got, sampleInvestmentProjectCode)
	}
	if delta, ok := audit[0].args[7].([]byte); !ok || !strings.Contains(string(delta), newVoucherID) {
		t.Errorf("delta không nêu chứng từ nào: %s", audit[0].args[7])
	}
}

// --- (2) the state is not the client's, and the INSERT proves it ----------------------------------

func TestCreateVoucher_StatusIsLiteralInInsert(t *testing.T) {
	// `'ke-toan-nhap'` IS A LITERAL IN THE INSERT, not a bound parameter, and that is the property
	// rather than a style: with no $n for the state there is no value any layer above could pass. A
	// voucher created already `Đã khoá` would be a figure nobody confirmed, frozen against editing,
	// counting toward the commune's disbursement total.
	k := &fakeVoucherStore{investmentProjectCode: sampleInvestmentProjectCode}
	uc, ctx := newVoucherUseCase(t, k)

	if _, err := uc.CreateVoucher(ctx, sampleCreateVoucher(), voucherStaff(accountantCode)); err != nil {
		t.Fatalf("Them lỗi: %v", err)
	}
	insert := k.stmtsContaining("INSERT INTO chung_tu_giai_ngan")[0]
	if !strings.Contains(insert.sql, "'ke-toan-nhap'") {
		t.Errorf("câu chèn không viết cứng trạng thái:\n%s", insert.sql)
	}
	for _, a := range insert.args {
		if s, ok := a.(string); ok && (s == "da-khoa" || s == "da-xac-nhan") {
			t.Fatalf("trạng thái đi vào câu chèn như THAM SỐ (%q) — client đặt được vòng đời", s)
		}
	}
}

// --- (3) so_tien > 0 is not worked around, in either direction ------------------------------------

func TestCreateVoucher_NonPositiveAmountRefusedBeforeTransaction(t *testing.T) {
	// BOTH DIRECTIONS. Zero records nothing and is only ever an import artefact; NEGATIVE is a
	// REFUND — a different business event with a different name (open question #30, ADR 0035 §B) —
	// and letting it in here would silently reduce a disbursement total a decision has already
	// quoted.
	//
	// "TRƯỚC KHI MỞ GIAO DỊCH" is the second half: a request that fails its shape must not hold a
	// row lock while doing so.
	for name, amount := range map[string]domain.Dong{"không đồng": 0, "âm — khoản hoàn": -30_000_000} {
		t.Run(name, func(t *testing.T) {
			k := &fakeVoucherStore{investmentProjectCode: sampleInvestmentProjectCode}
			uc, ctx := newVoucherUseCase(t, k)

			req := sampleCreateVoucher()
			req.Amount = amount
			_, err := uc.CreateVoucher(ctx, req, voucherStaff(accountantCode))
			if !errors.Is(err, domain.ErrAmountNotPositive) {
				t.Fatalf("lỗi = %v, muốn ErrSoTienKhongDuong", err)
			}
			if k.begins != 0 {
				t.Errorf("mở %d giao dịch cho một yêu cầu sai hình dạng, muốn 0", k.begins)
			}
		})
	}
}

func TestUpdateVoucher_NonPositiveAmountRefused(t *testing.T) {
	// The same rule on the edit path, because the detour open question #30 warns about is not a
	// negative INSERT — it is editing an old voucher down. Down to a SMALLER positive figure is
	// allowed and audited; down to zero or below is refused here as it is on create.
	k := &fakeVoucherStore{investmentProjectCode: sampleInvestmentProjectCode, row: voucherRowInStatus(domain.VoucherEntered)}
	uc, ctx := newVoucherUseCase(t, k)

	amount := domain.Dong(0)
	_, err := uc.UpdateVoucher(ctx, voucherID, UpdateDisbursementVoucherRequest{Amount: &amount}, voucherStaff(accountantCode))
	if !errors.Is(err, domain.ErrAmountNotPositive) {
		t.Fatalf("lỗi = %v, muốn ErrSoTienKhongDuong", err)
	}
	if k.begins != 0 {
		t.Errorf("mở %d giao dịch, muốn 0", k.begins)
	}
}

func TestUpdateVoucher_LoweringAmountLeavesBeforeAndAfterInAudit(t *testing.T) {
	// THE DETOUR THIS DELTA EXISTS TO MAKE VISIBLE. Editing an old voucher down to a smaller figure
	// is how a refund never appears as an event. The edit is legitimate as a correction and
	// indistinguishable from the detour on the row itself; the only thing that tells them apart
	// afterwards is this pair of numbers in a ledger that cannot be edited.
	k := &fakeVoucherStore{investmentProjectCode: sampleInvestmentProjectCode, row: voucherRowInStatus(domain.VoucherEntered)}
	uc, ctx := newVoucherUseCase(t, k)

	amount := domain.Dong(12_000_000)
	if _, err := uc.UpdateVoucher(ctx, voucherID, UpdateDisbursementVoucherRequest{Amount: &amount}, voucherStaff(accountantCode)); err != nil {
		t.Fatalf("Sua lỗi: %v", err)
	}
	audit := k.stmtsContaining("INSERT INTO audit_log")
	if len(audit) != 1 {
		t.Fatalf("có %d dòng vết, muốn 1", len(audit))
	}
	delta, _ := audit[0].args[7].([]byte)
	for _, want := range []string{"30000000", "12000000"} {
		if !strings.Contains(string(delta), want) {
			t.Errorf("delta thiếu %s — không đối chiếu được số trước và sau:\n%s", want, delta)
		}
	}
}

// --- (4) a locked voucher is frozen, and the sentence is the point --------------------------------

func TestUpdateVoucher_LockedIsRefusedAndWritesNothing(t *testing.T) {
	k := &fakeVoucherStore{investmentProjectCode: sampleInvestmentProjectCode, row: lockedVoucherRow(leaderCode)}
	uc, ctx := newVoucherUseCase(t, k)

	description := "Sửa nội dung"
	_, err := uc.UpdateVoucher(ctx, voucherID, UpdateDisbursementVoucherRequest{Description: &description}, voucherStaff(accountantCode))
	if !errors.Is(err, domain.ErrVoucherLocked) {
		t.Fatalf("lỗi = %v, muốn ErrChungTuDaKhoa", err)
	}
	// NOTHING COMMITTED. The trigger would refuse the UPDATE underneath too — but it would do so
	// with an English exception naming a constraint, after the statement had been sent.
	if k.commits != 0 || k.rollbacks != 1 {
		t.Fatalf("commit %d, rollback %d — muốn 0/1", k.commits, k.rollbacks)
	}
	if k.hasStmt("UPDATE chung_tu_giai_ngan") {
		t.Error("chứng từ đang khoá mà vẫn gửi câu UPDATE")
	}
	if k.hasStmt("INSERT INTO audit_log") {
		t.Error("từ chối mà vẫn ghi vết")
	}
}

func TestRemoveVoucher_LockedIsRefused(t *testing.T) {
	k := &fakeVoucherStore{investmentProjectCode: sampleInvestmentProjectCode, row: lockedVoucherRow(leaderCode)}
	uc, ctx := newVoucherUseCase(t, k)

	err := uc.Remove(ctx, voucherID, "nhập trùng", voucherStaff(leaderCode))
	if !errors.Is(err, domain.ErrVoucherLocked) {
		t.Fatalf("lỗi = %v, muốn ErrChungTuDaKhoa", err)
	}
	if k.hasStmt("deleted_at = now()") {
		t.Error("chứng từ đang khoá mà vẫn gửi câu xoá mềm")
	}
}

func TestRemoveVoucher_MissingReasonIsRefused(t *testing.T) {
	// Rule 7, invariant 1 names THREE columns — `deleted_at`, `deleted_by`, `delete_reason`. A
	// voucher that vanished from a project's total with no reason attached is money nobody can
	// account for, and the row is still there, so the question WILL be asked.
	k := &fakeVoucherStore{investmentProjectCode: sampleInvestmentProjectCode, row: voucherRowInStatus(domain.VoucherEntered)}
	uc, ctx := newVoucherUseCase(t, k)

	if err := uc.Remove(ctx, voucherID, "   ", voucherStaff(accountantCode)); !errors.Is(err, domain.ErrRemoveReasonMissing) {
		t.Fatalf("lỗi = %v, muốn ErrThieuLyDoGo", err)
	}
	if k.begins != 0 {
		t.Errorf("mở %d giao dịch, muốn 0", k.begins)
	}
}

func TestRemoveVoucher_SoftDeleteWritesAllThreeColumns(t *testing.T) {
	k := &fakeVoucherStore{investmentProjectCode: sampleInvestmentProjectCode, row: voucherRowInStatus(domain.VoucherEntered)}
	uc, ctx := newVoucherUseCase(t, k)

	if err := uc.Remove(ctx, voucherID, "nhập trùng hai lần", voucherStaff(accountantCode)); err != nil {
		t.Fatalf("Go lỗi: %v", err)
	}
	del := k.stmtsContaining("deleted_at = now()")
	if len(del) != 1 {
		t.Fatalf("có %d câu xoá mềm, muốn 1", len(del))
	}
	for _, column := range []string{"deleted_at", "deleted_by", "delete_reason", "deleted_at IS NULL"} {
		if !strings.Contains(del[0].sql, column) {
			t.Errorf("câu xoá mềm thiếu %q:\n%s", column, del[0].sql)
		}
	}
	// NO HARD DELETE ANYWHERE. `ho_so_luu_tru_cam_xoa_cung` refuses one underneath, and nothing in
	// this package sends one.
	if k.hasStmt("DELETE FROM") {
		t.Fatal("có câu xoá cứng — luật 7 cấm #1")
	}
	// `deleted_by` HOLDS THE STAFF BUSINESS CODE, the same value the entry's actor holds.
	if got := del[0].args[2]; got != accountantCode {
		t.Errorf("deleted_by = %v, muốn mã cán bộ %q", got, accountantCode)
	}
}

// --- (5) the lifecycle is a chain -----------------------------------------------------------------

func TestLock_UnconfirmedCannotBeLocked(t *testing.T) {
	// §8.2's screen draws `Xác nhận` and `Khoá` on the same `Kế toán nhập` row, so this WILL look
	// like a bug from the outside. It is not: unlocking has to restore the state before the lock,
	// and the row does not store what that was. Requiring the chain makes "before the lock" always
	// `Đã xác nhận`, so an unlock invents nothing.
	k := &fakeVoucherStore{investmentProjectCode: sampleInvestmentProjectCode, row: voucherRowInStatus(domain.VoucherEntered)}
	uc, ctx := newVoucherUseCase(t, k)

	_, err := uc.Lock(ctx, voucherID, voucherStaff(leaderCode))
	if !errors.Is(err, domain.ErrLockRequiresConfirmation) {
		t.Fatalf("lỗi = %v, muốn ErrChuaXacNhanThiChuaKhoaDuoc", err)
	}
	if k.hasStmt("trang_thai = 'da-khoa'") {
		t.Error("chưa xác nhận mà vẫn gửi câu khoá")
	}
}

func TestLock_WritesLockerAndTimeInOneStatement(t *testing.T) {
	// BOTH COLUMNS IN ONE STATEMENT because the database requires both: `Đã khoá` with no timestamp
	// fails `chung_tu_giai_ngan_khoa_co_thoi_diem` and `Đã khoá` with nobody attached fails
	// `chung_tu_giai_ngan_khoa_co_nguoi`. Written as two statements the first would simply be
	// refused — and decision (2) of migration 0005 would have nobody to compare an unlocker against.
	k := &fakeVoucherStore{investmentProjectCode: sampleInvestmentProjectCode, row: voucherRowInStatus(domain.VoucherConfirmed)}
	uc, ctx := newVoucherUseCase(t, k)

	after, err := uc.Lock(ctx, voucherID, voucherStaff(leaderCode))
	if err != nil {
		t.Fatalf("Khoa lỗi: %v", err)
	}
	if after.Status != domain.VoucherLocked || after.LockedByID != leaderCode {
		t.Errorf("sau khi khoá = %q bởi %q", after.Status, after.LockedByID)
	}
	lock := k.stmtsContaining("trang_thai = 'da-khoa'")
	if len(lock) != 1 {
		t.Fatalf("có %d câu khoá, muốn 1", len(lock))
	}
	if !strings.Contains(lock[0].sql, "nguoi_khoa_id") || !strings.Contains(lock[0].sql, "thoi_diem_khoa") {
		t.Errorf("câu khoá không ghi đủ người và thời điểm:\n%s", lock[0].sql)
	}
	if got := lock[0].args[3]; got != fixedTime {
		t.Errorf("thoi_diem_khoa = %v, muốn %v", got, fixedTime)
	}
}

func TestConfirm_SecondTimeIsRefused(t *testing.T) {
	// The second confirmation would overwrite who confirmed it and when — editing a historical fact
	// (rule 7, forbidden #5).
	k := &fakeVoucherStore{investmentProjectCode: sampleInvestmentProjectCode, row: voucherRowInStatus(domain.VoucherConfirmed)}
	uc, ctx := newVoucherUseCase(t, k)

	if _, err := uc.Confirm(ctx, voucherID, voucherStaff(leaderCode)); !errors.Is(err, domain.ErrVoucherAlreadyConfirmed) {
		t.Fatalf("lỗi = %v, muốn ErrChungTuDaXacNhan", err)
	}
	if k.hasStmt("trang_thai = 'da-xac-nhan'") {
		t.Error("xác nhận lần hai mà vẫn gửi câu UPDATE")
	}
}

// --- (6) the unlock: the two rules of migration 0005 ----------------------------------------------

func TestUnlock_MissingReasonRefusedBeforeTransaction(t *testing.T) {
	// DECISION (1) OF MIGRATION 0005, settled by this project on 2026-09-22 and not by the customer.
	// `chung_tu_giai_ngan_mo_khoa_du_vet` refuses the row underneath; this refuses the REQUEST, in
	// Vietnamese, before a lock is taken — so the accountant is told what to add rather than shown a
	// constraint name.
	for name, reason := range map[string]string{"rỗng": "", "toàn khoảng trắng": "   \t "} {
		t.Run(name, func(t *testing.T) {
			k := &fakeVoucherStore{investmentProjectCode: sampleInvestmentProjectCode, row: lockedVoucherRow(leaderCode)}
			uc, ctx := newVoucherUseCase(t, k)

			_, err := uc.Unlock(ctx, voucherID, reason, voucherStaff(accountantCode))
			if !errors.Is(err, domain.ErrUnlockReasonMissing) {
				t.Fatalf("lỗi = %v, muốn ErrThieuLyDoMoKhoa", err)
			}
			if k.begins != 0 {
				t.Errorf("mở %d giao dịch cho một lần mở khoá không lý do, muốn 0", k.begins)
			}
			if k.hasStmt("so_lan_mo_khoa + 1") {
				t.Error("không lý do mà vẫn gửi câu mở khoá")
			}
		})
	}
}

func TestUnlock_LockerCannotReopenOwnLock(t *testing.T) {
	// DECISION (2) OF MIGRATION 0005. Nobody acts alone on the act that gives themselves room — the
	// same shape the customer already settled for the staff register in #13 and #14.
	//
	// THE CALLER HOLDS `budget.confirm` IN THIS CASE. What is refused is this PERSON against THIS
	// row, which is why it is not a 403: sending them to the Phân quyền screen would offer a
	// permission they already have.
	k := &fakeVoucherStore{investmentProjectCode: sampleInvestmentProjectCode, row: lockedVoucherRow(leaderCode)}
	uc, ctx := newVoucherUseCase(t, k)

	_, err := uc.Unlock(ctx, voucherID, "sai số tiền, phải nhập lại", voucherStaff(leaderCode))
	if !errors.Is(err, domain.ErrSelfUnlockAfterOwnLock) {
		t.Fatalf("lỗi = %v, muốn ErrTuMoKhoaChungTuMinhVuaKhoa", err)
	}
	if k.commits != 0 || k.rollbacks != 1 {
		t.Fatalf("commit %d, rollback %d — muốn 0/1", k.commits, k.rollbacks)
	}
	if k.hasStmt("so_lan_mo_khoa + 1") {
		t.Error("người vừa khoá tự mở mà vẫn gửi câu mở khoá")
	}
}

func TestUnlock_OtherStaffCanUnlock_CountIncrementsInSQL_AuditInSameTransaction(t *testing.T) {
	// DECISION (3): no ceiling, but every unlock is counted — and counted IN SQL rather than
	// read-modify-written, so the count survives the day somebody removes the row lock.
	//
	// ONE STATEMENT CARRIES THE STATE, THE REASON, THE PERSON, THE INSTANT AND THE COUNT. Two
	// statements would be two events and the second can fail: a voucher unlocked with no reason
	// recorded is the precise state decision (1) exists to prevent (0005:100-112).
	k := &fakeVoucherStore{investmentProjectCode: sampleInvestmentProjectCode, row: lockedVoucherRow(leaderCode)}
	uc, ctx := newVoucherUseCase(t, k)

	const reason = "kho bạc trả lại chứng từ, phải nhập lại số tiền"
	after, err := uc.Unlock(ctx, voucherID, reason, voucherStaff(accountantCode))
	if err != nil {
		t.Fatalf("MoKhoa lỗi: %v", err)
	}
	// BACK TO `Đã xác nhận`, EXACTLY — not chosen, derived: CanLock only admits a lock from that
	// state, so it IS where the voucher was immediately before.
	if after.Status != domain.VoucherConfirmed {
		t.Errorf("trạng thái sau khi mở = %q, muốn %q", after.Status, domain.VoucherConfirmed)
	}
	if after.UnlockCount != 1 {
		t.Errorf("so_lan_mo_khoa = %d, muốn 1", after.UnlockCount)
	}

	unlockStmts := k.stmtsContaining("so_lan_mo_khoa + 1")
	if len(unlockStmts) != 1 {
		t.Fatalf("có %d câu mở khoá, muốn 1", len(unlockStmts))
	}
	for _, column := range []string{"trang_thai = $3", "nguoi_mo_khoa_id", "thoi_diem_mo_khoa", "ly_do_mo_khoa"} {
		if !strings.Contains(unlockStmts[0].sql, column) {
			t.Errorf("câu mở khoá thiếu %q — vết mở khoá không được tách thành hai lần ghi:\n%s",
				column, unlockStmts[0].sql)
		}
	}
	// The reason reaches the column, not only the ledger.
	hasReason := false
	for _, a := range unlockStmts[0].args {
		if a == reason {
			hasReason = true
		}
	}
	if !hasReason {
		t.Errorf("lý do không đi vào câu mở khoá: %v", unlockStmts[0].args)
	}

	// SAME TRANSACTION, and the reason is in the entry as well as in the column: the column is
	// overwritten by the NEXT unlock, the ledger is append-only. Neither is derivable from the other.
	if k.begins != 1 || k.commits != 1 {
		t.Fatalf("giao dịch: mở %d, commit %d — muốn 1/1", k.begins, k.commits)
	}
	audit := k.stmtsContaining("INSERT INTO audit_log")
	if len(audit) != 1 {
		t.Fatalf("có %d dòng vết, muốn 1", len(audit))
	}
	if got := audit[0].args[4]; got != ActionUnlockDisbursementVoucher {
		t.Errorf("action = %v, muốn %q", got, ActionUnlockDisbursementVoucher)
	}
	delta, _ := audit[0].args[7].([]byte)
	if !strings.Contains(string(delta), reason) {
		t.Errorf("delta không mang lý do mở khoá:\n%s", delta)
	}
}

func TestUnlock_UnlockedVoucherHasNothingToUnlock(t *testing.T) {
	k := &fakeVoucherStore{investmentProjectCode: sampleInvestmentProjectCode, row: voucherRowInStatus(domain.VoucherConfirmed)}
	uc, ctx := newVoucherUseCase(t, k)

	_, err := uc.Unlock(ctx, voucherID, "gõ nhầm", voucherStaff(accountantCode))
	if !errors.Is(err, domain.ErrVoucherNotLocked) {
		t.Fatalf("lỗi = %v, muốn ErrChungTuChuaKhoa", err)
	}
}

// --- (7) a voucher must belong to a live project of THIS commune ----------------------------------

func TestCreateVoucher_InvestmentProjectNotInTenantIsRefused(t *testing.T) {
	// Money filed against a project id that matches nothing counts toward NO project's total while
	// sitting in the register looking healthy — the commune's own figures then disagree with the sum
	// of its own vouchers and no row looks wrong. There is no foreign key underneath (0004:182-190
	// says why), so this check is the only one there is.
	k := &fakeVoucherStore{investmentProjectCode: ""}
	uc, ctx := newVoucherUseCase(t, k)

	_, err := uc.CreateVoucher(ctx, sampleCreateVoucher(), voucherStaff(accountantCode))
	if !errors.Is(err, fistore.ErrVoucherInvestmentProjectNotFound) {
		t.Fatalf("lỗi = %v, muốn ErrKhongThayDuAnCuaChungTu", err)
	}
	if k.hasStmt("INSERT INTO chung_tu_giai_ngan") {
		t.Error("dự án không có trong xã mà vẫn chèn chứng từ")
	}
	if k.commits != 0 {
		t.Errorf("commit %d, muốn 0", k.commits)
	}
}

// --- (8) a no-op writes nothing, which is what idem.KhongCan claims -------------------------------

func TestUpdateVoucher_NoFieldChangedWritesNothingAndNoAudit(t *testing.T) {
	// Sending a voucher the figures it already has is not an event. Recording it would fill a public
	// authority's ledger with entries saying nothing changed, and those are the entries that bury
	// the ones carrying legal weight. It is also what makes PATCH's `idem.KhongCan` declaration true
	// rather than hopeful.
	k := &fakeVoucherStore{investmentProjectCode: sampleInvestmentProjectCode, row: voucherRowInStatus(domain.VoucherEntered)}
	uc, ctx := newVoucherUseCase(t, k)

	description := "Thanh toán đợt 3" // exactly what the row already holds
	if _, err := uc.UpdateVoucher(ctx, voucherID, UpdateDisbursementVoucherRequest{Description: &description}, voucherStaff(accountantCode)); err != nil {
		t.Fatalf("Sua lỗi: %v", err)
	}
	if k.hasStmt("UPDATE chung_tu_giai_ngan") {
		t.Error("không có gì đổi mà vẫn gửi câu UPDATE")
	}
	if k.hasStmt("INSERT INTO audit_log") {
		t.Error("không có gì đổi mà vẫn ghi vết")
	}
	if k.commits != 1 {
		t.Errorf("commit %d, muốn 1 — không ghi gì vẫn là một giao dịch kết thúc sạch", k.commits)
	}
}

// --- (9) no write without an actor ----------------------------------------------------------------

func TestEveryWriteRequiresActor(t *testing.T) {
	// core/audit refuses an entry with no actor; refusing HERE keeps `nguoi_nhap_id` / `deleted_by` /
	// `nguoi_khoa_id` and the entry telling the same story, and avoids a rollback whose cause is a
	// missing principal rather than anything about the voucher.
	k := &fakeVoucherStore{investmentProjectCode: sampleInvestmentProjectCode, row: lockedVoucherRow(leaderCode)}
	uc, ctx := newVoucherUseCase(t, k)

	if _, err := uc.CreateVoucher(ctx, sampleCreateVoucher(), audit.Actor{}); err == nil {
		t.Error("Them chạy mà không có người thực hiện")
	}
	if err := uc.Remove(ctx, voucherID, "lý do", audit.Actor{}); err == nil {
		t.Error("Go chạy mà không có người thực hiện")
	}
	if _, err := uc.Lock(ctx, voucherID, audit.Actor{}); err == nil {
		t.Error("Khoa chạy mà không có người thực hiện")
	}
	if _, err := uc.Unlock(ctx, voucherID, "lý do", audit.Actor{}); err == nil {
		t.Error("MoKhoa chạy mà không có người thực hiện")
	}
	if k.begins != 0 {
		t.Errorf("mở %d giao dịch mà không có chủ thể, muốn 0", k.begins)
	}
}

// --- sửa một chứng từ ĐÃ XÁC NHẬN thì nó về nháp ---------------------------------------------------
//
// The customer's rule, measured in `../vigov-require` commit `c3f4d6a` and taken up on 23/09/2026:
// *"lãnh đạo xác nhận những con số kia, không phải những con số này"*. It fills a gap open questions
// #29 and #30 left — they settled UNLOCKING and REFUNDS and say nothing about a confirmation whose
// figures moved — and contradicts neither.

func TestUpdateConfirmedVoucherResetsToEnteredAndClearsConfirmer(t *testing.T) {
	before := voucherRowInStatus(domain.VoucherConfirmed)
	before.confirmedBy = leaderCode
	k := &fakeVoucherStore{investmentProjectCode: sampleInvestmentProjectCode, row: before}
	uc, ctx := newVoucherUseCase(t, k)

	amount := domain.Dong(31_000_000)
	after, err := uc.UpdateVoucher(ctx, voucherID, UpdateDisbursementVoucherRequest{Amount: &amount}, voucherStaff(accountantCode))
	if err != nil {
		t.Fatalf("Sua lỗi: %v", err)
	}
	if after.Status != domain.VoucherEntered {
		t.Fatalf("trạng thái sau khi sửa = %q, muốn %q", after.Status, domain.VoucherEntered)
	}
	if after.ConfirmedByID != "" {
		t.Fatalf("người xác nhận còn %q — dòng vẫn khai một lãnh đạo đã duyệt những con số họ "+
			"chưa từng thấy", after.ConfirmedByID)
	}

	// TWO STATEMENTS, ONE TRANSACTION. The figures and the state move separately so that a reader of
	// the row can tell WHICH of the two happened — the same reason 0004:132-135 gives for the
	// trigger's column allowlist.
	resetStmts := k.stmtsContaining("trang_thai = 'ke-toan-nhap'")
	if len(resetStmts) != 1 {
		t.Fatalf("có %d câu đưa về nháp, muốn 1", len(resetStmts))
	}
	if !strings.Contains(resetStmts[0].sql, "nguoi_xac_nhan_id = NULL") {
		t.Fatalf("câu về nháp không xoá người xác nhận: %s", resetStmts[0].sql)
	}
	if k.begins != 1 || k.commits != 1 || k.rollbacks != 0 {
		t.Fatalf("giao dịch: mở %d commit %d rollback %d, muốn 1/1/0",
			k.begins, k.commits, k.rollbacks)
	}

	// WHO HAD CONFIRMED IT IS IN THE ENTRY, and only there: the column has just been nulled, so the
	// append-only ledger is the one place that still answers "whose confirmation did this edit undo".
	audit := k.stmtsContaining("INSERT INTO audit_log")
	if len(audit) != 1 {
		t.Fatalf("có %d dòng vết, muốn 1", len(audit))
	}
	body := auditBody(t, audit[0])
	if !strings.Contains(body, "mat_xac_nhan") {
		t.Fatalf("vết không ghi việc mất xác nhận: %s", body)
	}
	if !strings.Contains(body, leaderCode) {
		t.Fatalf("vết không ghi NGƯỜI đã xác nhận trước đó: %s", body)
	}
}

func TestUpdateUnconfirmedVoucherKeepsStatus(t *testing.T) {
	// The other two states are untouched. A voucher already in `Kế toán nhập` must not pick up a
	// second, pointless state write — it would appear in the trail as a change that did not happen.
	k := &fakeVoucherStore{investmentProjectCode: sampleInvestmentProjectCode, row: voucherRowInStatus(domain.VoucherEntered)}
	uc, ctx := newVoucherUseCase(t, k)

	amount := domain.Dong(31_000_000)
	after, err := uc.UpdateVoucher(ctx, voucherID, UpdateDisbursementVoucherRequest{Amount: &amount}, voucherStaff(accountantCode))
	if err != nil {
		t.Fatalf("Sua lỗi: %v", err)
	}
	if after.Status != domain.VoucherEntered {
		t.Fatalf("trạng thái = %q, muốn %q", after.Status, domain.VoucherEntered)
	}
	if k.hasStmt("trang_thai = 'ke-toan-nhap'") {
		t.Fatal("chứng từ chưa xác nhận mà vẫn ghi lại trạng thái")
	}
}

func TestUpdateLockedVoucherIsStillRefusedNotReset(t *testing.T) {
	// THE RULE ADDED THIS TURN MUST NOT BECOME A WAY AROUND THE LOCK. A locked voucher is refused
	// before any UPDATE is attempted — CanUpdate first, and the `chung_tu_da_khoa` trigger underneath —
	// so the only path to a frozen figure is still the unlock route, with a reason, by somebody else.
	k := &fakeVoucherStore{investmentProjectCode: sampleInvestmentProjectCode, row: lockedVoucherRow(leaderCode)}
	uc, ctx := newVoucherUseCase(t, k)

	amount := domain.Dong(31_000_000)
	_, err := uc.UpdateVoucher(ctx, voucherID, UpdateDisbursementVoucherRequest{Amount: &amount}, voucherStaff(accountantCode))
	if !errors.Is(err, domain.ErrVoucherLocked) {
		t.Fatalf("lỗi = %v, muốn ErrChungTuDaKhoa", err)
	}
	if k.hasStmt("trang_thai = 'ke-toan-nhap'") {
		t.Fatal("chứng từ đã khoá mà vẫn bị đưa về nháp")
	}
	if k.commits != 0 || k.rollbacks != 1 {
		t.Fatalf("commit %d rollback %d, muốn 0/1", k.commits, k.rollbacks)
	}
}

func TestUpdateConfirmedVoucherWithNoChangeKeepsConfirmation(t *testing.T) {
	// THE NO-OP BRANCH PROTECTS A LEADER'S ACT. `idem.KhongCan` on the PATCH route claims a repeat is
	// harmless; without this, a second identical request would strip a confirmation off a voucher
	// nobody edited.
	before := voucherRowInStatus(domain.VoucherConfirmed)
	before.confirmedBy = leaderCode
	k := &fakeVoucherStore{investmentProjectCode: sampleInvestmentProjectCode, row: before}
	uc, ctx := newVoucherUseCase(t, k)

	description := "Thanh toán đợt 3" // exactly what the row already holds
	after, err := uc.UpdateVoucher(ctx, voucherID, UpdateDisbursementVoucherRequest{Description: &description}, voucherStaff(accountantCode))
	if err != nil {
		t.Fatalf("Sua lỗi: %v", err)
	}
	if after.Status != domain.VoucherConfirmed || after.ConfirmedByID != leaderCode {
		t.Fatalf("không đổi gì mà xác nhận bị gỡ: trạng thái %q, người %q",
			after.Status, after.ConfirmedByID)
	}
	if k.hasStmt("trang_thai = 'ke-toan-nhap'") || k.hasStmt("INSERT INTO audit_log") {
		t.Fatal("không đổi gì mà vẫn ghi")
	}
}

// --- (10) the funding source of a payment (§8.2's `NGUỒN VỐN` column, migration 0007) --------------
//
// FOUR PROPERTIES, and the first is the one that fails in COMPLETE SILENCE:
//
//  1. an edit that moves ONLY the funding source is a real edit — it writes, it audits, and it sends
//     a confirmed voucher back to `Kế toán nhập` (ADR 0036);
//  2. "no source" reaches the column as NULL and never as '' — the two spellings are the difference
//     between §6's warning counting a voucher and losing it;
//  3. the source must be a LIVE source OF THIS COMMUNE, checked inside the transaction because there
//     is no foreign key underneath (0007:102-113);
//  4. the audit delta carries the move, because *"ai chuyển chứng từ này sang nguồn khác"* is a
//     question an inspection asks and the row can only ever answer where the money sits NOW.

const (
	oldFundingSourceID = "01JNGUONVONNGANSACHXA0000"
	newFundingSourceID = "01JNGUONVONXAHOIHOA000000"
)

// storeWithFundingSources builds a store holding both sample sources of this commune.
func storeWithFundingSources(row *voucherRow) *fakeVoucherStore {
	return &fakeVoucherStore{
		investmentProjectCode: sampleInvestmentProjectCode,
		row:                   row,
		liveFundingSources:    map[string]bool{oldFundingSourceID: true, newFundingSourceID: true},
	}
}

func TestUpdateVoucher_OnlyFundingSourceChanged_IsStillARealUpdate(t *testing.T) {
	// ⚠ THE TRAP THIS TEST EXISTS FOR, AND IT IS INVISIBLE WITHOUT EXACTLY THIS CASE. `voucherUnchanged`
	// compares the editable fields and UpdateVoucher RETURNS EARLY when it reports "nothing moved". A field that
	// is editable but missing from that comparison is a field whose edit writes no row, leaves no audit
	// entry and does NOT send a confirmed voucher back to `Kế toán nhập` — while the request answers
	// 200 with the old values. Nothing is red anywhere.
	//
	// SO THE EDIT HERE TOUCHES THE FUNDING SOURCE AND NOTHING ELSE. Every other assertion about
	// `nguon_von_id` in this file changes a second field as well, and every one of them would stay
	// green with the comparison left unfixed.
	before := voucherRowInStatus(domain.VoucherConfirmed)
	before.fundingSourceID = oldFundingSourceID
	before.confirmedBy = leaderCode
	k := storeWithFundingSources(before)
	uc, ctx := newVoucherUseCase(t, k)

	next := newFundingSourceID
	after, err := uc.UpdateVoucher(ctx, voucherID, UpdateDisbursementVoucherRequest{FundingSourceID: &next}, voucherStaff(accountantCode))
	if err != nil {
		t.Fatalf("Sua lỗi: %v", err)
	}
	if after.FundingSourceID != newFundingSourceID {
		t.Fatalf("nguồn vốn sau khi sửa = %q, muốn %q", after.FundingSourceID, newFundingSourceID)
	}

	// (a) IT WRITES. A voucher that changed source and was never written is money still counted on the
	// old card of §6, with the screen showing the new one.
	update := k.stmtsContaining("UPDATE chung_tu_giai_ngan")
	if len(update) == 0 {
		t.Fatal("đổi RIÊNG nguồn vốn mà không gửi câu UPDATE nào — `khongDoiChungTu` đang coi đây là " +
			"không đổi gì")
	}
	if got := update[0].args[7]; got != newFundingSourceID {
		t.Errorf("nguon_von_id vào câu cập nhật = %v, muốn %q", got, newFundingSourceID)
	}

	// (b) IT COSTS THE CONFIRMATION (ADR 0036). Moving a payment to another funding source moves it
	// between two cards of §6 — the leader confirmed the figures on the old one.
	if after.Status != domain.VoucherEntered || after.ConfirmedByID != "" {
		t.Fatalf("trạng thái %q, người xác nhận %q — sửa một chứng từ đã xác nhận thì phải VỀ NHÁP",
			after.Status, after.ConfirmedByID)
	}
	if len(k.stmtsContaining("trang_thai = 'ke-toan-nhap'")) != 1 {
		t.Fatal("không có câu đưa về nháp")
	}

	// (c) IT LEAVES A TRAIL NAMING BOTH SOURCES, in the same transaction.
	if k.begins != 1 || k.commits != 1 || k.rollbacks != 0 {
		t.Fatalf("giao dịch: mở %d commit %d rollback %d, muốn 1/1/0",
			k.begins, k.commits, k.rollbacks)
	}
	audit := k.stmtsContaining("INSERT INTO audit_log")
	if len(audit) != 1 {
		t.Fatalf("có %d dòng vết, muốn 1", len(audit))
	}
	body := auditBody(t, audit[0])
	for _, want := range []string{oldFundingSourceID, newFundingSourceID} {
		if !strings.Contains(body, want) {
			t.Errorf("vết thiếu %q — không trả lời được *ai chuyển chứng từ này sang nguồn khác*:\n%s",
				want, body)
		}
	}
}

func TestCreateVoucher_NoSourceWritesNULLNotEmptyString(t *testing.T) {
	// §13 rule 6: a voucher with no funding source is a state the specification DEFINES and §6 REPORTS
	// ("Còn 3,4 tỷ đã chi nhưng chưa ghi rút từ nguồn nào"). The column carries
	// `CHECK (nguon_von_id IS NULL OR btrim(nguon_von_id) <> '')` (0007:274-277), so '' is refused
	// outright by the database — and if that CHECK were ever dropped, the voucher would fall out of the
	// warning while belonging to no source either: money missing from BOTH sides of one screen.
	for name, sent := range map[string]string{"bỏ trống": "", "toàn khoảng trắng": "   "} {
		t.Run(name, func(t *testing.T) {
			k := storeWithFundingSources(nil)
			uc, ctx := newVoucherUseCase(t, k)

			req := sampleCreateVoucher()
			req.FundingSourceID = sent
			if _, err := uc.CreateVoucher(ctx, req, voucherStaff(accountantCode)); err != nil {
				t.Fatalf("Them lỗi: %v", err)
			}
			insert := k.stmtsContaining("INSERT INTO chung_tu_giai_ngan")
			if len(insert) != 1 {
				t.Fatalf("có %d câu chèn, muốn 1", len(insert))
			}
			if got := insert[0].args[8]; got != nil {
				t.Fatalf("nguon_von_id vào câu chèn = %#v, muốn NULL", got)
			}
			// AND NOTHING WAS ASKED OF THE CATALOGUE. "No source" has nothing to verify; a lookup here
			// would turn the specification's own state into an error.
			if k.hasStmt("FROM nguon_von") {
				t.Error("không gắn nguồn mà vẫn đi hỏi bảng nguồn vốn")
			}
		})
	}
}

func TestUpdateVoucher_DetachFromSource_WritesNULLAndDoesNotQueryCatalogue(t *testing.T) {
	// `""` MEANS DETACH, and it is the middle of the three answers the pointer carries: nil leaves the
	// source alone, "" puts the voucher back into §6's "đã chi nhưng chưa ghi rút từ nguồn nào", an id
	// attaches it. A real correction — the accountant attributed a payment to the wrong source and the
	// right one is not yet known.
	before := voucherRowInStatus(domain.VoucherEntered)
	before.fundingSourceID = oldFundingSourceID
	k := storeWithFundingSources(before)
	uc, ctx := newVoucherUseCase(t, k)

	detach := ""
	after, err := uc.UpdateVoucher(ctx, voucherID, UpdateDisbursementVoucherRequest{FundingSourceID: &detach}, voucherStaff(accountantCode))
	if err != nil {
		t.Fatalf("Sua lỗi: %v", err)
	}
	if after.FundingSourceID != "" {
		t.Fatalf("nguồn vốn = %q, muốn rỗng", after.FundingSourceID)
	}
	update := k.stmtsContaining("UPDATE chung_tu_giai_ngan")
	if len(update) != 1 {
		t.Fatalf("có %d câu cập nhật, muốn 1", len(update))
	}
	if got := update[0].args[7]; got != nil {
		t.Fatalf("nguon_von_id vào câu cập nhật = %#v, muốn NULL — '' bị CSDL từ chối, và nếu "+
			"CHECK bị gỡ thì chứng từ rơi khỏi cả cảnh báo §6 lẫn mọi thẻ nguồn", got)
	}
	// DETACHING VERIFIES NOTHING. There is no source to look up, and demanding one would make the
	// state §13 rule 6 defines unreachable from the edit path.
	if k.hasStmt("FROM nguon_von") {
		t.Error("gỡ khỏi nguồn mà vẫn đi hỏi bảng nguồn vốn")
	}
}

func TestCreateVoucher_FundingSourceNotInTenantIsRefusedAndWritesNothing(t *testing.T) {
	// RULE 1, AND THERE IS NO FOREIGN KEY UNDERNEATH TO CATCH IT (0007:102-113). An id naming another
	// commune's source is indistinguishable from one that never existed, because the statement binds
	// tenant_id = $1 — and either way the money would sit on no card of §6 while ALSO being absent from
	// the "chưa ghi rút từ nguồn nào" warning, which counts NULL.
	k := &fakeVoucherStore{investmentProjectCode: sampleInvestmentProjectCode, liveFundingSources: map[string]bool{}}
	uc, ctx := newVoucherUseCase(t, k)

	req := sampleCreateVoucher()
	req.FundingSourceID = newFundingSourceID
	_, err := uc.CreateVoucher(ctx, req, voucherStaff(accountantCode))
	if !errors.Is(err, fistore.ErrVoucherFundingSourceNotFound) {
		t.Fatalf("lỗi = %v, muốn ErrKhongThayNguonVonCuaChungTu", err)
	}
	if k.hasStmt("INSERT INTO chung_tu_giai_ngan") {
		t.Error("nguồn vốn không có trong xã mà vẫn chèn chứng từ")
	}
	if k.hasStmt("INSERT INTO audit_log") {
		t.Error("từ chối mà vẫn ghi vết")
	}
	if k.commits != 0 || k.rollbacks != 1 {
		t.Fatalf("commit %d, rollback %d — muốn 0/1", k.commits, k.rollbacks)
	}
}

func TestUpdateVoucher_MoveToSourceNotInTenantIsRefused(t *testing.T) {
	// The same refusal on the edit path, and it is NOT redundant with the create one: the two call
	// sites are different branches, and the edit path is the one where the check is conditional.
	before := voucherRowInStatus(domain.VoucherEntered)
	before.fundingSourceID = oldFundingSourceID
	k := &fakeVoucherStore{investmentProjectCode: sampleInvestmentProjectCode, row: before, liveFundingSources: map[string]bool{oldFundingSourceID: true}}
	uc, ctx := newVoucherUseCase(t, k)

	foreign := "01JNGUONVONCUAXAKHAC00000"
	_, err := uc.UpdateVoucher(ctx, voucherID, UpdateDisbursementVoucherRequest{FundingSourceID: &foreign}, voucherStaff(accountantCode))
	if !errors.Is(err, fistore.ErrVoucherFundingSourceNotFound) {
		t.Fatalf("lỗi = %v, muốn ErrKhongThayNguonVonCuaChungTu", err)
	}
	if k.hasStmt("UPDATE chung_tu_giai_ngan") {
		t.Error("nguồn vốn của xã khác mà vẫn gửi câu cập nhật")
	}
	if k.commits != 0 || k.rollbacks != 1 {
		t.Fatalf("commit %d, rollback %d — muốn 0/1", k.commits, k.rollbacks)
	}
}

func TestUpdateVoucher_UnchangedSourceDoesNotRequireSourceStillExists(t *testing.T) {
	// THE CHECK IS ABOUT THE MOVE, NOT ABOUT THE ROW. A voucher recorded last year against a source the
	// commune has since removed from its catalogue is a historical fact; refusing to correct its
	// DESCRIPTION because of that would strand the record with no way to fix a typo. What is refused is
	// attaching money to a source that is not there — which this edit does not do.
	before := voucherRowInStatus(domain.VoucherEntered)
	before.fundingSourceID = oldFundingSourceID
	k := &fakeVoucherStore{investmentProjectCode: sampleInvestmentProjectCode, row: before, liveFundingSources: map[string]bool{}} // nguồn đã bị gỡ
	uc, ctx := newVoucherUseCase(t, k)

	description := "Thanh toán đợt 4"
	after, err := uc.UpdateVoucher(ctx, voucherID, UpdateDisbursementVoucherRequest{Description: &description}, voucherStaff(accountantCode))
	if err != nil {
		t.Fatalf("Sua lỗi: %v", err)
	}
	if after.FundingSourceID != oldFundingSourceID {
		t.Errorf("nguồn vốn = %q, muốn giữ nguyên %q", after.FundingSourceID, oldFundingSourceID)
	}
	if k.hasStmt("FROM nguon_von") {
		t.Error("không đụng tới nguồn vốn mà vẫn đi hỏi bảng nguồn vốn")
	}
	if k.commits != 1 {
		t.Errorf("commit %d, muốn 1", k.commits)
	}
}

func TestUpdateVoucher_ResendingCurrentSourceWritesNothing(t *testing.T) {
	// THE NO-OP BRANCH, WITH THE NEW FIELD. A screen that resends the whole form must not strip a
	// leader's confirmation off a voucher nobody changed — that is what `idem.KhongCan` on the PATCH
	// route claims, and this is the claim with `nguon_von_id` in the body.
	before := voucherRowInStatus(domain.VoucherConfirmed)
	before.fundingSourceID = oldFundingSourceID
	before.confirmedBy = leaderCode
	k := storeWithFundingSources(before)
	uc, ctx := newVoucherUseCase(t, k)

	same := oldFundingSourceID // exactly what the row already holds
	after, err := uc.UpdateVoucher(ctx, voucherID, UpdateDisbursementVoucherRequest{FundingSourceID: &same}, voucherStaff(accountantCode))
	if err != nil {
		t.Fatalf("Sua lỗi: %v", err)
	}
	if after.Status != domain.VoucherConfirmed || after.ConfirmedByID != leaderCode {
		t.Fatalf("không đổi gì mà xác nhận bị gỡ: %q / %q", after.Status, after.ConfirmedByID)
	}
	if k.hasStmt("UPDATE chung_tu_giai_ngan") || k.hasStmt("INSERT INTO audit_log") {
		t.Error("gửi lại đúng nguồn đang có mà vẫn ghi")
	}
}

func TestUpdateVoucher_LockedVoucherCannotChangeFundingSource(t *testing.T) {
	// 0007 ADDED `nguon_von_id` TO THE `chung_tu_da_khoa` TRIGGER'S FROZEN LIST (0007:329), because
	// without it the funding source would have been the ONLY business fact of a locked voucher anybody
	// could rewrite — moving money between two cards of §6 after the figure was signed off, leaving the
	// voucher itself looking untouched. This asserts the half that arrives FIRST: CanUpdate refuses before
	// any UPDATE is sent, in Vietnamese, naming the way out.
	k := storeWithFundingSources(lockedVoucherRow(leaderCode))
	uc, ctx := newVoucherUseCase(t, k)

	next := newFundingSourceID
	_, err := uc.UpdateVoucher(ctx, voucherID, UpdateDisbursementVoucherRequest{FundingSourceID: &next}, voucherStaff(accountantCode))
	if !errors.Is(err, domain.ErrVoucherLocked) {
		t.Fatalf("lỗi = %v, muốn ErrChungTuDaKhoa", err)
	}
	if k.hasStmt("UPDATE chung_tu_giai_ngan") {
		t.Error("chứng từ đã khoá mà vẫn gửi câu cập nhật nguồn vốn")
	}
	if k.commits != 0 || k.rollbacks != 1 {
		t.Fatalf("commit %d, rollback %d — muốn 0/1", k.commits, k.rollbacks)
	}
}

// auditBody returns the JSON delta of one audit statement as text.
func auditBody(t *testing.T, l recordedStmt) string {
	t.Helper()
	for _, a := range l.args {
		if b, ok := a.([]byte); ok {
			return string(b)
		}
	}
	t.Fatalf("câu vết không mang delta: %v", l.args)
	return ""
}
