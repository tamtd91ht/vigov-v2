package store

// ADR 0070 (as amended 05/10/2026) against a real PostgreSQL — skipped without VIGOV_TEST_DSN. What
// only the database can show: the replacement is ONE transaction (an audit failure leaves neither row
// changed nor any entry), the old row is SOFT-DELETED with who/why, every refusal writes nothing, a
// row of another commune is never touched, a removed App ID can never be bound again, and a commune
// never ends with two running dedicated apps through these writes.

import (
	"errors"
	"testing"

	corestore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-platform/internal/domain"
)

const (
	appOldFake   = "3291993990104489440"
	appNewFake   = "3043188591857102858"
	appOtherFake = "1111111111111111111"
)

func TestPgReplaceMiniAppOneTransaction(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Thăng Bình", true)
	themMiniApp(t, db, appOldFake, "rieng", ulidA, true)
	w := NewRegistryWriter(corestore.New(db))

	app, err := w.ReplaceMiniApp(inCommune(ulidA), appOldFake, appNewFake, "Xã đổi App ID", operatorFake)
	if err != nil || app.AppID != appNewFake || !app.Active || app.CreatedBy != "VH-00001" {
		t.Fatalf("replace: %+v %v", app, err)
	}
	if count(t, db, `SELECT count(*) FROM mini_app WHERE app_id = $1 AND tenant_id = $2 AND dang_hoat_dong`, appNewFake, ulidA) != 1 {
		t.Error("new row not active for the commune")
	}
	if count(t, db, `SELECT count(*) FROM mini_app WHERE app_id = $1 AND NOT dang_hoat_dong AND deleted_at IS NOT NULL
		AND deleted_by = 'VH-00001' AND delete_reason = 'Xã đổi App ID' AND cap_nhat_boi = 'VH-00001'`, appOldFake) != 1 {
		t.Error("old row not soft-deleted (kept, deleted_at/by/reason set by the operator, switched off)")
	}
	a := auditRows(t, db, ulidA)
	if len(a) != 2 || a[0].action != ActionAttachMiniApp || a[1].action != ActionDeactivateMiniApp {
		t.Fatalf("audit = %+v, want gan_mini_app then tat_mini_app", a)
	}
	if a[0].delta["thay_cho"] != appOldFake || a[0].subject != "MiniApp "+appNewFake {
		t.Errorf("gan_mini_app entry = %+v", a[0])
	}
	if a[1].delta["ly_do"] != "Xã đổi App ID" || a[1].subject != "MiniApp "+appOldFake ||
		a[1].delta["truoc"].(map[string]any)["dang_hoat_dong"] != true ||
		a[1].delta["sau"].(map[string]any)["dang_hoat_dong"] != false ||
		a[1].delta["sau"].(map[string]any)["da_xoa_mem"] != true || a[1].delta["thay_bang"] != appNewFake {
		t.Errorf("tat_mini_app entry = %+v", a[1])
	}
	for _, e := range a {
		if e.actorID != "VH-00001" || e.actorKind != domain.AuditKindOperator {
			t.Errorf("entry actor = %+v", e)
		}
	}
}

// The second audit entry is refused by the database: the INSERT of the new row, the UPDATE of the old
// one and the first entry must ALL be gone. Half of it is a commune with no app (ADR 0070 #1).
func TestPgReplaceMiniAppAuditFailureWritesNothing(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Thăng Bình", true)
	themMiniApp(t, db, appOldFake, "rieng", ulidA, true)
	if _, err := db.Exec(`ALTER TABLE audit_log ADD CONSTRAINT test_refuse_tat_mini_app
		CHECK (action <> 'tat_mini_app') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	w := NewRegistryWriter(corestore.New(db))
	if _, err := w.ReplaceMiniApp(inCommune(ulidA), appOldFake, appNewFake, "Xã đổi App ID", operatorFake); err == nil {
		t.Fatal("replacement succeeded although its audit entry was refused")
	}
	if count(t, db, `SELECT count(*) FROM mini_app WHERE app_id = $1`, appNewFake) != 0 ||
		count(t, db, `SELECT count(*) FROM mini_app WHERE app_id = $1 AND dang_hoat_dong AND deleted_at IS NULL`, appOldFake) != 1 ||
		count(t, db, `SELECT count(*) FROM audit_log WHERE tenant_id = $1`, ulidA) != 0 {
		t.Fatal("a replacement whose audit entry failed left a row, a change or an entry behind")
	}
}

func TestPgReplaceMiniAppRefusals(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Thăng Bình", true)
	themXa(t, db, ulidB, "Xã Tân Phú", true)
	themXa(t, db, ulidNew, "Xã Cũ", false)
	themMiniApp(t, db, appOldFake, "rieng", ulidA, true)
	themMiniApp(t, db, "2000000000000000001", "rieng", ulidA, false)  // switched off
	themMiniApp(t, db, appOtherFake, "rieng", ulidB, true)            // another commune's
	themMiniApp(t, db, "2000000000000000002", "chinh", nil, true)     // the main app
	themMiniApp(t, db, "2000000000000000003", "rieng", ulidNew, true) // on an inactive commune
	themMiniApp(t, db, "2000000000000000004", "rieng", ulidA, true)   // soft-deleted below
	if _, err := db.Exec(`UPDATE mini_app SET deleted_at = now(), deleted_by = 'VH-00001', delete_reason = 'thử'
		WHERE app_id = '2000000000000000004'`); err != nil {
		t.Fatal(err)
	}
	w := NewRegistryWriter(corestore.New(db))

	for name, c := range map[string]struct {
		commune, old, new string
		want              error
	}{
		"unknown commune":         {"01JD8ZQK9M3NPXR7TVWYB2C4EZ", appOldFake, appNewFake, ErrCommuneNotFound},
		"inactive commune":        {ulidNew, "2000000000000000003", appNewFake, ErrCommuneInactive},
		"old unknown":             {ulidA, "2999999999999999999", appNewFake, ErrMiniAppNotInCommune},
		"old of another commune":  {ulidA, appOtherFake, appNewFake, ErrMiniAppNotInCommune},
		"old is the main app":     {ulidA, "2000000000000000002", appNewFake, ErrMiniAppNotInCommune},
		"old soft-deleted":        {ulidA, "2000000000000000004", appNewFake, ErrMiniAppNotInCommune},
		"old switched off":        {ulidA, "2000000000000000001", appNewFake, ErrMiniAppInactive},
		"new held by another":     {ulidA, appOldFake, appOtherFake, ErrMiniAppTaken},
		"new is soft-deleted row": {ulidA, appOldFake, "2000000000000000004", ErrMiniAppRemoved},
		"new equals old":          {ulidA, appOldFake, appOldFake, ErrMiniAppTaken},
	} {
		if _, err := w.ReplaceMiniApp(inCommune(c.commune), c.old, c.new, "Đổi", operatorFake); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", name, err, c.want)
		}
	}
	if _, err := w.ReplaceMiniApp(inCommune(ulidA), appOldFake, appNewFake, "Đổi", domain.OperatorActor{}); !errors.Is(err, domain.ErrNoActor) {
		t.Errorf("empty actor: %v", err)
	}
	if count(t, db, `SELECT count(*) FROM mini_app WHERE app_id = $1`, appNewFake) != 0 ||
		count(t, db, `SELECT count(*) FROM mini_app WHERE dang_hoat_dong`) != 5 ||
		count(t, db, `SELECT count(*) FROM audit_log`) != 0 {
		t.Fatal("a refused replacement wrote something")
	}
}

func TestPgMiniAppRemoval(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Thăng Bình", true)
	themMiniApp(t, db, appOldFake, "rieng", ulidA, true)
	themMiniApp(t, db, appNewFake, "rieng", ulidA, false) // switched off before 05/10/2026
	w := NewRegistryWriter(corestore.New(db))
	ctx := inCommune(ulidA)

	if changed, err := w.RemoveMiniApp(ctx, appOldFake, "Xã ngừng dùng app riêng", operatorFake); err != nil || !changed {
		t.Fatalf("remove: %v %v", changed, err)
	}
	if count(t, db, `SELECT count(*) FROM mini_app WHERE app_id = $1 AND NOT dang_hoat_dong AND deleted_at IS NOT NULL
		AND deleted_by = 'VH-00001' AND delete_reason = 'Xã ngừng dùng app riêng'`, appOldFake) != 1 {
		t.Error("removed row not soft-deleted with who and why")
	}
	if _, err := NewDirectory(db, "").MiniApp(ctx, appOldFake); !errors.Is(err, ErrKhongCoMiniApp) {
		t.Errorf("a removed App ID still resolves: %v", err)
	}
	if _, apps, err := NewOperatorRegistry(db).Commune(ctx, ulidA); err != nil || len(apps) != 1 || apps[0].AppID != appNewFake {
		t.Errorf("the detail still lists the removed row: %+v %v", apps, err)
	}
	if held, err := NewOperatorRegistry(db).HeldMiniApp(ctx, ulidA, appOldFake); err != nil || !held {
		t.Errorf("HeldMiniApp must still see the removed row of this commune: %v %v", held, err)
	}
	// Already removed: a no-op, no entry — the repeat only retries the secret retirement.
	if changed, err := w.RemoveMiniApp(ctx, appOldFake, "Lần hai", operatorFake); err != nil || changed {
		t.Errorf("remove twice: %v %v, want unchanged", changed, err)
	}
	// A legacy switched-off row is removed too; its entry says it was already off.
	if changed, err := w.RemoveMiniApp(ctx, appNewFake, "Dọn dòng cũ", operatorFake); err != nil || !changed {
		t.Fatalf("remove switched-off: %v %v", changed, err)
	}
	// Never bound again — not even to the commune that held it.
	for _, id := range []string{appOldFake, appNewFake} {
		if _, err := w.AttachMiniApp(ctx, id, "", operatorFake); !errors.Is(err, ErrMiniAppRemoved) {
			t.Errorf("re-attach %s: %v, want ErrMiniAppRemoved", id, err)
		}
	}

	a := auditRows(t, db, ulidA)
	if len(a) != 2 || a[0].action != ActionDeactivateMiniApp || a[1].action != ActionDeactivateMiniApp {
		t.Fatalf("audit = %+v, want two tat_mini_app", a)
	}
	if a[0].delta["ly_do"] != "Xã ngừng dùng app riêng" ||
		a[0].delta["truoc"].(map[string]any)["dang_hoat_dong"] != true ||
		a[0].delta["sau"].(map[string]any)["da_xoa_mem"] != true ||
		a[1].delta["truoc"].(map[string]any)["dang_hoat_dong"] != false {
		t.Errorf("entries = %+v", a)
	}
	for _, e := range a {
		if e.actorID != "VH-00001" || e.actorKind != domain.AuditKindOperator {
			t.Errorf("entry actor = %+v", e)
		}
	}
}

func TestPgMiniAppRemovalRefusals(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Thăng Bình", true)
	themXa(t, db, ulidB, "Xã Tân Phú", true)
	themXa(t, db, ulidNew, "Xã Cũ", false)
	themMiniApp(t, db, appNewFake, "rieng", ulidA, true)
	themMiniApp(t, db, appOtherFake, "rieng", ulidB, true)
	themMiniApp(t, db, "2000000000000000002", "chinh", nil, true)
	themMiniApp(t, db, "2000000000000000003", "rieng", ulidNew, true)
	w := NewRegistryWriter(corestore.New(db))

	for name, c := range map[string]struct {
		commune, app string
		want         error
	}{
		// Never removed from — or moved to — another commune. Same answer as unknown.
		"another commune's": {ulidA, appOtherFake, ErrMiniAppNotInCommune},
		"the main app":      {ulidA, "2000000000000000002", ErrMiniAppNotInCommune},
		"unknown App ID":    {ulidA, "2999999999999999999", ErrMiniAppNotInCommune},
		"inactive commune":  {ulidNew, "2000000000000000003", ErrCommuneInactive},
		"unknown commune":   {"01JD8ZQK9M3NPXR7TVWYB2C4EZ", appNewFake, ErrCommuneNotFound},
	} {
		if _, err := w.RemoveMiniApp(inCommune(c.commune), c.app, "Lý do", operatorFake); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", name, err, c.want)
		}
	}
	if _, err := w.RemoveMiniApp(inCommune(ulidA), appNewFake, "Lý do", domain.OperatorActor{}); !errors.Is(err, domain.ErrNoActor) {
		t.Errorf("empty actor: %v", err)
	}
	if count(t, db, `SELECT count(*) FROM mini_app WHERE deleted_at IS NOT NULL`) != 0 ||
		count(t, db, `SELECT count(*) FROM mini_app WHERE dang_hoat_dong`) != 4 ||
		count(t, db, `SELECT count(*) FROM audit_log`) != 0 {
		t.Fatal("a refused removal wrote something")
	}
	if held, err := NewOperatorRegistry(db).HeldMiniApp(inCommune(ulidA), ulidA, appOtherFake); err != nil || held {
		t.Errorf("HeldMiniApp sees another commune's App ID: %v %v", held, err)
	}
}

// ADR 0070 #1 tightens ATTACH: a commune with a running dedicated app is refused; once that app is
// removed, attaching ANOTHER App ID is allowed again.
func TestPgAttachRefusedWhileAnotherAppRuns(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Thăng Bình", true)
	w := NewRegistryWriter(corestore.New(db))
	ctx := inCommune(ulidA)
	if _, err := w.AttachMiniApp(ctx, appOldFake, "", operatorFake); err != nil {
		t.Fatal(err)
	}
	if _, err := w.AttachMiniApp(ctx, appNewFake, "", operatorFake); !errors.Is(err, ErrMiniAppAlreadyRunning) {
		t.Fatalf("second attach: %v, want ErrMiniAppAlreadyRunning", err)
	}
	if count(t, db, `SELECT count(*) FROM mini_app WHERE app_id = $1`, appNewFake) != 0 {
		t.Fatal("a refused attach wrote a row")
	}
	if _, err := w.RemoveMiniApp(ctx, appOldFake, "Gỡ", operatorFake); err != nil {
		t.Fatal(err)
	}
	if _, err := w.AttachMiniApp(ctx, appNewFake, "", operatorFake); err != nil {
		t.Fatalf("attach after detach: %v", err)
	}
}
