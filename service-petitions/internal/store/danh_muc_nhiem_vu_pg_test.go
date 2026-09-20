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
	"github.com/vihat/vigov/service-petitions/migrations"
)

// Integration tests for the two catalogue reads, against a REAL PostgreSQL.
//
// WHY A REAL DATABASE: every property asserted below lives in the SQL and is invisible from the Go
// side — the `tenant_id = $1` predicate store.Scoped adds, `deleted_at IS NULL`, ORDER BY thu_tu
// with `ma` as the tie-break, and LIMIT ceiling+1. A fake would only agree with whatever the query
// already does.
//
// READ THIS BEFORE BELIEVING A GREEN RUN: these tests SKIP unless VIGOV_TEST_DSN is set, and the
// package still prints `ok`. There is no PostgreSQL reachable from the build environment this was
// written in, so NONE of the assertions below have ever been executed. `ok` next to this package
// means the handler tests ran; it does not mean the SQL here was verified. The DSN carries a
// password and lives only in the environment (rule 8).

// The schema is built ONCE for the whole package, not per test: migration 0003 alone creates two
// partitioned tables with 32 partitions each. Tests isolate themselves with their own commune ids
// instead, which is also closer to how the data really looks.
var (
	dbChung     *sql.DB
	schemaChung string
)

func TestMain(m *testing.M) {
	dsn := os.Getenv("VIGOV_TEST_DSN")
	if dsn == "" {
		os.Exit(m.Run()) // every test skips itself
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mở kết nối:", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	schemaChung = fmt.Sprintf("vigov_pet_test_%d", time.Now().UnixNano())
	// ONE physical connection for the whole suite. `SET search_path` is SESSION state, so on a pool
	// it applies to whichever connection happened to serve that statement and to no other — the
	// next statement can land on a fresh connection in the public schema.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+schemaChung); err != nil {
		fmt.Fprintln(os.Stderr, "tạo schema:", err)
		os.Exit(1)
	}
	if _, err := db.ExecContext(ctx, "SET search_path TO "+schemaChung); err != nil {
		fmt.Fprintln(os.Stderr, "đặt search_path:", err)
		os.Exit(1)
	}

	// THE REAL RUNNER over every migration this service ships, not a hand-picked file: a migration
	// added later is exercised the day it is added rather than the day somebody remembers to extend
	// a list here.
	kq, err := migrate.Chay(ctx, db, migrations.FS, "petitions")
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
		"DROP SCHEMA IF EXISTS "+schemaChung+" CASCADE"); err != nil {
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
// other's rows. Two ids, because the isolation cases need a second commune.
func xaRieng(t *testing.T) (string, string) {
	t.Helper()
	n := time.Now().UnixNano()
	return fmt.Sprintf("%026d", n), fmt.Sprintf("%026d", n+1)
}

// ctxXa LIVES IN driver_gia_test.go, not here, and is shared with the fake-driver suites: one
// helper, so a change to how the commune enters a context cannot apply to one suite and not the
// other. It takes a tenant.ID; the ids these tests generate are strings, hence the conversion at
// each call.

// themLoai inserts one task-type row. thu_tu AND dang_dung ARE PASSED EXPLICITLY: the schema
// defaults them (0 and true), and a test relying on a default cannot tell "the column is read" from
// "every row happens to have the same value".
func themLoai(t *testing.T, db *sql.DB, xa, id, ma, nhan string, thuTu int, dangDung bool) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO loai_nhiem_vu (tenant_id, id, ma, nhan, thu_tu, dang_dung)
		 VALUES ($1,$2,$3,$4,$5,$6)`, xa, id, ma, nhan, thuTu, dangDung)
	if err != nil {
		t.Fatalf("thêm loại nhiệm vụ: %v", err)
	}
}

func themUuTien(t *testing.T, db *sql.DB, xa, id, ma, nhan string, thuTu int) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO muc_uu_tien_nhiem_vu (tenant_id, id, ma, nhan, thu_tu)
		 VALUES ($1,$2,$3,$4,$5)`, xa, id, ma, nhan, thuTu)
	if err != nil {
		t.Fatalf("thêm mức ưu tiên: %v", err)
	}
}

func TestPgCotTrongMaKhopVoiLuocDoThat(t *testing.T) {
	// THE ONE ASSERTION ANYWHERE THAT CATCHES A LATER MIGRATION RENAMING A COLUMN OUT FROM UNDER A
	// STORE. cotLoaiNhiemVu and cotMucUuTien are strings, and the fake driver in driver_gia_test.go
	// builds its rows from those same strings — so a column misspelled here from the start, or
	// renamed in a migration written months from now, is invisible to every test that does not touch
	// a real schema. It surfaces as a 500 on the first real request, with "column … does not exist"
	// in a log nobody is reading yet.
	//
	// WHY IT IS ITS OWN TEST rather than left to the four behavioural cases below: those fail
	// together for many possible reasons, so a typo'd column reads as "the query returned nothing"
	// and sends the next person looking at the DATA instead of at the STRING. This one names the
	// missing column.
	//
	// THREE COLUMNS ARE CHECKED THAT NEVER APPEAR IN A SELECT LIST, because the statement is built
	// around them and cannot run without them:
	//
	//	tenant_id   the predicate that keeps two public authorities apart (rule 1)
	//	deleted_at  the predicate that keeps archival rows out of every read path (rule 7)
	//	thu_tu      the ORDER BY — and for muc_uu_tien_nhiem_vu it IS the rank, so a rename there
	//	            destroys the scale silently: the list still arrives, in an order that means
	//	            nothing, and every screen still shows plausible words
	db := moKetNoi(t)

	for _, tr := range []struct {
		bang string
		cot  string
	}{
		{"loai_nhiem_vu", cotLoaiNhiemVu},
		{"muc_uu_tien_nhiem_vu", cotMucUuTien},
	} {
		t.Run(tr.bang, func(t *testing.T) {
			co := map[string]bool{}
			rows, err := db.Query(
				`SELECT column_name FROM information_schema.columns
				  WHERE table_schema = current_schema() AND table_name = $1`, tr.bang)
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
			// THE TABLE ITSELF FIRST. With no rows from information_schema every column below would
			// be reported missing, which reads like a dozen renames instead of one wrong table name
			// — or a migration that never ran.
			if len(co) == 0 {
				t.Fatalf("bảng %s không tồn tại trong schema — tên bảng trong kho sai, hoặc migration 0003 chưa chạy", tr.bang)
			}

			var thieu []string
			for _, c := range strings.Split(tr.cot, ",") {
				if c = strings.TrimSpace(c); c != "" && !co[c] {
					thieu = append(thieu, c)
				}
			}
			for _, c := range []string{"tenant_id", "thu_tu", "deleted_at"} {
				if !co[c] {
					thieu = append(thieu, c)
				}
			}
			if len(thieu) > 0 {
				sort.Strings(thieu)
				t.Fatalf("cột không có trong bảng %s thật: %v — câu lệnh sẽ hỏng ở lần gọi thật đầu tiên",
					tr.bang, thieu)
			}
		})
	}
}

func TestPgLoaiNhiemVuChiDocXaCuaMinh(t *testing.T) {
	// THE PROPERTY THAT CANNOT BE FAKED: the store takes no commune, so the only thing that can
	// keep commune B's rows out of commune A's answer is the `tenant_id = $1` predicate
	// store.Scoped adds. This is the test that proves the predicate is actually there.
	db := moKetNoi(t)
	xa, xaKhac := xaRieng(t)

	themLoai(t, db, xa, "lnv-"+xa, "theo-van-ban", "Theo văn bản", 1, true)
	themLoai(t, db, xaKhac, "lnv-"+xaKhac, "chi-cua-xa-khac", "Chỉ của xã khác", 1, true)

	ds, err := NewLoaiNhiemVuStore(pkgstore.New(db)).DanhSach(ctxXa(tenant.ID(xa)))
	if err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	if len(ds) != 1 || ds[0].Ma != "theo-van-ban" {
		t.Fatalf("RÒ RỈ hoặc thiếu dòng: %+v", ds)
	}
}

func TestPgLoaiNhiemVuBoDongXoaMemNhungGiuDongDaTat(t *testing.T) {
	// TWO DIFFERENT STATES THAT LOOK ALIKE ON A SCREEN AND MUST NOT BEHAVE ALIKE HERE.
	//
	//	soft deleted  drops out of every read path — rule 7, invariant 2
	//	out of use    STAYS, flagged: an older task holds the code and would otherwise render as a
	//	              raw slug with no label
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themLoai(t, db, xa, "lnv-song-"+xa, "co-ban", "Cơ bản", 1, true)
	themLoai(t, db, xa, "lnv-tat-"+xa, "viec-cu", "Việc cũ", 2, false)
	themLoai(t, db, xa, "lnv-xoa-"+xa, "da-xoa", "Đã xoá", 3, true)
	if _, err := db.Exec(
		`UPDATE loai_nhiem_vu SET deleted_at = now(), delete_reason = $3
		 WHERE tenant_id = $1 AND id = $2`, xa, "lnv-xoa-"+xa, "sắp xếp lại danh mục"); err != nil {
		t.Fatalf("xoá mềm: %v", err)
	}

	ds, err := NewLoaiNhiemVuStore(pkgstore.New(db)).DanhSach(ctxXa(tenant.ID(xa)))
	if err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	if len(ds) != 2 {
		t.Fatalf("nhận %d dòng, muốn 2 (bỏ dòng đã xoá mềm, giữ dòng đã tắt): %+v", len(ds), ds)
	}
	if ds[1].Ma != "viec-cu" || ds[1].DangDung {
		t.Errorf("dòng đã tắt sai: %+v", ds[1])
	}
	// The soft-deleted row is still in the table — rule 7 keeps it, this read simply does not name
	// it. A read path that "cleaned up" would be destroying a commune's record.
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM loai_nhiem_vu WHERE tenant_id = $1`, xa).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("còn %d dòng trong bảng, muốn 3 — dữ liệu đã bị xoá thật", n)
	}
}

func TestPgMucUuTienDungThuTuThangVaPhaVoBangBangMa(t *testing.T) {
	// ORDER IS THE MEANING OF THIS LIST. Two properties in one test because they are one ORDER BY:
	//
	//	thu_tu   the commune's own ranking
	//	ma       the tie-break, which makes the order TOTAL — two levels sharing a thu_tu cannot
	//	         swap places between two calls, so a client-side diff does not flicker and a test
	//	         cannot compare sets by accident
	//
	// The codes are inserted in an order that is neither the expected one nor alphabetical.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	themUuTien(t, db, xa, "uu-thuong-"+xa, "thuong", "Thường", 3)
	themUuTien(t, db, xa, "uu-cao-"+xa, "cao", "Cao", 2)
	themUuTien(t, db, xa, "uu-khan-"+xa, "khan", "Khẩn", 1)
	// Two levels deliberately share thu_tu = 2: without the `ma` tie-break the order between these
	// two is whatever the plan happens to produce.
	themUuTien(t, db, xa, "uu-caohon-"+xa, "cao-hon", "Cao hơn", 2)

	ds, err := NewMucUuTienNhiemVuStore(pkgstore.New(db)).DanhSach(ctxXa(tenant.ID(xa)))
	if err != nil {
		t.Fatalf("DanhSach: %v", err)
	}
	muon := []string{"khan", "cao", "cao-hon", "thuong"}
	if len(ds) != len(muon) {
		t.Fatalf("nhận %d mức, muốn %d: %+v", len(ds), len(muon), ds)
	}
	for i, ma := range muon {
		if ds[i].Ma != ma {
			t.Fatalf("sai hạng ở vị trí %d: %q, muốn %q", i, ds[i].Ma, ma)
		}
	}
}

func TestPgMucUuTienVuotTranThiTuChoiChuKhongCatBot(t *testing.T) {
	// THE CEILING IS ENFORCED IN SQL (LIMIT ceiling+1) AND THE EXTRA ROW IS WHAT MAKES "too many"
	// DETECTABLE. Asserted here rather than in the handler tests because only the query knows the
	// LIMIT: a handler test can prove the error is not swallowed, never that it is raised.
	db := moKetNoi(t)
	xa, _ := xaRieng(t)

	for i := 0; i <= TranDanhMucMucUuTien; i++ {
		themUuTien(t, db, xa, fmt.Sprintf("uu-%03d-%s", i, xa), fmt.Sprintf("muc-%03d", i),
			fmt.Sprintf("Mức %03d", i), i)
	}

	ds, err := NewMucUuTienNhiemVuStore(pkgstore.New(db)).DanhSach(ctxXa(tenant.ID(xa)))
	if !errors.Is(err, ErrQuaNhieuMucUuTien) {
		t.Fatalf("err = %v, muốn ErrQuaNhieuMucUuTien", err)
	}
	// NOTHING COMES BACK WITH THE REFUSAL. A partial list handed to a caller that logs the error and
	// renders anyway is the silent truncation this whole mechanism exists to prevent.
	if ds != nil {
		t.Errorf("từ chối mà vẫn trả %d dòng", len(ds))
	}
}
