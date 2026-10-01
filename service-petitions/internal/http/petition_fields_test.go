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
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	docstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// The petition field catalogue routes (petition_fields.go):
//
//	staff    GET / PATCH /api/v1/citizen-report-fields — rule 5 invariant 7's four cases, on the
//	         mayChuGhi harness (commune-keyed checker), plus the error mapping
//	citizen  GET /api/v1/my-citizen-report-fields — commune from the SESSION, filtered list, a session
//	         without a verified phone accepted, 401 without a session, 503 when platform is down
//
// The merge and filter rules themselves are proved in internal/domain/petition_field_test.go and the
// transaction in internal/app/petition_field_catalogue_test.go.

// fieldCatalogueFake is the catalogue, KEYED BY COMMUNE read from the context the way the scoped store
// reads it — keyed any other way, the wrong-commune cases would pass while proving nothing.
type fieldCatalogueFake struct {
	byCommune map[tenant.ID][]domain.PetitionFieldView
	err       error

	catalogueCalls, editCalls, citizenCalls int
	lastCommune                             tenant.ID
	lastCode                                string
	lastEdit                                domain.PetitionFieldEdit
	lastActor                               audit.Actor
}

// newFieldCatalogueFake: commune A holds four codes covering every case a filter could get wrong —
// enabled, switched OFF by the commune, `can-bo`, and retired on the platform. Commune B has one code
// with a DIFFERENT label, so a leak between them shows.
func newFieldCatalogueFake() *fieldCatalogueFake {
	return &fieldCatalogueFake{byCommune: map[tenant.ID][]domain.PetitionFieldView{
		xaA: {
			{Code: "rac-thai", Label: "Rác thải xã A", DefaultLabel: "Rác thải – Vệ sinh môi trường",
				Order: 1, DefaultOrder: 2, Icon: "Trash2", Tone: "green", Active: true, Enabled: true, Customised: true},
			{Code: "giao-thong", Label: "Giao thông", DefaultLabel: "Giao thông",
				Order: 3, DefaultOrder: 3, Active: true, Enabled: false, Customised: true},
			{Code: "can-bo", Label: "Thái độ cán bộ", DefaultLabel: "Thái độ cán bộ",
				Order: 11, DefaultOrder: 11, Icon: "UserX", Tone: "red", Active: true, Enabled: true},
			{Code: "ma-cu", Label: "Mã đã ngừng", DefaultLabel: "Mã đã ngừng",
				Order: 13, DefaultOrder: 13, Active: false, Enabled: true},
		},
		xaB: {
			{Code: "rac-thai", Label: "Rác thải xã B", DefaultLabel: "Rác thải – Vệ sinh môi trường",
				Order: 2, DefaultOrder: 2, Active: true, Enabled: true, Customised: true},
		},
	}}
}

func (f *fieldCatalogueFake) Catalogue(ctx context.Context) ([]domain.PetitionFieldView, error) {
	f.catalogueCalls++
	f.lastCommune = tenant.MustFrom(ctx)
	if f.err != nil {
		return nil, f.err
	}
	return f.byCommune[f.lastCommune], nil
}

func (f *fieldCatalogueFake) Edit(ctx context.Context, code string, e domain.PetitionFieldEdit,
	actor audit.Actor) (domain.PetitionFieldView, error) {

	f.editCalls++
	f.lastCommune, f.lastCode, f.lastEdit, f.lastActor = tenant.MustFrom(ctx), code, e, actor
	if f.err != nil {
		return domain.PetitionFieldView{}, f.err
	}
	for _, v := range f.byCommune[f.lastCommune] {
		if v.Code == code {
			return v, nil
		}
	}
	return domain.PetitionFieldView{}, docstore.ErrDanhMucKhongTonTai
}

// CitizenCatalogue applies the REAL domain filter, so the handler is tested against what the use case
// would hand it rather than against a fixture that is already filtered.
func (f *fieldCatalogueFake) CitizenCatalogue(ctx context.Context) ([]domain.PetitionFieldView, error) {
	f.citizenCalls++
	f.lastCommune = tenant.MustFrom(ctx)
	if f.err != nil {
		return nil, f.err
	}
	return domain.CitizenCatalogue(f.byCommune[f.lastCommune]), nil
}

const (
	pathFields     = "/api/v1/citizen-report-fields"
	bodyEditFields = `{"label":"Rác thải xã A","enabled":false}`
)

// --- staff GET: rule 5, invariant 7 ------------------------------------------------------------------

func TestPetitionFields_ListAsksAdminLookup(t *testing.T) {
	// A LITERAL: a fake checker grants any string, so only this catches a key `quyen` lacks (rule 5, 3c).
	m := dungMayChuGhi(t)
	m.goi(t, http.MethodGet, hostA, pathFields, canBoGhi(xaA))
	if got := m.checker.hoiKhoaCuoi(); got != "admin.lookup" {
		t.Fatalf("route asked key %q, want \"admin.lookup\"", got)
	}
}

func TestPetitionFields_List401WithoutSession(t *testing.T) {
	m := dungMayChuGhi(t)
	m.capQuyen(xaA, QuyenDanhMuc)
	doiMa(t, m.goi(t, http.MethodGet, hostA, pathFields, nil), http.StatusUnauthorized)
	if m.fields.catalogueCalls != 0 {
		t.Error("catalogue read without a session")
	}
}

func TestPetitionFields_List403WrongPermission(t *testing.T) {
	// `feedback.read` is a real key an officer holds; it does not open the configuration screen.
	m := dungMayChuGhi(t)
	m.capQuyen(xaA, "feedback.read")
	doiMa(t, m.goi(t, http.MethodGet, hostA, pathFields, canBoGhi(xaA)), http.StatusForbidden)
	if m.fields.catalogueCalls != 0 {
		t.Error("catalogue read with the wrong permission")
	}
}

func TestPetitionFields_List403RightPermissionWrongCommune(t *testing.T) {
	m := dungMayChuGhi(t)
	m.capQuyen(xaA, QuyenDanhMuc)
	doiMa(t, m.goi(t, http.MethodGet, hostB, pathFields, canBoGhi(xaB)), http.StatusForbidden)
	if m.fields.catalogueCalls != 0 {
		t.Error("a grant in commune A opened commune B's configuration")
	}
}

func TestPetitionFields_List200EveryCodeInOrder(t *testing.T) {
	m := dungMayChuGhi(t)
	m.capQuyen(xaA, QuyenDanhMuc)
	w := m.goi(t, http.MethodGet, hostA, pathFields, canBoGhi(xaA))
	doiMa(t, w, http.StatusOK)
	var out petitionFieldListOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	// ALL FOUR — the switched-off and retired codes too: the configuration screen is where a commune
	// switches one back on, so hiding them there leaves no way back.
	var codes []string
	for _, it := range out.Items {
		codes = append(codes, it.Code)
	}
	if strings.Join(codes, ",") != "rac-thai,giao-thong,can-bo,ma-cu" {
		t.Fatalf("codes = %v", codes)
	}
	if out.Items[1].Enabled || out.Items[3].Active {
		t.Errorf("enabled/active not carried: %+v", out.Items)
	}
	if out.Items[0].DefaultLabel != "Rác thải – Vệ sinh môi trường" || out.Items[0].DefaultOrder != 2 {
		t.Errorf("defaults missing — the screen cannot offer 'về mặc định': %+v", out.Items[0])
	}
	if m.fields.lastCommune != xaA {
		t.Errorf("read commune %q, want %q", m.fields.lastCommune, xaA)
	}
}

// --- staff PATCH: rule 5, invariant 7 ----------------------------------------------------------------

func TestPetitionFields_EditAsksAdminLookup(t *testing.T) {
	m := dungMayChuGhi(t)
	m.goiThan(t, http.MethodPatch, hostA, pathFields+"/rac-thai", canBoGhi(xaA), bodyEditFields)
	if got := m.checker.hoiKhoaCuoi(); got != "admin.lookup" {
		t.Fatalf("route asked key %q, want \"admin.lookup\"", got)
	}
}

func TestPetitionFields_Edit401WithoutSession(t *testing.T) {
	m := dungMayChuGhi(t)
	m.capQuyen(xaA, QuyenDanhMuc)
	doiMa(t, m.goiThan(t, http.MethodPatch, hostA, pathFields+"/rac-thai", nil, bodyEditFields),
		http.StatusUnauthorized)
	if m.fields.editCalls != 0 {
		t.Error("edit ran without a session")
	}
}

func TestPetitionFields_Edit403WrongPermission(t *testing.T) {
	// `feedback.classify` decides a petition's field; it must not rename the catalogue it picks from.
	m := dungMayChuGhi(t)
	m.capQuyen(xaA, "feedback.classify")
	doiMa(t, m.goiThan(t, http.MethodPatch, hostA, pathFields+"/rac-thai", canBoGhi(xaA), bodyEditFields),
		http.StatusForbidden)
	if m.fields.editCalls != 0 {
		t.Error("edit ran with the wrong permission")
	}
}

func TestPetitionFields_Edit403RightPermissionWrongCommune(t *testing.T) {
	m := dungMayChuGhi(t)
	m.capQuyen(xaA, QuyenDanhMuc)
	doiMa(t, m.goiThan(t, http.MethodPatch, hostB, pathFields+"/rac-thai", canBoGhi(xaB), bodyEditFields),
		http.StatusForbidden)
	if m.fields.editCalls != 0 {
		t.Error("a grant in commune A edited commune B's catalogue")
	}
}

func TestPetitionFields_Edit200PassesEditAndBusinessCode(t *testing.T) {
	m := dungMayChuGhi(t)
	m.capQuyen(xaA, QuyenDanhMuc)
	w := m.goiThan(t, http.MethodPatch, hostA, pathFields+"/rac-thai", canBoGhi(xaA), bodyEditFields)
	doiMa(t, w, http.StatusOK)
	f := m.fields
	if f.lastCommune != xaA || f.lastCode != "rac-thai" {
		t.Errorf("edited commune %q code %q", f.lastCommune, f.lastCode)
	}
	if f.lastEdit.Label == nil || *f.lastEdit.Label != "Rác thải xã A" || f.lastEdit.Enabled == nil ||
		*f.lastEdit.Enabled || f.lastEdit.Order != nil {
		t.Errorf("edit not passed as sent: %+v", f.lastEdit)
	}
	// Rule 6, invariant 8: the trail names the BUSINESS code, never the internal id.
	if f.lastActor.ID != maCanBoGhi {
		t.Errorf("actor = %q, want the staff business code %q", f.lastActor.ID, maCanBoGhi)
	}
}

func TestPetitionFields_EditErrorMapping(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		want int
		key  string
	}{
		"unknown code":         {docstore.ErrDanhMucKhongTonTai, http.StatusNotFound, "not_found"},
		"platform unreachable": {app.ErrFieldCatalogueUnavailable, http.StatusServiceUnavailable, "field_catalogue_unavailable"},
		"blank label":          {domain.ErrNhanTrong, http.StatusBadRequest, "invalid_request"},
		"empty edit":           {domain.ErrFieldEditEmpty, http.StatusBadRequest, "invalid_request"},
		"store failure":        {errors.New("pg: connection refused"), http.StatusInternalServerError, "internal"},
	} {
		t.Run(name, func(t *testing.T) {
			m := dungMayChuGhi(t)
			m.capQuyen(xaA, QuyenDanhMuc)
			m.fields.err = tc.err
			w := m.goiThan(t, http.MethodPatch, hostA, pathFields+"/rac-thai", canBoGhi(xaA), bodyEditFields)
			doiMa(t, w, tc.want)
			if e := loiTra(t, w); e.Code != tc.key {
				t.Errorf("error key %q, want %q", e.Code, tc.key)
			}
			if strings.Contains(w.Body.String(), "connection refused") {
				t.Error("internal error text leaked to the client")
			}
		})
	}
}

func TestPetitionFields_EditRefusesRenamingTheCode(t *testing.T) {
	m := dungMayChuGhi(t)
	m.capQuyen(xaA, QuyenDanhMuc)
	w := m.goiThan(t, http.MethodPatch, hostA, pathFields+"/rac-thai", canBoGhi(xaA), `{"code":"rac"}`)
	doiMa(t, w, http.StatusBadRequest)
	if m.fields.editCalls != 0 {
		t.Error("a body renaming the code reached the use case")
	}
}

// --- citizen GET -------------------------------------------------------------------------------------

// fieldSessions answers three tokens: a verified session in A, one in B, and one in A WITHOUT a phone.
type fieldSessions struct{}

const (
	tokenFieldsA       = "token-fields-A-FAKE"
	tokenFieldsB       = "token-fields-B-FAKE"
	tokenFieldsNoPhone = "token-fields-no-phone-FAKE"
)

// vi-name-ok: implements the existing httpx.CitizenSessions interface method.
func (fieldSessions) TraCuu(_ context.Context, token string) (httpx.CitizenSession, bool, error) {
	switch token {
	case tokenFieldsA:
		return httpx.CitizenSession{ID: "sid-a", CitizenID: idToi, TenantID: xaA}, true, nil
	case tokenFieldsB:
		return httpx.CitizenSession{ID: "sid-b", CitizenID: idToi, TenantID: xaB}, true, nil
	case tokenFieldsNoPhone:
		return httpx.CitizenSession{ID: "sid-np", TenantID: xaA}, true, nil
	}
	return httpx.CitizenSession{}, false, nil
}

func citizenFieldsCall(t *testing.T, f *fieldCatalogueFake, token string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	RegisterCongDan(mux, DepsCongDan{
		Phieu: phieuCuaToiMau(), GuiPhieu: soPhieuMoi(), Rating: newRatingFake(),
		NhanLinhVuc: nhanLinhVucMau(), CitizenFields: f,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	var h http.Handler = mux
	h = authz.CitizenPrincipal()(h)
	h = httpx.CitizenEdge(fieldSessions{})(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)

	r := httptest.NewRequest(http.MethodGet, "https://"+hostMiniApp+CitizenFieldsPath, nil)
	r.Host = hostMiniApp
	r.RemoteAddr = "10.0.0.9:51000"
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	// A client naming its own commune must change nothing (rule 1, forbidden #2).
	r.Header.Set("X-Tenant-ID", string(xaB))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestCitizenFields_OffersOnlyEnabledActiveNonRestricted(t *testing.T) {
	f := newFieldCatalogueFake()
	w := citizenFieldsCall(t, f, tokenFieldsA)
	doiMa(t, w, http.StatusOK)
	// EXACT BYTES of the one item: switched-off, `can-bo` and retired codes absent; icon/tone as in the
	// requirement's Mini App shape; no default_label, order, enabled or any other configuration field.
	want := `{"items":[{"code":"rac-thai","label":"Rác thải xã A","icon":"Trash2","tone":"green"}]}` + "\n"
	if w.Body.String() != want {
		t.Fatalf("body =\n%s\nwant\n%s", w.Body.String(), want)
	}
	if f.lastCommune != xaA {
		t.Errorf("read commune %q — the session's is %q (the X-Tenant-ID header must be ignored)", f.lastCommune, xaA)
	}
}

func TestCitizenFields_CommuneComesFromTheSession(t *testing.T) {
	f := newFieldCatalogueFake()
	w := citizenFieldsCall(t, f, tokenFieldsB)
	doiMa(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), "Rác thải xã B") || strings.Contains(w.Body.String(), "xã A") {
		t.Errorf("commune B's session read the wrong catalogue: %s", w.Body.String())
	}
	// Undeclared icon/tone travel as null, which the Mini App replaces with its neutral defaults.
	if !strings.Contains(w.Body.String(), `"icon":null,"tone":null`) {
		t.Errorf("undeclared icon/tone not null: %s", w.Body.String())
	}
}

func TestCitizenFields_SessionWithoutPhoneMaySeeTheCatalogue(t *testing.T) {
	// XaTuPhienChiXem: configuration, nobody's records — step 1 of the form shows before the phone.
	f := newFieldCatalogueFake()
	doiMa(t, citizenFieldsCall(t, f, tokenFieldsNoPhone), http.StatusOK)
}

func TestCitizenFields_401WithoutSession(t *testing.T) {
	f := newFieldCatalogueFake()
	doiMa(t, citizenFieldsCall(t, f, ""), http.StatusUnauthorized)
	if f.citizenCalls != 0 {
		t.Error("catalogue read without a session")
	}
}

func TestCitizenFields_503WhenPlatformIsDown(t *testing.T) {
	f := newFieldCatalogueFake()
	f.err = app.ErrFieldCatalogueUnavailable
	w := citizenFieldsCall(t, f, tokenFieldsA)
	doiMa(t, w, http.StatusServiceUnavailable)
	if e := loiTra(t, w); e.Code != "field_catalogue_unavailable" {
		t.Errorf("error key %q", e.Code)
	}
}
