package grpc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// AdvanceWorkingHours — the server side of ADR 0007's deadline arithmetic.
//
// THIS FILE TRANSLATES; IT DOES NOT COUNT. The walk, every refusal and the zone live in
// domain.TienGioLamViec, so the rule a citizen's commitment is computed from is in one place and
// not in a transport handler. What is here is the wire contract: which faults are the CALLER's,
// which are the COMMUNE'S CONFIGURATION, and which are this deployment's.

// The three ceilings on the request, declared where they are enforced.
//
// NEITHER IS A CLAMP. A truncated batch stores an administrative record with one of its two
// commitments missing, and there is no cursor to carry on from — the same argument
// BatchGetStaffRequest makes, landing this time on a figure told to a citizen.
const (
	// TranSoMocMotLo — at most 20 amounts per call. ADR 0007's table needs TWO per record
	// (`Tiếp nhận` and `Xử lý xong`), so this is an order of magnitude of headroom.
	TranSoMocMotLo = 20

	// TranGioMotMoc — at most 2 000 working hours per amount, and the code for exceeding it is
	// INVALID_ARGUMENT rather than FAILED_PRECONDITION on purpose. An ordinary working year is
	// 8 × 5 × 52 = 2 080 hours, so nothing above this can be delivered inside the horizon by any
	// week a commune actually works: it is a fault in the CALLER by construction, and answering it
	// with the configuration code would send an operator to read a calendar that is perfectly
	// correct. ADR 0007's largest real value is 168.
	TranGioMotMoc = 2000
)

// The calendar collaborators, declared HERE at the point of use — the same discipline as the four
// interfaces on server.go, and for the same reason: every branch below is checkable without a
// PostgreSQL. *idstore.LichLamViecStore, *idstore.NgayNghiLeStore and *idstore.NgayLamBuStore
// satisfy these as they are; nothing was changed to accommodate them.
//
// THREE NARROW INTERFACES AND NOT ONE WIDE ONE, mirroring internal/http's split: three tables,
// three reads, three different windows (the week is read whole, the other two by year). A single
// interface would invite a fourth method that reads a commune's calendar some other way.
type (
	// LichLamViecDoc reads the commune's ordinary week, WHOLE. A calendar is only correct whole:
	// a page of it is a week with sessions missing, and nothing downstream can tell that apart
	// from a commune that genuinely does not work those hours.
	LichLamViecDoc interface {
		DanhSach(ctx context.Context) ([]domain.CaLamViec, error)
	}

	// NgayNghiLeDoc reads one calendar year of closures. Its statement is also what DETECTS a date
	// that is both a closure and a swap day, and refuses it.
	NgayNghiLeDoc interface {
		TheoNam(ctx context.Context, nam int) ([]domain.NgayNghiLe, error)
	}

	// NgayLamBuDoc reads one calendar year of swap-day sessions, with the mirror half of the same
	// conflict detection.
	NgayLamBuDoc interface {
		TheoNam(ctx context.Context, nam int) ([]domain.CaLamBu, error)
	}
)

// AdvanceWorkingHours answers, for every requested amount of working hours, the instant this
// commune reaches it — counted from `count_from` through this commune's own calendar.
//
// # Nothing is written, nothing is audited, and retrying is safe
//
// This is a READ of configuration, not a business act (rule 6, invariant 1 covers writes). The
// audited act is the INTAKE, written by the caller inside its own transaction. A retry after a
// failure costs nothing and changes nothing.
//
// # There is no partial success
//
// Every entry in one call is counted against ONE calendar for ONE commune, so anything that stops
// one entry stops every entry. A response holding one of two amounts would let a petition be
// stored carrying its `Tiếp nhận` commitment and no `Xử lý xong` commitment — a half-processed
// administrative file of exactly the kind rule 2, invariant 6 forbids.
//
// # No cache, and here a cache would be actively WRONG rather than merely unnecessary
//
// ADR 0007 decision 6 fixes that a configuration change is NOT retroactive: it applies to records
// received AFTER it. A cached calendar inverts exactly that — a record received after the commune
// corrected its holidays would get a deadline computed from the calendar as it was before, on
// whichever process still held the stale copy. There is nothing hot to cache anyway: rule 10,
// invariant 2 makes this ONE hop per received record, not one per request and not one per row.
//
// # The status codes are the contract
//
//	INVALID_ARGUMENT     the caller is wrong — no `count_from`, no amounts, a zero, a value over a
//	                     ceiling, or more amounts than allowed.
//	FAILED_PRECONDITION  the COMMUNE'S CONFIGURATION is wrong or missing. The distinction from
//	                     INVALID_ARGUMENT is not cosmetic: it is the difference between "fix the
//	                     calling service" and "open the configuration screen".
//	Internal             a store failure or this deployment's wiring. NEVER a business answer, and
//	                     never an empty OK — a caller that reads an outage as "no deadline" stores
//	                     an administrative record with no commitment attached to it.
func (s *Server) AdvanceWorkingHours(ctx context.Context, req *identityv1.AdvanceWorkingHoursRequest) (
	*identityv1.AdvanceWorkingHoursResponse, error) {

	tuMoc := req.GetCountFrom()
	if tuMoc == nil {
		// NO "COUNT FROM NOW" DEFAULT. A default here makes the deadline depend on when the call
		// happened to be made rather than on when the authority received the file — the same class
		// of mistake as a default on the isolation path, landing on a citizen-facing commitment.
		return nil, status.Error(codes.InvalidArgument, "thiếu count_from")
	}
	if err := tuMoc.CheckValid(); err != nil {
		return nil, status.Error(codes.InvalidArgument, "count_from không phải một mốc thời gian hợp lệ")
	}

	gio := req.GetWorkingHours()
	if len(gio) == 0 {
		return nil, status.Error(codes.InvalidArgument, "thiếu working_hours")
	}
	// THE CEILING IS CHECKED ON WHAT THE CALLER SENT, before duplicates are collapsed: the point is
	// to bound the request, and a caller sending 500 amounts of which 2 are distinct is still a
	// caller with no ceiling of its own.
	if len(gio) > TranSoMocMotLo {
		return nil, status.Errorf(codes.InvalidArgument,
			"working_hours vượt trần %d mốc cho một lời gọi", TranSoMocMotLo)
	}
	for _, g := range gio {
		if g == 0 {
			// THE CHEAP ONE TO GET WRONG. An SLA that was never configured arrives here as a zero,
			// and a zero answered politely returns `count_from` itself — a record due at the
			// instant it was received, which every report downstream reads as "already late".
			return nil, status.Error(codes.InvalidArgument,
				"working_hours có mốc 0 giờ — SLA chưa cấu hình phải hỏng ở bên gọi, không trả hạn bằng chính mốc tiếp nhận")
		}
		if g > TranGioMotMoc {
			return nil, status.Errorf(codes.InvalidArgument,
				"working_hours có mốc %d giờ, vượt trần %d giờ cho một mốc", g, TranGioMotMoc)
		}
	}

	// The commune is not used as a value below — every read is scoped to it from the context by
	// store.Scoped. This call is the guard that the commune interceptor is in the chain at all: it
	// closes the panic path through store.Scoped's MustFrom, which a gRPC handler has no recovery
	// for. The value is kept for the log lines, where "which commune's calendar" is the first
	// question an operator asks.
	xa, err := s.xa(ctx)
	if err != nil {
		return nil, err
	}

	tuan, err := s.d.Lich.DanhSach(ctx)
	if err != nil {
		// Including ErrQuaNhieuCaLamViec: a calendar over its ceiling has stopped being a working
		// week (an import run twice, a fixture on a live database), and the store's own note says
		// the caller answers 500 — the HTTP route does exactly that. It is not one of the four
		// configuration faults the contract names.
		return nil, s.loiLich(ctx, err, xa, "AdvanceWorkingHours/lich_lam_viec")
	}

	// LAZY, ONE YEAR AT A TIME, AND ONLY FOR A YEAR THE WALK ENTERS. A 16-hour deadline in March
	// reads one year of closures and one of swap days; it is not charged for the year after, and a
	// configuration fault in a year the count never reaches cannot change this answer.
	docNam := func(nam int) ([]domain.NgayNghiLe, []domain.CaLamBu, error) {
		nghi, err := s.d.NghiLe.TheoNam(ctx, nam)
		if err != nil {
			return nil, nil, err
		}
		bu, err := s.d.LamBu.TheoNam(ctx, nam)
		if err != nil {
			return nil, nil, err
		}
		return nghi, bu, nil
	}

	dat, err := domain.TienGioLamViec(tuMoc.AsTime(), tuan, docNam, sangGio(gio))
	if err != nil {
		return nil, s.loiLich(ctx, err, xa, "AdvanceWorkingHours/tinh")
	}

	// ONE ITEM PER DISTINCT AMOUNT, each echoing its own `working_hours`: duplicates collapse, so
	// len(items) can be smaller than len(working_hours) and an index is never a key.
	items := make([]*identityv1.WorkingHoursReached, 0, len(dat))
	for _, m := range dat {
		items = append(items, &identityv1.WorkingHoursReached{
			WorkingHours: uint32(m.Gio),
			ReachedAt:    timestamppb.New(m.DatLuc),
		})
	}
	return &identityv1.AdvanceWorkingHoursResponse{Items: items}, nil
}

// loiLich maps a calendar failure to a gRPC code, and the mapping is the part of this file that is
// expensive to get wrong.
//
// TWO OF THE THREE CODES ARE EASY TO CONFUSE, so the rule is stated rather than inferred:
//
//	FAILED_PRECONDITION  a state a HUMAN must fix on the configuration screen. The message NAMES
//	                     what to fix — a weekday, a date, a row id. None of that is personal data
//	                     (rule 3): it says when an authority is open, and a refusal nobody can act
//	                     on is a refusal that gets ignored.
//	Internal             the store failed, or this process is misconfigured. The cause goes to THIS
//	                     service's log, where the operator is, and never into a response that
//	                     crosses into another process's logs (rule 3, forbidden #3).
//
// Turning either into the other costs an afternoon: an operator sent to a calendar that is fine,
// or a commune's broken configuration reported as an outage of identity.
func (s *Server) loiLich(ctx context.Context, err error, xa tenant.ID, cho string) error {
	var cauHinh *domain.LoiKhongTinhDuocHan
	if errors.As(err, &cauHinh) {
		s.d.Log.WarnContext(ctx, "AdvanceWorkingHours: lịch của xã không tính được hạn — TỪ CHỐI",
			"xa", string(xa), "loai", string(cauHinh.Loai))
		return status.Error(codes.FailedPrecondition, cauHinh.Error())
	}

	var xungDot *domain.LoiNgayVuaNghiVuaLamBu
	if errors.As(err, &xungDot) {
		// A date declared closed AND working. There is deliberately no precedence rule: a silent
		// winner makes one of two rows a person can see on the configuration screen do nothing,
		// and nobody ever finds out which.
		s.d.Log.WarnContext(ctx, "AdvanceWorkingHours: lịch của xã mâu thuẫn — ngày vừa nghỉ lễ vừa làm bù, TỪ CHỐI thay vì chọn bên",
			"xa", string(xa), "so_ngay", len(xungDot.Ngay))
		return status.Error(codes.FailedPrecondition, xungDot.Error())
	}

	return s.loi(ctx, err, cho)
}

// sangGio widens the wire's uint32 amounts to the int the arithmetic works in.
//
// EVERY VALUE HAS ALREADY BEEN CHECKED against TranGioMotMoc above, so nothing here can overflow
// and nothing here rejects: a second, quieter rejection hidden in a converter is a refusal with no
// status code attached. Duplicates are NOT collapsed here — domain.TienGioLamViec does it, in the
// same pass that sorts them, so there is one place where "one item per distinct amount" is true.
func sangGio(gio []uint32) []int {
	ra := make([]int, 0, len(gio))
	for _, g := range gio {
		ra = append(ra, int(g))
	}
	return ra
}
