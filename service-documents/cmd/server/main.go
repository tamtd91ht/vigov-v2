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
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/core/migrate"
	"github.com/vihat/vigov/core/platformclient"
	"github.com/vihat/vigov/core/staffauth"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-documents/internal/app"
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

	// 3b. use cases — the business write and its audit entry share ONE transaction inside these
	//     (rule 6, invariant 3). The handlers only translate HTTP. It is given *store.DB rather
	//     than a transaction because opening one is precisely what it is for.
	ghiLoaiVanBan := app.NewDanhMucLoaiVanBan(kho, loaiVanBan)

	// 4. directory — Host -> commune, over gRPC to the platform service.
	//
	// THERE IS NO SECOND WAY TO RESOLVE A COMMUNE. This service does not read the registry tables:
	// they belong to the platform service, and a connection to another service's schema is rule 2,
	// forbidden #2. The cache is what makes a network call per request affordable at 200+ communes
	// (ADR 0004, decision 5). The caller key goes with the address: the port answers nothing
	// without it (ADR 0025).
	nenTang, err := platformclient.Dial(cfg.PlatformGRPCAddr, cfg.GRPCCallerKey, log)
	if err != nil {
		return err
	}
	defer nenTang.Close()
	directory := tenant.NewCachedDirectory(nenTang, cfg.TenantCacheTTL)

	// 5. identity — session cookie -> staff principal, over gRPC.
	//
	// WHY NOT A LOCAL COPY OF identity's XacThuc: the session registry and the grants live inside
	// service-identity/internal/, which rule 2, forbidden #1 forbids this service from importing,
	// and a second implementation of "is this session still valid" would disagree with the first on
	// the day a session is revoked. One RPC, one answer, asked per request with no cache — the
	// argument is on ResolveStaffPrincipal in proto/vigov/identity/v1/identity.proto.
	//
	// DIALLED, AND THEN REFUSED AT CONSTRUCTION IF THE ADDRESS IS MISSING. Starting without it
	// would produce a service that answers 503 to every member of staff while identity is healthy.
	dinhDanh, err := identityclient.Dial(cfg.IdentityGRPCAddr, cfg.GRPCCallerKey, log)
	if err != nil {
		return err
	}
	defer dinhDanh.Close()

	// 6. idempotency store. An empty REDIS_DSN is a valid deployment — local development with no
	//    cache — and each route then behaves per the CheDoHong it declared. A service must not fail
	//    to start because a cache is absent; the missing cache is already reported by cfg.CanhBao().
	//
	// The variable is declared as the INTERFACE and left nil when there is no Redis: assigning a
	// nil *idem.RedisStore into it would produce a non-nil interface holding a nil pointer, and
	// idem would call methods on it instead of taking its documented no-cache path.
	var idemStore idem.Store
	if cfg.RedisDSN != "" {
		r, err := idem.NewRedisStore(cfg.RedisDSN.Lo())
		if err != nil {
			return err
		}
		defer r.Close()
		idemStore = r
	}

	// 7. routes. Register refuses incomplete Deps at construction, not at request time.
	//
	// Checker is staffauth.Checker: it answers from the permission set the middleware obtained for
	// THIS request and put in its context, and it holds no state of its own. THE THREE WRITE ROUTES
	// OF THE CATALOGUE DECLARE authz.RequirePermission("admin.lookup"), so this is now read on every
	// one of them — Register panics on a nil Checker rather than letting them 403 or panic later.
	mux := http.NewServeMux()
	svchttp.Register(mux, svchttp.Deps{
		Checker:       staffauth.Checker{},
		LoaiVanBan:    loaiVanBan,
		GhiLoaiVanBan: ghiLoaiVanBan,
		Log:           log,
	})

	// Rule 11, invariant 1: the environment is read in core/config and nowhere else.
	// The default is this service's own — see config.ListenAddrHoac for why it lives here.
	addr := cfg.ListenAddrHoac(":8083")
	log.Info("starting", "service", "documents", "addr", addr,
		// secret.DSN redacts the password on every rendering path and keeps the host, so this line
		// still says which database was opened (rule 8).
		"dsn", cfg.DatabaseDSN)
	srv := &http.Server{
		Addr:              addr,
		Handler:           dungBien(mux, directory, dinhDanh, idemStore, log),
		ReadHeaderTimeout: 10 * time.Second,
	}
	return srv.ListenAndServe()
}

// dungBien builds the edge chain this binary serves.
//
// IT IS A FUNCTION SO A TEST CAN DRIVE THE REAL CHAIN. core/staffauth proves what the middleware
// does and core/httpx proves what TenantMiddleware does; neither can see whether THIS binary
// installs them. A middleware deleted from the chain below leaves a service that starts, serves and
// answers — and serves every request with no principal, which for a guarded route is 401 to
// everybody and for a Public one is a route with no commune check in front of it. Nothing else in
// this repository turns red for that, so cmd/server/main_test.go speaks through this function.
//
// ORDER MATTERS AND IS NOT NEGOTIABLE, outermost first:
//
//	StripTenantHeaders  a client naming its own commune is a client granting itself access
//	Recover             turns tenant.MustFrom's deliberate panic into a traceable 500
//	TenantMiddleware    resolves Host -> commune; unknown Host returns 404, never a default
//	idem.Middleware     installs the duplicate-request store for the routes that declare it
//	staffauth           rebuilds authz.Principal by asking identity; INSIDE TenantMiddleware,
//	                    because the outgoing call carries the commune from the context and the
//	                    principal is stamped with the commune resolved from Host
//
// Recover sits OUTSIDE TenantMiddleware so a panic raised while resolving the commune is still
// caught; it sits INSIDE StripTenantHeaders because stripping cannot panic.
//
// idem.Middleware sits AFTER TenantMiddleware because the idempotency key is prefixed with the
// commune (rule 1, invariant 7). Mounted the other way round it would build keys with no commune
// in them, so two communes whose clients generate the same key collide — one commune's request
// answered with another commune's result. It sits OUTSIDE staffauth only because it does not need
// the principal to be installed before it runs: idem.Required reads the principal from the context
// at REQUEST time, by which point staffauth (which is inside) has already put it there.
//
// /healthz IS DELIBERATELY OUTSIDE THE WHOLE CHAIN, on the outer mux. It answers whether this
// process is alive, which is true or false regardless of which commune is asking. Behind Host
// resolution it would fail whenever the platform service does, and an orchestrator would then
// restart a healthy process during somebody else's outage.
func dungBien(mux http.Handler, danhBa tenant.Directory, dinhDanh staffauth.Resolver,
	idemStore idem.Store, log *slog.Logger) http.Handler {

	var h http.Handler = mux
	h = staffauth.Middleware(dinhDanh, log)(h)
	h = idem.Middleware(idemStore, log)(h)
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
