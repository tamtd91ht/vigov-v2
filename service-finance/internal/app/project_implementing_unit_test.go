package app

// `du_an.implementing_unit` (migration 0016) on the write path, over the REAL store and the fake
// driver of driver_gia_du_an_test.go: written on create and on edit, NULL when blank, audited with its
// before/after when it moves, a no-op when it does not, refused before any transaction when malformed.

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-finance/internal/domain"
)

func implementingUnitDelta(t *testing.T, k *khoDAGia) map[string]any {
	t.Helper()
	vet := k.cau("INSERT INTO audit_log")
	if len(vet) != 1 {
		t.Fatalf("%d audit entries, want 1", len(vet))
	}
	var delta map[string]any
	if err := json.Unmarshal(vet[0].args[7].([]byte), &delta); err != nil {
		t.Fatalf("delta: %v", err)
	}
	return delta
}

func TestCreateProjectWritesImplementingUnitTrimmedAndAuditsIt(t *testing.T) {
	k := khoDuAnSan()
	uc, ctx := dungUseCaseDuAn(t, k)
	yc := themDuAnHopLe()
	yc.ImplementingUnit = "  Công ty Xây dựng Thành Long  "
	kq, err := uc.Them(ctx, yc, canBo)
	if err != nil {
		t.Fatalf("Them: %v", err)
	}
	ins := k.cau("INSERT INTO du_an")
	if len(ins) != 1 || !strings.Contains(ins[0].sql, "implementing_unit") {
		t.Fatalf("INSERT = %v", ins)
	}
	// $15 — args[14], the commune being $1.
	if got := ins[0].args[14]; got != "Công ty Xây dựng Thành Long" {
		t.Fatalf("implementing_unit written as %v", got)
	}
	if kq.DuAn.ImplementingUnit != "Công ty Xây dựng Thành Long" {
		t.Fatalf("returned %q", kq.DuAn.ImplementingUnit)
	}
	if sau := implementingUnitDelta(t, k)["sau"].(map[string]any); sau["implementing_unit"] != "Công ty Xây dựng Thành Long" {
		t.Fatalf("delta.sau = %v", sau)
	}
}

func TestCreateProjectBlankImplementingUnitIsNULL(t *testing.T) {
	// 0016's CHECK refuses '' — "not named" is NULL, and the delta does not invent the field.
	k := khoDuAnSan()
	uc, ctx := dungUseCaseDuAn(t, k)
	yc := themDuAnHopLe()
	yc.ImplementingUnit = "   "
	if _, err := uc.Them(ctx, yc, canBo); err != nil {
		t.Fatalf("Them: %v", err)
	}
	if got := k.cau("INSERT INTO du_an")[0].args[14]; got != nil {
		t.Fatalf("a blank unit must be NULL, got %v", got)
	}
	if _, has := implementingUnitDelta(t, k)["sau"].(map[string]any)["implementing_unit"]; has {
		t.Fatal("a blank unit must not appear in the delta")
	}
}

func TestCreateProjectImplementingUnitRefusedBeforeTx(t *testing.T) {
	for name, c := range map[string]struct {
		unit string
		want error
	}{
		"256 characters":    {strings.Repeat("ạ", domain.ImplementingUnitMax+1), domain.ErrImplementingUnitTooLong},
		"control character": {"Công ty\nABC", domain.ErrImplementingUnitInvalid},
	} {
		t.Run(name, func(t *testing.T) {
			k := khoDuAnSan()
			uc, ctx := dungUseCaseDuAn(t, k)
			yc := themDuAnHopLe()
			yc.ImplementingUnit = c.unit
			if _, err := uc.Them(ctx, yc, canBo); !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
			if k.batDau != 0 {
				t.Fatal("a refused shape opened a transaction")
			}
		})
	}
}

func TestEditProjectImplementingUnitMovesWithBeforeAndAfter(t *testing.T) {
	k := khoDuAnSan()
	k.hang = hangDuAnSan()
	k.hang.implementingUnit = "Công ty cũ"
	uc, ctx := dungUseCaseDuAn(t, k)

	unit := "Công ty mới"
	kq, err := uc.Sua(ctx, k.hang.id, YeuCauSuaDuAn{ImplementingUnit: &unit}, canBo)
	if err != nil {
		t.Fatalf("Sua: %v", err)
	}
	upd := k.cau("UPDATE du_an")
	if len(upd) != 1 || !strings.Contains(upd[0].sql, "implementing_unit = $13") {
		t.Fatalf("UPDATE = %v", upd)
	}
	if got := upd[0].args[12]; got != "Công ty mới" {
		t.Fatalf("$13 = %v", got)
	}
	if kq.DuAn.ImplementingUnit != "Công ty mới" {
		t.Fatalf("returned %q", kq.DuAn.ImplementingUnit)
	}
	delta := implementingUnitDelta(t, k)
	if delta["truoc"].(map[string]any)["implementing_unit"] != "Công ty cũ" ||
		delta["sau"].(map[string]any)["implementing_unit"] != "Công ty mới" {
		t.Fatalf("delta = %v", delta)
	}
}

func TestEditProjectOtherFieldKeepsImplementingUnit(t *testing.T) {
	// The UPDATE writes every editable column; an edit not naming the unit must write back what it read.
	k := khoDuAnSan()
	k.hang = hangDuAnSan()
	k.hang.implementingUnit = "Công ty giữ nguyên"
	uc, ctx := dungUseCaseDuAn(t, k)
	ten := "Tên mới"
	if _, err := uc.Sua(ctx, k.hang.id, YeuCauSuaDuAn{Ten: &ten}, canBo); err != nil {
		t.Fatalf("Sua: %v", err)
	}
	if got := k.cau("UPDATE du_an")[0].args[12]; got != "Công ty giữ nguyên" {
		t.Fatalf("an untouched unit was written as %v", got)
	}
	if _, has := implementingUnitDelta(t, k)["sau"].(map[string]any)["implementing_unit"]; has {
		t.Fatal("an unchanged unit must not appear in the delta")
	}
}

func TestEditProjectClearImplementingUnitWritesNULL(t *testing.T) {
	k := khoDuAnSan()
	k.hang = hangDuAnSan()
	k.hang.implementingUnit = "Công ty cũ"
	uc, ctx := dungUseCaseDuAn(t, k)
	blank := " "
	if _, err := uc.Sua(ctx, k.hang.id, YeuCauSuaDuAn{ImplementingUnit: &blank}, canBo); err != nil {
		t.Fatalf("Sua: %v", err)
	}
	if got := k.cau("UPDATE du_an")[0].args[12]; got != nil {
		t.Fatalf("a cleared unit must be NULL, got %v", got)
	}
}

func TestEditProjectSameImplementingUnitIsNoOp(t *testing.T) {
	k := khoDuAnSan()
	k.hang = hangDuAnSan()
	k.hang.implementingUnit = "Công ty cũ"
	uc, ctx := dungUseCaseDuAn(t, k)
	same := " Công ty cũ "
	if _, err := uc.Sua(ctx, k.hang.id, YeuCauSuaDuAn{ImplementingUnit: &same}, canBo); err != nil {
		t.Fatalf("Sua: %v", err)
	}
	if k.coCau("UPDATE du_an") || k.coCau("INSERT INTO audit_log") {
		t.Fatal("nothing moved, yet a row or an entry was written")
	}
}
