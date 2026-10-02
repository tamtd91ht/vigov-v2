package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	corestore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-platform/internal/domain"
)

// Integration tests for the branding half of ho_so_hien_thi_xa and stored_file (migration 0017) against a
// real PostgreSQL — SKIPPED without VIGOV_TEST_DSN, like every *_pg_test.go here. The trigger that only
// accepts a ready, published file of the right purpose and the read's exclusions are database behaviour.

const brandingFileID = "01JF0000000000000000000001"

// insertPublishedLogo inserts a `ready`, published tenant-logo file of commune tid.
func insertPublishedLogo(t *testing.T, db *sql.DB, tid, id string) string {
	t.Helper()
	low := strings.ToLower(tid)
	obj := "content-source/t_" + low + "/2026/10/platform/tenant-logo/" + strings.ToLower(id) + "/original.png"
	pub := "public-media/t_" + low + "/2026/10/platform/tenant-logo/" + strings.ToLower(id) + "/thumb-512.png"
	_, err := db.Exec(`INSERT INTO stored_file (tenant_id, id, bucket, object_key, retention_class, purpose,
		subject_type, subject_id, original_name, mime_type, size_bytes, sha256, status, public_object_key,
		uploaded_by, created_at, updated_at)
		VALUES ($1,$2,'private',$3,'content-source','tenant-logo','tenant-display-profile',$1,'logo.png',
		'image/png',100,$4,'ready',$5,$6,now(),now())`,
		tid, id, obj, strings.Repeat("a", 64), pub, nguoiGhiThu)
	if err != nil {
		t.Fatalf("insert logo file: %v", err)
	}
	return pub
}

func inTx(t *testing.T, db *sql.DB, tid string, fn func(ctx context.Context, tx *corestore.ScopedTx) error) error {
	t.Helper()
	ctx := tenant.Into(context.Background(), tenant.ID(tid))
	return corestore.New(db).For(ctx).Tx(ctx, func(tx *corestore.ScopedTx) error { return fn(ctx, tx) })
}

func TestPgBrandingProfileCreatedForACommuneWithNoneAndReadBack(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Thăng Bình", true)
	pub := insertPublishedLogo(t, db, ulidA, brandingFileID)
	s := NewHoSoHienThiStore(corestore.New(db))

	err := inTx(t, db, ulidA, func(ctx context.Context, tx *corestore.ScopedTx) error {
		created, err := s.EnsureProfile(ctx, tx, nguoiGhiThu, time.Now())
		if err != nil || !created {
			t.Fatalf("EnsureProfile = %v, %v", created, err)
		}
		return s.SetBrandingFile(ctx, tx, domain.BrandingLogo, brandingFileID, nguoiGhiThu, time.Now())
	})
	if err != nil {
		t.Fatal(err)
	}
	hs, err := s.Doc(tenant.Into(context.Background(), tenant.ID(ulidA)))
	if err != nil {
		t.Fatalf("a profile holding only branding must read: %v", err)
	}
	if hs.LogoPublicKey != pub || hs.WebAdminBannerPublicKey != "" || hs.DiaChiTruSo != "" || hs.LogoURL != "" {
		t.Errorf("profile = %+v", hs)
	}
}

func TestPgBrandingReadIsEmptyForUnpublishedOrDeletedFile(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Thăng Bình", true)
	insertPublishedLogo(t, db, ulidA, brandingFileID)
	themHoSo(t, db, ulidA, "Trụ sở A")
	s := NewHoSoHienThiStore(corestore.New(db))
	if err := inTx(t, db, ulidA, func(ctx context.Context, tx *corestore.ScopedTx) error {
		return s.SetBrandingFile(ctx, tx, domain.BrandingLogo, brandingFileID, nguoiGhiThu, time.Now())
	}); err != nil {
		t.Fatal(err)
	}
	ctx := tenant.Into(context.Background(), tenant.ID(ulidA))

	// Unpublished (key cleared after the pointer was set): "".
	if _, err := db.Exec(`UPDATE stored_file SET public_object_key = NULL WHERE tenant_id = $1 AND id = $2`,
		ulidA, brandingFileID); err != nil {
		t.Fatal(err)
	}
	if hs, err := s.Doc(ctx); err != nil || hs.LogoPublicKey != "" {
		t.Fatalf("unpublished: key = %q err = %v, want empty", hs.LogoPublicKey, err)
	}
	// Soft-deleted: "".
	if _, err := db.Exec(`UPDATE stored_file SET deleted_at = now(), deleted_by = $3, delete_reason = 'thu'
		WHERE tenant_id = $1 AND id = $2`, ulidA, brandingFileID, nguoiGhiThu); err != nil {
		t.Fatal(err)
	}
	if hs, err := s.Doc(ctx); err != nil || hs.LogoPublicKey != "" {
		t.Fatalf("deleted: key = %q err = %v, want empty", hs.LogoPublicKey, err)
	}
}

func TestPgBrandingTriggerRefusesAnUnpublishedFile(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Thăng Bình", true)
	insertPublishedLogo(t, db, ulidA, brandingFileID)
	if _, err := db.Exec(`UPDATE stored_file SET public_object_key = NULL WHERE tenant_id = $1`, ulidA); err != nil {
		t.Fatal(err)
	}
	themHoSo(t, db, ulidA, "Trụ sở A")
	s := NewHoSoHienThiStore(corestore.New(db))
	err := inTx(t, db, ulidA, func(ctx context.Context, tx *corestore.ScopedTx) error {
		return s.SetBrandingFile(ctx, tx, domain.BrandingLogo, brandingFileID, nguoiGhiThu, time.Now())
	})
	if err == nil {
		t.Fatal("pointed the profile at an unpublished file")
	}
}

func TestPgBrandingSoftDeletedProfileIsSeenAndNeverRevived(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Thăng Bình", true)
	themHoSo(t, db, ulidA, "Trụ sở A")
	if _, err := db.Exec(`UPDATE ho_so_hien_thi_xa SET deleted_at = now(), deleted_by = $1,
		delete_reason = 'thử' WHERE tenant_id = $2`, nguoiGhiThu, ulidA); err != nil {
		t.Fatal(err)
	}
	s := NewHoSoHienThiStore(corestore.New(db))
	err := inTx(t, db, ulidA, func(ctx context.Context, tx *corestore.ScopedTx) error {
		p, found, err := s.ProfileForUpdate(ctx, tx)
		if err != nil || !found || !p.Deleted {
			t.Fatalf("ProfileForUpdate = %+v found=%v err=%v, want the retired row", p, found, err)
		}
		created, err := s.EnsureProfile(ctx, tx, nguoiGhiThu, time.Now())
		if err != nil || created {
			t.Fatalf("EnsureProfile on a retired row = %v, %v — must write nothing", created, err)
		}
		return s.SetBrandingFile(ctx, tx, domain.BrandingLogo, "", nguoiGhiThu, time.Now())
	})
	if !errors.Is(err, domain.ErrProfileDeleted) {
		t.Fatalf("err = %v, want ErrProfileDeleted", err)
	}
}
