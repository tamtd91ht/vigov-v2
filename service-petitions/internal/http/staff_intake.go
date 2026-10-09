package http

// The STAFF intake — POST /api/v1/citizen-reports, "Nhập hộ phản ánh" (docs/ui-ux/09 §11). The act is
// app.StaffIntake; this file is the HTTP translation.
//
// FACTS THE BODY DOES NOT DECIDE, and a body naming one is REFUSED rather than silently dropped (the
// citizen intake's discipline, gui_phan_anh.go): whose petition it is (`citizen_id`, ADR 0028 §Bổ sung
// 2026-10-02 — none), the channel, the code, the status, the two deadlines. The scene location is
// refused too, as NOT ACCEPTED YET. The commune is not even declared (rule 1, forbidden #2 — the reason
// is on guiPhanAnhVao).
//
// THE RESIDENTIAL UNIT IS ACCEPTED as `residential_unit_id` (ADR 0088) and checked with identity before
// anything is written. The old spellings `hamlet` / `thon_id` stay REFUSED, naming the right field — a
// client sending them would otherwise believe they were recorded.

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// StaffIntakeBooker is the staff intake use case. *app.StaffIntake satisfies it.
type StaffIntakeBooker interface {
	Book(ctx context.Context, req app.StaffIntakeRequest, officer audit.Actor) (domain.PhieuPhanAnh, error)
}

// staffIntakeIn is the body of POST /api/v1/citizen-reports.
type staffIntakeIn struct {
	// Field is the tier-1 code from the commune's catalogue — REQUIRED on this channel (§11).
	Field string `json:"field"`

	// Content is what the citizen said, as the officer wrote it down. REQUIRED.
	Content string `json:"content"`

	// Address is the place, free text ("Đầu ngõ thôn Hà Lam"). Optional.
	Address string `json:"address,omitempty"`

	// ReporterName and ReporterPhone are what the officer typed — CITIZEN PERSONAL DATA (rule 3):
	// stored, never logged, masked in every response including this route's own 201.
	ReporterName  string `json:"reporter_name,omitempty"`
	ReporterPhone string `json:"reporter_phone,omitempty"`

	// Anonymous is "Người dân đề nghị gửi ẩn danh": hidden from staff screens; still recorded (ADR 0008).
	Anonymous bool `json:"anonymous,omitempty"`

	// ClockFrom is "Dân phản ánh lúc" (ADR 0028 F): an RFC 3339 instant WITH a zone, the moment the
	// citizen actually reported it. Absent = the booking instant. Earlier than 7 days before booking, or
	// later than booking, is REFUSED (400 `clock_from_out_of_range`), never clamped. THE SAME NAME AS
	// the response's `clock_from`, because it is the same column (`goc_dem_han`).
	ClockFrom *string `json:"clock_from,omitempty"`

	// ResidentialUnitID is §11's "Thôn, tổ dân phố" (ADR 0088): an id from GET /api/v1/residential-units.
	// OPTIONAL. Checked with identity BEFORE anything is written — not an active unit of this commune =
	// 400 `residential_unit_not_offered`; identity unreachable = 503 `residential_unit_check_unavailable`,
	// nothing written, no code issued. Stored as the id, never the name; never derived from coordinates.
	ResidentialUnitID string `json:"residential_unit_id,omitempty"`

	// --- refused, every one of them. The Vietnamese spellings are caught too: a refusal that only
	// catches the English one catches only the integrator who read the contract. ------------------

	CitizenID       *string  `json:"citizen_id,omitempty"`
	CitizenIDLegacy *string  `json:"cong_dan_id,omitempty"`
	FieldLegacy     *string  `json:"linh_vuc,omitempty"`
	Channel         *string  `json:"channel,omitempty"`
	Code            *string  `json:"code,omitempty"`
	Status          *string  `json:"status,omitempty"`
	AcknowledgeDue  *string  `json:"acknowledge_due,omitempty"`
	ResolveDue      *string  `json:"resolve_due,omitempty"`
	Hamlet          *string  `json:"hamlet,omitempty"`
	HamletLegacy    *string  `json:"thon_id,omitempty"`
	Lat             *float64 `json:"lat,omitempty"`
	Lng             *float64 `json:"lng,omitempty"`
}

func (v staffIntakeIn) notTheClients() bool {
	return v.CitizenID != nil || v.CitizenIDLegacy != nil || v.FieldLegacy != nil ||
		v.Channel != nil || v.Code != nil || v.Status != nil ||
		v.AcknowledgeDue != nil || v.ResolveDue != nil
}

// notAcceptedYet: the scene location, still not taken on this form.
func (v staffIntakeIn) notAcceptedYet() bool {
	return v.Lat != nil || v.Lng != nil
}

// oldResidentialUnitSpelling: `hamlet` / `thon_id` — the unit under a name this route does not read.
func (v staffIntakeIn) oldResidentialUnitSpelling() bool {
	return v.Hamlet != nil || v.HamletLegacy != nil
}

// BookStaffIntake serves POST /api/v1/citizen-reports.
//
// 201 is the petition as the staff detail renders it (phieuPhanAnhRa), MASKED: the officer just typed
// the name and number, but this body travels through the same proxies and screen caches as any other,
// and rule 3, invariant 3 has no "you just typed it" exception. `code` is what the officer reads to
// the citizen.
func (h *Handler) BookStaffIntake(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var in staffIntakeIn
	if !docThan(w, r, &in) {
		return
	}
	if in.notTheClients() {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Yêu cầu chứa thông tin do hệ thống tự xác định (người dân, kênh, mã tra cứu, trạng thái "+
				"hoặc thời hạn), hoặc ghi lĩnh vực bằng `linh_vuc` thay vì `field`.", "")
		return
	}
	if in.oldResidentialUnitSpelling() {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Thôn, tổ dân phố gửi bằng `residential_unit_id`, không phải `hamlet` hay `thon_id`.", "")
		return
	}
	if in.notAcceptedYet() {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Chưa nhận toạ độ ở biểu mẫu nhập hộ. Hãy ghi vị trí vào ô địa chỉ.", "")
		return
	}

	var clockFrom time.Time
	if in.ClockFrom != nil {
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(*in.ClockFrom))
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
				"`clock_from` phải là mốc thời gian RFC 3339 có múi giờ, ví dụ 2026-10-02T08:30:00+07:00.", "")
			return
		}
		clockFrom = t
	}

	officer, ok := nguoiThucHien(r)
	if !ok {
		// A wiring fault or an identity older than `ma`: no trail can name the officer, so no write
		// (rule 6, invariant 8). Never a fallback to the internal id.
		h.d.Log.Error("nhập hộ phản ánh: không có mã cán bộ trong phiên — không ghi được vết",
			"xa", string(tenant.MustFrom(ctx)))
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}

	p, err := h.d.StaffIntake.Book(ctx, app.StaffIntakeRequest{
		Content:       in.Content,
		Address:       in.Address,
		ReporterName:  in.ReporterName,
		ReporterPhone: in.ReporterPhone,
		Anonymous:     in.Anonymous,
		Field:         strings.TrimSpace(in.Field),
		ClockFrom:     clockFrom,

		ResidentialUnitID: in.ResidentialUnitID,
	}, officer)
	if err != nil {
		h.writeStaffIntakeError(w, r, err)
		return
	}

	// The code goes to idem BEFORE the response, so a retry with the same key is told this code rather
	// than a bare 409 (core/idem.RecordCode).
	idem.RecordCode(ctx, p.MaTraCuu)
	h.d.Log.Info("đã vào sổ phản ánh nhập hộ",
		"xa", string(tenant.MustFrom(ctx)), "ma_tra_cuu", p.MaTraCuu)

	// The label is read AFTER the commit: a failure here must not hide the code from the officer, who
	// has a citizen on the line. Empty label, logged — the citizen intake's call.
	label, err := h.nhanCuaLinhVuc(ctx, p.LinhVuc)
	if err != nil {
		h.d.Log.Error("đọc nhãn lĩnh vực sau khi nhập hộ: lỗi hệ thống — trả phiếu không nhãn",
			"xa", string(tenant.MustFrom(ctx)), "ma_tra_cuu", p.MaTraCuu, "err", err)
		label = ""
	}
	ra := phieuRaNgoai(p, label, false)
	// After the commit: a failed name lookup must not hide the code from the officer either.
	h.nameOneResidentialUnit(ctx, &ra)
	vietJSON(w, http.StatusCreated, ra)
}

// writeStaffIntakeError maps one failure onto a status and a sentence an officer can act on. The
// default is the narrow one (500), as in every mapping of this service.
func (h *Handler) writeStaffIntakeError(w http.ResponseWriter, r *http.Request, err error) {
	ctx := r.Context()
	switch {
	case errors.Is(err, app.ErrFieldNotOffered):
		writeFieldNotOffered(w)
	case errors.Is(err, app.ErrFieldCatalogueUnavailable):
		// Platform unreachable past the 60-second cache (ADR 0060 §3): nothing written, no code issued.
		h.d.Log.Warn("CẢNH BÁO: từ chối nhập hộ phản ánh vì chưa đọc được bộ mã lĩnh vực",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "field_catalogue_unavailable",
			"Chưa kiểm tra được lĩnh vực nên phiếu CHƯA được vào sổ. Vui lòng thử lại sau ít phút.", "")
	case errors.Is(err, app.ErrResidentialUnitNotActive):
		writeResidentialUnitNotOffered(w)
	case errors.Is(err, app.ErrResidentialUnitCheckUnavailable):
		// identity unreachable: nothing written, no code issued — never booked without the officer's unit.
		h.d.Log.Warn("CẢNH BÁO: từ chối nhập hộ phản ánh vì chưa kiểm được thôn / tổ dân phố",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		writeResidentialUnitCheckUnavailable(w)
	case errors.Is(err, domain.ErrGocDemHanNgoaiKhoang):
		httpx.WriteError(w, http.StatusBadRequest, "clock_from_out_of_range",
			"Thời điểm người dân phản ánh không được sớm hơn 7 ngày trước lúc vào sổ, và không được muộn "+
				"hơn lúc vào sổ. Hãy kiểm tra lại ô \"Dân phản ánh lúc\".", "")
	case app.IsStaffIntakeInputError(err):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
	case errors.Is(err, app.ErrChuaAnDinhDuocHan):
		h.d.Log.Warn("CẢNH BÁO: từ chối nhập hộ phản ánh vì chưa ấn định được hạn của xã",
			"xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "intake_not_configured",
			"Chưa ấn định được thời hạn xử lý theo cấu hình của xã nên phiếu CHƯA được vào sổ. "+
				"Hãy kiểm tra bảng thời hạn xử lý và lịch làm việc ở màn hình Cấu hình.", "")
	default:
		h.d.Log.Error("nhập hộ phản ánh: lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}
