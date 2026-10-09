package http

// `📎 Đính kèm` on the task timeline (docs/ui-ux/02-nhiem-vu.md §5.9) — the HTTP half of
// internal/app/task_attachment.go:
//
//	POST /api/v1/tasks/{ma}/attachments                  task.read gate; the log-entry right in the use case;
//	                                                     ONE multipart upload, answered with the stored file
//	GET  /api/v1/tasks/{ma}/attachments/{id}/download    task.read; domain.MayDownload in the use case
//	DELETE /api/v1/tasks/{ma}/attachments/{id}           task.read gate; uploader or task.update in the use case
//
// THESE HANDLERS DECIDE NOTHING, the shape of AddTaskLogEntry: they read whether the caller holds
// `task.update` (the one fact the log-entry rule takes) and translate the use case's answer.
//
// ⚠ THE DOWNLOAD REPLY CARRIES A BEARER CREDENTIAL (a presigned GET URL). It goes to the client and
// nowhere else: no handler here logs a reply, a URL, a field of the upload or a file name.

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// taskAttachmentOut is one file as a member of staff sees it — on the timeline, and in the reply of the
// upload route. No object key and no uploader: the download is its own signed
// request, and who wrote the entry is the entry's `actor_code`.
type taskAttachmentOut struct {
	ID       string `json:"id"`
	FileName string `json:"file_name"`
	// MIMEType is the SNIFFED type.
	MIMEType string `json:"mime_type"`
	// SizeBytes is the measured size.
	SizeBytes int64 `json:"size_bytes"`
	// Status is ADR 0052 §5's: pending · stored · ready · rejected · failed · purged (a timeline shows
	// only files that were attached, so stored / ready / purged in practice).
	Status string `json:"status"`
}

func taskAttachmentFromFile(f domain.StoredFile) taskAttachmentOut {
	return taskAttachmentOut{ID: f.ID, FileName: f.OriginalName, MIMEType: f.MIMEType, SizeBytes: f.SizeBytes,
		Status: string(f.Status)}
}

func taskAttachmentFromLog(a domain.TaskLogAttachment) taskAttachmentOut {
	return taskAttachmentOut{ID: a.FileID, FileName: a.OriginalName, MIMEType: a.MIMEType, SizeBytes: a.SizeBytes,
		Status: string(a.Status)}
}

// taskAttachmentsOut renders a list, never null.
func taskAttachmentsOut(in []domain.TaskLogAttachment) []taskAttachmentOut {
	out := make([]taskAttachmentOut, 0, len(in))
	for _, a := range in {
		out = append(out, taskAttachmentFromLog(a))
	}
	return out
}

// taskAttachmentDownloadOut is a presigned GET, valid until `expires_at` (at most 15 minutes). Open it
// directly; it is served from the object store's domain, never the commune's (ADR 0052 §4).
type taskAttachmentDownloadOut struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// UploadTaskAttachment receives ONE file through this service and answers it STORED (ADR 0052 §Sửa đổi
// 09/10/2026). POST /api/v1/tasks/{ma}/attachments
//
// ⚠ `file_name` (or the part's own name) IS PERSONAL DATA WHEN IT DESCRIBES A CASE (rule 3): stored,
// returned to readers of the task, never logged, never in an object key.
func (h *Handler) UploadTaskAttachment(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheNhiemVu(w, r)
		return
	}
	answer := func(err error) { h.answerTaskAttachmentError(w, r, "tải tệp đính kèm", err) }
	up, release, ok := receiveUpload(w, r, h.d.UploadSlots, h.d.Log, h.d.TaskAttachments,
		[]string{uploadFieldFileName, uploadFieldContentType}, answer)
	defer release()
	if !ok {
		return
	}
	ctx, cancel := context.WithDeadline(r.Context(), up.Deadline)
	defer cancel()
	f, err := h.d.TaskAttachments.Upload(ctx, r.PathValue("ma"), app.AttachmentUploadRequest{
		FileName: declaredFileName(up), ContentType: declaredType(up), Size: up.Size,
	}, uploadBody{&up}, actor, h.hasTaskUpdate(r))
	if err != nil {
		answer(err)
		return
	}
	// A replay with the same Idempotency-Key is told the FILE ID (core/idem) — never a second file.
	idem.RecordCode(ctx, f.ID)
	vietJSON(w, http.StatusCreated, taskAttachmentFromFile(f))
}

// TaskAttachmentDownload hands out a short-lived link. GET /api/v1/tasks/{ma}/attachments/{id}/download
func (h *Handler) TaskAttachmentDownload(w http.ResponseWriter, r *http.Request) {
	reader, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheNhiemVu(w, r)
		return
	}
	d, err := h.d.TaskAttachments.DownloadLink(r.Context(), r.PathValue("ma"), r.PathValue("id"), reader)
	if err != nil {
		h.answerTaskAttachmentError(w, r, "tải tệp đính kèm", err)
		return
	}
	// A link is a bearer credential: no cache between here and the officer's browser keeps it.
	w.Header().Set("Cache-Control", "no-store")
	vietJSON(w, http.StatusOK, taskAttachmentDownloadOut{URL: d.URL.URL(), ExpiresAt: d.ExpiresAt})
}

// taskAttachmentRemoveIn is the body of DELETE /api/v1/tasks/{ma}/attachments/{id}.
//
// A BODY ON A DELETE, xoaNhiemVuVao's reason: the reason is mandatory (rule 7, invariant 1), and the
// query string would put free text about a government record into every access log and proxy cache.
type taskAttachmentRemoveIn struct {
	Reason string `json:"reason"`
}

// RemoveTaskAttachment soft-deletes one file of the task. DELETE /api/v1/tasks/{ma}/attachments/{id}
func (h *Handler) RemoveTaskAttachment(w http.ResponseWriter, r *http.Request) {
	var in taskAttachmentRemoveIn
	if !docThan(w, r, &in) {
		return
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.thieuChuTheNhiemVu(w, r)
		return
	}
	if err := h.d.TaskAttachments.Remove(r.Context(), r.PathValue("ma"), r.PathValue("id"), in.Reason, actor,
		h.hasTaskUpdate(r)); err != nil {
		h.answerTaskAttachmentError(w, r, "gỡ tệp đính kèm", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// rejectionSentences is the officer-facing sentence per app.Reject* reason.
var rejectionSentences = map[string]string{
	app.RejectMalware:        "Tệp bị từ chối vì phát hiện mã độc và không được lưu.",
	app.RejectTypeNotAllowed: "Tệp bị từ chối: nội dung tệp không thuộc loại được phép đính kèm.",
	app.RejectTypeMismatch:   "Tệp bị từ chối: nội dung tệp không đúng loại tệp đã khai báo khi tải lên.",
	app.RejectTooLarge:       "Tệp bị từ chối: tệp lớn hơn dung lượng tối đa cho phép.",
	app.RejectCountReached:   "Tệp bị từ chối: nhiệm vụ đã có đủ số tệp đính kèm tối đa.",
}

// answerTaskAttachmentError maps the attachment refusals, then hands everything else — the task's own
// 404, the log-entry right's 403, the 500 — to traLoiLoiNhiemVu, so a task route answers one way.
//
// 503 FOR EVERYTHING THAT IS "NOT NOW" RATHER THAN "NO": storage / scanner / limits not configured, the
// platform or the scanner unreachable. Nothing was stored in any of them (ADR 0052 §9, §10). The commune
// is logged, never the file name.
func (h *Handler) answerTaskAttachmentError(w http.ResponseWriter, r *http.Request, what string, err error) {
	refused := fileRefusalLog(r, h.d.Log, "tệp đính kèm nhiệm vụ: từ chối", what, err)
	var rej *app.AttachmentRejection
	switch {
	case writeUploadEnvelopeError(w, err, refused):
	case errors.Is(err, app.ErrAttachmentNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy tệp đính kèm này trên nhiệm vụ.", "")

	case errors.Is(err, domain.ErrAttachmentNameInvalid), errors.Is(err, domain.ErrAttachmentSizeInvalid),
		errors.Is(err, domain.ErrAttachmentListInvalid), errors.Is(err, domain.ErrAttachmentNotUsable):
		// The domain's own sentence (cauTuChoi, never err.Error(): a refusal raised inside the
		// transaction arrives wrapped by app.bocNhiemVu with the commune id).
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", cauTuChoi(err,
			domain.ErrAttachmentNameInvalid, domain.ErrAttachmentSizeInvalid,
			domain.ErrAttachmentListInvalid, domain.ErrAttachmentNotUsable), "")
	case errors.Is(err, domain.ErrAttachmentRemovalReasonMissing),
		errors.Is(err, domain.ErrAttachmentRemovalReasonTooLong):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", cauTuChoi(err,
			domain.ErrAttachmentRemovalReasonMissing, domain.ErrAttachmentRemovalReasonTooLong), "")
	case errors.Is(err, domain.ErrAttachmentRemovalNotAllowed):
		httpx.WriteError(w, http.StatusForbidden, "forbidden",
			cauTuChoi(err, domain.ErrAttachmentRemovalNotAllowed), "")
	case errors.Is(err, domain.ErrAttachmentUnderLegalHold):
		// Its own code: the client can say "held for a complaint or an inspection" rather than a generic
		// conflict, and nothing the officer reloads will change it.
		refused("legal_hold")
		httpx.WriteError(w, http.StatusConflict, "legal_hold",
			"Tệp đang được giữ để phục vụ khiếu nại hoặc thanh tra nên chưa thể gỡ. "+
				"Khi việc giữ tệp kết thúc, bạn mới gỡ được.", "")
	case errors.Is(err, app.ErrAttachmentTypeNotAllowed):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Loại tệp này không được phép đính kèm vào nhật ký nhiệm vụ.", "")
	case errors.Is(err, app.ErrAttachmentTooLarge):
		refused("file_too_large")
		httpx.WriteError(w, http.StatusRequestEntityTooLarge, "file_too_large",
			"Tệp lớn hơn dung lượng tối đa được phép đính kèm.", "")

	case errors.As(err, &rej):
		sentence, ok := rejectionSentences[rej.Reason]
		if !ok {
			sentence = "Tệp bị từ chối và không được lưu."
		}
		refused("attachment_rejected")
		httpx.WriteError(w, http.StatusUnprocessableEntity, "attachment_rejected", sentence, "")
	case errors.Is(err, app.ErrAttachmentCountReached):
		refused("attachment_limit")
		httpx.WriteError(w, http.StatusConflict, "attachment_limit",
			"Nhiệm vụ đã có đủ số tệp đính kèm tối đa được phép.", "")
	case errors.Is(err, app.ErrAttachmentNotPending):
		refused("attachment_state")
		httpx.WriteError(w, http.StatusConflict, "attachment_state",
			"Tệp này đã bị từ chối. Hãy chọn tệp và tải lên lại.", "")
	case errors.Is(err, app.ErrUploadChanged):
		refused("upload_changed")
		httpx.WriteError(w, http.StatusConflict, "upload_changed",
			"Tệp vừa bị thay đổi trong lúc kiểm tra nên CHƯA được lưu. Hãy tải lên lại.", "")

	case errors.Is(err, app.ErrUploadNotConfigured):
		h.d.Log.Warn("CẢNH BÁO: từ chối tệp đính kèm vì chưa cấu hình kho lưu tệp / máy quét / giới hạn",
			"xa", string(tenant.MustFrom(r.Context())), "viec", what, "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "storage_not_configured",
			"Chưa cấu hình kho lưu tệp nên chưa đính kèm được tệp. Hãy báo quản trị hệ thống.", "")
	case errors.Is(err, app.ErrUploadLimitsUnavailable):
		h.d.Log.Warn("CẢNH BÁO: từ chối tệp đính kèm vì chưa đọc được giới hạn tải tệp từ platform",
			"xa", string(tenant.MustFrom(r.Context())), "viec", what, "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "upload_limits_unavailable",
			"Chưa đọc được giới hạn tải tệp nên tệp CHƯA được nhận. Vui lòng thử lại sau ít phút.", "")
	case errors.Is(err, app.ErrScanUnavailable):
		h.d.Log.Warn("CẢNH BÁO: chưa quét được mã độc, tệp giữ ở trạng thái chờ",
			"xa", string(tenant.MustFrom(r.Context())), "viec", what, "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "malware_scan_unavailable",
			"Chưa quét được mã độc cho tệp nên tệp CHƯA được lưu. Vui lòng thử lại sau ít phút.", "")

	default:
		h.traLoiLoiNhiemVu(w, r, what, err)
	}
}
