package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// The two ADR 0102 routes (routes_contact_petition.go).
//
//	PROVED HERE   rule 5 invariant 7 on both routes, with BOTH keys required (either one alone is 403)
//	              and nothing read or asked in the refusals · the match: anonymous → not_applicable with
//	              ZERO identity calls, no stored key → not_applicable with zero calls, found / not_found,
//	              phone_unverified from the petition, `can-bo` without `feedback.restricted` → 404, identity
//	              error → 503 (never not_found) · the list: identity's number reaches the store and nowhere
//	              else, found = false → 200 empty, identity error → 503, the restricted fact handed down,
//	              paging defaults / cap / 400, total and items, the actor is Principal.Ma · no number and no
//	              contact id in any response body or log line.
//	NOT PROVED    the SQL (anonymous / deleted / `can-bo` exclusion, the partial-index predicate):
//	              internal/store/contact_petition_test.go. The RPCs: core/identityclient.

// matchPhoneOfContact is the number identity hands back for contactA. The agreed fake (rule 3, inv. 5).
const (
	matchPhoneOfContact = "0900000000"
	contactA            = "01JCONTACTAAAAAAAAAAAAAAAA"
	contactFoundID      = "01JCONTACTFOUNDFOUNDFOUNDF"
)

// senderDirectoryFake is identity's two RPCs, recording what it was asked and in which commune.
type senderDirectoryFake struct {
	findCalls, getCalls int
	findPhone           string
	findCommune         tenant.ID
	getContact, getAct  string
	getCommune          tenant.ID

	found     bool // FindCitizenContactByPhone's answer
	contactID string
	getFound  bool // GetCitizenContactMatchPhone's answer
	err       error
}

func (f *senderDirectoryFake) FindCitizenContactByPhone(ctx context.Context, phoneNumber string) (bool, string, error) {
	f.findCalls++
	f.findPhone, f.findCommune = phoneNumber, tenant.MustFrom(ctx)
	if f.err != nil {
		return false, "", f.err
	}
	if !f.found {
		return false, "", nil
	}
	return true, f.contactID, nil
}

func (f *senderDirectoryFake) GetCitizenContactMatchPhone(ctx context.Context, contactID, actorCode string) (bool, string, error) {
	f.getCalls++
	f.getContact, f.getAct, f.getCommune = contactID, actorCode, tenant.MustFrom(ctx)
	if f.err != nil {
		return false, "", f.err
	}
	if !f.getFound {
		return false, "", nil
	}
	return true, matchPhoneOfContact, nil
}

// contactPetitionsFake is the register's key reads, keyed by commune.
type contactPetitionsFake struct {
	keys            map[tenant.ID]map[string]string // petition id → stored key
	rows            []domain.ContactPetition        // what the list query answers (the SQL is the store's test)
	keyErr, listErr error

	listCalls             int
	listPhone             string
	listRestricted        bool
	listLimit, listOffset int
}

func newContactPetitionsFake() *contactPetitionsFake {
	at := time.Date(2026, 10, 1, 2, 0, 0, 0, time.UTC)
	return &contactPetitionsFake{
		keys: map[tenant.ID]map[string]string{
			// pa-001 (maPhieuThuong) has a key; pa-003 (staff-booked, no number) has none; pa-002 is anonymous.
			xaA: {"pa-001": matchPhoneOfContact, "pa-002": matchPhoneOfContact},
		},
		rows: []domain.ContactPetition{
			{LookupCode: maPhieuThuong, Content: "Đống rác   ở đầu ngõ\nđã ba ngày.", Status: domain.DangPhanLoai, ReceivedAt: at},
			{LookupCode: maPhieuNhapHo, Content: strings.Repeat("dài ", 60), Status: domain.DaTiepNhan, ReceivedAt: at.Add(-time.Hour)},
		},
	}
}

func (f *contactPetitionsFake) SenderMatchKey(ctx context.Context, petitionID string) (string, error) {
	if f.keyErr != nil {
		return "", f.keyErr
	}
	return f.keys[tenant.MustFrom(ctx)][petitionID], nil
}

func (f *contactPetitionsFake) ContactPetitions(_ context.Context, matchPhone string, restricted bool, limit, offset int) (
	[]domain.ContactPetition, error) {
	f.listCalls++
	f.listPhone, f.listRestricted, f.listLimit, f.listOffset = matchPhone, restricted, limit, offset
	if f.listErr != nil {
		return nil, f.listErr
	}
	if offset >= len(f.rows) {
		return []domain.ContactPetition{}, nil
	}
	return f.rows[offset:min(len(f.rows), offset+limit)], nil
}

func (f *contactPetitionsFake) CountContactPetitions(_ context.Context, matchPhone string, restricted bool) (int, error) {
	f.listPhone, f.listRestricted = matchPhone, restricted
	if f.listErr != nil {
		return 0, f.listErr
	}
	return len(f.rows), nil
}

func matchPath(code string) string { return duong(code) + "/directory-match" }

func byContactPath(id, query string) string {
	return "/api/v1/citizen-report-lists/by-contact/" + id + query
}

// --- rule 5 invariant 7, both routes, both keys ---------------------------------------------------------

type linkRoute struct {
	name, path string
	keys       [2]string
	calls      func(m *mayChu) int
}

func linkRoutes() []linkRoute {
	return []linkRoute{
		{"directory-match", matchPath(maPhieuThuong), [2]string{"feedback.unmask", "citizen.update"},
			func(m *mayChu) int { return m.directory.findCalls + m.phieu.goi }},
		{"by-contact", byContactPath(contactA, ""), [2]string{"citizen.read", "feedback.read"},
			func(m *mayChu) int { return m.directory.getCalls + m.contacts.listCalls }},
	}
}

func TestContactLinkRoutesNoSession401(t *testing.T) {
	for _, r := range linkRoutes() {
		t.Run(r.name, func(t *testing.T) {
			m := dungMayChu(t)
			grantBoth(m, t, r.keys[0], r.keys[1])
			doiMa(t, m.goi(t, http.MethodGet, hostA, r.path, nil), http.StatusUnauthorized)
			if r.calls(m) != 0 {
				t.Error("read or asked identity without a session")
			}
		})
	}
}

// Either key alone is 403 — both are required, not either. The read-only keys of the OTHER route are
// granted too, so a route accidentally declared with its sibling's pair would pass and fail here.
func TestContactLinkRoutesMissingEitherKey403(t *testing.T) {
	for _, r := range linkRoutes() {
		for i := range 2 {
			t.Run(r.name+"/only "+r.keys[i], func(t *testing.T) {
				m := dungMayChu(t)
				grantBoth(m, t, r.keys[i], "task.read")
				doiMa(t, m.goi(t, http.MethodGet, hostA, r.path, canBoCuaXa(xaA)), http.StatusForbidden)
				if r.calls(m) != 0 {
					t.Error("read or asked identity with one key")
				}
			})
		}
	}
}

// Right keys, granted in BOTH communes, and a session of commune B at commune A's host: refused before
// anything is read or asked (authz compares the commune first — 401, the repository's answer).
func TestContactLinkRoutesRightKeysWrongCommune(t *testing.T) {
	for _, r := range linkRoutes() {
		t.Run(r.name, func(t *testing.T) {
			m := dungMayChu(t)
			grantBoth(m, t, r.keys[0], r.keys[1])
			doiMa(t, m.goi(t, http.MethodGet, hostA, r.path, canBoCuaXa(xaB)), http.StatusUnauthorized)
			if r.calls(m) != 0 {
				t.Error("commune A read or asked with commune B's session")
			}
		})
	}
}

func TestContactLinkRoutesRightKeysRightCommune200(t *testing.T) {
	for _, r := range linkRoutes() {
		t.Run(r.name, func(t *testing.T) {
			m := dungMayChu(t)
			m.directory.found, m.directory.contactID, m.directory.getFound = true, contactFoundID, true
			grantBoth(m, t, r.keys[0], r.keys[1])
			doiMa(t, m.goi(t, http.MethodGet, hostA, r.path, canBoCuaXa(xaA)), http.StatusOK)
		})
	}
}

// --- the match ---------------------------------------------------------------------------------------------

func matchAs(t *testing.T, m *mayChu, code string) (*bytes.Buffer, directoryMatchOut, int) {
	t.Helper()
	var logs bytes.Buffer
	grantBoth(m, t, "feedback.unmask", "citizen.update")
	m.dungLai(t, func(d *Deps) { d.Log = slog.New(slog.NewTextHandler(&logs, nil)) })
	w := m.goi(t, http.MethodGet, hostA, matchPath(code), canBoCuaXa(xaA))
	var out directoryMatchOut
	if w.Code == http.StatusOK {
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatalf("body is not JSON: %q", w.Body.String())
		}
	}
	if strings.Contains(w.Body.String(), matchPhoneOfContact) || strings.Contains(logs.String(), matchPhoneOfContact) {
		t.Errorf("the number left the service: body=%q log=%q", w.Body.String(), logs.String())
	}
	return &logs, out, w.Code
}

func TestDirectoryMatchFound(t *testing.T) {
	m := dungMayChu(t)
	m.directory.found, m.directory.contactID = true, contactFoundID
	_, out, code := matchAs(t, m, maPhieuThuong)
	if code != http.StatusOK || out.Status != "found" || out.ContactID == nil || *out.ContactID != contactFoundID ||
		out.PhoneUnverified {
		t.Fatalf("code %d, out %+v", code, out)
	}
	// The STORED key is what identity is asked with, in the request's commune.
	if m.directory.findPhone != matchPhoneOfContact || m.directory.findCommune != xaA {
		t.Errorf("identity asked with %q in %q", m.directory.findPhone, m.directory.findCommune)
	}
}

func TestDirectoryMatchNotFoundCarriesNullID(t *testing.T) {
	m := dungMayChu(t)
	_, out, code := matchAs(t, m, maPhieuThuong)
	if code != http.StatusOK || out.Status != "not_found" || out.ContactID != nil {
		t.Fatalf("code %d, out %+v", code, out)
	}
}

// ANONYMOUS → not_applicable and identity is NEVER asked (ADR 0102 #4). maPhieuCanBo is anonymous and in
// `can-bo`, so the reader also holds `feedback.restricted` to get past the 404.
func TestDirectoryMatchAnonymousIsNotApplicableWithoutCallingIdentity(t *testing.T) {
	m := dungMayChu(t)
	m.directory.found, m.directory.contactID = true, contactFoundID
	grantBoth(m, t, "feedback.unmask", "citizen.update", "feedback.restricted")
	w := m.goi(t, http.MethodGet, hostA, matchPath(maPhieuCanBo), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"status":"not_applicable"`) || !strings.Contains(w.Body.String(), `"contact_id":null`) {
		t.Errorf("body = %s", w.Body.String())
	}
	if m.directory.findCalls != 0 {
		t.Errorf("identity asked %d times about an anonymous petition", m.directory.findCalls)
	}
}

// No stored key (staff-booked with no number, or not yet backfilled) → not_applicable, no call.
func TestDirectoryMatchWithoutKeyIsNotApplicableWithoutCallingIdentity(t *testing.T) {
	m := dungMayChu(t)
	_, out, code := matchAs(t, m, maPhieuNhapHo)
	if code != http.StatusOK || out.Status != "not_applicable" || m.directory.findCalls != 0 {
		t.Fatalf("code %d, out %+v, calls %d", code, out, m.directory.findCalls)
	}
}

func TestDirectoryMatchRestrictedFieldWithoutKeyIs404(t *testing.T) {
	m := dungMayChu(t)
	_, _, code := matchAs(t, m, maPhieuCanBo)
	if code != http.StatusNotFound || m.directory.findCalls != 0 {
		t.Fatalf("code %d, calls %d — want the detail's 404 and no identity call", code, m.directory.findCalls)
	}
}

func TestDirectoryMatchUnknownCodeIs404(t *testing.T) {
	m := dungMayChu(t)
	_, _, code := matchAs(t, m, maPhieuXaB) // commune B's code at commune A
	if code != http.StatusNotFound {
		t.Fatalf("code %d, want 404", code)
	}
}

func TestDirectoryMatchPhoneUnverifiedFromPetition(t *testing.T) {
	m := dungMayChu(t)
	p := m.phieu.theo[xaA][maPhieuThuong]
	p.CongDanID, p.ZaloAccountID = "", "01JZALOACCOUNTAAAAAAAAAAAA"
	m.phieu.theo[xaA][maPhieuThuong] = p
	_, out, code := matchAs(t, m, maPhieuThuong)
	if code != http.StatusOK || !out.PhoneUnverified {
		t.Fatalf("code %d, out %+v — want phone_unverified true", code, out)
	}
}

// IDENTITY DOWN → 503 with a sentence, NEVER not_found.
func TestDirectoryMatchIdentityErrorIs503(t *testing.T) {
	m := dungMayChu(t)
	m.directory.err = errors.Join(identityclient.ErrIdentityUnavailable, errors.New("rpc"))
	if _, _, code := matchAs(t, m, maPhieuThuong); code != http.StatusServiceUnavailable {
		t.Fatalf("code %d, want 503", code)
	}
}

func TestDirectoryMatch503BodyIsTheSentence(t *testing.T) {
	m := dungMayChu(t)
	m.directory.err = errors.New("rpc down")
	grantBoth(m, t, "feedback.unmask", "citizen.update")
	w := m.goi(t, http.MethodGet, hostA, matchPath(maPhieuThuong), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusServiceUnavailable)
	e := loiTra(t, w)
	if e.Code != "citizen_directory_unavailable" || !strings.Contains(e.Message, "danh bạ người dân") ||
		strings.Contains(w.Body.String(), "not_found") || strings.Contains(w.Body.String(), "rpc down") {
		t.Errorf("error = %+v", e)
	}
}

// --- the list ----------------------------------------------------------------------------------------------

func listAs(t *testing.T, m *mayChu, path string, extra ...string) (*bytes.Buffer, *contactPetitionPageOut, int, string) {
	t.Helper()
	var logs bytes.Buffer
	grantBoth(m, t, append([]string{"citizen.read", "feedback.read"}, extra...)...)
	m.dungLai(t, func(d *Deps) { d.Log = slog.New(slog.NewTextHandler(&logs, nil)) })
	w := m.goi(t, http.MethodGet, hostA, path, canBoCuaXa(xaA))
	body := w.Body.String()
	for _, leak := range []string{matchPhoneOfContact, contactA} {
		if strings.Contains(logs.String(), leak) {
			t.Errorf("%q in a log line: %q", leak, logs.String())
		}
	}
	if strings.Contains(body, matchPhoneOfContact) {
		t.Errorf("the number is in the response: %s", body)
	}
	if w.Code != http.StatusOK {
		return &logs, nil, w.Code, body
	}
	var out contactPetitionPageOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("body is not JSON: %q", body)
	}
	return &logs, &out, w.Code, body
}

func TestContactPetitionsListsByIdentityNumber(t *testing.T) {
	m := dungMayChu(t)
	m.directory.getFound = true
	_, out, code, body := listAs(t, m, byContactPath(contactA, ""))
	if code != http.StatusOK || out.Total != 2 || out.Page != 1 || out.PageSize != domain.ContactPetitionPageSizeDefault ||
		len(out.Items) != 2 {
		t.Fatalf("code %d, out %+v", code, out)
	}
	if m.directory.getContact != contactA || m.directory.getAct != maCanBo || m.directory.getCommune != xaA {
		t.Errorf("identity asked %q as %q in %q — want the path id, Principal.Ma (never .ID), commune A",
			m.directory.getContact, m.directory.getAct, m.directory.getCommune)
	}
	if m.contacts.listPhone != matchPhoneOfContact || m.contacts.listRestricted {
		t.Errorf("store got key %q restricted %v", m.contacts.listPhone, m.contacts.listRestricted)
	}
	first := out.Items[0]
	if first.LookupCode != maPhieuThuong || first.Title != "Đống rác ở đầu ngõ đã ba ngày." ||
		first.Status != string(domain.DangPhanLoai) || first.ReceivedAt.IsZero() {
		t.Errorf("first row = %+v", first)
	}
	// Cut at the bound (a trailing space trimmed), then the ellipsis.
	if r := []rune(out.Items[1].Title); len(r) > domain.ContactPetitionTitleMaxRunes+1 ||
		len(r) < domain.ContactPetitionTitleMaxRunes || r[len(r)-1] != '…' {
		t.Errorf("long title = %q, want cut at %d runes plus an ellipsis", out.Items[1].Title, domain.ContactPetitionTitleMaxRunes)
	}
	for _, field := range []string{"reporter", "phone", "content", "anonymous"} {
		if strings.Contains(body, `"`+field) {
			t.Errorf("row carries %q: %s", field, body)
		}
	}
}

func TestContactPetitionsHandsDownRestrictedFact(t *testing.T) {
	m := dungMayChu(t)
	m.directory.getFound = true
	if _, _, code, _ := listAs(t, m, byContactPath(contactA, ""), "feedback.restricted"); code != http.StatusOK {
		t.Fatalf("code %d", code)
	}
	if !m.contacts.listRestricted {
		t.Error("a holder of feedback.restricted was not handed the restricted fact")
	}
}

// found = false → 200 EMPTY: a deleted or another commune's row reads as a person with no petitions.
func TestContactPetitionsUnknownContactIsEmpty200(t *testing.T) {
	m := dungMayChu(t)
	_, out, code, _ := listAs(t, m, byContactPath(contactA, ""))
	if code != http.StatusOK || out.Total != 0 || len(out.Items) != 0 || out.Items == nil {
		t.Fatalf("code %d, out %+v — want 200 with an empty (non-null) list", code, out)
	}
	if m.contacts.listCalls != 0 {
		t.Error("the register was queried without a key")
	}
}

func TestContactPetitionsIdentityErrorIs503(t *testing.T) {
	m := dungMayChu(t)
	m.directory.err = errors.New("rpc down")
	_, _, code, body := listAs(t, m, byContactPath(contactA, ""))
	if code != http.StatusServiceUnavailable || !strings.Contains(body, "citizen_directory_unavailable") ||
		strings.Contains(body, "rpc down") {
		t.Fatalf("code %d, body %s — want 503, never an empty list", code, body)
	}
	if m.contacts.listCalls != 0 {
		t.Error("the register was queried after identity failed")
	}
}

func TestContactPetitionsPaging(t *testing.T) {
	m := dungMayChu(t)
	m.directory.getFound = true
	_, out, code, _ := listAs(t, m, byContactPath(contactA, "?page=2&page_size=1"))
	if code != http.StatusOK || out.Page != 2 || out.PageSize != 1 || out.Total != 2 || len(out.Items) != 1 ||
		out.Items[0].LookupCode != maPhieuNhapHo {
		t.Fatalf("code %d, out %+v", code, out)
	}
	if m.contacts.listLimit != 1 || m.contacts.listOffset != 1 {
		t.Errorf("limit %d offset %d, want 1 and 1", m.contacts.listLimit, m.contacts.listOffset)
	}
}

func TestContactPetitionsPageSizeCapped(t *testing.T) {
	m := dungMayChu(t)
	m.directory.getFound = true
	_, out, code, _ := listAs(t, m, byContactPath(contactA, "?page_size=500"))
	if code != http.StatusOK || out.PageSize != domain.ContactPetitionPageSizeMax ||
		m.contacts.listLimit != domain.ContactPetitionPageSizeMax {
		t.Fatalf("code %d, page_size %v, limit %d", code, out, m.contacts.listLimit)
	}
}

// A page past the end is an empty page with the real total — the list query is not run at all.
func TestContactPetitionsPagePastEnd(t *testing.T) {
	m := dungMayChu(t)
	m.directory.getFound = true
	_, out, code, _ := listAs(t, m, byContactPath(contactA, "?page=9"))
	if code != http.StatusOK || out.Total != 2 || len(out.Items) != 0 || m.contacts.listCalls != 0 {
		t.Fatalf("code %d, out %+v, list calls %d", code, out, m.contacts.listCalls)
	}
}

func TestContactPetitionsBadPaging400BeforeIdentity(t *testing.T) {
	for _, q := range []string{"?page=0", "?page=x", "?page_size=0", "?page_size=-3", "?page=10001"} {
		t.Run(q, func(t *testing.T) {
			m := dungMayChu(t)
			m.directory.getFound = true
			if _, _, code, _ := listAs(t, m, byContactPath(contactA, q)); code != http.StatusBadRequest {
				t.Fatalf("code %d, want 400", code)
			}
			if m.directory.getCalls != 0 {
				t.Error("identity asked for a request that can only fail")
			}
		})
	}
}

func TestContactPetitionsStoreErrorIs500WithoutLeak(t *testing.T) {
	m := dungMayChu(t)
	m.directory.getFound = true
	m.contacts.listErr = errors.New("db down")
	_, _, code, body := listAs(t, m, byContactPath(contactA, ""))
	if code != http.StatusInternalServerError || strings.Contains(body, "db down") {
		t.Fatalf("code %d body %s", code, body)
	}
}

// NO BUSINESS CODE → no disclosure: identity's entry must name a CB- code, never Principal.ID.
func TestContactPetitionsWithoutBusinessCodeRefuses(t *testing.T) {
	m := dungMayChu(t)
	m.directory.getFound = true
	grantBoth(m, t, "citizen.read", "feedback.read")
	p := canBoCuaXa(xaA)
	p.Ma = ""
	w := m.goi(t, http.MethodGet, hostA, byContactPath(contactA, ""), p)
	if w.Code == http.StatusOK || m.directory.getCalls != 0 {
		t.Fatalf("code %d, identity calls %d — want a refusal before identity", w.Code, m.directory.getCalls)
	}
}

func TestContactPetitionsNoStoreCacheHeader(t *testing.T) {
	m := dungMayChu(t)
	m.directory.getFound = true
	grantBoth(m, t, "citizen.read", "feedback.read")
	w := m.goi(t, http.MethodGet, hostA, byContactPath(contactA, ""), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q", w.Header().Get("Cache-Control"))
	}
}

// The production types satisfy the two interfaces cmd/server wires them as.
var (
	_ SenderDirectoryLookup = (*identityclient.Client)(nil)
	_ ContactPetitionReader = (*petstore.PhieuPhanAnhStore)(nil)
)
