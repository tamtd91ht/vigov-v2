package store

import (
	"context"
	"errors"
	"testing"
)

// Runs WITHOUT a database, on purpose: the Directory here holds a nil handle, so a reserved host
// that reached the query would panic. Passing proves the refusal happens BEFORE the table is read
// — which is the property that keeps the two kept admin rows unreachable (migration 0007).
func TestTenMienDanhRiengKhongChamCSDL(t *testing.T) {
	t.Parallel()

	d := NewDirectory(nil, "")
	for _, h := range []string{
		"admin.vigov.vn", "ADMIN.vigov.vn.", "admin-stg.vigov.vn:443", "vigov.vn",
		"identity.api.vigov.vn", "petitions.api-stg.vigov.vn", "api.stg.vigov.vn",
	} {
		if _, ok := d.ByHost(context.Background(), h); ok {
			t.Errorf("ByHost(%q) = true, muốn false", h)
		}
		// The SAME sentinel as an unknown host — never a distinct one.
		if _, err := d.ByHostErr(context.Background(), h); !errors.Is(err, ErrKhongCoXa) {
			t.Errorf("ByHostErr(%q) = %v, muốn ErrKhongCoXa", h, err)
		}
	}
}

// OPERATOR_HOST is reserved like a platform address — even outside vigov.vn, where only the
// variable can say so — and refused before the (nil) handle is touched.
func TestOperatorHostNeverResolves(t *testing.T) {
	t.Parallel()

	d := NewDirectory(nil, "console.example.org")
	for _, h := range []string{"console.example.org", "CONSOLE.example.org:443", "console.example.org."} {
		if _, ok := d.ByHost(context.Background(), h); ok {
			t.Errorf("ByHost(%q) = true — the operator host resolved to a commune", h)
		}
		if _, err := d.ByHostErr(context.Background(), h); !errors.Is(err, ErrKhongCoXa) {
			t.Errorf("ByHostErr(%q) = %v, want ErrKhongCoXa (same as an unknown host)", h, err)
		}
	}
}
