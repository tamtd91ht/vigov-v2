package http

// THE BROADCAST AUDIO OF A `truyen-thanh` ITEM (ADR 0067 §4) — the HTTP half of
// internal/app/content_audio.go:
//
//	POST /api/v1/content-items/audio-files   content.update — ONE multipart request → 201 the stored file,
//	                                         ATTACHED to its item (ADR 0052 §Sửa đổi 09/10/2026)
//
// `audio-files` UNDER `content-items`, beside `cover-images`, for the same reasons (the noun is
// migration 0012's `audio_file_id`; tools/ingress groups by the first segment). The item is named in a
// field (`content_item_id`, REQUIRED here, unlike the cover): the audio is uploaded for a saved broadcast.
//
// THESE HANDLERS DECIDE NOTHING. ⚠ The staff detail carries a presigned GET: no handler here logs a
// reply, a URL, a field value or a file name.

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// ContentAudioActs is the audio upload and staff preview. *app.ContentAudio satisfies it.
type ContentAudioActs interface {
	UploadLimit(ctx context.Context) (int64, error)
	Upload(ctx context.Context, req app.AudioUploadRequest, actor audit.Actor) (app.AudioCompletion, error)
	View(ctx context.Context, fileID string) (app.AudioView, error)
}

// audioUploadFields are the text fields of the audio upload besides `size`, all BEFORE the `file` part:
//
//	content_item_id         REQUIRED: a saved, live `truyen-thanh` item of this commune
//	audio_duration_seconds  REQUIRED: the duration the officer typed, 1 .. 21600 (ADR 0067 §4.1) — checked
//	                        before a row is written or a byte of the file is read
//	file_name               the officer's file name (else the part's filename) — ⚠ CAN NAME A PERSON
//	                        (rule 3): stored, never logged, never in an object key or a resident's link
//	content_type            `audio/mpeg` (MP3) or `audio/mp4` (M4A) — platform's list (else the part's)
var audioUploadFields = []string{uploadFieldContentItemID, uploadFieldAudioDuration, uploadFieldFileName,
	uploadFieldContentType}

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

// UploadAudio receives one audio file, checks it and attaches it to its item with the typed duration.
// POST /api/v1/content-items/audio-files
func (h *Handler) UploadAudio(w http.ResponseWriter, r *http.Request) {
	const what = "tải âm thanh truyền thanh"
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.missingCoverPrincipal(w, r)
		return
	}
	answer := func(err error) { h.answerAudioError(w, r, what, err) }
	up, ok := h.receiveUpload(w, r, what, h.d.ContentAudio.UploadLimit, audioUploadFields, answer)
	if !ok {
		return
	}
	defer up.release()
	// THE DURATION FIRST — a field before the file, so a wrong figure is refused before the use case
	// writes a row or reads a byte. Not a whole number = the same refusal as out of range: one sentence.
	seconds, err := strconv.Atoi(up.Fields[uploadFieldAudioDuration])
	if err == nil {
		err = domain.CheckAudioDuration(seconds)
	}
	if err != nil {
		answer(domain.ErrAudioDurationInvalid)
		return
	}
	done, err := h.d.ContentAudio.Upload(r.Context(), app.AudioUploadRequest{
		ContentItemID: up.Fields[uploadFieldContentItemID], FileName: up.fileName, ContentType: up.contentType,
		Size: up.Size, DurationSeconds: seconds, File: up.File, Finish: up.Finish, Deadline: up.Deadline,
	}, actor)
	if err != nil {
		answer(err)
		return
	}
	idem.RecordCode(r.Context(), done.File.ID)
	vietJSON(w, http.StatusCreated, audioFileFrom(done.File, done.Item))
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
	if h.writeUploadError(w, r, what, err) {
		return
	}
	refused := fileRefusalLog(r, h.d.Log, "âm thanh truyền thanh: từ chối", what, err)
	var rej *app.AudioRejection
	switch {
	case errors.Is(err, app.ErrAudioUploadIncomplete):
		h.writeUploadStoreError(w, r, what, err)
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
		refused("file_too_large")
		httpx.WriteError(w, http.StatusRequestEntityTooLarge, "file_too_large",
			"Tệp âm thanh lớn hơn dung lượng tối đa được phép.", "")
	case errors.As(err, &rej):
		sentence, ok := audioRejectionSentences[rej.Reason]
		if !ok {
			sentence = "Tệp âm thanh bị từ chối và không được lưu."
		}
		refused("audio_rejected")
		httpx.WriteError(w, http.StatusUnprocessableEntity, "audio_rejected", sentence, "")
	case errors.Is(err, app.ErrAudioCountReached):
		refused("audio_limit")
		httpx.WriteError(w, http.StatusConflict, "audio_limit",
			"Mục truyền thanh này đã có tệp âm thanh (hoặc đang có một lượt tải lên). Hãy gỡ tệp cũ trước khi tải tệp mới.", "")
	case errors.Is(err, app.ErrAudioNotPending):
		refused("audio_state")
		httpx.WriteError(w, http.StatusConflict, "audio_state",
			"Tệp này không còn ở trạng thái chờ kiểm tra. Hãy chọn tệp và tải lên lại.", "")
	case errors.Is(err, app.ErrAudioUploadChanged):
		refused("upload_changed")
		httpx.WriteError(w, http.StatusConflict, "upload_changed",
			"Tệp vừa bị thay đổi trong lúc kiểm tra nên CHƯA được lưu. Hãy chọn tệp và tải lên lại.", "")
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
