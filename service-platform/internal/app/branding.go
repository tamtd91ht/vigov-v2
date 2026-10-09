package app

// THE COMMUNE'S IDENTITY IMAGES (ADR 0069) — the logo (#4) and the web-admin banner (#5), uploaded by
// the commune itself under `admin.org`, published the moment they are saved (#3: no review), every set
// and every removal audited in the same transaction with the CB- code.
//
//	Upload   POST …/logo-uploads | …/banner-uploads, ONE multipart request (ADR 0052 §Sửa đổi 09/10/2026):
//	           a. open      pending row + audit, one tx — before a byte of the file is read
//	           b. receive   the file STREAMED into the temp bucket (PutUpload) under this service's own
//	                        MinIO account; a failed write moves the row to `failed`
//	           c. complete  sniff · scan · hash · promote · derivative · PUBLISH · point the profile at
//	                        it — ONE tx
//	Remove   DELETE …/logo | …/banner     column NULL + audit, then withdraw the old public copy
//
// Until 09/10/2026 step b was the browser POSTing straight to MinIO with a presigned form and step c a
// separate `…/completion` call. Both are gone: a failure inside MinIO's write path was invisible to
// ViGov, and MinIO had to accept writes from the Internet (the owner's decision, ADR 0052 §Sửa đổi).
//
// THE SHAPE IS service-comms/internal/app/content_cover.go's, on purpose: one upload discipline for
// every service that stores files (ADR 0052). What differs, and why:
//
//   - COMPLETION IS ALSO THE PUBLICATION. A cover becomes public when its article is published, a later
//     act; here saving is publishing (ADR 0069 #3), so the derivative is copied to the public bucket and
//     the profile is pointed at it inside the completion's own transaction. `ready` therefore means
//     "was published and made current" — there is no ready-but-unused logo.
//   - THE DERIVATIVE KEEPS ALPHA FOR THE LOGO: a 512 px square PNG (imaging.RenderPNGSquare). The
//     cover's JPEG would turn a transparent logo into a white box on the sidebar (ADR 0069 §Vì sao).
//   - THE SUBJECT IS THE COMMUNE ITSELF (subject_id = tenant_id, 0017). There is no subject id on the
//     wire, so there is nothing a client could point at another commune.
//
// THE OBJECT STORE IS NOT TRANSACTIONAL, so the order is chosen per direction (comms' coverPublisher):
//
//	PUBLISH   inside the transaction, with the file row locked: copy (PublishDerivative), record
//	          `public_object_key`, point the profile, audit, commit. Copy fails → rollback, 503, nothing
//	          written. Transaction fails after the copy → the copy is withdrawn (undoPublish, best
//	          effort; a copy left behind is reachable only by its key — two random ULIDs — and the public
//	          bucket is not listable, ADR 0052 §2).
//	WITHDRAW  after commit: the profile already points at the new image (or at nothing), so the old one
//	          stops being CURRENT at commit whatever MinIO does. Then UnpublishDerivative, then a second
//	          short transaction clears the key, soft-deletes the row and audits it. If either fails, the
//	          key stays — the durable marker "a public copy may exist" — and the NEXT set or removal of
//	          that image retries it (PublishedOfPurpose finds it).
//
// What no order can fix, stated: a browser or CDN that fetched an old image may keep it for up to a year
// (storage.PublicCacheControl). That is why every upload gets a NEW key (a new ULID): a replaced logo is
// a new URL, never new bytes behind the old one.
//
// THE LIMITS ARE PLATFORM'S OWN `upload_policy` rows (tenant-logo, tenant-banner: 2 MB, PNG/WebP/JPEG,
// migration 0016), read through core/platformclient/uploadpolicy like every other service reads them, so
// the narrowing and the fail-closed semantics are the same code. Not configured → refusal (503).
//
// WHAT IS NEVER LOGGED OR PUT IN AN ERROR: the original file name (rule 3), file content. The trail
// carries file ids, the sniffed type, the size, the hash and the reason for a refusal — never the name.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"log/slog"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/imaging"
	"github.com/vihat/vigov/core/malwarescan"
	"github.com/vihat/vigov/core/platformclient/uploadpolicy"
	"github.com/vihat/vigov/core/storage"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-platform/internal/domain"
)

// The verbs in the trail — Vietnamese snake_case like every value written to audit_log (ADR 0011).
const (
	ActionBrandingUploadRequested = "yeu_cau_tai_anh_nhan_dien_xa"
	ActionBrandingSet             = "dat_anh_nhan_dien_xa"
	ActionBrandingRejected        = "tu_choi_anh_nhan_dien_xa"
	// ActionBrandingUploadFailed: the file never reached the temp bucket whole (the client stopped, sent
	// a malformed body, or the store refused the write). The row is `failed`; nothing was inspected.
	ActionBrandingUploadFailed = "tai_anh_nhan_dien_xa_khong_thanh"
	// ActionBrandingExpired is NO LONGER WRITTEN (since 09/10/2026 nothing can expire between the
	// request and the bytes: they are one request). Kept because audit entries carrying it exist.
	ActionBrandingExpired   = "anh_nhan_dien_xa_het_han_tai"
	ActionBrandingRemoved   = "go_anh_nhan_dien_xa"
	ActionBrandingWithdrawn = "go_ban_cong_khai_anh_nhan_dien_cu"
)

// Rejection reasons — the `ly_do` of a rejected entry and the key the handler picks its sentence by.
const (
	BrandingRejectMalware        = "nhiem-ma-doc"
	BrandingRejectTypeNotAllowed = "sai-kieu-tep"
	BrandingRejectTypeMismatch   = "khac-kieu-khai-bao"
	BrandingRejectTooLarge       = "vuot-dung-luong"
	BrandingRejectUndecodable    = "khong-doc-duoc-anh"
	BrandingRejectTooManyPixels  = "qua-nhieu-diem-anh"
)

// Soft-delete reasons of a withdrawn image (stored_file.delete_reason; 0017 refuses an empty one).
const (
	deleteReasonReplaced = "da-thay-bang-anh-moi"
	deleteReasonRemoved  = "da-go-khoi-ho-so-hien-thi"
)

var (
	// ErrBrandingNoActor: no staff business code. Refused — an empty "who" never falls back to the
	// internal id (rule 6, invariant 8).
	ErrBrandingNoActor = errors.New("nhận diện xã: thiếu mã cán bộ của người thực hiện")
	// ErrBrandingImageUnknown: neither logo nor banner — a routing bug.
	ErrBrandingImageUnknown = errors.New("nhận diện xã: loại ảnh không hợp lệ")
	// ErrBrandingUploadNotConfigured: object storage, the scanner, or platform's limit is absent. 503.
	ErrBrandingUploadNotConfigured = errors.New("nhận diện xã: chưa cấu hình kho lưu tệp")
	// ErrBrandingLimitsUnavailable: the limits could not be read and no fresh answer is cached. 503.
	ErrBrandingLimitsUnavailable = errors.New("nhận diện xã: chưa đọc được giới hạn tải tệp")
	// ErrBrandingScanUnavailable: clamd could not scan. The row stays `pending`; the file is NEVER
	// stored unscanned (ADR 0052 §9). 503.
	ErrBrandingScanUnavailable = errors.New("nhận diện xã: chưa quét được mã độc")
	// ErrBrandingTypeNotAllowed / ErrBrandingTooLarge: the DECLARATION is outside the policy. 400.
	ErrBrandingTypeNotAllowed = errors.New("nhận diện xã: loại tệp không được phép")
	ErrBrandingTooLarge       = errors.New("nhận diện xã: tệp vượt dung lượng cho phép")
	// ErrBrandingCountReached: platform's `max_files_per_subject` for this purpose is reached. 409.
	ErrBrandingCountReached = errors.New("nhận diện xã: đã đủ số tệp tối đa")
	// ErrBrandingFileNotFound: no such upload FOR THIS CALLER — unknown, another commune's, another
	// officer's, the other image's. One answer for all (rule 4, forbidden #2, applied to staff). 404.
	ErrBrandingFileNotFound = errors.New("nhận diện xã: không tìm thấy tệp đã tải lên")
	// ErrBrandingNotPending: the row left `pending` under the completion (a concurrent writer). 409.
	ErrBrandingNotPending = errors.New("nhận diện xã: tệp không còn chờ hoàn tất")
	// ErrBrandingUploadChanged: the temp object changed between two reads of the inspection (its ETag
	// no longer matched). 409.
	ErrBrandingUploadChanged = errors.New("nhận diện xã: tệp vừa bị thay đổi trong lúc kiểm tra")
	// ErrBrandingPublishUnavailable: the object store refused or could not be reached while publishing.
	// NOTHING was written; the profile is as it was. 503.
	ErrBrandingPublishUnavailable = errors.New("nhận diện xã: chưa đăng được ảnh lên kho công khai")
	// ErrBrandingDecodeBusy: the decode slot stayed taken for brandingDecodeWait. Nothing written past
	// the pending row; the officer uploads again. 503.
	ErrBrandingDecodeBusy = errors.New("nhận diện xã: máy chủ đang bận xử lý ảnh khác")
	// ErrBrandingRejected is what every *BrandingRejection matches.
	ErrBrandingRejected = errors.New("nhận diện xã: tệp bị từ chối")
)

// BrandingRejection is a completion that refused the FILE: the row is `rejected` (or `failed` when the
// image could not be re-encoded), the temp object deleted or expiring within a day, the trail says why.
type BrandingRejection struct{ Reason string }

func (e *BrandingRejection) Error() string   { return ErrBrandingRejected.Error() + ": " + e.Reason }
func (e *BrandingRejection) Is(t error) bool { return t == ErrBrandingRejected }

// THE DECODE BOUNDS — vendor bounds against decompression bombs and out-of-memory, not customer numbers.
//
// A 2 MB upload (the policy) can still DECLARE 30 000 × 30 000 pixels. Two checks, both from the header
// before any pixel is decoded:
//
//	brandingMaxSide / brandingMaxPixels  8192 px per side, 32 megapixels — far above any logo or banner a
//	                                     commune would draw (the output is 512 px or 1600 px), so a refusal
//	                                     costs a legitimate upload nothing but a resize
//	brandingDecodeBudget                 96 MiB ESTIMATED by imaging.DecodeCost — the platform pod runs at a
//	                                     384 MiB limit (deploy/base/platform/deployment.yaml) and also serves
//	                                     every commune's Host resolution, so it gets a smaller share than
//	                                     comms' 160 MiB. Raising it needs the pod limit raised with it
//
// decodeSlot admits ONE decode at a time in this process, so two budgets never stack.
const (
	brandingMaxSide      = 8192
	brandingMaxPixels    = 32_000_000
	brandingDecodeBudget = 96 << 20
	brandingHeadBytes    = imaging.HeadBytes

	// brandingMaxOriginalBytes bounds the in-memory copy of one original — 4x the 2 MB policy, so a
	// policy raised later is not silently truncated, and a value far above it is refused as too large.
	brandingMaxOriginalBytes = 8 << 20
	// brandingDecodeWait: how long a completion queues for the decode slot before answering 503.
	brandingDecodeWait = 10 * time.Second
)

// The normalised derivatives (ADR 0069 #4, #5). Variants are core/storage's `thumb-<width>` family.
const (
	LogoDerivativeVariant   = "thumb-512"
	logoSide                = 512
	BannerDerivativeVariant = "thumb-1600"
	bannerWidth             = 1600
	// bannerMaxHeight: the ceiling RenderJPEGWidth fits inside when width-scaling would make a TALL
	// image taller than this. No aspect rule is invented (ADR 0069 states none); this only stops an
	// unbounded strip.
	bannerMaxHeight   = 1600
	bannerJPEGQuality = 85
)

// BrandingObjectStore is the part of *storage.Client the branding acts call — an interface so the tests
// run with no MinIO. The semantics are core/storage's; read storage.go before implementing another.
type BrandingObjectStore interface {
	PutUpload(ctx context.Context, uploadKey string, r io.Reader, size int64, maxBytes int64,
		contentType string) (storage.ObjectInfo, error)
	Stat(ctx context.Context, b storage.Bucket, key string) (storage.ObjectInfo, error)
	ReadHead(ctx context.Context, b storage.Bucket, key, ifMatchETag string, n int) ([]byte, error)
	Open(ctx context.Context, b storage.Bucket, key, ifMatchETag string) (io.ReadCloser, int64, error)
	SHA256(ctx context.Context, b storage.Bucket, key, ifMatchETag string) (string, error)
	Promote(ctx context.Context, srcUploadKey, ifMatchETag string, dst storage.Key,
		b storage.Bucket) (storage.Promoted, error)
	PutServerProduced(ctx context.Context, dst storage.Key, r io.Reader, size int64) (storage.Produced, error)
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

// BrandingFiles is the part of *store.StoredFileStore the branding acts call.
type BrandingFiles interface {
	InsertPending(ctx context.Context, tx *store.ScopedTx, f domain.StoredFile) error
	ForUpdate(ctx context.Context, tx *store.ScopedTx, id string) (*domain.StoredFile, error)
	ByID(ctx context.Context, id string) (*domain.StoredFile, error)
	Transition(ctx context.Context, tx *store.ScopedTx, id string, from, to domain.StoredFileStatus,
		at time.Time) error
	MarkStored(ctx context.Context, tx *store.ScopedTx, id string, facts domain.StoredFileFacts,
		at time.Time) error
	SetPublicObjectKey(ctx context.Context, tx *store.ScopedTx, id, key string, at time.Time) error
	PublishedOfPurpose(ctx context.Context, tx *store.ScopedTx, purpose string) ([]domain.StoredFile, error)
	SoftDelete(ctx context.Context, tx *store.ScopedTx, id, by, reason string, at time.Time) error
	CountLiveOfPurpose(ctx context.Context, tx *store.ScopedTx, purpose string, pendingSince time.Time) (int, error)
}

// BrandingProfiles is the branding half of *store.HoSoHienThiStore.
type BrandingProfiles interface {
	ProfileForUpdate(ctx context.Context, tx *store.ScopedTx) (domain.ProfileBranding, bool, error)
	EnsureProfile(ctx context.Context, tx *store.ScopedTx, by string, at time.Time) (bool, error)
	SetBrandingFile(ctx context.Context, tx *store.ScopedTx, img domain.BrandingImage, fileID, by string,
		at time.Time) error
	BrandingView(ctx context.Context) (domain.BrandingView, error)
}

// Branding owns the upload, completion, removal and the settings read.
type Branding struct {
	db       *store.DB
	files    BrandingFiles
	profiles BrandingProfiles

	// ANY nil means "not configured" and every upload is refused with ErrBrandingUploadNotConfigured —
	// fail closed, while the rest of the service keeps serving. objects alone also builds the URLs.
	objects  BrandingObjectStore
	scanner  MalwareScanner
	policies UploadPolicies

	log        *slog.Logger
	decodeSlot chan struct{}
	decodeWait time.Duration // brandingDecodeWait; a field so a test need not wait ten seconds

	newID func() (string, error)
	now   func() time.Time
}

// NewBranding builds the use case. Pass UNTYPED nil for a dependency that is not configured — a nil
// *storage.Client inside a non-nil interface would pass the nil check and panic on first use.
func NewBranding(db *store.DB, files BrandingFiles, profiles BrandingProfiles, objects BrandingObjectStore,
	scanner MalwareScanner, policies UploadPolicies, log *slog.Logger) *Branding {
	if log == nil {
		log = slog.Default()
	}
	return &Branding{db: db, files: files, profiles: profiles, objects: objects, scanner: scanner,
		policies: policies, log: log, decodeSlot: make(chan struct{}, 1), decodeWait: brandingDecodeWait,
		newID: storage.NewObjectID}
}

func (uc *Branding) clock() time.Time {
	if uc.now == nil {
		return time.Now().UTC()
	}
	return uc.now().UTC()
}

func (uc *Branding) uploadsConfigured() bool {
	return uc.objects != nil && uc.scanner != nil && uc.policies != nil
}

func (uc *Branding) policy(ctx context.Context, img domain.BrandingImage) (uploadpolicy.Policy, error) {
	p, ok, err := uc.policies.Policy(ctx, storage.Purpose(img))
	switch {
	case errors.Is(err, uploadpolicy.ErrUnavailable):
		return uploadpolicy.Policy{}, fmt.Errorf("%w: %w", ErrBrandingLimitsUnavailable, err)
	case err != nil:
		return uploadpolicy.Policy{}, fmt.Errorf("nhận diện xã: đọc giới hạn tải tệp: %w", err)
	case !ok:
		return uploadpolicy.Policy{}, fmt.Errorf("%w: no upload policy for %s", ErrBrandingUploadNotConfigured, img)
	}
	return p, nil
}

func checkActor(img domain.BrandingImage, actor audit.Actor) error {
	if !img.Valid() {
		return ErrBrandingImageUnknown
	}
	if actor.ID == "" {
		return ErrBrandingNoActor
	}
	return nil
}

// --- the settings read --------------------------------------------------------------------------------

// BrandingSettings is what the Cấu hình tab shows. URLs are "" when the image is not set — or when the
// object store is not configured: no URL is ever guessed.
type BrandingSettings struct {
	LogoPublicURL           string
	WebAdminBannerPublicURL string
	UpdatedAt               time.Time // zero when the commune has no profile row
	UpdatedBy               string    // CB- code; "" when no profile row
}

// Settings reads the current images. NOT AUDITED: a member of staff, in their own commune, reading two
// public URLs and a staff code — no personal data, nothing that is not already public.
func (uc *Branding) Settings(ctx context.Context) (BrandingSettings, error) {
	v, err := uc.profiles.BrandingView(ctx)
	if err != nil {
		return BrandingSettings{}, err
	}
	return BrandingSettings{
		LogoPublicURL:           uc.PublicURL(v.LogoPublicKey),
		WebAdminBannerPublicURL: uc.PublicURL(v.BannerPublicKey),
		UpdatedAt:               v.UpdatedAt,
		UpdatedBy:               v.UpdatedBy,
	}, nil
}

// PublicURL turns a public-bucket key into the anonymous URL, or "" — an empty key, no object store, or
// a key storage refuses. "" is the contract's "not set" (platform.proto TenantProfile): a broken link is
// never the safe failure.
func (uc *Branding) PublicURL(key string) string {
	if key == "" || uc.objects == nil {
		return ""
	}
	u, err := uc.objects.PublicURL(key)
	if err != nil {
		return ""
	}
	return u
}

// --- a + b. one upload: open the row, stream the file in, complete ------------------------------------

// BrandingUploadRequest is one upload as the handler received it: the declaration, read from the
// multipart text fields and the file part's header, and the file itself as a stream. The declaration is
// checked before a byte of the stream is read; the bytes are checked again by the completion — a claim
// is never the fact (ADR 0052 §1c).
type BrandingUploadRequest struct {
	FileName    string
	ContentType string // DECLARED; the key's extension follows it, the completion sniffs the truth
	Size        int64  // DECLARED byte count; Body must yield exactly this many
	// Body is the file. NEVER BUFFERED here: it is piped into the temp bucket (core/storage PutUpload).
	Body io.Reader
	// Deadline bounds the write of Body (httpx.UploadRequest.Deadline). Zero = only ctx bounds it. The
	// completion that follows is NOT under it — see Upload.
	Deadline time.Time
	// Received confirms the transport delivered the whole file and nothing after it
	// (httpx.UploadRequest.Finish). Called after the write succeeds and before anything is recorded
	// about the bytes. nil = nothing to confirm.
	Received func() error
}

// MaxUploadBytes is the size cap of one image under platform's CURRENT policy — what the handler bounds
// the request body by before it reads it. Not configured / unreadable → the same refusals as Upload.
func (uc *Branding) MaxUploadBytes(ctx context.Context, img domain.BrandingImage) (int64, error) {
	if !img.Valid() {
		return 0, ErrBrandingImageUnknown
	}
	if !uc.uploadsConfigured() {
		return 0, ErrBrandingUploadNotConfigured
	}
	pol, err := uc.policy(ctx, img)
	if err != nil {
		return 0, err
	}
	if pol.MaxBytes <= 0 {
		// uploadpolicy never answers this; a zero cap would refuse every file, so it is "not configured".
		return 0, fmt.Errorf("%w: policy %s has no size cap", ErrBrandingUploadNotConfigured, img)
	}
	return pol.MaxBytes, nil
}

// Upload is the whole of ADR 0052 §Sửa đổi 09/10/2026 for one image, in the request that carries it:
//
//	a. open      one transaction: profile row locked, count, pending row, audit entry — nothing read yet
//	b. receive   Body streamed into the temp bucket under the row's upload key, bounded by Deadline;
//	             then Received. Either fails → the row moves to `failed` (audited) and the error returns
//	c. complete  the completion that used to be its own route, unchanged
//
// THE DEADLINE BOUNDS STEP b ONLY. A client may use most of its 180 s sending the file; putting the scan
// and the publication under what is left would turn a slow connection into a half-done completion.
// Step c is bounded by its own waits (decode slot, scanner, store) and the request's context.
//
// A step-c failure that is "not now" (scanner, decode slot, store) leaves the row `pending`, as it always
// did; there is no completion route left to retry it, so the officer uploads again and the abandoned row
// stops counting against the file limit after storage.UploadTTL. Every error after step a names the file
// id, so the refusal log line can be matched to the row.
func (uc *Branding) Upload(ctx context.Context, img domain.BrandingImage, req BrandingUploadRequest,
	actor audit.Actor) (domain.StoredFile, error) {

	f, uploadKey, pol, err := uc.open(ctx, img, req, actor)
	if err != nil {
		return domain.StoredFile{}, err
	}
	if err := uc.receive(ctx, req, uploadKey, pol.MaxBytes); err != nil {
		uc.failUnreceived(ctx, img, f.ID, uploadKey, actor)
		return domain.StoredFile{}, fmt.Errorf("nhận diện xã: tệp %s: %w", f.ID, err)
	}
	done, err := uc.complete(ctx, img, f.ID, actor)
	if err != nil {
		return domain.StoredFile{}, fmt.Errorf("nhận diện xã: tệp %s: %w", f.ID, err)
	}
	return done, nil
}

// open is step a (formerly RequestUpload, ADR 0052 §1a, without the presigned form). ONE TRANSACTION:
// the profile row locked when it exists (a soft-deleted one refuses here, before a byte is read), the
// count, the pending row, the audit entry.
func (uc *Branding) open(ctx context.Context, img domain.BrandingImage, req BrandingUploadRequest,
	actor audit.Actor) (domain.StoredFile, string, uploadpolicy.Policy, error) {

	fail := func(err error) (domain.StoredFile, string, uploadpolicy.Policy, error) {
		return domain.StoredFile{}, "", uploadpolicy.Policy{}, err
	}
	if err := checkActor(img, actor); err != nil {
		return fail(err)
	}
	if req.Body == nil {
		return fail(errors.New("nhận diện xã: lượt tải không có luồng tệp — lỗi nối dây ở handler"))
	}
	name, err := domain.CleanFileName(req.FileName)
	if err != nil {
		return fail(err)
	}
	if req.Size <= 0 {
		return fail(domain.ErrBrandingSizeInvalid)
	}
	if !uc.uploadsConfigured() {
		return fail(ErrBrandingUploadNotConfigured)
	}
	pol, err := uc.policy(ctx, img)
	if err != nil {
		return fail(err)
	}
	if !pol.AllowsMIME(req.ContentType) {
		return fail(ErrBrandingTypeNotAllowed)
	}
	if req.Size > pol.MaxBytes {
		return fail(ErrBrandingTooLarge)
	}
	ext, ok := storage.ExtForMIME(req.ContentType)
	if !ok {
		return fail(ErrBrandingTypeNotAllowed) // unreachable while uploadpolicy narrows to storage's list
	}

	id, err := uc.newID()
	if err != nil {
		return fail(fmt.Errorf("nhận diện xã: sinh mã tệp: %w", err))
	}
	now := uc.clock()
	key := storage.Key{
		Class: storage.ClassContentSource, TenantID: string(tenant.MustFrom(ctx)), CreatedAt: now,
		Service: storage.ServicePlatform, Purpose: storage.Purpose(img),
		ObjectID: id, Variant: storage.VariantOriginal, Ext: ext,
	}
	objectKey, err := key.Path()
	if err != nil {
		return fail(fmt.Errorf("nhận diện xã: dựng khoá đối tượng: %w", err))
	}
	uploadKey, err := key.UploadPath()
	if err != nil {
		return fail(fmt.Errorf("nhận diện xã: dựng khoá tải lên: %w", err))
	}

	var out domain.StoredFile
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		p, found, err := uc.profiles.ProfileForUpdate(ctx, tx)
		if err != nil {
			return err
		}
		if found && p.Deleted {
			return domain.ErrProfileDeleted
		}
		if pol.FileCountLimited {
			live, err := uc.files.CountLiveOfPurpose(ctx, tx, string(img), now.Add(-storage.UploadTTL))
			if err != nil {
				return err
			}
			if live >= pol.MaxFilesPerSubject {
				return ErrBrandingCountReached
			}
		}
		f := domain.StoredFile{
			ID: id, Bucket: domain.StoredFileBucketPrivate, ObjectKey: objectKey,
			RetentionClass: string(storage.ClassContentSource), Purpose: string(img),
			SubjectType: domain.StoredFileSubjectDisplayProfile, SubjectID: string(tx.TenantID()),
			OriginalName: name, Status: domain.StoredFilePending, UploadedBy: actor.ID,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := uc.files.InsertPending(ctx, tx, f); err != nil {
			return err
		}
		if err := writeBrandingAudit(ctx, tx, actor, ActionBrandingUploadRequested, img, now, map[string]any{
			"tep_id":          id,
			"muc_dich":        string(img),
			"loai_khai_bao":   req.ContentType,
			"kich_thuoc_khai": req.Size,
		}); err != nil {
			return err
		}
		out = f
		return nil
	})
	if err != nil {
		return fail(err)
	}
	return out, uploadKey, pol, nil
}

// receive is step b: the stream into the temp bucket, then the transport's own confirmation. Errors keep
// their sentinels (httpx's upload errors come back wrapped through PutUpload; storage's are its own), so
// the handler maps a client that stopped sending to a 4xx and a store that refused to a 5xx.
func (uc *Branding) receive(ctx context.Context, req BrandingUploadRequest, uploadKey string, maxBytes int64) error {
	putCtx := ctx
	if !req.Deadline.IsZero() {
		var cancel context.CancelFunc
		putCtx, cancel = context.WithDeadline(ctx, req.Deadline)
		defer cancel()
	}
	if _, err := uc.objects.PutUpload(putCtx, uploadKey, req.Body, req.Size, maxBytes, req.ContentType); err != nil {
		return brandingStorageErr("ghi tệp vào kho tạm", err)
	}
	if req.Received != nil {
		if err := req.Received(); err != nil {
			return fmt.Errorf("nhận diện xã: xác nhận đã nhận đủ tệp: %w", err)
		}
	}
	return nil
}

// failUnreceived closes a row whose file never arrived whole: the temp object (if a write left one) is
// purged, and the row moves pending → failed with its audit entry, in one transaction (rule 6 inv 3).
//
// ON A DETACHED CONTEXT: the usual cause is the client hanging up, which has already cancelled the
// request's. Best effort — a failure is logged and the row stays `pending`, which stops counting against
// the file limit after storage.UploadTTL; the temp lifecycle removes any object within a day.
func (uc *Branding) failUnreceived(ctx context.Context, img domain.BrandingImage, id, uploadKey string,
	actor audit.Actor) {

	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), detachedTimeout)
	defer cancel()
	removed := uc.objects.PurgeAllVersions(ctx, storage.BucketTemp, uploadKey) == nil
	now := uc.clock()
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		if err := uc.files.Transition(ctx, tx, id, domain.StoredFilePending, domain.StoredFileFailed, now); err != nil {
			return err
		}
		return writeBrandingAudit(ctx, tx, actor, ActionBrandingUploadFailed, img, now, map[string]any{
			"tep_id": id, "da_xoa_tep_tam": removed,
		})
	})
	if err != nil {
		uc.log.Warn("nhận diện xã: tải tệp không thành, chưa chuyển được dòng sang failed — dòng giữ pending",
			"xa", string(tenant.MustFrom(ctx)), "tep_id", id, "err", err)
	}
}

// --- c. complete an upload: inspect, derive, publish, point -------------------------------------------

type brandingOutcome int

const (
	brandingReady brandingOutcome = iota + 1
	brandingRejectedFile
	brandingFailed // stored, but no derivative can be made: `processing → failed`
)

type brandingInspection struct {
	kind        brandingOutcome
	facts       domain.StoredFileFacts
	recovered   bool
	reason      string
	signature   string
	tempRemoved bool
}

// complete is step c — ADR 0052 §1c, the derivative, the publication and the profile pointer — for the
// upload Upload has just written to the temp bucket. Only the officer it was issued to, only for the
// image it was issued for: the id never comes off the wire, but the checks stay, because they are what
// stands between two concurrent writers of the same row.
//
// IDEMPOTENT: a file already `ready` is returned as it is and nothing is written — `ready` is reached
// only by the transaction that also published it and pointed the profile at it.
func (uc *Branding) complete(ctx context.Context, img domain.BrandingImage, id string, actor audit.Actor) (
	domain.StoredFile, error) {

	if err := checkActor(img, actor); err != nil {
		return domain.StoredFile{}, err
	}
	if id == "" || len(id) > domain.MaxFileIDLen {
		return domain.StoredFile{}, ErrBrandingFileNotFound
	}
	if !uc.uploadsConfigured() {
		return domain.StoredFile{}, ErrBrandingUploadNotConfigured
	}

	f, err := uc.files.ByID(ctx, id)
	if err != nil {
		return domain.StoredFile{}, err
	}
	if !ownBrandingUpload(f, img, actor.ID) {
		return domain.StoredFile{}, ErrBrandingFileNotFound
	}
	switch f.Status {
	case domain.StoredFileReady:
		return *f, nil
	case domain.StoredFilePending, domain.StoredFileStored, domain.StoredFileProcessing:
	default:
		return domain.StoredFile{}, ErrBrandingNotPending
	}
	pol, err := uc.policy(ctx, img)
	if err != nil {
		return domain.StoredFile{}, err
	}
	key, err := storage.ParseKey(f.ObjectKey)
	if err != nil {
		return domain.StoredFile{}, fmt.Errorf("nhận diện xã: khoá đối tượng đã ghi không hợp lệ: %w", err)
	}

	insp, err := uc.inspect(ctx, img, *f, key, pol)
	if err != nil {
		return domain.StoredFile{}, err
	}

	now := uc.clock()
	var (
		done     domain.StoredFile
		copied   bool
		withdraw []domain.StoredFile
	)
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		cur, err := uc.files.ForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if !ownBrandingUpload(cur, img, actor.ID) {
			return ErrBrandingFileNotFound
		}
		if cur.Status == domain.StoredFileReady {
			done = *cur // another completion finished first: same outcome, nothing to write
			return nil
		}
		switch insp.kind {
		case brandingReady:
			// THE PROFILE FIRST: a retired profile refuses before the row moves, and the row lock
			// serialises two completions of two different uploads of the same image.
			prof, err := uc.lockProfile(ctx, tx, actor, now)
			if err != nil {
				return err
			}
			final, err := uc.walkToEnd(ctx, tx, *cur, insp, now)
			if err != nil {
				return err
			}
			pubPath, err := uc.publish(ctx, tx, img, final, now)
			copied = pubPath != "" || copied
			if err != nil {
				return err
			}
			before := prof.FileID(img)
			if err := uc.profiles.SetBrandingFile(ctx, tx, img, id, actor.ID, now); err != nil {
				return err
			}
			if withdraw, err = uc.othersPublished(ctx, tx, img, id); err != nil {
				return err
			}
			final.PublicObjectKey = pubPath
			done = final
			return writeBrandingAudit(ctx, tx, actor, ActionBrandingSet, img, now, map[string]any{
				"truoc":    map[string]any{"tep_id": before},
				"sau":      map[string]any{"tep_id": id},
				"loai_tep": insp.facts.MIMEType, "kich_thuoc": insp.facts.SizeBytes, "sha256": insp.facts.SHA256,
				"ban_dan_xuat": derivativeVariant(img), "khoi_phuc_tu_dich": insp.recovered,
				"tao_ho_so": prof.created,
			})
		case brandingFailed:
			if _, err := uc.walkToEnd(ctx, tx, *cur, insp, now); err != nil {
				return err
			}
			return writeBrandingAudit(ctx, tx, actor, ActionBrandingRejected, img, now, map[string]any{
				"tep_id": id, "ly_do": insp.reason,
			})
		case brandingRejectedFile:
			if cur.Status != domain.StoredFilePending {
				return ErrBrandingNotPending
			}
			d := map[string]any{"tep_id": id, "ly_do": insp.reason, "da_xoa_tep_tam": insp.tempRemoved}
			if insp.signature != "" {
				d["chu_ky_ma_doc"] = insp.signature
			}
			if err := uc.files.Transition(ctx, tx, id, domain.StoredFilePending, domain.StoredFileRejected, now); err != nil {
				return err
			}
			return writeBrandingAudit(ctx, tx, actor, ActionBrandingRejected, img, now, d)
		}
		return fmt.Errorf("nhận diện xã: kết quả kiểm tra không rõ (%d)", insp.kind)
	})
	if err != nil {
		if copied {
			uc.undoPublish(ctx, img, id)
		}
		return domain.StoredFile{}, err
	}
	uc.withdrawAfterCommit(ctx, img, withdraw, actor, deleteReasonReplaced)
	if done.ID != "" {
		return done, nil
	}
	return domain.StoredFile{}, &BrandingRejection{Reason: insp.reason}
}

// lockedProfile is the profile row as a write saw it, and whether this write created it.
type lockedProfile struct {
	domain.ProfileBranding
	created bool
}

// lockProfile locks the commune's profile row, creating it first when the commune has none (ADR 0069:
// a commune may set its logo before anybody filled in its address). A SOFT-DELETED row refuses
// (domain.ErrProfileDeleted) — never revived, never a second row.
//
// The read is repeated after the insert so the lock is held on the row this transaction will update, and
// a concurrent creator (whose INSERT won) is seen as it committed rather than as the empty row assumed.
func (uc *Branding) lockProfile(ctx context.Context, tx *store.ScopedTx, actor audit.Actor,
	now time.Time) (lockedProfile, error) {

	p, found, err := uc.profiles.ProfileForUpdate(ctx, tx)
	if err != nil {
		return lockedProfile{}, err
	}
	created := false
	if !found {
		if created, err = uc.profiles.EnsureProfile(ctx, tx, actor.ID, now); err != nil {
			return lockedProfile{}, err
		}
		if p, found, err = uc.profiles.ProfileForUpdate(ctx, tx); err != nil {
			return lockedProfile{}, err
		}
		if !found {
			return lockedProfile{}, errors.New("nhận diện xã: tạo dòng hồ sơ xong mà không đọc lại được")
		}
	}
	if p.Deleted {
		return lockedProfile{}, domain.ErrProfileDeleted
	}
	return lockedProfile{ProfileBranding: p, created: created}, nil
}

// publish copies the derivative of a `ready` file to the public bucket and records its key. Returns the
// public key path once the COPY has run (so a rollback after it knows to undo it), even with an error.
func (uc *Branding) publish(ctx context.Context, tx *store.ScopedTx, img domain.BrandingImage,
	f domain.StoredFile, now time.Time) (string, error) {

	orig, err := storage.ParseKey(f.ObjectKey)
	if err != nil {
		return "", fmt.Errorf("nhận diện xã: khoá đối tượng đã ghi không hợp lệ: %w", err)
	}
	pub := publicKey(img, orig)
	pubPath, err := pub.Path()
	if err != nil {
		return "", fmt.Errorf("nhận diện xã: dựng khoá công khai: %w", err)
	}
	if err := uc.objects.PublishDerivative(ctx, derivativeKey(img, orig), pub); err != nil {
		return "", fmt.Errorf("%w: %w", ErrBrandingPublishUnavailable, err)
	}
	if err := uc.files.SetPublicObjectKey(ctx, tx, f.ID, pubPath, now); err != nil {
		return pubPath, err
	}
	return pubPath, nil
}

// othersPublished lists every published file of this image other than keep — the ones a set or a removal
// withdraws after commit. Locked, so two writes of the same image never withdraw each other's file.
func (uc *Branding) othersPublished(ctx context.Context, tx *store.ScopedTx, img domain.BrandingImage,
	keep string) ([]domain.StoredFile, error) {

	public, err := uc.files.PublishedOfPurpose(ctx, tx, string(img))
	if err != nil {
		return nil, err
	}
	var out []domain.StoredFile
	for _, f := range public {
		if f.ID != keep {
			out = append(out, f)
		}
	}
	return out, nil
}

// walkToEnd moves a row along pending → scanning → stored → processing → ready (or → failed), from
// wherever it stands, one guarded edge per UPDATE so stored_file_guard sees every step.
func (uc *Branding) walkToEnd(ctx context.Context, tx *store.ScopedTx, cur domain.StoredFile,
	insp brandingInspection, now time.Time) (domain.StoredFile, error) {

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
		return domain.StoredFile{}, ErrBrandingNotPending
	}
	to := domain.StoredFileReady
	if insp.kind == brandingFailed {
		to = domain.StoredFileFailed
	}
	if err := uc.files.Transition(ctx, tx, cur.ID, domain.StoredFileProcessing, to, now); err != nil {
		return domain.StoredFile{}, err
	}
	cur.Status, cur.UpdatedAt = to, now
	return cur, nil
}

// ownBrandingUpload: the row exists, is this commune's display-profile file of THIS image, and was issued
// to THIS officer. Another officer's upload is "not found" — the same answer as a nonexistent id.
func ownBrandingUpload(f *domain.StoredFile, img domain.BrandingImage, officer string) bool {
	return f != nil && officer != "" && f.SubjectType == domain.StoredFileSubjectDisplayProfile &&
		f.Purpose == string(img) && f.UploadedBy == officer
}

// --- d. remove ----------------------------------------------------------------------------------------

// Remove clears one image (ADR 0069 #7: no logo → the building icon; no banner → no strip). ONE
// TRANSACTION: the profile row locked, the column set NULL, the audit entry; then, after commit, the old
// public copy is withdrawn and its row soft-deleted (never a hard delete, rule 7).
//
// IDEMPOTENT: an image that is not set writes nothing and answers nil (removed=false). It still retries
// any withdrawal an earlier write left behind — the marker is the key still set on a non-current file.
func (uc *Branding) Remove(ctx context.Context, img domain.BrandingImage, actor audit.Actor) (bool, error) {
	if err := checkActor(img, actor); err != nil {
		return false, err
	}
	now := uc.clock()
	var (
		removed  bool
		withdraw []domain.StoredFile
	)
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		p, found, err := uc.profiles.ProfileForUpdate(ctx, tx)
		if err != nil {
			return err
		}
		if found && p.Deleted {
			return domain.ErrProfileDeleted
		}
		if withdraw, err = uc.othersPublished(ctx, tx, img, ""); err != nil {
			return err
		}
		before := ""
		if found {
			before = p.FileID(img)
		}
		if before == "" {
			return nil // nothing set: nothing written, nothing audited
		}
		if err := uc.profiles.SetBrandingFile(ctx, tx, img, "", actor.ID, now); err != nil {
			return err
		}
		removed = true
		return writeBrandingAudit(ctx, tx, actor, ActionBrandingRemoved, img, now, map[string]any{
			"truoc": map[string]any{"tep_id": before},
			"sau":   map[string]any{"tep_id": ""},
		})
	})
	if err != nil {
		return false, err
	}
	if uc.objects == nil {
		// The column is cleared — the image is no longer CURRENT anywhere. The public copy stays until a
		// later write with storage configured withdraws it (its key is the durable marker).
		if len(withdraw) > 0 {
			uc.log.Warn("nhận diện xã: chưa cấu hình kho lưu tệp — bản công khai cũ CHƯA được gỡ",
				"xa", string(tenant.MustFrom(ctx)), "so_tep", len(withdraw))
		}
		return removed, nil
	}
	uc.withdrawAfterCommit(ctx, img, withdraw, actor, deleteReasonRemoved)
	return removed, nil
}

// --- compensation and withdrawal ----------------------------------------------------------------------

// detachedTimeout bounds the storage work done after the request's own transaction: it runs on a context
// the client's disconnect cannot cancel, because a withdrawal abandoned half-way is exactly the state the
// key marker exists to recover from — better not to create it for a closed browser tab.
const detachedTimeout = 30 * time.Second

// undoPublish is the compensation of a copy whose transaction did not commit.
//
// THE DECISION IS TAKEN UNDER THE ROW LOCK, and the unpublish runs while it is held. Two completions of
// the SAME upload copy to the SAME public twin (PublishDerivative is idempotent), so a lock-free "is the
// key recorded?" read could run before a concurrent completion commits its key — and this function would
// then delete the object that completion just made current. Holding FOR UPDATE makes the other
// completion either already committed (key visible: leave the copy) or still waiting at its own
// ForUpdate, before its PublishDerivative (it will copy again after we release). The lock is held for
// one object delete, bounded by detachedTimeout; the transaction writes nothing.
func (uc *Branding) undoPublish(ctx context.Context, img domain.BrandingImage, fileID string) {
	if uc.objects == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), detachedTimeout)
	defer cancel()
	var pub string
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		f, err := uc.files.ForUpdate(ctx, tx, fileID)
		if err != nil || f == nil || f.PublicObjectKey != "" {
			return err // a recorded key means a committed publish owns that copy — never withdraw it here
		}
		orig, err := storage.ParseKey(f.ObjectKey)
		if err != nil {
			return err
		}
		pub, _ = publicKey(img, orig).Path()
		return uc.objects.UnpublishDerivative(ctx, publicKey(img, orig))
	})
	if err != nil {
		uc.log.Warn("nhận diện xã: giao dịch không chốt, gỡ bản công khai vừa chép cũng hỏng — bản sao mồ côi",
			"xa", string(tenant.MustFrom(ctx)), "khoa_cong_khai", pub, "err", err)
	}
}

// withdrawAfterCommit removes the public copies a write listed, then — one short transaction per file —
// clears the key, soft-deletes the row and audits it. Failures are logged by object key (no personal
// data in a key, ADR 0052 §3) and left for the next write of this image to retry.
func (uc *Branding) withdrawAfterCommit(ctx context.Context, img domain.BrandingImage,
	files []domain.StoredFile, actor audit.Actor, reason string) {

	if len(files) == 0 || uc.objects == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), detachedTimeout)
	defer cancel()
	for _, f := range files {
		key, err := storage.ParseKey(f.PublicObjectKey)
		if err == nil {
			err = uc.objects.UnpublishDerivative(ctx, key)
		}
		if err != nil {
			uc.log.Warn("nhận diện xã: gỡ bản công khai cũ hỏng — giữ khoá để lần ghi sau thử lại",
				"xa", string(tenant.MustFrom(ctx)), "khoa_cong_khai", f.PublicObjectKey, "err", err)
			continue
		}
		at := uc.clock()
		err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
			if err := uc.files.SetPublicObjectKey(ctx, tx, f.ID, "", at); err != nil {
				return err
			}
			if err := uc.files.SoftDelete(ctx, tx, f.ID, actor.ID, reason, at); err != nil {
				return err
			}
			return writeBrandingAudit(ctx, tx, actor, ActionBrandingWithdrawn, img, at, map[string]any{
				"tep_id": f.ID, "ly_do_xoa_mem": reason,
			})
		})
		if err != nil {
			uc.log.Warn("nhận diện xã: đã gỡ bản công khai cũ nhưng chưa ghi được — lần ghi sau ghi lại",
				"xa", string(tenant.MustFrom(ctx)), "khoa_cong_khai", f.PublicObjectKey, "err", err)
		}
	}
}

// --- inspection (lock-free) ---------------------------------------------------------------------------

// inspect is the lock-free half of complete. An error means nothing may be decided yet (scanner down,
// object replaced mid-inspection, store failure): nothing is written and the row stays `pending`.
func (uc *Branding) inspect(ctx context.Context, img domain.BrandingImage, f domain.StoredFile,
	key storage.Key, pol uploadpolicy.Policy) (brandingInspection, error) {

	if f.Status != domain.StoredFilePending {
		// stored / processing: the original is already in the private bucket and was scanned before it
		// got there. Only the derivative is owed.
		return uc.fromDestination(ctx, img, f, key)
	}
	uploadKey, err := key.UploadPath()
	if err != nil {
		return brandingInspection{}, fmt.Errorf("nhận diện xã: dựng khoá tải lên: %w", err)
	}
	st, err := uc.objects.Stat(ctx, storage.BucketTemp, uploadKey)
	if errors.Is(err, storage.ErrNotFound) {
		return uc.fromDestination(ctx, img, f, key)
	}
	if err != nil {
		return brandingInspection{}, brandingStorageErr("đọc thông tin tệp tạm", err)
	}

	// The CURRENT policy decides: a limit tightened since the request rejects here.
	if st.Size <= 0 || st.Size > pol.MaxBytes {
		return uc.reject(ctx, uploadKey, BrandingRejectTooLarge, ""), nil
	}
	head, err := uc.objects.ReadHead(ctx, storage.BucketTemp, uploadKey, st.ETag, storage.SniffBytes)
	if err != nil {
		return brandingInspection{}, brandingStorageErr("đọc đầu tệp", err)
	}
	mime, ext, ok := storage.SniffMIME(head)
	if !ok || !pol.AllowsMIME(mime) {
		return uc.reject(ctx, uploadKey, BrandingRejectTypeNotAllowed, ""), nil
	}
	if ext != key.Ext {
		return uc.reject(ctx, uploadKey, BrandingRejectTypeMismatch, ""), nil
	}

	rc, size, err := uc.objects.Open(ctx, storage.BucketTemp, uploadKey, st.ETag)
	if err != nil {
		return brandingInspection{}, brandingStorageErr("mở tệp để quét", err)
	}
	res, err := uc.scanner.Scan(ctx, rc, size)
	closeErr := rc.Close()
	if err != nil {
		if errors.Is(err, malwarescan.ErrNotConfigured) {
			return brandingInspection{}, fmt.Errorf("%w: %w", ErrBrandingUploadNotConfigured, err)
		}
		if errors.Is(err, storage.ErrChanged) {
			return brandingInspection{}, fmt.Errorf("%w: %w", ErrBrandingUploadChanged, err)
		}
		// UNSCANNABLE IS NEVER CLEAN: unreachable, timed out, over clamd's limit, an unknown reply.
		return brandingInspection{}, fmt.Errorf("%w: %w", ErrBrandingScanUnavailable, err)
	}
	if closeErr != nil {
		return brandingInspection{}, brandingStorageErr("đóng tệp sau khi quét", closeErr)
	}
	if !res.Clean {
		return uc.reject(ctx, uploadKey, BrandingRejectMalware, res.Signature), nil
	}

	sum, err := uc.objects.SHA256(ctx, storage.BucketTemp, uploadKey, st.ETag)
	if err != nil {
		return brandingInspection{}, brandingStorageErr("tính sha256", err)
	}
	pr, err := uc.objects.Promote(ctx, uploadKey, st.ETag, key, storage.BucketPrivate)
	switch {
	case err == nil, errors.Is(err, storage.ErrTempCleanup):
		// ErrTempCleanup: promoted; the leftover in temp expires with the bucket's 1-day lifecycle.
	case errors.Is(err, storage.ErrExists), errors.Is(err, storage.ErrNotFound):
		return uc.fromDestination(ctx, img, f, key) // another completion promoted it first
	default:
		return brandingInspection{}, brandingStorageErr("chép tệp sang kho lưu", err)
	}
	facts := domain.StoredFileFacts{MIMEType: pr.ContentType, SizeBytes: st.Size, SHA256: sum}
	return uc.derive(ctx, img, f, key, pr.ETag, facts, false)
}

// fromDestination handles "the original is already in the private bucket" (a concurrent completion
// promoted it first, or a previous one whose transaction or derivative did not finish). A destination
// object is TRUSTED AS SCANNED because nothing else writes there: Promote is the only path into
// `content-source/…/platform/<purpose>/…/original.*`, it runs only after a clean scan, and IAM scopes
// this service's key to its own subtree (ADR 0052 §3).
func (uc *Branding) fromDestination(ctx context.Context, img domain.BrandingImage, f domain.StoredFile,
	key storage.Key) (brandingInspection, error) {

	st, err := uc.objects.Stat(ctx, storage.BucketPrivate, f.ObjectKey)
	if errors.Is(err, storage.ErrNotFound) {
		if f.Status != domain.StoredFilePending {
			return brandingInspection{}, errors.New("nhận diện xã: dòng đã lưu nhưng không thấy bản gốc ở kho lưu")
		}
		// Neither in temp nor promoted, right after this request wrote it: not the client's doing. A
		// temp bucket that drops what was just written, or a read pointed at another store (the 09/10
		// incident shape) — an operator's problem, so a 500 with the cause, never "upload again".
		return brandingInspection{}, fmt.Errorf("nhận diện xã: tệp vừa ghi không có ở kho tạm lẫn kho lưu: %w", err)
	}
	if err != nil {
		return brandingInspection{}, brandingStorageErr("đọc thông tin tệp đã lưu", err)
	}
	facts := domain.StoredFileFacts{MIMEType: f.MIMEType, SizeBytes: f.SizeBytes, SHA256: f.SHA256}
	if f.Status == domain.StoredFilePending {
		head, err := uc.objects.ReadHead(ctx, storage.BucketPrivate, f.ObjectKey, st.ETag, storage.SniffBytes)
		if err != nil {
			return brandingInspection{}, brandingStorageErr("đọc đầu tệp đã lưu", err)
		}
		mime, ext, ok := storage.SniffMIME(head)
		if !ok || ext != key.Ext {
			// Promote refuses exactly this, so it cannot be ours. Not recorded; an operator looks.
			return brandingInspection{}, errors.New("nhận diện xã: đối tượng ở kho lưu không khớp kiểu của khoá")
		}
		sum, err := uc.objects.SHA256(ctx, storage.BucketPrivate, f.ObjectKey, st.ETag)
		if err != nil {
			return brandingInspection{}, brandingStorageErr("tính sha256 tệp đã lưu", err)
		}
		facts = domain.StoredFileFacts{MIMEType: mime, SizeBytes: st.Size, SHA256: sum}
	}
	return uc.derive(ctx, img, f, key, st.ETag, facts, true)
}

// derive produces the normalised derivative from the promoted original and stores it with
// PutServerProduced. An image that cannot be re-encoded is a FILE outcome (brandingFailed, 422); a store
// that cannot be reached is an error (retry).
//
// THE DECODE SLOT IS HELD ONLY AROUND THE CPU WORK (decode + render), never across object-store I/O: the
// original is read into memory first (it is at most the policy's 2 MB, bounded again here by
// brandingMaxOriginalBytes), and the derivative is written after the slot is released. A stalled MinIO
// read therefore never holds the one slot every commune's upload queues on. Acquiring the slot waits at
// most brandingDecodeWait, then answers ErrBrandingDecodeBusy (503, retryable) — nothing written.
func (uc *Branding) derive(ctx context.Context, img domain.BrandingImage, f domain.StoredFile,
	orig storage.Key, etag string, facts domain.StoredFileFacts, recovered bool) (brandingInspection, error) {

	out := brandingInspection{kind: brandingReady, facts: facts, recovered: recovered}
	if facts.SizeBytes > brandingMaxOriginalBytes {
		out.kind, out.reason = brandingFailed, BrandingRejectTooLarge
		return out, nil
	}
	raw, err := uc.readOriginal(ctx, f.ObjectKey, etag)
	if err != nil {
		return brandingInspection{}, err
	}
	head := raw
	if len(head) > brandingHeadBytes {
		head = head[:brandingHeadBytes]
	}
	h, err := imaging.ReadHeader(facts.MIMEType, head)
	if err != nil {
		out.kind, out.reason = brandingFailed, BrandingRejectUndecodable
		return out, nil
	}
	if tooManyPixels(facts.MIMEType, h) {
		out.kind, out.reason = brandingFailed, BrandingRejectTooManyPixels
		return out, nil
	}

	rendered, undecodable, err := uc.decodeAndRender(ctx, img, facts.MIMEType, raw, h.Orientation)
	if err != nil {
		return brandingInspection{}, err
	}
	if undecodable {
		out.kind, out.reason = brandingFailed, BrandingRejectUndecodable
		return out, nil
	}
	_, err = uc.objects.PutServerProduced(ctx, derivativeKey(img, orig), bytes.NewReader(rendered), int64(len(rendered)))
	switch {
	case err == nil, errors.Is(err, storage.ErrExists):
		// ErrExists: a previous completion produced it; dst is derived 1:1 from an immutable original.
	default:
		return brandingInspection{}, brandingStorageErr("ghi bản dẫn xuất", err)
	}
	return out, nil
}

// decodeAndRender is the only code that runs inside decodeSlot: in-memory bytes in, bytes out.
func (uc *Branding) decodeAndRender(ctx context.Context, img domain.BrandingImage, mime string, raw []byte,
	orientation int) (out []byte, undecodable bool, err error) {

	wait, cancel := context.WithTimeout(ctx, uc.decodeWait)
	defer cancel()
	select {
	case uc.decodeSlot <- struct{}{}:
		defer func() { <-uc.decodeSlot }()
	case <-wait.Done():
		if ctx.Err() != nil {
			return nil, false, ctx.Err()
		}
		return nil, false, ErrBrandingDecodeBusy
	}
	decoded, err := imaging.Decode(mime, bytes.NewReader(raw))
	if err != nil {
		return nil, true, nil
	}
	rendered, err := renderDerivative(img, decoded, orientation)
	if err != nil {
		return nil, true, nil
	}
	return rendered, false, nil
}

// readOriginal reads the whole original (bounded by brandingMaxOriginalBytes), bound to its ETag.
func (uc *Branding) readOriginal(ctx context.Context, key, etag string) ([]byte, error) {
	rc, _, err := uc.objects.Open(ctx, storage.BucketPrivate, key, etag)
	if err != nil {
		return nil, brandingStorageErr("mở bản gốc", err)
	}
	raw, err := io.ReadAll(io.LimitReader(rc, brandingMaxOriginalBytes+1))
	closeErr := rc.Close()
	if err != nil {
		return nil, brandingStorageErr("đọc bản gốc", err)
	}
	if closeErr != nil {
		return nil, brandingStorageErr("đóng bản gốc", closeErr)
	}
	if int64(len(raw)) > brandingMaxOriginalBytes {
		return nil, errors.New("nhận diện xã: bản gốc lớn hơn kích thước đã đo")
	}
	return raw, nil
}

// tooManyPixels applies the three decode bounds (see brandingDecodeBudget) to a header.
func tooManyPixels(mime string, h imaging.Header) bool {
	w, ht := int64(h.Config.Width), int64(h.Config.Height)
	return w > brandingMaxSide || ht > brandingMaxSide || w*ht > brandingMaxPixels ||
		imaging.DecodeCost(mime, h) > brandingDecodeBudget
}

// renderDerivative is the per-image normalisation: logo → 512 px square PNG keeping alpha; banner →
// 1600 px wide JPEG on white (no transparency needed on a strip, and JPEG keeps a photo banner small).
func renderDerivative(img domain.BrandingImage, src image.Image, orientation int) ([]byte, error) {
	if img == domain.BrandingLogo {
		return imaging.RenderPNGSquare(src, orientation, logoSide)
	}
	return imaging.RenderJPEGWidth(src, orientation, bannerWidth, bannerMaxHeight, bannerJPEGQuality)
}

// reject deletes the temp object and reports the outcome. A delete that fails is RECORDED, not hidden
// (`da_xoa_tep_tam: false`); the temp bucket's lifecycle removes the object within a day.
func (uc *Branding) reject(ctx context.Context, uploadKey, reason, signature string) brandingInspection {
	removed := uc.objects.PurgeAllVersions(ctx, storage.BucketTemp, uploadKey) == nil
	return brandingInspection{kind: brandingRejectedFile, reason: reason, signature: signature, tempRemoved: removed}
}

// brandingStorageErr keeps the sentinels the handler maps (not configured, changed) and wraps the rest.
func brandingStorageErr(what string, err error) error {
	switch {
	case errors.Is(err, storage.ErrNotConfigured):
		return fmt.Errorf("%w: %w", ErrBrandingUploadNotConfigured, err)
	case errors.Is(err, storage.ErrChanged):
		return fmt.Errorf("%w: %w", ErrBrandingUploadChanged, err)
	}
	return fmt.Errorf("nhận diện xã: %s: %w", what, err)
}

// --- keys ---------------------------------------------------------------------------------------------

func derivativeVariant(img domain.BrandingImage) string {
	if img == domain.BrandingLogo {
		return LogoDerivativeVariant
	}
	return BannerDerivativeVariant
}

// derivativeKey is the private derivative of an original key: same commune, month, service, purpose and
// object id; the image's variant; PNG for the logo (alpha), JPEG for the banner.
func derivativeKey(img domain.BrandingImage, orig storage.Key) storage.Key {
	k := orig
	k.Variant = derivativeVariant(img)
	k.Ext = "jpg"
	if img == domain.BrandingLogo {
		k.Ext = "png"
	}
	return k
}

// publicKey is that derivative's public twin — the only key PublishDerivative accepts for it.
func publicKey(img domain.BrandingImage, orig storage.Key) storage.Key {
	k := derivativeKey(img, orig)
	k.Class = storage.ClassPublicMedia
	return k
}

// --- the trail ----------------------------------------------------------------------------------------

// brandingAuditSubject is the trail's locator: the commune's display profile has no business code of its
// own (its key is the commune), so the subject names the record and the image; file ids are in the delta.
func brandingAuditSubject(img domain.BrandingImage) string {
	if img == domain.BrandingLogo {
		return "ho-so-hien-thi-xa/logo"
	}
	return "ho-so-hien-thi-xa/banner-web-admin"
}

func writeBrandingAudit(ctx context.Context, tx *store.ScopedTx, actor audit.Actor, action string,
	img domain.BrandingImage, at time.Time, d map[string]any) error {
	delta, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("nhận diện xã: mã hoá delta: %w", err)
	}
	// SAME TRANSACTION AS THE ROW (rule 6, invariant 3); TenantID filled by audit.Write from the tx.
	return audit.Write(ctx, tx, audit.Entry{Actor: actor, Action: action, Subject: brandingAuditSubject(img),
		At: at, Delta: delta})
}
