package grpc

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	platformv1 "github.com/vihat/vigov/core/gen/vigov/platform/v1"
)

// ListUploadPolicies returns every live upload policy (ADR 0052 §10). The status table is on the
// RPC in platform.proto; this follows it.
//
// NOT EXEMPT from "x-tenant-id": the interceptor has already refused a call without one. The answer
// does not depend on the commune TODAY — every commune gets the platform-wide policy, and a
// per-commune override is open — so nothing below reads it. There is deliberately no branch on the
// commune here: writing one would be deciding the open question.
//
// Not audited: a read of platform-wide configuration with no personal data and no caller identity on
// this port (platform.proto, ADR 0025). NOT_FOUND is never produced — "not configured" is an absent
// entry.
func (s *Server) ListUploadPolicies(ctx context.Context, _ *platformv1.ListUploadPoliciesRequest) (
	*platformv1.ListUploadPoliciesResponse, error) {

	rows, err := s.policies.ListUploadPolicies(ctx)
	if err != nil {
		s.log.ErrorContext(ctx, "đọc giới hạn tải lên thất bại", "rpc", "ListUploadPolicies", "err", err)
		return nil, status.Error(codes.Internal, "lỗi nội bộ, vui lòng thử lại")
	}

	out := &platformv1.ListUploadPoliciesResponse{}
	for _, r := range rows {
		purpose, ok := purposeToProto(r.Purpose)
		if !ok {
			// A row the enum cannot name — the CHECK on the column should make this unreachable, so
			// reaching it means the schema and this build disagree. Skipped, which callers read as
			// "not configured" and refuse: the safe direction. The purpose is configuration, not
			// personal data, so it may be named.
			s.log.WarnContext(ctx, "giới hạn tải lên có mục đích không có trong enum, bỏ qua",
				"rpc", "ListUploadPolicies", "purpose", r.Purpose)
			continue
		}
		// Field by field, like toProto — ADR 0003's boundary.
		p := &platformv1.UploadPolicy{
			Purpose:          purpose,
			MaxBytes:         r.MaxBytes,
			AllowedMimeTypes: append([]string(nil), r.AllowedMIMETypes...),
			UpdatedAt:        timestamppb.New(r.UpdatedAt),
		}
		// PRESENCE IS THE SIGNAL (platform.proto): absent = Vihat set no count limit.
		if r.FileCountLimited {
			p.MaxFilesPerSubject = proto.Int32(r.MaxFilesPerSubject)
		}
		out.Policies = append(out.Policies, p)
	}
	return out, nil
}

// purposeToProto maps a stored purpose to the enum BY THE RULE platform.proto states — strip
// "UPLOAD_PURPOSE_", lowercase, "_" -> "-" — applied in reverse. No mapping table: a table would be
// a third copy of the list, and the third copy is the one that drifts.
//
// The reverse direction alone would accept "Content-Video" or "content_video" (upper-casing folds
// both onto a real name), so the enum name found is derived forward again and must reproduce the
// stored string exactly.
func purposeToProto(stored string) (platformv1.UploadPurpose, bool) {
	name := "UPLOAD_PURPOSE_" + strings.ToUpper(strings.ReplaceAll(stored, "-", "_"))
	v, ok := platformv1.UploadPurpose_value[name]
	if !ok || v == int32(platformv1.UploadPurpose_UPLOAD_PURPOSE_UNSPECIFIED) {
		return 0, false
	}
	if purposeFromEnumName(name) != stored {
		return 0, false
	}
	return platformv1.UploadPurpose(v), true
}

// purposeFromEnumName is the forward derivation, stated once for purposeToProto and its test.
func purposeFromEnumName(name string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimPrefix(name, "UPLOAD_PURPOSE_")), "_", "-")
}
