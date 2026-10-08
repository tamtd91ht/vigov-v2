package httpx

import (
	"net/http"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/tenant"
)

// ADR 0080: CommuneFromSessionOrZaloAccount is the ONE class that serves a session without a verified
// phone as an OWNER — through the Zalo account that opened it. Every other class keeps its answer.

const (
	tokenZaloOnly   = "token-zalo-account-only"
	tokenNoOwner    = "token-commune-no-owner"
	tokenBothOwners = "token-verified-and-zalo"
	reasonZalo      = "test: submit an unverified petition owned by the Zalo account"
)

func edgeWithZaloSessions(inner http.Handler) http.Handler {
	so := soPhienMau()
	so[tokenZaloOnly] = CitizenSession{ID: "phien-z", TenantID: xaA, ZaloAccountID: "tkz-1"}
	so[tokenNoOwner] = CitizenSession{ID: "phien-n", TenantID: xaA}
	so[tokenBothOwners] = CitizenSession{ID: "phien-b", CitizenID: "cd-9", TenantID: xaA, ZaloAccountID: "tkz-9"}
	return CitizenEdge(so)(inner)
}

func serving(seen *tenant.ID, ran *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*ran = true
		*seen = tenant.MustFrom(r.Context())
		w.WriteHeader(http.StatusNoContent)
	})
}

func TestXaTuPhienStillRefusesZaloAccountOnlySession(t *testing.T) {
	// The regression that matters most: identity filling ZaloAccountID must not open a single
	// existing route to a session without a verified phone.
	var seen tenant.ID
	var ran bool
	w := goiCongDan(edgeWithZaloSessions(XaTuPhien()(serving(&seen, &ran))), tokenZaloOnly)
	if ran || w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "chua_xac_thuc_so") {
		t.Fatalf("XaTuPhien with a Zalo-account-only session: code %d ran %v body %s — want 403 chua_xac_thuc_so",
			w.Code, ran, w.Body.String())
	}
}

func TestZaloAccountClassServesVerifiedCitizen(t *testing.T) {
	for _, tok := range []string{tokenA, tokenBothOwners} {
		var seen tenant.ID
		var ran bool
		w := goiCongDan(edgeWithZaloSessions(CommuneFromSessionOrZaloAccount(reasonZalo)(serving(&seen, &ran))), tok)
		if w.Code != http.StatusNoContent || !ran || seen != xaA {
			t.Errorf("%s: code %d ran %v commune %q — a verified session must be served as by XaTuPhien", tok, w.Code, ran, seen)
		}
	}
}

func TestZaloAccountClassServesZaloAccountOnlySession(t *testing.T) {
	var seen tenant.ID
	var ran bool
	w := goiCongDan(edgeWithZaloSessions(CommuneFromSessionOrZaloAccount(reasonZalo)(serving(&seen, &ran))), tokenZaloOnly)
	if w.Code != http.StatusNoContent || !ran {
		t.Fatalf("code %d ran %v — want served", w.Code, ran)
	}
	if seen != xaA {
		t.Fatalf("commune in context = %q, want the session's %q", seen, xaA)
	}
}

func TestZaloAccountClassRefusesSessionWithNoOwner(t *testing.T) {
	// Fail closed: neither a citizen id nor a Zalo account — the same 403 XaTuPhien gives, nothing in
	// the owner's place.
	var seen tenant.ID
	var ran bool
	h := edgeWithZaloSessions(CommuneFromSessionOrZaloAccount(reasonZalo)(serving(&seen, &ran)))
	w := goiCongDan(h, tokenNoOwner)
	want := goiCongDan(edgeWithZaloSessions(XaTuPhien()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))), tokenNoOwner)
	if ran || w.Code != http.StatusForbidden || w.Body.String() != want.Body.String() {
		t.Fatalf("code %d ran %v body %s — want exactly XaTuPhien's 403 %s", w.Code, ran, w.Body.String(), want.Body.String())
	}
}

func TestZaloAccountClassNeverWaivesTheCommune(t *testing.T) {
	h := edgeWithZaloSessions(CommuneFromSessionOrZaloAccount(reasonZalo)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("handler ran without a commune")
	})))
	want := goiCongDan(edgeWithZaloSessions(XaTuPhien()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))), "")
	so := soPhienMau()
	so["zalo-no-commune"] = CitizenSession{ID: "phien-x", ZaloAccountID: "tkz-2"}
	noCommune := CitizenEdge(so)(CommuneFromSessionOrZaloAccount(reasonZalo)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("handler ran for a Zalo account session with no commune")
	})))
	for name, w := range map[string]int{
		"no session":               goiCongDan(h, "").Code,
		"unknown token":            goiCongDan(h, "khong-co-that").Code,
		"commune not chosen":       goiCongDan(h, tokenChua).Code,
		"zalo account, no commune": goiCongDan(noCommune, "zalo-no-commune").Code,
	} {
		if w != http.StatusUnauthorized || want.Code != http.StatusUnauthorized {
			t.Errorf("%s: code %d, want 401 as XaTuPhien", name, w)
		}
	}
}

func TestZaloAccountClassNeedsAReason(t *testing.T) {
	for _, reason := range []string{"", "  "} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatalf("CommuneFromSessionOrZaloAccount(%q) did not panic — a waiver with no reason", reason)
				}
			}()
			CommuneFromSessionOrZaloAccount(reason)
		}()
	}
}

func TestZaloAccountClassCountsAsDeclared(t *testing.T) {
	// The undeclared-class wall must see this class as declared, or every such route answers 500.
	h := edgeWithZaloSessions(CommuneFromSessionOrZaloAccount(reasonZalo)(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })))
	if w := goiCongDan(h, tokenZaloOnly); w.Code != http.StatusOK {
		t.Fatalf("code %d, want 200", w.Code)
	}
}

func TestViewOnlyClassUnchangedForZaloAccountOnlySession(t *testing.T) {
	var seen tenant.ID
	var ran bool
	w := goiCongDan(edgeWithZaloSessions(XaTuPhienChiXem("test: view-only")(serving(&seen, &ran))), tokenZaloOnly)
	if w.Code != http.StatusNoContent || !ran {
		t.Fatalf("XaTuPhienChiXem with a Zalo-account session: code %d — want served as before", w.Code)
	}
}
