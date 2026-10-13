package http

// POST /api/v1/communes/{id}/deletion — owner only (ADR 0048 §Sửa đổi 13/10/2026 #2, #7).
//
// The operator realm's reading of rule 5 invariant 7 (operator_routes_test.go explains it): 401 · 401
// staff token · 403 a live NON-owner session holding EVERY ops.* key · 503 identity down · 201 the
// owner. "Right permission, wrong commune" has no operator analogue. The store's own checks (active,
// too recent, domain mismatch, already deleted) are exercised against PostgreSQL in
// store/commune_deletion_pg_test.go; here, that each refusal reaches the console with its code.

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/service-platform/internal/domain"
	"github.com/vihat/vigov/service-platform/internal/store"
)

const deletionPathFake = "/api/v1/communes/" + communeIDFake + "/deletion"

const deletionBodyFake = `{"confirm_domain":"thangbinh-danang.vigov.vn","reason":"Xã đã sáp nhập theo nghị quyết"}`

func TestCommuneDeletionIsOwnerOnly(t *testing.T) {
	h := newHarness(t)
	if rec := h.do("POST", deletionPathFake, deletionBodyFake, ""); rec.Code != 401 {
		t.Errorf("no cookie: %d, want 401", rec.Code)
	}
	if rec := h.do("POST", deletionPathFake, deletionBodyFake, staffCookie(t)); rec.Code != 401 {
		t.Errorf("staff token: %d, want 401", rec.Code)
	}
	h.id.down = true
	if rec := h.do("POST", deletionPathFake, deletionBodyFake, opCookie(t)); rec.Code != 503 {
		t.Errorf("identity down: %d, want 503", rec.Code)
	}
	h.id.down = false

	// Every decided key, not the owner: refused. A key is never a substitute for the owner flag.
	h.id.keys = decidedKeysFake
	if rec := h.do("POST", deletionPathFake, deletionBodyFake, opCookie(t)); rec.Code != 403 {
		t.Errorf("every ops key, not owner: %d, want 403", rec.Code)
	}
	if h.w.deletions != 0 {
		t.Fatal("a non-owner reached the store")
	}

	// The owner, holding NO key at all: allowed — the flag alone decides.
	h.id.keys, h.id.owner = nil, true
	rec := h.do("POST", deletionPathFake, deletionBodyFake, opCookie(t))
	if rec.Code != 201 {
		t.Fatalf("owner: %d, want 201 (%s)", rec.Code, rec.Body)
	}
	var v communeDeletionView
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	if v.ID != communeIDFake || v.DeletedBy != opCodeFake || v.DeletedAt.IsZero() {
		t.Errorf("view %+v", v)
	}
	// The target is the PATH's commune; the actor the business code (rule 6 invariant 8); the
	// confirmation passed through untouched (the store decides); the hosts forgotten from the cache.
	if h.w.target != communeIDFake || h.w.actor.Code != opCodeFake || h.w.actor.IP == "" {
		t.Errorf("target %q actor %+v", h.w.target, h.w.actor)
	}
	if h.w.lastConfirm != "thangbinh-danang.vigov.vn" || h.w.lastReason != "Xã đã sáp nhập theo nghị quyết" {
		t.Errorf("confirm %q reason %q", h.w.lastConfirm, h.w.lastReason)
	}
	if len(h.forgot) != 1 || h.forgot[0] != "thangbinh-danang.vigov.vn" {
		t.Errorf("hosts not dropped from the cache: %v", h.forgot)
	}
}

func TestCommuneDeletionRefusals(t *testing.T) {
	earliest := time.Date(2026, 10, 20, 1, 30, 0, 0, time.UTC)
	cases := []struct {
		name, path, body string
		storeErr         error
		want             int
		code             string
		reachesStore     bool
	}{
		{"malformed id", "/api/v1/communes/not-a-ulid/deletion", deletionBodyFake, nil, 404, "commune_not_found", false},
		{"unknown field (a commune in the body)", deletionPathFake,
			`{"confirm_domain":"a.vigov.vn","reason":"x","tenant_id":"01JD8ZQK9M3NPXR7TVWYB2C4EH"}`, nil, 400, "invalid_body", false},
		{"blank reason", deletionPathFake, `{"confirm_domain":"thangbinh-danang.vigov.vn","reason":"   "}`, nil, 422, "invalid_reason", false},
		{"reason over 500", deletionPathFake,
			`{"confirm_domain":"thangbinh-danang.vigov.vn","reason":"` + strings.Repeat("a", 501) + `"}`, nil, 422, "invalid_reason", false},
		{"unknown or already deleted", deletionPathFake, deletionBodyFake, store.ErrCommuneNotFound, 404, "commune_not_found", true},
		{"still active", deletionPathFake, deletionBodyFake, store.ErrCommuneActive, 409, "commune_active", true},
		{"inactive too recently", deletionPathFake, deletionBodyFake,
			&store.InactiveTooRecentError{EarliestAt: earliest}, 409, "commune_inactive_too_recent", true},
		{"domain mismatch", deletionPathFake, `{"confirm_domain":"khac.vigov.vn","reason":"x"}`,
			domain.ErrDeletionConfirmMismatch, 422, "confirm_domain_mismatch", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t)
			h.id.owner = true
			h.w.err = c.storeErr
			rec := h.do("POST", c.path, c.body, opCookie(t))
			if rec.Code != c.want || !strings.Contains(rec.Body.String(), `"code":"`+c.code+`"`) {
				t.Fatalf("status %d body %s; want %d %s", rec.Code, rec.Body, c.want, c.code)
			}
			if (h.w.deletions == 1) != c.reachesStore {
				t.Errorf("reached the store %d times, want reached=%v", h.w.deletions, c.reachesStore)
			}
			if len(h.forgot) != 0 {
				t.Errorf("a refused deletion dropped hosts from the cache: %v", h.forgot)
			}
		})
	}
}

// The too-recent refusal says WHEN, as a field and in the sentence; the mismatch names no domain.
func TestCommuneDeletionRefusalBodies(t *testing.T) {
	earliest := time.Date(2026, 10, 20, 1, 30, 0, 0, time.UTC)
	h := newHarness(t)
	h.id.owner = true
	h.w.err = &store.InactiveTooRecentError{EarliestAt: earliest}
	rec := h.do("POST", deletionPathFake, deletionBodyFake, opCookie(t))
	var v communeDeletionRefusalView
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	if v.EarliestDeletionAt == nil || !v.EarliestDeletionAt.Equal(earliest) {
		t.Errorf("earliest_deletion_at = %v, want %v", v.EarliestDeletionAt, earliest)
	}
	if !strings.Contains(v.Message, "08:30 ngày 20/10/2026") {
		t.Errorf("message does not state the earliest time in Viet Nam time: %q", v.Message)
	}

	h = newHarness(t)
	h.id.owner = true
	h.w.err = store.ErrCommuneActive
	rec = h.do("POST", deletionPathFake, deletionBodyFake, opCookie(t))
	if !strings.Contains(rec.Body.String(), `"earliest_deletion_at":null`) {
		t.Errorf("commune_active must carry earliest_deletion_at null: %s", rec.Body)
	}

	h = newHarness(t)
	h.id.owner = true
	h.w.err = domain.ErrDeletionConfirmMismatch
	rec = h.do("POST", deletionPathFake, `{"confirm_domain":"khac.vigov.vn","reason":"x"}`, opCookie(t))
	if strings.Contains(rec.Body.String(), "vigov.vn") {
		t.Errorf("the mismatch refusal leaks a domain: %s", rec.Body)
	}
}
