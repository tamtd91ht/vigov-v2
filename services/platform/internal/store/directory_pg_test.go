package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/vihat/vigov/pkg/migrate"
	"github.com/vihat/vigov/pkg/tenant"
	"github.com/vihat/vigov/services/platform/migrations"
)

// Integration tests against a real PostgreSQL.
//
// WHY A REAL DATABASE AND NOT A MOCK: everything this file checks is behaviour the database
// owns and a mock would simply agree with — the partial unique index that allows exactly one
// canonical host per commune, the foreign key, the lower-case CHECK constraint, and the join
// that resolves a Host. A mock of those is a mock of our own assumptions.
//
// Skipped unless VIGOV_TEST_DSN is set, so `go test ./...` stays green on a machine with no
// database. The DSN carries a password and therefore lives ONLY in the environment — never in
// a file, a commit, or a default value here (rule 8).
//
// Each run builds its own schema and drops it at the end, so runs do not collide and nothing
// outside the test schema is touched.

func moKetNoi(t *testing.T) (*sql.DB, string) {
	t.Helper()

	dsn := os.Getenv("VIGOV_TEST_DSN")
	if dsn == "" {
		t.Skip("VIGOV_TEST_DSN chưa đặt — bỏ qua test tích hợp")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("mở kết nối: %v", err)
	}

	// ONE physical connection for the whole test. `SET search_path` is SESSION state, so on a pool
	// it applies to whichever connection served that statement and to no other — the next
	// statement can land on a fresh connection in the public schema, where this test would then
	// create its tables. It happened to work while everything ran on one lazily-opened connection;
	// pkg/migrate deliberately pins its own connection (the advisory lock is session-scoped),
	// which would have been a second one.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}

	// A unique schema per run. Parallel runs and leftovers from a crashed run cannot collide.
	schema := fmt.Sprintf("vigov_test_%d", time.Now().UnixNano())
	if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("tạo schema: %v", err)
	}
	if _, err := db.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatalf("đặt search_path: %v", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		// Dropping a schema this test created, in a test database. Rule 7 protects archival
		// business data; this holds none.
		if _, err := db.ExecContext(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE"); err != nil {
			t.Logf("dọn schema %s: %v", schema, err)
		}
		db.Close()
	})

	return db, schema
}

// chayMigration applies the service's real migrations THROUGH THE REAL RUNNER. Testing against a
// hand-written copy of the schema would test the copy, not what ships.
//
// It used to read `0001_init.sql` by name. That is why `0002_audit_log_append_only.sql` had never
// executed anywhere: the only two places in the repository that touched migrations both named one
// file, so a file added afterwards was applied by nothing. Going through pkg/migrate means every
// migration this service ships is exercised here, including the ones added after this line was
// written.
func chayMigration(t *testing.T, db *sql.DB) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if _, err := migrate.Chay(ctx, db, migrations.FS, "platform"); err != nil {
		t.Fatalf("chạy migration: %v", err)
	}
}

const (
	ulidA = "01JD8ZQK9M3NPXR7TVWYB2C4EF"
	ulidB = "01JD8ZQK9M3NPXR7TVWYB2C4EG"
)

func themXa(t *testing.T, db *sql.DB, id, ten string, hoatDong bool) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO tenant (id, ten, tinh_thanh, dang_hoat_dong) VALUES ($1,$2,$3,$4)`,
		id, ten, "Thành phố Đà Nẵng", hoatDong)
	if err != nil {
		t.Fatalf("thêm xã %s: %v", ten, err)
	}
}

func themHost(t *testing.T, db *sql.DB, host, tenantID string, laChinh bool) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO tenant_domain (host, tenant_id, la_chinh) VALUES ($1,$2,$3)`,
		host, tenantID, laChinh)
	if err != nil {
		t.Fatalf("thêm host %s: %v", host, err)
	}
}

func TestPgPhanGiaiHost(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Thăng Bình", true)
	themHost(t, db, "thangbinh.vigov.vn", ulidA, true)

	d := NewDirectory(db)
	got, ok := d.ByHost(context.Background(), "thangbinh.vigov.vn")
	if !ok {
		t.Fatal("không phân giải được host đã đăng ký")
	}
	if got.ID != tenant.ID(ulidA) {
		t.Errorf("ID = %q, muốn %q", got.ID, ulidA)
	}
	if got.Name != "Xã Thăng Bình" {
		t.Errorf("Name = %q", got.Name)
	}
}

func TestPgHostChuanHoa(t *testing.T) {
	// Host arrives from the client. Every spelling must land on the same commune, or a whole
	// commune goes dark for a reason invisible in the logs.
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Thăng Bình", true)
	themHost(t, db, "thangbinh.vigov.vn", ulidA, true)

	d := NewDirectory(db)
	for _, vao := range []string{
		"thangbinh.vigov.vn",
		"ThangBinh.ViGov.VN",
		"thangbinh.vigov.vn:443",
		"  ThangBinh.vigov.vn:8080  ",
	} {
		if _, ok := d.ByHost(context.Background(), vao); !ok {
			t.Errorf("ByHost(%q) không phân giải được", vao)
		}
	}
}

func TestPgHostLaTraVeKhong(t *testing.T) {
	// Fail closed: an unknown Host must never fall back to some commune. The edge turns this
	// into 404 — rule 1, invariant 3.
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Thăng Bình", true)
	themHost(t, db, "thangbinh.vigov.vn", ulidA, true)

	d := NewDirectory(db)
	if _, ok := d.ByHost(context.Background(), "khong-ton-tai.vigov.vn"); ok {
		t.Error("host lạ mà vẫn phân giải ra xã — đây là lỗ hổng cách ly")
	}
}

func TestPgXaNgungHoatDongKhongPhucVu(t *testing.T) {
	// A merged commune keeps its data and its address (rule 7) but stops serving requests.
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidB, "Xã đã sáp nhập", false)
	themHost(t, db, "xacu.vigov.vn", ulidB, true)

	d := NewDirectory(db)
	if _, ok := d.ByHost(context.Background(), "xacu.vigov.vn"); ok {
		t.Error("xã đã ngừng hoạt động mà vẫn phục vụ request")
	}

	// The rows are still there — deactivated, not discarded.
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM tenant WHERE id = $1`, ulidB).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Error("dữ liệu xã đã sáp nhập phải được giữ nguyên (luật 7)")
	}
}

func TestPgMotHostChiThuocMotXa(t *testing.T) {
	// A host resolving to two communes makes isolation undecidable at the edge. The primary
	// key on host is what prevents it, and this checks the database really enforces that.
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã A", true)
	themXa(t, db, ulidB, "Xã B", true)
	themHost(t, db, "chung.vigov.vn", ulidA, true)

	_, err := db.Exec(
		`INSERT INTO tenant_domain (host, tenant_id, la_chinh) VALUES ($1,$2,$3)`,
		"chung.vigov.vn", ulidB, false)
	if err == nil {
		t.Fatal("cùng một host gán được cho hai xã — cách ly ở biên trở thành bất khả quyết")
	}
}

func TestPgMoiXaChiMotHostChinh(t *testing.T) {
	// Without this, link building picks whichever row comes back first and one commune shows
	// up under different addresses in notifications sent to citizens.
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Thăng Bình", true)
	themHost(t, db, "thangbinh.vigov.vn", ulidA, true)

	// A second non-canonical host is fine: after a merger the old address must keep working.
	themHost(t, db, "cu.vigov.vn", ulidA, false)

	_, err := db.Exec(
		`INSERT INTO tenant_domain (host, tenant_id, la_chinh) VALUES ($1,$2,$3)`,
		"moi.vigov.vn", ulidA, true)
	if err == nil {
		t.Fatal("một xã có hai host chính — khâu dựng liên kết sẽ không xác định")
	}
}

func TestPgHostPhaiChuThuong(t *testing.T) {
	// Normalisation lower-cases on the way in; the CHECK constraint stops a row that bypassed
	// it from ever being stored, which would make that host unresolvable forever.
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Thăng Bình", true)

	_, err := db.Exec(
		`INSERT INTO tenant_domain (host, tenant_id, la_chinh) VALUES ($1,$2,$3)`,
		"ThangBinh.ViGov.VN", ulidA, true)
	if err == nil {
		t.Fatal("host chữ hoa lọt vào bảng — host đó sẽ không bao giờ phân giải được")
	}
}

func TestPgTenantIdPhaiLaUlid(t *testing.T) {
	// Rule 1, invariant 2: an identifier carrying meaning forces rewriting foreign keys across
	// archival records at the first merger.
	db, _ := moKetNoi(t)
	chayMigration(t, db)

	_, err := db.Exec(
		`INSERT INTO tenant (id, ten) VALUES ($1,$2)`, "thang-binh", "Xã Thăng Bình")
	if err == nil {
		t.Fatal("mã hành chính dùng làm tenant_id — sáp nhập đầu tiên sẽ phải sửa hồ sơ lưu trữ")
	}
}

func TestPgAuditLogNhanDuocDuLieu(t *testing.T) {
	// The skeleton declared PARTITION BY HASH but created no partitions, so every insert
	// failed with "no partition of relation found". This is the test that would have caught it.
	db, _ := moKetNoi(t)
	chayMigration(t, db)

	_, err := db.Exec(
		`INSERT INTO audit_log (tenant_id, actor_id, actor_kind, action, subject, at)
		 VALUES ($1,$2,$3,$4,$5,now())`,
		ulidA, "CB001", "staff", "tiep_nhan_phan_anh", "PA-2026-0001")
	if err != nil {
		t.Fatalf("ghi nhật ký thất bại — bảng phân mảnh mà thiếu partition con: %v", err)
	}

	var n int
	if err := db.QueryRow(`SELECT count(*) FROM audit_log WHERE tenant_id = $1`, ulidA).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("đọc lại được %d dòng nhật ký, muốn 1", n)
	}
}

func TestPgCacheDungVoiDatabaseThat(t *testing.T) {
	db, _ := moKetNoi(t)
	chayMigration(t, db)
	themXa(t, db, ulidA, "Xã Thăng Bình", true)
	themHost(t, db, "thangbinh.vigov.vn", ulidA, true)

	c := tenant.NewCachedDirectory(NewDirectory(db), time.Minute)
	for range 3 {
		if _, ok := c.ByHost(context.Background(), "thangbinh.vigov.vn"); !ok {
			t.Fatal("cache trả về không phân giải được")
		}
	}
	if _, ok := c.ByHost(context.Background(), "khong-ton-tai.vigov.vn"); ok {
		t.Error("cache trả về xã cho host lạ")
	}
}
