package app

// The write use cases of "Loại đơn thư" on the Danh mục screen (domain/citizen_letter_type.go says what
// a row is). The shape of danh_muc_loai_van_ban.go, narrowed: every row is the commune's own, so there
// is no tier; there is no default; the code must be one of C4's four.
//
// Rule 6, invariant 3: every business write and its audit entry share ONE transaction, opened here.

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-documents/internal/domain"
	docstore "github.com/vihat/vigov/service-documents/internal/store"
)

// CitizenLetterTypeRepo is the store, declared at the point of use. Every write takes the transaction,
// whose tenant_id the store binds as $1. *docstore.CitizenLetterTypeStore satisfies it.
type CitizenLetterTypeRepo interface {
	ByIDForUpdate(ctx context.Context, tx *store.ScopedTx, id string) (domain.CitizenLetterTypeEntry, error)
	CodeTaken(ctx context.Context, tx *store.ScopedTx, code domain.LetterType) (bool, error)
	AddEntry(ctx context.Context, tx *store.ScopedTx, e domain.CitizenLetterTypeEntry) error
	RewriteEntry(ctx context.Context, tx *store.ScopedTx, e domain.CitizenLetterTypeEntry) error
	SoftDelete(ctx context.Context, tx *store.ScopedTx, id, by, reason string) error
}

// The business verbs in the trail — Vietnamese snake_case like every action this service writes.
const (
	ActionCreateCitizenLetterType = "them_loai_don_thu"
	ActionUpdateCitizenLetterType = "sua_loai_don_thu"
	ActionDeleteCitizenLetterType = "xoa_loai_don_thu"
)

// CitizenLetterTypes owns adding, editing and retiring one commune's letter-type labels.
type CitizenLetterTypes struct {
	db    *store.DB
	repo  CitizenLetterTypeRepo
	newID func() (string, error)
}

func NewCitizenLetterTypes(db *store.DB, repo CitizenLetterTypeRepo) *CitizenLetterTypes {
	return &CitizenLetterTypes{db: db, repo: repo, newID: ulid.Moi}
}

// AddLetterTypeRequest is one new row. No `Active`: a row just added is in use.
type AddLetterTypeRequest struct {
	Code  string
	Label string
	Order int
	Color *string // nil = none chosen
}

// EditLetterTypeRequest is a PARTIAL edit: nil leaves the field alone (order 0 and active false are
// meaningful values, so plain fields could not tell "absent" from "zero").
type EditLetterTypeRequest struct {
	Label  *string
	Order  *int
	Active *bool
	Color  *CatalogueColorChange
}

// AddType adds one row. Shape first (no lock), then the taken-code check inside the transaction.
func (uc *CitizenLetterTypes) AddType(ctx context.Context, req AddLetterTypeRequest,
	actor audit.Actor) (domain.CitizenLetterTypeEntry, error) {

	code, err := domain.NormalizeCitizenLetterTypeCode(req.Code)
	if err != nil {
		return domain.CitizenLetterTypeEntry{}, err
	}
	label, err := domain.ChuanHoaNhan(req.Label)
	if err != nil {
		return domain.CitizenLetterTypeEntry{}, err
	}
	if err := domain.KiemTraThuTu(req.Order); err != nil {
		return domain.CitizenLetterTypeEntry{}, err
	}
	var color string
	if req.Color != nil {
		if color, err = domain.NormalizeCatalogueColor(*req.Color); err != nil {
			return domain.CitizenLetterTypeEntry{}, err
		}
	}
	id, err := uc.newID()
	if err != nil {
		return domain.CitizenLetterTypeEntry{}, fmt.Errorf("citizen_letter_type: sinh mã: %w", err)
	}
	e := domain.CitizenLetterTypeEntry{ID: id, Code: code, Label: label, Order: req.Order, Active: true, Color: color}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		taken, err := uc.repo.CodeTaken(ctx, tx, code)
		if err != nil {
			return err
		}
		if taken {
			return docstore.ErrMaDaTonTai
		}
		if err := uc.repo.AddEntry(ctx, tx, e); err != nil {
			return err
		}
		return writeCitizenLetterTypeAudit(ctx, tx, actor, ActionCreateCitizenLetterType, e.Code,
			map[string]any{"sau": citizenLetterTypeView(e)})
	})
	if err != nil {
		return domain.CitizenLetterTypeEntry{}, boc(ctx, "thêm loại đơn thư", err)
	}
	return e, nil
}

// EditType applies a partial edit. A no-op writes nothing and audits nothing.
func (uc *CitizenLetterTypes) EditType(ctx context.Context, id string, req EditLetterTypeRequest,
	actor audit.Actor) (domain.CitizenLetterTypeEntry, error) {

	if id == "" {
		return domain.CitizenLetterTypeEntry{}, docstore.ErrDanhMucKhongTonTai
	}
	var label string
	if req.Label != nil {
		var err error
		if label, err = domain.ChuanHoaNhan(*req.Label); err != nil {
			return domain.CitizenLetterTypeEntry{}, err
		}
	}
	if req.Order != nil {
		if err := domain.KiemTraThuTu(*req.Order); err != nil {
			return domain.CitizenLetterTypeEntry{}, err
		}
	}
	var color string
	if c := req.Color; c != nil && c.Color != nil {
		var err error
		if color, err = domain.NormalizeCatalogueColor(*c.Color); err != nil {
			return domain.CitizenLetterTypeEntry{}, err
		}
	}

	var after domain.CitizenLetterTypeEntry
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		before, err := uc.repo.ByIDForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		after = before
		if req.Label != nil {
			after.Label = label
		}
		if req.Order != nil {
			after.Order = *req.Order
		}
		if req.Active != nil {
			after.Active = *req.Active
		}
		if req.Color != nil {
			after.Color = color // "" when the change clears it
		}
		if after == before {
			return nil
		}
		if err := uc.repo.RewriteEntry(ctx, tx, after); err != nil {
			return err
		}
		return writeCitizenLetterTypeAudit(ctx, tx, actor, ActionUpdateCitizenLetterType, after.Code,
			map[string]any{"truoc": citizenLetterTypeView(before), "sau": citizenLetterTypeView(after)})
	})
	if err != nil {
		return domain.CitizenLetterTypeEntry{}, boc(ctx, "sửa loại đơn thư", err)
	}
	return after, nil
}

// RemoveType soft deletes one row. A blank reason becomes the fixed sentence (owner decision
// 10/10/2026). The code stays taken (UNIQUE counts deleted rows), so switching off is the reversible act.
func (uc *CitizenLetterTypes) RemoveType(ctx context.Context, id, rawReason string, actor audit.Actor) error {
	if id == "" {
		return docstore.ErrDanhMucKhongTonTai
	}
	reason, err := domain.NormalizeCitizenLetterTypeDeleteReason(rawReason)
	if err != nil {
		return err
	}
	if actor.ID == "" {
		return fmt.Errorf("citizen_letter_type: thiếu người xoá")
	}
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		before, err := uc.repo.ByIDForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := uc.repo.SoftDelete(ctx, tx, before.ID, actor.ID, reason); err != nil {
			return err
		}
		return writeCitizenLetterTypeAudit(ctx, tx, actor, ActionDeleteCitizenLetterType, before.Code,
			map[string]any{"truoc": citizenLetterTypeView(before), "ly_do": reason, "xoa_mem": true})
	})
	if err != nil {
		return boc(ctx, "xoá loại đơn thư", err)
	}
	return nil
}

// writeCitizenLetterTypeAudit files one entry under the letter-type CODE (a business code, never the
// row id). Nothing here is personal data. TenantID is audit.Write's, from the transaction.
func writeCitizenLetterTypeAudit(ctx context.Context, tx *store.ScopedTx, actor audit.Actor, action string,
	code domain.LetterType, delta map[string]any) error {
	raw, err := json.Marshal(delta)
	if err != nil {
		return fmt.Errorf("citizen_letter_type: mã hoá delta: %w", err)
	}
	return audit.Write(ctx, tx, audit.Entry{Actor: actor, Action: action, Subject: string(code), Delta: raw})
}

func citizenLetterTypeView(e domain.CitizenLetterTypeEntry) map[string]any {
	return map[string]any{
		"ma": string(e.Code), "nhan": e.Label, "thu_tu": e.Order, "dang_dung": e.Active, "color": colorDelta(e.Color),
	}
}
