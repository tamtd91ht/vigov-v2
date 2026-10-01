package app

// THE PORTAL SYNC SCREEN'S USE CASES (docs/ui-ux/11-noi-dung-mini-app.md §3 `Cấu hình`; ADR 0067 §2):
// the settings, the live category tree, the selection, the run history and `⟳ Đồng bộ ngay`.
//
// THE PORTAL KEY FOLLOWS mail_settings.go EXACTLY (0013 says so; ADR 0009):
//
//  1. no read ever returns it — the view says only whether one is set;
//  2. a blank key on save means KEEP the stored one…
//  3. …UNLESS the api_url changed: then the key is required in the same request (ADR 0067 §2 decision
//     5 — the stored key never follows the configuration to another host);
//  4. sealed BEFORE the transaction (Envelope.Seal may open its own to create the commune's DEK), never
//     logged, never in an audit delta — the delta says `api_key_changed: true|false`;
//  5. no KEK in the process → every write and every call refuses with crypto.ErrNotConfigured (503),
//     before anything is written.
//
// THE api_url IS CHECKED BY THE SAME GUARD THAT GUARDS THE CALL (internal/portal.ParseBase →
// CheckURL) before it is stored, so a value the runner would refuse is refused at the screen, with a
// sentence, instead of failing every six hours in a run log.

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/crypto"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-comms/internal/domain"
	"github.com/vihat/vigov/service-comms/internal/portal"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// The verbs in the trail.
const (
	ActionSavePortalSyncSettings = "luu_cau_hinh_dong_bo_cong"
	ActionSavePortalCategories   = "luu_chuyen_muc_dong_bo_cong"
)

var (
	// ErrPortalKeyRequired — nothing is stored yet, so there is no key to keep.
	ErrPortalKeyRequired = errors.New("dong_bo_cong: lần lưu đầu tiên phải nhập mã bảo mật")
	// ErrPortalKeyRequiredForNewURL — rule 3 in the file header. 422.
	ErrPortalKeyRequiredForNewURL = errors.New("dong_bo_cong: đổi địa chỉ API thì phải nhập lại mã bảo mật")
	// ErrPortalMissingActor — no staff business code to name in the trail.
	ErrPortalMissingActor = errors.New("dong_bo_cong: thiếu người thực hiện")
	// ErrPortalUnavailable wraps a call to the portal that failed; the handler answers 502 with the
	// class (PortalCallError) and never the portal's words.
	ErrPortalUnavailable = errors.New("dong_bo_cong: không gọi được Cổng")
)

// PortalCallError carries the class of a failed portal call to the handler. Nothing else: the
// adapter's errors carry no URL and no key, and neither does this.
type PortalCallError struct{ Class string }

func (e *PortalCallError) Error() string   { return ErrPortalUnavailable.Error() + ": " + e.Class }
func (e *PortalCallError) Is(t error) bool { return t == ErrPortalUnavailable }

// PortalSyncSettingsView is what the screen reads. No key field exists to fill.
type PortalSyncSettingsView struct {
	Settings domain.PortalSyncSettings
	// Configured is false for a commune that never saved: the other fields are the defaults.
	Configured bool
	// EncryptionConfigured is false without SECRET_ENCRYPTION_KEYS: saving answers 503.
	EncryptionConfigured bool
}

// SavePortalSyncSettingsRequest is one save. APIKey empty means "keep the stored one".
type SavePortalSyncSettingsRequest struct {
	Input  domain.PortalSyncSettingsInput
	APIKey secret.Secret
}

// PortalCategoryNode is one category of the LIVE tree, with the commune's stored choice merged in.
type PortalCategoryNode struct {
	ExternalID string
	Name       string
	ParentID   string
	ParentName string
	IsSelected bool
	TargetKind string // "" when the commune never selected it
}

// PortalCategoryTree is the live tree, plus the stored SELECTED categories the portal no longer lists
// (still selected; a run reading them fails and says so).
type PortalCategoryTree struct {
	Items   []PortalCategoryNode
	Missing []domain.PortalCategory
}

// PortalSyncAdmin owns the screen's use cases.
type PortalSyncAdmin struct {
	db       *store.DB
	repo     PortalSyncRepo
	envelope *crypto.Envelope // nil when SECRET_ENCRYPTION_KEYS is unset
	client   PortalClient
	runner   *PortalSyncRunner
	newID    func() (string, error)
	log      *slog.Logger // the runner's: one stream for the security events of both (R2)
}

// NewPortalSyncAdmin builds the use cases. runner is the process's one runner (manual runs).
func NewPortalSyncAdmin(db *store.DB, repo PortalSyncRepo, envelope *crypto.Envelope, client PortalClient,
	runner *PortalSyncRunner) *PortalSyncAdmin {
	log := slog.Default()
	if runner != nil && runner.log != nil {
		log = runner.log
	}
	return &PortalSyncAdmin{db: db, repo: repo, envelope: envelope, client: client, runner: runner, newID: ulid.Moi,
		log: log}
}

// Settings reads the commune's settings; an unconfigured commune gets the defaults and
// Configured=false, never a 404 — this screen is where the first configuration is typed.
func (uc *PortalSyncAdmin) Settings(ctx context.Context) (PortalSyncSettingsView, error) {
	v := PortalSyncSettingsView{EncryptionConfigured: uc.envelope != nil}
	st, err := uc.repo.Settings(ctx)
	if errors.Is(err, commsstore.ErrPortalSyncSettingsNotFound) {
		v.Settings = domain.DefaultPortalSyncSettings()
		return v, nil
	}
	if err != nil {
		return PortalSyncSettingsView{}, fmt.Errorf("dong_bo_cong: đọc cấu hình cho xã %s: %w", tenant.MustFrom(ctx), err)
	}
	// D1: a row saved above today's ceilings READS at the ceilings — what the next run will use and what
	// the next save will write. The row itself is not rewritten (rule 7).
	st, _ = st.Clamped()
	v.Settings, v.Configured = st, true
	return v, nil
}

// SaveSettings validates, seals a new key if one was typed, and writes the row and its audit entry in
// ONE transaction. A save that moves nothing and carries no key writes and audits nothing.
func (uc *PortalSyncAdmin) SaveSettings(ctx context.Context, req SavePortalSyncSettingsRequest,
	actor audit.Actor) (PortalSyncSettingsView, error) {

	if uc.envelope == nil {
		return PortalSyncSettingsView{}, crypto.ErrNotConfigured
	}
	if actor.ID == "" {
		return PortalSyncSettingsView{}, ErrPortalMissingActor
	}
	// Shape on the defaults first: a malformed request is refused before any statement runs.
	if _, err := domain.MergePortalSyncSettings(domain.DefaultPortalSyncSettings(), req.Input); err != nil {
		return PortalSyncSettingsView{}, err
	}
	if _, err := portal.ParseBase(req.Input.APIURL); err != nil {
		return PortalSyncSettingsView{}, domain.ErrPortalAPIURLInvalid
	}
	var newSealed []byte
	if len(req.APIKey) > 0 {
		if err := domain.ValidatePortalAPIKey(req.APIKey.Lo()); err != nil {
			return PortalSyncSettingsView{}, err
		}
		var err error
		// BOUND TO THE api_url BEING SAVED (R6): the same trimmed value MergePortalSyncSettings stores.
		if newSealed, err = uc.envelope.Seal(ctx, req.APIKey, portalKeyAAD(ctx, req.Input.APIURL)); err != nil {
			return PortalSyncSettingsView{}, fmt.Errorf("dong_bo_cong: niêm mã bảo mật: %w", err)
		}
	}

	var saved domain.PortalSyncSettings
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		before, oldSealed, found, err := uc.repo.SettingsForUpdate(ctx, tx)
		if err != nil {
			return err
		}
		base := domain.DefaultPortalSyncSettings()
		if found {
			// D1: merged onto the CLAMPED row, so an omitted window/ceiling saves the allowed value instead
			// of refusing the whole form for a number staff did not type. The audit diff below compares
			// against the row AS STORED, so lowering 365 → 90 on that save is recorded.
			base, _ = before.Clamped()
		}
		after, err := domain.MergePortalSyncSettings(base, req.Input)
		if err != nil {
			return err
		}
		final := newSealed
		if final == nil {
			switch {
			case !found || len(oldSealed) == 0:
				return ErrPortalKeyRequired
			case domain.PortalAPIURLChanged(before, after):
				return ErrPortalKeyRequiredForNewURL
			case domain.SamePortalSyncSettings(before, after):
				saved = before
				return nil // nothing moved: no UPDATE, no entry
			}
			final = oldSealed
		}
		if err := uc.repo.UpsertSettings(ctx, tx, after, final, actor.ID); err != nil {
			return err
		}
		// SAME TRANSACTION AS THE UPSERT (rule 6, invariant 3). NEVER THE KEY, NEVER THE SEALED BYTES:
		// only whether one was set again.
		d := map[string]any{"api_key_changed": newSealed != nil}
		if found {
			d["truoc"], d["sau"] = diffPortalSettings(before, after, true), diffPortalSettings(before, after, false)
		} else {
			d["sau"] = summarizePortalSettings(after)
		}
		after.APIKeySet = true
		after.LastRunAt = before.LastRunAt
		after.UpdatedBy = actor.ID
		saved = after
		return writePortalAudit(ctx, tx, actor, ActionSavePortalSyncSettings, domain.PortalSyncSettingsSubject,
			time.Now().UTC(), d)
	})
	if err != nil {
		for _, r := range []error{ErrPortalKeyRequired, ErrPortalKeyRequiredForNewURL} {
			if errors.Is(err, r) {
				return PortalSyncSettingsView{}, r
			}
		}
		return PortalSyncSettingsView{}, fmt.Errorf("dong_bo_cong: lưu cấu hình cho xã %s: %w", tenant.MustFrom(ctx), err)
	}
	return PortalSyncSettingsView{Settings: saved, Configured: true, EncryptionConfigured: true}, nil
}

func summarizePortalSettings(s domain.PortalSyncSettings) map[string]any {
	return map[string]any{
		"provider": s.Provider, "api_url": s.APIURL, "publish_mode": s.PublishMode,
		"interval_hours": s.IntervalHours, "window_days": s.WindowDays, "max_items_per_run": s.MaxItemsPerRun,
		"keep_source_credit": s.KeepSourceCredit, "is_enabled": s.IsEnabled,
	}
}

// diffPortalSettings returns only the fields that moved, from the side asked for. The api_url carries
// no secret (the key is a separate column, and a base with a query is refused by ParseBase).
func diffPortalSettings(before, after domain.PortalSyncSettings, beforeSide bool) map[string]any {
	out := map[string]any{}
	pick := func(k string, b, a any) {
		if b != a {
			out[k] = chon(beforeSide, b, a)
		}
	}
	pick("api_url", before.APIURL, after.APIURL)
	pick("publish_mode", before.PublishMode, after.PublishMode)
	pick("interval_hours", before.IntervalHours, after.IntervalHours)
	pick("window_days", before.WindowDays, after.WindowDays)
	pick("max_items_per_run", before.MaxItemsPerRun, after.MaxItemsPerRun)
	pick("keep_source_credit", before.KeepSourceCredit, after.KeepSourceCredit)
	pick("is_enabled", before.IsEnabled, after.IsEnabled)
	return out
}

// endpoint opens the stored key for a call. actor names who asked, for the security event of a refused
// address (R2).
func (uc *PortalSyncAdmin) endpoint(ctx context.Context, actor audit.Actor) (portal.Endpoint, func(), error) {
	if uc.envelope == nil {
		return portal.Endpoint{}, nil, crypto.ErrNotConfigured
	}
	st, sealed, err := uc.repo.SettingsWithKey(ctx)
	if err != nil {
		return portal.Endpoint{}, nil, err
	}
	base, err := portal.ParseBase(st.APIURL)
	if err != nil {
		if portal.IsRefusal(err) {
			logOutboundRefused(ctx, uc.log, actor.ID, "", "api", err)
		}
		return portal.Endpoint{}, nil, &PortalCallError{Class: portal.ClassOf(err)}
	}
	key, err := openPortalKey(ctx, uc.db, uc.repo, uc.envelope, st, sealed, uc.log)
	if err != nil {
		return portal.Endpoint{}, nil, fmt.Errorf("dong_bo_cong: mở mã bảo mật cho xã %s: %w", tenant.MustFrom(ctx), err)
	}
	return portal.Endpoint{Base: base, Key: key}, func() { clear(key) }, nil
}

// CategoryTree asks the portal for its category tree NOW (ADR 0067 §2 "Chế độ đăng" #4: the tree is
// never copied) and merges the commune's stored choice into it. NOT AUDITED: it reads, in the commune
// it belongs to, from the host the commune saved — no destination a person chose at this moment. Its
// route needs `content.update` (owner, 02/10/2026, D3): the call spends the commune's secret.
func (uc *PortalSyncAdmin) CategoryTree(ctx context.Context, actor audit.Actor) (PortalCategoryTree, error) {
	ep, done, err := uc.endpoint(ctx, actor)
	if err != nil {
		return PortalCategoryTree{}, err
	}
	defer done()
	live, err := uc.client.Categories(ctx, ep)
	if err != nil {
		if portal.IsRefusal(err) {
			logOutboundRefused(ctx, uc.log, actor.ID, "", "api", err)
		}
		return PortalCategoryTree{}, &PortalCallError{Class: portal.ClassOf(err)}
	}
	stored, err := uc.repo.Categories(ctx)
	if err != nil {
		return PortalCategoryTree{}, fmt.Errorf("dong_bo_cong: đọc chuyên mục đã chọn cho xã %s: %w", tenant.MustFrom(ctx), err)
	}
	byExt := make(map[string]domain.PortalCategory, len(stored))
	for _, c := range stored {
		byExt[c.ExternalID] = c
	}
	out := PortalCategoryTree{Items: make([]PortalCategoryNode, 0, len(live)), Missing: []domain.PortalCategory{}}
	onPortal := make(map[string]bool, len(live))
	for _, c := range live {
		onPortal[c.ExternalID] = true
		n := PortalCategoryNode{ExternalID: c.ExternalID, Name: c.Name, ParentID: c.ParentID, ParentName: c.ParentName}
		if s, ok := byExt[c.ExternalID]; ok {
			n.IsSelected, n.TargetKind = s.IsSelected, s.TargetKind
		}
		out.Items = append(out.Items, n)
	}
	for _, s := range stored {
		if s.IsSelected && !onPortal[s.ExternalID] {
			out.Missing = append(out.Missing, s)
		}
	}
	return out, nil
}

// SaveCategories upserts the commune's selection in ONE transaction with ONE audit entry. A new row is
// written only for a category being SELECTED (0013: a row exists for a category selected at least
// once); an unticked one that has no row writes nothing. An entry that changes nothing writes nothing,
// and a save that changes nothing at all files no entry. Categories absent from the request are left
// as they are — the screen sends what it shows.
func (uc *PortalSyncAdmin) SaveCategories(ctx context.Context, sel []domain.PortalCategorySelection,
	actor audit.Actor) ([]domain.PortalCategory, error) {

	if actor.ID == "" {
		return nil, ErrPortalMissingActor
	}
	clean, err := domain.NormalizePortalSelection(sel)
	if err != nil {
		return nil, err
	}
	var result []domain.PortalCategory
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		rows, err := uc.repo.CategoriesForUpdate(ctx, tx)
		if err != nil {
			return err
		}
		byExt := make(map[string]domain.PortalCategory, len(rows))
		for _, c := range rows {
			byExt[c.ExternalID] = c
		}
		var changes []map[string]any
		for _, s := range clean {
			cur, exists := byExt[s.ExternalID]
			switch {
			case !exists && !s.IsSelected:
				continue
			case !exists:
				id, err := uc.newID()
				if err != nil {
					return fmt.Errorf("dong_bo_cong: sinh mã chuyên mục: %w", err)
				}
				c := domain.PortalCategory{ID: id, ExternalID: s.ExternalID, Name: s.Name, TargetKind: s.TargetKind,
					IsSelected: true}
				if err := uc.repo.InsertCategory(ctx, tx, c, actor.ID); err != nil {
					return err
				}
				byExt[s.ExternalID] = c
				changes = append(changes, map[string]any{"external_id": s.ExternalID, "name": s.Name,
					"target_kind": s.TargetKind, "is_selected": true, "new": true})
			case cur.Name != s.Name || cur.TargetKind != s.TargetKind || cur.IsSelected != s.IsSelected:
				next := cur
				next.Name, next.TargetKind, next.IsSelected = s.Name, s.TargetKind, s.IsSelected
				if err := uc.repo.UpdateCategory(ctx, tx, next, actor.ID); err != nil {
					return err
				}
				byExt[s.ExternalID] = next
				changes = append(changes, map[string]any{"external_id": s.ExternalID,
					"truoc": map[string]any{"name": cur.Name, "target_kind": cur.TargetKind, "is_selected": cur.IsSelected},
					"sau":   map[string]any{"name": next.Name, "target_kind": next.TargetKind, "is_selected": next.IsSelected}})
			}
		}
		selected := 0
		for _, c := range byExt {
			result = append(result, c)
			if c.IsSelected {
				selected++
			}
		}
		// D1: the ceiling is on what the commune HAS selected after this save — the rows absent from the
		// body keep their state, so the request alone cannot decide it. Refused before the entry: the
		// transaction rolls back every row above.
		if err := domain.CheckPortalSelectedCount(selected); err != nil {
			return err
		}
		if len(changes) == 0 {
			return nil
		}
		// A category NAME is an editorial label of the portal, not personal data (0013).
		return writePortalAudit(ctx, tx, actor, ActionSavePortalCategories, "dong-bo-cong/chuyen-muc",
			time.Now().UTC(), map[string]any{"changes": changes})
	})
	if err != nil {
		if errors.Is(err, domain.ErrPortalTooManySelected) {
			return nil, err
		}
		return nil, fmt.Errorf("dong_bo_cong: lưu chuyên mục cho xã %s: %w", tenant.MustFrom(ctx), err)
	}
	sortPortalCategories(result)
	return result, nil
}

func sortPortalCategories(cs []domain.PortalCategory) {
	for i := 1; i < len(cs); i++ {
		for j := i; j > 0 && (cs[j].Name < cs[j-1].Name ||
			(cs[j].Name == cs[j-1].Name && cs[j].ExternalID < cs[j-1].ExternalID)); j-- {
			cs[j], cs[j-1] = cs[j-1], cs[j]
		}
	}
}

// Runs reads one page of the run history, newest first.
func (uc *PortalSyncAdmin) Runs(ctx context.Context, req page.Request) (page.Result[domain.PortalSyncRun], error) {
	res, err := uc.repo.Runs(ctx, req)
	if err != nil {
		return res, fmt.Errorf("dong_bo_cong: đọc lịch sử chạy cho xã %s: %w", tenant.MustFrom(ctx), err)
	}
	return res, nil
}

// StartRun is `⟳ Đồng bộ ngay` (PortalSyncRunner.StartManual).
func (uc *PortalSyncAdmin) StartRun(ctx context.Context, actor audit.Actor) (domain.PortalSyncRun, error) {
	if uc.runner == nil {
		return domain.PortalSyncRun{}, ErrPortalRunnerStopped
	}
	return uc.runner.StartManual(ctx, actor)
}
