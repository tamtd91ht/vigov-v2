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
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/core/migrate"
	"github.com/vihat/vigov/core/platformclient"
	"github.com/vihat/vigov/core/staffauth"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	svchttp "github.com/vihat/vigov/service-comms/internal/http"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
	"github.com/vihat/vigov/service-comms/migrations"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Steps 1 to 4 of the wiring list are below and are no longer a skeleton: config, the pool, the
	// commune directory and the staff-authentication edge are all real.
	//
	// TODO(skeleton): still missing —
	//   5. consumers   event handlers; each refuses a message with no commune
	//
	// A consumer is NOT covered by the edge chain below: a message carries its commune inside
	// itself (rule 1, invariant 9), and a consumer that finds none refuses rather than guessing.

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

	// 3. directory — Host -> commune, over gRPC to the platform service. There is no second way to
	// resolve a commune: the registry tables belong to the platform service, and a connection to
	// another service's schema is rule 2, forbidden #2. The cache is what makes a network call per
	// request affordable at 200+ communes (ADR 0004, decision 5).
	//
	// NEITHER CLIENT IS CLOSED ON A defer, for the reason already stated on the pool above: this
	// process exits through os.Exit, which runs no defers.
	nenTang, err := platformclient.Dial(cfg.PlatformGRPCAddr, cfg.GRPCCallerKey, log)
	if err != nil {
		log.Error("không nối được dịch vụ nền tảng", "service", "comms", "err", err)
		os.Exit(1)
	}
	directory := tenant.NewCachedDirectory(nenTang, cfg.TenantCacheTTL)

	// 4. identity — session cookie -> staff principal, over gRPC. This service owns no session
	// registry and may not import identity's (rule 2, forbidden #1), so the principal comes from
	// the contract: ResolveStaffPrincipal, once per staff request, with no cache.
	dinhDanh, err := identityclient.Dial(cfg.IdentityGRPCAddr, cfg.GRPCCallerKey, log)
	if err != nil {
		log.Error("không nối được dịch vụ định danh", "service", "comms", "err", err)
		os.Exit(1)
	}

	// Deps.Checker is staffauth.Checker: it decides from the permission set the middleware obtained
	// for THIS request and holds no state of its own. No mounted route declares
	// authz.RequirePermission yet; wiring it now is what makes the first one that does work rather
	// than meet a nil interface at request time.
	mux := http.NewServeMux()
	svchttp.Register(mux, svchttp.Deps{
		Checker:       staffauth.Checker{},
		LoaiTaiNguyen: commsstore.NewLoaiTaiNguyenBanDoStore(kho),
		Log:           log,
	})

	// Rule 11, invariant 1: the environment is read in core/config and nowhere else.
	// The default is this service's own — see config.ListenAddrHoac for why it lives here.
	addr := cfg.ListenAddrHoac(":8087")
	log.Info("starting", "service", "comms", "addr", addr)
	if err := http.ListenAndServe(addr, dungBien(mux, directory, dinhDanh, log)); err != nil {
		log.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

// dungBien builds the edge chain this binary serves.
//
// IT IS A FUNCTION SO A TEST CAN DRIVE THE REAL CHAIN. core/staffauth proves what the
// authentication middleware does and core/httpx proves what TenantMiddleware does; neither can see
// whether THIS binary installs them. A middleware deleted from here leaves a service that starts,
// serves and answers — and answers 401 to every member of staff holding a valid session, which is
// the exact state this service shipped in until today. Nothing else in this repository turns red
// for that, so cmd/server/main_test.go speaks through this function.
//
// ORDER MATTERS AND IS NOT NEGOTIABLE, outermost first:
//
//	StripTenantHeaders  a client naming its own commune is a client granting itself access
//	Recover             turns tenant.MustFrom's deliberate panic into a traceable 500
//	TenantMiddleware    resolves Host -> commune; unknown Host returns 404, never a default
//	staffauth           rebuilds authz.Principal by asking identity; INSIDE TenantMiddleware,
//	                    because the outgoing call carries the commune from the context and the
//	                    principal is stamped with the commune resolved from Host
//
// /healthz IS DELIBERATELY OUTSIDE THE WHOLE CHAIN, on the outer mux: it answers whether this
// process is alive, which is true or false regardless of which commune is asking. Behind Host
// resolution it would fail whenever the platform service does, and an orchestrator would then
// restart a healthy process during somebody else's outage.
func dungBien(mux http.Handler, danhBa tenant.Directory, dinhDanh staffauth.Resolver,
	log *slog.Logger) http.Handler {

	var h http.Handler = mux
	h = staffauth.Middleware(dinhDanh, log)(h)
	h = httpx.TenantMiddleware(danhBa)(h)
	h = httpx.Recover(traceID)(h)
	h = httpx.StripTenantHeaders(h)

	ngoai := http.NewServeMux()
	ngoai.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	ngoai.Handle("/", h)
	return ngoai
}

// traceID returns the id a caller can quote when reporting a problem.
//
// TODO(next): lift this from the incoming request header once the reverse proxy sets one, so a
// single citizen complaint can be followed across services. Same shape and same TODO as identity's.
func traceID(context.Context) string { return "" }

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
