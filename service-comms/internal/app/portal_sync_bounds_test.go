package app

// The 02/10/2026 review fixes and owner decisions on the portal sync run: ceilings (D1), memory and time
// bounds (R1), the security event of a refused destination (R2), the api_url-bound key (R6) and the
// orphaned derivative (R7).

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	"github.com/vihat/vigov/service-comms/internal/portal"
)

func classesOf(run domain.PortalSyncRun) map[string]bool {
	out := map[string]bool{}
	for _, e := range run.Errors {
		out[e.Error] = true
	}
	return out
}

func logInto(r *portalRig) *bytes.Buffer {
	var buf bytes.Buffer
	l := slog.New(slog.NewJSONHandler(&buf, nil))
	r.runner.log, r.admin.log = l, l
	return &buf
}

// --- D1: ceilings -------------------------------------------------------------------------------------

// A row saved above today's ceilings (0013 still admits it) reads and runs AT the ceilings; it is not
// rewritten.
func TestPortalSettingsAboveTheCeilingAreClampedNotRewritten(t *testing.T) {
	r := newPortalRig(t)
	r.repo.settings.WindowDays, r.repo.settings.MaxItemsPerRun = 365, 1000

	v, err := r.admin.Settings(r.ctx)
	if err != nil || v.Settings.WindowDays != 90 || v.Settings.MaxItemsPerRun != 100 {
		t.Fatalf("view = %+v, %v — want the ceilings 90 / 100", v.Settings, err)
	}
	r.runner.Tick(context.Background())
	if len(r.client.since) == 0 || !r.client.since[0].Equal(portalNow.Add(-90*24*time.Hour)) {
		t.Fatalf("since = %v, want 90 days back", r.client.since)
	}
	for _, k := range r.client.keeps {
		if k != 100 {
			t.Fatalf("keep = %d, want the 100 ceiling", k)
		}
	}
	if len(r.repo.upserts) != 0 {
		t.Fatal("reading or running rewrote the settings row")
	}
}

// A save that omits the numbers on a row above the ceiling saves the ceilings — and the trail says so.
func TestPortalSaveOverAnOldRowWritesTheCeilingAndAuditsIt(t *testing.T) {
	r := newPortalRig(t)
	r.repo.settings.WindowDays = 365
	in := domain.PortalSyncSettingsInput{APIURL: r.repo.settings.APIURL}
	if _, err := r.admin.SaveSettings(r.ctx, SavePortalSyncSettingsRequest{Input: in}, audit.Actor{ID: "CB-00123", Kind: "staff"}); err != nil {
		t.Fatal(err)
	}
	if len(r.repo.upserts) != 1 || r.repo.upserts[0].st.WindowDays != 90 {
		t.Fatalf("upserts = %+v", r.repo.upserts)
	}
	au := r.sql.auditsOf(ActionSavePortalSyncSettings)
	if len(au) != 1 || !strings.Contains(au[0].delta, `"window_days":365`) || !strings.Contains(au[0].delta, `"window_days":90`) {
		t.Fatalf("audit = %+v", au)
	}
}

func TestPortalSaveRefusesValuesAboveTheCeiling(t *testing.T) {
	r := newPortalRig(t)
	staff := audit.Actor{ID: "CB-00123", Kind: "staff"}
	for _, in := range []domain.PortalSyncSettingsInput{
		{APIURL: r.repo.settings.APIURL, WindowDays: intp(91)},
		{APIURL: r.repo.settings.APIURL, MaxItemsPerRun: intp(101)},
	} {
		_, err := r.admin.SaveSettings(r.ctx, SavePortalSyncSettingsRequest{Input: in}, staff)
		if !errors.Is(err, domain.ErrPortalWindow) && !errors.Is(err, domain.ErrPortalMaxItems) {
			t.Errorf("%+v: err = %v", in, err)
		}
	}
}

// 30 selected at most — counted over the stored rows AFTER the save, not only the request.
func TestPortalSaveCategoriesRefusesMoreThanThirtySelected(t *testing.T) {
	staff := audit.Actor{ID: "CB-00123", Kind: "staff"}
	r := newPortalRig(t) // two stored rows already selected (120, 144)
	sel := make([]domain.PortalCategorySelection, 0, 29)
	for i := 0; i < 29; i++ {
		sel = append(sel, domain.PortalCategorySelection{ExternalID: fmt.Sprintf("9%02d", i), Name: "C", TargetKind: "tin-tuc", IsSelected: true})
	}
	if _, err := r.admin.SaveCategories(r.ctx, sel, staff); !errors.Is(err, domain.ErrPortalTooManySelected) {
		t.Fatalf("29 new + 2 stored = 31: err = %v", err)
	}
	if len(r.sql.audits()) != 0 || r.sql.commits != 0 {
		t.Fatal("a refused selection committed")
	}
	if _, err := r.admin.SaveCategories(r.ctx, sel[:28], staff); err != nil {
		t.Fatalf("28 + 2 = 30 is allowed: %v", err)
	}
}

// Rows selected before the ceiling: the run reads the first 30 and says so.
func TestPortalRunReadsAtMostThirtyCategories(t *testing.T) {
	r := newPortalRig(t)
	for i := 0; i < 40; i++ {
		r.repo.cats = append(r.repo.cats, domain.PortalCategory{ID: fmt.Sprintf("CAT-%02d", i), ExternalID: fmt.Sprintf("5%02d", i),
			Name: "C", TargetKind: "tin-tuc", IsSelected: true})
	}
	r.runner.Tick(context.Background())
	if n := len(r.client.keeps); n != domain.PortalSelectedCategoriesMax {
		t.Fatalf("read %d categories, want %d", n, domain.PortalSelectedCategoriesMax)
	}
	if !classesOf(r.onlyFinished(t))[domain.PortalRunErrorCategoriesOverCeiling] {
		t.Fatalf("summary = %+v", r.onlyFinished(t).Errors)
	}
}

// --- R1: bounds -----------------------------------------------------------------------------------------

func TestPortalRunAsksEachCategoryForAtMostTheCeiling(t *testing.T) {
	r := newPortalRig(t)
	r.repo.settings.MaxItemsPerRun = 7
	r.runner.Tick(context.Background())
	if len(r.client.keeps) != 2 || r.client.keeps[0] != 7 || r.client.keeps[1] != 7 {
		t.Fatalf("keeps = %v, want 7 per category", r.client.keeps)
	}
}

// Both slots held: a manual start does not queue — ErrPortalRunnerBusy, nothing written, lock released.
func TestPortalManualRunRefusedWhenEverySlotIsTaken(t *testing.T) {
	r := newPortalRig(t)
	startedRunner(r)
	for i := 0; i < portalMaxConcurrentRuns; i++ {
		r.runner.slots <- struct{}{}
	}
	if _, err := r.admin.StartRun(r.ctx, audit.Actor{ID: "CB-00123", Kind: "staff"}); !errors.Is(err, ErrPortalRunnerBusy) {
		t.Fatalf("err = %v, want busy", err)
	}
	if len(r.repo.runRows) != 0 || r.locks.released != 1 {
		t.Fatalf("run rows %d, released %d — want none written and the commune lock given back", len(r.repo.runRows), r.locks.released)
	}
	<-r.runner.slots
	if _, err := r.admin.StartRun(r.ctx, audit.Actor{ID: "CB-00123", Kind: "staff"}); err != nil {
		t.Fatalf("a freed slot still refused: %v", err)
	}
	if !r.runner.Wait(5 * time.Second) {
		t.Fatal("manual run did not finish")
	}
	if len(r.runner.slots) != portalMaxConcurrentRuns-1 {
		t.Fatalf("the manual run did not give its slot back (%d held)", len(r.runner.slots))
	}
}

// A run that outlives its budget stops and says `time-budget` — not `interrupted`, which is a shutdown.
func TestPortalRunCutByItsBudget(t *testing.T) {
	r := newPortalRig(t)
	r.runner.runBudget = 20 * time.Millisecond
	r.client.onArticles = func() { time.Sleep(60 * time.Millisecond) }
	r.runner.Tick(context.Background())
	got := r.onlyFinished(t)
	c := classesOf(got)
	if !c[domain.PortalRunErrorTimeBudget] || c[domain.PortalRunErrorInterrupted] || got.Outcome == domain.PortalRunSucceeded {
		t.Fatalf("finished = %+v", got)
	}
}

// The tick's budget spent: the remaining due communes are not started this tick.
func TestPortalTickBudgetLeavesTheRestForNextTick(t *testing.T) {
	r := newPortalRig(t)
	r.runner.tickBudget = 20 * time.Millisecond
	r.locks.due = []tenant.ID{xaA, xaB}
	r.client.onArticles = func() { time.Sleep(60 * time.Millisecond) }
	r.runner.Tick(context.Background())
	if len(r.repo.runRows) != 1 {
		t.Fatalf("started %d runs, want only the first commune's", len(r.repo.runRows))
	}
	if !classesOf(r.onlyFinished(t))[domain.PortalRunErrorTimeBudget] {
		t.Fatalf("summary = %+v", r.onlyFinished(t).Errors)
	}
}

// --- R2: the security event ----------------------------------------------------------------------------

func TestPortalRefusedDestinationsAreSecurityEvents(t *testing.T) {
	r := newPortalRig(t)
	buf := logInto(r)
	r.client.errs["144"] = portal.ErrRedirectRefused
	r.client.articles["120"] = []portal.Article{withImage(portalArticle("f", portalNow.Add(-time.Hour)), "https://cdn.example.com/x.jpg")}
	r.runner.Tick(context.Background())

	lines := strings.Count(buf.String(), `"event":"outbound_url_refused"`)
	if lines != 2 {
		t.Fatalf("events = %d, want 2 (the redirect, the foreign image): %s", lines, buf.String())
	}
	run := r.onlyFinished(t)
	for _, want := range []string{`"class":"redirect-refused"`, `"class":"image-foreign-host"`, `"actor":"system"`,
		`"run_id":"` + run.ID + `"`, `"xa":"` + string(xaA) + `"`, `"outcome":"refused"`} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("event lacks %s: %s", want, buf.String())
		}
	}
	for _, leak := range []string{"cdn.example.com", portalTestKey, "secret_code", "portal.example.gov.vn"} {
		if strings.Contains(buf.String(), leak) {
			t.Fatalf("the security log carries %q: %s", leak, buf.String())
		}
	}
}

func TestPortalCategoryTreeRefusalNamesTheStaffActor(t *testing.T) {
	r := newPortalRig(t)
	buf := logInto(r)
	r.client.catErr = portal.ErrAddressRefused
	_, _ = r.admin.CategoryTree(r.ctx, audit.Actor{ID: "CB-00123", Kind: "staff"})
	if !strings.Contains(buf.String(), `"event":"outbound_url_refused"`) || !strings.Contains(buf.String(), `"actor":"CB-00123"`) ||
		!strings.Contains(buf.String(), `"class":"address-refused"`) {
		t.Fatalf("event = %s", buf.String())
	}
}

// --- R6: the api_url-bound key ---------------------------------------------------------------------------

// A row sealed under 0013's tenant-only binding opens ONCE, is re-sealed under the api_url binding in a
// transaction with a system entry, and opens the new way from then on.
func TestPortalLegacySealedKeyIsResealedOnFirstOpen(t *testing.T) {
	r := newPortalRig(t)
	legacy, err := r.env.Seal(r.ctx, secret.Secret(portalTestKey), legacyPortalKeyAAD(r.ctx))
	if err != nil {
		t.Fatal(err)
	}
	r.repo.sealed = legacy
	r.runner.Tick(context.Background())

	if got := r.onlyFinished(t); got.Outcome != domain.PortalRunSucceeded {
		t.Fatalf("a legacy row did not run: %+v", got)
	}
	if len(r.repo.reseals) != 1 || !bytes.Equal(r.repo.reseals[0].old, legacy) || r.repo.reseals[0].apiURL != r.repo.settings.APIURL {
		t.Fatalf("reseals = %+v", r.repo.reseals)
	}
	opened, err := r.env.Open(r.ctx, r.repo.sealed, portalKeyAAD(r.ctx, r.repo.settings.APIURL))
	if err != nil || string(opened) != portalTestKey {
		t.Fatalf("the re-sealed key does not open under the new binding: %v", err)
	}
	au := r.sql.auditsOf(ActionResealPortalKey)
	if len(au) != 1 || au[0].actor != audit.SystemActor || au[0].tenant != string(xaA) {
		t.Fatalf("reseal audit = %+v", au)
	}
	r.assertNoPortalSecret(t)
}

// api_url changed BEHIND the screen (a direct write, the key not retyped): the key does not open for the
// new host, and the portal is never called.
func TestPortalKeyDoesNotFollowAnAPIURLChangedInTheDatabase(t *testing.T) {
	r := newPortalRig(t)
	r.repo.settings.APIURL = "https://other.example.gov.vn/api"
	r.runner.Tick(context.Background())
	got := r.onlyFinished(t)
	if got.Outcome != domain.PortalRunFailed || !classesOf(got)["credential-unavailable"] {
		t.Fatalf("finished = %+v", got)
	}
	if len(r.client.keys) != 0 || len(r.repo.reseals) != 0 {
		t.Fatal("the key reached the new host, or was re-sealed for it")
	}
}

// --- R7: the orphaned derivative ----------------------------------------------------------------------

func TestPortalFailedImportDiscardsItsDerivative(t *testing.T) {
	r := newPortalRig(t)
	r.client.articles["120"] = []portal.Article{withImage(portalArticle("a", portalNow.Add(-time.Hour)), "/a.jpg")}
	r.client.articles["144"] = []portal.Article{withImage(portalArticle("b", portalNow.Add(-2*time.Hour)), "/b.jpg")}
	r.client.images["https://portal.example.gov.vn/a.jpg"] = []byte("jpeg")
	r.client.images["https://portal.example.gov.vn/b.jpg"] = []byte("jpeg")
	r.repo.insertErr = map[string]error{"cttdt-danang:b": errors.New("insert broke")}
	r.runner.Tick(context.Background())

	got := r.onlyFinished(t)
	if got.Counts.Imported != 1 || got.Counts.Failed != 1 {
		t.Fatalf("counts = %+v", got.Counts)
	}
	// Only b's: a's import committed, so its row owns the object.
	if len(r.covers.discarded) != 1 || len(r.repo.inserted) != 1 {
		t.Fatalf("discarded = %v, inserted %d", r.covers.discarded, len(r.repo.inserted))
	}
	if r.covers.discarded[0] == r.repo.inserted[0].item.CoverImageFileID {
		t.Fatal("the committed article's cover was discarded")
	}
}

// The real pipeline: the object is purged only when no row claims it.
func TestDiscardPortalCoverKeepsAClaimedObject(t *testing.T) {
	rig := newCoverRig(t, nil)
	pc, err := rig.uc.preparePortalCover(rig.ctx, coverItemID, testJPEG(t, 400, 300, 1), coverClock)
	if err != nil {
		t.Fatal(err)
	}
	// Unclaimed (the import rolled back): purged from the PRIVATE bucket.
	deleted, err := rig.uc.discardPortalCover(rig.ctx, pc.file)
	if err != nil || !deleted || len(rig.objects.purged) != 1 || rig.objects.purged[0] != pc.file.ObjectKey {
		t.Fatalf("deleted=%v err=%v purged=%v", deleted, err, rig.objects.purged)
	}

	// Claimed (an ambiguous commit that landed): kept.
	pc2, err := rig.uc.preparePortalCover(rig.ctx, coverItemID, testJPEG(t, 400, 300, 1), coverClock)
	if err != nil {
		t.Fatal(err)
	}
	f := pc2.file
	rig.files.rows[f.ID] = &f
	deleted, err = rig.uc.discardPortalCover(rig.ctx, pc2.file)
	if err != nil || deleted || len(rig.objects.purged) != 1 {
		t.Fatalf("a claimed object: deleted=%v err=%v purged=%v", deleted, err, rig.objects.purged)
	}
}

// Follow-up 02/10/2026 — THE BACKLOG DRAINS: a category with more new articles than one run's ceiling
// imports the newest N now and the next N next run; imported (or soft-deleted) articles never take a slot.
func TestPortalBacklogDrainsOverRuns(t *testing.T) {
	r := newPortalRig(t)
	r.repo.settings.MaxItemsPerRun = 2
	r.repo.settings.IntervalHours = 0 // runs driven by hand below, not by the due check
	var arts []portal.Article
	for i := 1; i <= 5; i++ {
		arts = append(arts, portalArticle(fmt.Sprintf("b%d", i), portalNow.Add(-time.Duration(10-i)*time.Hour)))
	}
	r.client.articles["120"] = arts // b5 newest … b1 oldest
	r.client.articles["144"] = nil
	r.repo.existing["cttdt-danang:b5"] = true // soft-deleted: never re-imported, never a slot

	run := func() []string {
		before := len(r.repo.inserted)
		runID, err := r.runner.begin(r.ctx, domain.PortalRunScheduled, systemPrincipal)
		if err != nil {
			t.Fatal(err)
		}
		st, sealed, _ := r.repo.SettingsWithKey(r.ctx)
		r.runner.execute(r.ctx, runID, st, sealed)
		var got []string
		for _, in := range r.repo.inserted[before:] {
			got = append(got, in.item.NguonIDNgoai)
			r.repo.existing[in.item.NguonIDNgoai] = false // imported: held from now on
		}
		return got
	}
	if got := strings.Join(run(), ","); got != "cttdt-danang:b4,cttdt-danang:b3" {
		t.Fatalf("run 1 imported %s, want b4,b3", got)
	}
	if got := strings.Join(run(), ","); got != "cttdt-danang:b2,cttdt-danang:b1" {
		t.Fatalf("run 2 imported %s, want b2,b1", got)
	}
	if got := run(); len(got) != 0 {
		t.Fatalf("run 3 imported %v, want nothing left", got)
	}
	last := r.repo.finished[len(r.repo.finished)-1]
	if last.Counts.SkippedDeleted != 1 || last.Counts.SkippedExisting != 4 {
		t.Fatalf("run 3 counts = %+v, want 1 deleted + 4 existing skipped", last.Counts)
	}
}

// An article filed under two categories and already held is ONE skip, not two.
func TestPortalHeldSkipCountedOnceAcrossCategories(t *testing.T) {
	r := newPortalRig(t)
	same := portalArticle("x", portalNow.Add(-time.Hour))
	r.client.articles["120"] = []portal.Article{same}
	r.client.articles["144"] = []portal.Article{same}
	r.repo.existing["cttdt-danang:x"] = false
	r.runner.Tick(context.Background())
	if got := r.onlyFinished(t).Counts; got.SkippedExisting != 1 || got.Imported != 0 {
		t.Fatalf("counts = %+v", got)
	}
}
