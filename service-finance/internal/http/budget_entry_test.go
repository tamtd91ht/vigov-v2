package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-finance/internal/domain"
)

// WHAT THIS FILE IS FOR: the three batch routes and `method` on PATCH /api/v1/budget-lines/{id}, at
// the surface. The four rule-5 cases for all three routes run in budget_test.go, where the
// routes are rows of eightBudgetWriteRoutes; this file holds what is specific to the batches:
//
//  1. `budget.update` alone cannot remove a batch — removal is `budget.confirm`;
//  2. the list's wire shape: every number column a key, `null` for empty, newest first, `method`;
//  3. refusals reach the client with the right status and never quote the counterparty;
//  4. the counterparty never reaches a log line, even on a 500;
//  5. `method` on PATCH: `manual` / `entries` pass, `children` is 400 before the use case.

func TestEntry_BudgetUpdateCannotRemoveEntry(t *testing.T) {
	// The accountant who records batches holds `budget.read` + `budget.update`. Taking a batch back
	// changes an `entries` line's figure, possibly already read off a screen — `budget.confirm`.
	m := newBudgetServer(t)
	m.grant(tenantA, "budget.read", "budget.update")

	wantStatus(t, m.call(t, http.MethodDelete, hostA, entryPath+"/01JDOTMOINHAT0000000000000", writerPrincipal(tenantA),
		`{"reason":"ghi nhầm"}`), http.StatusForbidden)
	if m.writer.removeEntryCalls != 0 {
		t.Fatal("tài khoản chỉ có budget.update mà vẫn gỡ được đợt")
	}
	// ...while the same account CAN record one.
	wantStatus(t, m.call(t, http.MethodPost, hostA, linePath+"/k-a/entries", writerPrincipal(tenantA), recordEntryBody),
		http.StatusCreated)
}

func TestEntry_ListHasRightShape(t *testing.T) {
	m := newBudgetServer(t)
	m.grant(tenantA, "budget.read")

	var result danhSachDotRa
	decodeJSON(t, m.call(t, http.MethodGet, hostA, linePath+"/k-a/entries", writerPrincipal(tenantA), ""), &result)

	if result.LineID != "k-a" || result.Method != "entries" {
		t.Fatalf("line_id=%q method=%q", result.LineID, result.Method)
	}
	if len(result.Entries) != 2 || result.Entries[0].ID != "01JDOTMOINHAT0000000000000" {
		t.Fatalf("thứ tự đợt sai (mới nhất trước): %+v", result.Entries)
	}
	next, old := result.Entries[0], result.Entries[1]
	if next.Date != "2026-08-20" || next.DocumentNo != "PC-0102" {
		t.Fatalf("đợt mới nhất: %+v", next)
	}
	// 0 IS A STATED AMOUNT; an absent amount is `null`; the % column is not a key at all.
	if v := next.Values["c-dt"]; v == nil || *v != 0 {
		t.Fatalf("c-dt của đợt mới = %v, muốn 0", v)
	}
	if v, ok := old.Values["c-dt"]; !ok || v != nil {
		t.Fatalf("c-dt của đợt cũ = %v (có khoá: %v), muốn null", v, ok)
	}
	if v := old.Values["c-chi"]; v == nil || *v != -150_000 {
		t.Fatalf("số âm bị đổi: %v", v)
	}
	if _, ok := next.Values["c-ty"]; ok {
		t.Fatal("cột phần trăm có ô số tiền")
	}
	if old.Counterparty != "" {
		t.Fatalf("đợt không ghi đơn vị mà trả %q", old.Counterparty)
	}
}

// maskedCounterpartyHTTP is privacy.MaskName(sampleCounterpartyHTTP), written out as a LITERAL: computing it with the
// helper here would pass even if the handler called nothing, or called the helper on the wrong field.
const maskedCounterpartyHTTP = "Hộ b. T. T. M."

func TestEntry_ListMasksCounterparty(t *testing.T) {
	// Rule 3, invariant 3: "Đơn vị, cá nhân" may be a citizen's name and no key grants the full value.
	// Checked on the RAW BODY as well as the decoded struct: a second field carrying the raw text
	// would pass a struct-only assertion.
	m := newBudgetServer(t)
	m.grant(tenantA, "budget.read")

	w := m.call(t, http.MethodGet, hostA, linePath+"/k-a/entries", writerPrincipal(tenantA), "")
	var result danhSachDotRa
	decodeJSON(t, w, &result)
	if result.Entries[0].Counterparty != maskedCounterpartyHTTP {
		t.Fatalf("counterparty = %q, muốn %q", result.Entries[0].Counterparty, maskedCounterpartyHTTP)
	}
	if strings.Contains(w.Body.String(), sampleCounterpartyHTTP) || strings.Contains(w.Body.String(), "Trần Thị") {
		t.Fatalf("thân trả về chứa nguyên văn `đơn vị, cá nhân`: %s", w.Body.String())
	}
}

func TestEntry_RecordResponseMasksCounterparty(t *testing.T) {
	// The 201 of a create carries the batch back — the same rule holds there, even though the caller
	// has just typed the value: a reply is stored by proxies, clients and browser caches alike.
	m := newBudgetServer(t)
	m.grant(tenantA, "budget.update")
	m.writer.resultEntry = tenantALineEntries().Entries[0] // Counterparty = sampleCounterpartyHTTP, raw, as the use case returns it

	w := m.call(t, http.MethodPost, hostA, linePath+"/k-a/entries", writerPrincipal(tenantA), recordEntryBody)
	wantStatus(t, w, http.StatusCreated)
	var result dotRa
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("thân không phải JSON: %v", err)
	}
	if result.Counterparty != maskedCounterpartyHTTP {
		t.Fatalf("counterparty = %q, muốn %q", result.Counterparty, maskedCounterpartyHTTP)
	}
	if strings.Contains(w.Body.String(), sampleCounterpartyHTTP) || strings.Contains(w.Body.String(), "Trần Thị") {
		t.Fatalf("thân 201 chứa nguyên văn `đơn vị, cá nhân`: %s", w.Body.String())
	}
}

func TestEntry_ListOfEachCommuneIsThatCommunes(t *testing.T) {
	m := newBudgetServer(t)
	m.grant(tenantA, "budget.read")
	m.grant(tenantB, "budget.read")

	var a, b danhSachDotRa
	decodeJSON(t, m.call(t, http.MethodGet, hostA, linePath+"/k-a/entries", writerPrincipal(tenantA), ""), &a)
	decodeJSON(t, m.call(t, http.MethodGet, hostB, linePath+"/k-a/entries", writerPrincipal(tenantB), ""), &b)
	if len(a.Entries) != 2 || len(b.Entries) != 1 || !strings.Contains(b.Entries[0].Content, "BÌNH DƯƠNG") {
		t.Fatalf("đợt lẫn xã: A=%d B=%+v", len(a.Entries), b.Entries)
	}
}

func TestEntry_MissingLineIs404(t *testing.T) {
	m := newBudgetServer(t)
	m.grant(tenantA, "budget.read")
	wantStatus(t, m.call(t, http.MethodGet, hostA, linePath+"/khong-co/entries", writerPrincipal(tenantA), ""),
		http.StatusNotFound)
}

func TestEntry_RecordPassesRightFieldsToUseCase(t *testing.T) {
	m := newBudgetServer(t)
	m.grant(tenantA, "budget.update")

	wantStatus(t, m.call(t, http.MethodPost, hostA, linePath+"/k-a/entries", writerPrincipal(tenantA), recordEntryBody),
		http.StatusCreated)
	req := m.writer.lastRecordEntry
	if req.LineID != "k-a" || req.Content != "Chi hỗ trợ đợt 3" || req.Counterparty != sampleCounterpartyHTTP ||
		req.VoucherNo != "PC-0103" || req.Date.Format("2006-01-02") != "2026-08-20" {
		t.Fatalf("yêu cầu tới use case: %+v", req)
	}
	if g := req.Amounts["c-chi"]; g == nil || *g != 2_500_000 {
		t.Fatalf("c-chi = %v", g)
	}
	if g, ok := req.Amounts["c-dt"]; !ok || g != nil {
		t.Fatalf("c-dt null phải tới use case là nil có khoá: %v %v", g, ok)
	}
}

func TestEntry_MalformedDateIs400WithoutUseCase(t *testing.T) {
	m := newBudgetServer(t)
	m.grant(tenantA, "budget.update")
	for _, body := range []string{
		`{"date":"20/08/2026","content":"X","values":{"c-chi":1}}`,
		`not json`,
	} {
		wantStatus(t, m.call(t, http.MethodPost, hostA, linePath+"/k-a/entries", writerPrincipal(tenantA), body),
			http.StatusBadRequest)
	}
	if m.writer.recordEntryCalls != 0 {
		t.Fatalf("use case chạy %d lần với thân đã bị từ chối", m.writer.recordEntryCalls)
	}
}

func TestEntry_RefusalMapsToRightStatusWithoutCounterparty(t *testing.T) {
	// The use case's refusals, mapped. The app tests prove each refusal writes nothing; this proves
	// the status and that the sentence never quotes the counterparty (rule 3, forbidden #3).
	for _, tc := range []struct {
		failure error
		status  int
	}{
		{domain.ErrEntryLeafOnly, http.StatusConflict},
		{domain.ErrEntriesLineHasEntries, http.StatusConflict},
		{domain.ErrEntriesLineNoDirectValue, http.StatusConflict},
		{domain.ErrParentLineMethodFixed, http.StatusConflict},
		{domain.ErrColumnNotNumber, http.StatusBadRequest},
		{domain.ErrColumnInOtherSheet, http.StatusBadRequest},
		{domain.ErrEntryContentTooLong, http.StatusBadRequest},
		{domain.ErrEntryCounterpartyTooLong, http.StatusBadRequest},
		{domain.ErrEntryVoucherNoTooLong, http.StatusBadRequest},
		{domain.ErrEntryDateOutOfRange, http.StatusBadRequest},
		{domain.ErrEntryHasNoAmount, http.StatusBadRequest},
		{domain.ErrLineNotFound, http.StatusNotFound},
		// The batch ceiling at the write: the caller holds the right; the LINE is full.
		{domain.ErrLineEntriesFull, http.StatusConflict},
		// entries -> manual with an oversized sum: remove a batch, not resend the body. Wrapped with
		// ErrValueTooLarge (app.copyEntryTotalsIntoCells) and must still be 409, never the 400 of the typo guard.
		{domain.ErrEntryTotalOverflow, http.StatusConflict},
		{fmt.Errorf("%w: %w", domain.ErrEntryTotalOverflow, domain.ErrValueTooLarge), http.StatusConflict},
		// A typed amount past the new ceiling (2^53 − 1).
		{domain.ErrValueTooLarge, http.StatusBadRequest},
	} {
		t.Run(tc.failure.Error(), func(t *testing.T) {
			m := newBudgetServer(t)
			m.grant(tenantA, "budget.update")
			m.writer.err = fmt.Errorf("ngan_sach: ghi đợt cho xã x: %w", tc.failure)
			w := m.call(t, http.MethodPost, hostA, linePath+"/k-a/entries", writerPrincipal(tenantA), recordEntryBody)
			wantStatus(t, w, tc.status)
			if strings.Contains(w.Body.String(), sampleCounterpartyHTTP) {
				t.Fatal("thân lỗi chứa nguyên văn `đơn vị, cá nhân`")
			}
		})
	}
}

func TestEntry_RemoveRemovedOrOtherCommuneIs404(t *testing.T) {
	m := newBudgetServer(t)
	m.grant(tenantA, "budget.confirm")
	m.writer.err = fmt.Errorf("ngan_sach: gỡ đợt cho xã x: %w", domain.ErrEntryNotFound)
	wantStatus(t, m.call(t, http.MethodDelete, hostA, entryPath+"/01JDOTDAGO000000000000000", writerPrincipal(tenantA),
		`{"reason":"x"}`), http.StatusNotFound)
	if m.writer.lastID != "01JDOTDAGO000000000000000" {
		t.Fatalf("id tới use case = %q", m.writer.lastID)
	}
}

func TestEntry_SystemErrorLogsNoCounterparty(t *testing.T) {
	// A 500 logs the wrapped error. Nothing on that path may carry the counterparty — the log travels
	// into centralised logging across every commune (rule 3, invariant 1).
	m := newBudgetServer(t)
	m.grant(tenantA, "budget.update")
	m.writer.err = errors.New("store: begin: connection refused")

	wantStatus(t, m.call(t, http.MethodPost, hostA, linePath+"/k-a/entries", writerPrincipal(tenantA), recordEntryBody),
		http.StatusInternalServerError)
	if m.logBuf.Len() == 0 {
		t.Fatal("lỗi hệ thống mà không có dòng nhật ký nào — phép kiểm dưới đây sẽ xanh vô nghĩa")
	}
	if strings.Contains(m.logBuf.String(), sampleCounterpartyHTTP) || strings.Contains(m.logBuf.String(), "Trần Thị") {
		t.Fatalf("nhật ký chứa `đơn vị, cá nhân`: %s", m.logBuf.String())
	}
}

// --- `method` on PATCH /api/v1/budget-lines/{id} -------------------------------------------------------

func TestUpdateLine_MethodManualAndEntriesReachUseCase(t *testing.T) {
	for _, code := range []string{"manual", "entries"} {
		t.Run(code, func(t *testing.T) {
			m := newBudgetServer(t)
			m.grant(tenantA, "budget.update")
			wantStatus(t, m.call(t, http.MethodPatch, hostA, linePath+"/k-a", writerPrincipal(tenantA), `{"method":"`+code+`"}`),
				http.StatusOK)
			if method := m.writer.lastUpdate.Method; method == nil || string(*method) != code {
				t.Fatalf("cách tính tới use case = %v, muốn %q", method, code)
			}
		})
	}
}

func TestUpdateLine_MethodChildrenOrOtherIs400(t *testing.T) {
	m := newBudgetServer(t)
	m.grant(tenantA, "budget.update")
	for _, code := range []string{"children", "", "MANUAL"} {
		wantStatus(t, m.call(t, http.MethodPatch, hostA, linePath+"/k-a", writerPrincipal(tenantA), `{"method":"`+code+`"}`),
			http.StatusBadRequest)
	}
	if m.writer.updateCalls != 0 {
		t.Fatalf("use case chạy %d lần với `method` không hợp lệ", m.writer.updateCalls)
	}
	// POST still refuses `method` of any value: a new line is always a `manual` leaf.
	wantStatus(t, m.call(t, http.MethodPost, hostA, linePath, writerPrincipal(tenantA),
		`{"sheet_id":"`+expenditureSheetID+`","name":"X","order":1,"method":"entries"}`), http.StatusBadRequest)
}

func TestReadSheet_UncomputableCellIsNullWithReasonNotNumber(t *testing.T) {
	// The wire half of "a figure that cannot be computed comes back with a reason, never 0": the
	// entries leaf's batch sum did not fit int64. Its cell and its parent's cell are `null` WITH a key
	// in `unavailable_reasons`; an ordinary empty cell stays `null` with NO key; the summary card says
	// why instead of a blank; everything else on the sheet is a number as before. 200, not 500.
	m := newBudgetServer(t)
	m.grant(tenantA, "budget.read")

	b := tenantAExpenditureSheet()
	b.Lines[0].Method = domain.MethodChildren
	b.Lines[1].ParentID = expenditureHeadlineID
	b.Lines[1].Method = domain.MethodEntries
	b.EntryTotalOverflow = map[string]map[string]bool{"k-a": {"c-chi": true}}
	m.reader.byTenant[tenantA][sheetKey(2026, domain.SheetKindExpenditure)] = b

	w := m.call(t, http.MethodGet, hostA, sheetPath+"?year=2026&kind=chi", writerPrincipal(tenantA), "")
	wantStatus(t, w, http.StatusOK)
	var result bangDayDuRa
	decodeJSON(t, w, &result)
	if !strings.Contains(w.Body.String(), `"unavailable_reasons"`) {
		t.Fatalf("thân không có khoá `unavailable_reasons`: %s", w.Body.String())
	}
	for _, d := range result.Lines {
		if v, ok := d.Values["c-chi"]; !ok || v != nil {
			t.Fatalf("dòng %s c-chi = %v (có khoá %v), muốn null", d.ID, v, ok)
		}
		if !strings.Contains(d.UnavailableReasons["c-chi"], domain.ErrEntryTotalOverflow.Error()) {
			t.Fatalf("dòng %s thiếu lý do: %+v", d.ID, d.UnavailableReasons)
		}
		// c-dt of the entries leaf is simply EMPTY (no batch stated it): null and NO reason.
		if _, ok := d.UnavailableReasons["c-dt"]; ok {
			t.Fatalf("dòng %s: ô trống bị gắn lý do như ô không tính được", d.ID)
		}
	}
	var expenditureCell *oTongRa
	for i := range result.Summary.Cells {
		if result.Summary.Cells[i].ColumnID == "c-chi" {
			expenditureCell = &result.Summary.Cells[i]
		}
	}
	if expenditureCell == nil || expenditureCell.Value != nil || !strings.Contains(expenditureCell.UnavailableReason, domain.ErrEntryTotalOverflow.Error()) {
		t.Fatalf("ô tổng c-chi = %+v, muốn null kèm lý do", expenditureCell)
	}
	if result.Summary.Indicator.BasisPoints != nil || result.Summary.Indicator.UnavailableReason == "" {
		t.Fatalf("chỉ số = %+v, muốn không có số và có lý do", result.Summary.Indicator)
	}
}

func TestEntry_StoredAmountPastNewCeilingIsNullWithReason(t *testing.T) {
	// A batch amount stored under the old 10^17 ceiling. The list must still READ — it is how the
	// accountant finds the batch to remove — but the amount is not sent as a number the browser would
	// round.
	d := tenantALineEntries().Entries[0]
	d.Amounts = map[string]domain.Dong{"c-chi": 50_000_000_000_000_000, "c-dt": domain.ValueMax}
	result := entryToOut(d, tenantAExpenditureSheet().Columns)
	if v, ok := result.Values["c-chi"]; !ok || v != nil {
		t.Fatalf("c-chi = %v, muốn null", v)
	}
	if !strings.Contains(result.UnavailableReasons["c-chi"], domain.ErrStoredValueOverflow.Error()) {
		t.Fatalf("thiếu lý do: %+v", result.UnavailableReasons)
	}
	if v := result.Values["c-dt"]; v == nil || *v != int64(domain.ValueMax) {
		t.Fatalf("c-dt đúng trần = %v, muốn nguyên số", v)
	}
	if _, ok := result.UnavailableReasons["c-dt"]; ok {
		t.Fatal("số đúng trần bị gắn lý do")
	}
}

func TestReadSheet_EntriesLeafShowsEntryTotalAndParentSumsIt(t *testing.T) {
	// The read path at the surface: `k-a` in `entries` mode shows its batch sum, not its typed cell,
	// and a parent above it adds the batch sum in.
	m := newBudgetServer(t)
	m.grant(tenantA, "budget.read")

	b := tenantAExpenditureSheet()
	b.Lines[0].Method = domain.MethodChildren
	b.Lines[1].ParentID = expenditureHeadlineID
	b.Lines[1].Method = domain.MethodEntries
	b.EntryTotals = map[string]map[string]domain.Dong{"k-a": {"c-chi": 1_850_000}}
	m.reader.byTenant[tenantA][sheetKey(2026, domain.SheetKindExpenditure)] = b

	var result bangDayDuRa
	decodeJSON(t, m.call(t, http.MethodGet, hostA, sheetPath+"?year=2026&kind=chi", writerPrincipal(tenantA), ""), &result)
	for _, d := range result.Lines {
		switch d.ID {
		case "k-a":
			if v := d.Values["c-chi"]; v == nil || *v != 1_850_000 {
				t.Fatalf("lá `entries` c-chi = %v, muốn tổng đợt", v)
			}
			if v := d.Values["c-dt"]; v != nil {
				t.Fatalf("lá `entries` c-dt = %d, muốn null (không đợt nào ghi) — không phải số gõ tay đã khoá", *v)
			}
		case expenditureHeadlineID:
			if v := d.Values["c-chi"]; v == nil || *v != 1_850_000 {
				t.Fatalf("cha c-chi = %v, muốn cộng tổng đợt của lá", v)
			}
		}
	}
	if result.Summary.Indicator.BasisPoints != nil {
		// Dự toán năm on the marked row is empty now (the only child left c-dt empty): a sentence.
		t.Fatalf("chỉ số ra %d khi mẫu số trống", *result.Summary.Indicator.BasisPoints)
	}
}
