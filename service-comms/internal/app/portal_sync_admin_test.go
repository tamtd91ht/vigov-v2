package app

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/crypto"
	"github.com/vihat/vigov/core/malwarescan"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/storage"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	"github.com/vihat/vigov/service-comms/internal/portal"
)

// --- settings ---------------------------------------------------------------------------------------

func TestPortalSettingsSave(t *testing.T) {
	staff := audit.Actor{ID: "CB-00123", Kind: "staff", IP: "10.0.0.7"}
	in := domain.PortalSyncSettingsInput{APIURL: "https://portal.example.gov.vn/api", IntervalHours: intp(6),
		IsEnabled: boolp(true)}

	t.Run("first save needs the key", func(t *testing.T) {
		r := newPortalRig(t)
		r.repo.settings, r.repo.sealed = nil, nil
		if _, err := r.admin.SaveSettings(r.ctx, SavePortalSyncSettingsRequest{Input: in}, staff); !errors.Is(err, ErrPortalKeyRequired) {
			t.Fatalf("err = %v", err)
		}
		if len(r.repo.upserts) != 0 || r.sql.commits != 0 {
			t.Error("something was written")
		}
	})
	t.Run("first save seals the key and audits without it", func(t *testing.T) {
		r := newPortalRig(t)
		r.repo.settings, r.repo.sealed = nil, nil
		v, err := r.admin.SaveSettings(r.ctx, SavePortalSyncSettingsRequest{Input: in, APIKey: secret.Secret(portalTestKey)}, staff)
		if err != nil {
			t.Fatal(err)
		}
		if !v.Settings.APIKeySet || !v.Configured || len(r.repo.upserts) != 1 || r.repo.upserts[0].by != "CB-00123" {
			t.Fatalf("view = %+v, upserts = %+v", v, r.repo.upserts)
		}
		sealed := r.repo.upserts[0].sealed
		if bytes.Contains(sealed, []byte(portalTestKey)) {
			t.Fatal("the key is stored in the clear")
		}
		opened, err := r.env.Open(r.ctx, sealed, portalKeyAAD(r.ctx, in.APIURL))
		if err != nil || string(opened) != portalTestKey {
			t.Fatalf("the sealed key does not open back: %v", err)
		}
		// Bound to THIS commune: under another commune's binding it refuses.
		other := tenant.Into(context.Background(), xaB)
		if _, err := r.env.Open(other, sealed, portalKeyAAD(other, in.APIURL)); err == nil {
			t.Fatal("the sealed key opens in another commune")
		}
		// R6: bound to THIS api_url — under another address's binding it refuses too.
		if _, err := r.env.Open(r.ctx, sealed, portalKeyAAD(r.ctx, "https://other.example.gov.vn/api")); err == nil {
			t.Fatal("the sealed key opens for another api_url")
		}
		au := r.sql.auditsOf(ActionSavePortalSyncSettings)
		if len(au) != 1 || !strings.Contains(au[0].delta, `"api_key_changed":true`) || au[0].actor != "CB-00123" ||
			au[0].tenant != string(xaA) {
			t.Fatalf("audit = %+v", au)
		}
		if r.sql.commits != 1 {
			t.Errorf("commits = %d: the upsert and its entry share ONE transaction", r.sql.commits)
		}
		r.repo.sealed = sealed
		r.assertNoPortalSecret(t)
	})
	t.Run("changing the url without the key is refused", func(t *testing.T) {
		r := newPortalRig(t)
		moved := in
		moved.APIURL = "https://other.example.gov.vn/api"
		if _, err := r.admin.SaveSettings(r.ctx, SavePortalSyncSettingsRequest{Input: moved}, staff); !errors.Is(err, ErrPortalKeyRequiredForNewURL) {
			t.Fatalf("err = %v", err)
		}
		if len(r.repo.upserts) != 0 || len(r.sql.audits()) != 0 {
			t.Error("the old key followed the new url")
		}
	})
	t.Run("changing the url with a key", func(t *testing.T) {
		r := newPortalRig(t)
		moved := in
		moved.APIURL = "https://other.example.gov.vn/api"
		if _, err := r.admin.SaveSettings(r.ctx, SavePortalSyncSettingsRequest{Input: moved, APIKey: secret.Secret("new-key-1")}, staff); err != nil {
			t.Fatal(err)
		}
		if len(r.repo.upserts) != 1 || bytes.Equal(r.repo.upserts[0].sealed, r.repo.sealed) {
			t.Error("the new key was not sealed")
		}
		au := r.sql.auditsOf(ActionSavePortalSyncSettings)
		if len(au) != 1 || !strings.Contains(au[0].delta, "other.example.gov.vn") || strings.Contains(au[0].delta, "new-key-1") {
			t.Errorf("audit = %+v", au)
		}
	})
	t.Run("same url, other fields, no key keeps the stored key", func(t *testing.T) {
		r := newPortalRig(t)
		ch := in
		ch.IntervalHours = intp(12)
		if _, err := r.admin.SaveSettings(r.ctx, SavePortalSyncSettingsRequest{Input: ch}, staff); err != nil {
			t.Fatal(err)
		}
		if len(r.repo.upserts) != 1 || !bytes.Equal(r.repo.upserts[0].sealed, r.repo.sealed) {
			t.Fatal("the stored key was not kept")
		}
		au := r.sql.auditsOf(ActionSavePortalSyncSettings)
		if len(au) != 1 || !strings.Contains(au[0].delta, `"api_key_changed":false`) ||
			!strings.Contains(au[0].delta, `"interval_hours":12`) {
			t.Fatalf("audit = %+v", au)
		}
		r.assertNoPortalSecret(t)
	})
	t.Run("nothing moved writes nothing", func(t *testing.T) {
		r := newPortalRig(t)
		if _, err := r.admin.SaveSettings(r.ctx, SavePortalSyncSettingsRequest{Input: in}, staff); err != nil {
			t.Fatal(err)
		}
		if len(r.repo.upserts) != 0 || len(r.sql.audits()) != 0 {
			t.Fatal("an unchanged save wrote")
		}
	})
	t.Run("the outbound guard refuses at the screen", func(t *testing.T) {
		for _, u := range []string{"http://portal.example.gov.vn/api", "https://10.0.0.1/api",
			"https://portal.example.gov.vn:8443/api", "https://example.com/api",
			"https://portal.example.gov.vn/api?secret_code=1", "https://u:p@portal.example.gov.vn/api"} {
			r := newPortalRig(t)
			bad := in
			bad.APIURL = u
			if _, err := r.admin.SaveSettings(r.ctx, SavePortalSyncSettingsRequest{Input: bad, APIKey: secret.Secret("k")}, staff); !errors.Is(err, domain.ErrPortalAPIURLInvalid) {
				t.Errorf("%s: err = %v", u, err)
			}
			if r.sql.begun != 0 {
				t.Errorf("%s: a transaction opened", u)
			}
		}
	})
	t.Run("a key with a space is refused before sealing", func(t *testing.T) {
		r := newPortalRig(t)
		if _, err := r.admin.SaveSettings(r.ctx, SavePortalSyncSettingsRequest{Input: in, APIKey: secret.Secret("a b")}, staff); !errors.Is(err, domain.ErrPortalAPIKeyShape) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("no KEK", func(t *testing.T) {
		r := newPortalRig(t)
		r.admin.envelope = nil
		if _, err := r.admin.SaveSettings(r.ctx, SavePortalSyncSettingsRequest{Input: in, APIKey: secret.Secret("k")}, staff); !errors.Is(err, crypto.ErrNotConfigured) {
			t.Fatalf("err = %v", err)
		}
		if r.sql.begun != 0 {
			t.Error("a transaction opened")
		}
	})
	t.Run("no actor", func(t *testing.T) {
		r := newPortalRig(t)
		if _, err := r.admin.SaveSettings(r.ctx, SavePortalSyncSettingsRequest{Input: in}, audit.Actor{}); !errors.Is(err, ErrPortalMissingActor) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestPortalSettingsReadHasDefaultsAndNoKey(t *testing.T) {
	r := newPortalRig(t)
	r.repo.settings = nil
	v, err := r.admin.Settings(r.ctx)
	if err != nil || v.Configured || v.Settings.PublishMode != domain.PortalPublishReview || v.Settings.IntervalHours != 6 ||
		v.Settings.WindowDays != 90 || v.Settings.MaxItemsPerRun != 100 || !v.Settings.KeepSourceCredit ||
		v.Settings.APIKeySet || v.Settings.IsEnabled {
		t.Fatalf("view = %+v, %v", v, err)
	}
}

// --- categories -------------------------------------------------------------------------------------

func TestPortalSaveCategories(t *testing.T) {
	staff := audit.Actor{ID: "CB-00123", Kind: "staff"}
	r := newPortalRig(t)
	_, err := r.admin.SaveCategories(r.ctx, []domain.PortalCategorySelection{
		{ExternalID: "120", Name: "Tin tức", TargetKind: "tin-tuc", IsSelected: true},     // unchanged
		{ExternalID: "144", Name: "Sự kiện", TargetKind: "thong-bao", IsSelected: true},   // remapped
		{ExternalID: "300", Name: "Mới", TargetKind: "tin-tuc", IsSelected: true},         // new
		{ExternalID: "301", Name: "Không chọn", TargetKind: "tin-tuc", IsSelected: false}, // never selected: no row
	}, staff)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.repo.catInserts) != 1 || r.repo.catInserts[0].ExternalID != "300" {
		t.Errorf("inserts = %+v", r.repo.catInserts)
	}
	if len(r.repo.catUpdates) != 1 || r.repo.catUpdates[0].TargetKind != "thong-bao" || r.repo.catUpdates[0].ID != "CAT-EVENT" {
		t.Errorf("updates = %+v", r.repo.catUpdates)
	}
	if au := r.sql.auditsOf(ActionSavePortalCategories); len(au) != 1 || au[0].actor != "CB-00123" || r.sql.commits != 1 {
		t.Errorf("audit = %+v, commits %d", au, r.sql.commits)
	}

	r2 := newPortalRig(t)
	if _, err := r2.admin.SaveCategories(r2.ctx, []domain.PortalCategorySelection{
		{ExternalID: "120", Name: "Tin tức", TargetKind: "tin-tuc", IsSelected: true}}, staff); err != nil {
		t.Fatal(err)
	}
	if len(r2.sql.audits()) != 0 || len(r2.repo.catUpdates) != 0 {
		t.Error("an unchanged selection wrote")
	}
	// Unticking keeps the row (0013: a flag, not a delete).
	if _, err := r2.admin.SaveCategories(r2.ctx, []domain.PortalCategorySelection{
		{ExternalID: "120", Name: "Tin tức", TargetKind: "tin-tuc", IsSelected: false}}, staff); err != nil {
		t.Fatal(err)
	}
	if len(r2.repo.catUpdates) != 1 || r2.repo.catUpdates[0].IsSelected {
		t.Errorf("updates = %+v", r2.repo.catUpdates)
	}
	if _, err := r2.admin.SaveCategories(r2.ctx, []domain.PortalCategorySelection{
		{ExternalID: "1", Name: "x", TargetKind: "banner", IsSelected: true}}, staff); !errors.Is(err, domain.ErrPortalSelectionKind) {
		t.Errorf("banner mapping = %v", err)
	}
}

func TestPortalCategoryTreeIsAskedLiveAndMerged(t *testing.T) {
	r := newPortalRig(t)
	r.client.cats = []portal.Category{{ExternalID: "120", Name: "Tin tức"}, {ExternalID: "999", Name: "Chưa chọn"}}
	tree, err := r.admin.CategoryTree(r.ctx, audit.Actor{ID: "CB-00123", Kind: "staff"})
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Items) != 2 || !tree.Items[0].IsSelected || tree.Items[0].TargetKind != "tin-tuc" || tree.Items[1].IsSelected {
		t.Errorf("items = %+v", tree.Items)
	}
	if len(tree.Missing) != 1 || tree.Missing[0].ExternalID != "144" {
		t.Errorf("missing = %+v", tree.Missing)
	}
	if len(r.client.keys) != 1 || r.client.keys[0] != portalTestKey {
		t.Errorf("keys = %v", r.client.keys)
	}
	r.client.catErr = portal.ErrAddressRefused
	_, err = r.admin.CategoryTree(r.ctx, audit.Actor{ID: "CB-00123", Kind: "staff"})
	var call *PortalCallError
	if !errors.As(err, &call) || call.Class != "address-refused" || !errors.Is(err, ErrPortalUnavailable) {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(err.Error(), portalTestKey) {
		t.Fatal("error carries the key")
	}
}

// --- the cover pipeline for portal images ----------------------------------------------------------

func TestPortalCoverPipeline(t *testing.T) {
	jpg := testJPEG(t, 2000, 1000, 1)

	t.Run("stores only the derivative, signed by the system", func(t *testing.T) {
		rig := newCoverRig(t, nil)
		pc, err := rig.uc.preparePortalCover(rig.ctx, coverItemID, jpg, coverClock)
		if err != nil {
			t.Fatal(err)
		}
		f := pc.file
		if !strings.HasSuffix(f.ObjectKey, "/thumb-1280.jpg") || !strings.HasPrefix(f.ObjectKey, "content-source/t_") {
			t.Errorf("object key = %s, want the thumb-1280 derivative", f.ObjectKey)
		}
		if _, ok := rig.objects.produced[f.ObjectKey]; !ok || len(rig.objects.produced) != 1 {
			t.Errorf("produced = %d objects", len(rig.objects.produced))
		}
		for k := range rig.objects.private {
			if strings.Contains(k, "/original.") {
				t.Errorf("an original was stored: %s", k)
			}
		}
		if f.UploadedBy != audit.SystemActor || f.OriginalName != portalCoverName || f.SubjectID != coverItemID ||
			f.Purpose != string(storage.PurposeContentImage) || rig.scanner.scanned != 1 {
			t.Errorf("file = %+v, scanned %d", f, rig.scanner.scanned)
		}
		if len(pc.sourceSHA256) != 64 {
			t.Errorf("source hash = %q", pc.sourceSHA256)
		}
		// Recorded inside a transaction, along ADR 0052 §5's edges, ending ready.
		if err := rig.uc.db.For(rig.ctx).Tx(rig.ctx, func(tx *store.ScopedTx) error {
			return rig.uc.recordPortalCover(rig.ctx, tx, pc)
		}); err != nil {
			t.Fatal(err)
		}
		want := "pending>scanning,scanning>stored,stored>processing,processing>ready"
		if got := strings.Join(rig.files.transitions, ","); got != want {
			t.Errorf("transitions = %s, want %s", got, want)
		}
		if row := rig.files.get(f.ID); row == nil || row.Status != domain.StoredFileReady {
			t.Fatalf("row = %+v", row)
		}
		// Published and withdrawn as its public twin, never as an original.
		item := domain.NoiDungMiniApp{ID: coverItemID, TrangThai: domain.TrangThaiDangHien, CoverImageFileID: f.ID}
		var copied bool
		if err := rig.uc.db.For(rig.ctx).Tx(rig.ctx, func(tx *store.ScopedTx) error {
			var err error
			copied, err = rig.uc.publishPortalCover(rig.ctx, tx, item, coverClock)
			return err
		}); err != nil || !copied {
			t.Fatalf("publish: %v %v", copied, err)
		}
		if len(rig.objects.published) != 1 || !strings.HasPrefix(rig.objects.published[0], "public-media/") ||
			!strings.HasSuffix(rig.objects.published[0], "/thumb-1280.jpg") {
			t.Errorf("published = %v", rig.objects.published)
		}
		if err := rig.uc.withdrawPortalCover(rig.ctx, f); err != nil || len(rig.objects.unpublished) != 1 {
			t.Errorf("withdraw: %v %v", err, rig.objects.unpublished)
		}
	})
	t.Run("malware", func(t *testing.T) {
		rig := newCoverRig(t, nil)
		rig.scanner.res = malwarescan.Result{Clean: false, Signature: "Eicar"}
		_, err := rig.uc.preparePortalCover(rig.ctx, coverItemID, jpg, coverClock)
		var rej *CoverRejection
		if !errors.As(err, &rej) || rej.Reason != CoverRejectMalware || len(rig.objects.produced) != 0 {
			t.Fatalf("err = %v, produced %d", err, len(rig.objects.produced))
		}
		if portalCoverRejectReason(err) != "image-"+CoverRejectMalware {
			t.Errorf("reason = %s", portalCoverRejectReason(err))
		}
	})
	t.Run("scanner down is never clean", func(t *testing.T) {
		rig := newCoverRig(t, nil)
		rig.scanner.err = errors.New("clamd unreachable")
		_, err := rig.uc.preparePortalCover(rig.ctx, coverItemID, jpg, coverClock)
		if !errors.Is(err, ErrCoverScanUnavailable) || len(rig.objects.produced) != 0 {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("not an image", func(t *testing.T) {
		rig := newCoverRig(t, nil)
		_, err := rig.uc.preparePortalCover(rig.ctx, coverItemID, []byte("<html>not an image</html>"), coverClock)
		var rej *CoverRejection
		if !errors.As(err, &rej) || rej.Reason != CoverRejectTypeNotAllowed || rig.scanner.scanned != 0 {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("over the cap", func(t *testing.T) {
		rig := newCoverRig(t, nil)
		_, err := rig.uc.preparePortalCover(rig.ctx, coverItemID, make([]byte, portalImageMaxBytes+1), coverClock)
		var rej *CoverRejection
		if !errors.As(err, &rej) || rej.Reason != CoverRejectTooLarge {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("pipeline not configured", func(t *testing.T) {
		rig := newCoverRig(t, nil)
		rig.uc.scanner = nil
		if _, err := rig.uc.portalImageLimit(rig.ctx); !errors.Is(err, ErrCoverUploadNotConfigured) {
			t.Fatalf("err = %v", err)
		}
	})
}
