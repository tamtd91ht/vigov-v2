package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// The nine economic-map routes, on the staff harness the external contacts use: the commune-keyed
// checker (checkerDanhMucGia), the principal injector (chuTheGhi) and the principal whose internal id and
// business code differ (canBoGhi). The fake below APPLIES the store's predicates per commune (live rows
// only) so the handler's own half — which commune it asks for, what it masks, how it maps refusals — is
// what is under test. The SQL is internal/store/map_asset_test.go; the use cases internal/app.

type fakeMapAssets struct {
	byCommune map[tenant.ID][]domain.MapAsset
	deleted   map[string]bool

	pointsErr, writeErr, fullViewErr error
	row                              domain.MapAsset
	seed                             app.MapAssetTypeSeedResult

	// noOp makes Update report "nothing written" — what the use case answers for `{}` or unchanged values.
	noOp bool

	calls        int
	fullViews    int
	lastCommune  tenant.ID
	lastActor    audit.Actor
	lastFilter   domain.MapAssetFilter
	lastInput    app.MapAssetInput
	lastPatch    app.MapAssetPatch
	lastVerified *bool
}

func (f *fakeMapAssets) note(ctx context.Context) {
	f.calls++
	f.lastCommune = tenant.MustFrom(ctx)
}

func (f *fakeMapAssets) live(ctx context.Context) []domain.MapAsset {
	var out []domain.MapAsset
	for _, a := range f.byCommune[tenant.MustFrom(ctx)] {
		if !f.deleted[a.ID] {
			out = append(out, a)
		}
	}
	return out
}

func (f *fakeMapAssets) Points(ctx context.Context, flt domain.MapAssetFilter) ([]domain.MapAssetPoint, error) {
	f.note(ctx)
	f.lastFilter = flt
	if f.pointsErr != nil {
		return nil, f.pointsErr
	}
	var out []domain.MapAssetPoint
	for _, a := range f.live(ctx) {
		out = append(out, domain.MapAssetPoint{ID: a.ID, AssetTypeCode: a.AssetTypeCode, Name: a.Name, Status: a.Status,
			Verified: a.Verified, Lat: a.Lat, Lng: a.Lng})
	}
	return out, nil
}

func (f *fakeMapAssets) List(ctx context.Context, flt domain.MapAssetFilter, _ page.Request) (page.Result[domain.MapAsset], error) {
	f.note(ctx)
	f.lastFilter = flt
	res := page.NewResult[domain.MapAsset]()
	res.Items = append(res.Items, f.live(ctx)...)
	return res, nil
}

func (f *fakeMapAssets) ByID(ctx context.Context, id string) (domain.MapAsset, error) {
	f.note(ctx)
	for _, a := range f.live(ctx) {
		if a.ID == id {
			return a, nil
		}
	}
	return domain.MapAsset{}, commsstore.ErrMapAssetNotFound
}

func (f *fakeMapAssets) Summary(ctx context.Context) (domain.MapAssetSummary, error) {
	f.note(ctx)
	s := domain.MapAssetSummary{ByType: []domain.MapAssetTypeCount{}}
	counts := map[string]*domain.MapAssetTypeCount{}
	for _, a := range f.live(ctx) {
		c := counts[a.AssetTypeCode]
		if c == nil {
			s.ByType = append(s.ByType, domain.MapAssetTypeCount{AssetTypeCode: a.AssetTypeCode})
			c = &s.ByType[len(s.ByType)-1]
			counts[a.AssetTypeCode] = c
		}
		c.Count++
		s.Total++
		if a.Verified {
			c.Verified++
			s.Verified++
		}
	}
	return s, nil
}

func (f *fakeMapAssets) Create(ctx context.Context, in app.MapAssetInput, actor audit.Actor) (domain.MapAsset, error) {
	f.note(ctx)
	f.lastInput, f.lastActor = in, actor
	return f.row, f.writeErr
}

func (f *fakeMapAssets) Update(ctx context.Context, _ string, p app.MapAssetPatch, actor audit.Actor) (domain.MapAsset, bool, error) {
	f.note(ctx)
	f.lastPatch, f.lastActor = p, actor
	return f.row, f.writeErr == nil && !f.noOp, f.writeErr
}

func (f *fakeMapAssets) Delete(ctx context.Context, id, _ string, actor audit.Actor) error {
	f.note(ctx)
	f.lastActor = actor
	if f.writeErr != nil {
		return f.writeErr
	}
	f.deleted[id] = true
	return nil
}

func (f *fakeMapAssets) SetConfirmation(ctx context.Context, _ string, verified bool, actor audit.Actor) (domain.MapAsset, bool, error) {
	f.note(ctx)
	f.lastVerified, f.lastActor = &verified, actor
	r := f.row
	// The use case's rule: the same state twice writes and audits nothing.
	written := f.writeErr == nil && r.Verified != verified
	r.Verified = verified
	return r, written, f.writeErr
}

func (f *fakeMapAssets) RecordFullView(ctx context.Context, _ domain.MapAsset, actor audit.Actor) error {
	f.fullViews++
	f.lastActor = actor
	return f.fullViewErr
}

func (f *fakeMapAssets) SeedDefaults(ctx context.Context, actor audit.Actor) (app.MapAssetTypeSeedResult, error) {
	f.note(ctx)
	f.lastActor = actor
	return f.seed, f.writeErr
}

func mapAssetFixture() *fakeMapAssets {
	at := time.Date(2026, 10, 4, 3, 0, 0, 0, time.UTC)
	emp := 12
	full := domain.MapAsset{ID: "ma-a-1", AssetTypeCode: "doanh-nghiep", Name: "Công ty TNHH May Thăng Bình",
		Address: "Cụm công nghiệp Hà Lam", ResidentialUnitID: "thon-1", Lat: 15.730507, Lng: 108.37811,
		Representative: "Nguyễn Văn Hùng", Phone: "0900000000", Status: "dang-hoat-dong", Verified: true,
		VerifiedAt: &at, VerifiedBy: "CB-00123", TaxCode: "0101234567", IndustryCode: "14", EmployeeCount: &emp,
		CustomValues: map[string]json.RawMessage{"legal_form": json.RawMessage(`"tnhh"`)}, CreatedAt: at, UpdatedAt: at}
	return &fakeMapAssets{
		byCommune: map[tenant.ID][]domain.MapAsset{
			xaA: {
				full,
				{ID: "ma-a-2", AssetTypeCode: "cho", Name: "Chợ Bình Trị", Lat: 15.7, Lng: 108.3, Status: "dang-hoat-dong",
					CustomValues: map[string]json.RawMessage{}, CreatedAt: at, UpdatedAt: at},
			},
			xaB: {{ID: "ma-b-1", AssetTypeCode: "cho", Name: "CHỢ CỦA XÃ B", Lat: 10, Lng: 106, Status: "dang-hoat-dong",
				CustomValues: map[string]json.RawMessage{}}},
		},
		deleted: map[string]bool{},
		row:     full,
		seed: app.MapAssetTypeSeedResult{
			Created:        []app.MapAssetTypeRef{{Code: "doanh-nghiep", Label: "Doanh nghiệp"}},
			SkippedDeleted: []app.MapAssetTypeRef{{Code: "ocop", Label: "Sản phẩm OCOP"}},
		},
	}
}

type mapAssetServer struct {
	h       http.Handler
	fake    *fakeMapAssets
	checker *checkerDanhMucGia
	keys    int
}

func newMapAssetServer(t *testing.T) *mapAssetServer {
	t.Helper()
	fake := mapAssetFixture()
	checker := &checkerDanhMucGia{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	RegisterMapAssets(mux, MapAssetDeps{Checker: checker, Reader: fake, Writer: fake, TypeDefaults: fake, Log: log})

	var h http.Handler = mux
	h = chuTheGhi(h)
	h = idem.Middleware(moKhoIdemGia(), log)(h)
	h = httpx.TenantMiddleware(thuMucMau())(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)
	return &mapAssetServer{h: h, fake: fake, checker: checker}
}

func (s *mapAssetServer) grant(commune tenant.ID, perms ...authz.Perm) {
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

func (s *mapAssetServer) call(t *testing.T, method, host, path string, p *authz.Principal, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	r := httptest.NewRequest(method, "https://"+host+path, rd)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	r.Header.Set("Content-Type", "application/json")
	// A fresh key per call: one test sends several creates, and a repeated key is (correctly) a replay.
	s.keys++
	r.Header.Set(idem.Header, fmt.Sprintf("01JIDEMMAPASSET%010d", s.keys))
	if p != nil {
		r = r.WithContext(context.WithValue(r.Context(), khoaChuTheGhi{}, *p))
	}
	w := httptest.NewRecorder()
	s.h.ServeHTTP(w, r)
	return w
}

type mapAssetRoute struct {
	name, method, path, body string
	perm, wrongPerm          authz.Perm
	ok                       int
}

const createMapAssetBody = `{"asset_type_code":"doanh-nghiep","name":"Công ty","lat":15.73,"lng":108.37}`

// mapAssetRoutes — every permission case is asserted on ALL nine. The wrong keys are real and adjacent:
// a content reader must not read the map; a map READER must not write it; a map WRITER must not sow the
// catalogue.
func mapAssetRoutes() []mapAssetRoute {
	return []mapAssetRoute{
		{"seed", http.MethodPost, "/api/v1/map-asset-types/defaults", "", "admin.lookup", "asset.update", http.StatusOK},
		{"points", http.MethodGet, "/api/v1/map-asset-points", "", "asset.read", "content.read", http.StatusOK},
		{"list", http.MethodGet, "/api/v1/map-assets", "", "asset.read", "content.read", http.StatusOK},
		{"detail", http.MethodGet, "/api/v1/map-assets/ma-a-1", "", "asset.read", "content.read", http.StatusOK},
		{"summary", http.MethodGet, "/api/v1/map-asset-summary", "", "asset.read", "content.read", http.StatusOK},
		{"create", http.MethodPost, "/api/v1/map-assets", createMapAssetBody, "asset.update", "asset.read", http.StatusCreated},
		{"update", http.MethodPatch, "/api/v1/map-assets/ma-a-1", `{"name":"Tên mới"}`, "asset.update", "asset.read", http.StatusOK},
		{"delete", http.MethodDelete, "/api/v1/map-assets/ma-a-2", `{"reason":"trùng"}`, "asset.update", "asset.read", http.StatusNoContent},
		{"verify", http.MethodPost, "/api/v1/map-assets/ma-a-1/confirmation", `{"verified":true}`, "asset.update", "asset.read", http.StatusOK},
	}
}

func TestMapAssetRoutesAskForSeededKeys(t *testing.T) {
	// LITERALS: a fake checker grants any string, so a key the `quyen` table lacks would stay green here
	// while answering 403 to every account (rule 5, invariant 3c). asset.read / asset.update are seeded at
	// service-identity/migrations/0001_init.sql:287-288, admin.lookup at :281.
	for _, tc := range mapAssetRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newMapAssetServer(t)
			s.call(t, tc.method, hostA, tc.path, canBoGhi(xaA), tc.body)
			if len(s.checker.hoiGi) == 0 || s.checker.hoiGi[0] != tc.perm {
				t.Fatalf("route asked for %v, want %q first", s.checker.hoiGi, tc.perm)
			}
		})
	}
}

func TestMapAssetRoutes_401NoSession(t *testing.T) {
	for _, tc := range mapAssetRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newMapAssetServer(t)
			s.grant(xaA, tc.perm)
			doiMa(t, s.call(t, tc.method, hostA, tc.path, nil, tc.body), http.StatusUnauthorized)
			if s.fake.calls != 0 {
				t.Error("no session and the store/use case still ran")
			}
		})
	}
}

func TestMapAssetRoutes_403WrongPermission(t *testing.T) {
	for _, tc := range mapAssetRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newMapAssetServer(t)
			s.grant(xaA, tc.wrongPerm)
			doiMa(t, s.call(t, tc.method, hostA, tc.path, canBoGhi(xaA), tc.body), http.StatusForbidden)
			if s.fake.calls != 0 {
				t.Error("wrong permission and the store/use case still ran")
			}
		})
	}
}

func TestMapAssetRoutes_403RightPermissionWrongCommune(t *testing.T) {
	// Signed in at commune B as a member of B, the grant in commune A. A checker that ignored the commune
	// would let A's officer read and edit B's map.
	for _, tc := range mapAssetRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newMapAssetServer(t)
			s.grant(xaA, tc.perm)
			doiMa(t, s.call(t, tc.method, hostB, tc.path, canBoGhi(xaB), tc.body), http.StatusForbidden)
			if s.fake.calls != 0 {
				t.Error("a grant in another commune was enough")
			}
		})
	}
}

func TestMapAssetRoutes_2xxRightPermissionRightCommune(t *testing.T) {
	for _, tc := range mapAssetRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newMapAssetServer(t)
			s.grant(xaA, tc.perm)
			doiMa(t, s.call(t, tc.method, hostA, tc.path, canBoGhi(xaA), tc.body), tc.ok)
			if s.fake.lastCommune != xaA {
				t.Errorf("ran in %q, want the Host commune %q", s.fake.lastCommune, xaA)
			}
			if tc.method != http.MethodGet && s.fake.lastActor.ID != maCanBoGhi {
				// Rule 6, invariant 8: the trail records `CB-…`, never the internal id.
				t.Errorf("actor = %q, want the business code %q", s.fake.lastActor.ID, maCanBoGhi)
			}
		})
	}
}

// --- payloads ------------------------------------------------------------------------------------

func TestMapAssetPointsAreGeoJSONLngLatWithoutPersonalData(t *testing.T) {
	s := newMapAssetServer(t)
	s.grant(xaA, "asset.read")
	w := s.call(t, http.MethodGet, hostA, "/api/v1/map-asset-points?asset_type_code=doanh-nghiep&asset_type_code=cho&verified=true&q=Th%C4%83ng", canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	body := w.Body.String()
	for _, leaked := range []string{"0900000000", "Hùng", "0101234567", "Hà Lam", "CHỢ CỦA XÃ B"} {
		if strings.Contains(body, leaked) {
			t.Errorf("points payload carries %q: %s", leaked, body)
		}
	}
	var got struct {
		Type     string `json:"type"`
		Features []struct {
			Type     string `json:"type"`
			ID       string `json:"id"`
			Geometry struct {
				Type        string    `json:"type"`
				Coordinates []float64 `json:"coordinates"`
			} `json:"geometry"`
			Properties map[string]any `json:"properties"`
		} `json:"features"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Type != "FeatureCollection" || len(got.Features) != 2 {
		t.Fatalf("= %s", body)
	}
	f := got.Features[0]
	if f.Type != "Feature" || f.Geometry.Type != "Point" || len(f.Geometry.Coordinates) != 2 ||
		f.Geometry.Coordinates[0] != 108.37811 || f.Geometry.Coordinates[1] != 15.730507 {
		t.Errorf("feature = %+v — coordinates must be [lng, lat] (RFC 7946)", f)
	}
	if len(f.Properties) != 5 {
		t.Errorf("properties = %v — want exactly id, asset_type_code, name, status, verified", f.Properties)
	}
	flt := s.fake.lastFilter
	if len(flt.AssetTypeCodes) != 2 || flt.Verified == nil || !*flt.Verified || flt.Query != "Thăng" {
		t.Errorf("filter reaching the store = %+v", flt)
	}
}

func TestMapAssetPointsCapAndBadFilter(t *testing.T) {
	s := newMapAssetServer(t)
	s.grant(xaA, "asset.read")
	s.fake.pointsErr = fmt.Errorf("wrapped: %w", commsstore.ErrTooManyMapAssetPoints)
	w := s.call(t, http.MethodGet, hostA, "/api/v1/map-asset-points", canBoGhi(xaA), "")
	doiMa(t, w, http.StatusUnprocessableEntity)
	if !strings.Contains(w.Body.String(), `"too_many_points"`) {
		t.Errorf("body = %s", w.Body.String())
	}
	s.fake.calls = 0
	for _, q := range []string{"verified=yes", "status=dong", "industry_code=4", "asset_type_code=Bad%20Code"} {
		w := s.call(t, http.MethodGet, hostA, "/api/v1/map-asset-points?"+q, canBoGhi(xaA), "")
		doiMa(t, w, http.StatusBadRequest)
		if !strings.Contains(w.Body.String(), `"invalid_filter"`) {
			t.Errorf("%s: %s", q, w.Body.String())
		}
	}
	if s.fake.calls != 0 {
		t.Error("a refused filter still reached the store")
	}
}

func TestMapAssetListIsAlwaysMasked(t *testing.T) {
	s := newMapAssetServer(t)
	s.grant(xaA, "asset.read", "asset.update")
	w := s.call(t, http.MethodGet, hostA, "/api/v1/map-assets", canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	var got page.Result[map[string]any]
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 2 {
		t.Fatalf("items = %v", got.Items)
	}
	row := got.Items[0]
	if row["phone"] != "09****0000" || row["representative"] != "Nguyễn V. H." {
		t.Errorf("row = %v — the list masks even for asset.update", row)
	}
	if _, ok := row["tax_code"]; ok {
		t.Error("the list carries the tax code")
	}
	if _, ok := got.Items[1]["phone"]; ok {
		t.Error("absent phone rendered — `—` on the screen means the key is absent")
	}
	if s.fake.fullViews != 0 {
		t.Error("a list read was audited as a full view")
	}
	doiMa(t, s.call(t, http.MethodGet, hostA, "/api/v1/map-assets?cursor=xyz", canBoGhi(xaA), ""), http.StatusBadRequest)
}

func TestMapAssetDetailMaskingFollowsAssetUpdate(t *testing.T) {
	t.Run("asset.read sees masked", func(t *testing.T) {
		s := newMapAssetServer(t)
		s.grant(xaA, "asset.read")
		w := s.call(t, http.MethodGet, hostA, "/api/v1/map-assets/ma-a-1", canBoGhi(xaA), "")
		doiMa(t, w, http.StatusOK)
		var got map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &got)
		if got["phone"] != "09****0000" || got["tax_code"] != "010****567" || got["representative"] != "Nguyễn V. H." || got["masked"] != true {
			t.Errorf("detail = %v", got)
		}
		if got["custom_values"].(map[string]any)["legal_form"] != "tnhh" {
			t.Errorf("custom values = %v", got["custom_values"])
		}
		if s.fake.fullViews != 0 {
			t.Error("a masked read was audited as a full view")
		}
	})
	t.Run("asset.update sees full, audited first", func(t *testing.T) {
		s := newMapAssetServer(t)
		s.grant(xaA, "asset.read", "asset.update")
		w := s.call(t, http.MethodGet, hostA, "/api/v1/map-assets/ma-a-1", canBoGhi(xaA), "")
		doiMa(t, w, http.StatusOK)
		var got map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &got)
		if got["phone"] != "0900000000" || got["tax_code"] != "0101234567" || got["masked"] != false {
			t.Errorf("detail = %v", got)
		}
		if s.fake.fullViews != 1 || s.fake.lastActor.ID != maCanBoGhi || s.fake.lastActor.IP == "" {
			t.Errorf("full view audited %d times by %+v", s.fake.fullViews, s.fake.lastActor)
		}
	})
	t.Run("trail fails, nothing disclosed", func(t *testing.T) {
		s := newMapAssetServer(t)
		s.grant(xaA, "asset.read", "asset.update")
		s.fake.fullViewErr = errors.New("db down")
		w := s.call(t, http.MethodGet, hostA, "/api/v1/map-assets/ma-a-1", canBoGhi(xaA), "")
		doiMa(t, w, http.StatusInternalServerError)
		if strings.Contains(w.Body.String(), "0900000000") {
			t.Error("disclosed with no trail")
		}
	})
	t.Run("no business code, masked", func(t *testing.T) {
		s := newMapAssetServer(t)
		s.grant(xaA, "asset.read", "asset.update")
		p := canBoGhi(xaA)
		p.Ma = ""
		w := s.call(t, http.MethodGet, hostA, "/api/v1/map-assets/ma-a-1", p, "")
		doiMa(t, w, http.StatusOK)
		if strings.Contains(w.Body.String(), "0900000000") || s.fake.fullViews != 0 {
			t.Error("full view granted to a principal the trail cannot name")
		}
	})
	t.Run("another commune's id is 404", func(t *testing.T) {
		s := newMapAssetServer(t)
		s.grant(xaA, "asset.read")
		doiMa(t, s.call(t, http.MethodGet, hostA, "/api/v1/map-assets/ma-b-1", canBoGhi(xaA), ""), http.StatusNotFound)
	})
}

func TestMapAssetSoftDeleteHidesEverywhere(t *testing.T) {
	s := newMapAssetServer(t)
	s.grant(xaA, "asset.read", "asset.update")
	doiMa(t, s.call(t, http.MethodDelete, hostA, "/api/v1/map-assets/ma-a-1", canBoGhi(xaA), `{"reason":"trùng"}`), http.StatusNoContent)

	if w := s.call(t, http.MethodGet, hostA, "/api/v1/map-asset-points", canBoGhi(xaA), ""); strings.Contains(w.Body.String(), "ma-a-1") {
		t.Error("deleted asset on the map")
	}
	if w := s.call(t, http.MethodGet, hostA, "/api/v1/map-assets", canBoGhi(xaA), ""); strings.Contains(w.Body.String(), "ma-a-1") {
		t.Error("deleted asset in the list")
	}
	doiMa(t, s.call(t, http.MethodGet, hostA, "/api/v1/map-assets/ma-a-1", canBoGhi(xaA), ""), http.StatusNotFound)
	w := s.call(t, http.MethodGet, hostA, "/api/v1/map-asset-summary", canBoGhi(xaA), "")
	var sum mapAssetSummaryOut
	_ = json.Unmarshal(w.Body.Bytes(), &sum)
	if sum.Total != 1 || sum.Verified != 0 || sum.VerifiedRatio != 0 {
		t.Errorf("summary after delete = %+v", sum)
	}
}

func TestMapAssetSummaryShape(t *testing.T) {
	s := newMapAssetServer(t)
	s.grant(xaA, "asset.read")
	w := s.call(t, http.MethodGet, hostA, "/api/v1/map-asset-summary", canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	var sum mapAssetSummaryOut
	if err := json.Unmarshal(w.Body.Bytes(), &sum); err != nil {
		t.Fatal(err)
	}
	if sum.Total != 2 || sum.Verified != 1 || sum.VerifiedRatio != 0.5 || len(sum.ByType) != 2 {
		t.Errorf("summary = %+v", sum)
	}
}

func TestMapAssetWriteBodiesAndRefusals(t *testing.T) {
	s := newMapAssetServer(t)
	s.grant(xaA, "asset.update")
	doiMa(t, s.call(t, http.MethodPost, hostA, "/api/v1/map-assets", canBoGhi(xaA),
		`{"asset_type_code":"doanh-nghiep","name":"C","lat":15.73,"lng":108.37,"custom_values":{"staff_count":9007199254740993,"legal_form":"tnhh"}}`),
		http.StatusCreated)
	if got := string(s.fake.lastInput.CustomValues["staff_count"]); got != "9007199254740993" {
		t.Errorf("integer travelled through float64: %s", got)
	}
	if s.fake.lastInput.Lat == nil || *s.fake.lastInput.Lat != 15.73 {
		t.Errorf("input = %+v", s.fake.lastInput)
	}

	s.fake.calls = 0
	doiMa(t, s.call(t, http.MethodPost, hostA, "/api/v1/map-assets", canBoGhi(xaA),
		`{"asset_type_code":"cho","name":"C","lat":1,"lng":2,"verified":true}`), http.StatusBadRequest)
	doiMa(t, s.call(t, http.MethodPatch, hostA, "/api/v1/map-assets/ma-a-1", canBoGhi(xaA), `{"verified":false}`), http.StatusBadRequest)
	doiMa(t, s.call(t, http.MethodPost, hostA, "/api/v1/map-assets/ma-a-1/confirmation", canBoGhi(xaA), `{}`), http.StatusBadRequest)
	if s.fake.calls != 0 {
		t.Error("`verified` outside its route reached the use case")
	}

	doiMa(t, s.call(t, http.MethodPatch, hostA, "/api/v1/map-assets/ma-a-1", canBoGhi(xaA),
		`{"address":"","custom_values":{"legal_form":null}}`), http.StatusOK)
	p := s.fake.lastPatch
	if p.Address == nil || *p.Address != "" || p.Name != nil || string(p.CustomValues["legal_form"]) != "null" {
		t.Errorf("patch = %+v", p)
	}
	doiMa(t, s.call(t, http.MethodPost, hostA, "/api/v1/map-assets/ma-a-1/confirmation", canBoGhi(xaA), `{"verified":false}`), http.StatusOK)
	if s.fake.lastVerified == nil || *s.fake.lastVerified {
		t.Error("verified:false did not reach the use case as false")
	}
}

// A write reply is UNMASKED only when the use case wrote — its own entry, same transaction, records the
// act. A no-op wrote no entry, so an unmasked reply would be an untraced full read of the representative,
// phone and tax code: anybody holding asset.update could read them all by PATCHing `{}`.
func TestMapAssetWriteReplyIsUnmaskedOnlyWhenWritten(t *testing.T) {
	decode := func(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
		t.Helper()
		doiMa(t, w, http.StatusOK)
		var got map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		return got
	}
	masked := func(t *testing.T, got map[string]any) {
		t.Helper()
		// The same masking as GET detail without asset.update.
		if got["masked"] != true || got["phone"] != "09****0000" || got["tax_code"] != "010****567" ||
			got["representative"] != "Nguyễn V. H." {
			t.Errorf("no-op reply = %v — want masked", got)
		}
	}

	t.Run("PATCH {} is masked and audits nothing", func(t *testing.T) {
		s := newMapAssetServer(t)
		s.grant(xaA, "asset.update")
		s.fake.noOp = true
		masked(t, decode(t, s.call(t, http.MethodPatch, hostA, "/api/v1/map-assets/ma-a-1", canBoGhi(xaA), `{}`)))
		if s.fake.fullViews != 0 {
			t.Error("a no-op PATCH recorded a full view")
		}
	})
	t.Run("confirmation with the value it already has is masked", func(t *testing.T) {
		s := newMapAssetServer(t)
		s.grant(xaA, "asset.update")
		// The fixture row is verified already.
		masked(t, decode(t, s.call(t, http.MethodPost, hostA, "/api/v1/map-assets/ma-a-1/confirmation", canBoGhi(xaA), `{"verified":true}`)))
	})
	t.Run("a real edit is unmasked", func(t *testing.T) {
		s := newMapAssetServer(t)
		s.grant(xaA, "asset.update")
		got := decode(t, s.call(t, http.MethodPatch, hostA, "/api/v1/map-assets/ma-a-1", canBoGhi(xaA), `{"name":"Tên mới"}`))
		if got["masked"] != false || got["phone"] != "0900000000" || got["tax_code"] != "0101234567" {
			t.Errorf("written reply = %v — want unmasked", got)
		}
	})
	t.Run("a real confirmation change is unmasked", func(t *testing.T) {
		s := newMapAssetServer(t)
		s.grant(xaA, "asset.update")
		got := decode(t, s.call(t, http.MethodPost, hostA, "/api/v1/map-assets/ma-a-1/confirmation", canBoGhi(xaA), `{"verified":false}`))
		if got["masked"] != false || got["phone"] != "0900000000" {
			t.Errorf("written reply = %v — want unmasked", got)
		}
	})
	t.Run("address and coordinates are never masked", func(t *testing.T) {
		// ADR 0072 sửa đổi 04/10/2026 — chủ dự án chốt hộ kinh doanh hiển thị như doanh nghiệp.
		s := newMapAssetServer(t)
		s.grant(xaA, "asset.update")
		s.fake.noOp = true
		got := decode(t, s.call(t, http.MethodPatch, hostA, "/api/v1/map-assets/ma-a-1", canBoGhi(xaA), `{}`))
		if got["address"] != "Cụm công nghiệp Hà Lam" || got["lat"] != 15.730507 || got["lng"] != 108.37811 {
			t.Errorf("masked reply = %v — address and pin stay whole", got)
		}
	})
}

func TestMapAssetErrorMapping(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		code int
		want string
	}{
		"not found":     {fmt.Errorf("w: %w", commsstore.ErrMapAssetNotFound), http.StatusNotFound, "not_found"},
		"tax taken":     {fmt.Errorf("w: %w", commsstore.ErrMapAssetTaxCodeTaken), http.StatusConflict, "tax_code_taken"},
		"type missing":  {fmt.Errorf("w: %w", commsstore.ErrMapAssetTypeUnavailable), http.StatusUnprocessableEntity, "asset_type_unavailable"},
		"custom value":  {fmt.Errorf("w: %w", &domain.CustomValueError{FieldCode: "legal_form", Err: domain.ErrCustomValueNotAnOption}), http.StatusUnprocessableEntity, "invalid_custom_values"},
		"bad phone":     {fmt.Errorf("w: %w", domain.ErrMapAssetPhoneShape), http.StatusBadRequest, "invalid_request"},
		"no location":   {domain.ErrMapAssetLocationMissing, http.StatusBadRequest, "invalid_request"},
		"store failure": {errors.New("pq: connection reset"), http.StatusInternalServerError, "internal"},
	} {
		t.Run(name, func(t *testing.T) {
			s := newMapAssetServer(t)
			s.grant(xaA, "asset.update")
			s.fake.writeErr = tc.err
			w := s.call(t, http.MethodPost, hostA, "/api/v1/map-assets", canBoGhi(xaA), createMapAssetBody)
			doiMa(t, w, tc.code)
			if !strings.Contains(w.Body.String(), `"`+tc.want+`"`) || strings.Contains(w.Body.String(), "pq:") {
				t.Errorf("body = %s", w.Body.String())
			}
		})
	}
}

func TestSeedMapAssetTypesReply(t *testing.T) {
	s := newMapAssetServer(t)
	s.grant(xaA, "admin.lookup")
	w := s.call(t, http.MethodPost, hostA, "/api/v1/map-asset-types/defaults", canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	var got seedMapAssetTypesOut
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.CreatedCount != 1 || len(got.Created) != 1 || got.SkippedExisting == nil || len(got.SkippedDeleted) != 1 {
		t.Errorf("reply = %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"skipped_existing":[]`) {
		t.Errorf("empty list not [] — %s", w.Body.String())
	}
	s.fake.writeErr = fmt.Errorf("w: %w", commsstore.ErrDanhMucDayTran)
	doiMa(t, s.call(t, http.MethodPost, hostA, "/api/v1/map-asset-types/defaults", canBoGhi(xaA), ""), http.StatusConflict)
}
