package app

// The use cases behind the WRITE surface of the document-type catalogue.
//
// WHY THIS LAYER EXISTS FOR WHAT LOOKS LIKE THREE SMALL STATEMENTS — the same answer
// service-comms gives for its notification ledger, and it is worth repeating because it is the one
// structural rule this repository was built to keep: rule 6, invariant 3 requires the audit entry
// to share a transaction with the business write, and core/audit.Write takes a *store.ScopedTx with
// no overload that writes outside one. Opening that transaction is this layer's job. The handler
// translates HTTP and nothing else, and the store knows SQL and nothing else.
//
// THE SECOND REASON IS THE THREE-TIER MODEL. Every write here is read-decide-write: read the row
// under a lock, work out which tier it is in, refuse or apply. That decision needs the row AND the
// rules AND the transaction at once, which is exactly one place — here.
//
// WHAT THIS FILE DELIBERATELY DOES NOT DO: sow a commune's `nguon = 'he-thong'` rows. That step is
// COMMUNE ONBOARDING, it does not exist in this repository, and designing it decides who writes a
// commune's first rows and under which principal in the audit trail (rule 6, invariant 6). It is
// the customer's call, not a use case's — see migrations/0003_danh_muc_loai_tai_nguyen_ban_do.sql:47-52. An
// empty catalogue is the correct state of every commune today.

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-comms/internal/domain"
	docstore "github.com/vihat/vigov/service-comms/internal/store"
)

// MapAssetTypeStore is the store, declared at the point of use.
//
// EVERY METHOD TAKES THE TRANSACTION. That is what makes it impossible to write the row in one
// transaction and the audit entry in another: there is no signature here that would let you. It is
// also what makes the properties worth proving — that a refusal writes nothing, that the entry
// shares the transaction, that `nguon` is never written from a request — provable without a
// PostgreSQL. A test that needs infrastructure is a test that stops being run.
type MapAssetTypeStore interface {
	ByIDForUpdate(ctx context.Context, tx *store.ScopedTx, id string) (domain.MapAssetType, error)
	CodeTaken(ctx context.Context, tx *store.ScopedTx, code string) (bool, error)
	CountLive(ctx context.Context, tx *store.ScopedTx) (int, error)
	Insert(ctx context.Context, tx *store.ScopedTx, t domain.MapAssetType) error
	ClearOtherDefaults(ctx context.Context, tx *store.ScopedTx, exceptID string) error
	Update(ctx context.Context, tx *store.ScopedTx, t domain.MapAssetType) error
	SoftDelete(ctx context.Context, tx *store.ScopedTx, id, by, reason string) error
	// ImportSnapshot is every row, soft-deleted included — the Excel import's plan (ADR 0059).
	ImportSnapshot(ctx context.Context, tx *store.ScopedTx) ([]domain.ExistingMapAssetType, error)
}

// MapAssetTypeCatalogue owns adding, editing and retiring one commune's document types.
type MapAssetTypeCatalogue struct {
	db   *store.DB
	repo MapAssetTypeStore

	// newID is injected so a test can pin the id. In production it is ulid.Moi.
	newID func() (string, error)
}

func NewMapAssetTypeCatalogue(db *store.DB, repo MapAssetTypeStore) *MapAssetTypeCatalogue {
	return &MapAssetTypeCatalogue{db: db, repo: repo, newID: ulid.Moi}
}

// The business verbs written into the trail. Vietnamese snake_case, like every other action this
// system already writes (`dang_nhap`, `ghi_so_thong_bao`, `xem_day_du_nguoi_gui`): an inspection
// reads these strings, and a function name would tell them nothing.
//
// THE VERB NAMES THE CATALOGUE, not just the operation. `sua_danh_muc` across five tables would
// make the trail unable to answer which list changed without joining to a row that may since have
// been edited again.
//
// THE VALUES ARE STORED DATA: the rename campaign's layer A renames these constants, never the
// strings (ADR 0061 §Ánh xạ giá trị hành vi, X22).
const (
	ActionCreateMapAssetType = "them_loai_tai_nguyen_ban_do"
	ActionUpdateMapAssetType = "sua_loai_tai_nguyen_ban_do"
	ActionDeleteMapAssetType = "xoa_loai_tai_nguyen_ban_do"
)

// CreateMapAssetTypeRequest is one new row, as it arrives from the handler.
//
// THERE IS NO `Source` FIELD AND THERE MUST NEVER BE ONE. Provenance decides the tier, so a field
// here is a field a handler can fill from a request body — and the migration says what follows:
// "every guard below could be stepped around by setting nguon = 'don-vi' first". The store writes
// the value as a LITERAL for the same reason.
//
// THERE IS NO `IsActive` FIELD EITHER, and that is a smaller decision said out loud: a row the
// commune has just added is in use. Creating one already disabled is two requests — POST then
// PATCH — and the second one is the one that leaves a trail saying somebody turned it off.
type CreateMapAssetTypeRequest struct {
	Code      string
	Label     string
	SortOrder int
	IsDefault bool
}

// UpdateMapAssetTypeRequest is a PARTIAL edit: a nil pointer means "leave this alone".
//
// WHY POINTERS AND NOT A FULL REPLACEMENT. Three of the four fields have a meaningful zero —
// `thu_tu` 0 is the first position, `dang_dung` false is "taken out of use", `la_mac_dinh` false is
// "no longer the default". A struct of plain values cannot tell "the client did not mention this"
// from "the client set it to zero", so a screen that edits only the label would silently move the
// row to the top of the list and clear the commune's default.
type UpdateMapAssetTypeRequest struct {
	Label     *string
	SortOrder *int
	IsActive  *bool
	IsDefault *bool
}

// Create adds one row the commune owns.
//
// ORDER OF THE THREE REFUSALS, and it is not arbitrary: shape first (cheap, no lock), then the
// ceiling, then the duplicate code. The ceiling before the duplicate because a full catalogue is a
// condition of the whole list while a duplicate is a condition of one value — and the caller can
// act on the first without knowing anything about the second.
func (uc *MapAssetTypeCatalogue) Create(ctx context.Context, req CreateMapAssetTypeRequest,
	actor audit.Actor) (domain.MapAssetType, error) {

	// Validated BEFORE the transaction opens. A request that fails its shape must never hold a row
	// lock while doing so, and the caller needs the reason rather than a rollback.
	code, err := domain.NormalizeCode(req.Code)
	if err != nil {
		return domain.MapAssetType{}, err
	}
	label, err := domain.NormalizeLabel(req.Label)
	if err != nil {
		return domain.MapAssetType{}, err
	}
	if err := domain.ValidateSortOrder(req.SortOrder); err != nil {
		return domain.MapAssetType{}, err
	}

	id, err := uc.newID()
	if err != nil {
		return domain.MapAssetType{}, fmt.Errorf("danh_muc_loai_tai_nguyen_ban_do: sinh mã: %w", err)
	}

	created := domain.MapAssetType{
		ID: id, Code: code, Label: label, SortOrder: req.SortOrder,
		IsDefault: req.IsDefault,
		IsActive:  true,
		// Set here only so the value this function RETURNS describes the row that was written. The
		// store does not read them: it writes 'don-vi' and false as literals.
		Source:           domain.SourceCommune,
		BranchedInSource: false,
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		n, err := uc.repo.CountLive(ctx, tx)
		if err != nil {
			return err
		}
		if n >= docstore.MapAssetTypeCeiling {
			return docstore.ErrCatalogueFull
		}

		taken, err := uc.repo.CodeTaken(ctx, tx, code)
		if err != nil {
			return err
		}
		if taken {
			return docstore.ErrCodeTaken
		}

		if created.IsDefault {
			// BEFORE the insert, not after: `UNIQUE (tenant_id, moc_mac_dinh)` admits one live
			// default, and inserting the second one first is the statement that fails.
			if err := uc.repo.ClearOtherDefaults(ctx, tx, created.ID); err != nil {
				return err
			}
		}
		// Scoped: tx comes from uc.db.For(ctx).Tx — tenant_id is $1 of the INSERT.
		if err := uc.repo.Insert(ctx, tx, created); err != nil {
			return err
		}

		delta, err := json.Marshal(map[string]any{"sau": summarizeMapAssetType(created)})
		if err != nil {
			return fmt.Errorf("danh_muc_loai_tai_nguyen_ban_do: mã hoá delta: %w", err)
		}
		// SAME TRANSACTION AS THE INSERT (rule 6, invariant 3). TenantID is left unset on purpose:
		// audit.Write fills it from the transaction, which took it from the context (rule 1,
		// invariant 4). Passing it here would be a second source for the one fact that decides
		// which commune the entry belongs to.
		//
		// NOTHING IN THE DELTA IS PERSONAL DATA (rule 3): a document type is how the authority
		// classifies its paperwork, not anything about a person.
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionCreateMapAssetType,
			Subject: created.Code, // the business code, never the internal id
			Delta:   delta,
		})
	})
	if err != nil {
		// Nothing was committed: no row, no trail. The two states agree.
		return domain.MapAssetType{}, wrapErr(ctx, "thêm", err)
	}
	return created, nil
}

// Update applies a partial edit, refusing whatever this row's tier does not allow.
//
// A NO-OP WRITES NOTHING AND AUDITS NOTHING. Sending the label a row already has is not an event;
// recording it would fill a public authority's ledger with entries saying nothing changed, and
// those are the entries that bury the ones carrying legal weight. It is also what makes this route
// genuinely idempotent, which is what its `idem.KhongCan` declaration claims.
func (uc *MapAssetTypeCatalogue) Update(ctx context.Context, id string, req UpdateMapAssetTypeRequest,
	actor audit.Actor) (domain.MapAssetType, error) {

	if id == "" {
		return domain.MapAssetType{}, docstore.ErrCatalogueRowNotFound
	}
	// Shape first, outside the transaction, for the same reason as Create.
	var label string
	if req.Label != nil {
		var err error
		if label, err = domain.NormalizeLabel(*req.Label); err != nil {
			return domain.MapAssetType{}, err
		}
	}
	if req.SortOrder != nil {
		if err := domain.ValidateSortOrder(*req.SortOrder); err != nil {
			return domain.MapAssetType{}, err
		}
	}

	var after domain.MapAssetType
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		before, err := uc.repo.ByIDForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}

		after = before
		if req.Label != nil {
			after.Label = label
		}
		if req.SortOrder != nil {
			after.SortOrder = *req.SortOrder
		}
		if req.IsActive != nil {
			after.IsActive = *req.IsActive
		}
		if req.IsDefault != nil {
			after.IsDefault = *req.IsDefault
		}

		// THE TIER CHECK IS ON THE TRANSITION, not on the requested value. Asking a tier-3 row to
		// stay enabled is not an attempt to disable it, and refusing that would make the ordinary
		// "save the whole form" request fail on exactly the rows a commune may not touch.
		if before.IsActive && !after.IsActive {
			if err := before.Tier().AllowDisable(); err != nil {
				return err
			}
		}

		if mapAssetTypeUnchanged(before, after) {
			return nil
		}

		if after.IsDefault && !before.IsDefault {
			if err := uc.repo.ClearOtherDefaults(ctx, tx, after.ID); err != nil {
				return err
			}
		}
		// Scoped: tx comes from uc.db.For(ctx).Tx — tenant_id is $1 of the UPDATE.
		if err := uc.repo.Update(ctx, tx, after); err != nil {
			return err
		}

		// BEFORE AND AFTER, AND ONLY THE FIELDS THAT MOVED (rule 6, invariant 5). A delta carrying
		// every column on every edit makes the one field somebody actually changed impossible to
		// find in a ledger that is never deleted.
		delta, err := json.Marshal(map[string]any{
			"truoc": summarizeMapAssetTypeChange(before, after, true),
			"sau":   summarizeMapAssetTypeChange(before, after, false),
		})
		if err != nil {
			return fmt.Errorf("danh_muc_loai_tai_nguyen_ban_do: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionUpdateMapAssetType,
			Subject: after.Code,
			Delta:   delta,
		})
	})
	if err != nil {
		return domain.MapAssetType{}, wrapErr(ctx, "sửa", err)
	}
	return after, nil
}

// Delete soft deletes one row — tier 1 only.
//
// THIS IS NOT A DELETE AND THE NAME IS THE ONLY PLACE THAT COULD SUGGEST OTHERWISE. The row stays,
// carrying `deleted_at`, `deleted_by` and `delete_reason` (rule 7, invariant 1), and its `ma` stays
// taken forever: an issued code is never reissued, because business records hold it as a value.
func (uc *MapAssetTypeCatalogue) Delete(ctx context.Context, id, rawReason string, actor audit.Actor) error {
	if id == "" {
		return docstore.ErrCatalogueRowNotFound
	}
	reason, err := domain.NormalizeDeleteReason(rawReason)
	if err != nil {
		return err
	}
	if actor.ID == "" {
		// `deleted_by` with nothing in it is a deletion nobody can be asked about. core/audit
		// refuses an entry with no actor for the same reason; refusing here keeps the column and
		// the entry telling the same story.
		return fmt.Errorf("danh_muc_loai_tai_nguyen_ban_do: thiếu người xoá")
	}

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		before, err := uc.repo.ByIDForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := before.Tier().AllowSoftDelete(); err != nil {
			return err
		}
		if err := uc.repo.SoftDelete(ctx, tx, before.ID, actor.ID, reason); err != nil {
			return err
		}

		// THE REASON IS IN THE ENTRY AS WELL AS IN THE COLUMN, and that is not the duplication
		// rule 9 forbids: the column is the current state of the row and can only ever hold the
		// FIRST deletion, while the entry is the append-only record of the act. They answer two
		// different questions and neither can be derived from the other.
		delta, err := json.Marshal(map[string]any{
			"truoc":   summarizeMapAssetType(before),
			"ly_do":   reason,
			"xoa_mem": true,
		})
		if err != nil {
			return fmt.Errorf("danh_muc_loai_tai_nguyen_ban_do: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionDeleteMapAssetType,
			Subject: before.Code,
			Delta:   delta,
		})
	})
	if err != nil {
		return wrapErr(ctx, "xoá", err)
	}
	return nil
}

// summarizeMapAssetType is the audit delta's view of one row. `nguon` is in it BECAUSE it is the
// fact that decides what may later be done to this row, and an inspection reading the entry has no
// other way to know which tier the row was in at the time.
//
// THE KEYS ARE THE OLD COLUMN NAMES AND STAY SO: `audit_log.delta` keeps the names it was written
// with (ADR 0061 §Dòng chỉ-thêm), so a key renamed here would split one field's history in two.
func summarizeMapAssetType(t domain.MapAssetType) map[string]any {
	return map[string]any{
		"ma":          t.Code,
		"nhan":        t.Label,
		"thu_tu":      t.SortOrder,
		"dang_dung":   t.IsActive,
		"la_mac_dinh": t.IsDefault,
		"nguon":       t.Source,
	}
}

// summarizeMapAssetTypeChange returns only the fields that actually moved, from whichever side is
// asked for.
func summarizeMapAssetTypeChange(before, after domain.MapAssetType, side bool) map[string]any {
	out := map[string]any{}
	if before.Label != after.Label {
		out["nhan"] = pick(side, before.Label, after.Label)
	}
	if before.SortOrder != after.SortOrder {
		out["thu_tu"] = pick(side, before.SortOrder, after.SortOrder)
	}
	if before.IsActive != after.IsActive {
		out["dang_dung"] = pick(side, before.IsActive, after.IsActive)
	}
	if before.IsDefault != after.IsDefault {
		out["la_mac_dinh"] = pick(side, before.IsDefault, after.IsDefault)
	}
	return out
}

// mapAssetTypeUnchanged reports whether the edit would change nothing. `ma`, `nguon` and
// `ma_nguon_re_nhanh` are not compared because no path here can change them.
func mapAssetTypeUnchanged(before, after domain.MapAssetType) bool {
	return before.Label == after.Label && before.SortOrder == after.SortOrder &&
		before.IsActive == after.IsActive && before.IsDefault == after.IsDefault
}
