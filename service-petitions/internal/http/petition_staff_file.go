package http

// The STAFF files on a petition — the HTTP half of internal/app/petition_verification_photo.go and
// internal/app/petition_log_attachment.go (migration 0027, owner decisions of 02/10/2026):
//
//	POST /api/v1/citizen-reports/{maTraCuu}/verification-photos                    feedback.resolve
//	POST /api/v1/citizen-reports/{maTraCuu}/verification-photos/{id}/completion    feedback.resolve
//	GET  /api/v1/citizen-reports/{maTraCuu}/verification-photos                    feedback.read (audited)
//	POST /api/v1/citizen-reports/{maTraCuu}/log-attachments                        feedback.read + app.duocGhiChu
//	POST /api/v1/citizen-reports/{maTraCuu}/log-attachments/{id}/completion        feedback.read + app.duocGhiChu
//	GET  /api/v1/citizen-reports/{maTraCuu}/log-attachments/{id}/download          feedback.read (audited)
//
// THESE HANDLERS DECIDE NOTHING: they read the permission FACTS the use cases take (feedback.restricted;
// the three commune-wide petition keys for the note rule) and translate the answer. The citizen's read
// of verification photos is on HandlerCongDan (ListMyVerificationPhotos), never here.
//
// ⚠ REPLIES CARRY BEARER CREDENTIALS (presigned POST forms, presigned GET URLs): `Cache-Control:
// no-store`, never logged — nor the lookup code, nor a file name.

import (
	"context"
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// StaffVerificationPhotoActs is app.StaffVerificationPhotos.
type StaffVerificationPhotoActs interface {
	RequestUpload(ctx context.Context, ma string, req app.PhotoUploadRequest, actor audit.Actor,
		restricted app.QuyenXemHanChe) (app.PhotoUpload, error)
	Complete(ctx context.Context, ma, id string, actor audit.Actor, restricted app.QuyenXemHanChe) (
		domain.StoredFile, error)
	// reader is the staff member, ID = their business code (principal.Ma): the use case audits the read.
	ListPhotos(ctx context.Context, ma string, mayReadRestricted bool, reader audit.Actor) ([]app.PhotoLink, error)
}

// PetitionLogAttachmentActs is app.PetitionLogAttachments.
type PetitionLogAttachmentActs interface {
	RequestUpload(ctx context.Context, ma string, req app.AttachmentUploadRequest, actor audit.Actor,
		noteRight app.QuyenGhiChuCaXa, restricted app.QuyenXemHanChe) (app.AttachmentUpload, error)
	Complete(ctx context.Context, ma, id string, actor audit.Actor, noteRight app.QuyenGhiChuCaXa,
		restricted app.QuyenXemHanChe) (domain.StoredFile, error)
	DownloadLink(ctx context.Context, ma, id string, reader audit.Actor, restricted app.QuyenXemHanChe) (
		app.AttachmentDownload, error)
}

// PetitionLogAttachmentReader reads the attachments of ONE PAGE of petition timeline entries in one
// statement. *petstore.StoredFileStore satisfies it. STAFF-ONLY.
type PetitionLogAttachmentReader interface {
	PetitionAttachmentsByLogEntries(ctx context.Context, logEntryIDs []string) (
		map[string][]domain.PetitionLogAttachment, error)
}

// --- verification photos ------------------------------------------------------------------------------

// RequestVerificationPhotoUpload serves POST /api/v1/citizen-reports/{maTraCuu}/verification-photos.
func (h *Handler) RequestVerificationPhotoUpload(w http.ResponseWriter, r *http.Request) {
	var in photoUploadIn
	if !docThan(w, r, &in) {
		return
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheXuLy(w, r)
		return
	}
	ctx := r.Context()
	up, err := h.d.VerificationPhotos.RequestUpload(ctx, r.PathValue("maTraCuu"),
		app.PhotoUploadRequest{ContentType: in.ContentType, Size: in.Size}, actor, h.coQuyenHanChe(ctx))
	if err != nil {
		h.answerVerificationPhotoError(w, r, "xin tải ảnh sau xử lý", err)
		return
	}
	// A replay with the same Idempotency-Key is told the FILE ID — never the form (a bearer credential).
	idem.RecordCode(ctx, up.File.ID)
	noStore(w)
	vietJSON(w, http.StatusCreated, photoUploadOut{
		Photo:  photoFromFile(up.File),
		Upload: presignedUploadOut{URL: up.Post.URL, Fields: up.Post.Fields, ExpiresAt: up.Post.ExpiresAt},
	})
}

// CompleteVerificationPhoto serves POST …/verification-photos/{id}/completion.
func (h *Handler) CompleteVerificationPhoto(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheXuLy(w, r)
		return
	}
	ctx := r.Context()
	f, err := h.d.VerificationPhotos.Complete(ctx, r.PathValue("maTraCuu"), r.PathValue("id"), actor,
		h.coQuyenHanChe(ctx))
	if err != nil {
		h.answerVerificationPhotoError(w, r, "hoàn tất ảnh sau xử lý", err)
		return
	}
	vietJSON(w, http.StatusOK, photoFromFile(f))
}

// ListVerificationPhotos serves GET /api/v1/citizen-reports/{maTraCuu}/verification-photos — the
// ListPetitionPhotos shape: `feedback.read` at the gate, `feedback.restricted` decides a `can-bo`
// petition (404 without it), the trail's "who" is `principal.Ma` with no fallback.
func (h *Handler) ListVerificationPhotos(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ma := r.PathValue("maTraCuu")
	if ma == "" {
		h.khongTimThay(w)
		return
	}
	principal, ok := authz.From(ctx)
	mayRestricted := ok && h.d.Checker.Allows(ctx, principal, QuyenHanChe)
	reader := audit.Actor{Kind: principal.Kind, IP: httpx.ClientIP(r)}
	if ok {
		reader.ID = principal.Ma
	}
	links, err := h.d.VerificationPhotos.ListPhotos(ctx, ma, mayRestricted, reader)
	switch {
	case err == nil:
		noStore(w)
		vietJSON(w, http.StatusOK, photoListFrom(links))
	case errors.Is(err, petstore.ErrPhieuKhongTonTai):
		h.khongTimThay(w)
	case writePhotoUnavailable(ctx, w, h.d.Log, "đọc ảnh sau xử lý (cán bộ)", err):
	default:
		h.d.Log.Error("đọc ảnh sau xử lý: lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}

// verificationPhotoRejections is the sentence an OFFICER reads per refusal of the photo (422).
var verificationPhotoRejections = map[string]string{
	app.RejectMalware:        "Ảnh bị từ chối vì phát hiện mã độc và không được lưu.",
	app.RejectTypeNotAllowed: "Ảnh bị từ chối: tệp không phải ảnh JPEG, PNG hoặc WebP.",
	app.RejectTypeMismatch:   "Ảnh bị từ chối: nội dung tệp không đúng loại ảnh đã khai báo khi tải lên.",
	app.RejectTooLarge:       "Ảnh bị từ chối: ảnh lớn hơn dung lượng tối đa cho phép.",
	app.RejectCountReached:   "Ảnh bị từ chối: phiếu đã có đủ số ảnh sau xử lý tối đa.",
	app.RejectUndecodable:    "Ảnh bị từ chối: không đọc được ảnh. Hãy chụp hoặc chọn ảnh khác.",
	app.RejectTooManyPixels:  "Ảnh bị từ chối: ảnh có độ phân giải quá lớn để xử lý.",
}

// answerVerificationPhotoError maps one failure onto a status and a FIXED sentence — never err.Error(),
// which carries the commune id. Default 500.
func (h *Handler) answerVerificationPhotoError(w http.ResponseWriter, r *http.Request, what string, err error) {
	ctx := r.Context()
	var rej *app.AttachmentRejection
	switch {
	case errors.Is(err, petstore.ErrPhieuKhongTonTai), errors.Is(err, app.ErrPhieuHanChe):
		// One answer for no such code, another commune's, soft-deleted, and `can-bo` without the key.
		h.khongTimThay(w)
	case errors.Is(err, app.ErrVerificationPhotoNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy ảnh này trên phiếu.", "")
	case errors.Is(err, app.ErrPhotoTypeNotAllowed):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Chỉ nhận ảnh JPEG, PNG hoặc WebP.", "")
	case errors.Is(err, app.ErrPhotoTooLarge):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Ảnh lớn hơn dung lượng tối đa cho phép.", "")
	case errors.Is(err, app.ErrPhotoSizeInvalid):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Kích thước ảnh khai báo không hợp lệ.", "")
	case errors.As(err, &rej):
		sentence, ok := verificationPhotoRejections[rej.Reason]
		if !ok {
			sentence = "Ảnh bị từ chối và không được lưu."
		}
		httpx.WriteError(w, http.StatusUnprocessableEntity, "photo_rejected", sentence, "")
	case errors.Is(err, app.ErrVerificationPhotoWindowClosed):
		httpx.WriteError(w, http.StatusConflict, "petition_state",
			"Phiếu đã kết thúc nên không thêm ảnh sau xử lý được nữa.", "")
	case errors.Is(err, app.ErrVerificationPhotoCountReached):
		httpx.WriteError(w, http.StatusConflict, "photo_limit", "Phiếu đã có đủ số ảnh sau xử lý tối đa.", "")
	case errors.Is(err, app.ErrAttachmentNotPending):
		httpx.WriteError(w, http.StatusConflict, "photo_state",
			"Ảnh này đã bị từ chối hoặc lượt tải đã hết hạn. Hãy chọn ảnh và tải lên lại.", "")
	case errors.Is(err, app.ErrUploadNotReceived):
		httpx.WriteError(w, http.StatusConflict, "upload_not_received",
			"Chưa nhận được ảnh. Hãy chờ tải lên xong rồi bấm hoàn tất lại.", "")
	case errors.Is(err, app.ErrUploadExpired):
		httpx.WriteError(w, http.StatusConflict, "upload_expired",
			"Lượt tải lên đã hết hạn mà chưa nhận được ảnh. Hãy chọn ảnh và tải lên lại.", "")
	case errors.Is(err, app.ErrUploadChanged):
		httpx.WriteError(w, http.StatusConflict, "upload_changed",
			"Ảnh vừa bị thay đổi trong lúc kiểm tra. Hãy bấm hoàn tất lại.", "")
	case writePhotoUnavailable(ctx, w, h.d.Log, what, err):
	default:
		h.d.Log.Error("ảnh sau xử lý: lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "viec", what, "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}

// --- log attachments ----------------------------------------------------------------------------------

// RequestPetitionLogAttachmentUpload serves POST /api/v1/citizen-reports/{maTraCuu}/log-attachments.
func (h *Handler) RequestPetitionLogAttachmentUpload(w http.ResponseWriter, r *http.Request) {
	var in taskAttachmentUploadIn
	if !docThan(w, r, &in) {
		return
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheXuLy(w, r)
		return
	}
	ctx := r.Context()
	up, err := h.d.PetitionLogAttachments.RequestUpload(ctx, r.PathValue("maTraCuu"), app.AttachmentUploadRequest{
		FileName: in.FileName, ContentType: in.ContentType, Size: in.Size,
	}, actor, h.coQuyenGhiChuCaXa(ctx), h.coQuyenHanChe(ctx))
	if err != nil {
		h.answerPetitionLogAttachmentError(w, r, "xin tải tệp đính kèm nhật ký", err)
		return
	}
	idem.RecordCode(ctx, up.File.ID)
	noStore(w)
	vietJSON(w, http.StatusCreated, taskAttachmentUploadOut{
		Attachment: taskAttachmentFromFile(up.File),
		Upload:     presignedUploadOut{URL: up.Post.URL, Fields: up.Post.Fields, ExpiresAt: up.Post.ExpiresAt},
	})
}

// CompletePetitionLogAttachment serves POST …/log-attachments/{id}/completion.
func (h *Handler) CompletePetitionLogAttachment(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheXuLy(w, r)
		return
	}
	ctx := r.Context()
	f, err := h.d.PetitionLogAttachments.Complete(ctx, r.PathValue("maTraCuu"), r.PathValue("id"), actor,
		h.coQuyenGhiChuCaXa(ctx), h.coQuyenHanChe(ctx))
	if err != nil {
		h.answerPetitionLogAttachmentError(w, r, "hoàn tất tệp đính kèm nhật ký", err)
		return
	}
	vietJSON(w, http.StatusOK, taskAttachmentFromFile(f))
}

// PetitionLogAttachmentDownload serves GET …/log-attachments/{id}/download.
func (h *Handler) PetitionLogAttachmentDownload(w http.ResponseWriter, r *http.Request) {
	reader, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheXuLy(w, r)
		return
	}
	ctx := r.Context()
	d, err := h.d.PetitionLogAttachments.DownloadLink(ctx, r.PathValue("maTraCuu"), r.PathValue("id"), reader,
		h.coQuyenHanChe(ctx))
	if err != nil {
		h.answerPetitionLogAttachmentError(w, r, "tải tệp đính kèm nhật ký", err)
		return
	}
	noStore(w)
	vietJSON(w, http.StatusOK, taskAttachmentDownloadOut{URL: d.URL.URL(), ExpiresAt: d.ExpiresAt})
}

// petitionLogRejectionSentences is the officer-facing sentence per app.Reject* reason.
var petitionLogRejectionSentences = map[string]string{
	app.RejectMalware:        "Tệp bị từ chối vì phát hiện mã độc và không được lưu.",
	app.RejectTypeNotAllowed: "Tệp bị từ chối: nội dung tệp không thuộc loại được phép đính kèm.",
	app.RejectTypeMismatch:   "Tệp bị từ chối: nội dung tệp không đúng loại tệp đã khai báo khi tải lên.",
	app.RejectTooLarge:       "Tệp bị từ chối: tệp lớn hơn dung lượng tối đa cho phép.",
	app.RejectCountReached:   "Tệp bị từ chối: phiếu đã có đủ số tệp đính kèm tối đa.",
}

// answerPetitionLogAttachmentError maps the attachment refusals, then hands everything else — the
// petition's 404, the note rule's 403, the 500 — to traLoiLoiXuLy, so a petition route answers one way.
// Also used by GhiChuPhieu, whose two attachment refusals are 400s.
func (h *Handler) answerPetitionLogAttachmentError(w http.ResponseWriter, r *http.Request, what string, err error) {
	ctx := r.Context()
	var rej *app.AttachmentRejection
	switch {
	case errors.Is(err, app.ErrAttachmentNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy tệp đính kèm này trên phiếu.", "")
	case errors.Is(err, domain.ErrAttachmentNameInvalid), errors.Is(err, domain.ErrAttachmentSizeInvalid),
		errors.Is(err, domain.ErrAttachmentListInvalid), errors.Is(err, domain.ErrPetitionAttachmentNotUsable):
		// The domain's own sentence (cauTuChoi, never err.Error(): a refusal raised inside the
		// transaction arrives wrapped by app.bocPhieu with the commune id).
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", cauTuChoi(err,
			domain.ErrAttachmentNameInvalid, domain.ErrAttachmentSizeInvalid,
			domain.ErrAttachmentListInvalid, domain.ErrPetitionAttachmentNotUsable), "")
	case errors.Is(err, app.ErrAttachmentTypeNotAllowed):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Loại tệp này không được phép đính kèm vào nhật ký xử lý phiếu.", "")
	case errors.Is(err, app.ErrAttachmentTooLarge):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Tệp lớn hơn dung lượng tối đa được phép đính kèm.", "")
	case errors.As(err, &rej):
		sentence, ok := petitionLogRejectionSentences[rej.Reason]
		if !ok {
			sentence = "Tệp bị từ chối và không được lưu."
		}
		httpx.WriteError(w, http.StatusUnprocessableEntity, "attachment_rejected", sentence, "")
	case errors.Is(err, app.ErrPetitionLogAttachmentCountReached):
		httpx.WriteError(w, http.StatusConflict, "attachment_limit",
			"Phiếu đã có đủ số tệp đính kèm tối đa được phép.", "")
	case errors.Is(err, app.ErrAttachmentNotPending):
		httpx.WriteError(w, http.StatusConflict, "attachment_state",
			"Tệp này đã bị từ chối hoặc lượt tải đã hết hạn. Hãy chọn tệp và tải lên lại.", "")
	case errors.Is(err, app.ErrUploadNotReceived):
		httpx.WriteError(w, http.StatusConflict, "upload_not_received",
			"Chưa nhận được tệp. Hãy chờ tải lên xong rồi bấm hoàn tất lại.", "")
	case errors.Is(err, app.ErrUploadExpired):
		httpx.WriteError(w, http.StatusConflict, "upload_expired",
			"Lượt tải lên đã hết hạn mà chưa nhận được tệp. Hãy chọn tệp và tải lên lại.", "")
	case errors.Is(err, app.ErrUploadChanged):
		httpx.WriteError(w, http.StatusConflict, "upload_changed",
			"Tệp vừa bị thay đổi trong lúc kiểm tra. Hãy bấm hoàn tất lại.", "")
	case errors.Is(err, app.ErrUploadNotConfigured), errors.Is(err, app.ErrUploadLimitsUnavailable),
		errors.Is(err, app.ErrScanUnavailable):
		h.d.Log.Warn("CẢNH BÁO: tệp đính kèm nhật ký phiếu chưa xử lý được (kho / máy quét / giới hạn)",
			"xa", string(tenant.MustFrom(ctx)), "viec", what, "err", err)
		code, sentence := "storage_not_configured",
			"Chưa cấu hình kho lưu tệp nên chưa đính kèm được tệp. Hãy báo quản trị hệ thống."
		switch {
		case errors.Is(err, app.ErrUploadLimitsUnavailable):
			code, sentence = "upload_limits_unavailable",
				"Chưa đọc được giới hạn tải tệp nên tệp CHƯA được nhận. Vui lòng thử lại sau ít phút."
		case errors.Is(err, app.ErrScanUnavailable):
			code, sentence = "malware_scan_unavailable",
				"Chưa quét được mã độc cho tệp nên tệp CHƯA được lưu. Vui lòng thử lại sau ít phút."
		}
		httpx.WriteError(w, http.StatusServiceUnavailable, code, sentence, "")
	default:
		h.traLoiLoiXuLy(w, r, what, err)
	}
}
