package domain

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// The reason an administrator gives for revoking somebody's sign-in account (user decision
// 2026-10-03: the reason is MANDATORY).
//
// ITS OWN TWO SENTINELS AND NOT ErrThieuLyDoXoa / ErrLyDoXoaQuaDai, although the rule is the same
// shape: those sentences say "lý do xoá", and an administrator revoking an account would be told
// they are deleting somebody — the one confusion #10 exists to prevent.
var (
	ErrRevokeReasonMissing = errors.New("can_bo: thiếu lý do thu hồi tài khoản — thu hồi quyền đăng nhập của một người phải ghi vì sao")
	ErrRevokeReasonTooLong = errors.New("can_bo: lý do thu hồi tài khoản quá dài")
)

// maxRevokeReason is the same ceiling as the soft-delete reason (tranLyDoXoa): it refuses a
// payload, not an explanation.
const maxRevokeReason = 500

// NormalizeRevokeReason trims and bounds the reason. Counted in RUNES, for the reason
// ChuanHoaLyDoXoa gives: a byte bound would cut a Vietnamese sentence at a third of its length.
func NormalizeRevokeReason(raw string) (string, error) {
	reason := strings.TrimSpace(raw)
	if reason == "" {
		return "", ErrRevokeReasonMissing
	}
	if utf8.RuneCountInString(reason) > maxRevokeReason {
		return "", fmt.Errorf("%w (tối đa %d ký tự)", ErrRevokeReasonTooLong, maxRevokeReason)
	}
	return reason, nil
}
