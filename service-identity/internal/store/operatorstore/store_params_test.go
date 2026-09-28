package operatorstore

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/service-identity/internal/domain"
	"github.com/vihat/vigov/service-identity/migrations"
)

// THE POSITIONAL-BINDING TRAP, operator edition (see internal/store/can_bo_ghi_tham_so_test.go for
// the original argument). `$n` in the SQL text binds by POSITION to the Go argument list, and two
// adjacent TEXT arguments traded over raise nothing: PostgreSQL stores a string in a text column
// either way. The costly pairs here:
//
//	Disable      disabled_by ↔ disabled_reason   the trail's "who locked this operator" holds prose
//	Grant        granted_by ↔ grant_reason        the history of a platform-wide right loses its author
//	Revoke       revoked_by ↔ revoke_reason
//	AppendAudit  actor ↔ subject                  "VH-00001 did X to VH-00002" reads backwards
//	CreateSession created_ip ↔ user_agent
//
// The pg suite could see these, but it SKIPS without VIGOV_TEST_DSN — so these run the REAL store
// methods inside a real *sql.Tx over a recording driver that always runs.

// --- recording driver ------------------------------------------------------------------------------

type recorded struct {
	query string
	args  []driver.Value
}

type recorder struct {
	stmts []recorded
	// rowsAffected answers every Exec; default 1.
	rowsAffected int64
	zeroRows     bool
	// queryRows answers a Query by the first matching substring of its text.
	queryRows map[string][][]driver.Value
}

type recConnector struct{ r *recorder }

func (c recConnector) Connect(context.Context) (driver.Conn, error) { return &recConn{r: c.r}, nil }
func (c recConnector) Driver() driver.Driver                        { return recDriver{} }

type recDriver struct{}

func (recDriver) Open(string) (driver.Conn, error) { return nil, errors.New("connector only") }

type recConn struct{ r *recorder }

func (c *recConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("no Prepare") }
func (c *recConn) Close() error                        { return nil }
func (c *recConn) Begin() (driver.Tx, error)           { return recTx{}, nil }

func values(args []driver.NamedValue) []driver.Value {
	out := make([]driver.Value, 0, len(args))
	for _, a := range args {
		out = append(out, a.Value)
	}
	return out
}

func (c *recConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.r.stmts = append(c.r.stmts, recorded{query: q, args: values(args)})
	return driver.RowsAffected(c.r.rowsAffected), nil
}

func (c *recConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.r.stmts = append(c.r.stmts, recorded{query: q, args: values(args)})
	for key, rows := range c.r.queryRows {
		if strings.Contains(q, key) {
			return &recRows{data: rows}, nil
		}
	}
	return &recRows{}, nil
}

type recTx struct{}

func (recTx) Commit() error   { return nil }
func (recTx) Rollback() error { return nil }

type recRows struct {
	data [][]driver.Value
	i    int
}

func (r *recRows) Columns() []string {
	if len(r.data) == 0 {
		return []string{"c"}
	}
	cols := make([]string, len(r.data[0]))
	for i := range cols {
		cols[i] = fmt.Sprintf("c%d", i)
	}
	return cols
}
func (r *recRows) Close() error { return nil }
func (r *recRows) Next(dest []driver.Value) error {
	if r.i >= len(r.data) {
		return io.EOF
	}
	copy(dest, r.data[r.i])
	r.i++
	return nil
}

// runTx runs fn in one transaction over a fresh recorder and returns what was sent.
func runTx(t *testing.T, r *recorder, fn func(tx *Tx) error) []recorded {
	t.Helper()
	if r.rowsAffected == 0 && !r.zeroRows {
		r.rowsAffected = 1
	}
	db := sql.OpenDB(recConnector{r: r})
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	if err := New(db).InTx(context.Background(), fn); err != nil {
		t.Fatalf("tx: %v", err)
	}
	return r.stmts
}

var reAssign = regexp.MustCompile(`([a-z_]+) = (?:nullif\()?\$(\d+)`)

// assignments maps `column = $n` to the value bound at $n.
func assignments(t *testing.T, s recorded) map[string]driver.Value {
	t.Helper()
	out := map[string]driver.Value{}
	for _, m := range reAssign.FindAllStringSubmatch(strings.Join(strings.Fields(s.query), " "), -1) {
		n, _ := strconv.Atoi(m[2])
		if n < 1 || n > len(s.args) {
			t.Fatalf("placeholder $%d outside %d args: %s", n, len(s.args), s.query)
		}
		out[m[1]] = s.args[n-1]
	}
	return out
}

func splitTop(s string) []string {
	var out []string
	depth, start := 0, 0
	for i, r := range s {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	return append(out, strings.TrimSpace(s[start:]))
}

var reInsert = regexp.MustCompile(`(?s)\((.*?)\)\s*VALUES\s*\((.*)\)\s*$`)
var rePlaceholder = regexp.MustCompile(`\$(\d+)`)

// insertBindings pairs an INSERT's column list with its VALUES list, then each `$n` with its arg.
func insertBindings(t *testing.T, s recorded) map[string]driver.Value {
	t.Helper()
	m := reInsert.FindStringSubmatch(s.query)
	if m == nil {
		t.Fatalf("cannot read INSERT: %s", s.query)
	}
	cols, vals := splitTop(m[1]), splitTop(m[2])
	if len(cols) != len(vals) {
		t.Fatalf("INSERT has %d columns but %d values", len(cols), len(vals))
	}
	out := map[string]driver.Value{}
	for i, c := range cols {
		if p := rePlaceholder.FindStringSubmatch(vals[i]); p != nil {
			n, _ := strconv.Atoi(p[1])
			out[c] = s.args[n-1]
		}
	}
	return out
}

func expect(t *testing.T, what string, got, want map[string]driver.Value) {
	t.Helper()
	for col, v := range want {
		g, ok := got[col]
		if !ok {
			t.Errorf("%s: statement no longer binds %q by placeholder — this check is blind there", what, col)
			continue
		}
		if tv, ok := v.(time.Time); ok {
			if gt, ok := g.(time.Time); !ok || !gt.Equal(tv) {
				t.Errorf("%s: %s = %#v, want %v", what, col, g, tv)
			}
			continue
		}
		if g != v {
			t.Errorf("%s: %s = %#v, want %#v — positional arguments swapped", what, col, g, v)
		}
	}
}

func sha(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

var t0 = time.Date(2026, 9, 28, 9, 30, 0, 0, time.UTC)

const accountID = "01JD9A00000000000000000001"

// findRevokeAll returns the revoke-all-sessions statement, or fails.
func findRevokeAll(t *testing.T, stmts []recorded, reason string) {
	t.Helper()
	for _, s := range stmts {
		if strings.Contains(s.query, "UPDATE operator_session") && strings.Contains(s.query, "operator_account_id = $1") {
			expect(t, "revoke all", assignments(t, s), map[string]driver.Value{
				"operator_account_id": accountID, "revoked_at": t0, "revoked_reason": reason,
			})
			return
		}
	}
	t.Errorf("no revoke-all-sessions statement — the owner's rule (%s revokes every session) is not enforced", reason)
}

// --- tests ---------------------------------------------------------------------------------------

func TestCreateAccountDrawsCodeFromSequence(t *testing.T) {
	r := &recorder{queryRows: map[string][][]driver.Value{"nextval('operator_code_seq')": {{int64(7)}}}}
	var got domain.OperatorAccount
	stmts := runTx(t, r, func(tx *Tx) error {
		var err error
		got, err = tx.CreateAccount(context.Background(), NewAccount{
			Email: "op@example.test", DisplayName: "Operator One", PasswordHash: "hash-value", CreatedBy: "system",
		}, t0)
		return err
	})
	if len(stmts) != 2 {
		t.Fatalf("want 2 statements (nextval, insert), got %d", len(stmts))
	}
	if got.Code != "VH-00007" || len(got.ID) != 26 || !got.MustChangePassword {
		t.Errorf("account = %+v", got)
	}
	expect(t, "CreateAccount", insertBindings(t, stmts[1]), map[string]driver.Value{
		"id": got.ID, "code": "VH-00007", "email": "op@example.test", "display_name": "Operator One",
		"password_hash": "hash-value", "created_by": "system", "created_at": t0, "updated_at": t0,
	})
}

func TestCreateAccountRefusesBeforeTouchingTheDatabase(t *testing.T) {
	for name, in := range map[string]NewAccount{
		"no email":          {DisplayName: "x", PasswordHash: "h", CreatedBy: "system"},
		"no name":           {Email: "a@example.test", PasswordHash: "h", CreatedBy: "system"},
		"no hash":           {Email: "a@example.test", DisplayName: "x", CreatedBy: "system"},
		"internal id actor": {Email: "a@example.test", DisplayName: "x", PasswordHash: "h", CreatedBy: accountID},
		"empty actor":       {Email: "a@example.test", DisplayName: "x", PasswordHash: "h"},
	} {
		r := &recorder{}
		db := sql.OpenDB(recConnector{r: r})
		err := New(db).InTx(context.Background(), func(tx *Tx) error {
			_, err := tx.CreateAccount(context.Background(), in, t0)
			return err
		})
		db.Close()
		if err == nil {
			t.Errorf("%s: accepted", name)
		}
		if len(r.stmts) != 0 {
			t.Errorf("%s: %d statements sent before refusing", name, len(r.stmts))
		}
	}
}

func TestDisableBindsWhoAndWhyAndRevokesSessions(t *testing.T) {
	stmts := runTx(t, &recorder{}, func(tx *Tx) error {
		return tx.Disable(context.Background(), accountID, "system", "ticket OPS-42: left the company", t0)
	})
	expect(t, "Disable", assignments(t, stmts[0]), map[string]driver.Value{
		"id": accountID, "disabled_at": t0, "disabled_by": "system",
		"disabled_reason": "ticket OPS-42: left the company",
	})
	findRevokeAll(t, stmts, RevokeReasonDisabled)
}

func TestGrantBindsAndRevokesSessions(t *testing.T) {
	stmts := runTx(t, &recorder{}, func(tx *Tx) error {
		return tx.Grant(context.Background(), accountID, domain.OperatorPermissionDomainManage,
			"system", "ticket OPS-7", t0)
	})
	b := insertBindings(t, stmts[0])
	expect(t, "Grant", b, map[string]driver.Value{
		"operator_account_id": accountID, "permission_key": "ops.domain.manage",
		"granted_at": t0, "granted_by": "system", "grant_reason": "ticket OPS-7",
	})
	findRevokeAll(t, stmts, RevokeReasonGrantChanged)
}

func TestRevokeBindsAndRevokesSessions(t *testing.T) {
	stmts := runTx(t, &recorder{}, func(tx *Tx) error {
		return tx.Revoke(context.Background(), accountID, domain.OperatorPermissionQRIssue,
			"VH-00003", "ticket OPS-8", t0)
	})
	expect(t, "Revoke", assignments(t, stmts[0]), map[string]driver.Value{
		"operator_account_id": accountID, "permission_key": "ops.qr.issue",
		"revoked_at": t0, "revoked_by": "VH-00003", "revoke_reason": "ticket OPS-8",
	})
	findRevokeAll(t, stmts, RevokeReasonGrantChanged)
}

func TestGrantRefusesOffListKeyAndBadActor(t *testing.T) {
	for name, fn := range map[string]func(tx *Tx) error{
		"commune key": func(tx *Tx) error {
			return tx.Grant(context.Background(), accountID, "task.extend", "system", "r", t0)
		},
		"internal id actor": func(tx *Tx) error {
			return tx.Grant(context.Background(), accountID, domain.OperatorPermissionQRIssue, accountID, "r", t0)
		},
		"no reason": func(tx *Tx) error {
			return tx.Revoke(context.Background(), accountID, domain.OperatorPermissionQRIssue, "system", " ", t0)
		},
	} {
		r := &recorder{}
		db := sql.OpenDB(recConnector{r: r})
		if err := New(db).InTx(context.Background(), fn); err == nil {
			t.Errorf("%s: accepted", name)
		}
		db.Close()
		if len(r.stmts) != 0 {
			t.Errorf("%s: statements sent before refusing", name)
		}
	}
}

func TestPasswordChangeAndMFAResetRevokeSessions(t *testing.T) {
	stmts := runTx(t, &recorder{}, func(tx *Tx) error {
		return tx.SetPassword(context.Background(), accountID, "new-hash", false, t0)
	})
	expect(t, "SetPassword", assignments(t, stmts[0]), map[string]driver.Value{
		"id": accountID, "password_hash": "new-hash", "must_change_password": false, "updated_at": t0,
	})
	findRevokeAll(t, stmts, RevokeReasonPasswordChanged)

	stmts = runTx(t, &recorder{}, func(tx *Tx) error {
		return tx.ActivateTOTP(context.Background(), accountID, "own-hash", 59000000, t0)
	})
	expect(t, "ActivateTOTP", assignments(t, stmts[0]), map[string]driver.Value{
		"id": accountID, "totp_enrolled_at": t0, "totp_last_step": int64(59000000), "password_hash": "own-hash",
	})
	findRevokeAll(t, stmts, RevokeReasonPasswordChanged)

	stmts = runTx(t, &recorder{}, func(tx *Tx) error { return tx.ResetMFA(context.Background(), accountID, t0) })
	var voided bool
	for _, s := range stmts {
		if strings.Contains(s.query, "UPDATE operator_recovery_code") && strings.Contains(s.query, "voided_at") {
			voided = true
		}
	}
	if !voided {
		t.Error("ResetMFA does not void live recovery codes — a reset leaves a working second factor")
	}
	findRevokeAll(t, stmts, RevokeReasonMFAReset)
}

func TestSessionStoresHashNeverRawSid(t *testing.T) {
	const sid = "raw-session-id-value"
	var exp time.Time
	stmts := runTx(t, &recorder{}, func(tx *Tx) error {
		var err error
		exp, err = tx.CreateSession(context.Background(), sid, accountID, "203.0.113.5", "UA/1.0", t0)
		return err
	})
	b := insertBindings(t, stmts[0])
	expect(t, "CreateSession", b, map[string]driver.Value{
		"id": sha(sid), "operator_account_id": accountID, "created_at": t0,
		"expires_at": t0.Add(8 * time.Hour), "created_ip": "203.0.113.5", "user_agent": "UA/1.0",
	})
	if !exp.Equal(t0.Add(domain.SessionLifetime)) {
		t.Errorf("expiry = %v, want absolute 8h", exp)
	}
	for _, s := range stmts {
		for _, a := range s.args {
			if a == sid {
				t.Fatal("the raw sid reached a statement")
			}
		}
	}

	stmts = runTx(t, &recorder{}, func(tx *Tx) error {
		return tx.RevokeSession(context.Background(), sid, "logout", t0)
	})
	expect(t, "RevokeSession", assignments(t, stmts[0]), map[string]driver.Value{
		"id": sha(sid), "revoked_at": t0, "revoked_reason": "logout",
	})

	stmts = runTx(t, &recorder{}, func(tx *Tx) error { return tx.TouchSession(context.Background(), sid, t0) })
	if strings.Contains(stmts[0].query, "expires_at =") {
		t.Error("TouchSession writes expires_at — the lifetime must stay absolute")
	}
}

func TestCheckSessionPredicates(t *testing.T) {
	q := strings.Join(strings.Fields(checkSession), " ")
	for _, want := range []string{"s.id = $1", "s.revoked_at IS NULL", "s.expires_at > $2", "a.disabled_at IS NULL"} {
		if !strings.Contains(q, want) {
			t.Errorf("checkSession lacks %q: %s", want, q)
		}
	}
}

func TestRecordTOTPStepIsStrictlyGreater(t *testing.T) {
	q := strings.Join(strings.Fields(recordTOTPStep), " ")
	if !strings.Contains(q, "totp_last_step < $2") {
		t.Errorf("replay guard is not a strict compare: %s", q)
	}
	stmts := runTx(t, &recorder{}, func(tx *Tx) error {
		ok, err := tx.RecordTOTPStep(context.Background(), accountID, 100)
		if !ok {
			t.Error("step reported as replay with 1 row affected")
		}
		return err
	})
	if stmts[0].args[1] != int64(100) {
		t.Errorf("step bound as %#v", stmts[0].args[1])
	}

	// Zero rows affected = the step was not later than the last accepted one = a replay.
	runTx(t, &recorder{zeroRows: true}, func(tx *Tx) error {
		ok, err := tx.RecordTOTPStep(context.Background(), accountID, 100)
		if ok {
			t.Error("a step the database refused was reported as accepted")
		}
		return err
	})
}

func TestRegisterFailureBindsThresholdAndLockEnd(t *testing.T) {
	r := &recorder{queryRows: map[string][][]driver.Value{"RETURNING failed_attempts": {{int64(0)}}}}
	var locked bool
	var until time.Time
	stmts := runTx(t, r, func(tx *Tx) error {
		var err error
		locked, until, err = tx.RegisterFailure(context.Background(), accountID, t0)
		return err
	})
	a := stmts[0].args
	if a[0] != accountID || !a[1].(time.Time).Equal(t0) || a[2] != int64(domain.MaxFailedAttempts) ||
		!a[3].(time.Time).Equal(t0.Add(15*time.Minute)) {
		t.Errorf("RegisterFailure args = %#v", a)
	}
	if !locked || !until.Equal(t0.Add(15*time.Minute)) {
		t.Errorf("count 0 after update must mean 'this failure locked': locked=%v until=%v", locked, until)
	}

	r = &recorder{queryRows: map[string][][]driver.Value{"RETURNING failed_attempts": {{int64(3)}}}}
	runTx(t, r, func(tx *Tx) error {
		var err error
		locked, _, err = tx.RegisterFailure(context.Background(), accountID, t0)
		return err
	})
	if locked {
		t.Error("third failure reported as a lock")
	}
}

func TestRecoveryCodesHashedAndOldBatchVoided(t *testing.T) {
	codes := make([][]byte, domain.RecoveryCodeCount)
	for i := range codes {
		sum := sha256.Sum256([]byte(fmt.Sprintf("CODE%02d", i)))
		codes[i] = sum[:]
	}
	stmts := runTx(t, &recorder{}, func(tx *Tx) error {
		_, err := tx.ReplaceRecoveryCodes(context.Background(), accountID, codes, t0)
		return err
	})
	if len(stmts) != 1+domain.RecoveryCodeCount {
		t.Fatalf("want 1 void + %d inserts, got %d", domain.RecoveryCodeCount, len(stmts))
	}
	if !strings.Contains(stmts[0].query, "voided_at") {
		t.Error("the old batch is not voided FIRST")
	}
	for i, s := range stmts[1:] {
		b := insertBindings(t, s)
		// Stored as the hex of the digest it was given — NOT hashed a second time.
		if b["code_hash"] != hex.EncodeToString(codes[i]) {
			t.Errorf("code %d stored as %#v, want the hex of its digest", i, b["code_hash"])
		}
	}

	for name, set := range map[string][][]byte{
		"too few":   codes[:9],
		"duplicate": append(append([][]byte{}, codes[:9]...), codes[0]),
		"plaintext": append(append([][]byte{}, codes[:9]...), []byte("ABCD-EFGH-IJKL-MNOP")),
	} {
		r := &recorder{}
		db := sql.OpenDB(recConnector{r: r})
		err := New(db).InTx(context.Background(), func(tx *Tx) error {
			_, err := tx.ReplaceRecoveryCodes(context.Background(), accountID, set, t0)
			return err
		})
		db.Close()
		if !errors.Is(err, ErrRecoveryCodeSet) || len(r.stmts) != 0 {
			t.Errorf("%s: err=%v, statements=%d", name, err, len(r.stmts))
		}
	}

	stmts = runTx(t, &recorder{}, func(tx *Tx) error {
		_, err := tx.UseRecoveryCode(context.Background(), accountID, codes[3], t0)
		return err
	})
	expect(t, "UseRecoveryCode", assignments(t, stmts[0]), map[string]driver.Value{
		"operator_account_id": accountID, "code_hash": hex.EncodeToString(codes[3]), "used_at": t0,
	})
}

func TestAppendAuditBindsEveryColumn(t *testing.T) {
	stmts := runTx(t, &recorder{}, func(tx *Tx) error {
		return tx.AppendAudit(context.Background(), domain.OperatorAuditEntry{
			OccurredAt: t0, Actor: "VH-00001", ActorIP: "203.0.113.9",
			Action: domain.OperatorAuditPermissionGranted, Subject: "VH-00002",
			After: map[string]any{"permission_key": "ops.qr.issue"}, Reason: "ticket OPS-9",
		})
	})
	b := insertBindings(t, stmts[0])
	expect(t, "AppendAudit", b, map[string]driver.Value{
		"actor": "VH-00001", "actor_ip": "203.0.113.9", "action": "operator.permission_granted",
		"subject": "VH-00002", "after": `{"permission_key":"ops.qr.issue"}`, "reason": "ticket OPS-9",
	})
	if b["before"] != nil {
		t.Errorf("empty before stored as %#v, want NULL", b["before"])
	}
}

func TestAppendAuditRefusesWithoutWriting(t *testing.T) {
	r := &recorder{}
	db := sql.OpenDB(recConnector{r: r})
	defer db.Close()
	err := New(db).InTx(context.Background(), func(tx *Tx) error {
		return tx.AppendAudit(context.Background(), domain.OperatorAuditEntry{
			Actor: accountID, Action: domain.OperatorAuditLoggedOut, Subject: "VH-00001",
		})
	})
	if !errors.Is(err, domain.ErrOperatorAuditActor) || len(r.stmts) != 0 {
		t.Errorf("internal id as actor: err=%v statements=%d", err, len(r.stmts))
	}
}

func TestCredentialsNeverFormatted(t *testing.T) {
	c := Credentials{PasswordHash: "secret-hash", TOTPSecretSealed: []byte("sealed")}
	for _, s := range []string{fmt.Sprintf("%v", c), fmt.Sprintf("%+v", c), fmt.Sprintf("%#v", c), fmt.Sprint(c)} {
		if strings.Contains(s, "secret-hash") || strings.Contains(s, "sealed]") {
			t.Errorf("credential printed: %s", s)
		}
	}
}

// No DELETE statement anywhere in this package (rule 7). Split so the literal does not appear here.
func TestNoDeleteStatement(t *testing.T) {
	src, err := os.ReadFile("store.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToUpper(string(src)), "DELETE"+" FROM") {
		t.Error("store.go contains a DELETE statement — operator rows are revoked, voided or disabled, never deleted")
	}
}

// The migration's CHECK list and the Go closed list are the same six keys.
func TestPermissionCheckMatchesDomainList(t *testing.T) {
	b, err := fs.ReadFile(migrations.FS, "0012_operator_accounts.sql")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?s)operator_permission_grant_key_known CHECK \(permission_key IN \((.*?)\)\)`).FindSubmatch(b)
	if m == nil {
		t.Fatal("CHECK operator_permission_grant_key_known not found in 0012")
	}
	var inSQL []string
	for _, part := range strings.Split(string(m[1]), ",") {
		inSQL = append(inSQL, strings.Trim(strings.TrimSpace(part), "'"))
	}
	want := domain.OperatorPermissions()
	if len(inSQL) != len(want) {
		t.Fatalf("SQL lists %d keys, Go lists %d", len(inSQL), len(want))
	}
	for i, k := range want {
		if inSQL[i] != string(k) {
			t.Errorf("key %d: SQL %q, Go %q", i, inSQL[i], k)
		}
	}
}
