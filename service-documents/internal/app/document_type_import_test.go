package app

import (
	"database/sql/driver"
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/service-documents/internal/domain"
)

// THE EXCEL IMPORT'S TRANSACTION, the REAL use case over the REAL store over khoGia (driver_gia_test.go):
// the snapshot read inside the transaction, every insert through the form's statement, ONE entry, and
// a preview that commits nothing.

var importer = audit.Actor{ID: "CB-00123", Kind: "staff", IP: "10.0.0.7"}

func importSnapshot() [][]driver.Value {
	return [][]driver.Value{
		{"quyet-dinh", "Quyết định", false},
		{"cong-van", "Công văn cũ", true}, // soft-deleted: its code stays taken
	}
}

func importRows() []domain.DocumentTypeImportRow {
	return []domain.DocumentTypeImportRow{
		{Row: 2, Label: "Tờ trình", Order: "1"},
		{Row: 3, Label: "Báo cáo", Code: "bao-cao", Order: ""},
	}
}

func TestPreviewDocumentTypeImport_PlansAndCommitsNothing(t *testing.T) {
	k := &khoGia{snapshot: importSnapshot()}
	uc, ctx := dungUseCase(t, k)

	res, err := uc.PreviewDocumentTypeImport(ctx, append(importRows(), domain.DocumentTypeImportRow{Row: 4, Label: "Công văn", Code: "cong-van"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Errors) != 1 || res.Errors[0].Row != 4 || res.Types != nil {
		t.Fatalf("= %+v — mã của dòng đã xoá mềm vẫn là mã đã cấp", res)
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Errorf("commit %d, rollback %d — xem trước phải luôn cuộn lại", k.daCommit, k.daRollback)
	}
	if k.coCau("INSERT") {
		t.Error("xem trước đã chạy câu chèn")
	}
	snap := k.cau("deleted_at IS NOT NULL FROM loai_van_ban")
	if len(snap) != 1 || !strings.Contains(snap[0].sql, "tenant_id = $1") || snap[0].args[0] != string(xaA) {
		t.Errorf("ảnh chụp không lọc theo xã của context: %+v", snap)
	}
	if strings.Contains(snap[0].sql, "deleted_at IS NULL") {
		t.Error("ảnh chụp bỏ các dòng đã xoá mềm — mã của chúng vẫn đã cấp")
	}
}

func TestImportDocumentTypes_WritesEveryRowAndOneAuditEntry(t *testing.T) {
	k := &khoGia{snapshot: importSnapshot()}
	uc, ctx := dungUseCase(t, k)

	res, err := uc.ImportDocumentTypes(ctx, importRows(), importer)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Types) != 2 || res.Types[0].Code != "to-trinh" || res.Types[0].ID == "" {
		t.Fatalf("= %+v", res.Types)
	}
	ins := k.cau("INSERT INTO loai_van_ban")
	if len(ins) != 2 {
		t.Fatalf("%d câu chèn, muốn 2", len(ins))
	}
	for _, s := range ins {
		if !strings.Contains(s.sql, "'don-vi', false") {
			t.Error("dòng nhập không đi qua câu chèn của biểu mẫu (nguon là hằng)")
		}
		// (tenant, id, ma, nhan, thu_tu, la_mac_dinh, dang_dung): never the default, always in use.
		if s.args[5] != false || s.args[6] != true {
			t.Errorf("dòng nhập la_mac_dinh=%v dang_dung=%v — nhập không được đổi mặc định của xã", s.args[5], s.args[6])
		}
	}
	if k.coCau("la_mac_dinh = false") {
		t.Error("nhập đã xoá cờ mặc định của dòng khác")
	}
	aud := k.cau("INSERT INTO audit_log")
	if len(aud) != 1 {
		t.Fatalf("%d dòng vết, muốn đúng 1 cho cả tệp", len(aud))
	}
	if aud[0].args[1] != "CB-00123" || aud[0].args[4] != ActionImportDocumentTypes {
		t.Errorf("vết = %v", aud[0].args[:6])
	}
	delta := string(aud[0].args[7].([]byte))
	for _, must := range []string{"to-trinh", "bao-cao", `"so_dong":2`, "nhap_excel"} {
		if !strings.Contains(delta, must) {
			t.Errorf("delta thiếu %q: %s", must, delta)
		}
	}
	if k.batDau != 1 || k.daCommit != 1 || k.daRollback != 0 {
		t.Errorf("giao dịch: mở %d commit %d rollback %d", k.batDau, k.daCommit, k.daRollback)
	}
}

func TestImportDocumentTypes_RejectedFileWritesNothing(t *testing.T) {
	k := &khoGia{snapshot: importSnapshot()}
	uc, ctx := dungUseCase(t, k)

	rows := append(importRows(), domain.DocumentTypeImportRow{Row: 4, Label: "Quyết định"})
	_, err := uc.ImportDocumentTypes(ctx, rows, importer)
	var rej *DocumentTypeImportRejected
	if !errors.As(err, &rej) || len(rej.Errors) == 0 {
		t.Fatalf("lỗi = %v, muốn *DocumentTypeImportRejected", err)
	}
	if k.coCau("INSERT") || k.daCommit != 0 || k.daRollback != 1 {
		t.Errorf("tệp bị từ chối mà vẫn ghi (commit %d, rollback %d)", k.daCommit, k.daRollback)
	}
}

func TestImportDocumentTypes_AuditFailureRollsBackEveryRow(t *testing.T) {
	k := &khoGia{snapshot: importSnapshot(), loiSau: "INSERT INTO audit_log"}
	uc, ctx := dungUseCase(t, k)
	if _, err := uc.ImportDocumentTypes(ctx, importRows(), importer); err == nil {
		t.Fatal("vết hỏng mà nhập vẫn thành công")
	}
	if len(k.cau("INSERT INTO loai_van_ban")) != 2 || k.daCommit != 0 || k.daRollback != 1 {
		t.Error("các dòng đã chèn không bị cuộn lại cùng vết")
	}
}

func TestImportDocumentTypes_RefusesEmptyActor(t *testing.T) {
	k := &khoGia{snapshot: importSnapshot()}
	uc, ctx := dungUseCase(t, k)
	if _, err := uc.ImportDocumentTypes(ctx, importRows(), audit.Actor{}); !errors.Is(err, ErrNoImportActor) {
		t.Fatalf("lỗi = %v", err)
	}
	if k.batDau != 0 {
		t.Error("không có chủ thể mà đã mở giao dịch")
	}
}
