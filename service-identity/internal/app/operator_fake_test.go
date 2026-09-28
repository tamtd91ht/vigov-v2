package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/service-identity/internal/domain"
	"github.com/vihat/vigov/service-identity/internal/operatorauth"
	"github.com/vihat/vigov/service-identity/internal/store/operatorstore"
)

// An in-memory operator store with TRANSACTION SEMANTICS: every InTx works on a copy of the state
// and only a nil return commits it. That is what lets these tests prove the two properties a
// recording fake cannot — a refused attempt's failure count survives (the closure returned nil),
// and a failed audit write takes the business change down with it (the closure returned an error).
// Each committed or rolled-back transaction is kept as a list of the operations it ran, in order.
//
// The behaviours mirror operatorstore's SQL: strict TOTP step compare, failure count reset at the
// lock, no counting while locked, revoke-all on password change / disable / grant change / MFA reset.

type fakeCreds struct {
	passwordHash string
	totpSealed   []byte
	pendingTOTP  []byte
	lastStep     *int64
}

type fakeSession struct {
	accountID string
	created   time.Time
	expires   time.Time
	revoked   bool
	reason    string
	lastSeen  time.Time
}

type fakeGrant struct {
	accountID string
	key       domain.OperatorPermission
	revoked   bool
}

type fakeCode struct {
	accountID string
	digest    string
	used      bool
	voided    bool
}

type fakeState struct {
	nextCode int64
	accounts map[string]domain.OperatorAccount // by id
	creds    map[string]fakeCreds
	sessions map[string]fakeSession // by RAW sid (the real store hashes; irrelevant here)
	grants   []fakeGrant
	codes    []fakeCode
	audit    []domain.OperatorAuditEntry
}

func (s fakeState) clone() fakeState {
	c := fakeState{nextCode: s.nextCode,
		accounts: map[string]domain.OperatorAccount{}, creds: map[string]fakeCreds{},
		sessions: map[string]fakeSession{},
		grants:   append([]fakeGrant(nil), s.grants...),
		codes:    append([]fakeCode(nil), s.codes...),
		audit:    append([]domain.OperatorAuditEntry(nil), s.audit...),
	}
	for k, v := range s.accounts {
		c.accounts[k] = v
	}
	for k, v := range s.creds {
		c.creds[k] = v
	}
	for k, v := range s.sessions {
		c.sessions[k] = v
	}
	return c
}

type fakeTxRecord struct {
	ops       []string
	committed bool
}

type fakeOperatorStore struct {
	mu    sync.Mutex
	state fakeState
	txs   []*fakeTxRecord
	// failAudit makes AppendAudit fail for this action — the "audit write fails" case.
	failAudit domain.OperatorAuditAction
	// before runs just before the named operation, inside the transaction, with its state — how a
	// test stands in for a PARALLEL transaction that committed in between (a lock set, a pending
	// secret replaced). Transactions here are serial, so this is the only way to interleave.
	before map[string]func(st *fakeState)
}

func newFakeOperatorStore() *fakeOperatorStore {
	return &fakeOperatorStore{state: fakeState{
		accounts: map[string]domain.OperatorAccount{}, creds: map[string]fakeCreds{},
		sessions: map[string]fakeSession{},
	}}
}

func (f *fakeOperatorStore) InTx(_ context.Context, fn func(tx OperatorTx) error) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	rec := &fakeTxRecord{}
	f.txs = append(f.txs, rec)
	tx := &fakeOperatorTx{st: f.state.clone(), rec: rec, failAudit: f.failAudit, before: f.before}
	if err := fn(tx); err != nil {
		return err
	}
	f.state = tx.st
	rec.committed = true
	return nil
}

func (f *fakeOperatorStore) ListAccounts(context.Context) ([]domain.OperatorAccount, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.OperatorAccount
	for _, a := range f.state.accounts {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out, nil
}

func (f *fakeOperatorStore) ActivePermissions(_ context.Context, id string) ([]domain.OperatorPermission, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return activeKeys(f.state, id), nil
}

func activeKeys(s fakeState, id string) []domain.OperatorPermission {
	var out []domain.OperatorPermission
	for _, g := range s.grants {
		if g.accountID == id && !g.revoked {
			out = append(out, g.key)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// lastTx returns the most recent transaction record.
func (f *fakeOperatorStore) lastTx() *fakeTxRecord { return f.txs[len(f.txs)-1] }

func (f *fakeOperatorStore) auditActions() []domain.OperatorAuditAction {
	var out []domain.OperatorAuditAction
	for _, e := range f.state.audit {
		out = append(out, e.Action)
	}
	return out
}

func (f *fakeOperatorStore) account(t *testing.T, code string) domain.OperatorAccount {
	t.Helper()
	for _, a := range f.state.accounts {
		if a.Code == code {
			return a
		}
	}
	t.Fatalf("no account %s", code)
	return domain.OperatorAccount{}
}

type fakeOperatorTx struct {
	st        fakeState
	rec       *fakeTxRecord
	failAudit domain.OperatorAuditAction
	before    map[string]func(st *fakeState)
}

func (t *fakeOperatorTx) op(name string) {
	if fn := t.before[name]; fn != nil {
		fn(&t.st)
	}
	t.rec.ops = append(t.rec.ops, name)
}

// lockedAt mirrors the SQL guard `(locked_until IS NULL OR locked_until <= now)`, negated.
func (t *fakeOperatorTx) lockedAt(id string, now time.Time) bool {
	return t.st.accounts[id].LockedAt(now)
}

func (t *fakeOperatorTx) ByEmailForUpdate(ctx context.Context, email string) (domain.OperatorAccount, error) {
	t.op("ByEmailForUpdate")
	return t.find(func(a domain.OperatorAccount) bool {
		return strings.EqualFold(a.Email, strings.TrimSpace(email))
	})
}

func (t *fakeOperatorTx) ByIDForUpdate(_ context.Context, id string) (domain.OperatorAccount, error) {
	t.op("ByIDForUpdate")
	return t.find(func(a domain.OperatorAccount) bool { return a.ID == id })
}

func (t *fakeOperatorTx) find(pred func(domain.OperatorAccount) bool) (domain.OperatorAccount, error) {
	for _, a := range t.st.accounts {
		if pred(a) {
			return a, nil
		}
	}
	return domain.OperatorAccount{}, operatorstore.ErrNotFound
}

func (t *fakeOperatorTx) ByID(_ context.Context, id string) (domain.OperatorAccount, error) {
	t.op("ByID")
	return t.find(func(a domain.OperatorAccount) bool { return a.ID == id })
}

func (t *fakeOperatorTx) ByCode(_ context.Context, code string) (domain.OperatorAccount, error) {
	t.op("ByCode")
	return t.find(func(a domain.OperatorAccount) bool { return a.Code == code })
}

func (t *fakeOperatorTx) ByEmail(_ context.Context, email string) (domain.OperatorAccount, error) {
	t.op("ByEmail")
	return t.find(func(a domain.OperatorAccount) bool {
		return strings.EqualFold(a.Email, strings.TrimSpace(email))
	})
}

func (t *fakeOperatorTx) Credentials(_ context.Context, id string) (operatorstore.Credentials, error) {
	t.op("Credentials")
	c, ok := t.st.creds[id]
	if !ok {
		return operatorstore.Credentials{}, operatorstore.ErrNotFound
	}
	return operatorstore.Credentials{PasswordHash: c.passwordHash, TOTPSecretSealed: c.totpSealed,
		PendingTOTPSecretSealed: c.pendingTOTP, TOTPLastStep: c.lastStep}, nil
}

func (t *fakeOperatorTx) CreateAccount(_ context.Context, in operatorstore.NewAccount, now time.Time) (domain.OperatorAccount, error) {
	t.op("CreateAccount")
	if _, err := t.find(func(a domain.OperatorAccount) bool { return strings.EqualFold(a.Email, in.Email) }); err == nil {
		return domain.OperatorAccount{}, operatorstore.ErrEmailTaken
	}
	t.st.nextCode++
	code, _ := domain.FormatOperatorCode(t.st.nextCode)
	exp := now.Add(domain.TemporaryPasswordLifetime)
	a := domain.OperatorAccount{ID: fmt.Sprintf("01JFAKE%019d", t.st.nextCode), Code: code, Email: in.Email,
		DisplayName: in.DisplayName, MustChangePassword: true, CreatedAt: now, CreatedBy: in.CreatedBy, UpdatedAt: now,
		TemporaryPasswordExpiresAt: &exp}
	t.st.accounts[a.ID] = a
	t.st.creds[a.ID] = fakeCreds{passwordHash: in.PasswordHash}
	return a, nil
}

func (t *fakeOperatorTx) SetPendingTOTP(_ context.Context, id string, sealed []byte, now time.Time) error {
	t.op("SetPendingTOTP")
	a, c := t.st.accounts[id], t.st.creds[id]
	if c.totpSealed != nil || a.Disabled() || !a.MustChangePassword || a.TemporaryPasswordExpiredAt(now) {
		return operatorstore.ErrStateConflict
	}
	c.pendingTOTP = sealed
	n := now
	a.TOTPPending, a.PendingTOTPCreatedAt = true, &n
	t.st.creds[id], t.st.accounts[id] = c, a
	return nil
}

func (t *fakeOperatorTx) revokeAll(id, reason string) {
	for sid, s := range t.st.sessions {
		if s.accountID == id && !s.revoked {
			s.revoked, s.reason = true, reason
			t.st.sessions[sid] = s
		}
	}
}

func (t *fakeOperatorTx) ActivateTOTP(_ context.Context, id, hash string, step int64, proved []byte, now time.Time) error {
	t.op("ActivateTOTP")
	a, c := t.st.accounts[id], t.st.creds[id]
	if c.pendingTOTP == nil || !bytes.Equal(c.pendingTOTP, proved) || a.Disabled() ||
		a.PendingTOTPExpiredAt(now) || !a.MustChangePassword || a.TemporaryPasswordExpiredAt(now) {
		return operatorstore.ErrStateConflict
	}
	c.totpSealed, c.pendingTOTP, c.lastStep, c.passwordHash = c.pendingTOTP, nil, &step, hash
	n := now
	a.TOTPEnrolledAt, a.TOTPPending, a.MustChangePassword = &n, false, false
	a.PendingTOTPCreatedAt, a.TemporaryPasswordExpiresAt = nil, nil
	t.st.creds[id], t.st.accounts[id] = c, a
	t.revokeAll(id, operatorstore.RevokeReasonPasswordChanged)
	return nil
}

func (t *fakeOperatorTx) RecordTOTPStep(_ context.Context, id string, step int64, now time.Time) (bool, error) {
	t.op("RecordTOTPStep")
	c := t.st.creds[id]
	if c.totpSealed == nil || (c.lastStep != nil && *c.lastStep >= step) || t.lockedAt(id, now) {
		return false, nil
	}
	c.lastStep = &step
	t.st.creds[id] = c
	return true, nil
}

func (t *fakeOperatorTx) RegisterFailure(_ context.Context, id string, now time.Time) (bool, time.Time, error) {
	t.op("RegisterFailure")
	a, ok := t.st.accounts[id]
	if !ok {
		return false, time.Time{}, operatorstore.ErrNotFound
	}
	if a.LockedAt(now) {
		return false, *a.LockedUntil, operatorstore.ErrAccountLocked
	}
	a.FailedAttempts++
	if a.FailedAttempts >= domain.MaxFailedAttempts {
		until := now.Add(domain.LockoutDuration)
		a.FailedAttempts, a.LockedUntil = 0, &until
		t.st.accounts[id] = a
		return true, until, nil
	}
	t.st.accounts[id] = a
	return false, time.Time{}, nil
}

func (t *fakeOperatorTx) ResetFailures(_ context.Context, id string, now time.Time) error {
	t.op("ResetFailures")
	if t.lockedAt(id, now) {
		return operatorstore.ErrAccountLocked
	}
	a := t.st.accounts[id]
	a.FailedAttempts, a.LockedUntil = 0, nil
	t.st.accounts[id] = a
	return nil
}

func (t *fakeOperatorTx) SetPassword(_ context.Context, id, hash string, mustChange bool, now time.Time) error {
	t.op("SetPassword")
	a, c := t.st.accounts[id], t.st.creds[id]
	c.passwordHash, a.MustChangePassword, a.TemporaryPasswordExpiresAt = hash, mustChange, nil
	if mustChange {
		exp := now.Add(domain.TemporaryPasswordLifetime)
		a.TemporaryPasswordExpiresAt = &exp
	}
	t.st.creds[id], t.st.accounts[id] = c, a
	t.revokeAll(id, operatorstore.RevokeReasonPasswordChanged)
	return nil
}

func (t *fakeOperatorTx) UpgradePasswordHash(_ context.Context, id, oldHash, newHash string, _ time.Time) error {
	t.op("UpgradePasswordHash")
	c := t.st.creds[id]
	if c.passwordHash != oldHash {
		return operatorstore.ErrStateConflict
	}
	c.passwordHash = newHash
	t.st.creds[id] = c
	return nil
}

func (t *fakeOperatorTx) Disable(_ context.Context, id, by, reason string, now time.Time) error {
	t.op("Disable")
	a := t.st.accounts[id]
	if a.Disabled() {
		return operatorstore.ErrStateConflict
	}
	n := now
	a.DisabledAt, a.DisabledBy, a.DisabledReason = &n, by, reason
	t.st.accounts[id] = a
	t.revokeAll(id, operatorstore.RevokeReasonDisabled)
	return nil
}

func (t *fakeOperatorTx) Unlock(_ context.Context, id string, _ time.Time) error {
	t.op("Unlock")
	a, ok := t.st.accounts[id]
	if !ok {
		return operatorstore.ErrNotFound
	}
	a.FailedAttempts, a.LockedUntil = 0, nil
	t.st.accounts[id] = a
	return nil
}

func (t *fakeOperatorTx) RevokeAllSessions(_ context.Context, id, reason string, _ time.Time) (int64, error) {
	t.op("RevokeAllSessions")
	var n int64
	for _, s := range t.st.sessions {
		if s.accountID == id && !s.revoked {
			n++
		}
	}
	t.revokeAll(id, reason)
	return n, nil
}

// idle mirrors the SQL `coalesce(last_seen_at, created_at) > now − SessionIdleTimeout`, negated.
func idle(s fakeSession, now time.Time) bool {
	return !s.lastSeen.After(now.Add(-domain.SessionIdleTimeout))
}

func (t *fakeOperatorTx) Enable(_ context.Context, id string, _ time.Time) error {
	t.op("Enable")
	a := t.st.accounts[id]
	if !a.Disabled() {
		return operatorstore.ErrStateConflict
	}
	a.DisabledAt, a.DisabledBy, a.DisabledReason = nil, "", ""
	t.st.accounts[id] = a
	return nil
}

func (t *fakeOperatorTx) ResetMFA(_ context.Context, id string, _ time.Time) error {
	t.op("ResetMFA")
	a, c := t.st.accounts[id], t.st.creds[id]
	c.totpSealed, c.pendingTOTP, c.lastStep = nil, nil, nil
	a.TOTPEnrolledAt, a.TOTPPending, a.PendingTOTPCreatedAt = nil, false, nil
	t.st.creds[id], t.st.accounts[id] = c, a
	for i := range t.st.codes {
		if t.st.codes[i].accountID == id && !t.st.codes[i].used {
			t.st.codes[i].voided = true
		}
	}
	t.revokeAll(id, operatorstore.RevokeReasonMFAReset)
	return nil
}

func (t *fakeOperatorTx) CreateSession(_ context.Context, sid, id, _, _ string, now time.Time) (time.Time, error) {
	t.op("CreateSession")
	exp := now.Add(domain.SessionLifetime)
	t.st.sessions[sid] = fakeSession{accountID: id, created: now, expires: exp, lastSeen: now}
	return exp, nil
}

func (t *fakeOperatorTx) CheckSession(_ context.Context, sid string, now time.Time) (operatorstore.OperatorSession, error) {
	t.op("CheckSession")
	s, ok := t.st.sessions[sid]
	if !ok || s.revoked || !now.Before(s.expires) || t.st.accounts[s.accountID].Disabled() || idle(s, now) {
		return operatorstore.OperatorSession{}, operatorstore.ErrNotFound
	}
	return operatorstore.OperatorSession{AccountID: s.accountID, AccountCode: t.st.accounts[s.accountID].Code,
		CreatedAt: s.created, ExpiresAt: s.expires}, nil
}

func (t *fakeOperatorTx) RevokeSession(_ context.Context, sid, reason string, _ time.Time) error {
	t.op("RevokeSession")
	s, ok := t.st.sessions[sid]
	if !ok || s.revoked {
		return operatorstore.ErrNotFound
	}
	s.revoked, s.reason = true, reason
	t.st.sessions[sid] = s
	return nil
}

func (t *fakeOperatorTx) TouchSession(_ context.Context, sid string, now time.Time) error {
	t.op("TouchSession")
	s, ok := t.st.sessions[sid]
	if !ok || s.revoked || !now.Before(s.expires) || idle(s, now) {
		return operatorstore.ErrNotFound
	}
	s.lastSeen = now
	t.st.sessions[sid] = s
	return nil
}

func (t *fakeOperatorTx) Grant(_ context.Context, id string, key domain.OperatorPermission, by, reason string, _ time.Time) error {
	t.op("Grant")
	if !key.Valid() || by == "" || strings.TrimSpace(reason) == "" {
		return errors.New("fake: bad grant")
	}
	for _, g := range t.st.grants {
		if g.accountID == id && g.key == key && !g.revoked {
			return operatorstore.ErrAlreadyGranted
		}
	}
	t.st.grants = append(t.st.grants, fakeGrant{accountID: id, key: key})
	t.revokeAll(id, operatorstore.RevokeReasonGrantChanged)
	return nil
}

func (t *fakeOperatorTx) Revoke(_ context.Context, id string, key domain.OperatorPermission, _, _ string, _ time.Time) error {
	t.op("Revoke")
	for i, g := range t.st.grants {
		if g.accountID == id && g.key == key && !g.revoked {
			t.st.grants[i].revoked = true
			t.revokeAll(id, operatorstore.RevokeReasonGrantChanged)
			return nil
		}
	}
	return operatorstore.ErrNotGranted
}

func (t *fakeOperatorTx) ActivePermissions(_ context.Context, id string) ([]domain.OperatorPermission, error) {
	t.op("ActivePermissions")
	return activeKeys(t.st, id), nil
}

func (t *fakeOperatorTx) ReplaceRecoveryCodes(_ context.Context, id string, digests [][]byte, _ time.Time) (string, error) {
	t.op("ReplaceRecoveryCodes")
	if len(digests) != domain.RecoveryCodeCount {
		return "", operatorstore.ErrRecoveryCodeSet
	}
	for i := range t.st.codes {
		if t.st.codes[i].accountID == id && !t.st.codes[i].used {
			t.st.codes[i].voided = true
		}
	}
	for _, d := range digests {
		t.st.codes = append(t.st.codes, fakeCode{accountID: id, digest: string(d)})
	}
	return "batch", nil
}

func (t *fakeOperatorTx) UseRecoveryCode(_ context.Context, id string, digest []byte, now time.Time) (bool, error) {
	t.op("UseRecoveryCode")
	if t.lockedAt(id, now) {
		return false, nil
	}
	for i, c := range t.st.codes {
		if c.accountID == id && c.digest == string(digest) && !c.used && !c.voided {
			t.st.codes[i].used = true
			return true, nil
		}
	}
	return false, nil
}

func (t *fakeOperatorTx) AppendAudit(_ context.Context, e domain.OperatorAuditEntry) error {
	t.op("AppendAudit:" + string(e.Action))
	if err := e.Validate(); err != nil {
		return err
	}
	if t.failAudit != "" && e.Action == t.failAudit {
		return errors.New("fake: audit write failed")
	}
	t.st.audit = append(t.st.audit, e)
	return nil
}

// --- harness -------------------------------------------------------------------------------------

// Fake material. The email uses the reserved example.test domain; nothing here is a real person.
const (
	opEmail = "operator.one@example.test"
	opName  = "Operator One"
	opIP    = "203.0.113.7"
	opUA    = "test-agent/1.0"
	opNewPw = "a-new-password-for-tests-only"
)

var (
	opSignKey = secret.Secret("op-sign-key-FAKE-NOT-A-REAL-KEY-for-app-tests")
	opSealKey = secret.Secret("seal-key-FAKE-NOT-REAL-for-apps!")
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time { return c.t }

type opHarness struct {
	store *fakeOperatorStore
	clock *fakeClock
	auth  *OperatorAuth
	admin *OperatorAdmin
	logs  *bytes.Buffer
}

func newOpHarness(t *testing.T) *opHarness {
	t.Helper()
	signer, err := operatorauth.NewTokenSigner([]secret.Secret{opSignKey})
	if err != nil {
		t.Fatal(err)
	}
	sealer, err := operatorauth.NewSealer([]secret.Secret{opSealKey})
	if err != nil {
		t.Fatal(err)
	}
	h := &opHarness{store: newFakeOperatorStore(),
		clock: &fakeClock{t: time.Date(2026, 9, 28, 8, 0, 0, 0, time.UTC)}, logs: &bytes.Buffer{}}
	log := slog.New(slog.NewJSONHandler(h.logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	h.auth = NewOperatorAuth(h.store, signer, sealer, h.clock.now, log)
	h.admin = NewOperatorAdmin(h.store, h.clock.now, log)
	return h
}

// create makes an operator through the real admin use case and returns code + temporary password.
func (h *opHarness) create(t *testing.T) (string, string) {
	t.Helper()
	c, err := h.admin.CreateOperator(context.Background(), opEmail, opName, "OPS-1")
	if err != nil {
		t.Fatal(err)
	}
	return c.Code, string(c.TemporaryPassword.Lo())
}

// totpSecretOf opens the sealed secret (active or pending) the way the use case does.
func (h *opHarness) totpSecretOf(t *testing.T, code string, pending bool) secret.Secret {
	t.Helper()
	acc := h.store.account(t, code)
	c := h.store.state.creds[acc.ID]
	sealed := c.totpSealed
	if pending {
		sealed = c.pendingTOTP
	}
	s, err := h.auth.sealer.Open(sealed, []byte(acc.ID))
	if err != nil {
		t.Fatalf("open sealed secret: %v", err)
	}
	return s
}

// enrolled creates an operator and completes enrolment. Returns code, password, recovery codes and
// the session token. The clock is advanced one TOTP step so the next code is fresh.
func (h *opHarness) enrolled(t *testing.T) (code, pw string, recovery []secret.Secret, token string) {
	t.Helper()
	code, temp := h.create(t)
	ctx := context.Background()
	if _, err := h.auth.BeginEnrollment(ctx, OperatorEnrollmentRequest{Email: opEmail, TemporaryPassword: temp, IP: opIP}); err != nil {
		t.Fatalf("begin: %v", err)
	}
	s := h.totpSecretOf(t, code, true)
	res, err := h.auth.CompleteEnrollment(ctx, OperatorEnrollmentCompletion{Email: opEmail, TemporaryPassword: temp,
		NewPassword: opNewPw, TOTPCode: operatorauth.CodeAt(s, h.clock.t), IP: opIP, UserAgent: opUA})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	h.clock.t = h.clock.t.Add(operatorauth.TOTPPeriod)
	return code, opNewPw, res.RecoveryCodes, res.Token
}

// currentCode is the code the operator's authenticator shows now.
func (h *opHarness) currentCode(t *testing.T, code string) string {
	t.Helper()
	return operatorauth.CodeAt(h.totpSecretOf(t, code, false), h.clock.t)
}

// wrongCode is a well-formed code that is not valid at any of the three accepted steps.
func (h *opHarness) wrongCode(t *testing.T, code string) string {
	t.Helper()
	s := h.totpSecretOf(t, code, false)
	for i := 0; i < 1000000; i++ {
		c := fmt.Sprintf("%06d", i)
		if _, ok := operatorauth.Verify(s, c, h.clock.t); !ok {
			return c
		}
	}
	t.Fatal("no wrong code found")
	return ""
}
