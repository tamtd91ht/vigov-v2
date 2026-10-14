package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/vihat/vigov/core/config"
	"github.com/vihat/vigov/core/secret"
)

// Exit codes of RunMigrateOnly. Jenkins' `tao-bang` one-shot pod reads them, so they are fixed.
const (
	ExitMigrateOK     = 0 // every file applied or already in the ledger
	ExitMigrateFailed = 1 // database unreachable, checksum mismatch, a failing file
	ExitMigrateConfig = 2 // ENV / DATABASE_DSN missing or invalid; nothing was attempted
)

// migrateOnlyPrefix starts every line this mode prints. The first line,
// "migrate-only: <service> start", is printed before anything can fail: Jenkins treats its
// ABSENCE as an old image that ignored the argument and started the server instead.
const migrateOnlyPrefix = "migrate-only: "

// RunMigrateOnly applies the service's embedded migrations and returns an exit code; the
// caller passes it to os.Exit. It is what `/server -migrate-only` runs, so Jenkins' `tao-bang`
// job can create tables with the Deployment's current image without starting the server
// (ADR 0013, path two).
//
// service MUST be the same string the server passes to Chay: it is the advisory lock key, so a
// different spelling would let this pod and a starting replica migrate at the same time.
//
// The driver is the one the calling main registers (`_ "github.com/jackc/pgx/v5/stdlib"`), which
// keeps pgx out of core's go.mod. Fail-closed behaviour is Chay's, unchanged: no adopt, no
// baseline, no skipping a failing file.
func RunMigrateOnly(service string, files fs.FS) int {
	open := func(dsn string) (*sql.DB, error) { return sql.Open("pgx", dsn) }
	return runMigrateOnly(os.Stdout, service, files, config.Load, open)
}

type loadConfigFunc func(service string, uses config.Usage) (config.Config, error)

type openDBFunc func(dsn string) (*sql.DB, error)

func runMigrateOnly(out io.Writer, service string, files fs.FS, load loadConfigFunc, open openDBFunc) int {
	line := func(format string, args ...any) {
		_, _ = fmt.Fprintf(out, "%s%s %s\n", migrateOnlyPrefix, service, fmt.Sprintf(format, args...))
	}
	line("start")

	if strings.TrimSpace(service) == "" || files == nil {
		line("FAILED: configuration: service name or migration files missing")
		return ExitMigrateConfig
	}

	// An EMPTY group set: this mode reads ENV and DATABASE_DSN only, so a pod that lacks the
	// server's Redis or gRPC variables can still create tables (same as operatorctl).
	cfg, err := load(service, config.Uses())
	if err != nil {
		line("FAILED: configuration: %s", redactError(err, ""))
		return ExitMigrateConfig
	}
	dsn := cfg.DatabaseDSN.Lo()

	db, err := open(dsn)
	if err != nil {
		line("FAILED: open database: %s", redactError(err, dsn))
		return ExitMigrateFailed
	}
	defer db.Close() //nolint:errcheck // process exits right after; nothing to recover
	db.SetMaxOpenConns(2)

	pingCtx, cancelPing := context.WithTimeout(context.Background(), 30*time.Second)
	err = db.PingContext(pingCtx)
	cancelPing()
	if err != nil {
		line("FAILED: cannot reach the database: %s", redactError(err, dsn))
		return ExitMigrateFailed
	}

	// Tables but no ledger: Chay would create an empty ledger and re-apply from 0001, whose
	// IF NOT EXISTS makes that a silent adopt. The owner rejected adopting (14/10/2026, ADR 0013),
	// so this mode stops. Startup (Chay alone) deliberately keeps its behaviour: a refusal there
	// could take a running prod service down — a known gap, not an oversight.
	checkCtx, cancelCheck := context.WithTimeout(context.Background(), 30*time.Second)
	hasLedger, tables, err := ledgerState(checkCtx, db)
	cancelCheck()
	if err != nil {
		line("FAILED: inspect database: %s", redactError(err, dsn))
		return ExitMigrateFailed
	}
	if !hasLedger && tables > 0 {
		line("FAILED: database has %d tables but no %s ledger — not adopting; a person decides (ADR 0013)",
			tables, BangTienDo)
		return ExitMigrateFailed
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	result, err := Chay(ctx, db, files, service)
	cancel()
	// Printed on failure too: files before the failing one are committed, and the operator needs
	// to know which ones before rerunning.
	for _, name := range result.DaAp {
		line("applied %s", name)
	}
	if err != nil {
		line("FAILED: %s", redactError(err, dsn))
		return ExitMigrateFailed
	}
	line("ok applied=%d skipped=%d", len(result.DaAp), len(result.BoQua))
	return ExitMigrateOK
}

// ledgerState reports whether the ledger table exists and how many base tables the current
// schema holds. The ledger name is unqualified, as Chay creates it, so both resolve it through
// the same search_path.
func ledgerState(ctx context.Context, db *sql.DB) (hasLedger bool, tables int64, err error) {
	err = db.QueryRowContext(ctx, `
SELECT to_regclass('`+BangTienDo+`') IS NOT NULL,
       (SELECT count(*) FROM information_schema.tables
         WHERE table_schema = current_schema() AND table_type = 'BASE TABLE')`).
		Scan(&hasLedger, &tables)
	if err != nil {
		return false, 0, fmt.Errorf("migrate: read ledger state: %w", err)
	}
	return hasLedger, tables, nil
}

// passwordKeyword matches a keyword-form password (`password=...`) anywhere in an error text.
var passwordKeyword = regexp.MustCompile(`(?i)(password\s*=\s*)('(?:[^'\\]|\\.)*'|\S+)`)

// redactError renders err with the DSN and its password removed. Driver errors can echo the
// connection string they were given; this line lands in pod logs, so the credential must not
// (rule 8). It over-redacts on purpose: a mangled message is cheaper than a leaked password.
func redactError(err error, dsn string) string {
	msg := err.Error()
	for _, s := range sensitiveParts(dsn) {
		msg = strings.ReplaceAll(msg, s, secret.Che)
	}
	return passwordKeyword.ReplaceAllString(msg, "${1}"+secret.Che)
}

// sensitiveParts lists the substrings of dsn that must never be printed: the whole string and
// its password, raw and URL-decoded.
func sensitiveParts(dsn string) []string {
	if dsn == "" {
		return nil
	}
	parts := []string{dsn}
	// URL form: the password as written (possibly %-encoded) and decoded, since a driver may
	// echo either.
	if _, rest, found := strings.Cut(dsn, "://"); found {
		if at := strings.LastIndex(rest, "@"); at >= 0 {
			if colon := strings.Index(rest[:at], ":"); colon >= 0 {
				raw := rest[colon+1 : at]
				if raw != "" {
					parts = append(parts, raw)
				}
				if dec, err := url.PathUnescape(raw); err == nil && dec != "" && dec != raw {
					parts = append(parts, dec)
				}
			}
		}
	}
	for _, m := range passwordKeyword.FindAllStringSubmatch(dsn, -1) {
		if v := strings.Trim(m[2], "'"); v != "" {
			parts = append(parts, v)
		}
	}
	return parts
}
