package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/xlsx"
	"github.com/vihat/vigov/service-documents/internal/app"
	"github.com/vihat/vigov/service-documents/internal/domain"
)

// THE DOCUMENT-TYPE EXCEL IMPORT — three routes under `admin.lookup`. What these tests defend, the
// map-asset-type import's list (service-comms/internal/http/map_asset_type_import_test.go):
//
//  1. rule 5, invariant 7 on all three: 401 · 403 wrong permission · 403 right permission wrong
//     commune · 200/201;
//  2. the template round-trips: the file GET hands out is a file the preview accepts;
//  3. the preview reports every error and never reaches the write; the import writes all or nothing;
//  4. duplicates (in the file, and against the commune's catalogue) are row errors;
//  5. NO ERROR BODY CONTAINS CELL TEXT.
//
// The use case is faked here (its transaction and its one audit entry are proven in
// internal/app/document_type_import_test.go over the fake driver); the fake runs the REAL planner.

// The two import methods on the shared catalogue fake, so every other harness that passes
// &ghiLoaiVanBanGia{} still satisfies GhiLoaiVanBan. Only importFake below is expected to be asked.
func (g *ghiLoaiVanBanGia) PreviewDocumentTypeImport(context.Context, []domain.DocumentTypeImportRow) (
	app.DocumentTypeImportResult, error) {
	return app.DocumentTypeImportResult{}, errors.New("ghiLoaiVanBanGia: xem trước nhập không thuộc bộ kiểm này")
}

func (g *ghiLoaiVanBanGia) ImportDocumentTypes(context.Context, []domain.DocumentTypeImportRow, audit.Actor) (
	app.DocumentTypeImportResult, error) {
	return app.DocumentTypeImportResult{}, errors.New("ghiLoaiVanBanGia: nhập không thuộc bộ kiểm này")
}

// importFake overrides the two methods, KEYED BY COMMUNE, running the real planner.
type importFake struct {
	ghiLoaiVanBanGia
	existing map[tenant.ID][]domain.ExistingDocumentType

	previewCalls, importCalls int
	written                   []domain.PlannedDocumentType
	lastActor                 audit.Actor
	lastTenant                tenant.ID
}

func (f *importFake) PreviewDocumentTypeImport(ctx context.Context, rows []domain.DocumentTypeImportRow) (
	app.DocumentTypeImportResult, error) {
	f.previewCalls++
	f.lastTenant = tenant.MustFrom(ctx)
	plan, errs := domain.PlanDocumentTypeImport(rows, f.existing[f.lastTenant], 200)
	return app.DocumentTypeImportResult{Types: plan, Errors: errs}, nil
}

func (f *importFake) ImportDocumentTypes(ctx context.Context, rows []domain.DocumentTypeImportRow, actor audit.Actor) (
	app.DocumentTypeImportResult, error) {
	f.importCalls++
	f.lastTenant, f.lastActor = tenant.MustFrom(ctx), actor
	plan, errs := domain.PlanDocumentTypeImport(rows, f.existing[f.lastTenant], 200)
	if len(errs) > 0 {
		return app.DocumentTypeImportResult{}, &app.DocumentTypeImportRejected{Errors: errs}
	}
	for i := range plan {
		plan[i].ID = fmt.Sprintf("lvb-new-%d", i)
	}
	f.written = append(f.written, plan...)
	return app.DocumentTypeImportResult{Types: plan}, nil
}

type importServer struct {
	m    *mayChu
	fake *importFake
}

func newImportServer(t *testing.T) *importServer {
	t.Helper()
	fake := &importFake{existing: map[tenant.ID][]domain.ExistingDocumentType{
		xaA: {
			{Code: "quyet-dinh", Label: "Quyết định"},
			{Code: "cong-van", Label: "Công văn cũ", Deleted: true},
		},
	}}
	// A REAL in-memory idempotency store: the import declares DongKhiHong, which answers 503 with none.
	m := dungMayChuCoIdem(t)
	m.dungLai(t, func(d *Deps) { d.GhiLoaiVanBan = fake })
	return &importServer{m: m, fake: fake}
}

// workbook builds a real .xlsx whose first sheet is header + rows, through the same writer the
// template route uses.
func workbook(t *testing.T, header []string, rows ...[]string) []byte {
	t.Helper()
	b, err := xlsx.Template(xlsx.TemplateSpec{SheetName: "Loai van ban", Header: header, Examples: rows})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func goodHeader() []string { return domain.DocumentTypeImportColumns() }

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
		r = r.WithContext(context.WithValue(r.Context(), khoaChuTheThu{}, *p))
	}
	w := httptest.NewRecorder()
	s.m.h.ServeHTTP(w, r)
	return w
}

func (s *importServer) get(t *testing.T, path, host string, p *authz.Principal) *httptest.ResponseRecorder {
	t.Helper()
	return s.m.goi(t, http.MethodGet, host, path, p)
}

const (
	pathImportTemplate = "/api/v1/document-types/import-template"
	pathImportPreview  = "/api/v1/document-types/import-previews"
	pathImport         = "/api/v1/document-types/imports"
)

type importRoute struct {
	name string
	call func(s *importServer, t *testing.T, host string, p *authz.Principal) *httptest.ResponseRecorder
	ok   int
}

func importRoutes() []importRoute {
	valid := func(t *testing.T) []byte { return workbook(t, goodHeader(), []string{"Tờ trình", "", "2"}) }
	return []importRoute{
		{"template", func(s *importServer, t *testing.T, host string, p *authz.Principal) *httptest.ResponseRecorder {
			return s.get(t, pathImportTemplate, host, p)
		}, http.StatusOK},
		{"preview", func(s *importServer, t *testing.T, host string, p *authz.Principal) *httptest.ResponseRecorder {
			return s.upload(t, pathImportPreview, host, p, valid(t), "01JIMPORTPREVIEWKEY000000")
		}, http.StatusOK},
		{"import", func(s *importServer, t *testing.T, host string, p *authz.Principal) *httptest.ResponseRecorder {
			return s.upload(t, pathImport, host, p, valid(t), "01JIMPORTKEY0000000000000")
		}, http.StatusCreated},
	}
}

// --- (1) rule 5, invariant 7 -------------------------------------------------------------------------

func TestDocumentTypeImport_401WithoutSession(t *testing.T) {
	for _, rt := range importRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			s := newImportServer(t)
			s.m.capQuyen(xaA, QuyenDanhMuc)
			doiMa(t, rt.call(s, t, hostA, nil), http.StatusUnauthorized)
			if s.fake.previewCalls+s.fake.importCalls != 0 {
				t.Error("chưa đăng nhập mà use case đã chạy")
			}
		})
	}
}

func TestDocumentTypeImport_403WrongPermission(t *testing.T) {
	for _, rt := range importRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			s := newImportServer(t)
			s.m.capQuyen(xaA, "document.read") // a real key of the same subsystem, not the one required
			doiMa(t, rt.call(s, t, hostA, canBoCua(xaA)), http.StatusForbidden)
			if s.fake.previewCalls+s.fake.importCalls != 0 {
				t.Error("sai quyền mà use case đã chạy")
			}
		})
	}
}

// Signed in at commune B, as commune B — the grant is commune A's.
func TestDocumentTypeImport_403RightPermissionWrongCommune(t *testing.T) {
	for _, rt := range importRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			s := newImportServer(t)
			s.m.capQuyen(xaA, QuyenDanhMuc)
			doiMa(t, rt.call(s, t, hostB, canBoCua(xaB)), http.StatusForbidden)
			if s.fake.previewCalls+s.fake.importCalls != 0 {
				t.Error("quyền cấp ở xã khác mà vẫn chạy ở xã này")
			}
		})
	}
}

func TestDocumentTypeImport_200WithPermissionInCommune(t *testing.T) {
	for _, rt := range importRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			s := newImportServer(t)
			s.m.capQuyen(xaA, QuyenDanhMuc)
			doiMa(t, rt.call(s, t, hostA, canBoCua(xaA)), rt.ok)
			if got := s.m.checker.hoiKhoaCuoi(); got != QuyenDanhMuc {
				t.Errorf("tuyến hỏi khoá %q, muốn %q", got, QuyenDanhMuc)
			}
		})
	}
}

// --- (2) the template round-trips ------------------------------------------------------------------

func TestDocumentTypeImport_TemplateRoundTrip(t *testing.T) {
	s := newImportServer(t)
	s.m.capQuyen(xaA, QuyenDanhMuc)
	w := s.get(t, pathImportTemplate, hostA, canBoCua(xaA))
	doiMa(t, w, http.StatusOK)
	if ct := w.Header().Get("Content-Type"); ct != xlsxMIME {
		t.Fatalf("Content-Type = %q", ct)
	}
	pw := s.upload(t, pathImportPreview, hostA, canBoCua(xaA), w.Body.Bytes(), "01JROUNDTRIPKEY0000000000")
	doiMa(t, pw, http.StatusOK)
	var out documentTypeImportPreviewOut
	if err := json.Unmarshal(pw.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if !out.Valid || len(out.Types) != 1 || out.Types[0].Code != "to-trinh" || out.Types[0].Row != 2 || out.Types[0].Order != 1 {
		t.Fatalf("tệp mẫu tải về không nhập lại được: %+v", out)
	}
}

// --- (3) preview never writes; import is all or nothing ----------------------------------------------

func TestDocumentTypeImport_PreviewWithErrorsWritesNothing(t *testing.T) {
	s := newImportServer(t)
	s.m.capQuyen(xaA, QuyenDanhMuc)
	data := workbook(t, goodHeader(),
		[]string{"Tờ trình", "", "1"},
		[]string{"", "", ""},           // blank row: skipped, numbering kept
		[]string{"", "khong-ten", "2"}, // row 4: no label
		[]string{"Báo cáo", "", "1.5"}, // row 5: order not an integer
	)
	w := s.upload(t, pathImportPreview, hostA, canBoCua(xaA), data, "01JPREVIEWERRKEY000000000")
	doiMa(t, w, http.StatusOK)
	var out documentTypeImportPreviewOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Valid || len(out.Errors) != 2 || out.Errors[0].Row != 4 || out.Errors[1].Row != 5 ||
		out.Errors[1].Column != domain.DocumentTypeImportColOrder {
		t.Fatalf("= %+v", out)
	}
	if s.fake.importCalls != 0 || len(s.fake.written) != 0 {
		t.Error("xem trước đã ghi")
	}
}

func TestDocumentTypeImport_CommitWritesAllAndPassesActor(t *testing.T) {
	s := newImportServer(t)
	s.m.capQuyen(xaA, QuyenDanhMuc)
	data := workbook(t, goodHeader(),
		[]string{"Tờ trình", "", "1"},
		[]string{"Báo cáo", "bao-cao-tuan", ""},
	)
	w := s.upload(t, pathImport, hostA, canBoCua(xaA), data, "01JCOMMITKEY0000000000000")
	doiMa(t, w, http.StatusCreated)
	var out documentTypeImportCreatedOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Created) != 2 || out.Created[0].Code != "to-trinh" || out.Created[1].Code != "bao-cao-tuan" ||
		out.Created[0].ID == "" {
		t.Fatalf("= %+v", out)
	}
	if a := s.fake.lastActor; a.ID != maCanBo || a.IP == "" || s.fake.lastTenant != xaA {
		t.Errorf("chủ thể %+v / xã %q — muốn MÃ cán bộ và xã của Host", a, s.fake.lastTenant)
	}
}

func TestDocumentTypeImport_OneBadRowRefusesTheWholeFile(t *testing.T) {
	s := newImportServer(t)
	s.m.capQuyen(xaA, QuyenDanhMuc)
	data := workbook(t, goodHeader(),
		[]string{"Tờ trình", "", "1"},
		[]string{"Báo cáo", "Bao Cao", ""}, // upper case: refused, never lower-cased
	)
	w := s.upload(t, pathImport, hostA, canBoCua(xaA), data, "01JONEBADKEY0000000000000")
	doiMa(t, w, http.StatusBadRequest)
	var out documentTypeImportRejectedOut
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

func TestDocumentTypeImport_DuplicatesAreRowErrors(t *testing.T) {
	s := newImportServer(t)
	s.m.capQuyen(xaA, QuyenDanhMuc)
	data := workbook(t, goodHeader(),
		[]string{"Quyết định", "", ""},         // row 2: code and label of a live row
		[]string{"Công văn", "cong-van", ""},   // row 3: code of a SOFT-DELETED row — still taken
		[]string{"Tờ trình", "", ""},           // row 4: fine
		[]string{"tờ trình", "to-trinh-2", ""}, // row 5: same label as row 4 (case-folded)
		[]string{"Đề án", "to-trinh", ""},      // row 6: same code as row 4's generated one
	)
	w := s.upload(t, pathImport, hostA, canBoCua(xaA), data, "01JDUPKEY0000000000000000")
	doiMa(t, w, http.StatusBadRequest)
	var out documentTypeImportRejectedOut
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

func TestDocumentTypeImport_ErrorBodiesNeverEchoCells(t *testing.T) {
	const marker = "DULIEUONLYINCELL"
	s := newImportServer(t)
	s.m.capQuyen(xaA, QuyenDanhMuc)
	cases := map[string][]byte{
		"wrong header": workbook(t, []string{marker, "b", "c"}, []string{"x", "", ""}),
		"bad code":     workbook(t, goodHeader(), []string{"Tên", marker + " !", ""}),
		"bad order":    workbook(t, goodHeader(), []string{"Tên", "", marker}),
		"long label":   workbook(t, goodHeader(), []string{strings.Repeat(marker, 20), "", ""}),
		"extra column": workbook(t, append(goodHeader(), ""), []string{"Tên", "", "", marker}),
		"duplicate":    workbook(t, goodHeader(), []string{marker, "", ""}, []string{marker, "", ""}),
	}
	for name, data := range cases {
		for _, path := range []string{pathImportPreview, pathImport} {
			w := s.upload(t, path, hostA, canBoCua(xaA), data, "01JNOECHO"+strings.ReplaceAll(name, " ", "")+path[len(path)-4:])
			if strings.Contains(w.Body.String(), marker) {
				t.Errorf("%s %s: thân trả về chứa nội dung ô: %s", name, path, w.Body.String())
			}
			if path == pathImport && w.Code != http.StatusBadRequest {
				t.Errorf("%s: mã %d, muốn 400", name, w.Code)
			}
		}
	}
}

func TestDocumentTypeImport_NotAWorkbookIs415WithFixedSentence(t *testing.T) {
	s := newImportServer(t)
	s.m.capQuyen(xaA, QuyenDanhMuc)
	w := s.upload(t, pathImportPreview, hostA, canBoCua(xaA), []byte("ma,nhan\nDULIEUONLYINCELL,x\n"),
		"01JNOTXLSXKEY000000000000")
	doiMa(t, w, http.StatusUnsupportedMediaType)
	if strings.Contains(w.Body.String(), "DULIEUONLYINCELL") {
		t.Error("lỗi nhắc lại nội dung tệp")
	}
}
