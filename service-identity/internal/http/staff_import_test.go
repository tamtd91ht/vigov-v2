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

	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/xlsx"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
	"github.com/xuri/excelize/v2"
)

// THE THREE STAFF IMPORT ROUTES, all `admin.user` (ADR 0059 §1). What is asserted here is what the
// HANDLER owns: the permission, the upload, the wire shapes (masked mobile in the preview, passwords in
// the 201 only, no-store), the 403 for a role column, and that an idempotent replay returns the batch
// code and NO password. The rules are proved in domain/staff_import_test.go, the transaction and the
// `admin.role` gate over a real transaction boundary in app/staff_import_test.go.

const (
	pathStaffImports   = "/api/v1/staff/imports"
	pathStaffPreviews  = "/api/v1/staff/import-previews"
	pathStaffTemplate  = "/api/v1/staff/import-template"
	fakeStaffPassword  = "MAT-KHAU-TAM-GIA-KHONG-THAT"
	fakeStaffMobile    = "0900000000"
	fakeStaffBatchCode = "01JSTAFFBATCH0000000000000"
)

// staffImportsFake stands in for *app.StaffImporter.
type staffImportsFake struct {
	calls     int
	lastRows  []domain.StaffImportRow
	lastActor app.NguoiThucHien
	lastXa    tenant.ID

	units []domain.StaffImportOrgUnitChoice
	roles []domain.StaffImportRoleChoice
	res   app.StaffImportResult
	err   error
}

func staffImportsSample() *staffImportsFake {
	return &staffImportsFake{
		units: []domain.StaffImportOrgUnitChoice{{ID: "bp-1", Code: "van-phong", Name: "VĂN PHÒNG"}},
		roles: []domain.StaffImportRoleChoice{{ID: "vt-1", Code: "chuyen-vien", Name: "Chuyên viên"}},
		res: app.StaffImportResult{Batch: fakeStaffBatchCode, People: []app.ImportedStaff{
			{Row: 2, Staff: domain.CanBoTomTat{ID: "nd-moi-1", Ma: "CB-2026-AAAAAA", HoTen: "Nguyễn Văn An", Email: "an@xa.gov.vn",
				DiDongCaNhan: fakeStaffMobile, DienThoaiCoQuan: "0200000000"},
				OrgUnitCode: "van-phong", RoleCode: "chuyen-vien", RoleID: "vt-1", AccountIssued: true, TemporaryPassword: fakeStaffPassword},
			{Row: 3, Staff: domain.CanBoTomTat{ID: "nd-moi-2", Ma: "CB-2026-BBBBBB", HoTen: "Nguyễn Văn An"}},
		}},
	}
}

func (f *staffImportsFake) TemplateChoices(ctx context.Context) ([]domain.StaffImportOrgUnitChoice, []domain.StaffImportRoleChoice, error) {
	f.calls++
	f.lastXa = tenant.MustFrom(ctx)
	return f.units, f.roles, f.err
}

func (f *staffImportsFake) Preview(ctx context.Context, rows []domain.StaffImportRow, actor app.NguoiThucHien) (app.StaffImportResult, error) {
	f.calls++
	f.lastRows, f.lastActor, f.lastXa = rows, actor, tenant.MustFrom(ctx)
	res := f.res
	res.Batch = ""
	people := make([]app.ImportedStaff, len(res.People))
	for i, p := range res.People {
		p.TemporaryPassword, p.Staff.Ma, p.Staff.ID = "", "", ""
		people[i] = p
	}
	res.People = people
	return res, f.err
}

func (f *staffImportsFake) Import(ctx context.Context, rows []domain.StaffImportRow, actor app.NguoiThucHien) (app.StaffImportResult, error) {
	f.calls++
	f.lastRows, f.lastActor, f.lastXa = rows, actor, tenant.MustFrom(ctx)
	return f.res, f.err
}

func sampleStaffWorkbook(t *testing.T) []byte {
	return residentialUnitWorkbook(t, domain.StaffImportColumns(),
		[]any{"Nguyễn Văn An", "an@xa.gov.vn", "Chuyên viên", "van-phong · VĂN PHÒNG", "chuyen-vien · Chuyên viên", "0200000000", fakeStaffMobile},
		[]any{"Nguyễn Văn An", "", "", "", "", "", ""},
	)
}

type staffImportRoute struct {
	name, method, path string
	ok                 int
}

func staffImportRoutes() []staffImportRoute {
	return []staffImportRoute{
		{"tải mẫu", "GET", pathStaffTemplate, http.StatusOK},
		{"xem trước", "POST", pathStaffPreviews, http.StatusOK},
		{"nhập", "POST", pathStaffImports, http.StatusCreated},
	}
}

func (m *mayChu) callStaffImportRoute(t *testing.T, rt staffImportRoute, host, tok string) *httptest.ResponseRecorder {
	t.Helper()
	if rt.method == "GET" {
		return m.goi(t, "GET", host, rt.path, "", tok)
	}
	return m.sendWorkbook(t, rt.path, host, tok, sampleStaffWorkbook(t))
}

// --- rule 5, invariant 7: the four cases, for each of the three routes ----------------------------

func TestStaffImports_401WithoutToken(t *testing.T) {
	m := dungMayChu(t)
	for _, rt := range staffImportRoutes() {
		if w := m.callStaffImportRoute(t, rt, hostA, ""); w.Code != http.StatusUnauthorized {
			t.Errorf("%s: mã = %d, muốn 401", rt.name, w.Code)
		}
	}
	if m.staffImports.calls != 0 {
		t.Errorf("chưa đăng nhập mà use case đã chạy %d lần", m.staffImports.calls)
	}
}

// 403 WITH `admin.lookup` — a real key that is not `admin.user`.
func TestStaffImports_403WrongPermission(t *testing.T) {
	m := dungMayChuDanhMuc(t)
	tok := m.tokenCho(t, xaA, sidA)
	for _, rt := range staffImportRoutes() {
		if w := m.callStaffImportRoute(t, rt, hostA, tok); w.Code != http.StatusForbidden {
			t.Errorf("%s: mã = %d, muốn 403", rt.name, w.Code)
		}
	}
	if m.staffImports.calls != 0 {
		t.Errorf("sai quyền mà use case đã chạy %d lần", m.staffImports.calls)
	}
}

// 403 WITH `admin.user` HELD IN COMMUNE A, signed in properly at commune B (rule 5, invariant 3).
func TestStaffImports_403RightPermissionWrongCommune(t *testing.T) {
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaB, sidB)
	for _, rt := range staffImportRoutes() {
		if w := m.callStaffImportRoute(t, rt, hostB, tok); w.Code != http.StatusForbidden {
			t.Errorf("%s: mã = %d, muốn 403", rt.name, w.Code)
		}
	}
	if m.staffImports.calls != 0 {
		t.Errorf("sai xã mà use case đã chạy %d lần", m.staffImports.calls)
	}
}

// 2xx, THE HOST'S COMMUNE REACHES THE USE CASE, AND THE ACTOR CARRIES BOTH IDENTIFIERS.
func TestStaffImports_2xxCommuneAndStaffCode(t *testing.T) {
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)
	for _, rt := range staffImportRoutes() {
		if w := m.callStaffImportRoute(t, rt, hostA, tok); w.Code != rt.ok {
			t.Fatalf("%s: mã = %d, muốn %d — %s", rt.name, w.Code, rt.ok, w.Body.String())
		}
	}
	f := m.staffImports
	if f.lastXa != xaA || f.lastActor.Vet.ID != maCanBo || f.lastActor.ID != idNoiBo {
		t.Errorf("xã %q, người thực hiện %+v", f.lastXa, f.lastActor)
	}
	if len(f.lastRows) != 2 || f.lastRows[0].Mobile != fakeStaffMobile || f.lastRows[0].Role != "chuyen-vien · Chuyên viên" || f.lastRows[1].Email != "" {
		t.Errorf("dòng tới use case: %+v", f.lastRows)
	}
}

// --- the wire --------------------------------------------------------------------------------------

func TestStaffImports_PreviewMasksTheMobileAndCarriesNoPassword(t *testing.T) {
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)
	w := m.sendWorkbook(t, pathStaffPreviews, hostA, tok, sampleStaffWorkbook(t))
	doiMa(t, w, http.StatusOK)
	body := w.Body.String()
	if strings.Contains(body, fakeStaffMobile) || !strings.Contains(body, `"mobile":"09****0000"`) {
		t.Errorf("di động cá nhân phải bị che trong bản xem trước: %s", body)
	}
	if strings.Contains(body, fakeStaffPassword) || strings.Contains(body, "temporary_password") {
		t.Errorf("xem trước không bao giờ mang mật khẩu: %s", body)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q", w.Header().Get("Cache-Control"))
	}
	var out staffImportPreviewOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || !out.Valid || len(out.People) != 2 ||
		!out.People[0].IssuesAccount || out.People[1].IssuesAccount || out.People[0].OfficePhone != "0200000000" {
		t.Errorf("xem trước: %+v (%v)", out, err)
	}
}

func TestStaffImports_CreatedCarriesPasswordsOnceNoStore(t *testing.T) {
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)
	w := m.sendWorkbook(t, pathStaffImports, hostA, tok, sampleStaffWorkbook(t))
	doiMa(t, w, http.StatusCreated)
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q", w.Header().Get("Cache-Control"))
	}
	var out staffImportCreatedOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("thân: %v", err)
	}
	if out.BatchID != fakeStaffBatchCode || len(out.Created) != 2 {
		t.Fatalf("phản hồi: %+v", out)
	}
	a, b := out.Created[0], out.Created[1]
	if a.Code != "CB-2026-AAAAAA" || a.Login != "an@xa.gov.vn" || !a.AccountIssued || a.TemporaryPassword != fakeStaffPassword {
		t.Errorf("dòng 2: %+v", a)
	}
	if b.AccountIssued || b.Login != "" || strings.Contains(w.Body.String(), `"temporary_password":""`) {
		t.Errorf("dòng không tài khoản không mang trường mật khẩu: %+v %s", b, w.Body.String())
	}
	if strings.Contains(w.Body.String(), fakeStaffMobile) || strings.Contains(w.Body.String(), "0200000000") {
		t.Errorf("phản hồi ghi không mang số điện thoại: %s", w.Body.String())
	}
}

// A RETRY WITH THE SAME Idempotency-Key replays the BATCH CODE and never a password (core/idem stores
// no body). The use case ran once.
func TestStaffImports_ReplayReturnsBatchCodeWithoutPasswords(t *testing.T) {
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)
	send := func() *httptest.ResponseRecorder {
		body, ct := multipartBody(t, sampleStaffWorkbook(t), mimeXLSX, "can-bo.xlsx")
		r := httptest.NewRequest("POST", "https://"+hostA+pathStaffImports, body)
		r.Host = hostA
		r.RemoteAddr = "10.0.0.7:51000"
		r.Header.Set("Content-Type", ct)
		r.Header.Set(idem.Header, "01JIDEMKEYSTAFFIMPORTREPLAY")
		r.AddCookie(&http.Cookie{Name: CookiePhien, Value: tok})
		return m.chay(r)
	}
	first := send()
	doiMa(t, first, http.StatusCreated)
	if !strings.Contains(first.Body.String(), fakeStaffPassword) {
		t.Fatalf("lần đầu phải mang mật khẩu: %s", first.Body.String())
	}
	again := send()
	if again.Code != http.StatusCreated || again.Header().Get(idem.HeaderPhatLai) != "true" {
		t.Fatalf("lần hai: %d %v %s", again.Code, again.Header(), again.Body.String())
	}
	if strings.Contains(again.Body.String(), fakeStaffPassword) || !strings.Contains(again.Body.String(), fakeStaffBatchCode) {
		t.Errorf("phát lại phải chỉ mang mã lô, không mật khẩu: %s", again.Body.String())
	}
	if m.staffImports.calls != 1 {
		t.Errorf("use case chạy %d lần, muốn 1", m.staffImports.calls)
	}
}

func TestStaffImports_StatusMapping(t *testing.T) {
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)

	m.staffImports.err = &app.StaffImportRejected{Errors: []domain.StaffImportError{
		{Row: 3, Column: domain.StaffImportColEmail, Message: "Trùng"},
	}}
	w := m.sendWorkbook(t, pathStaffImports, hostA, tok, sampleStaffWorkbook(t))
	doiMa(t, w, http.StatusBadRequest)
	var out staffImportRejectedOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || out.Code != "import_invalid" || len(out.Errors) != 1 || out.Errors[0].Row != 3 {
		t.Errorf("phản hồi: %+v (%v)", out, err)
	}

	for _, c := range []struct {
		path string
		err  error
		code int
		id   string
	}{
		{pathStaffImports, app.ErrRoleAssignmentNotPermitted, http.StatusForbidden, "role_permission_required"},
		{pathStaffPreviews, app.ErrRoleAssignmentNotPermitted, http.StatusForbidden, "role_permission_required"},
		{pathStaffImports, idstore.ErrEmailDaDung, http.StatusConflict, "staff_changed"},
		{pathStaffImports, idstore.ErrVaiTroKhongTonTai, http.StatusConflict, "staff_changed"},
		{pathStaffImports, app.ErrStaffImportChanged, http.StatusConflict, "staff_changed"},
		{pathStaffImports, errors.New("cơ sở dữ liệu không phản hồi"), http.StatusInternalServerError, "internal"},
	} {
		m.staffImports.err = c.err
		w := m.sendWorkbook(t, c.path, hostA, tok, sampleStaffWorkbook(t))
		if got := loiTra(t, w).Code; w.Code != c.code || got != c.id || strings.Contains(w.Body.String(), "không phản hồi") {
			t.Errorf("%s %v: %d (%s)", c.path, c.err, w.Code, got)
		}
	}
}

// A WRONG HEADER is a content error answered without the use case, and never echoes a cell.
func TestStaffImports_WrongHeaderNeverEchoesCells(t *testing.T) {
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)
	data := residentialUnitWorkbook(t, []string{"Họ và tên", "Di động"}, []any{"Nguyễn Văn Bí Mật", fakeStaffMobile})
	for path, code := range map[string]int{pathStaffPreviews: http.StatusOK, pathStaffImports: http.StatusBadRequest} {
		w := m.sendWorkbook(t, path, hostA, tok, data)
		if w.Code != code || !strings.Contains(w.Body.String(), `"row":1`) ||
			strings.Contains(w.Body.String(), fakeStaffMobile) || strings.Contains(w.Body.String(), "Bí Mật") {
			t.Errorf("%s: %d %s", path, w.Code, w.Body.String())
		}
	}
	if m.staffImports.calls != 0 {
		t.Error("tiêu đề sai mà use case vẫn chạy")
	}
}

func TestStaffImports_TemplateHasDropdownsAndNoCodeColumn(t *testing.T) {
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)
	w := m.goi(t, "GET", hostA, pathStaffTemplate, "", tok)
	doiMa(t, w, http.StatusOK)
	if w.Header().Get("Content-Type") != mimeXLSX || w.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("header %v", w.Header())
	}
	f, err := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatalf("tệp mẫu: %v", err)
	}
	defer f.Close()
	rows, _ := f.GetRows(staffSheet)
	if len(rows) != 1 || strings.Join(rows[0], "|") != strings.Join(domain.StaffImportColumns(), "|") {
		t.Fatalf("trang dữ liệu chỉ có tiêu đề: %q", rows)
	}
	for _, h := range rows[0] {
		if strings.HasPrefix(h, "Mã") {
			t.Errorf("tệp mẫu không được có cột Mã cán bộ (ADR 0059 điều kiện dừng #4): %q", h)
		}
	}
	choices, _ := f.GetRows(xlsx.SheetChoices)
	joined := ""
	for _, r := range choices {
		joined += strings.Join(r, "|") + "\n"
	}
	if !strings.Contains(joined, "van-phong · VĂN PHÒNG") || !strings.Contains(joined, "chuyen-vien · Chuyên viên") {
		t.Errorf("danh sách chọn: %q", joined)
	}
}

func TestStaffImports_ImportNeedsIdempotencyKey(t *testing.T) {
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)
	body, ct := multipartBody(t, sampleStaffWorkbook(t), mimeXLSX, "can-bo.xlsx")
	r := httptest.NewRequest("POST", "https://"+hostA+pathStaffImports, body)
	r.Host = hostA
	r.RemoteAddr = "10.0.0.7:51000"
	r.Header.Set("Content-Type", ct)
	r.AddCookie(&http.Cookie{Name: CookiePhien, Value: tok})
	if w := m.chay(r); w.Code/100 != 4 {
		t.Errorf("thiếu Idempotency-Key: mã = %d", w.Code)
	}
	if m.staffImports.calls != 0 {
		t.Error("thiếu khoá mà use case vẫn chạy")
	}
}
