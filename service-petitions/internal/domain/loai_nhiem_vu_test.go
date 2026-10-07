package domain

import "testing"

// RequiresDirective is derived from the CODE alone — the label, the provenance and the tier change
// nothing, so a commune relabelling `theo-van-ban` or adding a look-alike code cannot move the flag.
func TestRequiresDirectiveIsDerivedFromTheCodeOnly(t *testing.T) {
	for _, tc := range []struct {
		l    LoaiNhiemVu
		want bool
	}{
		{LoaiNhiemVu{Ma: "theo-van-ban", Nhan: "Theo văn bản", Nguon: NguonHeThong, MaNguonReNhanh: true}, true},
		{LoaiNhiemVu{Ma: "theo-van-ban", Nhan: "Nhãn đổi bởi xã", Nguon: NguonDonVi}, true},
		{LoaiNhiemVu{Ma: "co-ban", Nhan: "Theo văn bản"}, false},
		{LoaiNhiemVu{Ma: "theo-van-ban-2"}, false},
		{LoaiNhiemVu{Ma: "Theo-Van-Ban"}, false},
		{LoaiNhiemVu{}, false},
	} {
		if got := tc.l.RequiresDirective(); got != tc.want {
			t.Errorf("%q / %q: RequiresDirective = %v, muốn %v", tc.l.Ma, tc.l.Nhan, got, tc.want)
		}
	}
}
