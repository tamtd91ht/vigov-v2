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
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// The two map-frame routes on the map assets' staff harness (checkerDanhMucGia, chuTheGhi, canBoGhi).
// The fake keeps one frame per commune and validates with the REAL domain function, so the handler's own
// half — which commune it asks for, the 422 mapping, the reply shape — is what is under test. The
// transaction and the audit entry are internal/app/map_frame_test.go.

type fakeMapFrames struct {
	byCommune map[tenant.ID]domain.MapFrame
	err       error

	calls       int
	lastCommune tenant.ID
	lastActor   audit.Actor
	lastInput   app.MapFrameInput
}

func (f *fakeMapFrames) Get(ctx context.Context) (app.MapFrameView, error) {
	f.calls++
	f.lastCommune = tenant.MustFrom(ctx)
	if f.err != nil {
		return app.MapFrameView{}, f.err
	}
	fr, ok := f.byCommune[f.lastCommune]
	if !ok {
		return app.MapFrameView{}, nil
	}
	return app.MapFrameView{Frame: fr, Bounds: fr.Bounds(), Configured: true}, nil
}

func (f *fakeMapFrames) Save(ctx context.Context, in app.MapFrameInput, actor audit.Actor) (app.MapFrameView, error) {
	f.calls++
	f.lastCommune, f.lastActor, f.lastInput = tenant.MustFrom(ctx), actor, in
	if f.err != nil {
		return app.MapFrameView{}, f.err
	}
	fr, err := domain.NormalizeMapFrame(in.CenterLat, in.CenterLng, in.RadiusKm)
	if err != nil {
		return app.MapFrameView{}, fmt.Errorf("map_frame: lưu: %w", err)
	}
	fr.UpdatedAt, fr.UpdatedBy = time.Date(2026, 10, 4, 9, 0, 0, 0, time.UTC), actor.ID
	f.byCommune[f.lastCommune] = fr
	return app.MapFrameView{Frame: fr, Bounds: fr.Bounds(), Configured: true}, nil
}

type mapFrameServer struct {
	h       http.Handler
	fake    *fakeMapFrames
	checker *checkerDanhMucGia
	keys    int
}

func newMapFrameServer(t *testing.T) *mapFrameServer {
	t.Helper()
	fake := &fakeMapFrames{byCommune: map[tenant.ID]domain.MapFrame{
		xaB: {CenterLat: 10.5, CenterLng: 106.5, RadiusKm: 5, UpdatedBy: "CB-00999"},
	}}
	checker := &checkerDanhMucGia{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	RegisterMapFrame(mux, MapFrameDeps{Checker: checker, Reader: fake, Writer: fake, Log: log})

	var h http.Handler = mux
	h = chuTheGhi(h)
	h = idem.Middleware(moKhoIdemGia(), log)(h)
	h = httpx.TenantMiddleware(thuMucMau())(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)
	return &mapFrameServer{h: h, fake: fake, checker: checker}
}

func (s *mapFrameServer) grant(commune tenant.ID, perms ...authz.Perm) {
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

func (s *mapFrameServer) call(t *testing.T, method, host string, p *authz.Principal, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	r := httptest.NewRequest(method, "https://"+host+"/api/v1/map-frame", rd)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	r.Header.Set("Content-Type", "application/json")
	s.keys++
	r.Header.Set(idem.Header, fmt.Sprintf("01JIDEMMAPFRAME%010d", s.keys))
	if p != nil {
		r = r.WithContext(context.WithValue(r.Context(), khoaChuTheGhi{}, *p))
	}
	w := httptest.NewRecorder()
	s.h.ServeHTTP(w, r)
	return w
}

const putMapFrameBody = `{"center_lat":16,"center_lng":108,"radius_km":10}`

// mapFrameRoutes — the wrong keys are real and adjacent: a content reader must not read the map's
// frame; a map READER must not set it, and neither must a map WRITER (H3: admin.lookup only).
func mapFrameRoutes() []mapAssetRoute {
	return []mapAssetRoute{
		{"get", http.MethodGet, "", "", "asset.read", "content.read", http.StatusOK},
		{"put", http.MethodPut, "", putMapFrameBody, "admin.lookup", "asset.read", http.StatusOK},
		{"put by asset writer", http.MethodPut, "", putMapFrameBody, "admin.lookup", "asset.update", http.StatusOK},
	}
}

func TestMapFrameRoutesAskForSeededKeys(t *testing.T) {
	// LITERALS: a fake checker grants any string (rule 5, invariant 3c). asset.read is seeded at
	// service-identity/migrations/0001_init.sql:287, admin.lookup at :274.
	for _, tc := range mapFrameRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newMapFrameServer(t)
			s.call(t, tc.method, hostA, canBoGhi(xaA), tc.body)
			if len(s.checker.hoiGi) == 0 || s.checker.hoiGi[0] != tc.perm {
				t.Fatalf("route asked for %v, want %q first", s.checker.hoiGi, tc.perm)
			}
		})
	}
}

func TestMapFrameRoutes_401NoSession(t *testing.T) {
	for _, tc := range mapFrameRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newMapFrameServer(t)
			s.grant(xaA, tc.perm)
			doiMa(t, s.call(t, tc.method, hostA, nil, tc.body), http.StatusUnauthorized)
			if s.fake.calls != 0 {
				t.Error("no session and the use case still ran")
			}
		})
	}
}

func TestMapFrameRoutes_403WrongPermission(t *testing.T) {
	for _, tc := range mapFrameRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newMapFrameServer(t)
			s.grant(xaA, tc.wrongPerm)
			doiMa(t, s.call(t, tc.method, hostA, canBoGhi(xaA), tc.body), http.StatusForbidden)
			if s.fake.calls != 0 {
				t.Error("wrong permission and the use case still ran")
			}
		})
	}
}

func TestMapFrameRoutes_403RightPermissionWrongCommune(t *testing.T) {
	// Signed in at commune B as a member of B, the grant in commune A.
	for _, tc := range mapFrameRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newMapFrameServer(t)
			s.grant(xaA, tc.perm)
			doiMa(t, s.call(t, tc.method, hostB, canBoGhi(xaB), tc.body), http.StatusForbidden)
			if s.fake.calls != 0 {
				t.Error("a grant in another commune was enough")
			}
		})
	}
}

func TestMapFrameRoutes_200RightPermissionRightCommune(t *testing.T) {
	for _, tc := range mapFrameRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newMapFrameServer(t)
			s.grant(xaA, tc.perm)
			doiMa(t, s.call(t, tc.method, hostA, canBoGhi(xaA), tc.body), tc.ok)
			if s.fake.lastCommune != xaA {
				t.Errorf("ran in %q, want the Host commune %q", s.fake.lastCommune, xaA)
			}
			if tc.method == http.MethodPut && (s.fake.lastActor.ID != maCanBoGhi || s.fake.lastActor.IP == "") {
				// Rule 6, invariant 8: the trail records `CB-…`, never the internal id.
				t.Errorf("actor = %+v, want the business code %q and an IP", s.fake.lastActor, maCanBoGhi)
			}
		})
	}
}

// --- payloads ------------------------------------------------------------------------------------

func TestMapFrameUnsetIsConfiguredFalseOnly(t *testing.T) {
	s := newMapFrameServer(t)
	s.grant(xaA, "asset.read")
	w := s.call(t, http.MethodGet, hostA, canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	if got := strings.TrimSpace(w.Body.String()); got != `{"configured":false}` {
		t.Errorf("unset frame = %s — want exactly {\"configured\":false}", got)
	}
}

func TestMapFrameSetCarriesBoundsLngFirst(t *testing.T) {
	s := newMapFrameServer(t)
	s.grant(xaA, "asset.read", "admin.lookup")
	doiMa(t, s.call(t, http.MethodPut, hostA, canBoGhi(xaA), putMapFrameBody), http.StatusOK)

	w := s.call(t, http.MethodGet, hostA, canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	var got struct {
		Configured bool      `json:"configured"`
		CenterLat  float64   `json:"center_lat"`
		CenterLng  float64   `json:"center_lng"`
		RadiusKm   float64   `json:"radius_km"`
		Bounds     []float64 `json:"bounds"`
		UpdatedAt  time.Time `json:"updated_at"`
		UpdatedBy  string    `json:"updated_by"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.Configured || got.CenterLat != 16 || got.CenterLng != 108 || got.RadiusKm != 10 ||
		got.UpdatedBy != maCanBoGhi || got.UpdatedAt.IsZero() {
		t.Errorf("frame = %s", w.Body.String())
	}
	// [minLng, minLat, maxLng, maxLat]: longitudes around 108, latitudes around 16.
	if len(got.Bounds) != 4 || got.Bounds[1] != 15.910168 || got.Bounds[3] != 16.089832 ||
		got.Bounds[0] >= 108 || got.Bounds[0] < 107.8 || got.Bounds[2] <= 108 || got.Bounds[2] > 108.2 {
		t.Errorf("bounds = %v — want [minLng, minLat, maxLng, maxLat]", got.Bounds)
	}
	if strings.Contains(w.Body.String(), "10.5") {
		t.Error("commune B's frame leaked into A's reply")
	}
}

func TestMapFramePutValidationEdges(t *testing.T) {
	for name, tc := range map[string]struct {
		body string
		code int
		want string
	}{
		"south edge 8.4":    {`{"center_lat":8.4,"center_lng":105,"radius_km":10}`, http.StatusOK, `"configured":true`},
		"north edge 23.4":   {`{"center_lat":23.4,"center_lng":105,"radius_km":10}`, http.StatusOK, `"configured":true`},
		"east edge 109.5":   {`{"center_lat":16,"center_lng":109.5,"radius_km":10}`, http.StatusOK, `"configured":true`},
		"east past 109.51":  {`{"center_lat":16,"center_lng":109.51,"radius_km":10}`, http.StatusUnprocessableEntity, `"center_outside_mainland"`},
		"Hoàng Sa":          {`{"center_lat":16.5,"center_lng":112,"radius_km":10}`, http.StatusUnprocessableEntity, `"center_outside_mainland"`},
		"radius 1":          {`{"center_lat":16,"center_lng":108,"radius_km":1}`, http.StatusOK, `"configured":true`},
		"radius 30":         {`{"center_lat":16,"center_lng":108,"radius_km":30}`, http.StatusOK, `"configured":true`},
		"radius 0.9":        {`{"center_lat":16,"center_lng":108,"radius_km":0.9}`, http.StatusUnprocessableEntity, `"radius_out_of_range"`},
		"radius 30.1":       {`{"center_lat":16,"center_lng":108,"radius_km":30.1}`, http.StatusUnprocessableEntity, `"radius_out_of_range"`},
		"missing radius":    {`{"center_lat":16,"center_lng":108}`, http.StatusBadRequest, `"invalid_request"`},
		"not JSON":          {`{"center_lat":`, http.StatusBadRequest, `"invalid_request"`},
		"string for number": {`{"center_lat":"16","center_lng":108,"radius_km":10}`, http.StatusBadRequest, `"invalid_request"`},
	} {
		t.Run(name, func(t *testing.T) {
			s := newMapFrameServer(t)
			s.grant(xaA, "admin.lookup")
			w := s.call(t, http.MethodPut, hostA, canBoGhi(xaA), tc.body)
			doiMa(t, w, tc.code)
			if !strings.Contains(w.Body.String(), tc.want) {
				t.Errorf("body = %s, want %s", w.Body.String(), tc.want)
			}
		})
	}
}

func TestMapFrameRefusalSentencesNameTheBounds(t *testing.T) {
	s := newMapFrameServer(t)
	s.grant(xaA, "admin.lookup")
	w := s.call(t, http.MethodPut, hostA, canBoGhi(xaA), `{"center_lat":16,"center_lng":112,"radius_km":10}`)
	if b := w.Body.String(); !strings.Contains(b, "8.4") || !strings.Contains(b, "109.5") {
		t.Errorf("centre refusal = %s", b)
	}
	w = s.call(t, http.MethodPut, hostA, canBoGhi(xaA), `{"center_lat":16,"center_lng":108,"radius_km":31}`)
	if b := w.Body.String(); !strings.Contains(b, "từ 1 đến 30 km") {
		t.Errorf("radius refusal = %s", b)
	}
}

func TestMapFrameStoreFailureIs500WithoutDetail(t *testing.T) {
	s := newMapFrameServer(t)
	s.grant(xaA, "asset.read", "admin.lookup")
	s.fake.err = errors.New("pq: connection reset")
	for _, m := range []string{http.MethodGet, http.MethodPut} {
		w := s.call(t, m, hostA, canBoGhi(xaA), putMapFrameBody)
		doiMa(t, w, http.StatusInternalServerError)
		if strings.Contains(w.Body.String(), "pq:") {
			t.Errorf("%s leaked the store error: %s", m, w.Body.String())
		}
	}
}

func TestMapFramePutWithoutBusinessCodeRefuses(t *testing.T) {
	s := newMapFrameServer(t)
	s.grant(xaA, "admin.lookup")
	p := canBoGhi(xaA)
	p.Ma = ""
	doiMa(t, s.call(t, http.MethodPut, hostA, p, putMapFrameBody), http.StatusInternalServerError)
	if s.fake.calls != 0 {
		t.Error("a write with no business code for the trail reached the use case")
	}
}
