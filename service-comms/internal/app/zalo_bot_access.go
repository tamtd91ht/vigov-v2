package app

// THE COMMUNE SIDE'S VIEW OF THE ONE SHARED ZALO BOT (ADR 0074): its display metadata (bot_name, chat_url
// — what a member of staff is shown to pair), its token (opened only to send), and the webhook's
// authentication against BOTH stored secret_tokens.
//
// Read-only on the platform tables: every write on them is an operator act (zalo_bot_operator.go). The
// token and the secrets never leave this file except as a secret.Secret handed straight to the adapter.

import (
	"context"
	"crypto/subtle"
	"fmt"

	"github.com/vihat/vigov/core/crypto"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/service-comms/internal/domain"
	"github.com/vihat/vigov/service-comms/internal/zalobot"
)

// ZaloWebhookSecrets reads both sealed webhook secrets. *platformstore.Store satisfies it.
type ZaloWebhookSecrets interface {
	WebhookSecretsSealed(ctx context.Context) (current, pending []byte, err error)
}

// ZaloMessenger is the adapter's send, declared at the point of use. *zalobot.Client satisfies it.
type ZaloMessenger interface {
	SendMessage(ctx context.Context, token secret.Secret, chatID, text string) zalobot.Outcome
}

// SharedZaloBot is what the staff routes, the webhook and the dispatcher need of the shared bot.
// *SharedZaloBotAccess satisfies it; tests fake it.
type SharedZaloBot interface {
	// Bot is the row's metadata; found=false: never configured.
	Bot(ctx context.Context) (domain.SharedZaloBot, bool, error)
	// Token opens the token; configured=false: no token stored. The caller clears it after use.
	Token(ctx context.Context) (token secret.Secret, configured bool, err error)
	// WebhookSecretMatches compares presented, in constant time, against the secret in force AND the
	// pending one (0018 PENDING-THEN-PROMOTE).
	WebhookSecretMatches(ctx context.Context, presented []byte) (bool, error)
}

// SharedZaloBotAccess is the production SharedZaloBot.
type SharedZaloBotAccess struct {
	store    ZaloBotStore
	secrets  ZaloWebhookSecrets
	envelope *crypto.PlatformEnvelope // nil without SECRET_ENCRYPTION_KEYS: no token, no secret opens
}

// NewSharedZaloBotAccess panics on a missing store — at construction, not on the first update.
func NewSharedZaloBotAccess(store ZaloBotStore, secrets ZaloWebhookSecrets, envelope *crypto.PlatformEnvelope) *SharedZaloBotAccess {
	if store == nil || secrets == nil {
		panic("app: NewSharedZaloBotAccess thiếu phụ thuộc")
	}
	return &SharedZaloBotAccess{store: store, secrets: secrets, envelope: envelope}
}

func (a *SharedZaloBotAccess) Bot(ctx context.Context) (domain.SharedZaloBot, bool, error) {
	b, found, err := a.store.SharedBot(ctx)
	if err != nil {
		return domain.SharedZaloBot{}, false, fmt.Errorf("zalo bot: read: %w", err)
	}
	return b, found, nil
}

func (a *SharedZaloBotAccess) Token(ctx context.Context) (secret.Secret, bool, error) {
	token, _, ok, err := openLiveToken(ctx, a.store, a.envelope)
	return token, ok, err
}

// WebhookSecretMatches — FAIL CLOSED everywhere: no row, no secret, no envelope or an unopenable seal all
// answer "no match" or an error, never "match". Both slots are always compared, so the time taken does
// not say which slot matched.
func (a *SharedZaloBotAccess) WebhookSecretMatches(ctx context.Context, presented []byte) (bool, error) {
	if len(presented) == 0 {
		return false, nil
	}
	current, pending, err := a.secrets.WebhookSecretsSealed(ctx)
	if err != nil {
		return false, err
	}
	if current == nil && pending == nil {
		return false, nil
	}
	if a.envelope == nil {
		return false, crypto.ErrNotConfigured
	}
	match := 0
	for _, sealed := range [][]byte{current, pending} {
		if sealed == nil {
			continue
		}
		plain, err := a.envelope.OpenPlatform(ctx, sealed, webhookSecretAAD)
		if err != nil {
			return false, fmt.Errorf("zalo bot: open a webhook secret: %w", err)
		}
		match |= subtle.ConstantTimeCompare(plain.Lo(), presented)
		clear(plain)
	}
	return match == 1, nil
}
