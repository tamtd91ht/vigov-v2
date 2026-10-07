package app

// `📎 Đính kèm` on the manual task log entry — docs/ui-ux/02-nhiem-vu.md §5.9 — built on ADR 0052's
// three-step upload (kb/10-decisions/0052-object-storage-minio.md §1):
//
//	a. RequestUpload  POST /api/v1/tasks/{ma}/attachments                  pending row + presigned POST
//	b. the browser    POST straight to OBJECT_STORAGE_PUBLIC_ENDPOINT       bytes never cross this service
//	c. Complete       POST /api/v1/tasks/{ma}/attachments/{id}/completion   sniff · scan · hash · copy → stored
//
// and then the file rides on the NEXT log entry the same officer writes (AddLogEntry, task_log_entry.go),
// linked in THAT entry's transaction. DownloadLink hands out a short-lived presigned GET. Remove
// soft-deletes a file (`Gỡ`) with a mandatory reason and a timeline line — the uploader's or a
// `task.update` holder's act, not the log-entry right (user decision 07/10/2026).
//
// # WHO
//
// The same people who may write the log (domain.TaskWorkRightFor / CheckMayWriteLogEntry): an upload
// exists only to be attached to an entry, so a right to upload that exceeded the right to write the
// entry would be a way to put files on a task nobody could attach. Completing is the UPLOADER's alone —
// the row is bound to one officer at issue time (migration 0021 freezes `uploaded_by`), and another
// officer's id answers exactly like an unknown one (rule 4, forbidden #2, applied to staff).
//
// # THE LIMITS ARE PLATFORM'S, NEVER THIS FILE'S (ADR 0052 §10, stop condition #4)
//
// Size, types and the per-task count come from uploadpolicy on every request and again at completion
// (the policy CURRENT AT COMPLETE decides — platform.proto, ListUploadPolicies). Not configured is a
// refusal; unreachable is a 503. There is no default anywhere below.
//
// # WHY THE OBJECT-STORE WORK HAPPENS OUTSIDE ANY TRANSACTION
//
// Sniffing, scanning and hashing a file is network I/O of unbounded length (clamd streams the whole
// object). Holding the task row or the file row locked across it would stall every other write on the
// task for as long as the scanner takes. So Complete inspects first, lock-free, and then writes the
// outcome in ONE short transaction that re-reads the row FOR UPDATE and re-checks everything the
// outcome depends on. Two completions racing on one upload: both inspect, one promotes, the other finds
// the destination taken — and the recovery path (measure the destination) makes it land on the same
// `stored` row instead of an error.
//
// # WHAT IS NEVER LOGGED OR PUT IN AN ERROR
//
// The original file name (personal data when it describes a case — rule 3, ADR 0052 §3), any presigned
// URL or form field (bearer credentials, core/storage), and file content. The trail carries the file id,
// the SNIFFED type, the size, the hash and the reason for a refusal; never the name.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
	"unicode/utf8"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/malwarescan"
	"github.com/vihat/vigov/core/platformclient/uploadpolicy"
	"github.com/vihat/vigov/core/storage"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// The verbs in the trail — Vietnamese snake_case like every other value this service writes (ADR 0011).
const (
	ActionTaskAttachmentRequested = "yeu_cau_tai_tep_nhiem_vu"
	ActionTaskAttachmentStored    = "luu_tep_nhiem_vu"
	ActionTaskAttachmentRejected  = "tu_choi_tep_nhiem_vu"
	ActionTaskAttachmentExpired   = "tep_nhiem_vu_het_han_tai"
	ActionTaskAttachmentRemoved   = "go_tep_nhiem_vu"
)

// Rejection reasons — the `ly_do` of a `tu_choi_tep_nhiem_vu` entry and the key the handler picks its
// sentence by. Values, so Vietnamese without diacritics (ADR 0011).
const (
	RejectMalware        = "nhiem-ma-doc"
	RejectTypeNotAllowed = "sai-kieu-tep"
	RejectTypeMismatch   = "khac-kieu-khai-bao"
	RejectTooLarge       = "vuot-dung-luong"
	RejectCountReached   = "vuot-so-tep"
)

var (
	// ErrUploadNotConfigured: object storage, the malware scanner, or platform's limit for this
	// purpose is absent. Every upload is refused (ADR 0052 §9, §10). 503 — a deployment state, not a
	// fault of the request, and nothing an officer can change.
	ErrUploadNotConfigured = errors.New("tệp đính kèm: chưa cấu hình kho lưu tệp")
	// ErrUploadLimitsUnavailable: platform could not be asked and no answer younger than its TTL is
	// held (uploadpolicy.ErrUnavailable). Retryable, 503. Never "allowed".
	ErrUploadLimitsUnavailable = errors.New("tệp đính kèm: chưa đọc được giới hạn tải tệp")
	// ErrScanUnavailable: clamd could not scan the file (unreachable, timed out, over its size limit,
	// an unknown reply). The row stays `pending` and the completion can be retried; the file is NEVER
	// stored unscanned (ADR 0052 §9). 503.
	ErrScanUnavailable = errors.New("tệp đính kèm: chưa quét được mã độc")
	// ErrAttachmentTypeNotAllowed: the DECLARED type is outside platform's list. 400, before any row.
	ErrAttachmentTypeNotAllowed = errors.New("tệp đính kèm: loại tệp không được phép")
	// ErrAttachmentTooLarge: the DECLARED size is above platform's limit. 400, before any row.
	ErrAttachmentTooLarge = errors.New("tệp đính kèm: tệp vượt dung lượng cho phép")
	// ErrAttachmentCountReached: the task already carries platform's `max_files_per_subject`. 409.
	ErrAttachmentCountReached = errors.New("tệp đính kèm: nhiệm vụ đã đủ số tệp tối đa")
	// ErrAttachmentNotFound: no such file ON THIS TASK FOR THIS CALLER — unknown, another commune's,
	// another task's, another officer's unfinished upload. One answer for all of them. 404.
	ErrAttachmentNotFound = errors.New("tệp đính kèm: không tìm thấy")
	// ErrAttachmentNotPending: completion of a file that was already refused or expired. 409.
	ErrAttachmentNotPending = errors.New("tệp đính kèm: tệp không còn chờ hoàn tất")
	// ErrUploadNotReceived: completion before the bytes arrived, while the form is still valid. Nothing
	// is written; the browser finishes the upload and calls again. 409.
	ErrUploadNotReceived = errors.New("tệp đính kèm: chưa nhận được tệp")
	// ErrUploadExpired: the form expired and nothing arrived. The row moves to `failed`. 409.
	ErrUploadExpired = errors.New("tệp đính kèm: lượt tải lên đã hết hạn")
	// ErrUploadChanged: the object was replaced through the still-valid form while it was being
	// inspected (storage.ErrChanged). Nothing is written; calling again inspects the new bytes. 409.
	ErrUploadChanged = errors.New("tệp đính kèm: tệp vừa bị thay đổi trong lúc kiểm tra")
	// ErrAttachmentRejected is what every *AttachmentRejection matches.
	ErrAttachmentRejected = errors.New("tệp đính kèm: tệp bị từ chối")
)

// AttachmentRejection is a completion that refused the FILE: the row is now `rejected`, the temp object
// has been deleted (or will expire within a day), and the trail says why. 422.
type AttachmentRejection struct{ Reason string }

func (e *AttachmentRejection) Error() string   { return ErrAttachmentRejected.Error() + ": " + e.Reason }
func (e *AttachmentRejection) Is(t error) bool { return t == ErrAttachmentRejected }

// ObjectStore is the part of *storage.Client this use case calls — an interface so the tests run with
// no MinIO. The semantics are core/storage's; read storage.go before implementing another.
type ObjectStore interface {
	PresignUpload(ctx context.Context, uploadKey string, maxBytes int64, contentType string,
		ttl time.Duration) (storage.PresignedPost, error)
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

// MalwareScanner is *malwarescan.Scanner.
type MalwareScanner interface {
	Scan(ctx context.Context, r io.Reader, size int64) (malwarescan.Result, error)
}

// UploadPolicies is *uploadpolicy.Reader.
type UploadPolicies interface {
	Policy(ctx context.Context, purpose storage.Purpose) (uploadpolicy.Policy, bool, error)
}

// TaskAttachmentFiles is the part of *store.StoredFileStore this use case calls.
type TaskAttachmentFiles interface {
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
	LinkedLogEntry(ctx context.Context, fileID string) (string, error)
	LinkedLogEntryTx(ctx context.Context, tx *store.ScopedTx, fileID string) (string, error)
	SoftDelete(ctx context.Context, tx *store.ScopedTx, id, by, reason string, at time.Time) error
}

// TaskRows is the task reads (and the one timeline write) this use case needs — *store.NhiemVuStore
// satisfies it.
type TaskRows interface {
	LiveByCode(ctx context.Context, code string) (domain.NhiemVu, error)
	// vi-name-ok: mirrors the existing NhiemVuStore method; rule 12 invariant 3 keeps existing names
	TheoMaDeSua(ctx context.Context, tx *store.ScopedTx, ma string) (domain.NhiemVu, error)
	// vi-name-ok: mirrors the existing NhiemVuStore method; rule 12 invariant 3 keeps existing names
	GhiNhatKy(ctx context.Context, tx *store.ScopedTx, e domain.NhatKyNhiemVu) error
}

// TaskAttachments owns the three attachment acts.
type TaskAttachments struct {
	db    *store.DB
	tasks TaskRows
	files TaskAttachmentFiles

	// The three dependencies ADR 0052 adds. ANY OF THEM nil means "not configured" and every upload is
	// refused with ErrUploadNotConfigured — fail closed, while the rest of the service keeps serving.
	// Download needs only `objects`.
	objects  ObjectStore
	scanner  MalwareScanner
	policies UploadPolicies

	newID func() (string, error)
	// newLogID mints the timeline row a removal appends — the same generator every other task log
	// row is minted with (GhiNhiemVu.sinhID), never the object-id one.
	newLogID func() (string, error)
	now      func() time.Time
}

// NewTaskAttachments builds the use case. Pass UNTYPED nil for a dependency that is not configured —
// a nil *storage.Client inside a non-nil interface would pass the nil check and panic on first use.
func NewTaskAttachments(db *store.DB, tasks TaskRows, files TaskAttachmentFiles, objects ObjectStore,
	scanner MalwareScanner, policies UploadPolicies) *TaskAttachments {
	return &TaskAttachments{db: db, tasks: tasks, files: files, objects: objects, scanner: scanner,
		policies: policies, newID: storage.NewObjectID, newLogID: ulid.Moi}
}

func (uc *TaskAttachments) clock() time.Time {
	if uc.now == nil {
		return time.Now().UTC()
	}
	return uc.now().UTC()
}

func (uc *TaskAttachments) uploadsConfigured() bool {
	return uc.objects != nil && uc.scanner != nil && uc.policies != nil
}

// policy reads platform's limit for task attachments, mapping the three answers of uploadpolicy.
func (uc *TaskAttachments) policy(ctx context.Context) (uploadpolicy.Policy, error) {
	return attachmentPolicyFor(ctx, uc.policies, storage.PurposeTaskAttachment)
}

// attachmentPolicyFor reads platform's limit for one attachment purpose — the task's and the petition
// log's — mapping the three answers of uploadpolicy. There is no default anywhere (ADR 0052 stop #4).
func attachmentPolicyFor(ctx context.Context, policies UploadPolicies, purpose storage.Purpose) (
	uploadpolicy.Policy, error) {
	p, ok, err := policies.Policy(ctx, purpose)
	switch {
	case errors.Is(err, uploadpolicy.ErrUnavailable):
		return uploadpolicy.Policy{}, fmt.Errorf("%w: %w", ErrUploadLimitsUnavailable, err)
	case err != nil:
		return uploadpolicy.Policy{}, fmt.Errorf("tệp đính kèm: đọc giới hạn tải tệp: %w", err)
	case !ok:
		return uploadpolicy.Policy{}, fmt.Errorf("%w: platform has no limit for %s",
			ErrUploadNotConfigured, purpose)
	}
	return p, nil
}

// --- a. request an upload ---------------------------------------------------------------------------

// AttachmentUploadRequest is what the browser declares before it uploads. The declaration is checked
// here and the bytes again at completion: a claim is never trusted as the fact (ADR 0052 §1c).
type AttachmentUploadRequest struct {
	FileName    string
	ContentType string // the DECLARED type; the key's extension follows it, completion sniffs the truth
	Size        int64
}

// AttachmentUpload is the pending row and the form the browser posts the file with.
type AttachmentUpload struct {
	File domain.StoredFile
	Post storage.PresignedPost // bearer credential for its TTL — never logged
}

// RequestUpload issues one upload slot for one file on one task (ADR 0052 §1a).
//
// ONE TRANSACTION: the task row read FOR UPDATE (who may write, and it serialises two requests for one
// task so both cannot pass the count), the count, the pending row, the audit entry. The presigned POST
// is signed INSIDE it — offline signing, no network — so a signing failure leaves no row behind.
func (uc *TaskAttachments) RequestUpload(ctx context.Context, ma string, req AttachmentUploadRequest,
	actor audit.Actor, update TaskUpdateRight) (AttachmentUpload, error) {

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
	pol, err := uc.policy(ctx)
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
		// Unreachable while uploadpolicy narrows to the storage allow-list; refused rather than trusted.
		return AttachmentUpload{}, ErrAttachmentTypeNotAllowed
	}

	id, err := uc.newID()
	if err != nil {
		return AttachmentUpload{}, fmt.Errorf("tệp đính kèm: sinh mã tệp: %w", err)
	}
	now := uc.clock()
	key := storage.Key{
		Class: storage.ClassRecords, TenantID: string(tenant.MustFrom(ctx)), CreatedAt: now,
		Service: storage.ServicePetitions, Purpose: storage.PurposeTaskAttachment,
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
		n, err := uc.tasks.TheoMaDeSua(ctx, tx, ma)
		if err != nil {
			return err
		}
		if err := domain.CheckMayWriteLogEntry(domain.TaskWorkRightFor(n, actor.ID, bool(update))); err != nil {
			return err
		}
		if pol.FileCountLimited {
			live, err := uc.files.CountForSubjectTx(ctx, tx, domain.StoredFileSubjectTask, n.ID,
				string(storage.PurposeTaskAttachment), now.Add(-storage.UploadTTL))
			if err != nil {
				return err
			}
			if live >= pol.MaxFilesPerSubject {
				return ErrAttachmentCountReached
			}
		}

		f := domain.StoredFile{
			ID: id, Bucket: domain.StoredFileBucketPrivate, ObjectKey: objectKey,
			RetentionClass: string(storage.ClassRecords), Purpose: string(storage.PurposeTaskAttachment),
			SubjectType: domain.StoredFileSubjectTask, SubjectID: n.ID, OriginalName: name,
			Status: domain.StoredFilePending, UploadedBy: actor.ID, CreatedAt: now, UpdatedAt: now,
		}
		if err := uc.files.InsertPending(ctx, tx, f); err != nil {
			return err
		}
		post, err := uc.objects.PresignUpload(ctx, uploadKey, pol.MaxBytes, req.ContentType, storage.UploadTTL)
		if err != nil {
			return fmt.Errorf("tệp đính kèm: ký lượt tải lên: %w", err)
		}
		if err := writeAttachmentAudit(ctx, tx, actor, n.Ma, ActionTaskAttachmentRequested, now,
			map[string]any{
				"tep_id":          id,
				"muc_dich":        string(storage.PurposeTaskAttachment),
				"loai_khai_bao":   req.ContentType,
				"kich_thuoc_khai": req.Size,
			}); err != nil {
			return err
		}
		out = AttachmentUpload{File: f, Post: post}
		return nil
	})
	if err != nil {
		return AttachmentUpload{}, bocNhiemVu(ctx, "xin tải tệp đính kèm", err)
	}
	return out, nil
}

// --- c. complete an upload --------------------------------------------------------------------------

type outcomeKind int

const (
	outcomeStored outcomeKind = iota + 1
	outcomeRejected
	outcomeNotReceived
	outcomeExpired
)

// inspection is what the lock-free half of Complete found.
type inspection struct {
	kind        outcomeKind
	facts       domain.StoredFileFacts
	recovered   bool   // measured at the destination: a previous completion promoted it
	reason      string // outcomeRejected
	signature   string // RejectMalware: the ClamAV signature name, never content
	tempRemoved bool   // outcomeRejected: the temp object is gone now
}

// Complete is ADR 0052 §1c for one upload: Stat, sniff, check against the CURRENT policy, scan, hash,
// promote to the private bucket at the records key, then `stored` and the trail in ONE transaction.
//
// IDEMPOTENT: a file already `stored` is returned as it is, so a retried request (a dropped reply, a
// double click) answers the same outcome. That is why the route declares idem.KhongCan.
func (uc *TaskAttachments) Complete(ctx context.Context, ma, id string, actor audit.Actor,
	update TaskUpdateRight) (domain.StoredFile, error) {

	if err := coCanBoThucHien(actor); err != nil {
		return domain.StoredFile{}, err
	}
	if !uc.uploadsConfigured() {
		return domain.StoredFile{}, ErrUploadNotConfigured
	}

	// 1. Lock-free pre-read: is this the caller's upload on this task, and is there anything to do?
	n, err := uc.tasks.LiveByCode(ctx, ma)
	if err != nil {
		return domain.StoredFile{}, bocNhiemVu(ctx, "hoàn tất tệp đính kèm", err)
	}
	f, err := uc.files.ByID(ctx, id)
	if err != nil {
		return domain.StoredFile{}, bocNhiemVu(ctx, "hoàn tất tệp đính kèm", err)
	}
	if !ownUpload(f, n.ID, actor.ID) {
		return domain.StoredFile{}, ErrAttachmentNotFound
	}
	if err := domain.CheckMayWriteLogEntry(domain.TaskWorkRightFor(n, actor.ID, bool(update))); err != nil {
		return domain.StoredFile{}, err
	}
	switch {
	case f.Status.Attachable() || f.Status == domain.StoredFileProcessing:
		return *f, nil
	case f.Status != domain.StoredFilePending:
		return domain.StoredFile{}, ErrAttachmentNotPending
	}
	pol, err := uc.policy(ctx)
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
		return domain.StoredFile{}, ErrUploadNotReceived
	}

	// 3. One short transaction writes the outcome, re-checking everything it depends on.
	now := uc.clock()
	var stored domain.StoredFile
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		n, err := uc.tasks.TheoMaDeSua(ctx, tx, ma)
		if err != nil {
			return err
		}
		if err := domain.CheckMayWriteLogEntry(domain.TaskWorkRightFor(n, actor.ID, bool(update))); err != nil {
			return err
		}
		cur, err := uc.files.ForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if !ownUpload(cur, n.ID, actor.ID) {
			return ErrAttachmentNotFound
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
			if err := uc.files.Transition(ctx, tx, id, domain.StoredFilePending, domain.StoredFileScanning, now); err != nil {
				return err
			}
			if err := uc.files.MarkStored(ctx, tx, id, insp.facts, now); err != nil {
				return err
			}
			stored = *cur
			stored.Status, stored.MIMEType, stored.SizeBytes, stored.SHA256, stored.UpdatedAt =
				domain.StoredFileStored, insp.facts.MIMEType, insp.facts.SizeBytes, insp.facts.SHA256, now
			return writeAttachmentAudit(ctx, tx, actor, n.Ma, ActionTaskAttachmentStored, now, map[string]any{
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
			return writeAttachmentAudit(ctx, tx, actor, n.Ma, ActionTaskAttachmentRejected, now, d)
		case outcomeExpired:
			if err := uc.files.Transition(ctx, tx, id, domain.StoredFilePending, domain.StoredFileFailed, now); err != nil {
				return err
			}
			return writeAttachmentAudit(ctx, tx, actor, n.Ma, ActionTaskAttachmentExpired, now,
				map[string]any{"tep_id": id})
		}
		return fmt.Errorf("tệp đính kèm: kết quả kiểm tra không rõ (%d)", insp.kind)
	})
	if err != nil {
		return domain.StoredFile{}, bocNhiemVu(ctx, "hoàn tất tệp đính kèm", err)
	}
	switch {
	case stored.ID != "":
		return stored, nil
	case insp.kind == outcomeRejected:
		return domain.StoredFile{}, &AttachmentRejection{Reason: insp.reason}
	default: // outcomeExpired
		return domain.StoredFile{}, ErrUploadExpired
	}
}

// ownUpload: the row exists, is a task attachment of THIS task, and was issued to THIS officer.
func ownUpload(f *domain.StoredFile, taskID, officer string) bool {
	return f != nil && officer != "" && f.SubjectType == domain.StoredFileSubjectTask &&
		f.SubjectID == taskID && f.UploadedBy == officer &&
		f.Purpose == string(storage.PurposeTaskAttachment)
}

func writeAttachmentAudit(ctx context.Context, tx *store.ScopedTx, actor audit.Actor, subject, action string,
	at time.Time, d map[string]any) error {
	delta, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("tệp đính kèm: mã hoá delta: %w", err)
	}
	return audit.Write(ctx, tx, audit.Entry{Actor: actor, Action: action, Subject: subject, At: at, Delta: delta})
}

// storedFileCounter is the one read the lock-free inspection makes: the per-subject count.
type storedFileCounter interface {
	CountForSubject(ctx context.Context, subjectType, subjectID, purpose string,
		pendingSince time.Time) (int, error)
}

// uploadInspector is the lock-free half of a completion that PROMOTES the uploaded bytes unchanged —
// the task attachment and the petition log attachment (petition_log_attachment.go). Both are records:
// sniffed, scanned, hashed and copied as uploaded, never re-encoded.
//
// ONE IMPLEMENTATION FOR BOTH, keyed on the ROW: the subject and the purpose the count is taken over
// are the stored_file row's own (frozen at issue time by migration 0021's guard), so a petition file can
// never be counted against a task's limit, nor the other way round.
type uploadInspector struct {
	objects ObjectStore
	scanner MalwareScanner
	files   storedFileCounter
	clock   func() time.Time
}

func (uc *TaskAttachments) inspector() uploadInspector {
	return uploadInspector{objects: uc.objects, scanner: uc.scanner, files: uc.files, clock: uc.clock}
}

// inspect is the lock-free half of Complete. It returns an outcome, or an error when nothing may be
// decided yet (scanner down, object replaced mid-inspection, store failure) — in which case nothing is
// written and the row stays `pending`, retryable.
func (uc uploadInspector) inspect(ctx context.Context, f domain.StoredFile, key storage.Key,
	pol uploadpolicy.Policy) (inspection, error) {

	uploadKey, err := key.UploadPath()
	if err != nil {
		return inspection{}, fmt.Errorf("tệp đính kèm: dựng khoá tải lên: %w", err)
	}
	st, err := uc.objects.Stat(ctx, storage.BucketTemp, uploadKey)
	if errors.Is(err, storage.ErrNotFound) {
		return uc.fromDestination(ctx, f, key)
	}
	if err != nil {
		return inspection{}, storageErr("đọc thông tin tệp tạm", err)
	}

	// The CURRENT policy decides (platform.proto (c)): a limit tightened since the request rejects here.
	if st.Size <= 0 || st.Size > pol.MaxBytes {
		return uc.reject(ctx, uploadKey, RejectTooLarge, ""), nil
	}
	head, err := uc.objects.ReadHead(ctx, storage.BucketTemp, uploadKey, st.ETag, storage.SniffBytes)
	if err != nil {
		return inspection{}, storageErr("đọc đầu tệp", err)
	}
	mime, ext, ok := storage.SniffMIME(head)
	if !ok || !pol.AllowsMIME(mime) {
		return uc.reject(ctx, uploadKey, RejectTypeNotAllowed, ""), nil
	}
	if ext != key.Ext {
		// The key — frozen in stored_file at issue time — carries the DECLARED type's extension, and the
		// object is stored under exactly that key. Bytes of another type cannot be stored under it.
		return uc.reject(ctx, uploadKey, RejectTypeMismatch, ""), nil
	}
	if pol.FileCountLimited {
		// BEFORE the copy: once promoted, a records object can never be purged again.
		have, err := uc.files.CountForSubject(ctx, f.SubjectType, f.SubjectID, f.Purpose, time.Time{})
		if err != nil {
			return inspection{}, err
		}
		if have >= pol.MaxFilesPerSubject {
			return uc.reject(ctx, uploadKey, RejectCountReached, ""), nil
		}
	}

	rc, size, err := uc.objects.Open(ctx, storage.BucketTemp, uploadKey, st.ETag)
	if err != nil {
		return inspection{}, storageErr("mở tệp để quét", err)
	}
	res, err := uc.scanner.Scan(ctx, rc, size)
	closeErr := rc.Close()
	if err != nil {
		if errors.Is(err, malwarescan.ErrNotConfigured) {
			return inspection{}, fmt.Errorf("%w: %w", ErrUploadNotConfigured, err)
		}
		if errors.Is(err, storage.ErrChanged) {
			return inspection{}, fmt.Errorf("%w: %w", ErrUploadChanged, err)
		}
		return inspection{}, fmt.Errorf("%w: %w", ErrScanUnavailable, err)
	}
	if closeErr != nil {
		return inspection{}, storageErr("đóng tệp sau khi quét", closeErr)
	}
	if !res.Clean {
		return uc.reject(ctx, uploadKey, RejectMalware, res.Signature), nil
	}

	sum, err := uc.objects.SHA256(ctx, storage.BucketTemp, uploadKey, st.ETag)
	if err != nil {
		return inspection{}, storageErr("tính sha256", err)
	}
	pr, err := uc.objects.Promote(ctx, uploadKey, st.ETag, key, storage.BucketPrivate)
	switch {
	case err == nil, errors.Is(err, storage.ErrTempCleanup):
		// ErrTempCleanup: promoted, the leftover in temp expires with the bucket's 1-day lifecycle.
	case errors.Is(err, storage.ErrExists), errors.Is(err, storage.ErrNotFound):
		// Another completion promoted it first (and removed the temp copy): measure the destination.
		return uc.fromDestination(ctx, f, key)
	default:
		return inspection{}, storageErr("chép tệp sang kho lưu", err)
	}
	return inspection{kind: outcomeStored, facts: domain.StoredFileFacts{
		MIMEType: pr.ContentType, SizeBytes: st.Size, SHA256: sum,
	}}, nil
}

// fromDestination handles "no object in temp": either a previous completion promoted it (the copy
// happened, the transaction did not — measure the destination and record it), or nothing arrived.
//
// A destination object is TRUSTED AS SCANNED because nothing else writes there: Promote is the only
// path into `records/…/petitions/task-attachment/…` and `records/…/petitions/petition-log-attachment/…`,
// it runs only after a clean scan, and IAM scopes this service's key to its own subtree (ADR 0052 §3).
func (uc uploadInspector) fromDestination(ctx context.Context, f domain.StoredFile,
	key storage.Key) (inspection, error) {

	st, err := uc.objects.Stat(ctx, storage.BucketPrivate, f.ObjectKey)
	if errors.Is(err, storage.ErrNotFound) {
		if uc.clock().After(f.CreatedAt.Add(storage.UploadTTL)) {
			return inspection{kind: outcomeExpired}, nil
		}
		return inspection{kind: outcomeNotReceived}, nil
	}
	if err != nil {
		return inspection{}, storageErr("đọc thông tin tệp đã lưu", err)
	}
	head, err := uc.objects.ReadHead(ctx, storage.BucketPrivate, f.ObjectKey, st.ETag, storage.SniffBytes)
	if err != nil {
		return inspection{}, storageErr("đọc đầu tệp đã lưu", err)
	}
	mime, ext, ok := storage.SniffMIME(head)
	if !ok || ext != key.Ext {
		// Promote refuses exactly this, so it cannot be ours. Not recorded as stored; an operator looks.
		return inspection{}, errors.New("tệp đính kèm: đối tượng ở kho lưu không khớp kiểu của khoá")
	}
	sum, err := uc.objects.SHA256(ctx, storage.BucketPrivate, f.ObjectKey, st.ETag)
	if err != nil {
		return inspection{}, storageErr("tính sha256 tệp đã lưu", err)
	}
	return inspection{kind: outcomeStored, recovered: true,
		facts: domain.StoredFileFacts{MIMEType: mime, SizeBytes: st.Size, SHA256: sum}}, nil
}

// reject deletes the temp object (ADR 0052 §1c "xoá đối tượng temp") and reports the outcome. A delete
// that fails is RECORDED, not hidden: the trail says `da_xoa_tep_tam: false`, and the temp bucket's
// lifecycle removes the object within a day. The rejection itself is not held back by it.
func (uc uploadInspector) reject(ctx context.Context, uploadKey, reason, signature string) inspection {
	removed := uc.objects.PurgeAllVersions(ctx, storage.BucketTemp, uploadKey) == nil
	return inspection{kind: outcomeRejected, reason: reason, signature: signature, tempRemoved: removed}
}

// storageErr keeps the sentinels the handler maps (not configured, changed) and wraps the rest.
func storageErr(what string, err error) error {
	switch {
	case errors.Is(err, storage.ErrNotConfigured):
		return fmt.Errorf("%w: %w", ErrUploadNotConfigured, err)
	case errors.Is(err, storage.ErrChanged):
		return fmt.Errorf("%w: %w", ErrUploadChanged, err)
	}
	return fmt.Errorf("tệp đính kèm: %s: %w", what, err)
}

// --- download ---------------------------------------------------------------------------------------

// AttachmentDownload is a presigned GET — a bearer credential until ExpiresAt; never logged.
type AttachmentDownload struct {
	URL       storage.PresignedURL
	ExpiresAt time.Time
}

// DownloadLink hands one reader of the task a short-lived link to one of its files
// (domain.MayDownload decides which). The link is signed against the public endpoint with the
// original name in a sanitised Content-Disposition, `attachment` for PDF (core/storage).
//
// NOT AUDITED, stated rather than assumed: rule 6 invariant 7 audits reading CITIZEN personal data in
// full and reading across communes. This is a member of staff, inside their own commune, reading a file
// on a task they may already read — the same footing as GET /api/v1/tasks/{ma}, which is not audited
// either. If a commune treats these files as personal data, that is the decision to revisit.
func (uc *TaskAttachments) DownloadLink(ctx context.Context, ma, id string, reader audit.Actor) (
	AttachmentDownload, error) {

	if uc.objects == nil {
		return AttachmentDownload{}, ErrUploadNotConfigured
	}
	n, err := uc.tasks.LiveByCode(ctx, ma)
	if err != nil {
		return AttachmentDownload{}, bocNhiemVu(ctx, "tải tệp đính kèm", err)
	}
	f, err := uc.files.ByID(ctx, id)
	if err != nil {
		return AttachmentDownload{}, bocNhiemVu(ctx, "tải tệp đính kèm", err)
	}
	if f == nil || f.Bucket != domain.StoredFileBucketPrivate {
		return AttachmentDownload{}, ErrAttachmentNotFound
	}
	linked, err := uc.files.LinkedLogEntry(ctx, id)
	if err != nil {
		return AttachmentDownload{}, bocNhiemVu(ctx, "tải tệp đính kèm", err)
	}
	if !domain.MayDownload(*f, n.ID, linked, reader.ID) {
		return AttachmentDownload{}, ErrAttachmentNotFound
	}
	now := uc.clock()
	u, err := uc.objects.PresignDownload(ctx, storage.BucketPrivate, f.ObjectKey, storage.MaxDownloadTTL,
		f.OriginalName)
	if err != nil {
		return AttachmentDownload{}, storageErr("ký liên kết tải về", err)
	}
	return AttachmentDownload{URL: u, ExpiresAt: now.Add(storage.MaxDownloadTTL)}, nil
}

// --- remove (`Gỡ`, user decision 07/10/2026) ---------------------------------------------------------

// Remove soft-deletes one file of the task: it disappears from the timeline, the drafts and the
// download, and the record keeps who removed it, when and why. Route permission: `task.read`; the real
// condition is domain.AttachmentRemovalRightFor — the uploader, or a holder of `task.update`, on a file
// the caller can see. A file already on a log entry MAY be removed (the owner's decision); the link row
// stays, because task_log_attachment is append-only and "this entry was written with this file" stays
// true.
//
// ONE TRANSACTION, FOUR STATEMENTS AND TWO LOCKS: the task row FOR UPDATE (the same order Complete and
// AddLogEntry take, task then file, so the three cannot deadlock on each other), the file row FOR
// UPDATE, the soft delete, the timeline line, the audit entry. A refusal writes nothing.
//
// THE OBJECT IS NOT TOUCHED. A records file is disposed of by the records schedule, never by a command
// (ADR 0052 §6, §7; rule 7). Removing is hiding.
//
// A SECOND REMOVAL IS A 404 (ErrAttachmentNotFound), the convention DELETE /api/v1/tasks/{ma} follows:
// the locked read sees only live rows, so the repeat finds nothing and cannot overwrite who removed the
// file or why.
//
// THE TRAIL CARRIES the file id, the entry it was on, the door the caller came through and the reason's
// LENGTH — never the reason's text nor the file name (rule 6, forbidden #4; rule 3). Both live in the
// business rows written in this transaction.
func (uc *TaskAttachments) Remove(ctx context.Context, ma, id, reasonRaw string, actor audit.Actor,
	update TaskUpdateRight) error {

	reason, err := domain.CheckAttachmentRemovalReason(reasonRaw)
	if err != nil {
		return err
	}
	if err := coCanBoThucHien(actor); err != nil {
		return err
	}
	now := uc.clock()

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		n, err := uc.tasks.TheoMaDeSua(ctx, tx, ma)
		if err != nil {
			return err
		}
		f, err := uc.files.ForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if f == nil || f.Purpose != string(storage.PurposeTaskAttachment) {
			return ErrAttachmentNotFound
		}
		linked, err := uc.files.LinkedLogEntryTx(ctx, tx, id)
		if err != nil {
			return err
		}
		right := domain.AttachmentRemovalRightFor(*f, n.ID, linked, actor.ID, bool(update))
		switch right {
		case domain.AttachmentRemovalNone:
			return ErrAttachmentNotFound
		case domain.AttachmentRemovalRefused:
			return domain.ErrAttachmentRemovalNotAllowed
		}
		if f.LegalHold {
			return domain.ErrAttachmentUnderLegalHold
		}

		// `deleted_by` IS THE STAFF BUSINESS CODE (rule 6, invariant 8) — coCanBoThucHien refused an
		// empty one above; there is no fallback.
		if err := uc.files.SoftDelete(ctx, tx, id, actor.ID, reason, now); err != nil {
			return err
		}

		logID, err := uc.newLogID()
		if err != nil {
			return fmt.Errorf("nhat_ky_nhiem_vu: sinh mã nội bộ: %w", err)
		}
		// THE SAME ROW SHAPE ghiNhatKy WRITES FOR EVERY OTHER ACT: the status the task stands in, and the
		// holder copied from the task.
		// NO FILE NAME in the line (domain.AttachmentRemovalLogText says why); the audit entry names the id.
		text, err := domain.KiemNoiDungNhatKy(domain.AttachmentRemovalLogText(reason))
		if err != nil {
			return err
		}
		if err := uc.tasks.GhiNhatKy(ctx, tx, domain.NhatKyNhiemVu{
			ID: logID, NhiemVuID: n.ID, NguoiMa: actor.ID, ThoiDiem: now,
			TrangThaiTaiThoiDiem: n.TrangThai, BoPhanID: n.BoPhanID, NguoiPhuTrachMa: n.NguoiThucHienMa,
			NoiDung: text,
		}); err != nil {
			return err
		}

		return writeAttachmentAudit(ctx, tx, actor, n.Ma, ActionTaskAttachmentRemoved, now, map[string]any{
			"tep_id":       id,
			"nhat_ky_id":   logID,
			"gan_nhat_ky":  linked, // "" when the file was a draft on no entry
			"quyen_go":     attachmentRemovalRightCode(right),
			"do_dai_ly_do": utf8.RuneCountInString(reason),
			"truoc":        map[string]any{"trang_thai": string(f.Status), "da_go": false},
			"sau":          map[string]any{"trang_thai": string(f.Status), "da_go": true},
		})
	})
	if err != nil {
		return bocNhiemVu(ctx, "gỡ tệp đính kèm", err)
	}
	return nil
}

// attachmentRemovalRightCode names the door in the trail. Values, so Vietnamese without diacritics
// (ADR 0011).
func attachmentRemovalRightCode(r domain.AttachmentRemovalRight) string {
	if r == domain.AttachmentRemovalUploader {
		return "nguoi-tai-len"
	}
	return "cap-nhat-nhiem-vu"
}
