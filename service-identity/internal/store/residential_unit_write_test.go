package store

import (
	"errors"
	"testing"
)

// PostgreSQL names the plain table's own constraint in the error, and named the partition's copy
// while the table was hash-partitioned (before ADR 0096); the substring match must recognise both,
// and must never flatten an unknown failure into a business refusal.
func TestTranslateResidentialUnitWrite(t *testing.T) {
	for raw, want := range map[string]error{
		`duplicate key value violates unique constraint "thon_to_dan_pho_p07_tenant_id_ma_key"`:                                 ErrResidentialUnitCodeTaken,
		`insert or update on table "thon_to_dan_pho_p02" violates foreign key constraint "thon_to_dan_pho_tenant_id_loai_fkey"`: ErrResidentialUnitTypeNotFound,
		`insert or update on table "thon_to_dan_pho_p02" violates foreign key constraint "thon_to_dan_pho_head_staff_fkey"`:     ErrHeadStaffNotFound,
	} {
		if got := translateResidentialUnitWrite("chèn", errors.New(raw)); !errors.Is(got, want) {
			t.Errorf("%q → %v, muốn %v", raw, got, want)
		}
	}
	other := errors.New("connection reset")
	got := translateResidentialUnitWrite("chèn", other)
	if !errors.Is(got, other) || errors.Is(got, ErrResidentialUnitCodeTaken) {
		t.Errorf("lỗi lạ phải được bọc và chuyển tiếp nguyên vẹn: %v", got)
	}
}
