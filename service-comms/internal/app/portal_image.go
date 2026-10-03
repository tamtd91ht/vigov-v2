package app

// A PORTAL ARTICLE'S IMAGE BECOMES ITS COVER through the cover pipeline (ADR 0067 §2 "Ghi" #5, ADR
// 0052 §9 §10, ADR 0047:256): the `content-image` policy, sniffing, ClamAV, a decode under the same
// pixel budget, the re-encode that strips EXIF, and the derivative that alone is ever published.
//
// WHAT DIFFERS FROM A STAFF UPLOAD, AND WHY — decided here and stated:
//
//   - NO ORIGINAL IS STORED. A staff upload's original arrives through core/storage.Promote (the one
//     way into `content-source/…/original.*`, ADR 0052 Bổ sung 30/09 lần hai), and PutServerProduced
//     refuses a server-written content-source original on purpose. A portal image is not something
//     the commune uploaded: it is fetched, scanned, re-encoded, and ONLY THE RE-ENCODE is stored. The
//     portal keeps its own copy. Changing core/storage to admit a fetched original would be a change to
//     ADR 0052's list of server-produced flows — the owner's, not this file's.
//   - SO THE stored_file ROW NAMES THE OBJECT THAT EXISTS: `object_key` is the `thumb-1280` derivative
//     key, and the measured facts (type, size, sha256) are the derivative's. The sha256 of the bytes
//     the portal served goes in the import's audit delta. coverDerivativeKey / coverPublicKey map that
//     key onto itself and its public twin, so View, settle and the public URLs work unchanged.
//   - `uploaded_by` is the system principal (rule 6, invariant 6) and `original_name` a FIXED name:
//     the portal's own file name can carry a person's name (rule 3), and nobody typed it here.
//   - the download is capped below the policy (portalImageMaxBytes): six images are fetched at once
//     (ADR 0067 §2 "Ghi" #3) and held in memory, and six × the policy's 50 MB would exceed the pod.
//   - an image that fails ANY check is dropped and counted; the article is still imported without it
//     (ADR 0067 §2, "6 ảnh song song; failures counted"). Never stored unscanned.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/storage"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

const (
	// portalImageMaxBytes caps ONE downloaded portal image, under the policy's own limit. A VENDOR
	// BOUND for memory (see the file header), not a customer number: a news photo re-encoded to
	// 1280 px is far below it.
	portalImageMaxBytes = 10 << 20
	// portalCoverName is stored_file.original_name of every imported cover.
	portalCoverName = "anh-dai-dien-cong-ttdt.jpg"
)

// portalCover is a cover made from a portal image: the derivative is in the private bucket; the row
// is written by recordPortalCover inside the import's transaction.
type portalCover struct {
	file         domain.StoredFile
	sourceSHA256 string
}

// portalImageLimit is the download cap for this commune: the smaller of the policy's MaxBytes and
// portalImageMaxBytes. ErrCoverUploadNotConfigured when the pipeline cannot run at all.
func (uc *ContentCovers) portalImageLimit(ctx context.Context) (int64, error) {
	if !uc.uploadsConfigured() {
		return 0, ErrCoverUploadNotConfigured
	}
	pol, err := uc.policy(ctx)
	if err != nil {
		return 0, err
	}
	return min(pol.MaxBytes, portalImageMaxBytes), nil
}

// preparePortalCover runs the pipeline over downloaded bytes and stores the derivative. A refusal of
// the FILE is a *CoverRejection (counted, dropped); anything else is an error (counted, dropped). No
// row is written here.
func (uc *ContentCovers) preparePortalCover(ctx context.Context, itemID string, data []byte,
	now time.Time) (portalCover, error) {

	if !uc.uploadsConfigured() {
		return portalCover{}, ErrCoverUploadNotConfigured
	}
	pol, err := uc.policy(ctx)
	if err != nil {
		return portalCover{}, err
	}
	if len(data) == 0 || int64(len(data)) > min(pol.MaxBytes, portalImageMaxBytes) {
		return portalCover{}, &CoverRejection{Reason: CoverRejectTooLarge}
	}
	head := data[:min(len(data), storage.SniffBytes)]
	mime, _, ok := storage.SniffMIME(head)
	if !ok || !pol.AllowsMIME(mime) {
		return portalCover{}, &CoverRejection{Reason: CoverRejectTypeNotAllowed}
	}

	// UNSCANNABLE IS NEVER CLEAN (ADR 0052 §9): scanner down → the image is dropped, not stored.
	res, err := uc.scanner.Scan(ctx, bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return portalCover{}, fmt.Errorf("%w: %w", ErrCoverScanUnavailable, err)
	}
	if !res.Clean {
		return portalCover{}, &CoverRejection{Reason: CoverRejectMalware}
	}

	jpg, err := uc.renderPortalDerivative(ctx, mime, data)
	if err != nil {
		return portalCover{}, err
	}

	fileID, err := uc.newID()
	if err != nil {
		return portalCover{}, fmt.Errorf("ảnh bìa: sinh mã tệp: %w", err)
	}
	key := storage.Key{
		Class: storage.ClassContentSource, TenantID: string(tenant.MustFrom(ctx)), CreatedAt: now,
		Service: storage.ServiceComms, Purpose: coverPurpose,
		ObjectID: fileID, Variant: CoverDerivativeVariant, Ext: "jpg",
	}
	objectKey, err := key.Path()
	if err != nil {
		return portalCover{}, fmt.Errorf("ảnh bìa: dựng khoá bản dẫn xuất: %w", err)
	}
	prod, err := uc.objects.PutServerProduced(ctx, key, bytes.NewReader(jpg), int64(len(jpg)))
	if err != nil {
		return portalCover{}, coverStorageErr("ghi ảnh bìa nhập từ Cổng", err)
	}
	sum := sha256.Sum256(data)
	return portalCover{
		file: domain.StoredFile{
			ID: fileID, Bucket: domain.StoredFileBucketPrivate, ObjectKey: objectKey,
			RetentionClass: string(storage.ClassContentSource), Purpose: string(coverPurpose),
			SubjectType: domain.StoredFileSubjectContentItem, SubjectID: itemID, OriginalName: portalCoverName,
			MIMEType: prod.ContentType, SizeBytes: prod.Size, SHA256: prod.SHA256,
			Status: domain.StoredFileReady, UploadedBy: audit.SystemActor, CreatedAt: now, UpdatedAt: now,
		},
		sourceSHA256: hex.EncodeToString(sum[:]),
	}, nil
}

// renderPortalDerivative is the cover decode under the SAME one-at-a-time slot and pixel budget as a
// staff upload's (coverDecodeBudget is per decode; two at once would be two budgets against one pod).
func (uc *ContentCovers) renderPortalDerivative(ctx context.Context, mime string, data []byte) ([]byte, error) {
	select {
	case uc.decodeSlot <- struct{}{}:
		defer func() { <-uc.decodeSlot }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	h, err := readCoverHeader(mime, data[:min(len(data), coverHeadBytes)])
	if err != nil {
		return nil, &CoverRejection{Reason: CoverRejectUndecodable}
	}
	if decodeCost(mime, h) > coverDecodeBudget {
		return nil, &CoverRejection{Reason: CoverRejectTooManyPixels}
	}
	img, err := decodeCover(mime, bytes.NewReader(data))
	if err != nil {
		return nil, &CoverRejection{Reason: CoverRejectUndecodable}
	}
	jpg, err := renderCoverDerivative(img, h.orientation)
	if err != nil {
		return nil, &CoverRejection{Reason: CoverRejectUndecodable}
	}
	return jpg, nil
}

// recordPortalCover writes the cover's row INSIDE the import's transaction, walking ADR 0052 §5's
// edges one guarded UPDATE at a time (stored_file_guard sees every step), so it ends `ready` before
// the article that points at it is inserted.
func (uc *ContentCovers) recordPortalCover(ctx context.Context, tx *store.ScopedTx, pc portalCover) error {
	f := pc.file
	f.Status = domain.StoredFilePending
	if err := uc.files.InsertPending(ctx, tx, f); err != nil {
		return err
	}
	at := f.CreatedAt
	if err := uc.files.Transition(ctx, tx, f.ID, domain.StoredFilePending, domain.StoredFileScanning, at); err != nil {
		return err
	}
	if err := uc.files.MarkStored(ctx, tx, f.ID, domain.StoredFileFacts{
		MIMEType: f.MIMEType, SizeBytes: f.SizeBytes, SHA256: f.SHA256}, at); err != nil {
		return err
	}
	if err := uc.files.Transition(ctx, tx, f.ID, domain.StoredFileStored, domain.StoredFileProcessing, at); err != nil {
		return err
	}
	return uc.files.Transition(ctx, tx, f.ID, domain.StoredFileProcessing, domain.StoredFileReady, at)
}

// publishPortalCover copies the derivative of a just-imported, born-published article to the public
// bucket INSIDE its transaction (coverPublisher.settle — the same order as a staff publish). It
// returns whether a copy was made, so a rollback can withdraw it.
func (uc *ContentCovers) publishPortalCover(ctx context.Context, tx *store.ScopedTx, n domain.NoiDungMiniApp,
	at time.Time) (bool, error) {

	p := &coverPublisher{files: uc.files, objects: uc.objects}
	s, err := p.settle(ctx, tx, n, at)
	return len(s.copied) > 0, err
}

// withdrawPortalCover is the compensation of a publish whose transaction did not commit. NOT
// coverPublisher.undoPublish: that one reads the row first and stops when it is gone — and here it is
// gone, because the rollback took it. A failure is returned for the caller to log by key; the copy is
// reachable only through two random ULIDs and the public bucket is not listable (ADR 0052 §2).
func (uc *ContentCovers) withdrawPortalCover(ctx context.Context, f domain.StoredFile) error {
	if uc.objects == nil {
		return ErrCoverUploadNotConfigured
	}
	orig, err := storage.ParseKey(f.ObjectKey)
	if err != nil {
		return fmt.Errorf("ảnh bìa: khoá đối tượng không hợp lệ: %w", err)
	}
	if err := uc.objects.UnpublishDerivative(ctx, coverPublicKey(orig)); err != nil {
		return fmt.Errorf("%w: %w", ErrCoverPublishUnavailable, err)
	}
	return nil
}

// discardPortalCover deletes the PRIVATE derivative of a cover whose import did not commit (R7,
// 02/10/2026) — before this, it stayed in the bucket until a purge worker that does not exist yet.
//
// THE ROW IS ASKED FIRST: an import whose commit returned an error may still have committed (the
// connection dropped after COMMIT reached the server). A stored_file row naming the file means the
// article owns the object, and it is kept (deleted=false, err=nil). An error reading the row is an
// error — "could not check" is not "no row" — and the object is kept for the caller to log by key.
func (uc *ContentCovers) discardPortalCover(ctx context.Context, f domain.StoredFile) (bool, error) {
	if uc.objects == nil || uc.files == nil {
		return false, ErrCoverUploadNotConfigured
	}
	row, err := uc.files.ByID(ctx, f.ID)
	if err != nil {
		return false, fmt.Errorf("ảnh bìa: kiểm dòng tệp trước khi xoá bản dẫn xuất: %w", err)
	}
	if row != nil {
		return false, nil
	}
	if err := uc.objects.PurgeAllVersions(ctx, storage.BucketPrivate, f.ObjectKey); err != nil {
		return false, coverStorageErr("xoá ảnh dẫn xuất của tin không nhập", err)
	}
	return true, nil
}

// portalCoverRejectReason names why an image was dropped, for the run's error summary.
func portalCoverRejectReason(err error) string {
	var rej *CoverRejection
	switch {
	case errors.As(err, &rej):
		return "image-" + rej.Reason
	case errors.Is(err, ErrCoverScanUnavailable):
		return "image-scan-unavailable"
	case errors.Is(err, ErrCoverUploadNotConfigured), errors.Is(err, ErrCoverLimitsUnavailable):
		return "image-storage-unavailable"
	default:
		return "image-failed"
	}
}
