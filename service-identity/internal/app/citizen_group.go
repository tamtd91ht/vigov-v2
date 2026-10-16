package app

// The groups of the citizen contact directory ("Nhóm", ADR 0097 D4; chapter 16 §6): a catalogue the
// commune defines, a few dozen rows. The same CitizenDirectory value as citizen_directory.go.
//
// A GROUP NAME IS NOT PERSONAL DATA — "Tổ trưởng dân phố", "Hộ kinh doanh" name a role or a kind,
// not a person — so unlike a contact entry, a group entry may carry before/after values. What is
// NOT in any group entry is who is in the group: membership changes are audited on the CONTACT
// (sua_nguoi_dan_danh_ba, field `groups`), where the subject is the person whose row changed.

import (
	"context"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The verbs in the trail (ADR 0011).
const (
	ActionAddCitizenGroup    = "them_nhom_nguoi_dan"
	ActionEditCitizenGroup   = "sua_nhom_nguoi_dan"
	ActionRemoveCitizenGroup = "go_nhom_nguoi_dan"
)

// CitizenGroupItem is one row of the group tab: the group and its live member count.
type CitizenGroupItem struct {
	domain.CitizenGroup
	MemberCount int
}

// ListGroups reads the commune's live groups in their own order, each with its member count — two
// statements for the whole tab, never one per group.
func (uc *CitizenDirectory) ListGroups(ctx context.Context) ([]CitizenGroupItem, error) {
	// The store scopes this read by tenant_id through store.DB.For(ctx).
	groups, err := uc.groups.List(ctx)
	if err != nil {
		return nil, err
	}
	counts, err := uc.contacts.MemberCounts(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]CitizenGroupItem, 0, len(groups))
	for _, g := range groups {
		out = append(out, CitizenGroupItem{CitizenGroup: g, MemberCount: counts[g.ID]})
	}
	return out, nil
}

// CreateGroup inserts one group and its audit entry in one transaction. idstore.ErrCitizenGroupNameTaken
// when a live group of this commune already carries the (folded) name. Returns the new id.
func (uc *CitizenDirectory) CreateGroup(ctx context.Context, actor NguoiThucHien, in domain.CitizenGroupInput) (string, error) {
	if err := actor.hopLe(); err != nil {
		return "", err
	}
	in, err := domain.CheckCitizenGroupInput(in)
	if err != nil {
		return "", err
	}
	id, err := uc.newID()
	if err != nil {
		return "", err
	}
	at := uc.now()
	g := domain.CitizenGroup{ID: id, Name: in.Name, Description: in.Description, SortOrder: in.SortOrder,
		CreatedAt: at, UpdatedAt: at}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// Inside the transaction, so the check and the insert see the same rows; the live-unique
		// key is the backstop for the race that remains.
		taken, err := uc.groups.NameTaken(ctx, tx, g.Name, "")
		if err != nil {
			return err
		}
		if taken {
			return idstore.ErrCitizenGroupNameTaken
		}
		// Insert binds tx.TenantID() as $1 (rule 1, invariant 5).
		if err := uc.groups.Insert(ctx, tx, g); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor.Vet,
			Action:  ActionAddCitizenGroup,
			Subject: g.ID,
			At:      at,
			Delta:   deltaSLA(map[string]any{"sau": groupTrail(g)}),
		})
	})
	if err != nil {
		return "", err
	}
	return g.ID, nil
}

// UpdateGroup rewrites name, description and order of one live group. A NO-OP WRITES NOTHING AND
// AUDITS NOTHING. The row is read before the transaction (the store has no in-transaction read);
// the UPDATE's `deleted_at IS NULL` refuses a group removed in between.
func (uc *CitizenDirectory) UpdateGroup(ctx context.Context, actor NguoiThucHien, id string, in domain.CitizenGroupInput) error {
	if err := actor.hopLe(); err != nil {
		return err
	}
	if id == "" {
		return idstore.ErrCitizenGroupNotFound
	}
	in, err := domain.CheckCitizenGroupInput(in)
	if err != nil {
		return err
	}
	// The store scopes this read by tenant_id through store.DB.For(ctx).
	before, found, err := uc.groups.Get(ctx, id)
	if err != nil {
		return err
	}
	if !found {
		return idstore.ErrCitizenGroupNotFound
	}
	after := before
	after.Name, after.Description, after.SortOrder = in.Name, in.Description, in.SortOrder
	if after.Name == before.Name && after.Description == before.Description && after.SortOrder == before.SortOrder {
		return nil
	}
	at := uc.now()
	after.UpdatedAt = at

	return uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		if after.Name != before.Name {
			taken, err := uc.groups.NameTaken(ctx, tx, after.Name, id)
			if err != nil {
				return err
			}
			if taken {
				return idstore.ErrCitizenGroupNameTaken
			}
		}
		// Update binds tx.TenantID() as $1 (rule 1, invariant 5).
		if err := uc.groups.Update(ctx, tx, after); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor.Vet,
			Action:  ActionEditCitizenGroup,
			Subject: id,
			At:      at,
			Delta:   deltaSLA(map[string]any{"truoc": groupTrail(before), "sau": groupTrail(after)}),
		})
	})
}

// RemoveGroup soft deletes one group. The reason is OPTIONAL (ADR 0097 D7 — removing a group only
// tidies a catalogue); members stay in the directory, their membership rows are kept (store).
func (uc *CitizenDirectory) RemoveGroup(ctx context.Context, actor NguoiThucHien, id, reason string) error {
	if err := actor.hopLe(); err != nil {
		return err
	}
	if id == "" {
		return idstore.ErrCitizenGroupNotFound
	}
	reason, err := domain.NormalizeCitizenGroupDeleteReason(reason)
	if err != nil {
		return err
	}
	at := uc.now()
	return uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// deleted_by is the STAFF CODE (rule 6, invariant 8) — the same value as the entry's actor.
		if err := uc.groups.SoftDelete(ctx, tx, id, actor.Vet.ID, reason, at); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor.Vet,
			Action:  ActionRemoveCitizenGroup,
			Subject: id,
			At:      at,
			Delta:   deltaSLA(map[string]any{"ly_do": reason, "xoa_mem": true}),
		})
	})
}

// groupTrail is the before/after payload of a group entry — the three editable columns, none of
// them about a person.
func groupTrail(g domain.CitizenGroup) map[string]any {
	return map[string]any{"name": g.Name, "description": g.Description, "sort_order": g.SortOrder}
}
