package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
	"github.com/xuri/excelize/v2"
)

// THE THREE IMPORT ROUTES of Danh bạ người dân (ADR 0097 D3, D13; chapter 16 §7), all
// `citizen.update`. The four rule-5 cases run in citizen_directory_test.go's table. Asserted here is
// what the HANDLER owns: the template's content (the dropdown lists ACTIVE units only, the example
// row carries the fake number), the upload reaching the use case as cells, the preview answering 200
// with row errors, the rejected import answering 400 with every error, the 201 shape, the race → 409.
// The sheet rules are proved in domain/citizen_directory_import_test.go, the transaction in app/.

const (
	pathCitizenContactImports  = "/api/v1/citizen-contact-imports"
	pathCitizenContactPreviews = "/api/v1/citizen-contact-import-previews"
	pathCitizenContactTemplate = "/api/v1/citizen-contact-import-template"
)

// sampleCitizenContactWorkbook is a two-row file with the template's header and the fake number.
func sampleCitizenContactWorkbook(t *testing.T) []byte {
	t.Helper()
	return residentialUnitWorkbook(t, domain.CitizenContactImportColumns(),
		[]any{"Nguyễn Văn A", citizenPhoneFixture, "thon-binh-an · Thôn Bình An", "Số 1", "Tổ trưởng dân phố", "tổ trưởng", ""},
		[]any{"Trần Thị B", "0900000002", "", "", "", "", "ghi chú"},
	)
}

// The template is an .xlsx, served no-store, whose dropdown holds the commune's ACTIVE units (tt-003 is
// out of use in thonToDanPhoMau) and whose example row carries the agreed fake number.
func TestCitizenContactImportTemplate(t *testing.T) {
	m := serverGrantingCitizen(t)
	w := m.goi(t, "GET", hostA, pathCitizenContactTemplate, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)
	if ct := w.Header().Get("Content-Type"); ct != mimeXLSX {
		t.Errorf("Content-Type = %q", ct)
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, "mau-nhap-danh-ba-nguoi-dan.xlsx") {
		t.Errorf("Content-Disposition = %q", cd)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q", w.Header().Get("Cache-Control"))
	}
	f, err := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatalf("mở tệp mẫu: %v", err)
	}
	defer f.Close()
	rows, err := f.GetRows(domain.CitizenContactImportSheetName)
	if err != nil || len(rows) < 2 {
		t.Fatalf("trang dữ liệu: %v, %d dòng", err, len(rows))
	}
	if strings.Join(rows[0], "|") != strings.Join(domain.CitizenContactImportColumns(), "|") {
		t.Errorf("tiêu đề = %v", rows[0])
	}
	if rows[1][1] != "0900000000" {
		t.Errorf("dòng ví dụ phải mang số giả: %v", rows[1])
	}
	choices, err := f.GetRows("Danh sách chọn")
	if err != nil {
		t.Fatalf("trang danh sách chọn: %v", err)
	}
	var all []string
	for _, r := range choices {
		all = append(all, r...)
	}
	joined := strings.Join(all, "\n")
	if !strings.Contains(joined, "thon-binh-an · Thôn Bình An") || !strings.Contains(joined, "thon-chua-phan-loai · Thôn Chưa Phân Loại") {
		t.Errorf("danh sách chọn thiếu đơn vị đang dùng: %q", joined)
	}
	if strings.Contains(joined, "to-dan-pho-so-1") {
		t.Errorf("đơn vị đã ngưng dùng không được vào danh sách chọn: %q", joined)
	}
	guide, err := f.GetRows("Hướng dẫn")
	if err != nil || !strings.Contains(strings.Join(flattenRows(guide), "\n"), domain.TagSensitiveWarning) {
		t.Errorf("trang hướng dẫn phải mang câu cấm nhãn nhạy cảm (%v): %v", err, guide)
	}
}

func flattenRows(rows [][]string) []string {
	var out []string
	for _, r := range rows {
		out = append(out, r...)
	}
	return out
}

// The upload reaches the use case as the sheet's cells, with the ACTIVE units only; a valid file is
// `valid: true` with the row count.
func TestCitizenContactImportPreview_CellsAndUnitsReachUseCase(t *testing.T) {
	m := serverGrantingCitizen(t)
	w := m.sendWorkbook(t, pathCitizenContactPreviews, hostA, m.tokenCho(t, xaA, sidA), sampleCitizenContactWorkbook(t))
	doiMa(t, w, http.StatusOK)
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q", w.Header().Get("Cache-Control"))
	}
	f := m.citizenDirectory
	if len(f.lastSheet) != 3 || f.lastSheet[0][0] != domain.CitizenContactImportColFullName || f.lastSheet[1][1] != citizenPhoneFixture {
		t.Errorf("ô tới use case: %v", f.lastSheet)
	}
	if len(f.lastUnits) != 2 || f.lastUnits[0].ID != "tt-001" || f.lastUnits[0].Code != "thon-binh-an" || f.lastUnits[1].ID != "tt-002" {
		t.Errorf("đơn vị tới use case (chỉ đang dùng): %+v", f.lastUnits)
	}
	var out citizenContactImportPreviewOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || !out.Valid || out.RowCount != 2 || len(out.Errors) != 0 {
		t.Errorf("xem trước = %+v (%v)", out, err)
	}
	if strings.Contains(w.Body.String(), citizenPhoneFixture) || strings.Contains(w.Body.String(), "Nguyễn Văn A") {
		t.Errorf("xem trước không được chép lại dòng của tệp: %s", w.Body.String())
	}
}

// A file with row errors is STILL 200 — the errors are the answer — and the errors name row and column.
func TestCitizenContactImportPreview_RowErrorsAre200(t *testing.T) {
	m := serverGrantingCitizen(t)
	m.citizenDirectory.preview = app.CitizenContactImportPreview{RowCount: 2, Errors: []domain.CitizenContactImportError{
		{Row: 3, Column: domain.CitizenContactImportColPhone, Message: "Số điện thoại này đã có trong danh bạ của xã."},
	}}
	w := m.sendWorkbook(t, pathCitizenContactPreviews, hostA, m.tokenCho(t, xaA, sidA), sampleCitizenContactWorkbook(t))
	doiMa(t, w, http.StatusOK)
	var out citizenContactImportPreviewOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || out.Valid || out.RowCount != 2 || len(out.Errors) != 1 ||
		out.Errors[0].Row != 3 || out.Errors[0].Column != domain.CitizenContactImportColPhone {
		t.Errorf("xem trước = %+v (%v)", out, err)
	}
}

// A rejected import is 400 `import_invalid` with every error; the 201 carries the batch and the count.
func TestCitizenContactImport_RejectedAndCreated(t *testing.T) {
	m := serverGrantingCitizen(t)
	tok := m.tokenCho(t, xaA, sidA)
	m.citizenDirectory.err = &app.CitizenContactImportRejected{Errors: []domain.CitizenContactImportError{
		{Row: 2, Column: domain.CitizenContactImportColGroups, Message: "Xã chưa lập nhóm mang tên này."},
		{Row: 3, Column: domain.CitizenContactImportColPhone, Message: "Số điện thoại này đã có trong danh bạ của xã."},
	}}
	w := m.sendWorkbook(t, pathCitizenContactImports, hostA, tok, sampleCitizenContactWorkbook(t))
	doiMa(t, w, http.StatusBadRequest)
	var rej citizenContactImportRejectedOut
	if err := json.Unmarshal(w.Body.Bytes(), &rej); err != nil || rej.Code != "import_invalid" || len(rej.Errors) != 2 || rej.Errors[1].Row != 3 {
		t.Errorf("từ chối = %+v (%v)", rej, err)
	}

	m.citizenDirectory.err = nil
	w = m.sendWorkbook(t, pathCitizenContactImports, hostA, tok, sampleCitizenContactWorkbook(t))
	doiMa(t, w, http.StatusCreated)
	var out citizenContactImportCreatedOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || out.BatchID != citizenBatchIDFixture || out.RowCount != 2 {
		t.Errorf("201 = %+v (%v)", out, err)
	}
	if a := m.citizenDirectory.lastActor; a.Vet.ID != maCanBo || a.ID != idNoiBo {
		t.Errorf("người nhập = %+v", a)
	}
}

// The commune changing under the plan is 409; anything else is 500. Neither echoes the file.
func TestCitizenContactImport_FailuresMapToStatus(t *testing.T) {
	for _, c := range []struct {
		err    error
		status int
		code   string
	}{
		{idstore.ErrCitizenPhoneTaken, http.StatusConflict, "citizen_contacts_changed"},
		{idstore.ErrCitizenGroupNotFound, http.StatusConflict, "citizen_contacts_changed"},
		{idstore.ErrCitizenContactUnitUnknown, http.StatusConflict, "citizen_contacts_changed"},
		{errors.New("db down"), http.StatusInternalServerError, "internal"},
	} {
		m := serverGrantingCitizen(t)
		m.citizenDirectory.err = c.err
		w := m.sendWorkbook(t, pathCitizenContactImports, hostA, m.tokenCho(t, xaA, sidA), sampleCitizenContactWorkbook(t))
		if w.Code != c.status || errorCode(t, w.Body.Bytes()) != c.code || strings.Contains(w.Body.String(), citizenPhoneFixture) {
			t.Errorf("%v: mã = %d %s, muốn %d %s", c.err, w.Code, w.Body.String(), c.status, c.code)
		}
	}
}

// The import without an Idempotency-Key is refused before the file is read.
func TestCitizenContactImport_NeedsIdempotencyKey(t *testing.T) {
	m := serverGrantingCitizen(t)
	body, ct := multipartBody(t, sampleCitizenContactWorkbook(t), mimeXLSX, "danh-ba.xlsx")
	r := httptest.NewRequest("POST", "https://"+hostA+pathCitizenContactImports, body)
	r.Host = hostA
	r.RemoteAddr = "10.0.0.7:51000"
	r.Header.Set("Content-Type", ct)
	r.AddCookie(&http.Cookie{Name: CookiePhien, Value: m.tokenCho(t, xaA, sidA)})
	w := m.chay(r)
	if w.Code < 400 || m.citizenDirectory.count() != 0 {
		t.Errorf("thiếu Idempotency-Key: mã = %d, use case %d lần", w.Code, m.citizenDirectory.count())
	}
}

// A non-xlsx part is 415 before the use case.
func TestCitizenContactImport_NotXLSXIs415(t *testing.T) {
	m := serverGrantingCitizen(t)
	body, ct := multipartBody(t, []byte("a,b,c"), "text/csv", "danh-ba.csv")
	w := m.sendUpload(t, pathCitizenContactPreviews, hostA, m.tokenCho(t, xaA, sidA), body, ct)
	if w.Code != http.StatusUnsupportedMediaType || m.citizenDirectory.count() != 0 {
		t.Errorf("csv: mã = %d, use case %d lần — %s", w.Code, m.citizenDirectory.count(), w.Body.String())
	}
}
