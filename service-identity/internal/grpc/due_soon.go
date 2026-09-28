package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// ResolveDueSoonCutoff — the server side of the "Sắp đến hạn" filter. The full contract, including
// why it returns an instant and never `gio_sap_den_han` as hours, is on the RPC in identity.proto.
//
// THIS FILE JOINS; IT DOES NOT COUNT AND IT DOES NOT DECIDE — the same shape as sla.go. The row lookup
// and its fallback are domain.DongTheoLinhVuc (ResolveDeadlines' own), the calendar read is
// Server.readCalendar (the one AdvanceWorkingHours and ResolveDeadlines use), and the walk is
// domain.DueSoonCutoff over the same workingWalk as every deadline.
//
// ORDER: caller faults first, before any store read → commune present → `sla` → the threshold's own
// sanity → the calendar. A commune with no usable row is FAILED_PRECONDITION with the same wording
// ResolveDeadlines uses, because it is the same hole on the same screen. There is NO number below the
// default row: not 72, not the specification's example.
func (s *Server) ResolveDueSoonCutoff(ctx context.Context, req *identityv1.ResolveDueSoonCutoffRequest) (
	*identityv1.ResolveDueSoonCutoffResponse, error) {

	kind, err := loaiViecTu(req.GetWorkKind())
	if err != nil {
		return nil, err
	}

	asOf := req.GetAsOf()
	if asOf == nil {
		// NO SERVER-SIDE "NOW": the caller compares the answer with the same `now` it uses for its
		// `late` filter, and two clocks one request apart put a record on neither list or on both.
		return nil, status.Error(codes.InvalidArgument, "thiếu as_of")
	}
	if err := asOf.CheckValid(); err != nil {
		return nil, status.Error(codes.InvalidArgument, "as_of không phải một mốc thời gian hợp lệ")
	}

	xa, err := s.xa(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := s.d.SLA.DanhSach(ctx)
	if err != nil {
		return nil, s.loi(ctx, err, "ResolveDueSoonCutoff/sla")
	}
	row, ok := domain.DongTheoLinhVuc(rows, kind, req.GetLinhVuc())
	if !ok {
		return nil, s.loiThieuCauHinhSLA(ctx, rows, kind, xa, "ResolveDueSoonCutoff")
	}

	// THE COMMUNE'S NUMBER IS CHECKED BEFORE THE ARITHMETIC, and it is FAILED_PRECONDITION because it
	// came from the commune's table — the same two bounds ResolveDeadlines applies to its columns.
	hours := row.GioSapDenHan
	switch {
	case hours <= 0:
		s.d.Log.WarnContext(ctx, "ResolveDueSoonCutoff: ngưỡng sắp đến hạn của xã không dương — TỪ CHỐI",
			"xa", string(xa), "dong_sla", row.ID, "so_gio", hours)
		return nil, status.Error(codes.FailedPrecondition,
			"cấu hình thời hạn xử lý của xã có ngưỡng sắp đến hạn không dương — sửa dòng SLA")
	case hours > TranGioMotMoc:
		s.d.Log.WarnContext(ctx, "ResolveDueSoonCutoff: ngưỡng sắp đến hạn của xã vượt trần — TỪ CHỐI",
			"xa", string(xa), "dong_sla", row.ID, "so_gio", hours)
		return nil, status.Errorf(codes.FailedPrecondition,
			"cấu hình thời hạn xử lý của xã đặt ngưỡng sắp đến hạn %d giờ, vượt trần %d giờ", hours, TranGioMotMoc)
	}

	week, readYear, err := s.readCalendar(ctx, xa, "ResolveDueSoonCutoff")
	if err != nil {
		return nil, err
	}
	cutoff, err := domain.DueSoonCutoff(asOf.AsTime(), week, readYear, hours)
	if err != nil {
		return nil, s.loiLich(ctx, err, xa, "ResolveDueSoonCutoff/tinh")
	}
	return &identityv1.ResolveDueSoonCutoffResponse{DueSoonUntil: timestamppb.New(cutoff)}, nil
}
