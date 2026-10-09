package http

// GET /api/v1/my-residential-units through the citizen chain the binary mounts (CitizenEdge ·
// CitizenPrincipal · the route's own two declarations). What is pinned:
//
//	401 without a session, and for a STAFF session (cookie, staff bearer token, or a staff principal)
//	the commune is the SESSION's — commune A's session reads only A's units, a client header is ignored
//	a session without a verified phone still sees the picker (XaTuPhienChiXem)
//	the body is {items:[{id,name}]} and NOTHING else — no head of unit, no counts
//	the predicate is the store's (pg test); here, an empty-name row is not offered
//	ceiling / store failure → 500, never a partial list

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

const (
	ruCommuneA       tenant.ID = "01JE5AAAAAAAAAAAAAAAAAAAAA"
	ruCommuneB       tenant.ID = "01JE5BBBBBBBBBBBBBBBBBBBBB"
	ruTokenA                   = "citizen-token-A-FAKE"
	ruTokenB                   = "citizen-token-B-FAKE"
	ruTokenNoPhone             = "citizen-token-no-phone-FAKE"
	ruTokenNoCommune           = "citizen-token-no-commune-FAKE"
	ruStaffToken               = "staff-token-FAKE"
)

// ruSessions is the citizen registry: A, B, A without a verified phone, and a session with no commune.
// Any other token — a staff token included — is not a citizen session.
type ruSessions struct{}

// vi-name-ok: implements the existing httpx.CitizenSessions interface method.
func (ruSessions) TraCuu(_ context.Context, token string) (httpx.CitizenSession, bool, error) {
	switch token {
	case ruTokenA:
		return httpx.CitizenSession{ID: "sid-a", CitizenID: "citizen-1", TenantID: ruCommuneA}, true, nil
	case ruTokenB:
		return httpx.CitizenSession{ID: "sid-b", CitizenID: "citizen-1", TenantID: ruCommuneB}, true, nil
	case ruTokenNoPhone:
		return httpx.CitizenSession{ID: "sid-np", ZaloAccountID: "zalo-acc-1", TenantID: ruCommuneA}, true, nil
	case ruTokenNoCommune:
		return httpx.CitizenSession{ID: "sid-nc", CitizenID: "citizen-1"}, true, nil
	}
	return httpx.CitizenSession{}, false, nil
}

// ruUnits answers each commune's ACTIVE units (what the store's predicate leaves), keyed by the commune
// in the context — so a read in the wrong commune shows up as the wrong names.
type ruUnits struct {
	err      error
	calls    int
	communes []tenant.ID
}

func (f *ruUnits) ActiveUnits(ctx context.Context) ([]domain.ActiveResidentialUnit, error) {
	f.calls++
	id := tenant.MustFrom(ctx)
	f.communes = append(f.communes, id)
	if f.err != nil {
		return nil, f.err
	}
	switch id {
	case ruCommuneA:
		return []domain.ActiveResidentialUnit{
			{ID: "tt-a1", Name: "Thôn Xuân A"},
			{ID: "tt-a2", Name: "Tổ dân phố 2 A"},
			{ID: "tt-broken", Name: ""},
		}, nil
	case ruCommuneB:
		return []domain.ActiveResidentialUnit{{ID: "tt-b1", Name: "Thôn Đông B"}}, nil
	}
	return nil, nil
}

// ruChain mirrors cmd/server buildCitizenEdge minus CORS (tested there): Strip · Recover · CitizenEdge ·
// CitizenPrincipal · mux. `outer` runs outermost, to plant a staff principal where one could leak in.
func ruChain(t *testing.T, units *ruUnits, outer func(http.Handler) http.Handler) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	RegisterCitizen(mux, DepsCitizen{ResidentialUnits: units, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	var h http.Handler = mux
	h = authz.CitizenPrincipal()(h)
	h = httpx.CitizenEdge(ruSessions{})(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)
	if outer != nil {
		h = outer(h)
	}
	return h
}

func ruCall(t *testing.T, h http.Handler, bearer string, mutate func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "https://identity.api.vigov.vn"+MyResidentialUnitsPath, nil)
	r.Host = "identity.api.vigov.vn"
	if bearer != "" {
		r.Header.Set("Authorization", "Bearer "+bearer)
	}
	// A client naming its own commune must change nothing (rule 1, forbidden #2).
	r.Header.Set("X-Tenant-ID", string(ruCommuneB))
	if mutate != nil {
		mutate(r)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestMyResidentialUnits_401WithoutSession(t *testing.T) {
	u := &ruUnits{}
	w := ruCall(t, ruChain(t, u, nil), "", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401 — body %s", w.Code, w.Body.String())
	}
	if u.calls != 0 {
		t.Error("the store was read without a session")
	}
}

func TestMyResidentialUnits_401ForUnusableTokenAndSessionWithoutCommune(t *testing.T) {
	for _, tok := range []string{"not-a-session-FAKE", ruTokenNoCommune} {
		u := &ruUnits{}
		if w := ruCall(t, ruChain(t, u, nil), tok, nil); w.Code != http.StatusUnauthorized {
			t.Errorf("%s: code = %d, want 401", tok, w.Code)
		}
		if u.calls != 0 {
			t.Errorf("%s: the store was read", tok)
		}
	}
}

// A STAFF SESSION IS NOT A CITIZEN SESSION, by any of the three ways it could arrive here.
func TestMyResidentialUnits_StaffSessionRefused(t *testing.T) {
	cases := map[string]struct {
		bearer string
		mutate func(*http.Request)
		outer  func(http.Handler) http.Handler
	}{
		// The staff cookie: the citizen edge reads no cookie at all.
		"staff cookie": {mutate: func(r *http.Request) {
			r.AddCookie(&http.Cookie{Name: "vigov_session", Value: ruStaffToken})
		}},
		// A staff token presented as a bearer: the citizen registry does not know it.
		"staff bearer": {bearer: ruStaffToken},
		// A staff principal already in the context (a mis-mounted staff layer): CitizenOnly refuses its Kind.
		"staff principal": {outer: func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				p := authz.Principal{ID: "nd-1", Ma: "CB-00001", Kind: "staff", TenantID: ruCommuneA}
				next.ServeHTTP(w, r.WithContext(authz.Into(r.Context(), p)))
			})
		}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			u := &ruUnits{}
			w := ruCall(t, ruChain(t, u, c.outer), c.bearer, c.mutate)
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("code = %d, want 401 — body %s", w.Code, w.Body.String())
			}
			if u.calls != 0 {
				t.Error("the store was read for a staff session")
			}
		})
	}
}

func TestMyResidentialUnits_CommuneAReadsOnlyItsOwnActiveUnits(t *testing.T) {
	u := &ruUnits{}
	w := ruCall(t, ruChain(t, u, nil), ruTokenA, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d — body %s", w.Code, w.Body.String())
	}
	// EXACT BYTES: commune A's two named units in the store's order; the empty-name row not offered;
	// commune B's unit absent although the request named B in a header.
	want := `{"items":[{"id":"tt-a1","name":"Thôn Xuân A"},{"id":"tt-a2","name":"Tổ dân phố 2 A"}]}` + "\n"
	if w.Body.String() != want {
		t.Fatalf("body =\n%s\nwant\n%s", w.Body.String(), want)
	}
	if len(u.communes) != 1 || u.communes[0] != ruCommuneA {
		t.Fatalf("read communes %v, want only the session's %q", u.communes, ruCommuneA)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "private, max-age=60" {
		t.Errorf("Cache-Control = %q", cc)
	}
	if !strings.Contains(strings.Join(w.Header().Values("Vary"), ","), "Authorization") {
		t.Errorf("Vary = %v, want Authorization — the answer depends on the session", w.Header().Values("Vary"))
	}
}

func TestMyResidentialUnits_CommuneBSessionReadsB(t *testing.T) {
	u := &ruUnits{}
	w := ruCall(t, ruChain(t, u, nil), ruTokenB, nil)
	if w.Code != http.StatusOK || strings.Contains(w.Body.String(), " A\"") || !strings.Contains(w.Body.String(), "Thôn Đông B") {
		t.Fatalf("code %d body %s — commune B's session read the wrong commune", w.Code, w.Body.String())
	}
}

// Nothing but id and name leaves — decoded generically so a field added to the output type turns this red.
func TestMyResidentialUnits_NoFieldBeyondIDAndName(t *testing.T) {
	w := ruCall(t, ruChain(t, &ruUnits{}, nil), ruTokenA, nil)
	var body map[string][]map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body) != 1 || body["items"] == nil {
		t.Fatalf("top-level keys = %v, want only items", body)
	}
	for _, it := range body["items"] {
		for k := range it {
			if k != "id" && k != "name" {
				t.Errorf("field %q leaves on the citizen surface — only id and name may (rule 3, rule 4 forbidden #5)", k)
			}
		}
	}
}

func TestMyResidentialUnits_SessionWithoutVerifiedPhoneSeesThePicker(t *testing.T) {
	if w := ruCall(t, ruChain(t, &ruUnits{}, nil), ruTokenNoPhone, nil); w.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 — the unverified petition form needs the picker (ADR 0080)", w.Code)
	}
}

func TestMyResidentialUnits_StoreFailureIs500NeverAPartialList(t *testing.T) {
	for name, err := range map[string]error{
		"ceiling": idstore.ErrQuaNhieuThonToDanPho,
		"store":   errors.New("db: connection reset"),
	} {
		t.Run(name, func(t *testing.T) {
			w := ruCall(t, ruChain(t, &ruUnits{err: err}, nil), ruTokenA, nil)
			if w.Code != http.StatusInternalServerError {
				t.Fatalf("code = %d, want 500", w.Code)
			}
			if strings.Contains(w.Body.String(), "connection reset") || strings.Contains(w.Body.String(), "items") {
				t.Errorf("body leaks the cause or a list: %s", w.Body.String())
			}
		})
	}
}

func TestRegisterCitizen_RefusesMissingStore(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("RegisterCitizen accepted a nil residential-unit store")
		}
	}()
	RegisterCitizen(http.NewServeMux(), DepsCitizen{})
}
