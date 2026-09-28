package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	pkgstore "github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// RestrictedPetitionAuditSubjects, wired into the REAL core/audit.Log as cmd/server wires it, on a
// fake driver that records every statement.
//
// PROVED HERE   the constant passes WithHiddenSubjects' wiring check · a read WITHOUT SeeHidden carries
//               the subquery, verbatim, inside `subject NOT IN (…)`, binding only the commune · a read
//               WITH SeeHidden carries none of it · the subquery is the complement of the list's own
//               restrictedFieldExclusion, not a second spelling.
// NOT PROVED    that the column and table exist, or what PostgreSQL returns — the pg test below.

type ahStmt struct {
	sql  string
	args []driver.Value
}

type ahDB struct{ stmts []ahStmt }

type ahConnector struct{ d *ahDB }

func (c ahConnector) Connect(context.Context) (driver.Conn, error) { return &ahConn{d: c.d}, nil }
func (c ahConnector) Driver() driver.Driver                        { return ahDriver{} }

type ahDriver struct{}

func (ahDriver) Open(string) (driver.Conn, error) { return nil, errors.New("connector only") }

type ahConn struct{ d *ahDB }

func (c *ahConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("no Prepare") }
func (c *ahConn) Close() error                        { return nil }
func (c *ahConn) Begin() (driver.Tx, error)           { return ahTx{}, nil }
func (c *ahConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return ahTx{}, nil
}

func ahVals(args []driver.NamedValue) []driver.Value {
	v := make([]driver.Value, 0, len(args))
	for _, a := range args {
		v = append(v, a.Value)
	}
	return v
}

func (c *ahConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.d.stmts = append(c.d.stmts, ahStmt{sql: q, args: ahVals(args)})
	return ahRows{}, nil // an empty page: the statement is what is under test
}

func (c *ahConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.d.stmts = append(c.d.stmts, ahStmt{sql: q, args: ahVals(args)})
	return driver.RowsAffected(1), nil
}

type ahTx struct{}

func (ahTx) Commit() error   { return nil }
func (ahTx) Rollback() error { return nil }

type ahRows struct{}

func (ahRows) Columns() []string {
	return []string{"id", "at", "actor_kind", "actor_id", "actor_ip", "action", "subject", "delta"}
}
func (ahRows) Close() error              { return nil }
func (ahRows) Next([]driver.Value) error { return io.EOF }

// readWith runs one Log.Read and returns the audit_log SELECT it issued.
func readWith(t *testing.T, seeHidden bool) ahStmt {
	t.Helper()
	d := &ahDB{}
	l := audit.NewLog(pkgstore.New(sql.OpenDB(ahConnector{d: d})),
		audit.WithHiddenSubjects(RestrictedPetitionAuditSubjects))
	if _, err := l.Read(ctxXa(xaThu), audit.Actor{ID: "CB-00123", Kind: "staff", IP: "10.0.0.7"},
		audit.Query{SeeHidden: seeHidden}); err != nil {
		t.Fatalf("Read: %v", err)
	}
	for _, s := range d.stmts {
		if strings.Contains(s.sql, "FROM audit_log") && strings.HasPrefix(strings.TrimSpace(s.sql), "SELECT") {
			return s
		}
	}
	t.Fatalf("no audit_log SELECT among %d statements", len(d.stmts))
	return ahStmt{}
}

func TestRestrictedPetitionAuditSubjects_HiddenWithoutSeeHidden(t *testing.T) {
	s := readWith(t, false)
	if !strings.Contains(s.sql, "subject NOT IN (SELECT h.s FROM ("+RestrictedPetitionAuditSubjects+") AS h(s)") {
		t.Errorf("restricted-field subquery missing from the read:\n%s", s.sql)
	}
	// The subquery binds nothing of its own: $1 is the commune ScopedTx.Query already bound.
	if len(s.args) == 0 || s.args[0] != string(xaThu) {
		t.Errorf("$1 = %v, want the context's commune %q", s.args, xaThu)
	}
}

// A READ ENTRY NEVER ECHOES A CAN-BO LOOKUP CODE (ADR 0030, ADR 0054 §4): the read withholds the
// read entries whose delta names a restricted petition, and the entry this read writes records only
// that a subject was searched.
func TestRestrictedPetitionAuditSubjects_ReadEntriesDoNotEchoTheSubject(t *testing.T) {
	s := readWith(t, false)
	if !strings.Contains(s.sql, "OR delta->>'subject' NOT IN (SELECT h.s FROM ("+RestrictedPetitionAuditSubjects+") AS h(s)") {
		t.Errorf("read entries naming a restricted petition are not withheld:\n%s", s.sql)
	}

	d := &ahDB{}
	l := audit.NewLog(pkgstore.New(sql.OpenDB(ahConnector{d: d})),
		audit.WithHiddenSubjects(RestrictedPetitionAuditSubjects))
	if _, err := l.Read(ctxXa(xaThu), audit.Actor{ID: "CB-00123", Kind: "staff", IP: "10.0.0.7"},
		audit.Query{Subject: "PA-AH1-0000-0001", SeeHidden: true}); err != nil {
		t.Fatalf("Read: %v", err)
	}
	var wrote bool
	for _, st := range d.stmts {
		if !strings.Contains(st.sql, "INSERT INTO audit_log") {
			continue
		}
		wrote = true
		b, _ := st.args[len(st.args)-1].([]byte)
		if strings.Contains(string(b), "PA-AH1-0000-0001") || !strings.Contains(string(b), `"subject_filtered":true`) {
			t.Errorf("read entry delta = %s, want subject_filtered and no lookup code", b)
		}
	}
	if !wrote {
		t.Fatal("the read wrote no entry")
	}
}

func TestRestrictedPetitionAuditSubjects_ShownWithSeeHidden(t *testing.T) {
	s := readWith(t, true)
	if strings.Contains(s.sql, "NOT IN") || strings.Contains(s.sql, "phieu_phan_anh") {
		t.Errorf("SeeHidden still withheld entries:\n%s", s.sql)
	}
}

func TestRestrictedPetitionAuditSubjects_IsTheListConstantsComplement(t *testing.T) {
	// ONE PREDICATE: the list's exclusion, negated — never a second spelling of the field.
	if !strings.Contains(RestrictedPetitionAuditSubjects, "NOT (TRUE"+restrictedFieldExclusion+")") {
		t.Errorf("not built from restrictedFieldExclusion: %s", RestrictedPetitionAuditSubjects)
	}
	if !strings.Contains(restrictedFieldExclusion, "'"+domain.LinhVucHanChe+"'") {
		t.Errorf("restrictedFieldExclusion no longer names %q: %s", domain.LinhVucHanChe, restrictedFieldExclusion)
	}
	// The subject petitions write is the lookup code (internal/app, `Subject: …MaTraCuu`).
	if !strings.HasPrefix(RestrictedPetitionAuditSubjects, "SELECT ma_tra_cuu FROM phieu_phan_anh WHERE tenant_id = $1") {
		t.Errorf("subquery does not select the lookup code for the commune: %s", RestrictedPetitionAuditSubjects)
	}
}

// TestPgRestrictedPetitionAuditSubjects runs the subquery and the full read against the real schema.
// SKIPS without VIGOV_TEST_DSN (the harness is danh_muc_nhiem_vu_pg_test.go), and the package still
// prints `ok` — a green run on such a machine proves nothing about the column names.
func TestPgRestrictedPetitionAuditSubjects(t *testing.T) {
	db := moKetNoi(t)
	commune, otherCommune := xaRieng(t)
	sentAt := time.Now().UTC().Add(-2 * time.Hour)

	for _, p := range []struct{ commune, id, code, field string }{
		{commune, "pa-ah-1", "PA-AH1-0000-0001", domain.LinhVucHanChe}, // restricted → hidden
		{commune, "pa-ah-2", "PA-AH2-0000-0002", "dien"},               // ordinary → shown
		{commune, "pa-ah-3", "PA-AH3-0000-0003", ""},                   // unclassified → shown, as in the list
		{otherCommune, "pa-ah-4", "PA-AH4-0000-0004", domain.LinhVucHanChe},
	} {
		var resolveBy any
		if p.field != "" {
			resolveBy = sentAt.Add(48 * time.Hour)
		}
		if err := themPhieu(db, p.commune, p.id, p.code, "web-xa", p.field, "da-tiep-nhan",
			sentAt, sentAt, sentAt.Add(8*time.Hour), resolveBy); err != nil {
			t.Fatalf("insert %s: %v", p.code, err)
		}
	}

	rows, err := db.Query(RestrictedPetitionAuditSubjects, commune)
	if err != nil {
		t.Fatalf("subquery against the real schema: %v", err)
	}
	var got []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		got = append(got, s)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "PA-AH1-0000-0001" {
		t.Errorf("restricted subjects = %v, want only this commune's can-bo petition", got)
	}

	// End to end: entries written as the app writes them, read through the Log as cmd/server wires it.
	handle := pkgstore.New(db)
	ctx := ctxXa(tenant.ID(commune))
	reader := audit.Actor{ID: "CB-00123", Kind: "staff", IP: "10.0.0.7"}
	if err := handle.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		for _, code := range []string{"PA-AH1-0000-0001", "PA-AH2-0000-0002", "PA-AH3-0000-0003"} {
			if err := audit.Write(ctx, tx, audit.Entry{Actor: reader, Action: "phan_loai_phieu", Subject: code}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("write entries: %v", err)
	}
	l := audit.NewLog(handle, audit.WithHiddenSubjects(RestrictedPetitionAuditSubjects))
	for _, c := range []struct {
		see  bool
		want int
	}{{false, 2}, {true, 3}} {
		res, err := l.Read(ctx, reader, audit.Query{Action: "phan_loai_phieu", SeeHidden: c.see})
		if err != nil {
			t.Fatalf("Read(SeeHidden=%v): %v", c.see, err)
		}
		if len(res.Items) != c.want {
			t.Errorf("SeeHidden=%v: %d entries, want %d", c.see, len(res.Items), c.want)
		}
		for _, e := range res.Items {
			if !c.see && e.Subject == "PA-AH1-0000-0001" {
				t.Error("a can-bo petition's entry reached a reader without feedback.restricted")
			}
		}
	}

	// READ ENTRIES IN THE OLD SHAPE — the searched subject inside the delta — are withheld from a
	// reader without the right when they name a restricted petition, and kept otherwise.
	if err := handle.For(ctx).Tx(ctx, func(tx *pkgstore.ScopedTx) error {
		for _, delta := range []string{
			`{"subject":"PA-AH1-0000-0001","cursor":false,"returned":1,"outcome":"ok"}`,
			`{"subject":"PA-AH2-0000-0002","cursor":false,"returned":1,"outcome":"ok"}`,
			`{"cursor":false,"returned":3,"outcome":"ok"}`,
		} {
			if err := audit.Write(ctx, tx, audit.Entry{Actor: reader, Action: audit.ActionReadLog,
				Subject: reader.ID, Delta: []byte(delta)}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("write old-shape read entries: %v", err)
	}
	for _, c := range []struct {
		see  bool
		minN int
	}{{false, 2}, {true, 3}} {
		res, err := l.Read(ctx, reader, audit.Query{Action: audit.ActionReadLog, SeeHidden: c.see})
		if err != nil {
			t.Fatalf("Read(read entries, SeeHidden=%v): %v", c.see, err)
		}
		restrictedEchoes := 0
		for _, e := range res.Items {
			if raw, ok := e.Delta.(json.RawMessage); ok && strings.Contains(string(raw), "PA-AH1-0000-0001") {
				restrictedEchoes++
			}
		}
		if !c.see && restrictedEchoes != 0 {
			t.Error("a read entry echoing a restricted lookup code reached a reader without feedback.restricted")
		}
		if c.see && restrictedEchoes == 0 {
			t.Error("SeeHidden did not show the read entry naming the restricted petition")
		}
		if len(res.Items) < c.minN {
			t.Errorf("SeeHidden=%v: %d read entries, want at least %d — entries without a hidden subject must stay", c.see, len(res.Items), c.minN)
		}
	}
}
