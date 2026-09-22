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
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/core/migrate"
	"github.com/vihat/vigov/core/platformclient"
	"github.com/vihat/vigov/core/staffauth"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/app"
	svchttp "github.com/vihat/vigov/service-finance/internal/http"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
	"github.com/vihat/vigov/service-finance/migrations"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// TODO(skeleton): still to wire —
	//   5. consumers   event handlers; each refuses a message with no commune
	//
	// A consumer is NOT covered by the edge chain below: a message carries its commune inside
	// itself (rule 1, invariant 9), and a consumer that finds none refuses rather than guessing.

	// 1 + 2. THE POOL IS OPENED ONCE, HERE, and lives as long as the process.
	//
	// It used to be opened and closed inside the migration step, because this service had no store
	// to hand it to. It has one now, and a second pool opened later for the store would be a second
	// place the DSN is read and a second set of connections to size.
	//
	// THE CONFIG COMES BACK OUT WITH IT. It used to be read and dropped inside moCSDL, which was
	// enough while the DSN was the only thing anybody needed; the two gRPC addresses below are read
	// from the same Config, and reading the environment a second time would be a second answer to
	// one question.
	cfg, db, err := moCSDL(log)
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

	// 3. directory — Host -> commune, over gRPC to the platform service. There is no second way to
	// resolve a commune: the registry tables belong to the platform service, and a connection to
	// another service's schema is rule 2, forbidden #2. The cache is what makes a network call per
	// request affordable at 200+ communes (ADR 0004, decision 5).
	nenTang, err := platformclient.Dial(cfg.PlatformGRPCAddr, cfg.GRPCCallerKey, log)
	if err != nil {
		log.Error("không nối được dịch vụ nền tảng", "service", "finance", "err", err)
		os.Exit(1)
	}
	defer nenTang.Close()
	directory := tenant.NewCachedDirectory(nenTang, cfg.TenantCacheTTL)

	// 4. identity — session cookie -> staff principal, over gRPC. This service owns no session
	// registry and may not import identity's (rule 2, forbidden #1), so the principal comes from
	// the contract: ResolveStaffPrincipal, once per staff request, with no cache.
	dinhDanh, err := identityclient.Dial(cfg.IdentityGRPCAddr, cfg.GRPCCallerKey, log)
	if err != nil {
		log.Error("không nối được dịch vụ định danh", "service", "finance", "err", err)
		os.Exit(1)
	}
	defer dinhDanh.Close()

	// Deps.Checker is staffauth.Checker: it decides from the permission set the middleware obtained
	// for THIS request and holds no state of its own. The disbursement routes declare
	// authz.RequirePermission("budget.read"), so it is now load-bearing rather than wired ahead of
	// its first user — Register refuses to start without it.
	//
	// Deps.Nay is left nil ON PURPOSE: in production the derived disbursement figures are computed
	// against the real clock (Handler.nay). Only tests replace it, so that the delay arithmetic can
	// be exercised on the first and last days of a budget year.
	mux := http.NewServeMux()
	// Idempotency store. An empty REDIS_DSN is a valid deployment — local development with no cache
	// — and each route then behaves per the CheDoHong it declared. A service must not fail to start
	// because a cache is absent; the missing cache is already reported by cfg.CanhBao().
	//
	// The variable is declared as the INTERFACE and left nil when there is no Redis: assigning a
	// nil *idem.RedisStore into it would produce a non-nil interface holding a nil pointer, and
	// idem would call methods on it instead of taking its documented no-cache path.
	var idemStore idem.Store
	if cfg.RedisDSN != "" {
		r, err := idem.NewRedisStore(cfg.RedisDSN.Lo())
		if err != nil {
			log.Error("không mở được Redis cho chống trùng thao tác", "err", err)
			os.Exit(1)
		}
		defer r.Close()
		idemStore = r
	}

	hangMuc := fistore.NewHangMucKeHoachVonStore(kho)

	svchttp.Register(mux, svchttp.Deps{
		Checker: staffauth.Checker{},
		HangMuc: hangMuc,
		// The write use case owns the transaction the business write and its audit entry share
		// (rule 6, invariant 3). It is given *store.DB rather than a transaction because opening
		// one is precisely what it is for.
		GhiHangMuc: app.NewDanhMucHangMuc(kho, hangMuc),
		DuAn:       fistore.NewDuAnStore(kho),
		Log:        log,
	})

	// Rule 11, invariant 1: the environment is read in core/config and nowhere else.
	// The default is this service's own — see config.ListenAddrHoac for why it lives here.
	addr := cfg.ListenAddrHoac(":8086")
	log.Info("starting", "service", "finance", "addr", addr)
	if err := http.ListenAndServe(addr, dungBien(mux, directory, dinhDanh, idemStore, log)); err != nil {
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
	idemStore idem.Store, log *slog.Logger) http.Handler {

	var h http.Handler = mux
	h = staffauth.Middleware(dinhDanh, log)(h)
	// idem.Middleware sits AFTER TenantMiddleware because the idempotency key is prefixed with the
	// commune (rule 1, invariant 7). Mounted the other way round it would build keys with no
	// commune in them, so two communes whose clients generate the same key collide — one commune's
	// request answered with another commune's result, which is a breach between two authorities
	// through a cache key.
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

// moCSDL reads the configuration and opens this service's OWN database.
//
// "Its own" is not a manner of speaking: a connection string to a schema this service does not own
// is a read path around the contract (rule 2, forbidden #2).
//
// IT RETURNS THE CONFIG TOO, because the caller now needs the two gRPC addresses out of it. Reading
// the environment a second time up there would be a second answer to one question, and the copy
// that drifts is the one nobody is looking at.
func moCSDL(log *slog.Logger) (config.Config, *sql.DB, error) {
	// Platform-wide constants only. Per-commune values are read at RUNTIME (rule 1, invariant
	// 10) — there is nothing per-commune on this path in any case: the schema is shared by every
	// commune the process serves, partitioned by tenant_id rather than split per commune.
	cfg, err := config.Load("finance")
	if err != nil {
		return config.Config{}, nil, err
	}
	for _, canhBao := range cfg.CanhBao() {
		log.Warn("CẢNH BÁO CẤU HÌNH", "chi_tiet", canhBao)
	}

	// .Lo() is the ONE place the DSN leaves secret.DSN with its password intact: the driver
	// argument, and nowhere else (rule 8).
	db, err := sql.Open("pgx", cfg.DatabaseDSN.Lo())
	if err != nil {
		return config.Config{}, nil, err
	}

	ctx, huy := context.WithTimeout(context.Background(), 30*time.Second)
	defer huy()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return config.Config{}, nil, fmt.Errorf("finance: không nối được cơ sở dữ liệu: %w", err)
	}
	return cfg, db, nil
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
