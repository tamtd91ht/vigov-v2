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
