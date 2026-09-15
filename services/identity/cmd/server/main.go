package main

// identity service — Tổ chức – cán bộ.
//
// This file only wires. No business logic, no init(), no global mutable state.
// See .claude/skills/go-service-pattern.

import (
	"log/slog"
	"net/http"
	"os"

	svchttp "github.com/vihat/vigov/services/identity/internal/http"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// TODO(skeleton): wire, in this order —
	//   1. config      platform-wide constants from the environment ONLY. Per-commune values
	//                  are read at RUNTIME from the platform service (rule 1, invariant 10)
	//   2. store.New   one schema per service; never another service's schema
	//   3. directory   tenant.Directory backed by the platform service, cached with a short
	//                  TTL — this sits on the path of every request at 200+ communes
	//   4. checker     authz.Checker backed by the identity service
	//   5. consumers   event handlers; each refuses a message with no commune

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	svchttp.Register(mux, svchttp.Deps{})

	// The edge chain. Order matters and is not negotiable:
	//   StripTenantHeaders  a client naming its own commune is a client granting itself access
	//   Recover             turns tenant.MustFrom's deliberate panic into a traceable 500
	//   TenantMiddleware    resolves Host -> commune; unknown Host returns 404, never a default
	//
	//	var h http.Handler = mux
	//	h = httpx.TenantMiddleware(directory)(h)
	//	h = httpx.Recover(traceID)(h)
	//	h = httpx.StripTenantHeaders(h)

	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8082"
	}
	log.Info("starting", "service", "identity", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
