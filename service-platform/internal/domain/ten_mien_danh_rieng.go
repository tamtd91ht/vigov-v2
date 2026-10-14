package domain

import (
	"slices"
	"strings"
)

// Reserved hosts: the addresses of the platform itself, which must NEVER resolve to a commune.
//
// THE DOMAIN MODEL THIS FOLLOWS (owner's decisions, 2026-09-26 and 2026-10-14):
//
//	<xa>.vigov.vn               prod web of one commune
//	<xa>.dev.vigov.vn           dev web of one commune
//	<service>.api.vigov.vn      prod API of one service
//	<service>-dev.api.vigov.vn  dev API of one service (covered by the `.api.vigov.vn` suffix)
//	admin.vigov.vn              the vendor's cross-commune console — NOT a commune host
//	admin.dev.vigov.vn          its dev twin (`admin` under the dev root)
//
// `admin-dev.vigov.vn` was the first name chosen for the dev console (14/10/2026) and was replaced
// the same day by `admin.dev.vigov.vn`, which the dev wildcard certificate covers. Its label stays
// reserved below: 0027 already ships it, and releasing a reserved label only ever widens what a
// commune row may claim.
//
// RETIRED, STILL RESERVED (owner, 2026-10-14): staging — `stg.vigov.vn`, `admin-stg`, `api-stg`,
// `*.api-stg.vigov.vn`. Prod's database still holds a `<xa>.stg.vigov.vn` row from when staging
// shared it; the root and its platform labels stay out of every commune's reach. Commune labels
// under stg.vigov.vn keep the meaning they had (allowed) so that row is not turned into a violation.
//
// WHY A HOST-SHAPE RULE AND NOT JUST "DO NOT INSERT THOSE ROWS": `admin.vigov.vn` and
// `admin-stg.vigov.vn` were inserted, mapped to a real commune, by the deploy pipeline. A request
// to the vendor's console address then entered that commune's edge with that commune in context
// — the vendor surface and one commune's surface became the same surface, which is exactly the
// boundary ADR 0003 draws. The rows are kept (rule 7; the owner decides their removal); what
// changes is that the lookup refuses them before it reads the table.
//
// THE ROOTS AND SUFFIXES ARE NOT TYPED HERE: platformWebRoots and platformAPISuffixes are GENERATED
// into hosts.gen.go from deploy/hosts.yaml, the one place public hostnames are written (owner,
// 01/10/2026). Generated into this package rather than imported from core/, because domain/ imports
// the standard library only.
//
// KEPT IN LOCK-STEP with the CHECK `tenant_domain_khong_danh_rieng`, last (re)defined in
// migrations/0027_reserve_dev_hosts.sql (first: 0007). Changing one without the other is caught by
// domain.TestLuatTenMienDanhRiengGoVaSQLKhop, which runs both over the same table of hosts. That
// includes an edit to deploy/hosts.yaml: a changed root or api_suffix moves the Go half on the next
// `make kb`, the SQL half cannot read that file, so it needs a NEW migration (0007 is applied and
// is never edited) — the test is red on the hosts its table holds until that migration exists.

// nhanDanhRieng are the first labels, directly under any platform web root (vigov.vn,
// dev.vigov.vn, stg.vigov.vn), that name a platform surface rather than a commune.
//
// `dev` / `stg` because `<env>.vigov.vn` is an environment's web root; `admin-dev` / `admin-stg` the
// console of that environment. `api-stg` because `api-stg.vigov.vn` was staging's API root. There is
// no `api-dev`: dev's API hosts live under `api.vigov.vn` (`<service>-dev.api.vigov.vn`), whose
// label `api` is already here — reserving a label no surface uses would only shrink the commune
// namespace. (`admin-dev` is such a label now, and is kept for the reason given at the top.)
var nhanDanhRieng = map[string]bool{
	"admin":     true,
	"admin-dev": true,
	"admin-stg": true,
	"api":       true,
	"api-stg":   true,
	"dev":       true,
	"stg":       true,
	"www":       true,
}

// LaTenMienDanhRieng reports whether host is a platform address that must never resolve to a
// commune. host may arrive raw: it is lower-cased, trimmed, stripped of its port and of any
// trailing dot here, so that "ADMIN.vigov.vn.:443" cannot slip past as a spelling nobody listed.
//
// A commune label merely CONTAINING "api" or "admin" ("apixa", "xa-api-moi") is NOT reserved:
// only the exact labels and the `.api.` / `.api-stg.` service suffixes are (`comms-dev.api.vigov.vn`
// is reserved by the `.api.` suffix). Anything outside
// vigov.vn is not this function's business and answers false.
//
// An input NormaliseHost refuses answers false: the lookup refuses it anyway, with its own code.
func LaTenMienDanhRieng(host string) bool {
	h, err := NormaliseHost(host)
	if err != nil {
		return false
	}
	h = strings.TrimRight(h, ".")

	if slices.Contains(platformWebRoots, h) {
		return true
	}
	for _, suffix := range platformAPISuffixes {
		if strings.HasSuffix(h, "."+suffix) {
			return true
		}
	}
	// Most specific root first (the generated order): "admin.dev.vigov.vn" also ends in
	// ".vigov.vn", and its label under vigov.vn ("admin.dev") would miss the table.
	for _, goc := range platformWebRoots {
		if nhan, ok := strings.CutSuffix(h, "."+goc); ok {
			return nhanDanhRieng[nhan]
		}
	}
	return false
}
