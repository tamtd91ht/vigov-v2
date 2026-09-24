package identityclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
)

// TranMaGiaoViecMotLo is the ceiling on ONE ResolveAssignableStaff call.
//
// IT MIRRORS THE CONTRACT (ResolveAssignableStaffRequest.ma), which is the source of truth (rule 2,
// invariant 7). Deliberately NOT TranMaMotLo: this is asked at the moment of an assignment, which
// names one assignee or a handful of co-handlers, and a caller sending more is listing a roster —
// GET /api/v1/staff-directory's job, behind its own permission.
//
// REFUSED HERE RATHER THAN SENT, and NEVER TRUNCATED — a clamped batch would answer real, assignable
// people as "not assignable" with nothing reporting it.
const TranMaGiaoViecMotLo = 50

// CanBoGiaoViecDuoc asks identity which of these staff codes may be handed NEW work in the commune
// the context carries: not soft-deleted, has a login account, not locked — the predicate of
// GET /api/v1/staff-directory, stated on ResolveAssignableStaff in identity.proto.
//
// WHY IT EXISTS: an assignee code arrives in a request body, so it is client-supplied. Written
// unchecked, a crafted body hands work to a code of another commune, a locked former employee, or a
// person with no account — and the work then sits with nobody, because every holder check compares
// the stored code with a signed-in Principal.Ma that will never exist.
//
// # THE CALLER DECIDES FROM THIS — unlike TenCanBoTheoMa
//
// A code ABSENT from the returned set MUST be refused. The five reasons a code is absent (unknown,
// deleted, no account, locked, another commune) are deliberately one answer; the caller's user sees
// one sentence and learns nothing about which.
//
// # AN ERROR IS NEVER "not assignable" AND NEVER "assignable"
//
// It means the check did not happen. Refuse the write with a retryable error (503); never fall back
// to writing the code unchecked, and never report it to the user as a bad assignee.
//
// NO PERSONAL DATA crosses this call — codes only — but a code still identifies a person to anyone
// holding the directory, so nothing below logs which codes were asked, only how many.
func (c *Client) CanBoGiaoViecDuoc(ctx context.Context, ma []string) (map[string]struct{}, error) {
	// AN EMPTY REQUEST IS ANSWERED WITHOUT A ROUND TRIP. It is NOT "everyone assignable" — there is
	// no spelling of this request that means that.
	if len(ma) == 0 {
		return map[string]struct{}{}, nil
	}
	if len(ma) > TranMaGiaoViecMotLo {
		return nil, fmt.Errorf(
			"identityclient: CanBoGiaoViecDuoc nhận %d mã, vượt trần %d — một lần giao việc không cần chừng ấy người, không được cắt bớt",
			len(ma), TranMaGiaoViecMotLo)
	}

	// The set actually asked for, so the response can be checked against it below.
	daHoi := make(map[string]struct{}, len(ma))
	for _, m := range ma {
		if m == "" {
			// A blank code matches no row; it is absent from the answer either way, so it is not sent.
			continue
		}
		daHoi[m] = struct{}{}
	}
	if len(daHoi) == 0 {
		return map[string]struct{}{}, nil
	}

	// THE SAME DEADLINE AS every other method of this package, on purpose — see HanGoi.
	ctx, huy := context.WithTimeout(ctx, HanGoi)
	defer huy()

	ra, err := c.cl.ResolveAssignableStaff(ctx, &identityv1.ResolveAssignableStaffRequest{Ma: ma})
	if err != nil {
		c.log.WarnContext(ctx, "CẢNH BÁO: không kiểm được cán bộ nhận việc",
			"ma_loi", status.Code(err).String(), "so_ma", len(daHoi), "err", err)
		return nil, fmt.Errorf("identityclient: ResolveAssignableStaff: %w", err)
	}

	duoc := make(map[string]struct{}, len(daHoi))
	for _, m := range ra.GetAssignableMa() {
		if _, co := daHoi[m]; !co {
			// A CODE NOBODY ASKED FOR — including "". The contract says the answer is a subset of the
			// request. Refused rather than ignored: the caller is about to DECIDE from this set, and a
			// far end that answers outside the question is not one to decide from.
			return nil, fmt.Errorf(
				"identityclient: ResolveAssignableStaff trả về một mã KHÔNG ĐƯỢC HỎI — phản hồi phải là tập con của yêu cầu")
		}
		duoc[m] = struct{}{}
	}

	// NO COMPLETENESS CHECK: a code asked for and not answered is the answer "not assignable".
	return duoc, nil
}
