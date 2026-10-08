package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/app"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// GET /api/v1/staff-directory/{code}/email — the audited full-email read (ADR 0082 §3, owner
// decision 08/10/2026). Rule 5 invariant 7 through the REAL XacThuc edge, plus 404 for another
// commune's code, and what the handler hands the use case (the reader's STAFF CODE and socket IP).
// That the read and its entry share one committed transaction is app/staff_email_reveal_test.go.

func revealPath(code string) string { return "/api/v1/staff-directory/" + code + "/email" }

// staffEmailsFake is KEYED BY COMMUNE, read from the context the way *store.ScopedTx binds it — a
// fake keyed any other way would let the other-commune case pass while proving nothing. Each
// successful Reveal records one disclosure, standing in for the audit row the real use case writes
// in the same transaction.
type staffEmailsFake struct {
	byCommune map[tenant.ID]map[string]string
	err       error
	calls     int
	disclosed []revealRecord
}

type revealRecord struct {
	commune tenant.ID
	code    string
	actor   app.NguoiThucHien
}

func (f *staffEmailsFake) Reveal(ctx context.Context, code string, actor app.NguoiThucHien) (string, error) {
	f.calls++
	if f.err != nil {
		return "", f.err
	}
	commune := tenant.MustFrom(ctx)
	email, ok := f.byCommune[commune][code]
	if !ok {
		return "", idstore.ErrCanBoKhongTonTai
	}
	f.disclosed = append(f.disclosed, revealRecord{commune: commune, code: code, actor: actor})
	return email, nil
}

// staffEmailsSample: CB-005 exists in commune A only; CB-900 in commune B only.
func staffEmailsSample() *staffEmailsFake {
	return &staffEmailsFake{byCommune: map[tenant.ID]map[string]string{
		xaA: {"CB-005": "do.van.e@example.gov.vn"},
		xaB: {"CB-900": "vu.thi.f@example.gov.vn"},
	}}
}

// revealServer grants `task.read` in commune A and nothing in commune B.
func revealServer(t *testing.T) *mayChu {
	t.Helper()
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
			xaA: {idNoiBo: {authz.Perm("task.read"): true}},
			xaB: {},
		}}
	})
	return m
}

func TestRevealStaffEmail_401WithoutToken(t *testing.T) {
	m := revealServer(t)
	doiMa(t, m.goi(t, "GET", hostA, revealPath("CB-005"), "", ""), http.StatusUnauthorized)
	if m.staffEmails.calls != 0 {
		t.Error("email read with no session")
	}
}

func TestRevealStaffEmail_403WrongPermission(t *testing.T) {
	m := dungMayChu(t) // the shipped default: admin.user in commune A — not task.read
	doiMa(t, m.goi(t, "GET", hostA, revealPath("CB-005"), "", m.tokenCho(t, xaA, sidA)), http.StatusForbidden)
	if m.staffEmails.calls != 0 {
		t.Error("email read without task.read")
	}
}

func TestRevealStaffEmail_403RightPermissionWrongCommune(t *testing.T) {
	// The same person holds task.read in commune A and is properly signed in at commune B.
	m := revealServer(t)
	doiMa(t, m.goi(t, "GET", hostB, revealPath("CB-900"), "", m.tokenCho(t, xaB, sidB)), http.StatusForbidden)
	if m.staffEmails.calls != 0 {
		t.Error("commune B's address read with commune A's grant")
	}
}

func TestRevealStaffEmail_404OtherCommunesCode(t *testing.T) {
	// CB-900 exists — in commune B. Asked from commune A it is the same 404 as an invented code.
	m := revealServer(t)
	tok := m.tokenCho(t, xaA, sidA)

	other := m.goi(t, "GET", hostA, revealPath("CB-900"), "", tok)
	doiMa(t, other, http.StatusNotFound)
	invented := m.goi(t, "GET", hostA, revealPath("CB-777"), "", tok)
	doiMa(t, invented, http.StatusNotFound)
	if other.Body.String() != invented.Body.String() {
		t.Errorf("another commune's code answers differently from an invented one:\n%s\n%s",
			other.Body.String(), invented.Body.String())
	}
	if loiTra(t, other).Code != "staff_not_found" {
		t.Errorf("code = %q, want staff_not_found", loiTra(t, other).Code)
	}
	if strings.Contains(other.Body.String(), "vu.thi.f") {
		t.Fatalf("LEAK: commune B's address in commune A's answer: %s", other.Body.String())
	}
	if len(m.staffEmails.disclosed) != 0 {
		t.Errorf("nothing was disclosed, yet %d disclosures were recorded", len(m.staffEmails.disclosed))
	}
}

func TestRevealStaffEmail_200AndAudited(t *testing.T) {
	m := revealServer(t)

	w := m.goi(t, "GET", hostA, revealPath("CB-005"), "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %s", w.Body.String())
	}
	if len(body) != 1 || body["email"] != "do.van.e@example.gov.vn" {
		t.Fatalf("body = %s, want exactly {email}", w.Body.String())
	}
	if len(m.staffEmails.disclosed) != 1 {
		t.Fatalf("%d disclosures recorded, want 1", len(m.staffEmails.disclosed))
	}
	d := m.staffEmails.disclosed[0]
	if d.commune != xaA || d.code != "CB-005" {
		t.Errorf("disclosure %+v — want commune A (from Host), code CB-005", d)
	}
	// rule 6, invariant 8: the trail's "who" is the STAFF CODE, never the internal id.
	if d.actor.Vet.ID != maCanBo || d.actor.Vet.ID == idNoiBo || d.actor.Vet.Kind != "staff" || d.actor.Vet.IP == "" {
		t.Errorf("actor = %+v, want staff code %s, kind staff and the socket IP", d.actor.Vet, maCanBo)
	}

	// Two reads, two disclosures — never deduplicated.
	doiMa(t, m.goi(t, "GET", hostA, revealPath("CB-005"), "", m.tokenCho(t, xaA, sidA)), http.StatusOK)
	if len(m.staffEmails.disclosed) != 2 {
		t.Errorf("second read recorded %d disclosures in total, want 2", len(m.staffEmails.disclosed))
	}
}

func TestRevealStaffEmail_FailureDisclosesNothing(t *testing.T) {
	m := revealServer(t)
	m.staffEmails.err = errors.New("audit write failed")

	w := m.goi(t, "GET", hostA, revealPath("CB-005"), "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusInternalServerError)
	if strings.Contains(w.Body.String(), "do.van.e") || strings.Contains(w.Body.String(), "audit") {
		t.Errorf("internal failure leaked to the client: %s", w.Body.String())
	}
}

// --- the masked field on the picker -----------------------------------------------------------

func TestStaffDirectoryEmailMaskedPresentNeverRaw(t *testing.T) {
	m := dungMayChu(t)
	m.chonNguoi.theo[xaA][0].EmailMasked = "n***@example.gov.vn"
	// [1] has no address: email_masked must be null, not "" and not absent.

	w := m.goi(t, "GET", hostA, duongChonNguoi, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	var raw struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if got := string(raw.Items[0]["email_masked"]); got != `"n***@example.gov.vn"` {
		t.Errorf("items[0].email_masked = %s", got)
	}
	if got, ok := raw.Items[1]["email_masked"]; !ok || string(got) != "null" {
		t.Errorf("items[1].email_masked = %s (present %v), want null", got, ok)
	}
	if strings.Contains(w.Body.String(), `"email"`) {
		t.Errorf("the picker carries a raw `email` key: %s", w.Body.String())
	}
}
