package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// The four routes of the map field schema. Harness pieces are the ones loai_tai_nguyen_ban_do_ghi_
// test.go built for the catalogue writes — the commune-keyed checker (checkerDanhMucGia), the
// principal injector (chuTheGhi) and the staff principal whose ID and business code differ
// (canBoGhi) — because the "right permission, wrong commune" case needs exactly those properties.

// fakeMapFieldSchemas stands in for both the read store and the write use case, RECORDING THE
// COMMUNE it was called in (read from the context, as *store.Scoped reads it) and the actor.
type fakeMapFieldSchemas struct {
	list []domain.MapFieldSchema
	row  domain.MapFieldSchema
	err  error

	calls        int
	lastCommune  tenant.ID
	lastActor    audit.Actor
	lastTypeCode string
	lastCreate   app.CreateMapFieldRequest
	lastUpdate   app.UpdateMapFieldRequest
	lastID       string
	lastReason   string
}

func (f *fakeMapFieldSchemas) note(ctx context.Context) {
	f.calls++
	f.lastCommune = tenant.MustFrom(ctx)
}

func (f *fakeMapFieldSchemas) List(ctx context.Context, typeCode string) ([]domain.MapFieldSchema, error) {
	f.note(ctx)
	f.lastTypeCode = typeCode
	return f.list, f.err
}

func (f *fakeMapFieldSchemas) Create(ctx context.Context, req app.CreateMapFieldRequest,
	actor audit.Actor) (domain.MapFieldSchema, error) {
	f.note(ctx)
	f.lastCreate, f.lastActor = req, actor
	return f.row, f.err
}

func (f *fakeMapFieldSchemas) Update(ctx context.Context, id string, req app.UpdateMapFieldRequest,
	actor audit.Actor) (domain.MapFieldSchema, error) {
	f.note(ctx)
	f.lastID, f.lastUpdate, f.lastActor = id, req, actor
	return f.row, f.err
}

func (f *fakeMapFieldSchemas) Delete(ctx context.Context, id, reason string, actor audit.Actor) error {
	f.note(ctx)
	f.lastID, f.lastReason, f.lastActor = id, reason, actor
	return f.err
}

type mapFieldServer struct {
	h       http.Handler
	fake    *fakeMapFieldSchemas
	checker *checkerDanhMucGia
}

func newMapFieldServer(t *testing.T) *mapFieldServer {
	t.Helper()
	fake := &fakeMapFieldSchemas{
		row: domain.MapFieldSchema{
			ID: "mf-001", AssetTypeCode: "nhom-mau", FieldCode: "legal_form", Label: "Loại hình",
			ValueType: domain.ValueTypeChoice,
			Options:   []domain.FieldOption{{Value: "tnhh", Label: "Công ty TNHH"}},
			IsActive:  true,
		},
	}
	checker := &checkerDanhMucGia{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	mux := http.NewServeMux()
	Register(mux, Deps{
		Checker:              checker,
		LoaiTaiNguyen:        danhMucMau(),
		GhiLoaiTaiNguyen:     &ghiDanhMucGia{},
		ThongBao:             &soThongBaoGia{},
		GhiThongBao:          &ghiThongBaoGia{},
		NoiDung:              &soNoiDungGia{},
		GhiNoiDung:           &ghiNoiDungGia{},
		DanhMucNoiDung:       &soDanhMucNDGia{},
		GhiDanhMucNoiDung:    &ghiDanhMucNDGia{},
		MapFieldSchemas:      fake,
		WriteMapFieldSchemas: fake,
		// The mail server: present because Register refuses a nil one; its suite is mail_settings_test.go.
		MailSettings:      &fakeMailSettings{},
		WriteMailSettings: &fakeMailSettings{},
		// The audit-log reader: present because Register refuses a nil one; its suite is audit_entries_test.go.
		AuditLog: &auditLogFake{},
		Log:      log,
	})

	// The real edge chain in the real order, idem with a nil store (a valid deployment) so each
	// route's declared mode is what is under test.
	var h http.Handler = mux
	h = chuTheGhi(h)
	h = idem.Middleware(nil, log)(h)
	h = httpx.TenantMiddleware(thuMucMau())(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)
	return &mapFieldServer{h: h, fake: fake, checker: checker}
}

func (s *mapFieldServer) grant(commune tenant.ID, perms ...authz.Perm) {
	if s.checker.co == nil {
		s.checker.co = map[tenant.ID]map[authz.Perm]struct{}{}
	}
	if s.checker.co[commune] == nil {
		s.checker.co[commune] = map[authz.Perm]struct{}{}
	}
	for _, p := range perms {
		s.checker.co[commune][p] = struct{}{}
	}
}

func (s *mapFieldServer) call(t *testing.T, method, host, path string, p *authz.Principal, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	r := httptest.NewRequest(method, "https://"+host+path, rd)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set(idem.Header, "01JIDEMPOTENCYKEYMAPFIELD")
	if p != nil {
		r = r.WithContext(context.WithValue(r.Context(), khoaChuTheGhi{}, *p))
	}
	w := httptest.NewRecorder()
	s.h.ServeHTTP(w, r)
	return w
}

const mapFieldPath = "/api/v1/map-field-schemas"

const createMapFieldBody = `{"asset_type_code":"nhom-mau","field_code":"legal_form","label":"Loại hình",` +
	`"value_type":"chon","options":[{"value":"tnhh","label":"Công ty TNHH"}],"sort_order":1}`

type mapFieldRoute struct {
	name, method, path, body string
	perm                     authz.Perm // the key the route must ask for
	wrongPerm                authz.Perm // a REAL key of the subsystem that must not be enough
	ok                       int
}

// fourRoutes — the permission cases are asserted on ALL of them, not on the first one written.
//
// The wrong keys are deliberately real and adjacent: a map reader (`asset.read`) must not be able
// to change the form; a content reader must not read it.
func fourRoutes() []mapFieldRoute {
	return []mapFieldRoute{
		{"GET", http.MethodGet, mapFieldPath, "", "asset.read", "content.read", http.StatusOK},
		{"POST", http.MethodPost, mapFieldPath, createMapFieldBody, "admin.lookup", "asset.read", http.StatusCreated},
		{"PATCH", http.MethodPatch, mapFieldPath + "/mf-001", `{"label":"Loại hình DN"}`, "admin.lookup", "asset.read", http.StatusOK},
		{"DELETE", http.MethodDelete, mapFieldPath + "/mf-001", `{"reason":"không dùng nữa"}`, "admin.lookup", "asset.update", http.StatusNoContent},
	}
}

func TestMapFieldRoutesAskForSeededKeys(t *testing.T) {
	// Compared with LITERALS: a fake checker grants any string, so without this a route holding a
	// key the `quyen` table lacks stays green here while answering 403 to every account (rule 5,
	// invariant 3c). tools/check_quyen.py is the whole-repo half.
	want := map[string]authz.Perm{"GET": "asset.read", "POST": "admin.lookup", "PATCH": "admin.lookup", "DELETE": "admin.lookup"}
	for _, tc := range fourRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newMapFieldServer(t)
			s.call(t, tc.method, hostA, tc.path, canBoGhi(xaA), tc.body)
			if got := s.checker.hoiKhoaCuoi(); got != want[tc.name] {
				t.Fatalf("route asked for %q, want %q", got, want[tc.name])
			}
		})
	}
}

func TestMapFieldRoutes_401NoSession(t *testing.T) {
	for _, tc := range fourRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newMapFieldServer(t)
			s.grant(xaA, tc.perm)
			doiMa(t, s.call(t, tc.method, hostA, tc.path, nil, tc.body), http.StatusUnauthorized)
			if s.fake.calls != 0 {
				t.Error("no session and the store/use case still ran")
			}
		})
	}
}

func TestMapFieldRoutes_403WrongPermission(t *testing.T) {
	for _, tc := range fourRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newMapFieldServer(t)
			s.grant(xaA, tc.wrongPerm)
			doiMa(t, s.call(t, tc.method, hostA, tc.path, canBoGhi(xaA), tc.body), http.StatusForbidden)
			if s.fake.calls != 0 {
				t.Error("wrong permission and the store/use case still ran")
			}
		})
	}
}

func TestMapFieldRoutes_403RightPermissionWrongCommune(t *testing.T) {
	// Signed in at commune B as a member of B — nothing malformed — but the grant is in commune A.
	// A checker that ignored the commune would let A's administrator configure B's map.
	for _, tc := range fourRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newMapFieldServer(t)
			s.grant(xaA, tc.perm)
			doiMa(t, s.call(t, tc.method, hostB, tc.path, canBoGhi(xaB), tc.body), http.StatusForbidden)
			if s.fake.calls != 0 {
				t.Error("a grant in another commune was enough")
			}
		})
	}
}

func TestMapFieldRoutes_401SessionOfAnotherCommune(t *testing.T) {
	for _, tc := range fourRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newMapFieldServer(t)
			s.grant(xaA, tc.perm)
			s.grant(xaB, tc.perm)
			doiMa(t, s.call(t, tc.method, hostB, tc.path, canBoGhi(xaA), tc.body), http.StatusUnauthorized)
			if s.fake.calls != 0 {
				t.Error("a session of another commune still reached the store/use case")
			}
		})
	}
}

func TestMapFieldRoutes_RightPermissionRightCommune(t *testing.T) {
	for _, tc := range fourRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newMapFieldServer(t)
			s.grant(xaA, tc.perm)
			doiMa(t, s.call(t, tc.method, hostA, tc.path, canBoGhi(xaA), tc.body), tc.ok)
			if s.fake.calls != 1 {
				t.Fatalf("store/use case ran %d times, want 1", s.fake.calls)
			}
			if s.fake.lastCommune != xaA {
				t.Errorf("ran in commune %q, want %q", s.fake.lastCommune, xaA)
			}
			if tc.method == http.MethodGet {
				return
			}
			// Rule 6, invariant 8: the trail's "who" is the BUSINESS CODE, never the internal id.
			if s.fake.lastActor.ID != maCanBoGhi || s.fake.lastActor.ID == idCanBoGhi {
				t.Errorf("actor = %+v, want business code %q", s.fake.lastActor, maCanBoGhi)
			}
			if s.fake.lastActor.IP != "10.0.0.7" {
				t.Errorf("actor IP = %q, want the socket address", s.fake.lastActor.IP)
			}
		})
	}
}

func TestPatchMapFieldRefusesImmutableFields(t *testing.T) {
	// Type, key and group are immutable — refused even when the value is unchanged, so a client
	// that posts the row back learns it could not have changed them.
	for name, body := range map[string]string{
		"value_type":      `{"label":"X","value_type":"chon"}`,
		"field_code":      `{"label":"X","field_code":"legal_form"}`,
		"asset_type_code": `{"label":"X","asset_type_code":"nhom-mau"}`,
	} {
		t.Run(name, func(t *testing.T) {
			s := newMapFieldServer(t)
			s.grant(xaA, "admin.lookup")
			w := s.call(t, http.MethodPatch, hostA, mapFieldPath+"/mf-001", canBoGhi(xaA), body)
			doiMa(t, w, http.StatusBadRequest)
			if s.fake.calls != 0 {
				t.Error("immutable field in the body and the use case still ran")
			}
			if e := loiTra(t, w); !strings.Contains(e.Message, name) {
				t.Errorf("message does not name the refused field: %q", e.Message)
			}
		})
	}
}

func TestMapFieldErrorsMapToStatuses(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		code int
		key  string
	}{
		"not found":       {commsstore.ErrMapFieldSchemaNotFound, http.StatusNotFound, "not_found"},
		"type missing":    {commsstore.ErrAssetTypeMissing, http.StatusConflict, "asset_type_missing"},
		"key taken":       {commsstore.ErrFieldCodeTaken, http.StatusConflict, "field_code_taken"},
		"key retired":     {commsstore.ErrFieldCodeRetired, http.StatusConflict, "field_code_retired"},
		"full":            {commsstore.ErrMapFieldSchemaFull, http.StatusConflict, "catalogue_full"},
		"option removed":  {domain.ErrOptionRemoved, http.StatusConflict, "option_removed"},
		"chon no options": {domain.ErrOptionsRequired, http.StatusBadRequest, "invalid_request"},
		"bad key":         {domain.ErrFieldCodeShape, http.StatusBadRequest, "invalid_request"},
		"unknown type":    {domain.ErrValueTypeUnknown, http.StatusBadRequest, "invalid_request"},
		"store broken":    {errors.New("cơ sở dữ liệu không phản hồi"), http.StatusInternalServerError, "internal"},
	} {
		t.Run(name, func(t *testing.T) {
			s := newMapFieldServer(t)
			s.grant(xaA, "admin.lookup")
			s.fake.err = tc.err
			w := s.call(t, http.MethodPost, hostA, mapFieldPath, canBoGhi(xaA), createMapFieldBody)
			doiMa(t, w, tc.code)
			e := loiTra(t, w)
			if e.Code != tc.key {
				t.Errorf("code = %q, want %q", e.Code, tc.key)
			}
			if strings.Contains(e.Message, "cơ sở dữ liệu không phản hồi") {
				t.Errorf("internal failure leaked to the client: %q", e.Message)
			}
		})
	}
}

func TestCreateMapFieldPassesBodyAndRequiresIdempotencyKey(t *testing.T) {
	s := newMapFieldServer(t)
	s.grant(xaA, "admin.lookup")
	w := s.call(t, http.MethodPost, hostA, mapFieldPath, canBoGhi(xaA), createMapFieldBody)
	doiMa(t, w, http.StatusCreated)
	c := s.fake.lastCreate
	if c.AssetTypeCode != "nhom-mau" || c.FieldCode != "legal_form" || c.ValueType != "chon" ||
		len(c.Options) != 1 || c.Options[0].Value != "tnhh" || c.SortOrder != 1 {
		t.Errorf("request reaching the use case = %+v", c)
	}

	// Without the header, POST is refused before the handler (idem.Required).
	r := httptest.NewRequest(http.MethodPost, "https://"+hostA+mapFieldPath, strings.NewReader(createMapFieldBody))
	r.Host = hostA
	r.RemoteAddr = "10.0.0.7:51000"
	r = r.WithContext(context.WithValue(r.Context(), khoaChuTheGhi{}, *canBoGhi(xaA)))
	w = httptest.NewRecorder()
	before := s.fake.calls
	s.h.ServeHTTP(w, r)
	doiMa(t, w, http.StatusBadRequest)
	if s.fake.calls != before {
		t.Error("missing Idempotency-Key and the use case still ran")
	}
}

func TestPatchMapFieldPointerSemantics(t *testing.T) {
	s := newMapFieldServer(t)
	s.grant(xaA, "admin.lookup")

	doiMa(t, s.call(t, http.MethodPatch, hostA, mapFieldPath+"/mf-001", canBoGhi(xaA), `{"label":"Mới"}`), http.StatusOK)
	u := s.fake.lastUpdate
	if s.fake.lastID != "mf-001" || u.Label == nil || *u.Label != "Mới" {
		t.Errorf("id/label = %q / %v", s.fake.lastID, u.Label)
	}
	if u.Options != nil || u.IsRequired != nil || u.SortOrder != nil || u.IsActive != nil {
		t.Errorf("unmentioned fields arrived set: %+v", u)
	}

	// `Tắt`, a zero sort order and an appended option must all arrive as values, not as absent.
	doiMa(t, s.call(t, http.MethodPatch, hostA, mapFieldPath+"/mf-001", canBoGhi(xaA),
		`{"is_active":false,"sort_order":0,"is_required":true,`+
			`"options":[{"value":"tnhh","label":"TNHH"},{"value":"cp","label":"Cổ phần"}]}`), http.StatusOK)
	u = s.fake.lastUpdate
	if u.IsActive == nil || *u.IsActive || u.SortOrder == nil || *u.SortOrder != 0 ||
		u.IsRequired == nil || !*u.IsRequired || u.Options == nil || len(*u.Options) != 2 {
		t.Errorf("values did not arrive: %+v", u)
	}
}

func TestListMapFieldsFilterAndShape(t *testing.T) {
	s := newMapFieldServer(t)
	s.grant(xaA, "asset.read")
	s.fake.list = []domain.MapFieldSchema{{
		ID: "mf-002", AssetTypeCode: "nhom-mau", FieldCode: "revenue_estimate", Label: "Doanh thu ước",
		ValueType: domain.ValueTypeDecimal, Options: []domain.FieldOption{}, IsActive: false,
	}}

	w := s.call(t, http.MethodGet, hostA, mapFieldPath+"?asset_type_code=nhom-mau", canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	if s.fake.lastTypeCode != "nhom-mau" {
		t.Errorf("filter reaching the store = %q", s.fake.lastTypeCode)
	}
	// `options` must be [] and never null, and a disabled field is still listed.
	if !strings.Contains(w.Body.String(), `"options":[]`) || !strings.Contains(w.Body.String(), `"is_active":false`) {
		t.Errorf("body = %s", w.Body.String())
	}
	var out mapFieldSchemaListOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || len(out.Items) != 1 {
		t.Fatalf("body = %s (%v)", w.Body.String(), err)
	}

	// An empty result is `{"items":[]}`, which is every commune today.
	s.fake.list = nil
	w = s.call(t, http.MethodGet, hostA, mapFieldPath, canBoGhi(xaA), "")
	if !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Errorf("empty list body = %s", w.Body.String())
	}

	// A malformed filter is a 400 that does not echo the input.
	calls := s.fake.calls
	w = s.call(t, http.MethodGet, hostA, mapFieldPath+"?asset_type_code=Nhom%20XYZ", canBoGhi(xaA), "")
	doiMa(t, w, http.StatusBadRequest)
	if s.fake.calls != calls || strings.Contains(w.Body.String(), "XYZ") {
		t.Errorf("bad filter reached the store or was echoed: %s", w.Body.String())
	}
}

func TestDeleteMapFieldPassesReasonAndAnswers204(t *testing.T) {
	s := newMapFieldServer(t)
	s.grant(xaA, "admin.lookup")
	w := s.call(t, http.MethodDelete, hostA, mapFieldPath+"/mf-001", canBoGhi(xaA), `{"reason":"không dùng nữa"}`)
	doiMa(t, w, http.StatusNoContent)
	if s.fake.lastID != "mf-001" || s.fake.lastReason != "không dùng nữa" || w.Body.Len() != 0 {
		t.Errorf("id/reason/body = %q / %q / %q", s.fake.lastID, s.fake.lastReason, w.Body.String())
	}
}
