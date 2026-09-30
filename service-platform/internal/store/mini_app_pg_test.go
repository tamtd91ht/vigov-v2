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

// nguoiGhiThu is a staff business code shape (rule 6, invariant 8) — never an internal id.
const nguoiGhiThu = "CB-00001"

func themMiniApp(t *testing.T, db *sql.DB, appID, cheDo string, tenantID any, hoatDong bool) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO mini_app (app_id, che_do, tenant_id, dang_hoat_dong, tao_boi, cap_nhat_boi)
		 VALUES ($1,$2,$3,$4,$5,$5)`, appID, cheDo, tenantID, hoatDong, nguoiGhiThu)
	if err != nil {
		t.Fatalf("thêm mini app %s: %v", appID, err)
	}
}

func TestPgMiniAppKhongDangKy(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	_, err := NewDirectory(db).MiniApp(context.Background(), "9999999999")
	if !errors.Is(err, ErrKhongCoMiniApp) {
		t.Fatalf("err = %v, muốn ErrKhongCoMiniApp", err)
	}
}

func TestPgMiniAppChinhVaRieng(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Thăng Bình", true)
	themXa(t, db, ulidB, "Xã Cũ", false)
	themMiniApp(t, db, "1001", "chinh", nil, true)
	themMiniApp(t, db, "1002", "rieng", ulidA, true)
	themMiniApp(t, db, "1003", "rieng", ulidB, true)

	d := NewDirectory(db)
	ctx := context.Background()

	chinh, err := d.MiniApp(ctx, "1001")
	if err != nil || chinh.CheDo != domain.CheDoChinh || chinh.Xa != nil {
		t.Fatalf("app chính = %+v, err %v", chinh, err)
	}

	rieng, err := d.MiniApp(ctx, "1002")
	if err != nil || rieng.CheDo != domain.CheDoRieng || rieng.Xa == nil {
		t.Fatalf("app riêng = %+v, err %v", rieng, err)
	}
	if rieng.Xa.ID != ulidA || rieng.Xa.Ten != "Xã Thăng Bình" || !rieng.Xa.DangHoatDong ||
		rieng.Xa.TinhThanh != "Thành phố Đà Nẵng" {
		t.Errorf("xã của app riêng = %+v", rieng.Xa)
	}

	// Inactive bound commune: the app IS returned, with the commune's state — the caller refuses.
	cu, err := d.MiniApp(ctx, "1003")
	if err != nil || cu.Xa == nil || cu.Xa.ID != ulidB || cu.Xa.DangHoatDong {
		t.Fatalf("app riêng của xã ngừng hoạt động = %+v, err %v", cu, err)
	}
}

func TestPgMiniAppTatHoacXoaMemCoiNhuKhongDangKy(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Thăng Bình", true)
	themMiniApp(t, db, "2001", "rieng", ulidA, false)
	themMiniApp(t, db, "2002", "rieng", ulidA, true)
	if _, err := db.Exec(`UPDATE mini_app SET deleted_at = now(), deleted_by = $1, delete_reason = 'thử'
		WHERE app_id = '2002'`, nguoiGhiThu); err != nil {
		t.Fatalf("xoá mềm: %v", err)
	}

	d := NewDirectory(db)
	for _, id := range []string{"2001", "2002"} {
		if _, err := d.MiniApp(context.Background(), id); !errors.Is(err, ErrKhongCoMiniApp) {
			t.Errorf("app %s: err = %v, muốn ErrKhongCoMiniApp", id, err)
		}
	}
}

// The CHECK is what makes "main app with a commune" and "dedicated app without one" impossible —
// the two shapes that would put a default commune on the isolation path, or name nobody.
func TestPgMiniAppRangBuocCheDoVaXa(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Thăng Bình", true)

	// One App ID per case: a row wrongly accepted must not make the NEXT case fail on the primary
	// key and pass for the wrong reason.
	for _, c := range []struct {
		ten, appID, cheDo string
		xa                any
	}{
		{"chính có xã", "3001", "chinh", ulidA},
		{"riêng không xã", "3002", "rieng", nil},
		{"chế độ lạ", "3003", "demo", nil},
		{"app_id có khoảng trắng", " 3004", "chinh", nil},
	} {
		_, err := db.Exec(`INSERT INTO mini_app (app_id, che_do, tenant_id, tao_boi, cap_nhat_boi)
			VALUES ($1, $2, $3, $4, $4)`, c.appID, c.cheDo, c.xa, nguoiGhiThu)
		if err == nil {
			t.Errorf("%s: CSDL nhận dòng lẽ ra phải từ chối", c.ten)
		}
	}
}

func TestPgMiniAppCamXoaCung(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themMiniApp(t, db, "4001", "chinh", nil, true)
	// Exercising the refusal is the point of the test; the trigger must reject it.
	if _, err := db.Exec(`DELETE FROM mini_app WHERE app_id = '4001'`); err == nil {
		t.Fatal("xoá cứng mini_app không bị từ chối")
	}
}

func themHoSo(t *testing.T, db *sql.DB, tenantID, diaChi string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO ho_so_hien_thi_xa
		(tenant_id, dia_chi_tru_so, duong_day_nong, gio_lam_viec_hien_thi, gioi_thieu, tao_boi, cap_nhat_boi)
		VALUES ($1,$2,'0900000000','Thứ 2 – Thứ 6','Giới thiệu',$3,$3)`, tenantID, diaChi, nguoiGhiThu)
	if err != nil {
		t.Fatalf("thêm hồ sơ %s: %v", tenantID, err)
	}
}

// The read is scoped by the commune in ctx: commune A reads A's, commune B — with no profile —
// reads nothing, never A's.
func TestPgHoSoHienThiTheoXaTrongContext(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Thăng Bình", true)
	themXa(t, db, ulidB, "Xã Bên Cạnh", true)
	themHoSo(t, db, ulidA, "Trụ sở A")

	s := NewHoSoHienThiStore(corestore.New(db))

	hs, err := s.Doc(tenant.Into(context.Background(), tenant.ID(ulidA)))
	if err != nil {
		t.Fatalf("đọc hồ sơ xã A: %v", err)
	}
	if hs.DiaChiTruSo != "Trụ sở A" || hs.DuongDayNong != "0900000000" ||
		hs.GioLamViecHienThi != "Thứ 2 – Thứ 6" || hs.GioiThieu != "Giới thiệu" || hs.LogoURL != "" {
		t.Errorf("hồ sơ = %+v", hs)
	}

	if _, err := s.Doc(tenant.Into(context.Background(), tenant.ID(ulidB))); !errors.Is(err, ErrChuaCoHoSoHienThi) {
		t.Fatalf("xã B đọc được hồ sơ: err = %v, muốn ErrChuaCoHoSoHienThi", err)
	}
}

func TestPgHoSoHienThiXoaMemKhongDocDuoc(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Thăng Bình", true)
	themHoSo(t, db, ulidA, "Trụ sở A")
	if _, err := db.Exec(`UPDATE ho_so_hien_thi_xa SET deleted_at = now(), deleted_by = $1,
		delete_reason = 'thử' WHERE tenant_id = $2`, nguoiGhiThu, ulidA); err != nil {
		t.Fatalf("xoá mềm: %v", err)
	}
	_, err := NewHoSoHienThiStore(corestore.New(db)).Doc(tenant.Into(context.Background(), tenant.ID(ulidA)))
	if !errors.Is(err, ErrChuaCoHoSoHienThi) {
		t.Fatalf("err = %v, muốn ErrChuaCoHoSoHienThi", err)
	}
}

func TestPgHoSoHienThiMotDongMoiXaVaLogoRongBiTuChoi(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Thăng Bình", true)
	themHoSo(t, db, ulidA, "Trụ sở A")

	if _, err := db.Exec(`INSERT INTO ho_so_hien_thi_xa (tenant_id, tao_boi, cap_nhat_boi)
		VALUES ($1,$2,$2)`, ulidA, nguoiGhiThu); err == nil {
		t.Error("xã có hai hồ sơ hiển thị")
	}
	if _, err := db.Exec(`UPDATE ho_so_hien_thi_xa SET logo_url = '' WHERE tenant_id = $1`, ulidA); err == nil {
		t.Error("logo_url = '' được nhận — hai cách viết cho 'không có logo'")
	}
	if _, err := db.Exec(`DELETE FROM ho_so_hien_thi_xa WHERE tenant_id = $1`, ulidA); err == nil {
		t.Error("xoá cứng hồ sơ hiển thị không bị từ chối")
	}
}
