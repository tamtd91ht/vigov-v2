package domain

import "testing"

// bangTenMienDanhRieng is the ONE table of hosts both halves of the rule are judged against: the Go
// function (TestLaTenMienDanhRieng) and the CHECK patterns parsed out of migration 0007
// (TestLuatTenMienDanhRiengGoVaSQLKhop). One table, because two would be two that drift.
//
// Every entry is in the stored shape the CHECK sees (no port, no surrounding space); the Go-only
// spellings live in TestLaTenMienDanhRiengChuanHoa below.
var bangTenMienDanhRieng = []struct {
	Host      string
	DanhRieng bool
}{
	// Reserved: the vendor console and its staging twin — the two rows that exist today.
	{"admin.vigov.vn", true},
	{"ADMIN.vigov.vn.", true},
	{"admin-stg.vigov.vn", true},
	// Reserved: the remaining first labels under vigov.vn.
	{"api.vigov.vn", true},
	{"api-stg.vigov.vn", true},
	{"stg.vigov.vn", true},
	{"www.vigov.vn", true},
	// Reserved: the apex.
	{"vigov.vn", true},
	{"vigov.vn.", true},
	// Reserved: service API hosts, prod and staging, including a nested label.
	{"identity.api.vigov.vn", true},
	{"petitions.api-stg.vigov.vn", true},
	{"a.b.api.vigov.vn", true},
	// Reserved: the same labels under stg.vigov.vn.
	{"admin.stg.vigov.vn", true},
	{"api.stg.vigov.vn", true},
	{"www.stg.vigov.vn", true},
	{"stg.stg.vigov.vn", true},
	{"admin-stg.stg.vigov.vn", true},

	// Allowed: commune hosts, prod and staging.
	{"thangbinh-danang.vigov.vn", false},
	{"thangbinh-danang.stg.vigov.vn", false},
	{"tanphu.vigov.vn", false},
	// Allowed: a commune label that merely CONTAINS a reserved word is still a commune.
	{"apixa.vigov.vn", false},
	{"xa-api-moi.vigov.vn", false},
	{"adminxa.vigov.vn", false},
	{"wwwxa.stg.vigov.vn", false},
	{"stgxa.vigov.vn", false},
	// Allowed: suffix look-alikes that are not the .api./.api-stg. zones.
	{"xa.myapi.vigov.vn", false},
	{"api.vigov.vn.example.com", false},
	{"vigov.vn.example.com", false},
	{"notvigov.vn", false},
	// Allowed: outside vigov.vn entirely — not this rule's business.
	{"tanphu.danang.gov.vn", false},
	{"localhost", false},
}

func TestLaTenMienDanhRieng(t *testing.T) {
	t.Parallel()
	for _, c := range bangTenMienDanhRieng {
		if got := LaTenMienDanhRieng(c.Host); got != c.DanhRieng {
			t.Errorf("LaTenMienDanhRieng(%q) = %v, muốn %v", c.Host, got, c.DanhRieng)
		}
	}
}

// The spellings a client can send that the stored shape never has: port, surrounding space, mixed
// case, several trailing dots. Each must be judged like its plain form, or "ADMIN.vigov.vn.:443"
// is the spelling that reaches the table.
func TestLaTenMienDanhRiengChuanHoa(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		host string
		muon bool
	}{
		{"admin.vigov.vn:443", true},
		{"  Admin.ViGov.VN  ", true},
		{"ADMIN.vigov.vn.:8443", true},
		{"admin.vigov.vn..", true},
		{"Identity.API.vigov.vn:443", true},
		{"tanphu.vigov.vn:443", false},
		{"TanPhu.ViGov.vn.", false},
		// Not a host at all: NormaliseHost refuses it, the lookup answers InvalidArgument itself.
		{"", false},
		{"https://admin.vigov.vn", false},
	} {
		if got := LaTenMienDanhRieng(c.host); got != c.muon {
			t.Errorf("LaTenMienDanhRieng(%q) = %v, muốn %v", c.host, got, c.muon)
		}
	}
}
