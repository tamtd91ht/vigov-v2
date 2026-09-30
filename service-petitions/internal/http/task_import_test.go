package http

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// Tests for POST /api/v1/tasks/imports and GET /api/v1/tasks/import-template (P11).
//
//	PROVED HERE   rule 5 invariant 7's four cases on both routes, with no import call on a refusal ·
//	              the upload is read from `file`, bounded at 2 MB (413), refused as unreadable when it
//	              is not a zip, has too many parts or too much declared unzipped data — before excelize
//	              parses it · the sheet reaches the use case as text, heading first, a date CELL turned
//	              into the domain's text (a whole-day serial stays date-only, so the domain refuses it) ·
//	              `dry_run` accepts only `true` · 201 when committed with the codes, 200 when nothing was
//	              written, the use case's sentinels mapped to 400 / 422 / 503 / 500 · the template is a
//	              real workbook whose heading row is the domain's, IN ORDER · the duplicate protection
//	              refuses a request with no Idempotency-Key.
//
//	NOT PROVED    the row rules (domain), the lookups and the one transaction (app).

type taskImportFake struct {
	calls  int
	sheet  [][]string
	dryRun bool
	actor  audit.Actor
	tenant tenant.ID
	res    app.TaskImportResult
	err    error
}

func (f *taskImportFake) Import(ctx context.Context, sheet [][]string, dryRun bool, actor audit.Actor) (
	app.TaskImportResult, error) {
	f.calls++
	f.sheet, f.dryRun, f.actor, f.tenant = sheet, dryRun, actor, tenant.MustFrom(ctx)
	return f.res, f.err
}

const (
	taskImportPath   = "/api/v1/tasks/imports"
	taskTemplatePath = "/api/v1/tasks/import-template"
)

// importWorkbook builds a real .xlsx: the template's heading row, then the given rows as text cells,
// and optionally a real DATE cell in the deadline column of the first data row.
func importWorkbook(t *testing.T, rows [][]string, dueSerial float64) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	all := append([][]string{domain.TaskImportHeadings}, rows...)
	for r, cells := range all {
		for c, v := range cells {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+1)
			if err := f.SetCellStr("Sheet1", cell, v); err != nil {
				t.Fatal(err)
			}
		}
	}
	if dueSerial > 0 {
		if err := f.SetCellFloat("Sheet1", "F2", dueSerial, 6, 64); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// postImport sends a multipart upload through the real edge chain plus idem.Middleware with an
// in-memory store, the way cmd/server wires it.
func postImport(t *testing.T, m *mayChu, query string, file []byte, p *authz.Principal, key string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if file != nil {
		part, err := mw.CreateFormFile("file", "tep.xlsx")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(file); err != nil {
			t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "https://"+hostA+taskImportPath+query, &body)
	r.Host = hostA
	r.RemoteAddr = "10.0.0.7:51000"
	r.Header.Set("Content-Type", mw.FormDataContentType())
	if key != "" {
		r.Header.Set(idem.Header, key)
	}
	if p != nil {
		r = r.WithContext(authz.Into(r.Context(), *p))
	}
	w := httptest.NewRecorder()
	idem.Middleware(khoIdemMoi(), slog.New(slog.NewTextHandler(io.Discard, nil)))(m.h).ServeHTTP(w, r)
	return w
}

const importKey = "01JIMPORTKEY0000000000000A"

func importCreatorGrant(m *mayChu, t *testing.T) {
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
			xaA: {idCanBo: {authz.Perm("task.create"): true}},
			xaB: {idCanBo: {authz.Perm("task.create"): true}},
		}}
	})
}

// --- rule 5, invariant 7 — the import ----------------------------------------------------------------

func TestTaskImportRBAC(t *testing.T) {
	file := importWorkbook(t, [][]string{{"Việc"}}, 0)

	m := dungMayChu(t)
	doiMa(t, postImport(t, m, "", file, nil, importKey), http.StatusUnauthorized)

	m = dungMayChu(t) // the default grant has task.read only
	doiMa(t, postImport(t, m, "", file, canBoCuaXa(xaA), importKey), http.StatusForbidden)

	m = dungMayChu(t)
	importCreatorGrant(m, t)
	doiMa(t, postImport(t, m, "", file, canBoCuaXa(xaB), importKey), http.StatusUnauthorized)
	if m.taskImport.calls != 0 {
		t.Fatalf("đã nhập %d lần dù bị từ chối", m.taskImport.calls)
	}

	m.taskImport.res = app.TaskImportResult{TotalRows: 1, Created: 1, Committed: true, Codes: []string{"NV41"}}
	w := postImport(t, m, "", file, canBoCuaXa(xaA), importKey)
	doiMa(t, w, http.StatusCreated)
	var out taskImportResultOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || !out.Committed || strings.Join(out.Codes, ",") != "NV41" {
		t.Errorf("thân = %s", w.Body.String())
	}
	f := m.taskImport
	if f.calls != 1 || f.tenant != xaA || f.actor.ID != maCanBo || f.dryRun {
		t.Errorf("gọi use case: %d lần, xã %q, người %q, thử %v", f.calls, f.tenant, f.actor.ID, f.dryRun)
	}
	if len(f.sheet) != 2 || f.sheet[0][0] != domain.TaskImportHeadings[0] || f.sheet[1][0] != "Việc" {
		t.Errorf("bảng tính xuống use case = %v", f.sheet)
	}
}

func TestTaskImportNeedsIdempotencyKey(t *testing.T) {
	m := dungMayChu(t)
	importCreatorGrant(m, t)
	doiMa(t, postImport(t, m, "", importWorkbook(t, [][]string{{"Việc"}}, 0), canBoCuaXa(xaA), ""),
		http.StatusBadRequest)
	if m.taskImport.calls != 0 {
		t.Error("nhập dù thiếu Idempotency-Key — nộp hai lần sẽ cấp mã hai lần")
	}
}

// --- the upload's defences --------------------------------------------------------------------------

func TestTaskImportUploadDefences(t *testing.T) {
	var zipMany bytes.Buffer
	zw := zip.NewWriter(&zipMany)
	for i := 0; i < taskImportMaxZipEntries+1; i++ {
		w, _ := zw.Create(fmt.Sprintf("x/%d.xml", i))
		_, _ = w.Write([]byte("<a/>"))
	}
	_ = zw.Close()

	var zipHuge bytes.Buffer
	zw = zip.NewWriter(&zipHuge)
	w, _ := zw.Create("xl/worksheets/sheet1.xml")
	_, _ = w.Write(bytes.Repeat([]byte(" "), taskImportMaxUnzipped+1)) // compresses to almost nothing
	_ = zw.Close()

	for name, tc := range map[string]struct {
		file   []byte
		status int
		code   string
	}{
		"không phải zip":       {[]byte("chỉ là chữ"), http.StatusBadRequest, "import_unreadable"},
		"quá nhiều phần":       {zipMany.Bytes(), http.StatusBadRequest, "import_unreadable"},
		"giải nén quá lớn":     {zipHuge.Bytes(), http.StatusBadRequest, "import_unreadable"},
		"quá 2 MB":             {bytes.Repeat([]byte("a"), taskImportMaxBytes+1), http.StatusRequestEntityTooLarge, "import_too_large"},
		"không có trường file": {nil, http.StatusBadRequest, "invalid_request"},
	} {
		t.Run(name, func(t *testing.T) {
			m := dungMayChu(t)
			importCreatorGrant(m, t)
			w := postImport(t, m, "", tc.file, canBoCuaXa(xaA), importKey)
			doiMa(t, w, tc.status)
			if e := loiTra(t, w); e.Code != tc.code {
				t.Errorf("mã lỗi = %q, muốn %q", e.Code, tc.code)
			}
			if m.taskImport.calls != 0 {
				t.Error("tệp bị từ chối vẫn tới use case")
			}
		})
	}
}

func TestTaskImportDateCellBecomesDomainText(t *testing.T) {
	m := dungMayChu(t)
	importCreatorGrant(m, t)
	// 46296.708333… = 2026-10-01 17:00 wall clock; 46296 alone = the date with no time.
	postImport(t, m, "?dry_run=true", importWorkbook(t, [][]string{{"Việc"}}, 46296+17.0/24), canBoCuaXa(xaA), importKey)
	if got := m.taskImport.sheet[1][taskImportDueColumn]; got != "2026-10-01 17:00" {
		t.Errorf("ô ngày giờ = %q, muốn 2026-10-01 17:00", got)
	}
	if !m.taskImport.dryRun {
		t.Error("dry_run=true không tới use case")
	}
	if importDueCellText("46296") != "2026-10-01" {
		t.Error("ô chỉ có ngày phải giữ dạng chỉ ngày để miền từ chối")
	}
	if importDueCellText("30/09/2026 17:00") != "30/09/2026 17:00" {
		t.Error("ô chữ phải đi nguyên")
	}
}

func TestTaskImportDryRunSpelling(t *testing.T) {
	m := dungMayChu(t)
	importCreatorGrant(m, t)
	doiMa(t, postImport(t, m, "?dry_run=1", importWorkbook(t, [][]string{{"Việc"}}, 0), canBoCuaXa(xaA), importKey),
		http.StatusBadRequest)
	if m.taskImport.calls != 0 {
		t.Error("dry_run sai chính tả vẫn nhập — có thể nhập THẬT khi người dùng muốn thử")
	}
}

func TestTaskImportResultAndErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		res    app.TaskImportResult
		err    error
		status int
		code   string
	}{
		{app.TaskImportResult{TotalRows: 2, Errors: []domain.TaskImportError{{Row: 3, Column: "Tên nhiệm vụ", Message: "thiếu"}}},
			nil, http.StatusOK, ""},
		{app.TaskImportResult{}, app.ErrTaskImportLayout, http.StatusBadRequest, "import_layout"},
		// ADR 0065 NV5: the old template (with "cơ quan chủ trì" / "chuyên viên theo dõi") has its own code.
		{app.TaskImportResult{}, app.ErrTaskImportRetiredColumns, http.StatusBadRequest, "import_retired_columns"},
		{app.TaskImportResult{}, app.ErrTaskImportTooManyRows, http.StatusUnprocessableEntity, "import_too_many_rows"},
		{app.TaskImportResult{}, fmt.Errorf("%w: x", app.ErrTaskImportUnchecked), http.StatusServiceUnavailable, "import_unchecked"},
		{app.TaskImportResult{}, errors.New("pg: db-07 down"), http.StatusInternalServerError, "internal"},
	} {
		m := dungMayChu(t)
		importCreatorGrant(m, t)
		m.taskImport.res, m.taskImport.err = tc.res, tc.err
		w := postImport(t, m, "", importWorkbook(t, [][]string{{"Việc"}}, 0), canBoCuaXa(xaA), importKey)
		doiMa(t, w, tc.status)
		if tc.code == "" {
			var out taskImportResultOut
			if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || out.Committed || len(out.Errors) != 1 ||
				out.Errors[0].Row != 3 {
				t.Errorf("báo cáo dòng = %s", w.Body.String())
			}
			continue
		}
		if e := loiTra(t, w); e.Code != tc.code || strings.Contains(e.Message, "db-07") {
			t.Errorf("%v: lỗi = %+v", tc.err, e)
		}
	}
}

// --- the template ------------------------------------------------------------------------------------

func TestTaskImportTemplate(t *testing.T) {
	m := dungMayChu(t)
	doiMa(t, m.goi(t, http.MethodGet, hostA, taskTemplatePath, nil), http.StatusUnauthorized)
	doiMa(t, m.goi(t, http.MethodGet, hostA, taskTemplatePath, canBoCuaXa(xaA)), http.StatusForbidden)
	importCreatorGrant(m, t)
	doiMa(t, m.goi(t, http.MethodGet, hostA, taskTemplatePath, canBoCuaXa(xaB)), http.StatusUnauthorized)

	w := m.goi(t, http.MethodGet, hostA, taskTemplatePath, canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	if cd := w.Header().Get("Content-Disposition"); cd != `attachment; filename="mau-nhap-nhiem-vu.xlsx"` {
		t.Errorf("Content-Disposition = %q", cd)
	}
	x, err := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatalf("mẫu không phải xlsx: %v", err)
	}
	defer x.Close()
	rows, _ := x.GetRows("Nhiem vu")
	if len(rows) != 2 || strings.Join(rows[0], "|") != strings.Join(domain.TaskImportHeadings, "|") {
		t.Fatalf("hàng tiêu đề mẫu = %v", rows)
	}
	// THE MODEL READS ITS OWN TEMPLATE: the example row passes the row rules.
	if _, errs := domain.ParseTaskImportRow(2, rows[1]); len(errs) != 0 {
		t.Errorf("dòng mẫu không qua luật của chính nó: %+v", errs)
	}
}
