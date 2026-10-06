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
)

// THE THREE IMPORT ROUTES OF THE VOUCHER REGISTER. What is asserted here is what the HANDLER owns: the
// permission, the upload, the wire mapping, and which identifier reaches the trail. The rules are
// proved in domain/disbursement_import_test.go, the transaction in app/disbursement_import_test.go.

// disbursementImportFake stands in for *app.DisbursementImporter, recording the commune.
type disbursementImportFake struct {
	calls     int
	lastRows  []domain.VoucherImportRow
	lastActor audit.Actor
	lastTen   tenant.ID

	res app.VoucherImportResult
	err error
}

func (f *disbursementImportFake) Preview(ctx context.Context, rows []domain.VoucherImportRow) (app.VoucherImportResult, error) {
	f.calls++
	f.lastRows, f.lastTen = rows, tenant.MustFrom(ctx)
	return f.res, f.err
}

func (f *disbursementImportFake) Import(ctx context.Context, rows []domain.VoucherImportRow, actor audit.Actor) (app.VoucherImportResult, error) {
	f.calls++
	f.lastRows, f.lastActor, f.lastTen = rows, actor, tenant.MustFrom(ctx)
	return f.res, f.err
}

const (
	disbursementImportBase = "/api/v1/disbursements"
	disbursementImportKey  = "01JDISBURSEMENTIMPORTKEY00F"
)

type disbursementImportHarness struct {
	h       http.Handler
	fake    *disbursementImportFake
	checker *checkerDanhMucGia
}

func newDisbursementImportHarness(t *testing.T) *disbursementImportHarness {
	t.Helper()
	fake := &disbursementImportFake{res: app.VoucherImportResult{
		RowCount: 2, Total: 3_000_000, Batch: "01JBATCH0000000000000000FF",
		Created: []app.ImportedVoucher{{Row: 2, ID: "ct-1", ProjectCode: "DA01"}, {Row: 3, ID: "ct-2", ProjectCode: "DA02"}},
	}}
	checker := &checkerDanhMucGia{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	Register(mux, Deps{
		Checker:                    checker,
		HangMuc:                    hangMucMau(),
		GhiHangMuc:                 &ghiDanhMucGia{},
		CapitalPlanCategoryImports: &catalogueImportFake{},
		DisbursementImports:        fake,
		FundingSources:             &fundingSourcesFake{},
		FundingSourceWrites:        &fundingSourceWritesFake{},
		ProjectDiscussion:          newProjectDiscussionFake(),
		ProjectDiscussionWrites:    &projectDiscussionWritesFake{},
		DuAn:                       duAnMau(),
		GhiDuAn:                    &ghiDuAnGia{},
		GhiChungTu:                 &ghiChungTuGia{},
		Nguong:                     nguongMacDinh(),
		NganSach:                   &nganSachGia{},
		GhiNganSach:                &ghiNganSachGia{},
		AuditLog:                   &auditLogFake{},
		SystemMessages:             &systemMessagesFake{},
		Nay:                        func() time.Time { return lucDaQua7096 },
		Log:                        logger,
	})
	var h http.Handler = mux
	h = chuTheGhi(h)
	h = idem.Middleware(moiKhoIdem(), logger)(h)
	h = httpx.TenantMiddleware(thuMucMau())(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)
	return &disbursementImportHarness{h: h, fake: fake, checker: checker}
}

// grant gives `budget.read` and `budget.update` IN COMMUNE A ONLY.
func (m *disbursementImportHarness) grant() {
	m.checker.co = map[tenant.ID]map[authz.Perm]struct{}{xaA: {"budget.read": {}, "budget.update": {}}}
}

func (m *disbursementImportHarness) send(t *testing.T, method, host, path string, file []byte, p *authz.Principal, key string) *httptest.ResponseRecorder {
	t.Helper()
	var body io.Reader
	ct := ""
	if file != nil {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		hdr := textproto.MIMEHeader{}
		hdr.Set("Content-Disposition", `form-data; name="file"; filename="giai-ngan.xlsx"`)
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

func sampleVoucherWorkbook(t *testing.T) []byte {
	return catalogueWorkbook(t, domain.VoucherImportColumns(),
		[]string{"DA01", "15/03/2026", "1.000.000", "Đợt 1", "Công ty TNHH Mẫu", "CT-1", "Ngân sách tỉnh"},
		[]string{"DA02", "46097", "2000000", "Đợt 2"})
}

type disbursementImportRoute struct {
	name, method, path, perm string
	ok                       int
}

func disbursementImportRoutes() []disbursementImportRoute {
	return []disbursementImportRoute{
		{"template", "GET", disbursementImportBase + "/import-template", "budget.read", http.StatusOK},
		{"preview", "POST", disbursementImportBase + "/import-previews", "budget.update", http.StatusOK},
		{"import", "POST", disbursementImportBase + "/imports", "budget.update", http.StatusCreated},
	}
}

func (m *disbursementImportHarness) call(t *testing.T, rt disbursementImportRoute, host string, p *authz.Principal) *httptest.ResponseRecorder {
	t.Helper()
	if rt.method == "GET" {
		return m.send(t, "GET", host, rt.path, nil, p, "")
	}
	return m.send(t, "POST", host, rt.path, sampleVoucherWorkbook(t), p, disbursementImportKey)
}

// --- rule 5, invariant 7: the four cases, for each of the three routes ----------------------------

func TestDisbursementImports_401WithoutPrincipal(t *testing.T) {
	m := newDisbursementImportHarness(t)
	m.grant()
	for _, rt := range disbursementImportRoutes() {
		if w := m.call(t, rt, hostA, nil); w.Code != http.StatusUnauthorized {
			t.Errorf("%s: code = %d, want 401", rt.name, w.Code)
		}
	}
	if m.fake.calls != 0 {
		t.Errorf("not signed in, yet the use case ran %d times", m.fake.calls)
	}
}

// 403 with a real key of this service that is not the route's: `admin.lookup` for all three, and
// `budget.read` for the two writes — reading the register is not entering payments.
func TestDisbursementImports_403WrongPermission(t *testing.T) {
	for _, held := range []authz.Perm{"admin.lookup", "budget.read"} {
		m := newDisbursementImportHarness(t)
		m.checker.co = map[tenant.ID]map[authz.Perm]struct{}{xaA: {held: {}}}
		for _, rt := range disbursementImportRoutes() {
			if authz.Perm(rt.perm) == held {
				continue
			}
			if w := m.call(t, rt, hostA, canBoGhi(xaA)); w.Code != http.StatusForbidden {
				t.Errorf("%s holding %s: code = %d, want 403", rt.name, held, w.Code)
			}
		}
		if m.fake.calls != 0 {
			t.Errorf("holding %s, yet the use case ran %d times", held, m.fake.calls)
		}
	}
}

// 403 WITH THE RIGHT KEY HELD IN COMMUNE A, signed in properly at commune B (rule 5, invariant 3).
func TestDisbursementImports_403RightPermissionWrongCommune(t *testing.T) {
	for _, rt := range disbursementImportRoutes() {
		m := newDisbursementImportHarness(t)
		m.grant()
		if w := m.call(t, rt, hostB, canBoGhi(xaB)); w.Code != http.StatusForbidden {
			t.Errorf("%s: code = %d, want 403", rt.name, w.Code)
		}
		if m.fake.calls != 0 {
			t.Errorf("%s: wrong commune, yet the use case ran", rt.name)
		}
		if got := m.checker.hoiKhoaCuoi(); got != authz.Perm(rt.perm) {
			t.Errorf("%s asked for key %q, want %q", rt.name, got, rt.perm)
		}
	}
}

// 2xx: THE HOST PICKS THE COMMUNE, THE ROWS ARRIVE AS TYPED, and the trail gets the staff code.
func TestDisbursementImports_2xxRightCommuneAndStaffCode(t *testing.T) {
	for _, rt := range disbursementImportRoutes() {
		m := newDisbursementImportHarness(t)
		m.grant()
		w := m.call(t, rt, hostA, canBoGhi(xaA))
		if w.Code != rt.ok {
			t.Fatalf("%s: code = %d, want %d — %s", rt.name, w.Code, rt.ok, w.Body.String())
		}
		if rt.method == "GET" {
			continue
		}
		if m.fake.calls != 1 || m.fake.lastTen != xaA {
			t.Errorf("%s: %d calls, commune %q", rt.name, m.fake.calls, m.fake.lastTen)
		}
		rows := m.fake.lastRows
		if len(rows) != 2 || rows[0].Source != "Ngân sách tỉnh" || rows[0].Amount != "1.000.000" || rows[1].PaymentDate != "46097" || rows[1].Row != 3 {
			t.Errorf("%s: rows reaching the use case: %+v", rt.name, rows)
		}
		if rt.ok == http.StatusCreated {
			if m.fake.lastActor.ID != maCanBoGhi || m.fake.lastActor.IP == "" {
				t.Errorf("the trail must carry the STAFF CODE: %+v", m.fake.lastActor)
			}
			var out disbursementImportCreatedOut
			if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || !out.Valid || out.RowCount != 2 ||
				out.TotalAmount != 3_000_000 || len(out.Created) != 2 || out.Batch == "" || out.Errors == nil {
				t.Errorf("201 body = %s", w.Body.String())
			}
		} else {
			var out disbursementImportPreviewOut
			if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || !out.Valid || out.RowCount != 2 || out.TotalAmount != 3_000_000 {
				t.Errorf("preview body = %s", w.Body.String())
			}
		}
	}
}

// THE TEMPLATE: the header, one example row of fake data, a guide — and the example row, uploaded as
// is, reaches the planner, which refuses it by name (domain test). Round trip through the preview.
func TestDisbursementImports_TemplateContent(t *testing.T) {
	m := newDisbursementImportHarness(t)
	m.grant()
	w := m.send(t, "GET", hostA, disbursementImportBase+"/import-template", nil, canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	if w.Header().Get("Content-Type") != catalogueXLSXMIME || w.Header().Get("Cache-Control") != "no-store" ||
		!strings.Contains(w.Header().Get("Content-Disposition"), disbursementImportFilename) {
		t.Errorf("headers %v", w.Header())
	}
	cells, err := xlsx.ReadSheet(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()), xlsx.DefaultLimits)
	if err != nil || len(cells) != 2 || strings.Join(cells[0], "|") != strings.Join(domain.VoucherImportColumns(), "|") {
		t.Fatalf("data sheet = %q (%v)", cells, err)
	}
	if cells[1][0] != domain.VoucherImportExampleProjectCode {
		t.Errorf("example row code = %q", cells[1][0])
	}
	// Rule 3: no digit run that could be a phone or national ID number in the example row.
	for _, c := range cells[1] {
		if strings.Contains(c, "09") && len(c) >= 10 && !strings.ContainsAny(c, " /") {
			t.Errorf("example cell looks like personal data: %q", c)
		}
	}
	f, err := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if list := f.GetSheetList(); list[0] != disbursementImportSheet || len(list) < 2 || list[1] != xlsx.SheetGuide {
		t.Errorf("sheets = %v", list)
	}
	w = m.send(t, "POST", hostA, disbursementImportBase+"/import-previews", w.Body.Bytes(), canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	if got := m.fake.lastRows; len(got) != 1 || got[0].ProjectCode != domain.VoucherImportExampleProjectCode || got[0].Row != 2 {
		t.Errorf("rows reaching the use case %+v", got)
	}
}

func TestDisbursementImports_StatusMapping(t *testing.T) {
	m := newDisbursementImportHarness(t)
	m.grant()
	path := disbursementImportBase + "/imports"

	m.fake.err = &app.VoucherImportRejected{Errors: []domain.VoucherImportError{
		{Row: 3, Column: domain.VoucherImportColSource, Message: "Dự án đã gắn nguồn vốn — ghi tên nguồn ở cột Nguồn vốn."},
	}}
	w := m.send(t, "POST", hostA, path, sampleVoucherWorkbook(t), canBoGhi(xaA), disbursementImportKey)
	doiMa(t, w, http.StatusBadRequest)
	var out catalogueImportRejectedOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || out.Code != "import_invalid" || len(out.Errors) != 1 ||
		out.Errors[0].Row != 3 || out.Errors[0].Column != domain.VoucherImportColSource {
		t.Errorf("response: %+v (%v)", out, err)
	}

	m.fake.err = errors.New("cơ sở dữ liệu không phản hồi")
	w = m.send(t, "POST", hostA, path, sampleVoucherWorkbook(t), canBoGhi(xaA), "01JDISBURSEMENTIMPORTKEY01F")
	var e httpx.Error
	_ = json.Unmarshal(w.Body.Bytes(), &e)
	if w.Code != http.StatusInternalServerError || e.Code != "internal" || strings.Contains(w.Body.String(), "không phản hồi") {
		t.Errorf("system failure: %d %s", w.Code, w.Body.String())
	}

	m.fake.err = nil
	m.fake.res = app.VoucherImportResult{RowCount: 2, Total: 1000, Errors: []domain.VoucherImportError{{Row: 2, Column: domain.VoucherImportColAmount, Message: "x"}}}
	w = m.send(t, "POST", hostA, disbursementImportBase+"/import-previews", sampleVoucherWorkbook(t), canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	var prev disbursementImportPreviewOut
	if err := json.Unmarshal(w.Body.Bytes(), &prev); err != nil || prev.Valid || len(prev.Errors) != 1 || prev.Errors[0].Row != 2 || prev.RowCount != 2 {
		t.Errorf("preview of a bad file: %s", w.Body.String())
	}
}

// A WRONG HEADER is a content error answered without the use case, and NEVER ECHOES A CELL.
func TestDisbursementImports_WrongHeaderNeverEchoesCells(t *testing.T) {
	const secret = "Nguyễn Văn A 0900000000"
	m := newDisbursementImportHarness(t)
	m.grant()
	data := catalogueWorkbook(t, []string{secret, "Mã dự án"}, []string{secret})
	for path, code := range map[string]int{
		disbursementImportBase + "/import-previews": http.StatusOK,
		disbursementImportBase + "/imports":         http.StatusBadRequest,
	} {
		w := m.send(t, "POST", hostA, path, data, canBoGhi(xaA), disbursementImportKey)
		if w.Code != code || !strings.Contains(w.Body.String(), `"row":1`) || strings.Contains(w.Body.String(), "0900000000") {
			t.Errorf("%s: %d %s", path, w.Code, w.Body.String())
		}
	}
	if m.fake.calls != 0 {
		t.Error("wrong header, yet the use case ran")
	}
}

// THE SIZE LIMIT: a file over core/xlsx's 2 MB cap is 413 with THIS import's sentence (500 rows, not the
// catalogue's 200), before a byte is parsed and without the use case.
func TestDisbursementImports_FileOverTheCapIs413(t *testing.T) {
	m := newDisbursementImportHarness(t)
	m.grant()
	big := bytes.Repeat([]byte("x"), int(xlsx.DefaultLimits.MaxFileBytes)+10)
	for _, path := range []string{disbursementImportBase + "/import-previews", disbursementImportBase + "/imports"} {
		w := m.send(t, "POST", hostA, path, big, canBoGhi(xaA), disbursementImportKey)
		if w.Code != http.StatusRequestEntityTooLarge || !strings.Contains(w.Body.String(), "500 dòng") {
			t.Errorf("%s: %d %s", path, w.Code, w.Body.String())
		}
	}
	if m.fake.calls != 0 {
		t.Error("oversized file, yet the use case ran")
	}
}

func TestDisbursementImports_NotAWorkbookIs415(t *testing.T) {
	m := newDisbursementImportHarness(t)
	m.grant()
	w := m.send(t, "POST", hostA, disbursementImportBase+"/import-previews", []byte("DA01,0900000000"), canBoGhi(xaA), "")
	if w.Code != http.StatusUnsupportedMediaType || strings.Contains(w.Body.String(), "0900000000") {
		t.Errorf("%d %s", w.Code, w.Body.String())
	}
	if m.fake.calls != 0 {
		t.Error("broken file, yet the use case ran")
	}
}

// THE IMPORT REQUIRES AN Idempotency-Key; the preview does not.
func TestDisbursementImports_ImportNeedsIdempotencyKey(t *testing.T) {
	m := newDisbursementImportHarness(t)
	m.grant()
	if w := m.send(t, "POST", hostA, disbursementImportBase+"/imports", sampleVoucherWorkbook(t), canBoGhi(xaA), ""); w.Code/100 != 4 {
		t.Errorf("import without a key: %d", w.Code)
	}
	if w := m.send(t, "POST", hostA, disbursementImportBase+"/import-previews", sampleVoucherWorkbook(t), canBoGhi(xaA), ""); w.Code != http.StatusOK {
		t.Errorf("the preview must not need a key: %d", w.Code)
	}
	if m.fake.calls != 1 {
		t.Errorf("use case ran %d times, want 1 (the preview only)", m.fake.calls)
	}
}
