package domain

import "testing"

func TestNormaliseHost(t *testing.T) {
	// Host arrives from the client, so every spelling that reaches the edge must land on the
	// same commune. A miss here is a whole commune returning 404.
	cases := []struct {
		name string
		in   string
		want string
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
		t.Run(c.name, func(t *testing.T) {
			got, err := NormaliseHost(c.in)
			if err != nil {
				t.Fatalf("NormaliseHost(%q) lỗi: %v", c.in, err)
			}
			if got != c.want {
				t.Errorf("NormaliseHost(%q) = %q, muốn %q", c.in, got, c.want)
			}
		})
	}
}

func TestNormaliseHostRefuses(t *testing.T) {
	// A Host carrying a scheme or a path is not a Host — accepting it would mean the caller
	// controls what we look up.
	for _, in := range []string{"", "   ", "https://tanphu.vigov.vn", "tanphu.vigov.vn/admin"} {
		if _, err := NormaliseHost(in); err == nil {
			t.Errorf("NormaliseHost(%q) phải lỗi, nhưng không", in)
		}
	}
}

func TestTenantValidate(t *testing.T) {
	const ulid = "01JD8ZQK9M3NPXR7TVWYB2C4EF" // 26 ký tự

	if err := (Tenant{ID: ulid, Name: "Xã Thăng Bình"}).Validate(); err != nil {
		t.Fatalf("xã hợp lệ bị từ chối: %v", err)
	}

	// A meaningful id is what rule 1 invariant 2 exists to prevent: the first merger would
	// force rewriting foreign keys across archival records.
	for _, bad := range []Tenant{
		{ID: "thang-binh", Name: "Xã Thăng Bình"}, // mã hành chính, không phải ULID
		{ID: "", Name: "Xã Thăng Bình"},
		{ID: ulid, Name: ""},
		{ID: ulid, Name: "   "},
	} {
		if err := bad.Validate(); err == nil {
			t.Errorf("Tenant{ID:%q, Name:%q} phải lỗi, nhưng không", bad.ID, bad.Name)
		}
	}
}

func TestValidateAppID(t *testing.T) {
	for _, id := range []string{"", " ", " 123", "123 ", "\t123"} {
		if err := ValidateAppID(id); err == nil {
			t.Errorf("ValidateAppID(%q) nhận — app_id không được chuẩn hoá ngầm", id)
		}
	}
	if err := ValidateAppID("1234567890"); err != nil {
		t.Errorf("ValidateAppID hợp lệ bị từ chối: %v", err)
	}
}
