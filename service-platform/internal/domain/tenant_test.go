package domain

import "testing"

func TestNormaliseHost(t *testing.T) {
	// Host arrives from the client, so every spelling that reaches the edge must land on the
	// same commune. A miss here is a whole commune returning 404.
	cases := []struct {
		ten  string
		vao  string
		muon string
	}{
		{"đã chuẩn", "tanphu.vigov.vn", "tanphu.vigov.vn"},
		{"chữ hoa", "TanPhu.ViGov.VN", "tanphu.vigov.vn"},
		{"có cổng", "tanphu.vigov.vn:443", "tanphu.vigov.vn"},
		{"hoa và cổng", "TanPhu.ViGov.vn:8080", "tanphu.vigov.vn"},
		{"khoảng trắng thừa", "  tanphu.vigov.vn  ", "tanphu.vigov.vn"},
		{"localhost có cổng", "localhost:3000", "localhost"},
		{"IPv6 có cổng", "[::1]:8080", "[::1]"},
	}
	for _, c := range cases {
		t.Run(c.ten, func(t *testing.T) {
			got, err := NormaliseHost(c.vao)
			if err != nil {
				t.Fatalf("NormaliseHost(%q) lỗi: %v", c.vao, err)
			}
			if got != c.muon {
				t.Errorf("NormaliseHost(%q) = %q, muốn %q", c.vao, got, c.muon)
			}
		})
	}
}

func TestNormaliseHostTuChoi(t *testing.T) {
	// A Host carrying a scheme or a path is not a Host — accepting it would mean the caller
	// controls what we look up.
	for _, vao := range []string{"", "   ", "https://tanphu.vigov.vn", "tanphu.vigov.vn/admin"} {
		if _, err := NormaliseHost(vao); err == nil {
			t.Errorf("NormaliseHost(%q) phải lỗi, nhưng không", vao)
		}
	}
}

func TestTenantValidate(t *testing.T) {
	const ulid = "01JD8ZQK9M3NPXR7TVWYB2C4EF" // 26 ký tự

	if err := (Tenant{ID: ulid, Ten: "Xã Thăng Bình"}).Validate(); err != nil {
		t.Fatalf("xã hợp lệ bị từ chối: %v", err)
	}

	// A meaningful id is what rule 1 invariant 2 exists to prevent: the first merger would
	// force rewriting foreign keys across archival records.
	for _, xau := range []Tenant{
		{ID: "thang-binh", Ten: "Xã Thăng Bình"}, // mã hành chính, không phải ULID
		{ID: "", Ten: "Xã Thăng Bình"},
		{ID: ulid, Ten: ""},
		{ID: ulid, Ten: "   "},
	} {
		if err := xau.Validate(); err == nil {
			t.Errorf("Tenant{ID:%q, Ten:%q} phải lỗi, nhưng không", xau.ID, xau.Ten)
		}
	}
}
