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
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"google.golang.org/grpc"

	"github.com/vihat/vigov/core/config"
	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/grpcx"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/migrate"
	"github.com/vihat/vigov/core/platformclient"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/token"
	"github.com/vihat/vigov/service-identity/internal/app"
	svcgrpc "github.com/vihat/vigov/service-identity/internal/grpc"
	svchttp "github.com/vihat/vigov/service-identity/internal/http"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
	"github.com/vihat/vigov/service-identity/migrations"
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
	// The files are EMBEDDED in this binary (identity/migrations), so what is applied is
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
	// The caller key goes with the address: the port answers nothing without it (ADR 0025).
	// An empty key panics inside the client interceptor, at construction — before this service
	// can start making calls that would all be refused.
	nenTang, err := platformclient.Dial(cfg.PlatformGRPCAddr, cfg.GRPCCallerKey, log)
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
	vaiTro := idstore.NewVaiTroStore(kho)
	boPhan := idstore.NewBoPhanStore(kho)
	// A store of its own and NOT the Checker, although both read `vai_tro_quyen`: Checker decides
	// access one key at a time for the caller, this one describes every role's grants for one
	// administration screen. See MaTranQuyenDoc for why the three grant readers stay apart.
	maTranQuyen := idstore.NewQuyenStore(kho)
	// The three reference reads of migration 0005 (ADR 0024). Three stores and not one, mirroring
	// the three narrow interfaces in internal/http: a store per table is what lets the panic in
	// Register name the route that would have failed, and it keeps the residential-unit read —
	// which joins — apart from the two catalogues, which do not.
	thonToDanPho := idstore.NewThonToDanPhoStore(kho)
	loaiDonViDanCu := idstore.NewLoaiDonViDanCuStore(kho)
	khoiNhiemVu := idstore.NewKhoiNhiemVuStore(kho)
	// The commune's working calendar (migration 0006): the ordinary week, the closures, the swap
	// days. Three stores, three tables, mirroring the three narrow interfaces in internal/http.
	//
	// Read only: GET /api/v1/working-hours, /api/v1/public-holidays, /api/v1/swap-working-days.
	// There is no write path — who may edit a commune's calendar has not been asked, and a
	// working calendar is the basis of an issued commitment (migration 0006:41).
	lichLamViec := idstore.NewLichLamViecStore(kho)
	ngayNghiLe := idstore.NewNgayNghiLeStore(kho)
	ngayLamBu := idstore.NewNgayLamBuStore(kho)
	// The commune's processing deadlines in working hours (migration 0008, ADR 0029) — the other
	// half of the three tables above. Read only, and NOT mounted on any HTTP route: who may edit a
	// commune's SLA is `admin.sla`, but whether changing it needs a leader's approval is ADR 0029's
	// stop condition #4 and nobody has asked the customer. It leaves this service through exactly
	// one door, the gRPC RPC ResolveDeadlines.
	sla := idstore.NewSLAStore(kho)
	// The CITIZEN session registry (migration 0004) — the only store here built on the RAW *sql.DB
	// rather than on `kho`, and the exemption is argued in full at NewPhienCongDanStore: this
	// lookup is what ESTABLISHES the commune, so there is no commune with which to scope it
	// (ADR 0022). It is read by exactly one thing, the gRPC RPC ResolveCitizenSession — no HTTP
	// route of this service touches it, because who may open or revoke a citizen session on an
	// HTTP route is a question nobody has asked.
	phienCongDan := idstore.NewPhienCongDanStore(db, log)

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
	// The WRITE surface of the staff register (open questions #10, #13, #14, #15, #16, all decided
	// 2026-09-22). It is given the SAME *idstore.CanBoStore the two read fields below carry — one
	// store, because the guards read the very rows they then write, inside one transaction.
	//
	// IT IS A USE CASE AND NOT A STORE ON Deps, and that is the whole reason this line exists here
	// rather than reusing `canBo` directly: every method opens the transaction that the business
	// write and its audit entry share (rule 6, invariant 3).
	ghiDanhBa := app.NewDanhBaCanBo(kho, canBo)

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
		Quyen: checker,
		// A store of its own, not the Checker: Checker reads GRANTS and decides access, VaiTroStore
		// reads what the role is CALLED and decides nothing. Putting a display read behind the
		// interface that guards every route is how the first caller comes to decide access from a
		// description (rule 5, forbidden #3).
		VaiTro: vaiTro,
		BoPhan: boPhan,
		// Cùng một *VaiTroStore, hai trường: một trả lời "vai trò của người gọi", một trả
		// lời "xã này có những vai trò nào". Hai câu hỏi, hai interface hẹp.
		VaiTroMuc: vaiTro,
		// Ma trận phân quyền — chỉ ĐỌC. Không có tuyến ghi nào, và lý do nằm ở đầu tệp
		// internal/http/quyen.go: lưu một cột vai trò chạm đúng câu hỏi mở #13 và #14.
		MaTran: maTranQuyen,
		// Ba tuyến đọc tham chiếu của migration 0005 — CHỈ ĐỌC. Không có tuyến ghi nào: câu hỏi
		// mở #21 (xã được sửa DANH SÁCH MÃ hay chỉ nhãn và thứ tự) chưa có lời đáp.
		ThonToDanPho:   thonToDanPho,
		LoaiDonViDanCu: loaiDonViDanCu,
		KhoiNhiemVu:    khoiNhiemVu,
		// Lịch làm việc của xã (migration 0006) — CHỈ ĐỌC, và chưa có tuyến nào được gắn: tên tài
		// nguyên URL của ba khái niệm này chưa có dòng trong bảng ánh xạ, nên đang HỎI chứ không
		// tự dịch (ADR 0011). Ai sửa được lịch của xã cũng chưa ai hỏi.
		LichLamViec: lichLamViec,
		NgayNghiLe:  ngayNghiLe,
		NgayLamBu:   ngayLamBu,
		Signer:      signer, // the SAME pointer app.NewDangNhap was given above
		Phien:       phien,
		CanBo:       canBo,
		// The SAME store behind two fields, and two fields on purpose: CanBoDoc is the
		// three-condition read the session middleware runs on every request, CanBoDanhBa is the
		// register the Cấu hình → Người dùng screen pages through. See the note on CanBoDanhBa.
		DanhBa: canBo,
		// The five WRITE routes of the register. A use case, not the store: see the note where it is
		// built. Register panics without it, so an unwired write surface fails at startup rather
		// than at the first administrator who tries to add a member of staff.
		GhiDanhBa: ghiDanhBa,
		DangNhap:  dangNhap,
		DangXuat:  dangXuat,
		Log:       log,
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

	// 10. gRPC — the inter-service surface, on its OWN port. gRPC needs HTTP/2 and the REST
	//     surface is served to browsers over HTTP/1.1; one listener for both means demultiplexing
	//     by protocol, and every proxy and health check in front of the process then has to
	//     understand both. Same shape as service-platform.
	//
	// WHAT THIS PORT NOW CARRIES, AND WHY IT IS NOT THE SAME EXPOSURE AS PLATFORM'S. ADR 0003
	// keeps the platform contract to registry metadata — a ULID already printed on a QR code, a
	// commune's display name. THIS contract carries neither of those things and is not covered by
	// that argument: ResolveStaffPrincipal accepts a live session credential and answers with a
	// person's whole grant set inside one commune. It is the first RPC in this system to read
	// anything a caller could act on, which is precisely the stop condition ADR 0012 decision 3
	// named — and it is already answered: the user decided on 2026-09-20, and ADR 0025 is the
	// decision. The two layers that decision rests on are:
	//
	//	1. the shared caller key on EVERY RPC, checked by grpcx.UnaryServerCallerAuth below;
	//	2. this port confined to the cluster's internal network — never published, never bound to
	//	   a public address. Layer 1 stops a process that reached the port; layer 2 is what stops
	//	   it reaching the port. Removing either removes half.
	//
	// STILL OWED, and the shared key does not deliver it: the CALLER'S IDENTITY on the call. One
	// key proves the caller is inside the deployment, never WHICH service it is — so an audit
	// entry written on this boundary could not name a "who" (rule 6, invariant 2), and
	// "x-tenant-id" remains a claim by the caller rather than evidence. What makes that claim
	// harmless for ResolveStaffPrincipal specifically is the comparison inside the handler: a
	// credential issued for commune A yields nothing at all when the metadata names commune B,
	// and nothing here mints credentials on a caller's say-so. Per-service identity (mTLS or a
	// mesh) is what closes the rest, and it is required before a real deployment.
	grpcSrv := dungGRPCServer(cfg.GRPCCallerKey, svcgrpc.Deps{
		// The SAME signer the HTTP side and app.DangNhap were given — see step 5. A second signer
		// here would verify tokens with a key that did not sign them, and every staff request in
		// the four calling services would come back with no principal while identity's own routes
		// kept working. That failure names nothing in any log.
		Signer: signer,
		Phien:  phien,
		// CanBo is the THREE-condition read (not deleted, has an account, not locked); Lo is the
		// register read, which filters only `deleted_at IS NULL`. The same *CanBoStore behind two
		// fields, two fields on purpose — identical to the CanBo/DanhBa split above, and for the
		// identical reason: merging them would put the register's looser predicate one careless
		// edit away from the authentication path.
		CanBo: canBo,
		Lo:    canBo,
		// Ten is the name read behind ResolveStaffNames — the THIRD predicate on the same
		// *CanBoStore, and the only one that does NOT filter `deleted_at`. Three fields on purpose,
		// for the same reason CanBo and Lo are two: a record removed from the directory must be
		// readable when an archival record names it (ADR 0034), and must stay invisible to the two
		// paths above, where "this person exists today" is what is being asked.
		Ten: canBo,
		// The SAME *idstore.Checker that guards every route, through its QuyenCua method. One
		// grant predicate for the guard and for the principal: a second one would drift, and
		// drift in either direction is a defect with no error attached.
		Quyen: checker,
		// The citizen session registry, for ResolveCitizenSession — the only way a citizen session
		// leaves this service, and the only thing that lets any OTHER service mount a citizen edge
		// at all (core/httpx.CitizenEdge needs a core/httpx.CitizenSessions, whose one
		// implementation is the store behind this field, inside this service's `internal/`).
		//
		// ⚠ WIRED BUT NOT YET REACHABLE. The RPC is absent from core/grpcx.methodsWithoutTenant, so
		// grpcx.UnaryServerInterceptor below refuses every call to it with InvalidArgument before
		// the handler runs. It cannot be on that list yet: adding a second name there is a STOP
		// CONDITION for the user (ADR 0012, decision 1), and it is not a decision to take while
		// wiring a binary. It is wired anyway so that the answer, when it comes, is one line in
		// core/grpcx and nothing else — not a second day's work discovering this field is missing.
		PhienCongDan: phienCongDan,
		// The commune's working calendar, for AdvanceWorkingHours — the only way that calendar
		// leaves this service. The SAME three read-only stores the HTTP routes were given above:
		// one read path per table, so a deadline is computed from exactly what the configuration
		// screen shows. All three are required; NewServer refuses to build without any of them,
		// because a deadline counted without the holidays is not a shorter answer, it is a wrong
		// one.
		Lich:   lichLamViec,
		NghiLe: ngayNghiLe,
		LamBu:  ngayLamBu,
		// The commune's SLA table, for ResolveDeadlines — the RPC that unblocks every write route
		// of `petitions` and the deadline path of `documents` (ADR 0029 §Hệ quả ngay). The same
		// read-only store, and there is no second reader: a deadline must come from ONE place, and
		// this field plus the three above are that place.
		SLA: sla,
		Log: log,
	}, log)

	grpcLis, err := net.Listen("tcp", cfg.GRPCListenAddr)
	if err != nil {
		return fmt.Errorf("identity: không mở được cổng gRPC %q: %w", cfg.GRPCListenAddr, err)
	}

	// 11. Graceful shutdown. A sign-in cut in half by a deploy would leave a session row whose
	//     audit entry says a person signed in while no cookie was ever issued — a state the
	//     retention rules do not permit (rule 2, invariant 6).
	dungLai := make(chan os.Signal, 1)
	signal.Notify(dungLai, os.Interrupt, syscall.SIGTERM)

	// Buffered for TWO now, not one: either server may fail, and an unbuffered send from a
	// goroutine nobody is reading any more would leak it.
	loi := make(chan error, 2)
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
	go func() {
		log.Info("khởi động gRPC", "service", "identity", "addr", cfg.GRPCListenAddr)
		// Serve returns nil after GracefulStop, so there is no ErrServerClosed equivalent to
		// filter out here.
		if err := grpcSrv.Serve(grpcLis); err != nil {
			loi <- err
		}
	}()

	select {
	case err := <-loi:
		// One surface failing takes the whole process down rather than leaving it half-serving.
		// An identity answering its own HTTP routes but not ResolveStaffPrincipal looks healthy
		// while every guarded route in four other services answers 401 to valid sessions — which
		// is exactly the outage this server was built to end.
		grpcSrv.Stop()
		_ = srv.Close()
		return err

	case <-dungLai:
		log.Info("nhận tín hiệu dừng, đang đóng kết nối")
		ctx, huy := context.WithTimeout(context.Background(), 20*time.Second)
		defer huy()

		// Both surfaces drain, and neither waits for the other.
		xongGRPC := make(chan struct{})
		go func() {
			grpcSrv.GracefulStop()
			close(xongGRPC)
		}()

		errHTTP := srv.Shutdown(ctx)

		select {
		case <-xongGRPC:
		case <-ctx.Done():
			// A call that will not finish must not hold a deploy open indefinitely. Forcing the
			// stop here is visible in the logs; hanging is not.
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
// see whether THIS binary installs them — an interceptor deleted from the chain leaves a server
// that starts, serves, and answers every unauthenticated call on a port that now carries session
// credentials and grant sets. main_test.go starts this function over a real connection, so that
// deletion turns something red.
//
// CHAINED, AND THE ORDER IS NOT NEGOTIABLE. Caller authentication runs FIRST: an unauthenticated
// caller must not reach the commune logic at all, or the errors it gets back begin describing
// what the server was expecting next.
//
// TWO INTERCEPTORS BECAUSE THERE ARE TWO QUESTIONS, AND THEY ARE ORTHOGONAL — this is the part
// most likely to be got wrong, and getting it wrong is silent. "Who is calling" applies to every
// RPC with NO exemption at all. "Which commune" has an exemption list, and NEITHER RPC OF THIS
// SERVICE IS ON IT. An RPC excused from carrying a COMMUNE is never thereby excused from proving
// the CALLER holds the key; folding either interceptor into the other is how the tenant exemption
// list quietly becomes a list of RPCs that skip authentication.
func dungGRPCServer(khoaGoi secret.Secret, d svcgrpc.Deps, log *slog.Logger) *grpc.Server {
	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			// Panics here, at construction, when GRPC_CALLER_KEY is empty. A server that starts
			// without the key accepts every call it is supposed to refuse, and nothing looks
			// wrong — no error, no failed request, no metric moving. config.Load already requires
			// the variable, so a process that starts is a process that authenticates; this is the
			// second lock on the same door, for the day somebody constructs a server elsewhere.
			grpcx.UnaryServerCallerAuth(khoaGoi, log),
			// The commune is lifted out of metadata into context here, once, before any handler,
			// so every handler reads it exactly as an HTTP handler does. A call to a non-exempt
			// RPC with no commune is refused with InvalidArgument — never defaulted (rule 1,
			// forbidden #1).
			grpcx.UnaryServerInterceptor(),
		),
	)
	// NewServer panics on any missing collaborator, at construction, for the same reason
	// identity/http.Register does: incomplete wiring must fail where a human is watching a
	// process fail to start, not at request time in four other services.
	identityv1.RegisterIdentityServiceServer(srv, svcgrpc.NewServer(d))
	return srv
}

// traceID returns the id a caller can quote when reporting a problem.
//
// TODO(next): lift this from the incoming request header once the reverse proxy sets one, so a
// single citizen complaint can be followed across services.
func traceID(context.Context) string { return "" }
