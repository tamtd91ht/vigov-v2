package http

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/xlsx"
	"github.com/vihat/vigov/service-documents/internal/app"
	"github.com/vihat/vigov/service-documents/internal/domain"
)

// THE CITIZEN-LETTER EXCEL IMPORT AND THE REPORT EXPORT — four routes. What these tests defend:
//
//  1. rule 5, invariant 7 on all four: 401 · 403 wrong permission · 403 right permission wrong
//     commune · 2xx; the export needs BOTH `report.export` and `petition.read`;
//  2. the template round-trips through the preview;
//  3. a file with bad rows is reported row by row and never reaches the use case; the import passes
//     the parsed rows and the caller (business code) to the use case; the use case's refusals map;
//  4. NO RESPONSE ECHOES A CELL, and THE EXPORTED WORKBOOK CARRIES NO PERSONAL DATA.
//
// The use cases are faked here; that every row goes through Book's own code with source `nhap-excel`,
// all or nothing, is proven in internal/app/citizen_letter_import_test.go over the fake driver.

// --- fakes ------------------------------------------------------------------------------------------

type letterImportFake struct {
	mu                sync.Mutex
	previews, imports int
	lastTenant        tenant.ID
	lastCaller        app.LetterCaller
	lastRows          []domain.LetterImportRow
	err               error
}

func (f *letterImportFake) note(ctx context.Context, rows []domain.LetterImportRow, c app.LetterCaller) {
	f.lastTenant, f.lastRows, f.lastCaller = tenant.MustFrom(ctx), rows, c
}

func importedFrom(rows []domain.LetterImportRow, booked bool) []app.ImportedLetter {
	out := make([]app.ImportedLetter, 0, len(rows))
	for i, r := range rows {
		il := app.ImportedLetter{Row: r.Row, Year: 2026, Type: r.Type, ReceivedDate: r.ReceivedDate,
			SenderUnknown: r.SenderName == ""}
		if booked {
			il.ID, il.Number = fmt.Sprintf("dt-nhap-%d", i), 10+i
		}
		out = append(out, il)
	}
	return out
}

func (f *letterImportFake) Preview(ctx context.Context, rows []domain.LetterImportRow, c app.LetterCaller) (app.LetterImportResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.previews++
	f.note(ctx, rows, c)
	if f.err != nil {
		return app.LetterImportResult{}, f.err
	}
	return app.LetterImportResult{Letters: importedFrom(rows, false)}, nil
}

func (f *letterImportFake) Import(ctx context.Context, rows []domain.LetterImportRow, c app.LetterCaller) (app.LetterImportResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.imports++
	f.note(ctx, rows, c)
	if f.err != nil {
		return app.LetterImportResult{}, f.err
	}
	return app.LetterImportResult{Letters: importedFrom(rows, true)}, nil
}

type letterExportFake struct {
	calls      int
	lastTenant tenant.ID
	lastActor  audit.Actor
	lastYear   int
	err        error
}

// exportSampleRows are letters FULL OF PERSONAL DATA, two units and one with none — the report built
// from them must reach the file without any of it.
func exportSampleRows() []domain.CitizenLetter {
	at := time.Date(2026, 9, 2, 3, 0, 0, 0, time.UTC)
	mk := func(id string, t domain.LetterType, unit string) domain.CitizenLetter {
		return domain.CitizenLetter{ID: id, Year: 2026, Number: 1, Type: t, ReceivedDate: at, CreatedAt: at,
			SenderName: ordinaryName, SenderPhone: fakePhone, SenderAddress: senderAddress, Summary: ordinarySummary,
			Status: domain.LetterStatusNew, HoldingUnitID: unit}
	}
	return []domain.CitizenLetter{
		mk("l1", domain.LetterTypeFeedback, "bp-dia-chinh"),
		mk("l2", domain.LetterTypeDenunciation, ""),
		mk("l3", domain.LetterTypeComplaint, "bp-go"),
	}
}

func (f *letterExportFake) Export(ctx context.Context, year int, actor audit.Actor,
	render func(app.LetterReportSheet) ([]byte, error)) ([]byte, error) {
	f.calls++
	f.lastTenant, f.lastActor, f.lastYear = tenant.MustFrom(ctx), actor, year
	if f.err != nil {
		return nil, f.err
	}
	return render(app.LetterReportSheet{
		Report: domain.BuildLetterReport(year, exportSampleRows(), testNow),
		UnitNames: map[string]app.LetterReportUnitName{
			"bp-dia-chinh": {Name: "Địa chính - Xây dựng", Known: true},
			"bp-go":        {Name: "Tư pháp - Hộ tịch", Known: true, Removed: true},
		},
	})
}

// --- harness ----------------------------------------------------------------------------------------

const (
	pathLetterTemplate = "/api/v1/citizen-letters/import-template"
	pathLetterPreview  = "/api/v1/citizen-letters/import-previews"
	pathLetterImport   = "/api/v1/citizen-letters/imports"
	pathLetterExport   = "/api/v1/citizen-letter-report/exports?year=2026"
)

func letterHeader() []string { return domain.LetterImportColumns() }

func validLetterFile(t *testing.T) []byte {
	return workbook(t, letterHeader(),
		[]string{"2026-09-10", ordinaryName, senderAddress, fakePhone, "kiến nghị", ordinarySummary, ""})
}

func uploadLetters(t *testing.T, m *mayChu, path, host string, p *authz.Principal, data []byte, key string) *httptest.ResponseRecorder {
	t.Helper()
	ct, body := multipartFile(t, "so-don-thu.xlsx", data)
	r := httptest.NewRequest(http.MethodPost, "https://"+host+path, body)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	r.Header.Set("Content-Type", ct)
	r.Header.Set(idem.Header, key)
	if p != nil {
		r = r.WithContext(context.WithValue(r.Context(), khoaChuTheThu{}, *p))
	}
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)
	return w
}

type letterFileRoute struct {
	name string
	keys []authz.Perm
	call func(m *mayChu, t *testing.T, host string, p *authz.Principal) *httptest.ResponseRecorder
	ok   int
}

func letterFileRoutes() []letterFileRoute {
	create := []authz.Perm{"petition.create"}
	return []letterFileRoute{
		{"mẫu", create, func(m *mayChu, t *testing.T, host string, p *authz.Principal) *httptest.ResponseRecorder {
			return m.goi(t, http.MethodGet, host, pathLetterTemplate, p)
		}, http.StatusOK},
		{"xem trước", create, func(m *mayChu, t *testing.T, host string, p *authz.Principal) *httptest.ResponseRecorder {
			return uploadLetters(t, m, pathLetterPreview, host, p, validLetterFile(t), "01JLETTERPREVIEWKEY000000")
		}, http.StatusOK},
		{"nhập", create, func(m *mayChu, t *testing.T, host string, p *authz.Principal) *httptest.ResponseRecorder {
			return uploadLetters(t, m, pathLetterImport, host, p, validLetterFile(t), "01JLETTERIMPORTKEY0000000")
		}, http.StatusCreated},
		{"xuất báo cáo", []authz.Perm{"report.export", "petition.read"}, func(m *mayChu, t *testing.T, host string, p *authz.Principal) *httptest.ResponseRecorder {
			return m.goi(t, http.MethodGet, host, pathLetterExport, p)
		}, http.StatusOK},
	}
}

func useCaseCalls(m *mayChu) int {
	return m.letterImport.previews + m.letterImport.imports + m.letterExport.calls
}

// --- (1) rule 5, invariant 7 ------------------------------------------------------------------------

func TestLetterFile_401WithoutSession(t *testing.T) {
	for _, rt := range letterFileRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			m := dungMayChuCoIdem(t)
			m.capQuyen(xaA, rt.keys...)
			doiMa(t, rt.call(m, t, hostA, nil), http.StatusUnauthorized)
			if useCaseCalls(m) != 0 {
				t.Error("chưa đăng nhập mà use case đã chạy")
			}
		})
	}
}

func TestLetterFile_403WrongPermission(t *testing.T) {
	for _, rt := range letterFileRoutes() {
		// Every proper subset of the keys the route needs — for the export, each of its two keys alone.
		grants := [][]authz.Perm{{"petition.read", "document.read"}}
		if len(rt.keys) == 2 {
			grants = [][]authz.Perm{{rt.keys[0]}, {rt.keys[1]}}
		}
		for _, g := range grants {
			t.Run(fmt.Sprintf("%s/%v", rt.name, g), func(t *testing.T) {
				m := dungMayChuCoIdem(t)
				m.capQuyen(xaA, g...)
				doiMa(t, rt.call(m, t, hostA, canBoCua(xaA)), http.StatusForbidden)
				if useCaseCalls(m) != 0 {
					t.Error("sai quyền mà use case đã chạy")
				}
			})
		}
	}
}

// Signed in at commune B, as commune B — the grants are commune A's.
func TestLetterFile_403RightPermissionWrongCommune(t *testing.T) {
	for _, rt := range letterFileRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			m := dungMayChuCoIdem(t)
			m.capQuyen(xaA, rt.keys...)
			doiMa(t, rt.call(m, t, hostB, canBoCua(xaB)), http.StatusForbidden)
			if useCaseCalls(m) != 0 {
				t.Error("quyền cấp ở xã khác mà vẫn chạy ở xã này")
			}
		})
	}
}

func TestLetterFile_OKWithPermissionInCommune(t *testing.T) {
	for _, rt := range letterFileRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			m := dungMayChuCoIdem(t)
			m.capQuyen(xaA, rt.keys...)
			doiMa(t, rt.call(m, t, hostA, canBoCua(xaA)), rt.ok)
			// The keys asked are the SEEDED literals (rule 5, 3c) — a fake checker grants any string.
			asked := map[authz.Perm]bool{}
			for _, k := range m.checker.hoiGi {
				asked[k] = true
			}
			for _, k := range rt.keys {
				if !asked[k] {
					t.Fatalf("tuyến không hỏi khoá %q (hỏi %v)", k, m.checker.hoiGi)
				}
			}
			if rt.name != "mẫu" {
				tn := m.letterImport.lastTenant
				if rt.name == "xuất báo cáo" {
					tn = m.letterExport.lastTenant
				}
				if tn != xaA {
					t.Fatalf("use case chạy ở xã %q, muốn %q", tn, xaA)
				}
			}
		})
	}
}

// --- (2) template round trip ------------------------------------------------------------------------

func TestLetterImport_TemplateRoundTrip(t *testing.T) {
	m := dungMayChuCoIdem(t)
	m.capQuyen(xaA, "petition.create")
	w := m.goi(t, http.MethodGet, hostA, pathLetterTemplate, canBoCua(xaA))
	doiMa(t, w, http.StatusOK)
	if ct := w.Header().Get("Content-Type"); ct != xlsxMIME {
		t.Fatalf("Content-Type = %q", ct)
	}
	pw := uploadLetters(t, m, pathLetterPreview, hostA, canBoCua(xaA), w.Body.Bytes(), "01JLETTERROUNDTRIP0000000")
	doiMa(t, pw, http.StatusOK)
	var out letterImportPreviewOut
	decodeJSON(t, pw, &out)
	if !out.Valid || len(out.Letters) != 1 || out.Letters[0].Row != 2 ||
		out.Letters[0].LetterType != string(domain.LetterTypeFeedback) || out.Letters[0].ReceivedDate != "2026-09-10" {
		t.Fatalf("tệp mẫu tải về không nhập lại được: %+v", out)
	}
}

// --- (3) preview, import ----------------------------------------------------------------------------

func TestLetterImport_PreviewWithBadRowsReportsEveryRowAndWritesNothing(t *testing.T) {
	m := dungMayChuCoIdem(t)
	m.capQuyen(xaA, "petition.create")
	const secretName, secretSummary = "Phạm Thị Bí Mật", "Nội dung không được lặp lại"
	data := workbook(t, letterHeader(),
		[]string{"2026-09-10", ordinaryName, "", fakePhone, "kiến nghị", ordinarySummary, ""}, // row 2 good
		[]string{"", "", "", "", "", "", ""},                                                  // row 3 blank
		[]string{"2026-09-11", "", "", "", "khác", secretSummary, ""},                         // row 4: sender, type
		[]string{"2026-12-01", secretName, "", "abc", "tố cáo", secretSummary, ""},            // row 5: future, phone
		[]string{"10/9/2026", secretName, "", "", "", "", ""},                                 // row 6: type, summary
	)
	w := uploadLetters(t, m, pathLetterPreview, hostA, canBoCua(xaA), data, "01JLETTERBADROWS000000000")
	doiMa(t, w, http.StatusOK)
	var out letterImportPreviewOut
	decodeJSON(t, w, &out)
	if out.Valid || len(out.Letters) != 0 {
		t.Fatalf("tệp lỗi mà valid / có dòng: %+v", out)
	}
	got := map[string]bool{}
	for _, e := range out.Errors {
		got[fmt.Sprintf("%d|%s", e.Row, e.Column)] = true
	}
	for _, want := range []string{
		"4|" + domain.LetterImportColSender, "4|" + domain.LetterImportColType,
		"5|" + domain.LetterImportColReceived, "5|" + domain.LetterImportColPhone,
		"6|" + domain.LetterImportColType, "6|" + domain.LetterImportColSummary,
	} {
		if !got[want] {
			t.Errorf("thiếu lỗi %s trong %+v", want, out.Errors)
		}
	}
	if len(out.Errors) != 6 {
		t.Errorf("%d lỗi, muốn 6: %+v", len(out.Errors), out.Errors)
	}
	if m.letterImport.previews+m.letterImport.imports != 0 {
		t.Error("tệp lỗi hình thức mà vẫn tới use case")
	}
	for _, cell := range []string{secretName, secretSummary, fakePhone, "abc", "khác"} {
		if strings.Contains(w.Body.String(), cell) {
			t.Fatalf("phản hồi lặp lại nội dung ô %q", cell)
		}
	}
}

func TestLetterImport_PassesParsedRowsAndTheCallerToTheUseCase(t *testing.T) {
	m := dungMayChuCoIdem(t)
	m.capQuyen(xaA, "petition.create")
	data := workbook(t, letterHeader(),
		[]string{"2026-09-10", "Không rõ", "", "", "Tố cáo", whistleSummary, ""},
		[]string{"5/9/2026", ordinaryName, senderAddress, fakePhone, "Kiến nghị, phản ánh", ordinarySummary, "dia-chinh"},
	)
	w := uploadLetters(t, m, pathLetterImport, hostA, canBoCua(xaA), data, "01JLETTERCOMMIT0000000000")
	doiMa(t, w, http.StatusCreated)
	var out letterImportCreatedOut
	decodeJSON(t, w, &out)
	if len(out.Created) != 2 || out.Created[0].ID == "" || out.Created[0].Number == 0 || !out.Created[0].SenderUnknown {
		t.Fatalf("= %+v", out)
	}
	rows := m.letterImport.lastRows
	if len(rows) != 2 || rows[0].Row != 2 || rows[0].SenderName != "" || rows[0].Type != domain.LetterTypeDenunciation ||
		rows[1].Type != domain.LetterTypeFeedback || rows[1].UnitCode != "dia-chinh" ||
		!rows[1].ReceivedDate.Equal(time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("dòng tới use case: %+v", rows)
	}
	c := m.letterImport.lastCaller
	if c.Actor.ID != maCanBo || !c.CanBook {
		t.Fatalf("người nhập = %+v, muốn mã cán bộ %q và CanBook", c, maCanBo)
	}
	// Nothing personal goes back: the uploader already has the file.
	for _, cell := range []string{ordinaryName, fakePhone, senderAddress, ordinarySummary, whistleSummary} {
		if strings.Contains(w.Body.String(), cell) {
			t.Fatalf("phản hồi nhập chứa %q", cell)
		}
	}
}

func TestLetterImport_ShapeErrorsRefuseTheWholeFileBeforeTheUseCase(t *testing.T) {
	m := dungMayChuCoIdem(t)
	m.capQuyen(xaA, "petition.create")
	data := workbook(t, letterHeader(),
		[]string{"2026-09-10", ordinaryName, "", "", "kiến nghị", ordinarySummary, ""},
		[]string{"", ordinaryName, "", "", "kiến nghị", ordinarySummary, ""}, // no date
	)
	w := uploadLetters(t, m, pathLetterImport, hostA, canBoCua(xaA), data, "01JLETTERSHAPEERR00000000")
	doiMa(t, w, http.StatusBadRequest)
	var out letterImportRejectedOut
	decodeJSON(t, w, &out)
	if out.Code != "import_invalid" || len(out.Errors) != 1 || out.Errors[0].Row != 3 ||
		out.Errors[0].Column != domain.LetterImportColReceived {
		t.Fatalf("= %+v", out)
	}
	if m.letterImport.imports != 0 {
		t.Fatal("tệp lỗi mà vẫn tới use case")
	}
}

func TestLetterImport_UseCaseRefusalsMap(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want int
	}{
		{"dòng bị quy tắc vào sổ từ chối", &app.LetterImportRejected{Errors: []domain.LetterImportError{
			{Row: 2, Column: domain.LetterImportColUnit, Message: "Không có bộ phận"}}}, http.StatusBadRequest},
		{"identity không trả lời", fmt.Errorf("x: %w", app.ErrLetterImportUnchecked), http.StatusServiceUnavailable},
		{"lỗi hệ thống", errors.New("boom"), http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := dungMayChuCoIdem(t)
			m.capQuyen(xaA, "petition.create")
			m.letterImport.err = tc.err
			w := uploadLetters(t, m, pathLetterImport, hostA, canBoCua(xaA), validLetterFile(t), "01JLETTERREFUSAL000000000")
			doiMa(t, w, tc.want)
			if tc.want != http.StatusBadRequest && strings.Contains(w.Body.String(), "boom") {
				t.Fatal("lỗi nội bộ lọt ra phản hồi")
			}
		})
	}
}

func TestLetterImport_RequiresAnIdempotencyKey(t *testing.T) {
	m := dungMayChuCoIdem(t)
	m.capQuyen(xaA, "petition.create")
	w := uploadLetters(t, m, pathLetterImport, hostA, canBoCua(xaA), validLetterFile(t), "")
	if w.Code < 400 || m.letterImport.imports != 0 {
		t.Fatalf("nhập không có Idempotency-Key mà vẫn chạy: %d", w.Code)
	}
}

// --- (4) the export -----------------------------------------------------------------------------------

func TestLetterReportExport_FileCarriesTheFiguresAndIsAttributed(t *testing.T) {
	m := dungMayChuCoIdem(t)
	m.capQuyen(xaA, "report.export", "petition.read")
	w := m.goi(t, http.MethodGet, hostA, pathLetterExport, canBoCua(xaA))
	doiMa(t, w, http.StatusOK)
	if ct := w.Header().Get("Content-Type"); ct != xlsxMIME {
		t.Fatalf("Content-Type = %q", ct)
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, "bao-cao-don-thu-nam-2026.xlsx") {
		t.Fatalf("Content-Disposition = %q", cd)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("tệp xuất phải no-store")
	}
	if m.letterExport.lastActor.ID != maCanBo || m.letterExport.lastYear != 2026 {
		t.Fatalf("xuất với người %q năm %d", m.letterExport.lastActor.ID, m.letterExport.lastYear)
	}
	text := sheetText(t, w.Body.Bytes())
	for _, want := range []string{"BÁO CÁO TIẾP NHẬN VÀ XỬ LÝ ĐƠN THƯ NĂM 2026", "TỔNG QUAN", "Tiếp nhận trong năm",
		"THEO LOẠI ĐƠN", "Tố cáo", "TIẾN ĐỘ XỬ LÝ THEO ĐƠN VỊ", "Chưa phân công", "Địa chính - Xây dựng",
		"Tư pháp - Hộ tịch (đã gỡ khỏi sơ đồ tổ chức)", "TIẾP NHẬN VÀ GIẢI QUYẾT THEO THÁNG", "Tháng 12"} {
		if !strings.Contains(text, want) {
			t.Errorf("tệp xuất thiếu %q", want)
		}
	}
}

func TestLetterReportExport_YearIsRequiredAndIdentityOutageIs503(t *testing.T) {
	m := dungMayChuCoIdem(t)
	m.capQuyen(xaA, "report.export", "petition.read")
	doiMa(t, m.goi(t, http.MethodGet, hostA, "/api/v1/citizen-letter-report/exports", canBoCua(xaA)), http.StatusBadRequest)
	if m.letterExport.calls != 0 {
		t.Fatal("thiếu năm mà vẫn xuất")
	}
	m.letterExport.err = fmt.Errorf("x: %w", app.ErrLetterReportNamesUnavailable)
	doiMa(t, m.goi(t, http.MethodGet, hostA, pathLetterExport, canBoCua(xaA)), http.StatusServiceUnavailable)
}

// The renderer over a report built from letters full of personal data: no name, phone, address or
// summary reaches ANY part of the workbook — not the sheet, not shared strings, not metadata.
func TestLetterReportExport_WorkbookContainsNoPersonalData(t *testing.T) {
	rows := exportSampleRows()
	b, err := renderLetterReport(app.LetterReportSheet{
		Report:    domain.BuildLetterReport(2026, rows, testNow),
		UnitNames: map[string]app.LetterReportUnitName{"bp-dia-chinh": {Name: "Địa chính", Known: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		part, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		for _, pii := range []string{ordinaryName, fakePhone, senderAddress, ordinarySummary, "bp-dia-chinh", "bp-go"} {
			if bytes.Contains(part, []byte(pii)) {
				t.Fatalf("phần %s của tệp xuất chứa %q", f.Name, pii)
			}
		}
	}
	text := sheetText(t, b)
	if !strings.Contains(text, "Bộ phận không xác định") {
		t.Error("bộ phận identity không biết phải in là không xác định, không in mã")
	}
}

// sheetText reads the first sheet back through the one reader of uploads and joins every cell.
func sheetText(t *testing.T, b []byte) string {
	t.Helper()
	rows, err := xlsx.ReadSheet(bytes.NewReader(b), int64(len(b)), xlsx.DefaultLimits)
	if err != nil {
		t.Fatalf("đọc lại tệp xuất: %v", err)
	}
	var sb strings.Builder
	for _, r := range rows {
		sb.WriteString(strings.Join(r, "|"))
		sb.WriteByte('\n')
	}
	return sb.String()
}
