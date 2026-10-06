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
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/xlsx"
	"github.com/vihat/vigov/service-finance/internal/app"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// THE THREE IMPORT ROUTES OF THE CAPITAL-PLAN-CATEGORY CATALOGUE, all `admin.lookup` (ADR 0059 §3).
// What is asserted here is what the HANDLER owns: the permission, the upload, the wire mapping, and
// which identifier reaches the trail. The rules are proved in domain/catalogue_import_test.go, the
// transaction in app/catalogue_import_test.go.

// catalogueImportFake stands in for *app.CapitalPlanCategoryImporter, recording the commune.
type catalogueImportFake struct {
	calls     int
	lastRows  []domain.CatalogueImportRow
	lastActor audit.Actor
	lastXa    tenant.ID

	res app.CatalogueImportResult
	err error
}

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

const (
	catalogueImportBase = "/api/v1/capital-plan-categories"
	catalogueImportKey  = "01JCATALOGUEIMPORTKEY00000F"
)

type catalogueImportHarness struct {
	h       http.Handler
	fake    *catalogueImportFake
	checker *checkerDanhMucGia
}

// newCatalogueImportHarness mounts the REAL routes through Register behind the real edge chain, with
// idem.Middleware over an IN-MEMORY store: the import declares DongKhiHong, so a nil store would refuse
// every import before the handler ran and the 201 case would test nothing.
func newCatalogueImportHarness(t *testing.T) *catalogueImportHarness {
	t.Helper()
	fake := &catalogueImportFake{res: app.CatalogueImportResult{
		Batch:   "01JBATCH0000000000000000FF",
		Entries: []domain.PlannedCatalogueEntry{{Row: 2, ID: "hm-1", Code: "xay-dung-moi", Label: "Xây dựng mới", Order: 3}},
	}}
	checker := &checkerDanhMucGia{}
	im := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	Register(mux, Deps{
		Checker:                    checker,
		HangMuc:                    hangMucMau(),
		GhiHangMuc:                 &ghiDanhMucGia{},
		CapitalPlanCategoryImports: fake, DisbursementImports: &disbursementImportFake{},
		FundingSources:          &fundingSourcesFake{},
		FundingSourceWrites:     &fundingSourceWritesFake{},
		ProjectDiscussion:       newProjectDiscussionFake(),
		ProjectDiscussionWrites: &projectDiscussionWritesFake{},
		DuAn:                    duAnMau(),
		GhiDuAn:                 &ghiDuAnGia{},
		GhiChungTu:              &ghiChungTuGia{},
		Nguong:                  nguongMacDinh(),
		NganSach:                &nganSachGia{},
		GhiNganSach:             &ghiNganSachGia{},
		AuditLog:                &auditLogFake{},
		SystemMessages:          &systemMessagesFake{},
		Nay:                     func() time.Time { return lucDaQua7096 },
		Log:                     im,
	})
	var h http.Handler = mux
	h = chuTheGhi(h)
	h = idem.Middleware(moiKhoIdem(), im)(h)
	h = httpx.TenantMiddleware(thuMucMau())(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)
	return &catalogueImportHarness{h: h, fake: fake, checker: checker}
}

// grantLookup gives `admin.lookup` IN COMMUNE A ONLY.
func (m *catalogueImportHarness) grantLookup() {
	m.checker.co = map[tenant.ID]map[authz.Perm]struct{}{xaA: {"admin.lookup": {}}}
}

// send goes through the chain. file == nil sends a GET.
func (m *catalogueImportHarness) send(t *testing.T, method, host, path string, file []byte, p *authz.Principal, key string) *httptest.ResponseRecorder {
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
		r = r.WithContext(context.WithValue(r.Context(), khoaChuTheGhi{}, *p))
	}
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)
	return w
}

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

func sampleCategoryWorkbook(t *testing.T) []byte {
	return catalogueWorkbook(t, domain.CatalogueImportColumns(),
		[]string{"Xây dựng mới", "", "3"}, []string{"Sửa chữa", "sua-chua", ""})
}

type catalogueImportRoute struct {
	name, method, path string
	ok                 int
}

func catalogueImportRoutes() []catalogueImportRoute {
	return []catalogueImportRoute{
		{"mẫu", "GET", catalogueImportBase + "/import-template", http.StatusOK},
		{"xem trước", "POST", catalogueImportBase + "/import-previews", http.StatusOK},
		{"nhập", "POST", catalogueImportBase + "/imports", http.StatusCreated},
	}
}

func (m *catalogueImportHarness) call(t *testing.T, rt catalogueImportRoute, host string, p *authz.Principal) *httptest.ResponseRecorder {
	t.Helper()
	if rt.method == "GET" {
		return m.send(t, "GET", host, rt.path, nil, p, "")
	}
	return m.send(t, "POST", host, rt.path, sampleCategoryWorkbook(t), p, catalogueImportKey)
}

// --- rule 5, invariant 7: the four cases, for each of the three routes ----------------------------

func TestCatalogueImports_401WithoutPrincipal(t *testing.T) {
	m := newCatalogueImportHarness(t)
	m.grantLookup()
	for _, rt := range catalogueImportRoutes() {
		if w := m.call(t, rt, hostA, nil); w.Code != http.StatusUnauthorized {
			t.Errorf("%s: mã = %d, muốn 401", rt.name, w.Code)
		}
	}
	if m.fake.calls != 0 {
		t.Errorf("chưa đăng nhập mà use case đã chạy %d lần", m.fake.calls)
	}
}

// 403 with `budget.update` — a real key of this service that is not `admin.lookup`.
func TestCatalogueImports_403WrongPermission(t *testing.T) {
	m := newCatalogueImportHarness(t)
	m.checker.co = map[tenant.ID]map[authz.Perm]struct{}{xaA: {"budget.update": {}}}
	for _, rt := range catalogueImportRoutes() {
		if w := m.call(t, rt, hostA, canBoGhi(xaA)); w.Code != http.StatusForbidden {
			t.Errorf("%s: mã = %d, muốn 403", rt.name, w.Code)
		}
	}
	if m.fake.calls != 0 {
		t.Errorf("sai quyền mà use case đã chạy %d lần", m.fake.calls)
	}
}

// 403 WITH `admin.lookup` HELD IN COMMUNE A, signed in properly at commune B (rule 5, invariant 3).
func TestCatalogueImports_403RightPermissionWrongCommune(t *testing.T) {
	m := newCatalogueImportHarness(t)
	m.grantLookup()
	for _, rt := range catalogueImportRoutes() {
		if w := m.call(t, rt, hostB, canBoGhi(xaB)); w.Code != http.StatusForbidden {
			t.Errorf("%s: mã = %d, muốn 403", rt.name, w.Code)
		}
	}
	if m.fake.calls != 0 {
		t.Errorf("sai xã mà use case đã chạy %d lần", m.fake.calls)
	}
	if got := m.checker.hoiKhoaCuoi(); got != "admin.lookup" {
		t.Errorf("tuyến hỏi khoá %q, muốn \"admin.lookup\"", got)
	}
}

// 2xx: THE HOST PICKS THE COMMUNE AND THE TRAIL GETS THE STAFF CODE, never the internal id.
func TestCatalogueImports_2xxRightCommuneAndStaffCode(t *testing.T) {
	for _, rt := range catalogueImportRoutes() {
		m := newCatalogueImportHarness(t)
		m.grantLookup()
		w := m.call(t, rt, hostA, canBoGhi(xaA))
		if w.Code != rt.ok {
			t.Fatalf("%s: mã = %d, muốn %d — %s", rt.name, w.Code, rt.ok, w.Body.String())
		}
		if rt.method == "GET" {
			if m.fake.calls != 0 {
				t.Errorf("%s: tải mẫu không cần use case", rt.name)
			}
			continue
		}
		if m.fake.calls != 1 || m.fake.lastXa != xaA {
			t.Errorf("%s: gọi %d lần, xã %q", rt.name, m.fake.calls, m.fake.lastXa)
		}
		if rt.ok == http.StatusCreated {
			if m.fake.lastActor.ID != maCanBoGhi || m.fake.lastActor.IP == "" {
				t.Errorf("%s: vết phải mang MÃ CÁN BỘ: %+v", rt.name, m.fake.lastActor)
			}
			var out catalogueImportCreatedOut
			if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || len(out.Created) != 1 || out.Created[0].ID == "" {
				t.Errorf("%s: thân 201 = %s", rt.name, w.Body.String())
			}
		}
		if len(m.fake.lastRows) != 2 || m.fake.lastRows[0].Order != "3" || m.fake.lastRows[1].Code != "sua-chua" {
			t.Errorf("%s: dòng tới use case: %+v", rt.name, m.fake.lastRows)
		}
	}
}

// THE TEMPLATE ROUND TRIP: download, fill in Excel's place, upload — the rows arrive as typed.
func TestCatalogueImports_TemplateRoundTrip(t *testing.T) {
	m := newCatalogueImportHarness(t)
	m.grantLookup()
	w := m.send(t, "GET", hostA, catalogueImportBase+"/import-template", nil, canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	if w.Header().Get("Content-Type") != catalogueXLSXMIME || w.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("header %v", w.Header())
	}
	cells, err := xlsx.ReadSheet(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()), xlsx.DefaultLimits)
	if err != nil || len(cells) != 1 || strings.Join(cells[0], "|") != strings.Join(domain.CatalogueImportColumns(), "|") {
		t.Fatalf("trang dữ liệu chỉ có tiêu đề, nhận %q (%v)", cells, err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if f.GetSheetList()[0] != capitalPlanCategorySheet {
		t.Errorf("trang đầu = %q", f.GetSheetList()[0])
	}
	for r, row := range [][]string{{"Xây dựng mới", "", "3"}, {"Sửa chữa", "sua-chua", ""}} {
		for c, v := range row {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+2)
			if err := f.SetCellStr(capitalPlanCategorySheet, cell, v); err != nil {
				t.Fatal(err)
			}
		}
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	f.Close()
	w = m.send(t, "POST", hostA, catalogueImportBase+"/import-previews", buf.Bytes(), canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	got := m.fake.lastRows
	if len(got) != 2 || got[0].Label != "Xây dựng mới" || got[0].Order != "3" || got[1].Code != "sua-chua" || got[0].Row != 2 {
		t.Errorf("dòng tới use case %+v", got)
	}
}

func TestCatalogueImports_StatusMapping(t *testing.T) {
	m := newCatalogueImportHarness(t)
	m.grantLookup()
	path := catalogueImportBase + "/imports"

	m.fake.err = &app.CatalogueImportRejected{Errors: []domain.CatalogueImportError{
		{Row: 3, Column: domain.CatalogueImportColCode, Message: "Mã trùng"},
	}}
	w := m.send(t, "POST", hostA, path, sampleCategoryWorkbook(t), canBoGhi(xaA), catalogueImportKey)
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
		{fistore.ErrMaDaTonTai, http.StatusConflict, "catalogue_changed"},
		{errors.New("cơ sở dữ liệu không phản hồi"), http.StatusInternalServerError, "internal"},
	} {
		m.fake.err = c.err
		w := m.send(t, "POST", hostA, path, sampleCategoryWorkbook(t), canBoGhi(xaA), catalogueImportKey)
		var e httpx.Error
		_ = json.Unmarshal(w.Body.Bytes(), &e)
		if w.Code != c.code || e.Code != c.id || strings.Contains(w.Body.String(), "không phản hồi") {
			t.Errorf("%v: %d (%s) %s", c.err, w.Code, e.Code, w.Body.String())
		}
	}

	m.fake.err = nil
	m.fake.res = app.CatalogueImportResult{Errors: []domain.CatalogueImportError{{Row: 2, Message: "x"}}}
	w = m.send(t, "POST", hostA, catalogueImportBase+"/import-previews", sampleCategoryWorkbook(t), canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"valid":false`) || !strings.Contains(w.Body.String(), `"entries":[]`) {
		t.Errorf("xem trước tệp lỗi: %s", w.Body.String())
	}
}

// A WRONG HEADER is a content error answered without the use case, and NEVER ECHOES A CELL.
func TestCatalogueImports_WrongHeaderNeverEchoesCells(t *testing.T) {
	const secret = "Nhóm bí mật 0900000000"
	m := newCatalogueImportHarness(t)
	m.grantLookup()
	data := catalogueWorkbook(t, []string{secret, "Tên hiển thị"}, []string{secret})
	for path, code := range map[string]int{
		catalogueImportBase + "/import-previews": http.StatusOK,
		catalogueImportBase + "/imports":         http.StatusBadRequest,
	} {
		w := m.send(t, "POST", hostA, path, data, canBoGhi(xaA), catalogueImportKey)
		if w.Code != code || !strings.Contains(w.Body.String(), `"row":1`) || strings.Contains(w.Body.String(), "0900000000") {
			t.Errorf("%s: %d %s", path, w.Code, w.Body.String())
		}
	}
	if m.fake.calls != 0 {
		t.Error("tiêu đề sai mà use case vẫn chạy")
	}
}

// Not a workbook: a fixed sentence, 415, no use case, nothing of the body echoed.
func TestCatalogueImports_NotAWorkbookIsRefusedWithAFixedSentence(t *testing.T) {
	m := newCatalogueImportHarness(t)
	m.grantLookup()
	w := m.send(t, "POST", hostA, catalogueImportBase+"/import-previews", []byte("Nguyễn Văn A,0900000000"), canBoGhi(xaA), "")
	if w.Code != http.StatusUnsupportedMediaType || strings.Contains(w.Body.String(), "0900000000") {
		t.Errorf("%d %s", w.Code, w.Body.String())
	}
	if m.fake.calls != 0 {
		t.Error("tệp hỏng mà use case vẫn chạy")
	}
}

// THE IMPORT REQUIRES AN Idempotency-Key; the preview does not.
func TestCatalogueImports_ImportNeedsIdempotencyKey(t *testing.T) {
	m := newCatalogueImportHarness(t)
	m.grantLookup()
	if w := m.send(t, "POST", hostA, catalogueImportBase+"/imports", sampleCategoryWorkbook(t), canBoGhi(xaA), ""); w.Code/100 != 4 {
		t.Errorf("nhập không khoá chống trùng: %d", w.Code)
	}
	if w := m.send(t, "POST", hostA, catalogueImportBase+"/import-previews", sampleCategoryWorkbook(t), canBoGhi(xaA), ""); w.Code != http.StatusOK {
		t.Errorf("xem trước phải không cần khoá: %d", w.Code)
	}
	if m.fake.calls != 1 {
		t.Errorf("use case chạy %d lần, muốn 1 (chỉ xem trước)", m.fake.calls)
	}
}
