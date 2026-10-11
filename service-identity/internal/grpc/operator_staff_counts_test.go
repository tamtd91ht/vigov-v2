package grpc

// ListActiveStaffCounts (operator.proto, owner's decision 11/10/2026): the status table and outcomes
// on fakes. The audit entry itself is internal/app/active_staff_counts_test.go; the count query is
// internal/store/crosstenant/active_staff_counts_pg_test.go.
//
// Rule 5 #7 in this service's shape: no HTTP edge, so 401/403 are outcomes — SESSION_NOT_LIVE,
// PERMISSION_DENIED — and ACCEPTED. There is no commune dimension to test: the RPC answers every
// commune and reads none from the metadata.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

type staffCountsFake struct {
	rows  []app.CommuneActiveStaffCount
	err   error
	calls int
	gotOp app.MiniAppOperator
}

func (f *staffCountsFake) ReadAsOperator(_ context.Context, op app.MiniAppOperator) ([]app.CommuneActiveStaffCount, error) {
	f.calls++
	f.gotOp = op
	if f.err != nil {
		return nil, f.err
	}
	return f.rows, nil
}

type scRig struct {
	s    *OperatorServer
	uc   *operatorAuthFake
	c    *staffCountsFake
	logs *bytes.Buffer
}

func newSCRig(ucErr, cErr error, p app.OperatorPrincipal) scRig {
	logs := &bytes.Buffer{}
	uc := &operatorAuthFake{err: ucErr, principal: p}
	c := &staffCountsFake{err: cErr, rows: []app.CommuneActiveStaffCount{
		{TenantID: string(maCommune), Count: 12}, {TenantID: string(maCommuneOther), Count: 1},
	}}
	s := NewOperatorServer(uc, slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))).
		WithActiveStaffCounts(c)
	return scRig{s: s, uc: uc, c: c, logs: logs}
}

func scReq() *identityv1.ListActiveStaffCountsRequest {
	return &identityv1.ListActiveStaffCountsRequest{SessionToken: opMarkToken, ClientIp: maClientIP}
}

// ACCEPTED: any granted ops.* key reads; the use case gets the RESOLVED VH- code and client_ip; the
// counts pass through in order. No commune in ctx is needed.
func TestListActiveStaffCountsAccepted(t *testing.T) {
	readOnly := managerPrincipal()
	readOnly.Permissions = []domain.OperatorPermission{domain.OperatorPermissionQRIssue}
	for name, p := range map[string]app.OperatorPrincipal{"manager": managerPrincipal(), "any other ops key": readOnly} {
		t.Run(name, func(t *testing.T) {
			r := newSCRig(nil, nil, p)
			resp, err := r.s.ListActiveStaffCounts(context.Background(), scReq())
			if err != nil || resp.GetOutcome() != outAccepted {
				t.Fatalf("resp %v err %v, want ACCEPTED", resp.GetOutcome(), err)
			}
			if r.uc.gotToken != opMarkToken || r.c.gotOp != (app.MiniAppOperator{Code: "VH-00001", IP: maClientIP}) {
				t.Fatalf("token %q / operator %+v — want the request's token, the resolved code and client_ip", r.uc.gotToken, r.c.gotOp)
			}
			c := resp.GetCounts()
			if len(c) != 2 || c[0].GetTenantId() != string(maCommune) || c[0].GetActiveStaffCount() != 12 ||
				c[1].GetTenantId() != string(maCommuneOther) || c[1].GetActiveStaffCount() != 1 {
				t.Fatalf("counts = %v", c)
			}
		})
	}
	// An empty answer is ACCEPTED with no entries, never an error.
	r := newSCRig(nil, nil, managerPrincipal())
	r.c.rows = nil
	resp, err := r.s.ListActiveStaffCounts(context.Background(), scReq())
	if err != nil || resp.GetOutcome() != outAccepted || len(resp.GetCounts()) != 0 {
		t.Fatalf("empty: %v %v", resp, err)
	}
}

// SESSION_NOT_LIVE and PERMISSION_DENIED: answered, nothing read, nothing audited, no counts.
func TestListActiveStaffCountsSessionAndPermission(t *testing.T) {
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
			r := newSCRig(c.ucErr, nil, c.p)
			resp, err := r.s.ListActiveStaffCounts(context.Background(), scReq())
			if err != nil || resp.GetOutcome() != c.want || len(resp.GetCounts()) != 0 {
				t.Fatalf("%v %v, want %v and no counts", resp, err, c.want)
			}
			if r.c.calls != 0 {
				t.Fatal("the use case ran")
			}
		})
	}
}

// The trail entry cannot be written → UNAVAILABLE, no counts, nothing of the request in the message
// or the log.
func TestListActiveStaffCountsUnauditedIsUnavailable(t *testing.T) {
	r := newSCRig(nil, fmt.Errorf("%w: %w", app.ErrActiveStaffCountsUnaudited, errors.New("insert refused")), managerPrincipal())
	resp, err := r.s.ListActiveStaffCounts(context.Background(), scReq())
	if status.Code(err) != codes.Unavailable || resp != nil {
		t.Fatalf("resp %v err %v, want Unavailable and no response", resp, err)
	}
	for _, v := range []string{opMarkToken, maClientIP} {
		if strings.Contains(err.Error(), v) || strings.Contains(r.logs.String(), opMarkToken) {
			t.Fatalf("%q reached the status or the log", v)
		}
	}
}

// Faults: empty token before any lookup; other use-case errors Internal; realm not configured
// FAILED_PRECONDITION; a principal with no code Internal and nothing read; unwired Unimplemented.
func TestListActiveStaffCountsFaults(t *testing.T) {
	r := newSCRig(nil, nil, managerPrincipal())
	if _, err := r.s.ListActiveStaffCounts(context.Background(), &identityv1.ListActiveStaffCountsRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("empty token: %v, want InvalidArgument", status.Code(err))
	}
	if r.uc.calls != 0 || r.c.calls != 0 {
		t.Fatalf("session resolved %d times, use case %d — want neither", r.uc.calls, r.c.calls)
	}

	r = newSCRig(nil, errors.New("store exploded"), managerPrincipal())
	_, err := r.s.ListActiveStaffCounts(context.Background(), scReq())
	if status.Code(err) != codes.Internal || strings.Contains(err.Error(), "exploded") || strings.Contains(r.logs.String(), opMarkToken) {
		t.Fatalf("store error: %v / log %s", err, r.logs.String())
	}

	r = newSCRig(app.ErrOperatorRealmNotConfigured, nil, app.OperatorPrincipal{})
	if _, err := r.s.ListActiveStaffCounts(context.Background(), scReq()); status.Code(err) != codes.FailedPrecondition {
		t.Errorf("realm not configured: %v, want FailedPrecondition", status.Code(err))
	}

	p := managerPrincipal()
	p.Code = ""
	r = newSCRig(nil, nil, p)
	if _, err := r.s.ListActiveStaffCounts(context.Background(), scReq()); status.Code(err) != codes.Internal || r.c.calls != 0 {
		t.Errorf("principal without a code: %v, calls %d — want Internal and nothing read", status.Code(err), r.c.calls)
	}

	s := NewOperatorServer(&operatorAuthFake{principal: managerPrincipal()}, nil)
	if _, err := s.ListActiveStaffCounts(context.Background(), scReq()); status.Code(err) != codes.Unimplemented {
		t.Errorf("unwired: %v, want Unimplemented", status.Code(err))
	}
}
