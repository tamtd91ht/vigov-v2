package domain

import "strings"

// Reserved hosts: the addresses of the platform itself, which must NEVER resolve to a commune.
//
// THE DOMAIN MODEL THIS FOLLOWS (owner's decision, 2026-09-26):
//
//	<xa>.vigov.vn               prod web of one commune
//	<xa>.stg.vigov.vn           staging web of one commune
//	<service>.api.vigov.vn      prod API of one service
//	<service>.api-stg.vigov.vn  staging API of one service
//	admin.vigov.vn              the vendor's cross-commune console — NOT a commune host
//
// WHY A HOST-SHAPE RULE AND NOT JUST "DO NOT INSERT THOSE ROWS": `admin.vigov.vn` and
// `admin-stg.vigov.vn` were inserted, mapped to a real commune, by the deploy pipeline. A request
// to the vendor's console address then entered that commune's edge with that commune in context
// — the vendor surface and one commune's surface became the same surface, which is exactly the
// boundary ADR 0003 draws. The rows are kept (rule 7; the owner decides their removal); what
// changes is that the lookup refuses them before it reads the table.
//
// KEPT IN LOCK-STEP with the CHECK `tenant_domain_khong_danh_rieng` in
// migrations/0007_tenant_domain_khong_danh_rieng.sql. Changing one without the other is caught by
// domain.TestLuatTenMienDanhRiengGoVaSQLKhop, which runs both over the same table of hosts.

// nhanDanhRieng are the first labels, directly under vigov.vn or stg.vigov.vn, that name a
// platform surface rather than a commune.
var nhanDanhRieng = map[string]bool{
	"admin":     true,
	"admin-stg": true,
	"api":       true,
	"api-stg":   true,
	"stg":       true,
	"www":       true,
}

const (
	mienGoc    = "vigov.vn"
	mienGocStg = "stg.vigov.vn"
)

// LaTenMienDanhRieng reports whether host is a platform address that must never resolve to a
// commune. host may arrive raw: it is lower-cased, trimmed, stripped of its port and of any
// trailing dot here, so that "ADMIN.vigov.vn.:443" cannot slip past as a spelling nobody listed.
//
// A commune label merely CONTAINING "api" or "admin" ("apixa", "xa-api-moi") is NOT reserved:
// only the exact labels and the `.api.` / `.api-stg.` service suffixes are. Anything outside
// vigov.vn is not this function's business and answers false.
//
// An input NormaliseHost refuses answers false: the lookup refuses it anyway, with its own code.
func LaTenMienDanhRieng(host string) bool {
	h, err := NormaliseHost(host)
	if err != nil {
		return false
	}
	h = strings.TrimRight(h, ".")

	if h == mienGoc || h == mienGocStg {
		return true
	}
	if strings.HasSuffix(h, ".api."+mienGoc) || strings.HasSuffix(h, ".api-stg."+mienGoc) {
		return true
	}
	// The stg suffix is tried first: "admin.stg.vigov.vn" also ends in ".vigov.vn", and its
	// label under vigov.vn ("admin.stg") would miss the table.
	for _, goc := range []string{mienGocStg, mienGoc} {
		if nhan, ok := strings.CutSuffix(h, "."+goc); ok {
			return nhanDanhRieng[nhan]
		}
	}
	return false
}
