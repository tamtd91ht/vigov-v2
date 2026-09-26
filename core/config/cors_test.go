package config

// CITIZEN_CORS_ALLOWED_ORIGINS — which browser origins may read the citizen edge.

import (
	"errors"
	"strings"
	"testing"
)

const nguonDeXuat = "https://h5.zdn.vn,https://zalo.me,https://*.zdn.vn,https://*.zalo.me"

func napCORS(t *testing.T, v string) (Config, error) {
	t.Helper()
	datMoiTruong(t, map[string]string{
		"DATABASE_DSN":                 dsnGia,
		"ENV":                          EnvDev,
		"CITIZEN_CORS_ALLOWED_ORIGINS": v,
	})
	return Load("petitions")
}

func TestNguonCORSKhopTheoBang(t *testing.T) {
	n, err := PhanTichNguonCORS(nguonDeXuat + ",https://Dev.Example.VN:8443")
	if err != nil {
		t.Fatalf("PhanTichNguonCORS lỗi: %v", err)
	}
	for _, tc := range []struct {
		origin string
		muon   bool
		vi     string
	}{
		{"https://h5.zdn.vn", true, "khớp chính xác"},
		{"https://zalo.me", true, "khớp chính xác"},
		{"https://H5.ZDN.VN", true, "hoa thường không phân biệt"},
		{"https://stc.zdn.vn", true, "đại diện một nhãn"},
		{"https://a.b.zdn.vn", true, "đại diện nhiều nhãn"},
		{"https://mini.zalo.me", true, "đại diện zalo.me"},
		{"https://dev.example.vn:8443", true, "khớp chính xác có cổng"},
		{"https://evil.zdn.vn.attacker.com", false, "đuôi giả nằm giữa"},
		{"https://evilzdn.vn", false, "không có dấu chấm trước đuôi"},
		{"https://.zdn.vn", false, "nhãn rỗng"},
		{"https://-x.zdn.vn", false, "nhãn bắt đầu bằng -"},
		{"http://h5.zdn.vn", false, "http"},
		{"http://x.zdn.vn", false, "http qua mẫu đại diện"},
		{"https://h5.zdn.vn:8443", false, "cổng khác origin chính xác"},
		{"https://x.zdn.vn:443", false, "cổng trên mẫu đại diện"},
		{"https://dev.example.vn", false, "thiếu cổng mà mục khai có cổng"},
		{"https://h5.zdn.vn/", false, "dấu / cuối"},
		{"https://x.zdn.vn/", false, "dấu / cuối qua mẫu đại diện"},
		{"", false, "rỗng"},
		{"null", false, "origin null"},
		{"https://zdn.vn", false, "đuôi trơn không có nhãn"},
		{"https://example.com", false, "không liên quan"},
	} {
		if got := n.ChoPhep(tc.origin); got != tc.muon {
			t.Errorf("%s: ChoPhep(%q) = %v, muốn %v", tc.vi, tc.origin, got, tc.muon)
		}
	}
}

func TestNguonCORSTrongLaTat(t *testing.T) {
	for _, v := range []string{"", "  ", " , ,"} {
		cfg, err := napCORS(t, v)
		if err != nil {
			t.Fatalf("%q: Load lỗi: %v", v, err)
		}
		if !cfg.CitizenCORSAllowedOrigins.Rong() || cfg.CitizenCORSAllowedOrigins.ChoPhep("https://h5.zdn.vn") {
			t.Errorf("%q: danh sách trống mà vẫn cho phép", v)
		}
	}
}

func TestNguonCORSDeXuatNapDuoc(t *testing.T) {
	cfg, err := napCORS(t, nguonDeXuat)
	if err != nil {
		t.Fatalf("Load lỗi với giá trị đề xuất: %v", err)
	}
	if len(cfg.CitizenCORSAllowedOrigins) != 4 {
		t.Errorf("số mục = %d, muốn 4", len(cfg.CitizenCORSAllowedOrigins))
	}
}

func TestNguonCORSMucHongThiLoadTuChoiVaGoiTen(t *testing.T) {
	for _, tc := range []struct{ gt, muc string }{
		{"*", "mục thứ 1"},
		{"https://h5.zdn.vn, *", "mục thứ 2"},
		{"http://h5.zdn.vn", "mục thứ 1"},
		{"https://h5.zdn.vn,http://localhost:3000", "mục thứ 2"},
		{"h5.zdn.vn", "mục thứ 1"},
		{"https://h5.zdn.vn/", "mục thứ 1"},
		{"https://h5.zdn.vn/app", "mục thứ 1"},
		{"https://*", "mục thứ 1"},
		{"https://*.vn", "mục thứ 1"},
		{"https://*.*.zdn.vn", "mục thứ 1"},
		{"https://h5*.zdn.vn", "mục thứ 1"},
		{"https://*.zdn.vn:443", "mục thứ 1"},
		{"https://h5.zdn.vn:0", "mục thứ 1"},
		{"https://h5.zdn.vn:abc", "mục thứ 1"},
		{"https://", "mục thứ 1"},
		{"https://zalo.me,,https://user@h5.zdn.vn", "mục thứ 3"},
	} {
		_, err := napCORS(t, tc.gt)
		if !errors.Is(err, ErrNguonCORSHong) {
			t.Fatalf("%q: err = %v, muốn ErrNguonCORSHong", tc.gt, err)
		}
		if !strings.Contains(err.Error(), "CITIZEN_CORS_ALLOWED_ORIGINS") || !strings.Contains(err.Error(), tc.muc) {
			t.Errorf("%q: lỗi phải gọi tên biến và %s: %v", tc.gt, tc.muc, err)
		}
	}
}
