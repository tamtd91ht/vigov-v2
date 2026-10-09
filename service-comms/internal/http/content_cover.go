package http

// THE COVER UPLOAD OF A MINI APP ARTICLE (docs/ui-ux/11-noi-dung-mini-app.md §7 `Ảnh đại diện`) — the
// HTTP half of internal/app/content_cover.go:
//
//	POST /api/v1/content-items/cover-images   content.update — ONE multipart request → 201 the stored file
//
// (ADR 0052 §Sửa đổi 09/10/2026: the presigned POST and the `…/{id}/completion` route are gone.) The
// file then rides on the article: `cover_image_file_id` on POST / PATCH /api/v1/content-items.
//
// `cover-images` UNDER `content-items`, NOT A NEW FIRST SEGMENT: the noun is migration 0011's own column
// (`cover_image_file_id`), and tools/ingress groups routes by the first segment after /api/v1/, which is
// already this service's. It sits at COLLECTION level, not under `{id}`, because §7's modal uploads
// before `Lưu` — the article may not exist yet (0011); an existing article is named in a field.
//
// THESE HANDLERS DECIDE NOTHING: they translate the use case's answer. The envelope (cap, slot, multipart)
// is internal/http/upload.go's.
//
// ⚠ The staff detail carries a presigned GET. It goes to the client and nowhere else: no handler here
// logs a reply, a URL, a field value or a file name.

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// ContentCoverActs is the cover upload and staff preview. *app.ContentCovers satisfies it.
// ITS OWN INTERFACE and not more methods on GhiNoiDungMiniApp: its dependencies — the object store, the
// scanner, platform's limits — may be absent in a deployment while every other content route works.
type ContentCoverActs interface {
	CoverUploadLimit(ctx context.Context) (int64, error)
	UploadCover(ctx context.Context, req app.CoverUploadRequest, actor audit.Actor) (domain.StoredFile, error)
	View(ctx context.Context, fileID string) (app.CoverView, error)

	// The body images (content_body_image.go): the same acts under `content-body-image`, and the staff
	// previews of the images one article's body references.
	BodyImageUploadLimit(ctx context.Context) (int64, error)
	UploadBodyImage(ctx context.Context, req app.CoverUploadRequest, actor audit.Actor) (domain.StoredFile, error)
	BodyImageViews(ctx context.Context, itemID string, fileIDs []string) ([]app.BodyImageView, error)
	// FetchBodyImage: the server downloads a pasted https link into a ready body image (H5, K6).
	FetchBodyImage(ctx context.Context, req app.BodyImageFromURLRequest, actor audit.Actor) (domain.StoredFile, error)
}

// coverUploadFields are the text fields of the cover (and body-image) upload besides `size`:
//
//	file_name        the officer's file name (else the part's filename) — ⚠ CAN NAME A PERSON (rule 3):
//	                 stored, never logged, never in an object key
//	content_type     the DECLARED type (else the part's Content-Type), checked against the policy
//	content_item_id  an EXISTING article, or one this officer's earlier upload reserved; ABSENT for an
//	                 article not saved yet — the server reserves an id and returns it
var coverUploadFields = []string{uploadFieldFileName, uploadFieldContentType, uploadFieldContentItemID}

// coverUploadFrom maps a received upload onto the use case's request.
func coverUploadFrom(up receivedUpload) app.CoverUploadRequest {
	return app.CoverUploadRequest{
		ContentItemID: up.Fields[uploadFieldContentItemID], FileName: up.fileName, ContentType: up.contentType,
		Size: up.Size, File: up.File, Finish: up.Finish, Deadline: up.Deadline,
	}
}

// coverFileOut is one cover file as staff see it. No object key, no uploader, no file name echo beyond
// what the uploader typed: the preview is its own signed URL on the article's detail.
type coverFileOut struct {
	ID string `json:"id"`
	// ContentItemID is the article the file was issued for — the id the server reserved when the upload
	// request named none. Send it as `content_item_id` on every later upload (cover or body image) of the
	// same unsaved article: without it a later body image reserves a SECOND id and the save answers 422
	// `invalid_body_image`. Always set (the row's subject).
	ContentItemID string `json:"content_item_id"`
	// MIMEType is the SNIFFED type of the original; "" while `pending`.
	MIMEType string `json:"mime_type"`
	// SizeBytes is the measured size of the original; 0 while `pending`.
	SizeBytes int64 `json:"size_bytes"`
	// Status is ADR 0052 §5's: pending · ready · rejected · failed (stored / processing are transient
	// inside one completion). Only `ready` may become a cover.
	Status string `json:"status"`
}

func coverFileFrom(f domain.StoredFile) coverFileOut {
	return coverFileOut{ID: f.ID, ContentItemID: f.SubjectID, MIMEType: f.MIMEType, SizeBytes: f.SizeBytes,
		Status: string(f.Status)}
}

// coverImageOut is the cover block of the staff DETAIL of an article.
type coverImageOut struct {
	FileID string `json:"file_id"`
	// Status of the file; "" when its row can no longer be read.
	Status string `json:"status"`
	// Public: a public copy is recorded — residents see this image now.
	Public bool `json:"public"`
	// PreviewURL is a presigned GET of the DERIVATIVE (what residents get: 1280 px JPEG, no EXIF), valid
	// until PreviewExpiresAt (≤ 15 minutes). Absent when the file is not ready or storage is not
	// configured. Open it directly; it is served from the object store's domain, never the commune's.
	PreviewURL       string     `json:"preview_url,omitempty"`
	PreviewExpiresAt *time.Time `json:"preview_expires_at,omitempty"`
}

// UploadCover receives one cover image and answers it stored. POST /api/v1/content-items/cover-images
func (h *Handler) UploadCover(w http.ResponseWriter, r *http.Request) {
	const what = "tải ảnh bìa"
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.missingCoverPrincipal(w, r)
		return
	}
	answer := func(err error) { h.answerCoverError(w, r, what, err) }
	up, ok := h.receiveUpload(w, r, what, h.d.ContentCovers.CoverUploadLimit, coverUploadFields, answer)
	if !ok {
		return
	}
	defer up.release()
	f, err := h.d.ContentCovers.UploadCover(r.Context(), coverUploadFrom(up), actor)
	if err != nil {
		answer(err)
		return
	}
	// A retry under the same Idempotency-Key learns which file the first attempt stored.
	idem.RecordCode(r.Context(), f.ID)
	vietJSON(w, http.StatusCreated, coverFileFrom(f))
}

func (h *Handler) missingCoverPrincipal(w http.ResponseWriter, r *http.Request) {
	h.d.Log.Error("tuyến ảnh bìa chạy mà không có chủ thể — SAI CẤU HÌNH ROUTE",
		"xa", string(tenant.MustFrom(r.Context())), "duong", r.URL.Path)
	httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
}

// coverView builds the detail's cover block. A failure to sign the preview is logged and the block
// still goes out without the URL: the article is worth showing without its preview.
func (h *Handler) coverView(r *http.Request, fileID string) *coverImageOut {
	v, err := h.d.ContentCovers.View(r.Context(), fileID)
	if err != nil {
		h.d.Log.Warn("ảnh bìa: không dựng được thông tin xem trước",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
	}
	out := &coverImageOut{FileID: fileID, Status: string(v.Status), Public: v.Public}
	if v.PreviewURL != "" {
		exp := v.PreviewExpiresAt
		out.PreviewURL, out.PreviewExpiresAt = v.PreviewURL.URL(), &exp
	}
	return out
}

// coverRejectionSentences is the officer-facing sentence per app.CoverReject* reason.
var coverRejectionSentences = map[string]string{
	app.CoverRejectMalware:        "Ảnh bị từ chối vì phát hiện mã độc và không được lưu.",
	app.CoverRejectTypeNotAllowed: "Ảnh bị từ chối: nội dung tệp không phải JPG, PNG hoặc WebP được phép.",
	app.CoverRejectTypeMismatch:   "Ảnh bị từ chối: nội dung tệp không đúng loại đã khai báo khi tải lên.",
	app.CoverRejectTooLarge:       "Ảnh bị từ chối: tệp lớn hơn dung lượng tối đa cho phép.",
	app.CoverRejectCountReached:   "Ảnh bị từ chối: mục nội dung đã có đủ số ảnh tối đa.",
	app.CoverRejectUndecodable:    "Ảnh bị từ chối: không đọc được nội dung ảnh. Hãy lưu lại ảnh rồi tải lên lại.",
	app.CoverRejectTooManyPixels:  "Ảnh bị từ chối: ảnh có kích thước điểm ảnh quá lớn để xử lý. Hãy thu nhỏ ảnh rồi tải lên lại.",
}

// fileRefusalLog returns the INFO line every refusal of an upload route writes (4xx of the envelope, 409,
// 422). The client reads a fixed sentence, so without it an operator cannot tell "the upload never
// arrived whole" from "it arrived in a store this service does not read" (09/10/2026, a missing temp
// bucket — petitions' fileRefusalLog). INFO and `ma_loi`, as petitions' tuChoiXuLy: a refusal is the
// rule doing its job. Commune, act, code and the wrapped error only — those errors name the commune, the
// file id and object keys, never a file name or a field value (rule 3). No `tep_id` field: the upload
// routes carry no id in their path any more, and the file id, when one exists, is in the trail.
func fileRefusalLog(r *http.Request, log *slog.Logger, msg, what string, err error) func(code string) {
	ctx := r.Context()
	return func(code string) {
		log.InfoContext(ctx, msg, "xa", string(tenant.MustFrom(ctx)), "viec", what, "ma_loi", code, "err", err)
	}
}

// answerCoverError maps the cover refusals, then hands the rest to traLoiLoiNoiDung so a content route
// answers one way. 503 for everything that is "not now" rather than "no": storage / scanner / limits
// not configured, the platform, the scanner or the temp bucket unreachable — nothing usable was stored.
func (h *Handler) answerCoverError(w http.ResponseWriter, r *http.Request, what string, err error) {
	if h.writeUploadError(w, r, what, err) {
		return
	}
	refused := fileRefusalLog(r, h.d.Log, "ảnh bìa: từ chối", what, err)
	var rej *app.CoverRejection
	switch {
	case errors.Is(err, app.ErrCoverUploadIncomplete):
		h.writeUploadStoreError(w, r, what, err)
	case errors.Is(err, app.ErrCoverFileNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy ảnh đã tải lên này.", "")
	case errors.Is(err, commsstore.ErrNoiDungKhongTonTai):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy nội dung này.", "")
	case errors.Is(err, domain.ErrCoverFileNameInvalid):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Tên tệp không hợp lệ — cần có tên, không quá 255 ký tự và không chứa ký tự điều khiển.", "")
	case errors.Is(err, domain.ErrCoverSizeInvalid):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Kích thước tệp khai báo không hợp lệ.", "")
	case errors.Is(err, app.ErrCoverTypeNotAllowed):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Loại tệp này không được phép làm ảnh bìa.", "")
	case errors.Is(err, app.ErrCoverTooLarge):
		// The declared `size` over the policy — the same fact httpx answers 413 for, so the same status.
		refused("file_too_large")
		httpx.WriteError(w, http.StatusRequestEntityTooLarge, "file_too_large",
			"Ảnh lớn hơn dung lượng tối đa được phép.", "")
	case errors.As(err, &rej):
		sentence, ok := coverRejectionSentences[rej.Reason]
		if !ok {
			sentence = "Ảnh bị từ chối và không được lưu."
		}
		refused("cover_rejected")
		httpx.WriteError(w, http.StatusUnprocessableEntity, "cover_rejected", sentence, "")
	case errors.Is(err, app.ErrCoverCountReached):
		refused("cover_limit")
		httpx.WriteError(w, http.StatusConflict, "cover_limit", "Mục nội dung đã có đủ số ảnh tối đa được phép.", "")
	case errors.Is(err, app.ErrCoverNotPending):
		refused("cover_state")
		httpx.WriteError(w, http.StatusConflict, "cover_state",
			"Ảnh này không còn ở trạng thái chờ kiểm tra. Hãy chọn ảnh và tải lên lại.", "")
	case errors.Is(err, app.ErrCoverUploadChanged):
		refused("upload_changed")
		httpx.WriteError(w, http.StatusConflict, "upload_changed",
			"Tệp vừa bị thay đổi trong lúc kiểm tra nên CHƯA được lưu. Hãy chọn ảnh và tải lên lại.", "")
	case errors.Is(err, app.ErrCoverLimitsUnavailable):
		h.d.Log.Warn("CẢNH BÁO: từ chối ảnh bìa vì chưa đọc được giới hạn tải tệp từ platform",
			"xa", string(tenant.MustFrom(r.Context())), "viec", what, "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "upload_limits_unavailable",
			"Chưa đọc được giới hạn tải tệp nên ảnh CHƯA được nhận. Vui lòng thử lại sau ít phút.", "")
	case errors.Is(err, app.ErrCoverScanUnavailable):
		h.d.Log.Warn("CẢNH BÁO: chưa quét được mã độc, ảnh giữ ở trạng thái chờ",
			"xa", string(tenant.MustFrom(r.Context())), "viec", what, "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "malware_scan_unavailable",
			"Chưa quét được mã độc cho ảnh nên ảnh CHƯA được lưu. Vui lòng thử lại sau ít phút.", "")
	default:
		h.traLoiLoiNoiDung(w, r, what, err)
	}
}
