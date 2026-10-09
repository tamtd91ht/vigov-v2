package http

// A citizen's scene photos on their own petition — the HTTP half of internal/app/petition_photo.go:
//
//	POST /api/v1/my-citizen-reports/{maTraCuu}/photos   citizen: ONE multipart upload → scan, re-encode, store
//	GET  /api/v1/my-citizen-reports/{maTraCuu}/photos   citizen: own photos + signed links
//	GET  /api/v1/citizen-reports/{maTraCuu}/photos                        staff, `feedback.read`
//
// THE CITIZEN HANDLERS SIT ON HandlerCongDan AND THE STAFF ONE ON Handler — two muxes, two Deps
// types, never a shared handler (rule 4, invariant 5).
//
// ⚠ THE LIST REPLIES CARRY BEARER CREDENTIALS (presigned GET URLs). They go to the client and nowhere
// else: nothing here logs a reply, a URL, a field of the upload, the lookup code or the citizen id. Every
// such reply carries `Cache-Control: no-store`.

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/ratelimit"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// CitizenPetitionPhotos is the citizen's three acts (app.CitizenPetitionPhotos). It takes the actor and
// no citizen identifier: the actor IS the owner, from the session.
type CitizenPetitionPhotos interface {
	uploadLimiter
	Upload(ctx context.Context, ma string, req app.PhotoUploadRequest, body app.UploadBody, citizen audit.Actor) (
		domain.StoredFile, error)
	ListPhotos(ctx context.Context, ma string, citizen audit.Actor) ([]app.PhotoLink, error)
}

// CitizenVerificationPhotoReader is the citizen's read of the staff verification photos on their own
// petition (app.CitizenVerificationPhotos). The actor IS the owner, from the session.
type CitizenVerificationPhotoReader interface {
	ListPhotos(ctx context.Context, ma string, citizen audit.Actor) ([]app.PhotoLink, error)
}

// StaffPetitionPhotos is the staff read (app.StaffPetitionPhotos).
type StaffPetitionPhotos interface {
	// reader is the staff member, ID = their business code (principal.Ma): the use case audits the read.
	ListPhotos(ctx context.Context, ma string, mayReadRestricted bool, reader audit.Actor) ([]app.PhotoLink, error)
}

// photoOut is one photo as its sender sees it on the upload reply. No object key, no uploader, no file
// name: nothing in them a reader needs, and the uploader is always the petition's own citizen.
type photoOut struct {
	ID string `json:"id"`
	// ContentType is the STORED type — always image/jpeg (the re-encode).
	ContentType string `json:"content_type"`
	// SizeBytes is the stored (re-encoded) size.
	SizeBytes int64 `json:"size_bytes"`
	// Status is ADR 0052 §5's: `stored` on the upload reply.
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

func photoFromFile(f domain.StoredFile) photoOut {
	return photoOut{ID: f.ID, ContentType: f.MIMEType, SizeBytes: f.SizeBytes, Status: string(f.Status),
		CreatedAt: f.CreatedAt}
}

// photoLinkOut is one stored photo with a presigned GET valid until `url_expires_at` (at most 15
// minutes). Open it directly; it is served from the object store's domain, never the commune's.
type photoLinkOut struct {
	ID           string    `json:"id"`
	ContentType  string    `json:"content_type"`
	SizeBytes    int64     `json:"size_bytes"`
	CreatedAt    time.Time `json:"created_at"`
	URL          string    `json:"url"`
	URLExpiresAt time.Time `json:"url_expires_at"`
}

// photoListOut is the reply of both GET …/photos. `items` is never null; at most 5 (no paging).
type photoListOut struct {
	Items []photoLinkOut `json:"items"`
}

func photoListFrom(links []app.PhotoLink) photoListOut {
	out := photoListOut{Items: make([]photoLinkOut, 0, len(links))}
	for _, l := range links {
		out.Items = append(out.Items, photoLinkOut{ID: l.File.ID, ContentType: l.File.MIMEType,
			SizeBytes: l.File.SizeBytes, CreatedAt: l.File.CreatedAt, URL: l.URL.URL(), URLExpiresAt: l.ExpiresAt})
	}
	return out
}

// noStore: a reply carrying a bearer credential is kept by no cache between here and the reader.
func noStore(w http.ResponseWriter) { w.Header().Set("Cache-Control", "no-store") }

// --- the citizen surface ----------------------------------------------------------------------------

// photoGate counts one photo write against the CITIZEN's budget (ratelimit.CitizenPhotoUpload) and
// answers the request when it may not proceed. The key is the session's commune and citizen — never a
// request value. false means the response was written.
//
// A ZALO-ACCOUNT OWNER (ADR 0080) COUNTS IN ITS OWN KEY SPACE: its id is prefixed with its kind before
// the digest, so an account id and a citizen id — two tables, nothing guaranteeing the strings never
// coincide — can never share one budget (the reason core/idem keys by Kind:ID).
func (h *HandlerCongDan) photoGate(w http.ResponseWriter, r *http.Request, citizen audit.Actor) bool {
	subject := citizen.ID
	if citizen.Kind == audit.KindZaloAccount {
		subject = audit.KindZaloAccount + ":" + citizen.ID
	}
	key, err := ratelimit.CitizenKey(r.Context(), subject)
	if err != nil {
		// Unreachable behind CitizenOnly + XaTuPhien, and refused if ever reached: an unscoped or shared
		// counter on the isolation path is the default rule 1 forbids.
		h.d.Log.ErrorContext(r.Context(), "ảnh hiện trường: không dựng được khoá giới hạn tần suất", "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return false
	}
	return ratelimit.Gate(w, r, h.d.PhotoLimiter, key, h.d.Log, "xa", string(tenant.MustFrom(r.Context())))
}

// citizenPhotoActor is congDanThucHien plus the 500 every citizen route answers without it.
func (h *HandlerCongDan) citizenPhotoActor(w http.ResponseWriter, r *http.Request) (audit.Actor, string, bool) {
	citizen, ok := h.congDanThucHien(r)
	if !ok {
		h.d.Log.Error("tuyến ảnh hiện trường chạy mà không có danh tính trong phiên — thiếu " +
			"authz.CitizenOnly hoặc authz.CitizenPrincipal trên chuỗi rìa")
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return audit.Actor{}, "", false
	}
	ma := r.PathValue("maTraCuu")
	if ma == "" {
		h.khongTimThay(w)
		return audit.Actor{}, "", false
	}
	return citizen, ma, true
}

// ownerPhotoActor is citizenPhotoActor for the three SCENE-photo routes, which since ADR 0080
// decision 8 also serve the Zalo account owning an unverified petition (class
// httpx.CommuneFromSessionOrZaloAccount). The owner comes from channelSender — the session, switched
// on its kind — and the actor from app.IntakeSender.Actor, the one place that turns an owner into an
// audit actor. The verification-photo route keeps citizenPhotoActor: it stays verified-phone only.
func (h *HandlerCongDan) ownerPhotoActor(w http.ResponseWriter, r *http.Request) (audit.Actor, string, bool) {
	fail := func() (audit.Actor, string, bool) {
		h.d.Log.Error("tuyến ảnh hiện trường chạy mà không có chủ phiếu trong phiên — thiếu " +
			"authz.CitizenOnly hoặc authz.CitizenPrincipal trên chuỗi rìa")
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return audit.Actor{}, "", false
	}
	sender, ok := h.channelSender(r)
	if !ok {
		return fail()
	}
	actor, err := sender.Actor()
	if err != nil {
		return fail()
	}
	ma := r.PathValue("maTraCuu")
	if ma == "" {
		h.khongTimThay(w)
		return audit.Actor{}, "", false
	}
	return actor, ma, true
}

// UploadPetitionPhoto serves POST /api/v1/my-citizen-reports/{maTraCuu}/photos — ONE multipart upload
// of ONE photo (ADR 0052 §Sửa đổi 09/10/2026), answered with the STORED photo.
//
// ORDER: the owner from the session · the citizen's photo budget, charged ONCE per upload · a slot of
// the pod's upload cap · the policy's cap · the envelope · the use case. Everything before the use case
// refuses without reading the file.
//
// THE PART'S FILE NAME IS IGNORED, never read: a photo is stored under domain.PetitionPhotoName, and the
// name a phone gives a file is often a person's name or number (rule 3).
func (h *HandlerCongDan) UploadPetitionPhoto(w http.ResponseWriter, r *http.Request) {
	citizen, ma, ok := h.ownerPhotoActor(w, r)
	if !ok || !h.photoGate(w, r, citizen) {
		return
	}
	answer := func(err error) { h.answerPhotoError(w, r, "tải ảnh hiện trường", err) }
	up, release, ok := receiveUpload(w, r, h.d.UploadSlots, h.d.Log, h.d.Photos,
		[]string{uploadFieldContentType}, answer)
	defer release()
	if !ok {
		return
	}
	ctx, cancel := context.WithDeadline(r.Context(), up.Deadline)
	defer cancel()
	f, err := h.d.Photos.Upload(ctx, ma, app.PhotoUploadRequest{ContentType: declaredType(up), Size: up.Size},
		uploadBody{&up}, citizen)
	if err != nil {
		answer(err)
		return
	}
	// LOGGED: the commune, the file id and the private key it was stored under — NOT the lookup code,
	// NOT the citizen (rule 3). The store's endpoint and buckets are on the startup line "kho lưu tệp".
	h.d.Log.Info("người dân đã đính ảnh hiện trường", "xa", string(tenant.MustFrom(ctx)), "tep_id", f.ID,
		"khoa_luu", f.ObjectKey)
	// A replay with the same Idempotency-Key is told the FILE ID (core/idem) — never a second file.
	idem.RecordCode(ctx, f.ID)
	vietJSON(w, http.StatusCreated, photoFromFile(f))
}

// ListMyPetitionPhotos serves GET /api/v1/my-citizen-reports/{maTraCuu}/photos.
func (h *HandlerCongDan) ListMyPetitionPhotos(w http.ResponseWriter, r *http.Request) {
	citizen, ma, ok := h.ownerPhotoActor(w, r)
	if !ok {
		return
	}
	links, err := h.d.Photos.ListPhotos(r.Context(), ma, citizen)
	if err != nil {
		h.answerPhotoError(w, r, "đọc ảnh hiện trường", err)
		return
	}
	noStore(w)
	vietJSON(w, http.StatusOK, photoListFrom(links))
}

// ListMyVerificationPhotos serves GET /api/v1/my-citizen-reports/{maTraCuu}/verification-photos — the
// staff "after" photos of the citizen's own petition. Same identity check, same 404 body, same 503.
func (h *HandlerCongDan) ListMyVerificationPhotos(w http.ResponseWriter, r *http.Request) {
	citizen, ma, ok := h.citizenPhotoActor(w, r)
	if !ok {
		return
	}
	links, err := h.d.VerificationPhotos.ListPhotos(r.Context(), ma, citizen)
	if err != nil {
		h.answerPhotoError(w, r, "đọc ảnh sau xử lý", err)
		return
	}
	noStore(w)
	vietJSON(w, http.StatusOK, photoListFrom(links))
}

// photoRejections is the sentence a CITIZEN reads per refusal of the FILE (422).
var photoRejections = map[string]string{
	app.RejectMalware:        "Ảnh bị từ chối vì phát hiện mã độc và không được lưu.",
	app.RejectTypeNotAllowed: "Ảnh bị từ chối: tệp không phải ảnh JPEG, PNG hoặc WebP.",
	app.RejectTypeMismatch:   "Ảnh bị từ chối: nội dung tệp không đúng loại ảnh đã khai báo khi tải lên.",
	app.RejectTooLarge:       "Ảnh bị từ chối: ảnh lớn hơn dung lượng tối đa cho phép.",
	app.RejectCountReached:   "Ảnh bị từ chối: phản ánh đã có đủ số ảnh tối đa.",
	app.RejectUndecodable:    "Ảnh bị từ chối: không đọc được ảnh. Vui lòng chụp hoặc chọn ảnh khác.",
	app.RejectTooManyPixels:  "Ảnh bị từ chối: ảnh có độ phân giải quá lớn để xử lý.",
}

// answerPhotoError maps one failure onto a status and a FIXED sentence — never err.Error(), which
// carries the commune id. The default is 500, never 400: "anything unrecognised is the sender's fault"
// would tell a citizen to fix a photo during an outage. The petition is untouched on every branch.
func (h *HandlerCongDan) answerPhotoError(w http.ResponseWriter, r *http.Request, what string, err error) {
	ctx := r.Context()
	var rej *app.AttachmentRejection
	refused := fileRefusalLog(r, h.d.Log, "ảnh hiện trường: từ chối", what, err)
	switch {
	case writeUploadEnvelopeError(w, err, refused):
	case errors.Is(err, petstore.ErrPhieuKhongTonTai):
		// Identical to an unknown code on GET /api/v1/my-citizen-reports/{code} — rule 4, forbidden #2.
		h.khongTimThay(w)
	case errors.Is(err, app.ErrPhotoNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy ảnh này trên phản ánh.", "")
	case errors.Is(err, app.ErrPhotoTypeNotAllowed):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Chỉ nhận ảnh JPEG, PNG hoặc WebP.", "")
	case errors.Is(err, app.ErrPhotoTooLarge):
		refused("file_too_large")
		httpx.WriteError(w, http.StatusRequestEntityTooLarge, "file_too_large",
			"Ảnh lớn hơn dung lượng tối đa cho phép.", "")
	case errors.Is(err, app.ErrPhotoSizeInvalid):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Kích thước ảnh khai báo không hợp lệ.", "")
	case errors.As(err, &rej):
		sentence, ok := photoRejections[rej.Reason]
		if !ok {
			sentence = "Ảnh bị từ chối và không được lưu."
		}
		refused("photo_rejected")
		httpx.WriteError(w, http.StatusUnprocessableEntity, "photo_rejected", sentence, "")
	case errors.Is(err, app.ErrPhotoWindowClosed):
		// 409, NOT 404: reached only after the read matched the citizen's OWN petition.
		refused("petition_state")
		httpx.WriteError(w, http.StatusConflict, "petition_state",
			"Phản ánh đã được chuyển sang bước xử lý nên không đính thêm ảnh được nữa.", "")
	case errors.Is(err, app.ErrPhotoCountReached):
		refused("photo_limit")
		httpx.WriteError(w, http.StatusConflict, "photo_limit", "Phản ánh đã có đủ số ảnh tối đa.", "")
	case errors.Is(err, app.ErrAttachmentNotPending):
		refused("photo_state")
		httpx.WriteError(w, http.StatusConflict, "photo_state",
			"Ảnh này đã bị từ chối. Vui lòng chọn ảnh và tải lên lại.", "")
	case errors.Is(err, app.ErrUploadChanged):
		refused("upload_changed")
		httpx.WriteError(w, http.StatusConflict, "upload_changed",
			"Ảnh vừa bị thay đổi trong lúc kiểm tra nên CHƯA được lưu. Vui lòng tải lên lại.", "")
	case writePhotoUnavailable(ctx, w, h.d.Log, what, err):
	default:
		h.d.Log.Error("ảnh hiện trường: lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "viec", what, "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}

// fileRefusalLog returns the line every 4xx refusal of a FILE route writes past the permission gate —
// the citizen's scene photo, staff's verification photo, the petition log and task attachments: the
// upload's own failures (408 / 413 / 415 / 400, upload.go) and the 409 / 422 of the flow. The client
// reads a fixed sentence, so without it an operator cannot tell a dropped upload from a refused file
// (09/10/2026, a missing temp bucket answered 409 with nothing in the log). `tep_id` is the path's file
// id where the route has one; an upload's id is inside `err` (app wraps it) once a row was reserved.
//
// INFO and `ma_loi`, as tuChoiXuLy (xu_ly_phan_anh.go): a refusal is the rule doing its job, and `ma`
// in this service is a petition's lookup code. Commune, file id, code and the wrapped error only: the
// error names object keys and the commune, never a lookup code, a person or a file name (rule 3).
func fileRefusalLog(r *http.Request, log *slog.Logger, msg, what string, err error) func(code string) {
	ctx := r.Context()
	return func(code string) {
		log.InfoContext(ctx, msg, "xa", string(tenant.MustFrom(ctx)), "viec", what,
			"tep_id", r.PathValue("id"), "ma_loi", code, "err", err)
	}
}

// writePhotoUnavailable answers the three "not now" causes with 503, for both surfaces, and reports
// whether it did. Nothing was stored in any of them (ADR 0052 §9, §10).
func writePhotoUnavailable(ctx context.Context, w http.ResponseWriter, log *slog.Logger, what string, err error) bool {
	switch {
	case errors.Is(err, app.ErrUploadNotConfigured):
		log.Warn("CẢNH BÁO: từ chối ảnh hiện trường vì chưa cấu hình kho lưu tệp / máy quét / giới hạn",
			"xa", string(tenant.MustFrom(ctx)), "viec", what, "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "storage_not_configured",
			"Hệ thống chưa sẵn sàng xử lý ảnh. Phản ánh vẫn được ghi nhận bình thường.", "")
	case errors.Is(err, app.ErrUploadLimitsUnavailable):
		log.Warn("CẢNH BÁO: từ chối ảnh hiện trường vì chưa đọc được giới hạn tải tệp từ platform",
			"xa", string(tenant.MustFrom(ctx)), "viec", what, "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "upload_limits_unavailable",
			"Tạm thời chưa nhận được ảnh. Vui lòng thử lại sau ít phút.", "")
	case errors.Is(err, app.ErrScanUnavailable):
		log.Warn("CẢNH BÁO: chưa quét được mã độc, ảnh hiện trường giữ ở trạng thái chờ",
			"xa", string(tenant.MustFrom(ctx)), "viec", what, "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "malware_scan_unavailable",
			"Tạm thời chưa kiểm tra được ảnh nên ảnh CHƯA được lưu. Vui lòng thử lại sau ít phút.", "")
	default:
		return false
	}
	return true
}

// --- the staff surface ------------------------------------------------------------------------------

// ListPetitionPhotos serves GET /api/v1/citizen-reports/{maTraCuu}/photos — "TRƯỚC KHI XỬ LÝ" on the
// petition detail (docs/ui-ux/09 §8.4). `feedback.read` at the gate; `feedback.restricted` decides a
// `can-bo` petition, answered 404 without it exactly as DocPhieuPhanAnh answers.
func (h *Handler) ListPetitionPhotos(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ma := r.PathValue("maTraCuu")
	if ma == "" {
		h.khongTimThay(w)
		return
	}
	principal, ok := authz.From(ctx)
	mayRestricted := ok && h.d.Checker.Allows(ctx, principal, QuyenHanChe)
	// The trail's "who" is `principal.Ma`, NEVER `principal.ID` (rule 6, invariant 8), and there is no
	// fallback: an empty code reaches the use case empty and the read is refused there. The IP is this
	// socket's (httpx.ClientIP does not trust X-Forwarded-For), as on DocPhieuPhanAnh's unmask trail.
	reader := audit.Actor{Kind: principal.Kind, IP: httpx.ClientIP(r)}
	if ok {
		reader.ID = principal.Ma
	}
	links, err := h.d.PetitionPhotos.ListPhotos(ctx, ma, mayRestricted, reader)
	switch {
	case err == nil:
		noStore(w)
		vietJSON(w, http.StatusOK, photoListFrom(links))
	case errors.Is(err, petstore.ErrPhieuKhongTonTai):
		h.khongTimThay(w)
	case writePhotoUnavailable(ctx, w, h.d.Log, "đọc ảnh hiện trường (cán bộ)", err):
	default:
		h.d.Log.Error("đọc ảnh hiện trường: lỗi hệ thống", "xa", string(tenant.MustFrom(ctx)), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}
