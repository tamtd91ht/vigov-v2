package http

// THE BROADCAST AUDIO OF A `truyen-thanh` ITEM (ADR 0067 §4) — the HTTP half of
// internal/app/content_audio.go:
//
//	POST /api/v1/content-items/audio-files                   content.update — pending row + presigned POST
//	POST /api/v1/content-items/audio-files/{id}/completion   content.update — the uploader only; attaches
//
// `audio-files` UNDER `content-items`, beside `cover-images`, for the same reasons (the noun is
// migration 0012's `audio_file_id`; tools/ingress groups by the first segment). The item is named in the
// body (`content_item_id`, REQUIRED here, unlike the cover): the audio is uploaded for a saved broadcast.
//
// THESE HANDLERS DECIDE NOTHING. ⚠ The first reply carries a bearer credential (the presigned POST form)
// and the staff detail a presigned GET: no handler here logs a reply, a URL, a form field or a file name.

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// ContentAudioActs is the audio upload, completion and staff preview. *app.ContentAudio satisfies it.
type ContentAudioActs interface {
	RequestUpload(ctx context.Context, req app.AudioUploadRequest, actor audit.Actor) (app.AudioUpload, error)
	Complete(ctx context.Context, id string, durationSeconds int, actor audit.Actor) (app.AudioCompletion, error)
	View(ctx context.Context, fileID string) (app.AudioView, error)
}

// audioUploadIn is what the browser declares before it uploads; CHECKED against platform's
// `content-audio` policy here, and the bytes again at completion (ADR 0052 §1c).
//
// ⚠ `file_name` CAN NAME A PERSON (rule 3): stored, never logged, never in an object key, never in a
// resident's link.
type audioUploadIn struct {
	// ContentItemID — REQUIRED: a saved, live `truyen-thanh` item of this commune.
	ContentItemID string `json:"content_item_id"`
	FileName      string `json:"file_name"`
	// ContentType is `audio/mpeg` (MP3) or `audio/mp4` (M4A) — platform's list.
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

// audioFileOut is one audio file as staff see it. No object key, no uploader, no file name.
type audioFileOut struct {
	ID string `json:"id"`
	// ContentItemID is the broadcast this file was uploaded for.
	ContentItemID string `json:"content_item_id"`
	// MIMEType is the SNIFFED type (`audio/mpeg` · `audio/mp4`); "" while `pending`.
	MIMEType string `json:"mime_type"`
	// SizeBytes is the measured size; 0 while `pending`.
	SizeBytes int64 `json:"size_bytes"`
	// Status is ADR 0052 §5's: pending · ready · rejected · failed. `ready` = attached to the item.
	Status string `json:"status"`
	// DurationSeconds is the item's typed duration once attached; absent before.
	DurationSeconds int `json:"duration_seconds,omitempty"`
}

func audioFileFrom(f domain.StoredFile, item domain.NoiDungMiniApp) audioFileOut {
	out := audioFileOut{ID: f.ID, ContentItemID: f.SubjectID, MIMEType: f.MIMEType, SizeBytes: f.SizeBytes,
		Status: string(f.Status)}
	if item.AudioFileID == f.ID {
		out.DurationSeconds = item.AudioDurationSeconds
	}
	return out
}

// audioUploadOut is the reply of POST …/audio-files: the form (every `fields` entry, then the file as
// the LAST field named `file`, POSTed to `url`, valid until `expires_at`).
type audioUploadOut struct {
	AudioFile audioFileOut       `json:"audio_file"`
	Upload    presignedUploadOut `json:"upload"`
}

// audioCompletionIn is the body of the completion: the duration the officer typed (ADR 0067 §4.1,
// ADR 0047 G7 — never measured by the server). 1 .. 21600 seconds.
type audioCompletionIn struct {
	DurationSeconds int `json:"audio_duration_seconds"`
}

// audioOut is the audio block of the staff DETAIL of an item.
type audioOut struct {
	FileID string `json:"file_id"`
	// Status of the file; "" when its row can no longer be read.
	Status string `json:"status"`
	// MIMEType and SizeBytes are the measured facts of the original.
	MIMEType  string `json:"mime_type,omitempty"`
	SizeBytes int64  `json:"size_bytes,omitempty"`
	// DurationSeconds is the typed duration (the item's `audio_duration_seconds`).
	DurationSeconds int `json:"duration_seconds"`
	// PreviewURL is a presigned GET of the private original (≤ 15 minutes), to listen before
	// publishing. Absent when the file is not ready or storage is not configured.
	PreviewURL       string     `json:"preview_url,omitempty"`
	PreviewExpiresAt *time.Time `json:"preview_expires_at,omitempty"`
}

// RequestAudioUpload issues one upload slot. POST /api/v1/content-items/audio-files
func (h *Handler) RequestAudioUpload(w http.ResponseWriter, r *http.Request) {
	var in audioUploadIn
	if !docThan(w, r, &in) {
		return
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.missingCoverPrincipal(w, r)
		return
	}
	up, err := h.d.ContentAudio.RequestUpload(r.Context(), app.AudioUploadRequest{
		ContentItemID: in.ContentItemID, FileName: in.FileName, ContentType: in.ContentType, Size: in.Size,
	}, actor)
	if err != nil {
		h.answerAudioError(w, r, "xin tải âm thanh truyền thanh", err)
		return
	}
	// A form is a bearer credential: no cache between here and the officer's browser keeps it.
	w.Header().Set("Cache-Control", "no-store")
	vietJSON(w, http.StatusCreated, audioUploadOut{
		AudioFile: audioFileFrom(up.File, domain.NoiDungMiniApp{}),
		Upload:    presignedUploadOut{URL: up.Post.URL, Fields: up.Post.Fields, ExpiresAt: up.Post.ExpiresAt},
	})
}

// CompleteAudioUpload checks one upload and attaches it to its item.
// POST /api/v1/content-items/audio-files/{id}/completion
func (h *Handler) CompleteAudioUpload(w http.ResponseWriter, r *http.Request) {
	var in audioCompletionIn
	if !docThan(w, r, &in) {
		return
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.missingCoverPrincipal(w, r)
		return
	}
	done, err := h.d.ContentAudio.Complete(r.Context(), r.PathValue("id"), in.DurationSeconds, actor)
	if err != nil {
		h.answerAudioError(w, r, "hoàn tất âm thanh truyền thanh", err)
		return
	}
	vietJSON(w, http.StatusOK, audioFileFrom(done.File, done.Item))
}

// audioView builds the detail's audio block. A failure to sign the preview is logged and the block
// still goes out without the URL.
func (h *Handler) audioView(r *http.Request, n domain.NoiDungMiniApp) *audioOut {
	v, err := h.d.ContentAudio.View(r.Context(), n.AudioFileID)
	if err != nil {
		h.d.Log.Warn("truyền thanh: không dựng được thông tin nghe thử",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
	}
	out := &audioOut{FileID: n.AudioFileID, Status: string(v.Status), MIMEType: v.MIMEType,
		SizeBytes: v.SizeBytes, DurationSeconds: n.AudioDurationSeconds}
	if v.PreviewURL != "" {
		exp := v.PreviewExpiresAt
		out.PreviewURL, out.PreviewExpiresAt = v.PreviewURL.URL(), &exp
	}
	return out
}

// audioRejectionSentences is the officer-facing sentence per app.AudioReject* reason.
var audioRejectionSentences = map[string]string{
	app.AudioRejectMalware:        "Tệp âm thanh bị từ chối vì phát hiện mã độc và không được lưu.",
	app.AudioRejectTypeNotAllowed: "Tệp bị từ chối: nội dung tệp không phải MP3 hoặc M4A được phép.",
	app.AudioRejectTypeMismatch:   "Tệp bị từ chối: nội dung tệp không đúng loại đã khai báo khi tải lên.",
	app.AudioRejectTooLarge:       "Tệp bị từ chối: tệp lớn hơn dung lượng tối đa cho phép.",
	app.AudioRejectCountReached:   "Tệp bị từ chối: mục truyền thanh đã có tệp âm thanh.",
	app.AudioRejectNotAudio:       "Tệp bị từ chối: tệp không phải âm thanh MP3/M4A hợp lệ (ví dụ tệp video đổi đuôi). Hãy xuất lại tệp âm thanh rồi tải lên.",
}

// answerAudioError maps the audio refusals, then hands the rest to traLoiLoiNoiDung.
func (h *Handler) answerAudioError(w http.ResponseWriter, r *http.Request, what string, err error) {
	var rej *app.AudioRejection
	switch {
	case errors.Is(err, app.ErrAudioFileNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy tệp âm thanh đã tải lên này.", "")
	case errors.Is(err, commsstore.ErrNoiDungKhongTonTai):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy nội dung này.", "")
	case errors.Is(err, app.ErrAudioItemRequired):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Cần `content_item_id` của mục truyền thanh đã lưu. Hãy lưu mục trước rồi tải tệp âm thanh.", "")
	case errors.Is(err, domain.ErrCoverFileNameInvalid):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Tên tệp không hợp lệ — cần có tên, không quá 255 ký tự và không chứa ký tự điều khiển.", "")
	case errors.Is(err, domain.ErrCoverSizeInvalid):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Kích thước tệp khai báo không hợp lệ.", "")
	case errors.Is(err, app.ErrAudioTypeNotAllowed):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Loại tệp này không được phép cho truyền thanh — chỉ MP3 hoặc M4A.", "")
	case errors.Is(err, app.ErrAudioTooLarge):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Tệp âm thanh lớn hơn dung lượng tối đa được phép.", "")
	case errors.As(err, &rej):
		sentence, ok := audioRejectionSentences[rej.Reason]
		if !ok {
			sentence = "Tệp âm thanh bị từ chối và không được lưu."
		}
		httpx.WriteError(w, http.StatusUnprocessableEntity, "audio_rejected", sentence, "")
	case errors.Is(err, app.ErrAudioCountReached):
		httpx.WriteError(w, http.StatusConflict, "audio_limit",
			"Mục truyền thanh này đã có tệp âm thanh (hoặc đang có một lượt tải lên). Hãy gỡ tệp cũ trước khi tải tệp mới.", "")
	case errors.Is(err, app.ErrAudioNotPending):
		httpx.WriteError(w, http.StatusConflict, "audio_state",
			"Tệp này đã bị từ chối hoặc lượt tải đã hết hạn. Hãy chọn tệp và tải lên lại.", "")
	case errors.Is(err, app.ErrAudioUploadNotReceived):
		httpx.WriteError(w, http.StatusConflict, "upload_not_received",
			"Chưa nhận được tệp. Hãy chờ tải lên xong rồi bấm hoàn tất lại.", "")
	case errors.Is(err, app.ErrAudioUploadExpired):
		httpx.WriteError(w, http.StatusConflict, "upload_expired",
			"Lượt tải lên đã hết hạn mà chưa nhận được tệp. Hãy chọn tệp và tải lên lại.", "")
	case errors.Is(err, app.ErrAudioUploadChanged):
		httpx.WriteError(w, http.StatusConflict, "upload_changed",
			"Tệp vừa bị thay đổi trong lúc kiểm tra. Hãy bấm hoàn tất lại.", "")
	case errors.Is(err, app.ErrAudioLimitsUnavailable):
		h.d.Log.Warn("CẢNH BÁO: từ chối âm thanh truyền thanh vì chưa đọc được giới hạn tải tệp từ platform",
			"xa", string(tenant.MustFrom(r.Context())), "viec", what, "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "upload_limits_unavailable",
			"Chưa đọc được giới hạn tải tệp nên tệp CHƯA được nhận. Vui lòng thử lại sau ít phút.", "")
	case errors.Is(err, app.ErrAudioScanUnavailable):
		h.d.Log.Warn("CẢNH BÁO: chưa quét được mã độc, tệp âm thanh giữ ở trạng thái chờ",
			"xa", string(tenant.MustFrom(r.Context())), "viec", what, "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "malware_scan_unavailable",
			"Chưa quét được mã độc cho tệp nên tệp CHƯA được lưu. Vui lòng thử lại sau ít phút.", "")
	default:
		h.traLoiLoiNoiDung(w, r, what, err)
	}
}
