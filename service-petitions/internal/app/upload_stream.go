package app

// THE UPLOAD GOES THROUGH THIS SERVICE (ADR 0052 §Sửa đổi 09/10/2026, owner decision): one multipart
// request per file, received by the handler, streamed by this service into the temp bucket, then the
// existing completion step runs on it in the same request. The four flows that take a file — the
// citizen's scene photo, staff's verification photo, the petition log attachment and the task
// attachment — share the middle step here:
//
//	reserve   the flow's own transaction: permission · owner · commune · window · count under lock ·
//	          policy · the `pending` row + its trail                                  (per flow)
//	receive   PutUpload into the reserved temp key · Finish (nothing followed the file)   (this file)
//	complete  the flow's existing completion: stat · sniff · scan · … · `stored` + trail  (per flow)
//
// WHY A FAILED RECEIVE MOVES THE ROW TO `failed`: a `pending` row counts against the per-subject cap
// for storage.UploadTTL (the old form's lifetime). With no form and no completion route any more, a
// row whose bytes never arrived can never be completed — left `pending`, five dropped connections
// would refuse a citizen's sixth attempt for 15 minutes with "đủ số ảnh". The same holds for a
// completion that could not decide (scanner down): there is no client-side retry of that step now,
// so the row is closed and the client uploads again.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/storage"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// UploadBody is the file of one upload as the handler received it (core/httpx.UploadRequest): Read
// yields exactly the declared size, Finish confirms nothing followed the file. Finish is called after
// the storage write succeeds and BEFORE anything is recorded as stored.
type UploadBody interface {
	io.Reader
	Finish() error
}

// The `ly_do` of the trail entry that closes a row whose upload did not finish — values, so Vietnamese
// without diacritics (ADR 0011).
const (
	// UploadFailedNotReceived: the bytes never fully arrived (client hung up, timed out, sent a broken
	// envelope) or the temp write failed.
	UploadFailedNotReceived = "khong-nhan-du-tep"
	// UploadFailedNotCompleted: the bytes arrived, but the completion could not decide (scanner or store
	// down) — nothing was stored.
	UploadFailedNotCompleted = "khong-hoan-tat-duoc"
)

// uploadCleanupTimeout bounds the bookkeeping after a failed upload. It runs on a context detached
// from the request's (the request may have died — that is often WHY the upload failed), so it needs a
// bound of its own. A vendor bound for one short transaction, not a business number.
const uploadCleanupTimeout = 10 * time.Second

// uploadReservation is what a flow's reserve step hands to receive and complete.
type uploadReservation struct {
	file        domain.StoredFile
	subject     string // the business code of the petition / task — the trail's Subject
	uploadKey   string // the temp key PutUpload writes
	contentType string // the DECLARED type, checked against the policy at reserve
	size        int64  // the DECLARED size, ≤ maxBytes
	maxBytes    int64  // the policy's cap at reserve
}

// uploadWriter is core/storage PutUpload.
type uploadWriter interface {
	PutUpload(ctx context.Context, uploadKey string, r io.Reader, size, maxBytes int64,
		contentType string) (storage.ObjectInfo, error)
	PurgeAllVersions(ctx context.Context, b storage.Bucket, key string) error
}

// pendingFileRows is the two row operations closing a failed upload needs.
type pendingFileRows interface {
	ForUpdate(ctx context.Context, tx *store.ScopedTx, id string) (*domain.StoredFile, error)
	Transition(ctx context.Context, tx *store.ScopedTx, id string, from, to domain.StoredFileStatus,
		at time.Time) error
}

// uploadCloser closes the reserved row of an upload that did not end stored. failAction is the flow's
// "upload did not arrive" verb (its `…het_han_tai`, reused: the row ends exactly as an expired form's
// did — `failed`, nothing stored).
type uploadCloser struct {
	db         *store.DB
	objects    uploadWriter
	files      pendingFileRows
	actor      audit.Actor
	failAction string
	clock      func() time.Time
}

// receive streams body into the reserved temp key and confirms the envelope ended there. On failure
// the reserved row is closed (`failed`, with its trail) and the error returned — wrapped so a handler
// still finds core/httpx's sentinels (408 / 413 / 400) and core/storage's ErrTooLarge with errors.Is.
func (c uploadCloser) receive(ctx context.Context, res uploadReservation, body UploadBody) error {
	if body == nil {
		return c.fail(ctx, res, false, UploadFailedNotReceived,
			fmt.Errorf("tệp tải lên %s: không có thân tệp", res.file.ID))
	}
	_, err := c.objects.PutUpload(ctx, res.uploadKey, body, res.size, res.maxBytes, res.contentType)
	if err != nil {
		// One PUT with DisableMultipart (core/storage): a failed body leaves no object behind.
		return c.fail(ctx, res, false, UploadFailedNotReceived,
			fmt.Errorf("tệp tải lên %s: %w", res.file.ID, storageErr("ghi tệp vào kho tạm", err)))
	}
	if err := body.Finish(); err != nil {
		// The object is written but the envelope was wrong after it (a second file, a late field): the
		// upload is refused, so the temp copy goes too.
		return c.fail(ctx, res, true, UploadFailedNotReceived,
			fmt.Errorf("tệp tải lên %s: phần sau tệp không hợp lệ: %w", res.file.ID, err))
	}
	return nil
}

// afterComplete closes the row when the completion returned an error WITHOUT deciding (the row is still
// `pending`): there is no completion route to retry it from any more. A decided refusal (rejected,
// window closed under the lock) already moved the row, and failIfPending leaves it alone.
func (c uploadCloser) afterComplete(ctx context.Context, res uploadReservation, err error) error {
	if err == nil {
		return nil
	}
	return c.fail(ctx, res, true, UploadFailedNotCompleted, fmt.Errorf("tệp tải lên %s: %w", res.file.ID, err))
}

// fail removes the temp object when one may exist, moves the row `pending` → `failed` with its trail,
// and returns cause — joined with the bookkeeping error if that failed too (the row then stays
// `pending` and stops counting after storage.UploadTTL, the residue a crash leaves as well).
func (c uploadCloser) fail(ctx context.Context, res uploadReservation, written bool, reason string, cause error) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), uploadCleanupTimeout)
	defer cancel()
	tempGone := !written
	if written {
		// Best effort: the temp bucket's lifecycle removes a leftover within a day.
		tempGone = c.objects.PurgeAllVersions(ctx, storage.BucketTemp, res.uploadKey) == nil
	}
	now := time.Now().UTC()
	if c.clock != nil {
		now = c.clock().UTC()
	}
	err := c.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		cur, err := c.files.ForUpdate(ctx, tx, res.file.ID)
		if err != nil {
			return err
		}
		if cur == nil || cur.Status != domain.StoredFilePending {
			return nil // decided already (rejected, stored by a racing path): nothing to close
		}
		if err := c.files.Transition(ctx, tx, res.file.ID, domain.StoredFilePending, domain.StoredFileFailed,
			now); err != nil {
			return err
		}
		return writeAttachmentAudit(ctx, tx, c.actor, res.subject, c.failAction, now, map[string]any{
			"tep_id": res.file.ID, "ly_do": reason, "da_xoa_tep_tam": tempGone,
		})
	})
	if err != nil {
		return errors.Join(cause, fmt.Errorf("tệp tải lên %s: đóng dòng chờ: %w", res.file.ID, err))
	}
	return cause
}
