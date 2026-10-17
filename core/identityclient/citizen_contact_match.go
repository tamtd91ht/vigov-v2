package identityclient

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc/status"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/phone"
)

// The two RPCs of the petition ↔ citizen-directory link (ADR 0102 #2). The contracts — who may call,
// what must never happen to the number — are on the RPCs in proto/vigov/identity/v1/identity.proto;
// this file keeps the three outcomes of each call apart, which only a gRPC client can see:
//
//	err != nil      the call did not happen. NEVER "not in the directory": a caller that folded an outage
//	                into not-found would offer to add somebody who is already there, or show a person with
//	                no petitions (transaction-boundaries.json, lien_ket_phan_anh_danh_ba_nguoi_dan (a)/(b)).
//	                UNAVAILABLE / DEADLINE_EXCEEDED carry ErrIdentityUnavailable.
//	found == false  identity answered: no live row. An ordinary answer, not an error.
//	found == true   the row's id (Find) or its number (GetMatchPhone).
//
// THE COMMUNE TRAVELS IN METADATA through grpcx.UnaryClientInterceptor, like every call on this client;
// neither RPC is commune-exempt, so a context without one is refused locally (InvalidArgument).
//
// NOTHING HERE LOGS A REQUEST OR A RESPONSE (rule 3). Both FindCitizenContactByPhoneRequest and
// GetCitizenContactMatchPhoneResponse print a full number from their generated String(). The warnings
// carry the gRPC code and nothing else; no error built here quotes a number or a contact id.

// FindCitizenContactByPhone asks whether ONE number has a LIVE row in this commune's citizen directory,
// and that row's opaque id. phoneNumber is FULL PERSONAL DATA; the caller already holds it.
func (c *Client) FindCitizenContactByPhone(ctx context.Context, phoneNumber string) (found bool, contactID string, err error) {
	// REFUSED LOCALLY: identity answers INVALID_ARGUMENT for it, and an anonymous or number-less
	// petition must not call at all (ADR 0102 #4) — reaching here with nothing is the caller's wiring.
	if strings.TrimSpace(phoneNumber) == "" {
		return false, "", fmt.Errorf("identityclient: FindCitizenContactByPhone không có số — " +
			"phiếu không có số (hoặc ẩn danh) thì bên gọi không được gọi")
	}

	ctx, cancel := context.WithTimeout(ctx, HanGoi)
	defer cancel()

	resp, err := c.cl.FindCitizenContactByPhone(ctx, &identityv1.FindCitizenContactByPhoneRequest{Phone: phoneNumber})
	if err != nil {
		c.log.WarnContext(ctx, "CẢNH BÁO: không đối chiếu được số người gửi với danh bạ người dân",
			"ma_loi", status.Code(err).String())
		return false, "", wrapCallError("FindCitizenContactByPhone", err, nil)
	}
	if !resp.GetFound() {
		return false, "", nil
	}
	if resp.GetContactId() == "" {
		// A CONTRACT FAULT, not "not found": the two ends disagree about what a match carries, and a
		// found-without-id would send the screen to a directory row nobody can open.
		c.log.WarnContext(ctx, "CẢNH BÁO HỢP ĐỒNG: FindCitizenContactByPhone trả found mà không có contact_id")
		return false, "", fmt.Errorf("identityclient: FindCitizenContactByPhone: found = true nhưng contact_id rỗng")
	}
	return true, resp.GetContactId(), nil
}

// GetCitizenContactMatchPhone turns ONE directory row id into its full normalised number — the match key
// service-petitions uses for its own query and then drops (ADR 0102 #2, #8). identity writes one audit
// entry per disclosing call, naming actorCode.
//
// actorCode IS THE STAFF BUSINESS CODE — authz.Principal.Ma of the request being served, never
// Principal.ID (rule 6, invariant 8). Empty is refused here: identity would refuse it anyway, and a
// disclosure attributed to nobody is the one state the trail exists to prevent.
//
// THE RETURNED NUMBER IS PERSONAL DATA HELD IN MEMORY FOR ONE QUERY: never logged, returned over HTTP,
// cached, stored or put in an event (the RPC's obligations — breaking one is ADR 0102 stop condition #2).
func (c *Client) GetCitizenContactMatchPhone(ctx context.Context, contactID, actorCode string) (found bool, matchPhone string, err error) {
	if strings.TrimSpace(contactID) == "" {
		return false, "", fmt.Errorf("identityclient: GetCitizenContactMatchPhone không có contact_id")
	}
	if strings.TrimSpace(actorCode) == "" {
		return false, "", fmt.Errorf("identityclient: GetCitizenContactMatchPhone không có mã cán bộ — " +
			"vết lộ số phải ghi tên người xem (luật 6 bất biến 8)")
	}

	ctx, cancel := context.WithTimeout(ctx, HanGoi)
	defer cancel()

	resp, err := c.cl.GetCitizenContactMatchPhone(ctx, &identityv1.GetCitizenContactMatchPhoneRequest{
		ContactId: contactID,
		ActorCode: actorCode,
	})
	if err != nil {
		c.log.WarnContext(ctx, "CẢNH BÁO: không lấy được khoá đối chiếu của dòng danh bạ người dân",
			"ma_loi", status.Code(err).String())
		return false, "", wrapCallError("GetCitizenContactMatchPhone", err, nil)
	}
	if !resp.GetFound() {
		return false, "", nil
	}
	// THE KEY MUST ALREADY BE IN THE NATIONAL FORM petitions stores (core/phone). Anything else is a
	// contract fault: compared as-is it would match nothing and show a person with no petitions — a
	// silent short list, the failure an error here turns into a visible 503. The value is not quoted.
	if !phone.IsNational(resp.GetPhone()) {
		c.log.WarnContext(ctx, "CẢNH BÁO HỢP ĐỒNG: GetCitizenContactMatchPhone trả số không ở dạng trong nước")
		return false, "", fmt.Errorf("identityclient: GetCitizenContactMatchPhone: số trả về không ở dạng chuẩn hoá trong nước")
	}
	return true, resp.GetPhone(), nil
}
