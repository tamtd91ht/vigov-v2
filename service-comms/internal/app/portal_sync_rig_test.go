package app

// The harness of the portal sync tests (portal_sync_runner_test.go, portal_sync_admin_test.go).
//
// WHAT RUNS FOR REAL: core/store's transactions and core/audit's INSERT over a recording driver (so
// "the write and its entry share a transaction" and "the delta holds no secret" are read off the
// statements), the real crypto.Envelope (testEnvelope, mail_settings_test.go), the real richtext
// sanitiser, and the real ContentCovers for the image tests. WHAT IS FAKED: the portal store (its SQL
// is PostgreSQL's to judge — see internal/store), the portal client (internal/portal has its own TLS
// suite), the locks, and — in the runner tests only — the cover pipeline, whose concurrency the cover
// fakes of content_cover_test.go were not written for.

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/crypto"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	"github.com/vihat/vigov/service-comms/internal/portal"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// portalTestKey is a fixture, distinctive so a leak is found by substring.
const portalTestKey = "fixture-portal-key-Z7WQ"

var portalNow = time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)

// --- a recording SQL driver -------------------------------------------------------------------------

type portalStmt struct {
	sql  string
	args []driver.Value
}

type portalSQL struct {
	mu                       sync.Mutex
	stmts                    []portalStmt
	begun, commits, rollback int
	failOn                   string
}

func (d *portalSQL) Connect(context.Context) (driver.Conn, error) { return &portalConn{d: d}, nil }
func (d *portalSQL) Driver() driver.Driver                        { return portalDriverOnly{} }

type portalDriverOnly struct{}

func (portalDriverOnly) Open(string) (driver.Conn, error) { return nil, errors.New("connector only") }

type portalConn struct{ d *portalSQL }

func (c *portalConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("no prepare") }
func (c *portalConn) Close() error                        { return nil }
func (c *portalConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c *portalConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.d.mu.Lock()
	c.d.begun++
	c.d.mu.Unlock()
	return &portalTx{d: c.d}, nil
}
func (c *portalConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	vals := make([]driver.Value, len(args))
	for i, a := range args {
		vals[i] = a.Value
	}
	c.d.mu.Lock()
	defer c.d.mu.Unlock()
	c.d.stmts = append(c.d.stmts, portalStmt{sql: q, args: vals})
	if c.d.failOn != "" {
		for _, v := range vals {
			if s, ok := v.(string); ok && s == c.d.failOn {
				return nil, errors.New("fake driver: built to fail")
			}
		}
	}
	return driver.RowsAffected(1), nil
}

type portalTx struct{ d *portalSQL }

func (t *portalTx) Commit() error {
	t.d.mu.Lock()
	t.d.commits++
	t.d.mu.Unlock()
	return nil
}
func (t *portalTx) Rollback() error {
	t.d.mu.Lock()
	t.d.rollback++
	t.d.mu.Unlock()
	return nil
}

// portalAudit is one audit_log INSERT as the driver saw it.
type portalAudit struct {
	tenant, actor, kind, action, subject string
	delta                                string
}

func (d *portalSQL) audits() []portalAudit {
	d.mu.Lock()
	defer d.mu.Unlock()
	var out []portalAudit
	for _, s := range d.stmts {
		if !strings.Contains(s.sql, "INSERT INTO audit_log") {
			continue
		}
		out = append(out, portalAudit{tenant: fmt.Sprint(s.args[0]), actor: fmt.Sprint(s.args[1]),
			kind: fmt.Sprint(s.args[2]), action: fmt.Sprint(s.args[4]), subject: fmt.Sprint(s.args[5]),
			delta: string(s.args[7].([]byte))})
	}
	return out
}

func (d *portalSQL) auditsOf(action string) []portalAudit {
	var out []portalAudit
	for _, a := range d.audits() {
		if a.action == action {
			out = append(out, a)
		}
	}
	return out
}

// --- fakes ------------------------------------------------------------------------------------------

type portalInsert struct {
	item    domain.NoiDungMiniApp
	catID   string
	commune tenant.ID
}

type portalUpsert struct {
	st     domain.PortalSyncSettings
	sealed []byte
	by     string
}

type fakePortalRepo struct {
	mu sync.Mutex

	settings *domain.PortalSyncSettings
	sealed   []byte
	cats     []domain.PortalCategory
	existing map[string]bool // nguon_id_ngoai → held by a soft-deleted item
	runs     page.Result[domain.PortalSyncRun]

	insertErr map[string]error // by nguon_id_ngoai
	reapIDs   []string

	upserts     []portalUpsert
	catInserts  []domain.PortalCategory
	catUpdates  []domain.PortalCategory
	inserted    []portalInsert
	runRows     []domain.PortalSyncRun
	finished    []domain.PortalSyncRun
	reapBefore  []time.Time
	markedStart []time.Time
	reseals     []portalReseal
}

type portalReseal struct {
	apiURL         string
	old, newSealed []byte
}

func (f *fakePortalRepo) Settings(context.Context) (domain.PortalSyncSettings, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.settings == nil {
		return domain.PortalSyncSettings{}, commsstore.ErrPortalSyncSettingsNotFound
	}
	return *f.settings, nil
}
func (f *fakePortalRepo) SettingsWithKey(ctx context.Context) (domain.PortalSyncSettings, []byte, error) {
	s, err := f.Settings(ctx)
	return s, f.sealed, err
}
func (f *fakePortalRepo) SettingsForUpdate(context.Context, *store.ScopedTx) (domain.PortalSyncSettings, []byte, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.settings == nil {
		return domain.PortalSyncSettings{}, nil, false, nil
	}
	return *f.settings, f.sealed, true, nil
}
func (f *fakePortalRepo) UpsertSettings(_ context.Context, _ *store.ScopedTx, s domain.PortalSyncSettings,
	sealed []byte, by string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.upserts = append(f.upserts, portalUpsert{s, sealed, by})
	return nil
}
func (f *fakePortalRepo) MarkRunStarted(_ context.Context, _ *store.ScopedTx, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.markedStart = append(f.markedStart, at)
	return nil
}
func (f *fakePortalRepo) ResealKey(_ context.Context, _ *store.ScopedTx, apiURL string, oldSealed, newSealed []byte) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reseals = append(f.reseals, portalReseal{apiURL, oldSealed, newSealed})
	if f.settings == nil || f.settings.APIURL != apiURL || !bytes.Equal(f.sealed, oldSealed) {
		return false, nil
	}
	f.sealed = newSealed
	return true, nil
}
func (f *fakePortalRepo) Categories(context.Context) ([]domain.PortalCategory, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]domain.PortalCategory(nil), f.cats...), nil
}
func (f *fakePortalRepo) CategoriesForUpdate(ctx context.Context, _ *store.ScopedTx) ([]domain.PortalCategory, error) {
	return f.Categories(ctx)
}
func (f *fakePortalRepo) InsertCategory(_ context.Context, _ *store.ScopedTx, c domain.PortalCategory, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.catInserts = append(f.catInserts, c)
	return nil
}
func (f *fakePortalRepo) UpdateCategory(_ context.Context, _ *store.ScopedTx, c domain.PortalCategory, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.catUpdates = append(f.catUpdates, c)
	return nil
}
func (f *fakePortalRepo) Runs(context.Context, page.Request) (page.Result[domain.PortalSyncRun], error) {
	return f.runs, nil
}
func (f *fakePortalRepo) InsertRun(_ context.Context, _ *store.ScopedTx, r domain.PortalSyncRun) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runRows = append(f.runRows, r)
	return nil
}
func (f *fakePortalRepo) FinishRun(_ context.Context, _ *store.ScopedTx, r domain.PortalSyncRun) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.finished = append(f.finished, r)
	return nil
}
func (f *fakePortalRepo) ReapStuckRuns(_ context.Context, _ *store.ScopedTx, before, _ time.Time) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reapBefore = append(f.reapBefore, before)
	return f.reapIDs, nil
}
func (f *fakePortalRepo) ExistingPortalItems(_ context.Context, ids []string) (map[string]bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[string]bool{}
	for _, id := range ids {
		if d, ok := f.existing[id]; ok {
			out[id] = d
		}
	}
	return out, nil
}
func (f *fakePortalRepo) InsertSyncedItem(ctx context.Context, _ *store.ScopedTx, n domain.NoiDungMiniApp, catID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.insertErr[n.NguonIDNgoai]; err != nil {
		return err
	}
	f.inserted = append(f.inserted, portalInsert{item: n, catID: catID, commune: tenant.MustFrom(ctx)})
	return nil
}

type fakePortalClient struct {
	mu         sync.Mutex
	articles   map[string][]portal.Article
	errs       map[string]error
	since      []time.Time
	cats       []portal.Category
	catErr     error
	images     map[string][]byte
	imageCalls []string
	keys       []string // the key every call received
	keeps      []int    // the keep every Articles call received
	onArticles func()
}

func (c *fakePortalClient) Categories(_ context.Context, e portal.Endpoint) ([]portal.Category, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.keys = append(c.keys, string(e.Key.Lo()))
	return c.cats, c.catErr
}

// Articles emulates the adapter's contract: the held lookup BEFORE the slots, then the newest keep.
func (c *fakePortalClient) Articles(ctx context.Context, e portal.Endpoint, id string, since time.Time, keep int,
	held portal.HeldFunc) (portal.ArticleBatch, error) {
	c.mu.Lock()
	c.keys = append(c.keys, string(e.Key.Lo()))
	c.since = append(c.since, since)
	c.keeps = append(c.keeps, keep)
	hook := c.onArticles
	arts, err := c.articles[id], c.errs[id]
	c.mu.Unlock()
	if hook != nil {
		hook()
	}
	if err != nil {
		return portal.ArticleBatch{}, err
	}
	ids := make([]string, len(arts))
	for i, a := range arts {
		ids[i] = a.ExternalID
	}
	have, herr := held(ctx, ids)
	if herr != nil {
		return portal.ArticleBatch{}, portal.ErrHeldLookup
	}
	kept := make([]portal.Article, 0, len(arts))
	for _, a := range arts {
		if _, ok := have[a.ExternalID]; !ok {
			kept = append(kept, a)
		}
	}
	sort.SliceStable(kept, func(i, j int) bool { return kept[i].PublishedAt.After(kept[j].PublishedAt) })
	if len(kept) > keep {
		kept = kept[:keep]
	}
	return portal.ArticleBatch{Articles: kept, Read: len(arts) + 1}, nil
}
func (c *fakePortalClient) Image(_ context.Context, _ portal.Endpoint, u *url.URL, _ int64) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.imageCalls = append(c.imageCalls, u.String())
	if b, ok := c.images[u.String()]; ok {
		return b, nil
	}
	return nil, portal.ErrTimeout
}

type fakePortalLocks struct {
	mu                         sync.Mutex
	schedulerBusy, communeBusy bool
	due                        []tenant.ID
	dueAsked, released         int
	lockedCommunes             []tenant.ID
}

func (l *fakePortalLocks) DueCommunes(context.Context) ([]tenant.ID, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.dueAsked++
	return l.due, nil
}
func (l *fakePortalLocks) TryLockScheduler(context.Context) (func(), bool, error) {
	if l.schedulerBusy {
		return func() {}, false, nil
	}
	return l.release, true, nil
}
func (l *fakePortalLocks) TryLockCommune(_ context.Context, id tenant.ID) (func(), bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.communeBusy {
		return func() {}, false, nil
	}
	l.lockedCommunes = append(l.lockedCommunes, id)
	return l.release, true, nil
}
func (l *fakePortalLocks) release() {
	l.mu.Lock()
	l.released++
	l.mu.Unlock()
}

type fakePortalCovers struct {
	mu        sync.Mutex
	prepErr   error
	recorded  []string // file ids
	published []string
	withdrawn []string
	discarded []string
	recordErr error
}

func (c *fakePortalCovers) portalImageLimit(context.Context) (int64, error) { return 1 << 20, nil }
func (c *fakePortalCovers) preparePortalCover(_ context.Context, itemID string, _ []byte, now time.Time) (portalCover, error) {
	if c.prepErr != nil {
		return portalCover{}, c.prepErr
	}
	return portalCover{file: domain.StoredFile{ID: "F" + itemID, SubjectID: itemID, Status: domain.StoredFileReady,
		CreatedAt: now}, sourceSHA256: strings.Repeat("ab", 32)}, nil
}
func (c *fakePortalCovers) recordPortalCover(_ context.Context, _ *store.ScopedTx, pc portalCover) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.recordErr != nil {
		return c.recordErr
	}
	c.recorded = append(c.recorded, pc.file.ID)
	return nil
}
func (c *fakePortalCovers) discardPortalCover(_ context.Context, f domain.StoredFile) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.discarded = append(c.discarded, f.ID)
	return true, nil
}
func (c *fakePortalCovers) publishPortalCover(_ context.Context, _ *store.ScopedTx, n domain.NoiDungMiniApp,
	_ time.Time) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.published = append(c.published, n.CoverImageFileID)
	return true, nil
}
func (c *fakePortalCovers) withdrawPortalCover(_ context.Context, f domain.StoredFile) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.withdrawn = append(c.withdrawn, f.ID)
	return nil
}

// portalRegistry stands in for platform's commune directory.
type portalRegistry struct{ active bool }

// vi-name-ok: the method name of the existing core/platformclient.Directory this fake must match
func (p portalRegistry) XaTrongNguCanh(ctx context.Context) (tenant.Tenant, bool, error) {
	return tenant.Tenant{ID: tenant.MustFrom(ctx), Active: p.active}, true, nil
}

// --- the rig ----------------------------------------------------------------------------------------

type portalRig struct {
	sql    *portalSQL
	repo   *fakePortalRepo
	client *fakePortalClient
	locks  *fakePortalLocks
	covers *fakePortalCovers
	env    *crypto.Envelope
	runner *PortalSyncRunner
	admin  *PortalSyncAdmin
	ctx    context.Context
}

func portalArticle(id string, at time.Time) portal.Article {
	return portal.Article{ExternalID: id, Title: "Tin " + id, Summary: "Tóm tắt " + id,
		BodyHTML: "<p>Thân bài " + id + "</p>", PublishedAt: at}
}

func withImage(a portal.Article, ref string) portal.Article {
	a.ImageRef = ref
	return a
}

func newPortalRig(t *testing.T) *portalRig {
	t.Helper()
	d := &portalSQL{}
	db := sql.OpenDB(d)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	kho := store.New(db)

	env := testEnvelope(t)
	ctx := tenant.Into(context.Background(), xaA)
	st := domain.DefaultPortalSyncSettings()
	st.APIURL, st.IsEnabled, st.APIKeySet = "https://portal.example.gov.vn/api", true, true
	sealed, err := env.Seal(ctx, secret.Secret(portalTestKey), portalKeyAAD(ctx, st.APIURL))
	if err != nil {
		t.Fatal(err)
	}
	repo := &fakePortalRepo{settings: &st, sealed: sealed, existing: map[string]bool{},
		cats: []domain.PortalCategory{
			{ID: "CAT-NEWS", ExternalID: "120", Name: "Tin tức", TargetKind: "tin-tuc", IsSelected: true},
			{ID: "CAT-EVENT", ExternalID: "144", Name: "Sự kiện", TargetKind: "su-kien", IsSelected: true},
			{ID: "CAT-OFF", ExternalID: "200", Name: "Không lấy", TargetKind: "tin-tuc", IsSelected: false},
		}}
	client := &fakePortalClient{articles: map[string][]portal.Article{
		"120": {portalArticle("1", portalNow.Add(-2*time.Hour))},
		"144": {portalArticle("2", portalNow.Add(-1*time.Hour))},
	}, errs: map[string]error{}, images: map[string][]byte{}}
	locks := &fakePortalLocks{due: []tenant.ID{xaA}}
	covers := &fakePortalCovers{}

	runner, err := NewPortalSyncRunner(PortalSyncRunnerDeps{DB: kho, Repo: repo, Locks: locks, Client: client,
		Envelope: env, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	runner.covers = covers
	runner.now = func() time.Time { return portalNow }
	n := 0
	var mu sync.Mutex
	runner.newID = func() (string, error) {
		mu.Lock()
		defer mu.Unlock()
		n++
		return fmt.Sprintf("01JID%021d", n), nil
	}
	admin := NewPortalSyncAdmin(kho, repo, env, client, runner)
	admin.newID = runner.newID
	return &portalRig{sql: d, repo: repo, client: client, locks: locks, covers: covers, env: env,
		runner: runner, admin: admin, ctx: ctx}
}

func (r *portalRig) onlyFinished(t *testing.T) domain.PortalSyncRun {
	t.Helper()
	if len(r.repo.finished) != 1 {
		t.Fatalf("finished %d runs, want 1", len(r.repo.finished))
	}
	return r.repo.finished[0]
}

// assertNoPortalSecret: no audit delta carries the key or its sealed bytes.
func (r *portalRig) assertNoPortalSecret(t *testing.T) {
	t.Helper()
	for _, a := range r.sql.audits() {
		if strings.Contains(a.delta, portalTestKey) || strings.Contains(a.delta, "sealed") {
			t.Fatalf("audit delta carries the key: %s", a.delta)
		}
		if len(r.repo.sealed) > 0 && bytes.Contains([]byte(a.delta), r.repo.sealed) {
			t.Fatal("audit delta carries the sealed key")
		}
	}
}

// startedRunner marks the runner as running, as Run does, without its ticker.
func startedRunner(r *portalRig) {
	r.runner.mu.Lock()
	r.runner.base = context.Background()
	r.runner.mu.Unlock()
}

func intp(v int) *int    { return &v }
func boolp(v bool) *bool { return &v }
