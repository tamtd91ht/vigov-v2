package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The five routes of Cấu hình → Thuộc tính danh bạ (ADR 0101), rule 5 invariant 7's four cases each, plus
// what the handler owns: the retired view's extra key, the body mapping, the status of each refusal — and
// the search body's new filters and its 503.

type citizenFieldsFake struct {
	mu          sync.Mutex
	calls       int
	lastCommune tenant.ID
	lastActor   app.NguoiThucHien
	lastRetired bool
	lastInput   domain.CitizenFieldInput
	lastPatch   domain.CitizenFieldPatch
	lastCodes   []string
	lastReason  string
	err         error
}

func (f *citizenFieldsFake) note(ctx context.Context) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.lastCommune = tenant.MustFrom(ctx)
}

func (f *citizenFieldsFake) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func sampleField() domain.CitizenField {
	return domain.CitizenField{Code: "hongheo1", Label: "Phân loại hộ", Type: domain.FieldCombobox,
		Options: []domain.CitizenFieldOption{{Value: "ho-ngheo", Label: "Hộ nghèo"}}}
}

func (f *citizenFieldsFake) List(ctx context.Context, includeRetired bool) ([]domain.CitizenField, error) {
	f.note(ctx)
	f.lastRetired = includeRetired
	return []domain.CitizenField{sampleField()}, f.err
}
func (f *citizenFieldsFake) Create(ctx context.Context, actor app.NguoiThucHien, in domain.CitizenFieldInput) (domain.CitizenField, error) {
	f.note(ctx)
	f.lastActor, f.lastInput = actor, in
	return sampleField(), f.err
}
func (f *citizenFieldsFake) Update(ctx context.Context, actor app.NguoiThucHien, code string, p domain.CitizenFieldPatch) (domain.CitizenField, error) {
	f.note(ctx)
	f.lastActor, f.lastPatch = actor, p
	return sampleField(), f.err
}
func (f *citizenFieldsFake) Retire(ctx context.Context, actor app.NguoiThucHien, code, reason string) error {
	f.note(ctx)
	f.lastActor, f.lastReason = actor, reason
	return f.err
}
func (f *citizenFieldsFake) Reorder(ctx context.Context, actor app.NguoiThucHien, codes []string) error {
	f.note(ctx)
	f.lastActor, f.lastCodes = actor, codes
	return f.err
}

const citizenFieldKey = authz.Perm("admin.citizen_field")

type citizenFieldRoute struct {
	name, method, path, body string
	ok                       int
}

func citizenFieldWriteRoutes() []citizenFieldRoute {
	return []citizenFieldRoute{
		{"thêm", "POST", "/api/v1/citizen-fields", `{"code":"hongheo1","label":"Phân loại hộ","field_type":"combobox","options":[{"value":"ho-ngheo","label":"Hộ nghèo"}]}`, http.StatusCreated},
		{"sửa", "PATCH", "/api/v1/citizen-fields/hongheo1", `{"label":"Phân loại hộ gia đình","options":[{"value":"ho-ngheo","label":"Hộ nghèo","retired":true}]}`, http.StatusOK},
		{"ngừng", "DELETE", "/api/v1/citizen-fields/hongheo1", `{"reason":"Không dùng nữa"}`, http.StatusNoContent},
		{"sắp xếp", "PUT", "/api/v1/citizen-field-order", `{"codes":["hongheo1"]}`, http.StatusNoContent},
		{"xem cả đã ngừng", "GET", "/api/v1/citizen-fields?include_retired=true", "", http.StatusOK},
	}
}

func TestCitizenFields_401WithoutToken(t *testing.T) {
	m := serverGranting(t, citizenFieldKey)
	for _, rt := range append(citizenFieldWriteRoutes(), citizenFieldRoute{"đọc", "GET", "/api/v1/citizen-fields", "", 200}) {
		if w := m.goiIdem(t, rt.method, hostA, rt.path, rt.body, ""); w.Code != http.StatusUnauthorized {
			t.Errorf("%s: %d, want 401", rt.name, w.Code)
		}
	}
	if m.citizenFields.count() != 0 {
		t.Error("use case ran without a session")
	}
}

// citizen.read opens the plain list only — never a write and never the retired view.
func TestCitizenFields_403WrongPermission(t *testing.T) {
	for _, held := range []authz.Perm{"admin.user", citizenReadKey, "admin.zalo"} {
		m := serverGranting(t, held)
		tok := m.tokenCho(t, xaA, sidA)
		for _, rt := range citizenFieldWriteRoutes() {
			if w := m.goiIdem(t, rt.method, hostA, rt.path, rt.body, tok); w.Code != http.StatusForbidden {
				t.Errorf("holding %s, %s: %d, want 403 — %s", held, rt.name, w.Code, w.Body.String())
			}
		}
	}
	m := serverGranting(t, "admin.user")
	if w := m.goi(t, "GET", hostA, "/api/v1/citizen-fields", "", m.tokenCho(t, xaA, sidA)); w.Code != http.StatusForbidden {
		t.Errorf("plain list without either key: %d", w.Code)
	}
}

func TestCitizenFields_403RightPermissionWrongCommune(t *testing.T) {
	m := serverGranting(t, citizenFieldKey, citizenReadKey)
	tok := m.tokenCho(t, xaB, sidB)
	for _, rt := range append(citizenFieldWriteRoutes(), citizenFieldRoute{"đọc", "GET", "/api/v1/citizen-fields", "", 200}) {
		if w := m.goiIdem(t, rt.method, hostB, rt.path, rt.body, tok); w.Code != http.StatusForbidden {
			t.Errorf("%s: %d, want 403", rt.name, w.Code)
		}
	}
	if m.citizenFields.count() != 0 {
		t.Error("use case ran in the wrong commune")
	}
}

func TestCitizenFields_2xxBothCorrect(t *testing.T) {
	m := serverGranting(t, citizenFieldKey)
	tok := m.tokenCho(t, xaA, sidA)
	for _, rt := range citizenFieldWriteRoutes() {
		if w := m.goiIdem(t, rt.method, hostA, rt.path, rt.body, tok); w.Code != rt.ok {
			t.Errorf("%s: %d, want %d — %s", rt.name, w.Code, rt.ok, w.Body.String())
		}
	}
	f := m.citizenFields
	if f.lastCommune != xaA || f.lastActor.Vet.ID != maCanBo || strings.Join(f.lastCodes, ",") != "hongheo1" || f.lastReason != "Không dùng nữa" {
		t.Errorf("use case saw %+v", f)
	}
	if f.lastInput.Type != domain.FieldCombobox || len(f.lastInput.Options) != 1 {
		t.Errorf("create input %+v", f.lastInput)
	}
	if f.lastPatch.Options == nil || !(*f.lastPatch.Options)[0].Retired || f.lastPatch.Label == nil {
		t.Errorf("patch %+v", f.lastPatch)
	}
	// The plain list with citizen.read, live only.
	m2 := serverGranting(t, citizenReadKey)
	w := m2.goi(t, "GET", hostA, "/api/v1/citizen-fields", "", m2.tokenCho(t, xaA, sidA))
	if w.Code != http.StatusOK || m2.citizenFields.lastRetired {
		t.Fatalf("plain list: %d retired=%v", w.Code, m2.citizenFields.lastRetired)
	}
	var out citizenFieldsOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || len(out.Items) != 1 || out.Items[0].FieldType != "combobox" || out.Items[0].Options[0].Value != "ho-ngheo" {
		t.Errorf("body %s", w.Body.String())
	}
}

func TestCitizenFieldErrorsMapToStatuses(t *testing.T) {
	m := serverGranting(t, citizenFieldKey)
	tok := m.tokenCho(t, xaA, sidA)
	post := citizenFieldWriteRoutes()[0]
	for _, c := range []struct {
		err  error
		code int
		want string
	}{
		{idstore.ErrCitizenFieldCodeTaken, http.StatusConflict, "citizen_field_code_taken"},
		{app.ErrCitizenFieldLimit, http.StatusConflict, "citizen_field_limit"},
		{domain.ErrCitizenFieldCodeInvalid, http.StatusBadRequest, "invalid_request"},
		{idstore.ErrCitizenFieldNotFound, http.StatusNotFound, "citizen_field_not_found"},
		{errors.New("db down"), http.StatusInternalServerError, "internal"},
	} {
		m.citizenFields.err = c.err
		w := m.goiIdem(t, post.method, hostA, post.path, post.body, tok)
		if w.Code != c.code || !strings.Contains(w.Body.String(), c.want) {
			t.Errorf("%v: %d %s", c.err, w.Code, w.Body.String())
		}
	}
	m.citizenFields.err = nil
	w := m.goi(t, "PATCH", hostA, "/api/v1/citizen-fields/hongheo1", `{"code":"khacma01"}`, tok)
	if w.Code != http.StatusBadRequest {
		t.Errorf("changing the code: %d", w.Code)
	}
}

// --- the search body ------------------------------------------------------------------------------------

func TestSearchBodyCarriesFiltersAndMaps503(t *testing.T) {
	m := serverGrantingCitizen(t)
	tok := m.tokenCho(t, xaA, sidA)
	body := `{"q":"an","created_from":1700000000000,"created_to":1700086399999,"filters":[{"code":"hongheo1","values":["ho-ngheo"]},{"code":"sonhan01","from":2}]}`
	w := m.goi(t, "POST", hostA, "/api/v1/citizen-contact-searches", body, tok)
	if w.Code != http.StatusOK {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	q := m.citizenDirectory.lastSearch
	if q.CreatedFrom == nil || *q.CreatedFrom != 1700000000000 || len(q.Options) != 1 || len(q.Ranges) != 1 || *q.Ranges[0].From != 2 || q.Filter.Keyword != "an" {
		t.Errorf("search = %+v", q)
	}
	if !strings.Contains(w.Body.String(), `"attributes":{}`) {
		t.Errorf("attributes must be {} not null: %s", w.Body.String())
	}
	if w := m.goi(t, "POST", hostA, "/api/v1/citizen-contact-searches", `{"filters":[{"code":"x"}]}`, tok); w.Code != http.StatusBadRequest {
		t.Errorf("a filter with neither values nor range: %d", w.Code)
	}
	m.citizenDirectory.err = app.ErrCitizenSearchUnavailable
	for _, path := range []string{"/api/v1/citizen-contact-searches", "/api/v1/citizen-contact-count-queries"} {
		w := m.goi(t, "POST", hostA, path, body, tok)
		if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "search_unavailable") {
			t.Errorf("%s: %d %s", path, w.Code, w.Body.String())
		}
	}
	m.citizenDirectory.err = app.ErrCitizenSearchCombination
	if w := m.goi(t, "POST", hostA, "/api/v1/citizen-contact-searches", body, tok); w.Code != http.StatusBadRequest {
		t.Errorf("combination: %d", w.Code)
	}
}

func TestContactBodyEmailAndAttributesPresence(t *testing.T) {
	for _, c := range []struct {
		body       string
		emailSet   bool
		attrsSet   bool
		emailValue string
	}{
		{`{"full_name":"A","phone":"0900000001"}`, false, false, ""},
		{`{"full_name":"A","phone":"0900000001","email":"","attributes":{}}`, true, true, ""},
		{`{"full_name":"A","phone":"0900000001","email":"ten.nguoi@example.vn","attributes":{"sonhan01":4}}`, true, true, "ten.nguoi@example.vn"},
	} {
		var in citizenContactIn
		if err := json.Unmarshal([]byte(c.body), &in); err != nil {
			t.Fatal(err)
		}
		req := in.toRequest()
		if (req.Email != nil) != c.emailSet || req.AttributesSet != c.attrsSet || (req.Email != nil && *req.Email != c.emailValue) {
			t.Errorf("%s → email %v attrsSet %v", c.body, req.Email, req.AttributesSet)
		}
	}
}

// The detail carries `email_masked`, never the address; the reveal route answers it, no-store.
func TestEmailMaskedOnDetailAndRevealRoute(t *testing.T) {
	m := serverGrantingCitizen(t)
	tok := m.tokenCho(t, xaA, sidA)
	w := m.goi(t, "GET", hostA, "/api/v1/citizen-contacts/"+citizenContactIDFixture, "", tok)
	if !strings.Contains(w.Body.String(), `"email_masked":`) || strings.Contains(w.Body.String(), citizenEmailFixture) {
		t.Errorf("detail: %s", w.Body.String())
	}
	w = m.goi(t, "GET", hostA, "/api/v1/citizen-contacts/"+citizenContactIDFixture+"/email", "", tok)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), citizenEmailFixture) || w.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("reveal: %d %s", w.Code, w.Body.String())
	}
}
