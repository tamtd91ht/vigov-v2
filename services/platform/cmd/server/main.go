package main

// platform service — Nền tảng.
//
// This file only wires. No business logic, no init(), no global mutable state.
// See .claude/skills/go-service-pattern.

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/vihat/vigov/pkg/config"
	"github.com/vihat/vigov/pkg/httpx"
	svchttp "github.com/vihat/vigov/services/platform/internal/http"
	svcstore "github.com/vihat/vigov/services/platform/internal/store"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(log); err != nil {
		log.Error("service dừng", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	// 1. config — platform-wide constants from the environment ONLY. Per-commune values are
	//    read at RUNTIME from the database (rule 1, invariant 10).
	cfg, err := config.Load("platform")
	if err != nil {
		return err
	}
	for _, canhBao := range cfg.CanhBao() {
		// Reported at EVERY startup, never once at deploy time: a flag switched on for a local
		// afternoon is forgotten by the next morning (rule 8, invariant 7).
		log.Warn("CẢNH BÁO CẤU HÌNH", "chi_tiet", canhBao)
	}

	// 2. store — one schema per service; never another service's schema (rule 2).
	db, err := sql.Open("pgx", cfg.DatabaseDSN)
	if err != nil {
		return err
	}
	defer db.Close()

	// A pool sized for 200+ communes sharing one process. Too large and the database runs out
	// of connections long before the service runs out of work.
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		// Failing to start beats starting and returning 404 for every commune: a service that
		// cannot reach its database cannot resolve a single Host, and every request would look
		// like an unknown commune.
		return err
	}

	// 3. directory — resolves Host -> commune, cached with a short TTL because this sits on the
	//    path of EVERY request at 200+ communes (ADR 0004, decision 5).
	directory := svcstore.NewCachedDirectory(svcstore.NewDirectory(db), cfg.TenantCacheTTL)

	// 4. checker — authz.Checker backed by the identity service.
	// TODO(next): identity does not expose the permission contract yet. Until it does, no
	// business route may be registered here: a route without a real checker is a route open to
	// every signed-in account (rule 5, invariant 2).

	// 5. consumers — event handlers; each refuses a message with no commune.
	// TODO(next): no events published yet.

	mux := http.NewServeMux()

	// /healthz is deliberately NOT behind the tenant middleware: it answers whether this
	// process is alive, which is true or false regardless of which commune is asking. Putting
	// it behind Host resolution would make the health check fail whenever the directory fails,
	// and an orchestrator would then restart a healthy process during a database blip.
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
	// Recover sits OUTSIDE TenantMiddleware so a panic raised while resolving the commune is
	// still caught; it sits INSIDE StripTenantHeaders because stripping cannot panic.
	var h http.Handler = mux
	h = httpx.TenantMiddleware(directory)(h)
	h = httpx.Recover(traceID)(h)
	h = httpx.StripTenantHeaders(h)

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Graceful shutdown. An administrative write cut in half by a deploy is a record in a state
	// the retention rules do not allow (rule 2, invariant 6).
	dungLai := make(chan os.Signal, 1)
	signal.Notify(dungLai, os.Interrupt, syscall.SIGTERM)

	loi := make(chan error, 1)
	go func() {
		log.Info("khởi động", "service", "platform", "addr", cfg.ListenAddr,
			"env", cfg.Env, "dsn", cfg.Redacted().DatabaseDSN)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			loi <- err
		}
	}()

	select {
	case err := <-loi:
		return err
	case <-dungLai:
		log.Info("nhận tín hiệu dừng, đang đóng kết nối")
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		return srv.Shutdown(ctx)
	}
}

// traceID returns the id a caller can quote when reporting a problem.
//
// TODO(next): lift this from the incoming request header once the reverse proxy sets one, so
// a single citizen complaint can be followed across services.
func traceID(context.Context) string { return "" }
