package app

// THE HEADER-BELL INBOX — delivering notices (the gRPC side, ADR 0058 §3) and reading them (the staff
// member's own REST side, docs/ui-ux/08-thong-bao.md §8).
//
// WHY THIS LAYER EXISTS: rule 6, invariant 3 — every write and its audit entry share ONE
// transaction, and core/audit.Write takes a *store.ScopedTx. Opening it is this layer's job.
//
//	Deliver       every notice of one call + ONE audit entry (system principal), all or nothing
//	MarkRead      one notice of the SIGNED-IN person + one entry; already read = no write, no entry
//	MarkAllRead   every unread notice of that person + one entry; nothing unread = no write, no entry
//
// THE ENTRIES NEVER CARRY `title` OR `body` (comms.proto, "ONE TRANSACTION, ONE AUDIT ENTRY"): free
// text another service composed can name a person, and the trail is never deleted. They carry kinds,
// keys, recipient STAFF codes and counts — the codes are what an inspection asks ("who was told"),
// and a staff business code acting in office is not what Decree 13 governs (same argument as
// tomTatThongBaoNoiBo).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-comms/internal/domain"
	docstore "github.com/vihat/vigov/service-comms/internal/store"
)

// StaffNotificationRepo is the store, declared at the point of use. EVERY METHOD TAKES THE
// TRANSACTION. *store.StaffNotificationStore satisfies it.
type StaffNotificationRepo interface {
	AddDelivery(ctx context.Context, tx *store.ScopedTx, n domain.NotificationDelivery, ids []string, at time.Time) (int, error)
	LockOwn(ctx context.Context, tx *store.ScopedTx, id, recipientCode string) (domain.StaffNotification, error)
	MarkRead(ctx context.Context, tx *store.ScopedTx, id, recipientCode string, at time.Time) error
	MarkAllRead(ctx context.Context, tx *store.ScopedTx, recipientCode string, at time.Time) (int, error)
}

// The business verbs of the trail. Vietnamese snake_case values, English constant names (rule 12).
const (
	ActionDeliverStaffNotifications = "gui_thong_bao_chuong"
	ActionReadStaffNotification     = "doc_thong_bao_chuong"
	ActionReadAllStaffNotifications = "doc_het_thong_bao_chuong"
)

// staffNotificationSubject prefixes every entry's subject. A notice has no business code of its own;
// the subject is a locator (the day, the key, or the reader), and the delta carries the rest.
const staffNotificationSubject = "hop-thu-thong-bao/"

// ErrNoActor refuses a write whose trail could name nobody — no fallback (rule 6, invariant 8).
var ErrNoActor = errors.New("staff_notification: thiếu chủ thể")

// StaffNotifications owns the inbox writes.
type StaffNotifications struct {
	db   *store.DB
	repo StaffNotificationRepo

	// Injected so a test can pin both. Production: ulid.Moi and time.Now.
	newID func() (string, error)
	now   func() time.Time

	// zalo is the Zalo Bot outbox (ADR 0074, migration 0018): one zalo_delivery row per notice created,
	// in the SAME transaction. nil = no Zalo channel wired (tests of the bell alone).
	zalo ZaloOutbox
	log  *slog.Logger
}

// ZaloOutbox queues the Zalo copies of the notices one Deliver call created, with the due-soon items each
// notice carried (keyed by idempotency key; ADR 0079 lô 5 Q13) — the Zalo copy is the only place they go.
// *store.ZaloLinkStore satisfies it. Its failure is rolled back to a savepoint INSIDE it and reported, never propagated into
// the bell's transaction (ADR 0074 #1: Zalo is an extra channel; the bell is delivered regardless).
type ZaloOutbox interface {
	EnqueueZaloDeliveries(ctx context.Context, tx *store.ScopedTx, keys []string,
		items map[string][]domain.DueSoonItem, createdAt time.Time) (int, error)
}

func NewStaffNotifications(db *store.DB, repo StaffNotificationRepo) *StaffNotifications {
	return &StaffNotifications{db: db, repo: repo, newID: ulid.Moi, now: time.Now, log: slog.Default()}
}

// WithZaloOutbox wires the Zalo Bot outbox. log reports an enqueue failure (commune and count only).
func (uc *StaffNotifications) WithZaloOutbox(o ZaloOutbox, log *slog.Logger) *StaffNotifications {
	uc.zalo = o
	if log != nil {
		uc.log = log
	}
	return uc
}

// Deliver writes every notice of one call, or nothing.
//
// A refusal of the SHAPE (domain.ErrInvalidDelivery) is returned before any transaction opens. Inside
// the transaction every (key, recipient) pair already present is skipped by the unique key and
// counted as already delivered; a retried run therefore creates nothing and — because nothing was
// written — files no entry either. A failure anywhere, the audit entry included, rolls the whole call
// back, so a half-delivered batch cannot exist.
func (uc *StaffNotifications) Deliver(ctx context.Context, in []domain.NotificationDelivery,
	actor audit.Actor) ([]domain.DeliveryOutcome, error) {

	clean, err := domain.ValidateDeliveries(in)
	if err != nil {
		return nil, err
	}
	if actor.ID == "" {
		return nil, ErrNoActor
	}
	// Microseconds: PostgreSQL's precision. The Zalo outbox finds this call's notices by (key, created_at),
	// and a nanosecond `at` compared with the stored, rounded value would find none of them.
	at := uc.now().UTC().Truncate(time.Microsecond)

	var out []domain.DeliveryOutcome
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		out = make([]domain.DeliveryOutcome, 0, len(clean))
		items := make([]map[string]any, 0, len(clean))
		totalCreated, totalAlready := 0, 0
		for _, n := range clean {
			ids := make([]string, len(n.RecipientCodes))
			for i := range ids {
				id, err := uc.newID()
				if err != nil {
					return fmt.Errorf("staff_notification: sinh id: %w", err)
				}
				ids[i] = id
			}
			created, err := uc.repo.AddDelivery(ctx, tx, n, ids, at)
			if err != nil {
				return err
			}
			already := len(n.RecipientCodes) - created
			totalCreated += created
			totalAlready += already
			out = append(out, domain.DeliveryOutcome{
				IdempotencyKey: n.IdempotencyKey, Created: created, AlreadyDelivered: already,
			})
			item := map[string]any{
				"khoa":          n.IdempotencyKey,
				"loai":          n.Kind,
				"nguoi_nhan_ma": n.RecipientCodes,
				"tao_moi":       created,
				"da_co":         already,
			}
			if len(n.DueSoonItems) > 0 {
				// A count, like the rest of this entry: the codes are the producer's and already name
				// exactly what the bell's body counts.
				item["so_muc_sap_den_han"] = len(n.DueSoonItems)
			}
			items = append(items, item)
		}
		if totalCreated == 0 {
			// A pure retry: nothing was written, so there is nothing to record. Same discipline as
			// app.Sua's no-op — an entry saying "nothing happened" buries the ones that matter.
			return nil
		}
		deltaMap := map[string]any{
			"so_thong_bao": len(clean),
			"tao_moi":      totalCreated,
			"da_co":        totalAlready,
			"muc":          items,
		}
		if uc.zalo != nil {
			keys := make([]string, len(clean))
			dueSoon := map[string][]domain.DueSoonItem{}
			for i, n := range clean {
				keys[i] = n.IdempotencyKey
				if len(n.DueSoonItems) > 0 {
					dueSoon[n.IdempotencyKey] = n.DueSoonItems
				}
			}
			queued, err := uc.zalo.EnqueueZaloDeliveries(ctx, tx, keys, dueSoon, at)
			if err != nil {
				// NOT returned: the bell is delivered without its Zalo copies, and the trail says so. The
				// error names the statement, never a title, a body or a recipient.
				uc.log.WarnContext(ctx, "CẢNH BÁO: không xếp được tin Zalo cho thông báo chuông — chuông vẫn giao đủ",
					"xa", string(tx.TenantID()), "so_thong_bao", len(clean), "err", err)
				deltaMap["zalo_loi_xep_hang"] = true
			} else {
				deltaMap["zalo_xep_hang"] = queued
			}
		}
		delta, err := json.Marshal(deltaMap)
		if err != nil {
			return fmt.Errorf("staff_notification: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionDeliverStaffNotifications,
			Subject: staffNotificationSubject + at.Format("2006-01-02"),
			At:      at,
			Delta:   delta,
		})
	})
	if err != nil {
		return nil, fmt.Errorf("staff_notification: giao thông báo: %w", err)
	}
	return out, nil
}

// MarkRead marks ONE notice of the signed-in person read. The person is actor.ID — the staff code
// from the session (internal/http builds it from authz.Principal.Ma) — and it is BOTH the filter and
// the trail's "who": there is no parameter through which one person could mark another's notice.
//
// Another person's id, another commune's id and no such id are one answer:
// store.ErrStaffNotificationNotFound.
func (uc *StaffNotifications) MarkRead(ctx context.Context, id string, actor audit.Actor) (
	domain.StaffNotification, error) {

	if actor.ID == "" {
		return domain.StaffNotification{}, ErrNoActor
	}
	if id == "" {
		return domain.StaffNotification{}, docstore.ErrStaffNotificationNotFound
	}
	at := uc.now().UTC()

	var res domain.StaffNotification
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		n, err := uc.repo.LockOwn(ctx, tx, id, actor.ID)
		if err != nil {
			return err
		}
		if n.Read() {
			// Idempotent: the first reading stands, and nothing is written or recorded.
			res = n
			return nil
		}
		if err := uc.repo.MarkRead(ctx, tx, n.ID, actor.ID, at); err != nil {
			return err
		}
		n.ReadAt = at
		res = n
		delta, err := json.Marshal(map[string]any{
			"id": n.ID, "loai": n.Kind, "khoa": n.IdempotencyKey, "doc_luc": at,
		})
		if err != nil {
			return fmt.Errorf("staff_notification: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionReadStaffNotification,
			Subject: staffNotificationSubject + n.IdempotencyKey,
			At:      at,
			Delta:   delta,
		})
	})
	if err != nil {
		return domain.StaffNotification{}, fmt.Errorf("staff_notification: đánh dấu đã đọc: %w", err)
	}
	return res, nil
}

// MarkAllRead is the panel's `Đọc hết` for the signed-in person. Returns how many moved.
func (uc *StaffNotifications) MarkAllRead(ctx context.Context, actor audit.Actor) (int, error) {
	if actor.ID == "" {
		return 0, ErrNoActor
	}
	at := uc.now().UTC()

	var moved int
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		n, err := uc.repo.MarkAllRead(ctx, tx, actor.ID, at)
		if err != nil {
			return err
		}
		moved = n
		if n == 0 {
			return nil
		}
		delta, err := json.Marshal(map[string]any{"so_da_doc": n, "doc_luc": at})
		if err != nil {
			return fmt.Errorf("staff_notification: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionReadAllStaffNotifications,
			Subject: staffNotificationSubject + actor.ID,
			At:      at,
			Delta:   delta,
		})
	})
	if err != nil {
		return 0, fmt.Errorf("staff_notification: đọc hết: %w", err)
	}
	return moved, nil
}
