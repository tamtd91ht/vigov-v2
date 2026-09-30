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
	"github.com/vihat/vigov/service-comms/migrations"
)

// Integration tests for the map-asset-type catalogue, against a real PostgreSQL.
//
// THE OTHER HALF OF loai_tai_nguyen_ban_do_test.go, NOT A REPLACEMENT FOR IT. That file runs a fake
// driver which RECORDS the statement and returns rows the test supplied; it proves the Go side —
// the positional Scan, the ceiling refusal, the wrapped error — and it runs everywhere, always. But
// it builds its rows FROM THE COLUMN LIST THE STORE ITSELF HANDS IT, so a column that does not
// exist in the real table passes it cleanly, and so does a table name that does not exist at all.
// That gap is structural and no amount of work on the fake closes it. This file is where the
// statement meets the schema the migrations actually create.
//
// WHAT ONLY A REAL SERVER CAN SHOW, and it is everything below:
//
//  1. the `WHERE tenant_id = $1` predicate Scoped.Query binds really does keep two communes apart —
//     no single-commune test can show whether the clause is there at all;
//  2. `deleted_at IS NULL` removes a soft-deleted row WHILE `dang_dung = false` keeps its row. Those
//     are two different questions and the fake can only agree with whichever the query asks;
//  3. ORDER BY thu_tu, ma against rows PostgreSQL ordered, including the tie-break;
//  4. the ceiling refuses at ceiling+1 real rows rather than truncating;
//  5. every column in cotLoaiTaiNguyen — and the table name — exists as migration 0003 creates it.
//     THIS IS THE ASSERTION THE FAKE DRIVER STRUCTURALLY CANNOT MAKE.
//
// SKIPPED UNLESS VIGOV_TEST_DSN IS SET, AND THAT IS NOT A FOOTNOTE — read it as a warning about
// this repository. No PostgreSQL is reachable from this build environment, so on every machine
// anybody has run this on so far, every test in this file SKIPS while the package still prints
// `ok`. A green `go test` here means this file COMPILES. Nothing in it may be described as verified
// until somebody sets the DSN and says what came out.
//
// The DSN carries a password and lives only in the environment (rule 8).

// The schema is built ONCE for the whole package, not per test: migration 0003 creates a
// partitioned table with 32 partitions, and a suite slow enough to skip is a suite that stops being
// run. Each test isolates itself with its own commune ids instead, which is also closer to how the
// data really looks.
var dbChung *sql.DB

func TestMain(m *testing.M) {
	dsn := os.Getenv("VIGOV_TEST_DSN")
	if dsn == "" {
		os.Exit(m.Run()) // mỗi test tự bỏ qua; các test dùng driver giả vẫn chạy bình thường
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mở kết nối:", err)
		os.Exit(1)
	}

	ctx, huy := context.WithTimeout(context.Background(), 60*time.Second)
	defer huy()

	schema := fmt.Sprintf("vigov_cm_test_%d", time.Now().UnixNano())
	// ONE physical connection for the whole suite. `SET search_path` is SESSION state, so on a pool
	// it applies to whichever connection happened to serve that statement and to no other — the next
	// statement can land on a fresh connection sitting in the public schema, where none of these
	// tables exist. This repository has been bitten by exactly that.
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

	// THE REAL RUNNER, not a hand-picked file: every migration this service ships is exercised here,
	// and a new one is exercised the day it is added rather than the day somebody remembers to
	// extend a list. It is also the only thing in this repository that checks 0003's PostgreSQL 13
	// floor and its partition backstop against a server.
	kq, err := migrate.Chay(ctx, db, migrations.FS, "comms")
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
// other's rows. Two are returned for the test that proves isolation.
func xaRieng(t *testing.T) (string, string) {
	t.Helper()
	n := time.Now().UnixNano()
	a := fmt.Sprintf("%026d", n)
	b := fmt.Sprintf("%026d", n+1)
	return a[:26], b[:26]
}

// ctxXa LIVES IN loai_tai_nguyen_ban_do_test.go, next to the fake driver, and is shared with this
// file. ONE way to put a commune into a context, not two: the second copy is the one that ends up
// subtly different from what the edge really does. The ids here are strings because they are also
// bound into the INSERT statements below, so the conversion is explicit at each call site.

// themLoai inserts one catalogue row. thu_tu, la_mac_dinh AND dang_dung ARE PASSED EXPLICITLY: the
// schema defaults all three, and a test that relied on a default could not tell "the column is
// read" from "the column is always its default".
//
// `nguon` is left at its default 'don-vi'. A 'he-thong' row cannot be soft deleted — the tier guard
// refuses it — so the soft-delete case below would fail for a reason that has nothing to do with
// the read path.
func themLoai(t *testing.T, db *sql.DB, tenantID, id, ma, nhan string, thuTu int, macDinh, dangDung bool) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO loai_tai_nguyen_ban_do (tenant_id, id, ma, nhan, thu_tu, la_mac_dinh, dang_dung)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		tenantID, id, ma, nhan, thuTu, macDinh, dangDung)
	if err != nil {
		t.Fatalf("thêm loại tài nguyên %q: %v", ma, err)
	}
}

func dungKhoThat(db *sql.DB) *LoaiTaiNguyenBanDoStore {
	return NewLoaiTaiNguyenBanDoStore(pkgstore.New(db))
}

// --- (5) the column list against the real schema -------------------------------------------------

func TestPgCotTrongMaKhopVoiLuocDoThat(t *testing.T) {
	// THE REASON THIS FILE EXISTS. cotLoaiTaiNguyen is a string, and the fake driver builds its rows
	// from that same string — so a column renamed in a later migration, or misspelled here from the
	// start, is invisible to every test that does not touch a real schema. It would surface as a
	// 500 on the first real request, with "column ... does not exist" in a log nobody is reading yet.
	//
	// The predicate and ordering columns are checked alongside the SELECT list: `deleted_at` and
	// `tenant_id` never appear in cotLoaiTaiNguyen but the statement cannot run without them.
	db := moKetNoi(t)

	co := map[string]bool{}
	rows, err := db.Query(
		`SELECT column_name FROM information_schema.columns
		  WHERE table_schema = current_schema() AND table_name = 'loai_tai_nguyen_ban_do'`)
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
	if len(co) == 0 {
		t.Fatal("bảng loai_tai_nguyen_ban_do không tồn tại trong schema — tên bảng trong kho sai, hoặc migration 0003 chưa chạy")
	}

	var thieu []string
	for _, c := range strings.Split(cotLoaiTaiNguyen, ",") {
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

// --- (1) two communes ----------------------------------------------------------------------------

func TestPgDanhSachChiDocXaCuaChinhMinh(t *testing.T) {
	// THE MOST IMPORTANT CASE IN THIS FILE. The predicate that keeps two public authorities apart is
	// a single `WHERE tenant_id = $1` bound by Scoped.Query, and no test with one commune in it can
	// show whether it is there. The two rows also carry the SAME `ma`, which is legal — the unique
	// key is composite with tenant_id — and which makes a leak unmistakable in the assertion.
	db := moKetNoi(t)
	xa, xaKhac := xaRieng(t)

	themLoai(t, db, xa, "ltn-a", "mau-chung", "Nhóm của xã A", 1, true, true)
	themLoai(t, db, xaKhac, "ltn-b", "mau-chung", "Nhóm của xã B", 1, true, true)

	ra, err := dungKhoThat(db).DanhSach(ctxXa(tenant.ID(xa)))
	if err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	if len(ra) != 1 || ra[0].ID != "ltn-a" || ra[0].Nhan != "Nhóm của xã A" {
		t.Fatalf("RÒ RỈ hoặc thiếu dòng: %+v", ra)
	}
}

// --- (2) soft-deleted leaves, disabled stays ------------------------------------------------------

func TestPgDanhSachBoQuaDongDaXoaMem(t *testing.T) {
	// Rule 7, invariant 2: a soft-deleted row leaves EVERY read path. The UPDATE below is the only
	// way a row goes — the tier trigger refuses a hard DELETE outright, which is itself the reason
	// this read may never rely on rows disappearing physically.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themLoai(t, db, xa, "ltn-con", "mau-mot", "Nhóm mẫu một", 1, false, true)
	themLoai(t, db, xa, "ltn-xoa", "mau-hai", "Nhóm mẫu hai", 2, false, true)
	if _, err := db.Exec(
		`UPDATE loai_tai_nguyen_ban_do SET deleted_at = now(), deleted_by = $1, delete_reason = $2
		  WHERE tenant_id = $3 AND id = $4`,
		"CB-TEST", "test", xa, "ltn-xoa"); err != nil {
		t.Fatalf("xoá mềm: %v", err)
	}

	ra, err := dungKhoThat(db).DanhSach(ctxXa(tenant.ID(xa)))
	if err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	if len(ra) != 1 || ra[0].ID != "ltn-con" {
		t.Fatalf("dòng đã xoá mềm vẫn lọt vào danh sách: %+v", ra)
	}
}

func TestPgDanhSachGiuDongDaTat(t *testing.T) {
	// `dang_dung = false` AND `deleted_at IS NOT NULL` ARE DIFFERENT QUESTIONS, and this is where the
	// difference is checked against a server: a group taken out of use stays in the list carrying its
	// flag, because the catalogue screen shows it with a "Đã tắt" chip and every asset already filed
	// under it still has to render its name.
	//
	// The two flags are set to OPPOSITE values, so a predicate or a Scan that confused them cannot
	// produce a passing assertion.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themLoai(t, db, xa, "ltn-tat", "mau-mot", "Nhóm mẫu một", 1, true, false)

	ra, err := dungKhoThat(db).DanhSach(ctxXa(tenant.ID(xa)))
	if err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	if len(ra) != 1 {
		t.Fatalf("dòng đã tắt bị loại khỏi danh sách: %+v", ra)
	}
	if ra[0].DangDung {
		t.Error("dang_dung = false mà đọc ra true — cờ bị hoán đổi với cột bên cạnh")
	}
	if !ra[0].LaMacDinh {
		t.Error("la_mac_dinh = true mà đọc ra false — hai cột BOOLEAN kề nhau đã bị đọc ngược")
	}
}

// --- (3) the order PostgreSQL produces ------------------------------------------------------------

func TestPgDanhSachTheoThuTuRoiTheoMa(t *testing.T) {
	// `thu_tu` is the commune's own arrangement; `ma` breaks ties so two rows sharing a rank cannot
	// swap places between two calls. The two rows at rank 1 are inserted in the WRONG order on
	// purpose, so an ORDER BY that dropped the tie-break would be free to return them as inserted.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themLoai(t, db, xa, "ltn-3", "mau-cuoi", "Nhóm mẫu cuối", 2, false, true)
	themLoai(t, db, xa, "ltn-2", "mau-hai", "Nhóm mẫu hai", 1, false, true)
	themLoai(t, db, xa, "ltn-1", "mau-ba", "Nhóm mẫu ba", 1, false, true)

	ra, err := dungKhoThat(db).DanhSach(ctxXa(tenant.ID(xa)))
	if err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	muon := []string{"mau-ba", "mau-hai", "mau-cuoi"}
	if len(ra) != len(muon) {
		t.Fatalf("nhận %d dòng, muốn %d", len(ra), len(muon))
	}
	for i := range muon {
		if ra[i].Ma != muon[i] {
			t.Fatalf("thứ tự sai ở vị trí %d: %q, muốn %q", i, ra[i].Ma, muon[i])
		}
	}
}

// --- (4) the ceiling, against real rows -----------------------------------------------------------

func TestPgVuotTranThiTuChoiChuKhongCatBot(t *testing.T) {
	// ONE ROW OVER THE CEILING, which is the only interesting number: at exactly the ceiling the list
	// is complete and must come back whole, and the LIMIT is ceiling+1 precisely so that the extra
	// row is visible. The refusal must also carry NO rows — a caller handed a list alongside an error
	// is a caller that renders it.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	for i := 0; i <= TranDanhMucLoaiTaiNguyen; i++ {
		themLoai(t, db, xa, fmt.Sprintf("ltn-%04d", i), fmt.Sprintf("mau-%04d", i),
			fmt.Sprintf("Nhóm mẫu %d", i), i, false, true)
	}

	ra, err := dungKhoThat(db).DanhSach(ctxXa(tenant.ID(xa)))
	if !errors.Is(err, ErrQuaNhieuLoaiTaiNguyen) {
		t.Fatalf("err = %v, muốn ErrQuaNhieuLoaiTaiNguyen", err)
	}
	if ra != nil {
		t.Errorf("từ chối mà vẫn trả %d dòng — một danh sách đi kèm lỗi là một danh sách sẽ được vẽ", len(ra))
	}
}

// --- the assumption the read slice rests on --------------------------------------------------------

func TestPgChiMotMacDinhSongMoiXa(t *testing.T) {
	// NOT A TEST OF THE STORE, AND THAT IS DELIBERATE. The handler test asserts that the response
	// carries exactly ONE `is_default`, and nothing in Go enforces it: the guarantee is the generated
	// `moc_mac_dinh` column plus UNIQUE (tenant_id, moc_mac_dinh). If that ever weakens, the read
	// slice keeps passing while the map's selector opens on whichever row came back first — which is
	// to say at random, with nothing on the screen showing it.
	//
	// The second half is the part that is easy to get wrong in the other direction: soft-deleting the
	// default must FREE the slot, with no second statement to remember.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themLoai(t, db, xa, "ltn-md", "mau-mot", "Nhóm mẫu một", 1, true, true)

	_, err := db.Exec(
		`INSERT INTO loai_tai_nguyen_ban_do (tenant_id, id, ma, nhan, thu_tu, la_mac_dinh, dang_dung)
		 VALUES ($1,$2,$3,$4,$5,true,true)`,
		xa, "ltn-md2", "mau-hai", "Nhóm mẫu hai", 2)
	if err == nil {
		t.Fatal("xã có HAI mốc mặc định cùng sống — ô chọn nhóm của bản đồ sẽ mở theo thứ tự đọc, tức ngẫu nhiên")
	}

	if _, err := db.Exec(
		`UPDATE loai_tai_nguyen_ban_do SET deleted_at = now(), deleted_by = $1, delete_reason = $2
		  WHERE tenant_id = $3 AND id = $4`,
		"CB-TEST", "test", xa, "ltn-md"); err != nil {
		t.Fatalf("xoá mềm mốc mặc định: %v", err)
	}
	themLoai(t, db, xa, "ltn-md3", "mau-ba", "Nhóm mẫu ba", 3, true, true)

	ra, err := dungKhoThat(db).DanhSach(ctxXa(tenant.ID(xa)))
	if err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	if len(ra) != 1 || ra[0].ID != "ltn-md3" || !ra[0].LaMacDinh {
		t.Fatalf("xoá mềm mốc mặc định không giải phóng chỗ: %+v", ra)
	}
}
