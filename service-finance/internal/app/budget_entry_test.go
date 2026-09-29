package app

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/service-finance/internal/domain"
)

// WHAT THIS FILE IS FOR: the batches (migration 0008) and the `entries` calculation mode, through the
// REAL store on the fake driver (fake_driver_budget_test.go says what that does and does not prove).
//
// THE PROPERTIES, each a defect that fails silently:
//
//  1. a batch, its amounts and its audit entry are ONE transaction; a failed entry takes all down;
//  2. `don_vi_ca_nhan` (may be a citizen's name) NEVER enters the audit delta — only presence/length;
//  3. every refusal — non-leaf, % column, foreign column, over-long text — writes nothing;
//  4. writing a batch does NOT switch the mode;
//  5. every batch sum joins `dot_thu_chi.deleted_at IS NULL`;
//  6. entries -> manual copies the sums into the cells (NULL where empty), same transaction, audited;
//     manual -> entries leaves the typed cells alone;
//  7. a line in `entries` mode with live batches cannot gain a child; without batches it flips.

// sampleCounterparty is a counterparty that looks like what a commune really types. The assertion is that
// these exact characters never reach the audit ledger.
const sampleCounterparty = "Hộ ông Nguyễn Văn Mẫu"

func entryDate() time.Time { return time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC) }

func dongPtr(g int64) *domain.Dong { d := domain.Dong(g); return &d }

func sampleEntryRequest() RecordBudgetEntryRequest {
	return RecordBudgetEntryRequest{
		LineID: otherLineID, Date: entryDate(), Content: "Thu tiền sử dụng đất đợt 2",
		Counterparty: sampleCounterparty, VoucherNo: "PT-0042",
		Amounts: map[string]*domain.Dong{"c-chi": dongPtr(1_500_000), "c-dt": nil},
	}
}

// --- (1)(2)(4) the write ------------------------------------------------------------------------------

func TestRecordEntryOneTransactionAndAuditOmitsCounterparty(t *testing.T) {
	k := sampleStore()
	uc, ctx := newBudgetUseCase(t, k)

	next, err := uc.RecordEntry(ctx, sampleEntryRequest(), writer())
	if err != nil {
		t.Fatalf("GhiDot lỗi: %v", err)
	}
	if k.begins != 1 || k.commits != 1 || k.rollbacks != 0 {
		t.Fatalf("giao dịch: mở %d commit %d rollback %d, muốn 1/1/0", k.begins, k.commits, k.rollbacks)
	}
	insert := k.stmtsContaining("INSERT INTO dot_thu_chi")
	if len(insert) != 1 {
		t.Fatalf("có %d câu chèn đợt, muốn 1", len(insert))
	}
	// nguoi_ghi_ma is the STAFF CODE (rule 6, invariant 8), and the counterparty goes in as a bound
	// parameter — the one place it may live.
	if !hasArg(insert[0], staffCode) || !hasArg(insert[0], sampleCounterparty) {
		t.Fatalf("câu chèn đợt thiếu mã cán bộ hoặc đơn vị: %v", insert[0].args)
	}
	// A nil amount writes NO row: "a column with no row is empty too" (0008).
	amountStmts := k.stmtsContaining("INSERT INTO gia_tri_dot")
	if len(amountStmts) != 1 || !hasArg(amountStmts[0], int64(1_500_000)) || !hasArg(amountStmts[0], "c-chi") {
		t.Fatalf("số tiền đợt ghi sai: %v", amountStmts)
	}
	if next.Amounts["c-chi"] != 1_500_000 || len(next.Amounts) != 1 {
		t.Fatalf("đợt trả về mang số tiền %v", next.Amounts)
	}

	audit := k.stmtsContaining("INSERT INTO audit_log")
	if len(audit) != 1 {
		t.Fatalf("có %d vết, muốn 1", len(audit))
	}
	if k.stmtIndex("INSERT INTO gia_tri_dot") > k.stmtIndex("INSERT INTO audit_log") {
		t.Fatal("vết ghi trước số tiền đợt")
	}
	if !hasArg(audit[0], "NS-2026-CHI-01") || !hasArg(audit[0], staffCode) || !hasArg(audit[0], ActionRecordBudgetEntry) {
		t.Fatalf("vết không mang mã bảng / mã cán bộ / hành vi: %v", audit[0].args)
	}
	// RULE 6 FORBIDDEN #4 / RULE 3: the ledger records THAT a counterparty was stated, never what.
	if deltaContains(audit[0], sampleCounterparty) || deltaContains(audit[0], "Nguyễn Văn") {
		t.Fatal("delta vết chứa nguyên văn `đơn vị, cá nhân` — sổ kiểm toán thành kho dữ liệu cá nhân")
	}
	for _, want := range []string{`"co_don_vi_ca_nhan":true`, `"do_dai_don_vi_ca_nhan":21`, "PT-0042", "2026-08-20"} {
		if !deltaContains(audit[0], want) {
			t.Errorf("delta thiếu %q", want)
		}
	}
	// (4) NO AUTO-SWITCH: the user chooses the mode (§4.2, decision 25/09/2026).
	if k.hasStmt("SET cach_tinh = $3") {
		t.Fatal("ghi đợt mà tự đổi cách tính của khoản mục")
	}
}

func TestFailedAuditLeavesNoEntryAndNoAmounts(t *testing.T) {
	k := sampleStore()
	k.failOnSQL = "INSERT INTO audit_log"
	uc, ctx := newBudgetUseCase(t, k)

	if _, err := uc.RecordEntry(ctx, sampleEntryRequest(), writer()); err == nil {
		t.Fatal("vết hỏng mà GhiDot vẫn báo thành công")
	}
	if k.commits != 0 || k.rollbacks != 1 {
		t.Fatalf("giao dịch: commit %d rollback %d, muốn 0/1", k.commits, k.rollbacks)
	}
}

// --- (3) refusals write nothing ----------------------------------------------------------------------

func TestRecordEntryOnLineWithChildrenIsRefused(t *testing.T) {
	k := sampleStore()
	k.lines = append(k.lines, domain.BudgetLine{
		ID: "k-con", SheetID: sampleSheetID, ParentID: otherLineID, Name: "Chi đầu tư", SortOrder: 3, Level: 1,
	})
	uc, ctx := newBudgetUseCase(t, k)

	_, err := uc.RecordEntry(ctx, sampleEntryRequest(), writer())
	if !errors.Is(err, domain.ErrEntryLeafOnly) {
		t.Fatalf("= %v, muốn ErrDotChiGhiVaoLa", err)
	}
	if k.hasStmt("INSERT INTO dot_thu_chi") || k.hasStmt("INSERT INTO audit_log") || k.commits != 0 {
		t.Fatal("bị từ chối mà vẫn ghi")
	}
}

func TestRecordEntryIntoPercentOrOtherSheetColumnIsRefused(t *testing.T) {
	for name, tc := range map[string]struct {
		column string
		want   error
	}{
		"cột phần trăm": {"c-ty", domain.ErrColumnNotNumber},
		"cột bảng khác": {"c-cua-bang-khac", domain.ErrColumnInOtherSheet},
	} {
		t.Run(name, func(t *testing.T) {
			k := sampleStore()
			uc, ctx := newBudgetUseCase(t, k)
			req := sampleEntryRequest()
			req.Amounts[tc.column] = dongPtr(10)
			if _, err := uc.RecordEntry(ctx, req, writer()); !errors.Is(err, tc.want) {
				t.Fatalf("= %v, muốn %v", err, tc.want)
			}
			if k.hasStmt("INSERT INTO dot_thu_chi") || k.hasStmt("INSERT INTO gia_tri_dot") {
				t.Fatal("bị từ chối mà vẫn chèn")
			}
		})
	}
}

func TestRecordEntryTextLimitsCountRunesAndOpenNoTransaction(t *testing.T) {
	// `ệ` is THREE bytes. 1000 of them is 3000 bytes and must be ACCEPTED — a byte count would refuse
	// it, which is the bug a Vietnamese commune meets first.
	fits := strings.Repeat("ệ", domain.VoucherDescriptionMax)
	k := sampleStore()
	uc, ctx := newBudgetUseCase(t, k)
	req := sampleEntryRequest()
	req.Content = fits
	if _, err := uc.RecordEntry(ctx, req, writer()); err != nil {
		t.Fatalf("nội dung đúng %d ký tự (3 byte mỗi ký tự) bị từ chối: %v", domain.VoucherDescriptionMax, err)
	}

	for name, tc := range map[string]struct {
		edit func(*RecordBudgetEntryRequest)
		want error
	}{
		"content quá dài":  {func(y *RecordBudgetEntryRequest) { y.Content = fits + "ệ" }, domain.ErrEntryContentTooLong},
		"content rỗng":     {func(y *RecordBudgetEntryRequest) { y.Content = "   " }, domain.ErrEntryContentMissing},
		"counterparty dài": {func(y *RecordBudgetEntryRequest) { y.Counterparty = strings.Repeat("ễ", domain.CounterpartyMax+1) }, domain.ErrEntryCounterpartyTooLong},
		"document_no dài":  {func(y *RecordBudgetEntryRequest) { y.VoucherNo = strings.Repeat("ố", domain.VoucherNoMax+1) }, domain.ErrEntryVoucherNoTooLong},
		"thiếu ngày":       {func(y *RecordBudgetEntryRequest) { y.Date = time.Time{} }, domain.ErrEntryDateMissing},
		"năm 1999":         {func(y *RecordBudgetEntryRequest) { y.Date = time.Date(1999, 12, 31, 0, 0, 0, 0, time.UTC) }, domain.ErrEntryDateOutOfRange},
		"không có số nào":  {func(y *RecordBudgetEntryRequest) { y.Amounts = map[string]*domain.Dong{"c-chi": nil} }, domain.ErrEntryHasNoAmount},
		"số vượt chặn gõ nhầm": {func(y *RecordBudgetEntryRequest) {
			y.Amounts = map[string]*domain.Dong{"c-chi": dongPtr(int64(domain.ValueMax) + 1)}
		}, domain.ErrValueTooLarge},
	} {
		t.Run(name, func(t *testing.T) {
			k := sampleStore()
			uc, ctx := newBudgetUseCase(t, k)
			req := sampleEntryRequest()
			tc.edit(&req)
			_, err := uc.RecordEntry(ctx, req, writer())
			if !errors.Is(err, tc.want) {
				t.Fatalf("= %v, muốn %v", err, tc.want)
			}
			if k.begins != 0 {
				t.Fatalf("mở %d giao dịch cho yêu cầu sai hình dạng, muốn 0", k.begins)
			}
			// The refusal names the field and the limit — never the value (rule 3, forbidden #3).
			if strings.Contains(err.Error(), sampleCounterparty) {
				t.Fatal("câu lỗi chứa nguyên văn `đơn vị, cá nhân`")
			}
		})
	}
}

// --- removal --------------------------------------------------------------------------------------------

func liveEntry() *domain.BudgetEntry {
	return &domain.BudgetEntry{
		ID: "01JDOTTHUCHIMAU00000000000", LineID: otherLineID, Date: entryDate(),
		Content: "Thu tiền sử dụng đất đợt 2", Counterparty: sampleCounterparty, VoucherNo: "PT-0042",
		EnteredBy: "CB-00007", CreatedAt: time.Date(2026, 8, 20, 9, 0, 0, 0, time.UTC),
		Amounts: map[string]domain.Dong{"c-chi": 1_500_000},
	}
}

func TestRemoveEntryWritesAllThreeColumnsAndAuditOmitsCounterparty(t *testing.T) {
	k := sampleStore()
	k.entry = liveEntry()
	uc, ctx := newBudgetUseCase(t, k)

	if err := uc.RemoveEntry(ctx, k.entry.ID, "ghi nhầm số chứng từ", writer()); err != nil {
		t.Fatalf("GoDot lỗi: %v", err)
	}
	del := k.stmtsContaining("UPDATE dot_thu_chi")
	if len(del) != 1 {
		t.Fatalf("có %d câu xoá mềm đợt, muốn 1", len(del))
	}
	for _, frag := range []string{"deleted_at = now()", "deleted_by = $3", "delete_reason = $4",
		"deleted_at IS NULL", "tenant_id = $1"} {
		if !strings.Contains(del[0].sql, frag) {
			t.Errorf("câu xoá mềm đợt thiếu %q: %s", frag, del[0].sql)
		}
	}
	if !hasArg(del[0], staffCode) {
		t.Fatalf("deleted_by không phải mã cán bộ: %v", del[0].args)
	}
	if k.hasStmt("DELETE ") || k.hasStmt("UPDATE gia_tri_dot") {
		t.Fatal("gỡ đợt mà đụng tới số tiền hoặc xoá cứng")
	}
	audit := k.stmtsContaining("INSERT INTO audit_log")
	if len(audit) != 1 || !hasArg(audit[0], ActionRemoveBudgetEntry) || !hasArg(audit[0], "NS-2026-CHI-01") {
		t.Fatalf("vết gỡ đợt sai: %v", audit)
	}
	if deltaContains(audit[0], sampleCounterparty) {
		t.Fatal("delta vết gỡ chứa nguyên văn `đơn vị, cá nhân`")
	}
	if !deltaContains(audit[0], "ghi nhầm số chứng từ") || !deltaContains(audit[0], "1500000") {
		t.Fatal("vết gỡ thiếu lý do hoặc số tiền trước khi gỡ")
	}
	if k.commits != 1 {
		t.Fatalf("commit %d, muốn 1", k.commits)
	}
}

func TestRemoveEntryMissingOrLineRemovedWritesNothing(t *testing.T) {
	for name, edit := range map[string]func(*fakeBudgetStore){
		"không có đợt sống": func(k *fakeBudgetStore) { k.entry = nil },
		"khoản mục đã gỡ":   func(k *fakeBudgetStore) { k.entry = liveEntry(); k.sheetIDOfLine = "" },
	} {
		t.Run(name, func(t *testing.T) {
			k := sampleStore()
			edit(k)
			uc, ctx := newBudgetUseCase(t, k)
			err := uc.RemoveEntry(ctx, "01JDOTTHUCHIMAU00000000000", "x", writer())
			if !errors.Is(err, domain.ErrEntryNotFound) {
				t.Fatalf("= %v, muốn ErrKhongThayDot", err)
			}
			if k.hasStmt("UPDATE dot_thu_chi") || k.hasStmt("INSERT INTO audit_log") {
				t.Fatal("không có đợt mà vẫn ghi")
			}
		})
	}
}

// --- (5) the sum counts live batches only -----------------------------------------------------------------

func TestEntryTotalSumsOnlyLiveEntries(t *testing.T) {
	// The fake cannot evaluate SQL, so the property is asserted where it lives: the statement text.
	// 0008 question 4a: "a sum that forgets the join counts removed batches".
	k := sampleStore()
	uc, ctx := newBudgetUseCase(t, k)
	if _, err := uc.RecordEntry(ctx, sampleEntryRequest(), writer()); err != nil {
		t.Fatalf("GhiDot lỗi: %v", err)
	}
	total := k.stmtsContaining("SUM(g.gia_tri)")
	if len(total) != 1 {
		t.Fatalf("có %d câu tổng đợt, muốn 1 (đọc dưới khoá bảng)", len(total))
	}
	for _, frag := range []string{"d.deleted_at IS NULL", "k.deleted_at IS NULL", "g.gia_tri IS NOT NULL",
		"g.tenant_id = $1", "d.tenant_id = g.tenant_id", "k.tenant_id = d.tenant_id", "GROUP BY"} {
		if !strings.Contains(total[0].sql, frag) {
			t.Errorf("câu tổng đợt thiếu %q", frag)
		}
	}
}

// --- (6) switching the mode ---------------------------------------------------------------------------------

func method(c domain.LineMethod) *domain.LineMethod { return &c }

func TestSwitchEntriesToManualCopiesTotalsIntoCellsInSameTransaction(t *testing.T) {
	k := sampleStore()
	k.lines[1].Method = domain.MethodEntries
	// An OLD typed figure in `c-dt`, from before the entries period, and a batch sum in `c-chi` only.
	k.value[otherLineID] = map[string]domain.Dong{"c-dt": 999}
	k.entryTotals = map[string]map[string]domain.Dong{otherLineID: {"c-chi": 7_000}}
	uc, ctx := newBudgetUseCase(t, k)

	after, err := uc.UpdateLine(ctx, otherLineID, UpdateBudgetLineRequest{Method: method(domain.MethodManual)}, writer())
	if err != nil {
		t.Fatalf("SuaKhoanMuc lỗi: %v", err)
	}
	if after.Method != domain.MethodManual {
		t.Fatalf("cách tính trả về %q", after.Method)
	}
	write := k.stmtsContaining("INSERT INTO gia_tri_khoan_muc")
	if len(write) != 2 {
		t.Fatalf("có %d câu ghi ô, muốn 2 (c-dt về NULL, c-chi nhận tổng)", len(write))
	}
	var hasTotal, hasNull bool
	for _, g := range write {
		if hasArg(g, "c-chi") && hasArg(g, int64(7_000)) {
			hasTotal = true
		}
		if hasArg(g, "c-dt") && g.args[3] == nil {
			hasNull = true
		}
	}
	if !hasTotal {
		t.Fatal("ô c-chi không nhận tổng các đợt (§9.1)")
	}
	// The screen showed `—` in c-dt; the old typed 999 must NOT come back as the starting value.
	if !hasNull {
		t.Fatal("ô c-dt giữ số gõ tay cũ thay vì trống như màn hình vừa hiện")
	}
	set := k.stmtsContaining("SET cach_tinh = $3")
	if len(set) != 1 || !hasArg(set[0], string(domain.MethodManual)) {
		t.Fatalf("câu đặt cách tính sai: %v", set)
	}
	if k.begins != 1 || k.commits != 1 {
		t.Fatalf("giao dịch: mở %d commit %d, muốn 1/1", k.begins, k.commits)
	}
	audit := k.stmtsContaining("INSERT INTO audit_log")
	if len(audit) != 1 {
		t.Fatalf("có %d vết, muốn 1", len(audit))
	}
	for _, want := range []string{`"cach_tinh"`, `"entries"`, `"manual"`, `"o_lay_tu_tong_dot"`, "7000", "999"} {
		if !deltaContains(audit[0], want) {
			t.Errorf("delta thiếu %q", want)
		}
	}
}

func TestSwitchManualToEntriesKeepsTypedCells(t *testing.T) {
	k := sampleStore()
	uc, ctx := newBudgetUseCase(t, k)

	if _, err := uc.UpdateLine(ctx, sampleLineID, UpdateBudgetLineRequest{Method: method(domain.MethodEntries)}, writer()); err != nil {
		t.Fatalf("SuaKhoanMuc lỗi: %v", err)
	}
	if k.hasStmt("INSERT INTO gia_tri_khoan_muc") {
		t.Fatal("manual -> entries mà ghi đè ô gõ tay — ô phải giữ nguyên, chỉ thôi hiển thị")
	}
	set := k.stmtsContaining("SET cach_tinh = $3")
	if len(set) != 1 || !hasArg(set[0], string(domain.MethodEntries)) {
		t.Fatalf("câu đặt cách tính sai: %v", set)
	}
	if !deltaContains(k.stmtsContaining("INSERT INTO audit_log")[0], `"entries"`) {
		t.Fatal("vết không ghi cách tính mới")
	}
}

func TestRefusedMethodChangeWritesNothing(t *testing.T) {
	for name, tc := range map[string]struct {
		edit func(*fakeBudgetStore)
		id   string
		req  UpdateBudgetLineRequest
		want error
	}{
		"children từ client": {func(*fakeBudgetStore) {}, sampleLineID,
			UpdateBudgetLineRequest{Method: method(domain.MethodChildren)}, domain.ErrMethodFromClient},
		"khoản mục có con": {func(k *fakeBudgetStore) {
			k.lines[1].Method = domain.MethodChildren
			k.lines = append(k.lines, domain.BudgetLine{
				ID: "k-con", SheetID: sampleSheetID, ParentID: otherLineID, Name: "Chi đầu tư", SortOrder: 3, Level: 1})
		}, otherLineID, UpdateBudgetLineRequest{Method: method(domain.MethodEntries)}, domain.ErrParentLineMethodFixed},
		"gõ số vào khoản mục theo đợt": {func(k *fakeBudgetStore) { k.lines[1].Method = domain.MethodEntries },
			otherLineID, UpdateBudgetLineRequest{Values: map[string]*domain.Dong{"c-chi": dongPtr(1)}},
			domain.ErrEntriesLineNoDirectValue},
		"đổi sang theo đợt kèm gõ số": {func(*fakeBudgetStore) {}, sampleLineID,
			UpdateBudgetLineRequest{Method: method(domain.MethodEntries), Values: map[string]*domain.Dong{"c-chi": dongPtr(1)}},
			domain.ErrEntriesLineNoDirectValue},
	} {
		t.Run(name, func(t *testing.T) {
			k := sampleStore()
			tc.edit(k)
			uc, ctx := newBudgetUseCase(t, k)
			if _, err := uc.UpdateLine(ctx, tc.id, tc.req, writer()); !errors.Is(err, tc.want) {
				t.Fatalf("= %v, muốn %v", err, tc.want)
			}
			if k.hasStmt("SET cach_tinh = $3") || k.hasStmt("INSERT INTO gia_tri_khoan_muc") ||
				k.hasStmt("INSERT INTO audit_log") {
				t.Fatal("bị từ chối mà vẫn ghi")
			}
		})
	}
}

func TestSwitchToManualThenTypeInSameRequest(t *testing.T) {
	// "Switch to manual and type a figure" in one PATCH: the hand-over happens first, then the typed
	// figure overrides the copied sum — and it is compared against the copied sum, not the old cell.
	k := sampleStore()
	k.lines[1].Method = domain.MethodEntries
	k.entryTotals = map[string]map[string]domain.Dong{otherLineID: {"c-chi": 7_000}}
	uc, ctx := newBudgetUseCase(t, k)

	if _, err := uc.UpdateLine(ctx, otherLineID, UpdateBudgetLineRequest{
		Method: method(domain.MethodManual), Values: map[string]*domain.Dong{"c-chi": dongPtr(8_000)},
	}, writer()); err != nil {
		t.Fatalf("SuaKhoanMuc lỗi: %v", err)
	}
	write := k.stmtsContaining("INSERT INTO gia_tri_khoan_muc")
	if len(write) != 2 || !hasArg(write[0], int64(7_000)) || !hasArg(write[1], int64(8_000)) {
		t.Fatalf("thứ tự ghi ô sai — muốn tổng đợt rồi số gõ: %v", write)
	}
}

// --- (7) adding a child under an `entries` line ----------------------------------------------------------------

func TestAddChildUnderEntriesLineWithEntriesIsRefused(t *testing.T) {
	k := sampleStore()
	k.lines[1].Method = domain.MethodEntries
	k.hasLiveEntries = true
	uc, ctx := newBudgetUseCase(t, k)

	_, err := uc.CreateLine(ctx, CreateBudgetLineRequest{
		SheetID: sampleSheetID, ParentID: otherLineID, OrdinalLabel: "1", Name: "Chi con", SortOrder: 3,
	}, writer())
	if !errors.Is(err, domain.ErrEntriesLineHasEntries) {
		t.Fatalf("= %v, muốn ErrKhoanMucTheoDotConDot", err)
	}
	if k.hasStmt("INSERT INTO khoan_muc_ngan_sach") || k.hasStmt("SET cach_tinh = $3") {
		t.Fatal("bị từ chối mà vẫn chèn / đổi cách tính")
	}
	// The refusal tells the user the two ways out.
	if !strings.Contains(err.Error(), "gỡ các đợt") || !strings.Contains(err.Error(), "manual") {
		t.Fatalf("câu từ chối không nói cách gỡ: %q", err.Error())
	}
	// The existence check is tenant-bound and counts LIVE batches only.
	check := k.stmtsContaining("SELECT EXISTS (SELECT 1 FROM dot_thu_chi")
	if len(check) != 1 || !strings.Contains(check[0].sql, "deleted_at IS NULL") ||
		!strings.Contains(check[0].sql, "tenant_id = $1") {
		t.Fatalf("câu kiểm đợt còn sống sai: %v", check)
	}
}

func TestAddChildUnderEntriesLineWithoutEntriesBecomesChildren(t *testing.T) {
	k := sampleStore()
	k.lines[1].Method = domain.MethodEntries
	uc, ctx := newBudgetUseCase(t, k)

	if _, err := uc.CreateLine(ctx, CreateBudgetLineRequest{
		SheetID: sampleSheetID, ParentID: otherLineID, OrdinalLabel: "1", Name: "Chi con", SortOrder: 3,
	}, writer()); err != nil {
		t.Fatalf("ThemKhoanMuc lỗi: %v", err)
	}
	set := k.stmtsContaining("SET cach_tinh = $3")
	if len(set) != 1 || !hasArg(set[0], string(domain.MethodChildren)) {
		t.Fatalf("cha không chuyển sang `children`: %v", set)
	}
	if !deltaContains(k.stmtsContaining("INSERT INTO audit_log")[0], `"cach_tinh_truoc":"entries"`) {
		t.Fatal("vết không ghi cha trước đó là `entries`")
	}
}
