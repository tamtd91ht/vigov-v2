package http

// THE ENVELOPE HALF OF EVERY UPLOAD ROUTE OF THIS SERVICE — the cover, the body image and the broadcast
// audio (ADR 0052 §Sửa đổi 09/10/2026: ONE multipart request through the owning service; devices no
// longer POST straight to MinIO). In core/httpx's order (core/httpx/upload.go):
//
//	1. the purpose's byte cap from platform's policy   — before a byte of the body (never a constant here)
//	2. one of the process's UploadSlots                 — taken without waiting; none free → 503 upload_busy
//	3. httpx.ReadUpload                                 — text fields, then the `file` part as a stream
//
// then the route's use case streams the part into the temp bucket and runs the completion. The slot is
// held until the handler returns: the inspection (scan, decode) is the memory the cap exists to bound.
//
// NOTHING A CLIENT SENT IS LOGGED: not the file name, not a field value. A refusal's log line carries
// the commune, the route's act, a fixed code and the wrapped error (whose text names object keys and
// commune ids, never a client value — core/httpx and core/storage hold to that).

import (
	"context"
	"errors"
	"mime"
	"net/http"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/storage"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
)

// The text fields of an upload, besides httpx's `size`. All are sent BEFORE the `file` part.
const (
	// uploadFieldFileName is the officer's file name; absent → the part's own filename. Personal data
	// (rule 3): stored on the row, never logged, never in a key.
	uploadFieldFileName = "file_name"
	// uploadFieldContentType is the DECLARED type; absent → the part's Content-Type. The key's extension
	// follows it; the completion sniffs the bytes and refuses a mismatch.
	uploadFieldContentType = "content_type"
	// uploadFieldContentItemID names the article / broadcast, as the JSON field of the same name did.
	uploadFieldContentItemID = "content_item_id"
	// uploadFieldAudioDuration is the broadcast's typed duration in seconds (audio route only).
	uploadFieldAudioDuration = "audio_duration_seconds"
)

// receivedUpload is what receiveUpload hands the route: the validated envelope and the slot's release.
type receivedUpload struct {
	httpx.UploadRequest
	// fileName / contentType are the declared values, field first, then the part's own.
	fileName, contentType string
	release               func()
}

// receiveUpload runs steps 1–3. ok=false means it has answered (answerLimit for a policy refusal, the
// envelope's own status otherwise); ok=true means the caller MUST call release (defer it).
func (h *Handler) receiveUpload(w http.ResponseWriter, r *http.Request, what string,
	limit func(context.Context) (int64, error), fields []string, answerLimit func(error)) (receivedUpload, bool) {

	if h.d.UploadSlots == nil {
		// Wiring fault, answered closed: an upload route with no per-pod bound is how a burst of large
		// files takes the pod — and every other route of it — down.
		h.d.Log.Error("tuyến tải tệp chạy mà không có giới hạn lượt tải đồng thời — SAI CẤU HÌNH, từ chối",
			"xa", string(tenant.MustFrom(r.Context())), "viec", what)
		httpx.WriteError(w, http.StatusServiceUnavailable, "upload_unavailable",
			"Hệ thống tạm thời chưa nhận được tệp. Vui lòng thử lại sau.", "")
		return receivedUpload{}, false
	}
	maxBytes, err := limit(r.Context())
	if err != nil {
		answerLimit(err)
		return receivedUpload{}, false
	}
	release, ok := h.d.UploadSlots.Acquire(r.Context())
	if !ok {
		release()
		uploadRefusalLog(r, h, what, errBusy)(httpx.UploadBusyCode)
		httpx.WriteUploadBusy(w, "")
		return receivedUpload{}, false
	}
	up, err := httpx.ReadUpload(w, r, httpx.UploadOptions{MaxFileBytes: maxBytes, Fields: fields})
	if err != nil {
		release()
		if !h.writeUploadError(w, r, what, err) {
			h.d.Log.Error("tải tệp: lỗi đọc lượt tải không thuộc nhóm đã biết", "xa",
				string(tenant.MustFrom(r.Context())), "viec", what, "err", err)
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		}
		return receivedUpload{}, false
	}
	out := receivedUpload{UploadRequest: up, release: release,
		fileName: up.Fields[uploadFieldFileName], contentType: up.Fields[uploadFieldContentType]}
	if out.fileName == "" {
		out.fileName = up.Filename
	}
	if out.contentType == "" {
		out.contentType = up.ContentType
	}
	// "image/jpeg; charset=binary" is still image/jpeg; anything unparseable is passed on as sent and the
	// policy refuses it as a type it does not allow.
	if mt, _, err := mime.ParseMediaType(out.contentType); err == nil {
		out.contentType = mt
	}
	return out, true
}

// errBusy is the cause logged with a 503 upload_busy — fixed, so the line says why without a client value.
var errBusy = errors.New("mọi lượt tải đồng thời của pod đang bận")

// writeUploadError answers the refusals every upload route shares — the envelope's (httpx sentinels) and
// the write's (storage) — wherever they surface: from ReadUpload, or wrapped inside the use case's
// Err*UploadIncomplete. false = none of them; the route maps the rest.
//
// A STORE THAT FAILED IS NEVER A 4xx: the client's bytes were fine, so it is 503 and "try again".
func (h *Handler) writeUploadError(w http.ResponseWriter, r *http.Request, what string, err error) bool {
	refused := uploadRefusalLog(r, h, what, err)
	switch httpx.UploadErrorStatus(err) {
	case http.StatusUnsupportedMediaType:
		refused("upload_not_multipart")
		httpx.WriteError(w, http.StatusUnsupportedMediaType, "upload_not_multipart",
			"Tệp phải được gửi dạng multipart/form-data, mỗi lượt một tệp.", "")
		return true
	case http.StatusRequestEntityTooLarge:
		refused("file_too_large")
		httpx.WriteError(w, http.StatusRequestEntityTooLarge, "file_too_large",
			"Tệp lớn hơn dung lượng tối đa được phép.", "")
		return true
	case http.StatusRequestTimeout:
		refused("upload_timeout")
		httpx.WriteError(w, http.StatusRequestTimeout, "upload_timeout",
			"Tải tệp lên quá thời gian cho phép nên tệp CHƯA được nhận. Vui lòng thử lại.", "")
		return true
	case http.StatusBadRequest:
		refused("invalid_upload")
		httpx.WriteError(w, http.StatusBadRequest, "invalid_upload",
			"Nội dung tải lên không hợp lệ nên tệp CHƯA được nhận. Hãy chọn tệp và tải lên lại.", "")
		return true
	}
	// The write's refusals, ONLY inside an incomplete upload: the same storage sentinels from another
	// path (PutServerProduced under the from-url route) are a server fault, not the client's.
	if !errors.Is(err, app.ErrCoverUploadIncomplete) && !errors.Is(err, app.ErrAudioUploadIncomplete) {
		return false
	}
	switch {
	case errors.Is(err, storage.ErrTooLarge):
		refused("file_too_large")
		httpx.WriteError(w, http.StatusRequestEntityTooLarge, "file_too_large",
			"Tệp lớn hơn dung lượng tối đa được phép.", "")
	case errors.Is(err, storage.ErrSizeMismatch):
		refused("invalid_upload")
		httpx.WriteError(w, http.StatusBadRequest, "invalid_upload",
			"Nội dung tải lên không hợp lệ nên tệp CHƯA được nhận. Hãy chọn tệp và tải lên lại.", "")
	case errors.Is(err, context.DeadlineExceeded):
		// The write outlived up.Deadline inside the store client rather than on a read of the body.
		refused("upload_timeout")
		httpx.WriteError(w, http.StatusRequestTimeout, "upload_timeout",
			"Tải tệp lên quá thời gian cho phép nên tệp CHƯA được nhận. Vui lòng thử lại.", "")
	default:
		return false
	}
	return true
}

// writeUploadStoreError answers an Err*UploadIncomplete that writeUploadError did not claim: the temp
// bucket refused or could not be reached. WARN, with the cause — this is the line an operator needs.
func (h *Handler) writeUploadStoreError(w http.ResponseWriter, r *http.Request, what string, err error) {
	h.d.Log.Warn("CẢNH BÁO: không ghi được tệp vào kho tạm — tệp CHƯA được nhận",
		"xa", string(tenant.MustFrom(r.Context())), "viec", what, "ma_loi", "upload_store_unavailable", "err", err)
	httpx.WriteError(w, http.StatusServiceUnavailable, "upload_store_unavailable",
		"Chưa ghi được tệp vào kho lưu nên tệp CHƯA được nhận. Vui lòng thử lại sau ít phút.", "")
}

// uploadRefusalLog is fileRefusalLog for the envelope and write refusals.
func uploadRefusalLog(r *http.Request, h *Handler, what string, err error) func(code string) {
	return fileRefusalLog(r, h.d.Log, "tải tệp: từ chối", what, err)
}
