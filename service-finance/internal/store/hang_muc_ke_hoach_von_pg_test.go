package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/vihat/vigov/core/migrate"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/migrations"
)

// Integration tests for the capital plan category catalogue, against a real PostgreSQL.
//
// WHY A REAL DATABASE: every property asserted here lives entirely in the SQL — the `tenant_id`
// predicate Scoped.Query binds, the `deleted_at IS NULL` clause, the ORDER BY, and the LIMIT that
// makes "too many" detectable. A fake would only agree with whatever the query already does, which
// is the one thing worth nothing.
//
// SKIPPED UNLESS VIGOV_TEST_DSN IS SET, AND THAT IS NOT A SMALL CAVEAT — read it as a warning about
// this repository, not as a footnote. No PostgreSQL is reachable from the build environment, so on
// every machine anybody has run this on so far, every test in this file SKIPS while the package
// still prints `ok`. A green `go test` here means the code COMPILES; it does not mean one statement
// below was executed. Nothing in this file may be described as verified until somebody sets the
// DSN and says what came out.
//
// The DSN carries a password and lives only in the environment (rule 8).

// The schema is built ONCE for the whole package, not per test: the finance migrations create
// partitioned tables with 32 partitions each, and a suite slow enough to skip is a suite that stops
// being run. Each test isolates itself with its own commune ids instead, which is also closer to
// how the data really looks.
var dbChung *sql.DB

func TestMain(m *testing.M) {
	dsn := os.Getenv("VIGOV_TEST_DSN")
	if dsn == "" {
		os.Exit(m.Run()) // mỗi test tự bỏ qua
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mở kết nối:", err)
		os.Exit(1)
	}

	ctx, huy := context.WithTimeout(context.Background(), 60*time.Second)
	defer huy()

	schema := fmt.Sprintf("vigov_fi_test_%d", time.Now().UnixNano())
	// ONE physical connection for the whole suite. `SET search_path` is SESSION state, so on a pool
	// it applies to whichever connection happened to serve that statement and to no other — the
	// next statement can land on a fresh connection in the public schema.
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

	// THE REAL RUNNER, not a hand-picked file: every migration this service ships is exercised
	// here, and a new one is exercised the day it is added rather than the day somebody remembers
	// to extend a list.
	kq, err := migrate.Chay(ctx, db, migrations.FS, "finance")
	if err != nil {
		fmt.Fprintln(os.Stderr, "chạy migration:", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "migration đã áp:", kq.DaAp)

	dbChung = db
	ma := m.Run()

	// A schema this suite created, in a test database. Rule 7 protects archival business data; this
	// holds none.
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

// xaRieng returns commune ids unique to this test, so tests sharing the schema cannot see each
// other's rows. Two are returned for the tests that prove isolation.
func xaRieng(t *testing.T) (string, string) {
	t.Helper()
	n := time.Now().UnixNano()
	a := fmt.Sprintf("%026d", n)
	b := fmt.Sprintf("%026d", n+1)
	return a[:26], b[:26]
}

// ctxXa LIVES IN hang_muc_ke_hoach_von_test.go, next to the fake driver, and is shared with this
// file. ONE way to put a commune into a context, not two: the second copy is the one that ends up
// subtly different from what the edge really does. The ids here are strings because they are also
// bound into the INSERT statements below, so the conversion is explicit at each call site.

// themHangMuc inserts one category. thu_tu AND dang_dung ARE PASSED EXPLICITLY: the schema defaults
// them, and a test that relied on a default could not tell "the column is read" from "the column is
// always its default".
func themHangMuc(t *testing.T, db *sql.DB, tenantID, id, ma, nhan string, thuTu int, dangDung bool) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO hang_muc_ke_hoach_von (tenant_id, id, ma, nhan, thu_tu, dang_dung)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		tenantID, id, ma, nhan, thuTu, dangDung)
	if err != nil {
		t.Fatalf("thêm hạng mục %q: %v", ma, err)
	}
}

func dungHangMucStore(db *sql.DB) *HangMucKeHoachVonStore {
	return NewHangMucKeHoachVonStore(pkgstore.New(db))
}

// --- the column list against the real schema ------------------------------------------------------

func TestPgCotTrongMaKhopVoiLuocDoThat(t *testing.T) {
	// THE REASON THIS FILE EXISTS. cotHangMuc is a STRING, and the fake driver in
	// hang_muc_ke_hoach_von_test.go builds its rows from that same string — so a column misspelled
	// there from the start, or renamed by a later migration, is invisible to every test that does not
	// touch a real schema. It would surface as a 500 on the first real request, with "column ... does
	// not exist" in a log nobody is reading yet.
	//
	// WHY THIS EARNS A TEST OF ITS OWN rather than being left to the five behavioural cases below:
	// those all fail together, and for many possible reasons. A typo'd column reads there as "the
	// query returned nothing", which sends the next person looking at the DATA instead of at the
	// string. This one names the missing column in one line.
	//
	// `tenant_id` and `deleted_at` are checked alongside the SELECT list although neither appears in
	// cotHangMuc: Scoped.Query builds `WHERE tenant_id = $1` around the first and DanhSach appends
	// `AND deleted_at IS NULL` around the second, so the statement cannot run without either.
	db := moKetNoi(t)

	co := map[string]bool{}
	rows, err := db.Query(
		`SELECT column_name FROM information_schema.columns
		  WHERE table_schema = current_schema() AND table_name = 'hang_muc_ke_hoach_von'`)
	if err != nil {
		t.Fatalf("đọc information_schema: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			t.Fatalf("đọc tên cột: %v", err)
		}
		co[c] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("duyệt tên cột: %v", err)
	}
	// THE TABLE ITSELF FIRST. Without this, a wrong table name produces an empty set and every
	// column below reads as missing — a list of six names where the real fault is one.
	if len(co) == 0 {
		t.Fatal("bảng hang_muc_ke_hoach_von không tồn tại trong schema — tên bảng trong kho sai, hoặc migration 0003 chưa chạy")
	}

	var thieu []string
	for _, c := range strings.Split(cotHangMuc, ",") {
		if c = strings.TrimSpace(c); c != "" && !co[c] {
			thieu = append(thieu, c)
		}
	}
	// Not part of the SELECT list, but the statement is built around them.
	for _, c := range []string{"tenant_id", "deleted_at"} {
		if !co[c] {
			thieu = append(thieu, c)
		}
	}
	if len(thieu) > 0 {
		sort.Strings(thieu)
		t.Fatalf("cột không có trong bảng thật: %v — câu lệnh sẽ hỏng ở lần gọi thật đầu tiên", thieu)
	}
}

func TestPgDanhSachChiDocXaCuaChinhMinh(t *testing.T) {
	// THE MOST IMPORTANT CASE IN THIS FILE. The predicate that keeps two public authorities apart
	// is a single `WHERE tenant_id = $1` bound by Scoped.Query, and no test with one commune in it
	// can show whether it is there.
	db := moKetNoi(t)
	xa, xaKhac := xaRieng(t)

	themHangMuc(t, db, xa, "hm-a", "xay-dung-moi", "Xây dựng mới", 1, true)
	themHangMuc(t, db, xaKhac, "hm-b", "tra-no", "Trả nợ", 1, true)

	ra, err := dungHangMucStore(db).DanhSach(ctxXa(tenant.ID(xa)))
	if err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	if len(ra) != 1 || ra[0].Ma != "xay-dung-moi" {
		t.Fatalf("RÒ RỈ hoặc thiếu dòng: %+v", ra)
	}
}

func TestPgDanhSachBoQuaDongDaXoaMem(t *testing.T) {
	// Rule 7, invariant 2: a soft-deleted row leaves EVERY read path. The UPDATE below is the only
	// way a row goes: the trigger refuses a hard DELETE outright.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themHangMuc(t, db, xa, "hm-con", "xay-dung-moi", "Xây dựng mới", 1, true)
	themHangMuc(t, db, xa, "hm-xoa", "tra-no", "Trả nợ", 2, true)
	if _, err := db.Exec(
		`UPDATE hang_muc_ke_hoach_von SET deleted_at = now(), deleted_by = $1, delete_reason = $2
		 WHERE tenant_id = $3 AND id = $4`,
		"CB-TEST", "test", xa, "hm-xoa"); err != nil {
		t.Fatalf("xoá mềm: %v", err)
	}

	ra, err := dungHangMucStore(db).DanhSach(ctxXa(tenant.ID(xa)))
	if err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	if len(ra) != 1 || ra[0].ID != "hm-con" {
		t.Fatalf("dòng đã xoá mềm vẫn lọt vào danh sách: %+v", ra)
	}
}

func TestPgDanhSachGiuDongDaTat(t *testing.T) {
	// `dang_dung = false` AND `deleted_at IS NOT NULL` ARE DIFFERENT QUESTIONS, and this is where
	// the difference is checked: a row taken out of use stays in the list carrying its flag,
	// because a plan line of an earlier budget year still holds its code as a value.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themHangMuc(t, db, xa, "hm-tat", "tra-no", "Trả nợ", 1, false)

	ra, err := dungHangMucStore(db).DanhSach(ctxXa(tenant.ID(xa)))
	if err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	if len(ra) != 1 {
		t.Fatalf("dòng đã tắt bị loại khỏi danh sách: %+v", ra)
	}
	if ra[0].DangDung {
		t.Error("dang_dung = false mà đọc ra true — cờ bị hoán đổi với cột bên cạnh")
	}
}

func TestPgDanhSachTheoThuTuRoiTheoMa(t *testing.T) {
	// `thu_tu` is the commune's own arrangement; `ma` breaks ties so two rows sharing a rank cannot
	// swap places between two calls. The two rows at rank 1 are inserted in the WRONG order on
	// purpose, so an ORDER BY that dropped the tie-break would return them as inserted.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themHangMuc(t, db, xa, "hm-3", "xay-dung-moi", "Xây dựng mới", 2, true)
	themHangMuc(t, db, xa, "hm-2", "cai-tao-nang-cap", "Cải tạo, nâng cấp", 1, true)
	themHangMuc(t, db, xa, "hm-1", "bao-tri", "Bảo trì", 1, true)

	ra, err := dungHangMucStore(db).DanhSach(ctxXa(tenant.ID(xa)))
	if err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	muon := []string{"bao-tri", "cai-tao-nang-cap", "xay-dung-moi"}
	if len(ra) != len(muon) {
		t.Fatalf("nhận %d dòng, muốn %d", len(ra), len(muon))
	}
	for i := range muon {
		if ra[i].Ma != muon[i] {
			t.Fatalf("thứ tự sai ở vị trí %d: %q, muốn %q", i, ra[i].Ma, muon[i])
		}
	}
}

func TestPgVuotTranThiTuChoiChuKhongCatBot(t *testing.T) {
	// ONE ROW OVER THE CEILING, which is the only interesting number: at exactly the ceiling the
	// list is complete and must come back whole. The refusal must also carry NO rows — a caller
	// handed a list alongside an error is a caller that renders it.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	for i := 0; i <= TranDanhMucHangMuc; i++ {
		themHangMuc(t, db, xa, fmt.Sprintf("hm-%04d", i), fmt.Sprintf("hang-muc-%04d", i),
			fmt.Sprintf("Hạng mục %d", i), i, true)
	}

	ra, err := dungHangMucStore(db).DanhSach(ctxXa(tenant.ID(xa)))
	if !errors.Is(err, ErrQuaNhieuHangMuc) {
		t.Fatalf("err = %v, muốn ErrQuaNhieuHangMuc", err)
	}
	if ra != nil {
		t.Errorf("từ chối mà vẫn trả %d dòng — một danh sách đi kèm lỗi là một danh sách sẽ được vẽ", len(ra))
	}
}
