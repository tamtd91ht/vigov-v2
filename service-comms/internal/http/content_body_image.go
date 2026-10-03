package http

// THE BODY IMAGES OF A MINI APP ARTICLE (ADR 0067 §Sửa đổi 03/10/2026, H1, K2, K3, K8) — the HTTP half of
// the body-image acts in internal/app/content_cover.go:
//
//	POST /api/v1/content-items/body-images                   content.update — pending row + presigned POST
//	POST /api/v1/content-items/body-images/{id}/completion   content.update — the uploader only
//	POST /api/v1/content-items/body-images/from-url          content.update — the server fetches a pasted
//	                                                         https link (H5, K6; app/content_body_image_url.go)
//
// The file then rides on the article INSIDE the body: `<figure><img data-file-id="{id}" alt="…">
// <figcaption>…</figcaption></figure>` in `body` on POST / PATCH /api/v1/content-items, checked on every
// save (422 `invalid_body_image` otherwise). A URL is never stored and never accepted (K2).
//
// `body-images` UNDER `content-items`, NEXT TO `cover-images` and for its reasons (content_cover.go): the
// article may not exist yet, so the routes sit at collection level and the article is named in the body.
// Unlike the cover, an article may hold many body images before it is saved, so the reply carries the
// article id the FIRST upload reserved; every later upload of the same unsaved article — body image or
// cover — sends it back as `content_item_id`.
//
// ⚠ THE REPLY OF THE FIRST ROUTE CARRIES A BEARER CREDENTIAL (the presigned POST form), and the staff detail
// carries presigned GETs. They go to the client and nowhere else: nothing here logs a reply, a URL, a form
// field or a file name.

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
	"github.com/vihat/vigov/service-comms/internal/imagefetch"
)

// bodyImageUploadIn is what the browser declares before it uploads one body image. CHECKED against
// platform's `content-body-image` policy (types, size, and the per-article count — H7) here, and the bytes
// again at completion (ADR 0052 §1c).
//
// ⚠ `file_name` CAN NAME A PERSON (rule 3): stored, never logged, never in an object key.
type bodyImageUploadIn struct {
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`

	// ContentItemID is ABSENT for the first image of an article not saved yet (the server reserves an id
	// and returns it). Otherwise: an existing article of this commune, or the id an earlier upload of THIS
	// officer reserved. Anything else → 404.
	ContentItemID string `json:"content_item_id,omitempty"`
}

// bodyImageUploadOut is the reply of POST …/body-images. `upload` is the form: every `fields` entry as a
// form field, then the file as the LAST field named `file`, POSTed to `url`; valid until `expires_at`.
type bodyImageUploadOut struct {
	BodyImage coverFileOut `json:"body_image"`
	// ContentItemID is the article this image belongs to — the reserved id when the request named none.
	// Send it as `content_item_id` on every later upload (body image or cover) of the same unsaved article.
	ContentItemID string             `json:"content_item_id"`
	Upload        presignedUploadOut `json:"upload"`
}

// bodyImageOut is one image the body references, on the staff DETAIL of an article (GET
// /api/v1/content-items/{id}), in body order — what the editor draws each `<img data-file-id>` with.
type bodyImageOut struct {
	FileID string `json:"file_id"`
	// Status of the file; "" when the id is no live body image of this article (the public read drops it).
	Status string `json:"status"`
	// Public: a public copy is recorded — residents see this image now.
	Public bool `json:"public"`
	// PreviewURL is a presigned GET of the DERIVATIVE (1280 px JPEG, no EXIF), valid until PreviewExpiresAt
	// (≤ 15 minutes). Absent when the file is not a ready body image of this article or storage is not
	// configured. Served from the object store's domain; never a URL the client chose.
	PreviewURL       string     `json:"preview_url,omitempty"`
	PreviewExpiresAt *time.Time `json:"preview_expires_at,omitempty"`
}

// RequestBodyImageUpload issues one upload slot. POST /api/v1/content-items/body-images
func (h *Handler) RequestBodyImageUpload(w http.ResponseWriter, r *http.Request) {
	var in bodyImageUploadIn
	if !docThan(w, r, &in) {
		return
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.missingCoverPrincipal(w, r)
		return
	}
	up, err := h.d.ContentCovers.RequestBodyImageUpload(r.Context(), app.CoverUploadRequest{
		ContentItemID: in.ContentItemID, FileName: in.FileName, ContentType: in.ContentType, Size: in.Size,
	}, actor)
	if err != nil {
		h.answerBodyImageError(w, r, "xin tải ảnh thân bài", err)
		return
	}
	// A form is a bearer credential: no cache between here and the officer's browser keeps it.
	w.Header().Set("Cache-Control", "no-store")
	vietJSON(w, http.StatusCreated, bodyImageUploadOut{
		BodyImage:     coverFileFrom(up.File),
		ContentItemID: up.File.SubjectID,
		Upload:        presignedUploadOut{URL: up.Post.URL, Fields: up.Post.Fields, ExpiresAt: up.Post.ExpiresAt},
	})
}

// CompleteBodyImageUpload runs ADR 0052 §1c and the derivative on one body-image upload.
// POST /api/v1/content-items/body-images/{id}/completion
func (h *Handler) CompleteBodyImageUpload(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.missingCoverPrincipal(w, r)
		return
	}
	f, err := h.d.ContentCovers.CompleteBodyImageUpload(r.Context(), r.PathValue("id"), actor)
	if err != nil {
		h.answerBodyImageError(w, r, "hoàn tất ảnh thân bài", err)
		return
	}
	vietJSON(w, http.StatusOK, h.bodyImageFileReply(w, r, f))
}

// bodyImageFileReply builds bodyImageFileOut for a body-image row — the completion's reply and the
// from-url reply alike.
func (h *Handler) bodyImageFileReply(w http.ResponseWriter, r *http.Request, f domain.StoredFile) bodyImageFileOut {
	out := bodyImageFileOut{ID: f.ID, ContentItemID: f.SubjectID, MIMEType: f.MIMEType, SizeBytes: f.SizeBytes,
		Status: string(f.Status)}
	if f.Status == domain.StoredFileReady {
		// THE EDITOR'S ONLY WAY TO SHOW THE IMAGE BEFORE THE FIRST SAVE: web-admin draws no local file
		// (no blob:, no createObjectURL), so the preview is the server's — the same signed derivative link
		// and TTL as GET /content-items/{id}'s `body_images` (BodyImageViews, one read, offline signature).
		if views := h.bodyImageViews(r, f.SubjectID, []string{f.ID}); len(views) == 1 && views[0].PreviewURL != "" {
			out.PreviewURL, out.PreviewExpiresAt = views[0].PreviewURL, views[0].PreviewExpiresAt
			// A presigned preview is a bearer credential: no shared cache keeps this reply.
			w.Header().Set("Cache-Control", "no-store")
		}
	}
	return out
}

// bodyImageFromURLIn is a pasted image link. ⚠ THE URL CAN CARRY A TOKEN OR A PERSON'S DATA in its path
// or query (rule 3): never logged, never echoed, never stored — the trail keeps its host.
type bodyImageFromURLIn struct {
	// URL is the pasted https link: port 443 only, no user:password@, a DNS host name (no IP address),
	// at most 2048 characters. ANY host — not only .gov.vn (K6).
	URL string `json:"url"`
	// ContentItemID follows the upload request's rule: ABSENT for the first image of an unsaved article
	// (the server reserves an id, returned as `content_item_id`); otherwise a live article of this commune
	// or an id an earlier upload of THIS officer reserved. Anything else → 404.
	ContentItemID string `json:"content_item_id,omitempty"`
}

// FetchBodyImageFromURL has the server download a pasted image link into a ready body image.
// POST /api/v1/content-items/body-images/from-url
func (h *Handler) FetchBodyImageFromURL(w http.ResponseWriter, r *http.Request) {
	var in bodyImageFromURLIn
	if !docThan(w, r, &in) {
		return
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.missingCoverPrincipal(w, r)
		return
	}
	f, err := h.d.ContentCovers.FetchBodyImage(r.Context(),
		app.BodyImageFromURLRequest{URL: in.URL, ContentItemID: in.ContentItemID}, actor)
	h.logBodyImageFetch(r, actor, in.URL, err)
	if err != nil {
		h.answerBodyImageFromURLError(w, r, err)
		return
	}
	vietJSON(w, http.StatusCreated, h.bodyImageFileReply(w, r, f))
}

// logBodyImageFetch records ONE line per pasted link: commune, actor, HOST and outcome — never the path,
// the query or a remote error text (rule 3). A destination this side refused is the security event
// `outbound_url_refused`, as in the portal sync (R2, 02/10/2026).
func (h *Handler) logBodyImageFetch(r *http.Request, actor audit.Actor, rawURL string, err error) {
	host := ""
	if u, perr := imagefetch.ParseURL(rawURL); perr == nil {
		host = strings.ToLower(u.Hostname())
	}
	xa := string(tenant.MustFrom(r.Context()))
	var fe *app.ImageFetchError
	switch {
	case err == nil:
		h.d.Log.Info("ảnh thân bài: đã tải ảnh từ liên kết", "xa", xa, "actor", actor.ID, "host", host,
			"outcome", "stored")
	case errors.As(err, &fe) && fe.Refused:
		h.d.Log.Warn("CẢNH BÁO BẢO MẬT: từ chối một địa chỉ gọi ra ngoài",
			"event", "outbound_url_refused", "outcome", "refused", "xa", xa, "actor", actor.ID,
			"class", fe.Class, "call", "body-image", "host", host)
	case errors.As(err, &fe):
		h.d.Log.Warn("ảnh thân bài: không tải được ảnh từ liên kết", "xa", xa, "actor", actor.ID, "host", host,
			"outcome", "fetch-failed", "class", fe.Class)
	default:
		var rej *app.CoverRejection
		outcome := "error"
		switch {
		case errors.Is(err, app.ErrImageURLInvalid):
			outcome = "url-invalid"
		case errors.As(err, &rej):
			outcome = "rejected-" + rej.Reason
		}
		h.d.Log.Info("ảnh thân bài: không nhận ảnh từ liên kết", "xa", xa, "actor", actor.ID, "host", host,
			"outcome", outcome)
	}
}

// answerBodyImageFromURLError maps the two refusals only this route has, then the body-image ones.
func (h *Handler) answerBodyImageFromURLError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, app.ErrImageURLInvalid):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_image_url",
			"Liên kết ảnh không hợp lệ: cần là địa chỉ https có tên miền, không kèm cổng khác 443 hay thông tin đăng nhập, và không quá 2048 ký tự.", "")
	case errors.Is(err, app.ErrImageFetchFailed):
		httpx.WriteError(w, http.StatusBadGateway, "image_fetch_failed",
			"Không tải được ảnh từ liên kết này. Hãy kiểm tra liên kết, hoặc tải ảnh về máy rồi tải lên.", "")
	default:
		h.answerBodyImageError(w, r, "tải ảnh thân bài từ liên kết", err)
	}
}

// bodyImageFileOut is the completion reply of a body image: coverFileOut's five fields, plus the preview
// once `ready`. ITS OWN TYPE so the cover's completion contract does not change.
//
// The five fields are WRITTEN OUT, not embedded: tools/apidoc reads named fields, and an embedded struct
// is how a contract silently loses half its shape.
type bodyImageFileOut struct {
	ID string `json:"id"`
	// ContentItemID is the article the image belongs to — the id the server reserved when the request
	// named none. Send it as `content_item_id` on every later image of the same unsaved article.
	ContentItemID string `json:"content_item_id"`
	// MIMEType is the SNIFFED type of the original; "" while `pending`.
	MIMEType string `json:"mime_type"`
	// SizeBytes is the measured size of the original; 0 while `pending`.
	SizeBytes int64 `json:"size_bytes"`
	// Status is ADR 0052 §5's: pending · ready · rejected · failed. Only `ready` may go into a body.
	Status string `json:"status"`
	// PreviewURL is a presigned GET of the DERIVATIVE (1280 px JPEG, no EXIF), valid until
	// PreviewExpiresAt (≤ 15 minutes). Present only when `status` is `ready` and storage is configured —
	// the src the editor gives the figure until the article is saved and re-read.
	PreviewURL       string     `json:"preview_url,omitempty"`
	PreviewExpiresAt *time.Time `json:"preview_expires_at,omitempty"`
}

// bodyImageViews builds the detail's `body_images`. A signing failure is logged and the images still go
// out (without that URL): the article is worth showing without one preview.
func (h *Handler) bodyImageViews(r *http.Request, itemID string, fileIDs []string) []bodyImageOut {
	views, err := h.d.ContentCovers.BodyImageViews(r.Context(), itemID, fileIDs)
	if err != nil {
		h.d.Log.Warn("ảnh thân bài: không dựng được thông tin xem trước",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
	}
	out := make([]bodyImageOut, 0, len(views))
	for _, v := range views {
		o := bodyImageOut{FileID: v.FileID, Status: string(v.Status), Public: v.Public}
		if v.PreviewURL != "" {
			exp := v.PreviewExpiresAt
			o.PreviewURL, o.PreviewExpiresAt = v.PreviewURL.URL(), &exp
		}
		out = append(out, o)
	}
	return out
}

// answerBodyImageError maps the refusals whose sentence or code would otherwise name the COVER, then hands
// the rest to answerCoverError (same pipeline, same statuses).
func (h *Handler) answerBodyImageError(w http.ResponseWriter, r *http.Request, what string, err error) {
	var rej *app.CoverRejection
	switch {
	case errors.As(err, &rej):
		sentence, ok := coverRejectionSentences[rej.Reason]
		if rej.Reason == app.CoverRejectCountReached {
			sentence, ok = "Ảnh bị từ chối: bài đã có đủ số ảnh trong thân bài tối đa được phép.", true
		}
		if !ok {
			sentence = "Ảnh bị từ chối và không được lưu."
		}
		httpx.WriteError(w, http.StatusUnprocessableEntity, "body_image_rejected", sentence, "")
	case errors.Is(err, app.ErrCoverCountReached):
		httpx.WriteError(w, http.StatusConflict, "body_image_limit",
			"Bài đã có đủ số ảnh trong thân bài tối đa được phép. Hãy gỡ bớt ảnh khỏi thân bài, lưu bài, rồi tải ảnh mới.", "")
	case errors.Is(err, app.ErrCoverTypeNotAllowed):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Loại tệp này không được phép làm ảnh trong thân bài.", "")
	case errors.Is(err, app.ErrCoverNotPending):
		httpx.WriteError(w, http.StatusConflict, "body_image_state",
			"Ảnh này đã bị từ chối hoặc lượt tải đã hết hạn. Hãy chọn ảnh và tải lên lại.", "")
	case errors.Is(err, app.ErrCoverUploadNotConfigured):
		h.d.Log.Warn("CẢNH BÁO: từ chối ảnh thân bài vì chưa cấu hình kho lưu tệp / máy quét / giới hạn",
			"xa", string(tenant.MustFrom(r.Context())), "viec", what, "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "storage_not_configured",
			"Chưa cấu hình kho lưu tệp nên chưa dùng được ảnh trong thân bài. Hãy báo quản trị hệ thống.", "")
	default:
		h.answerCoverError(w, r, what, err)
	}
}
