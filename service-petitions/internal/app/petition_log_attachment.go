package app

// `📎 Đính kèm` on the PETITION processing log — docs/ui-ux/09 §8.7 (:197, :312), owner decision B of
// 02/10/2026, storage of migration 0027. The task attachment's three steps (task_attachment.go), for a
// petition:
//
//	a. RequestUpload  POST …/citizen-reports/{ma}/log-attachments                  pending row + form
//	b. the browser    POST straight to OBJECT_STORAGE_PUBLIC_ENDPOINT
//	c. Complete       POST …/citizen-reports/{ma}/log-attachments/{id}/completion  sniff · scan · hash ·
//	                  copy → stored, the trail in ONE transaction
//
// and then the file rides on the NEXT manual note the same officer writes (GhiChuNoiBo, linked in that
// note's transaction — migration 0027's trigger refuses a link written later). DownloadLink hands out a
// short-lived presigned GET.
//
// # STAFF-ONLY — like the log it hangs off (rule 4, forbidden #5; rule 10, invariant 7)
//
// No citizen route reads these files, and the citizen verification-photo read binds its own purpose so
// it cannot return one. A record a member of staff uploaded as evidence: class records, promoted as
// uploaded (it is not shown to a citizen, so there is no EXIF reason to re-encode, and a re-encoded
// record would no longer be the record — core/storage PutServerProduced).
//
// # WHO — the note-writing rule, reused (app.duocGhiChu)
//
// An upload exists only to be attached to a note, so the right to upload is exactly the right to write
// the note: the officer the petition is assigned to, OR a holder of feedback.resolve / feedback.assign
// / feedback.classify — behind the route's `feedback.read` gate. A wider upload right would put files on
// a petition that nobody could attach. `feedback.restricted` narrows, as on every act. Completing is the
// UPLOADER's alone.
//
// # WHEN — every status, like the note (domain.VerificationPhotoUploadOpen explains the difference).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/storage"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// The verbs in the trail — Vietnamese snake_case values (ADR 0011), the task attachment's set.
const (
	ActionPetitionLogAttachmentRequested  = "yeu_cau_tai_tep_nhat_ky_phan_anh"
	ActionPetitionLogAttachmentStored     = "luu_tep_nhat_ky_phan_anh"
	ActionPetitionLogAttachmentRejected   = "tu_choi_tep_nhat_ky_phan_anh"
	ActionPetitionLogAttachmentExpired    = "tep_nhat_ky_phan_anh_het_han_tai"
	ActionPetitionLogAttachmentDownloaded = "tai_tep_nhat_ky_phan_anh"
	// ActionPetitionLogAttachmentRemoved is the soft delete (Remove) — the task twin's `go_tep_nhiem_vu`.
	ActionPetitionLogAttachmentRemoved = "go_tep_phan_anh"
)

// PetitionLogAttachmentPetitions is what the three acts read of the register, plus the timeline write the
// removal appends its line with. *petstore.PhieuPhanAnhStore satisfies it.
type PetitionLogAttachmentPetitions interface {
	VerificationPhotoPetitions
	// vi-name-ok: mirrors the existing PhieuPhanAnhStore method; rule 12 invariant 3 keeps existing names
	GhiNhatKy(ctx context.Context, tx *store.ScopedTx, e domain.NhatKyPhanAnh) error
}

// ErrPetitionLogAttachmentCountReached: the petition already carries platform's
// `max_files_per_subject` log attachments. 409.
var ErrPetitionLogAttachmentCountReached = errors.New("tệp đính kèm: phiếu đã đủ số tệp đính kèm tối đa")

// PetitionLogAttachmentFiles is the part of *petstore.StoredFileStore this use case calls.
type PetitionLogAttachmentFiles interface {
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
	LinkedPetitionLogEntry(ctx context.Context, fileID string) (string, error)
	// The removal's two: the link read on the transaction's connection, and the soft delete.
	LinkedPetitionLogEntryTx(ctx context.Context, tx *store.ScopedTx, fileID string) (string, error)
	SoftDelete(ctx context.Context, tx *store.ScopedTx, id, by, reason string, at time.Time) error
}

// PetitionLogAttachments owns the four attachment acts on a petition's log.
type PetitionLogAttachments struct {
	db        *store.DB
	petitions PetitionLogAttachmentPetitions // the staff register's two reads, and the removal's line
	files     PetitionLogAttachmentFiles

	// ANY OF THE THREE nil means "not configured" (503). Download needs only `objects`.
	objects  ObjectStore
	scanner  MalwareScanner
	policies UploadPolicies

	newID func() (string, error)
	now   func() time.Time
	// logID is the removal's timeline-row id seam; nil means ulid.
	logID func() (string, error)
}

// NewPetitionLogAttachments builds the use case. Pass UNTYPED nil for a dependency that is not
// configured (a nil *storage.Client inside a non-nil interface would panic on first use).
func NewPetitionLogAttachments(db *store.DB, petitions PetitionLogAttachmentPetitions,
	files PetitionLogAttachmentFiles, objects ObjectStore, scanner MalwareScanner,
	policies UploadPolicies) *PetitionLogAttachments {
	return &PetitionLogAttachments{db: db, petitions: petitions, files: files, objects: objects,
		scanner: scanner, policies: policies, newID: storage.NewObjectID}
}

func (uc *PetitionLogAttachments) clock() time.Time {
	if uc.now == nil {
		return time.Now().UTC()
	}
	return uc.now().UTC()
}

func (uc *PetitionLogAttachments) uploadsConfigured() bool {
	return uc.objects != nil && uc.scanner != nil && uc.policies != nil
}

func (uc *PetitionLogAttachments) inspector() uploadInspector {
	return uploadInspector{objects: uc.objects, scanner: uc.scanner, files: uc.files, clock: uc.clock}
}

// mayAttachHere is the two checks every act runs on the petition row: the restricted field (404, first,
// so a 403 cannot confirm a `can-bo` report exists) and the note-writing rule (403).
func mayAttachHere(p domain.PhieuPhanAnh, actor audit.Actor, noteRight QuyenGhiChuCaXa, restricted QuyenXemHanChe) error {
	if err := duocChamPhieuHanChe(p, restricted); err != nil {
		return err
	}
	return duocGhiChu(p, actor, noteRight)
}

// ownPetitionLogUpload: a log attachment of THIS petition, issued to THIS officer.
func ownPetitionLogUpload(f *domain.StoredFile, petitionID, officer string) bool {
	return f != nil && officer != "" && domain.IsPetitionLogAttachmentOf(*f, petitionID) && f.UploadedBy == officer
}

// RequestUpload issues one upload slot for one file on petition `ma`'s log (ADR 0052 §1a). ONE
// TRANSACTION: the petition read FOR UPDATE (the two checks, and two requests serialise so both cannot
// pass the count), the count, the pending row, the form (signed offline), the audit entry.
func (uc *PetitionLogAttachments) RequestUpload(ctx context.Context, ma string, req AttachmentUploadRequest,
	actor audit.Actor, noteRight QuyenGhiChuCaXa, restricted QuyenXemHanChe) (AttachmentUpload, error) {

	if err := coCanBoThucHien(actor); err != nil {
		return AttachmentUpload{}, err
	}
	name, err := domain.CleanAttachmentName(req.FileName)
	if err != nil {
		return AttachmentUpload{}, err
	}
	if req.Size <= 0 {
		return AttachmentUpload{}, domain.ErrAttachmentSizeInvalid
	}
	if !uc.uploadsConfigured() {
		return AttachmentUpload{}, ErrUploadNotConfigured
	}
	pol, err := attachmentPolicyFor(ctx, uc.policies, storage.PurposePetitionLogAttachment)
	if err != nil {
		return AttachmentUpload{}, err
	}
	if !pol.AllowsMIME(req.ContentType) {
		return AttachmentUpload{}, ErrAttachmentTypeNotAllowed
	}
	if req.Size > pol.MaxBytes {
		return AttachmentUpload{}, ErrAttachmentTooLarge
	}
	ext, ok := storage.ExtForMIME(req.ContentType)
	if !ok {
		return AttachmentUpload{}, ErrAttachmentTypeNotAllowed
	}

	id, err := uc.newID()
	if err != nil {
		return AttachmentUpload{}, fmt.Errorf("tệp đính kèm: sinh mã tệp: %w", err)
	}
	now := uc.clock()
	key := storage.Key{
		Class: storage.ClassRecords, TenantID: string(tenant.MustFrom(ctx)), CreatedAt: now,
		Service: storage.ServicePetitions, Purpose: storage.PurposePetitionLogAttachment,
		ObjectID: id, Variant: storage.VariantOriginal, Ext: ext,
	}
	objectKey, err := key.Path()
	if err != nil {
		return AttachmentUpload{}, fmt.Errorf("tệp đính kèm: dựng khoá đối tượng: %w", err)
	}
	uploadKey, err := key.UploadPath()
	if err != nil {
		return AttachmentUpload{}, fmt.Errorf("tệp đính kèm: dựng khoá tải lên: %w", err)
	}

	var out AttachmentUpload
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		p, err := uc.petitions.TheoMaTraCuuDeSua(ctx, tx, ma)
		if err != nil {
			return err
		}
		if err := mayAttachHere(p, actor, noteRight, restricted); err != nil {
			return err
		}
		if pol.FileCountLimited {
			live, err := uc.files.CountForSubjectTx(ctx, tx, domain.StoredFileSubjectPetition, p.ID,
				domain.PurposePetitionLogAttachment, now.Add(-storage.UploadTTL))
			if err != nil {
				return err
			}
			if live >= pol.MaxFilesPerSubject {
				return ErrPetitionLogAttachmentCountReached
			}
		}
		f := domain.StoredFile{
			ID: id, Bucket: domain.StoredFileBucketPrivate, ObjectKey: objectKey,
			RetentionClass: string(storage.ClassRecords), Purpose: domain.PurposePetitionLogAttachment,
			SubjectType: domain.StoredFileSubjectPetition, SubjectID: p.ID, OriginalName: name,
			Status: domain.StoredFilePending, UploadedBy: actor.ID, CreatedAt: now, UpdatedAt: now,
		}
		if err := uc.files.InsertPending(ctx, tx, f); err != nil {
			return err
		}
		post, err := uc.objects.PresignUpload(ctx, uploadKey, pol.MaxBytes, req.ContentType, storage.UploadTTL)
		if err != nil {
			return fmt.Errorf("tệp đính kèm: ký lượt tải lên: %w", err)
		}
		if err := writeAttachmentAudit(ctx, tx, actor, p.MaTraCuu, ActionPetitionLogAttachmentRequested, now,
			map[string]any{
				"tep_id":          id,
				"muc_dich":        domain.PurposePetitionLogAttachment,
				"loai_khai_bao":   req.ContentType,
				"kich_thuoc_khai": req.Size,
			}); err != nil {
			return err
		}
		out = AttachmentUpload{File: f, Post: post}
		return nil
	})
	if err != nil {
		return AttachmentUpload{}, bocPhieu(ctx, "xin tải tệp đính kèm nhật ký", err)
	}
	return out, nil
}

// Complete is ADR 0052 §1c for one upload — the task attachment's flow, on the shared uploadInspector.
// IDEMPOTENT: a file already `stored` is returned as it is (idem.KhongCan on the route).
func (uc *PetitionLogAttachments) Complete(ctx context.Context, ma, id string, actor audit.Actor,
	noteRight QuyenGhiChuCaXa, restricted QuyenXemHanChe) (domain.StoredFile, error) {

	if err := coCanBoThucHien(actor); err != nil {
		return domain.StoredFile{}, err
	}
	if !uc.uploadsConfigured() {
		return domain.StoredFile{}, ErrUploadNotConfigured
	}

	// 1. Lock-free pre-read.
	p, err := uc.petitions.TheoMaTraCuu(ctx, ma)
	if err != nil {
		return domain.StoredFile{}, bocPhieu(ctx, "hoàn tất tệp đính kèm nhật ký", err)
	}
	if err := mayAttachHere(p, actor, noteRight, restricted); err != nil {
		return domain.StoredFile{}, bocPhieu(ctx, "hoàn tất tệp đính kèm nhật ký", err)
	}
	f, err := uc.files.ByID(ctx, id)
	if err != nil {
		return domain.StoredFile{}, bocPhieu(ctx, "hoàn tất tệp đính kèm nhật ký", err)
	}
	if !ownPetitionLogUpload(f, p.ID, actor.ID) {
		return domain.StoredFile{}, ErrAttachmentNotFound
	}
	switch {
	case f.Status.Attachable() || f.Status == domain.StoredFileProcessing:
		return *f, nil
	case f.Status != domain.StoredFilePending:
		return domain.StoredFile{}, ErrAttachmentNotPending
	}
	pol, err := attachmentPolicyFor(ctx, uc.policies, storage.PurposePetitionLogAttachment)
	if err != nil {
		return domain.StoredFile{}, err
	}
	key, err := storage.ParseKey(f.ObjectKey)
	if err != nil {
		return domain.StoredFile{}, fmt.Errorf("tệp đính kèm: khoá đối tượng đã ghi không hợp lệ: %w", err)
	}

	// 2. The object-store work, no lock held.
	insp, err := uc.inspector().inspect(ctx, *f, key, pol)
	if err != nil {
		return domain.StoredFile{}, err
	}
	if insp.kind == outcomeNotReceived {
		return domain.StoredFile{}, fmt.Errorf("%w: %s", ErrUploadNotReceived, attachmentNotReceivedDetail(*f, key))
	}

	// 3. One short transaction writes the outcome, re-checking everything it depends on.
	now := uc.clock()
	var stored domain.StoredFile
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		lp, err := uc.petitions.TheoMaTraCuuDeSua(ctx, tx, ma)
		if err != nil {
			return err
		}
		if err := mayAttachHere(lp, actor, noteRight, restricted); err != nil {
			return err
		}
		cur, err := uc.files.ForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if !ownPetitionLogUpload(cur, lp.ID, actor.ID) {
			return ErrAttachmentNotFound
		}
		if cur.Status.Attachable() || cur.Status == domain.StoredFileProcessing {
			stored = *cur
			return nil
		}
		if cur.Status != domain.StoredFilePending {
			return ErrAttachmentNotPending
		}
		switch insp.kind {
		case outcomeStored:
			if err := uc.files.Transition(ctx, tx, id, domain.StoredFilePending, domain.StoredFileScanning, now); err != nil {
				return err
			}
			if err := uc.files.MarkStored(ctx, tx, id, insp.facts, now); err != nil {
				return err
			}
			stored = *cur
			stored.Status, stored.MIMEType, stored.SizeBytes, stored.SHA256, stored.UpdatedAt =
				domain.StoredFileStored, insp.facts.MIMEType, insp.facts.SizeBytes, insp.facts.SHA256, now
			return writeAttachmentAudit(ctx, tx, actor, lp.MaTraCuu, ActionPetitionLogAttachmentStored, now,
				map[string]any{
					"tep_id": id, "loai_tep": insp.facts.MIMEType, "kich_thuoc": insp.facts.SizeBytes,
					"sha256": insp.facts.SHA256, "khoi_phuc_tu_dich": insp.recovered,
				})
		case outcomeRejected:
			if err := uc.files.Transition(ctx, tx, id, domain.StoredFilePending, domain.StoredFileRejected, now); err != nil {
				return err
			}
			d := map[string]any{"tep_id": id, "ly_do": insp.reason, "da_xoa_tep_tam": insp.tempRemoved}
			if insp.signature != "" {
				d["chu_ky_ma_doc"] = insp.signature
			}
			return writeAttachmentAudit(ctx, tx, actor, lp.MaTraCuu, ActionPetitionLogAttachmentRejected, now, d)
		case outcomeExpired:
			if err := uc.files.Transition(ctx, tx, id, domain.StoredFilePending, domain.StoredFileFailed, now); err != nil {
				return err
			}
			return writeAttachmentAudit(ctx, tx, actor, lp.MaTraCuu, ActionPetitionLogAttachmentExpired, now,
				map[string]any{"tep_id": id})
		}
		return fmt.Errorf("tệp đính kèm: kết quả kiểm tra không rõ (%d)", insp.kind)
	})
	if err != nil {
		return domain.StoredFile{}, bocPhieu(ctx, "hoàn tất tệp đính kèm nhật ký", err)
	}
	switch {
	case stored.ID != "":
		return stored, nil
	case insp.kind == outcomeRejected:
		return domain.StoredFile{}, &AttachmentRejection{Reason: insp.reason}
	default: // outcomeExpired
		return domain.StoredFile{}, fmt.Errorf("%w: %s", ErrUploadExpired, attachmentNotReceivedDetail(*f, key))
	}
}

// DownloadLink hands one staff reader of petition `ma` a short-lived link to one of its log files
// (domain.MayDownloadPetitionLogAttachment decides which). Route gate `feedback.read` — the key of the
// timeline the file is shown on; `feedback.restricted` narrows to the 404 of an unknown code.
//
// AUDITED, AND THE TASK TWIN IS NOT — the difference is deliberate. A task file is a staff file about
// staff work; a file on a PETITION's log is evidence about one citizen's case (a scan of a letter, a
// photo of their street) and cannot be masked, so handing out a link is a read of full personal data
// (rule 6, invariant 7) — the footing the scene photos' staff list is on. The entry is COMMITTED BEFORE
// the link is returned; no entry, no link. It is written after the link is signed, so a signing failure
// leaves no entry for a disclosure that never happened.
func (uc *PetitionLogAttachments) DownloadLink(ctx context.Context, ma, id string, reader audit.Actor,
	restricted QuyenXemHanChe) (AttachmentDownload, error) {

	if reader.ID == "" || reader.Kind != chuThePhaiLaCanBo {
		return AttachmentDownload{}, fmt.Errorf("tệp đính kèm: người đọc không phải cán bộ có mã (kind=%q) — "+
			"không ghi được vết thì không mở tệp", reader.Kind)
	}
	if uc.objects == nil {
		return AttachmentDownload{}, ErrUploadNotConfigured
	}
	p, err := uc.petitions.TheoMaTraCuu(ctx, ma)
	if err != nil {
		return AttachmentDownload{}, bocPhieu(ctx, "tải tệp đính kèm nhật ký", err)
	}
	if err := duocChamPhieuHanChe(p, restricted); err != nil {
		return AttachmentDownload{}, bocPhieu(ctx, "tải tệp đính kèm nhật ký", err)
	}
	f, err := uc.files.ByID(ctx, id)
	if err != nil {
		return AttachmentDownload{}, bocPhieu(ctx, "tải tệp đính kèm nhật ký", err)
	}
	if f == nil || f.Bucket != domain.StoredFileBucketPrivate {
		return AttachmentDownload{}, ErrAttachmentNotFound
	}
	linked, err := uc.files.LinkedPetitionLogEntry(ctx, id)
	if err != nil {
		return AttachmentDownload{}, bocPhieu(ctx, "tải tệp đính kèm nhật ký", err)
	}
	if !domain.MayDownloadPetitionLogAttachment(*f, p.ID, linked, reader.ID) {
		return AttachmentDownload{}, ErrAttachmentNotFound
	}
	now := uc.clock()
	u, err := uc.objects.PresignDownload(ctx, storage.BucketPrivate, f.ObjectKey, storage.MaxDownloadTTL,
		f.OriginalName)
	if err != nil {
		return AttachmentDownload{}, storageErr("ký liên kết tải về", err)
	}
	// The file id and the entry it is on — never the name (personal data), never the URL (a credential).
	delta, err := json.Marshal(map[string]any{"tep_id": id, "nhat_ky_id": linked, "quyen": "feedback.read"})
	if err != nil {
		return AttachmentDownload{}, fmt.Errorf("tệp đính kèm: mã hoá delta: %w", err)
	}
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		return audit.Write(ctx, tx, audit.Entry{Actor: reader, Action: ActionPetitionLogAttachmentDownloaded,
			Subject: p.MaTraCuu, At: now, Delta: delta})
	})
	if err != nil {
		return AttachmentDownload{}, fmt.Errorf("tệp đính kèm: ghi vết tải tệp cho xã %s: %w", tenant.MustFrom(ctx), err)
	}
	return AttachmentDownload{URL: u, ExpiresAt: now.Add(storage.MaxDownloadTTL)}, nil
}

// Remove soft-deletes one log attachment of petition `ma` — the task twin (TaskAttachments.Remove, user
// decision 07/10/2026) on a petition (owner decision 09/10/2026, ADR 0076 §4b's shape): THE UPLOADER, OR
// A HOLDER OF `feedback.resolve` (`resolve`). Route permission: `feedback.read`, the gate; this decides.
//
// WHICH FILES: only one the caller can SEE (domain.MayDownloadPetitionLogAttachment) — this petition's
// log attachment, stored, on a log entry OR the caller's own draft. Anything else — unknown, another
// commune's, another petition's, a scene or verification photo, somebody else's draft, already removed —
// is ErrAttachmentNotFound, one answer. A visible file the caller neither uploaded nor holds
// `feedback.resolve` for is domain.ErrAttachmentRemovalNotAllowed (403: the file is visible to them, so
// hiding its existence hides nothing). `can-bo` without `feedback.restricted` is the petition's 404.
//
// ONE TRANSACTION: the petition FOR UPDATE (petition then file — the order RequestUpload, Complete and
// the note take), the file FOR UPDATE, the soft delete, the timeline line, the audit entry. A refusal
// writes nothing.
//
// THE OBJECT IS NOT TOUCHED and THE LINK STAYS: a records file is disposed of by the records schedule,
// never by a command (ADR 0052 §6, §7; rule 7), and petition_log_attachment is append-only — "this entry
// was written with this file" stays true. Every read path skips the deleted row (ByID, the timeline's
// batched read, the attach candidates).
//
// ⚠ THE TIMELINE LINE IS A `ghi-chu` ROW written by the remover, carrying domain.AttachmentRemovalLogText —
// the task twin's sentence, with no file name. A dedicated act code (`go-tep`) would need migration 0030's
// CHECK `nhat_ky_phan_anh_hanh_vi_hop_le` widened — a migration on a populated table, not built here.
//
// THE TRAIL carries the file id, the entry it was on, the new line's id and the reason's LENGTH — never
// the reason's text nor the file name (rule 6, forbidden #4; rule 3).
func (uc *PetitionLogAttachments) Remove(ctx context.Context, ma, id, reasonRaw string, actor audit.Actor,
	resolve QuyenXuLyCaXa, restricted QuyenXemHanChe) error {

	reason, err := domain.CheckAttachmentRemovalReason(reasonRaw)
	if err != nil {
		return err
	}
	if err := coCanBoThucHien(actor); err != nil {
		return err
	}
	text, err := domain.KiemGhiChu(domain.AttachmentRemovalLogText(reason))
	if err != nil {
		return err
	}
	now := uc.clock()

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		p, err := uc.petitions.TheoMaTraCuuDeSua(ctx, tx, ma)
		if err != nil {
			return err
		}
		if err := duocChamPhieuHanChe(p, restricted); err != nil {
			return err
		}
		f, err := uc.files.ForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if f == nil {
			return ErrAttachmentNotFound
		}
		linked, err := uc.files.LinkedPetitionLogEntryTx(ctx, tx, id)
		if err != nil {
			return err
		}
		if !domain.MayDownloadPetitionLogAttachment(*f, p.ID, linked, actor.ID) {
			return ErrAttachmentNotFound
		}
		// THE DOOR, uploader first — the narrower, truer reason, named in the trail.
		door := "nguoi-tai-len"
		if f.UploadedBy != actor.ID {
			if !resolve {
				return domain.ErrAttachmentRemovalNotAllowed
			}
			door = "ket-thuc-xu-ly-phan-anh"
		}
		if f.LegalHold {
			return domain.ErrAttachmentUnderLegalHold
		}

		// `deleted_by` IS THE STAFF BUSINESS CODE (rule 6, invariant 8); coCanBoThucHien refused an empty one.
		if err := uc.files.SoftDelete(ctx, tx, id, actor.ID, reason, now); err != nil {
			return err
		}

		logID, err := uc.newLogID()
		if err != nil {
			return fmt.Errorf("nhat_ky_phan_anh: sinh mã nội bộ: %w", err)
		}
		if err := uc.petitions.GhiNhatKy(ctx, tx, domain.NhatKyPhanAnh{
			ID: logID, PhieuPhanAnhID: p.ID, ThoiDiem: now, NguoiMa: actor.ID,
			HanhVi: domain.NhatKyGhiChu, TrangThai: p.TrangThai, NoiDung: text,
		}); err != nil {
			return err
		}

		return writeAttachmentAudit(ctx, tx, actor, p.MaTraCuu, ActionPetitionLogAttachmentRemoved, now, map[string]any{
			"tep_id":       id,
			"nhat_ky_id":   logID,
			"gan_nhat_ky":  linked, // "" when the file was a draft on no entry
			"quyen_go":     door,
			"do_dai_ly_do": utf8.RuneCountInString(reason),
			"truoc":        map[string]any{"trang_thai": string(f.Status), "da_go": false},
			"sau":          map[string]any{"trang_thai": string(f.Status), "da_go": true},
		})
	})
	if err != nil {
		return bocPhieu(ctx, "gỡ tệp đính kèm nhật ký", err)
	}
	return nil
}

// newLogID is the timeline row id seam; ulid in production.
func (uc *PetitionLogAttachments) newLogID() (string, error) {
	if uc.logID == nil {
		return ulid.Moi()
	}
	return uc.logID()
}

// Compile-time proof the real dependencies satisfy the interfaces.
var (
	_ PetitionLogAttachmentFiles     = (*petstore.StoredFileStore)(nil)
	_ PetitionLogAttachmentPetitions = (*petstore.PhieuPhanAnhStore)(nil)
)
