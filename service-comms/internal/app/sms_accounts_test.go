package app

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/crypto"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	"github.com/vihat/vigov/service-comms/internal/esms"
	docstore "github.com/vihat/vigov/service-comms/internal/store"
)

// The commune's SMS accounts (0032) over the recording fake driver, the REAL stores and the REAL commune
// envelope: what is proved is the order (eSMS before any write; nothing written when eSMS refuses), the
// transaction shape, what is sealed with which binding, what is audited, and that no key ever leaves.
// What PostgreSQL decides — the unique keys, the guard — is the store's pg test and the migration's.

var smsClock = time.Date(2026, 10, 17, 2, 0, 0, 0, time.UTC)

// Fixtures, not credentials.
const (
	smsAPIKey     = "FAKE-esms-api-key-NOT-REAL"
	smsSecretKey  = "FAKE-esms-secret-key-NOT-REAL"
	smsAPIKey2    = "FAKE-esms-api-key-2-NOT-REAL"
	smsRowID      = "01JSMSROW00000000000000001"
	smsRowNewID   = "01JSMSROW00000000000000002"
	smsTplNewID   = "01JSMSTPL00000000000000009"
	smsBrandname  = "XaMau"
	smsOldBalance = "1000.00"
)

// fakeGateway is a fake eSMS. It records the credentials presented and how many transactions had begun
// when it was asked (eSMS must be asked OUTSIDE any transaction).
type fakeGateway struct {
	f                  *sqlFake
	balance            string
	balanceOutcome     esms.Outcome
	templates          []esms.Template
	templatesOutcome   esms.Outcome
	calls              int
	presented          []string // "api|secret|brandname"
	begunWhenAsked     []int
	onCall             func()
	templateBrandnames []string
}

func (g *fakeGateway) note(c esms.Credentials, brandname string) {
	g.calls++
	g.presented = append(g.presented, string(c.APIKey.Lo())+"|"+string(c.SecretKey.Lo())+"|"+brandname)
	if g.f != nil {
		g.f.mu.Lock()
		g.begunWhenAsked = append(g.begunWhenAsked, g.f.begun)
		g.f.mu.Unlock()
	}
	if g.onCall != nil {
		g.onCall()
	}
}

func (g *fakeGateway) GetBalance(_ context.Context, c esms.Credentials) (string, esms.Outcome) {
	g.note(c, "")
	if g.balanceOutcome != esms.OutcomeOK {
		return "", g.balanceOutcome
	}
	return g.balance, esms.OutcomeOK
}

func (g *fakeGateway) ListCSKHTemplates(_ context.Context, c esms.Credentials, brandname string) ([]esms.Template, esms.Outcome) {
	g.note(c, brandname)
	g.templateBrandnames = append(g.templateBrandnames, brandname)
	if g.templatesOutcome != esms.OutcomeOK {
		return nil, g.templatesOutcome
	}
	return g.templates, esms.OutcomeOK
}

// smsRow is the fake's one sms_account row (nil = none live).
type smsRow struct {
	label, brandname     string
	isDefault            bool
	setAt                time.Time
	apiSealed, secSealed []byte
}

type smsTplRow struct {
	id, tempID, content string
	network             int
	vanished            bool
}

type smsRig struct {
	f         *sqlFake
	env       *crypto.Envelope
	gw        *fakeGateway
	uc        *SmsAccounts
	ctx       context.Context
	row       *smsRow
	templates []smsTplRow
	logs      *bytes.Buffer
}

func smsAccountCols(sealed bool) []string {
	c := []string{"id", "provider", "label", "brandname", "is_default", "last_check_at", "last_check_result",
		"last_balance", "set_at", "set_by", "created_at"}
	if sealed {
		c = append(c, "api_key_sealed", "secret_key_sealed")
	}
	return c
}

func (r *smsRig) accountValues(sealed bool) []driver.Value {
	v := []driver.Value{smsRowID, "esms", r.row.label, r.row.brandname, r.row.isDefault, smsClock.Add(-time.Hour),
		domain.SmsCheckOK, smsOldBalance, r.row.setAt, "CB-00099", smsClock.Add(-48 * time.Hour)}
	if sealed {
		v = append(v, r.row.apiSealed, r.row.secSealed)
	}
	return v
}

func newSmsRig(t *testing.T) *smsRig {
	t.Helper()
	r := &smsRig{env: testEnvelope(t), ctx: tenant.Into(context.Background(), xaA), logs: &bytes.Buffer{}}
	r.f = &sqlFake{query: func(q string, args []driver.Value) ([]string, [][]driver.Value, error) {
		switch {
		case strings.Contains(q, "FROM sms_account"):
			sealed := strings.Contains(q, "api_key_sealed")
			// Every read is scoped: $1 is the commune, $2 (when present) the row id.
			if len(args) == 0 || args[0] != string(xaA) || r.row == nil {
				return smsAccountCols(sealed), nil, nil
			}
			if len(args) > 1 && args[1] != smsRowID {
				return smsAccountCols(sealed), nil, nil
			}
			return smsAccountCols(sealed), [][]driver.Value{r.accountValues(sealed)}, nil
		case strings.Contains(q, "FROM sms_template"):
			cols := []string{"id", "account_id", "provider_template_id", "network_id", "content", "params", "synced_at", "vanished_at"}
			var rows [][]driver.Value
			for _, tpl := range r.templates {
				if tpl.vanished && strings.Contains(q, "vanished_at IS NULL") {
					continue
				}
				var vanished driver.Value
				if tpl.vanished {
					vanished = smsClock.Add(-time.Hour)
				}
				params, _ := json.Marshal([]map[string]any{})
				rows = append(rows, []driver.Value{tpl.id, smsRowID, tpl.tempID, int64(tpl.network), tpl.content, params,
					smsClock.Add(-2 * time.Hour), vanished})
			}
			return cols, rows, nil
		}
		return nil, nil, fmt.Errorf("fake: no answer for %q", q)
	}}
	r.gw = &fakeGateway{f: r.f, balance: "36693", balanceOutcome: esms.OutcomeOK, templatesOutcome: esms.OutcomeOK}
	db := sql.OpenDB(r.f)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	kho := store.New(db)
	r.uc = NewSmsAccounts(kho, docstore.NewSmsAccountStore(kho), docstore.NewSmsTemplateStore(kho), r.env, r.gw,
		slog.New(slog.NewJSONHandler(r.logs, nil)))
	ids := []string{smsRowNewID, smsTplNewID}
	r.uc.newID = func() (string, error) {
		id := ids[0]
		if len(ids) > 1 {
			ids = ids[1:]
		}
		return id, nil
	}
	r.uc.now = func() time.Time { return smsClock }
	return r
}

func (r *smsRig) seal(t *testing.T, plain string, aad func(context.Context, string) []byte) []byte {
	t.Helper()
	s, err := r.env.Seal(r.ctx, secret.Secret(plain), aad(r.ctx, smsRowID))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func (r *smsRig) withAccount(t *testing.T, isDefault bool) {
	t.Helper()
	r.row = &smsRow{label: "eSMS chính", brandname: smsBrandname, isDefault: isDefault, setAt: smsClock.Add(-24 * time.Hour),
		apiSealed: r.seal(t, smsAPIKey, smsAPIKeyAAD), secSealed: r.seal(t, smsSecretKey, smsSecretKeyAAD)}
}

func createInput() CreateSmsAccountInput {
	return CreateSmsAccountInput{Label: " eSMS chính ", APIKey: secret.Secret(smsAPIKey), SecretKey: secret.Secret(smsSecretKey),
		Brandname: smsBrandname}
}

// noKeyLeaves asserts no key appears in any audit delta, any statement argument other than the sealed
// columns, or the security log.
func (r *smsRig) noKeyLeaves(t *testing.T) {
	t.Helper()
	var all strings.Builder
	for _, s := range r.f.with("") {
		for _, a := range s.args {
			if b, ok := a.([]byte); ok {
				all.Write(b)
			} else {
				fmt.Fprint(&all, a)
			}
			all.WriteString(" ")
		}
	}
	all.WriteString(r.logs.String())
	for _, k := range []string{smsAPIKey, smsSecretKey, smsAPIKey2} {
		if strings.Contains(all.String(), k) {
			t.Fatalf("key %q left in a statement or a log", k)
		}
	}
	for _, e := range r.f.with("INSERT INTO audit_log") {
		if strings.Contains(string(e.args[7].([]byte)), "sealed") {
			t.Fatalf("an entry names a sealed column: %s", e.args[7])
		}
	}
}

func smsAuditDelta(t *testing.T, s sqlStmt) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(s.args[7].([]byte), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// --- Create ---------------------------------------------------------------------------------------------

func TestSmsCreateAsksEsmsFirstThenSealsAndAuditsInOneTransaction(t *testing.T) {
	r := newSmsRig(t)
	a, err := r.uc.CreateAccount(r.ctx, createInput(), staffActor)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	// eSMS asked twice (balance, Brandname's templates), both before any transaction.
	if r.gw.calls != 2 || r.gw.begunWhenAsked[0] != 0 || r.gw.begunWhenAsked[1] != 0 ||
		r.gw.presented[1] != smsAPIKey+"|"+smsSecretKey+"|"+smsBrandname {
		t.Fatalf("gateway = %d calls %v %v", r.gw.calls, r.gw.begunWhenAsked, r.gw.presented)
	}
	if r.f.begun != 1 || r.f.commits != 1 || r.f.rollbacks != 0 {
		t.Fatalf("tx %d/%d/%d", r.f.begun, r.f.commits, r.f.rollbacks)
	}
	if a.ID != smsRowNewID || a.Label != "eSMS chính" || a.LastBalance != "36693" || a.SetBy != "CB-00123" ||
		a.LastCheckResult != domain.SmsCheckOK || a.IsDefault {
		t.Fatalf("account = %+v", a)
	}
	ins := r.f.with("INSERT INTO sms_account")
	if len(ins) != 1 {
		t.Fatalf("%d inserts", len(ins))
	}
	args := ins[0].args
	if args[0] != string(xaA) || args[1] != smsRowNewID || args[2] != "esms" || args[6] != smsBrandname ||
		args[10] != "36693" || args[12] != "CB-00123" {
		t.Fatalf("insert args = %v", args)
	}
	if len(r.f.with("SET is_default = false")) != 0 {
		t.Error("the default was cleared for a non-default account")
	}
	// Each key opens under ITS binding, this row, this commune — and under nothing else.
	apiSealed, _ := args[4].([]byte)
	secSealed, _ := args[5].([]byte)
	if p, err := r.env.Open(r.ctx, apiSealed, smsAPIKeyAAD(r.ctx, smsRowNewID)); err != nil || string(p.Lo()) != smsAPIKey {
		t.Fatalf("ApiKey does not open back: %v", err)
	}
	if p, err := r.env.Open(r.ctx, secSealed, smsSecretKeyAAD(r.ctx, smsRowNewID)); err != nil || string(p.Lo()) != smsSecretKey {
		t.Fatalf("SecretKey does not open back: %v", err)
	}
	if _, err := r.env.Open(r.ctx, apiSealed, smsSecretKeyAAD(r.ctx, smsRowNewID)); err == nil {
		t.Error("the ApiKey opened as a SecretKey")
	}
	if _, err := r.env.Open(r.ctx, apiSealed, smsAPIKeyAAD(r.ctx, smsRowID)); err == nil {
		t.Error("the ApiKey opened under another row")
	}
	ctxB := tenant.Into(context.Background(), xaB)
	if _, err := r.env.Open(ctxB, apiSealed, smsAPIKeyAAD(ctxB, smsRowNewID)); err == nil {
		t.Error("commune A's ApiKey opened for commune B")
	}
	entries := r.f.with("INSERT INTO audit_log")
	if len(entries) != 1 || entries[0].args[4] != domain.ActionCreateSmsAccount ||
		entries[0].args[5] != "sms-account/"+smsRowNewID || entries[0].args[1] != "CB-00123" {
		t.Fatalf("entries = %v", entries)
	}
	if indexOfStmt(r.f, "INSERT INTO audit_log") < indexOfStmt(r.f, "INSERT INTO sms_account") {
		t.Error("the entry was written before the row")
	}
	r.noKeyLeaves(t)
}

func TestSmsCreateAsDefaultClearsTheOldDefaultFirst(t *testing.T) {
	r := newSmsRig(t)
	in := createInput()
	in.IsDefault = true
	if _, err := r.uc.CreateAccount(r.ctx, in, staffActor); err != nil {
		t.Fatal(err)
	}
	clr, ins := indexOfStmt(r.f, "SET is_default = false"), indexOfStmt(r.f, "INSERT INTO sms_account")
	if clr < 0 || clr > ins {
		t.Fatalf("clear at %d, insert at %d", clr, ins)
	}
	if c := r.f.with("SET is_default = false")[0]; c.args[0] != string(xaA) {
		t.Fatalf("clear not scoped: %v", c.args)
	}
}

func TestSmsCreateRefusedByEsmsWritesNothing(t *testing.T) {
	for name, tc := range map[string]struct {
		balance, templates esms.Outcome
		class              string
	}{
		"wrong keys":       {esms.OutcomeKeyRejected, esms.OutcomeOK, domain.SmsCheckKeyRejected},
		"brandname":        {esms.OutcomeOK, esms.OutcomeBrandnameInactive, domain.SmsCheckBrandnameInactive},
		"unreachable":      {esms.OutcomeUnavailable, esms.OutcomeOK, domain.SmsCheckUnavailable},
		"account locked":   {esms.OutcomeRejected, esms.OutcomeOK, domain.SmsCheckRejected},
		"unreadable reply": {esms.OutcomeMalformedResponse, esms.OutcomeOK, domain.SmsCheckMalformed},
	} {
		t.Run(name, func(t *testing.T) {
			r := newSmsRig(t)
			r.gw.balanceOutcome, r.gw.templatesOutcome = tc.balance, tc.templates
			_, err := r.uc.CreateAccount(r.ctx, createInput(), staffActor)
			var pe *SmsProviderError
			if !errors.As(err, &pe) || pe.Class != tc.class {
				t.Fatalf("err = %v, want class %s", err, tc.class)
			}
			if r.f.begun != 0 || len(r.f.with("")) != 0 {
				t.Fatalf("%d transactions, %d statements after a refusal", r.f.begun, len(r.f.with("")))
			}
			r.noKeyLeaves(t)
		})
	}
}

func TestSmsCreateRefusesBeforeAskingEsms(t *testing.T) {
	for name, mut := range map[string]func(*CreateSmsAccountInput){
		"label":      func(in *CreateSmsAccountInput) { in.Label = " " },
		"brandname":  func(in *CreateSmsAccountInput) { in.Brandname = "Xã" },
		"no secret":  func(in *CreateSmsAccountInput) { in.SecretKey = nil },
		"key spaces": func(in *CreateSmsAccountInput) { in.APIKey = secret.Secret("has space") },
	} {
		t.Run(name, func(t *testing.T) {
			r := newSmsRig(t)
			in := createInput()
			mut(&in)
			if _, err := r.uc.CreateAccount(r.ctx, in, staffActor); !errors.Is(err, domain.ErrSmsAccountInvalid) {
				t.Fatalf("err = %v", err)
			}
			if r.gw.calls != 0 {
				t.Fatal("eSMS asked about an invalid request")
			}
		})
	}
	r := newSmsRig(t)
	r.uc.envelope = nil
	if _, err := r.uc.CreateAccount(r.ctx, createInput(), staffActor); !errors.Is(err, crypto.ErrNotConfigured) || r.gw.calls != 0 {
		t.Fatalf("no KEK: %v, %d calls", err, r.gw.calls)
	}
}

// --- Update ---------------------------------------------------------------------------------------------

func TestSmsUpdateLabelOnlyNeverAsksEsms(t *testing.T) {
	r := newSmsRig(t)
	r.withAccount(t, false)
	label := "eSMS dự phòng"
	a, err := r.uc.UpdateAccount(r.ctx, smsRowID, UpdateSmsAccountInput{Label: &label}, staffActor)
	if err != nil {
		t.Fatal(err)
	}
	if r.gw.calls != 0 || a.Label != label || a.SetBy != "CB-00099" {
		t.Fatalf("calls %d, account %+v", r.gw.calls, a)
	}
	if len(r.f.with("UPDATE sms_account SET label = $3, updated_at")) != 1 || len(r.f.with("api_key_sealed =")) != 0 {
		t.Fatal("wrong statement")
	}
	e := r.f.with("INSERT INTO audit_log")
	if len(e) != 1 || e[0].args[4] != domain.ActionUpdateSmsAccount {
		t.Fatalf("entries = %v", e)
	}
	d := smsAuditDelta(t, e[0])
	if d["api_key_replaced"] != false || d["sau"].(map[string]any)["label"] != label ||
		d["truoc"].(map[string]any)["label"] != "eSMS chính" {
		t.Fatalf("delta = %v", d)
	}
}

func TestSmsUpdateNothingMovedWritesNothing(t *testing.T) {
	r := newSmsRig(t)
	r.withAccount(t, true)
	same, yes := "eSMS chính", true
	if _, err := r.uc.UpdateAccount(r.ctx, smsRowID, UpdateSmsAccountInput{Label: &same, IsDefault: &yes}, staffActor); err != nil {
		t.Fatal(err)
	}
	if r.f.begun != 0 || r.gw.calls != 0 {
		t.Fatalf("tx %d, calls %d", r.f.begun, r.gw.calls)
	}
}

func TestSmsUpdateOneNewKeyIsVerifiedWithTheStoredOtherAndResealed(t *testing.T) {
	r := newSmsRig(t)
	r.withAccount(t, false)
	oldSecret := r.row.secSealed
	a, err := r.uc.UpdateAccount(r.ctx, smsRowID, UpdateSmsAccountInput{APIKey: secret.Secret(smsAPIKey2)}, staffActor)
	if err != nil {
		t.Fatal(err)
	}
	// eSMS sees the NEW ApiKey with the STORED SecretKey, before any transaction.
	if r.gw.calls != 2 || r.gw.presented[0] != smsAPIKey2+"|"+smsSecretKey+"|" || r.gw.begunWhenAsked[1] != 0 {
		t.Fatalf("presented %v at %v", r.gw.presented, r.gw.begunWhenAsked)
	}
	up := r.f.with("api_key_sealed = $5")
	if len(up) != 1 {
		t.Fatalf("%d credential updates", len(up))
	}
	args := up[0].args
	if p, err := r.env.Open(r.ctx, args[4].([]byte), smsAPIKeyAAD(r.ctx, smsRowID)); err != nil || string(p.Lo()) != smsAPIKey2 {
		t.Fatalf("new ApiKey does not open: %v", err)
	}
	if !bytes.Equal(args[5].([]byte), oldSecret) {
		t.Error("the kept SecretKey was resealed or replaced")
	}
	if args[7] != "CB-00123" || args[8] != domain.SmsCheckOK || args[9] != "36693" || a.SetBy != "CB-00123" {
		t.Fatalf("update args = %v", args[6:])
	}
	if d := smsAuditDelta(t, r.f.with("INSERT INTO audit_log")[0]); d["api_key_replaced"] != true || d["secret_key_replaced"] != false {
		t.Fatalf("delta = %v", d)
	}
	r.noKeyLeaves(t)
}

func TestSmsUpdateRefusedByEsmsKeepsTheOldKeys(t *testing.T) {
	r := newSmsRig(t)
	r.withAccount(t, false)
	r.gw.balanceOutcome = esms.OutcomeKeyRejected
	_, err := r.uc.UpdateAccount(r.ctx, smsRowID, UpdateSmsAccountInput{SecretKey: secret.Secret(smsAPIKey2)}, staffActor)
	var pe *SmsProviderError
	if !errors.As(err, &pe) || pe.Class != domain.SmsCheckKeyRejected || r.f.begun != 0 {
		t.Fatalf("err = %v, tx %d", err, r.f.begun)
	}
}

func TestSmsUpdateRefusesWhenTheKeysChangedMeanwhile(t *testing.T) {
	r := newSmsRig(t)
	r.withAccount(t, false)
	// Another administrator saves new keys while eSMS is being asked.
	r.gw.onCall = func() { r.row.setAt = smsClock.Add(-time.Minute) }
	brand := "XaMoi"
	_, err := r.uc.UpdateAccount(r.ctx, smsRowID, UpdateSmsAccountInput{Brandname: &brand}, staffActor)
	if !errors.Is(err, docstore.ErrSmsAccountChanged) || r.f.commits != 0 || r.f.rollbacks != 1 {
		t.Fatalf("err = %v, tx %d/%d", err, r.f.commits, r.f.rollbacks)
	}
	if r.gw.templateBrandnames[0] != "XaMoi" {
		t.Fatalf("the new Brandname was not the one checked: %v", r.gw.templateBrandnames)
	}
}

func TestSmsUpdateDefaultMovesInOneTransaction(t *testing.T) {
	r := newSmsRig(t)
	r.withAccount(t, false)
	yes := true
	if _, err := r.uc.UpdateAccount(r.ctx, smsRowID, UpdateSmsAccountInput{IsDefault: &yes}, staffActor); err != nil {
		t.Fatal(err)
	}
	clr, set := indexOfStmt(r.f, "SET is_default = false, updated_at = $2"), indexOfStmt(r.f, "SET is_default = true")
	if r.f.commits != 1 || clr < 0 || set < clr {
		t.Fatalf("commits %d, clear %d, set %d", r.f.commits, clr, set)
	}
	r2 := newSmsRig(t)
	r2.withAccount(t, true)
	no := false
	if _, err := r2.uc.UpdateAccount(r2.ctx, smsRowID, UpdateSmsAccountInput{IsDefault: &no}, staffActor); err != nil {
		t.Fatal(err)
	}
	if len(r2.f.with("SET is_default = false, updated_at = $3")) != 1 || len(r2.f.with("SET is_default = true")) != 0 {
		t.Fatal("unsetting the default did not write exactly one unset")
	}
}

func TestSmsUpdateUnknownIsNotFound(t *testing.T) {
	r := newSmsRig(t)
	label := "x"
	if _, err := r.uc.UpdateAccount(r.ctx, smsRowID, UpdateSmsAccountInput{Label: &label}, staffActor); !errors.Is(err, ErrSmsAccountNotFound) {
		t.Fatalf("err = %v", err)
	}
}

// --- Delete ---------------------------------------------------------------------------------------------

func TestSmsDeleteRetiresWithReasonAndPicksNoNewDefault(t *testing.T) {
	r := newSmsRig(t)
	r.withAccount(t, true)
	if err := r.uc.RetireAccount(r.ctx, smsRowID, " Hết hợp đồng ", staffActor); err != nil {
		t.Fatal(err)
	}
	ret := r.f.with("SET retired_at")
	if len(ret) != 1 || ret[0].args[0] != string(xaA) || ret[0].args[3] != "CB-00123" || ret[0].args[4] != "Hết hợp đồng" ||
		!strings.Contains(ret[0].sql, "is_default = false") {
		t.Fatalf("retire = %v", ret)
	}
	if len(r.f.with("SET is_default = true")) != 0 {
		t.Fatal("another account was made the default")
	}
	if len(r.f.with("DELETE")) != 0 {
		t.Fatal("a DELETE statement ran")
	}
	e := r.f.with("INSERT INTO audit_log")
	if len(e) != 1 || e[0].args[4] != domain.ActionRetireSmsAccount || smsAuditDelta(t, e[0])["was_default"] != true {
		t.Fatalf("entries = %v", e)
	}
}

func TestSmsDeleteNeedsAReasonAndALiveRow(t *testing.T) {
	r := newSmsRig(t)
	r.withAccount(t, false)
	if err := r.uc.RetireAccount(r.ctx, smsRowID, "  ", staffActor); !errors.Is(err, domain.ErrSmsRetireReason) || r.f.begun != 0 {
		t.Fatalf("err = %v", err)
	}
	r.row = nil
	if err := r.uc.RetireAccount(r.ctx, smsRowID, "x", staffActor); !errors.Is(err, ErrSmsAccountNotFound) || r.f.rollbacks != 1 {
		t.Fatalf("err = %v, rollbacks %d", err, r.f.rollbacks)
	}
}

// --- Check ----------------------------------------------------------------------------------------------

func TestSmsCheckRecordsAFailingResultAsAResult(t *testing.T) {
	r := newSmsRig(t)
	r.withAccount(t, false)
	r.gw.templatesOutcome = esms.OutcomeBrandnameInactive
	out, err := r.uc.Check(r.ctx, smsRowID, staffActor)
	if err != nil {
		t.Fatal(err)
	}
	if out.Result != domain.SmsCheckBrandnameInactive || out.Balance != "" || !out.CheckedAt.Equal(smsClock) {
		t.Fatalf("out = %+v", out)
	}
	if r.gw.presented[0] != smsAPIKey+"|"+smsSecretKey+"|" || r.gw.begunWhenAsked[0] != 0 {
		t.Fatalf("presented %v", r.gw.presented)
	}
	rec := r.f.with("SET last_check_at")
	if len(rec) != 1 || rec[0].args[3] != domain.SmsCheckBrandnameInactive || rec[0].args[4] != nil {
		t.Fatalf("record = %v", rec)
	}
	e := r.f.with("INSERT INTO audit_log")
	if len(e) != 1 || e[0].args[4] != domain.ActionCheckSmsAccount {
		t.Fatalf("entries = %v", e)
	}
	after, _ := smsAuditDelta(t, e[0])["sau"].(map[string]any)
	if after["result"] != domain.SmsCheckBrandnameInactive {
		t.Fatalf("delta = %v", after)
	}
	if _, has := after["balance"]; has {
		t.Error("a failed check carries a balance")
	}
	r.noKeyLeaves(t)
}

func TestSmsCheckSuccessStoresTheBalance(t *testing.T) {
	r := newSmsRig(t)
	r.withAccount(t, false)
	out, err := r.uc.Check(r.ctx, smsRowID, staffActor)
	if err != nil || out.Result != domain.SmsCheckOK || out.Balance != "36693" {
		t.Fatalf("= %+v %v", out, err)
	}
	if rec := r.f.with("SET last_check_at"); rec[0].args[4] != "36693" {
		t.Fatalf("record = %v", rec[0].args)
	}
}

func TestSmsCheckFailsClosed(t *testing.T) {
	r := newSmsRig(t)
	if _, err := r.uc.Check(r.ctx, smsRowID, staffActor); !errors.Is(err, ErrSmsAccountNotFound) {
		t.Fatalf("no account: %v", err)
	}
	r.withAccount(t, false)
	r.uc.envelope = nil
	if _, err := r.uc.Check(r.ctx, smsRowID, staffActor); !errors.Is(err, crypto.ErrNotConfigured) || r.gw.calls != 0 {
		t.Fatalf("no KEK: %v", err)
	}
	// Sealed bytes of another row do not open (0032's row binding).
	r2 := newSmsRig(t)
	r2.withAccount(t, false)
	r2.row.apiSealed, _ = r2.env.Seal(r2.ctx, secret.Secret(smsAPIKey), smsAPIKeyAAD(r2.ctx, smsRowNewID))
	if _, err := r2.uc.Check(r2.ctx, smsRowID, staffActor); err == nil || r2.gw.calls != 0 {
		t.Fatalf("copied bytes opened: %v", err)
	}
}

// --- Templates ------------------------------------------------------------------------------------------

func TestSmsSyncUpsertsVanishesAndSkipsInOneTransaction(t *testing.T) {
	r := newSmsRig(t)
	r.withAccount(t, false)
	r.templates = []smsTplRow{
		{id: "01JTPLKEEP0000000000000001", tempID: "458", network: 6, content: "{P1,20} la ma"},
		{id: "01JTPLGONE0000000000000001", tempID: "999", network: 1, content: "Cu"},
		{id: "01JTPLWAS00000000000000001", tempID: "777", network: 2, content: "Da mat", vanished: true},
	}
	r.gw.templates = []esms.Template{
		{ID: "458", NetworkID: 6, Content: "{P1,20} la ma moi"},
		{ID: "12640", NetworkID: 2, Content: "Cam on quy khach"},
		{ID: "12.6", NetworkID: 2, Content: "TempId of another shape"},
	}
	r.uc.newID = func() (string, error) { return smsTplNewID, nil }
	out, err := r.uc.SyncTemplates(r.ctx, smsRowID, staffActor)
	if err != nil {
		t.Fatal(err)
	}
	if out.Added != 1 || out.Updated != 1 || out.Vanished != 1 || out.Skipped != 1 {
		t.Fatalf("out = %+v", out)
	}
	if r.gw.templateBrandnames[0] != smsBrandname || r.gw.begunWhenAsked[0] != 0 || r.f.commits != 1 {
		t.Fatalf("asked %v at %v, commits %d", r.gw.templateBrandnames, r.gw.begunWhenAsked, r.f.commits)
	}
	ups := r.f.with("INSERT INTO sms_template")
	if len(ups) != 2 || ups[0].args[1] != "01JTPLKEEP0000000000000001" || ups[1].args[1] != smsTplNewID ||
		ups[0].args[0] != string(xaA) {
		t.Fatalf("upserts = %v", ups)
	}
	var params []map[string]any
	if err := json.Unmarshal(ups[0].args[6].([]byte), &params); err != nil || len(params) != 1 || params[0]["name"] != "P1" ||
		params[0]["max_length"] != float64(20) {
		t.Fatalf("params = %s", ups[0].args[6])
	}
	van := r.f.with("SET vanished_at")
	if len(van) != 1 || van[0].args[1] != "01JTPLGONE0000000000000001" {
		t.Fatalf("vanished = %v", van)
	}
	e := r.f.with("INSERT INTO audit_log")
	if len(e) != 1 || e[0].args[4] != domain.ActionSyncSmsTemplates {
		t.Fatalf("entries = %v", e)
	}
	if d, _ := smsAuditDelta(t, e[0])["sau"].(map[string]any); d["added"] != float64(1) || d["skipped"] != float64(1) {
		t.Fatalf("delta = %v", d)
	}
	r.noKeyLeaves(t)
}

func TestSmsSyncRefusedByEsmsWritesNothing(t *testing.T) {
	r := newSmsRig(t)
	r.withAccount(t, false)
	r.gw.templatesOutcome = esms.OutcomeUnavailable
	_, err := r.uc.SyncTemplates(r.ctx, smsRowID, staffActor)
	var pe *SmsProviderError
	if !errors.As(err, &pe) || pe.Class != domain.SmsCheckUnavailable || r.f.begun != 0 {
		t.Fatalf("err = %v, tx %d", err, r.f.begun)
	}
}

func TestSmsListTemplatesGroupsByContent(t *testing.T) {
	r := newSmsRig(t)
	r.withAccount(t, false)
	r.templates = []smsTplRow{
		{id: "01JTPL00000000000000000001", tempID: "12640", network: 2, content: "Cam on quy khach"},
		{id: "01JTPL00000000000000000002", tempID: "12884", network: 1, content: "Cam on quy khach"},
		{id: "01JTPL00000000000000000003", tempID: "458", network: 6, content: "{P1,20} la ma"},
		{id: "01JTPL00000000000000000004", tempID: "777", network: 3, content: "Da mat", vanished: true},
	}
	groups, err := r.uc.ListTemplates(r.ctx, smsRowID)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 || groups[0].Content != "Cam on quy khach" || len(groups[0].Networks) != 2 ||
		groups[0].Networks[1] != (SmsTemplateNetwork{NetworkID: 1, ProviderTemplateID: "12884"}) || len(groups[1].Networks) != 1 {
		t.Fatalf("groups = %+v", groups)
	}
	if _, err := r.uc.ListTemplates(r.ctx, "01JSMSROWOTHER000000000001"); !errors.Is(err, ErrSmsAccountNotFound) {
		t.Fatalf("unknown account: %v", err)
	}
}

func TestSmsActsRefuseWithoutActor(t *testing.T) {
	r := newSmsRig(t)
	r.withAccount(t, false)
	if _, err := r.uc.CreateAccount(r.ctx, createInput(), staffActor0()); !errors.Is(err, ErrNoActor) {
		t.Fatalf("create: %v", err)
	}
	if err := r.uc.RetireAccount(r.ctx, smsRowID, "x", staffActor0()); !errors.Is(err, ErrNoActor) {
		t.Fatalf("delete: %v", err)
	}
	if _, err := r.uc.Check(r.ctx, smsRowID, staffActor0()); !errors.Is(err, ErrNoActor) {
		t.Fatalf("check: %v", err)
	}
	if r.gw.calls != 0 || r.f.begun != 0 {
		t.Fatal("an act without an actor reached eSMS or the database")
	}
}

// staffActor0 is an actor with no business code — what a route mounted without a principal would pass.
func staffActor0() audit.Actor { return audit.Actor{} }
