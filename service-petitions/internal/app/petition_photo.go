package app

// A CITIZEN'S SCENE PHOTOS ON THEIR OWN PETITION — menu phan-anh-nguoi-dan (docs/ui-ux/09 §8.4,
// "TRƯỚC KHI XỬ LÝ"), owner decisions of 02/10/2026 (ADR 0047 row "Ảnh hiện trường khi gửi phản ánh",
// and G3), storage of migration 0026. ONE multipart request through this service (ADR 0052 §Sửa đổi
// 09/10/2026 — the phone no longer posts to MinIO), in three steps inside Upload (upload_stream.go):
//
//	a. reserve   pending row + trail                                  the petition locked, ≤ 5, open
//	b. receive   PutUpload streams the bytes into temp                never buffered whole in this pod
//	c. complete  stat · sniff · scan · DECODE · ORIENT · RE-ENCODE JPEG WITHOUT EXIF ·
//	             PutServerProduced into private · purge temp · `stored` + trail in ONE transaction
//
// THE RAW UPLOAD NEVER REACHES THE PRIVATE BUCKET. It carries EXIF — the GPS of the citizen's home,
// device identifiers, the capture time (rule 3) — so step (c) does not Promote it, as the task path
// does: it stores only the re-encoded bytes, as variant `original` of class `citizen-media`
// (core/storage.PutServerProduced (b)), and deletes the temp object. WebP and PNG come out as JPEG.
//
// # WHO
//
// The OWNER who filed the petition, from the SESSION (rule 4, invariant 2): every petition read here
// filters by the owner column — `cong_dan_id = <session citizen>`, or since ADR 0080 decision 8
// `zalo_account_id = <session Zalo account>` for an unverified petition — and the commune of the
// session, so another owner's code, another commune's and an unknown one are one ErrPhieuKhongTonTai —
// one 404 body. `stored_file` does not hold the owner (migrations 0026, 0032 say why); a file belongs to
// the caller because it hangs off a petition that does.
//
// # WHEN — ONLY WHILE THE PETITION IS `da-tiep-nhan` (domain.PhotoUploadOpen)
//
// Checked when the row is reserved and again, under the petition's row lock, when the photo is stored.
// A photo whose petition moved on while it was being processed is RECORDED as refused, and the clean
// copy already written is removed, so the set staff are reading never grows behind them.
//
// # HOW MANY — platform's `max_files_per_subject` for `petition-photo`, counted under the lock
//
// The count includes pending uploads younger than storage.UploadTTL, inside the transaction that holds
// the petition FOR UPDATE, so concurrent requests cannot pass at 4 (the task path's CountForSubjectTx).
// Migration 0026's trigger is the floor under it for stored photos. A policy with NO count limit is
// treated as NOT CONFIGURED (503): the owner's ceiling is 5, and an unlimited policy is a misconfigured
// one, not permission to issue unlimited slots.
//
// # WHAT IS NEVER LOGGED OR PUT IN AN ERROR
//
// The lookup code, the citizen id, any presigned URL, image bytes. Errors carry the
// commune (bocPhieu) and the act. The trail carries the file id, the sniffed types, size and hash.
//
// # NOT CHECKED, BY DECISION — THE APP THE SESSION CAME FROM
//
// The citizen session this service sees (core/httpx.CitizenSession: session id, citizen id, commune,
// and since 7c0e55c9 the Zalo account that opened it — ADR 0080; identity.proto CitizenSessionPrincipal)
// records nothing about which APP opened it: the Zalo account is the session's owner, not its app.
// Asked 02/10/2026, the owner answered that a photo may come from either app ("mở ở đâu cũng
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
	"sync"
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
	// photoReadSlots is how many completions in this process may hold an upload's bytes in memory at
	// once — from the read of the temp object until inspect returns. Each holds at most pol.MaxBytes
	// (10 MB today). Before 2026-10-02 the bytes were read BEFORE waiting on decodeSlot, and the then
	// completion route was idem.KhongCan, so 25 parallel completions buffered 250 MB in a 384 MiB pod.
	//
	// THE UPLOAD ITSELF (ADR 0052 §Sửa đổi 09/10/2026) adds little: at most httpx.UploadSlotsPerPod (4)
	// uploads stream through the pod at once, each holding core/storage's 64 KiB copy buffer and the
	// multipart reader's — never the file. The file is held whole only here, under this slot.
	//
	// THE WORST CASE THIS PROCESS NOW REACHES, all of it bounded:
	//
	//	2 buffered uploads             2 × 10 MB              20 MB
	//	1 decode (decodeSlot)          ≤ photoDecodeBudget   160 MiB   (imaging.DecodeCost, measured)
	//	the render of that decode      2560² RGBA × 2 (scale, orient) + the JPEG out    ~55 MB
	//	                                                     ≈ 245 MB of the pod's 384 MiB
	//
	// The ~140 MiB left is the rest of the service (pgx, gRPC, every other route) and the GC's headroom:
	// without GOMEMLIMIT the Go heap may grow to twice its live size before collecting, so this table is
	// not by itself a proof the pod cannot be OOM-killed — see the report of TASK-03b. Raising this, the
	// policy's MaxBytes, or the decode budget means raising the pod limit with it.
	photoReadSlots = 2
)

// errPhotoCompletionAborted is what a waiting completion receives if the one doing the work never
// recorded a result (it panicked). Never a zero StoredFile with a nil error: that would answer 200.
var errPhotoCompletionAborted = errors.New("ảnh hiện trường: lượt hoàn tất song song dừng giữa chừng")

var (
	// ErrPhotoNotFound: no such photo ON THIS PETITION — unknown id, another petition's, another
	// commune's, a staff file. One answer for all. 404.
	ErrPhotoNotFound = errors.New("ảnh hiện trường: không tìm thấy")
	// ErrPhotoWindowClosed: the petition is no longer `da-tiep-nhan`. 409.
	ErrPhotoWindowClosed = errors.New("ảnh hiện trường: phiếu đã chuyển bước, không đính thêm ảnh được")
	// ErrPhotoCountReached: the petition already holds platform's maximum (uploads in flight included). 409.
	ErrPhotoCountReached = errors.New("ảnh hiện trường: phiếu đã đủ số ảnh tối đa")
	// ErrPhotoTypeNotAllowed: the DECLARED type is not an image type the policy and the re-encoder
	// both accept. 400, before any row.
	ErrPhotoTypeNotAllowed = errors.New("ảnh hiện trường: loại tệp không được phép")
	// ErrPhotoTooLarge: the DECLARED size is above platform's limit. 413, before any row.
	ErrPhotoTooLarge = errors.New("ảnh hiện trường: ảnh vượt dung lượng cho phép")
	// ErrPhotoSizeInvalid: the declared size is not a positive number of bytes. 400.
	ErrPhotoSizeInvalid = errors.New("ảnh hiện trường: kích thước khai báo không hợp lệ")
)

// PhotoObjectStore is the part of *storage.Client this flow calls — the task path's ObjectStore minus
// Promote (the raw upload is never promoted) plus PutServerProduced.
type PhotoObjectStore interface {
	PutUpload(ctx context.Context, uploadKey string, r io.Reader, size, maxBytes int64,
		contentType string) (storage.ObjectInfo, error)
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

// OwnedPhotoPetitions is the two OWNER-FILTERED petition reads the scene photos use — a verified
// citizen's petition or, since ADR 0080, a Zalo account's — never the staff register's unfiltered one,
// which this type cannot reach (rule 4, invariant 5).
type OwnedPhotoPetitions interface {
	OwnedByCode(ctx context.Context, owner domain.PetitionOwner, ma string) (domain.PhieuPhanAnh, error)
	OwnedForUpdate(ctx context.Context, tx *store.ScopedTx, owner domain.PetitionOwner, ma string) (
		domain.PhieuPhanAnh, error)
}

// CitizenPhotoPetitions is the two CITIZEN-IDENTITY-FILTERED petition reads — never the staff
// register's unfiltered one, which this type cannot reach (rule 4, invariant 5). The verification-photo
// read uses it; its route stays verified-phone only (XaTuPhien).
type CitizenPhotoPetitions interface {
	// vi-name-ok: mirrors the existing PhieuPhanAnhStore method; rule 12 invariant 3 keeps existing names
	CuaCongDanTheoMaTraCuu(ctx context.Context, congDanID, ma string) (domain.PhieuPhanAnh, error)
	CitizenPetitionForUpdate(ctx context.Context, tx *store.ScopedTx, congDanID, ma string) (
		domain.PhieuPhanAnh, error)
}

// PhotoUploadRequest is what the multipart upload declares beside the file. NO FILE NAME
// (domain.PetitionPhotoName says why — the part's file name is ignored). Both values are claims: the
// size is held to exactly by the stream, the type is checked again on the bytes at completion.
type PhotoUploadRequest struct {
	ContentType string
	Size        int64
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
	petitions OwnedPhotoPetitions
	files     PetitionPhotoFiles

	// ANY OF THE THREE nil means "not configured": every upload is refused with ErrUploadNotConfigured
	// (503) while the petition intake keeps serving. The list needs only `objects`.
	objects  PhotoObjectStore
	scanner  MalwareScanner
	policies UploadPolicies

	// The memory bounds of this process's image work — SHARED with the staff verification photos
	// (petition_verification_photo.go), which re-encode on the same pod against the same budget.
	*photoSlots

	newID func() (string, error)
	now   func() time.Time
}

// photoSlots are the PROCESS-WIDE bounds on image work, one value per process shared by every flow
// that decodes an upload (the citizen's scene photo, staff's verification photo). Two flows with a
// slot each would be two decode budgets against one 384 MiB pod — the exact arithmetic photoReadSlots
// lays out, silently doubled.
type photoSlots struct {
	// decodeSlot admits ONE decode at a time in this process: photoDecodeBudget is a per-decode bound,
	// and two at once would be two budgets against one pod.
	decodeSlot chan struct{}
	// readSlot admits photoReadSlots uploads into memory at a time. Always taken BEFORE decodeSlot, so
	// the two cannot deadlock.
	readSlot chan struct{}

	// inflight de-duplicates concurrent completions of ONE file in this process (key: commune + file id):
	// the second caller waits for the first one's answer instead of reading a second copy of the bytes.
	mu       sync.Mutex
	inflight map[string]*photoCompletion
}

func newPhotoSlots() *photoSlots {
	return &photoSlots{decodeSlot: make(chan struct{}, 1), readSlot: make(chan struct{}, photoReadSlots),
		inflight: map[string]*photoCompletion{}}
}

// photoCompletion is one completion in progress. file and err are written before done is closed.
type photoCompletion struct {
	done    chan struct{}
	file    domain.StoredFile
	err     error
	waiters int // callers waiting on done; read by the tests to know the race is set up
}

// NewCitizenPetitionPhotos builds the use case. Pass UNTYPED nil for a dependency that is not configured
// (a nil *storage.Client inside a non-nil interface would pass the nil check and panic on first use).
func NewCitizenPetitionPhotos(db *store.DB, petitions OwnedPhotoPetitions, files PetitionPhotoFiles,
	objects PhotoObjectStore, scanner MalwareScanner, policies UploadPolicies) *CitizenPetitionPhotos {
	return &CitizenPetitionPhotos{db: db, petitions: petitions, files: files, objects: objects,
		scanner: scanner, policies: policies, photoSlots: newPhotoSlots(), newID: storage.NewObjectID}
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

// photoOwner is the wall the three scene-photo acts put first: the session actor mapped onto the
// petition owner it must match — a verified citizen, or a Zalo account (ADR 0080 decision 8: an
// unverified petition HAS scene photos, owned by its account). Any other actor, or an empty id, means
// the route was mounted wrong; the kind is printed, the id never is.
//
// AN EXPLICIT SWITCH ON THE KIND, never "the id is the id": a Zalo account id read as a citizen id
// would match no petition and look like "not found" — or, worse, the reverse.
func photoOwner(actor audit.Actor) (domain.PetitionOwner, error) {
	o := domain.PetitionOwner{ID: actor.ID}
	switch actor.Kind {
	case "citizen":
		o.Kind = domain.OwnerCitizen
	case audit.KindZaloAccount:
		o.Kind = domain.OwnerZaloAccount
	}
	if !o.Valid() {
		return domain.PetitionOwner{}, fmt.Errorf("anh_hien_truong: chủ thể không phải công dân cũng không phải "+
			"tài khoản Zalo chủ phiếu (kind=%q) — tuyến thiếu authz.CitizenOnly hoặc lớp xã từ phiên", actor.Kind)
	}
	return o, nil
}

// photoPolicy reads platform's limit for petition photos. Not configured, unreachable, and a policy
// with no per-petition count are all refusals — there is no default anywhere (ADR 0052 stop #4).
func (uc *CitizenPetitionPhotos) photoPolicy(ctx context.Context) (uploadpolicy.Policy, error) {
	return photoPolicyFor(ctx, uc.policies, storage.PurposePetitionPhoto)
}

// photoPolicyFor is photoPolicy for any photo purpose of a petition — the citizen's scene photo, staff's
// verification photo. The owner's ceiling is 5 for both, so a policy with no count is NOT CONFIGURED.
func photoPolicyFor(ctx context.Context, policies UploadPolicies, purpose storage.Purpose) (
	uploadpolicy.Policy, error) {
	p, ok, err := policies.Policy(ctx, purpose)
	switch {
	case errors.Is(err, uploadpolicy.ErrUnavailable):
		return uploadpolicy.Policy{}, fmt.Errorf("%w: %w", ErrUploadLimitsUnavailable, err)
	case err != nil:
		return uploadpolicy.Policy{}, fmt.Errorf("ảnh phiếu: đọc giới hạn tải tệp: %w", err)
	case !ok:
		return uploadpolicy.Policy{}, fmt.Errorf("%w: platform has no limit for %s",
			ErrUploadNotConfigured, purpose)
	case !p.FileCountLimited || p.MaxFilesPerSubject <= 0:
		return uploadpolicy.Policy{}, fmt.Errorf("%w: the %s policy has no per-petition file count",
			ErrUploadNotConfigured, purpose)
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

// uploadKeyFor is the temp key the upload is streamed to: the destination key with the DECLARED type's
// extension (core/storage.PutUpload requires the two to match).
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

// --- the upload: reserve · receive · complete ------------------------------------------------------

// MaxUploadBytes is the policy's cap on one photo, for the handler to bound the request body BEFORE it
// reads it (core/httpx.UploadOptions.MaxFileBytes). The same refusals as Upload: not configured,
// platform unreachable, no per-petition count — there is no default (ADR 0052 stop #4).
func (uc *CitizenPetitionPhotos) MaxUploadBytes(ctx context.Context) (int64, error) {
	if !uc.uploadsConfigured() {
		return 0, ErrUploadNotConfigured
	}
	pol, err := uc.photoPolicy(ctx)
	if err != nil {
		return 0, err
	}
	return pol.MaxBytes, nil
}

// Upload stores one photo on the citizen's own petition `ma`: the row is reserved (a), the bytes are
// streamed into temp (b), and the completion (c) runs on them — one request, one answer: the STORED
// photo, or a refusal. A failure after (a) closes the reserved row (`failed`, with its trail), so it
// stops counting against the 5 at once.
func (uc *CitizenPetitionPhotos) Upload(ctx context.Context, ma string, req PhotoUploadRequest, body UploadBody,
	citizen audit.Actor) (domain.StoredFile, error) {

	res, err := uc.reserve(ctx, ma, req, citizen)
	if err != nil {
		return domain.StoredFile{}, err
	}
	closer := uploadCloser{db: uc.db, objects: uc.objects, files: uc.files, actor: citizen,
		failAction: ActionPetitionPhotoExpired, clock: uc.clock}
	if err := closer.receive(ctx, res, body); err != nil {
		return domain.StoredFile{}, err
	}
	f, err := uc.complete(ctx, ma, res.file.ID, citizen)
	if err != nil {
		return domain.StoredFile{}, closer.afterComplete(ctx, res, err)
	}
	return f, nil
}

// reserve is step (a): one pending row for one photo on the citizen's own petition `ma`.
//
// ONE TRANSACTION: the petition read FOR UPDATE with both isolation axes (it also serialises two
// uploads on one petition), the status, the count with live pending rows, the pending row and the
// audit entry.
func (uc *CitizenPetitionPhotos) reserve(ctx context.Context, ma string, req PhotoUploadRequest,
	citizen audit.Actor) (uploadReservation, error) {

	owner, err := photoOwner(citizen)
	if err != nil {
		return uploadReservation{}, err
	}
	if req.Size <= 0 {
		return uploadReservation{}, ErrPhotoSizeInvalid
	}
	if !uc.uploadsConfigured() {
		return uploadReservation{}, ErrUploadNotConfigured
	}
	pol, err := uc.photoPolicy(ctx)
	if err != nil {
		return uploadReservation{}, err
	}
	if !photoTypeAllowed(pol, req.ContentType) {
		return uploadReservation{}, ErrPhotoTypeNotAllowed
	}
	if req.Size > pol.MaxBytes {
		return uploadReservation{}, ErrPhotoTooLarge
	}
	upExt, ok := storage.ExtForMIME(req.ContentType)
	if !ok {
		return uploadReservation{}, ErrPhotoTypeNotAllowed
	}

	id, err := uc.newID()
	if err != nil {
		return uploadReservation{}, fmt.Errorf("ảnh hiện trường: sinh mã tệp: %w", err)
	}
	now := uc.clock()
	dst := photoKey(ctx, id, now)
	objectKey, err := dst.Path()
	if err != nil {
		return uploadReservation{}, fmt.Errorf("ảnh hiện trường: dựng khoá đối tượng: %w", err)
	}
	uploadKey, err := uploadKeyFor(dst, upExt)
	if err != nil {
		return uploadReservation{}, fmt.Errorf("ảnh hiện trường: dựng khoá tải lên: %w", err)
	}

	res := uploadReservation{uploadKey: uploadKey, contentType: req.ContentType, size: req.Size,
		maxBytes: pol.MaxBytes}
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		p, err := uc.petitions.OwnedForUpdate(ctx, tx, owner, ma)
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
		if err := writePhotoAudit(ctx, tx, citizen, p.MaTraCuu, ActionPetitionPhotoRequested, now, map[string]any{
			"tep_id": id, "muc_dich": domain.PurposePetitionPhoto,
			"loai_khai_bao": req.ContentType, "kich_thuoc_khai": req.Size,
		}); err != nil {
			return err
		}
		res.file, res.subject = f, p.MaTraCuu
		return nil
	})
	if err != nil {
		return uploadReservation{}, bocPhieu(ctx, "xin tải ảnh hiện trường", err)
	}
	return res, nil
}

// --- c. complete ------------------------------------------------------------------------------------

// photoInspection is what the lock-free half of complete found.
type photoInspection struct {
	kind        outcomeKind
	facts       domain.StoredFileFacts // of the STORED (re-encoded) object
	sourceMIME  string                 // the sniffed type of the upload, for the trail
	recovered   bool
	reason      string
	signature   string
	tempRemoved bool
	// tempKeys are the temp keys looked at and found empty — set when nothing was received, so the
	// error in the log names WHERE the upload was expected (this service's own temp write not being
	// visible there is a store fault). Object keys only: commune and file id, no person.
	tempKeys []string
}

// complete runs step (c) for one upload of the citizen's own petition `ma` — Upload calls it on the
// bytes it has just streamed; there is no completion route any more (ADR 0052 §Sửa đổi 09/10/2026).
//
// The object-store and pixel work happens lock-free first (task_attachment.go says why); then ONE short
// transaction re-reads the petition and the row FOR UPDATE, re-checks the status and the count under the
// lock, and records the outcome with its trail.
//
// IDEMPOTENT: a photo already stored answers as stored.
func (uc *CitizenPetitionPhotos) complete(ctx context.Context, ma, id string, citizen audit.Actor) (
	domain.StoredFile, error) {

	owner, err := photoOwner(citizen)
	if err != nil {
		return domain.StoredFile{}, err
	}
	if !uc.uploadsConfigured() {
		return domain.StoredFile{}, ErrUploadNotConfigured
	}

	// 1. Lock-free pre-read: the caller's own petition, a photo of it, and something left to do.
	p, err := uc.petitions.OwnedByCode(ctx, owner, ma)
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
	// ONLY NOW, with the caller proved to own this file, may it join another caller's completion: the
	// shared answer goes to nobody who could not have asked for it.
	return uc.completeOnce(ctx, id, func(ctx context.Context) (domain.StoredFile, error) {
		return uc.completePending(ctx, ma, id, citizen, owner, *f)
	})
}

// completeOnce runs complete for (commune, id) unless a completion of the same file is already running
// in this process, in which case it waits for that one and returns ITS answer — the same answer a
// retry after it would get, since a stored photo answers as stored. One read of the bytes, not two.
//
// If the first caller gave up (its context ended) while this one still wants the answer, this one does
// the work itself: the abandoned run wrote nothing it would not find again (fromDestination).
//
// Per process only: a second pod reads its own copy, which its own readSlot bounds.
func (uc *photoSlots) completeOnce(ctx context.Context, id string,
	complete func(context.Context) (domain.StoredFile, error)) (domain.StoredFile, error) {

	key := string(tenant.MustFrom(ctx)) + "/" + id
	uc.mu.Lock()
	if c, ok := uc.inflight[key]; ok {
		c.waiters++
		uc.mu.Unlock()
		select {
		case <-c.done:
		case <-ctx.Done():
			return domain.StoredFile{}, ctx.Err()
		}
		if (errors.Is(c.err, context.Canceled) || errors.Is(c.err, context.DeadlineExceeded)) && ctx.Err() == nil {
			return uc.completeOnce(ctx, id, complete)
		}
		return c.file, c.err
	}
	if uc.inflight == nil {
		uc.inflight = map[string]*photoCompletion{}
	}
	c := &photoCompletion{done: make(chan struct{}), err: errPhotoCompletionAborted}
	uc.inflight[key] = c
	uc.mu.Unlock()
	defer func() {
		uc.mu.Lock()
		delete(uc.inflight, key)
		uc.mu.Unlock()
		close(c.done)
	}()
	c.file, c.err = complete(ctx)
	return c.file, c.err
}

// completePending is steps 2 and 3 of complete for a file the caller owns and that was `pending`.
func (uc *CitizenPetitionPhotos) completePending(ctx context.Context, ma, id string, citizen audit.Actor,
	owner domain.PetitionOwner, f domain.StoredFile) (domain.StoredFile, error) {

	pol, err := uc.photoPolicy(ctx)
	if err != nil {
		return domain.StoredFile{}, err
	}
	dst, err := storage.ParseKey(f.ObjectKey)
	if err != nil {
		return domain.StoredFile{}, fmt.Errorf("ảnh hiện trường: khoá đối tượng đã ghi không hợp lệ: %w", err)
	}

	// 2. Object-store and pixel work, no lock held.
	insp, err := uc.inspector().inspect(ctx, f, dst, pol)
	if err != nil {
		return domain.StoredFile{}, err
	}
	if insp.kind == outcomeNotReceived {
		return domain.StoredFile{}, fmt.Errorf("%w: %s", ErrUploadNotReceived, notReceivedDetail(f, insp))
	}

	// 3. One short transaction.
	now := uc.clock()
	var stored domain.StoredFile
	lateReason := ""
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		lp, err := uc.petitions.OwnedForUpdate(ctx, tx, owner, ma)
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
		return domain.StoredFile{}, fmt.Errorf("%w: %s", ErrUploadExpired, notReceivedDetail(f, insp))
	}
}

// notReceivedDetail says, for the log only, where an upload that never arrived was looked for. The
// handler answers the citizen a fixed sentence and never this text.
func notReceivedDetail(f domain.StoredFile, insp photoInspection) string {
	return fmt.Sprintf("không có tệp tạm ở bucket temp (đã tìm %q), chưa có bản sạch ở bucket private (%q); "+
		"xin chỗ tải lúc %s", insp.tempKeys, f.ObjectKey, f.CreatedAt.UTC().Format(time.RFC3339))
}

// photoUploadExts are the temp-key extensions an upload of this petition photo can have been issued
// with — one per type photoTypeAllowed admits. The row stores only the DESTINATION key (`.jpg`), so the
// declared type is found again by looking: the object id is fresh and the upload wrote one key, so at
// most one of these exists.
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

// photoInspector is the lock-free half of a completion that RE-ENCODES the upload — the citizen's scene
// photo and staff's verification photo (petition_verification_photo.go). The count it pre-checks is over
// the ROW's own subject and purpose, so the two photo kinds never consume each other's 5.
type photoInspector struct {
	objects PhotoObjectStore
	scanner MalwareScanner
	files   storedFileCounter
	slots   *photoSlots
	clock   func() time.Time

	// preCount, when set, replaces the pre-check's whole-subject count. The staff verification photo
	// sets it because its cap counts PER PROCESSING ROUND (StaffVerificationPhotos.countThisRound); the
	// citizen photo leaves it nil and keeps counting every stored photo of the petition.
	preCount func(ctx context.Context, f domain.StoredFile) (int, error)
}

func (uc *CitizenPetitionPhotos) inspector() photoInspector {
	return photoInspector{objects: uc.objects, scanner: uc.scanner, files: uc.files, slots: uc.photoSlots,
		clock: uc.clock}
}

// inspect is the lock-free half of complete. An error means nothing may be decided yet (scanner down,
// object replaced, store failure): nothing is written here, and Upload then closes the row (`failed`,
// upload_stream.go) because no completion route is left to retry it from.
func (uc photoInspector) inspect(ctx context.Context, f domain.StoredFile, dst storage.Key,
	pol uploadpolicy.Policy) (photoInspection, error) {

	var (
		uploadKey, upExt string
		st               storage.ObjectInfo
		tried            []string
	)
	for _, ext := range photoUploadExts(pol) {
		k, err := uploadKeyFor(dst, ext)
		if err != nil {
			return photoInspection{}, fmt.Errorf("ảnh hiện trường: dựng khoá tải lên: %w", err)
		}
		info, err := uc.objects.Stat(ctx, storage.BucketTemp, k)
		if errors.Is(err, storage.ErrNotFound) {
			tried = append(tried, k)
			continue
		}
		if err != nil {
			return photoInspection{}, storageErr("đọc thông tin tệp tạm", err)
		}
		uploadKey, upExt, st = k, ext, info
		break
	}
	if uploadKey == "" {
		insp, err := uc.fromDestination(ctx, f)
		insp.tempKeys = tried
		return insp, err
	}

	// The CURRENT policy decides (platform.proto (c)): a limit tightened since the request rejects here.
	if st.Size <= 0 || st.Size > pol.MaxBytes {
		return uc.reject(ctx, uploadKey, RejectTooLarge, ""), nil
	}
	// A READ SLOT BEFORE THE BYTES, held until this function returns (photoReadSlots says why and how
	// much). Waiting here costs nothing but time; reading first and waiting later cost 10 MB per caller.
	select {
	case uc.slots.readSlot <- struct{}{}:
		defer func() { <-uc.slots.readSlot }()
	case <-ctx.Done():
		return photoInspection{}, ctx.Err()
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
	var have int
	if uc.preCount != nil {
		have, err = uc.preCount(ctx, f)
	} else {
		have, err = uc.files.CountForSubject(ctx, f.SubjectType, f.SubjectID, f.Purpose, time.Time{})
	}
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
func (uc photoInspector) reencode(ctx context.Context, mime string, data []byte) ([]byte, string, error) {
	select {
	case uc.slots.decodeSlot <- struct{}{}:
		defer func() { <-uc.slots.decodeSlot }()
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
// fresh server-side id is the only path into `citizen-media/…/petitions/petition-photo/…` and
// `records/…/petitions/petition-verification-photo/…`, it runs only after a clean scan and a re-encode,
// and IAM scopes this service's key to its own subtree.
func (uc photoInspector) fromDestination(ctx context.Context, f domain.StoredFile) (photoInspection, error) {
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
		return photoInspection{}, errors.New("ảnh phiếu: đối tượng ở kho lưu không phải JPEG đã mã hoá lại")
	}
	sum, err := uc.objects.SHA256(ctx, storage.BucketPrivate, f.ObjectKey, st.ETag)
	if err != nil {
		return photoInspection{}, storageErr("tính sha256 ảnh đã lưu", err)
	}
	return photoInspection{kind: outcomeStored, recovered: true,
		facts: domain.StoredFileFacts{MIMEType: imaging.MIMEJPEG, SizeBytes: st.Size, SHA256: sum}}, nil
}

// reject deletes the temp object and reports the outcome; a failed delete is recorded, not hidden.
func (uc photoInspector) reject(ctx context.Context, uploadKey, reason, signature string) photoInspection {
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
	owner, err := photoOwner(citizen)
	if err != nil {
		return nil, err
	}
	if uc.objects == nil {
		return nil, ErrUploadNotConfigured
	}
	p, err := uc.petitions.OwnedByCode(ctx, owner, ma)
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
	return signPhotoList(ctx, objects, photos, func(f domain.StoredFile) bool {
		return domain.IsCitizenPhotoOf(f, petitionID) // exactly what migration 0026 shapes
	}, now)
}

// signPhotoList signs one GET per photo that is in the private bucket AND passes `shaped` — the second
// wall behind the store's own filter, so a row of another purpose that a future query lets through is
// skipped rather than signed.
func signPhotoList(ctx context.Context, objects PhotoObjectStore, photos []domain.StoredFile,
	shaped func(domain.StoredFile) bool, now time.Time) ([]PhotoLink, error) {
	out := make([]PhotoLink, 0, len(photos))
	for _, f := range photos {
		if f.Bucket != domain.StoredFileBucketPrivate || !shaped(f) {
			continue
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
	db        *store.DB
	petitions StaffPhotoPetitions
	files     photoLister
	objects   PhotoObjectStore
	now       func() time.Time
}

// NewStaffPetitionPhotos builds the staff read. objects may be UNTYPED nil (not configured → 503).
func NewStaffPetitionPhotos(db *store.DB, petitions StaffPhotoPetitions, files PetitionPhotoFiles,
	objects PhotoObjectStore) *StaffPetitionPhotos {
	return &StaffPetitionPhotos{db: db, petitions: petitions, files: files, objects: objects}
}

// ActionPetitionPhotosViewed is the verb of the trail a staff read of the photos leaves.
const ActionPetitionPhotosViewed = "xem_anh_hien_truong"

// ListPhotos answers the photos of petition `ma` in this commune with signed links.
//
// `mayReadRestricted` is whether the reader holds `feedback.restricted`, decided by the handler's
// checker. A `can-bo` petition without it is ErrPhieuKhongTonTai — the SAME answer GET
// /api/v1/citizen-reports/{code} gives, so this route cannot confirm such a report exists.
//
// AUDITED (rule 6, invariant 7) — changed 2026-10-02 from "not audited". A scene photo is personal
// data (rule 3) that CANNOT BE MASKED, so every link handed to staff is a read of full personal data,
// the same footing as `feedback.unmask` on the detail (XemNguoiGui, ADR 0030). The precedent is
// followed exactly: the entry is COMMITTED BEFORE the links are returned, and an error writing it means
// no links at all — a disclosure with no trail is the one state rule 6 does not permit. It is written
// after the links are signed, so a signing failure does not leave an entry for a disclosure that never
// happened. A petition with no stored photo discloses nothing and writes nothing.
//
// `reader` is the STAFF member, its ID their business code (`principal.Ma`, rule 6 invariant 8) —
// built by the handler; an empty code refuses the read, never falls back to the internal id.
func (uc *StaffPetitionPhotos) ListPhotos(ctx context.Context, ma string, mayReadRestricted bool,
	reader audit.Actor) ([]PhotoLink, error) {

	if reader.ID == "" || reader.Kind != "staff" {
		// Fail closed BEFORE any read: with no business code there is nobody to attribute the read to.
		return nil, fmt.Errorf("anh_hien_truong: người đọc không phải cán bộ có mã (kind=%q) — "+
			"không ghi được vết thì không mở ảnh", reader.Kind)
	}
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
	links, err := signPhotos(ctx, uc.files, uc.objects, p.ID, now)
	if err != nil {
		return nil, err
	}
	return auditStaffPhotoRead(ctx, uc.db, reader, p.MaTraCuu, ActionPetitionPhotosViewed, links, now)
}

// auditStaffPhotoRead commits the trail of one staff read of a petition's photos, THEN returns the
// links — the scene photos' rule (ListPhotos above), shared with the verification photos so the two
// lists cannot drift apart on it. No photo, no disclosure, no entry. An error writing the entry means no
// links at all: a disclosure with no trail is the one state rule 6 does not permit.
func auditStaffPhotoRead(ctx context.Context, db *store.DB, reader audit.Actor, code, action string,
	links []PhotoLink, now time.Time) ([]PhotoLink, error) {
	if len(links) == 0 {
		return links, nil
	}
	// The delta names WHICH files were opened — ids and a count. Never a URL (a bearer credential until
	// it expires), never a file name, never a byte of the image.
	ids := make([]string, 0, len(links))
	for _, l := range links {
		ids = append(ids, l.File.ID)
	}
	delta, err := json.Marshal(map[string]any{"so_anh": len(ids), "tep_id": ids, "quyen": "feedback.read"})
	if err != nil {
		return nil, fmt.Errorf("ảnh phiếu: mã hoá delta: %w", err)
	}
	err = db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// TenantID unset: audit.Write takes it from the transaction (rule 1, invariant 4).
		return audit.Write(ctx, tx, audit.Entry{Actor: reader, Action: action, Subject: code, At: now,
			Delta: delta})
	})
	if err != nil {
		// Neither the code nor the reader in the message (rule 3) — the commune is what an operator needs.
		return nil, fmt.Errorf("ảnh phiếu: ghi vết cán bộ xem ảnh cho xã %s: %w", tenant.MustFrom(ctx), err)
	}
	return links, nil
}

// Compile-time proof the real dependencies satisfy the interfaces.
var (
	_ PhotoObjectStore      = (*storage.Client)(nil)
	_ PetitionPhotoFiles    = (*petstore.StoredFileStore)(nil)
	_ CitizenPhotoPetitions = (*petstore.PhieuPhanAnhStore)(nil)
	_ OwnedPhotoPetitions   = (*petstore.PhieuPhanAnhStore)(nil)
	_ StaffPhotoPetitions   = (*petstore.PhieuPhanAnhStore)(nil)
)
