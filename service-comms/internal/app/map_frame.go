package app

// The commune's map frame — ADR 0072 §"Sửa đổi 04/10/2026", H3; migrations/0016_map_frame.sql.
// Same structure as mail_settings.go: the read is a store call; the save is read-decide-write under a
// row lock with its audit entry in the SAME transaction (rule 6, invariant 3).
//
// NO PERSONAL DATA (0016 §PERSONAL DATA): a commune's centre is a public place, not a home coordinate,
// so the audit delta carries the values themselves, before and after.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// MapFrameRepo is the store, declared at the point of use. The write takes the transaction, so there
// is no signature that writes the row outside the one its entry is in.
type MapFrameRepo interface {
	Get(ctx context.Context) (domain.MapFrame, error)
	LockCommuneFrame(ctx context.Context, tx *store.ScopedTx) error
	ForUpdate(ctx context.Context, tx *store.ScopedTx) (domain.MapFrame, bool, error)
	Upsert(ctx context.Context, tx *store.ScopedTx, f domain.MapFrame, by string) (domain.MapFrame, error)
}

// ActionSaveMapFrame is the business verb written into the trail — a VALUE (ADR 0011).
const ActionSaveMapFrame = "luu_khung_ban_do"

// MapFrameView is what the map page and the form read. Configured false is H3's "Chưa đặt": the page
// draws no map, only the guidance sentence and — for a holder of `admin.lookup` — the form.
type MapFrameView struct {
	Frame      domain.MapFrame
	Bounds     domain.MapFrameBounds
	Configured bool
}

// MapFrameInput is one save as the form sends it, before rounding and validation.
type MapFrameInput struct {
	CenterLat, CenterLng, RadiusKm float64
}

// MapFrames owns reading and saving one commune's map frame.
type MapFrames struct {
	db   *store.DB
	repo MapFrameRepo
}

func NewMapFrames(db *store.DB, repo MapFrameRepo) *MapFrames {
	return &MapFrames{db: db, repo: repo}
}

func mapFrameView(f domain.MapFrame) MapFrameView {
	return MapFrameView{Frame: f, Bounds: f.Bounds(), Configured: true}
}

// Get reads the commune's frame. NOT SET IS NOT AN ERROR: it is a designed state of the page (H3), so
// it is a view with Configured=false — mail_settings' convention for a singleton not yet saved.
func (uc *MapFrames) Get(ctx context.Context) (MapFrameView, error) {
	// Scoped in the store: MapFrameStore.Get reads through db.For(ctx).Query.
	f, err := uc.repo.Get(ctx)
	if errors.Is(err, commsstore.ErrMapFrameNotFound) {
		return MapFrameView{}, nil
	}
	if err != nil {
		return MapFrameView{}, fmt.Errorf("map_frame: đọc cho xã %s: %w", tenant.MustFrom(ctx), err)
	}
	return mapFrameView(f), nil
}

// Save rounds, validates, and writes the row and its audit entry in ONE transaction. A save that
// changes nothing writes and audits nothing — which is what makes the PUT idempotent.
func (uc *MapFrames) Save(ctx context.Context, in MapFrameInput, actor audit.Actor) (MapFrameView, error) {
	if actor.ID == "" {
		return MapFrameView{}, ErrMissingActor
	}
	after, err := domain.NormalizeMapFrame(in.CenterLat, in.CenterLng, in.RadiusKm)
	if err != nil {
		return MapFrameView{}, err
	}

	var saved domain.MapFrame
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// Scoped: tx comes from uc.db.For(ctx).Tx — tenant_id is $1 of every statement.
		// The commune's lock FIRST: FOR UPDATE cannot lock a row that does not exist yet, so without it
		// two first saves both read "not found" and the second entry loses its `truoc`.
		if err := uc.repo.LockCommuneFrame(ctx, tx); err != nil {
			return err
		}
		before, found, err := uc.repo.ForUpdate(ctx, tx)
		if err != nil {
			return err
		}
		if found && domain.SameMapFrame(before, after) {
			saved = before // nothing moved: no UPDATE, no entry
			return nil
		}
		if saved, err = uc.repo.Upsert(ctx, tx, after, actor.ID); err != nil {
			return err
		}

		delta := map[string]any{"sau": mapFrameValues(after)}
		if found {
			delta["truoc"] = mapFrameValues(before)
		}
		b, err := json.Marshal(delta)
		if err != nil {
			return fmt.Errorf("map_frame: mã hoá delta: %w", err)
		}
		// SAME TRANSACTION AS THE UPSERT (rule 6, invariant 3).
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionSaveMapFrame,
			Subject: domain.MapFrameSubject,
			Delta:   b,
		})
	})
	if err != nil {
		return MapFrameView{}, fmt.Errorf("map_frame: lưu cho xã %s: %w", tenant.MustFrom(ctx), err)
	}
	return mapFrameView(saved), nil
}

// mapFrameValues is the full picture for the trail. All three values, both sides: there are only three,
// and "the radius was 12.0 before" answers an inspection without a second lookup.
func mapFrameValues(f domain.MapFrame) map[string]any {
	return map[string]any{"center_lat": f.CenterLat, "center_lng": f.CenterLng, "radius_km": f.RadiusKm}
}
