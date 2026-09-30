package app

// The "CẦN XỬ LÝ NGAY" queue of /tong-quan for the incoming register
// (docs/ui-ux/01-tong-quan-dieu-hanh.md §5).
//
// WHY A USE CASE AND NOT A STORE CALL IN THE HANDLER: deciding which late document is CRITICAL needs
// the commune's working calendar, which lives in identity and is reachable only over gRPC (rule 2,
// forbidden #2; ADR 0007). That call and the rule around it are business logic; the handler
// translates HTTP and the store knows SQL.
//
// READS ONLY, SO NO TRANSACTION AND NO AUDIT ENTRY: rule 6, invariant 7 audits reading full personal
// data or reading across communes, and this does neither — no free-text field leaves this use case
// towards the queue, and the commune is bound from the context.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/service-documents/internal/domain"
)

// OverdueIncomingReader is the store half, declared at the point of use. *store.VanBanDenStore
// satisfies it.
type OverdueIncomingReader interface {
	OverdueIncoming(ctx context.Context, now time.Time, limit int) ([]domain.VanBanDen, error)
}

// WorkingHoursAdvancer asks identity when a number of WORKING hours has elapsed from an instant, in
// the commune of the context. *identityclient.Client satisfies it as it is — the same client the
// register already holds for booking deadlines.
type WorkingHoursAdvancer interface {
	// vi-name-ok: mirrors the existing exported method core/identityclient.Client.TienGioLamViec
	TienGioLamViec(ctx context.Context, tuLuc time.Time, gio []uint32) (map[uint32]time.Time, error)
}

// ErrWorkingCalendarUnavailable — identity could not answer, so no item can be classed critical or
// not. ONE SENTINEL FOR EVERY CAUSE (calendar not configured, identity unreachable), the same choice
// ErrChuaAnDinhDuocHan makes; the gRPC code rides in the wrapped chain and core/identityclient has
// already logged it.
//
// THE REQUEST FAILS. A queue with `critical: false` on every row because the calendar was out of
// reach tells the chairman nothing is urgent, which is the one wrong answer a red block must not give.
var ErrWorkingCalendarUnavailable = errors.New(
	"van_ban_den: không hỏi được lịch làm việc của xã để xếp mức khẩn")

// OverdueQueueItem is one row of the queue: the document, and whether it is critical.
type OverdueQueueItem struct {
	Document domain.VanBanDen
	Critical bool
}

// IncomingDashboard serves the queue.
type IncomingDashboard struct {
	reader   OverdueIncomingReader
	advancer WorkingHoursAdvancer
}

func NewIncomingDashboard(reader OverdueIncomingReader, advancer WorkingHoursAdvancer) *IncomingDashboard {
	return &IncomingDashboard{reader: reader, advancer: advancer}
}

// OverdueQueue returns at most `limit` overdue documents, longest missed first, each with `Critical`.
//
// CRITICAL = the instant CriticalOverdueWorkingHours working hours after the missed deadline has
// arrived: AdvanceWorkingHours(due, 48) <= now. Identity counts it against THIS commune's calendar —
// nights, weekends, `ngay_nghi_le` and `ngay_lam_bu` are not working hours, and adding a duration
// here would count them (rule 10, forbidden #2).
//
// ONE RPC PER DISTINCT DEADLINE, AND THAT IS A MEASURED EXCEPTION TO skills/load-data-once, NOT AN
// OVERSIGHT. AdvanceWorkingHoursRequest takes ONE `count_from` and several amounts; here each item has
// its own origin and one amount, which is the transpose of what the RPC batches. N is bounded by
// domain.OverdueQueueMax (10), and equal deadlines share one call. The contract change that would make
// it one round trip — several origins per request — is a proto change, reported rather than made here.
func (d *IncomingDashboard) OverdueQueue(ctx context.Context, now time.Time,
	limit int) ([]OverdueQueueItem, error) {

	docs, err := d.reader.OverdueIncoming(ctx, now, limit)
	if err != nil {
		return nil, fmt.Errorf("hàng đợi quá hạn văn bản đến: %w", err)
	}

	// Keyed by the instant, not by time.Time: two equal instants with different locations are two
	// different map keys.
	reached := make(map[int64]time.Time, len(docs))
	out := make([]OverdueQueueItem, 0, len(docs))
	for _, v := range docs {
		due := v.HanXuLyXong.UTC()
		at, seen := reached[due.UnixNano()]
		if !seen {
			m, err := d.advancer.TienGioLamViec(ctx, due, []uint32{domain.CriticalOverdueWorkingHours})
			if err != nil {
				return nil, fmt.Errorf("%w: %w", ErrWorkingCalendarUnavailable, err)
			}
			at, seen = m[domain.CriticalOverdueWorkingHours]
			if !seen || at.IsZero() {
				// identityclient already refuses a partial answer; checked again because a zero here
				// would make EVERY item critical, and the check costs nothing.
				return nil, fmt.Errorf("%w: không có mốc cho %d giờ làm việc",
					ErrWorkingCalendarUnavailable, domain.CriticalOverdueWorkingHours)
			}
			reached[due.UnixNano()] = at
		}
		out = append(out, OverdueQueueItem{Document: v, Critical: !at.After(now)})
	}
	return out, nil
}
