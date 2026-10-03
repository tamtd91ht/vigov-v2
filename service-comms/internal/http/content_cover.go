package http

// THE COVER UPLOAD OF A MINI APP ARTICLE (docs/ui-ux/11-noi-dung-mini-app.md §7 `Ảnh đại diện`) — the
// HTTP half of internal/app/content_cover.go:
//
//	POST /api/v1/content-items/cover-images                   content.update — pending row + presigned POST
//	POST /api/v1/content-items/cover-images/{id}/completion   content.update — the uploader only
//
// The file then rides on the article: `cover_image_file_id` on POST / PATCH /api/v1/content-items.
//
// `cover-images` UNDER `content-items`, NOT A NEW FIRST SEGMENT: the noun is migration 0011's own column
// (`cover_image_file_id`), and tools/ingress groups routes by the first segment after /api/v1/, which is
// already this service's. It sits at COLLECTION level, not under `{id}`, because §7's modal uploads
// before `Lưu` — the article may not exist yet (0011); an existing article is named in the body.
//
// THESE HANDLERS DECIDE NOTHING: they translate the use case's answer.
//
// ⚠ THE REPLY OF THE FIRST ROUTE CARRIES A BEARER CREDENTIAL (the presigned POST form), and the staff
// detail carries a presigned GET. They go to the client and nowhere else: no handler here logs a reply,
// a URL, a form field or a file name.

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

// ContentCoverActs is the cover upload, completion and staff preview. *app.ContentCovers satisfies it.
// ITS OWN INTERFACE and not more methods on GhiNoiDungMiniApp: its dependencies — the object store, the
// scanner, platform's limits — may be absent in a deployment while every other content route works.
type ContentCoverActs interface {
	RequestUpload(ctx context.Context, req app.CoverUploadRequest, actor audit.Actor) (app.CoverUpload, error)
	Complete(ctx context.Context, id string, actor audit.Actor) (domain.StoredFile, error)
	View(ctx context.Context, fileID string) (app.CoverView, error)

	// The body images (content_body_image.go): the same acts under `content-body-image`, and the staff
	// previews of the images one article's body references.
	RequestBodyImageUpload(ctx context.Context, req app.CoverUploadRequest, actor audit.Actor) (app.CoverUpload, error)
	CompleteBodyImageUpload(ctx context.Context, id string, actor audit.Actor) (domain.StoredFile, error)
	BodyImageViews(ctx context.Context, itemID string, fileIDs []string) ([]app.BodyImageView, error)
	// FetchBodyImage: the server downloads a pasted https link into a ready body image (H5, K6).
	FetchBodyImage(ctx context.Context, req app.BodyImageFromURLRequest, actor audit.Actor) (domain.StoredFile, error)
}

// coverUploadIn is what the browser declares before it uploads. CHECKED against platform's
// `content-image` policy here and the bytes are checked again at completion (ADR 0052 §1c).
//
// ⚠ `file_name` CAN NAME A PERSON (rule 3): stored, never logged, never in an object key.
type coverUploadIn struct {
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`

	// ContentItemID names an EXISTING article whose cover is being replaced. ABSENT for an article not
	// saved yet: the server then reserves an id, and POST /api/v1/content-items with this upload's
	// `cover_image_file_id` creates the article under it.
	ContentItemID string `json:"content_item_id,omitempty"`
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

// coverUploadOut is the reply of POST …/cover-images. `upload` is the form: every `fields` entry as a
// form field, then the file as the LAST field named `file`, POSTed to `url`; valid until `expires_at`.
type coverUploadOut struct {
	CoverImage coverFileOut `json:"cover_image"`
	// ContentItemID is the article this cover belongs to — the reserved id when the request named none.
	// The same place in the reply as bodyImageUploadOut's, so web-admin reads one shape for both uploads.
	ContentItemID string             `json:"content_item_id"`
	Upload        presignedUploadOut `json:"upload"`
}

// presignedUploadOut is a presigned POST into the temp bucket (15 minutes, ADR 0052 §1a).
type presignedUploadOut struct {
	URL       string            `json:"url"`
	Fields    map[string]string `json:"fields"`
	ExpiresAt time.Time         `json:"expires_at"`
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

// RequestCoverUpload issues one upload slot. POST /api/v1/content-items/cover-images
func (h *Handler) RequestCoverUpload(w http.ResponseWriter, r *http.Request) {
	var in coverUploadIn
	if !docThan(w, r, &in) {
		return
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.missingCoverPrincipal(w, r)
		return
	}
	up, err := h.d.ContentCovers.RequestUpload(r.Context(), app.CoverUploadRequest{
		ContentItemID: in.ContentItemID, FileName: in.FileName, ContentType: in.ContentType, Size: in.Size,
	}, actor)
	if err != nil {
		h.answerCoverError(w, r, "xin tải ảnh bìa", err)
		return
	}
	// A form is a bearer credential: no cache between here and the officer's browser keeps it.
	w.Header().Set("Cache-Control", "no-store")
	vietJSON(w, http.StatusCreated, coverUploadOut{
		CoverImage:    coverFileFrom(up.File),
		ContentItemID: up.File.SubjectID,
		Upload:        presignedUploadOut{URL: up.Post.URL, Fields: up.Post.Fields, ExpiresAt: up.Post.ExpiresAt},
	})
}

// CompleteCoverUpload runs ADR 0052 §1c and the derivative on one upload.
// POST /api/v1/content-items/cover-images/{id}/completion
func (h *Handler) CompleteCoverUpload(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.missingCoverPrincipal(w, r)
		return
	}
	f, err := h.d.ContentCovers.Complete(r.Context(), r.PathValue("id"), actor)
	if err != nil {
		h.answerCoverError(w, r, "hoàn tất ảnh bìa", err)
		return
	}
	vietJSON(w, http.StatusOK, coverFileFrom(f))
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

// answerCoverError maps the cover refusals, then hands the rest to traLoiLoiNoiDung so a content route
// answers one way. 503 for everything that is "not now" rather than "no": storage / scanner / limits
// not configured, the platform or the scanner unreachable — nothing was stored in any of them.
func (h *Handler) answerCoverError(w http.ResponseWriter, r *http.Request, what string, err error) {
	var rej *app.CoverRejection
	switch {
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
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Ảnh lớn hơn dung lượng tối đa được phép.", "")
	case errors.As(err, &rej):
		sentence, ok := coverRejectionSentences[rej.Reason]
		if !ok {
			sentence = "Ảnh bị từ chối và không được lưu."
		}
		httpx.WriteError(w, http.StatusUnprocessableEntity, "cover_rejected", sentence, "")
	case errors.Is(err, app.ErrCoverCountReached):
		httpx.WriteError(w, http.StatusConflict, "cover_limit", "Mục nội dung đã có đủ số ảnh tối đa được phép.", "")
	case errors.Is(err, app.ErrCoverNotPending):
		httpx.WriteError(w, http.StatusConflict, "cover_state",
			"Ảnh này đã bị từ chối hoặc lượt tải đã hết hạn. Hãy chọn ảnh và tải lên lại.", "")
	case errors.Is(err, app.ErrCoverUploadNotReceived):
		httpx.WriteError(w, http.StatusConflict, "upload_not_received",
			"Chưa nhận được tệp. Hãy chờ tải lên xong rồi bấm hoàn tất lại.", "")
	case errors.Is(err, app.ErrCoverUploadExpired):
		httpx.WriteError(w, http.StatusConflict, "upload_expired",
			"Lượt tải lên đã hết hạn mà chưa nhận được tệp. Hãy chọn ảnh và tải lên lại.", "")
	case errors.Is(err, app.ErrCoverUploadChanged):
		httpx.WriteError(w, http.StatusConflict, "upload_changed",
			"Tệp vừa bị thay đổi trong lúc kiểm tra. Hãy bấm hoàn tất lại.", "")
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
