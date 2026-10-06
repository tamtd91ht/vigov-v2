package main

// What this test defends: the public App ID lookup is reachable on the platform's API host — which
// resolves to no commune — because run() mounts it OUTSIDE the commune chain, and mounting it changed
// nothing else on that chain. internal/http tests the handler; only this package sees where it is put.

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/vihat/vigov/core/ratelimit"
	"github.com/vihat/vigov/service-platform/internal/domain"
	svchttp "github.com/vihat/vigov/service-platform/internal/http"
	"github.com/vihat/vigov/service-platform/internal/store"
)

type sharedOne struct{}

func (sharedOne) SharedMiniApp(context.Context) (domain.SharedMiniApp, error) {
	return domain.SharedMiniApp{AppID: "3749147383835819800"}, nil
}

type ownNone struct{}

func (ownNone) LiveOwnMiniAppID(context.Context, string) (string, error) {
	return "", store.ErrKhongCoMiniApp
}

func TestMiniAppIDsReachableOutsideTheCommuneChain(t *testing.T) {
	lim, err := ratelimit.New(counterOK{}, ratelimit.MiniAppIDLookup)
	if err != nil {
		t.Fatal(err)
	}
	outer := buildOuter(communeChain(), operatorChain(t, &resolverCounting{}), operatorHostTest)
	mountMiniAppIDs(outer, svchttp.MiniAppIDDeps{Shared: sharedOne{}, Own: ownNone{}, Limiter: lim,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))})

	// The platform API host names no commune: the commune chain would 404 it. Same answer on a commune
	// host and the operator host — the route is outside every chain.
	for _, host := range []string{"platform.api.vigov.vn", hostThu, operatorHostTest} {
		if got := get(outer, host, "/api/v1/mini-app-ids?app=vihat", nil); got != http.StatusOK {
			t.Errorf("%s: %d, want 200", host, got)
		}
	}
	// Only that GET: another path on the API host still meets the commune chain's 404, and a write is
	// not routed to the lookup.
	if got := get(outer, "platform.api.vigov.vn", "/api/v1/commune-branding", nil); got != http.StatusNotFound {
		t.Errorf("other path on the API host: %d, want 404", got)
	}
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/mini-app-ids?app=vihat", nil)
	req.Host = "platform.api.vigov.vn"
	rec := &statusRecorder{header: http.Header{}}
	outer.ServeHTTP(rec, req)
	if rec.code == http.StatusOK {
		t.Error("POST reached the lookup")
	}
	if got := get(outer, "10.0.0.1:8080", "/healthz", nil); got != http.StatusOK {
		t.Errorf("/healthz: %d", got)
	}
}

type statusRecorder struct {
	header http.Header
	code   int
}

func (r *statusRecorder) Header() http.Header         { return r.header }
func (r *statusRecorder) Write(b []byte) (int, error) { return len(b), nil }
func (r *statusRecorder) WriteHeader(c int)           { r.code = c }
