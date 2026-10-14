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
  - name: dev
    web_host: "*.dev.vigov.vn"
    api_suffix: "api.vigov.vn"
    api_label_suffix: "-dev"
    web_tls_secret: c-tls
    api_tls_secret: b-tls
  - name: staging
    retired: true
    web_host: "*.stg.vigov.vn"
    api_suffix: "api-stg.vigov.vn"
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
	if len(plan.Retired) != 1 || plan.Retired[0].Ten != "staging" {
		t.Fatalf("retired = %+v, want staging only", plan.Retired)
	}
	// Retired roots stay in the reserved list; more labels first, then file order.
	if got := webRootsMostSpecificFirst(plan); strings.Join(got, ",") != "dev.vigov.vn,stg.vigov.vn,vigov.vn" {
		t.Errorf("web roots = %v, want dev and staging roots before the prod root", got)
	}
	// dev shares prod's suffix: listed once. The retired suffix stays.
	if got := apiSuffixes(plan); strings.Join(got, ",") != "api.vigov.vn,api-stg.vigov.vn" {
		t.Errorf("api suffixes = %v", got)
	}
	if h := plan.Environments[1].apiHost("comms"); h != "comms-dev.api.vigov.vn" {
		t.Errorf("dev api host = %q", h)
	}
	if h := plan.Environments[0].apiHost("comms"); h != "comms.api.vigov.vn" {
		t.Errorf("prod api host = %q — an empty api_label_suffix must leave prod unchanged", h)
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
		{"duplicate env", strings.Replace(validHosts, "name: dev", "name: prod", 1), "hai lần"},
		{"duplicate retired name", strings.Replace(validHosts, "name: staging", "name: dev", 1), "hai lần"},
		{"mini app not listed", strings.Replace(validHosts, "mini_app_environment: prod", "mini_app_environment: qa", 1), "mini_app_environment"},
		{"mini app missing", strings.Replace(validHosts, "mini_app_environment: prod\n", "", 1), "mini_app_environment"},
		{"no environments", "mini_app_environment: prod\n", "environments"},
		{"web host without wildcard", strings.Replace(validHosts, `"*.vigov.vn"`, `"vigov.vn"`, 1), "web_host"},
		{"web host double wildcard", strings.Replace(validHosts, `"*.vigov.vn"`, `"*.*.vigov.vn"`, 1), "web_host"},
		{"upper-case suffix", strings.Replace(validHosts, `"api.vigov.vn"`, `"API.vigov.vn"`, 1), "api_suffix"},
		{"suffix with port", strings.Replace(validHosts, `"api.vigov.vn"`, `"api.vigov.vn:443"`, 1), "api_suffix"},
		{"single-label suffix", strings.Replace(validHosts, `"api.vigov.vn"`, `"vn"`, 1), "api_suffix"},
		{"web secret reused as web", strings.Replace(validHosts, "web_tls_secret: c-tls", "web_tls_secret: a-tls", 1), "a-tls"},
		{"api secret reused as web", strings.Replace(validHosts, "web_tls_secret: c-tls", "web_tls_secret: b-tls", 1), "b-tls"},
		{"api secret shared across suffixes", strings.Replace(validHosts,
			"api_suffix: \"api.vigov.vn\"\n    api_label_suffix", "api_suffix: \"api-dev.vigov.vn\"\n    api_label_suffix", 1), "b-tls"},
		{"same api host family", strings.Replace(validHosts, "    api_label_suffix: \"-dev\"\n", "", 1), "trùng"},
		{"label suffix without dash", strings.Replace(validHosts, `"-dev"`, `"dev"`, 1), "api_label_suffix"},
		{"label suffix with dot", strings.Replace(validHosts, `"-dev"`, `"-dev.x"`, 1), "api_label_suffix"},
		{"label suffix upper-case", strings.Replace(validHosts, `"-dev"`, `"-DEV"`, 1), "api_label_suffix"},
		{"retired with TLS", strings.Replace(validHosts, "    retired: true\n", "    retired: true\n    web_tls_secret: e-tls\n", 1), "nghỉ hưu"},
		{"retired with label suffix", strings.Replace(validHosts, "    retired: true\n", "    retired: true\n    api_label_suffix: \"-stg\"\n", 1), "nghỉ hưu"},
		{"retired mini app", strings.Replace(validHosts, "mini_app_environment: prod", "mini_app_environment: staging", 1), "mini_app_environment"},
		{"retired first", strings.Replace(validHosts, "  - name: prod\n", "  - name: prod\n    retired: true\n", 1), "nghỉ hưu"},
		{"live without TLS", strings.Replace(validHosts, "    web_tls_secret: c-tls\n", "", 1), "web_tls_secret"},
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
