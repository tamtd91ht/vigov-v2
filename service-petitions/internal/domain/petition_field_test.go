package domain

import (
	"errors"
	"strings"
	"testing"
)

// The merge of tier 1 (platform) with tier 2 (the commune's overrides), and the edit rules.

func tier1Sample() []FieldDefault {
	return []FieldDefault{
		{Code: "giao-thong", DefaultLabel: "Giao thông", SortOrder: 3, Active: true},
		{Code: "rac-thai", DefaultLabel: "Rác thải – Vệ sinh môi trường", SortOrder: 2, Icon: "Trash2", Tone: "green", Active: true},
		{Code: "can-bo", DefaultLabel: "Thái độ cán bộ", SortOrder: 11, Active: true},
		{Code: "ma-cu", DefaultLabel: "Mã đã ngừng", SortOrder: 13, Active: false},
	}
}

func codesOf(vs []PetitionFieldView) string {
	var out []string
	for _, v := range vs {
		out = append(out, v.Code)
	}
	return strings.Join(out, ",")
}

func TestMergeWithNoOverridesIsTier1InPlatformOrder(t *testing.T) {
	got := MergePetitionFields(tier1Sample(), nil)
	if codesOf(got) != "rac-thai,giao-thong,can-bo,ma-cu" {
		t.Fatalf("order = %s", codesOf(got))
	}
	for _, v := range got {
		if !v.Enabled || v.Customised || v.Label != v.DefaultLabel {
			t.Errorf("no override yet the view is customised: %+v", v)
		}
	}
}

func TestMergeAppliesLabelOrderAndSwitch(t *testing.T) {
	got := MergePetitionFields(tier1Sample(), []NhanLinhVuc{
		{Ma: "giao-thong", Nhan: "Giao thông – Đường làng", SortOrder: 1, Enabled: true},
		{Ma: "rac-thai", Enabled: false}, // inherit label and order, switched off
	})
	if codesOf(got) != "giao-thong,rac-thai,can-bo,ma-cu" {
		t.Fatalf("commune order not applied: %s", codesOf(got))
	}
	if got[0].Label != "Giao thông – Đường làng" || got[0].Order != 1 || got[0].DefaultOrder != 3 || !got[0].Customised {
		t.Errorf("label/order override: %+v", got[0])
	}
	if got[1].Enabled || got[1].Label != "Rác thải – Vệ sinh môi trường" || !got[1].Customised {
		t.Errorf("switch-off: %+v", got[1])
	}
}

func TestMergeTieOnPositionFallsBackToPlatformOrderThenCode(t *testing.T) {
	// The commune moves the staff-conduct code to 2 — the same number as rac-thai's platform order.
	got := MergePetitionFields(tier1Sample(), []NhanLinhVuc{{Ma: "can-bo", SortOrder: 2, Enabled: true}})
	if codesOf(got) != "rac-thai,can-bo,giao-thong,ma-cu" {
		t.Fatalf("order = %s", codesOf(got))
	}
}

func TestMergeDropsAnOverrideForAnUnknownCode(t *testing.T) {
	got := MergePetitionFields(tier1Sample(), []NhanLinhVuc{{Ma: "ve-sinh-moi-truong", Nhan: "VSMT", Enabled: true}})
	if strings.Contains(codesOf(got), "ve-sinh-moi-truong") {
		t.Fatal("a code tier 1 does not know was shown — the raw-code defect of ADR 0026 §2")
	}
}

func TestCitizenCatalogueHidesDisabledRetiredAndStaffConduct(t *testing.T) {
	merged := MergePetitionFields(tier1Sample(), []NhanLinhVuc{{Ma: "giao-thong", Enabled: false}})
	if got := codesOf(CitizenCatalogue(merged)); got != "rac-thai" {
		t.Fatalf("citizen catalogue = %s, want only rac-thai", got)
	}
	// The merged list itself still holds all four: switching off never filters a read path.
	if codesOf(merged) != "rac-thai,giao-thong,can-bo,ma-cu" {
		t.Errorf("merge filtered something: %s", codesOf(merged))
	}
}

func TestStaffConductFieldIsNeverOfferedEvenWhenEnabled(t *testing.T) {
	v := PetitionFieldView{Code: LinhVucHanChe, Active: true, Enabled: true}
	if v.OfferedToCitizens() {
		t.Fatal("the staff-conduct field offered before its leaders-only flow exists (ADR 0050 point 10)")
	}
}

func TestApplyFieldEdit(t *testing.T) {
	d := tier1Sample()[1] // rac-thai, default order 2
	cur := NhanLinhVuc{Ma: "rac-thai", Nhan: "Rác", SortOrder: 7, Enabled: true}
	s := func(v string) *string { return &v }
	n := func(v int) *int { return &v }
	b := func(v bool) *bool { return &v }

	next, err := ApplyFieldEdit(d, cur, PetitionFieldEdit{Label: s("  Rác thải xã A  "), Enabled: b(false)})
	if err != nil || next.Nhan != "Rác thải xã A" || next.SortOrder != 7 || next.Enabled {
		t.Errorf("partial edit: %+v %v", next, err)
	}
	next, _ = ApplyFieldEdit(d, cur, PetitionFieldEdit{Label: s("Rác thải – Vệ sinh môi trường"), Order: n(2)})
	if next.Nhan != "" || next.SortOrder != 0 {
		t.Errorf("sending the defaults must store inherit (\"\"/0): %+v", next)
	}
	for name, e := range map[string]PetitionFieldEdit{
		"empty":       {},
		"blank label": {Label: s(" ")},
		"long label":  {Label: s(strings.Repeat("a", 101))},
		"order 0":     {Order: n(0)},
		"order big":   {Order: n(10000)},
	} {
		if _, err := ApplyFieldEdit(d, cur, e); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if _, err := ApplyFieldEdit(d, cur, PetitionFieldEdit{}); !errors.Is(err, ErrFieldEditEmpty) {
		t.Errorf("empty edit err = %v", err)
	}
}
