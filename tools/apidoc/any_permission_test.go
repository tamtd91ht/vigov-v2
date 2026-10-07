package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// authz.RequireAnyPermission is read like RequirePermission: literal keys only, kind "permission".
// The keys go to `any_of`, and `key` stays absent — a single-key reader must not mistake one of
// two alternatives for THE key the route demands.

func TestRequireAnyPermissionIsRead(t *testing.T) {
	ts, errs := trich(t, dauFile+`
	// @summary  Hạng mục
	// @reply    201 -
	mux.Handle("POST /api/v1/things",
		authz.RequireAnyPermission(d.Checker, "admin.lookup", "budget.update")(
			idem.KhongCan("tạo lại là trùng mã, bị từ chối")(http.HandlerFunc(nil))))
}
`)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(ts) != 1 {
		t.Fatalf("want 1 route, got %d", len(ts))
	}
	q := ts[0].Quyen
	if q.Kind != "permission" || q.Key != "" || strings.Join(q.AnyOf, ",") != "admin.lookup,budget.update" {
		t.Errorf("declaration read wrong: %+v", q)
	}
}

func TestRequireAnyPermissionRefusals(t *testing.T) {
	for name, stmt := range map[string]string{
		"key not literal": `mux.Handle("GET /api/v1/x", authz.RequireAnyPermission(d.Checker, "admin.lookup", k)(h))`,
		"one key":         `mux.Handle("GET /api/v1/x", authz.RequireAnyPermission(d.Checker, "admin.lookup")(h))`,
		"no key":          `mux.Handle("GET /api/v1/x", authz.RequireAnyPermission(d.Checker)(h))`,
		"empty key":       `mux.Handle("GET /api/v1/x", authz.RequireAnyPermission(d.Checker, "admin.lookup", "")(h))`,
	} {
		_, errs := trich(t, dauFile+"\t// @summary s\n\t// @reply 200 -\n\t"+stmt+"\n}\n")
		if len(errs) == 0 {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestPermissionJSONShapes(t *testing.T) {
	one, _ := json.Marshal(quyenJSON(quyenDecl{Kind: "permission", Key: "budget.read"}))
	if string(one) != `{"kind":"permission","key":"budget.read"}` {
		t.Errorf("single key changed shape: %s", one)
	}
	anyOf, _ := json.Marshal(quyenJSON(quyenDecl{Kind: "permission", AnyOf: []string{"admin.lookup", "budget.update"}}))
	if string(anyOf) != `{"kind":"permission","any_of":["admin.lookup","budget.update"]}` {
		t.Errorf("any-of shape: %s", anyOf)
	}
	if got := permissionText(quyenDecl{Kind: "permission", AnyOf: []string{"admin.lookup", "budget.update"}}); got != "admin.lookup|budget.update" {
		t.Errorf("flat permission text = %q", got)
	}
	if got := permissionText(quyenDecl{Kind: "permission", Key: "budget.read"}); got != "budget.read" {
		t.Errorf("flat permission text = %q", got)
	}
}
