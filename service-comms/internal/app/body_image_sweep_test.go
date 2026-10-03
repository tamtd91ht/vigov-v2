package app

// The abandoned-draft body-image sweep (body_image_sweep.go, ADR 0067 K11) over core/store's REAL
// transactions and core/audit's REAL INSERT on the recording driver of portal_sync_rig_test.go, with an
// in-memory stored_file / article model behind the repo — the SQL itself is the store's pg suite's to judge
// (internal/store/stored_file_pg_test.go). EACH CASE IS ONE FILE THE OWNER'S RULE MUST OR MUST NOT RETIRE.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

var sweepNow = time.Date(2026, 10, 20, 2, 0, 0, 0, time.UTC)

type sweepFile struct {
	tenant         tenant.ID
	id, subject    string
	purpose        string
	status         domain.StoredFileStatus
	completedAt    time.Time
	public         string
	deleted        bool
	deletedBy, why string
}

// fakeSweepRepo is stored_file + the article ids ever issued (soft-deleted articles INCLUDED), per commune.
type fakeSweepRepo struct {
	files    []*sweepFile
	articles map[tenant.ID]map[string]bool
	lockErr  error
}

func (f *fakeSweepRepo) abandoned(x *sweepFile, xa tenant.ID, purpose string, before time.Time) bool {
	return x.tenant == xa && x.purpose == purpose && x.status == domain.StoredFileReady && !x.deleted &&
		x.public == "" && x.completedAt.Before(before) && !f.articles[xa][x.subject]
}

func (f *fakeSweepRepo) LockAbandonedBodyImages(ctx context.Context, _ *store.ScopedTx, purpose string,
	before time.Time, limit int) ([]string, error) {
	if f.lockErr != nil {
		return nil, f.lockErr
	}
	xa := tenant.MustFrom(ctx)
	var out []string
	for _, x := range f.files {
		if f.abandoned(x, xa, purpose, before) && len(out) < limit {
			out = append(out, x.id)
		}
	}
	return out, nil
}

func (f *fakeSweepRepo) RetireAbandonedBodyImages(ctx context.Context, _ *store.ScopedTx, ids []string, purpose string,
	before time.Time, by, reason string, _ time.Time) ([]string, error) {
	xa := tenant.MustFrom(ctx)
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	var out []string
	for _, x := range f.files {
		if want[x.id] && f.abandoned(x, xa, purpose, before) {
			x.deleted, x.deletedBy, x.why = true, by, reason
			out = append(out, x.id)
		}
	}
	sort.Strings(out)
	return out, nil
}

// fakeSweepLocks lists the communes the model says have work — computed, so a commune is listed exactly
// when the per-commune read would find something.
type fakeSweepLocks struct {
	repo  *fakeSweepRepo
	busy  bool
	asked int
}

func (l *fakeSweepLocks) CommunesWithAbandonedBodyImages(_ context.Context, purpose, subjectType string,
	before time.Time) ([]tenant.ID, error) {
	l.asked++
	if subjectType != domain.StoredFileSubjectContentItem {
		return nil, errors.New("wrong subject type")
	}
	seen := map[tenant.ID]bool{}
	var out []tenant.ID
	for _, x := range l.repo.files {
		if !seen[x.tenant] && l.repo.abandoned(x, x.tenant, purpose, before) {
			seen[x.tenant] = true
			out = append(out, x.tenant)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

func (l *fakeSweepLocks) TryLockScheduler(context.Context) (func(), bool, error) {
	if l.busy {
		return func() {}, false, nil
	}
	return func() {}, true, nil
}

// sweepRegistry answers per commune.
type sweepRegistry struct{ inactive map[tenant.ID]bool }

// vi-name-ok: the method name of the existing core/platformclient.Directory this fake must match
func (r sweepRegistry) XaTrongNguCanh(ctx context.Context) (tenant.Tenant, bool, error) {
	id := tenant.MustFrom(ctx)
	return tenant.Tenant{ID: id, Active: !r.inactive[id]}, true, nil
}

type sweepRig struct {
	sql   *portalSQL
	repo  *fakeSweepRepo
	locks *fakeSweepLocks
	s     *BodyImageSweeper
}

func newSweepRig(t *testing.T, reg sweepRegistry) *sweepRig {
	t.Helper()
	d := &portalSQL{}
	db := sql.OpenDB(d)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	repo := &fakeSweepRepo{articles: map[tenant.ID]map[string]bool{}}
	locks := &fakeSweepLocks{repo: repo}
	s, err := NewBodyImageSweeper(BodyImageSweeperDeps{DB: store.New(db), Repo: repo, Locks: locks, Registry: reg,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return sweepNow }
	return &sweepRig{sql: d, repo: repo, locks: locks, s: s}
}

func (r *sweepRig) add(xa tenant.ID, id, subject string, age time.Duration, status domain.StoredFileStatus) *sweepFile {
	f := &sweepFile{tenant: xa, id: id, subject: subject, purpose: string(bodyImagePurpose), status: status,
		completedAt: sweepNow.Add(-age)}
	r.repo.files = append(r.repo.files, f)
	return f
}

const sweepDay = 24 * time.Hour

func TestBodyImageSweepRetiresOnlyAbandonedReadyImagesOlderThanSevenDays(t *testing.T) {
	r := newSweepRig(t, sweepRegistry{})
	old := r.add(xaA, "F-OLD", "ITEM-NEVER-SAVED", 8*sweepDay, domain.StoredFileReady)
	young := r.add(xaA, "F-YOUNG", "ITEM-NEVER-SAVED-2", 6*sweepDay, domain.StoredFileReady)
	saved := r.add(xaA, "F-SAVED", "ITEM-LIVE", 30*sweepDay, domain.StoredFileReady)
	ofDeleted := r.add(xaA, "F-OF-DELETED", "ITEM-SOFT-DELETED", 30*sweepDay, domain.StoredFileReady)
	pending := r.add(xaA, "F-PENDING", "ITEM-NEVER-SAVED", 30*sweepDay, domain.StoredFilePending)
	cover := r.add(xaA, "F-COVER", "ITEM-NEVER-SAVED", 30*sweepDay, domain.StoredFileReady)
	cover.purpose = string(coverPurpose)
	r.repo.articles[xaA] = map[string]bool{"ITEM-LIVE": true, "ITEM-SOFT-DELETED": true}
	// Commune B holds an abandoned image too, but is MERGED (inactive): its data stays exactly as it was.
	otherInactive := r.add(xaB, "F-B-OLD", "ITEM-B", 8*sweepDay, domain.StoredFileReady)
	r.s.registry = sweepRegistry{inactive: map[tenant.ID]bool{xaB: true}}

	r.s.Tick(context.Background())

	if !old.deleted || old.deletedBy != audit.SystemActor || old.why != bodyImageAbandonedReason {
		t.Errorf("8-day-old abandoned image: deleted=%v by=%q why=%q", old.deleted, old.deletedBy, old.why)
	}
	for name, f := range map[string]*sweepFile{"6 days old": young, "of a live article": saved,
		"of a soft-deleted article": ofDeleted, "pending": pending, "a cover": cover,
		"inactive commune": otherInactive} {
		if f.deleted {
			t.Errorf("%s: retired, must be kept", name)
		}
	}

	au := r.sql.audits()
	if len(au) != 1 {
		t.Fatalf("audit entries = %d, want 1 (one per commune run)", len(au))
	}
	a := au[0]
	if a.tenant != string(xaA) || a.actor != audit.SystemActor || a.kind != "system" ||
		a.action != ActionBodyImagesAbandonedRetired || !strings.HasPrefix(a.subject, "noi-dung-mini-app/anh-than-bai/") {
		t.Errorf("audit = %+v", a)
	}
	var delta struct {
		Files []string `json:"tep_ids"`
		N     int      `json:"so_tep"`
	}
	if err := json.Unmarshal([]byte(a.delta), &delta); err != nil || delta.N != 1 || len(delta.Files) != 1 ||
		delta.Files[0] != "F-OLD" {
		t.Errorf("delta = %s (err %v)", a.delta, err)
	}
	if r.sql.begun != 1 || r.sql.commits != 1 {
		t.Errorf("transactions begun=%d committed=%d, want one", r.sql.begun, r.sql.commits)
	}
}

// Two active communes: each swept in ITS OWN context and transaction, each with its own entry — and an
// image of commune B is never retired by commune A's run, even under the same subject id.
func TestBodyImageSweepIsPerCommune(t *testing.T) {
	r := newSweepRig(t, sweepRegistry{})
	a := r.add(xaA, "F-A", "ITEM-SHARED-ID", 8*sweepDay, domain.StoredFileReady)
	b := r.add(xaB, "F-B", "ITEM-SHARED-ID", 8*sweepDay, domain.StoredFileReady)
	bYoung := r.add(xaB, "F-B-YOUNG", "ITEM-B2", 2*sweepDay, domain.StoredFileReady)
	// The id is an article in commune B only: B's file stays, A's goes.
	r.repo.articles[xaB] = map[string]bool{"ITEM-SHARED-ID": true}

	r.s.Tick(context.Background())

	if !a.deleted || b.deleted || bYoung.deleted {
		t.Fatalf("A retired=%v, B (saved in B) retired=%v, B young retired=%v", a.deleted, b.deleted, bYoung.deleted)
	}
	au := r.sql.audits()
	if len(au) != 1 || au[0].tenant != string(xaA) {
		t.Fatalf("audits = %+v, want one for commune A only", au)
	}
}

// Nothing to do writes nothing: no transaction commits an empty entry.
func TestBodyImageSweepWithNothingAbandonedWritesNoEntry(t *testing.T) {
	r := newSweepRig(t, sweepRegistry{})
	r.add(xaA, "F-YOUNG", "ITEM", 6*sweepDay, domain.StoredFileReady)
	r.s.Tick(context.Background())
	if len(r.sql.audits()) != 0 || r.sql.begun != 0 {
		t.Fatalf("audits=%d begun=%d, want none", len(r.sql.audits()), r.sql.begun)
	}
}

// Another replica holds the lock: this one lists nothing and writes nothing.
func TestBodyImageSweepSkipsWhenAnotherReplicaSweeps(t *testing.T) {
	r := newSweepRig(t, sweepRegistry{})
	f := r.add(xaA, "F-OLD", "ITEM", 8*sweepDay, domain.StoredFileReady)
	r.locks.busy = true
	r.s.Tick(context.Background())
	if f.deleted || r.locks.asked != 0 {
		t.Fatalf("swept while another replica held the lock (deleted=%v asked=%d)", f.deleted, r.locks.asked)
	}
}

// A store failure rolls the commune back — no entry without its soft delete.
func TestBodyImageSweepFailureRollsBack(t *testing.T) {
	r := newSweepRig(t, sweepRegistry{})
	r.add(xaA, "F-A", "ITEM-A", 8*sweepDay, domain.StoredFileReady)
	r.repo.lockErr = errors.New("connection reset")
	r.s.Tick(context.Background())
	if len(r.sql.audits()) != 0 || r.sql.rollback != 1 || r.sql.commits != 0 {
		t.Fatalf("audits=%d rollback=%d commits=%d", len(r.sql.audits()), r.sql.rollback, r.sql.commits)
	}
}

// The threshold is the owner's figure, pinned.
func TestBodyImageAbandonedAfterIsSevenDays(t *testing.T) {
	if bodyImageAbandonedAfter != 7*24*time.Hour {
		t.Fatalf("K11 decided 7 days, got %v", bodyImageAbandonedAfter)
	}
	if _, err := NewBodyImageSweeper(BodyImageSweeperDeps{}); err == nil {
		t.Fatal("a sweep without its dependencies was built")
	}
}
