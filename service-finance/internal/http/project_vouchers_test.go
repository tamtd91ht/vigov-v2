package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"
)

// GET /api/v1/investment-projects/{id}/disbursements — rule 5 invariant 7's four cases, commune
// isolation on a COLLIDING project id, the store's order reaching the wire untouched, and the
// funding-source refusals of the write routes (decision 06/10/2026) reaching the caller as 409.

const projectVouchersPath = "/api/v1/investment-projects/da-001/disbursements"

func voucherOf(id string, day int, source, sourceName string) domain.ProjectVoucher {
	return domain.ProjectVoucher{
		Voucher: domain.ChungTuGiaiNgan{
			ID: id, DuAnID: "da-001", NgayChi: time.Date(2026, 9, day, 0, 0, 0, 0, time.UTC),
			SoTien: 1_000_000, NoiDung: "Thanh toán", NguonVonID: source,
			TrangThai: domain.ChungTuKeToanNhap, NguoiNhapID: "CB-00123",
		},
		FundingSourceName: sourceName,
	}
}

// withVouchers gives commune A's da-001 two vouchers (newest first, as the store returns them) and
// commune B's da-001 — the SAME id — one voucher of its own.
func withVouchers(d *duAnGia) {
	d.vouchers = map[tenant.ID]map[string][]domain.ProjectVoucher{
		xaA: {"da-001": {
			voucherOf("ct-new", 20, "nv-xa", "Ngân sách xã"),
			voucherOf("ct-old", 7, "", ""),
		}},
		xaB: {"da-001": {voucherOf("ct-of-commune-b", 1, "", "")}},
	}
}

func TestProjectVouchers_FourPermissionCases(t *testing.T) {
	t.Run("401 no session", func(t *testing.T) {
		m := dungMayChuVoi(t, coQuyen("budget.read"))
		doiMa(t, m.goi(t, http.MethodGet, hostA, projectVouchersPath, nil), http.StatusUnauthorized)
		if m.duAn.voucherReads != 0 {
			t.Fatal("the store was read with no session")
		}
	})
	t.Run("403 wrong permission", func(t *testing.T) {
		m := dungMayChuVoi(t, coQuyen("budget.update"))
		w := m.goi(t, http.MethodGet, hostA, projectVouchersPath, canBoCua(xaA))
		doiMa(t, w, http.StatusForbidden)
		if m.duAn.voucherReads != 0 {
			t.Fatal("the store was read with the wrong permission")
		}
	})
	t.Run("refused: right permission, wrong commune", func(t *testing.T) {
		// Answers 401, not 403 — the shipped edge behaviour du_an_test.go's header explains.
		m := dungMayChuVoi(t, coQuyen("budget.read"))
		doiMa(t, m.goi(t, http.MethodGet, hostA, projectVouchersPath, canBoCua(xaB)), http.StatusUnauthorized)
		if m.duAn.voucherReads != 0 {
			t.Fatal("the store was read with another commune's token")
		}
	})
	t.Run("200 right permission, right commune", func(t *testing.T) {
		m := dungMayChuVoi(t, coQuyen("budget.read"))
		withVouchers(m.duAn)
		w := m.goi(t, http.MethodGet, hostA, projectVouchersPath, canBoCua(xaA))
		doiMa(t, w, http.StatusOK)
		var out projectVouchersOut
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatalf("body is not JSON: %v", err)
		}
		if out.Count != 2 || len(out.Items) != 2 || out.ProjectID != "da-001" {
			t.Fatalf("out = %+v", out)
		}
		// The store's order (newest payment first) reaches the wire untouched.
		if out.Items[0].ID != "ct-new" || out.Items[1].ID != "ct-old" {
			t.Fatalf("order = %s, %s", out.Items[0].ID, out.Items[1].ID)
		}
		first := out.Items[0]
		if first.FundingSourceID != "nv-xa" || first.FundingSourceName != "Ngân sách xã" ||
			first.Status != "ke-toan-nhap" || first.PaymentDate != "2026-09-20" {
			t.Fatalf("first item = %+v", first)
		}
		// A voucher with no source carries neither field — the screen draws `—`.
		if !strings.Contains(w.Body.String(), `"id":"ct-old"`) ||
			strings.Count(w.Body.String(), "funding_source_name") != 1 {
			t.Fatalf("funding_source_name must be absent on a sourceless voucher: %s", w.Body.String())
		}
	})
}

func TestProjectVouchers_CollidingProjectIDStaysInItsCommune(t *testing.T) {
	m := dungMayChuVoi(t, coQuyen("budget.read"))
	withVouchers(m.duAn)
	w := m.goi(t, http.MethodGet, hostB, projectVouchersPath, canBoCua(xaB))
	doiMa(t, w, http.StatusOK)
	if strings.Contains(w.Body.String(), "ct-new") || !strings.Contains(w.Body.String(), "ct-of-commune-b") {
		t.Fatalf("commune B read another commune's vouchers: %s", w.Body.String())
	}
}

func TestProjectVouchers_EmptyProjectIsEmptyArrayNotNull(t *testing.T) {
	m := dungMayChuVoi(t, coQuyen("budget.read"))
	w := m.goi(t, http.MethodGet, hostA, "/api/v1/investment-projects/da-002/disbursements", canBoCua(xaA))
	doiMa(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"items":[]`) || !strings.Contains(w.Body.String(), `"count":0`) {
		t.Fatalf("body = %s", w.Body.String())
	}
}

func TestProjectVouchers_UnknownProjectIs404(t *testing.T) {
	m := dungMayChuVoi(t, coQuyen("budget.read"))
	w := m.goi(t, http.MethodGet, hostA, "/api/v1/investment-projects/da-missing/disbursements", canBoCua(xaA))
	doiMa(t, w, http.StatusNotFound)
}

func TestProjectVouchers_StoreFailureAndCeilingAre500WithoutDetail(t *testing.T) {
	for _, storeErr := range []error{fistore.ErrTooManyVouchers, errors.New("pq: connection refused")} {
		m := dungMayChuVoi(t, coQuyen("budget.read"))
		m.duAn.voucherErr = storeErr
		w := m.goi(t, http.MethodGet, hostA, projectVouchersPath, canBoCua(xaA))
		doiMa(t, w, http.StatusInternalServerError)
		if strings.Contains(w.Body.String(), "pq:") || strings.Contains(w.Body.String(), "vượt trần") {
			t.Fatalf("internal detail leaked: %s", w.Body.String())
		}
	}
}

// --- the funding-source refusals of the write routes ------------------------------------------------

func TestVoucherWrites_SourceRulesAre409WithTheirCodes(t *testing.T) {
	cases := []struct {
		err  error
		code string
	}{
		{domain.ErrSourceRequired, "source_required"},
		{domain.ErrSourceNotAllocated, "source_not_allocated"},
	}
	for _, c := range cases {
		for _, route := range []struct{ method, path, body string }{
			{http.MethodPost, duongChungTu, thanThemChungTu},
			{http.MethodPatch, duongChungTuMot(""), `{"funding_source_id":""}`},
		} {
			t.Run(c.code+" "+route.method, func(t *testing.T) {
				m := dungMayChuChungTu(t)
				m.capQuyen(xaA, "budget.update")
				m.ghi.loi = c.err
				w := m.goi(t, route.method, hostA, route.path, canBoGhi(xaA), route.body)
				doiMa(t, w, http.StatusConflict)
				e := loiTra(t, w)
				if e.Code != c.code {
					t.Fatalf("code = %q, want %q", e.Code, c.code)
				}
				if !strings.Contains(e.Message, "funding_source_id") {
					t.Errorf("the sentence does not name the field: %q", e.Message)
				}
			})
		}
	}
}
