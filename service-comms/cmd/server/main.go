package main

// comms service — Thông tin – truyền thông.
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
	pkgstore "github.com/vihat/vigov/core/store"
	svchttp "github.com/vihat/vigov/service-comms/internal/http"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
	"github.com/vihat/vigov/service-comms/migrations"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Steps 1 and 2 of the wiring list are below and are no longer a skeleton: config is loaded
	// from the environment, and the pool is opened once and handed to both the migration runner
	// and the repositories — the fold the note on chayMigration predicted.
	//
	// TODO(skeleton): still missing, in this order —
	//   3. directory   tenant.Directory backed by the platform service, cached with a short
	//                  TTL — this sits on the path of every request at 200+ communes
	//   4. checker     authz.Checker backed by the identity service
	//   5. consumers   event handlers; each refuses a message with no commune
	//
	// UNTIL 3 AND 4 LAND, THE ONE MOUNTED ROUTE ANSWERS 401 TO EVERYBODY — see the note at
	// svchttp.Register below. It fails closed, which is the right direction, but it is not
	// serving.

	// 1. config — platform-wide constants only. Per-commune values are read at RUNTIME (rule 1,
	// invariant 10); there is nothing per-commune on this path in any case, because the schema is
	// shared by every commune the process serves and partitioned by tenant_id.
	cfg, err := config.Load("comms")
	if err != nil {
		log.Error("cấu hình không nạp được", "service", "comms", "err", err)
		os.Exit(1)
	}
	for _, canhBao := range cfg.CanhBao() {
		log.Warn("CẢNH BÁO CẤU HÌNH", "chi_tiet", canhBao)
	}

	// 2. THE POOL, OPENED ONCE. It is not closed on a defer: the process exits through os.Exit
	// below, which runs no defers, and a pool that outlives the listener by nothing is not a leak.
	//
	// .Lo() is the ONE place the DSN leaves secret.DSN with its password intact: the driver
	// argument, and nowhere else (rule 8).
	db, err := sql.Open("pgx", cfg.DatabaseDSN.Lo())
	if err != nil {
		log.Error("không mở được pool cơ sở dữ liệu", "service", "comms", "err", err)
		os.Exit(1)
	}

	// 2b. SCHEMA MIGRATIONS — the one step below that is NOT a skeleton.
	//
	// It runs before the listener opens, and a failure stops the process. Starting on a schema
	// whose shape is unknown is worse than not starting: the fault then surfaces on somebody's
	// first request, as an error that says nothing about a migration.
	//
	// It is wired ahead of the rest on purpose. The schema is what every other step will be
	// written against, and `0002_audit_log_append_only.sql` — which turns "append-only" from a
	// comment into a database constraint — had been committed and never applied by anything.
	if err := chayMigration(log, db); err != nil {
		log.Error("migration không chạy được", "service", "comms", "err", err)
		os.Exit(1)
	}

	// THE ONLY HANDLE THE BUSINESS CODE EVER SEES. *sql.DB stops here: core/store.New wraps it and
	// exposes nothing but Scoped, which binds tenant_id from the context. A repository built from a
	// raw pool is a repository that can read every commune at once (rule 1, invariant 5).
	kho := pkgstore.New(db)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// WHAT THE MOUNTED ROUTE CAN AND CANNOT DO TODAY, said here rather than discovered in staging.
	//
	// The edge chain below is still commented out, because step 3 (a tenant.Directory backed by the
	// platform service) and step 4 (an authz.Checker backed by identity) do not exist in this
	// service yet. Nothing here builds an authz.Principal, so GET /api/v1/map-asset-types answers
	// 401 to every caller: authz.AnyAuthenticated refuses before the handler runs, before
	// tenant.MustFrom is ever reached. That is the correct direction to fail in — closed — but it
	// means this route serves no traffic until those two steps land, and no amount of testing in
	// this service can substitute for them.
	//
	// Deps.Checker is left nil on purpose: no mounted route consults it, and inventing a stand-in
	// that answers "yes" would be a permission check that is not one.
	svchttp.Register(mux, svchttp.Deps{
		LoaiTaiNguyen: commsstore.NewLoaiTaiNguyenBanDoStore(kho),
		Log:           log,
	})

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
		addr = ":8087"
	}
	log.Info("starting", "service", "comms", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

// chayMigration applies this service's embedded migrations to this service's OWN database.
//
// IT TAKES THE POOL RATHER THAN OPENING ONE, which is the fold the previous version of this
// comment predicted: the repositories need the same pool, and two pools against one database
// would double the connection count for no reason. The migration call itself did not change.
func chayMigration(log *slog.Logger, db *sql.DB) error {
	ctx, huy := context.WithTimeout(context.Background(), 5*time.Minute)
	defer huy()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("comms: không nối được cơ sở dữ liệu: %w", err)
	}

	kq, err := migrate.Chay(ctx, db, migrations.FS, "comms")
	if err != nil {
		return err
	}
	// Logged even when nothing was applied: "applied 0 files" at startup is how an operator
	// finds out the replica is already at the schema they expected, without opening a psql
	// prompt.
	log.Info("migration xong", "service", "comms", "da_ap", kq.DaAp, "bo_qua", len(kq.BoQua))
	return nil
}
