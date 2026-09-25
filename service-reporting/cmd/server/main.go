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
	"github.com/vihat/vigov/core/migrate"
	svchttp "github.com/vihat/vigov/service-reporting/internal/http"
	"github.com/vihat/vigov/service-reporting/migrations"
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

	// 2b. SCHEMA MIGRATIONS — the one step below that is NOT a skeleton.
	//
	// It runs before the listener opens, and a failure stops the process. Starting on a schema
	// whose shape is unknown is worse than not starting: the fault then surfaces on somebody's
	// first request, as an error that says nothing about a migration.
	//
	// It is wired ahead of the rest on purpose. The schema is what every other step will be
	// written against, and `0002_audit_log_append_only.sql` — which turns "append-only" from a
	// comment into a database constraint — had been committed and never applied by anything.
	// chayMigration returns the config it already loaded: loading it a second time here
	// would be two reads of one environment for one fact (rule 11, invariant 1).
	cfg, err := chayMigration(log)
	if err != nil {
		log.Error("migration không chạy được", "service", "reporting", "err", err)
		os.Exit(1)
	}

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

	// Rule 11, invariant 1: the environment is read in core/config and nowhere else.
	// LISTEN_ADDR or ":8080" — one default for every service, see config.Config.ListenAddr.
	addr := cfg.ListenAddr
	log.Info("starting", "service", "reporting", "addr", addr)
	// OUTERMOST already, so that when the edge chain above is wired it sits inside this and every
	// layer reads one client address per request (TRUSTED_PROXY_CIDRS, rule 6 invariant 2).
	if err := http.ListenAndServe(addr, httpx.ClientIPTuProxyTinCay(cfg.TrustedProxies)(mux)); err != nil {
		log.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

// chayMigration opens this service's OWN database and applies its embedded migrations.
//
// The pool is opened and closed here rather than handed onward because step 2 of the list above
// is not written yet: this service has no store. When store.New arrives, this function folds
// into the main wiring and the pool is opened once — the migration call itself does not change.
func chayMigration(log *slog.Logger) (config.Config, error) {
	// Platform-wide constants only. Per-commune values are read at RUNTIME (rule 1, invariant
	// 10) — there is nothing per-commune on this path in any case: the schema is shared by every
	// commune the process serves, partitioned by tenant_id rather than split per commune.
	cfg, err := config.Load("reporting")
	if err != nil {
		return config.Config{}, err
	}
	for _, canhBao := range cfg.CanhBao() {
		log.Warn("CẢNH BÁO CẤU HÌNH", "chi_tiet", canhBao)
	}

	// .Lo() is the ONE place the DSN leaves secret.DSN with its password intact: the driver
	// argument, and nowhere else (rule 8).
	db, err := sql.Open("pgx", cfg.DatabaseDSN.Lo())
	if err != nil {
		return config.Config{}, err
	}
	defer db.Close()

	ctx, huy := context.WithTimeout(context.Background(), 5*time.Minute)
	defer huy()
	if err := db.PingContext(ctx); err != nil {
		return config.Config{}, fmt.Errorf("reporting: không nối được cơ sở dữ liệu: %w", err)
	}

	kq, err := migrate.Chay(ctx, db, migrations.FS, "reporting")
	if err != nil {
		return config.Config{}, err
	}
	// Logged even when nothing was applied: "applied 0 files" at startup is how an operator
	// finds out the replica is already at the schema they expected, without opening a psql
	// prompt.
	log.Info("migration xong", "service", "reporting", "da_ap", kq.DaAp, "bo_qua", len(kq.BoQua))
	return cfg, nil
}
