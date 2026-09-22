package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// ResolveDeadlines — the server side of a commune's commitment, joined from its two halves.
//
// THIS FILE JOINS; IT DOES NOT COUNT AND IT DOES NOT DECIDE. The row lookup and its fallback live
// in domain.DongTheoLinhVuc, the walk lives in domain.TienGioLamViec, and the calendar read is
// Server.tienGioLamViec — the SAME one AdvanceWorkingHours enters, because ADR 0007 forbids two
// implementations of the working-hours count. What is here is the wire contract and one refusal
// that is the whole point of the RPC: a commune that has configured nothing is told so, never
// given a number.

// SLADoc reads this commune's deadline table, WHOLE.
//
// DECLARED HERE AT THE POINT OF USE, like the three calendar interfaces in lich_lam_viec.go, so
// every branch below is checkable without a PostgreSQL. *idstore.SLAStore satisfies it as it is;
// nothing was changed to accommodate this.
//
// WHOLE AND NOT PAGINATED, and the store's own note says why it cannot be: answering "what is the
// deadline for this field" needs TWO rows — the field's own row if it has one, and the default row
// it falls back to otherwise. A page cannot promise to hold both, and a caller handed a page
// cannot tell "this field has no row" from "this page does not reach it". The first means use the
// default; the second means the answer is wrong.
type SLADoc interface {
	DanhSach(ctx context.Context) ([]domain.DongSLA, error)
}

// TranMocHan — at most 2 entries in `deadlines`, checked on what the caller SENT and before
// duplicates are collapsed.
//
// NOT A HEADROOM ESTIMATE, UNLIKE TranSoMocMotLo. DeadlineKind has exactly two non-zero values, so
// a third entry cannot be anything legitimate: it is a caller with no ceiling of its own, and the
// refusal costs it nothing because there is no request it stops from being expressible.
const TranMocHan = 2

// ResolveDeadlines answers, for one record of one commune, the instants its processing
// commitments fall due.
//
// # The order of the steps is deliberate
//
// CALLER FAULTS FIRST, BEFORE ANY STORE READ. A misshapen call must not become read load on a
// process serving 200+ communes — the same discipline AdvanceWorkingHours keeps, and the tests
// assert the stores were not touched.
//
// # A commune with no configuration is REFUSED, and that is today's ordinary answer
//
// migration 0008 seeds nothing, deliberately, and the onboarding step does not exist, so every
// commune is in this state right now. FAILED_PRECONDITION is the honest code: it is a state a
// HUMAN fixes on a configuration screen, not a fault in the calling service and not an outage.
//
// NOTHING HERE FALLS BACK TO A NUMBER. Not 24 hours, and not the sixteen rows of
// docs/ui-ux/14-cau-hinh.md §8 either — those are ONE commune's prototype, and a commitment
// invented by software is still told to a citizen as though the authority made it (rule 10,
// forbidden #3). The refusal IS the answer.
//
// # Where the codes diverge from AdvanceWorkingHours, and why it matters
//
// A number of hours that is zero, negative or absurd is INVALID_ARGUMENT there and
// FAILED_PRECONDITION here. The shape is the same; the SOURCE of the number is not. There the
// caller sent it, so the caller is at fault; here it came out of the commune's own table, and an
// operator sent to fix the calling service would find nothing wrong with it.
//
// # Nothing is written, nothing is audited, and retrying is safe
//
// Two configuration reads and some arithmetic. The audited act is the INTAKE, written by the
// caller inside its own transaction (rule 6, invariant 3).
func (s *Server) ResolveDeadlines(ctx context.Context, req *identityv1.ResolveDeadlinesRequest) (
	*identityv1.ResolveDeadlinesResponse, error) {

	loai, err := loaiViecTu(req.GetWorkKind())
	if err != nil {
		return nil, err
	}

	tuMoc := req.GetCountFrom()
	if tuMoc == nil {
		// NO "COUNT FROM NOW" DEFAULT, for the reason AdvanceWorkingHoursRequest.count_from gives:
		// a default makes the commitment depend on when the call happened to be made rather than on
		// when the authority received the file.
		return nil, status.Error(codes.InvalidArgument, "thiếu count_from")
	}
	if err := tuMoc.CheckValid(); err != nil {
		return nil, status.Error(codes.InvalidArgument, "count_from không phải một mốc thời gian hợp lệ")
	}

	can, err := mocHanTu(req.GetDeadlines())
	if err != nil {
		return nil, err
	}

	// The commune is not used as a value below — both reads are scoped to it from the context by
	// store.Scoped. This call is the guard that the commune interceptor is in the chain at all, and
	// it closes the panic path through store.Scoped's MustFrom, which a gRPC handler cannot recover
	// from. The value is kept for the log lines, where "which commune's configuration" is the first
	// question an operator asks.
	xa, err := s.xa(ctx)
	if err != nil {
		return nil, err
	}

	ds, err := s.d.SLA.DanhSach(ctx)
	if err != nil {
		// Including ErrQuaNhieuDongSLA and ErrLoaiViecLa. Both are Internal rather than
		// FAILED_PRECONDITION on purpose: neither is a hole a commune can see or fill on its
		// configuration screen. A table over the ceiling is an import run twice or a fixture on a
		// live database, and an unknown `loai_viec` means the CHECK constraint is not there — a
		// database restored from elsewhere. Sending an operator to the SLA screen for either would
		// waste the afternoon that matters.
		return nil, s.loi(ctx, err, "ResolveDeadlines/sla")
	}

	// THE FALLBACK TO THE DEFAULT ROW IS THE SPECIFICATION'S, NOT THIS HANDLER'S — see
	// domain.DongTheoLinhVuc. FALSE MEANS REFUSE: there is nothing below the default row, and the
	// one thing worse than refusing to make a commitment is inventing one.
	dong, co := domain.DongTheoLinhVuc(ds, loai, req.GetLinhVuc())
	if !co {
		return nil, s.loiThieuCauHinhSLA(ctx, ds, loai, xa)
	}

	// THE HOURS ARE VALIDATED BEFORE THE ARITHMETIC, and the code is FAILED_PRECONDITION because
	// the number came from the commune's table. The schema already refuses a non-positive value
	// (`sla_gio_phai_duong`), so reaching this means the constraint is not there — the same class of
	// fault the store refuses an unknown `loai_viec` for, except that this one can be reported as
	// something a human fixes by editing the row.
	//
	// IT MATTERS MOST FOR ZERO. A zero handed to the arithmetic answers `count_from` itself — a
	// record due at the instant it was received, which every report downstream reads as "already
	// late", with nothing erroring anywhere.
	gioTheoMoc := make(map[identityv1.DeadlineKind]int, len(can))
	for _, k := range can {
		g := gioCua(dong, k)
		switch {
		case g <= 0:
			s.d.Log.WarnContext(ctx, "ResolveDeadlines: dòng SLA của xã có số giờ không dương — TỪ CHỐI",
				"xa", string(xa), "dong_sla", dong.ID, "moc", k.String(), "so_gio", g)
			return nil, status.Errorf(codes.FailedPrecondition,
				"cấu hình thời hạn xử lý của xã có số giờ không dương cho mốc %s — sửa dòng SLA, không tính hạn từ nó",
				k.String())
		case g > TranGioMotMoc:
			s.d.Log.WarnContext(ctx, "ResolveDeadlines: dòng SLA của xã vượt trần số giờ — TỪ CHỐI",
				"xa", string(xa), "dong_sla", dong.ID, "moc", k.String(), "so_gio", g)
			return nil, status.Errorf(codes.FailedPrecondition,
				"cấu hình thời hạn xử lý của xã đặt %d giờ cho mốc %s, vượt trần %d giờ",
				g, k.String(), TranGioMotMoc)
		}
		gioTheoMoc[k] = g
	}

	// ONE CALENDAR READ FOR BOTH CLOCKS. Two clocks counted from the same instant against the same
	// calendar is the property the plural shape of TienGioLamViec exists to preserve; splitting it
	// into two calls would give the second the chance to read a calendar the first did not.
	dat, err := s.tienGioLamViec(ctx, xa, tuMoc.AsTime(), gopGio(gioTheoMoc, can), "ResolveDeadlines")
	if err != nil {
		return nil, err
	}

	// KEYED BY THE AMOUNT, because the arithmetic collapses duplicates: two clocks configured with
	// the same number of hours come back as ONE entry, and an index would silently pair the second
	// clock with nothing.
	datLuc := make(map[int]*timestamppb.Timestamp, len(dat))
	for _, m := range dat {
		datLuc[m.Gio] = timestamppb.New(m.DatLuc)
	}

	items := make([]*identityv1.Deadline, 0, len(can))
	for _, k := range can {
		t, roi := datLuc[gioTheoMoc[k]]
		if !roi {
			// UNREACHABLE unless domain.TienGioLamViec stops answering every amount it was given.
			// It is checked rather than assumed because the alternative is an item with no `due_at`,
			// which reaches the caller as a zero instant — a deadline in year 1, i.e. a record
			// overdue the moment it is created. There is no partial success on this contract.
			s.d.Log.ErrorContext(ctx, "CẢNH BÁO HỢP ĐỒNG: phép tiến giờ không trả mốc cho một hạn đã hỏi",
				"xa", string(xa), "moc", k.String(), "so_gio", gioTheoMoc[k])
			return nil, status.Error(codes.Internal, "lỗi nội bộ, vui lòng thử lại")
		}
		items = append(items, &identityv1.Deadline{Kind: k, DueAt: t})
	}
	return &identityv1.ResolveDeadlinesResponse{Items: items}, nil
}

// loiThieuCauHinhSLA turns "no usable row" into a refusal that names WHAT to configure.
//
// TWO CAUSES, TOLD APART, because they send a person to two different places on one screen: an
// empty table means the commune has configured nothing at all, a missing default row means it
// configured some fields and left the row everything else falls back to. The second is the
// invisible one — a commune that filled in `an-ninh-trat-tu` and nothing else LOOKS configured,
// and every petition in any other field gets no deadline at all.
//
// THE NAMES ARE domain.LoaiVanDeSLA's, not new strings invented here: the configuration screen
// shows those values, and a second vocabulary for one fault is a fault an operator cannot match
// between a log line and a screen.
func (s *Server) loiThieuCauHinhSLA(ctx context.Context, ds []domain.DongSLA,
	loai domain.LoaiViec, xa tenant.ID) error {

	if len(ds) == 0 {
		s.d.Log.WarnContext(ctx, "ResolveDeadlines: xã chưa cấu hình bảng thời hạn xử lý — TỪ CHỐI",
			"xa", string(xa), "loai_van_de", string(domain.VanDeSLATrong), "loai_viec", string(loai))
		return status.Error(codes.FailedPrecondition,
			"xã chưa cấu hình bảng thời hạn xử lý — không có hạn nào để hứa, phải cấu hình trước khi tiếp nhận")
	}
	s.d.Log.WarnContext(ctx, "ResolveDeadlines: loại việc không có dòng mặc định trong bảng thời hạn xử lý — TỪ CHỐI",
		"xa", xa.String(), "loai_van_de", string(domain.VanDeThieuDongMacDinh), "loai_viec", string(loai))
	return status.Errorf(codes.FailedPrecondition,
		"bảng thời hạn xử lý của xã không có dòng nào dùng được cho loại việc %s — thiếu dòng mặc định", loai)
}

// loaiViecTu maps the wire enum onto the domain type.
//
// UNSPECIFIED IS A CALLER FAULT AND NOT A DEFAULT. Picking one here would read another business
// domain's numbers — `documents`' hours promised on a citizen's petition, or the reverse — and
// nothing downstream could see that it happened.
//
// THE SWITCH IS EXHAUSTIVE AND THE FALLTHROUGH REFUSES. A fourth value added to the contract
// without ADR 0029's stop condition #1 being answered arrives here as an unknown, and it is
// refused rather than silently treated as one of the three.
func loaiViecTu(k identityv1.WorkKind) (domain.LoaiViec, error) {
	switch k {
	case identityv1.WorkKind_WORK_KIND_VAN_BAN_DEN:
		return domain.LoaiViecVanBanDen, nil
	case identityv1.WorkKind_WORK_KIND_PHAN_ANH:
		return domain.LoaiViecPhanAnh, nil
	case identityv1.WorkKind_WORK_KIND_NHIEM_VU:
		return domain.LoaiViecNhiemVu, nil
	case identityv1.WorkKind_WORK_KIND_UNSPECIFIED:
		return "", status.Error(codes.InvalidArgument,
			"thiếu work_kind — không có loại việc mặc định, đoán một loại là đọc số giờ của nghiệp vụ khác")
	}
	return "", status.Errorf(codes.InvalidArgument,
		"work_kind %d không nằm trong ba loại việc được phép", int32(k))
}

// mocHanTu validates the requested clocks and collapses duplicates, KEEPING THE ORDER THE CALLER
// SENT — so a response read top to bottom matches the request read top to bottom, which is the
// only thing a human comparing the two by eye can rely on.
//
// THE CEILING IS CHECKED ON WHAT THE CALLER SENT, before duplicates are collapsed: the point is to
// bound the request, and a caller sending the same value five times is still a caller with no
// ceiling of its own.
//
// AN UNSPECIFIED ENTRY IS REFUSED, NEVER SKIPPED. Skipping it would return fewer items than were
// asked for, which this contract defines as a fault — so the caller would be shown a contract
// error where the real fault was its own zero value.
func mocHanTu(ds []identityv1.DeadlineKind) ([]identityv1.DeadlineKind, error) {
	if len(ds) == 0 {
		return nil, status.Error(codes.InvalidArgument,
			"thiếu deadlines — bên gọi phải nói rõ nó đang ấn định đồng hồ nào (ADR 0028 quyết định E)")
	}
	if len(ds) > TranMocHan {
		return nil, status.Errorf(codes.InvalidArgument,
			"deadlines có %d mục, vượt trần %d — chỉ có hai đồng hồ", len(ds), TranMocHan)
	}

	ra := make([]identityv1.DeadlineKind, 0, len(ds))
	thay := make(map[identityv1.DeadlineKind]struct{}, len(ds))
	for _, k := range ds {
		switch k {
		case identityv1.DeadlineKind_DEADLINE_KIND_TIEP_NHAN,
			identityv1.DeadlineKind_DEADLINE_KIND_XU_LY_XONG:
		default:
			return nil, status.Errorf(codes.InvalidArgument,
				"deadlines có mục %d không phải một đồng hồ hợp lệ", int32(k))
		}
		if _, co := thay[k]; co {
			continue
		}
		thay[k] = struct{}{}
		ra = append(ra, k)
	}
	return ra, nil
}

// gioCua reads the one column of an SLA row that the requested clock is fixed from.
//
// TWO ADJACENT INTEGER COLUMNS, AND SWAPPING THEM PRODUCES NO ERROR ANYWHERE — the store's own note
// on cotSLA makes the same warning about five of them. Both values are plausible working-hour
// counts, both produce a valid instant, and the only difference is which commitment the authority
// made. The tests therefore give the two columns DIFFERENT values.
//
// THE DEFAULT RETURNS 0, AND ITS CALLER REFUSES ON THAT. It is unreachable — mocHanTu has already
// rejected everything but the two — and it is written this way rather than as a panic because a
// panic in a gRPC handler has no recovery and takes down a process serving 200+ communes.
func gioCua(d domain.DongSLA, k identityv1.DeadlineKind) int {
	switch k {
	case identityv1.DeadlineKind_DEADLINE_KIND_TIEP_NHAN:
		return d.GioTiepNhan
	case identityv1.DeadlineKind_DEADLINE_KIND_XU_LY_XONG:
		return d.GioXuLyXong
	}
	return 0
}

// gopGio lists the distinct amounts of working time the walk has to answer.
//
// DISTINCT, because two clocks configured with the same number of hours are ONE question — and
// domain.TienGioLamViec collapses duplicates anyway, so sending both would make len(result) differ
// from len(request) for no reason and put a reader off the scent.
func gopGio(gioTheoMoc map[identityv1.DeadlineKind]int, can []identityv1.DeadlineKind) []int {
	ra := make([]int, 0, len(can))
	thay := make(map[int]struct{}, len(can))
	for _, k := range can {
		g := gioTheoMoc[k]
		if _, co := thay[g]; co {
			continue
		}
		thay[g] = struct{}{}
		ra = append(ra, g)
	}
	return ra
}
