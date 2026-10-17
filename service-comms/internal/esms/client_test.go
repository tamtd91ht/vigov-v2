package esms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/vihat/vigov/core/secret"
)

// Every credential here is FAKE — the text says so (rule 8, forbidden #1). The phone number is the
// agreed fake number (rule 3, invariant 5).
const (
	testAPIKey    = "FAKE-esms-api-key-NOT-REAL"
	testSecretKey = "FAKE-esms-secret-key-NOT-REAL"
	testPhone     = "0900000000"
	testBrand     = "XaMau"
	testRequestID = "01JRECIPIENT00000000000001"
)

var testCreds = Credentials{APIKey: secret.Secret(testAPIKey), SecretKey: secret.Secret(testSecretKey)}

// recorder is a fake eSMS: it records what reached it and answers what the test says.
type recorder struct {
	mu      sync.Mutex
	methods []string
	paths   []string
	queries []url.Values
	bodies  [][]byte
	answer  func(w http.ResponseWriter, r *http.Request)
}

func (rc *recorder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	rc.mu.Lock()
	rc.methods = append(rc.methods, r.Method)
	rc.paths = append(rc.paths, r.URL.Path)
	rc.queries = append(rc.queries, r.URL.Query())
	rc.bodies = append(rc.bodies, b)
	rc.mu.Unlock()
	rc.answer(w, r)
}

func start(t *testing.T, answer func(w http.ResponseWriter, r *http.Request)) (*Client, *recorder) {
	t.Helper()
	rc := &recorder{answer: answer}
	srv := httptest.NewTLSServer(rc)
	t.Cleanup(srv.Close)
	return newClient(srv.URL+"/MainService.svc/json", srv.Client()), rc
}

func jsonAnswer(status int, body string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

func bodyOf(t *testing.T, b []byte) map[string]string {
	t.Helper()
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("request body is not a JSON object of strings: %v", err)
	}
	return m
}

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

func TestNewUsesTheConstantHostAndVerifiedTLS12(t *testing.T) {
	c := New(0)
	if c.base != BaseURL || !strings.HasPrefix(BaseURL, "https://rest.esms.vn/") {
		t.Fatalf("base = %q", c.base)
	}
	tr, ok := c.http.Transport.(*http.Transport)
	if !ok || tr.Proxy != nil || tr.TLSClientConfig == nil || tr.TLSClientConfig.InsecureSkipVerify ||
		tr.TLSClientConfig.MinVersion < 0x0303 {
		t.Fatalf("transport = %+v", tr)
	}
	if c.http.Timeout != DefaultTimeout || c.http.CheckRedirect == nil || c.sandbox {
		t.Fatalf("client = timeout %v, redirect hook %v, sandbox %v", c.http.Timeout, c.http.CheckRedirect != nil, c.sandbox)
	}
}

func TestGetBalanceSendsBothKeysAndReadsCodeResponse(t *testing.T) {
	c, rc := start(t, jsonAnswer(200, `{"Balance": 36693, "CodeResponse": "100", "UserID": 9999}`))
	b, o := c.GetBalance(context.Background(), testCreds)
	if o != OutcomeOK || b != "36693" {
		t.Fatalf("= %q %v", b, o)
	}
	if rc.methods[0] != http.MethodPost || rc.paths[0] != "/MainService.svc/json/GetBalance_json" {
		t.Fatalf("%s %s", rc.methods[0], rc.paths[0])
	}
	m := bodyOf(t, rc.bodies[0])
	if m["ApiKey"] != testAPIKey || m["SecretKey"] != testSecretKey || len(m) != 2 {
		t.Fatalf("body = %v", m)
	}
}

func TestGetBalanceClasses(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   Outcome
		bal    string
	}{
		{"string balance", 200, `{"Balance":"120000.5","CodeResponse":"100"}`, OutcomeOK, "120000.5"},
		{"numeric code", 200, `{"Balance":10,"CodeResponse":100}`, OutcomeOK, "10"},
		{"wrong keys", 200, `{"Balance":0,"CodeResponse":"101","ErrorMessage":"Authorize Failed"}`, OutcomeKeyRejected, ""},
		{"no balance", 200, `{"CodeResponse":"100"}`, OutcomeMalformedResponse, ""},
		{"balance not a number", 200, `{"Balance":"lots","CodeResponse":"100"}`, OutcomeMalformedResponse, ""},
		{"error body", 200, `{"error":"Invalid request"}`, OutcomeInvalidRequest, ""},
		{"unknown code", 200, `{"CodeResponse":"143"}`, OutcomeRejected, ""},
		{"not json", 200, `<html>maintenance</html>`, OutcomeMalformedResponse, ""},
		{"empty object", 200, `{}`, OutcomeMalformedResponse, ""},
		{"429", 429, `{}`, OutcomeUnavailable, ""},
		{"503", 503, `oops`, OutcomeUnavailable, ""},
		{"408", 408, ``, OutcomeUnavailable, ""},
		{"400", 400, `{}`, OutcomeRejected, ""},
		{"redirect", 302, ``, OutcomeRejected, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := start(t, jsonAnswer(tc.status, tc.body))
			b, o := c.GetBalance(context.Background(), testCreds)
			if o != tc.want || b != tc.bal {
				t.Fatalf("= %q %v, want %q %v", b, o, tc.bal, tc.want)
			}
		})
	}
}

func TestEmptyCredentialsNeverReachESMS(t *testing.T) {
	c, rc := start(t, jsonAnswer(200, `{"CodeResult":"100"}`))
	empty := Credentials{APIKey: secret.Secret(testAPIKey)}
	if _, o := c.GetBalance(context.Background(), empty); o != OutcomeKeyRejected {
		t.Fatalf("balance = %v", o)
	}
	if _, o := c.ListCSKHTemplates(context.Background(), empty, testBrand); o != OutcomeKeyRejected {
		t.Fatalf("templates = %v", o)
	}
	if _, o := c.EstimateCSKH(context.Background(), empty, testBrand, "x", false); o != OutcomeKeyRejected {
		t.Fatalf("estimate = %v", o)
	}
	if _, o := c.SendCSKH(context.Background(), empty, Message{Phone: testPhone, Content: "x", Brandname: testBrand,
		RequestID: testRequestID}); o != OutcomeKeyRejected {
		t.Fatalf("send = %v", o)
	}
	if len(rc.paths) != 0 {
		t.Fatalf("eSMS was asked %d times", len(rc.paths))
	}
}

func TestNetworkFailureIsUnavailable(t *testing.T) {
	c, _ := start(t, jsonAnswer(200, `{}`))
	c.base = "https://127.0.0.1:1/MainService.svc/json" // nothing listens on port 1
	if _, o := c.GetBalance(context.Background(), testCreds); o != OutcomeUnavailable || !o.Retryable() {
		t.Fatalf("= %v", o)
	}
}

func TestOversizedBodyIsMalformed(t *testing.T) {
	big := `{"CodeResponse":"100","Balance":1,"pad":"` + strings.Repeat("x", maxBody) + `"}`
	c, _ := start(t, jsonAnswer(200, big))
	if _, o := c.GetBalance(context.Background(), testCreds); o != OutcomeMalformedResponse {
		t.Fatalf("= %v", o)
	}
}

func TestListCSKHTemplatesReadsEsmsShape(t *testing.T) {
	// The documentation's own sample, trimmed — numeric TempId and NetworkID.
	c, rc := start(t, jsonAnswer(200, `{"BrandnameTemplates":[
		{"NetworkID":6,"TempContent":"{P1,20} la ma xac minh dang ky XaMau cua ban","TempId":458},
		{"NetworkID":"2","TempContent":"Cam on quy khach","TempId":"12640"}],
		"CodeResult":"100","ErrorMessage":"success"}`))
	ts, o := c.ListCSKHTemplates(context.Background(), testCreds, testBrand)
	if o != OutcomeOK || len(ts) != 2 {
		t.Fatalf("= %v %v", ts, o)
	}
	if ts[0] != (Template{ID: "458", NetworkID: 6, Content: "{P1,20} la ma xac minh dang ky XaMau cua ban"}) ||
		ts[1] != (Template{ID: "12640", NetworkID: 2, Content: "Cam on quy khach"}) {
		t.Fatalf("templates = %+v", ts)
	}
	if rc.paths[0] != "/MainService.svc/json/GetTemplate/" {
		t.Fatalf("path %q", rc.paths[0])
	}
	m := bodyOf(t, rc.bodies[0])
	if m["ApiKey"] != testAPIKey || m["SecretKey"] != testSecretKey || m["SmsType"] != "2" || m["Brandname"] != testBrand || len(m) != 4 {
		t.Fatalf("body = %v", m)
	}
}

func TestListCSKHTemplatesClasses(t *testing.T) {
	for name, tc := range map[string]struct {
		body string
		want Outcome
	}{
		"brandname inactive": {`{"CodeResult":"104","ErrorMessage":"Brand name code is not exist"}`, OutcomeBrandnameInactive},
		"wrong keys":         {`{"CodeResult":"101"}`, OutcomeKeyRejected},
		"no TempId":          {`{"CodeResult":"100","BrandnameTemplates":[{"NetworkID":1,"TempContent":"x"}]}`, OutcomeMalformedResponse},
		"bad network":        {`{"CodeResult":"100","BrandnameTemplates":[{"NetworkID":0,"TempContent":"x","TempId":1}]}`, OutcomeMalformedResponse},
		"empty list":         {`{"CodeResult":"100","BrandnameTemplates":[]}`, OutcomeOK},
	} {
		t.Run(name, func(t *testing.T) {
			c, _ := start(t, jsonAnswer(200, tc.body))
			if _, o := c.ListCSKHTemplates(context.Background(), testCreds, testBrand); o != tc.want {
				t.Fatalf("= %v, want %v", o, tc.want)
			}
		})
	}
}

func TestEstimateUsesTheFakeProbeNumberNeverARecipient(t *testing.T) {
	c, rc := start(t, jsonAnswer(200, `{"CodeResult":"100","ErrorMessage":"success","TotalPrice":850,"TotalReceiver":1}`))
	p, o := c.EstimateCSKH(context.Background(), testCreds, testBrand, "Cam on quy khach", false)
	if o != OutcomeOK || p != "850" {
		t.Fatalf("= %q %v", p, o)
	}
	q := rc.queries[0]
	if rc.methods[0] != http.MethodGet || rc.paths[0] != "/MainService.svc/json/SummaryMultipleMessage_V4_get" {
		t.Fatalf("%s %s", rc.methods[0], rc.paths[0])
	}
	if q.Get("Phone") != EstimateProbePhone || q.Get("SmsType") != "2" || q.Get("BrandName") != testBrand ||
		q.Get("Content") != "Cam on quy khach" || q.Get("IsUnicode") != "0" || q.Get("ApiKey") != testAPIKey ||
		q.Get("SecretKey") != testSecretKey || len(q) != 7 {
		t.Fatalf("query = %v", q)
	}
	c2, _ := start(t, jsonAnswer(200, `{"CodeResult":"100","TotalPrice":-1}`))
	if _, o := c2.EstimateCSKH(context.Background(), testCreds, testBrand, "x", true); o != OutcomeMalformedResponse {
		t.Fatalf("negative price = %v", o)
	}
}

func TestSendCSKHBodyAndSMSID(t *testing.T) {
	c, rc := start(t, jsonAnswer(200, `{"CodeResult":"100","CountRegenerate":0,"SMSID":"d533459ee42b2b9525ba9eabf6a8156"}`))
	id, o := c.SendCSKH(context.Background(), testCreds, Message{Phone: testPhone, Content: "Cam on quy khach",
		Brandname: testBrand, IsUnicode: true, RequestID: testRequestID, CampaignID: "01JCAMPAIGN000000000000001"})
	if o != OutcomeOK || id != "d533459ee42b2b9525ba9eabf6a8156" || !o.Accepted() {
		t.Fatalf("= %q %v", id, o)
	}
	if rc.paths[0] != "/MainService.svc/json/SendMultipleMessage_V4_post_json/" {
		t.Fatalf("path %q", rc.paths[0])
	}
	m := bodyOf(t, rc.bodies[0])
	want := map[string]string{"ApiKey": testAPIKey, "SecretKey": testSecretKey, "Phone": testPhone,
		"Content": "Cam on quy khach", "Brandname": testBrand, "SmsType": "2", "IsUnicode": "1",
		"RequestId": testRequestID, "campaignid": "01JCAMPAIGN000000000000001", "Sandbox": "0"}
	if len(m) != len(want) {
		t.Fatalf("body = %v", m)
	}
	for k, v := range want {
		if m[k] != v {
			t.Errorf("%s = %q, want %q", k, m[k], v)
		}
	}
	// No CallbackUrl, no SendDate: ADR 0099 #9 leaves delivery callbacks to a later round.
	if _, ok := m["CallbackUrl"]; ok {
		t.Error("CallbackUrl sent")
	}
}

func TestSendCSKHSandboxFlagComesFromTheClient(t *testing.T) {
	c, rc := start(t, jsonAnswer(200, `{"CodeResult":"100","SMSID":"x1"}`))
	c.sandbox = true
	if _, o := c.SendCSKH(context.Background(), testCreds, Message{Phone: testPhone, Content: "x", Brandname: testBrand,
		RequestID: testRequestID}); o != OutcomeOK {
		t.Fatalf("= %v", o)
	}
	if m := bodyOf(t, rc.bodies[0]); m["Sandbox"] != "1" {
		t.Fatalf("Sandbox = %q", m["Sandbox"])
	}
}

func TestSendCSKHClasses(t *testing.T) {
	for name, tc := range map[string]struct {
		status   int
		body     string
		want     Outcome
		accepted bool
	}{
		"duplicate request id": {200, `{"CodeResult":"124","ErrorMessage":"Request exist 1","SendStatus":"5"}`, OutcomeDuplicateRequest, true},
		"wrong keys":           {200, `{"CodeResult":"101","ErrorMessage":"Authorize Failed"}`, OutcomeKeyRejected, false},
		"brandname":            {200, `{"CodeResult":"104"}`, OutcomeBrandnameInactive, false},
		"template":             {200, `{"CodeResult":"146","SMSID":"3f5e"}`, OutcomeTemplateMismatch, false},
		"99 code":              {200, `{"CodeResult":"99"}`, OutcomeInvalidRequest, false},
		"99 error body":        {200, `{"error":"Invalid request"}`, OutcomeInvalidRequest, false},
		"low balance":          {200, `{"CodeResult":"103"}`, OutcomeRejected, false},
		"100 without SMSID":    {200, `{"CodeResult":"100"}`, OutcomeMalformedResponse, false},
		"server error":         {502, ``, OutcomeUnavailable, false},
		"rate limited":         {429, ``, OutcomeUnavailable, false},
	} {
		t.Run(name, func(t *testing.T) {
			c, _ := start(t, jsonAnswer(tc.status, tc.body))
			id, o := c.SendCSKH(context.Background(), testCreds, Message{Phone: testPhone, Content: "x", Brandname: testBrand,
				RequestID: testRequestID})
			if o != tc.want || o.Accepted() != tc.accepted || id != "" {
				t.Fatalf("= %q %v (accepted %v), want %v", id, o, o.Accepted(), tc.want)
			}
		})
	}
}

func TestSendCSKHRefusesAnIncompleteMessageWithoutAskingESMS(t *testing.T) {
	c, rc := start(t, jsonAnswer(200, `{"CodeResult":"100","SMSID":"x"}`))
	full := Message{Phone: testPhone, Content: "x", Brandname: testBrand, RequestID: testRequestID}
	for name, mut := range map[string]func(*Message){
		"phone":      func(m *Message) { m.Phone = "" },
		"content":    func(m *Message) { m.Content = "" },
		"brandname":  func(m *Message) { m.Brandname = "" },
		"request id": func(m *Message) { m.RequestID = "" },
		"long id":    func(m *Message) { m.RequestID = strings.Repeat("a", MaxRequestIDLen+1) },
		"long camp":  func(m *Message) { m.CampaignID = strings.Repeat("a", maxCampaignIDLen+1) },
	} {
		m := full
		mut(&m)
		if _, o := c.SendCSKH(context.Background(), testCreds, m); o != OutcomeInvalidRequest {
			t.Errorf("%s: = %v", name, o)
		}
	}
	if len(rc.paths) != 0 {
		t.Fatalf("eSMS was asked %d times", len(rc.paths))
	}
}

// Nothing this package does may put a key, the phone number or the content into a log, and a Message
// formatted by mistake must not show them either.
func TestNothingLogsAndAMessageNeverRendersItsPhoneOrContent(t *testing.T) {
	logs := captureLogs(t)
	c, _ := start(t, jsonAnswer(200, `{"CodeResult":"101","ErrorMessage":"Authorize Failed `+testAPIKey+`"}`))
	m := Message{Phone: testPhone, Content: "Noi dung rieng", Brandname: testBrand, RequestID: testRequestID}
	_, _ = c.SendCSKH(context.Background(), testCreds, m)
	_, _ = c.GetBalance(context.Background(), testCreds)
	_, _ = c.EstimateCSKH(context.Background(), testCreds, testBrand, "x", false)
	slog.Info("a caller logs the message", "m", m)
	rendered := fmt.Sprintf("%v %+v %#v %s", m, m, m, m) + logs.String() + fmt.Sprint(testCreds)
	for _, bad := range []string{testAPIKey, testSecretKey, testPhone, "Noi dung rieng"} {
		if strings.Contains(rendered, bad) {
			t.Fatalf("%q rendered: %s", bad, rendered)
		}
	}
}

func TestOutcomeStringsAreClassesOnly(t *testing.T) {
	seen := map[string]bool{}
	for o := OutcomeOK; o <= OutcomeMalformedResponse; o++ {
		s := o.String()
		if s == "unspecified" || seen[s] {
			t.Fatalf("%d = %q", o, s)
		}
		seen[s] = true
		if o.Retryable() != (o == OutcomeUnavailable) {
			t.Errorf("%v retryable = %v", o, o.Retryable())
		}
	}
}
