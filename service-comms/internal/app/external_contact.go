package app

// The use cases behind the WRITE surface of the commune's external contacts (`liên hệ ngoài bộ máy xã`,
// migration 0014; staff edit them in a tab of /danh-ba, user decision 03/10/2026).
//
// Same structure as map_field_schema.go and for the same two reasons: the audit entry shares the
// business write's transaction (rule 6, invariant 3), and every edit is read-decide-write under a row
// lock.
//
// PERSONAL DATA (rule 3, rule 6 forbidden #4). The intended content is institutional, but nothing stops
// a commune typing an officer's own mobile into `phone` (0014, PERSONAL DATA). So the audit delta
// carries `phone` MASKED (privacy.MaskPhone) and `address` NOT AT ALL — only whether one is set, and on
// an edit whether it changed. There is no masking function for an address in core/privacy, and a
// truncated address is still an address.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/privacy"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// ExternalContactRepo is the store, declared at the point of use. Every write takes the transaction,
// so there is no signature that writes the row outside the one its entry is in.
type ExternalContactRepo interface {
	CountLive(ctx context.Context, tx *store.ScopedTx) (int, error)
	Insert(ctx context.Context, tx *store.ScopedTx, c domain.ExternalContact, by string) error
	ByIDForUpdate(ctx context.Context, tx *store.ScopedTx, id string) (domain.ExternalContact, error)
	Update(ctx context.Context, tx *store.ScopedTx, c domain.ExternalContact, by string) error
	SoftDelete(ctx context.Context, tx *store.ScopedTx, id, by, reason string) error
}

// ExternalContacts owns adding, editing and retiring one commune's external contacts.
type ExternalContacts struct {
	db   *store.DB
	repo ExternalContactRepo

	// newID is injected so a test can pin the id. In production it is ulid.Moi.
	newID func() (string, error)
}

func NewExternalContacts(db *store.DB, repo ExternalContactRepo) *ExternalContacts {
	return &ExternalContacts{db: db, repo: repo, newID: ulid.Moi}
}

// The business verbs written into the trail. VALUES, not identifiers: Vietnamese snake_case like every
// other action this service writes (ADR 0011), naming the register so the trail answers which list
// changed without a join.
const (
	ActionCreateExternalContact = "them_lien_he_ngoai_bo_may"
	ActionUpdateExternalContact = "sua_lien_he_ngoai_bo_may"
	ActionDeleteExternalContact = "xoa_lien_he_ngoai_bo_may"
)

// ErrMissingActor — the actor carries no business code. `created_by` / `updated_by` / `deleted_by`
// with nothing in them is a change nobody can be asked about (rule 6, invariant 8: no fallback).
var ErrMissingActor = errors.New("external_contact: thiếu mã cán bộ thực hiện")

// ExternalContactInput is one new contact, as it arrives from the handler.
type ExternalContactInput struct {
	Name         string
	Category     string
	Phone        string
	Address      string
	DisplayOrder *int
}

// ExternalContactPatch is a PARTIAL edit: nil means "leave this alone". Address "" CLEARS the address
// (the house convention for a nullable text on PATCH — content-items' `link_to`). DisplayOrder has no
// "clear": a JSON null cannot be told from an absent key through a pointer, the same limit
// content-items' `display_order` states.
type ExternalContactPatch struct {
	Name         *string
	Category     *string
	Phone        *string
	Address      *string
	DisplayOrder *int
}

func wrapExternalContact(ctx context.Context, op string, err error) error {
	return fmt.Errorf("external_contact: %s cho xã %s: %w", op, tenant.MustFrom(ctx), err)
}

// normalizeExternalContact applies every shape check of migration 0014 to a whole row.
func normalizeExternalContact(c domain.ExternalContact) (domain.ExternalContact, error) {
	var err error
	if c.Name, err = domain.NormalizeExternalContactName(c.Name); err != nil {
		return domain.ExternalContact{}, err
	}
	if c.Category, err = domain.NormalizeExternalContactCategory(c.Category); err != nil {
		return domain.ExternalContact{}, err
	}
	if c.Phone, err = domain.NormalizeExternalContactPhone(c.Phone); err != nil {
		return domain.ExternalContact{}, err
	}
	if c.Address, err = domain.NormalizeExternalContactAddress(c.Address); err != nil {
		return domain.ExternalContact{}, err
	}
	if err := domain.CheckExternalContactDisplayOrder(c.DisplayOrder); err != nil {
		return domain.ExternalContact{}, err
	}
	return c, nil
}

// Create adds one contact. Shape first (no transaction), then — inside it — the ceiling, the row and
// its entry.
func (uc *ExternalContacts) Create(ctx context.Context, in ExternalContactInput, actor audit.Actor) (domain.ExternalContact, error) {
	if actor.ID == "" {
		return domain.ExternalContact{}, ErrMissingActor
	}
	row, err := normalizeExternalContact(domain.ExternalContact{
		Name: in.Name, Category: in.Category, Phone: in.Phone, Address: in.Address, DisplayOrder: in.DisplayOrder,
	})
	if err != nil {
		return domain.ExternalContact{}, err
	}
	if row.ID, err = uc.newID(); err != nil {
		return domain.ExternalContact{}, fmt.Errorf("external_contact: sinh id: %w", err)
	}

	// Every repo call below runs on the ScopedTx that uc.db.For(ctx).Tx opens here: the commune is taken
	// from ctx once, and each statement binds it as $1 from tx.TenantID().
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		n, err := uc.repo.CountLive(ctx, tx)
		if err != nil {
			return err
		}
		if n >= commsstore.ExternalContactCeiling {
			return commsstore.ErrExternalContactsFull
		}
		// created_by / updated_by are the actor's BUSINESS CODE: the handler builds audit.Actor.ID from
		// Principal.Ma (rule 6, invariant 8).
		// Scoped: tx comes from uc.db.For(ctx).Tx — tenant_id is $1 of the INSERT.
		if err := uc.repo.Insert(ctx, tx, row, actor.ID); err != nil {
			return err
		}
		delta, err := json.Marshal(map[string]any{"sau": externalContactForAudit(row)})
		if err != nil {
			return fmt.Errorf("external_contact: mã hoá delta: %w", err)
		}
		// SAME TRANSACTION AS THE INSERT (rule 6, invariant 3).
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionCreateExternalContact,
			Subject: externalContactSubject(row),
			Delta:   delta,
		})
	})
	if err != nil {
		return domain.ExternalContact{}, wrapExternalContact(ctx, "thêm", err)
	}
	return row, nil
}

// Update applies a partial edit. A NO-OP WRITES AND AUDITS NOTHING — which is what makes the route's
// idem.KhongCan declaration true.
func (uc *ExternalContacts) Update(ctx context.Context, id string, p ExternalContactPatch, actor audit.Actor) (domain.ExternalContact, error) {
	if id == "" {
		return domain.ExternalContact{}, commsstore.ErrExternalContactNotFound
	}
	if actor.ID == "" {
		return domain.ExternalContact{}, ErrMissingActor
	}

	var after domain.ExternalContact
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		before, err := uc.repo.ByIDForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		merged := before
		if p.Name != nil {
			merged.Name = *p.Name
		}
		if p.Category != nil {
			merged.Category = *p.Category
		}
		if p.Phone != nil {
			merged.Phone = *p.Phone
		}
		if p.Address != nil {
			merged.Address = *p.Address
		}
		if p.DisplayOrder != nil {
			n := *p.DisplayOrder
			merged.DisplayOrder = &n
		}
		// Judged on the row AFTER the merge: the stored fields passed the CHECKs, the sent ones must too.
		if after, err = normalizeExternalContact(merged); err != nil {
			return err
		}
		if sameExternalContact(before, after) {
			return nil
		}
		// Scoped: tx comes from uc.db.For(ctx).Tx — tenant_id is $1 of the UPDATE.
		if err := uc.repo.Update(ctx, tx, after, actor.ID); err != nil {
			return err
		}
		beforeSide, afterSide := diffExternalContact(before, after)
		delta, err := json.Marshal(map[string]any{"id": after.ID, "truoc": beforeSide, "sau": afterSide})
		if err != nil {
			return fmt.Errorf("external_contact: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionUpdateExternalContact,
			Subject: externalContactSubject(after),
			Delta:   delta,
		})
	})
	if err != nil {
		return domain.ExternalContact{}, wrapExternalContact(ctx, "sửa", err)
	}
	return after, nil
}

// Delete soft deletes one contact with a mandatory reason. The row stays — deleted_at · deleted_by ·
// delete_reason — and leaves the Mini App in the same statement, because a live row is a public row.
func (uc *ExternalContacts) Delete(ctx context.Context, id, rawReason string, actor audit.Actor) error {
	if id == "" {
		return commsstore.ErrExternalContactNotFound
	}
	reason, err := domain.ChuanHoaLyDoXoa(rawReason)
	if err != nil {
		return err
	}
	if actor.ID == "" {
		return ErrMissingActor
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		before, err := uc.repo.ByIDForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		// Scoped: tx comes from uc.db.For(ctx).Tx — tenant_id is $1 of the UPDATE.
		if err := uc.repo.SoftDelete(ctx, tx, before.ID, actor.ID, reason); err != nil {
			return err
		}
		delta, err := json.Marshal(map[string]any{
			"truoc":   externalContactForAudit(before),
			"ly_do":   reason,
			"xoa_mem": true,
		})
		if err != nil {
			return fmt.Errorf("external_contact: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionDeleteExternalContact,
			Subject: externalContactSubject(before),
			Delta:   delta,
		})
	})
	if err != nil {
		return wrapExternalContact(ctx, "xoá", err)
	}
	return nil
}

// externalContactSubject is the trail's locator. AN EXTERNAL CONTACT HAS NO BUSINESS CODE — 0014 mints
// none — and a ULID names nothing to somebody handling a complaint years later (the argument
// chuDeNoiDungMiniApp makes). So it is the register plus the institution's name, which is what a
// complaint ("the number for the health station was wrong") quotes; the delta carries the id. The name
// is an institution's (0014 treats only `phone` and `address` as possibly personal).
func externalContactSubject(c domain.ExternalContact) string {
	return "lien-he-ngoai-bo-may/" + c.Name
}

// externalContactForAudit is the delta's view of one row: phone MASKED, address as presence only.
func externalContactForAudit(c domain.ExternalContact) map[string]any {
	return map[string]any{
		"id":            c.ID,
		"name":          c.Name,
		"category":      c.Category,
		"phone":         privacy.MaskPhone(c.Phone),
		"has_address":   c.Address != "",
		"display_order": orderOrNil(c.DisplayOrder),
	}
}

// diffExternalContact returns only the fields that moved, one map per side (rule 6, invariant 5) — the
// phone masked on both sides, the address reduced to whether it is set.
func diffExternalContact(before, after domain.ExternalContact) (map[string]any, map[string]any) {
	b, a := map[string]any{}, map[string]any{}
	if before.Name != after.Name {
		b["name"], a["name"] = before.Name, after.Name
	}
	if before.Category != after.Category {
		b["category"], a["category"] = before.Category, after.Category
	}
	if before.Phone != after.Phone {
		b["phone"], a["phone"] = privacy.MaskPhone(before.Phone), privacy.MaskPhone(after.Phone)
	}
	if before.Address != after.Address {
		// Presence is what is safe to keep; "changed" says the rest. Two addresses that both exist would
		// otherwise produce an entry recording no change at all.
		b["has_address"], a["has_address"] = before.Address != "", after.Address != ""
		a["address_changed"] = true
	}
	if !sameOrder(before.DisplayOrder, after.DisplayOrder) {
		b["display_order"], a["display_order"] = orderOrNil(before.DisplayOrder), orderOrNil(after.DisplayOrder)
	}
	return b, a
}

func sameExternalContact(x, y domain.ExternalContact) bool {
	return x.Name == y.Name && x.Category == y.Category && x.Phone == y.Phone && x.Address == y.Address &&
		sameOrder(x.DisplayOrder, y.DisplayOrder)
}
