package main

// platform service — Nền tảng.
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

	"github.com/vihat/vigov/core/config"
	platformv1 "github.com/vihat/vigov/core/gen/vigov/platform/v1"
	"github.com/vihat/vigov/core/grpcx"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/core/malwarescan"
	"github.com/vihat/vigov/core/migrate"
	"github.com/vihat/vigov/core/platformclient/uploadpolicy"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/staffauth"
	"github.com/vihat/vigov/core/storage"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	svcapp "github.com/vihat/vigov/service-platform/internal/app"
	svcgrpc "github.com/vihat/vigov/service-platform/internal/grpc"
	svchttp "github.com/vihat/vigov/service-platform/internal/http"
	svcstore "github.com/vihat/vigov/service-platform/internal/store"
	"github.com/vihat/vigov/service-platform/migrations"
)

// configUses is every configuration group this binary reads — and so, in staging and prod, every
// group whose variables must be set for it to start (core/config/uses.go). Undeclared groups are
// not read at all. TestConfigUsesMatchReads keeps this list equal to what the package reads.
//
// platform: serves REST and the inter-service port, and caches its own registry. Since 2026-10-01 it
// is also the operator area's only HTTP edge (ADR 0048 §01/10 #2): OperatorEdge (OPERATOR_HOST + the
// `op1.` signing keys + IDENTITY_OPERATOR_GRPC_ADDR, where OperatorService answers the per-request
// session check) and Redis (the per-IP sign-in limit, §01/10 #3). Declared even where OPERATOR_HOST
// is unset: "used ⇒ required" is derived from the declaration (ADR 0057), so staging/prod need
// IDENTITY_OPERATOR_GRPC_ADDR, REDIS_DSN and OPERATOR_SESSION_SIGNING_KEYS for platform before this
// image rolls out.
//
// SINCE 02/10/2026 (ADR 0069) platform takes its FIRST STAFF WRITES — the commune logo and web-admin
// banner — and so declares what every staff-facing file owner declares:
//
//	IdentityClient   IDENTITY_GRPC_ADDR: ResolveStaffPrincipal on identity's STAFF port 9090, the one
//	                 way a session cookie becomes a principal here (core/staffauth, as comms/petitions).
//	                 OperatorService stays on its own port (IDENTITY_OPERATOR_GRPC_ADDR, OperatorEdge)
//	ObjectStore      OBJECT_STORAGE_*: the private/temp/public buckets (ADR 0052)
//	PublicMedia      OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL: the published logo/banner URLs, also on
//	                 GetTenantProfile (logo_public_url, web_admin_banner_public_url)
//	MalwareScan      MALWARE_SCANNER_ADDRESS: every image is scanned before it is stored (ADR 0052 §9)
//
// Declared => required in staging and prod (ADR 0057): platform refuses to start there without them, and
// that is the point — a pod that started without the scanner would refuse every upload while green.
var configUses = config.Uses(
	config.HTTPServer,
	config.GRPCServer,
	config.TenantCache,
	config.Redis,
	config.OperatorEdge,
	config.IdentityClient,
	config.ObjectStore,
	config.PublicMedia,
	config.MalwareScan,
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
	cfg, err := config.Load("platform", configUses)
	if err != nil {
		return err
	}
	for _, canhBao := range cfg.CanhBao() {
		// Reported at EVERY startup, never once at deploy time: a flag switched on for a local
		// afternoon is forgotten by the next morning (rule 8, invariant 7).
		log.Warn("CẢNH BÁO CẤU HÌNH", "chi_tiet", canhBao)
	}

	// 2. store — one schema per service; never another service's schema (rule 2).
	// .Lo() is the ONE place the DSN leaves secret.DSN with its password intact: the driver
	// argument, and nowhere else. Anything else holding the raw string is a password waiting
	// for a log line (rule 8).
	db, err := sql.Open("pgx", cfg.DatabaseDSN.Lo())
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

	// 2b. schema migrations, BEFORE anything is served.
	//
	// WHY THE SERVICE MUST NOT START WHEN THIS FAILS: the registry queries below are written
	// against a schema this process assumes is there. A missing migration does not fail at
	// startup where somebody is watching — it fails on the first Host resolution, and every
	// commune then looks unknown for a reason the error message never names.
	//
	// The files are EMBEDDED in this binary (platform/migrations), so what is applied is
	// what was compiled — not whatever happens to be on the container's disk.
	ctxMig, huyMig := context.WithTimeout(context.Background(), 5*time.Minute)
	kqMig, err := migrate.Chay(ctxMig, db, migrations.FS, "platform")
	huyMig()
	if err != nil {
		return fmt.Errorf("platform: migration không chạy được: %w", err)
	}
	// Logged even when nothing was applied: "applied 0 files" at startup is how an operator finds
	// out the replica is already at the schema they expected, without opening a psql prompt.
	log.Info("migration xong", "service", "platform", "da_ap", kqMig.DaAp, "bo_qua", len(kqMig.BoQua))

	// 3. directory — resolves Host -> commune, cached with a short TTL because this sits on the
	//    path of EVERY request at 200+ communes (ADR 0004, decision 5).
	// The cache is pkg/tenant.CachedDirectory, not a platform-local type: it is a decorator over
	// tenant.Directory with no platform logic in it, and the other seven services wrap their
	// gRPC-backed directory with the same one. Two copies would be two invalidation rules.
	danhBa := svcstore.NewDirectory(db, cfg.OperatorHost())
	directory := tenant.NewCachedDirectory(danhBa, cfg.TenantCacheTTL())

	// 3b. OPERATOR_HOST must be no commune's host — refused before anything is served (operator_edge.go).
	ctxHost, huyHost := context.WithTimeout(context.Background(), 15*time.Second)
	err = refuseOperatorHostCollision(ctxHost, danhBa, cfg.OperatorHost())
	huyHost()
	if err != nil {
		return err
	}

	// 4. staff authentication — session cookie -> staff principal, over gRPC to identity's STAFF port
	//    (ResolveStaffPrincipal), exactly as comms and petitions do. This service owns no session registry
	//    and may not import identity's (rule 2, forbidden #1). Checker is core/staffauth.Checker: it decides
	//    from the permission set the middleware obtained for THIS request, in THIS commune.
	dinhDanh, err := identityclient.Dial(cfg.IdentityGRPCAddr(), cfg.GRPCCallerKey(), log)
	if err != nil {
		return fmt.Errorf("platform: không nối được dịch vụ định danh: %w", err)
	}
	defer dinhDanh.Close()

	// Idempotency store for the two upload-request routes (idem.Required(MoKhiHong)). REDIS_DSN is
	// required in staging/prod (config.Redis is declared); in DEV without it each route behaves per its
	// CheDoHong. Declared as the INTERFACE and left nil: a nil *idem.RedisStore inside it would be a
	// non-nil interface and idem would call methods on it.
	var idemStore idem.Store
	if cfg.RedisDSN() != "" {
		r, err := idem.NewRedisStore(cfg.RedisDSN().Lo())
		if err != nil {
			return fmt.Errorf("platform: không mở được Redis cho chống trùng thao tác: %w", err)
		}
		defer r.Close()
		idemStore = r
	}

	// The commune's identity images (ADR 0069). Object storage and the scanner are declared, so in
	// staging/prod config.Load already refused to start without them; in DEV they may be absent and the
	// upload routes then answer 503 while everything else serves. DECLARED AS THE INTERFACES AND LEFT nil
	// WHEN ABSENT, for the same nil-interface reason as idemStore.
	kho := store.New(db)
	hoSo := svcstore.NewHoSoHienThiStore(kho)
	var objects svcapp.BrandingObjectStore
	switch c, err := storage.New(cfg.ObjectStorage()); {
	case err == nil:
		objects = c
	case errors.Is(err, storage.ErrNotConfigured):
		log.Warn("CẢNH BÁO: chưa cấu hình kho lưu tệp — logo và banner của xã bị từ chối, URL ảnh trả rỗng",
			"service", "platform", "err", err)
	default:
		return fmt.Errorf("platform: cấu hình kho lưu tệp không hợp lệ: %w", err)
	}
	var scanner svcapp.MalwareScanner
	switch sc, err := malwarescan.New(cfg.MalwareScanner()); {
	case err == nil:
		scanner = sc
	case errors.Is(err, malwarescan.ErrNotConfigured):
		log.Warn("CẢNH BÁO: chưa cấu hình máy quét mã độc — logo và banner của xã bị từ chối",
			"service", "platform", "err", err)
	default:
		return fmt.Errorf("platform: cấu hình máy quét mã độc không hợp lệ: %w", err)
	}
	// The upload limits are THIS service's own table (migrations 0008, 0016). They are read through the
	// SAME core/platformclient/uploadpolicy reader every other file owner uses — same narrowing to the
	// storage allow-list, same fail-closed answers, same 60 s cache — fed in-process by this service's
	// own ListUploadPolicies handler instead of a network hop to itself (localUploadPolicies).
	policyReader := svcstore.NewUploadPolicyStore(db)
	fields := svcstore.NewPetitionFieldStore(db)
	policies := uploadpolicy.New(localUploadPolicies{srv: svcgrpc.NewServer(svcgrpc.Deps{
		Dir: danhBa, Apps: danhBa, HoSo: hoSo, Policies: policyReader, Fields: fields,
	}, log)}, log)
	branding := svcapp.NewBranding(kho, svcstore.NewStoredFileStore(kho), hoSo, objects, scanner, policies, log)

	// 5. consumers — event handlers; each refuses a message with no commune.
	// TODO(next): no events published yet.

	mux := http.NewServeMux()

	svchttp.Register(mux, svchttp.Deps{
		Checker:  staffauth.Checker{},
		Branding: branding,
		URLs:     branding,
		Log:      log,
	})

	// The commune-host edge chain — buildCommuneEdge, below, says why each layer sits where it does.
	h := buildCommuneEdge(mux, directory, dinhDanh, idemStore, log)

	// The OPERATOR edge (operator_edge.go): its own chain for Host == OPERATOR_HOST, or nothing at
	// all when OPERATOR_HOST is unset. Its registry writes go through the SCOPED store, each one
	// audited in the target commune's audit_log; its reads use the registry's raw handle, like
	// the directory. Forget drops a written host from THIS process's cache at once.
	// The upload limits (policyReader, the same raw-handle store the gRPC read uses) and the operator
	// log are this service's own tables too (ADR 0073 #2, #5), as are the shared Mini App row and the
	// tier-1 petition field codes (fields, the same store the gRPC read uses — ADR 0073 #3).
	opEdge, err := wireOperatorEdge(cfg, svcstore.NewOperatorRegistry(db),
		svcstore.NewRegistryWriter(store.New(db)), policyReader, svcstore.NewOperatorLog(db),
		svcstore.NewSharedMiniAppStore(db), fields, directory.Forget, log)
	if err != nil {
		return err
	}
	defer opEdge.close()

	// /healthz is mounted on an OUTER mux, so it is genuinely outside the chain above.
	//
	// It used to sit on the inner mux while the comment beside it claimed the opposite. That is
	// the shape this whole service exists to prevent: an orchestrator probes by IP, the Host
	// matches no commune, TenantMiddleware answers 404 — and a healthy process is restarted
	// during a database blip, which is exactly when restarting it is worst.
	ngoai := buildOuter(h, opEdge.handler, cfg.OperatorHost())
	// KHÔNG gắn webhook của Zalo Mini App ở đây, và chỗ trống này là có chủ ý — ADR 0032.
	//
	// Nó TỪNG nằm đúng chỗ này, biện hộ bằng `domain-boundaries.md`: "platform = Nền tảng —
	// nhà cung cấp vận hành". Câu ấy nghĩa là nhà cung cấp vận hành NỀN TẢNG ViGov — sổ đăng
	// ký xã, vòng đời xã, siêu dữ liệu (ADR 0003) — chứ KHÔNG phải "mọi thứ thuộc nhà cung
	// cấp thì để vào đây". Đọc rộng ra như thế thì service này dần thành sọt đựng.
	//
	// Phép thử đã chốt: một bề mặt tích hợp Zalo thuộc hệ thống nào là do KHOÁ BÍ MẬT NÀO KÝ
	// NÓ quyết. Webhook của Mini App ký bằng app secret của bên đứng tên app, nên nó thuộc
	// kho `vihat-miniapp`. Còn ZNS gửi từ OA của TỪNG XÃ ký bằng khoá của xã, nên đường ấy Ở
	// LẠI ViGov, tại `service-comms` (ADR 0018 giữ nguyên) — đừng suy rộng thành "mọi thứ
	// dính chữ Zalo đều rời đi".

	srv := &http.Server{
		Addr: cfg.ListenAddr(),
		// OUTERMOST, around everything above, so every layer reads one client address per
		// request, crossing only the proxies TRUSTED_PROXY_CIDRS names.
		Handler:           httpx.ClientIPTuProxyTinCay(cfg.TrustedProxies())(ngoai),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// 6. gRPC — the inter-service surface, on its OWN port. gRPC needs HTTP/2 and the REST
	//    surface is served to browsers over HTTP/1.1; one listener for both means
	//    demultiplexing by protocol, and every proxy and health check in front of the process
	//    then has to understand both.
	//
	// CALLER AUTHENTICATION ON THE gRPC PORT — what is now in place, and what is still not.
	//
	//	IN PLACE (ADR 0025), two layers, and neither is sufficient alone:
	//	  1. a shared caller key on EVERY RPC, checked by grpcx.UnaryServerCallerAuth below.
	//	     One key for the whole deployment, from GRPC_CALLER_KEY, sourced from a k8s secret
	//	  2. this port confined to the cluster's internal network — never published, never
	//	     bound to 0.0.0.0 on a public host. Layer 1 stops a process that reached the port;
	//	     layer 2 is what stops it reaching the port. Removing either removes half.
	//
	//	STILL NOT IN PLACE, and the shared key does NOT deliver it: THE CALLER'S IDENTITY
	//	RECORDED ON THE CALL, so a cross-service read is attributable. One key authenticates
	//	that the caller holds the key — never WHICH service it is. So an audit entry cannot
	//	name a caller on this boundary (rule 6, invariant 2 wants a "who"), and x-tenant-id in
	//	metadata remains a claim by the caller rather than evidence. Per-service identity —
	//	mTLS or a mesh — is what closes this, and it is still required before a real
	//	deployment. The full list of what the current shape does not buy is in core/grpcx's
	//	package doc; the decision and its cost are in ADR 0025.
	//
	//	Why the exposure was tolerable even BEFORE layer 1, and why that argument does not
	//	transfer: ADR 0003 means no business data crosses this boundary at all. ResolveHost
	//	returns a ULID that is already public — it travels in the QR deep link every citizen
	//	scans (ADR 0005) — and GetTenant returns registry metadata: name, host, active. Both
	//	answer about ONE commune the caller already names.
	//
	//	THAT STOPS BEING TRUE THE DAY ListTenants IS IMPLEMENTED. It answers about every
	//	commune at once, so one call is the whole registry rather than one row of it — and the
	//	ULIDs in it are the prefix of every cache key, queue, realtime room and file path in
	//	the system (rule 1, invariant 7). That is why the RPC is declared in the contract but
	//	NOT on core/grpcx.methodsWithoutTenant, and layer 1 arriving does not by itself add it:
	//	the exposure moves from "any process that can open a TCP connection" to "any holder of
	//	the one shared key", which is every service in the cluster. Adding that name is a stop
	//	condition of its own (ADR 0012, decision 1) — ask, do not infer.
	//
	// The UNCACHED directory on purpose: the gRPC paths use ByHostErr / ByID, which keep
	// "no such commune" and "the database is down" apart, and CachedDirectory implements only
	// tenant.Directory, whose bool answer collapses the two. Caching this path needs a cache
	// that preserves the distinction — TODO(next), it matters once every service edge resolves
	// its Host through here (ADR 0004, decision 5).
	//
	// The display profile is a commune's own content, so it gets the SCOPED store (core/store),
	// never the directory's raw handle.
	//
	// Upload limits are platform-wide configuration with no commune column (migration 0008), so
	// they get their own raw-handle reader — not the directory, which reads only the registry. The
	// tier-1 petition field codes (migration 0011, ADR 0060) are the same kind of table and get the
	// same kind of reader.
	//
	// URLs builds logo_public_url / web_admin_banner_public_url from OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL
	// (the branding use case; "" when object storage is not configured — never a guessed URL).
	grpcSrv := dungGRPCServer(cfg.GRPCCallerKey(), svcgrpc.Deps{
		Dir:      danhBa,
		Apps:     danhBa,
		HoSo:     hoSo,
		Policies: policyReader,
		Fields:   fields,
		URLs:     branding,
	}, log)

	grpcLis, err := net.Listen("tcp", cfg.GRPCListenAddr())
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
		log.Info("khởi động", "service", "platform", "addr", cfg.ListenAddr(),
			"env", cfg.Env, "dsn", cfg.Redacted().DatabaseDSN)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			loi <- err
		}
	}()
	go func() {
		log.Info("khởi động gRPC", "service", "platform", "addr", cfg.GRPCListenAddr())
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

// dungGRPCServer builds the inter-service gRPC surface with its COMPLETE interceptor chain.
//
// IT IS A NAMED FUNCTION AND NOT AN EXPRESSION INSIDE run() FOR ONE REASON: so a test can start
// it. core/grpcx proves the interceptors refuse what they should, but nothing in core/grpcx can
// see whether THIS binary installs them — and an interceptor deleted from a chain leaves a
// server that starts, serves, and answers every unauthenticated call. main_test.go starts this
// function over a real connection, so that deletion turns something red.
//
// CHAINED, AND THE ORDER IS NOT NEGOTIABLE. Caller authentication runs FIRST: an unauthenticated
// caller must not reach the commune logic at all, or the errors it gets back begin describing
// what the server was expecting next.
//
// TWO INTERCEPTORS BECAUSE THERE ARE TWO QUESTIONS, and they are orthogonal. "Who is calling"
// applies to every RPC with no exemption at all; "which commune" is exempt for ResolveHost,
// because that call is what ESTABLISHES a commune. Folding either into the other is how the
// tenant exemption list quietly becomes a list of RPCs that skip authentication.
func dungGRPCServer(khoaGoi secret.Secret, dv svcgrpc.Deps, log *slog.Logger) *grpc.Server {
	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			// Panics here, at construction, when GRPC_CALLER_KEY is empty. A server that starts
			// without the key accepts every call it is supposed to refuse, and nothing looks
			// wrong — no error, no failed request, no metric moving.
			grpcx.UnaryServerCallerAuth(khoaGoi, log),
			// The commune is lifted out of metadata into context here, once, before any handler.
			// A call to a non-exempt RPC with no commune is refused with InvalidArgument — never
			// defaulted (rule 1, forbidden #1).
			grpcx.UnaryServerInterceptor(),
		),
	)
	platformv1.RegisterPlatformServiceServer(srv, svcgrpc.NewServer(dv, log))
	return srv
}

// buildCommuneEdge builds the COMMUNE-HOST chain. A NAMED FUNCTION SO A TEST CAN DRIVE THE REAL CHAIN:
// core/staffauth proves what its middleware does, but only a test of THIS function sees whether this
// binary installs it — and without it every staff request answers 401, with nothing else turning red.
//
// ORDER MATTERS AND IS NOT NEGOTIABLE, outermost first:
//
//	StripTenantHeaders  a client naming its own commune is a client granting itself access
//	Recover             turns tenant.MustFrom's deliberate panic into a traceable 500
//	TenantMiddleware    resolves Host -> commune; unknown Host returns 404, never a default
//	idem.Middleware     INSIDE TenantMiddleware: the idempotency key is prefixed with the commune
//	                    (rule 1, invariant 7) — outside it two communes would share one key space
//	staffauth           rebuilds authz.Principal by asking identity; INSIDE TenantMiddleware, because
//	                    the outgoing call carries the commune from the context and identity compares
//	                    it with the commune inside the session (rule 1, invariant 8)
//
// Recover sits OUTSIDE TenantMiddleware so a panic raised while resolving the commune is still caught;
// it sits INSIDE StripTenantHeaders because stripping cannot panic.
func buildCommuneEdge(mux http.Handler, dir tenant.Directory, staff staffauth.Resolver, idemStore idem.Store,
	log *slog.Logger) http.Handler {

	h := mux
	h = staffauth.Middleware(staff, log)(h)
	h = idem.Middleware(idemStore, log)(h)
	h = httpx.TenantMiddleware(dir)(h)
	h = httpx.Recover(traceID)(h)
	h = httpx.StripTenantHeaders(h)
	return h
}

// localUploadPolicies feeds uploadpolicy.Reader from this service's OWN ListUploadPolicies handler,
// in-process. The reader's contract is the gRPC client's; the handler's answer is exactly what every
// other service receives over the wire, so the narrowing and the fail-closed rules are one code path.
// The handler reads no commune (upload_policy.go), and the reader requires one in ctx — the HTTP edge
// has resolved it from Host before any upload route runs.
type localUploadPolicies struct{ srv *svcgrpc.Server }

func (l localUploadPolicies) ListUploadPolicies(ctx context.Context, in *platformv1.ListUploadPoliciesRequest,
	_ ...grpc.CallOption) (*platformv1.ListUploadPoliciesResponse, error) {
	return l.srv.ListUploadPolicies(ctx, in)
}

// traceID returns the id a caller can quote when reporting a problem.
//
// TODO(next): lift this from the incoming request header once the reverse proxy sets one, so
// a single citizen complaint can be followed across services.
func traceID(context.Context) string { return "" }
