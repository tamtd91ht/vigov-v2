package domain

import "time"

// StaffSignInAccount is one SIGN-IN ACCOUNT of a commune (`nguoi_dung` with co_tai_khoan, not
// soft-deleted) as the operator OWNER's screen shows it (operator.proto, CommuneStaffAccount;
// ADR 0048 §"Sửa đổi 13/10/2026"). The narrowest shape that lets the owner find the person and act:
// no phone, no unit, no position, no last sign-in.
//
// Email and FullName are PERSONAL DATA (rule 3): never logged, never put in an error or a trail
// entry. Code (`CB-…`) is what identifies the person everywhere else.
type StaffSignInAccount struct {
	ID       string
	Code     string
	Email    string
	FullName string
	// RoleCode / RoleName — the one role the row holds (`vai_tro.ma`, `vai_tro.ten`); both empty
	// when it holds none, or when that role was soft-deleted (a deleted role grants nothing).
	RoleCode string
	RoleName string
	// Disabled is the commune's administrative lock: dang_hoat_dong = false.
	Disabled           bool
	MustChangePassword bool
	// SignInLockedUntil is the automatic failed-sign-in lock's end (migration 0020); nil = none.
	// Derived against now by the reader, never a flag.
	SignInLockedUntil *time.Time
}

// CommuneAdminRoleCode is the commune administrator role's code — store.MaVaiTroQuanTriMacDinh,
// restated here because domain imports nothing but the standard library; pinned equal by a test in
// internal/store.
const CommuneAdminRoleCode = "quan-tri-he-thong"

// IsAdmin reports whether the account holds the commune administrator role.
func (a StaffSignInAccount) IsAdmin() bool { return a.RoleCode == CommuneAdminRoleCode }
