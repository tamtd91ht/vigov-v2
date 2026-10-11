package app

// OperatorService.ListActiveStaffCounts' use case (operator.proto; owner's decision 11/10/2026): per
// commune, how many active staff accounts it has, for the operator's Zalo Bot communes table. A READ
// ACROSS COMMUNES, so it is audited (rule 6, invariant 7) in identity's operator trail — and FAILS
// CLOSED: the counts are returned only once the entry is committed. An unaudited cross-commune read is
// the state rule 6 forbids, so "the entry could not be written" means "nothing was answered".
//
// ORDER: read, THEN write the entry. The entry names how many communes were answered, which is only
// known after the read; and a read that fails leaves no entry claiming an answer that never left.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/service-identity/internal/domain"
	"github.com/vihat/vigov/service-identity/internal/store/crosstenant"
)

// ErrActiveStaffCountsUnaudited — the read succeeded but its trail entry could not be committed, so
// no count is returned. internal/grpc answers UNAVAILABLE (operator.proto: "FAIL CLOSED").
var ErrActiveStaffCountsUnaudited = errors.New("active staff counts: the trail entry could not be written — nothing is returned")

// ActiveStaffCountReader is *crosstenant.ActiveStaffCountStore's read, declared at the point of use.
type ActiveStaffCountReader interface {
	ActiveStaffCounts(ctx context.Context) ([]crosstenant.CommuneStaffCount, error)
}

// CommuneActiveStaffCount is one commune's count: the commune's ULID and a number >= 1. Nothing about
// any person.
type CommuneActiveStaffCount struct {
	TenantID string
	Count    uint32
}

// ActiveStaffCounts is the use case. It has no commune and reads none from ctx: the answer is every
// commune at once (the RPC is on core/grpcx.methodsWithoutTenant by full name).
type ActiveStaffCounts struct {
	store  OperatorStore
	reader ActiveStaffCountReader
	now    func() time.Time
}

func NewActiveStaffCounts(store OperatorStore, reader ActiveStaffCountReader, now func() time.Time) *ActiveStaffCounts {
	if store == nil || reader == nil {
		panic("app.NewActiveStaffCounts: store and reader are required")
	}
	if now == nil {
		now = time.Now
	}
	return &ActiveStaffCounts{store: store, reader: reader, now: now}
}

// ReadAsOperator returns the counts for op — the operator internal/grpc resolved from a live `op1.`
// session and checked for an ops.* key. op.Code is the trail's "who" (rule 6, invariant 8): empty is
// refused before anything is read, never replaced by the internal id.
func (u *ActiveStaffCounts) ReadAsOperator(ctx context.Context, op MiniAppOperator) ([]CommuneActiveStaffCount, error) {
	actor, err := operatorActor(op)
	if err != nil {
		return nil, err
	}
	rows, err := u.reader.ActiveStaffCounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("active staff counts: read: %w", err)
	}
	// One entry per answered call, in its own transaction — there is no business write to share one
	// with. The number of communes answered, never a count per commune (operator.proto).
	err = u.store.InTx(ctx, func(tx OperatorTx) error {
		return tx.AppendAudit(ctx, domain.OperatorAuditEntry{
			OccurredAt: u.now().UTC(),
			Actor:      actor.code,
			ActorIP:    actor.ip,
			Action:     domain.OperatorAuditActiveStaffCountsRead,
			Subject:    actor.code,
			After:      map[string]any{"communes": len(rows)},
		})
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrActiveStaffCountsUnaudited, err)
	}
	out := make([]CommuneActiveStaffCount, 0, len(rows))
	for _, r := range rows {
		out = append(out, CommuneActiveStaffCount{TenantID: r.TenantID, Count: r.Count})
	}
	return out, nil
}
