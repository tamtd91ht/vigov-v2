package app

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// The display colour of `loai_nhiem_vu` (migration 0033, ADR 0079 row 5 and lô 2 Q1 #9), over the real
// store and the recording driver — the same pattern as service-identity's catalogue_color_test.go.

func colorPtrLoaiNhiemVu(s string) *string { return &s }

func colorTrailLoaiNhiemVu(t *testing.T, k *khoLoaiNhiemVuGia) map[string]map[string]any {
	t.Helper()
	ins := k.cau("INSERT INTO audit_log")
	if len(ins) != 1 {
		t.Fatalf("audit entries = %d, want 1", len(ins))
	}
	// Top-level values that are not objects (finance records `auto_code`) are skipped.
	var raw map[string]any
	if err := json.Unmarshal(ins[0].args[7].([]byte), &raw); err != nil {
		t.Fatal(err)
	}
	d := map[string]map[string]any{}
	for k, v := range raw {
		if m, ok := v.(map[string]any); ok {
			d[k] = m
		}
	}
	return d
}

// CREATE: bound as $8, lower-cased, and in the trail.
func TestColorCreateStoresLowerCaseLoaiNhiemVu(t *testing.T) {
	k := &khoLoaiNhiemVuGia{}
	uc, ctx := dungUseCaseLoaiNhiemVu(t, k)
	yc := themMauLoaiNhiemVu()
	yc.Color = colorPtrLoaiNhiemVu("#1F6FEB")
	m, err := uc.Them(ctx, yc, nguoiThuLoaiNhiemVu())
	if err != nil {
		t.Fatal(err)
	}
	if m.Color != "#1f6feb" {
		t.Errorf("returned colour = %q", m.Color)
	}
	chen := k.cau("INSERT INTO loai_nhiem_vu")
	if len(chen) != 1 || chen[0].args[7] != "#1f6feb" {
		t.Fatalf("INSERT args = %v", chen)
	}
	if d := colorTrailLoaiNhiemVu(t, k); d["sau"]["color"] != "#1f6feb" {
		t.Errorf("trail = %v", d)
	}
}

// CREATE WITHOUT A COLOUR binds NULL, never "" (the CHECK refuses "").
func TestColorCreateWithoutIsNullLoaiNhiemVu(t *testing.T) {
	k := &khoLoaiNhiemVuGia{}
	uc, ctx := dungUseCaseLoaiNhiemVu(t, k)
	if _, err := uc.Them(ctx, themMauLoaiNhiemVu(), nguoiThuLoaiNhiemVu()); err != nil {
		t.Fatal(err)
	}
	if a := k.cau("INSERT INTO loai_nhiem_vu")[0].args; a[7] != nil {
		t.Errorf("color = %v, want NULL", a[7])
	}
}

// A MALFORMED COLOUR is refused before any transaction, on create and on edit.
func TestColorMalformedRefusedLoaiNhiemVu(t *testing.T) {
	for _, bad := range []string{"", "1f6feb", "#1f6fe", "#1f6febff", "#gggggg", "red", "rgb(0,0,0)"} {
		k := &khoLoaiNhiemVuGia{hang: dongTang1LoaiNhiemVu()}
		uc, ctx := dungUseCaseLoaiNhiemVu(t, k)
		yc := themMauLoaiNhiemVu()
		yc.Color = colorPtrLoaiNhiemVu(bad)
		if _, err := uc.Them(ctx, yc, nguoiThuLoaiNhiemVu()); !errors.Is(err, domain.ErrCatalogueColorInvalid) {
			t.Errorf("create %q: err = %v", bad, err)
		}
		if _, err := uc.Sua(ctx, dongTang1LoaiNhiemVu().id, YeuCauSuaLoaiNhiemVu{Color: &CatalogueColorChange{Color: colorPtrLoaiNhiemVu(bad)}}, nguoiThuLoaiNhiemVu()); !errors.Is(err, domain.ErrCatalogueColorInvalid) {
			t.Errorf("edit %q: err = %v", bad, err)
		}
		if k.batDau != 0 {
			t.Errorf("%q: opened a transaction", bad)
		}
	}
}

// EDIT: set on a TIER-3 row (presentation only — Q1 #9), with before/after in the trail; then clear.
func TestColorSetOnTier3AndClearLoaiNhiemVu(t *testing.T) {
	k := &khoLoaiNhiemVuGia{hang: dongTang3LoaiNhiemVu()}
	uc, ctx := dungUseCaseLoaiNhiemVu(t, k)
	m, err := uc.Sua(ctx, dongTang3LoaiNhiemVu().id, YeuCauSuaLoaiNhiemVu{Color: &CatalogueColorChange{Color: colorPtrLoaiNhiemVu("#ABCDEF")}}, nguoiThuLoaiNhiemVu())
	if err != nil {
		t.Fatalf("colour on a tier-3 row: %v", err)
	}
	if m.Color != "#abcdef" {
		t.Errorf("colour = %q", m.Color)
	}
	upd := k.cau("UPDATE loai_nhiem_vu SET nhan")
	if len(upd) != 1 || !strings.Contains(upd[0].sql, "color = $7") || upd[0].args[6] != "#abcdef" {
		t.Fatalf("UPDATE = %v", upd)
	}
	d := colorTrailLoaiNhiemVu(t, k)
	if v, ok := d["truoc"]["color"]; !ok || v != nil || d["sau"]["color"] != "#abcdef" {
		t.Errorf("trail = %v, want color null -> #abcdef", d)
	}

	h := dongTang1LoaiNhiemVu()
	h.color = "#112233"
	k = &khoLoaiNhiemVuGia{hang: h}
	uc, ctx = dungUseCaseLoaiNhiemVu(t, k)
	m, err = uc.Sua(ctx, h.id, YeuCauSuaLoaiNhiemVu{Color: &CatalogueColorChange{}}, nguoiThuLoaiNhiemVu())
	if err != nil || m.Color != "" {
		t.Fatalf("clear: %v, colour %q", err, m.Color)
	}
	if upd := k.cau("UPDATE loai_nhiem_vu SET nhan"); len(upd) != 1 || upd[0].args[6] != nil {
		t.Errorf("clear must write NULL: %v", upd)
	}
}

// THE SAME COLOUR AGAIN, or an edit not naming it, writes nothing about it.
func TestColorUnchangedWritesNothingLoaiNhiemVu(t *testing.T) {
	h := dongTang1LoaiNhiemVu()
	h.color = "#112233"
	k := &khoLoaiNhiemVuGia{hang: h}
	uc, ctx := dungUseCaseLoaiNhiemVu(t, k)
	if _, err := uc.Sua(ctx, h.id, YeuCauSuaLoaiNhiemVu{Color: &CatalogueColorChange{Color: colorPtrLoaiNhiemVu("#112233")}}, nguoiThuLoaiNhiemVu()); err != nil {
		t.Fatal(err)
	}
	if k.coCau("UPDATE loai_nhiem_vu SET nhan") || k.coCau("audit_log") {
		t.Error("the same colour wrote something")
	}
}

// THE TWO BOOLEANS SURVIVE AN EDIT OF ANOTHER FIELD. A Scan reading la_mac_dinh/dang_dung in the
// wrong order writes them back swapped on every PATCH — the defect petitions carried until
// 08/10/2026 and finance until 30/09/2026. An in-use, non-default row must stay exactly that.
func TestEditKeepsInUseAndDefaultFlagsLoaiNhiemVu(t *testing.T) {
	h := dongTang1LoaiNhiemVu() // dangDung true, macDinh false
	k := &khoLoaiNhiemVuGia{hang: h}
	uc, ctx := dungUseCaseLoaiNhiemVu(t, k)
	m, err := uc.Sua(ctx, h.id, YeuCauSuaLoaiNhiemVu{Color: &CatalogueColorChange{Color: colorPtrLoaiNhiemVu("#000000")}}, nguoiThuLoaiNhiemVu())
	if err != nil {
		t.Fatal(err)
	}
	if !m.DangDung || m.LaMacDinh {
		t.Errorf("after edit: dang_dung=%v la_mac_dinh=%v, want true/false", m.DangDung, m.LaMacDinh)
	}
	upd := k.cau("UPDATE loai_nhiem_vu SET nhan")
	if len(upd) != 1 || upd[0].args[4] != true || upd[0].args[5] != false {
		t.Errorf("UPDATE dang_dung=$5 %v la_mac_dinh=$6 %v, want true/false", upd[0].args[4], upd[0].args[5])
	}
}
