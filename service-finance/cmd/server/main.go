package main

// finance service — Tài chính – kế toán.
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
	svchttp "github.com/vihat/vigov/service-finance/internal/http"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
	"github.com/vihat/vigov/service-finance/migrations"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// TODO(skeleton): wire, in this order —
	//   1. config      platform-wide constants from the environment ONLY. Per-commune values
	//                  are read at RUNTIME from the platform service (rule 1, invariant 10)
	//   2. store.New   DONE — see below. One schema per service; never another service's schema
	//   3. directory   tenant.Directory backed by the platform service, cached with a short
	//                  TTL — this sits on the path of every request at 200+ communes
	//   4. checker     authz.Checker backed by the identity service
	//   5. consumers   event handlers; each refuses a message with no commune

	// 1 + 2. THE POOL IS OPENED ONCE, HERE, and lives as long as the process.
	//
	// It used to be opened and closed inside the migration step, because this service had no store
	// to hand it to. It has one now, and a second pool opened later for the store would be a second
	// place the DSN is read and a second set of connections to size.
	db, err := moCSDL(log)
	if err != nil {
		log.Error("không mở được cơ sở dữ liệu", "service", "finance", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	// 2b. SCHEMA MIGRATIONS — the one step below that is NOT a skeleton.
	//
	// It runs before the listener opens, and a failure stops the process. Starting on a schema
	// whose shape is unknown is worse than not starting: the fault then surfaces on somebody's
	// first request, as an error that says nothing about a migration.
	//
	// It is wired ahead of the rest on purpose. The schema is what every other step will be
	// written against, and `0002_audit_log_append_only.sql` — which turns "append-only" from a
	// comment into a database constraint — had been committed and never applied by anything.
	if err := chayMigration(db, log); err != nil {
		log.Error("migration không chạy được", "service", "finance", "err", err)
		os.Exit(1)
	}

	// pkgstore.New is the ONLY handle the stores get. There is deliberately no path from here that
	// hands a raw *sql.DB to business code: a repository that can be built without a commune is a
	// repository that can query across communes (rule 1, invariant 5).
	kho := pkgstore.New(db)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Deps.Checker is left nil ON PURPOSE and Register accepts that: no route in this service
	// declares RequirePermission yet, so there is nothing for a checker to decide. Step 4 above
	// fills it, and Register's own switch grows a case the day the first guarded route is added.
	svchttp.Register(mux, svchttp.Deps{
		HangMuc: fistore.NewHangMucKeHoachVonStore(kho),
		Log:     log,
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
	//
	// WHAT THIS MEANS FOR THE ROUTE MOUNTED ABOVE, said plainly rather than left to be discovered:
	// it cannot serve one real request yet, and it FAILS CLOSED while it cannot. Two pieces are
	// missing and neither belongs to this turn — the commune directory (step 3) and a staff
	// authentication middleware for this service (step 4; identity's XacThuc is in that service's
	// internal/ and finance may not import it, rule 2 forbidden #1). Until they exist no
	// authz.Principal reaches the context, so authz.AnyAuthenticated answers 401 to everything.
	// That is the correct failure: no commune resolved means no query may run (rule 1, invariant 3).
	// The route is mounted now because that is what makes the contract and its tests exist.

	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8086"
	}
	log.Info("starting", "service", "finance", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

// moCSDL reads the configuration and opens this service's OWN database.
//
// "Its own" is not a manner of speaking: a connection string to a schema this service does not own
// is a read path around the contract (rule 2, forbidden #2).
func moCSDL(log *slog.Logger) (*sql.DB, error) {
	// Platform-wide constants only. Per-commune values are read at RUNTIME (rule 1, invariant
	// 10) — there is nothing per-commune on this path in any case: the schema is shared by every
	// commune the process serves, partitioned by tenant_id rather than split per commune.
	cfg, err := config.Load("finance")
	if err != nil {
		return nil, err
	}
	for _, canhBao := range cfg.CanhBao() {
		log.Warn("CẢNH BÁO CẤU HÌNH", "chi_tiet", canhBao)
	}

	// .Lo() is the ONE place the DSN leaves secret.DSN with its password intact: the driver
	// argument, and nowhere else (rule 8).
	db, err := sql.Open("pgx", cfg.DatabaseDSN.Lo())
	if err != nil {
		return nil, err
	}

	ctx, huy := context.WithTimeout(context.Background(), 30*time.Second)
	defer huy()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("finance: không nối được cơ sở dữ liệu: %w", err)
	}
	return db, nil
}

// chayMigration applies this service's embedded migrations to the pool opened above.
func chayMigration(db *sql.DB, log *slog.Logger) error {
	ctx, huy := context.WithTimeout(context.Background(), 5*time.Minute)
	defer huy()

	kq, err := migrate.Chay(ctx, db, migrations.FS, "finance")
	if err != nil {
		return err
	}
	// Logged even when nothing was applied: "applied 0 files" at startup is how an operator
	// finds out the replica is already at the schema they expected, without opening a psql
	// prompt.
	log.Info("migration xong", "service", "finance", "da_ap", kq.DaAp, "bo_qua", len(kq.BoQua))
	return nil
}
