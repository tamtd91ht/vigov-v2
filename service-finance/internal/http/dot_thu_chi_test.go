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
// the surface. The four rule-5 cases for all three routes run in thu_chi_ngan_sach_test.go, where the
// routes are rows of tamTuyenNganSach; this file holds what is specific to the batches:
//
//  1. `budget.update` alone cannot remove a batch — removal is `budget.confirm`;
//  2. the list's wire shape: every number column a key, `null` for empty, newest first, `method`;
//  3. refusals reach the client with the right status and never quote the counterparty;
//  4. the counterparty never reaches a log line, even on a 500;
//  5. `method` on PATCH: `manual` / `entries` pass, `children` is 400 before the use case.

func TestDot_BudgetUpdateKhongGoDuocDot(t *testing.T) {
	// The accountant who records batches holds `budget.read` + `budget.update`. Taking a batch back
	// changes an `entries` line's figure, possibly already read off a screen — `budget.confirm`.
	m := dungMayChuNganSach(t)
	m.capQuyen(xaA, "budget.read", "budget.update")

	doiMa(t, m.goi(t, http.MethodDelete, hostA, duongDot+"/01JDOTMOINHAT0000000000000", canBoGhi(xaA),
		`{"reason":"ghi nhầm"}`), http.StatusForbidden)
	if m.ghi.goDotGoi != 0 {
		t.Fatal("tài khoản chỉ có budget.update mà vẫn gỡ được đợt")
	}
	// ...while the same account CAN record one.
	doiMa(t, m.goi(t, http.MethodPost, hostA, duongDong+"/k-a/entries", canBoGhi(xaA), thanGhiDot),
		http.StatusCreated)
}

func TestDot_DanhSachDungHinhDang(t *testing.T) {
	m := dungMayChuNganSach(t)
	m.capQuyen(xaA, "budget.read")

	var ra danhSachDotRa
	docJSON(t, m.goi(t, http.MethodGet, hostA, duongDong+"/k-a/entries", canBoGhi(xaA), ""), &ra)

	if ra.LineID != "k-a" || ra.Method != "entries" {
		t.Fatalf("line_id=%q method=%q", ra.LineID, ra.Method)
	}
	if len(ra.Entries) != 2 || ra.Entries[0].ID != "01JDOTMOINHAT0000000000000" {
		t.Fatalf("thứ tự đợt sai (mới nhất trước): %+v", ra.Entries)
	}
	moi, cu := ra.Entries[0], ra.Entries[1]
	if moi.Date != "2026-08-20" || moi.DocumentNo != "PC-0102" {
		t.Fatalf("đợt mới nhất: %+v", moi)
	}
	// 0 IS A STATED AMOUNT; an absent amount is `null`; the % column is not a key at all.
	if v := moi.Values["c-dt"]; v == nil || *v != 0 {
		t.Fatalf("c-dt của đợt mới = %v, muốn 0", v)
	}
	if v, co := cu.Values["c-dt"]; !co || v != nil {
		t.Fatalf("c-dt của đợt cũ = %v (có khoá: %v), muốn null", v, co)
	}
	if v := cu.Values["c-chi"]; v == nil || *v != -150_000 {
		t.Fatalf("số âm bị đổi: %v", v)
	}
	if _, co := moi.Values["c-ty"]; co {
		t.Fatal("cột phần trăm có ô số tiền")
	}
	if cu.Counterparty != "" {
		t.Fatalf("đợt không ghi đơn vị mà trả %q", cu.Counterparty)
	}
}

// donViDaCheHTTP is privacy.MaskName(donViMauHTTP), written out as a LITERAL: computing it with the
// helper here would pass even if the handler called nothing, or called the helper on the wrong field.
const donViDaCheHTTP = "Hộ b. T. T. M."

func TestDot_DanhSachCheDonViCaNhan(t *testing.T) {
	// Rule 3, invariant 3: "Đơn vị, cá nhân" may be a citizen's name and no key grants the full value.
	// Checked on the RAW BODY as well as the decoded struct: a second field carrying the raw text
	// would pass a struct-only assertion.
	m := dungMayChuNganSach(t)
	m.capQuyen(xaA, "budget.read")

	w := m.goi(t, http.MethodGet, hostA, duongDong+"/k-a/entries", canBoGhi(xaA), "")
	var ra danhSachDotRa
	docJSON(t, w, &ra)
	if ra.Entries[0].Counterparty != donViDaCheHTTP {
		t.Fatalf("counterparty = %q, muốn %q", ra.Entries[0].Counterparty, donViDaCheHTTP)
	}
	if strings.Contains(w.Body.String(), donViMauHTTP) || strings.Contains(w.Body.String(), "Trần Thị") {
		t.Fatalf("thân trả về chứa nguyên văn `đơn vị, cá nhân`: %s", w.Body.String())
	}
}

func TestDot_PhanHoiGhiCheDonViCaNhan(t *testing.T) {
	// The 201 of a create carries the batch back — the same rule holds there, even though the caller
	// has just typed the value: a reply is stored by proxies, clients and browser caches alike.
	m := dungMayChuNganSach(t)
	m.capQuyen(xaA, "budget.update")
	m.ghi.raDot = dotCuaXaA().Dot[0] // DonViCaNhan = donViMauHTTP, raw, as the use case returns it

	w := m.goi(t, http.MethodPost, hostA, duongDong+"/k-a/entries", canBoGhi(xaA), thanGhiDot)
	doiMa(t, w, http.StatusCreated)
	var ra dotRa
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân không phải JSON: %v", err)
	}
	if ra.Counterparty != donViDaCheHTTP {
		t.Fatalf("counterparty = %q, muốn %q", ra.Counterparty, donViDaCheHTTP)
	}
	if strings.Contains(w.Body.String(), donViMauHTTP) || strings.Contains(w.Body.String(), "Trần Thị") {
		t.Fatalf("thân 201 chứa nguyên văn `đơn vị, cá nhân`: %s", w.Body.String())
	}
}

func TestDot_DanhSachCuaXaNaoRaCuaXaAy(t *testing.T) {
	m := dungMayChuNganSach(t)
	m.capQuyen(xaA, "budget.read")
	m.capQuyen(xaB, "budget.read")

	var a, b danhSachDotRa
	docJSON(t, m.goi(t, http.MethodGet, hostA, duongDong+"/k-a/entries", canBoGhi(xaA), ""), &a)
	docJSON(t, m.goi(t, http.MethodGet, hostB, duongDong+"/k-a/entries", canBoGhi(xaB), ""), &b)
	if len(a.Entries) != 2 || len(b.Entries) != 1 || !strings.Contains(b.Entries[0].Content, "BÌNH DƯƠNG") {
		t.Fatalf("đợt lẫn xã: A=%d B=%+v", len(a.Entries), b.Entries)
	}
}

func TestDot_KhoanMucKhongCoTra404(t *testing.T) {
	m := dungMayChuNganSach(t)
	m.capQuyen(xaA, "budget.read")
	doiMa(t, m.goi(t, http.MethodGet, hostA, duongDong+"/khong-co/entries", canBoGhi(xaA), ""),
		http.StatusNotFound)
}

func TestDot_GhiChuyenDungTruongToiUseCase(t *testing.T) {
	m := dungMayChuNganSach(t)
	m.capQuyen(xaA, "budget.update")

	doiMa(t, m.goi(t, http.MethodPost, hostA, duongDong+"/k-a/entries", canBoGhi(xaA), thanGhiDot),
		http.StatusCreated)
	yc := m.ghi.ghiDotCuoi
	if yc.KhoanMucID != "k-a" || yc.NoiDung != "Chi hỗ trợ đợt 3" || yc.DonViCaNhan != donViMauHTTP ||
		yc.SoChungTu != "PC-0103" || yc.Ngay.Format("2006-01-02") != "2026-08-20" {
		t.Fatalf("yêu cầu tới use case: %+v", yc)
	}
	if g := yc.GiaTri["c-chi"]; g == nil || *g != 2_500_000 {
		t.Fatalf("c-chi = %v", g)
	}
	if g, co := yc.GiaTri["c-dt"]; !co || g != nil {
		t.Fatalf("c-dt null phải tới use case là nil có khoá: %v %v", g, co)
	}
}

func TestDot_NgaySaiDangTra400KhongGoiUseCase(t *testing.T) {
	m := dungMayChuNganSach(t)
	m.capQuyen(xaA, "budget.update")
	for _, than := range []string{
		`{"date":"20/08/2026","content":"X","values":{"c-chi":1}}`,
		`not json`,
	} {
		doiMa(t, m.goi(t, http.MethodPost, hostA, duongDong+"/k-a/entries", canBoGhi(xaA), than),
			http.StatusBadRequest)
	}
	if m.ghi.ghiDotGoi != 0 {
		t.Fatalf("use case chạy %d lần với thân đã bị từ chối", m.ghi.ghiDotGoi)
	}
}

func TestDot_TuChoiRaDungMaVaKhongNhacDonVi(t *testing.T) {
	// The use case's refusals, mapped. The app tests prove each refusal writes nothing; this proves
	// the status and that the sentence never quotes the counterparty (rule 3, forbidden #3).
	for _, tc := range []struct {
		loi error
		ma  int
	}{
		{domain.ErrDotChiGhiVaoLa, http.StatusConflict},
		{domain.ErrKhoanMucTheoDotConDot, http.StatusConflict},
		{domain.ErrKhoanMucTheoDotKhongGoThang, http.StatusConflict},
		{domain.ErrKhoanMucChaKhongDoiCachTinh, http.StatusConflict},
		{domain.ErrCotKhongPhaiCotSo, http.StatusBadRequest},
		{domain.ErrCotKhongThuocBang, http.StatusBadRequest},
		{domain.ErrNoiDungDotQuaDai, http.StatusBadRequest},
		{domain.ErrDonViCaNhanQuaDai, http.StatusBadRequest},
		{domain.ErrSoChungTuDotQuaDai, http.StatusBadRequest},
		{domain.ErrNgayDotNgoaiLich, http.StatusBadRequest},
		{domain.ErrDotKhongCoSoTienNao, http.StatusBadRequest},
		{domain.ErrKhongThayKhoanMuc, http.StatusNotFound},
	} {
		t.Run(tc.loi.Error(), func(t *testing.T) {
			m := dungMayChuNganSach(t)
			m.capQuyen(xaA, "budget.update")
			m.ghi.loi = fmt.Errorf("ngan_sach: ghi đợt cho xã x: %w", tc.loi)
			w := m.goi(t, http.MethodPost, hostA, duongDong+"/k-a/entries", canBoGhi(xaA), thanGhiDot)
			doiMa(t, w, tc.ma)
			if strings.Contains(w.Body.String(), donViMauHTTP) {
				t.Fatal("thân lỗi chứa nguyên văn `đơn vị, cá nhân`")
			}
		})
	}
}

func TestDot_GoDotDaGoHoacXaKhacTra404(t *testing.T) {
	m := dungMayChuNganSach(t)
	m.capQuyen(xaA, "budget.confirm")
	m.ghi.loi = fmt.Errorf("ngan_sach: gỡ đợt cho xã x: %w", domain.ErrKhongThayDot)
	doiMa(t, m.goi(t, http.MethodDelete, hostA, duongDot+"/01JDOTDAGO000000000000000", canBoGhi(xaA),
		`{"reason":"x"}`), http.StatusNotFound)
	if m.ghi.idCuoi != "01JDOTDAGO000000000000000" {
		t.Fatalf("id tới use case = %q", m.ghi.idCuoi)
	}
}

func TestDot_LoiHeThongKhongGhiDonViVaoNhatKy(t *testing.T) {
	// A 500 logs the wrapped error. Nothing on that path may carry the counterparty — the log travels
	// into centralised logging across every commune (rule 3, invariant 1).
	m := dungMayChuNganSach(t)
	m.capQuyen(xaA, "budget.update")
	m.ghi.loi = errors.New("store: begin: connection refused")

	doiMa(t, m.goi(t, http.MethodPost, hostA, duongDong+"/k-a/entries", canBoGhi(xaA), thanGhiDot),
		http.StatusInternalServerError)
	if m.nhatKy.Len() == 0 {
		t.Fatal("lỗi hệ thống mà không có dòng nhật ký nào — phép kiểm dưới đây sẽ xanh vô nghĩa")
	}
	if strings.Contains(m.nhatKy.String(), donViMauHTTP) || strings.Contains(m.nhatKy.String(), "Trần Thị") {
		t.Fatalf("nhật ký chứa `đơn vị, cá nhân`: %s", m.nhatKy.String())
	}
}

// --- `method` on PATCH /api/v1/budget-lines/{id} -------------------------------------------------------

func TestSuaDong_MethodManualVaEntriesToiUseCase(t *testing.T) {
	for _, ma := range []string{"manual", "entries"} {
		t.Run(ma, func(t *testing.T) {
			m := dungMayChuNganSach(t)
			m.capQuyen(xaA, "budget.update")
			doiMa(t, m.goi(t, http.MethodPatch, hostA, duongDong+"/k-a", canBoGhi(xaA), `{"method":"`+ma+`"}`),
				http.StatusOK)
			if ct := m.ghi.suaCuoi.CachTinh; ct == nil || string(*ct) != ma {
				t.Fatalf("cách tính tới use case = %v, muốn %q", ct, ma)
			}
		})
	}
}

func TestSuaDong_MethodChildrenHoacLaTra400(t *testing.T) {
	m := dungMayChuNganSach(t)
	m.capQuyen(xaA, "budget.update")
	for _, ma := range []string{"children", "", "MANUAL"} {
		doiMa(t, m.goi(t, http.MethodPatch, hostA, duongDong+"/k-a", canBoGhi(xaA), `{"method":"`+ma+`"}`),
			http.StatusBadRequest)
	}
	if m.ghi.suaGoi != 0 {
		t.Fatalf("use case chạy %d lần với `method` không hợp lệ", m.ghi.suaGoi)
	}
	// POST still refuses `method` of any value: a new line is always a `manual` leaf.
	doiMa(t, m.goi(t, http.MethodPost, hostA, duongDong, canBoGhi(xaA),
		`{"sheet_id":"`+idBangChi+`","name":"X","order":1,"method":"entries"}`), http.StatusBadRequest)
}

func TestDocBang_LaTheoDotHienTongDotVaChaCongVao(t *testing.T) {
	// The read path at the surface: `k-a` in `entries` mode shows its batch sum, not its typed cell,
	// and a parent above it adds the batch sum in.
	m := dungMayChuNganSach(t)
	m.capQuyen(xaA, "budget.read")

	b := bangChiCuaXaA()
	b.KhoanMuc[0].CachTinh = domain.TinhTheoCon
	b.KhoanMuc[1].ChaID = idDongChi
	b.KhoanMuc[1].CachTinh = domain.TinhTheoDot
	b.GiaDot = map[string]map[string]domain.Dong{"k-a": {"c-chi": 1_850_000}}
	m.doc.theo[xaA][khoaBang(2026, domain.BangChi)] = b

	var ra bangDayDuRa
	docJSON(t, m.goi(t, http.MethodGet, hostA, duongBang+"?year=2026&kind=chi", canBoGhi(xaA), ""), &ra)
	for _, d := range ra.Lines {
		switch d.ID {
		case "k-a":
			if v := d.Values["c-chi"]; v == nil || *v != 1_850_000 {
				t.Fatalf("lá `entries` c-chi = %v, muốn tổng đợt", v)
			}
			if v := d.Values["c-dt"]; v != nil {
				t.Fatalf("lá `entries` c-dt = %d, muốn null (không đợt nào ghi) — không phải số gõ tay đã khoá", *v)
			}
		case idDongChi:
			if v := d.Values["c-chi"]; v == nil || *v != 1_850_000 {
				t.Fatalf("cha c-chi = %v, muốn cộng tổng đợt của lá", v)
			}
		}
	}
	if ra.Summary.Indicator.BasisPoints != nil {
		// Dự toán năm on the marked row is empty now (the only child left c-dt empty): a sentence.
		t.Fatalf("chỉ số ra %d khi mẫu số trống", *ra.Summary.Indicator.BasisPoints)
	}
}
