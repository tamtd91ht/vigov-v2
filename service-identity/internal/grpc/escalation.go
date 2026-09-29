package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// ResolveEscalationInstants — for each deadline a record has missed, the two instants at which its
// lateness reaches the commune's escalation thresholds. The full contract (anchor, why instants and
// not hours, the status table) is on the RPC in identity.proto.
//
// THIS FILE JOINS; IT DOES NOT COUNT. The row lookup and its fallback are domain.DongTheoLinhVuc, the
// calendar read is Server.readCalendar, and each instant is domain.TienGioLamViec counted FORWARD from
// the missed deadline — the one walk every deadline in this system uses (ADR 0007: no second
// implementation of the working-hours count).

// MaxMissedDeadlines — 1 to 500 entries, checked on what the caller SENT, before duplicates collapse.
// Never a truncation: a truncated answer leaves late records un-escalated with nothing reporting it.
const MaxMissedDeadlines = 500

// instantKey compares two timestamps AS INSTANTS (seconds and nanos), which is how the contract tells
// the caller to map the response.
type instantKey struct {
	s int64
	n int32
}

func (s *Server) ResolveEscalationInstants(ctx context.Context, req *identityv1.ResolveEscalationInstantsRequest) (
	*identityv1.ResolveEscalationInstantsResponse, error) {

	kind, err := loaiViecTu(req.GetWorkKind())
	if err != nil {
		return nil, err
	}
	sent := req.GetMissedDeadlines()
	if len(sent) == 0 {
		return nil, status.Error(codes.InvalidArgument, "thiếu missed_deadlines")
	}
	if len(sent) > MaxMissedDeadlines {
		return nil, status.Errorf(codes.InvalidArgument,
			"missed_deadlines có %d mục, vượt trần %d — bên gọi phải chia lô", len(sent), MaxMissedDeadlines)
	}
	distinct := make([]*timestamppb.Timestamp, 0, len(sent))
	seen := make(map[instantKey]struct{}, len(sent))
	for _, d := range sent {
		if d == nil {
			return nil, status.Error(codes.InvalidArgument, "missed_deadlines có mục trống")
		}
		if err := d.CheckValid(); err != nil {
			return nil, status.Error(codes.InvalidArgument, "missed_deadlines có mục không phải một mốc thời gian hợp lệ")
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
		return nil, s.loi(ctx, err, "ResolveEscalationInstants/sla")
	}
	row, ok := domain.DongTheoLinhVuc(rows, kind, req.GetLinhVuc())
	if !ok {
		return nil, s.loiThieuCauHinhSLA(ctx, rows, kind, xa, "ResolveEscalationInstants")
	}

	// THE COMMUNE'S TWO NUMBERS ARE CHECKED BEFORE THE ARITHMETIC, and a bad one is
	// FAILED_PRECONDITION because it came from the commune's table — the bounds ResolveDeadlines
	// applies. NOT a check that Y >= X: the write path enforces that (domain.KiemTraDongSLA), and this
	// read evaluates the two levels independently, as the contract tells the caller to.
	unitHead, chairman := row.GioBaoLanhDao, row.GioBaoChuTich
	for _, h := range []int{unitHead, chairman} {
		if h <= 0 || h > TranGioMotMoc {
			s.d.Log.WarnContext(ctx, "ResolveEscalationInstants: ngưỡng leo thang của xã không dùng được — TỪ CHỐI",
				"xa", string(xa), "dong_sla", row.ID, "so_gio", h)
			return nil, status.Errorf(codes.FailedPrecondition,
				"cấu hình thời hạn xử lý của xã có ngưỡng leo thang %d giờ, ngoài khoảng 1–%d — sửa dòng SLA", h, TranGioMotMoc)
		}
	}

	week, readYear, err := s.readCalendar(ctx, xa, "ResolveEscalationInstants")
	if err != nil {
		return nil, err
	}
	// ONE READ PER YEAR FOR THE WHOLE CALL, NOT PER DEADLINE: up to 500 walks share this commune's
	// calendar, and each TienGioLamViec keeps its own per-call memo. Scoped to this request and dropped
	// with it — not a cache (ADR 0007 decision 6, and the note on lichTheoNam).
	yearMemo := map[int]yearRead{}
	memoYear := func(year int) ([]domain.NgayNghiLe, []domain.CaLamBu, error) {
		if r, ok := yearMemo[year]; ok {
			return r.closures, r.swaps, r.err
		}
		c, sw, err := readYear(year)
		yearMemo[year] = yearRead{c, sw, err}
		return c, sw, err
	}

	items := make([]*identityv1.EscalationInstants, 0, len(distinct))
	for _, d := range distinct {
		reached, err := domain.TienGioLamViec(d.AsTime(), week, memoYear, []int{unitHead, chairman})
		if err != nil {
			return nil, s.loiLich(ctx, err, xa, "ResolveEscalationInstants/tinh")
		}
		at := make(map[int]*timestamppb.Timestamp, len(reached))
		for _, m := range reached {
			at[m.Gio] = timestamppb.New(m.DatLuc)
		}
		u, c := at[unitHead], at[chairman]
		if u == nil || c == nil {
			// UNREACHABLE unless the walk stops answering every amount it was given; checked because
			// an unset instant reaches the caller as year 1 — "escalate now" for every record.
			s.d.Log.ErrorContext(ctx, "CẢNH BÁO HỢP ĐỒNG: phép tiến giờ không trả đủ mốc leo thang", "xa", string(xa))
			return nil, status.Error(codes.Internal, "lỗi nội bộ, vui lòng thử lại")
		}
		items = append(items, &identityv1.EscalationInstants{
			MissedDeadline: d,
			UnitHeadDueAt:  u,
			ChairmanDueAt:  c,
		})
	}
	return &identityv1.ResolveEscalationInstantsResponse{Items: items}, nil
}

type yearRead struct {
	closures []domain.NgayNghiLe
	swaps    []domain.CaLamBu
	err      error
}
