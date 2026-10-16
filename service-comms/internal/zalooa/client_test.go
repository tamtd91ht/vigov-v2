package zalooa

import (
	"bytes"
	"context"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/secret"
)

// Every credential here is FAKE — the text says so (rule 8, forbidden #1).
const (
	testAppID    = "1234567890"
	testSecret   = "FAKE-zalo-app-secret-NOT-REAL"
	testCode     = "FAKE-authorization-code-NOT-REAL"
	testAccess   = "FAKE-access-token-NOT-REAL"
	testRefresh  = "FAKE-refresh-token-NOT-REAL"
	testVerifier = "FAKEverifierFAKEverifierFAKEverifierFAKEv43"
)

// recorder is a fake Zalo: it records what reached it and answers what the test says.
type recorder struct {
	mu      sync.Mutex
	paths   []string
	queries []url.Values
	headers []http.Header
	bodies  [][]byte
	answer  func(w http.ResponseWriter, r *http.Request)
}

func (rc *recorder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	rc.mu.Lock()
	rc.paths = append(rc.paths, r.URL.Path)
	rc.queries = append(rc.queries, r.URL.Query())
	rc.headers = append(rc.headers, r.Header.Clone())
	rc.bodies = append(rc.bodies, b)
	rc.mu.Unlock()
	rc.answer(w, r)
}

// start serves all three hosts from ONE fake server: the path tells which endpoint was called.
func start(t *testing.T, answer func(w http.ResponseWriter, r *http.Request)) (*Client, *recorder, *httptest.Server) {
	t.Helper()
	rc := &recorder{answer: answer}
	srv := httptest.NewTLSServer(rc)
	t.Cleanup(srv.Close)
	return newClient(srv.URL+"/v4", srv.URL+"/v2.0", srv.URL, srv.Client()), rc, srv
}

func jsonAnswer(status int, body string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

// captureLogs routes slog's default logger AND the standard log package into one buffer for the test.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prevSlog := slog.Default()
	prevOut := log.Writer()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	log.SetOutput(&buf)
	t.Cleanup(func() {
		slog.SetDefault(prevSlog)
		log.SetOutput(prevOut)
	})
	return &buf
}

func TestPermissionURLIsZalosShape(t *testing.T) {
	u := PermissionURL(testAppID, "https://bot.api.vigov.vn/api/v1/zalo-oa-authorizations", "chal_lenge", "st-ate")
	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Scheme != "https" || parsed.Host != "oauth.zaloapp.com" || parsed.Path != "/v4/oa/permission" {
		t.Fatalf("url = %s", u)
	}
	q := parsed.Query()
	if q.Get("app_id") != testAppID || q.Get("redirect_uri") != "https://bot.api.vigov.vn/api/v1/zalo-oa-authorizations" ||
		q.Get("code_challenge") != "chal_lenge" || q.Get("state") != "st-ate" || len(q) != 4 {
		t.Fatalf("query = %v", q)
	}
}

func TestNewPKCEIs43UnreservedCharactersAndS256(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		v, ch, err := NewPKCE()
		if err != nil {
			t.Fatal(err)
		}
		if len(v) != codeVerifierLength {
			t.Fatalf("verifier length %d", len(v))
		}
		for _, b := range v.Lo() {
			if !(b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' || b >= '0' && b <= '9') {
				t.Fatalf("verifier byte %q outside [A-Za-z0-9]", b)
			}
		}
		if ch != ChallengeOf(v) || strings.ContainsAny(ch, "=+/") || len(ch) != 43 {
			t.Fatalf("challenge %q is not unpadded base64url of SHA-256", ch)
		}
		if seen[string(v)] {
			t.Fatal("two verifiers equal — not random")
		}
		seen[string(v)] = true
	}
	// RFC 7636 appendix B's known pair, so S256 is the real one and not a look-alike.
	if got := ChallengeOf(secret.Secret("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk")); got != "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM" {
		t.Fatalf("S256 = %q", got)
	}
}

func TestExchangeCodeSendsFormAndSecretHeaderAndReadsTokens(t *testing.T) {
	c, rc, _ := start(t, jsonAnswer(200, `{"access_token":"`+testAccess+`","refresh_token":"`+testRefresh+`","expires_in":"90000"}`))
	tk, o := c.ExchangeCode(context.Background(), testAppID, secret.Secret(testSecret), testCode, secret.Secret(testVerifier))
	if o != OutcomeOK || string(tk.Access) != testAccess || string(tk.Refresh) != testRefresh || tk.ExpiresIn != 90000*time.Second {
		t.Fatalf("= %v %v", tk.ExpiresIn, o)
	}
	if rc.paths[0] != "/v4/oa/access_token" || rc.headers[0].Get("secret_key") != testSecret {
		t.Fatalf("path %q headers %v", rc.paths[0], rc.headers[0])
	}
	form, err := url.ParseQuery(string(rc.bodies[0]))
	if err != nil {
		t.Fatal(err)
	}
	if form.Get("app_id") != testAppID || form.Get("grant_type") != "authorization_code" || form.Get("code") != testCode ||
		form.Get("code_verifier") != testVerifier || len(form) != 4 {
		t.Fatalf("form = %v", form)
	}
	if ct := rc.headers[0].Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
		t.Fatalf("content type %q", ct)
	}
}

func TestRefreshSendsRefreshGrant(t *testing.T) {
	c, rc, _ := start(t, jsonAnswer(200, `{"access_token":"`+testAccess+`","refresh_token":"`+testRefresh+`-2","expires_in":90000}`))
	tk, o := c.Refresh(context.Background(), testAppID, secret.Secret(testSecret), secret.Secret(testRefresh))
	if o != OutcomeOK || string(tk.Refresh) != testRefresh+"-2" || tk.ExpiresIn != 90000*time.Second {
		t.Fatalf("= %v", o)
	}
	form, _ := url.ParseQuery(string(rc.bodies[0]))
	if form.Get("grant_type") != "refresh_token" || form.Get("refresh_token") != testRefresh || form.Get("app_id") != testAppID || len(form) != 3 {
		t.Fatalf("form = %v", form)
	}
}

func TestTokenEndpointClasses(t *testing.T) {
	cases := map[string]struct {
		status int
		body   string
		want   Outcome
	}{
		"grant refused in a 2xx body": {200, `{"error":-14014,"error_name":"Invalid refresh token","error_reason":"x"}`, OutcomeTokenRejected},
		"400":                         {400, `{"error":-14019}`, OutcomeTokenRejected},
		"401":                         {401, `{}`, OutcomeTokenRejected},
		"429":                         {429, `{}`, OutcomeRateLimited},
		"503":                         {503, `{}`, OutcomeUnavailable},
		"404":                         {404, `{}`, OutcomeRejected},
		"no access token":             {200, `{"refresh_token":"x"}`, OutcomeMalformedResponse},
		"no expires_in is OK, zero":   {200, `{"access_token":"a","refresh_token":"b"}`, OutcomeOK},
		"html":                        {200, `<html>`, OutcomeMalformedResponse},
	}
	for name, cs := range cases {
		t.Run(name, func(t *testing.T) {
			c, _, _ := start(t, jsonAnswer(cs.status, cs.body))
			tk, o := c.Refresh(context.Background(), testAppID, secret.Secret(testSecret), secret.Secret(testRefresh))
			if o != cs.want {
				t.Fatalf("= %v, want %v", o, cs.want)
			}
			if o == OutcomeOK && tk.ExpiresIn != 0 {
				t.Fatalf("expires_in guessed: %v", tk.ExpiresIn)
			}
		})
	}
}

func TestCallerFaultsNeverReachTheNetwork(t *testing.T) {
	c, rc, _ := start(t, jsonAnswer(200, `{}`))
	ctx := context.Background()
	if _, o := c.ExchangeCode(ctx, "", secret.Secret(testSecret), testCode, secret.Secret(testVerifier)); o != OutcomeRejected {
		t.Errorf("no app id: %v", o)
	}
	if _, o := c.ExchangeCode(ctx, testAppID, nil, testCode, secret.Secret(testVerifier)); o != OutcomeRejected {
		t.Errorf("no secret: %v", o)
	}
	if _, o := c.Refresh(ctx, testAppID, secret.Secret(testSecret), nil); o != OutcomeRejected {
		t.Errorf("no refresh token: %v", o)
	}
	if _, o := c.GetOA(ctx, nil); o != OutcomeTokenRejected {
		t.Errorf("no access token: %v", o)
	}
	if _, o := c.TemplateInfo(ctx, secret.Secret(testAccess), ""); o != OutcomeRejected {
		t.Errorf("no template id: %v", o)
	}
	if len(rc.paths) != 0 {
		t.Fatalf("a caller fault reached Zalo: %v", rc.paths)
	}
}

func TestGetOAPutsTheTokenInTheHeaderAndReadsBothIDTypes(t *testing.T) {
	for name, body := range map[string]string{
		"numeric id": `{"error":0,"message":"Success","data":{"oa_id":579745863508352884,"name":"UBND Xã A"}}`,
		"string id":  `{"error":0,"data":{"oa_id":"579745863508352884","name":"UBND Xã A"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			c, rc, _ := start(t, jsonAnswer(200, body))
			info, o := c.GetOA(context.Background(), secret.Secret(testAccess))
			if o != OutcomeOK || info.OAID != "579745863508352884" || info.Name != "UBND Xã A" {
				t.Fatalf("= %+v, %v", info, o)
			}
			if rc.paths[0] != "/v2.0/oa/getoa" || rc.headers[0].Get("access_token") != testAccess {
				t.Fatalf("path %q headers %v", rc.paths[0], rc.headers[0])
			}
			if len(rc.queries[0]) != 0 {
				t.Fatalf("the token must not be in the query: %v", rc.queries[0])
			}
		})
	}
	c, _, _ := start(t, jsonAnswer(200, `{"error":0,"data":{"name":"no id"}}`))
	if _, o := c.GetOA(context.Background(), secret.Secret(testAccess)); o != OutcomeMalformedResponse {
		t.Fatalf("no oa_id: %v", o)
	}
}

func TestOpenAPIBodyErrorClasses(t *testing.T) {
	cases := map[string]Outcome{
		`{"error":-216,"message":"Access token is invalid"}`: OutcomeTokenRejected,
		`{"error":"-124","message":"expired"}`:               OutcomeTokenRejected,
		`{"error":-201,"message":"Invalid parameter"}`:       OutcomeRejected,
		`{"error":"abc"}`: OutcomeMalformedResponse, // no readable code, no data
		`[]`:              OutcomeMalformedResponse,
	}
	for body, want := range cases {
		c, _, _ := start(t, jsonAnswer(200, body))
		if _, o := c.GetOA(context.Background(), secret.Secret(testAccess)); o != want {
			t.Errorf("%s: = %v, want %v", body, o, want)
		}
	}
}

func TestStatusClasses(t *testing.T) {
	cases := map[int]Outcome{
		401: OutcomeTokenRejected, 403: OutcomeTokenRejected,
		429: OutcomeRateLimited,
		408: OutcomeUnavailable, 500: OutcomeUnavailable, 502: OutcomeUnavailable, 503: OutcomeUnavailable,
		400: OutcomeRejected, 404: OutcomeRejected, 409: OutcomeRejected,
	}
	for status, want := range cases {
		c, _, _ := start(t, jsonAnswer(status, `{"error":`+strconv.Itoa(-status)+`,"message":"`+testAccess+`"}`))
		if _, o := c.GetOA(context.Background(), secret.Secret(testAccess)); o != want {
			t.Errorf("HTTP %d: = %v, want %v", status, o, want)
		}
	}
}

func TestRetryPolicy(t *testing.T) {
	for o, want := range map[Outcome]bool{
		OutcomeOK: false, OutcomeTokenRejected: false, OutcomeRateLimited: true, OutcomeUnavailable: true,
		OutcomeRejected: false, OutcomeMalformedResponse: false,
	} {
		if o.Retryable() != want {
			t.Errorf("%v.Retryable() = %v", o, !want)
		}
	}
}

func TestListTemplatesPagesAndReadsTotal(t *testing.T) {
	c, rc, _ := start(t, jsonAnswer(200, `{"error":0,"message":"Success","data":[
		{"templateId":1001,"templateName":"Tiếp nhận","createdTime":1700000000000,"status":"ENABLE","templateQuality":"HIGH"},
		{"templateId":"1002","templateName":"Đóng","status":"PENDING_REVIEW"}],"metadata":{"total":2}}`))
	list, total, o := c.ListTemplates(context.Background(), secret.Secret(testAccess), 0, 500)
	if o != OutcomeOK || total != 2 || len(list) != 2 {
		t.Fatalf("= %v %d %v", list, total, o)
	}
	if list[0] != (TemplateSummary{ID: "1001", Name: "Tiếp nhận", Status: "ENABLE", Quality: "HIGH"}) ||
		list[1] != (TemplateSummary{ID: "1002", Name: "Đóng", Status: "PENDING_REVIEW"}) {
		t.Fatalf("list = %+v", list)
	}
	if rc.paths[0] != "/template/all" || rc.queries[0].Get("offset") != "0" || rc.queries[0].Get("limit") != "100" {
		t.Fatalf("path %q query %v — limit must be clipped to 100", rc.paths[0], rc.queries[0])
	}
	if rc.headers[0].Get("access_token") != testAccess {
		t.Fatal("no access_token header")
	}
	c2, _, _ := start(t, jsonAnswer(200, `{"error":0,"data":[]}`))
	if list, total, o := c2.ListTemplates(context.Background(), secret.Secret(testAccess), 100, 100); o != OutcomeOK || total != -1 || len(list) != 0 {
		t.Fatalf("no total: %v %d %v", list, total, o)
	}
}

func TestTemplateInfoReadsParams(t *testing.T) {
	c, rc, _ := start(t, jsonAnswer(200, `{"error":0,"message":"Success","data":{"templateId":1001,"templateName":"Tiếp nhận",
		"status":"ENABLE","templateQuality":"MEDIUM","templateTag":"TRANSACTION","timeout":86400000,
		"listParams":[{"name":"ma_tra_cuu","require":true,"type":"STRING","maxLength":30},
		{"name":"moc_nhan","require":false,"type":"STRING","maxLength":60},
		{"name":"viec_tiep_theo","require":"true","type":"STRING","maxLength":"200"}]}}`))
	d, o := c.TemplateInfo(context.Background(), secret.Secret(testAccess), "1001")
	if o != OutcomeOK || d.ID != "1001" || d.Name != "Tiếp nhận" || d.Status != "ENABLE" || d.Quality != "MEDIUM" || d.Tag != "TRANSACTION" {
		t.Fatalf("= %+v, %v", d, o)
	}
	if len(d.Params) != 3 || d.Params[0] != (Param{Name: "ma_tra_cuu", Required: true, Type: "STRING", MaxLength: 30}) ||
		d.Params[1].Required || d.Params[2] != (Param{Name: "viec_tiep_theo", Required: true, Type: "STRING", MaxLength: 200}) {
		t.Fatalf("params = %+v", d.Params)
	}
	if rc.paths[0] != "/template/info" || rc.queries[0].Get("template_id") != "1001" {
		t.Fatalf("path %q query %v", rc.paths[0], rc.queries[0])
	}
	// snake_case spelling, and a parameter with no name is malformed.
	c2, _, _ := start(t, jsonAnswer(200, `{"error":0,"data":{"templateId":"7","list_params":[{"name":"a","required":1,"max_length":5}]}}`))
	if d, o := c2.TemplateInfo(context.Background(), secret.Secret(testAccess), "7"); o != OutcomeOK || len(d.Params) != 1 || !d.Params[0].Required || d.Params[0].MaxLength != 5 {
		t.Fatalf("snake_case: %+v %v", d, o)
	}
	c3, _, _ := start(t, jsonAnswer(200, `{"error":0,"data":{"templateId":"7","listParams":[{"type":"STRING"}]}}`))
	if _, o := c3.TemplateInfo(context.Background(), secret.Secret(testAccess), "7"); o != OutcomeMalformedResponse {
		t.Fatalf("nameless param: %v", o)
	}
}

// A redirect is never followed: the request carries the app secret / access token in its headers.
func TestRedirectIsNotFollowed(t *testing.T) {
	var hit bool
	other := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hit = true }))
	defer other.Close()
	c, _, _ := start(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+r.URL.Path, http.StatusTemporaryRedirect)
	})
	if _, o := c.GetOA(context.Background(), secret.Secret(testAccess)); o != OutcomeRejected {
		t.Fatalf("= %v", o)
	}
	if _, o := c.Refresh(context.Background(), testAppID, secret.Secret(testSecret), secret.Secret(testRefresh)); o != OutcomeRejected {
		t.Fatalf("token endpoint redirect = %v", o)
	}
	if hit {
		t.Fatal("the redirect was followed — the credential went to another host")
	}
}

// THE PROPERTY THE PACKAGE EXISTS FOR: whatever fails, nothing carrying a credential comes out — no
// error (there is none to return), no log line from this package or from net/http, no Outcome text.
func TestNothingSecretLeavesThroughAFailure(t *testing.T) {
	buf := captureLogs(t)
	ctx := context.Background()

	// 1. A server that is gone: the transport error (*url.Error) names the full URL.
	c, _, srv := start(t, jsonAnswer(200, `{}`))
	srv.Close()
	if _, o := c.ExchangeCode(ctx, testAppID, secret.Secret(testSecret), testCode, secret.Secret(testVerifier)); o != OutcomeUnavailable {
		t.Fatalf("closed server: = %v", o)
	}

	// 2. A timeout.
	slow, _, _ := start(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	})
	slow.http.Timeout = 50 * time.Millisecond
	if _, o := slow.GetOA(ctx, secret.Secret(testAccess)); o != OutcomeUnavailable {
		t.Fatalf("timeout: = %v", o)
	}

	// 3. A cancelled context.
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	ok, _, _ := start(t, jsonAnswer(200, `{"error":0,"data":{"oa_id":1}}`))
	if _, o := ok.GetOA(cctx, secret.Secret(testAccess)); o != OutcomeUnavailable {
		t.Fatalf("cancelled: = %v", o)
	}

	// 4. Zalo echoing the code and the secret back in an error body.
	echo, _, _ := start(t, jsonAnswer(400, `{"error":-14019,"error_description":"bad code `+testCode+` `+testSecret+`"}`))
	_, o := echo.ExchangeCode(ctx, testAppID, secret.Secret(testSecret), testCode, secret.Secret(testVerifier))
	if o.String() != "token_rejected" {
		t.Fatalf("echo: = %v", o)
	}

	for _, s := range []string{buf.String(), o.String(), OutcomeUnavailable.String()} {
		for _, needle := range []string{testSecret, testCode, testAccess, testRefresh, testVerifier, "FAKE-"} {
			if strings.Contains(s, needle) {
				t.Fatalf("a credential leaked: %q", s)
			}
		}
	}
	if buf.Len() != 0 {
		t.Fatalf("the adapter (or net/http) logged: %q", buf.String())
	}
}

func TestProductionClientIsHTTPSOnly(t *testing.T) {
	c := New(0)
	if c.oauth != OAuthBaseURL || c.openapi != OpenAPIBaseURL || c.business != BusinessBaseURL || c.http.Timeout != DefaultTimeout {
		t.Fatalf("hosts %q %q %q timeout %v", c.oauth, c.openapi, c.business, c.http.Timeout)
	}
	for _, u := range []string{OAuthBaseURL, OpenAPIBaseURL, BusinessBaseURL} {
		if !strings.HasPrefix(u, "https://") {
			t.Fatalf("%s is not https", u)
		}
	}
	tr := c.http.Transport.(*http.Transport)
	if tr.Proxy != nil || tr.TLSClientConfig.InsecureSkipVerify || tr.TLSClientConfig.MinVersion < 0x0303 {
		t.Fatal("transport security lowered")
	}
}

func TestOversizedBodyIsMalformed(t *testing.T) {
	c, _, _ := start(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"error":0,"data":{"oa_id":1,"name":"`))
		_, _ = w.Write(bytes.Repeat([]byte("x"), maxBody))
		_, _ = w.Write([]byte(`"}}`))
	})
	if _, o := c.GetOA(context.Background(), secret.Secret(testAccess)); o != OutcomeMalformedResponse {
		t.Fatalf("= %v", o)
	}
}
