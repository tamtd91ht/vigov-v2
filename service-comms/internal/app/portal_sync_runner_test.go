package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	"github.com/vihat/vigov/service-comms/internal/portal"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// --- the scheduled run ------------------------------------------------------------------------------

func TestPortalScheduledRunUsesSystemActorAndTheCommune(t *testing.T) {
	r := newPortalRig(t)
	r.runner.Tick(context.Background())

	if len(r.repo.runRows) != 1 || r.repo.runRows[0].TriggerKind != domain.PortalRunScheduled ||
		r.repo.runRows[0].Actor != audit.SystemActor {
		t.Fatalf("run rows = %+v", r.repo.runRows)
	}
	if len(r.repo.markedStart) != 1 || !r.repo.markedStart[0].Equal(portalNow) {
		t.Errorf("last_run_at = %v", r.repo.markedStart)
	}
	if len(r.repo.inserted) != 2 {
		t.Fatalf("inserted %d, want 2", len(r.repo.inserted))
	}
	for _, in := range r.repo.inserted {
		if in.commune != xaA || in.item.NguoiTaoMa != audit.SystemActor || in.item.Nguon != domain.NguonDongBoCong ||
			in.item.DanhMucID != "" || in.catID == "" {
			t.Errorf("insert = %+v in %s under %s", in.item, in.commune, in.catID)
		}
	}
	for _, a := range r.sql.audits() {
		if a.tenant != string(xaA) || a.actor != audit.SystemActor || a.kind != "system" {
			t.Errorf("audit entry %s by %s/%s in %s, want the system principal in %s", a.action, a.actor, a.kind, a.tenant, xaA)
		}
	}
	if len(r.sql.auditsOf(ActionImportPortalItem)) != 2 || len(r.sql.auditsOf(ActionStartPortalSync)) != 1 ||
		len(r.sql.auditsOf(ActionFinishPortalSync)) != 1 {
		t.Errorf("audits = %+v", r.sql.audits())
	}
	// Each import commits on its own with its entry: begin + 2 imports + finish.
	if r.sql.commits != 4 || r.sql.rollback != 0 {
		t.Errorf("commits %d, rollbacks %d", r.sql.commits, r.sql.rollback)
	}
	// The key opened from the sealed bytes is what reached the portal; nothing wrote it anywhere.
	for _, k := range r.client.keys {
		if k != portalTestKey {
			t.Fatalf("portal got key %q", k)
		}
	}
	r.assertNoPortalSecret(t)
	if got := r.onlyFinished(t); got.Outcome != domain.PortalRunSucceeded || got.Counts.Imported != 2 {
		t.Errorf("finished = %+v", got)
	}
	if r.locks.released != 2 { // the scheduler's and the commune's
		t.Errorf("released %d locks, want 2", r.locks.released)
	}
}

func TestPortalRunReadsOnlySelectedCategories(t *testing.T) {
	r := newPortalRig(t)
	r.client.articles["200"] = []portal.Article{portalArticle("3", portalNow)}
	r.runner.Tick(context.Background())
	for _, in := range r.repo.inserted {
		if in.catID == "CAT-OFF" {
			t.Fatal("an unselected category was imported")
		}
	}
	if len(r.client.since) != 2 {
		t.Errorf("asked %d categories, want the 2 selected", len(r.client.since))
	}
}

func TestPortalRunSkipsExistingAndSoftDeletedAndNeverUpdates(t *testing.T) {
	r := newPortalRig(t)
	r.client.articles["120"] = []portal.Article{
		portalArticle("live", portalNow.Add(-3*time.Hour)),
		portalArticle("gone", portalNow.Add(-2*time.Hour)),
		portalArticle("new", portalNow.Add(-1*time.Hour)),
	}
	r.client.articles["144"] = nil
	r.repo.existing[domain.PortalExternalItemID(domain.PortalProviderCityShared, "live")] = false
	r.repo.existing[domain.PortalExternalItemID(domain.PortalProviderCityShared, "gone")] = true

	r.runner.Tick(context.Background())

	if len(r.repo.inserted) != 1 || r.repo.inserted[0].item.NguonIDNgoai != "cttdt-danang:new" {
		t.Fatalf("inserted %+v, want only the new article", r.repo.inserted)
	}
	got := r.onlyFinished(t)
	if got.Counts.SkippedExisting != 1 || got.Counts.SkippedDeleted != 1 || got.Counts.Imported != 1 {
		t.Errorf("counts = %+v", got.Counts)
	}
	// NEVER AN UPDATE: the run's only write to the register is the repo's INSERT, and no statement it
	// sent itself touches noi_dung_mini_app.
	for _, s := range r.sql.stmts {
		if strings.Contains(s.sql, "noi_dung_mini_app") {
			t.Errorf("the run sent %q", s.sql)
		}
	}
}

func TestPortalRunPublishModes(t *testing.T) {
	for _, tc := range []struct {
		mode      string
		state     domain.TrangThaiNoiDung
		published bool
	}{
		{domain.PortalPublishReview, domain.TrangThaiChoDuyet, false},
		{domain.PortalPublishDirect, domain.TrangThaiDangHien, true},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			r := newPortalRig(t)
			r.repo.settings.PublishMode = tc.mode
			r.client.articles["120"][0].ImageRef = "/a.jpg"
			r.client.images["https://portal.example.gov.vn/a.jpg"] = []byte("jpeg")
			r.runner.Tick(context.Background())
			if len(r.repo.inserted) != 2 {
				t.Fatalf("inserted %d", len(r.repo.inserted))
			}
			for _, in := range r.repo.inserted {
				if in.item.TrangThai != tc.state || in.item.PublishedAt.IsZero() == tc.published {
					t.Errorf("item %s: state %s published_at %v", in.item.ID, in.item.TrangThai, in.item.PublishedAt)
				}
			}
			if tc.published != (len(r.covers.published) == 1) {
				t.Errorf("published covers = %v", r.covers.published)
			}
		})
	}
}

func TestPortalRunWindowAndCeiling(t *testing.T) {
	r := newPortalRig(t)
	r.repo.settings.WindowDays = 30
	r.repo.settings.MaxItemsPerRun = 2
	r.client.articles["120"] = []portal.Article{
		portalArticle("a", portalNow.Add(-4*time.Hour)),
		portalArticle("b", portalNow.Add(-1*time.Hour)), // newest
	}
	r.client.articles["144"] = []portal.Article{
		portalArticle("c", portalNow.Add(-2*time.Hour)),
		portalArticle("d", portalNow.Add(-3*time.Hour)),
	}
	r.runner.Tick(context.Background())

	for _, s := range r.client.since {
		if want := portalNow.Add(-30 * 24 * time.Hour); !s.Equal(want) {
			t.Errorf("since = %v, want %v", s, want)
		}
	}
	if len(r.repo.inserted) != 2 || r.repo.inserted[0].item.NguonIDNgoai != "cttdt-danang:b" ||
		r.repo.inserted[1].item.NguonIDNgoai != "cttdt-danang:c" {
		t.Fatalf("inserted %+v, want the two newest across categories", r.repo.inserted)
	}
	// The category's mapping decides the type.
	if r.repo.inserted[1].item.Loai != domain.LoaiSuKien || r.repo.inserted[1].catID != "CAT-EVENT" {
		t.Errorf("second item = %s / %s", r.repo.inserted[1].item.Loai, r.repo.inserted[1].catID)
	}
	if got := r.onlyFinished(t); got.Counts.Fetched != 6 { // the fake reports Read = len + 1 per category
		t.Errorf("fetched = %d", got.Counts.Fetched)
	}
}

func TestPortalRunSameArticleInTwoCategoriesIsImportedOnce(t *testing.T) {
	r := newPortalRig(t)
	r.client.articles["144"] = []portal.Article{portalArticle("1", portalNow.Add(-2*time.Hour))}
	r.runner.Tick(context.Background())
	if len(r.repo.inserted) != 1 || r.repo.inserted[0].catID != "CAT-NEWS" {
		t.Fatalf("inserted %+v, want one, under the first selected category", r.repo.inserted)
	}
}

func TestPortalRunOneCategoryFailingIsPartial(t *testing.T) {
	r := newPortalRig(t)
	r.client.errs["144"] = portal.ErrTimeout
	r.runner.Tick(context.Background())
	got := r.onlyFinished(t)
	if got.Outcome != domain.PortalRunPartial || got.Counts.Imported != 1 {
		t.Fatalf("finished = %+v", got)
	}
	if len(got.Errors) != 1 || got.Errors[0].CategoryExternalID != "144" || got.Errors[0].Error != "timeout" ||
		got.Errors[0].CategoryName != "Sự kiện" || got.Errors[0].Count != 1 {
		t.Errorf("summary = %+v", got.Errors)
	}
}

func TestPortalRunEveryCategoryFailingIsFailure(t *testing.T) {
	r := newPortalRig(t)
	r.client.errs["120"] = portal.ErrConnect
	r.client.errs["144"] = portal.ErrDNS
	r.runner.Tick(context.Background())
	if got := r.onlyFinished(t); got.Outcome != domain.PortalRunFailed || len(r.repo.inserted) != 0 {
		t.Fatalf("finished = %+v", got)
	}
}

func TestPortalRunNoCategorySelected(t *testing.T) {
	r := newPortalRig(t)
	for i := range r.repo.cats {
		r.repo.cats[i].IsSelected = false
	}
	r.runner.Tick(context.Background())
	got := r.onlyFinished(t)
	if got.Outcome != domain.PortalRunFailed || len(got.Errors) != 1 || got.Errors[0].Error != domain.PortalRunErrorNoCategory {
		t.Fatalf("finished = %+v", got)
	}
}

func TestPortalRunReapsStuckRunsBeforeStarting(t *testing.T) {
	r := newPortalRig(t)
	r.repo.reapIDs = []string{"01JOLDCRASHEDRUN"}
	r.runner.Tick(context.Background())
	if len(r.repo.reapBefore) != 1 || !r.repo.reapBefore[0].Equal(portalNow.Add(-time.Hour)) {
		t.Fatalf("reap before = %v, want now − 1 h", r.repo.reapBefore)
	}
	found := false
	for _, a := range r.sql.auditsOf(ActionFinishPortalSync) {
		if strings.Contains(a.delta, "01JOLDCRASHEDRUN") && strings.Contains(a.delta, domain.PortalRunFailed) {
			found = true
		}
	}
	if !found {
		t.Error("the reaped run's finish is not in the trail")
	}
}

func TestPortalSchedulerLockContention(t *testing.T) {
	t.Run("another replica ticks", func(t *testing.T) {
		r := newPortalRig(t)
		r.locks.schedulerBusy = true
		r.runner.Tick(context.Background())
		if r.locks.dueAsked != 0 || len(r.repo.runRows) != 0 {
			t.Fatal("a tick ran without the scheduler lock")
		}
	})
	t.Run("the commune is running elsewhere", func(t *testing.T) {
		r := newPortalRig(t)
		r.locks.communeBusy = true
		r.runner.Tick(context.Background())
		if len(r.repo.runRows) != 0 {
			t.Fatal("a run started without the commune lock")
		}
	})
	t.Run("no longer due under the lock", func(t *testing.T) {
		r := newPortalRig(t)
		r.repo.settings.LastRunAt = portalNow.Add(-time.Hour) // interval 6 h
		r.runner.Tick(context.Background())
		if len(r.repo.runRows) != 0 {
			t.Fatal("a run started before it was due")
		}
	})
}

func TestPortalRunSkipsAnInactiveCommune(t *testing.T) {
	r := newPortalRig(t)
	r.runner.registry = portalRegistry{active: false}
	r.runner.Tick(context.Background())
	if len(r.repo.runRows) != 0 || len(r.locks.lockedCommunes) != 0 {
		t.Fatal("a merged (inactive) commune was synced")
	}
}

// --- manual runs -----------------------------------------------------------------------------------

func TestPortalManualRunIsSignedByStaffAndFinishes(t *testing.T) {
	r := newPortalRig(t)
	startedRunner(r)
	r.repo.settings.IsEnabled = false // the button does not need the schedule switched on
	staff := audit.Actor{ID: "CB-00123", Kind: "staff", IP: "10.0.0.7"}
	run, err := r.admin.StartRun(r.ctx, staff)
	if err != nil {
		t.Fatal(err)
	}
	if !r.runner.Wait(5 * time.Second) {
		t.Fatal("the manual run did not finish")
	}
	if run.TriggerKind != domain.PortalRunManual || run.Actor != "CB-00123" {
		t.Errorf("run = %+v", run)
	}
	start := r.sql.auditsOf(ActionStartPortalSync)
	if len(start) != 1 || start[0].actor != "CB-00123" || start[0].tenant != string(xaA) {
		t.Errorf("start entry = %+v", start)
	}
	// The imports are the run's acts, signed by the system principal.
	for _, a := range r.sql.auditsOf(ActionImportPortalItem) {
		if a.actor != audit.SystemActor {
			t.Errorf("import signed by %s", a.actor)
		}
	}
	for _, in := range r.repo.inserted {
		if in.commune != xaA {
			t.Errorf("imported into %s", in.commune)
		}
	}
	if got := r.onlyFinished(t); got.ID != run.ID || got.Outcome != domain.PortalRunSucceeded {
		t.Errorf("finished = %+v", got)
	}
	if r.locks.released != 1 {
		t.Errorf("commune lock released %d times, want 1", r.locks.released)
	}
}

func TestPortalManualRunRefusals(t *testing.T) {
	staff := audit.Actor{ID: "CB-00123", Kind: "staff"}
	t.Run("in progress", func(t *testing.T) {
		r := newPortalRig(t)
		startedRunner(r)
		r.locks.communeBusy = true
		if _, err := r.admin.StartRun(r.ctx, staff); !errors.Is(err, ErrPortalRunInProgress) {
			t.Fatalf("err = %v, want in progress", err)
		}
		if len(r.repo.runRows) != 0 {
			t.Error("a run row was written")
		}
	})
	t.Run("runner not started", func(t *testing.T) {
		r := newPortalRig(t)
		if _, err := r.admin.StartRun(r.ctx, staff); !errors.Is(err, ErrPortalRunnerStopped) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("not configured", func(t *testing.T) {
		r := newPortalRig(t)
		startedRunner(r)
		r.repo.settings = nil
		if _, err := r.admin.StartRun(r.ctx, staff); !errors.Is(err, commsstore.ErrPortalSyncSettingsNotFound) {
			t.Fatalf("err = %v", err)
		}
		if r.locks.released != 1 {
			t.Error("the lock was not released on refusal")
		}
	})
	t.Run("signed as the system", func(t *testing.T) {
		r := newPortalRig(t)
		startedRunner(r)
		if _, err := r.admin.StartRun(r.ctx, systemPrincipal); !errors.Is(err, ErrPortalMissingActor) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestPortalRunInterruptedStillFinishes(t *testing.T) {
	r := newPortalRig(t)
	ctx, cancel := context.WithCancel(context.Background())
	r.client.onArticles = cancel // the shutdown arrives while categories are read
	r.runner.Tick(ctx)
	got := r.onlyFinished(t)
	found := false
	for _, e := range got.Errors {
		found = found || e.Error == domain.PortalRunErrorInterrupted
	}
	if !found || got.Outcome == domain.PortalRunSucceeded {
		t.Fatalf("finished = %+v, want an interrupted, not successful, run", got)
	}
}

// --- what lands -------------------------------------------------------------------------------------

func TestPortalRunSanitisesTheBodyAndKeepsTheCredit(t *testing.T) {
	r := newPortalRig(t)
	r.client.articles["120"] = []portal.Article{{ExternalID: "x", Title: "T", PublishedAt: portalNow.Add(-time.Hour),
		BodyHTML:    `<p>Một</p><script>alert(1)</script><img src="https://evil.example/x.png"><a href="javascript:x()">l</a>`,
		SourceLabel: "Báo <Đà Nẵng>"}}
	r.client.articles["144"] = nil
	r.runner.Tick(context.Background())
	if len(r.repo.inserted) != 1 {
		t.Fatalf("inserted %d", len(r.repo.inserted))
	}
	body := r.repo.inserted[0].item.NoiDung
	for _, bad := range []string{"<script", "alert", "<img", "javascript:"} {
		if strings.Contains(body, bad) {
			t.Errorf("body keeps %q: %s", bad, body)
		}
	}
	if !strings.Contains(body, "<p><em>Nguồn: Báo &lt;Đà Nẵng&gt;</em></p>") {
		t.Errorf("credit missing: %s", body)
	}
	// The portal's own date, at the portal's midnight, not the import's clock.
	if d := r.repo.inserted[0].item.NgayDang; d.Format("2006-01-02") != "2026-10-01" {
		t.Errorf("ngay_dang = %v", d)
	}
}

func TestPortalRunCreditFallsBackToThePortalHost(t *testing.T) {
	r := newPortalRig(t)
	r.client.articles["144"] = nil
	r.runner.Tick(context.Background())
	if body := r.repo.inserted[0].item.NoiDung; !strings.Contains(body, "Nguồn: portal.example.gov.vn") {
		t.Errorf("body = %s", body)
	}
	r2 := newPortalRig(t)
	r2.repo.settings.KeepSourceCredit = false
	r2.client.articles["144"] = nil
	r2.runner.Tick(context.Background())
	if body := r2.repo.inserted[0].item.NoiDung; strings.Contains(body, "Nguồn:") {
		t.Errorf("credit kept though switched off: %s", body)
	}
}

func TestPortalRunAuditCarriesNoArticleText(t *testing.T) {
	r := newPortalRig(t)
	r.runner.Tick(context.Background())
	for _, a := range r.sql.audits() {
		for _, text := range []string{"Tin 1", "Tóm tắt", "Thân bài"} {
			if strings.Contains(a.delta, text) || strings.Contains(a.subject, text) {
				t.Fatalf("audit %s carries article text: %s", a.action, a.delta)
			}
		}
	}
}

func TestPortalRunTooLongTitleFailsTheArticleNotTheRun(t *testing.T) {
	r := newPortalRig(t)
	r.client.articles["120"][0].Title = strings.Repeat("a", domain.TieuDeNoiDungToiDa+1)
	r.runner.Tick(context.Background())
	got := r.onlyFinished(t)
	if got.Counts.Failed != 1 || got.Counts.Imported != 1 || got.Outcome != domain.PortalRunPartial {
		t.Fatalf("finished = %+v", got)
	}
}

func TestPortalRunInsertRaceCountsAsSkipped(t *testing.T) {
	r := newPortalRig(t)
	r.repo.insertErr = map[string]error{"cttdt-danang:1": commsstore.ErrPortalItemExists}
	r.runner.Tick(context.Background())
	got := r.onlyFinished(t)
	if got.Counts.SkippedExisting != 1 || got.Counts.Imported != 1 || got.Counts.Failed != 0 {
		t.Fatalf("counts = %+v", got.Counts)
	}
}

func TestPortalRunImages(t *testing.T) {
	r := newPortalRig(t)
	r.repo.settings.PublishMode = domain.PortalPublishDirect
	r.client.articles["120"] = []portal.Article{
		withImage(portalArticle("ok", portalNow.Add(-1*time.Hour)), "/ok.jpg"),
		withImage(portalArticle("foreign", portalNow.Add(-2*time.Hour)), "https://cdn.example.com/x.jpg"),
		withImage(portalArticle("broken", portalNow.Add(-3*time.Hour)), "/broken.jpg"),
	}
	r.client.articles["144"] = nil
	r.client.images["https://portal.example.gov.vn/ok.jpg"] = []byte("jpeg")
	r.runner.Tick(context.Background())

	if len(r.repo.inserted) != 3 {
		t.Fatalf("inserted %d, want all three — an image never stops its article", len(r.repo.inserted))
	}
	byExt := map[string]domain.NoiDungMiniApp{}
	for _, in := range r.repo.inserted {
		byExt[in.item.NguonIDNgoai] = in.item
	}
	ok := byExt["cttdt-danang:ok"]
	if ok.CoverImageFileID == "" || len(r.covers.recorded) != 1 || r.covers.recorded[0] != ok.CoverImageFileID {
		t.Errorf("cover not attached: %q / %v", ok.CoverImageFileID, r.covers.recorded)
	}
	if len(r.covers.published) != 1 {
		t.Errorf("published = %v", r.covers.published)
	}
	if byExt["cttdt-danang:foreign"].CoverImageFileID != "" || byExt["cttdt-danang:broken"].CoverImageFileID != "" {
		t.Error("a dropped image is attached")
	}
	for _, u := range r.client.imageCalls {
		if strings.Contains(u, "cdn.example.com") {
			t.Fatal("a foreign-host image was fetched")
		}
	}
	got := r.onlyFinished(t)
	classes := map[string]bool{}
	for _, e := range got.Errors {
		classes[e.Error] = true
	}
	if !classes["image-foreign-host"] || !classes["image-timeout"] {
		t.Errorf("summary = %+v", got.Errors)
	}
	if got.Outcome != domain.PortalRunSucceeded {
		t.Errorf("image problems lowered the outcome to %s", got.Outcome)
	}
}

func TestPortalRunImageThatFailsThePipelineIsDropped(t *testing.T) {
	r := newPortalRig(t)
	r.covers.prepErr = &CoverRejection{Reason: CoverRejectMalware}
	r.client.articles["120"][0].ImageRef = "/a.jpg"
	r.client.articles["144"] = nil
	r.client.images["https://portal.example.gov.vn/a.jpg"] = []byte("x")
	r.runner.Tick(context.Background())
	if len(r.repo.inserted) != 1 || r.repo.inserted[0].item.CoverImageFileID != "" || len(r.covers.recorded) != 0 {
		t.Fatalf("inserted %+v, recorded %v", r.repo.inserted, r.covers.recorded)
	}
	if got := r.onlyFinished(t); len(got.Errors) != 1 || got.Errors[0].Error != "image-"+CoverRejectMalware {
		t.Errorf("summary = %+v", got.Errors)
	}
}

func TestPortalRunWithdrawsACopyWhenTheImportRollsBack(t *testing.T) {
	r := newPortalRig(t)
	r.repo.settings.PublishMode = domain.PortalPublishDirect
	r.client.articles["120"][0].ImageRef = "/a.jpg"
	r.client.articles["144"] = nil
	r.client.images["https://portal.example.gov.vn/a.jpg"] = []byte("jpeg")
	r.sql.failOn = ActionImportPortalItem // the import's audit entry fails, after the copy
	r.runner.Tick(context.Background())
	if len(r.covers.withdrawn) != 1 {
		t.Fatalf("withdrawn = %v, want the copy taken back", r.covers.withdrawn)
	}
	if got := r.onlyFinished(t); got.Counts.Failed != 1 || got.Counts.Imported != 0 {
		t.Errorf("counts = %+v", got.Counts)
	}
	if r.sql.rollback != 1 {
		t.Errorf("rollbacks = %d, want the import's", r.sql.rollback)
	}
}

func TestPortalRunWithoutAKEKFailsByName(t *testing.T) {
	r := newPortalRig(t)
	r.runner.envelope = nil
	r.runner.Tick(context.Background())
	got := r.onlyFinished(t)
	if got.Outcome != domain.PortalRunFailed || got.Errors[0].Error != "encryption-not-configured" {
		t.Fatalf("finished = %+v", got)
	}
}

func TestPortalRunTheKeyOfAnotherCommuneDoesNotOpen(t *testing.T) {
	r := newPortalRig(t)
	r.locks.due = []tenant.ID{xaB}
	r.runner.Tick(context.Background()) // B's context, A's sealed key (the fake returns A's row)
	got := r.onlyFinished(t)
	if got.Outcome != domain.PortalRunFailed || got.Errors[0].Error != "credential-unavailable" {
		t.Fatalf("finished = %+v", got)
	}
	if len(r.client.keys) != 0 {
		t.Fatal("the portal was called with a key opened in another commune")
	}
}
