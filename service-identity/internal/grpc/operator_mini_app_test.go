package grpc

// SetMiniAppSecret / RetireMiniAppSecret (operator.proto, ADR 0070 §"Bổ sung 02/10/2026" #6): the
// status table and outcomes of the two RPCs that act ON a commune, on fakes. The use case's own
// behaviour (version rows, trail entry, binding check) is internal/app/mini_app_secret_operator_test.go.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

const (
	maCommune      = tenant.ID("01J0000000000000000000000A")
	maAppID        = "1234567890123456789"
	maSecretMarker = "MA-MARKER-APP-SECRET-not-real"
	maReasonMarker = "MA-MARKER-REASON"
	maClientIP     = "198.51.100.7"
	maVersion      = "01JVERSIONAAAAAAAAAAAAAAAA"
)

var maAt = time.Date(2026, 10, 2, 9, 30, 0, 0, time.UTC)

// miniAppsFake records what the handler passed and answers err.
type miniAppsFake struct {
	err       error
	calls     int
	gotOp     app.MiniAppOperator
	gotApp    string
	gotSecret secret.Secret
	gotReason string
	gotTenant tenant.ID
	statuses  map[tenant.ID][]app.MiniAppSecretStatus
}

func (f *miniAppsFake) SetSecretAsOperator(ctx context.Context, op app.MiniAppOperator, appID string, v secret.Secret,
	reason string) (app.MiniAppSecretSet, error) {
	f.calls++
	f.gotOp, f.gotApp, f.gotSecret, f.gotReason = op, appID, v, reason
	f.gotTenant, _ = tenant.From(ctx)
	if f.err != nil {
		return app.MiniAppSecretSet{}, f.err
	}
	return app.MiniAppSecretSet{AppID: strings.TrimSpace(appID), Version: maVersion, SetAt: maAt, SetBy: op.Code}, nil
}

func (f *miniAppsFake) RetireAsOperator(ctx context.Context, op app.MiniAppOperator, appID, reason string) (app.MiniAppSecretRetired, error) {
	f.calls++
	f.gotOp, f.gotApp, f.gotReason = op, appID, reason
	f.gotTenant, _ = tenant.From(ctx)
	if f.err != nil {
		return app.MiniAppSecretRetired{}, f.err
	}
	return app.MiniAppSecretRetired{AppID: strings.TrimSpace(appID), Version: maVersion, RetiredAt: maAt, RetiredBy: op.Code}, nil
}

// LiveStatuses answers the statuses stored per commune — so a test can show that the commune of the
// metadata, and only it, decides which rows come back.
func (f *miniAppsFake) LiveStatuses(ctx context.Context) ([]app.MiniAppSecretStatus, error) {
	f.calls++
	f.gotTenant, _ = tenant.From(ctx)
	if f.err != nil {
		return nil, f.err
	}
	return f.statuses[f.gotTenant], nil
}

func managerPrincipal() app.OperatorPrincipal {
	return app.OperatorPrincipal{ID: "01JDOPERATORAAAAAAAAAAAAAA", Code: "VH-00001",
		Permissions: []domain.OperatorPermission{domain.OperatorPermissionTenantManage, domain.OperatorPermissionMiniAppManage}}
}

type maRig struct {
	s    *OperatorServer
	uc   *operatorAuthFake
	m    *miniAppsFake
	logs *bytes.Buffer
}

func newMARig(ucErr, mErr error, p app.OperatorPrincipal) maRig {
	logs := &bytes.Buffer{}
	uc := &operatorAuthFake{err: ucErr, principal: p}
	m := &miniAppsFake{err: mErr}
	s := NewOperatorServer(uc, slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))).WithMiniAppSecrets(m)
	return maRig{s: s, uc: uc, m: m, logs: logs}
}

func maCtx() context.Context { return tenant.Into(context.Background(), maCommune) }

func setReq() *identityv1.SetMiniAppSecretRequest {
	return &identityv1.SetMiniAppSecretRequest{SessionToken: opMarkToken, AppId: maAppID, AppSecret: maSecretMarker,
		Reason: maReasonMarker, ClientIp: maClientIP}
}

func retireReq() *identityv1.RetireMiniAppSecretRequest {
	return &identityv1.RetireMiniAppSecretRequest{SessionToken: opMarkToken, AppId: maAppID, Reason: maReasonMarker, ClientIp: maClientIP}
}

func TestSetMiniAppSecretAccepted(t *testing.T) {
	r := newMARig(nil, nil, managerPrincipal())
	resp, err := r.s.SetMiniAppSecret(maCtx(), setReq())
	if err != nil || resp.GetOutcome() != outAccepted {
		t.Fatalf("resp %v err %v, want ACCEPTED", resp.GetOutcome(), err)
	}
	if r.uc.gotToken != opMarkToken {
		t.Error("the operator was not resolved from the request's token")
	}
	if r.m.gotOp != (app.MiniAppOperator{Code: "VH-00001", IP: maClientIP}) || r.m.gotTenant != maCommune {
		t.Fatalf("use case got operator %+v in commune %q — want the resolved VH- code, client_ip, and the metadata commune",
			r.m.gotOp, r.m.gotTenant)
	}
	if string(r.m.gotSecret.Lo()) != maSecretMarker || r.m.gotReason != maReasonMarker || r.m.gotApp != maAppID {
		t.Fatal("the use case did not receive the request's values")
	}
	v := resp.GetVersion()
	if v.GetAppId() != maAppID || v.GetVersion() != maVersion || v.GetSetBy() != "VH-00001" || !v.GetSetAt().AsTime().Equal(maAt) {
		t.Fatalf("version = %v", v)
	}
	if strings.Contains(resp.String(), maSecretMarker) {
		t.Fatal("the response carries the secret")
	}
}

func TestRetireMiniAppSecretAccepted(t *testing.T) {
	r := newMARig(nil, nil, managerPrincipal())
	resp, err := r.s.RetireMiniAppSecret(maCtx(), retireReq())
	if err != nil || resp.GetOutcome() != outAccepted {
		t.Fatalf("resp %v err %v, want ACCEPTED", resp.GetOutcome(), err)
	}
	rt := resp.GetRetirement()
	if rt.GetAppId() != maAppID || rt.GetRetiredVersion() != maVersion || rt.GetRetiredBy() != "VH-00001" ||
		!rt.GetRetiredAt().AsTime().Equal(maAt) {
		t.Fatalf("retirement = %v", rt)
	}
	if r.m.gotOp.Code != "VH-00001" || r.m.gotOp.IP != maClientIP || r.m.gotTenant != maCommune {
		t.Fatalf("use case got %+v / %q", r.m.gotOp, r.m.gotTenant)
	}
}

// SESSION_NOT_LIVE and PERMISSION_DENIED: answered, nothing written.
func TestMiniAppSecretRPCsSessionAndPermission(t *testing.T) {
	noKey := managerPrincipal()
	noKey.Permissions = []domain.OperatorPermission{domain.OperatorPermissionTenantManage}
	for name, c := range map[string]struct {
		ucErr error
		p     app.OperatorPrincipal
		want  identityv1.OperatorAuthOutcome
	}{
		"not live":       {app.ErrOperatorUnauthenticated, app.OperatorPrincipal{}, outNotLive},
		"no key":         {nil, noKey, outDenied},
		"no keys at all": {nil, app.OperatorPrincipal{ID: "x", Code: "VH-00002"}, outDenied},
	} {
		t.Run(name, func(t *testing.T) {
			r := newMARig(c.ucErr, nil, c.p)
			sr, err := r.s.SetMiniAppSecret(maCtx(), setReq())
			if err != nil || sr.GetOutcome() != c.want || sr.GetVersion() != nil {
				t.Errorf("Set: %v %v, want %v and no version", sr, err, c.want)
			}
			rr, err := r.s.RetireMiniAppSecret(maCtx(), retireReq())
			if err != nil || rr.GetOutcome() != c.want || rr.GetRetirement() != nil {
				t.Errorf("Retire: %v %v, want %v and no retirement", rr, err, c.want)
			}
			if r.m.calls != 0 {
				t.Fatal("the use case ran")
			}
		})
	}
}

// Every INVALID_ARGUMENT is decided from the request alone: the session is NOT resolved (that is a
// lookup, and it refreshes the idle timer), the use case does not run, and the message quotes nothing.
func TestMiniAppSecretRPCsInvalidArgumentBeforeAnyLookup(t *testing.T) {
	type call func(s *OperatorServer, ctx context.Context) error
	set := func(mut func(*identityv1.SetMiniAppSecretRequest)) call {
		return func(s *OperatorServer, ctx context.Context) error {
			req := setReq()
			mut(req)
			_, err := s.SetMiniAppSecret(ctx, req)
			return err
		}
	}
	ret := func(mut func(*identityv1.RetireMiniAppSecretRequest)) call {
		return func(s *OperatorServer, ctx context.Context) error {
			req := retireReq()
			mut(req)
			_, err := s.RetireMiniAppSecret(ctx, req)
			return err
		}
	}
	noCommune := func(c call) call {
		return func(s *OperatorServer, _ context.Context) error { return c(s, context.Background()) }
	}
	cases := map[string]call{
		"set no commune":        noCommune(set(func(*identityv1.SetMiniAppSecretRequest) {})),
		"retire no commune":     noCommune(ret(func(*identityv1.RetireMiniAppSecretRequest) {})),
		"set empty token":       set(func(r *identityv1.SetMiniAppSecretRequest) { r.SessionToken = "" }),
		"retire empty token":    ret(func(r *identityv1.RetireMiniAppSecretRequest) { r.SessionToken = "" }),
		"set app id letters":    set(func(r *identityv1.SetMiniAppSecretRequest) { r.AppId = "abc" }),
		"set app id 33":         set(func(r *identityv1.SetMiniAppSecretRequest) { r.AppId = strings.Repeat("1", 33) }),
		"set app id empty":      set(func(r *identityv1.SetMiniAppSecretRequest) { r.AppId = " " }),
		"retire app id":         ret(func(r *identityv1.RetireMiniAppSecretRequest) { r.AppId = "12a" }),
		"set secret empty":      set(func(r *identityv1.SetMiniAppSecretRequest) { r.AppSecret = "" }),
		"set secret space":      set(func(r *identityv1.SetMiniAppSecretRequest) { r.AppSecret = "MA-MARKER secret" }),
		"set secret 257":        set(func(r *identityv1.SetMiniAppSecretRequest) { r.AppSecret = strings.Repeat("M", 257) }),
		"set secret newline":    set(func(r *identityv1.SetMiniAppSecretRequest) { r.AppSecret = maSecretMarker + "\n" }),
		"set reason empty":      set(func(r *identityv1.SetMiniAppSecretRequest) { r.Reason = "  " }),
		"set reason 501":        set(func(r *identityv1.SetMiniAppSecretRequest) { r.Reason = strings.Repeat("ạ", 501) }),
		"set reason control":    set(func(r *identityv1.SetMiniAppSecretRequest) { r.Reason = "a\x07b" }),
		"retire reason empty":   ret(func(r *identityv1.RetireMiniAppSecretRequest) { r.Reason = "" }),
		"retire reason invalid": ret(func(r *identityv1.RetireMiniAppSecretRequest) { r.Reason = "a\xffb" }),
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			r := newMARig(nil, nil, managerPrincipal())
			err := c(r.s, maCtx())
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("code = %v (%v), want InvalidArgument", status.Code(err), err)
			}
			if r.uc.calls != 0 || r.m.calls != 0 {
				t.Fatalf("session resolved %d times, use case %d — want neither", r.uc.calls, r.m.calls)
			}
			msg := status.Convert(err).Message()
			for _, v := range []string{maSecretMarker, maReasonMarker, "MA-MARKER", opMarkToken} {
				if strings.Contains(msg, v) {
					t.Fatalf("message %q quotes a value", msg)
				}
			}
		})
	}
}

func TestMiniAppSecretRPCsUseCaseErrors(t *testing.T) {
	// Set: no live own-app binding of this App ID to this commune → FAILED_PRECONDITION.
	r := newMARig(nil, fmt.Errorf("wrapped: %w", app.ErrMiniAppNotOwnApp), managerPrincipal())
	if _, err := r.s.SetMiniAppSecret(maCtx(), setReq()); status.Code(err) != codes.FailedPrecondition {
		t.Errorf("Set not own app: %v, want FailedPrecondition", status.Code(err))
	}
	// Retire: no live settings → NOT_FOUND.
	r = newMARig(nil, app.ErrMiniAppSettingsNotFound, managerPrincipal())
	if _, err := r.s.RetireMiniAppSecret(maCtx(), retireReq()); status.Code(err) != codes.NotFound {
		t.Errorf("Retire nothing live: %v, want NotFound", status.Code(err))
	}
	// NotFound is Retire's only: on Set it is not a listed answer → Internal, fail closed.
	r = newMARig(nil, app.ErrMiniAppSettingsNotFound, managerPrincipal())
	if _, err := r.s.SetMiniAppSecret(maCtx(), setReq()); status.Code(err) != codes.Internal {
		t.Errorf("Set with an unlisted sentinel: %v, want Internal", status.Code(err))
	}
	// Realm not configured, from ResolveSession → FAILED_PRECONDITION (the service table's meaning).
	r = newMARig(app.ErrOperatorRealmNotConfigured, nil, app.OperatorPrincipal{})
	if _, err := r.s.RetireMiniAppSecret(maCtx(), retireReq()); status.Code(err) != codes.FailedPrecondition {
		t.Errorf("realm not configured: %v, want FailedPrecondition", status.Code(err))
	}
	// A principal with no VH- code is never acted as: Internal, use case not run.
	p := managerPrincipal()
	p.Code = ""
	r = newMARig(nil, nil, p)
	if _, err := r.s.SetMiniAppSecret(maCtx(), setReq()); status.Code(err) != codes.Internal || r.m.calls != 0 {
		t.Errorf("principal without a code: %v, calls %d — want Internal and no write", status.Code(err), r.m.calls)
	}
	// Not wired: Unimplemented, as before the RPCs existed.
	s := NewOperatorServer(&operatorAuthFake{principal: managerPrincipal()}, nil)
	if _, err := s.SetMiniAppSecret(maCtx(), setReq()); status.Code(err) != codes.Unimplemented {
		t.Errorf("unwired Set: %v", status.Code(err))
	}
	if _, err := s.RetireMiniAppSecret(maCtx(), retireReq()); status.Code(err) != codes.Unimplemented {
		t.Errorf("unwired Retire: %v", status.Code(err))
	}
}

// The secret, the token and the reason never reach a log line or a status message — on every path,
// including the Internal one that logs its cause.
func TestMiniAppSecretRPCsNeverLogValues(t *testing.T) {
	noKey := managerPrincipal()
	noKey.Permissions = nil
	for name, c := range map[string]struct {
		ucErr, mErr error
		p           app.OperatorPrincipal
	}{
		"accepted":    {nil, nil, managerPrincipal()},
		"not live":    {app.ErrOperatorUnauthenticated, nil, app.OperatorPrincipal{}},
		"denied":      {nil, nil, noKey},
		"not own app": {nil, app.ErrMiniAppNotOwnApp, managerPrincipal()},
		"not found":   {nil, app.ErrMiniAppSettingsNotFound, managerPrincipal()},
		"internal":    {nil, errors.New("store exploded"), managerPrincipal()},
		"resolve err": {errors.New("operator store down"), nil, app.OperatorPrincipal{}},
	} {
		t.Run(name, func(t *testing.T) {
			r := newMARig(c.ucErr, c.mErr, c.p)
			_, errS := r.s.SetMiniAppSecret(maCtx(), setReq())
			_, errR := r.s.RetireMiniAppSecret(maCtx(), retireReq())
			for _, v := range []string{maSecretMarker, opMarkToken, maReasonMarker} {
				if strings.Contains(r.logs.String(), v) {
					t.Fatalf("%q reached the log: %s", v, r.logs.String())
				}
				for _, e := range []error{errS, errR} {
					if e != nil && strings.Contains(e.Error(), v) {
						t.Fatalf("%q reached a status: %v", v, e)
					}
				}
			}
		})
	}
	// The secret, formatted the way a careless log line would, prints the mask.
	if strings.Contains(fmt.Sprintf("%v %+v %s", secret.Secret(maSecretMarker), secret.Secret(maSecretMarker),
		secret.Secret(maSecretMarker)), maSecretMarker) {
		t.Fatal("secret.Secret formats its value")
	}
}

// --- ListMiniAppSecretStatuses (owner decision 05/10/2026) -------------------------------------------
//
// Rule 5 #7 in this service's shape: there is no HTTP edge here, so 401/403 are outcomes —
// SESSION_NOT_LIVE (no live session), PERMISSION_DENIED (no key), the commune dimension (the rows are
// those of the METADATA commune and of no other), and ACCEPTED.

const maCommuneOther = tenant.ID("01J0000000000000000000000B")

func listReq() *identityv1.ListMiniAppSecretStatusesRequest {
	return &identityv1.ListMiniAppSecretStatusesRequest{SessionToken: opMarkToken}
}

func listRig(ucErr error, p app.OperatorPrincipal) maRig {
	r := newMARig(ucErr, nil, p)
	r.m.statuses = map[tenant.ID][]app.MiniAppSecretStatus{
		maCommune: {
			{AppID: "1111", Version: maVersion, SetAt: maAt, SetBy: "VH-00001", SecretSet: true},
			{AppID: "2222", Version: "01JVERSIONBBBBBBBBBBBBBBBB", SetAt: maAt.Add(time.Hour), SetBy: "system", SecretSet: false},
		},
		maCommuneOther: {
			{AppID: "9999", Version: "01JVERSIONCCCCCCCCCCCCCCCC", SetAt: maAt, SetBy: "VH-00009", SecretSet: true},
		},
	}
	return r
}

// ACCEPTED: every live version of the metadata commune, metadata + one bool. A principal holding ANY
// ops.* key may read — not only ops.mini_app.manage (ADR 0073 #1).
func TestListMiniAppSecretStatusesAccepted(t *testing.T) {
	readOnly := managerPrincipal()
	readOnly.Permissions = []domain.OperatorPermission{domain.OperatorPermissionQRIssue}
	for name, p := range map[string]app.OperatorPrincipal{"manager": managerPrincipal(), "any other ops key": readOnly} {
		t.Run(name, func(t *testing.T) {
			r := listRig(nil, p)
			resp, err := r.s.ListMiniAppSecretStatuses(maCtx(), listReq())
			if err != nil || resp.GetOutcome() != outAccepted {
				t.Fatalf("resp %v err %v, want ACCEPTED", resp.GetOutcome(), err)
			}
			if r.uc.gotToken != opMarkToken || r.m.gotTenant != maCommune {
				t.Fatalf("token %q / commune %q — want the request's token and the metadata commune", r.uc.gotToken, r.m.gotTenant)
			}
			st := resp.GetStatuses()
			if len(st) != 2 {
				t.Fatalf("statuses = %v", st)
			}
			a, b := st[0], st[1]
			if v := a.GetLiveVersion(); !a.GetSecretSet() || v.GetAppId() != "1111" || v.GetVersion() != maVersion ||
				v.GetSetBy() != "VH-00001" || !v.GetSetAt().AsTime().Equal(maAt) {
				t.Errorf("first = %v", a)
			}
			if v := b.GetLiveVersion(); b.GetSecretSet() || v.GetAppId() != "2222" || v.GetSetBy() != "system" {
				t.Errorf("second = %v — secret_set must be false for a live row with no secret", b)
			}
		})
	}
}

// The commune dimension: the same operator, the same token, another commune in the metadata → that
// commune's rows only; a commune with nothing live → ACCEPTED with an EMPTY list, never NOT_FOUND.
func TestListMiniAppSecretStatusesScopedToTheMetadataCommune(t *testing.T) {
	r := listRig(nil, managerPrincipal())
	resp, err := r.s.ListMiniAppSecretStatuses(tenant.Into(context.Background(), maCommuneOther), listReq())
	if err != nil || resp.GetOutcome() != outAccepted || r.m.gotTenant != maCommuneOther {
		t.Fatalf("resp %v err %v commune %q", resp.GetOutcome(), err, r.m.gotTenant)
	}
	if st := resp.GetStatuses(); len(st) != 1 || st[0].GetLiveVersion().GetAppId() != "9999" {
		t.Fatalf("statuses = %v — want commune B's row only", st)
	}
	resp, err = r.s.ListMiniAppSecretStatuses(tenant.Into(context.Background(), "01J000000000000000000000ZZ"), listReq())
	if err != nil || resp.GetOutcome() != outAccepted || len(resp.GetStatuses()) != 0 {
		t.Fatalf("empty commune: %v %v, want ACCEPTED with no statuses", resp, err)
	}
}

// SESSION_NOT_LIVE and PERMISSION_DENIED: answered, the store is not read, no status is returned.
func TestListMiniAppSecretStatusesSessionAndPermission(t *testing.T) {
	unknownOnly := managerPrincipal()
	unknownOnly.Permissions = []domain.OperatorPermission{"ops.not_a_key", "admin.user"}
	for name, c := range map[string]struct {
		ucErr error
		p     app.OperatorPrincipal
		want  identityv1.OperatorAuthOutcome
	}{
		"not live":          {app.ErrOperatorUnauthenticated, app.OperatorPrincipal{}, outNotLive},
		"no keys at all":    {nil, app.OperatorPrincipal{ID: "x", Code: "VH-00002"}, outDenied},
		"only unknown keys": {nil, unknownOnly, outDenied},
	} {
		t.Run(name, func(t *testing.T) {
			r := listRig(c.ucErr, c.p)
			resp, err := r.s.ListMiniAppSecretStatuses(maCtx(), listReq())
			if err != nil || resp.GetOutcome() != c.want || len(resp.GetStatuses()) != 0 {
				t.Fatalf("%v %v, want %v and no statuses", resp, err, c.want)
			}
			if r.m.calls != 0 {
				t.Fatal("the store was read")
			}
		})
	}
}

// INVALID_ARGUMENT before any lookup; store errors are Internal (fail closed) with nothing quoted;
// unwired is Unimplemented.
func TestListMiniAppSecretStatusesFaults(t *testing.T) {
	r := listRig(nil, managerPrincipal())
	if _, err := r.s.ListMiniAppSecretStatuses(context.Background(), listReq()); status.Code(err) != codes.InvalidArgument {
		t.Errorf("no commune: %v, want InvalidArgument", status.Code(err))
	}
	if _, err := r.s.ListMiniAppSecretStatuses(maCtx(), &identityv1.ListMiniAppSecretStatusesRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("empty token: %v, want InvalidArgument", status.Code(err))
	}
	if r.uc.calls != 0 || r.m.calls != 0 {
		t.Fatalf("session resolved %d times, store read %d — want neither", r.uc.calls, r.m.calls)
	}

	r = listRig(nil, managerPrincipal())
	r.m.err = errors.New("store exploded")
	_, err := r.s.ListMiniAppSecretStatuses(maCtx(), listReq())
	if status.Code(err) != codes.Internal || strings.Contains(err.Error(), "exploded") || strings.Contains(r.logs.String(), opMarkToken) {
		t.Fatalf("store error: %v / log %s", err, r.logs.String())
	}

	r = listRig(app.ErrOperatorRealmNotConfigured, app.OperatorPrincipal{})
	if _, err := r.s.ListMiniAppSecretStatuses(maCtx(), listReq()); status.Code(err) != codes.FailedPrecondition {
		t.Errorf("realm not configured: %v, want FailedPrecondition", status.Code(err))
	}

	s := NewOperatorServer(&operatorAuthFake{principal: managerPrincipal()}, nil)
	if _, err := s.ListMiniAppSecretStatuses(maCtx(), listReq()); status.Code(err) != codes.Unimplemented {
		t.Errorf("unwired: %v, want Unimplemented", status.Code(err))
	}
}
