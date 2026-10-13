package grpc

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// Fake material. The temporary password is a marker that must never appear in a log line.
const (
	ownerTemp  = "OWNER-MARKER-TEMP-PASSWORD-not-real"
	ownerIP    = "203.0.113.9"
	ownerEmail = "owner-marker@example.test"
)

var (
	ownerPrincipal = app.OperatorPrincipal{ID: "01JOWNER000000000000000000", Code: "VH-00001", IsOwner: true,
		Permissions: domain.OperatorPermissions()}
	// Holds every ops.* key and is still not the owner: the gate is the flag, never the keys.
	nonOwnerPrincipal = app.OperatorPrincipal{ID: "01JOTHER000000000000000000", Code: "VH-00002",
		Permissions: domain.OperatorPermissions()}
)

type ownerOpsFake struct {
	calls     int
	err       error
	gotOwner  app.MiniAppOperator
	gotTarget string
	gotKeys   []domain.OperatorPermission
	gotIn     app.NewOperatorByOwner
	gotReason string
}

func (f *ownerOpsFake) view() app.OperatorAccountView {
	locked := time.Date(2026, 10, 13, 20, 0, 0, 0, time.UTC)
	return app.OperatorAccountView{Account: domain.OperatorAccount{Code: "VH-00003", Email: "x@example.test",
		DisplayName: "X", MustChangePassword: true, LockedUntil: &locked,
		CreatedAt: time.Date(2026, 10, 13, 8, 0, 0, 0, time.UTC)},
		Permissions: []domain.OperatorPermission{domain.OperatorPermissionQRIssue}}
}

func (f *ownerOpsFake) ListAccounts(context.Context) ([]app.OperatorAccountView, error) {
	f.calls++
	return []app.OperatorAccountView{f.view()}, f.err
}
func (f *ownerOpsFake) CreateAccount(_ context.Context, o app.MiniAppOperator, in app.NewOperatorByOwner) (app.OperatorAccountView, error) {
	f.calls++
	f.gotOwner, f.gotIn = o, in
	return f.view(), f.err
}
func (f *ownerOpsFake) SetPermissions(_ context.Context, o app.MiniAppOperator, target string, keys []domain.OperatorPermission,
	reason string) (app.OperatorAccountView, error) {
	f.calls++
	f.gotOwner, f.gotTarget, f.gotKeys, f.gotReason = o, target, keys, reason
	return f.view(), f.err
}
func (f *ownerOpsFake) ResetPassword(_ context.Context, o app.MiniAppOperator, target, _, reason string) (app.OperatorAccountView, error) {
	f.calls++
	f.gotOwner, f.gotTarget, f.gotReason = o, target, reason
	return f.view(), f.err
}

type ownerCommunesFake struct {
	calls         int
	err           error
	gotOwner      app.MiniAppOperator
	gotOperatorID string
	gotUser       string
}

func (f *ownerCommunesFake) account() domain.StaffSignInAccount {
	return domain.StaffSignInAccount{ID: "01JSTAFF000000000000000000", Code: "CB-00001", Email: "a@xa.example.test",
		FullName: "Nguyen Van A", RoleCode: domain.CommuneAdminRoleCode, RoleName: "Quản trị hệ thống",
		MustChangePassword: true}
}

func (f *ownerCommunesFake) List(_ context.Context, o app.MiniAppOperator) ([]domain.StaffSignInAccount, error) {
	f.calls++
	f.gotOwner = o
	return []domain.StaffSignInAccount{f.account()}, f.err
}
func (f *ownerCommunesFake) CreateAdmin(_ context.Context, o app.MiniAppOperator, _, _, _ string) (domain.StaffSignInAccount, error) {
	f.calls++
	f.gotOwner = o
	return f.account(), f.err
}
func (f *ownerCommunesFake) ResetPassword(_ context.Context, o app.MiniAppOperator, operatorID, userID, _, _ string) (domain.StaffSignInAccount, error) {
	f.calls++
	f.gotOwner, f.gotOperatorID, f.gotUser = o, operatorID, userID
	return f.account(), f.err
}

type ownerRig struct {
	s    *OperatorServer
	auth *operatorAuthFake
	ops  *ownerOpsFake
	comm *ownerCommunesFake
	logs *bytes.Buffer
}

func newOwnerRig(p app.OperatorPrincipal, authErr error) *ownerRig {
	r := &ownerRig{auth: &operatorAuthFake{principal: p, err: authErr}, ops: &ownerOpsFake{},
		comm: &ownerCommunesFake{}, logs: &bytes.Buffer{}}
	log := slog.New(slog.NewJSONHandler(r.logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	r.s = NewOperatorServer(r.auth, log).WithOwnerAdmin(r.ops, r.comm)
	return r
}

func (r *ownerRig) useCaseCalls() int { return r.ops.calls + r.comm.calls }

// ownerCall is one owner-only RPC with a well-formed request; it returns the outcome.
type ownerCall func(s *OperatorServer) (identityv1.OperatorAuthOutcome, error)

func ownerCalls(temp string) map[string]ownerCall {
	return map[string]ownerCall{
		"ListOperatorAccounts": func(s *OperatorServer) (identityv1.OperatorAuthOutcome, error) {
			r, err := s.ListOperatorAccounts(context.Background(), &identityv1.ListOperatorAccountsRequest{SessionToken: "op1.t"})
			return r.GetOutcome(), err
		},
		"CreateOperatorAccount": func(s *OperatorServer) (identityv1.OperatorAuthOutcome, error) {
			r, err := s.CreateOperatorAccount(context.Background(), &identityv1.CreateOperatorAccountRequest{
				SessionToken: "op1.t", Email: ownerEmail, DisplayName: "New Operator", TemporaryPassword: temp,
				PermissionKeys: []string{"ops.qr.issue", "ops.qr.issue"}, ClientIp: ownerIP})
			return r.GetOutcome(), err
		},
		"SetOperatorPermissions": func(s *OperatorServer) (identityv1.OperatorAuthOutcome, error) {
			r, err := s.SetOperatorPermissions(context.Background(), &identityv1.SetOperatorPermissionsRequest{
				SessionToken: "op1.t", TargetCode: "VH-00003", PermissionKeys: []string{"ops.qr.issue"},
				Reason: "ticket 12", ClientIp: ownerIP})
			return r.GetOutcome(), err
		},
		"ResetOperatorPassword": func(s *OperatorServer) (identityv1.OperatorAuthOutcome, error) {
			r, err := s.ResetOperatorPassword(context.Background(), &identityv1.ResetOperatorPasswordRequest{
				SessionToken: "op1.t", TargetCode: "VH-00003", TemporaryPassword: temp, Reason: "forgot", ClientIp: ownerIP})
			return r.GetOutcome(), err
		},
		"ListCommuneStaffAccounts": func(s *OperatorServer) (identityv1.OperatorAuthOutcome, error) {
			r, err := s.ListCommuneStaffAccounts(maCtx(), &identityv1.ListCommuneStaffAccountsRequest{
				SessionToken: "op1.t", ClientIp: ownerIP})
			return r.GetOutcome(), err
		},
		"CreateCommuneAdminAccount": func(s *OperatorServer) (identityv1.OperatorAuthOutcome, error) {
			r, err := s.CreateCommuneAdminAccount(maCtx(), &identityv1.CreateCommuneAdminAccountRequest{
				SessionToken: "op1.t", Email: "Admin@Xa.Example.Test", FullName: "Quan Tri", TemporaryPassword: temp,
				ClientIp: ownerIP})
			return r.GetOutcome(), err
		},
		"ResetStaffPassword": func(s *OperatorServer) (identityv1.OperatorAuthOutcome, error) {
			r, err := s.ResetStaffPassword(maCtx(), &identityv1.ResetStaffPasswordRequest{
				SessionToken: "op1.t", UserId: "01JSTAFF000000000000000000", TemporaryPassword: temp,
				Reason: "forgot", ClientIp: ownerIP})
			return r.GetOutcome(), err
		},
	}
}

// The gate, for all seven: a non-owner (holding every ops.* key) is PERMISSION_DENIED and a dead
// session is SESSION_NOT_LIVE — both with NOTHING read or written by the use case.
func TestOwnerRPCsGate(t *testing.T) {
	for name, call := range ownerCalls(ownerTemp) {
		t.Run(name+"/not owner", func(t *testing.T) {
			r := newOwnerRig(nonOwnerPrincipal, nil)
			out, err := call(r.s)
			if err != nil || out != outDenied {
				t.Fatalf("outcome %v err %v, want PERMISSION_DENIED", out, err)
			}
			if r.useCaseCalls() != 0 {
				t.Fatalf("use case called %d times for a non-owner", r.useCaseCalls())
			}
		})
		t.Run(name+"/dead session", func(t *testing.T) {
			r := newOwnerRig(app.OperatorPrincipal{}, app.ErrOperatorUnauthenticated)
			out, err := call(r.s)
			if err != nil || out != outNotLive {
				t.Fatalf("outcome %v err %v, want SESSION_NOT_LIVE", out, err)
			}
			if r.useCaseCalls() != 0 {
				t.Fatalf("use case called for a dead session")
			}
		})
		t.Run(name+"/owner", func(t *testing.T) {
			r := newOwnerRig(ownerPrincipal, nil)
			out, err := call(r.s)
			if err != nil || out != outAccepted {
				t.Fatalf("outcome %v err %v, want ACCEPTED", out, err)
			}
			if r.useCaseCalls() != 1 {
				t.Fatalf("use case calls = %d, want 1", r.useCaseCalls())
			}
			if strings.Contains(r.logs.String(), ownerTemp) || strings.Contains(r.logs.String(), ownerEmail) {
				t.Fatalf("a credential or an email reached the log: %s", r.logs.String())
			}
		})
		t.Run(name+"/unwired", func(t *testing.T) {
			s := NewOperatorServer(&operatorAuthFake{principal: ownerPrincipal}, slog.Default())
			if _, err := call(s); status.Code(err) != codes.Unimplemented {
				t.Fatalf("err %v, want Unimplemented", err)
			}
		})
	}
}

// The password policy answers NEW_PASSWORD_REJECTED for the owner (before the use case), and never
// for a non-owner, who is denied first.
func TestOwnerRPCsPasswordPolicy(t *testing.T) {
	for _, name := range []string{"CreateOperatorAccount", "ResetOperatorPassword", "CreateCommuneAdminAccount", "ResetStaffPassword"} {
		call := ownerCalls("short")[name]
		r := newOwnerRig(ownerPrincipal, nil)
		out, err := call(r.s)
		if err != nil || out != outRejected {
			t.Fatalf("%s: outcome %v err %v, want NEW_PASSWORD_REJECTED", name, out, err)
		}
		if r.useCaseCalls() != 0 {
			t.Fatalf("%s: use case reached with a refused password", name)
		}
		r = newOwnerRig(nonOwnerPrincipal, nil)
		if out, _ := call(r.s); out != outDenied {
			t.Fatalf("%s: non-owner got %v before the owner check", name, out)
		}
	}
	r := newOwnerRig(ownerPrincipal, nil)
	resp, _ := r.s.ResetStaffPassword(maCtx(), &identityv1.ResetStaffPasswordRequest{
		SessionToken: "op1.t", UserId: "u", TemporaryPassword: "short", Reason: "r"})
	if resp.GetNewPasswordRefusal().GetProblem() != identityv1.NewPasswordProblem_NEW_PASSWORD_PROBLEM_TOO_SHORT ||
		resp.GetNewPasswordRefusal().GetMinLength() != domain.DaiMatKhauToiThieu {
		t.Fatalf("refusal = %v", resp.GetNewPasswordRefusal())
	}
}

func TestOwnerRPCsInvalidArgumentBeforeAnyLookup(t *testing.T) {
	r := newOwnerRig(ownerPrincipal, nil)
	bg := context.Background()
	cases := map[string]func() error{
		"list empty token": func() error {
			_, err := r.s.ListOperatorAccounts(bg, &identityv1.ListOperatorAccountsRequest{})
			return err
		},
		"create bad email": func() error {
			_, err := r.s.CreateOperatorAccount(bg, &identityv1.CreateOperatorAccountRequest{SessionToken: "t",
				Email: "not-an-email", DisplayName: "X", TemporaryPassword: ownerTemp})
			return err
		},
		"create unknown key": func() error {
			_, err := r.s.CreateOperatorAccount(bg, &identityv1.CreateOperatorAccountRequest{SessionToken: "t",
				Email: ownerEmail, DisplayName: "X", TemporaryPassword: ownerTemp, PermissionKeys: []string{"admin.user"}})
			return err
		},
		"set empty target": func() error {
			_, err := r.s.SetOperatorPermissions(bg, &identityv1.SetOperatorPermissionsRequest{SessionToken: "t", Reason: "r"})
			return err
		},
		"set no reason": func() error {
			_, err := r.s.SetOperatorPermissions(bg, &identityv1.SetOperatorPermissionsRequest{SessionToken: "t",
				TargetCode: "VH-00003", Reason: "   "})
			return err
		},
		"reset no reason": func() error {
			_, err := r.s.ResetOperatorPassword(bg, &identityv1.ResetOperatorPasswordRequest{SessionToken: "t",
				TargetCode: "VH-00003", TemporaryPassword: ownerTemp})
			return err
		},
		"list staff no commune": func() error {
			_, err := r.s.ListCommuneStaffAccounts(bg, &identityv1.ListCommuneStaffAccountsRequest{SessionToken: "t"})
			return err
		},
		"create admin no commune": func() error {
			_, err := r.s.CreateCommuneAdminAccount(bg, &identityv1.CreateCommuneAdminAccountRequest{SessionToken: "t",
				Email: "a@xa.example.test", FullName: "A", TemporaryPassword: ownerTemp})
			return err
		},
		"create admin literal admin": func() error {
			_, err := r.s.CreateCommuneAdminAccount(maCtx(), &identityv1.CreateCommuneAdminAccountRequest{SessionToken: "t",
				Email: "admin", FullName: "A", TemporaryPassword: ownerTemp})
			return err
		},
		"create admin no name": func() error {
			_, err := r.s.CreateCommuneAdminAccount(maCtx(), &identityv1.CreateCommuneAdminAccountRequest{SessionToken: "t",
				Email: "a@xa.example.test", FullName: "  ", TemporaryPassword: ownerTemp})
			return err
		},
		"reset staff no user": func() error {
			_, err := r.s.ResetStaffPassword(maCtx(), &identityv1.ResetStaffPasswordRequest{SessionToken: "t", Reason: "r",
				TemporaryPassword: ownerTemp})
			return err
		},
		"reset staff empty token": func() error {
			_, err := r.s.ResetStaffPassword(maCtx(), &identityv1.ResetStaffPasswordRequest{UserId: "u", Reason: "r",
				TemporaryPassword: ownerTemp})
			return err
		},
	}
	for name, fn := range cases {
		if err := fn(); status.Code(err) != codes.InvalidArgument {
			t.Errorf("%s: err %v, want InvalidArgument", name, err)
		}
	}
	if r.auth.calls != 0 || r.useCaseCalls() != 0 {
		t.Fatalf("a malformed request reached a lookup: auth %d, use cases %d", r.auth.calls, r.useCaseCalls())
	}
}

func TestOwnerRPCsMapRefusals(t *testing.T) {
	type tc struct {
		name string
		rpc  string
		err  error
		code codes.Code
	}
	for _, c := range []tc{
		{"create email taken", "CreateOperatorAccount", app.ErrOperatorEmailTaken, codes.AlreadyExists},
		{"set unknown", "SetOperatorPermissions", app.ErrOperatorNotFound, codes.NotFound},
		{"set owner target", "SetOperatorPermissions", app.ErrOwnerTargetIsOwner, codes.FailedPrecondition},
		{"reset unknown", "ResetOperatorPassword", app.ErrOperatorNotFound, codes.NotFound},
		{"reset self", "ResetOperatorPassword", app.ErrOwnerTargetIsSelf, codes.FailedPrecondition},
		{"admin email taken", "CreateCommuneAdminAccount", app.ErrCommuneAdminEmailTaken, codes.AlreadyExists},
		{"admin commune not live", "CreateCommuneAdminAccount", app.ErrCommuneNotLive, codes.FailedPrecondition},
		{"admin role deleted", "CreateCommuneAdminAccount", idstore.ErrVaiTroQuanTriDaXoa, codes.FailedPrecondition},
		{"staff unknown", "ResetStaffPassword", idstore.ErrCanBoKhongTonTai, codes.NotFound},
		{"staff directory only", "ResetStaffPassword", app.ErrChuaCoTaiKhoan, codes.NotFound},
		{"staff locked", "ResetStaffPassword", app.ErrTaiKhoanDangKhoa, codes.FailedPrecondition},
		{"list staff store failure", "ListCommuneStaffAccounts", errors.New("disk"), codes.Internal},
		{"list ops store failure", "ListOperatorAccounts", errors.New("disk"), codes.Internal},
	} {
		r := newOwnerRig(ownerPrincipal, nil)
		r.ops.err, r.comm.err = c.err, c.err
		if _, err := ownerCalls(ownerTemp)[c.rpc](r.s); status.Code(err) != c.code {
			t.Errorf("%s: err %v, want %v", c.name, err, c.code)
		}
	}
}

// The actor handed to the use case is THE resolved principal's code and the request's client_ip —
// and, for the staff reset, the principal's internal id, never anything the request names.
func TestOwnerRPCsPassResolvedActor(t *testing.T) {
	r := newOwnerRig(ownerPrincipal, nil)
	calls := ownerCalls(ownerTemp)
	if _, err := calls["SetOperatorPermissions"](r.s); err != nil {
		t.Fatal(err)
	}
	if r.ops.gotOwner != (app.MiniAppOperator{Code: "VH-00001", IP: ownerIP}) || r.ops.gotTarget != "VH-00003" ||
		len(r.ops.gotKeys) != 1 || r.ops.gotReason != "ticket 12" {
		t.Fatalf("set permissions got %+v %q %v %q", r.ops.gotOwner, r.ops.gotTarget, r.ops.gotKeys, r.ops.gotReason)
	}
	if _, err := calls["CreateOperatorAccount"](r.s); err != nil {
		t.Fatal(err)
	}
	if len(r.ops.gotIn.Permissions) != 1 || r.ops.gotIn.Email != ownerEmail {
		t.Fatalf("create got %d keys (duplicates must fold)", len(r.ops.gotIn.Permissions))
	}
	if _, err := calls["ResetStaffPassword"](r.s); err != nil {
		t.Fatal(err)
	}
	if r.comm.gotOperatorID != ownerPrincipal.ID || r.comm.gotOwner.Code != "VH-00001" || r.comm.gotOwner.IP != ownerIP {
		t.Fatalf("staff reset got %+v / %q", r.comm.gotOwner, r.comm.gotOperatorID)
	}
}

func TestOwnerRPCsResponseShapes(t *testing.T) {
	r := newOwnerRig(ownerPrincipal, nil)
	list, err := r.s.ListOperatorAccounts(context.Background(), &identityv1.ListOperatorAccountsRequest{SessionToken: "t"})
	if err != nil || len(list.GetAccounts()) != 1 {
		t.Fatalf("list: %v %v", list, err)
	}
	a := list.GetAccounts()[0]
	if a.GetCode() != "VH-00003" || !a.GetMustChangePassword() || a.GetLockedUntil() == nil ||
		len(a.GetPermissionKeys()) != 1 || a.GetCreatedAt() == nil {
		t.Fatalf("operator account = %v", a)
	}
	staff, err := r.s.ListCommuneStaffAccounts(maCtx(), &identityv1.ListCommuneStaffAccountsRequest{SessionToken: "t"})
	if err != nil || len(staff.GetAccounts()) != 1 {
		t.Fatalf("staff list: %v %v", staff, err)
	}
	sa := staff.GetAccounts()[0]
	if !sa.GetIsAdmin() || sa.GetStaffCode() != "CB-00001" || sa.GetSignInLockedUntil() != nil {
		t.Fatalf("staff account = %v", sa)
	}
}

func TestResolveOperatorSessionCarriesIsOwner(t *testing.T) {
	for _, p := range []app.OperatorPrincipal{ownerPrincipal, nonOwnerPrincipal} {
		s := NewOperatorServer(&operatorAuthFake{principal: p}, slog.Default())
		resp, err := s.ResolveOperatorSession(context.Background(), &identityv1.ResolveOperatorSessionRequest{SessionToken: "t"})
		if err != nil || resp.GetPrincipal().GetIsOwner() != p.IsOwner {
			t.Fatalf("principal %s: is_owner = %v, want %v (err %v)", p.Code, resp.GetPrincipal().GetIsOwner(), p.IsOwner, err)
		}
	}
}
