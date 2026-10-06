package store

import (
	"context"
	"errors"
	"testing"
)

// LiveOwnMiniAppID against a real PostgreSQL — skipped without VIGOV_TEST_DSN, like every
// *_pg_test.go here.

func TestPgLiveOwnMiniAppIDAnyDomainOfActiveCommune(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Thăng Bình", true)
	themHost(t, db, "thangbinh.vigov.vn", ulidA, true)
	themHost(t, db, "thangbinh-cu.vigov.vn", ulidA, false)
	themMiniApp(t, db, "3001", "rieng", ulidA, true)
	themMiniApp(t, db, "3002", "rieng", ulidA, false) // switched off: not live
	themMiniApp(t, db, "3003", "chinh", nil, true)    // the shared app is never a commune's own

	d := NewDirectory(db, "")
	for _, host := range []string{"thangbinh.vigov.vn", "THANGBINH-CU.vigov.vn"} {
		got, err := d.LiveOwnMiniAppID(context.Background(), host)
		if err != nil || got != "3001" {
			t.Errorf("%s: got %q, %v — want 3001", host, got, err)
		}
	}
}

func TestPgLiveOwnMiniAppIDOneSentinelForEveryAbsence(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Không App", true)
	themHost(t, db, "khongapp.vigov.vn", ulidA, true)
	themXa(t, db, ulidB, "Xã Đã Sáp Nhập", false)
	themHost(t, db, "dasapnhap.vigov.vn", ulidB, true)
	themMiniApp(t, db, "3101", "rieng", ulidB, true) // live app of an INACTIVE commune

	d := NewDirectory(db, "")
	for _, host := range []string{"khongapp.vigov.vn", "dasapnhap.vigov.vn", "khongco.vigov.vn", "admin.vigov.vn"} {
		if _, err := d.LiveOwnMiniAppID(context.Background(), host); !errors.Is(err, ErrKhongCoMiniApp) {
			t.Errorf("%s: err = %v, want ErrKhongCoMiniApp", host, err)
		}
	}
}

func TestPgLiveOwnMiniAppIDTwoLiveRefused(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Hai App", true)
	themHost(t, db, "haiapp.vigov.vn", ulidA, true)
	themMiniApp(t, db, "3201", "rieng", ulidA, true)
	themMiniApp(t, db, "3202", "rieng", ulidA, true)

	_, err := NewDirectory(db, "").LiveOwnMiniAppID(context.Background(), "haiapp.vigov.vn")
	if !errors.Is(err, ErrOwnMiniAppAmbiguous) {
		t.Fatalf("err = %v, want ErrOwnMiniAppAmbiguous", err)
	}
}
