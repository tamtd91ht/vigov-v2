package config

// TRUSTED_PROXY_CIDRS — which hops may state the client address (rule 6, invariant 2).
//
// Addresses are from the documentation ranges (RFC 5737, RFC 3849): nothing here names a real host.

import (
	"errors"
	"net/netip"
	"strings"
	"testing"
)

func napProxy(t *testing.T, v string) (Config, error) {
	t.Helper()
	datMoiTruong(t, map[string]string{
		"DATABASE_DSN":        dsnGia,
		"ENV":                 EnvDev,
		"TRUSTED_PROXY_CIDRS": v,
	})
	return Load("identity")
}

func TestProxyTinCayDocDuocCidrVaIpTron(t *testing.T) {
	cfg, err := napProxy(t, " 10.0.0.0/8, 192.0.2.10 ,2001:db8::/32,2001:db8::1,, ::ffff:198.51.100.7")
	if err != nil {
		t.Fatalf("Load lỗi: %v", err)
	}
	muon := []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("192.0.2.10/32"),
		netip.MustParsePrefix("2001:db8::/32"),
		netip.MustParsePrefix("2001:db8::1/128"),
		// A mapped bare IP is the IPv4 host: the request side unmaps before comparing.
		netip.MustParsePrefix("198.51.100.7/32"),
	}
	if len(cfg.TrustedProxies) != len(muon) {
		t.Fatalf("TrustedProxies = %v, muốn %v", cfg.TrustedProxies, muon)
	}
	for i := range muon {
		if cfg.TrustedProxies[i] != muon[i] {
			t.Errorf("mục %d = %v, muốn %v", i, cfg.TrustedProxies[i], muon[i])
		}
	}
}

func TestProxyTinCayCidrCoBitThuaDuocChuanHoa(t *testing.T) {
	// "10.1.2.3/8" is a common way to write the range a pod sits in; it means 10.0.0.0/8.
	cfg, err := napProxy(t, "10.1.2.3/8")
	if err != nil {
		t.Fatalf("Load lỗi: %v", err)
	}
	if len(cfg.TrustedProxies) != 1 || cfg.TrustedProxies[0] != netip.MustParsePrefix("10.0.0.0/8") {
		t.Errorf("TrustedProxies = %v", cfg.TrustedProxies)
	}
}

func TestProxyTinCayTrongLaKhongTinAi(t *testing.T) {
	for _, v := range []string{"", "  ", " , ,"} {
		cfg, err := napProxy(t, v)
		if err != nil {
			t.Fatalf("%q: Load lỗi: %v", v, err)
		}
		if cfg.TrustedProxies != nil {
			t.Errorf("%q: TrustedProxies = %v, muốn nil", v, cfg.TrustedProxies)
		}
	}
}

func TestProxyTinCayMucHongThiLoadTuChoiVaGoiTen(t *testing.T) {
	// The typo must stop startup: skipping it silently trusts fewer hops than the operator wrote.
	for _, tc := range []struct{ gt, muc string }{
		{"10.0.0.0/8, 10.0.0.300", "mục thứ 2"},
		{"khong-phai-dia-chi", "mục thứ 1"},
		{"10.0.0.0/8,,10.0.0.0/33", "mục thứ 3"},
		{"10.0.0.0/8,192.0.2.1:8080", "mục thứ 2"},
	} {
		_, err := napProxy(t, tc.gt)
		if !errors.Is(err, ErrProxyTinCayHong) {
			t.Fatalf("%q: err = %v, muốn ErrProxyTinCayHong", tc.gt, err)
		}
		if !strings.Contains(err.Error(), "TRUSTED_PROXY_CIDRS") || !strings.Contains(err.Error(), tc.muc) {
			t.Errorf("%q: lỗi phải gọi tên biến và %s: %v", tc.gt, tc.muc, err)
		}
	}
}

func TestProxyTinCayTuChoiTinCaInternet(t *testing.T) {
	for _, v := range []string{"0.0.0.0/0", "10.0.0.0/8, ::/0", "192.0.2.1/0"} {
		_, err := napProxy(t, v)
		if !errors.Is(err, ErrProxyTinCayHong) {
			t.Errorf("%q: err = %v, muốn ErrProxyTinCayHong", v, err)
			continue
		}
		if !strings.Contains(err.Error(), "TRUSTED_PROXY_CIDRS") {
			t.Errorf("%q: lỗi phải gọi tên biến: %v", v, err)
		}
	}
}
