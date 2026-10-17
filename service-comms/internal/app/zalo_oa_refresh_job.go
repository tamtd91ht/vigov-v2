package app

// THE ZALO OA TOKEN REFRESH JOB (ADR 0098 #6; migration 0031). Zalo's access token lives 25 hours and its
// refresh token is single-use; nothing may ever send with an expired token. So a job refreshes the access
// token of EVERY live OA of every commune — several per commune since ADR 0099 #1, each with its own token
// pair — while less than domain.AccessRefreshLead (2 hours) remains, and flags the row "needs
// re-authorization" when Zalo refuses the refresh token or it aged past 3 months. The cross-commune list
// names the communes; inside one commune, CommuneZaloOAs.RefreshIfDue walks that commune's due OAs.
//
// THE SHAPE IS THE DISPATCHER'S (zalo_dispatcher.go): an in-process ticker, ONE advisory lock per tick so
// one replica refreshes (two replicas presenting the same single-use refresh token would void each
// other's result), the communes with work listed as identifiers by crosstenant.ZaloOA, each commune IN ITS
// OWN CONTEXT behind its own recover, an inactive (merged) commune left untouched.
//
// THE SAME TICK SWEEPS EXPIRED AUTHORIZATION REQUESTS: an open zalo_oa_authorization past its 10 minutes
// is closed `het-han` (0031 question 4) so the table never holds a "live-looking" link. Both writes are
// audited by the system principal (rule 6, invariant 6).

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

const (
	// ZaloOARefreshTickInterval — a VENDOR choice. With a 2-hour lead, a 15-minute tick leaves eight
	// chances before a token expires; a tick with no work is two identifiers-only queries.
	ZaloOARefreshTickInterval = 15 * time.Minute
	// zaloOARefreshTickBudget bounds one tick and so the scheduler lock's hold.
	zaloOARefreshTickBudget = 5 * time.Minute
)

// ZaloOARefreshLocks is *crosstenant.ZaloOA's job half.
type ZaloOARefreshLocks interface {
	CommunesWithOAToRefresh(ctx context.Context, before time.Time) ([]tenant.ID, error)
	CommunesWithOpenAuthorizations(ctx context.Context, now time.Time) ([]tenant.ID, error)
	TryLockRefreshScheduler(ctx context.Context) (func(), bool, error)
}

// ZaloOATokenKeeper is the per-commune work. *CommuneZaloOAs satisfies it; tests fake it.
type ZaloOATokenKeeper interface {
	RefreshIfDue(ctx context.Context) (bool, error)
	ExpireOpenAuthorizations(ctx context.Context, now time.Time) (int, error)
}

// ZaloOARefreshJobDeps wires the job. Registry is REQUIRED: a merged commune is left untouched.
type ZaloOARefreshJobDeps struct {
	Locks    ZaloOARefreshLocks
	Keeper   ZaloOATokenKeeper
	Registry PortalCommuneRegistry
	Log      *slog.Logger
}

// ZaloOARefreshJob refreshes OA tokens and sweeps expired authorization requests. Run blocks until ctx
// is done.
type ZaloOARefreshJob struct {
	d          ZaloOARefreshJobDeps
	interval   time.Duration
	tickBudget time.Duration
	now        func() time.Time
}

// NewZaloOARefreshJob builds the job.
func NewZaloOARefreshJob(d ZaloOARefreshJobDeps) (*ZaloOARefreshJob, error) {
	if d.Locks == nil || d.Keeper == nil || d.Registry == nil {
		return nil, errors.New("zalo_oa: việc nền làm mới token thiếu phụ thuộc")
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &ZaloOARefreshJob{d: d, interval: ZaloOARefreshTickInterval, tickBudget: zaloOARefreshTickBudget,
		now: func() time.Time { return time.Now().UTC() }}, nil
}

// Run ticks until ctx is cancelled — once at start, then every interval.
func (j *ZaloOARefreshJob) Run(ctx context.Context) {
	j.Tick(ctx)
	t := time.NewTicker(j.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			j.Tick(ctx)
		}
	}
}

// Tick does one pass: refresh every commune with a token due, then sweep expired requests.
func (j *ZaloOARefreshJob) Tick(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	release, ok, err := j.d.Locks.TryLockRefreshScheduler(ctx)
	if err != nil {
		j.d.Log.WarnContext(ctx, "CẢNH BÁO: không thử được khoá việc nền làm mới token OA Zalo, bỏ nhịp này",
			"service", "comms", "err", err)
		return
	}
	if !ok {
		return // another replica refreshes
	}
	defer release()
	tctx, cancel := context.WithTimeout(ctx, j.tickBudget)
	defer cancel()

	now := j.now()
	due, err := j.d.Locks.CommunesWithOAToRefresh(tctx, now.Add(domain.AccessRefreshLead))
	if err != nil {
		j.d.Log.WarnContext(ctx, "CẢNH BÁO: không liệt kê được xã có token OA Zalo sắp hết hạn", "service", "comms", "err", err)
	} else {
		for i, id := range due {
			if tctx.Err() != nil {
				j.d.Log.WarnContext(ctx, "CẢNH BÁO: nhịp làm mới token OA Zalo hết thời gian, các xã còn lại chờ nhịp sau",
					"service", "comms", "con_lai", len(due)-i)
				return
			}
			j.inCommune(tctx, id, "làm mới token OA", func(cctx context.Context) error {
				_, err := j.d.Keeper.RefreshIfDue(cctx)
				return err
			})
		}
	}

	open, err := j.d.Locks.CommunesWithOpenAuthorizations(tctx, now)
	if err != nil {
		j.d.Log.WarnContext(ctx, "CẢNH BÁO: không liệt kê được xã có yêu cầu uỷ quyền OA quá hạn", "service", "comms", "err", err)
		return
	}
	for _, id := range open {
		if tctx.Err() != nil {
			return
		}
		j.inCommune(tctx, id, "đóng yêu cầu uỷ quyền OA quá hạn", func(cctx context.Context) error {
			_, err := j.d.Keeper.ExpireOpenAuthorizations(cctx, now)
			return err
		})
	}
}

// inCommune runs fn in the commune's own context, behind its own recover. A merged or unknown commune is
// left untouched (rule 7, invariant 6). Nothing it does can stop the next commune. The error logged is
// the use case's — it never carries a token (ADR 0098 #13: Zalo failures are classes).
func (j *ZaloOARefreshJob) inCommune(ctx context.Context, id tenant.ID, what string, fn func(context.Context) error) {
	defer func() {
		if p := recover(); p != nil {
			j.d.Log.ErrorContext(ctx, "LỖI: việc nền OA Zalo của một xã bị panic, chuyển sang xã kế", "service", "comms",
				"xa", string(id), "viec", what, "panic", fmt.Sprint(p))
		}
	}()
	cctx := tenant.Into(ctx, id)
	t, ok, err := j.d.Registry.XaTrongNguCanh(cctx)
	if err != nil {
		j.d.Log.WarnContext(cctx, "CẢNH BÁO: không đọc được trạng thái xã, bỏ xã này ở nhịp này", "service", "comms",
			"xa", string(id), "err", err)
		return
	}
	if !ok || !t.Active {
		return
	}
	if err := fn(cctx); err != nil {
		j.d.Log.WarnContext(cctx, "CẢNH BÁO: việc nền OA Zalo của xã chưa xong — thử lại nhịp sau", "service", "comms",
			"xa", string(id), "viec", what, "err", err)
	}
}
