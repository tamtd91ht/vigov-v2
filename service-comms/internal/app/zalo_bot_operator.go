package app

// The operator's acts on the ONE shared platform Zalo Bot (ADR 0074; the contract, and the spec of every
// step below, is proto/vigov/comms/v1/zalo_bot_operator.proto — read its comments, they are not restated
// here). Reached only through ZaloBotOperatorService, from service-platform's operator routes after its
// `ops.zalo_bot.manage` check, which comms TRUSTS (ADR 0074 #3).
//
// PLATFORM SCOPE. Nothing here reads a commune from the context; the shared bot belongs to none (ADR
// 0074 #4). The store is internal/store/platformstore, the sealing is crypto.PlatformEnvelope, the trail
// is platform_operator_audit_log — plus, on a change of bot account, one entry per ended link in that
// link's own commune audit_log.
//
// THE TOKEN AND THE WEBHOOK secret_token ARE WRITE-ONLY. No return value of this file carries either,
// nor anything derived from either. Zalo failures are CLASSES (domain.ZaloCall*), never text — the
// token sits in the URL of every Zalo call (internal/zalobot).
//
// THE ORDER OF EVERY ACT: validate the request alone (INVALID_ARGUMENT, nothing read) → the Zalo call,
// OUTSIDE any transaction → seal, OUTSIDE the transaction (a first seal creates the platform DEK in its
// own short transaction — mail_settings.go's reason) → ONE transaction with the change and its trail
// entry. platformstore.Tx refuses to commit a write without its entry.

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/crypto"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/service-comms/internal/domain"
	"github.com/vihat/vigov/service-comms/internal/store/platformstore"
	"github.com/vihat/vigov/service-comms/internal/zalobot"
)

// ZaloBotAPI is the Zalo adapter (internal/zalobot), declared at the point of use.
type ZaloBotAPI interface {
	GetMe(ctx context.Context, token secret.Secret) (zalobot.BotInfo, zalobot.Outcome)
	SetWebhook(ctx context.Context, token secret.Secret, webhookURL string, secretToken secret.Secret) zalobot.Outcome
	GetWebhookInfo(ctx context.Context, token secret.Secret) (zalobot.WebhookInfo, zalobot.Outcome)
}

// ZaloBotStore is the platform store, declared at the point of use.
type ZaloBotStore interface {
	SharedBot(ctx context.Context) (domain.SharedZaloBot, bool, error)
	SharedBotToken(ctx context.Context) ([]byte, bool, error)
	PendingWebhookSecretRead(ctx context.Context) ([]byte, error)
	InTx(ctx context.Context, fn func(ZaloBotTx) error) error
}

// ZaloBotTx is one platform transaction. *platformstore.Tx satisfies it.
type ZaloBotTx interface {
	LockSharedBot(ctx context.Context) (domain.SharedZaloBot, []byte, bool, error)
	SaveToken(ctx context.Context, in platformstore.SavedToken) (time.Time, error)
	RecordCheck(ctx context.Context, checkedSealed []byte, result string) (time.Time, error)
	PendingWebhookSecret(ctx context.Context) ([]byte, error)
	SetPendingWebhookSecret(ctx context.Context, sealed []byte) (time.Time, error)
	PromotePendingWebhookSecret(ctx context.Context, pendingSealed []byte, by string) (time.Time, error)
	ClearPendingWebhookSecret(ctx context.Context, pendingSealed []byte) error
	CountLiveLinks(ctx context.Context) (int, error)
	EndAllLiveLinks(ctx context.Context, op platformstore.Operator) (map[string]int, int, error)
	CommuneStats(ctx context.Context) ([]domain.ZaloBotCommuneStat, error)
	AppendOperatorAudit(ctx context.Context, e platformstore.OperatorAudit) error
}

// NewZaloBotStore adapts *platformstore.Store to ZaloBotStore.
func NewZaloBotStore(s *platformstore.Store) ZaloBotStore { return platformStoreAdapter{s} }

type platformStoreAdapter struct{ *platformstore.Store }

func (a platformStoreAdapter) InTx(ctx context.Context, fn func(ZaloBotTx) error) error {
	return a.Store.InTx(ctx, func(t *platformstore.Tx) error { return fn(t) })
}

// ErrZaloBotWebhookHostNotConfigured — ZALO_BOT_WEBHOOK_HOST is empty (dev only; staging and prod refuse
// to start without it). Nothing written, Zalo not called.
var ErrZaloBotWebhookHostNotConfigured = errors.New("zalo bot: ZALO_BOT_WEBHOOK_HOST is not set — the webhook has no host to point at")

// The additional data binding each sealed value to its row and field. The webhook secret moves from the
// pending column to the current one, so its binding names the SECRET, not a column — and differs from
// the token's, so the two can never be swapped.
var (
	tokenAAD         = []byte("zalo_bot_shared/token_sealed/" + domain.SharedZaloBotRef)
	webhookSecretAAD = []byte("zalo_bot_shared/webhook_secret/" + domain.SharedZaloBotRef)
)

// webhookSecretBytes is the CSPRNG output behind one secret_token (rule 13 #2): 32 bytes, sent as 43
// characters of unpadded base64url — inside Zalo's 8-256 and its likely [A-Za-z0-9_-] alphabet.
const webhookSecretBytes = 32

// SetResult is what SetSharedZaloBot decided (the contract's SetSharedZaloBotResult).
type SetResult int

const (
	SetSaved SetResult = iota + 1
	SetTokenCheckFailed
	SetRelinkConfirmationRequired
)

// SetSharedZaloBotInput is one SetSharedZaloBot request. Token is a secret.Secret: it never renders.
type SetSharedZaloBotInput struct {
	Token               secret.Secret
	BotName             string
	ChatURL             string
	ActorCode           string
	ActorIP             string
	ExpectedRelinkCount int
}

// SetSharedZaloBotOutput mirrors SetSharedZaloBotResponse.
type SetSharedZaloBotOutput struct {
	Result         SetResult
	Bot            domain.SharedZaloBot // set exactly when SetSaved
	ZaloOutcome    string               // domain.ZaloCall*
	LiveLinkCount  int                  // set when SetRelinkConfirmationRequired
	EndedLinkCount int                  // set when SetSaved
}

// CheckOutput mirrors CheckSharedZaloBotResponse.
type CheckOutput struct {
	Outcome     string    // domain.ZaloCall*
	CheckedAt   time.Time // zero when nothing was recorded (NOT_CONFIGURED)
	AccountName string    // set only on OK
}

// SetWebhookOutput mirrors SetSharedZaloBotWebhookResponse.
type SetWebhookOutput struct {
	Outcome string
	URL     string    // set whenever Zalo was called
	SetAt   time.Time // set only on OK
	SetBy   string
}

// GetWebhookOutput mirrors GetSharedZaloBotWebhookResponse.
type GetWebhookOutput struct {
	Outcome       string
	URL           string
	URLMatches    bool
	SecretSetAt   time.Time
	SecretSetBy   string
	ZaloUpdatedAt time.Time
}

// ZaloBotOperator owns the six operator acts.
type ZaloBotOperator struct {
	store       ZaloBotStore
	envelope    *crypto.PlatformEnvelope // nil without SECRET_ENCRYPTION_KEYS: FAILED_PRECONDITION
	bot         ZaloBotAPI
	webhookHost string // ZALO_BOT_WEBHOOK_HOST; "" in dev only
}

// NewZaloBotOperator panics on a missing store or adapter — at construction, not on the first call.
func NewZaloBotOperator(store ZaloBotStore, envelope *crypto.PlatformEnvelope, bot ZaloBotAPI, webhookHost string) *ZaloBotOperator {
	if store == nil || bot == nil {
		panic("app: NewZaloBotOperator thiếu phụ thuộc")
	}
	return &ZaloBotOperator{store: store, envelope: envelope, bot: bot, webhookHost: webhookHost}
}

// operator validates the actor of an act. No fallback: an empty or malformed code is refused.
func operator(code, ip string) (platformstore.Operator, error) {
	if !domain.ValidOperatorCode(code) {
		return platformstore.Operator{}, domain.ErrZaloBotActorCode
	}
	nip, err := domain.NormalizeActorIP(ip)
	if err != nil {
		return platformstore.Operator{}, err
	}
	return platformstore.Operator{Code: code, IP: nip}, nil
}

// errRelink carries the actual live count out of a rolled-back transaction.
type errRelink struct{ live int }

func (e errRelink) Error() string {
	return fmt.Sprintf("zalo bot: %d live links need confirming", e.live)
}

// SetSharedZaloBot — see the RPC. Every field is checked before anything is read or sent.
func (uc *ZaloBotOperator) SetSharedZaloBot(ctx context.Context, in SetSharedZaloBotInput) (SetSharedZaloBotOutput, error) {
	op, err := operator(in.ActorCode, in.ActorIP)
	if err != nil {
		return SetSharedZaloBotOutput{}, err
	}
	if !zalobot.ValidToken(in.Token.Lo()) {
		return SetSharedZaloBotOutput{}, domain.ErrZaloBotToken
	}
	name, err := domain.NormalizeZaloBotName(in.BotName)
	if err != nil {
		return SetSharedZaloBotOutput{}, err
	}
	if err := domain.ValidateZaloChatURL(in.ChatURL); err != nil {
		return SetSharedZaloBotOutput{}, err
	}
	if in.ExpectedRelinkCount < 0 {
		return SetSharedZaloBotOutput{}, domain.ErrZaloBotRelinkCount
	}
	if uc.envelope == nil {
		return SetSharedZaloBotOutput{}, crypto.ErrNotConfigured
	}

	// 1. Zalo getMe with the NEW token, outside any transaction. A token Zalo refuses is never saved.
	info, o := uc.bot.GetMe(ctx, in.Token)
	if o != zalobot.OutcomeOK {
		return SetSharedZaloBotOutput{Result: SetTokenCheckFailed, ZaloOutcome: outcomeValue(o)}, nil
	}

	// Sealed before the transaction; the operator rides in the context for the platform DEK's trail
	// should this be the first platform seal of the deployment.
	sealed, err := uc.envelope.SealPlatform(platformstore.WithOperator(ctx, op), in.Token, tokenAAD)
	if err != nil {
		return SetSharedZaloBotOutput{}, fmt.Errorf("zalo bot: seal the token: %w", err)
	}

	var out SetSharedZaloBotOutput
	err = uc.store.InTx(ctx, func(tx ZaloBotTx) error {
		cur, _, found, err := tx.LockSharedBot(ctx)
		if err != nil {
			return err
		}
		// "Unknown" is "different" (fail closed): with nothing stored, any live link is treated as a
		// link of another bot.
		accountChanged := !found || cur.BotAccountID != info.AccountID
		ended, endedTotal := map[string]int{}, 0
		if accountChanged {
			live, err := tx.CountLiveLinks(ctx)
			if err != nil {
				return err
			}
			if live != in.ExpectedRelinkCount {
				return errRelink{live: live}
			}
			if live > 0 {
				if ended, endedTotal, err = tx.EndAllLiveLinks(ctx, op); err != nil {
					return err
				}
				// A link made after the count but before the update would be ended without having been
				// confirmed. Never end a number the operator did not see.
				if endedTotal != in.ExpectedRelinkCount {
					return errRelink{live: endedTotal}
				}
			}
		}
		setAt, err := tx.SaveToken(ctx, platformstore.SavedToken{
			BotAccountID: info.AccountID, BotName: name, ChatURL: in.ChatURL, SetBy: op.Code, Sealed: sealed,
		})
		if err != nil {
			return err
		}
		var before any
		if found {
			before = map[string]any{
				"bot_account_id": cur.BotAccountID, "bot_name": cur.BotName, "chat_url": cur.ChatURL,
				"set_at": cur.SetAt, "set_by": cur.SetBy,
			}
		}
		after := map[string]any{
			"bot_account_id": info.AccountID, "bot_name": name, "chat_url": in.ChatURL,
			"set_at": setAt, "set_by": op.Code,
			// Facts about the act, never about the value (proto: "token replaced").
			"token_replaced":   found,
			"account_changed":  found && accountChanged,
			"ended_link_count": endedTotal,
		}
		if endedTotal > 0 {
			after["ended_links_by_commune"] = ended
		}
		if err := tx.AppendOperatorAudit(ctx, platformstore.OperatorAudit{
			Operator: op, Action: domain.ActionZaloBotTokenSet, Target: domain.ZaloBotTarget, Before: before, After: after,
		}); err != nil {
			return err
		}
		out = SetSharedZaloBotOutput{
			Result: SetSaved, ZaloOutcome: domain.ZaloCallOK, EndedLinkCount: endedTotal,
			Bot: domain.SharedZaloBot{
				BotAccountID: info.AccountID, BotName: name, ChatURL: in.ChatURL, SetAt: setAt, SetBy: op.Code,
				LastCheckAt: setAt, LastCheckResult: domain.ZaloCallOK,
			},
		}
		return nil
	})
	var relink errRelink
	if errors.As(err, &relink) {
		return SetSharedZaloBotOutput{
			Result: SetRelinkConfirmationRequired, ZaloOutcome: domain.ZaloCallOK, LiveLinkCount: relink.live,
		}, nil
	}
	if err != nil {
		return SetSharedZaloBotOutput{}, fmt.Errorf("zalo bot: save the token: %w", err)
	}
	return out, nil
}

// GetSharedZaloBot — a pure read of comms' own row. found=false: never configured.
func (uc *ZaloBotOperator) GetSharedZaloBot(ctx context.Context) (domain.SharedZaloBot, bool, error) {
	b, found, err := uc.store.SharedBot(ctx)
	if err != nil {
		return domain.SharedZaloBot{}, false, fmt.Errorf("zalo bot: read: %w", err)
	}
	return b, found, nil
}

// CheckSharedZaloBot — getMe with the live token, then (when there was one) the result recorded with
// its trail entry in one transaction.
func (uc *ZaloBotOperator) CheckSharedZaloBot(ctx context.Context, actorCode, actorIP string) (CheckOutput, error) {
	op, err := operator(actorCode, actorIP)
	if err != nil {
		return CheckOutput{}, err
	}
	token, sealed, ok, err := uc.liveToken(ctx)
	if err != nil {
		return CheckOutput{}, err
	}
	if !ok {
		return CheckOutput{Outcome: domain.ZaloCallNotConfigured}, nil
	}
	defer clear(token)

	info, o := uc.bot.GetMe(ctx, token)
	result := outcomeValue(o)
	var at time.Time
	err = uc.store.InTx(ctx, func(tx ZaloBotTx) error {
		if at, err = tx.RecordCheck(ctx, sealed, result); err != nil {
			return err
		}
		return tx.AppendOperatorAudit(ctx, platformstore.OperatorAudit{
			Operator: op, Action: domain.ActionZaloBotChecked, Target: domain.ZaloBotTarget,
			After: map[string]any{"result": result, "checked_at": at},
		})
	})
	if err != nil {
		// Zalo answered, the record failed: the check did not happen as far as the screen knows.
		return CheckOutput{}, fmt.Errorf("zalo bot: record the check: %w", err)
	}
	out := CheckOutput{Outcome: result, CheckedAt: at}
	if o == zalobot.OutcomeOK {
		out.AccountName = info.AccountName
	}
	return out, nil
}

// SetSharedZaloBotWebhook — pending-then-promote (the RPC's numbered steps).
func (uc *ZaloBotOperator) SetSharedZaloBotWebhook(ctx context.Context, actorCode, actorIP string) (SetWebhookOutput, error) {
	op, err := operator(actorCode, actorIP)
	if err != nil {
		return SetWebhookOutput{}, err
	}
	token, _, ok, err := uc.liveToken(ctx)
	if err != nil {
		return SetWebhookOutput{}, err
	}
	if !ok {
		return SetWebhookOutput{Outcome: domain.ZaloCallNotConfigured}, nil
	}
	defer clear(token)
	webhookURL := domain.ZaloWebhookURL(uc.webhookHost)
	if webhookURL == "" {
		return SetWebhookOutput{}, ErrZaloBotWebhookHostNotConfigured
	}

	// ONE PENDING SLOT (0018). A pending secret still in the slot is one Zalo MAY already be using — the
	// last attempt's answer was ambiguous. Replacing it could 403 every staff update, the one state the
	// transaction boundary forbids ("Không bao giờ có trạng thái Zalo gửi một secret mà comms không
	// nhận"). So a pending secret is REUSED, and only an empty slot gets fresh CSPRNG output. Both are
	// prepared outside the transaction; the transaction only checks the slot did not move meanwhile.
	seenPending, err := uc.store.PendingWebhookSecretRead(ctx)
	if err != nil {
		return SetWebhookOutput{}, fmt.Errorf("zalo bot: read the pending webhook secret: %w", err)
	}
	var plain secret.Secret
	var pendingSealed []byte
	reused := seenPending != nil
	if reused {
		if plain, err = uc.envelope.OpenPlatform(ctx, seenPending, webhookSecretAAD); err != nil {
			return SetWebhookOutput{}, fmt.Errorf("zalo bot: open the pending webhook secret: %w", err)
		}
		pendingSealed = seenPending
	} else {
		if plain, err = newWebhookSecret(); err != nil {
			return SetWebhookOutput{}, err
		}
		if pendingSealed, err = uc.envelope.SealPlatform(platformstore.WithOperator(ctx, op), plain, webhookSecretAAD); err != nil {
			clear(plain)
			return SetWebhookOutput{}, fmt.Errorf("zalo bot: seal the webhook secret: %w", err)
		}
	}
	defer clear(plain)

	// Step 1: the pending secret is stored (or confirmed) with its entry, BEFORE Zalo hears of it.
	err = uc.store.InTx(ctx, func(tx ZaloBotTx) error {
		if _, _, found, err := tx.LockSharedBot(ctx); err != nil {
			return err
		} else if !found {
			return platformstore.ErrNotConfigured
		}
		cur, err := tx.PendingWebhookSecret(ctx)
		if err != nil {
			return err
		}
		switch {
		case reused && !bytes.Equal(cur, seenPending):
			return platformstore.ErrChanged
		case !reused && cur != nil:
			return platformstore.ErrChanged
		case !reused:
			if _, err := tx.SetPendingWebhookSecret(ctx, pendingSealed); err != nil {
				return err
			}
		}
		return tx.AppendOperatorAudit(ctx, platformstore.OperatorAudit{
			Operator: op, Action: domain.ActionZaloBotWebhookRequested, Target: domain.ZaloBotTarget,
			After: map[string]any{"url": webhookURL, "pending_secret_reused": reused},
		})
	})
	if err != nil {
		return SetWebhookOutput{}, fmt.Errorf("zalo bot: store the pending webhook secret: %w", err)
	}

	// Step 2: Zalo, outside any transaction.
	o := uc.bot.SetWebhook(ctx, token, webhookURL, plain)
	result := outcomeValue(o)
	out := SetWebhookOutput{Outcome: result, URL: webhookURL}

	// Step 3: one transaction by the outcome's kind.
	err = uc.store.InTx(ctx, func(tx ZaloBotTx) error {
		if _, _, _, err := tx.LockSharedBot(ctx); err != nil {
			return err
		}
		switch o {
		case zalobot.OutcomeOK:
			at, err := tx.PromotePendingWebhookSecret(ctx, pendingSealed, op.Code)
			if err != nil {
				return err
			}
			out.SetAt, out.SetBy = at, op.Code
			return tx.AppendOperatorAudit(ctx, platformstore.OperatorAudit{
				Operator: op, Action: domain.ActionZaloBotWebhookSet, Target: domain.ZaloBotTarget,
				After: map[string]any{"url": webhookURL, "set_at": at, "set_by": op.Code},
			})
		case zalobot.OutcomeTokenRejected, zalobot.OutcomeRejected, zalobot.OutcomeRateLimited:
			// A definite refusal: Zalo never used this secret. A slot that moved meanwhile is left alone.
			if err := tx.ClearPendingWebhookSecret(ctx, pendingSealed); err != nil && !errors.Is(err, platformstore.ErrChanged) {
				return err
			}
			return tx.AppendOperatorAudit(ctx, platformstore.OperatorAudit{
				Operator: op, Action: domain.ActionZaloBotWebhookRefused, Target: domain.ZaloBotTarget,
				After: map[string]any{"url": webhookURL, "result": result},
			})
		default:
			// Ambiguous (Unavailable, MalformedResponse): Zalo may have applied it; the pending secret
			// stays accepted until the next OK retires it.
			return tx.AppendOperatorAudit(ctx, platformstore.OperatorAudit{
				Operator: op, Action: domain.ActionZaloBotWebhookUnconfirmed, Target: domain.ZaloBotTarget,
				After: map[string]any{"url": webhookURL, "result": result},
			})
		}
	})
	if err != nil {
		return SetWebhookOutput{}, fmt.Errorf("zalo bot: record the webhook outcome: %w", err)
	}
	return out, nil
}

// GetSharedZaloBotWebhook — getWebhookInfo, compared with the URL comms would set. Nothing written.
func (uc *ZaloBotOperator) GetSharedZaloBotWebhook(ctx context.Context) (GetWebhookOutput, error) {
	token, _, ok, err := uc.liveToken(ctx)
	if err != nil {
		return GetWebhookOutput{}, err
	}
	if !ok {
		return GetWebhookOutput{Outcome: domain.ZaloCallNotConfigured}, nil
	}
	defer clear(token)
	info, o := uc.bot.GetWebhookInfo(ctx, token)
	if o != zalobot.OutcomeOK {
		return GetWebhookOutput{Outcome: outcomeValue(o)}, nil
	}
	b, _, err := uc.store.SharedBot(ctx)
	if err != nil {
		return GetWebhookOutput{}, fmt.Errorf("zalo bot: read: %w", err)
	}
	want := domain.ZaloWebhookURL(uc.webhookHost)
	return GetWebhookOutput{
		Outcome: domain.ZaloCallOK, URL: info.URL, URLMatches: want != "" && info.URL == want,
		SecretSetAt: b.WebhookSetAt, SecretSetBy: b.WebhookSetBy, ZaloUpdatedAt: info.UpdatedAt,
	}, nil
}

// ListZaloBotCommuneStats — the cross-commune read, audited in the same transaction (rule 6 inv 7).
func (uc *ZaloBotOperator) ListZaloBotCommuneStats(ctx context.Context, actorCode, actorIP string) ([]domain.ZaloBotCommuneStat, error) {
	op, err := operator(actorCode, actorIP)
	if err != nil {
		return nil, err
	}
	var out []domain.ZaloBotCommuneStat
	err = uc.store.InTx(ctx, func(tx ZaloBotTx) error {
		if out, err = tx.CommuneStats(ctx); err != nil {
			return err
		}
		return tx.AppendOperatorAudit(ctx, platformstore.OperatorAudit{
			Operator: op, Action: domain.ActionZaloBotCommunesRead, Target: domain.ZaloBotTarget,
			After: map[string]any{"commune_count": len(out)},
		})
	})
	if err != nil {
		return nil, fmt.Errorf("zalo bot: commune stats: %w", err)
	}
	return out, nil
}

// liveToken opens the stored token. ok=false: none stored (NOT_CONFIGURED). A nil envelope with a
// stored token is crypto.ErrNotConfigured — nothing can be opened.
func (uc *ZaloBotOperator) liveToken(ctx context.Context) (secret.Secret, []byte, bool, error) {
	sealed, found, err := uc.store.SharedBotToken(ctx)
	if err != nil {
		return nil, nil, false, fmt.Errorf("zalo bot: read the token: %w", err)
	}
	if !found {
		return nil, nil, false, nil
	}
	if uc.envelope == nil {
		return nil, nil, false, crypto.ErrNotConfigured
	}
	token, err := uc.envelope.OpenPlatform(ctx, sealed, tokenAAD)
	if err != nil {
		return nil, nil, false, fmt.Errorf("zalo bot: open the token: %w", err)
	}
	return token, sealed, true, nil
}

// newWebhookSecret is fresh CSPRNG output (rule 13 #2), base64url without padding.
func newWebhookSecret() (secret.Secret, error) {
	raw := make([]byte, webhookSecretBytes)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("zalo bot: generate the webhook secret: %w", err)
	}
	defer clear(raw)
	out := make([]byte, base64.RawURLEncoding.EncodedLen(len(raw)))
	base64.RawURLEncoding.Encode(out, raw)
	return secret.Secret(out), nil
}

// outcomeValue maps the adapter's class onto the stored vocabulary (ADR 0011).
func outcomeValue(o zalobot.Outcome) string {
	switch o {
	case zalobot.OutcomeOK:
		return domain.ZaloCallOK
	case zalobot.OutcomeTokenRejected:
		return domain.ZaloCallTokenRejected
	case zalobot.OutcomeRateLimited:
		return domain.ZaloCallRateLimited
	case zalobot.OutcomeUnavailable:
		return domain.ZaloCallUnavailable
	case zalobot.OutcomeRejected:
		return domain.ZaloCallRejected
	}
	return domain.ZaloCallMalformedResponse
}
