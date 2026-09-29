package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
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

// WHAT THIS FILE IS FOR: the EIGHT routes of the budget board — the screen whose figures go into a
// document the commune sends to a higher authority.
//
// SIX THINGS, each of which fails silently if it stops holding:
//
//  1. the four cases of rule 5, invariant 7, on EVERY one of the eight — and the third case is the
//     one that is easy to fake, see TestBudget_403RightPermissionWrongCommune;
//  2. each route asks for the key the SPECIFICATION's group assigns (§9 rule 7) and that key exists
//     in the `quyen` table. A key no migration seeds is a route that answers 403 to every account
//     forever while every test stays green (rule 5, invariant 3c);
//  3. WITH NO ROW STARRED THERE IS NO TOTAL, and a SENTENCE says so — not 0 (ADR 0035 §A);
//  4. `Dự toán TP giao` left blank makes the indicator DISAPPEAR with a sentence — not 0, not NaN;
//  5. `method`, `level` and `is_headline` are REFUSED on the write bodies, not ignored;
//  6. the commune and the acting person reach the use case, because they are what the audit entry is
//     filed under (rule 6, invariant 2).

// --- fakes ---------------------------------------------------------------------------------------

// fakeBudgetReader is the read side, KEYED BY COMMUNE and by (year, kind), reading the commune from the
// context exactly as *store.Scoped reads it. A fake that ignored the commune would let the
// wrong-commune case pass while proving nothing.
type fakeBudgetReader struct {
	byTenant map[tenant.ID]map[string]domain.FullSheet
	// entries is the `⇄` list, keyed by commune and line id — same discipline as `byTenant`.
	entries map[tenant.ID]map[string]domain.LineEntries
	err     error
	calls   int
}

func (n *fakeBudgetReader) LineEntries(ctx context.Context, lineID string) (domain.LineEntries, error) {
	n.calls++
	if n.err != nil {
		return domain.LineEntries{}, n.err
	}
	d, ok := n.entries[tenant.MustFrom(ctx)][lineID]
	if !ok {
		return domain.LineEntries{}, domain.ErrLineNotFound
	}
	return d, nil
}

func sheetKey(year int, kind domain.SheetKind) string {
	return string(kind) + ":" + strconv.Itoa(year)
}

func (n *fakeBudgetReader) FullSheet(ctx context.Context, year int,
	kind domain.SheetKind) (domain.FullSheet, error) {

	n.calls++
	if n.err != nil {
		return domain.FullSheet{}, n.err
	}
	d, ok := n.byTenant[tenant.MustFrom(ctx)][sheetKey(year, kind)]
	if !ok {
		return domain.FullSheet{}, fistore.ErrBudgetSheetNotFound
	}
	return d, nil
}

// fakeBudgetWriter stands in for the write use case, RECORDING THE COMMUNE IT WAS CALLED IN.
type fakeBudgetWriter struct {
	result      domain.BudgetLine
	resultSheet domain.BudgetSheet
	err         error

	createSheetCalls, removeSheetCalls, updateSheetCalls int
	createCalls, updateCalls, removeCalls, headlineCalls int
	recordEntryCalls, removeEntryCalls                   int
	resultEntry                                          domain.BudgetEntry
	lastRecordEntry                                      app.RecordBudgetEntryRequest

	lastTenant      tenant.ID
	lastActor       audit.Actor
	lastCreateSheet app.CreateBudgetSheetRequest
	lastCreate      app.CreateBudgetLineRequest
	lastUpdate      app.UpdateBudgetLineRequest
	lastUpdateSheet app.UpdateBudgetSheetRequest
	lastID          string
	lastReason      string
}

func (g *fakeBudgetWriter) record(ctx context.Context, actor audit.Actor) {
	g.lastTenant = tenant.MustFrom(ctx)
	g.lastActor = actor
}

func (g *fakeBudgetWriter) CreateSheet(ctx context.Context, req app.CreateBudgetSheetRequest,
	actor audit.Actor) (domain.BudgetSheet, error) {
	g.createSheetCalls++
	g.lastCreateSheet = req
	g.record(ctx, actor)
	if g.err != nil {
		return domain.BudgetSheet{}, g.err
	}
	return g.resultSheet, nil
}

func (g *fakeBudgetWriter) RemoveSheet(ctx context.Context, id, reason string, actor audit.Actor) error {
	g.removeSheetCalls++
	g.lastID, g.lastReason = id, reason
	g.record(ctx, actor)
	return g.err
}

func (g *fakeBudgetWriter) UpdateSheet(ctx context.Context, id string, req app.UpdateBudgetSheetRequest,
	actor audit.Actor) (domain.BudgetSheet, error) {
	g.updateSheetCalls++
	g.lastID, g.lastUpdateSheet = id, req
	g.record(ctx, actor)
	if g.err != nil {
		return domain.BudgetSheet{}, g.err
	}
	return g.resultSheet, nil
}

func (g *fakeBudgetWriter) CreateLine(ctx context.Context, req app.CreateBudgetLineRequest,
	actor audit.Actor) (domain.BudgetLine, error) {
	g.createCalls++
	g.lastCreate = req
	g.record(ctx, actor)
	if g.err != nil {
		return domain.BudgetLine{}, g.err
	}
	return g.result, nil
}

func (g *fakeBudgetWriter) UpdateLine(ctx context.Context, id string, req app.UpdateBudgetLineRequest,
	actor audit.Actor) (domain.BudgetLine, error) {
	g.updateCalls++
	g.lastID, g.lastUpdate = id, req
	g.record(ctx, actor)
	if g.err != nil {
		return domain.BudgetLine{}, g.err
	}
	return g.result, nil
}

func (g *fakeBudgetWriter) RemoveLine(ctx context.Context, id, reason string, actor audit.Actor) error {
	g.removeCalls++
	g.lastID, g.lastReason = id, reason
	g.record(ctx, actor)
	return g.err
}

func (g *fakeBudgetWriter) SetHeadline(ctx context.Context, id string,
	actor audit.Actor) (domain.BudgetLine, error) {
	g.headlineCalls++
	g.lastID = id
	g.record(ctx, actor)
	if g.err != nil {
		return domain.BudgetLine{}, g.err
	}
	return g.result, nil
}

func (g *fakeBudgetWriter) RecordEntry(ctx context.Context, req app.RecordBudgetEntryRequest,
	actor audit.Actor) (domain.BudgetEntry, error) {
	g.recordEntryCalls++
	g.lastRecordEntry = req
	g.record(ctx, actor)
	if g.err != nil {
		return domain.BudgetEntry{}, g.err
	}
	return g.resultEntry, nil
}

func (g *fakeBudgetWriter) RemoveEntry(ctx context.Context, id, reason string, actor audit.Actor) error {
	g.removeEntryCalls++
	g.lastID, g.lastReason = id, reason
	g.record(ctx, actor)
	return g.err
}

func (g *fakeBudgetWriter) totalCalls() int {
	return g.createSheetCalls + g.removeSheetCalls + g.updateSheetCalls + g.createCalls + g.updateCalls + g.removeCalls + g.headlineCalls +
		g.recordEntryCalls + g.removeEntryCalls
}

// --- fixtures ------------------------------------------------------------------------------------

// The same figures §3 prints, in đồng. See internal/domain/budget_test.go for why the
// specification's own numbers are used rather than invented ones.
func millionDong(tenths int64) domain.Dong { return domain.Dong(tenths * 100_000) }

const (
	expenditureSheetID    = "01JBANGCHI0000000000000000"
	revenueSheetID        = "01JBANGTHU0000000000000000"
	expenditureHeadlineID = "01JKHOANMUCTONGCHI00000000"
	revenueHeadlineID     = "01JKHOANMUCTONGTHU00000000"
)

// tenantAExpenditureSheet is the chi sheet: `Tổng số` MARKED, and `A. CHI NGÂN SÁCH NHÀ NƯỚC` beside it as a
// SIBLING carrying LARGER figures of its own. The sibling is the fixture's point — anything that
// summed or picked among top-level rows would produce a bigger number than the sheet's own total.
func tenantAExpenditureSheet() domain.FullSheet {
	return domain.FullSheet{
		Sheet: domain.BudgetSheet{
			ID: expenditureSheetID, Code: "NS-2026-CHI-01", Year: 2026, Kind: domain.SheetKindExpenditure, Revision: 1,
			Title:        "BÁO CÁO CHI NGÂN SÁCH NHÀ NƯỚC XÃ THĂNG BÌNH NĂM 2026",
			Unit:         "Triệu đồng",
			CumulativeTo: time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC),
		},
		Columns: []domain.BudgetColumn{
			{ID: "c-dt", SheetID: expenditureSheetID, Name: "Dự toán năm", SortOrder: 1,
				Format: domain.ColumnFormatNumber, Indicator: domain.IndicatorAnnualEstimate},
			{ID: "c-chi", SheetID: expenditureSheetID, Name: "Chi ngân sách", SortOrder: 2,
				Format: domain.ColumnFormatNumber, Indicator: domain.IndicatorBudgetExpenditure},
			{ID: "c-ty", SheetID: expenditureSheetID, Name: "So sánh TH/DT (%)", SortOrder: 3,
				Format: domain.ColumnFormatPercent, Formula: "col_2 / col_1 * 100"},
		},
		Lines: []domain.BudgetLine{
			{ID: expenditureHeadlineID, SheetID: expenditureSheetID, Name: "Tổng số", SortOrder: 1,
				Method: domain.MethodManual, IsHeadline: true},
			{ID: "k-a", SheetID: expenditureSheetID, OrdinalLabel: "A", Name: "CHI NGÂN SÁCH NHÀ NƯỚC", SortOrder: 2,
				Method: domain.MethodManual},
		},
		Values: map[string]map[string]domain.Dong{
			expenditureHeadlineID: {"c-dt": millionDong(37_947_400), "c-chi": millionDong(34_634_592)},
			"k-a":                 {"c-dt": millionDong(55_026_600), "c-chi": millionDong(34_016_733)},
		},
	}
}

// tenantARevenueSheet is the thu sheet with all four columns of §3.2 — the two revenue ones differ by over
// a million units, which is what ADR 0035 #32 turns on.
func tenantARevenueSheet() domain.FullSheet {
	return domain.FullSheet{
		Sheet: domain.BudgetSheet{
			ID: revenueSheetID, Code: "NS-2026-THU-01", Year: 2026, Kind: domain.SheetKindRevenue, Revision: 1,
			Title: "THU NGÂN SÁCH XÃ THĂNG BÌNH NĂM 2026", Unit: "Triệu đồng",
		},
		Columns: []domain.BudgetColumn{
			{ID: "c-tp", SheetID: revenueSheetID, Name: "Dự toán 2026 TP giao", SortOrder: 1,
				Format: domain.ColumnFormatNumber, Indicator: domain.IndicatorEstimateAssignedByProvince},
			{ID: "c-xa", SheetID: revenueSheetID, Name: "Dự toán 2026 Xã giao", SortOrder: 2,
				Format: domain.ColumnFormatNumber, Indicator: domain.IndicatorEstimateAssignedByCommune},
			{ID: "c-nsnn", SheetID: revenueSheetID, Name: "Thu ngân sách NSNN", SortOrder: 3,
				Format: domain.ColumnFormatNumber, Indicator: domain.IndicatorStateBudgetRevenue},
			{ID: "c-huong", SheetID: revenueSheetID, Name: "Thu ngân sách Thu xã hưởng", SortOrder: 4,
				Format: domain.ColumnFormatNumber, Indicator: domain.IndicatorCommuneRetainedRevenue},
		},
		Lines: []domain.BudgetLine{
			{ID: revenueHeadlineID, SheetID: revenueSheetID, OrdinalLabel: "A",
				Name: "TỔNG THU NỘI ĐỊA PHÁT SINH TRÊN ĐỊA BÀN", SortOrder: 1,
				Method: domain.MethodManual, IsHeadline: true},
		},
		Values: map[string]map[string]domain.Dong{
			revenueHeadlineID: {
				"c-tp":    millionDong(39_930_100),
				"c-xa":    millionDong(46_812_900),
				"c-nsnn":  millionDong(43_167_643),
				"c-huong": millionDong(33_008_005),
			},
		},
	}
}

// sampleBudget gives commune A both sheets of 2026 and commune B a chi sheet with a DIFFERENT title
// and different figures. Two communes whose sheets looked alike could not show a leak.
func sampleBudget() *fakeBudgetReader {
	tenantBExpenditureSheet := tenantAExpenditureSheet()
	tenantBExpenditureSheet.Sheet.Title = "BÁO CÁO CHI NGÂN SÁCH NHÀ NƯỚC XÃ BÌNH DƯƠNG NĂM 2026"
	tenantBExpenditureSheet.Sheet.Code = "NS-2026-CHI-01"
	tenantBExpenditureSheet.Values = map[string]map[string]domain.Dong{
		expenditureHeadlineID: {"c-dt": millionDong(1_000_000), "c-chi": millionDong(500_000)},
	}
	tenantBLineEntries := tenantALineEntries()
	tenantBLineEntries.Entries = tenantBLineEntries.Entries[:1]
	tenantBLineEntries.Entries[0].Content = "Thu phí chợ BÌNH DƯƠNG"
	return &fakeBudgetReader{
		byTenant: map[tenant.ID]map[string]domain.FullSheet{
			tenantA: {
				sheetKey(2026, domain.SheetKindExpenditure): tenantAExpenditureSheet(),
				sheetKey(2026, domain.SheetKindRevenue):     tenantARevenueSheet(),
			},
			tenantB: {sheetKey(2026, domain.SheetKindExpenditure): tenantBExpenditureSheet},
		},
		entries: map[tenant.ID]map[string]domain.LineEntries{
			tenantA: {"k-a": tenantALineEntries()},
			tenantB: {"k-a": tenantBLineEntries},
		},
	}
}

// sampleCounterpartyHTTP stands for what a commune types into "Đơn vị, cá nhân" — possibly a person's name.
const sampleCounterpartyHTTP = "Hộ bà Trần Thị Mẫu"

// tenantALineEntries is the `⇄` list of commune A's line `k-a`, in `entries` mode: two live batches, newest
// first, the second leaving `c-dt` empty.
func tenantALineEntries() domain.LineEntries {
	b := tenantAExpenditureSheet()
	k := b.Lines[1]
	k.Method = domain.MethodEntries
	return domain.LineEntries{
		Line:    k,
		Columns: b.Columns,
		Entries: []domain.BudgetEntry{
			{ID: "01JDOTMOINHAT0000000000000", LineID: "k-a",
				Date: time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC), Content: "Chi hỗ trợ đợt 2",
				Counterparty: sampleCounterpartyHTTP, VoucherNo: "PC-0102", EnteredBy: writerStaffCode,
				CreatedAt: time.Date(2026, 8, 20, 9, 0, 0, 0, time.UTC),
				Amounts:   map[string]domain.Dong{"c-dt": 0, "c-chi": 2_000_000}},
			{ID: "01JDOTCUHON00000000000000", LineID: "k-a",
				Date: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), Content: "Chi hỗ trợ đợt 1",
				EnteredBy: writerStaffCode, CreatedAt: time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC),
				Amounts: map[string]domain.Dong{"c-chi": -150_000}},
		},
	}
}

// --- harness -------------------------------------------------------------------------------------
//
// ITS OWN, and not routes_test.go's, for the same reason the voucher suite has one: the "right
// permission, wrong commune" case needs a checker whose grants are KEYED BY COMMUNE, which a flat
// permission set cannot express.

type budgetServer struct {
	h       http.Handler
	reader  *fakeBudgetReader
	writer  *fakeBudgetWriter
	checker *fakeCatalogueChecker

	// logBuf captures everything the handler logs, so a test can assert what NEVER reaches a log line
	// (rule 3 — the batch counterparty).
	logBuf *bytes.Buffer
}

func newBudgetServer(t *testing.T) *budgetServer {
	t.Helper()

	reader := sampleBudget()
	writer := &fakeBudgetWriter{
		result: domain.BudgetLine{
			ID: "01JKHOANMUCMOI000000000000", SheetID: expenditureSheetID, OrdinalLabel: "1.1",
			Name: "Chi quốc phòng", SortOrder: 10, Method: domain.MethodManual, Level: 2,
		},
		resultSheet: domain.BudgetSheet{
			ID: expenditureSheetID, Code: "NS-2026-CHI-01", Year: 2026, Kind: domain.SheetKindExpenditure, Revision: 1,
			Title: "BÁO CÁO CHI NGÂN SÁCH NHÀ NƯỚC XÃ THĂNG BÌNH NĂM 2026", Unit: "Triệu đồng",
		},
	}
	checker := &fakeCatalogueChecker{}
	logBuf := &bytes.Buffer{}
	quiet := slog.New(slog.NewTextHandler(logBuf, nil))

	mux := http.NewServeMux()
	Register(mux, Deps{
		Checker:                   checker,
		CapitalPlanCategories:     sampleCategories(),
		CapitalPlanCategoryWriter: &fakeCategoryWriter{},
		InvestmentProjects:        sampleInvestmentProjects(),
		InvestmentProjectWriter:   &fakeInvestmentProjectWriter{},
		DisbursementVoucherWriter: &fakeVoucherWriter{},
		DelayThresholds:           defaultThresholds(),
		Budget:                    reader,
		BudgetWriter:              writer,
		AuditLog:                  &auditLogFake{},
		SystemMessages:            &systemMessagesFake{},
		Now:                       func() time.Time { return atElapsed7096 },
		Log:                       quiet,

		// The catalogue's Excel import — never called here; own suite in internal/http/catalogue_import_test.go.
		CapitalPlanCategoryImports: &catalogueImportFake{},
	})

	var h http.Handler = mux
	h = injectPrincipal(h)
	h = idem.Middleware(newFakeIdemStore(), quiet)(h)
	h = httpx.TenantMiddleware(sampleDirectory())(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)

	return &budgetServer{h: h, reader: reader, writer: writer, checker: checker, logBuf: logBuf}
}

func (m *budgetServer) grant(tenantID tenant.ID, perm ...authz.Perm) {
	if m.checker.grants == nil {
		m.checker.grants = map[tenant.ID]map[authz.Perm]struct{}{}
	}
	if m.checker.grants[tenantID] == nil {
		m.checker.grants[tenantID] = map[authz.Perm]struct{}{}
	}
	for _, p := range perm {
		m.checker.grants[tenantID][p] = struct{}{}
	}
}

// call ALWAYS SENDS AN Idempotency-Key. The two POST routes declare idem.Required, which refuses a
// request without the header BEFORE it reaches the handler — so a harness that omitted it would turn
// every POST assertion into an assertion about the header.
func (m *budgetServer) call(t *testing.T, method, host, path string,
	p *authz.Principal, payload string) *httptest.ResponseRecorder {
	t.Helper()
	var body io.Reader
	if payload != "" {
		body = strings.NewReader(payload)
	}
	r := httptest.NewRequest(method, "https://"+host+path, body)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set(idem.Header, "01JIDEMPOTENCYKEYNGANSACH")
	if p != nil {
		r = r.WithContext(context.WithValue(r.Context(), principalCtxKey{}, *p))
	}
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)
	return w
}

const (
	sheetPath       = "/api/v1/budget-sheets"
	indicatorPath   = "/api/v1/budget-indicators"
	linePath        = "/api/v1/budget-lines"
	createSheetBody = `{"year":2027,"kind":"chi","title":"BÁO CÁO CHI NGÂN SÁCH 2027","unit":"trieu-dong",` +
		`"columns":[{"name":"Dự toán năm","order":1,"type":"so","role":"du-toan-nam"},` +
		`{"name":"Chi ngân sách","order":2,"type":"so","role":"chi-ngan-sach"}]}`
	createLineBody = `{"sheet_id":"01JBANGCHI0000000000000000","parent_id":"k-a","no":"1.1",` +
		`"name":"Chi quốc phòng","order":10}`
	entryPath       = "/api/v1/budget-entries"
	recordEntryBody = `{"date":"2026-08-20","content":"Chi hỗ trợ đợt 3","counterparty":"` + sampleCounterpartyHTTP +
		`","document_no":"PC-0103","values":{"c-chi":2500000,"c-dt":null}}`
)

// budgetWriteRoute is one of the eight routes, with the key §9 rule 7's group assigns to it.
type budgetWriteRoute struct {
	name   string
	method string
	path   string
	body   string
	perm   authz.Perm
	ok     int
	count  func(m *budgetServer) int
}

// eightBudgetWriteRoutes — the four permission cases are asserted on ALL EIGHT rather than on whichever
// one was written first, and the `perm` column is what makes the suite able to tell the three keys
// apart: a route that quietly asked for `budget.update` where this session assigned `budget.confirm`
// would let a clerk move the star that decides the commune's reported total.
func eightBudgetWriteRoutes() []budgetWriteRoute {
	return []budgetWriteRoute{
		{"GET bang", http.MethodGet, sheetPath + "?year=2026&kind=chi", "",
			"budget.read", http.StatusOK, func(m *budgetServer) int { return m.reader.calls }},
		{"GET chi so", http.MethodGet, indicatorPath + "?year=2026", "",
			"budget.read", http.StatusOK, func(m *budgetServer) int {
				// Two sheets are read for one request, so this counts requests rather than reads.
				if m.reader.calls > 0 {
					return 1
				}
				return 0
			}},
		{"POST bang", http.MethodPost, sheetPath, createSheetBody,
			"budget.update", http.StatusCreated, func(m *budgetServer) int { return m.writer.createSheetCalls }},
		{"PATCH bang", http.MethodPatch, sheetPath + "/" + expenditureSheetID, `{"title":"BÁO CÁO CHI (đã sửa)"}`,
			"budget.update", http.StatusOK, func(m *budgetServer) int { return m.writer.updateSheetCalls }},
		{"DELETE bang", http.MethodDelete, sheetPath + "/" + expenditureSheetID, `{"reason":"nạp lại từ tệp đã sửa"}`,
			"budget.confirm", http.StatusNoContent, func(m *budgetServer) int { return m.writer.removeSheetCalls }},
		{"POST dong", http.MethodPost, linePath, createLineBody,
			"budget.update", http.StatusCreated, func(m *budgetServer) int { return m.writer.createCalls }},
		{"PATCH dong", http.MethodPatch, linePath + "/" + expenditureHeadlineID, `{"name":"Tổng số (đã sửa)"}`,
			"budget.update", http.StatusOK, func(m *budgetServer) int { return m.writer.updateCalls }},
		{"DELETE dong", http.MethodDelete, linePath + "/" + expenditureHeadlineID, `{"reason":"nhập trùng"}`,
			"budget.confirm", http.StatusNoContent, func(m *budgetServer) int { return m.writer.removeCalls }},
		{"POST dong tong", http.MethodPost, linePath + "/" + expenditureHeadlineID + "/headline", "",
			"budget.confirm", http.StatusOK, func(m *budgetServer) int { return m.writer.headlineCalls }},
		{"GET dot", http.MethodGet, linePath + "/k-a/entries", "",
			"budget.read", http.StatusOK, func(m *budgetServer) int { return m.reader.calls }},
		{"POST dot", http.MethodPost, linePath + "/k-a/entries", recordEntryBody,
			"budget.update", http.StatusCreated, func(m *budgetServer) int { return m.writer.recordEntryCalls }},
		{"DELETE dot", http.MethodDelete, entryPath + "/01JDOTMOINHAT0000000000000", `{"reason":"ghi nhầm số chứng từ"}`,
			"budget.confirm", http.StatusNoContent, func(m *budgetServer) int { return m.writer.removeEntryCalls }},
	}
}

func (m *budgetServer) ran() int { return m.reader.calls + m.writer.totalCalls() }

// --- (2) the permission keys themselves ------------------------------------------------------------

func TestBudgetPermissionKeysMatchPermissionTable(t *testing.T) {
	// THE KEY EACH ROUTE ACTUALLY ASKS FOR, COMPARED AGAINST A LITERAL — and that is the whole point.
	//
	// Every other assertion in this file grants the key and then expects it to be accepted, so
	// changing a route's key to ANY string leaves them all green: the fake checker grants whatever it
	// was handed. That is not hypothetical — this repository carried three invented keys through
	// several sessions with every suite green, because a route holding a key the `quyen` table lacks
	// answers 403 to EVERY account, forever, and nothing says so (rule 5, invariant 3c).
	//
	// All three keys are seeded at service-identity/migrations/0001_init.sql:282-284 and named for
	// this screen at docs/ui-ux/07-thu-chi-ngan-sach.md:236. tools/check_quyen.py scans the whole
	// repository against that table on every `make check` and is the guard a fixture cannot fool;
	// this is the cheap half that turns red in `go test` too.
	for _, tc := range eightBudgetWriteRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			m := newBudgetServer(t)
			m.call(t, tc.method, hostA, tc.path, writerPrincipal(tenantA), tc.body)

			if got := m.checker.lastAskedKey(); got != tc.perm {
				t.Fatalf("tuyến hỏi khoá %q, muốn %q — một khoá bảng `quyen` không có là một tuyến "+
					"trả 403 với MỌI tài khoản, mãi mãi, và không phép kiểm nào đỏ", got, tc.perm)
			}
		})
	}
}

// --- (1) the four cases of rule 5, invariant 7 -------------------------------------------------------

func TestBudget_401WithoutSession(t *testing.T) {
	for _, tc := range eightBudgetWriteRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			m := newBudgetServer(t)
			m.grant(tenantA, "budget.read", "budget.update", "budget.confirm") // granted, still refused
			wantStatus(t, m.call(t, tc.method, hostA, tc.path, nil, tc.body), http.StatusUnauthorized)
			if m.ran() != 0 {
				t.Error("chưa đăng nhập mà kho/use case đã chạy")
			}
		})
	}
}

func TestBudget_403WrongPermission(t *testing.T) {
	// A signed-in account of the right commune holding a DIFFERENT permission. `admin.lookup` is
	// deliberately a REAL key of this service — the failure being guarded against is not "an account
	// with nothing", it is somebody whose job is the catalogue screen being able to move the figures
	// this commune reports upward.
	for _, tc := range eightBudgetWriteRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			m := newBudgetServer(t)
			m.grant(tenantA, "admin.lookup")

			wantStatus(t, m.call(t, tc.method, hostA, tc.path, writerPrincipal(tenantA), tc.body),
				http.StatusForbidden)
			if m.ran() != 0 {
				t.Error("sai quyền mà kho/use case vẫn chạy")
			}
		})
	}
}

func TestBudget_403EntererCannotSetHeadline(t *testing.T) {
	// THE SPLIT THIS SESSION DREW, ASSERTED AS A SEPARATE CASE because it is the one an ordinary
	// "grant the module's permissions" fixture would paper over: an accountant holding `budget.read`
	// and `budget.update` may create the sheet, add lines and type figures, and may NOT remove a
	// sheet, remove a line, or MOVE THE STAR. The star decides which row every summary cell and both
	// indicators are read from (ADR 0035 §A), which is the weight of a confirmation and not of data
	// entry.
	m := newBudgetServer(t)
	m.grant(tenantA, "budget.read", "budget.update")

	for _, tc := range eightBudgetWriteRoutes() {
		if tc.perm != "budget.confirm" {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			wantStatus(t, m.call(t, tc.method, hostA, tc.path, writerPrincipal(tenantA), tc.body),
				http.StatusForbidden)
		})
	}
	if m.writer.totalCalls() != 0 {
		t.Errorf("use case ghi chạy %d lần với tài khoản chỉ có `budget.read` + `budget.update`",
			m.writer.totalCalls())
	}
}

func TestBudget_403RightPermissionWrongCommune(t *testing.T) {
	// THE CASE THAT IS EASIEST TO FAKE AND HARDEST TO GET RIGHT, so read what it actually sets up.
	//
	// The account belongs to commune B and is signed in AT COMMUNE B: nothing about the request is
	// malformed, and authz's own commune comparison passes. What is wrong is the GRANT — the right to
	// read and write this board was given in commune A. A checker that ignored the commune would
	// answer yes here, and commune A's accountant would be editing commune B's budget.
	//
	// That is rule 5, invariant 3 in one sentence: a permission missing its commune is cross-commune
	// escalation, not a lesser bug. And it answers 403 rather than 401 because the session is
	// perfectly valid — it is the authority that is absent.
	for _, tc := range eightBudgetWriteRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			m := newBudgetServer(t)
			m.grant(tenantA, "budget.read", "budget.update", "budget.confirm")

			wantStatus(t, m.call(t, tc.method, hostB, tc.path, writerPrincipal(tenantB), tc.body),
				http.StatusForbidden)
			if m.ran() != 0 {
				t.Error("quyền cấp ở xã khác mà vẫn đọc/ghi được vào xã này")
			}
		})
	}
}

func TestBudget_401SessionOfOtherCommune(t *testing.T) {
	// The other shape of "wrong commune": a principal issued by commune A presented at commune B's
	// domain. authz.RequirePermission compares the commune BEFORE the permission and answers 401,
	// not 403 — a browser does not send a cookie across hosts, so this is never an ordinary user
	// error.
	for _, tc := range eightBudgetWriteRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			m := newBudgetServer(t)
			m.grant(tenantA, "budget.read", "budget.update", "budget.confirm")
			m.grant(tenantB, "budget.read", "budget.update", "budget.confirm")

			wantStatus(t, m.call(t, tc.method, hostB, tc.path, writerPrincipal(tenantA), tc.body),
				http.StatusUnauthorized)
			if m.ran() != 0 {
				t.Error("phiên của xã khác mà vẫn đọc/ghi được")
			}
		})
	}
}

func TestBudget_RightPermissionRightCommune(t *testing.T) {
	for _, tc := range eightBudgetWriteRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			m := newBudgetServer(t)
			m.grant(tenantA, "budget.read", "budget.update", "budget.confirm")

			wantStatus(t, m.call(t, tc.method, hostA, tc.path, writerPrincipal(tenantA), tc.body), tc.ok)
			if got := tc.count(m); got != 1 {
				t.Fatalf("tầng dưới chạy %d lần, muốn 1", got)
			}
			if m.writer.totalCalls() == 0 {
				return // a read route: the commune is asserted through the fixture below
			}
			// Rule 6, invariant 2: WHO, and IN WHICH COMMUNE. Both have to reach the layer that
			// writes the entry, or the trail cannot answer the only question it exists for.
			if m.writer.lastTenant != tenantA {
				t.Errorf("use case chạy trong xã %q, muốn %q", m.writer.lastTenant, tenantA)
			}
			// THE TRAIL CARRIES THE BUSINESS CODE, AND THE SECOND CHECK NAMES THE WRONG VALUE
			// OUTRIGHT. Asserting only "equals the code" would stay green the day somebody made the
			// two constants the same string.
			if m.writer.lastActor.ID != writerStaffCode || m.writer.lastActor.ID == internalStaffID {
				t.Errorf("chủ thể vết = %q, muốn mã cán bộ %q (KHÔNG phải id nội bộ %q)",
					m.writer.lastActor.ID, writerStaffCode, internalStaffID)
			}
		})
	}
}

func TestReadSheetOfEachCommuneReturnsThatCommunesSheet(t *testing.T) {
	// Rule 1 at the surface it is actually read from: two communes, one year, one kind, and the
	// titles differ. A store that ignored the commune would return commune A's sheet to commune B.
	m := newBudgetServer(t)
	m.grant(tenantA, "budget.read")
	m.grant(tenantB, "budget.read")

	var a, b bangDayDuRa
	decodeJSON(t, m.call(t, http.MethodGet, hostA, sheetPath+"?year=2026&kind=chi", writerPrincipal(tenantA), ""), &a)
	decodeJSON(t, m.call(t, http.MethodGet, hostB, sheetPath+"?year=2026&kind=chi", writerPrincipal(tenantB), ""), &b)

	if a.Sheet.Title == b.Sheet.Title {
		t.Fatalf("hai xã nhận cùng một tiêu đề %q", a.Sheet.Title)
	}
	if !strings.Contains(a.Sheet.Title, "THĂNG BÌNH") || !strings.Contains(b.Sheet.Title, "BÌNH DƯƠNG") {
		t.Fatalf("bảng lẫn xã: A=%q B=%q", a.Sheet.Title, b.Sheet.Title)
	}
}

// --- (3) the marked row -------------------------------------------------------------------------------

func TestNoMarkedRowMeansNoHeadlineValueAndOneSentence(t *testing.T) {
	// ADR 0035 §A at the surface the commune actually reads. THE ASSERTION IS ON THE WIRE FORMAT, not
	// on the domain: a handler that turned domain.MoneyFigure's "no figure" into a JSON `0` would leave
	// every domain test green and put a zero on a government screen.
	m := newBudgetServer(t)
	m.grant(tenantA, "budget.read")

	zero := tenantAExpenditureSheet()
	for i := range zero.Lines {
		zero.Lines[i].IsHeadline = false
	}
	m.reader.byTenant[tenantA][sheetKey(2026, domain.SheetKindExpenditure)] = zero

	var result bangDayDuRa
	decodeJSON(t, m.call(t, http.MethodGet, hostA, sheetPath+"?year=2026&kind=chi", writerPrincipal(tenantA), ""), &result)

	if result.Summary.HeadlineLineID != "" {
		t.Fatalf("không dòng nào được đánh dấu mà vẫn trả dòng tổng %q", result.Summary.HeadlineLineID)
	}
	if len(result.Summary.Cells) != 0 {
		t.Fatalf("không dòng tổng mà vẫn có %d ô tóm tắt", len(result.Summary.Cells))
	}
	if result.Summary.UnavailableReason == "" {
		t.Fatal("không có dòng tổng mà cũng không có CÂU nào nói ra — đó là một màn hình trông như hỏng")
	}
	if result.Summary.Indicator.BasisPoints != nil {
		t.Fatalf("chỉ số vẫn ra %d phần vạn khi chưa có dòng tổng — phải là null",
			*result.Summary.Indicator.BasisPoints)
	}
	if result.Summary.Indicator.UnavailableReason == "" {
		t.Fatal("chỉ số biến mất mà không kèm lý do")
	}
}

func TestTwoMarkedRowsMeanNoHeadlineValueAndSayDoubleCount(t *testing.T) {
	// THE DECISION OF THIS TURN, ASSERTED AT THE SURFACE. Two marked rows is a sheet with two
	// candidate totals; the thu sheet's two nested top-level rows and the chi sheet's `Tổng số`
	// beside A…E are exactly why summing them, or taking the first, is a DOUBLE COUNT.
	//
	// The write path makes the star a radio so this is unreachable through any route here; it is
	// asserted because the database carries no constraint for it (migration 0006 says why) and the
	// state can arrive from an import or a psql session.
	m := newBudgetServer(t)
	m.grant(tenantA, "budget.read")

	two := tenantAExpenditureSheet()
	two.Lines[1].IsHeadline = true
	m.reader.byTenant[tenantA][sheetKey(2026, domain.SheetKindExpenditure)] = two

	var result bangDayDuRa
	decodeJSON(t, m.call(t, http.MethodGet, hostA, sheetPath+"?year=2026&kind=chi", writerPrincipal(tenantA), ""), &result)

	if result.Summary.HeadlineLineID != "" || len(result.Summary.Cells) != 0 {
		t.Fatal("hai dòng cùng đánh dấu mà vẫn chọn một dòng làm tổng")
	}
	if !strings.Contains(result.Summary.UnavailableReason, "đếm đôi") {
		t.Fatalf("lý do không nói ra phép đếm đôi: %q", result.Summary.UnavailableReason)
	}
	if result.Summary.Indicator.BasisPoints != nil {
		t.Fatal("chỉ số vẫn ra số khi có hai dòng tổng")
	}
}

// --- (4) the two figures ADR 0035 §A fixed --------------------------------------------------------------

func TestYearIndicatorsShowBothRevenueFiguresAndBalanceUsesCommuneRetained(t *testing.T) {
	// ADR 0035 §A's two mandatory consequences, on the wire:
	//   - BOTH revenue figures reach the screen, each by its own name;
	//   - the ONE thing there is a single answer for — `Cân đối` — is built on `Thu xã hưởng`.
	m := newBudgetServer(t)
	m.grant(tenantA, "budget.read")

	var result chiSoNamRa
	decodeJSON(t, m.call(t, http.MethodGet, hostA, indicatorPath+"?year=2026", writerPrincipal(tenantA), ""), &result)

	if len(result.RevenueTotals) != 2 {
		t.Fatalf("màn hình nhận %d số thu, ADR 0035 §A bắt hiện CẢ HAI", len(result.RevenueTotals))
	}
	indicator := map[string]int64{}
	for _, o := range result.RevenueTotals {
		if o.Value == nil {
			t.Fatalf("số thu %q không có giá trị", o.Name)
		}
		indicator[o.Role] = *o.Value
	}
	if indicator["thu-nsnn"] != int64(millionDong(43_167_643)) {
		t.Errorf("Thu ngân sách NSNN = %d", indicator["thu-nsnn"])
	}
	if indicator["thu-xa-huong"] != int64(millionDong(33_008_005)) {
		t.Errorf("Thu xã hưởng = %d", indicator["thu-xa-huong"])
	}

	if result.Balance.Amount == nil {
		t.Fatalf("Cân đối không ra số: %s", result.Balance.UnavailableReason)
	}
	want := int64(millionDong(33_008_005) - millionDong(34_634_592))
	if *result.Balance.Amount != want {
		t.Fatalf("Cân đối = %d, muốn %d", *result.Balance.Amount, want)
	}
	if *result.Balance.Amount >= 0 {
		t.Fatal("Cân đối dương trên số liệu mẫu — đang lấy `Thu ngân sách NSNN` chứ không phải " +
			"`Thu xã hưởng` (ADR 0035 #32)")
	}

	if result.RevenueAchievement.BasisPoints == nil || *result.RevenueAchievement.BasisPoints != 10811 {
		t.Fatalf("Thu đạt dự toán = %v, muốn 10811 phần vạn (chia cho `Dự toán TP giao`, ADR 0035 #33)",
			result.RevenueAchievement.BasisPoints)
	}
	if result.ExpenditureAchievement.BasisPoints == nil || *result.ExpenditureAchievement.BasisPoints != 9127 {
		t.Fatalf("Chi đạt dự toán = %v, muốn 9127 phần vạn", result.ExpenditureAchievement.BasisPoints)
	}
}

func TestEmptyEstimateByProvinceDropsIndicatorWithSentenceNotZero(t *testing.T) {
	// ADR 0035 §A: `Dự toán TP giao` becomes a column that MUST have data, and a commune that leaves
	// it blank loses the indicator — "và điều đó phải hiện ra thành một CÂU, không thành một ô
	// trống". Neither 0 nor NaN, and the JSON field is `null` rather than absent so that a client
	// cannot read "the server does not report this" as "zero".
	m := newBudgetServer(t)
	m.grant(tenantA, "budget.read")

	revenue := tenantARevenueSheet()
	delete(revenue.Values[revenueHeadlineID], "c-tp")
	m.reader.byTenant[tenantA][sheetKey(2026, domain.SheetKindRevenue)] = revenue

	var result chiSoNamRa
	decodeJSON(t, m.call(t, http.MethodGet, hostA, indicatorPath+"?year=2026", writerPrincipal(tenantA), ""), &result)

	if result.RevenueAchievement.BasisPoints != nil {
		t.Fatalf("Thu đạt dự toán vẫn ra %d khi `Dự toán TP giao` trống",
			*result.RevenueAchievement.BasisPoints)
	}
	if result.RevenueAchievement.UnavailableReason == "" {
		t.Fatal("chỉ số biến mất mà không có CÂU nào nói vì sao")
	}
	// The raw JSON is checked as well: `null` and `0` are one character apart in a struct that has
	// just been round-tripped, and a `omitempty` added to BasisPoints would make the field vanish
	// instead — which a client reads as "not reported" rather than as "no figure".
	w := m.call(t, http.MethodGet, hostA, indicatorPath+"?year=2026", writerPrincipal(tenantA), "")
	if !strings.Contains(w.Body.String(), `"basis_points":null`) {
		t.Fatalf("thân JSON không mang `\"basis_points\":null`: %s", w.Body.String())
	}
}

func TestMissingExpenditureSheetDropsBalanceAndAttainmentNotZero(t *testing.T) {
	// "Xã chưa nhập bảng chi" and "xã chi 0 đồng" are different statements and only one of them is
	// ever true. 200 with reasons, NOT 404: the card belongs on the screen, with a job to do on it.
	m := newBudgetServer(t)
	m.grant(tenantA, "budget.read")
	delete(m.reader.byTenant[tenantA], sheetKey(2026, domain.SheetKindExpenditure))

	w := m.call(t, http.MethodGet, hostA, indicatorPath+"?year=2026", writerPrincipal(tenantA), "")
	wantStatus(t, w, http.StatusOK)

	var result chiSoNamRa
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("thân không phải JSON: %v", err)
	}
	if result.Balance.Amount != nil {
		t.Fatalf("thiếu bảng chi mà Cân đối vẫn ra %d", *result.Balance.Amount)
	}
	if result.Balance.UnavailableReason == "" || result.ExpenditureAchievement.UnavailableReason == "" {
		t.Fatal("thiếu bảng chi mà không có lý do nào hiện ra")
	}
	// The revenue half is untouched: a commune that has entered its revenue sheet must still see it.
	if result.RevenueAchievement.BasisPoints == nil {
		t.Fatal("thiếu bảng CHI mà chỉ số THU cũng mất")
	}
}

// --- (5) the fields a client may not set -----------------------------------------------------------------

func TestClientCannotSetMethodLevelOrHeadline(t *testing.T) {
	// REFUSED, NOT IGNORED, and each for its own reason:
	//   `method`      a parent marked `manual` is the direct entry anh Hà's 06/09/2026 rule forbids;
	//   `level`       a second answer to a question the tree already answers;
	//   `is_headline` a client believing it just made this row the commune's reported total.
	m := newBudgetServer(t)
	m.grant(tenantA, "budget.update")

	for _, tc := range []struct{ name, body string }{
		{"method", `{"sheet_id":"` + expenditureSheetID + `","name":"X","order":1,"method":"manual"}`},
		{"level", `{"sheet_id":"` + expenditureSheetID + `","name":"X","order":1,"level":3}`},
		{"is_headline", `{"sheet_id":"` + expenditureSheetID + `","name":"X","order":1,"is_headline":true}`},
	} {
		t.Run("POST "+tc.name, func(t *testing.T) {
			w := m.call(t, http.MethodPost, hostA, linePath, writerPrincipal(tenantA), tc.body)
			wantStatus(t, w, http.StatusBadRequest)
		})
	}
	for _, tc := range []struct{ name, body string }{
		{"method", `{"method":"children"}`},
		{"level", `{"level":0}`},
		{"is_headline", `{"is_headline":true}`},
		{"parent_id", `{"parent_id":"k-a"}`},
		{"sheet_id", `{"sheet_id":"` + revenueSheetID + `"}`},
	} {
		t.Run("PATCH "+tc.name, func(t *testing.T) {
			w := m.call(t, http.MethodPatch, hostA, linePath+"/"+expenditureHeadlineID, writerPrincipal(tenantA), tc.body)
			wantStatus(t, w, http.StatusBadRequest)
		})
	}
	if m.writer.totalCalls() != 0 {
		t.Errorf("use case chạy %d lần với thân yêu cầu đã bị từ chối", m.writer.totalCalls())
	}
}

func TestTypingIntoParentLineIs409WithCustomersSentence(t *testing.T) {
	// The customer's decision of 06/09/2026 reaching the accountant as a sentence, with 409 rather
	// than 403: the caller HOLDS `budget.update` and is allowed to type figures. What is refused is
	// this figure on THIS row, because the row has children.
	m := newBudgetServer(t)
	m.grant(tenantA, "budget.update")
	m.writer.err = domain.ErrParentLineNoDirectValue

	w := m.call(t, http.MethodPatch, hostA, linePath+"/"+expenditureHeadlineID, writerPrincipal(tenantA),
		`{"values":{"c-chi":1000000}}`)
	wantStatus(t, w, http.StatusConflict)
	if e := errorBody(t, w); !strings.Contains(e.Message, "cộng từ các dòng con") {
		t.Fatalf("câu trả về không nói ra quy tắc: %q", e.Message)
	}
}

func TestEmptyCellDiffersFromZeroCell(t *testing.T) {
	// §9 rule 4 on the wire: a filled cell is a number, an empty one is `null`, and `0` is a filled
	// cell holding zero. A handler that emitted 0 for an empty cell would print `0` where the screen
	// must draw `—`, and that zero would be totalled into a figure sent upward.
	m := newBudgetServer(t)
	m.grant(tenantA, "budget.read")

	b := tenantAExpenditureSheet()
	b.Values["k-a"] = map[string]domain.Dong{"c-dt": 0} // 0 typed in; `c-chi` left empty
	m.reader.byTenant[tenantA][sheetKey(2026, domain.SheetKindExpenditure)] = b

	var result bangDayDuRa
	decodeJSON(t, m.call(t, http.MethodGet, hostA, sheetPath+"?year=2026&kind=chi", writerPrincipal(tenantA), ""), &result)

	var row dongRa
	for _, d := range result.Lines {
		if d.ID == "k-a" {
			row = d
		}
	}
	if row.Values["c-dt"] == nil || *row.Values["c-dt"] != 0 {
		t.Fatalf("ô ghi 0 trả về %v, muốn 0", row.Values["c-dt"])
	}
	if v, ok := row.Values["c-chi"]; !ok || v != nil {
		t.Fatalf("ô trống trả về %v, muốn null", v)
	}
	if _, ok := row.Values["c-ty"]; ok {
		t.Error("cột phần trăm không được có ô giá trị — phần trăm tính khi hiển thị (§9 quy tắc 3)")
	}
}

func TestMissingYearOrKindIsRefusedNotDefaulted(t *testing.T) {
	// A default here decides WHICH MONEY gets reported, invisibly. §13's reasoning for du_an applies
	// unchanged, and `kind` has no sensible default at all — the two tabs are two different reports.
	m := newBudgetServer(t)
	m.grant(tenantA, "budget.read")

	for _, path := range []string{
		sheetPath,
		sheetPath + "?kind=chi",
		sheetPath + "?year=2026",
		sheetPath + "?year=2026&kind=ca-hai",
		sheetPath + "?year=1026&kind=chi",
		indicatorPath,
	} {
		t.Run(path, func(t *testing.T) {
			wantStatus(t, m.call(t, http.MethodGet, hostA, path, writerPrincipal(tenantA), ""), http.StatusBadRequest)
		})
	}
	if m.reader.calls != 0 {
		t.Errorf("kho chạy %d lần với tham số đã bị từ chối", m.reader.calls)
	}
}

// --- PATCH /api/v1/budget-sheets/{id} and the closed unit list -----------------------------------------

func TestUpdateSheet_EveryFieldIsOptional(t *testing.T) {
	// EACH FIELD ALONE reaches the use case alone: a nil pointer is "leave it", so a body naming only
	// the title must not blank the unit or the cut-off date.
	for _, tc := range []struct {
		name, body                string
		title, unit, cumulativeTo bool
		cumulativeToEmpty         bool
	}{
		{"title", `{"title":"BÁO CÁO CHI (đã sửa)"}`, true, false, false, false},
		{"unit", `{"unit":"nghin-dong"}`, false, true, false, false},
		{"cumulative_to", `{"cumulative_to":"2026-09-30"}`, false, false, true, false},
		{"cumulative_to rỗng là bỏ mốc", `{"cumulative_to":""}`, false, false, true, true},
		{"cumulative_to null là để nguyên", `{"cumulative_to":null}`, false, false, false, false},
		{"rỗng", `{}`, false, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newBudgetServer(t)
			m.grant(tenantA, "budget.update")
			wantStatus(t, m.call(t, http.MethodPatch, hostA, sheetPath+"/"+expenditureSheetID, writerPrincipal(tenantA), tc.body),
				http.StatusOK)
			req := m.writer.lastUpdateSheet
			if (req.Title != nil) != tc.title || (req.Unit != nil) != tc.unit ||
				(req.CumulativeTo != nil) != tc.cumulativeTo {
				t.Fatalf("trường tới use case: title=%v unit=%v cumulative_to=%v",
					req.Title != nil, req.Unit != nil, req.CumulativeTo != nil)
			}
			if tc.cumulativeTo && req.CumulativeTo.IsZero() != tc.cumulativeToEmpty {
				t.Fatalf("cumulative_to = %v, muốn rỗng=%v", *req.CumulativeTo, tc.cumulativeToEmpty)
			}
			if m.writer.lastID != expenditureSheetID {
				t.Fatalf("id tới use case = %q", m.writer.lastID)
			}
		})
	}
}

func TestUpdateSheet_IdentifyingFieldsAndBadDateAreRefused(t *testing.T) {
	// REFUSED, NOT IGNORED: the decoder accepts unknown fields, so a `year` that silently vanished
	// would leave the client believing it had moved a year's budget.
	m := newBudgetServer(t)
	m.grant(tenantA, "budget.update")
	for _, body := range []string{
		`{"year":2027}`, `{"kind":"thu"}`, `{"code":"NS-2026-CHI-09"}`, `{"columns":[]}`,
		`{"cumulative_to":"30/09/2026"}`,
	} {
		t.Run(body, func(t *testing.T) {
			wantStatus(t, m.call(t, http.MethodPatch, hostA, sheetPath+"/"+expenditureSheetID, writerPrincipal(tenantA), body),
				http.StatusBadRequest)
		})
	}
	if m.writer.updateSheetCalls != 0 {
		t.Errorf("use case chạy %d lần với thân đã bị từ chối", m.writer.updateSheetCalls)
	}
}

func TestUpdateSheet_BadUnitIs400(t *testing.T) {
	// The closed list is enforced in internal/domain (the app tests prove the real refusal opens no
	// transaction); this proves the refusal reaches the client as 400 with the domain's sentence.
	m := newBudgetServer(t)
	m.grant(tenantA, "budget.update")
	m.writer.err = fmt.Errorf("ngan_sach: sửa bảng cho xã x: %w", domain.ErrInvalidUnit)

	w := m.call(t, http.MethodPatch, hostA, sheetPath+"/"+expenditureSheetID, writerPrincipal(tenantA), `{"unit":"Triệu đồng"}`)
	wantStatus(t, w, http.StatusBadRequest)
	if e := errorBody(t, w); !strings.Contains(e.Message, "trieu-dong") {
		t.Fatalf("câu trả về không liệt kê các mã hợp lệ: %q", e.Message)
	}
}

func TestUpdateSheet_RemovedOrOtherCommuneIs404WithSameBody(t *testing.T) {
	// A removed sheet and another commune's sheet are the SAME store answer (the locked read excludes
	// deleted rows and is scoped by tenant_id), so they must be the same bytes on the wire.
	m := newBudgetServer(t)
	m.grant(tenantA, "budget.update")
	m.writer.err = fmt.Errorf("ngan_sach: sửa bảng cho xã x: %w", fistore.ErrBudgetSheetNotFound)

	a := m.call(t, http.MethodPatch, hostA, sheetPath+"/"+expenditureSheetID, writerPrincipal(tenantA), `{"title":"X"}`)
	b := m.call(t, http.MethodPatch, hostA, sheetPath+"/01JBANGCUAXAKHAC000000000", writerPrincipal(tenantA), `{"title":"X"}`)
	wantStatus(t, a, http.StatusNotFound)
	wantStatus(t, b, http.StatusNotFound)
	if a.Body.String() != b.Body.String() {
		t.Fatalf("hai thân 404 khác nhau:\n%s\n%s", a.Body.String(), b.Body.String())
	}
}

func TestUpdateSheet_ReturnsSheetWithUnitCode(t *testing.T) {
	m := newBudgetServer(t)
	m.grant(tenantA, "budget.update")

	var result bangRa
	decodeJSON(t, m.call(t, http.MethodPatch, hostA, sheetPath+"/"+expenditureSheetID, writerPrincipal(tenantA),
		`{"unit":"trieu-dong"}`), &result)
	if result.Unit != "trieu-dong" || result.UnitLabel != "Triệu đồng" || result.UnitWarning != "" {
		t.Fatalf("unit=%q label=%q warning=%q", result.Unit, result.UnitLabel, result.UnitWarning)
	}
}

func TestLegacyUnitReadsAsCodeOrWarning(t *testing.T) {
	// A sheet created before 25/09/2026 holds free text. Mapped when unambiguous; otherwise `unit` is
	// EMPTY, the stored text is printed verbatim, and a warning says so. Never a guessed code: a wrong
	// one displays every figure a thousand times off.
	for _, tc := range []struct {
		stored, code, label string
		warning             bool
	}{
		{"Triệu đồng", "trieu-dong", "Triệu đồng", false},
		{"ngàn đồng", "nghin-dong", "Nghìn đồng", false},
		{"Tỷ đồng", "", "Tỷ đồng", true},
	} {
		t.Run(tc.stored, func(t *testing.T) {
			m := newBudgetServer(t)
			m.grant(tenantA, "budget.read")
			b := tenantAExpenditureSheet()
			b.Sheet.Unit = tc.stored
			m.reader.byTenant[tenantA][sheetKey(2026, domain.SheetKindExpenditure)] = b

			var result bangDayDuRa
			decodeJSON(t, m.call(t, http.MethodGet, hostA, sheetPath+"?year=2026&kind=chi", writerPrincipal(tenantA), ""), &result)
			if result.Sheet.Unit != tc.code || result.Sheet.UnitLabel != tc.label || (result.Sheet.UnitWarning != "") != tc.warning {
				t.Fatalf("unit=%q label=%q warning=%q", result.Sheet.Unit, result.Sheet.UnitLabel, result.Sheet.UnitWarning)
			}
			// The figures are untouched by the unit: still đồng.
			if v := result.Lines[0].Values["c-chi"]; v == nil || *v != int64(millionDong(34_634_592)) {
				t.Fatalf("số liệu bị đổi theo đơn vị: %v", v)
			}
		})
	}
}

func decodeJSON(t *testing.T, w *httptest.ResponseRecorder, in any) {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("mã trạng thái = %d, muốn 200 — thân: %s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), in); err != nil {
		t.Fatalf("thân không phải JSON: %v — %s", err, w.Body.String())
	}
}
