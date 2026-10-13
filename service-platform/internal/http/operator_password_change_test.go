package http

// ADR 0048 §13/10 #6: a session opened with a temporary password the owner set may call who-am-I,
// sign-out and change-own-password — nothing else, whatever keys it holds and even as the owner.

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/operatorclient"
)

// allowedWhilePasswordChange is THE list, spelled out independently of operator_routes.go. Growing it
// is an owner decision (ADR 0048 §13/10 #6), never a convenience.
var allowedWhilePasswordChange = []string{
	"GET /api/v1/operator-sessions/current",
	"DELETE /api/v1/operator-sessions/current",
	"PUT /api/v1/operators/current/password",
}

type routeDecl struct {
	pattern string
	public  bool // opauth.Public
	guarded bool // opauth.SignedIn or opauth.RequireKey
	// exempt: opauth.AllowWhilePasswordChangeRequired as the DIRECT argument of opauth.SignedIn(…).
	exempt bool
	// exemptElsewhere: the call appears in the statement but not in that position — inert at runtime
	// (SignedIn would not see it), and a sign somebody misunderstood the declaration.
	exemptElsewhere bool
}

func opauthName(e ast.Expr) string {
	c, ok := e.(*ast.CallExpr)
	if !ok {
		return ""
	}
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	if id, ok := sel.X.(*ast.Ident); ok && id.Name == "opauth" {
		return sel.Sel.Name
	}
	return ""
}

// operatorRouteDecls reads every mux.Handle statement in registerOperator — the source, so a route
// added tomorrow is in this table without anybody remembering to list it.
func operatorRouteDecls(t *testing.T) []routeDecl {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "operator_routes.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var out []routeDecl
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "registerOperator" {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 2 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Handle" {
				return true
			}
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok {
				t.Errorf("mux.Handle at %s: pattern is not a literal", fset.Position(call.Pos()))
				return false
			}
			pattern, _ := strconv.Unquote(lit.Value)
			rd := routeDecl{pattern: pattern}
			anyExempt := false
			ast.Inspect(call.Args[1], func(n ast.Node) bool {
				c, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				switch opauthName(c) {
				case "Public":
					rd.public = true
				case "SignedIn", "RequireKey":
					rd.guarded = true
				case "AllowWhilePasswordChangeRequired":
					anyExempt = true
				}
				// opauth.SignedIn(a, why)(<arg>): the exemption counts only as that arg.
				if inner, ok := c.Fun.(*ast.CallExpr); ok && opauthName(inner) == "SignedIn" && len(c.Args) == 1 {
					if a, ok := c.Args[0].(*ast.CallExpr); ok {
						if f, ok := a.Fun.(*ast.CallExpr); ok && opauthName(f) == "AllowWhilePasswordChangeRequired" {
							rd.exempt = true
						}
					}
				}
				return true
			})
			rd.exemptElsewhere = anyExempt && !rd.exempt
			out = append(out, rd)
			return false
		})
	}
	if len(out) < 40 {
		t.Fatalf("read %d operator routes from operator_routes.go — the reader is broken, not the routes", len(out))
	}
	return out
}

// TestPasswordChangeRequiredRouteTable: every operator route is Public, or guarded and refusing a
// restricted session, or one of the three declared exemptions — checked against the source AND
// against the real mux, with the widest principal there is (every key, the owner).
func TestPasswordChangeRequiredRouteTable(t *testing.T) {
	var exempt []string
	checked := map[string]bool{}
	param := regexp.MustCompile(`\{[a-z_]+\}`)

	for _, rd := range operatorRouteDecls(t) {
		if rd.exemptElsewhere {
			t.Errorf("%s: AllowWhilePasswordChangeRequired must be the direct argument of opauth.SignedIn", rd.pattern)
		}
		if rd.exempt {
			exempt = append(exempt, rd.pattern)
		}
		if rd.public {
			if rd.guarded || rd.exempt {
				t.Errorf("%s: Public together with a session guard", rd.pattern)
			}
			continue
		}
		if !rd.guarded {
			t.Errorf("%s: no opauth.SignedIn / RequireKey / Public declaration", rd.pattern)
			continue
		}
		if rd.exempt {
			continue // exercised below with real bodies
		}

		method, path, _ := strings.Cut(rd.pattern, " ")
		path = param.ReplaceAllString(path, "x1")
		h := newHarness(t)
		h.id.keys, h.id.owner, h.id.mustChange = decidedKeysFake, true, true
		body := ""
		if method != "GET" {
			body = "{}"
		}
		rec := h.do(method, path, body, opCookie(t))
		var e struct{ Code, Message string }
		_ = json.Unmarshal(rec.Body.Bytes(), &e)
		if rec.Code != 403 || e.Code != "password_change_required" || e.Message != "Bạn cần đổi mật khẩu trước khi tiếp tục." {
			t.Errorf("%s: %d %s, want 403 password_change_required", rd.pattern, rec.Code, rec.Body)
		}
		// Refused at the guard: the only identity call is the session resolve; nothing was written.
		if h.id.calls != 1 || h.w.target != "" || h.zalo.calls != 0 || h.fields.writes != 0 || h.frames.writes != 0 {
			t.Errorf("%s: work ran behind the refusal (identity calls %d, write target %q)", rd.pattern, h.id.calls, h.w.target)
		}
		checked[rd.pattern] = true
	}

	slices.Sort(exempt)
	want := slices.Clone(allowedWhilePasswordChange)
	slices.Sort(want)
	if !slices.Equal(exempt, want) {
		t.Fatalf("routes exempt from the password change: %v, want exactly %v", exempt, want)
	}
	// The sample the decision names explicitly is in the table: owner-only acts, deletion, keyed writes,
	// AnyKey reads, and the recovery codes route a restricted session must not reach either.
	for _, p := range []string{
		"POST /api/v1/communes/{id}/deletion",
		"GET /api/v1/operator-accounts",
		"PUT /api/v1/operator-accounts/{code}/password",
		"PUT /api/v1/communes/{id}/staff-accounts/{user_id}/password",
		"POST /api/v1/communes",
		"GET /api/v1/communes",
		"PUT /api/v1/zalo-bots/shared",
		"POST /api/v1/operators/current/recovery-codes",
	} {
		if !checked[p] {
			t.Errorf("%s was not checked against a restricted session", p)
		}
	}
}

// The three exemptions work for a restricted session, owner or not, with or without keys.
func TestPasswordChangeRequiredAllowsTheThree(t *testing.T) {
	h := newHarness(t)
	h.id.keys, h.id.mustChange = []string{"ops.tenant.manage"}, true

	rec := h.do("GET", "/api/v1/operator-sessions/current", "", opCookie(t))
	const wantWhoAmI = `{"operator_code":"VH-00001","permission_keys":["ops.tenant.manage"],"is_owner":false,"must_change_password":true}`
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != wantWhoAmI {
		t.Fatalf("who am I: %d %s, want %s", rec.Code, rec.Body, wantWhoAmI)
	}

	h.id.change = operatorclient.ChangePasswordResult{Outcome: operatorclient.OutcomeAccepted}
	rec = h.do("PUT", "/api/v1/operators/current/password",
		`{"current_password":"a","new_password":"b","totp_code":"000000"}`, opCookie(t))
	if rec.Code != 204 {
		t.Fatalf("change password: %d %s", rec.Code, rec.Body)
	}

	rec = h.do("DELETE", "/api/v1/operator-sessions/current", "", opCookie(t))
	if rec.Code != 204 || h.id.revoked != 1 {
		t.Fatalf("sign out: %d revoked %d", rec.Code, h.id.revoked)
	}
}

// An unrestricted session is untouched: who-am-I says false and a keyed route still answers.
func TestPasswordChangeNotRequiredIsUnaffected(t *testing.T) {
	h := newHarness(t)
	h.id.keys = []string{"ops.tenant.manage"}
	rec := h.do("GET", "/api/v1/operator-sessions/current", "", opCookie(t))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"must_change_password":false`) {
		t.Fatalf("who am I: %d %s", rec.Code, rec.Body)
	}
	if rec := h.do("GET", "/api/v1/communes", "", opCookie(t)); rec.Code != 200 {
		t.Fatalf("list communes: %d %s", rec.Code, rec.Body)
	}
}
