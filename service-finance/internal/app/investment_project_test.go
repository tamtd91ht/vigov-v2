package app

// The invariants of the investment project WRITE path (docs/ui-ux/06-giai-ngan.md §9, §8, §13).
//
// EVERY CASE HERE RUNS THE REAL USE CASE OVER THE REAL STORE over the fake driver of
// fake_driver_investment_project_test.go. What is being proved is the SQL and the transaction boundaries — a
// hand-written fake store would erase exactly those.
//
// WHAT IS NOT PROVED HERE, so nobody reads more into a green run than is there: nothing PostgreSQL
// does. `UNIQUE (tenant_id, ma)`, `ho_so_luu_tru_cam_xoa_cung`, `CHECK (ke_hoach_von_nam >= 0)` and
// the partition routing are the FLOOR under all of this, and they need a real server.

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// testStaff is the acting principal. `CB-…` AND NEVER A ULID — rule 6, invariant 8: `audit_log.actor_id`
// is read years later by somebody handling an inspection, and a business code names a person with no
// lookup still alive while a ULID names nobody.
var testStaff = audit.Actor{ID: "CB-2026-7K3M9Q", Kind: "staff", IP: "10.0.0.7"}

func validCreateInvestmentProject() CreateInvestmentProjectRequest {
	return CreateInvestmentProjectRequest{
		Code:          "DA-2026-be-tong-hoa-duong-ngo-xom",
		Year:          2026,
		CategoryID:    "hm-chuyen-tiep",
		Name:          "Bê tông hoá đường ngõ xóm tổ 6",
		PlannedAmount: 100_000_000,
	}
}

func readyInvestmentProjectStore() *fakeInvestmentProjectStore {
	return &fakeInvestmentProjectStore{liveCategories: map[string]bool{"hm-chuyen-tiep": true}}
}

// --- creating ----------------------------------------------------------------------------------

// The invariant rule 6, invariant 3 exists for: the project, its allocation lines and the audit
// entry are ONE transaction. Not "the write, then the entry if it works".
func TestCreateInvestmentProjectWritesProjectAllocationsAndAuditInOneTransaction(t *testing.T) {
	k := readyInvestmentProjectStore()
	k.liveFundingSources = map[string]bool{"nv-xa|2026": true, "nv-thanh-pho|2026": true}
	uc, ctx := newInvestmentProjectUseCase(t, k)

	req := validCreateInvestmentProject()
	req.Allocations = []domain.NewAllocationLine{
		{FundingSourceID: "nv-xa", Amount: 40_000_000},
		{FundingSourceID: "nv-thanh-pho", Amount: 60_000_000},
	}

	res, err := uc.CreateInvestmentProject(ctx, req, testStaff)
	if err != nil {
		t.Fatalf("Them: %v", err)
	}

	if k.begins != 1 {
		t.Errorf("mở %d giao dịch, muốn đúng 1 — dự án, phân bổ và vết phải cùng một giao dịch", k.begins)
	}
	if k.commits != 1 || k.rollbacks != 0 {
		t.Errorf("commit=%d rollback=%d, muốn commit=1 rollback=0", k.commits, k.rollbacks)
	}
	if !k.hasStmt("INSERT INTO du_an") {
		t.Error("không thấy câu chèn dự án")
	}
	if n := len(k.stmtsContaining("INSERT INTO phan_bo_nguon_von")); n != 2 {
		t.Errorf("chèn %d dòng phân bổ, muốn 2", n)
	}
	if !k.hasStmt("INSERT INTO audit_log") {
		t.Error("không thấy vết kiểm toán")
	}
	if res.InvestmentProject.ID != newInvestmentProjectID {
		t.Errorf("id dự án = %q, muốn %q", res.InvestmentProject.ID, newInvestmentProjectID)
	}
	// EVERY ROW GETS ITS OWN ID. A single id shared between `du_an` and `phan_bo_nguon_von` is
	// something the separate PRIMARY KEYs permit and nothing would notice until somebody joined them.
	distinct := map[string]bool{res.InvestmentProject.ID: true}
	for _, pb := range res.Allocations {
		if distinct[pb.ID] {
			t.Errorf("id %q bị dùng lại cho dòng phân bổ", pb.ID)
		}
		distinct[pb.ID] = true
		if pb.InvestmentProjectID != res.InvestmentProject.ID {
			t.Errorf("dòng phân bổ trỏ vào dự án %q, muốn %q", pb.InvestmentProjectID, res.InvestmentProject.ID)
		}
	}
}

// Rule 6, invariant 8: the trail's subject is the BUSINESS CODE, never the internal id. A ULID names
// nobody to somebody handling an inspection years later, and the row it points at may by then be
// gone. `tools/check_audit_actor.py` scans the whole repository for the same defect.
func TestCreateInvestmentProjectAuditsByProjectCodeAndStaffCode(t *testing.T) {
	k := readyInvestmentProjectStore()
	uc, ctx := newInvestmentProjectUseCase(t, k)

	req := validCreateInvestmentProject()
	if _, err := uc.CreateInvestmentProject(ctx, req, testStaff); err != nil {
		t.Fatalf("Them: %v", err)
	}

	audit := k.stmtsContaining("INSERT INTO audit_log")
	if len(audit) != 1 {
		t.Fatalf("có %d vết, muốn 1", len(audit))
	}
	args := audit[0].args
	if got := args[1]; got != testStaff.ID {
		t.Errorf("actor_id = %v, muốn mã cán bộ %q — không bao giờ là id nội bộ", got, testStaff.ID)
	}
	if got := args[4]; got != ActionCreateInvestmentProject {
		t.Errorf("action = %v, muốn %q", got, ActionCreateInvestmentProject)
	}
	if got := args[5]; got != req.Code {
		t.Errorf("subject = %v, muốn mã dự án %q — không bao giờ là ULID", got, req.Code)
	}
}

// §11's "mặc định 31/12", applied in ONE place. A commune that leaves the deadline blank must not
// end up with the zero time — which stores 01/01/0001 and makes every project on the screen overdue.
func TestCreateInvestmentProjectBlankDeadlineUsesDec31OfBudgetYear(t *testing.T) {
	k := readyInvestmentProjectStore()
	uc, ctx := newInvestmentProjectUseCase(t, k)

	res, err := uc.CreateInvestmentProject(ctx, validCreateInvestmentProject(), testStaff)
	if err != nil {
		t.Fatalf("Them: %v", err)
	}
	want := time.Date(2026, time.December, 31, 0, 0, 0, 0, time.UTC)
	if !res.InvestmentProject.DisbursementDeadline.Equal(want) {
		t.Errorf("thoi_han_giai_ngan = %v, muốn %v", res.InvestmentProject.DisbursementDeadline, want)
	}
}

// §9: "Để trống thì lấy bằng số tiền bố trí năm nay." The column goes to NULL and the RULE is read
// back out by domain.EffectiveApprovedAmount — a COPIED value would silently stop following the plan the day
// the plan is revised (0004:199-203).
func TestCreateInvestmentProjectBlankApprovedAmountWritesNULLNotPlanCopy(t *testing.T) {
	k := readyInvestmentProjectStore()
	uc, ctx := newInvestmentProjectUseCase(t, k)

	if _, err := uc.CreateInvestmentProject(ctx, validCreateInvestmentProject(), testStaff); err != nil {
		t.Fatalf("Them: %v", err)
	}
	insert := k.stmtsContaining("INSERT INTO du_an")
	if len(insert) != 1 {
		t.Fatalf("có %d câu chèn, muốn 1", len(insert))
	}
	// $9 is tong_muc_duoc_duyet — index 8 in the arg slice.
	if got := insert[0].args[8]; got != nil {
		t.Errorf("tong_muc_duoc_duyet = %v, muốn NULL khi xã để trống", got)
	}
}

// §9 makes the allocation list OPTIONAL and §11 names the state it produces (`Chưa gắn nguồn`). Every
// rule on this path has to survive a project with no allocation at all — §13 rule 6 says the same
// thing about vouchers.
func TestCreateInvestmentProjectWithoutFundingSourcesStillCreates(t *testing.T) {
	k := readyInvestmentProjectStore()
	uc, ctx := newInvestmentProjectUseCase(t, k)

	res, err := uc.CreateInvestmentProject(ctx, validCreateInvestmentProject(), testStaff)
	if err != nil {
		t.Fatalf("Them không khai nguồn vốn phải được: %v", err)
	}
	if len(res.Allocations) != 0 {
		t.Errorf("có %d dòng phân bổ, muốn 0", len(res.Allocations))
	}
	if k.hasStmt("INSERT INTO phan_bo_nguon_von") {
		t.Error("không khai nguồn nào mà vẫn chèn dòng phân bổ")
	}
	if k.commits != 1 {
		t.Errorf("commit=%d, muốn 1", k.commits)
	}
}

// ⚠ §9 SAYS THE MISMATCH IS A WARNING, NOT A REFUSAL: the system "đối chiếu tổng các nguồn với số ấy
// và CẢNH BÁO khi thiếu hoặc vượt". This is the case that stops a future reader from turning that
// sentence into a constraint — both directions, short AND over, must be accepted and written.
func TestCreateInvestmentProjectSourceTotalMismatchStillCreatesBecauseOnlyAWarning(t *testing.T) {
	for _, tc := range []struct {
		name   string
		amount domain.Dong
	}{
		{"thiếu so với kế hoạch", 10_000_000},
		{"vượt kế hoạch", 500_000_000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k := readyInvestmentProjectStore()
			k.liveFundingSources = map[string]bool{"nv-xa|2026": true}
			uc, ctx := newInvestmentProjectUseCase(t, k)

			req := validCreateInvestmentProject() // kế hoạch 100.000.000
			req.Allocations = []domain.NewAllocationLine{{FundingSourceID: "nv-xa", Amount: tc.amount}}

			res, err := uc.CreateInvestmentProject(ctx, req, testStaff)
			if err != nil {
				t.Fatalf("lệch tổng nguồn KHÔNG được từ chối (§9 là cảnh báo): %v", err)
			}
			if len(res.Allocations) != 1 {
				t.Errorf("có %d dòng phân bổ, muốn 1", len(res.Allocations))
			}
			if k.commits != 1 {
				t.Errorf("commit=%d, muốn 1", k.commits)
			}
		})
	}
}

// §13 rule 8: each budget year is its own set. A source of the right commune but the WRONG year must
// be refused, or a 2026 project's allocation lands on a card §6 draws for 2027.
func TestCreateInvestmentProjectRefusesFundingSourceOfOtherBudgetYear(t *testing.T) {
	k := readyInvestmentProjectStore()
	k.liveFundingSources = map[string]bool{"nv-xa|2027": true} // live, but for 2027
	uc, ctx := newInvestmentProjectUseCase(t, k)

	req := validCreateInvestmentProject() // nam 2026
	req.Allocations = []domain.NewAllocationLine{{FundingSourceID: "nv-xa", Amount: 100_000_000}}

	_, err := uc.CreateInvestmentProject(ctx, req, testStaff)
	if !errors.Is(err, fistore.ErrAllocationFundingSourceNotFound) {
		t.Fatalf("lỗi = %v, muốn ErrKhongThayNguonVonPhanBo", err)
	}
	if k.commits != 0 {
		t.Errorf("commit=%d, muốn 0 — từ chối thì không ghi gì", k.commits)
	}
}

// §9: "Mã tự nhập phải chưa từng được dùng, KỂ CẢ BỞI DỰ ÁN ĐÃ RÚT KHỎI DANH SÁCH." The check must
// carry NO `deleted_at` predicate — rule 7, invariant 3: an issued code is never reissued.
func TestCreateInvestmentProjectCodeCheckIncludesSoftDeletedProjects(t *testing.T) {
	k := readyInvestmentProjectStore()
	k.codeUsedCount = 1 // the only row carrying this code is soft-deleted
	uc, ctx := newInvestmentProjectUseCase(t, k)

	_, err := uc.CreateInvestmentProject(ctx, validCreateInvestmentProject(), testStaff)
	if !errors.Is(err, fistore.ErrInvestmentProjectCodeTaken) {
		t.Fatalf("lỗi = %v, muốn ErrMaDuAnDaTonTai", err)
	}

	check := k.stmtsContaining("count(*) FROM du_an")
	if len(check) != 1 {
		t.Fatalf("có %d câu kiểm mã, muốn 1", len(check))
	}
	// THE ASSERTION THAT MATTERS. With `AND deleted_at IS NULL` in that statement, a withdrawn
	// project's code becomes available again — and the vouchers of the first project then read as
	// belonging to the second.
	if strings.Contains(check[0].sql, "deleted_at") {
		t.Errorf("câu kiểm mã trùng có lọc deleted_at — mã đã cấp thì không cấp lại: %q", check[0].sql)
	}
	if k.commits != 0 {
		t.Errorf("commit=%d, muốn 0", k.commits)
	}
}

// There is no foreign key under `du_an.hang_muc_id` (0004:182-190), so this check IS the constraint.
// A project classified under nothing is money counted in §3's KPI card and missing from every row of
// §5's table — two totals on one screen that disagree.
func TestCreateInvestmentProjectRefusesCategoryNotInTenant(t *testing.T) {
	k := readyInvestmentProjectStore()
	uc, ctx := newInvestmentProjectUseCase(t, k)

	req := validCreateInvestmentProject()
	req.CategoryID = "hm-cua-xa-khac"

	_, err := uc.CreateInvestmentProject(ctx, req, testStaff)
	if !errors.Is(err, fistore.ErrCategoryNotFound) {
		t.Fatalf("lỗi = %v, muốn ErrKhongThayHangMuc", err)
	}
	if k.hasStmt("INSERT INTO du_an") {
		t.Error("từ chối hạng mục mà vẫn chèn dự án")
	}
	if k.commits != 0 {
		t.Errorf("commit=%d, muốn 0", k.commits)
	}
}

// §9 offers `☑ Tự sinh mã` and this service does not generate one — the specification gives two
// incompatible formats and no scope for the sequence, and a project code is an ISSUED code. The
// refusal must be a 400-shaped domain error, not a 500 and not a silent blank code.
func TestCreateInvestmentProjectMissingCodeIsRefusedNotGenerated(t *testing.T) {
	k := readyInvestmentProjectStore()
	uc, ctx := newInvestmentProjectUseCase(t, k)

	req := validCreateInvestmentProject()
	req.Code = "   "

	_, err := uc.CreateInvestmentProject(ctx, req, testStaff)
	if !errors.Is(err, domain.ErrInvestmentProjectCodeMissing) {
		t.Fatalf("lỗi = %v, muốn ErrThieuMaDuAn", err)
	}
	if k.begins != 0 {
		t.Errorf("mở %d giao dịch, muốn 0 — hình dạng sai thì không được giữ khoá dòng", k.begins)
	}
}

// Rule 6: a business write whose trail cannot name its author is refused BEFORE the transaction
// opens. core/audit refuses an entry with no actor too; refusing here keeps the row and the entry
// telling the same story and avoids a rollback whose cause is a missing principal.
func TestCreateInvestmentProjectWithoutActorRefusedBeforeTransaction(t *testing.T) {
	k := readyInvestmentProjectStore()
	uc, ctx := newInvestmentProjectUseCase(t, k)

	if _, err := uc.CreateInvestmentProject(ctx, validCreateInvestmentProject(), audit.Actor{Kind: "staff"}); err == nil {
		t.Fatal("thiếu chủ thể mà vẫn ghi được")
	}
	if k.begins != 0 {
		t.Errorf("mở %d giao dịch, muốn 0", k.begins)
	}
}

// The entry is the LAST statement of the transaction, so a failure there must take the project and
// its allocation lines down with it. This is the shape rule 6, invariant 3 exists for, and it is why
// `failOnSQL` fails one statement rather than everything.
func TestCreateInvestmentProjectFailedAuditLeavesNoProject(t *testing.T) {
	k := readyInvestmentProjectStore()
	k.failOnSQL = "INSERT INTO audit_log"
	uc, ctx := newInvestmentProjectUseCase(t, k)

	if _, err := uc.CreateInvestmentProject(ctx, validCreateInvestmentProject(), testStaff); err == nil {
		t.Fatal("vết hỏng mà Them vẫn thành công")
	}
	if k.commits != 0 {
		t.Errorf("commit=%d, muốn 0 — dự án không được commit khi vết hỏng", k.commits)
	}
	if k.rollbacks != 1 {
		t.Errorf("rollback=%d, muốn 1", k.rollbacks)
	}
}

// --- editing -----------------------------------------------------------------------------------

func readyInvestmentProjectRow() *investmentProjectRow {
	return &investmentProjectRow{
		id:                   "01JDUANCU0000000000000000",
		code:                 "DA-2026-cu",
		year:                 2026,
		categoryID:           "hm-chuyen-tiep",
		name:                 "Dự án cũ",
		plan:                 100_000_000,
		disbursementDeadline: time.Date(2026, time.December, 31, 0, 0, 0, 0, time.UTC),
	}
}

// ⚠ RULE 7, FORBIDDEN #4 AND §13 RULE 8, PROVED AT THE STATEMENT. `ma` and `nam` must appear in NO
// update statement: a code that has been issued is never renumbered, and moving a project between
// budget years takes its whole plan and every voucher filed against it out of one year's totals.
// The domain refuses a body naming either; this proves the column is not even reachable.
func TestUpdateInvestmentProjectNeverWritesCodeOrYear(t *testing.T) {
	k := readyInvestmentProjectStore()
	k.row = readyInvestmentProjectRow()
	uc, ctx := newInvestmentProjectUseCase(t, k)

	name := "Dự án đã đổi tên"
	if _, err := uc.UpdateInvestmentProject(ctx, k.row.id, UpdateInvestmentProjectRequest{Name: &name}, testStaff); err != nil {
		t.Fatalf("Sua: %v", err)
	}

	update := k.stmtsContaining("UPDATE du_an")
	if len(update) != 1 {
		t.Fatalf("có %d câu cập nhật, muốn 1", len(update))
	}
	// A WORD BOUNDARY AND NOT strings.Contains, AND THE FIRST VERSION OF THIS ASSERTION WAS WRONG
	// BECAUSE OF IT: `ke_hoach_von_nam = $6` contains the substring "nam =", so a plain Contains
	// reported a correct statement as a violation. `\b` does not match between `_` and `n`, so
	// `\bnam\s*=` sees the column `nam` and never a column merely ending in it.
	for _, column := range []string{`\bma\s*=`, `\bnam\s*=`} {
		if regexp.MustCompile(column).MatchString(update[0].sql) {
			t.Errorf("câu cập nhật có cột %s — cột này không được sửa: %q", column, update[0].sql)
		}
	}
}

// A no-op writes NOTHING and audits NOTHING. That is what makes `idem.KhongCan` on the PATCH route a
// property rather than a hope, and it is what keeps a public authority's ledger free of entries
// saying nothing changed — the entries that bury the ones carrying legal weight.
func TestUpdateInvestmentProjectWithNoChangeWritesNothingAndNoAudit(t *testing.T) {
	k := readyInvestmentProjectStore()
	k.row = readyInvestmentProjectRow()
	uc, ctx := newInvestmentProjectUseCase(t, k)

	name := k.row.name // exactly what the row already holds
	if _, err := uc.UpdateInvestmentProject(ctx, k.row.id, UpdateInvestmentProjectRequest{Name: &name}, testStaff); err != nil {
		t.Fatalf("Sua: %v", err)
	}
	if k.hasStmt("UPDATE du_an") {
		t.Error("không có gì đổi mà vẫn ghi dòng")
	}
	if k.hasStmt("INSERT INTO audit_log") {
		t.Error("không có gì đổi mà vẫn ghi vết")
	}
}

// Rule 6, invariant 5: before AND after, and only the fields that moved. `ke_hoach_von_nam` is the
// field this delta exists for — it is the denominator of §3's delay score and §7.2's ratio, so
// revising it moves a project from "chậm" to "bám sát tiến độ" without one đồng having moved.
func TestUpdateInvestmentProjectAuditCarriesBeforeAndAfterOfPlannedAmount(t *testing.T) {
	k := readyInvestmentProjectStore()
	k.row = readyInvestmentProjectRow()
	uc, ctx := newInvestmentProjectUseCase(t, k)

	next := domain.Dong(250_000_000)
	if _, err := uc.UpdateInvestmentProject(ctx, k.row.id, UpdateInvestmentProjectRequest{PlannedAmount: &next}, testStaff); err != nil {
		t.Fatalf("Sua: %v", err)
	}

	audit := k.stmtsContaining("INSERT INTO audit_log")
	if len(audit) != 1 {
		t.Fatalf("có %d vết, muốn 1", len(audit))
	}
	if got := audit[0].args[5]; got != k.row.code {
		t.Errorf("subject = %v, muốn mã dự án %q", got, k.row.code)
	}

	var body struct {
		Before map[string]any `json:"truoc"`
		After  map[string]any `json:"sau"`
	}
	raw, ok := audit[0].args[7].([]byte)
	if !ok {
		t.Fatalf("delta không phải []byte: %T", audit[0].args[7])
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("đọc delta: %v", err)
	}
	if body.Before["ke_hoach_von_nam"] != float64(k.row.plan) {
		t.Errorf("truoc.ke_hoach_von_nam = %v, muốn %d", body.Before["ke_hoach_von_nam"], k.row.plan)
	}
	if body.After["ke_hoach_von_nam"] != float64(next) {
		t.Errorf("sau.ke_hoach_von_nam = %v, muốn %d", body.After["ke_hoach_von_nam"], next)
	}
	// ONLY THE FIELDS THAT MOVED. A delta carrying every column on every edit makes the one field
	// somebody actually changed impossible to find in a ledger that is never deleted.
	if _, ok := body.Before["ten"]; ok {
		t.Error("delta mang `ten` mà tên không đổi")
	}
}

// ⚠ ADR 0036 DECIDED FOR THE VOUCHER, NOT FOR THE PROJECT. A project has no `trang_thai` column and
// no confirmation to lose, so editing one whose vouchers are confirmed or locked is an ordinary
// edit. This case exists so that a future reader does not carry that ADR across and invent a
// lifecycle the specification never gave this record.
func TestUpdateInvestmentProjectWithConfirmedVoucherUpdatesNormally(t *testing.T) {
	k := readyInvestmentProjectStore()
	k.row = readyInvestmentProjectRow()
	k.voucherCount = 3 // vouchers exist, some confirmed — irrelevant to an edit
	uc, ctx := newInvestmentProjectUseCase(t, k)

	next := domain.Dong(250_000_000)
	after, err := uc.UpdateInvestmentProject(ctx, k.row.id, UpdateInvestmentProjectRequest{PlannedAmount: &next}, testStaff)
	if err != nil {
		t.Fatalf("Sua: %v", err)
	}
	if after.PlannedAmount != next {
		t.Errorf("ke_hoach_von_nam = %d, muốn %d", after.PlannedAmount, next)
	}
	if k.commits != 1 {
		t.Errorf("commit=%d, muốn 1", k.commits)
	}
	// AND NOTHING COUNTED THE VOUCHERS. A count here would mean the edit path had grown a rule the
	// specification never stated.
	if k.hasStmt("count(*) FROM chung_tu_giai_ngan") {
		t.Error("đường SỬA đếm chứng từ — đó là luật của đường XOÁ, không phải của đường sửa")
	}
}

// Correcting a name must not fail because the category the project has always been classified under
// was removed from the catalogue afterwards. The row is the historical fact; refusing to edit
// anything else would strand it.
func TestUpdateInvestmentProjectUnchangedCategoryIsNotChecked(t *testing.T) {
	k := readyInvestmentProjectStore()
	k.row = readyInvestmentProjectRow()
	k.liveCategories = map[string]bool{} // the category has since been removed
	uc, ctx := newInvestmentProjectUseCase(t, k)

	name := "Tên mới"
	if _, err := uc.UpdateInvestmentProject(ctx, k.row.id, UpdateInvestmentProjectRequest{Name: &name}, testStaff); err != nil {
		t.Fatalf("sửa tên không được phụ thuộc vào hạng mục cũ: %v", err)
	}
	if k.hasStmt("FROM hang_muc_ke_hoach_von") {
		t.Error("không đổi hạng mục mà vẫn kiểm hạng mục")
	}
}

// Reclassifying INTO a category this commune does not have is what is refused — rule 1: a category
// of another commune is indistinguishable from one that does not exist.
func TestUpdateInvestmentProjectToMissingCategoryIsRefused(t *testing.T) {
	k := readyInvestmentProjectStore()
	k.row = readyInvestmentProjectRow()
	uc, ctx := newInvestmentProjectUseCase(t, k)

	next := "hm-cua-xa-khac"
	_, err := uc.UpdateInvestmentProject(ctx, k.row.id, UpdateInvestmentProjectRequest{CategoryID: &next}, testStaff)
	if !errors.Is(err, fistore.ErrCategoryNotFound) {
		t.Fatalf("lỗi = %v, muốn ErrKhongThayHangMuc", err)
	}
	if k.hasStmt("UPDATE du_an") {
		t.Error("từ chối hạng mục mà vẫn ghi dòng")
	}
}

// A project of another commune, or one that does not exist, is one answer: the query cannot reach
// another commune's row at all (rule 1). The handler answers 404 either way.
func TestUpdateMissingInvestmentProjectReportsNotFound(t *testing.T) {
	k := readyInvestmentProjectStore() // k.row is nil
	uc, ctx := newInvestmentProjectUseCase(t, k)

	name := "Tên mới"
	_, err := uc.UpdateInvestmentProject(ctx, "01JKHONGCO000000000000000", UpdateInvestmentProjectRequest{Name: &name}, testStaff)
	if !errors.Is(err, fistore.ErrInvestmentProjectNotFound) {
		t.Fatalf("lỗi = %v, muốn ErrKhongThayDuAn", err)
	}
}

// --- removing ----------------------------------------------------------------------------------

// ⚠ THE STOP CONDITION, PROVED. The specification says nothing about removing a project that still
// carries vouchers; refusing is the only direction that writes nothing and can be loosened later
// with one branch. What must NOT happen is the project going while the vouchers stay live — their
// money would vanish from §3 and §5 while the rows sit in the table, and the commune's own totals
// would stop agreeing with the sum of its own vouchers.
func TestDeleteInvestmentProjectWithVouchersIsRefusedAndWritesNothing(t *testing.T) {
	k := readyInvestmentProjectStore()
	k.row = readyInvestmentProjectRow()
	k.voucherCount = 1
	uc, ctx := newInvestmentProjectUseCase(t, k)

	err := uc.DeleteInvestmentProject(ctx, k.row.id, "nhập nhầm năm ngân sách", testStaff)
	if !errors.Is(err, fistore.ErrInvestmentProjectHasVouchers) {
		t.Fatalf("lỗi = %v, muốn ErrDuAnConChungTu", err)
	}
	if k.hasStmt("UPDATE du_an") {
		t.Error("từ chối mà vẫn xoá mềm dự án")
	}
	if k.hasStmt("UPDATE phan_bo_nguon_von") {
		t.Error("từ chối mà vẫn xoá mềm dòng phân bổ")
	}
	if k.hasStmt("INSERT INTO audit_log") {
		t.Error("từ chối mà vẫn ghi vết")
	}
	if k.commits != 0 {
		t.Errorf("commit=%d, muốn 0", k.commits)
	}
}

// A soft-deleted voucher is already out of every read path and out of `da_giai_ngan`, so nothing is
// stranded by removing the project it pointed at. The count must therefore exclude them.
func TestDeleteInvestmentProjectWithOnlyRemovedVouchersSucceeds(t *testing.T) {
	k := readyInvestmentProjectStore()
	k.row = readyInvestmentProjectRow()
	k.voucherCount = 0 // every voucher carries deleted_at
	uc, ctx := newInvestmentProjectUseCase(t, k)

	if err := uc.DeleteInvestmentProject(ctx, k.row.id, "dự án rút khỏi kế hoạch năm", testStaff); err != nil {
		t.Fatalf("Xoa: %v", err)
	}
	count := k.stmtsContaining("count(*) FROM chung_tu_giai_ngan")
	if len(count) != 1 {
		t.Fatalf("có %d câu đếm chứng từ, muốn 1", len(count))
	}
	if !strings.Contains(count[0].sql, "deleted_at IS NULL") {
		t.Errorf("câu đếm chứng từ không loại chứng từ đã gỡ: %q", count[0].sql)
	}
}

// Rule 7, invariant 1: `deleted_at`, `deleted_by` AND `delete_reason`, all three, in ONE statement so
// none can be forgotten. `deleted_by` holds the STAFF BUSINESS CODE — two kinds of identifier in one
// column is a column nobody can query (rule 6, invariant 8).
func TestDeleteInvestmentProjectWritesAllThreeColumnsAndAllocationLines(t *testing.T) {
	k := readyInvestmentProjectStore()
	k.row = readyInvestmentProjectRow()
	k.deletedAllocations = 3
	uc, ctx := newInvestmentProjectUseCase(t, k)

	const reason = "xã rút dự án khỏi kế hoạch vốn năm 2026"
	if err := uc.DeleteInvestmentProject(ctx, k.row.id, reason, testStaff); err != nil {
		t.Fatalf("Xoa: %v", err)
	}

	del := k.stmtsContaining("UPDATE du_an")
	if len(del) != 1 {
		t.Fatalf("có %d câu xoá mềm dự án, muốn 1", len(del))
	}
	for _, column := range []string{"deleted_at", "deleted_by", "delete_reason"} {
		if !strings.Contains(del[0].sql, column) {
			t.Errorf("câu xoá mềm thiếu %q: %q", column, del[0].sql)
		}
	}
	// A SECOND REMOVAL MUST BE A 404, NOT A SILENT REWRITE of who removed it and why — the first
	// removal is the one that happened (rule 7, forbidden #5).
	if !strings.Contains(del[0].sql, "deleted_at IS NULL") {
		t.Errorf("câu xoá mềm thiếu `AND deleted_at IS NULL`: %q", del[0].sql)
	}
	if got := del[0].args[2]; got != testStaff.ID {
		t.Errorf("deleted_by = %v, muốn mã cán bộ %q", got, testStaff.ID)
	}

	// THE ALLOCATION LINES GO WITH THE PROJECT. Left live they would keep counting toward §6's
	// "đã phân bổ" and toward that source's "Số dự án" for a project no screen can show.
	pb := k.stmtsContaining("UPDATE phan_bo_nguon_von")
	if len(pb) != 1 {
		t.Fatalf("có %d câu xoá mềm dòng phân bổ, muốn 1", len(pb))
	}
	if got := pb[0].args[2]; got != testStaff.ID {
		t.Errorf("deleted_by của dòng phân bổ = %v, muốn %q", got, testStaff.ID)
	}

	// THE COUNT REACHES THE TRAIL, because nothing else can rebuild it once the lines carry
	// `deleted_at` — and it is what explains why a §6 card's "đã phân bổ" dropped.
	audit := k.stmtsContaining("INSERT INTO audit_log")
	if len(audit) != 1 {
		t.Fatalf("có %d vết, muốn 1", len(audit))
	}
	var body map[string]any
	raw, _ := audit[0].args[7].([]byte)
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("đọc delta: %v", err)
	}
	if body["so_dong_phan_bo"] != float64(k.deletedAllocations) {
		t.Errorf("so_dong_phan_bo = %v, muốn %d", body["so_dong_phan_bo"], k.deletedAllocations)
	}
	if body["ly_do"] != reason {
		t.Errorf("ly_do = %v, muốn %q", body["ly_do"], reason)
	}
}

// Rule 7, invariant 1 names `delete_reason` and it is not optional: a project that vanished from the
// commune's plan with no reason attached is a whole year's allocation nobody can account for — and
// the row is still there, so the question WILL be asked.
func TestDeleteInvestmentProjectMissingReasonRefusedBeforeTransaction(t *testing.T) {
	k := readyInvestmentProjectStore()
	k.row = readyInvestmentProjectRow()
	uc, ctx := newInvestmentProjectUseCase(t, k)

	err := uc.DeleteInvestmentProject(ctx, k.row.id, "   ", testStaff)
	if !errors.Is(err, domain.ErrInvestmentProjectDeleteReasonMissing) {
		t.Fatalf("lỗi = %v, muốn ErrThieuLyDoXoaDuAn", err)
	}
	if k.begins != 0 {
		t.Errorf("mở %d giao dịch, muốn 0", k.begins)
	}
}

// There is no hard delete anywhere on this path. `ho_so_luu_tru_cam_xoa_cung` refuses one underneath
// (0004:325-328); this proves the application never even writes the statement.
func TestDeleteInvestmentProjectNeverIssuesDELETE(t *testing.T) {
	k := readyInvestmentProjectStore()
	k.row = readyInvestmentProjectRow()
	uc, ctx := newInvestmentProjectUseCase(t, k)

	if err := uc.DeleteInvestmentProject(ctx, k.row.id, "rút khỏi kế hoạch", testStaff); err != nil {
		t.Fatalf("Xoa: %v", err)
	}
	for _, l := range k.stmts {
		if strings.Contains(strings.ToUpper(l.sql), "DELETE FROM") {
			t.Errorf("sinh câu xoá cứng: %q", l.sql)
		}
	}
}
