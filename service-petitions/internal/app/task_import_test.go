package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// Tests for app.TaskImport (P11).
//
//	PROVED HERE   a clean file books EVERY row in ONE committed transaction through the create path —
//	              each its own auto-issued number, its INSERT, its timeline row and its own
//	              `tao_nhiem_vu` entry marked `nguon_tao`, plus ONE batch entry · ONE refused row and
//	              NOTHING is written, every row's refusals still reported · dry run writes nothing ·
//	              identity down refuses the whole file as unchecked · unknown unit / unassignable staff /
//	              removed bloc / type not in use are refused per row · a blank type takes the commune's
//	              default, and no default refuses · staff codes are asked in chunks of 50 · a wrong
//	              heading row and more than 500 rows are refused whole.
//
//	NOT PROVED    PostgreSQL's numbering across rows (the fake's high-water mark follows its own
//	              inserts, as the AFTER INSERT trigger of migration 0015 makes the real one do).

type importLookupsFake struct {
	units       map[string]string
	blocs       map[string]identityclient.TaskBlocLabel
	assignable  map[string]bool
	err         error
	staffChunks []int
}

func (f *importLookupsFake) LiveOrgUnitIDsByCode(_ context.Context, codes []string) (map[string]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := map[string]string{}
	for _, c := range codes {
		if id, ok := f.units[c]; ok {
			out[c] = id
		}
	}
	return out, nil
}

func (f *importLookupsFake) TaskBlocLabels(_ context.Context, codes []string) (map[string]identityclient.TaskBlocLabel, error) {
	out := map[string]identityclient.TaskBlocLabel{}
	for _, c := range codes {
		if b, ok := f.blocs[c]; ok {
			out[c] = b
		}
	}
	return out, nil
}

// vi-name-ok: implements TaskImportLookups, which mirrors core/identityclient.Client's existing method
func (f *importLookupsFake) CanBoGiaoViecDuoc(_ context.Context, ma []string) (map[string]struct{}, error) {
	f.staffChunks = append(f.staffChunks, len(ma))
	out := map[string]struct{}{}
	for _, m := range ma {
		if f.assignable[m] {
			out[m] = struct{}{}
		}
	}
	return out, nil
}

type importTypesFake struct{ rows []domain.LoaiNhiemVu }

// vi-name-ok: implements TaskTypeCatalogue, which mirrors the existing catalogue stores' method
func (f importTypesFake) DanhSach(context.Context) ([]domain.LoaiNhiemVu, error) { return f.rows, nil }

type importPrioritiesFake struct{ rows []domain.MucUuTienNhiemVu }

// vi-name-ok: implements TaskPriorityCatalogue, which mirrors the existing catalogue stores' method
func (f importPrioritiesFake) DanhSach(context.Context) ([]domain.MucUuTienNhiemVu, error) {
	return f.rows, nil
}

func importLookupsSample() *importLookupsFake {
	return &importLookupsFake{
		units: map[string]string{"dia-chinh": "bp-dia-chinh", "van-phong": "bp-van-phong"},
		blocs: map[string]identityclient.TaskBlocLabel{
			"khoi-uy-ban": {Label: "Khối Uỷ ban", Standing: identityv1.RecordStanding_RECORD_STANDING_LIVE},
			"khoi-cu":     {Label: "Khối cũ", Standing: identityv1.RecordStanding_RECORD_STANDING_REMOVED},
		},
		assignable: map[string]bool{"CB-00311": true, "CB-00412": true},
	}
}

func newTaskImport(t *testing.T, k *khoNhiemVuGia, lookups *importLookupsFake, defaultType bool) (*TaskImport, context.Context) {
	uc, ctx := dungGhiNhiemVu(t, k)
	types := importTypesFake{rows: []domain.LoaiNhiemVu{
		{Ma: "theo-van-ban", DangDung: true, LaMacDinh: defaultType},
		{Ma: "co-ban", DangDung: true},
		{Ma: "da-tat", DangDung: false},
	}}
	pr := importPrioritiesFake{rows: []domain.MucUuTienNhiemVu{{Ma: "cao", DangDung: true}}}
	return NewTaskImport(uc, lookups, types, pr), ctx
}

func importSheet(rows ...[]string) [][]string {
	return append([][]string{append([]string(nil), domain.TaskImportHeadings...)}, rows...)
}

func importLine(title, unit, assignee string, over map[int]string) []string {
	cells := make([]string, len(domain.TaskImportHeadings))
	cells[0], cells[2], cells[3] = title, unit, assignee
	cells[5] = "30/09/2026 17:00"
	for i, v := range over {
		cells[i] = v
	}
	return cells
}

func TestTaskImportBooksEveryRowInOneTransaction(t *testing.T) {
	k := khoNVMau()
	k.soLonNhat = 40
	ti, ctx := newTaskImport(t, k, importLookupsSample(), true)

	res, err := ti.Import(ctx, importSheet(
		importLine("Rà soát quỹ đất", "dia-chinh", "CB-00311", map[int]string{
			7: "khoi-uy-ban", 8: "van-phong", 9: "CB-00412", 4: "cao", 10: "Công văn cấp trên", 13: "x"}),
		make([]string, 16), // a blank row is skipped
		importLine("Tổng hợp báo cáo", "", "CB-00311", map[int]string{6: "co-ban"}),
	), false, canBoThu())
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if !res.Committed || res.Created != 2 || res.TotalRows != 2 || len(res.Errors) != 0 {
		t.Fatalf("kết quả = %+v", res)
	}
	if strings.Join(res.Codes, ",") != "NV41,NV42" {
		t.Errorf("mã cấp = %v, muốn NV41,NV42 — cùng bộ sinh mã với giao việc, tăng dần trong một giao dịch", res.Codes)
	}
	if k.daCommit != 1 || k.batDau != 1 {
		t.Errorf("mở %d giao dịch, commit %d — muốn đúng một giao dịch cho cả tệp", k.batDau, k.daCommit)
	}
	inserts := k.cau("INSERT INTO nhiem_vu (")
	if len(inserts) != 2 {
		t.Fatalf("%d câu INSERT nhiệm vụ, muốn 2", len(inserts))
	}
	// The first row: unit resolved from its CODE to identity's id, marks written, the default type.
	first := inserts[0].args
	if first[3] != "theo-van-ban" || first[11] != "bp-dia-chinh" || first[14] != "bp-van-phong" ||
		first[24] != true || first[25] != false {
		t.Errorf("dòng 1 ghi: loại %v, bộ phận %v, chủ trì %v, duyệt %v/%v", first[3], first[11], first[14], first[24], first[25])
	}
	if len(k.cau("INSERT INTO nhiem_vu_van_ban")) != 1 {
		t.Error("ô văn bản không thành một dòng văn bản")
	}
	audits := k.cau("INSERT INTO audit_log")
	if len(audits) != 3 {
		t.Fatalf("%d dòng vết, muốn 2 tao_nhiem_vu + 1 nhap_nhiem_vu_tu_excel", len(audits))
	}
	for _, a := range audits[:2] {
		if a.args[4] != HanhViTaoNhiemVu || !strings.Contains(string(a.args[7].([]byte)), `"nguon_tao":"nhap_excel"`) {
			t.Errorf("vết từng nhiệm vụ: %v %s", a.args[4], a.args[7])
		}
	}
	if audits[2].args[4] != ActionTaskImport || !strings.Contains(string(audits[2].args[7].([]byte)), `"ma_cuoi":"NV42"`) {
		t.Errorf("vết cả tệp: %v %s", audits[2].args[4], audits[2].args[7])
	}
	chiGhiTrongGiaoDich(t, k)
}

func TestTaskImportOneBadRowWritesNothing(t *testing.T) {
	k := khoNVMau()
	ti, ctx := newTaskImport(t, k, importLookupsSample(), true)

	res, err := ti.Import(ctx, importSheet(
		importLine("Việc hợp lệ", "dia-chinh", "", nil),
		importLine("Bộ phận lạ", "khong-co", "CB-99999", map[int]string{7: "khoi-cu", 6: "da-tat"}),
	), false, canBoThu())
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if res.Committed || res.Created != 0 {
		t.Fatalf("kết quả = %+v — một dòng sai thì không nhập dòng nào", res)
	}
	want := map[string]bool{
		domain.TaskImportHeadings[2]: false, domain.TaskImportHeadings[3]: false,
		domain.TaskImportHeadings[7]: false, domain.TaskImportHeadings[6]: false,
	}
	for _, e := range res.Errors {
		if e.Row != 3 {
			t.Errorf("lỗi ở dòng %d, muốn 3", e.Row)
		}
		want[e.Column] = true
		if strings.Contains(e.Message, "khong-co") || strings.Contains(e.Message, "CB-99999") {
			t.Errorf("thông báo nhắc lại giá trị ô: %q", e.Message)
		}
	}
	for col, seen := range want {
		if !seen {
			t.Errorf("thiếu lỗi ở cột %q", col)
		}
	}
	khongGhiGi(t, k)
}

func TestTaskImportDryRunWritesNothing(t *testing.T) {
	k := khoNVMau()
	ti, ctx := newTaskImport(t, k, importLookupsSample(), true)
	res, err := ti.Import(ctx, importSheet(importLine("Việc", "dia-chinh", "", nil)), true, canBoThu())
	if err != nil || res.Committed || len(res.Errors) != 0 || res.TotalRows != 1 {
		t.Fatalf("thử nghiệm: %+v, %v", res, err)
	}
	khongGhiGi(t, k)
}

func TestTaskImportIdentityDownRefusesTheWholeFile(t *testing.T) {
	k := khoNVMau()
	lookups := importLookupsSample()
	lookups.err = fmt.Errorf("x: %w", identityclient.ErrIdentityUnavailable)
	ti, ctx := newTaskImport(t, k, lookups, true)

	_, err := ti.Import(ctx, importSheet(importLine("Việc", "dia-chinh", "", nil)), false, canBoThu())
	if !errors.Is(err, ErrTaskImportUnchecked) {
		t.Fatalf("lỗi = %v, muốn ErrTaskImportUnchecked — không được đọc thành \"bộ phận không có\"", err)
	}
	khongGhiGi(t, k)
}

func TestTaskImportBlankTypeWithoutDefaultRefused(t *testing.T) {
	k := khoNVMau()
	ti, ctx := newTaskImport(t, k, importLookupsSample(), false)
	res, err := ti.Import(ctx, importSheet(importLine("Việc", "dia-chinh", "", nil)), false, canBoThu())
	if err != nil || res.Committed || len(res.Errors) != 1 || res.Errors[0].Column != domain.TaskImportHeadings[6] {
		t.Fatalf("kết quả = %+v, %v", res, err)
	}
	khongGhiGi(t, k)
}

func TestTaskImportStaffAskedInChunksOfFifty(t *testing.T) {
	k := khoNVMau()
	lookups := importLookupsSample()
	var rows [][]string
	for i := 0; i < 60; i++ {
		code := fmt.Sprintf("CB-%05d", i)
		lookups.assignable[code] = true
		rows = append(rows, importLine(fmt.Sprintf("Việc %d", i), "dia-chinh", code, nil))
	}
	ti, ctx := newTaskImport(t, k, lookups, true)
	if _, err := ti.Import(ctx, importSheet(rows...), true, canBoThu()); err != nil {
		t.Fatalf("Import: %v", err)
	}
	if len(lookups.staffChunks) != 2 || lookups.staffChunks[0] != identityclient.TranMaGiaoViecMotLo || lookups.staffChunks[1] != 10 {
		t.Errorf("lô hỏi cán bộ = %v, muốn [50 10]", lookups.staffChunks)
	}
}

func TestTaskImportRefusedWhole(t *testing.T) {
	k := khoNVMau()
	ti, ctx := newTaskImport(t, k, importLookupsSample(), true)

	wrong := importSheet(importLine("Việc", "dia-chinh", "", nil))
	wrong[0][0] = "Tên việc"
	if _, err := ti.Import(ctx, wrong, false, canBoThu()); !errors.Is(err, ErrTaskImportLayout) {
		t.Errorf("mẫu sai: lỗi = %v", err)
	}
	if _, err := ti.Import(ctx, importSheet(), false, canBoThu()); !errors.Is(err, ErrTaskImportLayout) {
		t.Errorf("không có dòng nào: lỗi = %v", err)
	}
	var many [][]string
	for i := 0; i <= domain.TaskImportRowCap; i++ {
		many = append(many, importLine("Việc", "dia-chinh", "", nil))
	}
	if _, err := ti.Import(ctx, importSheet(many...), false, canBoThu()); !errors.Is(err, ErrTaskImportTooManyRows) {
		t.Errorf("quá trần: lỗi = %v", err)
	}
	khongGhiGi(t, k)
}
