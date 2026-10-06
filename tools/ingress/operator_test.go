package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// What these tests defend: no OPERATOR route (ADR 0048) ever becomes an Ingress rule on a commune or
// API host, nor an entry of web-admin's gateway table — both are generated from the contract this
// package reads. An operator route there would be routable from a commune's host (ADR 0048 stop
// condition #6). tools/apidoc keeps such routes out of openapi.json; this is the second line.

func TestOperatorRouteInContractStopsTheGenerator(t *testing.T) {
	for _, kind := range []string{"operator-key", "operator-signed-in", "operator-public"} {
		than := `"/api/v1/org-units":{"get":{"tags":["identity"],"operationId":"identity_get_org_units"}},` +
			`"/api/v1/operator-sessions":{"post":{"tags":["platform"],"operationId":"platform_post_operator_sessions",` +
			`"x-vigov-permission":{"kind":"` + kind + `"}}}`
		_, err := docHopDong(hopDongGia(than))
		if err == nil || !strings.Contains(err.Error(), "miền vận hành") {
			t.Errorf("%s: err = %v, want a refusal naming the operator realm", kind, err)
		}
	}
	// A commune declaration of the same shape passes.
	ok := `"/api/v1/org-units":{"get":{"tags":["identity"],"operationId":"identity_get_org_units",` +
		`"x-vigov-permission":{"kind":"any-authenticated"}}}`
	if _, err := docHopDong(hopDongGia(ok)); err != nil {
		t.Errorf("commune route refused: %v", err)
	}
}

// platformCommunePrefix is the ONE commune-surface resource service-platform owns: the commune's own
// logo and web-admin banner, staff routes under admin.org (ADR 0069, owner decision 02/10/2026 —
// "service-platform lần đầu nhận tuyến ghi của cán bộ"). Until that decision platform served no commune
// REST route at all and this test said so; it now admits exactly this prefix and still refuses any other
// platform path — a second one is a decision to record in an ADR first, then here.
const platformCommunePrefix = "/api/v1/commune-branding"

// THE REAL CONTRACT AND THE REAL GENERATED FILES: nothing but platformCommunePrefix routes to
// service-platform's REST port, and its operator routes are reached only on OPERATOR_HOST through
// platform-admin — never in the Ingress or web-admin's gateway table.
func TestRealTablesRouteNothingToPlatform(t *testing.T) {
	root, err := timGoc()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, duongHopDong))
	if err != nil {
		t.Fatal(err)
	}
	routes, err := docHopDong(raw)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range routes {
		if _, public := pathScopedPublicRoutes["platform"][r.Duong]; public {
			continue
		}
		if r.DichVu == "platform" && r.Duong != platformCommunePrefix &&
			!strings.HasPrefix(r.Duong, platformCommunePrefix+"/") {
			t.Errorf("contract routes %s to platform — only %s is a platform commune route (ADR 0069), "+
				"plus the owner-approved public paths in pathScopedPublicRoutes", r.Duong, platformCommunePrefix)
		}
	}
	for _, f := range []string{duongTepSinh, duongTepTS} {
		b, err := os.ReadFile(filepath.Join(root, f))
		if err != nil {
			t.Fatal(err)
		}
		for _, needle := range []string{"operator-sessions", "operator-enrollments", "operator-communes", "/api/v1/operator"} {
			if strings.Contains(string(b), needle) {
				t.Errorf("%s contains %q — an operator route reached a commune surface", f, needle)
			}
		}
	}
	// The gateway table names platform only for commune-branding and the allow-listed public paths.
	ts, err := os.ReadFile(filepath.Join(root, duongTepTS))
	if err != nil {
		t.Fatal(err)
	}
	allowedTS := []string{platformCommunePrefix}
	for p := range pathScopedPublicRoutes["platform"] {
		allowedTS = append(allowedTS, p)
	}
	n := strings.Count(string(ts), `dichVu: "platform"`)
	found := 0
	for _, p := range allowedTS {
		if strings.Contains(string(ts), `{ tienTo: "`+p+`", dichVu: "platform" }`) {
			found++
		}
	}
	if n != found {
		t.Errorf("%s routes %d prefixes to platform, only %d of them allowed (%v)", duongTepTS, n, found, allowedTS)
	}
	// The ONLY Ingress rules reaching platform are the Exact rules of pathScopedPublicRoutes. Its REST
	// port also serves the operator realm, so any other public rule — above all a `path: /` host rule —
	// puts /api/v1/operator-* one spoofed Host away from the internet (ADR 0048 #6(c)).
	ing, err := os.ReadFile(filepath.Join(root, duongTepSinh))
	if err != nil {
		t.Fatal(err)
	}
	if n, want := platformBackends(string(ing)), len(pathScopedPublicRoutes["platform"]); n != want {
		t.Errorf("%s has %d rule(s) whose backend is platform, want %d (the Exact public paths only)",
			duongTepSinh, n, want)
	}
	for _, r := range docTepSinh(t).Spec.Rules {
		for _, p := range r.HTTP.Paths {
			if p.Backend.Service.Name != "platform" {
				continue
			}
			if _, ok := pathScopedPublicRoutes["platform"][p.Path]; !ok || p.PathType != "Exact" {
				t.Errorf("Ingress rule %s %s → platform: only Exact rules of pathScopedPublicRoutes may reach platform",
					p.PathType, p.Path)
			}
		}
	}
}

// platformBackends counts Ingress backends naming the platform Service. Comments are skipped: the
// generated file's prose mentions `platform` and must not count.
func platformBackends(ingressYAML string) int {
	n := 0
	for _, line := range strings.Split(ingressYAML, "\n") {
		s := strings.TrimSpace(line)
		if strings.HasPrefix(s, "#") {
			continue
		}
		if s == "name: platform" || strings.HasPrefix(s, "name: platform ") || s == `name: "platform"` {
			n++
		}
	}
	return n
}

// The generator itself emits no host rule for platform even when the contract routes a commune path
// to it — the property the file check above relies on, tested without the real files.
func TestNoAPIHostForPlatform(t *testing.T) {
	hosts, err := cacHostDichVu([]luatIngress{
		{Duong: "/api/v1/commune-branding", DichVu: "platform", Cong: "rest"},
		{Duong: "/api/v1/org-units", DichVu: "identity", Cong: "rest"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range hosts {
		if h.DichVu == "platform" {
			t.Fatalf("generator emitted a public host for platform: %+v", h)
		}
	}
	if len(hosts) != 1 || hosts[0].DichVu != "identity" {
		t.Errorf("hosts = %+v, want identity only", hosts)
	}
}

// An allow-listed public path gives platform a host with that Exact path ONLY — commune-branding,
// grouped beside it, never reaches the host.
func TestPlatformHostCarriesOnlyAllowListedExactPaths(t *testing.T) {
	hosts, err := cacHostDichVu([]luatIngress{
		{Duong: "/api/v1/commune-branding", DichVu: "platform", Cong: "rest", Phu: []string{"/api/v1/commune-branding"}},
		{Duong: "/api/v1/mini-app-ids", DichVu: "platform", Cong: "rest", Phu: []string{"/api/v1/mini-app-ids"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 1 || hosts[0].DichVu != "platform" ||
		strings.Join(hosts[0].Exact, ",") != "/api/v1/mini-app-ids" {
		t.Fatalf("hosts = %+v, want platform with exactly /api/v1/mini-app-ids", hosts)
	}
}

// The allow-list is checked against the contract: missing, owned elsewhere, or not public → stop.
func TestPathScopedPublicRoutesCheckedAgainstContract(t *testing.T) {
	ok := []tuyenHopDong{{Duong: "/api/v1/mini-app-ids", DichVu: "platform", Public: true}}
	if err := checkPathScopedPublicRoutes(ok); err != nil {
		t.Fatalf("valid contract refused: %v", err)
	}
	for name, bad := range map[string][]tuyenHopDong{
		"missing":    {},
		"not public": {{Duong: "/api/v1/mini-app-ids", DichVu: "platform", Public: false}},
		"other svc":  {{Duong: "/api/v1/mini-app-ids", DichVu: "identity", Public: true}},
	} {
		if err := checkPathScopedPublicRoutes(bad); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestPlatformBackendsCountsRulesNotComments(t *testing.T) {
	fixture := "    # platform — mentioned in prose\n" +
		"    - host: \"platform.api.vigov.vn\"\n      http:\n        paths:\n          - path: /\n" +
		"            pathType: Prefix\n            backend:\n              service:\n" +
		"                name: platform\n                port: { name: rest }\n"
	if n := platformBackends(fixture); n != 1 {
		t.Errorf("platformBackends(public platform rule) = %d, want 1", n)
	}
	if n := platformBackends("    # name: platform\n                name: petitions\n"); n != 0 {
		t.Errorf("platformBackends(comment only) = %d, want 0", n)
	}
}
