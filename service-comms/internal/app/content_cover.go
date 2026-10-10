package app

// THE COVER IMAGE OF A MINI APP ARTICLE (docs/ui-ux/11-noi-dung-mini-app.md §7 `Ảnh đại diện`,
// ADR 0047 §6 (1), ADR 0052 §1, §11) — ONE upload request through this service (ADR 0052 §Sửa đổi
// 09/10/2026: devices no longer POST straight to MinIO), in three acts inside UploadCover:
//
//	a. admitUpload  the declaration against the policy · the article · the count → pending row + trail (tx 1)
//	b. PutUpload    the multipart file part streamed into the temp bucket under the row's `upload/…` key
//	c. complete     sniff · scan · hash · copy · derivative → ready + trail (tx 2) — ADR 0052 §1c, unchanged
//
// A failure after (a) never leaves the row `pending` with nothing that could finish it (the old
// `…/completion` routes are gone): abandon moves it to `failed` with its own trail entry.
//
// and then the file rides on the article: `cover_image_file_id` on POST / PATCH /api/v1/content-items
// (SoanNoiDungMiniApp.Them / Sua), checked and linked in THAT write's transaction. Publishing the article
// publishes the derivative (coverPublisher, below).
//
// THE SHAPE IS service-petitions/internal/app/task_attachment.go's (b37ec2d5), on purpose: one upload
// discipline for every service that stores files. What differs:
//
//   - the subject may not exist yet. §7's modal uploads BEFORE `Lưu` (migration 0011), so an upload
//     issued without `content_item_id` mints the article id here; the create then takes THAT id from the
//     file row, never from the client.
//   - the class is content-source and the bucket private; nothing here can write a records object.
//   - completion also produces the `thumb-1280` JPEG derivative (cover_image.go): an upload is `ready`,
//     and so usable as a cover, only once a publishable derivative exists.
//
// THE BODY IMAGES (ADR 0067 §Sửa đổi 03/10/2026, K2/K3) RUN THROUGH THE SAME ACTS under their own purpose,
// `content-body-image` — POST …/body-images (UploadBodyImage) — and ride on the article
// as `<img data-file-id>` inside the body, checked on every save (coverPublisher.checkBodyImages). The
// publish step decides per purpose (settle), so publishing or replacing a cover never withdraws an image
// the body still shows.
//
// THE LIMITS ARE PLATFORM'S, NEVER THIS FILE'S (ADR 0052 §10): size and types come from the
// `content-image` (or `content-body-image`) policy before the body is read (the handler's cap,
// CoverUploadLimit), at admission and again at completion. Not configured → refusal; platform
// unreachable → 503. The 50 MB / JPG·PNG·WebP of 01/10/2026 lives in platform's seed, not here.
//
// WHY THE OBJECT-STORE WORK HAPPENS OUTSIDE ANY TRANSACTION: streaming, scanning, hashing and decoding are
// network I/O and CPU of unbounded length; holding a row lock across them stalls every other write. The
// pending row is committed first, the bytes are written and inspected lock-free, then the outcome is
// written in ONE short transaction that re-reads the row FOR UPDATE.
//
// WHAT IS NEVER LOGGED OR PUT IN AN ERROR: the original file name (rule 3), any presigned URL, file
// content. The trail carries the file id, the sniffed type, the size, the hash and the reason for a
// refusal — never the name.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/malwarescan"
	"github.com/vihat/vigov/core/platformclient/uploadpolicy"
	"github.com/vihat/vigov/core/storage"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	"github.com/vihat/vigov/service-comms/internal/richtext"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// The verbs in the trail — Vietnamese snake_case like every other value this service writes (ADR 0011).
const (
	ActionCoverUploadRequested = "yeu_cau_tai_anh_bia_noi_dung"
	ActionCoverStored          = "luu_anh_bia_noi_dung"
	ActionCoverRejected        = "tu_choi_anh_bia_noi_dung"
	ActionCoverExpired         = "anh_bia_noi_dung_het_han_tai"
	ActionCoverWithdrawn       = "go_anh_bia_cong_khai"

	// The body images of an article (ADR 0067 §Sửa đổi 03/10/2026, K3): the SAME upload acts under their
	// own purpose, so their own verbs — an inspection reading "ảnh bìa" for an image in the body would be
	// reading the wrong record.
	ActionBodyImageUploadRequested = "yeu_cau_tai_anh_than_bai_noi_dung"
	ActionBodyImageStored          = "luu_anh_than_bai_noi_dung"
	ActionBodyImageRejected        = "tu_choi_anh_than_bai_noi_dung"
	ActionBodyImageExpired         = "anh_than_bai_noi_dung_het_han_tai"
	ActionBodyImageWithdrawn       = "go_anh_than_bai_cong_khai"
)

// uploadVerbs are the trail's verbs and subject prefix for one upload purpose.
//
// `abandoned` is the …_het_han_tai verb: before 09/10/2026 it recorded a presigned form that expired with
// nothing uploaded; now it records an upload that ENDED WITH NOTHING STORED — the stream or the write
// failed, or the inspection could not run — and `ly_do` says which (CoverAbandon*). The same state
// change (`pending → failed`, no file), so the same verb rather than a new one no screen has a label for.
type uploadVerbs struct {
	requested, stored, rejected, abandoned, withdrawn string
	subject                                           string // + the UTC date, see coverAuditSubject
}

var (
	coverVerbs = uploadVerbs{ActionCoverUploadRequested, ActionCoverStored, ActionCoverRejected,
		ActionCoverExpired, ActionCoverWithdrawn, "noi-dung-mini-app/anh-bia/"}
	bodyImageVerbs = uploadVerbs{ActionBodyImageUploadRequested, ActionBodyImageStored,
		ActionBodyImageRejected, ActionBodyImageExpired, ActionBodyImageWithdrawn, "noi-dung-mini-app/anh-than-bai/"}
)

// verbsFor picks the verbs by the FILE's purpose — the row says what it is, never the route it came by.
func verbsFor(purpose string) uploadVerbs {
	if purpose == string(bodyImagePurpose) {
		return bodyImageVerbs
	}
	return coverVerbs
}

// Rejection reasons — the `ly_do` of a rejected entry and the key the handler picks its sentence by.
const (
	CoverRejectMalware        = "nhiem-ma-doc"
	CoverRejectTypeNotAllowed = "sai-kieu-tep"
	CoverRejectTypeMismatch   = "khac-kieu-khai-bao"
	CoverRejectTooLarge       = "vuot-dung-luong"
	CoverRejectCountReached   = "vuot-so-tep"
	CoverRejectUndecodable    = "khong-doc-duoc-anh"
	CoverRejectTooManyPixels  = "qua-nhieu-diem-anh"
)

// Why an upload ended with nothing stored — the `ly_do` of an abandoned entry (uploadVerbs.abandoned).
// Shared by the audio upload: one vocabulary in the trail.
const (
	// CoverAbandonNotReceived: the file part did not arrive whole (client hung up, too slow, malformed,
	// longer or shorter than declared) or the temp bucket refused the write.
	CoverAbandonNotReceived = "khong-nhan-du-tep"
	// CoverAbandonNotInspected: the bytes arrived but the completion could not decide (scanner down,
	// object store unreachable, a refusal at attach time). Nothing was stored as a usable file.
	CoverAbandonNotInspected = "khong-kiem-tra-duoc"
)

var (
	// ErrCoverUploadNotConfigured: object storage, the scanner, or platform's limit is absent. 503.
	ErrCoverUploadNotConfigured = errors.New("ảnh bìa: chưa cấu hình kho lưu tệp")
	// ErrCoverLimitsUnavailable: platform could not be asked and no fresh answer is cached. 503.
	ErrCoverLimitsUnavailable = errors.New("ảnh bìa: chưa đọc được giới hạn tải tệp")
	// ErrCoverScanUnavailable: clamd could not scan (unreachable, timeout, over its size limit, unknown
	// reply). The row stays `pending`; the file is NEVER stored unscanned (ADR 0052 §9). 503.
	ErrCoverScanUnavailable = errors.New("ảnh bìa: chưa quét được mã độc")
	// ErrCoverTypeNotAllowed / ErrCoverTooLarge: the DECLARATION is outside the policy. 400, no row.
	ErrCoverTypeNotAllowed = errors.New("ảnh bìa: loại tệp không được phép")
	ErrCoverTooLarge       = errors.New("ảnh bìa: tệp vượt dung lượng cho phép")
	// ErrCoverCountReached: the article already holds platform's `max_files_per_subject`. 409.
	ErrCoverCountReached = errors.New("ảnh bìa: mục nội dung đã đủ số tệp tối đa")
	// ErrCoverFileNotFound: no such upload FOR THIS CALLER — unknown, another commune's, another
	// officer's. One answer for all (rule 4, forbidden #2). 404.
	ErrCoverFileNotFound = errors.New("ảnh bìa: không tìm thấy")
	// ErrCoverNotPending: the row moved past `pending` under the completion (a concurrent writer). 409.
	ErrCoverNotPending = errors.New("ảnh bìa: tệp không còn chờ hoàn tất")
	// ErrCoverUploadIncomplete: the file part was not written whole into the temp bucket. It always wraps
	// the cause: an httpx upload sentinel (the client's stream — 4xx by httpx.UploadErrorStatus),
	// storage.ErrTooLarge / ErrSizeMismatch (4xx), or a store failure (503). The row is `failed`.
	ErrCoverUploadIncomplete = errors.New("ảnh bìa: chưa nhận đủ tệp")
	// ErrCoverUploadChanged: the temp object changed between two reads of the inspection. 409.
	ErrCoverUploadChanged = errors.New("ảnh bìa: tệp vừa bị thay đổi trong lúc kiểm tra")
	// ErrCoverPublishUnavailable: the object store refused or could not be reached while publishing or
	// withdrawing the derivative. NOTHING was written; the article is as it was. 503.
	ErrCoverPublishUnavailable = errors.New("ảnh bìa: chưa đăng được ảnh lên kho công khai")
	// ErrCoverRejected is what every *CoverRejection matches.
	ErrCoverRejected = errors.New("ảnh bìa: tệp bị từ chối")
)

// CoverRejection is a completion that refused the FILE: the row is `rejected` (or `failed` when the
// image could not be re-encoded), the temp object deleted or expiring within a day, the trail says why.
type CoverRejection struct{ Reason string }

func (e *CoverRejection) Error() string   { return ErrCoverRejected.Error() + ": " + e.Reason }
func (e *CoverRejection) Is(t error) bool { return t == ErrCoverRejected }

// CoverObjectStore is the part of *storage.Client the cover acts call — an interface so the tests run
// with no MinIO. The semantics are core/storage's; read storage.go before implementing another.
type CoverObjectStore interface {
	PutUpload(ctx context.Context, uploadKey string, r io.Reader, size int64, maxBytes int64,
		contentType string) (storage.ObjectInfo, error)
	Stat(ctx context.Context, b storage.Bucket, key string) (storage.ObjectInfo, error)
	ReadHead(ctx context.Context, b storage.Bucket, key, ifMatchETag string, n int) ([]byte, error)
	Open(ctx context.Context, b storage.Bucket, key, ifMatchETag string) (io.ReadCloser, int64, error)
	SHA256(ctx context.Context, b storage.Bucket, key, ifMatchETag string) (string, error)
	Promote(ctx context.Context, srcUploadKey, ifMatchETag string, dst storage.Key,
		b storage.Bucket) (storage.Promoted, error)
	PutServerProduced(ctx context.Context, dst storage.Key, r io.Reader, size int64) (storage.Produced, error)
	PresignDownload(ctx context.Context, b storage.Bucket, key string, ttl time.Duration,
		filename string) (storage.PresignedURL, error)
	PurgeAllVersions(ctx context.Context, b storage.Bucket, key string) error
	PublishDerivative(ctx context.Context, src, dst storage.Key) error
	UnpublishDerivative(ctx context.Context, dst storage.Key) error
	PublicURL(key string) (string, error)
}

// MalwareScanner is *malwarescan.Scanner.
type MalwareScanner interface {
	Scan(ctx context.Context, r io.Reader, size int64) (malwarescan.Result, error)
}

// UploadPolicies is *uploadpolicy.Reader.
type UploadPolicies interface {
	Policy(ctx context.Context, purpose storage.Purpose) (uploadpolicy.Policy, bool, error)
}

// CoverFiles is the part of *store.StoredFileStore the cover and body-image acts call.
type CoverFiles interface {
	// The body images (ADR 0067 §Sửa đổi 03/10/2026): one batched read per article, the reservation of an
	// unsaved article's id, and the retire of an image taken out of the body.
	LiveForSubject(ctx context.Context, subjectID, purpose string) ([]domain.StoredFile, error)
	SubjectReservedBy(ctx context.Context, tx *store.ScopedTx, subjectID, uploadedBy string) (bool, error)
	SoftDelete(ctx context.Context, tx *store.ScopedTx, id, by, reason string, at time.Time) error

	InsertPending(ctx context.Context, tx *store.ScopedTx, f domain.StoredFile) error
	ForUpdate(ctx context.Context, tx *store.ScopedTx, id string) (*domain.StoredFile, error)
	ByID(ctx context.Context, id string) (*domain.StoredFile, error)
	// LiveByIDs is ByID for a page: one statement, absent = no live row in this commune.
	LiveByIDs(ctx context.Context, ids []string) (map[string]domain.StoredFile, error)
	Transition(ctx context.Context, tx *store.ScopedTx, id string, from, to domain.StoredFileStatus,
		at time.Time) error
	MarkStored(ctx context.Context, tx *store.ScopedTx, id string, facts domain.StoredFileFacts,
		at time.Time) error
	SetPublicObjectKey(ctx context.Context, tx *store.ScopedTx, id, key string, at time.Time) error
	PublicForSubject(ctx context.Context, tx *store.ScopedTx, subjectID string) ([]domain.StoredFile, error)
	PublicObjectKeys(ctx context.Context, ids []string) (map[string]string, error)
	LockSubjectCount(ctx context.Context, tx *store.ScopedTx, subjectID, purpose string) error
	CountForSubjectTx(ctx context.Context, tx *store.ScopedTx, subjectID, purpose string,
		pendingSince time.Time) (int, error)
	CountForSubject(ctx context.Context, subjectID, purpose string) (int, error)
}

// coverPurpose is the one purpose a cover is uploaded under (core/storage, platform's policy).
const coverPurpose = storage.PurposeContentImage

// bodyImagePurpose is the one purpose an image INSIDE the body is uploaded under (ADR 0067 §Sửa đổi
// 03/10/2026, K3; platform's `content-body-image` policy: types, size and the per-article count H7).
//
// A PURPOSE OF ITS OWN, NOT content-image: the cover's publish step withdraws every other public file of
// the article, and the per-article count is a body-image limit the cover must not share. Same pipeline —
// sniff, ClamAV, EXIF-free `thumb-1280` derivative, publish/withdraw with the article — no copy of it.
const bodyImagePurpose = storage.PurposeContentBodyImage

// bodyImageRetireReason is the `delete_reason` of a body image an edit took out of the body (rule 7: the
// row and the object stay; only the per-article slot of H7 is freed).
const bodyImageRetireReason = "gỡ ảnh khỏi thân bài (sửa mục nội dung)"

// ContentCovers owns the upload acts, the staff preview and the public URLs.
type ContentCovers struct {
	db    *store.DB
	items KhoNoiDungMiniApp
	files CoverFiles

	// The three dependencies ADR 0052 adds. ANY nil means "not configured" and every upload is refused
	// with ErrCoverUploadNotConfigured — fail closed, while the rest of the service keeps serving.
	objects  CoverObjectStore
	scanner  MalwareScanner
	policies UploadPolicies

	// decodeSlot admits ONE decode at a time in this process: coverDecodeBudget is a per-decode bound,
	// and two at once would be two budgets against one pod memory limit.
	decodeSlot chan struct{}

	// fetcher downloads a body image from a pasted link (content_body_image_url.go); nil = not configured.
	// fetchSlots bounds the downloads in flight (bodyImageFetchSlots).
	fetcher    ImageFetcher
	fetchSlots chan struct{}

	newID func() (string, error)
	now   func() time.Time
}

// NewContentCovers builds the use case. Pass UNTYPED nil for a dependency that is not configured — a
// nil *storage.Client inside a non-nil interface would pass the nil check and panic on first use.
func NewContentCovers(db *store.DB, items KhoNoiDungMiniApp, files CoverFiles, objects CoverObjectStore,
	scanner MalwareScanner, policies UploadPolicies) *ContentCovers {
	return &ContentCovers{db: db, items: items, files: files, objects: objects, scanner: scanner,
		policies: policies, decodeSlot: make(chan struct{}, 1), newID: storage.NewObjectID}
}

func (uc *ContentCovers) clock() time.Time {
	if uc.now == nil {
		return time.Now().UTC()
	}
	return uc.now().UTC()
}

func (uc *ContentCovers) uploadsConfigured() bool {
	return uc.objects != nil && uc.scanner != nil && uc.policies != nil
}

func (uc *ContentCovers) policy(ctx context.Context) (uploadpolicy.Policy, error) {
	return uc.policyFor(ctx, coverPurpose)
}

// policyFor reads platform's limits for one purpose — size, types and `max_files_per_subject` (for body
// images the 20 of H7). NEVER a Go constant: the owner changes them by configuration (ADR 0052 §10).
func (uc *ContentCovers) policyFor(ctx context.Context, purpose storage.Purpose) (uploadpolicy.Policy, error) {
	p, ok, err := uc.policies.Policy(ctx, purpose)
	switch {
	case errors.Is(err, uploadpolicy.ErrUnavailable):
		return uploadpolicy.Policy{}, fmt.Errorf("%w: %w", ErrCoverLimitsUnavailable, err)
	case err != nil:
		return uploadpolicy.Policy{}, fmt.Errorf("ảnh bìa: đọc giới hạn tải tệp: %w", err)
	case !ok:
		return uploadpolicy.Policy{}, fmt.Errorf("%w: platform has no limit for %s",
			ErrCoverUploadNotConfigured, purpose)
	}
	return p, nil
}

// --- the upload: admit, receive, complete -----------------------------------------------------------

// CoverUploadRequest is one upload as the handler received it: the declaration (checked here against the
// policy) and the file stream, whose bytes the completion checks again — a claim is never the fact
// (ADR 0052 §1c).
type CoverUploadRequest struct {
	// ContentItemID is "" for an article not saved yet (the id is minted here), or the id of an existing
	// LIVE article of this commune whose cover is being replaced.
	ContentItemID string
	FileName      string
	ContentType   string // DECLARED; the key's extension follows it, completion sniffs the truth
	Size          int64  // DECLARED; File must yield exactly this many bytes

	// File streams the file part (httpx.UploadRequest.File). Never buffered here: PutUpload pipes it.
	File io.Reader
	// Finish confirms nothing followed the file (httpx.UploadRequest.Finish). Called after the write and
	// BEFORE anything is recorded as received.
	Finish func() error
	// Deadline bounds the write (httpx.UploadRequest.Deadline); zero = the caller's context only.
	Deadline time.Time
}

// CoverUploadLimit is the `content-image` policy's byte cap — what the handler caps the request body at
// BEFORE reading it (httpx.UploadOptions.MaxFileBytes). Never a constant here (ADR 0052 §10).
func (uc *ContentCovers) CoverUploadLimit(ctx context.Context) (int64, error) {
	return uc.uploadLimit(ctx, coverPurpose)
}

// BodyImageUploadLimit is CoverUploadLimit for `content-body-image`.
func (uc *ContentCovers) BodyImageUploadLimit(ctx context.Context) (int64, error) {
	return uc.uploadLimit(ctx, bodyImagePurpose)
}

func (uc *ContentCovers) uploadLimit(ctx context.Context, purpose storage.Purpose) (int64, error) {
	if !uc.uploadsConfigured() {
		return 0, ErrCoverUploadNotConfigured
	}
	pol, err := uc.policyFor(ctx, purpose)
	if err != nil {
		return 0, err
	}
	if pol.MaxBytes <= 0 {
		// A policy with no cap is no policy: refused, never read as "unbounded".
		return 0, fmt.Errorf("%w: %s has no byte cap", ErrCoverUploadNotConfigured, purpose)
	}
	return pol.MaxBytes, nil
}

// UploadCover receives one cover image and returns it stored: `ready` with its derivative, or a refusal.
// The three acts of the file's header comment; see upload for what each failure leaves behind.
func (uc *ContentCovers) UploadCover(ctx context.Context, req CoverUploadRequest, actor audit.Actor) (
	domain.StoredFile, error) {
	return uc.upload(ctx, coverPurpose, req, actor)
}

// UploadBodyImage is UploadCover for an image INSIDE the body (ADR 0067 §Sửa đổi 03/10/2026, K3): same acts,
// purpose `content-body-image`, its own policy — and so its own `max_files_per_subject` (H7), counted
// against the article's live body images in the same transaction as the pending row.
func (uc *ContentCovers) UploadBodyImage(ctx context.Context, req CoverUploadRequest, actor audit.Actor) (
	domain.StoredFile, error) {
	return uc.upload(ctx, bodyImagePurpose, req, actor)
}

// upload is the one implementation behind both. What each outcome leaves:
//
//	refused at admission       nothing — no row, no trail, the body's file part never read
//	stream / write failed      row `failed` + abandoned entry (khong-nhan-du-tep); ErrCoverUploadIncomplete
//	file refused (*Rejection)  row `rejected` (or `failed`: undecodable) + rejected entry — complete's own
//	could not inspect          row `failed` + abandoned entry (khong-kiem-tra-duoc); the cause (scanner down…)
//	stored                     row `ready` + stored entry
func (uc *ContentCovers) upload(ctx context.Context, purpose storage.Purpose, req CoverUploadRequest,
	actor audit.Actor) (domain.StoredFile, error) {

	if req.File == nil || req.Finish == nil {
		return domain.StoredFile{}, errors.New("ảnh bìa: lượt tải lên thiếu luồng tệp — lỗi nối dây ở tầng HTTP")
	}
	f, uploadKey, pol, err := uc.admitUpload(ctx, purpose, req, actor)
	if err != nil {
		return domain.StoredFile{}, err
	}

	wctx, cancel := ctx, context.CancelFunc(func() {})
	if !req.Deadline.IsZero() {
		wctx, cancel = context.WithDeadline(ctx, req.Deadline)
	}
	_, err = uc.objects.PutUpload(wctx, uploadKey, req.File, req.Size, pol.MaxBytes, req.ContentType)
	cancel()
	if err == nil {
		// Trailing bytes after the declared size: the object holds exactly Size bytes, but the request
		// was not what it said — refused like any malformed upload. Its temp object is never read again
		// and expires with the bucket's 1-day lifecycle.
		err = req.Finish()
	}
	if err != nil {
		return domain.StoredFile{}, uc.abandon(ctx, purpose, f.ID, actor, CoverAbandonNotReceived,
			fmt.Errorf("%w: %w", ErrCoverUploadIncomplete, err))
	}

	done, err := uc.complete(ctx, purpose, f.ID, actor)
	if err != nil && !errors.Is(err, ErrCoverRejected) {
		return domain.StoredFile{}, uc.abandon(ctx, purpose, f.ID, actor, CoverAbandonNotInspected, err)
	}
	return done, err
}

// abandon moves a row this upload left `pending` to `failed`, with the abandoned trail entry, in ONE
// transaction (rule 6, invariant 3), and returns cause. WITHOUT the request's cancellation: a client that
// hung up is the commonest reason to be here, and its cancelled context must not leave the row pending.
//
// WHY NOT LEAVE IT `pending`: nothing can finish it any more — the completion route is gone (ADR 0052
// §Sửa đổi 09/10/2026) — and a pending row holds one of the article's slots for UploadTTL (audio has one).
// A row no longer pending (another writer moved it) is left alone. The temp object, if any, expires with
// the bucket's 1-day lifecycle; one the inspection already promoted stays as an unreferenced original.
func (uc *ContentCovers) abandon(ctx context.Context, purpose storage.Purpose, id string, actor audit.Actor,
	reason string, cause error) error {

	ctx = context.WithoutCancel(ctx)
	now := uc.clock()
	verbs := verbsFor(string(purpose))
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		cur, err := uc.files.ForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if cur == nil || cur.Status != domain.StoredFilePending {
			return nil
		}
		if err := uc.files.Transition(ctx, tx, id, domain.StoredFilePending, domain.StoredFileFailed, now); err != nil {
			return err
		}
		return writeCoverAudit(ctx, tx, actor, verbs, verbs.abandoned, now, map[string]any{
			"tep_id": id, "muc_noi_dung_id": cur.SubjectID, "ly_do": reason,
		})
	})
	if err != nil {
		return fmt.Errorf("%w; ảnh bìa: dòng tệp %s chưa chuyển được sang failed: %w", cause, id, err)
	}
	return cause
}

// admitUpload is act (a). ContentItemID may name:
//
//	""                          a new article: its id is minted here (File.SubjectID tells the client);
//	a live article of this commune
//	an unsaved article's id     ONLY one THIS OFFICER already holds an upload on (SubjectReservedBy) — how a
//	                            second body image, or the cover after a body image, joins the article the
//	                            first upload reserved. Anybody else's minted id answers 404, like an unknown
//	                            one (rule 4, forbidden #2 on the commune axis).
//
// ONE TRANSACTION: the article row FOR UPDATE when one is named, the per-subject count lock and the count
// (admitSubject), the pending row, the audit entry. Returns the row, its temp `upload/…` key and the policy.
func (uc *ContentCovers) admitUpload(ctx context.Context, purpose storage.Purpose, req CoverUploadRequest,
	actor audit.Actor) (domain.StoredFile, string, uploadpolicy.Policy, error) {

	var none uploadpolicy.Policy
	if actor.ID == "" {
		return domain.StoredFile{}, "", none, ErrThieuNguoiTaoNoiDung
	}
	name, err := domain.CleanCoverFileName(req.FileName)
	if err != nil {
		return domain.StoredFile{}, "", none, err
	}
	if req.Size <= 0 {
		return domain.StoredFile{}, "", none, domain.ErrCoverSizeInvalid
	}
	if len(req.ContentItemID) > domain.MaxFileIDLen {
		return domain.StoredFile{}, "", none, commsstore.ErrNoiDungKhongTonTai
	}
	if !uc.uploadsConfigured() {
		return domain.StoredFile{}, "", none, ErrCoverUploadNotConfigured
	}
	pol, err := uc.policyFor(ctx, purpose)
	if err != nil {
		return domain.StoredFile{}, "", none, err
	}
	if !pol.AllowsMIME(req.ContentType) {
		return domain.StoredFile{}, "", none, ErrCoverTypeNotAllowed
	}
	if req.Size > pol.MaxBytes {
		return domain.StoredFile{}, "", none, ErrCoverTooLarge
	}
	ext, ok := storage.ExtForMIME(req.ContentType)
	if !ok {
		return domain.StoredFile{}, "", none, ErrCoverTypeNotAllowed // unreachable while uploadpolicy narrows to storage's list
	}

	id, err := uc.newID()
	if err != nil {
		return domain.StoredFile{}, "", none, fmt.Errorf("ảnh bìa: sinh mã tệp: %w", err)
	}
	subject := req.ContentItemID
	if subject == "" {
		// THE FUTURE ARTICLE'S ID, minted by the server. The create takes it from this row.
		if subject, err = uc.newID(); err != nil {
			return domain.StoredFile{}, "", none, fmt.Errorf("ảnh bìa: sinh mã mục nội dung: %w", err)
		}
	}
	now := uc.clock()
	key := storage.Key{
		Class: storage.ClassContentSource, TenantID: string(tenant.MustFrom(ctx)), CreatedAt: now,
		Service: storage.ServiceComms, Purpose: purpose,
		ObjectID: id, Variant: storage.VariantOriginal, Ext: ext,
	}
	objectKey, err := key.Path()
	if err != nil {
		return domain.StoredFile{}, "", none, fmt.Errorf("ảnh bìa: dựng khoá đối tượng: %w", err)
	}
	uploadKey, err := key.UploadPath()
	if err != nil {
		return domain.StoredFile{}, "", none, fmt.Errorf("ảnh bìa: dựng khoá tải lên: %w", err)
	}

	verbs := verbsFor(string(purpose))
	f := domain.StoredFile{
		ID: id, Bucket: domain.StoredFileBucketPrivate, ObjectKey: objectKey,
		RetentionClass: string(storage.ClassContentSource), Purpose: string(purpose),
		SubjectType: domain.StoredFileSubjectContentItem, SubjectID: subject, OriginalName: name,
		Status: domain.StoredFilePending, UploadedBy: actor.ID, CreatedAt: now, UpdatedAt: now,
	}
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		if err := uc.admitSubject(ctx, tx, purpose, pol, req.ContentItemID, subject, actor, now); err != nil {
			return err
		}
		if err := uc.files.InsertPending(ctx, tx, f); err != nil {
			return err
		}
		return writeCoverAudit(ctx, tx, actor, verbs, verbs.requested, now, map[string]any{
			"tep_id":          id,
			"muc_noi_dung_id": subject,
			"muc_dich":        string(purpose),
			"loai_khai_bao":   req.ContentType,
			"kich_thuoc_khai": req.Size,
		})
	})
	if err != nil {
		return domain.StoredFile{}, "", none, bocNoiDung(ctx, "nhận ảnh bìa", err)
	}
	return f, uploadKey, pol, nil
}

// admitSubject is the in-transaction check of the article an upload names (named; "" = a fresh
// reservation) and of the purpose's per-article count on subject. Shared by admitUpload and the body
// image fetched from a link (FetchBodyImage), so the two cannot drift on who may add a file to what.
//
// THE COUNT IS TAKEN UNDER LockSubjectCount, ON EVERY PATH — article row or not. The row lock above
// serialises only an article that exists; a reserved id has no row, and without the subject lock two
// requests at max-1 both count max-1 and both insert. The caller inserts the row in THIS transaction.
func (uc *ContentCovers) admitSubject(ctx context.Context, tx *store.ScopedTx, purpose storage.Purpose,
	pol uploadpolicy.Policy, named, subject string, actor audit.Actor, now time.Time) error {

	if named != "" {
		if _, err := uc.items.TheoIDDeSua(ctx, tx, named); err != nil {
			if !errors.Is(err, commsstore.ErrNoiDungKhongTonTai) {
				return err
			}
			// No article yet: an id this officer's earlier upload reserved, or nothing.
			reserved, rerr := uc.files.SubjectReservedBy(ctx, tx, named, actor.ID)
			if rerr != nil {
				return rerr
			}
			if !reserved {
				return err
			}
		}
	}
	if pol.FileCountLimited {
		if err := uc.files.LockSubjectCount(ctx, tx, subject, string(purpose)); err != nil {
			return err
		}
		live, err := uc.files.CountForSubjectTx(ctx, tx, subject, string(purpose), now.Add(-storage.UploadTTL))
		if err != nil {
			return err
		}
		if live >= pol.MaxFilesPerSubject {
			return ErrCoverCountReached
		}
	}
	return nil
}

// --- c. complete the upload -------------------------------------------------------------------------

type coverOutcome int

const (
	coverReady coverOutcome = iota + 1
	coverRejected
	coverFailed // stored, but no derivative can be made: `processing → failed`
)

type coverInspection struct {
	kind        coverOutcome
	facts       domain.StoredFileFacts
	recovered   bool
	reason      string
	signature   string
	tempRemoved bool
}

// complete is ADR 0052 §1c plus the derivative, for the upload upload() just wrote: the cover's and the body
// image's alike — the same sniff, scan, hash, promote and `thumb-1280` EXIF-free derivative. It re-reads the
// row and checks it is THIS officer's upload of THIS purpose, as when it was a route of its own.
//
// A file already `ready` is returned as it is (another writer finished first: the same outcome).
func (uc *ContentCovers) complete(ctx context.Context, purpose storage.Purpose, id string, actor audit.Actor) (
	domain.StoredFile, error) {
	if actor.ID == "" {
		return domain.StoredFile{}, ErrThieuNguoiTaoNoiDung
	}
	if id == "" || len(id) > domain.MaxFileIDLen {
		return domain.StoredFile{}, ErrCoverFileNotFound
	}
	if !uc.uploadsConfigured() {
		return domain.StoredFile{}, ErrCoverUploadNotConfigured
	}

	f, err := uc.files.ByID(ctx, id)
	if err != nil {
		return domain.StoredFile{}, bocNoiDung(ctx, "hoàn tất ảnh bìa", err)
	}
	if !ownUpload(f, actor.ID, purpose) {
		return domain.StoredFile{}, ErrCoverFileNotFound
	}
	switch f.Status {
	case domain.StoredFileReady:
		return *f, nil
	case domain.StoredFilePending, domain.StoredFileStored, domain.StoredFileProcessing:
	default:
		return domain.StoredFile{}, ErrCoverNotPending
	}
	pol, err := uc.policyFor(ctx, purpose)
	if err != nil {
		return domain.StoredFile{}, err
	}
	key, err := storage.ParseKey(f.ObjectKey)
	if err != nil {
		return domain.StoredFile{}, fmt.Errorf("ảnh bìa: khoá đối tượng đã ghi không hợp lệ: %w", err)
	}

	insp, err := uc.inspect(ctx, *f, key, pol)
	if err != nil {
		return domain.StoredFile{}, err
	}

	now := uc.clock()
	verbs := verbsFor(string(purpose))
	var done domain.StoredFile
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		cur, err := uc.files.ForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if !ownUpload(cur, actor.ID, purpose) {
			return ErrCoverFileNotFound
		}
		if cur.Status == domain.StoredFileReady {
			done = *cur // another completion finished first: same outcome, nothing to write
			return nil
		}
		switch insp.kind {
		case coverReady, coverFailed:
			final, err := uc.walkToEnd(ctx, tx, *cur, insp, now)
			if err != nil {
				return err
			}
			if insp.kind == coverFailed {
				return writeCoverAudit(ctx, tx, actor, verbs, verbs.rejected, now, map[string]any{
					"tep_id": id, "ly_do": insp.reason, "muc_noi_dung_id": cur.SubjectID,
				})
			}
			done = final
			return writeCoverAudit(ctx, tx, actor, verbs, verbs.stored, now, map[string]any{
				"tep_id": id, "muc_noi_dung_id": cur.SubjectID, "loai_tep": insp.facts.MIMEType,
				"kich_thuoc": insp.facts.SizeBytes, "sha256": insp.facts.SHA256,
				"ban_dan_xuat": CoverDerivativeVariant, "khoi_phuc_tu_dich": insp.recovered,
			})
		case coverRejected:
			if cur.Status != domain.StoredFilePending {
				return ErrCoverNotPending
			}
			d := map[string]any{"tep_id": id, "muc_noi_dung_id": cur.SubjectID,
				"ly_do": insp.reason, "da_xoa_tep_tam": insp.tempRemoved}
			if insp.signature != "" {
				d["chu_ky_ma_doc"] = insp.signature
			}
			if err := uc.files.Transition(ctx, tx, id, domain.StoredFilePending, domain.StoredFileRejected, now); err != nil {
				return err
			}
			return writeCoverAudit(ctx, tx, actor, verbs, verbs.rejected, now, d)
		}
		return fmt.Errorf("ảnh bìa: kết quả kiểm tra không rõ (%d)", insp.kind)
	})
	if err != nil {
		return domain.StoredFile{}, bocNoiDung(ctx, "hoàn tất ảnh bìa", err)
	}
	if done.ID != "" {
		return done, nil
	}
	return domain.StoredFile{}, &CoverRejection{Reason: insp.reason}
}

// walkToEnd moves a row along pending → scanning → stored → processing → ready (or → failed), from
// wherever it stands, one guarded edge per UPDATE so stored_file_guard sees every step.
func (uc *ContentCovers) walkToEnd(ctx context.Context, tx *store.ScopedTx, cur domain.StoredFile,
	insp coverInspection, now time.Time) (domain.StoredFile, error) {

	if cur.Status == domain.StoredFilePending {
		if err := uc.files.Transition(ctx, tx, cur.ID, domain.StoredFilePending, domain.StoredFileScanning, now); err != nil {
			return domain.StoredFile{}, err
		}
		if err := uc.files.MarkStored(ctx, tx, cur.ID, insp.facts, now); err != nil {
			return domain.StoredFile{}, err
		}
		cur.Status, cur.MIMEType, cur.SizeBytes, cur.SHA256 =
			domain.StoredFileStored, insp.facts.MIMEType, insp.facts.SizeBytes, insp.facts.SHA256
	}
	if cur.Status == domain.StoredFileStored {
		if err := uc.files.Transition(ctx, tx, cur.ID, domain.StoredFileStored, domain.StoredFileProcessing, now); err != nil {
			return domain.StoredFile{}, err
		}
		cur.Status = domain.StoredFileProcessing
	}
	if cur.Status != domain.StoredFileProcessing {
		return domain.StoredFile{}, ErrCoverNotPending
	}
	to := domain.StoredFileReady
	if insp.kind == coverFailed {
		to = domain.StoredFileFailed
	}
	if err := uc.files.Transition(ctx, tx, cur.ID, domain.StoredFileProcessing, to, now); err != nil {
		return domain.StoredFile{}, err
	}
	cur.Status, cur.UpdatedAt = to, now
	return cur, nil
}

// ownUpload: the row exists, is a content-item file of THIS purpose, and was issued to THIS officer.
func ownUpload(f *domain.StoredFile, officer string, purpose storage.Purpose) bool {
	return f != nil && officer != "" && f.SubjectType == domain.StoredFileSubjectContentItem &&
		f.UploadedBy == officer && f.Purpose == string(purpose)
}

// coverAuditSubject is the trail's locator for an article's file (cover or body image, by v.subject): an
// article has no business code (chuDeNoiDungMiniApp explains), and the file's own id is in the delta.
func coverAuditSubject(v uploadVerbs, at time.Time) string {
	return v.subject + at.UTC().Format("2006-01-02")
}

func writeCoverAudit(ctx context.Context, tx *store.ScopedTx, actor audit.Actor, v uploadVerbs, action string,
	at time.Time, d map[string]any) error {
	delta, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("ảnh bìa: mã hoá delta: %w", err)
	}
	// SAME TRANSACTION AS THE ROW (rule 6, invariant 3); TenantID filled by audit.Write from the tx.
	return audit.Write(ctx, tx, audit.Entry{Actor: actor, Action: action, Subject: coverAuditSubject(v, at),
		At: at, Delta: delta})
}

// inspect is the lock-free half of complete. An error means nothing may be decided (scanner down, object
// replaced mid-inspection, store failure): complete writes nothing and upload abandons the row.
func (uc *ContentCovers) inspect(ctx context.Context, f domain.StoredFile, key storage.Key,
	pol uploadpolicy.Policy) (coverInspection, error) {

	if f.Status != domain.StoredFilePending {
		// stored / processing: the original is already in the private bucket and was scanned before it
		// got there. Only the derivative is owed.
		return uc.fromDestination(ctx, f, key)
	}
	uploadKey, err := key.UploadPath()
	if err != nil {
		return coverInspection{}, fmt.Errorf("ảnh bìa: dựng khoá tải lên: %w", err)
	}
	st, err := uc.objects.Stat(ctx, storage.BucketTemp, uploadKey)
	if errors.Is(err, storage.ErrNotFound) {
		return uc.fromDestination(ctx, f, key)
	}
	if err != nil {
		return coverInspection{}, coverStorageErr("đọc thông tin tệp tạm", err)
	}

	// The CURRENT policy decides: a limit tightened since the request rejects here.
	if st.Size <= 0 || st.Size > pol.MaxBytes {
		return uc.reject(ctx, uploadKey, CoverRejectTooLarge, ""), nil
	}
	head, err := uc.objects.ReadHead(ctx, storage.BucketTemp, uploadKey, st.ETag, storage.SniffBytes)
	if err != nil {
		return coverInspection{}, coverStorageErr("đọc đầu tệp", err)
	}
	mime, ext, ok := storage.SniffMIME(head)
	if !ok || !pol.AllowsMIME(mime) {
		return uc.reject(ctx, uploadKey, CoverRejectTypeNotAllowed, ""), nil
	}
	if ext != key.Ext {
		return uc.reject(ctx, uploadKey, CoverRejectTypeMismatch, ""), nil
	}
	if pol.FileCountLimited {
		have, err := uc.files.CountForSubject(ctx, f.SubjectID, f.Purpose) // the row's purpose: cover or body image
		if err != nil {
			return coverInspection{}, err
		}
		if have >= pol.MaxFilesPerSubject {
			return uc.reject(ctx, uploadKey, CoverRejectCountReached, ""), nil
		}
	}

	rc, size, err := uc.objects.Open(ctx, storage.BucketTemp, uploadKey, st.ETag)
	if err != nil {
		return coverInspection{}, coverStorageErr("mở tệp để quét", err)
	}
	res, err := uc.scanner.Scan(ctx, rc, size)
	closeErr := rc.Close()
	if err != nil {
		if errors.Is(err, malwarescan.ErrNotConfigured) {
			return coverInspection{}, fmt.Errorf("%w: %w", ErrCoverUploadNotConfigured, err)
		}
		if errors.Is(err, storage.ErrChanged) {
			return coverInspection{}, fmt.Errorf("%w: %w", ErrCoverUploadChanged, err)
		}
		// UNSCANNABLE IS NEVER CLEAN: unreachable, timed out, over clamd's limit, an unknown reply.
		return coverInspection{}, fmt.Errorf("%w: %w", ErrCoverScanUnavailable, err)
	}
	if closeErr != nil {
		return coverInspection{}, coverStorageErr("đóng tệp sau khi quét", closeErr)
	}
	if !res.Clean {
		return uc.reject(ctx, uploadKey, CoverRejectMalware, res.Signature), nil
	}

	sum, err := uc.objects.SHA256(ctx, storage.BucketTemp, uploadKey, st.ETag)
	if err != nil {
		return coverInspection{}, coverStorageErr("tính sha256", err)
	}
	pr, err := uc.objects.Promote(ctx, uploadKey, st.ETag, key, storage.BucketPrivate)
	switch {
	case err == nil, errors.Is(err, storage.ErrTempCleanup):
		// ErrTempCleanup: promoted; the leftover in temp expires with the bucket's 1-day lifecycle.
	case errors.Is(err, storage.ErrExists), errors.Is(err, storage.ErrNotFound):
		return uc.fromDestination(ctx, f, key) // another completion promoted it first
	default:
		return coverInspection{}, coverStorageErr("chép tệp sang kho lưu", err)
	}
	facts := domain.StoredFileFacts{MIMEType: pr.ContentType, SizeBytes: st.Size, SHA256: sum}
	return uc.derive(ctx, f, key, pr.ETag, facts, false)
}

// fromDestination handles "the original is already in the private bucket" (a concurrent Promote got
// there first) or "nothing is in either bucket". A destination object is TRUSTED AS SCANNED because
// nothing else writes there: Promote is the only path into `content-source/…/comms/content-image/…/
// original.*`, it runs only after a clean scan, and IAM scopes this service's key to its own subtree
// (ADR 0052 §3).
//
// NOTHING IN EITHER BUCKET right after PutUpload succeeded is not a client's doing — the temp bucket is
// not the one this service writes to, or something removed the object (09/10/2026: a missing temp
// bucket). An error, so the caller abandons the row and the log names the cause.
func (uc *ContentCovers) fromDestination(ctx context.Context, f domain.StoredFile,
	key storage.Key) (coverInspection, error) {

	st, err := uc.objects.Stat(ctx, storage.BucketPrivate, f.ObjectKey)
	if errors.Is(err, storage.ErrNotFound) {
		if f.Status != domain.StoredFilePending {
			return coverInspection{}, errors.New("ảnh bìa: dòng đã lưu nhưng không thấy bản gốc ở kho lưu")
		}
		return coverInspection{}, fmt.Errorf("ảnh bìa: vừa ghi tệp %s vào kho tạm nhưng không thấy ở kho tạm lẫn kho lưu", f.ID)
	}
	if err != nil {
		return coverInspection{}, coverStorageErr("đọc thông tin tệp đã lưu", err)
	}
	facts := domain.StoredFileFacts{MIMEType: f.MIMEType, SizeBytes: f.SizeBytes, SHA256: f.SHA256}
	if f.Status == domain.StoredFilePending {
		head, err := uc.objects.ReadHead(ctx, storage.BucketPrivate, f.ObjectKey, st.ETag, storage.SniffBytes)
		if err != nil {
			return coverInspection{}, coverStorageErr("đọc đầu tệp đã lưu", err)
		}
		mime, ext, ok := storage.SniffMIME(head)
		if !ok || ext != key.Ext {
			// Promote refuses exactly this, so it cannot be ours. Not recorded; an operator looks.
			return coverInspection{}, errors.New("ảnh bìa: đối tượng ở kho lưu không khớp kiểu của khoá")
		}
		sum, err := uc.objects.SHA256(ctx, storage.BucketPrivate, f.ObjectKey, st.ETag)
		if err != nil {
			return coverInspection{}, coverStorageErr("tính sha256 tệp đã lưu", err)
		}
		facts = domain.StoredFileFacts{MIMEType: mime, SizeBytes: st.Size, SHA256: sum}
	}
	return uc.derive(ctx, f, key, st.ETag, facts, true)
}

// derive produces the `thumb-1280` JPEG from the promoted original and stores it with
// PutServerProduced. An image that cannot be re-encoded is a FILE outcome (coverFailed, 422); a store
// that cannot be reached is an error (retry).
func (uc *ContentCovers) derive(ctx context.Context, f domain.StoredFile, orig storage.Key, etag string,
	facts domain.StoredFileFacts, recovered bool) (coverInspection, error) {

	out := coverInspection{kind: coverReady, facts: facts, recovered: recovered}
	select {
	case uc.decodeSlot <- struct{}{}:
		defer func() { <-uc.decodeSlot }()
	case <-ctx.Done():
		return coverInspection{}, ctx.Err()
	}

	head, err := uc.readHead(ctx, f.ObjectKey, etag)
	if err != nil {
		return coverInspection{}, err
	}
	h, err := readCoverHeader(facts.MIMEType, head)
	if err != nil {
		out.kind, out.reason = coverFailed, CoverRejectUndecodable
		return out, nil
	}
	if decodeCost(facts.MIMEType, h) > coverDecodeBudget {
		out.kind, out.reason = coverFailed, CoverRejectTooManyPixels
		return out, nil
	}
	rc, _, err := uc.objects.Open(ctx, storage.BucketPrivate, f.ObjectKey, etag)
	if err != nil {
		return coverInspection{}, coverStorageErr("mở bản gốc để giải mã", err)
	}
	img, err := decodeCover(facts.MIMEType, rc)
	closeErr := rc.Close()
	if err != nil {
		if errors.Is(err, storage.ErrChanged) || errors.Is(err, storage.ErrNotFound) {
			return coverInspection{}, coverStorageErr("đọc bản gốc", err)
		}
		out.kind, out.reason = coverFailed, CoverRejectUndecodable
		return out, nil
	}
	if closeErr != nil {
		return coverInspection{}, coverStorageErr("đóng bản gốc", closeErr)
	}
	jpg, err := renderCoverDerivative(img, h.orientation)
	if err != nil {
		out.kind, out.reason = coverFailed, CoverRejectUndecodable
		return out, nil
	}
	_, err = uc.objects.PutServerProduced(ctx, coverDerivativeKey(orig), bytes.NewReader(jpg), int64(len(jpg)))
	switch {
	case err == nil, errors.Is(err, storage.ErrExists):
		// ErrExists: a previous completion produced it; dst is derived 1:1 from an immutable original.
	default:
		return coverInspection{}, coverStorageErr("ghi bản dẫn xuất", err)
	}
	return out, nil
}

// readHead reads up to coverHeadBytes of the original, bound to its ETag.
func (uc *ContentCovers) readHead(ctx context.Context, key, etag string) ([]byte, error) {
	rc, _, err := uc.objects.Open(ctx, storage.BucketPrivate, key, etag)
	if err != nil {
		return nil, coverStorageErr("mở bản gốc", err)
	}
	head, err := io.ReadAll(io.LimitReader(rc, coverHeadBytes))
	closeErr := rc.Close()
	if err != nil {
		return nil, coverStorageErr("đọc đầu bản gốc", err)
	}
	if closeErr != nil {
		return nil, coverStorageErr("đóng bản gốc", closeErr)
	}
	return head, nil
}

// reject deletes the temp object and reports the outcome. A delete that fails is RECORDED, not hidden
// (`da_xoa_tep_tam: false`); the temp bucket's lifecycle removes the object within a day.
func (uc *ContentCovers) reject(ctx context.Context, uploadKey, reason, signature string) coverInspection {
	removed := uc.objects.PurgeAllVersions(ctx, storage.BucketTemp, uploadKey) == nil
	return coverInspection{kind: coverRejected, reason: reason, signature: signature, tempRemoved: removed}
}

// coverStorageErr keeps the sentinels the handler maps (not configured, changed) and wraps the rest.
func coverStorageErr(what string, err error) error {
	switch {
	case errors.Is(err, storage.ErrNotConfigured):
		return fmt.Errorf("%w: %w", ErrCoverUploadNotConfigured, err)
	case errors.Is(err, storage.ErrChanged):
		return fmt.Errorf("%w: %w", ErrCoverUploadChanged, err)
	}
	return fmt.Errorf("ảnh bìa: %s: %w", what, err)
}

// coverDerivativeKey is the private derivative of an original key: same commune, month, service,
// purpose and object id; variant thumb-1280; always JPEG (cover_image.go).
func coverDerivativeKey(orig storage.Key) storage.Key {
	k := orig
	k.Variant, k.Ext = CoverDerivativeVariant, "jpg"
	return k
}

// coverPublicKey is that derivative's public twin — the only key PublishDerivative accepts for it.
func coverPublicKey(orig storage.Key) storage.Key {
	k := coverDerivativeKey(orig)
	k.Class = storage.ClassPublicMedia
	return k
}

// --- staff preview, public URLs -----------------------------------------------------------------------

// CoverView is what the staff detail shows about an article's cover.
type CoverView struct {
	FileID string
	Status domain.StoredFileStatus
	// Public reports that a public copy is recorded (the article is published with this cover).
	Public bool
	// PreviewURL is a presigned GET of the DERIVATIVE — what residents would see, EXIF-free — valid until
	// PreviewExpiresAt. A bearer credential: never logged. Empty when not ready or not configured.
	PreviewURL       storage.PresignedURL
	PreviewExpiresAt time.Time
}

// View describes the cover file of one article. NOT AUDITED: a member of staff, in their own commune,
// looking at a picture their commune is about to publish — the footing of GET /content-items/{id}.
//
// A missing row (FK makes it impossible while the column points at it, unless soft-deleted) answers a
// view with no status rather than an error: the article is still worth showing.
func (uc *ContentCovers) View(ctx context.Context, fileID string) (CoverView, error) {
	v := CoverView{FileID: fileID}
	f, err := uc.files.ByID(ctx, fileID)
	if err != nil {
		return v, fmt.Errorf("ảnh bìa: đọc tệp: %w", err)
	}
	if f == nil {
		return v, nil
	}
	v.Status, v.Public = f.Status, f.PublicObjectKey != ""
	if f.Status != domain.StoredFileReady || uc.objects == nil {
		return v, nil
	}
	orig, err := storage.ParseKey(f.ObjectKey)
	if err != nil {
		return v, fmt.Errorf("ảnh bìa: khoá đối tượng đã ghi không hợp lệ: %w", err)
	}
	derivKey, err := coverDerivativeKey(orig).Path()
	if err != nil {
		return v, fmt.Errorf("ảnh bìa: dựng khoá bản dẫn xuất: %w", err)
	}
	u, err := uc.objects.PresignDownload(ctx, storage.BucketPrivate, derivKey, storage.MaxDownloadTTL,
		f.OriginalName)
	if err != nil {
		return v, coverStorageErr("ký liên kết xem trước", err)
	}
	v.PreviewURL, v.PreviewExpiresAt = u, uc.clock().Add(storage.MaxDownloadTTL)
	return v, nil
}

// CoverViews is View for a PAGE of the staff list (owner decision 10/10/2026, bug sheet row 22: the list
// shows each article's cover thumbnail). Keyed by file id; every requested id is present, built exactly as
// View builds it — same derivative, same signer, same TTL (storage.MaxDownloadTTL) — except that no file
// name is signed into the link (below).
//
// WHAT IT COSTS, and why that is the cheapest correct shape:
//
//	one SQL read    LiveByIDs — the whole page's rows in one statement, never one ByID per row
//	N signatures    PresignDownload signs OFFLINE (HMAC over the URL, no round trip to the object store)
//	no audit entry  same footing as View: staff of THIS commune looking at pictures their commune
//	                publishes. Rule 6 invariant 7 audits full personal data and cross-commune reads; a
//	                news cover is neither (the derivative is the EXIF-free JPEG residents get on publish),
//	                so N previews cost N HMACs and zero ledger rows
//
// A signing failure leaves that row without its URL and is returned beside the views (the first one), so
// the handler can log it and still show the list.
func (uc *ContentCovers) CoverViews(ctx context.Context, fileIDs []string) (map[string]CoverView, error) {
	out := make(map[string]CoverView, len(fileIDs))
	if len(fileIDs) == 0 {
		return out, nil
	}
	rows, err := uc.files.LiveByIDs(ctx, fileIDs)
	if err != nil {
		return out, fmt.Errorf("ảnh bìa: đọc tệp theo trang: %w", err)
	}
	var firstErr error
	for _, id := range fileIDs {
		v := CoverView{FileID: id}
		f, ok := rows[id]
		if ok {
			v.Status, v.Public = f.Status, f.PublicObjectKey != ""
		}
		if ok && f.Status == domain.StoredFileReady && uc.objects != nil {
			// THE ONE DIFFERENCE FROM View: no file name in the signed disposition. `original_name` CAN
			// NAME A PERSON (rule 3) and rides in the presigned query string; a thumbnail is shown, never
			// saved, so a hundred of them need no name — the store answers `download.jpg`.
			f.OriginalName = ""
			u, err := uc.presignDerivative(ctx, f)
			if err != nil && firstErr == nil {
				firstErr = err
			}
			if err == nil {
				v.PreviewURL, v.PreviewExpiresAt = u, uc.clock().Add(storage.MaxDownloadTTL)
			}
		}
		out[id] = v
	}
	return out, firstErr
}

// PublicImageURLs maps cover file ids to the anonymous URL of their published derivative, for the
// public news routes. A file with no recorded public copy is absent — and so is every file when object
// storage is not configured: an article without its picture is the safe failure, never a broken link.
func (uc *ContentCovers) PublicImageURLs(ctx context.Context, fileIDs []string) (map[string]string, error) {
	out := map[string]string{}
	if len(fileIDs) == 0 || uc.objects == nil {
		return out, nil
	}
	keys, err := uc.files.PublicObjectKeys(ctx, fileIDs)
	if err != nil {
		return nil, err
	}
	for id, key := range keys {
		u, err := uc.objects.PublicURL(key)
		if err != nil {
			continue // a key the schema admitted but storage refuses: omitted, never guessed
		}
		out[id] = u
	}
	return out, nil
}

// BodyImageView is what the staff detail shows about one image the body references: CoverView's fields,
// for the editor to draw the figure. PreviewURL is "" when the id is not a ready body image of THIS article
// (unknown, another article's, another purpose, not finished) or storage is not configured.
type BodyImageView = CoverView

// BodyImageViews describes the body images an article's body references, in fileIDs order (the caller
// passes richtext.ImageFileIDs of the stored body). ONE READ of the article's live body-image rows, then
// one offline signature per ready file — never a read per image. NOT AUDITED, for View's reason.
//
// A signing failure leaves that image without its URL and is returned beside the views, so the handler
// can log it and still show the article.
func (uc *ContentCovers) BodyImageViews(ctx context.Context, itemID string, fileIDs []string) (
	[]BodyImageView, error) {

	if len(fileIDs) == 0 || itemID == "" {
		return nil, nil
	}
	rows, err := uc.files.LiveForSubject(ctx, itemID, string(bodyImagePurpose))
	if err != nil {
		return nil, fmt.Errorf("ảnh thân bài: đọc tệp của mục: %w", err)
	}
	byID := make(map[string]domain.StoredFile, len(rows))
	for _, f := range rows {
		byID[f.ID] = f
	}
	var firstErr error
	out := make([]BodyImageView, 0, len(fileIDs))
	for _, id := range fileIDs {
		v := BodyImageView{FileID: id}
		f, ok := byID[id]
		if ok {
			v.Status, v.Public = f.Status, f.PublicObjectKey != ""
		}
		if ok && f.Status == domain.StoredFileReady && uc.objects != nil {
			u, err := uc.presignDerivative(ctx, f)
			if err != nil && firstErr == nil {
				firstErr = err
			}
			if err == nil {
				v.PreviewURL, v.PreviewExpiresAt = u, uc.clock().Add(storage.MaxDownloadTTL)
			}
		}
		out = append(out, v)
	}
	return out, firstErr
}

// presignDerivative signs a short GET of a file's PRIVATE `thumb-1280` derivative — what residents would
// see once published, EXIF-free. The mechanism and TTL of the cover preview (View).
func (uc *ContentCovers) presignDerivative(ctx context.Context, f domain.StoredFile) (storage.PresignedURL, error) {
	orig, err := storage.ParseKey(f.ObjectKey)
	if err != nil {
		return "", fmt.Errorf("ảnh: khoá đối tượng đã ghi không hợp lệ: %w", err)
	}
	derivKey, err := coverDerivativeKey(orig).Path()
	if err != nil {
		return "", fmt.Errorf("ảnh: dựng khoá bản dẫn xuất: %w", err)
	}
	u, err := uc.objects.PresignDownload(ctx, storage.BucketPrivate, derivKey, storage.MaxDownloadTTL, f.OriginalName)
	if err != nil {
		return "", coverStorageErr("ký liên kết xem trước", err)
	}
	return u, nil
}

// PublicBodyImageURLs maps the PUBLISHED body images of ONE article of the CURRENT commune to their
// anonymous URL — the public detail's resolver (ADR 0067 §Sửa đổi 03/10/2026, K2/K7). One read: the
// article's live `content-body-image` rows (commune bound to $1, subject = itemID), keeping only those
// with a recorded public copy. Another commune's, another article's, the cover, an unpublished file: absent,
// and the read path drops the image block. No object storage → empty, never a guessed link.
func (uc *ContentCovers) PublicBodyImageURLs(ctx context.Context, itemID string) (map[string]string, error) {
	out := map[string]string{}
	if itemID == "" || uc.objects == nil {
		return out, nil
	}
	rows, err := uc.files.LiveForSubject(ctx, itemID, string(bodyImagePurpose))
	if err != nil {
		return nil, fmt.Errorf("ảnh thân bài: đọc tệp công khai của mục: %w", err)
	}
	for _, f := range rows {
		if f.PublicObjectKey == "" || f.Status != domain.StoredFileReady {
			continue
		}
		u, err := uc.objects.PublicURL(f.PublicObjectKey)
		if err != nil {
			continue // a key the schema admitted but storage refuses: omitted, never guessed
		}
		out[f.ID] = u
	}
	return out, nil
}

// --- publishing the derivative with the article (ADR 0052 §11) ---------------------------------------

// coverPublisher keeps the public copy of an article's cover in step with the article. It is used by
// SoanNoiDungMiniApp.Them / Sua and holds no state of its own.
//
// THE OBJECT STORE IS NOT TRANSACTIONAL, so the order is chosen per direction (the compensation is the
// `noi_dung_mini_app_anh_bia_cong_khai` flow owed to kb/30-indexes/transaction-boundaries.json):
//
//	PUBLISH   INSIDE the article's transaction, with the article row locked: copy (PublishDerivative),
//	          then record `public_object_key`, then commit. Copy fails → the transaction rolls back and
//	          the request is refused (503): nothing written, the article as it was. Transaction fails
//	          after the copy → the copy is withdrawn (best effort; PublishDerivative is idempotent, so a
//	          retry is safe). A copy left behind by a failed withdrawal is reachable only through its key
//	          — two random ULIDs — and the public bucket is not listable (ADR 0052 §2).
//	          The lock is held for ONE server-side copy of a ≤ 1 MB JPEG, bounded by the request
//	          context; that is the price of having no window in which another edit changes the cover
//	          between the copy and the record (a scan or a decode would NOT be acceptable here — those
//	          stay outside, in the upload's completion).
//	WITHDRAW  the transaction first (the article stops being public the moment it commits, whatever
//	          MinIO is doing — hiding an article must never wait on storage), KEEPING the file's key
//	          as the marker "a public copy may exist". After commit: UnpublishDerivative, then a second
//	          short transaction clears the key with its own audit entry. If either fails, the key stays,
//	          and the NEXT edit of the article retries it (PublicForSubject finds it).
//
// What no order can fix, stated: a CDN or browser that fetched the image may serve it for up to a year
// after withdrawal (storage.PublicCacheControl); purging caches is not provisioned.
type coverPublisher struct {
	files   CoverFiles
	objects CoverObjectStore // nil = not configured
	log     *slog.Logger
}

// undoPublish is the compensation of a copy whose transaction did not commit.
func (p *coverPublisher) undoPublish(ctx context.Context, fileID string) {
	f, err := p.files.ByID(ctx, fileID)
	if err != nil || f == nil || f.PublicObjectKey != "" || p.objects == nil {
		return // a recorded key means a committed publish owns that copy — never withdraw it here
	}
	orig, err := storage.ParseKey(f.ObjectKey)
	if err != nil {
		return
	}
	if err := p.objects.UnpublishDerivative(ctx, coverPublicKey(orig)); err != nil {
		pub, _ := coverPublicKey(orig).Path()
		p.log.Warn("ảnh bìa: giao dịch không chốt, gỡ bản công khai vừa chép cũng hỏng — bản sao mồ côi",
			"xa", string(tenant.MustFrom(ctx)), "khoa_cong_khai", pub, "err", err)
	}
}

// checkAttach is the in-transaction half of attaching fileID to itemID: the row locked and checked
// again (CheckCoverUsable), the floor 0011's trigger also holds.
func (p *coverPublisher) checkAttach(ctx context.Context, tx *store.ScopedTx, itemID, fileID string) error {
	f, err := p.files.ForUpdate(ctx, tx, fileID)
	if err != nil {
		return err
	}
	return domain.CheckCoverUsable(f, itemID, string(coverPurpose))
}

// checkBodyImages is the attach check of a body on save (ADR 0067 §Sửa đổi 03/10/2026, K2): EVERY file id
// the sanitised body references, locked and checked in the article's transaction — a completed, clean
// `content-body-image` of THIS commune (the read is commune-bound), uploaded for THIS article. One error
// for every cause (domain.ErrBodyImageNotUsable). No trigger stands under this one: the body is free text.
func (p *coverPublisher) checkBodyImages(ctx context.Context, tx *store.ScopedTx, itemID string, ids []string) error {
	for _, id := range ids {
		f, err := p.files.ForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := domain.CheckBodyImageUsable(f, itemID, string(bodyImagePurpose)); err != nil {
			return err
		}
	}
	return nil
}

// retireUnreferencedBodyImages soft-deletes, in the save's transaction, the body images the saved body does
// not show — freeing their slot of H7's per-article count (rule 7: soft, the object stays, `deleted_by` is
// the officer's business code). Only a `content-body-image` file uploaded for THIS article is ever touched
// (read by subject and purpose, re-checked under lock): never the cover, never another article's file. A file still carrying a public key is NOT retired here — the schema refuses a
// deleted row with a key, and the copy must go first: withdrawAfterCommit retires it once withdrawn.
//
// NO audit.Write HERE, deliberately: tx is the SAVE's transaction, and Them / Sua write the returned ids
// into that save's own entry (`body_images_retired`) — one act, one entry, one transaction (rule 6, invariant 3),
// exactly as it records `audio_file_retired`.
//
// WHICH FILES (coordinator, 03/10/2026): every READY body image of this article the SAVED body does not
// reference — the ones this edit removed AND the ones uploaded, completed and never put in the body (the
// "orphans" that would otherwise hold a slot for ever). A `pending` / `scanning` / `stored` / `processing`
// upload is LEFT ALONE: it may be in flight while the officer saves, and its figure arrives with the next
// save. Run on every successful create and edit, so the count stays honest without a sweep job.
func (p *coverPublisher) retireUnreferencedBodyImages(ctx context.Context, tx *store.ScopedTx, itemID,
	body string, by string, at time.Time) ([]string, error) {

	live, err := p.files.LiveForSubject(ctx, itemID, string(bodyImagePurpose))
	if err != nil {
		return nil, err
	}
	inBody := map[string]bool{}
	for _, id := range richtext.ImageFileIDs(body) {
		inBody[id] = true
	}
	var retired []string
	for _, cand := range live {
		if inBody[cand.ID] || cand.Status != domain.StoredFileReady {
			continue
		}
		id := cand.ID
		f, err := p.files.ForUpdate(ctx, tx, id)
		if err != nil {
			return nil, err
		}
		if f == nil || f.SubjectType != domain.StoredFileSubjectContentItem || f.SubjectID != itemID ||
			f.Purpose != string(bodyImagePurpose) || f.Status != domain.StoredFileReady || f.PublicObjectKey != "" {
			continue
		}
		err = p.files.SoftDelete(ctx, tx, id, by, bodyImageRetireReason, at)
		if err != nil && !errors.Is(err, commsstore.ErrStoredFileMoved) {
			return nil, err
		}
		retired = append(retired, id)
	}
	return retired, nil
}

// settlement is what settle decided inside the article's transaction.
type settlement struct {
	// published is the cover whose public key this transaction records ("" = none newly recorded).
	published string
	// bodyPublished are the body images whose public key this transaction records.
	bodyPublished []string
	// copied are the files PublishDerivative ran for in this transaction — a rollback must undo each
	// (undoPublish).
	copied []string
	// withdraw are the files whose public copy goes after commit (withdrawAfterCommit).
	withdraw []domain.StoredFile
}

// settle runs INSIDE the article's transaction, after the article row is written and while it is
// locked, and decides PER PURPOSE which public copies the article must have:
//
//	content-image        the cover — public iff the article is `dang-hien` and points at it
//	content-body-image   every image the body references (richtext.ImageFileIDs) — public iff the
//	                     article is `dang-hien` (ADR 0067 §Sửa đổi 03/10/2026, K3)
//	anything else        never public: withdrawn
//
// Wanted and not yet public → copied and recorded here. Public and not wanted (hidden, unpublished,
// deleted, replaced, removed from the body) → listed for withdrawal after commit. A body image that is
// referenced but not usable (not this article's, not ready — a body saved before the attach check) is
// skipped, never copied: the public read drops it anyway. An unusable COVER still refuses the write.
//
// THE LOCK IS HELD FOR THE COPIES: one ≤ 1 MB server-side copy per newly published image, at most the
// policy's per-article count (H7) — the same reasoning as coverPublisher's PUBLISH paragraph, times n.
func (p *coverPublisher) settle(ctx context.Context, tx *store.ScopedTx, n domain.NoiDungMiniApp,
	at time.Time) (settlement, error) {

	public, err := p.files.PublicForSubject(ctx, tx, n.ID)
	if err != nil {
		return settlement{}, err
	}
	wantCover := ""
	var wantBody []string
	if n.HienChoDan() {
		wantCover = n.CoverImageFileID
		wantBody = richtext.ImageFileIDs(n.NoiDung)
	}
	inBody := make(map[string]bool, len(wantBody))
	for _, id := range wantBody {
		inBody[id] = true
	}
	var s settlement
	have := map[string]bool{}
	for _, f := range public {
		keep := (f.Purpose == string(coverPurpose) && f.ID == wantCover && wantCover != "") ||
			(f.Purpose == string(bodyImagePurpose) && inBody[f.ID])
		if keep {
			have[f.ID] = true
			continue
		}
		s.withdraw = append(s.withdraw, f)
	}
	if wantCover != "" && !have[wantCover] {
		f, err := p.files.ForUpdate(ctx, tx, wantCover)
		if err != nil {
			return settlement{}, err
		}
		if err := domain.CheckCoverUsable(f, n.ID, string(coverPurpose)); err != nil {
			return settlement{}, err
		}
		if err := p.publish(ctx, tx, *f, at, &s); err != nil {
			return s, err
		}
		s.published = wantCover
	}
	for _, id := range wantBody {
		if have[id] {
			continue
		}
		f, err := p.files.ForUpdate(ctx, tx, id)
		if err != nil {
			return s, err
		}
		if domain.CheckBodyImageUsable(f, n.ID, string(bodyImagePurpose)) != nil {
			continue
		}
		if err := p.publish(ctx, tx, *f, at, &s); err != nil {
			return s, err
		}
		s.bodyPublished = append(s.bodyPublished, id)
	}
	return s, nil
}

// publish copies one ready file's derivative to its public twin and records the key, in the article's
// transaction. The copy is listed in s.copied BEFORE the key is written, so a failure of the write still
// undoes the copy.
func (p *coverPublisher) publish(ctx context.Context, tx *store.ScopedTx, f domain.StoredFile, at time.Time,
	s *settlement) error {

	if p.objects == nil {
		return ErrCoverUploadNotConfigured
	}
	orig, err := storage.ParseKey(f.ObjectKey)
	if err != nil {
		return fmt.Errorf("ảnh: khoá đối tượng đã ghi không hợp lệ: %w", err)
	}
	pub, err := coverPublicKey(orig).Path()
	if err != nil {
		return fmt.Errorf("ảnh: dựng khoá công khai: %w", err)
	}
	if err := p.objects.PublishDerivative(ctx, coverDerivativeKey(orig), coverPublicKey(orig)); err != nil {
		return fmt.Errorf("%w: %w", ErrCoverPublishUnavailable, err)
	}
	s.copied = append(s.copied, f.ID)
	return p.files.SetPublicObjectKey(ctx, tx, f.ID, pub, at)
}

// undoCopies is the compensation of every copy a rolled-back transaction made.
func (p *coverPublisher) undoCopies(ctx context.Context, s settlement) {
	for _, id := range s.copied {
		p.undoPublish(ctx, id)
	}
}

// withdrawAfterCommit removes the public copies settle listed, then clears each key with its own audit
// entry. Failures are logged by object key (no personal data in a key, ADR 0052 §3) and left for the
// next edit to retry: the key stays set, which is the durable record that a copy may still exist.
//
// A BODY IMAGE THE ARTICLE'S BODY NO LONGER REFERENCES is also retired (soft-deleted) in that second
// transaction, once its key is cleared — the published counterpart of retireUnreferencedBodyImages. One that is
// still referenced (the article was hidden or deleted) keeps its row: it is part of the record (rule 7).
func (p *coverPublisher) withdrawAfterCommit(ctx context.Context, db *store.DB, n domain.NoiDungMiniApp,
	files []domain.StoredFile, actor audit.Actor, at time.Time) {

	inBody := map[string]bool{}
	for _, id := range richtext.ImageFileIDs(n.NoiDung) {
		inBody[id] = true
	}
	for _, f := range files {
		if p.objects == nil {
			p.log.Warn("ảnh bìa: chưa cấu hình kho lưu tệp — bản công khai của ảnh bìa cũ CHƯA được gỡ",
				"xa", string(tenant.MustFrom(ctx)), "khoa_cong_khai", f.PublicObjectKey)
			continue
		}
		key, err := storage.ParseKey(f.PublicObjectKey)
		if err == nil {
			err = p.objects.UnpublishDerivative(ctx, key)
		}
		if err != nil {
			p.log.Warn("ảnh bìa: gỡ bản công khai hỏng — giữ khoá để lần sửa sau thử lại",
				"xa", string(tenant.MustFrom(ctx)), "khoa_cong_khai", f.PublicObjectKey, "err", err)
			continue
		}
		retire := f.Purpose == string(bodyImagePurpose) && !inBody[f.ID]
		err = db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
			if err := p.files.SetPublicObjectKey(ctx, tx, f.ID, "", at); err != nil {
				return err
			}
			d := map[string]any{"id": n.ID, "tep_id": f.ID}
			if retire {
				err := p.files.SoftDelete(ctx, tx, f.ID, actor.ID, bodyImageRetireReason, at)
				if err != nil && !errors.Is(err, commsstore.ErrStoredFileMoved) {
					return err
				}
				d["da_go_khoi_than_bai"] = true
			}
			delta, err := json.Marshal(d)
			if err != nil {
				return fmt.Errorf("ảnh bìa: mã hoá delta: %w", err)
			}
			// SAME TRANSACTION AS THE KEY CLEAR AND THE RETIRE (rule 6, invariant 3).
			return audit.Write(ctx, tx, audit.Entry{Actor: actor, Action: verbsFor(f.Purpose).withdrawn,
				Subject: chuDeNoiDungMiniApp(n), At: at, Delta: delta})
		})
		if err != nil {
			p.log.Warn("ảnh bìa: đã gỡ bản công khai nhưng chưa ghi được — lần sửa sau ghi lại",
				"xa", string(tenant.MustFrom(ctx)), "khoa_cong_khai", f.PublicObjectKey, "err", err)
		}
	}
}
