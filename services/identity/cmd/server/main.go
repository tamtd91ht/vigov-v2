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

	// TODO(next): NOT RUNNABLE YET, and deliberately left that way.
	//
	// The session routes and the XacThuc middleware are written and tested
	// (services/identity/internal/http), but mounting them for real needs a tenant.Directory,
	// and the only legitimate source of one is a gRPC call to the platform service
	// (proto/vigov/platform/v1/platform.proto, ResolveHost) — which does not serve gRPC yet.
	//
	// Wiring it against anything else would mean inventing a second way to resolve a commune,
	// and a second way to resolve a commune is how one commune ends up serving another's data.
	// What is still missing, in order:
	//   a. platform serves ResolveHost over gRPC
	//   b. a cached tenant.Directory client here (short TTL — this is on every request)
	//   c. cfg.KhoaKyBytes() -> token.NewSigner; refuses to start with no key outside dev
	//   d. THE SAME signer goes to app.NewDangNhap as well as to svchttp.Deps: the use case signs
	//      the session token INSIDE its transaction (a signing failure must roll the session and
	//      its audit entry back), while Deps.Signer is what XacThuc verifies incoming tokens with.
	//      Two different signers would issue tokens the very next request cannot verify.
	//      svchttp.Deps{Checker, Signer, Phien, CanBo, DangNhap, DangXuat, Log}
	//   e. h = svchttp.XacThuc(deps)(mux), INSIDE httpx.TenantMiddleware — never outside it
	//
	// svchttp.Register is NOT called here on purpose: it refuses incomplete Deps, and mounting
	// a sign-in route with no signing key and no session store would answer requests it cannot
	// honour. Until (a)–(e) are done this binary serves /healthz and nothing else.
	_ = svchttp.Deps{}

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
