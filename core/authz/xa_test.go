package authz

import (
	"net/http"
	"testing"

	"github.com/vihat/vigov/core/tenant"
)

// THE DEFECT CLASS THIS FILE CLOSES: only ONE of the three guards compared the commune.
//
// RequirePermission did; AnyAuthenticated did not. Nothing was exploitable through the identity
// service today, because its XacThuc middleware refuses a token from another commune before any
// guard runs. But this package is SHARED: the next service to mount AnyAuthenticated inherits
// whatever it does, and a service whose edge is assembled slightly differently inherits a route
// where a token from commune A is accepted at commune B's domain.
//
// The failure is invisible: the request succeeds, the response looks right, and no test of a
// single commune can produce it.

func TestAnyAuthenticatedTuChoiTokenLechXa(t *testing.T) {
	h := AnyAuthenticated("mọi tài khoản đều được kết thúc phiên của chính mình")(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))

	p := Principal{ID: "CB001", Kind: "staff", TenantID: tenant.ID(xaB)} // token của xã B
	if got := chay(h, dungRequest(tenant.ID(xaA), &p)); got != http.StatusUnauthorized {
		t.Errorf("token của xã B tới tên miền xã A: mã = %d, muốn 401 — bỏ kiểm quyền không có "+
			"nghĩa là bỏ kiểm xã", got)
	}
}

func TestAnyAuthenticatedChoQuaKhiDungXa(t *testing.T) {
	// The guard must refuse the mismatch and nothing else: an ordinary signed-in account of this
	// commune still gets through, permission or no permission.
	h := AnyAuthenticated("mọi tài khoản đều được kết thúc phiên của chính mình")(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))

	p := Principal{ID: "CB001", Kind: "staff", TenantID: tenant.ID(xaA)}
	if got := chay(h, dungRequest(tenant.ID(xaA), &p)); got != http.StatusOK {
		t.Errorf("đúng xã: mã = %d, muốn 200", got)
	}
	// A citizen of this commune is a signed-in account too.
	cd := Principal{ID: "CD001", Kind: "citizen", TenantID: tenant.ID(xaA)}
	if got := chay(h, dungRequest(tenant.ID(xaA), &cd)); got != http.StatusOK {
		t.Errorf("công dân cùng xã: mã = %d, muốn 200", got)
	}
}

func TestAnyAuthenticatedKhongTokenVanLa401(t *testing.T) {
	h := AnyAuthenticated("mọi tài khoản đều được kết thúc phiên của chính mình")(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))

	if got := chay(h, dungRequest(tenant.ID(xaA), nil)); got != http.StatusUnauthorized {
		t.Errorf("không token: mã = %d, muốn 401", got)
	}
}

func TestHaiGuardTraLoiGiongNhauTruocTokenLechXa(t *testing.T) {
	// One invariant, one answer. If the two guards ever disagree, one of them has grown its own
	// copy of the rule — which is how the copy that gets forgotten appears.
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	c := &checkerGia{co: map[Perm]bool{"task.read": true}}

	p := Principal{ID: "CB001", Kind: "staff", TenantID: tenant.ID(xaB)}
	coQuyen := chay(RequirePermission(c, "task.read")(ok), dungRequest(tenant.ID(xaA), &p))
	moiTaiKhoan := chay(AnyAuthenticated("lý do có thật")(ok), dungRequest(tenant.ID(xaA), &p))

	if coQuyen != moiTaiKhoan {
		t.Errorf("RequirePermission trả %d còn AnyAuthenticated trả %d cho cùng một token lệch xã",
			coQuyen, moiTaiKhoan)
	}
	if len(c.hoiGi) != 0 {
		t.Error("đã hỏi Checker trước khi kiểm xã")
	}
}
