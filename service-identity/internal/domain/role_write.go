package domain

// The shape rules of the commune's OWN roles, as they arrive on the write routes
// POST /api/v1/roles, PUT /api/v1/roles/{id} and POST /api/v1/roles/{id}/deletion.
//
// OWNER DECISION 15/10/2026: a "vai trò" is a staff JOB TITLE carrying a permission set (Chủ tịch
// UBND, Phó Chủ tịch, Bí thư…); a commune creates, renames and soft-deletes its own on the Phân quyền
// screen. This reverses the 28/09/2026 "no create-role route" note (role_template.go).
//
// STANDARD LIBRARY ONLY, like every file in this package. Unicode normalisation (NFC) is the app
// layer's step before these functions run — bo_phan_ghi.go states why this package carries no
// golang.org/x/text.
//
// NOTHING HERE IS PERSONAL DATA (rule 3): a role's name describes the authority's structure.

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// MaxRoleName bounds a role name in RUNES. "Trưởng thôn, Tổ trưởng dân phố" — the longest template
	// — is 30; the matrix renders one column header per role, so a hundred is already more than a
	// header can show.
	MaxRoleName = 100

	// MaxRoleDeleteReason — the same 500 every other soft delete in this service uses
	// (MaxOrgUnitDeleteReason, ChuanHoaLyDoXoa).
	MaxRoleDeleteReason = 500

	// roleCodeSuffixes is how many candidates a derived code tries (`base`, `base-2` … `base-100`).
	// Past that the commune is not naming roles; it is a loop.
	roleCodeSuffixes = 100
)

var (
	// ErrRoleNameInvalid — blank after trimming, over MaxRoleName runes, a control character, or a
	// name nothing can be derived from ("!!!": no letter or digit, so no code). 422 invalid_name.
	ErrRoleNameInvalid = errors.New("vai_tro: tên vai trò không hợp lệ")

	// ErrRoleNameTaken — another LIVE role of the commune already carries this name, compared
	// case-insensitively after trimming (foldName). A soft-deleted role's name is free again: it is
	// on no screen and holds nobody. 409 duplicate_name.
	ErrRoleNameTaken = errors.New("vai_tro: xã đã có một vai trò cùng tên")

	// ErrRoleDeleteReasonInvalid — the reason is REQUIRED (rule 7, invariant 1: `delete_reason`) and
	// bounded. 422 invalid_reason.
	ErrRoleDeleteReasonInvalid = errors.New("vai_tro: lý do xoá vai trò là bắt buộc và tối đa 500 ký tự")
)

// ExistingRole is one role row as the write path reads it.
type ExistingRole struct {
	ID       string
	Code     string // vai_tro.ma — issued once, never rewritten
	Name     string
	IsLeader bool
	Order    int // vai_tro.thu_tu
}

// IsSystemRole reports the commune administrator role. It is created by the system (first `admin`
// sign-in, or the owner's CreateCommuneAdminAccount) holding every key, and both of those paths find
// it BY THIS CODE — so it is never renamed, re-flagged or deleted through the commune's routes.
func IsSystemRole(code string) bool { return code == CommuneAdminRoleCode }

// NormalizeRoleName trims a role name and checks it. The caller has already NFC-normalised it.
//
// THE CASE IS KEPT AS TYPED, for the reason ChuanHoaTenBoPhan gives: this is what a person typed into
// a record, and the screen can style it.
func NormalizeRoleName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	n := utf8.RuneCountInString(name)
	if n == 0 || n > MaxRoleName {
		return "", ErrRoleNameInvalid
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			// A newline inside a column header breaks the matrix and every role picker.
			return "", ErrRoleNameInvalid
		}
	}
	return name, nil
}

// CheckRoleNameFree refuses `name` when a live role other than `selfID` carries the same folded name.
// `live` must be the commune's LIVE roles — the caller's query decides that; this only compares.
// selfID "" (a creation) excludes nothing; a rename to the role's own name passes.
func CheckRoleNameFree(name, selfID string, live []ExistingRole) error {
	key := foldName(name)
	for _, r := range live {
		if r.ID != selfID && foldName(r.Name) == key {
			return ErrRoleNameTaken
		}
	}
	return nil
}

// RoleCodeBase derives the code base from a role name — the org chart's slug (SinhMaBoPhan):
// "Phó Chủ tịch UBND" → "pho-chu-tich-ubnd". A name with no letter or digit has no code and is
// refused as an invalid name, never given a random one: the code is permanent (rule 7, invariant 3).
func RoleCodeBase(name string) (string, error) {
	base, err := SinhMaBoPhan(name)
	if err != nil {
		return "", ErrRoleNameInvalid
	}
	return base, nil
}

// PickRoleCode returns the first free candidate of base, base-2, base-3, … — "free" counting every
// code in `taken`, which must include SOFT-DELETED rows: `UNIQUE (tenant_id, ma)` is not partial
// (migration 0001), so a deleted role's code is never issued again.
//
// THE ADMINISTRATOR CODE IS NEVER A CANDIDATE, even when the commune has no such role yet. A role
// typed "Quản trị hệ thống" would otherwise take `quan-tri-he-thong`, and the first-admin seed and the
// owner's CreateCommuneAdminAccount find the administrator role BY THAT CODE — they would adopt a role
// the commune made, with whatever keys it was given, as the administrator role.
//
// false when every candidate is taken.
func PickRoleCode(base string, taken []string) (string, bool) {
	used := make(map[string]bool, len(taken))
	for _, c := range taken {
		used[c] = true
	}
	for n := 1; n <= roleCodeSuffixes; n++ {
		c := MaBoPhanThuN(base, n)
		if !used[c] && !IsSystemRole(c) {
			return c, true
		}
	}
	return "", false
}

// NextRoleOrder is `thu_tu` for a new role: after the last live one, so it appears as the matrix's
// last column. 1 on a commune with no role, the templates' own first rank.
func NextRoleOrder(live []ExistingRole) int {
	last := 0
	for _, r := range live {
		last = max(last, r.Order)
	}
	return last + 1
}

// NormalizeRoleDeleteReason trims and bounds the deletion reason. Unlike the org chart's (optional
// since 10/10/2026) it is REQUIRED: removing a job title is a decision about the authority's
// structure, and nothing else on the row says why it was taken.
func NormalizeRoleDeleteReason(raw string) (string, error) {
	reason := strings.TrimSpace(raw)
	n := utf8.RuneCountInString(reason)
	if n == 0 || n > MaxRoleDeleteReason {
		return "", ErrRoleDeleteReasonInvalid
	}
	for _, r := range reason {
		// Line breaks and tabs are text in a free-form reason; anything else in Cc is not.
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return "", fmt.Errorf("%w (chứa ký tự điều khiển)", ErrRoleDeleteReasonInvalid)
		}
	}
	return reason, nil
}
