package http

// DELETE /api/v1/content-items/{id} beyond the four-case permission suite (noi_dung_mini_app_test.go
// carries it in cacTuyenND): what reaches the use case, and how each refusal is answered.

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

func TestDeleteContentItemPassesIDReasonAndStaffCodeAndAnswers204(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)

	w := m.goi(t, http.MethodDelete, hostA, duongNoiDung+"/nd-001", `{"reason":"Đăng nhầm bài"}`, canBo(xaA))
	doiMa(t, w, http.StatusNoContent)
	if w.Body.Len() != 0 {
		t.Errorf("204 phải không có thân: %q", w.Body.String())
	}
	// The actor is the staff BUSINESS CODE, never the internal id (rule 6, invariant 8).
	if m.ghi.idCuoi != "nd-001" || m.ghi.lastReason != "Đăng nhầm bài" || m.ghi.nguoi.ID != "CB-2026-7K3M9Q" {
		t.Errorf("tới use case: id %q, lý do %q, chủ thể %q", m.ghi.idCuoi, m.ghi.lastReason, m.ghi.nguoi.ID)
	}
	if m.ghi.xa != xaA {
		t.Errorf("use case chạy ở xã %q, muốn %q", m.ghi.xa, xaA)
	}
}

func TestDeleteContentItemRefusals(t *testing.T) {
	for name, tc := range map[string]struct {
		err    error
		status int
		code   string
	}{
		// The use case refuses blank / missing reasons (domain.ChuanHoaLyDoXoa); the handler answers 400.
		"blank reason": {domain.ErrThieuLyDoXoa, http.StatusBadRequest, "invalid_request"},
		"long reason":  {fmt.Errorf("%w (tối đa 500 ký tự)", domain.ErrLyDoXoaQuaDai), http.StatusBadRequest, "invalid_request"},
		// Already deleted, another commune's id, invented — ONE answer (rule 4, forbidden #2). The use
		// case wraps it with the commune; the handler still sees it with errors.Is and names no commune.
		"second delete / other commune": {fmt.Errorf("noi_dung_mini_app: xoá cho xã X: %w", commsstore.ErrNoiDungKhongTonTai),
			http.StatusNotFound, "not_found"},
		"store down": {errors.New("db down"), http.StatusInternalServerError, "internal"},
	} {
		t.Run(name, func(t *testing.T) {
			m := dungMayChuND(t)
			m.capQuyen(xaA, QuyenSuaNoiDung)
			m.ghi.loi = tc.err
			w := m.goi(t, http.MethodDelete, hostA, duongNoiDung+"/nd-001", `{"reason":"  "}`, canBo(xaA))
			doiMa(t, w, tc.status)
			e := loiTra(t, w)
			if e.Code != tc.code {
				t.Errorf("mã lỗi = %q, muốn %q", e.Code, tc.code)
			}
			// No internal detail, no catalogue wording, no commune id in what the client reads.
			for _, leak := range []string{"danh_muc", "xã X", "db down"} {
				if strings.Contains(e.Message, leak) {
					t.Errorf("câu trả lời lộ %q: %q", leak, e.Message)
				}
			}
		})
	}
}

func TestDeleteContentItemNonJSONBodyIs400BeforeTheUseCase(t *testing.T) {
	m := dungMayChuND(t)
	m.capQuyen(xaA, QuyenSuaNoiDung)
	w := m.goi(t, http.MethodDelete, hostA, duongNoiDung+"/nd-001", `lý do`, canBo(xaA))
	doiMa(t, w, http.StatusBadRequest)
	if m.ghi.deletes != 0 {
		t.Error("thân hỏng mà vẫn gọi use case")
	}
}
