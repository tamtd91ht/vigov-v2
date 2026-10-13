package store

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	corestore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-platform/internal/domain"
)

// The soft delete of a commune (migration 0026, ADR 0048 §Sửa đổi 13/10/2026 #7–#8) against a real
// PostgreSQL — skipped without VIGOV_TEST_DSN. What only the database can show: deactivated_at moving
// with dang_hoat_dong under 0026's CHECK, the 7-day wait measured by the database clock, the soft-delete
// columns and the `xoa_mem_xa` entry committing together, and every read path then failing to find the
// commune while its domains stay reserved.

// ageDeactivation moves a fixture commune's deactivated_at into the past. Test schema only.
func ageDeactivation(t *testing.T, db *sql.DB, id string, ago string) {
	t.Helper()
	if _, err := db.Exec(`UPDATE tenant SET deactivated_at = now() - $2::interval WHERE id = $1`, id, ago); err != nil {
		t.Fatal(err)
	}
}

func TestPgSetActivationWritesDeactivatedAt(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Thăng Bình", true)
	w := NewRegistryWriter(corestore.New(db))

	if _, _, err := w.SetActivation(inCommune(ulidA), false, "Tạm dừng", operatorFake); err != nil {
		t.Fatalf("deactivate (a 23514 here is SetActivation not writing deactivated_at): %v", err)
	}
	if count(t, db, `SELECT count(*) FROM tenant WHERE id = $1 AND NOT dang_hoat_dong
		AND deactivated_at IS NOT NULL AND deactivated_at > now() - interval '1 minute'`, ulidA) != 1 {
		t.Fatal("deactivation did not record deactivated_at = now()")
	}
	if _, _, err := w.SetActivation(inCommune(ulidA), true, "Mở lại", operatorFake); err != nil {
		t.Fatalf("reactivate: %v", err)
	}
	if count(t, db, `SELECT count(*) FROM tenant WHERE id = $1 AND dang_hoat_dong AND deactivated_at IS NULL`, ulidA) != 1 {
		t.Fatal("reactivation did not clear deactivated_at")
	}
}

func TestPgDeleteCommuneRefusals(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Đang Chạy", true)
	themHost(t, db, "a.vigov.vn", ulidA, true)
	themXa(t, db, ulidB, "Xã Mới Ngừng", false)
	themHost(t, db, "b.vigov.vn", ulidB, true)
	themHost(t, db, "b2.example.gov.vn", ulidB, false)
	ageDeactivation(t, db, ulidB, "6 days 23 hours")
	w := NewRegistryWriter(corestore.New(db))

	if _, _, err := w.DeleteCommune(inCommune("01JD8ZQK9M3NPXR7TVWYB2C4EZ"), "x.vigov.vn", "Lý do", operatorFake); !errors.Is(err, ErrCommuneNotFound) {
		t.Errorf("unknown commune: %v, want ErrCommuneNotFound", err)
	}
	if _, _, err := w.DeleteCommune(inCommune(ulidA), "a.vigov.vn", "Lý do", operatorFake); !errors.Is(err, ErrCommuneActive) {
		t.Errorf("active commune: %v, want ErrCommuneActive", err)
	}
	_, _, err := w.DeleteCommune(inCommune(ulidB), "b.vigov.vn", "Lý do", operatorFake)
	var tooRecent *InactiveTooRecentError
	if !errors.As(err, &tooRecent) || !errors.Is(err, ErrCommuneInactiveTooRecent) {
		t.Fatalf("inactive 6d23h: %v, want *InactiveTooRecentError", err)
	}
	// earliest = deactivated_at + 7 days ≈ one hour from now.
	if d := time.Until(tooRecent.EarliestAt); d < 50*time.Minute || d > 70*time.Minute {
		t.Errorf("earliest %v is %v from now, want about one hour", tooRecent.EarliestAt, d)
	}

	ageDeactivation(t, db, ulidB, "7 days 1 minute")
	if _, _, err := w.DeleteCommune(inCommune(ulidB), "khac.vigov.vn", "Lý do", operatorFake); !errors.Is(err, domain.ErrDeletionConfirmMismatch) {
		t.Errorf("wrong domain: %v, want ErrDeletionConfirmMismatch", err)
	}
	// Another commune's domain is not this commune's.
	if _, _, err := w.DeleteCommune(inCommune(ulidB), "a.vigov.vn", "Lý do", operatorFake); !errors.Is(err, domain.ErrDeletionConfirmMismatch) {
		t.Errorf("another commune's domain: %v, want ErrDeletionConfirmMismatch", err)
	}
	if count(t, db, `SELECT count(*) FROM tenant WHERE deleted_at IS NOT NULL`) != 0 ||
		count(t, db, `SELECT count(*) FROM audit_log WHERE action = $1`, ActionSoftDeleteCommune) != 0 {
		t.Fatal("a refused deletion changed a row or left an entry")
	}
}

func TestPgDeleteCommuneSoftDeletesAndHidesEverywhere(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Còn Lại", true)
	themHost(t, db, "a.vigov.vn", ulidA, true)
	themXa(t, db, ulidB, "Xã Đã Sáp Nhập", false)
	themHost(t, db, "b.vigov.vn", ulidB, true)
	themHost(t, db, "b2.example.gov.vn", ulidB, false)
	ageDeactivation(t, db, ulidB, "8 days")
	var launchCode string
	if err := db.QueryRow(`SELECT launch_code FROM tenant WHERE id = $1`, ulidB).Scan(&launchCode); err != nil {
		t.Fatal(err)
	}
	w := NewRegistryWriter(corestore.New(db))

	// Either domain confirms, trimmed and case-folded.
	del, hosts, err := w.DeleteCommune(inCommune(ulidB), "  B2.Example.GOV.vn ", "Đã sáp nhập theo NQ-FAKE", operatorFake)
	if err != nil {
		t.Fatal(err)
	}
	if del.ID != ulidB || del.Name != "Xã Đã Sáp Nhập" || del.DeletedBy != "VH-00001" || del.DeletedAt.IsZero() {
		t.Errorf("deletion %+v", del)
	}
	if len(hosts) != 2 {
		t.Errorf("hosts %v", hosts)
	}
	if count(t, db, `SELECT count(*) FROM tenant WHERE id = $1 AND deleted_at IS NOT NULL
		AND deleted_by = 'VH-00001' AND delete_reason = 'Đã sáp nhập theo NQ-FAKE'`, ulidB) != 1 {
		t.Fatal("soft-delete columns not written")
	}
	// Nothing removed: the row, its domains (still reserved), its trail.
	if count(t, db, `SELECT count(*) FROM tenant WHERE id = $1`, ulidB) != 1 ||
		count(t, db, `SELECT count(*) FROM tenant_domain WHERE tenant_id = $1`, ulidB) != 2 {
		t.Fatal("a deletion removed rows")
	}
	a := auditRows(t, db, ulidB)
	last := a[len(a)-1]
	if last.action != ActionSoftDeleteCommune || last.actorID != "VH-00001" || last.actorKind != domain.AuditKindOperator ||
		last.subject != "Xã Đã Sáp Nhập" || last.delta["ly_do"] != "Đã sáp nhập theo NQ-FAKE" {
		t.Errorf("audit entry %+v", last)
	}
	if sau, _ := last.delta["sau"].(map[string]any); sau["da_xoa_mem"] != true || sau["xoa_boi"] != "VH-00001" {
		t.Errorf("delta.sau %+v", last.delta["sau"])
	}

	// Every READ path: unknown.
	d := NewDirectory(db, "")
	if _, ok := d.ByHost(context.Background(), "b.vigov.vn"); ok {
		t.Error("ByHost resolved a deleted commune")
	}
	if _, err := d.ByHostErr(context.Background(), "b2.example.gov.vn"); !errors.Is(err, ErrKhongCoXa) {
		t.Errorf("ByHostErr: %v, want ErrKhongCoXa (unknown, not merely inactive)", err)
	}
	if _, err := d.ByLaunchCodeErr(context.Background(), launchCode); !errors.Is(err, ErrKhongCoXa) {
		t.Errorf("ByLaunchCodeErr: %v, want ErrKhongCoXa", err)
	}
	if _, err := d.ByID(context.Background(), tenant.ID(ulidB)); !errors.Is(err, ErrKhongCoXa) {
		t.Errorf("ByID: %v, want ErrKhongCoXa", err)
	}
	// ... but the hosts stay HELD: no other commune can ever take them.
	if held, err := d.HostHeld(context.Background(), "b.vigov.vn"); err != nil || !held {
		t.Errorf("HostHeld = %v, %v; a deleted commune's host must stay reserved", held, err)
	}
	if err := w.AddDomain(inCommune(ulidA), "b.vigov.vn", operatorFake); !errors.Is(err, ErrDomainTaken) {
		t.Errorf("re-using a deleted commune's host: %v, want ErrDomainTaken", err)
	}
	// ... and its name keeps blocking a duplicate in the province.
	if _, err := w.CreateCommune(inCommune(ulidNew),
		NewCommune{Name: "Xã Đã Sáp Nhập", ProvinceID: seededProvinceID, Host: "moi.vigov.vn"}, operatorFake); !errors.Is(err, ErrDuplicateName) {
		t.Errorf("re-using a deleted commune's name: %v, want ErrDuplicateName", err)
	}

	r := NewOperatorRegistry(db)
	req, err := pageFirst("50")
	if err != nil {
		t.Fatal(err)
	}
	p, err := r.ListCommunes(context.Background(), req)
	if err != nil || len(p.Items) != 1 || p.Items[0].ID != ulidA {
		t.Errorf("list = %+v, %v; want only the live commune", p.Items, err)
	}
	if _, _, err := r.Commune(context.Background(), ulidB); !errors.Is(err, ErrCommuneNotFound) {
		t.Errorf("detail: %v, want ErrCommuneNotFound", err)
	}
	if _, err := NewSharedMiniAppStore(db).CommuneLaunchTarget(context.Background(), ulidB); !errors.Is(err, ErrCommuneNotFound) {
		t.Errorf("CommuneLaunchTarget: %v, want ErrCommuneNotFound", err)
	}
	if _, err := NewCommuneLaunchStore(corestore.New(db)).CommuneLaunch(inCommune(ulidB)); !errors.Is(err, ErrCommuneNotFound) {
		t.Errorf("CommuneLaunch: %v, want ErrCommuneNotFound", err)
	}
	if _, _, err := NewMapFrameDefaultStore(corestore.New(db)).OperatorMapFrameDefault(inCommune(ulidB)); !errors.Is(err, ErrCommuneNotFound) {
		t.Errorf("OperatorMapFrameDefault: %v, want ErrCommuneNotFound", err)
	}

	// Every WRITE: not found — no reactivation, no domain, no second deletion.
	if _, _, err := w.SetActivation(inCommune(ulidB), true, "Mở lại", operatorFake); !errors.Is(err, ErrCommuneNotFound) {
		t.Errorf("reactivating a deleted commune: %v, want ErrCommuneNotFound", err)
	}
	if err := w.AddDomain(inCommune(ulidB), "b3.vigov.vn", operatorFake); !errors.Is(err, ErrCommuneNotFound) {
		t.Errorf("adding a domain to a deleted commune: %v, want ErrCommuneNotFound", err)
	}
	if _, _, err := w.DeleteCommune(inCommune(ulidB), "b.vigov.vn", "Lần hai", operatorFake); !errors.Is(err, ErrCommuneNotFound) {
		t.Errorf("deleting twice: %v, want ErrCommuneNotFound", err)
	}
	if n := count(t, db, `SELECT count(*) FROM audit_log WHERE tenant_id = $1 AND action = $2`, ulidB, ActionSoftDeleteCommune); n != 1 {
		t.Errorf("%d xoa_mem_xa entries, want 1", n)
	}
}

// 0026's CHECKs are the database's own copy of the rules: an active commune cannot be marked deleted,
// and the actor must be a VH- code, even by a write that bypasses DeleteCommune.
func TestPgSoftDeleteChecks(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Đang Chạy", true)
	themXa(t, db, ulidB, "Xã Đã Ngừng", false)
	for _, c := range []struct {
		name, id, by string
	}{
		{"active commune", ulidA, "VH-00001"},
		{"internal id as actor", ulidB, "01JD8ZQK9M3NPXR7TVWYB2C4EZ"},
		{"staff code as actor", ulidB, "CB-00001"},
	} {
		if _, err := db.Exec(`UPDATE tenant SET deleted_at = now(), deleted_by = $2, delete_reason = 'x' WHERE id = $1`,
			c.id, c.by); err == nil {
			t.Errorf("%s: the database accepted the soft delete", c.name)
		}
	}
}
