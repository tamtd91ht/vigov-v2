package identityclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
)

// ResolveCitizenContactPhone asks identity for the FULL verified phone of the citizen behind one live
// citizen session, so the citizen intake can attach it to a petition sent with the phone box empty
// (ADR 0050 §Sửa đổi 08/10/2026 point 2). The contract, and why it is keyed on a session rather than a
// citizen id, is on the RPC in proto/vigov/identity/v1/identity.proto.
//
// THE THREE OUTCOMES ARE KEPT APART HERE, because this is the only place that sees a gRPC status:
//
//	err != nil         the call did not happen (UNAVAILABLE, UNIMPLEMENTED on an older identity, the
//	                   caller key, the commune interceptor). NEVER "no phone": the caller fails the
//	                   intake with 503, exactly as it does when ResolveDeadlines cannot be reached.
//	"" , nil           identity answered and disclosed nothing — no live session of this citizen in this
//	                   commune at this instant. The caller refuses the intake as "no session" (401); it
//	                   never files the petition without the number the citizen's screen promised.
//	phone, nil         the number, already disclosed and audited by identity.
//
// THE COMMUNE TRAVELS IN METADATA through grpcx.UnaryClientInterceptor, like every other call on this
// client. The RPC is not on grpcx.methodsWithoutTenant and must never be: a context with no commune is
// refused locally with InvalidArgument before anything leaves this process.
//
// NEVER LOGGED, AT EITHER END (rule 3). The generated String() of the response prints the number in
// full, so neither message is ever passed to a logger, and the number is never put into an error. The
// warning below carries the gRPC code and nothing else.
func (c *Client) ResolveCitizenContactPhone(ctx context.Context, sessionID, citizenID string) (string, error) {
	// REFUSED LOCALLY: identity answers INVALID_ARGUMENT for both, and an operator reading that would
	// inspect the contract for a fault that is in the caller's wiring. A session with no citizen id
	// (ADR 0080) has no verified phone and must not ask for one.
	if sessionID == "" {
		return "", fmt.Errorf("identityclient: ResolveCitizenContactPhone không có session_id — " +
			"bên gọi phải lấy nó từ phiên của chính yêu cầu")
	}
	if citizenID == "" {
		return "", fmt.Errorf("identityclient: ResolveCitizenContactPhone không có citizen_id — " +
			"phiên chưa xác thực số không có số để gắn")
	}

	// THE SAME DEADLINE AS EVERY OTHER CALL ON THIS CLIENT: one indexed session read and one audit
	// insert in identity, once per intake.
	ctx, cancel := context.WithTimeout(ctx, HanGoi)
	defer cancel()

	resp, err := c.cl.ResolveCitizenContactPhone(ctx, &identityv1.ResolveCitizenContactPhoneRequest{
		SessionId: sessionID,
		CitizenId: citizenID,
	})
	if err != nil {
		// Warn, not Error: this service is healthy. NO session id, NO citizen id, NO number — the code
		// is what tells an operator "identity down" from "identity older than this RPC" (deploy order).
		c.log.WarnContext(ctx, "CẢNH BÁO: không lấy được số đã xác thực của phiên công dân — "+
			"lượt gửi phản ánh để trống số sẽ bị từ chối 503",
			"ma_loi", status.Code(err).String())
		return "", fmt.Errorf("identityclient: ResolveCitizenContactPhone: %w", err)
	}
	return resp.GetVerifiedPhone(), nil
}
