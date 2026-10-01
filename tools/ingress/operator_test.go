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

// THE REAL CONTRACT AND THE REAL GENERATED FILES: nothing routes to service-platform's REST port —
// platform serves no commune REST route, and its operator routes are reached only on OPERATOR_HOST
// through platform-admin.
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
		if r.DichVu == "platform" {
			t.Errorf("contract routes %s to platform", r.Duong)
		}
	}
	for _, f := range []string{duongTepSinh, duongTepTS} {
		b, err := os.ReadFile(filepath.Join(root, f))
		if err != nil {
			t.Fatal(err)
		}
		for _, needle := range []string{"name: platform", `"platform"`, "operator-sessions", "operator-enrollments"} {
			if strings.Contains(string(b), needle) {
				t.Errorf("%s contains %q — an operator or platform route reached a commune surface", f, needle)
			}
		}
	}
}
