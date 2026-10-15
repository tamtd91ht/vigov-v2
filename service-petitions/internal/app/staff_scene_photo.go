package app

// SCENE PHOTOS ATTACHED BY THE OFFICER WHO BOOKED THE PETITION — owner decision 15/10/2026 (tester bug
// sheet web-admin-141026, "Nhập hộ phản ánh": §11's "Đính ảnh hiện trường"). The citizen who rang or
// came in hands the officer a photo (Zalo, a USB stick, a phone shown at the desk); the officer attaches
// it to the petition they have just booked, right after POST /api/v1/citizen-reports returned its code.
//
// THE CITIZEN FLOW'S PIPELINE, UNCHANGED (petition_photo.go): ONE multipart request per photo, reserve ·
// stream into temp · stat · sniff · clamd scan · decode · orient · RE-ENCODE JPEG WITHOUT EXIF ·
// PutServerProduced into the PRIVATE bucket under `citizen-media/t_<tenant>/…/petitions/petition-photo/…`
// · purge temp · `stored` + trail in ONE transaction. Same platform policy (`petition-photo`: types, size,
// at most 5 per petition), same object store, scanner and process-wide decode/read slots — a second set
// of slots would let two decodes run against a pod sized for one.
//
// # WHAT DIFFERS FROM THE CITIZEN'S, AND WHY
//
//	who        the OFFICER: `uploaded_by` = their business code (rule 6, invariant 8), actor kind staff.
//	           Migration 0026's citizen-shape CHECK and its "a citizen upload needs a citizen petition"
//	           trigger bind only the 'cong-dan' marker, so this row passes them as written
//	which      ONLY a petition the staff intake booked (domain.PhieuPhanAnh.BookedByStaff — no owner, a
//	           channel only staff record). A citizen's petition gets its photos from the citizen; an
//	           officer adding to it would change, behind the citizen's back, what the citizen sent
//	           (409 `petition_not_staff_booked`)
//	read       the staff register's read (TheoMaTraCuu…), NEVER an owner-filtered citizen read (rule 4,
//	           invariant 5); `feedback.restricted` narrows as on every petition act: a `can-bo` petition
//	           without it is the detail's 404, decided on the locked row
//	when       while the petition is still `da-tiep-nhan` (domain.PhotoUploadOpen) — "at intake"
//	count      platform's `petition-photo` count over EVERY uploader of the petition, under its row lock
//	           (CountForSubjectTx binds subject + purpose, not the uploader). ⚠ 0026's trigger, the DB
//	           floor, counts only citizen rows; on a staff-booked petition there are none, so the floor
//	           under THIS path is the use case's count alone
//
// The completion is the UPLOADER's alone — another officer's pending file answers ErrPhotoNotFound.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/storage"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// The verbs this flow adds to the trail — Vietnamese snake_case values (ADR 0011), the citizen's verbs
// with the officer named, so an inspection reading the trail tells who attached the scene photo. The
// rejected / expired outcomes reuse the citizen flow's verbs, which name no actor.
const (
	ActionStaffScenePhotoRequested = "can_bo_xin_tai_anh_hien_truong"
	ActionStaffScenePhotoStored    = "can_bo_luu_anh_hien_truong"
)

// ErrScenePhotoNotStaffBooked: the petition was filed by a citizen (or through the accountless Mini App
// path), not booked by staff — its scene photos are the sender's to attach. 409.
var ErrScenePhotoNotStaffBooked = errors.New("ảnh hiện trường: phiếu không do cán bộ nhập hộ — " +
	"cán bộ chỉ đính ảnh cho phiếu mình nhập hộ")

// StaffScenePhotoPetitions is the STAFF register's two reads — never the citizen's identity-filtered one
// (rule 4, invariant 5). *petstore.PhieuPhanAnhStore satisfies it.
type StaffScenePhotoPetitions interface {
	// vi-name-ok: mirrors the existing PhieuPhanAnhStore method; rule 12 invariant 3 keeps existing names
	TheoMaTraCuu(ctx context.Context, ma string) (domain.PhieuPhanAnh, error)
	// vi-name-ok: mirrors the existing PhieuPhanAnhStore method; rule 12 invariant 3 keeps existing names
	TheoMaTraCuuDeSua(ctx context.Context, tx *store.ScopedTx, ma string) (domain.PhieuPhanAnh, error)
}

// StaffScenePhotos owns the officer's scene-photo upload on a staff-booked petition.
type StaffScenePhotos struct {
	db        *store.DB
	petitions StaffScenePhotoPetitions
	files     PetitionPhotoFiles

	// ANY OF THE THREE nil means "not configured": every upload is refused with ErrUploadNotConfigured
	// (503) while the intake itself keeps serving.
	objects  PhotoObjectStore
	scanner  MalwareScanner
	policies UploadPolicies

	*photoSlots

	newID func() (string, error)
	now   func() time.Time
}

// NewStaffScenePhotos builds the use case ON THE CITIZEN PHOTO FLOW'S DEPENDENCIES AND SLOTS (the file
// header says why sharing them is the point). A nil `citizen` builds an unconfigured use case: the route
// answers 503.
func NewStaffScenePhotos(db *store.DB, petitions StaffScenePhotoPetitions, files PetitionPhotoFiles,
	citizen *CitizenPetitionPhotos) *StaffScenePhotos {
	uc := &StaffScenePhotos{db: db, petitions: petitions, files: files, newID: storage.NewObjectID}
	if citizen == nil {
		uc.photoSlots = newPhotoSlots()
		return uc
	}
	uc.objects, uc.scanner, uc.policies, uc.photoSlots = citizen.objects, citizen.scanner, citizen.policies,
		citizen.photoSlots
	return uc
}

func (uc *StaffScenePhotos) clock() time.Time {
	if uc.now == nil {
		return time.Now().UTC()
	}
	return uc.now().UTC()
}

func (uc *StaffScenePhotos) uploadsConfigured() bool {
	return uc.objects != nil && uc.scanner != nil && uc.policies != nil && uc.files != nil
}

func (uc *StaffScenePhotos) inspector() photoInspector {
	return photoInspector{objects: uc.objects, scanner: uc.scanner, files: uc.files, slots: uc.photoSlots,
		clock: uc.clock}
}

// staffScenePhotoOpen is the ONE gate of this act on a petition already read in this commune, in the
// order that leaks nothing: the restricted field first (404 — a 409 would confirm a `can-bo` report
// exists), then "booked by staff", then the status window.
func staffScenePhotoOpen(p domain.PhieuPhanAnh, restricted QuyenXemHanChe) error {
	if err := duocChamPhieuHanChe(p, restricted); err != nil {
		return err
	}
	if !p.BookedByStaff() {
		return ErrScenePhotoNotStaffBooked
	}
	if !domain.PhotoUploadOpen(p.TrangThai) {
		return ErrPhotoWindowClosed
	}
	return nil
}

// ownStaffScenePhoto: the row is a scene photo of THIS petition, issued to THIS officer.
func ownStaffScenePhoto(f *domain.StoredFile, petitionID, officer string) bool {
	return f != nil && officer != "" && officer != domain.CitizenLogActor &&
		domain.IsScenePhotoOf(*f, petitionID) && f.UploadedBy == officer
}

// MaxUploadBytes is the policy's cap on one photo, for the handler to bound the request body BEFORE it
// reads it. Not configured / unreachable / no count are refusals, never a default (ADR 0052 stop #4).
func (uc *StaffScenePhotos) MaxUploadBytes(ctx context.Context) (int64, error) {
	if !uc.uploadsConfigured() {
		return 0, ErrUploadNotConfigured
	}
	pol, err := photoPolicyFor(ctx, uc.policies, storage.PurposePetitionPhoto)
	if err != nil {
		return 0, err
	}
	return pol.MaxBytes, nil
}

// Upload stores one scene photo on the staff-booked petition `ma`: reserve (a), stream (b), complete (c)
// — one request, answered with the STORED photo or a refusal. A failure after (a) closes the reserved row.
func (uc *StaffScenePhotos) Upload(ctx context.Context, ma string, req PhotoUploadRequest, body UploadBody,
	actor audit.Actor, restricted QuyenXemHanChe) (domain.StoredFile, error) {

	res, err := uc.reserve(ctx, ma, req, actor, restricted)
	if err != nil {
		return domain.StoredFile{}, err
	}
	closer := uploadCloser{db: uc.db, objects: uc.objects, files: uc.files, actor: actor,
		failAction: ActionPetitionPhotoExpired, clock: uc.clock}
	if err := closer.receive(ctx, res, body); err != nil {
		return domain.StoredFile{}, err
	}
	f, err := uc.complete(ctx, ma, res.file.ID, actor, restricted)
	if err != nil {
		return domain.StoredFile{}, closer.afterComplete(ctx, res, err)
	}
	return f, nil
}

// reserve is step (a). ONE TRANSACTION: the petition read FOR UPDATE (the gate is decided on the locked
// row, and two uploads on one petition serialise so both cannot pass at 4), the count with live pending
// rows, the pending row and the audit entry.
func (uc *StaffScenePhotos) reserve(ctx context.Context, ma string, req PhotoUploadRequest,
	actor audit.Actor, restricted QuyenXemHanChe) (uploadReservation, error) {

	if err := coCanBoThucHien(actor); err != nil {
		return uploadReservation{}, err
	}
	if req.Size <= 0 {
		return uploadReservation{}, ErrPhotoSizeInvalid
	}
	if !uc.uploadsConfigured() {
		return uploadReservation{}, ErrUploadNotConfigured
	}
	pol, err := photoPolicyFor(ctx, uc.policies, storage.PurposePetitionPhoto)
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
		return uploadReservation{}, fmt.Errorf("ảnh hiện trường (cán bộ): sinh mã tệp: %w", err)
	}
	now := uc.clock()
	// THE CITIZEN PHOTO'S KEY, class citizen-media: a scene photo is citizen personal data whoever
	// uploaded it (rule 3), and migration 0026's shape CHECK forces exactly this class and bucket.
	dst := photoKey(ctx, id, now)
	objectKey, err := dst.Path()
	if err != nil {
		return uploadReservation{}, fmt.Errorf("ảnh hiện trường (cán bộ): dựng khoá đối tượng: %w", err)
	}
	uploadKey, err := uploadKeyFor(dst, upExt)
	if err != nil {
		return uploadReservation{}, fmt.Errorf("ảnh hiện trường (cán bộ): dựng khoá tải lên: %w", err)
	}

	res := uploadReservation{uploadKey: uploadKey, contentType: req.ContentType, size: req.Size,
		maxBytes: pol.MaxBytes}
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		p, err := uc.petitions.TheoMaTraCuuDeSua(ctx, tx, ma)
		if err != nil {
			return err
		}
		if err := staffScenePhotoOpen(p, restricted); err != nil {
			return err
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
			Status: domain.StoredFilePending,
			// The STAFF BUSINESS CODE (rule 6, invariant 8) — audit.Actor.ID is Principal.Ma.
			UploadedBy: actor.ID, CreatedAt: now, UpdatedAt: now,
		}
		if err := uc.files.InsertPending(ctx, tx, f); err != nil {
			return err
		}
		if err := writePhotoAudit(ctx, tx, actor, p.MaTraCuu, ActionStaffScenePhotoRequested, now, map[string]any{
			"tep_id": id, "muc_dich": domain.PurposePetitionPhoto,
			"loai_khai_bao": req.ContentType, "kich_thuoc_khai": req.Size,
		}); err != nil {
			return err
		}
		res.file, res.subject = f, p.MaTraCuu
		return nil
	})
	if err != nil {
		return uploadReservation{}, bocPhieu(ctx, "cán bộ xin tải ảnh hiện trường", err)
	}
	return res, nil
}

// complete runs step (c) on the bytes Upload has just streamed. IDEMPOTENT: a stored photo answers as
// stored.
func (uc *StaffScenePhotos) complete(ctx context.Context, ma, id string, actor audit.Actor,
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
		return domain.StoredFile{}, bocPhieu(ctx, "cán bộ hoàn tất ảnh hiện trường", err)
	}
	if err := duocChamPhieuHanChe(p, restricted); err != nil {
		return domain.StoredFile{}, bocPhieu(ctx, "cán bộ hoàn tất ảnh hiện trường", err)
	}
	f, err := uc.files.ByID(ctx, id)
	if err != nil {
		return domain.StoredFile{}, bocPhieu(ctx, "cán bộ hoàn tất ảnh hiện trường", err)
	}
	if !ownStaffScenePhoto(f, p.ID, actor.ID) {
		return domain.StoredFile{}, ErrPhotoNotFound
	}
	switch {
	case f.Status.Attachable() || f.Status == domain.StoredFileProcessing:
		return *f, nil
	case f.Status != domain.StoredFilePending:
		return domain.StoredFile{}, ErrAttachmentNotPending
	}
	if err := staffScenePhotoOpen(p, restricted); err != nil {
		return domain.StoredFile{}, err
	}
	return uc.completeOnce(ctx, id, func(ctx context.Context) (domain.StoredFile, error) {
		return uc.completePending(ctx, ma, id, actor, restricted, *f)
	})
}

// completePending is steps 2 and 3 for a file the caller owns and that was `pending`.
func (uc *StaffScenePhotos) completePending(ctx context.Context, ma, id string, actor audit.Actor,
	restricted QuyenXemHanChe, f domain.StoredFile) (domain.StoredFile, error) {

	pol, err := photoPolicyFor(ctx, uc.policies, storage.PurposePetitionPhoto)
	if err != nil {
		return domain.StoredFile{}, err
	}
	dst, err := storage.ParseKey(f.ObjectKey)
	if err != nil {
		return domain.StoredFile{}, fmt.Errorf("ảnh hiện trường (cán bộ): khoá đối tượng đã ghi không hợp lệ: %w", err)
	}

	// 2. Object-store and pixel work, no lock held — the citizen flow's inspector, unchanged.
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
		if !ownStaffScenePhoto(cur, lp.ID, actor.ID) {
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
			if !lp.BookedByStaff() || !domain.PhotoUploadOpen(lp.TrangThai) {
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
				// citizen-media is purgeable: the clean copy that will never be recorded goes now, and the
				// trail says whether that worked.
				removed := uc.objects.PurgeAllVersions(ctx, storage.BucketPrivate, f.ObjectKey) == nil
				if err := uc.files.Transition(ctx, tx, id, domain.StoredFilePending, domain.StoredFileRejected, now); err != nil {
					return err
				}
				return writePhotoAudit(ctx, tx, actor, lp.MaTraCuu, ActionPetitionPhotoRejected, now, map[string]any{
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
			return writePhotoAudit(ctx, tx, actor, lp.MaTraCuu, ActionStaffScenePhotoStored, now, map[string]any{
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
			return writePhotoAudit(ctx, tx, actor, lp.MaTraCuu, ActionPetitionPhotoRejected, now, d)
		case outcomeExpired:
			if err := uc.files.Transition(ctx, tx, id, domain.StoredFilePending, domain.StoredFileFailed, now); err != nil {
				return err
			}
			return writePhotoAudit(ctx, tx, actor, lp.MaTraCuu, ActionPetitionPhotoExpired, now,
				map[string]any{"tep_id": id})
		}
		return fmt.Errorf("ảnh hiện trường (cán bộ): kết quả kiểm tra không rõ (%d)", insp.kind)
	})
	if err != nil {
		return domain.StoredFile{}, bocPhieu(ctx, "cán bộ hoàn tất ảnh hiện trường", err)
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

// Compile-time proof the real dependencies satisfy the interfaces.
var (
	_ StaffScenePhotoPetitions = (*petstore.PhieuPhanAnhStore)(nil)
	_ PetitionPhotoFiles       = (*petstore.StoredFileStore)(nil)
)
