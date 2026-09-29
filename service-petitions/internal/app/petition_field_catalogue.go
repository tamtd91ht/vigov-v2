package app

// The petition field catalogue of one commune: tier 1 from `platform` (ADR 0060) merged with this
// commune's tier-2 overrides (`nhan_linh_vuc`, ADR 0026) — read by staff and citizens, and edited by
// staff holding `admin.lookup`.
//
// THE COMMUNE IS NEVER A PARAMETER. Tier 2 is read through the scoped store (tenant_id bound from the
// context, rule 1 invariant 5); tier 1 is the same set for every commune and the platform call still
// carries `x-tenant-id` (ADR 0060 §1).
//
// WHY TIER 1 IS ASKED OUTSIDE THE TRANSACTION: it is a gRPC round trip (cached ≤ 60 s). Held inside
// a transaction it would pin a connection and a row lock for the length of a network call — the
// reason ChotLinhVuc asks identity before it opens one.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/platformclient"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	docstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// ErrFieldCatalogueUnavailable means tier 1 could not be read from the platform and no answer
// younger than the reader's TTL is held. Callers answer 503. It is NEVER "the code is unknown" and
// never "use a built-in list": there is no fallback list in this service (ADR 0060 §3, stop
// condition #2).
var ErrFieldCatalogueUnavailable = errors.New("linh_vuc: không đọc được bộ mã lĩnh vực từ nền tảng")

// Tier1Fields is tier 1 as the domain needs it.
type Tier1Fields interface {
	Fields(ctx context.Context) ([]domain.FieldDefault, error)
}

// platformTier1 adapts core/platformclient's reader. The only place in this service that knows the
// platform's types, so the domain stays standard-library only.
type platformTier1 struct {
	r *platformclient.PetitionFields
}

// NewTier1Fields wraps the platform reader. r must be non-nil: a nil reader is a panic on the first
// catalogue read, on a citizen's phone, instead of here.
func NewTier1Fields(r *platformclient.PetitionFields) Tier1Fields {
	if r == nil {
		panic("app: NewTier1Fields with a nil reader")
	}
	return platformTier1{r: r}
}

func (p platformTier1) Fields(ctx context.Context) ([]domain.FieldDefault, error) {
	set, err := p.r.Set(ctx)
	switch {
	case errors.Is(err, platformclient.ErrPetitionFieldsUnavailable):
		return nil, fmt.Errorf("%w: %w", ErrFieldCatalogueUnavailable, err)
	case err != nil:
		return nil, fmt.Errorf("linh_vuc: đọc bộ mã tầng 1: %w", err)
	}
	all := set.All()
	out := make([]domain.FieldDefault, 0, len(all))
	for _, f := range all {
		out = append(out, domain.FieldDefault{Code: f.Code, DefaultLabel: f.DefaultLabel,
			SortOrder: f.SortOrder, Icon: f.Icon, Tone: f.Tone, Active: f.Active})
	}
	return out, nil
}

// FieldOverrideStore is the tier-2 store, declared at the point of use. The two write methods take
// the transaction, so the override cannot be written in one transaction and the audit entry in
// another (rule 6, invariant 3).
type FieldOverrideStore interface {
	// vi-name-ok: the existing NhanLinhVucStore method every label reader already calls.
	DanhSach(ctx context.Context) ([]domain.NhanLinhVuc, error)
	ForUpdate(ctx context.Context, tx *store.ScopedTx, code string) (domain.NhanLinhVuc, bool, error)
	Upsert(ctx context.Context, tx *store.ScopedTx, n domain.NhanLinhVuc) error
}

// PetitionFieldCatalogue owns reading and editing one commune's field catalogue.
type PetitionFieldCatalogue struct {
	db        *store.DB
	tier1     Tier1Fields
	overrides FieldOverrideStore
}

func NewPetitionFieldCatalogue(db *store.DB, tier1 Tier1Fields, overrides FieldOverrideStore) *PetitionFieldCatalogue {
	return &PetitionFieldCatalogue{db: db, tier1: tier1, overrides: overrides}
}

// ActionUpdatePetitionField is the business verb written into the trail.
const ActionUpdatePetitionField = "update_petition_field"

// Catalogue returns every tier-1 code, retired ones included, with this commune's overrides applied,
// in the commune's order. For the staff configuration screen: disabled and retired codes are listed —
// hiding them there would leave no way to switch one back on.
func (c *PetitionFieldCatalogue) Catalogue(ctx context.Context) ([]domain.PetitionFieldView, error) {
	defaults, err := c.tier1.Fields(ctx)
	if err != nil {
		return nil, err
	}
	overrides, err := c.overrides.DanhSach(ctx)
	if err != nil {
		return nil, fmt.Errorf("linh_vuc: đọc cấu hình của xã: %w", err)
	}
	return domain.MergePetitionFields(defaults, overrides), nil
}

// CitizenCatalogue is what the citizen's new-submission form offers: active on the platform, enabled
// by the commune, and never `can-bo` yet (domain.PetitionFieldView.OfferedToCitizens).
func (c *PetitionFieldCatalogue) CitizenCatalogue(ctx context.Context) ([]domain.PetitionFieldView, error) {
	merged, err := c.Catalogue(ctx)
	if err != nil {
		return nil, err
	}
	return domain.CitizenCatalogue(merged), nil
}

// ErrFieldNotOffered — the citizen named a field the commune's form does not offer. ONE sentinel, and
// so one identical answer, for every cause: not a tier-1 code, retired on the platform, switched off by
// the commune, `can-bo` (not yet open to citizens), or not even code-shaped. Telling them apart would
// tell a prober which codes exist and which ones this commune turned off — none of which the citizen
// needs; what they need is to pick again from the catalogue.
var ErrFieldNotOffered = errors.New("linh_vuc: lĩnh vực không có trong danh sách xã đang nhận")

// CheckCitizenIntakeField returns the code to store if the commune's new-submission form offers it —
// the SAME rule the citizen catalogue lists by (domain.PetitionFieldView.OfferedToCitizens: active on
// the platform, enabled by the commune, not `can-bo`), so the form and the check cannot disagree.
//
// THE RETURNED CODE IS THE CATALOGUE'S, not the input: it is what reaches identity and the row, so a
// value that merely compared equal after trimming never travels further than this function.
func (c *PetitionFieldCatalogue) CheckCitizenIntakeField(ctx context.Context, code string) (string, error) {
	merged, err := c.Catalogue(ctx)
	if err != nil {
		return "", err
	}
	for _, v := range merged {
		if v.Code == code && v.OfferedToCitizens() {
			return v.Code, nil
		}
	}
	return "", ErrFieldNotOffered
}

// EffectiveFieldLabels answers every tier-1 code, retired ones included, with the label a screen shows:
// the commune's wording, else the platform default (ADR 0026 §Quyết định). It satisfies the label
// catalogue interface every petition read path already calls, so the list, the detail and the staff
// screens all switch from "commune override or nothing" to "override, else default" in one place.
//
// NEVER FILTERED by `enabled` or `active`: a petition carrying a switched-off or retired code keeps its
// label (ADR 0026 §Bổ sung cuối ngày). Platform unreachable -> ErrFieldCatalogueUnavailable; the read
// routes answer 503 rather than show raw codes (ADR 0060 §3).
type EffectiveFieldLabels struct{ c *PetitionFieldCatalogue }

func NewEffectiveFieldLabels(c *PetitionFieldCatalogue) EffectiveFieldLabels {
	return EffectiveFieldLabels{c: c}
}

// vi-name-ok: implements the existing http.NhanLinhVucDanhMuc interface every label reader calls.
func (l EffectiveFieldLabels) DanhSach(ctx context.Context) ([]domain.NhanLinhVuc, error) {
	merged, err := l.c.Catalogue(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.NhanLinhVuc, 0, len(merged))
	for _, v := range merged {
		out = append(out, domain.NhanLinhVuc{Ma: v.Code, Nhan: v.Label, SortOrder: v.Order, Enabled: v.Enabled})
	}
	return out, nil
}

// Edit applies a partial edit to one code and returns the code as the commune now sees it.
//
// UNKNOWN CODE -> docstore.ErrDanhMucKhongTonTai (404), checked against tier 1 BEFORE the transaction.
// A RETIRED code is still editable: its label shows on old petitions, and a commune may still want
// to word it — retiring decides intake, not display (ADR 0060 §4).
//
// A NO-OP WRITES NOTHING AND AUDITS NOTHING — what the route's idem.KhongCan rests on.
func (c *PetitionFieldCatalogue) Edit(ctx context.Context, code string, e domain.PetitionFieldEdit,
	actor audit.Actor) (domain.PetitionFieldView, error) {

	defaults, err := c.tier1.Fields(ctx)
	if err != nil {
		return domain.PetitionFieldView{}, err
	}
	d, ok := domain.FindFieldDefault(defaults, code)
	if !ok {
		return domain.PetitionFieldView{}, docstore.ErrDanhMucKhongTonTai
	}
	// Validation before any transaction; ApplyFieldEdit is re-run inside on the locked row.
	if _, err := domain.ApplyFieldEdit(d, domain.NhanLinhVuc{Enabled: true}, e); err != nil {
		return domain.PetitionFieldView{}, err
	}
	// The trail names a staff BUSINESS CODE; empty refuses the write, never a fallback (rule 6, inv. 8).
	if actor.ID == "" {
		return domain.PetitionFieldView{}, fmt.Errorf("linh_vuc: thiếu người sửa")
	}

	var after domain.PetitionFieldView
	err = c.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		cur, exists, err := c.overrides.ForUpdate(ctx, tx, d.Code)
		if err != nil {
			return err
		}
		var curPtr *domain.NhanLinhVuc
		if exists {
			curPtr = &cur
		} else {
			cur = domain.NhanLinhVuc{Ma: d.Code, Enabled: true}
		}
		before := domain.ViewOfOverride(d, curPtr)

		next, err := domain.ApplyFieldEdit(d, cur, e)
		if err != nil {
			return err
		}
		after = domain.ViewOfOverride(d, &next)
		if next.Nhan == cur.Nhan && next.SortOrder == cur.SortOrder && next.Enabled == cur.Enabled {
			after = before
			return nil
		}
		if err := c.overrides.Upsert(ctx, tx, next); err != nil {
			return err
		}

		// BEFORE AND AFTER of the fields that moved, as the commune SEES them (effective values) —
		// what a person handling a complaint needs to read. No personal data here (rule 3).
		was, now := map[string]any{}, map[string]any{}
		if before.Label != after.Label {
			was["label"], now["label"] = before.Label, after.Label
		}
		if before.Order != after.Order {
			was["order"], now["order"] = before.Order, after.Order
		}
		if before.Enabled != after.Enabled {
			was["enabled"], now["enabled"] = before.Enabled, after.Enabled
		}
		delta, err := json.Marshal(map[string]any{"truoc": was, "sau": now})
		if err != nil {
			return fmt.Errorf("linh_vuc: mã hoá delta: %w", err)
		}
		// SAME TRANSACTION AS THE UPSERT (rule 6, invariant 3). The commune comes from the transaction.
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionUpdatePetitionField,
			Subject: d.Code, // the field CODE — the business key of the row
			Delta:   delta,
		})
	})
	if err != nil {
		return domain.PetitionFieldView{}, fmt.Errorf("linh_vuc: sửa cho xã %s: %w", tenant.MustFrom(ctx), err)
	}
	return after, nil
}
