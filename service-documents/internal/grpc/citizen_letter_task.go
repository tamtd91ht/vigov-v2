package grpc

import (
	"context"
	"strings"
	"unicode/utf8"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	documentsv1 "github.com/vihat/vigov/core/gen/vigov/documents/v1"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-documents/internal/domain"
)

// MaxCitizenLetterIDLen mirrors the contract (ResolveCitizenLetterForTaskRequest.letter_id: "over 64
// characters is INVALID_ARGUMENT") and domain.MaxLetterIDLen, the length 0006 allows an id.
const MaxCitizenLetterIDLen = domain.MaxLetterIDLen

// ResolveCitizenLetterForTask answers whether a task may be created from ONE letter of the commune in
// "x-tenant-id", and what the task inherits (ADR 0085 A; contract and status codes: documents.proto).
//
// ORDER: id shape → commune present → read → decide. The id is not logged.
//
// WHAT CROSSES THE BOUNDARY IS CODES, IDS AND ONE INSTANT. The store returns the whole row (sender and
// summary included, because one scan serves every read); NOTHING personal is copied into the answer,
// and for a denunciation NOTHING AT ALL is — not the number, not the holder, not the deadline (C9,
// Luật Tố cáo 2018 Đ.8, ADR 0078 #4). Hence no audit entry: no personal data is disclosed (rule 6,
// invariant 7 covers full personal data).
func (s *Server) ResolveCitizenLetterForTask(ctx context.Context, req *documentsv1.ResolveCitizenLetterForTaskRequest) (
	*documentsv1.ResolveCitizenLetterForTaskResponse, error) {

	id := strings.TrimSpace(req.GetLetterId())
	if id == "" || utf8.RuneCountInString(id) > MaxCitizenLetterIDLen {
		return nil, status.Error(codes.InvalidArgument,
			"letter_id rỗng hoặc dài quá 64 ký tự — bên gọi phải gửi citizen_letter.id")
	}
	// The interceptor already refused a call without a commune; reaching here without one means it is
	// missing from the chain. Refused, never defaulted (rule 1, forbidden #1).
	if _, ok := tenant.From(ctx); !ok {
		s.d.Log.ErrorContext(ctx, "CẢNH BÁO: handler gRPC chạy mà không có xã trong context — "+
			"grpcx.UnaryServerInterceptor không nằm trong chuỗi", "rpc", "ResolveCitizenLetterForTask")
		return nil, status.Error(codes.Internal, "lỗi cấu hình máy chủ")
	}

	l, found, err := s.d.Letters.LiveByID(ctx, id)
	if err != nil {
		// The cause stays in THIS service's log; the response crosses a boundary (rule 3, forbidden
		// #3). The caller refuses the write on any error — never reads it as NOT_FOUND or ELIGIBLE.
		s.d.Log.ErrorContext(ctx, "documents/grpc: đọc đơn thư cho nhiệm vụ thất bại",
			"rpc", "ResolveCitizenLetterForTask", "err", err)
		return nil, status.Error(codes.Internal, "lỗi nội bộ, vui lòng thử lại")
	}
	if !found {
		// Unknown, soft-deleted and another commune's id: one answer (rule 1; rule 7, invariant 2).
		return &documentsv1.ResolveCitizenLetterForTaskResponse{
			Eligibility: documentsv1.CitizenLetterTaskEligibility_CITIZEN_LETTER_TASK_ELIGIBILITY_NOT_FOUND,
		}, nil
	}
	if l.Type.ProtectsIdentity() {
		return &documentsv1.ResolveCitizenLetterForTaskResponse{
			Eligibility: documentsv1.CitizenLetterTaskEligibility_CITIZEN_LETTER_TASK_ELIGIBILITY_DENUNCIATION,
		}, nil
	}

	src := &documentsv1.CitizenLetterTaskSource{
		LetterId:      l.ID,
		Number:        clampUint32(l.Number),
		Year:          clampUint32(l.Year),
		HoldingUnitId: l.HoldingUnitID,
		AssigneeCode:  l.AssigneeCode,
	}
	// UNSET = no deadline ("Không đặt hạn") — never a default (contract, current_due_at).
	if due := l.CurrentStageDueAt(); !due.IsZero() {
		src.CurrentDueAt = timestamppb.New(due)
	}
	return &documentsv1.ResolveCitizenLetterForTaskResponse{
		Eligibility: documentsv1.CitizenLetterTaskEligibility_CITIZEN_LETTER_TASK_ELIGIBILITY_ELIGIBLE,
		Letter:      src,
	}, nil
}
