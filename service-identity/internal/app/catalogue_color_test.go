package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-identity/internal/domain"
)

// The display colour of both identity catalogues (migration 0026, ADR 0079 lô 2 Q1 #9), over the real
// store and the fake driver of driver_gia_danh_muc_test.go.

func strPtr(s string) *string { return &s }

// CREATE: the colour is bound as $8, lower-cased, and reaches the audit entry.
func TestAddCatalogueRowStoresColorLowerCase(t *testing.T) {
	for i := range dungCatalogue(t, &khoDanhMucGia{}) {
		k := &khoDanhMucGia{hang: hangMau()}
		c := dungCatalogue(t, k)[i]
		m, err := c.them(ctxDanhMuc(), YeuCauThemDanhMuc{Ma: "moi", Nhan: "Mới", Color: strPtr("#1F6FEB")})
		if err != nil {
			t.Fatalf("%s: %v", c.ten, err)
		}
		if m.Color != "#1f6feb" {
			t.Errorf("%s: màu trả về = %q, muốn #1f6feb", c.ten, m.Color)
		}
		chen := k.cau("INSERT INTO " + c.bang + " ")
		if len(chen) != 1 || chen[0].args[7] != "#1f6feb" {
			t.Errorf("%s: INSERT tham số = %v", c.ten, chen[0].args)
		}
		delta := motVetDanhMuc(t, c.ten, k, c.hanhViThem, "moi")
		if sau, _ := delta["sau"].(map[string]any); sau["color"] != "#1f6feb" {
			t.Errorf("%s: vết thiếu màu: %v", c.ten, delta)
		}
	}
}

// CREATE WITHOUT A COLOUR binds NULL, never "".
func TestAddCatalogueRowWithoutColorIsNull(t *testing.T) {
	for i := range dungCatalogue(t, &khoDanhMucGia{}) {
		k := &khoDanhMucGia{hang: hangMau()}
		c := dungCatalogue(t, k)[i]
		if _, err := c.them(ctxDanhMuc(), YeuCauThemDanhMuc{Ma: "moi", Nhan: "Mới"}); err != nil {
			t.Fatalf("%s: %v", c.ten, err)
		}
		if a := k.cau("INSERT INTO " + c.bang + " ")[0].args; a[7] != nil {
			t.Errorf("%s: color = %v, muốn NULL", c.ten, a[7])
		}
	}
}

// A MALFORMED COLOUR is refused before any transaction, on create and on edit.
func TestCatalogueColorMalformedRefused(t *testing.T) {
	for _, bad := range []string{"", "1f6feb", "#1f6fe", "#1f6febff", "#gggggg", "red", "rgb(0,0,0)"} {
		for i := range dungCatalogue(t, &khoDanhMucGia{}) {
			k := &khoDanhMucGia{hang: hangMau()}
			c := dungCatalogue(t, k)[i]
			if _, err := c.them(ctxDanhMuc(), YeuCauThemDanhMuc{Ma: "moi", Nhan: "Mới", Color: strPtr(bad)}); !errors.Is(err, domain.ErrCatalogueColorInvalid) {
				t.Errorf("%s thêm %q: lỗi = %v", c.ten, bad, err)
			}
			if _, err := c.sua(ctxDanhMuc(), "m-t1", YeuCauSuaDanhMuc{Color: &CatalogueColorChange{Color: strPtr(bad)}}); !errors.Is(err, domain.ErrCatalogueColorInvalid) {
				t.Errorf("%s sửa %q: lỗi = %v", c.ten, bad, err)
			}
			if k.batDau != 0 {
				t.Errorf("%s %q: mở giao dịch", c.ten, bad)
			}
		}
	}
}

// EDIT: set on a TIER-3 row (presentation only — Q1 #9), then clear; the trail carries before/after.
func TestEditCatalogueColorSetAndClearOnEveryTier(t *testing.T) {
	for i := range dungCatalogue(t, &khoDanhMucGia{}) {
		k := &khoDanhMucGia{hang: hangMau()}
		c := dungCatalogue(t, k)[i]
		m, err := c.sua(ctxDanhMuc(), "m-t3", YeuCauSuaDanhMuc{Color: &CatalogueColorChange{Color: strPtr("#ABCDEF")}})
		if err != nil {
			t.Fatalf("%s: đặt màu dòng tầng 3: %v", c.ten, err)
		}
		if m.Color != "#abcdef" {
			t.Errorf("%s: màu = %q", c.ten, m.Color)
		}
		upd := k.cau("UPDATE " + c.bang + " SET nhan")
		if len(upd) != 1 || !strings.Contains(upd[0].sql, "color = $7") || upd[0].args[6] != "#abcdef" {
			t.Fatalf("%s: UPDATE = %v", c.ten, upd)
		}
		delta := motVetDanhMuc(t, c.ten, k, c.hanhViSua, "re-nhanh")
		truoc, _ := delta["truoc"].(map[string]any)
		sau, _ := delta["sau"].(map[string]any)
		if _, ok := truoc["color"]; !ok || truoc["color"] != nil || sau["color"] != "#abcdef" {
			t.Errorf("%s: delta = %v, muốn color null → #abcdef", c.ten, delta)
		}
	}

	// Clear: a row holding a colour, a change with Color nil.
	for i := range dungCatalogue(t, &khoDanhMucGia{}) {
		hang := hangMau()
		for j := range hang {
			if hang[j].id == "m-t1" {
				hang[j].color = "#112233"
			}
		}
		k := &khoDanhMucGia{hang: hang}
		c := dungCatalogue(t, k)[i]
		m, err := c.sua(ctxDanhMuc(), "m-t1", YeuCauSuaDanhMuc{Color: &CatalogueColorChange{}})
		if err != nil || m.Color != "" {
			t.Fatalf("%s: xoá màu: %v, màu %q", c.ten, err, m.Color)
		}
		upd := k.cau("UPDATE " + c.bang + " SET nhan")
		if len(upd) != 1 || upd[0].args[6] != nil {
			t.Errorf("%s: xoá màu phải ghi NULL: %v", c.ten, upd)
		}
	}
}

// EDIT NOT MENTIONING THE COLOUR leaves it, and the same colour again writes nothing.
func TestEditCatalogueColorUnchangedWritesNothing(t *testing.T) {
	for i := range dungCatalogue(t, &khoDanhMucGia{}) {
		hang := hangMau()
		for j := range hang {
			if hang[j].id == "m-t1" {
				hang[j].color = "#112233"
			}
		}
		k := &khoDanhMucGia{hang: hang}
		c := dungCatalogue(t, k)[i]
		if _, err := c.sua(ctxDanhMuc(), "m-t1", YeuCauSuaDanhMuc{Color: &CatalogueColorChange{Color: strPtr("#112233")}}); err != nil {
			t.Fatal(err)
		}
		if len(k.cau("UPDATE "+c.bang+" SET nhan")) != 0 || len(k.cau("audit_log")) != 0 {
			t.Errorf("%s: cùng màu mà vẫn ghi", c.ten)
		}
	}
}
