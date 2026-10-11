package app

// The citizen's "Nhận thông báo" switch (migration 0027; owner decision 11/10/2026, ADR 0091 §4).
//
// FLIPPING IT IS A WRITE BY A CITIZEN, so it is audited in the SAME transaction as the row (rule 6,
// invariant 3; ADR 0091 §Hệ quả). The actor is the citizen principal — the opaque citizen id with
// Kind "citizen", the shape service-petitions' citizen writes use (a citizen has no staff code; rule 6
// invariant 8 is about staff).
//
// A NO-OP WRITES AND AUDITS NOTHING: the same tap twice leaves one state and one entry.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// CitizenNotificationPreferenceRepo is the store, declared at the point of use.
type CitizenNotificationPreferenceRepo interface {
	Get(ctx context.Context, citizenID string) (domain.CitizenNotificationPreference, error)
	ForUpdate(ctx context.Context, tx *store.ScopedTx, citizenID string) (domain.CitizenNotificationPreference, error)
	Set(ctx context.Context, tx *store.ScopedTx, citizenID string, zaloEnabled bool) error
}

// CitizenNotificationPreferences owns reading and flipping one citizen's switch.
type CitizenNotificationPreferences struct {
	db   *store.DB
	repo CitizenNotificationPreferenceRepo
}

func NewCitizenNotificationPreferences(db *store.DB, repo CitizenNotificationPreferenceRepo) *CitizenNotificationPreferences {
	return &CitizenNotificationPreferences{db: db, repo: repo}
}

// ActionSetCitizenNotificationPreference is the business verb on the trail — a VALUE (ADR 0011).
const ActionSetCitizenNotificationPreference = "doi_lua_chon_nhan_thong_bao"

// citizenPreferenceSubject is the trail's locator. The record has no business code, and the citizen
// is already the entry's ACTOR (the opaque id), so the subject names the register only — writing the
// citizen id a second time would add nothing but another copy of a handle on a person.
const citizenPreferenceSubject = "lua-chon-nhan-thong-bao"

// ErrNotACitizen — the actor is not a verified citizen principal. The route is behind CitizenOnly +
// XaTuPhien, so reaching this is a wiring fault; nothing is written in someone else's name.
var ErrNotACitizen = errors.New("citizen_notification_preference: chủ thể không phải công dân đã xác thực")

// Get returns the switch of the citizen the actor names, in the commune of ctx.
func (uc *CitizenNotificationPreferences) Get(ctx context.Context, actor audit.Actor) (domain.CitizenNotificationPreference, error) {
	if err := checkCitizenActor(actor); err != nil {
		return domain.CitizenNotificationPreference{}, err
	}
	// Scoped in the store: db.For(ctx).Query binds the commune as $1.
	p, err := uc.repo.Get(ctx, actor.ID)
	if err != nil {
		return domain.CitizenNotificationPreference{}, fmt.Errorf("citizen_notification_preference: đọc cho xã %s: %w",
			tenant.MustFrom(ctx), err)
	}
	return p, nil
}

// Set records the citizen's choice and its audit entry in one transaction.
func (uc *CitizenNotificationPreferences) Set(ctx context.Context, zaloEnabled bool, actor audit.Actor) (
	domain.CitizenNotificationPreference, error) {

	if err := checkCitizenActor(actor); err != nil {
		return domain.CitizenNotificationPreference{}, err
	}
	after := domain.CitizenNotificationPreference{ZaloEnabled: zaloEnabled, Stored: true}
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		before, err := uc.repo.ForUpdate(ctx, tx, actor.ID)
		if err != nil {
			return err
		}
		if before.Stored && before.ZaloEnabled == zaloEnabled {
			return nil
		}
		// Scoped: tx comes from uc.db.For(ctx).Tx — tenant_id is $1 of the INSERT … ON CONFLICT.
		if err := uc.repo.Set(ctx, tx, actor.ID, zaloEnabled); err != nil {
			return err
		}
		// `truoc` is null when no row existed: the citizen had never touched the switch, and the
		// effective value then was "on" — written as `truoc_hieu_luc` so the trail says both facts.
		var storedBefore any
		if before.Stored {
			storedBefore = before.ZaloEnabled
		}
		delta, err := json.Marshal(map[string]any{
			"truoc":          storedBefore,
			"truoc_hieu_luc": before.ZaloEnabled,
			"sau":            zaloEnabled,
		})
		if err != nil {
			return fmt.Errorf("citizen_notification_preference: mã hoá delta: %w", err)
		}
		// SAME TRANSACTION AS THE ROW (rule 6, invariant 3).
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   actor,
			Action:  ActionSetCitizenNotificationPreference,
			Subject: citizenPreferenceSubject,
			Delta:   delta,
		})
	})
	if err != nil {
		return domain.CitizenNotificationPreference{}, fmt.Errorf("citizen_notification_preference: ghi cho xã %s: %w",
			tenant.MustFrom(ctx), err)
	}
	return after, nil
}

func checkCitizenActor(a audit.Actor) error {
	if a.Kind != "citizen" {
		return ErrNotACitizen
	}
	if err := domain.CheckCitizenID(a.ID); err != nil {
		return ErrNotACitizen
	}
	return nil
}
