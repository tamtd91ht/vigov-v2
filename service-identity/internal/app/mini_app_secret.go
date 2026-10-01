package app

// The operator's acts on a commune's OWN Mini App sign-in settings (ADR 0066, both "Đã quyết
// 01/10/2026" tables; migration 0021): set or replace the Zalo app secret, turn the `--demo` fixed
// identity on or off, retire the App ID's settings. Reached from cmd/operatorctl until
// platform-admin has a screen (ADR 0066 decision row 3).
//
// EVERY CHANGE IS A NEW VERSION ROW — retire the live one, insert the next — in ONE transaction with
// ONE audit entry in the TARGET commune's audit_log. The entry names the App ID and whether a secret
// is set, NEVER the value (ADR 0066: "vết kiểm toán không chứa giá trị"; rule 8).
//
// WHO: actor = system and reason "ticket:<n>", exactly the operator realm's CLI rule
// (operator_admin.go): the person at the terminal is not an account of this system, and the ticket is
// what links the entry to a human decision. No ticket, no act — refused before anything is sealed.
//
// THE PLATFORM BINDING IS CHECKED BEFORE ANYTHING THAT ENABLES SIGN-IN (set, demo on): service-platform's
// mini_app is the one place that says which commune an App ID belongs to (migration 0021 §TENANT
// SCOPE). A secret stored under a commune the App ID is not bound to would never be read, and an
// operator would spend a morning finding out why. Retiring and demo-off REDUCE what the App ID can do,
// so they are allowed even after the binding moved — that is exactly when they are needed.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/platformclient"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// Audit verbs — values, Vietnamese (ADR 0011). Subject is the App ID: a Zalo-issued public
// identifier, not personal data, and the business key an inspector would ask about.
const (
	ActionSetOwnAppSecret     = "dat_secret_app_rieng"
	ActionEnableDemoIdentity  = "bat_danh_tinh_demo_app_rieng"
	ActionDisableDemoIdentity = "tat_danh_tinh_demo_app_rieng"
	ActionRetireOwnAppConfig  = "ngung_cau_hinh_app_rieng"
)

// maxAppSecretBytes bounds what is sealed. Zalo app secrets seen are ~20 characters; the bound is
// generous and exists so a pasted file or certificate is refused rather than stored.
const maxAppSecretBytes = 256

var (
	ErrMiniAppIDInvalid         = errors.New("mini app settings: app id must be 1-32 digits")
	ErrMiniAppSecretInvalid     = errors.New("mini app settings: secret must be 1-256 printable characters with no spaces")
	ErrMiniAppNotOwnApp         = errors.New("mini app settings: the platform does not bind this app id to this commune as its own app")
	ErrMiniAppSettingsNotFound  = errors.New("mini app settings: this app id has no live settings in this commune")
	ErrMiniAppSettingsUnchanged = errors.New("mini app settings: already in that state — nothing written")
)

// MiniAppBinding is the platform's App ID → commune read. *platformclient.Directory satisfies it.
type MiniAppBinding interface {
	MiniApp(ctx context.Context, appID string) (platformclient.MiniApp, bool, error)
}

// SecretSealer is the sealing half of *crypto.Envelope.
type SecretSealer interface {
	Seal(ctx context.Context, plaintext secret.Secret, aad []byte) ([]byte, error)
}

// MiniAppSecretRepo is the write half of *idstore.MiniAppSecretStore.
type MiniAppSecretRepo interface {
	LiveForUpdate(ctx context.Context, tx *store.ScopedTx, appID string) (idstore.MiniAppSecret, bool, error)
	Retire(ctx context.Context, tx *store.ScopedTx, id, by, reason string) error
	Insert(ctx context.Context, tx *store.ScopedTx, v idstore.MiniAppSecret) error
}

// miniAppSecretAAD binds sealed bytes to this table, this column, this commune and this App ID —
// NOT the row id, because a version may carry the previous version's bytes verbatim (migration 0021
// §THE SEALED BYTES). Copied onto another commune's row or another App ID's, they do not open.
func miniAppSecretAAD(ctx context.Context, appID string) []byte {
	return []byte("mini_app_secret/app_secret_sealed/" + string(tenant.MustFrom(ctx)) + "/" + appID)
}

// MiniAppSecretAdmin is the operator's use cases. The commune comes from ctx (rule 1, invariant 4).
type MiniAppSecretAdmin struct {
	db      *store.DB
	repo    MiniAppSecretRepo
	sealer  SecretSealer
	binding MiniAppBinding
	now     func() time.Time
}

func NewMiniAppSecretAdmin(db *store.DB, repo MiniAppSecretRepo, sealer SecretSealer, binding MiniAppBinding,
	now func() time.Time) *MiniAppSecretAdmin {
	if db == nil || repo == nil || sealer == nil || binding == nil {
		panic("app.NewMiniAppSecretAdmin: db, repo, sealer and binding are required")
	}
	if now == nil {
		now = time.Now
	}
	return &MiniAppSecretAdmin{db: db, repo: repo, sealer: sealer, binding: binding, now: now}
}

// SetSecret seals value and makes it the App ID's secret. The demo switch carries over unchanged.
func (a *MiniAppSecretAdmin) SetSecret(ctx context.Context, appID string, value secret.Secret, ticket string) error {
	reason, err := ticketReason(ticket, "")
	if err != nil {
		return err
	}
	appID = strings.TrimSpace(appID)
	if !domain.ValidMiniAppID(appID) {
		return ErrMiniAppIDInvalid
	}
	if !validAppSecret(value) {
		return ErrMiniAppSecretInvalid
	}
	if err := a.requireOwnApp(ctx, appID); err != nil {
		return err
	}
	// Sealed BEFORE the transaction, as comms' mail settings do: Seal may create the commune's DEK
	// in its own short transaction (store/data_encryption_key.go).
	sealed, err := a.sealer.Seal(ctx, value, miniAppSecretAAD(ctx, appID))
	if err != nil {
		return fmt.Errorf("mini app settings: seal: %w", err)
	}
	return a.change(ctx, appID, reason, ActionSetOwnAppSecret, func(old idstore.MiniAppSecret, found bool) (*idstore.MiniAppSecret, error) {
		return &idstore.MiniAppSecret{Sealed: sealed, DemoIdentity: found && old.DemoIdentity}, nil
	})
}

// SetDemo turns the App ID's `--demo` fixed identity on or off.
//
// ON is allowed with no secret: stage 2 of ADR 0066's lifecycle runs before anybody has a secret.
// OFF with no secret retires the settings — a row with neither is refused by the schema, and it
// would be an App ID that can sign nobody in.
func (a *MiniAppSecretAdmin) SetDemo(ctx context.Context, appID string, on bool, ticket string) error {
	reason, err := ticketReason(ticket, "")
	if err != nil {
		return err
	}
	appID = strings.TrimSpace(appID)
	if !domain.ValidMiniAppID(appID) {
		return ErrMiniAppIDInvalid
	}
	if on {
		if err := a.requireOwnApp(ctx, appID); err != nil {
			return err
		}
		return a.change(ctx, appID, reason, ActionEnableDemoIdentity, func(old idstore.MiniAppSecret, found bool) (*idstore.MiniAppSecret, error) {
			if found && old.DemoIdentity {
				return nil, ErrMiniAppSettingsUnchanged
			}
			return &idstore.MiniAppSecret{Sealed: old.Sealed, DemoIdentity: true}, nil
		})
	}
	return a.change(ctx, appID, reason, ActionDisableDemoIdentity, func(old idstore.MiniAppSecret, found bool) (*idstore.MiniAppSecret, error) {
		switch {
		case !found:
			return nil, ErrMiniAppSettingsNotFound
		case !old.DemoIdentity:
			return nil, ErrMiniAppSettingsUnchanged
		case old.Sealed == nil:
			return nil, nil // retire only
		}
		return &idstore.MiniAppSecret{Sealed: old.Sealed, DemoIdentity: false}, nil
	})
}

// Retire ends the App ID's settings: its own app signs nobody in until they are set again.
func (a *MiniAppSecretAdmin) Retire(ctx context.Context, appID, ticket string) error {
	reason, err := ticketReason(ticket, "")
	if err != nil {
		return err
	}
	appID = strings.TrimSpace(appID)
	if !domain.ValidMiniAppID(appID) {
		return ErrMiniAppIDInvalid
	}
	return a.change(ctx, appID, reason, ActionRetireOwnAppConfig, func(_ idstore.MiniAppSecret, found bool) (*idstore.MiniAppSecret, error) {
		if !found {
			return nil, ErrMiniAppSettingsNotFound
		}
		return nil, nil
	})
}

// change is the one transaction every act runs: lock the live version, let next decide the new one
// (nil = retire only), retire, insert, audit.
func (a *MiniAppSecretAdmin) change(ctx context.Context, appID, reason, action string,
	next func(old idstore.MiniAppSecret, found bool) (*idstore.MiniAppSecret, error)) error {
	// Scoped: tx comes from a.db.For(ctx).Tx — the store binds tx.TenantID() as $1 of every statement.
	return a.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		old, found, err := a.repo.LiveForUpdate(ctx, tx, appID)
		if err != nil {
			return err
		}
		nv, err := next(old, found)
		if err != nil {
			return err
		}
		if found {
			if err := a.repo.Retire(ctx, tx, old.ID, domain.SystemActor, reason); err != nil {
				return err
			}
		}
		delta := map[string]any{"app_id": appID, "ly_do": reason, "truoc": nil, "sau": nil}
		if found {
			delta["truoc"] = versionView(old)
		}
		if nv != nil {
			id, err := ulid.Moi()
			if err != nil {
				return fmt.Errorf("mini app settings: new id: %w", err)
			}
			nv.ID, nv.AppID, nv.SetAt, nv.SetBy = id, appID, a.now().UTC(), domain.SystemActor
			// Scoped: same tx from a.db.For(ctx).Tx; Insert binds tx.TenantID() as $1.
			if err := a.repo.Insert(ctx, tx, *nv); err != nil {
				return err
			}
			delta["sau"] = versionView(*nv)
		}
		b, err := json.Marshal(delta)
		if err != nil {
			return fmt.Errorf("mini app settings: encode delta: %w", err)
		}
		// SAME transaction as the version change (rule 6, invariant 3), in the target commune.
		return audit.Write(ctx, tx, audit.Entry{
			Actor:   audit.Actor{ID: domain.SystemActor, Kind: "system"},
			Action:  action,
			Subject: appID,
			Delta:   b,
		})
	})
}

// versionView is what the trail holds of a version: whether a secret is set, never its bytes.
func versionView(v idstore.MiniAppSecret) map[string]any {
	return map[string]any{"phien_ban": v.ID, "co_secret": v.Sealed != nil, "danh_tinh_demo": v.DemoIdentity}
}

func (a *MiniAppSecretAdmin) requireOwnApp(ctx context.Context, appID string) error {
	app, found, err := a.binding.MiniApp(ctx, appID)
	if err != nil {
		return fmt.Errorf("mini app settings: read the platform binding: %w", err)
	}
	if !found || app.CheDo != platformclient.CheDoAppRieng || app.XaRieng != tenant.MustFrom(ctx) {
		return ErrMiniAppNotOwnApp
	}
	return nil
}

func validAppSecret(v secret.Secret) bool {
	b := v.Lo()
	if len(b) == 0 || len(b) > maxAppSecretBytes {
		return false
	}
	for _, r := range string(b) {
		if unicode.IsSpace(r) || !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}
