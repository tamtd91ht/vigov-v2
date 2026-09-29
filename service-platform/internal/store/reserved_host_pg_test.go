package store

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/vihat/vigov/core/migrate"
	"github.com/vihat/vigov/service-platform/migrations"
)

// Migration 0007 against a real PostgreSQL: what the CHECK refuses, and that the rows it was
// written NOT to touch are still there afterwards. Skipped without VIGOV_TEST_DSN (openTestDB).

const migration0007 = "0007_tenant_domain_khong_danh_rieng.sql"

// before0007 is the service's real migrations minus 0007 and anything after it — the schema as a
// deployed environment had it on the day the admin rows were written.
func before0007(t *testing.T) fs.FS {
	t.Helper()
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		t.Fatalf("đọc migrations: %v", err)
	}
	out := fstest.MapFS{}
	for _, m := range entries {
		if !strings.HasSuffix(m.Name(), ".sql") || m.Name() >= migration0007 {
			continue
		}
		b, err := fs.ReadFile(migrations.FS, m.Name())
		if err != nil {
			t.Fatalf("đọc %s: %v", m.Name(), err)
		}
		out[m.Name()] = &fstest.MapFile{Data: b}
	}
	return out
}

// The CHECK 0007 added, under the name 0012 gave it (ADR 0061 layer B). Its body is still 0007's.
func isReservedHostViolation(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23514" && pg.ConstraintName == "tenant_domain_not_reserved"
}

func TestPg0007KeepsOldRowsAndRefusesNew(t *testing.T) {
	db, _ := openTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 1. The state before 0007: the pipeline's two admin rows, mapped to an active commune. Written
	// with the column names that schema HAD — insertTenant/insertHost speak the post-0012 names.
	if _, err := migrate.Chay(ctx, db, before0007(t), "platform"); err != nil {
		t.Fatalf("chạy migration tới trước 0007: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO tenant (id, ten, tinh_thanh, dang_hoat_dong) VALUES ($1, 'Xã Thăng Bình', 'Thành phố Đà Nẵng', true)`,
		ulidA); err != nil {
		t.Fatalf("thêm xã trước 0007: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO tenant_domain (host, tenant_id, la_chinh) VALUES ('admin.vigov.vn', $1, true), ('admin-stg.vigov.vn', $1, false)`,
		ulidA); err != nil {
		t.Fatalf("thêm host admin trước 0007: %v", err)
	}

	// 2. Apply 0007 — and everything after it, 0012's rename and table rewrite included — through
	// the real runner. NOT VALID is what lets this succeed with the two violating rows present; a
	// plain CHECK would fail here, and that failure is the one this test pins the absence of.
	if _, err := migrate.Chay(ctx, db, migrations.FS, "platform"); err != nil {
		t.Fatalf("0007 không áp được lên bảng đang có dòng admin: %v", err)
	}

	// 3. The rows survive, unchanged (rule 7).
	var n int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM tenant_domain WHERE host IN ('admin.vigov.vn','admin-stg.vigov.vn') AND tenant_id = $1`,
		ulidA).Scan(&n); err != nil {
		t.Fatalf("đếm dòng admin: %v", err)
	}
	if n != 2 {
		t.Fatalf("còn %d dòng admin, muốn 2 — migration không được đụng dữ liệu cũ", n)
	}

	// 4. ...and they no longer resolve, on either path.
	d := NewDirectory(db)
	for _, h := range []string{"admin.vigov.vn", "admin-stg.vigov.vn", "ADMIN.vigov.vn:443"} {
		if _, ok := d.ByHost(ctx, h); ok {
			t.Errorf("ByHost(%q) vẫn phân giải ra xã", h)
		}
		if _, err := d.ByHostErr(ctx, h); !errors.Is(err, ErrTenantNotFound) {
			t.Errorf("ByHostErr(%q) = %v, muốn ErrTenantNotFound", h, err)
		}
	}

	// 5. A new reserved row is refused by the CHECK itself — including a spelling the Go side
	// normalises, since an operator's SQL does not pass through NormaliseHost.
	for _, h := range []string{
		"www.vigov.vn", "vigov.vn", "stg.vigov.vn", "api.vigov.vn", "admin.stg.vigov.vn",
		"identity.api.vigov.vn", "petitions.api-stg.vigov.vn", "admin.vigov.vn.",
	} {
		_, err := db.ExecContext(ctx,
			`INSERT INTO tenant_domain (host, tenant_id, is_primary) VALUES ($1,$2,false)`, h, ulidA)
		if !isReservedHostViolation(err) {
			t.Errorf("chèn %q: lỗi = %v, muốn vi phạm tenant_domain_not_reserved", h, err)
		}
	}

	// 6. An UPDATE of a kept row is refused too — the header of 0007 says so, and a header claim
	// nothing checks is a claim that drifts.
	_, err := db.ExecContext(ctx, `UPDATE tenant_domain SET is_primary = false WHERE host = 'admin.vigov.vn'`)
	if !isReservedHostViolation(err) {
		t.Errorf("sửa dòng admin cũ: lỗi = %v, muốn vi phạm tenant_domain_not_reserved", err)
	}

	// 7. Commune hosts, including ones merely containing a reserved word, are still accepted.
	for _, h := range []string{"thangbinh-danang.vigov.vn", "thangbinh-danang.stg.vigov.vn", "apixa.vigov.vn"} {
		insertHost(t, db, h, ulidA, false)
	}
	if _, ok := d.ByHost(ctx, "thangbinh-danang.vigov.vn"); !ok {
		t.Error("host của xã không phân giải được sau 0007")
	}
}
