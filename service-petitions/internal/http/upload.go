package http

// The envelope half every upload route of this service shares — the citizen's scene photo, staff's
// verification photo, the petition log attachment and the task attachment (ADR 0052 §Sửa đổi
// 09/10/2026: ONE multipart request through the owning service). core/httpx/upload.go fixes the order:
//
//	1. a slot of the process-wide UploadSlots, BEFORE a byte of the body is read    (busy → 503)
//	2. the purpose's cap from platform's policy, so the body is bounded by it       (no default)
//	3. httpx.ReadUpload: the text fields, then the file part as a stream            (415 · 413 · 400 · 408)
//	4. the use case, under a context that ends at the upload's deadline
//
// The file is never buffered here: it is handed to the use case as a reader (uploadBody), which pipes
// it into core/storage PutUpload. Nothing a client sent — a field value, the file name — is logged.

import (
	"context"
	"errors"
	"log/slog"
	"mime"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/storage"
	"github.com/vihat/vigov/core/tenant"
)

// The text fields an upload may carry beside `size` and the `file` part.
const (
	// uploadFieldContentType is the DECLARED type. Optional: without it the file part's own
	// Content-Type header is read. Either way a claim — the completion sniffs the bytes.
	uploadFieldContentType = "content_type"
	// uploadFieldFileName is the name an attachment is shown under. Optional: without it the file part's
	// own file name is read. Personal data when it describes a case (rule 3): stored, never logged.
	uploadFieldFileName = "file_name"
)

// uploadLimiter is the one question asked before the body is read: the purpose's cap.
type uploadLimiter interface {
	MaxUploadBytes(ctx context.Context) (int64, error)
}

// uploadBody hands the received file to the use case as app.UploadBody.
type uploadBody struct{ up *httpx.UploadRequest }

func (b uploadBody) Read(p []byte) (int, error) { return b.up.File.Read(p) }
func (b uploadBody) Finish() error              { return b.up.Finish() }

// receiveUpload runs steps 1–3. ok=false means the response was written — a use-case refusal of step
// 2 through `answer`, so a route answers one way. release is never nil; defer it.
func receiveUpload(w http.ResponseWriter, r *http.Request, slots *httpx.UploadSlots, log *slog.Logger,
	limit uploadLimiter, fields []string, answer func(error)) (httpx.UploadRequest, func(), bool) {

	ctx := r.Context()
	release, ok := slots.Acquire(ctx)
	if !ok {
		// LOGGED: a pod refusing uploads is an operator's signal (scale, or a client hammering). The
		// commune only — never who.
		log.WarnContext(ctx, "tải tệp lên: hết chỗ tải cùng lúc trên pod, trả 503",
			"xa", string(tenant.MustFrom(ctx)), "so_cho", httpx.UploadSlotsPerPod)
		httpx.WriteUploadBusy(w, "")
		return httpx.UploadRequest{}, release, false
	}
	maxBytes, err := limit.MaxUploadBytes(ctx)
	if err != nil {
		answer(err)
		return httpx.UploadRequest{}, release, false
	}
	up, err := httpx.ReadUpload(w, r, httpx.UploadOptions{MaxFileBytes: maxBytes, Fields: fields})
	if err != nil {
		answer(err)
		return httpx.UploadRequest{}, release, false
	}
	return up, release, true
}

// declaredType is the `content_type` field, else the file part's header — its media type only.
func declaredType(up httpx.UploadRequest) string {
	if v := up.Fields[uploadFieldContentType]; v != "" {
		return v
	}
	mt, _, err := mime.ParseMediaType(up.ContentType)
	if err != nil {
		return "" // refused by the use case as a type it does not accept
	}
	return mt
}

// declaredFileName is the `file_name` field, else the file part's own name.
func declaredFileName(up httpx.UploadRequest) string {
	if v, ok := up.Fields[uploadFieldFileName]; ok {
		return v
	}
	return up.Filename
}

// writeUploadEnvelopeError answers a failure of the upload itself — the envelope read by
// httpx.ReadUpload, or the stream cut while core/storage was writing it — and reports whether it did.
// The client's fault, never a 5xx; every one is LOGGED through `refused` (INFO), since an upload that
// failed on the wire is exactly what was invisible before the bytes went through this service.
func writeUploadEnvelopeError(w http.ResponseWriter, err error, refused func(code string)) bool {
	status := httpx.UploadErrorStatus(err)
	switch {
	case status == http.StatusRequestEntityTooLarge, errors.Is(err, storage.ErrTooLarge):
		refused("file_too_large")
		httpx.WriteError(w, http.StatusRequestEntityTooLarge, "file_too_large",
			"Tệp lớn hơn dung lượng tối đa cho phép.", "")
	case status == http.StatusUnsupportedMediaType:
		refused("unsupported_media_type")
		httpx.WriteError(w, http.StatusUnsupportedMediaType, "unsupported_media_type",
			"Tệp phải được gửi bằng biểu mẫu multipart/form-data.", "")
	case status == http.StatusRequestTimeout:
		refused("upload_timeout")
		httpx.WriteError(w, http.StatusRequestTimeout, "upload_timeout",
			"Tải tệp lên quá thời gian cho phép nên tệp CHƯA được nhận. Vui lòng thử lại.", "")
	case status == http.StatusBadRequest, errors.Is(err, storage.ErrSizeMismatch):
		refused("invalid_upload")
		httpx.WriteError(w, http.StatusBadRequest, "invalid_upload",
			"Dữ liệu tải lên không hợp lệ hoặc không khớp kích thước đã khai báo. Vui lòng chọn tệp và thử lại.", "")
	default:
		return false
	}
	return true
}
