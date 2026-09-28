package app

// The use cases behind the WRITE surface of the map field schema (docs/ui-ux/14-cau-hinh.md §6).
//
// Same structure as danh_muc_loai_tai_nguyen_ban_do.go and for the same two reasons: the audit
// entry must share the business write's transaction (rule 6, invariant 3), and every write is
// read-decide-write under a row lock.
//
// NO ASSET REGISTER EXISTS YET (`doi_tuong_ban_do`). These fields describe values nobody can store
// today, so no rule here can consult stored data — which is why option removal is refused outright
// rather than "when unused", and why `is_required` false -> true is accepted without a check it
// will need the day the register exists: every asset already filed without that value becomes
// invalid on its next edit.
//
// WHAT THIS FILE DOES NOT DO: seed any field or any asset type. The group list is undecided (8 vs
// 11, two spellings — migrations/0003, REASON TWO), and commune onboarding does not exist.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// MapFieldSchemaRepo is the store, declared at the point of use. Every method takes the
// transaction, so there is no signature that writes the row outside the one its entry is in.
type MapFieldSchemaRepo interface {
	AssetTypeLive(ctx context.Context, tx *store.ScopedTx, code string) (bool, error)
	FieldCodeState(ctx context.Context, tx *store.ScopedTx, assetTypeCode, fieldCode string) (live, retired bool, err error)
	CountLive(ctx context.Context, tx *store.ScopedTx) (int, error)
	Insert(ctx context.Context, tx *store.ScopedTx, m domain.MapFieldSchema) error
	ByIDForUpdate(ctx context.Context, tx *store.ScopedTx, id string) (domain.MapFieldSchema, error)
	Update(ctx context.Context, tx *store.ScopedTx, m domain.MapFieldSchema) error
	SoftDelete(ctx context.Context, tx *store.ScopedTx, id, by, reason string) error
}

// MapFieldSchemas owns adding, editing and retiring one commune's map fields.
type MapFieldSchemas struct {
	db   *store.DB
	repo MapFieldSchemaRepo

	// newID is injected so a test can pin the id. In production it is ulid.Moi.
	newID func() (string, error)
}

func NewMapFieldSchemas(db *store.DB, repo MapFieldSchemaRepo) *MapFieldSchemas {
	return &MapFieldSchemas{db: db, repo: repo, newID: ulid.Moi}
}

// The business verbs written into the trail. VALUES, not identifiers: Vietnamese snake_case like
// every other action this service writes (`them_loai_tai_nguyen_ban_do`), because an inspection
// reads them. They name the catalogue so the trail answers which list changed without a join.
const (
	ActionCreateMapField = "them_truong_ban_do"
	ActionUpdateMapField = "sua_truong_ban_do"
	ActionDeleteMapField = "xoa_truong_ban_do"
)

// ErrMissingDeleter — the actor carries no business code. `deleted_by` with nothing in it is a
// deletion nobody can be asked about.
var ErrMissingDeleter = errors.New("map_field_schema: thiếu người xoá")

// CreateMapFieldRequest is one new field, as it arrives from the handler. No IsActive: a new field
// is in use (see store.Insert).
type CreateMapFieldRequest struct {
	AssetTypeCode string
	FieldCode     string
	Label         string
	ValueType     string
	Options       []domain.FieldOption
	IsRequired    bool
	SortOrder     int
}

// UpdateMapFieldRequest is a PARTIAL edit: nil means "leave this alone". Pointers because three
// fields have a meaningful zero (sort_order 0, is_required false, is_active false) and Options nil
// must differ from "no options".
//
// No AssetTypeCode, FieldCode or ValueType: all three are immutable and the handler refuses a body
// naming them before this is built.
type UpdateMapFieldRequest struct {
	Label      *string
	Options    *[]domain.FieldOption
	IsRequired *bool
	SortOrder  *int
	IsActive   *bool
}

func wrapMapField(ctx context.Context, op string, err error) error {
	return fmt.Errorf("map_field_schema: %s cho xã %s: %w", op, tenant.MustFrom(ctx), err)
}

// Create adds one field to one asset type's form.
//
// Order of refusals: shape (no lock), then — inside the transaction — the type must be a live row
// of THIS commune's catalogue, the ceiling, then the key. The retired-key refusal is its own error
// so the caller is told why a key nowhere on the screen is taken.
func (uc *MapFieldSchemas) Create(ctx context.Context, req CreateMapFieldRequest,
	actor audit.Actor) (domain.MapFieldSchema, error) {

	typeCode, err := domain.ChuanHoaMa(req.AssetTypeCode)
	if err != nil {
		return domain.MapFieldSchema{}, err
	}
	key, err := domain.NormalizeFieldCode(req.FieldCode)
	if err != nil {
		return domain.MapFieldSchema{}, err
	}
	label, err := domain.NormalizeFieldLabel(req.Label)
	if err != nil {
		return domain.MapFieldSchema{}, err
	}
	if !domain.ValidValueType(req.ValueType) {
		return domain.MapFieldSchema{}, domain.ErrValueTypeUnknown
	}
	opts, err := domain.NormalizeOptions(req.ValueType, req.Options)
	if err != nil {
		return domain.MapFieldSchema{}, err
	}
	if err := domain.ValidateFieldSortOrder(req.SortOrder); err != nil {
		return domain.MapFieldSchema{}, err
	}

	id, err := uc.newID()
	if err != nil {
		return domain.MapFieldSchema{}, fmt.Errorf("map_field_schema: sinh id: %w", err)
	}
	row := domain.MapFieldSchema{
		ID: id, AssetTypeCode: typeCode, FieldCode: key, Label: label,
		ValueType: req.ValueType, Options: opts,
		IsRequired: req.IsRequired, SortOrder: req.SortOrder, IsActive: true,
	}

	// Every repo call below runs on the ScopedTx that uc.db.For(ctx).Tx opens here: the commune is
	// taken from ctx once, and each statement binds it as $1 from tx.TenantID().
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		ok, err := uc.repo.AssetTypeLive(ctx, tx, typeCode)
		if err != nil {
			return err
		}
		if !ok {
			return commsstore.ErrAssetTypeMissing
		}
		n, err := uc.repo.CountLive(ctx, tx)
		if err != nil {
			return err
		}
		if n >= commsstore.MapFieldSchemaCeiling {
			return commsstore.ErrMapFieldSchemaFull
		}
		live, retired, err := uc.repo.FieldCodeState(ctx, tx, typeCode, key)
		if err != nil {
			return err
		}
		switch {
		case live:
			return commsstore.ErrFieldCodeTaken
		case retired:
			return commsstore.ErrFieldCodeRetired
		}
		// Scoped: tx comes from uc.db.For(ctx).Tx — tenant_id is $1 of the INSERT.
		if err := uc.repo.Insert(ctx, tx, row); err != nil {
			return err
		}

		delta, err := json.Marshal(map[string]any{"sau": summarizeMapField(row)})
		if err != nil {
			return fmt.Errorf("map_field_schema: mã hoá delta: %w", err)
		}
		// SAME TRANSACTION AS THE INSERT (rule 6, invariant 3). TenantID filled by audit.Write from
		// the transaction. Nothing in the delta is personal data: a field definition describes a
		// form, not a person.
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionCreateMapField,
			Subject: row.Subject(),
			Delta:   delta,
		})
	})
	if err != nil {
		return domain.MapFieldSchema{}, wrapMapField(ctx, "thêm", err)
	}
	return row, nil
}

// Update applies a partial edit. A NO-OP WRITES AND AUDITS NOTHING — which is what makes the route's
// idem.KhongCan declaration true.
func (uc *MapFieldSchemas) Update(ctx context.Context, id string, req UpdateMapFieldRequest,
	actor audit.Actor) (domain.MapFieldSchema, error) {

	if id == "" {
		return domain.MapFieldSchema{}, commsstore.ErrMapFieldSchemaNotFound
	}
	var label string
	if req.Label != nil {
		var err error
		if label, err = domain.NormalizeFieldLabel(*req.Label); err != nil {
			return domain.MapFieldSchema{}, err
		}
	}
	if req.SortOrder != nil {
		if err := domain.ValidateFieldSortOrder(*req.SortOrder); err != nil {
			return domain.MapFieldSchema{}, err
		}
	}

	var after domain.MapFieldSchema
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		before, err := uc.repo.ByIDForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		after = before
		if req.Label != nil {
			after.Label = label
		}
		if req.Options != nil {
			// Validated against the row's OWN value type, which is why this is inside the
			// transaction: the type is only known once the row is read.
			opts, err := domain.NormalizeOptions(before.ValueType, *req.Options)
			if err != nil {
				return err
			}
			if err := domain.CheckOptionsKept(before.Options, opts); err != nil {
				return err
			}
			after.Options = opts
		}
		if req.IsRequired != nil {
			after.IsRequired = *req.IsRequired
		}
		if req.SortOrder != nil {
			after.SortOrder = *req.SortOrder
		}
		if req.IsActive != nil {
			after.IsActive = *req.IsActive
		}

		if sameMapField(before, after) {
			return nil
		}
		// Scoped: tx comes from uc.db.For(ctx).Tx — tenant_id is $1 of the UPDATE.
		if err := uc.repo.Update(ctx, tx, after); err != nil {
			return err
		}
		delta, err := json.Marshal(map[string]any{
			"truoc": diffMapField(before, after, true),
			"sau":   diffMapField(before, after, false),
		})
		if err != nil {
			return fmt.Errorf("map_field_schema: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionUpdateMapField,
			Subject: after.Subject(),
			Delta:   delta,
		})
	})
	if err != nil {
		return domain.MapFieldSchema{}, wrapMapField(ctx, "sửa", err)
	}
	return after, nil
}

// Delete soft deletes one field. The row stays with deleted_at · deleted_by · delete_reason, its
// key stays taken forever, and values assets stored under it stay where they are.
func (uc *MapFieldSchemas) Delete(ctx context.Context, id, rawReason string, actor audit.Actor) error {
	if id == "" {
		return commsstore.ErrMapFieldSchemaNotFound
	}
	reason, err := domain.ChuanHoaLyDoXoa(rawReason)
	if err != nil {
		return err
	}
	if actor.ID == "" {
		return ErrMissingDeleter
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		before, err := uc.repo.ByIDForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		// deleted_by is the actor's BUSINESS CODE — the handler builds audit.Actor.ID from
		// Principal.Ma (rule 6, invariant 8), so the column and the entry name the same person.
		if err := uc.repo.SoftDelete(ctx, tx, before.ID, actor.ID, reason); err != nil {
			return err
		}
		delta, err := json.Marshal(map[string]any{
			"truoc":   summarizeMapField(before),
			"ly_do":   reason,
			"xoa_mem": true,
		})
		if err != nil {
			return fmt.Errorf("map_field_schema: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionDeleteMapField,
			Subject: before.Subject(),
			Delta:   delta,
		})
	})
	if err != nil {
		return wrapMapField(ctx, "xoá", err)
	}
	return nil
}

func optionsForAudit(opts []domain.FieldOption) []map[string]string {
	out := make([]map[string]string, 0, len(opts))
	for _, o := range opts {
		out = append(out, map[string]string{"value": o.Value, "label": o.Label})
	}
	return out
}

func summarizeMapField(m domain.MapFieldSchema) map[string]any {
	return map[string]any{
		"asset_type_code": m.AssetTypeCode,
		"field_code":      m.FieldCode,
		"label":           m.Label,
		"value_type":      m.ValueType,
		"options":         optionsForAudit(m.Options),
		"is_required":     m.IsRequired,
		"sort_order":      m.SortOrder,
		"is_active":       m.IsActive,
	}
}

// diffMapField returns only the fields that moved, from the side asked for (rule 6, invariant 5).
func diffMapField(before, after domain.MapFieldSchema, beforeSide bool) map[string]any {
	out := map[string]any{}
	if before.Label != after.Label {
		out["label"] = chon(beforeSide, before.Label, after.Label)
	}
	if !domain.SameOptions(before.Options, after.Options) {
		out["options"] = chon(beforeSide, optionsForAudit(before.Options), optionsForAudit(after.Options))
	}
	if before.IsRequired != after.IsRequired {
		out["is_required"] = chon(beforeSide, before.IsRequired, after.IsRequired)
	}
	if before.SortOrder != after.SortOrder {
		out["sort_order"] = chon(beforeSide, before.SortOrder, after.SortOrder)
	}
	if before.IsActive != after.IsActive {
		out["is_active"] = chon(beforeSide, before.IsActive, after.IsActive)
	}
	return out
}

func sameMapField(a, b domain.MapFieldSchema) bool {
	return a.Label == b.Label && domain.SameOptions(a.Options, b.Options) &&
		a.IsRequired == b.IsRequired && a.SortOrder == b.SortOrder && a.IsActive == b.IsActive
}
