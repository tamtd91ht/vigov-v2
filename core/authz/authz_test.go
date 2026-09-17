package authz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vihat/vigov/core/tenant"
)

const (
	xaA = "01JD8ZQK9M3NPXR7TVWYB2C4EF"
	xaB = "01JD8ZQK9M3NPXR7TVWYB2C4EG"
)

// checkerGia grants exactly the permissions it is given, and records what it was asked.
type checkerGia struct {
	co    map[Perm]bool
	hoiGi []Perm
}

func (c *checkerGia) Allows(_ context.Context, _ Principal, p Perm) bool {
	c.hoiGi = append(c.hoiGi, p)
	return c.co[p]
}

// dungRequest builds a request whose context carries the commune from Host and, when p is not
// the zero value, a signed-in principal.
func dungRequest(xa tenant.ID, p *Principal) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/nhiem-vu", nil)
	ctx := tenant.Into(r.Context(), xa)
	if p != nil {
		ctx = Into(ctx, *p)
	}
	return r.WithContext(ctx)
}

func chay(h http.Handler, r *http.Request) int {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w.Code
}

// Rule 5, invariant 7 requires these four cases on every endpoint. They are written here once,
// against the middleware itself, so a route only has to repeat them for its own permission.

func TestKhongCoTokenTraVe401(t *testing.T) {
	c := &checkerGia{co: map[Perm]bool{"task.read": true}}
	h := RequirePermission(c, "task.read")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	if got := chay(h, dungRequest(tenant.ID(xaA), nil)); got != http.StatusUnauthorized {
		t.Errorf("không token: mã = %d, muốn 401", got)
	}
}

func TestSaiQuyenTraVe403(t *testing.T) {
	c := &checkerGia{co: map[Perm]bool{"task.read": true}}
	h := RequirePermission(c, "task.approve")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	p := Principal{ID: "CB001", Kind: "staff", TenantID: tenant.ID(xaA)}
	if got := chay(h, dungRequest(tenant.ID(xaA), &p)); got != http.StatusForbidden {
		t.Errorf("thiếu quyền: mã = %d, muốn 403", got)
	}
}

func TestDungQuyenNhungSaiXaTraVe401(t *testing.T) {
	// A token issued for commune A arriving at commune B's domain. Browsers do not send
	// cookies across hosts, so this is a deliberate probe or a stolen token — not an ordinary
	// user error, and never a 403 (which would confirm the permission exists).
	c := &checkerGia{co: map[Perm]bool{"task.read": true}}
	h := RequirePermission(c, "task.read")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	p := Principal{ID: "CB001", Kind: "staff", TenantID: tenant.ID(xaB)} // token của xã B
	if got := chay(h, dungRequest(tenant.ID(xaA), &p)); got != http.StatusUnauthorized {
		t.Errorf("token lệch xã: mã = %d, muốn 401", got)
	}
	if len(c.hoiGi) != 0 {
		t.Error("đã hỏi Checker trước khi kiểm xã — lệch xã phải chặn TRƯỚC khi xét quyền")
	}
}

func TestDungCaQuyenVaXaTraVe200(t *testing.T) {
	c := &checkerGia{co: map[Perm]bool{"task.read": true}}
	h := RequirePermission(c, "task.read")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	p := Principal{ID: "CB001", Kind: "staff", TenantID: tenant.ID(xaA)}
	if got := chay(h, dungRequest(tenant.ID(xaA), &p)); got != http.StatusOK {
		t.Errorf("đúng quyền đúng xã: mã = %d, muốn 200", got)
	}
}

func TestQuyenLaKhoaNguyenVen(t *testing.T) {
	// "task.approve" and "task.extend" are separate rights: one closes a commitment to a
	// citizen, the other moves its deadline. Holding one must never imply the other.
	c := &checkerGia{co: map[Perm]bool{"task.approve": true}}
	h := RequirePermission(c, "task.extend")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	p := Principal{ID: "CB001", Kind: "staff", TenantID: tenant.ID(xaA)}
	if got := chay(h, dungRequest(tenant.ID(xaA), &p)); got != http.StatusForbidden {
		t.Errorf("có task.approve mà vào được task.extend: mã = %d, muốn 403", got)
	}
	if len(c.hoiGi) != 1 || c.hoiGi[0] != "task.extend" {
		t.Errorf("Checker được hỏi %v, muốn đúng một lần với \"task.extend\"", c.hoiGi)
	}
}

func TestCitizenOnly(t *testing.T) {
	h := CitizenOnly()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	canBo := Principal{ID: "CB001", Kind: "staff", TenantID: tenant.ID(xaA)}
	if got := chay(h, dungRequest(tenant.ID(xaA), &canBo)); got != http.StatusUnauthorized {
		t.Errorf("cán bộ vào route công dân: mã = %d, muốn 401", got)
	}

	congDan := Principal{ID: "CD001", Kind: "citizen", TenantID: tenant.ID(xaA)}
	if got := chay(h, dungRequest(tenant.ID(xaA), &congDan)); got != http.StatusOK {
		t.Errorf("công dân vào route công dân: mã = %d, muốn 200", got)
	}

	if got := chay(h, dungRequest(tenant.ID(xaA), nil)); got != http.StatusUnauthorized {
		t.Errorf("không đăng nhập: mã = %d, muốn 401", got)
	}
}

func TestLyDoLaBatBuoc(t *testing.T) {
	// Rule 5, forbidden #4: an exemption with no specific reason is an exemption nobody dares
	// remove six months later. Panicking at wiring time is the only moment this can be caught.
	for ten, goi := range map[string]func(){
		"AnyAuthenticated": func() { AnyAuthenticated("") },
		"Public":           func() { Public("") },
	} {
		t.Run(ten, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("%s(\"\") phải panic khi thiếu lý do", ten)
				}
			}()
			goi()
		})
	}
}

func TestNhom(t *testing.T) {
	cases := map[Perm]string{
		"task.extend":         "task",
		"feedback.restricted": "feedback",
		"admin":               "admin", // không có dấu chấm
	}
	for p, muon := range cases {
		if got := Nhom(p); got != muon {
			t.Errorf("Nhom(%q) = %q, muốn %q", p, got, muon)
		}
	}
}
