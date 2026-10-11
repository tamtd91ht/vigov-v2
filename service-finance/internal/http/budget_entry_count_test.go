package http

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/vihat/vigov/service-finance/internal/domain"
)

// Row 53: `entry_count` — the live batches of each line, the badge on "Các đợt thu, chi" (prototype
// FiscalReportPanel.tsx:647-655). No new route and no new permission: it rides on the sheet read, whose
// four RBAC cases stay in thu_chi_ngan_sach_test.go.

func TestSheetReadCarriesEntryCountOnEveryLine(t *testing.T) {
	m := dungMayChuNganSach(t)
	m.capQuyen(xaA, "budget.read")

	b := bangChiCuaXaA()
	b.KhoanMuc[0].EntryCount = 3
	m.doc.theo[xaA][khoaBang(2026, domain.BangChi)] = b

	w := m.goi(t, http.MethodGet, hostA, duongBang+"?year=2026&kind=chi", canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	var raw struct {
		Lines []map[string]json.RawMessage `json:"lines"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(raw.Lines) != len(b.KhoanMuc) {
		t.Fatalf("%d lines, want %d", len(raw.Lines), len(b.KhoanMuc))
	}
	for i, line := range raw.Lines {
		want := "0"
		if i == 0 {
			want = "3"
		}
		// PRESENT AS 0 on a line with no batch: the full-sheet answer always knows the count, so an absent
		// key there would be a client guessing.
		if v, has := line["entry_count"]; !has || string(v) != want {
			t.Fatalf("line %d entry_count = %s (present %v), want %s", i, v, has, want)
		}
	}
}

// The single-line write answers hold one row, not the count: the key is ABSENT there rather than a 0
// that would read as "no batches" (dongRaMot's reason for omitting `values`).
func TestSingleLineAnswerOmitsEntryCount(t *testing.T) {
	out, err := json.Marshal(dongRaMot(domain.KhoanMucNganSach{ID: "k-1", Ten: "Chi thường xuyên", EntryCount: 0}))
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatal(err)
	}
	if _, has := raw["entry_count"]; has {
		t.Fatalf("single-line answer carries entry_count: %s", out)
	}
}
