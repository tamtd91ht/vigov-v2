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
		if r.DichVu == "platform" && r.Duong != platformCommunePrefix &&
			!strings.HasPrefix(r.Duong, platformCommunePrefix+"/") {
			t.Errorf("contract routes %s to platform — only %s is a platform commune route (ADR 0069)",
				r.Duong, platformCommunePrefix)
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
	// The gateway table names platform for exactly one prefix, and the Ingress has one platform rule.
	ts, err := os.ReadFile(filepath.Join(root, duongTepTS))
	if err != nil {
		t.Fatal(err)
	}
	if n, ok := strings.Count(string(ts), `dichVu: "platform"`), strings.Contains(string(ts),
		`{ tienTo: "`+platformCommunePrefix+`", dichVu: "platform" }`); n > 1 || (n == 1 && !ok) {
		t.Errorf("%s routes %d prefixes to platform, want only %s", duongTepTS, n, platformCommunePrefix)
	}
	// ZERO Ingress rules reach platform. Its REST port also serves the operator realm, so any public
	// rule — above all a `path: /` host rule — puts /api/v1/operator-* one spoofed Host away from the
	// internet (servicesWithoutAPIHost, ADR 0048 #6(c)). Its commune routes are reached in-cluster
	// through web-admin's gateway, which the dinh-tuyen check above covers.
	ing, err := os.ReadFile(filepath.Join(root, duongTepSinh))
	if err != nil {
		t.Fatal(err)
	}
	if n := platformBackends(string(ing)); n != 0 {
		t.Errorf("%s has %d rule(s) whose backend is platform, want 0 — a public route to the pod that "+
			"serves the operator realm", duongTepSinh, n)
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
