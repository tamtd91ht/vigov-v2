package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// POST /api/v1/citizen-reports — "Nhập hộ phản ánh". What the ROUTE does with a principal, a body and
// an answer. What the ACT decides (deadlines, NULLs, the 7-day window, one transaction, the delta) is
// pinned in internal/app/staff_intake_test.go.

// staffIntakeFake records what reached it, with the commune FROM THE CONTEXT.
type staffIntakeFake struct {
	calls   int
	commune tenant.ID
	req     app.StaffIntakeRequest
	officer audit.Actor
	err     error
}

const staffIntakeCode = "PA-3KQM-7XRT-NWPD"

func (f *staffIntakeFake) Book(ctx context.Context, req app.StaffIntakeRequest, officer audit.Actor) (
	domain.PhieuPhanAnh, error) {
	f.calls++
	f.commune = tenant.MustFrom(ctx)
	f.req, f.officer = req, officer
	if f.err != nil {
		return domain.PhieuPhanAnh{}, f.err
	}
	booked := time.Date(2026, 10, 2, 2, 0, 0, 0, time.UTC)
	return domain.PhieuPhanAnh{
		ID: "pa-new", MaTraCuu: staffIntakeCode, Kenh: domain.KenhCanBoNhapHo,
		NoiDung: req.Content, LinhVuc: req.Field, DiaChi: req.Address,
		NguoiGuiHoTen: req.ReporterName, NguoiGuiDienThoai: req.ReporterPhone, AnDanh: req.Anonymous,
		TrangThai: domain.DaTiepNhan, GocDemHan: booked, VaoSoLuc: booked,
		HanXuLyXong:       time.Date(2026, 10, 9, 3, 30, 0, 0, time.UTC),
		PublicationStatus: domain.PublicationPending,
	}, nil
}

const staffIntakePath = "/api/v1/citizen-reports"

func staffIntakeBody() map[string]any {
	return map[string]any{
		"field":          "rac-thai",
		"content":        "Bà con gọi điện báo đống rác ở đầu ngõ đã ba ngày chưa ai dọn.",
		"address":        "Đầu ngõ thôn Hà Lam",
		"reporter_name":  "Nguyễn Văn An",
		"reporter_phone": "0900000000", // the agreed fake number (rule 3, invariant 5)
	}
}

// --- rule 5, invariant 7 --------------------------------------------------------------------------

func TestStaffIntake_NoSessionIs401(t *testing.T) {
	m := dungMayChu(t)
	w := m.goiGhiNV(t, http.MethodPost, hostA, staffIntakePath, nil, staffIntakeBody())
	doiMa(t, w, http.StatusUnauthorized)
	if m.staffIntake.calls != 0 {
		t.Error("use case chạy dù chưa có phiên")
	}
}

// `feedback.read` (and every other petition key) is not `feedback.create`.
func TestStaffIntake_WrongKeyIs403(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, "feedback.read", "feedback.classify", "feedback.assign", "feedback.resolve")
	w := m.goiGhiNV(t, http.MethodPost, hostA, staffIntakePath, canBoCuaXa(xaA), staffIntakeBody())
	doiMa(t, w, http.StatusForbidden)
	if m.staffIntake.calls != 0 {
		t.Error("use case chạy dù thiếu feedback.create")
	}
}

// 401 AND NOT 403 — the package's convention: the commune on the principal is compared with the one
// resolved from Host BEFORE the key is consulted.
func TestStaffIntake_RightKeyWrongCommuneIs401(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, "feedback.create")
	w := m.goiGhiNV(t, http.MethodPost, hostB, staffIntakePath, canBoCuaXa(xaA), staffIntakeBody())
	doiMa(t, w, http.StatusUnauthorized)
	if m.staffIntake.calls != 0 {
		t.Error("phiên của xã A vào sổ được phiếu cho xã B — rò rỉ giữa hai cơ quan nhà nước")
	}
}

func TestStaffIntake_RightKeyRightCommuneIs201(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, "feedback.create")
	w := m.goiGhiNV(t, http.MethodPost, hostA, staffIntakePath, canBoCuaXa(xaA), staffIntakeBody())
	doiMa(t, w, http.StatusCreated)

	f := m.staffIntake
	if f.calls != 1 || f.commune != xaA {
		t.Fatalf("xuống use case %d lần, xã %q", f.calls, f.commune)
	}
	if f.officer.ID != maCanBo || f.officer.Kind != "staff" || f.officer.IP == "" {
		t.Errorf("chủ thể = %+v, muốn mã cán bộ %q, kind staff, có IP (luật 6 bất biến 2, 8)", f.officer, maCanBo)
	}
	if f.req.Field != "rac-thai" || f.req.Content == "" || f.req.Address != "Đầu ngõ thôn Hà Lam" ||
		f.req.ReporterName != "Nguyễn Văn An" || f.req.ReporterPhone != "0900000000" || f.req.Anonymous {
		t.Errorf("biểu mẫu xuống use case sai: %+v", f.req)
	}
	if !f.req.ClockFrom.IsZero() {
		t.Errorf("không gửi clock_from mà use case nhận %v — mặc định là của use case (lúc vào sổ)", f.req.ClockFrom)
	}

	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("thân: %s", w.Body.String())
	}
	if out["code"] != staffIntakeCode {
		t.Errorf("thân 201 thiếu mã tra cứu để cán bộ đưa người dân: %s", w.Body.String())
	}
	if out["channel"] != string(domain.KenhCanBoNhapHo) {
		t.Errorf("channel = %v", out["channel"])
	}
	// "KHÔNG ÁP DỤNG" travels as null, never a time and never absent.
	if v, ok := out["acknowledge_due"]; !ok || v != nil {
		t.Errorf("acknowledge_due = %v (có=%v), muốn null", v, ok)
	}
	if out["resolve_due"] == nil {
		t.Error("resolve_due rỗng — kênh nhập hộ ấn định hạn xử lý ngay lúc vào sổ")
	}
	if out["has_citizen"] != false {
		t.Errorf("has_citizen = %v, muốn false — phiếu nhập hộ không gắn tài khoản nào", out["has_citizen"])
	}
	if out["field_label"] != "Rác thải – Vệ sinh môi trường" {
		t.Errorf("field_label = %v", out["field_label"])
	}
	// MASKED, though the officer just typed them (rule 3, invariant 3).
	if strings.Contains(w.Body.String(), "0900000000") || strings.Contains(w.Body.String(), "Nguyễn Văn An") {
		t.Errorf("thân 201 mang họ tên / số điện thoại đầy đủ: %s", w.Body.String())
	}
}

func TestStaffIntake_ClockFromReachesUseCase(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, "feedback.create")
	b := staffIntakeBody()
	b["clock_from"] = "2026-09-30T08:15:00+07:00"
	doiMa(t, m.goiGhiNV(t, http.MethodPost, hostA, staffIntakePath, canBoCuaXa(xaA), b), http.StatusCreated)
	if want := time.Date(2026, 9, 30, 1, 15, 0, 0, time.UTC); !m.staffIntake.req.ClockFrom.Equal(want) {
		t.Errorf("clock_from xuống use case = %v, muốn %v", m.staffIntake.req.ClockFrom, want)
	}
}

func TestStaffIntake_MissingIdempotencyKeyIs400(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, "feedback.create")
	w := m.goiThan(t, http.MethodPost, hostA, staffIntakePath, canBoCuaXa(xaA), staffIntakeBody())
	doiMa(t, w, http.StatusBadRequest)
	if m.staffIntake.calls != 0 {
		t.Error("use case chạy dù thiếu Idempotency-Key — bấm hai lần sẽ vào sổ hai phiếu, hai mã")
	}
}

// --- the body ------------------------------------------------------------------------------------

func TestStaffIntake_SystemDecidedFieldsAre400(t *testing.T) {
	for _, k := range []string{"citizen_id", "cong_dan_id", "linh_vuc", "channel", "code", "status",
		"acknowledge_due", "resolve_due"} {
		t.Run(k, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, "feedback.create")
			b := staffIntakeBody()
			b[k] = "x"
			w := m.goiGhiNV(t, http.MethodPost, hostA, staffIntakePath, canBoCuaXa(xaA), b)
			doiMa(t, w, http.StatusBadRequest)
			if m.staffIntake.calls != 0 {
				t.Errorf("use case chạy dù thân tự đặt %q", k)
			}
		})
	}
}

// The hamlet and the coordinates are NOT ACCEPTED YET — refused so a client does not believe they were
// recorded (no identity RPC validates a hamlet; routes.go).
func TestStaffIntake_HamletAndCoordinatesAre400(t *testing.T) {
	for k, v := range map[string]any{"hamlet": "thon-ha-lam", "thon_id": "thon-ha-lam", "lat": 15.5, "lng": 108.2} {
		t.Run(k, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, "feedback.create")
			b := staffIntakeBody()
			b[k] = v
			w := m.goiGhiNV(t, http.MethodPost, hostA, staffIntakePath, canBoCuaXa(xaA), b)
			doiMa(t, w, http.StatusBadRequest)
			if m.staffIntake.calls != 0 {
				t.Errorf("use case chạy dù thân gửi %q", k)
			}
		})
	}
}

func TestStaffIntake_ClockFromNotRFC3339Is400(t *testing.T) {
	for _, v := range []string{"2026-09-30", "2026-09-30T08:15:00", "hôm qua"} {
		t.Run(v, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, "feedback.create")
			b := staffIntakeBody()
			b["clock_from"] = v
			w := m.goiGhiNV(t, http.MethodPost, hostA, staffIntakePath, canBoCuaXa(xaA), b)
			doiMa(t, w, http.StatusBadRequest)
			if m.staffIntake.calls != 0 {
				t.Error("use case chạy với clock_from không đọc được")
			}
		})
	}
}

// No business code on the principal: no trail can name the officer, so no write and no fallback.
func TestStaffIntake_OfficerWithoutCodeIs500(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, "feedback.create")
	p := canBoCuaXa(xaA)
	p.Ma = ""
	w := m.goiGhiNV(t, http.MethodPost, hostA, staffIntakePath, p, staffIntakeBody())
	doiMa(t, w, http.StatusInternalServerError)
	if m.staffIntake.calls != 0 {
		t.Error("use case chạy dù không có mã cán bộ cho vết")
	}
}

// The label is read after the commit: its failure must not hide the code from the officer.
func TestStaffIntake_LabelFailureStillReturnsCode(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, "feedback.create")
	m.nhan.loi = errors.New("platform không trả lời")
	w := m.goiGhiNV(t, http.MethodPost, hostA, staffIntakePath, canBoCuaXa(xaA), staffIntakeBody())
	doiMa(t, w, http.StatusCreated)
	if !strings.Contains(w.Body.String(), staffIntakeCode) {
		t.Errorf("lỗi đọc nhãn làm mất mã tra cứu: %s", w.Body.String())
	}
}

// --- the answers ---------------------------------------------------------------------------------

func TestStaffIntake_RefusalsMapToTheirAnswers(t *testing.T) {
	wrapped := func(e error) error { return fmt.Errorf("nhap_ho_phan_anh: x cho xã %s: %w", xaA, e) }
	for name, c := range map[string]struct {
		err    error
		status int
		code   string
	}{
		"lĩnh vực xã không nhận":    {app.ErrFieldNotOffered, http.StatusBadRequest, "field_not_offered"},
		"chưa đọc được bộ mã":       {wrapped(app.ErrFieldCatalogueUnavailable), http.StatusServiceUnavailable, "field_catalogue_unavailable"},
		"mốc ngoài khoảng 7 ngày":   {fmt.Errorf("%w: sớm hơn 7 ngày", domain.ErrGocDemHanNgoaiKhoang), http.StatusBadRequest, "clock_from_out_of_range"},
		"thiếu nội dung":            {domain.ErrNoiDungTrong, http.StatusBadRequest, "invalid_request"},
		"thiếu lĩnh vực":            {domain.ErrThieuLinhVuc, http.StatusBadRequest, "invalid_request"},
		"lĩnh vực sai dạng":         {domain.ErrLinhVucSaiDang, http.StatusBadRequest, "invalid_request"},
		"số điện thoại quá dài":     {domain.ErrDienThoaiQuaDai, http.StatusBadRequest, "invalid_request"},
		"chưa ấn định được hạn":     {wrapped(app.ErrChuaAnDinhDuocHan), http.StatusServiceUnavailable, "intake_not_configured"},
		"lỗi hệ thống":              {errors.New("kho hỏng"), http.StatusInternalServerError, "internal"},
		"chủ thể không phải cán bộ": {errors.New("xu_ly_phan_anh: chủ thể không phải cán bộ"), http.StatusInternalServerError, "internal"},
	} {
		t.Run(name, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, "feedback.create")
			m.staffIntake.err = c.err
			w := m.goiGhiNV(t, http.MethodPost, hostA, staffIntakePath, canBoCuaXa(xaA), staffIntakeBody())
			doiMa(t, w, c.status)
			if e := loiTra(t, w); e.Code != c.code {
				t.Errorf("mã lỗi = %q, muốn %q", e.Code, c.code)
			}
			for _, cam := range []string{string(xaA), "kho hỏng", "0900000000", "Nguyễn Văn An"} {
				if strings.Contains(w.Body.String(), cam) {
					t.Errorf("thân lỗi lộ %q: %s", cam, w.Body.String())
				}
			}
			if strings.Contains(w.Body.String(), "PA-") {
				t.Errorf("thân lỗi mang một mã tra cứu — lượt vào sổ đã hỏng không được cấp mã: %s", w.Body.String())
			}
		})
	}
}
