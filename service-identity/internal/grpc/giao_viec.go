package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
)

// TranMaGiaoViecMotLo is the ceiling on ResolveAssignableStaffRequest.ma, declared where it is
// enforced. 50, NOT TranIDMotLo's 200: the contract derives it from the callers — one assignment
// names one assignee or a handful of co-handlers — and a caller needing more is listing a roster,
// which is GET /api/v1/staff-directory's job behind its own permission.
const TranMaGiaoViecMotLo = 50

// CanBoGiaoViec is the read behind ResolveAssignableStaff. SEPARATE FROM CanBoLo and CanBoTen, and
// the separation is the point: each of the three answers a different question with a different
// predicate, and this one's predicate is the PICKER's (store.locChonNguoi), shared by constant.
type CanBoGiaoViec interface {
	GiaoViecDuoc(ctx context.Context, ma []string) ([]string, error)
}

// ResolveAssignableStaff answers which of the requested staff codes may be handed NEW work in the
// commune named by "x-tenant-id": not soft deleted, has an account, not locked. The full contract is
// on the RPC in identity.proto; what this handler owns is the boundary around the read.
//
// ORDER: ceiling (on what was SENT, before dedup) → commune present → empty answers empty → read.
// The same order as ResolveStaffNames, for the same reasons stated there: refusing an unbounded
// request before any read, and never reaching store.Scoped (which panics) without a commune.
//
// ONE ANSWER FOR EVERY REFUSAL — absent. Unknown, deleted, account-less, locked and another commune's
// code are indistinguishable here; there is no field in which to tell them apart and none may be
// added (rule 1: "another commune" vs "unknown" leaks existence; "locked" publishes an employment
// fact).
//
// NEVER NOT_FOUND, NEVER UNAUTHENTICATED. A non-assignable code is an absent code; UNAUTHENTICATED
// belongs to the caller-key interceptor.
//
// LOG COUNTS, NEVER CODES. A code identifies a person to anyone holding the directory.
//
// NOT AUDITED: a read of codes with no principal to name. The caller's WRITE of the assignee is the
// audited act, in the caller's own transaction.
func (s *Server) ResolveAssignableStaff(ctx context.Context, req *identityv1.ResolveAssignableStaffRequest) (
	*identityv1.ResolveAssignableStaffResponse, error) {

	ma := req.GetMa()

	// Refused, never truncated: a clamped batch answers real, assignable people as "not assignable".
	if len(ma) > TranMaGiaoViecMotLo {
		return nil, status.Errorf(codes.InvalidArgument,
			"ma vượt trần %d cho một lời gọi — một lần giao việc không cần chừng ấy người", TranMaGiaoViecMotLo)
	}

	if _, err := s.xa(ctx); err != nil {
		return nil, err
	}

	can := locID(ma)
	if len(can) == 0 {
		// EMPTY ANSWERS EMPTY, held at the boundary the contract names — never "every assignable
		// person", and not dependent on which store is wired into Deps.GiaoViec.
		return &identityv1.ResolveAssignableStaffResponse{}, nil
	}

	duoc, err := s.d.GiaoViec.GiaoViecDuoc(ctx, can)
	if err != nil {
		return nil, s.loi(ctx, err, "ResolveAssignableStaff")
	}

	// THE ANSWER IS A SUBSET OF THE QUESTION, EACH CODE ONCE — enforced here rather than trusted to
	// the SQL, because the caller DECIDES from this set and treats an unrequested code as a contract
	// fault. The store already guarantees both; this makes the property independent of it.
	daHoi := make(map[string]struct{}, len(can))
	for _, m := range can {
		daHoi[m] = struct{}{}
	}
	ra := make([]string, 0, len(duoc))
	for _, m := range duoc {
		if _, co := daHoi[m]; !co {
			continue
		}
		delete(daHoi, m)
		ra = append(ra, m)
	}
	return &identityv1.ResolveAssignableStaffResponse{AssignableMa: ra}, nil
}
