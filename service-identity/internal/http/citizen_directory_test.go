package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The sixteen routes of Danh bạ người dân (ADR 0097). Rule 5, invariant 7 asks four cases of each —
// 401 no token · 403 wrong permission (the harness default `admin.user`) · 403 right permission in the
// wrong commune · 2xx both correct — plus the one this screen adds: `citizen.read` does not open a write
// route and `citizen.update` does not open a read route. Then what the HANDLER owns: body and query
// mapping, the mask on the wire, which identifier reaches the trail, and the status each refusal
// becomes. The rules themselves are proved in domain/ and app/.

// --- fake ----------------------------------------------------------------------------------------

const (
	citizenContactIDFixture = "01JCTZNCNTCT00000000000000"
	citizenGroupIDFixture   = "01JCTZNGRP0000000000000000"
	citizenBatchIDFixture   = "01JCTZNBTCH000000000000000"
	// The agreed fake number (rule 3, invariant 5) and its mask.
	citizenPhoneFixture       = "0900000001"
	citizenPhoneMaskedFixture = "09****0001"
	citizenEmailFixture       = "ten.nguoi@example.vn"
)

// citizenDirectoryFake stands in for *app.CitizenDirectory. One `err` for every method: a test sets
// it and calls the route it is about.
type citizenDirectoryFake struct {
	mu sync.Mutex

	calls       int
	lastActor   app.NguoiThucHien
	lastCommune tenant.ID
	lastID      string
	lastFilter  idstore.CitizenContactFilter
	lastSearch  app.CitizenContactSearch
	lastPage    page.Request
	lastContact app.CitizenContactRequest
	lastGroup   domain.CitizenGroupInput
	lastReason  string
	lastSheet   [][]string
	lastUnits   []app.ImportResidentialUnit

	preview app.CitizenContactImportPreview
	err     error
}

func citizenDirectorySample() *citizenDirectoryFake {
	return &citizenDirectoryFake{preview: app.CitizenContactImportPreview{RowCount: 2}}
}

func (f *citizenDirectoryFake) note(ctx context.Context) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.lastCommune = tenant.MustFrom(ctx) // reached with a commune in context, or this panics
}

func (f *citizenDirectoryFake) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func (f *citizenDirectoryFake) List(ctx context.Context, fl idstore.CitizenContactFilter, req page.Request) (page.Result[app.CitizenContactListItem], error) {
	f.note(ctx)
	f.lastFilter, f.lastPage = fl, req
	if f.err != nil {
		return page.NewResult[app.CitizenContactListItem](), f.err
	}
	return page.Result[app.CitizenContactListItem]{Items: []app.CitizenContactListItem{{
		ID: citizenContactIDFixture, FullName: "Nguyễn Văn A", PhoneMasked: citizenPhoneMaskedFixture,
		ResidentialUnitID: "tt-001", ResidentialUnitName: "Thôn Bình An", Tags: []string{"tổ trưởng"},
		GroupIDs: []string{}, IsActive: true, Source: domain.CitizenContactSourceManual, HasOwnAppSession: true,
		CreatedAt: time.Date(2026, 10, 16, 8, 0, 0, 0, time.UTC),
	}}}, nil
}

func (f *citizenDirectoryFake) Count(ctx context.Context, fl idstore.CitizenContactFilter) (int, error) {
	f.note(ctx)
	f.lastFilter = fl
	return 57, f.err
}

func (f *citizenDirectoryFake) Search(ctx context.Context, q app.CitizenContactSearch, req page.Request) (page.Result[app.CitizenContactListItem], error) {
	f.mu.Lock()
	f.lastSearch = q
	f.mu.Unlock()
	return f.List(ctx, q.Filter, req)
}

func (f *citizenDirectoryFake) CountSearch(ctx context.Context, q app.CitizenContactSearch) (int, error) {
	f.mu.Lock()
	f.lastSearch = q
	f.mu.Unlock()
	return f.Count(ctx, q.Filter)
}

func (f *citizenDirectoryFake) RevealEmail(ctx context.Context, actor app.NguoiThucHien, id string) (string, error) {
	f.note(ctx)
	f.lastActor, f.lastID = actor, id
	if f.err != nil {
		return "", f.err
	}
	return citizenEmailFixture, nil
}

func (f *citizenDirectoryFake) TagCounts(ctx context.Context) ([]idstore.TagCount, error) {
	f.note(ctx)
	return []idstore.TagCount{{Tag: "hộ kinh doanh", Count: 12}, {Tag: "tổ trưởng", Count: 8}}, f.err
}

func (f *citizenDirectoryFake) Get(ctx context.Context, id string) (app.CitizenContactDetail, error) {
	f.note(ctx)
	f.lastID = id
	if f.err != nil {
		return app.CitizenContactDetail{}, f.err
	}
	return app.CitizenContactDetail{
		ID: id, FullName: "Nguyễn Văn A", PhoneMasked: citizenPhoneMaskedFixture, Address: "Số 1, đường Lê Lợi",
		Note: "ghi chú", Tags: []string{}, GroupIDs: []string{citizenGroupIDFixture}, IsActive: true,
		Source: domain.CitizenContactSourceExcel, ImportBatchID: citizenBatchIDFixture,
	}, nil
}

func (f *citizenDirectoryFake) RevealPhone(ctx context.Context, actor app.NguoiThucHien, id string) (string, error) {
	f.note(ctx)
	f.lastActor, f.lastID = actor, id
	if f.err != nil {
		return "", f.err
	}
	return citizenPhoneFixture, nil
}

func (f *citizenDirectoryFake) Create(ctx context.Context, actor app.NguoiThucHien, req app.CitizenContactRequest) (string, error) {
	f.note(ctx)
	f.lastActor, f.lastContact = actor, req
	return citizenContactIDFixture, f.err
}

func (f *citizenDirectoryFake) Update(ctx context.Context, actor app.NguoiThucHien, id string, req app.CitizenContactRequest) error {
	f.note(ctx)
	f.lastActor, f.lastID, f.lastContact = actor, id, req
	return f.err
}

func (f *citizenDirectoryFake) Remove(ctx context.Context, actor app.NguoiThucHien, id, reason string) error {
	f.note(ctx)
	f.lastActor, f.lastID, f.lastReason = actor, id, reason
	return f.err
}

func (f *citizenDirectoryFake) ListGroups(ctx context.Context) ([]app.CitizenGroupItem, error) {
	f.note(ctx)
	return []app.CitizenGroupItem{{CitizenGroup: domain.CitizenGroup{ID: citizenGroupIDFixture, Name: "Tổ trưởng dân phố"}, MemberCount: 8}}, f.err
}

func (f *citizenDirectoryFake) CreateGroup(ctx context.Context, actor app.NguoiThucHien, in domain.CitizenGroupInput) (string, error) {
	f.note(ctx)
	f.lastActor, f.lastGroup = actor, in
	return citizenGroupIDFixture, f.err
}

func (f *citizenDirectoryFake) UpdateGroup(ctx context.Context, actor app.NguoiThucHien, id string, in domain.CitizenGroupInput) error {
	f.note(ctx)
	f.lastActor, f.lastID, f.lastGroup = actor, id, in
	return f.err
}

func (f *citizenDirectoryFake) RemoveGroup(ctx context.Context, actor app.NguoiThucHien, id, reason string) error {
	f.note(ctx)
	f.lastActor, f.lastID, f.lastReason = actor, id, reason
	return f.err
}

func (f *citizenDirectoryFake) ImportPreview(ctx context.Context, sheet [][]string, units []app.ImportResidentialUnit) (app.CitizenContactImportPreview, error) {
	f.note(ctx)
	f.lastSheet, f.lastUnits = sheet, units
	return f.preview, f.err
}

func (f *citizenDirectoryFake) ImportCommit(ctx context.Context, actor app.NguoiThucHien, sheet [][]string, units []app.ImportResidentialUnit) (app.CitizenContactImportResult, error) {
	f.note(ctx)
	f.lastActor, f.lastSheet, f.lastUnits = actor, sheet, units
	if f.err != nil {
		return app.CitizenContactImportResult{}, f.err
	}
	return app.CitizenContactImportResult{Batch: citizenBatchIDFixture, RowCount: len(sheet) - 1}, nil
}

// --- harness -------------------------------------------------------------------------------------

const (
	citizenReadKey   = authz.Perm("citizen.read")
	citizenUpdateKey = authz.Perm("citizen.update")

	createCitizenContactBody = `{"full_name":"Nguyễn Văn A","phone":"` + citizenPhoneFixture + `","residential_unit_id":"","address":"Số 1","note":"","tags":["tổ trưởng"],"group_ids":["` + citizenGroupIDFixture + `"]}`
	createCitizenGroupBody   = `{"name":"Tổ trưởng dân phố","description":"","sort_order":1}`
)

// citizenRoute is one route with a body that satisfies it. `upload` marks the two multipart routes.
type citizenRoute struct {
	name, method, path, body string
	key                      authz.Perm
	upload                   bool
	ok                       int
}

func citizenRoutes() []citizenRoute {
	return []citizenRoute{
		{name: "danh sách", method: "GET", path: "/api/v1/citizen-contacts?limit=20", key: citizenReadKey, ok: http.StatusOK},
		{name: "tìm", method: "POST", path: "/api/v1/citizen-contact-searches", body: `{"q":"0900"}`, key: citizenReadKey, ok: http.StatusOK},
		{name: "đếm", method: "POST", path: "/api/v1/citizen-contact-count-queries", body: `{"q":""}`, key: citizenReadKey, ok: http.StatusOK},
		{name: "nhãn", method: "GET", path: "/api/v1/citizen-contact-tags", key: citizenReadKey, ok: http.StatusOK},
		{name: "thêm", method: "POST", path: "/api/v1/citizen-contacts", body: createCitizenContactBody, key: citizenUpdateKey, ok: http.StatusCreated},
		{name: "chi tiết", method: "GET", path: "/api/v1/citizen-contacts/" + citizenContactIDFixture, key: citizenReadKey, ok: http.StatusOK},
		{name: "sửa", method: "PATCH", path: "/api/v1/citizen-contacts/" + citizenContactIDFixture, body: createCitizenContactBody, key: citizenUpdateKey, ok: http.StatusNoContent},
		{name: "gỡ", method: "POST", path: "/api/v1/citizen-contacts/" + citizenContactIDFixture + "/deletion", body: `{"reason":"Người dân đã chuyển đi"}`, key: citizenUpdateKey, ok: http.StatusNoContent},
		{name: "xem số", method: "GET", path: "/api/v1/citizen-contacts/" + citizenContactIDFixture + "/phone", key: citizenUpdateKey, ok: http.StatusOK},
		{name: "nhóm", method: "GET", path: "/api/v1/citizen-groups", key: citizenReadKey, ok: http.StatusOK},
		{name: "lập nhóm", method: "POST", path: "/api/v1/citizen-groups", body: createCitizenGroupBody, key: citizenUpdateKey, ok: http.StatusCreated},
		{name: "sửa nhóm", method: "PATCH", path: "/api/v1/citizen-groups/" + citizenGroupIDFixture, body: createCitizenGroupBody, key: citizenUpdateKey, ok: http.StatusNoContent},
		{name: "gỡ nhóm", method: "POST", path: "/api/v1/citizen-groups/" + citizenGroupIDFixture + "/deletion", body: `{}`, key: citizenUpdateKey, ok: http.StatusNoContent},
		{name: "tải mẫu", method: "GET", path: "/api/v1/citizen-contact-import-template", key: citizenUpdateKey, ok: http.StatusOK},
		{name: "xem trước", method: "POST", path: "/api/v1/citizen-contact-import-previews", key: citizenUpdateKey, upload: true, ok: http.StatusOK},
		{name: "nhập", method: "POST", path: "/api/v1/citizen-contact-imports", key: citizenUpdateKey, upload: true, ok: http.StatusCreated},
		// ADR 0101 #6 — appended LAST: tests address the routes above by index.
		{name: "xem email", method: "GET", path: "/api/v1/citizen-contacts/" + citizenContactIDFixture + "/email", key: citizenUpdateKey, ok: http.StatusOK},
	}
}

// serverGrantingCitizen grants both keys in commune A and nothing in commune B.
func serverGrantingCitizen(t *testing.T) *mayChu {
	t.Helper()
	return serverGranting(t, citizenReadKey, citizenUpdateKey)
}

func (m *mayChu) callCitizenRoute(t *testing.T, rt citizenRoute, host, tok string) *httptest.ResponseRecorder {
	t.Helper()
	if rt.upload {
		return m.sendWorkbook(t, rt.path, host, tok, sampleCitizenContactWorkbook(t))
	}
	// goiIdem: the two POST creates and the import declare idem.Required; the others ignore the header.
	return m.goiIdem(t, rt.method, host, rt.path, rt.body, tok)
}

// --- rule 5, invariant 7: four cases, every route ------------------------------------------------

func TestCitizenDirectory_401WithoutToken(t *testing.T) {
	m := serverGrantingCitizen(t)
	for _, rt := range citizenRoutes() {
		if w := m.callCitizenRoute(t, rt, hostA, ""); w.Code != http.StatusUnauthorized {
			t.Errorf("%s: mã = %d, muốn 401 — %s", rt.name, w.Code, w.Body.String())
		}
	}
	if n := m.citizenDirectory.count(); n != 0 {
		t.Errorf("chưa đăng nhập mà use case chạy %d lần", n)
	}
}

// `admin.user` — a real key, the harness default — is neither `citizen.read` nor `citizen.update`.
func TestCitizenDirectory_403WrongPermission(t *testing.T) {
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)
	for _, rt := range citizenRoutes() {
		if w := m.callCitizenRoute(t, rt, hostA, tok); w.Code != http.StatusForbidden {
			t.Errorf("%s: mã = %d, muốn 403 — %s", rt.name, w.Code, w.Body.String())
		}
	}
	if n := m.citizenDirectory.count(); n != 0 {
		t.Errorf("sai quyền mà use case chạy %d lần", n)
	}
}

// Both keys held in commune A, signed in at commune B: the grants do not exist there.
func TestCitizenDirectory_403RightPermissionWrongCommune(t *testing.T) {
	m := serverGrantingCitizen(t)
	tok := m.tokenCho(t, xaB, sidB)
	for _, rt := range citizenRoutes() {
		if w := m.callCitizenRoute(t, rt, hostB, tok); w.Code != http.StatusForbidden {
			t.Errorf("%s: mã = %d, muốn 403 — %s", rt.name, w.Code, w.Body.String())
		}
	}
	if n := m.citizenDirectory.count(); n != 0 {
		t.Errorf("sai xã mà use case chạy %d lần", n)
	}
}

func TestCitizenDirectory_2xxBothCorrect(t *testing.T) {
	m := serverGrantingCitizen(t)
	tok := m.tokenCho(t, xaA, sidA)
	for _, rt := range citizenRoutes() {
		if w := m.callCitizenRoute(t, rt, hostA, tok); w.Code != rt.ok {
			t.Errorf("%s: mã = %d, muốn %d — %s", rt.name, w.Code, rt.ok, w.Body.String())
		}
	}
	if m.citizenDirectory.lastCommune != xaA {
		t.Errorf("use case chạy ở xã %q, muốn %q", m.citizenDirectory.lastCommune, xaA)
	}
}

// THE TWO KEYS DO NOT OPEN EACH OTHER'S ROUTES (ADR 0097 D2, D12): a reader cannot write, cannot open
// a number, cannot import; a writer without the read key cannot list. The reveal is on the WRITE key.
func TestCitizenDirectory_ReadKeyAndUpdateKeyAreSeparate(t *testing.T) {
	for _, held := range []authz.Perm{citizenReadKey, citizenUpdateKey} {
		m := serverGranting(t, held)
		tok := m.tokenCho(t, xaA, sidA)
		for _, rt := range citizenRoutes() {
			want := http.StatusForbidden
			if rt.key == held {
				want = rt.ok
			}
			if w := m.callCitizenRoute(t, rt, hostA, tok); w.Code != want {
				t.Errorf("giữ %s, gọi %s: mã = %d, muốn %d — %s", held, rt.name, w.Code, want, w.Body.String())
			}
		}
	}
}

// --- rule 6, invariant 8: the staff code is the trail's actor ----------------------------------------

func TestCitizenDirectoryActorIsStaffCode(t *testing.T) {
	m := serverGrantingCitizen(t)
	tok := m.tokenCho(t, xaA, sidA)
	for _, rt := range citizenRoutes() {
		if rt.key != citizenUpdateKey || rt.name == "tải mẫu" || rt.name == "xem trước" {
			continue // reads and the two routes that write nothing carry no actor
		}
		m.citizenDirectory.lastActor = app.NguoiThucHien{}
		if w := m.callCitizenRoute(t, rt, hostA, tok); w.Code != rt.ok {
			t.Fatalf("%s: mã = %d — %s", rt.name, w.Code, w.Body.String())
		}
		a := m.citizenDirectory.lastActor
		if a.Vet.ID != maCanBo || a.ID != idNoiBo || a.Vet.IP == "" {
			t.Errorf("%s: vết %q, quyết định %q, IP %q — muốn mã cán bộ %q và id nội bộ %q", rt.name, a.Vet.ID, a.ID, a.Vet.IP, maCanBo, idNoiBo)
		}
	}
}

// --- the wire: the mask ----------------------------------------------------------------------------

// The list carries `phone_masked` and nothing that could hold the full number, no address, no note.
func TestListCitizenContactsShapeIsMasked(t *testing.T) {
	m := serverGrantingCitizen(t)
	w := m.goi(t, "GET", hostA, "/api/v1/citizen-contacts", "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)
	body := w.Body.String()
	if strings.Contains(body, citizenPhoneFixture) || !strings.Contains(body, `"phone_masked":"`+citizenPhoneMaskedFixture+`"`) {
		t.Errorf("số phải bị che trong danh sách: %s", body)
	}
	for _, forbidden := range []string{`"address"`, `"note"`, `"phone":`} {
		if strings.Contains(body, forbidden) {
			t.Errorf("danh sách không được mang %s: %s", forbidden, body)
		}
	}
	var out page.Result[citizenContactOut]
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || len(out.Items) != 1 {
		t.Fatalf("thân = %s (%v)", body, err)
	}
	it := out.Items[0]
	if it.ID != citizenContactIDFixture || !it.HasOwnAppSession || it.Source != "nhap-tay" ||
		it.ResidentialUnitName != "Thôn Bình An" || len(it.Tags) != 1 || it.GroupIDs == nil {
		t.Errorf("dòng = %+v", it)
	}
	if !strings.Contains(body, `"group_ids":[]`) {
		t.Errorf("danh sách rỗng phải là [], không null: %s", body)
	}
}

// The detail carries address and note, and the number is STILL masked.
func TestGetCitizenContactDetailStillMasked(t *testing.T) {
	m := serverGrantingCitizen(t)
	w := m.goi(t, "GET", hostA, "/api/v1/citizen-contacts/"+citizenContactIDFixture, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)
	body := w.Body.String()
	if strings.Contains(body, citizenPhoneFixture) {
		t.Errorf("chi tiết vẫn phải che số: %s", body)
	}
	var out citizenContactDetailOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || out.Address != "Số 1, đường Lê Lợi" || out.Note != "ghi chú" ||
		out.PhoneMasked != citizenPhoneMaskedFixture || out.ImportBatchID != citizenBatchIDFixture || out.Source != "nhap-excel" {
		t.Errorf("chi tiết = %+v (%v)", out, err)
	}
	if m.citizenDirectory.lastID != citizenContactIDFixture {
		t.Errorf("id tới use case = %q", m.citizenDirectory.lastID)
	}
}

// The reveal is the ONE route that answers the full number — `{phone}`, no-store — and 404 maps the
// store's not-found.
func TestRevealCitizenContactPhone(t *testing.T) {
	m := serverGrantingCitizen(t)
	tok := m.tokenCho(t, xaA, sidA)
	w := m.goi(t, "GET", hostA, "/api/v1/citizen-contacts/"+citizenContactIDFixture+"/phone", "", tok)
	doiMa(t, w, http.StatusOK)
	var out citizenContactPhoneOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || out.Phone != citizenPhoneFixture {
		t.Errorf("thân = %s (%v)", w.Body.String(), err)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q", w.Header().Get("Cache-Control"))
	}
	if m.citizenDirectory.lastID != citizenContactIDFixture || m.citizenDirectory.lastActor.Vet.ID != maCanBo {
		t.Errorf("id %q, người xem %q", m.citizenDirectory.lastID, m.citizenDirectory.lastActor.Vet.ID)
	}

	m.citizenDirectory.err = idstore.ErrCitizenContactNotFound
	w = m.goi(t, "GET", hostA, "/api/v1/citizen-contacts/"+citizenContactIDFixture+"/phone", "", tok)
	if w.Code != http.StatusNotFound || errorCode(t, w.Body.Bytes()) != "citizen_contact_not_found" {
		t.Errorf("không tìm thấy: mã = %d %s", w.Code, w.Body.String())
	}
}

// --- the wire: bodies and filters -------------------------------------------------------------------

func TestCreateCitizenContactMapsBody(t *testing.T) {
	m := serverGrantingCitizen(t)
	w := m.callCitizenRoute(t, citizenRoutes()[4], hostA, m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusCreated)
	var out citizenContactCreatedOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || out.ID != citizenContactIDFixture {
		t.Errorf("thân = %s (%v)", w.Body.String(), err)
	}
	got := m.citizenDirectory.lastContact
	if got.Contact.FullName != "Nguyễn Văn A" || got.Contact.Phone != citizenPhoneFixture || got.Contact.Address != "Số 1" ||
		len(got.Contact.Tags) != 1 || len(got.GroupIDs) != 1 || got.GroupIDs[0] != citizenGroupIDFixture || !got.Contact.IsActive {
		t.Errorf("yêu cầu = %+v", got)
	}
}

// `is_active` absent is true (the checkbox's default); `false` reaches the use case as false.
func TestCitizenContactIsActiveDefaultsToTrue(t *testing.T) {
	m := serverGrantingCitizen(t)
	tok := m.tokenCho(t, xaA, sidA)
	w := m.goi(t, "PATCH", hostA, "/api/v1/citizen-contacts/"+citizenContactIDFixture,
		`{"full_name":"A","phone":"`+citizenPhoneFixture+`","is_active":false}`, tok)
	doiMa(t, w, http.StatusNoContent)
	if m.citizenDirectory.lastContact.Contact.IsActive || m.citizenDirectory.lastID != citizenContactIDFixture {
		t.Errorf("is_active=false phải tới use case: %+v", m.citizenDirectory.lastContact)
	}
	w = m.goi(t, "PATCH", hostA, "/api/v1/citizen-contacts/"+citizenContactIDFixture,
		`{"full_name":"A","phone":"`+citizenPhoneFixture+`"}`, tok)
	doiMa(t, w, http.StatusNoContent)
	if !m.citizenDirectory.lastContact.Contact.IsActive {
		t.Errorf("thiếu is_active phải là true")
	}
}

// POST without an Idempotency-Key is refused before the use case — both creates.
func TestCitizenDirectoryCreatesNeedIdempotencyKey(t *testing.T) {
	m := serverGrantingCitizen(t)
	tok := m.tokenCho(t, xaA, sidA)
	for _, c := range []struct{ path, body string }{
		{"/api/v1/citizen-contacts", createCitizenContactBody},
		{"/api/v1/citizen-groups", createCitizenGroupBody},
	} {
		w := m.goi(t, "POST", hostA, c.path, c.body, tok)
		if w.Code < 400 || m.citizenDirectory.count() != 0 {
			t.Errorf("%s thiếu Idempotency-Key: mã = %d, use case %d lần", c.path, w.Code, m.citizenDirectory.count())
		}
	}
}

// The keyword, the filters and the paging of the search body reach the store's filter unchanged; the
// keyword is trimmed.
func TestSearchCitizenContactsMapsBody(t *testing.T) {
	m := serverGrantingCitizen(t)
	tok := m.tokenCho(t, xaA, sidA)
	w := m.goiIdem(t, "POST", hostA, "/api/v1/citizen-contact-searches",
		`{"q":"  0900  ","group_id":"`+citizenGroupIDFixture+`","residential_unit_id":"`+citizenContactIDFixture+`","active":true,"has_own_app_session":true,"limit":20}`, tok)
	doiMa(t, w, http.StatusOK)
	f := m.citizenDirectory.lastFilter
	if f.Keyword != "0900" || f.GroupID != citizenGroupIDFixture || f.ResidentialUnitID != citizenContactIDFixture ||
		!f.ActiveOnly || f.HasOwnAppSession == nil || !*f.HasOwnAppSession {
		t.Errorf("bộ lọc = %+v", f)
	}
	if m.citizenDirectory.lastPage.Limit() != 20 {
		t.Errorf("limit = %d, muốn 20", m.citizenDirectory.lastPage.Limit())
	}

	// `active: false` and an absent keyword: everyone, no keyword.
	w = m.goiIdem(t, "POST", hostA, "/api/v1/citizen-contact-searches", `{"active":false}`, tok)
	doiMa(t, w, http.StatusOK)
	f = m.citizenDirectory.lastFilter
	if f.Keyword != "" || f.ActiveOnly || f.HasOwnAppSession != nil {
		t.Errorf("bộ lọc mặc định = %+v", f)
	}

	// Over the bound: 400, nothing reaches the store, and the keyword is not echoed.
	before := m.citizenDirectory.count()
	w = m.goiIdem(t, "POST", hostA, "/api/v1/citizen-contact-searches", `{"q":"`+strings.Repeat("9", 201)+`"}`, tok)
	if w.Code != http.StatusBadRequest || m.citizenDirectory.count() != before || strings.Contains(w.Body.String(), "999") {
		t.Errorf("từ khoá quá dài: mã = %d — %s", w.Code, w.Body.String())
	}
}

// The GET list reads the same filters from the query, and refuses a spelling it did not publish.
func TestListCitizenContactsQueryFilters(t *testing.T) {
	m := serverGrantingCitizen(t)
	tok := m.tokenCho(t, xaA, sidA)
	w := m.goi(t, "GET", hostA, "/api/v1/citizen-contacts?group_id="+citizenGroupIDFixture+"&active=true&has_own_app_session=false&limit=10", "", tok)
	doiMa(t, w, http.StatusOK)
	f := m.citizenDirectory.lastFilter
	if f.Keyword != "" || f.GroupID != citizenGroupIDFixture || !f.ActiveOnly || f.HasOwnAppSession == nil || *f.HasOwnAppSession ||
		m.citizenDirectory.lastPage.Limit() != 10 {
		t.Errorf("bộ lọc = %+v, limit %d", f, m.citizenDirectory.lastPage.Limit())
	}
	before := m.citizenDirectory.count()
	for _, bad := range []string{"?active=maybe", "?has_own_app_session=1", "?group_id=not-a-ulid", "?residential_unit_id=x", "?limit=0"} {
		if w := m.goi(t, "GET", hostA, "/api/v1/citizen-contacts"+bad, "", tok); w.Code != http.StatusBadRequest {
			t.Errorf("%s: mã = %d, muốn 400 — %s", bad, w.Code, w.Body.String())
		}
	}
	if m.citizenDirectory.count() != before {
		t.Errorf("tham số sai mà use case chạy thêm %d lần", m.citizenDirectory.count()-before)
	}
}

func TestCountCitizenContactsShape(t *testing.T) {
	m := serverGrantingCitizen(t)
	w := m.goiIdem(t, "POST", hostA, "/api/v1/citizen-contact-count-queries", `{"q":"Nguyễn","has_own_app_session":true}`, m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)
	if w.Body.String() != `{"count":57}`+"\n" && strings.TrimSpace(w.Body.String()) != `{"count":57}` {
		t.Errorf("thân = %q", w.Body.String())
	}
	f := m.citizenDirectory.lastFilter
	if f.Keyword != "Nguyễn" || f.HasOwnAppSession == nil || !*f.HasOwnAppSession {
		t.Errorf("bộ lọc = %+v", f)
	}
}

func TestListCitizenContactTagsAndGroupsShape(t *testing.T) {
	m := serverGrantingCitizen(t)
	tok := m.tokenCho(t, xaA, sidA)
	w := m.goi(t, "GET", hostA, "/api/v1/citizen-contact-tags", "", tok)
	doiMa(t, w, http.StatusOK)
	var tags citizenContactTagsOut
	if err := json.Unmarshal(w.Body.Bytes(), &tags); err != nil || len(tags.Items) != 2 || tags.Items[0].Tag != "hộ kinh doanh" || tags.Items[0].Count != 12 {
		t.Errorf("nhãn = %+v (%v)", tags, err)
	}
	w = m.goi(t, "GET", hostA, "/api/v1/citizen-groups", "", tok)
	doiMa(t, w, http.StatusOK)
	var groups citizenGroupsOut
	if err := json.Unmarshal(w.Body.Bytes(), &groups); err != nil || len(groups.Items) != 1 ||
		groups.Items[0].ID != citizenGroupIDFixture || groups.Items[0].MemberCount != 8 {
		t.Errorf("nhóm = %+v (%v)", groups, err)
	}
}

func TestCitizenGroupWritesMapBody(t *testing.T) {
	m := serverGrantingCitizen(t)
	tok := m.tokenCho(t, xaA, sidA)
	w := m.goiIdem(t, "POST", hostA, "/api/v1/citizen-groups", createCitizenGroupBody, tok)
	doiMa(t, w, http.StatusCreated)
	if g := m.citizenDirectory.lastGroup; g.Name != "Tổ trưởng dân phố" || g.SortOrder != 1 {
		t.Errorf("nhóm = %+v", g)
	}
	w = m.goiIdem(t, "POST", hostA, "/api/v1/citizen-groups/"+citizenGroupIDFixture+"/deletion", `{"reason":"Trùng nhóm khác"}`, tok)
	doiMa(t, w, http.StatusNoContent)
	if m.citizenDirectory.lastID != citizenGroupIDFixture || m.citizenDirectory.lastReason != "Trùng nhóm khác" {
		t.Errorf("id %q, lý do %q", m.citizenDirectory.lastID, m.citizenDirectory.lastReason)
	}
	w = m.goiIdem(t, "POST", hostA, "/api/v1/citizen-contacts/"+citizenContactIDFixture+"/deletion", `{"reason":"Đã chuyển đi"}`, tok)
	doiMa(t, w, http.StatusNoContent)
	if m.citizenDirectory.lastID != citizenContactIDFixture || m.citizenDirectory.lastReason != "Đã chuyển đi" {
		t.Errorf("id %q, lý do %q", m.citizenDirectory.lastID, m.citizenDirectory.lastReason)
	}
}

// --- refusals → status ------------------------------------------------------------------------------

func TestCitizenContactRefusalsMapToStatus(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{idstore.ErrCitizenContactNotFound, http.StatusNotFound, "citizen_contact_not_found"},
		{idstore.ErrCitizenPhoneTaken, http.StatusConflict, "phone_taken"},
		{idstore.ErrCitizenGroupNotFound, http.StatusBadRequest, "citizen_group_not_found"},
		{domain.ErrCitizenPhoneInvalid, http.StatusBadRequest, "invalid_request"},
		{domain.ErrCitizenContactFullNameMissing, http.StatusBadRequest, "invalid_request"},
		{idstore.ErrCitizenContactUnitUnknown, http.StatusBadRequest, "invalid_request"},
		{domain.ErrCitizenDirectoryReasonMissing, http.StatusBadRequest, "invalid_request"},
		{errors.New("db down"), http.StatusInternalServerError, "internal"},
	}
	// The three contact write routes: thêm (4), sửa (6), gỡ (7).
	for _, c := range cases {
		for _, i := range []int{4, 6, 7} {
			rt := citizenRoutes()[i]
			m := serverGrantingCitizen(t)
			m.citizenDirectory.err = c.err
			w := m.callCitizenRoute(t, rt, hostA, m.tokenCho(t, xaA, sidA))
			if w.Code != c.status || errorCode(t, w.Body.Bytes()) != c.code {
				t.Errorf("%s / %v: mã = %d %s, muốn %d %s", rt.name, c.err, w.Code, w.Body.String(), c.status, c.code)
			}
			if strings.Contains(w.Body.String(), citizenPhoneFixture) {
				t.Errorf("%s: thân lỗi không được mang số: %s", rt.name, w.Body.String())
			}
		}
	}
}

func TestCitizenGroupRefusalsMapToStatus(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{idstore.ErrCitizenGroupNotFound, http.StatusNotFound, "citizen_group_not_found"},
		{idstore.ErrCitizenGroupNameTaken, http.StatusConflict, "name_taken"},
		{domain.ErrCitizenGroupNameMissing, http.StatusBadRequest, "invalid_request"},
		{errors.New("db down"), http.StatusInternalServerError, "internal"},
	}
	// The three group write routes: lập (10), sửa (11), gỡ (12).
	for _, c := range cases {
		for _, i := range []int{10, 11, 12} {
			rt := citizenRoutes()[i]
			m := serverGrantingCitizen(t)
			m.citizenDirectory.err = c.err
			w := m.callCitizenRoute(t, rt, hostA, m.tokenCho(t, xaA, sidA))
			if w.Code != c.status || errorCode(t, w.Body.Bytes()) != c.code {
				t.Errorf("%s / %v: mã = %d %s, muốn %d %s", rt.name, c.err, w.Code, w.Body.String(), c.status, c.code)
			}
		}
	}
}

// A body that is not JSON is 400 before the use case, on every JSON route.
func TestCitizenDirectoryBadJSONIs400(t *testing.T) {
	m := serverGrantingCitizen(t)
	tok := m.tokenCho(t, xaA, sidA)
	for _, rt := range citizenRoutes() {
		if rt.body == "" {
			continue
		}
		if w := m.goiIdem(t, rt.method, hostA, rt.path, `{not json`, tok); w.Code != http.StatusBadRequest {
			t.Errorf("%s: mã = %d, muốn 400 — %s", rt.name, w.Code, w.Body.String())
		}
	}
	if n := m.citizenDirectory.count(); n != 0 {
		t.Errorf("thân hỏng mà use case chạy %d lần", n)
	}
}

// A read failure is 500 with the fixed sentence.
func TestCitizenDirectoryReadFailuresAre500(t *testing.T) {
	m := serverGrantingCitizen(t)
	m.citizenDirectory.err = errors.New("db down")
	tok := m.tokenCho(t, xaA, sidA)
	for _, i := range []int{0, 1, 2, 3, 5, 9} {
		rt := citizenRoutes()[i]
		if w := m.callCitizenRoute(t, rt, hostA, tok); w.Code != http.StatusInternalServerError {
			t.Errorf("%s: mã = %d, muốn 500 — %s", rt.name, w.Code, w.Body.String())
		}
	}
}
