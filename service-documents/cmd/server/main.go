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
	"github.com/vihat/vigov/core/store"
	svchttp "github.com/vihat/vigov/service-documents/internal/http"
	docstore "github.com/vihat/vigov/service-documents/internal/store"
	"github.com/vihat/vigov/service-documents/migrations"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// THE WORK IS IN run() SO THE deferred Close() CALLS ACTUALLY RUN. os.Exit skips defers, so a
	// pool opened beside an os.Exit is a pool that is never closed on the failure paths — which is
	// precisely when the connections matter.
	if err := run(log); err != nil {
		log.Error("service dừng", "service", "documents", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	// 1. config — platform-wide constants from the environment ONLY. Per-commune values are read
	//    at RUNTIME from the platform service (rule 1, invariant 10). There is nothing per-commune
	//    on this path in any case: the schema is one set of tables for every commune this process
	//    serves, partitioned by tenant_id rather than split per commune.
	cfg, err := config.Load("documents")
	if err != nil {
		return err
	}
	for _, canhBao := range cfg.CanhBao() {
		// Reported at EVERY startup, never once at deploy time: a flag switched on for a local
		// afternoon is forgotten by the next morning (rule 8, invariant 7).
		log.Warn("CẢNH BÁO CẤU HÌNH", "chi_tiet", canhBao)
	}

	// 2. store — one schema per service; never another service's schema (rule 2).
	//
	// .Lo() is the ONE place the DSN leaves secret.DSN with its password intact: the driver
	// argument, and nowhere else (rule 8).
	db, err := sql.Open("pgx", cfg.DatabaseDSN.Lo())
	if err != nil {
		return err
	}
	defer db.Close()

	// The same pool the other services use. Sized for 200+ communes sharing one process: too large
	// and the database runs out of connections long before the service runs out of work.
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	ctxPing, huyPing := context.WithTimeout(context.Background(), 15*time.Second)
	defer huyPing()
	if err := db.PingContext(ctxPing); err != nil {
		return fmt.Errorf("documents: không nối được cơ sở dữ liệu: %w", err)
	}

	// 2b. SCHEMA MIGRATIONS, BEFORE the listener opens, and a failure stops the process.
	//
	// Starting on a schema whose shape is unknown is worse than not starting: the fault then
	// surfaces on somebody's first request, as an error that says nothing about a migration.
	//
	// The files are EMBEDDED in this binary, so what is applied is what was compiled — not
	// whatever happens to be on the container's disk.
	ctxMig, huyMig := context.WithTimeout(context.Background(), 5*time.Minute)
	kqMig, err := migrate.Chay(ctxMig, db, migrations.FS, "documents")
	huyMig()
	if err != nil {
		return fmt.Errorf("documents: migration không chạy được: %w", err)
	}
	// Logged even when nothing was applied: "applied 0 files" at startup is how an operator finds
	// out the replica is already at the schema they expected, without opening a psql prompt.
	log.Info("migration xong", "service", "documents", "da_ap", kqMig.DaAp, "bo_qua", len(kqMig.BoQua))

	// 3. stores. Built on *store.DB, which only hands out commune-scoped access — there is no path
	//    here to an unscoped query (rule 1, invariant 5).
	kho := store.New(db)
	loaiVanBan := docstore.NewLoaiVanBanStore(kho)

	// TODO(skeleton): still to wire, in this order —
	//   directory   tenant.Directory backed by the platform service, cached with a short TTL —
	//               this sits on the path of every request at 200+ communes
	//   principal   the middleware that rebuilds authz.Principal for a signed-in member of staff.
	//               identity does it in its own XacThuc, against its session registry; this
	//               service owns no session registry and may not import identity's (rule 2,
	//               forbidden #1), so HOW it verifies a session is an open wiring question
	//   checker     authz.Checker backed by the identity service, for the first route here that
	//               declares authz.RequirePermission
	//   consumers   event handlers; each refuses a message with no commune
	//
	// WHAT THAT COSTS TODAY, STATED RATHER THAN LEFT TO BE DISCOVERED: with neither of the first
	// two mounted below, GET /api/v1/document-types answers 401 to every caller —
	// authz.AnyAuthenticated finds no principal in the context and refuses. That is the fail-closed
	// direction (a route that is unreachable, never one that is open), and it is why the route is
	// tested against the full chain in internal/http rather than only through this binary.

	mux := http.NewServeMux()
	// /healthz MUST END UP OUTSIDE THE TENANT CHAIN once that chain is mounted — on an outer mux,
	// the way identity does it. It answers whether this process is alive, which is true or false
	// regardless of which commune is asking; behind Host resolution it would fail whenever the
	// platform service does, and an orchestrator would then restart a healthy process during
	// somebody else's outage.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// 4. routes. Register refuses incomplete Deps at construction, not at request time.
	//
	// Checker is deliberately absent: no route in this service declares authz.RequirePermission
	// yet, and passing a value nothing reads would suggest a permission path that does not exist.
	svchttp.Register(mux, svchttp.Deps{
		LoaiVanBan: loaiVanBan,
		Log:        log,
	})

	// The edge chain. Order matters and is not negotiable:
	//   StripTenantHeaders  a client naming its own commune is a client granting itself access
	//   Recover             turns tenant.MustFrom's deliberate panic into a traceable 500
	//   TenantMiddleware    resolves Host -> commune; unknown Host returns 404, never a default
	//   <principal>         rebuilds authz.Principal — see the TODO above; INSIDE TenantMiddleware,
	//                       because it compares the commune in the session against the commune
	//                       resolved from Host
	//
	//	var h http.Handler = mux
	//	h = httpx.TenantMiddleware(directory)(h)
	//	h = httpx.Recover(traceID)(h)
	//	h = httpx.StripTenantHeaders(h)

	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8083"
	}
	log.Info("starting", "service", "documents", "addr", addr,
		// secret.DSN redacts the password on every rendering path and keeps the host, so this line
		// still says which database was opened (rule 8).
		"dsn", cfg.DatabaseDSN)
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return srv.ListenAndServe()
}
