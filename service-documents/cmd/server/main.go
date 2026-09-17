package main

// documents service — Văn thư.
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
	"github.com/vihat/vigov/core/migrate"
	svchttp "github.com/vihat/vigov/service-documents/internal/http"
	"github.com/vihat/vigov/service-documents/migrations"
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
	if err := chayMigration(log); err != nil {
		log.Error("migration không chạy được", "service", "documents", "err", err)
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

	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8083"
	}
	log.Info("starting", "service", "documents", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

// chayMigration opens this service's OWN database and applies its embedded migrations.
//
// The pool is opened and closed here rather than handed onward because step 2 of the list above
// is not written yet: this service has no store. When store.New arrives, this function folds
// into the main wiring and the pool is opened once — the migration call itself does not change.
func chayMigration(log *slog.Logger) error {
	// Platform-wide constants only. Per-commune values are read at RUNTIME (rule 1, invariant
	// 10) — there is nothing per-commune on this path in any case: the schema is shared by every
	// commune the process serves, partitioned by tenant_id rather than split per commune.
	cfg, err := config.Load("documents")
	if err != nil {
		return err
	}
	for _, canhBao := range cfg.CanhBao() {
		log.Warn("CẢNH BÁO CẤU HÌNH", "chi_tiet", canhBao)
	}

	// .Lo() is the ONE place the DSN leaves secret.DSN with its password intact: the driver
	// argument, and nowhere else (rule 8).
	db, err := sql.Open("pgx", cfg.DatabaseDSN.Lo())
	if err != nil {
		return err
	}
	defer db.Close()

	ctx, huy := context.WithTimeout(context.Background(), 5*time.Minute)
	defer huy()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("documents: không nối được cơ sở dữ liệu: %w", err)
	}

	kq, err := migrate.Chay(ctx, db, migrations.FS, "documents")
	if err != nil {
		return err
	}
	// Logged even when nothing was applied: "applied 0 files" at startup is how an operator
	// finds out the replica is already at the schema they expected, without opening a psql
	// prompt.
	log.Info("migration xong", "service", "documents", "da_ap", kq.DaAp, "bo_qua", len(kq.BoQua))
	return nil
}
