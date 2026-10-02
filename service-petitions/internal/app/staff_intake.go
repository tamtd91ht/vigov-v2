package app

// The use case behind the STAFF intake route — POST /api/v1/citizen-reports, "Nhập hộ phản ánh"
// (docs/ui-ux/09 §11): a citizen phoned, came to the office, or told the hamlet head, and an officer
// books the report into the register on their behalf.
//
// THE SAME ORDER AS THE CITIZEN INTAKE (gui_phan_anh.go), for the same reasons — check, ask identity,
// mint the code only once nothing can fail any more, then ONE transaction for the row and its trail:
//
//	0. CHECK the field against the commune's catalogue (ADR 0026, 0060) — REQUIRED on this channel
//	1. ASK identity for the RESOLVE deadline only, counted from `goc_dem_han`
//	2. MINT the lookup code — the same crypto/rand generator as the citizen path (rule 4, invariant 4)
//	3. ONE TRANSACTION: the row and its audit entry (rule 6, invariant 3)
//
// # WHAT ADR 0028 DECIDES ABOUT THIS CHANNEL, AND WHERE EACH DECISION LANDS
//
//	E   the form carries the field, so the commitments are fixed at booking. The acknowledge one does
//	    not exist here (F #5), so that is the resolve deadline — fixed now, from the field's SLA row
//	F1  `goc_dem_han` = when the citizen ACTUALLY reported it, if the officer recorded that (optional)
//	F2  absent -> `goc_dem_han` = the booking instant
//	F3  bounded to [booking − 7 calendar days, booking]; outside is REFUSED, never clamped
//	    (domain.KiemGocDemHan, and migration 0004's CHECK under it)
//	F4  every non-default origin is AUDITED with before and after
//	F5  `han_tiep_nhan` is NULL — "KHÔNG ÁP DỤNG", never 0 hours (and 0004's CHECK refuses otherwise)
//	+   `han_phan_loai` is NULL too: the petition arrives classified, so there is no ceiling to bound
//
// # NO CITIZEN ACCOUNT, EVER (ADR 0028 §Bổ sung 2026-10-02)
//
// `cong_dan_id` stays empty. The petition never appears in anybody's "Phản ánh của tôi" — not even when
// the number the officer typed matches an account: one mistyped digit would show the report to another
// citizen (rule 4, invariant 1). The officer hands the lookup code over instead, so the code is in the
// response. With no recipient there is also no `petitions.status_changed.v1` row (ghiSuKienDoiTrangThai,
// "no recipient, no row") — the notification gap that ADR names as still open.
//
// # NO HAMLET, NO PHOTOS, THIS ROUND
//
// §11 offers `Thôn, tổ dân phố` and `Đính ảnh hiện trường`. Neither is accepted: no identity RPC
// validates a hamlet id, so `thon_id` would be written straight from the client (rule 1, forbidden #2 in
// spirit); and the scene-photo flow is bound to a citizen session (migration 0026 refuses a petition with
// no `cong_dan_id`). Both need a contract first.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/audit"
	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// ResolveDeadlineReader is identity's ResolveDeadlines as core/identityclient exposes it. The staff
// channel asks it ONE question (the resolve clock); the classification ceiling's AdvanceWorkingHours is
// not needed here, which is why this is not HanTiepNhanDoc. *identityclient.Client satisfies it.
type ResolveDeadlineReader interface {
	// vi-name-ok: the method name of the existing core/identityclient.Client this interface must match
	HanXuLy(ctx context.Context, loaiViec identityv1.WorkKind, linhVuc string, tuLuc time.Time,
		can []identityv1.DeadlineKind) (map[identityv1.DeadlineKind]time.Time, error)
}

// StaffIntakeFields checks the field an officer picked. *PetitionFieldCatalogue satisfies it; answers
// ErrFieldNotOffered or ErrFieldCatalogueUnavailable.
type StaffIntakeFields interface {
	CheckStaffIntakeField(ctx context.Context, code string) (string, error)
}

// staffIntakeChannel is the channel EVERY petition booked through this use case carries. A constant,
// never a request field: `kenh_tiep_nhan` decides which deadline rules apply (ADR 0028 decision E) and
// is immutable once written. §11's "Tiếp nhận qua kênh" select is NOT honoured — see the route comment.
const staffIntakeChannel = domain.KenhCanBoNhapHo

// ActionStaffIntake is the business verb in the trail. A DIFFERENT verb from HanhViGuiPhanAnh, as that
// constant's comment asks: "who filed this" is the first question of a disputed petition. The value is
// Vietnamese snake_case like the citizen verb it sits beside, because an inspection reads the two
// together.
const ActionStaffIntake = "can_bo_nhap_ho_phan_anh"

// StaffIntakeRequest is one petition as an officer books it. NO citizen-id field — see the file header.
type StaffIntakeRequest struct {
	Content       string
	Address       string
	ReporterName  string // personal data (rule 3) — masked in every read
	ReporterPhone string // personal data (rule 3) — masked in every read
	Anonymous     bool

	// Field is REQUIRED on this channel (§11, migration 0004 `phieu_phan_anh_nhap_ho_co_linh_vuc`).
	Field string

	// ClockFrom is when the citizen ACTUALLY reported it (ADR 0028 F1). Zero = the booking instant (F2).
	ClockFrom time.Time
}

// StaffIntake owns the staff-booked intake.
type StaffIntake struct {
	db        *store.DB
	petitions KhoPhieuGhi
	deadlines ResolveDeadlineReader
	fields    StaffIntakeFields

	// events is passed to the shared outbox builder, which writes nothing for a petition with no
	// citizen. Kept so the "no recipient, no row" rule stays in ONE place (ghiSuKienDoiTrangThai).
	events KhoSuKien

	// newID, newCode and clock are injected so a test can pin them. In production: ulid.Moi,
	// domain.SinhMaTraCuu and the UTC wall clock.
	newID   func() (string, error)
	newCode func() (string, error)
	clock   func() time.Time
}

func NewStaffIntake(db *store.DB, petitions KhoPhieuGhi, events KhoSuKien, deadlines ResolveDeadlineReader,
	fields StaffIntakeFields) *StaffIntake {
	return &StaffIntake{
		db: db, petitions: petitions, events: events, deadlines: deadlines, fields: fields,
		newID:   ulid.Moi,
		newCode: domain.SinhMaTraCuu,
		clock:   func() time.Time { return time.Now().UTC() },
	}
}

// Book records one petition an officer took by telephone or in person, and returns the record —
// including the lookup code the officer hands to the citizen (rule 10, invariant 1).
func (uc *StaffIntake) Book(ctx context.Context, req StaffIntakeRequest, officer audit.Actor) (
	domain.PhieuPhanAnh, error) {

	// The trail names the officer's BUSINESS CODE; an empty one refuses before anything is read
	// (rule 6, invariant 8). A citizen principal here means the route was mounted on the wrong mux.
	if err := coCanBoThucHien(officer); err != nil {
		return domain.PhieuPhanAnh{}, err
	}
	// The channel's own predicates, consulted rather than assumed — the day this constant changes,
	// these are what must fail before a wrong deadline shape reaches a record.
	if staffIntakeChannel.HanTiepNhanApDung() || !staffIntakeChannel.CoLinhVucLucVaoSo() {
		return domain.PhieuPhanAnh{}, fmt.Errorf(
			"nhap_ho_phan_anh: kênh %q không phải kênh cán bộ nhập hộ (ADR 0028 quyết định E)", staffIntakeChannel)
	}

	// Validated BEFORE identity is asked and before any transaction opens.
	content, err := domain.ChuanHoaNoiDung(req.Content)
	if err != nil {
		return domain.PhieuPhanAnh{}, err
	}
	address, err := domain.ChuanHoaDiaChi(req.Address)
	if err != nil {
		return domain.PhieuPhanAnh{}, err
	}
	name, err := domain.ChuanHoaHoTen(req.ReporterName)
	if err != nil {
		return domain.PhieuPhanAnh{}, err
	}
	phone, err := domain.ChuanHoaDienThoai(req.ReporterPhone)
	if err != nil {
		return domain.PhieuPhanAnh{}, err
	}
	field, err := domain.KiemLinhVuc(req.Field)
	if err != nil {
		return domain.PhieuPhanAnh{}, err
	}

	// ONE READ OF THE CLOCK. `vao_so_luc` is this instant; `goc_dem_han` is it too unless the officer
	// recorded an earlier one — and the bound is checked against this same value, so migration 0004's
	// CHECK (which compares the two stored columns) cannot disagree with the refusal here.
	bookedAt := uc.clock()
	clockFrom := bookedAt
	overridden := !req.ClockFrom.IsZero()
	if overridden {
		clockFrom = req.ClockFrom.UTC()
		// REFUSED, NEVER CLAMPED (ADR 0028 F3, stop condition #4).
		if err := domain.KiemGocDemHan(clockFrom, bookedAt); err != nil {
			return domain.PhieuPhanAnh{}, err
		}
	}

	// THE FIELD, CHECKED BEFORE IDENTITY IS ASKED: identity answers a code it has no row for with the
	// DEFAULT row, silently, so an unchecked code would fix a wrong commitment onto an archival record.
	if uc.fields == nil {
		return domain.PhieuPhanAnh{}, errors.New("nhap_ho_phan_anh: thiếu bộ kiểm lĩnh vực — sai nối dây")
	}
	if field, err = uc.fields.CheckStaffIntakeField(ctx, field); err != nil {
		return domain.PhieuPhanAnh{}, err
	}

	// STEP 1 — the resolve commitment, from the field's own SLA row, counted in WORKING HOURS by identity
	// from `goc_dem_han` (ADR 0007, ADR 0028 E). ONLY that clock: asking for the acknowledge clock would
	// produce a value for a column that must stay NULL (F5).
	due, err := uc.deadlines.HanXuLy(ctx, identityv1.WorkKind_WORK_KIND_PHAN_ANH, field, clockFrom,
		[]identityv1.DeadlineKind{identityv1.DeadlineKind_DEADLINE_KIND_XU_LY_XONG})
	if err != nil {
		return domain.PhieuPhanAnh{}, fmt.Errorf("%w cho xã %s: %w", ErrChuaAnDinhDuocHan, tenant.MustFrom(ctx), err)
	}
	resolveDue, ok := due[identityv1.DeadlineKind_DEADLINE_KIND_XU_LY_XONG]
	if !ok || resolveDue.IsZero() {
		// A zero here reaches `han_xu_ly_xong` as NULL — "CHƯA CÓ" on a petition no classification will
		// ever come back to fill, because the field is already settled.
		return domain.PhieuPhanAnh{}, fmt.Errorf("%w cho xã %s: hợp đồng trả về hạn xử lý xong rỗng",
			ErrChuaAnDinhDuocHan, tenant.MustFrom(ctx))
	}

	// STEP 2 — the code, minted only now.
	id, err := uc.newID()
	if err != nil {
		return domain.PhieuPhanAnh{}, fmt.Errorf("nhap_ho_phan_anh: sinh định danh: %w", err)
	}
	code, err := uc.newCode()
	if err != nil {
		return domain.PhieuPhanAnh{}, fmt.Errorf("nhap_ho_phan_anh: sinh mã tra cứu: %w", err)
	}

	p := domain.PhieuPhanAnh{
		ID:       id,
		MaTraCuu: code,
		Kenh:     staffIntakeChannel,
		// CongDanID deliberately empty — ADR 0028 §Bổ sung 2026-10-02.
		NoiDung:           content,
		LinhVuc:           field,
		DiaChi:            address,
		NguoiGuiHoTen:     name,
		NguoiGuiDienThoai: phone,
		AnDanh:            req.Anonymous,

		// ⚠ `da-tiep-nhan`, THE FIRST OF THE NINE, and an ASSUMPTION stated as one: §11 says the booked
		// petition "đi cùng quy trình với phiếu gửi từ Zalo", and ADR 0027 D#2 makes the first status the
		// automatic one. The petition then goes through `dang-phan-loai` like every other — where
		// classification may only SHORTEN the deadline fixed here (ADR 0027 C) and where the two branch
		// endings leave from. `phan_loai_luc` stays NULL until that act.
		TrangThai: domain.DaTiepNhan,

		GocDemHan: clockFrom,
		VaoSoLuc:  bookedAt,

		// HanTiepNhan and HanPhanLoai left ZERO -> SQL NULL -> "KHÔNG ÁP DỤNG" (ADR 0028 F5).
		HanXuLyXong: resolveDue,
	}
	p.PublicationStatus = domain.InitialPublicationStatus(p.LinhVuc)

	// STEP 3 — the row and its trail, in ONE transaction (rule 6, invariant 3).
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		if err := uc.petitions.Tao(ctx, tx, p); err != nil {
			return err
		}
		// NO PERSONAL DATA IN THE DELTA, not even masked — the reasoning is on the citizen intake's delta.
		d := map[string]any{
			"kenh_tiep_nhan": string(p.Kenh),
			"trang_thai":     string(p.TrangThai),
			"an_danh":        p.AnDanh,
			"linh_vuc":       p.LinhVuc,
			"goc_dem_han":    p.GocDemHan,
			"vao_so_luc":     p.VaoSoLuc,
			// JSON null, said outright: "không áp dụng", never 0 hours.
			"han_tiep_nhan":      nil,
			"han_phan_loai":      nil,
			"han_xu_ly_xong":     p.HanXuLyXong,
			"truong_da_dien":     truongDaDien(p),
			"do_dai_noi_dung":    len([]rune(p.NoiDung)),
			"publication_status": string(p.PublicationStatus),
		}
		// ADR 0028 F4: every origin the officer typed is recorded with BEFORE (the default the system
		// would have used — the booking instant) and AFTER (what was typed). English key, rule 12.
		if overridden {
			d["clock_from_override"] = map[string]any{"before": p.VaoSoLuc, "after": p.GocDemHan}
		}
		delta, err := json.Marshal(d)
		if err != nil {
			return fmt.Errorf("nhap_ho_phan_anh: mã hoá delta: %w", err)
		}
		if err := audit.Write(ctx, tx, audit.Entry{
			Actor:   officer,
			Action:  ActionStaffIntake,
			Subject: p.MaTraCuu,
			Delta:   delta,
		}); err != nil {
			return err
		}
		// No citizen account -> the builder writes nothing (see the file header). Called rather than
		// skipped so the rule is decided in one place.
		return ghiSuKienDoiTrangThai(ctx, tx, uc.events, uc.newID, p, domain.DaTiepNhan, bookedAt)
	})
	if err != nil {
		// Neither the code, the content nor the reporter is in the message (rule 3).
		return domain.PhieuPhanAnh{}, fmt.Errorf("nhap_ho_phan_anh: ghi phiếu cho xã %s: %w", tenant.MustFrom(ctx), err)
	}
	return p, nil
}

// IsStaffIntakeInputError reports whether err refuses what the officer SENT (400). Listed explicitly —
// the same discipline as domain.LaLoiGuiPhanAnh — so an outage never turns into a 400.
func IsStaffIntakeInputError(err error) bool {
	return domain.LaLoiGuiPhanAnh(err) ||
		errors.Is(err, domain.ErrThieuLinhVuc) || errors.Is(err, domain.ErrLinhVucSaiDang)
}
