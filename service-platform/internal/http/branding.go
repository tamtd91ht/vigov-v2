package http

// THE COMMUNE'S IDENTITY IMAGES (ADR 0069) — the HTTP half of internal/app/branding.go. The routes are
// declared in routes.go; these handlers DECIDE NOTHING, they translate the use case's answer.
//
// AN UPLOAD IS ONE multipart REQUEST (ADR 0052 §Sửa đổi 09/10/2026). No handler here logs a field value, a file
// name or file content (rule 3): a file name is often a person's name.

import (
	"context"
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/storage"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-platform/internal/app"
	"github.com/vihat/vigov/service-platform/internal/domain"
)

// BrandingActs is the branding use case. *app.Branding satisfies it. An interface at the point of use so
// the routes' permission suite runs without MinIO, clamd or PostgreSQL.
type BrandingActs interface {
	Settings(ctx context.Context) (app.BrandingSettings, error)
	MaxUploadBytes(ctx context.Context, img domain.BrandingImage) (int64, error)
	Upload(ctx context.Context, img domain.BrandingImage, req app.BrandingUploadRequest,
		actor audit.Actor) (domain.StoredFile, error)
	Remove(ctx context.Context, img domain.BrandingImage, actor audit.Actor) (bool, error)
}

// brandingSettingsOut is the Cấu hình › Nhận diện xã tab. "" = not set (the building icon / no strip,
// ADR 0069 #7). The URLs are the SAME public URLs the login screen and the Mini App receive.
type brandingSettingsOut struct {
	LogoPublicURL           string `json:"logo_public_url"`
	WebAdminBannerPublicURL string `json:"web_admin_banner_public_url"`
	// UpdatedAt / UpdatedBy: who last changed the commune's display profile (CB- code); absent when the
	// commune has no profile yet.
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
	UpdatedBy string     `json:"updated_by,omitempty"`
}

// brandingFileNameField is the optional text part naming the file; absent, the file part's own filename
// is used. ⚠ IT CAN NAME A PERSON (rule 3): stored, never logged, never in an object key.
const brandingFileNameField = "file_name"

// brandingFileOut is one uploaded file as staff see it. No object key, no uploader, no file name.
type brandingFileOut struct {
	ID string `json:"id"`
	// MIMEType is the SNIFFED type of the original; "" while `pending`.
	MIMEType string `json:"mime_type"`
	// SizeBytes is the measured size of the original; 0 while `pending`.
	SizeBytes int64 `json:"size_bytes"`
	// Status is ADR 0052 §5's: pending · ready · rejected · failed. `ready` means published and current
	// (or current at the time — a later upload may have replaced it).
	Status string `json:"status"`
	// PublicURL is the anonymous URL of the published 512 px PNG / 1600 px JPEG; "" until ready.
	PublicURL string `json:"public_url"`
}

type brandingHandlers struct {
	acts BrandingActs
	urls interface{ PublicURL(key string) string }
	// slots is the PROCESS's one httpx.UploadSlots, shared with every other upload route it may grow.
	slots *httpx.UploadSlots
	log   *slog.Logger
}

func (h *brandingHandlers) fileOut(f domain.StoredFile) brandingFileOut {
	out := brandingFileOut{ID: f.ID, MIMEType: f.MIMEType, SizeBytes: f.SizeBytes, Status: string(f.Status)}
	if h.urls != nil {
		out.PublicURL = h.urls.PublicURL(f.PublicObjectKey)
	}
	return out
}

// staffActor builds the audit actor from the request.
//
// THE TRAIL RECORDS THE BUSINESS CODE `Ma`, NEVER THE INTERNAL ID (rule 6, invariant 8). AN EMPTY `Ma`
// REFUSES THE WRITE and never falls back to p.ID — a fallback would put internal ids back into
// audit_log.actor_id silently, the defect of 2026-09-22.
func staffActor(r *http.Request) (audit.Actor, bool) {
	p, ok := authz.From(r.Context())
	if !ok || p.Ma == "" {
		return audit.Actor{}, false
	}
	return audit.Actor{ID: p.Ma, Kind: p.Kind, IP: httpx.ClientIP(r)}, true
}

func (h *brandingHandlers) settings(w http.ResponseWriter, r *http.Request) {
	s, err := h.acts.Settings(r.Context())
	if err != nil {
		h.answerError(w, r, "đọc nhận diện xã", err)
		return
	}
	out := brandingSettingsOut{LogoPublicURL: s.LogoPublicURL, WebAdminBannerPublicURL: s.WebAdminBannerPublicURL,
		UpdatedBy: s.UpdatedBy}
	if !s.UpdatedAt.IsZero() {
		at := s.UpdatedAt
		out.UpdatedAt = &at
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *brandingHandlers) uploadLogo(w http.ResponseWriter, r *http.Request) {
	h.upload(w, r, domain.BrandingLogo)
}

func (h *brandingHandlers) uploadBanner(w http.ResponseWriter, r *http.Request) {
	h.upload(w, r, domain.BrandingBanner)
}

func (h *brandingHandlers) removeLogo(w http.ResponseWriter, r *http.Request) {
	h.remove(w, r, domain.BrandingLogo)
}

func (h *brandingHandlers) removeBanner(w http.ResponseWriter, r *http.Request) {
	h.remove(w, r, domain.BrandingBanner)
}

// upload is one image upload, in core/httpx/upload.go's order: the actor, a slot (before a byte is
// read), the cap from the CURRENT policy, the envelope (ReadUpload), then the use case, which streams the
// file into the temp bucket, confirms the transport (Finish) and completes. 201 with the stored file.
func (h *brandingHandlers) upload(w http.ResponseWriter, r *http.Request, img domain.BrandingImage) {
	const what = "tải ảnh nhận diện"
	actor, ok := staffActor(r)
	if !ok {
		h.missingPrincipal(w, r)
		return
	}
	release, ok := h.slots.Acquire(r.Context())
	defer release()
	if !ok {
		h.log.InfoContext(r.Context(), "nhận diện xã: hết lượt tải đồng thời của pod — trả 503",
			"xa", string(tenant.MustFrom(r.Context())), "viec", what, "ma_loi", httpx.UploadBusyCode)
		httpx.WriteUploadBusy(w, "")
		return
	}
	maxBytes, err := h.acts.MaxUploadBytes(r.Context(), img)
	if err != nil {
		h.answerError(w, r, what, err)
		return
	}
	up, err := httpx.ReadUpload(w, r, httpx.UploadOptions{MaxFileBytes: maxBytes, Fields: []string{brandingFileNameField}})
	if err != nil {
		h.answerError(w, r, what, err)
		return
	}
	name := up.Fields[brandingFileNameField]
	if name == "" {
		name = up.Filename
	}
	f, err := h.acts.Upload(r.Context(), img, app.BrandingUploadRequest{
		FileName: name, ContentType: declaredType(up.ContentType), Size: up.Size,
		Body: up.File, Deadline: up.Deadline, Received: up.Finish,
	}, actor)
	if err != nil {
		h.answerError(w, r, what, err)
		return
	}
	writeJSON(w, http.StatusCreated, h.fileOut(f))
}

// declaredType is the file part's Content-Type without parameters, lower-cased — a hint only (the
// completion sniffs the bytes), but the policy matches it exactly. Unparseable → "", refused as a type.
func declaredType(raw string) string {
	mt, _, err := mime.ParseMediaType(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(mt)
}

func (h *brandingHandlers) remove(w http.ResponseWriter, r *http.Request, img domain.BrandingImage) {
	actor, ok := staffActor(r)
	if !ok {
		h.missingPrincipal(w, r)
		return
	}
	if _, err := h.acts.Remove(r.Context(), img, actor); err != nil {
		h.answerError(w, r, "gỡ ảnh nhận diện", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *brandingHandlers) missingPrincipal(w http.ResponseWriter, r *http.Request) {
	h.log.Error("tuyến nhận diện xã chạy mà không có mã cán bộ — SAI CẤU HÌNH ROUTE hoặc identity cũ",
		"xa", string(tenant.MustFrom(r.Context())), "duong", r.URL.Path)
	httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
}

// brandingRejectionSentences is the officer-facing sentence per app.BrandingReject* reason.
var brandingRejectionSentences = map[string]string{
	app.BrandingRejectMalware:        "Ảnh bị từ chối vì phát hiện mã độc và không được lưu.",
	app.BrandingRejectTypeNotAllowed: "Ảnh bị từ chối: nội dung tệp không phải PNG, WebP hoặc JPEG được phép.",
	app.BrandingRejectTypeMismatch:   "Ảnh bị từ chối: nội dung tệp không đúng loại đã khai báo khi tải lên.",
	app.BrandingRejectTooLarge:       "Ảnh bị từ chối: tệp lớn hơn dung lượng tối đa cho phép.",
	app.BrandingRejectUndecodable:    "Ảnh bị từ chối: không đọc được nội dung ảnh. Hãy lưu lại ảnh rồi tải lên lại.",
	app.BrandingRejectTooManyPixels:  "Ảnh bị từ chối: ảnh có kích thước điểm ảnh quá lớn để xử lý. Hãy thu nhỏ ảnh rồi tải lên lại.",
}

// fileRefusalLog returns the INFO line every refusal of an upload that is the client's or the file's
// (408/409/413/415/422, and the 400s of the transport) writes. The client reads a fixed sentence, so
// without it an operator cannot tell "the upload never arrived" from "it arrived in a store this service
// does not read" (09/10/2026, a missing temp bucket — petitions' fileRefusalLog). INFO and `ma_loi`, as
// petitions' tuChoiXuLy: a refusal is the rule doing its job. Commune, code and the wrapped error only —
// the use case's errors name the file id (Upload wraps every error after the row exists) and object
// keys, never a file name; httpx's never carry a client value (rule 3).
func fileRefusalLog(r *http.Request, log *slog.Logger, msg, what string, err error) func(code string) {
	ctx := r.Context()
	return func(code string) {
		log.InfoContext(ctx, msg, "xa", string(tenant.MustFrom(ctx)), "viec", what, "ma_loi", code, "err", err)
	}
}

// answerUploadTransport answers the refusals of the request body itself — httpx's sentinels, and
// storage's when the stream did not match its declaration. false = err is none of them.
func answerUploadTransport(w http.ResponseWriter, err error, refused func(code string)) bool {
	var status int
	var code, sentence string
	switch st := httpx.UploadErrorStatus(err); {
	case st == http.StatusUnsupportedMediaType:
		status, code, sentence = st, "unsupported_media_type",
			"Ảnh phải được gửi dạng multipart/form-data."
	case st == http.StatusRequestEntityTooLarge, errors.Is(err, storage.ErrTooLarge), errors.Is(err, app.ErrBrandingTooLarge):
		status, code, sentence = http.StatusRequestEntityTooLarge, "file_too_large",
			"Ảnh lớn hơn dung lượng tối đa được phép."
	case st == http.StatusRequestTimeout:
		status, code, sentence = st, "upload_timeout",
			"Tải ảnh lên quá thời gian cho phép nên ảnh CHƯA được nhận. Vui lòng kiểm tra kết nối rồi tải lên lại."
	case st == http.StatusBadRequest, errors.Is(err, storage.ErrSizeMismatch):
		status, code, sentence = http.StatusBadRequest, "invalid_upload",
			"Tệp gửi lên không trọn vẹn hoặc sai định dạng nên ảnh CHƯA được nhận. Vui lòng chọn ảnh và tải lên lại."
	default:
		return false
	}
	refused(code)
	httpx.WriteError(w, status, code, sentence, "")
	return true
}

// answerError maps the use case's refusals. 503 for everything that is "not now" rather than "no":
// storage / scanner / limits not configured or unreachable — nothing was stored in any of them.
func (h *brandingHandlers) answerError(w http.ResponseWriter, r *http.Request, what string, err error) {
	refused := fileRefusalLog(r, h.log, "nhận diện xã: từ chối", what, err)
	if answerUploadTransport(w, err, refused) {
		return
	}
	var rej *app.BrandingRejection
	switch {
	case errors.Is(err, app.ErrBrandingFileNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy ảnh đã tải lên này.", "")
	case errors.Is(err, domain.ErrBrandingFileNameInvalid):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Tên tệp không hợp lệ — cần có tên, không quá 255 ký tự và không chứa ký tự điều khiển.", "")
	case errors.Is(err, domain.ErrBrandingSizeInvalid):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Kích thước tệp khai báo không hợp lệ.", "")
	case errors.Is(err, app.ErrBrandingTypeNotAllowed), errors.Is(err, storage.ErrTypeNotAllowed):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Loại tệp này không được phép. Hãy chọn ảnh PNG, WebP hoặc JPEG.", "")
	case errors.As(err, &rej):
		sentence, ok := brandingRejectionSentences[rej.Reason]
		if !ok {
			sentence = "Ảnh bị từ chối và không được lưu."
		}
		refused("image_rejected")
		httpx.WriteError(w, http.StatusUnprocessableEntity, "image_rejected", sentence, "")
	case errors.Is(err, domain.ErrProfileDeleted):
		// 409, NOT 404 OR 500: the commune exists and the caller may manage it; what refuses is the
		// state of ONE row — its display profile was retired, and reviving it is an operator's decision.
		h.log.Warn("nhận diện xã: từ chối ghi vì hồ sơ hiển thị của xã đã bị xoá mềm",
			"xa", string(tenant.MustFrom(r.Context())), "viec", what)
		httpx.WriteError(w, http.StatusConflict, "profile_deleted",
			"Hồ sơ hiển thị của xã đã bị gỡ nên chưa đặt được ảnh nhận diện. Vui lòng liên hệ đơn vị vận hành.", "")
	case errors.Is(err, app.ErrBrandingCountReached):
		refused("upload_limit")
		httpx.WriteError(w, http.StatusConflict, "upload_limit", "Đã đủ số ảnh tải lên tối đa được phép.", "")
	case errors.Is(err, app.ErrBrandingNotPending):
		refused("upload_state")
		httpx.WriteError(w, http.StatusConflict, "upload_state",
			"Ảnh này vừa được xử lý ở một lượt khác. Hãy tải lại trang để xem ảnh hiện tại.", "")
	case errors.Is(err, app.ErrBrandingUploadChanged):
		refused("upload_changed")
		httpx.WriteError(w, http.StatusConflict, "upload_changed",
			"Tệp vừa bị thay đổi trong lúc kiểm tra nên ảnh CHƯA được nhận. Hãy tải ảnh lên lại.", "")
	case errors.Is(err, app.ErrBrandingUploadNotConfigured):
		h.log.Warn("CẢNH BÁO: từ chối ảnh nhận diện vì chưa cấu hình kho lưu tệp / máy quét / giới hạn",
			"xa", string(tenant.MustFrom(r.Context())), "viec", what, "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "storage_not_configured",
			"Hệ thống chưa sẵn sàng nhận ảnh. Vui lòng liên hệ đơn vị vận hành.", "")
	case errors.Is(err, app.ErrBrandingLimitsUnavailable):
		h.log.Warn("CẢNH BÁO: từ chối ảnh nhận diện vì chưa đọc được giới hạn tải tệp",
			"xa", string(tenant.MustFrom(r.Context())), "viec", what, "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "upload_limits_unavailable",
			"Chưa đọc được giới hạn tải tệp nên ảnh CHƯA được nhận. Vui lòng thử lại sau ít phút.", "")
	case errors.Is(err, app.ErrBrandingScanUnavailable):
		h.log.Warn("CẢNH BÁO: chưa quét được mã độc, ảnh nhận diện giữ ở trạng thái chờ",
			"xa", string(tenant.MustFrom(r.Context())), "viec", what, "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "malware_scan_unavailable",
			"Chưa quét được mã độc cho ảnh nên ảnh CHƯA được lưu. Vui lòng thử lại sau ít phút.", "")
	case errors.Is(err, app.ErrBrandingDecodeBusy):
		h.log.Warn("nhận diện xã: hết chờ lượt xử lý ảnh — trả 503, không ghi gì",
			"xa", string(tenant.MustFrom(r.Context())), "viec", what)
		httpx.WriteError(w, http.StatusServiceUnavailable, "image_processing_busy",
			"Hệ thống đang xử lý ảnh khác nên ảnh CHƯA được nhận. Vui lòng tải ảnh lên lại sau ít giây.", "")
	case errors.Is(err, app.ErrBrandingPublishUnavailable):
		h.log.Warn("CẢNH BÁO: chưa đăng được ảnh nhận diện lên kho công khai — không ghi gì",
			"xa", string(tenant.MustFrom(r.Context())), "viec", what, "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "publish_unavailable",
			"Chưa đăng được ảnh nên ảnh CHƯA được đặt. Vui lòng thử lại sau ít phút.", "")
	default:
		h.log.Error("nhận diện xã: lỗi không lường trước",
			"xa", string(tenant.MustFrom(r.Context())), "viec", what, "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}
