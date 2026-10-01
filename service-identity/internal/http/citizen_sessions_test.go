package http

// POST /api/v1/citizen-sessions (ADR 0066), driven END TO END below the HTTP edge: the real
// app.OwnAppSignIn, a real core/crypto Envelope over an in-memory DEK store, and the real
// internal/zalo client against an httptest server standing in for Zalo. Only the platform registry,
// the settings row and the session bridge (Mo) are fakes — Mo's own transaction is tested in
// internal/app.
//
// RULE 5 #7 ADAPTED FOR A PUBLIC ROUTE: there is no token to be missing (401 "no token") and no
// permission to be wrong (403). The commune dimension IS tested: the commune is the App ID's bound
// commune from the platform, never anything the body says, and a secret sealed for another commune
// or App ID does not open.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/crypto"
	"github.com/vihat/vigov/core/platformclient"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
	"github.com/vihat/vigov/service-identity/internal/zalo"
)

const (
	ownApp       = "1234567890123456789"
	ownAppDemo   = "2234567890123456789"
	sharedApp    = "3234567890123456789"
	ownAppIdle   = "4234567890123456789" // bound, commune inactive
	ownAppNoConf = "5234567890123456789" // bound, no settings row
	ownAppOnly   = "6234567890123456789" // bound, demo-only settings (no secret)

	communeA tenant.ID = "01J9XA0000000000000000000A"
	communeB tenant.ID = "01J9XB0000000000000000000B"

	fakeAppSecret   = "FAKE-APP-SECRET-0000"
	fakeAccessToken = "FAKE-ACCESS-TOKEN"
	fakePhoneToken  = "FAKE-PHONE-TOKEN"
	fakeZaloID      = "9876543210987654321"
)

// signInFake is the OwnAppSignInner the OTHER public-route tests mount: it must never be reached.
type signInFake struct{ calls int }

func (f *signInFake) SignIn(context.Context, app.OwnAppSignInRequest) (app.KetQuaMoPhienCau, error) {
	f.calls++
	return app.KetQuaMoPhienCau{}, errors.New("signInFake: not under test")
}

// --- fakes under the real use case ---------------------------------------------------------------

type platformFake struct {
	apps map[string]platformclient.MiniApp
	err  error
}

func (p *platformFake) MiniApp(_ context.Context, id string) (platformclient.MiniApp, bool, error) {
	if p.err != nil {
		return platformclient.MiniApp{}, false, p.err
	}
	a, ok := p.apps[id]
	return a, ok, nil
}

type settingsFake struct {
	rows map[string]idstore.MiniAppSecret // key: tenant|appID
}

func (s *settingsFake) Live(ctx context.Context, appID string) (idstore.MiniAppSecret, bool, error) {
	r, ok := s.rows[string(tenant.MustFrom(ctx))+"|"+appID]
	return r, ok, nil
}

// memDEKs is crypto.DEKStore in a map, keyed by the commune in ctx.
type memDEKs struct {
	mu sync.Mutex
	m  map[tenant.ID]crypto.WrappedDEK
}

func (d *memDEKs) GetDEK(ctx context.Context) (crypto.WrappedDEK, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	w, ok := d.m[tenant.MustFrom(ctx)]
	if !ok {
		return crypto.WrappedDEK{}, crypto.ErrDEKNotFound
	}
	return w, nil
}

func (d *memDEKs) CreateDEK(ctx context.Context, w crypto.WrappedDEK) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.m[tenant.MustFrom(ctx)]; ok {
		return crypto.ErrDEKExists
	}
	d.m[tenant.MustFrom(ctx)] = w
	return nil
}

func (d *memDEKs) ReplaceDEK(ctx context.Context, _, w crypto.WrappedDEK) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.m[tenant.MustFrom(ctx)] = w
	return nil
}

// moFake is the session bridge: it records what it was asked and issues a token.
type moFake struct {
	calls []app.YeuCauMoPhienCau
	err   error
}

func (m *moFake) Mo(_ context.Context, yc app.YeuCauMoPhienCau) (app.KetQuaMoPhienCau, error) {
	m.calls = append(m.calls, yc)
	if m.err != nil {
		return app.KetQuaMoPhienCau{}, m.err
	}
	return app.KetQuaMoPhienCau{Token: "VIGOV-TOKEN", Sid: "SID-1", HetHan: time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC),
		Xa: yc.RequireOwnAppOf, TenXa: "Xã Thăng Bình", TenMienChinh: "thangbinh-danang.vigov.vn",
		DaCoSo: yc.SoDaXacThuc != "", CheDo: domain.CheDoAppRieng}, nil
}

// zaloFake answers the two Graph API calls; the phone exchange succeeds only with wantSecret.
type zaloFake struct {
	mu         sync.Mutex
	calls      []string
	wantSecret string
	meError    int
	down       bool
}

func (z *zaloFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	z.mu.Lock()
	z.calls = append(z.calls, r.URL.Path)
	z.mu.Unlock()
	if z.down {
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	switch r.URL.Path {
	case zalo.AccountIDPath:
		_, _ = io.WriteString(w, `{"id":"`+fakeZaloID+`","error":`+itoa(z.meError)+`}`)
	case zalo.InfoPath:
		if r.Header.Get(zalo.HeaderSecretKey) != z.wantSecret {
			_, _ = io.WriteString(w, `{"error":452,"message":"session key invalid"}`)
			return
		}
		_, _ = io.WriteString(w, `{"data":{"number":"0900000000"},"error":0}`)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }

type harness struct {
	h        http.Handler
	mo       *moFake
	zalo     *zaloFake
	plat     *platformFake
	settings *settingsFake
	log      *bytes.Buffer
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	zf := &zaloFake{wantSecret: fakeAppSecret}
	srv := httptest.NewServer(zf)
	t.Cleanup(srv.Close)

	env, err := crypto.New([]secret.Secret{secret.Secret(bytes.Repeat([]byte{7}, crypto.KeyLength))},
		&memDEKs{m: map[tenant.ID]crypto.WrappedDEK{}})
	if err != nil {
		t.Fatal(err)
	}
	seal := func(xa tenant.ID, appID, value string) []byte {
		b, err := env.Seal(tenant.Into(context.Background(), xa), secret.Secret(value),
			[]byte("mini_app_secret/app_secret_sealed/"+string(xa)+"/"+appID))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}

	own := func(xa tenant.ID) platformclient.MiniApp {
		return platformclient.MiniApp{CheDo: platformclient.CheDoAppRieng, XaRieng: xa}
	}
	plat := &platformFake{apps: map[string]platformclient.MiniApp{
		ownApp:       own(communeA),
		ownAppDemo:   own(communeB),
		sharedApp:    {CheDo: platformclient.CheDoAppChinh},
		ownAppIdle:   {CheDo: platformclient.CheDoAppRieng}, // XaRieng empty: bound commune inactive
		ownAppNoConf: own(communeA),
		ownAppOnly:   own(communeA),
	}}
	settings := &settingsFake{rows: map[string]idstore.MiniAppSecret{
		string(communeA) + "|" + ownApp:     {ID: "V1", AppID: ownApp, Sealed: seal(communeA, ownApp, fakeAppSecret)},
		string(communeB) + "|" + ownAppDemo: {ID: "V2", AppID: ownAppDemo, Sealed: seal(communeB, ownAppDemo, fakeAppSecret), DemoIdentity: true},
		string(communeA) + "|" + ownAppOnly: {ID: "V3", AppID: ownAppOnly, DemoIdentity: true},
	}}

	var logBuf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	mo := &moFake{}
	uc := app.NewOwnAppSignIn(plat, settings, env, zalo.New(srv.URL), mo, log)

	mux := http.NewServeMux()
	RegisterCongKhai(mux, DepsCongKhai{Xa: &nenTangXaGia{}, DanhBa: &danhBaCongKhaiGia{}, Profile: &profileReaderFake{},
		CitizenSessions: uc, Log: log})
	return &harness{h: mux, mo: mo, zalo: zf, plat: plat, settings: settings, log: &logBuf}
}

func (hs *harness) post(body string, ip string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, "https://identity.api.vigov.vn"+CitizenSessionsPath, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("User-Agent", "Zalo/test")
	if ip == "" {
		ip = "203.0.113.7"
	}
	r.RemoteAddr = ip + ":5555"
	w := httptest.NewRecorder()
	hs.h.ServeHTTP(w, r)
	return w
}

func normalBody(appID string) string {
	return `{"appId":"` + appID + `","accessToken":"` + fakeAccessToken + `","phoneToken":"` + fakePhoneToken + `"}`
}

func demoBody(appID string) string { return `{"appId":"` + appID + `","demoIdentity":true}` }

func wantError(t *testing.T, w *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status = %d, want %d — body %s", w.Code, status, w.Body.String())
	}
	var e struct{ Code, Message string }
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil || e.Code != code || e.Message == "" {
		t.Fatalf("body = %s, want code %q with a message", w.Body.String(), code)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", w.Header().Get("Cache-Control"))
	}
}

// --- the route exists where cmd/server sends it ---------------------------------------------------

func TestCitizenSessionsLiteralMatchesConstant(t *testing.T) {
	hs := newHarness(t)
	_, pattern := hs.h.(*http.ServeMux).Handler(httptest.NewRequest(http.MethodPost, CitizenSessionsPath, nil))
	if pattern != "POST "+CitizenSessionsPath {
		t.Fatalf("pattern = %q", pattern)
	}
	if CitizenSessionsPath == "/api/v1/sessions" {
		t.Fatal("the own-app citizen sign-in must not share the staff sign-in path")
	}
}

// --- 201 ------------------------------------------------------------------------------------------

func TestCitizenSessionsNormal201(t *testing.T) {
	hs := newHarness(t)
	w := hs.post(normalBody(ownApp), "")
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d — %s", w.Code, w.Body.String())
	}
	// The exact wire of vihat-miniapp's phanHoiPhienViGov.
	const want = `{"vigovSession":{"token":"VIGOV-TOKEN","expiresAt":"2026-10-31T00:00:00Z","tenantDisplayName":"Xã Thăng Bình","phoneVerified":true,"communePrimaryHost":"thangbinh-danang.vigov.vn"}}`
	if got := strings.TrimSpace(w.Body.String()); got != want {
		t.Fatalf("body =\n%s\nwant\n%s", got, want)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Error("201 carries a bearer token and must be no-store")
	}
	if len(hs.mo.calls) != 1 {
		t.Fatalf("Mo called %d times", len(hs.mo.calls))
	}
	yc := hs.mo.calls[0]
	if yc.AppID != ownApp || yc.MaZalo != fakeZaloID || yc.SoDaXacThuc != "84900000000" ||
		yc.RequireOwnAppOf != communeA || yc.DemoIdentity || yc.IP != "203.0.113.7" || yc.ThietBi != "Zalo/test" {
		t.Fatalf("Mo request = %+v", yc)
	}
	// Account id FIRST, then the phone exchange (internal/zalo, ORDER).
	if strings.Join(hs.zalo.calls, ",") != zalo.AccountIDPath+","+zalo.InfoPath {
		t.Fatalf("Zalo calls = %v", hs.zalo.calls)
	}
	for _, leak := range []string{fakeAccessToken, fakePhoneToken, fakeAppSecret, fakeZaloID, "900000000"} {
		if strings.Contains(hs.log.String(), leak) {
			t.Errorf("log carries %q", leak)
		}
	}
}

func TestCitizenSessionsDemo201CallsNoZalo(t *testing.T) {
	hs := newHarness(t)
	w := hs.post(demoBody(ownAppDemo), "")
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d — %s", w.Code, w.Body.String())
	}
	if len(hs.zalo.calls) != 0 {
		t.Fatalf("demo sign-in called Zalo: %v (ADR 0066: không gọi lệnh Zalo nào)", hs.zalo.calls)
	}
	yc := hs.mo.calls[0]
	if yc.MaZalo != domain.DemoZaloAccountID(ownAppDemo) || yc.SoDaXacThuc != domain.DemoIdentityPhone ||
		!yc.DemoIdentity || yc.RequireOwnAppOf != communeB {
		t.Fatalf("Mo request = %+v", yc)
	}
	if !strings.Contains(hs.log.String(), "level=WARN") || !strings.Contains(hs.log.String(), "demo_identity=true") ||
		!strings.Contains(hs.log.String(), "app_id="+ownAppDemo) {
		t.Fatalf("demo session not logged at WARN with app_id and demo_identity: %s", hs.log.String())
	}
	if strings.Contains(hs.log.String(), domain.DemoIdentityPhone) {
		t.Error("the demo number reached the log")
	}
	// Demo-only settings (no secret yet) sign in as demo too.
	if w := hs.post(demoBody(ownAppOnly), ""); w.Code != http.StatusCreated {
		t.Fatalf("demo-only settings: status = %d", w.Code)
	}
}

// --- 400 ------------------------------------------------------------------------------------------

func TestCitizenSessionsBadBody400(t *testing.T) {
	for name, body := range map[string]string{
		"not json":            `appId=1`,
		"empty":               ``,
		"array":               `[]`,
		"unknown field":       `{"appId":"` + ownApp + `","accessToken":"a","phoneToken":"p","communeHostHint":"x.vigov.vn"}`,
		"two objects":         normalBody(ownApp) + normalBody(ownApp),
		"over 8 KB":           `{"appId":"` + ownApp + `","accessToken":"` + strings.Repeat("a", 9<<10) + `","phoneToken":"p"}`,
		"app id not digits":   normalBody("abc"),
		"app id missing":      `{"accessToken":"a","phoneToken":"p"}`,
		"no access token":     `{"appId":"` + ownApp + `","phoneToken":"p"}`,
		"demo with tokens":    `{"appId":"` + ownAppDemo + `","demoIdentity":true,"accessToken":"a"}`,
		"demo false no token": `{"appId":"` + ownApp + `","demoIdentity":false}`,
	} {
		hs := newHarness(t)
		w := hs.post(body, "")
		wantError(t, w, http.StatusBadRequest, "invalid_body")
		if len(hs.mo.calls) != 0 || len(hs.zalo.calls) != 0 {
			t.Errorf("%s: reached Mo %d / Zalo %d times", name, len(hs.mo.calls), len(hs.zalo.calls))
		}
	}
}

func TestCitizenSessionsPhoneRequired400(t *testing.T) {
	hs := newHarness(t)
	// No phoneToken: refused before the platform is asked and before Zalo.
	hs.plat.err = errors.New("platform must not be asked")
	wantError(t, hs.post(`{"appId":"`+ownApp+`","accessToken":"a"}`, ""), http.StatusBadRequest, "phone_required")
	hs.plat.err = nil
	// Demo body for an App ID whose demo identity is OFF: the same answer, no Zalo call.
	w := hs.post(demoBody(ownApp), "")
	wantError(t, w, http.StatusBadRequest, "phone_required")
	if len(hs.zalo.calls) != 0 || len(hs.mo.calls) != 0 {
		t.Fatalf("Zalo %v / Mo %d", hs.zalo.calls, len(hs.mo.calls))
	}
}

// --- 422: one answer ----------------------------------------------------------------------------

func TestCitizenSessionsNotReady422Identical(t *testing.T) {
	var first string
	for name, body := range map[string]string{
		"unknown app id":            normalBody("7234567890123456789"),
		"shared ViHAT app":          normalBody(sharedApp),
		"bound commune inactive":    normalBody(ownAppIdle),
		"no settings row":           normalBody(ownAppNoConf),
		"demo-only, real sign-in":   normalBody(ownAppOnly),
		"shared app, demo body":     demoBody(sharedApp),
		"no settings row, demo":     demoBody(ownAppNoConf),
		"unknown app id, demo body": demoBody("7234567890123456789"),
	} {
		hs := newHarness(t)
		w := hs.post(body, "")
		wantError(t, w, http.StatusUnprocessableEntity, "app_not_ready")
		if first == "" {
			first = w.Body.String()
		} else if w.Body.String() != first {
			t.Errorf("%s: body differs from the other 422s — %s vs %s", name, w.Body.String(), first)
		}
		if len(hs.zalo.calls) != 0 || len(hs.mo.calls) != 0 {
			t.Errorf("%s: Zalo %v / Mo %d", name, hs.zalo.calls, len(hs.mo.calls))
		}
	}
}

// The secret sealed for commune A's App ID does not open as another App ID's, nor in commune B:
// copied rows are refused as 503 (our fault), never a sign-in with the wrong app's secret.
func TestCitizenSessionsSecretBoundToCommuneAndApp(t *testing.T) {
	// Commune A's sealed bytes for ownApp, copied onto ANOTHER App ID's row of the same commune.
	hs := newHarness(t)
	hs.settings.rows[string(communeA)+"|"+ownAppNoConf] = idstore.MiniAppSecret{ID: "VX", AppID: ownAppNoConf,
		Sealed: hs.settings.rows[string(communeA)+"|"+ownApp].Sealed}
	wantError(t, hs.post(normalBody(ownAppNoConf), ""), http.StatusServiceUnavailable, "sign_in_unavailable")

	// The same bytes under the same App ID in ANOTHER commune (the binding moved to B).
	hs.plat.apps[ownApp] = platformclient.MiniApp{CheDo: platformclient.CheDoAppRieng, XaRieng: communeB}
	hs.settings.rows[string(communeB)+"|"+ownApp] = hs.settings.rows[string(communeA)+"|"+ownApp]
	wantError(t, hs.post(normalBody(ownApp), ""), http.StatusServiceUnavailable, "sign_in_unavailable")

	// Opened BEFORE Zalo: a broken secret never spends the citizen's phoneToken.
	if len(hs.zalo.calls) != 0 || len(hs.mo.calls) != 0 {
		t.Fatalf("Zalo %v / Mo %d", hs.zalo.calls, len(hs.mo.calls))
	}
	if !strings.Contains(hs.log.String(), "CẢNH BÁO VẬN HÀNH") {
		t.Errorf("a secret that does not open must alert the operator: %s", hs.log.String())
	}
}

// --- 401 / 502 / 503 -------------------------------------------------------------------------------

func TestCitizenSessionsZaloMapping(t *testing.T) {
	hs := newHarness(t)
	hs.zalo.meError = -216
	wantError(t, hs.post(normalBody(ownApp), ""), http.StatusUnauthorized, "zalo_token_invalid")

	hs = newHarness(t)
	hs.zalo.wantSecret = "ANOTHER-SECRET" // our stored secret is not the one Zalo knows → error 452
	wantError(t, hs.post(normalBody(ownApp), ""), http.StatusUnauthorized, "zalo_token_invalid")

	hs = newHarness(t)
	hs.zalo.down = true
	wantError(t, hs.post(normalBody(ownApp), ""), http.StatusBadGateway, "zalo_unreachable")
	if len(hs.mo.calls) != 0 {
		t.Fatal("a session was opened although Zalo failed")
	}
}

func TestCitizenSessionsUnavailable503(t *testing.T) {
	hs := newHarness(t)
	hs.plat.err = errors.New("rpc error: code = Unavailable")
	wantError(t, hs.post(normalBody(ownApp), ""), http.StatusServiceUnavailable, "sign_in_unavailable")

	hs = newHarness(t)
	hs.mo.err = app.ErrCauNenTang
	wantError(t, hs.post(normalBody(ownApp), ""), http.StatusServiceUnavailable, "sign_in_unavailable")

	hs = newHarness(t)
	hs.mo.err = errors.New("store: commit: connection reset")
	wantError(t, hs.post(normalBody(ownApp), ""), http.StatusServiceUnavailable, "sign_in_unavailable")
	if strings.Contains(hs.post(normalBody(ownApp), "198.51.100.1").Body.String(), "connection reset") {
		t.Fatal("503 body leaks the cause")
	}

	// Mo refusing because the binding moved between the two reads → the one 422.
	hs = newHarness(t)
	hs.mo.err = app.ErrCauAppChuaSanSang
	wantError(t, hs.post(normalBody(ownApp), ""), http.StatusUnprocessableEntity, "app_not_ready")
}

// --- 429 --------------------------------------------------------------------------------------------

func TestCitizenSessionsRateLimit(t *testing.T) {
	hs := newHarness(t)
	for i := 0; i < citizenSessionLimit; i++ {
		// Even malformed attempts count: the limit stands before the body is read.
		if w := hs.post(`{`, "203.0.113.9"); w.Code != http.StatusBadRequest {
			t.Fatalf("attempt %d: status %d", i+1, w.Code)
		}
	}
	w := hs.post(normalBody(ownApp), "203.0.113.9")
	wantError(t, w, http.StatusTooManyRequests, "too_many_attempts")
	if ra := w.Header().Get("Retry-After"); ra == "" || ra == "0" {
		t.Fatalf("Retry-After = %q", ra)
	}
	if len(hs.zalo.calls) != 0 || len(hs.mo.calls) != 0 {
		t.Fatal("a limited request reached Zalo or Mo")
	}
	// Another client is not limited by the first.
	if w := hs.post(normalBody(ownApp), "203.0.113.10"); w.Code != http.StatusCreated {
		t.Fatalf("other client: status %d", w.Code)
	}
}

func TestIPRateLimiterSlidingWindow(t *testing.T) {
	now := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	l := newIPRateLimiter(3, time.Minute, 2, func() time.Time { return now })
	for i := 0; i < 3; i++ {
		if ok, _ := l.allow("a"); !ok {
			t.Fatalf("attempt %d refused", i+1)
		}
		now = now.Add(10 * time.Second)
	}
	ok, retry := l.allow("a")
	if ok || retry != 30*time.Second {
		t.Fatalf("4th: ok=%v retry=%v, want refused with 30s (first attempt leaves at +60s)", ok, retry)
	}
	// A refused attempt is not counted: once the first leaves the window, one more is allowed.
	now = now.Add(30 * time.Second)
	if ok, _ := l.allow("a"); !ok {
		t.Fatal("refused after the oldest attempt left the window")
	}
	// Key table full: a NEW client is refused (fail closed) until the sweep frees space.
	if ok, _ := l.allow("b"); !ok {
		t.Fatal("second key refused below the cap")
	}
	if ok, _ := l.allow("c"); ok {
		t.Fatal("third key admitted past maxKeys")
	}
	now = now.Add(2 * time.Minute)
	if ok, _ := l.allow("c"); !ok {
		t.Fatal("sweep did not free stale keys")
	}
}
