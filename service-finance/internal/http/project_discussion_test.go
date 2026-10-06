package http

// §8.1 issues and §8.4 discussion (migration 0015, user decision 06/10/2026): rule 5 invariant 7's
// four cases on ALL FIVE routes, commune isolation on a COLLIDING project id, the refusals each route
// may answer, and the two figures the list and the summary now carry (§7.2 latest issue, §3 open
// count).
//
// THE KEYS UNDER TEST follow the prototype's router: `budget.read` reads both tabs AND posts a
// message; `budget.update` records and resolves an issue. The wrong-key case grants the OTHER real
// `budget.*` keys, so a route that wanted "some budget key" rather than its own turns red.

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
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/app"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// --- fakes ---------------------------------------------------------------------------------------

var issueAt = time.Date(2026, time.August, 27, 2, 0, 0, 0, time.UTC)

// projectDiscussionFake reads KEYED BY COMMUNE from the context, as *store.Scoped does — keyed any
// other way, the isolation cases would pass while proving nothing. A project not in `projects` of
// the commune is ErrKhongThayDuAn, as the store answers.
type projectDiscussionFake struct {
	projects map[tenant.ID]map[string]bool
	issues   map[tenant.ID]map[string][]domain.ProjectIssue
	comments map[tenant.ID]map[string][]domain.ProjectComment
	latest   map[tenant.ID]map[string]domain.ProjectIssue
	open     map[tenant.ID]int

	err   error
	reads int
}

func newProjectDiscussionFake() *projectDiscussionFake {
	return &projectDiscussionFake{
		projects: map[tenant.ID]map[string]bool{
			xaA: {"da-001": true, "da-002": true},
			xaB: {"da-001": true},
		},
		issues: map[tenant.ID]map[string][]domain.ProjectIssue{
			xaA: {"da-001": {
				{ID: "vm-new", ProjectID: "da-001", Title: "Chưa bàn giao mặt bằng", Description: "Chờ huyện",
					RecordedBy: "CB-00002", RecordedAt: issueAt},
				{ID: "vm-old", ProjectID: "da-001", Title: "Nhà thầu chậm", RecordedBy: "CB-00001",
					RecordedAt: issueAt.Add(-48 * time.Hour), ResolvedAt: issueAt, ResolvedBy: "CB-00003"},
			}},
			xaB: {"da-001": {{ID: "vm-of-commune-b", ProjectID: "da-001", Title: "Của xã B",
				RecordedBy: "CB-90000", RecordedAt: issueAt}}},
		},
		comments: map[tenant.ID]map[string][]domain.ProjectComment{
			xaA: {"da-001": {{ID: "td-1", ProjectID: "da-001", Body: "Đề nghị kiểm lại", AuthorCode: "CB-00001",
				MentionedStaffCodes: nil, CreatedAt: issueAt}}},
		},
		latest: map[tenant.ID]map[string]domain.ProjectIssue{
			xaA: {"da-001": {ID: "vm-new", ProjectID: "da-001", Title: "Chưa bàn giao mặt bằng",
				RecordedAt: issueAt, ResolvedAt: issueAt, ResolvedBy: "CB-00003"}},
		},
		open: map[tenant.ID]int{xaA: 3},
	}
}

func (f *projectDiscussionFake) IssuesOfProject(ctx context.Context, id string) ([]domain.ProjectIssue, error) {
	f.reads++
	if f.err != nil {
		return nil, f.err
	}
	xa := tenant.MustFrom(ctx)
	if !f.projects[xa][id] {
		return nil, fistore.ErrKhongThayDuAn
	}
	return f.issues[xa][id], nil
}

func (f *projectDiscussionFake) CommentsOfProject(ctx context.Context, id string) ([]domain.ProjectComment, error) {
	f.reads++
	if f.err != nil {
		return nil, f.err
	}
	xa := tenant.MustFrom(ctx)
	if !f.projects[xa][id] {
		return nil, fistore.ErrKhongThayDuAn
	}
	return f.comments[xa][id], nil
}

func (f *projectDiscussionFake) LatestIssuesOfYear(ctx context.Context, _ fistore.LocDuAn) (map[string]domain.ProjectIssue, error) {
	f.reads++
	if f.err != nil {
		return nil, f.err
	}
	return f.latest[tenant.MustFrom(ctx)], nil
}

func (f *projectDiscussionFake) OpenIssueCount(ctx context.Context, _ int) (int, error) {
	f.reads++
	if f.err != nil {
		return 0, f.err
	}
	return f.open[tenant.MustFrom(ctx)], nil
}

// projectDiscussionWritesFake records the commune and the acting person of every call.
type projectDiscussionWritesFake struct {
	err error

	calls       int
	lastTenant  tenant.ID
	lastActor   audit.Actor
	lastID      string
	lastText    string
	lastComment app.ProjectCommentRequest
}

func (g *projectDiscussionWritesFake) note(ctx context.Context, id string, a audit.Actor) {
	g.calls++
	g.lastTenant, g.lastID, g.lastActor = tenant.MustFrom(ctx), id, a
}

func (g *projectDiscussionWritesFake) RecordIssue(ctx context.Context, projectID, text string,
	actor audit.Actor) (domain.ProjectIssue, error) {
	g.note(ctx, projectID, actor)
	g.lastText = text
	if g.err != nil {
		return domain.ProjectIssue{}, g.err
	}
	title, desc, _ := domain.SplitIssueText(text)
	return domain.ProjectIssue{ID: "vm-created", ProjectID: projectID, Title: title, Description: desc,
		RecordedBy: actor.ID, RecordedAt: issueAt}, nil
}

func (g *projectDiscussionWritesFake) ResolveIssue(ctx context.Context, issueID string,
	actor audit.Actor) (domain.ProjectIssue, error) {
	g.note(ctx, issueID, actor)
	if g.err != nil {
		return domain.ProjectIssue{}, g.err
	}
	return domain.ProjectIssue{ID: issueID, ProjectID: "da-001", Title: "x", RecordedBy: "CB-00001",
		RecordedAt: issueAt, ResolvedAt: issueAt.Add(time.Hour), ResolvedBy: actor.ID}, nil
}

func (g *projectDiscussionWritesFake) AddComment(ctx context.Context, projectID string,
	req app.ProjectCommentRequest, actor audit.Actor) (domain.ProjectComment, error) {
	g.note(ctx, projectID, actor)
	g.lastComment = req
	if g.err != nil {
		return domain.ProjectComment{}, g.err
	}
	return domain.ProjectComment{ID: "td-created", ProjectID: projectID, Body: req.Body,
		AuthorCode: actor.ID, MentionedStaffCodes: req.MentionedStaffCodes, CreatedAt: issueAt}, nil
}

// --- harness -------------------------------------------------------------------------------------

type discussionServer struct {
	h       http.Handler
	read    *projectDiscussionFake
	write   *projectDiscussionWritesFake
	checker *checkerDanhMucGia
}

// newDiscussionServer mounts the REAL routes through Register behind the REAL edge chain, with a
// checker KEYED BY COMMUNE — the one property the wrong-commune case needs (du_an_ghi_test.go).
func newDiscussionServer(t *testing.T) *discussionServer {
	t.Helper()
	read, write := newProjectDiscussionFake(), &projectDiscussionWritesFake{}
	checker := &checkerDanhMucGia{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	mux := http.NewServeMux()
	Register(mux, Deps{
		Checker: checker, HangMuc: hangMucMau(), GhiHangMuc: &ghiDanhMucGia{}, DuAn: duAnMau(),
		GhiDuAn: &ghiDuAnGia{}, GhiChungTu: &ghiChungTuGia{}, Nguong: nguongMau(),
		NganSach: &nganSachGia{}, GhiNganSach: &ghiNganSachGia{}, AuditLog: &auditLogFake{},
		SystemMessages: &systemMessagesFake{}, Nay: func() time.Time { return lucDaQua7096 }, Log: logger,
		CapitalPlanCategoryImports: &catalogueImportFake{},
		FundingSources:             &fundingSourcesFake{}, FundingSourceWrites: &fundingSourceWritesFake{},
		ProjectDiscussion: read, ProjectDiscussionWrites: write,
	})
	var h http.Handler = mux
	h = chuTheGhi(h)
	h = idem.Middleware(moiKhoIdem(), logger)(h)
	h = httpx.TenantMiddleware(thuMucMau())(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)
	return &discussionServer{h: h, read: read, write: write, checker: checker}
}

func (m *discussionServer) grant(commune tenant.ID, perm ...authz.Perm) {
	if m.checker.co == nil {
		m.checker.co = map[tenant.ID]map[authz.Perm]struct{}{}
	}
	if m.checker.co[commune] == nil {
		m.checker.co[commune] = map[authz.Perm]struct{}{}
	}
	for _, p := range perm {
		m.checker.co[commune][p] = struct{}{}
	}
}

// call ALWAYS SENDS AN Idempotency-Key — the two POSTs declare idem.Required, which would otherwise
// refuse before the handler and turn every assertion into one about the header.
func (m *discussionServer) call(t *testing.T, method, host, path string, p *authz.Principal,
	body string) *httptest.ResponseRecorder {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	r := httptest.NewRequest(method, "https://"+host+path, rd)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set(idem.Header, "01JIDEMPOTENCYKEYDISCUSS0")
	if p != nil {
		r = r.WithContext(context.WithValue(r.Context(), khoaChuTheGhi{}, *p))
	}
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)
	return w
}

type discussionRoute struct {
	name, method, path, body string
	key                      authz.Perm
	wrongKeys                []authz.Perm
	ok                       int
	write                    bool
}

func fiveDiscussionRoutes() []discussionRoute {
	readKeys := []authz.Perm{"budget.update", "budget.confirm"}
	writeKeys := []authz.Perm{"budget.read", "budget.confirm"}
	return []discussionRoute{
		{"đọc vướng mắc", http.MethodGet, "/api/v1/investment-projects/da-001/issues", "",
			"budget.read", readKeys, http.StatusOK, false},
		{"ghi nhận vướng mắc", http.MethodPost, "/api/v1/investment-projects/da-001/issues",
			`{"text":"Chưa bàn giao mặt bằng\nChờ huyện duyệt"}`, "budget.update", writeKeys, http.StatusCreated, true},
		{"gỡ vướng mắc", http.MethodPost, "/api/v1/project-issues/vm-new/resolution", "",
			"budget.update", writeKeys, http.StatusOK, true},
		{"đọc trao đổi", http.MethodGet, "/api/v1/investment-projects/da-001/comments", "",
			"budget.read", readKeys, http.StatusOK, false},
		{"gửi trao đổi", http.MethodPost, "/api/v1/investment-projects/da-001/comments",
			`{"body":"Đề nghị kế toán kiểm lại","mentioned_staff_codes":["CB-00011"]}`, "budget.read", readKeys,
			http.StatusCreated, true},
	}
}

func (m *discussionServer) touched() int { return m.read.reads + m.write.calls }

// --- rule 5, invariant 7 ---------------------------------------------------------------------------

func TestDiscussionRoutes_401NoSession(t *testing.T) {
	for _, rt := range fiveDiscussionRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			m := newDiscussionServer(t)
			m.grant(xaA, rt.key)
			doiMa(t, m.call(t, rt.method, hostA, rt.path, nil, rt.body), http.StatusUnauthorized)
			if m.touched() != 0 {
				t.Fatal("chạm kho khi chưa đăng nhập")
			}
		})
	}
}

func TestDiscussionRoutes_403WrongPermission(t *testing.T) {
	for _, rt := range fiveDiscussionRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			m := newDiscussionServer(t)
			m.grant(xaA, rt.wrongKeys...)
			doiMa(t, m.call(t, rt.method, hostA, rt.path, canBoGhi(xaA), rt.body), http.StatusForbidden)
			if m.touched() != 0 {
				t.Fatal("chạm kho khi sai quyền")
			}
		})
	}
}

// The account is of commune B, signed in at B; the right key was granted in commune A only.
func TestDiscussionRoutes_403RightPermissionWrongCommune(t *testing.T) {
	for _, rt := range fiveDiscussionRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			m := newDiscussionServer(t)
			m.grant(xaA, rt.key)
			doiMa(t, m.call(t, rt.method, hostB, rt.path, canBoGhi(xaB), rt.body), http.StatusForbidden)
			if m.touched() != 0 {
				t.Fatal("quyền cấp ở xã khác mà vẫn chạm kho của xã này")
			}
		})
	}
}

func TestDiscussionRoutes_OKRightPermissionRightCommune(t *testing.T) {
	for _, rt := range fiveDiscussionRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			m := newDiscussionServer(t)
			m.grant(xaA, rt.key)
			doiMa(t, m.call(t, rt.method, hostA, rt.path, canBoGhi(xaA), rt.body), rt.ok)
			if rt.write {
				// The trail's actor is the BUSINESS CODE, and the commune is the Host's (rule 6 inv 8, rule 1).
				if m.write.calls != 1 || m.write.lastTenant != xaA || m.write.lastActor.ID != maCanBoGhi {
					t.Fatalf("use case: %d lần, xã %v, người %q", m.write.calls, m.write.lastTenant, m.write.lastActor.ID)
				}
			}
		})
	}
}

// --- what each route answers -----------------------------------------------------------------------

func TestListIssues_ShapeAndCollidingIDStaysInItsCommune(t *testing.T) {
	m := newDiscussionServer(t)
	m.grant(xaA, "budget.read")
	m.grant(xaB, "budget.read")

	w := m.call(t, http.MethodGet, hostA, "/api/v1/investment-projects/da-001/issues", canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	var out projectIssuesOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	if out.Count != 2 || out.OpenCount != 1 || out.Items[0].ID != "vm-new" || out.Items[0].Resolved ||
		!out.Items[1].Resolved || out.Items[1].ResolvedBy != "CB-00003" || out.Items[0].RecordedBy != "CB-00002" ||
		out.Items[0].RecordedAt != "2026-08-27T02:00:00Z" || out.Items[0].Description != "Chờ huyện" {
		t.Fatalf("out = %+v", out)
	}
	if strings.Contains(w.Body.String(), "tracking_task_id") {
		t.Fatalf("tracking_task_id phải vắng khi chưa có nhiệm vụ: %s", w.Body.String())
	}

	w = m.call(t, http.MethodGet, hostB, "/api/v1/investment-projects/da-001/issues", canBoGhi(xaB), "")
	doiMa(t, w, http.StatusOK)
	if strings.Contains(w.Body.String(), "vm-new") || !strings.Contains(w.Body.String(), "vm-of-commune-b") {
		t.Fatalf("xã B đọc được vướng mắc của xã A: %s", w.Body.String())
	}
}

func TestListDiscussion_UnknownProjectIs404EmptyIsArray(t *testing.T) {
	m := newDiscussionServer(t)
	m.grant(xaA, "budget.read")
	for _, p := range []string{"/issues", "/comments"} {
		doiMa(t, m.call(t, http.MethodGet, hostA, "/api/v1/investment-projects/da-missing"+p, canBoGhi(xaA), ""),
			http.StatusNotFound)
		w := m.call(t, http.MethodGet, hostA, "/api/v1/investment-projects/da-002"+p, canBoGhi(xaA), "")
		doiMa(t, w, http.StatusOK)
		if !strings.Contains(w.Body.String(), `"items":[]`) {
			t.Fatalf("%s rỗng phải là [] chứ không null: %s", p, w.Body.String())
		}
	}
}

func TestListComments_MentionsAreNeverNull(t *testing.T) {
	m := newDiscussionServer(t)
	m.grant(xaA, "budget.read")
	w := m.call(t, http.MethodGet, hostA, "/api/v1/investment-projects/da-001/comments", canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"mentioned_staff_codes":[]`) || !strings.Contains(w.Body.String(), `"author_code":"CB-00001"`) {
		t.Fatalf("body = %s", w.Body.String())
	}
}

func TestRecordIssue_TextReachesUseCaseVerbatimAndReplyIsSplit(t *testing.T) {
	m := newDiscussionServer(t)
	m.grant(xaA, "budget.update")
	w := m.call(t, http.MethodPost, hostA, "/api/v1/investment-projects/da-001/issues", canBoGhi(xaA),
		`{"text":"Chưa bàn giao mặt bằng\nChờ huyện duyệt"}`)
	doiMa(t, w, http.StatusCreated)
	if m.write.lastText != "Chưa bàn giao mặt bằng\nChờ huyện duyệt" || m.write.lastID != "da-001" {
		t.Fatalf("use case nhận %q cho %q", m.write.lastText, m.write.lastID)
	}
	var out projectIssueOut
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out.Title != "Chưa bàn giao mặt bằng" || out.Description != "Chờ huyện duyệt" || out.RecordedBy != maCanBoGhi {
		t.Fatalf("out = %+v", out)
	}
}

func TestAddComment_MentionsReachTheUseCase(t *testing.T) {
	m := newDiscussionServer(t)
	m.grant(xaA, "budget.read")
	w := m.call(t, http.MethodPost, hostA, "/api/v1/investment-projects/da-001/comments", canBoGhi(xaA),
		`{"body":"Đề nghị kế toán kiểm lại","mentioned_staff_codes":["CB-00011","CB-00012"]}`)
	doiMa(t, w, http.StatusCreated)
	if got := m.write.lastComment; got.Body != "Đề nghị kế toán kiểm lại" || len(got.MentionedStaffCodes) != 2 {
		t.Fatalf("use case nhận %+v", got)
	}
	if !strings.Contains(w.Body.String(), `"author_code":"`+maCanBoGhi+`"`) {
		t.Fatalf("body = %s", w.Body.String())
	}
}

func TestDiscussionWrites_RefusalsMapToStatusAndNeverEchoTheText(t *testing.T) {
	cases := []struct {
		name, method, path, body string
		key                      authz.Perm
		err                      error
		status                   int
		code                     string
	}{
		{"dự án không có", http.MethodPost, "/api/v1/investment-projects/da-x/issues", `{"text":"x"}`,
			"budget.update", fistore.ErrKhongThayDuAn, http.StatusNotFound, "not_found"},
		{"vướng mắc không có", http.MethodPost, "/api/v1/project-issues/vm-x/resolution", "",
			"budget.update", fistore.ErrIssueNotFound, http.StatusNotFound, "not_found"},
		{"đã gỡ rồi", http.MethodPost, "/api/v1/project-issues/vm-old/resolution", "",
			"budget.update", domain.ErrIssueAlreadyResolved, http.StatusConflict, "issue_already_resolved"},
		{"dòng đầu quá dài", http.MethodPost, "/api/v1/investment-projects/da-001/issues", `{"text":"SECRET-TEXT-123"}`,
			"budget.update", domain.ErrIssueTitleTooLong, http.StatusBadRequest, "invalid_request"},
		{"nhắc sai", http.MethodPost, "/api/v1/investment-projects/da-001/comments", `{"body":"SECRET-TEXT-123"}`,
			"budget.read", domain.ErrMentionInvalid, http.StatusBadRequest, "invalid_request"},
		{"lỗi hệ thống", http.MethodPost, "/api/v1/investment-projects/da-001/comments", `{"body":"SECRET-TEXT-123"}`,
			"budget.read", errors.New("pq: connection refused SECRET-TEXT-123"), http.StatusInternalServerError, "internal"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := newDiscussionServer(t)
			m.grant(xaA, c.key)
			m.write.err = c.err
			w := m.call(t, c.method, hostA, c.path, canBoGhi(xaA), c.body)
			doiMa(t, w, c.status)
			if e := loiTra(t, w); e.Code != c.code {
				t.Fatalf("code = %q, muốn %q", e.Code, c.code)
			}
			if strings.Contains(w.Body.String(), "SECRET-TEXT-123") || strings.Contains(w.Body.String(), "pq:") {
				t.Fatalf("câu trả lời lặp lại nội dung gửi lên hoặc lỗi nội bộ: %s", w.Body.String())
			}
		})
	}
}

func TestReadDiscussion_StoreFailureIs500WithoutDetail(t *testing.T) {
	for _, storeErr := range []error{fistore.ErrTooManyIssues, errors.New("pq: connection refused")} {
		m := newDiscussionServer(t)
		m.grant(xaA, "budget.read")
		m.read.err = storeErr
		w := m.call(t, http.MethodGet, hostA, "/api/v1/investment-projects/da-001/issues", canBoGhi(xaA), "")
		doiMa(t, w, http.StatusInternalServerError)
		if strings.Contains(w.Body.String(), "pq:") || strings.Contains(w.Body.String(), "vượt trần") {
			t.Fatalf("lộ chi tiết nội bộ: %s", w.Body.String())
		}
	}
}

// --- §7.2 latest issue on the list, §3 open count on the summary --------------------------------

func TestProjectList_CarriesLatestIssueOnlyWhereThereIsOne(t *testing.T) {
	m := newDiscussionServer(t)
	m.grant(xaA, "budget.read")
	w := m.call(t, http.MethodGet, hostA, "/api/v1/investment-projects?year=2026", canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	var out danhSachDuAnRa
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	seen := false
	for _, it := range out.Items {
		switch it.ID {
		case "da-001":
			seen = true
			if it.LatestIssue == nil || it.LatestIssue.Text != "Chưa bàn giao mặt bằng" || !it.LatestIssue.Resolved ||
				it.LatestIssue.RecordedAt != "2026-08-27T02:00:00Z" {
				t.Fatalf("da-001 latest_issue = %+v", it.LatestIssue)
			}
		default:
			if it.LatestIssue != nil {
				t.Fatalf("%s không có vướng mắc mà vẫn có latest_issue", it.ID)
			}
		}
	}
	if !seen {
		t.Fatal("danh sách thiếu da-001")
	}
}

func TestProjectSummary_CarriesTheYearsOpenIssueCount(t *testing.T) {
	m := newDiscussionServer(t)
	m.grant(xaA, "budget.read")
	w := m.call(t, http.MethodGet, hostA, "/api/v1/investment-project-summary?year=2026", canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"open_issue_count":3`) {
		t.Fatalf("body = %s", w.Body.String())
	}
}

// Fail closed: a failed count is a 500, never "0 vướng mắc"; a failed latest-issue read is a 500,
// never a column of "—".
func TestListAndSummary_DiscussionFailureIs500NotZero(t *testing.T) {
	for _, path := range []string{"/api/v1/investment-project-summary?year=2026", "/api/v1/investment-projects?year=2026"} {
		m := newDiscussionServer(t)
		m.grant(xaA, "budget.read")
		m.read.err = errors.New("pq: connection refused")
		w := m.call(t, http.MethodGet, hostA, path, canBoGhi(xaA), "")
		doiMa(t, w, http.StatusInternalServerError)
	}
}

func TestRegisterRefusesMissingDiscussionStores(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Register chấp nhận Deps thiếu kho vướng mắc/trao đổi")
		}
	}()
	Register(http.NewServeMux(), Deps{
		Checker: khongQuyen(), HangMuc: hangMucMau(), GhiHangMuc: &ghiDanhMucGia{}, DuAn: duAnMau(),
		GhiDuAn: &ghiDuAnGia{}, GhiChungTu: &ghiChungTuGia{}, Nguong: nguongMau(),
		NganSach: &nganSachGia{}, GhiNganSach: &ghiNganSachGia{}, AuditLog: &auditLogFake{},
		SystemMessages: &systemMessagesFake{}, CapitalPlanCategoryImports: &catalogueImportFake{},
		FundingSources: &fundingSourcesFake{}, FundingSourceWrites: &fundingSourceWritesFake{},
	})
}
