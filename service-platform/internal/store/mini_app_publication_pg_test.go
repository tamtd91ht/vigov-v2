package store

// SetMiniAppPublication (migration 0025, ADR 0070 bổ sung 13/10/2026) against a real PostgreSQL —
// skipped without VIGOV_TEST_DSN. What only the database can show: the mark and its audit entry are ONE
// transaction, published_by is the operator's business code, a repeat writes nothing, every refusal
// writes nothing, and the registry read returns the mark.

import (
	"errors"
	"testing"

	corestore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-platform/internal/domain"
)

func TestPgMiniAppPublication(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Thăng Bình", true)
	themMiniApp(t, db, appOldFake, "rieng", ulidA, true)
	w := NewRegistryWriter(corestore.New(db))
	ctx := inCommune(ulidA)

	if changed, err := w.SetMiniAppPublication(ctx, appOldFake, true, "Zalo đã phát hành", operatorFake); err != nil || !changed {
		t.Fatalf("mark: %v %v", changed, err)
	}
	if count(t, db, `SELECT count(*) FROM mini_app WHERE app_id = $1 AND published_at IS NOT NULL
		AND published_by = 'VH-00001' AND cap_nhat_boi = 'VH-00001'`, appOldFake) != 1 {
		t.Fatal("row not marked by the operator's business code")
	}
	_, apps, err := NewOperatorRegistry(db).Commune(ctx, ulidA)
	if err != nil || len(apps) != 1 || !apps[0].Published() || apps[0].PublishedBy != "VH-00001" {
		t.Fatalf("detail read: %+v %v", apps, err)
	}
	if got, err := NewCommuneLaunchStore(corestore.New(db)).CommuneLaunch(ctx); err != nil ||
		len(got.LiveOwnAppIDs) != 1 || got.LiveOwnAppIDs[0] != appOldFake {
		t.Fatalf("QR read after marking: %+v %v", got, err)
	}

	// The same state again: no write, no entry.
	if changed, err := w.SetMiniAppPublication(ctx, appOldFake, true, "Lần hai", operatorFake); err != nil || changed {
		t.Fatalf("mark twice: %v %v, want unchanged", changed, err)
	}

	if changed, err := w.SetMiniAppPublication(ctx, appOldFake, false, "Zalo gỡ bản phát hành", operatorFake); err != nil || !changed {
		t.Fatalf("unmark: %v %v", changed, err)
	}
	if count(t, db, `SELECT count(*) FROM mini_app WHERE app_id = $1 AND published_at IS NULL AND published_by IS NULL`, appOldFake) != 1 {
		t.Fatal("mark not cleared")
	}
	if changed, err := w.SetMiniAppPublication(ctx, appOldFake, false, "Lần hai", operatorFake); err != nil || changed {
		t.Fatalf("unmark twice: %v %v, want unchanged", changed, err)
	}

	a := auditRows(t, db, ulidA)
	if len(a) != 2 || a[0].action != ActionMarkMiniAppPublished || a[1].action != ActionUnmarkMiniAppPublished {
		t.Fatalf("audit = %+v, want danh_dau then bo_danh_dau", a)
	}
	before0, after0 := a[0].delta["truoc"].(map[string]any), a[0].delta["sau"].(map[string]any)
	if before0["da_phat_hanh"] != false || after0["da_phat_hanh"] != true || after0["phat_hanh_boi"] != "VH-00001" ||
		after0["phat_hanh_luc"] == nil || a[0].delta["ly_do"] != "Zalo đã phát hành" || a[0].subject != "MiniApp "+appOldFake {
		t.Errorf("mark entry = %+v", a[0])
	}
	before1, after1 := a[1].delta["truoc"].(map[string]any), a[1].delta["sau"].(map[string]any)
	if before1["da_phat_hanh"] != true || before1["phat_hanh_boi"] != "VH-00001" || after1["da_phat_hanh"] != false ||
		after1["phat_hanh_boi"] != nil || a[1].delta["ly_do"] != "Zalo gỡ bản phát hành" {
		t.Errorf("unmark entry = %+v", a[1])
	}
	for _, e := range a {
		if e.actorID != "VH-00001" || e.actorKind != domain.AuditKindOperator || e.actorIP != "203.0.113.7" {
			t.Errorf("entry actor = %+v", e)
		}
	}
}

// The audit entry is refused by the database: the mark must not survive it (rule 6 invariant 3).
func TestPgMiniAppPublicationAuditFailureWritesNothing(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Thăng Bình", true)
	themMiniApp(t, db, appOldFake, "rieng", ulidA, true)
	if _, err := db.Exec(`ALTER TABLE audit_log ADD CONSTRAINT test_refuse_publication
		CHECK (action <> 'danh_dau_phat_hanh_mini_app') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	w := NewRegistryWriter(corestore.New(db))
	if _, err := w.SetMiniAppPublication(inCommune(ulidA), appOldFake, true, "Phát hành", operatorFake); err == nil {
		t.Fatal("marking succeeded although its audit entry was refused")
	}
	if count(t, db, `SELECT count(*) FROM mini_app WHERE published_at IS NOT NULL OR published_by IS NOT NULL`) != 0 ||
		count(t, db, `SELECT count(*) FROM audit_log`) != 0 {
		t.Fatal("a mark whose audit entry failed was left behind")
	}
}

func TestPgMiniAppPublicationRefusals(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Thăng Bình", true)
	themXa(t, db, ulidB, "Xã Tân Phú", true)
	themXa(t, db, ulidNew, "Xã Cũ", false)
	themMiniApp(t, db, appNewFake, "rieng", ulidA, true)
	themMiniApp(t, db, "2000000000000000001", "rieng", ulidA, false)  // switched off
	themMiniApp(t, db, appOtherFake, "rieng", ulidB, true)            // another commune's
	themMiniApp(t, db, "2000000000000000002", "chinh", nil, true)     // the shared app
	themMiniApp(t, db, "2000000000000000003", "rieng", ulidNew, true) // on an inactive commune
	themMiniApp(t, db, "2000000000000000004", "rieng", ulidA, true)   // soft-deleted below
	if _, err := db.Exec(`UPDATE mini_app SET deleted_at = now(), deleted_by = 'VH-00001', delete_reason = 'thử'
		WHERE app_id = '2000000000000000004'`); err != nil {
		t.Fatal(err)
	}
	w := NewRegistryWriter(corestore.New(db))

	for name, c := range map[string]struct {
		commune, app string
		want         error
	}{
		"another commune's": {ulidA, appOtherFake, ErrMiniAppNotInCommune},
		"the shared app":    {ulidA, "2000000000000000002", ErrMiniAppNotInCommune},
		"unknown App ID":    {ulidA, "2999999999999999999", ErrMiniAppNotInCommune},
		"soft-deleted":      {ulidA, "2000000000000000004", ErrMiniAppNotInCommune},
		"switched off":      {ulidA, "2000000000000000001", ErrMiniAppInactive},
		"inactive commune":  {ulidNew, "2000000000000000003", ErrCommuneInactive},
		"unknown commune":   {"01JD8ZQK9M3NPXR7TVWYB2C4EZ", appNewFake, ErrCommuneNotFound},
	} {
		if _, err := w.SetMiniAppPublication(inCommune(c.commune), c.app, true, "Lý do", operatorFake); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", name, err, c.want)
		}
	}
	if _, err := w.SetMiniAppPublication(inCommune(ulidA), appNewFake, true, "Lý do", domain.OperatorActor{}); !errors.Is(err, domain.ErrNoActor) {
		t.Errorf("empty actor: %v", err)
	}
	// Clearing a mark that is not there on a switched-off row is the no-op, not a refusal.
	if changed, err := w.SetMiniAppPublication(inCommune(ulidA), "2000000000000000001", false, "Lý do", operatorFake); err != nil || changed {
		t.Errorf("unmark switched-off: %v %v, want unchanged", changed, err)
	}
	if count(t, db, `SELECT count(*) FROM mini_app WHERE published_at IS NOT NULL`) != 0 ||
		count(t, db, `SELECT count(*) FROM audit_log`) != 0 {
		t.Fatal("a refused publication wrote something")
	}
}
