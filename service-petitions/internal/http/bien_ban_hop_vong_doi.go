package http

// The LIFECYCLE surface of the meeting-minutes register (user decisions 25/09/2026, migration 0012;
// URL nouns chosen by the user the same day).
//
//	PATCH  /api/v1/meetings/{id}                                  task.create   draft edit; notice once after signing
//	DELETE /api/v1/meetings/{id}                                  task.create   soft delete a draft, reason required
//	POST   /api/v1/meetings/{id}/signature                        task.approve  du-thao -> da-ky
//	PATCH  /api/v1/meetings/{id}/conclusions/{stt}                task.create   reword, draft, no live task
//	DELETE /api/v1/meetings/{id}/conclusions/{stt}                task.create   soft delete, draft, no live task
//	PUT    /api/v1/meetings/{id}/conclusions/{stt}/no-task-marker task.create   set "không phát sinh nhiệm vụ"
//	DELETE /api/v1/meetings/{id}/conclusions/{stt}/no-task-marker task.create   clear it, draft only
//
// NO KEY IS INVENTED (rule 5, invariant 3c). `task.approve` ("Duyệt hoàn thành") guards the one act
// that makes a record FINAL; everything else rides on `task.create`, the key of the write routes next
// door, for the reason bien_ban_hop_ghi.go gives. Whether a commune wants "may sign minutes" separate
// from "may approve task completion" is open question #27's to answer, not this file's.
//
// PATCH AND SIGNATURE ANSWER WITH THE DETAIL SHAPE, RE-READ AFTER THE COMMIT (DocBienBan's read), so
// the counters and conclusions on the reply are the register's, not a partial copy the use case held.

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"

	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// docThanTuyChon reads an OPTIONAL JSON body: an empty body is "nothing sent", not a 400. Everything
// else is docThan's rule — bounded, and malformed JSON refused.
func docThanTuyChon(w http.ResponseWriter, r *http.Request, vao any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, thanToiDa)
	if err := json.NewDecoder(r.Body).Decode(vao); err != nil && !errors.Is(err, io.EOF) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Nội dung gửi lên không phải JSON hợp lệ hoặc quá lớn.", "")
		return false
	}
	return true
}

// thongBaoTuVao turns the wire notice into the use case's. A day that does not parse is refused here
// with the domain's sentence; a missing one reaches domain.KiemThongBaoKetLuan as the zero time.
func thongBaoTuVao(v *thongBaoVao) (*app.ThongBaoKetLuan, error) {
	if v == nil {
		return nil, nil
	}
	tb := &app.ThongBaoKetLuan{SoKyHieu: v.ReferenceNo}
	if v.IssuedOn != "" {
		t, err := time.Parse("2006-01-02", v.IssuedOn)
		if err != nil {
			return nil, domain.ErrNgayThongBaoKhongDocDuoc
		}
		tb.Ngay = t
	}
	return tb, nil
}

// thuTuTuDuong parses `{stt}` — the same parse and sentence as the split route.
func thuTuTuDuong(w http.ResponseWriter, r *http.Request) (int, bool) {
	thuTu, err := strconv.Atoi(r.PathValue("stt"))
	if err != nil || thuTu < 1 {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Số thứ tự kết luận phải là một số nguyên dương.", "")
		return 0, false
	}
	return thuTu, true
}

// traChiTietSauGhi answers with the detail shape, read after the commit.
func (h *Handler) traChiTietSauGhi(w http.ResponseWriter, r *http.Request, id string) {
	b, err := h.d.DanhSachBienBan.TheoID(r.Context(), id)
	if errors.Is(err, petstore.ErrBienBanKhongTonTai) {
		h.khongTimThayBienBan(w)
		return
	}
	if err != nil {
		// THE WRITE HAS COMMITTED. Both routes that call this are safe to repeat (a repeat is a no-op
		// or a 409), so a client that retries on this 500 cannot write twice.
		h.d.Log.Error("đọc lại biên bản sau khi ghi: lỗi hệ thống",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	vietJSON(w, http.StatusOK, bienBanChiTietRaNgoai(b))
}

// SuaBienBan edits draft minutes, or records the notice once on signed minutes.
// PATCH /api/v1/meetings/{id}
func (h *Handler) SuaBienBan(w http.ResponseWriter, r *http.Request) {
	var vao suaBienBanVao
	if !docThan(w, r, &vao) {
		return
	}
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheNhiemVu(w, r)
		return
	}

	yc := app.YeuCauSuaBienBan{
		TenCuocHop: vao.Title,
		SoHieu:     vao.ReferenceNo,
		DiaDiem:    vao.Location,
		ChuTriMa:   vao.ChairedBy,
		ThuKyMa:    vao.Secretary,
		NoiDung:    vao.Content,
		ThanhPhan:  vao.Attendees,
	}
	if vao.HeldOn != nil {
		ngay, err := ngayHopVao(*vao.HeldOn)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
			return
		}
		yc.NgayHop = &ngay
	}
	tb, err := thongBaoTuVao(vao.Notice)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
		return
	}
	yc.ThongBao = tb

	id := r.PathValue("id")
	if _, err := h.d.GhiBienBan.SuaBienBan(r.Context(), id, yc, nguoi); err != nil {
		h.traLoiLoiBienBan(w, r, "sửa biên bản họp", err)
		return
	}
	h.traChiTietSauGhi(w, r, id)
}

// XoaBienBan soft deletes draft minutes. DELETE /api/v1/meetings/{id}
func (h *Handler) XoaBienBan(w http.ResponseWriter, r *http.Request) {
	var vao xoaBienBanVao
	if !docThan(w, r, &vao) {
		return
	}
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheNhiemVu(w, r)
		return
	}
	if err := h.d.GhiBienBan.XoaBienBan(r.Context(), r.PathValue("id"), vao.Reason, nguoi); err != nil {
		h.traLoiLoiBienBan(w, r, "xoá biên bản họp", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// KyBienBan signs draft minutes. POST /api/v1/meetings/{id}/signature
func (h *Handler) KyBienBan(w http.ResponseWriter, r *http.Request) {
	var vao kyBienBanVao
	if !docThanTuyChon(w, r, &vao) {
		return
	}
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheNhiemVu(w, r)
		return
	}
	tb, err := thongBaoTuVao(vao.Notice)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
		return
	}
	id := r.PathValue("id")
	if _, err := h.d.GhiBienBan.KyBienBan(r.Context(), id, app.YeuCauKyBienBan{ThongBao: tb}, nguoi); err != nil {
		h.traLoiLoiBienBan(w, r, "ký biên bản họp", err)
		return
	}
	h.traChiTietSauGhi(w, r, id)
}

// traKetLuanSauGhi answers with ONE conclusion as the detail read renders it — counters and derived
// status included — read after the commit. Re-read rather than built from the use case's copy: a
// no-op edit of a conclusion that HAS tasks writes nothing and holds no counters, and a reply saying
// `chua-giao` for it would be false.
func (h *Handler) traKetLuanSauGhi(w http.ResponseWriter, r *http.Request, bienBanID string, thuTu int) {
	b, err := h.d.DanhSachBienBan.TheoID(r.Context(), bienBanID)
	if err == nil {
		for _, k := range bienBanRaNgoai(b).Conclusions {
			if k.Ordinal == thuTu {
				vietJSON(w, http.StatusOK, k)
				return
			}
		}
		err = petstore.ErrKetLuanKhongTonTai
	}
	if errors.Is(err, petstore.ErrBienBanKhongTonTai) || errors.Is(err, petstore.ErrKetLuanKhongTonTai) {
		httpx.WriteError(w, http.StatusNotFound, "not_found",
			"Không tìm thấy kết luận này trong biên bản.", "")
		return
	}
	h.d.Log.Error("đọc lại kết luận sau khi ghi: lỗi hệ thống",
		"xa", string(tenant.MustFrom(r.Context())), "err", err)
	httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
}

// SuaKetLuan rewords one conclusion. PATCH /api/v1/meetings/{id}/conclusions/{stt}
func (h *Handler) SuaKetLuan(w http.ResponseWriter, r *http.Request) {
	var vao suaKetLuanVao
	if !docThan(w, r, &vao) {
		return
	}
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheNhiemVu(w, r)
		return
	}
	thuTu, ok := thuTuTuDuong(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	if _, err := h.d.GhiBienBan.SuaKetLuan(r.Context(), id, thuTu,
		app.YeuCauSuaKetLuan{NoiDung: vao.Content}, nguoi); err != nil {
		h.traLoiLoiBienBan(w, r, "sửa kết luận họp", err)
		return
	}
	h.traKetLuanSauGhi(w, r, id, thuTu)
}

// XoaKetLuan soft deletes one conclusion. DELETE /api/v1/meetings/{id}/conclusions/{stt}
func (h *Handler) XoaKetLuan(w http.ResponseWriter, r *http.Request) {
	var vao xoaBienBanVao
	if !docThan(w, r, &vao) {
		return
	}
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheNhiemVu(w, r)
		return
	}
	thuTu, ok := thuTuTuDuong(w, r)
	if !ok {
		return
	}
	if err := h.d.GhiBienBan.XoaKetLuan(r.Context(), r.PathValue("id"), thuTu, vao.Reason, nguoi); err != nil {
		h.traLoiLoiBienBan(w, r, "xoá kết luận họp", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DanhDauKhongPhatSinh sets the mark. PUT /api/v1/meetings/{id}/conclusions/{stt}/no-task-marker
// No body is read: the marker has no fields a client may decide (who and when are the server's).
func (h *Handler) DanhDauKhongPhatSinh(w http.ResponseWriter, r *http.Request) {
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheNhiemVu(w, r)
		return
	}
	thuTu, ok := thuTuTuDuong(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	if _, err := h.d.GhiBienBan.DanhDauKhongPhatSinh(r.Context(), id, thuTu, nguoi); err != nil {
		h.traLoiLoiBienBan(w, r, "đánh dấu không phát sinh nhiệm vụ", err)
		return
	}
	h.traKetLuanSauGhi(w, r, id, thuTu)
}

// BoDanhDauKhongPhatSinh clears the mark. DELETE /api/v1/meetings/{id}/conclusions/{stt}/no-task-marker
func (h *Handler) BoDanhDauKhongPhatSinh(w http.ResponseWriter, r *http.Request) {
	nguoi, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheNhiemVu(w, r)
		return
	}
	thuTu, ok := thuTuTuDuong(w, r)
	if !ok {
		return
	}
	if err := h.d.GhiBienBan.BoDanhDauKhongPhatSinh(r.Context(), r.PathValue("id"), thuTu, nguoi); err != nil {
		h.traLoiLoiBienBan(w, r, "bỏ dấu không phát sinh nhiệm vụ", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
