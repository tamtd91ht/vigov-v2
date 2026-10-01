package zalo

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestAccountIDStringAndHeaders(t *testing.T) {
	c, cp := fakeZalo(t, 200, `{"id":"1234567890123456789","error":0,"message":"Success"}`)
	got, err := c.AccountID(context.Background(), fakeAccessToken)
	if err != nil {
		t.Fatalf("AccountID: %v", err)
	}
	if got != "1234567890123456789" {
		t.Errorf("id = %q", got)
	}
	if cp.method != http.MethodGet || cp.path != AccountIDPath || cp.query != "fields=id" {
		t.Errorf("request = %s %s?%s, want GET %s?fields=id", cp.method, cp.path, cp.query, AccountIDPath)
	}
	if v := cp.header.Get(HeaderAccessToken); v != fakeAccessToken {
		t.Errorf("access_token header = %q", v)
	}
	// The account-id call carries NO secret and NO phone token.
	for _, h := range []string{HeaderSecretKey, HeaderCode, HeaderAppSecretProof} {
		if _, ok := cp.header[http.CanonicalHeaderKey(h)]; ok {
			t.Errorf("header %s must not be sent on the account-id call", h)
		}
	}
}

func TestAccountIDNumericPreservedExactly(t *testing.T) {
	// 20 digits: float64 would round this (it holds ~15-17 significant digits).
	const large = "98765432109876543219"
	for body, want := range map[string]string{
		`{"id":12345,"error":0}`:             "12345",
		`{"id":` + large + `,"error":0}`:     large,
		`{"id":" ` + large + ` ","error":0}`: large,
	} {
		c, _ := fakeZalo(t, 200, body)
		got, err := c.AccountID(context.Background(), fakeAccessToken)
		if err != nil || got != want {
			t.Errorf("body %s -> (%q, %v), want %q", body, got, err, want)
		}
	}
}

func TestAccountIDUnreadableIsUnreachable(t *testing.T) {
	for _, body := range []string{
		`{"error":0}`,
		`{"id":null,"error":0}`,
		`{"id":"","error":0}`,
		`{"id":"   ","error":0}`,
		`{"id":1.5,"error":0}`,
		`{"id":-1,"error":0}`,
		`{"id":1e20,"error":0}`,
		`{"id":{},"error":0}`,
	} {
		c, _ := fakeZalo(t, 200, body)
		_, err := c.AccountID(context.Background(), fakeAccessToken)
		if !errors.Is(err, ErrZaloUnreachable) || errors.Is(err, ErrTokenInvalid) {
			t.Errorf("body %s: err = %v, want ErrZaloUnreachable only", body, err)
		}
	}
}

func TestAccountIDZaloErrorIsTokenInvalid(t *testing.T) {
	c, _ := fakeZalo(t, 200, `{"id":"1234567890123456789","error":-216,"message":"Access token is invalid"}`)
	_, err := c.AccountID(context.Background(), fakeAccessToken)
	if !errors.Is(err, ErrTokenInvalid) || errors.Is(err, ErrZaloUnreachable) {
		t.Fatalf("err = %v, want ErrTokenInvalid only", err)
	}
	if !strings.Contains(err.Error(), "-216") {
		t.Errorf("err %q should carry the numeric code", err)
	}
	assertClean(t, err, "Access token is invalid", "1234567890123456789")
}

func TestAccountIDMissingToken(t *testing.T) {
	if _, err := New("http://127.0.0.1:1").AccountID(context.Background(), ""); !errors.Is(err, ErrTokenInvalid) {
		t.Errorf("err = %v, want ErrTokenInvalid", err)
	}
}

func TestAccountIDUnreachable(t *testing.T) {
	for name, c := range unreachableCases(t) {
		_, err := c.AccountID(context.Background(), fakeAccessToken)
		if !errors.Is(err, ErrZaloUnreachable) || errors.Is(err, ErrTokenInvalid) {
			t.Errorf("%s: err = %v, want ErrZaloUnreachable only", name, err)
		}
		assertClean(t, err)
	}
}
