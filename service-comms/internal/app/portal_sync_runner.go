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
// WHAT IS NEVER LOGGED: an article's title, summary, body or URL (they name people — rule 3), the
// api_url's query, the key. Logs carry the commune id, the run id, counts and error classes.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"sort"
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
)

var (
	// ErrPortalRunInProgress — the commune's run lock is held: a run is in progress somewhere. 409.
	ErrPortalRunInProgress = errors.New("dong_bo_cong: đang có một lượt đồng bộ của xã")
	// ErrPortalRunnerStopped — the runner is not running (shutting down, or not started). 503.
	ErrPortalRunnerStopped = errors.New("dong_bo_cong: bộ chạy đồng bộ chưa sẵn sàng")
)

// The verbs in the trail (ADR 0011: Vietnamese snake_case values).
const (
	ActionStartPortalSync  = "bat_dau_dong_bo_cong"
	ActionFinishPortalSync = "ket_thuc_dong_bo_cong"
	ActionImportPortalItem = "nhap_tin_tu_cong"
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
	Articles(ctx context.Context, e portal.Endpoint, categoryID string, since time.Time) (portal.ArticleBatch, error)
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
		now: func() time.Time { return time.Now().UTC() }, newID: ulid.Moi}
	if d.Covers != nil {
		r.covers = d.Covers
	}
	return r, nil
}

// portalKeyAAD binds the sealed key to THIS table, column and commune — 0013's binding string, the
// shape mail_settings uses. Copied onto another row, it does not open.
func portalKeyAAD(ctx context.Context) []byte {
	return []byte("portal_sync_settings/api_key_sealed/" + string(tenant.MustFrom(ctx)))
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
	ids, err := r.locks.DueCommunes(ctx)
	if err != nil {
		r.log.WarnContext(ctx, "CẢNH BÁO: không liệt kê được xã tới hạn đồng bộ Cổng", "service", "comms", "err", err)
		return
	}
	for _, id := range ids {
		if ctx.Err() != nil {
			return
		}
		r.runScheduled(ctx, id)
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
	st, sealed, err := r.repo.SettingsWithKey(ctx)
	if err != nil {
		release()
		return domain.PortalSyncRun{}, err
	}
	if r.envelope == nil {
		release()
		return domain.PortalSyncRun{}, crypto.ErrNotConfigured
	}
	run, err := r.begin(ctx, domain.PortalRunManual, actor)
	if err != nil {
		release()
		return domain.PortalSyncRun{}, err
	}
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		defer release()
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
}

// execute is steps 3–7. It ALWAYS ends with the run's finish fill (or a logged failure to write it).
func (r *PortalSyncRunner) execute(ctx context.Context, run domain.PortalSyncRun, st domain.PortalSyncSettings, sealed []byte) {
	t := &runTally{}
	defer func() {
		if p := recover(); p != nil {
			r.log.ErrorContext(ctx, "LỖI: lượt đồng bộ Cổng bị panic", "service", "comms", "run_id", run.ID, "panic", fmt.Sprint(p))
			t.note(domain.PortalCategory{}, "internal")
			r.finish(ctx, run, t, false)
		}
	}()
	r.work(ctx, run, st, sealed, t)
	r.finish(ctx, run, t, ctx.Err() != nil)
}

func (r *PortalSyncRunner) work(ctx context.Context, run domain.PortalSyncRun, st domain.PortalSyncSettings,
	sealed []byte, t *runTally) {

	base, err := portal.ParseBase(st.APIURL)
	if err != nil {
		t.note(domain.PortalCategory{}, portal.ClassOf(err))
		return
	}
	if r.envelope == nil {
		t.note(domain.PortalCategory{}, "encryption-not-configured")
		return
	}
	key, err := r.envelope.Open(ctx, sealed, portalKeyAAD(ctx))
	if err != nil {
		t.note(domain.PortalCategory{}, "credential-unavailable")
		return
	}
	defer clear(key)
	ep := portal.Endpoint{Base: base, Key: secret.Secret(key)}

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

	since := r.now().Add(-time.Duration(st.WindowDays) * 24 * time.Hour)
	batches := r.fetchCategories(ctx, ep, cats, since, t)
	chosen := r.choose(ctx, cats, batches, st.MaxItemsPerRun, t)
	if len(chosen) == 0 || ctx.Err() != nil {
		return
	}
	r.fetchImages(ctx, ep, chosen, t)
	for i := range chosen {
		if ctx.Err() != nil {
			return
		}
		r.importOne(ctx, run, st, base, &chosen[i], t)
	}
}

// fetchCategories is step 3, three categories at a time. A failure is recorded per category.
func (r *PortalSyncRunner) fetchCategories(ctx context.Context, ep portal.Endpoint, cats []domain.PortalCategory,
	since time.Time, t *runTally) [][]portal.Article {

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
			b, err := r.client.Articles(ctx, ep, c.ExternalID, since)
			t.mu.Lock()
			if err != nil {
				t.categoriesFailed++
			} else {
				t.categoriesRead++
				t.counts.Fetched += b.Read
			}
			t.mu.Unlock()
			if err != nil {
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

// choose is step 4: merge newest first (ties keep selection order), deduplicate by portal id across
// categories (the first category in selection order wins), check against the register COUNTING
// SOFT-DELETED ITEMS, and keep at most max new ones.
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
	ids := make([]string, len(all))
	for i, c := range all {
		ids[i] = c.extID
	}
	held, err := r.repo.ExistingPortalItems(ctx, ids)
	if err != nil {
		t.note(domain.PortalCategory{}, "store")
		return nil
	}
	var chosen []candidate
	for _, c := range all {
		if deleted, ok := held[c.extID]; ok {
			t.mu.Lock()
			if deleted {
				t.counts.SkippedDeleted++
			} else {
				t.counts.SkippedExisting++
			}
			t.mu.Unlock()
			continue
		}
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
func (r *PortalSyncRunner) fetchImages(ctx context.Context, ep portal.Endpoint, chosen []candidate, t *runTally) {
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
	// THE SANITISER, on this write path like on every other (ADR 0067 §1 decision 2).
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

// finish is step 7: the run's one fill and its audit entry, on a context a shutdown cannot cancel —
// an interrupted run is still recorded as what it was.
func (r *PortalSyncRunner) finish(ctx context.Context, run domain.PortalSyncRun, t *runTally, interrupted bool) {
	fctx := context.WithoutCancel(ctx)
	if interrupted {
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
