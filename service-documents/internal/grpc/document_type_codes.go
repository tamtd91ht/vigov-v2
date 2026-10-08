package grpc

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	documentsv1 "github.com/vihat/vigov/core/gen/vigov/documents/v1"
	"github.com/vihat/vigov/core/tenant"
)

// MaxDocumentTypeCodesPerCall is the ceiling on ONE ResolveDocumentTypeCodes request. It mirrors the
// contract (documents.proto, ResolveDocumentTypeCodesRequest), the source of truth (rule 2,
// invariant 7). Over it is INVALID_ARGUMENT — never a silent truncation, which would answer real
// codes as "absent" and have the caller refuse them as unknown.
const MaxDocumentTypeCodesPerCall = 50

// ResolveDocumentTypeCodes answers which of the asked codes a live document type of the commune in
// "x-tenant-id" carries, and whether it is switched on. Contract and status codes: documents.proto.
//
// ORDER: ceiling (on what was SENT, before de-duplication) → no blank code → commune present (the
// store panics without one) → empty answers empty without a read → read. Codes are logged by count
// only; they are not personal data, but the log has no use for them.
//
// THE ANSWER IS A SUBSET OF THE QUESTION, EACH CODE ONCE — enforced here rather than trusted to the
// SQL, because the contract promises it at this boundary.
func (s *Server) ResolveDocumentTypeCodes(ctx context.Context, req *documentsv1.ResolveDocumentTypeCodesRequest) (
	*documentsv1.ResolveDocumentTypeCodesResponse, error) {

	sent := req.GetCodes()
	if len(sent) > MaxDocumentTypeCodesPerCall {
		return nil, status.Errorf(codes.InvalidArgument,
			"codes vượt trần %d cho một lời gọi — bên gọi phải tự chia lô", MaxDocumentTypeCodesPerCall)
	}
	for _, c := range sent {
		// Blank, not "empty": a whitespace-only code can match no `ma` either, and asking for it is the
		// same caller bug. A non-blank code is NOT trimmed — the match is exact (contract).
		if strings.TrimSpace(c) == "" {
			return nil, status.Error(codes.InvalidArgument, "codes có phần tử rỗng — bên gọi phải bỏ trước khi hỏi")
		}
	}
	if _, ok := tenant.From(ctx); !ok {
		s.d.Log.ErrorContext(ctx, "CẢNH BÁO: handler gRPC chạy mà không có xã trong context — "+
			"grpcx.UnaryServerInterceptor không nằm trong chuỗi", "rpc", "ResolveDocumentTypeCodes")
		return nil, status.Error(codes.Internal, "lỗi cấu hình máy chủ")
	}

	pending := make(map[string]struct{}, len(sent))
	asked := make([]string, 0, len(sent))
	for _, c := range sent {
		if _, dup := pending[c]; dup {
			continue
		}
		pending[c] = struct{}{}
		asked = append(asked, c)
	}
	if len(asked) == 0 {
		return &documentsv1.ResolveDocumentTypeCodesResponse{}, nil
	}

	rows, err := s.d.DocumentTypes.StatesByCode(ctx, asked)
	if err != nil {
		// The cause stays in THIS service's log; the response crosses a boundary (rule 3, forbidden
		// #3). The caller refuses the write on any error.
		s.d.Log.ErrorContext(ctx, "documents/grpc: tra mã loại văn bản thất bại",
			"rpc", "ResolveDocumentTypeCodes", "so_ma", len(asked), "err", err)
		return nil, status.Error(codes.Internal, "lỗi nội bộ, vui lòng thử lại")
	}

	out := make([]*documentsv1.DocumentTypeCode, 0, len(rows))
	for _, r := range rows {
		if _, ok := pending[r.Code]; !ok {
			continue
		}
		delete(pending, r.Code)
		out = append(out, &documentsv1.DocumentTypeCode{Code: r.Code, Active: r.Active})
	}
	return &documentsv1.ResolveDocumentTypeCodesResponse{Items: out}, nil
}
