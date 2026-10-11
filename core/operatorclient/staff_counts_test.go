package operatorclient

// What these tests defend for ListActiveStaffCounts: it travels WITHOUT a commune (exempt by full
// name — a commune in ctx is not sent), the token and client IP travel verbatim, and every answer
// that is not a clean ACCEPTED list is an error or a non-accepted outcome — never "zero staff".

import (
	"context"
	"errors"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/grpcx"
	"github.com/vihat/vigov/core/secret"
)

func (f *fakeServer) ListActiveStaffCounts(ctx context.Context, r *identityv1.ListActiveStaffCountsRequest) (*identityv1.ListActiveStaffCountsResponse, error) {
	f.record(ctx)
	f.sawStaffCounts = r
	return f.staffCounts, f.err
}

func TestListActiveStaffCountsAccepted(t *testing.T) {
	a, b := "01JA"+strings.Repeat("A", 22), "01JA"+strings.Repeat("B", 22)
	f := &fakeServer{staffCounts: &identityv1.ListActiveStaffCountsResponse{Outcome: wAccepted,
		Counts: []*identityv1.CommuneActiveStaffCount{{TenantId: a, ActiveStaffCount: 12}, {TenantId: b, ActiveStaffCount: 3}}}}
	c, _ := start(t, f)
	// A commune in ctx must NOT travel: the RPC is exempt, and the exemption depends on the list.
	res, err := c.ListActiveStaffCounts(ctxWithTenant(), secret.Secret(tokenFake), "203.0.113.7")
	if err != nil || res.Outcome != OutcomeAccepted || res.Counts[a] != 12 || res.Counts[b] != 3 || len(res.Counts) != 2 {
		t.Fatalf("%+v %v", res, err)
	}
	if f.sawStaffCounts.GetSessionToken() != tokenFake || f.sawStaffCounts.GetClientIp() != "203.0.113.7" {
		t.Error("token or client IP did not travel verbatim")
	}
	if len(f.sawTenant) != 1 || len(f.sawTenant[0]) != 0 {
		t.Errorf("%s = %v — this call must carry no commune", grpcx.MetadataTenantKey, f.sawTenant)
	}

	f.staffCounts = &identityv1.ListActiveStaffCountsResponse{Outcome: wAccepted}
	if res, err := c.ListActiveStaffCounts(context.Background(), secret.Secret(tokenFake), ""); err != nil || len(res.Counts) != 0 {
		t.Errorf("empty list is a valid answer: %+v %v", res, err)
	}
}

func TestListActiveStaffCountsOutcomesAndContract(t *testing.T) {
	f := &fakeServer{}
	c, _ := start(t, f)
	for _, o := range []struct {
		wire identityv1.OperatorAuthOutcome
		want Outcome
	}{{wNotLive, OutcomeSessionNotLive}, {wDenied, OutcomePermissionDenied}} {
		f.staffCounts = &identityv1.ListActiveStaffCountsResponse{Outcome: o.wire}
		if res, err := c.ListActiveStaffCounts(context.Background(), secret.Secret(tokenFake), ""); err != nil || res.Outcome != o.want {
			t.Errorf("%v: %v %v", o.wire, res.Outcome, err)
		}
	}
	a := "01JA" + strings.Repeat("A", 22)
	for name, resp := range map[string]*identityv1.ListActiveStaffCountsResponse{
		"UNSPECIFIED":     {},
		"REFUSED outside": {Outcome: wRefused},
		"empty tenant":    {Outcome: wAccepted, Counts: []*identityv1.CommuneActiveStaffCount{{ActiveStaffCount: 2}}},
		"zero count":      {Outcome: wAccepted, Counts: []*identityv1.CommuneActiveStaffCount{{TenantId: a}}},
		"duplicate": {Outcome: wAccepted, Counts: []*identityv1.CommuneActiveStaffCount{
			{TenantId: a, ActiveStaffCount: 1}, {TenantId: a, ActiveStaffCount: 2}}},
	} {
		f.staffCounts = resp
		if _, err := c.ListActiveStaffCounts(context.Background(), secret.Secret(tokenFake), ""); !errors.Is(err, ErrContract) {
			t.Errorf("%s: %v, want ErrContract (fail closed)", name, err)
		}
	}
}

func TestListActiveStaffCountsFailures(t *testing.T) {
	f := &fakeServer{}
	c, _ := start(t, f)
	if _, err := c.ListActiveStaffCounts(context.Background(), nil, ""); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("without a token: %v", err)
	}
	if f.calls != 0 {
		t.Fatalf("%d refused calls reached the server", f.calls)
	}
	old, _ := start(t, &fakeServer{err: status.Error(codes.Unimplemented, "x")})
	if _, err := old.ListActiveStaffCounts(context.Background(), secret.Secret(tokenFake), ""); !errors.Is(err, ErrNotSupported) {
		t.Errorf("UNIMPLEMENTED: %v, want ErrNotSupported", err)
	}
	down, logBuf := start(t, &fakeServer{err: status.Error(codes.Unavailable, "x")})
	_, err := down.ListActiveStaffCounts(context.Background(), secret.Secret(tokenFake), "")
	if err == nil || errors.Is(err, ErrNotSupported) || errors.Is(err, ErrContract) {
		t.Errorf("UNAVAILABLE: %v", err)
	}
	if strings.Contains(logBuf.String(), tokenFake) {
		t.Fatal("the log carries the token")
	}
}
