package grpc

// The SEVEN OWNER-ONLY OperatorService RPCs (operator.proto §"THE OWNER-ONLY RPCs"; ADR 0048
// §"Sửa đổi 13/10/2026 — owner, …"). This file translates; the use cases are app.OwnerAdmin
// (operator accounts, no commune) and app.CommuneAccountsByOwner (one commune's sign-in accounts).
//
// THE ORDER OF EVERY HANDLER, and why it is this order:
//
//  1. INVALID_ARGUMENT from the request alone — commune in ctx (the three commune-scoped ones), token,
//     field shapes. Decided before any lookup, so it says nothing about any account.
//  2. The session, resolved HERE from session_token (idle and absolute expiry, revocation, disabled
//     account — ResolveSession). Not live → OK + SESSION_NOT_LIVE.
//  3. is_owner on THE principal resolved in step 2 — never a code or a flag the caller names (the
//     caller key is shared by every ViGov service). Not the owner → OK + PERMISSION_DENIED, before
//     any account is read, so a non-owner learns nothing about who exists.
//  4. The password policy (NEW_PASSWORD_REJECTED) — after the owner check, so a non-owner cannot use
//     these RPCs as a policy oracle either; before any account read.
//  5. The act. NOT_FOUND / ALREADY_EXISTS / FAILED_PRECONDITION only from here on.
//
// NEVER LOG A REQUEST: temporary passwords, bearer tokens, emails and names travel in them.

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// OwnerOperatorAdmin is the use case behind the four operator-account RPCs. *app.OwnerAdmin.
type OwnerOperatorAdmin interface {
	ListAccounts(ctx context.Context) ([]app.OperatorAccountView, error)
	CreateAccount(ctx context.Context, owner app.MiniAppOperator, in app.NewOperatorByOwner) (app.OperatorAccountView, error)
	SetPermissions(ctx context.Context, owner app.MiniAppOperator, targetCode string, keys []domain.OperatorPermission,
		reason string) (app.OperatorAccountView, error)
	ResetPassword(ctx context.Context, owner app.MiniAppOperator, targetCode, temp, reason string) (app.OperatorAccountView, error)
}

// OwnerCommuneAccounts is the use case behind the three commune-scoped RPCs. *app.CommuneAccountsByOwner.
// Every method scopes to the commune in ctx (store.DB.For(ctx)); none takes a commune argument.
type OwnerCommuneAccounts interface {
	List(ctx context.Context, owner app.MiniAppOperator) ([]domain.StaffSignInAccount, error)
	CreateAdmin(ctx context.Context, owner app.MiniAppOperator, email, fullName, temp string) (domain.StaffSignInAccount, error)
	ResetPassword(ctx context.Context, owner app.MiniAppOperator, operatorID, userID, temp, reason string) (domain.StaffSignInAccount, error)
}

// WithOwnerAdmin wires the seven owner-only RPCs. Without it they answer Unimplemented. Both use cases
// are required together: half the owner's screen working is a state nobody tested.
func (s *OperatorServer) WithOwnerAdmin(ops OwnerOperatorAdmin, communes OwnerCommuneAccounts) *OperatorServer {
	if ops == nil || communes == nil {
		panic("identity/grpc: WithOwnerAdmin without both use cases")
	}
	s.ownerOperators, s.ownerCommunes = ops, communes
	return s
}

var errOwnerNotWired = status.Error(codes.Unimplemented, "chưa nối các thao tác chỉ dành cho owner")

// resolveOwner is steps 2 and 3. outAccepted with the principal, or the outcome to answer.
func (s *OperatorServer) resolveOwner(ctx context.Context, rpc, token string) (app.OperatorPrincipal,
	identityv1.OperatorAuthOutcome, error) {
	p, err := s.uc.ResolveSession(ctx, token)
	if errors.Is(err, app.ErrOperatorUnauthenticated) {
		return app.OperatorPrincipal{}, outNotLive, nil
	}
	if err != nil {
		return app.OperatorPrincipal{}, 0, s.fault(ctx, rpc, err)
	}
	if p.ID == "" || p.Code == "" {
		// As ResolveOperatorSession: no VH- code means no "who" for the trail (rule 6, invariant 8).
		s.log.ErrorContext(ctx, "CẢNH BÁO HỢP ĐỒNG: phiên vận hành không có operator_id hoặc operator_code", "rpc", rpc)
		return app.OperatorPrincipal{}, 0, status.Error(codes.Internal, "lỗi nội bộ, vui lòng thử lại")
	}
	// The owner flag alone is not enough: an owner whose session is RESTRICTED (temporary password
	// not yet replaced) is refused like anybody else — second layer behind service-platform.
	if !p.IsOwner || p.MustChangePassword {
		return app.OperatorPrincipal{}, outDenied, nil
	}
	return p, outAccepted, nil
}

func ownerActor(p app.OperatorPrincipal, ip string) app.MiniAppOperator {
	return app.MiniAppOperator{Code: p.Code, IP: ip}
}

// policyRefusal is step 4: the staff password policy on the typed temporary password.
func policyRefusal(temp string) (*identityv1.NewPasswordRefusal, bool) {
	return newPasswordRefusal(domain.KiemTraMatKhauMoi(temp))
}

// --- shape (step 1) -------------------------------------------------------------------------------

func invalid(msg string) error { return status.Error(codes.InvalidArgument, msg) }

func keysShape(raw []string) ([]domain.OperatorPermission, error) {
	keys, err := app.CheckOperatorPermissionKeys(raw)
	if err != nil {
		return nil, invalid("permission_keys có khoá không thuộc danh sách ops.*")
	}
	return keys, nil
}

func reasonShape(raw string) error {
	if _, err := app.CheckOwnerReason(raw); err != nil {
		return invalid("reason phải có 1-500 ký tự, không ký tự điều khiển ngoài xuống dòng")
	}
	return nil
}

// communeShape is the commune-scoped RPCs' first check: x-tenant-id lifted into ctx, then the token.
func communeShape(ctx context.Context, token string) error {
	if _, ok := tenant.From(ctx); !ok {
		return invalid("thiếu xã đích trong metadata x-tenant-id")
	}
	if token == "" {
		return errEmptyToken
	}
	return nil
}

// --- the four operator-account RPCs (no commune) --------------------------------------------------

// ListOperatorAccounts — ACCEPTED, SESSION_NOT_LIVE or PERMISSION_DENIED.
func (s *OperatorServer) ListOperatorAccounts(ctx context.Context, req *identityv1.ListOperatorAccountsRequest) (
	*identityv1.ListOperatorAccountsResponse, error) {
	if s.ownerOperators == nil {
		return nil, errOwnerNotWired
	}
	if req.GetSessionToken() == "" {
		return nil, errEmptyToken
	}
	_, out, err := s.resolveOwner(ctx, "ListOperatorAccounts", req.GetSessionToken())
	if err != nil || out != outAccepted {
		return &identityv1.ListOperatorAccountsResponse{Outcome: out}, err
	}
	// @cross-tenant: operator realm has no commune (ADR 0048 §Chốt #1) — the operator register only.
	views, err := s.ownerOperators.ListAccounts(ctx)
	if err != nil {
		return nil, s.fault(ctx, "ListOperatorAccounts", err)
	}
	accounts := make([]*identityv1.OperatorAccount, 0, len(views))
	for _, v := range views {
		accounts = append(accounts, operatorAccountMsg(v))
	}
	return &identityv1.ListOperatorAccountsResponse{Outcome: outAccepted, Accounts: accounts}, nil
}

// CreateOperatorAccount — ACCEPTED, SESSION_NOT_LIVE, PERMISSION_DENIED or NEW_PASSWORD_REJECTED;
// ALREADY_EXISTS on an email an account already has.
func (s *OperatorServer) CreateOperatorAccount(ctx context.Context, req *identityv1.CreateOperatorAccountRequest) (
	*identityv1.CreateOperatorAccountResponse, error) {
	if s.ownerOperators == nil {
		return nil, errOwnerNotWired
	}
	if req.GetSessionToken() == "" {
		return nil, errEmptyToken
	}
	email, name, keys, err := app.CheckNewOperatorShape(req.GetEmail(), req.GetDisplayName(), req.GetPermissionKeys())
	switch {
	case errors.Is(err, app.ErrInvalidOperatorEmail):
		return nil, invalid("email không đúng định dạng")
	case errors.Is(err, app.ErrInvalidOperatorName):
		return nil, invalid("display_name phải có 1-200 ký tự")
	case err != nil:
		return nil, invalid("permission_keys có khoá không thuộc danh sách ops.*")
	}
	p, out, err := s.resolveOwner(ctx, "CreateOperatorAccount", req.GetSessionToken())
	if err != nil || out != outAccepted {
		return &identityv1.CreateOperatorAccountResponse{Outcome: out}, err
	}
	if r, bad := policyRefusal(req.GetTemporaryPassword()); bad {
		return &identityv1.CreateOperatorAccountResponse{Outcome: outRejected, NewPasswordRefusal: r}, nil
	}
	v, err := s.ownerOperators.CreateAccount(ctx, ownerActor(p, req.GetClientIp()), app.NewOperatorByOwner{
		Email: email, DisplayName: name, TemporaryPassword: req.GetTemporaryPassword(), Permissions: keys,
	})
	if r, bad := newPasswordRefusal(err); bad {
		return &identityv1.CreateOperatorAccountResponse{Outcome: outRejected, NewPasswordRefusal: r}, nil
	}
	switch {
	case err == nil:
		return &identityv1.CreateOperatorAccountResponse{Outcome: outAccepted, Account: operatorAccountMsg(v)}, nil
	case errors.Is(err, app.ErrOperatorEmailTaken):
		return nil, status.Error(codes.AlreadyExists, "đã có tài khoản vận hành dùng email này")
	}
	return nil, s.fault(ctx, "CreateOperatorAccount", err)
}

// SetOperatorPermissions — ACCEPTED, SESSION_NOT_LIVE or PERMISSION_DENIED; NOT_FOUND for an unknown
// target_code; FAILED_PRECONDITION when the target is the owner (its rights come from the flag).
func (s *OperatorServer) SetOperatorPermissions(ctx context.Context, req *identityv1.SetOperatorPermissionsRequest) (
	*identityv1.SetOperatorPermissionsResponse, error) {
	if s.ownerOperators == nil {
		return nil, errOwnerNotWired
	}
	if req.GetSessionToken() == "" {
		return nil, errEmptyToken
	}
	if req.GetTargetCode() == "" {
		return nil, invalid("target_code trống")
	}
	keys, err := keysShape(req.GetPermissionKeys())
	if err != nil {
		return nil, err
	}
	if err := reasonShape(req.GetReason()); err != nil {
		return nil, err
	}
	p, out, err := s.resolveOwner(ctx, "SetOperatorPermissions", req.GetSessionToken())
	if err != nil || out != outAccepted {
		return &identityv1.SetOperatorPermissionsResponse{Outcome: out}, err
	}
	v, err := s.ownerOperators.SetPermissions(ctx, ownerActor(p, req.GetClientIp()), req.GetTargetCode(), keys, req.GetReason())
	if err == nil {
		return &identityv1.SetOperatorPermissionsResponse{Outcome: outAccepted, Account: operatorAccountMsg(v)}, nil
	}
	return nil, s.ownerOperatorFault(ctx, "SetOperatorPermissions", err)
}

// ResetOperatorPassword — ACCEPTED, SESSION_NOT_LIVE, PERMISSION_DENIED or NEW_PASSWORD_REJECTED;
// NOT_FOUND for an unknown target_code; FAILED_PRECONDITION when the target is the caller.
func (s *OperatorServer) ResetOperatorPassword(ctx context.Context, req *identityv1.ResetOperatorPasswordRequest) (
	*identityv1.ResetOperatorPasswordResponse, error) {
	if s.ownerOperators == nil {
		return nil, errOwnerNotWired
	}
	if req.GetSessionToken() == "" {
		return nil, errEmptyToken
	}
	if req.GetTargetCode() == "" {
		return nil, invalid("target_code trống")
	}
	if err := reasonShape(req.GetReason()); err != nil {
		return nil, err
	}
	p, out, err := s.resolveOwner(ctx, "ResetOperatorPassword", req.GetSessionToken())
	if err != nil || out != outAccepted {
		return &identityv1.ResetOperatorPasswordResponse{Outcome: out}, err
	}
	if r, bad := policyRefusal(req.GetTemporaryPassword()); bad {
		return &identityv1.ResetOperatorPasswordResponse{Outcome: outRejected, NewPasswordRefusal: r}, nil
	}
	v, err := s.ownerOperators.ResetPassword(ctx, ownerActor(p, req.GetClientIp()), req.GetTargetCode(),
		req.GetTemporaryPassword(), req.GetReason())
	if r, bad := newPasswordRefusal(err); bad {
		return &identityv1.ResetOperatorPasswordResponse{Outcome: outRejected, NewPasswordRefusal: r}, nil
	}
	if err == nil {
		return &identityv1.ResetOperatorPasswordResponse{Outcome: outAccepted, Account: operatorAccountMsg(v)}, nil
	}
	return nil, s.ownerOperatorFault(ctx, "ResetOperatorPassword", err)
}

// ownerOperatorFault maps the operator-account refusals; anything else is fault's table.
func (s *OperatorServer) ownerOperatorFault(ctx context.Context, rpc string, err error) error {
	switch {
	case errors.Is(err, app.ErrOperatorNotFound):
		return status.Error(codes.NotFound, "không có tài khoản vận hành với mã này")
	case errors.Is(err, app.ErrOwnerTargetIsOwner):
		return status.Error(codes.FailedPrecondition, "owner có mọi quyền theo cờ owner — không đặt khoá cho owner")
	case errors.Is(err, app.ErrOwnerTargetIsSelf):
		return status.Error(codes.FailedPrecondition, "không đặt lại mật khẩu của chính mình — dùng đổi mật khẩu")
	}
	return s.fault(ctx, rpc, err)
}

// --- the three commune-scoped RPCs (commune in "x-tenant-id") -------------------------------------

// ListCommuneStaffAccounts — ACCEPTED (possibly empty), SESSION_NOT_LIVE or PERMISSION_DENIED. The
// read is audited in the commune's audit_log; no answer leaves unless the entry committed.
func (s *OperatorServer) ListCommuneStaffAccounts(ctx context.Context, req *identityv1.ListCommuneStaffAccountsRequest) (
	*identityv1.ListCommuneStaffAccountsResponse, error) {
	if s.ownerCommunes == nil {
		return nil, errOwnerNotWired
	}
	if err := communeShape(ctx, req.GetSessionToken()); err != nil {
		return nil, err
	}
	p, out, err := s.resolveOwner(ctx, "ListCommuneStaffAccounts", req.GetSessionToken())
	if err != nil || out != outAccepted {
		return &identityv1.ListCommuneStaffAccountsResponse{Outcome: out}, err
	}
	// Scoped: the commune is the one communeShape found in ctx; the use case opens db.For(ctx).Tx and
	// the store binds it as $1.
	rows, err := s.ownerCommunes.List(ctx, ownerActor(p, req.GetClientIp()))
	if err != nil {
		return nil, s.fault(ctx, "ListCommuneStaffAccounts", err)
	}
	accounts := make([]*identityv1.CommuneStaffAccount, 0, len(rows))
	for _, a := range rows {
		accounts = append(accounts, communeStaffAccountMsg(a))
	}
	return &identityv1.ListCommuneStaffAccountsResponse{Outcome: outAccepted, Accounts: accounts}, nil
}

// CreateCommuneAdminAccount — ACCEPTED, SESSION_NOT_LIVE, PERMISSION_DENIED or NEW_PASSWORD_REJECTED;
// ALREADY_EXISTS on an email the commune holds; FAILED_PRECONDITION when the commune is not live (or
// its administrator role was soft-deleted — see the message).
func (s *OperatorServer) CreateCommuneAdminAccount(ctx context.Context, req *identityv1.CreateCommuneAdminAccountRequest) (
	*identityv1.CreateCommuneAdminAccountResponse, error) {
	if s.ownerCommunes == nil {
		return nil, errOwnerNotWired
	}
	if err := communeShape(ctx, req.GetSessionToken()); err != nil {
		return nil, err
	}
	if _, _, err := app.CheckCommuneAdminShape(req.GetEmail(), req.GetFullName()); err != nil {
		if errors.Is(err, domain.ErrThieuHoTen) || errors.Is(err, domain.ErrHoTenQuaDai) {
			return nil, invalid("full_name trống hoặc quá dài")
		}
		return nil, invalid("email không đúng định dạng thư điện tử")
	}
	p, out, err := s.resolveOwner(ctx, "CreateCommuneAdminAccount", req.GetSessionToken())
	if err != nil || out != outAccepted {
		return &identityv1.CreateCommuneAdminAccountResponse{Outcome: out}, err
	}
	if r, bad := policyRefusal(req.GetTemporaryPassword()); bad {
		return &identityv1.CreateCommuneAdminAccountResponse{Outcome: outRejected, NewPasswordRefusal: r}, nil
	}
	// Scoped: the commune is the one communeShape found in ctx (db.For(ctx) in the use case).
	a, err := s.ownerCommunes.CreateAdmin(ctx, ownerActor(p, req.GetClientIp()), req.GetEmail(), req.GetFullName(),
		req.GetTemporaryPassword())
	if r, bad := newPasswordRefusal(err); bad {
		return &identityv1.CreateCommuneAdminAccountResponse{Outcome: outRejected, NewPasswordRefusal: r}, nil
	}
	switch {
	case err == nil:
		return &identityv1.CreateCommuneAdminAccountResponse{Outcome: outAccepted, Account: communeStaffAccountMsg(a)}, nil
	case errors.Is(err, app.ErrCommuneAdminEmailTaken):
		return nil, status.Error(codes.AlreadyExists, "xã đã có tài khoản dùng email này")
	case errors.Is(err, app.ErrCommuneNotLive):
		return nil, status.Error(codes.FailedPrecondition, "xã không tồn tại hoặc đang ngừng hoạt động trên nền tảng")
	case errors.Is(err, idstore.ErrVaiTroQuanTriDaXoa):
		return nil, status.Error(codes.FailedPrecondition,
			"vai trò quản trị hệ thống của xã đã bị xoá mềm — không tạo lại thay xã")
	}
	return nil, s.fault(ctx, "CreateCommuneAdminAccount", err)
}

// ResetStaffPassword — ACCEPTED, SESSION_NOT_LIVE, PERMISSION_DENIED or NEW_PASSWORD_REJECTED;
// NOT_FOUND when user_id is not a live sign-in account of THIS commune; FAILED_PRECONDITION when the
// account is administratively locked.
func (s *OperatorServer) ResetStaffPassword(ctx context.Context, req *identityv1.ResetStaffPasswordRequest) (
	*identityv1.ResetStaffPasswordResponse, error) {
	if s.ownerCommunes == nil {
		return nil, errOwnerNotWired
	}
	if err := communeShape(ctx, req.GetSessionToken()); err != nil {
		return nil, err
	}
	if req.GetUserId() == "" {
		return nil, invalid("user_id trống")
	}
	if err := reasonShape(req.GetReason()); err != nil {
		return nil, err
	}
	p, out, err := s.resolveOwner(ctx, "ResetStaffPassword", req.GetSessionToken())
	if err != nil || out != outAccepted {
		return &identityv1.ResetStaffPasswordResponse{Outcome: out}, err
	}
	if r, bad := policyRefusal(req.GetTemporaryPassword()); bad {
		return &identityv1.ResetStaffPasswordResponse{Outcome: outRejected, NewPasswordRefusal: r}, nil
	}
	// Scoped: the commune is the one communeShape found in ctx (db.For(ctx) in the use case).
	a, err := s.ownerCommunes.ResetPassword(ctx, ownerActor(p, req.GetClientIp()), p.ID, req.GetUserId(),
		req.GetTemporaryPassword(), req.GetReason())
	if r, bad := newPasswordRefusal(err); bad {
		return &identityv1.ResetStaffPasswordResponse{Outcome: outRejected, NewPasswordRefusal: r}, nil
	}
	switch {
	case err == nil:
		return &identityv1.ResetStaffPasswordResponse{Outcome: outAccepted, Account: communeStaffAccountMsg(a)}, nil
	case errors.Is(err, idstore.ErrCanBoKhongTonTai), errors.Is(err, app.ErrChuaCoTaiKhoan):
		// One answer for unknown, another commune's, soft-deleted and directory-only.
		return nil, status.Error(codes.NotFound, "xã không có tài khoản đăng nhập này")
	case errors.Is(err, app.ErrTaiKhoanDangKhoa):
		return nil, status.Error(codes.FailedPrecondition, "tài khoản đang bị xã khoá — xã mở khoá trước")
	}
	return nil, s.fault(ctx, "ResetStaffPassword", err)
}

// --- messages ---------------------------------------------------------------------------------------

func operatorAccountMsg(v app.OperatorAccountView) *identityv1.OperatorAccount {
	keys := make([]string, 0, len(v.Permissions))
	for _, k := range v.Permissions {
		keys = append(keys, string(k))
	}
	a := v.Account
	m := &identityv1.OperatorAccount{
		Code: a.Code, Email: a.Email, DisplayName: a.DisplayName, PermissionKeys: keys,
		IsOwner: a.IsOwner, Disabled: a.Disabled(), MustChangePassword: a.MustChangePassword,
		CreatedAt: timestamppb.New(a.CreatedAt),
	}
	if a.LockedUntil != nil {
		m.LockedUntil = timestamppb.New(*a.LockedUntil)
	}
	return m
}

func communeStaffAccountMsg(a domain.StaffSignInAccount) *identityv1.CommuneStaffAccount {
	m := &identityv1.CommuneStaffAccount{
		UserId: a.ID, StaffCode: a.Code, Email: a.Email, FullName: a.FullName,
		RoleCode: a.RoleCode, RoleName: a.RoleName, IsAdmin: a.IsAdmin(),
		Disabled: a.Disabled, MustChangePassword: a.MustChangePassword,
	}
	if a.SignInLockedUntil != nil {
		m.SignInLockedUntil = timestamppb.New(*a.SignInLockedUntil)
	}
	return m
}
