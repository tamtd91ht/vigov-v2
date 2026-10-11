package http

// THE PUBLIC MAP FRAME (ADR 0072 §"Sửa đổi 11/10/2026 (Mini App)" M2) — the handler's own half: the
// commune the host resolves to is the only one asked, the reply carries no staff-only field, "no frame"
// and "no such commune" are one answer, the 503 for an unreadable default, and the SHARED news limiter.
// The effective-frame rule is app.MapFrames.Get's (internal/app/map_frame_test.go).

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/ratelimit"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

func publicMapFrameServer(t *testing.T, f *fakeMapFrames, c *memCounter) http.Handler {
	t.Helper()
	if c == nil {
		c = &memCounter{}
	}
	mux := http.NewServeMux()
	RegisterPublicMapFrame(mux, PublicMapFrameDeps{Xa: &ckNenTang{}, Frames: f, Limiter: ckLimiterOn(c)})
	return mux
}

func frameFor(t *testing.T, lat, lng, km float64) domain.MapFrame {
	t.Helper()
	fr, err := domain.NormalizeMapFrame(lat, lng, km)
	if err != nil {
		t.Fatal(err)
	}
	fr.Enabled = true
	return fr
}

// Commune A's host reads A's frame — bounds, centre, radius — and NOTHING the staff form needs.
func TestPublicMapFrameReadsTheHostsCommune(t *testing.T) {
	f := &fakeMapFrames{byCommune: map[tenant.ID]domain.MapFrame{
		xaA: frameFor(t, 16.05, 108.2, 5), xaB: frameFor(t, 21.0, 105.8, 3)}}
	w := ckGoi(publicMapFrameServer(t, f, nil), PublicMapFramePath, ckHostA)
	if w.Code != http.StatusOK {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if f.lastCommune != xaA {
		t.Fatalf("asked commune %q, want A", f.lastCommune)
	}
	b := f.byCommune[xaA].Bounds()
	want := fmt.Sprintf(`{"configured":true,"center_lat":16.05,"center_lng":108.2,"radius_km":5,"bounds":[%v,%v,%v,%v]}`,
		b.MinLng, b.MinLat, b.MaxLng, b.MaxLat)
	if got := strings.TrimSpace(w.Body.String()); got != want {
		t.Fatalf("body = %s\nwant   %s", got, want)
	}
	for _, staffOnly := range []string{"source", "default", "updated_at", "updated_by", "notice"} {
		if strings.Contains(w.Body.String(), `"`+staffOnly) {
			t.Errorf("the public frame carries %q", staffOnly)
		}
	}
}

// No frame of its own and no platform default (K3 "Không có cả hai"), a domain nobody holds, an
// inactive commune and a reserved host: ONE answer, byte for byte.
func TestPublicMapFrameNotConfiguredIsOneAnswer(t *testing.T) {
	f := &fakeMapFrames{byCommune: map[tenant.ID]domain.MapFrame{}}
	h := publicMapFrameServer(t, f, nil)
	none := ckGoi(h, PublicMapFramePath, ckHostA)
	if none.Code != http.StatusOK || strings.TrimSpace(none.Body.String()) != `{"configured":false}` {
		t.Fatalf("commune without frame: %d %s", none.Code, none.Body.String())
	}
	for _, host := range []string{ckHostKhongCo, ckHostNgung, ckHostRieng} {
		f.calls = 0
		w := ckGoi(h, PublicMapFramePath, host)
		if w.Code != none.Code || w.Body.String() != none.Body.String() {
			t.Errorf("%s: %d %q — differs from 'no frame'", host, w.Code, w.Body.String())
		}
		if f.calls != 0 {
			t.Errorf("%s: the frame store was asked for a host no active commune holds", host)
		}
	}
}

func TestPublicMapFrameDefaultUnavailableIs503(t *testing.T) {
	f := &fakeMapFrames{err: fmt.Errorf("map_frame: đọc: %w", app.ErrMapFrameDefaultUnavailable)}
	w := ckGoi(publicMapFrameServer(t, f, nil), PublicMapFramePath, ckHostA)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("%d, want 503", w.Code)
	}
	if e := loiTra(t, w); e.Code != "map_frame_default_unavailable" {
		t.Fatalf("code %q", e.Code)
	}
}

func TestPublicMapFrameStoreFailureIs500(t *testing.T) {
	f := &fakeMapFrames{err: errors.New("pq: connection reset")}
	w := ckGoi(publicMapFrameServer(t, f, nil), PublicMapFramePath, ckHostA)
	if w.Code != http.StatusInternalServerError || strings.Contains(w.Body.String(), "pq:") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}

func TestPublicMapFrameRefusesBadHost(t *testing.T) {
	c := &memCounter{}
	h := publicMapFrameServer(t, &fakeMapFrames{}, c)
	for _, q := range []string{"", "not a host"} {
		if w := ckGoi(h, PublicMapFramePath, q); w.Code != http.StatusBadRequest {
			t.Errorf("host %q: %d, want 400", q, w.Code)
		}
	}
	if w := ckGoi(h, PublicMapFramePath, ckHostA, "&host="+ckHostB); w.Code != http.StatusBadRequest {
		t.Errorf("repeated host: %d, want 400", w.Code)
	}
	if len(c.keys) != 0 {
		t.Fatalf("a 400 was counted: %v", c.keys)
	}
}

// The SHARED news budget (owner, M2): policy `public-news`, keyed `t:<tenant_id>`.
func TestPublicMapFrameUsesTheNewsLimiter(t *testing.T) {
	c := &memCounter{}
	h := publicMapFrameServer(t, &fakeMapFrames{byCommune: map[tenant.ID]domain.MapFrame{}}, c)
	rlGet(h, PublicMapFramePath, ckHostA, "203.0.113.7:5000")
	if want := "t:" + string(xaA) + ":rl:public-news:host:" + ckHostA + ":ip:203.0.113.7"; len(c.keys) != 1 || c.keys[0] != want {
		t.Fatalf("keys = %v, want [%s]", c.keys, want)
	}
	for i := 1; i < ratelimit.PublicNewsReadLimit; i++ {
		rlGet(h, PublicMapFramePath, ckHostA, "203.0.113.7:5000")
	}
	if w := rlGet(h, PublicMapFramePath, ckHostA, "203.0.113.7:5000"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("past the limit: %d, want 429", w.Code)
	}
}

func TestRegisterPublicMapFrameRefusesMissingDeps(t *testing.T) {
	lim := ckLimiter()
	for name, d := range map[string]PublicMapFrameDeps{
		"no lookup":  {Frames: &fakeMapFrames{}, Limiter: lim},
		"no frames":  {Xa: &ckNenTang{}, Limiter: lim},
		"no limiter": {Xa: &ckNenTang{}, Frames: &fakeMapFrames{}},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: RegisterPublicMapFrame did not refuse", name)
				}
			}()
			RegisterPublicMapFrame(http.NewServeMux(), d)
		}()
	}
}
