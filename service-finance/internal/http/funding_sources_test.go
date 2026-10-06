package http

import (
	"context"
	"fmt"
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

// WHAT THIS FILE IS FOR — the four funding source routes (funding_sources.go):
//
//  1. the four cases of rule 5, invariant 7 on EVERY route, plus the key each route actually asks
//     for compared against a literal, plus `budget.read` alone being refused on the two writes;
//  2. the commune and the acting person's BUSINESS CODE reach the write use case (rule 6, inv. 2, 8);
//  3. `year` is required and validated before any store call; a missing `granted_amount` is 400;
//  4. a taken name is 409 with the screen's sentence and never the commune id; an unknown source 404;
//  5. the card shape: ratios NULL on a zero denominator, never clamped; overrun separate; the §13
//     rule 6 warning and the scope notice at the top level; `items` is [] when empty;
//  6. one commune's cards never reach another commune's caller.

// --- fakes -------------------------------------------------------------------------------------

// fundingSourcesFake is the read half, KEYED BY COMMUNE read from the context exactly as the store
// binds it — keyed any other way the isolation case would pass while proving nothing.
type fundingSourcesFake struct {
	cards        map[tenant.ID]map[int][]domain.TienDoNguonVon
	unattributed map[tenant.ID]domain.Dong
	projects     map[tenant.ID]map[string]domain.FundingSourceProjects
	err          error

	calls    int
	lastYear int
}

// vi-name-ok: implements FundingSourceReading, whose method mirrors the existing NguonVonStore.TienDoTheoNguon
func (f *fundingSourcesFake) TienDoTheoNguon(ctx context.Context, year int) ([]domain.TienDoNguonVon, error) {
	f.calls++
	f.lastYear = year
	if f.err != nil {
		return nil, f.err
	}
	return f.cards[tenant.MustFrom(ctx)][year], nil
}

func (f *fundingSourcesFake) UnattributedDisbursed(ctx context.Context, year int) (domain.Dong, error) {
	return f.unattributed[tenant.MustFrom(ctx)], nil
}

func (f *fundingSourcesFake) SourceProjects(ctx context.Context, id string, year int) (domain.FundingSourceProjects, error) {
	f.calls++
	f.lastYear = year
	if f.err != nil {
		return domain.FundingSourceProjects{}, f.err
	}
	p, ok := f.projects[tenant.MustFrom(ctx)][id]
	if !ok {
		return domain.FundingSourceProjects{}, fistore.ErrFundingSourceNotFound
	}
	return p, nil
}

// fundingSourceWritesFake is the write half, RECORDING THE COMMUNE AND THE ACTOR it was called with.
type fundingSourceWritesFake struct {
	added  domain.NguonVon
	amount domain.FundingSourceAnnualAmount
	err    error

	addCalls, setCalls int
	commune            tenant.ID
	actor              audit.Actor
	lastAdd            app.FundingSourceCreateRequest
	lastSourceID       string
	lastYear           int
	lastAmount         domain.Dong
}

func (f *fundingSourceWritesFake) AddFundingSource(ctx context.Context, req app.FundingSourceCreateRequest,
	actor audit.Actor) (domain.NguonVon, error) {
	f.addCalls++
	f.commune, f.actor, f.lastAdd = tenant.MustFrom(ctx), actor, req
	if f.err != nil {
		return domain.NguonVon{}, f.err
	}
	return f.added, nil
}

func (f *fundingSourceWritesFake) SetGrantedAmount(ctx context.Context, id string, year int, amount domain.Dong,
	actor audit.Actor) (domain.FundingSourceAnnualAmount, error) {
	f.setCalls++
	f.commune, f.actor = tenant.MustFrom(ctx), actor
	f.lastSourceID, f.lastYear, f.lastAmount = id, year, amount
	if f.err != nil {
		return domain.FundingSourceAnnualAmount{}, f.err
	}
	return f.amount, nil
}

// --- harness -----------------------------------------------------------------------------------

type fundingSourceHarness struct {
	h       http.Handler
	read    *fundingSourcesFake
	write   *fundingSourceWritesFake
	checker *checkerDanhMucGia
}

func newFundingSourceHarness(t *testing.T) *fundingSourceHarness {
	t.Helper()
	read := &fundingSourcesFake{}
	write := &fundingSourceWritesFake{
		added:  domain.NguonVon{ID: "nv-moi", Ten: "Nguồn xã hội hoá", ThuTu: 5, Nam: 2026, TongNguon: 1_100_000_000},
		amount: domain.FundingSourceAnnualAmount{ID: "fa-1", FundingSourceID: "nv-xa", Year: 2026, GrantedAmount: 9_200_000_000},
	}
	checker := &checkerDanhMucGia{}
	lg := slog.New(slog.NewTextHandler(io.Discard, nil))

	mux := http.NewServeMux()
	Register(mux, Deps{
		Checker:                    checker,
		HangMuc:                    hangMucMau(),
		GhiHangMuc:                 &ghiDanhMucGia{},
		DuAn:                       duAnMau(),
		GhiDuAn:                    &ghiDuAnGia{},
		GhiChungTu:                 &ghiChungTuGia{},
		Nguong:                     nguongMacDinh(),
		NganSach:                   &nganSachGia{},
		GhiNganSach:                &ghiNganSachGia{},
		AuditLog:                   &auditLogFake{},
		SystemMessages:             &systemMessagesFake{},
		CapitalPlanCategoryImports: &catalogueImportFake{}, DisbursementImports: &disbursementImportFake{},
		FundingSources:          read,
		FundingSourceWrites:     write,
		ProjectDiscussion:       newProjectDiscussionFake(),
		ProjectDiscussionWrites: &projectDiscussionWritesFake{},
		Nay:                     func() time.Time { return lucDaQua7096 },
		Log:                     lg,
	})

	var h http.Handler = mux
	h = chuTheGhi(h)
	h = idem.Middleware(nil, lg)(h)
	h = httpx.TenantMiddleware(thuMucMau())(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)
	return &fundingSourceHarness{h: h, read: read, write: write, checker: checker}
}

func (m *fundingSourceHarness) grant(xa tenant.ID, perm ...authz.Perm) {
	if m.checker.co == nil {
		m.checker.co = map[tenant.ID]map[authz.Perm]struct{}{}
	}
	if m.checker.co[xa] == nil {
		m.checker.co[xa] = map[authz.Perm]struct{}{}
	}
	for _, p := range perm {
		m.checker.co[xa][p] = struct{}{}
	}
}

// call ALWAYS SENDS AN Idempotency-Key: POST declares idem.Required, which refuses without it before
// the handler runs.
func (m *fundingSourceHarness) call(t *testing.T, method, host, path string, p *authz.Principal,
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
	r.Header.Set(idem.Header, "01JIDEMPOTENCYKEYNGUONVON")
	if p != nil {
		r = r.WithContext(context.WithValue(r.Context(), khoaChuTheGhi{}, *p))
	}
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)
	return w
}

func (m *fundingSourceHarness) ran() int { return m.read.calls + m.write.addCalls + m.write.setCalls }

const pathFundingSources = "/api/v1/funding-sources"

type fundingSourceRoute struct {
	name, method, path, body string
	key                      authz.Perm
	ok                       int
	write                    bool
}

func fundingSourceRoutes() []fundingSourceRoute {
	return []fundingSourceRoute{
		{"GET list", http.MethodGet, pathFundingSources + "?year=2026", "", "budget.read", http.StatusOK, false},
		{"POST", http.MethodPost, pathFundingSources, `{"name":"Nguồn xã hội hoá","year":2026,"granted_amount":1100000000}`,
			"budget.update", http.StatusCreated, true},
		{"PUT amount", http.MethodPut, pathFundingSources + "/nv-xa/annual-amounts/2026", `{"granted_amount":9200000000}`,
			"budget.update", http.StatusOK, true},
		{"GET projects", http.MethodGet, pathFundingSources + "/nv-xa/projects?year=2026", "", "budget.read", http.StatusOK, false},
	}
}

// seedA gives commune A one source so the 200 case of the projects route has something to find.
func (m *fundingSourceHarness) seedA() {
	m.read.projects = map[tenant.ID]map[string]domain.FundingSourceProjects{
		xaA: {"nv-xa": {Source: domain.NguonVon{ID: "nv-xa", Ten: "Ngân sách xã, phường", Nam: 2026}}},
	}
}

// --- (1) the permission cases ------------------------------------------------------------------

func TestFundingSourceRoutesAskForTheDecidedKey(t *testing.T) {
	// The fake checker grants whatever it is handed, so only a comparison against a LITERAL catches a
	// route whose key drifted to one the `quyen` table lacks (rule 5, invariant 3c).
	for _, tc := range fundingSourceRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			m := newFundingSourceHarness(t)
			m.call(t, tc.method, hostA, tc.path, canBoGhi(xaA), tc.body)
			if got := m.checker.hoiKhoaCuoi(); got != tc.key {
				t.Fatalf("tuyến hỏi khoá %q, muốn %q", got, tc.key)
			}
		})
	}
}

func TestFundingSource_401WithoutSession(t *testing.T) {
	for _, tc := range fundingSourceRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			m := newFundingSourceHarness(t)
			m.grant(xaA, "budget.read", "budget.update")
			doiMa(t, m.call(t, tc.method, hostA, tc.path, nil, tc.body), http.StatusUnauthorized)
			if m.ran() != 0 {
				t.Error("chưa đăng nhập mà kho/use case đã chạy")
			}
		})
	}
}

func TestFundingSource_403WrongPermission(t *testing.T) {
	// A REAL key of this service, for somebody whose job is the catalogue screen.
	for _, tc := range fundingSourceRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			m := newFundingSourceHarness(t)
			m.grant(xaA, "admin.lookup")
			doiMa(t, m.call(t, tc.method, hostA, tc.path, canBoGhi(xaA), tc.body), http.StatusForbidden)
			if m.ran() != 0 {
				t.Error("sai quyền mà kho/use case vẫn chạy")
			}
		})
	}
}

func TestFundingSource_403ReadOnlyCannotWrite(t *testing.T) {
	m := newFundingSourceHarness(t)
	m.grant(xaA, "budget.read")
	for _, tc := range fundingSourceRoutes() {
		if !tc.write {
			continue
		}
		doiMa(t, m.call(t, tc.method, hostA, tc.path, canBoGhi(xaA), tc.body), http.StatusForbidden)
	}
	if m.write.addCalls+m.write.setCalls != 0 {
		t.Error("tài khoản chỉ có `budget.read` mà ghi được nguồn vốn")
	}
}

func TestFundingSource_403RightPermissionWrongCommune(t *testing.T) {
	// Signed in AT commune B as a member of commune B; the grant exists only in commune A.
	for _, tc := range fundingSourceRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			m := newFundingSourceHarness(t)
			m.grant(xaA, "budget.read", "budget.update")
			doiMa(t, m.call(t, tc.method, hostB, tc.path, canBoGhi(xaB), tc.body), http.StatusForbidden)
			if m.ran() != 0 {
				t.Error("quyền cấp ở xã khác mà vẫn đọc/ghi được vào xã này")
			}
		})
	}
}

func TestFundingSource_401SessionOfAnotherCommune(t *testing.T) {
	for _, tc := range fundingSourceRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			m := newFundingSourceHarness(t)
			m.grant(xaA, "budget.read", "budget.update")
			m.grant(xaB, "budget.read", "budget.update")
			doiMa(t, m.call(t, tc.method, hostB, tc.path, canBoGhi(xaA), tc.body), http.StatusUnauthorized)
			if m.ran() != 0 {
				t.Error("phiên của xã khác mà vẫn đọc/ghi được")
			}
		})
	}
}

func TestFundingSource_200RightPermissionRightCommune(t *testing.T) {
	for _, tc := range fundingSourceRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			m := newFundingSourceHarness(t)
			m.seedA()
			m.grant(xaA, "budget.read", "budget.update")
			doiMa(t, m.call(t, tc.method, hostA, tc.path, canBoGhi(xaA), tc.body), tc.ok)
			if m.ran() != 1 {
				t.Fatalf("tầng dưới chạy %d lần, muốn 1", m.ran())
			}
			if !tc.write {
				return
			}
			// WHO and IN WHICH COMMUNE reach the layer that writes the entry — the business code,
			// and the check names the wrong value outright.
			if m.write.commune != xaA {
				t.Errorf("use case chạy trong xã %q, muốn %q", m.write.commune, xaA)
			}
			if m.write.actor.ID != maCanBoGhi || m.write.actor.ID == idCanBoGhi {
				t.Errorf("chủ thể vết = %q, muốn mã cán bộ %q (KHÔNG phải id nội bộ)", m.write.actor.ID, maCanBoGhi)
			}
		})
	}
}

// --- (3) required inputs -----------------------------------------------------------------------

func TestFundingSourceYearIsRequiredAndValidatedBeforeTheStore(t *testing.T) {
	for _, path := range []string{
		pathFundingSources, pathFundingSources + "?year=", pathFundingSources + "?year=abc",
		pathFundingSources + "?year=1999", pathFundingSources + "?year=2101",
		pathFundingSources + "/nv-xa/projects", pathFundingSources + "/nv-xa/projects?year=20226",
	} {
		m := newFundingSourceHarness(t)
		m.grant(xaA, "budget.read")
		doiMa(t, m.call(t, http.MethodGet, hostA, path, canBoGhi(xaA), ""), http.StatusBadRequest)
		if m.read.calls != 0 {
			t.Fatalf("%s: kho chạy dù thiếu/sai năm — năm không bao giờ mặc định", path)
		}
	}
	for _, year := range []string{"abc", "1999", "0"} {
		m := newFundingSourceHarness(t)
		m.grant(xaA, "budget.update")
		doiMa(t, m.call(t, http.MethodPut, hostA, pathFundingSources+"/nv-xa/annual-amounts/"+year,
			canBoGhi(xaA), `{"granted_amount":1}`), http.StatusBadRequest)
		if m.write.setCalls != 0 {
			t.Fatalf("năm %q trên đường dẫn: use case vẫn chạy", year)
		}
	}
}

func TestPutGrantedAmountRequiresTheField(t *testing.T) {
	// Absent is NOT 0: "forgot the field" and "granted nothing" are different statements.
	for _, body := range []string{`{}`, `{"granted_amount":null}`} {
		m := newFundingSourceHarness(t)
		m.grant(xaA, "budget.update")
		doiMa(t, m.call(t, http.MethodPut, hostA, pathFundingSources+"/nv-xa/annual-amounts/2026",
			canBoGhi(xaA), body), http.StatusBadRequest)
		if m.write.setCalls != 0 {
			t.Fatalf("%s: use case chạy dù thiếu granted_amount", body)
		}
	}
	m := newFundingSourceHarness(t)
	m.grant(xaA, "budget.update")
	doiMa(t, m.call(t, http.MethodPut, hostA, pathFundingSources+"/nv-xa/annual-amounts/2027",
		canBoGhi(xaA), `{"granted_amount":0}`), http.StatusOK)
	if m.write.lastSourceID != "nv-xa" || m.write.lastYear != 2027 || m.write.lastAmount != 0 {
		t.Fatalf("use case nhận %q/%d/%d", m.write.lastSourceID, m.write.lastYear, m.write.lastAmount)
	}
}

func TestPostGrantedAmountAbsentIsBlankAndZeroIsExplicit(t *testing.T) {
	m := newFundingSourceHarness(t)
	m.grant(xaA, "budget.update")
	doiMa(t, m.call(t, http.MethodPost, hostA, pathFundingSources, canBoGhi(xaA),
		`{"name":"  Nguồn xã hội hoá ","year":2026}`), http.StatusCreated)
	if m.write.lastAdd.GrantedAmount != nil || m.write.lastAdd.Year != 2026 {
		t.Fatalf("để trống mà use case nhận %+v", m.write.lastAdd)
	}
	m2 := newFundingSourceHarness(t)
	m2.grant(xaA, "budget.update")
	doiMa(t, m2.call(t, http.MethodPost, hostA, pathFundingSources, canBoGhi(xaA),
		`{"name":"Nguồn xã hội hoá","year":2026,"granted_amount":0}`), http.StatusCreated)
	if g := m2.write.lastAdd.GrantedAmount; g == nil || *g != 0 {
		t.Fatalf("số 0 khai rõ phải tới use case là con trỏ tới 0, nhận %v", g)
	}
}

// --- (4) refusals ------------------------------------------------------------------------------

func TestPostDuplicateNameIs409WithTheScreensSentence(t *testing.T) {
	m := newFundingSourceHarness(t)
	m.grant(xaA, "budget.update")
	// Wrapped the way app.wrapFundingSource wraps it: the commune id must NOT reach the client.
	m.write.err = fmt.Errorf("nguon_von: thêm cho xã %s: %w", xaA, fistore.ErrFundingSourceNameTaken)

	w := m.call(t, http.MethodPost, hostA, pathFundingSources, canBoGhi(xaA), `{"name":"Ngân sách xã, phường","year":2026}`)
	doiMa(t, w, http.StatusConflict)
	e := loiTra(t, w)
	if e.Code != "funding_source_name_taken" || !strings.Contains(e.Message, "đã có nguồn vốn mang tên này") {
		t.Fatalf("409 = %q %q", e.Code, e.Message)
	}
	if strings.Contains(e.Message, string(xaA)) {
		t.Fatalf("câu trả cho client mang mã xã: %q", e.Message)
	}
}

func TestFundingSourceRefusalsMapToTheirStatus(t *testing.T) {
	for name, c := range map[string]struct {
		method, path, body string
		key                authz.Perm
		err                error
		status             int
		code               string
	}{
		"đủ trần": {http.MethodPost, pathFundingSources, `{"name":"A","year":2026}`, "budget.update",
			fistore.ErrFundingSourceCatalogueFull, http.StatusConflict, "funding_source_catalogue_full"},
		"tên rỗng": {http.MethodPost, pathFundingSources, `{"name":"","year":2026}`, "budget.update",
			domain.ErrFundingSourceNameMissing, http.StatusBadRequest, "invalid_request"},
		"POST thiếu năm": {http.MethodPost, pathFundingSources, `{"name":"A"}`, "budget.update",
			domain.ErrFundingSourceYearMissing, http.StatusBadRequest, "invalid_request"},
		"vốn âm": {http.MethodPut, pathFundingSources + "/nv-xa/annual-amounts/2026", `{"granted_amount":-1}`, "budget.update",
			domain.ErrGrantedAmountNegative, http.StatusBadRequest, "invalid_request"},
		"nguồn không có (hoặc của xã khác)": {http.MethodPut, pathFundingSources + "/nv-b/annual-amounts/2026",
			`{"granted_amount":1}`, "budget.update",
			fmt.Errorf("nguon_von: ghi cho xã %s: %w", xaA, fistore.ErrFundingSourceNotFound), http.StatusNotFound, "not_found"},
		"hỏng hệ thống": {http.MethodPut, pathFundingSources + "/nv-xa/annual-amounts/2026", `{"granted_amount":1}`, "budget.update",
			fmt.Errorf("nguon_von: ghi cho xã %s: %w", xaA, fmt.Errorf("pq: connection refused")), http.StatusInternalServerError, "internal"},
	} {
		t.Run(name, func(t *testing.T) {
			m := newFundingSourceHarness(t)
			m.grant(xaA, c.key)
			m.write.err = c.err
			w := m.call(t, c.method, hostA, c.path, canBoGhi(xaA), c.body)
			doiMa(t, w, c.status)
			e := loiTra(t, w)
			if e.Code != c.code || strings.Contains(e.Message, string(xaA)) || strings.Contains(e.Message, "pq:") {
				t.Fatalf("= %q %q", e.Code, e.Message)
			}
		})
	}
}

func TestProjectsOfUnknownSourceIs404(t *testing.T) {
	m := newFundingSourceHarness(t)
	m.seedA()
	m.grant(xaA, "budget.read")
	doiMa(t, m.call(t, http.MethodGet, hostA, pathFundingSources+"/nv-cua-xa-khac/projects?year=2026",
		canBoGhi(xaA), ""), http.StatusNotFound)
}

// --- (5) shapes --------------------------------------------------------------------------------

func TestListCardShape(t *testing.T) {
	m := newFundingSourceHarness(t)
	m.grant(xaA, "budget.read")
	m.read.cards = map[tenant.ID]map[int][]domain.TienDoNguonVon{xaA: {2026: {
		// §6's first card.
		{NguonVon: domain.NguonVon{ID: "nv-xa", Ten: "Ngân sách xã, phường", ThuTu: 1, Nam: 2026, TongNguon: 9_200_000_000},
			DaPhanBo: 70_000_000, DaGiaiNgan: 13_200_000, SoDuAn: 2},
		// Nothing granted, nothing allocated: every bar is NULL, nothing is an overrun.
		{NguonVon: domain.NguonVon{ID: "nv-mtqg", Ten: "Chương trình mục tiêu quốc gia", ThuTu: 2, Nam: 2026}},
		// Over-committed and over-disbursed: shown, not clamped.
		{NguonVon: domain.NguonVon{ID: "nv-xhh", Ten: "Nguồn xã hội hoá", ThuTu: 3, Nam: 2026, TongNguon: 1_000},
			DaPhanBo: 5_800, DaGiaiNgan: 10_237, SoDuAn: 1},
	}}}
	m.read.unattributed = map[tenant.ID]domain.Dong{xaA: 3_400_000_000}

	var out fundingSourcesOut
	docJSON(t, m.call(t, http.MethodGet, hostA, pathFundingSources+"?year=2026", canBoGhi(xaA), ""), &out)
	if out.Year != 2026 || len(out.Items) != 3 || out.UnattributedDisbursedAmount != 3_400_000_000 || out.ScopeNotice == "" {
		t.Fatalf("đầu trang = năm %d, %d thẻ, cảnh báo %d, scope %q", out.Year, len(out.Items),
			out.UnattributedDisbursedAmount, out.ScopeNotice)
	}
	a := out.Items[0]
	if a.UnallocatedAmount != 9_130_000_000 || a.OverallocatedAmount != 0 ||
		*a.AllocatedRatio != 76 || *a.DisbursedOfAllocatedRatio != 1885 || *a.DisbursedOfGrantedRatio != 14 {
		t.Fatalf("thẻ §6 = %+v", a)
	}
	b := out.Items[1]
	if b.AllocatedRatio != nil || b.DisbursedOfAllocatedRatio != nil || b.DisbursedOfGrantedRatio != nil ||
		b.UnallocatedAmount != 0 || b.OverallocatedAmount != 0 {
		t.Fatalf("thẻ chưa có số = %+v — tỷ lệ phải null, không phải 0%%", b)
	}
	c := out.Items[2]
	if c.OverallocatedAmount != 4_800 || c.UnallocatedAmount != 0 || *c.AllocatedRatio != 58000 ||
		*c.DisbursedOfGrantedRatio != 102370 {
		t.Fatalf("thẻ vượt = %+v — vượt phải hiện, không cắt", c)
	}
}

func TestListIsTheCommunesOwnAndEmptyIsAnArray(t *testing.T) {
	m := newFundingSourceHarness(t)
	m.grant(xaA, "budget.read")
	m.grant(xaB, "budget.read")
	m.read.cards = map[tenant.ID]map[int][]domain.TienDoNguonVon{
		xaA: {2026: {{NguonVon: domain.NguonVon{ID: "nv-a", Ten: "Nguồn của xã A", Nam: 2026}}}},
	}
	w := m.call(t, http.MethodGet, hostB, pathFundingSources+"?year=2026", canBoGhi(xaB), "")
	doiMa(t, w, http.StatusOK)
	if strings.Contains(w.Body.String(), "Nguồn của xã A") || !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Fatalf("xã B nhận %s", w.Body.String())
	}
}

func TestCreateReplyIsACardForTheYear(t *testing.T) {
	m := newFundingSourceHarness(t)
	m.grant(xaA, "budget.update")
	var out fundingSourceOut
	w := m.call(t, http.MethodPost, hostA, pathFundingSources, canBoGhi(xaA),
		`{"name":"Nguồn xã hội hoá","year":2026,"granted_amount":1100000000}`)
	doiMa(t, w, http.StatusCreated)
	decodeAny(t, w, &out)
	if out.ID != "nv-moi" || out.Year != 2026 || out.GrantedAmount != 1_100_000_000 || out.Order != 5 ||
		out.UnallocatedAmount != 1_100_000_000 || out.AllocatedRatio == nil || *out.AllocatedRatio != 0 ||
		out.DisbursedOfAllocatedRatio != nil {
		t.Fatalf("201 = %+v", out)
	}
	if g := m.write.lastAdd.GrantedAmount; g == nil || *g != 1_100_000_000 || m.write.lastAdd.Name != "Nguồn xã hội hoá" {
		t.Fatalf("use case nhận %+v", m.write.lastAdd)
	}
}

func TestProjectsBreakdownShape(t *testing.T) {
	m := newFundingSourceHarness(t)
	m.grant(xaA, "budget.read")
	m.read.projects = map[tenant.ID]map[string]domain.FundingSourceProjects{xaA: {"nv-xa": {
		Source: domain.NguonVon{ID: "nv-xa", Ten: "Ngân sách xã, phường", Nam: 2026},
		Projects: []domain.FundingSourceProject{
			{ProjectID: "da-1", Code: "DA01", Name: "Bê tông hoá đường", PlannedAmount: 60_000_000,
				AllocatedAmount: 30_000_000, DisbursedAmount: 13_200_000},
			{ProjectID: "da-2", Code: "DA02", Name: "Nhà văn hoá", AllocatedAmount: 0, DisbursedAmount: 0},
		},
		DisbursedWithoutAllocation: 2_000_000,
	}}}

	var out fundingSourceProjectsOut
	docJSON(t, m.call(t, http.MethodGet, hostA, pathFundingSources+"/nv-xa/projects?year=2026", canBoGhi(xaA), ""), &out)
	if out.FundingSourceID != "nv-xa" || out.Year != 2026 || len(out.Items) != 2 ||
		out.DisbursedWithoutAllocationAmount != 2_000_000 || m.read.lastYear != 2026 {
		t.Fatalf("= %+v", out)
	}
	if out.Items[0].DisbursedRatio == nil || *out.Items[0].DisbursedRatio != 4400 || out.Items[0].PlannedAmount != 60_000_000 {
		t.Fatalf("dòng 1 = %+v", out.Items[0])
	}
	if out.Items[1].DisbursedRatio != nil {
		t.Fatalf("dòng phân bổ 0 đồng mà có tỷ lệ: %+v", out.Items[1])
	}
}
