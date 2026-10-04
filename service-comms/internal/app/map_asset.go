package app

// The use cases behind the WRITE surface of the commune's map asset register (migration 0015; ADR
// 0072; menu `ban-do-kinh-te-so`). Same structure as external_contact.go and map_field_schema.go, for
// the same two reasons: the audit entry shares the business write's transaction (rule 6, invariant 3),
// and every edit is read-decide-write under a row lock.
//
// PERSONAL DATA IN THE TRAIL (rule 6 forbidden #4, 0015 §PERSONAL DATA). The delta carries
// `representative` masked (privacy.MaskName), `phone` masked (MaskPhone), `tax_code` masked (MaskCccd —
// it may be a citizen ID number), `address` and `description` as presence only, the coordinates NOT AT
// ALL (a household's pin is a home location — only "moved" is recorded), and custom values as KEYS only
// (a free-text field may hold anything). There is no masking function for an address or a coordinate in
// core/privacy, and a truncated address is still an address.

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/privacy"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// MapAssetRepo is the store, declared at the point of use. Every method takes the transaction, so there
// is no signature that writes the row outside the one its entry is in.
type MapAssetRepo interface {
	AssetTypeAvailable(ctx context.Context, tx *store.ScopedTx, code string) (bool, error)
	FieldsOfType(ctx context.Context, tx *store.ScopedTx, code string) ([]domain.MapFieldSchema, error)
	TaxCodeTaken(ctx context.Context, tx *store.ScopedTx, taxCode, exceptID string) (bool, error)
	Insert(ctx context.Context, tx *store.ScopedTx, a domain.MapAsset, by string) (domain.MapAsset, error)
	ByIDForUpdate(ctx context.Context, tx *store.ScopedTx, id string) (domain.MapAsset, error)
	Update(ctx context.Context, tx *store.ScopedTx, a domain.MapAsset, by string) (domain.MapAsset, error)
	SetConfirmation(ctx context.Context, tx *store.ScopedTx, a domain.MapAsset, verified bool, by string) (domain.MapAsset, error)
	SoftDelete(ctx context.Context, tx *store.ScopedTx, id, by, reason string) error
}

// MapAssets owns adding, editing, verifying and retiring one commune's map assets.
type MapAssets struct {
	db   *store.DB
	repo MapAssetRepo

	// newID is injected so a test can pin the id. In production it is ulid.Moi.
	newID func() (string, error)
}

func NewMapAssets(db *store.DB, repo MapAssetRepo) *MapAssets {
	return &MapAssets{db: db, repo: repo, newID: ulid.Moi}
}

// The business verbs written into the trail. VALUES, not identifiers: Vietnamese snake_case like every
// other action this service writes (ADR 0011), naming the register so the trail answers which list
// changed without a join.
const (
	ActionCreateMapAsset    = "them_doi_tuong_ban_do"
	ActionUpdateMapAsset    = "sua_doi_tuong_ban_do"
	ActionDeleteMapAsset    = "xoa_doi_tuong_ban_do"
	ActionConfirmMapAsset   = "xac_minh_doi_tuong_ban_do"
	ActionUnconfirmMapAsset = "bo_xac_minh_doi_tuong_ban_do"

	// ActionReadMapAssetFull — reading the UNMASKED representative / phone / tax code (rule 6,
	// invariant 7), the same shape as petitions' `xem_day_du_nguoi_gui`.
	ActionReadMapAssetFull = "xem_day_du_doi_tuong_ban_do"
)

// MapAssetInput is one new asset, as it arrives from the handler. Lat/Lng are pointers: a missing
// coordinate is refused, never read as 0,0 in the Gulf of Guinea.
type MapAssetInput struct {
	AssetTypeCode     string
	Name              string
	Address           string
	ResidentialUnitID string
	Lat, Lng          *float64
	Representative    string
	Phone             string
	Status            string // "" = dang-hoat-dong (spec §8.1 default)
	TaxCode           string
	IndustryCode      string
	EmployeeCount     *int
	EstablishedOn     string
	Description       string
	CustomValues      map[string]json.RawMessage
}

// MapAssetPatch is a PARTIAL edit: nil means "leave this alone"; "" CLEARS an optional text (the house
// convention — external contacts' `address`). EmployeeCount has no "clear": a JSON null cannot be told
// from an absent key through a pointer, the limit content-items' `display_order` states.
// CustomValues MERGES by key (domain.ApplyCustomValues); a key set to null is removed.
type MapAssetPatch struct {
	AssetTypeCode     *string
	Name              *string
	Address           *string
	ResidentialUnitID *string
	Lat, Lng          *float64
	Representative    *string
	Phone             *string
	Status            *string
	TaxCode           *string
	IndustryCode      *string
	EmployeeCount     *int
	EstablishedOn     *string
	Description       *string
	CustomValues      map[string]json.RawMessage
}

func wrapMapAsset(ctx context.Context, op string, err error) error {
	return fmt.Errorf("map_asset: %s cho xã %s: %w", op, tenant.MustFrom(ctx), err)
}

// Create adds one asset. Shape first (no transaction), then — inside it — the type, the custom values
// against this commune's schema, the tax code, the row and its entry.
func (uc *MapAssets) Create(ctx context.Context, in MapAssetInput, actor audit.Actor) (domain.MapAsset, error) {
	if actor.ID == "" {
		return domain.MapAsset{}, ErrMissingActor
	}
	if in.Lat == nil || in.Lng == nil {
		return domain.MapAsset{}, domain.ErrMapAssetLocationMissing
	}
	status := in.Status
	if status == "" {
		status = domain.MapAssetStatusActive
	}
	row, err := domain.NormalizeMapAsset(domain.MapAsset{
		AssetTypeCode: in.AssetTypeCode, Name: in.Name, Address: in.Address,
		ResidentialUnitID: in.ResidentialUnitID, Lat: *in.Lat, Lng: *in.Lng,
		Representative: in.Representative, Phone: in.Phone, Status: status, TaxCode: in.TaxCode,
		IndustryCode: in.IndustryCode, EmployeeCount: in.EmployeeCount, EstablishedOn: in.EstablishedOn,
		Description: in.Description,
	})
	if err != nil {
		return domain.MapAsset{}, err
	}
	if row.ID, err = uc.newID(); err != nil {
		return domain.MapAsset{}, fmt.Errorf("map_asset: sinh id: %w", err)
	}

	// Every repo call below runs on the ScopedTx uc.db.For(ctx).Tx opens: the commune is taken from ctx
	// once, and each statement binds it as $1 from tx.TenantID().
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		fields, err := uc.checkType(ctx, tx, row.AssetTypeCode)
		if err != nil {
			return err
		}
		if row.CustomValues, err = domain.ApplyCustomValues(fields, map[string]json.RawMessage{}, in.CustomValues); err != nil {
			return err
		}
		if err := domain.CheckRequiredCustomValues(fields, row.CustomValues); err != nil {
			return err
		}
		if err := uc.checkTaxCode(ctx, tx, row.TaxCode, row.ID); err != nil {
			return err
		}
		// created_by / updated_by are the actor's BUSINESS CODE: the handler builds audit.Actor.ID from
		// Principal.Ma (rule 6, invariant 8).
		// Scoped: tx comes from uc.db.For(ctx).Tx — tenant_id is $1 of the INSERT.
		if row, err = uc.repo.Insert(ctx, tx, row, actor.ID); err != nil {
			return err
		}
		delta, err := json.Marshal(map[string]any{"sau": mapAssetForAudit(row)})
		if err != nil {
			return fmt.Errorf("map_asset: mã hoá delta: %w", err)
		}
		// SAME TRANSACTION AS THE INSERT (rule 6, invariant 3).
		return audit.Write(ctx, tx, audit.Entry{
			Actor: actor, Action: ActionCreateMapAsset, Subject: mapAssetSubject(row), Delta: delta,
		})
	})
	if err != nil {
		return domain.MapAsset{}, wrapMapAsset(ctx, "thêm", err)
	}
	return row, nil
}

// Update applies a partial edit, judged on the row AFTER the merge. A NO-OP WRITES AND AUDITS NOTHING —
// which is what makes the route's idem.KhongCan declaration true.
func (uc *MapAssets) Update(ctx context.Context, id string, p MapAssetPatch, actor audit.Actor) (domain.MapAsset, error) {
	if id == "" {
		return domain.MapAsset{}, commsstore.ErrMapAssetNotFound
	}
	if actor.ID == "" {
		return domain.MapAsset{}, ErrMissingActor
	}
	if (p.Lat == nil) != (p.Lng == nil) {
		// One coordinate alone is not a place.
		return domain.MapAsset{}, domain.ErrMapAssetLocationMissing
	}

	var after domain.MapAsset
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		before, err := uc.repo.ByIDForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if after, err = domain.NormalizeMapAsset(mergeMapAsset(before, p)); err != nil {
			return err
		}
		after.CustomValues = before.CustomValues

		// The type is checked only when it is being SET: an asset filed under a group that was later
		// disabled keeps it, and editing its phone must not fail on that.
		var fields []domain.MapFieldSchema
		if after.AssetTypeCode != before.AssetTypeCode {
			if fields, err = uc.checkType(ctx, tx, after.AssetTypeCode); err != nil {
				return err
			}
		} else if fields, err = uc.repo.FieldsOfType(ctx, tx, after.AssetTypeCode); err != nil {
			return err
		}
		if len(p.CustomValues) > 0 {
			if after.CustomValues, err = domain.ApplyCustomValues(fields, before.CustomValues, p.CustomValues); err != nil {
				return err
			}
		}
		if sameMapAsset(before, after) {
			return nil
		}
		if err := domain.CheckRequiredCustomValues(fields, after.CustomValues); err != nil {
			return err
		}
		if after.TaxCode != before.TaxCode {
			if err := uc.checkTaxCode(ctx, tx, after.TaxCode, after.ID); err != nil {
				return err
			}
		}
		// Scoped: tx comes from uc.db.For(ctx).Tx — tenant_id is $1 of the UPDATE.
		if after, err = uc.repo.Update(ctx, tx, after, actor.ID); err != nil {
			return err
		}
		beforeSide, afterSide := diffMapAsset(before, after)
		delta, err := json.Marshal(map[string]any{"id": after.ID, "truoc": beforeSide, "sau": afterSide})
		if err != nil {
			return fmt.Errorf("map_asset: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor: actor, Action: ActionUpdateMapAsset, Subject: mapAssetSubject(after), Delta: delta,
		})
	})
	if err != nil {
		return domain.MapAsset{}, wrapMapAsset(ctx, "sửa", err)
	}
	return after, nil
}

// SetConfirmation sets or clears the verified flag. The same state twice writes and audits nothing.
func (uc *MapAssets) SetConfirmation(ctx context.Context, id string, verified bool, actor audit.Actor) (domain.MapAsset, error) {
	if id == "" {
		return domain.MapAsset{}, commsstore.ErrMapAssetNotFound
	}
	if actor.ID == "" {
		return domain.MapAsset{}, ErrMissingActor
	}
	var after domain.MapAsset
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		before, err := uc.repo.ByIDForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if before.Verified == verified {
			after = before
			return nil
		}
		// verified_by is the actor's BUSINESS CODE (rule 6, invariant 8; 0015's column comment).
		// Scoped: tx comes from uc.db.For(ctx).Tx — tenant_id is $1 of the UPDATE.
		if after, err = uc.repo.SetConfirmation(ctx, tx, before, verified, actor.ID); err != nil {
			return err
		}
		action := ActionConfirmMapAsset
		if !verified {
			action = ActionUnconfirmMapAsset
		}
		// Clearing erases verified_at / verified_by from the row (0015); the entry is where who had
		// verified, and when, survives.
		delta, err := json.Marshal(map[string]any{
			"id":    after.ID,
			"truoc": confirmationForAudit(before),
			"sau":   confirmationForAudit(after),
		})
		if err != nil {
			return fmt.Errorf("map_asset: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor: actor, Action: action, Subject: mapAssetSubject(after), Delta: delta,
		})
	})
	if err != nil {
		return domain.MapAsset{}, wrapMapAsset(ctx, "xác minh", err)
	}
	return after, nil
}

// Delete soft deletes one asset with a mandatory reason. The row stays — deleted_at · deleted_by ·
// delete_reason — and leaves every read path; its tax code is free for a new live row (0015).
func (uc *MapAssets) Delete(ctx context.Context, id, rawReason string, actor audit.Actor) error {
	if id == "" {
		return commsstore.ErrMapAssetNotFound
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
			"truoc":   mapAssetForAudit(before),
			"ly_do":   reason,
			"xoa_mem": true,
		})
		if err != nil {
			return fmt.Errorf("map_asset: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor: actor, Action: ActionDeleteMapAsset, Subject: mapAssetSubject(before), Delta: delta,
		})
	})
	if err != nil {
		return wrapMapAsset(ctx, "xoá", err)
	}
	return nil
}

// RecordFullView writes the trail of a member of staff reading the UNMASKED personal fields of one
// asset (rule 6, invariant 7) — called by the detail route BEFORE it answers; if it fails the route
// answers 500, never unmasked without a trail. The delta names WHICH fields were disclosed, never their
// values. An asset holding none of them discloses nothing and writes nothing.
func (uc *MapAssets) RecordFullView(ctx context.Context, a domain.MapAsset, actor audit.Actor) error {
	if actor.ID == "" {
		return ErrMissingActor
	}
	disclosed := MapAssetPersonalFields(a)
	if len(disclosed) == 0 {
		return nil
	}
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		delta, err := json.Marshal(map[string]any{"id": a.ID, "truong": disclosed})
		if err != nil {
			return fmt.Errorf("map_asset: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor: actor, Action: ActionReadMapAssetFull, Subject: mapAssetSubject(a), Delta: delta,
		})
	})
	if err != nil {
		return wrapMapAsset(ctx, "ghi vết xem đầy đủ", err)
	}
	return nil
}

// MapAssetPersonalFields lists the masked-by-default fields this asset actually holds — what a full
// view discloses. One list for the detail route and its trail, so the two cannot disagree.
func MapAssetPersonalFields(a domain.MapAsset) []string {
	var out []string
	if a.Representative != "" {
		out = append(out, "representative")
	}
	if a.Phone != "" {
		out = append(out, "phone")
	}
	if a.TaxCode != "" {
		out = append(out, "tax_code")
	}
	return out
}

// checkType refuses a type that is not live and in use in THIS commune's catalogue, and returns its
// field schema.
func (uc *MapAssets) checkType(ctx context.Context, tx *store.ScopedTx, code string) ([]domain.MapFieldSchema, error) {
	ok, err := uc.repo.AssetTypeAvailable(ctx, tx, code)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, commsstore.ErrMapAssetTypeUnavailable
	}
	return uc.repo.FieldsOfType(ctx, tx, code)
}

func (uc *MapAssets) checkTaxCode(ctx context.Context, tx *store.ScopedTx, taxCode, id string) error {
	if taxCode == "" {
		return nil
	}
	taken, err := uc.repo.TaxCodeTaken(ctx, tx, taxCode, id)
	if err != nil {
		return err
	}
	if taken {
		return commsstore.ErrMapAssetTaxCodeTaken
	}
	return nil
}

func mergeMapAsset(m domain.MapAsset, p MapAssetPatch) domain.MapAsset {
	set := func(dst *string, src *string) {
		if src != nil {
			*dst = *src
		}
	}
	set(&m.AssetTypeCode, p.AssetTypeCode)
	set(&m.Name, p.Name)
	set(&m.Address, p.Address)
	set(&m.ResidentialUnitID, p.ResidentialUnitID)
	set(&m.Representative, p.Representative)
	set(&m.Phone, p.Phone)
	set(&m.Status, p.Status)
	set(&m.TaxCode, p.TaxCode)
	set(&m.IndustryCode, p.IndustryCode)
	set(&m.EstablishedOn, p.EstablishedOn)
	set(&m.Description, p.Description)
	if p.Lat != nil && p.Lng != nil {
		m.Lat, m.Lng = *p.Lat, *p.Lng
	}
	if p.EmployeeCount != nil {
		n := *p.EmployeeCount
		m.EmployeeCount = &n
	}
	return m
}

func sameMapAsset(x, y domain.MapAsset) bool {
	return x.AssetTypeCode == y.AssetTypeCode && x.Name == y.Name && x.Address == y.Address &&
		x.ResidentialUnitID == y.ResidentialUnitID && x.Lat == y.Lat && x.Lng == y.Lng &&
		x.Representative == y.Representative && x.Phone == y.Phone && x.Status == y.Status &&
		x.TaxCode == y.TaxCode && x.IndustryCode == y.IndustryCode && sameOrder(x.EmployeeCount, y.EmployeeCount) &&
		x.EstablishedOn == y.EstablishedOn && x.Description == y.Description &&
		len(changedCustomKeys(x.CustomValues, y.CustomValues)) == 0
}

// mapAssetSubject is the trail's locator. A MAP ASSET HAS NO BUSINESS CODE — 0015 mints none, and the
// tax code is not one this system issues (and may be a person's ID). The NAME IS DELIBERATELY NOT IN IT:
// a household business is often named after its owner (0015 §PERSONAL DATA) and the ledger is never
// deleted. So, like chuDeNoiDungMiniApp, it is composed from the facts that locate the row on the
// `Sổ địa điểm` — the group and the day it was entered; the delta carries the id.
func mapAssetSubject(a domain.MapAsset) string {
	day := a.CreatedAt
	if day.IsZero() {
		day = time.Now()
	}
	return "doi-tuong-ban-do/" + a.AssetTypeCode + "/" + day.UTC().Format(time.DateOnly)
}

// mapAssetForAudit is the delta's view of one row — personal fields masked or reduced to presence.
func mapAssetForAudit(a domain.MapAsset) map[string]any {
	return map[string]any{
		"id":                  a.ID,
		"asset_type_code":     a.AssetTypeCode,
		"name":                a.Name,
		"has_address":         a.Address != "",
		"residential_unit_id": a.ResidentialUnitID,
		"representative":      privacy.MaskName(a.Representative),
		"phone":               privacy.MaskPhone(a.Phone),
		"tax_code":            privacy.MaskCccd(a.TaxCode),
		"status":              a.Status,
		"verified":            a.Verified,
		"industry_code":       a.IndustryCode,
		"employee_count":      orderOrNil(a.EmployeeCount),
		"established_on":      a.EstablishedOn,
		"has_description":     a.Description != "",
		"custom_value_keys":   customKeys(a.CustomValues),
	}
}

// diffMapAsset returns only the fields that moved, one map per side (rule 6, invariant 5) — masked the
// same way as mapAssetForAudit.
func diffMapAsset(before, after domain.MapAsset) (map[string]any, map[string]any) {
	b, a := map[string]any{}, map[string]any{}
	pair := func(key string, x, y any, changed bool) {
		if changed {
			b[key], a[key] = x, y
		}
	}
	pair("asset_type_code", before.AssetTypeCode, after.AssetTypeCode, before.AssetTypeCode != after.AssetTypeCode)
	pair("name", before.Name, after.Name, before.Name != after.Name)
	pair("residential_unit_id", before.ResidentialUnitID, after.ResidentialUnitID, before.ResidentialUnitID != after.ResidentialUnitID)
	pair("status", before.Status, after.Status, before.Status != after.Status)
	pair("industry_code", before.IndustryCode, after.IndustryCode, before.IndustryCode != after.IndustryCode)
	pair("employee_count", orderOrNil(before.EmployeeCount), orderOrNil(after.EmployeeCount), !sameOrder(before.EmployeeCount, after.EmployeeCount))
	pair("established_on", before.EstablishedOn, after.EstablishedOn, before.EstablishedOn != after.EstablishedOn)
	pair("representative", privacy.MaskName(before.Representative), privacy.MaskName(after.Representative), before.Representative != after.Representative)
	pair("phone", privacy.MaskPhone(before.Phone), privacy.MaskPhone(after.Phone), before.Phone != after.Phone)
	pair("tax_code", privacy.MaskCccd(before.TaxCode), privacy.MaskCccd(after.TaxCode), before.TaxCode != after.TaxCode)
	if before.Address != after.Address {
		// Presence is what is safe to keep; "changed" says the rest.
		b["has_address"], a["has_address"] = before.Address != "", after.Address != ""
		a["address_changed"] = true
	}
	if before.Description != after.Description {
		b["has_description"], a["has_description"] = before.Description != "", after.Description != ""
		a["description_changed"] = true
	}
	if before.Lat != after.Lat || before.Lng != after.Lng {
		a["location_changed"] = true
	}
	if changed := changedCustomKeys(before.CustomValues, after.CustomValues); len(changed) > 0 {
		a["custom_values_changed"] = changed
	}
	return b, a
}

func confirmationForAudit(a domain.MapAsset) map[string]any {
	out := map[string]any{"verified": a.Verified}
	if a.VerifiedAt != nil {
		out["verified_at"] = a.VerifiedAt.UTC().Format(time.RFC3339)
		out["verified_by"] = a.VerifiedBy
	}
	return out
}

func customKeys(v map[string]json.RawMessage) []string {
	out := make([]string, 0, len(v))
	for k := range v {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// changedCustomKeys lists the keys added, removed or whose value differs — compared after compaction,
// so formatting is not a change.
func changedCustomKeys(before, after map[string]json.RawMessage) []string {
	seen := map[string]bool{}
	for k := range before {
		seen[k] = true
	}
	for k := range after {
		seen[k] = true
	}
	var out []string
	for k := range seen {
		x, okx := before[k]
		y, oky := after[k]
		if okx != oky || !sameRaw(x, y) {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// sameRaw compares two JSON values after compaction (json.Marshal of a RawMessage compacts it).
func sameRaw(x, y json.RawMessage) bool {
	bx, errx := json.Marshal(x)
	by, erry := json.Marshal(y)
	return errx == nil && erry == nil && string(bx) == string(by)
}
