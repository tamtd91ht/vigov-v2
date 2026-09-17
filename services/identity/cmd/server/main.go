package main

// identity service — Tổ chức – cán bộ.
//
// This file only wires. No business logic, no init(), no global mutable state.
// See .claude/skills/go-service-pattern.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/vihat/vigov/pkg/config"
	"github.com/vihat/vigov/pkg/httpx"
	"github.com/vihat/vigov/pkg/idem"
	"github.com/vihat/vigov/pkg/migrate"
	"github.com/vihat/vigov/pkg/platformclient"
	"github.com/vihat/vigov/pkg/store"
	"github.com/vihat/vigov/pkg/tenant"
	"github.com/vihat/vigov/pkg/token"
	"github.com/vihat/vigov/services/identity/internal/app"
	svchttp "github.com/vihat/vigov/services/identity/internal/http"
	idstore "github.com/vihat/vigov/services/identity/internal/store"
	"github.com/vihat/vigov/services/identity/migrations"
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
	//    read at RUNTIME from the platform service (rule 1, invariant 10).
	cfg, err := config.Load("identity")
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
	// .Lo() is the one place the DSN leaves secret.DSN with its password intact. The driver
	// argument, and nowhere else: a raw copy in a variable is a password waiting for a log line.
	db, err := sql.Open("pgx", cfg.DatabaseDSN.Lo())
	if err != nil {
		return err
	}
	defer db.Close()

	// The same pool the platform service uses. Sized for 200+ communes sharing one process: too
	// large and the database runs out of connections long before the service runs out of work.
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	ctxPing, huy := context.WithTimeout(context.Background(), 15*time.Second)
	defer huy()
	if err := db.PingContext(ctxPing); err != nil {
		// Failing to start beats starting and refusing every sign-in: a service that cannot
		// reach its database cannot verify a single password, and every member of staff would
		// see "wrong email or password" for a reason that has nothing to do with either.
		return fmt.Errorf("identity: không nối được cơ sở dữ liệu: %w", err)
	}

	// 2b. schema migrations, BEFORE anything is served.
	//
	// WHY THE SERVICE MUST NOT START WHEN THIS FAILS: every query below is written against a
	// schema this process assumes is there. Serving on a schema of unknown shape does not fail
	// at startup where somebody is watching — it fails on the first request of whichever commune
	// happens to hit the missing column, and the error reaching the counter says nothing about a
	// migration. `0002_audit_log_append_only.sql` is the case in hand: until it runs, audit
	// entries are editable and everything downstream believes they are not.
	//
	// The files are EMBEDDED in this binary (services/identity/migrations), so what is applied is
	// what was compiled — not whatever happens to be on the container's disk.
	ctxMig, huyMig := context.WithTimeout(context.Background(), 5*time.Minute)
	kqMig, err := migrate.Chay(ctxMig, db, migrations.FS, "identity")
	huyMig()
	if err != nil {
		return fmt.Errorf("identity: migration không chạy được: %w", err)
	}
	// Logged even when nothing was applied: "applied 0 files" at startup is how an operator finds
	// out the replica is already at the schema they expected, without opening a psql prompt.
	log.Info("migration xong", "service", "identity", "da_ap", kqMig.DaAp, "bo_qua", len(kqMig.BoQua))

	kho := store.New(db)

	// 3. directory — Host -> commune, over gRPC to the platform service.
	//
	// THERE IS NO SECOND WAY TO RESOLVE A COMMUNE. This service does not read the registry
	// tables: they belong to the platform service, and a connection to another service's schema
	// is rule 2, forbidden #2 — the read path around the contract. A second way to answer
	// "which commune" is how one commune ends up serving another's data.
	//
	// The cache is what makes a network call per request affordable at 200+ communes; it lives
	// in pkg/tenant because every service edge needs the same one (ADR 0004, decision 5).
	nenTang, err := platformclient.Dial(cfg.PlatformGRPCAddr, log)
	if err != nil {
		return err
	}
	defer nenTang.Close()
	directory := tenant.NewCachedDirectory(nenTang, cfg.TenantCacheTTL)

	// 4. stores and the permission checker. Every one of them is built on *store.DB, which only
	//    hands out scoped access — there is no path here to an unscoped query (rule 1,
	//    invariant 5).
	checker := idstore.NewChecker(kho, log)
	canBo := idstore.NewCanBoStore(kho)
	phien := idstore.NewPhienStore(kho)

	// 5. ONE signer, and the variable is used twice on purpose.
	//
	// READ THIS BEFORE CHANGING THE NEXT TEN LINES. Two places hold a signer: app.DangNhap signs
	// the session token INSIDE its transaction (so a signing failure rolls back the session and
	// its audit entry together), and svchttp.Deps.Signer is what XacThuc verifies incoming
	// tokens with. Build two signers from two calls and the token issued at sign-in cannot be
	// read on the very next request: the person signs in successfully and is immediately treated
	// as signed out, with nothing in the logs to say why. One variable, passed twice, is the
	// only shape in which that cannot happen.
	//
	// NewSigner also enforces token.KhoaToiThieu. Refusing to start beats issuing sessions a
	// short key makes forgeable.
	signer, err := token.NewSigner(cfg.KhoaKyBytes())
	if err != nil {
		return fmt.Errorf("identity: khoá ký phiên không dùng được: %w", err)
	}
	log.Info("khoá ký phiên đã nạp", "so_khoa", signer.SoKhoa())

	// 6. use cases — the business write and its audit entry share one transaction inside these
	//    (rule 6, invariant 3). The handlers only translate HTTP.
	dangNhap := app.NewDangNhap(kho, canBo, phien, signer, log)
	dangXuat := app.NewDangXuat(kho, phien)

	// 7. idempotency store. An empty REDIS_DSN is a valid deployment — local development with no
	//    cache — and the routes then behave per the CheDoHong each one declared. A service must
	//    not fail to start because a cache is absent; the missing cache is already reported by
	//    cfg.CanhBao() above.
	//
	// The variable is declared as the INTERFACE and left nil when there is no Redis: assigning a
	// nil *idem.RedisStore into it would produce a non-nil interface holding a nil pointer, and
	// idem would then call methods on it instead of taking its documented no-cache path.
	var idemStore idem.Store
	if cfg.RedisDSN != "" {
		r, err := idem.NewRedisStore(cfg.RedisDSN.Lo())
		if err != nil {
			return err
		}
		defer r.Close()
		idemStore = r
	}

	// 8. routes. Register refuses incomplete Deps at construction, not at request time: a
	//    sign-in route mounted without a signing key or without the session registry would
	//    accept requests it cannot honour.
	deps := svchttp.Deps{
		Checker: checker,
		// The SAME *idstore.Checker behind two fields, and two fields on purpose: Checker DECIDES
		// one permission at a time and guards every route; Quyen only LISTS what the caller already
		// holds, so the admin web can avoid drawing what the server would refuse. See QuyenDoc.
		Quyen:  checker,
		Signer: signer, // the SAME pointer app.NewDangNhap was given above
		Phien:  phien,
		CanBo:  canBo,
		// The SAME store behind two fields, and two fields on purpose: CanBoDoc is the
		// three-condition read the session middleware runs on every request, CanBoDanhBa is the
		// register the Cấu hình → Người dùng screen pages through. See the note on CanBoDanhBa.
		DanhBa:   canBo,
		DangNhap: dangNhap,
		DangXuat: dangXuat,
		// THE SAME directory the edge below resolves Host with, deliberately not a second one.
		// GET /api/v1/commune has to turn the commune already in the context back into a name, and
		// two ways to answer "which commune is this Host" is how one commune ends up described
		// with another's name (see step 3 above: there is no second way to resolve a commune).
		Xa:  directory,
		Log: log,
	}

	mux := http.NewServeMux()
	svchttp.Register(mux, deps)

	// 9. The edge chain. Order matters and is not negotiable, outermost first:
	//
	//   StripTenantHeaders  a client naming its own commune is a client granting itself access
	//   Recover             turns tenant.MustFrom's deliberate panic into a traceable 500
	//   TenantMiddleware    resolves Host -> commune; unknown Host returns 404, never a default
	//   idem.Middleware     installs the duplicate-request store for the routes that declare it
	//   XacThuc             rebuilds the principal from the session cookie
	//
	// Recover sits OUTSIDE TenantMiddleware so a panic raised while resolving the commune is
	// still caught; it sits INSIDE StripTenantHeaders because stripping cannot panic.
	//
	// idem.Middleware sits AFTER TenantMiddleware because the idempotency key is prefixed with
	// the commune (rule 1, invariant 7). Mounted the other way round it would build keys with no
	// commune in them, so two communes sending the same client-supplied key would collide — one
	// commune's request answered with another commune's result.
	//
	// XacThuc sits INSIDE TenantMiddleware because it compares the commune in the token against
	// the commune resolved from Host, and reads the session registry scoped to that commune. It
	// panics deliberately if mounted outside; the alternative is a session lookup with no
	// commune, which reads every commune.
	var h http.Handler = mux
	h = svchttp.XacThuc(deps)(h)
	h = idem.Middleware(idemStore, log)(h)
	h = httpx.TenantMiddleware(directory)(h)
	h = httpx.Recover(traceID)(h)
	h = httpx.StripTenantHeaders(h)

	// /healthz is deliberately OUTSIDE the tenant chain, which is why it is registered on an
	// outer mux rather than on the one above. It answers whether this process is alive, which is
	// true or false regardless of which commune is asking. Behind Host resolution it would fail
	// whenever the platform service does, and an orchestrator would then restart a healthy
	// process during somebody else's outage — turning one dependency's blip into an outage of
	// its own.
	ngoai := http.NewServeMux()
	ngoai.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	ngoai.Handle("/", h)

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           ngoai,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// 10. Graceful shutdown. A sign-in cut in half by a deploy would leave a session row whose
	//     audit entry says a person signed in while no cookie was ever issued — a state the
	//     retention rules do not permit (rule 2, invariant 6).
	dungLai := make(chan os.Signal, 1)
	signal.Notify(dungLai, os.Interrupt, syscall.SIGTERM)

	loi := make(chan error, 1)
	go func() {
		log.Info("khởi động", "service", "identity", "addr", cfg.ListenAddr,
			"env", cfg.Env,
			// secret.DSN redacts the password on every rendering path and keeps the host, so
			// this line still says which database was opened (rule 8).
			"dsn", cfg.DatabaseDSN,
			"nen_tang", cfg.PlatformGRPCAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			loi <- err
		}
	}()

	select {
	case err := <-loi:
		return err

	case <-dungLai:
		log.Info("nhận tín hiệu dừng, đang đóng kết nối")
		ctx, huy := context.WithTimeout(context.Background(), 20*time.Second)
		defer huy()
		return srv.Shutdown(ctx)
	}
}

// traceID returns the id a caller can quote when reporting a problem.
//
// TODO(next): lift this from the incoming request header once the reverse proxy sets one, so a
// single citizen complaint can be followed across services.
func traceID(context.Context) string { return "" }
