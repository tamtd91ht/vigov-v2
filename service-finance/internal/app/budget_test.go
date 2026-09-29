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

// WHAT THIS FILE IS FOR: the SQL and the transaction boundaries of the budget board, asserted
// through the REAL store on a fake `database/sql` driver — see fake_driver_budget_test.go for why
// this shape and what it still does not prove.
//
// THE PROPERTIES, and each is a defect that fails silently:
//
//  1. the audit entry is in the SAME transaction as the business write (rule 6, invariant 3), and a
//     failure on the entry takes the write down with it;
//  2. a REFUSAL writes nothing at all — no UPDATE, no INSERT, no entry;
//  3. marking a row as the total CLEARS every other row of the sheet, in the same transaction;
//  4. removing a line writes all three soft-delete columns AND releases the star;
//  5. clearing a cell writes NULL and never a DELETE;
//  6. `cach_tinh` is never written from anything a request could reach — it follows the tree;
//  7. the actor written into `deleted_by` is the STAFF CODE, never a ULID (rule 6, invariant 8).

const (
	sampleSheetID = "01JBANGCHIMAU0000000000000"
	sampleLineID  = "01JKHOANMUCMAU000000000000"
	otherLineID   = "01JKHOANMUCKIA000000000000"
	staffCode     = "CB-00123"
)

func writer() audit.Actor { return audit.Actor{ID: staffCode, Kind: "staff", IP: "10.0.0.7"} }

// sampleStore is a chi sheet with two top-level lines, the first of them MARKED. It is the shape §10
// describes for the chi tab — `Tổng số` beside `A. CHI NGÂN SÁCH NHÀ NƯỚC` — which is the shape that
// makes "take the first row" and "sum the roots" both wrong.
func sampleStore() *fakeBudgetStore {
	return &fakeBudgetStore{
		sheet: &sheetRow{id: sampleSheetID, code: "NS-2026-CHI-01", year: 2026, kind: "chi", revision: 1,
			title: "BÁO CÁO CHI NGÂN SÁCH NHÀ NƯỚC XÃ THĂNG BÌNH NĂM 2026", unit: "Triệu đồng"},
		column: []domain.BudgetColumn{
			{ID: "c-dt", SheetID: sampleSheetID, Name: "Dự toán năm", SortOrder: 1,
				Format: domain.ColumnFormatNumber, Indicator: domain.IndicatorAnnualEstimate},
			{ID: "c-chi", SheetID: sampleSheetID, Name: "Chi ngân sách", SortOrder: 2,
				Format: domain.ColumnFormatNumber, Indicator: domain.IndicatorBudgetExpenditure},
			{ID: "c-ty", SheetID: sampleSheetID, Name: "So sánh TH/DT (%)", SortOrder: 3,
				Format: domain.ColumnFormatPercent, Formula: "col_2 / col_1 * 100"},
		},
		lines: []domain.BudgetLine{
			{ID: sampleLineID, SheetID: sampleSheetID, Name: "Tổng số", SortOrder: 1,
				Method: domain.MethodManual, IsHeadline: true},
			{ID: otherLineID, SheetID: sampleSheetID, OrdinalLabel: "A", Name: "CHI NGÂN SÁCH NHÀ NƯỚC", SortOrder: 2,
				Method: domain.MethodManual},
		},
		value:         map[string]map[string]domain.Dong{sampleLineID: {"c-chi": 5_000_000}},
		sheetIDOfLine: sampleSheetID,
	}
}

// --- (1) the entry shares the transaction ------------------------------------------------------------

func TestCreateLineAuditInTheSAMETransaction(t *testing.T) {
	k := sampleStore()
	uc, ctx := newBudgetUseCase(t, k)

	if _, err := uc.CreateLine(ctx, CreateBudgetLineRequest{
		SheetID: sampleSheetID, ParentID: otherLineID, OrdinalLabel: "I", Name: "Chi đầu tư phát triển", SortOrder: 3,
	}, writer()); err != nil {
		t.Fatalf("ThemKhoanMuc lỗi: %v", err)
	}

	if k.begins != 1 || k.commits != 1 || k.rollbacks != 0 {
		t.Fatalf("giao dịch: mở %d commit %d rollback %d, muốn 1/1/0",
			k.begins, k.commits, k.rollbacks)
	}
	if !k.hasStmt("INSERT INTO khoan_muc_ngan_sach") {
		t.Fatal("không có câu chèn khoản mục")
	}
	if !k.hasStmt("INSERT INTO audit_log") {
		t.Fatal("không có vết kiểm toán")
	}
	// THE SUBJECT IS THE SHEET'S BUSINESS CODE, not an internal id. A budget LINE has no code of its
	// own, so `NS-2026-CHI-01` is the handle an inspection can look up years later; a ULID names
	// nobody (rule 6, invariant 8).
	audit := k.stmtsContaining("INSERT INTO audit_log")[0]
	if !hasArg(audit, "NS-2026-CHI-01") {
		t.Fatalf("subject của vết không phải mã bảng: %v", audit.args)
	}
	if !hasArg(audit, staffCode) {
		t.Fatalf("chủ thể của vết không phải mã cán bộ %q: %v", staffCode, audit.args)
	}
	if hasArg(audit, "nd-01JINTERNALIDCUACANBO") {
		t.Fatal("chủ thể của vết là id nội bộ — rule 6 bất biến 8")
	}
}

func TestFailedAuditLeavesNoLine(t *testing.T) {
	// Rule 6, invariant 3 stated as its consequence: the entry is the LAST statement of the
	// transaction, so a failure there must take the line with it. If the two could be committed
	// separately, the register would hold a line nobody can attribute.
	k := sampleStore()
	k.failOnSQL = "INSERT INTO audit_log"
	uc, ctx := newBudgetUseCase(t, k)

	if _, err := uc.CreateLine(ctx, CreateBudgetLineRequest{
		SheetID: sampleSheetID, Name: "Chi quốc phòng", SortOrder: 3,
	}, writer()); err == nil {
		t.Fatal("vết hỏng mà ThemKhoanMuc vẫn báo thành công")
	}
	if k.commits != 0 || k.rollbacks != 1 {
		t.Fatalf("giao dịch: commit %d rollback %d, muốn 0/1", k.commits, k.rollbacks)
	}
}

// --- (2) a refusal writes nothing ------------------------------------------------------------------------

func TestTypingIntoParentLineWritesNothingMore(t *testing.T) {
	// The customer's decision of 06/09/2026 (anh Hà), enforced in the business layer. `otherLineID` is
	// about to be given a child, so it becomes a parent; typing a figure into it is refused.
	k := sampleStore()
	k.lines = append(k.lines, domain.BudgetLine{
		ID: "k-con", SheetID: sampleSheetID, ParentID: otherLineID, OrdinalLabel: "I", Name: "Chi đầu tư", SortOrder: 3, Level: 1,
	})
	k.sheetIDOfLine = sampleSheetID
	uc, ctx := newBudgetUseCase(t, k)

	one := domain.Dong(9_000_000)
	_, err := uc.UpdateLine(ctx, otherLineID, UpdateBudgetLineRequest{
		Values: map[string]*domain.Dong{"c-chi": &one},
	}, writer())
	if !errors.Is(err, domain.ErrParentLineNoDirectValue) {
		t.Fatalf("= %v, muốn ErrKhoanMucChaKhongGoThang", err)
	}
	if k.hasStmt("INSERT INTO gia_tri_khoan_muc") {
		t.Fatal("bị từ chối mà vẫn ghi giá trị")
	}
	if k.hasStmt("INSERT INTO audit_log") {
		t.Fatal("bị từ chối mà vẫn ghi vết")
	}
	if k.commits != 0 || k.rollbacks != 1 {
		t.Fatalf("giao dịch: commit %d rollback %d, muốn 0/1", k.commits, k.rollbacks)
	}
}

func TestRemoveLineWithChildRowsIsRefused(t *testing.T) {
	// Refused rather than cascaded: a cascade takes a branch off every total in one click, and these
	// rows are archival — "undo" is not a button, it is re-entering them.
	k := sampleStore()
	k.lines = append(k.lines, domain.BudgetLine{
		ID: "k-con", SheetID: sampleSheetID, ParentID: otherLineID, Name: "Chi đầu tư", SortOrder: 3, Level: 1,
	})
	uc, ctx := newBudgetUseCase(t, k)

	err := uc.RemoveLine(ctx, otherLineID, "nhập trùng", writer())
	if !errors.Is(err, domain.ErrLineHasChildren) {
		t.Fatalf("= %v, muốn ErrConGiuKhoanMucCon", err)
	}
	if k.hasStmt("deleted_at = now()") {
		t.Fatal("bị từ chối mà vẫn xoá mềm")
	}
}

// --- (3) the star is a radio ----------------------------------------------------------------------------

func TestSetHeadlineUnmarksEveryOtherRowInSameTransaction(t *testing.T) {
	// THE PROPERTY THAT HAS NO FLOOR IN THE DATABASE. Migration 0006 states why it carries no partial
	// unique index for "at most one marked row", so these two statements under the sheet's row lock
	// ARE the enforcement. Two marked rows is the double count `is_headline` was introduced to end.
	k := sampleStore()
	uc, ctx := newBudgetUseCase(t, k)

	if _, err := uc.SetHeadline(ctx, otherLineID, writer()); err != nil {
		t.Fatalf("DatDongTong lỗi: %v", err)
	}

	unmark := k.stmtsContaining("is_headline = false")
	set := k.stmtsContaining("is_headline = true")
	if len(unmark) != 1 || len(set) != 1 {
		t.Fatalf("câu bỏ đánh dấu %d, câu đánh dấu %d — muốn 1 và 1", len(unmark), len(set))
	}
	// THE CLEAR MUST COME FIRST. Reversed, the sheet would momentarily have two marked rows and then
	// none, and a reader inside the transaction would see a sheet with no total at all.
	if k.stmtIndex("is_headline = false") > k.stmtIndex("is_headline = true") {
		t.Fatal("đánh dấu TRƯỚC khi bỏ đánh dấu — giữa hai câu bảng có hai dòng tổng")
	}
	// The clear is scoped to the SHEET and excludes the row being marked.
	if !strings.Contains(unmark[0].sql, "bang_id = $2") || !strings.Contains(unmark[0].sql, "id <> $3") {
		t.Fatalf("câu bỏ đánh dấu không giới hạn đúng phạm vi: %s", unmark[0].sql)
	}
	if !strings.Contains(unmark[0].sql, "tenant_id = $1") {
		t.Fatalf("câu bỏ đánh dấu không mang tenant_id: %s", unmark[0].sql)
	}
	if k.begins != 1 || k.commits != 1 {
		t.Fatalf("giao dịch: mở %d commit %d, muốn 1/1", k.begins, k.commits)
	}
	// The trail records WHICH ROW HELD IT BEFORE. After the write that answer is gone from the table,
	// so this entry is the only place "the reported total moved from here to there" survives.
	audit := k.stmtsContaining("INSERT INTO audit_log")[0]
	if !deltaContains(audit, sampleLineID) {
		t.Fatalf("vết không ghi dòng tổng CŨ: %v", audit.args)
	}
}

func TestSetHeadlineOnMissingRowWritesNothing(t *testing.T) {
	k := sampleStore()
	k.sheetIDOfLine = "" // no such live line
	uc, ctx := newBudgetUseCase(t, k)

	_, err := uc.SetHeadline(ctx, "khong-co", writer())
	if !errors.Is(err, domain.ErrLineNotFound) {
		t.Fatalf("= %v, muốn ErrKhongThayKhoanMuc", err)
	}
	if k.hasStmt("is_headline = true") || k.hasStmt("INSERT INTO audit_log") {
		t.Fatal("dòng không tồn tại mà vẫn ghi")
	}
}

// --- (4) removal -----------------------------------------------------------------------------------------

func TestRemoveLineWritesAllThreeColumnsAndReturnsStarBack(t *testing.T) {
	// rule 7, invariant 1's three columns in ONE statement so none can be forgotten — plus
	// `is_headline = false`, which is what lets the commune star another row and get its summary back.
	// Without it the flag would sit on a dead row: the total would be gone and nothing would say why.
	k := sampleStore()
	uc, ctx := newBudgetUseCase(t, k)

	if err := uc.RemoveLine(ctx, sampleLineID, "kế toán nhập trùng dòng này", writer()); err != nil {
		t.Fatalf("GoKhoanMuc lỗi: %v", err)
	}
	del := k.stmtsContaining("deleted_at = now()")
	if len(del) != 1 {
		t.Fatalf("có %d câu xoá mềm, muốn 1", len(del))
	}
	for _, frag := range []string{"deleted_at = now()", "deleted_by = $3", "delete_reason = $4",
		"is_headline = false", "deleted_at IS NULL"} {
		if !strings.Contains(del[0].sql, frag) {
			t.Errorf("câu xoá mềm thiếu %q: %s", frag, del[0].sql)
		}
	}
	// NO HARD DELETE ANYWHERE. `ho_so_luu_tru_cam_xoa_cung` refuses one underneath; this is the
	// assertion that the application never even writes one.
	if k.hasStmt("DELETE FROM") {
		t.Fatal("có câu DELETE FROM trên dữ liệu nghiệp vụ")
	}
	// `deleted_by` holds the STAFF CODE, the same value the entry's actor holds. Two kinds of
	// identifier in one column is a column nobody can query.
	if !hasArg(del[0], staffCode) {
		t.Fatalf("deleted_by không phải mã cán bộ: %v", del[0].args)
	}
}

func TestRemoveLastChildRowReturnsParentToManualAndKeepsJustComputedFigure(t *testing.T) {
	// §9 rule 1's second half: "đổi ngược lại thì giữ giá trị vừa tính làm giá trị khởi đầu". Without
	// it the parent goes blank, and blank on this screen means "nobody has entered this" — which is
	// not what happened.
	k := sampleStore()
	k.lines = []domain.BudgetLine{
		{ID: otherLineID, SheetID: sampleSheetID, OrdinalLabel: "A", Name: "CHI NGÂN SÁCH NHÀ NƯỚC", SortOrder: 1,
			Method: domain.MethodChildren},
		{ID: "k-con", SheetID: sampleSheetID, ParentID: otherLineID, OrdinalLabel: "I", Name: "Chi đầu tư", SortOrder: 2,
			Method: domain.MethodManual, Level: 1},
	}
	k.value = map[string]map[string]domain.Dong{"k-con": {"c-chi": 7_777_000}}
	uc, ctx := newBudgetUseCase(t, k)

	if err := uc.RemoveLine(ctx, "k-con", "gộp vào dòng khác", writer()); err != nil {
		t.Fatalf("GoKhoanMuc lỗi: %v", err)
	}
	set := k.stmtsContaining("SET cach_tinh = $3")
	if len(set) != 1 {
		t.Fatalf("có %d câu đặt cách tính, muốn 1", len(set))
	}
	if !hasArg(set[0], string(domain.MethodManual)) {
		t.Fatalf("cha không về `manual`: %v", set[0].args)
	}
	write := k.stmtsContaining("INSERT INTO gia_tri_khoan_muc")
	if len(write) != 1 {
		t.Fatalf("có %d câu ghi giá trị trả lại, muốn 1", len(write))
	}
	if !hasArg(write[0], int64(7_777_000)) {
		t.Fatalf("giá trị trả lại cho cha không phải số vừa tính: %v", write[0].args)
	}
}

// --- (5) cells ---------------------------------------------------------------------------------------------

func TestClearCellWritesNULLNotDeleteRow(t *testing.T) {
	// §9 rule 4 and rule 7, forbidden #1 at once: an empty cell is a STATE the screen draws as `—`,
	// and the row stays. A DELETE here would be a hard delete on business data.
	k := sampleStore()
	uc, ctx := newBudgetUseCase(t, k)

	if _, err := uc.UpdateLine(ctx, sampleLineID, UpdateBudgetLineRequest{
		Values: map[string]*domain.Dong{"c-chi": nil},
	}, writer()); err != nil {
		t.Fatalf("SuaKhoanMuc lỗi: %v", err)
	}
	write := k.stmtsContaining("INSERT INTO gia_tri_khoan_muc")
	if len(write) != 1 {
		t.Fatalf("có %d câu ghi ô, muốn 1", len(write))
	}
	if !strings.Contains(write[0].sql, "ON CONFLICT") {
		t.Fatalf("câu ghi ô không phải upsert: %s", write[0].sql)
	}
	if write[0].args[3] != nil {
		t.Fatalf("xoá ô mà ghi %v thay vì NULL", write[0].args[3])
	}
	if k.hasStmt("DELETE FROM") {
		t.Fatal("xoá ô bằng DELETE — luật 7 cấm #1")
	}
}

func TestUpdateWithNoChangeWritesNothingAndNoAuditEntry(t *testing.T) {
	// A no-op is not an event. Recording it would fill a public authority's ledger with entries saying
	// nothing changed, and those bury the entries that carry legal weight. It is also what makes the
	// route's `idem.KhongCan` declaration true rather than hopeful.
	k := sampleStore()
	uc, ctx := newBudgetUseCase(t, k)

	old := domain.Dong(5_000_000) // exactly what the fixture already holds
	name := "Tổng số"
	if _, err := uc.UpdateLine(ctx, sampleLineID, UpdateBudgetLineRequest{
		Name: &name, Values: map[string]*domain.Dong{"c-chi": &old},
	}, writer()); err != nil {
		t.Fatalf("SuaKhoanMuc lỗi: %v", err)
	}
	if k.hasStmt("INSERT INTO gia_tri_khoan_muc") || k.hasStmt("SET tt = $3") ||
		k.hasStmt("INSERT INTO audit_log") {
		t.Fatal("không có gì đổi mà vẫn ghi")
	}
	if k.commits != 1 {
		t.Fatalf("commit %d, muốn 1 — không ghi gì vẫn phải đóng giao dịch sạch", k.commits)
	}
}

func TestWriteIntoPercentColumnIsRefused(t *testing.T) {
	// §9 rule 3: a percentage is computed at render and never stored. A stored one is a second home
	// for a derivable number, and the stale copy is the one that reaches the report.
	k := sampleStore()
	uc, ctx := newBudgetUseCase(t, k)

	one := domain.Dong(9127)
	_, err := uc.UpdateLine(ctx, sampleLineID, UpdateBudgetLineRequest{
		Values: map[string]*domain.Dong{"c-ty": &one},
	}, writer())
	if !errors.Is(err, domain.ErrColumnNotNumber) {
		t.Fatalf("= %v, muốn ErrCotKhongPhaiCotSo", err)
	}
	if k.hasStmt("INSERT INTO gia_tri_khoan_muc") {
		t.Fatal("bị từ chối mà vẫn ghi ô")
	}
}

func TestWriteIntoColumnOfOtherSheetIsRefused(t *testing.T) {
	// A figure filed against another sheet's column is a figure on no screen, sitting in the table
	// looking healthy. Nothing in the database catches it — `gia_tri_khoan_muc` has no foreign key.
	k := sampleStore()
	uc, ctx := newBudgetUseCase(t, k)

	one := domain.Dong(1_000)
	_, err := uc.UpdateLine(ctx, sampleLineID, UpdateBudgetLineRequest{
		Values: map[string]*domain.Dong{"c-cua-bang-khac": &one},
	}, writer())
	if !errors.Is(err, domain.ErrColumnInOtherSheet) {
		t.Fatalf("= %v, muốn ErrCotKhongThuocBang", err)
	}
	if k.hasStmt("INSERT INTO gia_tri_khoan_muc") {
		t.Fatal("bị từ chối mà vẫn ghi ô")
	}
}

// --- (6) `cach_tinh` follows the tree ------------------------------------------------------------------------

func TestFirstChildSwitchesParentToSumOfChildRows(t *testing.T) {
	// The customer's rule as a PROPERTY rather than as a field somebody has to keep consistent: a leaf
	// that gains its first child stops being typed into and starts summing, in the SAME transaction.
	k := sampleStore()
	uc, ctx := newBudgetUseCase(t, k)

	if _, err := uc.CreateLine(ctx, CreateBudgetLineRequest{
		SheetID: sampleSheetID, ParentID: otherLineID, OrdinalLabel: "I", Name: "Chi đầu tư phát triển", SortOrder: 3,
	}, writer()); err != nil {
		t.Fatalf("ThemKhoanMuc lỗi: %v", err)
	}
	set := k.stmtsContaining("SET cach_tinh = $3")
	if len(set) != 1 {
		t.Fatalf("có %d câu đặt cách tính cho cha, muốn 1", len(set))
	}
	if !hasArg(set[0], string(domain.MethodChildren)) {
		t.Fatalf("cha không chuyển sang `children`: %v", set[0].args)
	}
	// The new line is a LEAF: `manual`, depth = parent + 1, and `is_headline` is not in the INSERT at
	// all — it takes the column default, which is what stops a client creating a second total row.
	insert := k.stmtsContaining("INSERT INTO khoan_muc_ngan_sach")[0]
	if !hasArg(insert, string(domain.MethodManual)) {
		t.Fatalf("dòng mới không phải `manual`: %v", insert.args)
	}
	if strings.Contains(insert.sql, "is_headline") {
		t.Fatalf("câu chèn có nhắc `is_headline` — dòng mới không được là dòng tổng: %s", insert.sql)
	}
	if !hasArg(insert, int64(1)) {
		t.Fatalf("cấp của dòng mới không phải cấp cha + 1: %v", insert.args)
	}
}

func TestAddChildToSummingParentDoesNotRewriteMethod(t *testing.T) {
	// The second child must not rewrite a column that already holds the right value: a no-op UPDATE
	// on a sheet with 59 rows is 59 writes nobody asked for, and it would show in the trail as a
	// change that did not happen.
	k := sampleStore()
	k.lines[1].Method = domain.MethodChildren
	k.lines = append(k.lines, domain.BudgetLine{
		ID: "k-con", SheetID: sampleSheetID, ParentID: otherLineID, Name: "Chi đầu tư", SortOrder: 3, Level: 1,
	})
	uc, ctx := newBudgetUseCase(t, k)

	if _, err := uc.CreateLine(ctx, CreateBudgetLineRequest{
		SheetID: sampleSheetID, ParentID: otherLineID, Name: "Chi an ninh", SortOrder: 4,
	}, writer()); err != nil {
		t.Fatalf("ThemKhoanMuc lỗi: %v", err)
	}
	if k.hasStmt("SET cach_tinh = $3") {
		t.Fatal("cha đã `children` mà vẫn ghi lại cách tính")
	}
}

func TestParentOutsideSheetIsRefused(t *testing.T) {
	// Accepting it would put one year's line under another year's tree and total them together.
	k := sampleStore()
	uc, ctx := newBudgetUseCase(t, k)

	_, err := uc.CreateLine(ctx, CreateBudgetLineRequest{
		SheetID: sampleSheetID, ParentID: "k-cua-bang-khac", Name: "X", SortOrder: 9,
	}, writer())
	if !errors.Is(err, domain.ErrParentInOtherSheet) {
		t.Fatalf("= %v, muốn ErrChaKhongCungBang", err)
	}
	if k.hasStmt("INSERT INTO khoan_muc_ngan_sach") {
		t.Fatal("cha ngoài bảng mà vẫn chèn")
	}
}

// --- (7) creating a sheet ---------------------------------------------------------------------------------------

func TestCreateSheetCodesByRevisionAndInsertsAllColumnsInOneTransaction(t *testing.T) {
	k := &fakeBudgetStore{nextRevision: 2} // this year+kind has been loaded and removed once before
	uc, ctx := newBudgetUseCase(t, k)

	next, err := uc.CreateSheet(ctx, CreateBudgetSheetRequest{
		Year: 2026, Kind: domain.SheetKindExpenditure,
		Title: "BÁO CÁO CHI NGÂN SÁCH NHÀ NƯỚC XÃ THĂNG BÌNH NĂM 2026", Unit: "trieu-dong",
		Columns: []domain.BudgetColumn{
			{Name: "Dự toán năm", SortOrder: 1, Format: domain.ColumnFormatNumber, Indicator: domain.IndicatorAnnualEstimate},
			{Name: "Chi ngân sách", SortOrder: 2, Format: domain.ColumnFormatNumber, Indicator: domain.IndicatorBudgetExpenditure},
			{Name: "So sánh TH/DT (%)", SortOrder: 3, Format: domain.ColumnFormatPercent, Formula: "col_2 / col_1 * 100"},
		},
	}, writer())
	if err != nil {
		t.Fatalf("TaoBang lỗi: %v", err)
	}
	// `NS-2026-CHI-02`, NOT `-01`: the previous load's code is taken forever, even though its sheet is
	// soft-deleted (rule 7, invariant 3). Reissuing it would put one code on two sheets in a table
	// where the old one is still there.
	if next.Code != "NS-2026-CHI-02" {
		t.Fatalf("mã bảng = %q, muốn NS-2026-CHI-02", next.Code)
	}
	if len(k.stmtsContaining("INSERT INTO cot_ngan_sach")) != 3 {
		t.Fatalf("chèn %d cột, muốn 3", len(k.stmtsContaining("INSERT INTO cot_ngan_sach")))
	}
	if k.begins != 1 || k.commits != 1 {
		t.Fatalf("giao dịch: mở %d commit %d, muốn 1/1 — bảng và cột phải cùng một giao dịch",
			k.begins, k.commits)
	}
	// An ordinary column's role goes in as NULL and not ''. `UNIQUE (tenant_id, bang_id, vai_tro)`
	// lets any number of NULLs coexist; two '' would COLLIDE, and every sheet would then be limited
	// to one column without a role — which is every sheet.
	// THE COLUMN STORES THE LABEL, not the wire code — the same bytes as migration 0006's default, so
	// legacy and new rows are one shape (domain.SheetUnit).
	insertSheet := k.stmtsContaining("INSERT INTO bang_ngan_sach")[0]
	if !hasArg(insertSheet, "Triệu đồng") || hasArg(insertSheet, "trieu-dong") {
		t.Fatalf("don_vi_tinh lưu không phải nhãn 'Triệu đồng': %v", insertSheet.args)
	}
	percent := k.stmtsContaining("INSERT INTO cot_ngan_sach")[2]
	if percent.args[7] != nil {
		t.Fatalf("vai trò của cột thường ghi %v thay vì NULL", percent.args[7])
	}
}

func TestExistingLiveSheetIsRefusedAndInsertsNothing(t *testing.T) {
	// §6's `🗑 Gỡ` is how a year's sheet is replaced. A second live sheet would give the year two
	// answers with nothing on either screen saying which the report was built from.
	k := &fakeBudgetStore{hasLiveSheet: true, nextRevision: 2}
	uc, ctx := newBudgetUseCase(t, k)

	_, err := uc.CreateSheet(ctx, CreateBudgetSheetRequest{
		Year: 2026, Kind: domain.SheetKindExpenditure, Title: "X", Unit: "trieu-dong",
		Columns: []domain.BudgetColumn{{Name: "Chi ngân sách", SortOrder: 1, Format: domain.ColumnFormatNumber}},
	}, writer())
	if !errors.Is(err, fistore.ErrSheetExists) {
		t.Fatalf("= %v, muốn ErrBangDaTonTai", err)
	}
	if k.hasStmt("INSERT INTO bang_ngan_sach") || k.hasStmt("INSERT INTO audit_log") {
		t.Fatal("đã có bảng còn sống mà vẫn chèn")
	}
}

func TestTwoColumnsWithSameIndicatorRefusedBeforeTransaction(t *testing.T) {
	// The shape is validated BEFORE the transaction opens: a request that fails its shape must never
	// hold a row lock while doing so, and the caller needs the reason rather than a rollback.
	k := &fakeBudgetStore{nextRevision: 1}
	uc, ctx := newBudgetUseCase(t, k)

	_, err := uc.CreateSheet(ctx, CreateBudgetSheetRequest{
		Year: 2026, Kind: domain.SheetKindRevenue, Title: "X", Unit: "trieu-dong",
		Columns: []domain.BudgetColumn{
			{Name: "Thu xã hưởng", SortOrder: 1, Format: domain.ColumnFormatNumber, Indicator: domain.IndicatorCommuneRetainedRevenue},
			{Name: "Thu xã hưởng (điều chỉnh)", SortOrder: 2, Format: domain.ColumnFormatNumber, Indicator: domain.IndicatorCommuneRetainedRevenue},
		},
	}, writer())
	if !errors.Is(err, domain.ErrIndicatorDuplicateInSheet) {
		t.Fatalf("= %v, muốn ErrVaiTroTrungTrongBang", err)
	}
	if k.begins != 0 {
		t.Fatalf("mở %d giao dịch cho một yêu cầu sai hình dạng, muốn 0", k.begins)
	}
}

func TestRemoveSheetWritesAllThreeColumnsAndAuditCarriesSheetCode(t *testing.T) {
	k := sampleStore()
	uc, ctx := newBudgetUseCase(t, k)

	if err := uc.RemoveSheet(ctx, sampleSheetID, "nạp lại từ tệp Phòng Tài chính đã sửa", writer()); err != nil {
		t.Fatalf("GoBang lỗi: %v", err)
	}
	del := k.stmtsContaining("UPDATE bang_ngan_sach")
	if len(del) != 1 {
		t.Fatalf("có %d câu xoá mềm bảng, muốn 1", len(del))
	}
	for _, frag := range []string{"deleted_at = now()", "deleted_by = $3", "delete_reason = $4"} {
		if !strings.Contains(del[0].sql, frag) {
			t.Errorf("câu xoá mềm bảng thiếu %q", frag)
		}
	}
	// THE TREE IS NOT TOUCHED. Every read reaches the lines THROUGH the sheet, so a removed sheet
	// takes its whole tree off every screen without a single extra row being written.
	if k.hasStmt("UPDATE khoan_muc_ngan_sach") {
		t.Fatal("gỡ bảng mà vẫn ghi vào khoản mục — một hành vi, một lần ghi")
	}
	if !hasArg(k.stmtsContaining("INSERT INTO audit_log")[0], "NS-2026-CHI-01") {
		t.Fatal("vết gỡ bảng không mang mã bảng")
	}
}

func TestMissingActorOpensNoTransaction(t *testing.T) {
	// Rule 6 does not permit a business write whose trail cannot name its author. Refusing BEFORE the
	// transaction opens keeps the row's `deleted_by` and the entry telling the same story, and avoids
	// a rollback whose cause is a missing principal rather than anything about the budget.
	k := sampleStore()
	uc, ctx := newBudgetUseCase(t, k)

	if _, err := uc.CreateLine(ctx, CreateBudgetLineRequest{
		SheetID: sampleSheetID, Name: "X", SortOrder: 1,
	}, audit.Actor{}); err == nil {
		t.Fatal("không có chủ thể mà vẫn ghi được")
	}
	if k.begins != 0 {
		t.Fatalf("mở %d giao dịch, muốn 0", k.begins)
	}
}

// --- (8) editing a sheet's header: title, display unit, cut-off date -------------------------------------------

func strPtr(s string) *string { return &s }

func TestUpdateSheetWriteAndAuditInTheSAMETransaction(t *testing.T) {
	k := sampleStore()
	uc, ctx := newBudgetUseCase(t, k)

	cumulativeTo := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	after, err := uc.UpdateSheet(ctx, sampleSheetID, UpdateBudgetSheetRequest{
		Title: strPtr("BÁO CÁO CHI (đã sửa)"), Unit: strPtr("nghin-dong"), CumulativeTo: &cumulativeTo,
	}, writer())
	if err != nil {
		t.Fatalf("SuaBang lỗi: %v", err)
	}
	if k.begins != 1 || k.commits != 1 || k.rollbacks != 0 {
		t.Fatalf("giao dịch: mở %d commit %d rollback %d, muốn 1/1/0", k.begins, k.commits, k.rollbacks)
	}
	edit := k.stmtsContaining("UPDATE bang_ngan_sach")
	if len(edit) != 1 {
		t.Fatalf("có %d câu cập nhật bảng, muốn 1", len(edit))
	}
	for _, frag := range []string{"tenant_id = $1", "deleted_at IS NULL"} {
		if !strings.Contains(edit[0].sql, frag) {
			t.Errorf("câu cập nhật bảng thiếu %q: %s", frag, edit[0].sql)
		}
	}
	// THE LABEL IS STORED, the code is what the client sent.
	if !hasArg(edit[0], "Nghìn đồng") || !hasArg(edit[0], "BÁO CÁO CHI (đã sửa)") {
		t.Fatalf("giá trị cập nhật sai: %v", edit[0].args)
	}
	if after.Unit != "Nghìn đồng" || !after.CumulativeTo.Equal(cumulativeTo) {
		t.Fatalf("bảng trả về: đơn vị %q, luỹ kế %v", after.Unit, after.CumulativeTo)
	}
	// The locked read excludes removed sheets and is tenant-scoped.
	read := k.stmtsContaining("FOR UPDATE")
	if len(read) == 0 || !strings.Contains(read[0].sql, "deleted_at IS NULL") ||
		!strings.Contains(read[0].sql, "tenant_id = $1") {
		t.Fatalf("câu đọc khoá bảng không loại bảng đã gỡ / không mang tenant_id: %v", read)
	}

	audit := k.stmtsContaining("INSERT INTO audit_log")
	if len(audit) != 1 {
		t.Fatalf("có %d vết, muốn 1", len(audit))
	}
	if k.stmtIndex("UPDATE bang_ngan_sach") > k.stmtIndex("INSERT INTO audit_log") {
		t.Fatal("vết ghi trước câu cập nhật")
	}
	if !hasArg(audit[0], "NS-2026-CHI-01") || !hasArg(audit[0], staffCode) || !hasArg(audit[0], ActionUpdateBudgetSheet) {
		t.Fatalf("vết không mang mã bảng / mã cán bộ / hành vi: %v", audit[0].args)
	}
	// Before AND after of the unit, as the column holds it.
	for _, want := range []string{`"truoc"`, `"sau"`, "Triệu đồng", "Nghìn đồng", "2026-09-30"} {
		if !deltaContains(audit[0], want) {
			t.Errorf("delta thiếu %q", want)
		}
	}
}

func TestUpdateSheetFailedAuditLeavesNothing(t *testing.T) {
	k := sampleStore()
	k.failOnSQL = "INSERT INTO audit_log"
	uc, ctx := newBudgetUseCase(t, k)

	if _, err := uc.UpdateSheet(ctx, sampleSheetID, UpdateBudgetSheetRequest{Title: strPtr("X")}, writer()); err == nil {
		t.Fatal("vết hỏng mà SuaBang vẫn báo thành công")
	}
	if k.commits != 0 || k.rollbacks != 1 {
		t.Fatalf("giao dịch: commit %d rollback %d, muốn 0/1", k.commits, k.rollbacks)
	}
}

func TestUpdateSheetWithNoChangeWritesNothingAndNoAudit(t *testing.T) {
	// The same values the sheet already holds — including `trieu-dong` against a stored
	// "Triệu đồng" — and an empty request: neither is an event.
	for name, req := range map[string]UpdateBudgetSheetRequest{
		"cùng giá trị": {Title: strPtr("BÁO CÁO CHI NGÂN SÁCH NHÀ NƯỚC XÃ THĂNG BÌNH NĂM 2026"),
			Unit: strPtr("trieu-dong"), CumulativeTo: &time.Time{}},
		"rỗng": {},
	} {
		t.Run(name, func(t *testing.T) {
			k := sampleStore()
			uc, ctx := newBudgetUseCase(t, k)
			if _, err := uc.UpdateSheet(ctx, sampleSheetID, req, writer()); err != nil {
				t.Fatalf("SuaBang lỗi: %v", err)
			}
			if k.hasStmt("UPDATE bang_ngan_sach") || k.hasStmt("INSERT INTO audit_log") {
				t.Fatal("không có gì đổi mà vẫn ghi")
			}
			if k.commits != 1 {
				t.Fatalf("commit %d, muốn 1", k.commits)
			}
		})
	}
}

func TestUpdateSheetBadUnitOrEmptyTitleOpensNoTransaction(t *testing.T) {
	for name, tc := range map[string]struct {
		req  UpdateBudgetSheetRequest
		want error
	}{
		"nhãn thay vì mã": {UpdateBudgetSheetRequest{Unit: strPtr("Triệu đồng")}, domain.ErrInvalidUnit},
		"tỷ đồng":         {UpdateBudgetSheetRequest{Unit: strPtr("ty-dong")}, domain.ErrInvalidUnit},
		"tiêu đề rỗng":    {UpdateBudgetSheetRequest{Title: strPtr("  ")}, domain.ErrTitleMissing},
	} {
		t.Run(name, func(t *testing.T) {
			k := sampleStore()
			uc, ctx := newBudgetUseCase(t, k)
			if _, err := uc.UpdateSheet(ctx, sampleSheetID, tc.req, writer()); !errors.Is(err, tc.want) {
				t.Fatalf("= %v, muốn %v", err, tc.want)
			}
			if k.begins != 0 {
				t.Fatalf("mở %d giao dịch cho yêu cầu sai hình dạng, muốn 0", k.begins)
			}
		})
	}
}

func TestUpdateSheetRemovedOrMissingWritesNothing(t *testing.T) {
	k := sampleStore()
	k.sheet = nil // the locked read found no LIVE sheet of this commune
	uc, ctx := newBudgetUseCase(t, k)

	_, err := uc.UpdateSheet(ctx, sampleSheetID, UpdateBudgetSheetRequest{Title: strPtr("X")}, writer())
	if !errors.Is(err, fistore.ErrBudgetSheetNotFound) {
		t.Fatalf("= %v, muốn ErrKhongThayBangNganSach", err)
	}
	if k.hasStmt("UPDATE bang_ngan_sach") || k.hasStmt("INSERT INTO audit_log") {
		t.Fatal("bảng không tồn tại mà vẫn ghi")
	}
}

func TestUpdateSheetTitleOnlyKeepsUnitAndCumulativeTo(t *testing.T) {
	k := sampleStore()
	uc, ctx := newBudgetUseCase(t, k)

	if _, err := uc.UpdateSheet(ctx, sampleSheetID, UpdateBudgetSheetRequest{Title: strPtr("TIÊU ĐỀ MỚI")}, writer()); err != nil {
		t.Fatalf("SuaBang lỗi: %v", err)
	}
	edit := k.stmtsContaining("UPDATE bang_ngan_sach")
	if len(edit) != 1 {
		t.Fatalf("có %d câu cập nhật, muốn 1", len(edit))
	}
	// args: tenant, id, tieu_de, don_vi_tinh, luy_ke_den
	if edit[0].args[3] != "Triệu đồng" || edit[0].args[4] != nil {
		t.Fatalf("sửa tiêu đề mà đơn vị/luỹ kế đổi theo: %v", edit[0].args)
	}
}

func TestUpdateSheetLegacyFreeTextToCodeWritesAndKeepsOldTextInAudit(t *testing.T) {
	// A legacy sheet holding free text set to a code DOES change — its column becomes the label and
	// the screen prints differently from that moment — so it is written and the old text recorded.
	k := sampleStore()
	k.sheet.unit = "tr.đồng"
	uc, ctx := newBudgetUseCase(t, k)

	if _, err := uc.UpdateSheet(ctx, sampleSheetID, UpdateBudgetSheetRequest{Unit: strPtr("trieu-dong")}, writer()); err != nil {
		t.Fatalf("SuaBang lỗi: %v", err)
	}
	if !k.hasStmt("UPDATE bang_ngan_sach") {
		t.Fatal("chữ cũ đổi sang nhãn chuẩn mà không ghi")
	}
	if !deltaContains(k.stmtsContaining("INSERT INTO audit_log")[0], "tr.đồng") {
		t.Fatal("vết không giữ chữ đơn vị cũ")
	}
}

// --- helpers ------------------------------------------------------------------------------------------------

// stmtIndex is the index of the first recorded statement containing `substr`, or -1.
func (k *fakeBudgetStore) stmtIndex(substr string) int {
	k.mu.Lock()
	defer k.mu.Unlock()
	for i, l := range k.stmts {
		if strings.Contains(l.sql, substr) {
			return i
		}
	}
	return -1
}

func hasArg(l recordedStmt, want any) bool {
	for _, a := range l.args {
		if a == want {
			return true
		}
	}
	return false
}

// deltaContains looks inside the audit entry's JSON delta, which arrives as a []byte argument.
func deltaContains(l recordedStmt, want string) bool {
	for _, a := range l.args {
		b, ok := a.([]byte)
		if ok && strings.Contains(string(b), want) {
			return true
		}
	}
	return false
}
