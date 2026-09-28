package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
)

// GET /api/v1/documents-audit-entries — the rule 5 invariant 7 set, plus what the handler itself
// owns: which query parameters reach core/audit, who the reader is, and how a failure is answered.
// The reader's own behaviour (filters, order, the read's entry) is core/audit/read_test.go.

const auditEntriesPath = "/api/v1/documents-audit-entries"

// auditLogFake records the commune it was called in, read from the context the way *store.Scoped
// reads it — a fake that ignored the commune would let a wrong-commune test pass proving nothing.
type auditLogFake struct {
	calls   int
	commune tenant.ID
	reader  audit.Actor
	query   audit.Query
	res     page.Result[audit.EntryView]
	err     error
}

func (f *auditLogFake) Read(ctx context.Context, reader audit.Actor, q audit.Query) (page.Result[audit.EntryView], error) {
	f.calls++
	f.commune, f.reader, f.query = tenant.MustFrom(ctx), reader, q
	if f.err != nil {
		return page.NewResult[audit.EntryView](), f.err
	}
	return f.res, nil
}

func TestAuditEntries_401WithoutSession(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(xaA, "admin.audit")
	doiMa(t, m.goi(t, http.MethodGet, hostA, auditEntriesPath, nil), http.StatusUnauthorized)
	if m.auditLog.calls != 0 {
		t.Error("audit log read with no session")
	}
}

func TestAuditEntries_403WrongPermission(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(xaA, "document.read", "admin.lookup")
	doiMa(t, m.goi(t, http.MethodGet, hostA, auditEntriesPath, canBoCua(xaA)), http.StatusForbidden)
	if m.auditLog.calls != 0 {
		t.Error("audit log read without admin.audit")
	}
	if got := m.checker.hoiKhoaCuoi(); got != "admin.audit" {
		t.Errorf("checker asked %q, want admin.audit", got)
	}
}

func TestAuditEntries_403RightPermissionWrongCommune(t *testing.T) {
	// Granted in commune A; the account is commune B's, signed in at commune B.
	m := dungMayChu(t)
	m.capQuyen(xaA, "admin.audit")
	doiMa(t, m.goi(t, http.MethodGet, hostB, auditEntriesPath, canBoCua(xaB)), http.StatusForbidden)
	if m.auditLog.calls != 0 {
		t.Error("commune B read its audit log with commune A's grant")
	}
}

func TestAuditEntries_200PassesFiltersReaderAndCommune(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(xaA, "admin.audit")
	at := time.Date(2026, 9, 28, 3, 0, 0, 0, time.UTC)
	m.auditLog.res = page.Result[audit.EntryView]{
		Items:   []audit.EntryView{{At: at, ActorKind: "staff", ActorCode: "CB-00777", Action: "vao_so_van_ban_den", Subject: "VBD-8"}},
		HasMore: true, NextCursor: "c2",
	}
	w := m.goi(t, http.MethodGet, hostA, auditEntriesPath+
		"?from=2026-09-01T00:00:00Z&to=2026-10-01T00:00:00Z&actor=CB-00777&action=vao_so_van_ban_den"+
		"&subject=VBD-8&limit=5&cursor=c1&tenant_id=01JOTHERCOMMUNE", canBoCua(xaA))
	doiMa(t, w, http.StatusOK)

	f := m.auditLog
	if f.commune != xaA {
		t.Errorf("read in commune %q, want the Host's %q — never a query parameter", f.commune, xaA)
	}
	if f.reader.ID != maCanBo || f.reader.Kind != "staff" || f.reader.IP == "" {
		t.Errorf("reader = %+v, want business code %s (rule 6 inv 8), kind and socket IP", f.reader, maCanBo)
	}
	want := audit.Query{From: "2026-09-01T00:00:00Z", To: "2026-10-01T00:00:00Z", Actor: "CB-00777",
		Action: "vao_so_van_ban_den", Subject: "VBD-8", Limit: "5", Cursor: "c1"}
	if f.query != want {
		t.Errorf("query = %+v, want %+v", f.query, want)
	}

	var body struct {
		Items      []map[string]any `json:"items"`
		NextCursor string           `json:"next_cursor"`
		HasMore    bool             `json:"has_more"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Items) != 1 || body.Items[0]["actor_code"] != "CB-00777" || !body.HasMore || body.NextCursor != "c2" {
		t.Errorf("body = %s", w.Body.String())
	}
}

func TestAuditEntries_FailuresMapped(t *testing.T) {
	for name, c := range map[string]struct {
		err  error
		want int
	}{
		"bad range":  {audit.ErrRange, http.StatusBadRequest},
		"bad cursor": {page.ErrCursor, http.StatusBadRequest},
		"store down": {errors.New("connection reset"), http.StatusInternalServerError},
	} {
		t.Run(name, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(xaA, "admin.audit")
			m.auditLog.err = c.err
			w := m.goi(t, http.MethodGet, hostA, auditEntriesPath, canBoCua(xaA))
			doiMa(t, w, c.want)
			if e := loiTra(t, w); e.Message == "" || e.Message == c.err.Error() {
				t.Errorf("error body = %+v — a sentence for a person, never the raw error", e)
			}
		})
	}
}

func TestAuditEntries_NoBusinessCodeIs500AndReadsNothing(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(xaA, "admin.audit")
	p := canBoCua(xaA)
	p.Ma = ""
	doiMa(t, m.goi(t, http.MethodGet, hostA, auditEntriesPath, p), http.StatusInternalServerError)
	if m.auditLog.calls != 0 {
		t.Error("read ran for a reader with no business code — its entry could not name who read")
	}
}
