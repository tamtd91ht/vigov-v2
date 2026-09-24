package http

import (
	"net/http"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// A ROW THE MERGE REFUSES IS A 500, NEVER THE DEFAULTS.
//
// TestTrangThaiNhiemVu_DocLoiKhoLa500KhongTraMacDinh covers the store FAILING. This covers the store
// SUCCEEDING with a row the domain refuses (a code outside the seven — impossible under migration
// 0010's CHECK, so reaching it means the schema moved under this service). The handler has two error
// sources on one path; a refactor that checks only the first answers 200 with an empty or default
// board, and a commune that re-worded its statuses reads the shipped wording with nothing on screen
// saying so.
//
// MUTATION THAT MUST TURN THIS RED: in DanhSachTrangThaiNhiemVu, ignore GopNhanTrangThai's error
// (`gop, _ := domain.GopNhanTrangThai(ghiDe)`) and write 200.
func TestTrangThaiNhiemVu_DocDongMaLaLa500KhongTraMacDinh(t *testing.T) {
	m := dungMayChuGhi(t)
	m.docTT.theoXa[xaA] = append(m.docTT.theoXa[xaA],
		domain.NhanTrangThaiNhiemVu{Ma: domain.TrangThaiNhiemVu("da-huy"), Nhan: "Đã huỷ", ThuTu: 8, CapNhatBoi: "CB-00123"})

	w := m.goi(t, http.MethodGet, hostA, duongTrangThai, canBoGhi(xaA))
	doiMa(t, w, http.StatusInternalServerError)
	if body := w.Body.String(); strings.Contains(body, "Mới giao") || strings.Contains(body, "Đang treo") || strings.Contains(body, "da-huy") {
		t.Errorf("thân 500 trả nhãn (mặc định hoặc của xã) hoặc lộ mã lạ: %s", body)
	}
}
