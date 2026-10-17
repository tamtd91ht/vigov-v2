package app

// The configuration of the citizen directory's DYNAMIC ATTRIBUTES (ADR 0101 #1–#5; migration 0036): list,
// add, edit, retire, reorder. Every write and its audit entry share one transaction (rule 6, invariant 3);
// the actor is the staff business code (rule 6, invariant 8). Nothing personal is written to the trail — a
// definition is configuration, and its delta names codes, column names, counts and booleans.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The verbs in the trail (ADR 0011: Vietnamese snake_case).
const (
	ActionAddCitizenField     = "them_thuoc_tinh_danh_ba"
	ActionEditCitizenField    = "sua_thuoc_tinh_danh_ba"
	ActionRetireCitizenField  = "ngung_thuoc_tinh_danh_ba"
	ActionReorderCitizenField = "sap_xep_thuoc_tinh_danh_ba"

	// citizenFieldOrderSubject is the subject of a reorder entry: the act concerns the commune's whole
	// list, not one field, and the codes in their new order are in the delta.
	citizenFieldOrderSubject = "thu_tu_thuoc_tinh_danh_ba"
)

// ErrCitizenFieldLimit — the commune already has MaxCitizenFieldsLive live attributes (ADR 0101 #1).
var ErrCitizenFieldLimit = fmt.Errorf("thuộc tính danh bạ: mỗi xã tối đa %d thuộc tính đang dùng — hãy ngừng dùng bớt trước khi thêm", domain.MaxCitizenFieldsLive)

// CitizenFieldRepo is the definition store, declared at the point of use. *idstore.CitizenFieldStore.
type CitizenFieldRepo interface {
	List(ctx context.Context, includeRetired bool) ([]domain.CitizenField, error)
	Get(ctx context.Context, code string) (domain.CitizenField, bool, error)
	LockAndListLive(ctx context.Context, tx *store.ScopedTx) ([]domain.CitizenField, error)
	Insert(ctx context.Context, tx *store.ScopedTx, f domain.CitizenField) error
	Update(ctx context.Context, tx *store.ScopedTx, f domain.CitizenField) error
	Retire(ctx context.Context, tx *store.ScopedTx, code, by, reason string, at time.Time) error
	SetOrder(ctx context.Context, tx *store.ScopedTx, codes []string, at time.Time) error
}

// CitizenFields owns the attribute configuration use cases.
type CitizenFields struct {
	db     *store.DB
	fields CitizenFieldRepo
	now    func() time.Time
}

func NewCitizenFields(db *store.DB, fields CitizenFieldRepo) *CitizenFields {
	if db == nil || fields == nil {
		panic("app.NewCitizenFields: db and fields are required")
	}
	return &CitizenFields{db: db, fields: fields, now: func() time.Time { return time.Now().UTC() }}
}

// List reads the commune's fields in order; retired ones only for the configuration screen.
func (uc *CitizenFields) List(ctx context.Context, includeRetired bool) ([]domain.CitizenField, error) {
	// The store scopes this read by tenant_id through store.DB.For(ctx).
	return uc.fields.List(ctx, includeRetired)
}

// Create adds one field at the END of the order. idstore.ErrCitizenFieldCodeTaken for a code the commune
// has ever used; ErrCitizenFieldLimit beyond 50 live fields (counted under the commune's lock, inside the
// transaction).
func (uc *CitizenFields) Create(ctx context.Context, actor NguoiThucHien, in domain.CitizenFieldInput) (domain.CitizenField, error) {
	if err := actor.hopLe(); err != nil {
		return domain.CitizenField{}, err
	}
	in, err := domain.CheckNewCitizenField(in)
	if err != nil {
		return domain.CitizenField{}, err
	}
	at := uc.now()
	f := domain.CitizenField{
		Code: in.Code, Label: in.Label, Type: in.Type, Placeholder: in.Placeholder, Options: in.Options,
		NumberKind: in.NumberKind, DatePattern: in.DatePattern, Required: in.Required, ShowInList: in.ShowInList,
		DisplayMode: in.DisplayMode, LayoutWidth: in.LayoutWidth, CreatedAt: at, UpdatedAt: at,
	}
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		live, err := uc.fields.LockAndListLive(ctx, tx)
		if err != nil {
			return err
		}
		if len(live) >= domain.MaxCitizenFieldsLive {
			return ErrCitizenFieldLimit
		}
		for _, l := range live {
			if l.SortOrder >= f.SortOrder {
				f.SortOrder = min(l.SortOrder+1, domain.MaxCitizenFieldSortOrder)
			}
		}
		// Insert binds tx.TenantID() as $1 (rule 1, invariant 5).
		if err := uc.fields.Insert(ctx, tx, f); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor: actor.Vet, Action: ActionAddCitizenField, Subject: f.Code, At: at,
			Delta: deltaSLA(map[string]any{
				"ma": f.Code, "loai": string(f.Type), "bat_buoc": f.Required, "hien_o_danh_sach": f.ShowInList,
				"so_lua_chon": len(f.Options), "thu_tu": f.SortOrder,
				"cach_hien_thi": f.DisplayMode, "do_rong": f.LayoutWidth,
			}),
		})
	})
	if err != nil {
		return domain.CitizenField{}, err
	}
	return f, nil
}

// Update edits one LIVE field. A no-op writes and audits nothing. idstore.ErrCitizenFieldNotFound for an
// unknown or retired code.
func (uc *CitizenFields) Update(ctx context.Context, actor NguoiThucHien, code string, p domain.CitizenFieldPatch) (domain.CitizenField, error) {
	if err := actor.hopLe(); err != nil {
		return domain.CitizenField{}, err
	}
	// The store scopes this read by tenant_id through store.DB.For(ctx).
	cur, found, err := uc.fields.Get(ctx, code)
	if err != nil {
		return domain.CitizenField{}, err
	}
	if !found || !cur.Live() {
		return domain.CitizenField{}, idstore.ErrCitizenFieldNotFound
	}
	next, changed, err := domain.ApplyCitizenFieldPatch(cur, p)
	if err != nil {
		return domain.CitizenField{}, err
	}
	if len(changed) == 0 {
		return cur, nil
	}
	next.UpdatedAt = uc.now()
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// Update binds tx.TenantID() as $1; the 0036 trigger refuses a removed option even in a race.
		if err := uc.fields.Update(ctx, tx, next); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor: actor.Vet, Action: ActionEditCitizenField, Subject: code, At: next.UpdatedAt,
			Delta: deltaSLA(map[string]any{
				"truong_da_doi": changed,
				"truoc": map[string]any{"bat_buoc": cur.Required, "hien_o_danh_sach": cur.ShowInList, "so_lua_chon": len(cur.Options),
					"cach_hien_thi": cur.DisplayMode, "do_rong": cur.LayoutWidth},
				"sau": map[string]any{"bat_buoc": next.Required, "hien_o_danh_sach": next.ShowInList, "so_lua_chon": len(next.Options),
					"cach_hien_thi": next.DisplayMode, "do_rong": next.LayoutWidth},
			}),
		})
	})
	if err != nil {
		return domain.CitizenField{}, err
	}
	return next, nil
}

// Retire hides one live field from the form and the filters, keeping every stored value (ADR 0101 #4).
func (uc *CitizenFields) Retire(ctx context.Context, actor NguoiThucHien, code, reason string) error {
	if err := actor.hopLe(); err != nil {
		return err
	}
	reason, err := domain.NormalizeCitizenFieldRetireReason(reason)
	if err != nil {
		return err
	}
	at := uc.now()
	return uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// retired_by is the STAFF CODE (rule 6, invariant 8) — the entry's actor.
		if err := uc.fields.Retire(ctx, tx, code, actor.Vet.ID, reason, at); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor: actor.Vet, Action: ActionRetireCitizenField, Subject: code, At: at,
			Delta: deltaSLA(map[string]any{"ly_do": reason, "ngung_dung": true}),
		})
	})
}

// Reorder sets the drag-and-drop order (ADR 0101 #3): `codes` must be exactly the live codes, each once,
// checked under the commune's lock. Identical to the stored order: nothing written, nothing audited.
func (uc *CitizenFields) Reorder(ctx context.Context, actor NguoiThucHien, codes []string) error {
	if err := actor.hopLe(); err != nil {
		return err
	}
	at := uc.now()
	return uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		live, err := uc.fields.LockAndListLive(ctx, tx)
		if err != nil {
			return err
		}
		if err := domain.CheckCitizenFieldOrder(live, codes); err != nil {
			return err
		}
		before := make([]string, 0, len(live))
		same := true
		for i, f := range live {
			before = append(before, f.Code)
			if f.Code != codes[i] || f.SortOrder != i {
				same = false
			}
		}
		if same {
			return nil
		}
		if err := uc.fields.SetOrder(ctx, tx, codes, at); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor: actor.Vet, Action: ActionReorderCitizenField, Subject: citizenFieldOrderSubject, At: at,
			Delta: deltaSLA(map[string]any{"truoc": before, "sau": codes}),
		})
	})
}

// IsCitizenFieldInputError reports a refusal of what the client sent (400). Listed explicitly so an outage
// never becomes a 400.
func IsCitizenFieldInputError(err error) bool {
	for _, e := range []error{
		domain.ErrCitizenFieldCodeInvalid, domain.ErrCitizenFieldLabelMissing, domain.ErrCitizenFieldLabelTooLong,
		domain.ErrCitizenFieldTypeInvalid, domain.ErrCitizenFieldImmutable, domain.ErrCitizenFieldPlaceholder,
		domain.ErrCitizenFieldNumberKind, domain.ErrCitizenFieldDatePattern, domain.ErrCitizenFieldOptionsMissing,
		domain.ErrCitizenFieldOptionInvalid, domain.ErrCitizenFieldOptionDuplicate, domain.ErrCitizenFieldOptionRemoved,
		domain.ErrCitizenFieldTooManyOptions, domain.ErrCitizenFieldSortOrderRange, domain.ErrCitizenFieldReasonMissing,
		domain.ErrCitizenFieldReasonTooLong, domain.ErrCitizenFieldOrderMismatch,
		domain.ErrCitizenFieldDisplayMode, domain.ErrCitizenFieldLayoutWidth,
	} {
		if errors.Is(err, e) {
			return true
		}
	}
	return false
}
