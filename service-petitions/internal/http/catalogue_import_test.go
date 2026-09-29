package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
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
	"github.com/vihat/vigov/core/xlsx"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	docstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// THE SIX IMPORT ROUTES OF THE TWO TASK CATALOGUES, all `admin.lookup` (ADR 0059 §3). What is asserted
// here is what the HANDLER owns: the permission, the upload, the wire mapping, and which identifier
// reaches the trail. The rules are proved in domain/catalogue_import_test.go, the transaction in
// app/catalogue_import_test.go.

// catalogueImportFake stands in for *app.CatalogueImporter[T], recording the commune it was called in.
type catalogueImportFake struct {
	layout domain.CatalogueImportLayout

	calls     int
	lastRows  []domain.CatalogueImportRow
	lastActor audit.Actor
	lastXa    tenant.ID

	res app.CatalogueImportResult
	err error
}

func taskTypeImportsFake() *catalogueImportFake {
	return &catalogueImportFake{layout: domain.CatalogueOrderFromColumn, res: app.CatalogueImportResult{
		Batch:   "01JBATCH0000000000000000AA",
		Entries: []domain.PlannedCatalogueEntry{{Row: 2, ID: "lnv-1", Code: "theo-van-ban", Label: "Theo văn bản", Order: 3}},
	}}
}

func taskPriorityImportsFake() *catalogueImportFake {
	return &catalogueImportFake{layout: domain.CatalogueOrderFromPosition, res: app.CatalogueImportResult{
		Batch:   "01JBATCH0000000000000000BB",
		Entries: []domain.PlannedCatalogueEntry{{Row: 2, ID: "muu-1", Code: "khan", Label: "Khẩn", Order: 0}},
	}}
}

func (f *catalogueImportFake) Layout() domain.CatalogueImportLayout { return f.layout }

func (f *catalogueImportFake) Preview(ctx context.Context, rows []domain.CatalogueImportRow) (app.CatalogueImportResult, error) {
	f.calls++
	f.lastRows, f.lastXa = rows, tenant.MustFrom(ctx)
	return f.res, f.err
}

func (f *catalogueImportFake) Import(ctx context.Context, rows []domain.CatalogueImportRow, actor audit.Actor) (app.CatalogueImportResult, error) {
	f.calls++
	f.lastRows, f.lastActor, f.lastXa = rows, actor, tenant.MustFrom(ctx)
	return f.res, f.err
}

const catalogueImportKey = "01JCATALOGUEIMPORTKEY00000A"

// catalogueWorkbook builds a real .xlsx: header row, then rows as TEXT cells.
func catalogueWorkbook(t *testing.T, header []string, rows ...[]string) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	for r, cells := range append([][]string{header}, rows...) {
		for c, v := range cells {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+1)
			if err := f.SetCellStr("Sheet1", cell, v); err != nil {
				t.Fatal(err)
			}
		}
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

type catalogueImportRoute struct {
	name, method, path string
	ok                 int
	fake               func(m *mayChu) *catalogueImportFake
	workbook           func(t *testing.T) []byte
}

func typesFake(m *mayChu) *catalogueImportFake { return m.d.TaskTypeImports.(*catalogueImportFake) }
func prioritiesFake(m *mayChu) *catalogueImportFake {
	return m.d.TaskPriorityImports.(*catalogueImportFake)
}

func typesWorkbook(t *testing.T) []byte {
	return catalogueWorkbook(t, domain.CatalogueOrderFromColumn.Columns(),
		[]string{"Theo văn bản", "", "3"}, []string{"Cơ bản", "co-ban", ""})
}

func prioritiesWorkbook(t *testing.T) []byte {
	return catalogueWorkbook(t, domain.CatalogueOrderFromPosition.Columns(),
		[]string{"Khẩn", ""}, []string{"Thường", "thuong"})
}

func catalogueImportRoutes() []catalogueImportRoute {
	var out []catalogueImportRoute
	for _, c := range []struct {
		base string
		fake func(m *mayChu) *catalogueImportFake
		wb   func(t *testing.T) []byte
	}{
		{"/api/v1/task-types", typesFake, typesWorkbook},
		{"/api/v1/task-priorities", prioritiesFake, prioritiesWorkbook},
	} {
		out = append(out,
			catalogueImportRoute{c.base + " mẫu", "GET", c.base + "/import-template", http.StatusOK, c.fake, c.wb},
			catalogueImportRoute{c.base + " xem trước", "POST", c.base + "/import-previews", http.StatusOK, c.fake, c.wb},
			catalogueImportRoute{c.base + " nhập", "POST", c.base + "/imports", http.StatusCreated, c.fake, c.wb},
		)
	}
	return out
}

// sendCatalogue sends through the real edge chain plus idem.Middleware with an in-memory store, the
// way cmd/server wires it. file == nil sends a GET.
func sendCatalogue(t *testing.T, m *mayChu, method, host, path string, file []byte, p *authz.Principal, key string) *httptest.ResponseRecorder {
	t.Helper()
	var body io.Reader
	ct := ""
	if file != nil {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		hdr := textproto.MIMEHeader{}
		hdr.Set("Content-Disposition", `form-data; name="file"; filename="danh-muc.xlsx"`)
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
		body, ct = &buf, mw.FormDataContentType()
	}
	r := httptest.NewRequest(method, "https://"+host+path, body)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	if ct != "" {
		r.Header.Set("Content-Type", ct)
	}
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

func (rt catalogueImportRoute) call(t *testing.T, m *mayChu, host string, p *authz.Principal) *httptest.ResponseRecorder {
	t.Helper()
	if rt.method == "GET" {
		return sendCatalogue(t, m, "GET", host, rt.path, nil, p, "")
	}
	return sendCatalogue(t, m, "POST", host, rt.path, rt.workbook(t), p, catalogueImportKey)
}

// lookupGrant gives the harness's staff member `admin.lookup` IN COMMUNE A ONLY.
func lookupGrant(t *testing.T, m *mayChu) {
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
			xaA: {idCanBo: {authz.Perm("admin.lookup"): true}},
			xaB: {},
		}}
	})
}

func catalogueImportCalls(m *mayChu) int { return typesFake(m).calls + prioritiesFake(m).calls }

// --- rule 5, invariant 7: the four cases, for each of the six routes ------------------------------

func TestCatalogueImports_401WithoutPrincipal(t *testing.T) {
	m := dungMayChu(t)
	lookupGrant(t, m)
	for _, rt := range catalogueImportRoutes() {
		if w := rt.call(t, m, hostA, nil); w.Code != http.StatusUnauthorized {
			t.Errorf("%s: mã = %d, muốn 401", rt.name, w.Code)
		}
	}
	if n := catalogueImportCalls(m); n != 0 {
		t.Errorf("chưa đăng nhập mà use case đã chạy %d lần", n)
	}
}

// 403 with the harness default grant — `task.read` and `feedback.read`, real keys that are not
// `admin.lookup`.
func TestCatalogueImports_403WrongPermission(t *testing.T) {
	m := dungMayChu(t)
	for _, rt := range catalogueImportRoutes() {
		if w := rt.call(t, m, hostA, canBoCuaXa(xaA)); w.Code != http.StatusForbidden {
			t.Errorf("%s: mã = %d, muốn 403", rt.name, w.Code)
		}
	}
	if n := catalogueImportCalls(m); n != 0 {
		t.Errorf("sai quyền mà use case đã chạy %d lần", n)
	}
}

// 403 WITH `admin.lookup` HELD IN COMMUNE A, signed in properly at commune B (rule 5, invariant 3).
func TestCatalogueImports_403RightPermissionWrongCommune(t *testing.T) {
	m := dungMayChu(t)
	lookupGrant(t, m)
	for _, rt := range catalogueImportRoutes() {
		if w := rt.call(t, m, hostB, canBoCuaXa(xaB)); w.Code != http.StatusForbidden {
			t.Errorf("%s: mã = %d, muốn 403", rt.name, w.Code)
		}
	}
	if n := catalogueImportCalls(m); n != 0 {
		t.Errorf("sai xã mà use case đã chạy %d lần", n)
	}
}

// 2xx: THE ROUTE PICKS THE CATALOGUE, THE HOST PICKS THE COMMUNE, AND THE TRAIL GETS THE STAFF CODE.
func TestCatalogueImports_2xxRightCatalogueCommuneAndStaffCode(t *testing.T) {
	for _, rt := range catalogueImportRoutes() {
		m := dungMayChu(t)
		lookupGrant(t, m)
		w := rt.call(t, m, hostA, canBoCuaXa(xaA))
		if w.Code != rt.ok {
			t.Fatalf("%s: mã = %d, muốn %d — %s", rt.name, w.Code, rt.ok, w.Body.String())
		}
		if rt.method == "GET" {
			if catalogueImportCalls(m) != 0 {
				t.Errorf("%s: tải mẫu không cần use case", rt.name)
			}
			continue
		}
		f := rt.fake(m)
		if f.calls != 1 || catalogueImportCalls(m) != 1 {
			t.Errorf("%s: phải gọi ĐÚNG use case của danh mục trên đường dẫn (%d / %d)", rt.name, f.calls, catalogueImportCalls(m))
		}
		if f.lastXa != xaA {
			t.Errorf("%s: xã = %q", rt.name, f.lastXa)
		}
		if rt.ok == http.StatusCreated {
			if f.lastActor.ID != maCanBo || f.lastActor.IP == "" {
				t.Errorf("%s: vết phải mang MÃ CÁN BỘ, không phải id nội bộ: %+v", rt.name, f.lastActor)
			}
			var out catalogueImportCreatedOut
			if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || len(out.Created) != 1 || out.Created[0].ID == "" {
				t.Errorf("%s: thân 201 = %s", rt.name, w.Body.String())
			}
		}
		if len(f.lastRows) != 2 || f.lastRows[1].Row != 3 {
			t.Errorf("%s: dòng tới use case: %+v", rt.name, f.lastRows)
		}
	}
}

// THE TEMPLATE ROUND TRIP: download, fill in Excel's place, upload — the rows arrive as typed, and the
// priority template has NO `Thứ tự` column.
func TestCatalogueImports_TemplateRoundTrip(t *testing.T) {
	for _, c := range []struct {
		base   string
		sheet  string
		fake   func(m *mayChu) *catalogueImportFake
		header []string
		fill   [][]string
	}{
		{"/api/v1/task-types", taskTypeImportSpec.sheet, typesFake, domain.CatalogueOrderFromColumn.Columns(),
			[][]string{{"Theo văn bản", "", "3"}, {"Cơ bản", "co-ban", ""}}},
		{"/api/v1/task-priorities", taskPriorityImportSpec.sheet, prioritiesFake, domain.CatalogueOrderFromPosition.Columns(),
			[][]string{{"Khẩn", ""}, {"Thường", "thuong"}}},
	} {
		m := dungMayChu(t)
		lookupGrant(t, m)
		w := sendCatalogue(t, m, "GET", hostA, c.base+"/import-template", nil, canBoCuaXa(xaA), "")
		doiMa(t, w, http.StatusOK)
		if w.Header().Get("Content-Type") != catalogueXLSXMIME || w.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%s: header %v", c.base, w.Header())
		}
		cells, err := xlsx.ReadSheet(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()), xlsx.DefaultLimits)
		if err != nil || len(cells) != 1 || strings.Join(cells[0], "|") != strings.Join(c.header, "|") {
			t.Fatalf("%s: trang dữ liệu chỉ có tiêu đề %q, nhận %q (%v)", c.base, c.header, cells, err)
		}

		f, err := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		if f.GetSheetList()[0] != c.sheet {
			t.Errorf("%s: trang đầu = %q", c.base, f.GetSheetList()[0])
		}
		for r, row := range c.fill {
			for col, v := range row {
				cell, _ := excelize.CoordinatesToCellName(col+1, r+2)
				if err := f.SetCellStr(c.sheet, cell, v); err != nil {
					t.Fatal(err)
				}
			}
		}
		var buf bytes.Buffer
		if err := f.Write(&buf); err != nil {
			t.Fatal(err)
		}
		f.Close()

		w = sendCatalogue(t, m, "POST", hostA, c.base+"/import-previews", buf.Bytes(), canBoCuaXa(xaA), "")
		doiMa(t, w, http.StatusOK)
		got := c.fake(m).lastRows
		if len(got) != 2 || got[0].Label != c.fill[0][0] || got[1].Code != c.fill[1][1] || got[0].Row != 2 {
			t.Errorf("%s: dòng tới use case %+v", c.base, got)
		}
		if len(c.header) == 3 && got[0].Order != "3" {
			t.Errorf("%s: cột Thứ tự không tới use case: %+v", c.base, got[0])
		}
	}
}

func TestCatalogueImports_StatusMapping(t *testing.T) {
	m := dungMayChu(t)
	lookupGrant(t, m)
	path := "/api/v1/task-priorities/imports"
	fake := prioritiesFake(m)

	fake.err = &app.CatalogueImportRejected{Errors: []domain.CatalogueImportError{
		{Row: 3, Column: domain.CatalogueImportColCode, Message: "Mã trùng"},
	}}
	w := sendCatalogue(t, m, "POST", hostA, path, prioritiesWorkbook(t), canBoCuaXa(xaA), catalogueImportKey)
	doiMa(t, w, http.StatusBadRequest)
	var out catalogueImportRejectedOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || out.Code != "import_invalid" || len(out.Errors) != 1 || out.Errors[0].Row != 3 {
		t.Errorf("phản hồi: %+v (%v)", out, err)
	}
	for _, c := range []struct {
		err  error
		code int
		id   string
	}{
		{docstore.ErrMaDaTonTai, http.StatusConflict, "catalogue_changed"},
		{errors.New("cơ sở dữ liệu không phản hồi"), http.StatusInternalServerError, "internal"},
	} {
		fake.err = c.err
		w := sendCatalogue(t, m, "POST", hostA, path, prioritiesWorkbook(t), canBoCuaXa(xaA), catalogueImportKey)
		if got := loiTra(t, w).Code; w.Code != c.code || got != c.id || strings.Contains(w.Body.String(), "không phản hồi") {
			t.Errorf("%v: %d (%s) %s", c.err, w.Code, got, w.Body.String())
		}
	}

	fake.err = nil
	fake.res = app.CatalogueImportResult{Errors: []domain.CatalogueImportError{{Row: 2, Message: "x"}}}
	w = sendCatalogue(t, m, "POST", hostA, "/api/v1/task-priorities/import-previews", prioritiesWorkbook(t), canBoCuaXa(xaA), "")
	doiMa(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"valid":false`) || !strings.Contains(w.Body.String(), `"entries":[]`) {
		t.Errorf("xem trước tệp lỗi: %s", w.Body.String())
	}
}

// A WRONG HEADER is a content error answered without the use case, and NEVER ECHOES A CELL — here the
// priority template refusing a file that still carries a `Thứ tự` column.
func TestCatalogueImports_WrongHeaderNeverEchoesCells(t *testing.T) {
	const secret = "Nhóm bí mật 0900000000"
	m := dungMayChu(t)
	lookupGrant(t, m)
	for path, code := range map[string]int{
		"/api/v1/task-priorities/import-previews": http.StatusOK,
		"/api/v1/task-priorities/imports":         http.StatusBadRequest,
		"/api/v1/task-types/import-previews":      http.StatusOK,
	} {
		data := catalogueWorkbook(t, domain.CatalogueOrderFromColumn.Columns(), []string{secret, "", "1"})
		if strings.HasPrefix(path, "/api/v1/task-types") {
			data = catalogueWorkbook(t, []string{secret, "Tên hiển thị"}, []string{secret})
		}
		w := sendCatalogue(t, m, "POST", hostA, path, data, canBoCuaXa(xaA), catalogueImportKey)
		if w.Code != code || !strings.Contains(w.Body.String(), `"row":1`) || strings.Contains(w.Body.String(), "0900000000") {
			t.Errorf("%s: %d %s", path, w.Code, w.Body.String())
		}
	}
	if catalogueImportCalls(m) != 0 {
		t.Error("tiêu đề sai mà use case vẫn chạy")
	}
}

// Not a workbook: a fixed sentence, 415, no use case, nothing of the body echoed.
func TestCatalogueImports_NotAWorkbookIsRefusedWithAFixedSentence(t *testing.T) {
	m := dungMayChu(t)
	lookupGrant(t, m)
	w := sendCatalogue(t, m, "POST", hostA, "/api/v1/task-types/import-previews",
		[]byte("Nguyễn Văn A,0900000000"), canBoCuaXa(xaA), "")
	if w.Code != http.StatusUnsupportedMediaType || strings.Contains(w.Body.String(), "0900000000") {
		t.Errorf("%d %s", w.Code, w.Body.String())
	}
	if catalogueImportCalls(m) != 0 {
		t.Error("tệp hỏng mà use case vẫn chạy")
	}
}

// THE IMPORT REQUIRES AN Idempotency-Key; the preview does not.
func TestCatalogueImports_ImportNeedsIdempotencyKey(t *testing.T) {
	m := dungMayChu(t)
	lookupGrant(t, m)
	if w := sendCatalogue(t, m, "POST", hostA, "/api/v1/task-types/imports", typesWorkbook(t), canBoCuaXa(xaA), ""); w.Code/100 != 4 {
		t.Errorf("nhập không khoá chống trùng: %d", w.Code)
	}
	if w := sendCatalogue(t, m, "POST", hostA, "/api/v1/task-types/import-previews", typesWorkbook(t), canBoCuaXa(xaA), ""); w.Code != http.StatusOK {
		t.Errorf("xem trước phải không cần khoá: %d", w.Code)
	}
	if typesFake(m).calls != 1 {
		t.Errorf("use case chạy %d lần, muốn 1 (chỉ xem trước)", typesFake(m).calls)
	}
}
