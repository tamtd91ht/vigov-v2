package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/app"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// WHAT THIS FILE IS FOR: POST /api/v1/roles/defaults. Rule 5, invariant 7 — 401 · 403 wrong
// permission · 403 right permission wrong commune · 200 — plus the wire shape, the two refusal
// mappings and which of the principal's identifiers reaches the use case. The seeding rules
// themselves are proved against a real transaction in app/role_template_test.go.

type roleTemplatesFake struct {
	mu        sync.Mutex
	calls     int
	lastActor app.NguoiThucHien
	lastXa    tenant.ID

	res app.RoleTemplateResult
	err error
}

func roleTemplatesSample() *roleTemplatesFake {
	return &roleTemplatesFake{res: app.RoleTemplateResult{
		Created:         []app.RoleTemplateRef{{Code: "chu-tich-ubnd", Name: "Chủ tịch UBND"}},
		SkippedExisting: []app.RoleTemplateRef{{Code: "ke-toan", Name: "Kế toán"}},
	}}
}

func (f *roleTemplatesFake) SeedDefaults(ctx context.Context, actor app.NguoiThucHien) (app.RoleTemplateResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.lastActor = actor
	f.lastXa = tenant.MustFrom(ctx)
	return f.res, f.err
}

func (f *roleTemplatesFake) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

const pathRoleDefaults = "/api/v1/roles/defaults"

// newRoleTemplateServer grants `admin.role` in commune A and nothing in commune B.
func newRoleTemplateServer(t *testing.T) *mayChu {
	t.Helper()
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
			xaA: {idNoiBo: {authz.Perm("admin.role"): true}},
			xaB: {},
		}}
	})
	return m
}

func TestSeedRoleTemplates_401NoToken(t *testing.T) {
	m := newRoleTemplateServer(t)
	w := m.goi(t, "POST", hostA, pathRoleDefaults, "", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("mã = %d, muốn 401 — thân: %s", w.Code, w.Body.String())
	}
	if n := m.roleTemplates.callCount(); n != 0 {
		t.Errorf("chưa đăng nhập mà use case đã chạy %d lần", n)
	}
}

// The shipped default grants `admin.user` — a real key with nothing to do with permissions.
func TestSeedRoleTemplates_403WrongPermission(t *testing.T) {
	m := dungMayChu(t)
	w := m.goi(t, "POST", hostA, pathRoleDefaults, "", m.tokenCho(t, xaA, sidA))
	if w.Code != http.StatusForbidden {
		t.Fatalf("mã = %d, muốn 403 — thân: %s", w.Code, w.Body.String())
	}
	if n := m.roleTemplates.callCount(); n != 0 {
		t.Errorf("sai quyền mà use case đã chạy %d lần", n)
	}
}

func TestSeedRoleTemplates_403RightPermissionWrongCommune(t *testing.T) {
	m := newRoleTemplateServer(t)
	w := m.goi(t, "POST", hostB, pathRoleDefaults, "", m.tokenCho(t, xaB, sidB))
	if w.Code != http.StatusForbidden {
		t.Fatalf("mã = %d, muốn 403 — thân: %s", w.Code, w.Body.String())
	}
	if n := m.roleTemplates.callCount(); n != 0 {
		t.Errorf("sai xã mà use case đã chạy %d lần", n)
	}
}

func TestSeedRoleTemplates_200BothCorrect(t *testing.T) {
	m := newRoleTemplateServer(t)
	w := m.goi(t, "POST", hostA, pathRoleDefaults, "", m.tokenCho(t, xaA, sidA))
	if w.Code != http.StatusOK {
		t.Fatalf("mã = %d, muốn 200 — thân: %s", w.Code, w.Body.String())
	}
	var out struct {
		Created         []map[string]string `json:"created"`
		SkippedExisting []map[string]string `json:"skipped_existing"`
		SkippedDeleted  []map[string]string `json:"skipped_deleted"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("thân không đọc được: %v — %s", err, w.Body.String())
	}
	if len(out.Created) != 1 || out.Created[0]["code"] != "chu-tich-ubnd" || out.Created[0]["name"] != "Chủ tịch UBND" {
		t.Errorf("created = %v", out.Created)
	}
	if len(out.SkippedExisting) != 1 || out.SkippedExisting[0]["code"] != "ke-toan" {
		t.Errorf("skipped_existing = %v", out.SkippedExisting)
	}
	// An empty list is [] on the wire, never null.
	if !strings.Contains(w.Body.String(), `"skipped_deleted":[]`) {
		t.Errorf("skipped_deleted phải là [] khi rỗng — thân: %s", w.Body.String())
	}
	f := m.roleTemplates
	if f.lastXa != xaA {
		t.Errorf("use case chạy ở xã %q, muốn %q", f.lastXa, xaA)
	}
	// Rule 6, invariant 8: the staff code records, the internal id decides.
	if f.lastActor.Vet.ID != maCanBo || f.lastActor.ID != idNoiBo {
		t.Errorf("người thực hiện = {ID:%q Vet:%q}, muốn {ID:%q Vet:%q}",
			f.lastActor.ID, f.lastActor.Vet.ID, idNoiBo, maCanBo)
	}
}

func TestSeedRoleTemplatesMissingKeyIs403AndNamesTheKey(t *testing.T) {
	m := newRoleTemplateServer(t)
	m.roleTemplates.err = &app.LoiTraoQuyenKhongCam{Thieu: []string{"budget.confirm"}}
	w := m.goi(t, "POST", hostA, pathRoleDefaults, "", m.tokenCho(t, xaA, sidA))
	if w.Code != http.StatusForbidden {
		t.Fatalf("mã = %d, muốn 403 — thân: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "permission_escalation") || !strings.Contains(w.Body.String(), "budget.confirm") {
		t.Errorf("thân phải mang mã permission_escalation và tên khoá thiếu: %s", w.Body.String())
	}
}

func TestSeedRoleTemplatesLostRaceIs409(t *testing.T) {
	m := newRoleTemplateServer(t)
	m.roleTemplates.err = idstore.ErrRoleCodeTaken
	w := m.goi(t, "POST", hostA, pathRoleDefaults, "", m.tokenCho(t, xaA, sidA))
	if w.Code != http.StatusConflict {
		t.Fatalf("mã = %d, muốn 409 — thân: %s", w.Code, w.Body.String())
	}
}
