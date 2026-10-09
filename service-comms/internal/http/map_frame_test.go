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
	"slices"
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

// The three map-frame routes on the map assets' staff harness (checkerDanhMucGia, chuTheGhi, canBoGhi).
// The fake keeps one frame per commune and validates with the REAL domain functions in the use case's
// order (notice, then values), so the handler's own half — which commune it asks for, what it passes
// on, the 422/503 mapping, the reply shape — is what is under test. The transaction, the audit entry
// and "nothing written" are internal/app/map_frame_test.go.

type fakeMapFrames struct {
	byCommune map[tenant.ID]domain.MapFrame
	// def is the platform default every commune falls back to; nil = none.
	def *domain.MapFrame
	err error

	calls       int
	lastCommune tenant.ID
	lastActor   audit.Actor
	lastInput   app.MapFrameInput
	lastNotice  string
}

func (f *fakeMapFrames) view(c tenant.ID) app.MapFrameView {
	var def *app.MapFrameDefaultView
	if f.def != nil {
		def = &app.MapFrameDefaultView{Frame: *f.def, Bounds: f.def.Bounds()}
	}
	if fr, ok := f.byCommune[c]; ok && fr.Enabled {
		return app.MapFrameView{Frame: fr, Bounds: fr.Bounds(), Configured: true,
			Source: domain.MapFrameSourceCommune, Default: def}
	}
	if def == nil {
		return app.MapFrameView{}
	}
	return app.MapFrameView{Frame: def.Frame, Bounds: def.Bounds, Configured: true,
		Source: domain.MapFrameSourceDefault, Default: def}
}

func (f *fakeMapFrames) Get(ctx context.Context) (app.MapFrameView, error) {
	f.calls++
	f.lastCommune = tenant.MustFrom(ctx)
	if f.err != nil {
		return app.MapFrameView{}, f.err
	}
	return f.view(f.lastCommune), nil
}

func (f *fakeMapFrames) Save(ctx context.Context, in app.MapFrameInput, actor audit.Actor) (app.MapFrameView, error) {
	f.calls++
	f.lastCommune, f.lastActor, f.lastInput = tenant.MustFrom(ctx), actor, in
	if f.err != nil {
		return app.MapFrameView{}, f.err
	}
	if err := domain.CheckMapFrameNotice(in.NoticeVersion); err != nil {
		return app.MapFrameView{}, fmt.Errorf("map_frame: lưu: %w", err)
	}
	fr, err := domain.NormalizeMapFrame(in.CenterLat, in.CenterLng, in.RadiusKm)
	if err != nil {
		return app.MapFrameView{}, fmt.Errorf("map_frame: lưu: %w", err)
	}
	fr.Enabled = true
	fr.UpdatedAt, fr.UpdatedBy = time.Date(2026, 10, 4, 9, 0, 0, 0, time.UTC), actor.ID
	f.byCommune[f.lastCommune] = fr
	return f.view(f.lastCommune), nil
}

func (f *fakeMapFrames) Reset(ctx context.Context, notice string, actor audit.Actor) (app.MapFrameView, error) {
	f.calls++
	f.lastCommune, f.lastActor, f.lastNotice = tenant.MustFrom(ctx), actor, notice
	if f.err != nil {
		return app.MapFrameView{}, f.err
	}
	if err := domain.CheckMapFrameNotice(notice); err != nil {
		return app.MapFrameView{}, fmt.Errorf("map_frame: về mặc định: %w", err)
	}
	if fr, ok := f.byCommune[f.lastCommune]; ok {
		fr.Enabled = false
		f.byCommune[f.lastCommune] = fr
	}
	return f.view(f.lastCommune), nil
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
		xaB: {CenterLat: 10.5, CenterLng: 106.5, RadiusKm: 5, Enabled: true, UpdatedBy: "CB-00999"},
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

const (
	mapFramePath      = "/api/v1/map-frame"
	mapFrameResetPath = "/api/v1/map-frame/reset"
)

func (s *mapFrameServer) call(t *testing.T, method, host string, p *authz.Principal, body string) *httptest.ResponseRecorder {
	t.Helper()
	return s.callPath(t, method, mapFramePath, host, p, body)
}

func (s *mapFrameServer) callPath(t *testing.T, method, path, host string, p *authz.Principal, body string) *httptest.ResponseRecorder {
	t.Helper()
	if path == "" {
		path = mapFramePath
	}
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	r := httptest.NewRequest(method, "https://"+host+path, rd)
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

// The current notice version, spelled out: the tests pin ADR 0072 K6's value, not the constant.
const (
	putMapFrameBody   = `{"center_lat":16,"center_lng":108,"radius_km":10,"notice_version":"2026-10-04.1"}`
	resetMapFrameBody = `{"notice_version":"2026-10-04.1"}`
)

func putBody(lat, lng, r string) string {
	return fmt.Sprintf(`{"center_lat":%s,"center_lng":%s,"radius_km":%s,"notice_version":"2026-10-04.1"}`, lat, lng, r)
}

// mapFrameRoutes — the wrong keys are real and adjacent: a content reader must not read the map's
// frame; a map READER must not set it or reset it, and neither must a map WRITER (H3/K4: admin.lookup).
// The GET is listed twice, once per key it accepts (ADR 0072 §Trả lời 09/10/2026): each of 401 · 403
// neither key · 403 other commune · 200 then holds for a petition officer exactly as for a map reader.
func mapFrameRoutes() []mapAssetRoute {
	return []mapAssetRoute{
		{"get", http.MethodGet, "", "", "asset.read", "content.read", http.StatusOK},
		{"get by petition reader", http.MethodGet, "", "", "feedback.read", "content.read", http.StatusOK},
		{"put", http.MethodPut, "", putMapFrameBody, "admin.lookup", "asset.read", http.StatusOK},
		{"put by asset writer", http.MethodPut, "", putMapFrameBody, "admin.lookup", "asset.update", http.StatusOK},
		{"reset", http.MethodPost, mapFrameResetPath, resetMapFrameBody, "admin.lookup", "asset.read", http.StatusOK},
		{"reset by asset writer", http.MethodPost, mapFrameResetPath, resetMapFrameBody, "admin.lookup", "asset.update", http.StatusOK},
	}
}

func TestMapFrameRoutesAskForSeededKeys(t *testing.T) {
	// LITERALS: a fake checker grants any string (rule 5, invariant 3c). asset.read is seeded at
	// service-identity/migrations/0001_init.sql:287, admin.lookup at :281.
	for _, tc := range mapFrameRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newMapFrameServer(t)
			s.callPath(t, tc.method, tc.path, hostA, canBoGhi(xaA), tc.body)
			if !slices.Contains(s.checker.hoiGi, tc.perm) {
				t.Fatalf("route asked for %v, want it to ask for %q", s.checker.hoiGi, tc.perm)
			}
		})
	}
}

// TestMapFrameGetAsksForExactlyTheTwoKeys pins the OR: the GET asks for asset.read and feedback.read
// and nothing else — a third key slipping in would widen the read, a missing one would lock the
// petition maps out. The writes still ask for admin.lookup alone (the decision opened the READ only).
// feedback.read is seeded at service-identity/migrations/0001_init.sql:299.
func TestMapFrameGetAsksForExactlyTheTwoKeys(t *testing.T) {
	s := newMapFrameServer(t)
	doiMa(t, s.call(t, http.MethodGet, hostA, canBoGhi(xaA), ""), http.StatusForbidden)
	if !slices.Equal(s.checker.hoiGi, []authz.Perm{"asset.read", "feedback.read"}) {
		t.Fatalf("GET asked for %v, want [asset.read feedback.read]", s.checker.hoiGi)
	}
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPut, mapFramePath, putMapFrameBody},
		{http.MethodPost, mapFrameResetPath, resetMapFrameBody},
	} {
		s := newMapFrameServer(t)
		s.grant(xaA, "feedback.read")
		doiMa(t, s.callPath(t, tc.method, tc.path, hostA, canBoGhi(xaA), tc.body), http.StatusForbidden)
		if !slices.Equal(s.checker.hoiGi, []authz.Perm{"admin.lookup"}) {
			t.Errorf("%s %s asked for %v, want [admin.lookup] alone", tc.method, tc.path, s.checker.hoiGi)
		}
		if s.fake.calls != 0 {
			t.Errorf("%s %s ran for a petition reader", tc.method, tc.path)
		}
	}
}

func TestMapFrameRoutes_401NoSession(t *testing.T) {
	for _, tc := range mapFrameRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newMapFrameServer(t)
			s.grant(xaA, tc.perm)
			doiMa(t, s.callPath(t, tc.method, tc.path, hostA, nil, tc.body), http.StatusUnauthorized)
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
			doiMa(t, s.callPath(t, tc.method, tc.path, hostA, canBoGhi(xaA), tc.body), http.StatusForbidden)
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
			doiMa(t, s.callPath(t, tc.method, tc.path, hostB, canBoGhi(xaB), tc.body), http.StatusForbidden)
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
			doiMa(t, s.callPath(t, tc.method, tc.path, hostA, canBoGhi(xaA), tc.body), tc.ok)
			if s.fake.lastCommune != xaA {
				t.Errorf("ran in %q, want the Host commune %q", s.fake.lastCommune, xaA)
			}
			if tc.method != http.MethodGet && (s.fake.lastActor.ID != maCanBoGhi || s.fake.lastActor.IP == "") {
				// Rule 6, invariant 8: the trail records `CB-…`, never the internal id.
				t.Errorf("actor = %+v, want the business code %q and an IP", s.fake.lastActor, maCanBoGhi)
			}
		})
	}
}

// --- payloads ------------------------------------------------------------------------------------

// mapFrameReply is the whole reply shape web-admin reads.
type mapFrameReply struct {
	Configured *bool      `json:"configured"`
	Source     string     `json:"source"`
	CenterLat  *float64   `json:"center_lat"`
	CenterLng  *float64   `json:"center_lng"`
	RadiusKm   *float64   `json:"radius_km"`
	Bounds     []float64  `json:"bounds"`
	UpdatedAt  *time.Time `json:"updated_at"`
	UpdatedBy  string     `json:"updated_by"`
	Default    *struct {
		CenterLat float64   `json:"center_lat"`
		CenterLng float64   `json:"center_lng"`
		RadiusKm  float64   `json:"radius_km"`
		Bounds    []float64 `json:"bounds"`
	} `json:"default"`
	RecommendedRadiusKm *float64  `json:"recommended_radius_km"`
	UsualRadiusKm       []float64 `json:"usual_radius_km"`
	MaxRadiusKm         *float64  `json:"max_radius_km"`
	NoticeVersion       string    `json:"notice_version"`
}

func readReply(t *testing.T, w *httptest.ResponseRecorder) mapFrameReply {
	t.Helper()
	var got mapFrameReply
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("%v: %s", err, w.Body.String())
	}
	return got
}

func assertHints(t *testing.T, got mapFrameReply) {
	t.Helper()
	if got.RecommendedRadiusKm == nil || *got.RecommendedRadiusKm != 10 ||
		len(got.UsualRadiusKm) != 2 || got.UsualRadiusKm[0] != 3 || got.UsualRadiusKm[1] != 20 ||
		got.MaxRadiusKm == nil || *got.MaxRadiusKm != 50 || got.NoticeVersion != "2026-10-04.1" {
		t.Errorf("hints = %+v", got)
	}
}

func TestMapFrameUnsetIsConfiguredFalseWithHintsOnly(t *testing.T) {
	s := newMapFrameServer(t)
	s.grant(xaA, "asset.read")
	w := s.call(t, http.MethodGet, hostA, canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	got := readReply(t, w)
	if got.Configured == nil || *got.Configured || got.Source != "" || got.CenterLat != nil || got.Bounds != nil ||
		got.Default != nil || got.UpdatedAt != nil {
		t.Errorf("unset frame = %s — want configured=false and no frame field", w.Body.String())
	}
	// The form shown under "Chưa đặt" needs the ceiling and the notice version.
	assertHints(t, got)
}

func TestMapFrameSetCarriesBoundsLngFirst(t *testing.T) {
	s := newMapFrameServer(t)
	s.grant(xaA, "asset.read", "admin.lookup")
	doiMa(t, s.call(t, http.MethodPut, hostA, canBoGhi(xaA), putMapFrameBody), http.StatusOK)

	w := s.call(t, http.MethodGet, hostA, canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	got := readReply(t, w)
	if got.Configured == nil || !*got.Configured || got.Source != "commune" || *got.CenterLat != 16 ||
		*got.CenterLng != 108 || *got.RadiusKm != 10 || got.UpdatedBy != maCanBoGhi || got.UpdatedAt == nil ||
		got.Default != nil {
		t.Errorf("frame = %s", w.Body.String())
	}
	// [minLng, minLat, maxLng, maxLat]: longitudes around 108, latitudes around 16.
	if len(got.Bounds) != 4 || got.Bounds[1] != 15.910168 || got.Bounds[3] != 16.089832 ||
		got.Bounds[0] >= 108 || got.Bounds[0] < 107.8 || got.Bounds[2] <= 108 || got.Bounds[2] > 108.2 {
		t.Errorf("bounds = %v — want [minLng, minLat, maxLng, maxLat]", got.Bounds)
	}
	assertHints(t, got)
	if strings.Contains(w.Body.String(), "10.5") {
		t.Error("commune B's frame leaked into A's reply")
	}
}

func TestMapFrameDefaultSourceCarriesTheDefault(t *testing.T) {
	s := newMapFrameServer(t)
	s.grant(xaA, "asset.read")
	def := domain.MapFrame{CenterLat: 21.0285, CenterLng: 105.8542, RadiusKm: 12}
	s.fake.def = &def
	w := s.call(t, http.MethodGet, hostA, canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	got := readReply(t, w)
	b := def.Bounds()
	wantBounds := []float64{b.MinLng, b.MinLat, b.MaxLng, b.MaxLat}
	if got.Configured == nil || !*got.Configured || got.Source != "default" || *got.CenterLat != 21.0285 ||
		got.UpdatedAt != nil || got.UpdatedBy != "" {
		t.Errorf("reply = %s", w.Body.String())
	}
	if got.Default == nil || got.Default.CenterLat != 21.0285 || got.Default.RadiusKm != 12 ||
		fmt.Sprint(got.Default.Bounds) != fmt.Sprint(wantBounds) || fmt.Sprint(got.Bounds) != fmt.Sprint(wantBounds) {
		t.Errorf("default = %+v, bounds %v — want %v from domain.MapFrame.Bounds", got.Default, got.Bounds, wantBounds)
	}
	assertHints(t, got)
}

func TestMapFrameOwnFrameCarriesTheDefaultToo(t *testing.T) {
	// "Về mặc định" needs to show what it returns to while the commune's own frame applies.
	s := newMapFrameServer(t)
	s.grant(xaA, "asset.read", "admin.lookup")
	def := domain.MapFrame{CenterLat: 21, CenterLng: 105.8, RadiusKm: 8}
	s.fake.def = &def
	for _, c := range []struct{ method, body string }{
		{http.MethodPut, putMapFrameBody},
		{http.MethodGet, ""},
	} {
		w := s.call(t, c.method, hostA, canBoGhi(xaA), c.body)
		doiMa(t, w, http.StatusOK)
		got := readReply(t, w)
		if got.Source != "commune" || *got.CenterLat != 16 || got.Default == nil || got.Default.CenterLat != 21 ||
			len(got.Default.Bounds) != 4 {
			t.Errorf("%s = %s — want the own frame applied and the default attached", c.method, w.Body.String())
		}
	}
}

func TestMapFramePutValidationEdges(t *testing.T) {
	for name, tc := range map[string]struct {
		body string
		code int
		want string
	}{
		"south edge 8.4":     {putBody("8.4", "105", "10"), http.StatusOK, `"configured":true`},
		"north edge 23.4":    {putBody("23.4", "105", "10"), http.StatusOK, `"configured":true`},
		"east edge 109.5":    {putBody("16", "109.5", "10"), http.StatusOK, `"configured":true`},
		"east past 109.51":   {putBody("16", "109.51", "10"), http.StatusUnprocessableEntity, `"center_outside_mainland"`},
		"Hoàng Sa":           {putBody("16.5", "112", "10"), http.StatusUnprocessableEntity, `"center_outside_mainland"`},
		"radius 0":           {putBody("16", "108", "0"), http.StatusUnprocessableEntity, `"radius_out_of_range"`},
		"radius 0.1":         {putBody("16", "108", "0.1"), http.StatusOK, `"configured":true`},
		"radius 30.1":        {putBody("16", "108", "30.1"), http.StatusOK, `"configured":true`},
		"radius 50":          {putBody("16", "108", "50"), http.StatusOK, `"configured":true`},
		"radius 50.1":        {putBody("16", "108", "50.1"), http.StatusUnprocessableEntity, `"radius_out_of_range"`},
		"radius 25 off-band": {putBody("16", "108", "25"), http.StatusOK, `"configured":true`}, // outside 3–20: a hint, never a refusal
		"missing radius":     {`{"center_lat":16,"center_lng":108,"notice_version":"2026-10-04.1"}`, http.StatusBadRequest, `"invalid_request"`},
		"not JSON":           {`{"center_lat":`, http.StatusBadRequest, `"invalid_request"`},
		"string for number":  {`{"center_lat":"16","center_lng":108,"radius_km":10}`, http.StatusBadRequest, `"invalid_request"`},
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

func TestMapFramePutWithoutTheCurrentNoticeIs422AndWritesNothing(t *testing.T) {
	for name, body := range map[string]string{
		"missing":   `{"center_lat":16,"center_lng":108,"radius_km":10}`,
		"empty":     `{"center_lat":16,"center_lng":108,"radius_km":10,"notice_version":""}`,
		"null":      `{"center_lat":16,"center_lng":108,"radius_km":10,"notice_version":null}`,
		"stale":     `{"center_lat":16,"center_lng":108,"radius_km":10,"notice_version":"2026-09-01.1"}`,
		"ticked":    `{"center_lat":16,"center_lng":108,"radius_km":10,"notice_version":"true"}`,
		"bad frame": `{"center_lat":16,"center_lng":112,"radius_km":10}`, // the notice is the first gate
	} {
		t.Run(name, func(t *testing.T) {
			s := newMapFrameServer(t)
			s.grant(xaA, "admin.lookup")
			w := s.call(t, http.MethodPut, hostA, canBoGhi(xaA), body)
			doiMa(t, w, http.StatusUnprocessableEntity)
			if !strings.Contains(w.Body.String(), `"notice_not_acknowledged"`) ||
				!strings.Contains(w.Body.String(), "2026-10-04.1") {
				t.Errorf("body = %s — want notice_not_acknowledged naming the current version", w.Body.String())
			}
			if _, ok := s.fake.byCommune[xaA]; ok {
				t.Error("a frame was stored without the acknowledgement")
			}
		})
	}
}

func TestMapFramePutPassesTheNoticeVersionOn(t *testing.T) {
	s := newMapFrameServer(t)
	s.grant(xaA, "admin.lookup")
	doiMa(t, s.call(t, http.MethodPut, hostA, canBoGhi(xaA), putMapFrameBody), http.StatusOK)
	if s.fake.lastInput.NoticeVersion != "2026-10-04.1" {
		t.Errorf("input = %+v — the acknowledgement must reach the use case, which audits it", s.fake.lastInput)
	}
}

func TestMapFrameRefusalSentencesNameTheBounds(t *testing.T) {
	s := newMapFrameServer(t)
	s.grant(xaA, "admin.lookup")
	w := s.call(t, http.MethodPut, hostA, canBoGhi(xaA), putBody("16", "112", "10"))
	if b := w.Body.String(); !strings.Contains(b, "8.4") || !strings.Contains(b, "109.5") {
		t.Errorf("centre refusal = %s", b)
	}
	w = s.call(t, http.MethodPut, hostA, canBoGhi(xaA), putBody("16", "108", "51"))
	if b := w.Body.String(); !strings.Contains(b, "lớn hơn 0 và không quá 50 km") {
		t.Errorf("radius refusal = %s", b)
	}
}

func TestMapFrameStoreFailureIs500WithoutDetail(t *testing.T) {
	s := newMapFrameServer(t)
	s.grant(xaA, "asset.read", "admin.lookup")
	s.fake.err = errors.New("pq: connection reset")
	for _, c := range []struct{ method, path, body string }{
		{http.MethodGet, mapFramePath, ""},
		{http.MethodPut, mapFramePath, putMapFrameBody},
		{http.MethodPost, mapFrameResetPath, resetMapFrameBody},
	} {
		w := s.callPath(t, c.method, c.path, hostA, canBoGhi(xaA), c.body)
		doiMa(t, w, http.StatusInternalServerError)
		if strings.Contains(w.Body.String(), "pq:") {
			t.Errorf("%s %s leaked the store error: %s", c.method, c.path, w.Body.String())
		}
	}
}

func TestMapFramePlatformUnavailableIs503WithoutDetail(t *testing.T) {
	s := newMapFrameServer(t)
	s.grant(xaA, "asset.read", "admin.lookup")
	s.fake.err = fmt.Errorf("%w: rpc error: code = Unavailable desc = dial tcp 10.1.2.3:9000",
		app.ErrMapFrameDefaultUnavailable)
	for _, c := range []struct{ method, path, body string }{
		{http.MethodGet, mapFramePath, ""},
		{http.MethodPost, mapFrameResetPath, resetMapFrameBody},
	} {
		w := s.callPath(t, c.method, c.path, hostA, canBoGhi(xaA), c.body)
		doiMa(t, w, http.StatusServiceUnavailable)
		if b := w.Body.String(); !strings.Contains(b, `"map_frame_default_unavailable"`) || strings.Contains(b, "10.1.2.3") ||
			strings.Contains(b, "center_lat") {
			t.Errorf("%s %s = %s — want the code, no detail, no guessed frame", c.method, c.path, b)
		}
	}
}

func TestMapFramePutWithoutBusinessCodeRefuses(t *testing.T) {
	for _, c := range []struct{ method, path, body string }{
		{http.MethodPut, mapFramePath, putMapFrameBody},
		{http.MethodPost, mapFrameResetPath, resetMapFrameBody},
	} {
		s := newMapFrameServer(t)
		s.grant(xaA, "admin.lookup")
		p := canBoGhi(xaA)
		p.Ma = ""
		doiMa(t, s.callPath(t, c.method, c.path, hostA, p, c.body), http.StatusInternalServerError)
		if s.fake.calls != 0 {
			t.Errorf("%s: a write with no business code for the trail reached the use case", c.method)
		}
	}
}

// --- "Về mặc định" -------------------------------------------------------------------------------

func TestMapFrameResetReturnsTheEffectiveFrame(t *testing.T) {
	s := newMapFrameServer(t)
	s.grant(xaA, "asset.read", "admin.lookup")
	def := domain.MapFrame{CenterLat: 21, CenterLng: 105.8, RadiusKm: 8}
	s.fake.def = &def
	doiMa(t, s.call(t, http.MethodPut, hostA, canBoGhi(xaA), putMapFrameBody), http.StatusOK)

	w := s.callPath(t, http.MethodPost, mapFrameResetPath, hostA, canBoGhi(xaA), resetMapFrameBody)
	doiMa(t, w, http.StatusOK)
	got := readReply(t, w)
	if got.Source != "default" || *got.CenterLat != 21 || got.Default == nil {
		t.Errorf("reset reply = %s — want the platform default now applying", w.Body.String())
	}
	if s.fake.lastNotice != "2026-10-04.1" || s.fake.lastActor.ID != maCanBoGhi {
		t.Errorf("notice %q, actor %+v", s.fake.lastNotice, s.fake.lastActor)
	}
	// Idempotent at the route: the second reset answers the same state.
	w = s.callPath(t, http.MethodPost, mapFrameResetPath, hostA, canBoGhi(xaA), resetMapFrameBody)
	doiMa(t, w, http.StatusOK)
	if readReply(t, w).Source != "default" {
		t.Errorf("second reset = %s", w.Body.String())
	}
}

func TestMapFrameResetWithNoDefaultIsConfiguredFalse(t *testing.T) {
	s := newMapFrameServer(t)
	s.grant(xaA, "admin.lookup")
	w := s.callPath(t, http.MethodPost, mapFrameResetPath, hostA, canBoGhi(xaA), resetMapFrameBody)
	doiMa(t, w, http.StatusOK)
	if got := readReply(t, w); got.Configured == nil || *got.Configured {
		t.Errorf("reply = %s", w.Body.String())
	}
}

func TestMapFrameResetWithoutTheCurrentNoticeIs422(t *testing.T) {
	for name, body := range map[string]string{
		"empty object": `{}`,
		"empty":        `{"notice_version":""}`,
		"stale":        `{"notice_version":"2026-09-01.1"}`,
	} {
		t.Run(name, func(t *testing.T) {
			s := newMapFrameServer(t)
			s.grant(xaA, "admin.lookup")
			s.fake.byCommune[xaA] = domain.MapFrame{CenterLat: 16, CenterLng: 108, RadiusKm: 10, Enabled: true}
			w := s.callPath(t, http.MethodPost, mapFrameResetPath, hostA, canBoGhi(xaA), body)
			doiMa(t, w, http.StatusUnprocessableEntity)
			if !strings.Contains(w.Body.String(), `"notice_not_acknowledged"`) {
				t.Errorf("body = %s", w.Body.String())
			}
			if !s.fake.byCommune[xaA].Enabled {
				t.Error("the frame was switched off without the acknowledgement")
			}
		})
	}
}

func TestMapFrameResetNotJSONIs400(t *testing.T) {
	s := newMapFrameServer(t)
	s.grant(xaA, "admin.lookup")
	doiMa(t, s.callPath(t, http.MethodPost, mapFrameResetPath, hostA, canBoGhi(xaA), `{"notice_version":`), http.StatusBadRequest)
	if s.fake.calls != 0 {
		t.Error("a malformed body reached the use case")
	}
}

func TestMapFrameNoDeleteRoute(t *testing.T) {
	// "Về mặc định" is a POST that keeps the row; a DELETE must not exist (rule 7; 0016's trigger).
	s := newMapFrameServer(t)
	s.grant(xaA, "admin.lookup")
	if w := s.call(t, http.MethodDelete, hostA, canBoGhi(xaA), ""); w.Code == http.StatusOK || w.Code == http.StatusNoContent {
		t.Errorf("DELETE /api/v1/map-frame answered %d", w.Code)
	}
	if s.fake.calls != 0 {
		t.Error("DELETE reached the use case")
	}
}
