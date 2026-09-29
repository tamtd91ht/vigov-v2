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
	m := newTestServer(t)
	m.grant(tenantA, "admin.audit")
	wantStatus(t, m.call(t, http.MethodGet, hostA, auditEntriesPath, nil), http.StatusUnauthorized)
	if m.auditLog.calls != 0 {
		t.Error("audit log read with no session")
	}
}

func TestAuditEntries_403WrongPermission(t *testing.T) {
	m := newTestServer(t)
	m.grant(tenantA, "document.read", "admin.lookup")
	wantStatus(t, m.call(t, http.MethodGet, hostA, auditEntriesPath, staffOf(tenantA)), http.StatusForbidden)
	if m.auditLog.calls != 0 {
		t.Error("audit log read without admin.audit")
	}
	if got := m.checker.lastAsked(); got != "admin.audit" {
		t.Errorf("checker asked %q, want admin.audit", got)
	}
}

func TestAuditEntries_403RightPermissionWrongCommune(t *testing.T) {
	// Granted in commune A; the account is commune B's, signed in at commune B.
	m := newTestServer(t)
	m.grant(tenantA, "admin.audit")
	wantStatus(t, m.call(t, http.MethodGet, hostB, auditEntriesPath, staffOf(tenantB)), http.StatusForbidden)
	if m.auditLog.calls != 0 {
		t.Error("commune B read its audit log with commune A's grant")
	}
}

func TestAuditEntries_200PassesFiltersReaderAndCommune(t *testing.T) {
	m := newTestServer(t)
	m.grant(tenantA, "admin.audit")
	at := time.Date(2026, 9, 28, 3, 0, 0, 0, time.UTC)
	m.auditLog.res = page.Result[audit.EntryView]{
		Items:   []audit.EntryView{{At: at, ActorKind: "staff", ActorCode: "CB-00777", Action: "vao_so_van_ban_den", Subject: "VBD-8"}},
		HasMore: true, NextCursor: "c2",
	}
	w := m.call(t, http.MethodGet, hostA, auditEntriesPath+
		"?from=2026-09-01T00:00:00Z&to=2026-10-01T00:00:00Z&actor=CB-00777&action=vao_so_van_ban_den"+
		"&subject=VBD-8&limit=5&cursor=c1&tenant_id=01JOTHERCOMMUNE", staffOf(tenantA))
	wantStatus(t, w, http.StatusOK)

	f := m.auditLog
	if f.commune != tenantA {
		t.Errorf("read in commune %q, want the Host's %q — never a query parameter", f.commune, tenantA)
	}
	if f.reader.ID != staffCode || f.reader.Kind != "staff" || f.reader.IP == "" {
		t.Errorf("reader = %+v, want business code %s (rule 6 inv 8), kind and socket IP", f.reader, staffCode)
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
			m := newTestServer(t)
			m.grant(tenantA, "admin.audit")
			m.auditLog.err = c.err
			w := m.call(t, http.MethodGet, hostA, auditEntriesPath, staffOf(tenantA))
			wantStatus(t, w, c.want)
			if e := errorBody(t, w); e.Message == "" || e.Message == c.err.Error() {
				t.Errorf("error body = %+v — a sentence for a person, never the raw error", e)
			}
		})
	}
}

func TestAuditEntries_NoBusinessCodeIs500AndReadsNothing(t *testing.T) {
	m := newTestServer(t)
	m.grant(tenantA, "admin.audit")
	p := staffOf(tenantA)
	p.Ma = ""
	wantStatus(t, m.call(t, http.MethodGet, hostA, auditEntriesPath, p), http.StatusInternalServerError)
	if m.auditLog.calls != 0 {
		t.Error("read ran for a reader with no business code — its entry could not name who read")
	}
}
