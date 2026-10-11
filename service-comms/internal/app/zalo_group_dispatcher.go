package app

// THE ZALO GROUP SENDER — sends the owed `zalo_group_delivery` rows (migration 0029, ADR 0092; transaction
// boundary gui_tin_nhom_zalo). The personal sender's three steps (zalo_dispatcher.go), for one commune,
// through the SAME bot it opened:
//
//  1. one transaction: claim the due rows (SKIP LOCKED); decide each —
//     kenh-tat → loai-tat (the commune switched the kind off, or the group no longer carries it) →
//     bot-chua-cau-hinh → chua-lien-ket (the link ended, or was made through a bot that no longer serves the
//     commune) → the commune's quiet hours (POSTPONE, ADR 0092 §Hệ quả: no group-specific hours) — and lease
//     the rest (attempts + 1); one audit entry
//  2. the sends, outside any transaction
//  3. one transaction: each outcome — da-gui; a retryable class owed again after a back-off; anything else,
//     or the last attempt, that-bai with its class; one audit entry
//
// A Zalo refusal (bot removed from the group, group dissolved) is that-bai and does NOT end the link:
// unlinking is a staff act on ViGov (ADR 0092 #8). The group's chat_id is personal data: never logged.

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	docstore "github.com/vihat/vigov/service-comms/internal/store"
	"github.com/vihat/vigov/service-comms/internal/zalobot"
)

// groupLabel prefixes a group row's outcome in the dispatch entry, so one verb's tally tells the two
// outboxes apart.
const groupLabel = "nhom:"

// sendGroups runs the three steps over zalo_group_delivery for one commune.
func (z *ZaloDispatcher) sendGroups(cctx context.Context, id tenant.ID, host string, bot ActiveZaloBot,
	token secret.Secret) {

	now := z.now().Truncate(time.Microsecond)
	var jobs []zaloJob
	err := z.d.DB.For(cctx).Tx(cctx, func(tx *store.ScopedTx) error {
		jobs = nil
		due, err := z.groups.ClaimDueGroupDeliveries(cctx, tx, now, docstore.MaxZaloDeliveryBatch)
		if err != nil || len(due) == 0 {
			return err
		}
		settings, err := z.d.Repo.LockChannelSetting(cctx, tx)
		if err != nil {
			return err
		}
		tally := map[string][]string{}
		for _, d := range due {
			outcome, err := z.decideGroup(cctx, tx, d, settings, bot, host, now, &jobs)
			if err != nil {
				return err
			}
			tally[groupLabel+outcome] = append(tally[groupLabel+outcome], d.ID)
		}
		return writeDispatchAudit(cctx, tx, "xep_lich", tally, now)
	})
	if err != nil {
		z.d.Log.WarnContext(cctx, "CẢNH BÁO: không xếp được tin nhóm Zalo của xã — thử lại nhịp sau", "service", "comms",
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
			var err error
			switch {
			case o == zalobot.OutcomeOK:
				err = z.groups.MarkGroupDeliverySent(cctx, tx, j.id, done)
				class = domain.ZaloDeliverySent
			case o.Retryable() && j.attempts < domain.ZaloDeliveryMaxAttempts:
				err = z.groups.RetryGroupDelivery(cctx, tx, j.id, class, done.Add(domain.ZaloDeliveryBackoff(j.attempts)), done)
				class = "thu_lai:" + class
			default:
				err = z.groups.FailGroupDelivery(cctx, tx, j.id, class, done)
				class = domain.ZaloDeliveryFailed + ":" + class
			}
			if errors.Is(err, docstore.ErrZaloRowChanged) {
				continue // settled meanwhile (another pass after the lease)
			}
			if err != nil {
				return err
			}
			tally[groupLabel+class] = append(tally[groupLabel+class], j.id)
		}
		return writeDispatchAudit(cctx, tx, "ket_qua_gui", tally, done)
	})
	if err != nil {
		z.d.Log.WarnContext(cctx, "CẢNH BÁO: không ghi được kết quả gửi tin nhóm Zalo — các tin sẽ được gửi lại sau hạn giữ",
			"service", "comms", "xa", string(id), "so_tin", len(jobs), "err", err)
	}
}

// decideGroup settles one claimed group row and returns its outcome's label for the trail.
func (z *ZaloDispatcher) decideGroup(ctx context.Context, tx *store.ScopedTx, d docstore.DueZaloGroupDelivery,
	s domain.ZaloChannelSetting, bot ActiveZaloBot, host string, now time.Time, jobs *[]zaloJob) (string, error) {

	skip := func(reason string) (string, error) {
		return domain.ZaloDeliverySkipped + ":" + reason, z.groups.SkipGroupDelivery(ctx, tx, d.ID, reason, now)
	}
	switch {
	case !s.Saved || !s.IsEnabled:
		return skip(domain.ZaloSkipChannelOff)
	case !s.KindEnabled(d.Kind) || !slices.Contains(d.GroupKinds, d.Kind):
		return skip(domain.ZaloSkipKindOff)
	case !bot.Configured:
		return skip(domain.ZaloSkipBotNotConfigured)
	case !d.LinkLive || d.BotRef != bot.Ref:
		// A link through a bot that no longer serves the commune reaches nobody through the bot in use; a
		// bot switch ends such links in its own transaction (0029), so this is the floor, not the path.
		// 0029 has no "bot changed" reason: 'chua-lien-ket' — not linked for the bot that sends.
		return skip(domain.ZaloSkipNotLinked)
	}
	if until, quiet := domain.QuietUntil(now, s.QuietStartMinute, s.QuietEndMinute); quiet {
		return "hoan_gio_yen_tinh", z.groups.PostponeGroupDelivery(ctx, tx, d.ID, until, now)
	}
	text := domain.ZaloReminderText(d.Title, d.Body, domain.AbsoluteStaffLink(host, d.Link))
	if err := z.groups.StartGroupAttempt(ctx, tx, d.ID, now.Add(zaloSendLease), now); err != nil {
		return "", err
	}
	*jobs = append(*jobs, zaloJob{id: d.ID, chatID: d.ChatID, text: text, attempts: d.Attempts + 1})
	return "dang_gui", nil
}
