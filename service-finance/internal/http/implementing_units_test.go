package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/domain"
)

// Migration 0016's HTTP surface (owner instruction 07/10/2026, following the prototype):
//
//	GET /api/v1/implementing-units?year=   rule 5 invariant 7's four cases, commune isolation, year
//	                                       demanded rather than defaulted, a failure that leaks nothing
//	?implementing_unit= on the project list an exact-match filter inside the commune
//	`implementing_unit` on the project      read, created, PATCHed (absent / cleared) and refused
//	`owner_code` / `due_on` on an issue      reach the use case and the reply; a malformed date never does
//
// The wrong-commune case uses newDiscussionServer, whose checker is KEYED BY COMMUNE: the account of
// commune B is signed in at B, and the key was granted in commune A only — so it answers 403 rather
// than the 401 a Host/principal mismatch produces (du_an_test.go's header).

const implementingUnitsPath = "/api/v1/implementing-units?year=2026"

// withUnits types an implementing unit on commune A's two 2026 projects and its 2025 one, and on
// commune B's project that shares A's id. The 2025 unit must never be offered for 2026.
func withUnits(d *duAnGia) {
	set := func(xa tenant.ID, id, unit string) {
		for i := range d.theo[xa] {
			if d.theo[xa][i].DuAn.ID == id {
				d.theo[xa][i].DuAn.ImplementingUnit = unit
			}
		}
	}
	set(xaA, "da-001", "Công ty Xây dựng Thành Long")
	set(xaA, "da-002", "Ban quản lý dự án xã")
	set(xaA, "da-2025", "Công ty của năm cũ")
	set(xaB, "da-001", "Đơn vị của XÃ B")
}

// --- rule 5, invariant 7 ---------------------------------------------------------------------------

func TestImplementingUnits_401NoSession(t *testing.T) {
	m := newDiscussionServer(t)
	m.grant(xaA, "budget.read")
	doiMa(t, m.call(t, http.MethodGet, hostA, implementingUnitsPath, nil, ""), http.StatusUnauthorized)
	if m.projects.unitReads != 0 {
		t.Fatal("the store was read with no session")
	}
}

func TestImplementingUnits_403WrongPermission(t *testing.T) {
	m := newDiscussionServer(t)
	// Real keys, none of them the one the route asks for (rule 5, invariant 3b).
	m.grant(xaA, "budget.update", "budget.confirm")
	w := m.call(t, http.MethodGet, hostA, implementingUnitsPath, canBoGhi(xaA), "")
	doiMa(t, w, http.StatusForbidden)
	if m.projects.unitReads != 0 {
		t.Fatal("the store was read with the wrong permission")
	}
}

func TestImplementingUnits_403RightPermissionWrongCommune(t *testing.T) {
	m := newDiscussionServer(t)
	m.grant(xaA, "budget.read")
	w := m.call(t, http.MethodGet, hostB, implementingUnitsPath, canBoGhi(xaB), "")
	doiMa(t, w, http.StatusForbidden)
	if m.projects.unitReads != 0 {
		t.Fatal("a key granted in another commune reached this commune's store")
	}
}

func TestImplementingUnits_200SortedDistinctOfTheYearAndCommune(t *testing.T) {
	m := newDiscussionServer(t)
	withUnits(m.projects)
	m.grant(xaA, "budget.read")
	w := m.call(t, http.MethodGet, hostA, implementingUnitsPath, canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)

	var out implementingUnitsOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	want := []string{"Ban quản lý dự án xã", "Công ty Xây dựng Thành Long"}
	if out.Year != 2026 || fmt.Sprint(out.Items) != fmt.Sprint(want) {
		t.Fatalf("out = %+v, want year 2026 and %v (the 2025 unit and commune B's must be absent)", out, want)
	}
}

// --- isolation, the year, failures -------------------------------------------------------------------

func TestImplementingUnits_CommuneBReadsOnlyItsOwn(t *testing.T) {
	m := newDiscussionServer(t)
	withUnits(m.projects)
	m.grant(xaB, "budget.read")
	w := m.call(t, http.MethodGet, hostB, implementingUnitsPath, canBoGhi(xaB), "")
	doiMa(t, w, http.StatusOK)
	if strings.Contains(w.Body.String(), "Thành Long") || !strings.Contains(w.Body.String(), "XÃ B") {
		t.Fatalf("commune B read another commune's units: %s", w.Body.String())
	}
}

func TestImplementingUnits_EmptyIsArrayNotNull(t *testing.T) {
	m := newDiscussionServer(t)
	m.grant(xaA, "budget.read")
	w := m.call(t, http.MethodGet, hostA, implementingUnitsPath, canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Fatalf("body = %s", w.Body.String())
	}
}

func TestImplementingUnits_YearIsRequiredNotDefaulted(t *testing.T) {
	for _, path := range []string{"/api/v1/implementing-units", "/api/v1/implementing-units?year=26",
		"/api/v1/implementing-units?year=abc"} {
		m := newDiscussionServer(t)
		m.grant(xaA, "budget.read")
		doiMa(t, m.call(t, http.MethodGet, hostA, path, canBoGhi(xaA), ""), http.StatusBadRequest)
		if m.projects.unitReads != 0 {
			t.Fatalf("%s: a rejected year still ran a statement", path)
		}
	}
}

func TestImplementingUnits_StoreFailureIs500WithoutDetail(t *testing.T) {
	m := newDiscussionServer(t)
	m.projects.unitErr = errors.New("pq: connection refused")
	m.grant(xaA, "budget.read")
	w := m.call(t, http.MethodGet, hostA, implementingUnitsPath, canBoGhi(xaA), "")
	doiMa(t, w, http.StatusInternalServerError)
	if strings.Contains(w.Body.String(), "pq:") {
		t.Fatalf("internal detail leaked: %s", w.Body.String())
	}
}

// --- the list filter and the read shape ----------------------------------------------------------------

func TestProjectList_ImplementingUnitIsExactMatchFilterAndOnTheItem(t *testing.T) {
	m := dungMayChuVoi(t, coQuyen("budget.read"))
	withUnits(m.duAn)
	w := m.goi(t, http.MethodGet, hostA,
		duongDanDuAn+"&implementing_unit=C%C3%B4ng%20ty%20X%C3%A2y%20d%E1%BB%B1ng%20Th%C3%A0nh%20Long", canBoCua(xaA))
	doiMa(t, w, http.StatusOK)
	var out danhSachDuAnRa
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if len(out.Items) != 1 || out.Items[0].ID != "da-001" || out.Items[0].ImplementingUnit == nil ||
		*out.Items[0].ImplementingUnit != "Công ty Xây dựng Thành Long" {
		t.Fatalf("items = %+v", out.Items)
	}
}

func TestProjectList_NoUnitIsAbsentNotEmptyString(t *testing.T) {
	m := dungMayChuVoi(t, coQuyen("budget.read"))
	w := m.goi(t, http.MethodGet, hostA, duongDanDuAn, canBoCua(xaA))
	doiMa(t, w, http.StatusOK)
	if strings.Contains(w.Body.String(), "implementing_unit") {
		t.Fatalf("a project with no unit must not carry the field: %s", w.Body.String())
	}
}

func TestProjectList_OverlongUnitFilterIs400BeforeTheStore(t *testing.T) {
	m := dungMayChuVoi(t, coQuyen("budget.read"))
	w := m.goi(t, http.MethodGet, hostA,
		duongDanDuAn+"&implementing_unit="+strings.Repeat("a", domain.ImplementingUnitMax+1), canBoCua(xaA))
	doiMa(t, w, http.StatusBadRequest)
	if m.duAn.goi != 0 {
		t.Fatal("an overlong filter still reached the store")
	}
}

func TestProjectDetail_CarriesImplementingUnit(t *testing.T) {
	m := dungMayChuVoi(t, coQuyen("budget.read"))
	withUnits(m.duAn)
	w := m.goi(t, http.MethodGet, hostA, "/api/v1/investment-projects/da-002", canBoCua(xaA))
	doiMa(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"implementing_unit":"Ban quản lý dự án xã"`) {
		t.Fatalf("body = %s", w.Body.String())
	}
}

// --- the write routes ----------------------------------------------------------------------------------

func TestCreateProject_ImplementingUnitReachesUseCaseAndReply(t *testing.T) {
	m := dungMayChuDuAnGhi(t)
	m.capQuyen(xaA, "budget.update")
	m.ghi.ra.ImplementingUnit = "Công ty Xây dựng Thành Long"
	body := strings.TrimSuffix(thanThemDA, "}") + `,"implementing_unit":"Công ty Xây dựng Thành Long"}`
	w := m.goi(t, http.MethodPost, hostA, duongDuAn, canBoGhi(xaA), body)
	doiMa(t, w, http.StatusCreated)
	if m.ghi.themCuoi.ImplementingUnit != "Công ty Xây dựng Thành Long" {
		t.Fatalf("use case received %q", m.ghi.themCuoi.ImplementingUnit)
	}
	if !strings.Contains(w.Body.String(), `"implementing_unit":"Công ty Xây dựng Thành Long"`) {
		t.Fatalf("reply = %s", w.Body.String())
	}
}

func TestPatchProject_ImplementingUnitAbsentIsUnchangedEmptyIsClear(t *testing.T) {
	t.Run("absent", func(t *testing.T) {
		m := dungMayChuDuAnGhi(t)
		m.capQuyen(xaA, "budget.update")
		doiMa(t, m.goi(t, http.MethodPatch, hostA, duongDuAnMot(), canBoGhi(xaA), thanSuaDA), http.StatusOK)
		if m.ghi.suaCuoi.ImplementingUnit != nil {
			t.Fatalf("an absent field reached the use case as %q", *m.ghi.suaCuoi.ImplementingUnit)
		}
	})
	t.Run("cleared", func(t *testing.T) {
		m := dungMayChuDuAnGhi(t)
		m.capQuyen(xaA, "budget.update")
		doiMa(t, m.goi(t, http.MethodPatch, hostA, duongDuAnMot(), canBoGhi(xaA), `{"implementing_unit":""}`),
			http.StatusOK)
		if p := m.ghi.suaCuoi.ImplementingUnit; p == nil || *p != "" {
			t.Fatalf("a cleared field must reach the use case as a pointer to \"\", got %v", p)
		}
	})
}

func TestProjectWrites_ImplementingUnitRefusalsAre400WithTheSentence(t *testing.T) {
	for _, e := range []error{domain.ErrImplementingUnitTooLong, domain.ErrImplementingUnitInvalid} {
		m := dungMayChuDuAnGhi(t)
		m.capQuyen(xaA, "budget.update")
		m.ghi.loi = fmt.Errorf("du_an: sửa cho xã %s: %w", xaA, e)
		w := m.goi(t, http.MethodPatch, hostA, duongDuAnMot(), canBoGhi(xaA), `{"implementing_unit":"x"}`)
		doiMa(t, w, http.StatusBadRequest)
		if !strings.Contains(loiTra(t, w).Message, "implementing_unit") {
			t.Fatalf("the sentence does not name the field: %q", loiTra(t, w).Message)
		}
	}
}

// --- issue owner and due date ------------------------------------------------------------------------

func TestRecordIssue_OwnerAndDueReachUseCaseAndReply(t *testing.T) {
	m := newDiscussionServer(t)
	m.grant(xaA, "budget.update")
	w := m.call(t, http.MethodPost, hostA, "/api/v1/investment-projects/da-001/issues", canBoGhi(xaA),
		`{"text":"Chưa bàn giao mặt bằng","owner_code":"CB-00042","due_on":"2026-10-31"}`)
	doiMa(t, w, http.StatusCreated)
	if m.write.lastIssue.OwnerCode != "CB-00042" ||
		!m.write.lastIssue.DueOn.Equal(time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("use case received %+v", m.write.lastIssue)
	}
	var out projectIssueOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if out.OwnerCode != "CB-00042" || out.DueOn != "2026-10-31" {
		t.Fatalf("out = %+v", out)
	}
}

func TestRecordIssue_NoOwnerNoDueAreAbsentInTheReply(t *testing.T) {
	m := newDiscussionServer(t)
	m.grant(xaA, "budget.update")
	w := m.call(t, http.MethodPost, hostA, "/api/v1/investment-projects/da-001/issues", canBoGhi(xaA),
		`{"text":"Vướng"}`)
	doiMa(t, w, http.StatusCreated)
	if strings.Contains(w.Body.String(), "owner_code") || strings.Contains(w.Body.String(), "due_on") {
		t.Fatalf("reply = %s", w.Body.String())
	}
}

func TestRecordIssue_MalformedDueOnIs400BeforeTheUseCase(t *testing.T) {
	m := newDiscussionServer(t)
	m.grant(xaA, "budget.update")
	w := m.call(t, http.MethodPost, hostA, "/api/v1/investment-projects/da-001/issues", canBoGhi(xaA),
		`{"text":"Vướng","due_on":"31/10/2026"}`)
	doiMa(t, w, http.StatusBadRequest)
	if m.write.calls != 0 || !strings.Contains(loiTra(t, w).Message, "due_on") {
		t.Fatalf("calls %d, message %q", m.write.calls, loiTra(t, w).Message)
	}
}

func TestRecordIssue_OwnerAndDueRefusalsAre400WithTheSentence(t *testing.T) {
	for _, e := range []error{domain.ErrIssueOwnerInvalid, domain.ErrIssueDueOnInvalid} {
		m := newDiscussionServer(t)
		m.grant(xaA, "budget.update")
		m.write.err = e
		w := m.call(t, http.MethodPost, hostA, "/api/v1/investment-projects/da-001/issues", canBoGhi(xaA),
			`{"text":"Vướng","owner_code":"CB 1"}`)
		doiMa(t, w, http.StatusBadRequest)
		if loiTra(t, w).Message != e.Error() {
			t.Fatalf("message = %q, want the sentinel's own text", loiTra(t, w).Message)
		}
	}
}
