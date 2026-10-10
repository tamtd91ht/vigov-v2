package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeOrgUnitDeleteReason(t *testing.T) {
	if got, err := NormalizeOrgUnitDeleteReason("  sáp nhập vào Văn phòng  "); err != nil || got != "sáp nhập vào Văn phòng" {
		t.Errorf("= %q, %v", got, err)
	}
	// OPTIONAL ON THE WAY IN, NEVER EMPTY IN THE ROW (owner decision 10/10/2026; rule 7, invariant 1).
	for _, raw := range []string{"", "  \t "} {
		if got, err := NormalizeOrgUnitDeleteReason(raw); err != nil || got != OrgUnitDeleteDefaultReason {
			t.Errorf("%q: = %q, %v — muốn câu cố định OrgUnitDeleteDefaultReason", raw, got, err)
		}
	}
	if n := len([]rune(OrgUnitDeleteDefaultReason)); n == 0 || n > MaxOrgUnitDeleteReason {
		t.Errorf("câu cố định dài %d ký tự — phải khác rỗng và trong trần", n)
	}
	// Counted in runes: 500 accented letters are 1500 bytes and must pass.
	if _, err := NormalizeOrgUnitDeleteReason(strings.Repeat("ệ", MaxOrgUnitDeleteReason)); err != nil {
		t.Errorf("500 ký tự có dấu bị từ chối: %v", err)
	}
	if _, err := NormalizeOrgUnitDeleteReason(strings.Repeat("ệ", MaxOrgUnitDeleteReason+1)); !errors.Is(err, ErrOrgUnitDeleteReasonTooLong) {
		t.Errorf("501 ký tự: lỗi = %v", err)
	}
}

// EVERY KIND COUNTS ON ITS OWN. A kind left out of Any is a kind that lets a delete through.
func TestOrgUnitHoldingsAnyCountsEveryKind(t *testing.T) {
	if (OrgUnitHoldings{}).Any() {
		t.Fatal("không giữ gì mà Any() = true")
	}
	for name, h := range map[string]OrgUnitHoldings{
		"staff":     {Staff: 1},
		"children":  {ChildUnits: 1},
		"petitions": {OpenPetitions: 1},
		"tasks":     {OpenTasks: 1},
		"documents": {OpenIncomingDocuments: 1},
	} {
		if !h.Any() {
			t.Errorf("%s: Any() = false", name)
		}
		if h.Sentence() == "" {
			t.Errorf("%s: câu từ chối rỗng", name)
		}
	}
}

func TestOrgUnitHoldingsSentenceNamesOnlyNonZeroKinds(t *testing.T) {
	got := OrgUnitHoldings{Staff: 3, OpenTasks: 2}.Sentence()
	want := "Bộ phận còn 3 cán bộ, 2 nhiệm vụ chưa hoàn thành — chuyển trước khi xoá."
	if got != want {
		t.Errorf("câu = %q, muốn %q", got, want)
	}
	if (OrgUnitHoldings{}).Sentence() != "" {
		t.Error("không giữ gì mà vẫn có câu từ chối")
	}
}
