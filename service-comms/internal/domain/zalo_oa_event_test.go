package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

const fakeOAWebhookKey = "oa-webhook-key-FAKE-NOT-REAL"

// signZaloOAEvent is Zalo's documented scheme, written independently of the code under test.
func signZaloOAEvent(appID, timestamp string, body []byte, key string) string {
	sum := sha256.Sum256([]byte(appID + string(body) + timestamp + key))
	return "mac=" + hex.EncodeToString(sum[:])
}

func TestZaloOAEventOAIDRule(t *testing.T) {
	cases := map[string]struct {
		body string
		want string
	}{
		"top-level oa_id wins": {`{"app_id":"1","timestamp":"2","event_name":"follow","oa_id":"111","sender":{"id":"222"}}`, "111"},
		"user_ → recipient":    {`{"app_id":"1","timestamp":"2","event_name":"user_send_text","sender":{"id":"222"},"recipient":{"id":"333"}}`, "333"},
		"oa_ → sender":         {`{"app_id":"1","timestamp":"2","event_name":"oa_send_text","sender":{"id":"444"},"recipient":{"id":"555"}}`, "444"},
		"other → none":         {`{"app_id":"1","timestamp":"2","event_name":"change_template_status","sender":{"id":"444"}}`, ""},
		// A malformed top-level oa_id is "none" — never a fall-back to another field.
		"bad oa_id → none":        {`{"app_id":"1","timestamp":"2","event_name":"user_send_text","oa_id":"abc","recipient":{"id":"333"}}`, ""},
		"numeric ids are text":    {`{"app_id":1,"timestamp":2,"event_name":"oa_send_text","sender":{"id":4440}}`, "4440"},
		"user_ without recipient": {`{"app_id":"1","timestamp":"2","event_name":"user_send_text","sender":{"id":"222"}}`, ""},
	}
	for name, c := range cases {
		e, err := ParseZaloOAEvent([]byte(c.body))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if e.OAID != c.want {
			t.Errorf("%s: OA %q, want %q", name, e.OAID, c.want)
		}
	}
}

func TestParseZaloOAEventRefusesMalformed(t *testing.T) {
	for _, body := range []string{``, `[]`, `{`, `{"timestamp":"2"}`, `{"app_id":"1"}`, `{"app_id":{},"timestamp":"2"}`} {
		if _, err := ParseZaloOAEvent([]byte(body)); err != ErrZaloOAEventMalformed {
			t.Errorf("%q: want ErrZaloOAEventMalformed, got %v", body, err)
		}
	}
}

func TestVerifyZaloOAEventSignature(t *testing.T) {
	body := []byte(`{"app_id":"4000000000000000001","timestamp":"1760000000000","event_name":"user_send_text","recipient":{"id":"111"}}`)
	e, err := ParseZaloOAEvent(body)
	if err != nil {
		t.Fatal(err)
	}
	good := signZaloOAEvent(e.AppID, e.Timestamp, body, fakeOAWebhookKey)
	if !VerifyZaloOAEventSignature(good, e, body, []byte(fakeOAWebhookKey)) {
		t.Fatal("a correct signature was refused")
	}
	// Upper-case hex is the same signature.
	if !VerifyZaloOAEventSignature("mac="+strings.ToUpper(strings.TrimPrefix(good, "mac=")), e, body, []byte(fakeOAWebhookKey)) {
		t.Error("upper-case hex refused")
	}
	tampered := []byte(strings.Replace(string(body), "111", "112", 1))
	for name, c := range map[string]struct {
		header string
		body   []byte
		key    string
	}{
		"missing":       {"", body, fakeOAWebhookKey},
		"no mac= label": {strings.TrimPrefix(good, "mac="), body, fakeOAWebhookKey},
		"not hex":       {"mac=zz", body, fakeOAWebhookKey},
		"short":         {"mac=abcd", body, fakeOAWebhookKey},
		"wrong key":     {good, body, "another-key"},
		"tampered body": {good, tampered, fakeOAWebhookKey},
		"empty key":     {signZaloOAEvent(e.AppID, e.Timestamp, body, ""), body, ""},
	} {
		if VerifyZaloOAEventSignature(c.header, e, c.body, []byte(c.key)) {
			t.Errorf("%s: verified", name)
		}
	}
}

func TestLoggableZaloOAEventName(t *testing.T) {
	if LoggableZaloOAEventName("user_send_text") != "user_send_text" {
		t.Error("a Zalo-shaped name must be logged as is")
	}
	for _, n := range []string{"", "0900000000 gọi", "User_Send", strings.Repeat("a", 65), "a\nb"} {
		if LoggableZaloOAEventName(n) != "unrecognised" {
			t.Errorf("%q reached the log", n)
		}
	}
}
