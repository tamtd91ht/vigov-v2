package store

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	corestore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-platform/internal/domain"
)

// Integration tests for migration 0006 against a real PostgreSQL — skipped without VIGOV_TEST_DSN,
// like every *_pg_test.go here. The CHECK constraints, the hard-delete trigger and the scoped read
// are behaviour the database owns; a mock would only agree with our assumptions.

// testActor is a staff business code shape (rule 6, invariant 8) — never an internal id.
const testActor = "CB-00001"

func insertMiniApp(t *testing.T, db *sql.DB, appID, mode string, tenantID any, active bool) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO mini_app (app_id, mode, tenant_id, is_active, created_by, updated_by)
		 VALUES ($1,$2,$3,$4,$5,$5)`, appID, mode, tenantID, active, testActor)
	if err != nil {
		t.Fatalf("thêm mini app %s: %v", appID, err)
	}
}

func TestPgMiniAppNotRegistered(t *testing.T) {
	db, _ := openTestDB(t)
	runMigrations(t, db)
	_, err := NewDirectory(db).MiniApp(context.Background(), "9999999999")
	if !errors.Is(err, ErrMiniAppNotFound) {
		t.Fatalf("err = %v, muốn ErrMiniAppNotFound", err)
	}
}

func TestPgMiniAppMainAndCommune(t *testing.T) {
	db, _ := openTestDB(t)
	runMigrations(t, db)
	insertTenant(t, db, ulidA, "Xã Thăng Bình", true)
	insertTenant(t, db, ulidB, "Xã Cũ", false)
	insertMiniApp(t, db, "1001", "chinh", nil, true)
	insertMiniApp(t, db, "1002", "rieng", ulidA, true)
	insertMiniApp(t, db, "1003", "rieng", ulidB, true)

	d := NewDirectory(db)
	ctx := context.Background()

	mainApp, err := d.MiniApp(ctx, "1001")
	if err != nil || mainApp.Mode != domain.MiniAppModeMain || mainApp.Tenant != nil {
		t.Fatalf("app chính = %+v, err %v", mainApp, err)
	}

	communeApp, err := d.MiniApp(ctx, "1002")
	if err != nil || communeApp.Mode != domain.MiniAppModeCommune || communeApp.Tenant == nil {
		t.Fatalf("app riêng = %+v, err %v", communeApp, err)
	}
	if communeApp.Tenant.ID != ulidA || communeApp.Tenant.Name != "Xã Thăng Bình" || !communeApp.Tenant.IsActive ||
		communeApp.Tenant.Province != "Thành phố Đà Nẵng" {
		t.Errorf("xã của app riêng = %+v", communeApp.Tenant)
	}

	// Inactive bound commune: the app IS returned, with the commune's state — the caller refuses.
	inactive, err := d.MiniApp(ctx, "1003")
	if err != nil || inactive.Tenant == nil || inactive.Tenant.ID != ulidB || inactive.Tenant.IsActive {
		t.Fatalf("app riêng của xã ngừng hoạt động = %+v, err %v", inactive, err)
	}
}

func TestPgMiniAppInactiveOrSoftDeletedIsNotRegistered(t *testing.T) {
	db, _ := openTestDB(t)
	runMigrations(t, db)
	insertTenant(t, db, ulidA, "Xã Thăng Bình", true)
	insertMiniApp(t, db, "2001", "rieng", ulidA, false)
	insertMiniApp(t, db, "2002", "rieng", ulidA, true)
	if _, err := db.Exec(`UPDATE mini_app SET deleted_at = now(), deleted_by = $1, delete_reason = 'thử'
		WHERE app_id = '2002'`, testActor); err != nil {
		t.Fatalf("xoá mềm: %v", err)
	}

	d := NewDirectory(db)
	for _, id := range []string{"2001", "2002"} {
		if _, err := d.MiniApp(context.Background(), id); !errors.Is(err, ErrMiniAppNotFound) {
			t.Errorf("app %s: err = %v, muốn ErrMiniAppNotFound", id, err)
		}
	}
}

// The CHECK is what makes "main app with a commune" and "dedicated app without one" impossible —
// the two shapes that would put a default commune on the isolation path, or name nobody.
func TestPgMiniAppModeAndTenantConstraint(t *testing.T) {
	db, _ := openTestDB(t)
	runMigrations(t, db)
	insertTenant(t, db, ulidA, "Xã Thăng Bình", true)

	// One App ID per case: a row wrongly accepted must not make the NEXT case fail on the primary
	// key and pass for the wrong reason.
	for _, c := range []struct {
		name, appID, mode string
		tenantID          any
	}{
		{"chính có xã", "3001", "chinh", ulidA},
		{"riêng không xã", "3002", "rieng", nil},
		{"chế độ lạ", "3003", "demo", nil},
		{"app_id có khoảng trắng", " 3004", "chinh", nil},
	} {
		_, err := db.Exec(`INSERT INTO mini_app (app_id, mode, tenant_id, created_by, updated_by)
			VALUES ($1, $2, $3, $4, $4)`, c.appID, c.mode, c.tenantID, testActor)
		if err == nil {
			t.Errorf("%s: CSDL nhận dòng lẽ ra phải từ chối", c.name)
		}
	}
}

func TestPgMiniAppHardDeleteRefused(t *testing.T) {
	db, _ := openTestDB(t)
	runMigrations(t, db)
	insertMiniApp(t, db, "4001", "chinh", nil, true)
	// Exercising the refusal is the point of the test; the trigger must reject it.
	if _, err := db.Exec(`DELETE FROM mini_app WHERE app_id = '4001'`); err == nil {
		t.Fatal("xoá cứng mini_app không bị từ chối")
	}
}

func insertCommuneProfile(t *testing.T, db *sql.DB, tenantID, address string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO commune_profile
		(tenant_id, office_address, hotline, office_hours_text, introduction, created_by, updated_by)
		VALUES ($1,$2,'0900000000','Thứ 2 – Thứ 6','Giới thiệu',$3,$3)`, tenantID, address, testActor)
	if err != nil {
		t.Fatalf("thêm hồ sơ %s: %v", tenantID, err)
	}
}

// The read is scoped by the commune in ctx: commune A reads A's, commune B — with no profile —
// reads nothing, never A's.
func TestPgCommuneProfileScopedByContextTenant(t *testing.T) {
	db, _ := openTestDB(t)
	runMigrations(t, db)
	insertTenant(t, db, ulidA, "Xã Thăng Bình", true)
	insertTenant(t, db, ulidB, "Xã Bên Cạnh", true)
	insertCommuneProfile(t, db, ulidA, "Trụ sở A")

	s := NewCommuneProfileStore(corestore.New(db))

	profile, err := s.Read(tenant.Into(context.Background(), tenant.ID(ulidA)))
	if err != nil {
		t.Fatalf("đọc hồ sơ xã A: %v", err)
	}
	if profile.OfficeAddress != "Trụ sở A" || profile.Hotline != "0900000000" ||
		profile.OfficeHoursText != "Thứ 2 – Thứ 6" || profile.Introduction != "Giới thiệu" || profile.LogoURL != "" {
		t.Errorf("hồ sơ = %+v", profile)
	}

	if _, err := s.Read(tenant.Into(context.Background(), tenant.ID(ulidB))); !errors.Is(err, ErrCommuneProfileNotDeclared) {
		t.Fatalf("xã B đọc được hồ sơ: err = %v, muốn ErrCommuneProfileNotDeclared", err)
	}
}

func TestPgCommuneProfileSoftDeletedNotRead(t *testing.T) {
	db, _ := openTestDB(t)
	runMigrations(t, db)
	insertTenant(t, db, ulidA, "Xã Thăng Bình", true)
	insertCommuneProfile(t, db, ulidA, "Trụ sở A")
	if _, err := db.Exec(`UPDATE commune_profile SET deleted_at = now(), deleted_by = $1,
		delete_reason = 'thử' WHERE tenant_id = $2`, testActor, ulidA); err != nil {
		t.Fatalf("xoá mềm: %v", err)
	}
	_, err := NewCommuneProfileStore(corestore.New(db)).Read(tenant.Into(context.Background(), tenant.ID(ulidA)))
	if !errors.Is(err, ErrCommuneProfileNotDeclared) {
		t.Fatalf("err = %v, muốn ErrCommuneProfileNotDeclared", err)
	}
}

func TestPgCommuneProfileOneRowPerTenantAndEmptyLogoRefused(t *testing.T) {
	db, _ := openTestDB(t)
	runMigrations(t, db)
	insertTenant(t, db, ulidA, "Xã Thăng Bình", true)
	insertCommuneProfile(t, db, ulidA, "Trụ sở A")

	if _, err := db.Exec(`INSERT INTO commune_profile (tenant_id, created_by, updated_by)
		VALUES ($1,$2,$2)`, ulidA, testActor); err == nil {
		t.Error("xã có hai hồ sơ hiển thị")
	}
	if _, err := db.Exec(`UPDATE commune_profile SET logo_url = '' WHERE tenant_id = $1`, ulidA); err == nil {
		t.Error("logo_url = '' được nhận — hai cách viết cho 'không có logo'")
	}
	if _, err := db.Exec(`DELETE FROM commune_profile WHERE tenant_id = $1`, ulidA); err == nil {
		t.Error("xoá cứng hồ sơ hiển thị không bị từ chối")
	}
}
