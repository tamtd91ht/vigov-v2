package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// ResolveUnassignedHoldInstants — for each instant a unit began holding work with nobody assigned, the
// instant that hold reaches the commune's `sla.unassigned_hold_hours`. The full contract (anchor, why
// a sibling of ResolveEscalationInstants, NULL = do not report, the status table) is on the RPC in
// identity.proto.
//
// THIS FILE JOINS; IT DOES NOT COUNT — the same division as escalation.go: the row and its fallback
// are domain.DongTheoLinhVuc, the calendar is Server.readCalendar, and each instant is
// domain.TienGioLamViec counted FORWARD from the hold start (ADR 0007: one implementation of the count).

// MaxHoldStarts — 1 to 500 entries, checked on what the caller SENT, before duplicates collapse. The
// MaxMissedDeadlines bound, for the same reason: one server walk per entry, never a truncation.
const MaxHoldStarts = MaxMissedDeadlines

func (s *Server) ResolveUnassignedHoldInstants(ctx context.Context, req *identityv1.ResolveUnassignedHoldInstantsRequest) (
	*identityv1.ResolveUnassignedHoldInstantsResponse, error) {

	// CALLER FAULTS FIRST, before any read — so a malformed call is refused even for a field whose
	// threshold is NULL, and never becomes read load.
	kind, err := loaiViecTu(req.GetWorkKind())
	if err != nil {
		return nil, err
	}
	sent := req.GetHoldStartedAt()
	if len(sent) == 0 {
		return nil, status.Error(codes.InvalidArgument, "thiếu hold_started_at")
	}
	if len(sent) > MaxHoldStarts {
		return nil, status.Errorf(codes.InvalidArgument,
			"hold_started_at có %d mục, vượt trần %d — bên gọi phải chia lô", len(sent), MaxHoldStarts)
	}
	distinct := make([]*timestamppb.Timestamp, 0, len(sent))
	seen := make(map[instantKey]struct{}, len(sent))
	for _, d := range sent {
		if d == nil {
			return nil, status.Error(codes.InvalidArgument, "hold_started_at có mục trống")
		}
		if err := d.CheckValid(); err != nil {
			return nil, status.Error(codes.InvalidArgument, "hold_started_at có mục không phải một mốc thời gian hợp lệ")
		}
		k := instantKey{d.GetSeconds(), d.GetNanos()}
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		distinct = append(distinct, d)
	}

	xa, err := s.xa(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := s.d.SLA.DanhSach(ctx)
	if err != nil {
		return nil, s.loi(ctx, err, "ResolveUnassignedHoldInstants/sla")
	}
	row, ok := domain.DongTheoLinhVuc(rows, kind, req.GetLinhVuc())
	if !ok {
		return nil, s.loiThieuCauHinhSLA(ctx, rows, kind, xa, "ResolveUnassignedHoldInstants")
	}

	// NULL (0 in Go — see domain.DongSLA.UnassignedHoldHours) IS THE COMMUNE'S ANSWER "DO NOT REPORT",
	// read from THIS row: the fallback above is by row, never by column, so a field row holding NULL
	// stays NULL even when the default row holds a number. Answered BEFORE the calendar is read — there
	// is nothing to count, and a calendar fault must not turn "disabled" into a failed run.
	hours := row.UnassignedHoldHours
	if hours == 0 {
		return &identityv1.ResolveUnassignedHoldInstantsResponse{ReportingDisabled: true}, nil
	}
	// A NEGATIVE OR ABSURD VALUE came from the commune's table (the schema refuses <= 0, the write path
	// bounds the rest): FAILED_PRECONDITION, the bounds ResolveEscalationInstants applies — never a
	// clamp to some number nobody chose.
	if hours < 0 || hours > TranGioMotMoc {
		s.d.Log.WarnContext(ctx, "ResolveUnassignedHoldInstants: ngưỡng giữ việc chưa giao của xã không dùng được — TỪ CHỐI",
			"xa", string(xa), "dong_sla", row.ID, "so_gio", hours)
		return nil, status.Errorf(codes.FailedPrecondition,
			"cấu hình thời hạn xử lý của xã có ngưỡng giữ việc chưa giao %d giờ, ngoài khoảng 1–%d — sửa dòng SLA", hours, TranGioMotMoc)
	}

	week, readYear, err := s.readCalendar(ctx, xa, "ResolveUnassignedHoldInstants")
	if err != nil {
		return nil, err
	}
	// One read per year for the whole call, scoped to this request — the escalation.go memo, for the
	// same reason (up to 500 walks share one commune's calendar). Not a cache.
	yearMemo := map[int]yearRead{}
	memoYear := func(year int) ([]domain.NgayNghiLe, []domain.CaLamBu, error) {
		if r, ok := yearMemo[year]; ok {
			return r.closures, r.swaps, r.err
		}
		c, sw, err := readYear(year)
		yearMemo[year] = yearRead{c, sw, err}
		return c, sw, err
	}

	items := make([]*identityv1.UnassignedHoldInstant, 0, len(distinct))
	for _, d := range distinct {
		reached, err := domain.TienGioLamViec(d.AsTime(), week, memoYear, []int{hours})
		if err != nil {
			return nil, s.loiLich(ctx, err, xa, "ResolveUnassignedHoldInstants/tinh")
		}
		var due *timestamppb.Timestamp
		for _, m := range reached {
			if m.Gio == hours {
				due = timestamppb.New(m.DatLuc)
			}
		}
		if due == nil {
			// UNREACHABLE unless the walk stops answering the amount it was given; checked because an
			// unset instant reaches the caller as the epoch — "report every hold now".
			s.d.Log.ErrorContext(ctx, "CẢNH BÁO HỢP ĐỒNG: phép tiến giờ không trả mốc giữ việc chưa giao", "xa", string(xa))
			return nil, status.Error(codes.Internal, "lỗi nội bộ, vui lòng thử lại")
		}
		items = append(items, &identityv1.UnassignedHoldInstant{HoldStartedAt: d, UnassignedReportDueAt: due})
	}
	return &identityv1.ResolveUnassignedHoldInstantsResponse{Items: items}, nil
}
