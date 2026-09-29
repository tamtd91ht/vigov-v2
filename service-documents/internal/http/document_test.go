package http

// WHAT THIS FILE IS FOR: the ELEVEN routes of the two document registers.
//
// FIVE THINGS, each of which fails silently if it stops holding:
//
//  1. the four cases of rule 5, invariant 7, on EVERY one of the eleven routes — and the third case
//     is the one that is easy to fake, see TestDocuments_403RightPermissionWrongCommune;
//  2. each route asks for the key it is SUPPOSED to ask for, compared against a literal rather than
//     against a constant this file also defines — the only assertion here a fake checker cannot
//     satisfy by accident;
//  3. `number`, `status` and `due_at` are REFUSED, not ignored. The first of the three is the one
//     that matters: a client that could name its own register number could reissue a number already
//     printed on a sealed document (rule 7, invariant 3);
//  4. the commune and the acting person reach the use case, because they are what the audit entry is
//     filed under (rule 6, invariant 2) — and the register rows of one commune never reach another;
//  5. the register never answers with a commitment it invented: an SLA that is not configured is a
//     409 naming the screen that fixes it, not a document booked with a made-up deadline.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/service-documents/internal/app"
	"github.com/vihat/vigov/service-documents/internal/domain"
	docstore "github.com/vihat/vigov/service-documents/internal/store"
)

const (
	pathIncoming = "/api/v1/incoming-documents"
	pathOutgoing = "/api/v1/outgoing-documents"
)

// The three keys these routes declare. They live HERE, in the test, and not in routes.go —
// tools/apidoc refuses a key that is not a string literal at the RequirePermission call site, so the
// routes spell them out (see the block above Register).
//
// A second spelling is a second place to drift, so the drift itself is what
// TestDocuments_PermissionKeysArePermissionTableStrings asserts: it reads the key the ROUTE asked for
// and compares it with a literal, not with these constants.
const (
	PermDocumentCreate authz.Perm = "document.create"
	PermDocumentRead   authz.Perm = "document.read"
	PermDocumentRoute  authz.Perm = "document.route"
)

// registerBody is the smallest valid booking body. Written out rather than marshalled from a struct
// so a test can send a field the Go type does not have — which is the whole point of case (3).
const registerBody = `{"received_date":"2026-09-22","issuing_body":"Huyện uỷ",` +
	`"document_type":"cong-van","summary":"Về việc rà soát hộ nghèo"}`

const issueBody = `{"document_date":"2026-09-22","document_type":"cong-van",` +
	`"summary":"Trả lời đơn của công dân","recipient":"UBND huyện"}`

// documentRoute is one of the eleven routes, so the four permission cases are asserted on ALL of
// them rather than on whichever one was written first.
type documentRoute struct {
	name   string
	method string
	path   string
	body   string
	perm   authz.Perm // the key this route is SUPPOSED to declare
	ok     int        // the status a correctly-permitted call returns
}

func documentRoutes() []documentRoute {
	return []documentRoute{
		{"POST đến", http.MethodPost, pathIncoming, registerBody, PermDocumentCreate, http.StatusCreated},
		{"PATCH đến", http.MethodPatch, pathIncoming + "/vbd-a-001", `{"summary":"Sửa trích yếu"}`,
			PermDocumentCreate, http.StatusOK},
		{"DELETE đến", http.MethodDelete, pathIncoming + "/vbd-a-001", `{"reason":"vào sổ nhầm"}`,
			PermDocumentCreate, http.StatusNoContent},
		{"POST chuyển", http.MethodPost, pathIncoming + "/vbd-a-001/routings",
			`{"to_unit":"bp-dia-chinh","reason":"Thuộc thẩm quyền bộ phận Địa chính"}`,
			PermDocumentRoute, http.StatusOK},
		{"GET đến", http.MethodGet, pathIncoming, "", PermDocumentRead, http.StatusOK},
		{"GET chi tiết đến", http.MethodGet, pathIncoming + "/vbd-a-001", "", PermDocumentRead,
			http.StatusOK},
		{"GET lịch sử chuyển", http.MethodGet, pathIncoming + "/vbd-a-001/routings", "",
			PermDocumentRead, http.StatusOK},

		{"POST đi", http.MethodPost, pathOutgoing, issueBody, PermDocumentCreate, http.StatusCreated},
		{"PATCH đi", http.MethodPatch, pathOutgoing + "/vbdi-a-001", `{"summary":"Sửa trích yếu"}`,
			PermDocumentCreate, http.StatusOK},
		{"DELETE đi", http.MethodDelete, pathOutgoing + "/vbdi-a-001", `{"reason":"cấp số nhầm"}`,
			PermDocumentCreate, http.StatusNoContent},
		{"GET đi", http.MethodGet, pathOutgoing, "", PermDocumentRead, http.StatusOK},
	}
}

func (m *testServer) documentWrites() int {
	return m.incomingWriter.totalCalls() + m.outgoingWriter.totalCalls()
}
func (m *testServer) documentReads() int {
	return m.incoming.calls + m.outgoing.calls + m.incomingReader.totalCalls()
}

// --- (2) the permission keys themselves --------------------------------------------------------

func TestDocuments_PermissionKeysArePermissionTableStrings(t *testing.T) {
	// THE KEY EACH ROUTE ACTUALLY ASKS FOR, COMPARED AGAINST A LITERAL — and that is the whole point.
	//
	// Every other assertion in this file grants the key and then expects that same value to be
	// accepted, so changing a route's key to ANY string at all would leave them all green: the fake
	// checker grants whatever it was handed. That is not hypothetical. This repository carried three
	// invented keys — `finance.read`, `map.read`, `document.approve` — through several sessions with
	// every suite green, because a route holding a key the `quyen` table lacks answers 403 to EVERY
	// account, forever, and nothing says so (rule 5, invariant 3c).
	//
	// All three keys are seeded at service-identity/migrations/0001_init.sql:287-289 and listed in
	// the permission matrix at docs/ui-ux/14-cau-hinh.md:122-124. tools/check_quyen.py scans the
	// whole repository against that table on every `make check` and is the guard a fixture cannot
	// fool; this is the cheap half that turns red in `go test` too.
	for _, tc := range documentRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestServer(t)
			m.callWithBody(t, tc.method, hostA, tc.path, staffOf(tenantA), tc.body)

			got := m.checker.lastAsked()
			want := map[authz.Perm]string{
				PermDocumentCreate: "document.create",
				PermDocumentRead:   "document.read",
				PermDocumentRoute:  "document.route",
			}[tc.perm]
			if string(got) != want {
				t.Fatalf("tuyến hỏi khoá %q, muốn %q — một khoá bảng `quyen` không có là một tuyến "+
					"trả 403 với MỌI tài khoản, mãi mãi, và không phép kiểm nào đỏ", got, want)
			}
		})
	}
}

// --- (1) the four cases of rule 5, invariant 7 --------------------------------------------------

func TestDocuments_401WithoutSession(t *testing.T) {
	for _, tc := range documentRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestServer(t)
			m.grant(tenantA, tc.perm) // granted, and still refused: there is nobody to grant it to

			wantStatus(t, m.callWithBody(t, tc.method, hostA, tc.path, nil, tc.body), http.StatusUnauthorized)
			if m.documentWrites() != 0 || m.documentReads() != 0 {
				t.Error("chưa đăng nhập mà sổ văn bản đã bị đụng tới")
			}
		})
	}
}

func TestDocuments_403WrongPermission(t *testing.T) {
	// A signed-in account of the right commune holding a DIFFERENT permission. `admin.lookup` is
	// deliberately a real key of this service — the failure being guarded against is not "an account
	// with nothing", it is an administrator who may manage the type catalogue being able to book, to
	// remove, or to read the commune's correspondence.
	for _, tc := range documentRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestServer(t)
			m.grant(tenantA, "admin.lookup")

			wantStatus(t, m.callWithBody(t, tc.method, hostA, tc.path, staffOf(tenantA), tc.body),
				http.StatusForbidden)
			if m.documentWrites() != 0 || m.documentReads() != 0 {
				t.Error("sai quyền mà sổ văn bản vẫn chạy")
			}
			if m.checker.lastAsked() != tc.perm {
				t.Errorf("tuyến hỏi khoá %q, muốn %q — một khoá khác là một quyền khác",
					m.checker.lastAsked(), tc.perm)
			}
		})
	}
}

func TestDocuments_403RightPermissionWrongCommune(t *testing.T) {
	// THE CASE THAT IS EASIEST TO FAKE AND HARDEST TO GET RIGHT, so read what it actually sets up.
	//
	// The account belongs to commune B and is signed in AT COMMUNE B: nothing about the request is
	// malformed, and authz's own commune comparison passes. What is wrong is the GRANT — the right
	// to work in the register was given in commune A. A checker that ignored the commune would answer
	// yes here, and commune A's clerk would be booking documents into commune B's register.
	//
	// That is rule 5, invariant 3 in one sentence: a permission missing its commune is cross-commune
	// escalation, not a lesser bug. And it answers 403 rather than 401 because the session is
	// perfectly valid — it is the authority that is absent.
	for _, tc := range documentRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestServer(t)
			m.grant(tenantA, tc.perm)

			wantStatus(t, m.callWithBody(t, tc.method, hostB, tc.path, staffOf(tenantB), tc.body),
				http.StatusForbidden)
			if m.documentWrites() != 0 || m.documentReads() != 0 {
				t.Error("quyền cấp ở xã khác mà vẫn vào được sổ của xã này")
			}
		})
	}
}

func TestDocuments_401SessionOfAnotherCommune(t *testing.T) {
	// The other shape of "wrong commune": a principal issued by commune A presented at commune B's
	// domain. authz.RequirePermission compares the commune BEFORE the permission and answers 401,
	// not 403 — a browser does not send a cookie across hosts, so this is never an ordinary user
	// error. Asserted so that nobody "corrects" it to 403 and turns a deliberate probe into something
	// that reads like a permissions problem.
	for _, tc := range documentRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestServer(t)
			m.grant(tenantA, tc.perm)
			m.grant(tenantB, tc.perm)

			wantStatus(t, m.callWithBody(t, tc.method, hostB, tc.path, staffOf(tenantA), tc.body),
				http.StatusUnauthorized)
			if m.documentWrites() != 0 || m.documentReads() != 0 {
				t.Error("phiên của xã khác mà vẫn vào được sổ")
			}
		})
	}
}

func TestDocuments_RightPermissionRightCommune(t *testing.T) {
	for _, tc := range documentRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestServerWithIdem(t)
			m.grant(tenantA, tc.perm)

			w := m.callWithBody(t, tc.method, hostA, tc.path, staffOf(tenantA), tc.body)
			wantStatus(t, w, tc.ok)

			// (4) THE COMMUNE AND THE ACTING PERSON REACHED THE LAYER BELOW. The audit entry is filed
			// under both (rule 6, invariant 2), and `actor_id` must hold the staff BUSINESS CODE —
			// `CB-00123` names somebody years later with no lookup still alive; a ULID names nobody
			// (rule 6, invariant 8).
			if tc.method == http.MethodGet {
				if m.documentReads() != 1 {
					t.Fatalf("tuyến đọc gọi kho %d lần, muốn 1", m.documentReads())
				}
				return
			}
			if m.documentWrites() != 1 {
				t.Fatalf("use case ghi chạy %d lần, muốn 1", m.documentWrites())
			}
			actor := m.incomingWriter.lastActor
			tenantID := m.incomingWriter.lastTenant
			if m.outgoingWriter.totalCalls() == 1 {
				actor, tenantID = m.outgoingWriter.lastActor, m.outgoingWriter.lastTenant
			}
			if tenantID != tenantA {
				t.Errorf("use case chạy trong xã %q, muốn %q", tenantID, tenantA)
			}
			if actor.ID != staffCode {
				t.Errorf("chủ thể vết = %q, muốn MÃ cán bộ %q — `audit_log.actor_id` giữ mã nghiệp "+
					"vụ, không bao giờ giữ id nội bộ (luật 6 bất biến 8)", actor.ID, staffCode)
			}
			if actor.IP == "" {
				t.Error("vết không có địa chỉ IP — luật 6 bất biến 2 đòi ai · làm gì · từ IP nào")
			}
		})
	}
}

// --- (3) the three facts a client may never state -----------------------------------------------

func TestDocuments_RefusesClientSuppliedNumber(t *testing.T) {
	// THE MOST EXPENSIVE FIELD IN THE SERVICE. A client that could name its own register number
	// could put a number already printed on a sealed, delivered document onto a second one — and
	// rule 7, invariant 3 does not permit taking either of them back.
	//
	// REFUSED, NOT IGNORED, on all four write routes of both registers: a clerk who watched the
	// number they typed disappear would believe the register had accepted it.
	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"vào sổ đến", http.MethodPost, pathIncoming,
			`{"received_date":"2026-09-22","issuing_body":"Huyện uỷ","document_type":"cong-van","summary":"x","number":7}`},
		{"sửa đến", http.MethodPatch, pathIncoming + "/vbd-a-001", `{"number":7}`},
		{"cấp số đi", http.MethodPost, pathOutgoing,
			`{"document_date":"2026-09-22","document_type":"cong-van","summary":"x","recipient":"y","number":7}`},
		{"sửa đi", http.MethodPatch, pathOutgoing + "/vbdi-a-001", `{"number":7}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestServerWithIdem(t)
			m.grant(tenantA, PermDocumentCreate)

			w := m.callWithBody(t, tc.method, hostA, tc.path, staffOf(tenantA), tc.body)
			wantStatus(t, w, http.StatusBadRequest)
			if m.documentWrites() != 0 {
				t.Fatal("thân mang `number` mà use case vẫn chạy — số vào sổ / số đi là của sổ cấp")
			}
		})
	}
}

func TestDocuments_RefusesClientSuppliedStatusAndDeadline(t *testing.T) {
	// `status` — the state moves by ROUTING and by nothing else on this surface. A client that could
	// set it could create a document already `da-giai-quyet`: a record closed by nobody, counted as
	// done in every figure the commune reports.
	//
	// `due_at` — the commitment is this commune's SLA and this commune's calendar, computed once by
	// identity (rule 10, invariant 2). A client choosing its own deadline is a client choosing how
	// long the authority may take.
	for name, body := range map[string]string{
		"status": `{"received_date":"2026-09-22","issuing_body":"H","document_type":"cong-van","summary":"x","status":"da-giai-quyet"}`,
		"due_at": `{"received_date":"2026-09-22","issuing_body":"H","document_type":"cong-van","summary":"x","due_at":"2030-01-01T00:00:00Z"}`,
	} {
		t.Run(name, func(t *testing.T) {
			m := newTestServerWithIdem(t)
			m.grant(tenantA, PermDocumentCreate)

			wantStatus(t, m.callWithBody(t, http.MethodPost, hostA, pathIncoming, staffOf(tenantA), body),
				http.StatusBadRequest)
			if m.documentWrites() != 0 {
				t.Fatalf("thân mang `%s` mà use case vẫn chạy", name)
			}
		})
	}
}

// --- (5) an SLA nobody configured is a refusal, never an invented deadline -----------------------

func TestIncoming_UnconfiguredSLARefusedNamingTheScreen(t *testing.T) {
	// THE ORDINARY ANSWER IN EVERY COMMUNE TODAY, and it is the contract working rather than a bug:
	// `sla` is empty everywhere (service-identity migration 0008 seeds nothing) and the onboarding
	// step that fills it does not exist in this repository.
	//
	// WHAT THIS PINS IS THAT NOTHING FALLS BACK. A booked document with a deadline invented by
	// software is a commitment reported upward as though the authority made it (rule 10, forbidden
	// #3), and it would look completely normal on the screen. 409 with the name of the screen that
	// fixes it is the honest answer.
	m := newTestServerWithIdem(t)
	m.grant(tenantA, PermDocumentCreate)
	m.incomingWriter.err = app.ErrDeadlineNotEstablished

	w := m.callWithBody(t, http.MethodPost, hostA, pathIncoming, staffOf(tenantA), registerBody)
	wantStatus(t, w, http.StatusConflict)

	e := errorBody(t, w)
	if e.Code != "sla_chua_cau_hinh" {
		t.Fatalf("mã lỗi = %q, muốn \"sla_chua_cau_hinh\"", e.Code)
	}
	if !strings.Contains(e.Message, "Thời hạn xử lý") {
		t.Errorf("thông báo không chỉ ra màn hình cần sửa: %q", e.Message)
	}
}

func TestIncoming_RetiredDocumentTypeIs409(t *testing.T) {
	// 409 AND NOT 400: the value is well formed and the caller is allowed to send it. What is wrong
	// is this code against the state of the commune's catalogue — the type may have been retired
	// between the form loading and the clerk pressing save, and the sentence has to say so or the
	// clerk retypes the same value.
	m := newTestServerWithIdem(t)
	m.grant(tenantA, PermDocumentCreate)
	m.incomingWriter.err = docstore.ErrDocumentTypeNotInUse

	w := m.callWithBody(t, http.MethodPost, hostA, pathIncoming, staffOf(tenantA), registerBody)
	wantStatus(t, w, http.StatusConflict)
	if e := errorBody(t, w); e.Code != "document_type_unknown" {
		t.Fatalf("mã lỗi = %q, muốn \"document_type_unknown\"", e.Code)
	}
}

func TestIncoming_RoutingFinishedDocumentIs409(t *testing.T) {
	// A document the commune has settled is not routed further: doing so would reopen a closed
	// record as a side effect of a routing form. 409 and not 403 — the caller HOLDS `document.route`;
	// what is refused is this act on THIS document.
	m := newTestServer(t)
	m.grant(tenantA, PermDocumentRoute)
	m.incomingWriter.err = domain.ErrDocumentFinished

	w := m.callWithBody(t, http.MethodPost, hostA, pathIncoming+"/vbd-a-001/routings", staffOf(tenantA),
		`{"to_unit":"bp-dia-chinh","reason":"x"}`)
	wantStatus(t, w, http.StatusConflict)
	if e := errorBody(t, w); e.Code != "document_state" {
		t.Fatalf("mã lỗi = %q, muốn \"document_state\"", e.Code)
	}
}

func TestIncoming_RoutingWithoutReasonRefused(t *testing.T) {
	// THE REASON IS MANDATORY, and this assertion is what stops that from being quietly relaxed: the
	// timeline entry can never be edited afterwards (rule 7, forbidden #5), so a routing with no
	// reason is an instruction nobody can ever account for.
	//
	// It is refused by the USE CASE rather than by the handler, so this drives the real refusal
	// through the fake — see the app tests for the refusal itself.
	m := newTestServer(t)
	m.grant(tenantA, PermDocumentRoute)
	m.incomingWriter.err = domain.ErrMissingRoutingReason

	wantStatus(t, m.callWithBody(t, http.MethodPost, hostA, pathIncoming+"/vbd-a-001/routings",
		staffOf(tenantA), `{"to_unit":"bp-dia-chinh"}`), http.StatusBadRequest)
}

// --- (4) one commune's register never reaches another -------------------------------------------

func TestDocuments_ListReturnsOnlyTheHostCommunesRegister(t *testing.T) {
	// THE SECOND WALL, and the one this package owns. The first is the permission check; this is the
	// commune the QUERY ran in. The fake reads it from the context exactly as *store.Scoped reads it,
	// so a handler that passed a commune from anywhere else — a query parameter, a field on the
	// principal — turns this red.
	for _, tc := range []struct {
		name      string
		path      string
		own       string // a value that appears ONLY in this commune's fixture
		forbidden string // and one that appears only in the OTHER commune's
	}{
		{"đến", pathIncoming, "UBND huyện", "Sở Nội vụ xã B"},
		{"đi", pathOutgoing, "Trả lời đơn của công dân", "Báo cáo của xã B"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestServer(t)
			m.grant(tenantA, PermDocumentRead)

			w := m.call(t, http.MethodGet, hostA, tc.path, staffOf(tenantA))
			wantStatus(t, w, http.StatusOK)

			body := w.Body.String()
			if !strings.Contains(body, tc.own) {
				t.Fatalf("danh sách thiếu dòng của chính xã: %s", body)
			}
			if strings.Contains(body, tc.forbidden) {
				t.Fatalf("SỔ CỦA XÃ KHÁC LỌT SANG — rò rỉ giữa hai cơ quan nhà nước: %s", body)
			}
		})
	}
}

func TestDocuments_EmptyListIsAnArrayNotNull(t *testing.T) {
	// `items: []` AND NEVER `null`. A commune onboarded this morning has an empty register, and a
	// client that has to handle both shapes handles one of them wrong — usually by rendering nothing
	// and saying nothing.
	m := newTestServer(t)
	m.grant(tenantA, PermDocumentRead)
	m.incoming.byTenant = nil

	w := m.call(t, http.MethodGet, hostA, pathIncoming, staffOf(tenantA))
	wantStatus(t, w, http.StatusOK)

	var out page.Result[vanBanDenRa]
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	if out.Items == nil {
		t.Fatal("`items` là null — phải là []")
	}
	if !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Fatalf("`items` không phải mảng rỗng: %s", w.Body.String())
	}
}

// --- the filters -------------------------------------------------------------------------------

func TestIncoming_InvalidFilterRefusedNotIgnored(t *testing.T) {
	// A FILTER SILENTLY DROPPED IS THE FAILURE THIS GUARDS. The screen asked for one slice of the
	// register and would be handed the whole of it, with nothing saying so — which on this surface
	// means a clerk believing they are looking at every document of one kind.
	for name, query := range map[string]string{
		"trạng thái lạ":     "?status=khong-co-that",
		"năm không phải số": "?year=hai-nghin",
		"năm ngoài khoảng":  "?year=1999",
	} {
		t.Run(name, func(t *testing.T) {
			m := newTestServer(t)
			m.grant(tenantA, PermDocumentRead)

			wantStatus(t, m.call(t, http.MethodGet, hostA, pathIncoming+query, staffOf(tenantA)),
				http.StatusBadRequest)
			if m.incoming.calls != 0 {
				t.Error("bộ lọc sai mà vẫn chạy câu truy vấn")
			}
		})
	}
}

func TestIncoming_ValidFilterReachesStoreIntact(t *testing.T) {
	// The values reach the store AS VALUES. Nothing between the query string and the SQL assembles
	// text: they become bound parameters (see store.incomingFilterSQL), which is what keeps a
	// government register free of an injection point.
	m := newTestServer(t)
	m.grant(tenantA, PermDocumentRead)

	wantStatus(t, m.call(t, http.MethodGet, hostA,
		pathIncoming+"?year=2026&status=da-phan-cong&document_type=cong-van&holding_unit=bp-01&q=hộ%20nghèo",
		staffOf(tenantA)), http.StatusOK)

	want := docstore.IncomingDocumentFilter{
		Year: 2026, Status: "da-phan-cong", DocumentType: "cong-van",
		OrgUnitID: "bp-01", Search: "hộ nghèo",
	}
	if m.incoming.lastFilter != want {
		t.Fatalf("bộ lọc xuống kho = %+v, muốn %+v", m.incoming.lastFilter, want)
	}
}

// THE `q` CEILING IS 200 CHARACTERS, NOT 200 BYTES. "ệ" is three bytes in UTF-8, so 200 of them are
// 600 bytes: a byte cap refuses this search, and it refused anything past ~70 Vietnamese letters.
// Both registers, because both parse `q` and a fix on one is the copy that drifts.
func TestDocuments_SearchCeilingCountsRunesNotBytes(t *testing.T) {
	for _, s := range []struct {
		name, path string
		runes      int
		want       int
	}{
		{"đến 200 ký tự nhận", pathIncoming, 200, http.StatusOK},
		{"đến 201 ký tự từ chối", pathIncoming, 201, http.StatusBadRequest},
		{"đi 200 ký tự nhận", pathOutgoing, 200, http.StatusOK},
		{"đi 201 ký tự từ chối", pathOutgoing, 201, http.StatusBadRequest},
	} {
		t.Run(s.name, func(t *testing.T) {
			m := newTestServer(t)
			m.grant(tenantA, PermDocumentRead)

			search := strings.Repeat("ệ", s.runes)
			wantStatus(t, m.call(t, http.MethodGet, hostA, s.path+"?q="+url.QueryEscape(search), staffOf(tenantA)),
				s.want)

			calls := m.incoming.calls + m.outgoing.calls
			if s.want == http.StatusOK && calls != 1 {
				t.Fatalf("`q` hợp lệ mà kho được gọi %d lần, muốn 1", calls)
			}
			if s.want == http.StatusBadRequest && calls != 0 {
				t.Fatal("`q` quá trần mà vẫn chạy câu truy vấn")
			}
		})
	}
}

// --- the response shape --------------------------------------------------------------------------

func TestIncoming_ReturnsTheIssuedNumberNotTheClients(t *testing.T) {
	// The body sent carries no number at all; the response carries 8. That is the register issuing
	// it, which is the only way a number is ever produced.
	m := newTestServerWithIdem(t)
	m.grant(tenantA, PermDocumentCreate)

	w := m.callWithBody(t, http.MethodPost, hostA, pathIncoming, staffOf(tenantA), registerBody)
	wantStatus(t, w, http.StatusCreated)

	var out vanBanDenRa
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	if out.Number != 8 || out.Year != 2026 {
		t.Fatalf("số/năm trả về = %d/%d, muốn 8/2026", out.Number, out.Year)
	}
	if out.Status != string(domain.IncomingStatusRegistered) {
		t.Fatalf("trạng thái = %q, muốn %q", out.Status, domain.IncomingStatusRegistered)
	}
}

func TestIncoming_ResponseHasNoOverdueField(t *testing.T) {
	// `overdue` IS NOT ON THE WIRE, AND THIS TEST IS WHAT KEEPS IT OFF. A boolean beside the deadline
	// is a second representation of one fact, and the two disagree the moment a response is cached:
	// `overdue: false` next to a deadline that has since passed, with the screen believing the
	// boolean. Rule 10, invariant 3 — overdue is DERIVED, here by the client from `due_at`, and on
	// the server by domain.IncomingDocument.IsOverdue.
	m := newTestServerWithIdem(t)
	m.grant(tenantA, PermDocumentCreate)

	w := m.callWithBody(t, http.MethodPost, hostA, pathIncoming, staffOf(tenantA), registerBody)
	wantStatus(t, w, http.StatusCreated)

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	for _, forbidden := range []string{"overdue", "qua_han", "is_overdue"} {
		if _, ok := body[forbidden]; ok {
			t.Fatalf("phản hồi mang trường %q — quá hạn là SUY RA từ `due_at`, không phải một giá "+
				"trị thứ hai đi trên dây (luật 10 bất biến 3)", forbidden)
		}
	}
	if _, ok := body["due_at"]; !ok {
		t.Fatal("phản hồi thiếu `due_at` — không có nó thì client không suy ra được quá hạn")
	}
}

// --- idempotency ----------------------------------------------------------------------------------

func TestDocuments_NumberIssuingRoutesRequireIdempotencyKey(t *testing.T) {
	// THE TWO ROUTES THAT ISSUE A NUMBER DECLARE idem.Required. A double-submitted form without the
	// header must be refused BEFORE it reaches the handler: the second submission would take THE NEXT
	// NUMBER, and that is not a duplicate row somebody can remove — it is a second entry in an
	// archival register whose number can never be given back (rule 7, invariant 3).
	for name, tc := range map[string]struct {
		path string
		body string
	}{
		"vào sổ đến": {pathIncoming, registerBody},
		"cấp số đi":  {pathOutgoing, issueBody},
	} {
		t.Run(name, func(t *testing.T) {
			m := newTestServerWithIdem(t)
			m.grant(tenantA, PermDocumentCreate)

			// Built by hand: the harness always sets the header, which is what makes every other
			// assertion about the route rather than about the header.
			r := httptest.NewRequest(http.MethodPost, "https://"+hostA+tc.path,
				strings.NewReader(tc.body))
			r.Host = hostA
			r.RemoteAddr = "10.0.0.7:51000"
			r.Header.Set("Content-Type", "application/json")
			r = r.WithContext(context.WithValue(r.Context(), principalKey{}, *staffOf(tenantA)))
			w := httptest.NewRecorder()
			m.h.ServeHTTP(w, r)

			if w.Code != http.StatusBadRequest && w.Code != http.StatusPreconditionRequired {
				t.Fatalf("thiếu Idempotency-Key mà vẫn được %d — tuyến cấp số phải đòi khoá", w.Code)
			}
			if m.documentWrites() != 0 {
				t.Error("thiếu Idempotency-Key mà use case cấp số vẫn chạy")
			}
		})
	}
}

func TestDocuments_NoRedisClosesNumberIssuingRoutes(t *testing.T) {
	// THE DongKhiHong DECLARATION, PINNED. With no idempotency store reachable, the two routes that
	// ISSUE A NUMBER answer 503 and write nothing — and that is the choice, not an accident.
	//
	// MoKhiHong would let a double-submitted form through while the cache was down, and the second
	// submission would take THE NEXT NUMBER. That is not a duplicate row somebody can remove: it is a
	// second entry in an archival register, and rule 7, invariant 3 does not allow the number back.
	// Refusing to book while the cache is down is the cheaper failure, and it is visible.
	//
	// THE OTHER ROUTES ARE NOT AFFECTED — they declare idem.KhongCan — so a cache outage costs the
	// commune the ability to book and issue, not the whole register.
	m := newTestServer(t) // no store, on purpose
	m.grant(tenantA, PermDocumentCreate, PermDocumentRead, PermDocumentRoute)

	for name, tc := range map[string]struct {
		method, path, body string
		want               int
	}{
		"vào sổ đến": {http.MethodPost, pathIncoming, registerBody, http.StatusServiceUnavailable},
		"cấp số đi":  {http.MethodPost, pathOutgoing, issueBody, http.StatusServiceUnavailable},
		"sửa đến":    {http.MethodPatch, pathIncoming + "/vbd-a-001", `{"summary":"x"}`, http.StatusOK},
		"chuyển":     {http.MethodPost, pathIncoming + "/vbd-a-001/routings", `{"to_unit":"bp","reason":"x"}`, http.StatusOK},
		"đọc sổ đến": {http.MethodGet, pathIncoming, "", http.StatusOK},
	} {
		t.Run(name, func(t *testing.T) {
			wantStatus(t, m.callWithBody(t, tc.method, hostA, tc.path, staffOf(tenantA), tc.body), tc.want)
		})
	}
}

// --- the detail drawer: one document and its routing timeline ------------------------------------

func TestIncoming_DetailHasTheListRowShape(t *testing.T) {
	// ONE SHAPE FOR ONE ROW. The drawer is opened from a list row; a second shape would be a second
	// contract the web side has to keep in step with the first.
	m := newTestServer(t)
	m.grant(tenantA, PermDocumentRead)

	w := m.call(t, http.MethodGet, hostA, pathIncoming+"/vbd-a-001", staffOf(tenantA))
	wantStatus(t, w, http.StatusOK)

	var out vanBanDenRa
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	if out.ID != "vbd-a-001" || out.Number != 1 || out.IssuingBody != "UBND huyện" {
		t.Fatalf("chi tiết sai dòng: %+v", out)
	}
	if m.incomingReader.lastTenant != tenantA {
		t.Fatalf("đọc trong xã %q, muốn %q — xã phải lấy từ Host, không từ nơi nào khác",
			m.incomingReader.lastTenant, tenantA)
	}
}

func TestIncoming_DetailAndHistory404IsOneBodyForThreeCases(t *testing.T) {
	// THREE CAUSES, ONE ANSWER, BYTE FOR BYTE. An id that never existed, a document removed from the
	// register (rule 7, invariant 2) and a document of ANOTHER COMMUNE (rule 1). If any of the three
	// answered differently — a different status, code or sentence — a caller could probe which ids
	// another authority holds, or learn that a document was removed.
	for _, suffix := range []string{"", "/routings"} {
		t.Run("tuyến"+suffix, func(t *testing.T) {
			m := newTestServer(t)
			m.grant(tenantA, PermDocumentRead)
			m.incomingReader.removed["vbd-a-002"] = true

			var first string
			for name, id := range map[string]string{
				"không tồn tại": "vbd-khong-co",
				"xã khác":       "vbd-b-001",
				"đã gỡ":         "vbd-a-002",
			} {
				w := m.call(t, http.MethodGet, hostA, pathIncoming+"/"+id+suffix, staffOf(tenantA))
				wantStatus(t, w, http.StatusNotFound)
				body := w.Body.String()
				if strings.Contains(body, "Sở Nội vụ xã B") || strings.Contains(body, id) {
					t.Fatalf("%s: thân 404 mang dữ liệu hoặc mã của văn bản: %s", name, body)
				}
				if first == "" {
					first = body
				} else if body != first {
					t.Fatalf("%s: thân 404 khác các trường hợp kia — lộ sự tồn tại của văn bản:\n%s\nvs\n%s",
						name, body, first)
				}
			}
		})
	}
}

func TestIncoming_RoutingHistoryOldestFirstWithEveryColumn(t *testing.T) {
	// OLDEST FIRST, as the use case hands it over — the handler must not reorder. The SQL's
	// `ORDER BY thoi_diem ASC, id ASC` is asserted in the app tests; this pins that nothing between
	// the store and the wire undoes it.
	m := newTestServer(t)
	m.grant(tenantA, PermDocumentRead)

	w := m.call(t, http.MethodGet, hostA, pathIncoming+"/vbd-a-001/routings", staffOf(tenantA))
	wantStatus(t, w, http.StatusOK)

	var out danhSachLichSuChuyenRa
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	if len(out.Items) != 2 || out.Items[0].ID != "ls-1" || out.Items[1].ID != "ls-2" {
		t.Fatalf("thứ tự lịch sử sai, muốn ls-1 rồi ls-2: %+v", out.Items)
	}
	second := out.Items[1]
	want := lichSuChuyenRa{
		ID: "ls-2", DocumentID: "vbd-a-001", RoutedAt: "2026-09-23T09:00:00Z", RoutedBy: "CB-00002",
		Status: "dang-xu-ly", FromUnit: "bp-van-phong", ToUnit: "bp-dia-chinh", Assignee: "CB-00003",
		Reason: "Thuộc thẩm quyền bộ phận Địa chính",
	}
	if second != want {
		t.Fatalf("dòng lịch sử = %+v, muốn %+v", second, want)
	}
	if strings.Contains(w.Body.String(), "tenant") {
		t.Fatalf("phản hồi mang mã xã — người gọi đã là xã đó rồi: %s", w.Body.String())
	}
}

func TestIncoming_EmptyRoutingHistoryIsAnArray(t *testing.T) {
	// A document booked and never routed has a timeline of nothing — `items: []`, never `null`, and
	// never a 404: the document IS visible.
	m := newTestServer(t)
	m.grant(tenantA, PermDocumentRead)

	w := m.call(t, http.MethodGet, hostA, pathIncoming+"/vbd-a-002/routings", staffOf(tenantA))
	wantStatus(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Fatalf("`items` không phải mảng rỗng: %s", w.Body.String())
	}
}

func TestIncoming_RoutingHistoryOverCeilingIs500NotTruncated(t *testing.T) {
	// The ceiling refusal reaches the client as a plain 500 — never a short list that reads as whole.
	m := newTestServer(t)
	m.grant(tenantA, PermDocumentRead)
	// WRAPPED EXACTLY AS app.wrapDocumentErr WRAPS IT — the commune's ULID and the store's own sentence
	// ride inside err.Error(). A bare sentinel here would let a handler that echoes err.Error() pass.
	m.incomingReader.err = fmt.Errorf("van_ban: đọc lịch sử chuyển văn bản đến cho xã %s: %w",
		tenantA, docstore.ErrTooMuchRoutingHistory)

	w := m.call(t, http.MethodGet, hostA, pathIncoming+"/vbd-a-001/routings", staffOf(tenantA))
	wantStatus(t, w, http.StatusInternalServerError)
	body := w.Body.String()
	if strings.Contains(body, "items") {
		t.Fatalf("vượt trần mà vẫn trả danh sách: %s", body)
	}
	// THE INTERNAL SENTENCE STAYS ON THE SERVER. The ceiling is a server-side fault, not something the
	// clerk can act on, and the wrapped text names the commune's internal id and the store's wording.
	for _, leak := range []string{"vượt trần", "van_ban", string(tenantA)} {
		if strings.Contains(body, leak) {
			t.Fatalf("thân 500 mang chữ nội bộ %q: %s", leak, body)
		}
	}
}
