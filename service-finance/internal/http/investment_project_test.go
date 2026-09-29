package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// The four cases rule 5, invariant 7 requires, for both disbursement routes:
//
//	401  no session
//	403  a session with the WRONG permission
//	403  the RIGHT permission, the WRONG commune   <- the one a single-commune test cannot produce
//	200  both correct
//
// plus what those four cannot show: that a commune's projects never reach another commune's
// caller, that the delay score is derived rather than stored, and that a budget year is demanded
// rather than defaulted.
//
// THE 403-WRONG-COMMUNE CASE ANSWERS 401 HERE, NOT 403, and that is the shipped behaviour rather
// than a compromise: authz.RequirePermission compares the commune in the principal against the
// commune resolved from Host BEFORE it consults the checker, and answers 401 — a browser does not
// send a cookie across hosts, so a mismatch is never an ordinary user error but a stolen or
// replayed token. What the rule asks for is that the case be EXERCISED and refused; the assertion
// below pins the code that ships so that a change to it cannot pass unnoticed.

// atElapsed7096 is the instant at which the 2026 budget year is 70,96% elapsed — the figure §3 uses
// in its worked examples. The routes are built with a clock fixed here (newTestServerWith), so every
// delay score below is reproducible.
var atElapsed7096 = time.Date(2026, time.September, 17, 0, 6, 0, 0, time.FixedZone("ICT", 7*3600))

// fakeInvestmentProjectReader is the project store, KEYED BY COMMUNE, reading the commune from the context exactly as
// *store.Scoped does. Keyed any other way, the isolation cases below would pass while proving
// nothing.
//
// `calls` counts the reads: the count is what proves the permission guard runs BEFORE any store
// access, rather than merely producing the right status afterwards.
type fakeInvestmentProjectReader struct {
	byTenant map[tenant.ID][]domain.InvestmentProjectProgress
	err      error
	calls    int
}

func (d *fakeInvestmentProjectReader) ListInvestmentProjects(ctx context.Context, filter fistore.InvestmentProjectFilter) ([]domain.InvestmentProjectProgress, error) {
	d.calls++
	if d.err != nil {
		return nil, d.err
	}
	var result []domain.InvestmentProjectProgress
	for _, one := range d.byTenant[tenant.MustFrom(ctx)] {
		if one.InvestmentProject.Year != filter.Year {
			continue
		}
		if filter.CategoryID != "" && one.InvestmentProject.CategoryID != filter.CategoryID {
			continue
		}
		result = append(result, one)
	}
	return result, nil
}

func (d *fakeInvestmentProjectReader) GetInvestmentProject(ctx context.Context, id string) (domain.InvestmentProjectProgress, error) {
	d.calls++
	if d.err != nil {
		return domain.InvestmentProjectProgress{}, d.err
	}
	for _, one := range d.byTenant[tenant.MustFrom(ctx)] {
		if one.InvestmentProject.ID == id {
			return one, nil
		}
	}
	// The store answers the same way for "not here" and "belongs to another commune", because it
	// cannot tell them apart — the query never reaches the other commune's rows.
	return domain.InvestmentProjectProgress{}, fistore.ErrInvestmentProjectNotFound
}

// sampleInvestmentProjects gives commune A two projects of 2026 and one of 2025, and commune B one project whose
// id COLLIDES with one of A's.
//
// THE COLLIDING ID IS THE POINT: it is the only fixture shape that can show a detail route reading
// across communes, and ids do collide in the field — two communes onboarded from the same import
// produce the same sequence.
func sampleInvestmentProjects() *fakeInvestmentProjectReader {
	deadline := time.Date(2026, time.December, 31, 0, 0, 0, 0, time.UTC)
	return &fakeInvestmentProjectReader{byTenant: map[tenant.ID][]domain.InvestmentProjectProgress{
		tenantA: {
			{
				// §8's worked project: 100 triệu planned, 90 triệu disbursed -> 90%, ahead of the
				// calendar at 70,96% elapsed, so NOT delayed.
				InvestmentProject: domain.InvestmentProject{ID: "da-001", Code: "DA-2026-be-tong-hoa-duong-ngo-xo-2", Year: 2026,
					CategoryID: "hm-001", Name: "Bê tông hoá đường ngõ xóm tổ 6",
					PlannedAmount: 100_000_000, DisbursementDeadline: deadline},
				DisbursedAmount: 90_000_000,
			},
			{
				// Nothing disbursed: delay score is exactly the elapsed share of the year, 7096.
				InvestmentProject: domain.InvestmentProject{ID: "da-002", Code: "DA-2026-nong-thon-moi", Year: 2026,
					CategoryID: "hm-002", Name: "Kế hoạch vốn nông thôn mới",
					PlannedAmount: 25_000_000_000, DisbursementDeadline: deadline},
				DisbursedAmount: 0,
			},
			{
				// A DIFFERENT budget year. It must never appear in a 2026 list (§13 rule 8).
				InvestmentProject: domain.InvestmentProject{ID: "da-2025", Code: "DA-2025-cu", Year: 2025,
					CategoryID: "hm-001", Name: "Dự án năm cũ",
					PlannedAmount: 50_000_000, DisbursementDeadline: deadline},
				DisbursedAmount: 50_000_000,
			},
		},
		tenantB: {
			{
				InvestmentProject: domain.InvestmentProject{ID: "da-001", Code: "DA-2026-CUA-XA-B", Year: 2026,
					CategoryID: "hm-b-001", Name: "Dự án của XÃ B",
					PlannedAmount: 9_000_000_000, DisbursementDeadline: deadline},
				DisbursedAmount: 1_000_000,
			},
		},
	}}
}

const investmentProjectListPath = "/api/v1/investment-projects?year=2026"

// --- rule 5, invariant 7: the four cases, on the list route -----------------------------------

func TestListInvestmentProjectsWithoutSessionIs401(t *testing.T) {
	m := newTestServerWith(t, withPermission("budget.read"))
	w := m.call(t, http.MethodGet, hostA, investmentProjectListPath, nil)
	wantStatus(t, w, http.StatusUnauthorized)
	if m.investmentProjects.calls != 0 {
		t.Fatal("không có phiên mà vẫn chạm kho — rào quyền phải chặn TRƯỚC")
	}
}

func TestListInvestmentProjectsWithWrongPermissionIs403(t *testing.T) {
	// An account that can sign in and holds a different budget key. `budget.update` is a real key
	// and deliberately NOT the one the route asks for: rule 5 invariant 3b says these rights are
	// not a Cartesian product, so holding one budget permission grants nothing about another.
	m := newTestServerWith(t, withPermission("budget.update"))
	w := m.call(t, http.MethodGet, hostA, investmentProjectListPath, staffOf(tenantA))
	wantStatus(t, w, http.StatusForbidden)
	if m.investmentProjects.calls != 0 {
		t.Fatal("sai quyền mà vẫn chạm kho — rào quyền phải chặn TRƯỚC")
	}
	if errorBody(t, w).Code != "forbidden" {
		t.Fatalf("mã lỗi = %q, muốn forbidden", errorBody(t, w).Code)
	}
}

func TestListInvestmentProjectsRightPermissionWrongCommuneIsREFUSED(t *testing.T) {
	// THE CASE A SINGLE-COMMUNE TEST CANNOT PRODUCE: a principal issued for commune B, holding the
	// right permission, arriving at commune A's host. Refused before the checker is consulted, and
	// before the store is touched.
	m := newTestServerWith(t, withPermission("budget.read"))
	w := m.call(t, http.MethodGet, hostA, investmentProjectListPath, staffOf(tenantB))
	wantStatus(t, w, http.StatusUnauthorized)
	if m.investmentProjects.calls != 0 {
		t.Fatal("token của xã khác mà vẫn chạm kho — leo thang quyền chéo xã")
	}
}

func TestListInvestmentProjectsRightPermissionRightCommuneIs200(t *testing.T) {
	m := newTestServerWith(t, withPermission("budget.read"))
	w := m.call(t, http.MethodGet, hostA, investmentProjectListPath, staffOf(tenantA))
	wantStatus(t, w, http.StatusOK)

	var result danhSachDuAnRa
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("thân không phải JSON: %v", err)
	}
	// Two projects of 2026 — the 2025 one must not be here (§13 rule 8).
	if len(result.Items) != 2 {
		t.Fatalf("số dự án = %d, muốn 2 (dự án năm 2025 KHÔNG được lẫn vào)", len(result.Items))
	}
	if result.Year != 2026 {
		t.Fatalf("year = %d, muốn 2026", result.Year)
	}
	if result.DelayThreshold != int64(domain.DefaultDelayThreshold) {
		t.Fatalf("ngưỡng = %d, muốn %d — phản hồi phải nói NÓ đã dùng ngưỡng nào",
			result.DelayThreshold, int64(domain.DefaultDelayThreshold))
	}
}

// --- rule 5, invariant 7: the four cases, on the detail route ---------------------------------

func TestGetInvestmentProjectFourPermissionCases(t *testing.T) {
	path := "/api/v1/investment-projects/da-001"

	t.Run("401 không phiên", func(t *testing.T) {
		m := newTestServerWith(t, withPermission("budget.read"))
		wantStatus(t, m.call(t, http.MethodGet, hostA, path, nil), http.StatusUnauthorized)
		if m.investmentProjects.calls != 0 {
			t.Fatal("chạm kho khi chưa có phiên")
		}
	})
	t.Run("403 sai quyền", func(t *testing.T) {
		m := newTestServerWith(t, withPermission("budget.confirm"))
		wantStatus(t, m.call(t, http.MethodGet, hostA, path, staffOf(tenantA)), http.StatusForbidden)
		if m.investmentProjects.calls != 0 {
			t.Fatal("chạm kho khi sai quyền")
		}
	})
	t.Run("từ chối khi đúng quyền nhưng sai xã", func(t *testing.T) {
		m := newTestServerWith(t, withPermission("budget.read"))
		wantStatus(t, m.call(t, http.MethodGet, hostA, path, staffOf(tenantB)), http.StatusUnauthorized)
		if m.investmentProjects.calls != 0 {
			t.Fatal("chạm kho với token của xã khác")
		}
	})
	t.Run("200 đúng quyền đúng xã", func(t *testing.T) {
		m := newTestServerWith(t, withPermission("budget.read"))
		w := m.call(t, http.MethodGet, hostA, path, staffOf(tenantA))
		wantStatus(t, w, http.StatusOK)
		var result duAnRa
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("thân không phải JSON: %v", err)
		}
		if result.Code != "DA-2026-be-tong-hoa-duong-ngo-xo-2" {
			t.Fatalf("mã dự án = %q", result.Code)
		}
	})
}

// --- isolation --------------------------------------------------------------------------------

func TestGetInvestmentProjectNeverReachesOTHERCommuneEvenWithSameID(t *testing.T) {
	// Commune B holds a project with the SAME id. A caller in commune B must get B's project, not
	// A's — and this is the only fixture shape that can show it.
	m := newTestServerWith(t, withPermission("budget.read"))
	w := m.call(t, http.MethodGet, hostB, "/api/v1/investment-projects/da-001", staffOf(tenantB))
	wantStatus(t, w, http.StatusOK)

	var result duAnRa
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("thân không phải JSON: %v", err)
	}
	if result.Code != "DA-2026-CUA-XA-B" {
		t.Fatalf("mã dự án = %q — đọc sang dữ liệu của xã khác là rò rỉ giữa hai cơ quan", result.Code)
	}
}

func TestListInvestmentProjectsReturnsOnlyHostCommunesProjects(t *testing.T) {
	m := newTestServerWith(t, withPermission("budget.read"))
	w := m.call(t, http.MethodGet, hostB, investmentProjectListPath, staffOf(tenantB))
	wantStatus(t, w, http.StatusOK)

	var result danhSachDuAnRa
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("thân không phải JSON: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].Code != "DA-2026-CUA-XA-B" {
		t.Fatalf("xã B nhận được %d dự án: %+v", len(result.Items), result.Items)
	}
}

// --- the derived figures ----------------------------------------------------------------------

func TestListInvestmentProjectsDerivesDelayScoreNotStored(t *testing.T) {
	m := newTestServerWith(t, withPermission("budget.read"))
	w := m.call(t, http.MethodGet, hostA, investmentProjectListPath, staffOf(tenantA))
	wantStatus(t, w, http.StatusOK)

	var result danhSachDuAnRa
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("thân không phải JSON: %v", err)
	}

	byCode := map[string]duAnRa{}
	for _, one := range result.Items {
		byCode[one.Code] = one
	}

	// 90% disbursed against 70,96% of the year elapsed: ahead, so delay score is negative and the
	// project is NOT flagged. The figure comes from the clock, not from any stored column.
	before := byCode["DA-2026-be-tong-hoa-duong-ngo-xo-2"]
	if before.DisbursedRatio == nil || *before.DisbursedRatio != 9000 {
		t.Fatalf("tỷ lệ = %v, muốn 9000", before.DisbursedRatio)
	}
	if before.DelayScore == nil || *before.DelayScore != 7096-9000 {
		t.Fatalf("điểm chậm = %v, muốn %d", before.DelayScore, 7096-9000)
	}
	if before.IsDelayed {
		t.Fatal("dự án đi trước lịch bị đánh dấu chậm")
	}
	if before.RemainingAmount != 10_000_000 {
		t.Fatalf("còn lại = %d, muốn 10000000 (§8)", before.RemainingAmount)
	}
	// §9: a blank approved total reads as this year's plan.
	if before.ApprovedAmount != 100_000_000 {
		t.Fatalf("tổng mức được duyệt = %d, muốn 100000000", before.ApprovedAmount)
	}

	// Nothing disbursed: the delay score is exactly the elapsed share of the year (§3).
	delayed := byCode["DA-2026-nong-thon-moi"]
	if delayed.DelayScore == nil || *delayed.DelayScore != 7096 {
		t.Fatalf("điểm chậm = %v, muốn 7096", delayed.DelayScore)
	}
	if !delayed.IsDelayed {
		t.Fatal("chậm 70,96 điểm mà không bị đánh dấu chậm")
	}
}

// --- the budget year --------------------------------------------------------------------------

func TestListInvestmentProjectsMissingYearIs400NotDefault(t *testing.T) {
	// A default would report another year's money under this year's heading, with every figure on
	// the page internally consistent and wrong (§13 rule 8).
	for _, path := range []string{
		"/api/v1/investment-projects",
		"/api/v1/investment-projects?year=",
		"/api/v1/investment-projects?year=khong-phai-so",
		"/api/v1/investment-projects?year=12",
	} {
		t.Run(path, func(t *testing.T) {
			m := newTestServerWith(t, withPermission("budget.read"))
			w := m.call(t, http.MethodGet, hostA, path, staffOf(tenantA))
			wantStatus(t, w, http.StatusBadRequest)
			if m.investmentProjects.calls != 0 {
				t.Fatal("năm sai mà vẫn chạy truy vấn — phải từ chối TRƯỚC khi chạm kho")
			}
		})
	}
}

// --- failures ---------------------------------------------------------------------------------

func TestListInvestmentProjectsPastCeilingIs500NotTruncated(t *testing.T) {
	m := newTestServerWith(t, withPermission("budget.read"))
	m.investmentProjects.err = fistore.ErrTooManyInvestmentProjects
	w := m.call(t, http.MethodGet, hostA, investmentProjectListPath, staffOf(tenantA))
	wantStatus(t, w, http.StatusInternalServerError)
}

func TestGetMissingInvestmentProjectIs404(t *testing.T) {
	m := newTestServerWith(t, withPermission("budget.read"))
	w := m.call(t, http.MethodGet, hostA, "/api/v1/investment-projects/khong-co", staffOf(tenantA))
	wantStatus(t, w, http.StatusNotFound)
}

func TestGetInvestmentProjectStoreErrorIs500AndDoesNotLeak(t *testing.T) {
	m := newTestServerWith(t, withPermission("budget.read"))
	m.investmentProjects.err = errors.New("chi tiết kết nối kho: dsn=postgres://nguoi:matkhau@may-chu")
	w := m.call(t, http.MethodGet, hostA, "/api/v1/investment-projects/da-001", staffOf(tenantA))
	wantStatus(t, w, http.StatusInternalServerError)
	if body := w.Body.String(); len(body) > 0 && (contains(body, "matkhau") || contains(body, "dsn")) {
		t.Fatalf("chi tiết lỗi hệ thống lọt ra cho client: %s", body)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// Register must refuse incomplete wiring at construction, not at request time: a route mounted
// without its store would accept requests it cannot honour, and the first person to find out would
// be a member of staff in front of a government screen.
func TestRegisterRefusesWithoutInvestmentProjectStore(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("thiếu kho dự án mà Register vẫn gắn tuyến")
		}
	}()
	Register(http.NewServeMux(), Deps{Checker: noPermissions(), CapitalPlanCategories: sampleCategories()})
}

func TestRegisterRefusesWithoutChecker(t *testing.T) {
	// A nil Checker would make authz.RequirePermission meet a nil interface at request time —
	// the worst possible moment to find out.
	defer func() {
		if recover() == nil {
			t.Fatal("thiếu Checker mà Register vẫn gắn tuyến khai budget.read")
		}
	}()
	Register(http.NewServeMux(), Deps{CapitalPlanCategories: sampleCategories(), InvestmentProjects: sampleInvestmentProjects()})
}

var _ authz.Checker = fakeChecker{}

// --- budget.scope_notice travels with the figures (rule 1, invariant 10) ------------------------

func TestInvestmentProjectReadsCarryTheCommunesScopeNotice(t *testing.T) {
	// THE COMMUNE'S OWN WORDING, not the constant: before this the web hardcoded the banner, so a
	// commune that reworded it under Cấu hình → Lời hệ thống kept reading the vendor's sentence.
	const own = "Số liệu trên màn hình này chỉ để theo dõi, không dùng thay sổ kế toán của xã."
	for _, tc := range []struct{ name, path string }{
		{"list", "/api/v1/investment-projects?year=2026"},
		{"detail", "/api/v1/investment-projects/da-001"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestServerWith(t, withPermission("budget.read"))
			fake := m.d.SystemMessages.(*systemMessagesFake)
			fake.textOut = own
			w := m.call(t, http.MethodGet, hostA, tc.path, staffOf(tenantA))
			wantStatus(t, w, http.StatusOK)

			var got struct {
				ScopeNotice string `json:"scope_notice"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.ScopeNotice != own {
				t.Errorf("scope_notice = %q, want the commune's own wording", got.ScopeNotice)
			}
			// Read under the request's commune, for the one key this service ships.
			if fake.textKey != domain.KeyBudgetScopeNotice || fake.textCommune != tenantA {
				t.Errorf("Text(%q) in commune %q, want %q in %q", fake.textKey, fake.textCommune,
					domain.KeyBudgetScopeNotice, tenantA)
			}
		})
	}
}

func TestInvestmentProjectListCarriesScopeNoticeOnceNotPerItem(t *testing.T) {
	m := newTestServerWith(t, withPermission("budget.read"))
	w := m.call(t, http.MethodGet, hostA, "/api/v1/investment-projects?year=2026", staffOf(tenantA))
	wantStatus(t, w, http.StatusOK)
	var result danhSachDuAnRa
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	m0, _ := domain.LookupShippedMessage(domain.KeyBudgetScopeNotice)
	if result.ScopeNotice != m0.DefaultText {
		t.Errorf("top-level scope_notice = %q, want the shipped default for a commune that reworded nothing", result.ScopeNotice)
	}
	if len(result.Items) == 0 {
		t.Fatal("fixture has no project in 2026 — the per-item half of this test asserts nothing")
	}
	for _, it := range result.Items {
		if it.ScopeNotice != "" {
			t.Fatalf("item %s repeats scope_notice", it.ID)
		}
	}
}

func TestInvestmentProjectReadsRefuseWhenScopeNoticeCannotBeRead(t *testing.T) {
	// FAIL CLOSED: a commune that replaced the sentence must never see the vendor's because the read
	// failed. 500, and the store's words stay in the log.
	for _, path := range []string{"/api/v1/investment-projects?year=2026", "/api/v1/investment-projects/da-001"} {
		m := newTestServerWith(t, withPermission("budget.read"))
		m.d.SystemMessages.(*systemMessagesFake).textErr = errors.New("cơ sở dữ liệu không phản hồi")
		w := m.call(t, http.MethodGet, hostA, path, staffOf(tenantA))
		wantStatus(t, w, http.StatusInternalServerError)
		body := w.Body.String()
		m0, _ := domain.LookupShippedMessage(domain.KeyBudgetScopeNotice)
		if strings.Contains(body, "không phản hồi") || strings.Contains(body, m0.DefaultText) {
			t.Errorf("%s: body leaks the failure or falls back to the default: %s", path, body)
		}
	}
}
