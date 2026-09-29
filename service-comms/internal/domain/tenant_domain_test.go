package domain

import (
	"strings"
	"testing"
)

// The SAME table as service-identity/internal/domain/ten_mien_xa_test.go's intent: the two copies must
// accept and refuse the same values (see the note on IsValidTenantDomain).
func TestIsValidTenantDomain(t *testing.T) {
	for _, s := range []string{"xa-qr.vigov.vn", "xn--x-7ga.vigov.vn", "a.b"} {
		if !IsValidTenantDomain(s) {
			t.Errorf("%q bị từ chối", s)
		}
	}
	for _, s := range []string{
		"", "current", "Xa.vigov.vn", "https://xa.vigov.vn", "xa.vigov.vn:443", "xa.vigov.vn/x",
		"a@xa.vigov.vn", "xa.vigov.vn.", ".xa.vigov.vn", "xa..vigov.vn", "-xa.vigov.vn", "xa-.vigov.vn",
		"xa_qr.vigov.vn", "xã.vigov.vn", strings.Repeat("a", 64) + ".vn", "10.0.0.1",
		strings.Repeat("a.", 127) + "vn",
	} {
		if IsValidTenantDomain(s) {
			t.Errorf("%q được nhận", s)
		}
	}
}
