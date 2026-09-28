package app

// Verifying the org-unit ids a task act is about to WRITE — `bo_phan_id` ("unit") and
// `co_quan_chu_tri_id` ("lead_unit") — against identity's ResolveLiveOrgUnits (user decision
// 28/09/2026; proto/vigov/identity/v1/identity.proto on that RPC).
//
// WHY: the ids arrive in the request BODY, so they are client-supplied. Written unchecked, a crafted body
// hands work to a unit of ANOTHER commune, to a unit removed from the org chart, or to an id naming
// nothing — and the work then sits with no unit, forever, on a list nobody opens. This is the unit half
// of the staff check (checkAssignableStaff, kiemLanhDaoGiaoViec) and has its shape on purpose.
//
// WHO CALLS IT: task creation (Tao / TaoTuNguon — both doors) and the assignment act (Reassign), each
// BEFORE its transaction opens: a gRPC round trip inside would hold a government register's row lock
// for a network call, and an identity outage would become a register that hangs rather than refuses.

import (
	"context"
	"errors"
	"fmt"
)

// OrgUnitChecker asks identity which of these org-unit ids are live units of the commune the context
// carries. *identityclient.Client satisfies it as it is (LiveOrgUnits).
type OrgUnitChecker interface {
	LiveOrgUnits(ctx context.Context, ids []string) (map[string]struct{}, error)
}

// ErrOrgUnitNotLive refuses an id identity did not answer as a live unit of THIS commune. 400.
//
// ONE ERROR FOR THREE REASONS — unknown, removed, another commune — because the contract answers them
// alike ("absent") and telling "another commune" apart would leak that the id exists elsewhere (rule 1).
// The id is not echoed.
var ErrOrgUnitNotLive = errors.New("nhiem_vu: bộ phận được chọn không nhận được việc trong xã này")

// ErrOrgUnitUnchecked means identity could not be asked (or the check is not wired), so NOTHING was
// written. Never "live", never "not live": the check did not happen. Retryable — the handler answers 503.
var ErrOrgUnitUnchecked = errors.New("nhiem_vu: chưa kiểm được bộ phận nhận việc")

// checkLiveOrgUnits refuses unless every non-empty id is live. Empty ids ask nothing — "no lead unit" is
// a real answer on a `co-ban` task, and an empty `unit` is refused elsewhere where it must be.
//
// FAIL CLOSED: no checker wired, with an id to check, refuses; it never writes an id unchecked.
func (uc *GhiNhiemVu) checkLiveOrgUnits(ctx context.Context, ids ...string) error {
	asked := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		asked = append(asked, id)
	}
	if len(asked) == 0 {
		return nil
	}
	if uc.orgUnits == nil {
		return fmt.Errorf("%w: chưa nối dây kiểm bộ phận", ErrOrgUnitUnchecked)
	}
	live, err := uc.orgUnits.LiveOrgUnits(ctx, asked)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrOrgUnitUnchecked, err)
	}
	for _, id := range asked {
		if _, ok := live[id]; !ok {
			return ErrOrgUnitNotLive
		}
	}
	return nil
}
