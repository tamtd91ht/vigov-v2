package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/vihat/vigov/core/events"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
)

const (
	oaEventsAppID   = "4000000000000000001"
	oaEventsKey     = "oa-webhook-key-FAKE-NOT-REAL"
	oaEventsCommune = tenant.ID("01J0000000000000000000000A")
	oaEventsOA      = "1234567890"
)

type fakeLiveOAs struct {
	byOA  map[string]tenant.ID
	err   error
	asked []string
}

func (f *fakeLiveOAs) TenantByLiveOAID(_ context.Context, oaID string) (tenant.ID, bool, error) {
	f.asked = append(f.asked, oaID)
	if f.err != nil {
		return "", false, f.err
	}
	t, ok := f.byOA[oaID]
	return t, ok, nil
}

type fakePublisher struct {
	got []events.Envelope
	err error
}

func (f *fakePublisher) Publish(_ context.Context, e events.Envelope) error {
	if f.err != nil {
		return f.err
	}
	f.got = append(f.got, e)
	return nil
}

func signedOAEvent(body []byte, appID, timestamp string) string {
	sum := sha256.Sum256([]byte(appID + string(body) + timestamp + oaEventsKey))
	return "mac=" + hex.EncodeToString(sum[:])
}

func newOAEventsRig(pub events.Publisher) (*ZaloOAEvents, *fakeLiveOAs) {
	live := &fakeLiveOAs{byOA: map[string]tenant.ID{oaEventsOA: oaEventsCommune}}
	z := NewZaloOAEvents(live, pub, ZaloOAEventsConfig{AppID: oaEventsAppID, WebhookSecret: secret.Secret(oaEventsKey)})
	z.now = func() time.Time { return time.Date(2026, 10, 17, 1, 2, 3, 0, time.UTC) }
	z.newID = func() (string, error) { return "01J0000000000000000000000E", nil }
	return z, live
}

// A user event to the commune's OA, with whitespace a re-encoding would lose.
var userEventBody = []byte("{\"app_id\":\"" + oaEventsAppID + "\", \"timestamp\":\"1760000000000\",\n" +
	"\"event_name\":\"user_send_text\",\"sender\":{\"id\":\"9988\"},\"recipient\":{\"id\":\"" + oaEventsOA + "\"}," +
	"\"message\":{\"text\":\"xin chào\",\"msg_id\":\"m1\"}}")

func TestOAEventPublishedWithTenantAndRawBody(t *testing.T) {
	pub := &fakePublisher{}
	z, _ := newOAEventsRig(pub)
	res, err := z.Receive(context.Background(), signedOAEvent(userEventBody, oaEventsAppID, "1760000000000"), userEventBody)
	if err != nil || res.Outcome != ZaloOAEventPublished {
		t.Fatalf("res %+v err %v", res, err)
	}
	if len(pub.got) != 1 {
		t.Fatalf("published %d envelopes", len(pub.got))
	}
	e := pub.got[0]
	if e.TenantID != oaEventsCommune || e.Name != ZaloOAEventReceivedName || e.ID == "" || e.At.IsZero() {
		t.Errorf("envelope %+v", e)
	}
	if !bytes.Equal(e.Payload, userEventBody) {
		t.Errorf("payload is not the raw body byte for byte:\n%s\n%s", e.Payload, userEventBody)
	}
}

func TestOAEventSignatureAndAppRefused(t *testing.T) {
	wrongApp := bytes.Replace(userEventBody, []byte(oaEventsAppID), []byte("4000000000000000002"), 1)
	cases := map[string]struct {
		sig  string
		body []byte
	}{
		"missing signature": {"", userEventBody},
		"bad signature":     {"mac=" + hex.EncodeToString(make([]byte, 32)), userEventBody},
		// Correctly signed with the key, but by another app: still refused.
		"wrong app_id":    {signedOAEvent(wrongApp, "4000000000000000002", "1760000000000"), wrongApp},
		"malformed body":  {"mac=00", []byte("not json")},
		"signature reuse": {signedOAEvent(userEventBody, oaEventsAppID, "1760000000000"), append(append([]byte{}, userEventBody...), ' ')},
	}
	for name, c := range cases {
		pub := &fakePublisher{}
		z, live := newOAEventsRig(pub)
		_, err := z.Receive(context.Background(), c.sig, c.body)
		if !errors.Is(err, ErrZaloOAEventForbidden) {
			t.Errorf("%s: want ErrZaloOAEventForbidden, got %v", name, err)
		}
		if len(pub.got) != 0 || len(live.asked) != 0 {
			t.Errorf("%s: a refused event reached the lookup or the broker", name)
		}
	}
}

func TestOAEventResolution(t *testing.T) {
	cases := map[string]struct {
		body string
		oa   string
	}{
		"user_ → recipient.id": {`{"app_id":"` + oaEventsAppID + `","timestamp":"7","event_name":"user_send_image","sender":{"id":"1"},"recipient":{"id":"` + oaEventsOA + `"}}`, oaEventsOA},
		"oa_ → sender.id":      {`{"app_id":"` + oaEventsAppID + `","timestamp":"7","event_name":"oa_send_text","sender":{"id":"` + oaEventsOA + `"},"recipient":{"id":"1"}}`, oaEventsOA},
		"top-level oa_id":      {`{"app_id":"` + oaEventsAppID + `","timestamp":"7","event_name":"follow","oa_id":"` + oaEventsOA + `","follower":{"id":"1"}}`, oaEventsOA},
	}
	for name, c := range cases {
		pub := &fakePublisher{}
		z, live := newOAEventsRig(pub)
		res, err := z.Receive(context.Background(), signedOAEvent([]byte(c.body), oaEventsAppID, "7"), []byte(c.body))
		if err != nil || res.Outcome != ZaloOAEventPublished || res.Tenant != oaEventsCommune {
			t.Errorf("%s: res %+v err %v", name, res, err)
		}
		if len(live.asked) != 1 || live.asked[0] != c.oa {
			t.Errorf("%s: looked up %v, want %q", name, live.asked, c.oa)
		}
		if len(pub.got) != 1 || pub.got[0].TenantID != oaEventsCommune {
			t.Errorf("%s: published %+v", name, pub.got)
		}
	}
}

// Unknown or not-live OA, or no OA at all: authenticated, answered without error, NOTHING published.
func TestOAEventWithoutLiveOAIsNotPublished(t *testing.T) {
	cases := map[string]struct {
		body string
		want ZaloOAEventOutcome
	}{
		"unknown OA": {`{"app_id":"` + oaEventsAppID + `","timestamp":"7","event_name":"user_send_text","recipient":{"id":"5555"}}`, ZaloOAEventUnknownOA},
		"no OA":      {`{"app_id":"` + oaEventsAppID + `","timestamp":"7","event_name":"change_template_status"}`, ZaloOAEventNoOA},
	}
	for name, c := range cases {
		pub := &fakePublisher{}
		z, _ := newOAEventsRig(pub)
		res, err := z.Receive(context.Background(), signedOAEvent([]byte(c.body), oaEventsAppID, "7"), []byte(c.body))
		if err != nil || res.Outcome != c.want || res.Tenant != "" {
			t.Errorf("%s: res %+v err %v", name, res, err)
		}
		if len(pub.got) != 0 {
			t.Errorf("%s: published without a commune", name)
		}
	}
}

func TestOAEventPublisherOrLookupErrorIsUnavailable(t *testing.T) {
	sig := signedOAEvent(userEventBody, oaEventsAppID, "1760000000000")

	z, _ := newOAEventsRig(&fakePublisher{err: errors.New("kafka: publish: broker down")})
	if _, err := z.Receive(context.Background(), sig, userEventBody); !errors.Is(err, ErrZaloOAEventsUnavailable) {
		t.Errorf("broker error: want ErrZaloOAEventsUnavailable, got %v", err)
	}

	pub := &fakePublisher{}
	z, live := newOAEventsRig(pub)
	live.err = errors.New("db down")
	if _, err := z.Receive(context.Background(), sig, userEventBody); !errors.Is(err, ErrZaloOAEventsUnavailable) {
		t.Errorf("lookup error: want ErrZaloOAEventsUnavailable, got %v", err)
	}
	if len(pub.got) != 0 {
		t.Error("published after the lookup failed")
	}
}

func TestOAEventNotConfigured(t *testing.T) {
	live := &fakeLiveOAs{}
	for name, z := range map[string]*ZaloOAEvents{
		"no publisher": NewZaloOAEvents(live, nil, ZaloOAEventsConfig{AppID: oaEventsAppID, WebhookSecret: secret.Secret(oaEventsKey)}),
		"no secret":    NewZaloOAEvents(live, &fakePublisher{}, ZaloOAEventsConfig{AppID: oaEventsAppID}),
		"no app id":    NewZaloOAEvents(live, &fakePublisher{}, ZaloOAEventsConfig{WebhookSecret: secret.Secret(oaEventsKey)}),
	} {
		if _, err := z.Receive(context.Background(), "mac=00", userEventBody); !errors.Is(err, ErrZaloOAEventsNotConfigured) {
			t.Errorf("%s: want ErrZaloOAEventsNotConfigured, got %v", name, err)
		}
	}
}
