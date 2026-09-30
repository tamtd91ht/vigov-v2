package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// POST /api/v1/staff/publications — rule 5 invariant 7's four cases, plus what the handler alone
// decides: that the body reaches the use case row by row, that each outcome is answered with the
// single route's code, and that no personal data is in the answer.
//
// The batch rules themselves (per-row consent, locked refused, one transaction, one audit per
// person, rollback on an audit failure, 400 on a malformed batch) are proven against a transaction
// in app/bulk_publication_test.go; here the refusals are only MAPPED.

const bulkBodyOK = `{"items":[{"id":"nd-1","consent_confirmed":true,"display_order":2},{"id":"nd-2"}]}`

func bulkRoute(body string) tuyenGhi {
	return tuyenGhi{"công khai hàng loạt", "POST", "/api/v1/staff/publications", body, http.StatusOK}
}

func TestBulkPublication_401NoToken(t *testing.T) {
	m := dungMayChu(t)
	coContentUpdate(t, m)

	doiMa(t, m.goiGhi(t, bulkRoute(bulkBodyOK), hostA, ""), http.StatusUnauthorized)
	if n := m.ghiDanhBa.soLanGoi(); n != 0 {
		t.Errorf("not signed in, yet the use case ran %d times", n)
	}
}

// 403 with `admin.user` only — the key of every OTHER staff write; managing accounts is not
// deciding what the public sees (user decision 2026-09-24).
func TestBulkPublication_403WithoutContentUpdate(t *testing.T) {
	m := dungMayChu(t) // shipped default: admin.user in commune A, nothing else

	doiMa(t, m.goiGhi(t, bulkRoute(bulkBodyOK), hostA, m.tokenCho(t, xaA, sidA)), http.StatusForbidden)
	if n := m.ghiDanhBa.soLanGoi(); n != 0 {
		t.Errorf("admin.user only, yet the use case ran %d times", n)
	}
}

// 403 with the right key in the WRONG commune — `content.update` in A, signed in at B.
func TestBulkPublication_403RightPermissionWrongCommune(t *testing.T) {
	m := dungMayChu(t)
	coContentUpdate(t, m)

	doiMa(t, m.goiGhi(t, bulkRoute(bulkBodyOK), hostB, m.tokenCho(t, xaB, sidB)), http.StatusForbidden)
	if n := m.ghiDanhBa.soLanGoi(); n != 0 {
		t.Errorf("wrong commune, yet the use case ran %d times", n)
	}
}

// 200 — the body reaches the use case row by row, in the right commune, with the STAFF CODE as actor.
func TestBulkPublication_200PassesRowsThrough(t *testing.T) {
	m := dungMayChu(t)
	coContentUpdate(t, m)
	m.ghiDanhBa.bulkOutcomes = []app.BulkPublishOutcome{
		{ID: "nd-1", Published: true},
		{ID: "nd-2", Refusal: app.ErrChuaXacNhanDongY},
	}

	doiMa(t, m.goiGhi(t, bulkRoute(bulkBodyOK), hostA, m.tokenCho(t, xaA, sidA)), http.StatusOK)

	g := m.ghiDanhBa
	if g.xaCuoi != xaA {
		t.Errorf("commune at the use case = %q", g.xaCuoi)
	}
	if g.nguoiCuoi.Vet.ID != maCanBo || g.nguoiCuoi.ID != idNoiBo {
		t.Errorf("actor = %+v, want staff code %q deciding as %q", g.nguoiCuoi, maCanBo, idNoiBo)
	}
	if len(g.bulkItems) != 2 {
		t.Fatalf("%d items reached the use case, want 2", len(g.bulkItems))
	}
	a, b := g.bulkItems[0], g.bulkItems[1]
	if a.ID != "nd-1" || !a.ConsentConfirmed || a.DisplayOrder == nil || *a.DisplayOrder != 2 {
		t.Errorf("item 0 = %+v", a)
	}
	// consent_confirmed ABSENT reaches the use case as false; display_order absent as nil.
	if b.ID != "nd-2" || b.ConsentConfirmed || b.DisplayOrder != nil {
		t.Errorf("item 1 = %+v", b)
	}
}

// EACH OUTCOME IS ANSWERED WITH THE SINGLE ROUTE'S CODE, in request order, and the answer carries
// no personal data.
func TestBulkPublicationAnswersPerItem(t *testing.T) {
	m := dungMayChu(t)
	coContentUpdate(t, m)
	m.ghiDanhBa.bulkOutcomes = []app.BulkPublishOutcome{
		{ID: "nd-ok", Published: true},
		{ID: "nd-noconsent", Refusal: app.ErrChuaXacNhanDongY},
		{ID: "nd-locked", Refusal: app.ErrStaffLocked},
		{ID: "nd-unknown", Refusal: idstore.ErrCanBoKhongTonTai},
	}

	w := m.goiGhi(t, bulkRoute(bulkBodyOK), hostA, m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	var out struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("body is not JSON: %s", w.Body.String())
	}
	want := []struct{ id, result, reason string }{
		{"nd-ok", "published", ""},
		{"nd-noconsent", "skipped", "consent_required"},
		{"nd-locked", "skipped", "staff_locked"},
		{"nd-unknown", "skipped", "staff_not_found"},
	}
	if len(out.Items) != len(want) {
		t.Fatalf("%d items, want %d: %s", len(out.Items), len(want), w.Body.String())
	}
	for i, x := range want {
		got := out.Items[i]
		if got["id"] != x.id || got["result"] != x.result {
			t.Errorf("item %d = %v, want %s/%s", i, got, x.id, x.result)
		}
		rc, has := got["reason_code"]
		if x.reason == "" && has {
			t.Errorf("item %d: published row carries reason_code %v", i, rc)
		}
		if x.reason != "" && rc != x.reason {
			t.Errorf("item %d: reason_code = %v, want %s", i, rc, x.reason)
		}
		for _, k := range []string{"mobile", "full_name", "office_phone", "email"} {
			if _, leak := got[k]; leak {
				t.Errorf("item %d carries %q — the answer must hold no personal data", i, k)
			}
		}
	}
}

// A MALFORMED BATCH IS 400 invalid_request: the use case's shape refusals, and a body that is not JSON.
func TestBulkPublicationMalformedIs400(t *testing.T) {
	for _, c := range []struct {
		name string
		err  error
	}{
		{"empty", domain.ErrBulkPublicationEmpty},
		{"over the cap", domain.ErrBulkPublicationTooLarge},
		{"duplicate id", domain.ErrBulkPublicationDuplicateID},
		{"empty id", domain.ErrBulkPublicationMissingID},
		{"negative order", domain.ErrThuTuDanhBaAm},
	} {
		m := dungMayChu(t)
		coContentUpdate(t, m)
		m.ghiDanhBa.loi = c.err

		w := m.goiGhi(t, bulkRoute(bulkBodyOK), hostA, m.tokenCho(t, xaA, sidA))
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400 — %s", c.name, w.Code, w.Body.String())
			continue
		}
		if e := loiTra(t, w); e.Code != "invalid_request" {
			t.Errorf("%s: code = %q, want invalid_request", c.name, e.Code)
		}
	}

	m := dungMayChu(t)
	coContentUpdate(t, m)
	doiMa(t, m.goiGhi(t, bulkRoute(`{"items":[`), hostA, m.tokenCho(t, xaA, sidA)), http.StatusBadRequest)
	if n := m.ghiDanhBa.soLanGoi(); n != 0 {
		t.Errorf("broken JSON, yet the use case ran %d times", n)
	}
}

// THE BODY IS BOUNDED: a payload past bulkPublicationBodyMax is refused before the use case.
func TestBulkPublicationBodyTooLargeIs400(t *testing.T) {
	m := dungMayChu(t)
	coContentUpdate(t, m)
	huge := `{"items":[{"id":"` + strings.Repeat("x", bulkPublicationBodyMax) + `"}]}`

	doiMa(t, m.goiGhi(t, bulkRoute(huge), hostA, m.tokenCho(t, xaA, sidA)), http.StatusBadRequest)
	if n := m.ghiDanhBa.soLanGoi(); n != 0 {
		t.Errorf("oversized body, yet the use case ran %d times", n)
	}
}

// A STORE OR AUDIT FAILURE (the whole batch rolled back) is 500, never a per-row list.
func TestBulkPublicationStoreFailureIs500(t *testing.T) {
	m := dungMayChu(t)
	coContentUpdate(t, m)
	m.ghiDanhBa.loi = errors.New("audit down")

	doiMa(t, m.goiGhi(t, bulkRoute(bulkBodyOK), hostA, m.tokenCho(t, xaA, sidA)), http.StatusInternalServerError)
}

// AN UNEXPECTED REFUSAL ON A ROW is a defect, answered 500 — never "skipped" with an invented code.
func TestBulkPublicationUnknownRefusalIs500(t *testing.T) {
	m := dungMayChu(t)
	coContentUpdate(t, m)
	m.ghiDanhBa.bulkOutcomes = []app.BulkPublishOutcome{{ID: "nd-1", Refusal: errors.New("surprise")}}

	doiMa(t, m.goiGhi(t, bulkRoute(bulkBodyOK), hostA, m.tokenCho(t, xaA, sidA)), http.StatusInternalServerError)
}

// THE IDEMPOTENCY KEY IS REQUIRED — refused before the use case runs.
func TestBulkPublicationRequiresIdempotencyKey(t *testing.T) {
	m := dungMayChu(t)
	coContentUpdate(t, m)

	doiMa(t, m.goiGhiKhoa(t, bulkRoute(bulkBodyOK), hostA, m.tokenCho(t, xaA, sidA), ""), http.StatusBadRequest)
	if n := m.ghiDanhBa.soLanGoi(); n != 0 {
		t.Errorf("no Idempotency-Key, yet the use case ran %d times", n)
	}
}
