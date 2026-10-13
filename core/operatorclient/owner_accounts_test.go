package operatorclient

// What these tests defend for the seven owner-only RPCs: the four operator-account RPCs travel
// WITHOUT a commune and the three staff-account RPCs WITH the context's (refused locally without one),
// passwords and tokens travel verbatim and never reach a log, every status maps to the typed error the
// HTTP edge turns into its status, and an answer outside the contract is ErrContract — never served.

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
const tempPasswordFake = "  temp-password-FAKE-not-real  " // spaces: must travel verbatim

type ownerAnswers struct {
	listOps    *identityv1.ListOperatorAccountsResponse
	createOp   *identityv1.CreateOperatorAccountResponse
	setPerms   *identityv1.SetOperatorPermissionsResponse
	resetOp    *identityv1.ResetOperatorPasswordResponse
	listStaff  *identityv1.ListCommuneStaffAccountsResponse
	createAdm  *identityv1.CreateCommuneAdminAccountResponse
	resetStaff *identityv1.ResetStaffPasswordResponse

	sawCreateOp   *identityv1.CreateOperatorAccountRequest
	sawSetPerms   *identityv1.SetOperatorPermissionsRequest
	sawResetOp    *identityv1.ResetOperatorPasswordRequest
	sawListStaff  *identityv1.ListCommuneStaffAccountsRequest
	sawCreateAdm  *identityv1.CreateCommuneAdminAccountRequest
	sawResetStaff *identityv1.ResetStaffPasswordRequest
}

func (f *fakeServer) ListOperatorAccounts(ctx context.Context, _ *identityv1.ListOperatorAccountsRequest) (*identityv1.ListOperatorAccountsResponse, error) {
	f.record(ctx)
	return f.owner.listOps, f.err
}

func (f *fakeServer) CreateOperatorAccount(ctx context.Context, r *identityv1.CreateOperatorAccountRequest) (*identityv1.CreateOperatorAccountResponse, error) {
	f.record(ctx)
	f.owner.sawCreateOp = r
	return f.owner.createOp, f.err
}

func (f *fakeServer) SetOperatorPermissions(ctx context.Context, r *identityv1.SetOperatorPermissionsRequest) (*identityv1.SetOperatorPermissionsResponse, error) {
	f.record(ctx)
	f.owner.sawSetPerms = r
	return f.owner.setPerms, f.err
}

func (f *fakeServer) ResetOperatorPassword(ctx context.Context, r *identityv1.ResetOperatorPasswordRequest) (*identityv1.ResetOperatorPasswordResponse, error) {
	f.record(ctx)
	f.owner.sawResetOp = r
	return f.owner.resetOp, f.err
}

func (f *fakeServer) ListCommuneStaffAccounts(ctx context.Context, r *identityv1.ListCommuneStaffAccountsRequest) (*identityv1.ListCommuneStaffAccountsResponse, error) {
	f.record(ctx)
	f.owner.sawListStaff = r
	return f.owner.listStaff, f.err
}

func (f *fakeServer) CreateCommuneAdminAccount(ctx context.Context, r *identityv1.CreateCommuneAdminAccountRequest) (*identityv1.CreateCommuneAdminAccountResponse, error) {
	f.record(ctx)
	f.owner.sawCreateAdm = r
	return f.owner.createAdm, f.err
}

func (f *fakeServer) ResetStaffPassword(ctx context.Context, r *identityv1.ResetStaffPasswordRequest) (*identityv1.ResetStaffPasswordResponse, error) {
	f.record(ctx)
	f.owner.sawResetStaff = r
	return f.owner.resetStaff, f.err
}

func opAccountFake(code string) *identityv1.OperatorAccount {
	return &identityv1.OperatorAccount{Code: code, Email: emailFake, DisplayName: "Người vận hành FAKE",
		PermissionKeys: []string{"ops.tenant.manage", ""}, MustChangePassword: true,
		CreatedAt: timestamppb.New(time.Unix(1_800_000_000, 0))}
}

func staffAccountFake(id string) *identityv1.CommuneStaffAccount {
	return &identityv1.CommuneStaffAccount{UserId: id, StaffCode: "CB-00001", Email: "admin", FullName: "Cán bộ FAKE",
		RoleCode: "quan-tri-he-thong", RoleName: "Quản trị hệ thống", IsAdmin: true, MustChangePassword: true}
}

func acceptedOwnerFake() ownerAnswers {
	return ownerAnswers{
		listOps:    &identityv1.ListOperatorAccountsResponse{Outcome: wAccepted, Accounts: []*identityv1.OperatorAccount{opAccountFake("VH-00001"), opAccountFake("VH-00002")}},
		createOp:   &identityv1.CreateOperatorAccountResponse{Outcome: wAccepted, Account: opAccountFake("VH-00003")},
		setPerms:   &identityv1.SetOperatorPermissionsResponse{Outcome: wAccepted, Account: opAccountFake("VH-00002")},
		resetOp:    &identityv1.ResetOperatorPasswordResponse{Outcome: wAccepted, Account: opAccountFake("VH-00002")},
		listStaff:  &identityv1.ListCommuneStaffAccountsResponse{Outcome: wAccepted, Accounts: []*identityv1.CommuneStaffAccount{staffAccountFake("u1"), staffAccountFake("u2")}},
		createAdm:  &identityv1.CreateCommuneAdminAccountResponse{Outcome: wAccepted, Account: staffAccountFake("u3")},
		resetStaff: &identityv1.ResetStaffPasswordResponse{Outcome: wAccepted, Account: staffAccountFake("u1")},
	}
}

// callAll runs the seven with complete requests; returns each error in RPC order.
func callAll(c *Client, ctx context.Context) []error {
	tok, pw := secret.Secret(tokenFake), secret.Secret(tempPasswordFake)
	_, e1 := c.ListOperatorAccounts(ctx, tok)
	_, e2 := c.CreateOperatorAccount(ctx, CreateOperatorAccountRequest{Token: tok, Email: emailFake, DisplayName: "A",
		TemporaryPassword: pw, PermissionKeys: []string{"ops.qr.issue"}, ClientIP: "203.0.113.7"})
	_, e3 := c.SetOperatorPermissions(ctx, SetOperatorPermissionsRequest{Token: tok, TargetCode: "VH-00002", Reason: "Đổi việc", ClientIP: "203.0.113.7"})
	_, e4 := c.ResetOperatorPassword(ctx, ResetOperatorPasswordRequest{Token: tok, TargetCode: "VH-00002", TemporaryPassword: pw, Reason: "Quên", ClientIP: "203.0.113.7"})
	_, e5 := c.ListCommuneStaffAccounts(ctx, tok, "203.0.113.7")
	_, e6 := c.CreateCommuneAdminAccount(ctx, CreateCommuneAdminAccountRequest{Token: tok, Email: "admin", FullName: "B", TemporaryPassword: pw, ClientIP: "203.0.113.7"})
	_, e7 := c.ResetStaffPassword(ctx, ResetStaffPasswordRequest{Token: tok, UserID: "u1", TemporaryPassword: pw, Reason: "Quên", ClientIP: "203.0.113.7"})
	return []error{e1, e2, e3, e4, e5, e6, e7}
}

func TestOwnerRPCsAcceptedAndCommuneTravel(t *testing.T) {
	f := &fakeServer{owner: acceptedOwnerFake()}
	c, logBuf := start(t, f)
	for i, err := range callAll(c, ctxWithTenant()) {
		if err != nil {
			t.Fatalf("call %d: %v", i+1, err)
		}
	}
	// The first four carry NO commune although ctx holds one; the last three carry exactly ctx's.
	want := "01JA" + strings.Repeat("A", 22)
	for i, v := range f.sawTenant {
		if i < 4 && len(v) != 0 {
			t.Errorf("call %d carried x-tenant-id=%v, want none", i+1, v)
		}
		if i >= 4 && (len(v) != 1 || v[0] != want) {
			t.Errorf("call %d carried x-tenant-id=%v, want [%s]", i+1, v, want)
		}
	}
	o := f.owner
	if o.sawCreateOp.GetTemporaryPassword() != tempPasswordFake || o.sawCreateOp.GetSessionToken() != tokenFake ||
		o.sawCreateOp.GetEmail() != emailFake || o.sawCreateOp.GetClientIp() != "203.0.113.7" ||
		len(o.sawCreateOp.GetPermissionKeys()) != 1 {
		t.Error("CreateOperatorAccount fields did not travel verbatim")
	}
	if o.sawResetOp.GetTemporaryPassword() != tempPasswordFake || o.sawResetOp.GetTargetCode() != "VH-00002" || o.sawResetOp.GetReason() != "Quên" {
		t.Error("ResetOperatorPassword fields did not travel verbatim")
	}
	if o.sawSetPerms.GetTargetCode() != "VH-00002" || o.sawSetPerms.GetReason() != "Đổi việc" || len(o.sawSetPerms.GetPermissionKeys()) != 0 {
		t.Error("SetOperatorPermissions fields did not travel")
	}
	if o.sawResetStaff.GetUserId() != "u1" || o.sawResetStaff.GetTemporaryPassword() != tempPasswordFake ||
		o.sawCreateAdm.GetTemporaryPassword() != tempPasswordFake || o.sawListStaff.GetClientIp() != "203.0.113.7" {
		t.Error("staff-account fields did not travel verbatim")
	}
	if strings.Contains(logBuf.String(), "FAKE") || strings.Contains(logBuf.String(), emailFake) {
		t.Fatalf("the log carries a credential or personal data: %s", logBuf)
	}

	// The mapping itself: blank keys dropped, absent lock is nil, created_at kept.
	res, _ := c.ListOperatorAccounts(context.Background(), secret.Secret(tokenFake))
	if len(res.Accounts) != 2 || len(res.Accounts[0].PermissionKeys) != 1 || res.Accounts[0].LockedUntil != nil ||
		res.Accounts[0].CreatedAt.Unix() != 1_800_000_000 || !res.Accounts[0].MustChangePassword {
		t.Errorf("operator accounts mapped as %+v", res.Accounts)
	}
	staff, _ := c.ListCommuneStaffAccounts(ctxWithTenant(), secret.Secret(tokenFake), "")
	if len(staff.Accounts) != 2 || !staff.Accounts[0].IsAdmin || staff.Accounts[0].StaffCode != "CB-00001" {
		t.Errorf("staff accounts mapped as %+v", staff.Accounts)
	}
	f.owner.listStaff = &identityv1.ListCommuneStaffAccountsResponse{Outcome: wAccepted}
	if staff, err := c.ListCommuneStaffAccounts(ctxWithTenant(), secret.Secret(tokenFake), ""); err != nil || staff.Accounts == nil || len(staff.Accounts) != 0 {
		t.Errorf("an empty staff list is a valid answer: %+v %v", staff, err)
	}
}

func TestOwnerRPCsRefuseLocally(t *testing.T) {
	f := &fakeServer{owner: acceptedOwnerFake()}
	c, _ := start(t, f)
	// No commune in ctx: the four exempt ones go, the three commune-scoped ones are refused here.
	errs := callAll(c, context.Background())
	for i, err := range errs {
		if i < 4 && err != nil {
			t.Errorf("call %d without a commune: %v (it is exempt)", i+1, err)
		}
		if i >= 4 && !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("call %d without a commune: %v, want ErrInvalidRequest", i+1, err)
		}
	}
	if f.calls != 4 {
		t.Fatalf("%d calls reached the server, want the 4 exempt ones", f.calls)
	}
	f.calls = 0
	ctx := ctxWithTenant()
	for name, err := range map[string]error{
		"list ops no token":  func() error { _, e := c.ListOperatorAccounts(ctx, nil); return e }(),
		"create op no token": func() error { _, e := c.CreateOperatorAccount(ctx, CreateOperatorAccountRequest{}); return e }(),
		"perms no target": func() error {
			_, e := c.SetOperatorPermissions(ctx, SetOperatorPermissionsRequest{Token: secret.Secret(tokenFake)})
			return e
		}(),
		"reset op no target": func() error {
			_, e := c.ResetOperatorPassword(ctx, ResetOperatorPasswordRequest{Token: secret.Secret(tokenFake)})
			return e
		}(),
		"list staff no token": func() error { _, e := c.ListCommuneStaffAccounts(ctx, nil, ""); return e }(),
		"create adm no token": func() error { _, e := c.CreateCommuneAdminAccount(ctx, CreateCommuneAdminAccountRequest{}); return e }(),
		"reset staff no user": func() error {
			_, e := c.ResetStaffPassword(ctx, ResetStaffPasswordRequest{Token: secret.Secret(tokenFake)})
			return e
		}(),
	} {
		if !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("%s: %v, want ErrInvalidRequest", name, err)
		}
	}
	if f.calls != 0 {
		t.Fatalf("%d refused calls reached the server", f.calls)
	}
}

func TestOwnerRPCStatusMapping(t *testing.T) {
	// want[i] is what call i (callAll's order) must answer; nil = "an untyped outage".
	for _, c := range []struct {
		name string
		code codes.Code
		want []error
	}{
		{"UNIMPLEMENTED", codes.Unimplemented, []error{ErrNotSupported, ErrNotSupported, ErrNotSupported, ErrNotSupported, ErrNotSupported, ErrNotSupported, ErrNotSupported}},
		{"NOT_FOUND", codes.NotFound, []error{nil, nil, ErrAccountNotFound, ErrAccountNotFound, nil, nil, ErrAccountNotFound}},
		{"ALREADY_EXISTS", codes.AlreadyExists, []error{nil, ErrAccountExists, nil, nil, nil, ErrAccountExists, nil}},
		{"FAILED_PRECONDITION", codes.FailedPrecondition, []error{ErrRealmNotConfigured, ErrRealmNotConfigured, ErrRealmNotConfigured, ErrRealmNotConfigured, ErrRealmNotConfigured, ErrCommuneNotLive, ErrStaffAccountLocked}},
		{"INVALID_ARGUMENT", codes.InvalidArgument, []error{ErrArgumentRefused, ErrArgumentRefused, ErrArgumentRefused, ErrArgumentRefused, ErrArgumentRefused, ErrArgumentRefused, ErrArgumentRefused}},
		{"UNAVAILABLE", codes.Unavailable, make([]error, 7)},
	} {
		t.Run(c.name, func(t *testing.T) {
			cl, logBuf := start(t, &fakeServer{err: status.Error(c.code, "x")})
			typed := []error{ErrNotSupported, ErrAccountNotFound, ErrAccountExists, ErrCommuneNotLive, ErrStaffAccountLocked, ErrArgumentRefused}
			for i, err := range callAll(cl, ctxWithTenant()) {
				if err == nil {
					t.Fatalf("call %d: a failed call answered no error", i+1)
				}
				if c.want[i] != nil && !errors.Is(err, c.want[i]) {
					t.Errorf("call %d: %v, want %v", i+1, err, c.want[i])
				}
				if c.want[i] == nil {
					// A code this RPC never answers with as a typed refusal is an outage, read as 503.
					for _, e := range typed {
						if errors.Is(err, e) {
							t.Errorf("call %d: %s was read as %v", i+1, c.name, e)
						}
					}
				}
			}
			if strings.Contains(logBuf.String(), "FAKE") {
				t.Fatalf("the log carries a credential: %s", logBuf)
			}
		})
	}
}

func TestOwnerRPCOutcomesAndContract(t *testing.T) {
	f := &fakeServer{}
	c, _ := start(t, f)
	ctx := ctxWithTenant()

	// Non-accepted outcomes are answers, not errors.
	f.owner = ownerAnswers{
		listOps:    &identityv1.ListOperatorAccountsResponse{Outcome: wNotLive},
		createOp:   &identityv1.CreateOperatorAccountResponse{Outcome: wDenied},
		setPerms:   &identityv1.SetOperatorPermissionsResponse{Outcome: wDenied},
		resetOp:    &identityv1.ResetOperatorPasswordResponse{Outcome: wNotLive},
		listStaff:  &identityv1.ListCommuneStaffAccountsResponse{Outcome: wDenied},
		createAdm:  &identityv1.CreateCommuneAdminAccountResponse{Outcome: wNotLive},
		resetStaff: &identityv1.ResetStaffPasswordResponse{Outcome: wDenied},
	}
	for i, err := range callAll(c, ctx) {
		if err != nil {
			t.Errorf("call %d: non-accepted outcome answered an error: %v", i+1, err)
		}
	}
	if r, _ := c.ListOperatorAccounts(ctx, secret.Secret(tokenFake)); r.Outcome != OutcomeSessionNotLive {
		t.Errorf("SESSION_NOT_LIVE read as %v", r.Outcome)
	}
	if r, _ := c.ResetStaffPassword(ctx, ResetStaffPasswordRequest{Token: secret.Secret(tokenFake), UserID: "u1"}); r.Outcome != OutcomePermissionDenied {
		t.Errorf("PERMISSION_DENIED read as %v", r.Outcome)
	}

	// NEW_PASSWORD_REJECTED carries the rule on the four RPCs that set a password.
	ref := &identityv1.NewPasswordRefusal{Problem: identityv1.NewPasswordProblem_NEW_PASSWORD_PROBLEM_TOO_SHORT, MinLength: 12, MaxLength: 128}
	f.owner.createOp = &identityv1.CreateOperatorAccountResponse{Outcome: wRejected, NewPasswordRefusal: ref}
	f.owner.resetOp = &identityv1.ResetOperatorPasswordResponse{Outcome: wRejected, NewPasswordRefusal: ref}
	f.owner.createAdm = &identityv1.CreateCommuneAdminAccountResponse{Outcome: wRejected, NewPasswordRefusal: ref}
	f.owner.resetStaff = &identityv1.ResetStaffPasswordResponse{Outcome: wRejected, NewPasswordRefusal: ref}
	r1, e1 := c.CreateOperatorAccount(ctx, CreateOperatorAccountRequest{Token: secret.Secret(tokenFake)})
	r2, e2 := c.ResetStaffPassword(ctx, ResetStaffPasswordRequest{Token: secret.Secret(tokenFake), UserID: "u1"})
	if e1 != nil || e2 != nil || r1.Refusal.MinLength != 12 || r2.Refusal.Problem != ref.Problem {
		t.Errorf("rejection: %+v %v / %+v %v", r1, e1, r2, e2)
	}

	// Contract faults: never served.
	for name, set := range map[string]func(){
		"list ops UNSPECIFIED": func() { f.owner.listOps = &identityv1.ListOperatorAccountsResponse{} },
		"list ops REFUSED":     func() { f.owner.listOps = &identityv1.ListOperatorAccountsResponse{Outcome: wRefused} },
		"list ops empty":       func() { f.owner.listOps = &identityv1.ListOperatorAccountsResponse{Outcome: wAccepted} },
		"list ops no code": func() {
			f.owner.listOps = &identityv1.ListOperatorAccountsResponse{Outcome: wAccepted, Accounts: []*identityv1.OperatorAccount{opAccountFake("")}}
		},
		"list ops duplicate": func() {
			f.owner.listOps = &identityv1.ListOperatorAccountsResponse{Outcome: wAccepted, Accounts: []*identityv1.OperatorAccount{opAccountFake("VH-1"), opAccountFake("VH-1")}}
		},
		"perms REJECTED (not set)": func() { f.owner.setPerms = &identityv1.SetOperatorPermissionsResponse{Outcome: wRejected} },
		"perms no account":         func() { f.owner.setPerms = &identityv1.SetOperatorPermissionsResponse{Outcome: wAccepted} },
		"create op no refusal":     func() { f.owner.createOp = &identityv1.CreateOperatorAccountResponse{Outcome: wRejected} },
		"staff no user_id": func() {
			f.owner.listStaff = &identityv1.ListCommuneStaffAccountsResponse{Outcome: wAccepted, Accounts: []*identityv1.CommuneStaffAccount{staffAccountFake("")}}
		},
		"staff duplicate": func() {
			f.owner.listStaff = &identityv1.ListCommuneStaffAccountsResponse{Outcome: wAccepted, Accounts: []*identityv1.CommuneStaffAccount{staffAccountFake("u"), staffAccountFake("u")}}
		},
		"create adm no account":   func() { f.owner.createAdm = &identityv1.CreateCommuneAdminAccountResponse{Outcome: wAccepted} },
		"reset staff UNSPECIFIED": func() { f.owner.resetStaff = &identityv1.ResetStaffPasswordResponse{} },
	} {
		f.owner = acceptedOwnerFake()
		set()
		gotContract := false
		for _, err := range callAll(c, ctx) {
			if errors.Is(err, ErrContract) {
				gotContract = true
			}
		}
		if !gotContract {
			t.Errorf("%s: no ErrContract (fail closed)", name)
		}
	}
}
