package http

// The HTTP surface of `at_risk` (migration 0018, ADR 0080 #2). The four permission cases of the PATCH
// carrying it are in du_an_ghi_test.go's matrix ("đánh dấu nguy cơ"); this file covers the body, the
// replies and the summary count.

import (
	"net/http"
	"strings"
	"testing"
)

func TestPatchProject_AtRiskReachesUseCaseAndReply(t *testing.T) {
	for _, c := range []struct {
		body string
		want bool
	}{{`{"at_risk":true}`, true}, {`{"at_risk":false}`, false}} {
		m := dungMayChuDuAnGhi(t)
		m.capQuyen(xaA, "budget.update")
		m.ghi.ra.AtRisk = c.want
		w := m.goi(t, http.MethodPatch, hostA, duongDuAnMot(), canBoGhi(xaA), c.body)
		doiMa(t, w, http.StatusOK)
		if p := m.ghi.suaCuoi.AtRisk; p == nil || *p != c.want {
			t.Fatalf("%s: use case received %v", c.body, p)
		}
		want := `"at_risk":false`
		if c.want {
			want = `"at_risk":true`
		}
		if !strings.Contains(w.Body.String(), want) {
			t.Fatalf("%s: reply = %s", c.body, w.Body.String())
		}
	}
}

func TestPatchProject_AtRiskAbsentIsUnchanged(t *testing.T) {
	m := dungMayChuDuAnGhi(t)
	m.capQuyen(xaA, "budget.update")
	doiMa(t, m.goi(t, http.MethodPatch, hostA, duongDuAnMot(), canBoGhi(xaA), thanSuaDA), http.StatusOK)
	if m.ghi.suaCuoi.AtRisk != nil {
		t.Fatalf("an absent flag reached the use case as %v", *m.ghi.suaCuoi.AtRisk)
	}
}

func TestPatchProject_AtRiskNotABooleanIs400(t *testing.T) {
	m := dungMayChuDuAnGhi(t)
	m.capQuyen(xaA, "budget.update")
	doiMa(t, m.goi(t, http.MethodPatch, hostA, duongDuAnMot(), canBoGhi(xaA), `{"at_risk":"yes"}`),
		http.StatusBadRequest)
	if m.ghi.suaGoi != 0 {
		t.Fatal("a malformed flag reached the use case")
	}
}

// The read always states the flag, false included — a client must not have to guess what absent means.
func TestProjectDetail_CarriesAtRiskAlways(t *testing.T) {
	m := dungMayChuVoi(t, coQuyen("budget.read"))
	w := m.goi(t, http.MethodGet, hostA, "/api/v1/investment-projects/da-001", canBoCua(xaA))
	doiMa(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"at_risk":false`) {
		t.Fatalf("body = %s", w.Body.String())
	}

	m = dungMayChuVoi(t, coQuyen("budget.read"))
	m.duAn.theo[xaA][0].DuAn.AtRisk = true
	w = m.goi(t, http.MethodGet, hostA, duongDanDuAn, canBoCua(xaA))
	doiMa(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"at_risk":true`) {
		t.Fatalf("list body = %s", w.Body.String())
	}
}

// §3 card 4: the summary counts this year's flagged projects of THIS commune — 0 stated, not omitted.
func TestProjectSummary_AtRiskCount(t *testing.T) {
	m := dungMayChuVoi(t, coQuyen("budget.read"))
	w := m.goi(t, http.MethodGet, hostA, summaryPath, canBoCua(xaA))
	doiMa(t, w, http.StatusOK)
	if s := decode[projectSummaryOut](t, w.Body.Bytes()); s.AtRiskCount == nil || *s.AtRiskCount != 0 {
		t.Fatalf("at_risk_count = %v, want 0 (stated)", s.AtRiskCount)
	}

	m = dungMayChuVoi(t, coQuyen("budget.read"))
	for i := range m.duAn.theo[xaA] {
		m.duAn.theo[xaA][i].DuAn.AtRisk = true // the 2025 project too — it must not be counted for 2026
	}
	for i := range m.duAn.theo[xaB] {
		m.duAn.theo[xaB][i].DuAn.AtRisk = true // commune B's flags must not reach A's card
	}
	w = m.goi(t, http.MethodGet, hostA, summaryPath, canBoCua(xaA))
	doiMa(t, w, http.StatusOK)
	s := decode[projectSummaryOut](t, w.Body.Bytes())
	if s.AtRiskCount == nil || *s.AtRiskCount != 2 {
		t.Fatalf("at_risk_count = %v, want 2 (commune A's two 2026 projects)", s.AtRiskCount)
	}
}
