package app

// SenderPhoneBackfill fills `sender_phone_normalized` (migration 0046, ADR 0102 #3) on petitions booked
// before the intake wrote it — the "resumable, per-commune Go backfill" 0046's header promises.
//
// WHICH PATTERN, AND WHY: core/migrate has no data-backfill mechanism (its header says so: "THAT MECHANISM
// DOES NOT EXIST YET"), and no operator command exists for this service. The established shape for
// per-commune background work here is the automation runner's and the staff-notice relay's: a goroutine
// started by cmd/server, ONE advisory lock in this service's own database so one replica works, the
// commune list from store/crosstenant (identifiers only), and each commune processed IN ITS OWN CONTEXT
// through the scoped store. This job reuses exactly those three pieces.
//
// RULE 7, INVARIANT 5 — per commune, resumable, progress recorded:
//
//	per commune  one commune at a time, in that commune's context; a failure in one does not stop others.
//	resumable    the selector is `sender_phone_normalized IS NULL`: what a run finished is never read
//	             again, and a run stopped half-way resumes at the next start with nothing re-done.
//	progress     the column itself, and one audit entry per row keyed, in the SAME transaction as the
//	             write (rule 6, invariants 3 and 6 — a system action, by the system actor). A separate
//	             progress table would need a migration; TASK-04 ships none (stated in the report).
//
// A NUMBER THAT DOES NOT NORMALISE stays NULL and is skipped by the id cursor, so one run ends. It is read
// once more at each process start — bounded by the count of such rows, which the log line states.
//
// NEVER LOGGED: a number, a lookup code. Counts and the commune id only (rule 3).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/privacy"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

const (
	// senderPhoneBackfillLock names the job's advisory lock (crosstenant.LockKey derives the key).
	senderPhoneBackfillLock = "sender_phone_backfill"
	// SenderPhoneBackfillBatch rows per transaction: small enough that a batch's row locks are held for
	// milliseconds beside live intake, large enough that a commune of thousands is a few dozen commits.
	SenderPhoneBackfillBatch = 200
	// senderPhoneBackfillRetryMax caps the wait between attempts after a failed pass.
	senderPhoneBackfillRetryMax = 10 * time.Minute

	// AuditActionSenderPhoneKeyed is the trail's verb for one row keyed by the backfill.
	AuditActionSenderPhoneKeyed = "sender_phone_match_key_backfilled"
)

// BackfillCommunes lists the communes holding petitions. *crosstenant.Automation.
type BackfillCommunes interface {
	CommunesWithRecords(ctx context.Context) ([]tenant.ID, error)
}

// SenderPhoneBackfillStore is the job's half of the register, in the commune of ctx. *petstore.PhieuPhanAnhStore.
type SenderPhoneBackfillStore interface {
	SenderPhoneBackfillBatch(ctx context.Context, afterID string, limit int) ([]petstore.SenderPhoneBackfillRow, error)
	SetSenderPhoneMatchKey(ctx context.Context, tx *store.ScopedTx, id, rawPhone, key string) (bool, error)
}

// SenderPhoneBackfill is the job. Build with NewSenderPhoneBackfill; Run returns when every commune is done.
type SenderPhoneBackfill struct {
	db       *store.DB
	rows     SenderPhoneBackfillStore
	communes BackfillCommunes
	locks    JobLocker
	log      *slog.Logger
	batch    int
	retry    time.Duration
}

// NewSenderPhoneBackfill builds the job. Every dependency but log is required.
func NewSenderPhoneBackfill(db *store.DB, rows SenderPhoneBackfillStore, communes BackfillCommunes,
	locks JobLocker, log *slog.Logger) (*SenderPhoneBackfill, error) {
	if db == nil || rows == nil || communes == nil || locks == nil {
		return nil, errors.New("điền khoá đối chiếu số người gửi: thiếu phụ thuộc")
	}
	if log == nil {
		log = slog.Default()
	}
	return &SenderPhoneBackfill{db: db, rows: rows, communes: communes, locks: locks, log: log,
		batch: SenderPhoneBackfillBatch, retry: 30 * time.Second}, nil
}

// SenderPhoneBackfillCounts is what one commune's pass did — counts only.
type SenderPhoneBackfillCounts struct {
	Keyed          int // rows whose key this pass wrote (one audit entry each)
	Unnormalisable int // typed numbers core's normaliser refuses — left NULL, never guessed
	Skipped        int // rows keyed or changed meanwhile by another writer — nothing written
}

// Run passes over every commune until one pass completes without error, then returns. A replica that does
// not win the lock returns at once: the winner does the work, and a later start resumes if it died.
func (b *SenderPhoneBackfill) Run(ctx context.Context) {
	wait := b.retry
	for {
		done, err := b.Pass(ctx)
		if done || ctx.Err() != nil {
			return
		}
		b.log.WarnContext(ctx, "CẢNH BÁO: lượt điền khoá đối chiếu số người gửi chưa xong, sẽ chạy lại",
			"service", "petitions", "cho", wait.String(), "err", err)
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
		}
		wait = min(wait*2, senderPhoneBackfillRetryMax)
	}
}

// Pass does one pass. done is true when there is nothing left to retry: every commune finished, or
// another replica holds the lock.
func (b *SenderPhoneBackfill) Pass(ctx context.Context) (done bool, err error) {
	held, release, err := b.locks.TryLockJobs(ctx, []string{senderPhoneBackfillLock})
	if err != nil {
		return false, fmt.Errorf("điền khoá đối chiếu: khoá việc: %w", err)
	}
	defer release()
	if !held[senderPhoneBackfillLock] {
		return true, nil
	}
	communes, err := b.communes.CommunesWithRecords(ctx)
	if err != nil {
		return false, fmt.Errorf("điền khoá đối chiếu: liệt kê xã: %w", err)
	}
	var failed int
	for _, id := range communes {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		n, err := b.Commune(tenant.Into(ctx, id))
		if err != nil {
			failed++
			b.log.WarnContext(ctx, "CẢNH BÁO: điền khoá đối chiếu số người gửi dừng ở một xã",
				"service", "petitions", "xa", string(id), "da_dien", n.Keyed, "err", err)
			continue
		}
		if n.Keyed+n.Unnormalisable+n.Skipped > 0 {
			b.log.InfoContext(ctx, "điền khoá đối chiếu số người gửi: xong một xã", "service", "petitions",
				"xa", string(id), "da_dien", n.Keyed, "khong_chuan_hoa_duoc", n.Unnormalisable, "bo_qua", n.Skipped)
		}
	}
	if failed > 0 {
		return false, fmt.Errorf("điền khoá đối chiếu: %d xã chưa xong", failed)
	}
	return true, nil
}

// Commune keys the rows of the commune in ctx, batch by batch, each batch one transaction with its audit
// entries. A failure leaves the committed batches in place; the next run starts after them.
func (b *SenderPhoneBackfill) Commune(ctx context.Context) (n SenderPhoneBackfillCounts, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("điền khoá đối chiếu: panic: %v", p)
		}
	}()
	after := ""
	for {
		if ctx.Err() != nil {
			return n, ctx.Err()
		}
		rows, err := b.rows.SenderPhoneBackfillBatch(ctx, after, b.batch)
		if err != nil {
			return n, err
		}
		if len(rows) == 0 {
			return n, nil
		}
		if err := b.batchTx(ctx, rows, &n); err != nil {
			return n, err
		}
		if len(rows) < b.batch {
			return n, nil
		}
		after = rows[len(rows)-1].ID
	}
}

// batchTx writes one batch's keys and their audit entries in ONE transaction (rule 6, invariant 3).
func (b *SenderPhoneBackfill) batchTx(ctx context.Context, rows []petstore.SenderPhoneBackfillRow,
	n *SenderPhoneBackfillCounts) error {
	var batch SenderPhoneBackfillCounts
	err := b.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		batch = SenderPhoneBackfillCounts{}
		for _, r := range rows {
			key, ok := petstore.SenderPhoneMatchKey(r.RawPhone)
			if !ok {
				batch.Unnormalisable++
				continue
			}
			written, err := b.rows.SetSenderPhoneMatchKey(ctx, tx, r.ID, r.RawPhone, key)
			if err != nil {
				return err
			}
			if !written {
				batch.Skipped++
				continue
			}
			delta, err := senderPhoneKeyDelta(key)
			if err != nil {
				return err
			}
			if err := audit.Write(ctx, tx, audit.Entry{
				Actor:   audit.Actor{ID: audit.SystemActor, Kind: "system"},
				Action:  AuditActionSenderPhoneKeyed,
				Subject: r.LookupCode,
				Delta:   delta,
			}); err != nil {
				return fmt.Errorf("điền khoá đối chiếu: ghi vết: %w", err)
			}
			batch.Keyed++
		}
		return nil
	})
	if err != nil {
		return err
	}
	n.Keyed += batch.Keyed
	n.Unnormalisable += batch.Unnormalisable
	n.Skipped += batch.Skipped
	return nil
}

// senderPhoneKeyDelta is the entry's before/after (rule 6, invariant 5) — the AFTER value MASKED (rule 6,
// forbidden #4): the key is the citizen's number in another spelling.
func senderPhoneKeyDelta(key string) (json.RawMessage, error) {
	d, err := json.Marshal(map[string]any{
		"truong": "sender_phone_normalized",
		"truoc":  nil,
		"sau":    privacy.MaskPhone(key),
		"nguon":  "nguoi_gui_dien_thoai",
	})
	if err != nil {
		return nil, fmt.Errorf("điền khoá đối chiếu: dựng delta: %w", err)
	}
	return d, nil
}
