package http

// THE PUBLIC NEWS RATE LIMIT (ratelimit.PublicNewsRead, owner 02/10/2026) on the three public routes:
// 120 per minute per (host, client network), keyed `t:<tenant_id>:…` once the host resolved, 429 with
// Retry-After past it, and SERVING when the counter store is down (the owner's fail-open exception).

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/ratelimit"
)

// memCounter is an in-memory fixed-window counter: one window that never ends unless reset, which is
// all these tests need. Recording every key it was asked for.
type memCounter struct {
	mu   sync.Mutex
	n    map[string]int64
	keys []string
	fail error
}

func (m *memCounter) Incr(_ context.Context, key string, window time.Duration) (int64, time.Duration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.keys = append(m.keys, key)
	if m.fail != nil {
		return 0, 0, m.fail
	}
	if m.n == nil {
		m.n = map[string]int64{}
	}
	m.n[key]++
	return m.n[key], window / 2, nil
}

// ckLimiter is the limiter every public-route test mounts: the real policy over a fresh counter.
func ckLimiter() *ratelimit.Limiter { return ckLimiterOn(&memCounter{}) }

func ckLimiterOn(c ratelimit.Counter) *ratelimit.Limiter {
	l, err := ratelimit.New(c, ratelimit.PublicNewsRead)
	if err != nil {
		panic(err)
	}
	return l
}

func rlServer(t *testing.T, c *memCounter, log *slog.Logger) http.Handler {
	t.Helper()
	nd, dm := ckDuLieu()
	if log == nil {
		log = slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	}
	mux := http.NewServeMux()
	RegisterCongKhai(mux, DepsCongKhai{Limiter: ckLimiterOn(c), Xa: &ckNenTang{}, NoiDung: nd, DanhMuc: dm,
		CoverImages: &fakePublicCovers{}, Audio: &fakePublicAudio{}, Log: log})
	return mux
}

func rlGet(h http.Handler, path, host, remote string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, "https://comms.api.vigov.vn"+path+"?host="+url.QueryEscape(host), nil)
	r.RemoteAddr = remote
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

var rlPaths = []string{MauTinXa, MauTinXa + "/categories", MauTinXa + "/nd-a-1"}

// Every route: 120 served, the 121st is 429 with Retry-After, and the handler behind is not reached.
func TestPublicNewsRoutesAre429PastTheLimit(t *testing.T) {
	for _, p := range rlPaths {
		t.Run(p, func(t *testing.T) {
			c := &memCounter{}
			h := rlServer(t, c, nil)
			for i := 0; i < ratelimit.PublicNewsReadLimit; i++ {
				if w := rlGet(h, p, ckHostA, "203.0.113.7:5000"); w.Code != http.StatusOK {
					t.Fatalf("request %d: %d %s", i+1, w.Code, w.Body.String())
				}
			}
			w := rlGet(h, p, ckHostA, "203.0.113.7:5000")
			if w.Code != http.StatusTooManyRequests {
				t.Fatalf("request 121: %d, want 429", w.Code)
			}
			if w.Header().Get("Retry-After") == "" {
				t.Fatal("429 without Retry-After")
			}
			if e := loiTra(t, w); e.Code != "rate_limited" {
				t.Fatalf("code = %q", e.Code)
			}
			// Another client network keeps its own budget.
			if w := rlGet(h, p, ckHostA, "203.0.113.8:5000"); w.Code != http.StatusOK {
				t.Fatalf("another address refused: %d", w.Code)
			}
		})
	}
}

// Rule 1 invariant 7: a resolved commune's counter carries `t:<tenant_id>` FROM the platform's answer;
// an unknown host's counter carries none — and is keyed per host, so the two behave alike.
func TestPublicNewsLimiterKeyIsCommuneScoped(t *testing.T) {
	c := &memCounter{}
	h := rlServer(t, c, nil)
	rlGet(h, MauTinXa, ckHostA, "203.0.113.7:5000")
	rlGet(h, MauTinXa, "khong-co.vigov.vn", "203.0.113.7:5000")
	rlGet(h, MauTinXa, ckHostNgung, "203.0.113.7:5000")
	want := []string{
		"t:" + string(xaA) + ":rl:public-news:host:" + ckHostA + ":ip:203.0.113.7",
		"rl:public-news:host:khong-co.vigov.vn:ip:203.0.113.7",
		// An INACTIVE commune is "no commune" on this surface: never its tenant prefix.
		"rl:public-news:host:" + ckHostNgung + ":ip:203.0.113.7",
	}
	if strings.Join(c.keys, "|") != strings.Join(want, "|") {
		t.Fatalf("keys = %v\nwant   %v", c.keys, want)
	}
}

// A 429 does not tell which domains are communes: an unknown host is limited at the same threshold.
func TestPublicNewsLimitSameForUnknownHost(t *testing.T) {
	h := rlServer(t, &memCounter{}, nil)
	for i := 0; i < ratelimit.PublicNewsReadLimit; i++ {
		if w := rlGet(h, MauTinXa, "khong-co.vigov.vn", "203.0.113.7:5000"); w.Code != http.StatusOK {
			t.Fatalf("request %d: %d", i+1, w.Code)
		}
	}
	if w := rlGet(h, MauTinXa, "khong-co.vigov.vn", "203.0.113.7:5000"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("unknown host past the limit: %d, want 429", w.Code)
	}
}

// FAIL OPEN (owner's exception): Redis down → served, one security warning, no address in it.
func TestPublicNewsServesWhenTheCounterStoreIsDown(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	c := &memCounter{fail: errors.New("dial tcp 10.0.0.9:6379: connection refused")}
	h := rlServer(t, c, log)
	for _, p := range rlPaths {
		if w := rlGet(h, p, ckHostA, "203.0.113.7:5000"); w.Code != http.StatusOK {
			t.Fatalf("%s with Redis down: %d, want 200 (fail open)", p, w.Code)
		}
	}
	if n := strings.Count(buf.String(), "rate_limit.store_unavailable"); n != 1 {
		t.Fatalf("outage warnings = %d, want 1 per window: %s", n, buf.String())
	}
	if strings.Contains(buf.String(), "203.0.113.7") {
		t.Fatal("the outage warning names a client address")
	}
}

// A malformed host is refused before anything is counted: the 400 costs no platform call and no key.
func TestPublicNewsBadHostIsNotCounted(t *testing.T) {
	c := &memCounter{}
	h := rlServer(t, c, nil)
	if w := rlGet(h, MauTinXa, "not a host", "203.0.113.7:5000"); w.Code != http.StatusBadRequest {
		t.Fatalf("bad host: %d", w.Code)
	}
	if len(c.keys) != 0 {
		t.Fatalf("a 400 was counted: %v", c.keys)
	}
}
