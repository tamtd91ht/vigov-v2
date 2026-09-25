package app

// The petition processing logbook `nhat_ky_phan_anh` (migration 0013) — the automatic row every
// processing act writes, and the one act that writes nothing else: the manual internal note.
//
// # ONE ROW PER ACT, IN THE ACT'S OWN TRANSACTION
//
// The six acts of xu_ly_phan_anh.go each call ghiNhatKy between audit.Write and the outbox row, so
// the petition's change, the trail, the timeline row and the notification obligation stand or fall
// together (rule 6, invariant 3; rule 2, invariant 6). A timeline row that could be missing for an
// act that committed is a timeline an officer reads as "nothing happened here".
//
// INTAKE WRITES NO ROW (owner's decision, 2026-09-26): the drawer's timeline starts at the first
// staff act. The intake is already on the petition itself and in `audit_log`.
//
// # THE NOTE IS PERSONAL DATA
//
// `noi_dung` is staff free text and will eventually quote the reporter (rule 3). It is stored in the
// row and NOWHERE ELSE: the audit delta carries its LENGTH (`do_dai_ghi_chu`), the event carries
// nothing about it, and no error or log line built on this path quotes it.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// ghiNhatKy appends the timeline row for one act. `sau` is the petition AFTER the act, so the row's
// status is the status the act landed it in (migration 0013: "for a status-changing act it is the
// status AFTER the act"); a re-assignment or a note records the status it stood in, unchanged.
//
// THE ASSIGNMENT PAIR ONLY ON `phan-cong` (laPhanCong), copied from the petition as the act left it —
// migration 0013's CHECK refuses the pair on every other act. `CanBoXuLyID` on the petition holds a
// staff BUSINESS CODE (see duocTienTrangThai), which is what `can_bo_xu_ly_ma` is named for.
//
// THE AUTHOR IS audit.Actor.ID, WHICH IS Principal.Ma (rule 6, invariant 8). Empty refuses, with no
// fallback: coCanBoThucHien already refused before the transaction, and this second wall is here so
// a future caller that skips it cannot write a row naming nobody.
func (uc *XuLyPhanAnh) ghiNhatKy(ctx context.Context, tx *store.ScopedTx, sau domain.PhieuPhanAnh,
	hanhVi domain.HanhViNhatKy, luc time.Time, nguoi audit.Actor, ghiChu string, laPhanCong bool) error {

	if nguoi.ID == "" {
		return errors.New(loiThieuChuThe)
	}
	id, err := uc.sinhID()
	if err != nil {
		return fmt.Errorf("nhat_ky_phan_anh: sinh mã nội bộ: %w", err)
	}
	e := domain.NhatKyPhanAnh{
		ID:             id,
		PhieuPhanAnhID: sau.ID,
		ThoiDiem:       luc,
		NguoiMa:        nguoi.ID,
		HanhVi:         hanhVi,
		TrangThai:      sau.TrangThai,
		NoiDung:        ghiChu,
	}
	if laPhanCong {
		e.BoPhanID, e.CanBoXuLyMa = sau.BoPhanID, sau.CanBoXuLyID
	}
	return uc.kho.GhiNhatKy(ctx, tx, e)
}

// voiDoDaiGhiChu adds the note's LENGTH to an audit delta when a note was sent, and nothing else
// about it. The text itself never enters `audit_log` — append-only and never deleted, a copy there
// is a second permanent store of personal data (rule 6, forbidden #4).
func voiDoDaiGhiChu(delta map[string]any, ghiChu string) map[string]any {
	if ghiChu != "" {
		delta["do_dai_ghi_chu"] = utf8.RuneCountInString(ghiChu)
	}
	return delta
}

// QuyenGhiChuCaXa is ONE fact asked at the edge: does this account hold ANY of the three commune-wide
// processing keys — `feedback.resolve`, `feedback.assign`, `feedback.classify`? (Owner's decision,
// 2026-09-26.) A NAMED TYPE for the reason QuyenXuLyCaXa is one: a bare bool at a call site reads as
// nothing, and the wrong literal opens every petition's timeline to every reader.
//
// IT IS NOT QuyenXuLyCaXa, and the difference is deliberate: that one is `feedback.resolve` alone and
// gates MOVING a petition. Writing a note moves nothing, so the three keys of anybody who works on
// petitions commune-wide are enough.
type QuyenGhiChuCaXa bool

// duocGhiChu decides who may write a manual note: the commune-wide fact, OR being the officer the
// petition is assigned to — THE SAME holding rule duocTienTrangThai applies, reused rather than
// restated, so the "unassigned matches nobody" guard cannot drift between the two. Refusal is
// ErrKhongPhaiNguoiDuocGiao, the 403 the advance route already answers.
func duocGhiChu(p domain.PhieuPhanAnh, nguoi audit.Actor, quyen QuyenGhiChuCaXa) error {
	return duocTienTrangThai(p, nguoi, QuyenXuLyCaXa(quyen))
}

// GhiChuNoiBo appends one manual internal note (`ghi-chu`) to a petition's timeline.
// Route permission: `feedback.read`; the real condition is duocGhiChu.
//
// ALLOWED ON EVERY STATUS, the three final ones included: a note records something learned about the
// case and moves nothing, so it is not "editing an archival record" (rule 7) — it is appending to it.
//
// THE CHECKS RUN INSIDE THE TRANSACTION, ON THE ROW READ `FOR UPDATE`, in the order the advance route
// uses and for its reason: the restricted field first (404, identical to an unknown code), then the
// holding rule (403). The lock also makes the row's status the status at this instant.
//
// NO EVENT: nothing the citizen can see changed, and the note is staff-internal (rule 4,
// forbidden #5).
func (uc *XuLyPhanAnh) GhiChuNoiBo(ctx context.Context, ma, ghiChuTho string, nguoi audit.Actor,
	quyen QuyenGhiChuCaXa, hanChe QuyenXemHanChe) (domain.NhatKyPhanAnh, error) {

	ghiChu, err := domain.KiemGhiChu(ghiChuTho)
	if err != nil {
		return domain.NhatKyPhanAnh{}, err
	}
	if err := coCanBoThucHien(nguoi); err != nil {
		return domain.NhatKyPhanAnh{}, err
	}

	bayGio := uc.nayHoac()
	var dong domain.NhatKyPhanAnh

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		p, err := uc.kho.TheoMaTraCuuDeSua(ctx, tx, ma)
		if err != nil {
			return err
		}
		if err := duocChamPhieuHanChe(p, hanChe); err != nil {
			return err
		}
		if err := duocGhiChu(p, nguoi, quyen); err != nil {
			return err
		}

		id, err := uc.sinhID()
		if err != nil {
			return fmt.Errorf("nhat_ky_phan_anh: sinh mã nội bộ: %w", err)
		}
		dong = domain.NhatKyPhanAnh{
			ID:             id,
			PhieuPhanAnhID: p.ID,
			ThoiDiem:       bayGio,
			NguoiMa:        nguoi.ID,
			HanhVi:         domain.NhatKyGhiChu,
			TrangThai:      p.TrangThai,
			NoiDung:        ghiChu,
		}
		if err := uc.kho.GhiNhatKy(ctx, tx, dong); err != nil {
			return err
		}

		// THE LENGTH AND THE ROW, NEVER THE TEXT (rule 6, forbidden #4). The row id lets an inspection
		// go from the trail to the timeline entry; the text is there, frozen by the trigger.
		delta, err := json.Marshal(map[string]any{
			"nhat_ky_id":               id,
			"trang_thai_tai_thoi_diem": string(p.TrangThai),
			"do_dai_ghi_chu":           utf8.RuneCountInString(ghiChu),
		})
		if err != nil {
			return fmt.Errorf("xu_ly_phan_anh: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   nguoi,
			Action:  HanhViGhiChuPhanAnh,
			Subject: p.MaTraCuu,
			Delta:   delta,
		})
	})
	if err != nil {
		return domain.NhatKyPhanAnh{}, bocPhieu(ctx, "ghi chú", err)
	}
	return dong, nil
}
