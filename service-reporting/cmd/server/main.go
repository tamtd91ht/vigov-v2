package main

// reporting service — Read model.
//
// This file only wires. No business logic, no init(), no global mutable state.
// See .claude/skills/go-service-pattern.

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/vihat/vigov/core/config"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/core/migrate"
	"github.com/vihat/vigov/core/platformclient"
	"github.com/vihat/vigov/core/staffauth"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-reporting/internal/app"
	svchttp "github.com/vihat/vigov/service-reporting/internal/http"
	rpstore "github.com/vihat/vigov/service-reporting/internal/store"
	"github.com/vihat/vigov/service-reporting/migrations"
)

// configUses is every configuration group this binary reads — and so, in staging and prod, every
// group whose variables must be set for it to start (core/config/uses.go). Undeclared groups are
// not read at all. TestConfigUsesMatchReads keeps this list equal to what the package reads.
//
// reporting: REST only — no gRPC server; resolves communes and staff through platform and identity
// (the "Lời hệ thống" routes, ADR 0024 §Phụ). No Redis: every route here declares idem.KhongCan, so
// there is nothing for a duplicate-request store to protect — declaring it would make staging and
// prod refuse to start without a variable this service never reads.
var configUses = config.Uses(
	config.HTTPServer,
	config.PlatformClient,
	config.IdentityClient,
	config.TenantCache,
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// TODO(skeleton): still to wire —
	//   5. consumers   event handlers; each refuses a message with no commune

	// 1 + 2. THE POOL IS OPENED ONCE, HERE, and lives as long as the process. It used to be opened
	// and closed inside the migration step because this service had no store to hand it to; the
	// "Lời hệ thống" store is that first reader, and a second pool would be a second place the DSN
	// is read.
	cfg, db, err := openDatabase(log)
	if err != nil {
		log.Error("không mở được cơ sở dữ liệu", "service", "reporting", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	// 2b. SCHEMA MIGRATIONS.
	//
	// It runs before the listener opens, and a failure stops the process. Starting on a schema
	// whose shape is unknown is worse than not starting: the fault then surfaces on somebody's
	// first request, as an error that says nothing about a migration.
	//
	// It is wired ahead of the rest on purpose. The schema is what every other step will be
	// written against, and `0002_audit_log_append_only.sql` — which turns "append-only" from a
	// comment into a database constraint — had been committed and never applied by anything.
	if err := runMigrations(db, log); err != nil {
		log.Error("migration không chạy được", "service", "reporting", "err", err)
		os.Exit(1)
	}

	// pkgstore.New is the ONLY handle the stores get. There is deliberately no path from here that
	// hands a raw *sql.DB to business code (rule 1, invariant 5).
	scopedDB := pkgstore.New(db)

	// 3. directory — Host -> commune, over gRPC to the platform service, cached (ADR 0004,
	// decision 5). There is no second way to resolve a commune (rule 2, forbidden #2).
	platform, err := platformclient.Dial(cfg.PlatformGRPCAddr(), cfg.GRPCCallerKey(), log)
	if err != nil {
		log.Error("không nối được dịch vụ nền tảng", "service", "reporting", "err", err)
		os.Exit(1)
	}
	defer platform.Close()
	directory := tenant.NewCachedDirectory(platform, cfg.TenantCacheTTL())

	// 4. identity — session cookie -> staff principal, over gRPC, once per staff request, no cache.
	// Deps.Checker is staffauth.Checker: it decides from the permission set the middleware obtained
	// for THIS request (`admin.lookup` on the three "Lời hệ thống" routes).
	identity, err := identityclient.Dial(cfg.IdentityGRPCAddr(), cfg.GRPCCallerKey(), log)
	if err != nil {
		log.Error("không nối được dịch vụ định danh", "service", "reporting", "err", err)
		os.Exit(1)
	}
	defer identity.Close()

	mux := http.NewServeMux()
	svchttp.Register(mux, svchttp.Deps{
		Checker: staffauth.Checker{},
		// "Lời hệ thống": a commune's wording of the `report.*` sentences. The use case owns the
		// transaction the override row and its audit entry share (rule 6, invariant 3).
		SystemMessages: app.NewSystemMessages(scopedDB, rpstore.NewSystemMessageOverrideStore(scopedDB)),
		Log:            log,
	})

	// Rule 11, invariant 1: the environment is read in core/config and nowhere else.
	// LISTEN_ADDR or ":8080" — one default for every service, see config.Config.ListenAddr.
	addr := cfg.ListenAddr()
	log.Info("starting", "service", "reporting", "addr", addr)
	// OUTERMOST, around the whole edge chain: every layer reads one client address per request
	// (TRUSTED_PROXY_CIDRS, rule 6 invariant 2).
	if err := http.ListenAndServe(addr, httpx.ClientIPTuProxyTinCay(cfg.TrustedProxies())(edgeChain(mux, directory, identity, log))); err != nil {
		log.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

// edgeChain builds the edge chain this binary serves — the same chain finance's cmd/server builds,
// minus idem.Middleware (no route here declares Required; see configUses).
//
// IT IS A FUNCTION SO A TEST CAN DRIVE THE REAL CHAIN (main_test.go): a middleware deleted from
// here leaves a service that starts and answers 401 to every member of staff holding a valid
// session, and nothing else in the repository turns red for that.
//
// ORDER MATTERS AND IS NOT NEGOTIABLE, outermost first:
//
//	StripTenantHeaders  a client naming its own commune is a client granting itself access
//	Recover             turns tenant.MustFrom's deliberate panic into a traceable 500
//	TenantMiddleware    resolves Host -> commune; unknown Host returns 404, never a default
//	staffauth           rebuilds authz.Principal by asking identity; INSIDE TenantMiddleware,
//	                    because the outgoing call carries the commune from the context
//
// /healthz IS DELIBERATELY OUTSIDE THE WHOLE CHAIN: it answers whether this process is alive,
// regardless of which commune is asking. Behind Host resolution it would fail whenever the
// platform service does, and an orchestrator would restart a healthy process.
func edgeChain(mux http.Handler, directory tenant.Directory, identity staffauth.Resolver, log *slog.Logger) http.Handler {
	var h http.Handler = mux
	h = staffauth.Middleware(identity, log)(h)
	h = httpx.TenantMiddleware(directory)(h)
	h = httpx.Recover(traceID)(h)
	h = httpx.StripTenantHeaders(h)

	outer := http.NewServeMux()
	outer.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	outer.Handle("/", h)
	return outer
}

// traceID returns the id a caller can quote when reporting a problem.
//
// TODO(next): lift this from the incoming request header once the reverse proxy sets one. Same
// shape and same TODO as finance's and identity's.
func traceID(context.Context) string { return "" }

// openDatabase reads the configuration and opens this service's OWN database — never another
// service's schema (rule 2, forbidden #2). It returns the config too: reading the environment a
// second time would be a second answer to one question (rule 11, invariant 1).
func openDatabase(log *slog.Logger) (config.Config, *sql.DB, error) {
	// Platform-wide constants only. Per-commune values are read at RUNTIME (rule 1, invariant
	// 10) — there is nothing per-commune on this path in any case: the schema is shared by every
	// commune the process serves, partitioned by tenant_id rather than split per commune.
	cfg, err := config.Load("reporting", configUses)
	if err != nil {
		return config.Config{}, nil, err
	}
	for _, warning := range cfg.CanhBao() {
		log.Warn("CẢNH BÁO CẤU HÌNH", "chi_tiet", warning)
	}

	// .Lo() is the ONE place the DSN leaves secret.DSN with its password intact: the driver
	// argument, and nowhere else (rule 8).
	db, err := sql.Open("pgx", cfg.DatabaseDSN.Lo())
	if err != nil {
		return config.Config{}, nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return config.Config{}, nil, fmt.Errorf("reporting: không nối được cơ sở dữ liệu: %w", err)
	}
	return cfg, db, nil
}

// runMigrations applies this service's embedded migrations to the pool opened above.
func runMigrations(db *sql.DB, log *slog.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	result, err := migrate.Chay(ctx, db, migrations.FS, "reporting")
	if err != nil {
		return err
	}
	// Logged even when nothing was applied: "applied 0 files" at startup is how an operator
	// finds out the replica is already at the schema they expected, without opening a psql
	// prompt.
	log.Info("migration xong", "service", "reporting", "da_ap", result.DaAp, "bo_qua", len(result.BoQua))
	return nil
}
