package app

// The use case behind POST /api/v1/map-asset-types/defaults — sowing the eleven groups of ADR 0072 §3
// (domain.DefaultMapAssetTypes) into the request's commune. The rules are ADR 0055 §2's for the
// template roles, the precedent ADR 0072 §3 names:
//
//	PRESSED, NEVER AUTOMATIC   not at onboarding, not at startup, not in a migration (0003 REASON ONE)
//	EXISTING MEANS UNTOUCHED   a live row with one of the codes keeps its label, order, state, tier
//	DELETED MEANS DECIDED      a soft-deleted row with one of the codes is skipped and REPORTED — never
//	                           revived and never re-created (UNIQUE (tenant_id, ma) counts it anyway)
//	ONE TRANSACTION            every row and the audit entry, or none of them
//
// ONE AUDIT ENTRY PER RUN, NOT ONE PER ROW — the SLA seed's shape rather than the role seed's. A role
// carries grants and a later history of its own under its own subject; a catalogue group created here
// is a label in a list, and the later edits of each row are audited under its code by the catalogue's
// own routes. The entry lists every code created and every code skipped, so the one act a commune
// administrator performed reads as one record.
//
// A RUN THAT CREATES NOTHING WRITES NOTHING AND AUDITS NOTHING — which is what makes the route's
// idem.KhongCan declaration true.

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// MapAssetTypeDefaultsRepo is the store, declared at the point of use. Every method takes the
// transaction. *commsstore.LoaiTaiNguyenBanDoStore satisfies it.
type MapAssetTypeDefaultsRepo interface {
	LockCatalogueForSeed(ctx context.Context, tx *store.ScopedTx) error
	CodeStates(ctx context.Context, tx *store.ScopedTx, codes []string) (map[string]bool, error)
	DemDangSong(ctx context.Context, tx *store.ScopedTx) (int, error) // vi-name-ok: the existing store method, reused not renamed (rule 12 inv 3)
	InsertSystemRow(ctx context.Context, tx *store.ScopedTx, id string, d domain.MapAssetTypeDefault) error
}

// ActionSeedMapAssetTypes is the verb in the trail.
const ActionSeedMapAssetTypes = "nap_loai_tai_nguyen_ban_do_mac_dinh"

// MapAssetTypeRef names one group in the result: code and label only.
type MapAssetTypeRef struct {
	Code  string
	Label string
}

// MapAssetTypeSeedResult is what one run did. THREE LISTS AND NOT A COUNT, for ADR 0055's reason:
// "already there" and "deleted by somebody" are two different things the administrator must be told
// apart — the second is the commune's own earlier decision.
type MapAssetTypeSeedResult struct {
	Created         []MapAssetTypeRef
	SkippedExisting []MapAssetTypeRef
	SkippedDeleted  []MapAssetTypeRef
}

// MapAssetTypeDefaults seeds the eleven groups.
type MapAssetTypeDefaults struct {
	db   *store.DB
	repo MapAssetTypeDefaultsRepo

	// newID is injected so a test can pin it. In production: ulid.Moi.
	newID func() (string, error)
}

func NewMapAssetTypeDefaults(db *store.DB, repo MapAssetTypeDefaultsRepo) *MapAssetTypeDefaults {
	return &MapAssetTypeDefaults{db: db, repo: repo, newID: ulid.Moi}
}

// SeedDefaults writes the groups this commune does not have yet.
func (uc *MapAssetTypeDefaults) SeedDefaults(ctx context.Context, actor audit.Actor) (MapAssetTypeSeedResult, error) {
	if actor.ID == "" {
		return MapAssetTypeSeedResult{}, ErrMissingActor
	}
	defaults := domain.DefaultMapAssetTypes()
	codes := make([]string, 0, len(defaults))
	for _, d := range defaults {
		codes = append(codes, d.Code)
	}

	var res MapAssetTypeSeedResult
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// Reset on every attempt: a result surviving into a retried closure would report a total.
		res = MapAssetTypeSeedResult{}

		if err := uc.repo.LockCatalogueForSeed(ctx, tx); err != nil {
			return err
		}
		states, err := uc.repo.CodeStates(ctx, tx, codes)
		if err != nil {
			return err
		}
		var todo []domain.MapAssetTypeDefault
		for _, d := range defaults {
			ref := MapAssetTypeRef{Code: d.Code, Label: d.Label}
			deleted, exists := states[d.Code]
			switch {
			case !exists:
				todo = append(todo, d)
			case deleted:
				res.SkippedDeleted = append(res.SkippedDeleted, ref)
			default:
				res.SkippedExisting = append(res.SkippedExisting, ref)
			}
		}
		if len(todo) == 0 {
			return nil
		}

		// THE CATALOGUE'S CEILING, checked for the whole batch: the read route REFUSES past it, so a seed
		// that crossed it would turn every screen's group selector into a 500 (store/danh_muc_ghi.go).
		live, err := uc.repo.DemDangSong(ctx, tx)
		if err != nil {
			return err
		}
		if live+len(todo) > commsstore.TranDanhMucLoaiTaiNguyen {
			return commsstore.ErrDanhMucDayTran
		}

		created := make([]map[string]any, 0, len(todo))
		for _, d := range todo {
			id, err := uc.newID()
			if err != nil {
				return fmt.Errorf("loai_tai_nguyen_ban_do: sinh id nhóm %s: %w", d.Code, err)
			}
			// Scoped: tx comes from uc.db.For(ctx).Tx — tenant_id is $1 of the INSERT.
			if err := uc.repo.InsertSystemRow(ctx, tx, id, d); err != nil {
				return err
			}
			res.Created = append(res.Created, MapAssetTypeRef{Code: d.Code, Label: d.Label})
			created = append(created, map[string]any{"ma": d.Code, "nhan": d.Label, "thu_tu": d.Order, "nguon": domain.NguonHeThong})
		}

		// Nothing here is personal data: group codes and labels.
		delta, err := json.Marshal(map[string]any{
			"tao":           created,
			"bo_qua_da_co":  refCodes(res.SkippedExisting),
			"bo_qua_da_xoa": refCodes(res.SkippedDeleted),
		})
		if err != nil {
			return fmt.Errorf("loai_tai_nguyen_ban_do: mã hoá delta: %w", err)
		}
		// SAME TRANSACTION AS THE INSERTS (rule 6, invariant 3). The subject names the act — a seed of the
		// catalogue — since it is not one row's code.
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionSeedMapAssetTypes,
			Subject: "loai-tai-nguyen-ban-do/mac-dinh",
			Delta:   delta,
		})
	})
	if err != nil {
		return MapAssetTypeSeedResult{}, boc(ctx, "nạp nhóm mặc định", err)
	}
	return res, nil
}

func refCodes(refs []MapAssetTypeRef) []string {
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		out = append(out, r.Code)
	}
	return out
}
