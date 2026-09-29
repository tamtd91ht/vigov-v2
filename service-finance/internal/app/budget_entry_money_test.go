package app

import (
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// WHAT THIS FILE ADDS to budget_entry_test.go: the edges of the MONEY, where a defect leaves every
// figure plausible and nobody notices until an inspection compares the report with the receipts.
//
//  1. entries -> manual copies each column EXACTLY: a sum of 0 becomes a stored 0 (not an empty
//     cell), a negative sum stays negative, and a percentage column never receives an amount;
//  2. a batch sum that does not fit int64, or exceeds the typo guard, REFUSES the switch and writes
//     nothing — it is never skipped into an empty cell or truncated into a wrong one;
//  3. changing a sheet's display unit writes the header and the entry, and NOTHING ELSE: the stored
//     đồng are never rescaled;
//  4. the `⇄` list, read through the REAL store, reaches only live batches of a live line of a live
//     sheet — the three soft-delete predicates of that read are in the statements.

// --- (1) the entries -> manual hand-over, column by column ----------------------------------------------

func TestSwitchEntriesToManualCopiesEachColumnZeroStaysZeroNegativeKeepsSign(t *testing.T) {
	k := sampleStore()
	k.lines[1].Method = domain.MethodEntries
	// No typed figure at all on the line, so EVERY write below is the hand-over's own.
	k.value[otherLineID] = map[string]domain.Dong{}
	k.entryTotals = map[string]map[string]domain.Dong{otherLineID: {
		// Two instalments that cancel out: the batches STATED an amount, and it adds up to 0. The screen
		// showed `0`, and the manual starting value must be 0 — not `—` (§9 rule 4).
		"c-chi": 0,
		// A net refund.
		"c-dt": -123_456_789,
		// A sum row for the PERCENTAGE column. 0008 says the database cannot stop such a row; §9 rule 3
		// says a percentage is never stored. The hand-over must not write it.
		"c-ty": 55,
	}}
	uc, ctx := newBudgetUseCase(t, k)

	if _, err := uc.UpdateLine(ctx, otherLineID, UpdateBudgetLineRequest{Method: method(domain.MethodManual)}, writer()); err != nil {
		t.Fatalf("SuaKhoanMuc lỗi: %v", err)
	}

	write := k.stmtsContaining("INSERT INTO gia_tri_khoan_muc")
	byColumn := map[string]any{}
	for _, g := range write {
		// args: tenant, khoan_muc_id, cot_id, gia_tri
		column, _ := g.args[2].(string)
		byColumn[column] = g.args[3]
	}
	if v, ok := byColumn["c-chi"]; !ok || v != int64(0) {
		t.Fatalf("c-chi (tổng đợt = 0) ghi %v (có ghi: %v), muốn int64(0) — 0 là một con số, không phải ô trống", v, ok)
	}
	if v := byColumn["c-dt"]; v != int64(-123_456_789) {
		t.Fatalf("c-dt (tổng đợt âm) ghi %v, muốn -123456789 đúng từng đồng", v)
	}
	if _, ok := byColumn["c-ty"]; ok {
		t.Fatal("cột phần trăm nhận số tiền từ tổng đợt (§9 quy tắc 3)")
	}
	if len(write) != 2 {
		t.Fatalf("có %d câu ghi ô, muốn đúng 2 (c-chi, c-dt)", len(write))
	}
	audit := k.stmtsContaining("INSERT INTO audit_log")
	if len(audit) != 1 || !deltaContains(audit[0], "-123456789") {
		t.Fatalf("vết không ghi số âm được chép vào ô: %v", audit)
	}
}

// --- (2) a sum that is not a budget figure refuses the switch -------------------------------------------

func TestSwitchEntriesToManualTotalOverflowIsRefusedAndWritesNothing(t *testing.T) {
	for name, tc := range map[string]struct {
		edit func(*fakeBudgetStore)
		want error
	}{
		// 2^63: SUM(BIGINT) is NUMERIC in PostgreSQL and CAN exceed int64. Skipping the row would copy
		// NULL (`—`) over the figure; wrapping would copy a negative number. Both look healthy.
		"tổng vượt int64": {func(k *fakeBudgetStore) {
			k.rawEntryTotals = map[string]map[string]string{otherLineID: {"c-chi": "9223372036854775808"}}
		}, domain.ErrEntryTotalOverflow},
		"tổng âm vượt int64": {func(k *fakeBudgetStore) {
			k.rawEntryTotals = map[string]map[string]string{otherLineID: {"c-chi": "-9223372036854775809"}}
		}, domain.ErrEntryTotalOverflow},
		// Fits int64 but is past the typo guard every typed cell passes: copying it would put into a
		// manual cell what no person could have typed there.
		"tổng vượt chặn gõ nhầm": {func(k *fakeBudgetStore) {
			k.entryTotals = map[string]map[string]domain.Dong{otherLineID: {"c-chi": domain.ValueMax + 1}}
		}, domain.ErrValueTooLarge},
	} {
		t.Run(name, func(t *testing.T) {
			k := sampleStore()
			k.lines[1].Method = domain.MethodEntries
			k.value[otherLineID] = map[string]domain.Dong{"c-chi": 42}
			tc.edit(k)
			uc, ctx := newBudgetUseCase(t, k)

			_, err := uc.UpdateLine(ctx, otherLineID, UpdateBudgetLineRequest{Method: method(domain.MethodManual)}, writer())
			if !errors.Is(err, tc.want) {
				t.Fatalf("= %v, muốn %v", err, tc.want)
			}
			if k.hasStmt("INSERT INTO gia_tri_khoan_muc") || k.hasStmt("SET cach_tinh = $3") ||
				k.hasStmt("INSERT INTO audit_log") {
				t.Fatal("tổng đợt không phải một con số ngân sách mà vẫn ghi")
			}
			if k.commits != 0 {
				t.Fatalf("commit %d, muốn 0", k.commits)
			}
			// The refusal never quotes the figure (it is the commune's budget, and it travels into logs).
			if strings.Contains(err.Error(), "9223372036854775808") {
				t.Fatalf("câu lỗi trích con số: %q", err.Error())
			}
		})
	}
}

// --- (3) the display unit never touches the stored đồng -------------------------------------------------

func TestUpdateSheetUnitChangeNeverTouchesStoredFigures(t *testing.T) {
	// ĐƠN VỊ LÀ CÁCH HIỂN THỊ. Changing "Triệu đồng" to "Đồng" must write the header and its entry,
	// and nothing else: a rescale of the cells would multiply every figure of the report by 10^6 in
	// one click, with an entry that reads as a label change.
	for _, code := range []string{"dong", "nghin-dong"} {
		t.Run(code, func(t *testing.T) {
			k := sampleStore()
			k.entryTotals = map[string]map[string]domain.Dong{otherLineID: {"c-chi": 7_000}}
			uc, ctx := newBudgetUseCase(t, k)

			if _, err := uc.UpdateSheet(ctx, sampleSheetID, UpdateBudgetSheetRequest{Unit: strPtr(code)}, writer()); err != nil {
				t.Fatalf("SuaBang lỗi: %v", err)
			}
			if !k.hasStmt("UPDATE bang_ngan_sach") {
				t.Fatal("đổi đơn vị mà không ghi đầu bảng — phép kiểm dưới đây sẽ xanh vô nghĩa")
			}
			k.mu.Lock()
			defer k.mu.Unlock()
			for _, l := range k.stmts {
				// By the statement's FIRST word: `SELECT … FOR UPDATE` is the lock, not a write.
				start := strings.ToUpper(strings.TrimSpace(l.sql))
				write := strings.HasPrefix(start, "INSERT") || strings.HasPrefix(start, "UPDATE") ||
					strings.HasPrefix(start, "DELETE") || strings.HasPrefix(start, "WITH")
				if !write {
					continue
				}
				if !strings.Contains(l.sql, "UPDATE bang_ngan_sach") && !strings.Contains(l.sql, "INSERT INTO audit_log") {
					t.Errorf("đổi đơn vị mà ghi thêm câu khác: %s", l.sql)
				}
				if strings.Contains(l.sql, "gia_tri") {
					t.Errorf("đổi đơn vị mà câu ghi chạm tới giá trị: %s", l.sql)
				}
			}
		})
	}
}

// --- (4) the `⇄` list through the real store --------------------------------------------------------------

func TestEntryListThroughRealStoreSeesOnlyLiveEntriesOfLiveLineInLiveSheet(t *testing.T) {
	// The HTTP tests read the list through a fake READER, so the store's own statements — the only
	// place the three soft-delete predicates live — ran under no test at all. A removed batch shown in
	// the list is a batch an accountant reconciles against a receipt while it no longer counts; a line
	// of a removed sheet answering 200 is a sheet that is off every screen except this one.
	k := sampleStore()
	k.lines = k.lines[1:] // the line read answers with THIS line
	k.lines[0].Method = domain.MethodEntries
	k.entry = liveEntry()
	k.entry.Amounts = map[string]domain.Dong{"c-chi": -1_500_000, "c-dt": 0}

	db := sql.OpenDB(k)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	_, ctx := newBudgetUseCase(t, sampleStore()) // only for a context carrying commune A
	s := fistore.NewBudgetStore(store.New(db))

	list, err := s.LineEntries(ctx, otherLineID)
	if err != nil {
		t.Fatalf("DotCuaKhoanMuc lỗi: %v", err)
	}
	if len(list.Entries) != 1 || list.Entries[0].Amounts["c-chi"] != -1_500_000 {
		t.Fatalf("đợt đọc ra: %+v", list.Entries)
	}
	// 0 is a stated amount and must survive the read as a PRESENT key.
	if g, ok := list.Entries[0].Amounts["c-dt"]; !ok || g != 0 {
		t.Fatalf("số tiền 0 của đợt: %v (có khoá: %v), muốn 0 có mặt", g, ok)
	}

	row := k.stmtsContaining("JOIN bang_ngan_sach b")
	if len(row) != 1 {
		t.Fatalf("có %d câu đọc khoản mục kèm bảng, muốn 1", len(row))
	}
	for _, frag := range []string{"k.tenant_id = $1", "k.deleted_at IS NULL", "b.deleted_at IS NULL",
		"b.tenant_id = k.tenant_id"} {
		if !strings.Contains(row[0].sql, frag) {
			t.Errorf("câu đọc khoản mục của hộp đợt thiếu %q: %s", frag, row[0].sql)
		}
	}

	listStmts := k.stmtsContaining("FROM dot_thu_chi WHERE")
	if len(listStmts) != 1 {
		t.Fatalf("có %d câu đọc danh sách đợt, muốn 1", len(listStmts))
	}
	for _, frag := range []string{"tenant_id = $1", "deleted_at IS NULL", "khoan_muc_id = $2"} {
		if !strings.Contains(listStmts[0].sql, frag) {
			t.Errorf("câu danh sách đợt thiếu %q: %s", frag, listStmts[0].sql)
		}
	}
	if !hasArg(listStmts[0], int64(fistore.MaxEntriesPerLine+1)) {
		t.Errorf("LIMIT của danh sách đợt không phải trần cộng một: %v", listStmts[0].args)
	}

	lineStmts := k.stmtsContaining("d.khoan_muc_id = $2")
	if len(lineStmts) != 1 {
		t.Fatalf("có %d câu đọc số tiền đợt, muốn 1", len(lineStmts))
	}
	for _, frag := range []string{"g.tenant_id = $1", "d.tenant_id = g.tenant_id", "d.deleted_at IS NULL"} {
		if !strings.Contains(lineStmts[0].sql, frag) {
			t.Errorf("câu số tiền đợt thiếu %q: %s", frag, lineStmts[0].sql)
		}
	}
}

// --- (5) one oversized batch sum never locks the sheet --------------------------------------------------
//
// Before 25/09/2026 the sheet read computed the batch sum of EVERY line and failed on the first that
// did not fit int64 — so one bad sum refused every write on the board, RemoveEntry included, and RemoveEntry is
// the only act that removes the batch that caused it.

const totalPastInt64 = "9223372036854775808" // 2^63: SUM(BIGINT) is NUMERIC and can reach it

func storeWithEntryTotalOverflow() *fakeBudgetStore {
	k := sampleStore()
	k.lines[1].Method = domain.MethodEntries
	k.rawEntryTotals = map[string]map[string]string{otherLineID: {"c-chi": totalPastInt64}}
	return k
}

func TestEntryTotalPastInt64RemoveEntryStillWorks(t *testing.T) {
	k := storeWithEntryTotalOverflow()
	k.entry = liveEntry() // a batch of the offending line
	uc, ctx := newBudgetUseCase(t, k)

	if err := uc.RemoveEntry(ctx, k.entry.ID, "ghi nhầm số tiền", writer()); err != nil {
		t.Fatalf("gỡ đợt gây ra tổng vượt mức bị chặn: %v", err)
	}
	if !k.hasStmt("UPDATE dot_thu_chi") || !k.hasStmt("INSERT INTO audit_log") || k.commits != 1 {
		t.Fatalf("gỡ đợt không ghi đủ: commit %d", k.commits)
	}
}

func TestEntryTotalPastInt64OtherWritesStillWork(t *testing.T) {
	t.Run("gõ số vào dòng khác", func(t *testing.T) {
		k := storeWithEntryTotalOverflow()
		uc, ctx := newBudgetUseCase(t, k)
		_, err := uc.UpdateLine(ctx, sampleLineID,
			UpdateBudgetLineRequest{Values: map[string]*domain.Dong{"c-chi": dongPtr(6_000_000)}}, writer())
		if err != nil {
			t.Fatalf("ghi ô dòng khác bị chặn bởi tổng đợt của dòng kia: %v", err)
		}
		if k.commits != 1 {
			t.Fatalf("commit %d, muốn 1", k.commits)
		}
	})
	t.Run("ghi thêm đợt vào chính dòng đó", func(t *testing.T) {
		k := storeWithEntryTotalOverflow()
		uc, ctx := newBudgetUseCase(t, k)
		if _, err := uc.RecordEntry(ctx, sampleEntryRequest(), writer()); err != nil {
			t.Fatalf("ghi đợt bị chặn: %v", err)
		}
	})
}

func TestEntryTotalPastInt64SheetReadStillWorksAndOnlyThatLineIsUnavailable(t *testing.T) {
	// Through the REAL store's non-transactional read (GET /budget-sheets).
	k := storeWithEntryTotalOverflow()
	db := sql.OpenDB(k)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	_, ctx := newBudgetUseCase(t, sampleStore())
	s := fistore.NewBudgetStore(store.New(db))

	full, err := s.FullSheet(ctx, 2026, domain.SheetKindExpenditure)
	if err != nil {
		t.Fatalf("đọc bảng thất bại vì một tổng đợt: %v", err)
	}
	if !full.EntryTotalOverflow[otherLineID]["c-chi"] {
		t.Fatalf("không đánh dấu tổng vượt mức: %+v", full.EntryTotalOverflow)
	}
	if o := full.Value(otherLineID, "c-chi"); o.Present || !strings.Contains(o.Reason, domain.ErrEntryTotalOverflow.Error()) {
		t.Fatalf("ô vượt mức = %+v, muốn không tính được kèm lý do", o)
	}
	if o := full.Value(sampleLineID, "c-chi"); !o.Present || o.Value != 5_000_000 {
		t.Fatalf("dòng khác = %+v, muốn 5000000", o)
	}
	// The statement binds `entries` as $3 — the only mode whose figure is the batch sum.
	total := k.stmtsContaining("SUM(g.gia_tri)")
	if len(total) != 1 || !hasArg(total[0], string(domain.MethodEntries)) || !strings.Contains(total[0].sql, "NOT EXISTS") {
		t.Fatalf("câu tổng đợt không lọc theo lá `entries`: %+v", total)
	}
}

func TestEntryTotalReadOnlyForEntriesLeaf(t *testing.T) {
	// A MANUAL line whose batch "sum" is not even a number. If the sheet read summed that line, the
	// write would fail on it; it must not be read at all. Then the same text on an ENTRIES line does
	// fail — proving the filter, not a lenient parser, is what let the first case through.
	k := sampleStore()
	k.rawEntryTotals = map[string]map[string]string{otherLineID: {"c-chi": "khong-phai-so"}}
	uc, ctx := newBudgetUseCase(t, k)
	if _, err := uc.RecordEntry(ctx, sampleEntryRequest(), writer()); err != nil {
		t.Fatalf("dòng `manual` vẫn bị đọc tổng đợt: %v", err)
	}

	k2 := sampleStore()
	k2.lines[1].Method = domain.MethodEntries
	k2.rawEntryTotals = map[string]map[string]string{otherLineID: {"c-chi": "khong-phai-so"}}
	uc2, ctx2 := newBudgetUseCase(t, k2)
	if _, err := uc2.RecordEntry(ctx2, sampleEntryRequest(), writer()); err == nil {
		t.Fatal("tổng đợt không phải số trên dòng `entries` mà vẫn ghi — phép kiểm trên xanh vô nghĩa")
	}
	if k2.commits != 0 {
		t.Fatalf("commit %d, muốn 0", k2.commits)
	}
}

// --- (6) the batch ceiling holds at the write --------------------------------------------------------------

func TestRecord2001stEntryIsRefusedAndWritesNothing(t *testing.T) {
	k := sampleStore()
	k.liveEntryCount = domain.MaxEntriesPerLine
	uc, ctx := newBudgetUseCase(t, k)

	_, err := uc.RecordEntry(ctx, sampleEntryRequest(), writer())
	if !errors.Is(err, domain.ErrLineEntriesFull) {
		t.Fatalf("đợt thứ %d = %v, muốn ErrKhoanMucDaDuDot", domain.MaxEntriesPerLine+1, err)
	}
	if k.hasStmt("INSERT INTO dot_thu_chi") || k.hasStmt("INSERT INTO gia_tri_dot") || k.hasStmt("INSERT INTO audit_log") {
		t.Fatal("vượt trần đợt mà vẫn ghi")
	}
	if k.commits != 0 {
		t.Fatalf("commit %d, muốn 0", k.commits)
	}
	// Counted UNDER the sheet lock, so two writers cannot both see 1999.
	lock, count0 := k.stmtIndex("FOR UPDATE"), k.stmtIndex("count(*) FROM dot_thu_chi")
	if lock < 0 || count0 < 0 || lock > count0 {
		t.Fatal("đếm đợt trước khi khoá bảng")
	}
	count := k.stmtsContaining("count(*) FROM dot_thu_chi")
	if len(count) != 1 || !strings.Contains(count[0].sql, "deleted_at IS NULL") || !hasArg(count[0], otherLineID) {
		t.Fatalf("câu đếm đợt sai: %+v", count)
	}

	// The 2000th is still accepted.
	k2 := sampleStore()
	k2.liveEntryCount = domain.MaxEntriesPerLine - 1
	uc2, ctx2 := newBudgetUseCase(t, k2)
	if _, err := uc2.RecordEntry(ctx2, sampleEntryRequest(), writer()); err != nil {
		t.Fatalf("đợt thứ %d bị từ chối: %v", domain.MaxEntriesPerLine, err)
	}
}

// --- (7) the new ceiling on a typed cell ----------------------------------------------------------------------

func TestTypingPastNewCeilingIsRefusedBeforeTransaction(t *testing.T) {
	k := sampleStore()
	uc, ctx := newBudgetUseCase(t, k)
	_, err := uc.UpdateLine(ctx, sampleLineID,
		UpdateBudgetLineRequest{Values: map[string]*domain.Dong{"c-chi": dongPtr(int64(domain.ValueMax) + 1)}}, writer())
	if !errors.Is(err, domain.ErrValueTooLarge) {
		t.Fatalf("= %v, muốn ErrGiaTriQuaLon", err)
	}
	if k.hasStmt("INSERT INTO gia_tri_khoan_muc") || k.commits != 0 {
		t.Fatal("số vượt trần mà vẫn ghi")
	}
}
