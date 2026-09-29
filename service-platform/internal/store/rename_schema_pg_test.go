package store

// Migration 0012 (ADR 0061 layer B) against a real PostgreSQL — skipped without VIGOV_TEST_DSN
// (openTestDB), like every *_pg_test.go here.
//
// What it proves, in one run on one schema that holds rows the whole time:
//   - forward: every read of THIS image works on the renamed schema, and every statement the
//     PREVIOUS image runs at runtime still works too (the rolling-update aliases);
//   - reverse: the catalogue is EXACTLY what 0001–0011 left — names, definitions, function bodies,
//     comments — and no row changed;
//   - forward again: the catalogue is exactly what the first forward produced.
//   - in every state, each blocking trigger and CHECK still refuses, and refuses FOR ITS OWN REASON
//     (its own message or constraint name), so a statement failing on a missing column cannot pass
//     as a refusal (ADR 0061 §Cạm bẫy).

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/vihat/vigov/core/migrate"
	corestore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-platform/migrations"
)

// previousImage holds the statements the image BEFORE 0012 runs at runtime, verbatim from
// store/directory.go, mini_app.go, commune_profile.go and citizen_report_field.go at commit
// d36862a. Old pods keep running them during the rollout; 0012's aliases exist for exactly these.
var previousImage = struct{ byHost, byID, miniApp, profile, fields string }{
	byHost: `
		SELECT t.id, d.host, t.ten, t.tinh_thanh, t.dang_hoat_dong
		FROM tenant_domain d
		JOIN tenant t ON t.id = d.tenant_id
		WHERE d.host = $1`,
	byID: `
		SELECT t.id, COALESCE(d.host, ''), t.ten, t.tinh_thanh, t.dang_hoat_dong
		FROM tenant t
		LEFT JOIN tenant_domain d ON d.tenant_id = t.id AND d.la_chinh
		WHERE t.id = $1`,
	miniApp: `
	SELECT m.app_id, m.che_do,
	       COALESCE(t.id, ''), COALESCE(t.ten, ''), COALESCE(t.tinh_thanh, ''),
	       COALESCE(t.dang_hoat_dong, false)
	FROM mini_app m
	LEFT JOIN tenant t ON t.id = m.tenant_id
	WHERE m.app_id = $1 AND m.dang_hoat_dong AND m.deleted_at IS NULL`,
	// core/store.Scoped.Query's shape: SELECT <cols> FROM <table> WHERE tenant_id = $1 <tail>.
	profile: `SELECT dia_chi_tru_so, COALESCE(logo_url, ''), duong_day_nong, gio_lam_viec_hien_thi, gioi_thieu ` +
		`FROM ho_so_hien_thi_xa WHERE tenant_id = $1 AND deleted_at IS NULL`,
	fields: `
	SELECT code, default_label, sort_order, icon, tone, active
	FROM petition_field
	ORDER BY sort_order, code`,
}

const renamePgProvince = "Thành phố Đà Nẵng"

// migrationsBefore is the service's real migrations up to, not including, name.
func migrationsBefore(t *testing.T, name string) fs.FS {
	t.Helper()
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		t.Fatalf("read migrations: %v", err)
	}
	out := fstest.MapFS{}
	for _, m := range entries {
		if !strings.HasSuffix(m.Name(), ".sql") || m.Name() >= name {
			continue
		}
		b, err := fs.ReadFile(migrations.FS, m.Name())
		if err != nil {
			t.Fatalf("read %s: %v", m.Name(), err)
		}
		out[m.Name()] = &fstest.MapFile{Data: b}
	}
	return out
}

// seedBeforeRename writes, in the pre-0012 names, one row of every renamed table — the state a
// deployed environment is in on the day 0012 reaches it.
func seedBeforeRename(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO tenant (id, ten, tinh_thanh, dang_hoat_dong) VALUES ($1, 'Xã Thăng Bình', $3, true), ($2, 'Xã Cũ', '', false)`,
			[]any{ulidA, ulidB, renamePgProvince}},
		{`INSERT INTO tenant_domain (host, tenant_id, la_chinh) VALUES ('thangbinh.vigov.vn', $1, true), ('cu.vigov.vn', $1, false)`,
			[]any{ulidA}},
		{`INSERT INTO tenant_succession (tu_id, den_id, can_cu, hieu_luc_tu, tao_boi) VALUES ($1, $2, 'NQ 1/2025', '2025-07-01', 'CB-00001')`,
			[]any{ulidB, ulidA}},
		{`INSERT INTO mini_app (app_id, che_do, tenant_id, tao_boi, cap_nhat_boi) VALUES ('7001', 'rieng', $1, 'CB-00001', 'CB-00001'), ('7002', 'chinh', NULL, 'CB-00001', 'CB-00001')`,
			[]any{ulidA}},
		{`INSERT INTO ho_so_hien_thi_xa (tenant_id, dia_chi_tru_so, duong_day_nong, gio_lam_viec_hien_thi, gioi_thieu, tao_boi, cap_nhat_boi)
		  VALUES ($1, 'Trụ sở A', '0900000000', 'Thứ 2 – Thứ 6', 'Giới thiệu', 'CB-00001', 'CB-00001')`,
			[]any{ulidA}},
		{`UPDATE petition_field SET active = false, updated_by = 'system' WHERE code = 'dien'`, nil},
	} {
		if _, err := db.ExecContext(ctx, stmt.sql, stmt.args...); err != nil {
			t.Fatalf("seed before 0012: %v\n%s", err, stmt.sql)
		}
	}
}

// catalogue lists every name and definition in the test schema, one line each, sorted: relations,
// columns (type, NOT NULL, generated, default, comment), constraints (definition, comment),
// indexes, triggers and function bodies. schema_migration is left out — its rows are the point of
// the reverse, not part of the schema it restores.
func catalogue(ctx context.Context, t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.QueryContext(ctx, `
SELECT format('rel %s %s %s', c.relkind, c.relname, coalesce(obj_description(c.oid, 'pg_class'), ''))
FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = current_schema() AND c.relname NOT LIKE 'schema\_migration%'
UNION ALL
SELECT format('col %s.%s %s %s %s %s %s', c.relname, a.attname, format_type(a.atttypid, a.atttypmod),
              a.attnotnull, a.attgenerated, coalesce(pg_get_expr(d.adbin, d.adrelid), ''),
              coalesce(col_description(c.oid, a.attnum), ''))
FROM pg_attribute a
JOIN pg_class c ON c.oid = a.attrelid
JOIN pg_namespace n ON n.oid = c.relnamespace
LEFT JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
WHERE n.nspname = current_schema() AND a.attnum > 0 AND NOT a.attisdropped
  AND c.relkind IN ('r', 'p', 'v') AND c.relname <> 'schema_migration'
UNION ALL
SELECT format('con %s %s %s %s', c.relname, k.conname, pg_get_constraintdef(k.oid),
              coalesce(obj_description(k.oid, 'pg_constraint'), ''))
FROM pg_constraint k
JOIN pg_class c ON c.oid = k.conrelid
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = current_schema() AND c.relname <> 'schema_migration'
UNION ALL
SELECT format('idx %s', pg_get_indexdef(i.indexrelid))
FROM pg_index i
JOIN pg_class c ON c.oid = i.indrelid
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = current_schema() AND c.relname <> 'schema_migration'
UNION ALL
SELECT format('trg %s', pg_get_triggerdef(tg.oid))
FROM pg_trigger tg
JOIN pg_class c ON c.oid = tg.tgrelid
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = current_schema() AND NOT tg.tgisinternal
UNION ALL
SELECT format('fn %s %s', p.proname, p.prosrc)
FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
WHERE n.nspname = current_schema()
ORDER BY 1`)
	if err != nil {
		t.Fatalf("read catalogue: %v", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Err below is what matters
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatalf("scan catalogue: %v", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("catalogue: %v", err)
	}
	return out
}

// catalogueDiff reports the lines present on one side only; "" when the two are identical.
func catalogueDiff(want, got []string) string {
	count := map[string]int{}
	for _, s := range want {
		count[s]++
	}
	for _, s := range got {
		count[s]--
	}
	var b strings.Builder
	n := 0
	for s, c := range count {
		if c == 0 {
			continue
		}
		if n++; n > 40 {
			b.WriteString("  …\n")
			break
		}
		if c > 0 {
			b.WriteString("  missing: " + s + "\n")
		} else {
			b.WriteString("  extra:   " + s + "\n")
		}
	}
	return b.String()
}

// checkCurrentImage reads the seeded rows through THIS image's stores.
func checkCurrentImage(ctx context.Context, t *testing.T, db *sql.DB, state string) {
	t.Helper()
	d := NewDirectory(db)
	got, ok := d.ByHost(ctx, "thangbinh.vigov.vn")
	if !ok || got.ID != tenant.ID(ulidA) || got.Name != "Xã Thăng Bình" || got.Province != renamePgProvince || !got.Active {
		t.Errorf("%s: ByHost = %+v, ok %v", state, got, ok)
	}
	byID, err := d.ByID(ctx, tenant.ID(ulidA))
	if err != nil || byID.Host != "thangbinh.vigov.vn" {
		t.Errorf("%s: ByID = %+v, err %v — the primary host did not come back", state, byID, err)
	}
	app, err := d.MiniApp(ctx, "7001")
	if err != nil || app.Tenant == nil || app.Tenant.ID != ulidA || app.Tenant.Province != renamePgProvince {
		t.Errorf("%s: MiniApp = %+v, err %v", state, app, err)
	}
	profile, err := NewCommuneProfileStore(corestore.New(db)).Read(tenant.Into(ctx, tenant.ID(ulidA)))
	if err != nil || profile.OfficeAddress != "Trụ sở A" || profile.Hotline != "0900000000" ||
		profile.OfficeHoursText != "Thứ 2 – Thứ 6" || profile.Introduction != "Giới thiệu" {
		t.Errorf("%s: commune profile = %+v, err %v", state, profile, err)
	}
	fields, err := NewCitizenReportFieldStore(db).ListCitizenReportFields(ctx)
	if err != nil || len(fields) != 12 {
		t.Fatalf("%s: fields = %d, err %v", state, len(fields), err)
	}
	for _, f := range fields {
		if (f.Code == "dien") == f.IsActive {
			t.Errorf("%s: field %s is_active = %v — the retirement was lost or spread", state, f.Code, f.IsActive)
		}
	}
	var from, to, basis, since string
	if err := db.QueryRowContext(ctx, `SELECT from_tenant_id, to_tenant_id, legal_basis, effective_from::text
		FROM tenant_succession`).Scan(&from, &to, &basis, &since); err != nil ||
		from != ulidB || to != ulidA || basis != "NQ 1/2025" || since != "2025-07-01" {
		t.Errorf("%s: succession = %s → %s (%s, %s), err %v", state, from, to, basis, since, err)
	}
}

// checkPreviousImage runs the previous image's statements: what old pods see during the rollout
// (aliases), and after the reverse (real columns again). Same values either way.
func checkPreviousImage(ctx context.Context, t *testing.T, db *sql.DB, state string) {
	t.Helper()
	var id, host, name, province string
	var active bool
	if err := db.QueryRowContext(ctx, previousImage.byHost, "thangbinh.vigov.vn").
		Scan(&id, &host, &name, &province, &active); err != nil ||
		id != ulidA || name != "Xã Thăng Bình" || province != renamePgProvince || !active {
		t.Errorf("%s: previous image ByHost = %s %s %s %v, err %v", state, id, name, province, active, err)
	}
	if err := db.QueryRowContext(ctx, previousImage.byID, ulidA).
		Scan(&id, &host, &name, &province, &active); err != nil || host != "thangbinh.vigov.vn" {
		t.Errorf("%s: previous image ByID host = %q, err %v", state, host, err)
	}
	var appID, mode string
	if err := db.QueryRowContext(ctx, previousImage.miniApp, "7001").
		Scan(&appID, &mode, &id, &name, &province, &active); err != nil ||
		mode != "rieng" || id != ulidA || !active {
		t.Errorf("%s: previous image MiniApp = %s %s %s %v, err %v", state, appID, mode, id, active, err)
	}
	var address, logo, hotline, hours, intro string
	if err := db.QueryRowContext(ctx, previousImage.profile, ulidA).
		Scan(&address, &logo, &hotline, &hours, &intro); err != nil || address != "Trụ sở A" || hotline != "0900000000" {
		t.Errorf("%s: previous image profile = %q %q, err %v", state, address, hotline, err)
	}
	rows, err := db.QueryContext(ctx, previousImage.fields)
	if err != nil {
		t.Fatalf("%s: previous image fields: %v", state, err)
	}
	defer rows.Close() //nolint:errcheck // rows.Err below is what matters
	n := 0
	for rows.Next() {
		var code, label, icon, tone string
		var order int
		var isActive bool
		if err := rows.Scan(&code, &label, &order, &icon, &tone, &isActive); err != nil {
			t.Fatalf("%s: previous image fields scan: %v", state, err)
		}
		if (code == "dien") == isActive {
			t.Errorf("%s: previous image sees %s active = %v", state, code, isActive)
		}
		n++
	}
	if err := rows.Err(); err != nil || n != 12 {
		t.Errorf("%s: previous image fields = %d, err %v", state, n, err)
	}
}

// schemaNames are the names a guard statement needs, in one state of the schema.
type schemaNames struct {
	succession, field, fieldMessage, reserved, primary string
}

var (
	namesAfter  = schemaNames{`INSERT INTO tenant_succession (from_tenant_id, to_tenant_id, legal_basis, effective_from) VALUES ($1, $2, 'x', '2025-07-02')`, "citizen_report_field", "citizen_report_field: renaming code", "tenant_domain_not_reserved", "is_primary"}
	namesBefore = schemaNames{`INSERT INTO tenant_succession (tu_id, den_id, can_cu, hieu_luc_tu) VALUES ($1, $2, 'x', '2025-07-02')`, "petition_field", "petition_field: renaming code", "tenant_domain_khong_danh_rieng", "la_chinh"}
)

// refusedWith asserts err is a PostgreSQL error whose message contains want — the guard's OWN words.
func refusedWith(t *testing.T, state, what string, err error, want string) {
	t.Helper()
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || !strings.Contains(pg.Message, want) {
		t.Errorf("%s: %s: err = %v, want a refusal saying %q", state, what, err, want)
	}
}

// checkGuards: every blocking trigger and CHECK of the renamed tables still refuses in this state.
func checkGuards(ctx context.Context, t *testing.T, db *sql.DB, state string, n schemaNames) {
	t.Helper()
	// B → A exists, so A → B closes a cycle. The body reads the columns by name: had 0012 not
	// rewritten it, this fails with "record new has no field" — which is why the message is checked.
	_, err := db.ExecContext(ctx, n.succession, ulidA, ulidB)
	refusedWith(t, state, "succession cycle", err, "vong lap ke thua")

	_, err = db.ExecContext(ctx, `UPDATE `+n.field+` SET code = 'khac-2' WHERE code = 'khac'`)
	refusedWith(t, state, "field code rename", err, n.fieldMessage)

	_, err = db.ExecContext(ctx, `INSERT INTO tenant_domain (host, tenant_id, `+n.primary+`) VALUES ('www.vigov.vn', $1, false)`, ulidA)
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || pg.Code != "23514" || pg.ConstraintName != n.reserved {
		t.Errorf("%s: reserved host: err = %v, want a violation of %s", state, err, n.reserved)
	}

	// mini_app keeps its name in both states; the hard-delete trigger must follow its function's
	// rename (platform_cam_xoa_cung ↔ platform_no_hard_delete).
	_, err = db.ExecContext(ctx, `DELETE FROM mini_app WHERE app_id = '7002'`)
	refusedWith(t, state, "mini_app hard delete", err, "hard delete refused")
}

// checkNotNullNames: on PostgreSQL 18+, where NOT NULL constraints are catalogued with names, every
// one on a renamed table carries `<table>_<column>_not_null` under the table and column names of
// this state. The catalogue comparison cannot see a loop that renamed nothing — before and after
// the reverse would then agree anyway — so this asserts the names themselves. On 13–17 it counts 0.
func checkNotNullNames(ctx context.Context, t *testing.T, db *sql.DB, state string, tables []string) {
	t.Helper()
	var version int
	if err := db.QueryRowContext(ctx, `SELECT current_setting('server_version_num')::int`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	var total, wrong int
	var sample string
	err := db.QueryRowContext(ctx, `
		SELECT count(*),
		       count(*) FILTER (WHERE k.conname <> cl.relname || '_' || a.attname || '_not_null'),
		       coalesce(min(k.conname) FILTER (WHERE k.conname <> cl.relname || '_' || a.attname || '_not_null'), '')
		FROM pg_constraint k
		JOIN pg_class cl ON cl.oid = k.conrelid
		JOIN pg_namespace n ON n.oid = cl.relnamespace
		JOIN pg_attribute a ON a.attrelid = k.conrelid AND a.attnum = k.conkey[1]
		WHERE k.contype = 'n' AND n.nspname = current_schema() AND cl.relname = ANY($1)`,
		tables).Scan(&total, &wrong, &sample)
	if err != nil {
		t.Fatalf("%s: read NOT NULL constraints: %v", state, err)
	}
	if version >= 180000 && total == 0 {
		t.Errorf("%s: PostgreSQL %d catalogues no NOT NULL constraint on %v — the check reads nothing", state, version, tables)
	}
	if wrong != 0 {
		t.Errorf("%s: %d NOT NULL constraint(s) keep a stale name, e.g. %s", state, wrong, sample)
	}
}

var (
	tablesAfter  = []string{"tenant", "tenant_domain", "tenant_succession", "province", "mini_app", "commune_profile", "citizen_report_field"}
	tablesBefore = []string{"tenant", "tenant_domain", "tenant_succession", "tinh_thanh", "mini_app", "ho_so_hien_thi_xa", "petition_field"}
)

func TestPgRenameForwardReverseForward(t *testing.T) {
	db, _ := openTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// 0. The schema before 0012, with rows, in its own names.
	if _, err := migrate.Chay(ctx, db, migrationsBefore(t, renameMigration), "platform"); err != nil {
		t.Fatalf("apply up to 0012: %v", err)
	}
	seedBeforeRename(ctx, t, db)
	before := catalogue(ctx, t, db)
	checkPreviousImage(ctx, t, db, "before 0012")
	checkGuards(ctx, t, db, "before 0012", namesBefore)

	// 1. Forward, through the real runner.
	runMigrations(t, db)
	after := catalogue(ctx, t, db)
	checkCurrentImage(ctx, t, db, "after 0012")
	checkPreviousImage(ctx, t, db, "after 0012 (rolling-update aliases)")
	checkGuards(ctx, t, db, "after 0012", namesAfter)
	checkNotNullNames(ctx, t, db, "after 0012", tablesAfter)
	_, err := db.ExecContext(ctx, `DELETE FROM commune_profile WHERE tenant_id = $1`, ulidA)
	refusedWith(t, "after 0012", "commune_profile hard delete", err, "hard delete refused")

	// 2. Reverse, as an operator runs it: the whole file, which opens and commits its own transaction.
	reverse, err := os.ReadFile(renameReversePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, string(reverse)); err != nil {
		t.Fatalf("reverse: %v", err)
	}
	if diff := catalogueDiff(before, catalogue(ctx, t, db)); diff != "" {
		t.Errorf("reverse did not restore the pre-0012 catalogue:\n%s", diff)
	}
	var left int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM schema_migration WHERE ten = $1`, renameMigration).Scan(&left); err != nil || left != 0 {
		t.Errorf("reverse left %d progress rows for %s (err %v)", left, renameMigration, err)
	}
	checkPreviousImage(ctx, t, db, "after reverse")
	checkGuards(ctx, t, db, "after reverse", namesBefore)
	checkNotNullNames(ctx, t, db, "after reverse", tablesBefore)

	// A second reverse must refuse: 0012 is no longer applied.
	//
	// ON ONE PINNED CONNECTION, with the ROLLBACK on that same connection. The failed script leaves
	// its BEGIN open and aborted; returned to the pool in that state, pgx's ResetSession discards the
	// connection, the next statement opens a fresh one whose search_path is `public`, and the second
	// forward below lands in the wrong schema. Measured on the first real run, 2026-09-29.
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, string(reverse)); err == nil {
		t.Error("reverse ran twice")
	}
	if _, err := conn.ExecContext(ctx, `ROLLBACK`); err != nil {
		t.Fatalf("close the refused reverse's transaction: %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	var schemaNow string
	if err := db.QueryRowContext(ctx, `SELECT current_schema()`).Scan(&schemaNow); err != nil ||
		!strings.HasPrefix(schemaNow, "vigov_test_") {
		t.Fatalf("after the refused reverse the session is on schema %q (err %v) — later steps would test the wrong schema", schemaNow, err)
	}

	// 3. Forward again.
	runMigrations(t, db)
	if diff := catalogueDiff(after, catalogue(ctx, t, db)); diff != "" {
		t.Errorf("re-applying 0012 did not reproduce the first forward's catalogue:\n%s", diff)
	}
	checkCurrentImage(ctx, t, db, "after 0012 again")
	checkPreviousImage(ctx, t, db, "after 0012 again (aliases)")
	checkGuards(ctx, t, db, "after 0012 again", namesAfter)
	checkNotNullNames(ctx, t, db, "after 0012 again", tablesAfter)
}
