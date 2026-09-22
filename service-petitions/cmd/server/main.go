package main

// petitions service — Tiếp dân.
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

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/config"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
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

	// 3b. The CITIZEN session registry, over the SAME connection to identity.
	//
	// WHY IT CAN EXIST AT ALL, since it could not until 2026-09-21: httpx.CitizenEdge needs an
	// httpx.CitizenSessions, whose only implementation is a store inside service-identity's
	// `internal/` that rule 2, forbidden #1 forbids this service from importing. core/identityclient
	// is the sanctioned path, and the RPC behind it is exempt from carrying a commune
	// (core/grpcx.methodsWithoutTenant) because it is the call that RESOLVES the commune — the user
	// answered that stop condition, ADR 0012 decision 1.
	//
	// THE SAME *Client AS THE STAFF PATH, ON PURPOSE. A second dial would be a second connection
	// with its own pool and its own view of identity's health, and the two would disagree about
	// whether identity is reachable at the exact moment that matters.
	soPhien := identityclient.NewSoPhienCongDan(dinhDanh)

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
			return err
		}
		defer r.Close()
		idemStore = r
	}

	loaiNhiemVu := petstore.NewLoaiNhiemVuStore(kho)
	mucUuTien := petstore.NewMucUuTienNhiemVuStore(kho)

	// ONE PhieuPhanAnhStore FOR EVERY READER AND WRITER OF THE REGISTER, and that is a change from
	// the three separate constructions this file used to do. A store holds no state beyond the pool,
	// so several were harmless — but they invited the reading that the citizen path and the staff
	// path talk to different things, which is exactly the confusion rule 4, invariant 5 is about.
	// They do not: ONE table, and what separates the two surfaces is the mux, the Deps type and the
	// method each is given.
	phieu := petstore.NewPhieuPhanAnhStore(kho)

	// The outbox. It is a SEPARATE store from the register because the two hold different kinds of
	// thing — one archival record, one piece of infrastructure state a relay will drain — and the
	// use case takes both so it can write the change and the notification obligation in ONE
	// transaction (rule 10, invariant 5).
	//
	// ⚠ NOTHING DRAINS IT YET. There is no broker client in this repository; `core/events.Publisher`
	// is a bare interface with no implementation. Rows accumulate with `gui_luc` NULL and no citizen
	// is messaged. That is a recorded, recoverable backlog rather than a silent loss — see the table
	// comment in migration 0005 — and it is reported as an open gap.
	suKien := petstore.NewSuKienDiStore(kho)

	mux := http.NewServeMux()
	svchttp.Register(mux, svchttp.Deps{
		// Deps.Checker is staffauth.Checker: it decides from the permission set the middleware
		// obtained for THIS request and holds no state of its own — not a stand-in that answers
		// questions nobody asked it. It is now load-bearing: GET /api/v1/citizen-reports/{maTraCuu}
		// declares authz.RequirePermission("feedback.read"), which is the first route in this
		// service to guard a real operation.
		Checker:     staffauth.Checker{},
		LoaiNhiemVu: loaiNhiemVu,
		MucUuTien:   mucUuTien,
		// The write use cases own the transaction the business write and its audit entry share
		// (rule 6, invariant 3). Each is given *store.DB rather than a transaction because opening
		// one is precisely what it is for.
		GhiLoaiNhiemVu: app.NewDanhMucLoaiNhiemVu(kho, loaiNhiemVu),
		GhiMucUuTien:   app.NewDanhMucMucUuTien(kho, mucUuTien),
		Phieu:          phieu,
		NhanLinhVuc:    petstore.NewNhanLinhVucStore(kho),
		// The register list, and the four staff acts that finally make a petition processable. Each
		// act opens a transaction and writes the change, the audit entry and the notification
		// obligation inside it, which is why the use case takes *store.DB rather than a transaction.
		// `dinhDanh` is here because classification ASKS identity for the commune's resolve deadline —
		// the same client the intake path uses, so the two never disagree about identity's health.
		DanhSachPhieu: phieu,
		XuLyPhieu:     app.NewXuLyPhanAnh(kho, phieu, suKien, dinhDanh),
		// The trail for a full-view read of a reporter's name and number. It takes the same
		// *store.DB as the repositories because it opens its own transaction: rule 6, invariant 3
		// admits no audit write outside one, and audit.Write takes only a *store.ScopedTx.
		Vet: app.NewXemNguoiGui(kho),
		Log: log,
	})

	// THE CITIZEN SURFACE — ITS OWN MUX, and that is rule 4, invariant 5 made mechanical rather
	// than remembered. It is a second mux and not two more lines on `mux` above because the two
	// surfaces must run behind DIFFERENT edge chains: this one resolves the commune from the
	// citizen session, the one above from `Host`. See dungBien.
	//
	// IT IS GIVEN A DIFFERENT Deps TYPE, carrying the identity-filtered read and nothing else, so
	// a citizen route cannot reach the unfiltered staff read even by typing it.
	muxCongDan := http.NewServeMux()
	svchttp.RegisterCongDan(muxCongDan, svchttp.DepsCongDan{
		Phieu: phieu,
		// THE CITIZEN INTAKE. It is given *store.DB rather than a transaction because opening one is
		// precisely what it is for (rule 6, invariant 3), and `dinhDanh` because the acknowledge
		// deadline is read from the commune's own table by the ONE service that owns the working-hours
		// calendar (ADR 0007, ADR 0029). THE SAME *identityclient.Client the staff path uses: a second
		// dial would be a second connection with its own view of identity's health, and the two would
		// disagree at the exact moment that matters.
		GuiPhieu: app.NewGuiPhanAnh(kho, phieu, dinhDanh),
		// THE SAME label catalogue the staff routes read. Sharing is right here and only here: the
		// commune's wording for a field code is its public vocabulary, and two readers of one
		// catalogue are two things to keep in step.
		NhanLinhVuc: petstore.NewNhanLinhVucStore(kho),
		Log:         log,
	})

	// Rule 11, invariant 1: the environment is read in core/config and nowhere else.
	// The default is this service's own — see config.ListenAddrHoac for why it lives here.
	addr := cfg.ListenAddrHoac(":8084")
	log.Info("starting", "service", "petitions", "addr", addr)
	srv := &http.Server{
		Addr:              addr,
		Handler:           dungBien(mux, muxCongDan, soPhien, directory, dinhDanh, idemStore, log),
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

	select {
	case err := <-loi:
		return err
	case <-dungLai:
		log.Info("nhận tín hiệu dừng, đang đóng kết nối", "service", "petitions")
		ctx, huy := context.WithTimeout(context.Background(), 20*time.Second)
		defer huy()
		return srv.Shutdown(ctx)
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
//
// # TWO CHAINS, ONE PORT, SPLIT ON THE OUTER MUX BY PATH PREFIX (chốt 22/09/2026)
//
// THIS IS THE FIRST CITIZEN EDGE IN THE REPOSITORY, so this shape is the one other services will
// copy. Read the whole note before changing any of it.
//
//	/api/v1/my-citizen-reports/…   CITIZEN chain — commune from the SESSION (ADR 0022)
//	everything else                STAFF chain   — commune from `Host` (rule 1, invariant 3)
//
// WHY THE SPLIT HAS TO EXIST. The two chains disagree about the one question every request must
// answer first: which commune. The Mini App has NO DOMAIN — it calls one API host that maps to no
// commune — so httpx.TenantMiddleware answers 404 for every citizen request. Mounting the citizen
// routes on the staff mux therefore produces a service that starts, serves, passes every test in
// internal/http, and 404s every member of the public.
//
// WHY ONE PORT AND NOT TWO. One port is one k8s Service, one port name in the Ingress, and one
// health check. The separation rule 4, invariant 5 asks for is between ROUTERS and HANDLERS, and
// that is delivered here by two muxes and two Deps types — not by a second listener, which would
// buy the same isolation and cost a second deployment surface to keep in step.
//
// WHY THE PREFIX IS SAFE TO SPLIT ON, which is the part a reviewer should check rather than
// assume: `my-citizen-reports` is a RESOURCE OF ITS OWN (kb/00-foundation/ubiquitous-language.md
// §Tiền tố `my-`). Go's ServeMux matches path ELEMENTS, so this pattern cannot capture
// `/api/v1/citizen-reports/…` — the staff route — even though one string is a prefix of the other.
// tools/ingress groups by the same element, so the cluster splits them the same way this line does.
//
// THE CITIZEN CHAIN HAS NO staffauth AND NO TenantMiddleware, and both absences are the design:
//
//	StripTenantHeaders  a client naming its own commune is a client granting itself access — and
//	                    it matters MORE here, because this chain has no `Host` to contradict it
//	Recover             turns tenant.MustFrom's deliberate panic into a traceable 500
//	CitizenEdge         bearer token -> the session the server issued. Does NOT put the commune in
//	                    the context and does NOT refuse: httpx.XaTuPhien on the route does both
//	CitizenPrincipal    the same resolved session, read on the identity axis. ONE registry lookup
//	                    feeds both axes; two would be two answers that can disagree mid-revocation
//
// idem.Middleware IS NOW ON THE CITIZEN CHAIN TOO, and that note used to say the opposite for a
// reason that has expired: it said the surface carried one GET, so duplicate protection would be a
// claim with nothing to protect. POST /api/v1/my-citizen-reports is the write that changed it — a
// double-tapped `Gửi` producing two petitions with two lookup codes is permanent, because rule 7
// forbids hard delete.
//
// IT SITS INNERMOST, unlike the staff chain where it sits immediately inside TenantMiddleware. The
// difference is not a style choice: Middleware only puts the Store on the context, while the
// COMMUNE that prefixes the key is put there by httpx.XaTuPhien — which is declared PER ROUTE on
// this surface, not on the chain (ADR 0022). So the commune is in place by the time
// idem.Required runs inside the route, and nowhere earlier on this chain is it available at all.
//
// # TWO PATTERNS ON THE OUTER MUX FOR ONE RESOURCE, AND THE SECOND IS LOAD-BEARING
//
// `tienToCongDan` ends in `/` so it matches the SUBTREE. It does NOT match the collection itself.
//
// MEASURED RATHER THAN ASSUMED, because the first version of this note guessed and guessed wrong:
// with `tapCongDan` removed, a POST to `/api/v1/my-citizen-reports` is answered **307** with
// `Location: /api/v1/my-citizen-reports/` — and following that redirect lands on a path NO pattern
// matches, so the citizen gets `404 page not found` in plain text. 307 preserves the method and the
// body, so a compliant client really does re-send the report; it just re-sends it at a door that
// does not exist.
//
// Registering `tapCongDan` explicitly is what puts the intake on the citizen chain rather than on
// the staff chain, where TenantMiddleware would answer 404 to every citizen who pressed send.
func dungBien(mux, muxCongDan http.Handler, soPhien httpx.CitizenSessions,
	danhBa tenant.Directory, dinhDanh staffauth.Resolver,
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

	// The citizen chain. Order is outermost-last here, exactly as above.
	var c http.Handler = muxCongDan
	c = idem.Middleware(idemStore, log)(c)
	c = authz.CitizenPrincipal()(c)
	c = httpx.CitizenEdge(soPhien)(c)
	c = httpx.Recover(traceID)(c)
	c = httpx.StripTenantHeaders(c)

	ngoai := http.NewServeMux()
	ngoai.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	// THE CITIZEN PREFIX IS REGISTERED FIRST FOR READABILITY ONLY — Go's ServeMux picks the most
	// SPECIFIC pattern, not the first registered, so the order of these two lines does not decide
	// anything. Swapping them changes nothing; deleting the first sends every citizen request into
	// the staff chain and answers 404 to all of them.
	ngoai.Handle(tienToCongDan, c)
	// THE COLLECTION ITSELF. Deleting this line turns every citizen submission into a 307 to the
	// subtree root followed by a bare `404 page not found` — measured, not assumed. See the note on
	// dungBien.
	ngoai.Handle(tapCongDan, c)
	ngoai.Handle("/", h)
	return ngoai
}

// tienToCongDan is the one path prefix served by the citizen chain.
//
// THE TRAILING SLASH IS LOAD-BEARING: without it this is an exact-match pattern and
// `/api/v1/my-citizen-reports/PA-…` falls through to the staff chain, where TenantMiddleware
// answers 404 to every citizen — a failure that looks exactly like "no such petition".
//
// A CONSTANT RATHER THAN A LITERAL, because this string has to agree with the route registered in
// internal/http/routes_cong_dan.go, and main_test.go asserts the agreement. Two spellings of one
// path is the defect this names out of existence.
const tienToCongDan = tapCongDan + "/"

// tapCongDan is the same resource WITHOUT the trailing slash — the collection a citizen POSTs to.
//
// DERIVED FROM, NOT PARALLEL TO, the prefix above: one literal in the file, so the two patterns
// registered on the outer mux cannot come to name two different resources. Two spellings of one
// path is the defect this arrangement names out of existence, and here it would be a silent one —
// a citizen's report answered with a redirect.
const tapCongDan = "/api/v1/my-citizen-reports"

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
