package http

import (
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
	"net/textproto"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/xlsx"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// THE MAP-ASSET-TYPE EXCEL IMPORT — three routes under `admin.lookup`. What these tests defend:
//
//  1. rule 5, invariant 7 on all three: 401 · 403 wrong permission · 403 right permission wrong
//     commune · 200/201;
//  2. the template round-trips: the file GET hands out is a file the preview accepts;
//  3. the preview reports every error and never reaches the write; the import writes all or nothing;
//  4. duplicates (in the file, and against the commune's catalogue) are row errors;
//  5. NO ERROR BODY CONTAINS CELL TEXT.
//
// The use case is faked here (its transaction and its one audit entry are proven in
// internal/app/map_asset_type_import_test.go over the fake driver); the fake runs the REAL planner,
// so the errors asserted are the domain's.

// The two import methods on the shared catalogue fake, so every other harness that passes
// &ghiDanhMucGia{} still satisfies GhiLoaiTaiNguyen. Nothing but importFake below is expected to be
// asked for an import.
func (g *ghiDanhMucGia) PreviewMapAssetTypeImport(context.Context, []domain.MapAssetTypeImportRow) (
	app.MapAssetTypeImportResult, error) {
	return app.MapAssetTypeImportResult{}, errors.New("ghiDanhMucGia: xem trước nhập không thuộc bộ kiểm này")
}

func (g *ghiDanhMucGia) ImportMapAssetTypes(context.Context, []domain.MapAssetTypeImportRow, audit.Actor) (
	app.MapAssetTypeImportResult, error) {
	return app.MapAssetTypeImportResult{}, errors.New("ghiDanhMucGia: nhập không thuộc bộ kiểm này")
}

// importFake overrides the two methods, KEYED BY COMMUNE, running the real planner.
type importFake struct {
	ghiDanhMucGia
	existing map[tenant.ID][]domain.ExistingMapAssetType

	previewCalls, importCalls int
	written                   []domain.PlannedMapAssetType
	lastActor                 audit.Actor
	lastTenant                tenant.ID
}

func (f *importFake) PreviewMapAssetTypeImport(ctx context.Context, rows []domain.MapAssetTypeImportRow) (
	app.MapAssetTypeImportResult, error) {
	f.previewCalls++
	f.lastTenant = tenant.MustFrom(ctx)
	plan, errs := domain.PlanMapAssetTypeImport(rows, f.existing[f.lastTenant], 500)
	return app.MapAssetTypeImportResult{Types: plan, Errors: errs}, nil
}

func (f *importFake) ImportMapAssetTypes(ctx context.Context, rows []domain.MapAssetTypeImportRow, actor audit.Actor) (
	app.MapAssetTypeImportResult, error) {
	f.importCalls++
	f.lastTenant, f.lastActor = tenant.MustFrom(ctx), actor
	plan, errs := domain.PlanMapAssetTypeImport(rows, f.existing[f.lastTenant], 500)
	if len(errs) > 0 {
		return app.MapAssetTypeImportResult{}, &app.MapAssetTypeImportRejected{Errors: errs}
	}
	for i := range plan {
		plan[i].ID = fmt.Sprintf("ltn-new-%d", i)
	}
	f.written = append(f.written, plan...)
	return app.MapAssetTypeImportResult{Types: plan}, nil
}

type importServer struct {
	h       http.Handler
	fake    *importFake
	checker *checkerDanhMucGia
}

func newImportServer(t *testing.T) *importServer {
	t.Helper()
	fake := &importFake{existing: map[tenant.ID][]domain.ExistingMapAssetType{
		xaA: {
			{Code: "doanh-nghiep", Label: "Doanh nghiệp"},
			{Code: "cho", Label: "Chợ cũ", Deleted: true},
		},
	}}
	checker := &checkerDanhMucGia{}
	im := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	Register(mux, Deps{
		Checker:              checker,
		LoaiTaiNguyen:        danhMucMau(),
		GhiLoaiTaiNguyen:     fake,
		ThongBao:             &soThongBaoGia{},
		GhiThongBao:          &ghiThongBaoGia{},
		NoiDung:              &soNoiDungGia{},
		GhiNoiDung:           &ghiNoiDungGia{},
		DanhMucNoiDung:       &soDanhMucNDGia{},
		GhiDanhMucNoiDung:    &ghiDanhMucNDGia{},
		ContentCovers:        &fakeCovers{},
		MapFieldSchemas:      &fakeMapFieldSchemas{},
		WriteMapFieldSchemas: &fakeMapFieldSchemas{},
		MailSettings:         &fakeMailSettings{},
		WriteMailSettings:    &fakeMailSettings{},
		AuditLog:             &auditLogFake{},
		StaffInbox:           &fakeInbox{},
		WriteStaffInbox:      &fakeInbox{},
		Log:                  im,
	})
	var h http.Handler = mux
	h = chuTheGhi(h)
	// A REAL in-memory idempotency store: the import declares DongKhiHong, which answers 503 with none.
	h = idem.Middleware(moKhoIdemGia(), im)(h)
	h = httpx.TenantMiddleware(thuMucMau())(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)
	return &importServer{h: h, fake: fake, checker: checker}
}

func (s *importServer) grant(xa tenant.ID, perms ...authz.Perm) {
	if s.checker.co == nil {
		s.checker.co = map[tenant.ID]map[authz.Perm]struct{}{}
	}
	if s.checker.co[xa] == nil {
		s.checker.co[xa] = map[authz.Perm]struct{}{}
	}
	for _, p := range perms {
		s.checker.co[xa][p] = struct{}{}
	}
}

// workbook builds a real .xlsx whose first sheet is header + rows, through the same writer the
// template route uses.
func workbook(t *testing.T, header []string, rows ...[]string) []byte {
	t.Helper()
	b, err := xlsx.Template(xlsx.TemplateSpec{SheetName: "Loai tai nguyen", Header: header, Examples: rows})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func goodHeader() []string { return domain.MapAssetTypeImportColumns() }

func multipartFile(t *testing.T, name string, data []byte) (string, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	hdr := textproto.MIMEHeader{}
	hdr.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, name))
	hdr.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	part, err := mw.CreatePart(hdr)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return mw.FormDataContentType(), &buf
}

func (s *importServer) upload(t *testing.T, path, host string, p *authz.Principal, data []byte, key string) *httptest.ResponseRecorder {
	t.Helper()
	ct, body := multipartFile(t, "loai.xlsx", data)
	r := httptest.NewRequest(http.MethodPost, "https://"+host+path, body)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	r.Header.Set("Content-Type", ct)
	r.Header.Set(idem.Header, key)
	if p != nil {
		r = r.WithContext(context.WithValue(r.Context(), khoaChuTheGhi{}, *p))
	}
	w := httptest.NewRecorder()
	s.h.ServeHTTP(w, r)
	return w
}

func (s *importServer) get(t *testing.T, path, host string, p *authz.Principal) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "https://"+host+path, nil)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	if p != nil {
		r = r.WithContext(context.WithValue(r.Context(), khoaChuTheGhi{}, *p))
	}
	w := httptest.NewRecorder()
	s.h.ServeHTTP(w, r)
	return w
}

const (
	pathTemplate = "/api/v1/map-asset-types/import-template"
	pathPreview  = "/api/v1/map-asset-types/import-previews"
	pathImport   = "/api/v1/map-asset-types/imports"
)

type importRoute struct {
	name string
	call func(s *importServer, t *testing.T, host string, p *authz.Principal) *httptest.ResponseRecorder
	ok   int
}

func importRoutes() []importRoute {
	valid := func(t *testing.T) []byte { return workbook(t, goodHeader(), []string{"Hộ kinh doanh", "", "2"}) }
	return []importRoute{
		{"template", func(s *importServer, t *testing.T, host string, p *authz.Principal) *httptest.ResponseRecorder {
			return s.get(t, pathTemplate, host, p)
		}, http.StatusOK},
		{"preview", func(s *importServer, t *testing.T, host string, p *authz.Principal) *httptest.ResponseRecorder {
			return s.upload(t, pathPreview, host, p, valid(t), "01JIMPORTPREVIEWKEY000000")
		}, http.StatusOK},
		{"import", func(s *importServer, t *testing.T, host string, p *authz.Principal) *httptest.ResponseRecorder {
			return s.upload(t, pathImport, host, p, valid(t), "01JIMPORTKEY0000000000000")
		}, http.StatusCreated},
	}
}

// --- (1) rule 5, invariant 7 -------------------------------------------------------------------------

func TestMapAssetTypeImport_401WithoutSession(t *testing.T) {
	for _, rt := range importRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			s := newImportServer(t)
			s.grant(xaA, "admin.lookup")
			doiMa(t, rt.call(s, t, hostA, nil), http.StatusUnauthorized)
			if s.fake.previewCalls+s.fake.importCalls != 0 {
				t.Error("chưa đăng nhập mà use case đã chạy")
			}
		})
	}
}

func TestMapAssetTypeImport_403WrongPermission(t *testing.T) {
	for _, rt := range importRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			s := newImportServer(t)
			s.grant(xaA, "asset.read") // a real key of the same subsystem, not the one required
			doiMa(t, rt.call(s, t, hostA, canBoGhi(xaA)), http.StatusForbidden)
			if s.fake.previewCalls+s.fake.importCalls != 0 {
				t.Error("sai quyền mà use case đã chạy")
			}
		})
	}
}

// Signed in at commune B, as commune B — the grant is commune A's. See TestGhiLoaiTaiNguyen_403DungQuyenSaiXa.
func TestMapAssetTypeImport_403RightPermissionWrongCommune(t *testing.T) {
	for _, rt := range importRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			s := newImportServer(t)
			s.grant(xaA, "admin.lookup")
			doiMa(t, rt.call(s, t, hostB, canBoGhi(xaB)), http.StatusForbidden)
			if s.fake.previewCalls+s.fake.importCalls != 0 {
				t.Error("quyền cấp ở xã khác mà vẫn chạy ở xã này")
			}
		})
	}
}

func TestMapAssetTypeImport_200WithPermissionInCommune(t *testing.T) {
	for _, rt := range importRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			s := newImportServer(t)
			s.grant(xaA, "admin.lookup")
			doiMa(t, rt.call(s, t, hostA, canBoGhi(xaA)), rt.ok)
			if got := s.checker.hoiKhoaCuoi(); got != "admin.lookup" {
				t.Errorf("tuyến hỏi khoá %q, muốn \"admin.lookup\"", got)
			}
		})
	}
}

// --- (2) the template round-trips ------------------------------------------------------------------

func TestMapAssetTypeImport_TemplateRoundTrip(t *testing.T) {
	s := newImportServer(t)
	s.grant(xaA, "admin.lookup")
	w := s.get(t, pathTemplate, hostA, canBoGhi(xaA))
	doiMa(t, w, http.StatusOK)
	if ct := w.Header().Get("Content-Type"); ct != xlsxMIME {
		t.Fatalf("Content-Type = %q", ct)
	}
	// The downloaded file, uploaded back as is: its example row is valid, so the preview says so.
	pw := s.upload(t, pathPreview, hostA, canBoGhi(xaA), w.Body.Bytes(), "01JROUNDTRIPKEY0000000000")
	doiMa(t, pw, http.StatusOK)
	var out mapAssetTypeImportPreviewOut
	if err := json.Unmarshal(pw.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if !out.Valid || len(out.Types) != 1 || out.Types[0].Code != "ho-kinh-doanh" || out.Types[0].Row != 2 || out.Types[0].Order != 1 {
		t.Fatalf("tệp mẫu tải về không nhập lại được: %+v", out)
	}
}

// --- (3) preview never writes; import is all or nothing ----------------------------------------------

func TestMapAssetTypeImport_PreviewWithErrorsWritesNothing(t *testing.T) {
	s := newImportServer(t)
	s.grant(xaA, "admin.lookup")
	data := workbook(t, goodHeader(),
		[]string{"Hợp tác xã", "", "1"},
		[]string{"", "", ""},              // blank row: skipped, numbering kept
		[]string{"", "khong-ten", "2"},    // row 4: no label
		[]string{"Trang trại", "", "1.5"}, // row 5: order not an integer
	)
	w := s.upload(t, pathPreview, hostA, canBoGhi(xaA), data, "01JPREVIEWERRKEY000000000")
	doiMa(t, w, http.StatusOK)
	var out mapAssetTypeImportPreviewOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Valid || len(out.Errors) != 2 || out.Errors[0].Row != 4 || out.Errors[1].Row != 5 ||
		out.Errors[1].Column != domain.MapAssetTypeImportColOrder {
		t.Fatalf("= %+v", out)
	}
	if s.fake.importCalls != 0 || len(s.fake.written) != 0 {
		t.Error("xem trước đã ghi")
	}
}

func TestMapAssetTypeImport_CommitWritesAllAndPassesActor(t *testing.T) {
	s := newImportServer(t)
	s.grant(xaA, "admin.lookup")
	data := workbook(t, goodHeader(),
		[]string{"Hợp tác xã", "", "1"},
		[]string{"Trang trại", "trang-trai-lon", ""},
	)
	w := s.upload(t, pathImport, hostA, canBoGhi(xaA), data, "01JCOMMITKEY0000000000000")
	doiMa(t, w, http.StatusCreated)
	var out mapAssetTypeImportCreatedOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Created) != 2 || out.Created[0].Code != "hop-tac-xa" || out.Created[1].Code != "trang-trai-lon" ||
		out.Created[0].ID == "" {
		t.Fatalf("= %+v", out)
	}
	if a := s.fake.lastActor; a.ID != maCanBoGhi || a.IP == "" || s.fake.lastTenant != xaA {
		t.Errorf("chủ thể %+v / xã %q — muốn MÃ cán bộ và xã của Host", a, s.fake.lastTenant)
	}
}

func TestMapAssetTypeImport_OneBadRowRefusesTheWholeFile(t *testing.T) {
	s := newImportServer(t)
	s.grant(xaA, "admin.lookup")
	data := workbook(t, goodHeader(),
		[]string{"Hợp tác xã", "", "1"},
		[]string{"Trang trại", "Trang Trai", ""}, // upper case: refused, never lower-cased
	)
	w := s.upload(t, pathImport, hostA, canBoGhi(xaA), data, "01JONEBADKEY0000000000000")
	doiMa(t, w, http.StatusBadRequest)
	var out mapAssetTypeImportRejectedOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Code != "import_invalid" || len(out.Errors) != 1 || out.Errors[0].Row != 3 {
		t.Fatalf("= %+v", out)
	}
	if len(s.fake.written) != 0 {
		t.Error("một dòng lỗi mà vẫn ghi những dòng khác")
	}
}

// --- (4) duplicates -------------------------------------------------------------------------------

func TestMapAssetTypeImport_DuplicatesAreRowErrors(t *testing.T) {
	s := newImportServer(t)
	s.grant(xaA, "admin.lookup")
	data := workbook(t, goodHeader(),
		[]string{"Doanh nghiệp", "", ""},             // row 2: code and label of a live row
		[]string{"Chợ", "cho", ""},                   // row 3: code of a SOFT-DELETED row — still taken
		[]string{"Nhà văn hoá", "", ""},              // row 4: fine
		[]string{"nhà văn hoá", "nha-van-hoa-2", ""}, // row 5: same label as row 4 (case-folded)
		[]string{"Bến xe", "nha-van-hoa", ""},        // row 6: same code as row 4's generated one
	)
	w := s.upload(t, pathImport, hostA, canBoGhi(xaA), data, "01JDUPKEY0000000000000000")
	doiMa(t, w, http.StatusBadRequest)
	var out mapAssetTypeImportRejectedOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	rows := map[int]int{}
	for _, e := range out.Errors {
		rows[e.Row]++
	}
	if rows[2] != 2 || rows[3] != 1 || rows[4] != 0 || rows[5] != 1 || rows[6] != 1 {
		t.Fatalf("lỗi theo dòng = %v (%+v)", rows, out.Errors)
	}
	if len(s.fake.written) != 0 {
		t.Error("tệp có dòng trùng mà vẫn ghi")
	}
}

// --- (5) no cell text in any error body --------------------------------------------------------------

func TestMapAssetTypeImport_ErrorBodiesNeverEchoCells(t *testing.T) {
	const marker = "DULIEUONLYINCELL"
	s := newImportServer(t)
	s.grant(xaA, "admin.lookup")
	cases := map[string][]byte{
		"wrong header": workbook(t, []string{marker, "b", "c"}, []string{"x", "", ""}),
		"bad code":     workbook(t, goodHeader(), []string{"Tên", marker + " !", ""}),
		"bad order":    workbook(t, goodHeader(), []string{"Tên", "", marker}),
		"long label":   workbook(t, goodHeader(), []string{strings.Repeat(marker, 20), "", ""}),
		"extra column": workbook(t, append(goodHeader(), ""), []string{"Tên", "", "", marker}),
		"duplicate":    workbook(t, goodHeader(), []string{marker, "", ""}, []string{marker, "", ""}),
	}
	for name, data := range cases {
		for _, path := range []string{pathPreview, pathImport} {
			w := s.upload(t, path, hostA, canBoGhi(xaA), data, "01JNOECHO"+strings.ReplaceAll(name, " ", "")+path[len(path)-4:])
			if strings.Contains(w.Body.String(), marker) {
				t.Errorf("%s %s: thân trả về chứa nội dung ô: %s", name, path, w.Body.String())
			}
			if path == pathImport && w.Code != http.StatusBadRequest {
				t.Errorf("%s: mã %d, muốn 400", name, w.Code)
			}
		}
	}
}

func TestMapAssetTypeImport_NotAWorkbookIs415WithFixedSentence(t *testing.T) {
	s := newImportServer(t)
	s.grant(xaA, "admin.lookup")
	w := s.upload(t, pathPreview, hostA, canBoGhi(xaA), []byte("ho-ten,so-dien-thoai\nDULIEUONLYINCELL,0900000000\n"),
		"01JNOTXLSXKEY000000000000")
	doiMa(t, w, http.StatusUnsupportedMediaType)
	if strings.Contains(w.Body.String(), "DULIEUONLYINCELL") || strings.Contains(w.Body.String(), "0900000000") {
		t.Error("lỗi nhắc lại nội dung tệp")
	}
}
