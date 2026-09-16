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
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"google.golang.org/grpc"

	platformv1 "github.com/vihat/vigov/gen/vigov/platform/v1"
	"github.com/vihat/vigov/pkg/config"
	"github.com/vihat/vigov/pkg/grpcx"
	"github.com/vihat/vigov/pkg/httpx"
	svcgrpc "github.com/vihat/vigov/services/platform/internal/grpc"
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
	danhBa := svcstore.NewDirectory(db)
	directory := svcstore.NewCachedDirectory(danhBa, cfg.TenantCacheTTL)

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

	// 6. gRPC — the inter-service surface, on its OWN port. gRPC needs HTTP/2 and the REST
	//    surface is served to browsers over HTTP/1.1; one listener for both means
	//    demultiplexing by protocol, and every proxy and health check in front of the process
	//    then has to understand both.
	//
	// TODO(security): CALLER AUTHENTICATION ON THE gRPC PORT IS NOT IMPLEMENTED.
	//
	//	What is in place today: network isolation only. This port is internal to the cluster
	//	and must not be published. There is no mTLS, no service token, no caller identity —
	//	any process that can open a TCP connection here can call both RPCs.
	//
	//	Why that is tolerable for THIS service and no other: ADR 0003 means no business data
	//	crosses this boundary at all. ResolveHost returns a ULID that is already public — it
	//	travels in the QR deep link every citizen scans (ADR 0005) — and GetTenant returns
	//	registry metadata: name, host, active. An unauthenticated caller here learns which
	//	communes exist, and nothing about any of them.
	//
	//	REQUIRED BEFORE A REAL DEPLOYMENT, and required BEFORE any other service exposes gRPC:
	//	  1. mTLS between services, or a signed service token verified by a server interceptor
	//	  2. the caller's identity recorded on the call, so a cross-service read is attributable
	//	  3. this port bound to the internal interface only, never 0.0.0.0 on a public host
	//
	//	No stop-gap scheme is invented here on purpose. A hand-rolled shared secret would be
	//	replaced by whatever is chosen in step 1 anyway, and in the meantime it would read
	//	like authentication to anyone reviewing this file. A NAMED gap can be audited; a
	//	silent one cannot.
	grpcSrv := grpc.NewServer(
		// The commune is lifted out of metadata into context here, once, before any handler.
		// A call to a non-exempt RPC with no commune is refused with InvalidArgument — never
		// defaulted (rule 1, forbidden #1).
		grpc.UnaryInterceptor(grpcx.UnaryServerInterceptor()),
	)
	// The UNCACHED directory on purpose: the gRPC paths use ByHostErr / ByID, which keep
	// "no such commune" and "the database is down" apart, and CachedDirectory implements only
	// tenant.Directory, whose bool answer collapses the two. Caching this path needs a cache
	// that preserves the distinction — TODO(next), it matters once every service edge resolves
	// its Host through here (ADR 0004, decision 5).
	platformv1.RegisterPlatformServiceServer(grpcSrv, svcgrpc.NewServer(danhBa, log))

	grpcLis, err := net.Listen("tcp", cfg.GRPCListenAddr)
	if err != nil {
		return err
	}

	// Graceful shutdown. An administrative write cut in half by a deploy is a record in a state
	// the retention rules do not allow (rule 2, invariant 6).
	dungLai := make(chan os.Signal, 1)
	signal.Notify(dungLai, os.Interrupt, syscall.SIGTERM)

	// Buffered for two: either server may fail, and an unbuffered send from a goroutine
	// nobody is reading any more would leak it.
	loi := make(chan error, 2)
	go func() {
		log.Info("khởi động", "service", "platform", "addr", cfg.ListenAddr,
			"env", cfg.Env, "dsn", cfg.Redacted().DatabaseDSN)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			loi <- err
		}
	}()
	go func() {
		log.Info("khởi động gRPC", "service", "platform", "addr", cfg.GRPCListenAddr)
		// Serve returns nil after GracefulStop, so no ErrServerClosed equivalent to filter.
		if err := grpcSrv.Serve(grpcLis); err != nil {
			loi <- err
		}
	}()

	select {
	case err := <-loi:
		// One surface failing takes the process down rather than leaving it half-serving: a
		// platform answering HTTP but not ResolveHost looks healthy while every other
		// service's edge is failing to resolve its Host.
		grpcSrv.Stop()
		_ = srv.Close()
		return err

	case <-dungLai:
		log.Info("nhận tín hiệu dừng, đang đóng kết nối")
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		// Both surfaces drain, and neither waits for the other: an administrative write cut in
		// half by a deploy is a record in a state the retention rules do not allow (rule 2,
		// invariant 6).
		xongGRPC := make(chan struct{})
		go func() {
			grpcSrv.GracefulStop()
			close(xongGRPC)
		}()

		errHTTP := srv.Shutdown(ctx)

		select {
		case <-xongGRPC:
		case <-ctx.Done():
			// A call that will not finish must not hold a deploy open indefinitely. Forcing
			// the stop here is visible in the logs; hanging is not.
			log.Warn("gRPC không đóng kịp hạn, buộc dừng")
			grpcSrv.Stop()
		}
		return errHTTP
	}
}

// traceID returns the id a caller can quote when reporting a problem.
//
// TODO(next): lift this from the incoming request header once the reverse proxy sets one, so
// a single citizen complaint can be followed across services.
func traceID(context.Context) string { return "" }
