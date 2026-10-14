package migrate

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/config"
	"github.com/vihat/vigov/core/secret"
)

// Jenkins' `tao-bang` pod decides from this output and the exit code alone: the start marker
// proves the image knows the argument, the verdict line says what happened. A password in any of
// these lines would sit in the job log of every run (rule 8).

const testPassword = "Sentinel%2Fpw-7f3a" // %-encoded on purpose: the decoded form must go too

func testDSN() string { return "postgres://mig:" + testPassword + "@db.invalid:5432/vigov" }

func loadWith(dsn string) loadConfigFunc {
	return func(string, config.Usage) (config.Config, error) {
		return config.Config{DatabaseDSN: secret.DSN(dsn)}, nil
	}
}

func outputLines(b *bytes.Buffer) []string {
	return strings.Split(strings.TrimRight(b.String(), "\n"), "\n")
}

func assertNoCredential(t *testing.T, out string) {
	t.Helper()
	for _, s := range []string{testDSN(), testPassword, "Sentinel/pw-7f3a", "kwSecret-91"} {
		if strings.Contains(out, s) {
			t.Fatalf("output carries a credential fragment %q:\n%s", s, out)
		}
	}
}

func TestMigrateOnlyAppliesAndReports(t *testing.T) {
	// A fresh handle per run over the SAME fake database: runMigrateOnly closes what it opens.
	_, g := moDB(t, 2)
	open := func(string) (*sql.DB, error) { return sql.OpenDB(ketNoiGia{g: g}), nil }
	var gotUses config.Usage
	gotService := ""
	load := func(s string, u config.Usage) (config.Config, error) {
		gotService, gotUses = s, u
		return loadWith(testDSN())(s, u)
	}

	var out bytes.Buffer
	if code := runMigrateOnly(&out, "platform", nguonBa(), load, open); code != ExitMigrateOK {
		t.Fatalf("exit %d, want %d; output:\n%s", code, ExitMigrateOK, out.String())
	}
	want := []string{
		"migrate-only: platform start",
		"migrate-only: platform applied 0001_init.sql",
		"migrate-only: platform applied 0002_them.sql",
		"migrate-only: platform applied 0003_them_2.sql",
		"migrate-only: platform ok applied=3 skipped=0",
	}
	if got := outputLines(&out); !bangNhau(got, want) {
		t.Fatalf("output\n%v\nwant\n%v", got, want)
	}
	// An empty group set: a pod without the server's Redis/gRPC variables must still migrate.
	if gotService != "platform" || gotUses != config.Uses() {
		t.Fatalf("Load(%q, %v), want Load(\"platform\", config.Uses())", gotService, gotUses)
	}
	assertNoCredential(t, out.String())

	// Second run: everything is in the ledger, skipped silently.
	out.Reset()
	if code := runMigrateOnly(&out, "platform", nguonBa(), load, open); code != ExitMigrateOK {
		t.Fatalf("rerun exit %d; output:\n%s", code, out.String())
	}
	want = []string{"migrate-only: platform start", "migrate-only: platform ok applied=0 skipped=3"}
	if got := outputLines(&out); !bangNhau(got, want) {
		t.Fatalf("rerun output\n%v\nwant\n%v", got, want)
	}
}

func TestMigrateOnlyChecksumMismatchFailsClosed(t *testing.T) {
	db, g := moDB(t, 2)
	g.datTienDo(hangTienDo{ten: "0001_init.sql", checksum: bam("old body of 0001")})
	g.coBangTienDo, g.soBangKhac = true, 1

	var out bytes.Buffer
	code := runMigrateOnly(&out, "identity", nguonBa(), loadWith(testDSN()),
		func(string) (*sql.DB, error) { return db, nil })
	if code != ExitMigrateFailed {
		t.Fatalf("exit %d, want %d; output:\n%s", code, ExitMigrateFailed, out.String())
	}
	lines := outputLines(&out)
	if lines[0] != "migrate-only: identity start" {
		t.Fatalf("first line %q", lines[0])
	}
	last := lines[len(lines)-1]
	if !strings.HasPrefix(last, "migrate-only: identity FAILED: ") || !strings.Contains(last, "0001_init.sql") {
		t.Fatalf("verdict line must say FAILED and name the file: %q", last)
	}
	if len(lines) != 2 {
		t.Fatalf("nothing may be applied after a mismatch; output:\n%s", out.String())
	}
	for _, ma := range []string{"0002", "0003"} {
		if n := g.dem("CREATE TABLE bang_" + ma); n != 0 {
			t.Fatalf("%s applied %d times after a checksum mismatch", ma, n)
		}
	}
}

// Tables with no ledger: Chay would re-apply from 0001 and IF NOT EXISTS would adopt the schema
// silently. The owner rejected that (14/10/2026), so this mode stops with nothing applied.
func TestMigrateOnlyTablesWithoutLedgerStops(t *testing.T) {
	db, g := moDB(t, 2)
	g.soBangKhac = 5

	var out bytes.Buffer
	code := runMigrateOnly(&out, "documents", nguonBa(), loadWith(testDSN()),
		func(string) (*sql.DB, error) { return db, nil })
	if code != ExitMigrateFailed {
		t.Fatalf("exit %d, want %d; output:\n%s", code, ExitMigrateFailed, out.String())
	}
	want := []string{
		"migrate-only: documents start",
		"migrate-only: documents FAILED: database has 5 tables but no schema_migration ledger — " +
			"not adopting; a person decides (ADR 0013)",
	}
	if got := outputLines(&out); !bangNhau(got, want) {
		t.Fatalf("output\n%v\nwant\n%v", got, want)
	}
	// Nothing touched: no lock, no ledger created, no file run.
	if n := g.dem("CREATE TABLE"); n != 0 {
		t.Fatalf("%d CREATE TABLE statements ran; nothing may run", n)
	}
	if n := g.dem("pg_advisory_lock"); n != 0 {
		t.Fatalf("migration lock taken %d times; Chay must not be reached", n)
	}
}

func TestMigrateOnlyLedgerPresentProceeds(t *testing.T) {
	db, g := moDB(t, 2)
	g.datTienDo(hangTienDo{ten: "0001_init.sql", checksum: bam(than("0001"))})
	g.coBangTienDo, g.soBangKhac = true, 2

	var out bytes.Buffer
	code := runMigrateOnly(&out, "documents", nguonBa(), loadWith(testDSN()),
		func(string) (*sql.DB, error) { return db, nil })
	if code != ExitMigrateOK {
		t.Fatalf("exit %d; output:\n%s", code, out.String())
	}
	want := []string{
		"migrate-only: documents start",
		"migrate-only: documents applied 0002_them.sql",
		"migrate-only: documents applied 0003_them_2.sql",
		"migrate-only: documents ok applied=2 skipped=1",
	}
	if got := outputLines(&out); !bangNhau(got, want) {
		t.Fatalf("output\n%v\nwant\n%v", got, want)
	}
}

func TestMigrateOnlyConfigErrorExits2(t *testing.T) {
	cases := map[string]map[string]string{
		"DATABASE_DSN missing": {"ENV": "dev", "DATABASE_DSN": ""},
		"ENV invalid":          {"ENV": "nonsense", "DATABASE_DSN": testDSN()},
	}
	for name, env := range cases {
		t.Run(name, func(t *testing.T) {
			for k, v := range env {
				t.Setenv(k, v)
			}
			opened := false
			var out bytes.Buffer
			code := runMigrateOnly(&out, "comms", nguonBa(), config.Load,
				func(string) (*sql.DB, error) { opened = true; return nil, errors.New("must not open") })
			if code != ExitMigrateConfig {
				t.Fatalf("exit %d, want %d; output:\n%s", code, ExitMigrateConfig, out.String())
			}
			if opened {
				t.Fatal("a configuration error must stop before the database is opened")
			}
			// config.Load's error carries a multi-line operator hint; the verdict is the line
			// right after the marker, the hint follows it.
			lines := outputLines(&out)
			if len(lines) < 2 || lines[0] != "migrate-only: comms start" ||
				!strings.HasPrefix(lines[1], "migrate-only: comms FAILED: configuration: ") {
				t.Fatalf("output:\n%s", out.String())
			}
			assertNoCredential(t, out.String())
		})
	}
}

func TestMigrateOnlyMissingServiceOrFilesExits2(t *testing.T) {
	var out bytes.Buffer
	if code := runMigrateOnly(&out, " ", nguonBa(), loadWith(testDSN()), nil); code != ExitMigrateConfig {
		t.Fatalf("blank service: exit %d", code)
	}
	out.Reset()
	if code := runMigrateOnly(&out, "finance", nil, loadWith(testDSN()), nil); code != ExitMigrateConfig {
		t.Fatalf("nil files: exit %d", code)
	}
}

func TestMigrateOnlyOpenErrorIsRedacted(t *testing.T) {
	var out bytes.Buffer
	code := runMigrateOnly(&out, "documents", nguonBa(), loadWith(testDSN()),
		func(dsn string) (*sql.DB, error) { return nil, errors.New("cannot parse `" + dsn + "`") })
	if code != ExitMigrateFailed {
		t.Fatalf("exit %d, want %d", code, ExitMigrateFailed)
	}
	if !strings.Contains(out.String(), "migrate-only: documents FAILED: open database: ") {
		t.Fatalf("output:\n%s", out.String())
	}
	assertNoCredential(t, out.String())
}

// echoConnector fails every connection with an error that repeats the DSN in every shape a
// driver might: verbatim, password decoded, and keyword form.
type echoConnector struct{ dsn string }

func (c echoConnector) Connect(context.Context) (driver.Conn, error) {
	return nil, errors.New("dial " + c.dsn + " (password Sentinel/pw-7f3a) password=kwSecret-91 refused")
}
func (c echoConnector) Driver() driver.Driver { return trinhGia{} }

func TestMigrateOnlyPingErrorIsRedacted(t *testing.T) {
	var out bytes.Buffer
	code := runMigrateOnly(&out, "petitions", nguonBa(), loadWith(testDSN()),
		func(dsn string) (*sql.DB, error) { return sql.OpenDB(echoConnector{dsn: dsn}), nil })
	if code != ExitMigrateFailed {
		t.Fatalf("exit %d, want %d", code, ExitMigrateFailed)
	}
	if !strings.Contains(out.String(), "migrate-only: petitions FAILED: cannot reach the database: ") {
		t.Fatalf("output:\n%s", out.String())
	}
	assertNoCredential(t, out.String())
}

func TestRedactErrorKeywordDSN(t *testing.T) {
	dsn := "host=db.invalid user=mig password='kw Secret 91' dbname=vigov"
	got := redactError(errors.New("connect: "+dsn+" / kw Secret 91"), dsn)
	if strings.Contains(got, "kw Secret 91") || strings.Contains(got, dsn) {
		t.Fatalf("keyword password leaked: %q", got)
	}
}
