package domain

import "testing"

// The same cases the comms and identity copies are held to, so the three cannot drift unnoticed here.
func TestValidCommuneHost(t *testing.T) {
	for _, ok := range []string{"thangbinh.vigov.vn", "xa-1.vigov.vn", "xn--thng-bnh.vn"} {
		if !ValidCommuneHost(ok) {
			t.Errorf("%q refused", ok)
		}
	}
	for _, bad := range []string{"", "localhost", "ThangBinh.vigov.vn", "https://xa.vigov.vn",
		"xa.vigov.vn:443", "xa.vigov.vn/x", ".xa.vn", "xa..vn", "xa.vn.", "-xa.vn", "10.0.0.1", "xa vn.vn"} {
		if ValidCommuneHost(bad) {
			t.Errorf("%q accepted", bad)
		}
	}
}
