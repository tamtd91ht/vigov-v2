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
	"github.com/vihat/vigov/core/xlsx"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
	"github.com/xuri/excelize/v2"
)

// THE FIVE WRITE / IMPORT ROUTES OF THE RESIDENTIAL UNITS, all `admin.org`. What is asserted here is
// what the HANDLER owns: the permission, the wire mapping, the upload reader, the template, and which
// identifier reaches the trail. The rules are proved in domain/residential_unit_import_test.go, the
// transaction and "deactivate keeps references" in app/residential_unit_test.go.

const pathResidentialUnits = "/api/v1/residential-units"

// residentialUnitsFake stands in for *app.ResidentialUnits.
type residentialUnitsFake struct {
	calls      int
	lastActor  app.NguoiThucHien
	lastXa     tenant.ID
	lastID     string
	lastCreate app.CreateResidentialUnit
	lastUpdate app.UpdateResidentialUnit

	res domain.ThonToDanPho
	err error
}

func residentialUnitsSample() *residentialUnitsFake {
	n := 284
	return &residentialUnitsFake{res: domain.ThonToDanPho{
		ID: "tt-new", Ma: "thon-hoa-binh", Ten: "Thôn Hoà Bình", LoaiMa: "thon", LoaiNhan: "Thôn",
		SoHo: &n, DangDung: true, HeadStaffID: "nd-internal", HeadStaffCode: "CB-2026-AAAAAA", HeadStaffName: "Cán Bộ Một", SortOrder: 2,
	}}
}

func (f *residentialUnitsFake) Create(ctx context.Context, req app.CreateResidentialUnit, actor app.NguoiThucHien) (domain.ThonToDanPho, error) {
	f.calls++
	f.lastCreate, f.lastActor, f.lastXa = req, actor, tenant.MustFrom(ctx)
	return f.res, f.err
}

func (f *residentialUnitsFake) Update(ctx context.Context, id string, req app.UpdateResidentialUnit, actor app.NguoiThucHien) (domain.ThonToDanPho, error) {
	f.calls++
	f.lastID, f.lastUpdate, f.lastActor, f.lastXa = id, req, actor, tenant.MustFrom(ctx)
	return f.res, f.err
}

// residentialUnitImportsFake stands in for *app.ResidentialUnitImporter.
type residentialUnitImportsFake struct {
	calls     int
	lastRows  []domain.ResidentialUnitImportRow
	lastActor app.NguoiThucHien
	lastXa    tenant.ID

	types []domain.ResidentialUnitTypeChoice
	staff []domain.HeadStaffChoice
	res   app.ResidentialUnitImportResult
	err   error
}

func residentialUnitImportsSample() *residentialUnitImportsFake {
	return &residentialUnitImportsFake{
		types: []domain.ResidentialUnitTypeChoice{{Code: "thon", Label: "Thôn"}, {Code: "to-dan-pho", Label: "Tổ dân phố"}},
		staff: []domain.HeadStaffChoice{{ID: "nd-1", Code: "CB-2026-AAAAAA", Name: "Cán Bộ Một"}},
		res: app.ResidentialUnitImportResult{Units: []domain.PlannedResidentialUnit{
			{Row: 2, Unit: domain.ThonToDanPho{ID: "tt-1", Ma: "thon-hoa-binh", Ten: "Thôn Hoà Bình", DangDung: true}},
		}},
	}
}

func (f *residentialUnitImportsFake) TemplateChoices(ctx context.Context) ([]domain.ResidentialUnitTypeChoice, []domain.HeadStaffChoice, error) {
	f.calls++
	f.lastXa = tenant.MustFrom(ctx)
	return f.types, f.staff, f.err
}

func (f *residentialUnitImportsFake) Preview(ctx context.Context, rows []domain.ResidentialUnitImportRow) (app.ResidentialUnitImportResult, error) {
	f.calls++
	f.lastRows, f.lastXa = rows, tenant.MustFrom(ctx)
	return f.res, f.err
}

func (f *residentialUnitImportsFake) Import(ctx context.Context, rows []domain.ResidentialUnitImportRow, actor app.NguoiThucHien) (app.ResidentialUnitImportResult, error) {
	f.calls++
	f.lastRows, f.lastActor, f.lastXa = rows, actor, tenant.MustFrom(ctx)
	return f.res, f.err
}

// residentialUnitWorkbook is a valid two-row file with the template's header.
func residentialUnitWorkbook(t *testing.T, header []string, rows ...[]any) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	h := make([]any, 0, len(header))
	for _, c := range header {
		h = append(h, c)
	}
	if err := f.SetSheetRow("Sheet1", "A1", &h); err != nil {
		t.Fatalf("SetSheetRow: %v", err)
	}
	for i, r := range rows {
		r := r
		cell, _ := excelize.CoordinatesToCellName(1, i+2)
		if err := f.SetSheetRow("Sheet1", cell, &r); err != nil {
			t.Fatalf("SetSheetRow: %v", err)
		}
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatalf("WriteToBuffer: %v", err)
	}
	return buf.Bytes()
}

func sampleResidentialUnitWorkbook(t *testing.T) []byte {
	return residentialUnitWorkbook(t, domain.ResidentialUnitImportColumns(),
		[]any{"Thôn Hoà Bình", "Thôn", "CB-2026-AAAAAA · Cán Bộ Một", 284, "", "", 2},
		[]any{"Tổ dân phố 1", "", "", "", 1132, "tdp-1", ""},
	)
}

type residentialUnitRoute struct {
	name, method, path, body string
	ok                       int
}

func residentialUnitRoutes() []residentialUnitRoute {
	return []residentialUnitRoute{
		{"thêm", "POST", pathResidentialUnits, `{"name":"Thôn Hoà Bình","type_code":"thon"}`, http.StatusCreated},
		{"sửa / ngưng dùng", "PATCH", pathResidentialUnits + "/tt-1", `{"active":false}`, http.StatusOK},
		{"tải mẫu", "GET", pathResidentialUnits + "/import-template", "", http.StatusOK},
		{"xem trước", "POST", pathResidentialUnits + "/import-previews", "", http.StatusOK},
		{"nhập", "POST", pathResidentialUnits + "/imports", "", http.StatusCreated},
	}
}

func (m *mayChu) callResidentialUnitRoute(t *testing.T, rt residentialUnitRoute, host, tok string) *httptest.ResponseRecorder {
	t.Helper()
	switch {
	case rt.method == "GET":
		return m.goi(t, "GET", host, rt.path, "", tok)
	case strings.HasSuffix(rt.path, "import-previews") || strings.HasSuffix(rt.path, "/imports"):
		return m.sendWorkbook(t, rt.path, host, tok, sampleResidentialUnitWorkbook(t))
	default:
		return m.goiIdem(t, rt.method, host, rt.path, rt.body, tok)
	}
}

func (m *mayChu) residentialUnitCalls() int {
	return m.residentialUnits.calls + m.residentialUnitImports.calls
}

// --- rule 5, invariant 7: the four cases, for each of the five routes -----------------------------

func TestResidentialUnits_401WithoutToken(t *testing.T) {
	m := dungMayChuSoDo(t)
	for _, rt := range residentialUnitRoutes() {
		if w := m.callResidentialUnitRoute(t, rt, hostA, ""); w.Code != http.StatusUnauthorized {
			t.Errorf("%s: mã = %d, muốn 401 — %s", rt.name, w.Code, w.Body.String())
		}
	}
	if n := m.residentialUnitCalls(); n != 0 {
		t.Errorf("chưa đăng nhập mà use case đã chạy %d lần", n)
	}
}

// 403 WITH `admin.user` — a real key, not `admin.org`.
func TestResidentialUnits_403WrongPermission(t *testing.T) {
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)
	for _, rt := range residentialUnitRoutes() {
		if w := m.callResidentialUnitRoute(t, rt, hostA, tok); w.Code != http.StatusForbidden {
			t.Errorf("%s: mã = %d, muốn 403 — %s", rt.name, w.Code, w.Body.String())
		}
	}
	if n := m.residentialUnitCalls(); n != 0 {
		t.Errorf("sai quyền mà use case đã chạy %d lần", n)
	}
}

// 403 WITH `admin.org` HELD IN COMMUNE A, signed in properly at commune B (rule 5, invariant 3).
func TestResidentialUnits_403RightPermissionWrongCommune(t *testing.T) {
	m := dungMayChuSoDo(t)
	tok := m.tokenCho(t, xaB, sidB)
	for _, rt := range residentialUnitRoutes() {
		if w := m.callResidentialUnitRoute(t, rt, hostB, tok); w.Code != http.StatusForbidden {
			t.Errorf("%s: mã = %d, muốn 403 — %s", rt.name, w.Code, w.Body.String())
		}
	}
	if n := m.residentialUnitCalls(); n != 0 {
		t.Errorf("sai xã mà use case đã chạy %d lần", n)
	}
}

// 2xx, THE COMMUNE OF THE HOST REACHES THE USE CASE, AND THE TRAIL GETS THE STAFF CODE.
func TestResidentialUnits_2xxCommuneAndStaffCode(t *testing.T) {
	m := dungMayChuSoDo(t)
	tok := m.tokenCho(t, xaA, sidA)
	for _, rt := range residentialUnitRoutes() {
		w := m.callResidentialUnitRoute(t, rt, hostA, tok)
		if w.Code != rt.ok {
			t.Fatalf("%s: mã = %d, muốn %d — %s", rt.name, w.Code, rt.ok, w.Body.String())
		}
	}
	if m.residentialUnits.lastXa != xaA || m.residentialUnitImports.lastXa != xaA {
		t.Errorf("use case chạy ở xã %q / %q, muốn %q", m.residentialUnits.lastXa, m.residentialUnitImports.lastXa, xaA)
	}
	for _, a := range []app.NguoiThucHien{m.residentialUnits.lastActor, m.residentialUnitImports.lastActor} {
		if a.Vet.ID != maCanBo || a.ID != idNoiBo {
			t.Errorf("Vet.ID=%q ID=%q — vết phải mang MÃ CÁN BỘ %q", a.Vet.ID, a.ID, maCanBo)
		}
	}
	if m.residentialUnits.lastID != "tt-1" || m.residentialUnits.lastUpdate.Active == nil || *m.residentialUnits.lastUpdate.Active {
		t.Errorf("PATCH active:false phải tới use case: %+v", m.residentialUnits.lastUpdate)
	}
}

// --- the write wire -------------------------------------------------------------------------------

func TestResidentialUnits_CreateWire(t *testing.T) {
	m := dungMayChuSoDo(t)
	tok := m.tokenCho(t, xaA, sidA)
	w := m.goiIdem(t, "POST", hostA, pathResidentialUnits,
		`{"name":"Thôn Hoà Bình","code":"thon-hb","type_code":"thon","head_staff_code":"CB-2026-AAAAAA","household_count":0,"order":2}`, tok)
	doiMa(t, w, http.StatusCreated)
	c := m.residentialUnits.lastCreate
	if c.Name != "Thôn Hoà Bình" || c.Code != "thon-hb" || c.TypeCode != "thon" || c.HeadCode != "CB-2026-AAAAAA" ||
		c.Households == nil || *c.Households != 0 || c.Population != nil || c.Order != 2 {
		t.Errorf("yêu cầu tới use case: %+v", c)
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("thân: %v", err)
	}
	for k, want := range map[string]string{
		"code": `"thon-hoa-binh"`, "head_staff_code": `"CB-2026-AAAAAA"`, "head_staff_name": `"Cán Bộ Một"`,
		"order": `2`, "active": `true`, "household_count": `284`, "population_count": `null`, "type_label": `"Thôn"`,
	} {
		if string(out[k]) != want {
			t.Errorf("%s = %s, muốn %s", k, out[k], want)
		}
	}
	if strings.Contains(w.Body.String(), "nd-internal") {
		t.Errorf("id nội bộ của cán bộ lọt ra ngoài: %s", w.Body.String())
	}

	w = m.goiIdem(t, "POST", hostA, pathResidentialUnits, `{"name":"Thôn X","active":false}`, tok)
	if got := loiTra(t, w).Code; w.Code != http.StatusBadRequest || got != "active_not_settable" {
		t.Errorf("active khi tạo: %d %s", w.Code, got)
	}
}

func TestResidentialUnits_UpdateWire(t *testing.T) {
	m := dungMayChuSoDo(t)
	tok := m.tokenCho(t, xaA, sidA)
	path := pathResidentialUnits + "/tt-1"

	doiMa(t, m.goi(t, "PATCH", hostA, path, `{"household_count":null,"population_count":1132,"head_staff_code":""}`, tok), http.StatusOK)
	u := m.residentialUnits.lastUpdate
	if !u.Households.Set || u.Households.Value != nil || !u.Population.Set || *u.Population.Value != 1132 ||
		u.HeadCode == nil || *u.HeadCode != "" || u.Name != nil || u.Active != nil || u.TypeCode != nil {
		t.Errorf("null xoá, số đặt, vắng giữ nguyên: %+v", u)
	}
	doiMa(t, m.goi(t, "PATCH", hostA, path, `{"name":"Thôn Mới"}`, tok), http.StatusOK)
	if u := m.residentialUnits.lastUpdate; u.Households.Set || u.Population.Set {
		t.Errorf("số liệu vắng phải là KHÔNG ĐỔI: %+v", u)
	}

	calls := m.residentialUnits.calls
	for body, code := range map[string]string{
		`{"code":"thon-khac"}`:     "code_not_editable",
		`{}`:                       "invalid_request",
		`{"household_count":"12"}`: "invalid_request",
		`not json`:                 "invalid_request",
	} {
		w := m.goi(t, "PATCH", hostA, path, body, tok)
		if got := loiTra(t, w).Code; w.Code != http.StatusBadRequest || got != code {
			t.Errorf("%s: %d %s, muốn 400 %s", body, w.Code, got, code)
		}
	}
	if m.residentialUnits.calls != calls {
		t.Error("thân bị từ chối mà use case vẫn chạy")
	}
}

func TestResidentialUnits_WriteStatusMapping(t *testing.T) {
	for _, c := range []struct {
		err  error
		code int
		id   string
	}{
		{idstore.ErrResidentialUnitNotFound, http.StatusNotFound, "residential_unit_not_found"},
		{idstore.ErrResidentialUnitCodeTaken, http.StatusConflict, "residential_unit_code_taken"},
		{idstore.ErrResidentialUnitNameTaken, http.StatusConflict, "residential_unit_name_taken"},
		{app.ErrResidentialUnitListFull, http.StatusConflict, "residential_unit_list_full"},
		{idstore.ErrResidentialUnitTypeNotFound, http.StatusBadRequest, "residential_unit_type_not_found"},
		{idstore.ErrHeadStaffNotFound, http.StatusBadRequest, "head_staff_not_found"},
		{domain.ErrResidentialUnitCountRange, http.StatusBadRequest, "invalid_request"},
		{errors.New("cơ sở dữ liệu không phản hồi"), http.StatusInternalServerError, "internal"},
	} {
		m := dungMayChuSoDo(t)
		m.residentialUnits.err = c.err
		w := m.goi(t, "PATCH", hostA, pathResidentialUnits+"/tt-1", `{"active":false}`, m.tokenCho(t, xaA, sidA))
		if got := loiTra(t, w).Code; w.Code != c.code || got != c.id {
			t.Errorf("%v: %d (%s), muốn %d (%s)", c.err, w.Code, got, c.code, c.id)
		}
		if strings.Contains(w.Body.String(), "không phản hồi") {
			t.Errorf("lỗi nội bộ lọt ra ngoài: %s", w.Body.String())
		}
	}
}

// THE READ carries the three new fields, `order` and the head's code and name.
func TestResidentialUnits_ListCarriesHeadAndOrder(t *testing.T) {
	m := dungMayChu(t)
	w := m.goi(t, "GET", hostA, pathResidentialUnits, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)
	for _, f := range []string{`"head_staff_code":`, `"head_staff_name":`, `"order":`} {
		if !strings.Contains(w.Body.String(), f) {
			t.Errorf("thiếu %s trong %s", f, w.Body.String())
		}
	}
}

// --- the template ---------------------------------------------------------------------------------

// THE TEMPLATE ROUND TRIP: download, fill one row with the dropdown values, upload to the preview —
// the rows reaching the use case are what the person picked, through the real core/xlsx reader.
func TestResidentialUnits_TemplateRoundTrip(t *testing.T) {
	m := dungMayChuSoDo(t)
	tok := m.tokenCho(t, xaA, sidA)
	w := m.goi(t, "GET", hostA, pathResidentialUnits+"/import-template", "", tok)
	doiMa(t, w, http.StatusOK)
	if ct := w.Header().Get("Content-Type"); ct != mimeXLSX {
		t.Errorf("Content-Type = %q", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q", cc)
	}

	f, err := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatalf("tệp mẫu không mở được: %v", err)
	}
	defer f.Close()
	if sheets := f.GetSheetList(); sheets[0] != residentialUnitSheet {
		t.Errorf("trang đầu = %q", sheets[0])
	}
	rows, _ := f.GetRows(residentialUnitSheet)
	if len(rows) != 1 || strings.Join(rows[0], "|") != strings.Join(domain.ResidentialUnitImportColumns(), "|") {
		t.Fatalf("trang dữ liệu phải chỉ có dòng tiêu đề (không dòng ví dụ nào bị nhập nhầm): %q", rows)
	}
	choices, _ := f.GetRows(xlsx.SheetChoices)
	joined := ""
	for _, r := range choices {
		joined += strings.Join(r, "|") + "\n"
	}
	if !strings.Contains(joined, "Tổ dân phố") || !strings.Contains(joined, "CB-2026-AAAAAA · Cán Bộ Một") {
		t.Errorf("danh sách chọn thiếu loại hoặc cán bộ: %q", joined)
	}
	if v, _ := f.GetSheetVisible(xlsx.SheetChoices); v {
		t.Error("trang danh sách chọn phải ẩn")
	}

	// Fill row 2 with dropdown values and upload the SAME workbook.
	for i, v := range []string{"Thôn Hoà Bình", "Tổ dân phố", "CB-2026-AAAAAA · Cán Bộ Một", "284", "", "", "3"} {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		if err := f.SetCellStr(residentialUnitSheet, cell, v); err != nil {
			t.Fatalf("ghi ô: %v", err)
		}
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatalf("WriteToBuffer: %v", err)
	}
	doiMa(t, m.sendWorkbook(t, pathResidentialUnits+"/import-previews", hostA, tok, buf.Bytes()), http.StatusOK)
	got := m.residentialUnitImports.lastRows
	if len(got) != 1 || got[0].Row != 2 || got[0].Name != "Thôn Hoà Bình" || got[0].Type != "Tổ dân phố" ||
		got[0].Head != "CB-2026-AAAAAA · Cán Bộ Một" || got[0].Households != "284" || got[0].Order != "3" {
		t.Errorf("dòng tới use case: %+v", got)
	}
}

// --- the upload -----------------------------------------------------------------------------------

// FILE ERRORS NEVER ECHO CELL TEXT: a wrong header full of personal data answers with a fixed sentence.
func TestResidentialUnits_FileErrorsNeverEchoCells(t *testing.T) {
	const secret = "0900000000 Nguyễn Văn Bí Mật"
	m := dungMayChuSoDo(t)
	tok := m.tokenCho(t, xaA, sidA)
	data := residentialUnitWorkbook(t, []string{secret, "Số điện thoại"}, []any{secret})
	for _, c := range []struct {
		path string
		code int
	}{{pathResidentialUnits + "/import-previews", http.StatusOK}, {pathResidentialUnits + "/imports", http.StatusBadRequest}} {
		w := m.sendWorkbook(t, c.path, hostA, tok, data)
		if w.Code != c.code || !strings.Contains(w.Body.String(), `"row":1`) {
			t.Errorf("%s: %d %s", c.path, w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "0900000000") || strings.Contains(w.Body.String(), "Bí Mật") {
			t.Errorf("%s: phản hồi lặp lại nội dung ô: %s", c.path, w.Body.String())
		}
	}
	if m.residentialUnitImports.calls != 0 {
		t.Error("tiêu đề sai mà use case vẫn chạy")
	}
}

func TestResidentialUnits_UploadRefusals(t *testing.T) {
	m := dungMayChuSoDo(t)
	tok := m.tokenCho(t, xaA, sidA)
	for _, c := range []struct {
		name string
		data []byte
		code int
		id   string
	}{
		{"không phải xlsx", []byte("Tên thôn\nA\n"), http.StatusUnsupportedMediaType, "unsupported_file_type"},
		{"tệp quá lớn", bytes.Repeat([]byte("x"), int(xlsx.DefaultLimits.MaxFileBytes)+1), http.StatusRequestEntityTooLarge, "file_too_large"},
	} {
		w := m.sendWorkbook(t, pathResidentialUnits+"/imports", hostA, tok, c.data)
		if got := loiTra(t, w).Code; w.Code != c.code || got != c.id {
			t.Errorf("%s: %d (%s), muốn %d (%s)", c.name, w.Code, got, c.code, c.id)
		}
	}
	if m.residentialUnitImports.calls != 0 {
		t.Error("tệp bị từ chối mà use case vẫn chạy")
	}
}

func TestResidentialUnits_ImportStatusMapping(t *testing.T) {
	m := dungMayChuSoDo(t)
	tok := m.tokenCho(t, xaA, sidA)
	m.residentialUnitImports.err = &app.ResidentialUnitImportRejected{Errors: []domain.ResidentialUnitImportError{
		{Row: 3, Column: domain.ResidentialUnitImportColHouseholds, Message: "Phải là số nguyên"},
	}}
	w := m.sendWorkbook(t, pathResidentialUnits+"/imports", hostA, tok, sampleResidentialUnitWorkbook(t))
	doiMa(t, w, http.StatusBadRequest)
	var out residentialUnitImportRejectedOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || out.Code != "import_invalid" || len(out.Errors) != 1 || out.Errors[0].Row != 3 {
		t.Errorf("phản hồi: %+v (%v)", out, err)
	}

	for _, c := range []struct {
		err  error
		code int
		id   string
	}{
		{idstore.ErrResidentialUnitCodeTaken, http.StatusConflict, "residential_units_changed"},
		{idstore.ErrHeadStaffNotFound, http.StatusConflict, "residential_units_changed"},
		{errors.New("cơ sở dữ liệu không phản hồi"), http.StatusInternalServerError, "internal"},
	} {
		m.residentialUnitImports.err = c.err
		w := m.sendWorkbook(t, pathResidentialUnits+"/imports", hostA, tok, sampleResidentialUnitWorkbook(t))
		if got := loiTra(t, w).Code; w.Code != c.code || got != c.id {
			t.Errorf("%v: %d (%s), muốn %d (%s)", c.err, w.Code, got, c.code, c.id)
		}
	}

	m.residentialUnitImports.err = nil
	m.residentialUnitImports.res = app.ResidentialUnitImportResult{Errors: []domain.ResidentialUnitImportError{{Row: 2, Message: "x"}}}
	w = m.sendWorkbook(t, pathResidentialUnits+"/import-previews", hostA, tok, sampleResidentialUnitWorkbook(t))
	doiMa(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"valid":false`) || !strings.Contains(w.Body.String(), `"units":[]`) {
		t.Errorf("xem trước tệp lỗi: %s", w.Body.String())
	}
}

// THE IMPORT REQUIRES AN Idempotency-Key; the preview, which writes nothing, does not.
func TestResidentialUnits_ImportNeedsIdempotencyKey(t *testing.T) {
	m := dungMayChuSoDo(t)
	tok := m.tokenCho(t, xaA, sidA)
	for _, c := range []struct {
		path string
		ok   bool
	}{{pathResidentialUnits + "/imports", false}, {pathResidentialUnits + "/import-previews", true}} {
		body, ct := multipartBody(t, sampleResidentialUnitWorkbook(t), mimeXLSX, "thon.xlsx")
		r := httptest.NewRequest("POST", "https://"+hostA+c.path, body)
		r.Host = hostA
		r.RemoteAddr = "10.0.0.7:51000"
		r.Header.Set("Content-Type", ct)
		r.AddCookie(&http.Cookie{Name: CookiePhien, Value: tok})
		w := m.chay(r)
		if c.ok != (w.Code == http.StatusOK) || (!c.ok && w.Code/100 != 4) {
			t.Errorf("%s không khoá: mã = %d", c.path, w.Code)
		}
	}
	if m.residentialUnitImports.calls != 1 {
		t.Errorf("use case chạy %d lần, muốn 1 (chỉ xem trước)", m.residentialUnitImports.calls)
	}
}
