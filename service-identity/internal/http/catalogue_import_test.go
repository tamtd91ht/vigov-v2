package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
	"github.com/xuri/excelize/v2"
)

// THE SIX IMPORT ROUTES OF THE TWO CATALOGUES, all `admin.lookup` (ADR 0059 §3). What is asserted here
// is what the HANDLER owns: the permission, the upload, the wire mapping, and which identifier reaches
// the trail. The rules are proved in domain/catalogue_import_test.go, the transaction in
// app/catalogue_import_test.go.

// catalogueImportsFake stands in for *app.CatalogueImporter[T].
type catalogueImportsFake struct {
	calls     int
	lastRows  []domain.CatalogueImportRow
	lastActor app.NguoiThucHien
	lastXa    tenant.ID

	res app.CatalogueImportResult
	err error
}

func catalogueImportsSample() *catalogueImportsFake {
	return &catalogueImportsFake{res: app.CatalogueImportResult{Entries: []domain.PlannedCatalogueEntry{
		{Row: 2, ID: "muc-1", Code: "khu-pho", Label: "Khu phố", Order: 3},
	}}}
}

func (f *catalogueImportsFake) Preview(ctx context.Context, rows []domain.CatalogueImportRow) (app.CatalogueImportResult, error) {
	f.calls++
	f.lastRows, f.lastXa = rows, tenant.MustFrom(ctx)
	return f.res, f.err
}

func (f *catalogueImportsFake) Import(ctx context.Context, rows []domain.CatalogueImportRow, actor app.NguoiThucHien) (app.CatalogueImportResult, error) {
	f.calls++
	f.lastRows, f.lastActor, f.lastXa = rows, actor, tenant.MustFrom(ctx)
	return f.res, f.err
}

func sampleCatalogueWorkbook(t *testing.T) []byte {
	return residentialUnitWorkbook(t, domain.CatalogueImportColumns(),
		[]any{"Khu phố", "", 3},
		[]any{"Ấp", "ap", ""},
	)
}

type catalogueImportRoute struct {
	name, method, path string
	ok                 int
	fake               func(m *mayChu) *catalogueImportsFake
}

func catalogueImportRoutes() []catalogueImportRoute {
	var out []catalogueImportRoute
	for _, c := range []struct {
		base string
		fake func(m *mayChu) *catalogueImportsFake
	}{
		{duongLoaiDonViDanCu, func(m *mayChu) *catalogueImportsFake { return m.residentialUnitTypeImports }},
		{duongKhoiNhiemVu, func(m *mayChu) *catalogueImportsFake { return m.taskBlocImports }},
	} {
		out = append(out,
			catalogueImportRoute{c.base + " mẫu", "GET", c.base + "/import-template", http.StatusOK, c.fake},
			catalogueImportRoute{c.base + " xem trước", "POST", c.base + "/import-previews", http.StatusOK, c.fake},
			catalogueImportRoute{c.base + " nhập", "POST", c.base + "/imports", http.StatusCreated, c.fake},
		)
	}
	return out
}

func (m *mayChu) callCatalogueImportRoute(t *testing.T, rt catalogueImportRoute, host, tok string) *httptest.ResponseRecorder {
	t.Helper()
	if rt.method == "GET" {
		return m.goi(t, "GET", host, rt.path, "", tok)
	}
	return m.sendWorkbook(t, rt.path, host, tok, sampleCatalogueWorkbook(t))
}

func (m *mayChu) catalogueImportCalls() int {
	return m.residentialUnitTypeImports.calls + m.taskBlocImports.calls
}

// --- rule 5, invariant 7: the four cases, for each of the six routes ------------------------------

func TestCatalogueImports_401WithoutToken(t *testing.T) {
	m := dungMayChuDanhMuc(t)
	for _, rt := range catalogueImportRoutes() {
		if w := m.callCatalogueImportRoute(t, rt, hostA, ""); w.Code != http.StatusUnauthorized {
			t.Errorf("%s: mã = %d, muốn 401", rt.name, w.Code)
		}
	}
	if n := m.catalogueImportCalls(); n != 0 {
		t.Errorf("chưa đăng nhập mà use case đã chạy %d lần", n)
	}
}

// 403 WITH `admin.user` — the harness default, a real key that is not `admin.lookup`.
func TestCatalogueImports_403WrongPermission(t *testing.T) {
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)
	for _, rt := range catalogueImportRoutes() {
		if w := m.callCatalogueImportRoute(t, rt, hostA, tok); w.Code != http.StatusForbidden {
			t.Errorf("%s: mã = %d, muốn 403", rt.name, w.Code)
		}
	}
	if n := m.catalogueImportCalls(); n != 0 {
		t.Errorf("sai quyền mà use case đã chạy %d lần", n)
	}
}

// 403 WITH `admin.lookup` HELD IN COMMUNE A, signed in properly at commune B (rule 5, invariant 3).
func TestCatalogueImports_403RightPermissionWrongCommune(t *testing.T) {
	m := dungMayChuDanhMuc(t)
	tok := m.tokenCho(t, xaB, sidB)
	for _, rt := range catalogueImportRoutes() {
		if w := m.callCatalogueImportRoute(t, rt, hostB, tok); w.Code != http.StatusForbidden {
			t.Errorf("%s: mã = %d, muốn 403", rt.name, w.Code)
		}
	}
	if n := m.catalogueImportCalls(); n != 0 {
		t.Errorf("sai xã mà use case đã chạy %d lần", n)
	}
}

// 2xx, THE ROUTE PICKS THE CATALOGUE, THE HOST PICKS THE COMMUNE, AND THE TRAIL GETS THE STAFF CODE.
func TestCatalogueImports_2xxRightCatalogueCommuneAndStaffCode(t *testing.T) {
	for _, rt := range catalogueImportRoutes() {
		m := dungMayChuDanhMuc(t)
		tok := m.tokenCho(t, xaA, sidA)
		w := m.callCatalogueImportRoute(t, rt, hostA, tok)
		if w.Code != rt.ok {
			t.Fatalf("%s: mã = %d, muốn %d — %s", rt.name, w.Code, rt.ok, w.Body.String())
		}
		if rt.method == "GET" {
			if m.catalogueImportCalls() != 0 {
				t.Errorf("%s: tải mẫu không cần use case", rt.name)
			}
			continue
		}
		f := rt.fake(m)
		if f.calls != 1 || m.catalogueImportCalls() != 1 {
			t.Errorf("%s: phải gọi ĐÚNG use case của danh mục trên đường dẫn (%d / %d)", rt.name, f.calls, m.catalogueImportCalls())
		}
		if f.lastXa != xaA {
			t.Errorf("%s: xã = %q", rt.name, f.lastXa)
		}
		if rt.ok == http.StatusCreated && (f.lastActor.Vet.ID != maCanBo || f.lastActor.ID != idNoiBo) {
			t.Errorf("%s: vết phải mang MÃ CÁN BỘ: %+v", rt.name, f.lastActor)
		}
		if len(f.lastRows) != 2 || f.lastRows[0].Label != "Khu phố" || f.lastRows[0].Order != "3" || f.lastRows[1].Code != "ap" {
			t.Errorf("%s: dòng tới use case: %+v", rt.name, f.lastRows)
		}
	}
}

func TestCatalogueImports_TemplateHasOnlyTheHeader(t *testing.T) {
	m := dungMayChuDanhMuc(t)
	tok := m.tokenCho(t, xaA, sidA)
	for path, sheet := range map[string]string{
		duongLoaiDonViDanCu + "/import-template": residentialUnitTypeImportSpec.sheet,
		duongKhoiNhiemVu + "/import-template":    taskBlocImportSpec.sheet,
	} {
		w := m.goi(t, "GET", hostA, path, "", tok)
		doiMa(t, w, http.StatusOK)
		if w.Header().Get("Content-Type") != mimeXLSX || w.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%s: header %v", path, w.Header())
		}
		f, err := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if f.GetSheetList()[0] != sheet {
			t.Errorf("%s: trang đầu = %q", path, f.GetSheetList()[0])
		}
		rows, _ := f.GetRows(sheet)
		if len(rows) != 1 || strings.Join(rows[0], "|") != strings.Join(domain.CatalogueImportColumns(), "|") {
			t.Errorf("%s: trang dữ liệu chỉ có tiêu đề, nhận %q", path, rows)
		}
		f.Close()
	}
}

func TestCatalogueImports_StatusMapping(t *testing.T) {
	m := dungMayChuDanhMuc(t)
	tok := m.tokenCho(t, xaA, sidA)
	path := duongKhoiNhiemVu + "/imports"

	m.taskBlocImports.err = &app.CatalogueImportRejected{Errors: []domain.CatalogueImportError{
		{Row: 3, Column: domain.CatalogueImportColCode, Message: "Mã trùng"},
	}}
	w := m.sendWorkbook(t, path, hostA, tok, sampleCatalogueWorkbook(t))
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
		{idstore.ErrMaDaTonTai, http.StatusConflict, "catalogue_changed"},
		{errors.New("cơ sở dữ liệu không phản hồi"), http.StatusInternalServerError, "internal"},
	} {
		m.taskBlocImports.err = c.err
		w := m.sendWorkbook(t, path, hostA, tok, sampleCatalogueWorkbook(t))
		if got := loiTra(t, w).Code; w.Code != c.code || got != c.id || strings.Contains(w.Body.String(), "không phản hồi") {
			t.Errorf("%v: %d (%s) %s", c.err, w.Code, got, w.Body.String())
		}
	}

	m.taskBlocImports.err = nil
	m.taskBlocImports.res = app.CatalogueImportResult{Errors: []domain.CatalogueImportError{{Row: 2, Message: "x"}}}
	w = m.sendWorkbook(t, duongKhoiNhiemVu+"/import-previews", hostA, tok, sampleCatalogueWorkbook(t))
	doiMa(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"valid":false`) || !strings.Contains(w.Body.String(), `"entries":[]`) {
		t.Errorf("xem trước tệp lỗi: %s", w.Body.String())
	}
}

// A WRONG HEADER is a content error answered without the use case, and never echoes a cell.
func TestCatalogueImports_WrongHeaderNeverEchoesCells(t *testing.T) {
	const secret = "Nhóm bí mật 0900000000"
	m := dungMayChuDanhMuc(t)
	tok := m.tokenCho(t, xaA, sidA)
	data := residentialUnitWorkbook(t, []string{secret, "Tên hiển thị"}, []any{secret})
	for path, code := range map[string]int{
		duongLoaiDonViDanCu + "/import-previews": http.StatusOK,
		duongLoaiDonViDanCu + "/imports":         http.StatusBadRequest,
	} {
		w := m.sendWorkbook(t, path, hostA, tok, data)
		if w.Code != code || !strings.Contains(w.Body.String(), `"row":1`) || strings.Contains(w.Body.String(), "0900000000") {
			t.Errorf("%s: %d %s", path, w.Code, w.Body.String())
		}
	}
	if m.catalogueImportCalls() != 0 {
		t.Error("tiêu đề sai mà use case vẫn chạy")
	}
}

// THE IMPORT REQUIRES AN Idempotency-Key; the preview does not.
func TestCatalogueImports_ImportNeedsIdempotencyKey(t *testing.T) {
	m := dungMayChuDanhMuc(t)
	tok := m.tokenCho(t, xaA, sidA)
	for _, c := range []struct {
		path string
		ok   bool
	}{{duongLoaiDonViDanCu + "/imports", false}, {duongLoaiDonViDanCu + "/import-previews", true}} {
		body, ct := multipartBody(t, sampleCatalogueWorkbook(t), mimeXLSX, "danh-muc.xlsx")
		r := httptest.NewRequest("POST", "https://"+hostA+c.path, body)
		r.Host = hostA
		r.RemoteAddr = "10.0.0.7:51000"
		r.Header.Set("Content-Type", ct)
		r.AddCookie(&http.Cookie{Name: CookiePhien, Value: tok})
		w := m.chay(r)
		if c.ok != (w.Code == http.StatusOK) || (!c.ok && w.Code/100 != 4) {
			t.Errorf("%s: mã = %d", c.path, w.Code)
		}
	}
	if m.residentialUnitTypeImports.calls != 1 {
		t.Errorf("use case chạy %d lần, muốn 1 (chỉ xem trước)", m.residentialUnitTypeImports.calls)
	}
}
