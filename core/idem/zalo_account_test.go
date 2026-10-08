package idem

import (
	"context"
	"net/http"
	"testing"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/tenant"
)

// ADR 0080 #9: the idem subject of an unverified petition is the Zalo account. Its key space must be
// distinct from the citizens' — the two ids come from two tables and nothing guarantees they never
// coincide — and an account principal with no id is still the anonymous case, refused.

func principalOf(kind, id string) *authz.Principal {
	return &authz.Principal{ID: id, Kind: kind, TenantID: tenant.ID(xaA)}
}

func TestChuTheKeysZaloAccountApartFromCitizen(t *testing.T) {
	const same = "01J8ZQG7QY5V3N4C6K2D1SAME1"
	c := ChuThe(authz.Into(context.Background(), *principalOf(authz.KindCitizen, same)))
	z := ChuThe(authz.Into(context.Background(), *principalOf(authz.KindZaloAccount, same)))
	if c == z {
		t.Fatalf("citizen and Zalo account with the same id share subject %q", c)
	}
	if z != authz.KindZaloAccount+":"+same {
		t.Fatalf("zalo-account subject = %q, want kind-prefixed", z)
	}
	if Key(tenant.ID(xaA), c, "POST", duongDan, khoaKhachHang) == Key(tenant.ID(xaA), z, "POST", duongDan, khoaKhachHang) {
		t.Fatal("same Redis key for a citizen and a Zalo account")
	}
}

func TestZaloAccountWithoutIDIsAnonymous(t *testing.T) {
	if got := ChuThe(authz.Into(context.Background(), *principalOf(authz.KindZaloAccount, ""))); got != ChuTheAnDanh {
		t.Fatalf("subject = %q, want %q", got, ChuTheAnDanh)
	}
	h := Required(DongKhiHong)(handlerTao("PA-7F3K9Q"))
	w, _ := goiLog(t, h, newStoreGia(), xaA, khoaKhachHang, principalOf(authz.KindZaloAccount, ""))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("code %d, want 500 — an ownerless request must not get a shared key space", w.Code)
	}
}

func TestCitizenAndZaloAccountNeverReplayEachOther(t *testing.T) {
	const same = "01J8ZQG7QY5V3N4C6K2D1SAME1"
	s := newStoreGia()
	h := Required(DongKhiHong)(handlerTao("PA-7F3K9Q"))

	if w, _ := goiLog(t, h, s, xaA, khoaKhachHang, principalOf(authz.KindCitizen, same)); w.Code != http.StatusCreated {
		t.Fatalf("citizen: code %d, want 201", w.Code)
	}
	w, _ := goiLog(t, h, s, xaA, khoaKhachHang, principalOf(authz.KindZaloAccount, same))
	if w.Header().Get(HeaderPhatLai) != "" || w.Code != http.StatusCreated {
		t.Fatalf("Zalo account: code %d replay %q — it was served the citizen's result",
			w.Code, w.Header().Get(HeaderPhatLai))
	}
	w2, _ := goiLog(t, h, s, xaA, khoaKhachHang, principalOf(authz.KindZaloAccount, same))
	if w2.Header().Get(HeaderPhatLai) != "true" {
		t.Error("the same Zalo account resending the same key must be replayed its OWN result")
	}
}
