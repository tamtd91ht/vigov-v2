package http

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// Tests for the two TERMINAL BRANCHES at the HTTP edge — POST …/rejection and …/referral — and for
// the two READ surfaces that now carry the branch's reason.
//
// The rule 5 matrix (401 · 403 wrong key · 401 right key WRONG COMMUNE · 200), the restricted-field
// fact being handed down, and the restricted-field 404 are proved for both routes by the shared table
// in xu_ly_phan_anh_test.go (caCacTuyen / tuyenGhi), where both routes are rows. What is here is what
// that table cannot express.

const (
	lyDoThatHTTP   = "Việc này thuộc thẩm quyền của Điện lực huyện, xã đã chuyển kèm hồ sơ."
	coQuanThatHTTP = "Điện lực huyện Thăng Bình"
)

// TestKetThucNhanhSaiTrangThaiThi409 — the caller HOLDS `feedback.classify`; what is refused is this
// act on a petition that is no longer being classified. 403 would send them to the Phân quyền screen.
func TestKetThucNhanhSaiTrangThaiThi409(t *testing.T) {
	for _, ca := range tuyenGhiNhanh() {
		t.Run(ca.ten, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, ca.khoa)
			m.xuLy.loi = domain.ErrKetThucNhanhSaiLuc

			w := m.goiThan(t, ca.method, hostA, ca.duong, canBoCuaXa(xaA), ca.than)

			doiMa(t, w, http.StatusConflict)
			if loiTra(t, w).Code != "petition_state" {
				t.Errorf("mã lỗi = %q, muốn petition_state", loiTra(t, w).Code)
			}
		})
	}
}

// TestKetThucNhanhDauVaoSaiThi400 — the refusals of what the officer TYPED map to 400. The refusals
// themselves are proved in internal/app over the real store; the mapping is this layer's.
func TestKetThucNhanhDauVaoSaiThi400(t *testing.T) {
	for _, ca := range tuyenGhiNhanh() {
		for _, loi := range []error{domain.ErrThieuLyDo, domain.ErrLyDoQuaNgan, domain.ErrLyDoQuaDai,
			domain.ErrThieuCoQuanNhan, domain.ErrCoQuanNhanQuaDai} {
			t.Run(ca.ten+"/"+loi.Error(), func(t *testing.T) {
				m := dungMayChu(t)
				m.capQuyen(t, ca.khoa)
				m.xuLy.loi = loi

				w := m.goiThan(t, ca.method, hostA, ca.duong, canBoCuaXa(xaA), ca.than)

				doiMa(t, w, http.StatusBadRequest)
				if loiTra(t, w).Code != "invalid_request" {
					t.Errorf("mã lỗi = %q, muốn invalid_request", loiTra(t, w).Code)
				}
			})
		}
	}
}

// TestKetThucNhanhThanDiNguyenVenXuongUseCase — the texts must arrive UNCHANGED and to the right act.
// A handler that swapped reason and body would store the authority's name as the reason.
func TestKetThucNhanhThanDiNguyenVenXuongUseCase(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("feedback.classify"))
	w := m.goiThan(t, http.MethodPost, hostA, duongKhongTiepNhan(maPhieuThuong), canBoCuaXa(xaA),
		khongTiepNhanVao{Reason: lyDoThatHTTP})
	doiMa(t, w, http.StatusOK)
	if m.xuLy.viec != "khong-tiep-nhan" || m.xuLy.lyDo != lyDoThatHTTP || m.xuLy.maDa != maPhieuThuong {
		t.Errorf("việc=%q lý do=%q mã=%q", m.xuLy.viec, m.xuLy.lyDo, m.xuLy.maDa)
	}
	if m.xuLy.nguoi.ID != maCanBo {
		t.Errorf("chủ thể = %q, muốn MÃ cán bộ %q", m.xuLy.nguoi.ID, maCanBo)
	}

	m = dungMayChu(t)
	m.capQuyen(t, authz.Perm("feedback.classify"))
	w = m.goiThan(t, http.MethodPost, hostA, duongChuyenCapTren(maPhieuThuong), canBoCuaXa(xaA),
		chuyenCapTrenVao{Reason: lyDoThatHTTP, ReceivingBody: coQuanThatHTTP})
	doiMa(t, w, http.StatusOK)
	if m.xuLy.viec != "chuyen-cap-tren" || m.xuLy.ycChuyen.LyDo != lyDoThatHTTP ||
		m.xuLy.ycChuyen.CoQuanNhan != coQuanThatHTTP {
		t.Errorf("việc=%q yêu cầu=%+v", m.xuLy.viec, m.xuLy.ycChuyen)
	}
}

// TestChuyenCapTrenThieuCoQuanTrongThanVanXuongUseCaseDeTuChoi — the handler does not validate; an
// absent `receiving_body` reaches the use case as "" and the use case refuses it (proved in
// internal/app). What this pins is that the handler does not fill a default.
func TestChuyenCapTrenThieuCoQuanTrongThanVanXuongUseCaseDeTuChoi(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("feedback.classify"))
	m.xuLy.loi = domain.ErrThieuCoQuanNhan

	w := m.goiThan(t, http.MethodPost, hostA, duongChuyenCapTren(maPhieuThuong), canBoCuaXa(xaA),
		map[string]string{"reason": lyDoThatHTTP})

	doiMa(t, w, http.StatusBadRequest)
	if m.xuLy.ycChuyen.CoQuanNhan != "" {
		t.Errorf("handler tự điền cơ quan nhận %q", m.xuLy.ycChuyen.CoQuanNhan)
	}
}

// --- the staff detail carries the branch -----------------------------------------------------------

func TestDocPhieuNhanhTraLyDoCoQuanVaLuc(t *testing.T) {
	m := dungMayChu(t)
	luc := time.Date(2026, 9, 9, 10, 17, 0, 0, time.UTC)
	p := m.phieu.theo[xaA][maPhieuThuong]
	p.TrangThai = domain.ChuyenCapTren
	p.LyDoKetThucNhanh = lyDoThatHTTP
	p.CoQuanNhan = coQuanThatHTTP
	p.KetThucNhanhLuc = luc
	m.phieu.theo[xaA][maPhieuThuong] = p

	w := m.goi(t, http.MethodGet, hostA, duong(maPhieuThuong), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)
	ra := docPhieu(t, w.Body.Bytes())

	if ra.Reason != lyDoThatHTTP || ra.ReceivingBody != coQuanThatHTTP {
		t.Errorf("reason=%q receiving_body=%q", ra.Reason, ra.ReceivingBody)
	}
	if ra.BranchEndedAt == nil || !ra.BranchEndedAt.Equal(luc) {
		t.Errorf("branch_ended_at = %v, muốn %v", ra.BranchEndedAt, luc)
	}
}

// TestDocPhieuKhongNhanhKhongCoKhoaNhanh — the three keys are ABSENT, not empty, on the seven other
// statuses. That is what omitempty buys, and what keeps apidoc from declaring them required.
func TestDocPhieuKhongNhanhKhongCoKhoaNhanh(t *testing.T) {
	m := dungMayChu(t)
	w := m.goi(t, http.MethodGet, hostA, duong(maPhieuThuong), canBoCuaXa(xaA))
	doiMa(t, w, http.StatusOK)

	var tho map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &tho); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	for _, khoa := range []string{"reason", "receiving_body", "branch_ended_at"} {
		if _, co := tho[khoa]; co {
			t.Errorf("khoá %q có mặt trên phiếu không ở nhánh: %s", khoa, w.Body.String())
		}
	}
}

// --- the citizen reads why --------------------------------------------------------------------------

func TestCuaToiNhanhTraLyDoVaCoQuan(t *testing.T) {
	for _, tt := range []domain.TrangThai{domain.KhongTiepNhan, domain.ChuyenCapTren} {
		t.Run(string(tt), func(t *testing.T) {
			m := dungMayChuCongDan(t)
			p := m.phieu.theo[xaA][maCuaToi]
			p.TrangThai = tt
			p.LyDoKetThucNhanh = lyDoThatHTTP
			if tt == domain.ChuyenCapTren {
				p.CoQuanNhan = coQuanThatHTTP
			}
			m.phieu.theo[xaA][maCuaToi] = p

			w := m.goi(t, duongCuaToi(maCuaToi), tokenCuaToi)
			doiMa(t, w, http.StatusOK)
			ra := docPhieuCuaToi(t, w.Body.Bytes())

			if ra.Reason != lyDoThatHTTP {
				t.Errorf("reason = %q — người dân không đọc được vì sao phiếu kết thúc", ra.Reason)
			}
			if tt == domain.ChuyenCapTren && ra.ReceivingBody != coQuanThatHTTP {
				t.Errorf("receiving_body = %q — người dân không biết phiếu đi đâu", ra.ReceivingBody)
			}
			// The instant of the act is staff-only; nobody decided the citizen screen shows it.
			if strings.Contains(w.Body.String(), "branch_ended_at") {
				t.Errorf("mốc kết thúc nhánh lọt ra bề mặt công dân: %s", w.Body.String())
			}
		})
	}
}

// TestCuaToiKhongNhanhKhongTraLyDo — the citizen surface GATES ON THE STATUS itself: a row carrying
// branch text on a non-branch status (which the CHECK forbids, but a restore or a hand fix can
// produce) must not show "why we refused" on a petition nobody refused.
func TestCuaToiKhongNhanhKhongTraLyDo(t *testing.T) {
	m := dungMayChuCongDan(t)
	p := m.phieu.theo[xaA][maCuaToi] // dang-phan-loai
	p.LyDoKetThucNhanh = lyDoThatHTTP
	p.CoQuanNhan = coQuanThatHTTP
	m.phieu.theo[xaA][maCuaToi] = p

	w := m.goi(t, duongCuaToi(maCuaToi), tokenCuaToi)
	doiMa(t, w, http.StatusOK)

	var tho map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &tho); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	for _, khoa := range []string{"reason", "receiving_body"} {
		if _, co := tho[khoa]; co {
			t.Errorf("khoá %q có mặt trên phiếu đang phân loại: %s", khoa, w.Body.String())
		}
	}
	if strings.Contains(w.Body.String(), "Điện lực") {
		t.Errorf("văn bản nhánh lọt ra phiếu không ở nhánh: %s", w.Body.String())
	}
}

// tuyenGhiNhanh is the two branch rows of the shared route table, so a case here cannot drift from
// the path and key the matrix proves.
func tuyenGhiNhanh() []caTuyen {
	var ra []caTuyen
	for _, ca := range caCacTuyen() {
		if strings.HasSuffix(ca.duong, "/rejection") || strings.HasSuffix(ca.duong, "/referral") {
			ra = append(ra, ca)
		}
	}
	return ra
}
