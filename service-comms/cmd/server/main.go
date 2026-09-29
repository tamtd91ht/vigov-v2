package main

// comms service — Thông tin – truyền thông.
//
// This file only wires. No business logic, no init(), no global mutable state.
// See .claude/skills/go-service-pattern.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"google.golang.org/grpc"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/config"
	"github.com/vihat/vigov/core/crypto"
	commsv1 "github.com/vihat/vigov/core/gen/vigov/comms/v1"
	"github.com/vihat/vigov/core/grpcx"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/core/migrate"
	"github.com/vihat/vigov/core/platformclient"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/staffauth"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	commsapp "github.com/vihat/vigov/service-comms/internal/app"
	svcgrpc "github.com/vihat/vigov/service-comms/internal/grpc"
	svchttp "github.com/vihat/vigov/service-comms/internal/http"
	"github.com/vihat/vigov/service-comms/internal/mail"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
	"github.com/vihat/vigov/service-comms/migrations"
)

// configUses is every configuration group this binary reads — and so, in staging and prod, every
// group whose variables must be set for it to start (core/config/uses.go). Undeclared groups are
// not read at all. TestConfigUsesMatchReads keeps this list equal to what the package reads.
//
// comms: REST, plus since 2026-09-29 a gRPC server (GRPCServer) for DeliverStaffNotifications — the
// automation jobs' notices into the header bell (ADR 0058 §3); the one service storing per-commune
// secrets (SecretEncryption).
var configUses = config.Uses(
	config.HTTPServer,
	config.GRPCServer,
	config.PlatformClient,
	config.IdentityClient,
	config.TenantCache,
	config.Redis,
	config.CitizenCORS,
	config.SecretEncryption,
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Steps 1 to 4 of the wiring list are below and are no longer a skeleton: config, the pool, the
	// commune directory and the staff-authentication edge are all real.
	//
	// TODO(skeleton): still missing —
	//   5. consumers   the HANDLER now exists — internal/event.CitizenReportStatusChanged, which consumes
	//                  `petitions.status_changed.v1` and writes the citizen notification ledger.
	//                  TWO THINGS ARE MISSING BEFORE IT CAN BE WIRED HERE, and both are decisions
	//                  rather than code: (a) a Kafka client — ADR 0010 puts inter-service events on
	//                  Kafka, but `core/events.Publisher` is still a bare interface with no
	//                  implementation, and a broker client nobody has decided does not belong in a
	//                  wiring file; (b) an implementation of event.TemplateResolver — which APPROVED ZNS
	//                  template this commune sends for a transition is per-commune configuration
	//                  (ADR 0018, consequence 1) with no store, no adapter and no customer answer
	//                  yet. A constant here would be one commune's template id serving 200+.
	//
	// A consumer is NOT covered by the edge chain below: a message carries its commune inside
	// itself (rule 1, invariant 9), and a consumer that finds none refuses rather than guessing.

	// 1. config — platform-wide constants only. Per-commune values are read at RUNTIME (rule 1,
	// invariant 10); there is nothing per-commune on this path in any case, because the schema is
	// shared by every commune the process serves and partitioned by tenant_id.
	cfg, err := config.Load("comms", configUses)
	if err != nil {
		log.Error("cấu hình không nạp được", "service", "comms", "err", err)
		os.Exit(1)
	}
	for _, warning := range cfg.CanhBao() {
		log.Warn("CẢNH BÁO CẤU HÌNH", "chi_tiet", warning)
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
	if err := runMigrations(log, db); err != nil {
		log.Error("migration không chạy được", "service", "comms", "err", err)
		os.Exit(1)
	}

	// THE ONLY HANDLE THE BUSINESS CODE EVER SEES. *sql.DB stops here: core/store.New wraps it and
	// exposes nothing but Scoped, which binds tenant_id from the context. A repository built from a
	// raw pool is a repository that can read every commune at once (rule 1, invariant 5).
	scoped := pkgstore.New(db)

	// 3. directory — Host -> commune, over gRPC to the platform service. There is no second way to
	// resolve a commune: the registry tables belong to the platform service, and a connection to
	// another service's schema is rule 2, forbidden #2. The cache is what makes a network call per
	// request affordable at 200+ communes (ADR 0004, decision 5).
	//
	// NEITHER CLIENT IS CLOSED ON A defer, for the reason already stated on the pool above: this
	// process exits through os.Exit, which runs no defers.
	platform, err := platformclient.Dial(cfg.PlatformGRPCAddr(), cfg.GRPCCallerKey(), log)
	if err != nil {
		log.Error("không nối được dịch vụ nền tảng", "service", "comms", "err", err)
		os.Exit(1)
	}
	directory := tenant.NewCachedDirectory(platform, cfg.TenantCacheTTL())

	// 4. identity — session cookie -> staff principal, over gRPC. This service owns no session
	// registry and may not import identity's (rule 2, forbidden #1), so the principal comes from
	// the contract: ResolveStaffPrincipal, once per staff request, with no cache.
	identity, err := identityclient.Dial(cfg.IdentityGRPCAddr(), cfg.GRPCCallerKey(), log)
	if err != nil {
		log.Error("không nối được dịch vụ định danh", "service", "comms", "err", err)
		os.Exit(1)
	}

	// Deps.Checker is staffauth.Checker: it decides from the permission set the middleware obtained
	// for THIS request and holds no state of its own. No mounted route declares
	// authz.RequirePermission yet; wiring it now is what makes the first one that does work rather
	// than meet a nil interface at request time.
	// Idempotency store. An empty REDIS_DSN happens in DEV only — local development with no cache —
	// and each route then behaves per the CheDoHong it declared. Staging and prod refuse to start
	// without it (config.Redis is declared in configUses).
	//
	// The variable is declared as the INTERFACE and left nil when there is no Redis: assigning a
	// nil *idem.RedisStore into it would produce a non-nil interface holding a nil pointer, and
	// idem would call methods on it instead of taking its documented no-cache path.
	var idemStore idem.Store
	if cfg.RedisDSN() != "" {
		r, err := idem.NewRedisStore(cfg.RedisDSN().Lo())
		if err != nil {
			log.Error("không mở được Redis cho chống trùng thao tác", "service", "comms", "err", err)
			os.Exit(1)
		}
		idemStore = r
	}

	mapAssetTypes := commsstore.NewMapAssetTypeStore(scoped)

	// The internal announcement book (migration 0005). ONE store behind both routes: the read is a
	// query, the write goes through the use case that owns the transaction its audit entry shares.
	announcements := commsstore.NewAnnouncementStore(scoped)

	// Mini App content (migration 0006). TWO stores for two tables, and the content write use case
	// takes BOTH: composing an item has to check that the category it is filed under is a live
	// category of this commune, which is a read of the other table inside the SAME transaction
	// (rule 6, invariant 3 — the audit entry shares it).
	contentItems := commsstore.NewContentItemStore(scoped)
	contentCategories := commsstore.NewContentCategoryStore(scoped)

	// The map field schema (migration 0007). One store behind the read route and the write use
	// case; the use case owns the transaction its audit entry shares.
	mapFieldSchemas := commsstore.NewMapFieldSchemaStore(scoped)

	// The commune's mail server (migration 0008) and the envelope that seals its password (ADR 0009).
	//
	// A NIL *crypto.Envelope IS A VALID PROCESS, and a deliberate one: SECRET_ENCRYPTION_KEYS is
	// optional at config.Load (core/config/secret_encryption.go), because making it required would
	// stop this service on every machine to protect one screen. Without it the two mail-settings
	// WRITE routes answer 503 by name and write nothing; every other route keeps serving. A MALFORMED
	// value never reaches here — config.Load refuses it and the pod does not start.
	var envelope *crypto.Envelope
	if cfg.SecretEncryptionConfigured() {
		envelope, err = crypto.New(cfg.SecretEncryptionKeys(), commsstore.NewDataEncryptionKeyStore(scoped))
		if err != nil {
			log.Error("không dựng được bộ niêm bí mật theo xã", "service", "comms", "err", err)
			os.Exit(1)
		}
	} else {
		log.Warn("CẢNH BÁO CẤU HÌNH", "chi_tiet",
			"SECRET_ENCRYPTION_KEYS trống — lưu và gửi thử máy chủ thư của xã sẽ trả 503 (ADR 0009)")
	}
	mailSettings := commsapp.NewMailSettingsAdmin(scoped, commsstore.NewMailSettingsStore(scoped), envelope,
		mail.NewSender(nil, mail.DefaultTimeout))

	// The header-bell inbox (migration 0010). ONE store behind the staff reads and the use case; the
	// use case owns every write's transaction and audit entry — the REST mark-read routes AND the gRPC
	// delivery below share it.
	staffInbox := commsstore.NewStaffNotificationStore(scoped)
	staffNotifications := commsapp.NewStaffNotifications(scoped, staffInbox)

	mux := http.NewServeMux()
	svchttp.Register(mux, svchttp.Deps{
		Checker:            staffauth.Checker{},
		MapAssetTypes:      mapAssetTypes,
		Announcements:      announcements,
		WriteAnnouncements: commsapp.NewAnnouncements(scoped, announcements),

		ContentItems:           contentItems,
		WriteContentItems:      commsapp.NewContentItems(scoped, contentItems, contentCategories),
		ContentCategories:      contentCategories,
		WriteContentCategories: commsapp.NewContentCategories(scoped, contentCategories),
		// The write use case owns the transaction the business write and its audit entry share
		// (rule 6, invariant 3). It is given *store.DB rather than a transaction because opening one
		// is precisely what it is for.
		WriteMapAssetTypes:   commsapp.NewMapAssetTypeCatalogue(scoped, mapAssetTypes),
		MapFieldSchemas:      mapFieldSchemas,
		WriteMapFieldSchemas: commsapp.NewMapFieldSchemas(scoped, mapFieldSchemas),
		MailSettings:         mailSettings,
		WriteMailSettings:    mailSettings,
		// This service's OWN audit_log, on its own handle — never another service's (ADR 0054 §1).
		AuditLog:        audit.NewLog(scoped),
		StaffInbox:      staffInbox,
		WriteStaffInbox: staffNotifications,
		Log:             log,
	})

	// THE PUBLIC SURFACE (owner decision 2026-09-27) — its own mux, its own Deps, its own chain. `platform`
	// is the SAME platform client the Host edge uses, asked through XaTheoHost so an outage is a 503 and
	// never "no such commune". The two content stores are the SAME ones the staff routes use, reached
	// only through their published-only reads.
	publicMux := http.NewServeMux()
	svchttp.RegisterPublic(publicMux, svchttp.PublicDeps{
		Tenants:      platform,
		ContentItems: contentItems,
		Categories:   contentCategories,
		Log:          log,
	})
	public := buildPublicChain(publicMux, cfg.CitizenCORSAllowedOrigins())

	// Rule 11, invariant 1: the environment is read in core/config and nowhere else.
	// LISTEN_ADDR or ":8080" — one default for every service, see config.Config.ListenAddr.
	addr := cfg.ListenAddr()
	log.Info("starting", "service", "comms", "addr", addr)
	srv := &http.Server{
		Addr: addr,
		// OUTERMOST, around BOTH chains: every layer reads one client address per request, crossing
		// only the proxies TRUSTED_PROXY_CIDRS names (rule 6, invariant 2).
		Handler:           httpx.ClientIPTuProxyTinCay(cfg.TrustedProxies())(buildEdge(mux, public, directory, identity, idemStore, log)),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// THE gRPC SURFACE — its own port (GRPC_LISTEN_ADDR, default :9090), the same shape as petitions.
	// One business RPC, DeliverStaffNotifications (ADR 0058 §3), writing through the SAME use case the
	// bell's REST routes use.
	//
	// Plaintext, like every gRPC port here (ADR 0025): the guard is GRPC_CALLER_KEY on every RPC plus
	// the NetworkPolicy confining the port to the cluster — both, not either.
	grpcSrv := buildGRPCServer(cfg.GRPCCallerKey(), svcgrpc.Deps{Notifications: staffNotifications, Log: log})
	grpcLis, err := net.Listen("tcp", cfg.GRPCListenAddr())
	if err != nil {
		log.Error("không mở được cổng gRPC", "service", "comms", "addr", cfg.GRPCListenAddr(), "err", err)
		os.Exit(1)
	}

	// ĐÓNG ÊM. Trước 2026-09-22 bốn dịch vụ này gọi thẳng `http.ListenAndServe`, nên `SIGTERM`
	// giết tiến trình NGAY — giữa một yêu cầu đang chạy, giữa một giao dịch chưa commit.
	//
	// VÌ SAO NÓ ĐẮT Ở ĐÂY CHỨ KHÔNG PHẢI MỘT CHI TIẾT VẬN HÀNH: mọi tuyến ghi của kho này viết
	// bản ghi nghiệp vụ VÀ dòng vết kiểm toán trong CÙNG một giao dịch (luật 6, bất biến 3). Một
	// tiến trình chết giữa chừng thì giao dịch ấy bị CSDL cuộn lại — điều đó vẫn đúng. Cái mất là
	// thứ nằm NGOÀI giao dịch: công dân đã cầm mã tra cứu trên tay, hoặc người gửi đã nhận HTTP
	// 202, trong khi phía máy chủ không còn gì cả. Không vết nào ghi rằng chuyện đó đã xảy ra, vì
	// dòng vết cũng vừa bị cuộn lại.
	//
	// 20 GIÂY, và con số ấy phải NHỎ HƠN `terminationGracePeriodSeconds` của manifest (45). Ngược
	// lại thì k8s `SIGKILL` trước khi hạn ở đây trôi hết, và toàn bộ đoạn mã này trở thành thứ
	// trông như đang canh mà không bao giờ chạy tới cuối.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Buffered for two: either server may fail, and a send nobody reads would leak its goroutine.
	serveErr := make(chan error, 2)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()
	go func() {
		log.Info("starting gRPC", "service", "comms", "addr", cfg.GRPCListenAddr())
		// Serve returns nil after GracefulStop, so there is no ErrServerClosed to filter.
		if err := grpcSrv.Serve(grpcLis); err != nil {
			serveErr <- err
		}
	}()

	// `os.Exit` CHỨ KHÔNG `return err` NHƯ HAI DỊCH VỤ KIA, và sự khác nhau ấy không phải tuỳ
	// hứng: dịch vụ này chưa tách một hàm `chay(log) error` ra khỏi `main`, nên ở đây không có
	// chỗ nào để trả lỗi về. Giữ nguyên cách báo lỗi vốn có của tệp thay vì tách hàm nhân một
	// lượt vá đóng êm — tách `main` là một thay đổi khác, với lý do khác.
	//
	// ⚠ `os.Exit` KHÔNG CHẠY `defer`. Mọi thứ phải dọn khi tiến trình dừng phải nằm TRƯỚC lời
	// gọi ấy, không nằm trong một `defer` phía trên.
	select {
	case err := <-serveErr:
		// One surface failing takes the process down rather than leaving it half-serving: REST up with
		// gRPC down looks healthy while every automation job's delivery is refused.
		log.Error("server stopped", "err", err)
		grpcSrv.Stop()
		_ = srv.Close()
		os.Exit(1)
	case <-stop:
		log.Info("nhận tín hiệu dừng, đang đóng kết nối", "service", "comms")
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		// Both surfaces drain in parallel, inside the same 20 seconds (< the manifest's 45).
		grpcDone := make(chan struct{})
		go func() {
			grpcSrv.GracefulStop()
			close(grpcDone)
		}()
		errHTTP := srv.Shutdown(ctx)
		select {
		case <-grpcDone:
		case <-ctx.Done():
			log.Warn("gRPC không đóng kịp hạn, buộc dừng", "service", "comms")
			grpcSrv.Stop()
		}
		if errHTTP != nil {
			log.Error("đóng không sạch", "err", errHTTP)
			os.Exit(1)
		}
	}
}

// buildGRPCServer builds the inter-service gRPC surface with its COMPLETE interceptor chain.
//
// A NAMED FUNCTION SO A TEST CAN START IT (grpc_server_test.go): core/grpcx proves the interceptors
// refuse what they should, but only a test of THIS function sees whether this binary installs them.
//
// ORDER IS NOT NEGOTIABLE: caller key first, so an unauthenticated caller never reaches the commune
// logic; then the commune from metadata into context. DeliverStaffNotifications is NOT tenant-exempt,
// so a call without "x-tenant-id" is refused with InvalidArgument before the handler (rule 1).
func buildGRPCServer(callerKey secret.Secret, d svcgrpc.Deps) *grpc.Server {
	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			// Panics at construction when GRPC_CALLER_KEY is empty: a server without the key accepts
			// every call it should refuse, and nothing looks wrong.
			grpcx.UnaryServerCallerAuth(callerKey, d.Log),
			grpcx.UnaryServerInterceptor(),
		),
	)
	commsv1.RegisterCommsServiceServer(srv, svcgrpc.NewServer(d))
	return srv
}

// buildEdge builds the edge chain this binary serves.
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
//
// # TWO CHAINS, ONE PORT, SPLIT ON THE OUTER MUX BY PATH (since 2026-09-27)
//
//	/api/v1/commune-news, /api/v1/commune-news/…   PUBLIC chain (public, buildPublicChain)
//	everything else                                STAFF chain — commune from `Host`
//
// The split exists because the two disagree on the first question of every request — which commune.
// The Mini App calls the reserved API host (ADR 0046), which TenantMiddleware answers 404, so a public
// route mounted on the staff mux would start, pass every internal/http test, and 404 every resident.
// `commune-news` is its own path ELEMENT: Go's ServeMux matches whole elements, so neither pattern can
// capture `/api/v1/content-items/…`, the staff register. main_test.go asserts both directions.
func buildEdge(mux, public http.Handler, directory tenant.Directory, identity staffauth.Resolver,
	idemStore idem.Store, log *slog.Logger) http.Handler {

	var h http.Handler = mux
	h = staffauth.Middleware(identity, log)(h)
	// idem.Middleware sits AFTER TenantMiddleware because the idempotency key is prefixed with the
	// commune (rule 1, invariant 7). Mounted the other way round it would build keys with no
	// commune in them, so two communes whose clients generate the same key collide — one commune's
	// request answered with another commune's result, which is a breach between two authorities
	// through a cache key.
	h = idem.Middleware(idemStore, log)(h)
	h = httpx.TenantMiddleware(directory)(h)
	h = httpx.Recover(traceID)(h)
	h = httpx.StripTenantHeaders(h)

	outer := http.NewServeMux()
	outer.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	// BOTH patterns, and the first is load-bearing: the subtree pattern alone does NOT match the
	// collection itself — ServeMux answers `/api/v1/commune-news` with a redirect to the slash form
	// (service-petitions/cmd/server/main.go §TWO PATTERNS measured it), and following it lands on
	// `/api/v1/commune-news/` — a path no public route matches, so the list would 404.
	outer.Handle(svchttp.CommuneNewsPath, public)
	outer.Handle(svchttp.CommuneNewsPath+"/", public)
	outer.Handle("/", h)
	return outer
}

// buildPublicChain builds the PUBLIC edge chain, outermost last:
//
//	CORSCongDan         the Mini App webview is cross-origin; a preflight is answered before anything
//	                    below sees it. The SAME middleware and the SAME CITIZEN_CORS_ALLOWED_ORIGINS the
//	                    citizen chains of petitions and identity use — the Mini App is one origin set.
//	                    PUBLIC chain ONLY: staff are same-origin with a host-only cookie (ADR 0043)
//	StripTenantHeaders  a client naming its own commune is granting itself access — heavier here, where
//	                    there is no `Host` and no session to contradict it
//	Recover             a panic becomes a traceable 500 — including tenant.MustFrom's, if a public
//	                    handler ever reached a scoped store before resolving a commune
//
// NO TenantMiddleware, NO staffauth, NO idem: the reserved API host maps to no commune, there is no
// session, and nothing on this surface writes. The commune is resolved per request from `?host=`.
//
// A NAMED FUNCTION so main_test.go can drive the real chain.
func buildPublicChain(publicMux http.Handler, corsOrigins httpx.NguonCORS) http.Handler {
	c := publicMux
	c = httpx.Recover(traceID)(c)
	c = httpx.StripTenantHeaders(c)
	c = httpx.CORSCongDan(corsOrigins)(c)
	return c
}

// traceID returns the id a caller can quote when reporting a problem.
//
// TODO(next): lift this from the incoming request header once the reverse proxy sets one, so a
// single citizen complaint can be followed across services. Same shape and same TODO as identity's.
func traceID(context.Context) string { return "" }

// runMigrations applies this service's embedded migrations to this service's OWN database.
//
// IT TAKES THE POOL RATHER THAN OPENING ONE, which is the fold the previous version of this
// comment predicted: the repositories need the same pool, and two pools against one database
// would double the connection count for no reason. The migration call itself did not change.
func runMigrations(log *slog.Logger, db *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("comms: không nối được cơ sở dữ liệu: %w", err)
	}

	res, err := migrate.Chay(ctx, db, migrations.FS, "comms")
	if err != nil {
		return err
	}
	// Logged even when nothing was applied: "applied 0 files" at startup is how an operator
	// finds out the replica is already at the schema they expected, without opening a psql
	// prompt.
	log.Info("migration xong", "service", "comms", "da_ap", res.DaAp, "bo_qua", len(res.BoQua))
	return nil
}
