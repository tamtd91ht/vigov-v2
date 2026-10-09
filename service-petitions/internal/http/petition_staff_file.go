package http

// The STAFF files on a petition — the HTTP half of internal/app/petition_verification_photo.go and
// internal/app/petition_log_attachment.go (migration 0027, owner decisions of 02/10/2026):
//
//	POST /api/v1/citizen-reports/{maTraCuu}/verification-photos             feedback.resolve (multipart upload)
//	GET  /api/v1/citizen-reports/{maTraCuu}/verification-photos             feedback.read (audited)
//	POST /api/v1/citizen-reports/{maTraCuu}/log-attachments                 feedback.read + app.duocGhiChu (multipart upload)
//	GET  /api/v1/citizen-reports/{maTraCuu}/log-attachments/{id}/download   feedback.read (audited)
//
// THESE HANDLERS DECIDE NOTHING: they read the permission FACTS the use cases take (feedback.restricted;
// the three commune-wide petition keys for the note rule) and translate the answer. The citizen's read
// of verification photos is on HandlerCongDan (ListMyVerificationPhotos), never here.
//
// ⚠ THE READ REPLIES CARRY BEARER CREDENTIALS (presigned GET URLs): `Cache-Control: no-store`, never
// logged — nor the lookup code, nor a file name. The uploads go THROUGH this service (ADR 0052 §Sửa đổi
// 09/10/2026, upload.go) and answer the stored file.

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
	uploadLimiter
	Upload(ctx context.Context, ma string, req app.PhotoUploadRequest, body app.UploadBody, actor audit.Actor,
		restricted app.QuyenXemHanChe) (domain.StoredFile, error)
	// reader is the staff member, ID = their business code (principal.Ma): the use case audits the read.
	ListPhotos(ctx context.Context, ma string, mayReadRestricted bool, reader audit.Actor) ([]app.PhotoLink, error)
}

// PetitionLogAttachmentActs is app.PetitionLogAttachments.
type PetitionLogAttachmentActs interface {
	uploadLimiter
	Upload(ctx context.Context, ma string, req app.AttachmentUploadRequest, body app.UploadBody,
		actor audit.Actor, noteRight app.QuyenGhiChuCaXa, restricted app.QuyenXemHanChe) (domain.StoredFile, error)
	DownloadLink(ctx context.Context, ma, id string, reader audit.Actor, restricted app.QuyenXemHanChe) (
		app.AttachmentDownload, error)
	// Remove is the soft delete (DELETE …/log-attachments/{id}, 09/10/2026) — citizen_report_figures.go.
	// `resolve` is the caller's `feedback.resolve` fact — the second door beside being the uploader.
	Remove(ctx context.Context, ma, id, reason string, actor audit.Actor, resolve app.QuyenXuLyCaXa,
		restricted app.QuyenXemHanChe) error
}

// PetitionLogAttachmentReader reads the attachments of ONE PAGE of petition timeline entries in one
// statement. *petstore.StoredFileStore satisfies it. STAFF-ONLY.
type PetitionLogAttachmentReader interface {
	PetitionAttachmentsByLogEntries(ctx context.Context, logEntryIDs []string) (
		map[string][]domain.PetitionLogAttachment, error)
}

// --- verification photos ------------------------------------------------------------------------------

// UploadVerificationPhoto serves POST /api/v1/citizen-reports/{maTraCuu}/verification-photos — ONE
// multipart upload of ONE photo, answered with the STORED photo. The part's file name is ignored (the
// photo is stored under domain.VerificationPhotoName).
func (h *Handler) UploadVerificationPhoto(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheXuLy(w, r)
		return
	}
	answer := func(err error) { h.answerVerificationPhotoError(w, r, "tải ảnh sau xử lý", err) }
	up, release, ok := receiveUpload(w, r, h.d.UploadSlots, h.d.Log, h.d.VerificationPhotos,
		[]string{uploadFieldContentType}, answer)
	defer release()
	if !ok {
		return
	}
	ctx, cancel := context.WithDeadline(r.Context(), up.Deadline)
	defer cancel()
	f, err := h.d.VerificationPhotos.Upload(ctx, r.PathValue("maTraCuu"),
		app.PhotoUploadRequest{ContentType: declaredType(up), Size: up.Size}, uploadBody{&up}, actor,
		h.coQuyenHanChe(ctx))
	if err != nil {
		answer(err)
		return
	}
	// A replay with the same Idempotency-Key is told the FILE ID — never a second file.
	idem.RecordCode(ctx, f.ID)
	vietJSON(w, http.StatusCreated, photoFromFile(f))
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
	refused := fileRefusalLog(r, h.d.Log, "ảnh sau xử lý: từ chối", what, err)
	ctx := r.Context()
	var rej *app.AttachmentRejection
	switch {
	case writeUploadEnvelopeError(w, err, refused):
	case errors.Is(err, petstore.ErrPhieuKhongTonTai), errors.Is(err, app.ErrPhieuHanChe):
		// One answer for no such code, another commune's, soft-deleted, and `can-bo` without the key.
		h.khongTimThay(w)
	case errors.Is(err, app.ErrVerificationPhotoNotFound):
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
		sentence, ok := verificationPhotoRejections[rej.Reason]
		if !ok {
			sentence = "Ảnh bị từ chối và không được lưu."
		}
		refused("photo_rejected")
		httpx.WriteError(w, http.StatusUnprocessableEntity, "photo_rejected", sentence, "")
	case errors.Is(err, app.ErrVerificationPhotoWindowClosed):
		refused("petition_state")
		httpx.WriteError(w, http.StatusConflict, "petition_state",
			"Phiếu đã kết thúc nên không thêm ảnh sau xử lý được nữa.", "")
	case errors.Is(err, app.ErrVerificationPhotoCountReached):
		refused("photo_limit")
		httpx.WriteError(w, http.StatusConflict, "photo_limit", "Phiếu đã có đủ số ảnh sau xử lý tối đa.", "")
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
		h.d.Log.Error("ảnh sau xử lý: lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "viec", what, "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}

// --- log attachments ----------------------------------------------------------------------------------

// UploadPetitionLogAttachment serves POST /api/v1/citizen-reports/{maTraCuu}/log-attachments — ONE
// multipart upload of ONE file, answered with the STORED file. `file_name` (or the part's own name) is
// the name it is shown under: personal data when it describes a case — stored, never logged.
func (h *Handler) UploadPetitionLogAttachment(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheXuLy(w, r)
		return
	}
	answer := func(err error) { h.answerPetitionLogAttachmentError(w, r, "tải tệp đính kèm nhật ký", err) }
	up, release, ok := receiveUpload(w, r, h.d.UploadSlots, h.d.Log, h.d.PetitionLogAttachments,
		[]string{uploadFieldFileName, uploadFieldContentType}, answer)
	defer release()
	if !ok {
		return
	}
	ctx, cancel := context.WithDeadline(r.Context(), up.Deadline)
	defer cancel()
	f, err := h.d.PetitionLogAttachments.Upload(ctx, r.PathValue("maTraCuu"), app.AttachmentUploadRequest{
		FileName: declaredFileName(up), ContentType: declaredType(up), Size: up.Size,
	}, uploadBody{&up}, actor, h.coQuyenGhiChuCaXa(ctx), h.coQuyenHanChe(ctx))
	if err != nil {
		answer(err)
		return
	}
	idem.RecordCode(ctx, f.ID)
	vietJSON(w, http.StatusCreated, taskAttachmentFromFile(f))
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
	refused := fileRefusalLog(r, h.d.Log, "tệp đính kèm nhật ký phiếu: từ chối", what, err)
	ctx := r.Context()
	var rej *app.AttachmentRejection
	switch {
	case writeUploadEnvelopeError(w, err, refused):
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
		refused("file_too_large")
		httpx.WriteError(w, http.StatusRequestEntityTooLarge, "file_too_large",
			"Tệp lớn hơn dung lượng tối đa được phép đính kèm.", "")
	case errors.As(err, &rej):
		sentence, ok := petitionLogRejectionSentences[rej.Reason]
		if !ok {
			sentence = "Tệp bị từ chối và không được lưu."
		}
		refused("attachment_rejected")
		httpx.WriteError(w, http.StatusUnprocessableEntity, "attachment_rejected", sentence, "")
	case errors.Is(err, app.ErrPetitionLogAttachmentCountReached):
		refused("attachment_limit")
		httpx.WriteError(w, http.StatusConflict, "attachment_limit",
			"Phiếu đã có đủ số tệp đính kèm tối đa được phép.", "")
	case errors.Is(err, app.ErrAttachmentNotPending):
		refused("attachment_state")
		httpx.WriteError(w, http.StatusConflict, "attachment_state",
			"Tệp này đã bị từ chối. Hãy chọn tệp và tải lên lại.", "")
	case errors.Is(err, app.ErrUploadChanged):
		refused("upload_changed")
		httpx.WriteError(w, http.StatusConflict, "upload_changed",
			"Tệp vừa bị thay đổi trong lúc kiểm tra nên CHƯA được lưu. Hãy tải lên lại.", "")
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
