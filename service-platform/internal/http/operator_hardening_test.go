package http

// What these tests defend: the findings of the 01/10 security and isolation reviews of the operator
// area — the CSRF guard on every write (operator_request_guard.go), the realm boundary on the two
// account routes, both halves of POST /communes' key pair, the reserved-host check on the
// primary-domain switch, and the refusal to reactivate a commune that has a successor.

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/operatorclient"
	"github.com/vihat/vigov/service-platform/internal/domain"
	"github.com/vihat/vigov/service-platform/internal/store"
)

// Every state-changing operator route: the guarded writes, the account routes and the three
// Public sign-in steps (a login-CSRF is still a CSRF).
var writeRoutes = []struct{ method, path, body string }{
	{"POST", "/api/v1/operator-sessions", `{"email":"x@example.invalid","password":"p"}`},
	{"POST", "/api/v1/operator-enrollments", `{"email":"x@example.invalid","temporary_password":"t"}`},
	{"POST", "/api/v1/operator-enrollments/completion", `{"email":"x@example.invalid","temporary_password":"t","new_password":"n","totp_code":"000000"}`},
	{"DELETE", "/api/v1/operator-sessions/current", `{}`},
	{"PUT", "/api/v1/operators/current/password", `{"current_password":"a","new_password":"b","totp_code":"000000"}`},
	{"POST", "/api/v1/operators/current/recovery-codes", `{"totp_code":"000000"}`},
	{"POST", "/api/v1/communes", `{"name":"Xã Mới","province_id":"66QW36RCJ7GVW79W8GJRYH3ZNR","primary_domain":"xamoi.vigov.vn"}`},
	{"POST", "/api/v1/communes/" + communeIDFake + "/domains", `{"domain":"thangbinh.example.gov.vn"}`},
	{"PUT", "/api/v1/communes/" + communeIDFake + "/primary-domain", `{"domain":"thangbinh-danang.vigov.vn"}`},
	{"PUT", "/api/v1/communes/" + communeIDFake + "/name", `{"name":"Xã Thăng Bình","reason":"Sửa lỗi gõ"}`},
	{"PUT", "/api/v1/communes/" + communeIDFake + "/activation", `{"active":false,"reason":"Sáp nhập"}`},
	{"POST", "/api/v1/communes/" + communeIDFake + "/mini-apps", `{"app_id":"3291993990104489440"}`},
}

func hdr(kv ...string) http.Header {
	h := http.Header{}
	for i := 0; i+1 < len(kv); i += 2 {
		h.Add(kv[i], kv[i+1])
	}
	return h
}

func TestRequestGuardRefusesCrossOriginAndNonJSON(t *testing.T) {
	const own = "https://" + operatorHostFake
	cases := []struct {
		name string
		h    http.Header
		want int
		code string
	}{
		// The attack the review found: a commune page's script, same SITE, text/plain, no preflight.
		{"commune origin, text/plain", hdr("Origin", "https://thangbinh.vigov.vn", "Content-Type", "text/plain"), 403, "origin_refused"},
		{"commune origin, JSON", hdr("Origin", "https://thangbinh.vigov.vn", "Content-Type", "application/json"), 403, "origin_refused"},
		{"own origin over http", hdr("Origin", "http://"+operatorHostFake, "Content-Type", "application/json"), 403, "origin_refused"},
		{"opaque origin", hdr("Origin", "null", "Content-Type", "application/json"), 403, "origin_refused"},
		{"two origins", hdr("Origin", own, "Origin", own, "Content-Type", "application/json"), 403, "origin_refused"},
		{"no origin, same-site fetch", hdr("Sec-Fetch-Site", "same-site", "Content-Type", "application/json"), 403, "origin_refused"},
		{"no origin, no fetch metadata", hdr("Content-Type", "application/json"), 403, "origin_refused"},
		{"own origin, text/plain", hdr("Origin", own, "Content-Type", "text/plain"), 415, "unsupported_media_type"},
		{"own origin, form", hdr("Origin", own, "Content-Type", "application/x-www-form-urlencoded"), 415, "unsupported_media_type"},
		{"own origin, body without type", hdr("Origin", own), 415, "unsupported_media_type"},
		{"own origin, json-ish type", hdr("Origin", own, "Content-Type", "application/json-patch+json"), 415, "unsupported_media_type"},
		{"own origin, two types", hdr("Origin", own, "Content-Type", "application/json", "Content-Type", "text/plain"), 415, "unsupported_media_type"},
	}
	for _, rt := range writeRoutes {
		for _, c := range cases {
			t.Run(rt.method+" "+rt.path+" / "+c.name, func(t *testing.T) {
				h := newHarness(t)
				h.id.keys = []string{"ops.tenant.manage", "ops.domain.manage", "ops.mini_app.manage"}
				rec := h.doRaw(rt.method, rt.path, rt.body, opCookie(t), c.h)
				if rec.Code != c.want || !strings.Contains(rec.Body.String(), `"code":"`+c.code+`"`) {
					t.Fatalf("status %d body %s; want %d %s", rec.Code, rec.Body, c.want, c.code)
				}
				if h.id.calls != 0 || h.w.target != "" || len(h.counter.n) != 0 {
					t.Fatalf("a refused request went further: identity %d, writer %q, limiter %v",
						h.id.calls, h.w.target, h.counter.n)
				}
				var line map[string]any
				if err := json.Unmarshal(h.logs.Bytes(), &line); err != nil {
					t.Fatalf("one security event expected: %v (%s)", err, h.logs)
				}
				if line["event"] != "operator.request_refused" || line["level"] != "WARN" || line["ip"] == "" {
					t.Fatalf("event shape: %v", line)
				}
				// No body value, no cookie, no header value beyond the reason.
				for _, leak := range []string{"example.invalid", "op1.", "thangbinh.vigov.vn", "Xã"} {
					if strings.Contains(h.logs.String(), leak) {
						t.Fatalf("the refusal logged %q: %s", leak, h.logs)
					}
				}
			})
		}
	}
}

// What the console actually sends passes: its own Origin with JSON; or, when a browser omits Origin,
// Sec-Fetch-Site: same-origin; charset parameters are fine; a bodyless DELETE needs no type.
func TestRequestGuardAdmitsTheConsole(t *testing.T) {
	h := newHarness(t)
	h.id.open = operatorclient.OpenResult{Outcome: operatorclient.OutcomeRefused}
	body := `{"email":"x@example.invalid","password":"p"}`
	for name, hh := range map[string]http.Header{
		"origin + json":           hdr("Origin", "https://"+operatorHostFake, "Content-Type", "application/json"),
		"origin + json charset":   hdr("Origin", "https://"+operatorHostFake, "Content-Type", "Application/JSON; charset=utf-8"),
		"fetch-metadata fallback": hdr("Sec-Fetch-Site", "same-origin", "Content-Type", "application/json"),
	} {
		if rec := h.doRaw("POST", "/api/v1/operator-sessions", body, "", hh); rec.Code != 401 {
			t.Errorf("%s: %d %s — want the handler's 401, not a guard refusal", name, rec.Code, rec.Body)
		}
	}
	rec := h.doRaw("DELETE", "/api/v1/operator-sessions/current", "", opCookie(t), hdr("Origin", "https://"+operatorHostFake))
	if rec.Code != 204 {
		t.Errorf("bodyless sign-out from the console: %d %s", rec.Code, rec.Body)
	}
	// A bodyless DELETE from a commune page is still refused by Origin.
	if rec := h.doRaw("DELETE", "/api/v1/operator-sessions/current", "", opCookie(t),
		hdr("Origin", "https://thangbinh.vigov.vn")); rec.Code != 403 {
		t.Errorf("cross-origin bodyless DELETE: %d", rec.Code)
	}
	// Reads are not guarded: a GET changes nothing and the console's navigations carry no Origin.
	h.id.keys = []string{"ops.tenant.manage"}
	if rec := h.doRaw("GET", "/api/v1/communes", "", opCookie(t), http.Header{}); rec.Code != 200 {
		t.Errorf("GET without Origin: %d", rec.Code)
	}
}

// The realm boundary on the two account routes: a well-formed STAFF token is 401 and never reaches
// identity (rule 5 invariant 7, read for the operator realm — see operator_routes_test.go).
func TestAccountRoutesRefuseStaffToken(t *testing.T) {
	for _, rt := range []struct{ method, path, body string }{
		{"PUT", "/api/v1/operators/current/password", `{"current_password":"a","new_password":"b","totp_code":"000000"}`},
		{"POST", "/api/v1/operators/current/recovery-codes", `{"totp_code":"000000"}`},
	} {
		h := newHarness(t)
		for name, cookie := range map[string]string{"none": "", "forged": "op1.forged.AAAA", "staff": staffCookie(t)} {
			if rec := h.do(rt.method, rt.path, rt.body, cookie); rec.Code != 401 {
				t.Errorf("%s %s with %s cookie: %d, want 401", rt.method, rt.path, name, rec.Code)
			}
		}
		if h.id.calls != 0 {
			t.Errorf("%s %s: identity called %d times for tokens the signature refused", rt.method, rt.path, h.id.calls)
		}
	}
}

// POST /communes needs BOTH keys: missing either one — not only the second — is 403 and no write.
func TestCreateCommuneNeedsBothKeys(t *testing.T) {
	body := `{"name":"Xã Mới","province_id":"66QW36RCJ7GVW79W8GJRYH3ZNR","primary_domain":"xamoi.vigov.vn"}`
	for _, keys := range [][]string{{"ops.domain.manage"}, {"ops.tenant.manage"}, {}} {
		h := newHarness(t)
		h.id.keys = keys
		if rec := h.do("POST", "/api/v1/communes", body, opCookie(t)); rec.Code != 403 {
			t.Errorf("keys %v: %d, want 403", keys, rec.Code)
		}
		if h.w.target != "" {
			t.Errorf("keys %v: the write ran", keys)
		}
	}
}

// The primary-domain switch refuses a reserved host BEFORE the writer — the two frozen admin rows
// and OPERATOR_HOST are some commune's rows, so "it must be one of this commune's" is not the check —
// and a 0007 CHECK refusal from the store is 422 reserved_domain, never 500.
func TestPrimaryDomainRefusesReservedHost(t *testing.T) {
	path := "/api/v1/communes/" + communeIDFake + "/primary-domain"
	for _, host := range []string{"admin.vigov.vn", "ADMIN-STG.vigov.vn", operatorHostFake, "identity.api.vigov.vn"} {
		h := newHarness(t)
		h.id.keys = []string{"ops.domain.manage"}
		rec := h.do("PUT", path, `{"domain":"`+host+`"}`, opCookie(t))
		if rec.Code != 422 || !strings.Contains(rec.Body.String(), `"code":"reserved_domain"`) {
			t.Errorf("%s: %d %s, want 422 reserved_domain", host, rec.Code, rec.Body)
		}
		if h.w.target != "" {
			t.Errorf("%s: reached the writer", host)
		}
	}
	for _, rt := range []struct{ method, path, body string }{
		{"PUT", path, `{"domain":"thangbinh-danang.vigov.vn"}`},
		{"POST", "/api/v1/communes/" + communeIDFake + "/domains", `{"domain":"thangbinh.example.gov.vn"}`},
		{"POST", "/api/v1/communes", `{"name":"Xã Mới","province_id":"66QW36RCJ7GVW79W8GJRYH3ZNR","primary_domain":"xamoi.vigov.vn"}`},
	} {
		h := newHarness(t)
		h.id.keys = []string{"ops.domain.manage", "ops.tenant.manage"}
		h.w.err = domain.ErrCommuneHostReserved // what store.domainWriteError maps 0007's CHECK to
		if rec := h.do(rt.method, rt.path, rt.body, opCookie(t)); rec.Code != 422 ||
			!strings.Contains(rec.Body.String(), `"code":"reserved_domain"`) {
			t.Errorf("%s %s with the CHECK refusing: %d %s, want 422 reserved_domain", rt.method, rt.path, rec.Code, rec.Body)
		}
	}
}

// A commune with a successor is not switched back on: 409 commune_succeeded, not 500.
func TestActivationOfSucceededCommuneIs409(t *testing.T) {
	h := newHarness(t)
	h.id.keys = []string{"ops.tenant.manage"}
	h.w.err = store.ErrCommuneSucceeded
	rec := h.do("PUT", "/api/v1/communes/"+communeIDFake+"/activation", `{"active":true,"reason":"Mở lại"}`, opCookie(t))
	if rec.Code != 409 || !strings.Contains(rec.Body.String(), `"code":"commune_succeeded"`) {
		t.Fatalf("%d %s, want 409 commune_succeeded", rec.Code, rec.Body)
	}
	if len(h.forgot) != 0 {
		t.Error("a refused reactivation touched the cache")
	}
}
