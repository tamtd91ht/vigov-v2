package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// WHAT THIS FILE IS FOR: the three role write routes (owner decision 15/10/2026). Rule 5, invariant 7
// for each — 401 · 403 wrong permission · 403 right permission wrong commune · 2xx — plus the wire
// shapes, every refusal's status and code, and which of the principal's identifiers reaches the use
// case. The rules themselves are proved in app/role_write_test.go and against PostgreSQL in
// store/role_write_pg_test.go.

type rolesFake struct {
	mu        sync.Mutex
	calls     int
	lastActor app.NguoiThucHien
	lastXa    tenant.ID
	lastID    string
	lastIn    app.RoleInput
	lastWhy   string

	role domain.VaiTro
	del  app.RoleDeletion
	err  error
}

func rolesSample() *rolesFake {
	return &rolesFake{
		role: domain.VaiTro{ID: "vt-bi-thu", Ma: "bi-thu", Ten: "Bí thư", LaLanhDao: true},
		del: app.RoleDeletion{
			Role:      domain.VaiTro{ID: "vt-bi-thu", Ma: "bi-thu", Ten: "Bí thư"},
			DeletedAt: time.Date(2026, 10, 15, 3, 0, 0, 0, time.UTC),
			DeletedBy: maCanBo,
		},
	}
}

func (f *rolesFake) record(ctx context.Context, id string, in app.RoleInput, why string, actor app.NguoiThucHien) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.lastActor, f.lastXa, f.lastID, f.lastIn, f.lastWhy = actor, tenant.MustFrom(ctx), id, in, why
}

func (f *rolesFake) Add(ctx context.Context, in app.RoleInput, actor app.NguoiThucHien) (domain.VaiTro, error) {
	f.record(ctx, "", in, "", actor)
	return f.role, f.err
}

func (f *rolesFake) Edit(ctx context.Context, id string, in app.RoleInput, actor app.NguoiThucHien) (domain.VaiTro, error) {
	f.record(ctx, id, in, "", actor)
	return f.role, f.err
}

func (f *rolesFake) Remove(ctx context.Context, id, reason string, actor app.NguoiThucHien) (app.RoleDeletion, error) {
	f.record(ctx, id, app.RoleInput{}, reason, actor)
	return f.del, f.err
}

func (f *rolesFake) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// newRolesServer grants `admin.role` in commune A and nothing in commune B.
func newRolesServer(t *testing.T) *mayChu {
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

type roleRoute struct {
	name, method, path, body string
	ok                       int
}

var roleRoutes = []roleRoute{
	{"create", "POST", "/api/v1/roles", `{"name":"Bí thư","is_leader":true}`, http.StatusCreated},
	{"edit", "PUT", "/api/v1/roles/vt-bi-thu", `{"name":"Bí thư Đảng uỷ","is_leader":true}`, http.StatusOK},
	{"deletion", "POST", "/api/v1/roles/vt-bi-thu/deletion", `{"reason":"Không còn chức danh này"}`, http.StatusCreated},
}

func TestRoleWrites_401NoToken(t *testing.T) {
	for _, rt := range roleRoutes {
		m := newRolesServer(t)
		w := m.goi(t, rt.method, hostA, rt.path, rt.body, "")
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s: mã = %d, muốn 401 — thân: %s", rt.name, w.Code, w.Body.String())
		}
		if n := m.roles.callCount(); n != 0 {
			t.Errorf("%s: chưa đăng nhập mà use case đã chạy %d lần", rt.name, n)
		}
	}
}

// The shipped default grants `admin.user` — a real key with nothing to do with roles.
func TestRoleWrites_403WrongPermission(t *testing.T) {
	for _, rt := range roleRoutes {
		m := dungMayChu(t)
		w := m.goi(t, rt.method, hostA, rt.path, rt.body, m.tokenCho(t, xaA, sidA))
		if w.Code != http.StatusForbidden {
			t.Errorf("%s: mã = %d, muốn 403 — thân: %s", rt.name, w.Code, w.Body.String())
		}
		if n := m.roles.callCount(); n != 0 {
			t.Errorf("%s: sai quyền mà use case đã chạy %d lần", rt.name, n)
		}
	}
}

func TestRoleWrites_403RightPermissionWrongCommune(t *testing.T) {
	for _, rt := range roleRoutes {
		m := newRolesServer(t)
		w := m.goi(t, rt.method, hostB, rt.path, rt.body, m.tokenCho(t, xaB, sidB))
		if w.Code != http.StatusForbidden {
			t.Errorf("%s: mã = %d, muốn 403 — thân: %s", rt.name, w.Code, w.Body.String())
		}
		if n := m.roles.callCount(); n != 0 {
			t.Errorf("%s: sai xã mà use case đã chạy %d lần", rt.name, n)
		}
	}
}

func TestRoleWrites_2xxBothCorrect(t *testing.T) {
	for _, rt := range roleRoutes {
		m := newRolesServer(t)
		w := m.goi(t, rt.method, hostA, rt.path, rt.body, m.tokenCho(t, xaA, sidA))
		if w.Code != rt.ok {
			t.Fatalf("%s: mã = %d, muốn %d — thân: %s", rt.name, w.Code, rt.ok, w.Body.String())
		}
		f := m.roles
		if f.lastXa != xaA {
			t.Errorf("%s: use case chạy ở xã %q, muốn %q", rt.name, f.lastXa, xaA)
		}
		// Rule 6, invariant 8: the staff code records, the internal id decides.
		if f.lastActor.Vet.ID != maCanBo || f.lastActor.ID != idNoiBo {
			t.Errorf("%s: người thực hiện = {ID:%q Vet:%q}", rt.name, f.lastActor.ID, f.lastActor.Vet.ID)
		}
	}
}

// Create and edit answer with ONE ITEM of GET /api/v1/roles — the same four fields, nothing else.
func TestRoleCreateAndEditAnswerTheCatalogueItemShape(t *testing.T) {
	for _, rt := range roleRoutes[:2] {
		m := newRolesServer(t)
		w := m.goi(t, rt.method, hostA, rt.path, rt.body, m.tokenCho(t, xaA, sidA))
		var got map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("%s: thân không đọc được: %v — %s", rt.name, err, w.Body.String())
		}
		want := map[string]any{"id": "vt-bi-thu", "code": "bi-thu", "name": "Bí thư", "is_leader": true}
		if len(got) != len(want) {
			t.Errorf("%s: thân = %v, muốn đúng 4 trường %v", rt.name, got, want)
		}
		for k, v := range want {
			if got[k] != v {
				t.Errorf("%s: %s = %v, muốn %v", rt.name, k, got[k], v)
			}
		}
	}
	// The body reaches the use case as sent; the path id reaches edit.
	m := newRolesServer(t)
	m.goi(t, "PUT", hostA, "/api/v1/roles/vt-bi-thu", `{"name":"Bí thư Đảng uỷ","is_leader":false}`, m.tokenCho(t, xaA, sidA))
	if f := m.roles; f.lastID != "vt-bi-thu" || f.lastIn.Name != "Bí thư Đảng uỷ" || f.lastIn.IsLeader {
		t.Errorf("use case nhận id=%q in=%+v", f.lastID, f.lastIn)
	}
}

func TestRoleDeletionAnswersTheRecordOfTheAct(t *testing.T) {
	m := newRolesServer(t)
	w := m.goi(t, "POST", hostA, "/api/v1/roles/vt-bi-thu/deletion", `{"reason":"Không còn chức danh này"}`,
		m.tokenCho(t, xaA, sidA))
	if w.Code != http.StatusCreated {
		t.Fatalf("mã = %d — %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{`"id":"vt-bi-thu"`, `"code":"bi-thu"`, `"name":"Bí thư"`,
		`"deleted_at":"2026-10-15T03:00:00Z"`, `"deleted_by":"` + maCanBo + `"`} {
		if !strings.Contains(body, want) {
			t.Errorf("thân thiếu %s: %s", want, body)
		}
	}
	if f := m.roles; f.lastID != "vt-bi-thu" || f.lastWhy != "Không còn chức danh này" {
		t.Errorf("use case nhận id=%q lý do=%q", f.lastID, f.lastWhy)
	}
}

// A missing is_leader is refused, never read as false — and the use case is never reached.
func TestRoleWriteMissingLeaderFlagIs400(t *testing.T) {
	for _, rt := range roleRoutes[:2] {
		m := newRolesServer(t)
		w := m.goi(t, rt.method, hostA, rt.path, `{"name":"Bí thư"}`, m.tokenCho(t, xaA, sidA))
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "invalid_request") {
			t.Errorf("%s: mã = %d — %s", rt.name, w.Code, w.Body.String())
		}
		if m.roles.callCount() != 0 {
			t.Errorf("%s: use case vẫn chạy", rt.name)
		}
	}
}

func TestRoleWriteRefusalsMapToStatusAndCode(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"not found", idstore.ErrVaiTroKhongTonTaiDeGhi, 404, "role_not_found"},
		{"system role", app.ErrSystemRole, 409, "system_role"},
		{"in use", &app.RoleInUseError{Holders: 3}, 409, "role_in_use"},
		{"duplicate", domain.ErrRoleNameTaken, 409, "duplicate_name"},
		{"bad name", domain.ErrRoleNameInvalid, 422, "invalid_name"},
		{"bad reason", domain.ErrRoleDeleteReasonInvalid, 422, "invalid_reason"},
		{"full", app.ErrRoleCatalogueFull, 409, "role_limit_reached"},
		{"code race", idstore.ErrRoleCodeTaken, 409, "role_code_taken"},
	}
	for _, c := range cases {
		for _, rt := range roleRoutes {
			m := newRolesServer(t)
			m.roles.err = c.err
			w := m.goi(t, rt.method, hostA, rt.path, rt.body, m.tokenCho(t, xaA, sidA))
			if w.Code != c.status || !strings.Contains(w.Body.String(), `"code":"`+c.code+`"`) {
				t.Errorf("%s / %s: mã = %d — %s, muốn %d %s", c.name, rt.name, w.Code, w.Body.String(), c.status, c.code)
			}
		}
	}
	// role_in_use names the count.
	m := newRolesServer(t)
	m.roles.err = &app.RoleInUseError{Holders: 3}
	w := m.goi(t, "POST", hostA, "/api/v1/roles/vt-bi-thu/deletion", `{"reason":"x"}`, m.tokenCho(t, xaA, sidA))
	if !strings.Contains(w.Body.String(), "Còn 3 cán bộ") {
		t.Errorf("role_in_use phải nêu số cán bộ: %s", w.Body.String())
	}
}
