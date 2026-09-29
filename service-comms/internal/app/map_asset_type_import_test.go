package app

import (
	"database/sql/driver"
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// THE EXCEL IMPORT'S TRANSACTION, the REAL use case over the REAL store over catalogueDB
// (fake_driver_catalogue_test.go): the snapshot read inside the transaction, every insert, ONE entry,
// and a preview that commits nothing.

var importer = audit.Actor{ID: "CB-00123", Kind: "staff", IP: "10.0.0.7"}

func importSnapshot() [][]driver.Value {
	return [][]driver.Value{
		{"doanh-nghiep", "Doanh nghiệp", false},
		{"cho", "Chợ cũ", true}, // soft-deleted: its code stays taken
	}
}

func importRows() []domain.MapAssetTypeImportRow {
	return []domain.MapAssetTypeImportRow{
		{Row: 2, Label: "Hợp tác xã", Order: "1"},
		{Row: 3, Label: "Trang trại", Code: "trang-trai", Order: ""},
	}
}

func TestPreviewImport_PlansAndCommitsNothing(t *testing.T) {
	d := &catalogueDB{snapshot: importSnapshot()}
	uc, ctx := newTestCatalogue(t, d)

	res, err := uc.PreviewMapAssetTypeImport(ctx, append(importRows(), domain.MapAssetTypeImportRow{Row: 4, Label: "Chợ", Code: "cho"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Errors) != 1 || res.Errors[0].Row != 4 || res.Types != nil {
		t.Fatalf("= %+v — mã của dòng đã xoá mềm vẫn là mã đã cấp", res)
	}
	if d.committed != 0 || d.rolledBack != 1 {
		t.Errorf("commit %d, rollback %d — xem trước phải luôn cuộn lại", d.committed, d.rolledBack)
	}
	if d.ran("INSERT") {
		t.Error("xem trước đã chạy câu chèn")
	}
	snap := d.with("deleted_at IS NOT NULL FROM loai_tai_nguyen_ban_do")
	if len(snap) != 1 || !strings.Contains(snap[0].sql, "tenant_id = $1") || snap[0].args[0] != string(tenantA) {
		t.Errorf("ảnh chụp không lọc theo xã của context: %+v", snap)
	}
	if strings.Contains(snap[0].sql, "deleted_at IS NULL") {
		t.Error("ảnh chụp bỏ các dòng đã xoá mềm — mã của chúng vẫn đã cấp")
	}
}

func TestImport_WritesEveryRowAndOneAuditEntry(t *testing.T) {
	d := &catalogueDB{snapshot: importSnapshot()}
	uc, ctx := newTestCatalogue(t, d)

	res, err := uc.ImportMapAssetTypes(ctx, importRows(), importer)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Types) != 2 || res.Types[0].Code != "hop-tac-xa" || res.Types[0].ID == "" {
		t.Fatalf("= %+v", res.Types)
	}
	ins := d.with("INSERT INTO loai_tai_nguyen_ban_do")
	if len(ins) != 2 {
		t.Fatalf("%d câu chèn, muốn 2", len(ins))
	}
	for _, s := range ins {
		if !strings.Contains(s.sql, "'don-vi', false") {
			t.Error("dòng nhập không đi qua câu chèn của biểu mẫu (nguon là hằng)")
		}
	}
	aud := d.with("INSERT INTO audit_log")
	if len(aud) != 1 {
		t.Fatalf("%d dòng vết, muốn đúng 1 cho cả tệp", len(aud))
	}
	if aud[0].args[1] != "CB-00123" || aud[0].args[4] != ActionImportMapAssetTypes {
		t.Errorf("vết = %v", aud[0].args[:6])
	}
	delta := string(aud[0].args[7].([]byte))
	for _, must := range []string{"hop-tac-xa", "trang-trai", `"so_dong":2`, "nhap_excel"} {
		if !strings.Contains(delta, must) {
			t.Errorf("delta thiếu %q: %s", must, delta)
		}
	}
	if d.begun != 1 || d.committed != 1 || d.rolledBack != 0 {
		t.Errorf("giao dịch: mở %d commit %d rollback %d", d.begun, d.committed, d.rolledBack)
	}
}

func TestImport_RejectedFileWritesNothing(t *testing.T) {
	d := &catalogueDB{snapshot: importSnapshot()}
	uc, ctx := newTestCatalogue(t, d)

	rows := append(importRows(), domain.MapAssetTypeImportRow{Row: 4, Label: "Doanh nghiệp"})
	_, err := uc.ImportMapAssetTypes(ctx, rows, importer)
	var rej *MapAssetTypeImportRejected
	if !errors.As(err, &rej) || len(rej.Errors) == 0 {
		t.Fatalf("lỗi = %v, muốn *MapAssetTypeImportRejected", err)
	}
	if d.ran("INSERT") || d.committed != 0 || d.rolledBack != 1 {
		t.Errorf("tệp bị từ chối mà vẫn ghi (commit %d, rollback %d)", d.committed, d.rolledBack)
	}
}

func TestImport_AuditFailureRollsBackEveryRow(t *testing.T) {
	d := &catalogueDB{snapshot: importSnapshot(), failOn: "INSERT INTO audit_log"}
	uc, ctx := newTestCatalogue(t, d)
	if _, err := uc.ImportMapAssetTypes(ctx, importRows(), importer); err == nil {
		t.Fatal("vết hỏng mà nhập vẫn thành công")
	}
	if len(d.with("INSERT INTO loai_tai_nguyen_ban_do")) != 2 || d.committed != 0 || d.rolledBack != 1 {
		t.Error("các dòng đã chèn không bị cuộn lại cùng vết")
	}
}

func TestImport_RefusesEmptyActor(t *testing.T) {
	d := &catalogueDB{snapshot: importSnapshot()}
	uc, ctx := newTestCatalogue(t, d)
	if _, err := uc.ImportMapAssetTypes(ctx, importRows(), audit.Actor{}); !errors.Is(err, ErrNoActor) {
		t.Fatalf("lỗi = %v", err)
	}
	if d.begun != 0 {
		t.Error("không có chủ thể mà đã mở giao dịch")
	}
}

// The fake driver's row shape must be the store's SELECT list, read from the statement the store
// actually sent — not a second hand-written copy of it. This is what would have caught the
// scan-order defect fixed on 2026-09-29 (store.scanMapAssetType).
func TestFakeDriverMirrorsTheSelectedColumns(t *testing.T) {
	d := &catalogueDB{row: &fakeMapAssetTypeRow{id: "ltn-1", code: "cong-van", label: "Công văn", isActive: true, sortOrder: 3, source: "don-vi"}}
	uc, ctx := newTestCatalogue(t, d)
	label := "Công văn mới"
	got, err := uc.Update(ctx, "ltn-1", UpdateMapAssetTypeRequest{Label: &label}, importer)
	if err != nil {
		t.Fatal(err)
	}
	if got.SortOrder != 3 || !got.IsActive || got.IsDefault {
		t.Errorf("đọc sai cột: thu_tu=%d dang_dung=%v la_mac_dinh=%v", got.SortOrder, got.IsActive, got.IsDefault)
	}
	lock := d.with("FOR UPDATE")[0].sql
	sel := lock[len("SELECT "):strings.Index(lock, " FROM ")]
	if want := strings.Join(mapAssetTypeCols(), ", "); sel != want {
		t.Fatalf("câu SELECT chọn %q, driver giả trả %q — lệch thứ tự là quét sai trên PostgreSQL", sel, want)
	}
}
