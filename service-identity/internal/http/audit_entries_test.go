package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
)

// GET /api/v1/identity-audit-entries — the rule 5 invariant 7 set, through the REAL XacThuc edge,
// plus what the handler owns: which query parameters reach core/audit, who the reader is, and how a
// failure is answered. The reader's own behaviour is core/audit/read_test.go.

const auditEntriesPath = "/api/v1/identity-audit-entries"

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

// newAuditServer grants `admin.audit` in commune A and nothing in commune B. The harness default
// grants `admin.user`, which is what makes the wrong-permission case use a real, unrelated key.
func newAuditServer(t *testing.T) *mayChu {
	t.Helper()
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
			xaA: {idNoiBo: {authz.Perm("admin.audit"): true}},
			xaB: {},
		}}
	})
	return m
}

func TestAuditEntries_401WithoutToken(t *testing.T) {
	m := newAuditServer(t)
	doiMa(t, m.goi(t, "GET", hostA, auditEntriesPath, "", ""), http.StatusUnauthorized)
	if m.auditLog.calls != 0 {
		t.Error("audit log read with no session")
	}
}

func TestAuditEntries_403WrongPermission(t *testing.T) {
	m := dungMayChu(t) // the shipped default: admin.user in commune A
	doiMa(t, m.goi(t, "GET", hostA, auditEntriesPath, "", m.tokenCho(t, xaA, sidA)), http.StatusForbidden)
	if m.auditLog.calls != 0 {
		t.Error("audit log read with admin.user — admin.audit is its own key")
	}
}

func TestAuditEntries_403RightPermissionWrongCommune(t *testing.T) {
	// The same person holds admin.audit in commune A and is properly signed in at commune B.
	m := newAuditServer(t)
	doiMa(t, m.goi(t, "GET", hostB, auditEntriesPath, "", m.tokenCho(t, xaB, sidB)), http.StatusForbidden)
	if m.auditLog.calls != 0 {
		t.Error("commune B's audit log read with commune A's grant")
	}
}

func TestAuditEntries_200PassesFiltersReaderAndCommune(t *testing.T) {
	m := newAuditServer(t)
	m.auditLog.res = page.Result[audit.EntryView]{
		Items: []audit.EntryView{{At: time.Date(2026, 9, 28, 3, 0, 0, 0, time.UTC),
			ActorKind: "staff", ActorCode: "CB-00777", Action: "khoa_tai_khoan_can_bo", Subject: "CB-00555"}},
	}
	w := m.goi(t, "GET", hostA, auditEntriesPath+
		"?from=2026-09-01T00:00:00Z&to=2026-10-01T00:00:00Z&actor=CB-00777&action=khoa_tai_khoan_can_bo"+
		"&subject=CB-00555&limit=5&cursor=c1&tenant_id=01JOTHERCOMMUNE", "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	f := m.auditLog
	if f.commune != xaA {
		t.Errorf("read in commune %q, want the Host's %q — never a query parameter", f.commune, xaA)
	}
	if f.reader.ID != maCanBo || f.reader.ID == idNoiBo || f.reader.Kind != "staff" || f.reader.IP == "" {
		t.Errorf("reader = %+v, want business code %s (rule 6 inv 8), kind and socket IP", f.reader, maCanBo)
	}
	want := audit.Query{From: "2026-09-01T00:00:00Z", To: "2026-10-01T00:00:00Z", Actor: "CB-00777",
		Action: "khoa_tai_khoan_can_bo", Subject: "CB-00555", Limit: "5", Cursor: "c1"}
	if f.query != want {
		t.Errorf("query = %+v, want %+v", f.query, want)
	}
	var body page.Result[map[string]any]
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Items) != 1 || body.Items[0]["action"] != "khoa_tai_khoan_can_bo" {
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
			m := newAuditServer(t)
			m.auditLog.err = c.err
			w := m.goi(t, "GET", hostA, auditEntriesPath, "", m.tokenCho(t, xaA, sidA))
			doiMa(t, w, c.want)
			if e := loiTra(t, w); e.Message == "" || e.Message == c.err.Error() {
				t.Errorf("error body = %+v — a sentence for a person, never the raw error", e)
			}
		})
	}
}
