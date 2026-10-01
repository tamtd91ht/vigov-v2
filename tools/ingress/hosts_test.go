package main

import (
	"strings"
	"testing"
)

const validHosts = `environments:
  - name: prod
    web_host: "*.vigov.vn"
    api_suffix: "api.vigov.vn"
    web_tls_secret: a-tls
    api_tls_secret: b-tls
  - name: staging
    web_host: "*.stg.vigov.vn"
    api_suffix: "api-stg.vigov.vn"
    web_tls_secret: c-tls
    api_tls_secret: d-tls
mini_app_environment: prod
`

func TestParseHostPlanAcceptsTheShape(t *testing.T) {
	plan, err := parseHostPlan([]byte(validHosts))
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Environments) != 2 || plan.Environments[0].Ten != "prod" || plan.MiniApp.DuoiAPI != "api.vigov.vn" {
		t.Fatalf("plan = %+v", plan)
	}
	if got := webRootsMostSpecificFirst(plan); strings.Join(got, ",") != "stg.vigov.vn,vigov.vn" {
		t.Errorf("web roots = %v, want the staging root before the prod root", got)
	}
}

// Every refusal below is a value that would otherwise reach an Ingress host, a TLS SAN or a
// reserved-domain rule. None may be repaired or defaulted.
func TestParseHostPlanFailsClosed(t *testing.T) {
	for _, c := range []struct{ name, yaml, want string }{
		{"unknown key", strings.Replace(validHosts, "api_suffix: \"api.vigov.vn\"", "api_sufix: \"api.vigov.vn\"", 1), "api_sufix"},
		{"unknown top-level key", validHosts + "extra: 1\n", "extra"},
		{"missing field", strings.Replace(validHosts, "    api_tls_secret: b-tls\n", "", 1), "api_tls_secret"},
		{"empty string", strings.Replace(validHosts, `"*.vigov.vn"`, `""`, 1), "web_host"},
		{"blank string", strings.Replace(validHosts, `"*.vigov.vn"`, `"  "`, 1), "web_host"},
		{"duplicate env", strings.Replace(validHosts, "name: staging", "name: prod", 1), "hai lần"},
		{"mini app not listed", strings.Replace(validHosts, "mini_app_environment: prod", "mini_app_environment: dev", 1), "mini_app_environment"},
		{"mini app missing", strings.Replace(validHosts, "mini_app_environment: prod\n", "", 1), "mini_app_environment"},
		{"no environments", "mini_app_environment: prod\n", "environments"},
		{"web host without wildcard", strings.Replace(validHosts, `"*.vigov.vn"`, `"vigov.vn"`, 1), "web_host"},
		{"web host double wildcard", strings.Replace(validHosts, `"*.vigov.vn"`, `"*.*.vigov.vn"`, 1), "web_host"},
		{"upper-case suffix", strings.Replace(validHosts, `"api.vigov.vn"`, `"API.vigov.vn"`, 1), "api_suffix"},
		{"suffix with port", strings.Replace(validHosts, `"api.vigov.vn"`, `"api.vigov.vn:443"`, 1), "api_suffix"},
		{"single-label suffix", strings.Replace(validHosts, `"api.vigov.vn"`, `"vn"`, 1), "api_suffix"},
		{"shared TLS secret", strings.Replace(validHosts, "d-tls", "a-tls", 1), "a-tls"},
		{"second document", validHosts + "---\nenvironments: []\n", "MỘT"},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := parseHostPlan([]byte(c.yaml))
			if err == nil {
				t.Fatalf("accepted:\n%s", c.yaml)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %v, want it to name %q", err, c.want)
			}
		})
	}
}

// The real file must parse: a red here means the generator would stop, and `make kb` with it.
func TestRealHostsFileParses(t *testing.T) {
	if _, err := loadHostPlan(goc(t)); err != nil {
		t.Fatal(err)
	}
}
