package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// What these tests defend: an OPERATOR route (service-platform's operator edge, ADR 0048) never
// reaches the commune-facing contract. openapi.json feeds tools/ingress — the Ingress rules on every
// commune and API host, and web-admin's gateway table — so an operator route there is a route
// routable from a commune's host (ADR 0048 stop condition #6, §01/10 #6c). Nothing else turns red
// for that: the route would work, on the wrong surface.

const operatorFileHeader = `package http

import (
	"net/http"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/service-platform/internal/opauth"
)

func Register(mux *http.ServeMux) {
`

func TestOperatorDeclarationsAreRecognisedAndSplit(t *testing.T) {
	ts, errs := trich(t, operatorFileHeader+`
	// @summary  a
	// @reply    200 -
	mux.Handle("GET /api/v1/communes", opauth.RequireKey(d.Auth, opauth.KeyTenantManage)(h))

	// @summary  b
	// @reply    204 -
	mux.Handle("DELETE /api/v1/operator-sessions/current", opauth.SignedIn(d.Auth, "own session")(idem.KhongCan("same outcome")(h)))

	// @summary  c
	// @reply    201 -
	mux.Handle("POST /api/v1/operator-sessions", opauth.Public("creates the session")(idem.KhongCan("new session")(h)))

	// @summary  d
	// @reply    200 -
	mux.Handle("GET /api/v1/org-units", authz.AnyAuthenticated("every account")(h))
}
`)
	if len(errs) > 0 {
		t.Fatalf("errors: %v", errs)
	}
	commune, operator := splitOperatorRoutes(ts)
	if len(commune) != 1 || commune[0].Path != "/api/v1/org-units" {
		t.Fatalf("commune surface = %+v", commune)
	}
	if len(operator) != 3 {
		t.Fatalf("operator routes = %d, want 3", len(operator))
	}
	for _, o := range operator {
		if o.Quyen.Kind == kindOperatorKey && o.Quyen.Key != "KeyTenantManage" {
			t.Errorf("keys = %q", o.Quyen.Key)
		}
	}
}

func TestOperatorDeclarationRefusals(t *testing.T) {
	for name, stmt := range map[string]string{
		"both realms":      `mux.Handle("GET /api/v1/x", authz.AnyAuthenticated("a")(opauth.SignedIn(d.Auth, "b")(h)))`,
		"signed-in no why": `mux.Handle("GET /api/v1/x", opauth.SignedIn(d.Auth, "")(h))`,
		"public no why":    `mux.Handle("GET /api/v1/x", opauth.Public("")(h))`,
		"key not constant": `mux.Handle("GET /api/v1/x", opauth.RequireKey(d.Auth, k)(h))`,
		"no key":           `mux.Handle("GET /api/v1/x", opauth.RequireKey(d.Auth)(h))`,
	} {
		_, errs := trich(t, operatorFileHeader+"\t// @summary s\n\t// @reply 200 -\n\t"+stmt+"\n}\n")
		if len(errs) == 0 {
			t.Errorf("%s: accepted", name)
		}
	}
}

// THE REAL REPOSITORY: the operator routes service-platform registers are found, and the contract
// apidoc writes contains none of them.
func TestRealContractCarriesNoOperatorRoute(t *testing.T) {
	root, err := timGoc()
	if err != nil {
		t.Fatal(err)
	}
	all, err := quetTuyen(root)
	if err != nil {
		t.Fatal(err)
	}
	_, operator := splitOperatorRoutes(all)
	if len(operator) < 15 {
		t.Fatalf("found %d operator routes in the repository — the declarations are no longer recognised", len(operator))
	}
	for _, o := range operator {
		if o.Service != "platform" {
			t.Errorf("operator route %s %s declared outside service-platform (%s)", o.Method, o.Path, o.File)
		}
	}

	d := t.TempDir()
	c := cauHinh{Root: root, OpenAPI: filepath.Join(d, "openapi.json"),
		Surface: filepath.Join(d, "api-surface.json"), TasksDir: filepath.Join(d, "tasks", "web")}
	if _, _, err := chay(c); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(c.OpenAPI)
	if err != nil {
		t.Fatal(err)
	}
	type operation struct {
		Tags       []string `json:"tags"`
		Permission any      `json:"x-vigov-permission"`
	}
	var rawDoc struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(raw, &rawDoc); err != nil {
		t.Fatal(err)
	}
	// Only the HTTP verbs of a path item are operations; "parameters" and friends are not.
	var doc struct {
		Paths map[string]map[string]operation
	}
	doc.Paths = map[string]map[string]operation{}
	for p, item := range rawDoc.Paths {
		doc.Paths[p] = map[string]operation{}
		for k, v := range item {
			switch k {
			case "get", "put", "post", "delete", "patch", "head", "options":
				var op operation
				if err := json.Unmarshal(v, &op); err != nil {
					t.Fatal(err)
				}
				doc.Paths[p][k] = op
			}
		}
	}
	for p, ops := range doc.Paths {
		for m, op := range ops {
			for _, tag := range op.Tags {
				if tag == "platform" {
					t.Errorf("%s %s tagged platform in the commune contract — platform has no commune REST route", m, p)
				}
			}
			if b, _ := json.Marshal(op.Permission); strings.Contains(string(b), "operator-") {
				t.Errorf("%s %s carries an operator-realm declaration", m, p)
			}
		}
	}
	for _, o := range operator {
		if ops, ok := doc.Paths[o.Path]; ok {
			if _, ok := ops[strings.ToLower(o.Method)]; ok && o.Path != "/api/v1/communes" {
				t.Errorf("operator route %s %s is in openapi.json", o.Method, o.Path)
			}
		}
	}
	// GET /api/v1/communes exists on BOTH surfaces with different owners (identity's Mini App picker,
	// platform's operator list). Only identity's may be in the contract.
	if op, ok := doc.Paths["/api/v1/communes"]["get"]; ok && (len(op.Tags) != 1 || op.Tags[0] != "identity") {
		t.Errorf("GET /api/v1/communes tags = %v, want [identity]", op.Tags)
	}
}
