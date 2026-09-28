package app

// The manual timeline entry on a task — POST /api/v1/tasks/{ma}/log-entries, §5.9 "Nhật ký & Trao đổi".
//
// MODELLED ON THE PETITION'S MANUAL NOTE (nhat_ky_phan_anh.go, GhiChuNoiBo): the route's gate is the
// READ key, and who may actually write is decided here, on the row read FOR UPDATE — because the
// answer depends on THIS task (vigov-require a37ec96: "cửa chỉ đòi quyền đọc, tầng nghiệp vụ mới quyết
// ai ghi được gì trên đúng nhiệm vụ này"). The rule itself is domain.TaskWorkRightFor.
//
// # ONE TRANSACTION, TWO WRITES (rule 6, invariant 3)
//
// The timeline row and the audit entry. The entry TEXT lives in the row only: the audit delta carries
// its LENGTH and the row id, never the words — staff free text about the work can name a citizen's
// case, and audit_log is append-only for ever (rule 6, forbidden #4).
//
// NO ATTACHMENTS IN THIS PASS: `dinh_kem` exists but there is no file store.

import (
	"context"
	"encoding/json"
	"fmt"
	"unicode/utf8"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// ActionTaskLogEntry is the verb in the trail — Vietnamese snake_case like every other value this
// service writes (an inspection reads it; ADR 0011), and the task twin of `ghi_chu_phan_anh`.
const ActionTaskLogEntry = "ghi_nhat_ky_nhiem_vu"

// TaskUpdateRight is ONE fact asked at the edge: does this account hold the commune-wide `task.update`?
// A NAMED TYPE for the reason QuyenDuyetHoanThanh is one — a bare bool at a call site reads as nothing,
// and the wrong literal opens every task's timeline to every reader of the register.
type TaskUpdateRight bool

// AddLogEntry appends one manual entry to the task's timeline. Route permission: `task.read`; the real
// condition is domain.TaskWorkRightFor (assignee, related person, or `task.update`).
//
// ALLOWED ON EVERY STATUS, `hoan-thanh` included: an entry records something learned about the work
// and moves nothing, so it appends to the record rather than editing it (rule 7).
//
// THE CHECK RUNS ON THE ROW READ `FOR UPDATE`, so it is decided against the holder as it stands at this
// instant — a reassignment committing concurrently cannot let the previous holder write after it.
func (uc *GhiNhiemVu) AddLogEntry(ctx context.Context, ma, text string, actor audit.Actor,
	update TaskUpdateRight) (domain.NhatKyNhiemVu, error) {

	text, err := domain.KiemNoiDungNhatKy(text)
	if err != nil {
		return domain.NhatKyNhiemVu{}, err
	}
	if err := coCanBoThucHien(actor); err != nil {
		return domain.NhatKyNhiemVu{}, err
	}

	now := uc.nayHoac()
	var row domain.NhatKyNhiemVu

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		n, err := uc.kho.TheoMaDeSua(ctx, tx, ma)
		if err != nil {
			return err
		}
		right := domain.TaskWorkRightFor(n, actor.ID, bool(update))
		if err := domain.CheckMayWriteLogEntry(right); err != nil {
			return err
		}

		id, err := uc.sinhID()
		if err != nil {
			return fmt.Errorf("nhat_ky_nhiem_vu: sinh mã nội bộ: %w", err)
		}
		// THE SAME ROW SHAPE ghiNhatKy WRITES FOR EVERY OTHER ACT: the status the task stands in, and
		// the holder copied from the task. Built here rather than through ghiNhatKy only because the
		// reply needs the row back.
		row = domain.NhatKyNhiemVu{
			ID:                   id,
			NhiemVuID:            n.ID,
			NguoiMa:              actor.ID,
			ThoiDiem:             now,
			TrangThaiTaiThoiDiem: n.TrangThai,
			BoPhanID:             n.BoPhanID,
			NguoiPhuTrachMa:      n.NguoiThucHienMa,
			NoiDung:              text,
		}
		if err := uc.kho.GhiNhatKy(ctx, tx, row); err != nil {
			return err
		}

		delta, err := json.Marshal(map[string]any{
			"nhat_ky_id":               id,
			"trang_thai_tai_thoi_diem": string(n.TrangThai),
			"do_dai_noi_dung":          utf8.RuneCountInString(text),
			// WHICH DOOR the writer came through — an inspection asks "was this the holder, or somebody
			// related". A code, not a name.
			"quyen_ghi": taskWorkRightCode(right),
		})
		if err != nil {
			return fmt.Errorf("nhiem_vu: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionTaskLogEntry,
			Subject: n.Ma,
			Delta:   delta,
		})
	})
	if err != nil {
		return domain.NhatKyNhiemVu{}, bocNhiemVu(ctx, "ghi nhật ký", err)
	}
	return row, nil
}

// taskWorkRightCode names the right in the trail: `day-du` (holder or `task.update`) or `ghi-nhat-ky`
// (related person). TaskWorkNone never reaches a write.
func taskWorkRightCode(r domain.TaskWorkRight) string {
	if r == domain.TaskWorkFull {
		return "day-du"
	}
	return "ghi-nhat-ky"
}
