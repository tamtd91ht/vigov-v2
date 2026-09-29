package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	platformv1 "github.com/vihat/vigov/core/gen/vigov/platform/v1"
)

// ListPetitionFields returns every tier-1 petition field code, retired ones included (ADR 0026,
// ADR 0060). The status table is on the RPC in platform.proto; this follows it.
//
// NOT EXEMPT from "x-tenant-id": the interceptor has already refused a call without one. Nothing
// below reads it, because tier 1 is one set for every commune — a commune's own switch, order and
// wording are tier 2, in service-petitions. A branch on the commune here would be building a
// per-commune tier 1, which ADR 0060 stop condition #3 forbids.
//
// Not audited: a read of platform-wide configuration (platform.proto, ADR 0025). NOT_FOUND is never
// produced — an empty set is an OK with no entries.
func (s *Server) ListPetitionFields(ctx context.Context, _ *platformv1.ListPetitionFieldsRequest) (
	*platformv1.ListPetitionFieldsResponse, error) {

	rows, err := s.fields.ListCitizenReportFields(ctx)
	if err != nil {
		s.log.ErrorContext(ctx, "đọc bộ mã lĩnh vực phản ánh thất bại", "rpc", "ListPetitionFields", "err", err)
		// Internal, never an empty OK: an empty set means "no code is valid", and an outage that
		// read that way would refuse every intake with nobody paged.
		return nil, status.Error(codes.Internal, "lỗi nội bộ, vui lòng thử lại")
	}

	out := &platformv1.ListPetitionFieldsResponse{Fields: make([]*platformv1.CitizenReportField, 0, len(rows))}
	for _, r := range rows {
		// Field by field, like toProto — ADR 0003's boundary: nothing leaves this service because it
		// happened to be on the struct.
		out.Fields = append(out.Fields, &platformv1.CitizenReportField{
			Code:         r.Code,
			DefaultLabel: r.DefaultLabel,
			SortOrder:    r.SortOrder,
			Icon:         r.Icon,
			Tone:         r.Tone,
			Active:       r.IsActive,
		})
	}
	return out, nil
}
