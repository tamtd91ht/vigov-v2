package app

// THE ABANDONED-DRAFT BODY-IMAGE SWEEP (owner, 03/10/2026, ADR 0067 K11): a READY `content-body-image` file
// whose article id was NEVER SAVED, and that completed more than bodyImageAbandonedAfter ago, is soft-deleted
// by a background job — per commune, by the system principal, with one audit entry per commune per run.
//
// WHY A JOB AT ALL: the save retires the images a body no longer shows (retireUnreferencedBodyImages, K3), but
// only when the article IS saved. An editor closed before the first save leaves its pasted and uploaded
// images in the private store, ready, counted by nothing and retired by nothing, for ever.
//
// WHAT IT NEVER TOUCHES: a file of an id ANY article ever carried — the article soft-deleted or not (rule 7,
// invariant 3: those files belong to a record); a pending / scanning / stored / processing / rejected / failed
// row (only `ready` is a finished upload — the others are another lifecycle, ADR 0052 §5); a row with a public
// key; another commune (every statement is scoped). It NEVER hard-deletes, and the private OBJECT stays: what
// becomes of objects is the storage lifecycle's (ADR 0052 §6), not this job's.
//
// THE SHAPE IS THE PORTAL SYNC RUNNER'S (portal_sync_runner.go, ADR 0058's pattern): an in-process ticker;
// ONE advisory lock per tick, so one replica sweeps; the communes with work listed as identifiers by
// internal/store/crosstenant; each commune IN ITS OWN CONTEXT, sequentially, behind its own recover; an
// inactive (merged) commune skipped — its data stays exactly as it was (rule 7, invariant 6).
//
// BOUNDS: one tick ≤ bodyImageSweepTickBudget (the scheduler lock is held no longer); one commune ≤
// bodyImageSweepBatch files per run — one transaction, one audit entry whose delta lists them, bounded. A
// commune with more is continued on the next tick. Sequential: no commune waits on a second connection.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

const (
	// bodyImageAbandonedAfter — 7 DAYS, THE OWNER'S FIGURE (03/10/2026, ADR 0067 K11). A ready body image of
	// a never-saved article younger than this is left alone: the officer may still be writing. Changing it
	// changes what the authority keeps of a draft — ask the owner, then edit here.
	bodyImageAbandonedAfter = 7 * 24 * time.Hour

	// BodyImageSweepTickInterval is how often the sweep runs — a VENDOR choice, not a customer figure. Hourly:
	// against a 7-day threshold an hour of lateness is noise, and a tick with nothing to do is one
	// identifiers-only query.
	BodyImageSweepTickInterval = time.Hour
	// bodyImageSweepTickBudget bounds one tick and so the scheduler lock's hold.
	bodyImageSweepTickBudget = 10 * time.Minute
	// bodyImageSweepBatch bounds one commune's run: the files locked in one transaction and listed in one
	// audit entry. commsstore.MaxStoredFileBatch, the store's own batch ceiling.
	bodyImageSweepBatch = commsstore.MaxStoredFileBatch
)

// ActionBodyImagesAbandonedRetired is the trail's verb: the system retired a commune's abandoned-draft body
// images (ADR 0011: a Vietnamese snake_case value).
const ActionBodyImagesAbandonedRetired = "go_anh_than_bai_cua_bai_chua_luu"

// bodyImageAbandonedReason is the `delete_reason` of every row the sweep retires.
const bodyImageAbandonedReason = "ảnh thân bài của bài chưa từng được lưu, quá 7 ngày — việc nền gỡ (ADR 0067 K11)"

// AbandonedBodyImageRepo is the part of *store.StoredFileStore the sweep calls.
type AbandonedBodyImageRepo interface {
	LockAbandonedBodyImages(ctx context.Context, tx *store.ScopedTx, purpose string, completedBefore time.Time,
		limit int) ([]string, error)
	RetireAbandonedBodyImages(ctx context.Context, tx *store.ScopedTx, ids []string, purpose string,
		completedBefore time.Time, by, reason string, at time.Time) ([]string, error)
}

// BodyImageSweepLocks is *crosstenant.BodyImageSweep.
type BodyImageSweepLocks interface {
	CommunesWithAbandonedBodyImages(ctx context.Context, purpose, subjectType string,
		completedBefore time.Time) ([]tenant.ID, error)
	TryLockScheduler(ctx context.Context) (func(), bool, error)
}

// BodyImageSweeperDeps wires the sweep. Registry is REQUIRED, unlike the portal runner's: without it the
// sweep could not tell a merged commune, whose data must stay unchanged, from an active one.
type BodyImageSweeperDeps struct {
	DB       *store.DB
	Repo     AbandonedBodyImageRepo
	Locks    BodyImageSweepLocks
	Registry PortalCommuneRegistry
	Log      *slog.Logger
}

// BodyImageSweeper runs the sweep. Build with NewBodyImageSweeper; Run blocks until ctx is done.
type BodyImageSweeper struct {
	db       *store.DB
	repo     AbandonedBodyImageRepo
	locks    BodyImageSweepLocks
	registry PortalCommuneRegistry
	log      *slog.Logger

	interval   time.Duration
	tickBudget time.Duration
	now        func() time.Time
}

// NewBodyImageSweeper builds the sweep.
func NewBodyImageSweeper(d BodyImageSweeperDeps) (*BodyImageSweeper, error) {
	if d.DB == nil || d.Repo == nil || d.Locks == nil || d.Registry == nil {
		return nil, errors.New("go_anh_than_bai: việc nền thiếu phụ thuộc")
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &BodyImageSweeper{db: d.DB, repo: d.Repo, locks: d.Locks, registry: d.Registry, log: d.Log,
		interval: BodyImageSweepTickInterval, tickBudget: bodyImageSweepTickBudget,
		now: func() time.Time { return time.Now().UTC() }}, nil
}

// Run ticks until ctx is cancelled — once at start, then every interval.
func (s *BodyImageSweeper) Run(ctx context.Context) {
	s.Tick(ctx)
	t := time.NewTicker(s.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.Tick(ctx)
		}
	}
}

// Tick does one pass: the scheduler lock, the communes with work, each in turn.
func (s *BodyImageSweeper) Tick(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	release, ok, err := s.locks.TryLockScheduler(ctx)
	if err != nil {
		s.log.WarnContext(ctx, "CẢNH BÁO: không thử được khoá việc nền gỡ ảnh thân bài, bỏ nhịp này", "service", "comms", "err", err)
		return
	}
	if !ok {
		return // another replica sweeps
	}
	defer release()
	tctx, cancel := context.WithTimeout(ctx, s.tickBudget)
	defer cancel()
	// ONE cut-off for the whole tick, so the list and every commune's statements agree on "7 days ago".
	cutoff := s.now().Add(-bodyImageAbandonedAfter)
	ids, err := s.locks.CommunesWithAbandonedBodyImages(tctx, string(bodyImagePurpose),
		domain.StoredFileSubjectContentItem, cutoff)
	if err != nil {
		s.log.WarnContext(ctx, "CẢNH BÁO: không liệt kê được xã có ảnh thân bài bỏ dở", "service", "comms", "err", err)
		return
	}
	for i, id := range ids {
		if tctx.Err() != nil {
			s.log.WarnContext(ctx, "CẢNH BÁO: nhịp gỡ ảnh thân bài bỏ dở hết thời gian, các xã còn lại chờ nhịp sau",
				"service", "comms", "con_lai", len(ids)-i)
			return
		}
		s.sweepCommune(tctx, id, cutoff)
	}
}

// sweepCommune retires one commune's abandoned body images in ONE transaction with ONE audit entry. Nothing
// it does can stop the next commune.
func (s *BodyImageSweeper) sweepCommune(ctx context.Context, id tenant.ID, cutoff time.Time) {
	defer func() {
		if p := recover(); p != nil {
			s.log.ErrorContext(ctx, "LỖI: gỡ ảnh thân bài bỏ dở của một xã bị panic, chuyển sang xã kế", "service", "comms",
				"xa", string(id), "panic", fmt.Sprint(p))
		}
	}()
	cctx := tenant.Into(ctx, id)
	t, ok, err := s.registry.XaTrongNguCanh(cctx)
	if err != nil {
		// FAIL CLOSED: a commune whose state cannot be read is not touched this tick.
		s.log.WarnContext(cctx, "CẢNH BÁO: không đọc được trạng thái xã, bỏ xã này ở nhịp này", "service", "comms",
			"xa", string(id), "err", err)
		return
	}
	if !ok || !t.Active {
		return // a merged or unknown commune keeps its data unchanged (rule 7, invariant 6)
	}
	at := s.now()
	var retired []string
	err = s.db.For(cctx).Tx(cctx, func(tx *store.ScopedTx) error {
		locked, err := s.repo.LockAbandonedBodyImages(cctx, tx, string(bodyImagePurpose), cutoff, bodyImageSweepBatch)
		if err != nil || len(locked) == 0 {
			return err
		}
		retired, err = s.repo.RetireAbandonedBodyImages(cctx, tx, locked, string(bodyImagePurpose), cutoff,
			systemPrincipal.ID, bodyImageAbandonedReason, at)
		if err != nil || len(retired) == 0 {
			return err
		}
		return writeSweepAudit(cctx, tx, retired, cutoff, at)
	})
	if err != nil {
		s.log.WarnContext(cctx, "CẢNH BÁO: không gỡ được ảnh thân bài bỏ dở của xã — thử lại nhịp sau", "service", "comms",
			"xa", string(id), "err", err)
		return
	}
	if len(retired) > 0 {
		// Identifiers and a count only — a file id names no person.
		s.log.InfoContext(cctx, "đã gỡ mềm ảnh thân bài của bài chưa từng được lưu", "service", "comms",
			"xa", string(id), "so_tep", len(retired))
	}
}

// writeSweepAudit is the commune run's ONE entry, in the transaction of the soft delete (rule 6, invariant
// 3), actor the system principal (invariant 6). The delta lists the file ids and the cut-off — no name, no
// URL, no content.
func writeSweepAudit(ctx context.Context, tx *store.ScopedTx, ids []string, cutoff, at time.Time) error {
	delta, err := json.Marshal(map[string]any{
		"tep_ids":        ids,
		"so_tep":         len(ids),
		"muc_dich":       string(bodyImagePurpose),
		"hoan_tat_truoc": cutoff.UTC().Format(time.RFC3339),
		"ly_do":          bodyImageAbandonedReason,
	})
	if err != nil {
		return fmt.Errorf("go_anh_than_bai: mã hoá delta: %w", err)
	}
	// TenantID filled by audit.Write from the tx.
	return audit.Write(ctx, tx, audit.Entry{Actor: systemPrincipal, Action: ActionBodyImagesAbandonedRetired,
		Subject: coverAuditSubject(bodyImageVerbs, at), At: at, Delta: delta})
}
