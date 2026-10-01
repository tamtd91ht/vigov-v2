package zalo

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/secret"
)

// Fake values only (rule 3, invariant 5). The secret is a test literal, never a real one.
const (
	fakeAccessToken = "fake-access-token"
	fakePhoneToken  = "fake-phone-token"
	fakeSecretText  = "fake-app-secret-for-tests"
)

var fakeSecret = secret.Secret(fakeSecretText)

// capture records the last request a fake Zalo received.
type capture struct {
	mu     sync.Mutex
	method string
	path   string
	query  string
	header http.Header
	calls  int
}

func (c *capture) record(r *http.Request) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.method, c.path, c.query = r.Method, r.URL.Path, r.URL.RawQuery
	c.header = r.Header.Clone()
	c.calls++
}

func fakeZalo(t *testing.T, status int, body string) (*Client, *capture) {
	t.Helper()
	cp := &capture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cp.record(r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return New(srv.URL), cp
}

// assertClean fails if an error string carries a credential or personal data.
func assertClean(t *testing.T, err error, forbidden ...string) {
	t.Helper()
	if err == nil {
		return
	}
	msg := err.Error()
	for _, f := range append([]string{fakeSecretText, fakeAccessToken, fakePhoneToken}, forbidden...) {
		if f != "" && strings.Contains(msg, f) {
			t.Errorf("error %q leaks %q", msg, f)
		}
	}
}

func TestPhoneHappyPathAndHeaders(t *testing.T) {
	c, cp := fakeZalo(t, 200, `{"data":{"number":"84900000000"},"error":0,"message":"Success"}`)
	got, err := c.Phone(context.Background(), fakeAccessToken, fakePhoneToken, fakeSecret)
	if err != nil {
		t.Fatalf("Phone: %v", err)
	}
	if got != "84900000000" {
		t.Errorf("number = %q, want 84900000000", got)
	}
	if cp.method != http.MethodGet || cp.path != InfoPath || cp.query != "" {
		t.Errorf("request = %s %s?%s, want GET %s", cp.method, cp.path, cp.query, InfoPath)
	}
	for name, want := range map[string]string{
		HeaderAccessToken: fakeAccessToken,
		HeaderCode:        fakePhoneToken,
		HeaderSecretKey:   fakeSecretText,
	} {
		if v := cp.header.Get(name); v != want {
			t.Errorf("header %s = %q, want %q", name, v, want)
		}
	}
	if _, ok := cp.header[http.CanonicalHeaderKey(HeaderAppSecretProof)]; ok {
		t.Error("appsecret_proof sent although the option is off by default")
	}
}

func TestPhoneNormalisesNumber(t *testing.T) {
	for raw, want := range map[string]string{
		"84900000000":     "84900000000",
		"0900000000":      "84900000000",
		"+84900000000":    "84900000000",
		" 84 900-000-000": "84900000000",
	} {
		c, _ := fakeZalo(t, 200, `{"data":{"number":"`+raw+`"},"error":0}`)
		got, err := c.Phone(context.Background(), fakeAccessToken, fakePhoneToken, fakeSecret)
		if err != nil || got != want {
			t.Errorf("number %q -> (%q, %v), want %q", raw, got, err, want)
		}
	}
}

func TestPhoneMalformedNumberIsUnreachable(t *testing.T) {
	for _, body := range []string{
		`{"data":{"number":""},"error":0}`,
		`{"data":{"number":"abc"},"error":0}`,
		`{"data":{"number":"12"},"error":0}`,
		`{"error":0}`,
	} {
		c, _ := fakeZalo(t, 200, body)
		_, err := c.Phone(context.Background(), fakeAccessToken, fakePhoneToken, fakeSecret)
		if !errors.Is(err, ErrZaloUnreachable) || errors.Is(err, ErrTokenInvalid) {
			t.Errorf("body %s: err = %v, want ErrZaloUnreachable only", body, err)
		}
		assertClean(t, err, "abc")
	}
}

func TestPhoneZaloErrorIsTokenInvalid(t *testing.T) {
	c, _ := fakeZalo(t, 200, `{"error":452,"message":"Session key invalid 84900000000"}`)
	_, err := c.Phone(context.Background(), fakeAccessToken, fakePhoneToken, fakeSecret)
	if !errors.Is(err, ErrTokenInvalid) || errors.Is(err, ErrZaloUnreachable) {
		t.Fatalf("err = %v, want ErrTokenInvalid only", err)
	}
	if !strings.Contains(err.Error(), "452") {
		t.Errorf("err %q should carry the numeric code", err)
	}
	assertClean(t, err, "Session key invalid", "84900000000")
}

func TestPhoneMissingInputs(t *testing.T) {
	c := New("http://127.0.0.1:1") // never reached
	if _, err := c.Phone(context.Background(), "", fakePhoneToken, fakeSecret); !errors.Is(err, ErrTokenInvalid) {
		t.Errorf("empty accessToken: %v, want ErrTokenInvalid", err)
	}
	if _, err := c.Phone(context.Background(), fakeAccessToken, "", fakeSecret); !errors.Is(err, ErrTokenInvalid) {
		t.Errorf("empty phoneToken: %v, want ErrTokenInvalid", err)
	}
	if _, err := c.Phone(context.Background(), fakeAccessToken, fakePhoneToken, nil); !errors.Is(err, ErrZaloUnreachable) {
		t.Errorf("empty secret: %v, want ErrZaloUnreachable (our configuration, not the citizen)", err)
	}
}

func TestPhoneAppSecretProofWhenEnabled(t *testing.T) {
	cp := &capture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cp.record(r)
		_, _ = w.Write([]byte(`{"data":{"number":"84900000000"},"error":0}`))
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, WithAppSecretProof())
	if _, err := c.Phone(context.Background(), fakeAccessToken, fakePhoneToken, fakeSecret); err != nil {
		t.Fatalf("Phone: %v", err)
	}
	mac := hmac.New(sha256.New, []byte(fakeSecretText))
	mac.Write([]byte(fakeAccessToken))
	want := hex.EncodeToString(mac.Sum(nil))
	if got := cp.header.Get(HeaderAppSecretProof); got != want {
		t.Errorf("appsecret_proof = %q, want %q", got, want)
	}
	if got := AppSecretProof(fakeAccessToken, fakeSecret); got != want {
		t.Errorf("AppSecretProof = %q, want %q", got, want)
	}
}

// unreachableCases covers every infrastructure failure both calls share through do().
func unreachableCases(t *testing.T) map[string]*Client {
	t.Helper()
	cases := map[string]*Client{}

	c500, _ := fakeZalo(t, 500, `{"error":0}`)
	cases["http 500"] = c500
	c403, _ := fakeZalo(t, 403, `{"error":0}`)
	cases["http 403"] = c403
	cGarbage, _ := fakeZalo(t, 200, `<html>84900000000</html>`)
	cases["garbage"] = cGarbage
	big := `{"pad":"` + strings.Repeat("x", bodyLimit) + `","error":0}`
	cBig, _ := fakeZalo(t, 200, big)
	cases["oversized"] = cBig

	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	t.Cleanup(slow.Close)
	cases["timeout"] = New(slow.URL, WithHTTPClient(&http.Client{Timeout: 50 * time.Millisecond}))

	closed := httptest.NewServer(http.NotFoundHandler())
	closedURL := closed.URL
	closed.Close()
	cases["network"] = New(closedURL)
	return cases
}

func TestPhoneUnreachable(t *testing.T) {
	for name, c := range unreachableCases(t) {
		_, err := c.Phone(context.Background(), fakeAccessToken, fakePhoneToken, fakeSecret)
		if !errors.Is(err, ErrZaloUnreachable) || errors.Is(err, ErrTokenInvalid) {
			t.Errorf("%s: err = %v, want ErrZaloUnreachable only", name, err)
		}
		assertClean(t, err, "84900000000")
	}
}

func TestPhoneTimeoutIsNamed(t *testing.T) {
	c := unreachableCases(t)["timeout"]
	_, err := c.Phone(context.Background(), fakeAccessToken, fakePhoneToken, fakeSecret)
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Errorf("err = %v, want a timeout", err)
	}
}

func TestDefaultsAndBaseURL(t *testing.T) {
	c := New("  ")
	if c.baseURL != DefaultBaseURL {
		t.Errorf("baseURL = %q, want %q", c.baseURL, DefaultBaseURL)
	}
	if c.hc.Timeout != 8*time.Second {
		t.Errorf("timeout = %v, want 8s", c.hc.Timeout)
	}
	if c.sendAppSecretProof {
		t.Error("appsecret_proof must be off by default")
	}
	if New("http://x/").baseURL != "http://x" {
		t.Error("trailing slash not trimmed")
	}
	if accountIDTimeout != 6*time.Second {
		t.Errorf("account id timeout = %v, want 6s", accountIDTimeout)
	}
}
