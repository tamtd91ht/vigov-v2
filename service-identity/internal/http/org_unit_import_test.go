package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	"github.com/vihat/vigov/service-identity/internal/orgunitxlsx"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
	"github.com/xuri/excelize/v2"
)

// THE THREE IMPORT ROUTES OF THE ORG CHART, all `admin.org`. What is asserted here is what the
// HANDLER owns: the permission, the upload limits, the status mapping and which identifier reaches
// the trail. The rules are proved in domain/org_unit_import_test.go, the transaction in
// app/org_unit_import_test.go, the workbook in orgunitxlsx.

const (
	pathImportTemplate = "/api/v1/org-units/import-template"
	pathImportPreview  = "/api/v1/org-units/import-previews"
	pathImport         = "/api/v1/org-units/imports"
)

// orgUnitImportsFake stands in for *app.OrgUnitImporter.
type orgUnitImportsFake struct {
	calls     int
	lastRows  []domain.OrgUnitImportRow
	lastActor app.NguoiThucHien
	lastXa    tenant.ID

	res app.OrgUnitImportResult
	err error
}

func orgUnitImportsSample() *orgUnitImportsFake {
	return &orgUnitImportsFake{res: app.OrgUnitImportResult{Units: []domain.PlannedOrgUnit{
		{Row: 2, Unit: domain.BoPhan{ID: "bp-new", Ma: "van-phong-dang-uy", Ten: "VĂN PHÒNG ĐẢNG ỦY", ThuTu: 1}},
	}}}
}

func (f *orgUnitImportsFake) Preview(ctx context.Context, rows []domain.OrgUnitImportRow) (app.OrgUnitImportResult, error) {
	f.calls++
	f.lastRows, f.lastXa = rows, tenant.MustFrom(ctx)
	return f.res, f.err
}

func (f *orgUnitImportsFake) Import(ctx context.Context, rows []domain.OrgUnitImportRow, actor app.NguoiThucHien) (app.OrgUnitImportResult, error) {
	f.calls++
	f.lastRows, f.lastActor, f.lastXa = rows, actor, tenant.MustFrom(ctx)
	return f.res, f.err
}

// sampleWorkbook is a valid two-row import file.
func sampleWorkbook(t *testing.T) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	var h []any
	for _, c := range domain.OrgUnitImportColumns() {
		h = append(h, c)
	}
	r2 := []any{"VĂN PHÒNG ĐẢNG ỦY", "", "", 1}
	r3 := []any{"TỔ TỔNG HỢP", "VĂN PHÒNG ĐẢNG ỦY", "", 2}
	for cell, row := range map[string]*[]any{"A1": &h, "A2": &r2, "A3": &r3} {
		if err := f.SetSheetRow("Sheet1", cell, row); err != nil {
			t.Fatalf("SetSheetRow: %v", err)
		}
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatalf("WriteToBuffer: %v", err)
	}
	return buf.Bytes()
}

// multipartBody wraps data as the `file` part with the given part type and file name.
func multipartBody(t *testing.T, data []byte, partType, fileName string) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	hdr := textproto.MIMEHeader{}
	hdr.Set("Content-Disposition", `form-data; name="file"; filename="`+fileName+`"`)
	if partType != "" {
		hdr.Set("Content-Type", partType)
	}
	w, err := mw.CreatePart(hdr)
	if err != nil {
		t.Fatalf("CreatePart: %v", err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatalf("ghi part: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("đóng multipart: %v", err)
	}
	return &body, mw.FormDataContentType()
}

var importIdemCounter int

// sendUpload posts an upload with a FRESH Idempotency-Key.
func (m *mayChu) sendUpload(t *testing.T, path, host, tok string, body *bytes.Buffer, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	importIdemCounter++
	r := httptest.NewRequest("POST", "https://"+host+path, body)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	r.Header.Set("Content-Type", contentType)
	r.Header.Set(idem.Header, "01JIDEMKEYIMPORT"+string(rune('A'+importIdemCounter%26))+string(rune('A'+(importIdemCounter/26)%26)))
	if tok != "" {
		r.AddCookie(&http.Cookie{Name: CookiePhien, Value: tok})
	}
	return m.chay(r)
}

func (m *mayChu) sendWorkbook(t *testing.T, path, host, tok string, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	body, ct := multipartBody(t, data, mimeXLSX, "so-do.xlsx")
	return m.sendUpload(t, path, host, tok, body, ct)
}

type importRoute struct {
	name, path string
	ok         int
}

func importRoutes() []importRoute {
	return []importRoute{
		{"tải mẫu", pathImportTemplate, http.StatusOK},
		{"xem trước", pathImportPreview, http.StatusOK},
		{"nhập", pathImport, http.StatusCreated},
	}
}

func (m *mayChu) callImportRoute(t *testing.T, rt importRoute, host, tok string) *httptest.ResponseRecorder {
	t.Helper()
	if rt.path == pathImportTemplate {
		return m.goi(t, "GET", host, rt.path, "", tok)
	}
	return m.sendWorkbook(t, rt.path, host, tok, sampleWorkbook(t))
}

// --- rule 5, invariant 7: the four cases, for each of the three routes ----------------------------

func TestOrgUnitImport_401WithoutToken(t *testing.T) {
	m := dungMayChuSoDo(t)
	for _, rt := range importRoutes() {
		if w := m.callImportRoute(t, rt, hostA, ""); w.Code != http.StatusUnauthorized {
			t.Errorf("%s: mã = %d, muốn 401 — %s", rt.name, w.Code, w.Body.String())
		}
	}
	if m.orgUnitImports.calls != 0 {
		t.Errorf("chưa đăng nhập mà use case đã chạy %d lần", m.orgUnitImports.calls)
	}
}

// 403 WITH `admin.user` — a real key, not `admin.org`.
func TestOrgUnitImport_403WrongPermission(t *testing.T) {
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)
	for _, rt := range importRoutes() {
		if w := m.callImportRoute(t, rt, hostA, tok); w.Code != http.StatusForbidden {
			t.Errorf("%s: mã = %d, muốn 403 — %s", rt.name, w.Code, w.Body.String())
		}
	}
	if m.orgUnitImports.calls != 0 || m.boPhan.goi != 0 {
		t.Errorf("sai quyền mà vẫn chạy: import %d, đọc danh mục %d", m.orgUnitImports.calls, m.boPhan.goi)
	}
}

// 403 WITH `admin.org` HELD IN COMMUNE A, signed in properly at commune B (rule 5, invariant 3).
func TestOrgUnitImport_403RightPermissionWrongCommune(t *testing.T) {
	m := dungMayChuSoDo(t)
	tok := m.tokenCho(t, xaB, sidB)
	for _, rt := range importRoutes() {
		if w := m.callImportRoute(t, rt, hostB, tok); w.Code != http.StatusForbidden {
			t.Errorf("%s: mã = %d, muốn 403 — %s", rt.name, w.Code, w.Body.String())
		}
	}
	if m.orgUnitImports.calls != 0 || m.boPhan.goi != 0 {
		t.Errorf("sai xã mà vẫn chạy: import %d, đọc danh mục %d", m.orgUnitImports.calls, m.boPhan.goi)
	}
}

// 2xx, THE COMMUNE OF THE HOST REACHES THE USE CASE, AND THE TRAIL GETS THE STAFF CODE.
func TestOrgUnitImport_2xxCommuneAndStaffCode(t *testing.T) {
	m := dungMayChuSoDo(t)
	tok := m.tokenCho(t, xaA, sidA)
	for _, rt := range importRoutes() {
		w := m.callImportRoute(t, rt, hostA, tok)
		if w.Code != rt.ok {
			t.Fatalf("%s: mã = %d, muốn %d — %s", rt.name, w.Code, rt.ok, w.Body.String())
		}
	}
	f := m.orgUnitImports
	if f.lastXa != xaA {
		t.Errorf("use case chạy ở xã %q, muốn %q", f.lastXa, xaA)
	}
	if f.lastActor.Vet.ID != maCanBo || f.lastActor.ID != idNoiBo {
		t.Errorf("Vet.ID=%q ID=%q — vết phải mang MÃ CÁN BỘ %q", f.lastActor.Vet.ID, f.lastActor.ID, maCanBo)
	}
	if len(f.lastRows) != 2 || f.lastRows[1].Row != 3 || f.lastRows[1].Parent != "VĂN PHÒNG ĐẢNG ỦY" || f.lastRows[0].Order != "1" {
		t.Errorf("dòng tới use case: %+v", f.lastRows)
	}
}

// --- the template ----------------------------------------------------------------------------

// THE TEMPLATE IS AN .xlsx OF THIS COMMUNE'S UNITS — never another commune's.
func TestOrgUnitImportTemplate_IsThisCommunesWorkbook(t *testing.T) {
	m := dungMayChuSoDo(t)
	w := m.goi(t, "GET", hostA, pathImportTemplate, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)
	if ct := w.Header().Get("Content-Type"); ct != mimeXLSX {
		t.Errorf("Content-Type = %q", ct)
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, "attachment;") {
		t.Errorf("Content-Disposition = %q", cd)
	}
	f, err := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatalf("tệp mẫu không mở được: %v", err)
	}
	defer f.Close()
	list, _ := f.GetRows(orgunitxlsx.SheetUnits)
	joined := ""
	for _, r := range list {
		joined += strings.Join(r, "|") + "\n"
	}
	if !strings.Contains(joined, "VĂN PHÒNG ĐẢNG ỦY") || strings.Contains(joined, "XÃ B") {
		t.Errorf("danh sách trong mẫu phải là bộ phận của xã A, không của xã B: %q", joined)
	}
}

// --- the upload limits -------------------------------------------------------------------------

func TestOrgUnitImport_RefusesWrongTypesAndOversize(t *testing.T) {
	good := sampleWorkbook(t)
	cases := []struct {
		name     string
		body     func() (*bytes.Buffer, string)
		wantCode int
		wantErr  string
	}{
		{"không phải multipart", func() (*bytes.Buffer, string) {
			return bytes.NewBufferString(`{"file":"x"}`), "application/json"
		}, http.StatusUnsupportedMediaType, "unsupported_media_type"},
		{"part khai .xlsm", func() (*bytes.Buffer, string) {
			return multipartBody(t, good, "application/vnd.ms-excel.sheet.macroEnabled.12", "so-do.xlsm")
		}, http.StatusUnsupportedMediaType, "unsupported_file_type"},
		{"đuôi .csv", func() (*bytes.Buffer, string) {
			return multipartBody(t, []byte("a,b\n"), "text/csv", "so-do.csv")
		}, http.StatusUnsupportedMediaType, "unsupported_file_type"},
		{"nội dung không phải xlsx dù khai đúng", func() (*bytes.Buffer, string) {
			return multipartBody(t, []byte("Tên bộ phận\nA\n"), mimeXLSX, "so-do.xlsx")
		}, http.StatusUnsupportedMediaType, "unsupported_file_type"},
		{"octet-stream nhưng đúng là xlsx thì nhận", func() (*bytes.Buffer, string) {
			return multipartBody(t, good, "application/octet-stream", "so-do.xlsx")
		}, 0, ""},
		{"tệp quá 2 MB", func() (*bytes.Buffer, string) {
			return multipartBody(t, bytes.Repeat([]byte("x"), orgunitxlsx.MaxFileBytes+1), mimeXLSX, "so-do.xlsx")
		}, http.StatusRequestEntityTooLarge, "file_too_large"},
		{"thân quá cỡ", func() (*bytes.Buffer, string) {
			return multipartBody(t, bytes.Repeat([]byte("x"), importBodyMax+1), mimeXLSX, "so-do.xlsx")
		}, http.StatusRequestEntityTooLarge, "file_too_large"},
		{"thiếu trường file", func() (*bytes.Buffer, string) {
			var b bytes.Buffer
			mw := multipart.NewWriter(&b)
			_ = mw.WriteField("other", "x")
			_ = mw.Close()
			return &b, mw.FormDataContentType()
		}, http.StatusBadRequest, "missing_file"},
	}
	for _, path := range []string{pathImportPreview, pathImport} {
		for _, c := range cases {
			m := dungMayChuSoDo(t)
			body, ct := c.body()
			w := m.sendUpload(t, path, hostA, m.tokenCho(t, xaA, sidA), body, ct)
			if c.wantCode == 0 {
				if w.Code/100 != 2 {
					t.Errorf("%s %s: mã = %d, muốn 2xx — %s", path, c.name, w.Code, w.Body.String())
				}
				continue
			}
			if got := loiTra(t, w).Code; w.Code != c.wantCode || got != c.wantErr {
				t.Errorf("%s %s: mã = %d (%s), muốn %d (%s)", path, c.name, w.Code, got, c.wantCode, c.wantErr)
			}
			if m.orgUnitImports.calls != 0 {
				t.Errorf("%s %s: tệp bị từ chối mà use case vẫn chạy", path, c.name)
			}
		}
	}
}

// --- status mapping ------------------------------------------------------------------------------

func TestOrgUnitImport_RejectedFileIs400WithEveryError(t *testing.T) {
	m := dungMayChuSoDo(t)
	m.orgUnitImports.err = &app.OrgUnitImportRejected{Errors: []domain.OrgUnitImportError{
		{Row: 3, Column: domain.OrgUnitImportColOrder, Message: "Thứ tự phải là số nguyên"},
		{Row: 4, Column: domain.OrgUnitImportColParent, Message: "Không tìm thấy bộ phận cha"},
	}}
	w := m.sendWorkbook(t, pathImport, hostA, m.tokenCho(t, xaA, sidA), sampleWorkbook(t))
	doiMa(t, w, http.StatusBadRequest)
	var out orgUnitImportRejectedOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("thân: %v", err)
	}
	if out.Code != "import_invalid" || len(out.Errors) != 2 || out.Errors[1].Row != 4 || out.Errors[1].Column != domain.OrgUnitImportColParent {
		t.Errorf("phản hồi: %+v", out)
	}
}

func TestOrgUnitImport_PreviewReportsErrorsAs200(t *testing.T) {
	m := dungMayChuSoDo(t)
	m.orgUnitImports.res = app.OrgUnitImportResult{Errors: []domain.OrgUnitImportError{{Row: 2, Column: domain.OrgUnitImportColCode, Message: "Mã đã được cấp"}}}
	w := m.sendWorkbook(t, pathImportPreview, hostA, m.tokenCho(t, xaA, sidA), sampleWorkbook(t))
	doiMa(t, w, http.StatusOK)
	var out orgUnitImportPreviewOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("thân: %v", err)
	}
	if out.Valid || len(out.Errors) != 1 || out.Units == nil {
		t.Errorf("xem trước tệp lỗi: %+v (units phải là [] chứ không null)", out)
	}
}

// A WRONG HEADER never reaches the use case, and is reported like any other file error.
func TestOrgUnitImport_WrongHeaderNeverReachesUseCase(t *testing.T) {
	f := excelize.NewFile()
	h := []any{"Họ và tên", "Số điện thoại"}
	_ = f.SetSheetRow("Sheet1", "A1", &h)
	buf, _ := f.WriteToBuffer()
	f.Close()

	m := dungMayChuSoDo(t)
	tok := m.tokenCho(t, xaA, sidA)
	w := m.sendWorkbook(t, pathImport, hostA, tok, buf.Bytes())
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), `"row":1`) {
		t.Errorf("tiêu đề sai khi nhập: %d %s", w.Code, w.Body.String())
	}
	w = m.sendWorkbook(t, pathImportPreview, hostA, tok, buf.Bytes())
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"valid":false`) {
		t.Errorf("tiêu đề sai khi xem trước: %d %s", w.Code, w.Body.String())
	}
	if m.orgUnitImports.calls != 0 {
		t.Error("tiêu đề sai mà use case vẫn chạy")
	}
}

func TestOrgUnitImport_StatusMapping(t *testing.T) {
	for _, c := range []struct {
		err  error
		code int
		id   string
	}{
		{idstore.ErrMaBoPhanDaDung, http.StatusConflict, "org_chart_changed"},
		{idstore.ErrBoPhanChaKhongTonTai, http.StatusConflict, "org_chart_changed"},
		{errors.New("cơ sở dữ liệu không phản hồi"), http.StatusInternalServerError, "internal"},
	} {
		m := dungMayChuSoDo(t)
		m.orgUnitImports.err = c.err
		w := m.sendWorkbook(t, pathImport, hostA, m.tokenCho(t, xaA, sidA), sampleWorkbook(t))
		if got := loiTra(t, w).Code; w.Code != c.code || got != c.id {
			t.Errorf("%v: mã = %d (%s), muốn %d (%s)", c.err, w.Code, got, c.code, c.id)
		}
		if strings.Contains(w.Body.String(), "không phản hồi") {
			t.Errorf("lỗi nội bộ lọt ra ngoài: %s", w.Body.String())
		}
	}
}

// THE IMPORT REQUIRES AN Idempotency-Key; the preview, which writes nothing, does not.
func TestOrgUnitImport_IdempotencyKey(t *testing.T) {
	m := dungMayChuSoDo(t)
	tok := m.tokenCho(t, xaA, sidA)
	for _, c := range []struct {
		path string
		ok   bool
	}{{pathImport, false}, {pathImportPreview, true}} {
		body, ct := multipartBody(t, sampleWorkbook(t), mimeXLSX, "so-do.xlsx")
		r := httptest.NewRequest("POST", "https://"+hostA+c.path, body)
		r.Host = hostA
		r.RemoteAddr = "10.0.0.7:51000"
		r.Header.Set("Content-Type", ct)
		r.AddCookie(&http.Cookie{Name: CookiePhien, Value: tok})
		w := m.chay(r)
		if c.ok && w.Code != http.StatusOK {
			t.Errorf("%s không khoá: mã = %d, muốn 200", c.path, w.Code)
		}
		if !c.ok && w.Code/100 != 4 {
			t.Errorf("%s không khoá: mã = %d, muốn 4xx", c.path, w.Code)
		}
	}
	if m.orgUnitImports.calls != 1 {
		t.Errorf("use case chạy %d lần, muốn 1 (chỉ xem trước)", m.orgUnitImports.calls)
	}
}
