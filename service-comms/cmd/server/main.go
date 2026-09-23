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
	"net/http"
	"os"
	"os/signal"
	"syscall"
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
	commsapp "github.com/vihat/vigov/service-comms/internal/app"
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
	//   5. consumers   the HANDLER now exists — internal/event.PhieuDoiTrangThai, which consumes
	//                  `petitions.status_changed.v1` and writes the citizen notification ledger.
	//                  TWO THINGS ARE MISSING BEFORE IT CAN BE WIRED HERE, and both are decisions
	//                  rather than code: (a) a Kafka client — ADR 0010 puts inter-service events on
	//                  Kafka, but `core/events.Publisher` is still a bare interface with no
	//                  implementation, and a broker client nobody has decided does not belong in a
	//                  wiring file; (b) an implementation of event.MauTinXa — which APPROVED ZNS
	//                  template this commune sends for a transition is per-commune configuration
	//                  (ADR 0018, consequence 1) with no store, no adapter and no customer answer
	//                  yet. A constant here would be one commune's template id serving 200+.
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
			log.Error("không mở được Redis cho chống trùng thao tác", "service", "comms", "err", err)
			os.Exit(1)
		}
		idemStore = r
	}

	loaiTaiNguyen := commsstore.NewLoaiTaiNguyenBanDoStore(kho)

	// The internal announcement book (migration 0005). ONE store behind both routes: the read is a
	// query, the write goes through the use case that owns the transaction its audit entry shares.
	thongBao := commsstore.NewThongBaoNoiBoStore(kho)

	mux := http.NewServeMux()
	svchttp.Register(mux, svchttp.Deps{
		Checker:       staffauth.Checker{},
		LoaiTaiNguyen: loaiTaiNguyen,
		ThongBao:      thongBao,
		GhiThongBao:   commsapp.NewSoanThongBaoNoiBo(kho, thongBao),
		// The write use case owns the transaction the business write and its audit entry share
		// (rule 6, invariant 3). It is given *store.DB rather than a transaction because opening one
		// is precisely what it is for.
		GhiLoaiTaiNguyen: commsapp.NewDanhMucLoaiTaiNguyen(kho, loaiTaiNguyen),
		Log:              log,
	})

	// Rule 11, invariant 1: the environment is read in core/config and nowhere else.
	// The default is this service's own — see config.ListenAddrHoac for why it lives here.
	addr := cfg.ListenAddrHoac(":8087")
	log.Info("starting", "service", "comms", "addr", addr)
	srv := &http.Server{
		Addr:              addr,
		Handler:           dungBien(mux, directory, dinhDanh, idemStore, log),
		ReadHeaderTimeout: 10 * time.Second,
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
	dungLai := make(chan os.Signal, 1)
	signal.Notify(dungLai, os.Interrupt, syscall.SIGTERM)

	// Có đệm: `ListenAndServe` hỏng sau khi đã có ai đọc kênh là một goroutine rò lại mãi mãi.
	loi := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			loi <- err
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
	case err := <-loi:
		log.Error("server stopped", "err", err)
		os.Exit(1)
	case <-dungLai:
		log.Info("nhận tín hiệu dừng, đang đóng kết nối", "service", "comms")
		ctx, huy := context.WithTimeout(context.Background(), 20*time.Second)
		defer huy()
		if err := srv.Shutdown(ctx); err != nil {
			log.Error("đóng không sạch", "err", err)
			os.Exit(1)
		}
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
