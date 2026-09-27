package domain

import (
	"strings"
	"testing"
)

// Each row is one clause of the SHAPE comment on `commune_host_hint`
// (proto/vigov/identity/v1/citizen_session_bridge.proto).
func TestHopLeTenMienXa(t *testing.T) {
	for s, muon := range map[string]bool{
		"xa-a.vigov.vn":                  true,
		"xn--x-4ja.vigov.vn":             true,
		"a1.b2":                          true,
		strings.Repeat("a", 63) + ".vn":  true,
		"":                               false,
		"current":                        false, // one label
		"Xa-A.vigov.vn":                  false, // not lowercase — refused, never folded
		"https://xa-a.vigov.vn":          false, // scheme
		"xa-a.vigov.vn:443":              false, // port
		"xa-a.vigov.vn/x":                false, // path
		"xa-a.vigov.vn?x=1":              false, // query
		"xa-a.vigov.vn#x":                false, // fragment
		"canbo@xa-a.vigov.vn":            false, // userinfo
		"xa-a.vigov.vn.":                 false, // trailing dot
		".xa-a.vigov.vn":                 false, // leading dot
		"xa..vigov.vn":                   false, // doubled dot
		"-xa.vigov.vn":                   false, // label starts with '-'
		"xa-.vigov.vn":                   false, // label ends with '-'
		"xa_a.vigov.vn":                  false, // underscore
		"xã.vigov.vn":                    false, // non-ASCII
		" xa-a.vigov.vn":                 false, // whitespace is the caller's trim, not ours
		strings.Repeat("a", 64) + ".vn":  false, // label over 63
		strings.Repeat("a.", 126) + "vn": false, // 254 characters
		"10.0.0.1":                       false, // IPv4 literal
		"[::1]":                          false,
	} {
		if got := HopLeTenMienXa(s); got != muon {
			t.Errorf("HopLeTenMienXa(%q) = %v, muốn %v", s, got, muon)
		}
	}
	if n := len(strings.Repeat("a.", 126) + "vn"); n != 254 {
		t.Fatalf("ca 254 ký tự dài %d", n)
	}
	// 253 exactly is accepted.
	s := strings.Repeat("a.", 125) + "abc"
	if len(s) != 253 || !HopLeTenMienXa(s) {
		t.Fatalf("tên miền 253 ký tự (%d) phải hợp lệ", len(s))
	}
}
