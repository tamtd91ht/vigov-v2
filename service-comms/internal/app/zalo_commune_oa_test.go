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
	tenant                                tenant.ID // "" = xaA
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
	oaRow                  string // "" = oaRowID
}

func (t fakeTplRow) row() string {
	if t.oaRow == "" {
		return oaRowID
	}
	return t.oaRow
}

func (t fakeTplRow) values() []driver.Value {
	var setAt, setBy driver.Value
	if t.milestone != nil {
		setAt, setBy = oaClock.Add(-time.Hour), "CB-00099"
	}
	return []driver.Value{t.id, t.row(), t.templateID, "Mẫu " + t.templateID, t.status, "HIGH", "TRANSACTION", t.params,
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
	oa       *fakeOARow   // the first OA withOA gave the commune
	oas      []*fakeOARow // every live OA row the fake answers, filtered by commune, id and oa_id
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
			// The scope the database applies: $1 is the commune; `id = $2` / `oa_id = $2` narrow it.
			var rows [][]driver.Value
			for _, o := range r.oas {
				owner := o.tenant
				if owner == "" {
					owner = xaA
				}
				if args[0] != string(owner) {
					continue
				}
				if strings.Contains(q, "AND id = $2") && args[1] != o.id {
					continue
				}
				if strings.Contains(q, "AND oa_id = $2") && args[1] != o.oaID {
					continue
				}
				rows = append(rows, o.values(sealed))
			}
			return cols, rows, nil
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
		case strings.Contains(q, "FROM zalo_oa_template") && strings.Contains(q, "GROUP BY oa_row_id"):
			counts := map[string][2]int64{}
			var order []string
			for _, tp := range r.tpls {
				c, ok := counts[tp.row()]
				if !ok {
					order = append(order, tp.row())
				}
				c[0]++
				if tp.milestone != nil {
					c[1]++
				}
				counts[tp.row()] = c
			}
			var rows [][]driver.Value
			for _, id := range order {
				rows = append(rows, []driver.Value{id, counts[id][0], counts[id][1]})
			}
			return []string{"oa_row_id", "count", "count"}, rows, nil
		case strings.Contains(q, "FROM zalo_oa_template"):
			var rows [][]driver.Value
			for _, tp := range r.tpls {
				if strings.Contains(q, "AND id = $2") && len(args) > 1 && args[1] != tp.id {
					continue
				}
				if strings.Contains(q, "AND oa_row_id = $2") && args[1] != tp.row() {
					continue
				}
				if strings.Contains(q, "AND template_id = $3") && args[2] != tp.templateID {
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
	r.oa = r.addOA(t, oaRowID, oaZaloID, accessIn)
}

// addOA gives the commune one more live OA (ADR 0099 #1: several at once).
func (r *oaRig) addOA(t *testing.T, rowID, zaloID string, accessIn time.Duration) *fakeOARow {
	t.Helper()
	o := &fakeOARow{id: rowID, oaID: zaloID, name: "OA " + zaloID, accessExp: oaClock.Add(accessIn),
		refreshIssued: oaClock.Add(-23 * time.Hour), refreshedAt: oaClock.Add(-23 * time.Hour),
		access: r.seal(t, oaAccess, communeOAAccessAAD(r.ctx, rowID)), refresh: r.seal(t, oaRefresh, communeOARefreshAAD(r.ctx, rowID))}
	if zaloID == oaZaloID {
		o.name = "UBND Xã A"
	}
	r.oas = append(r.oas, o)
	return o
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

// ADR 0098 #11, kept for the SAME OA by ADR 0099 #1: authorizing an OA already live in this commune
// retires its row (milestones detached) and inserts a new one, in one transaction.
func TestCallbackReauthorizingTheSameOARetiresAndDetachesFirst(t *testing.T) {
	r := newOARig(t)
	r.withOpenAuthorization(t)
	r.withOA(t, 10*time.Hour)
	r.tpls = []fakeTplRow{{id: "01JTPL1", templateID: "1001", status: "ENABLE", params: []byte(goodParams), match: true,
		milestone: domain.MilestoneReceived}}
	if _, err := r.uc.CompleteAuthorization(context.Background(), r.callbackInput()); err != nil {
		t.Fatal(err)
	}
	if q := r.f.with("AND oa_id = $2"); len(q) != 1 || q[0].args[1] != oaZaloID {
		t.Fatalf("the callback did not look this OA up by Zalo's oa_id: %v", q)
	}
	ret := r.f.with("SET retired_at")
	if len(ret) != 1 || ret[0].args[1] != oaRowID || ret[0].args[4] != domain.CommuneZaloOAReauthorizedReason {
		t.Fatalf("retire = %v", ret)
	}
	if len(r.f.with("SET milestone = NULL")) != 1 {
		t.Fatal("the retired OA's milestones were not detached")
	}
	iR, iI := indexOfStmt(r.f, "SET retired_at"), indexOfStmt(r.f, "INSERT INTO zalo_commune_oa")
	if iR < 0 || iR > iI {
		t.Fatalf("retire at %d, insert at %d — 0031's one-live-per-OA floor needs retire first", iR, iI)
	}
	if got := strings.Join(entryActions(r.f), ","); got != domain.ActionDisconnectZaloOA+","+domain.ActionConnectZaloOA {
		t.Fatalf("entries = %s", got)
	}
	if r.f.begun != 1 {
		t.Fatalf("%d transactions, want one", r.f.begun)
	}
}

// ADR 0099 #1: a commune holds several OAs. Connecting a DIFFERENT OA retires nothing and detaches no
// milestone of the OA already there — one insert, one entry.
func TestCallbackSecondOAIsAddedBesideTheFirst(t *testing.T) {
	r := newOARig(t)
	r.withOpenAuthorization(t)
	r.withOA(t, 10*time.Hour)
	r.tpls = []fakeTplRow{{id: "01JTPL1", templateID: "1001", status: "ENABLE", params: []byte(goodParams), match: true,
		milestone: domain.MilestoneReceived}}
	const second = "111222333444"
	r.zalo.getoa = zalooa.OAInfo{OAID: second, Name: "OA thứ hai"}
	in := r.callbackInput()
	in.OAID = second
	res, err := r.uc.CompleteAuthorization(context.Background(), in)
	if err != nil || res.Outcome != domain.AuthorizationSucceeded || res.OA.OAID != second {
		t.Fatalf("res %+v err %v", res, err)
	}
	if len(r.f.with("SET retired_at")) != 0 || len(r.f.with("SET milestone = NULL")) != 0 {
		t.Fatal("connecting a second OA retired or detached the first")
	}
	if ins := r.f.with("INSERT INTO zalo_commune_oa"); len(ins) != 1 || ins[0].args[2] != second {
		t.Fatalf("insert = %v", ins)
	}
	if got := strings.Join(entryActions(r.f), ","); got != domain.ActionConnectZaloOA {
		t.Fatalf("entries = %s", got)
	}
	if r.f.begun != 1 || r.f.commits != 1 {
		t.Fatalf("tx %d/%d", r.f.begun, r.f.commits)
	}
	r.noSecretAnywhere(t)
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

// Several OAs: the job refreshes every DUE one, leaves the fresh and the flagged ones alone, and one OA
// failing does not stop the next.
func TestRefreshIfDueWalksEveryDueOAOfTheCommune(t *testing.T) {
	r := newOARig(t)
	r.withOA(t, time.Hour)                                       // due
	r.addOA(t, "01JOAROW000000000000000002", "222", 5*time.Hour) // fresh
	flagged := r.addOA(t, "01JOAROW000000000000000003", "333", time.Hour)
	flagged.reauthSince, flagged.reauthReason = oaClock.Add(-time.Hour), domain.ReauthorizationRefreshTokenRejected
	r.addOA(t, "01JOAROW000000000000000004", "444", 30*time.Minute) // due
	refreshed, err := r.uc.RefreshIfDue(r.ctx)
	if err != nil || !refreshed {
		t.Fatalf("refreshed %v err %v", refreshed, err)
	}
	var ids []string
	for _, s := range r.f.with("SET access_token_sealed") {
		ids = append(ids, s.args[1].(string))
	}
	if strings.Join(ids, ",") != oaRowID+",01JOAROW000000000000000004" || r.zalo.calls["refresh"] != 2 {
		t.Fatalf("resealed %v, Zalo refresh calls %d", ids, r.zalo.calls["refresh"])
	}

	r2 := newOARig(t)
	r2.withOA(t, time.Hour)
	r2.addOA(t, "01JOAROW000000000000000002", "222", time.Hour)
	r2.zalo.refresh = zalooa.OutcomeUnavailable
	_, err = r2.uc.RefreshIfDue(r2.ctx)
	var c *ZaloOACallError
	if !errors.As(err, &c) || r2.zalo.calls["refresh"] != 2 {
		t.Fatalf("err %v calls %d — the second OA was not tried after the first failed", err, r2.zalo.calls["refresh"])
	}
}

func TestFlaggedOARefusesCheckAndSync(t *testing.T) {
	r := newOARig(t)
	r.withOA(t, 5*time.Hour)
	r.oa.reauthSince, r.oa.reauthReason = oaClock.Add(-time.Hour), domain.ReauthorizationRefreshTokenRejected
	if _, err := r.uc.Check(r.ctx, oaRowID, staffActor); !errors.Is(err, ErrZaloOANeedsReauthorization) {
		t.Fatalf("check: %v", err)
	}
	if _, err := r.uc.SyncTemplates(r.ctx, oaRowID, staffActor); !errors.Is(err, ErrZaloOANeedsReauthorization) {
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
	r.addOA(t, "01JOAROW000000000000000002", "222", 5*time.Hour)
	r.zalo.getoa.Name = "UBND Xã A mới"
	out, err := r.uc.Check(r.ctx, oaRowID, staffActor)
	if err != nil || out.Outcome != domain.ZaloCallOK || out.OAName != "UBND Xã A mới" {
		t.Fatalf("out %+v err %v", out, err)
	}
	if rc := r.f.with("SET last_check_at"); len(rc) != 1 || rc[0].args[3] != domain.ZaloCallOK {
		t.Fatalf("check = %v", rc)
	}
	if nm := r.f.with("SET oa_name"); len(nm) != 1 || nm[0].args[1] != oaRowID || nm[0].args[2] != "UBND Xã A mới" {
		t.Fatalf("name = %v", nm)
	}
	if e, _ := oaEntry(t, r.f, domain.ActionCheckZaloOA); e.args[1] != "CB-00123" {
		t.Fatalf("actor = %v", e.args[1])
	}
	if r.f.begun != 1 {
		t.Fatalf("%d tx", r.f.begun)
	}
}

// An unknown id, a retired OA and ANOTHER COMMUNE's OA are one answer, and nothing reaches Zalo.
func TestActsOnAnIDThatIsNotALiveOAOfTheCommune(t *testing.T) {
	r := newOARig(t)
	r.withOA(t, 5*time.Hour)
	other := r.addOA(t, "01JOAROWXAB000000000000001", "999", 5*time.Hour)
	other.tenant = xaB
	for _, id := range []string{"01JNOSUCHOA000000000000001", other.id} {
		if _, err := r.uc.Check(r.ctx, id, staffActor); !errors.Is(err, ErrCommuneZaloOAMissing) {
			t.Fatalf("check %s: %v", id, err)
		}
		if _, err := r.uc.SyncTemplates(r.ctx, id, staffActor); !errors.Is(err, ErrCommuneZaloOAMissing) {
			t.Fatalf("sync %s: %v", id, err)
		}
		if _, err := r.uc.ListTemplates(r.ctx, id); !errors.Is(err, ErrCommuneZaloOAMissing) {
			t.Fatalf("templates %s: %v", id, err)
		}
		if _, err := r.uc.Disconnect(r.ctx, id, "lý do", staffActor); !errors.Is(err, ErrCommuneZaloOAMissing) {
			t.Fatalf("disconnect %s: %v", id, err)
		}
		if _, _, err := r.uc.OpenAccessToken(r.ctx, id); !errors.Is(err, ErrCommuneZaloOAMissing) {
			t.Fatalf("open %s: %v", id, err)
		}
	}
	if len(r.zalo.calls) != 0 || r.f.commits != 0 || len(r.f.with("SET retired_at")) != 0 {
		t.Fatalf("calls %v commits %d", r.zalo.calls, r.f.commits)
	}
	// Commune B, asking in ITS context, reaches its own OA and not A's.
	ctxB := tenant.Into(context.Background(), xaB)
	if _, err := r.uc.ListTemplates(ctxB, oaRowID); !errors.Is(err, ErrCommuneZaloOAMissing) {
		t.Fatalf("B read A's OA: %v", err)
	}
	if _, err := r.uc.ListTemplates(ctxB, other.id); err != nil {
		t.Fatalf("B's own OA: %v", err)
	}
}

func TestDisconnectRetiresDetachesAndSaysToRevokeAtZalo(t *testing.T) {
	r := newOARig(t)
	r.withOA(t, 5*time.Hour)
	r.addOA(t, "01JOAROW000000000000000002", "222", 5*time.Hour)
	r.tpls = []fakeTplRow{{id: "01JTPL1", templateID: "1001", status: "ENABLE", params: []byte(goodParams), match: true,
		milestone: domain.MilestoneClosed}, {id: "01JTPL2", templateID: "1002", status: "ENABLE", params: []byte(goodParams)}}
	out, err := r.uc.Disconnect(r.ctx, oaRowID, "  Xã đổi OA  ", staffActor)
	if err != nil || !out.Retired || out.DetachedTemplateCount != 1 || out.RevokeNotice != domain.CommuneZaloOARevokeNotice {
		t.Fatalf("out %+v err %v", out, err)
	}
	// Only the named OA: the commune's other OA is untouched (ADR 0099 #1).
	ret := r.f.with("SET retired_at")
	if len(ret) != 1 || ret[0].args[1] != oaRowID || ret[0].args[3] != "CB-00123" || ret[0].args[4] != "Xã đổi OA" {
		t.Fatalf("retire = %v", ret)
	}
	if det := r.f.with("SET milestone = NULL"); len(det) != 1 || det[0].args[1] != oaRowID || len(r.f.with("DELETE")) != 0 {
		t.Fatal("milestones not detached for that OA, or a DELETE was issued")
	}
	if r.f.begun != 1 || r.f.commits != 1 {
		t.Fatalf("tx %d/%d", r.f.begun, r.f.commits)
	}
	// No live OA with that id = 404 at the edge, nothing written.
	r2 := newOARig(t)
	if _, err := r2.uc.Disconnect(r2.ctx, oaRowID, "lý do", staffActor); !errors.Is(err, ErrCommuneZaloOAMissing) || r2.f.commits != 0 {
		t.Fatalf("no OA: %v, %d commits", err, r2.f.commits)
	}
	if _, err := r2.uc.Disconnect(r2.ctx, oaRowID, "  ", staffActor); !errors.Is(err, domain.ErrCommuneZaloOARetireReason) {
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

	out, err := r.uc.SyncTemplates(r.ctx, oaRowID, staffActor)
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
	_, err := r.uc.SyncTemplates(r.ctx, oaRowID, staffActor)
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
	t.Run("template of a retired OA", func(t *testing.T) {
		r := newOARig(t)
		r.withOA(t, 5*time.Hour)
		r.oa.id = "01JOTHERROW" // the template's OA row (oaRowID) is no longer live
		r.tpls = []fakeTplRow{{id: "01JTPL1", templateID: "1001", status: "ENABLE", params: []byte(goodParams), match: true}}
		if _, err := r.uc.SetMilestone(r.ctx, "01JTPL1", &m, staffActor); !errors.Is(err, ErrZaloOATemplateMissing) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("unknown template", func(t *testing.T) {
		r := newOARig(t)
		r.withOA(t, 5*time.Hour)
		if _, err := r.uc.SetMilestone(r.ctx, "01JNOSUCHTPL", &m, staffActor); !errors.Is(err, ErrZaloOATemplateMissing) || r.f.begun != 0 {
			t.Fatalf("err = %v, tx %d", err, r.f.begun)
		}
	})
	// The template of the commune's SECOND OA takes a milestone: the lock is on ITS OA row, read from the
	// template, and the OA row is locked before the template (the order "Ngắt OA" takes).
	t.Run("template of the second OA", func(t *testing.T) {
		r := newOARig(t)
		r.withOA(t, 5*time.Hour)
		r.addOA(t, "01JOAROW000000000000000002", "222", 5*time.Hour)
		r.tpls = []fakeTplRow{{id: "01JTPL2", templateID: "2001", status: "ENABLE", params: []byte(goodParams), match: true,
			oaRow: "01JOAROW000000000000000002"}}
		out, err := r.uc.SetMilestone(r.ctx, "01JTPL2", &m, staffActor)
		if err != nil || out.Milestone != m || out.OARowID != "01JOAROW000000000000000002" {
			t.Fatalf("out %+v err %v", out, err)
		}
		lk := r.f.with("FOR UPDATE")
		if len(lk) != 2 || !strings.Contains(lk[0].sql, "zalo_commune_oa") || lk[0].args[1] != "01JOAROW000000000000000002" ||
			!strings.Contains(lk[1].sql, "zalo_oa_template") {
			t.Fatalf("locks = %v — the OA row must be locked before the template", lk)
		}
	})
}

func TestListReturnsEveryLiveOAWithItsCounts(t *testing.T) {
	r := newOARig(t)
	v, err := r.uc.List(r.ctx)
	if err != nil || len(v.Items) != 0 || !v.Configured {
		t.Fatalf("no OA: %+v %v", v, err)
	}
	r.withOA(t, 5*time.Hour)
	r.addOA(t, "01JOAROW000000000000000002", "222", 5*time.Hour)
	other := r.addOA(t, "01JOAROWXAB000000000000001", "999", 5*time.Hour)
	other.tenant = xaB
	r.tpls = []fakeTplRow{{id: "a", templateID: "1", status: "ENABLE", params: []byte(goodParams), milestone: domain.MilestoneClosed},
		{id: "b", templateID: "2", status: "ENABLE", params: []byte(goodParams)},
		{id: "c", templateID: "3", status: "ENABLE", params: []byte(goodParams), oaRow: "01JOAROW000000000000000002"}}
	v, err = r.uc.List(r.ctx)
	if err != nil || len(v.Items) != 2 {
		t.Fatalf("list = %+v %v — another commune's OA listed, or one missing", v, err)
	}
	a, b := v.Items[0], v.Items[1]
	if a.OA.ID != oaRowID || a.TemplateCount != 2 || a.AttachedCount != 1 ||
		b.OA.OAID != "222" || b.TemplateCount != 1 || b.AttachedCount != 0 {
		t.Fatalf("items = %+v", v.Items)
	}
	if len(r.f.with("GROUP BY oa_row_id")) != 1 {
		t.Fatal("the counts were not read in one grouped query")
	}
}

// ---- the sender's reads (D-5) -----------------------------------------------------------------------------

func TestOpenAccessTokenOpensOrRefreshesAsTheSystem(t *testing.T) {
	r := newOARig(t)
	r.withOA(t, 5*time.Hour)
	tok, oaID, err := r.uc.OpenAccessToken(r.ctx, oaRowID)
	if err != nil || string(tok.Lo()) != oaAccess || oaID != oaZaloID || r.zalo.calls["refresh"] != 0 {
		t.Fatalf("fresh: oa %q err %v calls %v", oaID, err, r.zalo.calls)
	}
	clear(tok)

	r2 := newOARig(t)
	r2.withOA(t, time.Hour)
	tok, _, err = r2.uc.OpenAccessToken(r2.ctx, oaRowID)
	if err != nil || string(tok.Lo()) != oaAccess+"-2" || r2.zalo.calls["refresh"] != 1 {
		t.Fatalf("due: err %v calls %v", err, r2.zalo.calls)
	}
	if e, _ := oaEntry(t, r2.f, domain.ActionRefreshZaloOAToken); e.args[1] != "system" {
		t.Fatalf("refresh actor = %v", e.args[1])
	}
	r2.noSecretAnywhere(t)

	r3 := newOARig(t)
	r3.withOA(t, 5*time.Hour)
	r3.oa.reauthSince, r3.oa.reauthReason = oaClock.Add(-time.Hour), domain.ReauthorizationRefreshTokenRejected
	if tok, _, err := r3.uc.OpenAccessToken(r3.ctx, oaRowID); !errors.Is(err, ErrZaloOANeedsReauthorization) || tok != nil {
		t.Fatalf("flagged: %v", err)
	}
}

func TestTemplateForSendNeedsALiveOAAndAnEnabledTemplate(t *testing.T) {
	r := newOARig(t)
	r.withOA(t, 5*time.Hour)
	r.tpls = []fakeTplRow{
		{id: "01JTPL1", templateID: "1001", status: "ENABLE", params: []byte(`[{"name":"ten_dot"}]`)},
		{id: "01JTPL2", templateID: "1002", status: "DISABLE", params: []byte(goodParams), match: true},
	}
	got, err := r.uc.TemplateForSend(r.ctx, oaRowID, "1001")
	if err != nil || got.ID != "01JTPL1" || len(got.Params) != 1 || got.Params[0].Name != "ten_dot" {
		t.Fatalf("got %+v err %v — a campaign template needs no milestone parameters", got, err)
	}
	if _, err := r.uc.TemplateForSend(r.ctx, oaRowID, "1002"); !errors.Is(err, ErrZaloOATemplateNotSendable) {
		t.Fatalf("disabled: %v", err)
	}
	if _, err := r.uc.TemplateForSend(r.ctx, oaRowID, "9999"); !errors.Is(err, ErrZaloOATemplateMissing) {
		t.Fatalf("unknown: %v", err)
	}
	if _, err := r.uc.TemplateForSend(r.ctx, "01JNOSUCHOA000000000000001", "1001"); !errors.Is(err, ErrCommuneZaloOAMissing) {
		t.Fatalf("unknown OA: %v", err)
	}
	r.oa.reauthSince, r.oa.reauthReason = oaClock.Add(-time.Hour), domain.ReauthorizationRefreshTokenRejected
	if _, err := r.uc.TemplateForSend(r.ctx, oaRowID, "1001"); !errors.Is(err, ErrZaloOANeedsReauthorization) {
		t.Fatalf("flagged: %v", err)
	}
	if len(r.zalo.calls) != 0 || r.f.begun != 0 {
		t.Fatal("a read called Zalo or opened a transaction")
	}
}
