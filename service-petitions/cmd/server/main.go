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
	"github.com/vihat/vigov/core/migrate"
	pkgstore "github.com/vihat/vigov/core/store"
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
	// TODO(skeleton): still to wire, in this order —
	//   1. directory   tenant.Directory backed by the platform service, cached with a short
	//                  TTL — this sits on the path of every request at 200+ communes
	//   2. session     the middleware that turns the session cookie into an authz.Principal.
	//                  service-identity/internal/http/middleware.go is the shape it takes
	//   3. checker     authz.Checker backed by the identity service
	//   4. consumers   event handlers; each refuses a message with no commune
	//
	// UNTIL 1 AND 2 EXIST, THE ROUTES BELOW ANSWER 401 TO EVERYTHING, and that is fail-closed
	// rather than broken: authz.AnyAuthenticated looks for a principal first and finds none, so no
	// request ever reaches a handler and no query ever runs without a commune. Mounting them now is
	// what makes the wiring — and the tests around it — real rather than a plan.

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

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	svchttp.Register(mux, svchttp.Deps{
		// Deps.Checker is left nil ON PURPOSE: no route in this service declares a permission yet,
		// and a stand-in checker wired "so it is not nil" is a stand-in that answers questions
		// nobody asked it — the first route to use it would be guarded by a fake. The day a
		// RequirePermission route is added, the real checker is wired here and Register's panic
		// switch gains a case for it.
		LoaiNhiemVu: petstore.NewLoaiNhiemVuStore(kho),
		MucUuTien:   petstore.NewMucUuTienNhiemVuStore(kho),
		Log:         log,
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
		addr = ":8084"
	}
	log.Info("starting", "service", "petitions", "addr", addr)
	return http.ListenAndServe(addr, mux)
}

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
