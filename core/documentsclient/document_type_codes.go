package documentsclient

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc/status"

	documentsv1 "github.com/vihat/vigov/core/gen/vigov/documents/v1"
)

// MaxDocumentTypeCodesPerCall mirrors the ceiling of ResolveDocumentTypeCodesRequest
// (proto/vigov/documents/v1/documents.proto, the source of truth — rule 2, invariant 7). REFUSED HERE
// RATHER THAN SENT, AND NEVER TRUNCATED: a clamped batch answers real codes as "absent", and the
// caller would refuse them as unknown. The caller chunks.
const MaxDocumentTypeCodesPerCall = 50

// DocumentTypeCodes asks documents which of the given document-type codes (`loai_van_ban.ma`) a LIVE
// type of the commune in ctx carries. The map holds one entry per ANSWERED code, its value the type's
// `dang_dung`. The predicate is stated once, on the RPC in documents.proto.
//
// # READING THE ANSWER — fail closed
//
//	present, true     the commune issues documents under this type.
//	present, false    the type exists but is switched off.
//	absent            no live type of this commune carries the code — unknown, soft-deleted and another
//	                  commune's code are one answer.
//	error             the call did not happen (retryable when errors.Is(err, ErrDocumentsUnavailable)).
//	                  NEVER "valid", never "unknown".
//
// Codes are sent EXACTLY as given — the server matches exactly, and the caller stores what it sent. A
// blank code is refused here (the server would answer INVALID_ARGUMENT); duplicates are sent once. An
// empty list answers an empty map without a round trip. Not cached.
func (c *Client) DocumentTypeCodes(ctx context.Context, codes []string) (map[string]bool, error) {
	if len(codes) > MaxDocumentTypeCodesPerCall {
		return nil, fmt.Errorf(
			"documentsclient: DocumentTypeCodes nhận %d mã, vượt trần %d — bên gọi phải tự chia lô, không được cắt bớt",
			len(codes), MaxDocumentTypeCodesPerCall)
	}
	asked := make(map[string]struct{}, len(codes))
	sent := make([]string, 0, len(codes))
	for _, code := range codes {
		if strings.TrimSpace(code) == "" {
			return nil, fmt.Errorf("documentsclient: DocumentTypeCodes với mã rỗng — bên gọi phải chuẩn hoá trước khi hỏi")
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

	resp, err := c.cl.ResolveDocumentTypeCodes(ctx, &documentsv1.ResolveDocumentTypeCodesRequest{Codes: sent})
	if err != nil {
		c.log.WarnContext(ctx, "CẢNH BÁO: không tra được mã loại văn bản ở documents",
			"ma_loi", status.Code(err).String(), "so_ma", len(sent), "err", err)
		return nil, wrapCallError("ResolveDocumentTypeCodes", err)
	}

	out := make(map[string]bool, len(sent))
	for _, it := range resp.GetItems() {
		code, active := it.GetCode(), it.GetActive()
		if _, ok := asked[code]; !ok {
			// The caller writes from this map: a key it did not ask for is refused, not ignored.
			return nil, fmt.Errorf(
				"documentsclient: ResolveDocumentTypeCodes trả về một mã KHÔNG ĐƯỢC HỎI — phản hồi phải là tập con của yêu cầu")
		}
		if prev, dup := out[code]; dup && prev != active {
			// `(tenant_id, ma)` is unique, so one code is one type. Two different answers means the far
			// end is wrong, and picking either decides a write on a guess.
			return nil, fmt.Errorf("documentsclient: ResolveDocumentTypeCodes trả hai trạng thái khác nhau cho cùng một mã")
		}
		out[code] = active
	}
	// NO COMPLETENESS CHECK: an unanswered code is the answer "no live type carries it".
	return out, nil
}
