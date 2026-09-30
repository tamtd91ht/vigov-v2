package app

// Creating a task FROM a petition — POST /api/v1/citizen-reports/{maTraCuu}/tasks (user decision
// 30/09/2026; docs/ui-ux/09 §13, 02 §57 "Từ phản ánh").
//
// # WHY A DOOR OF ITS OWN
//
// POST /api/v1/tasks refuses `source = phan-anh` since commit 2d34eba4: it took any `source_id` and
// checked none of it — not that the petition exists, not that it is live, not that it is this
// commune's, not that the caller may even see it (a `can-bo` petition is a report about a member of
// staff). This door names the petition in the PATH, reads it through the scoped store, and sets the
// source pair itself — the body carries no source field at all.
//
// # ONE CREATE PATH, ONE TRANSACTION
//
// The task is booked by GhiNhiemVu.CreateFromSource — the same number minting, tree rules, deadline,
// first timeline row and `tao_nhiem_vu` audit entry as every other door. This file adds the two steps
// only the petition knows:
//
//	Check   the petition re-read FOR UPDATE inside the task's transaction — live, this commune's,
//	        visible to this caller, not closed — before anything is written
//	Record  the petition's timeline row (`tao-nhiem-vu`) and its own audit entry, after the task row
//
// So the task, both timeline rows and both audit entries commit or roll back together (rule 2,
// invariant 6; rule 6, invariant 3).
//
// # TWO AUDIT ENTRIES, AND WHY THAT IS NOT COUNTING ONE ACT TWICE
//
// The meeting split writes one (`tao_nhiem_vu`) because it writes nothing on the meeting register.
// Here the petition's own record IS written — its timeline gains a row — and rule 6, invariant 1 wants
// an entry for that write, as GhiChuNoiBo files one for a note. The second entry's subject is the
// petition's LOOKUP CODE, which is also what lets the audit screen withhold it for a `can-bo`
// petition (petstore.RestrictedPetitionAuditSubjects); putting the code into the task entry's delta
// instead would have shown it to every `admin.audit` reader.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// AuditActionTaskFromPetition is the petition-side verb. Vietnamese snake_case like every verb an
// inspection reads (ADR 0011: values are not translated).
const AuditActionTaskFromPetition = "tao_nhiem_vu_tu_phan_anh"

// PetitionTaskStore is the petition register as this act needs it. *petstore.PhieuPhanAnhStore
// satisfies it as it is — the method names are that store's existing ones.
type PetitionTaskStore interface {
	TheoMaTraCuu(ctx context.Context, ma string) (domain.PhieuPhanAnh, error) // vi-name-ok: existing petstore method
	// vi-name-ok: existing petstore method (locking read)
	TheoMaTraCuuDeSua(ctx context.Context, tx *store.ScopedTx, ma string) (domain.PhieuPhanAnh, error)
	GhiNhatKy(ctx context.Context, tx *store.ScopedTx, e domain.NhatKyPhanAnh) error // vi-name-ok: existing petstore method
}

// TaskFromSourceCreator is the ONE act borrowed from the task register. *GhiNhiemVu satisfies it; a
// one-method interface so this file cannot move, edit or remove a task.
type TaskFromSourceCreator interface {
	CreateFromSource(ctx context.Context, yc YeuCauTaoNhiemVu, nguoi audit.Actor,
		steps SourceSteps) (domain.NhiemVu, error)
}

// PetitionTaskCreation owns the act.
type PetitionTaskCreation struct {
	petitions PetitionTaskStore
	tasks     TaskFromSourceCreator

	// newID mints the timeline row's id. A seam so a test can pin it; ulid.Moi in production.
	newID func() (string, error)
}

func NewPetitionTaskCreation(petitions PetitionTaskStore, tasks TaskFromSourceCreator) *PetitionTaskCreation {
	return &PetitionTaskCreation{petitions: petitions, tasks: tasks, newID: ulid.Moi}
}

// CreateTask books one task whose source is the petition `code`. Route permissions: `task.create` AND
// `feedback.read`; `restricted` is the `feedback.restricted` FACT, decided here (duocChamPhieuHanChe).
//
// EVERY "YOU MAY NOT SEE IT" IS petstore.ErrPhieuKhongTonTai OR ErrPhieuHanChe, and the handler answers
// both with the one 404 an unknown code gets: unknown, another commune's (the scoped read binds the
// commune from the context), soft-deleted, and a `can-bo` petition without the key.
//
// THE FIRST READ IS UNLOCKED AND ONLY GIVES THE ID the source pair needs before the transaction opens
// (and refuses early, readably). The DECISION is the second read, FOR UPDATE, inside the task's
// transaction: between the two another officer can close, reclassify into `can-bo`, or remove the
// petition, and the locked read is what makes the task lose that race rather than land on it.
func (uc *PetitionTaskCreation) CreateTask(ctx context.Context, code string, yc YeuCauTaoNhiemVu,
	actor audit.Actor, restricted QuyenXemHanChe) (domain.NhiemVu, error) {

	if err := coCanBoThucHien(actor); err != nil {
		return domain.NhiemVu{}, err
	}
	first, err := uc.petitions.TheoMaTraCuu(ctx, code)
	if err != nil {
		return domain.NhiemVu{}, bocPhieu(ctx, "tạo nhiệm vụ từ phiếu", err)
	}
	// RESTRICTED FIELD BEFORE STATUS: a closed `can-bo` petition must answer the 404, not a 409 that
	// confirms it exists.
	if err := duocChamPhieuHanChe(first, restricted); err != nil {
		return domain.NhiemVu{}, err
	}
	if err := domain.PetitionAcceptsTask(first); err != nil {
		return domain.NhiemVu{}, err
	}

	// SET HERE, UNCONDITIONALLY. The handler's body has no source field and refuses one that is sent;
	// this is the second wall, for any other caller.
	yc.NguonGiao = string(domain.NguonPhanAnh)
	yc.NguonID = first.ID

	var locked domain.PhieuPhanAnh
	check := func(ctx context.Context, tx *store.ScopedTx) error {
		current, err := uc.petitions.TheoMaTraCuuDeSua(ctx, tx, code)
		if err != nil {
			return err
		}
		if current.ID != first.ID {
			// Defensive: a lookup code is immutable (migration 0004's trigger), so this cannot happen.
			return petstore.ErrPhieuKhongTonTai
		}
		if err := duocChamPhieuHanChe(current, restricted); err != nil {
			return err
		}
		if err := domain.PetitionAcceptsTask(current); err != nil {
			return err
		}
		locked = current
		return nil
	}

	record := func(ctx context.Context, tx *store.ScopedTx, n domain.NhiemVu) error {
		if actor.ID == "" {
			// Second wall, as ghiNhatKy has (rule 6, invariant 8): no row naming nobody.
			return errors.New(loiThieuChuThe)
		}
		id, err := uc.newID()
		if err != nil {
			return fmt.Errorf("nhat_ky_phan_anh: sinh mã nội bộ: %w", err)
		}
		// THE TASK'S OWN INSTANT, so the two timelines agree on when it happened.
		if err := uc.petitions.GhiNhatKy(ctx, tx, domain.NhatKyPhanAnh{
			ID:             id,
			PhieuPhanAnhID: locked.ID,
			ThoiDiem:       n.TaoLuc,
			NguoiMa:        actor.ID,
			HanhVi:         domain.LogActionTaskCreated,
			TrangThai:      locked.TrangThai,
			NoiDung:        domain.TaskCreatedLogText(n.Ma),
		}); err != nil {
			return err
		}
		// Codes and ids only — no title, no description: both are free text that may quote the reporter,
		// and audit_log is permanent (rule 6, forbidden #4).
		delta, err := json.Marshal(map[string]any{
			"nhat_ky_id":               id,
			"ma_nhiem_vu":              n.Ma,
			"trang_thai_tai_thoi_diem": string(locked.TrangThai),
		})
		if err != nil {
			return fmt.Errorf("petition_task: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  AuditActionTaskFromPetition,
			Subject: locked.MaTraCuu,
			Delta:   delta,
		})
	}

	// The error is returned AS IT IS (already wrapped by bocNhiemVu): the handler maps the petition's
	// refusals first and hands every other one to the task register's own mapping.
	return uc.tasks.CreateFromSource(ctx, yc, actor, SourceSteps{Check: check, Record: record})
}
