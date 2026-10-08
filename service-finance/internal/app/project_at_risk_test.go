package app

// `du_an.at_risk` (migration 0018, ADR 0080 #2) on the edit path, over the REAL store and the fake
// driver of driver_gia_du_an_test.go: ticked and unticked with its before/after in the SAME transaction
// as the UPDATE, written back unchanged by an edit that does not name it, and a no-op — no row, no entry
// — when the PATCH sends the value it already has (what keeps PATCH's idem.KhongCan honest).

import (
	"strings"
	"testing"
)

func TestEditProjectTicksAtRiskWithBeforeAndAfter(t *testing.T) {
	for name, c := range map[string]struct{ from, to bool }{
		"tick":   {false, true},
		"untick": {true, false},
	} {
		t.Run(name, func(t *testing.T) {
			k := khoDuAnSan()
			k.hang = hangDuAnSan()
			k.hang.atRisk = c.from
			uc, ctx := dungUseCaseDuAn(t, k)

			to := c.to
			kq, err := uc.Sua(ctx, k.hang.id, YeuCauSuaDuAn{AtRisk: &to}, canBo)
			if err != nil {
				t.Fatalf("Sua: %v", err)
			}
			upd := k.cau("UPDATE du_an")
			if len(upd) != 1 || !strings.Contains(upd[0].sql, "at_risk = $14") {
				t.Fatalf("UPDATE = %v", upd)
			}
			if got := upd[0].args[13]; got != c.to {
				t.Fatalf("$14 = %v, want %v", got, c.to)
			}
			if kq.DuAn.AtRisk != c.to {
				t.Fatalf("returned %v", kq.DuAn.AtRisk)
			}
			delta := implementingUnitDelta(t, k) // exactly one audit entry, decoded
			if delta["truoc"].(map[string]any)["at_risk"] != c.from || delta["sau"].(map[string]any)["at_risk"] != c.to {
				t.Fatalf("delta = %v", delta)
			}
			if k.daCommit != 1 || k.batDau != 1 {
				t.Fatalf("transactions: began %d, committed %d — the write and its entry share ONE", k.batDau, k.daCommit)
			}
		})
	}
}

func TestEditProjectSameAtRiskIsNoOp(t *testing.T) {
	k := khoDuAnSan()
	k.hang = hangDuAnSan()
	k.hang.atRisk = true
	uc, ctx := dungUseCaseDuAn(t, k)
	same := true
	kq, err := uc.Sua(ctx, k.hang.id, YeuCauSuaDuAn{AtRisk: &same}, canBo)
	if err != nil {
		t.Fatalf("Sua: %v", err)
	}
	if k.coCau("UPDATE du_an") || k.coCau("INSERT INTO audit_log") {
		t.Fatal("nothing moved, yet a row or an entry was written")
	}
	if !kq.DuAn.AtRisk {
		t.Fatal("the no-op reply must still carry the stored flag")
	}
}

func TestEditProjectOtherFieldKeepsAtRisk(t *testing.T) {
	// The UPDATE writes every editable column; an edit not naming the flag must write back what it read.
	k := khoDuAnSan()
	k.hang = hangDuAnSan()
	k.hang.atRisk = true
	uc, ctx := dungUseCaseDuAn(t, k)
	ten := "Tên mới"
	if _, err := uc.Sua(ctx, k.hang.id, YeuCauSuaDuAn{Ten: &ten}, canBo); err != nil {
		t.Fatalf("Sua: %v", err)
	}
	if got := k.cau("UPDATE du_an")[0].args[13]; got != true {
		t.Fatalf("an untouched flag was written as %v", got)
	}
	if _, has := implementingUnitDelta(t, k)["sau"].(map[string]any)["at_risk"]; has {
		t.Fatal("an unchanged flag must not appear in the delta")
	}
}

func TestEditProjectAtRiskAuditFailureRollsBack(t *testing.T) {
	// Rule 6 invariant 3: the entry fails → the flag change does not survive.
	k := khoDuAnSan()
	k.hang = hangDuAnSan()
	k.loiSau = "INSERT INTO audit_log"
	uc, ctx := dungUseCaseDuAn(t, k)
	to := true
	if _, err := uc.Sua(ctx, k.hang.id, YeuCauSuaDuAn{AtRisk: &to}, canBo); err == nil {
		t.Fatal("the audit entry failed, yet the edit reported success")
	}
	if !k.coCau("UPDATE du_an") || k.daCommit != 0 || k.daRollback != 1 {
		t.Fatalf("update ran %v, committed %d, rolled back %d — want the UPDATE undone with its entry",
			k.coCau("UPDATE du_an"), k.daCommit, k.daRollback)
	}
}
