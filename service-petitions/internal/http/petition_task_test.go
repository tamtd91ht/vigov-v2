package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// POST /api/v1/citizen-reports/{maTraCuu}/tasks. What the route does with a principal, a body and an
// answer; what the ACT decides (which petitions, which commune, one transaction) is pinned on the
// real stores in internal/app/petition_task_test.go.

// petitionTaskFake records what reached it, with the commune FROM THE CONTEXT.
type petitionTaskFake struct {
	calls      int
	commune    tenant.ID
	code       string
	yc         app.YeuCauTaoNhiemVu
	actor      audit.Actor
	restricted app.QuyenXemHanChe
	err        error
}

func (f *petitionTaskFake) CreateTask(ctx context.Context, code string, yc app.YeuCauTaoNhiemVu,
	actor audit.Actor, restricted app.QuyenXemHanChe) (domain.NhiemVu, error) {

	f.calls++
	f.commune = tenant.MustFrom(ctx)
	f.code, f.yc, f.actor, f.restricted = code, yc, actor, restricted
	if f.err != nil {
		return domain.NhiemVu{}, f.err
	}
	return domain.NhiemVu{ID: "nv-01", Ma: "NV07", TieuDe: yc.TieuDe, Loai: yc.Loai,
		NguonGiao: domain.NguonPhanAnh, NguonID: "pa-001", TrangThai: "moi-giao"}, nil
}

func petitionTaskPath(code string) string { return duong(code) + "/tasks" }

func petitionTaskBody() petitionTaskIn {
	return petitionTaskIn{AutoCode: true, Type: "co-ban", Title: "Dọn rác đầu ngõ thôn Hà Lam"}
}

// --- rule 5, invariant 7 --------------------------------------------------------------------------

func TestPetitionTask_NoSessionIs401(t *testing.T) {
	m := dungMayChu(t)
	w := m.goiGhiNV(t, http.MethodPost, hostA, petitionTaskPath(maPhieuThuong), nil, petitionTaskBody())
	doiMa(t, w, http.StatusUnauthorized)
	if m.petitionTasks.calls != 0 {
		t.Error("use case chạy dù chưa có phiên")
	}
}

// BOTH keys are required: each one alone is a 403, and the use case is never reached.
func TestPetitionTask_EachKeyAloneIs403(t *testing.T) {
	for ten, keys := range map[string][]authz.Perm{
		"thiếu task.create":   {"feedback.read", "feedback.resolve"},
		"thiếu feedback.read": {"task.create", "task.read"},
	} {
		t.Run(ten, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, keys...)
			w := m.goiGhiNV(t, http.MethodPost, hostA, petitionTaskPath(maPhieuThuong), canBoCuaXa(xaA),
				petitionTaskBody())
			doiMa(t, w, http.StatusForbidden)
			if m.petitionTasks.calls != 0 {
				t.Error("use case chạy dù thiếu một trong hai quyền")
			}
		})
	}
}

// 401 AND NOT 403 — this package's convention (TestTuyenXuLyDungQuyenSaiXaThi401): the commune on the
// principal is compared with the one resolved from Host BEFORE either key is consulted.
func TestPetitionTask_RightKeysWrongCommuneIs401(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, "task.create", "feedback.read")
	w := m.goiGhiNV(t, http.MethodPost, hostB, petitionTaskPath(maPhieuXaB), canBoCuaXa(xaA), petitionTaskBody())
	doiMa(t, w, http.StatusUnauthorized)
	if m.petitionTasks.calls != 0 {
		t.Error("phiên của xã A chạm được phiếu của xã B — rò rỉ giữa hai cơ quan nhà nước")
	}
}

func TestPetitionTask_BothKeysRightCommuneIs201(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, "task.create", "feedback.read")
	w := m.goiGhiNV(t, http.MethodPost, hostA, petitionTaskPath(maPhieuThuong), canBoCuaXa(xaA), petitionTaskBody())
	doiMa(t, w, http.StatusCreated)

	f := m.petitionTasks
	if f.calls != 1 || f.commune != xaA || f.code != maPhieuThuong {
		t.Fatalf("xuống use case: %d lần, xã %q, mã %q", f.calls, f.commune, f.code)
	}
	if f.actor.ID != maCanBo {
		t.Errorf("chủ thể = %q, muốn mã cán bộ %q (luật 6 bất biến 8)", f.actor.ID, maCanBo)
	}
	if f.yc.NguonGiao != "" || f.yc.NguonID != "" {
		t.Errorf("handler tự đặt nguồn (%q, %q) — việc đó của use case", f.yc.NguonGiao, f.yc.NguonID)
	}
	if f.yc.TieuDe != "Dọn rác đầu ngõ thôn Hà Lam" || f.yc.Loai != "co-ban" || !f.yc.TuSinhMa {
		t.Errorf("biểu mẫu xuống use case sai: %+v", f.yc)
	}
	if f.restricted {
		t.Error("báo có feedback.restricted dù tài khoản không giữ khoá đó")
	}
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("thân: %s", w.Body.String())
	}
	if out["code"] != "NV07" {
		t.Errorf("thân 201 thiếu mã nhiệm vụ vừa cấp: %s", w.Body.String())
	}
}

func TestPetitionTask_RestrictedKeyIsHandedDownAsAFact(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, "task.create", "feedback.read", QuyenHanChe)
	w := m.goiGhiNV(t, http.MethodPost, hostA, petitionTaskPath(maPhieuCanBo), canBoCuaXa(xaA), petitionTaskBody())
	doiMa(t, w, http.StatusCreated)
	if !m.petitionTasks.restricted {
		t.Error("giữ feedback.restricted mà use case không được báo")
	}
}

// --- the source is the server's ----------------------------------------------------------------------

func TestPetitionTask_ClientSentSourceIs400(t *testing.T) {
	for _, body := range []string{
		`{"auto_code":true,"type":"co-ban","title":"x","source":"phan-anh"}`,
		`{"auto_code":true,"type":"co-ban","title":"x","source_id":"01JPHIEUCUAXAKHAC00000000"}`,
		`{"auto_code":true,"type":"co-ban","title":"x","source":"","source_id":""}`,
	} {
		t.Run(body, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, "task.create", "feedback.read")
			w := m.goiGhiNVTho(t, http.MethodPost, hostA, petitionTaskPath(maPhieuThuong), canBoCuaXa(xaA), body)
			doiMa(t, w, http.StatusBadRequest)
			if m.petitionTasks.calls != 0 {
				t.Error("use case chạy dù thân tự chọn nguồn")
			}
			if e := loiTra(t, w); e.Code != "invalid_request" || !strings.Contains(e.Message, "source") {
				t.Errorf("câu từ chối = %+v", e)
			}
		})
	}
}

func TestPetitionTask_NotJSONObjectIs400(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, "task.create", "feedback.read")
	w := m.goiGhiNVTho(t, http.MethodPost, hostA, petitionTaskPath(maPhieuThuong), canBoCuaXa(xaA), `[1,2]`)
	doiMa(t, w, http.StatusBadRequest)
	if m.petitionTasks.calls != 0 {
		t.Error("use case chạy với thân không phải đối tượng JSON")
	}
}

func TestPetitionTask_MissingIdempotencyKeyIs400(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, "task.create", "feedback.read")
	w := m.goiThan(t, http.MethodPost, hostA, petitionTaskPath(maPhieuThuong), canBoCuaXa(xaA), petitionTaskBody())
	doiMa(t, w, http.StatusBadRequest)
	if m.petitionTasks.calls != 0 {
		t.Error("use case chạy dù thiếu Idempotency-Key — bấm hai lần sẽ sinh hai nhiệm vụ")
	}
}

// --- the answers --------------------------------------------------------------------------------------

func TestPetitionTask_RefusalsMapToTheirAnswers(t *testing.T) {
	wrapped := func(e error) error { return fmt.Errorf("nhiem_vu: giao việc mới cho xã %s: %w", xaA, e) }
	for ten, ca := range map[string]struct {
		err    error
		status int
		code   string
	}{
		"mã không có / xã khác / đã xoá": {wrapped(petstore.ErrPhieuKhongTonTai), http.StatusNotFound, "not_found"},
		"can-bo không có restricted":     {app.ErrPhieuHanChe, http.StatusNotFound, "not_found"},
		"phiếu đã đóng":                  {wrapped(domain.ErrPetitionClosedForTask), http.StatusConflict, "petition_state"},
		"mã nhiệm vụ đã cấp (của sổ NV)": {wrapped(petstore.ErrMaNhiemVuDaTonTai), http.StatusConflict, "code_taken"},
		"thiếu tiêu đề (của sổ NV)":      {domain.ErrThieuTieuDeNhiemVu, http.StatusBadRequest, "invalid_request"},
		"lỗi hệ thống":                   {errors.New("kho hỏng"), http.StatusInternalServerError, "internal"},
	} {
		t.Run(ten, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, "task.create", "feedback.read")
			m.petitionTasks.err = ca.err
			w := m.goiGhiNV(t, http.MethodPost, hostA, petitionTaskPath(maPhieuThuong), canBoCuaXa(xaA),
				petitionTaskBody())
			doiMa(t, w, ca.status)
			e := loiTra(t, w)
			if e.Code != ca.code {
				t.Errorf("mã lỗi = %q, muốn %q", e.Code, ca.code)
			}
			if strings.Contains(w.Body.String(), string(xaA)) || strings.Contains(w.Body.String(), "kho hỏng") {
				t.Errorf("thân lộ mã xã hoặc lỗi nội bộ: %s", w.Body.String())
			}
			if ca.status == http.StatusNotFound && e.Message != "Không tìm thấy phiếu phản ánh." {
				t.Errorf("404 không phải câu chung của sổ phiếu: %q", e.Message)
			}
		})
	}
}
