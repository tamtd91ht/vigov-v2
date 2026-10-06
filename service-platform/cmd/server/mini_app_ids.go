package main

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/vihat/vigov/core/config"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/ratelimit"
	svchttp "github.com/vihat/vigov/service-platform/internal/http"
)

// miniAppIDsPattern is where mountMiniAppIDs puts the public App ID lookup on the OUTER mux.
//
// OUTSIDE BOTH CHAINS, beside /healthz, and the reason is the Host: the caller (the Mini App deploy
// script, owner option A 06/10/2026) addresses the platform's own API host, which the commune chain's
// TenantMiddleware answers 404 — it names no commune, and must not. The route reads only the registry
// (mini_app, tenant, tenant_domain) and answers one App ID; it carries no session and no commune, so
// neither staffauth nor idem has anything to do on it. Served on every Host for that reason: on a
// commune host (through web-admin's gateway) the answer is byte-for-byte the same.
const miniAppIDsPattern = "GET /api/v1/mini-app-ids"

// mountMiniAppIDs registers the lookup on its own mux, wraps it in Recover, and routes the pattern
// of the outer mux to it. A NAMED FUNCTION so the edge test drives exactly what run() mounts.
func mountMiniAppIDs(outer *http.ServeMux, d svchttp.MiniAppIDDeps) {
	inner := http.NewServeMux()
	svchttp.RegisterMiniAppIDs(inner, d)
	outer.Handle(miniAppIDsPattern, httpx.Recover(traceID)(inner))
}

// miniAppIDLimiter builds the lookup's per-IP limiter (ratelimit.MiniAppIDLookup, fails CLOSED) over
// Redis — or, in dev without REDIS_DSN, over failingCounter, which answers every lookup 503 rather than
// serving an unauthenticated route unbounded. Staging/prod refuse to start without REDIS_DSN
// (config.Redis is declared).
func miniAppIDLimiter(cfg config.Config, log *slog.Logger) (*ratelimit.Limiter, func(), error) {
	var counter ratelimit.Counter = failingCounter{}
	closeCounter := func() {}
	if dsn := cfg.RedisDSN().Lo(); dsn != "" {
		rc, err := ratelimit.NewRedisCounter(dsn)
		if err != nil {
			return nil, nil, fmt.Errorf("platform: tra App ID Mini App: %w", err)
		}
		counter, closeCounter = rc, func() { _ = rc.Close() }
	} else {
		log.Warn("CẢNH BÁO CẤU HÌNH: REDIS_DSN trống — GET /api/v1/mini-app-ids trả 503 (giới hạn tần suất đóng kín)")
	}
	lim, err := ratelimit.New(counter, ratelimit.MiniAppIDLookup)
	if err != nil {
		closeCounter()
		return nil, nil, fmt.Errorf("platform: %w", err)
	}
	return lim, closeCounter, nil
}
