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
	"github.com/vihat/vigov/core/malwarescan"
	"github.com/vihat/vigov/core/migrate"
	"github.com/vihat/vigov/core/platformclient"
	"github.com/vihat/vigov/core/platformclient/uploadpolicy"
	"github.com/vihat/vigov/core/ratelimit"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/staffauth"
	"github.com/vihat/vigov/core/storage"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	commsapp "github.com/vihat/vigov/service-comms/internal/app"
	svcgrpc "github.com/vihat/vigov/service-comms/internal/grpc"
	svchttp "github.com/vihat/vigov/service-comms/internal/http"
	"github.com/vihat/vigov/service-comms/internal/imagefetch"
	"github.com/vihat/vigov/service-comms/internal/mail"
	"github.com/vihat/vigov/service-comms/internal/portal"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
	"github.com/vihat/vigov/service-comms/internal/store/crosstenant"
	"github.com/vihat/vigov/service-comms/internal/store/platformstore"
	"github.com/vihat/vigov/service-comms/internal/zalobot"
	"github.com/vihat/vigov/service-comms/migrations"
)

// configUses is every configuration group this binary reads — and so, in staging and prod, every
// group whose variables must be set for it to start (core/config/uses.go). Undeclared groups are
// not read at all. TestConfigUsesMatchReads keeps this list equal to what the package reads.
//
// comms: REST, plus since 2026-09-29 a gRPC server (GRPCServer) for DeliverStaffNotifications — the
// automation jobs' notices into the header bell (ADR 0058 §3); the one service storing per-commune
// secrets (SecretEncryption). Since 2026-10-01 it stores the Mini App cover image (ADR 0047 §6 (1),
// ADR 0052): ObjectStore + MalwareScan like petitions, and PublicMedia because it is the first service
// to PUBLISH a derivative — `image_url` on the public news routes is built from that base URL. Declared
// ⇒ required in staging and prod (ADR 0057): comms refuses to start there without them. Since 2026-10-05
// it holds the ONE shared Zalo Bot (ADR 0074): ZaloBotWebhook is the host its webhook points at.
var configUses = config.Uses(
	config.HTTPServer,
	config.GRPCServer,
	config.PlatformClient,
	config.IdentityClient,
	config.TenantCache,
	config.Redis,
	config.CitizenCORS,
	config.SecretEncryption,
	config.ObjectStore,
	config.PublicMedia,
	config.MalwareScan,
	config.ZaloBotWebhook,
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
	//                  internal/event.PetitionMergeChangedConsumer (`petitions.merge_changed.v1`,
	//                  ADR 0087) waits on the SAME two things and is wired beside it on that day.
	//                  Its MauTinXa keys are `gop-phieu` / `tach-phieu`: whoever builds the template
	//                  store must give each commune an approved template for both, not only for
	//                  the status codes.
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
	nenTang, err := platformclient.Dial(cfg.PlatformGRPCAddr(), cfg.GRPCCallerKey(), log)
	if err != nil {
		log.Error("không nối được dịch vụ nền tảng", "service", "comms", "err", err)
		os.Exit(1)
	}
	directory := tenant.NewCachedDirectory(nenTang, cfg.TenantCacheTTL())

	// 4. identity — session cookie -> staff principal, over gRPC. This service owns no session
	// registry and may not import identity's (rule 2, forbidden #1), so the principal comes from
	// the contract: ResolveStaffPrincipal, once per staff request, with no cache.
	dinhDanh, err := identityclient.Dial(cfg.IdentityGRPCAddr(), cfg.GRPCCallerKey(), log)
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

	loaiTaiNguyen := commsstore.NewLoaiTaiNguyenBanDoStore(kho)

	// The internal announcement book (migration 0005). ONE store behind both routes: the read is a
	// query, the write goes through the use case that owns the transaction its audit entry shares.
	thongBao := commsstore.NewThongBaoNoiBoStore(kho)

	// Mini App content (migration 0006). TWO stores for two tables, and the content write use case
	// takes BOTH: composing an item has to check that the category it is filed under is a live
	// category of this commune, which is a read of the other table inside the SAME transaction
	// (rule 6, invariant 3 — the audit entry shares it).
	noiDung := commsstore.NewNoiDungMiniAppStore(kho)
	danhMucNoiDung := commsstore.NewDanhMucMiniAppStore(kho)

	// The cover image (migration 0011's stored_file, ADR 0052). In staging and prod the three groups are
	// declared, so config.Load already refused to start without them. In DEV they may be absent: the
	// cover routes then answer 503 "chưa cấu hình kho lưu tệp" and everything else serves. A value that
	// is present but malformed was refused by config.Load, and is refused again here.
	//
	// DECLARED AS THE INTERFACES AND LEFT nil WHEN ABSENT: a nil *storage.Client assigned into the
	// interface would be a non-nil interface, pass the use case's nil check, and panic on first use.
	storedFiles := commsstore.NewStoredFileStore(kho)
	var objects commsapp.CoverObjectStore
	switch c, err := storage.New(cfg.ObjectStorage()); {
	case err == nil:
		objects = c
		log.Info("kho lưu tệp", append([]any{"service", "comms"}, c.LogAttrs()...)...)
	case errors.Is(err, storage.ErrNotConfigured):
		log.Warn("CẢNH BÁO: chưa cấu hình kho lưu tệp — ảnh bìa nội dung Mini App bị từ chối", "service", "comms", "err", err)
	default:
		log.Error("cấu hình kho lưu tệp không hợp lệ", "service", "comms", "err", err)
		os.Exit(1)
	}
	var scanner commsapp.MalwareScanner
	switch s, err := malwarescan.New(cfg.MalwareScanner()); {
	case err == nil:
		scanner = s
	case errors.Is(err, malwarescan.ErrNotConfigured):
		log.Warn("CẢNH BÁO: chưa cấu hình máy quét mã độc — ảnh bìa nội dung Mini App bị từ chối", "service", "comms", "err", err)
	default:
		log.Error("cấu hình máy quét mã độc không hợp lệ", "service", "comms", "err", err)
		os.Exit(1)
	}
	// Platform's per-purpose limits over the SAME connection the directory uses — its interceptors put
	// "x-tenant-id" on every call, which ListUploadPolicies requires; the reader caches per commune.
	policies := uploadpolicy.New(nenTang.Client(), log)
	// The body image from a pasted link (ADR 0067 §Sửa đổi 03/10/2026, H5, K6) gets its OWN outbound
	// client: any host is allowed, unlike the portal's, so it shares the portal's address predicate
	// (internal/imagefetch → portal.AddrAllowed) and nothing of its `.gov.vn`/same-host rules. No variable.
	covers := commsapp.NewContentCovers(kho, noiDung, storedFiles, objects, scanner, policies).
		WithImageFetcher(imagefetch.New(imagefetch.Options{}))
	// The broadcast audio (ADR 0067 §4): the same object store, scanner and limits reader, its own
	// purpose (content-audio). A nil `objects` converts to a nil AudioObjectStore — still "not configured".
	var audioObjects commsapp.AudioObjectStore
	if objects != nil {
		audioObjects = objects
	}
	broadcastAudio := commsapp.NewContentAudio(kho, noiDung, storedFiles, audioObjects, scanner, policies)

	// The map field schema (migration 0007). One store behind the read route and the write use
	// case; the use case owns the transaction its audit entry shares.
	mapFieldSchemas := commsstore.NewMapFieldSchemaStore(kho)

	// The commune's mail server (migration 0008) and the envelope that seals its password (ADR 0009).
	//
	// A NIL *crypto.Envelope IS A VALID PROCESS, and a deliberate one: SECRET_ENCRYPTION_KEYS is
	// optional at config.Load (core/config/secret_encryption.go), because making it required would
	// stop this service on every machine to protect one screen. Without it the two mail-settings
	// WRITE routes answer 503 by name and write nothing; every other route keeps serving. A MALFORMED
	// value never reaches here — config.Load refuses it and the pod does not start.
	var envelope *crypto.Envelope
	if cfg.SecretEncryptionConfigured() {
		envelope, err = crypto.New(cfg.SecretEncryptionKeys(), commsstore.NewDataEncryptionKeyStore(kho))
		if err != nil {
			log.Error("không dựng được bộ niêm bí mật theo xã", "service", "comms", "err", err)
			os.Exit(1)
		}
	} else {
		log.Warn("CẢNH BÁO CẤU HÌNH", "chi_tiet",
			"SECRET_ENCRYPTION_KEYS trống — lưu và gửi thử máy chủ thư của xã sẽ trả 503 (ADR 0009)")
	}
	// THE SHARED ZALO BOT (migration 0018, ADR 0074) — platform-scope, no commune by design. Its store is
	// internal/store/platformstore on the RAW pool, the countable unscoped package beside crosstenant; its
	// secrets are sealed by the PLATFORM envelope (same KEKs, a different key space — core/crypto
	// platform.go), nil without SECRET_ENCRYPTION_KEYS exactly like the commune envelope: the six RPCs
	// that need a key then answer FAILED_PRECONDITION by name. The Zalo base URL is a constant of the
	// adapter, not a variable; the webhook host is ZALO_BOT_WEBHOOK_HOST (required in staging/prod).
	platformStore := platformstore.New(db)
	var platformEnvelope *crypto.PlatformEnvelope
	if cfg.SecretEncryptionConfigured() {
		platformEnvelope, err = crypto.NewPlatform(cfg.SecretEncryptionKeys(), platformStore)
		if err != nil {
			log.Error("không dựng được bộ niêm bí mật cấp nền tảng", "service", "comms", "err", err)
			os.Exit(1)
		}
	}
	// ONE Zalo client for the process: the operator acts, the staff test message, the webhook's replies and
	// the dispatcher all go through it (internal/zalobot: no proxy, verified TLS, no redirects).
	zaloClient := zalobot.New(zalobot.DefaultTimeout)
	zaloBotOperator := commsapp.NewZaloBotOperator(commsapp.NewZaloBotStore(platformStore), platformEnvelope,
		zaloClient, cfg.ZaloBotWebhookHost())
	// The commune side's read of the same bot (metadata, token, both webhook secrets), and the scoped store
	// of the four commune tables of 0018. The cross-commune statements are crosstenant.ZaloBot, on the raw
	// pool, like the portal runner's.
	zaloBot := commsapp.NewSharedZaloBotAccess(commsapp.NewZaloBotStore(platformStore), platformStore, platformEnvelope)
	zaloLinks := commsstore.NewZaloLinkStore(kho)
	zaloCross := crosstenant.NewZaloBot(db)
	// A COMMUNE'S OWN BOT (migration 0022, ADR 0079 Q1): a COMMUNE table on the scoped handle, its token and
	// webhook secret sealed by the COMMUNE envelope (the mail password's, above). The router answers, per
	// commune, which bot sends — its own live bot, else the shared one — for the staff routes and the
	// dispatcher alike.
	zaloCommuneBots := commsstore.NewZaloCommuneBotStore(kho)
	zaloRouter := commsapp.NewZaloBotRouter(zaloBot, zaloCommuneBots, envelope)

	mailSettings := commsapp.NewMailSettingsAdmin(kho, commsstore.NewMailSettingsStore(kho), envelope,
		mail.NewSender(nil, mail.DefaultTimeout))

	// The header-bell inbox (migration 0010). ONE store behind the staff reads and the use case; the
	// use case owns every write's transaction and audit entry — the REST mark-read routes AND the gRPC
	// delivery below share it.
	staffInbox := commsstore.NewStaffNotificationStore(kho)
	// Since 2026-10-05 the same transaction also queues each notice's Zalo copy (ADR 0074; zalo_delivery is
	// the outbox), inside a savepoint: a Zalo failure never fails the bell.
	staffNotifications := commsapp.NewStaffNotifications(kho, staffInbox).WithZaloOutbox(zaloLinks, log)

	// The portal sync (migration 0013, ADR 0067 §2). ONE outbound client for the process — its
	// transport is the only way to a socket and it carries both SSRF checks (internal/portal). NO NEW
	// VARIABLE: the portal address and key are PER-COMMUNE values, read at runtime from the commune's own
	// row (rule 1 invariant 10, rule 11 forbidden #6); the timeouts and caps are vendor constants in the
	// adapter. The runner's locks and its "which communes are due" list are the countable unscoped
	// statements of internal/store/crosstenant, on the raw pool, and nothing else gets that pool.
	//
	// The same envelope as the mail server: nil without SECRET_ENCRYPTION_KEYS, and then every portal
	// write answers 503 and every scheduled run ends `that-bai` by name. The same cover pipeline as the
	// staff upload (`covers`): without object storage or a scanner, articles import without images.
	portalStore := commsstore.NewPortalSyncStore(kho)
	portalClient := portal.New(portal.Options{})
	portalRunner, err := commsapp.NewPortalSyncRunner(commsapp.PortalSyncRunnerDeps{
		DB: kho, Repo: portalStore, Locks: crosstenant.NewPortalSync(db), Registry: nenTang,
		Client: portalClient, Envelope: envelope, Covers: covers, Log: log,
	})
	if err != nil {
		log.Error("không dựng được bộ chạy đồng bộ Cổng", "service", "comms", "err", err)
		os.Exit(1)
	}
	portalSync := commsapp.NewPortalSyncAdmin(kho, portalStore, envelope, portalClient, portalRunner)

	// THE RATE-LIMIT COUNTER — one Redis client over REDIS_DSN (the idempotency store's; config.Redis is
	// declared, so staging and prod refuse to start without it), shared by the two policies below. In DEV
	// without Redis it fails every count: the PUBLIC news read then serves (its owner-decided fail-open,
	// D2) and the staff image fetch answers 503 (closed, K10) — each policy decides, not the counter.
	var rateCounter ratelimit.Counter = unavailableCounter{}
	if dsn := cfg.RedisDSN(); dsn != "" {
		rc, err := ratelimit.NewRedisCounter(dsn.Lo())
		if err != nil {
			log.Error("không mở được Redis cho giới hạn tần suất", "service", "comms", "err", err)
			os.Exit(1)
		}
		rateCounter = rc
	}
	// The image-from-a-pasted-link limit (owner, 03/10/2026, ADR 0067 K10): 30 per hour per officer.
	imageFetchLimiter, err := ratelimit.New(rateCounter, ratelimit.StaffImageFetch)
	if err != nil {
		log.Error("không dựng được bộ giới hạn tần suất lấy ảnh từ liên kết", "service", "comms", "err", err)
		os.Exit(1)
	}

	// THE ABANDONED-DRAFT BODY-IMAGE SWEEP (owner, 03/10/2026, ADR 0067 K11) — the portal runner's
	// pattern: an in-process ticker, ONE advisory lock so one replica sweeps, the communes with work listed
	// as identifiers (internal/store/crosstenant), each commune swept in its own context by the system.
	bodyImageSweeper, err := commsapp.NewBodyImageSweeper(commsapp.BodyImageSweeperDeps{
		DB: kho, Repo: storedFiles, Locks: crosstenant.NewBodyImageSweep(db), Registry: nenTang, Log: log,
	})
	if err != nil {
		log.Error("không dựng được việc nền gỡ ảnh thân bài bỏ dở", "service", "comms", "err", err)
		os.Exit(1)
	}

	// THE ZALO BOT CHANNEL (ADR 0074): two limiters on the same counter. The pairing one is the owner's
	// "5 lần thử/giờ mỗi chat"; the webhook one is PROVISIONAL (core/ratelimit ZaloBotWebhookLimit). Both
	// fail closed: without Redis, pairing replies "try later" and the webhook answers 503.
	zaloPairingLimiter, err := ratelimit.New(rateCounter, ratelimit.ZaloBotPairing)
	if err != nil {
		log.Error("không dựng được bộ giới hạn thử mã ghép Zalo", "service", "comms", "err", err)
		os.Exit(1)
	}
	zaloWebhookLimiter, err := ratelimit.New(rateCounter, ratelimit.ZaloBotWebhook)
	if err != nil {
		log.Error("không dựng được bộ giới hạn tần suất webhook Zalo Bot", "service", "comms", "err", err)
		os.Exit(1)
	}
	// The sender of owed Zalo messages — the sweep's pattern (one advisory lock, per-commune context).
	// `nenTang` gives a commune's state (a merged one is left untouched) and its host for message links.
	zaloDispatcher, err := commsapp.NewZaloDispatcher(commsapp.ZaloDispatcherDeps{
		DB: kho, Repo: zaloLinks, Locks: zaloCross, Registry: nenTang, Bots: zaloRouter, Send: zaloClient, Log: log,
	})
	if err != nil {
		log.Error("không dựng được việc nền gửi tin Zalo", "service", "comms", "err", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	svchttp.Register(mux, svchttp.Deps{
		Checker:       staffauth.Checker{},
		LoaiTaiNguyen: loaiTaiNguyen,
		ThongBao:      thongBao,
		// Issuing rings each recipient's bell in the same transaction (`thong-bao.moi`, migration 0025),
		// through the bell's own store, and queues the Zalo copies like the gRPC delivery does.
		GhiThongBao: commsapp.NewSoanThongBaoNoiBo(kho, thongBao, staffInbox).WithZaloOutbox(zaloLinks, log),

		NoiDung: noiDung,
		GhiNoiDung: commsapp.NewSoanNoiDungMiniApp(kho, noiDung, danhMucNoiDung).
			WithCovers(storedFiles, objects, log).WithAudioFiles(storedFiles),
		DanhMucNoiDung:    danhMucNoiDung,
		GhiDanhMucNoiDung: commsapp.NewDanhMucNoiDungMiniApp(kho, danhMucNoiDung),
		ContentCovers:     covers,
		ImageFetchLimiter: imageFetchLimiter,
		ContentAudio:      broadcastAudio,
		// The write use case owns the transaction the business write and its audit entry share
		// (rule 6, invariant 3). It is given *store.DB rather than a transaction because opening one
		// is precisely what it is for.
		GhiLoaiTaiNguyen:     commsapp.NewDanhMucLoaiTaiNguyen(kho, loaiTaiNguyen),
		MapFieldSchemas:      mapFieldSchemas,
		WriteMapFieldSchemas: commsapp.NewMapFieldSchemas(kho, mapFieldSchemas),
		MailSettings:         mailSettings,
		WriteMailSettings:    mailSettings,
		// This service's OWN audit_log, on its own handle — never another service's (ADR 0054 §1).
		AuditLog:        audit.NewLog(kho),
		StaffInbox:      staffInbox,
		WriteStaffInbox: staffNotifications,
		PortalSync:      portalSync,
		WritePortalSync: portalSync,
		Log:             log,
	})

	// External contacts (migration 0014; user decision 03/10/2026) — four staff routes on the SAME staff
	// mux, the same checker. ONE store behind the read and the use case; the use case owns every write's
	// transaction and its audit entry. Registered beside Register rather than inside it: see
	// internal/http/external_contacts.go.
	externalContacts := commsstore.NewExternalContactStore(kho)
	svchttp.RegisterExternalContacts(mux, svchttp.ExternalContactDeps{
		Checker: staffauth.Checker{},
		Reader:  externalContacts,
		Writer:  commsapp.NewExternalContacts(kho, externalContacts),
		Log:     log,
	})

	// The economic map's asset register (migration 0015; ADR 0072) — nine staff routes on the SAME staff
	// mux, the same checker. ONE store behind the reads and the use case; the use case owns every write's
	// transaction and its audit entry. The default-groups seed writes the type catalogue through its own
	// store (loaiTaiNguyen, above). See internal/http/map_assets.go.
	mapAssets := commsstore.NewMapAssetStore(kho)
	svchttp.RegisterMapAssets(mux, svchttp.MapAssetDeps{
		Checker:      staffauth.Checker{},
		Reader:       mapAssets,
		Writer:       commsapp.NewMapAssets(kho, mapAssets),
		TypeDefaults: commsapp.NewMapAssetTypeDefaults(kho, loaiTaiNguyen),
		Log:          log,
	})

	// The economic map's frame (migrations 0016/0017; ADR 0072 H3, amendment 2) — three staff routes on
	// the SAME staff mux. One use case behind them: the effective read, the save and "Về mặc định" (each
	// write with its transaction and audit entry). `nenTang` — the same platform client as the Host edge
	// — supplies the platform DEFAULT frame when the commune applies none of its own (K3). See
	// internal/http/map_frame.go.
	mapFrames := commsapp.NewMapFrames(kho, commsstore.NewMapFrameStore(kho), nenTang, log)
	svchttp.RegisterMapFrame(mux, svchttp.MapFrameDeps{
		Checker: staffauth.Checker{},
		Reader:  mapFrames,
		Writer:  mapFrames,
		Log:     log,
	})

	// The Zalo Bot channel's staff routes (ADR 0074 §"Tài nguyên URL") — the caller's own link
	// (AnyAuthenticated) and the commune's administration (admin.lookup), on the SAME staff mux. Staff names
	// for the linked list come from identity's ResolveStaffNames over the client the edge already holds.
	zaloLinkActs := commsapp.NewZaloLinks(kho, zaloLinks, zaloRouter, zaloClient, dinhDanh, log)
	svchttp.RegisterZaloLinks(mux, svchttp.ZaloLinkDeps{
		Checker: staffauth.Checker{},
		Reader:  zaloLinkActs,
		Writer:  zaloLinkActs,
		Log:     log,
	})
	// The commune's own bot (ADR 0079 Q1 #5) — five admin.lookup routes on the SAME staff mux. `nenTang`
	// gives the commune's host, which the webhook URL registered with Zalo is built on.
	svchttp.RegisterZaloCommuneBot(mux, svchttp.ZaloCommuneBotDeps{
		Checker: staffauth.Checker{},
		Bots:    commsapp.NewCommuneZaloBots(kho, zaloCommuneBots, zaloLinks, envelope, zaloClient, nenTang, log),
		Log:     log,
	})

	// THE THIRD EDGE CHAIN — the Zalo Bot webhook (ADR 0074 #5; ADR 0079 Q1 #2). Its own mux: no session,
	// no CORS. Mounted twice by withZaloBotWebhook: on ZALO_BOT_WEBHOOK_HOST with no commune (the shared
	// bot), and on every other host behind TenantMiddleware (a commune's own bot, commune from Host).
	muxZaloBot := http.NewServeMux()
	svchttp.RegisterZaloBotUpdates(muxZaloBot, svchttp.ZaloBotUpdateDeps{
		Updates: commsapp.NewZaloWebhook(kho, zaloLinks, zaloCommuneBots, zaloCross, zaloPairingLimiter, zaloBot,
			zaloClient, log),
		CommuneUpdates: commsapp.NewCommuneZaloWebhook(kho, zaloLinks, zaloCommuneBots, envelope, zaloPairingLimiter,
			zaloClient, log),
		Limiter: zaloWebhookLimiter,
		Log:     log,
	})
	if cfg.ZaloBotWebhookHost() == "" {
		log.Warn("CẢNH BÁO CẤU HÌNH", "chi_tiet",
			"ZALO_BOT_WEBHOOK_HOST trống — webhook Zalo Bot không được phục vụ ở host nào (chỉ dev)")
	}

	// THE PUBLIC SURFACE (owner decision 2026-09-27) — its own mux, its own Deps, its own chain. `nenTang`
	// is the SAME platform client the Host edge uses, asked through XaTheoHost so an outage is a 503 and
	// never "no such commune". The two content stores are the SAME ones the staff routes use, reached
	// only through their published-only reads.
	//
	// THE PUBLIC RATE LIMIT (ratelimit.PublicNewsRead, owner 02/10/2026) counts in the same Redis counter
	// as the staff image-fetch limit above (REDIS_DSN). In DEV without Redis the counter fails every count
	// and this policy FAILS OPEN by the owner's decision: the routes serve and one security warning per
	// minute says the bound is off.
	publicLimiter, err := ratelimit.New(rateCounter, ratelimit.PublicNewsRead)
	if err != nil {
		log.Error("không dựng được bộ giới hạn tần suất tuyến công khai", "service", "comms", "err", err)
		os.Exit(1)
	}
	// THE PUBLIC HOST LOOKUP IS CACHED (02/10/2026): the same tenant.CachedDirectory, the same
	// TENANT_CACHE_TTL and TranMuc ceiling as the staff edge, misses remembered — so a flood of one
	// unknown host costs one ResolveHost per TTL, not one per request (the limiter above counts only
	// AFTER the lookup). Asked through XaTheoHost, which never caches an error, so an outage stays a 503.
	//
	// ITS OWN INSTANCE, NOT `directory`: the staff edge asks through ByHost, which cannot tell an
	// outage from an unknown host and caches both as a miss. Shared, a registry blip seen by a staff
	// request would serve residents an empty notice board for one TTL instead of the 503 this surface
	// promises (TraXaTheoHost). The cost is a second bounded map; comms never calls Forget on either.
	publicDirectory := tenant.NewCachedDirectory(nenTang, cfg.TenantCacheTTL())
	muxCongKhai := http.NewServeMux()
	svchttp.RegisterCongKhai(muxCongKhai, svchttp.DepsCongKhai{
		Limiter:     publicLimiter,
		Xa:          publicDirectory,
		NoiDung:     noiDung,
		Views:       noiDung,
		DanhMuc:     danhMucNoiDung,
		CoverImages: covers,
		Audio:       broadcastAudio,
		Log:         log,
	})
	// The public external-contact read — the SAME platform lookup and the SAME limiter instance as the
	// news routes (the user chose the existing PublicNewsRead policy, 03/10/2026), so one client network
	// has one budget per host across both.
	svchttp.RegisterPublicExternalContacts(muxCongKhai, svchttp.PublicExternalContactDeps{
		Xa:       publicDirectory,
		Contacts: externalContacts,
		Limiter:  publicLimiter,
		Log:      log,
	})
	congKhai := dungBienCongKhai(muxCongKhai, cfg.CitizenCORSAllowedOrigins(), log)

	// Rule 11, invariant 1: the environment is read in core/config and nowhere else.
	// LISTEN_ADDR or ":8080" — one default for every service, see config.Config.ListenAddr.
	addr := cfg.ListenAddr()
	log.Info("starting", "service", "comms", "addr", addr)
	srv := &http.Server{
		Addr: addr,
		// OUTERMOST, around BOTH chains: every layer reads one client address per request, crossing
		// only the proxies TRUSTED_PROXY_CIDRS names (rule 6, invariant 2).
		Handler: httpx.ClientIPTuProxyTinCay(cfg.TrustedProxies())(withZaloBotWebhook(
			dungBien(mux, congKhai, directory, dinhDanh, idemStore, log),
			zaloBotChain(muxZaloBot), communeZaloBotChain(muxZaloBot, directory), cfg.ZaloBotWebhookHost())),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// THE gRPC SURFACE — its own port (GRPC_LISTEN_ADDR, default :9090), the same shape as petitions.
	// DeliverStaffNotifications (ADR 0058 §3), writing through the SAME use case the bell's REST routes
	// use; and since 2026-10-05 ZaloBotOperatorService (ADR 0074), the six platform-scope RPCs
	// service-platform's operator routes call — on THIS port, behind the SAME caller key and chain, exempt
	// from the commune by name in core/grpcx.methodsWithoutTenant.
	//
	// Plaintext, like every gRPC port here (ADR 0025): the guard is GRPC_CALLER_KEY on every RPC plus
	// the NetworkPolicy confining the port to the cluster — both, not either.
	grpcSrv := buildGRPCServer(cfg.GRPCCallerKey(), svcgrpc.Deps{Notifications: staffNotifications, Log: log},
		svcgrpc.NewZaloBotServer(zaloBotOperator, log))
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
	dungLai := make(chan os.Signal, 1)
	signal.Notify(dungLai, os.Interrupt, syscall.SIGTERM)

	// THE PORTAL SYNC RUNNER lives as long as this context. Cancelled FIRST on shutdown, so a run in
	// progress stops between articles and still writes its one finish fill (on a context the cancel
	// cannot reach — app.PortalSyncRunner.finish); manual runs started from HTTP derive from it too.
	jobs, stopJobs := context.WithCancel(context.Background())
	runnerDone := make(chan struct{})
	go func() {
		defer close(runnerDone)
		portalRunner.Run(jobs)
	}()
	// The body-image sweep (K11) on the same context: cancelled first on shutdown. A sweep cut mid-commune
	// loses nothing — each commune is one transaction, and what it did not reach is swept next tick.
	sweepDone := make(chan struct{})
	go func() {
		defer close(sweepDone)
		bodyImageSweeper.Run(jobs)
	}()
	// The Zalo sender (ADR 0074) on the same context. A pass cut mid-send leaves its rows leased; the lease
	// expires and they are sent again — never lost, at worst sent twice.
	zaloDone := make(chan struct{})
	go func() {
		defer close(zaloDone)
		zaloDispatcher.Run(jobs)
	}()

	// Buffered for two: either server may fail, and a send nobody reads would leak its goroutine.
	loi := make(chan error, 2)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			loi <- err
		}
	}()
	go func() {
		log.Info("starting gRPC", "service", "comms", "addr", cfg.GRPCListenAddr())
		// Serve returns nil after GracefulStop, so there is no ErrServerClosed to filter.
		if err := grpcSrv.Serve(grpcLis); err != nil {
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
		// One surface failing takes the process down rather than leaving it half-serving: REST up with
		// gRPC down looks healthy while every automation job's delivery is refused.
		log.Error("server stopped", "err", err)
		stopJobs()
		grpcSrv.Stop()
		_ = srv.Close()
		os.Exit(1)
	case <-dungLai:
		log.Info("nhận tín hiệu dừng, đang đóng kết nối", "service", "comms")
		ctx, huy := context.WithTimeout(context.Background(), 20*time.Second)
		defer huy()

		// The sync stops first and drains in parallel with the servers, inside the same 20 seconds. A run
		// that cannot finish in time is left unfinished and is closed `that-bai` by the next run (0013).
		stopJobs()
		jobsDrained := make(chan bool, 1)
		go func() {
			select {
			case <-runnerDone:
			case <-ctx.Done():
			}
			jobsDrained <- portalRunner.Wait(time.Until(deadlineOf(ctx)))
		}()

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
		if !<-jobsDrained {
			log.Warn("lượt đồng bộ Cổng chưa xong kịp hạn — sẽ được đóng `that-bai` ở lượt sau", "service", "comms")
		}
		select {
		case <-sweepDone:
		case <-ctx.Done():
			log.Warn("việc nền gỡ ảnh thân bài bỏ dở chưa dừng kịp hạn — giao dịch dở sẽ bị cuộn lại", "service", "comms")
		}
		select {
		case <-zaloDone:
		case <-ctx.Done():
			log.Warn("việc nền gửi Zalo chưa dừng kịp hạn — tin đang giữ sẽ được gửi lại sau hạn giữ", "service", "comms")
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
// so a call without "x-tenant-id" is refused with InvalidArgument before the handler (rule 1). The six
// ZaloBotOperatorService RPCs ARE exempt, by full name (core/grpcx) — the caller key is not.
func buildGRPCServer(callerKey secret.Secret, d svcgrpc.Deps, zaloBot commsv1.ZaloBotOperatorServiceServer) *grpc.Server {
	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			// Panics at construction when GRPC_CALLER_KEY is empty: a server without the key accepts
			// every call it should refuse, and nothing looks wrong.
			grpcx.UnaryServerCallerAuth(callerKey, d.Log),
			grpcx.UnaryServerInterceptor(),
		),
	)
	commsv1.RegisterCommsServiceServer(srv, svcgrpc.NewServer(d))
	commsv1.RegisterZaloBotOperatorServiceServer(srv, zaloBot)
	return srv
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
// # TWO CHAINS, ONE PORT, SPLIT ON THE OUTER MUX BY PATH (since 2026-09-27)
//
//	/api/v1/commune-news, /api/v1/commune-news/…   PUBLIC chain (congKhai, dungBienCongKhai)
//	/api/v1/commune-external-contacts              PUBLIC chain
//	everything else                                STAFF chain — commune from `Host`
//
// The split exists because the two disagree on the first question of every request — which commune.
// The Mini App calls the reserved API host (ADR 0046), which TenantMiddleware answers 404, so a public
// route mounted on the staff mux would start, pass every internal/http test, and 404 every resident.
// `commune-news` is its own path ELEMENT: Go's ServeMux matches whole elements, so neither pattern can
// capture `/api/v1/content-items/…`, the staff register. main_test.go asserts both directions.
func dungBien(mux, congKhai http.Handler, danhBa tenant.Directory, dinhDanh staffauth.Resolver,
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
	// BOTH patterns, and the first is load-bearing: the subtree pattern alone does NOT match the
	// collection itself — ServeMux answers `/api/v1/commune-news` with a redirect to the slash form
	// (service-petitions/cmd/server/main.go §TWO PATTERNS measured it), and following it lands on
	// `/api/v1/commune-news/` — a path no public route matches, so the list would 404.
	ngoai.Handle(svchttp.MauTinXa, congKhai)
	ngoai.Handle(svchttp.MauTinXa+"/", congKhai)
	// The collection only: the public external-contact surface has no subtree. Its own path ELEMENT, so
	// it captures neither `/api/v1/external-contacts` (staff) nor anything under it.
	ngoai.Handle(svchttp.PublicExternalContactsPath, congKhai)
	ngoai.Handle("/", h)
	return ngoai
}

// dungBienCongKhai builds the PUBLIC edge chain, outermost last:
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
// unavailableCounter is the public rate-limit store when REDIS_DSN is unset (dev only — staging and
// prod refuse to start without it). It fails every count; ratelimit.PublicNewsRead fails OPEN, so the
// public routes serve and the limiter warns once per window that its bound is off.
type unavailableCounter struct{}

func (unavailableCounter) Incr(context.Context, string, time.Duration) (int64, time.Duration, error) {
	return 0, 0, errors.New("REDIS_DSN chưa đặt")
}

func dungBienCongKhai(muxCongKhai http.Handler, nguonCORS httpx.NguonCORS, log *slog.Logger) http.Handler {
	c := muxCongKhai
	c = httpx.Recover(traceID)(c)
	c = httpx.StripTenantHeaders(c)
	c = httpx.CORSCongDan(nguonCORS, log)(c)
	return c
}

// zaloBotChain builds the THIRD edge chain — the shared Zalo Bot's webhook (ADR 0074 #5), outermost last:
//
//	StripTenantHeaders  a client naming its own commune is granting itself access; here there is not even
//	                    a Host commune to contradict it
//	Recover             a panic becomes a traceable 500
//
// NO TenantMiddleware (the webhook host maps to no commune — the commune comes from a pairing code or a
// link), NO staffauth (Zalo has no session), NO idem (Zalo sends no key), NO CORS (server to server). The
// route authenticates itself: rate limit, then the secret header, then the body (internal/http).
func zaloBotChain(muxZaloBot http.Handler) http.Handler {
	h := muxZaloBot
	h = httpx.Recover(traceID)(h)
	h = httpx.StripTenantHeaders(h)
	return h
}

// communeZaloBotChain is the webhook chain for a COMMUNE's own bot (ADR 0079 Q1 #2), outermost last:
//
//	StripTenantHeaders  a client naming its own commune is granting itself access
//	Recover             a panic becomes a traceable 500
//	TenantMiddleware    the commune from Host; unknown Host = 404, never a default (rule 1, invariant 3)
//
// The SAME mux as the shared bot's: the handler reads the commune TenantMiddleware put in the context
// (internal/http/zalo_bot_updates.go). NO staffauth (Zalo has no session), NO idem (Zalo sends no key),
// NO CORS (server to server).
func communeZaloBotChain(muxZaloBot http.Handler, danhBa tenant.Directory) http.Handler {
	h := muxZaloBot
	h = httpx.TenantMiddleware(danhBa)(h)
	h = httpx.Recover(traceID)(h)
	h = httpx.StripTenantHeaders(h)
	return h
}

// withZaloBotWebhook puts the webhook in front of the other two chains — for ONE path.
//
//	<ZALO_BOT_WEBHOOK_HOST>/api/v1/zalo-bot-updates   the SHARED bot's chain (no commune). A HOST
//	                                                  PATTERN: ServeMux prefers it over the bare path
//	/api/v1/zalo-bot-updates on any other host        a COMMUNE's own bot (ADR 0079 Q1 #2): Zalo calls
//	                                                  `https://<xã>/api/v1/zalo-bot-updates`, and the
//	                                                  commune chain resolves the commune from Host
//
// The webhook host serves nothing else (the staff chain 404s it: it is no commune's host — core/config
// refuses a commune-shaped value), and a commune's host never reaches the shared bot's chain.
//
// host "" (dev only — staging and prod refuse to start without it): no SHARED webhook at all, fail
// closed. A commune's own bot does not depend on it and is served either way.
func withZaloBotWebhook(rest, sharedBot, communeBot http.Handler, host string) http.Handler {
	m := http.NewServeMux()
	if host != "" {
		m.Handle(host+svchttp.ZaloBotUpdatesPath, sharedBot)
	}
	m.Handle(svchttp.ZaloBotUpdatesPath, communeBot)
	m.Handle("/", rest)
	return m
}

// deadlineOf is ctx's deadline, or now when it has none (the shutdown context always has one).
func deadlineOf(ctx context.Context) time.Time {
	if d, ok := ctx.Deadline(); ok {
		return d
	}
	return time.Now()
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
