package operatorclient

// What these tests defend for the two RPCs that act ON a commune: the target commune TRAVELS (they
// are not exempt), a context without one is refused before anything is sent, every status and
// outcome maps to the error the HTTP edge turns into its status, and the secret never reaches a log.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/secret"
)

// Fake material — says so in full (rule 8, forbidden #1).
const (
	appSecretFake = "zalo-app-secret-FAKE-NOT-REAL"
	appIDFake     = "3291993990104489440"
)

func (f *fakeServer) SetMiniAppSecret(ctx context.Context, r *identityv1.SetMiniAppSecretRequest) (*identityv1.SetMiniAppSecretResponse, error) {
	f.record(ctx)
	f.sawSet = r
	return f.setSecret, f.err
}

func (f *fakeServer) RetireMiniAppSecret(ctx context.Context, _ *identityv1.RetireMiniAppSecretRequest) (*identityv1.RetireMiniAppSecretResponse, error) {
	f.record(ctx)
	return f.retireSecret, f.err
}

func setReq() SetMiniAppSecretRequest {
	return SetMiniAppSecretRequest{Token: secret.Secret(tokenFake), AppID: appIDFake,
		AppSecret: secret.Secret(appSecretFake), Reason: "Xã đổi App ID", ClientIP: "203.0.113.7"}
}

func retireReq() RetireMiniAppSecretRequest {
	return RetireMiniAppSecretRequest{Token: secret.Secret(tokenFake), AppID: appIDFake, Reason: "Xã đổi App ID"}
}

func TestSecretRPCsCarryTheTargetCommune(t *testing.T) {
	f := &fakeServer{
		setSecret: &identityv1.SetMiniAppSecretResponse{Outcome: wAccepted, Version: &identityv1.MiniAppSecretVersion{
			AppId: appIDFake, Version: "01JDVERSIONFAKE00000000000", SetAt: timestamppb.New(time.Unix(1_800_000_000, 0)), SetBy: "VH-00001"}},
		retireSecret: &identityv1.RetireMiniAppSecretResponse{Outcome: wAccepted, Retirement: &identityv1.MiniAppSecretRetirement{
			AppId: appIDFake, RetiredVersion: "01JDVERSIONFAKE00000000000", RetiredAt: timestamppb.New(time.Unix(1_800_000_000, 0)), RetiredBy: "VH-00001"}},
	}
	c, _ := start(t, f)
	ctx := ctxWithTenant()

	res, err := c.SetMiniAppSecret(ctx, setReq())
	if err != nil || res.Outcome != OutcomeAccepted || res.Version.SetBy != "VH-00001" || res.Version.SetAt.Unix() != 1_800_000_000 {
		t.Fatalf("Set: %+v %v", res, err)
	}
	if f.sawSet.GetAppSecret() != appSecretFake || f.sawSet.GetSessionToken() != tokenFake || f.sawSet.GetClientIp() != "203.0.113.7" {
		t.Fatal("Set fields did not travel verbatim")
	}
	ret, err := c.RetireMiniAppSecret(ctx, retireReq())
	if err != nil || ret.Outcome != OutcomeAccepted || ret.Retirement.RetiredBy != "VH-00001" {
		t.Fatalf("Retire: %+v %v", ret, err)
	}
	want := "01JA" + strings.Repeat("A", 22)
	for i, v := range f.sawTenant {
		if len(v) != 1 || v[0] != want {
			t.Errorf("call %d carried x-tenant-id=%v, want exactly [%s]", i+1, v, want)
		}
	}
}

func TestSecretRPCsRefuseLocallyWithoutCommuneOrToken(t *testing.T) {
	f := &fakeServer{}
	c, _ := start(t, f)
	if _, err := c.SetMiniAppSecret(context.Background(), setReq()); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("Set without a commune: %v", err)
	}
	if _, err := c.RetireMiniAppSecret(context.Background(), retireReq()); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("Retire without a commune: %v", err)
	}
	r := setReq()
	r.Token = nil
	if _, err := c.SetMiniAppSecret(ctxWithTenant(), r); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("Set without a token: %v", err)
	}
	if f.calls != 0 {
		t.Fatalf("%d refused calls reached the server", f.calls)
	}
}

func TestSecretRPCStatusMapping(t *testing.T) {
	for _, c := range []struct {
		name  string
		code  codes.Code
		set   error // what Set must answer
		retir error // what Retire must answer
	}{
		{"FAILED_PRECONDITION", codes.FailedPrecondition, ErrMiniAppNotBound, ErrRealmNotConfigured},
		{"NOT_FOUND", codes.NotFound, nil, ErrNoLiveSecret},
		{"INVALID_ARGUMENT", codes.InvalidArgument, ErrArgumentRefused, ErrArgumentRefused},
		{"UNAVAILABLE", codes.Unavailable, nil, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := &fakeServer{err: status.Error(c.code, "x")}
			cl, logBuf := start(t, f)
			_, errSet := cl.SetMiniAppSecret(ctxWithTenant(), setReq())
			_, errRet := cl.RetireMiniAppSecret(ctxWithTenant(), retireReq())
			if errSet == nil || errRet == nil {
				t.Fatalf("a failed call answered no error: set=%v retire=%v", errSet, errRet)
			}
			if c.set != nil && !errors.Is(errSet, c.set) {
				t.Errorf("Set: %v, want %v", errSet, c.set)
			}
			if c.retir != nil && !errors.Is(errRet, c.retir) {
				t.Errorf("Retire: %v, want %v", errRet, c.retir)
			}
			// Outages are none of the typed answers: the edge must read them as 503.
			if c.code == codes.Unavailable {
				for _, e := range []error{ErrMiniAppNotBound, ErrNoLiveSecret, ErrArgumentRefused} {
					if errors.Is(errSet, e) || errors.Is(errRet, e) {
						t.Errorf("an outage was read as %v", e)
					}
				}
			}
			if strings.Contains(logBuf.String(), appSecretFake) || strings.Contains(logBuf.String(), tokenFake) {
				t.Fatalf("the log carries the secret or the token: %s", logBuf.String())
			}
		})
	}
}

func TestSecretRPCOutcomes(t *testing.T) {
	f := &fakeServer{
		setSecret:    &identityv1.SetMiniAppSecretResponse{Outcome: wDenied},
		retireSecret: &identityv1.RetireMiniAppSecretResponse{Outcome: wNotLive},
	}
	c, _ := start(t, f)
	ctx := ctxWithTenant()
	if res, err := c.SetMiniAppSecret(ctx, setReq()); err != nil || res.Outcome != OutcomePermissionDenied {
		t.Errorf("PERMISSION_DENIED: %v %v", res.Outcome, err)
	}
	if res, err := c.RetireMiniAppSecret(ctx, retireReq()); err != nil || res.Outcome != OutcomeSessionNotLive {
		t.Errorf("SESSION_NOT_LIVE: %v %v", res.Outcome, err)
	}
	f.setSecret = &identityv1.SetMiniAppSecretResponse{Outcome: wAccepted}
	if _, err := c.SetMiniAppSecret(ctx, setReq()); !errors.Is(err, ErrContract) {
		t.Errorf("ACCEPTED without a version: %v", err)
	}
	f.setSecret = &identityv1.SetMiniAppSecretResponse{}
	if _, err := c.SetMiniAppSecret(ctx, setReq()); !errors.Is(err, ErrContract) {
		t.Errorf("UNSPECIFIED: %v, want a contract fault (no credential to be refused)", err)
	}
	f.retireSecret = &identityv1.RetireMiniAppSecretResponse{Outcome: wRefused}
	if _, err := c.RetireMiniAppSecret(ctx, retireReq()); !errors.Is(err, ErrContract) {
		t.Errorf("REFUSED is outside the subset: %v", err)
	}
}

func (f *fakeServer) ListMiniAppSecretStatuses(ctx context.Context, _ *identityv1.ListMiniAppSecretStatusesRequest) (*identityv1.ListMiniAppSecretStatusesResponse, error) {
	f.record(ctx)
	return f.statuses, f.err
}

func statusFake(appID string, set bool) *identityv1.MiniAppSecretStatus {
	return &identityv1.MiniAppSecretStatus{SecretSet: set, LiveVersion: &identityv1.MiniAppSecretVersion{
		AppId: appID, Version: "01JDVERSIONFAKE00000000000", SetAt: timestamppb.New(time.Unix(1_800_000_000, 0)), SetBy: "VH-00001"}}
}

// The read carries the target commune like the two writes, and a version WITHOUT a secret never
// reaches the caller dated: "chưa đặt" must not show the version's set_at as if a secret were set.
func TestListMiniAppSecretStatuses(t *testing.T) {
	f := &fakeServer{statuses: &identityv1.ListMiniAppSecretStatusesResponse{Outcome: wAccepted,
		Statuses: []*identityv1.MiniAppSecretStatus{statusFake(appIDFake, true), statusFake("3043188591857102858", false)}}}
	c, _ := start(t, f)
	res, err := c.ListMiniAppSecretStatuses(ctxWithTenant(), secret.Secret(tokenFake))
	if err != nil || res.Outcome != OutcomeAccepted || len(res.Statuses) != 2 {
		t.Fatalf("%+v %v", res, err)
	}
	if s := res.Statuses[0]; !s.SecretSet || s.SetBy != "VH-00001" || s.SetAt.Unix() != 1_800_000_000 {
		t.Errorf("set entry = %+v", s)
	}
	if s := res.Statuses[1]; s.SecretSet || s.SetBy != "" || !s.SetAt.IsZero() {
		t.Errorf("a version without a secret was dated: %+v", s)
	}
	if len(f.sawTenant) != 1 || len(f.sawTenant[0]) != 1 || f.sawTenant[0][0] != "01JA"+strings.Repeat("A", 22) {
		t.Errorf("x-tenant-id = %v", f.sawTenant)
	}

	for name, resp := range map[string]*identityv1.ListMiniAppSecretStatusesResponse{
		"UNSPECIFIED":           {},
		"REFUSED outside":       {Outcome: wRefused},
		"entry without version": {Outcome: wAccepted, Statuses: []*identityv1.MiniAppSecretStatus{{SecretSet: true}}},
		"set without set_by": {Outcome: wAccepted, Statuses: []*identityv1.MiniAppSecretStatus{{SecretSet: true,
			LiveVersion: &identityv1.MiniAppSecretVersion{AppId: appIDFake}}}},
	} {
		f.statuses = resp
		if _, err := c.ListMiniAppSecretStatuses(ctxWithTenant(), secret.Secret(tokenFake)); !errors.Is(err, ErrContract) {
			t.Errorf("%s: %v, want ErrContract (fail closed)", name, err)
		}
	}
	f.statuses = &identityv1.ListMiniAppSecretStatusesResponse{Outcome: wDenied}
	if res, err := c.ListMiniAppSecretStatuses(ctxWithTenant(), secret.Secret(tokenFake)); err != nil || res.Outcome != OutcomePermissionDenied {
		t.Errorf("PERMISSION_DENIED: %v %v", res.Outcome, err)
	}
	f.statuses = &identityv1.ListMiniAppSecretStatusesResponse{Outcome: wNotLive}
	if res, err := c.ListMiniAppSecretStatuses(ctxWithTenant(), secret.Secret(tokenFake)); err != nil || res.Outcome != OutcomeSessionNotLive {
		t.Errorf("SESSION_NOT_LIVE: %v %v", res.Outcome, err)
	}
}

func TestListMiniAppSecretStatusesFailures(t *testing.T) {
	f := &fakeServer{}
	c, _ := start(t, f)
	if _, err := c.ListMiniAppSecretStatuses(context.Background(), secret.Secret(tokenFake)); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("without a commune: %v", err)
	}
	if _, err := c.ListMiniAppSecretStatuses(ctxWithTenant(), nil); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("without a token: %v", err)
	}
	if f.calls != 0 {
		t.Fatalf("%d refused calls reached the server", f.calls)
	}
	// An identity older than the RPC: the embedded Unimplemented server answers UNIMPLEMENTED.
	old, _ := start(t, &fakeServer{err: status.Error(codes.Unimplemented, "x")})
	if _, err := old.ListMiniAppSecretStatuses(ctxWithTenant(), secret.Secret(tokenFake)); !errors.Is(err, ErrNotSupported) {
		t.Errorf("UNIMPLEMENTED: %v, want ErrNotSupported", err)
	}
	down, logBuf := start(t, &fakeServer{err: status.Error(codes.Unavailable, "x")})
	_, err := down.ListMiniAppSecretStatuses(ctxWithTenant(), secret.Secret(tokenFake))
	if err == nil || errors.Is(err, ErrNotSupported) || errors.Is(err, ErrContract) {
		t.Errorf("UNAVAILABLE: %v", err)
	}
	if strings.Contains(logBuf.String(), tokenFake) {
		t.Fatal("the log carries the token")
	}
}
