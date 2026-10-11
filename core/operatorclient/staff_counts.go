package operatorclient

// OperatorService.ListActiveStaffCounts (proto/vigov/identity/v1/operator.proto): per commune, how
// many ACTIVE STAFF ACCOUNTS it has — the denominator of "Đã ghép nối" on platform-admin's Zalo Bot
// communes table (ADR 0074, owner 11/10/2026).
//
// NO COMMUNE TRAVELS: the RPC answers about every commune at once and is on
// core/grpcx.methodsWithoutTenant by full name (owner, 2026-10-11). Unlike the Mini App secret calls
// there is therefore no requireCommune here — a commune in ctx would not be sent anyway.
//
// The reader is resolved by identity from the `op1.` token, which also refreshes the idle timer: call
// it only for a request a person made. Nothing here logs a request or a response.

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/secret"
)

// ListActiveStaffCountsResult: Counts is meaningful exactly when Outcome is OutcomeAccepted. Keyed by
// commune ULID; a commune absent from the map has zero active staff accounts (operator.proto).
type ListActiveStaffCountsResult struct {
	Outcome Outcome
	Counts  map[string]uint32
}

// ListActiveStaffCounts is OperatorService.ListActiveStaffCounts.
//
//	err == nil        Outcome ACCEPTED (Counts), SESSION_NOT_LIVE (401) or PERMISSION_DENIED (403)
//	ErrNotSupported   identity predates the RPC (UNIMPLEMENTED)
//	ErrContract       UNSPECIFIED, an outcome outside the subset, or an entry that breaks the contract
//	                  (empty tenant_id, a zero count, a commune twice) — never read as "zero staff"
//	ErrInvalidRequest empty token — a wiring fault in the caller; nothing sent
//	any other error   the call did not happen (identity down, trail entry not written → UNAVAILABLE)
func (c *Client) ListActiveStaffCounts(ctx context.Context, token secret.Secret, clientIP string) (ListActiveStaffCountsResult, error) {
	const rpc = "ListActiveStaffCounts"
	if token.Rong() {
		return ListActiveStaffCountsResult{}, fmt.Errorf("%w: %s with an empty token", ErrInvalidRequest, rpc)
	}
	ctx, cancel := context.WithTimeout(ctx, CallTimeout)
	defer cancel()
	ra, err := c.cl.ListActiveStaffCounts(ctx, &identityv1.ListActiveStaffCountsRequest{
		SessionToken: raw(token),
		ClientIp:     clientIP,
	})
	if err != nil {
		if status.Code(err) == codes.Unimplemented {
			return ListActiveStaffCountsResult{}, fmt.Errorf("%w: %s", ErrNotSupported, rpc)
		}
		return ListActiveStaffCountsResult{}, c.callFailed(ctx, rpc, err)
	}
	o, err := outcome(rpc, ra.GetOutcome(), wAccepted, wNotLive, wDenied)
	if err != nil {
		return ListActiveStaffCountsResult{}, err
	}
	if o == OutcomeRefused {
		// UNSPECIFIED: fail closed — an answer nobody filled in must never read as "no staff anywhere".
		return ListActiveStaffCountsResult{}, fmt.Errorf("%w: %s answered no outcome", ErrContract, rpc)
	}
	res := ListActiveStaffCountsResult{Outcome: o}
	if o != OutcomeAccepted {
		return res, nil
	}
	res.Counts = make(map[string]uint32, len(ra.GetCounts()))
	for _, e := range ra.GetCounts() {
		id, n := e.GetTenantId(), e.GetActiveStaffCount()
		if id == "" || n == 0 {
			return ListActiveStaffCountsResult{}, fmt.Errorf("%w: %s entry without a commune or with a zero count", ErrContract, rpc)
		}
		if _, dup := res.Counts[id]; dup {
			// Two counts for one commune: which one is the denominator is not something to guess.
			return ListActiveStaffCountsResult{}, fmt.Errorf("%w: %s named one commune twice", ErrContract, rpc)
		}
		res.Counts[id] = n
	}
	return res, nil
}
