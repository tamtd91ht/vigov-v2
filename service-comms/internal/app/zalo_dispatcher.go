package app

// THE ZALO SENDER — sends the owed `zalo_delivery` rows (migration 0018, ADR 0074). The bell's Deliver
// queued them in its own transaction (staff_notification.go); this job sends them.
//
// THE SHAPE IS THE BODY-IMAGE SWEEP'S (body_image_sweep.go): an in-process ticker, ONE advisory lock per
// tick so one replica sends, the communes with work listed as identifiers by internal/store/crosstenant,
// each commune IN ITS OWN CONTEXT behind its own recover, an inactive (merged) commune left untouched.
//
// PER COMMUNE, THREE STEPS, AND THE NETWORK IS NEVER INSIDE A TRANSACTION:
//
//  1. one transaction: claim the due rows (SKIP LOCKED); decide each, in ADR 0074's order —
//     kenh-tat → loai-tat → bot-chua-cau-hinh → chua-lien-ket → quiet hours (POSTPONE to their end, never
//     skip) → the commune's due-soon lead (ngoai-nhac-truoc, 0024) — and lease the rest (attempts + 1,
//     link chosen); one audit entry
//  2. the sends, outside any transaction
//  3. one transaction: each outcome — da-gui; a retryable class (429 / 408 / 5xx / network) owed again
//     after a back-off; anything else, or the last attempt, that-bai with its class; one audit entry
//
// A pass that dies between 1 and 3 leaves its rows leased; the lease expires and they are sent again — at
// least once, never silently lost. A Zalo failure touches only zalo_delivery: the bell is another table,
// already committed.
//
// THE OVERDUE CADENCE IS NOT APPLIED. 0018 stores "start after N days, repeat every M days", but a bell
// notice carries no deadline and no "days late" (comms.proto StaffNotification), so nothing here can
// compute it; each overdue notice the producing job delivers is sent once, as delivered. Applying the
// cadence needs the deadline on the contract — a contract-designer question, reported, not guessed.

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
	docstore "github.com/vihat/vigov/service-comms/internal/store"
	"github.com/vihat/vigov/service-comms/internal/zalobot"
)

const (
	// ZaloDispatchTickInterval — a VENDOR choice. A reminder is minutes-sensitive at most; a tick with
	// nothing due is one identifiers-only query.
	ZaloDispatchTickInterval = 30 * time.Second
	// zaloDispatchTickBudget bounds one tick and so the scheduler lock's hold.
	zaloDispatchTickBudget = 5 * time.Minute
	// zaloSendLease is how long a claimed row is held while its send is in flight: longer than one call
	// (zalobot.DefaultTimeout) times a batch is not needed — the lease only has to outlive THIS pass.
	zaloSendLease = 10 * time.Minute
)

// ZaloDispatchLocks is *crosstenant.ZaloBot's dispatcher half.
type ZaloDispatchLocks interface {
	CommunesWithDueZaloDeliveries(ctx context.Context, now time.Time) ([]tenant.ID, error)
	TryLockScheduler(ctx context.Context) (func(), bool, error)
}

// ZaloDispatcherDeps wires the sender. Registry is REQUIRED: it tells a merged commune (left untouched)
// from an active one, and gives the commune's host for the link in a message.
type ZaloDispatcherDeps struct {
	DB       *store.DB
	Repo     *docstore.ZaloLinkStore
	Locks    ZaloDispatchLocks
	Registry PortalCommuneRegistry
	// Bots answers, per commune, which bot sends: the commune's own live bot (0022, ADR 0079 Q1 #4),
	// else the shared bot.
	Bots ZaloBotSource
	Send ZaloMessenger
	Log  *slog.Logger
}

// ZaloDispatcher sends owed Zalo messages. Run blocks until ctx is done.
type ZaloDispatcher struct {
	d          ZaloDispatcherDeps
	interval   time.Duration
	tickBudget time.Duration
	now        func() time.Time
}

// NewZaloDispatcher builds the sender.
func NewZaloDispatcher(d ZaloDispatcherDeps) (*ZaloDispatcher, error) {
	if d.DB == nil || d.Repo == nil || d.Locks == nil || d.Registry == nil || d.Bots == nil || d.Send == nil {
		return nil, errors.New("gui_zalo: việc nền thiếu phụ thuộc")
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &ZaloDispatcher{d: d, interval: ZaloDispatchTickInterval, tickBudget: zaloDispatchTickBudget,
		now: func() time.Time { return time.Now().UTC() }}, nil
}

// Run ticks until ctx is cancelled — once at start, then every interval.
func (z *ZaloDispatcher) Run(ctx context.Context) {
	z.Tick(ctx)
	t := time.NewTicker(z.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			z.Tick(ctx)
		}
	}
}

// Tick does one pass over every commune with owed messages.
func (z *ZaloDispatcher) Tick(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	release, ok, err := z.d.Locks.TryLockScheduler(ctx)
	if err != nil {
		z.d.Log.WarnContext(ctx, "CẢNH BÁO: không thử được khoá việc nền gửi Zalo, bỏ nhịp này", "service", "comms", "err", err)
		return
	}
	if !ok {
		return // another replica sends
	}
	defer release()
	tctx, cancel := context.WithTimeout(ctx, z.tickBudget)
	defer cancel()

	ids, err := z.d.Locks.CommunesWithDueZaloDeliveries(tctx, z.now())
	if err != nil {
		z.d.Log.WarnContext(ctx, "CẢNH BÁO: không liệt kê được xã có tin Zalo đến hạn", "service", "comms", "err", err)
		return
	}
	for i, id := range ids {
		if tctx.Err() != nil {
			z.d.Log.WarnContext(ctx, "CẢNH BÁO: nhịp gửi Zalo hết thời gian, các xã còn lại chờ nhịp sau",
				"service", "comms", "con_lai", len(ids)-i)
			return
		}
		z.sendCommune(tctx, id)
	}
}

// zaloJob is one message leased for sending. chatID is personal data: never logged.
type zaloJob struct {
	id, chatID, text string
	attempts         int // AFTER this attempt
}

// sendCommune runs the three steps for one commune. Nothing it does can stop the next commune.
func (z *ZaloDispatcher) sendCommune(ctx context.Context, id tenant.ID) {
	defer func() {
		if p := recover(); p != nil {
			z.d.Log.ErrorContext(ctx, "LỖI: gửi Zalo của một xã bị panic, chuyển sang xã kế", "service", "comms",
				"xa", string(id), "panic", fmt.Sprint(p))
		}
	}()
	cctx := tenant.Into(ctx, id)
	t, ok, err := z.d.Registry.XaTrongNguCanh(cctx)
	if err != nil {
		z.d.Log.WarnContext(cctx, "CẢNH BÁO: không đọc được trạng thái xã, bỏ xã này ở nhịp này", "service", "comms",
			"xa", string(id), "err", err)
		return
	}
	if !ok || !t.Active {
		return // a merged or unknown commune keeps its rows unchanged (rule 7, invariant 6)
	}
	// THE COMMUNE'S BOT, opened once per commune per tick: its own live bot when it has one, else the
	// shared bot (ADR 0079 Q1 #4). An unopenable token (no key in this deployment, a broken seal) is not
	// "not configured": the rows stay owed and the next tick tries again — nothing is skipped on a fault
	// of ours, and an own bot never silently falls back to the shared one.
	bot, token, err := z.d.Bots.ActiveToken(cctx)
	if err != nil {
		z.d.Log.WarnContext(cctx, "CẢNH BÁO: không mở được token Zalo Bot của xã — tin Zalo chờ nhịp sau",
			"service", "comms", "xa", string(id), "err", err)
		return
	}
	defer clear(token)

	now := z.now().Truncate(time.Microsecond)
	var jobs []zaloJob
	err = z.d.DB.For(cctx).Tx(cctx, func(tx *store.ScopedTx) error {
		jobs = nil
		due, err := z.d.Repo.ClaimDueDeliveries(cctx, tx, now, docstore.MaxZaloDeliveryBatch)
		if err != nil || len(due) == 0 {
			return err
		}
		settings, err := z.d.Repo.LockChannelSetting(cctx, tx)
		if err != nil {
			return err
		}
		codes := make([]string, 0, len(due))
		for _, d := range due {
			codes = append(codes, d.StaffCode)
		}
		chats, err := z.d.Repo.LiveChatsOf(cctx, tx, bot.Ref, codes)
		if err != nil {
			return err
		}
		tally := map[string][]string{}
		for _, d := range due {
			outcome, err := z.decide(cctx, tx, d, settings, chats, bot.Configured, t.Host, now, &jobs)
			if err != nil {
				return err
			}
			tally[outcome] = append(tally[outcome], d.ID)
		}
		return writeDispatchAudit(cctx, tx, "xep_lich", tally, now)
	})
	if err != nil {
		z.d.Log.WarnContext(cctx, "CẢNH BÁO: không xếp được tin Zalo của xã — thử lại nhịp sau", "service", "comms",
			"xa", string(id), "err", err)
		return
	}
	if len(jobs) == 0 {
		return
	}

	outcomes := make([]zalobot.Outcome, len(jobs))
	for i, j := range jobs {
		outcomes[i] = z.d.Send.SendMessage(cctx, token, j.chatID, j.text)
	}

	done := z.now().Truncate(time.Microsecond)
	err = z.d.DB.For(cctx).Tx(cctx, func(tx *store.ScopedTx) error {
		tally := map[string][]string{}
		for i, j := range jobs {
			o := outcomes[i]
			class := outcomeValue(o)
			switch {
			case o == zalobot.OutcomeOK:
				err = z.d.Repo.MarkDeliverySent(cctx, tx, j.id, done)
				class = domain.ZaloDeliverySent
			case o.Retryable() && j.attempts < domain.ZaloDeliveryMaxAttempts:
				err = z.d.Repo.RetryDelivery(cctx, tx, j.id, class, done.Add(domain.ZaloDeliveryBackoff(j.attempts)), done)
				class = "thu_lai:" + class
			default:
				err = z.d.Repo.FailDelivery(cctx, tx, j.id, class, done)
				class = domain.ZaloDeliveryFailed + ":" + class
			}
			if errors.Is(err, docstore.ErrZaloRowChanged) {
				continue // retired or settled meanwhile (soft-deleted, or another pass after the lease)
			}
			if err != nil {
				return err
			}
			tally[class] = append(tally[class], j.id)
		}
		return writeDispatchAudit(cctx, tx, "ket_qua_gui", tally, done)
	})
	if err != nil {
		// The sends happened; the record did not land. The leases expire and those rows are sent again.
		z.d.Log.WarnContext(cctx, "CẢNH BÁO: không ghi được kết quả gửi Zalo — các tin sẽ được gửi lại sau hạn giữ",
			"service", "comms", "xa", string(id), "so_tin", len(jobs), "err", err)
	}
}

// decide settles one claimed row in ADR 0074's order and returns the outcome's label for the trail.
func (z *ZaloDispatcher) decide(ctx context.Context, tx *store.ScopedTx, d docstore.DueZaloDelivery,
	s domain.ZaloChannelSetting, chats map[string]docstore.LinkedChat, configured bool, host string,
	now time.Time, jobs *[]zaloJob) (string, error) {

	skip := func(reason string) (string, error) {
		return domain.ZaloDeliverySkipped + ":" + reason, z.d.Repo.SkipDelivery(ctx, tx, d.ID, reason, now)
	}
	test := d.Kind == domain.ZaloKindTest
	switch {
	case !test && (!s.Saved || !s.IsEnabled):
		return skip(domain.ZaloSkipChannelOff)
	case !test && !s.KindEnabled(d.Kind):
		return skip(domain.ZaloSkipKindOff)
	case !configured:
		return skip(domain.ZaloSkipBotNotConfigured)
	}
	chat, linked := chats[d.StaffCode]
	if !linked {
		return skip(domain.ZaloSkipNotLinked)
	}
	if until, quiet := domain.QuietUntil(now, s.QuietStartMinute, s.QuietEndMinute); quiet && s.Saved {
		return "hoan_gio_yen_tinh", z.d.Repo.PostponeDelivery(ctx, tx, d.ID, until, now)
	}
	text := domain.ZaloTestMessageText
	if !test {
		link := domain.AbsoluteStaffLink(host, d.Link)
		text = domain.ZaloReminderText(d.Title, d.Body, link)
		// THE COMMUNE'S ZALO LEAD — a deliberate SECOND threshold that only NARROWS the bell's due-soon set
		// (ADR 0079 lô 5 Q13, owner: "Zalo thu hẹp, chuông giữ cột SLA"; domain.ZaloDueSoonCutoff says why
		// it never computes a deadline). Applied HERE, after the quiet-hours wait, so the cut-off is measured
		// when the message leaves: later can only keep more. No items (an old producer) or no lead set is
		// today's message, unfiltered — never "send nothing".
		if s.DueSoonDays != nil && len(d.DueSoonItems) > 0 && domain.IsDueSoonKind(d.Kind) {
			kept := domain.NarrowDueSoonItems(d.DueSoonItems, domain.ZaloDueSoonCutoff(now, *s.DueSoonDays))
			if len(kept) == 0 {
				return skip(domain.ZaloSkipOutsideDueSoonLead)
			}
			text = domain.ZaloDueSoonText(d.Kind, kept, link)
		}
	}
	if err := z.d.Repo.StartAttempt(ctx, tx, d.ID, chat.LinkID, now.Add(zaloSendLease), now); err != nil {
		return "", err
	}
	*jobs = append(*jobs, zaloJob{id: d.ID, chatID: chat.ChatID, text: text, attempts: d.Attempts + 1})
	return "dang_gui", nil
}

// writeDispatchAudit is one entry per transaction, by the system principal (rule 6, invariant 6): the row
// ids under each outcome. Never a chat_id, a title or a body.
func writeDispatchAudit(ctx context.Context, tx *store.ScopedTx, step string, tally map[string][]string, at time.Time) error {
	if len(tally) == 0 {
		return nil
	}
	delta, err := json.Marshal(map[string]any{"buoc": step, "ket_qua": tally})
	if err != nil {
		return fmt.Errorf("gui_zalo: encode delta: %w", err)
	}
	return audit.Write(ctx, tx, audit.Entry{Actor: systemPrincipal, Action: domain.ActionDispatchZaloReminders,
		Subject: "zalo-delivery/" + at.Format("2006-01-02"), At: at, Delta: delta})
}
