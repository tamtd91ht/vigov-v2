package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/app"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// THE TWO EXCEL-IMPORT ROUTES OF THE BUDGET BOARD. What the handler owns: the permission (rule 5's four
// cases), the upload, the REAL workbook reaching the parser through core/xlsx.ReadSheets with each
// cell's number flag, the wire, and the status of each refusal. The transaction is proved in
// app/budget_import_test.go, the rules in domain/budget_import_test.go.

// --- the fake's two methods (its fields live with the rest of it in thu_chi_ngan_sach_test.go) -------

func (g *ghiNganSachGia) PreviewBudgetImport(ctx context.Context, req app.BudgetImportRequest) (app.BudgetImportResult, error) {
	g.previewImportCalls++
	g.lastImport = req
	g.xaCuoi = tenant.MustFrom(ctx)
	if g.importErr != nil {
		return app.BudgetImportResult{}, g.importErr
	}
	return fakeImportResult(req, false), nil
}

func (g *ghiNganSachGia) ImportBudgetWorkbook(ctx context.Context, req app.BudgetImportRequest,
	actor audit.Actor) (app.BudgetImportResult, error) {
	g.importCalls++
	g.lastImport = req
	g.ghiNhan(ctx, actor)
	if g.importErr != nil {
		return app.BudgetImportResult{}, g.importErr
	}
	return fakeImportResult(req, true), nil
}

func fakeImportResult(req app.BudgetImportRequest, written bool) app.BudgetImportResult {
	res := app.BudgetImportResult{Year: req.Year, FileName: domain.NormaliseSourceFileName(req.FileName)}
	for _, s := range req.Sheets {
		o := app.BudgetImportOutcome{Sheet: s}
		if written {
			o.Created = domain.BangNganSach{ID: "01JIMPORTED" + strings.ToUpper(string(s.Kind)), Ma: fmt.Sprintf("NS-%d-%s-01", req.Year, strings.ToUpper(string(s.Kind))),
				Nam: req.Year, Loai: s.Kind, Lan: 1, TieuDe: s.Title, DonViTinh: s.Unit.Nhan(), NguonTep: req.FileName}
		}
		res.Sheets = append(res.Sheets, o)
	}
	return res
}

// --- the fixture: a REAL two-sheet workbook ---------------------------------------------------------

// budgetWorkbook builds the Phòng Tài chính's file shape: a "Thu" tab with a MERGED two-row header and
// indentation-only detail lines, a "chi NS" tab with Roman / letter / Arabic / dotted codes and a
// "Tổng số" line, unit lines, and a hidden notes tab.
func budgetWorkbook(t *testing.T) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(f.SetSheetName("Sheet1", "Thu"))
	row := func(sheet string, n int, vals ...any) {
		t.Helper()
		cell, _ := excelize.CoordinatesToCellName(1, n)
		must(f.SetSheetRow(sheet, cell, &vals))
	}
	row("Thu", 1, "UBND XÃ DEMO")
	row("Thu", 2, "THU NGÂN SÁCH XÃ DEMO NĂM 2026")
	row("Thu", 3, nil, nil, nil, nil, nil, "Đơn vị tính: Triệu đồng")
	row("Thu", 4, "TT", "Nội dung", "Dự toán 2026", nil, "Thu ngân sách", nil, "Tỷ lệ % thu")
	row("Thu", 5, nil, nil, "TP giao", "Xã giao", "NSNN", "Thu xã hưởng")
	must(f.MergeCell("Thu", "C4", "D4"))
	must(f.MergeCell("Thu", "E4", "F4"))
	must(f.MergeCell("Thu", "G4", "G5"))
	row("Thu", 6, "A", "TỔNG THU NỘI ĐỊA", 100, 110, 90, 70, 0.9)
	row("Thu", 7, "I", "THUẾ THÀNH PHỐ QUẢN LÝ THU", 40, 40, 36, 30)
	row("Thu", 8, "1", "    Thu từ doanh nghiệp nhà nước", 10, 10, 8, 6)
	row("Thu", 9, nil, "        Thuế GTGT", 6, 6, 5, 4.25)
	row("Thu", 10, nil, "        Thuế TNDN", 4, 4, 3, 2)
	row("Thu", 11, "B", "THU NGÂN SÁCH ĐỊA PHƯƠNG", 300, 300, 280, 280)

	_, err := f.NewSheet("chi NS")
	must(err)
	row("chi NS", 1, "BÁO CÁO CHI NGÂN SÁCH NHÀ NƯỚC XÃ DEMO NĂM 2026")
	row("chi NS", 2, "Đơn vị tính: Triệu đồng")
	row("chi NS", 4, "STT", "Chỉ tiêu", "Dự toán năm", "Chi ngân sách", "So sánh TH/DT (%)")
	row("chi NS", 5, nil, "Tổng số", 400, 300)
	row("chi NS", 6, "A", "CHI NGÂN SÁCH NHÀ NƯỚC", 400, 300)
	row("chi NS", 7, "I", "Chi đầu tư phát triển", 60, 80)
	row("chi NS", 8, "I.1", "Đầu tư cho các dự án", nil, 80)
	row("chi NS", 9, "1.1", "Chi quốc phòng", 0, 59.5)
	row("chi NS", 10, "1.2", "Chi giáo dục", 0, 20.5)
	row("chi NS", 11, "II", "Chi thường xuyên", 340, 220)
	row("chi NS", 12, "10", "Chi các hoạt động kinh tế", 340, 220)
	row("chi NS", 13, "10.1", "- Chi giao thông vận tải", "-", 220)
	// A stray figure outside the headed columns — the preparer's scratch, dropped.
	must(f.SetCellValue("chi NS", "G7", 129249))

	_, err = f.NewSheet("Ghi chú")
	must(err)
	row("Ghi chú", 1, "TT", "nháp của kế toán")
	must(f.SetSheetVisible("Ghi chú", false))

	buf, err := f.WriteToBuffer()
	must(err)
	return buf.Bytes()
}

const (
	pathBudgetImport  = "/api/v1/budget-sheets/imports"
	pathBudgetPreview = "/api/v1/budget-sheets/import-previews"
)

// sendBudgetFile posts the file as multipart to path?year=2026 with an Idempotency-Key.
func (m *mayChuNganSach) sendBudgetFile(t *testing.T, host, path string, file []byte, p *authz.Principal) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	hdr := textproto.MIMEHeader{}
	hdr.Set("Content-Disposition", `form-data; name="file"; filename="bao-cao-thu-chi-2026.xlsx"`)
	hdr.Set("Content-Type", catalogueXLSXMIME)
	part, err := mw.CreatePart(hdr)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(file); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "https://"+host+path, io.Reader(&buf))
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	r.Header.Set("Content-Type", mw.FormDataContentType())
	r.Header.Set(idem.Header, "01JIDEMPOTENCYKEYNAPEXCEL0")
	if p != nil {
		r = r.WithContext(context.WithValue(r.Context(), khoaChuTheGhi{}, *p))
	}
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)
	return w
}

type budgetImportRoute struct {
	name, path string
	ok         int
}

func budgetImportRoutes() []budgetImportRoute {
	return []budgetImportRoute{
		{"preview", pathBudgetPreview + "?year=2026", http.StatusOK},
		{"import", pathBudgetImport + "?year=2026", http.StatusCreated},
	}
}

func importCallsOf(m *mayChuNganSach) int { return m.ghi.previewImportCalls + m.ghi.importCalls }

// --- rule 5, invariant 7: the four cases, on both routes ----------------------------------------------

func TestBudgetImport_AsksForBudgetUpdate(t *testing.T) {
	for _, rt := range budgetImportRoutes() {
		m := dungMayChuNganSach(t)
		m.sendBudgetFile(t, hostA, rt.path, budgetWorkbook(t), canBoGhi(xaA))
		if got := m.checker.hoiKhoaCuoi(); got != "budget.update" {
			t.Errorf("%s asked for %q, want budget.update (the create route's key, seeded in `quyen`)", rt.name, got)
		}
	}
}

func TestBudgetImport_401WithoutPrincipal(t *testing.T) {
	for _, rt := range budgetImportRoutes() {
		m := dungMayChuNganSach(t)
		m.capQuyen(xaA, "budget.read", "budget.update", "budget.confirm")
		doiMa(t, m.sendBudgetFile(t, hostA, rt.path, budgetWorkbook(t), nil), http.StatusUnauthorized)
		if importCallsOf(m) != 0 {
			t.Errorf("%s: not signed in, yet the use case ran", rt.name)
		}
	}
}

func TestBudgetImport_403WrongPermission(t *testing.T) {
	for _, held := range []authz.Perm{"admin.lookup", "budget.read"} {
		for _, rt := range budgetImportRoutes() {
			m := dungMayChuNganSach(t)
			m.capQuyen(xaA, held)
			doiMa(t, m.sendBudgetFile(t, hostA, rt.path, budgetWorkbook(t), canBoGhi(xaA)), http.StatusForbidden)
			if importCallsOf(m) != 0 {
				t.Errorf("%s holding %s: the use case ran", rt.name, held)
			}
		}
	}
}

func TestBudgetImport_403RightPermissionWrongCommune(t *testing.T) {
	for _, rt := range budgetImportRoutes() {
		m := dungMayChuNganSach(t)
		m.capQuyen(xaA, "budget.read", "budget.update", "budget.confirm")
		doiMa(t, m.sendBudgetFile(t, hostB, rt.path, budgetWorkbook(t), canBoGhi(xaB)), http.StatusForbidden)
		if importCallsOf(m) != 0 {
			t.Errorf("%s: granted in commune A, yet it ran in commune B", rt.name)
		}
	}
}

func TestBudgetImport_2xxRightCommuneStaffCodeAndParsedWorkbook(t *testing.T) {
	for _, rt := range budgetImportRoutes() {
		m := dungMayChuNganSach(t)
		m.capQuyen(xaA, "budget.update")
		w := m.sendBudgetFile(t, hostA, rt.path, budgetWorkbook(t), canBoGhi(xaA))
		doiMa(t, w, rt.ok)
		if m.ghi.xaCuoi != xaA {
			t.Errorf("%s ran in commune %q", rt.name, m.ghi.xaCuoi)
		}
		if rt.ok == http.StatusCreated && (m.ghi.nguoiCuoi.ID != maCanBoGhi || m.ghi.nguoiCuoi.ID == idNoiBo) {
			t.Errorf("trail actor = %q, want the staff code", m.ghi.nguoiCuoi.ID)
		}
		req := m.ghi.lastImport
		if req.Year != 2026 || req.FileName != "bao-cao-thu-chi-2026.xlsx" || len(req.Sheets) != 2 {
			t.Fatalf("%s: request = year %d file %q sheets %d", rt.name, req.Year, req.FileName, len(req.Sheets))
		}
		var out budgetImportOut
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || !out.Valid || len(out.Sheets) != 2 {
			t.Fatalf("%s body = %s", rt.name, w.Body.String())
		}
		// The hidden notes tab is reported, not loaded.
		if len(out.Warnings) == 0 || !strings.Contains(out.Warnings[0].Message, "ẩn") {
			t.Errorf("%s: hidden tab not reported: %+v", rt.name, out.Warnings)
		}
		if rt.ok == http.StatusCreated && (out.Sheets[1].Sheet == nil || out.Sheets[1].Sheet.Code != "NS-2026-CHI-01") {
			t.Errorf("import body lacks the created sheet: %s", w.Body.String())
		}
	}
}

// THE REAL WORKBOOK THROUGH core/xlsx.ReadSheets INTO THE PARSER: merged two-row header, depth rules,
// indentation, units, "Tổng số", and number cells read as numbers.
func TestBudgetImport_RealWorkbookParsedEndToEnd(t *testing.T) {
	m := dungMayChuNganSach(t)
	m.capQuyen(xaA, "budget.update")
	doiMa(t, m.sendBudgetFile(t, hostA, pathBudgetPreview+"?year=2026", budgetWorkbook(t), canBoGhi(xaA)), http.StatusOK)
	sheets := m.ghi.lastImport.Sheets
	rev, exp := sheets[0], sheets[1]
	if rev.Kind != domain.BangThu || exp.Kind != domain.BangChi || rev.Unit != domain.DonViTrieuDong {
		t.Fatalf("kinds/unit = %q %q %q", rev.Kind, exp.Kind, rev.Unit)
	}
	var names []string
	for _, c := range rev.Columns {
		names = append(names, c.Name+"="+string(c.Role))
	}
	if got := strings.Join(names, "|"); got != "Dự toán 2026 TP giao=du-toan-tp-giao|Dự toán 2026 Xã giao=du-toan-xa-giao|"+
		"Thu ngân sách NSNN=thu-nsnn|Thu ngân sách Thu xã hưởng=thu-xa-huong|Tỷ lệ % thu=" {
		t.Errorf("revenue columns = %s", got)
	}
	// Indentation-only lines under "1", at the same depth as each other.
	vat := rev.Lines[3]
	if vat.Name != "Thuế GTGT" || rev.Lines[vat.Parent].TT != "1" || vat.Depth != 3 || rev.Lines[4].Parent != vat.Parent {
		t.Errorf("indent fallback: %+v / %+v", vat, rev.Lines[4])
	}
	// 4.25 triệu, a NUMBER cell → 4 250 000 đồng exactly.
	if vat.Values[3] != 4_250_000 {
		t.Errorf("GTGT thu xã hưởng = %d", vat.Values[3])
	}
	codes := map[string]int{}
	for _, l := range exp.Lines {
		codes[l.TT] = l.Depth
	}
	if codes["A"] != 0 || codes["I"] != 1 || codes["I.1"] != 2 || codes["1.1"] != 3 || codes["10"] != 2 || codes["10.1"] != 3 {
		t.Errorf("expenditure depths = %v", codes)
	}
	if exp.Lines[exp.Headline].Name != "Tổng số" {
		t.Errorf("headline = %q", exp.Lines[exp.Headline].Name)
	}
	if op := exp.Columns[2].Operands; op.Numerator == nil || *op.Numerator != 1 || *op.Denominator != 0 {
		t.Errorf("chi %% operands = %+v", op)
	}
	if len(exp.Columns) != 3 {
		t.Errorf("the stray figure in column G became a column: %+v", exp.Columns)
	}
}

func TestBudgetImport_StatusOfEachRefusal(t *testing.T) {
	sheet := domain.BangNganSach{Ma: "NS-2026-CHI-01", Nam: 2026, Loai: domain.BangChi}
	cases := []struct {
		name string
		err  error
		code int
		want string
	}{
		{"hand entries", fmt.Errorf("ngan_sach: nạp Excel cho xã X: %w", domain.HandEntriesError(sheet, []string{"1.1 Chi quốc phòng"}, 1, 0)),
			http.StatusConflict, "budget_sheet_has_entries"},
		{"closed month", fmt.Errorf("ngan_sach: nạp Excel cho xã X: %w", domain.ImportPeriodClosedError(
			domain.BudgetPeriodClose{Code: "CK-2026-05-01", Year: 2026, Month: 5})),
			http.StatusConflict, "budget_period_closed"},
	}
	for _, c := range cases {
		m := dungMayChuNganSach(t)
		m.capQuyen(xaA, "budget.update")
		m.ghi.importErr = c.err
		w := m.sendBudgetFile(t, hostA, pathBudgetImport+"?year=2026", budgetWorkbook(t), canBoGhi(xaA))
		doiMa(t, w, c.code)
		if !strings.Contains(w.Body.String(), c.want) || strings.Contains(w.Body.String(), "cho xã X") {
			t.Errorf("%s: body = %s — the conflict's own sentence, never the commune-tagged wrap", c.name, w.Body.String())
		}
	}
}

func TestBudgetImport_FileThatDoesNotParse(t *testing.T) {
	f := excelize.NewFile()
	_ = f.SetSheetRow("Sheet1", "A1", &[]any{"Ghi chú"})
	buf, _ := f.WriteToBuffer()
	f.Close()
	m := dungMayChuNganSach(t)
	m.capQuyen(xaA, "budget.update")
	// Preview: 200 with the errors. Import: 400 import_invalid. Neither reaches the use case.
	w := m.sendBudgetFile(t, hostA, pathBudgetPreview+"?year=2026", buf.Bytes(), canBoGhi(xaA))
	doiMa(t, w, http.StatusOK)
	var out budgetImportOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || out.Valid || len(out.Errors) == 0 {
		t.Errorf("preview = %s", w.Body.String())
	}
	w = m.sendBudgetFile(t, hostA, pathBudgetImport+"?year=2026", buf.Bytes(), canBoGhi(xaA))
	doiMa(t, w, http.StatusBadRequest)
	if !strings.Contains(w.Body.String(), "import_invalid") || importCallsOf(m) != 0 {
		t.Errorf("import = %s, calls %d", w.Body.String(), importCallsOf(m))
	}
}

func TestBudgetImport_YearIsRequiredNeverDefaulted(t *testing.T) {
	for _, q := range []string{"", "?year=abc", "?year=1999", "?year=2026&year=2027"} {
		m := dungMayChuNganSach(t)
		m.capQuyen(xaA, "budget.update")
		doiMa(t, m.sendBudgetFile(t, hostA, pathBudgetImport+q, budgetWorkbook(t), canBoGhi(xaA)), http.StatusBadRequest)
		if importCallsOf(m) != 0 {
			t.Errorf("%q: the use case ran", q)
		}
	}
}
