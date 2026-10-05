package app

// The operator's acts on mini_app_secret (ADR 0066) and the two own-app fields of the bridge, on the
// fake driver of dang_nhap_giao_dich_test.go (transaction + audit_log) and a REAL core/crypto
// Envelope over an in-memory DEK store. The SQL itself runs in internal/store's pg tests.

import (
	"bytes"
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/crypto"
	"github.com/vihat/vigov/core/platformclient"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

const (
	testOwnApp    = "1234567890123456789"
	testAppSecret = "FAKE-APP-SECRET-0000"
)

// memDEKStore is crypto.DEKStore in a map keyed by the commune in ctx.
type memDEKStore struct {
	mu sync.Mutex
	m  map[tenant.ID]crypto.WrappedDEK
}

func (d *memDEKStore) GetDEK(ctx context.Context) (crypto.WrappedDEK, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if w, ok := d.m[tenant.MustFrom(ctx)]; ok {
		return w, nil
	}
	return crypto.WrappedDEK{}, crypto.ErrDEKNotFound
}

func (d *memDEKStore) CreateDEK(ctx context.Context, w crypto.WrappedDEK) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.m[tenant.MustFrom(ctx)]; ok {
		return crypto.ErrDEKExists
	}
	d.m[tenant.MustFrom(ctx)] = w
	return nil
}

func (d *memDEKStore) ReplaceDEK(ctx context.Context, _, w crypto.WrappedDEK) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.m[tenant.MustFrom(ctx)] = w
	return nil
}

func testEnvelope(t *testing.T) *crypto.Envelope {
	t.Helper()
	env, err := crypto.New([]secret.Secret{secret.Secret(bytes.Repeat([]byte{9}, crypto.KeyLength))},
		&memDEKStore{m: map[tenant.ID]crypto.WrappedDEK{}})
	if err != nil {
		t.Fatal(err)
	}
	return env
}

// versionRepoFake keeps the version rows of one commune and records the order of calls.
type versionRepoFake struct {
	rows  []idstore.MiniAppSecret
	dead  map[string]string // id -> reason
	by    map[string]string // id -> who retired it
	calls []string
	// listErr, when set, is what LiveStatuses answers.
	listErr error
}

func (r *versionRepoFake) live(appID string) (idstore.MiniAppSecret, bool) {
	for _, v := range r.rows {
		if v.AppID == appID {
			if _, gone := r.dead[v.ID]; !gone {
				return v, true
			}
		}
	}
	return idstore.MiniAppSecret{}, false
}

func (r *versionRepoFake) LiveForUpdate(_ context.Context, _ *store.ScopedTx, appID string) (idstore.MiniAppSecret, bool, error) {
	r.calls = append(r.calls, "lock")
	v, ok := r.live(appID)
	return v, ok, nil
}

func (r *versionRepoFake) Retire(_ context.Context, _ *store.ScopedTx, id, by, reason string) error {
	r.calls = append(r.calls, "retire")
	// CHECK mini_app_secret_soft_delete_complete: both non-blank. WHICH who and reason is asserted by
	// each path's own test (system + ticket for operatorctl, VH- + typed reason for an operator).
	if strings.TrimSpace(by) == "" || strings.TrimSpace(reason) == "" {
		return errors.New("CHECK mini_app_secret_soft_delete_complete violated")
	}
	r.dead[id] = reason
	if r.by == nil {
		r.by = map[string]string{}
	}
	r.by[id] = by
	return nil
}

func (r *versionRepoFake) Insert(_ context.Context, _ *store.ScopedTx, v idstore.MiniAppSecret) error {
	r.calls = append(r.calls, "insert")
	if _, ok := r.live(v.AppID); ok {
		return errors.New("unique (tenant_id, live_app_id) violated")
	}
	if v.Sealed == nil {
		return idstore.ErrMiniAppSecretNoSecret
	}
	r.rows = append(r.rows, v)
	return nil
}

func (r *versionRepoFake) LiveStatuses(context.Context) ([]idstore.MiniAppSecretStatus, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	var out []idstore.MiniAppSecretStatus
	for _, v := range r.rows {
		if _, gone := r.dead[v.ID]; !gone {
			out = append(out, idstore.MiniAppSecretStatus{ID: v.ID, AppID: v.AppID, SecretSet: v.Sealed != nil, SetAt: v.SetAt, SetBy: v.SetBy})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AppID < out[j].AppID })
	return out, nil
}

type bindingFake struct {
	apps map[string]platformclient.MiniApp
}

func (b bindingFake) MiniApp(_ context.Context, id string) (platformclient.MiniApp, bool, error) {
	a, ok := b.apps[id]
	return a, ok, nil
}

type adminRig struct {
	a   *MiniAppSecretAdmin
	r   *versionRepoFake
	g   *ghiChep
	env *crypto.Envelope
	ctx context.Context
}

func newAdminRig(t *testing.T) adminRig {
	t.Helper()
	db, g := moDB(t)
	r := &versionRepoFake{dead: map[string]string{}}
	env := testEnvelope(t)
	b := bindingFake{apps: map[string]platformclient.MiniApp{
		testOwnApp: {CheDo: platformclient.CheDoAppRieng, XaRieng: xaThu},
	}}
	a := NewMiniAppSecretAdmin(db, r, env, b, func() time.Time { return time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC) })
	return adminRig{a: a, r: r, g: g, env: env, ctx: tenant.Into(context.Background(), xaThu)}
}

// auditDeltas returns the delta argument of every audit_log insert carrying action.
func (rig adminRig) auditDeltas(action string) []string {
	rig.g.mu.Lock()
	defer rig.g.mu.Unlock()
	var out []string
	for _, l := range rig.g.lenh {
		if !strings.Contains(l.sql, "INSERT INTO audit_log") {
			continue
		}
		hasAction := false
		var delta string
		for _, a := range l.args {
			switch v := a.(type) {
			case string:
				if v == action {
					hasAction = true
				}
				if strings.HasPrefix(v, "{") {
					delta = v
				}
			case []byte:
				delta = string(v)
			}
		}
		if hasAction {
			out = append(out, delta)
		}
	}
	return out
}

func TestMiniAppSecretSetSealsAndAuditsWithoutTheValue(t *testing.T) {
	rig := newAdminRig(t)
	if err := rig.a.SetSecret(rig.ctx, testOwnApp, secret.Secret(testAppSecret), "OPS-1"); err != nil {
		t.Fatal(err)
	}
	v, ok := rig.r.live(testOwnApp)
	if !ok || v.Sealed == nil || v.SetBy != domain.SystemActor || v.ID == "" {
		t.Fatalf("live version = %+v", v)
	}
	if bytes.Contains(v.Sealed, []byte(testAppSecret)) {
		t.Fatal("the secret is stored in the clear")
	}
	// It opens with the AAD the sign-in uses ...
	got, err := rig.env.Open(rig.ctx, v.Sealed, miniAppSecretAAD(rig.ctx, testOwnApp))
	if err != nil || string(got.Lo()) != testAppSecret {
		t.Fatalf("open with the right AAD: %v", err)
	}
	// ... and NOT as another App ID's, nor in another commune.
	if _, err := rig.env.Open(rig.ctx, v.Sealed, miniAppSecretAAD(rig.ctx, "999")); !errors.Is(err, crypto.ErrOpenFailed) {
		t.Fatalf("opened under another App ID's AAD: %v", err)
	}
	ctxB := tenant.Into(context.Background(), xaKhac)
	if _, err := rig.env.Open(ctxB, v.Sealed, miniAppSecretAAD(ctxB, testOwnApp)); err == nil {
		t.Fatal("opened in another commune")
	}

	deltas := rig.auditDeltas(ActionSetOwnAppSecret)
	if len(deltas) != 1 || !strings.Contains(deltas[0], `"ly_do":"ticket:OPS-1"`) || !strings.Contains(deltas[0], `"co_secret":true`) {
		t.Fatalf("audit = %v", deltas)
	}
	if strings.Contains(deltas[0], testAppSecret) {
		t.Fatal("the secret reached the audit trail")
	}
	if rig.g.soGiaoDich() != 1 || rig.g.ketThucCua(1) != "commit" {
		t.Fatalf("transactions = %d, end = %q — want one committed", rig.g.soGiaoDich(), rig.g.ketThucCua(1))
	}
}

func TestMiniAppSecretEveryChangeIsANewVersion(t *testing.T) {
	rig := newAdminRig(t)
	if err := rig.a.SetSecret(rig.ctx, testOwnApp, secret.Secret(testAppSecret), "OPS-1"); err != nil {
		t.Fatal(err)
	}
	if err := rig.a.SetSecret(rig.ctx, testOwnApp, secret.Secret("FAKE-APP-SECRET-0001"), "OPS-2"); err != nil {
		t.Fatal(err)
	}
	if len(rig.r.rows) != 2 || len(rig.r.dead) != 1 {
		t.Fatalf("rows %d, retired %d — want 2 versions, 1 retired", len(rig.r.rows), len(rig.r.dead))
	}
	v, _ := rig.r.live(testOwnApp)
	if got, err := rig.env.Open(rig.ctx, v.Sealed, miniAppSecretAAD(rig.ctx, testOwnApp)); err != nil || string(got.Lo()) != "FAKE-APP-SECRET-0001" {
		t.Fatalf("the live version is not the new secret: %v", err)
	}
	if want := "lock,insert,lock,retire,insert"; strings.Join(rig.r.calls, ",") != want {
		t.Fatalf("calls = %v, want %s", rig.r.calls, want)
	}
	// The demo switch is gone (owner decision 05/10/2026): no new entry names it, not even as false.
	for _, d := range rig.auditDeltas(ActionSetOwnAppSecret) {
		if strings.Contains(d, "danh_tinh_demo") {
			t.Fatalf("a new trail entry still carries the retired demo switch: %s", d)
		}
	}
}

func TestMiniAppSecretRefusals(t *testing.T) {
	rig := newAdminRig(t)
	ctx := rig.ctx
	for name, c := range map[string]struct {
		err  error
		want error
	}{
		"no ticket":        {rig.a.SetSecret(ctx, testOwnApp, secret.Secret(testAppSecret), " "), ErrTicketRequired},
		"app id shape":     {rig.a.SetSecret(ctx, "abc", secret.Secret(testAppSecret), "OPS-1"), ErrMiniAppIDInvalid},
		"empty secret":     {rig.a.SetSecret(ctx, testOwnApp, nil, "OPS-1"), ErrMiniAppSecretInvalid},
		"secret has space": {rig.a.SetSecret(ctx, testOwnApp, secret.Secret("a b"), "OPS-1"), ErrMiniAppSecretInvalid},
		"not bound here": {rig.a.SetSecret(tenant.Into(context.Background(), xaKhac), testOwnApp,
			secret.Secret(testAppSecret), "OPS-1"), ErrMiniAppNotOwnApp},
		"unknown app":    {rig.a.SetSecret(ctx, "777", secret.Secret(testAppSecret), "OPS-1"), ErrMiniAppNotOwnApp},
		"retire nothing": {rig.a.Retire(ctx, testOwnApp, "OPS-1"), ErrMiniAppSettingsNotFound},
	} {
		if !errors.Is(c.err, c.want) {
			t.Errorf("%s: err = %v, want %v", name, c.err, c.want)
		}
	}
	if len(rig.r.rows) != 0 || len(rig.auditDeltas(ActionSetOwnAppSecret)) != 0 {
		t.Fatal("a refused act wrote something")
	}
}

// LiveStatuses: every live version of the commune, metadata only, no transaction and no trail entry
// (a pure read — operator.proto ListMiniAppSecretStatuses).
func TestMiniAppSecretLiveStatuses(t *testing.T) {
	rig := newAdminRig(t)
	got, err := rig.a.LiveStatuses(rig.ctx)
	if err != nil || len(got) != 0 {
		t.Fatalf("nothing set: %+v %v, want an empty list", got, err)
	}
	if _, err := rig.a.SetSecretAsOperator(rig.ctx, testOperator, testOwnApp, secret.Secret(testAppSecret), "đặt khoá"); err != nil {
		t.Fatal(err)
	}
	live, _ := rig.r.live(testOwnApp)
	txBefore := rig.g.soGiaoDich()
	got, err = rig.a.LiveStatuses(rig.ctx)
	if err != nil || len(got) != 1 {
		t.Fatalf("statuses = %+v %v", got, err)
	}
	if got[0] != (MiniAppSecretStatus{AppID: testOwnApp, Version: live.ID, SetAt: live.SetAt, SetBy: testOperatorCode, SecretSet: true}) {
		t.Fatalf("status = %+v, want the live version's metadata", got[0])
	}
	if rig.g.soGiaoDich() != txBefore {
		t.Fatal("a pure read opened a transaction")
	}
	// Retired: no longer listed.
	if err := rig.a.Retire(rig.ctx, testOwnApp, "OPS-2"); err != nil {
		t.Fatal(err)
	}
	if got, err := rig.a.LiveStatuses(rig.ctx); err != nil || len(got) != 0 {
		t.Fatalf("after retire: %+v %v", got, err)
	}
	rig.r.listErr = errors.New("store down")
	if _, err := rig.a.LiveStatuses(rig.ctx); err == nil || !errors.Is(err, rig.r.listErr) {
		t.Fatalf("store error not wrapped: %v", err)
	}
}

func TestMiniAppRetireAudited(t *testing.T) {
	rig := newAdminRig(t)
	if err := rig.a.SetSecret(rig.ctx, testOwnApp, secret.Secret(testAppSecret), "OPS-1"); err != nil {
		t.Fatal(err)
	}
	if err := rig.a.Retire(rig.ctx, testOwnApp, "OPS-9"); err != nil {
		t.Fatal(err)
	}
	if _, ok := rig.r.live(testOwnApp); ok {
		t.Fatal("still live after retire")
	}
	d := rig.auditDeltas(ActionRetireOwnAppConfig)
	if len(d) != 1 || !strings.Contains(d[0], "ticket:OPS-9") || strings.Contains(d[0], testAppSecret) {
		t.Fatalf("audit = %v", d)
	}
}

// --- the two own-app fields of the bridge ----------------------------------------------------------

func TestBridgeRequireOwnAppOfRefusesAMovedBinding(t *testing.T) {
	b := dungBanThuCau(t)
	yc := yeuCauCau(cauAppRieng)
	yc.RequireOwnAppOf = xaKhac // the commune whose secret was checked is not the one bound now
	if _, err := b.uc.Mo(context.Background(), yc); !errors.Is(err, ErrCauAppChuaSanSang) {
		t.Fatalf("err = %v, want ErrCauAppChuaSanSang", err)
	}
	yc = yeuCauCau(cauAppChinh)
	yc.RequireOwnAppOf = xaThu // the shared app is never an own app
	if _, err := b.uc.Mo(context.Background(), yc); !errors.Is(err, ErrCauAppChuaSanSang) {
		t.Fatalf("shared app: err = %v", err)
	}
	if b.g.soGiaoDich() != 0 {
		t.Fatal("a refused own-app open started a transaction")
	}
	yc = yeuCauCau(cauAppRieng)
	yc.RequireOwnAppOf = xaThu
	if kq, err := b.uc.Mo(context.Background(), yc); err != nil || kq.Xa != xaThu {
		t.Fatalf("matching binding: %+v %v", kq, err)
	}
}

// The demo identity input is gone (owner decision 05/10/2026): no session entry carries the flag.
// Entries written before that keep theirs — this pins only what is written from now on.
func TestBridgeSessionEntryCarriesNoDemoFlag(t *testing.T) {
	b := dungBanThuCau(t)
	if _, err := b.uc.Mo(context.Background(), yeuCauCau(cauAppRieng)); err != nil {
		t.Fatal(err)
	}
	entries := b.vet(HanhDongMoPhienCongDan)
	if len(entries) != 1 {
		t.Fatalf("entries = %d", len(entries))
	}
	for _, a := range entries[0].args {
		switch d := a.(type) {
		case []byte:
			if strings.Contains(string(d), "danh_tinh_demo") {
				t.Fatal("a session entry is marked demo")
			}
		case string:
			if strings.Contains(d, "danh_tinh_demo") {
				t.Fatal("a session entry is marked demo")
			}
		}
	}
}
