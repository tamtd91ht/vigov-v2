package app

// The use cases behind the STAFF write surface of disaster alerts (migration 0026; owner decision
// 11/10/2026, ADR 0091 §2 — a tab of "Nội dung Mini App", `content.read` / `content.update`).
//
// external_contact.go's structure and for the same two reasons: the audit entry shares the business
// write's transaction (rule 6, invariant 3), and every edit is read-decide-write under a row lock.
//
// PERSONAL DATA IN THE TRAIL (rule 3, rule 6 forbidden #4). 0026 treats the contact names and every
// phone in the two lists as possibly an officer's own. So the delta carries phones MASKED
// (privacy.MaskPhone), contact names MASKED (privacy.MaskName) and evacuation addresses as presence only.
// Title, headline and detail are free text that can name a household ("nhà ông …"), so — content items'
// rule (tomTatNoiDungMiniApp) — they enter the trail as "changed" only; kind, level and the times enter by
// value; the danger zone by point count (relative coordinates name nobody, but 200 points of them say
// nothing to an inspector either).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/privacy"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// DisasterAlertRepo is the store, declared at the point of use. Every write takes the transaction.
type DisasterAlertRepo interface {
	Insert(ctx context.Context, tx *store.ScopedTx, a domain.DisasterAlert, by string) error
	ByIDForUpdate(ctx context.Context, tx *store.ScopedTx, id string) (domain.DisasterAlert, error)
	Update(ctx context.Context, tx *store.ScopedTx, a domain.DisasterAlert, by string) error
	End(ctx context.Context, tx *store.ScopedTx, id, by string) error
	SoftDelete(ctx context.Context, tx *store.ScopedTx, id, by, reason string) error
}

// DisasterAlerts owns posting, correcting, ending early and retiring one commune's alerts.
type DisasterAlerts struct {
	db   *store.DB
	repo DisasterAlertRepo

	// newID and now are injected so a test can pin them. In production: ulid.Moi and time.Now.
	newID func() (string, error)
	now   func() time.Time
}

func NewDisasterAlerts(db *store.DB, repo DisasterAlertRepo) *DisasterAlerts {
	return &DisasterAlerts{db: db, repo: repo, newID: ulid.Moi, now: time.Now}
}

// The business verbs on the trail — VALUES (ADR 0011), naming the register.
const (
	ActionCreateDisasterAlert = "dang_canh_bao_thien_tai"
	ActionUpdateDisasterAlert = "sua_canh_bao_thien_tai"
	ActionEndDisasterAlert    = "ket_thuc_canh_bao_thien_tai"
	ActionDeleteDisasterAlert = "xoa_canh_bao_thien_tai"
)

// ErrDisasterAlertMissingActor — no business code: a change nobody can be asked about (rule 6, inv. 8).
var ErrDisasterAlertMissingActor = errors.New("disaster_alert: thiếu mã cán bộ thực hiện")

// DisasterAlertInput is one new alert as it arrives from the handler. Zero EffectiveTo = open-ended.
type DisasterAlertInput struct {
	Kind              string
	Title             string
	Level             string
	Headline          string
	Detail            string
	EffectiveFrom     time.Time
	EffectiveTo       time.Time
	DangerZone        []domain.DangerZonePoint
	EvacuationPoints  []domain.EvacuationPoint
	EmergencyContacts []domain.EmergencyContact
}

// DisasterAlertPatch is a PARTIAL edit: nil means "leave this alone". Level / Headline "" CLEAR them;
// EffectiveTo pointing at the zero time clears it (open-ended); a list pointer REPLACES the whole list
// (an empty list clears it).
type DisasterAlertPatch struct {
	Kind              *string
	Title             *string
	Level             *string
	Headline          *string
	Detail            *string
	EffectiveFrom     *time.Time
	EffectiveTo       *time.Time
	DangerZone        *[]domain.DangerZonePoint
	EvacuationPoints  *[]domain.EvacuationPoint
	EmergencyContacts *[]domain.EmergencyContact
}

func wrapDisasterAlert(ctx context.Context, op string, err error) error {
	return fmt.Errorf("disaster_alert: %s cho xã %s: %w", op, tenant.MustFrom(ctx), err)
}

// Create posts one alert. Shape first (no transaction), then the row and its entry in one.
func (uc *DisasterAlerts) Create(ctx context.Context, in DisasterAlertInput, actor audit.Actor) (domain.DisasterAlert, error) {
	if actor.ID == "" {
		return domain.DisasterAlert{}, ErrDisasterAlertMissingActor
	}
	row, err := domain.NormalizeDisasterAlert(domain.DisasterAlert{
		Kind: domain.DisasterKind(in.Kind), Title: in.Title, Level: in.Level, Headline: in.Headline,
		Detail: in.Detail, EffectiveFrom: in.EffectiveFrom, EffectiveTo: in.EffectiveTo,
		DangerZone: in.DangerZone, EvacuationPoints: in.EvacuationPoints, EmergencyContacts: in.EmergencyContacts,
	})
	if err != nil {
		return domain.DisasterAlert{}, err
	}
	if row.ID, err = uc.newID(); err != nil {
		return domain.DisasterAlert{}, fmt.Errorf("disaster_alert: sinh id: %w", err)
	}
	row.CreatedBy, row.UpdatedBy = actor.ID, actor.ID

	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		// created_by / updated_by are the actor's BUSINESS CODE: the handler builds audit.Actor.ID from
		// Principal.Ma (rule 6, invariant 8). Scoped: tenant_id is $1 of the INSERT.
		if err := uc.repo.Insert(ctx, tx, row, actor.ID); err != nil {
			return err
		}
		delta, err := json.Marshal(map[string]any{"sau": disasterAlertForAudit(row)})
		if err != nil {
			return fmt.Errorf("disaster_alert: mã hoá delta: %w", err)
		}
		// SAME TRANSACTION AS THE INSERT (rule 6, invariant 3).
		return audit.Write(ctx, tx, audit.Entry{
			Actor: actor, Action: ActionCreateDisasterAlert, Subject: disasterAlertSubject(row), Delta: delta,
		})
	})
	if err != nil {
		return domain.DisasterAlert{}, wrapDisasterAlert(ctx, "đăng", err)
	}
	return row, nil
}

// Update applies a partial edit to a LIVE, NOT-ENDED alert. A no-op writes and audits nothing — which
// is what makes the route's idem.KhongCan true.
func (uc *DisasterAlerts) Update(ctx context.Context, id string, p DisasterAlertPatch, actor audit.Actor) (domain.DisasterAlert, error) {
	if id == "" {
		return domain.DisasterAlert{}, commsstore.ErrDisasterAlertNotFound
	}
	if actor.ID == "" {
		return domain.DisasterAlert{}, ErrDisasterAlertMissingActor
	}

	// A REFUSAL IS RETURNED UNWRAPPED: the handler sends its sentence to the client, and the wrap below
	// names the commune — which a client message must not carry.
	var after domain.DisasterAlert
	var refusal error
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		before, err := uc.repo.ByIDForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if before.Ended() {
			// Residents were shown this text until it ended; it is a historical record now (0026).
			refusal = domain.ErrDisasterEnded
			return refusal
		}
		merged := mergeDisasterAlert(before, p)
		// Judged on the row AFTER the merge: the window rule needs both ends.
		if after, err = domain.NormalizeDisasterAlert(merged); err != nil {
			refusal = err
			return refusal
		}
		if sameDisasterAlert(before, after) {
			after = before
			return nil
		}
		// Scoped: tenant_id is $1 of the UPDATE.
		if err := uc.repo.Update(ctx, tx, after, actor.ID); err != nil {
			return err
		}
		after.UpdatedBy = actor.ID
		beforeSide, afterSide := diffDisasterAlert(before, after)
		delta, err := json.Marshal(map[string]any{"id": after.ID, "truoc": beforeSide, "sau": afterSide})
		if err != nil {
			return fmt.Errorf("disaster_alert: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor: actor, Action: ActionUpdateDisasterAlert, Subject: disasterAlertSubject(after), Delta: delta,
		})
	})
	if refusal != nil {
		return domain.DisasterAlert{}, refusal
	}
	if err != nil {
		return domain.DisasterAlert{}, wrapDisasterAlert(ctx, "sửa", err)
	}
	return after, nil
}

// End ends a live alert early: it leaves the Mini App in this statement. A second end is refused with
// ErrDisasterEnded and overwrites nothing.
func (uc *DisasterAlerts) End(ctx context.Context, id string, actor audit.Actor) (domain.DisasterAlert, error) {
	if id == "" {
		return domain.DisasterAlert{}, commsstore.ErrDisasterAlertNotFound
	}
	if actor.ID == "" {
		return domain.DisasterAlert{}, ErrDisasterAlertMissingActor
	}
	var after domain.DisasterAlert
	var refusal error
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		before, err := uc.repo.ByIDForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if before.Ended() {
			refusal = domain.ErrDisasterEnded
			return refusal
		}
		// Scoped: tenant_id is $1 of the UPDATE.
		if err := uc.repo.End(ctx, tx, before.ID, actor.ID); err != nil {
			return err
		}
		// The database stamps now(); the returned row carries this process's reading of the same act.
		after = before
		after.EndedAt, after.EndedBy, after.UpdatedBy = uc.now().UTC(), actor.ID, actor.ID
		delta, err := json.Marshal(map[string]any{
			"id":                  before.ID,
			"was_in_force":        before.InForce(after.EndedAt),
			"effective_from":      instantOrEmpty(before.EffectiveFrom),
			"effective_to_before": instantOrEmpty(before.EffectiveTo),
		})
		if err != nil {
			return fmt.Errorf("disaster_alert: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor: actor, Action: ActionEndDisasterAlert, Subject: disasterAlertSubject(before), Delta: delta,
		})
	})
	if refusal != nil {
		return domain.DisasterAlert{}, refusal
	}
	if err != nil {
		return domain.DisasterAlert{}, wrapDisasterAlert(ctx, "kết thúc", err)
	}
	return after, nil
}

// Delete soft deletes one alert with a mandatory reason (rule 7, invariant 1). Allowed on an ended alert
// too — 0026's guard lets exactly the soft-delete trio move.
func (uc *DisasterAlerts) Delete(ctx context.Context, id, rawReason string, actor audit.Actor) error {
	if id == "" {
		return commsstore.ErrDisasterAlertNotFound
	}
	reason, err := domain.ChuanHoaLyDoXoa(rawReason)
	if err != nil {
		return err
	}
	if actor.ID == "" {
		return ErrDisasterAlertMissingActor
	}
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		before, err := uc.repo.ByIDForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		// Scoped: tenant_id is $1 of the UPDATE.
		if err := uc.repo.SoftDelete(ctx, tx, before.ID, actor.ID, reason); err != nil {
			return err
		}
		delta, err := json.Marshal(map[string]any{
			"truoc":   disasterAlertForAudit(before),
			"ly_do":   reason,
			"xoa_mem": true,
		})
		if err != nil {
			return fmt.Errorf("disaster_alert: mã hoá delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{
			Actor: actor, Action: ActionDeleteDisasterAlert, Subject: disasterAlertSubject(before), Delta: delta,
		})
	})
	if err != nil {
		return wrapDisasterAlert(ctx, "xoá", err)
	}
	return nil
}

func mergeDisasterAlert(a domain.DisasterAlert, p DisasterAlertPatch) domain.DisasterAlert {
	if p.Kind != nil {
		a.Kind = domain.DisasterKind(*p.Kind)
	}
	if p.Title != nil {
		a.Title = *p.Title
	}
	if p.Level != nil {
		a.Level = *p.Level
	}
	if p.Headline != nil {
		a.Headline = *p.Headline
	}
	if p.Detail != nil {
		a.Detail = *p.Detail
	}
	if p.EffectiveFrom != nil {
		a.EffectiveFrom = *p.EffectiveFrom
	}
	if p.EffectiveTo != nil {
		a.EffectiveTo = *p.EffectiveTo
	}
	if p.DangerZone != nil {
		a.DangerZone = *p.DangerZone
	}
	if p.EvacuationPoints != nil {
		a.EvacuationPoints = *p.EvacuationPoints
	}
	if p.EmergencyContacts != nil {
		a.EmergencyContacts = *p.EmergencyContacts
	}
	return a
}

// disasterAlertSubject is the trail's locator. An alert has no business code (0026 mints none) and a
// ULID names nothing to the person handling a complaint, so: the register, the kind and the start date
// — what a complaint quotes ("the flood warning of 12/10"). The title is NOT used: it is free text.
func disasterAlertSubject(a domain.DisasterAlert) string {
	return "canh-bao-thien-tai/" + string(a.Kind) + "/" + a.EffectiveFrom.UTC().Format("2006-01-02")
}

// disasterAlertForAudit is the delta's view of one row (see the file comment for what is masked).
func disasterAlertForAudit(a domain.DisasterAlert) map[string]any {
	return map[string]any{
		"id":                 a.ID,
		"kind":               string(a.Kind),
		"level":              a.Level,
		"has_headline":       a.Headline != "",
		"effective_from":     instantOrEmpty(a.EffectiveFrom),
		"effective_to":       instantOrEmpty(a.EffectiveTo),
		"ended_at":           instantOrEmpty(a.EndedAt),
		"danger_zone_points": len(a.DangerZone),
		"evacuation_points":  evacuationForAudit(a.EvacuationPoints),
		"emergency_contacts": contactsForAudit(a.EmergencyContacts),
	}
}

func evacuationForAudit(pts []domain.EvacuationPoint) []map[string]any {
	out := make([]map[string]any, 0, len(pts))
	for _, p := range pts {
		out = append(out, map[string]any{
			"name":        p.Name, // a place (school, cultural house) — 0026 lists only the phone as possibly personal
			"has_address": p.Address != "",
			"phone":       privacy.MaskPhone(p.Phone),
			"capacity":    orderOrNil(p.Capacity),
		})
	}
	return out
}

func contactsForAudit(cs []domain.EmergencyContact) []map[string]any {
	out := make([]map[string]any, 0, len(cs))
	for _, c := range cs {
		out = append(out, map[string]any{
			"role":  c.Role,
			"name":  privacy.MaskName(c.Name),
			"phone": privacy.MaskPhone(c.Phone),
		})
	}
	return out
}

// diffDisasterAlert returns only what moved, one map per side (rule 6, invariant 5), masked as above.
func diffDisasterAlert(before, after domain.DisasterAlert) (map[string]any, map[string]any) {
	b, a := map[string]any{}, map[string]any{}
	if before.Kind != after.Kind {
		b["kind"], a["kind"] = string(before.Kind), string(after.Kind)
	}
	if before.Level != after.Level {
		b["level"], a["level"] = before.Level, after.Level
	}
	if before.Title != after.Title {
		a["title_changed"] = true
	}
	if before.Headline != after.Headline {
		b["has_headline"], a["has_headline"] = before.Headline != "", after.Headline != ""
		a["headline_changed"] = true
	}
	if before.Detail != after.Detail {
		a["detail_changed"] = true
	}
	if !before.EffectiveFrom.Equal(after.EffectiveFrom) {
		b["effective_from"], a["effective_from"] = instantOrEmpty(before.EffectiveFrom), instantOrEmpty(after.EffectiveFrom)
	}
	if !before.EffectiveTo.Equal(after.EffectiveTo) {
		b["effective_to"], a["effective_to"] = instantOrEmpty(before.EffectiveTo), instantOrEmpty(after.EffectiveTo)
	}
	if !sameJSON(before.DangerZone, after.DangerZone) {
		b["danger_zone_points"], a["danger_zone_points"] = len(before.DangerZone), len(after.DangerZone)
		a["danger_zone_changed"] = true
	}
	if !sameJSON(before.EvacuationPoints, after.EvacuationPoints) {
		b["evacuation_points"], a["evacuation_points"] = evacuationForAudit(before.EvacuationPoints),
			evacuationForAudit(after.EvacuationPoints)
	}
	if !sameJSON(before.EmergencyContacts, after.EmergencyContacts) {
		b["emergency_contacts"], a["emergency_contacts"] = contactsForAudit(before.EmergencyContacts),
			contactsForAudit(after.EmergencyContacts)
	}
	return b, a
}

func sameDisasterAlert(x, y domain.DisasterAlert) bool {
	return x.Kind == y.Kind && x.Title == y.Title && x.Level == y.Level && x.Headline == y.Headline &&
		x.Detail == y.Detail && x.EffectiveFrom.Equal(y.EffectiveFrom) && x.EffectiveTo.Equal(y.EffectiveTo) &&
		sameJSON(x.DangerZone, y.DangerZone) && sameJSON(x.EvacuationPoints, y.EvacuationPoints) &&
		sameJSON(x.EmergencyContacts, y.EmergencyContacts)
}

// sameJSON compares two lists by their stored form — the form the column holds, so "equal" means "the
// UPDATE would write the same bytes". Both sides are already normalised (nil → empty).
func sameJSON(x, y any) bool {
	bx, errX := json.Marshal(x)
	by, errY := json.Marshal(y)
	return errX == nil && errY == nil && string(bx) == string(by)
}
