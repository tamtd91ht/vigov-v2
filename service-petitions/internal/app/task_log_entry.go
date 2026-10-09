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
// # ATTACHMENTS (`📎 Đính kèm`, migration 0021)
//
// An entry may carry files the SAME officer uploaded for the SAME task and completed beforehand
// (task_attachment.go). They are linked IN THIS TRANSACTION, after the row exists — migration 0021's
// trigger refuses a link to an entry another transaction wrote, because the log is append-only and a
// later link would be an edit. The candidates are read FOR UPDATE and checked here first
// (domain.CheckAttachable), so a wrong id is a 400 with a sentence rather than the trigger's 500.
// `nhat_ky_nhiem_vu.dinh_kem` is NOT written: the link table is the one source (migration 0021).
//
// # MENTIONS (`mentioned_staff_codes`, ADR 0086 kind 21 `nhiem-vu.nhac-ten`)
//
// An entry may name colleagues. The codes are shape-checked (domain.NormaliseMentions) and then asked of
// identity BEFORE the transaction — the same ResolveAssignableStaff check a hand-over uses, in the
// commune the context carries — so a code of another commune, unknown or locked is ONE refusal (400,
// no existence leak, rule 1) and identity down is 503 with nothing written. Inside the transaction ONE
// staff-notice outbox row names every mentioned code but the author's, keyed by the entry's id. The
// codes go into the audit delta (staff business codes, not personal data); the text does not.
//
// NO POLICY CALL ON THIS PATH, on purpose: platform's per-task count is enforced when each upload is
// ISSUED and again when it is COMPLETED, and a file can be linked only once, so the set an entry can
// carry is already bounded by it. Asking platform again here would make writing a log entry fail
// whenever platform does, for a limit that cannot be exceeded at this point.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// LogAttachmentLinker is the part of *store.StoredFileStore the log entry needs.
type LogAttachmentLinker interface {
	AttachCandidates(ctx context.Context, tx *store.ScopedTx, ids []string) (
		map[string]domain.AttachCandidate, error)
	LinkToLogEntry(ctx context.Context, tx *store.ScopedTx, logEntryID string, fileIDs []string) error
}

// errAttachmentsNotWired: an entry with attachments reached a use case built without the file store.
// A wiring fault (500), never a reason to drop the files silently and write the text alone.
var errAttachmentsNotWired = errors.New("nhat_ky_nhiem_vu: chưa nối kho tệp đính kèm")

// ErrMentionStaffInvalid refuses a mentioned code identity did not answer as an active staff member of
// THIS commune. ONE error for unknown / locked / another commune, and the code is not echoed (rule 1).
var ErrMentionStaffInvalid = errors.New("nhat_ky_nhiem_vu: người được nhắc tên không phải cán bộ đang làm việc của xã")

// ErrMentionStaffUnchecked means identity could not be asked, so the entry was NOT written. Retryable (503).
var ErrMentionStaffUnchecked = errors.New("nhat_ky_nhiem_vu: chưa kiểm được người được nhắc tên")

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
func (uc *GhiNhiemVu) AddLogEntry(ctx context.Context, ma, text string, attachmentIDs, mentionedCodes []string,
	actor audit.Actor, update TaskUpdateRight) (domain.NhatKyNhiemVu, []domain.TaskLogAttachment, error) {

	text, err := domain.KiemNoiDungNhatKy(text)
	if err != nil {
		return domain.NhatKyNhiemVu{}, nil, err
	}
	if err := coCanBoThucHien(actor); err != nil {
		return domain.NhatKyNhiemVu{}, nil, err
	}
	if err := domain.CheckAttachmentList(attachmentIDs); err != nil {
		return domain.NhatKyNhiemVu{}, nil, err
	}
	if len(attachmentIDs) > 0 && uc.files == nil {
		return domain.NhatKyNhiemVu{}, nil, errAttachmentsNotWired
	}
	mentions, err := domain.NormaliseMentions(mentionedCodes)
	if err != nil {
		return domain.NhatKyNhiemVu{}, nil, err
	}
	if err := uc.checkMentionedStaff(ctx, mentions); err != nil {
		return domain.NhatKyNhiemVu{}, nil, err
	}

	now := uc.nayHoac()
	var (
		row      domain.NhatKyNhiemVu
		attached []domain.TaskLogAttachment
	)

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		n, err := uc.kho.TheoMaDeSua(ctx, tx, ma)
		if err != nil {
			return err
		}
		right := domain.TaskWorkRightFor(n, actor.ID, bool(update))
		if err := domain.CheckMayWriteLogEntry(right); err != nil {
			return err
		}
		// The files, read under lock and checked BEFORE the entry is written, so a refusal writes nothing.
		attached = make([]domain.TaskLogAttachment, 0, len(attachmentIDs))
		if len(attachmentIDs) > 0 {
			cands, err := uc.files.AttachCandidates(ctx, tx, attachmentIDs)
			if err != nil {
				return err
			}
			for _, fid := range attachmentIDs {
				c, ok := cands[fid]
				if !ok {
					return domain.ErrAttachmentNotUsable
				}
				if err := domain.CheckAttachable(c, n.ID, actor.ID); err != nil {
					return err
				}
				attached = append(attached, domain.TaskLogAttachment{
					FileID: fid, OriginalName: c.File.OriginalName, MIMEType: c.File.MIMEType,
					SizeBytes: c.File.SizeBytes, Status: c.File.Status,
				})
			}
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
		if len(attachmentIDs) > 0 {
			if err := uc.files.LinkToLogEntry(ctx, tx, id, attachmentIDs); err != nil {
				return err
			}
			for i := range attached {
				attached[i].LogEntryID = id
			}
		}

		delta, err := json.Marshal(map[string]any{
			"nhat_ky_id":               id,
			"trang_thai_tai_thoi_diem": string(n.TrangThai),
			"do_dai_noi_dung":          utf8.RuneCountInString(text),
			// WHICH DOOR the writer came through — an inspection asks "was this the holder, or somebody
			// related". A code, not a name.
			"quyen_ghi": taskWorkRightCode(right),
			// The file IDS, never their names (personal data when they describe a case — rule 3).
			"tep_dinh_kem": attachmentIDs,
			// STAFF business codes of the colleagues named (kind 21) — not personal data (rule 3 is about
			// the people a commune serves). Always present, `[]` for none, so one key answers "who was named".
			"nhac_ten": mentions,
		})
		if err != nil {
			return fmt.Errorf("nhiem_vu: mã hoá delta: %w", err)
		}
		if err := audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionTaskLogEntry,
			Subject: n.Ma,
			Delta:   delta,
		}); err != nil {
			return err
		}
		// KIND 21 — one row for the entry, every mentioned colleague but the author (writeStaffNotice).
		return writeStaffNotice(ctx, tx, uc.notices, uc.noticeID,
			domain.TaskMentionNotice(n, id, text, mentions), actor.ID, now)
	})
	if err != nil {
		return domain.NhatKyNhiemVu{}, nil, bocNhiemVu(ctx, "ghi nhật ký", err)
	}
	return row, attached, nil
}

// checkMentionedStaff asks identity whether every mentioned code is an active staff member of THIS
// commune — the checker hand-overs use. None mentioned asks nothing. FAIL CLOSED: no checker wired, or
// identity not answering, refuses the entry; a mention is never written unchecked.
func (uc *GhiNhiemVu) checkMentionedStaff(ctx context.Context, codes []string) error {
	err := uc.checkAssignableStaff(ctx, codes)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrAssignmentStaffInvalid):
		return ErrMentionStaffInvalid
	case errors.Is(err, ErrAssignmentStaffUnchecked):
		return fmt.Errorf("%w: %w", ErrMentionStaffUnchecked, err)
	}
	return err
}

// taskWorkRightCode names the right in the trail: `day-du` (holder or `task.update`) or `ghi-nhat-ky`
// (related person). TaskWorkNone never reaches a write.
func taskWorkRightCode(r domain.TaskWorkRight) string {
	if r == domain.TaskWorkFull {
		return "day-du"
	}
	return "ghi-nhat-ky"
}
