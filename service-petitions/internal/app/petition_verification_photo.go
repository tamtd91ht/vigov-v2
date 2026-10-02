package app

// STAFF VERIFICATION PHOTOS — "SAU KHI XỬ LÝ" on the petition detail (docs/ui-ux/09 §8.4), owner
// decisions of 02/10/2026 (ADR 0047 row "Ảnh 'sau xử lý' của cán bộ — THAY G8"; ADR 0008 decision 3),
// storage of migration 0027. The scene photo's three steps (petition_photo.go), for a member of staff:
//
//	a. RequestUpload  pending row + presigned POST              the petition locked, ≤ 5, not ended
//	b. the browser    POST straight to OBJECT_STORAGE_PUBLIC_ENDPOINT
//	c. Complete       stat · sniff · scan · DECODE · ORIENT · RE-ENCODE JPEG WITHOUT EXIF ·
//	                  PutServerProduced into private (class records) · purge temp · `stored` + trail
//
// # WHY RE-ENCODED ALTHOUGH IT IS A RECORD (owner's choice, 02/10/2026)
//
// The citizen SEES this photo on their own petition. A staff phone's EXIF carries the GPS of where the
// officer stood, device identifiers and the capture time — personal data of the officer and, near a
// home, of the citizen (rule 3). So the raw upload never reaches the private bucket, exactly as for the
// citizen's photo; core/storage.PutServerProduced admits this one records purpose and no other.
//
// ⚠ THE COST OF `records`: a records object can never be purged outside temp. A clean copy written in
// step (c) whose transaction then refuses it (the petition was closed, or the 5th slot was taken, while
// the image was processed) STAYS in the private bucket with a `rejected` row — the trail says so
// (`ban_sach_con_trong_kho`). It is never silently deleted, and it is never shown: every read binds
// status stored/ready.
//
// # WHO
//
// Route gate `feedback.resolve` for both writes — the key that guards closing, because this photo IS
// the closing's evidence (ADR 0008 decision 3), and open question #7 settled that `feedback.resolve`
// decides who closes. `feedback.restricted` narrows as on every petition act: a `can-bo` petition
// without it is the 404 of an unknown code, decided on the locked row. Completing is the UPLOADER's
// alone — another officer's file id answers 404 like an unknown one.
//
// # WHEN — every status except the three endings (domain.VerificationPhotoUploadOpen says why)
//
// # SHARED WITH THE CITIZEN FLOW, ON PURPOSE
//
// The decode and read slots, the in-flight de-duplication, the inspector and the re-encoder are the
// citizen photo's (photoSlots, photoInspector). One process, one decode budget: a second set of slots
// would let two decodes run at once against a pod sized for one.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/storage"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// The verbs in the trail — Vietnamese snake_case values (ADR 0011), the scene photo's set for staff.
const (
	ActionVerificationPhotoRequested = "xin_tai_anh_sau_xu_ly"
	ActionVerificationPhotoStored    = "luu_anh_sau_xu_ly"
	ActionVerificationPhotoRejected  = "tu_choi_anh_sau_xu_ly"
	ActionVerificationPhotoExpired   = "anh_sau_xu_ly_het_han_tai"
	ActionVerificationPhotosViewed   = "xem_anh_sau_xu_ly"
)

var (
	// ErrVerificationPhotoNotFound: no such verification photo ON THIS PETITION FOR THIS CALLER — unknown,
	// another petition's, another commune's, another officer's unfinished upload, a citizen photo. 404.
	ErrVerificationPhotoNotFound = errors.New("ảnh sau xử lý: không tìm thấy")
	// ErrVerificationPhotoWindowClosed: the petition has ended (closed, refused, referred). 409.
	ErrVerificationPhotoWindowClosed = errors.New("ảnh sau xử lý: phiếu đã kết thúc, không thêm ảnh được")
	// ErrVerificationPhotoCountReached: the petition already holds platform's maximum, live slots
	// included. 409.
	ErrVerificationPhotoCountReached = errors.New("ảnh sau xử lý: phiếu đã đủ số ảnh tối đa")
)

// VerificationPhotoPetitions is the STAFF register's two reads — never the citizen's identity-filtered
// one (rule 4, invariant 5). *petstore.PhieuPhanAnhStore satisfies it.
type VerificationPhotoPetitions interface {
	// vi-name-ok: mirrors the existing PhieuPhanAnhStore method; rule 12 invariant 3 keeps existing names
	TheoMaTraCuu(ctx context.Context, ma string) (domain.PhieuPhanAnh, error)
	// vi-name-ok: mirrors the existing PhieuPhanAnhStore method; rule 12 invariant 3 keeps existing names
	TheoMaTraCuuDeSua(ctx context.Context, tx *store.ScopedTx, ma string) (domain.PhieuPhanAnh, error)
}

// verificationPhotoLister is the one read the link signing needs.
type verificationPhotoLister interface {
	VerificationPhotos(ctx context.Context, petitionID string) ([]domain.StoredFile, error)
}

// VerificationPhotoFiles is the part of *petstore.StoredFileStore this flow calls.
type VerificationPhotoFiles interface {
	verificationPhotoLister
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

// StaffVerificationPhotos owns staff's three acts on verification photos.
type StaffVerificationPhotos struct {
	db        *store.DB
	petitions VerificationPhotoPetitions
	files     VerificationPhotoFiles

	// ANY OF THE THREE nil means "not configured" (503). The list needs only `objects`.
	objects  PhotoObjectStore
	scanner  MalwareScanner
	policies UploadPolicies

	*photoSlots

	newID func() (string, error)
	now   func() time.Time
}

// NewStaffVerificationPhotos builds the use case ON THE CITIZEN PHOTO FLOW'S DEPENDENCIES AND SLOTS —
// the same object store, scanner and policy reader, and the same process-wide photoSlots (see the file
// header for why sharing the slots is the point). A nil `citizen` builds an unconfigured use case: every
// route answers 503.
func NewStaffVerificationPhotos(db *store.DB, petitions VerificationPhotoPetitions,
	files VerificationPhotoFiles, citizen *CitizenPetitionPhotos) *StaffVerificationPhotos {
	uc := &StaffVerificationPhotos{db: db, petitions: petitions, files: files, newID: storage.NewObjectID}
	if citizen == nil {
		uc.photoSlots = newPhotoSlots()
		return uc
	}
	uc.objects, uc.scanner, uc.policies, uc.photoSlots = citizen.objects, citizen.scanner, citizen.policies,
		citizen.photoSlots
	return uc
}

func (uc *StaffVerificationPhotos) clock() time.Time {
	if uc.now == nil {
		return time.Now().UTC()
	}
	return uc.now().UTC()
}

func (uc *StaffVerificationPhotos) uploadsConfigured() bool {
	return uc.objects != nil && uc.scanner != nil && uc.policies != nil
}

func (uc *StaffVerificationPhotos) inspector() photoInspector {
	return photoInspector{objects: uc.objects, scanner: uc.scanner, files: uc.files, slots: uc.photoSlots,
		clock: uc.clock}
}

// verificationPhotoKey is the destination key: class records, purpose petition-verification-photo,
// variant original, ALWAYS `.jpg` (the stored bytes are the re-encode) — the one records key
// core/storage.PutServerProduced admits.
func verificationPhotoKey(ctx context.Context, id string, at time.Time) storage.Key {
	return storage.Key{
		Class: storage.ClassRecords, TenantID: string(tenant.MustFrom(ctx)), CreatedAt: at,
		Service: storage.ServicePetitions, Purpose: storage.PurposePetitionVerificationPhoto,
		ObjectID: id, Variant: storage.VariantOriginal, Ext: "jpg",
	}
}

// ownVerificationPhoto: the row is a verification photo of THIS petition, issued to THIS officer.
func ownVerificationPhoto(f *domain.StoredFile, petitionID, officer string) bool {
	return f != nil && officer != "" && domain.IsVerificationPhotoOf(*f, petitionID) && f.UploadedBy == officer
}

// --- a. request a slot ------------------------------------------------------------------------------

// RequestUpload issues one upload slot for one verification photo on petition `ma`.
//
// ONE TRANSACTION: the petition read FOR UPDATE (the restricted field and the status are decided on the
// locked row, and two requests on one petition serialise so both cannot pass at 4), the count with live
// pending slots, the pending row, the form (signed offline) and the audit entry.
func (uc *StaffVerificationPhotos) RequestUpload(ctx context.Context, ma string, req PhotoUploadRequest,
	actor audit.Actor, restricted QuyenXemHanChe) (PhotoUpload, error) {

	if err := coCanBoThucHien(actor); err != nil {
		return PhotoUpload{}, err
	}
	if req.Size <= 0 {
		return PhotoUpload{}, ErrPhotoSizeInvalid
	}
	if !uc.uploadsConfigured() {
		return PhotoUpload{}, ErrUploadNotConfigured
	}
	pol, err := photoPolicyFor(ctx, uc.policies, storage.PurposePetitionVerificationPhoto)
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
		return PhotoUpload{}, fmt.Errorf("ảnh sau xử lý: sinh mã tệp: %w", err)
	}
	now := uc.clock()
	dst := verificationPhotoKey(ctx, id, now)
	objectKey, err := dst.Path()
	if err != nil {
		return PhotoUpload{}, fmt.Errorf("ảnh sau xử lý: dựng khoá đối tượng: %w", err)
	}
	uploadKey, err := uploadKeyFor(dst, upExt)
	if err != nil {
		return PhotoUpload{}, fmt.Errorf("ảnh sau xử lý: dựng khoá tải lên: %w", err)
	}

	var out PhotoUpload
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		p, err := uc.petitions.TheoMaTraCuuDeSua(ctx, tx, ma)
		if err != nil {
			return err
		}
		// BEFORE THE STATUS CHECK: a 409 about the state would confirm a `can-bo` report exists.
		if err := duocChamPhieuHanChe(p, restricted); err != nil {
			return err
		}
		if !domain.VerificationPhotoUploadOpen(p.TrangThai) {
			return ErrVerificationPhotoWindowClosed
		}
		live, err := uc.files.CountForSubjectTx(ctx, tx, domain.StoredFileSubjectPetition, p.ID,
			domain.PurposePetitionVerificationPhoto, now.Add(-storage.UploadTTL))
		if err != nil {
			return err
		}
		if live >= pol.MaxFilesPerSubject {
			return ErrVerificationPhotoCountReached
		}
		f := domain.StoredFile{
			ID: id, Bucket: domain.StoredFileBucketPrivate, ObjectKey: objectKey,
			RetentionClass: string(storage.ClassRecords), Purpose: domain.PurposePetitionVerificationPhoto,
			SubjectType: domain.StoredFileSubjectPetition, SubjectID: p.ID,
			OriginalName: domain.VerificationPhotoName, Status: domain.StoredFilePending,
			// The STAFF BUSINESS CODE (rule 6, invariant 8) — audit.Actor.ID is Principal.Ma.
			UploadedBy: actor.ID, CreatedAt: now, UpdatedAt: now,
		}
		if err := uc.files.InsertPending(ctx, tx, f); err != nil {
			return err
		}
		post, err := uc.objects.PresignUpload(ctx, uploadKey, pol.MaxBytes, req.ContentType, storage.UploadTTL)
		if err != nil {
			return fmt.Errorf("ảnh sau xử lý: ký lượt tải lên: %w", err)
		}
		if err := writePhotoAudit(ctx, tx, actor, p.MaTraCuu, ActionVerificationPhotoRequested, now, map[string]any{
			"tep_id": id, "muc_dich": domain.PurposePetitionVerificationPhoto,
			"loai_khai_bao": req.ContentType, "kich_thuoc_khai": req.Size,
		}); err != nil {
			return err
		}
		out = PhotoUpload{File: f, Post: post}
		return nil
	})
	if err != nil {
		return PhotoUpload{}, bocPhieu(ctx, "xin tải ảnh sau xử lý", err)
	}
	return out, nil
}

// --- c. complete ------------------------------------------------------------------------------------

// Complete runs step (c) for one upload on petition `ma`. Lock-free inspection first (shared with the
// citizen flow), then ONE short transaction that re-reads the petition and the row FOR UPDATE, re-checks
// the restricted field, the window and the count under the lock, and records the outcome with its trail.
//
// IDEMPOTENT: a photo already stored answers as stored (the route declares idem.KhongCan).
func (uc *StaffVerificationPhotos) Complete(ctx context.Context, ma, id string, actor audit.Actor,
	restricted QuyenXemHanChe) (domain.StoredFile, error) {

	if err := coCanBoThucHien(actor); err != nil {
		return domain.StoredFile{}, err
	}
	if !uc.uploadsConfigured() {
		return domain.StoredFile{}, ErrUploadNotConfigured
	}

	// 1. Lock-free pre-read: a petition this officer may touch, their own upload on it, work left.
	p, err := uc.petitions.TheoMaTraCuu(ctx, ma)
	if err != nil {
		return domain.StoredFile{}, bocPhieu(ctx, "hoàn tất ảnh sau xử lý", err)
	}
	if err := duocChamPhieuHanChe(p, restricted); err != nil {
		return domain.StoredFile{}, bocPhieu(ctx, "hoàn tất ảnh sau xử lý", err)
	}
	f, err := uc.files.ByID(ctx, id)
	if err != nil {
		return domain.StoredFile{}, bocPhieu(ctx, "hoàn tất ảnh sau xử lý", err)
	}
	if !ownVerificationPhoto(f, p.ID, actor.ID) {
		return domain.StoredFile{}, ErrVerificationPhotoNotFound
	}
	switch {
	case f.Status.Attachable() || f.Status == domain.StoredFileProcessing:
		return *f, nil
	case f.Status != domain.StoredFilePending:
		return domain.StoredFile{}, ErrAttachmentNotPending
	}
	if !domain.VerificationPhotoUploadOpen(p.TrangThai) {
		return domain.StoredFile{}, ErrVerificationPhotoWindowClosed
	}
	return uc.completeOnce(ctx, id, func(ctx context.Context) (domain.StoredFile, error) {
		return uc.completePending(ctx, ma, id, actor, restricted, *f)
	})
}

// completePending is steps 2 and 3 of Complete for a file the caller owns and that was `pending`.
func (uc *StaffVerificationPhotos) completePending(ctx context.Context, ma, id string, actor audit.Actor,
	restricted QuyenXemHanChe, f domain.StoredFile) (domain.StoredFile, error) {

	pol, err := photoPolicyFor(ctx, uc.policies, storage.PurposePetitionVerificationPhoto)
	if err != nil {
		return domain.StoredFile{}, err
	}
	dst, err := storage.ParseKey(f.ObjectKey)
	if err != nil {
		return domain.StoredFile{}, fmt.Errorf("ảnh sau xử lý: khoá đối tượng đã ghi không hợp lệ: %w", err)
	}

	// 2. Object-store and pixel work, no lock held.
	insp, err := uc.inspector().inspect(ctx, f, dst, pol)
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
		lp, err := uc.petitions.TheoMaTraCuuDeSua(ctx, tx, ma)
		if err != nil {
			return err
		}
		if err := duocChamPhieuHanChe(lp, restricted); err != nil {
			return err
		}
		cur, err := uc.files.ForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if !ownVerificationPhoto(cur, lp.ID, actor.ID) {
			return ErrVerificationPhotoNotFound
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
			if !domain.VerificationPhotoUploadOpen(lp.TrangThai) {
				lateReason = RejectPetitionMoved
			} else {
				have, err := uc.files.CountForSubjectTx(ctx, tx, domain.StoredFileSubjectPetition, lp.ID,
					domain.PurposePetitionVerificationPhoto, time.Time{})
				if err != nil {
					return err
				}
				if have >= pol.MaxFilesPerSubject {
					lateReason = RejectCountReached
				}
			}
			if lateReason != "" {
				// NOT PURGED: a records object cannot be (core/storage PurgeAllVersions). It stays, never
				// listed, and the trail says so — the cost the file header states.
				if err := uc.files.Transition(ctx, tx, id, domain.StoredFilePending, domain.StoredFileRejected, now); err != nil {
					return err
				}
				return writePhotoAudit(ctx, tx, actor, lp.MaTraCuu, ActionVerificationPhotoRejected, now, map[string]any{
					"tep_id": id, "ly_do": lateReason, "da_xoa_tep_tam": insp.tempRemoved,
					// Always true on this branch: outcomeStored means the clean copy is in the private bucket
					// (written now, or found there by fromDestination), and records cannot be purged.
					"ban_sach_con_trong_kho": true,
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
			return writePhotoAudit(ctx, tx, actor, lp.MaTraCuu, ActionVerificationPhotoStored, now, map[string]any{
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
			return writePhotoAudit(ctx, tx, actor, lp.MaTraCuu, ActionVerificationPhotoRejected, now, d)
		case outcomeExpired:
			if err := uc.files.Transition(ctx, tx, id, domain.StoredFilePending, domain.StoredFileFailed, now); err != nil {
				return err
			}
			return writePhotoAudit(ctx, tx, actor, lp.MaTraCuu, ActionVerificationPhotoExpired, now,
				map[string]any{"tep_id": id})
		}
		return fmt.Errorf("ảnh sau xử lý: kết quả kiểm tra không rõ (%d)", insp.kind)
	})
	if err != nil {
		return domain.StoredFile{}, bocPhieu(ctx, "hoàn tất ảnh sau xử lý", err)
	}
	switch {
	case stored.ID != "":
		return stored, nil
	case lateReason == RejectPetitionMoved:
		return domain.StoredFile{}, ErrVerificationPhotoWindowClosed
	case lateReason != "":
		return domain.StoredFile{}, &AttachmentRejection{Reason: lateReason}
	case insp.kind == outcomeRejected:
		return domain.StoredFile{}, &AttachmentRejection{Reason: insp.reason}
	default: // outcomeExpired
		return domain.StoredFile{}, ErrUploadExpired
	}
}

// --- the staff list -----------------------------------------------------------------------------------

// ListPhotos answers the stored verification photos of petition `ma` with signed links (15 minutes).
//
// AUDITED, exactly like the scene photos' staff list (auditStaffPhotoRead, rule 6 invariant 7): a photo
// of a place a citizen reported cannot be masked. `mayReadRestricted` is `feedback.restricted`; a
// `can-bo` petition without it is ErrPhieuKhongTonTai, the detail's own 404.
func (uc *StaffVerificationPhotos) ListPhotos(ctx context.Context, ma string, mayReadRestricted bool,
	reader audit.Actor) ([]PhotoLink, error) {

	if reader.ID == "" || reader.Kind != "staff" {
		return nil, fmt.Errorf("ảnh sau xử lý: người đọc không phải cán bộ có mã (kind=%q) — "+
			"không ghi được vết thì không mở ảnh", reader.Kind)
	}
	if uc.objects == nil {
		return nil, ErrUploadNotConfigured
	}
	p, err := uc.petitions.TheoMaTraCuu(ctx, ma)
	if err != nil {
		return nil, bocPhieu(ctx, "đọc ảnh sau xử lý", err)
	}
	if p.LinhVuc == domain.LinhVucHanChe && !mayReadRestricted {
		return nil, bocPhieu(ctx, "đọc ảnh sau xử lý", petstore.ErrPhieuKhongTonTai)
	}
	now := uc.clock()
	links, err := signVerificationPhotos(ctx, uc.files, uc.objects, p.ID, now)
	if err != nil {
		return nil, err
	}
	return auditStaffPhotoRead(ctx, uc.db, reader, p.MaTraCuu, ActionVerificationPhotosViewed, links, now)
}

// signVerificationPhotos reads a petition's verification photos and signs one GET each. The shape test
// binds the PURPOSE (migration 0027 question 4): a log attachment is never signed here, whatever a
// future query returns.
func signVerificationPhotos(ctx context.Context, files verificationPhotoLister, objects PhotoObjectStore,
	petitionID string, now time.Time) ([]PhotoLink, error) {
	photos, err := files.VerificationPhotos(ctx, petitionID)
	if err != nil {
		return nil, bocPhieu(ctx, "đọc ảnh sau xử lý", err)
	}
	return signPhotoList(ctx, objects, photos, func(f domain.StoredFile) bool {
		return domain.IsVerificationPhotoOf(f, petitionID)
	}, now)
}

// --- the citizen's read ---------------------------------------------------------------------------------

// CitizenVerificationPhotos lists the verification photos of the citizen's OWN petition — the "after"
// half of "Ảnh trước và sau khi xử lý" in the commune app (owner decision C, 02/10/2026). A SEPARATE type
// from the staff one, built on the identity-filtered petition read, so the two surfaces never share a
// reader (rule 4, invariant 5).
type CitizenVerificationPhotos struct {
	petitions CitizenPhotoPetitions
	files     verificationPhotoLister
	objects   PhotoObjectStore
	now       func() time.Time
}

// NewCitizenVerificationPhotos builds the citizen read. objects may be UNTYPED nil (→ 503).
func NewCitizenVerificationPhotos(petitions CitizenPhotoPetitions, files VerificationPhotoFiles,
	objects PhotoObjectStore) *CitizenVerificationPhotos {
	return &CitizenVerificationPhotos{petitions: petitions, files: files, objects: objects}
}

// ListPhotos returns the stored verification photos of the citizen's own petition `ma`, signed links
// issued only AFTER the session's citizen and commune matched it (rule 4, invariants 2, 3 and 7).
// Another citizen's code, another commune's and an unknown one are ONE ErrPhieuKhongTonTai.
//
// ONLY AT `cho-dan-xac-nhan` AND `da-dong` (owner decision (b), 02/10/2026 —
// domain.VerificationPhotosVisibleToCitizen states the rule and the reopened case). At any other status
// the answer is an EMPTY list, not a 404: the petition exists and is this citizen's (the identity read
// above already passed), so a 404 would be false, and "no photo to show yet" leaks nothing — the same
// shape as the citizen view, which gates the refusal reason on the status and otherwise leaves it
// absent in a 200 (http/phieu_cua_toi.go, `ra.Reason`).
// The file register is not read at all on that branch. NEVER A LOG ATTACHMENT: the read binds purpose
// petition-verification-photo.
//
// NOT AUDITED: the citizen reading their own petition, as the scene-photo list is not.
func (uc *CitizenVerificationPhotos) ListPhotos(ctx context.Context, ma string, citizen audit.Actor) (
	[]PhotoLink, error) {
	if err := citizenOnly(citizen); err != nil {
		return nil, err
	}
	if uc.objects == nil {
		return nil, ErrUploadNotConfigured
	}
	p, err := uc.petitions.CuaCongDanTheoMaTraCuu(ctx, citizen.ID, ma)
	if err != nil {
		return nil, bocPhieu(ctx, "đọc ảnh sau xử lý", err)
	}
	if !domain.VerificationPhotosVisibleToCitizen(p.TrangThai) {
		return []PhotoLink{}, nil
	}
	now := time.Now().UTC()
	if uc.now != nil {
		now = uc.now().UTC()
	}
	return signVerificationPhotos(ctx, uc.files, uc.objects, p.ID, now)
}

// Compile-time proof the real dependencies satisfy the interfaces.
var (
	_ VerificationPhotoPetitions = (*petstore.PhieuPhanAnhStore)(nil)
	_ VerificationPhotoFiles     = (*petstore.StoredFileStore)(nil)
)
