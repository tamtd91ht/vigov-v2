package crosstenant

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/vihat/vigov/core/migrate"
	"github.com/vihat/vigov/service-identity/migrations"
)

// Integration tests against a real PostgreSQL.
//
// WHY A REAL DATABASE: what is being verified is the UNIQUE constraint on so_dien_thoai, the
// 26-character CHECK on the id, and the fact that this write lives or dies with the caller's
// transaction. All three live in the SQL, and a mock would simply agree with whatever the Go
// code believes.
//
// THEY SKIP WITHOUT VIGOV_TEST_DSN, AND THE PACKAGE STILL PRINTS `ok`. That is the repository's
// worst known trap: a green gate here does NOT mean these assertions ran. Read the skip lines.
// The DSN carries a password and lives only in the environment (rule 8).

var dbChung *sql.DB

func TestMain(m *testing.M) {
	dsn := os.Getenv("VIGOV_TEST_DSN")
	if dsn == "" {
		os.Exit(m.Run()) // each test skips for itself; the pure tests still run
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mở kết nối:", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	schema := fmt.Sprintf("vigov_ct_test_%d", time.Now().UnixNano())
	// ONE physical connection for the whole suite. `SET search_path` is SESSION state, so on a
	// pool it applies only to the connection that happened to serve that statement — and
	// core/migrate pins a connection of its own for its advisory lock, which would be a second.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		fmt.Fprintln(os.Stderr, "tạo schema:", err)
		os.Exit(1)
	}
	if _, err := db.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
		fmt.Fprintln(os.Stderr, "đặt search_path:", err)
		os.Exit(1)
	}
	if _, err := migrate.Chay(ctx, db, migrations.FS, "identity"); err != nil {
		fmt.Fprintln(os.Stderr, "chạy migration:", err)
		os.Exit(1)
	}

	dbChung = db
	ma := m.Run()

	// A schema this suite created, in a test database. Rule 7 protects archival business data;
	// this holds none.
	if _, err := db.ExecContext(context.Background(),
		"DROP SCHEMA IF EXISTS "+schema+" CASCADE"); err != nil {
		fmt.Fprintln(os.Stderr, "dọn schema:", err)
	}
	db.Close()
	os.Exit(ma)
}

func moKetNoi(t *testing.T) *sql.DB {
	t.Helper()
	if dbChung == nil {
		t.Skip("VIGOV_TEST_DSN chưa đặt — bỏ qua test tích hợp")
	}
	return dbChung
}

// soGiaRieng returns a fake number unique to this test.
//
// WHY NOT THE AGREED CONSTANT EVERYWHERE: dinh_danh_cong_dan is unique on the number
// PLATFORM-WIDE and these tests share one schema, so two tests using 0900000000 would be two
// tests fighting over one row. The prefix keeps every value inside the same obviously-fake
// range as the agreed number (rule 3, invariant 5); no real number appears in this repository.
func soGiaRieng(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("09%08d", time.Now().UnixNano()%100000000)
}

// trongGiaoDich runs fn in a transaction and commits, the way a use case would.
func trongGiaoDich(t *testing.T, db *sql.DB, fn func(tx *sql.Tx) error) {
	t.Helper()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("mở giao dịch: %v", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		t.Fatalf("trong giao dịch: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

func TestTaoLanDauRoiDungLaiDinhDanhCu(t *testing.T) {
	// One phone number is ONE record for the whole platform (ADR 0002). Two rows for one number
	// means one human being seen as two people, and the citizen loses sight of files they filed
	// themselves.
	db := moKetNoi(t)
	so := soGiaRieng(t)
	s := NewDinhDanhStore()

	var dau, sau DinhDanh
	var vuaTaoDau, vuaTaoSau bool
	trongGiaoDich(t, db, func(tx *sql.Tx) error {
		var err error
		dau, vuaTaoDau, err = s.TimHoacTao(context.Background(), tx, so)
		return err
	})
	trongGiaoDich(t, db, func(tx *sql.Tx) error {
		var err error
		sau, vuaTaoSau, err = s.TimHoacTao(context.Background(), tx, so)
		return err
	})

	if !vuaTaoDau {
		t.Error("lần đầu phải báo vừa tạo — bản ghi vết sẽ ghi sai việc đã xảy ra")
	}
	if vuaTaoSau {
		t.Error("lần sau báo vừa tạo, trong khi định danh đã có")
	}
	if dau.ID == "" || dau.ID != sau.ID {
		t.Errorf("hai lần đăng nhập cùng một số cho hai định danh: %q và %q", dau.ID, sau.ID)
	}
	if len(dau.ID) != 26 {
		t.Errorf("định danh dài %d ký tự — CHECK của lược đồ đòi 26", len(dau.ID))
	}
}

func TestHaiSoKhacNhauLaHaiDinhDanh(t *testing.T) {
	db := moKetNoi(t)
	s := NewDinhDanhStore()
	soA, soB := soGiaRieng(t), soGiaRieng(t)+"1"

	var a, b DinhDanh
	trongGiaoDich(t, db, func(tx *sql.Tx) error {
		var err error
		if a, _, err = s.TimHoacTao(context.Background(), tx, soA); err != nil {
			return err
		}
		b, _, err = s.TimHoacTao(context.Background(), tx, soB)
		return err
	})
	if a.ID == b.ID {
		t.Error("hai số điện thoại khác nhau dùng chung một định danh")
	}
}

func TestDinhDanhKhongSuyNguocRaSoDienThoai(t *testing.T) {
	// An identifier that can be reversed into personal data IS personal data, and this one
	// travels into audit entries and into the business records of every commune the citizen
	// deals with (ADR 0020, invariant 5).
	db := moKetNoi(t)
	so := soGiaRieng(t)
	s := NewDinhDanhStore()

	var dd DinhDanh
	trongGiaoDich(t, db, func(tx *sql.Tx) error {
		var err error
		dd, _, err = s.TimHoacTao(context.Background(), tx, so)
		return err
	})
	if strings.Contains(dd.ID, so) || strings.Contains(dd.ID, so[2:]) {
		t.Errorf("định danh mang theo số điện thoại bên trong nó")
	}
}

func TestGiaoDichHuyThiKhongConDinhDanh(t *testing.T) {
	// THE PROPERTY RULE 6 INVARIANT 3 RESTS ON. The identity row and the audit entry for the
	// sign-in must succeed or fail together. If this write committed on its own, a rolled-back
	// sign-in would still leave a row holding a citizen's phone number with no trail beside it.
	db := moKetNoi(t)
	so := soGiaRieng(t)
	s := NewDinhDanhStore()

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.TimHoacTao(context.Background(), tx, so); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}

	var n int
	if err := db.QueryRow(
		`SELECT count(*) FROM dinh_danh_cong_dan WHERE so_dien_thoai = $1`, so).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Error("định danh vẫn còn sau khi giao dịch của lời gọi bị huỷ")
	}
}
