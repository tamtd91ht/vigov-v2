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
	"github.com/vihat/vigov/service-documents/migrations"
)

// Integration tests for DocumentTypeStore.List against a real PostgreSQL.
//
// WHY A REAL DATABASE: every property asserted below lives in the SQL and nowhere else — the
// `tenant_id = $1` predicate Scoped.Query adds, the `deleted_at IS NULL` filter, the ORDER BY, and
// the LIMIT that is the ceiling plus one. A fake would only agree with whatever the query already
// does, which is the opposite of a test.
//
// SKIPPED UNLESS VIGOV_TEST_DSN IS SET, AND ON A MACHINE WITHOUT IT THE PACKAGE STILL PRINTS `ok`.
// That is this repository's worst trap: a green run here is evidence about the Go code only, never
// about the SQL. Do not report the statements below as verified on a run where they skipped.
//
// The DSN carries a password and lives only in the environment (rule 8).

// The schema is built ONCE for the whole package, not per test: migration 0003 alone creates 32
// partitions, and a suite slow enough to skip is a suite that stops being run. Each test isolates
// itself with its own commune ids instead, which is also closer to how the data really looks.
var (
	sharedDB     *sql.DB
	sharedSchema string
)

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

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	sharedSchema = fmt.Sprintf("vigov_vt_test_%d", time.Now().UnixNano())
	// ONE physical connection for the whole suite. `SET search_path` is SESSION state, so on a pool
	// it applies to whichever connection happened to serve that statement and to no other — the next
	// statement can land on a fresh connection in the public schema.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+sharedSchema); err != nil {
		fmt.Fprintln(os.Stderr, "tạo schema:", err)
		os.Exit(1)
	}
	if _, err := db.ExecContext(ctx, "SET search_path TO "+sharedSchema); err != nil {
		fmt.Fprintln(os.Stderr, "đặt search_path:", err)
		os.Exit(1)
	}

	// THE REAL RUNNER, not a hand-picked file: every migration this service ships is exercised
	// here, and a new one is exercised the day it is added rather than the day somebody remembers
	// to extend a list.
	res, err := migrate.Chay(ctx, db, migrations.FS, "documents")
	if err != nil {
		fmt.Fprintln(os.Stderr, "chạy migration:", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "migration đã áp:", res.DaAp)

	sharedDB = db
	code := m.Run()

	// A schema this suite created, in a test database. Rule 7 protects archival business data; this
	// holds none.
	if _, err := db.ExecContext(context.Background(),
		"DROP SCHEMA IF EXISTS "+sharedSchema+" CASCADE"); err != nil {
		fmt.Fprintln(os.Stderr, "dọn schema:", err)
	}
	db.Close()
	os.Exit(code)
}

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	if sharedDB == nil {
		t.Skip("VIGOV_TEST_DSN chưa đặt — bỏ qua test tích hợp")
	}
	return sharedDB
}

// uniqueTenants returns commune ids unique to this test, so tests sharing the schema cannot see each
// other's rows. Two are returned for the tests that prove isolation.
func uniqueTenants(t *testing.T) (string, string) {
	t.Helper()
	n := time.Now().UnixNano()
	a := fmt.Sprintf("%026d", n)
	b := fmt.Sprintf("%026d", n+1)
	return a[:26], b[:26]
}

func tenantCtx(id string) context.Context {
	return tenant.Into(context.Background(), tenant.ID(id))
}

func newDocumentTypeStore(db *sql.DB) *DocumentTypeStore {
	return NewDocumentTypeStore(pkgstore.New(db))
}

// seedDocumentType inserts one catalogue row. `nguon` is left at its default 'don-vi' — a commune's own
// row — because the tier guards are the migration's subject, not this store's.
func seedDocumentType(t *testing.T, db *sql.DB, tenantID, id, code, label string, sortOrder int, isActive, isDefault bool) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO loai_van_ban (tenant_id, id, ma, nhan, thu_tu, dang_dung, la_mac_dinh)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		tenantID, id, code, label, sortOrder, isActive, isDefault)
	if err != nil {
		t.Fatalf("thêm loại văn bản %s: %v", code, err)
	}
}

// --- the column list against the real schema --------------------------------------------------

func TestPgColumnsInCodeMatchTheRealSchema(t *testing.T) {
	// THE ONE ASSERTION NO FAKE CAN MAKE, and the exact gap document_type_test.go names about itself:
	// documentTypeColumns is a string, and the fake driver builds its rows out of that same string — so a
	// column misspelled here from the start, or renamed by a later migration, is invisible to every
	// test that never touches a real schema. It would surface as a 500 on the first real request,
	// with "column ... does not exist" in a log nobody is reading yet.
	//
	// WHY IT IS WORTH A TEST OF ITS OWN rather than being left to the four behavioural cases below:
	// those fail for many reasons at once, so a typo'd column name reads as "the query returned
	// nothing" and sends the next person looking at the DATA. This one names the missing column in
	// one line.
	//
	// `tenant_id` and `deleted_at` are checked alongside the SELECT list although neither ever
	// appears in it: the statement is built around both — Scoped.Query binds the first, the tail
	// filters on the second — so either of them disappearing breaks the read just as completely.
	// `thu_tu` goes in for the same reason: it is the ORDER BY and nothing else.
	db := openDB(t)

	has := map[string]bool{}
	rows, err := db.Query(
		`SELECT column_name FROM information_schema.columns
		  WHERE table_schema = current_schema() AND table_name = 'loai_van_ban'`)
	if err != nil {
		t.Fatalf("đọc information_schema: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			t.Fatalf("đọc tên cột: %v", err)
		}
		has[c] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("duyệt tên cột: %v", err)
	}
	// ASSERTED FIRST, AND SEPARATELY: an empty result means the TABLE is not there, which is a
	// different fault from a missing column — the store names the wrong relation, or migration 0003
	// never ran in this schema. Rolled into the loop below it would read as "every column missing".
	if len(has) == 0 {
		t.Fatal("bảng loai_van_ban không tồn tại trong schema — tên bảng trong kho sai, hoặc migration 0003 chưa chạy")
	}

	var missing []string
	for _, c := range strings.Split(documentTypeColumns, ",") {
		if c = strings.TrimSpace(c); c != "" && !has[c] {
			missing = append(missing, c)
		}
	}
	// Not part of the SELECT list, but the statement cannot run without them.
	for _, c := range []string{"tenant_id", "deleted_at", "thu_tu"} {
		if !has[c] {
			missing = append(missing, c)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("cột không có trong bảng thật: %v — câu lệnh sẽ hỏng ở lần gọi thật đầu tiên", missing)
	}
}

func TestPgListInSortOrderReadsActiveAndDefaultFlags(t *testing.T) {
	db := openDB(t)
	tenantID, _ := uniqueTenants(t)

	// Inserted OUT of display order, so a query that forgot its ORDER BY could not pass by luck.
	seedDocumentType(t, db, tenantID, "lvb-2", "cong-van", "Công văn", 20, false, false)
	seedDocumentType(t, db, tenantID, "lvb-1", "quyet-dinh", "Quyết định", 10, true, true)

	out, err := newDocumentTypeStore(db).List(tenantCtx(tenantID))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("nhận %d dòng, muốn 2", len(out))
	}
	if out[0].Code != "quyet-dinh" || out[1].Code != "cong-van" {
		t.Fatalf("sai thứ tự thu_tu: %+v", out)
	}
	if !out[0].IsDefault || !out[0].IsActive {
		t.Errorf("cờ của dòng mặc định đọc sai: %+v", out[0])
	}
	// A row out of use IS returned, carrying dang_dung=false. The configuration screen shows it
	// with a "Đã tắt" chip, and an old document registered under it still needs its label.
	if out[1].IsActive {
		t.Errorf("dòng đã tắt đọc thành đang dùng: %+v", out[1])
	}
	if out[1].Label != "Công văn" {
		t.Errorf("nhãn đọc sai — có thể ma/nhan đã bị hoán vị theo vị trí: %+v", out[1])
	}
}

func TestPgSoftDeletedRowLeavesTheList(t *testing.T) {
	// Rule 7, invariant 2: every read path excludes soft-deleted rows — and invariant 1: the row
	// itself stays. Both halves are asserted, because a query that hard-deleted would pass the
	// first on its own.
	db := openDB(t)
	tenantID, _ := uniqueTenants(t)

	seedDocumentType(t, db, tenantID, "lvb-1", "cong-van", "Công văn", 10, true, false)
	if _, err := db.Exec(
		`UPDATE loai_van_ban SET deleted_at = now(), deleted_by = $3, delete_reason = $4
		 WHERE tenant_id = $1 AND id = $2`,
		tenantID, "lvb-1", "CB-001", "gộp vào loại khác"); err != nil {
		t.Fatalf("xoá mềm: %v", err)
	}

	out, err := newDocumentTypeStore(db).List(tenantCtx(tenantID))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("dòng đã xoá mềm vẫn hiện lên màn hình: %+v", out)
	}

	var n int
	if err := db.QueryRow(`SELECT count(*) FROM loai_van_ban WHERE tenant_id = $1 AND id = $2`,
		tenantID, "lvb-1").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("dòng bị xoá thật: count = %d, muốn 1", n)
	}
}

func TestPgOneCommunesCatalogueNeverLeaksIntoAnother(t *testing.T) {
	// THE ONE THAT MATTERS MOST. Two communes, the SAME `ma` and the same display order — which the
	// schema allows, because every unique key is composite with tenant_id. The only thing keeping
	// them apart is the predicate Scoped.Query binds, and this is what proves it binds.
	db := openDB(t)
	tenantA, tenantB := uniqueTenants(t)

	seedDocumentType(t, db, tenantA, "lvb-a", "cong-van", "Công văn xã A", 10, true, false)
	seedDocumentType(t, db, tenantB, "lvb-b", "cong-van", "Công văn xã B", 10, true, false)

	out, err := newDocumentTypeStore(db).List(tenantCtx(tenantA))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(out) != 1 || out[0].ID != "lvb-a" {
		t.Fatalf("RÒ RỈ hoặc đọc thiếu: %+v", out)
	}
}

func TestPgOverTheCeilingRefusesRatherThanTruncates(t *testing.T) {
	// The ceiling plus one is what makes "too many" DETECTABLE: at exactly the ceiling the result
	// is a complete list, and one row past it the store refuses instead of handing back a list the
	// caller would render as if it were whole.
	db := openDB(t)
	tenantID, _ := uniqueTenants(t)

	for i := 0; i <= MaxDocumentTypes; i++ {
		seedDocumentType(t, db, tenantID,
			fmt.Sprintf("lvb-%04d", i), fmt.Sprintf("loai-%04d", i),
			fmt.Sprintf("Loại %04d", i), i, true, false)
	}

	out, err := newDocumentTypeStore(db).List(tenantCtx(tenantID))
	if !errors.Is(err, ErrTooManyDocumentTypes) {
		t.Fatalf("err = %v, muốn ErrTooManyDocumentTypes", err)
	}
	// Nothing comes back with the refusal: a list handed over alongside an error is a truncation
	// waiting for one careless `if err != nil { log }`.
	if out != nil {
		t.Errorf("từ chối vẫn kèm %d dòng", len(out))
	}
}
