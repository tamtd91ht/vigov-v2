package app

import (
	"strings"
	"testing"
)

// THE SOFT DELETE AND THE EDIT ARE BOUNDED ON (commune, id, live).
//
// `AND deleted_at IS NULL` on the soft-delete UPDATE is what makes a second delete a 404 instead of a
// REWRITE of who deleted the row and why (rule 7, forbidden #5 — editing a historical record). The
// fake driver does not evaluate an UPDATE's WHERE, so without this pin that clause could be dropped
// with every other catalogue test still green.
//
// MUTATION THAT MUST TURN THIS RED: drop `AND deleted_at IS NULL` from xoaMemDanhMuc.
func TestXoaVaSuaDanhMucUpdateBoTrenDongSongCuaXa(t *testing.T) {
	for i := range dungCatalogue(t, &khoDanhMucGia{}) {
		k := &khoDanhMucGia{hang: hangMau()}
		c := dungCatalogue(t, k)[i]
		if err := c.xoa(ctxDanhMuc(), "m-t1", "nhập nhầm"); err != nil {
			t.Fatalf("%s: xoá: %v", c.ten, err)
		}
		if _, err := c.sua(ctxDanhMuc(), "m-t2", YeuCauSuaDanhMuc{Nhan: chuoiDM("Nhãn khác")}); err != nil {
			t.Fatalf("%s: sửa: %v", c.ten, err)
		}
		for _, tu := range []string{"SET deleted_at = now()", "SET nhan = $3"} {
			ds := k.cau("UPDATE " + c.bang + " " + tu)
			if len(ds) != 1 {
				t.Fatalf("%s: có %d câu %q, muốn 1", c.ten, len(ds), tu)
			}
			gon := strings.Join(strings.Fields(ds[0].sql), " ")
			if !strings.HasSuffix(gon, "WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL") {
				t.Errorf("%s: %q không bó theo (xã, id, dòng sống): %s", c.ten, tu, gon)
			}
		}
	}
}
