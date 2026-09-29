package app

import (
	"context"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/domain"
	docstore "github.com/vihat/vigov/service-finance/internal/store"
)

// WHAT THIS FILE PROVES. These are the first business WRITE routes in the repository, so the
// invariants below are being asserted here for the first time and each of them fails SILENTLY:
//
//	rule 6, invariant 3   the business write and its audit entry share ONE transaction
//	rule 6, invariant 2   who · what · on which record · from which IP · in which commune
//	rule 7, invariant 1   a delete is a SOFT delete, with all three columns
//	rule 7, invariant 3   a code, once issued, is never reissued — the duplicate check counts
//	                      soft-deleted rows
//	ADR 0024              tier 2 and 3 cannot be deleted, tier 3 cannot be disabled
//	rule 1, invariant 4   the commune comes from the context, and every statement carries it
//
// The tier rules are ALSO enforced by a database trigger, which is the floor that holds against
// every writer. What is asserted here is that this service refuses FIRST, in a sentence somebody can
// act on, and — more importantly — that a refusal commits nothing.

func testActor() audit.Actor {
	// `CB-00123` AND NOT A ULID. `audit_log.actor_id` holds the BUSINESS CODE for a staff
	// actor (rule 6, invariant 2), so a fixture handing the use case an internal id builds a
	// world that does not exist and then asserts things about it. It held
	// "CB-00123" until 2026-09-22, which is the same day the HTTP layer was
	// found writing exactly that value for real.
	return audit.Actor{ID: "CB-00123", Kind: "staff", IP: "10.0.0.7"}
}

func sampleCreate() CreateCapitalPlanCategoryRequest {
	return CreateCapitalPlanCategoryRequest{Code: "bao-cao", Label: "Báo cáo", SortOrder: 5}
}

// tier1Row is a row the commune added itself — tier 1, the only tier that may be deleted.
func tier1Row() *categoryRow {
	return &categoryRow{id: "hm-001", code: "bao-cao", label: "Báo cáo", isActive: true,
		sortOrder: 5, source: domain.SourceCommune}
}

// tier2Row ships with the software: it may be disabled and relabelled, never deleted.
func tier2Row() *categoryRow {
	return &categoryRow{id: "hm-002", code: "cong-van", label: "Công văn", isActive: true,
		sortOrder: 1, source: domain.SourceSystem}
}

// tier3Row is a system row the SOURCE CODE BRANCHES ON. Relabelling is all that is left.
func tier3Row() *categoryRow {
	return &categoryRow{id: "hm-003", code: "quyet-dinh", label: "Quyết định", isActive: true,
		sortOrder: 2, source: domain.SourceSystem, branched: true}
}

// --- rule 6, invariant 3: ONE transaction --------------------------------------------------------

func TestCreateWritesRowAndAuditInTheSAMETransaction(t *testing.T) {
	// THE INVARIANT THIS WHOLE ARCHITECTURE EXISTS FOR. The measured defect on the previous system
	// was zero transactions across the entire backend, so "every write leaves a trail" could not
	// hold: there was always a window where the record had changed and the trail had not.
	//
	// Asserted three ways, because each one alone can be satisfied by the wrong code:
	//   ONE BeginTx      — two would mean two transactions, which is the defect itself
	//   ONE Commit       — and no Rollback
	//   both statements  — the catalogue INSERT and the audit INSERT, inside that one transaction
	k := &fakeStore{}
	uc, ctx := newCategoryUseCase(t, k)

	if _, err := uc.CreateCategory(ctx, sampleCreate(), testActor()); err != nil {
		t.Fatalf("Them lỗi: %v", err)
	}

	if k.begins != 1 {
		t.Errorf("mở %d giao dịch, muốn 1 — hai giao dịch là đúng khiếm khuyết luật 6 bất biến 3 cấm", k.begins)
	}
	if k.commits != 1 || k.rollbacks != 0 {
		t.Errorf("commit=%d rollback=%d, muốn 1/0", k.commits, k.rollbacks)
	}
	if !k.hasStmt("INSERT INTO hang_muc_ke_hoach_von") {
		t.Error("không có câu chèn dòng danh mục")
	}
	if !k.hasStmt("INSERT INTO audit_log") {
		t.Fatal("KHÔNG CÓ VẾT KIỂM TOÁN — một thay đổi không ai truy được là thứ luật 6 cấm")
	}
}

func TestCreateAuditCarriesActorActionAndCode(t *testing.T) {
	// Rule 6, invariant 2: who · what · on which record · when · from which IP · in which commune.
	// The subject is the BUSINESS code and never the internal ULID — a trail nobody can match to
	// the row in front of them answers nothing.
	k := &fakeStore{}
	uc, ctx := newCategoryUseCase(t, k)

	if _, err := uc.CreateCategory(ctx, sampleCreate(), testActor()); err != nil {
		t.Fatalf("Them lỗi: %v", err)
	}

	audit := k.stmtsContaining("INSERT INTO audit_log")
	if len(audit) != 1 {
		t.Fatalf("ghi %d vết, muốn 1", len(audit))
	}
	args := stringArgs(audit[0].args)
	for _, want := range []string{string(tenantA), "CB-00123", "staff", "10.0.0.7",
		ActionCreateCapitalPlanCategory, "bao-cao"} {
		if !args[want] {
			t.Errorf("vết thiếu %q — đối số: %v", want, audit[0].args)
		}
	}
	// The internal id must NOT be the subject. It is allowed to appear nowhere in the entry.
	if args["01JIDMOICUADONGVUATAO0000"] {
		t.Error("vết lấy mã nội bộ làm chủ thể thay vì mã nghiệp vụ")
	}
}

func TestCreateWritesNothingWhenAuditFails(t *testing.T) {
	// THE DIRECTION THAT MATTERS MOST. If the audit entry fails, the row must not exist either:
	// a catalogue row nobody can attribute is precisely the state the records rules do not permit.
	//
	// The failure is injected on the LAST statement, so everything before it has already run — which
	// is the only way to tell "rolled back" from "never started".
	k := &fakeStore{failOnSQL: "INSERT INTO audit_log"}
	uc, ctx := newCategoryUseCase(t, k)

	if _, err := uc.CreateCategory(ctx, sampleCreate(), testActor()); err == nil {
		t.Fatal("vết hỏng mà Them vẫn báo thành công")
	}
	if k.commits != 0 {
		t.Error("commit dù vết không ghi được")
	}
	if k.rollbacks != 1 {
		t.Errorf("rollback=%d, muốn 1", k.rollbacks)
	}
}

// --- ADR 0024: the three tiers -------------------------------------------------------------------

func TestDeleteOnlyTier1(t *testing.T) {
	for name, tc := range map[string]struct {
		row     *categoryRow
		wantErr error
	}{
		"tầng 1 xoá được":       {tier1Row(), nil},
		"tầng 2 không xoá được": {tier2Row(), domain.ErrSystemRowNotDeletable},
		"tầng 3 không xoá được": {tier3Row(), domain.ErrSystemRowNotDeletable},
	} {
		t.Run(name, func(t *testing.T) {
			k := &fakeStore{row: tc.row}
			uc, ctx := newCategoryUseCase(t, k)

			err := uc.DeleteCategory(ctx, tc.row.id, "gộp vào loại khác", testActor())
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("Xoá lỗi: %v", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("lỗi = %v, muốn %v", err, tc.wantErr)
			}
			// A REFUSAL WRITES NOTHING. Not the row, and — just as important — not an audit entry
			// saying somebody deleted something they did not delete.
			if k.hasStmt("UPDATE hang_muc_ke_hoach_von") {
				t.Error("từ chối rồi mà vẫn chạy câu cập nhật")
			}
			if k.hasStmt("INSERT INTO audit_log") {
				t.Error("từ chối rồi mà vẫn ghi vết — vết nói một việc chưa hề xảy ra")
			}
		})
	}
}

func TestDisableIsRefusedOnlyOnTier3(t *testing.T) {
	// The operation the specification allows and the system cannot survive: disabling a code the
	// source code branches on leaves that branch with no reachable row, and the screen offering the
	// button reports nothing wrong (open question #21 describes the same shape for task statuses).
	bad := false
	for name, tc := range map[string]struct {
		row     *categoryRow
		allowed bool
	}{
		"tầng 1 tắt được":       {tier1Row(), true},
		"tầng 2 tắt được":       {tier2Row(), true},
		"tầng 3 không tắt được": {tier3Row(), false},
	} {
		t.Run(name, func(t *testing.T) {
			k := &fakeStore{row: tc.row}
			uc, ctx := newCategoryUseCase(t, k)

			_, err := uc.UpdateCategory(ctx, tc.row.id, UpdateCapitalPlanCategoryRequest{IsActive: &bad}, testActor())
			if tc.allowed {
				if err != nil {
					t.Fatalf("tắt lỗi: %v", err)
				}
				if !k.hasStmt("UPDATE hang_muc_ke_hoach_von") || !k.hasStmt("INSERT INTO audit_log") {
					t.Error("tắt thành công mà thiếu câu cập nhật hoặc vết")
				}
				return
			}
			if !errors.Is(err, domain.ErrBranchedRowNotDisablable) {
				t.Fatalf("lỗi = %v, muốn ErrKhongTatDuocMucReNhanh", err)
			}
			if k.hasStmt("UPDATE hang_muc_ke_hoach_von") || k.hasStmt("INSERT INTO audit_log") {
				t.Error("từ chối rồi mà vẫn ghi")
			}
		})
	}
}

func TestRelabelWorksOnEveryTier(t *testing.T) {
	// RELABELLING IS THE ONE OPERATION ALL THREE TIERS ALLOW, and at tier 3 it is the only one left.
	// The wording on a screen belongs to the commune; the code does not. A guard that refused this
	// would leave a commune unable to correct a misspelt Vietnamese label on its own screens.
	label := "Quyết định (sửa)"
	for name, row := range map[string]*categoryRow{
		"tầng 1": tier1Row(), "tầng 2": tier2Row(), "tầng 3": tier3Row(),
	} {
		t.Run(name, func(t *testing.T) {
			k := &fakeStore{row: row}
			uc, ctx := newCategoryUseCase(t, k)

			after, err := uc.UpdateCategory(ctx, row.id, UpdateCapitalPlanCategoryRequest{Label: &label}, testActor())
			if err != nil {
				t.Fatalf("đổi nhãn lỗi: %v", err)
			}
			if after.Label != label {
				t.Errorf("nhãn sau = %q, muốn %q", after.Label, label)
			}
			if !k.hasStmt("INSERT INTO audit_log") {
				t.Error("đổi nhãn mà không để lại vết")
			}
		})
	}
}

// --- what the commune may never supply -----------------------------------------------------------

func TestInsertHasNoParameterForSource(t *testing.T) {
	// THE SECURITY PROPERTY, ASSERTED ON THE STATEMENT ITSELF. `nguon` and `ma_nguon_re_nhanh`
	// decide which tier a row is in, and the migration says what a writable `nguon` would cost:
	// "every guard below could be stepped around by setting nguon = 'don-vi' first".
	//
	// So the INSERT must carry them as LITERALS and bind neither. A `$8` appearing here would mean
	// a value travelled from somewhere — and the only somewhere above this line is a request.
	k := &fakeStore{}
	uc, ctx := newCategoryUseCase(t, k)

	if _, err := uc.CreateCategory(ctx, sampleCreate(), testActor()); err != nil {
		t.Fatalf("Them lỗi: %v", err)
	}
	insert := k.stmtsContaining("INSERT INTO hang_muc_ke_hoach_von")
	if len(insert) != 1 {
		t.Fatalf("chạy %d câu chèn, muốn 1", len(insert))
	}
	if !strings.Contains(insert[0].sql, "'don-vi'") {
		t.Errorf("`nguon` không phải hằng trong câu lệnh: %q", insert[0].sql)
	}
	// Seven bound parameters: tenant_id, id, ma, nhan, thu_tu, la_mac_dinh, dang_dung. Nothing for
	// `nguon`, nothing for `ma_nguon_re_nhanh`.
	if len(insert[0].args) != 7 {
		t.Errorf("câu chèn nhận %d tham số, muốn 7 — thêm một tham số là thêm một đường cho client",
			len(insert[0].args))
	}
	for _, a := range insert[0].args {
		if s, ok := a.(string); ok && (s == domain.SourceSystem || s == domain.SourceCommune) {
			t.Errorf("`nguon` đi vào câu lệnh như một THAM SỐ: %v", insert[0].args)
		}
	}
}

func TestUpdateNeverTouchesCodeOrSource(t *testing.T) {
	// `ma` because an issued code is never renumbered (rule 7, invariant 3) — document records hold
	// it as a value and nothing rewrites them. `nguon` and `ma_nguon_re_nhanh` because they decide
	// the tier. All three are refused by the trigger too; their ABSENCE here is what makes that
	// refusal unreachable from this service in the first place.
	label := "Công văn mới"
	k := &fakeStore{row: tier2Row()}
	uc, ctx := newCategoryUseCase(t, k)

	if _, err := uc.UpdateCategory(ctx, "hm-002", UpdateCapitalPlanCategoryRequest{Label: &label}, testActor()); err != nil {
		t.Fatalf("Sua lỗi: %v", err)
	}
	update := k.stmtsContaining("UPDATE hang_muc_ke_hoach_von")
	if len(update) != 1 {
		t.Fatalf("chạy %d câu cập nhật, muốn 1", len(update))
	}
	for _, column := range []string{"ma =", "nguon =", "ma_nguon_re_nhanh ="} {
		if strings.Contains(update[0].sql, column) {
			t.Errorf("câu cập nhật đụng tới `%s`: %q", strings.TrimSuffix(column, " ="), update[0].sql)
		}
	}
}

// --- rule 7: soft delete, and a code that stays taken ---------------------------------------------

func TestDeleteIsSoftDeleteAndWritesAllThreeColumns(t *testing.T) {
	// Rule 7, invariant 1 names THREE columns, and the reason all three are asserted is that a row
	// which vanished from every screen with no reason attached is a row nobody can explain when
	// somebody asks why a document type disappeared — and the row is still there, so the question
	// WILL be asked.
	k := &fakeStore{row: tier1Row()}
	uc, ctx := newCategoryUseCase(t, k)

	if err := uc.DeleteCategory(ctx, "hm-001", "gộp vào loại khác", testActor()); err != nil {
		t.Fatalf("Xoá lỗi: %v", err)
	}
	// NOT A DELETE. `DELETE FROM` anywhere on this path is rule 7, forbidden #1, and the trigger
	// refuses it — but the statement must never be written in the first place.
	if k.hasStmt("DELETE FROM") {
		t.Fatal("XOÁ CỨNG trên dữ liệu nghiệp vụ — luật 7 cấm #1")
	}
	del := k.stmtsContaining("deleted_at = now()")
	if len(del) != 1 {
		t.Fatalf("chạy %d câu xoá mềm, muốn 1", len(del))
	}
	for _, column := range []string{"deleted_at", "deleted_by", "delete_reason"} {
		if !strings.Contains(del[0].sql, column) {
			t.Errorf("câu xoá mềm thiếu cột `%s`: %q", column, del[0].sql)
		}
	}
	args := stringArgs(del[0].args)
	if !args["gộp vào loại khác"] || !args["CB-00123"] {
		t.Errorf("xoá mềm không ghi ai xoá và vì sao: %v", del[0].args)
	}
}

func TestEveryWritePathReadExcludesSoftDeletedRows(t *testing.T) {
	// RULE 7, INVARIANT 2 — "EVERYWHERE, ALWAYS", and the write path is where "everywhere" is
	// easiest to forget, because a soft-deleted row is invisible on the screen so nobody tries.
	//
	// Two statements, two different consequences of dropping the predicate:
	//
	//	the FOR UPDATE read  — an edit would RESURRECT a deleted row: it would come back on every
	//	                       screen, under a code that is still recorded as withdrawn;
	//	the soft delete      — a second delete would OVERWRITE `deleted_by` and `delete_reason`,
	//	                       which is editing a historical record (rule 7, forbidden #5). The
	//	                       first deletion is the one that happened.
	//
	// Asserted on the SQL because the fake driver has no notion of a deleted row: what can be
	// checked without a PostgreSQL is that the statement ASKS for it, and that is the half that gets
	// deleted while tidying a query.
	k := &fakeStore{row: tier1Row()}
	uc, ctx := newCategoryUseCase(t, k)

	if err := uc.DeleteCategory(ctx, "hm-001", "gộp vào loại khác", testActor()); err != nil {
		t.Fatalf("Xoá lỗi: %v", err)
	}
	read := k.stmtsContaining("FOR UPDATE")
	if len(read) != 1 {
		t.Fatalf("chạy %d câu đọc-để-sửa, muốn 1", len(read))
	}
	if !strings.Contains(read[0].sql, "deleted_at IS NULL") {
		t.Errorf("câu đọc-để-sửa KHÔNG loại dòng đã xoá — sửa một dòng đã xoá sẽ làm nó sống lại: %q",
			read[0].sql)
	}
	del := k.stmtsContaining("deleted_at = now()")
	if !strings.Contains(del[0].sql, "deleted_at IS NULL") {
		t.Errorf("câu xoá mềm KHÔNG loại dòng đã xoá — lần xoá thứ hai ghi đè người xoá và lý do: %q",
			del[0].sql)
	}
	// The same predicate on the UPDATE that edits a row, for the first of the two reasons above.
	label := "Báo cáo mới"
	k2 := &fakeStore{row: tier1Row()}
	uc2, ctx2 := newCategoryUseCase(t, k2)
	if _, err := uc2.UpdateCategory(ctx2, "hm-001", UpdateCapitalPlanCategoryRequest{Label: &label}, testActor()); err != nil {
		t.Fatalf("Sua lỗi: %v", err)
	}
	for _, l := range k2.stmtsContaining("UPDATE hang_muc_ke_hoach_von") {
		if !strings.Contains(l.sql, "deleted_at IS NULL") {
			t.Errorf("câu ghi không loại dòng đã xoá: %q", l.sql)
		}
	}
}

func TestUpdateDeletedRowIs404(t *testing.T) {
	// The other half of the same rule, from the caller's side: a row the FOR UPDATE read does not
	// find is "not there", and every path answers that the same way. `row: nil` is how the fake
	// says the predicate excluded it.
	label := "Báo cáo mới"
	k := &fakeStore{row: nil}
	uc, ctx := newCategoryUseCase(t, k)

	if _, err := uc.UpdateCategory(ctx, "hm-001", UpdateCapitalPlanCategoryRequest{Label: &label}, testActor()); !errors.Is(err, docstore.ErrCatalogueRowNotFound) {
		t.Fatalf("lỗi = %v, muốn ErrDanhMucKhongTonTai", err)
	}
	if k.hasStmt("UPDATE hang_muc_ke_hoach_von") || k.hasStmt("INSERT INTO audit_log") {
		t.Error("không tìm thấy dòng mà vẫn ghi")
	}
	if k.commits != 0 || k.rollbacks != 1 {
		t.Errorf("commit=%d rollback=%d, muốn 0/1", k.commits, k.rollbacks)
	}
}

func TestDeleteRequiresReason(t *testing.T) {
	// A soft delete with no reason is refused BEFORE the transaction opens. `delete_reason` is not
	// decoration: it is the only thing that will ever answer "why is this type gone".
	k := &fakeStore{row: tier1Row()}
	uc, ctx := newCategoryUseCase(t, k)

	if err := uc.DeleteCategory(ctx, "hm-001", "   ", testActor()); !errors.Is(err, domain.ErrDeleteReasonMissing) {
		t.Fatalf("lỗi = %v, muốn ErrThieuLyDoXoa", err)
	}
	if k.begins != 0 {
		t.Error("mở giao dịch cho một yêu cầu đã sai hình dạng — giữ khoá dòng mà không cần")
	}
}

func TestCreateRefusesUsedCodeIncludingDeletedRows(t *testing.T) {
	// THE DUPLICATE CHECK COUNTS SOFT-DELETED ROWS, and that single predicate decides whether old
	// documents can still be read: a commune that could soft-delete `cong-van` and create a new,
	// unrelated `cong-van` would silently change the type shown on every document already
	// registered under the old one — and numbering follows the type (ADR 0024).
	k := &fakeStore{codeTaken: 1}
	uc, ctx := newCategoryUseCase(t, k)

	_, err := uc.CreateCategory(ctx, sampleCreate(), testActor())
	if !errors.Is(err, docstore.ErrCodeTaken) {
		t.Fatalf("lỗi = %v, muốn ErrMaDaTonTai", err)
	}
	if k.hasStmt("INSERT INTO") {
		t.Error("mã trùng mà vẫn chèn")
	}
	// The statement that asked must NOT exclude deleted rows. Asserted on the SQL because this is
	// exactly the predicate somebody "fixes" while tidying up a query.
	query := k.stmtsContaining("count(*)")
	var check string
	for _, l := range query {
		if strings.Contains(l.sql, "ma = $2") {
			check = l.sql
		}
	}
	if check == "" {
		t.Fatal("không có câu kiểm mã trùng")
	}
	if strings.Contains(check, "deleted_at") {
		t.Errorf("câu kiểm mã trùng LOẠI dòng đã xoá — mã đã cấp sẽ được cấp lại: %q", check)
	}
}

func TestCreateRefusesWhenCeilingReached(t *testing.T) {
	// A CREATE HAS TO CARE ABOUT THE READ ROUTE'S CEILING. List REFUSES rather than truncates
	// past MaxCapitalPlanCategories, so the row that crosses the line does not merely add itself — it
	// turns the whole catalogue into a 500 for every screen in the commune.
	k := &fakeStore{count: docstore.MaxCapitalPlanCategories}
	uc, ctx := newCategoryUseCase(t, k)

	_, err := uc.CreateCategory(ctx, sampleCreate(), testActor())
	if !errors.Is(err, docstore.ErrCatalogueFull) {
		t.Fatalf("lỗi = %v, muốn ErrDanhMucDayTran", err)
	}
	if k.hasStmt("INSERT INTO") {
		t.Error("đầy trần mà vẫn chèn")
	}

	// One row below the ceiling is accepted. An off-by-one here refuses a commune whose data is
	// perfectly valid.
	k2 := &fakeStore{count: docstore.MaxCapitalPlanCategories - 1}
	uc2, ctx2 := newCategoryUseCase(t, k2)
	if _, err := uc2.CreateCategory(ctx2, sampleCreate(), testActor()); err != nil {
		t.Fatalf("dưới trần mà bị từ chối: %v", err)
	}
}

// --- idempotent edit -------------------------------------------------------------------------------

func TestUpdateWithNoChangeWritesNothingAndNoAudit(t *testing.T) {
	// THE PROPERTY THE ROUTE'S idem.KhongCan DECLARATION RESTS ON. Sending the label a row already
	// has is not an event; recording it would fill a public authority's ledger with entries saying
	// nothing changed, and those are the entries that bury the ones carrying legal weight.
	//
	// Delete the comparison in UpdateCategory and this goes red — which is the point, because the route would
	// then be claiming an idempotency it no longer has.
	label := "Báo cáo" // exactly what tier1Row() already holds
	k := &fakeStore{row: tier1Row()}
	uc, ctx := newCategoryUseCase(t, k)

	after, err := uc.UpdateCategory(ctx, "hm-001", UpdateCapitalPlanCategoryRequest{Label: &label}, testActor())
	if err != nil {
		t.Fatalf("Sua lỗi: %v", err)
	}
	if after.Label != "Báo cáo" {
		t.Errorf("trả về sai: %+v", after)
	}
	if k.hasStmt("UPDATE hang_muc_ke_hoach_von") {
		t.Error("không có gì đổi mà vẫn chạy câu cập nhật")
	}
	if k.hasStmt("INSERT INTO audit_log") {
		t.Error("không có gì đổi mà vẫn ghi vết — vết nói một việc chưa hề xảy ra")
	}
}

func TestUpdateSetDefaultClearsOldDefaultFirstInSameTransaction(t *testing.T) {
	// `UNIQUE (tenant_id, moc_mac_dinh)` admits exactly ONE live default, so setting a new one
	// without clearing the old one fails the constraint. Doing it in a SECOND transaction would
	// leave a window where the commune has no default at all and every form pre-selects nothing.
	active := true
	k := &fakeStore{row: tier1Row()}
	uc, ctx := newCategoryUseCase(t, k)

	if _, err := uc.UpdateCategory(ctx, "hm-001", UpdateCapitalPlanCategoryRequest{IsDefault: &active}, testActor()); err != nil {
		t.Fatalf("Sua lỗi: %v", err)
	}
	if !k.hasStmt("la_mac_dinh = false") {
		t.Fatal("đặt mặc định mới mà không bỏ mặc định cũ — khoá duy nhất sẽ từ chối")
	}
	if k.begins != 1 {
		t.Errorf("mở %d giao dịch, muốn 1", k.begins)
	}
}

// --- rule 1: the commune is in every statement, and comes from the context -------------------------

func TestEveryStatementCarriesTenantFromContext(t *testing.T) {
	// Rule 1, invariants 4 and 5. The commune is not a parameter of any method on this use case and
	// cannot be: it arrives in the context, Scoped binds it to $1, and a store built without one
	// does not exist. Asserted on EVERY statement rather than on one, because the leak is whichever
	// statement forgot.
	for _, tenantID := range []string{string(tenantA), string(tenantB)} {
		k := &fakeStore{}
		uc, _ := newCategoryUseCase(t, k)
		ctx := tenantCtx(tenantID)
		if _, err := uc.CreateCategory(ctx, sampleCreate(), testActor()); err != nil {
			t.Fatalf("Them lỗi: %v", err)
		}
		for _, l := range k.stmts {
			if len(l.args) == 0 || l.args[0] != tenantID {
				t.Errorf("câu lệnh %q chạy với $1 = %v, muốn xã %q", l.sql, l.args, tenantID)
			}
		}
	}
}

func TestNoTenantInContextPanics(t *testing.T) {
	// FAIL CLOSED, LOUDLY. A write that ran without a commune would file a catalogue row — and its
	// audit entry — in nobody's archive, or in everybody's. tenant.MustFrom panics by design and
	// httpx.Recover turns that into a traceable 500 at the edge. What must never happen is a
	// default commune (rule 1, forbidden #1).
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("ghi danh mục khi context không có xã mà không panic")
		}
	}()
	k := &fakeStore{}
	uc, _ := newCategoryUseCase(t, k)
	_, _ = uc.CreateCategory(context.Background(), sampleCreate(), testActor())
}

// --- helpers ----------------------------------------------------------------------------------------

func tenantCtx(tenantID string) context.Context {
	return tenant.Into(context.Background(), tenant.ID(tenantID))
}

// stringArgs indexes a statement's arguments by their string form, so an assertion names the
// VALUE it expects instead of a position that shifts whenever a column is added.
func stringArgs(args []driver.Value) map[string]bool {
	result := map[string]bool{}
	for _, a := range args {
		switch v := a.(type) {
		case string:
			result[v] = true
		case []byte:
			// The audit delta arrives as JSON bytes. Indexed whole so a test can look for a field
			// name inside it without re-parsing.
			result[string(v)] = true
		}
	}
	return result
}
