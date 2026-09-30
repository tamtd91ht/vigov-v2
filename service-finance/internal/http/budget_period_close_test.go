package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// WHAT THIS FILE IS FOR: the three budget-period-close routes beyond the four permission cases
// (those run for them in thu_chi_ngan_sach_test.go, tamTuyenNganSach), and the period guard's answer
// on the entry routes:
//
//  1. 409 on a double close and on a second reopen, each with its own code and a sentence naming the
//     close — never the commune id the use case wraps around it;
//  2. 400 on a missing / over-long reason and on a month outside 1..12, before any use case runs
//     for what the handler itself can see;
//  3. the list is the commune's own, and a year close reads as `month: null`, `scope: "year"`;
//  4. a closed period answers 409 `budget_period_closed` on the entry routes;
//  5. `adjustment_reason` reaches the use case and comes back on the entry list.

func errorBody(t *testing.T, m *mayChuNganSach, method, path, body string, want int) httpx.Error {
	t.Helper()
	w := m.goi(t, method, hostA, path, canBoGhi(xaA), body)
	doiMa(t, w, want)
	var e httpx.Error
	decodeAny(t, w, &e)
	return e
}

// decodeAny decodes a body whatever its status (docJSON insists on 200).
func decodeAny(t *testing.T, w *httptest.ResponseRecorder, out any) {
	t.Helper()
	if err := json.Unmarshal(w.Body.Bytes(), out); err != nil {
		t.Fatalf("thân không phải JSON: %v — %s", err, w.Body.String())
	}
}

func closeOfSeptember() domain.BudgetPeriodClose {
	return domain.BudgetPeriodClose{
		ID: "01JCHOTKY00000000000000000", Code: "CK-2026-09-01", Year: 2026, Month: 9, Revision: 1,
		ClosedBy: maCanBoGhi, ClosedAt: time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC),
	}
}

func TestCloseTwiceIs409NamingTheExistingClose(t *testing.T) {
	m := dungMayChuNganSach(t)
	m.capQuyen(xaA, "budget.confirm")
	// Wrapped the way bocNganSach wraps it: the commune id must NOT reach the client.
	m.ghi.loi = fmt.Errorf("ngan_sach: chốt kỳ cho xã %s: %w", xaA, domain.AlreadyClosedError(closeOfSeptember()))

	e := errorBody(t, m, http.MethodPost, pathPeriodCloses, `{"year":2026,"month":9}`, http.StatusConflict)
	if e.Code != "budget_period_already_closed" {
		t.Fatalf("code = %q, muốn budget_period_already_closed", e.Code)
	}
	if !strings.Contains(e.Message, "CK-2026-09-01") || !strings.Contains(e.Message, "tháng 09/2026") {
		t.Fatalf("câu 409 không nêu lần chốt đang có: %q", e.Message)
	}
	if strings.Contains(e.Message, string(xaA)) {
		t.Fatalf("câu trả cho client mang mã xã: %q", e.Message)
	}
}

func TestReopenTwiceIs409(t *testing.T) {
	m := dungMayChuNganSach(t)
	m.capQuyen(xaA, "budget.confirm")
	c := closeOfSeptember()
	c.ReopenedAt, c.ReopenedBy, c.ReopenReason = time.Now(), maCanBoGhi, "lần trước"
	m.ghi.loi = domain.AlreadyReopenedError(c)

	e := errorBody(t, m, http.MethodPost, pathPeriodCloses+"/CK-2026-09-01/reopening",
		`{"reason":"mở lần hai"}`, http.StatusConflict)
	if e.Code != "budget_period_close_reopened" || !strings.Contains(e.Message, "CK-2026-09-01") {
		t.Fatalf("= %q %q, muốn budget_period_close_reopened nêu mã lần chốt", e.Code, e.Message)
	}
}

func TestReopenUnknownCodeIs404(t *testing.T) {
	m := dungMayChuNganSach(t)
	m.capQuyen(xaA, "budget.confirm")
	m.ghi.loi = domain.ErrBudgetPeriodCloseNotFound
	e := errorBody(t, m, http.MethodPost, pathPeriodCloses+"/CK-2099-01-01/reopening",
		`{"reason":"x"}`, http.StatusNotFound)
	if e.Code != "not_found" {
		t.Fatalf("code = %q", e.Code)
	}
}

func TestReopenReasonMissingOrTooLongIs400(t *testing.T) {
	for ten, loi := range map[string]error{
		"thiếu lý do":   domain.ErrReopenReasonMissing,
		"lý do quá dài": fmt.Errorf("%w (tối đa 500 ký tự)", domain.ErrReopenReasonTooLong),
	} {
		t.Run(ten, func(t *testing.T) {
			m := dungMayChuNganSach(t)
			m.capQuyen(xaA, "budget.confirm")
			m.ghi.loi = loi
			e := errorBody(t, m, http.MethodPost, pathPeriodCloses+"/CK-2026-09-01/reopening",
				`{"reason":"`+strings.Repeat("ệ", 501)+`"}`, http.StatusBadRequest)
			if e.Code != "invalid_request" {
				t.Fatalf("code = %q", e.Code)
			}
		})
	}
}

func TestCloseMonthOutsideRangeIs400BeforeTheUseCase(t *testing.T) {
	for _, body := range []string{`{"year":2026,"month":0}`, `{"year":2026,"month":13}`} {
		m := dungMayChuNganSach(t)
		m.capQuyen(xaA, "budget.confirm")
		errorBody(t, m, http.MethodPost, pathPeriodCloses, body, http.StatusBadRequest)
		if m.ghi.closeCalls != 0 {
			t.Fatalf("%s: use case chạy dù tháng sai", body)
		}
	}
}

func TestCloseWithoutMonthIsTheWholeYear(t *testing.T) {
	m := dungMayChuNganSach(t)
	m.capQuyen(xaA, "budget.confirm")
	m.ghi.raClose = domain.BudgetPeriodClose{ID: "x", Code: "CK-2026-CN-01", Year: 2026, Revision: 1, ClosedBy: maCanBoGhi}

	w := m.goi(t, http.MethodPost, hostA, pathPeriodCloses, canBoGhi(xaA), `{"year":2026}`)
	doiMa(t, w, http.StatusCreated)
	if m.ghi.lastClose.Year != 2026 || m.ghi.lastClose.Month != 0 {
		t.Fatalf("use case nhận %+v, muốn năm 2026 tháng 0 (cả năm)", m.ghi.lastClose)
	}
	var out budgetPeriodCloseOut
	decodeAny(t, w, &out)
	if out.Month != nil || out.Scope != "year" || out.Code != "CK-2026-CN-01" || !out.Active {
		t.Fatalf("201 = %+v, muốn month null, scope year, active", out)
	}
}

func TestListYearIsRequired(t *testing.T) {
	for _, q := range []string{"", "?year=", "?year=abc", "?year=1999"} {
		m := dungMayChuNganSach(t)
		m.capQuyen(xaA, "budget.read")
		errorBody(t, m, http.MethodGet, pathPeriodCloses+q, "", http.StatusBadRequest)
		if m.doc.goi != 0 {
			t.Fatalf("%q: kho chạy dù thiếu năm", q)
		}
	}
}

func TestListIsTheCommunesOwnHistory(t *testing.T) {
	m := dungMayChuNganSach(t)
	m.capQuyen(xaA, "budget.read")
	m.capQuyen(xaB, "budget.read")
	reopened := closeOfSeptember()
	reopened.ReopenedAt = time.Date(2026, 10, 2, 8, 0, 0, 0, time.UTC)
	reopened.ReopenedBy, reopened.ReopenReason = "CB-00003", "Ghi sót đợt thu phí chợ"
	again := closeOfSeptember()
	again.ID, again.Code, again.Revision = "01JCHOTKY2", "CK-2026-09-02", 2
	year := domain.BudgetPeriodClose{ID: "01JCHOTKYNAM", Code: "CK-2026-CN-01", Year: 2026, Revision: 1, ClosedBy: maCanBoGhi}
	m.doc.closes = map[tenant.ID]map[int][]domain.BudgetPeriodClose{
		xaA: {2026: {year, reopened, again}},
		xaB: {2026: {}},
	}

	var a, b budgetPeriodClosesOut
	docJSON(t, m.goi(t, http.MethodGet, hostA, pathPeriodCloses+"?year=2026", canBoGhi(xaA), ""), &a)
	docJSON(t, m.goi(t, http.MethodGet, hostB, pathPeriodCloses+"?year=2026", canBoGhi(xaB), ""), &b)

	if len(a.Closes) != 3 || len(b.Closes) != 0 {
		t.Fatalf("xã A %d lần chốt, xã B %d — muốn 3 và 0", len(a.Closes), len(b.Closes))
	}
	if a.Closes[0].Month != nil || a.Closes[0].Scope != "year" {
		t.Fatalf("chốt cả năm phải là month null / scope year: %+v", a.Closes[0])
	}
	if a.Closes[1].Active || a.Closes[1].ReopenReason == "" || a.Closes[1].ReopenedBy != "CB-00003" {
		t.Fatalf("lần chốt đã mở phải hiện đã mở, ai mở, lý do: %+v", a.Closes[1])
	}
	if !a.Closes[2].Active || a.Closes[2].Revision != 2 || *a.Closes[2].Month != 9 {
		t.Fatalf("lần chốt lại: %+v", a.Closes[2])
	}
}

func TestEntryInClosedPeriodIs409NamingTheClose(t *testing.T) {
	m := dungMayChuNganSach(t)
	m.capQuyen(xaA, "budget.update", "budget.confirm")
	m.ghi.loi = fmt.Errorf("ngan_sach: ghi đợt cho xã %s: %w", xaA, domain.EntryPeriodClosedError(closeOfSeptember()))

	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPost, duongDong + "/k-a/entries", thanGhiDot},
		{http.MethodDelete, duongDot + "/01JDOTMOINHAT0000000000000", `{"reason":"ghi nhầm"}`},
	} {
		e := errorBody(t, m, tc.method, tc.path, tc.body, http.StatusConflict)
		if e.Code != "budget_period_closed" || !strings.Contains(e.Message, "CK-2026-09-01") {
			t.Fatalf("%s %s = %q %q, muốn budget_period_closed nêu mã lần chốt", tc.method, tc.path, e.Code, e.Message)
		}
		if strings.Contains(e.Message, donViMauHTTP) || strings.Contains(e.Message, string(xaA)) {
			t.Fatalf("câu 409 mang dữ liệu không được lộ: %q", e.Message)
		}
	}
}

func TestSheetWriteInClosedYearIs409(t *testing.T) {
	m := dungMayChuNganSach(t)
	m.capQuyen(xaA, "budget.update")
	year := domain.BudgetPeriodClose{Code: "CK-2026-CN-01", Year: 2026, Revision: 1}
	m.ghi.loi = domain.SheetYearClosedError(year)
	e := errorBody(t, m, http.MethodPatch, duongDong+"/"+idDongChi, `{"values":{"c-dt":1}}`, http.StatusConflict)
	if e.Code != "budget_period_closed" || !strings.Contains(e.Message, "CK-2026-CN-01") {
		t.Fatalf("= %q %q", e.Code, e.Message)
	}
}

func TestAdjustmentReasonReachesTheUseCaseAndComesBack(t *testing.T) {
	m := dungMayChuNganSach(t)
	m.capQuyen(xaA, "budget.read", "budget.update")

	body := strings.Replace(thanGhiDot, `"values"`, `"adjustment_reason":"Bù đợt ghi sót tháng 9","values"`, 1)
	doiMa(t, m.goi(t, http.MethodPost, hostA, duongDong+"/k-a/entries", canBoGhi(xaA), body), http.StatusCreated)
	if r := m.ghi.ghiDotCuoi.AdjustmentReason; r == nil || *r != "Bù đợt ghi sót tháng 9" {
		t.Fatalf("adjustment_reason tới use case = %v", r)
	}
	// Absent means an ordinary entry — nil, never "". A fresh harness: the harness sends one fixed
	// Idempotency-Key, so a second POST on the same one would be a replay, not a call.
	m2 := dungMayChuNganSach(t)
	m2.capQuyen(xaA, "budget.update")
	doiMa(t, m2.goi(t, http.MethodPost, hostA, duongDong+"/k-a/entries", canBoGhi(xaA), thanGhiDot), http.StatusCreated)
	if m2.ghi.ghiDotGoi != 1 || m2.ghi.ghiDotCuoi.AdjustmentReason != nil {
		t.Fatal("đợt thường mà use case nhận adjustment_reason")
	}

	ds := m.doc.dot[xaA]["k-a"]
	ds.Dot[1].AdjustmentReason = "Bù đợt ghi sót tháng 6"
	m.doc.dot[xaA]["k-a"] = ds
	var out danhSachDotRa
	docJSON(t, m.goi(t, http.MethodGet, hostA, duongDong+"/k-a/entries", canBoGhi(xaA), ""), &out)
	if out.Entries[0].AdjustmentReason != "" || out.Entries[1].AdjustmentReason != "Bù đợt ghi sót tháng 6" {
		t.Fatalf("adjustment_reason trên danh sách = %q / %q",
			out.Entries[0].AdjustmentReason, out.Entries[1].AdjustmentReason)
	}
}
