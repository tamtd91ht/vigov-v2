package authz

import (
	"net/http"
	"testing"

	"github.com/vihat/vigov/core/tenant"
)

// RequireAnyPermission must answer exactly as RequirePermission does on every axis except one:
// it lets through a holder of ANY listed key. These are rule 5 invariant 7's four cases, plus the
// second "200" that proves the second key is not decoration.

func anyGuarded(c Checker) http.Handler {
	return RequireAnyPermission(c, "admin.lookup", "budget.update")(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
}

func TestRequireAnyPermissionNoTokenIs401(t *testing.T) {
	c := &checkerGia{co: map[Perm]bool{"admin.lookup": true}}
	if got := chay(anyGuarded(c), dungRequest(tenant.ID(xaA), nil)); got != http.StatusUnauthorized {
		t.Errorf("no token: code = %d, want 401", got)
	}
}

func TestRequireAnyPermissionHoldsNeitherIs403(t *testing.T) {
	c := &checkerGia{co: map[Perm]bool{"task.read": true}}
	p := Principal{ID: "CB001", Kind: "staff", TenantID: tenant.ID(xaA)}
	if got := chay(anyGuarded(c), dungRequest(tenant.ID(xaA), &p)); got != http.StatusForbidden {
		t.Errorf("holds neither key: code = %d, want 403", got)
	}
	if len(c.hoiGi) != 2 {
		t.Errorf("Checker asked %v, want both listed keys asked before refusing", c.hoiGi)
	}
}

func TestRequireAnyPermissionRightKeyWrongCommuneMatchesRequirePermission(t *testing.T) {
	// Same answer as RequirePermission for a token of commune B at commune A's host (401, see
	// TestDungQuyenNhungSaiXaTraVe401), and the Checker is never consulted before the commune.
	c := &checkerGia{co: map[Perm]bool{"admin.lookup": true, "budget.update": true}}
	p := Principal{ID: "CB001", Kind: "staff", TenantID: tenant.ID(xaB)}

	got := chay(anyGuarded(c), dungRequest(tenant.ID(xaA), &p))
	if got != http.StatusUnauthorized {
		t.Errorf("token of another commune: code = %d, want 401", got)
	}
	single := chay(RequirePermission(c, "admin.lookup")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})), dungRequest(tenant.ID(xaA), &p))
	if got != single {
		t.Errorf("RequireAnyPermission answered %d, RequirePermission %d for the same mismatched token", got, single)
	}
	if len(c.hoiGi) != 0 {
		t.Errorf("Checker asked %v before the commune check", c.hoiGi)
	}
}

func TestRequireAnyPermissionFirstKeyIs200(t *testing.T) {
	c := &checkerGia{co: map[Perm]bool{"admin.lookup": true}}
	p := Principal{ID: "CB001", Kind: "staff", TenantID: tenant.ID(xaA)}
	if got := chay(anyGuarded(c), dungRequest(tenant.ID(xaA), &p)); got != http.StatusOK {
		t.Errorf("holds the first key: code = %d, want 200", got)
	}
}

func TestRequireAnyPermissionSecondKeyIs200(t *testing.T) {
	c := &checkerGia{co: map[Perm]bool{"budget.update": true}}
	p := Principal{ID: "CB001", Kind: "staff", TenantID: tenant.ID(xaA)}
	if got := chay(anyGuarded(c), dungRequest(tenant.ID(xaA), &p)); got != http.StatusOK {
		t.Errorf("holds the second key: code = %d, want 200", got)
	}
}

func TestRequireAnyPermissionRefusesFewerThanTwoKeysAtWiring(t *testing.T) {
	// No default allow: an empty list must never become "any signed-in account", and a single key
	// is RequirePermission's spelling — two spellings of one declaration is two things to audit.
	for name, keys := range map[string][]Perm{
		"none":      nil,
		"one":       {"admin.lookup"},
		"empty key": {"admin.lookup", ""},
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("RequireAnyPermission(%q) must panic at construction", keys)
				}
			}()
			RequireAnyPermission(&checkerGia{}, keys...)
		})
	}
}
