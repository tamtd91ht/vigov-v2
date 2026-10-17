package http

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/events"
	"github.com/vihat/vigov/core/ratelimit"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// THE OA WEBHOOK, through the real use case with a fake broker and a fake live-OA lookup. What these
// tests defend (owner decision 17/10/2026, TASK W-1):
//
//  1. a missing / wrong signature or another app's event is 403 and NOTHING is published;
//  2. the commune comes from the event's OA (user_ → recipient, oa_ → sender, top-level oa_id);
//  3. an OA live in no commune is 200 and NOTHING is published;
//  4. 200 only after the broker acknowledged — a broker error is 503;
//  5. the envelope carries the commune and the raw body byte for byte;
//  6. no log line carries the body, the OA id, a user id or the message text.

// Fixtures, not credentials.
const (
	whAppID = "4000000000000000001"
	whKey   = "oa-webhook-key-FAKE-NOT-REAL"
	whOA    = "579745863508352884"
	whUser  = "8800000000000000077"
	whText  = "noi-dung-tin-nhan-cua-nguoi-dan"
	whHost  = "oa-callback.example.vn"
)

type whLiveOAs struct{ byOA map[string]tenant.ID }

func (f whLiveOAs) TenantByLiveOAID(_ context.Context, oaID string) (tenant.ID, bool, error) {
	t, ok := f.byOA[oaID]
	return t, ok, nil
}

type whPublisher struct {
	got []events.Envelope
	err error
}

func (f *whPublisher) Publish(_ context.Context, e events.Envelope) error {
	if f.err != nil {
		return f.err
	}
	f.got = append(f.got, e)
	return nil
}

type oaWebhookServer struct {
	h       http.Handler
	pub     *whPublisher
	counter *memCounter
	logs    *bytes.Buffer
}

func newOAWebhookServer(t *testing.T, pub events.Publisher) *oaWebhookServer {
	t.Helper()
	counter := &memCounter{}
	lim, err := ratelimit.New(counter, ratelimit.ZaloOAWebhook)
	if err != nil {
		t.Fatal(err)
	}
	logs := &bytes.Buffer{}
	uc := app.NewZaloOAEvents(whLiveOAs{byOA: map[string]tenant.ID{whOA: xaA}}, pub,
		app.ZaloOAEventsConfig{AppID: whAppID, WebhookSecret: secret.Secret(whKey)})
	mux := http.NewServeMux()
	RegisterZaloOAEvents(mux, ZaloOAEventDeps{Events: uc, Limiter: lim, Log: slog.New(slog.NewTextHandler(logs, nil))})
	s := &oaWebhookServer{h: mux, counter: counter, logs: logs}
	if p, ok := pub.(*whPublisher); ok {
		s.pub = p
	}
	return s
}

func whSign(body []byte, appID, ts string) string {
	sum := sha256.Sum256([]byte(appID + string(body) + ts + whKey))
	return "mac=" + hex.EncodeToString(sum[:])
}

func whUserEvent() []byte {
	return []byte(`{"app_id":"` + whAppID + `","timestamp":"1760000000000","event_name":"user_send_text",` +
		`"sender":{"id":"` + whUser + `"},"recipient":{"id":"` + whOA + `"},"message":{"text":"` + whText + `","msg_id":"m1"}}`)
}

func (s *oaWebhookServer) post(t *testing.T, sig string, body []byte, ctx context.Context) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "https://"+whHost+ZaloOAEventsPath, bytes.NewReader(body)).WithContext(ctx)
	r.Host = whHost
	r.RemoteAddr = "203.0.113.9:443"
	if sig != "" {
		r.Header.Set(domain.ZaloOAEventSignatureHeader, sig)
	}
	w := httptest.NewRecorder()
	s.h.ServeHTTP(w, r)
	return w
}

func (s *oaWebhookServer) noPersonalDataInLogs(t *testing.T) {
	t.Helper()
	for _, leak := range []string{whOA, whUser, whText, whKey, "mac="} {
		if strings.Contains(s.logs.String(), leak) {
			t.Errorf("a log line carries %q: %s", leak, s.logs.String())
		}
	}
}

func TestZaloOAEventsPathMatchesRoute(t *testing.T) {
	if ZaloOAEventsPath != "/api/v1/zalo-oa-events" {
		t.Fatalf("ZaloOAEventsPath = %q — cmd/server mounts this constant, the route spells the literal", ZaloOAEventsPath)
	}
}

func TestOAWebhookPublishesTenantAndRawBody(t *testing.T) {
	s := newOAWebhookServer(t, &whPublisher{})
	body := whUserEvent()
	w := s.post(t, whSign(body, whAppID, "1760000000000"), body, context.Background())
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if len(s.pub.got) != 1 {
		t.Fatalf("published %d", len(s.pub.got))
	}
	e := s.pub.got[0]
	if e.TenantID != xaA || e.Name != "comms.zalo_oa_event_received.v1" {
		t.Errorf("envelope %+v", e)
	}
	if !bytes.Equal(e.Payload, whUserEvent()) {
		t.Errorf("payload is not the raw body byte for byte: %s", e.Payload)
	}
	s.noPersonalDataInLogs(t)
}

func TestOAWebhookRefusesBadSignatures(t *testing.T) {
	body := whUserEvent()
	otherApp := bytes.Replace(body, []byte(whAppID), []byte("4000000000000000002"), 1)
	cases := map[string]struct {
		sig  string
		body []byte
	}{
		"missing":   {"", body},
		"bad":       {"mac=" + hex.EncodeToString(make([]byte, 32)), body},
		"wrong app": {whSign(otherApp, "4000000000000000002", "1760000000000"), otherApp},
		"not json":  {"mac=00", []byte("hello")},
	}
	for name, c := range cases {
		s := newOAWebhookServer(t, &whPublisher{})
		w := s.post(t, c.sig, c.body, context.Background())
		if w.Code != http.StatusForbidden {
			t.Errorf("%s: status %d, want 403", name, w.Code)
		}
		if len(s.pub.got) != 0 {
			t.Errorf("%s: published a refused event", name)
		}
		if !strings.Contains(s.logs.String(), "zalo_oa_webhook.signature_refused") {
			t.Errorf("%s: the refusal is a security event and must be logged", name)
		}
		s.noPersonalDataInLogs(t)
	}
}

func TestOAWebhookResolvesOAByRule(t *testing.T) {
	for name, body := range map[string]string{
		"oa_ → sender":    `{"app_id":"` + whAppID + `","timestamp":"5","event_name":"oa_send_text","sender":{"id":"` + whOA + `"},"recipient":{"id":"` + whUser + `"}}`,
		"top-level oa_id": `{"app_id":"` + whAppID + `","timestamp":"5","event_name":"follow","oa_id":"` + whOA + `","follower":{"id":"` + whUser + `"}}`,
		"user_ → recipient": `{"app_id":"` + whAppID + `","timestamp":"5","event_name":"user_send_image","sender":{"id":"` + whUser +
			`"},"recipient":{"id":"` + whOA + `"}}`,
	} {
		s := newOAWebhookServer(t, &whPublisher{})
		w := s.post(t, whSign([]byte(body), whAppID, "5"), []byte(body), context.Background())
		if w.Code != http.StatusOK || len(s.pub.got) != 1 || s.pub.got[0].TenantID != xaA {
			t.Errorf("%s: status %d, published %+v", name, w.Code, s.pub.got)
		}
	}
}

func TestOAWebhookUnknownOAIs200AndNotPublished(t *testing.T) {
	s := newOAWebhookServer(t, &whPublisher{})
	body := []byte(`{"app_id":"` + whAppID + `","timestamp":"5","event_name":"user_send_text","recipient":{"id":"1111"},"message":{"text":"` + whText + `"}}`)
	w := s.post(t, whSign(body, whAppID, "5"), body, context.Background())
	if w.Code != http.StatusOK {
		t.Fatalf("status %d, want 200 so Zalo stops retrying", w.Code)
	}
	if len(s.pub.got) != 0 {
		t.Fatal("an event of no live OA was published")
	}
	if !strings.Contains(s.logs.String(), "event_name=user_send_text") || strings.Contains(s.logs.String(), "1111") {
		t.Errorf("the ignored event must log its name only: %s", s.logs.String())
	}
	s.noPersonalDataInLogs(t)
}

func TestOAWebhookBrokerErrorIs503(t *testing.T) {
	s := newOAWebhookServer(t, &whPublisher{err: errors.New("kafka: publish to \"comms.zalo_oa_event_received.v1\": broker down")})
	body := whUserEvent()
	w := s.post(t, whSign(body, whAppID, "1760000000000"), body, context.Background())
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d, want 503 so Zalo retries", w.Code)
	}
	s.noPersonalDataInLogs(t)
}

// KAFKA_EVENT_ADDRESS unset (dev): every event 503 — refused at use, never dropped with a 200.
func TestOAWebhookWithoutBrokerIs503(t *testing.T) {
	s := newOAWebhookServer(t, nil)
	body := whUserEvent()
	if w := s.post(t, whSign(body, whAppID, "1760000000000"), body, context.Background()); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d, want 503", w.Code)
	}
}

// The limit is counted BEFORE the body is read, and fails closed.
func TestOAWebhookRateLimitFailsClosed(t *testing.T) {
	s := newOAWebhookServer(t, &whPublisher{})
	s.counter.fail = errors.New("dial tcp: connection refused")
	body := whUserEvent()
	if w := s.post(t, whSign(body, whAppID, "1760000000000"), body, context.Background()); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d, want 503 with the counter down", w.Code)
	}
	if len(s.pub.got) != 0 {
		t.Fatal("published with the rate limit down")
	}
	if len(s.counter.keys) != 1 || s.counter.keys[0] != "rl:zalo-oa-webhook:ip:203.0.113.9" {
		t.Errorf("counter keys %v", s.counter.keys)
	}
}

func TestOAWebhookTooLargeIs413(t *testing.T) {
	s := newOAWebhookServer(t, &whPublisher{})
	body := append([]byte(`{"app_id":"`+whAppID+`","timestamp":"5","pad":"`), bytes.Repeat([]byte("a"), maxZaloOAEventBytes)...)
	body = append(body, []byte(`"}`)...)
	if w := s.post(t, whSign(body, whAppID, "5"), body, context.Background()); w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d, want 413", w.Code)
	}
	if len(s.pub.got) != 0 {
		t.Fatal("published an oversized body")
	}
}

// Mounted behind a commune Host by mistake: refused, never published under Host's commune.
func TestOAWebhookRefusesACommuneContext(t *testing.T) {
	s := newOAWebhookServer(t, &whPublisher{})
	body := whUserEvent()
	w := s.post(t, whSign(body, whAppID, "1760000000000"), body, tenant.Into(context.Background(), xaB))
	if w.Code != http.StatusNotFound || len(s.pub.got) != 0 {
		t.Fatalf("status %d, published %d", w.Code, len(s.pub.got))
	}
}
