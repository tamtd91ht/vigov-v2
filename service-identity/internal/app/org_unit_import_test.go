package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The org-chart import over the REAL store and a REAL transaction boundary (driver_gia_bo_phan_test.go).
// The row rules are the planner's and are tabled in domain/org_unit_import_test.go; what is proved
// here is what the transaction owns: all or nothing, one entry per unit in the same transaction,
// the commune on every statement, and a preview that writes nothing.

func newImporterForTest(t *testing.T, k *khoBoPhanGia) (*OrgUnitImporter, context.Context) {
	t.Helper()
	db := sql.OpenDB(k)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	kho := store.New(db)
	uc := NewOrgUnitImporter(kho, idstore.NewBoPhanStore(kho))
	n := 0
	uc.newID = func() (string, error) {
		n++
		return fmt.Sprintf("01JIMPORTUNIT%013d", n), nil
	}
	return uc, tenant.Into(context.Background(), xaBoPhan)
}

func importRows() []domain.OrgUnitImportRow {
	return []domain.OrgUnitImportRow{
		{Row: 2, Name: "VĂN PHÒNG ĐẢNG ỦY", Order: "3"},
		{Row: 3, Name: "TỔ TỔNG HỢP", Parent: "van-phong-dang-uy"}, // a row above
		{Row: 4, Name: "TỔ MỚI", Parent: "LÃNH ĐẠO"},               // an existing unit (bp-a), by name
	}
}

func TestImportOrgUnits_WritesTreeAndOneAuditPerUnitInOneTransaction(t *testing.T) {
	k := &khoBoPhanGia{hang: cayBaTang()}
	uc, ctx := newImporterForTest(t, k)

	res, err := uc.Import(ctx, importRows(), nguoiBoPhan())
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if len(res.Units) != 3 {
		t.Fatalf("tạo %d bộ phận, muốn 3", len(res.Units))
	}
	if k.batDau != 1 || k.daCommit != 1 || k.daRollback != 0 {
		t.Fatalf("giao dịch: mở %d, commit %d, rollback %d — muốn MỘT giao dịch cho cả tệp", k.batDau, k.daCommit, k.daRollback)
	}

	ins := k.cau("INSERT INTO bo_phan")
	if len(ins) != 3 {
		t.Fatalf("có %d INSERT bo_phan, muốn 3", len(ins))
	}
	// Row 3's parent is row 2, created in the same transaction: its cha_id is row 2's new id.
	if ins[0].args[3] != "van-phong-dang-uy" || ins[0].args[4] != "" || ins[0].args[5] != int64(3) {
		t.Errorf("INSERT dòng 2: %v", ins[0].args)
	}
	if ins[1].args[4] != ins[0].args[1] {
		t.Errorf("dòng 3 phải có cha_id = id mới của dòng 2 (%v), nhận %v", ins[0].args[1], ins[1].args[4])
	}
	if ins[2].args[4] != "bp-a" {
		t.Errorf("dòng 4 phải có cha_id = bp-a, nhận %v", ins[2].args[4])
	}

	// The existing parent was LOCKED, as Them locks its one.
	locks := k.cau("FOR UPDATE")
	if len(locks) != 1 || locks[0].args[1] != "bp-a" {
		t.Errorf("khoá cha đang có: %+v", locks)
	}

	vet := k.cau("INSERT INTO audit_log")
	if len(vet) != 3 {
		t.Fatalf("có %d vết, muốn MỘT vết cho MỖI bộ phận (3)", len(vet))
	}
	for i, v := range vet {
		a := v.args
		if a[1] != maCanBoBoPhan {
			t.Errorf("vết %d: actor_id = %v, muốn MÃ CÁN BỘ %q (luật 6 bất biến 8)", i, a[1], maCanBoBoPhan)
		}
		if a[4] != HanhViThemBoPhan {
			t.Errorf("vết %d: hành vi = %v", i, a[4])
		}
		var delta map[string]any
		b, _ := a[7].([]byte)
		if err := json.Unmarshal(b, &delta); err != nil || delta["nguon"] != importSource || delta["sau"] == nil {
			t.Errorf("vết %d: delta %s (%v)", i, b, err)
		}
	}
	if vet[0].args[5] != "van-phong-dang-uy" {
		t.Errorf("chủ thể vết đầu = %v, muốn mã bộ phận", vet[0].args[5])
	}

	// EVERY STATEMENT BINDS THE COMMUNE OF THE CONTEXT AT $1 (rule 1).
	for _, l := range k.lenh {
		if len(l.args) == 0 || l.args[0] != string(xaBoPhan) {
			t.Errorf("câu lệnh không mang xã của ngữ cảnh ở $1: %q %v", l.sql, l.args)
		}
	}
}

// ONE BAD ROW = ZERO WRITES, and every error of the file is returned.
func TestImportOrgUnits_OneBadRowWritesNothing(t *testing.T) {
	k := &khoBoPhanGia{hang: cayBaTang()}
	uc, ctx := newImporterForTest(t, k)

	rows := append(importRows(), domain.OrgUnitImportRow{Row: 5, Name: "TỔ LẺ", Order: "1.5"})
	_, err := uc.Import(ctx, rows, nguoiBoPhan())
	var rej *OrgUnitImportRejected
	if !errors.As(err, &rej) || len(rej.Errors) != 1 || rej.Errors[0].Row != 5 {
		t.Fatalf("lỗi = %v, muốn OrgUnitImportRejected với đúng lỗi dòng 5", err)
	}
	if n := len(k.cau("INSERT")); n != 0 {
		t.Errorf("tệp có lỗi mà vẫn có %d INSERT", n)
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Errorf("commit %d, rollback %d — muốn không commit gì", k.daCommit, k.daRollback)
	}
}

// AN AUDIT FAILURE ROLLS THE WHOLE FILE BACK — the units already inserted in this transaction go too.
func TestImportOrgUnits_AuditFailureRollsBackEverything(t *testing.T) {
	k := &khoBoPhanGia{hang: cayBaTang(), loiSau: "INSERT INTO audit_log"}
	uc, ctx := newImporterForTest(t, k)

	if _, err := uc.Import(ctx, importRows(), nguoiBoPhan()); err == nil {
		t.Fatal("vết hỏng mà Import vẫn thành công")
	}
	if k.daCommit != 0 || k.daRollback != 1 {
		t.Fatalf("commit %d, rollback %d — vết hỏng phải huỷ cả tệp (luật 6 bất biến 3)", k.daCommit, k.daRollback)
	}
}

// A CODE THE UNIQUE KEY REFUSES (a concurrent creation took it) rolls the whole file back.
func TestImportOrgUnits_InsertFailureRollsBackEverything(t *testing.T) {
	k := &khoBoPhanGia{hang: cayBaTang(), loiSau: "INSERT INTO bo_phan"}
	uc, ctx := newImporterForTest(t, k)

	if _, err := uc.Import(ctx, importRows(), nguoiBoPhan()); err == nil {
		t.Fatal("INSERT hỏng mà Import vẫn thành công")
	}
	if k.daCommit != 0 || len(k.cau("INSERT INTO audit_log")) != 0 {
		t.Fatalf("commit %d, vết %d — INSERT hỏng phải huỷ cả tệp", k.daCommit, len(k.cau("INSERT INTO audit_log")))
	}
}

// THE PREVIEW WRITES NOTHING, whether the file is good or bad, and always rolls back.
func TestPreviewOrgUnits_WritesNothing(t *testing.T) {
	for _, c := range []struct {
		name      string
		rows      []domain.OrgUnitImportRow
		wantUnits int
		wantErrs  int
	}{
		{"tệp hợp lệ", importRows(), 3, 0},
		{"tệp có lỗi", []domain.OrgUnitImportRow{{Row: 2, Name: "", Order: "x"}}, 0, 2},
	} {
		t.Run(c.name, func(t *testing.T) {
			k := &khoBoPhanGia{hang: cayBaTang()}
			uc, ctx := newImporterForTest(t, k)
			res, err := uc.Preview(ctx, c.rows)
			if err != nil {
				t.Fatalf("Preview: %v", err)
			}
			if len(res.Units) != c.wantUnits || len(res.Errors) != c.wantErrs {
				t.Errorf("units %d errors %d, muốn %d / %d: %+v", len(res.Units), len(res.Errors), c.wantUnits, c.wantErrs, res.Errors)
			}
			if n := len(k.cau("INSERT")); n != 0 {
				t.Errorf("xem trước mà có %d INSERT", n)
			}
			if n := len(k.cau("FOR UPDATE")); n != 0 {
				t.Errorf("xem trước mà khoá %d dòng", n)
			}
			if k.daCommit != 0 || k.daRollback != 1 {
				t.Errorf("commit %d, rollback %d — xem trước luôn huỷ giao dịch", k.daCommit, k.daRollback)
			}
		})
	}
}

// ANOTHER COMMUNE'S UNIT IS NOT A PARENT, and a soft-deleted unit's code is not free.
func TestImportOrgUnits_SnapshotIsThisCommuneIncludingDeleted(t *testing.T) {
	k := &khoBoPhanGia{hang: cayBaTang()}
	uc, ctx := newImporterForTest(t, k)

	res, err := uc.Preview(ctx, []domain.OrgUnitImportRow{
		{Row: 2, Name: "TỔ A", Parent: "van-phong-hdnd"}, // commune B's unit
		{Row: 3, Name: "TỔ B", Code: "da-xoa"},           // commune A's SOFT-DELETED unit's code
	})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	var parentErr, codeErr bool
	for _, e := range res.Errors {
		parentErr = parentErr || (e.Row == 2 && e.Column == domain.OrgUnitImportColParent)
		codeErr = codeErr || (e.Row == 3 && e.Column == domain.OrgUnitImportColCode)
	}
	if !parentErr {
		t.Errorf("RÒ RỈ: bộ phận của xã khác được nhận làm cha: %+v", res.Errors)
	}
	if !codeErr {
		t.Errorf("mã của bộ phận đã xoá mềm bị coi là còn trống: %+v", res.Errors)
	}
}

func TestImportOrgUnits_RefusesActorWithoutStaffCode(t *testing.T) {
	k := &khoBoPhanGia{hang: cayBaTang()}
	uc, ctx := newImporterForTest(t, k)
	n := nguoiBoPhan()
	n.Vet.ID = ""
	if _, err := uc.Import(ctx, importRows(), n); err == nil {
		t.Fatal("thiếu mã cán bộ mà vẫn nhập")
	}
	if k.batDau != 0 {
		t.Error("thiếu mã cán bộ mà vẫn mở giao dịch")
	}
}
