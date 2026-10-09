package http

// THE COMMUNE'S IDENTITY IMAGES (ADR 0069) — the HTTP half of internal/app/branding.go. The routes are
// declared in routes.go; these handlers DECIDE NOTHING, they translate the use case's answer.
//
// ⚠ THE REPLY OF AN UPLOAD REQUEST CARRIES A BEARER CREDENTIAL (the presigned POST form). It goes to the
// client and nowhere else: no handler here logs a reply, a URL, a form field or a file name.

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-platform/internal/app"
	"github.com/vihat/vigov/service-platform/internal/domain"
)

// BrandingActs is the branding use case. *app.Branding satisfies it. An interface at the point of use so
// the routes' permission suite runs without MinIO, clamd or PostgreSQL.
type BrandingActs interface {
	Settings(ctx context.Context) (app.BrandingSettings, error)
	RequestUpload(ctx context.Context, img domain.BrandingImage, req app.BrandingUploadRequest,
		actor audit.Actor) (app.BrandingUpload, error)
	Complete(ctx context.Context, img domain.BrandingImage, id string, actor audit.Actor) (domain.StoredFile, error)
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

// brandingUploadIn is what the browser declares before it uploads. CHECKED against platform's
// `tenant-logo` / `tenant-banner` policy here, and the bytes are checked again at completion.
//
// ⚠ `file_name` CAN NAME A PERSON (rule 3): stored, never logged, never in an object key.
type brandingUploadIn struct {
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

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

// brandingUploadOut is the reply of an upload request. `upload` is the form: every `fields` entry as a
// form field, then the file as the LAST field named `file`, POSTed to `url`; valid until `expires_at`.
type brandingUploadOut struct {
	File   brandingFileOut       `json:"file"`
	Upload brandingPresignedPost `json:"upload"`
}

// brandingPresignedPost is a presigned POST into the temp bucket (15 minutes, ADR 0052 §1a).
type brandingPresignedPost struct {
	URL       string            `json:"url"`
	Fields    map[string]string `json:"fields"`
	ExpiresAt time.Time         `json:"expires_at"`
}

type brandingHandlers struct {
	acts BrandingActs
	urls interface{ PublicURL(key string) string }
	log  *slog.Logger
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

func (h *brandingHandlers) requestLogoUpload(w http.ResponseWriter, r *http.Request) {
	h.requestUpload(w, r, domain.BrandingLogo)
}

func (h *brandingHandlers) requestBannerUpload(w http.ResponseWriter, r *http.Request) {
	h.requestUpload(w, r, domain.BrandingBanner)
}

func (h *brandingHandlers) completeLogoUpload(w http.ResponseWriter, r *http.Request) {
	h.complete(w, r, domain.BrandingLogo)
}

func (h *brandingHandlers) completeBannerUpload(w http.ResponseWriter, r *http.Request) {
	h.complete(w, r, domain.BrandingBanner)
}

func (h *brandingHandlers) removeLogo(w http.ResponseWriter, r *http.Request) {
	h.remove(w, r, domain.BrandingLogo)
}

func (h *brandingHandlers) removeBanner(w http.ResponseWriter, r *http.Request) {
	h.remove(w, r, domain.BrandingBanner)
}

func (h *brandingHandlers) requestUpload(w http.ResponseWriter, r *http.Request, img domain.BrandingImage) {
	var in brandingUploadIn
	if !decodeBody(w, r, &in) {
		return
	}
	actor, ok := staffActor(r)
	if !ok {
		h.missingPrincipal(w, r)
		return
	}
	up, err := h.acts.RequestUpload(r.Context(), img, app.BrandingUploadRequest{
		FileName: in.FileName, ContentType: in.ContentType, Size: in.Size,
	}, actor)
	if err != nil {
		h.answerError(w, r, "xin tải ảnh nhận diện", err)
		return
	}
	// A form is a bearer credential: no cache between here and the officer's browser keeps it.
	noStore(w)
	writeJSON(w, http.StatusCreated, brandingUploadOut{
		File:   h.fileOut(up.File),
		Upload: brandingPresignedPost{URL: up.Post.URL, Fields: up.Post.Fields, ExpiresAt: up.Post.ExpiresAt},
	})
}

func (h *brandingHandlers) complete(w http.ResponseWriter, r *http.Request, img domain.BrandingImage) {
	actor, ok := staffActor(r)
	if !ok {
		h.missingPrincipal(w, r)
		return
	}
	f, err := h.acts.Complete(r.Context(), img, r.PathValue("id"), actor)
	if err != nil {
		h.answerError(w, r, "hoàn tất ảnh nhận diện", err)
		return
	}
	writeJSON(w, http.StatusOK, h.fileOut(f))
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

// fileRefusalLog returns the INFO line every 409/422 of an upload route writes. The client reads a fixed
// sentence, so without it an operator cannot tell "the upload never arrived" from "it arrived in a store
// this service does not read" (09/10/2026, a missing temp bucket — petitions' fileRefusalLog). INFO and
// `ma_loi`, as petitions' tuChoiXuLy: a refusal is the rule doing its job. Commune, file id, code and the
// wrapped error only — those errors name the commune and object keys, never a file name (rule 3).
func fileRefusalLog(r *http.Request, log *slog.Logger, msg, what string, err error) func(code string) {
	ctx := r.Context()
	return func(code string) {
		log.InfoContext(ctx, msg, "xa", string(tenant.MustFrom(ctx)), "viec", what,
			"tep_id", r.PathValue("id"), "ma_loi", code, "err", err)
	}
}

// answerError maps the use case's refusals. 503 for everything that is "not now" rather than "no":
// storage / scanner / limits not configured or unreachable — nothing was stored in any of them.
func (h *brandingHandlers) answerError(w http.ResponseWriter, r *http.Request, what string, err error) {
	refused := fileRefusalLog(r, h.log, "nhận diện xã: từ chối", what, err)
	var rej *app.BrandingRejection
	switch {
	case errors.Is(err, app.ErrBrandingFileNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy ảnh đã tải lên này.", "")
	case errors.Is(err, domain.ErrBrandingFileNameInvalid):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Tên tệp không hợp lệ — cần có tên, không quá 255 ký tự và không chứa ký tự điều khiển.", "")
	case errors.Is(err, domain.ErrBrandingSizeInvalid):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Kích thước tệp khai báo không hợp lệ.", "")
	case errors.Is(err, app.ErrBrandingTypeNotAllowed):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Loại tệp này không được phép. Hãy chọn ảnh PNG, WebP hoặc JPEG.", "")
	case errors.Is(err, app.ErrBrandingTooLarge):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Ảnh lớn hơn dung lượng tối đa được phép.", "")
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
			"Ảnh này đã bị từ chối hoặc lượt tải đã hết hạn. Hãy chọn ảnh và tải lên lại.", "")
	case errors.Is(err, app.ErrBrandingUploadNotReceived):
		refused("upload_not_received")
		httpx.WriteError(w, http.StatusConflict, "upload_not_received",
			"Chưa nhận được tệp. Hãy chờ tải lên xong rồi bấm hoàn tất lại.", "")
	case errors.Is(err, app.ErrBrandingUploadExpired):
		refused("upload_expired")
		httpx.WriteError(w, http.StatusConflict, "upload_expired",
			"Lượt tải lên đã hết hạn mà chưa nhận được tệp. Hãy chọn ảnh và tải lên lại.", "")
	case errors.Is(err, app.ErrBrandingUploadChanged):
		refused("upload_changed")
		httpx.WriteError(w, http.StatusConflict, "upload_changed",
			"Tệp vừa bị thay đổi trong lúc kiểm tra. Hãy bấm hoàn tất lại.", "")
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
			"Hệ thống đang xử lý ảnh khác nên ảnh CHƯA được nhận. Vui lòng bấm hoàn tất lại sau ít giây.", "")
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
