package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// WHAT THIS FILE IS FOR: budget period close (migration 0012) through the REAL store on the fake
// driver (driver_gia_ngan_sach_test.go says what that does and does not prove — in particular NOTHING
// here proves the advisory lock serialises anything; that needs a PostgreSQL).
//
//  1. close / reopen: one transaction, the audit entry inside it, subject = the close's code, actor =
//     the staff code; the code is built from the revision; a second close / reopen is refused;
//  2. every guarded write takes the (tenant, year) advisory lock BEFORE reading the closes, ascending;
//  3. an entry dated in a closed month or year, or on a sheet of a closed year, is refused — add AND
//     remove — and a reopened close no longer locks;
//  4. a YEAR close refuses every sheet / line / value write of that year; a MONTH close does not;
//  5. an adjustment entry stores and audits its reason; a blank one is refused.

func closeRow(id, code string, year, month, rev int) domain.BudgetPeriodClose {
	return domain.BudgetPeriodClose{ID: id, Code: code, Year: year, Month: month, Revision: rev,
		ClosedBy: "CB-00012", ClosedAt: time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)}
}

func reopenedRow(c domain.BudgetPeriodClose) domain.BudgetPeriodClose {
	c.ReopenedAt = time.Date(2026, 10, 2, 8, 0, 0, 0, time.UTC)
	c.ReopenedBy, c.ReopenReason = "CB-00003", "Ghi sót đợt thu phí chợ"
	return c
}

// wroteNothing — no business write and no audit entry reached the driver, and nothing committed.
func wroteNothing(t *testing.T, k *khoNSGia) {
	t.Helper()
	for _, tu := range []string{"INSERT INTO", "UPDATE ", "audit_log"} {
		if k.coCau(tu) {
			t.Fatalf("bị từ chối mà vẫn có câu %q", tu)
		}
	}
	if k.daCommit != 0 {
		t.Fatalf("bị từ chối mà commit %d lần", k.daCommit)
	}
}

// --- (1) close ---------------------------------------------------------------------------------------

func TestCloseMonthOneTransactionAuditedUnderTheLock(t *testing.T) {
	k := khoMau()
	uc, ctx := dungUseCaseNganSach(t, k)

	c, err := uc.CloseBudgetPeriod(ctx, BudgetPeriodCloseRequest{Year: 2026, Month: 9}, nguoiGhi())
	if err != nil {
		t.Fatalf("CloseBudgetPeriod lỗi: %v", err)
	}
	if c.Code != "CK-2026-09-01" || c.Revision != 1 || c.ClosedBy != maCanBo {
		t.Fatalf("lần chốt = %+v", c)
	}
	if k.batDau != 1 || k.daCommit != 1 || k.daRollback != 0 {
		t.Fatalf("giao dịch: mở %d commit %d rollback %d, muốn 1/1/0", k.batDau, k.daCommit, k.daRollback)
	}
	// THE LOCK BEFORE THE CHECK, THE CHECK BEFORE THE INSERT, THE INSERT BEFORE THE ENTRY.
	order := []string{"pg_advisory_xact_lock", "FROM budget_period_closes", "MAX(revision)",
		"INSERT INTO budget_period_closes", "INSERT INTO audit_log"}
	for i := 1; i < len(order); i++ {
		if k.thuTuCua(order[i-1]) > k.thuTuCua(order[i]) {
			t.Fatalf("%q chạy sau %q", order[i-1], order[i])
		}
	}
	lock := k.cau("pg_advisory_xact_lock")
	if len(lock) != 1 || !coGiaTri(lock[0], string(xaA)) || !coGiaTri(lock[0], int64(2026)) {
		t.Fatalf("khoá tư vấn phải mang xã của giao dịch và năm: %v", lock)
	}
	ins := k.cau("INSERT INTO budget_period_closes")[0]
	if !coGiaTri(ins, "CK-2026-09-01") || !coGiaTri(ins, maCanBo) || !coGiaTri(ins, int64(9)) {
		t.Fatalf("câu chèn chốt kỳ: %v", ins.args)
	}
	vet := k.cau("INSERT INTO audit_log")
	if len(vet) != 1 || !coGiaTri(vet[0], "CK-2026-09-01") || !coGiaTri(vet[0], maCanBo) ||
		!coGiaTri(vet[0], ActionBudgetPeriodClose) {
		t.Fatalf("vết chốt kỳ: %v", vet)
	}
	for _, want := range []string{`"year":2026`, `"month":9`, `"revision":1`} {
		if !coChuoiTrongDelta(vet[0], want) {
			t.Errorf("delta thiếu %s", want)
		}
	}
}

func TestCloseYearIsCNCodeAndNullMonth(t *testing.T) {
	k := khoMau()
	uc, ctx := dungUseCaseNganSach(t, k)
	c, err := uc.CloseBudgetPeriod(ctx, BudgetPeriodCloseRequest{Year: 2026}, nguoiGhi())
	if err != nil {
		t.Fatalf("lỗi: %v", err)
	}
	if c.Code != "CK-2026-CN-01" || !c.IsYearClose() {
		t.Fatalf("= %+v", c)
	}
	ins := k.cau("INSERT INTO budget_period_closes")[0]
	if ins.args[4] != nil {
		t.Fatalf("tháng của lần chốt cả năm phải là NULL, được %v", ins.args[4])
	}
	if !coChuoiTrongDelta(k.cau("INSERT INTO audit_log")[0], `"month":null`) {
		t.Fatal("delta chốt cả năm phải ghi month null")
	}
}

func TestCloseAlreadyClosedIsRefusedNamingIt(t *testing.T) {
	k := khoMau()
	k.closes = []domain.BudgetPeriodClose{closeRow("k1", "CK-2026-09-01", 2026, 9, 1)}
	uc, ctx := dungUseCaseNganSach(t, k)

	_, err := uc.CloseBudgetPeriod(ctx, BudgetPeriodCloseRequest{Year: 2026, Month: 9}, nguoiGhi())
	if !errors.Is(err, domain.ErrPeriodAlreadyClosed) || !strings.Contains(err.Error(), "CK-2026-09-01") {
		t.Fatalf("= %v, muốn ErrPeriodAlreadyClosed nêu CK-2026-09-01", err)
	}
	wroteNothing(t, k)
}

func TestReCloseAfterReopenIsTheNextRevision(t *testing.T) {
	k := khoMau()
	k.closes = []domain.BudgetPeriodClose{reopenedRow(closeRow("k1", "CK-2026-09-01", 2026, 9, 1))}
	k.nextRevision = 2
	uc, ctx := dungUseCaseNganSach(t, k)

	c, err := uc.CloseBudgetPeriod(ctx, BudgetPeriodCloseRequest{Year: 2026, Month: 9}, nguoiGhi())
	if err != nil {
		t.Fatalf("chốt lại sau mở chốt bị từ chối: %v", err)
	}
	if c.Code != "CK-2026-09-02" || c.Revision != 2 {
		t.Fatalf("= %+v, muốn CK-2026-09-02 lần 2", c)
	}
}

func TestCloseUniqueViolationIsAlreadyClosed(t *testing.T) {
	// The floor under a writer that did not take the lock (an old replica mid-rollout).
	k := khoMau()
	k.loiSau = "INSERT INTO budget_period_closes"
	k.loiSauLa = &pgconn.PgError{Code: "23505", ConstraintName: "budget_period_closes_p03_tenant_id_year_month_idx"}
	uc, ctx := dungUseCaseNganSach(t, k)

	_, err := uc.CloseBudgetPeriod(ctx, BudgetPeriodCloseRequest{Year: 2026, Month: 9}, nguoiGhi())
	if !errors.Is(err, domain.ErrPeriodAlreadyClosed) {
		t.Fatalf("= %v, muốn ErrPeriodAlreadyClosed", err)
	}
	if k.daCommit != 0 || k.coCau("INSERT INTO audit_log") {
		t.Fatal("vi phạm khoá duy nhất mà vẫn ghi vết / commit")
	}
}

func TestCloseAuditFailureTakesTheCloseDown(t *testing.T) {
	k := khoMau()
	k.loiSau = "INSERT INTO audit_log"
	uc, ctx := dungUseCaseNganSach(t, k)
	if _, err := uc.CloseBudgetPeriod(ctx, BudgetPeriodCloseRequest{Year: 2026, Month: 9}, nguoiGhi()); err == nil {
		t.Fatal("vết hỏng mà chốt kỳ vẫn báo thành công")
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Fatalf("commit %d rollback %d, muốn 0/1", k.daCommit, k.daRollback)
	}
}

func TestCloseShapeRefusedBeforeTheTransaction(t *testing.T) {
	for ten, tc := range map[string]struct {
		req   BudgetPeriodCloseRequest
		actor audit.Actor
		want  error
	}{
		"tháng 13":        {BudgetPeriodCloseRequest{Year: 2026, Month: 13}, nguoiGhi(), domain.ErrCloseMonthInvalid},
		"năm 1999":        {BudgetPeriodCloseRequest{Year: 1999, Month: 1}, nguoiGhi(), domain.ErrNamNgoaiLich},
		"không người làm": {BudgetPeriodCloseRequest{Year: 2026, Month: 1}, audit.Actor{}, nil},
	} {
		t.Run(ten, func(t *testing.T) {
			k := khoMau()
			uc, ctx := dungUseCaseNganSach(t, k)
			_, err := uc.CloseBudgetPeriod(ctx, tc.req, tc.actor)
			if err == nil || (tc.want != nil && !errors.Is(err, tc.want)) {
				t.Fatalf("= %v, muốn %v", err, tc.want)
			}
			if k.batDau != 0 {
				t.Fatalf("mở %d giao dịch, muốn 0", k.batDau)
			}
		})
	}
}

// --- (1) reopen --------------------------------------------------------------------------------------

func TestReopenFillsOnceAuditedWithTheReason(t *testing.T) {
	k := khoMau()
	k.closes = []domain.BudgetPeriodClose{closeRow("k1", "CK-2026-09-01", 2026, 9, 1)}
	uc, ctx := dungUseCaseNganSach(t, k)

	after, err := uc.ReopenBudgetPeriodClose(ctx, "CK-2026-09-01", "  Ghi sót đợt thu phí chợ  ", nguoiGhi())
	if err != nil {
		t.Fatalf("lỗi: %v", err)
	}
	if after.Active() || after.ReopenedBy != maCanBo || after.ReopenReason != "Ghi sót đợt thu phí chợ" {
		t.Fatalf("sau khi mở chốt = %+v", after)
	}
	upd := k.cau("UPDATE budget_period_closes")
	if len(upd) != 1 || !coGiaTri(upd[0], maCanBo) || !coGiaTri(upd[0], "Ghi sót đợt thu phí chợ") {
		t.Fatalf("câu mở chốt: %v", upd)
	}
	if !strings.Contains(upd[0].sql, "reopened_at IS NULL") {
		t.Fatal("câu mở chốt thiếu `AND reopened_at IS NULL` — lần mở thứ hai sẽ thành lỗi trigger 500")
	}
	if k.thuTuCua("pg_advisory_xact_lock") > k.thuTuCua("UPDATE budget_period_closes") {
		t.Fatal("mở chốt trước khi khoá năm")
	}
	vet := k.cau("INSERT INTO audit_log")
	if len(vet) != 1 || !coGiaTri(vet[0], "CK-2026-09-01") || !coGiaTri(vet[0], ActionBudgetPeriodReopen) ||
		!coChuoiTrongDelta(vet[0], "Ghi sót đợt thu phí chợ") || !coChuoiTrongDelta(vet[0], `"revision":1`) {
		t.Fatalf("vết mở chốt: %v", vet)
	}
	if k.daCommit != 1 {
		t.Fatalf("commit %d", k.daCommit)
	}
}

func TestReopenTwiceIsRefused(t *testing.T) {
	k := khoMau()
	k.closes = []domain.BudgetPeriodClose{reopenedRow(closeRow("k1", "CK-2026-09-01", 2026, 9, 1))}
	uc, ctx := dungUseCaseNganSach(t, k)
	_, err := uc.ReopenBudgetPeriodClose(ctx, "CK-2026-09-01", "lần hai", nguoiGhi())
	if !errors.Is(err, domain.ErrCloseAlreadyReopened) || !strings.Contains(err.Error(), "CK-2026-09-01") {
		t.Fatalf("= %v, muốn ErrCloseAlreadyReopened", err)
	}
	wroteNothing(t, k)
}

func TestReopenUnknownCodeIsNotFound(t *testing.T) {
	k := khoMau()
	uc, ctx := dungUseCaseNganSach(t, k)
	if _, err := uc.ReopenBudgetPeriodClose(ctx, "CK-2099-01-01", "x", nguoiGhi()); !errors.Is(err, domain.ErrBudgetPeriodCloseNotFound) {
		t.Fatalf("= %v", err)
	}
	wroteNothing(t, k)
}

func TestReopenReasonRequiredAndBounded(t *testing.T) {
	ok := strings.Repeat("ệ", domain.ReopenReasonMax) // 500 runes, 1500 bytes — accepted
	for ten, tc := range map[string]struct {
		reason string
		want   error
	}{
		"thiếu":   {"   ", domain.ErrReopenReasonMissing},
		"quá dài": {ok + "ệ", domain.ErrReopenReasonTooLong},
	} {
		t.Run(ten, func(t *testing.T) {
			k := khoMau()
			k.closes = []domain.BudgetPeriodClose{closeRow("k1", "CK-2026-09-01", 2026, 9, 1)}
			uc, ctx := dungUseCaseNganSach(t, k)
			if _, err := uc.ReopenBudgetPeriodClose(ctx, "CK-2026-09-01", tc.reason, nguoiGhi()); !errors.Is(err, tc.want) {
				t.Fatalf("= %v, muốn %v", err, tc.want)
			}
			if k.batDau != 0 {
				t.Fatal("lý do sai mà vẫn mở giao dịch")
			}
		})
	}
	k := khoMau()
	k.closes = []domain.BudgetPeriodClose{closeRow("k1", "CK-2026-09-01", 2026, 9, 1)}
	uc, ctx := dungUseCaseNganSach(t, k)
	if _, err := uc.ReopenBudgetPeriodClose(ctx, "CK-2026-09-01", ok, nguoiGhi()); err != nil {
		t.Fatalf("lý do đúng %d ký tự bị từ chối: %v", domain.ReopenReasonMax, err)
	}
}

// --- (2)(3) entry guards --------------------------------------------------------------------------------

func TestEntryGuards(t *testing.T) {
	aug := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	jan27 := time.Date(2027, 1, 5, 0, 0, 0, 0, time.UTC)
	for ten, tc := range map[string]struct {
		closes []domain.BudgetPeriodClose
		date   time.Time
		locked string // the code the refusal must name; "" = allowed
	}{
		"đóng tháng 8 chặn đợt ngày 20/08": {
			[]domain.BudgetPeriodClose{closeRow("k1", "CK-2026-08-01", 2026, 8, 1)}, aug, "CK-2026-08-01"},
		"đóng tháng 7 không chặn tháng 8": {
			[]domain.BudgetPeriodClose{closeRow("k1", "CK-2026-07-01", 2026, 7, 1)}, aug, ""},
		"đóng tháng 8 đã mở chốt thì không chặn": {
			[]domain.BudgetPeriodClose{reopenedRow(closeRow("k1", "CK-2026-08-01", 2026, 8, 1))}, aug, ""},
		"đóng năm theo năm của ngày": {
			[]domain.BudgetPeriodClose{closeRow("y1", "CK-2027-CN-01", 2027, 0, 1)}, jan27, "CK-2027-CN-01"},
		"đóng năm theo năm của bảng": {
			[]domain.BudgetPeriodClose{closeRow("y1", "CK-2026-CN-01", 2026, 0, 1)}, jan27, "CK-2026-CN-01"},
		"năm khác không chặn": {
			[]domain.BudgetPeriodClose{closeRow("y1", "CK-2025-CN-01", 2025, 0, 1)}, aug, ""},
	} {
		t.Run(ten, func(t *testing.T) {
			// ADD
			k := khoMau()
			k.closes = tc.closes
			uc, ctx := dungUseCaseNganSach(t, k)
			yc := yeuCauDotMau()
			yc.Ngay = tc.date
			_, err := uc.GhiDot(ctx, yc, nguoiGhi())
			checkGuard(t, "ghi đợt", k, err, tc.locked, "INSERT INTO dot_thu_chi")

			// REMOVE — the same answer on the batch's own date.
			k = khoMau()
			k.closes = tc.closes
			k.dot = &domain.DotThuChi{ID: "01JDOT0000000000000000000A", KhoanMucID: idDongKia, Ngay: tc.date,
				NoiDung: "Thu phí chợ", NguoiGhiMa: maCanBo, GiaTri: map[string]domain.Dong{"c-chi": 10}}
			uc, ctx = dungUseCaseNganSach(t, k)
			err = uc.GoDot(ctx, k.dot.ID, "ghi nhầm", nguoiGhi())
			checkGuard(t, "gỡ đợt", k, err, tc.locked, "UPDATE dot_thu_chi")
		})
	}
}

func checkGuard(t *testing.T, act string, k *khoNSGia, err error, locked, write string) {
	t.Helper()
	if locked == "" {
		if err != nil {
			t.Fatalf("%s: kỳ còn mở mà bị từ chối: %v", act, err)
		}
		if !k.coCau(write) {
			t.Fatalf("%s: kỳ còn mở mà không ghi", act)
		}
		return
	}
	if !errors.Is(err, domain.ErrPeriodClosed) || !strings.Contains(err.Error(), locked) {
		t.Fatalf("%s: = %v, muốn ErrPeriodClosed nêu %s", act, err, locked)
	}
	if k.coCau(write) || k.coCau("INSERT INTO audit_log") || k.daCommit != 0 {
		t.Fatalf("%s: kỳ đã chốt mà vẫn ghi", act)
	}
	// The guard runs AFTER the lock.
	if k.thuTuCua("pg_advisory_xact_lock") > k.thuTuCua("FROM budget_period_closes") {
		t.Fatalf("%s: đọc chốt kỳ trước khi khoá năm", act)
	}
}

func TestEntryAcrossTwoYearsLocksBothAscending(t *testing.T) {
	// Sheet 2026, entry dated 2027: both years are locked, 2026 first — the order every writer uses,
	// so two writers needing the same two years queue instead of deadlocking.
	k := khoMau()
	uc, ctx := dungUseCaseNganSach(t, k)
	yc := yeuCauDotMau()
	yc.Ngay = time.Date(2027, 1, 5, 0, 0, 0, 0, time.UTC)
	if _, err := uc.GhiDot(ctx, yc, nguoiGhi()); err != nil {
		t.Fatalf("lỗi: %v", err)
	}
	locks := k.cau("pg_advisory_xact_lock")
	if len(locks) != 2 || !coGiaTri(locks[0], int64(2026)) || !coGiaTri(locks[1], int64(2027)) {
		t.Fatalf("khoá = %v, muốn 2026 rồi 2027", locks)
	}
	// The sheet's row lock comes first — the lock order stated in store.LockBudgetYears.
	if k.thuTuCua("FOR UPDATE") > k.thuTuCua("pg_advisory_xact_lock") {
		t.Fatal("khoá năm trước khi khoá bảng")
	}
}

// --- (5) adjustment entry -------------------------------------------------------------------------------

func TestAdjustmentEntryStoresAndAuditsItsReason(t *testing.T) {
	k := khoMau()
	uc, ctx := dungUseCaseNganSach(t, k)
	yc := yeuCauDotMau()
	reason := "  Bù đợt ghi sót tháng 9  "
	yc.AdjustmentReason = &reason

	moi, err := uc.GhiDot(ctx, yc, nguoiGhi())
	if err != nil {
		t.Fatalf("lỗi: %v", err)
	}
	if moi.AdjustmentReason != "Bù đợt ghi sót tháng 9" {
		t.Fatalf("lý do điều chỉnh = %q", moi.AdjustmentReason)
	}
	if !coGiaTri(k.cau("INSERT INTO dot_thu_chi")[0], "Bù đợt ghi sót tháng 9") {
		t.Fatal("câu chèn đợt thiếu adjustment_reason")
	}
	vet := k.cau("INSERT INTO audit_log")[0]
	if !coChuoiTrongDelta(vet, `"is_adjustment":true`) || !coChuoiTrongDelta(vet, "Bù đợt ghi sót tháng 9") {
		t.Fatal("vết đợt điều chỉnh thiếu cờ / lý do")
	}
	if coChuoiTrongDelta(vet, donViMau) {
		t.Fatal("delta chứa nguyên văn `đơn vị, cá nhân`")
	}
}

func TestOrdinaryEntryHasNullAdjustmentReason(t *testing.T) {
	k := khoMau()
	uc, ctx := dungUseCaseNganSach(t, k)
	if _, err := uc.GhiDot(ctx, yeuCauDotMau(), nguoiGhi()); err != nil {
		t.Fatalf("lỗi: %v", err)
	}
	ins := k.cau("INSERT INTO dot_thu_chi")[0]
	if ins.args[len(ins.args)-1] != nil {
		t.Fatalf("đợt thường phải chèn adjustment_reason NULL, được %v", ins.args[len(ins.args)-1])
	}
	if !coChuoiTrongDelta(k.cau("INSERT INTO audit_log")[0], `"is_adjustment":false`) {
		t.Fatal("vết đợt thường phải ghi is_adjustment false")
	}
}

func TestAdjustmentReasonBlankOrTooLongRefusedBeforeTheTransaction(t *testing.T) {
	for ten, tc := range map[string]struct {
		reason string
		want   error
	}{
		"trắng":   {"   ", domain.ErrAdjustmentReasonBlank},
		"quá dài": {strings.Repeat("ệ", domain.AdjustmentReasonMax+1), domain.ErrAdjustmentReasonTooLong},
	} {
		t.Run(ten, func(t *testing.T) {
			k := khoMau()
			uc, ctx := dungUseCaseNganSach(t, k)
			yc := yeuCauDotMau()
			r := tc.reason
			yc.AdjustmentReason = &r
			if _, err := uc.GhiDot(ctx, yc, nguoiGhi()); !errors.Is(err, tc.want) {
				t.Fatalf("= %v, muốn %v", err, tc.want)
			}
			if k.batDau != 0 {
				t.Fatal("mở giao dịch cho lý do sai")
			}
		})
	}
}

// --- (4) sheet-year guards ------------------------------------------------------------------------------

func sheetWrites() map[string]func(uc *NganSach, ctx context.Context, k *khoNSGia) error {
	title := "BÁO CÁO CHI (đã sửa)"
	return map[string]func(uc *NganSach, ctx context.Context, k *khoNSGia) error{
		"TaoBang": func(uc *NganSach, ctx context.Context, k *khoNSGia) error {
			k.lanKeTiep = 1
			_, err := uc.TaoBang(ctx, YeuCauTaoBang{
				Nam: 2026, Loai: domain.BangChi, TieuDe: "X", DonViTinh: "trieu-dong",
				Cot: []NewColumn{{CotNganSach: domain.CotNganSach{Ten: "Chi ngân sách", ThuTu: 1, Kieu: domain.CotSo}}},
			}, nguoiGhi())
			return err
		},
		"SuaBang": func(uc *NganSach, ctx context.Context, k *khoNSGia) error {
			_, err := uc.SuaBang(ctx, idBangMau, YeuCauSuaBang{TieuDe: &title}, nguoiGhi())
			return err
		},
		"GoBang": func(uc *NganSach, ctx context.Context, k *khoNSGia) error {
			return uc.GoBang(ctx, idBangMau, "nạp lại", nguoiGhi())
		},
		"ThemKhoanMuc": func(uc *NganSach, ctx context.Context, k *khoNSGia) error {
			_, err := uc.ThemKhoanMuc(ctx, YeuCauThemKhoanMuc{BangID: idBangMau, Ten: "Chi mới", ThuTu: 5}, nguoiGhi())
			return err
		},
		"SuaKhoanMuc (số gõ tay)": func(uc *NganSach, ctx context.Context, k *khoNSGia) error {
			_, err := uc.SuaKhoanMuc(ctx, idDongKia,
				YeuCauSuaKhoanMuc{GiaTri: map[string]*domain.Dong{"c-chi": dong(7)}}, nguoiGhi())
			return err
		},
		"GoKhoanMuc": func(uc *NganSach, ctx context.Context, k *khoNSGia) error {
			return uc.GoKhoanMuc(ctx, idDongKia, "nhập trùng", nguoiGhi())
		},
		"DatDongTong": func(uc *NganSach, ctx context.Context, k *khoNSGia) error {
			_, err := uc.DatDongTong(ctx, idDongKia, nguoiGhi())
			return err
		},
	}
}

func TestYearCloseLocksEverySheetWrite(t *testing.T) {
	for ten, write := range sheetWrites() {
		t.Run(ten, func(t *testing.T) {
			k := khoMau()
			k.closes = []domain.BudgetPeriodClose{closeRow("y1", "CK-2026-CN-01", 2026, 0, 1)}
			uc, ctx := dungUseCaseNganSach(t, k)
			err := write(uc, ctx, k)
			if !errors.Is(err, domain.ErrPeriodClosed) || !strings.Contains(err.Error(), "CK-2026-CN-01") {
				t.Fatalf("= %v, muốn ErrPeriodClosed nêu CK-2026-CN-01", err)
			}
			wroteNothing(t, k)
		})
	}
}

func TestMonthCloseDoesNotLockSheetWrites(t *testing.T) {
	// Hand-entered values carry no date, so a MONTH close cannot lock them (main-session decision).
	for ten, write := range sheetWrites() {
		t.Run(ten, func(t *testing.T) {
			k := khoMau()
			k.closes = []domain.BudgetPeriodClose{closeRow("k1", "CK-2026-08-01", 2026, 8, 1),
				reopenedRow(closeRow("y0", "CK-2026-CN-01", 2026, 0, 1))}
			uc, ctx := dungUseCaseNganSach(t, k)
			if err := write(uc, ctx, k); err != nil {
				t.Fatalf("chốt tháng / chốt năm đã mở mà vẫn chặn: %v", err)
			}
			if !k.coCau("pg_advisory_xact_lock") {
				t.Fatal("ghi bảng mà không khoá năm")
			}
		})
	}
}
