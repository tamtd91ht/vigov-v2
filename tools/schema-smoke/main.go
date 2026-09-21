// Command schema-smoke applies every service's schema migrations to a real PostgreSQL, once,
// and reports exactly which file the server refused and why.
//
// WHY THIS EXISTS: `core/migrate` is the real runner, but the only things that called it were
// six per-service integration suites — and every one of them skips itself when VIGOV_TEST_DSN
// is unset, while still printing `ok`. So on a machine without a database the whole schema was
// reported green without a single statement reaching a server. `service-reporting` had no
// integration suite at all, so its migrations had no reader of any kind.
//
// This binary removes the skip: it takes a DSN, and if it cannot connect it FAILS rather than
// passing quietly. That difference is the entire point of the file.
//
// It is a development and CI tool. It creates ONE throwaway schema per service and applies the
// service's own embedded migrations into it through `migrate.Chay` — the same call
// `cmd/server/main.go` makes at startup, so what is verified here is the real startup path and
// not a re-implementation that could disagree with it.
//
// It holds no business data and writes none: rule 7 is about archival records, and a schema
// this process created seconds ago in a test database has none. It also never removes anything
// it did not create in this run.
//
//	go run ./schema-smoke            # needs VIGOV_TEST_DSN
//	go run ./schema-smoke -giu       # keep the schemas, to inspect them afterwards
package main

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/vihat/vigov/core/migrate"

	mgComms "github.com/vihat/vigov/service-comms/migrations"
	mgDocuments "github.com/vihat/vigov/service-documents/migrations"
	mgFinance "github.com/vihat/vigov/service-finance/migrations"
	mgIdentity "github.com/vihat/vigov/service-identity/migrations"
	mgPetitions "github.com/vihat/vigov/service-petitions/migrations"
	mgPlatform "github.com/vihat/vigov/service-platform/migrations"
	mgReporting "github.com/vihat/vigov/service-reporting/migrations"
)

// dichVu pairs a service name with the migrations its binary carries.
//
// The name is not decoration: `migrate.Chay` derives its advisory lock key from it, so the name
// here must be the one `cmd/server/main.go` passes, or this tool would exercise a lock key no
// deployment ever takes.
type dichVu struct {
	ten string
	fs  embed.FS
}

// danhSach lists every service that ships migrations. ALL SEVEN, including service-reporting,
// which has no integration suite — before this tool its two migration files had no reader at
// all, in tests or anywhere else.
func danhSach() []dichVu {
	return []dichVu{
		{"platform", mgPlatform.FS},
		{"identity", mgIdentity.FS},
		{"comms", mgComms.FS},
		{"documents", mgDocuments.FS},
		{"finance", mgFinance.FS},
		{"petitions", mgPetitions.FS},
		{"reporting", mgReporting.FS},
	}
}

func main() {
	giu := flag.Bool("giu", false, "giữ lại schema sau khi chạy, để xem xét bằng tay")
	flag.Parse()

	// Công cụ phát triển/CI, không phải tiến trình phục vụ. Nó CỐ Ý không đi qua core/config:
	// core/config mô tả cấu hình của một dịch vụ ĐANG CHẠY, còn đây là DSN của một CSDL dùng một
	// lần. Đưa nó vào core/config là thêm một biến vào hợp đồng vận hành thật cho một thứ không
	// bao giờ tồn tại trong cụm. Cùng tên biến mà các bộ test tích hợp đang dùng.
	// @env-ok: DSN của CSDL thử dùng một lần, không phải cấu hình của dịch vụ chạy trong cụm
	dsn := os.Getenv("VIGOV_TEST_DSN")
	if dsn == "" {
		// FAIL, not skip. A skip here would reproduce the exact defect this tool exists to end.
		fmt.Fprintln(os.Stderr, "schema-smoke: thiếu VIGOV_TEST_DSN — cần một PostgreSQL thật.")
		fmt.Fprintln(os.Stderr, "  Công cụ này CỐ Ý không tự bỏ qua: một lược đồ báo xanh mà không")
		fmt.Fprintln(os.Stderr, "  câu lệnh nào tới được máy chủ đúng là lỗi nó sinh ra để chấm dứt.")
		os.Exit(2)
	}

	if ma := chay(dsn, *giu); ma != 0 {
		os.Exit(ma)
	}
}

func chay(dsn string, giu bool) int {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "schema-smoke: mở kết nối:", err)
		return 2
	}
	defer db.Close() //nolint:errcheck // đang thoát tiến trình

	// ONE physical connection for the whole run, for the reason checker_pg_test.go documents:
	// `SET search_path` is SESSION state, so on a pool it applies to whichever connection served
	// that one statement, and the next statement can land back in `public`.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := db.PingContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "schema-smoke: không nối được tới CSDL:", err)
		return 2
	}

	loat := time.Now().UnixNano()
	hong := 0

	// Every message below formats an ERROR with %s, never %v. `%v` on an unknown value is how a
	// struct full of citizen fields reaches a log line (rule 3); an error prints the same text
	// under %s and cannot drag a struct in behind it.
	for _, d := range danhSach() {
		schema := fmt.Sprintf("smoke_%s_%d", d.ten, loat)
		fmt.Printf("\n=== %s → schema %s ===\n", d.ten, schema)

		if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
			fmt.Printf("  TẠO SCHEMA HỎNG: %s\n", err)
			hong++
			continue
		}
		if _, err := db.ExecContext(ctx, "SET search_path TO "+schema); err != nil {
			fmt.Printf("  ĐẶT search_path HỎNG: %s\n", err)
			hong++
			continue
		}

		kq, err := migrate.Chay(ctx, db, d.fs, d.ten)
		for _, t := range kq.DaAp {
			fmt.Printf("  ÁP   %s\n", t)
		}
		if err != nil {
			// The verbatim server message, unwrapped as far as it goes. A summarised error is an
			// error somebody has to reproduce before they can act on it.
			fmt.Printf("  HỎNG: %s\n", err)
			for u := errors.Unwrap(err); u != nil; u = errors.Unwrap(u) {
				fmt.Printf("    ↳ %s\n", u)
			}
			hong++
			continue
		}
		fmt.Printf("  XANH: %d tệp đã áp\n", len(kq.DaAp))
	}

	if giu {
		fmt.Printf("\n-giu: schema hậu tố _%d còn nguyên để xem xét.\n", loat)
	} else {
		donSchema(db, loat)
	}

	fmt.Printf("\n===== %d dịch vụ HỎNG =====\n", hong)
	if hong > 0 {
		return 1
	}
	return 0
}

// donSchema removes only the schemas THIS run created, identified by the run's own suffix.
//
// Scoped deliberately: a sweep over "anything that looks like a smoke schema" would take a
// concurrent run's schemas with it, and this tool is meant to be safe to run twice at once.
func donSchema(db *sql.DB, loat int64) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	for _, d := range danhSach() {
		schema := fmt.Sprintf("smoke_%s_%d", d.ten, loat)
		if _, err := db.ExecContext(ctx,
			fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schema)); err != nil {
			fmt.Fprintf(os.Stderr, "dọn %s: %s\n", schema, err)
		}
	}
}
