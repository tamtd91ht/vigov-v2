package app

// THE BROADCAST AUDIO OF A `truyen-thanh` ITEM (ADR 0067 §4; ADR 0052; ADR 0047 G7) — ONE upload request
// through this service (ADR 0052 §Sửa đổi 09/10/2026), the cover's shape (content_cover.go) WITHOUT a
// derivative and WITHOUT a public copy, in three acts inside Upload:
//
//	a. admitUpload  duration · declaration · the item (FOR UPDATE, `truyen-thanh`) · the count → pending row
//	b. PutUpload    the multipart file part streamed into the temp bucket
//	c. complete     sniff · audio check · scan · hash · copy to PRIVATE → ready, and ATTACHED to the item
//	                with the typed duration — ADR 0052 §1c, unchanged
//
// A failure after (a) moves the row to `failed` (ContentCovers.abandon's reasons and its verb, here
// ActionAudioExpired): with ONE file per item, a row left `pending` would refuse the officer's retry.
//
// WHAT DIFFERS FROM THE COVER, AND WHY:
//
//   - THE ITEM MUST EXIST AND BE `truyen-thanh` WHEN THE UPLOAD IS REQUESTED. A cover may be uploaded
//     before `Lưu` (0011), and mints the article id; the audio does not, because 0012's CHECK binds the
//     file to `truyen-thanh` and an id minted twice (cover and audio) cannot become one article.
//   - THE FILE IS ATTACHED AT COMPLETION, NOT BY A LATER PATCH as a cover is. Two reasons, both from
//     the schema: (1) platform's content-audio policy allows ONE live file per item
//     (`max_files_per_subject` = 1, platform 0013), so a completed upload left unattached would hold the
//     only slot and refuse every later upload for that item; (2) 0012's all-or-none CHECK needs the
//     duration in the same UPDATE as the file, and the officer types it with the file (ADR 0067 §4.1).
//     Completing is therefore one transaction: the file `ready`, the item's two columns set, one trail
//     entry. PATCH /content-items/{id} corrects the duration and REMOVES the audio (which soft-deletes
//     the file row, freeing the slot); it never attaches one.
//   - A REPLACEMENT IS "REMOVE, THEN UPLOAD": with one slot, a second upload while the first file is
//     live is refused (409 `audio_limit`), per 0013's "the old row is soft-deleted, never a second live
//     audio on one item".
//   - NO DERIVATIVE, NO PUBLIC COPY (ADR 0067 §4.2 and Còn mở #2: no transcoder). The original stays in
//     the PRIVATE bucket; residents get a presigned GET of it, valid PublicAudioURLTTL (PublicAudioURLs).
//     `ready` here means "verified original in the private bucket", not "derivative exists".
//   - THE TYPE IS PROVEN, NOT ONLY SNIFFED: after SniffMIME, storage.CheckAudioStream reads the whole
//     object — an M4A must declare a sound track and no video track, an MP3 two real Layer III frames.
//
// THE LIMITS ARE PLATFORM'S (`content-audio`, ADR 0052 §10): 30 MiB, audio/mpeg + audio/mp4, one file
// per item — on every request and again at completion. Not configured → refusal; platform down → 503.
//
// WHAT IS NEVER LOGGED OR PUT IN AN ERROR: the original file name (rule 3), a presigned URL (a bearer
// credential), file content. A public audio link is signed with NO file name, so the
// officer's name for the file never reaches a resident's URL.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/malwarescan"
	"github.com/vihat/vigov/core/platformclient/uploadpolicy"
	"github.com/vihat/vigov/core/storage"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// The verbs in the trail — Vietnamese snake_case like every other value this service writes (ADR 0011).
const (
	ActionAudioUploadRequested = "yeu_cau_tai_am_thanh_truyen_thanh"
	// ActionAudioStored: the file passed every check, is `ready`, and is now the item's audio — ONE entry
	// for both, because they are one transaction.
	ActionAudioStored   = "luu_am_thanh_truyen_thanh"
	ActionAudioRejected = "tu_choi_am_thanh_truyen_thanh"
	// ActionAudioExpired: an upload that ENDED WITH NOTHING STORED (uploadVerbs.abandoned says why the
	// verb kept its name); `ly_do` is a CoverAbandon* reason.
	ActionAudioExpired = "am_thanh_truyen_thanh_het_han_tai"
)

// Rejection reasons of an audio file. The shared ones keep the cover's values (one vocabulary in the
// trail); AudioRejectNotAudio is the whole-stream check.
const (
	AudioRejectMalware        = CoverRejectMalware
	AudioRejectTypeNotAllowed = CoverRejectTypeNotAllowed
	AudioRejectTypeMismatch   = CoverRejectTypeMismatch
	AudioRejectTooLarge       = CoverRejectTooLarge
	AudioRejectCountReached   = CoverRejectCountReached
	AudioRejectNotAudio       = "khong-phai-am-thanh"
)

// PublicAudioURLTTL is how long a resident's audio link lives. ADR 0067 §4.2 says "short-lived" with no
// number; this is core/storage's own cap (MaxDownloadTTL), the longest it will sign. A player that seeks
// after expiry needs a fresh link: the Mini App re-reads the item (`audio_url_expires_at` says when).
const PublicAudioURLTTL = 15 * time.Minute

var (
	ErrAudioUploadNotConfigured = errors.New("truyền thanh: chưa cấu hình kho lưu tệp")
	ErrAudioLimitsUnavailable   = errors.New("truyền thanh: chưa đọc được giới hạn tải tệp")
	// ErrAudioScanUnavailable: clamd could not scan. The row stays `pending`; NEVER stored unscanned.
	ErrAudioScanUnavailable = errors.New("truyền thanh: chưa quét được mã độc")
	ErrAudioTypeNotAllowed  = errors.New("truyền thanh: loại tệp không được phép")
	ErrAudioTooLarge        = errors.New("truyền thanh: tệp vượt dung lượng cho phép")
	// ErrAudioCountReached: the item already holds platform's one file (or an upload in flight). 409.
	ErrAudioCountReached = errors.New("truyền thanh: mục đã có tệp âm thanh")
	// ErrAudioItemRequired: no `content_item_id` — the audio is uploaded for a SAVED broadcast. 400.
	ErrAudioItemRequired = errors.New("truyền thanh: cần `content_item_id` của mục truyền thanh đã lưu")
	// ErrAudioFileNotFound: no such upload FOR THIS CALLER — one answer (rule 4, forbidden #2). 404.
	ErrAudioFileNotFound = errors.New("truyền thanh: không tìm thấy tệp âm thanh")
	ErrAudioNotPending   = errors.New("truyền thanh: tệp không còn chờ hoàn tất")
	// ErrAudioUploadIncomplete is ErrCoverUploadIncomplete for audio: always wraps the cause.
	ErrAudioUploadIncomplete = errors.New("truyền thanh: chưa nhận đủ tệp")
	ErrAudioUploadChanged    = errors.New("truyền thanh: tệp vừa bị thay đổi trong lúc kiểm tra")
	ErrAudioRejected         = errors.New("truyền thanh: tệp bị từ chối")
)

// AudioRejection is a completion that refused the FILE: the row is `rejected`, the temp object deleted
// or expiring within a day, the trail says why.
type AudioRejection struct{ Reason string }

func (e *AudioRejection) Error() string   { return ErrAudioRejected.Error() + ": " + e.Reason }
func (e *AudioRejection) Is(t error) bool { return t == ErrAudioRejected }

// AudioObjectStore is the part of *storage.Client the audio acts call (no derivative, no publish).
type AudioObjectStore interface {
	PutUpload(ctx context.Context, uploadKey string, r io.Reader, size int64, maxBytes int64,
		contentType string) (storage.ObjectInfo, error)
	Stat(ctx context.Context, b storage.Bucket, key string) (storage.ObjectInfo, error)
	ReadHead(ctx context.Context, b storage.Bucket, key, ifMatchETag string, n int) ([]byte, error)
	Open(ctx context.Context, b storage.Bucket, key, ifMatchETag string) (io.ReadCloser, int64, error)
	SHA256(ctx context.Context, b storage.Bucket, key, ifMatchETag string) (string, error)
	Promote(ctx context.Context, srcUploadKey, ifMatchETag string, dst storage.Key,
		b storage.Bucket) (storage.Promoted, error)
	PresignDownload(ctx context.Context, b storage.Bucket, key string, ttl time.Duration,
		filename string) (storage.PresignedURL, error)
	PurgeAllVersions(ctx context.Context, b storage.Bucket, key string) error
}

// AudioFiles is the part of *store.StoredFileStore the audio acts call.
type AudioFiles interface {
	InsertPending(ctx context.Context, tx *store.ScopedTx, f domain.StoredFile) error
	ForUpdate(ctx context.Context, tx *store.ScopedTx, id string) (*domain.StoredFile, error)
	ByID(ctx context.Context, id string) (*domain.StoredFile, error)
	Transition(ctx context.Context, tx *store.ScopedTx, id string, from, to domain.StoredFileStatus,
		at time.Time) error
	MarkStored(ctx context.Context, tx *store.ScopedTx, id string, facts domain.StoredFileFacts,
		at time.Time) error
	CountForSubjectTx(ctx context.Context, tx *store.ScopedTx, subjectID, purpose string,
		pendingSince time.Time) (int, error)
	CountForSubject(ctx context.Context, subjectID, purpose string) (int, error)
	ReadyObjectKeys(ctx context.Context, purpose string, ids []string) (map[string]string, error)
}

// audioPurpose is the one purpose an audio file is uploaded under (core/storage, platform 0013).
const audioPurpose = storage.PurposeContentAudio

// ContentAudio owns the audio upload acts, the staff preview and the public links.
type ContentAudio struct {
	db    *store.DB
	items KhoNoiDungMiniApp
	files AudioFiles

	// ANY nil means "not configured": every upload is refused with ErrAudioUploadNotConfigured and the
	// public read simply carries no audio link — fail closed, the rest of the service keeps serving.
	objects  AudioObjectStore
	scanner  MalwareScanner
	policies UploadPolicies

	newID func() (string, error)
	now   func() time.Time
}

// NewContentAudio builds the use case. Pass UNTYPED nil for a dependency that is not configured.
func NewContentAudio(db *store.DB, items KhoNoiDungMiniApp, files AudioFiles, objects AudioObjectStore,
	scanner MalwareScanner, policies UploadPolicies) *ContentAudio {
	return &ContentAudio{db: db, items: items, files: files, objects: objects, scanner: scanner,
		policies: policies, newID: storage.NewObjectID}
}

func (uc *ContentAudio) clock() time.Time {
	if uc.now == nil {
		return time.Now().UTC()
	}
	return uc.now().UTC()
}

func (uc *ContentAudio) uploadsConfigured() bool {
	return uc.objects != nil && uc.scanner != nil && uc.policies != nil && uc.files != nil
}

func (uc *ContentAudio) policy(ctx context.Context) (uploadpolicy.Policy, error) {
	p, ok, err := uc.policies.Policy(ctx, audioPurpose)
	switch {
	case errors.Is(err, uploadpolicy.ErrUnavailable):
		return uploadpolicy.Policy{}, fmt.Errorf("%w: %w", ErrAudioLimitsUnavailable, err)
	case err != nil:
		return uploadpolicy.Policy{}, fmt.Errorf("truyền thanh: đọc giới hạn tải tệp: %w", err)
	case !ok:
		return uploadpolicy.Policy{}, fmt.Errorf("%w: platform has no limit for %s",
			ErrAudioUploadNotConfigured, audioPurpose)
	}
	return p, nil
}

// --- the upload: admit, receive, complete -----------------------------------------------------------

// AudioUploadRequest is one upload as the handler received it: the declaration and the duration the
// officer typed (checked here), and the file stream, whose bytes the completion checks again (ADR 0052
// §1c). The stream fields are CoverUploadRequest's, with the same contract.
type AudioUploadRequest struct {
	// ContentItemID names an EXISTING, live `truyen-thanh` item of this commune. Required.
	ContentItemID string
	FileName      string
	ContentType   string // DECLARED; the key's extension follows it, completion sniffs the truth
	Size          int64  // DECLARED; File must yield exactly this many bytes
	// DurationSeconds is what the officer typed (ADR 0067 §4.1, ADR 0047 G7 — never measured by the
	// server), 1 .. 21600. Checked FIRST: a wrong figure refuses before a row or a byte.
	DurationSeconds int

	File     io.Reader
	Finish   func() error
	Deadline time.Time
}

// UploadLimit is the `content-audio` policy's byte cap — the handler's body cap, read before the body.
func (uc *ContentAudio) UploadLimit(ctx context.Context) (int64, error) {
	if !uc.uploadsConfigured() {
		return 0, ErrAudioUploadNotConfigured
	}
	pol, err := uc.policy(ctx)
	if err != nil {
		return 0, err
	}
	if pol.MaxBytes <= 0 {
		return 0, fmt.Errorf("%w: %s has no byte cap", ErrAudioUploadNotConfigured, audioPurpose)
	}
	return pol.MaxBytes, nil
}

// Upload receives one audio file, checks it and attaches it to its item with the typed duration. What
// each outcome leaves is ContentCovers.upload's table; on success the item carries the file.
func (uc *ContentAudio) Upload(ctx context.Context, req AudioUploadRequest, actor audit.Actor) (
	AudioCompletion, error) {

	if req.File == nil || req.Finish == nil {
		return AudioCompletion{}, errors.New("truyền thanh: lượt tải lên thiếu luồng tệp — lỗi nối dây ở tầng HTTP")
	}
	if actor.ID == "" {
		return AudioCompletion{}, ErrThieuNguoiTaoNoiDung
	}
	if err := domain.CheckAudioDuration(req.DurationSeconds); err != nil {
		return AudioCompletion{}, err
	}
	f, uploadKey, pol, err := uc.admitUpload(ctx, req, actor)
	if err != nil {
		return AudioCompletion{}, err
	}

	wctx, cancel := ctx, context.CancelFunc(func() {})
	if !req.Deadline.IsZero() {
		wctx, cancel = context.WithDeadline(ctx, req.Deadline)
	}
	_, err = uc.objects.PutUpload(wctx, uploadKey, req.File, req.Size, pol.MaxBytes, req.ContentType)
	cancel()
	if err == nil {
		err = req.Finish() // trailing bytes: refused; the temp object expires with the bucket lifecycle
	}
	if err != nil {
		return AudioCompletion{}, uc.abandon(ctx, f.ID, actor, CoverAbandonNotReceived,
			fmt.Errorf("%w: %w", ErrAudioUploadIncomplete, err))
	}

	done, err := uc.complete(ctx, f.ID, req.DurationSeconds, actor)
	if err != nil && !errors.Is(err, ErrAudioRejected) {
		return AudioCompletion{}, uc.abandon(ctx, f.ID, actor, CoverAbandonNotInspected, err)
	}
	return done, err
}

// abandon is ContentCovers.abandon for audio: `pending → failed` and ActionAudioExpired with the reason,
// one transaction, outside the request's cancellation; returns cause. It matters MORE here: the item has
// one slot, and a pending row would answer the officer's retry with 409 `audio_limit` for UploadTTL.
func (uc *ContentAudio) abandon(ctx context.Context, id string, actor audit.Actor, reason string, cause error) error {
	ctx = context.WithoutCancel(ctx)
	now := uc.clock()
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
		return writeAudioAudit(ctx, tx, actor, ActionAudioExpired, domain.NoiDungMiniApp{ID: cur.SubjectID},
			map[string]any{"tep_id": id, "muc_noi_dung_id": cur.SubjectID, "ly_do": reason})
	})
	if err != nil {
		return fmt.Errorf("%w; truyền thanh: dòng tệp %s chưa chuyển được sang failed: %w", cause, id, err)
	}
	return cause
}

// admitUpload is act (a). ONE TRANSACTION: the item FOR UPDATE (it must be a live `truyen-thanh`; two
// uploads serialise on it and on the count), the count, the pending row, the trail.
func (uc *ContentAudio) admitUpload(ctx context.Context, req AudioUploadRequest, actor audit.Actor) (
	domain.StoredFile, string, uploadpolicy.Policy, error) {

	var none uploadpolicy.Policy
	if req.ContentItemID == "" {
		return domain.StoredFile{}, "", none, ErrAudioItemRequired
	}
	if len(req.ContentItemID) > domain.MaxFileIDLen {
		return domain.StoredFile{}, "", none, commsstore.ErrNoiDungKhongTonTai
	}
	name, err := domain.CleanCoverFileName(req.FileName)
	if err != nil {
		return domain.StoredFile{}, "", none, err
	}
	if req.Size <= 0 {
		return domain.StoredFile{}, "", none, domain.ErrCoverSizeInvalid
	}
	if !uc.uploadsConfigured() {
		return domain.StoredFile{}, "", none, ErrAudioUploadNotConfigured
	}
	pol, err := uc.policy(ctx)
	if err != nil {
		return domain.StoredFile{}, "", none, err
	}
	if !pol.AllowsMIME(req.ContentType) {
		return domain.StoredFile{}, "", none, ErrAudioTypeNotAllowed
	}
	if req.Size > pol.MaxBytes {
		return domain.StoredFile{}, "", none, ErrAudioTooLarge
	}
	ext, ok := storage.ExtForMIME(req.ContentType)
	if !ok {
		return domain.StoredFile{}, "", none, ErrAudioTypeNotAllowed // unreachable while uploadpolicy narrows to storage's list
	}

	id, err := uc.newID()
	if err != nil {
		return domain.StoredFile{}, "", none, fmt.Errorf("truyền thanh: sinh mã tệp: %w", err)
	}
	now := uc.clock()
	key := storage.Key{
		Class: storage.ClassContentSource, TenantID: string(tenant.MustFrom(ctx)), CreatedAt: now,
		Service: storage.ServiceComms, Purpose: audioPurpose,
		ObjectID: id, Variant: storage.VariantOriginal, Ext: ext,
	}
	objectKey, err := key.Path()
	if err != nil {
		return domain.StoredFile{}, "", none, fmt.Errorf("truyền thanh: dựng khoá đối tượng: %w", err)
	}
	uploadKey, err := key.UploadPath()
	if err != nil {
		return domain.StoredFile{}, "", none, fmt.Errorf("truyền thanh: dựng khoá tải lên: %w", err)
	}

	var out domain.StoredFile
	var refusal error
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		item, err := uc.items.TheoIDDeSua(ctx, tx, req.ContentItemID)
		if err != nil {
			return err
		}
		if item.Loai != domain.LoaiTruyenThanh {
			refusal = domain.ErrAudioOnlyForBroadcast
			return refusal
		}
		if pol.FileCountLimited {
			live, err := uc.files.CountForSubjectTx(ctx, tx, item.ID, string(audioPurpose),
				now.Add(-storage.UploadTTL))
			if err != nil {
				return err
			}
			if live >= pol.MaxFilesPerSubject {
				refusal = ErrAudioCountReached
				return refusal
			}
		}
		f := domain.StoredFile{
			ID: id, Bucket: domain.StoredFileBucketPrivate, ObjectKey: objectKey,
			RetentionClass: string(storage.ClassContentSource), Purpose: string(audioPurpose),
			SubjectType: domain.StoredFileSubjectContentItem, SubjectID: item.ID, OriginalName: name,
			Status: domain.StoredFilePending, UploadedBy: actor.ID, CreatedAt: now, UpdatedAt: now,
		}
		if err := uc.files.InsertPending(ctx, tx, f); err != nil {
			return err
		}
		if err := writeAudioAudit(ctx, tx, actor, ActionAudioUploadRequested, item, map[string]any{
			"tep_id":          id,
			"muc_noi_dung_id": item.ID,
			"muc_dich":        string(audioPurpose),
			"loai_khai_bao":   req.ContentType,
			"kich_thuoc_khai": req.Size,
		}); err != nil {
			return err
		}
		out = f
		return nil
	})
	if refusal != nil {
		return domain.StoredFile{}, "", none, refusal
	}
	if err != nil {
		return domain.StoredFile{}, "", none, bocNoiDung(ctx, "nhận âm thanh truyền thanh", err)
	}
	return out, uploadKey, pol, nil
}

// --- c. complete the upload, and attach it ----------------------------------------------------------

type audioOutcome int

const (
	audioReady audioOutcome = iota + 1
	audioRejected
)

type audioInspection struct {
	kind        audioOutcome
	facts       domain.StoredFileFacts
	recovered   bool
	reason      string
	signature   string
	tempRemoved bool
}

// AudioCompletion is what Upload returns: the file, and the item as it now stands.
type AudioCompletion struct {
	File domain.StoredFile
	Item domain.NoiDungMiniApp
}

// complete is ADR 0052 §1c for the audio upload Upload just wrote, plus attaching it to its item with the
// duration the officer typed. It re-reads the row and checks it is THIS officer's audio upload.
//
// A file already `ready` is returned as it is and nothing is written (another writer finished first; the
// duration is NOT re-applied — correcting it is PATCH /content-items/{id}).
func (uc *ContentAudio) complete(ctx context.Context, id string, durationSeconds int, actor audit.Actor) (
	AudioCompletion, error) {

	if actor.ID == "" {
		return AudioCompletion{}, ErrThieuNguoiTaoNoiDung
	}
	if id == "" || len(id) > domain.MaxFileIDLen {
		return AudioCompletion{}, ErrAudioFileNotFound
	}
	if err := domain.CheckAudioDuration(durationSeconds); err != nil {
		return AudioCompletion{}, err
	}
	if !uc.uploadsConfigured() {
		return AudioCompletion{}, ErrAudioUploadNotConfigured
	}

	f, err := uc.files.ByID(ctx, id)
	if err != nil {
		return AudioCompletion{}, bocNoiDung(ctx, "hoàn tất âm thanh truyền thanh", err)
	}
	if !ownAudioUpload(f, actor.ID) {
		return AudioCompletion{}, ErrAudioFileNotFound
	}
	switch f.Status {
	case domain.StoredFileReady:
		return uc.alreadyDone(ctx, *f)
	case domain.StoredFilePending, domain.StoredFileStored, domain.StoredFileProcessing:
	default:
		return AudioCompletion{}, ErrAudioNotPending
	}
	pol, err := uc.policy(ctx)
	if err != nil {
		return AudioCompletion{}, err
	}
	key, err := storage.ParseKey(f.ObjectKey)
	if err != nil {
		return AudioCompletion{}, fmt.Errorf("truyền thanh: khoá đối tượng đã ghi không hợp lệ: %w", err)
	}

	insp, err := uc.inspect(ctx, *f, key, pol)
	if err != nil {
		return AudioCompletion{}, err
	}

	now := uc.clock()
	var done AudioCompletion
	var refusal error
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		cur, err := uc.files.ForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if !ownAudioUpload(cur, actor.ID) {
			refusal = ErrAudioFileNotFound
			return refusal
		}
		if cur.Status == domain.StoredFileReady {
			done.File = *cur // another completion finished first: same outcome, nothing to write
			return nil
		}
		switch insp.kind {
		case audioReady:
			// THE ITEM, LOCKED, STILL A LIVE BROADCAST WITH NO OTHER AUDIO. Otherwise nothing is written:
			// the row stays where it is and stops counting against the slot once its form expires.
			truoc, err := uc.items.TheoIDDeSua(ctx, tx, cur.SubjectID)
			if err != nil {
				return err
			}
			if truoc.Loai != domain.LoaiTruyenThanh {
				refusal = domain.ErrAudioOnlyForBroadcast
				return refusal
			}
			if truoc.AudioFileID != "" && truoc.AudioFileID != cur.ID {
				refusal = ErrAudioCountReached
				return refusal
			}
			final, err := uc.walkToReady(ctx, tx, *cur, insp.facts, now)
			if err != nil {
				return err
			}
			sau := truoc
			sau.AudioFileID, sau.AudioDurationSeconds = final.ID, durationSeconds
			if truoc.Nguon == domain.NguonDongBoCong {
				sau.DaSuaTay = true // §10.4: attaching a broadcast to a synced item is a staff edit
			}
			if err := sau.CheckTypeFields(); err != nil {
				refusal = err
				return refusal
			}
			if err := uc.items.CapNhat(ctx, tx, sau); err != nil {
				return err
			}
			done = AudioCompletion{File: final, Item: sau}
			return writeAudioAudit(ctx, tx, actor, ActionAudioStored, sau, map[string]any{
				"tep_id": id, "muc_noi_dung_id": sau.ID, "loai_tep": insp.facts.MIMEType,
				"kich_thuoc": insp.facts.SizeBytes, "sha256": insp.facts.SHA256,
				"khoi_phuc_tu_dich": insp.recovered,
				"truoc": map[string]any{
					"audio_file_id": truoc.AudioFileID, "audio_duration_seconds": truoc.AudioDurationSeconds},
				"sau": map[string]any{
					"audio_file_id": sau.AudioFileID, "audio_duration_seconds": sau.AudioDurationSeconds,
					"da_sua_tay": sau.DaSuaTay},
			})
		case audioRejected:
			if cur.Status != domain.StoredFilePending {
				return ErrAudioNotPending
			}
			d := map[string]any{"tep_id": id, "muc_noi_dung_id": cur.SubjectID,
				"ly_do": insp.reason, "da_xoa_tep_tam": insp.tempRemoved}
			if insp.signature != "" {
				d["chu_ky_ma_doc"] = insp.signature
			}
			if err := uc.files.Transition(ctx, tx, id, domain.StoredFilePending, domain.StoredFileRejected, now); err != nil {
				return err
			}
			return writeAudioAudit(ctx, tx, actor, ActionAudioRejected, domain.NoiDungMiniApp{ID: cur.SubjectID}, d)
		}
		return fmt.Errorf("truyền thanh: kết quả kiểm tra không rõ (%d)", insp.kind)
	})
	if refusal != nil {
		return AudioCompletion{}, refusal
	}
	if err != nil {
		return AudioCompletion{}, bocNoiDung(ctx, "hoàn tất âm thanh truyền thanh", err)
	}
	switch {
	case done.File.ID != "" && done.Item.ID == "":
		return uc.alreadyDone(ctx, done.File)
	case done.File.ID != "":
		return done, nil
	default:
		return AudioCompletion{}, &AudioRejection{Reason: insp.reason}
	}
}

// alreadyDone answers a completion of a file that is already `ready`: the file and its item as stored.
func (uc *ContentAudio) alreadyDone(ctx context.Context, f domain.StoredFile) (AudioCompletion, error) {
	out := AudioCompletion{File: f}
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		item, err := uc.items.TheoIDDeSua(ctx, tx, f.SubjectID)
		if err != nil {
			return err
		}
		out.Item = item
		return nil
	})
	if err != nil {
		return AudioCompletion{}, bocNoiDung(ctx, "hoàn tất âm thanh truyền thanh", err)
	}
	return out, nil
}

// walkToReady moves a row along pending → scanning → stored → processing → ready, from wherever it
// stands, one guarded edge per UPDATE so stored_file_guard sees every step. `processing` is crossed in
// the same transaction: there is no derivative to produce, and the guard has no stored → ready edge.
func (uc *ContentAudio) walkToReady(ctx context.Context, tx *store.ScopedTx, cur domain.StoredFile,
	facts domain.StoredFileFacts, now time.Time) (domain.StoredFile, error) {

	if cur.Status == domain.StoredFilePending {
		if err := uc.files.Transition(ctx, tx, cur.ID, domain.StoredFilePending, domain.StoredFileScanning, now); err != nil {
			return domain.StoredFile{}, err
		}
		if err := uc.files.MarkStored(ctx, tx, cur.ID, facts, now); err != nil {
			return domain.StoredFile{}, err
		}
		cur.Status, cur.MIMEType, cur.SizeBytes, cur.SHA256 =
			domain.StoredFileStored, facts.MIMEType, facts.SizeBytes, facts.SHA256
	}
	if cur.Status == domain.StoredFileStored {
		if err := uc.files.Transition(ctx, tx, cur.ID, domain.StoredFileStored, domain.StoredFileProcessing, now); err != nil {
			return domain.StoredFile{}, err
		}
		cur.Status = domain.StoredFileProcessing
	}
	if cur.Status != domain.StoredFileProcessing {
		return domain.StoredFile{}, ErrAudioNotPending
	}
	if err := uc.files.Transition(ctx, tx, cur.ID, domain.StoredFileProcessing, domain.StoredFileReady, now); err != nil {
		return domain.StoredFile{}, err
	}
	cur.Status, cur.UpdatedAt = domain.StoredFileReady, now
	return cur, nil
}

// ownAudioUpload: the row exists, is a content-item AUDIO file, and was issued to THIS officer.
func ownAudioUpload(f *domain.StoredFile, officer string) bool {
	return f != nil && officer != "" && f.SubjectType == domain.StoredFileSubjectContentItem &&
		f.UploadedBy == officer && f.Purpose == string(audioPurpose)
}

func writeAudioAudit(ctx context.Context, tx *store.ScopedTx, actor audit.Actor, action string,
	item domain.NoiDungMiniApp, d map[string]any) error {
	delta, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("truyền thanh: mã hoá delta: %w", err)
	}
	subject := "noi-dung-mini-app/truyen-thanh/" + item.ID
	if !item.NgayDang.IsZero() {
		subject = chuDeNoiDungMiniApp(item)
	}
	// SAME TRANSACTION AS THE ROW (rule 6, invariant 3); TenantID filled by audit.Write from the tx.
	return audit.Write(ctx, tx, audit.Entry{Actor: actor, Action: action, Subject: subject, Delta: delta})
}

// inspect is the lock-free half of complete. An error means nothing may be decided: complete writes
// nothing and Upload abandons the row.
func (uc *ContentAudio) inspect(ctx context.Context, f domain.StoredFile, key storage.Key,
	pol uploadpolicy.Policy) (audioInspection, error) {

	if f.Status != domain.StoredFilePending {
		return uc.fromDestination(ctx, f, key)
	}
	uploadKey, err := key.UploadPath()
	if err != nil {
		return audioInspection{}, fmt.Errorf("truyền thanh: dựng khoá tải lên: %w", err)
	}
	st, err := uc.objects.Stat(ctx, storage.BucketTemp, uploadKey)
	if errors.Is(err, storage.ErrNotFound) {
		return uc.fromDestination(ctx, f, key)
	}
	if err != nil {
		return audioInspection{}, audioStorageErr("đọc thông tin tệp tạm", err)
	}

	// The CURRENT policy decides: a limit tightened since the request rejects here.
	if st.Size <= 0 || st.Size > pol.MaxBytes {
		return uc.reject(ctx, uploadKey, AudioRejectTooLarge, ""), nil
	}
	head, err := uc.objects.ReadHead(ctx, storage.BucketTemp, uploadKey, st.ETag, storage.SniffBytes)
	if err != nil {
		return audioInspection{}, audioStorageErr("đọc đầu tệp", err)
	}
	mime, ext, ok := storage.SniffMIME(head)
	if !ok || !pol.AllowsMIME(mime) {
		// A video renamed .m4a sniffs video/mp4, which content-audio does not allow: refused here.
		return uc.reject(ctx, uploadKey, AudioRejectTypeNotAllowed, ""), nil
	}
	if ext != key.Ext {
		return uc.reject(ctx, uploadKey, AudioRejectTypeMismatch, ""), nil
	}
	if pol.FileCountLimited {
		have, err := uc.files.CountForSubject(ctx, f.SubjectID, string(audioPurpose))
		if err != nil {
			return audioInspection{}, err
		}
		if have >= pol.MaxFilesPerSubject {
			return uc.reject(ctx, uploadKey, AudioRejectCountReached, ""), nil
		}
	}

	rc, size, err := uc.objects.Open(ctx, storage.BucketTemp, uploadKey, st.ETag)
	if err != nil {
		return audioInspection{}, audioStorageErr("mở tệp để quét", err)
	}
	res, err := uc.scanner.Scan(ctx, rc, size)
	closeErr := rc.Close()
	if err != nil {
		if errors.Is(err, malwarescan.ErrNotConfigured) {
			return audioInspection{}, fmt.Errorf("%w: %w", ErrAudioUploadNotConfigured, err)
		}
		if errors.Is(err, storage.ErrChanged) {
			return audioInspection{}, fmt.Errorf("%w: %w", ErrAudioUploadChanged, err)
		}
		// UNSCANNABLE IS NEVER CLEAN.
		return audioInspection{}, fmt.Errorf("%w: %w", ErrAudioScanUnavailable, err)
	}
	if closeErr != nil {
		return audioInspection{}, audioStorageErr("đóng tệp sau khi quét", closeErr)
	}
	if !res.Clean {
		return uc.reject(ctx, uploadKey, AudioRejectMalware, res.Signature), nil
	}

	// THE PROOF THE SNIFF CANNOT GIVE: the whole stream is audio of the sniffed type (storage says how).
	rc, _, err = uc.objects.Open(ctx, storage.BucketTemp, uploadKey, st.ETag)
	if err != nil {
		return audioInspection{}, audioStorageErr("mở tệp để kiểm tra âm thanh", err)
	}
	isAudio, err := storage.CheckAudioStream(mime, rc)
	closeErr = rc.Close()
	if err != nil {
		return audioInspection{}, audioStorageErr("kiểm tra âm thanh", err)
	}
	if closeErr != nil {
		return audioInspection{}, audioStorageErr("đóng tệp sau khi kiểm tra âm thanh", closeErr)
	}
	if !isAudio {
		return uc.reject(ctx, uploadKey, AudioRejectNotAudio, ""), nil
	}

	sum, err := uc.objects.SHA256(ctx, storage.BucketTemp, uploadKey, st.ETag)
	if err != nil {
		return audioInspection{}, audioStorageErr("tính sha256", err)
	}
	pr, err := uc.objects.Promote(ctx, uploadKey, st.ETag, key, storage.BucketPrivate)
	switch {
	case err == nil, errors.Is(err, storage.ErrTempCleanup):
		// ErrTempCleanup: promoted; the leftover in temp expires with the bucket's 1-day lifecycle.
	case errors.Is(err, storage.ErrExists), errors.Is(err, storage.ErrNotFound):
		return uc.fromDestination(ctx, f, key) // another completion promoted it first
	default:
		return audioInspection{}, audioStorageErr("chép tệp sang kho lưu", err)
	}
	return audioInspection{kind: audioReady,
		facts: domain.StoredFileFacts{MIMEType: pr.ContentType, SizeBytes: st.Size, SHA256: sum}}, nil
}

// fromDestination handles "the original is already in the private bucket" (a concurrent Promote got there
// first) or "nothing in either bucket right after PutUpload succeeded" — an error, never the client's
// doing (ContentCovers.fromDestination says why). A destination object is TRUSTED
// AS CHECKED because nothing else writes there: Promote is the only path into
// `content-source/…/comms/content-audio/…/original.*`, and it runs only after the scan and the audio
// check passed (ADR 0052 §3 scopes this service's key to its own subtree).
func (uc *ContentAudio) fromDestination(ctx context.Context, f domain.StoredFile,
	key storage.Key) (audioInspection, error) {

	st, err := uc.objects.Stat(ctx, storage.BucketPrivate, f.ObjectKey)
	if errors.Is(err, storage.ErrNotFound) {
		if f.Status != domain.StoredFilePending {
			return audioInspection{}, errors.New("truyền thanh: dòng đã lưu nhưng không thấy bản gốc ở kho lưu")
		}
		return audioInspection{}, fmt.Errorf("truyền thanh: vừa ghi tệp %s vào kho tạm nhưng không thấy ở kho tạm lẫn kho lưu", f.ID)
	}
	if err != nil {
		return audioInspection{}, audioStorageErr("đọc thông tin tệp đã lưu", err)
	}
	facts := domain.StoredFileFacts{MIMEType: f.MIMEType, SizeBytes: f.SizeBytes, SHA256: f.SHA256}
	if f.Status == domain.StoredFilePending {
		head, err := uc.objects.ReadHead(ctx, storage.BucketPrivate, f.ObjectKey, st.ETag, storage.SniffBytes)
		if err != nil {
			return audioInspection{}, audioStorageErr("đọc đầu tệp đã lưu", err)
		}
		mime, ext, ok := storage.SniffMIME(head)
		if !ok || ext != key.Ext {
			return audioInspection{}, errors.New("truyền thanh: đối tượng ở kho lưu không khớp kiểu của khoá")
		}
		sum, err := uc.objects.SHA256(ctx, storage.BucketPrivate, f.ObjectKey, st.ETag)
		if err != nil {
			return audioInspection{}, audioStorageErr("tính sha256 tệp đã lưu", err)
		}
		facts = domain.StoredFileFacts{MIMEType: mime, SizeBytes: st.Size, SHA256: sum}
	}
	return audioInspection{kind: audioReady, facts: facts, recovered: true}, nil
}

// reject deletes the temp object and reports the outcome; a failed delete is RECORDED, not hidden.
func (uc *ContentAudio) reject(ctx context.Context, uploadKey, reason, signature string) audioInspection {
	removed := uc.objects.PurgeAllVersions(ctx, storage.BucketTemp, uploadKey) == nil
	return audioInspection{kind: audioRejected, reason: reason, signature: signature, tempRemoved: removed}
}

// audioStorageErr keeps the sentinels the handler maps (not configured, changed) and wraps the rest.
func audioStorageErr(what string, err error) error {
	switch {
	case errors.Is(err, storage.ErrNotConfigured):
		return fmt.Errorf("%w: %w", ErrAudioUploadNotConfigured, err)
	case errors.Is(err, storage.ErrChanged):
		return fmt.Errorf("%w: %w", ErrAudioUploadChanged, err)
	}
	return fmt.Errorf("truyền thanh: %s: %w", what, err)
}

// --- staff preview, public links ----------------------------------------------------------------------

// AudioView is what the staff detail shows about an item's audio file.
type AudioView struct {
	FileID    string
	Status    domain.StoredFileStatus
	MIMEType  string
	SizeBytes int64
	// PreviewURL is a presigned GET of the ORIGINAL in the private bucket, valid until PreviewExpiresAt.
	// A bearer credential: never logged. Empty when not ready or not configured.
	PreviewURL       storage.PresignedURL
	PreviewExpiresAt time.Time
}

// View describes the audio file of one item. NOT AUDITED: staff of the commune listening to what their
// commune is about to broadcast — the footing of GET /content-items/{id}, which is not audited either.
func (uc *ContentAudio) View(ctx context.Context, fileID string) (AudioView, error) {
	v := AudioView{FileID: fileID}
	if uc.files == nil {
		return v, nil
	}
	f, err := uc.files.ByID(ctx, fileID)
	if err != nil {
		return v, fmt.Errorf("truyền thanh: đọc tệp: %w", err)
	}
	if f == nil {
		return v, nil
	}
	v.Status, v.MIMEType, v.SizeBytes = f.Status, f.MIMEType, f.SizeBytes
	if f.Status != domain.StoredFileReady || uc.objects == nil {
		return v, nil
	}
	u, err := uc.objects.PresignDownload(ctx, storage.BucketPrivate, f.ObjectKey, storage.MaxDownloadTTL,
		f.OriginalName)
	if err != nil {
		return v, audioStorageErr("ký liên kết nghe thử", err)
	}
	v.PreviewURL, v.PreviewExpiresAt = u, uc.clock().Add(storage.MaxDownloadTTL)
	return v, nil
}

// PublicAudio is one resident-facing audio link.
type PublicAudio struct {
	URL       storage.PresignedURL // bearer credential for its TTL — never logged
	ExpiresAt time.Time
}

// PublicAudioURLs maps audio file ids to a presigned GET of the PRIVATE original (ADR 0067 §4.2), for
// the public news routes. Only `ready`, live content-audio rows are signed; anything else is absent —
// and so is every file when object storage is not configured: an item without its player is the safe
// failure, never a broken link.
//
// SIGNED WITHOUT IDENTITY BINDING, AND THAT IS A DECISION, NOT AN OMISSION: rule 4 invariant 7 binds
// CITIZEN ATTACHMENTS (a resident's own petition photo) to identity and commune. This is a public
// authority's broadcast to every resident of the commune, on a route with no session to bind to. What
// the link keeps is the rest of ADR 0052: private bucket, signature, expiry ≤ PublicAudioURLTTL, and NO
// FILE NAME in it (the disposition falls back to `download.<ext>`), so the officer's name for the file
// never reaches a resident. Only the caller decides an item is published; this signs what it is given.
func (uc *ContentAudio) PublicAudioURLs(ctx context.Context, fileIDs []string) (map[string]PublicAudio, error) {
	out := map[string]PublicAudio{}
	if len(fileIDs) == 0 || uc.objects == nil || uc.files == nil {
		return out, nil
	}
	keys, err := uc.files.ReadyObjectKeys(ctx, string(audioPurpose), fileIDs)
	if err != nil {
		return nil, err
	}
	exp := uc.clock().Add(PublicAudioURLTTL)
	for id, key := range keys {
		u, err := uc.objects.PresignDownload(ctx, storage.BucketPrivate, key, PublicAudioURLTTL, "")
		if err != nil {
			continue // a key storage refuses: omitted, never guessed
		}
		out[id] = PublicAudio{URL: u, ExpiresAt: exp}
	}
	return out, nil
}
