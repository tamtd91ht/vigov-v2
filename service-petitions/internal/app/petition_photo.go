package app

// A CITIZEN'S SCENE PHOTOS ON THEIR OWN PETITION — menu phan-anh-nguoi-dan (docs/ui-ux/09 §8.4,
// "TRƯỚC KHI XỬ LÝ"), owner decisions of 02/10/2026 (ADR 0047 row "Ảnh hiện trường khi gửi phản ánh",
// and G3), storage of migration 0026. ADR 0052's three-step upload, with the one change G3 makes:
//
//	a. RequestUpload  pending row + presigned POST into temp            the petition locked, ≤ 5, open
//	b. the phone      POST straight to OBJECT_STORAGE_PUBLIC_ENDPOINT   bytes never cross this service
//	c. Complete       stat · sniff · scan · DECODE · ORIENT · RE-ENCODE JPEG WITHOUT EXIF ·
//	                  PutServerProduced into private · purge temp · `stored` + trail in ONE transaction
//
// THE RAW UPLOAD NEVER REACHES THE PRIVATE BUCKET. It carries EXIF — the GPS of the citizen's home,
// device identifiers, the capture time (rule 3) — so step (c) does not Promote it, as the task path
// does: it stores only the re-encoded bytes, as variant `original` of class `citizen-media`
// (core/storage.PutServerProduced (b)), and deletes the temp object. WebP and PNG come out as JPEG.
//
// # WHO
//
// The citizen who FILED the petition, from the SESSION (rule 4, invariant 2): every petition read
// here filters by `cong_dan_id = <session citizen>` and the commune of the session, so another
// citizen's code, another commune's and an unknown one are one ErrPhieuKhongTonTai — one 404 body.
// `stored_file` does not hold the citizen (migration 0026 says why); a file belongs to the caller
// because it hangs off a petition that does.
//
// # WHEN — ONLY WHILE THE PETITION IS `da-tiep-nhan` (domain.PhotoUploadOpen)
//
// Checked when the slot is issued and again, under the petition's row lock, when the photo is stored.
// A photo whose petition moved on while it was being processed is RECORDED as refused, and the clean
// copy already written is removed, so the set staff are reading never grows behind them.
//
// # HOW MANY — platform's `max_files_per_subject` for `petition-photo`, counted under the lock
//
// The slot count includes pending uploads whose form is still alive, inside the transaction that holds
// the petition FOR UPDATE, so concurrent requests cannot pass at 4 (the task path's CountForSubjectTx).
// Migration 0026's trigger is the floor under it for stored photos. A policy with NO count limit is
// treated as NOT CONFIGURED (503): the owner's ceiling is 5, and an unlimited policy is a misconfigured
// one, not permission to issue unlimited slots.
//
// # WHAT IS NEVER LOGGED OR PUT IN AN ERROR
//
// The lookup code, the citizen id, any presigned URL or form field, image bytes. Errors carry the
// commune (bocPhieu) and the act. The trail carries the file id, the sniffed types, size and hash.
//
// # NOT CHECKED, BY DECISION — THE APP THE SESSION CAME FROM
//
// The citizen session this service sees (core/httpx.CitizenSession: session id, citizen id, commune —
// "there is no fourth field"; identity.proto CitizenSessionPrincipal) records nothing about which app
// opened it. Asked 02/10/2026, the owner answered that a photo may come from either app ("mở ở đâu cũng
// được"): "only the commune's own app" is the scope of the BUTTON (citizen-app `AppRieng`), not a server
// rule. What still holds here is the sender and the commune — the session's citizen must own the
// petition, in the session's commune. A field recording the opening app is optional future work
// (ADR 0047, row "Ảnh hiện trường khi gửi phản ánh"); do not add a check against it without that.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/imaging"
	"github.com/vihat/vigov/core/malwarescan"
	"github.com/vihat/vigov/core/platformclient/uploadpolicy"
	"github.com/vihat/vigov/core/storage"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// The verbs in the trail — Vietnamese snake_case like every other value this service writes (ADR 0011).
const (
	ActionPetitionPhotoRequested = "cong_dan_xin_tai_anh_hien_truong"
	ActionPetitionPhotoStored    = "cong_dan_luu_anh_hien_truong"
	ActionPetitionPhotoRejected  = "tu_choi_anh_hien_truong"
	ActionPetitionPhotoExpired   = "anh_hien_truong_het_han_tai"
)

// Rejection reasons this flow adds to task_attachment.go's (RejectMalware, RejectTypeNotAllowed, …).
const (
	RejectUndecodable   = "khong-giai-ma-duoc"
	RejectTooManyPixels = "qua-nhieu-diem-anh"
	// RejectPetitionMoved: staff moved the petition out of `da-tiep-nhan` while the photo was processed.
	RejectPetitionMoved = "phieu-da-chuyen-buoc"
)

// The re-encode's three VENDOR bounds — memory and output size, not customer numbers.
const (
	// photoMaxSide caps the longer side of the stored photo. Without a cap a 48-megapixel phone photo is
	// re-encoded at full size: three full-size buffers (decode, RGBA, rotation) are ~500 MB, above the
	// petitions pod's 384 MiB limit (deploy/base/petitions/deployment.yaml). 2560 px keeps the detail
	// staff need to read a scene (≈ 5 MP at 4:3, ~1–2 MB at this quality) and an RGBA buffer of ~20 MB.
	// ⚠ A DOWNSCALE OF CITIZEN EVIDENCE — put to the owner as an open question, not decided by them.
	photoMaxSide = 2560
	// photoJPEGQuality is comms' cover quality: visually lossless at this size.
	photoJPEGQuality = 85
	// photoDecodeBudget is comms' coverDecodeBudget, for the same pod size and the same reason: the
	// estimated decode of ONE image (imaging.DecodeCost), refused from the header before any pixel.
	photoDecodeBudget = 160 << 20
)

var (
	// ErrPhotoNotFound: no such photo ON THIS PETITION — unknown id, another petition's, another
	// commune's, a staff file. One answer for all. 404.
	ErrPhotoNotFound = errors.New("ảnh hiện trường: không tìm thấy")
	// ErrPhotoWindowClosed: the petition is no longer `da-tiep-nhan`. 409.
	ErrPhotoWindowClosed = errors.New("ảnh hiện trường: phiếu đã chuyển bước, không đính thêm ảnh được")
	// ErrPhotoCountReached: the petition already holds platform's maximum (live slots included). 409.
	ErrPhotoCountReached = errors.New("ảnh hiện trường: phiếu đã đủ số ảnh tối đa")
	// ErrPhotoTypeNotAllowed: the DECLARED type is not an image type the policy and the re-encoder
	// both accept. 400, before any row.
	ErrPhotoTypeNotAllowed = errors.New("ảnh hiện trường: loại tệp không được phép")
	// ErrPhotoTooLarge: the DECLARED size is above platform's limit. 400, before any row.
	ErrPhotoTooLarge = errors.New("ảnh hiện trường: ảnh vượt dung lượng cho phép")
	// ErrPhotoSizeInvalid: the declared size is not a positive number of bytes. 400.
	ErrPhotoSizeInvalid = errors.New("ảnh hiện trường: kích thước khai báo không hợp lệ")
)

// PhotoObjectStore is the part of *storage.Client this flow calls — the task path's ObjectStore minus
// Promote (the raw upload is never promoted) plus PutServerProduced.
type PhotoObjectStore interface {
	PresignUpload(ctx context.Context, uploadKey string, maxBytes int64, contentType string,
		ttl time.Duration) (storage.PresignedPost, error)
	Stat(ctx context.Context, b storage.Bucket, key string) (storage.ObjectInfo, error)
	ReadHead(ctx context.Context, b storage.Bucket, key, ifMatchETag string, n int) ([]byte, error)
	Open(ctx context.Context, b storage.Bucket, key, ifMatchETag string) (io.ReadCloser, int64, error)
	SHA256(ctx context.Context, b storage.Bucket, key, ifMatchETag string) (string, error)
	PutServerProduced(ctx context.Context, dst storage.Key, r io.Reader, size int64) (storage.Produced, error)
	PresignDownload(ctx context.Context, b storage.Bucket, key string, ttl time.Duration,
		filename string) (storage.PresignedURL, error)
	PurgeAllVersions(ctx context.Context, b storage.Bucket, key string) error
}

// photoLister is the one read the link signing needs.
type photoLister interface {
	PetitionPhotos(ctx context.Context, petitionID string) ([]domain.StoredFile, error)
}

// PetitionPhotoFiles is the part of *petstore.StoredFileStore this flow calls.
type PetitionPhotoFiles interface {
	photoLister
	InsertPending(ctx context.Context, tx *store.ScopedTx, f domain.StoredFile) error
	ForUpdate(ctx context.Context, tx *store.ScopedTx, id string) (*domain.StoredFile, error)
	Transition(ctx context.Context, tx *store.ScopedTx, id string, from, to domain.StoredFileStatus,
		at time.Time) error
	MarkStored(ctx context.Context, tx *store.ScopedTx, id string, facts domain.StoredFileFacts,
		at time.Time) error
	CountForSubjectTx(ctx context.Context, tx *store.ScopedTx, subjectType, subjectID, purpose string,
		pendingSince time.Time) (int, error)
	CountForSubject(ctx context.Context, subjectType, subjectID, purpose string,
		pendingSince time.Time) (int, error)
	ByID(ctx context.Context, id string) (*domain.StoredFile, error)
}

// CitizenPhotoPetitions is the two IDENTITY-FILTERED petition reads — never the staff register's
// unfiltered one, which this type cannot reach (rule 4, invariant 5).
type CitizenPhotoPetitions interface {
	// vi-name-ok: mirrors the existing PhieuPhanAnhStore method; rule 12 invariant 3 keeps existing names
	CuaCongDanTheoMaTraCuu(ctx context.Context, congDanID, ma string) (domain.PhieuPhanAnh, error)
	CitizenPetitionForUpdate(ctx context.Context, tx *store.ScopedTx, congDanID, ma string) (
		domain.PhieuPhanAnh, error)
}

// PhotoUploadRequest is what the phone declares before it uploads. NO FILE NAME (domain.PetitionPhotoName
// says why). Both values are claims, checked again on the bytes at completion (ADR 0052 §1c).
type PhotoUploadRequest struct {
	ContentType string
	Size        int64
}

// PhotoUpload is the pending row and the form the phone posts the image with.
type PhotoUpload struct {
	File domain.StoredFile
	Post storage.PresignedPost // bearer credential for its TTL — never logged
}

// PhotoLink is one stored photo with a presigned GET — a bearer credential until ExpiresAt.
type PhotoLink struct {
	File      domain.StoredFile
	URL       storage.PresignedURL
	ExpiresAt time.Time
}

// CitizenPetitionPhotos owns the citizen's three acts: request a slot, complete it, list their photos.
type CitizenPetitionPhotos struct {
	db        *store.DB
	petitions CitizenPhotoPetitions
	files     PetitionPhotoFiles

	// ANY OF THE THREE nil means "not configured": every upload is refused with ErrUploadNotConfigured
	// (503) while the petition intake keeps serving. The list needs only `objects`.
	objects  PhotoObjectStore
	scanner  MalwareScanner
	policies UploadPolicies

	// decodeSlot admits ONE decode at a time in this process: photoDecodeBudget is a per-decode bound,
	// and two at once would be two budgets against one pod.
	decodeSlot chan struct{}

	newID func() (string, error)
	now   func() time.Time
}

// NewCitizenPetitionPhotos builds the use case. Pass UNTYPED nil for a dependency that is not configured
// (a nil *storage.Client inside a non-nil interface would pass the nil check and panic on first use).
func NewCitizenPetitionPhotos(db *store.DB, petitions CitizenPhotoPetitions, files PetitionPhotoFiles,
	objects PhotoObjectStore, scanner MalwareScanner, policies UploadPolicies) *CitizenPetitionPhotos {
	return &CitizenPetitionPhotos{db: db, petitions: petitions, files: files, objects: objects,
		scanner: scanner, policies: policies, decodeSlot: make(chan struct{}, 1), newID: storage.NewObjectID}
}

func (uc *CitizenPetitionPhotos) clock() time.Time {
	if uc.now == nil {
		return time.Now().UTC()
	}
	return uc.now().UTC()
}

func (uc *CitizenPetitionPhotos) uploadsConfigured() bool {
	return uc.objects != nil && uc.scanner != nil && uc.policies != nil
}

// citizenOnly is the wall GuiPhanAnh.Gui and RatePetition.Rate put first: authz.CitizenOnly runs on the
// route, so reaching here without a citizen means the route was mounted wrong. The kind is printed, the
// id never is.
func citizenOnly(citizen audit.Actor) error {
	if citizen.ID == "" || citizen.Kind != "citizen" {
		return fmt.Errorf("anh_hien_truong: chủ thể không phải công dân (kind=%q) — tuyến thiếu "+
			"authz.CitizenOnly hoặc authz.CitizenPrincipal", citizen.Kind)
	}
	return nil
}

// photoPolicy reads platform's limit for petition photos. Not configured, unreachable, and a policy
// with no per-petition count are all refusals — there is no default anywhere (ADR 0052 stop #4).
func (uc *CitizenPetitionPhotos) photoPolicy(ctx context.Context) (uploadpolicy.Policy, error) {
	p, ok, err := uc.policies.Policy(ctx, storage.PurposePetitionPhoto)
	switch {
	case errors.Is(err, uploadpolicy.ErrUnavailable):
		return uploadpolicy.Policy{}, fmt.Errorf("%w: %w", ErrUploadLimitsUnavailable, err)
	case err != nil:
		return uploadpolicy.Policy{}, fmt.Errorf("ảnh hiện trường: đọc giới hạn tải tệp: %w", err)
	case !ok:
		return uploadpolicy.Policy{}, fmt.Errorf("%w: platform has no limit for %s",
			ErrUploadNotConfigured, storage.PurposePetitionPhoto)
	case !p.FileCountLimited || p.MaxFilesPerSubject <= 0:
		return uploadpolicy.Policy{}, fmt.Errorf("%w: the %s policy has no per-petition file count",
			ErrUploadNotConfigured, storage.PurposePetitionPhoto)
	}
	return p, nil
}

// photoTypeAllowed: the policy admits the type AND the re-encoder decodes it. Both, because a policy
// row that still listed HEIC (the seed did until platform migration 0014) must not issue a slot for a
// file this service could only refuse at completion.
func photoTypeAllowed(pol uploadpolicy.Policy, mime string) bool {
	switch mime {
	case imaging.MIMEJPEG, imaging.MIMEPNG, imaging.MIMEWebP:
		return pol.AllowsMIME(mime)
	}
	return false
}

// photoKey is the destination key of one photo: class citizen-media, this service, purpose
// petition-photo, variant original — and ALWAYS `.jpg`, because the stored bytes are the re-encode.
func photoKey(ctx context.Context, id string, at time.Time) storage.Key {
	return storage.Key{
		Class: storage.ClassCitizenMedia, TenantID: string(tenant.MustFrom(ctx)), CreatedAt: at,
		Service: storage.ServicePetitions, Purpose: storage.PurposePetitionPhoto,
		ObjectID: id, Variant: storage.VariantOriginal, Ext: "jpg",
	}
}

// uploadKeyFor is the temp key the phone writes: the destination key with the DECLARED type's
// extension (core/storage.PresignUpload requires the two to match).
func uploadKeyFor(dst storage.Key, ext string) (string, error) {
	k := dst
	k.Ext = ext
	return k.UploadPath()
}

func writePhotoAudit(ctx context.Context, tx *store.ScopedTx, citizen audit.Actor, code, action string,
	at time.Time, d map[string]any) error {
	delta, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("ảnh hiện trường: mã hoá delta: %w", err)
	}
	// TenantID left unset: audit.Write takes it from the transaction (rule 1, invariant 4). Subject is
	// the BUSINESS CODE of the petition — what every screen names it by — never the file's internal id.
	return audit.Write(ctx, tx, audit.Entry{Actor: citizen, Action: action, Subject: code, At: at, Delta: delta})
}

// --- a. request a slot ------------------------------------------------------------------------------

// RequestUpload issues one upload slot for one photo on the citizen's own petition `ma`.
//
// ONE TRANSACTION: the petition read FOR UPDATE with both isolation axes (it also serialises two
// requests on one petition), the status, the count with live pending slots, the pending row, the form
// (signed offline, so a signing failure leaves no row) and the audit entry.
func (uc *CitizenPetitionPhotos) RequestUpload(ctx context.Context, ma string, req PhotoUploadRequest,
	citizen audit.Actor) (PhotoUpload, error) {

	if err := citizenOnly(citizen); err != nil {
		return PhotoUpload{}, err
	}
	if req.Size <= 0 {
		return PhotoUpload{}, ErrPhotoSizeInvalid
	}
	if !uc.uploadsConfigured() {
		return PhotoUpload{}, ErrUploadNotConfigured
	}
	pol, err := uc.photoPolicy(ctx)
	if err != nil {
		return PhotoUpload{}, err
	}
	if !photoTypeAllowed(pol, req.ContentType) {
		return PhotoUpload{}, ErrPhotoTypeNotAllowed
	}
	if req.Size > pol.MaxBytes {
		return PhotoUpload{}, ErrPhotoTooLarge
	}
	upExt, ok := storage.ExtForMIME(req.ContentType)
	if !ok {
		return PhotoUpload{}, ErrPhotoTypeNotAllowed
	}

	id, err := uc.newID()
	if err != nil {
		return PhotoUpload{}, fmt.Errorf("ảnh hiện trường: sinh mã tệp: %w", err)
	}
	now := uc.clock()
	dst := photoKey(ctx, id, now)
	objectKey, err := dst.Path()
	if err != nil {
		return PhotoUpload{}, fmt.Errorf("ảnh hiện trường: dựng khoá đối tượng: %w", err)
	}
	uploadKey, err := uploadKeyFor(dst, upExt)
	if err != nil {
		return PhotoUpload{}, fmt.Errorf("ảnh hiện trường: dựng khoá tải lên: %w", err)
	}

	var out PhotoUpload
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		p, err := uc.petitions.CitizenPetitionForUpdate(ctx, tx, citizen.ID, ma)
		if err != nil {
			return err
		}
		if !domain.PhotoUploadOpen(p.TrangThai) {
			return ErrPhotoWindowClosed
		}
		live, err := uc.files.CountForSubjectTx(ctx, tx, domain.StoredFileSubjectPetition, p.ID,
			domain.PurposePetitionPhoto, now.Add(-storage.UploadTTL))
		if err != nil {
			return err
		}
		if live >= pol.MaxFilesPerSubject {
			return ErrPhotoCountReached
		}
		f := domain.StoredFile{
			ID: id, Bucket: domain.StoredFileBucketPrivate, ObjectKey: objectKey,
			RetentionClass: string(storage.ClassCitizenMedia), Purpose: domain.PurposePetitionPhoto,
			SubjectType: domain.StoredFileSubjectPetition, SubjectID: p.ID, OriginalName: domain.PetitionPhotoName,
			Status: domain.StoredFilePending, UploadedBy: domain.CitizenLogActor, CreatedAt: now, UpdatedAt: now,
		}
		if err := uc.files.InsertPending(ctx, tx, f); err != nil {
			return err
		}
		post, err := uc.objects.PresignUpload(ctx, uploadKey, pol.MaxBytes, req.ContentType, storage.UploadTTL)
		if err != nil {
			return fmt.Errorf("ảnh hiện trường: ký lượt tải lên: %w", err)
		}
		if err := writePhotoAudit(ctx, tx, citizen, p.MaTraCuu, ActionPetitionPhotoRequested, now, map[string]any{
			"tep_id": id, "muc_dich": domain.PurposePetitionPhoto,
			"loai_khai_bao": req.ContentType, "kich_thuoc_khai": req.Size,
		}); err != nil {
			return err
		}
		out = PhotoUpload{File: f, Post: post}
		return nil
	})
	if err != nil {
		return PhotoUpload{}, bocPhieu(ctx, "xin tải ảnh hiện trường", err)
	}
	return out, nil
}

// --- c. complete ------------------------------------------------------------------------------------

// photoInspection is what the lock-free half of Complete found.
type photoInspection struct {
	kind        outcomeKind
	facts       domain.StoredFileFacts // of the STORED (re-encoded) object
	sourceMIME  string                 // the sniffed type of the upload, for the trail
	recovered   bool
	reason      string
	signature   string
	tempRemoved bool
}

// Complete runs step (c) for one upload of the citizen's own petition `ma`.
//
// The object-store and pixel work happens lock-free first (task_attachment.go says why); then ONE short
// transaction re-reads the petition and the row FOR UPDATE, re-checks the status and the count under the
// lock, and records the outcome with its trail.
//
// IDEMPOTENT: a photo already stored answers as stored, so a retried request answers the same outcome
// (the route declares idem.KhongCan).
func (uc *CitizenPetitionPhotos) Complete(ctx context.Context, ma, id string, citizen audit.Actor) (
	domain.StoredFile, error) {

	if err := citizenOnly(citizen); err != nil {
		return domain.StoredFile{}, err
	}
	if !uc.uploadsConfigured() {
		return domain.StoredFile{}, ErrUploadNotConfigured
	}

	// 1. Lock-free pre-read: the caller's own petition, a photo of it, and something left to do.
	p, err := uc.petitions.CuaCongDanTheoMaTraCuu(ctx, citizen.ID, ma)
	if err != nil {
		return domain.StoredFile{}, bocPhieu(ctx, "hoàn tất ảnh hiện trường", err)
	}
	f, err := uc.files.ByID(ctx, id)
	if err != nil {
		return domain.StoredFile{}, bocPhieu(ctx, "hoàn tất ảnh hiện trường", err)
	}
	if f == nil || !domain.IsCitizenPhotoOf(*f, p.ID) {
		return domain.StoredFile{}, ErrPhotoNotFound
	}
	switch {
	case f.Status.Attachable() || f.Status == domain.StoredFileProcessing:
		return *f, nil
	case f.Status != domain.StoredFilePending:
		return domain.StoredFile{}, ErrAttachmentNotPending
	}
	if !domain.PhotoUploadOpen(p.TrangThai) {
		return domain.StoredFile{}, ErrPhotoWindowClosed
	}
	pol, err := uc.photoPolicy(ctx)
	if err != nil {
		return domain.StoredFile{}, err
	}
	dst, err := storage.ParseKey(f.ObjectKey)
	if err != nil {
		return domain.StoredFile{}, fmt.Errorf("ảnh hiện trường: khoá đối tượng đã ghi không hợp lệ: %w", err)
	}

	// 2. Object-store and pixel work, no lock held.
	insp, err := uc.inspect(ctx, *f, dst, pol)
	if err != nil {
		return domain.StoredFile{}, err
	}
	if insp.kind == outcomeNotReceived {
		return domain.StoredFile{}, ErrUploadNotReceived
	}

	// 3. One short transaction.
	now := uc.clock()
	var stored domain.StoredFile
	lateReason := ""
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		lp, err := uc.petitions.CitizenPetitionForUpdate(ctx, tx, citizen.ID, ma)
		if err != nil {
			return err
		}
		cur, err := uc.files.ForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if cur == nil || !domain.IsCitizenPhotoOf(*cur, lp.ID) {
			return ErrPhotoNotFound
		}
		if cur.Status.Attachable() || cur.Status == domain.StoredFileProcessing {
			stored = *cur // another completion finished first: the same outcome, nothing to write
			return nil
		}
		if cur.Status != domain.StoredFilePending {
			return ErrAttachmentNotPending
		}

		switch insp.kind {
		case outcomeStored:
			// UNDER THE LOCK: the status and the count may have moved while the image was processed.
			if !domain.PhotoUploadOpen(lp.TrangThai) {
				lateReason = RejectPetitionMoved
			} else {
				have, err := uc.files.CountForSubjectTx(ctx, tx, domain.StoredFileSubjectPetition, lp.ID,
					domain.PurposePetitionPhoto, time.Time{})
				if err != nil {
					return err
				}
				if have >= pol.MaxFilesPerSubject {
					lateReason = RejectCountReached
				}
			}
			if lateReason != "" {
				// The clean copy is already in the private bucket and will never be recorded: remove it
				// now, and say in the trail whether that worked (citizen-media is purgeable; records are not).
				removed := uc.objects.PurgeAllVersions(ctx, storage.BucketPrivate, f.ObjectKey) == nil
				if err := uc.files.Transition(ctx, tx, id, domain.StoredFilePending, domain.StoredFileRejected, now); err != nil {
					return err
				}
				return writePhotoAudit(ctx, tx, citizen, lp.MaTraCuu, ActionPetitionPhotoRejected, now, map[string]any{
					"tep_id": id, "ly_do": lateReason, "da_xoa_tep_tam": insp.tempRemoved, "da_xoa_ban_sach": removed,
				})
			}
			if err := uc.files.Transition(ctx, tx, id, domain.StoredFilePending, domain.StoredFileScanning, now); err != nil {
				return err
			}
			if err := uc.files.MarkStored(ctx, tx, id, insp.facts, now); err != nil {
				return err
			}
			stored = *cur
			stored.Status, stored.MIMEType, stored.SizeBytes, stored.SHA256, stored.UpdatedAt =
				domain.StoredFileStored, insp.facts.MIMEType, insp.facts.SizeBytes, insp.facts.SHA256, now
			return writePhotoAudit(ctx, tx, citizen, lp.MaTraCuu, ActionPetitionPhotoStored, now, map[string]any{
				"tep_id": id, "loai_tai_len": insp.sourceMIME, "loai_tep": insp.facts.MIMEType,
				"kich_thuoc": insp.facts.SizeBytes, "sha256": insp.facts.SHA256,
				"ma_hoa_lai_bo_exif": true, "da_xoa_tep_tam": insp.tempRemoved, "khoi_phuc_tu_dich": insp.recovered,
			})
		case outcomeRejected:
			if err := uc.files.Transition(ctx, tx, id, domain.StoredFilePending, domain.StoredFileRejected, now); err != nil {
				return err
			}
			d := map[string]any{"tep_id": id, "ly_do": insp.reason, "da_xoa_tep_tam": insp.tempRemoved}
			if insp.signature != "" {
				d["chu_ky_ma_doc"] = insp.signature
			}
			return writePhotoAudit(ctx, tx, citizen, lp.MaTraCuu, ActionPetitionPhotoRejected, now, d)
		case outcomeExpired:
			if err := uc.files.Transition(ctx, tx, id, domain.StoredFilePending, domain.StoredFileFailed, now); err != nil {
				return err
			}
			return writePhotoAudit(ctx, tx, citizen, lp.MaTraCuu, ActionPetitionPhotoExpired, now,
				map[string]any{"tep_id": id})
		}
		return fmt.Errorf("ảnh hiện trường: kết quả kiểm tra không rõ (%d)", insp.kind)
	})
	if err != nil {
		return domain.StoredFile{}, bocPhieu(ctx, "hoàn tất ảnh hiện trường", err)
	}
	switch {
	case stored.ID != "":
		return stored, nil
	case lateReason == RejectPetitionMoved:
		return domain.StoredFile{}, ErrPhotoWindowClosed
	case lateReason != "":
		return domain.StoredFile{}, &AttachmentRejection{Reason: lateReason}
	case insp.kind == outcomeRejected:
		return domain.StoredFile{}, &AttachmentRejection{Reason: insp.reason}
	default: // outcomeExpired
		return domain.StoredFile{}, ErrUploadExpired
	}
}

// photoUploadExts are the temp-key extensions an upload of this petition photo can have been issued
// with — one per type photoTypeAllowed admits. The row stores only the DESTINATION key (`.jpg`), so the
// declared type is found again by looking: the object id is fresh and the form fixed one key, so at most
// one of these exists.
func photoUploadExts(pol uploadpolicy.Policy) []string {
	var out []string
	for _, m := range []string{imaging.MIMEJPEG, imaging.MIMEPNG, imaging.MIMEWebP} {
		if !photoTypeAllowed(pol, m) {
			continue
		}
		if ext, ok := storage.ExtForMIME(m); ok {
			out = append(out, ext)
		}
	}
	return out
}

// inspect is the lock-free half of Complete. An error means nothing may be decided yet (scanner down,
// object replaced, store failure): nothing is written and the row stays `pending`, retryable.
func (uc *CitizenPetitionPhotos) inspect(ctx context.Context, f domain.StoredFile, dst storage.Key,
	pol uploadpolicy.Policy) (photoInspection, error) {

	var (
		uploadKey, upExt string
		st               storage.ObjectInfo
	)
	for _, ext := range photoUploadExts(pol) {
		k, err := uploadKeyFor(dst, ext)
		if err != nil {
			return photoInspection{}, fmt.Errorf("ảnh hiện trường: dựng khoá tải lên: %w", err)
		}
		info, err := uc.objects.Stat(ctx, storage.BucketTemp, k)
		if errors.Is(err, storage.ErrNotFound) {
			continue
		}
		if err != nil {
			return photoInspection{}, storageErr("đọc thông tin tệp tạm", err)
		}
		uploadKey, upExt, st = k, ext, info
		break
	}
	if uploadKey == "" {
		return uc.fromDestination(ctx, f)
	}

	// The CURRENT policy decides (platform.proto (c)): a limit tightened since the request rejects here.
	if st.Size <= 0 || st.Size > pol.MaxBytes {
		return uc.reject(ctx, uploadKey, RejectTooLarge, ""), nil
	}
	// The bytes are read ONCE, bound to the ETag, and everything below — sniff, scan, decode — runs on
	// that one copy, so the bytes scanned are exactly the bytes re-encoded. At most pol.MaxBytes (10 MB).
	rc, _, err := uc.objects.Open(ctx, storage.BucketTemp, uploadKey, st.ETag)
	if err != nil {
		return photoInspection{}, storageErr("mở tệp tạm", err)
	}
	data, err := io.ReadAll(io.LimitReader(rc, pol.MaxBytes+1))
	closeErr := rc.Close()
	if err != nil {
		return photoInspection{}, storageErr("đọc tệp tạm", err)
	}
	if closeErr != nil {
		return photoInspection{}, storageErr("đóng tệp tạm", closeErr)
	}
	if int64(len(data)) != st.Size {
		// What was read is not the object Stat measured: it changed underneath. Inspect again.
		return photoInspection{}, fmt.Errorf("%w: %w", ErrUploadChanged, storage.ErrChanged)
	}
	mime, ext, ok := storage.SniffMIME(data[:min(len(data), storage.SniffBytes)])
	if !ok || !photoTypeAllowed(pol, mime) {
		return uc.reject(ctx, uploadKey, RejectTypeNotAllowed, ""), nil
	}
	if ext != upExt {
		return uc.reject(ctx, uploadKey, RejectTypeMismatch, ""), nil
	}
	// BEFORE the scan and the decode: the cheap refusal first. Re-checked under the lock at the end.
	have, err := uc.files.CountForSubject(ctx, domain.StoredFileSubjectPetition, f.SubjectID,
		domain.PurposePetitionPhoto, time.Time{})
	if err != nil {
		return photoInspection{}, err
	}
	if have >= pol.MaxFilesPerSubject {
		return uc.reject(ctx, uploadKey, RejectCountReached, ""), nil
	}

	// SCANNED BEFORE IT IS DECODED: an image decoder is attack surface, and ADR 0052 §9 lets nothing
	// leave temp unscanned. Fail closed: no scan, no photo.
	res, err := uc.scanner.Scan(ctx, bytes.NewReader(data), int64(len(data)))
	if err != nil {
		if errors.Is(err, malwarescan.ErrNotConfigured) {
			return photoInspection{}, fmt.Errorf("%w: %w", ErrUploadNotConfigured, err)
		}
		return photoInspection{}, fmt.Errorf("%w: %w", ErrScanUnavailable, err)
	}
	if !res.Clean {
		return uc.reject(ctx, uploadKey, RejectMalware, res.Signature), nil
	}

	jpg, reason, err := uc.reencode(ctx, mime, data)
	if err != nil {
		return photoInspection{}, err
	}
	if reason != "" {
		return uc.reject(ctx, uploadKey, reason, ""), nil
	}
	prod, err := uc.objects.PutServerProduced(ctx, dst, bytes.NewReader(jpg), int64(len(jpg)))
	switch {
	case err == nil:
	case errors.Is(err, storage.ErrExists):
		// A previous completion wrote it (dst carries a fresh server-side id): measure the destination.
		return uc.fromDestination(ctx, f)
	default:
		return photoInspection{}, storageErr("ghi ảnh đã mã hoá lại", err)
	}
	// THE RAW UPLOAD GOES NOW (ADR 0052 (b)). A failed delete is recorded, not hidden; the temp bucket's
	// lifecycle removes it within a day.
	removed := uc.objects.PurgeAllVersions(ctx, storage.BucketTemp, uploadKey) == nil
	return photoInspection{kind: outcomeStored, sourceMIME: mime, tempRemoved: removed,
		facts: domain.StoredFileFacts{MIMEType: prod.ContentType, SizeBytes: prod.Size, SHA256: prod.SHA256}}, nil
}

// reencode is the G3 step: decode, orient, fit, re-encode JPEG with no metadata. A refusal of THE IMAGE
// is a reason (422 to the citizen); only a cancelled context is an error.
func (uc *CitizenPetitionPhotos) reencode(ctx context.Context, mime string, data []byte) ([]byte, string, error) {
	select {
	case uc.decodeSlot <- struct{}{}:
		defer func() { <-uc.decodeSlot }()
	case <-ctx.Done():
		return nil, "", ctx.Err()
	}
	h, err := imaging.ReadHeader(mime, data[:min(len(data), imaging.HeadBytes)])
	if err != nil {
		return nil, RejectUndecodable, nil
	}
	if imaging.DecodeCost(mime, h) > photoDecodeBudget {
		return nil, RejectTooManyPixels, nil
	}
	img, err := imaging.Decode(mime, bytes.NewReader(data))
	if err != nil {
		return nil, RejectUndecodable, nil
	}
	jpg, err := imaging.RenderJPEG(img, h.Orientation, photoMaxSide, photoJPEGQuality)
	if err != nil || int64(len(jpg)) > storage.MaxServerProducedBytes {
		return nil, RejectUndecodable, nil
	}
	return jpg, "", nil
}

// fromDestination handles "no upload in temp": a previous completion wrote the clean copy (the write
// happened, the transaction did not — measure it and record it), or nothing arrived.
//
// A destination object is TRUSTED AS CLEAN because nothing else writes there: PutServerProduced on a
// fresh server-side id is the only path into `citizen-media/…/petitions/petition-photo/…`, it runs only
// after a clean scan and a re-encode, and IAM scopes this service's key to its own subtree.
func (uc *CitizenPetitionPhotos) fromDestination(ctx context.Context, f domain.StoredFile) (photoInspection, error) {
	st, err := uc.objects.Stat(ctx, storage.BucketPrivate, f.ObjectKey)
	if errors.Is(err, storage.ErrNotFound) {
		if uc.clock().After(f.CreatedAt.Add(storage.UploadTTL)) {
			return photoInspection{kind: outcomeExpired}, nil
		}
		return photoInspection{kind: outcomeNotReceived}, nil
	}
	if err != nil {
		return photoInspection{}, storageErr("đọc thông tin ảnh đã lưu", err)
	}
	head, err := uc.objects.ReadHead(ctx, storage.BucketPrivate, f.ObjectKey, st.ETag, storage.SniffBytes)
	if err != nil {
		return photoInspection{}, storageErr("đọc đầu ảnh đã lưu", err)
	}
	if mime, _, ok := storage.SniffMIME(head); !ok || mime != imaging.MIMEJPEG {
		// PutServerProduced refuses exactly this, so it cannot be ours. Not recorded; an operator looks.
		return photoInspection{}, errors.New("ảnh hiện trường: đối tượng ở kho lưu không phải JPEG đã mã hoá lại")
	}
	sum, err := uc.objects.SHA256(ctx, storage.BucketPrivate, f.ObjectKey, st.ETag)
	if err != nil {
		return photoInspection{}, storageErr("tính sha256 ảnh đã lưu", err)
	}
	return photoInspection{kind: outcomeStored, recovered: true,
		facts: domain.StoredFileFacts{MIMEType: imaging.MIMEJPEG, SizeBytes: st.Size, SHA256: sum}}, nil
}

// reject deletes the temp object and reports the outcome; a failed delete is recorded, not hidden.
func (uc *CitizenPetitionPhotos) reject(ctx context.Context, uploadKey, reason, signature string) photoInspection {
	removed := uc.objects.PurgeAllVersions(ctx, storage.BucketTemp, uploadKey) == nil
	return photoInspection{kind: outcomeRejected, reason: reason, signature: signature, tempRemoved: removed}
}

// --- the citizen's own list -------------------------------------------------------------------------

// ListPhotos returns the stored photos of the citizen's own petition `ma`, each with a short-lived signed
// link, issued only AFTER the session identity and commune matched the petition (rule 4, invariant 7;
// ADR 0052 §12). Any status: the citizen sees their own photos after the petition moved on, too.
//
// NOT AUDITED: the citizen reading their own petition, as GET /api/v1/my-citizen-reports/{code} is not.
func (uc *CitizenPetitionPhotos) ListPhotos(ctx context.Context, ma string, citizen audit.Actor) ([]PhotoLink, error) {
	if err := citizenOnly(citizen); err != nil {
		return nil, err
	}
	if uc.objects == nil {
		return nil, ErrUploadNotConfigured
	}
	p, err := uc.petitions.CuaCongDanTheoMaTraCuu(ctx, citizen.ID, ma)
	if err != nil {
		return nil, bocPhieu(ctx, "đọc ảnh hiện trường", err)
	}
	return signPhotos(ctx, uc.files, uc.objects, p.ID, uc.clock())
}

// signPhotos reads a petition's photos and signs one GET per photo, for MaxDownloadTTL (15 minutes).
func signPhotos(ctx context.Context, files photoLister, objects PhotoObjectStore, petitionID string,
	now time.Time) ([]PhotoLink, error) {
	photos, err := files.PetitionPhotos(ctx, petitionID)
	if err != nil {
		return nil, bocPhieu(ctx, "đọc ảnh hiện trường", err)
	}
	out := make([]PhotoLink, 0, len(photos))
	for _, f := range photos {
		if f.Bucket != domain.StoredFileBucketPrivate || !domain.IsCitizenPhotoOf(f, petitionID) {
			continue // never sign anything that is not exactly what migration 0026 shapes
		}
		u, err := objects.PresignDownload(ctx, storage.BucketPrivate, f.ObjectKey, storage.MaxDownloadTTL,
			f.OriginalName)
		if err != nil {
			return nil, storageErr("ký liên kết xem ảnh", err)
		}
		out = append(out, PhotoLink{File: f, URL: u, ExpiresAt: now.Add(storage.MaxDownloadTTL)})
	}
	return out, nil
}

// --- the staff read ---------------------------------------------------------------------------------

// StaffPhotoPetitions is the staff register's read — a SEPARATE type from the citizen one, so the two
// surfaces never share a reader (rule 4, invariant 5).
type StaffPhotoPetitions interface {
	// vi-name-ok: mirrors the existing PhieuPhanAnhStore method; rule 12 invariant 3 keeps existing names
	TheoMaTraCuu(ctx context.Context, ma string) (domain.PhieuPhanAnh, error)
}

// StaffPetitionPhotos lists a petition's citizen photos to a member of staff who may read the petition.
type StaffPetitionPhotos struct {
	petitions StaffPhotoPetitions
	files     photoLister
	objects   PhotoObjectStore
	now       func() time.Time
}

// NewStaffPetitionPhotos builds the staff read. objects may be UNTYPED nil (not configured → 503).
func NewStaffPetitionPhotos(petitions StaffPhotoPetitions, files PetitionPhotoFiles,
	objects PhotoObjectStore) *StaffPetitionPhotos {
	return &StaffPetitionPhotos{petitions: petitions, files: files, objects: objects}
}

// ListPhotos answers the photos of petition `ma` in this commune with signed links.
//
// `mayReadRestricted` is whether the reader holds `feedback.restricted`, decided by the handler's
// checker. A `can-bo` petition without it is ErrPhieuKhongTonTai — the SAME answer GET
// /api/v1/citizen-reports/{code} gives, so this route cannot confirm such a report exists.
//
// NOT AUDITED, stated rather than assumed, on the footing of the petition read itself: a member of
// staff with `feedback.read`, in their own commune, reading a petition's content — which is not audited
// on GET /api/v1/citizen-reports/{code} either (only the unmasked contact details are, rule 6
// invariant 7, ADR 0030). A photo cannot be masked, so whether opening one counts as reading "full
// personal data" is put to the owner as an open question.
func (uc *StaffPetitionPhotos) ListPhotos(ctx context.Context, ma string, mayReadRestricted bool) ([]PhotoLink, error) {
	if uc.objects == nil {
		return nil, ErrUploadNotConfigured
	}
	p, err := uc.petitions.TheoMaTraCuu(ctx, ma)
	if err != nil {
		return nil, bocPhieu(ctx, "đọc ảnh hiện trường", err)
	}
	if p.LinhVuc == domain.LinhVucHanChe && !mayReadRestricted {
		return nil, bocPhieu(ctx, "đọc ảnh hiện trường", petstore.ErrPhieuKhongTonTai)
	}
	now := time.Now().UTC()
	if uc.now != nil {
		now = uc.now().UTC()
	}
	return signPhotos(ctx, uc.files, uc.objects, p.ID, now)
}

// Compile-time proof the real dependencies satisfy the interfaces.
var (
	_ PhotoObjectStore      = (*storage.Client)(nil)
	_ PetitionPhotoFiles    = (*petstore.StoredFileStore)(nil)
	_ CitizenPhotoPetitions = (*petstore.PhieuPhanAnhStore)(nil)
	_ StaffPhotoPetitions   = (*petstore.PhieuPhanAnhStore)(nil)
)
