package app

// THE PORTAL SYNC RUN — scheduled and manual (ADR 0067 §2 "Ghi", "Việc nền"; migration 0013), in the
// service that owns the content it writes, on ADR 0058's pattern: a ticker, ONE advisory lock so one
// replica ticks, the communes with work listed as identifiers (internal/store/crosstenant), then each
// commune IN ITS OWN CONTEXT, sequentially, with the system principal as the actor of every scheduled
// write (rule 6, invariant 6) and the commune in the unit of work (rule 1, invariant 9).
//
// ONE RUN, IN ORDER:
//
//  1. the commune's run lock (0013 owes it BEFORE the run row): taken or the run does not start —
//     a manual request is 409, a scheduled tick skips the commune;
//  2. ONE transaction: crashed runs older than portalStuckAfter finished `that-bai`, the run row
//     inserted, `last_run_at` set, the start audited;
//  3. the key opened, the selected categories read from the portal — three at a time
//     (portalCategoryConcurrency), each keeping only the articles inside the window;
//  4. candidates merged newest first across categories, deduplicated by portal id, checked against the
//     register COUNTING SOFT-DELETED ROWS; at most MaxItemsPerRun new ones attempted;
//  5. their images fetched six at a time (portalImageConcurrency) through the cover pipeline;
//  6. each article imported in ITS OWN transaction with its audit entry — NEVER an UPDATE of an
//     existing item (ADR 0067 §2 "Ghi" #1);
//  7. the run's one finish fill, with its audit entry, on a context a shutdown cannot cancel.
//
// ONE CATEGORY FAILING NEVER FAILS THE RUN (spec §10.3): it is recorded by class in the summary and the
// run ends `mot-phan`. ONE COMMUNE FAILING NEVER STOPS ANOTHER: each runs behind its own recover.
//
// BOUNDS ON ONE PROCESS (R1, 02/10/2026) — what keeps one slow portal from holding the scheduler lock
// for hours, and N communes from holding N × a run's memory:
//
//	portalMaxConcurrentRuns   a process-wide semaphore, shared by scheduled and manual runs. A scheduled
//	                          commune WAITS for a slot (inside the tick's budget); a manual start does not
//	                          wait — it answers ErrPortalRunnerBusy (HTTP 503 `portal_sync_busy`): the
//	                          condition is this process's capacity, not the commune's data, so not 409.
//	portalCommuneRunBudget    one run's wall-clock budget. Hit, the run stops between articles and ends
//	                          with the class `time-budget`; the commune's next due tick continues.
//	portalTickBudget          one scheduler pass's budget. Hit, the remaining due communes wait for the
//	                          next tick — and come FIRST then (crosstenant orders by last_run_at, oldest
//	                          first), so no commune is starved by the ones ahead of it.
//
// Memory of one run is bounded by the adapter: each category keeps only the newest MaxItemsPerRun
// articles THE COMMUNE DOES NOT ALREADY HOLD (portal.Articles' keep + HeldFunc — the register, soft-
// deleted rows included, is asked per chunk of 200 BEFORE an article may take a slot). So a backlog
// larger than one run's ceiling DRAINS: each run imports the newest not-yet-imported ones, and the next
// run continues below them (follow-up of 02/10/2026, owner "làm theo đề xuất").
//
// WHAT IS NEVER LOGGED: an article's title, summary, body or URL (they name people — rule 3), the
// api_url's query, the key. Logs carry the commune id, the run id, counts and error classes.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/crypto"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-comms/internal/domain"
	"github.com/vihat/vigov/service-comms/internal/portal"
	"github.com/vihat/vigov/service-comms/internal/richtext"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

const (
	// PortalSyncTickInterval is how often the scheduler asks which communes are due. The shortest
	// interval a commune can set is one hour, so five minutes late is the worst a run starts.
	PortalSyncTickInterval = 5 * time.Minute
	// portalStuckAfter — a run unfinished this long after it started is a crashed one, finished
	// `that-bai` before the commune's next run starts. The task card's figure; a run holds its commune's
	// lock while alive, so a live run is never reaped, whatever its length.
	portalStuckAfter = time.Hour
	// The concurrency ADR 0067 §2 "Ghi" #3 fixes: three categories, six images. The portal is one
	// commune's server, not a CDN.
	portalCategoryConcurrency = 3
	portalImageConcurrency    = 6
	// portalErrorSummaryMax bounds the summary's entries, under 0013's 64 KiB CHECK with room: one entry
	// per failing category and per error class, never per article.
	portalErrorSummaryMax = 200

	// R1 (02/10/2026) — VENDOR BOUNDS on one process, not customer figures (file header). Two runs at
	// once: each holds up to 3 category bodies (≤ 32 MiB each) and 6 images (≤ 10 MiB each).
	portalMaxConcurrentRuns = 2
	// portalCommuneRunBudget — well under portalStuckAfter, so a live run is never mistaken for a
	// crashed one by the reaper.
	portalCommuneRunBudget = 10 * time.Minute
	// portalTickBudget — one pass of the scheduler, under portalStuckAfter, and the longest the global
	// scheduler lock is held.
	portalTickBudget = 30 * time.Minute
)

var (
	// ErrPortalRunInProgress — the commune's run lock is held: a run is in progress somewhere. 409.
	ErrPortalRunInProgress = errors.New("dong_bo_cong: đang có một lượt đồng bộ của xã")
	// ErrPortalRunnerStopped — the runner is not running (shutting down, or not started). 503.
	ErrPortalRunnerStopped = errors.New("dong_bo_cong: bộ chạy đồng bộ chưa sẵn sàng")
	// ErrPortalRunnerBusy — every run slot of this process is taken (R1). 503 `portal_sync_busy`.
	ErrPortalRunnerBusy = errors.New("dong_bo_cong: bộ chạy đồng bộ đang bận với các xã khác")
)

// The verbs in the trail (ADR 0011: Vietnamese snake_case values).
const (
	ActionStartPortalSync  = "bat_dau_dong_bo_cong"
	ActionFinishPortalSync = "ket_thuc_dong_bo_cong"
	ActionImportPortalItem = "nhap_tin_tu_cong"
	// ActionResealPortalKey — the stored key re-sealed under the api_url-bound additional data (R6),
	// by the system, the first time a row sealed before 02/10/2026 is opened.
	ActionResealPortalKey = "niem_lai_ma_bao_mat_cong"
)

// systemPrincipal is the "who" of every scheduled write and of every import (rule 6, invariant 6).
var systemPrincipal = audit.Actor{ID: audit.SystemActor, Kind: "system"}

// PortalSyncRepo is the store, declared at the point of use. *store.PortalSyncStore satisfies it.
type PortalSyncRepo interface {
	Settings(ctx context.Context) (domain.PortalSyncSettings, error)
	SettingsWithKey(ctx context.Context) (domain.PortalSyncSettings, []byte, error)
	SettingsForUpdate(ctx context.Context, tx *store.ScopedTx) (domain.PortalSyncSettings, []byte, bool, error)
	UpsertSettings(ctx context.Context, tx *store.ScopedTx, s domain.PortalSyncSettings, sealed []byte, by string) error
	MarkRunStarted(ctx context.Context, tx *store.ScopedTx, at time.Time) error
	ResealKey(ctx context.Context, tx *store.ScopedTx, apiURL string, oldSealed, newSealed []byte) (bool, error)

	Categories(ctx context.Context) ([]domain.PortalCategory, error)
	CategoriesForUpdate(ctx context.Context, tx *store.ScopedTx) ([]domain.PortalCategory, error)
	InsertCategory(ctx context.Context, tx *store.ScopedTx, c domain.PortalCategory, by string) error
	UpdateCategory(ctx context.Context, tx *store.ScopedTx, c domain.PortalCategory, by string) error

	Runs(ctx context.Context, req page.Request) (page.Result[domain.PortalSyncRun], error)
	InsertRun(ctx context.Context, tx *store.ScopedTx, r domain.PortalSyncRun) error
	FinishRun(ctx context.Context, tx *store.ScopedTx, r domain.PortalSyncRun) error
	ReapStuckRuns(ctx context.Context, tx *store.ScopedTx, before, at time.Time) ([]string, error)

	ExistingPortalItems(ctx context.Context, externalIDs []string) (map[string]bool, error)
	InsertSyncedItem(ctx context.Context, tx *store.ScopedTx, n domain.NoiDungMiniApp, portalCategoryID string) error
}

// PortalClient is internal/portal's *Client.
type PortalClient interface {
	Categories(ctx context.Context, e portal.Endpoint) ([]portal.Category, error)
	Articles(ctx context.Context, e portal.Endpoint, categoryID string, since time.Time, keep int,
		held portal.HeldFunc) (portal.ArticleBatch, error)
	Image(ctx context.Context, e portal.Endpoint, u *url.URL, max int64) ([]byte, error)
}

// PortalRunLocks is *crosstenant.PortalSync.
type PortalRunLocks interface {
	DueCommunes(ctx context.Context) ([]tenant.ID, error)
	TryLockScheduler(ctx context.Context) (func(), bool, error)
	TryLockCommune(ctx context.Context, id tenant.ID) (func(), bool, error)
}

// PortalCommuneRegistry reads the commune in ctx from platform (*platformclient.Directory). A merged
// commune is inactive and keeps its data (rule 7, invariant 6): nothing is imported for it.
type PortalCommuneRegistry interface {
	// vi-name-ok: the method name of the existing core/platformclient.Directory this interface must match
	XaTrongNguCanh(ctx context.Context) (tenant.Tenant, bool, error)
}

// portalCoverPipeline is the part of *ContentCovers a run uses (portal_image.go).
type portalCoverPipeline interface {
	portalImageLimit(ctx context.Context) (int64, error)
	preparePortalCover(ctx context.Context, itemID string, data []byte, now time.Time) (portalCover, error)
	recordPortalCover(ctx context.Context, tx *store.ScopedTx, pc portalCover) error
	publishPortalCover(ctx context.Context, tx *store.ScopedTx, n domain.NoiDungMiniApp, at time.Time) (bool, error)
	withdrawPortalCover(ctx context.Context, f domain.StoredFile) error
	discardPortalCover(ctx context.Context, f domain.StoredFile) (bool, error)
}

// PortalSyncRunnerDeps wires the runner. Envelope may be nil (no SECRET_ENCRYPTION_KEYS): every run then
// ends `that-bai` by name rather than not existing. Covers may be nil: articles import without images.
type PortalSyncRunnerDeps struct {
	DB       *store.DB
	Repo     PortalSyncRepo
	Locks    PortalRunLocks
	Registry PortalCommuneRegistry
	Client   PortalClient
	Envelope *crypto.Envelope
	Covers   *ContentCovers
	Log      *slog.Logger
}

// PortalSyncRunner runs portal syncs. Build with NewPortalSyncRunner; Run blocks until ctx is done;
// Wait drains manual runs started from HTTP.
type PortalSyncRunner struct {
	db       *store.DB
	repo     PortalSyncRepo
	locks    PortalRunLocks
	registry PortalCommuneRegistry
	client   PortalClient
	envelope *crypto.Envelope
	covers   portalCoverPipeline
	log      *slog.Logger

	interval time.Duration
	now      func() time.Time
	newID    func() (string, error)

	// slots is the process-wide run semaphore (portalMaxConcurrentRuns); runBudget / tickBudget are
	// portalCommuneRunBudget / portalTickBudget, fields so a test can shorten them.
	slots      chan struct{}
	runBudget  time.Duration
	tickBudget time.Duration

	mu   sync.Mutex
	base context.Context // Run's context: manual runs live as long as the process, not the request
	wg   sync.WaitGroup
}

// NewPortalSyncRunner builds the runner.
func NewPortalSyncRunner(d PortalSyncRunnerDeps) (*PortalSyncRunner, error) {
	if d.DB == nil || d.Repo == nil || d.Locks == nil || d.Client == nil {
		return nil, errors.New("dong_bo_cong: bộ chạy thiếu phụ thuộc")
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	r := &PortalSyncRunner{db: d.DB, repo: d.Repo, locks: d.Locks, registry: d.Registry, client: d.Client,
		envelope: d.Envelope, log: d.Log, interval: PortalSyncTickInterval,
		now: func() time.Time { return time.Now().UTC() }, newID: ulid.Moi,
		slots: make(chan struct{}, portalMaxConcurrentRuns), runBudget: portalCommuneRunBudget,
		tickBudget: portalTickBudget}
	if d.Covers != nil {
		r.covers = d.Covers
	}
	return r, nil
}

// portalKeyAAD binds the sealed key to THIS table, column, commune AND api_url (R6, 02/10/2026): the
// tenant-only string of 0013 plus the sha256 of the trimmed api_url. Copied onto another row it does not
// open, and neither does it after `api_url` was changed in the database without the key being typed
// again — the screen already re-seals on every address change (ADR 0067 §2 decision 5); this makes a
// write that went around the screen unable to send the key to a new host. The hash, not the URL: the
// value is additional data, and its length should not depend on what was typed.
//
// 0013's own comment still describes the tenant-only string; that migration is applied (checksum) and
// is not edited — this function is the source of truth for the binding.
func portalKeyAAD(ctx context.Context, apiURL string) []byte {
	sum := sha256.Sum256([]byte(strings.TrimSpace(apiURL)))
	return []byte("portal_sync_settings/api_key_sealed/" + string(tenant.MustFrom(ctx)) + "/" + hex.EncodeToString(sum[:]))
}

// legacyPortalKeyAAD is 0013's tenant-only binding, kept ONLY to open — once — a row sealed before
// 02/10/2026, which openPortalKey then re-seals under portalKeyAAD. Never used to seal.
//
// WHY A FALLBACK AND NOT "NO SUCH ROW EXISTS": the sync was committed on 01/10/2026 (fa7b8377) and
// nothing in this repository proves no commune saved a key since; refusing those rows would end every
// such commune's runs `credential-unavailable` until staff retype a key they have no reason to think is
// wrong. The fallback's cost: until its first open, a legacy row is exactly as protected as before R6.
// Remove it once no legacy row remains (each migration files ActionResealPortalKey in the trail).
func legacyPortalKeyAAD(ctx context.Context) []byte {
	return []byte("portal_sync_settings/api_key_sealed/" + string(tenant.MustFrom(ctx)))
}

// openPortalKey opens the commune's sealed key for st.APIURL. A row sealed under the legacy binding is
// opened with it ONCE and re-sealed under the api_url binding in a transaction with its audit entry
// (system principal, rule 6 invariant 6) — compare-and-swap on the sealed bytes and the api_url, so a
// save that raced it wins. A failed re-seal is logged and the call proceeds with the key it opened: the
// row stays legacy and the next open tries again.
func openPortalKey(ctx context.Context, db *store.DB, repo PortalSyncRepo, env *crypto.Envelope,
	st domain.PortalSyncSettings, sealed []byte, log *slog.Logger) (secret.Secret, error) {

	key, err := env.Open(ctx, sealed, portalKeyAAD(ctx, st.APIURL))
	if err == nil {
		return key, nil
	}
	key, legacyErr := env.Open(ctx, sealed, legacyPortalKeyAAD(ctx))
	if legacyErr != nil {
		return nil, err
	}
	resealed, err := env.Seal(ctx, key, portalKeyAAD(ctx, st.APIURL))
	if err != nil {
		log.WarnContext(ctx, "CẢNH BÁO: không niêm lại được mã bảo mật Cổng theo địa chỉ API — giữ dạng cũ, thử lại lần sau",
			"service", "comms", "xa", string(tenant.MustFrom(ctx)), "err", err)
		return key, nil
	}
	now := time.Now().UTC()
	err = db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		swapped, err := repo.ResealKey(ctx, tx, st.APIURL, sealed, resealed)
		if err != nil || !swapped {
			return err // !swapped: a save replaced the row meanwhile — nothing left to migrate
		}
		// NEVER THE KEY OR EITHER SEALED VALUE in the delta: only that the binding moved.
		return writePortalAudit(ctx, tx, systemPrincipal, ActionResealPortalKey, domain.PortalSyncSettingsSubject,
			now, map[string]any{"api_key_changed": false, "binding": "tenant+api_url"})
	})
	if err != nil {
		log.WarnContext(ctx, "CẢNH BÁO: không ghi được bản niêm mới của mã bảo mật Cổng — giữ dạng cũ, thử lại lần sau",
			"service", "comms", "xa", string(tenant.MustFrom(ctx)), "err", err)
	}
	return key, nil
}

// logOutboundRefused is the security event of an outbound destination this side refused (R2,
// 02/10/2026; skills/security-logging): event, outcome, commune, actor (the staff business code, or
// `system`), the error CLASS, the run id ("" outside a run) and which kind of call it was (`api`,
// `image`). NEVER the URL — its query carries the key — nor a host typed by staff, nor an article field.
func logOutboundRefused(ctx context.Context, log *slog.Logger, actor, runID, call string, err error) {
	log.WarnContext(ctx, "CẢNH BÁO BẢO MẬT: từ chối một địa chỉ gọi ra ngoài",
		"event", "outbound_url_refused", "outcome", "refused", "xa", string(tenant.MustFrom(ctx)),
		"actor", actor, "class", portal.ClassOf(err), "run_id", runID, "call", call)
}

// Run ticks until ctx is cancelled — once at start, then every interval. Manual runs started while it
// runs derive from ctx, so cancelling it interrupts them too (and they still finish their row).
func (r *PortalSyncRunner) Run(ctx context.Context) {
	r.mu.Lock()
	r.base = ctx
	r.mu.Unlock()
	r.Tick(ctx)
	t := time.NewTicker(r.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.Tick(ctx)
		}
	}
}

// Wait blocks until every manual run started from HTTP has finished, or timeout passes. Called on
// shutdown AFTER cancelling Run's context.
func (r *PortalSyncRunner) Wait(timeout time.Duration) bool {
	done := make(chan struct{})
	go func() { r.wg.Wait(); close(done) }()
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}

// Tick does one scheduler pass: the scheduler lock, the due communes, each in turn.
func (r *PortalSyncRunner) Tick(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	release, ok, err := r.locks.TryLockScheduler(ctx)
	if err != nil {
		r.log.WarnContext(ctx, "CẢNH BÁO: không thử được khoá đồng bộ Cổng, bỏ nhịp này", "service", "comms", "err", err)
		return
	}
	if !ok {
		return // another replica ticks
	}
	defer release()
	// THE TICK'S BUDGET (R1): the global lock is held at most this long. Every run below derives from
	// tctx, so a run that would outlive the tick is cut at its end like one that outlives its own budget.
	tctx, cancel := context.WithTimeout(ctx, r.tickBudget)
	defer cancel()
	ids, err := r.locks.DueCommunes(tctx)
	if err != nil {
		r.log.WarnContext(ctx, "CẢNH BÁO: không liệt kê được xã tới hạn đồng bộ Cổng", "service", "comms", "err", err)
		return
	}
	for i, id := range ids {
		if ctx.Err() != nil {
			return
		}
		if tctx.Err() != nil {
			// Not an error of any commune: they stay due and come first next tick (oldest last_run_at).
			r.log.WarnContext(ctx, "CẢNH BÁO: nhịp đồng bộ Cổng hết thời gian, các xã còn lại chờ nhịp sau",
				"service", "comms", "con_lai", len(ids)-i)
			return
		}
		r.runScheduled(tctx, id)
	}
}

// runScheduled runs one commune's due sync. Nothing it does can stop the next commune.
func (r *PortalSyncRunner) runScheduled(ctx context.Context, id tenant.ID) {
	defer func() {
		if p := recover(); p != nil {
			r.log.ErrorContext(ctx, "LỖI: đồng bộ Cổng của một xã bị panic, chuyển sang xã kế", "service", "comms",
				"xa", string(id), "panic", fmt.Sprint(p))
		}
	}()
	cctx := tenant.Into(ctx, id)
	if r.registry != nil {
		t, ok, err := r.registry.XaTrongNguCanh(cctx)
		if err != nil {
			r.log.WarnContext(cctx, "CẢNH BÁO: không đọc được trạng thái xã, bỏ xã này ở nhịp này", "service", "comms",
				"xa", string(id), "err", err)
			return
		}
		if !ok || !t.Active {
			return
		}
	}
	// A RUN SLOT FIRST (R1), waited for inside the tick's budget: manual runs may hold every slot.
	select {
	case r.slots <- struct{}{}:
		defer func() { <-r.slots }()
	case <-ctx.Done():
		return
	}
	release, ok, err := r.locks.TryLockCommune(cctx, id)
	if err != nil {
		r.log.WarnContext(cctx, "CẢNH BÁO: không thử được khoá đồng bộ của xã", "service", "comms", "xa", string(id), "err", err)
		return
	}
	if !ok {
		return // a run is in progress for this commune
	}
	defer release()
	// RE-CHECKED UNDER THE LOCK: another replica may have started (and finished) a run between the list
	// and the lock, and `last_run_at` says so.
	st, sealed, err := r.repo.SettingsWithKey(cctx)
	if err != nil {
		if !errors.Is(err, commsstore.ErrPortalSyncSettingsNotFound) {
			r.log.WarnContext(cctx, "CẢNH BÁO: không đọc được cấu hình đồng bộ Cổng", "service", "comms", "xa", string(id), "err", err)
		}
		return
	}
	if !domain.PortalSyncDue(st, r.now()) {
		return
	}
	run, err := r.begin(cctx, domain.PortalRunScheduled, systemPrincipal)
	if err != nil {
		r.log.WarnContext(cctx, "CẢNH BÁO: không mở được lượt đồng bộ Cổng", "service", "comms", "xa", string(id), "err", err)
		return
	}
	r.execute(cctx, run, st, sealed)
}

// StartManual is `⟳ Đồng bộ ngay`: the run row is written before this returns, the work happens in the
// background (minutes of network I/O no request should wait on), and the lock is held until it ends.
// The commune's sync need not be enabled: staff pressing the button is the decision.
func (r *PortalSyncRunner) StartManual(ctx context.Context, actor audit.Actor) (domain.PortalSyncRun, error) {
	if actor.ID == "" || actor.ID == audit.SystemActor {
		return domain.PortalSyncRun{}, ErrPortalMissingActor
	}
	r.mu.Lock()
	base := r.base
	r.mu.Unlock()
	if base == nil || base.Err() != nil {
		return domain.PortalSyncRun{}, ErrPortalRunnerStopped
	}
	id := tenant.MustFrom(ctx)
	release, ok, err := r.locks.TryLockCommune(ctx, id)
	if err != nil {
		return domain.PortalSyncRun{}, fmt.Errorf("dong_bo_cong: khoá lượt chạy cho xã %s: %w", id, err)
	}
	if !ok {
		return domain.PortalSyncRun{}, ErrPortalRunInProgress
	}
	// A RUN SLOT, NOT WAITED FOR (R1): a request does not queue behind other communes' runs.
	select {
	case r.slots <- struct{}{}:
	default:
		release()
		return domain.PortalSyncRun{}, ErrPortalRunnerBusy
	}
	releaseAll := func() { <-r.slots; release() }
	st, sealed, err := r.repo.SettingsWithKey(ctx)
	if err != nil {
		releaseAll()
		return domain.PortalSyncRun{}, err
	}
	if r.envelope == nil {
		releaseAll()
		return domain.PortalSyncRun{}, crypto.ErrNotConfigured
	}
	run, err := r.begin(ctx, domain.PortalRunManual, actor)
	if err != nil {
		releaseAll()
		return domain.PortalSyncRun{}, err
	}
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		defer releaseAll()
		defer func() {
			if p := recover(); p != nil {
				r.log.Error("LỖI: lượt đồng bộ Cổng chạy tay bị panic", "service", "comms", "xa", string(id),
					"run_id", run.ID, "panic", fmt.Sprint(p))
			}
		}()
		// THE COMMUNE TRAVELS IN THE UNIT OF WORK (rule 1, invariant 9), on the runner's context — the
		// request's context ends with the 202.
		r.execute(tenant.Into(base, id), run, st, sealed)
	}()
	return run, nil
}

// begin is step 2: reap, insert, mark, audit — one transaction.
func (r *PortalSyncRunner) begin(ctx context.Context, trigger string, actor audit.Actor) (domain.PortalSyncRun, error) {
	runID, err := r.newID()
	if err != nil {
		return domain.PortalSyncRun{}, fmt.Errorf("dong_bo_cong: sinh mã lượt chạy: %w", err)
	}
	now := r.now()
	run := domain.PortalSyncRun{ID: runID, TriggerKind: trigger, Actor: actor.ID, StartedAt: now}
	err = r.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		reaped, err := r.repo.ReapStuckRuns(ctx, tx, now.Add(-portalStuckAfter), now)
		if err != nil {
			return err
		}
		for _, id := range reaped {
			// A crashed run's one fill is a system act (rule 6, invariant 6), recorded as one.
			if err := writePortalAudit(ctx, tx, systemPrincipal, ActionFinishPortalSync, runSubject(now), now,
				map[string]any{"run_id": id, "outcome": domain.PortalRunFailed, "reason": domain.PortalRunErrorInterrupted}); err != nil {
				return err
			}
		}
		if err := r.repo.InsertRun(ctx, tx, run); err != nil {
			return err
		}
		if err := r.repo.MarkRunStarted(ctx, tx, now); err != nil {
			return err
		}
		return writePortalAudit(ctx, tx, actor, ActionStartPortalSync, runSubject(now), now,
			map[string]any{"run_id": runID, "trigger_kind": trigger, "reaped_run_ids": reaped})
	})
	if err != nil {
		return domain.PortalSyncRun{}, fmt.Errorf("dong_bo_cong: mở lượt chạy cho xã %s: %w", tenant.MustFrom(ctx), err)
	}
	return run, nil
}

// runSubject is the audit locator of a run: a run has no business code, so it is the day it started;
// the delta carries the run id.
func runSubject(at time.Time) string {
	return "dong-bo-cong/luot-chay/" + at.UTC().Format("2006-01-02")
}

func writePortalAudit(ctx context.Context, tx *store.ScopedTx, actor audit.Actor, action, subject string,
	at time.Time, d map[string]any) error {
	delta, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("dong_bo_cong: mã hoá delta: %w", err)
	}
	// SAME TRANSACTION AS THE WRITE (rule 6, invariant 3); TenantID filled by audit.Write from the tx.
	return audit.Write(ctx, tx, audit.Entry{Actor: actor, Action: action, Subject: subject, At: at, Delta: delta})
}

// --- the run itself ------------------------------------------------------------------------------------

// runTally accumulates what a run did, safe for the concurrent stages.
type runTally struct {
	mu               sync.Mutex
	counts           domain.PortalRunCounts
	categoriesRead   int
	categoriesFailed int
	notes            map[noteKey]*domain.PortalRunError
	order            []noteKey
	// heldSeen deduplicates the skipped counts across categories: an article filed under two categories
	// is one skip. Short ids only, bounded by the rows the portal returned in the window.
	heldSeen map[string]bool
}

type noteKey struct{ categoryID, class string }

// note records one error class, aggregated per (category, class). Never an article's text.
func (t *runTally) note(cat domain.PortalCategory, class string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.notes == nil {
		t.notes = map[noteKey]*domain.PortalRunError{}
	}
	k := noteKey{cat.ExternalID, class}
	if e, ok := t.notes[k]; ok {
		e.Count++
		return
	}
	if len(t.order) >= portalErrorSummaryMax {
		return
	}
	t.notes[k] = &domain.PortalRunError{CategoryExternalID: cat.ExternalID, CategoryName: cat.Name, Error: class, Count: 1}
	t.order = append(t.order, k)
}

func (t *runTally) summary() []domain.PortalRunError {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]domain.PortalRunError, 0, len(t.order))
	for _, k := range t.order {
		out = append(out, *t.notes[k])
	}
	return out
}

// candidate is one article chosen for import, with the category it came under.
type candidate struct {
	cat     domain.PortalCategory
	article portal.Article
	extID   string // domain.PortalExternalItemID
	itemID  string
	cover   *portalCover
	// settled: the import committed, so the cover's row owns its object. An unsettled cover's private
	// derivative is discarded at the end of the run (R7).
	settled bool
}

// execute is steps 3–7. It ALWAYS ends with the run's finish fill (or a logged failure to write it).
//
// THE RUN'S BUDGET (R1) is a deadline on the work, not on the finish: a run cut by it — or by the tick's
// budget it derives from — ends with the class `time-budget`; one cut by a shutdown, `interrupted`.
func (r *PortalSyncRunner) execute(ctx context.Context, run domain.PortalSyncRun, st domain.PortalSyncSettings, sealed []byte) {
	t := &runTally{}
	wctx, cancel := context.WithTimeout(ctx, r.runBudget)
	defer cancel()
	defer func() {
		if p := recover(); p != nil {
			r.log.ErrorContext(ctx, "LỖI: lượt đồng bộ Cổng bị panic", "service", "comms", "run_id", run.ID, "panic", fmt.Sprint(p))
			t.note(domain.PortalCategory{}, "internal")
			r.finish(ctx, run, t, nil)
		}
	}()
	r.work(wctx, run, st, sealed, t)
	r.finish(ctx, run, t, wctx.Err())
}

func (r *PortalSyncRunner) work(ctx context.Context, run domain.PortalSyncRun, st domain.PortalSyncSettings,
	sealed []byte, t *runTally) {

	// D1 (owner, 02/10/2026): a row saved above today's ceilings runs AT the ceilings; the row is not
	// rewritten (domain.PortalSyncSettings.Clamped).
	if clamped, lowered := st.Clamped(); lowered {
		r.log.InfoContext(ctx, "cấu hình đồng bộ Cổng vượt trần — lượt này chạy theo trần", "service", "comms",
			"xa", string(tenant.MustFrom(ctx)), "run_id", run.ID, "so_ngay", clamped.WindowDays, "so_tin", clamped.MaxItemsPerRun)
		st = clamped
	}
	base, err := portal.ParseBase(st.APIURL)
	if err != nil {
		if portal.IsRefusal(err) {
			logOutboundRefused(ctx, r.log, run.Actor, run.ID, "api", err)
		}
		t.note(domain.PortalCategory{}, portal.ClassOf(err))
		return
	}
	if r.envelope == nil {
		t.note(domain.PortalCategory{}, "encryption-not-configured")
		return
	}
	key, err := openPortalKey(ctx, r.db, r.repo, r.envelope, st, sealed, r.log)
	if err != nil {
		t.note(domain.PortalCategory{}, "credential-unavailable")
		return
	}
	defer clear(key)
	ep := portal.Endpoint{Base: base, Key: key}

	all, err := r.repo.Categories(ctx)
	if err != nil {
		t.note(domain.PortalCategory{}, "store")
		return
	}
	var cats []domain.PortalCategory
	for _, c := range all {
		if c.IsSelected {
			cats = append(cats, c)
		}
	}
	if len(cats) == 0 {
		t.note(domain.PortalCategory{}, domain.PortalRunErrorNoCategory)
		return
	}
	if len(cats) > domain.PortalSelectedCategoriesMax {
		// Selected before the ceiling existed (D1): the first ones in the store's order (name) run; the
		// summary says the rest were not read, so the screen can tell staff to untick some.
		t.note(domain.PortalCategory{}, domain.PortalRunErrorCategoriesOverCeiling)
		cats = cats[:domain.PortalSelectedCategoriesMax]
	}

	since := r.now().Add(-time.Duration(st.WindowDays) * 24 * time.Hour)
	batches := r.fetchCategories(ctx, run, ep, cats, since, st.MaxItemsPerRun, t)
	chosen := r.choose(ctx, cats, batches, st.MaxItemsPerRun, t)
	if len(chosen) == 0 || ctx.Err() != nil {
		return
	}
	// R7: whatever does not end imported — a failed or raced transaction, a refused item, a run cut by
	// its budget before reaching it — leaves its private derivative behind unless discarded here.
	defer r.discardUnsettled(ctx, run, chosen)
	r.fetchImages(ctx, run, ep, chosen, t)
	for i := range chosen {
		if ctx.Err() != nil {
			return
		}
		r.importOne(ctx, run, st, base, &chosen[i], t)
	}
}

// fetchCategories is step 3, three categories at a time. A failure is recorded per category.
func (r *PortalSyncRunner) fetchCategories(ctx context.Context, run domain.PortalSyncRun, ep portal.Endpoint,
	cats []domain.PortalCategory, since time.Time, keep int, t *runTally) [][]portal.Article {

	out := make([][]portal.Article, len(cats))
	gate := make(chan struct{}, portalCategoryConcurrency)
	var wg sync.WaitGroup
	for i, c := range cats {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case gate <- struct{}{}:
				defer func() { <-gate }()
			case <-ctx.Done():
				return
			}
			b, err := r.client.Articles(ctx, ep, c.ExternalID, since, keep, r.heldFunc(t))
			t.mu.Lock()
			if err != nil {
				t.categoriesFailed++
			} else {
				t.categoriesRead++
				t.counts.Fetched += b.Read
			}
			t.mu.Unlock()
			if err != nil {
				if portal.IsRefusal(err) {
					logOutboundRefused(ctx, r.log, run.Actor, run.ID, "api", err)
				}
				// The CLASS only: the adapter's errors carry nothing else, and this is the run log.
				t.note(c, portal.ClassOf(err))
				r.log.WarnContext(ctx, "CẢNH BÁO: đọc một chuyên mục Cổng không được", "service", "comms",
					"xa", string(tenant.MustFrom(ctx)), "chuyen_muc", c.ExternalID, "loi", portal.ClassOf(err))
				return
			}
			out[i] = b.Articles
		}()
	}
	wg.Wait()
	return out
}

// heldFunc is the adapter's register lookup for one run: portal ids → namespaced external ids, ONE
// tenant-scoped ExistingPortalItems per chunk, COUNTING SOFT-DELETED ITEMS (a removed article is never
// imported again — ADR 0067 §2 "Ghi" #2). It also counts the skips, once per article across categories.
// A store error is logged here (the adapter keeps only the class) and fails that category.
func (r *PortalSyncRunner) heldFunc(t *runTally) portal.HeldFunc {
	return func(ctx context.Context, ids []string) (map[string]bool, error) {
		ext := make([]string, len(ids))
		for i, id := range ids {
			ext[i] = domain.PortalExternalItemID(domain.PortalProviderCityShared, id)
		}
		have, err := r.repo.ExistingPortalItems(ctx, ext)
		if err != nil {
			r.log.WarnContext(ctx, "CẢNH BÁO: không kiểm được tin Cổng đã có", "service", "comms",
				"xa", string(tenant.MustFrom(ctx)), "err", err)
			return nil, err
		}
		out := make(map[string]bool, len(have))
		t.mu.Lock()
		defer t.mu.Unlock()
		if t.heldSeen == nil {
			t.heldSeen = map[string]bool{}
		}
		for i, id := range ids {
			deleted, ok := have[ext[i]]
			if !ok {
				continue
			}
			out[id] = deleted
			if t.heldSeen[ext[i]] {
				continue
			}
			t.heldSeen[ext[i]] = true
			if deleted {
				t.counts.SkippedDeleted++
			} else {
				t.counts.SkippedExisting++
			}
		}
		return out, nil
	}
}

// choose is step 4: merge newest first (ties keep selection order), deduplicate by portal id across
// categories (the first category in selection order wins), and keep at most max. The register was
// already asked per chunk by heldFunc, BEFORE the heap: nothing here is held, soft-deleted or live (a
// concurrent insert is still caught by the unique key at import — ErrPortalItemExists).
func (r *PortalSyncRunner) choose(ctx context.Context, cats []domain.PortalCategory, batches [][]portal.Article,
	max int, t *runTally) []candidate {

	var all []candidate
	seen := map[string]bool{}
	for i, arts := range batches {
		for _, a := range arts {
			ext := domain.PortalExternalItemID(domain.PortalProviderCityShared, a.ExternalID)
			if seen[ext] {
				continue
			}
			seen[ext] = true
			all = append(all, candidate{cat: cats[i], article: a, extID: ext})
		}
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].article.PublishedAt.After(all[j].article.PublishedAt) })
	if len(all) == 0 {
		return nil
	}
	var chosen []candidate
	for _, c := range all {
		if len(chosen) >= max {
			break // the ceiling: the rest wait for the next run
		}
		id, err := r.newID()
		if err != nil {
			t.mu.Lock()
			t.counts.Failed++
			t.mu.Unlock()
			t.note(c.cat, "item-id")
			continue
		}
		c.itemID = id
		chosen = append(chosen, c)
	}
	return chosen
}

// fetchImages is step 5, six at a time. Every failure is noted and the article goes on without one.
func (r *PortalSyncRunner) fetchImages(ctx context.Context, run domain.PortalSyncRun, ep portal.Endpoint,
	chosen []candidate, t *runTally) {
	if r.covers == nil {
		for _, c := range chosen {
			if c.article.ImageRef != "" {
				t.note(c.cat, "image-storage-unavailable")
			}
		}
		return
	}
	limit, limitErr := r.covers.portalImageLimit(ctx)
	gate := make(chan struct{}, portalImageConcurrency)
	var wg sync.WaitGroup
	for i := range chosen {
		c := &chosen[i]
		if c.article.ImageRef == "" {
			continue
		}
		u, err := portal.ResolveImage(ep.Base, c.article.ImageRef)
		if err != nil {
			if portal.IsRefusal(err) {
				logOutboundRefused(ctx, r.log, run.Actor, run.ID, "image", err)
			}
			t.note(c.cat, portal.ClassOf(err))
			continue
		}
		if limitErr != nil {
			t.note(c.cat, portalCoverRejectReason(limitErr))
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case gate <- struct{}{}:
				defer func() { <-gate }()
			case <-ctx.Done():
				return
			}
			data, err := r.client.Image(ctx, ep, u, limit)
			if err != nil {
				if portal.IsRefusal(err) {
					logOutboundRefused(ctx, r.log, run.Actor, run.ID, "image", err)
				}
				t.note(c.cat, "image-"+portal.ClassOf(err))
				return
			}
			pc, err := r.covers.preparePortalCover(ctx, c.itemID, data, r.now())
			if err != nil {
				t.note(c.cat, portalCoverRejectReason(err))
				return
			}
			c.cover = &pc
		}()
	}
	wg.Wait()
}

// importOne is step 6 for one article: one transaction, one audit entry, never an UPDATE.
func (r *PortalSyncRunner) importOne(ctx context.Context, run domain.PortalSyncRun, st domain.PortalSyncSettings,
	base *url.URL, c *candidate, t *runTally) {

	fail := func(class string) {
		t.mu.Lock()
		t.counts.Failed++
		t.mu.Unlock()
		t.note(c.cat, class)
	}
	n, class := buildPortalItem(c, st, base, r.now())
	if class != "" {
		fail(class)
		return
	}

	copied := false
	err := r.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		if c.cover != nil {
			if err := r.covers.recordPortalCover(ctx, tx, *c.cover); err != nil {
				return err
			}
		}
		if err := r.repo.InsertSyncedItem(ctx, tx, n, c.cat.ID); err != nil {
			return err
		}
		after := tomTatNoiDungMiniApp(n)
		if c.cover != nil && n.HienChoDan() {
			var err error
			if copied, err = r.covers.publishPortalCover(ctx, tx, n, n.TaoLuc); err != nil {
				return err
			}
			after["cover_published_file_id"] = n.CoverImageFileID
		}
		d := map[string]any{
			"sau": after, "run_id": run.ID, "portal_category_id": c.cat.ID,
			"portal_category_external_id": c.cat.ExternalID, "nguon_id_ngoai": n.NguonIDNgoai,
		}
		if c.cover != nil {
			d["anh_nguon_sha256"] = c.cover.sourceSHA256
		}
		// The SYSTEM principal for every import, scheduled or manual: the article is the portal's, the
		// act is the run's; who pressed the button is on the run row and its start entry.
		return writePortalAudit(ctx, tx, systemPrincipal, ActionImportPortalItem, chuDeNoiDungMiniApp(n), n.TaoLuc, d)
	})
	if err == nil {
		c.settled = true
		t.mu.Lock()
		t.counts.Imported++
		t.mu.Unlock()
		return
	}
	if copied && c.cover != nil {
		if werr := r.covers.withdrawPortalCover(context.WithoutCancel(ctx), c.cover.file); werr != nil {
			r.log.WarnContext(ctx, "ảnh bìa: nhập tin không chốt, gỡ bản công khai vừa chép cũng hỏng — bản sao mồ côi",
				"service", "comms", "xa", string(tenant.MustFrom(ctx)), "tep_id", c.cover.file.ID, "err", werr)
		}
	}
	if errors.Is(err, commsstore.ErrPortalItemExists) {
		t.mu.Lock()
		t.counts.SkippedExisting++
		t.mu.Unlock()
		return
	}
	r.log.WarnContext(ctx, "CẢNH BÁO: nhập một tin từ Cổng không được", "service", "comms",
		"xa", string(tenant.MustFrom(ctx)), "run_id", run.ID, "chuyen_muc", c.cat.ExternalID, "err", err)
	fail("item-store")
}

// buildPortalItem makes the row an article becomes, or names why it cannot (a REFUSAL, never a
// truncation — domain's bounds: an article silently cut is one whose meaning changed).
func buildPortalItem(c *candidate, st domain.PortalSyncSettings, base *url.URL, now time.Time) (domain.NoiDungMiniApp, string) {
	a := c.article
	if utf8.RuneCountInString(a.Title) > domain.TieuDeNoiDungToiDa {
		return domain.NoiDungMiniApp{}, "item-title-too-long"
	}
	if utf8.RuneCountInString(a.Summary) > domain.TomTatNoiDungToiDa {
		return domain.NoiDungMiniApp{}, "item-summary-too-long"
	}
	body := a.BodyHTML
	if st.KeepSourceCredit {
		label := a.SourceLabel
		if label == "" {
			label = base.Hostname()
		}
		body += domain.PortalSourceCredit(label)
	}
	// THE SANITISER, on this write path like on every other (ADR 0067 §1 decision 2) — the NARROW portal
	// policy, deliberately not SanitizeStaff: portal images live on every host, and taking them is H4's
	// later batch (ADR 0067 §Sửa đổi 03/10/2026, stop condition 2).
	body = richtext.Sanitize(body)
	if utf8.RuneCountInString(body) > domain.ThanNoiDungToiDa {
		return domain.NoiDungMiniApp{}, "item-body-too-long"
	}
	// ngay_dang is the portal's own publication DATE, in the portal's zone (0006: it "differs for an
	// article backdated to the day the portal published it").
	local := a.PublishedAt.In(portal.PortalLocation())
	n := domain.NoiDungMiniApp{
		ID: c.itemID, Loai: domain.LoaiNoiDung(c.cat.TargetKind), TieuDe: a.Title, TomTat: a.Summary, NoiDung: body,
		NgayDang:     time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC),
		Nguon:        domain.NguonDongBoCong,
		NguonIDNgoai: c.extID,
		NguoiTaoMa:   audit.SystemActor,
		TaoLuc:       now, CapNhatLuc: now,
	}
	if st.PublishMode == domain.PortalPublishDirect {
		n.TrangThai = domain.TrangThaiDangHien
		// G1: first published NOW, by this act (migration 0011: owed by the write path at INSERT).
		n.PublishedAt = now
	} else {
		n.TrangThai = domain.TrangThaiChoDuyet
	}
	if c.cover != nil {
		n.CoverImageFileID = c.cover.file.ID
	}
	return n, ""
}

// discardUnsettled is R7 (02/10/2026): every prepared cover whose import did not commit has its PRIVATE
// derivative deleted — after a check that no stored_file row claims it (an ambiguous commit that did
// land keeps its object). On a context the run's budget or a shutdown cannot cancel.
//
// A FAILED DELETE IS LOGGED BY OBJECT KEY, as a structured event the purge worker of ADR 0052 §6 can
// act on when it exists (ADR 0067 C5). The key names a commune, a date and two random ids — no person.
func (r *PortalSyncRunner) discardUnsettled(ctx context.Context, run domain.PortalSyncRun, chosen []candidate) {
	if r.covers == nil {
		return
	}
	dctx := context.WithoutCancel(ctx)
	for i := range chosen {
		c := &chosen[i]
		if c.cover == nil || c.settled {
			continue
		}
		if _, err := r.covers.discardPortalCover(dctx, c.cover.file); err != nil {
			r.log.WarnContext(dctx, "CẢNH BÁO: không xoá được ảnh dẫn xuất của tin không nhập — đối tượng mồ côi chờ dọn",
				"event", "orphan_object", "service", "comms", "xa", string(tenant.MustFrom(dctx)), "run_id", run.ID,
				"tep_id", c.cover.file.ID, "object_key", c.cover.file.ObjectKey, "err", err)
		}
	}
}

// finish is step 7: the run's one fill and its audit entry, on a context a shutdown cannot cancel —
// an interrupted run is still recorded as what it was.
//
// stopped is why the work stopped early (its context's error) or nil: DeadlineExceeded is the run's or
// the tick's budget (`time-budget`), anything else a shutdown (`interrupted`). Both lower the outcome.
func (r *PortalSyncRunner) finish(ctx context.Context, run domain.PortalSyncRun, t *runTally, stopped error) {
	fctx := context.WithoutCancel(ctx)
	interrupted := stopped != nil
	switch {
	case errors.Is(stopped, context.DeadlineExceeded):
		t.note(domain.PortalCategory{}, domain.PortalRunErrorTimeBudget)
	case interrupted:
		t.note(domain.PortalCategory{}, domain.PortalRunErrorInterrupted)
	}
	t.mu.Lock()
	counts, read, failed := t.counts, t.categoriesRead, t.categoriesFailed
	t.mu.Unlock()
	run.FinishedAt = r.now()
	if run.FinishedAt.Before(run.StartedAt) {
		run.FinishedAt = run.StartedAt
	}
	run.Outcome = domain.PortalRunOutcome(read, failed, counts, interrupted)
	run.Counts = counts
	run.Errors = t.summary()
	err := r.db.For(fctx).Tx(fctx, func(tx *store.ScopedTx) error {
		if err := r.repo.FinishRun(fctx, tx, run); err != nil {
			return err
		}
		actor := systemPrincipal
		return writePortalAudit(fctx, tx, actor, ActionFinishPortalSync, runSubject(run.StartedAt), run.FinishedAt,
			map[string]any{"run_id": run.ID, "outcome": run.Outcome, "fetched_count": counts.Fetched,
				"imported_count": counts.Imported, "skipped_existing_count": counts.SkippedExisting,
				"skipped_deleted_count": counts.SkippedDeleted, "failed_count": counts.Failed})
	})
	if err != nil {
		r.log.ErrorContext(fctx, "LỖI: không ghi được kết thúc lượt đồng bộ Cổng — lượt sẽ được đóng `that-bai` sau 1 giờ",
			"service", "comms", "xa", string(tenant.MustFrom(fctx)), "run_id", run.ID, "err", err)
		return
	}
	// Counts and classes only (rule 3).
	r.log.InfoContext(fctx, "lượt đồng bộ Cổng xong", "service", "comms", "xa", string(tenant.MustFrom(fctx)),
		"run_id", run.ID, "kich_hoat", run.TriggerKind, "ket_qua", run.Outcome, "doc", counts.Fetched,
		"nhap", counts.Imported, "bo_qua_da_co", counts.SkippedExisting, "bo_qua_da_xoa", counts.SkippedDeleted,
		"loi", counts.Failed)
}
