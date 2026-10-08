package petitionsclient

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc/status"

	petitionsv1 "github.com/vihat/vigov/core/gen/vigov/petitions/v1"
)

// MaxTaskPriorityCodesPerCall mirrors the ceiling of ResolveTaskPriorityCodesRequest
// (proto/vigov/petitions/v1/petitions.proto, the source of truth — rule 2, invariant 7). REFUSED HERE
// RATHER THAN SENT, AND NEVER TRUNCATED: a clamped batch answers real codes as "absent", and the caller
// would refuse them as unknown. The caller chunks.
const MaxTaskPriorityCodesPerCall = 50

// TaskPriorityCodes asks petitions which of the given task-priority codes (`muc_uu_tien_nhiem_vu.ma`) a
// LIVE priority of the commune in ctx carries. The map holds one entry per ANSWERED code, its value the
// priority's `dang_dung`. The predicate is stated once, on the RPC in petitions.proto.
//
// # READING THE ANSWER — fail closed
//
//	present, true     the commune assigns tasks at this priority.
//	present, false    the priority exists but is switched off.
//	absent            no live priority of this commune carries the code — unknown, soft-deleted and
//	                  another commune's code are one answer.
//	error             the call did not happen (retryable when errors.Is(err, ErrPetitionsUnavailable)).
//	                  NEVER "valid", never "unknown".
//
// Codes are sent EXACTLY as given — the server matches exactly, and the caller stores what it sent. A
// blank code is refused here (the server would answer INVALID_ARGUMENT); duplicates are sent once. An
// empty list answers an empty map without a round trip. Not cached.
func (c *Client) TaskPriorityCodes(ctx context.Context, codes []string) (map[string]bool, error) {
	if len(codes) > MaxTaskPriorityCodesPerCall {
		return nil, fmt.Errorf(
			"petitionsclient: TaskPriorityCodes nhận %d mã, vượt trần %d — bên gọi phải tự chia lô, không được cắt bớt",
			len(codes), MaxTaskPriorityCodesPerCall)
	}
	asked := make(map[string]struct{}, len(codes))
	sent := make([]string, 0, len(codes))
	for _, code := range codes {
		if strings.TrimSpace(code) == "" {
			return nil, fmt.Errorf("petitionsclient: TaskPriorityCodes với mã rỗng — bên gọi phải chuẩn hoá trước khi hỏi")
		}
		if _, dup := asked[code]; dup {
			continue
		}
		asked[code] = struct{}{}
		sent = append(sent, code)
	}
	if len(sent) == 0 {
		return map[string]bool{}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, CallTimeout)
	defer cancel()

	resp, err := c.cl.ResolveTaskPriorityCodes(ctx, &petitionsv1.ResolveTaskPriorityCodesRequest{Codes: sent})
	if err != nil {
		c.log.WarnContext(ctx, "CẢNH BÁO: không tra được mã mức ưu tiên nhiệm vụ ở petitions",
			"ma_loi", status.Code(err).String(), "so_ma", len(sent), "err", err)
		return nil, wrapCallError("ResolveTaskPriorityCodes", err)
	}

	out := make(map[string]bool, len(sent))
	for _, it := range resp.GetItems() {
		code, active := it.GetCode(), it.GetActive()
		if _, ok := asked[code]; !ok {
			// The caller writes from this map: a key it did not ask for is refused, not ignored.
			return nil, fmt.Errorf(
				"petitionsclient: ResolveTaskPriorityCodes trả về một mã KHÔNG ĐƯỢC HỎI — phản hồi phải là tập con của yêu cầu")
		}
		if prev, dup := out[code]; dup && prev != active {
			// `(tenant_id, ma)` is unique, so one code is one priority. Two different answers means the
			// far end is wrong, and picking either decides a write on a guess.
			return nil, fmt.Errorf("petitionsclient: ResolveTaskPriorityCodes trả hai trạng thái khác nhau cho cùng một mã")
		}
		out[code] = active
	}
	// NO COMPLETENESS CHECK: an unanswered code is the answer "no live priority carries it".
	return out, nil
}
