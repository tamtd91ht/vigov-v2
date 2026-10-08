package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// THE EXCEL IMPORT OF THE BUDGET BOARD, through the real store on the fake driver
// (driver_gia_ngan_sach_test.go says what that proves and what it does not).
//
//  1. a closed YEAR, and a closed MONTH, refuse the whole file and write nothing;
//  2. a live sheet holding hand entries (batches, or hand-typed cells) refuses with "Gỡ … trước";
//  3. a live sheet WITHOUT hand entries is soft deleted and replaced by the next revision, with an
//     entry for each act, in the same transaction;
//  4. parents' figures are not stored, the headline goes through the star's own statement;
//  5. a failing audit entry rolls the whole file back; the preview never writes;
//  6. `Gỡ` removes an imported sheet exactly like a hand-made one.

const importFileName = "bao-cao-thu-chi-2026.xlsx"

func cellsOf(rows ...[]string) [][]domain.BudgetImportCell {
	out := make([][]domain.BudgetImportCell, len(rows))
	for i, r := range rows {
		for j, v := range r {
			// Figures from column C on, below the header, are number cells, as Excel stores them.
			out[i] = append(out[i], domain.BudgetImportCell{Text: v, Number: j >= 2 && v != "" && i >= 3})
		}
	}
	return out
}

// importedExpenditure parses a small chi sheet: "Tổng số" (leaf), A with two children.
func importedExpenditure(t *testing.T) domain.ImportedSheet {
	t.Helper()
	wb, errs := domain.ParseBudgetWorkbook([]domain.BudgetImportSheet{{Name: "Chi", Rows: cellsOf(
		[]string{"BÁO CÁO CHI NGÂN SÁCH XÃ DEMO NĂM 2026"},
		[]string{"Đơn vị tính: Triệu đồng"},
		[]string{"STT", "Chỉ tiêu", "Dự toán năm", "Chi ngân sách", "So sánh TH/DT (%)"},
		[]string{"", "Tổng số", "400", "300"},
		[]string{"A", "CHI NGÂN SÁCH NHÀ NƯỚC", "400", "290"},
		[]string{"I", "Chi đầu tư phát triển", "60", "80"},
		[]string{"II", "Chi thường xuyên", "340", "220"},
	)}})
	if len(errs) > 0 {
		t.Fatalf("parse: %+v", errs)
	}
	return wb.Sheets[0]
}

func importRequest(t *testing.T) BudgetImportRequest {
	return BudgetImportRequest{Year: 2026, FileName: importFileName, Sheets: []domain.ImportedSheet{importedExpenditure(t)}}
}

// emptyYear is a commune with no live chi sheet for 2026 and nothing closed.
func emptyYear() *khoNSGia { return &khoNSGia{lanKeTiep: 1} }

func auditEntriesWith(k *khoNSGia, action string) []lenhGhi {
	var out []lenhGhi
	for _, l := range k.cau("INSERT INTO audit_log") {
		if coGiaTri(l, action) {
			out = append(out, l)
		}
	}
	return out
}

func anyStatementHas(ls []lenhGhi, want any) bool {
	for _, l := range ls {
		if coGiaTri(l, want) {
			return true
		}
	}
	return false
}

func TestBudgetImport_CreatesSheetLinesLeafValuesStarAndEntryInOneTx(t *testing.T) {
	k := emptyYear()
	uc, ctx := dungUseCaseNganSach(t, k)

	res, err := uc.ImportBudgetWorkbook(ctx, importRequest(t), nguoiGhi())
	if err != nil {
		t.Fatalf("ImportBudgetWorkbook: %v", err)
	}
	if k.batDau != 1 || k.daCommit != 1 {
		t.Fatalf("transactions: begun %d committed %d, want 1/1", k.batDau, k.daCommit)
	}
	created := res.Sheets[0].Created
	if created.Ma != "NS-2026-CHI-01" || created.NguonTep != importFileName || created.NapLuc.IsZero() || created.DonViTinh != "Triệu đồng" {
		t.Errorf("created = %+v", created)
	}
	ins := k.cau("INSERT INTO bang_ngan_sach")
	if len(ins) != 1 || !coGiaTri(ins[0], importFileName) {
		t.Fatalf("sheet insert = %+v — `nguon_tep` must carry the file name", ins)
	}
	if n := len(k.cau("INSERT INTO cot_ngan_sach")); n != 3 {
		t.Errorf("%d column inserts, want 3", n)
	}
	if n := len(k.cau("INSERT INTO khoan_muc_ngan_sach")); n != 4 {
		t.Errorf("%d line inserts, want 4", n)
	}
	// LEAVES ONLY: "Tổng số" (2 cells), I (2), II (2). A is a parent — its 400/290 are never stored.
	values := k.cau("INSERT INTO gia_tri_khoan_muc")
	if len(values) != 6 {
		t.Errorf("%d value writes, want 6 (parent figures must not be stored)", len(values))
	}
	if anyStatementHas(values, int64(290_000_000)) {
		t.Error("the parent's own figure from the file was stored")
	}
	if !anyStatementHas(values, int64(300_000_000)) {
		t.Error("300 triệu đồng was not stored as 300 000 000 đồng")
	}
	// THE STAR through its own statement pair, on "Tổng số".
	if !k.coCau("is_headline = true") {
		t.Error("the headline was not starred")
	}
	if k.coCau("UPDATE bang_ngan_sach") {
		t.Error("nothing to replace, yet a sheet was soft deleted")
	}
	entries := auditEntriesWith(k, ActionBudgetSheetImport)
	if len(entries) != 1 || !coGiaTri(entries[0], "NS-2026-CHI-01") || !coGiaTri(entries[0], maCanBo) {
		t.Fatalf("import entry = %+v — subject = the sheet code, actor = the staff code", entries)
	}
	for _, want := range []string{importFileName, "dong_tong_tu_danh_dau", "Tổng số"} {
		if !coChuoiTrongDelta(entries[0], want) {
			t.Errorf("entry delta lacks %q", want)
		}
	}
	if k.thuTuCua("INSERT INTO bang_ngan_sach") > k.thuTuCua("INSERT INTO audit_log") {
		t.Error("the entry was written before the sheet")
	}
}

func TestBudgetImport_ClosedYearOrClosedMonthRefusesAndWritesNothing(t *testing.T) {
	for _, closed := range []domain.BudgetPeriodClose{
		{ID: "c1", Code: "CK-2026-CN-01", Year: 2026},
		{ID: "c2", Code: "CK-2026-05-01", Year: 2026, Month: 5},
	} {
		k := emptyYear()
		k.closes = []domain.BudgetPeriodClose{closed}
		uc, ctx := dungUseCaseNganSach(t, k)

		_, err := uc.ImportBudgetWorkbook(ctx, importRequest(t), nguoiGhi())
		if !errors.Is(err, domain.ErrPeriodClosed) || !strings.Contains(err.Error(), closed.Code) {
			t.Fatalf("%s: err = %v", closed.Code, err)
		}
		if k.coCau("INSERT INTO") || k.coCau("UPDATE bang_ngan_sach") || k.daCommit != 0 {
			t.Errorf("%s: a closed period still wrote something", closed.Code)
		}
		// The lock was taken before the closes were read (budget_period_close.go's order).
		if k.thuTuCua("pg_advisory_xact_lock") > k.thuTuCua("FROM budget_period_closes") {
			t.Errorf("%s: closes read before the year lock", closed.Code)
		}
	}
	// Another year's close does not lock 2026.
	k := emptyYear()
	k.closes = []domain.BudgetPeriodClose{{ID: "c3", Code: "CK-2025-CN-01", Year: 2025}}
	uc, ctx := dungUseCaseNganSach(t, k)
	if _, err := uc.ImportBudgetWorkbook(ctx, importRequest(t), nguoiGhi()); err != nil {
		t.Fatalf("2025's close refused a 2026 import: %v", err)
	}
}

func liveSheet(sourceFile string) *hangBang {
	return &hangBang{id: idBangMau, ma: "NS-2026-CHI-01", nam: 2026, loai: "chi", lan: 1,
		tieuDe: "BÁO CÁO CHI CŨ", donViTinh: "Triệu đồng", sourceFile: sourceFile}
}

func TestBudgetImport_LiveSheetWithHandEntriesIsRefusedRemoveFirst(t *testing.T) {
	cases := map[string]func(k *khoNSGia){
		"batches": func(k *khoNSGia) {
			k.entryLines = []handEntryLine{{code: "1.1", name: "Chi quốc phòng", batches: 2}}
		},
		"hand-typed cells": func(k *khoNSGia) { k.handCells = 3 },
	}
	for name, set := range cases {
		k := emptyYear()
		k.bang = liveSheet("")
		k.lanKeTiep = 2
		set(k)
		uc, ctx := dungUseCaseNganSach(t, k)

		_, err := uc.ImportBudgetWorkbook(ctx, importRequest(t), nguoiGhi())
		if !errors.Is(err, domain.ErrBudgetSheetHasHandEntries) || !strings.Contains(err.Error(), "Gỡ bảng trước") ||
			!strings.Contains(err.Error(), "NS-2026-CHI-01") {
			t.Fatalf("%s: err = %v", name, err)
		}
		if k.coCau("INSERT INTO") || k.coCau("UPDATE bang_ngan_sach") || k.daCommit != 0 {
			t.Errorf("%s: refused, yet something was written", name)
		}
	}
	// The cell read carries the rule in its SQL: hand sheet = any filled cell; imported sheet = cells
	// written after the load (same-transaction now() equality).
	k := emptyYear()
	k.bang = liveSheet("")
	uc, ctx := dungUseCaseNganSach(t, k)
	_, _ = uc.ImportBudgetWorkbook(ctx, importRequest(t), nguoiGhi())
	q := k.cau("hand_entry_cells")
	if len(q) != 1 || !strings.Contains(q[0].sql, "g.cap_nhat_luc > b.tao_luc") ||
		!strings.Contains(q[0].sql, "WHEN b.nguon_tep IS NULL THEN g.gia_tri IS NOT NULL") ||
		!strings.Contains(q[0].sql, "k.deleted_at IS NULL") {
		t.Fatalf("hand-cell read = %+v", q)
	}
	lines := k.cau("hand_entry_lines")
	if len(lines) != 1 || !strings.Contains(lines[0].sql, "d.deleted_at IS NULL") {
		t.Fatalf("batch read must count LIVE batches only: %+v", lines)
	}
}

func TestBudgetImport_LiveSheetWithoutHandEntriesIsReplacedByNextRevision(t *testing.T) {
	k := emptyYear()
	k.bang = liveSheet("bao-cao-thang-8.xlsx")
	k.lanKeTiep = 2
	uc, ctx := dungUseCaseNganSach(t, k)

	res, err := uc.ImportBudgetWorkbook(ctx, importRequest(t), nguoiGhi())
	if err != nil {
		t.Fatalf("ImportBudgetWorkbook: %v", err)
	}
	if res.Sheets[0].Created.Ma != "NS-2026-CHI-02" || res.Sheets[0].Replaces.Ma != "NS-2026-CHI-01" {
		t.Fatalf("outcome = %+v", res.Sheets[0])
	}
	del := k.cau("UPDATE bang_ngan_sach")
	if len(del) != 1 || !strings.Contains(del[0].sql, "deleted_at = now()") ||
		!coGiaTri(del[0], maCanBo) || !coGiaTri(del[0], importReplaceReason) {
		t.Fatalf("soft delete = %+v — all three columns, staff code, reason", del)
	}
	if k.coCau("DELETE FROM") {
		t.Fatal("a hard delete on an archival record")
	}
	removed := auditEntriesWith(k, HanhViGoBangNganSach)
	if len(removed) != 1 || !coGiaTri(removed[0], "NS-2026-CHI-01") || !coChuoiTrongDelta(removed[0], "NS-2026-CHI-02") {
		t.Fatalf("removal entry = %+v", removed)
	}
	if n := len(auditEntriesWith(k, ActionBudgetSheetImport)); n != 1 {
		t.Fatalf("%d import entries, want 1", n)
	}
	if k.thuTuCua("UPDATE bang_ngan_sach") > k.thuTuCua("INSERT INTO bang_ngan_sach") {
		t.Error("the new sheet was inserted while the old one was still live")
	}
	if k.batDau != 1 || k.daCommit != 1 {
		t.Errorf("transactions: begun %d committed %d", k.batDau, k.daCommit)
	}
}

func TestBudgetImport_FailingEntryRollsBackTheWholeFile(t *testing.T) {
	k := emptyYear()
	k.loiSau = "INSERT INTO audit_log"
	uc, ctx := dungUseCaseNganSach(t, k)
	if _, err := uc.ImportBudgetWorkbook(ctx, importRequest(t), nguoiGhi()); err == nil {
		t.Fatal("the entry failed, yet the import succeeded")
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Fatalf("commit %d rollback %d — the sheet must go down with its entry", k.daCommit, k.daRollback)
	}
}

func TestBudgetImport_PreviewWritesNothingAndReportsRefusals(t *testing.T) {
	k := emptyYear()
	k.bang = liveSheet("")
	k.entryLines = []handEntryLine{{code: "1.1", name: "Chi quốc phòng", batches: 1}}
	k.closes = []domain.BudgetPeriodClose{{ID: "c2", Code: "CK-2026-05-01", Year: 2026, Month: 5}}
	uc, ctx := dungUseCaseNganSach(t, k)

	res, err := uc.PreviewBudgetImport(ctx, importRequest(t))
	if err != nil {
		t.Fatalf("PreviewBudgetImport: %v", err)
	}
	if res.Valid() || !strings.Contains(res.Refusal, "CK-2026-05-01") || !strings.Contains(res.Sheets[0].Refusal, "Gỡ bảng trước") {
		t.Fatalf("preview = %+v", res)
	}
	if res.Sheets[0].Replaces.Ma != "NS-2026-CHI-01" {
		t.Errorf("preview does not name the sheet it would replace: %+v", res.Sheets[0].Replaces)
	}
	if k.coCau("INSERT INTO") || k.coCau("UPDATE bang_ngan_sach") || k.daCommit != 0 || k.daRollback != 1 {
		t.Fatalf("preview wrote: commit %d rollback %d", k.daCommit, k.daRollback)
	}
}

func TestBudgetImport_NoActorOpensNoTransaction(t *testing.T) {
	k := emptyYear()
	uc, ctx := dungUseCaseNganSach(t, k)
	if _, err := uc.ImportBudgetWorkbook(ctx, importRequest(t), audit.Actor{}); err == nil || k.batDau != 0 {
		t.Fatalf("err %v, %d transactions", err, k.batDau)
	}
}

// `Gỡ` ON AN IMPORTED SHEET is the ordinary removal: three soft-delete columns, an entry under the
// sheet's code, the tree untouched (it leaves every screen through the sheet).
func TestGoBang_RemovesAnImportedSheet(t *testing.T) {
	k := khoMau()
	k.bang.sourceFile = importFileName
	uc, ctx := dungUseCaseNganSach(t, k)
	if err := uc.GoBang(ctx, idBangMau, "nạp nhầm tệp tháng trước", nguoiGhi()); err != nil {
		t.Fatalf("GoBang: %v", err)
	}
	del := k.cau("UPDATE bang_ngan_sach")
	if len(del) != 1 || !strings.Contains(del[0].sql, "delete_reason = $4") || k.coCau("DELETE FROM") {
		t.Fatalf("removal = %+v", del)
	}
	if e := auditEntriesWith(k, HanhViGoBangNganSach); len(e) != 1 || !coGiaTri(e[0], "NS-2026-CHI-01") {
		t.Fatalf("removal entry = %+v", e)
	}
}
