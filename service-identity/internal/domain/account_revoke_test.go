package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeRevokeReasonRequiredTrimmedAndBoundedInRunes(t *testing.T) {
	got, err := NormalizeRevokeReason("  Dòng nhập trùng, cần xoá  ")
	if err != nil {
		t.Fatalf("lỗi: %v", err)
	}
	if got != "Dòng nhập trùng, cần xoá" {
		t.Errorf("= %q, muốn bản đã cắt khoảng trắng hai đầu", got)
	}

	for _, raw := range []string{"", "   ", "\t\n"} {
		if _, err := NormalizeRevokeReason(raw); !errors.Is(err, ErrRevokeReasonMissing) {
			t.Errorf("%q: lỗi = %v, muốn ErrRevokeReasonMissing", raw, err)
		}
	}

	// Exactly at the ceiling in multi-byte runes: accepted. One more: refused.
	if _, err := NormalizeRevokeReason(strings.Repeat("ữ", maxRevokeReason)); err != nil {
		t.Errorf("đúng trần %d ký tự mà bị từ chối: %v", maxRevokeReason, err)
	}
	if _, err := NormalizeRevokeReason(strings.Repeat("ữ", maxRevokeReason+1)); !errors.Is(err, ErrRevokeReasonTooLong) {
		t.Errorf("vượt trần: lỗi = %v, muốn ErrRevokeReasonTooLong", err)
	}

	// The sentence must not send an administrator to think they are DELETING somebody (#10).
	if strings.Contains(ErrRevokeReasonMissing.Error(), "xoá") {
		t.Error("thông báo thiếu lý do thu hồi nói về 'xoá'")
	}
}
