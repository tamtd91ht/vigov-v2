package identityclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
)

// MaxOrgUnitIDsPerCall is the ceiling on ONE ResolveLiveOrgUnits call.
//
// IT MIRRORS THE CONTRACT (ResolveLiveOrgUnitsRequest.ids), which is the source of truth (rule 2,
// invariant 7), and it is the same number as TranMaGiaoViecMotLo from the same derivation: one
// assignment names one unit and perhaps a lead unit.
//
// REFUSED HERE RATHER THAN CHUNKED, AND NEVER TRUNCATED. Chunking would make a request the contract
// calls wrong succeed anyway — a caller sending 51 unit ids is listing the org chart, which is
// GET /api/v1/org-units' job — and truncating would answer real, live units as "not live" with
// nothing reporting it.
const MaxOrgUnitIDsPerCall = 50

// LiveOrgUnits asks identity which of these org-unit ids (`bo_phan.id`) are live units of the
// commune the context carries: the row exists in this commune and is not soft deleted — the
// predicate of GET /api/v1/org-units, stated on ResolveLiveOrgUnits in identity.proto.
//
// WHY IT EXISTS: a unit id arrives in a request body, so it is client-supplied. Written unchecked,
// a crafted body hands work to a unit of another commune, to a removed unit, or to an id naming
// nothing — and the work then sits with no unit on a list nobody opens. It is the unit half of
// CanBoGiaoViecDuoc and has the same shape on purpose.
//
// # THE CALLER DECIDES FROM THIS
//
// An id ABSENT from the returned set MUST be refused. Unknown, deleted and another commune's id are
// deliberately one answer; the user sees one sentence ("bộ phận này không nhận được việc").
//
// # AN ERROR IS NEVER "not live" AND NEVER "live"
//
// It means the check did not happen. Refuse the write — 503, retryable when
// errors.Is(err, ErrIdentityUnavailable) — and never fall back to writing the id unchecked.
//
// Ids only cross this call; nothing personal. Only the count is logged.
func (c *Client) LiveOrgUnits(ctx context.Context, ids []string) (map[string]struct{}, error) {
	// AN EMPTY REQUEST IS ANSWERED WITHOUT A ROUND TRIP. It is NOT "every live unit" — there is no
	// spelling of this request that means that.
	if len(ids) == 0 {
		return map[string]struct{}{}, nil
	}
	if len(ids) > MaxOrgUnitIDsPerCall {
		return nil, fmt.Errorf(
			"identityclient: LiveOrgUnits nhận %d mã bộ phận, vượt trần %d — một lần giao việc không cần chừng ấy bộ phận, không được cắt bớt",
			len(ids), MaxOrgUnitIDsPerCall)
	}

	// The set actually asked for, so the response can be checked against it below.
	asked := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id == "" {
			// A blank id matches no row; it is absent from the answer either way.
			continue
		}
		asked[id] = struct{}{}
	}
	if len(asked) == 0 {
		return map[string]struct{}{}, nil
	}

	// THE SAME DEADLINE AS every other method of this package, on purpose — see HanGoi.
	ctx, cancel := context.WithTimeout(ctx, HanGoi)
	defer cancel()

	resp, err := c.cl.ResolveLiveOrgUnits(ctx, &identityv1.ResolveLiveOrgUnitsRequest{Ids: ids})
	if err != nil {
		c.log.WarnContext(ctx, "CẢNH BÁO: không kiểm được bộ phận nhận việc",
			"ma_loi", status.Code(err).String(), "so_bo_phan", len(asked), "err", err)
		return nil, wrapCallError("ResolveLiveOrgUnits", err, nil)
	}

	live := make(map[string]struct{}, len(asked))
	for _, id := range resp.GetLiveIds() {
		if _, ok := asked[id]; !ok {
			// AN ID NOBODY ASKED FOR — including "". The contract says the answer is a subset of the
			// request. Refused rather than ignored: the caller is about to DECIDE from this set.
			return nil, fmt.Errorf(
				"identityclient: ResolveLiveOrgUnits trả về một bộ phận KHÔNG ĐƯỢC HỎI — phản hồi phải là tập con của yêu cầu")
		}
		live[id] = struct{}{}
	}

	// NO COMPLETENESS CHECK: an id asked for and not answered is the answer "not live".
	return live, nil
}

// StaffOrgUnits asks identity which live org unit(s) ONE member of staff sits in, in the commune the
// context carries, keyed by the staff BUSINESS CODE (authz.Principal.Ma).
//
// staffCode MUST BE THE CALLER'S OWN PRINCIPAL'S CODE, taken from the session — never from a request
// parameter (rule 4, invariant 2 in its staff form). Identity cannot check this across the hop
// (ADR 0025), so the caller is the only place it holds.
//
// # IT NARROWS A LIST. IT GRANTS NOTHING
//
// The answer serves the "Liên quan đến tôi" tab (`scope=related`). It must never be used in a
// guard: "same unit may edit" decided here is authorisation decided outside identity (rule 5).
//
// # EMPTY IS AN ORDINARY ANSWER; AN ERROR IS NOT EMPTY
//
// An empty slice (no unit, unknown code, removed record, removed unit) means the unit clause of the
// filter matches nothing — never "every unit". An error means the call did not happen: answer 503
// (retryable when errors.Is(err, ErrIdentityUnavailable)), never the tab with the unit clause
// dropped, which would hide exactly the tasks the tab is named for.
//
// NOT BATCHED, as the contract states: one request asks about one person.
func (c *Client) StaffOrgUnits(ctx context.Context, staffCode string) ([]string, error) {
	if staffCode == "" {
		// A staff principal's code is never empty (rule 6, invariant 8), so this is a wiring fault in
		// the caller. Refused locally: the server answers INVALID_ARGUMENT for exactly this, and the
		// message here names the cause.
		return nil, fmt.Errorf(
			"identityclient: StaffOrgUnits với mã cán bộ rỗng — bên gọi phải truyền Principal.Ma của chính phiên")
	}

	ctx, cancel := context.WithTimeout(ctx, HanGoi)
	defer cancel()

	resp, err := c.cl.ResolveStaffOrgUnits(ctx, &identityv1.ResolveStaffOrgUnitsRequest{Ma: staffCode})
	if err != nil {
		// The code identifies a person to anyone holding the directory, so it is not logged.
		c.log.WarnContext(ctx, "CẢNH BÁO: không lấy được bộ phận của cán bộ",
			"ma_loi", status.Code(err).String(), "err", err)
		return nil, wrapCallError("ResolveStaffOrgUnits", err, nil)
	}

	// Copied into our own slice, never aliasing the response's; duplicates collapsed so the caller's
	// SQL `IN` list is a set.
	units := make([]string, 0, len(resp.GetOrgUnitIds()))
	seen := make(map[string]struct{}, len(resp.GetOrgUnitIds()))
	for _, id := range resp.GetOrgUnitIds() {
		if id == "" {
			// A blank unit id is a contract fault, and passing it on is not harmless: a filter
			// `bo_phan_id IN ('')` against a column that may hold '' matches rows nobody holds.
			return nil, fmt.Errorf("identityclient: ResolveStaffOrgUnits trả về một mã bộ phận rỗng")
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		units = append(units, id)
	}
	return units, nil
}
