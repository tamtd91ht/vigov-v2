package http

// The disaster-alert routes. Staff: the four cases of rule 5 invariant 7 on EVERY route (401 · 403
// wrong permission · 403 right permission wrong commune · 2xx), over the external-contacts harness
// (checkerDanhMucGia, chuTheGhi, canBoGhi). Public: host resolution, identical empty bytes, rate limit —
// the news harness (ckNenTang, memCounter).

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/ratelimit"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

var alertStart = time.Date(2026, 10, 11, 1, 0, 0, 0, time.UTC)

// fakeDisasterAlerts stands in for the store and the use case, recording the commune and the actor.
type fakeDisasterAlerts struct {
	byCommune map[tenant.ID][]domain.DisasterAlert
	row       domain.DisasterAlert
	err       error

	calls       int
	lastCommune tenant.ID
	lastActor   audit.Actor
	lastInput   app.DisasterAlertInput
	lastPatch   app.DisasterAlertPatch
	lastID      string
	lastReason  string
	lastNow     time.Time
}

func (f *fakeDisasterAlerts) note(ctx context.Context) {
	f.calls++
	f.lastCommune = tenant.MustFrom(ctx)
}

func (f *fakeDisasterAlerts) List(ctx context.Context, _ page.Request) (page.Result[domain.DisasterAlert], error) {
	f.note(ctx)
	out := page.NewResult[domain.DisasterAlert]()
	out.Items = append(out.Items, f.byCommune[tenant.MustFrom(ctx)]...)
	return out, f.err
}

func (f *fakeDisasterAlerts) ByID(ctx context.Context, id string) (domain.DisasterAlert, error) {
	f.note(ctx)
	f.lastID = id
	return f.row, f.err
}

func (f *fakeDisasterAlerts) InForce(ctx context.Context, now time.Time) ([]domain.DisasterAlert, error) {
	f.note(ctx)
	f.lastNow = now
	return f.byCommune[tenant.MustFrom(ctx)], f.err
}

func (f *fakeDisasterAlerts) Create(ctx context.Context, in app.DisasterAlertInput, actor audit.Actor) (domain.DisasterAlert, error) {
	f.note(ctx)
	f.lastInput, f.lastActor = in, actor
	return f.row, f.err
}

func (f *fakeDisasterAlerts) Update(ctx context.Context, id string, p app.DisasterAlertPatch, actor audit.Actor) (domain.DisasterAlert, error) {
	f.note(ctx)
	f.lastID, f.lastPatch, f.lastActor = id, p, actor
	return f.row, f.err
}

func (f *fakeDisasterAlerts) End(ctx context.Context, id string, actor audit.Actor) (domain.DisasterAlert, error) {
	f.note(ctx)
	f.lastID, f.lastActor = id, actor
	return f.row, f.err
}

func (f *fakeDisasterAlerts) Delete(ctx context.Context, id, reason string, actor audit.Actor) error {
	f.note(ctx)
	f.lastID, f.lastReason, f.lastActor = id, reason, actor
	return f.err
}

func alertFixture() *fakeDisasterAlerts {
	a := domain.DisasterAlert{ID: "da-a-1", Kind: domain.DisasterKindFlood, Title: "Lũ <b>sông</b> Hồng",
		Detail: "Sơ tán ngay", EffectiveFrom: alertStart,
		DangerZone:        []domain.DangerZonePoint{{X: 10, Y: 10}, {X: 50, Y: 10}, {X: 30, Y: 40}},
		EvacuationPoints:  []domain.EvacuationPoint{{Name: "Trường tiểu học", Phone: "0900000000"}},
		EmergencyContacts: []domain.EmergencyContact{{Role: "Trực ban", Name: "Nguyễn Văn A", Phone: "0900000000"}}}
	return &fakeDisasterAlerts{
		byCommune: map[tenant.ID][]domain.DisasterAlert{
			xaA: {a},
			xaB: {{ID: "da-b-1", Kind: domain.DisasterKindStorm, Title: "CẢNH BÁO XÃ B", Detail: "x", EffectiveFrom: alertStart}},
		},
		row: a,
	}
}

type alertServer struct {
	h       http.Handler
	fake    *fakeDisasterAlerts
	checker *checkerDanhMucGia
}

func newAlertServer(t *testing.T) *alertServer {
	t.Helper()
	fake := alertFixture()
	checker := &checkerDanhMucGia{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	RegisterDisasterAlerts(mux, DisasterAlertDeps{Checker: checker, Reader: fake, Writer: fake, Log: log,
		Now: func() time.Time { return alertStart.Add(time.Hour) }})
	var h http.Handler = mux
	h = chuTheGhi(h)
	h = idem.Middleware(moKhoIdemGia(), log)(h)
	h = httpx.TenantMiddleware(thuMucMau())(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)
	return &alertServer{h: h, fake: fake, checker: checker}
}

func (s *alertServer) grant(commune tenant.ID, perms ...authz.Perm) {
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

func (s *alertServer) call(t *testing.T, method, host, path string, p *authz.Principal, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	r := httptest.NewRequest(method, "https://"+host+path, rd)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set(idem.Header, "01JIDEMPOTENCYKEYALERTS00")
	if p != nil {
		r = r.WithContext(context.WithValue(r.Context(), khoaChuTheGhi{}, *p))
	}
	w := httptest.NewRecorder()
	s.h.ServeHTTP(w, r)
	return w
}

const alertPath = "/api/v1/disaster-alerts"

const validAlertBody = `{"kind":"lu","title":"Lũ","detail":"Sơ tán","effective_from":"2026-10-11T08:00:00+07:00",` +
	`"danger_zone":[{"x":10,"y":10},{"x":50,"y":10},{"x":30,"y":40}],` +
	`"evacuation_points":[{"name":"Trường","capacity":200}],` +
	`"emergency_contacts":[{"role":"Trực ban","name":"Nguyễn Văn A","phone":"0900000000"}]}`

type alertRoute struct {
	name, method, path, body string
	perm, wrongPerm          authz.Perm
	ok                       int
}

func alertRoutes() []alertRoute {
	return []alertRoute{
		{"LIST", http.MethodGet, alertPath, "", "content.read", "asset.read", http.StatusOK},
		{"GET", http.MethodGet, alertPath + "/da-a-1", "", "content.read", "asset.read", http.StatusOK},
		{"POST", http.MethodPost, alertPath, validAlertBody, "content.update", "content.read", http.StatusCreated},
		{"PATCH", http.MethodPatch, alertPath + "/da-a-1", `{"title":"Lũ lớn"}`, "content.update", "content.read", http.StatusOK},
		{"ENDING", http.MethodPost, alertPath + "/da-a-1/ending", "", "content.update", "content.read", http.StatusOK},
		{"DELETE", http.MethodDelete, alertPath + "/da-a-1", `{"reason":"đăng nhầm"}`, "content.update", "content.read", http.StatusNoContent},
	}
}

func TestDisasterAlertRoutesAskForSeededKeys(t *testing.T) {
	// LITERALS: a fake checker grants any string (rule 5, invariant 3c).
	for _, tc := range alertRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newAlertServer(t)
			s.call(t, tc.method, hostA, tc.path, canBoGhi(xaA), tc.body)
			if got := s.checker.hoiKhoaCuoi(); got != tc.perm {
				t.Fatalf("route asked for %q, want %q", got, tc.perm)
			}
		})
	}
}

func TestDisasterAlertRoutes_401NoSession(t *testing.T) {
	for _, tc := range alertRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newAlertServer(t)
			s.grant(xaA, tc.perm)
			doiMa(t, s.call(t, tc.method, hostA, tc.path, nil, tc.body), http.StatusUnauthorized)
			if s.fake.calls != 0 {
				t.Error("no session and the store/use case still ran")
			}
		})
	}
}

func TestDisasterAlertRoutes_403WrongPermission(t *testing.T) {
	for _, tc := range alertRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newAlertServer(t)
			s.grant(xaA, tc.wrongPerm)
			doiMa(t, s.call(t, tc.method, hostA, tc.path, canBoGhi(xaA), tc.body), http.StatusForbidden)
			if s.fake.calls != 0 {
				t.Error("wrong permission and the store/use case still ran")
			}
		})
	}
}

func TestDisasterAlertRoutes_403RightPermissionWrongCommune(t *testing.T) {
	for _, tc := range alertRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newAlertServer(t)
			s.grant(xaA, tc.perm)
			doiMa(t, s.call(t, tc.method, hostB, tc.path, canBoGhi(xaB), tc.body), http.StatusForbidden)
			if s.fake.calls != 0 {
				t.Error("a grant in another commune was enough")
			}
		})
	}
}

func TestDisasterAlertRoutes_200RightPermissionRightCommune(t *testing.T) {
	for _, tc := range alertRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newAlertServer(t)
			s.grant(xaA, tc.perm)
			doiMa(t, s.call(t, tc.method, hostA, tc.path, canBoGhi(xaA), tc.body), tc.ok)
			if s.fake.lastCommune != xaA {
				t.Errorf("ran in %q, want the Host commune %q", s.fake.lastCommune, xaA)
			}
			if tc.method != http.MethodGet && s.fake.lastActor.ID != maCanBoGhi {
				t.Errorf("actor = %q, want the business code %q (rule 6, invariant 8)", s.fake.lastActor.ID, maCanBoGhi)
			}
		})
	}
}

func TestCreateDisasterAlertDecodesTheBody(t *testing.T) {
	s := newAlertServer(t)
	s.grant(xaA, "content.update")
	doiMa(t, s.call(t, http.MethodPost, hostA, alertPath, canBoGhi(xaA), validAlertBody), http.StatusCreated)
	in := s.fake.lastInput
	if in.Kind != "lu" || !in.EffectiveFrom.Equal(alertStart) || !in.EffectiveTo.IsZero() ||
		len(in.DangerZone) != 3 || len(in.EvacuationPoints) != 1 || *in.EvacuationPoints[0].Capacity != 200 ||
		in.EmergencyContacts[0].Phone != "0900000000" {
		t.Errorf("input = %+v", in)
	}
}

func TestPatchDisasterAlertClearsEffectiveTo(t *testing.T) {
	s := newAlertServer(t)
	s.grant(xaA, "content.update")
	doiMa(t, s.call(t, http.MethodPatch, hostA, alertPath+"/da-a-1", canBoGhi(xaA), `{"effective_to":""}`), http.StatusOK)
	if p := s.fake.lastPatch.EffectiveTo; p == nil || !p.IsZero() {
		t.Errorf("`effective_to: \"\"` must reach the use case as a pointer to the zero time: %v", p)
	}
	if s.fake.lastPatch.Title != nil || s.fake.lastPatch.DangerZone != nil {
		t.Error("absent keys must reach the use case as nil")
	}
}

func TestDisasterAlertRefusalsMapTo4xx(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want int
		code string
	}{
		{commsstore.ErrDisasterAlertNotFound, http.StatusNotFound, "not_found"},
		{domain.ErrDisasterEnded, http.StatusConflict, "alert_ended"},
		{domain.ErrDisasterWindowInverted, http.StatusBadRequest, "invalid_request"},
		{domain.ErrDisasterContacts, http.StatusBadRequest, "invalid_request"},
		{errors.New("pq: boom"), http.StatusInternalServerError, "internal"},
	} {
		s := newAlertServer(t)
		s.grant(xaA, "content.update")
		s.fake.err = tc.err
		w := s.call(t, http.MethodPatch, hostA, alertPath+"/da-a-1", canBoGhi(xaA), `{"title":"x"}`)
		doiMa(t, w, tc.want)
		body := w.Body.String()
		if !strings.Contains(body, `"code":"`+tc.code+`"`) || strings.Contains(body, "pq:") ||
			strings.Contains(body, "disaster_alert:") {
			t.Errorf("%v: body %s", tc.err, body)
		}
	}
}

func TestStaffAlertCarriesDerivedInForce(t *testing.T) {
	s := newAlertServer(t)
	s.grant(xaA, "content.read")
	w := s.call(t, http.MethodGet, hostA, alertPath+"/da-a-1", canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	var got map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got["in_force"] != true || got["id"] != "da-a-1" {
		t.Errorf("detail = %v", got)
	}
}

// --- public -----------------------------------------------------------------------------------

func publicAlertServer(t *testing.T, nt *ckNenTang, f *fakeDisasterAlerts, c *memCounter) http.Handler {
	t.Helper()
	if c == nil {
		c = &memCounter{}
	}
	return publicAlertServerOver(t, nt, f, c)
}

// publicAlertServerOver mounts the public route over any reader — the fake directly, or the production
// cache around it.
func publicAlertServerOver(t *testing.T, nt *ckNenTang, alerts PublicDisasterAlertsReader, c *memCounter) http.Handler {
	t.Helper()
	if c == nil {
		c = &memCounter{}
	}
	mux := http.NewServeMux()
	RegisterPublicDisasterAlerts(mux, PublicDisasterAlertDeps{Xa: nt, Alerts: alerts, Limiter: alertLimiterOn(c),
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Now: func() time.Time { return alertStart.Add(time.Hour) }})
	return mux
}

// alertLimiterOn is the production policy of the public alert read over a test counter.
func alertLimiterOn(c ratelimit.Counter) *ratelimit.Limiter {
	l, err := ratelimit.New(c, ratelimit.PublicDisasterAlertsRead)
	if err != nil {
		panic(err)
	}
	return l
}

func publicAlertGet(h http.Handler, host, remote string, extra ...string) *httptest.ResponseRecorder {
	q := ""
	if host != "" {
		q = "?host=" + url.QueryEscape(host)
	}
	q += strings.Join(extra, "")
	r := httptest.NewRequest(http.MethodGet, "https://comms.api.vigov.vn"+PublicDisasterAlertsPath+q, nil)
	if remote != "" {
		r.RemoteAddr = remote
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestPublicDisasterAlertsPathMatchesRoute(t *testing.T) {
	mux := http.NewServeMux()
	RegisterPublicDisasterAlerts(mux, PublicDisasterAlertDeps{Xa: &ckNenTang{}, Alerts: alertFixture(), Limiter: ckLimiter()})
	if _, pattern := mux.Handler(httptest.NewRequest(http.MethodGet, PublicDisasterAlertsPath, nil)); pattern != "GET "+PublicDisasterAlertsPath {
		t.Fatalf("%s matched %q", PublicDisasterAlertsPath, pattern)
	}
}

func TestPublicDisasterAlertsCommuneIsolationAndShape(t *testing.T) {
	f := alertFixture()
	h := publicAlertServer(t, &ckNenTang{}, f, nil)
	w := publicAlertGet(h, ckHostA, "")
	doiMa(t, w, http.StatusOK)
	if f.lastCommune != xaA || !f.lastNow.Equal(alertStart.Add(time.Hour)) {
		t.Fatalf("asked in %q at %v", f.lastCommune, f.lastNow)
	}
	var got struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || len(got.Items) != 1 {
		t.Fatalf("body %s", w.Body.String())
	}
	a := got.Items[0]
	// Plain text; the headline falls back to the title so the red banner is never empty.
	if a["title"] != "Lũ sông Hồng" || a["headline"] != "Lũ sông Hồng" || a["kind"] != "lu" {
		t.Errorf("alert = %v", a)
	}
	for _, k := range []string{"danger_zone", "evacuation_points", "emergency_contacts", "effective_from", "detail"} {
		if _, ok := a[k]; !ok {
			t.Errorf("missing %q", k)
		}
	}
	for _, banned := range []string{"<b>", "XÃ B", "da-b-1", "created_by", "updated_by", "ended_by"} {
		if strings.Contains(w.Body.String(), banned) {
			t.Errorf("public body holds %q", banned)
		}
	}
}

func TestPublicDisasterAlertsUnknownHostIsEmptyAndIdentical(t *testing.T) {
	empty := &fakeDisasterAlerts{byCommune: map[tenant.ID][]domain.DisasterAlert{}}
	reference := publicAlertGet(publicAlertServer(t, &ckNenTang{}, empty, nil), ckHostA, "")
	doiMa(t, reference, http.StatusOK)
	if strings.TrimSpace(reference.Body.String()) != `{"items":[]}` {
		t.Fatalf("empty = %q", reference.Body.String())
	}
	for _, host := range []string{ckHostKhongCo, ckHostNgung, ckHostRieng} {
		f := alertFixture()
		w := publicAlertGet(publicAlertServer(t, &ckNenTang{}, f, nil), host, "")
		if w.Code != http.StatusOK || w.Body.String() != reference.Body.String() || f.calls != 0 {
			t.Errorf("%s: %d %q (store asked %d)", host, w.Code, w.Body.String(), f.calls)
		}
	}
}

func TestPublicDisasterAlertsInvalidHostAndOutage(t *testing.T) {
	f := alertFixture()
	w := publicAlertGet(publicAlertServer(t, &ckNenTang{}, f, nil), "", "", "?host=not%20a%20host")
	doiMa(t, w, http.StatusBadRequest)
	w = publicAlertGet(publicAlertServer(t, &ckNenTang{chet: true}, f, nil), ckHostA, "")
	doiMa(t, w, http.StatusServiceUnavailable)
	if f.calls != 0 {
		t.Error("the store was read")
	}
}

func TestPublicDisasterAlertsOverCeilingStillServes(t *testing.T) {
	f := alertFixture()
	f.err = commsstore.ErrTooManyAlertsInForce
	w := publicAlertGet(publicAlertServer(t, &ckNenTang{}, f, nil), ckHostA, "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "da-a-1") {
		t.Fatalf("over the ceiling the newest alerts must still be served: %d %s", w.Code, w.Body.String())
	}
	f.err = errors.New("pq: boom")
	doiMa(t, publicAlertGet(publicAlertServer(t, &ckNenTang{}, f, nil), ckHostA, ""), http.StatusInternalServerError)
}

func TestPublicDisasterAlertsRateLimited(t *testing.T) {
	c := &memCounter{}
	f := alertFixture()
	h := publicAlertServer(t, &ckNenTang{}, f, c)
	for i := 0; i < ratelimit.PublicDisasterAlertsReadLimit; i++ {
		if w := publicAlertGet(h, ckHostA, "203.0.113.7:5000"); w.Code != http.StatusOK {
			t.Fatalf("request %d: %d", i+1, w.Code)
		}
	}
	w := publicAlertGet(h, ckHostA, "203.0.113.7:5000")
	doiMa(t, w, http.StatusTooManyRequests)
	// Its OWN budget (owner, 11/10/2026): the counter is the alert policy's, never the news one's.
	if want := "t:" + string(xaA) + ":rl:public-disaster-alerts:host:" + ckHostA + ":ip:203.0.113.7"; c.keys[0] != want {
		t.Errorf("limiter key = %q, want %q — the alert read's own budget", c.keys[0], want)
	}
}

// Through the production cache: an unknown, reserved or inactive host is still byte-identical to a
// commune with no alert and never reaches the cache or the store; a known commune's second read is a hit.
func TestPublicDisasterAlertsThroughCache(t *testing.T) {
	empty := &fakeDisasterAlerts{byCommune: map[tenant.ID][]domain.DisasterAlert{}}
	reference := publicAlertGet(publicAlertServerOver(t, &ckNenTang{}, app.NewInForceAlertCache(empty), nil), ckHostA, "")
	doiMa(t, reference, http.StatusOK)
	if strings.TrimSpace(reference.Body.String()) != `{"items":[]}` {
		t.Fatalf("empty = %q", reference.Body.String())
	}
	for _, host := range []string{ckHostKhongCo, ckHostNgung, ckHostRieng} {
		f := alertFixture()
		w := publicAlertGet(publicAlertServerOver(t, &ckNenTang{}, app.NewInForceAlertCache(f), nil), host, "")
		if w.Code != http.StatusOK || w.Body.String() != reference.Body.String() || f.calls != 0 {
			t.Errorf("%s: %d %q (store asked %d)", host, w.Code, w.Body.String(), f.calls)
		}
	}

	f := alertFixture()
	h := publicAlertServerOver(t, &ckNenTang{}, app.NewInForceAlertCache(f), nil)
	first := publicAlertGet(h, ckHostA, "")
	second := publicAlertGet(h, ckHostA, "")
	if first.Code != http.StatusOK || first.Body.String() != second.Body.String() || f.calls != 1 {
		t.Fatalf("second read: %d, same=%v, store asked %d — want a cache hit", second.Code,
			first.Body.String() == second.Body.String(), f.calls)
	}
	if strings.Contains(first.Body.String(), "da-b-1") {
		t.Fatal("commune B's alert in commune A's reply")
	}
}

// --- body cap ---------------------------------------------------------------------------------

// maximalAlertBody is the LARGEST body the domain accepts: every text at its bound, every list full —
// and every text character spelt as an escaped surrogate pair (12 bytes), the longest JSON spelling.
func maximalAlertBody() string {
	const siren = `🚨` // ONE character, U+1F6A8, escaped: 12 bytes of JSON
	txt := func(n int) string { return strings.Repeat(siren, n) }
	var b strings.Builder
	b.WriteString(`{"kind":"lu","title":"` + txt(domain.DisasterTitleMaxLen) + `","level":"` + txt(domain.DisasterLevelMaxLen) +
		`","headline":"` + txt(domain.DisasterHeadlineMaxLen) + `","detail":"` + txt(domain.DisasterDetailMaxLen) +
		`","effective_from":"2026-10-11T08:00:00+07:00","effective_to":"2026-10-12T08:00:00+07:00","danger_zone":[`)
	for i := 0; i < domain.DisasterDangerZoneMax; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`{"x":12.345678901234567,"y":98.76543210987654}`)
	}
	// The longest dial string the domain accepts: the agreed fake number, separators up to 32 characters.
	phone := "0900000000" + strings.Repeat("-", domain.ExternalContactPhoneMaxLen-11) + "0"
	b.WriteString(`],"evacuation_points":[`)
	for i := 0; i < domain.DisasterEvacuationMax; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`{"name":"` + txt(domain.DisasterPlaceNameMaxLen) + `","address":"` + txt(domain.DisasterAddressMaxLen) +
			`","phone":"` + phone + `","capacity":2147483647}`)
	}
	b.WriteString(`],"emergency_contacts":[`)
	for i := 0; i < domain.DisasterContactsMax; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`{"role":"` + txt(domain.DisasterContactRoleMaxLen) + `","name":"` + txt(domain.DisasterContactNameMaxLen) +
			`","phone":"` + phone + `"}`)
	}
	b.WriteString(`]}`)
	return b.String()
}

// The maximal valid alert passes the cap on POST and PATCH, reaches the use case whole, and IS valid —
// the domain accepts it, so the cap is the only thing that could have refused it.
func TestDisasterAlertMaximalValidBodyPassesTheCap(t *testing.T) {
	body := maximalAlertBody()
	if len(body) <= thanToiDa {
		t.Fatalf("the maximal body is %d bytes — the test no longer exercises the cap", len(body))
	}
	if len(body) > disasterAlertBodyMax {
		t.Fatalf("maximal valid body %d > cap %d", len(body), disasterAlertBodyMax)
	}

	s := newAlertServer(t)
	s.grant(xaA, "content.update")
	doiMa(t, s.call(t, http.MethodPost, hostA, alertPath, canBoGhi(xaA), body), http.StatusCreated)
	in := s.fake.lastInput
	if len([]rune(in.Detail)) != domain.DisasterDetailMaxLen || len(in.EvacuationPoints) != domain.DisasterEvacuationMax ||
		len(in.EmergencyContacts) != domain.DisasterContactsMax || len(in.DangerZone) != domain.DisasterDangerZoneMax {
		t.Fatalf("decoded detail %d runes, %d/%d/%d items", len([]rune(in.Detail)), len(in.EvacuationPoints),
			len(in.EmergencyContacts), len(in.DangerZone))
	}
	if _, err := domain.NormalizeDisasterAlert(domain.DisasterAlert{Kind: domain.DisasterKind(in.Kind), Title: in.Title,
		Level: in.Level, Headline: in.Headline, Detail: in.Detail, EffectiveFrom: in.EffectiveFrom, EffectiveTo: in.EffectiveTo,
		DangerZone: in.DangerZone, EvacuationPoints: in.EvacuationPoints, EmergencyContacts: in.EmergencyContacts}); err != nil {
		t.Fatalf("the 'maximal valid' body is not valid: %v", err)
	}

	s = newAlertServer(t)
	s.grant(xaA, "content.update")
	doiMa(t, s.call(t, http.MethodPatch, hostA, alertPath+"/da-a-1", canBoGhi(xaA), body), http.StatusOK)
}

// Past the cap is refused before the use case — 400 with the fixed sentence, no input echoed.
func TestDisasterAlertOversizeBodyIsRefused(t *testing.T) {
	pad := strings.Repeat(" ", disasterAlertBodyMax)
	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, alertPath}, {http.MethodPatch, alertPath + "/da-a-1"},
	} {
		s := newAlertServer(t)
		s.grant(xaA, "content.update")
		w := s.call(t, tc.method, hostA, tc.path, canBoGhi(xaA), `{"title":"Lũ"`+pad+`}`)
		doiMa(t, w, http.StatusBadRequest)
		if s.fake.calls != 0 || !strings.Contains(w.Body.String(), "quá lớn") {
			t.Errorf("%s: use case ran %d times, body %s", tc.method, s.fake.calls, w.Body.String())
		}
	}
	// DELETE keeps the service-wide 64 KiB: its body is one reason.
	s := newAlertServer(t)
	s.grant(xaA, "content.update")
	w := s.call(t, http.MethodDelete, hostA, alertPath+"/da-a-1", canBoGhi(xaA),
		`{"reason":"x"`+strings.Repeat(" ", thanToiDa)+`}`)
	doiMa(t, w, http.StatusBadRequest)
}

func TestRegisterPublicDisasterAlertsRefusesIncompleteWiring(t *testing.T) {
	f := alertFixture()
	for name, d := range map[string]PublicDisasterAlertDeps{
		"no platform": {Alerts: f, Limiter: ckLimiter()},
		"no store":    {Xa: &ckNenTang{}, Limiter: ckLimiter()},
		"no limiter":  {Xa: &ckNenTang{}, Alerts: f},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: registered anyway", name)
				}
			}()
			RegisterPublicDisasterAlerts(http.NewServeMux(), d)
		}()
	}
}
