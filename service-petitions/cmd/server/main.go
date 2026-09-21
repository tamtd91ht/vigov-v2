package main

// petitions service — Tiếp dân.
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
	"github.com/vihat/vigov/service-petitions/internal/app"
	svchttp "github.com/vihat/vigov/service-petitions/internal/http"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
	"github.com/vihat/vigov/service-petitions/migrations"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// main only starts and reports. Everything else is in chay() so the database pool can be
	// closed by a `defer` — with os.Exit in the middle of the wiring, no deferred close ever runs.
	if err := chay(log); err != nil {
		log.Error("petitions không khởi động được", "service", "petitions", "err", err)
		os.Exit(1)
	}
}

// chay wires the service and serves until the listener fails.
func chay(log *slog.Logger) error {
	// TODO(skeleton): still to wire —
	//   4. consumers   event handlers; each refuses a message with no commune
	//
	// A consumer is NOT covered by the edge chain below: a message carries its commune inside
	// itself (rule 1, invariant 9), and a consumer that finds none refuses rather than guessing.

	// Platform-wide constants only. Per-commune values are read at RUNTIME from the platform
	// service (rule 1, invariant 10) — there is nothing per-commune on this path in any case: the
	// schema is shared by every commune the process serves, partitioned by tenant_id rather than
	// split per commune.
	cfg, err := config.Load("petitions")
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
		return fmt.Errorf("petitions: không mở được kết nối: %w", err)
	}
	defer db.Close()

	// ONE POOL FOR THE WHOLE PROCESS. It used to be opened and closed inside the migration step,
	// because the service had no store; the stores below share this one, which is what the comment
	// on that step said would happen the day store.New arrived.
	khoiDong, huy := context.WithTimeout(context.Background(), 5*time.Minute)
	defer huy()
	if err := db.PingContext(khoiDong); err != nil {
		return fmt.Errorf("petitions: không nối được cơ sở dữ liệu: %w", err)
	}

	// SCHEMA MIGRATIONS run before the listener opens, and a failure stops the process. Starting on
	// a schema whose shape is unknown is worse than not starting: the fault then surfaces on
	// somebody's first request, as an error that says nothing about a migration.
	if err := chayMigration(khoiDong, log, db); err != nil {
		return err
	}

	// *store.DB is the only handle the repositories get. There is deliberately no path from here
	// that hands a repository the raw *sql.DB — a repository that can be built without a commune is
	// a repository that can query across communes (rule 1, invariant 5).
	kho := pkgstore.New(db)

	// 1. directory — Host -> commune, over gRPC to the platform service. There is no second way to
	// resolve a commune: the registry tables belong to the platform service, and a connection to
	// another service's schema is rule 2, forbidden #2. The cache is what makes a network call per
	// request affordable at 200+ communes (ADR 0004, decision 5).
	nenTang, err := platformclient.Dial(cfg.PlatformGRPCAddr, cfg.GRPCCallerKey, log)
	if err != nil {
		return err
	}
	defer nenTang.Close()
	directory := tenant.NewCachedDirectory(nenTang, cfg.TenantCacheTTL)

	// 2 + 3. identity — session cookie -> staff principal, over gRPC, and the Checker that reads
	// the permission set it returns.
	//
	// WHY NOT A LOCAL COPY OF identity's XacThuc: the session registry and the grants live inside
	// service-identity/internal/, which rule 2, forbidden #1 forbids this service from importing,
	// and a second implementation of "is this session still valid" would disagree with the first on
	// the day a session is revoked. One RPC, once per staff request, with no cache — the argument
	// is on ResolveStaffPrincipal in proto/vigov/identity/v1/identity.proto.
	dinhDanh, err := identityclient.Dial(cfg.IdentityGRPCAddr, cfg.GRPCCallerKey, log)
	if err != nil {
		return err
	}
	defer dinhDanh.Close()

	mux := http.NewServeMux()
	svchttp.Register(mux, svchttp.Deps{
		// Deps.Checker is staffauth.Checker: it decides from the permission set the middleware
		// obtained for THIS request and holds no state of its own — not a stand-in that answers
		// questions nobody asked it. It is now load-bearing: GET /api/v1/citizen-reports/{maTraCuu}
		// declares authz.RequirePermission("feedback.read"), which is the first route in this
		// service to guard a real operation.
		Checker:     staffauth.Checker{},
		LoaiNhiemVu: petstore.NewLoaiNhiemVuStore(kho),
		MucUuTien:   petstore.NewMucUuTienNhiemVuStore(kho),
		Phieu:       petstore.NewPhieuPhanAnhStore(kho),
		NhanLinhVuc: petstore.NewNhanLinhVucStore(kho),
		// The trail for a full-view read of a reporter's name and number. It takes the same
		// *store.DB as the repositories because it opens its own transaction: rule 6, invariant 3
		// admits no audit write outside one, and audit.Write takes only a *store.ScopedTx.
		Vet: app.NewXemNguoiGui(kho),
		Log: log,
	})

	// Rule 11, invariant 1: the environment is read in core/config and nowhere else.
	// The default is this service's own — see config.ListenAddrHoac for why it lives here.
	addr := cfg.ListenAddrHoac(":8084")
	log.Info("starting", "service", "petitions", "addr", addr)
	return http.ListenAndServe(addr, dungBien(mux, directory, dinhDanh, log))
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

// chayMigration applies this service's embedded migrations to its OWN database.
//
// It is wired ahead of the rest on purpose. The schema is what every other step is written
// against, and `0002_audit_log_append_only.sql` — which turns "append-only" from a comment into a
// database constraint — had been committed and never applied by anything.
func chayMigration(ctx context.Context, log *slog.Logger, db *sql.DB) error {
	kq, err := migrate.Chay(ctx, db, migrations.FS, "petitions")
	if err != nil {
		return err
	}
	// Logged even when nothing was applied: "applied 0 files" at startup is how an operator finds
	// out the replica is already at the schema they expected, without opening a psql prompt.
	log.Info("migration xong", "service", "petitions", "da_ap", kq.DaAp, "bo_qua", len(kq.BoQua))
	return nil
}
