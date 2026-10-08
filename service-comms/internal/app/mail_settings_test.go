package app

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/crypto"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	"github.com/vihat/vigov/service-comms/internal/mail"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// A fake database/sql driver under the REAL MailSettingsStore, and the REAL crypto.Envelope over an
// in-memory DEKStore. The properties worth proving — one transaction for the write and its entry, a
// refusal commits nothing, the password is sealed before it is written and opens back, the delta
// holds no secret — live in the SQL, the transaction boundaries and the envelope; fakes of any of the
// three would erase them.
//
// WHAT IT DOES NOT PROVE: anything PostgreSQL does — the CHECK constraints, the triggers, the
// partition routing (VIGOV_TEST_DSN is unset here).

// testMailPassword is a fixture; no server accepts it.
const testMailPassword = "fixture-not-a-real-password-7Q"

type mailStmt struct {
	sql  string
	args []driver.Value
}

type mailRow struct {
	host, security, username, from, name string
	port                                 int
	enabled                              bool
	sealed                               []byte
	lastTest                             *domain.MailTestResult
}

type mailDB struct {
	mu    sync.Mutex
	stmts []mailStmt

	begun, committed, rolledBack int
	row                          *mailRow
	failOnContain                string
}

func (d *mailDB) record(q string, args []driver.NamedValue) {
	vals := make([]driver.Value, 0, len(args))
	for _, a := range args {
		vals = append(vals, a.Value)
	}
	d.mu.Lock()
	d.stmts = append(d.stmts, mailStmt{sql: q, args: vals})
	d.mu.Unlock()
}

func (d *mailDB) with(sub string) []mailStmt {
	d.mu.Lock()
	defer d.mu.Unlock()
	var out []mailStmt
	for _, s := range d.stmts {
		if strings.Contains(s.sql, sub) {
			out = append(out, s)
		}
	}
	return out
}

func (d *mailDB) indexOf(sub string) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i, s := range d.stmts {
		if strings.Contains(s.sql, sub) {
			return i
		}
	}
	return -1
}

func (d *mailDB) Connect(context.Context) (driver.Conn, error) { return &mailConn{d: d}, nil }
func (d *mailDB) Driver() driver.Driver                        { return mailDriverOpen{} }

type mailDriverOpen struct{}

func (mailDriverOpen) Open(string) (driver.Conn, error) { return nil, errors.New("connector only") }

type mailConn struct{ d *mailDB }

func (c *mailConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("no Prepare") }
func (c *mailConn) Close() error                        { return nil }
func (c *mailConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *mailConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.d.mu.Lock()
	c.d.begun++
	c.d.mu.Unlock()
	return &mailTx{d: c.d}, nil
}

func (c *mailConn) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.d.record(q, args)
	if c.d.failOnContain != "" && strings.Contains(q, c.d.failOnContain) {
		return nil, errors.New("fake driver: this statement is built to fail")
	}
	return driver.RowsAffected(1), nil
}

func (c *mailConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.d.record(q, args)
	if !strings.Contains(q, "FROM mail_settings") {
		return nil, fmt.Errorf("fake driver: no answer for %q", q)
	}
	cols := []string{"host", "port", "security", "username", "from_address", "from_name", "is_enabled", "x"}
	withTest := strings.Contains(q, "last_test_at")
	if withTest {
		cols = append(cols, "t_at", "t_to", "t_ok", "t_class")
	}
	if c.d.row == nil {
		return &mailRows{cols: cols}, nil
	}
	r := c.d.row
	var last driver.Value = r.sealed
	if strings.Contains(q, "IS NOT NULL") {
		last = len(r.sealed) > 0
	}
	row := []driver.Value{r.host, int64(r.port), r.security, r.username, r.from, r.name, r.enabled, last}
	if withTest {
		if t := r.lastTest; t != nil {
			var class driver.Value
			if t.ErrorClass != "" {
				class = t.ErrorClass
			}
			row = append(row, t.At, t.ToMasked, t.OK, class)
		} else {
			row = append(row, nil, nil, nil, nil)
		}
	}
	return &mailRows{cols: cols, rows: [][]driver.Value{row}}, nil
}

type mailTx struct{ d *mailDB }

func (t *mailTx) Commit() error   { t.d.mu.Lock(); t.d.committed++; t.d.mu.Unlock(); return nil }
func (t *mailTx) Rollback() error { t.d.mu.Lock(); t.d.rolledBack++; t.d.mu.Unlock(); return nil }

type mailRows struct {
	cols []string
	rows [][]driver.Value
	i    int
}

func (r *mailRows) Columns() []string { return r.cols }
func (r *mailRows) Close() error      { return nil }
func (r *mailRows) Next(dest []driver.Value) error {
	if r.i >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.i])
	r.i++
	return nil
}

// memoryDEKStore is crypto.DEKStore in memory, KEYED BY THE COMMUNE IN THE CONTEXT like the real
// one — keyed any other way, the wrong-commune case below would prove nothing.
type memoryDEKStore struct {
	mu   sync.Mutex
	rows map[tenant.ID]crypto.WrappedDEK
}

func (m *memoryDEKStore) GetDEK(ctx context.Context) (crypto.WrappedDEK, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	w, ok := m.rows[tenant.MustFrom(ctx)]
	if !ok {
		return crypto.WrappedDEK{}, crypto.ErrDEKNotFound
	}
	return w, nil
}

func (m *memoryDEKStore) CreateDEK(ctx context.Context, dek crypto.WrappedDEK) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.rows == nil {
		m.rows = map[tenant.ID]crypto.WrappedDEK{}
	}
	if _, ok := m.rows[tenant.MustFrom(ctx)]; ok {
		return crypto.ErrDEKExists
	}
	m.rows[tenant.MustFrom(ctx)] = dek
	return nil
}

func (m *memoryDEKStore) ReplaceDEK(context.Context, crypto.WrappedDEK, crypto.WrappedDEK) error {
	return errors.New("not used")
}

// fakeSender records what would have gone to the network.
type fakeSender struct {
	calls    int
	account  mail.Account
	password string
	message  mail.Message
	err      error
	// auditsBefore is how many audit INSERTs had been recorded when Send was called.
	auditsBefore int
	db           *mailDB
}

func (f *fakeSender) Send(_ context.Context, a mail.Account, m mail.Message) error {
	f.calls++
	f.account, f.message = a, m
	f.password = string(a.Password.Lo())
	if f.db != nil {
		f.auditsBefore = len(f.db.with("INSERT INTO audit_log"))
	}
	return f.err
}

func testEnvelope(t *testing.T) *crypto.Envelope {
	t.Helper()
	// A test KEK: 32 fixed bytes, not a key anyone uses.
	env, err := crypto.New([]secret.Secret{bytes.Repeat([]byte{7}, crypto.KeyLength)}, &memoryDEKStore{})
	if err != nil {
		t.Fatalf("crypto.New: %v", err)
	}
	return env
}

func newMailUseCase(t *testing.T, d *mailDB, env *crypto.Envelope) (*MailSettingsAdmin, *fakeSender, context.Context) {
	t.Helper()
	db := sql.OpenDB(d)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	handle := store.New(db)
	sender := &fakeSender{db: d}
	return NewMailSettingsAdmin(handle, commsstore.NewMailSettingsStore(handle), env, sender),
		sender, tenant.Into(context.Background(), xaA)
}

func validMailInput() domain.MailSettingsInput {
	return domain.MailSettingsInput{
		Host: "smtp.example.test", Port: 587, Security: domain.MailSecurityStartTLS,
		Username: "ubnd@example.test", FromAddress: "ubnd@example.test", FromName: "UBND xã", IsEnabled: true,
	}
}

// storedRow is a row whose password was sealed by env for commune A.
func storedRow(t *testing.T, env *crypto.Envelope) *mailRow {
	t.Helper()
	ctx := tenant.Into(context.Background(), xaA)
	sealed, err := env.Seal(ctx, secret.Secret(testMailPassword), passwordAAD(ctx))
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	return &mailRow{host: "smtp.example.test", port: 587, security: "starttls", username: "ubnd@example.test",
		from: "ubnd@example.test", name: "UBND xã", enabled: true, sealed: sealed}
}

// assertNoSecret — the delta must hold neither the password nor anything derived from its sealed form.
func assertNoSecret(t *testing.T, delta []byte, sealed []byte) {
	t.Helper()
	s := string(delta)
	if strings.Contains(s, testMailPassword) {
		t.Fatalf("audit delta contains the password: %s", s)
	}
	if len(sealed) > 0 && (strings.Contains(s, base64.StdEncoding.EncodeToString(sealed)) ||
		bytes.Contains(delta, sealed)) {
		t.Fatalf("audit delta contains the sealed bytes: %s", s)
	}
	if strings.Contains(s, `"password"`) {
		t.Fatalf("audit delta has a password key: %s", s)
	}
}

// --- save ---------------------------------------------------------------------------------------

func TestSaveMailSettingsFirstTimeSealsAndAuditsInOneTransaction(t *testing.T) {
	d := &mailDB{}
	env := testEnvelope(t)
	uc, _, ctx := newMailUseCase(t, d, env)

	v, err := uc.Save(ctx, SaveMailSettingsRequest{Input: validMailInput(), Password: secret.Secret(testMailPassword)}, staffActor)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if d.begun != 1 || d.committed != 1 || d.rolledBack != 0 {
		t.Fatalf("tx begun/committed/rolled back = %d/%d/%d, want 1/1/0", d.begun, d.committed, d.rolledBack)
	}
	up := d.with("INSERT INTO mail_settings")
	if len(up) != 1 {
		t.Fatalf("%d upserts, want 1", len(up))
	}
	if up[0].args[0] != string(xaA) {
		t.Errorf("$1 = %v, want the context commune", up[0].args[0])
	}
	sealed, _ := up[0].args[8].([]byte)
	if len(sealed) == 0 || bytes.Contains(sealed, []byte(testMailPassword)) {
		t.Fatalf("the password column is not sealed: %q", sealed)
	}
	// ROUND TRIP through the real envelope, with the same AAD: what was written opens to what was typed.
	plain, err := env.Open(ctx, sealed, passwordAAD(ctx))
	if err != nil || string(plain.Lo()) != testMailPassword {
		t.Fatalf("sealed password does not open back: %v", err)
	}
	// And it does NOT open as another commune's, or as another column.
	ctxB := tenant.Into(context.Background(), xaB)
	if _, err := env.Open(ctxB, sealed, passwordAAD(ctxB)); err == nil {
		t.Error("commune A's sealed password opened for commune B")
	}
	if _, err := env.Open(ctx, sealed, []byte("other_table/other_column/"+string(xaA))); err == nil {
		t.Error("the sealed password opened under another AAD")
	}
	if up[0].args[9] != "CB-00123" {
		t.Errorf("updated_by = %v, want the business code", up[0].args[9])
	}
	iUp, iAudit := d.indexOf("INSERT INTO mail_settings"), d.indexOf("INSERT INTO audit_log")
	if iAudit < iUp {
		t.Fatalf("audit at %d, upsert at %d — the entry must follow the write in its tx", iAudit, iUp)
	}
	a := d.with("INSERT INTO audit_log")[0]
	if a.args[1] != "CB-00123" || a.args[4] != ActionSaveMailSettings || a.args[5] != domain.MailSettingsSubject {
		t.Errorf("audit actor/action/subject = %v/%v/%v", a.args[1], a.args[4], a.args[5])
	}
	delta, _ := a.args[7].([]byte)
	assertNoSecret(t, delta, sealed)
	if !strings.Contains(string(delta), `"password_changed":true`) {
		t.Errorf("delta does not record the password change: %s", delta)
	}
	if !v.PasswordSet || !v.Configured {
		t.Errorf("view = %+v", v)
	}
}

func TestSaveMailSettingsBlankPasswordKeepsTheStoredOne(t *testing.T) {
	env := testEnvelope(t)
	d := &mailDB{row: storedRow(t, env)}
	uc, _, ctx := newMailUseCase(t, d, env)

	in := validMailInput()
	in.FromName = "UBND xã (đổi tên)"
	if _, err := uc.Save(ctx, SaveMailSettingsRequest{Input: in}, staffActor); err != nil {
		t.Fatalf("Save: %v", err)
	}
	up := d.with("INSERT INTO mail_settings")
	if len(up) != 1 {
		t.Fatalf("%d upserts, want 1", len(up))
	}
	if kept, _ := up[0].args[8].([]byte); !bytes.Equal(kept, d.row.sealed) {
		t.Fatal("a blank password did not keep the stored sealed bytes")
	}
	delta, _ := d.with("INSERT INTO audit_log")[0].args[7].([]byte)
	assertNoSecret(t, delta, d.row.sealed)
	if !strings.Contains(string(delta), `"password_changed":false`) || !strings.Contains(string(delta), "đổi tên") {
		t.Errorf("delta = %s", delta)
	}
}

func TestSaveMailSettingsDestinationChangeWithoutPasswordIsRefused(t *testing.T) {
	for name, edit := range map[string]func(*domain.MailSettingsInput){
		"host":     func(in *domain.MailSettingsInput) { in.Host = "smtp.attacker.example" },
		"port":     func(in *domain.MailSettingsInput) { in.Port = 465; in.Security = domain.MailSecurityTLS },
		"username": func(in *domain.MailSettingsInput) { in.Username = "someone-else@example.test" },
	} {
		t.Run(name, func(t *testing.T) {
			env := testEnvelope(t)
			d := &mailDB{row: storedRow(t, env)}
			uc, _, ctx := newMailUseCase(t, d, env)
			in := validMailInput()
			edit(&in)
			_, err := uc.Save(ctx, SaveMailSettingsRequest{Input: in}, staffActor)
			if !errors.Is(err, ErrMailPasswordRequiredForNewDestination) {
				t.Fatalf("err = %v, want ErrMailPasswordRequiredForNewDestination", err)
			}
			if len(d.with("INSERT INTO")) != 0 || d.committed != 0 || d.rolledBack != 1 {
				t.Fatalf("refusal wrote something: stmts=%d committed=%d rolledBack=%d",
					len(d.with("INSERT INTO")), d.committed, d.rolledBack)
			}
		})
	}
}

func TestSaveMailSettingsHostCapitalisationIsNotADestinationChange(t *testing.T) {
	env := testEnvelope(t)
	d := &mailDB{row: storedRow(t, env)}
	uc, _, ctx := newMailUseCase(t, d, env)
	in := validMailInput()
	in.Host = "  SMTP.Example.TEST "
	if _, err := uc.Save(ctx, SaveMailSettingsRequest{Input: in}, staffActor); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if len(d.with("INSERT INTO")) != 0 {
		t.Error("an unchanged configuration (host spelled differently) was written")
	}
}

func TestSaveMailSettingsFirstSaveWithoutPasswordIsRefused(t *testing.T) {
	d := &mailDB{}
	uc, _, ctx := newMailUseCase(t, d, testEnvelope(t))
	_, err := uc.Save(ctx, SaveMailSettingsRequest{Input: validMailInput()}, staffActor)
	if !errors.Is(err, ErrMailPasswordRequired) {
		t.Fatalf("err = %v, want ErrMailPasswordRequired", err)
	}
	if len(d.with("INSERT INTO")) != 0 || d.committed != 0 {
		t.Fatal("refusal wrote something")
	}
}

func TestSaveMailSettingsNoOpWritesAndAuditsNothing(t *testing.T) {
	env := testEnvelope(t)
	d := &mailDB{row: storedRow(t, env)}
	uc, _, ctx := newMailUseCase(t, d, env)
	if _, err := uc.Save(ctx, SaveMailSettingsRequest{Input: validMailInput()}, staffActor); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if len(d.with("INSERT INTO")) != 0 {
		t.Error("a save that changed nothing wrote or audited")
	}
}

func TestSaveMailSettingsNewPasswordOnNewHostIsAccepted(t *testing.T) {
	env := testEnvelope(t)
	d := &mailDB{row: storedRow(t, env)}
	uc, _, ctx := newMailUseCase(t, d, env)
	in := validMailInput()
	in.Host = "smtp.new.example.test"
	if _, err := uc.Save(ctx, SaveMailSettingsRequest{Input: in, Password: secret.Secret("another-fixture-9")}, staffActor); err != nil {
		t.Fatalf("Save: %v", err)
	}
	up := d.with("INSERT INTO mail_settings")
	sealed, _ := up[0].args[8].([]byte)
	if bytes.Equal(sealed, d.row.sealed) {
		t.Fatal("a new password did not replace the stored one")
	}
	delta, _ := d.with("INSERT INTO audit_log")[0].args[7].([]byte)
	assertNoSecret(t, delta, sealed)
	if strings.Contains(string(delta), "another-fixture-9") {
		t.Fatal("delta holds the new password")
	}
}

func TestSaveMailSettingsWithoutEnvelopeRefusesAndWritesNothing(t *testing.T) {
	for name, pwd := range map[string]secret.Secret{"with password": secret.Secret(testMailPassword), "blank": nil} {
		t.Run(name, func(t *testing.T) {
			d := &mailDB{row: &mailRow{host: "smtp.example.test", port: 587, security: "starttls",
				username: "u@example.test", from: "u@example.test", sealed: []byte("x")}}
			uc, _, ctx := newMailUseCase(t, d, nil)
			in := validMailInput()
			in.FromName = "changed"
			_, err := uc.Save(ctx, SaveMailSettingsRequest{Input: in, Password: pwd}, staffActor)
			if !errors.Is(err, crypto.ErrNotConfigured) {
				t.Fatalf("err = %v, want crypto.ErrNotConfigured", err)
			}
			if d.begun != 0 || len(d.stmts) != 0 {
				t.Fatalf("no KEK and the database was touched: begun=%d stmts=%d", d.begun, len(d.stmts))
			}
		})
	}
}

func TestSaveMailSettingsAuditFailureRollsBackTheRow(t *testing.T) {
	d := &mailDB{failOnContain: "INSERT INTO audit_log"}
	uc, _, ctx := newMailUseCase(t, d, testEnvelope(t))
	if _, err := uc.Save(ctx, SaveMailSettingsRequest{Input: validMailInput(), Password: secret.Secret(testMailPassword)}, staffActor); err == nil {
		t.Fatal("audit failure did not fail the save")
	}
	if d.committed != 0 || d.rolledBack != 1 {
		t.Fatalf("committed=%d rolledBack=%d, want 0/1", d.committed, d.rolledBack)
	}
}

func TestSaveMailSettingsErrorNeverCarriesThePassword(t *testing.T) {
	d := &mailDB{failOnContain: "INSERT INTO mail_settings"}
	uc, _, ctx := newMailUseCase(t, d, testEnvelope(t))
	_, err := uc.Save(ctx, SaveMailSettingsRequest{Input: validMailInput(), Password: secret.Secret(testMailPassword)}, staffActor)
	if err == nil || strings.Contains(err.Error(), testMailPassword) {
		t.Fatalf("err = %v", err)
	}
}

// --- read ---------------------------------------------------------------------------------------

func TestGetMailSettingsUnconfiguredReturnsDefaults(t *testing.T) {
	uc, _, ctx := newMailUseCase(t, &mailDB{}, nil)
	v, err := uc.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if v.Configured || v.EncryptionConfigured || v.Port != 587 || v.Security != domain.MailSecurityStartTLS {
		t.Errorf("view = %+v", v)
	}
}

func TestGetMailSettingsReadsNoSealedBytes(t *testing.T) {
	env := testEnvelope(t)
	d := &mailDB{row: storedRow(t, env)}
	uc, _, ctx := newMailUseCase(t, d, env)
	v, err := uc.Get(ctx)
	if err != nil || !v.PasswordSet || !v.Configured || !v.EncryptionConfigured {
		t.Fatalf("view = %+v err = %v", v, err)
	}
	q := d.with("FROM mail_settings")
	if len(q) != 1 || !strings.Contains(q[0].sql, "password_sealed IS NOT NULL") {
		t.Fatalf("the read selected the sealed column itself: %v", q)
	}
}

// --- test message -------------------------------------------------------------------------------

func TestSendTestMessageOpensPasswordAuditsFirstThenSends(t *testing.T) {
	env := testEnvelope(t)
	d := &mailDB{row: storedRow(t, env)}
	uc, sender, ctx := newMailUseCase(t, d, env)

	if err := uc.SendTestMessage(ctx, "canbo@example.test", staffActor); err != nil {
		t.Fatalf("SendTestMessage: %v", err)
	}
	if sender.calls != 1 || sender.password != testMailPassword {
		t.Fatalf("sender calls=%d, password opened correctly=%v", sender.calls, sender.password == testMailPassword)
	}
	// Two transactions: the attempt's entry BEFORE the send, the result (0019) after it.
	if sender.auditsBefore != 1 || d.committed != 2 {
		t.Fatalf("audit entries before send = %d, commits = %d — the attempt must be on record first",
			sender.auditsBefore, d.committed)
	}
	if sender.account.Host != "smtp.example.test" || sender.account.Security != mail.SecurityStartTLS ||
		sender.message.To != "canbo@example.test" || sender.message.Subject != testMailSubject {
		t.Errorf("account/message = %+v / %+v", sender.account, sender.message)
	}
	a := d.with("INSERT INTO audit_log")[0]
	delta, _ := a.args[7].([]byte)
	assertNoSecret(t, delta, d.row.sealed)
	if a.args[4] != ActionSendTestMail || strings.Contains(string(delta), "canbo@") ||
		!strings.Contains(string(delta), "c***@example.test") {
		t.Errorf("audit action %v, delta %s — recipient must be masked", a.args[4], delta)
	}
}

func TestSendTestMessageAuditFailureSendsNothing(t *testing.T) {
	env := testEnvelope(t)
	d := &mailDB{row: storedRow(t, env), failOnContain: "INSERT INTO audit_log"}
	uc, sender, ctx := newMailUseCase(t, d, env)
	if err := uc.SendTestMessage(ctx, "canbo@example.test", staffActor); err == nil {
		t.Fatal("audit failure did not fail the send")
	}
	if sender.calls != 0 {
		t.Fatal("the credential was used with no entry on record")
	}
}

func TestSendTestMessageRefusals(t *testing.T) {
	env := testEnvelope(t)
	for name, tc := range map[string]struct {
		env  *crypto.Envelope
		row  *mailRow
		to   string
		want error
	}{
		"no KEK":        {nil, storedRow(t, env), "a@example.test", crypto.ErrNotConfigured},
		"nothing saved": {env, nil, "a@example.test", commsstore.ErrMailSettingsNotFound},
		"bad recipient": {env, storedRow(t, env), "Tên <a@example.test>", domain.ErrMailRecipient},
		"header inject": {env, storedRow(t, env), "a@example.test\r\nBcc: x@example.test", domain.ErrMailRecipient},
		"foreign bytes": {env, &mailRow{host: "h.example.test", port: 587, security: "starttls",
			username: "u", from: "u@example.test", sealed: bytes.Repeat([]byte{1}, 40)}, "a@example.test", crypto.ErrOpenFailed},
	} {
		t.Run(name, func(t *testing.T) {
			d := &mailDB{row: tc.row}
			uc, sender, ctx := newMailUseCase(t, d, tc.env)
			err := uc.SendTestMessage(ctx, tc.to, staffActor)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if sender.calls != 0 || len(d.with("INSERT INTO")) != 0 {
				t.Fatal("a refused test message still audited or sent")
			}
			// 0019: a failure before the send records no result — no test reached a server.
			if len(d.with("UPDATE mail_settings")) != 0 {
				t.Fatal("a refusal before the send recorded a test result")
			}
		})
	}
}

func TestSendTestMessagePassesSenderErrorThrough(t *testing.T) {
	env := testEnvelope(t)
	d := &mailDB{row: storedRow(t, env)}
	uc, sender, ctx := newMailUseCase(t, d, env)
	sender.err = mail.ErrAuthRejected
	if err := uc.SendTestMessage(ctx, "a@example.test", staffActor); !errors.Is(err, mail.ErrAuthRejected) {
		t.Fatalf("err = %v", err)
	}
}

// --- the last test result (migration 0019) -------------------------------------------------------

var mailTestClock = time.Date(2026, 10, 8, 3, 0, 0, 0, time.UTC)

func TestSendTestMessageRecordsTheResultWithItsEntryInOneTransaction(t *testing.T) {
	env := testEnvelope(t)
	d := &mailDB{row: storedRow(t, env)}
	uc, _, ctx := newMailUseCase(t, d, env)
	uc.now = func() time.Time { return mailTestClock }

	if err := uc.SendTestMessage(ctx, "canbo@example.test", staffActor); err != nil {
		t.Fatalf("SendTestMessage: %v", err)
	}
	up := d.with("UPDATE mail_settings")
	if len(up) != 1 {
		t.Fatalf("%d result updates, want 1", len(up))
	}
	// Recording a test is NOT a save: updated_at / updated_by stay as the last saver left them.
	if strings.Contains(up[0].sql, "updated_at") || strings.Contains(up[0].sql, "updated_by") {
		t.Fatalf("the result update touches updated_at/updated_by: %s", up[0].sql)
	}
	if up[0].args[0] != string(xaA) || up[0].args[2] != "c***@example.test" || up[0].args[3] != true || up[0].args[4] != nil {
		t.Fatalf("result args = %v — want commune, masked recipient, ok, no class", up[0].args)
	}
	if strings.Contains(fmt.Sprint(up[0].args), "canbo@") {
		t.Fatal("the raw recipient reached the row")
	}
	iUp := d.indexOf("UPDATE mail_settings")
	entries := d.with("INSERT INTO audit_log")
	if len(entries) != 2 || entries[1].args[4] != ActionRecordTestMailResult {
		t.Fatalf("entries = %d, second action = %v — the result needs its own entry", len(entries), entries[1].args[4])
	}
	if iAudit := len(d.stmts) - 1; d.stmts[iAudit].sql != entries[1].sql || iAudit < iUp {
		t.Error("the result's entry must follow its UPDATE in the same transaction")
	}
	if d.begun != 2 || d.committed != 2 {
		t.Fatalf("begun/committed = %d/%d, want 2/2", d.begun, d.committed)
	}
}

func TestSendTestMessageFailureIsRecordedAsItsClass(t *testing.T) {
	env := testEnvelope(t)
	d := &mailDB{row: storedRow(t, env)}
	uc, sender, ctx := newMailUseCase(t, d, env)
	sender.err = fmt.Errorf("%w (smtp 535)", mail.ErrAuthRejected)

	err := uc.SendTestMessage(ctx, "canbo@example.test", staffActor)
	if !errors.Is(err, mail.ErrAuthRejected) {
		t.Fatalf("err = %v — the send's own failure must still reach the handler", err)
	}
	up := d.with("UPDATE mail_settings")
	if len(up) != 1 || up[0].args[3] != false || up[0].args[4] != domain.MailTestErrorAuthRejected {
		t.Fatalf("result = %v, want ok=false class %q", up, domain.MailTestErrorAuthRejected)
	}
	delta, _ := d.with("INSERT INTO audit_log")[1].args[7].([]byte)
	if strings.Contains(string(delta), "535") || strings.Contains(string(delta), "canbo@") {
		t.Errorf("the result entry carries the reply code or the raw address: %s", delta)
	}
}

func TestMailErrorClassIsOneToOneWithTheSentinels(t *testing.T) {
	// 0019's CHECK list, verbatim.
	allowed := map[string]bool{"khong-ket-noi": true, "het-thoi-gian": true, "chung-chi-khong-hop-le": true,
		"loi-tls": true, "khong-co-starttls": true, "tu-choi-khong-ma-hoa": true, "khong-ho-tro-dang-nhap": true,
		"sai-tai-khoan": true, "tu-choi-dia-chi": true, "sai-giao-thuc": true, "khac": true}
	sentinels := []error{mail.ErrPlaintextRefused, mail.ErrConnect, mail.ErrTimeout, mail.ErrCertificate, mail.ErrTLS,
		mail.ErrStartTLSMissing, mail.ErrAuthUnsupported, mail.ErrAuthRejected, mail.ErrRecipientRejected, mail.ErrProtocol}
	seen := map[string]bool{}
	for _, s := range sentinels {
		c := mailErrorClass(fmt.Errorf("wrapped: %w", s))
		if !allowed[c] || c == domain.MailTestErrorOther || seen[c] {
			t.Errorf("%v → %q: not a distinct 0019 class", s, c)
		}
		seen[c] = true
	}
	if c := mailErrorClass(errors.New("something else")); c != domain.MailTestErrorOther {
		t.Errorf("an unknown error → %q, want %q", c, domain.MailTestErrorOther)
	}
}

func TestSendTestMessageResultRecordFailureIsReported(t *testing.T) {
	env := testEnvelope(t)
	d := &mailDB{row: storedRow(t, env), failOnContain: "UPDATE mail_settings"}
	uc, sender, ctx := newMailUseCase(t, d, env)
	if err := uc.SendTestMessage(ctx, "canbo@example.test", staffActor); err == nil {
		t.Fatal("a result that did not land was reported as success")
	}
	if sender.calls != 1 || d.rolledBack != 1 {
		t.Fatalf("calls=%d rolledBack=%d", sender.calls, d.rolledBack)
	}
}

func TestSaveMailSettingsClearsTheLastTestWhenTheTargetMoves(t *testing.T) {
	for name, tc := range map[string]struct {
		edit  func(*domain.MailSettingsInput)
		clear bool
	}{
		"host":      {func(in *domain.MailSettingsInput) { in.Host = "smtp.new.example.test" }, true},
		"port":      {func(in *domain.MailSettingsInput) { in.Port = 2525 }, true},
		"security":  {func(in *domain.MailSettingsInput) { in.Port = 465; in.Security = domain.MailSecurityTLS }, true},
		"username":  {func(in *domain.MailSettingsInput) { in.Username = "khac@example.test" }, true},
		"from name": {func(in *domain.MailSettingsInput) { in.FromName = "Tên khác" }, false},
	} {
		t.Run(name, func(t *testing.T) {
			env := testEnvelope(t)
			row := storedRow(t, env)
			row.lastTest = &domain.MailTestResult{At: mailTestClock, ToMasked: "c***@example.test", OK: true}
			d := &mailDB{row: row}
			uc, _, ctx := newMailUseCase(t, d, env)
			in := validMailInput()
			tc.edit(&in)
			v, err := uc.Save(ctx, SaveMailSettingsRequest{Input: in, Password: secret.Secret("fixture-new-pass-3")}, staffActor)
			if err != nil {
				t.Fatalf("Save: %v", err)
			}
			cleared := len(d.with("last_test_at = NULL")) == 1
			if cleared != tc.clear {
				t.Fatalf("cleared = %v, want %v", cleared, tc.clear)
			}
			if (v.LastTest == nil) != tc.clear {
				t.Errorf("reply last test = %+v, want cleared=%v", v.LastTest, tc.clear)
			}
			delta, _ := d.with("INSERT INTO audit_log")[0].args[7].([]byte)
			if !strings.Contains(string(delta), fmt.Sprintf(`"last_test_cleared":%v`, tc.clear)) {
				t.Errorf("delta = %s", delta)
			}
		})
	}
}

func TestGetMailSettingsCarriesTheLastTest(t *testing.T) {
	env := testEnvelope(t)
	row := storedRow(t, env)
	row.lastTest = &domain.MailTestResult{At: mailTestClock, ToMasked: "c***@example.test", ErrorClass: domain.MailTestErrorTimeout}
	uc, _, ctx := newMailUseCase(t, &mailDB{row: row}, env)
	v, err := uc.Get(ctx)
	if err != nil || v.LastTest == nil || v.LastTest.OK || v.LastTest.ErrorClass != domain.MailTestErrorTimeout ||
		v.LastTest.ToMasked != "c***@example.test" {
		t.Fatalf("view = %+v err = %v", v.LastTest, err)
	}
}
