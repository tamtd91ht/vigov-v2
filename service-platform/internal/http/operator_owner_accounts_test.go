package http

// The seven OWNER-ONLY account routes (ADR 0048 §Sửa đổi 13/10/2026 #2–#6).
//
// The operator realm's reading of rule 5 invariant 7 (operator_routes_test.go explains it): 401 no
// cookie · 401 staff token · 503 identity down · 403 a live NON-owner holding EVERY ops.* key · 2xx the
// owner holding no key at all. "Right permission, wrong commune" has no operator analogue; what stands
// in for it on the three commune routes is that the commune identity is asked about is the PATH's and
// nothing else's, and that an unknown or deleted commune never reaches identity.

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/operatorclient"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
)

// Fake material — says so in full (rule 8, forbidden #1; rule 3 invariant 5).
const (
	tempPasswordFake = "temp-password-FAKE-not-real"
	accountEmailFake = "nguoi-van-hanh@example.invalid"
)

type ownerAccountsFake struct {
	// outcome nil answers ACCEPTED with a complete account.
	outcome *operatorclient.Outcome
	err     error

	calls     int
	sawRPC    []string
	sawToken  []string
	sawIP     []string
	sawTenant []tenant.ID
	sawPass   []string
	sawReason []string
	sawTarget []string
	sawKeys   [][]string
}

func (f *identityFake) ownerCall(ctx context.Context, rpc string, tok secret.Secret, ip string) (operatorclient.Outcome, error) {
	a := &f.acct
	t, _ := tenant.From(ctx)
	a.calls++
	a.sawRPC = append(a.sawRPC, rpc)
	a.sawToken = append(a.sawToken, string(tok.Lo()))
	a.sawIP = append(a.sawIP, ip)
	a.sawTenant = append(a.sawTenant, t)
	if a.err != nil {
		return operatorclient.OutcomeRefused, a.err
	}
	if a.outcome != nil {
		return *a.outcome, nil
	}
	return operatorclient.OutcomeAccepted, nil
}

var refusalFake = operatorclient.NewPasswordRefusal{
	Problem: identityv1.NewPasswordProblem_NEW_PASSWORD_PROBLEM_TOO_SHORT, MinLength: 12, MaxLength: 128}

func opAccountViewFake(code string) operatorclient.OperatorAccount {
	return operatorclient.OperatorAccount{Code: code, Email: accountEmailFake, DisplayName: "Người FAKE",
		PermissionKeys: []string{"ops.qr.issue"}, MustChangePassword: true, CreatedAt: nowFake}
}

func staffViewFake(id string) operatorclient.CommuneStaffAccount {
	return operatorclient.CommuneStaffAccount{UserID: id, StaffCode: "CB-00001", Email: "admin", FullName: "Cán bộ FAKE",
		RoleCode: "quan-tri-he-thong", RoleName: "Quản trị hệ thống", IsAdmin: true, MustChangePassword: true}
}

func (f *identityFake) opAccountAnswer(o operatorclient.Outcome, code string) operatorclient.OperatorAccountResult {
	res := operatorclient.OperatorAccountResult{Outcome: o}
	switch o {
	case operatorclient.OutcomeAccepted:
		res.Account = opAccountViewFake(code)
	case operatorclient.OutcomeNewPasswordRejected:
		res.Refusal = refusalFake
	}
	return res
}

func (f *identityFake) staffAnswer(o operatorclient.Outcome, id string) operatorclient.StaffAccountResult {
	res := operatorclient.StaffAccountResult{Outcome: o}
	switch o {
	case operatorclient.OutcomeAccepted:
		res.Account = staffViewFake(id)
	case operatorclient.OutcomeNewPasswordRejected:
		res.Refusal = refusalFake
	}
	return res
}

func (f *identityFake) ListOperatorAccounts(ctx context.Context, tok secret.Secret) (operatorclient.OperatorAccountsResult, error) {
	o, err := f.ownerCall(ctx, "ListOperatorAccounts", tok, "")
	if err != nil {
		return operatorclient.OperatorAccountsResult{}, err
	}
	res := operatorclient.OperatorAccountsResult{Outcome: o}
	if o == operatorclient.OutcomeAccepted {
		owner := opAccountViewFake(opCodeFake)
		owner.IsOwner, owner.PermissionKeys = true, nil
		res.Accounts = []operatorclient.OperatorAccount{owner, opAccountViewFake("VH-00002")}
	}
	return res, nil
}

func (f *identityFake) CreateOperatorAccount(ctx context.Context, req operatorclient.CreateOperatorAccountRequest) (operatorclient.OperatorAccountResult, error) {
	o, err := f.ownerCall(ctx, "CreateOperatorAccount", req.Token, req.ClientIP)
	f.acct.sawPass = append(f.acct.sawPass, string(req.TemporaryPassword.Lo()))
	f.acct.sawKeys = append(f.acct.sawKeys, req.PermissionKeys)
	if err != nil {
		return operatorclient.OperatorAccountResult{}, err
	}
	return f.opAccountAnswer(o, "VH-00003"), nil
}

func (f *identityFake) SetOperatorPermissions(ctx context.Context, req operatorclient.SetOperatorPermissionsRequest) (operatorclient.OperatorAccountResult, error) {
	o, err := f.ownerCall(ctx, "SetOperatorPermissions", req.Token, req.ClientIP)
	f.acct.sawTarget = append(f.acct.sawTarget, req.TargetCode)
	f.acct.sawReason = append(f.acct.sawReason, req.Reason)
	f.acct.sawKeys = append(f.acct.sawKeys, req.PermissionKeys)
	if err != nil {
		return operatorclient.OperatorAccountResult{}, err
	}
	return f.opAccountAnswer(o, req.TargetCode), nil
}

func (f *identityFake) ResetOperatorPassword(ctx context.Context, req operatorclient.ResetOperatorPasswordRequest) (operatorclient.OperatorAccountResult, error) {
	o, err := f.ownerCall(ctx, "ResetOperatorPassword", req.Token, req.ClientIP)
	f.acct.sawTarget = append(f.acct.sawTarget, req.TargetCode)
	f.acct.sawReason = append(f.acct.sawReason, req.Reason)
	f.acct.sawPass = append(f.acct.sawPass, string(req.TemporaryPassword.Lo()))
	if err != nil {
		return operatorclient.OperatorAccountResult{}, err
	}
	return f.opAccountAnswer(o, req.TargetCode), nil
}

func (f *identityFake) ListCommuneStaffAccounts(ctx context.Context, tok secret.Secret, ip string) (operatorclient.StaffAccountsResult, error) {
	o, err := f.ownerCall(ctx, "ListCommuneStaffAccounts", tok, ip)
	if err != nil {
		return operatorclient.StaffAccountsResult{}, err
	}
	res := operatorclient.StaffAccountsResult{Outcome: o}
	if o == operatorclient.OutcomeAccepted {
		res.Accounts = []operatorclient.CommuneStaffAccount{staffViewFake("01JD8ZQK9M3NPXR7TVWYB2C4F0")}
	}
	return res, nil
}

func (f *identityFake) CreateCommuneAdminAccount(ctx context.Context, req operatorclient.CreateCommuneAdminAccountRequest) (operatorclient.StaffAccountResult, error) {
	o, err := f.ownerCall(ctx, "CreateCommuneAdminAccount", req.Token, req.ClientIP)
	f.acct.sawPass = append(f.acct.sawPass, string(req.TemporaryPassword.Lo()))
	if err != nil {
		return operatorclient.StaffAccountResult{}, err
	}
	return f.staffAnswer(o, "01JD8ZQK9M3NPXR7TVWYB2C4F1"), nil
}

func (f *identityFake) ResetStaffPassword(ctx context.Context, req operatorclient.ResetStaffPasswordRequest) (operatorclient.StaffAccountResult, error) {
	o, err := f.ownerCall(ctx, "ResetStaffPassword", req.Token, req.ClientIP)
	f.acct.sawTarget = append(f.acct.sawTarget, req.UserID)
	f.acct.sawReason = append(f.acct.sawReason, req.Reason)
	f.acct.sawPass = append(f.acct.sawPass, string(req.TemporaryPassword.Lo()))
	if err != nil {
		return operatorclient.StaffAccountResult{}, err
	}
	return f.staffAnswer(o, req.UserID), nil
}

// --- the routes -----------------------------------------------------------------------------------

const staffUserFake = "01JD8ZQK9M3NPXR7TVWYB2C4F0"

type ownerRoute struct {
	method, path, body string
	ok                 int
	commune            bool // carries the path's commune to identity
	setsPassword       bool
	notFound           string // the route's 404 code for ErrAccountNotFound
}

var ownerRoutes = []ownerRoute{
	{"GET", "/api/v1/operator-accounts", "", 200, false, false, "operator_account_not_found"},
	{"POST", "/api/v1/operator-accounts",
		`{"email":"` + accountEmailFake + `","display_name":"Người FAKE","temporary_password":"` + tempPasswordFake + `","permission_keys":["ops.qr.issue"]}`,
		201, false, true, "operator_account_not_found"},
	{"PUT", "/api/v1/operator-accounts/VH-00002/permissions", `{"permission_keys":["ops.tenant.manage"],"reason":"Chuyển việc"}`,
		200, false, false, "operator_account_not_found"},
	{"PUT", "/api/v1/operator-accounts/VH-00002/password", `{"temporary_password":"` + tempPasswordFake + `","reason":"Quên mật khẩu"}`,
		200, false, true, "operator_account_not_found"},
	{"GET", "/api/v1/communes/" + communeIDFake + "/staff-accounts", "", 200, true, false, "staff_account_not_found"},
	{"POST", "/api/v1/communes/" + communeIDFake + "/admin-accounts",
		`{"email":"admin","full_name":"Cán bộ FAKE","temporary_password":"` + tempPasswordFake + `"}`, 201, true, true, "staff_account_not_found"},
	{"PUT", "/api/v1/communes/" + communeIDFake + "/staff-accounts/" + staffUserFake + "/password",
		`{"temporary_password":"` + tempPasswordFake + `","reason":"Cán bộ quên mật khẩu"}`, 200, true, true, "staff_account_not_found"},
}

func TestOwnerAccountRoutesAreOwnerOnly(t *testing.T) {
	for _, rt := range ownerRoutes {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			h := newHarness(t)
			if rec := h.do(rt.method, rt.path, rt.body, ""); rec.Code != 401 {
				t.Errorf("no cookie: %d, want 401", rec.Code)
			}
			if rec := h.do(rt.method, rt.path, rt.body, staffCookie(t)); rec.Code != 401 {
				t.Errorf("staff token: %d, want 401", rec.Code)
			}
			h.id.down = true
			if rec := h.do(rt.method, rt.path, rt.body, opCookie(t)); rec.Code != 503 {
				t.Errorf("identity down: %d, want 503", rec.Code)
			}
			h.id.down = false

			// Every decided key, not the owner: refused before identity's account RPCs.
			h.id.keys = decidedKeysFake
			if rec := h.do(rt.method, rt.path, rt.body, opCookie(t)); rec.Code != 403 {
				t.Errorf("every ops key, not owner: %d, want 403", rec.Code)
			}
			if h.id.acct.calls != 0 {
				t.Fatal("a non-owner reached identity's account RPCs")
			}

			// The owner holding NO key: allowed — the flag alone decides.
			h.id.keys, h.id.owner = nil, true
			cookie := opCookie(t)
			rec := h.do(rt.method, rt.path, rt.body, cookie)
			if rec.Code != rt.ok {
				t.Fatalf("owner: %d, want %d (%s)", rec.Code, rt.ok, rec.Body)
			}
			if rec.Header().Get("Cache-Control") != "no-store" {
				t.Error("an account response must not be cached (personal data, rule 3)")
			}
			a := h.id.acct
			if a.calls != 1 || a.sawToken[0] != cookie {
				t.Errorf("identity calls %d, token forwarded verbatim: %v", a.calls, a.sawToken)
			}
			wantTenant := tenant.ID("")
			if rt.commune {
				wantTenant = communeIDFake
			}
			if a.sawTenant[0] != wantTenant {
				t.Errorf("commune sent to identity %q, want %q (the PATH's, or none)", a.sawTenant[0], wantTenant)
			}
			if rt.method != "GET" || rt.commune {
				if a.sawIP[0] == "" {
					t.Error("client IP not forwarded")
				}
			}
			if rt.setsPassword && (len(a.sawPass) != 1 || a.sawPass[0] != tempPasswordFake) {
				t.Errorf("temporary password did not travel verbatim: %v", a.sawPass)
			}
			// The answer never echoes the password, and nothing logged a credential or personal data.
			if strings.Contains(rec.Body.String(), tempPasswordFake) {
				t.Fatalf("the response echoes the temporary password: %s", rec.Body)
			}
			for _, leak := range []string{tempPasswordFake, accountEmailFake, "FAKE", "op1."} {
				if strings.Contains(h.logs.String(), leak) {
					t.Fatalf("the log carries %q: %s", leak, h.logs)
				}
			}
		})
	}
}

func outcomePtr(o operatorclient.Outcome) *operatorclient.Outcome { return &o }

func TestOwnerAccountRoutesMapIdentityAnswers(t *testing.T) {
	type answer struct {
		name    string
		outcome *operatorclient.Outcome
		err     error
		status  int
		code    string // "" = the route's own 404 code
	}
	answers := []answer{
		{"session not live", outcomePtr(operatorclient.OutcomeSessionNotLive), nil, 401, "unauthorized"},
		{"identity refuses the owner", outcomePtr(operatorclient.OutcomePermissionDenied), nil, 403, "forbidden"},
		{"UNSPECIFIED read as refused", outcomePtr(operatorclient.OutcomeRefused), nil, 503, "account_management_unavailable"},
		{"not found", nil, operatorclient.ErrAccountNotFound, 404, ""},
		{"exists", nil, operatorclient.ErrAccountExists, 409, "account_email_taken"},
		{"commune not live", nil, operatorclient.ErrCommuneNotLive, 409, "commune_inactive"},
		{"staff locked", nil, operatorclient.ErrStaffAccountLocked, 409, "staff_account_locked"},
		{"shape refused", nil, operatorclient.ErrArgumentRefused, 400, "invalid_account_input"},
		{"not implemented yet", nil, operatorclient.ErrNotSupported, 503, "account_management_not_ready"},
		{"contract fault", nil, operatorclient.ErrContract, 503, "account_management_unavailable"},
		{"realm not configured", nil, operatorclient.ErrRealmNotConfigured, 503, "account_management_unavailable"},
		{"outage", nil, errors.New("identity: unavailable (fake)"), 503, "account_management_unavailable"},
	}
	for _, rt := range ownerRoutes {
		for _, an := range answers {
			t.Run(rt.method+" "+rt.path+" / "+an.name, func(t *testing.T) {
				h := newHarness(t)
				h.id.owner = true
				h.id.acct.outcome, h.id.acct.err = an.outcome, an.err
				rec := h.do(rt.method, rt.path, rt.body, opCookie(t))
				code := an.code
				if code == "" {
					code = rt.notFound
				}
				if rec.Code != an.status || !strings.Contains(rec.Body.String(), `"code":"`+code+`"`) {
					t.Fatalf("status %d body %s; want %d %s", rec.Code, rec.Body, an.status, code)
				}
				if an.status == 401 && !strings.Contains(rec.Header().Get("Set-Cookie"), "Max-Age=0") {
					t.Error("a dead session must clear the cookie")
				}
			})
		}
		if !rt.setsPassword {
			continue
		}
		t.Run(rt.method+" "+rt.path+" / password rejected", func(t *testing.T) {
			h := newHarness(t)
			h.id.owner = true
			h.id.acct.outcome = outcomePtr(operatorclient.OutcomeNewPasswordRejected)
			rec := h.do(rt.method, rt.path, rt.body, opCookie(t))
			var v passwordRejectionView
			if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
				t.Fatal(err)
			}
			if rec.Code != 422 || v.Code != "new_password_rejected" || v.Problem != "too_short" || v.MinLength != 12 || v.MaxLength != 128 {
				t.Fatalf("%d %+v", rec.Code, v)
			}
		})
	}
}

func TestOwnerAccountRoutesValidateBodies(t *testing.T) {
	commune := "/api/v1/communes/" + communeIDFake
	cases := []struct {
		name, method, path, body string
		want                     int
		code                     string
	}{
		{"create: a commune in the body", "POST", "/api/v1/operator-accounts",
			`{"email":"a@example.invalid","display_name":"A","temporary_password":"x","tenant_id":"` + communeIDFake + `"}`, 400, "invalid_body"},
		{"create: password not a string", "POST", "/api/v1/operator-accounts",
			`{"email":"a@example.invalid","display_name":"A","temporary_password":12345678901234}`, 400, "invalid_body"},
		{"create: NUL in the password", "POST", "/api/v1/operator-accounts",
			`{"email":"a@example.invalid","display_name":"A","temporary_password":"abc\u0000def"}`, 400, "invalid_body"},
		{"create: is_owner is no field", "POST", "/api/v1/operator-accounts",
			`{"email":"a@example.invalid","display_name":"A","temporary_password":"x","is_owner":true}`, 400, "invalid_body"},
		{"permissions: field absent", "PUT", "/api/v1/operator-accounts/VH-00002/permissions", `{"reason":"x"}`, 400, "invalid_body"},
		{"permissions: blank reason", "PUT", "/api/v1/operator-accounts/VH-00002/permissions", `{"permission_keys":[],"reason":"  "}`, 422, "invalid_reason"},
		{"permissions: reason over 500", "PUT", "/api/v1/operator-accounts/VH-00002/permissions",
			`{"permission_keys":[],"reason":"` + strings.Repeat("a", 501) + `"}`, 422, "invalid_reason"},
		{"operator password: no reason", "PUT", "/api/v1/operator-accounts/VH-00002/password", `{"temporary_password":"x"}`, 422, "invalid_reason"},
		{"operator password: absurd code", "PUT", "/api/v1/operator-accounts/" + strings.Repeat("V", 65) + "/password",
			`{"temporary_password":"x","reason":"y"}`, 404, "operator_account_not_found"},
		{"staff list: malformed commune", "GET", "/api/v1/communes/not-a-ulid/staff-accounts", "", 404, "commune_not_found"},
		{"staff list: unknown or deleted commune", "GET", "/api/v1/communes/01JD8ZQK9M3NPXR7TVWYB2C4EH/staff-accounts", "", 404, "commune_not_found"},
		{"admin: unknown commune", "POST", "/api/v1/communes/01JD8ZQK9M3NPXR7TVWYB2C4EH/admin-accounts",
			`{"email":"admin","full_name":"A","temporary_password":"x"}`, 404, "commune_not_found"},
		{"admin: a role in the body", "POST", commune + "/admin-accounts",
			`{"email":"admin","full_name":"A","temporary_password":"x","role_code":"quan-tri-he-thong"}`, 400, "invalid_body"},
		{"staff password: reason over 500", "PUT", commune + "/staff-accounts/" + staffUserFake + "/password",
			`{"temporary_password":"x","reason":"` + strings.Repeat("a", 501) + `"}`, 422, "invalid_reason"},
		{"staff password: unknown commune", "PUT", "/api/v1/communes/01JD8ZQK9M3NPXR7TVWYB2C4EH/staff-accounts/" + staffUserFake + "/password",
			`{"temporary_password":"x","reason":"y"}`, 404, "commune_not_found"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t)
			h.id.owner = true
			rec := h.do(c.method, c.path, c.body, opCookie(t))
			if rec.Code != c.want || !strings.Contains(rec.Body.String(), `"code":"`+c.code+`"`) {
				t.Fatalf("status %d body %s; want %d %s", rec.Code, rec.Body, c.want, c.code)
			}
			if h.id.acct.calls != 0 {
				t.Errorf("a refused request reached identity (%v)", h.id.acct.sawRPC)
			}
		})
	}

	// An EMPTY key list is meaningful — revoke every key — and travels as such.
	h := newHarness(t)
	h.id.owner = true
	rec := h.do("PUT", "/api/v1/operator-accounts/VH-00002/permissions", `{"permission_keys":[],"reason":" Thôi việc "}`, opCookie(t))
	if rec.Code != 200 || len(h.id.acct.sawKeys) != 1 || h.id.acct.sawKeys[0] == nil || len(h.id.acct.sawKeys[0]) != 0 {
		t.Fatalf("empty set: %d keys %v (%s)", rec.Code, h.id.acct.sawKeys, rec.Body)
	}
	if h.id.acct.sawTarget[0] != "VH-00002" || h.id.acct.sawReason[0] != "Thôi việc" {
		t.Errorf("target %v reason %v — the path's code, the trimmed reason", h.id.acct.sawTarget, h.id.acct.sawReason)
	}

	// An inactive commune gets no administrator: refused here, identity never asked.
	h = newHarness(t)
	h.id.owner = true
	h.reg.inactive = true
	rec = h.do("POST", commune+"/admin-accounts", `{"email":"admin","full_name":"A","temporary_password":"x"}`, opCookie(t))
	if rec.Code != 409 || !strings.Contains(rec.Body.String(), `"code":"commune_inactive"`) || h.id.acct.calls != 0 {
		t.Fatalf("inactive commune: %d %s, identity calls %d", rec.Code, rec.Body, h.id.acct.calls)
	}
	// ... but its accounts may still be listed and reset (the commune's access, not its records).
	rec = h.do("GET", commune+"/staff-accounts", "", opCookie(t))
	if rec.Code != 200 {
		t.Errorf("listing an inactive commune's accounts: %d", rec.Code)
	}
}

// The response shapes the platform-admin console is built against.
func TestOwnerAccountViews(t *testing.T) {
	h := newHarness(t)
	h.id.owner = true
	rec := h.do("GET", "/api/v1/operator-accounts", "", opCookie(t))
	var ops operatorAccountListView
	if err := json.Unmarshal(rec.Body.Bytes(), &ops); err != nil {
		t.Fatal(err)
	}
	if len(ops.Items) != 2 || !ops.Items[0].IsOwner || ops.Items[0].PermissionKeys == nil || ops.Items[1].Code != "VH-00002" {
		t.Errorf("operator accounts %+v", ops.Items)
	}
	for _, field := range []string{`"locked_until":null`, `"created_at":"2026-10-01T08:00:00Z"`, `"must_change_password":true`} {
		if !strings.Contains(rec.Body.String(), field) {
			t.Errorf("operator account view lacks %s: %s", field, rec.Body)
		}
	}
	for _, absent := range []string{"operator_id", "op-internal-1", `"password"`, `"temporary_password"`, "totp"} {
		if strings.Contains(rec.Body.String(), absent) {
			t.Errorf("operator account view carries %q: %s", absent, rec.Body)
		}
	}

	rec = h.do("GET", "/api/v1/communes/"+communeIDFake+"/staff-accounts", "", opCookie(t))
	var staff staffAccountListView
	if err := json.Unmarshal(rec.Body.Bytes(), &staff); err != nil {
		t.Fatal(err)
	}
	if len(staff.Items) != 1 || staff.Items[0].UserID != staffUserFake || !staff.Items[0].IsAdmin || staff.Items[0].StaffCode != "CB-00001" {
		t.Errorf("staff accounts %+v", staff.Items)
	}
	if !strings.Contains(rec.Body.String(), `"sign_in_locked_until":null`) || strings.Contains(rec.Body.String(), "tenant_id") {
		t.Errorf("staff account view: %s", rec.Body)
	}
}

// GET /operator-sessions/current says whether the operator is THE owner — display only.
func TestWhoAmIStatesOwner(t *testing.T) {
	for _, owner := range []bool{false, true} {
		h := newHarness(t)
		h.id.owner = owner
		rec := h.do("GET", "/api/v1/operator-sessions/current", "", opCookie(t))
		var v operatorWhoAmIView
		if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
			t.Fatal(err)
		}
		if rec.Code != 200 || v.IsOwner != owner || !strings.Contains(rec.Body.String(), `"is_owner":`) {
			t.Errorf("owner=%v: %d %s", owner, rec.Code, rec.Body)
		}
	}
}
