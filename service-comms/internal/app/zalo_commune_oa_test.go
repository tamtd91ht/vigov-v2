package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/vihat/vigov/core/crypto"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	docstore "github.com/vihat/vigov/service-comms/internal/store"
	"github.com/vihat/vigov/service-comms/internal/store/crosstenant"
	"github.com/vihat/vigov/service-comms/internal/zalooa"
)

// The commune OA (0031, ADR 0098 block A) over the recording fake driver, the REAL stores and the REAL
// commune envelope: what is proved is the transaction shape, what is sealed with which binding, what is
// audited by whom, and what never leaves. What PostgreSQL decides — the unique keys, the guards — is
// store/zalo_commune_oa_pg_test.go's.

// Compile-time: the production types satisfy the interfaces the use case and the job declare.
var (
	_ ZaloOAAPI                   = (*zalooa.Client)(nil)
	_ ZaloOAAuthorizationResolver = (*crosstenant.ZaloOA)(nil)
	_ ZaloOARefreshLocks          = (*crosstenant.ZaloOA)(nil)
	_ ZaloOATokenKeeper           = (*CommuneZaloOAs)(nil)
	_ ZaloOATemplateLookup        = (*docstore.ZaloOATemplateStore)(nil)
)

var oaClock = time.Date(2026, 10, 16, 2, 0, 0, 0, time.UTC)

// Every credential here is FAKE — the text says so (rule 8, forbidden #1).
const (
	oaAppID     = "1234567890"
	oaSecret    = "FAKE-zalo-app-secret-NOT-REAL"
	oaCode      = "FAKE-authorization-code-NOT-REAL"
	oaAccess    = "FAKE-access-token-NOT-REAL"
	oaRefresh   = "FAKE-refresh-token-NOT-REAL"
	oaVerifier  = "FAKEverifierFAKEverifierFAKEverifierFAKEv43"
	oaChallenge = "FAKE-challenge"
	oaRowID     = "01JOAROW000000000000000001"
	oaAuthID    = "01JOAAUTH00000000000000001"
	oaZaloID    = "579745863508352884"
	oaHost      = "xa-a.example.gov.vn"
	oaCallback  = "bot.api.example.vn"
)

var oaState = bytes.Repeat([]byte{0x42}, 32)

// ---- fake Zalo --------------------------------------------------------------------------------------------

type fakeOAAPI struct {
	exchange     zalooa.Outcome
	refresh      zalooa.Outcome
	expiresIn    time.Duration
	getoa        zalooa.OAInfo
	getoaOutcome zalooa.Outcome
	pages        [][]zalooa.TemplateSummary
	total        int
	listOutcome  zalooa.Outcome
	infos        map[string]zalooa.TemplateDetail
	infoOutcome  map[string]zalooa.Outcome
	calls        map[string]int
	gotVerifier  string
	gotRefresh   string
	gotCode      string
}

func newFakeOAAPI() *fakeOAAPI {
	return &fakeOAAPI{exchange: zalooa.OutcomeOK, refresh: zalooa.OutcomeOK, expiresIn: 90000 * time.Second,
		getoa: zalooa.OAInfo{OAID: oaZaloID, Name: "UBND Xã A"}, getoaOutcome: zalooa.OutcomeOK, total: -1,
		listOutcome: zalooa.OutcomeOK, infos: map[string]zalooa.TemplateDetail{}, infoOutcome: map[string]zalooa.Outcome{},
		calls: map[string]int{}}
}

// fresh copies: the use case clears every token it is handed.
func (z *fakeOAAPI) tokens(suffix string) zalooa.Tokens {
	return zalooa.Tokens{Access: secret.Secret(oaAccess + suffix), Refresh: secret.Secret(oaRefresh + suffix), ExpiresIn: z.expiresIn}
}

func (z *fakeOAAPI) ExchangeCode(_ context.Context, appID string, s secret.Secret, code string, v secret.Secret) (zalooa.Tokens, zalooa.Outcome) {
	z.calls["exchange"]++
	z.gotVerifier, z.gotCode = string(v.Lo()), code
	if appID != oaAppID || string(s.Lo()) != oaSecret {
		return zalooa.Tokens{}, zalooa.OutcomeTokenRejected
	}
	if z.exchange != zalooa.OutcomeOK {
		return zalooa.Tokens{}, z.exchange
	}
	return z.tokens(""), zalooa.OutcomeOK
}

func (z *fakeOAAPI) Refresh(_ context.Context, _ string, _ secret.Secret, r secret.Secret) (zalooa.Tokens, zalooa.Outcome) {
	z.calls["refresh"]++
	z.gotRefresh = string(r.Lo())
	if z.refresh != zalooa.OutcomeOK {
		return zalooa.Tokens{}, z.refresh
	}
	return z.tokens("-2"), zalooa.OutcomeOK
}

func (z *fakeOAAPI) GetOA(context.Context, secret.Secret) (zalooa.OAInfo, zalooa.Outcome) {
	z.calls["getoa"]++
	if z.getoaOutcome != zalooa.OutcomeOK {
		return zalooa.OAInfo{}, z.getoaOutcome
	}
	return z.getoa, zalooa.OutcomeOK
}

func (z *fakeOAAPI) ListTemplates(_ context.Context, _ secret.Secret, offset, _ int) ([]zalooa.TemplateSummary, int, zalooa.Outcome) {
	z.calls["list"]++
	if z.listOutcome != zalooa.OutcomeOK {
		return nil, 0, z.listOutcome
	}
	i := offset / zalooa.MaxTemplatePageSize
	if i >= len(z.pages) {
		return nil, z.total, zalooa.OutcomeOK
	}
	return z.pages[i], z.total, zalooa.OutcomeOK
}

func (z *fakeOAAPI) TemplateInfo(_ context.Context, _ secret.Secret, id string) (zalooa.TemplateDetail, zalooa.Outcome) {
	z.calls["info"]++
	if o, ok := z.infoOutcome[id]; ok && o != zalooa.OutcomeOK {
		return zalooa.TemplateDetail{}, o
	}
	return z.infos[id], zalooa.OutcomeOK
}

type fakeStateResolver struct {
	tid   tenant.ID
	id    string
	found bool
	asked int
	hash  []byte
}

func (r *fakeStateResolver) OpenAuthorizationByStateHash(_ context.Context, h []byte) (tenant.ID, string, bool, error) {
	r.asked++
	r.hash = append([]byte(nil), h...)
	return r.tid, r.id, r.found, nil
}

type oaRegistry struct{ host string }

func (g oaRegistry) XaTrongNguCanh(ctx context.Context) (tenant.Tenant, bool, error) { // vi-name-ok: the method of the existing PortalCommuneRegistry interface
	if g.host == "" {
		return tenant.Tenant{}, false, nil
	}
	return tenant.Tenant{ID: tenant.MustFrom(ctx), Host: g.host, Active: true}, true, nil
}

// ---- the rig -----------------------------------------------------------------------------------------------

type fakeOARow struct {
	id, oaID, name                        string
	accessExp, refreshIssued, refreshedAt time.Time
	reauthSince                           driver.Value
	reauthReason                          driver.Value
	access, refresh                       []byte
}

func (o *fakeOARow) values(sealed bool) []driver.Value {
	v := []driver.Value{o.id, o.oaID, o.name, o.accessExp, o.refreshIssued, o.refreshedAt, oaClock.Add(-48 * time.Hour),
		"CB-00099", nil, nil, o.reauthSince, o.reauthReason, nil, nil, nil}
	if sealed {
		v = append(v, o.access, o.refresh)
	}
	return v
}

var oaCols = []string{"id", "oa_id", "oa_name", "access_expires_at", "refresh_issued_at", "token_refreshed_at",
	"authorized_at", "authorized_by", "last_check_at", "last_check_result", "needs_reauthorization_since",
	"needs_reauthorization_reason", "retired_at", "retired_by", "retire_reason"}

type fakeAuthRow struct {
	expires  time.Time
	verifier []byte
}

type fakeTplRow struct {
	id, templateID, status string
	params                 []byte
	match                  bool
	milestone              driver.Value
}

func (t fakeTplRow) values() []driver.Value {
	var setAt, setBy driver.Value
	if t.milestone != nil {
		setAt, setBy = oaClock.Add(-time.Hour), "CB-00099"
	}
	return []driver.Value{t.id, oaRowID, t.templateID, "Mẫu " + t.templateID, t.status, "HIGH", "TRANSACTION", t.params,
		t.match, t.milestone, setAt, setBy, oaClock.Add(-time.Hour)}
}

const goodParams = `[{"name":"ma_tra_cuu","required":true,"type":"STRING","max_length":30},{"name":"moc_nhan"},{"name":"viec_tiep_theo"}]`

type oaRig struct {
	f        *sqlFake
	env      *crypto.Envelope
	zalo     *fakeOAAPI
	resolver *fakeStateResolver
	uc       *CommuneZaloOAs
	ctx      context.Context
	oa       *fakeOARow
	auth     *fakeAuthRow
	tpls     []fakeTplRow
	logs     *bytes.Buffer
	ids      int
}

func newOARig(t *testing.T) *oaRig {
	t.Helper()
	r := &oaRig{env: testEnvelope(t), zalo: newFakeOAAPI(), ctx: tenant.Into(context.Background(), xaA),
		resolver: &fakeStateResolver{tid: xaA, id: oaAuthID, found: true}}
	r.f = &sqlFake{query: func(q string, args []driver.Value) ([]string, [][]driver.Value, error) {
		switch {
		case strings.Contains(q, "FROM zalo_oa_template t"):
			return nil, nil, errors.New("fake: join not used by the use case")
		case strings.Contains(q, "FROM zalo_commune_oa"):
			sealed := strings.Contains(q, "access_token_sealed")
			cols := append([]string(nil), oaCols...)
			if sealed {
				cols = append(cols, "access_token_sealed", "refresh_token_sealed")
			}
			if r.oa == nil {
				return cols, nil, nil
			}
			return cols, [][]driver.Value{r.oa.values(sealed)}, nil
		case strings.Contains(q, "UPDATE zalo_oa_authorization SET used_at") && strings.Contains(q, "RETURNING id"):
			return []string{"id"}, [][]driver.Value{{"01JOAAUTHEXPIRED0000000001"}}, nil
		case strings.Contains(q, "FROM zalo_oa_authorization"):
			cols := []string{"id", "requested_at", "requested_by", "expires_at", "used_at", "outcome", "oa_row_id", "cv"}
			if r.auth == nil {
				return cols, nil, nil
			}
			return cols, [][]driver.Value{{oaAuthID, r.auth.expires.Add(-domain.AuthorizationTTL), "CB-00123", r.auth.expires,
				nil, nil, nil, r.auth.verifier}}, nil
		case strings.Contains(q, "UPDATE zalo_oa_template SET milestone = NULL"):
			var rows [][]driver.Value
			for _, tp := range r.tpls {
				if tp.milestone != nil {
					rows = append(rows, []driver.Value{tp.id})
				}
			}
			return []string{"id"}, rows, nil
		case strings.Contains(q, "FROM zalo_oa_template"):
			var rows [][]driver.Value
			for _, tp := range r.tpls {
				if strings.Contains(q, "AND id = $2") && len(args) > 1 && args[1] != tp.id {
					continue
				}
				rows = append(rows, tp.values())
			}
			return make([]string, 13), rows, nil
		}
		return nil, nil, fmt.Errorf("fake: no answer for %q", q)
	}}
	db := sql.OpenDB(r.f)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	r.logs = &bytes.Buffer{}
	r.uc = NewCommuneZaloOAs(store.New(db), r.resolver, r.env, r.zalo,
		ZaloOAConfig{AppID: oaAppID, AppSecret: secret.Secret(oaSecret), CallbackHost: oaCallback},
		oaRegistry{host: oaHost}, slog.New(slog.NewJSONHandler(r.logs, nil)))
	r.uc.now = func() time.Time { return oaClock }
	r.uc.newID = func() (string, error) { r.ids++; return fmt.Sprintf("01JNEWID%018d", r.ids), nil }
	r.uc.newState = func() ([]byte, error) { return append([]byte(nil), oaState...), nil }
	r.uc.newPKCE = func() (secret.Secret, string, error) { return secret.Secret(oaVerifier), oaChallenge, nil }
	return r
}

func (r *oaRig) seal(t *testing.T, plain string, aad []byte) []byte {
	t.Helper()
	s, err := r.env.Seal(r.ctx, secret.Secret(plain), aad)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// withOA gives the commune a live OA whose access token expires accessIn from the clock.
func (r *oaRig) withOA(t *testing.T, accessIn time.Duration) {
	t.Helper()
	r.oa = &fakeOARow{id: oaRowID, oaID: oaZaloID, name: "UBND Xã A", accessExp: oaClock.Add(accessIn),
		refreshIssued: oaClock.Add(-23 * time.Hour), refreshedAt: oaClock.Add(-23 * time.Hour),
		access: r.seal(t, oaAccess, communeOAAccessAAD(r.ctx, oaRowID)), refresh: r.seal(t, oaRefresh, communeOARefreshAAD(r.ctx, oaRowID))}
}

func (r *oaRig) withOpenAuthorization(t *testing.T) {
	t.Helper()
	r.auth = &fakeAuthRow{expires: oaClock.Add(5 * time.Minute), verifier: r.seal(t, oaVerifier, oaAuthorizationVerifierAAD(r.ctx, oaAuthID))}
}

func (r *oaRig) callbackInput() CompleteAuthorizationInput {
	return CompleteAuthorizationInput{State: base64.RawURLEncoding.EncodeToString(oaState), Code: oaCode, OAID: oaZaloID,
		ClientIP: "203.0.113.9"}
}

// noSecretAnywhere: no audit delta, no security-log line carries a credential or the state.
func (r *oaRig) noSecretAnywhere(t *testing.T) {
	t.Helper()
	stateB64 := base64.RawURLEncoding.EncodeToString(oaState)
	hay := r.logs.String()
	for _, e := range r.f.with("INSERT INTO audit_log") {
		hay += string(e.args[7].([]byte))
	}
	for _, needle := range []string{oaSecret, oaCode, oaAccess, oaRefresh, oaVerifier, stateB64, "_sealed", oaChallenge} {
		if strings.Contains(hay, needle) {
			t.Fatalf("%q leaked into the trail or the security log", needle)
		}
	}
}

func oaEntry(t *testing.T, f *sqlFake, action string) (sqlStmt, map[string]any) {
	t.Helper()
	for _, e := range f.with("INSERT INTO audit_log") {
		if e.args[4] == action {
			var d map[string]any
			if err := json.Unmarshal(e.args[7].([]byte), &d); err != nil {
				t.Fatal(err)
			}
			return e, d
		}
	}
	t.Fatalf("no %s entry; entries = %v", action, entryActions(f))
	return sqlStmt{}, nil
}

// ---- StartAuthorization -------------------------------------------------------------------------------------

func TestStartAuthorizationStoresHashAndSealedVerifierAndBuildsZalosLink(t *testing.T) {
	r := newOARig(t)
	link, err := r.uc.StartAuthorization(r.ctx, staffActor)
	if err != nil {
		t.Fatalf("StartAuthorization: %v", err)
	}
	if r.f.begun != 1 || r.f.commits != 1 || !link.ExpiresAt.Equal(oaClock.Add(10*time.Minute)) {
		t.Fatalf("tx %d/%d expires %v", r.f.begun, r.f.commits, link.ExpiresAt)
	}
	u, err := url.Parse(link.URL)
	if err != nil || u.Host != "oauth.zaloapp.com" || u.Path != "/v4/oa/permission" {
		t.Fatalf("link = %s", link.URL)
	}
	q := u.Query()
	if q.Get("app_id") != oaAppID || q.Get("redirect_uri") != "https://"+oaCallback+"/api/v1/zalo-oa-authorizations" ||
		q.Get("code_challenge") != oaChallenge || q.Get("state") != base64.RawURLEncoding.EncodeToString(oaState) {
		t.Fatalf("query = %v", q)
	}
	ins := r.f.with("INSERT INTO zalo_oa_authorization")
	if len(ins) != 1 {
		t.Fatalf("%d inserts", len(ins))
	}
	a := ins[0].args
	want := sha256.Sum256(oaState)
	if a[0] != string(xaA) || !bytes.Equal(a[2].([]byte), want[:]) || a[5] != "CB-00123" {
		t.Fatalf("insert args = %v", a[:3])
	}
	sealed := a[3].([]byte)
	if bytes.Contains(sealed, []byte(oaVerifier)) {
		t.Fatal("verifier stored in the clear")
	}
	if p, err := r.env.Open(r.ctx, sealed, oaAuthorizationVerifierAAD(r.ctx, a[1].(string))); err != nil || string(p.Lo()) != oaVerifier {
		t.Fatalf("verifier does not open under its binding: %v", err)
	}
	if e, _ := oaEntry(t, r.f, domain.ActionCreateZaloOAAuthorizationLink); e.args[1] != "CB-00123" {
		t.Fatalf("actor = %v", e.args[1])
	}
	r.noSecretAnywhere(t)
}

func TestStartAuthorizationRefusalsWriteNothing(t *testing.T) {
	cases := map[string]func(*oaRig) error{
		"not configured": func(r *oaRig) error { r.uc.cfg.AppSecret = nil; return ErrZaloOANotConfigured },
		"no KEK":         func(r *oaRig) error { r.uc.envelope = nil; return crypto.ErrNotConfigured },
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			r := newOARig(t)
			want := setup(r)
			if _, err := r.uc.StartAuthorization(r.ctx, staffActor); !errors.Is(err, want) {
				t.Fatalf("err = %v", err)
			}
			if r.f.begun != 0 {
				t.Fatal("a refusal opened a transaction")
			}
		})
	}
	r := newOARig(t)
	if _, err := r.uc.StartAuthorization(r.ctx, staffActor); err != nil {
		t.Fatal(err)
	}
}

// ---- CompleteAuthorization ---------------------------------------------------------------------------------

func TestCallbackConnectsTheOAInTheCommuneFoundFromState(t *testing.T) {
	r := newOARig(t)
	r.withOpenAuthorization(t)
	ctx := context.Background() // NO commune: the callback has no commune Host
	res, err := r.uc.CompleteAuthorization(ctx, r.callbackInput())
	if err != nil {
		t.Fatalf("CompleteAuthorization: %v", err)
	}
	if res.Tenant != xaA || res.Host != oaHost || res.OA.OAID != oaZaloID || res.OA.OAName != "UBND Xã A" {
		t.Fatalf("result = %+v", res)
	}
	h := sha256.Sum256(oaState)
	if !bytes.Equal(r.resolver.hash, h[:]) {
		t.Fatal("the resolver was not asked by the state's hash")
	}
	if r.zalo.gotVerifier != oaVerifier || r.zalo.gotCode != oaCode {
		t.Fatal("the exchange did not carry the opened verifier and the code")
	}
	if r.f.begun != 1 || r.f.commits != 1 {
		t.Fatalf("tx %d/%d, want ONE", r.f.begun, r.f.commits)
	}
	ins := r.f.with("INSERT INTO zalo_commune_oa")
	if len(ins) != 1 {
		t.Fatalf("%d inserts", len(ins))
	}
	a := ins[0].args
	rowID := a[1].(string)
	if a[0] != string(xaA) || a[2] != oaZaloID || a[9] != "CB-00123" || a[11] != domain.ZaloCallOK ||
		!a[6].(time.Time).Equal(oaClock.Add(90000*time.Second)) {
		t.Fatalf("insert args = %v", a)
	}
	for i, pair := range []struct {
		plain string
		aad   func(context.Context, string) []byte
	}{{oaAccess, communeOAAccessAAD}, {oaRefresh, communeOARefreshAAD}} {
		sealed := a[4+i].([]byte)
		if p, err := r.env.Open(r.ctx, sealed, pair.aad(r.ctx, rowID)); err != nil || string(p.Lo()) != pair.plain {
			t.Fatalf("token %d does not open under its binding: %v", i, err)
		}
	}
	// The refresh token never opens as an access token.
	if _, err := r.env.Open(r.ctx, a[5].([]byte), communeOAAccessAAD(r.ctx, rowID)); err == nil {
		t.Fatal("the refresh token opened under the access binding")
	}
	cl := r.f.with("oa_row_id = $5")
	if len(cl) != 1 || cl[0].args[3] != domain.AuthorizationSucceeded || cl[0].args[4] != rowID {
		t.Fatalf("close = %v", cl)
	}
	e, d := oaEntry(t, r.f, domain.ActionConnectZaloOA)
	if e.args[1] != "CB-00123" || e.args[3] != "203.0.113.9" || e.args[5] != "zalo-oa/"+oaZaloID || d["truoc"] != nil {
		t.Fatalf("entry = %v delta = %v", e.args[:6], d)
	}
	r.noSecretAnywhere(t)
}

func TestCallbackReplacingTheLiveOARetiresAndDetachesFirst(t *testing.T) {
	r := newOARig(t)
	r.withOpenAuthorization(t)
	r.withOA(t, 10*time.Hour)
	r.tpls = []fakeTplRow{{id: "01JTPL1", templateID: "1001", status: "ENABLE", params: []byte(goodParams), match: true,
		milestone: domain.MilestoneReceived}}
	if _, err := r.uc.CompleteAuthorization(context.Background(), r.callbackInput()); err != nil {
		t.Fatal(err)
	}
	ret := r.f.with("UPDATE zalo_commune_oa SET retired_at")
	if len(ret) != 1 || ret[0].args[1] != oaRowID || ret[0].args[4] != domain.CommuneZaloOAReplacedReason {
		t.Fatalf("retire = %v", ret)
	}
	if len(r.f.with("UPDATE zalo_oa_template SET milestone = NULL")) != 1 {
		t.Fatal("the retired OA's milestones were not detached")
	}
	iR, iI := indexOfStmt(r.f, "UPDATE zalo_commune_oa SET retired_at"), indexOfStmt(r.f, "INSERT INTO zalo_commune_oa")
	if iR < 0 || iR > iI {
		t.Fatalf("retire at %d, insert at %d — 0031's one-live index needs retire first", iR, iI)
	}
	if got := strings.Join(entryActions(r.f), ","); got != domain.ActionDisconnectZaloOA+","+domain.ActionConnectZaloOA {
		t.Fatalf("entries = %s", got)
	}
	if r.f.begun != 1 {
		t.Fatalf("%d transactions, want one", r.f.begun)
	}
}

func TestCallbackUnknownOrMalformedStateIsOneAnswerAndWritesNothing(t *testing.T) {
	for name, mutate := range map[string]func(*oaRig, *CompleteAuthorizationInput){
		"not base64":   func(_ *oaRig, in *CompleteAuthorizationInput) { in.State = "%%%" },
		"short state":  func(_ *oaRig, in *CompleteAuthorizationInput) { in.State = "AAAA" },
		"unknown hash": func(r *oaRig, _ *CompleteAuthorizationInput) { r.resolver.found = false },
		"expired row": func(r *oaRig, _ *CompleteAuthorizationInput) {
			r.auth.expires = oaClock.Add(-time.Second)
		},
		"spent row": func(r *oaRig, _ *CompleteAuthorizationInput) { r.auth = nil },
	} {
		t.Run(name, func(t *testing.T) {
			r := newOARig(t)
			r.withOpenAuthorization(t)
			in := r.callbackInput()
			mutate(r, &in)
			_, err := r.uc.CompleteAuthorization(context.Background(), in)
			if !errors.Is(err, ErrAuthorizationNotFound) {
				t.Fatalf("err = %v", err)
			}
			if r.f.begun != 0 || r.zalo.calls["exchange"] != 0 {
				t.Fatalf("wrote %d tx, called Zalo %d", r.f.begun, r.zalo.calls["exchange"])
			}
		})
	}
}

func TestCallbackFailuresSpendTheRequestWithTheirOutcome(t *testing.T) {
	cases := map[string]struct {
		setup   func(*oaRig, *CompleteAuthorizationInput)
		outcome string
		want    func(error) bool
	}{
		"bad oa_id": {func(_ *oaRig, in *CompleteAuthorizationInput) { in.OAID = "abc" }, domain.AuthorizationMalformedCallback,
			func(e error) bool { return errors.Is(e, ErrAuthorizationMalformed) }},
		"no code": {func(_ *oaRig, in *CompleteAuthorizationInput) { in.Code = "" }, domain.AuthorizationMalformedCallback,
			func(e error) bool { return errors.Is(e, ErrAuthorizationMalformed) }},
		"zalo refuses": {func(r *oaRig, _ *CompleteAuthorizationInput) { r.zalo.exchange = zalooa.OutcomeTokenRejected },
			domain.AuthorizationZaloRefused, func(e error) bool { var c *ZaloOACallError; return errors.As(e, &c) }},
		"zalo down": {func(r *oaRig, _ *CompleteAuthorizationInput) { r.zalo.exchange = zalooa.OutcomeUnavailable },
			domain.AuthorizationZaloUnavailable, func(e error) bool { var c *ZaloOACallError; return errors.As(e, &c) }},
		"getoa names another OA": {func(r *oaRig, _ *CompleteAuthorizationInput) { r.zalo.getoa.OAID = "111" },
			domain.AuthorizationMalformedResponse, func(e error) bool { var c *ZaloOACallError; return errors.As(e, &c) }},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			r := newOARig(t)
			r.withOpenAuthorization(t)
			in := r.callbackInput()
			c.setup(r, &in)
			_, err := r.uc.CompleteAuthorization(context.Background(), in)
			if !c.want(err) {
				t.Fatalf("err = %v", err)
			}
			if len(r.f.with("INSERT INTO zalo_commune_oa")) != 0 {
				t.Fatal("an OA row was written on a failure")
			}
			cl := r.f.with("oa_row_id = $5")
			if len(cl) != 1 || cl[0].args[3] != c.outcome || cl[0].args[4] != nil {
				t.Fatalf("close = %v", cl)
			}
			if e, _ := oaEntry(t, r.f, domain.ActionZaloOAAuthorizationFailed); e.args[1] != "CB-00123" || e.args[3] != "203.0.113.9" {
				t.Fatalf("entry actor/ip = %v %v", e.args[1], e.args[3])
			}
			r.noSecretAnywhere(t)
		})
	}
}

// Every spent request names its outcome and the commune's host, so the edge sends the OA admin back to the
// commune's screen with what happened; success says thanh-cong.
func TestCallbackResultCarriesOutcomeAndHostOnEveryPath(t *testing.T) {
	cases := map[string]struct {
		setup   func(*oaRig, *CompleteAuthorizationInput)
		outcome string
	}{
		"success":      {func(*oaRig, *CompleteAuthorizationInput) {}, domain.AuthorizationSucceeded},
		"no code":      {func(_ *oaRig, in *CompleteAuthorizationInput) { in.Code = "" }, domain.AuthorizationMalformedCallback},
		"zalo refuses": {func(r *oaRig, _ *CompleteAuthorizationInput) { r.zalo.exchange = zalooa.OutcomeRejected }, domain.AuthorizationZaloRefused},
		"zalo down":    {func(r *oaRig, _ *CompleteAuthorizationInput) { r.zalo.exchange = zalooa.OutcomeUnavailable }, domain.AuthorizationZaloUnavailable},
		"getoa other":  {func(r *oaRig, _ *CompleteAuthorizationInput) { r.zalo.getoa.OAID = "111" }, domain.AuthorizationMalformedResponse},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			r := newOARig(t)
			r.withOpenAuthorization(t)
			in := r.callbackInput()
			c.setup(r, &in)
			res, _ := r.uc.CompleteAuthorization(context.Background(), in)
			if res.Outcome != c.outcome || res.Host != oaHost || res.Tenant != xaA {
				t.Fatalf("result = %+v, want outcome %q host %q", res, c.outcome, oaHost)
			}
		})
	}
	r := newOARig(t)
	r.resolver.found = false
	res, err := r.uc.CompleteAuthorization(context.Background(), r.callbackInput())
	if !errors.Is(err, ErrAuthorizationNotFound) || res != (CompleteAuthorizationResult{}) {
		t.Fatalf("unknown state: %+v %v — names a commune or a host", res, err)
	}
}

// The OA admin pressing Từ chối at Zalo: Zalo comes back with `error`, no code. The request is spent as
// zalo-tu-choi with its entry; Zalo is never asked to exchange anything.
func TestCallbackDeniedAtZaloSpendsTheRequestWithoutCallingZalo(t *testing.T) {
	r := newOARig(t)
	r.withOpenAuthorization(t)
	in := r.callbackInput()
	in.Code, in.Denied = "", true
	res, err := r.uc.CompleteAuthorization(context.Background(), in)
	if !errors.Is(err, ErrAuthorizationDenied) || res.Outcome != domain.AuthorizationZaloRefused || res.Host != oaHost {
		t.Fatalf("res = %+v err = %v", res, err)
	}
	if r.zalo.calls["exchange"] != 0 || len(r.f.with("INSERT INTO zalo_commune_oa")) != 0 {
		t.Fatal("a denied authorization reached Zalo or wrote an OA row")
	}
	cl := r.f.with("oa_row_id = $5")
	if len(cl) != 1 || cl[0].args[3] != domain.AuthorizationZaloRefused {
		t.Fatalf("close = %v", cl)
	}
	if e, _ := oaEntry(t, r.f, domain.ActionZaloOAAuthorizationFailed); e.args[1] != "CB-00123" {
		t.Fatalf("entry actor = %v", e.args[1])
	}
	r.noSecretAnywhere(t)

	// A denial with an unknown state is the same 404 as any unknown state: nothing spent.
	r2 := newOARig(t)
	r2.resolver.found = false
	in2 := r2.callbackInput()
	in2.Denied = true
	if _, err := r2.uc.CompleteAuthorization(context.Background(), in2); !errors.Is(err, ErrAuthorizationNotFound) || r2.f.begun != 0 {
		t.Fatalf("err = %v, tx %d", err, r2.f.begun)
	}
}

func TestCallbackGetOAOutageStillConnectsWithTheClass(t *testing.T) {
	r := newOARig(t)
	r.withOpenAuthorization(t)
	r.zalo.getoaOutcome = zalooa.OutcomeUnavailable
	res, err := r.uc.CompleteAuthorization(context.Background(), r.callbackInput())
	if err != nil {
		t.Fatal(err)
	}
	a := r.f.with("INSERT INTO zalo_commune_oa")[0].args
	if res.OA.OAName != "" || a[3] != "" || a[11] != domain.ZaloCallUnavailable {
		t.Fatalf("name %q check %v", a[3], a[11])
	}
}

func TestCallbackOALiveElsewhereIsOneGenericRefusal(t *testing.T) {
	r := newOARig(t)
	r.withOpenAuthorization(t)
	r.f.exec = func(q string, _ []driver.Value) (int64, error) {
		if strings.Contains(q, "INSERT INTO zalo_commune_oa") {
			return 0, &pgconn.PgError{Code: "23505", ConstraintName: "zalo_commune_oa_one_live_per_oa",
				Message: "another commune's details"}
		}
		return 1, nil
	}
	_, err := r.uc.CompleteAuthorization(context.Background(), r.callbackInput())
	if !errors.Is(err, docstore.ErrZaloOATaken) || strings.Contains(err.Error(), "another commune") {
		t.Fatalf("err = %v", err)
	}
	if r.f.rollbacks != 1 || r.f.commits != 1 {
		t.Fatalf("rollbacks %d commits %d — the connect rolls back, the close commits", r.f.rollbacks, r.f.commits)
	}
	cl := r.f.with("oa_row_id = $5")
	if cl[len(cl)-1].args[3] != domain.AuthorizationOALiveElsewhere {
		t.Fatalf("close = %v", cl[len(cl)-1].args)
	}
}

// ---- the token lifecycle -------------------------------------------------------------------------------------

func TestRefreshIfDueLeavesAFreshTokenAlone(t *testing.T) {
	r := newOARig(t)
	r.withOA(t, 5*time.Hour)
	refreshed, err := r.uc.RefreshIfDue(r.ctx)
	if err != nil || refreshed || r.zalo.calls["refresh"] != 0 || r.f.begun != 0 {
		t.Fatalf("refreshed %v err %v calls %v", refreshed, err, r.zalo.calls)
	}
}

func TestRefreshIfDueResealsBothTokensAsTheSystem(t *testing.T) {
	r := newOARig(t)
	r.withOA(t, time.Hour)
	refreshed, err := r.uc.RefreshIfDue(r.ctx)
	if err != nil || !refreshed {
		t.Fatalf("refreshed %v err %v", refreshed, err)
	}
	if r.zalo.gotRefresh != oaRefresh {
		t.Fatal("Zalo was not given the stored refresh token")
	}
	rs := r.f.with("UPDATE zalo_commune_oa SET access_token_sealed")
	if len(rs) != 1 || r.f.begun != 1 {
		t.Fatalf("reseal = %v, %d tx", rs, r.f.begun)
	}
	a := rs[0].args
	if p, err := r.env.Open(r.ctx, a[3].([]byte), communeOARefreshAAD(r.ctx, oaRowID)); err != nil || string(p.Lo()) != oaRefresh+"-2" {
		t.Fatalf("new refresh token not sealed to the row: %v", err)
	}
	if !a[6].(time.Time).Equal(oaClock) || !a[7].(time.Time).Equal(r.oa.refreshedAt) || !a[4].(time.Time).Equal(oaClock.Add(90000*time.Second)) {
		t.Fatalf("reseal times = %v", a[4:])
	}
	if e, _ := oaEntry(t, r.f, domain.ActionRefreshZaloOAToken); e.args[1] != "system" || e.args[2] != "system" {
		t.Fatalf("refresh actor = %v/%v, want the system principal", e.args[1], e.args[2])
	}
	r.noSecretAnywhere(t)
}

func TestRefreshRefusedOrAgedFlagsNeedsReauthorization(t *testing.T) {
	cases := map[string]struct {
		setup      func(*oaRig)
		reason     string
		zaloCalled bool
	}{
		"zalo refuses": {func(r *oaRig) { r.zalo.refresh = zalooa.OutcomeTokenRejected }, domain.ReauthorizationRefreshTokenRejected, true},
		"older than 3 months": {func(r *oaRig) {
			r.oa.refreshIssued = oaClock.Add(-91 * 24 * time.Hour)
			r.oa.refreshedAt = r.oa.refreshIssued
		}, domain.ReauthorizationRefreshTokenExpired, false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			r := newOARig(t)
			r.withOA(t, time.Hour)
			c.setup(r)
			refreshed, err := r.uc.RefreshIfDue(r.ctx)
			if err != nil || refreshed {
				t.Fatalf("refreshed %v err %v — a flagged row is not a job failure", refreshed, err)
			}
			if (r.zalo.calls["refresh"] > 0) != c.zaloCalled {
				t.Fatalf("Zalo called %d times", r.zalo.calls["refresh"])
			}
			mk := r.f.with("SET needs_reauthorization_since")
			if len(mk) != 1 || mk[0].args[3] != c.reason {
				t.Fatalf("mark = %v", mk)
			}
			if e, _ := oaEntry(t, r.f, domain.ActionZaloOANeedsReauthorization); e.args[1] != "system" {
				t.Fatalf("actor = %v", e.args[1])
			}
		})
	}
}

func TestRefreshOutageWritesNothingAndReportsTheClass(t *testing.T) {
	r := newOARig(t)
	r.withOA(t, time.Hour)
	r.zalo.refresh = zalooa.OutcomeUnavailable
	_, err := r.uc.RefreshIfDue(r.ctx)
	var c *ZaloOACallError
	if !errors.As(err, &c) || c.Class != domain.ZaloCallUnavailable || r.f.begun != 0 {
		t.Fatalf("err %v tx %d", err, r.f.begun)
	}
}

func TestFlaggedOARefusesCheckAndSync(t *testing.T) {
	r := newOARig(t)
	r.withOA(t, 5*time.Hour)
	r.oa.reauthSince, r.oa.reauthReason = oaClock.Add(-time.Hour), domain.ReauthorizationRefreshTokenRejected
	if _, err := r.uc.Check(r.ctx, staffActor); !errors.Is(err, ErrZaloOANeedsReauthorization) {
		t.Fatalf("check: %v", err)
	}
	if _, err := r.uc.SyncTemplates(r.ctx, staffActor); !errors.Is(err, ErrZaloOANeedsReauthorization) {
		t.Fatalf("sync: %v", err)
	}
	if len(r.zalo.calls) != 0 || r.f.begun != 0 {
		t.Fatalf("calls %v tx %d", r.zalo.calls, r.f.begun)
	}
}

func TestExpireOpenAuthorizationsAuditsAsTheSystem(t *testing.T) {
	r := newOARig(t)
	n, err := r.uc.ExpireOpenAuthorizations(r.ctx, oaClock)
	if err != nil || n != 1 {
		t.Fatalf("n %d err %v", n, err)
	}
	ex := r.f.with("RETURNING id")
	if len(ex) != 1 || ex[0].args[3] != domain.AuthorizationExpired {
		t.Fatalf("expire = %v", ex)
	}
	if e, _ := oaEntry(t, r.f, domain.ActionZaloOAAuthorizationExpired); e.args[1] != "system" {
		t.Fatalf("actor = %v", e.args[1])
	}
}

// ---- the buttons ---------------------------------------------------------------------------------------------

func TestCheckRecordsTheClassAndTheName(t *testing.T) {
	r := newOARig(t)
	r.withOA(t, 5*time.Hour)
	r.zalo.getoa.Name = "UBND Xã A mới"
	out, err := r.uc.Check(r.ctx, staffActor)
	if err != nil || out.Outcome != domain.ZaloCallOK || out.OAName != "UBND Xã A mới" {
		t.Fatalf("out %+v err %v", out, err)
	}
	if rc := r.f.with("SET last_check_at"); len(rc) != 1 || rc[0].args[3] != domain.ZaloCallOK {
		t.Fatalf("check = %v", rc)
	}
	if nm := r.f.with("SET oa_name"); len(nm) != 1 || nm[0].args[2] != "UBND Xã A mới" {
		t.Fatalf("name = %v", nm)
	}
	if e, _ := oaEntry(t, r.f, domain.ActionCheckZaloOA); e.args[1] != "CB-00123" {
		t.Fatalf("actor = %v", e.args[1])
	}
	if r.f.begun != 1 {
		t.Fatalf("%d tx", r.f.begun)
	}
}

func TestCheckWithNoOA(t *testing.T) {
	r := newOARig(t)
	if _, err := r.uc.Check(r.ctx, staffActor); !errors.Is(err, ErrCommuneZaloOAMissing) {
		t.Fatalf("err = %v", err)
	}
}

func TestDisconnectRetiresDetachesAndSaysToRevokeAtZalo(t *testing.T) {
	r := newOARig(t)
	r.withOA(t, 5*time.Hour)
	r.tpls = []fakeTplRow{{id: "01JTPL1", templateID: "1001", status: "ENABLE", params: []byte(goodParams), match: true,
		milestone: domain.MilestoneClosed}, {id: "01JTPL2", templateID: "1002", status: "ENABLE", params: []byte(goodParams)}}
	out, err := r.uc.Disconnect(r.ctx, "  Xã đổi OA  ", staffActor)
	if err != nil || !out.Retired || out.DetachedTemplateCount != 1 || out.RevokeNotice != domain.CommuneZaloOARevokeNotice {
		t.Fatalf("out %+v err %v", out, err)
	}
	ret := r.f.with("SET retired_at")
	if len(ret) != 1 || ret[0].args[3] != "CB-00123" || ret[0].args[4] != "Xã đổi OA" {
		t.Fatalf("retire = %v", ret)
	}
	if len(r.f.with("SET milestone = NULL")) != 1 || len(r.f.with("DELETE")) != 0 {
		t.Fatal("milestones not detached, or a DELETE was issued")
	}
	if r.f.begun != 1 || r.f.commits != 1 {
		t.Fatalf("tx %d/%d", r.f.begun, r.f.commits)
	}
	// Idempotent: no live OA = nothing written, no notice.
	r2 := newOARig(t)
	if out, err := r2.uc.Disconnect(r2.ctx, "lý do", staffActor); err != nil || out.Retired || out.RevokeNotice != "" {
		t.Fatalf("no OA: %+v %v", out, err)
	}
	if _, err := r2.uc.Disconnect(r2.ctx, "  ", staffActor); !errors.Is(err, domain.ErrCommuneZaloOARetireReason) {
		t.Fatalf("empty reason: %v", err)
	}
}

func TestSyncTemplatesUpsertsJudgesParamsAndMarksVanishedDeleted(t *testing.T) {
	r := newOARig(t)
	r.withOA(t, 5*time.Hour)
	// Known row 1001 (was PENDING_REVIEW, attached elsewhere irrelevant), vanished row 1009.
	r.tpls = []fakeTplRow{
		{id: "01JTPLKNOWN", templateID: "1001", status: "PENDING_REVIEW", params: []byte(goodParams)},
		{id: "01JTPLGONE", templateID: "1009", status: "ENABLE", params: []byte(goodParams), match: true, milestone: domain.MilestoneClosed},
	}
	r.zalo.total = 3
	r.zalo.pages = [][]zalooa.TemplateSummary{{{ID: "1001", Status: "ENABLE"}, {ID: "1002", Status: "ENABLE"}, {ID: "1003"}}}
	full := []zalooa.Param{{Name: "ma_tra_cuu"}, {Name: "moc_nhan"}, {Name: "viec_tiep_theo"}}
	r.zalo.infos["1001"] = zalooa.TemplateDetail{ID: "1001", Name: "Tiếp nhận", Status: "ENABLE", Quality: "HIGH", Tag: "TRANSACTION", Params: full}
	r.zalo.infos["1002"] = zalooa.TemplateDetail{ID: "1002", Name: "Thiếu", Status: "ENABLE", Params: full[:2]}
	r.zalo.infoOutcome["1003"] = zalooa.OutcomeMalformedResponse

	out, err := r.uc.SyncTemplates(r.ctx, staffActor)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if out.Loaded != 2 || out.Enabled != 2 || out.Matching != 1 || strings.Join(out.SkippedTemplateIDs, ",") != "1003" || out.Truncated {
		t.Fatalf("out = %+v", out)
	}
	if strings.Join(out.ChangedTemplateIDs, ",") != "1001,1002,1009" {
		t.Fatalf("changed = %v", out.ChangedTemplateIDs)
	}
	up := r.f.with("INSERT INTO zalo_oa_template")
	if len(up) != 2 || up[0].args[1] != "01JTPLKNOWN" || up[0].args[9] != true || up[1].args[9] != false ||
		up[1].args[6] != domain.ZaloTemplateQualityUndefined {
		t.Fatalf("upserts = %v / %v", up[0].args, up[1].args)
	}
	if strings.Contains(up[0].sql, "milestone") {
		t.Fatal("the sync writes the commune's milestone")
	}
	if del := r.f.with("SET zalo_status"); len(del) != 1 || del[0].args[1] != "01JTPLGONE" || del[0].args[2] != "DELETE" {
		t.Fatalf("mark deleted = %v", del)
	}
	_, d := oaEntry(t, r.f, domain.ActionSyncZaloOATemplates)
	if raw, _ := json.Marshal(d); strings.Contains(string(raw), "ma_tra_cuu") || strings.Contains(string(raw), "Tiếp nhận") {
		t.Fatalf("the sync entry carries parameters or names: %s", raw)
	}
	if r.f.begun != 1 {
		t.Fatalf("%d tx", r.f.begun)
	}
}

func TestSyncTemplatesOutageWritesNothing(t *testing.T) {
	r := newOARig(t)
	r.withOA(t, 5*time.Hour)
	r.zalo.pages = [][]zalooa.TemplateSummary{{{ID: "1001"}}}
	r.zalo.infoOutcome["1001"] = zalooa.OutcomeRateLimited
	_, err := r.uc.SyncTemplates(r.ctx, staffActor)
	var c *ZaloOACallError
	if !errors.As(err, &c) || c.Class != domain.ZaloCallRateLimited || r.f.begun != 0 {
		t.Fatalf("err %v tx %d", err, r.f.begun)
	}
}

func TestSetMilestoneRules(t *testing.T) {
	m := domain.MilestoneReceived
	t.Run("not ENABLE", func(t *testing.T) {
		r := newOARig(t)
		r.withOA(t, 5*time.Hour)
		r.tpls = []fakeTplRow{{id: "01JTPL1", templateID: "1001", status: "PENDING_REVIEW", params: []byte(goodParams), match: true}}
		if _, err := r.uc.SetMilestone(r.ctx, "01JTPL1", &m, staffActor); !errors.Is(err, domain.ErrZaloOATemplateNotEnabled) {
			t.Fatalf("err = %v", err)
		}
		if len(r.f.with("UPDATE zalo_oa_template")) != 0 {
			t.Fatal("written")
		}
	})
	t.Run("missing params named", func(t *testing.T) {
		r := newOARig(t)
		r.withOA(t, 5*time.Hour)
		r.tpls = []fakeTplRow{{id: "01JTPL1", templateID: "1001", status: "ENABLE", params: []byte(`[{"name":"moc_nhan"}]`)}}
		_, err := r.uc.SetMilestone(r.ctx, "01JTPL1", &m, staffActor)
		var pm *domain.TemplateParamsMissingError
		if !errors.As(err, &pm) || strings.Join(pm.Missing, ",") != "ma_tra_cuu,viec_tiep_theo" {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("unknown milestone", func(t *testing.T) {
		r := newOARig(t)
		bad := "moi"
		if _, err := r.uc.SetMilestone(r.ctx, "01JTPL1", &bad, staffActor); !errors.Is(err, domain.ErrZaloOAMilestoneUnknown) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("attach then taken", func(t *testing.T) {
		r := newOARig(t)
		r.withOA(t, 5*time.Hour)
		r.tpls = []fakeTplRow{{id: "01JTPL1", templateID: "1001", status: "ENABLE", params: []byte(goodParams), match: true}}
		out, err := r.uc.SetMilestone(r.ctx, "01JTPL1", &m, staffActor)
		if err != nil || out.Milestone != m || out.MilestoneSetBy != "CB-00123" {
			t.Fatalf("out %+v err %v", out, err)
		}
		up := r.f.with("UPDATE zalo_oa_template SET milestone = $3")
		if len(up) != 1 || up[0].args[2] != m || up[0].args[4] != "CB-00123" {
			t.Fatalf("update = %v", up)
		}
		if e, _ := oaEntry(t, r.f, domain.ActionAttachZaloOATemplateMilestone); e.args[5] != "zalo-oa-template/1001" {
			t.Fatalf("subject = %v", e.args[5])
		}
		r.f.exec = func(q string, _ []driver.Value) (int64, error) {
			if strings.Contains(q, "SET milestone = $3") {
				return 0, &pgconn.PgError{Code: "23505", ConstraintName: "zalo_oa_template_one_per_milestone"}
			}
			return 1, nil
		}
		other := domain.MilestoneClosed
		if _, err := r.uc.SetMilestone(r.ctx, "01JTPL1", &other, staffActor); !errors.Is(err, docstore.ErrMilestoneTaken) {
			t.Fatalf("taken: %v", err)
		}
	})
	t.Run("detach always allowed", func(t *testing.T) {
		r := newOARig(t)
		r.withOA(t, 5*time.Hour)
		r.tpls = []fakeTplRow{{id: "01JTPL1", templateID: "1001", status: "REJECT", params: []byte(`[]`), milestone: m}}
		out, err := r.uc.SetMilestone(r.ctx, "01JTPL1", nil, staffActor)
		if err != nil || out.Attached() {
			t.Fatalf("out %+v err %v", out, err)
		}
		up := r.f.with("UPDATE zalo_oa_template SET milestone = $3")
		if len(up) != 1 || up[0].args[2] != nil || up[0].args[4] != nil {
			t.Fatalf("detach = %v", up)
		}
		oaEntry(t, r.f, domain.ActionDetachZaloOATemplateMilestone)
	})
	// Coordinator 16/10/2026 (web A6): detaching never depends on the template's state — a template Zalo
	// disabled, or whose parameters stopped matching, must still release its milestone, or its replacement
	// meets milestone_taken for ever.
	t.Run("detach ignores status and params_match", func(t *testing.T) {
		for _, tp := range []fakeTplRow{
			{id: "01JTPL1", templateID: "1001", status: "DISABLE", params: []byte(goodParams), match: true, milestone: m},
			{id: "01JTPL1", templateID: "1001", status: "ENABLE", params: []byte(`[{"name":"moc_nhan"}]`), match: false, milestone: m},
		} {
			r := newOARig(t)
			r.withOA(t, 5*time.Hour)
			r.tpls = []fakeTplRow{tp}
			out, err := r.uc.SetMilestone(r.ctx, "01JTPL1", nil, staffActor)
			if err != nil || out.Attached() {
				t.Fatalf("%s/match=%v: out %+v err %v", tp.status, tp.match, out, err)
			}
			if len(r.f.with("UPDATE zalo_oa_template SET milestone = $3")) != 1 {
				t.Fatalf("%s/match=%v: no detach written", tp.status, tp.match)
			}
		}
	})
	t.Run("template of another OA row", func(t *testing.T) {
		r := newOARig(t)
		r.withOA(t, 5*time.Hour)
		r.oa.id = "01JOTHERROW"
		r.tpls = []fakeTplRow{{id: "01JTPL1", templateID: "1001", status: "ENABLE", params: []byte(goodParams), match: true}}
		if _, err := r.uc.SetMilestone(r.ctx, "01JTPL1", &m, staffActor); !errors.Is(err, ErrZaloOATemplateMissing) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestCurrentCountsTemplatesAndReportsConfiguration(t *testing.T) {
	r := newOARig(t)
	v, err := r.uc.Current(r.ctx)
	if err != nil || v.HasOA || !v.Configured {
		t.Fatalf("no OA: %+v %v", v, err)
	}
	r.withOA(t, 5*time.Hour)
	r.tpls = []fakeTplRow{{id: "a", templateID: "1", status: "ENABLE", params: []byte(goodParams), milestone: domain.MilestoneClosed},
		{id: "b", templateID: "2", status: "ENABLE", params: []byte(goodParams)}}
	v, err = r.uc.Current(r.ctx)
	if err != nil || !v.HasOA || v.TemplateCount != 2 || v.AttachedCount != 1 || v.OA.OAID != oaZaloID {
		t.Fatalf("view = %+v %v", v, err)
	}
}
