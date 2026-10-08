package authz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/httpx"
)

// ADR 0080: a session without a verified phone becomes a principal of its OWN Kind, owned by the Zalo
// account, so no pre-0080 path that tests Kind == "citizen" can store the account id as a citizen id.

const (
	tokenZaloOnly = "token-zalo-only"
	tokenNoOwner  = "token-no-owner"
	tokenBoth     = "token-verified-and-zalo"
)

func zaloChain(class func(http.Handler) http.Handler, inner http.Handler) http.Handler {
	so := soPhienCongDanGia{
		tokenCongDan:  {ID: "phien-1", CitizenID: "cd-1", TenantID: xaCongDan},
		tokenZaloOnly: {ID: "phien-2", TenantID: xaCongDan, ZaloAccountID: "tkz-1"},
		tokenNoOwner:  {ID: "phien-3", TenantID: xaCongDan},
		tokenBoth:     {ID: "phien-4", CitizenID: "cd-4", TenantID: xaCongDan, ZaloAccountID: "tkz-4"},
	}
	return httpx.CitizenEdge(so)(CitizenPrincipal()(CitizenOnly()(class(inner))))
}

func capture(p *Principal, owner *Principal, ownerOK *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*p, _ = From(r.Context())
		*owner, *ownerOK = CitizenChannelOwner(r.Context())
		w.WriteHeader(http.StatusNoContent)
	})
}

func TestKindConstantsMatchAudit(t *testing.T) {
	if KindZaloAccount != audit.KindZaloAccount {
		t.Fatalf("authz.KindZaloAccount %q != audit.KindZaloAccount %q — the principal kind is the actor_kind",
			KindZaloAccount, audit.KindZaloAccount)
	}
	if KindZaloAccount == KindCitizen {
		t.Fatal("the Zalo account kind must differ from the citizen kind")
	}
}

func TestZaloAccountOnlySessionIsItsOwnKind(t *testing.T) {
	var p, owner Principal
	var ok bool
	h := zaloChain(httpx.CommuneFromSessionOrZaloAccount("test: unverified petition"), capture(&p, &owner, &ok))
	if w := goiCongDan(h, tokenZaloOnly); w.Code != http.StatusNoContent {
		t.Fatalf("code %d, want 204", w.Code)
	}
	if p.Kind != KindZaloAccount || p.ID != "tkz-1" || p.TenantID != xaCongDan || len(p.Roles) != 0 {
		t.Fatalf("principal = %+v, want zalo-account tkz-1 of %q with no roles", p, xaCongDan)
	}
	if !ok || owner.Kind != KindZaloAccount || owner.ID != "tkz-1" {
		t.Fatalf("CitizenChannelOwner = %+v, %v", owner, ok)
	}
}

func TestVerifiedCitizenWinsOverZaloAccount(t *testing.T) {
	for tok, id := range map[string]string{tokenCongDan: "cd-1", tokenBoth: "cd-4"} {
		var p, owner Principal
		var ok bool
		h := zaloChain(httpx.CommuneFromSessionOrZaloAccount("test: unverified petition"), capture(&p, &owner, &ok))
		if w := goiCongDan(h, tok); w.Code != http.StatusNoContent {
			t.Fatalf("%s: code %d", tok, w.Code)
		}
		if p.Kind != KindCitizen || p.ID != id || !ok || owner.ID != id {
			t.Errorf("%s: principal %+v owner %+v/%v, want citizen %s", tok, p, owner, ok, id)
		}
	}
}

func TestXaTuPhienRouteStillRefusesZaloAccountOnly(t *testing.T) {
	ran := false
	h := zaloChain(httpx.XaTuPhien(), http.HandlerFunc(func(http.ResponseWriter, *http.Request) { ran = true }))
	if w := goiCongDan(h, tokenZaloOnly); w.Code != http.StatusForbidden || ran {
		t.Fatalf("code %d ran %v — an existing citizen route must still answer 403 chua_xac_thuc_so", w.Code, ran)
	}
}

func TestViewOnlyRouteStillServesZaloAccountOnly(t *testing.T) {
	// CitizenOnly admits the new kind precisely so this route does not turn into a 401 loop.
	var p, owner Principal
	var ok bool
	h := zaloChain(httpx.XaTuPhienChiXem("test: field catalogue"), capture(&p, &owner, &ok))
	if w := goiCongDan(h, tokenZaloOnly); w.Code != http.StatusNoContent {
		t.Fatalf("code %d, want 204", w.Code)
	}
}

func TestSessionWithNoOwnerHasNoChannelOwner(t *testing.T) {
	var p, owner Principal
	var ok bool
	h := zaloChain(httpx.XaTuPhienChiXem("test: field catalogue"), capture(&p, &owner, &ok))
	if w := goiCongDan(h, tokenNoOwner); w.Code != http.StatusNoContent {
		t.Fatalf("code %d", w.Code)
	}
	if p.Kind != KindCitizen || p.ID != "" {
		t.Errorf("principal = %+v, want the pre-0080 shape: citizen with an empty id", p)
	}
	if ok {
		t.Errorf("CitizenChannelOwner reported an owner %+v for a session with none", owner)
	}
}

func TestCitizenChannelOwnerRefusesStaffAndEmpty(t *testing.T) {
	for name, ctx := range map[string]context.Context{
		"no principal":       context.Background(),
		"staff":              Into(context.Background(), Principal{ID: "nd-1", Ma: "CB-00001", Kind: "staff", TenantID: xaCongDan}),
		"empty citizen":      Into(context.Background(), Principal{Kind: KindCitizen, TenantID: xaCongDan}),
		"empty zalo account": Into(context.Background(), Principal{Kind: KindZaloAccount, TenantID: xaCongDan}),
		"unknown kind":       Into(context.Background(), Principal{ID: "x-1", Kind: "system", TenantID: xaCongDan}),
	} {
		if p, ok := CitizenChannelOwner(ctx); ok {
			t.Errorf("%s: owner %+v, want none", name, p)
		}
	}
}

func TestCitizenOnlyStillRefusesStaff(t *testing.T) {
	ran := false
	h := CitizenOnly()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { ran = true }))
	r, _ := http.NewRequestWithContext(Into(context.Background(), Principal{ID: "nd-1", Kind: "staff"}), "GET", "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized || ran {
		t.Fatalf("code %d ran %v — a staff principal on a citizen route must be 401", w.Code, ran)
	}
}
