package http

// POST /api/v1/citizen-reports/{maTraCuu}/photos — the officer attaches a scene photo to a petition THEY
// booked (owner decision 15/10/2026). The HTTP half of internal/app/staff_scene_photo.go.
//
// ON Handler (the staff mux), never HandlerCongDan: the citizen's upload of the same kind of photo is
// UploadPetitionPhoto on the citizen mux — two handlers, two Deps types (rule 4, invariant 5). The handler
// decides nothing: it reads the officer (principal.Ma, no fallback) and the `feedback.restricted` fact the
// use case takes, and translates the answer.

import (
	"context"
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// StaffScenePhotoActs is app.StaffScenePhotos.
type StaffScenePhotoActs interface {
	uploadLimiter
	Upload(ctx context.Context, ma string, req app.PhotoUploadRequest, body app.UploadBody, actor audit.Actor,
		restricted app.QuyenXemHanChe) (domain.StoredFile, error)
}

// UploadStaffScenePhoto serves POST /api/v1/citizen-reports/{maTraCuu}/photos — ONE multipart upload of
// ONE photo, answered with the STORED photo (photoOut, the citizen upload's reply shape). The part's file
// name is ignored (the photo is stored under domain.PetitionPhotoName).
func (h *Handler) UploadStaffScenePhoto(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheXuLy(w, r)
		return
	}
	answer := func(err error) { h.answerStaffScenePhotoError(w, r, "cán bộ tải ảnh hiện trường", err) }
	up, release, ok := receiveUpload(w, r, h.d.UploadSlots, h.d.Log, h.d.StaffScenePhotos,
		[]string{uploadFieldContentType}, answer)
	defer release()
	if !ok {
		return
	}
	ctx, cancel := context.WithDeadline(r.Context(), up.Deadline)
	defer cancel()
	f, err := h.d.StaffScenePhotos.Upload(ctx, r.PathValue("maTraCuu"),
		app.PhotoUploadRequest{ContentType: declaredType(up), Size: up.Size}, uploadBody{&up}, actor,
		h.coQuyenHanChe(ctx))
	if err != nil {
		answer(err)
		return
	}
	// LOGGED: the commune, the file id and the private key — NOT the lookup code, NOT the officer (rule 3).
	h.d.Log.Info("cán bộ đã đính ảnh hiện trường", "xa", string(tenant.MustFrom(ctx)), "tep_id", f.ID,
		"khoa_luu", f.ObjectKey)
	// A replay with the same Idempotency-Key is told the FILE ID — never a second file.
	idem.RecordCode(ctx, f.ID)
	vietJSON(w, http.StatusCreated, photoFromFile(f))
}

// answerStaffScenePhotoError maps one failure onto a status and a FIXED sentence an officer reads — never
// err.Error(), which carries the commune id. Default 500.
func (h *Handler) answerStaffScenePhotoError(w http.ResponseWriter, r *http.Request, what string, err error) {
	refused := fileRefusalLog(r, h.d.Log, "ảnh hiện trường (cán bộ): từ chối", what, err)
	ctx := r.Context()
	var rej *app.AttachmentRejection
	switch {
	case writeUploadEnvelopeError(w, err, refused):
	case errors.Is(err, petstore.ErrPhieuKhongTonTai), errors.Is(err, app.ErrPhieuHanChe):
		// One answer for no such code, another commune's, soft-deleted, and `can-bo` without the key.
		h.khongTimThay(w)
	case errors.Is(err, app.ErrPhotoNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy ảnh này trên phiếu.", "")
	case errors.Is(err, app.ErrPhotoTypeNotAllowed):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Chỉ nhận ảnh JPEG, PNG hoặc WebP.", "")
	case errors.Is(err, app.ErrPhotoTooLarge):
		refused("file_too_large")
		httpx.WriteError(w, http.StatusRequestEntityTooLarge, "file_too_large",
			"Ảnh lớn hơn dung lượng tối đa cho phép.", "")
	case errors.Is(err, app.ErrPhotoSizeInvalid):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Kích thước ảnh khai báo không hợp lệ.", "")
	case errors.As(err, &rej):
		sentence, ok := photoRejections[rej.Reason]
		if !ok {
			sentence = "Ảnh bị từ chối và không được lưu."
		}
		refused("photo_rejected")
		httpx.WriteError(w, http.StatusUnprocessableEntity, "photo_rejected", sentence, "")
	case errors.Is(err, app.ErrScenePhotoNotStaffBooked):
		refused("petition_not_staff_booked")
		httpx.WriteError(w, http.StatusConflict, "petition_not_staff_booked",
			"Chỉ đính ảnh hiện trường cho phiếu do cán bộ nhập hộ. Ảnh của phiếu người dân tự gửi do chính "+
				"người dân đính kèm.", "")
	case errors.Is(err, app.ErrPhotoWindowClosed):
		refused("petition_state")
		httpx.WriteError(w, http.StatusConflict, "petition_state",
			"Phiếu đã chuyển sang bước xử lý nên không đính thêm ảnh hiện trường được nữa.", "")
	case errors.Is(err, app.ErrPhotoCountReached):
		refused("photo_limit")
		httpx.WriteError(w, http.StatusConflict, "photo_limit", "Phiếu đã có đủ số ảnh hiện trường tối đa.", "")
	case errors.Is(err, app.ErrAttachmentNotPending):
		refused("photo_state")
		httpx.WriteError(w, http.StatusConflict, "photo_state",
			"Ảnh này đã bị từ chối. Hãy chọn ảnh và tải lên lại.", "")
	case errors.Is(err, app.ErrUploadChanged):
		refused("upload_changed")
		httpx.WriteError(w, http.StatusConflict, "upload_changed",
			"Ảnh vừa bị thay đổi trong lúc kiểm tra nên CHƯA được lưu. Hãy tải lên lại.", "")
	case writePhotoUnavailable(ctx, w, h.d.Log, what, err):
	default:
		h.d.Log.Error("ảnh hiện trường (cán bộ): lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "viec", what, "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}
