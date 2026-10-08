package ratelimit

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// The ACCOUNTLESS petition policies (ADR 0083, TEMPORARY). Pinned: the send and lookup figures are the
// owner's (rows 3 and 9), so changing one must turn a test red in the same commit.
func TestAccountlessThresholdsArePinned(t *testing.T) {
	for _, c := range []struct {
		p      Policy
		limit  int64
		window time.Duration
	}{
		{AccountlessSend, 5, time.Hour},
		{AccountlessLookup, 30, time.Hour},
		{AccountlessFieldRead, 120, time.Minute},
	} {
		if c.p.limit != c.limit || c.p.window != c.window {
			t.Errorf("%s = %d/%v, want %d/%v", c.p.name, c.p.limit, c.p.window, c.limit, c.window)
		}
		if c.p.FailsOpen() {
			t.Errorf("%s fails OPEN — ADR 0083 row 12 says a Redis outage refuses (503)", c.p.name)
		}
		if _, err := New(newFake(), c.p); err != nil {
			t.Errorf("%s: %v", c.p.name, err)
		}
	}
}

// The send policy's 429 names the reception desk; a Redis outage is a 503 for all three.
func TestAccountlessSendRefusalPointsToReceptionDesk(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	l, _ := New(newFake(), AccountlessSend)
	k, _ := PublicHostIPKey(context.Background(), false, "xa.vigov.vn", "203.0.113.7")
	var rec *httptest.ResponseRecorder
	for i := 0; i < AccountlessSendLimit+1; i++ {
		rec = httptest.NewRecorder()
		Gate(rec, gateReq(), l, k, log)
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("6th send = %d, want 429", rec.Code)
	}
	var e struct{ Code, Message string }
	_ = json.Unmarshal(rec.Body.Bytes(), &e)
	if e.Code != "rate_limited" || !strings.Contains(e.Message, "Bộ phận tiếp nhận của Ủy ban nhân dân xã") {
		t.Errorf("429 = %+v", e)
	}

	for _, p := range []Policy{AccountlessSend, AccountlessLookup, AccountlessFieldRead} {
		f := newFake()
		f.fail = errors.New("dial tcp: connection refused")
		lf, _ := New(f, p)
		rec := httptest.NewRecorder()
		if Gate(rec, gateReq(), lf, AccountlessLookupKey("203.0.113.7"), log) || rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s with Redis down = %d, want 503", p.name, rec.Code)
		}
	}
}

// The lookup key is per client network and carries no commune: naming more hosts buys no more budget.
func TestAccountlessLookupKeyIsPerNetworkOnly(t *testing.T) {
	f := newFake()
	l, _ := New(f, AccountlessLookup)
	_, _, _ = l.Allow(context.Background(), AccountlessLookupKey("2001:db8:1:2::7"))
	if len(f.keys) != 1 || f.keys[0] != "rl:accountless-lookup:ip:2001:db8:1:2::/64" {
		t.Errorf("key = %v", f.keys)
	}
}
