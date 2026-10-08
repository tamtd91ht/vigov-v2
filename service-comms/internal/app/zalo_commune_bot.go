package app

// A COMMUNE'S OWN ZALO BOT (migration 0022; ADR 0079 #4 and §Q1 #1–#5) — the administration of it
// (CommuneZaloBots, behind /api/v1/zalo-bots/current, admin.lookup) and the one answer every sender needs:
// WHICH bot serves this commune now (ZaloBotRouter).
//
// THE COMMUNE IS THE CONTEXT'S, always (rule 1, invariant 4): the routes resolve it from Host; nothing
// here takes a commune as an argument, and nothing reads another commune's bot.
//
// THE TOKEN AND THE WEBHOOK SECRET ARE WRITE-ONLY. Sealed under the COMMUNE's data key (ADR 0079 Q1 #1 —
// the mail password's discipline, mail_settings.go), with additional data naming the commune, the ROW and
// the SECRET (0022 "OWED BY GO"). No return value carries the token; the webhook secret is returned ONCE,
// by the call that generated it (ADR 0079 Q1 #3). Zalo failures are CLASSES, never text — the token sits
// in the URL of every Zalo call (internal/zalobot), so a Zalo URL is never logged.
//
// THE ORDER OF EVERY ACT is the shared bot operator's (zalo_bot_operator.go): validate the request alone →
// the Zalo call OUTSIDE any transaction → seal OUTSIDE the transaction (a first seal may create the
// commune's DEK in its own short transaction) → ONE transaction with the change and its audit entry.

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/crypto"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-comms/internal/domain"
	docstore "github.com/vihat/vigov/service-comms/internal/store"
	"github.com/vihat/vigov/service-comms/internal/zalobot"
)

// communeBotTokenAAD / communeBotWebhookAAD — 0022's bindings, verbatim. Bound to the ROW, so bytes copied
// from a retired row onto the live one do not open; to the commune, so they do not open in another; and
// to the SECRET rather than the column, because a pending webhook secret is promoted by copying its bytes.
func communeBotTokenAAD(ctx context.Context, rowID string) []byte {
	return []byte("zalo_commune_bot/token/" + string(tenant.MustFrom(ctx)) + "/" + rowID)
}

func communeBotWebhookAAD(ctx context.Context, rowID string) []byte {
	return []byte("zalo_commune_bot/webhook_secret/" + string(tenant.MustFrom(ctx)) + "/" + rowID)
}

// ---- which bot serves the commune ------------------------------------------------------------------

// ActiveZaloBot is the bot that serves the commune in the context: its own live bot, else the shared one.
type ActiveZaloBot struct {
	// Ref is the bot_ref its links carry (domain.SharedZaloBotRef, or the own bot's Ref()).
	Ref string
	// Own: the commune's own bot (0022); false = the shared bot.
	Own bool
	// Configured: a bot with a token exists — the own bot whenever there is one; the shared bot when the
	// operator has set it (ADR 0074).
	Configured bool
	BotName    string
	ChatURL    string
}

// ZaloBotSource answers which bot serves the commune in ctx, and opens its token. *ZaloBotRouter
// satisfies it; tests fake it.
type ZaloBotSource interface {
	Active(ctx context.Context) (ActiveZaloBot, error)
	// ActiveToken is Active plus the opened token (nil when !Configured). The caller clears it.
	ActiveToken(ctx context.Context) (ActiveZaloBot, secret.Secret, error)
}

// ZaloBotRouter is the production ZaloBotSource.
type ZaloBotRouter struct {
	shared   SharedZaloBot
	own      *docstore.ZaloCommuneBotStore
	envelope *crypto.Envelope // the COMMUNE envelope; nil without SECRET_ENCRYPTION_KEYS
}

// NewZaloBotRouter panics on a missing dependency — at construction, not on the first send.
func NewZaloBotRouter(shared SharedZaloBot, own *docstore.ZaloCommuneBotStore, envelope *crypto.Envelope) *ZaloBotRouter {
	if shared == nil || own == nil {
		panic("app: NewZaloBotRouter thiếu phụ thuộc")
	}
	return &ZaloBotRouter{shared: shared, own: own, envelope: envelope}
}

// Active — the commune's live own bot when there is one (every message goes through it, ADR 0079 Q1 #4),
// else the shared bot.
func (r *ZaloBotRouter) Active(ctx context.Context) (ActiveZaloBot, error) {
	b, found, err := r.own.Live(ctx)
	if err != nil {
		return ActiveZaloBot{}, fmt.Errorf("zalo: read the commune's bot: %w", err)
	}
	if found {
		return ActiveZaloBot{Ref: b.Ref(), Own: true, Configured: true, BotName: b.BotName, ChatURL: b.ChatURL}, nil
	}
	s, found, err := r.shared.Bot(ctx)
	if err != nil {
		return ActiveZaloBot{}, err
	}
	out := ActiveZaloBot{Ref: domain.SharedZaloBotRef, Configured: found}
	if found {
		out.BotName, out.ChatURL = s.BotName, s.ChatURL
	}
	return out, nil
}

// ActiveToken — FAIL CLOSED: an own bot whose token cannot be opened (no KEK, a broken seal) is an error,
// never a silent fall back to the shared bot, which the commune has switched away from.
func (r *ZaloBotRouter) ActiveToken(ctx context.Context) (ActiveZaloBot, secret.Secret, error) {
	s, found, err := r.own.Sealed(ctx)
	if err != nil {
		return ActiveZaloBot{}, nil, fmt.Errorf("zalo: read the commune's bot: %w", err)
	}
	if found {
		if r.envelope == nil {
			return ActiveZaloBot{}, nil, crypto.ErrNotConfigured
		}
		token, err := r.envelope.Open(ctx, s.TokenSealed, communeBotTokenAAD(ctx, s.Bot.ID))
		if err != nil {
			return ActiveZaloBot{}, nil, fmt.Errorf("zalo: open the commune bot's token: %w", err)
		}
		return ActiveZaloBot{Ref: s.Bot.Ref(), Own: true, Configured: true, BotName: s.Bot.BotName,
			ChatURL: s.Bot.ChatURL}, token, nil
	}
	token, configured, err := r.shared.Token(ctx)
	if err != nil {
		return ActiveZaloBot{}, nil, err
	}
	return ActiveZaloBot{Ref: domain.SharedZaloBotRef, Configured: configured}, token, nil
}

// ---- the administration ------------------------------------------------------------------------------

var (
	// ErrCommuneZaloBotMissing — the commune has no live own bot (it uses the shared one).
	ErrCommuneZaloBotMissing = errors.New("zalo: the commune has no own bot")
	// ErrCommuneZaloBotChanged — the live bot changed between the read and the write (a concurrent save).
	ErrCommuneZaloBotChanged = errors.New("zalo: the commune's bot changed meanwhile")
	// ErrCommuneHostUnknown — the platform registry gave no active host for the commune: the webhook URL
	// cannot be built. Fail closed — never a guessed host.
	ErrCommuneHostUnknown = errors.New("zalo: the commune's host is not known")
)

// ZaloTokenCheckError — Zalo did not accept the token (getMe). Class only. Nothing was written.
type ZaloTokenCheckError struct{ Class string }

func (e *ZaloTokenCheckError) Error() string { return "zalo: token check failed: " + e.Class }

// CommuneZaloBotView is GET zalo-bots/current.
type CommuneZaloBotView struct {
	HasOwnBot bool
	Bot       domain.CommuneZaloBot // zero when !HasOwnBot
	// LiveLinkCount is how many staff are paired through the bot that serves the commune now — what a
	// switch of bot would end (ADR 0079 Q1 #4: "hộp xác nhận nêu số người bị ảnh hưởng").
	LiveLinkCount int
}

// SetCommuneZaloBotInput is one PUT. Token empty = keep the live bot's token (then a live bot is required).
type SetCommuneZaloBotInput struct {
	Token   secret.Secret
	BotName string
	ChatURL string
}

// SetCommuneZaloBotOutput — the bot now live, and what the save ended.
type SetCommuneZaloBotOutput struct {
	Bot            domain.CommuneZaloBot
	Adopted        bool // a NEW live row: shared → own, or a different own bot
	EndedLinkCount int
	// RetiredPrevious: a different own bot was retired — the reply tells the commune to revoke its token.
	RetiredPrevious bool
}

// CommuneWebhookOutput mirrors the shared operator's SetWebhookOutput, plus the secret shown ONCE.
type CommuneWebhookOutput struct {
	Outcome string
	URL     string
	SetAt   time.Time
	SetBy   string
	// Secret is set ONLY by the call that generated it, and only when the webhook now accepts it (Zalo
	// confirmed, or answered ambiguously and it stays pending). The caller clears it after replying.
	Secret secret.Secret
}

// RetireCommuneZaloBotOutput is DELETE zalo-bots/current.
type RetireCommuneZaloBotOutput struct {
	Retired        bool
	EndedLinkCount int
}

// CommuneZaloBots owns the five acts on the commune's own bot.
type CommuneZaloBots struct {
	db       *store.DB
	bots     *docstore.ZaloCommuneBotStore
	links    *docstore.ZaloLinkStore
	envelope *crypto.Envelope
	api      ZaloBotAPI
	registry PortalCommuneRegistry // the commune's host, for the webhook URL
	log      *slog.Logger          // the SECURITY LOG (skills/security-logging) — never the audit trail

	newID     func() (string, error)
	now       func() time.Time
	newSecret func() (secret.Secret, error)
}

// NewCommuneZaloBots panics on a missing dependency.
func NewCommuneZaloBots(db *store.DB, bots *docstore.ZaloCommuneBotStore, links *docstore.ZaloLinkStore,
	envelope *crypto.Envelope, api ZaloBotAPI, registry PortalCommuneRegistry, log *slog.Logger) *CommuneZaloBots {

	if db == nil || bots == nil || links == nil || api == nil || registry == nil {
		panic("app: NewCommuneZaloBots thiếu phụ thuộc")
	}
	if log == nil {
		log = slog.Default()
	}
	return &CommuneZaloBots{db: db, bots: bots, links: links, envelope: envelope, api: api, registry: registry,
		log: log, newID: ulid.Moi, now: time.Now, newSecret: newWebhookSecret}
}

func (uc *CommuneZaloBots) clock() time.Time { return uc.now().UTC().Truncate(time.Microsecond) }

// The security-log events of the commune bot (TCVN 14423 §6.8.2.1, "configuration changed"). The audit
// trail already holds who did what; these are the operational stream an incident responder watches —
// a commune's bot is where its staff's chats go.
const (
	SecEventCommuneBotSet     = "zalo_commune_bot.set"
	SecEventCommuneBotWebhook = "zalo_commune_bot.webhook"
	SecEventCommuneBotRetired = "zalo_commune_bot.retired"
	SecEventCommuneBotChecked = "zalo_commune_bot.checked"
)

// securityEvent writes one security-log line: event, outcome, commune, the actor's BUSINESS CODE, the
// client IP, and the bot's Zalo account id (not personal data). NEVER a token, a secret or a Zalo URL —
// the token is in the path of every Zalo URL (rule 3; rule 8).
func (uc *CommuneZaloBots) securityEvent(ctx context.Context, event, outcome string, actor audit.Actor,
	botAccountID string) {

	uc.log.InfoContext(ctx, "NHẬT KÝ AN NINH: thay đổi cấu hình bot Zalo riêng của xã",
		"event", event, "outcome", outcome, "xa", string(tenant.MustFrom(ctx)), "actor", actor.ID, "ip", actor.IP,
		"bot_account_id", botAccountID, "at", uc.clock())
}

// Current reads the commune's own bot — no sealed column, no secret. LiveLinkCount counts the links of
// the bot that serves the commune NOW (its own, else the shared one) — a scoped COUNT, so a commune past
// the linked-staff list's ceiling still gets its number.
func (uc *CommuneZaloBots) Current(ctx context.Context) (CommuneZaloBotView, error) {
	b, found, err := uc.bots.Live(ctx)
	if err != nil {
		return CommuneZaloBotView{}, err
	}
	ref := domain.SharedZaloBotRef
	if found {
		ref = b.Ref()
	}
	n, err := uc.links.CountLiveLinksOfBot(ctx, ref)
	if err != nil {
		return CommuneZaloBotView{}, fmt.Errorf("zalo: count the links: %w", err)
	}
	return CommuneZaloBotView{HasOwnBot: found, Bot: b, LiveLinkCount: n}, nil
}

// Set saves the commune's own bot (spec Cấu hình 11 §3 "Lưu con bot").
//
//	no live bot, token given            ADOPT: a new live row; every live SHARED link of the commune ends
//	live bot, token of the SAME account a new token in place (new set_at), name and link updated
//	live bot, token of ANOTHER account  REPLACE: retire the live row, end its links, add the new one
//	live bot, no token                  name and link only
//	no live bot, no token               refused: nothing to save a name for
//
// bot_account_id is ALWAYS what Zalo's getMe answers for the given token — never from the request.
func (uc *CommuneZaloBots) Set(ctx context.Context, in SetCommuneZaloBotInput, actor audit.Actor) (
	SetCommuneZaloBotOutput, error) {

	if actor.ID == "" {
		return SetCommuneZaloBotOutput{}, ErrNoActor
	}
	name, err := domain.NormalizeCommuneZaloBotName(in.BotName)
	if err != nil {
		return SetCommuneZaloBotOutput{}, err
	}
	if err := domain.ValidateCommuneZaloChatURL(in.ChatURL); err != nil {
		return SetCommuneZaloBotOutput{}, err
	}
	hasToken := len(in.Token) > 0
	if hasToken && !zalobot.ValidToken(in.Token.Lo()) {
		return SetCommuneZaloBotOutput{}, domain.ErrCommuneZaloBotToken
	}
	if uc.envelope == nil {
		return SetCommuneZaloBotOutput{}, crypto.ErrNotConfigured
	}

	seen, seenFound, err := uc.bots.Live(ctx)
	if err != nil {
		return SetCommuneZaloBotOutput{}, err
	}
	if !hasToken {
		if !seenFound {
			return SetCommuneZaloBotOutput{}, domain.ErrCommuneZaloBotTokenRequired
		}
		return uc.updateMeta(ctx, seen, name, in.ChatURL, actor)
	}

	// 1. Zalo getMe with the GIVEN token, outside any transaction. A token Zalo refuses is never saved.
	info, o := uc.api.GetMe(ctx, in.Token)
	if o != zalobot.OutcomeOK {
		uc.securityEvent(ctx, SecEventCommuneBotSet, "token_refused:"+outcomeValue(o), actor, "")
		return SetCommuneZaloBotOutput{}, &ZaloTokenCheckError{Class: outcomeValue(o)}
	}
	if !domain.ValidCommuneZaloBotAccountID(info.AccountID) {
		return SetCommuneZaloBotOutput{}, &ZaloTokenCheckError{Class: domain.ZaloCallMalformedResponse}
	}

	// 2. Which row the token belongs to decides its additional data, so the id is chosen BEFORE sealing,
	// from what was seen; the transaction re-checks that nothing moved meanwhile.
	sameAccount := seenFound && seen.BotAccountID == info.AccountID
	rowID := seen.ID
	if !sameAccount {
		if rowID, err = uc.newID(); err != nil {
			return SetCommuneZaloBotOutput{}, fmt.Errorf("zalo: new id: %w", err)
		}
	}
	sealed, err := uc.envelope.Seal(ctx, in.Token, communeBotTokenAAD(ctx, rowID))
	if err != nil {
		return SetCommuneZaloBotOutput{}, fmt.Errorf("zalo: seal the bot token: %w", err)
	}

	at := uc.clock()
	var out SetCommuneZaloBotOutput
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		cur, found, err := uc.bots.LockLive(ctx, tx)
		if err != nil {
			return err
		}
		if found != seenFound || (found && cur.Bot.ID != seen.ID) {
			return ErrCommuneZaloBotChanged
		}
		if sameAccount {
			if err := uc.bots.UpdateToken(ctx, tx, rowID, sealed, name, in.ChatURL, actor.ID, at); err != nil {
				return err
			}
			after := cur.Bot
			after.BotName, after.ChatURL, after.SetAt, after.SetBy = name, in.ChatURL, at, actor.ID
			after.LastCheckAt, after.LastCheckResult = at, domain.ZaloCallOK
			out = SetCommuneZaloBotOutput{Bot: after}
			return writeCommuneBotAudit(ctx, tx, actor, domain.ActionSetCommuneZaloBot, after.Subject(), at,
				communeBotDelta(cur.Bot), communeBotDelta(after), map[string]any{"token_replaced": true})
		}

		// A switch of bot ends every live link of the bot that served until now (ADR 0079 Q1 #4) — the
		// shared bot's when adopting, the retired own bot's when replacing — each ended link audited.
		endedFrom := domain.SharedZaloBotRef
		if found {
			if err := uc.bots.Retire(ctx, tx, cur.Bot.ID, actor.ID, domain.CommuneZaloBotReplacedReason, at); err != nil {
				return err
			}
			endedFrom = cur.Bot.Ref()
		}
		ended, err := uc.endLinks(ctx, tx, endedFrom, actor, at)
		if err != nil {
			return err
		}
		after := domain.CommuneZaloBot{ID: rowID, BotAccountID: info.AccountID, BotName: name, ChatURL: in.ChatURL,
			SetAt: at, SetBy: actor.ID, LastCheckAt: at, LastCheckResult: domain.ZaloCallOK}
		if err := uc.bots.InsertLive(ctx, tx, after, sealed); err != nil {
			return err
		}
		out = SetCommuneZaloBotOutput{Bot: after, Adopted: true, EndedLinkCount: ended, RetiredPrevious: found}
		var before any
		if found {
			before = communeBotDelta(cur.Bot)
		}
		return writeCommuneBotAudit(ctx, tx, actor, domain.ActionSetCommuneZaloBot, after.Subject(), at,
			before, communeBotDelta(after), map[string]any{
				"adopted": true, "replaced_bot": found, "ended_link_count": ended, "ended_bot_ref": endedFrom,
			})
	})
	if err != nil {
		if errors.Is(err, docstore.ErrCommuneZaloBotAccountTaken) {
			// A commune trying a bot account live elsewhere — possibly the shared bot's: worth watching.
			uc.securityEvent(ctx, SecEventCommuneBotSet, "account_in_use", actor, info.AccountID)
		}
		return SetCommuneZaloBotOutput{}, fmt.Errorf("zalo: save the commune bot: %w", err)
	}
	outcome := "token_set"
	switch {
	case out.RetiredPrevious:
		outcome = "replaced"
	case out.Adopted:
		outcome = "adopted"
	}
	uc.securityEvent(ctx, SecEventCommuneBotSet, outcome, actor, out.Bot.BotAccountID)
	return out, nil
}

func (uc *CommuneZaloBots) updateMeta(ctx context.Context, seen domain.CommuneZaloBot, name, chatURL string,
	actor audit.Actor) (SetCommuneZaloBotOutput, error) {

	at := uc.clock()
	var out SetCommuneZaloBotOutput
	changed := false
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		cur, found, err := uc.bots.LockLive(ctx, tx)
		if err != nil {
			return err
		}
		if !found || cur.Bot.ID != seen.ID {
			return ErrCommuneZaloBotChanged
		}
		after := cur.Bot
		after.BotName, after.ChatURL = name, chatURL
		out = SetCommuneZaloBotOutput{Bot: after}
		if after.BotName == cur.Bot.BotName && after.ChatURL == cur.Bot.ChatURL {
			return nil // nothing moved: no UPDATE, no entry
		}
		if err := uc.bots.UpdateMeta(ctx, tx, cur.Bot.ID, name, chatURL, at); err != nil {
			return err
		}
		changed = true
		return writeCommuneBotAudit(ctx, tx, actor, domain.ActionSetCommuneZaloBot, after.Subject(), at,
			communeBotDelta(cur.Bot), communeBotDelta(after), map[string]any{"token_replaced": false})
	})
	if err != nil {
		return SetCommuneZaloBotOutput{}, fmt.Errorf("zalo: save the commune bot: %w", err)
	}
	if changed {
		uc.securityEvent(ctx, SecEventCommuneBotSet, "updated", actor, out.Bot.BotAccountID)
	}
	return out, nil
}

// endLinks ends every live link of botRef in this commune, one ket_thuc_lien_ket_zalo entry each.
func (uc *CommuneZaloBots) endLinks(ctx context.Context, tx *store.ScopedTx, botRef string, actor audit.Actor,
	at time.Time) (int, error) {

	ended, err := uc.links.EndLiveLinksOfBot(ctx, tx, botRef, actor.ID, domain.ZaloLinkEndedByBotChange, at)
	if err != nil {
		return 0, err
	}
	for _, l := range ended {
		if err := writeLinkEnded(ctx, tx, actor, l, domain.ZaloLinkEndedByBotChange, "doi_bot_zalo_cua_xa", at); err != nil {
			return 0, err
		}
	}
	return len(ended), nil
}

// Check — getMe with the live own bot's token, then the result recorded with its entry in one
// transaction (spec 11 §3 "Kiểm tra kết nối").
func (uc *CommuneZaloBots) Check(ctx context.Context, actor audit.Actor) (CheckOutput, error) {
	if actor.ID == "" {
		return CheckOutput{}, ErrNoActor
	}
	s, token, err := uc.openLive(ctx)
	if err != nil {
		return CheckOutput{}, err
	}
	defer clear(token)
	info, o := uc.api.GetMe(ctx, token)
	result := outcomeValue(o)
	at := uc.clock()
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		if err := uc.bots.RecordCheck(ctx, tx, s.Bot.ID, result, at); err != nil {
			return err
		}
		return writeCommuneBotAudit(ctx, tx, actor, domain.ActionCheckCommuneZaloBot, s.Bot.Subject(), at, nil,
			map[string]any{"result": result, "checked_at": at}, nil)
	})
	if err != nil {
		if errors.Is(err, docstore.ErrZaloRowChanged) {
			return CheckOutput{}, ErrCommuneZaloBotChanged
		}
		return CheckOutput{}, fmt.Errorf("zalo: record the check: %w", err)
	}
	uc.securityEvent(ctx, SecEventCommuneBotChecked, result, actor, s.Bot.BotAccountID)
	out := CheckOutput{Outcome: result, CheckedAt: at}
	if o == zalobot.OutcomeOK {
		out.AccountName = info.AccountName
	}
	return out, nil
}

// RegisterWebhook — pending-then-promote, the shared operator's three steps (zalo_bot_operator.go), on
// the commune's OWN host: `https://<commune host>/api/v1/zalo-bot-updates` (ADR 0079 Q1 #2). A pending
// secret still in the slot is REUSED (Zalo may already hold it) and is NOT shown again; only an empty
// slot gets fresh CSPRNG output, which is shown once.
func (uc *CommuneZaloBots) RegisterWebhook(ctx context.Context, actor audit.Actor) (CommuneWebhookOutput, error) {
	if actor.ID == "" {
		return CommuneWebhookOutput{}, ErrNoActor
	}
	t, ok, err := uc.registry.XaTrongNguCanh(ctx)
	if err != nil {
		return CommuneWebhookOutput{}, fmt.Errorf("zalo: read the commune's host: %w", err)
	}
	if !ok || !t.Active || t.Host == "" {
		return CommuneWebhookOutput{}, ErrCommuneHostUnknown
	}
	webhookURL := domain.ZaloWebhookURL(t.Host)

	s, token, err := uc.openLive(ctx)
	if err != nil {
		return CommuneWebhookOutput{}, err
	}
	defer clear(token)

	aad := communeBotWebhookAAD(ctx, s.Bot.ID)
	reused := s.PendingSecretSealed != nil
	var plain secret.Secret
	pendingSealed := s.PendingSecretSealed
	if reused {
		if plain, err = uc.envelope.Open(ctx, pendingSealed, aad); err != nil {
			return CommuneWebhookOutput{}, fmt.Errorf("zalo: open the pending webhook secret: %w", err)
		}
	} else {
		if plain, err = uc.newSecret(); err != nil {
			return CommuneWebhookOutput{}, err
		}
		if pendingSealed, err = uc.envelope.Seal(ctx, plain, aad); err != nil {
			clear(plain)
			return CommuneWebhookOutput{}, fmt.Errorf("zalo: seal the webhook secret: %w", err)
		}
	}
	shown := false // whether plain leaves this function — then the caller clears it
	defer func() {
		if !shown {
			clear(plain)
		}
	}()

	// Step 1: the pending secret is stored (or confirmed) with its entry, BEFORE Zalo hears of it.
	at := uc.clock()
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		cur, found, err := uc.bots.LockLive(ctx, tx)
		if err != nil {
			return err
		}
		if !found || cur.Bot.ID != s.Bot.ID {
			return ErrCommuneZaloBotChanged
		}
		switch {
		case reused && !bytes.Equal(cur.PendingSecretSealed, pendingSealed):
			return ErrCommuneZaloBotChanged
		case !reused:
			if err := uc.bots.SetPendingWebhookSecret(ctx, tx, s.Bot.ID, pendingSealed, at); err != nil {
				if errors.Is(err, docstore.ErrZaloRowChanged) {
					return ErrCommuneZaloBotChanged
				}
				return err
			}
		}
		return writeCommuneBotAudit(ctx, tx, actor, domain.ActionRequestCommuneZaloWebhook, s.Bot.Subject(), at, nil,
			map[string]any{"url": webhookURL, "pending_secret_reused": reused}, nil)
	})
	if err != nil {
		return CommuneWebhookOutput{}, fmt.Errorf("zalo: store the pending webhook secret: %w", err)
	}

	// Step 2: Zalo, outside any transaction.
	o := uc.api.SetWebhook(ctx, token, webhookURL, plain)
	result := outcomeValue(o)
	out := CommuneWebhookOutput{Outcome: result, URL: webhookURL}
	refused := o == zalobot.OutcomeTokenRejected || o == zalobot.OutcomeRejected || o == zalobot.OutcomeRateLimited

	// Step 3: one transaction by the outcome's kind.
	done := uc.clock()
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		if _, _, err := uc.bots.LockLive(ctx, tx); err != nil {
			return err
		}
		switch {
		case o == zalobot.OutcomeOK:
			if err := uc.bots.PromotePendingWebhookSecret(ctx, tx, s.Bot.ID, pendingSealed, actor.ID, done); err != nil {
				return err
			}
			out.SetAt, out.SetBy = done, actor.ID
			return writeCommuneBotAudit(ctx, tx, actor, domain.ActionSetCommuneZaloWebhook, s.Bot.Subject(), done, nil,
				map[string]any{"url": webhookURL, "set_at": done, "set_by": actor.ID}, nil)
		case refused:
			// A definite refusal: Zalo never used this secret. A slot that moved meanwhile is left alone.
			if err := uc.bots.ClearPendingWebhookSecret(ctx, tx, s.Bot.ID, pendingSealed, done); err != nil &&
				!errors.Is(err, docstore.ErrZaloRowChanged) {
				return err
			}
			return writeCommuneBotAudit(ctx, tx, actor, domain.ActionRefusedCommuneZaloWebhook, s.Bot.Subject(), done,
				nil, map[string]any{"url": webhookURL, "result": result}, nil)
		default:
			// Ambiguous: Zalo may have applied it; the pending secret stays accepted until the next OK.
			return writeCommuneBotAudit(ctx, tx, actor, domain.ActionUnconfirmedCommuneZaloWebhook, s.Bot.Subject(),
				done, nil, map[string]any{"url": webhookURL, "result": result}, nil)
		}
	})
	if err != nil {
		return CommuneWebhookOutput{}, fmt.Errorf("zalo: record the webhook outcome: %w", err)
	}
	// No URL in the event: the outcome and the bot say what an incident responder needs.
	switch {
	case o == zalobot.OutcomeOK:
		uc.securityEvent(ctx, SecEventCommuneBotWebhook, "confirmed", actor, s.Bot.BotAccountID)
	case refused:
		uc.securityEvent(ctx, SecEventCommuneBotWebhook, "refused:"+result, actor, s.Bot.BotAccountID)
	default:
		uc.securityEvent(ctx, SecEventCommuneBotWebhook, "pending:"+result, actor, s.Bot.BotAccountID)
	}
	if !reused && !refused {
		out.Secret, shown = plain, true
	}
	return out, nil
}

// Retire — "Quay về bot chung" (ADR 0079 Q1 #5): the live own bot is retired with the administrator's
// reason, and every one of its live links ends in the same transaction (0022's deferred check). No live
// own bot = nothing written (the DELETE is idempotent).
func (uc *CommuneZaloBots) Retire(ctx context.Context, rawReason string, actor audit.Actor) (RetireCommuneZaloBotOutput, error) {
	if actor.ID == "" {
		return RetireCommuneZaloBotOutput{}, ErrNoActor
	}
	reason, err := domain.NormalizeCommuneZaloBotRetireReason(rawReason)
	if err != nil {
		return RetireCommuneZaloBotOutput{}, err
	}
	at := uc.clock()
	var out RetireCommuneZaloBotOutput
	var retiredAccount string
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		cur, found, err := uc.bots.LockLive(ctx, tx)
		if err != nil || !found {
			return err
		}
		retiredAccount = cur.Bot.BotAccountID
		if err := uc.bots.Retire(ctx, tx, cur.Bot.ID, actor.ID, reason, at); err != nil {
			return err
		}
		ended, err := uc.endLinks(ctx, tx, cur.Bot.Ref(), actor, at)
		if err != nil {
			return err
		}
		out = RetireCommuneZaloBotOutput{Retired: true, EndedLinkCount: ended}
		return writeCommuneBotAudit(ctx, tx, actor, domain.ActionRetireCommuneZaloBot, cur.Bot.Subject(), at,
			communeBotDelta(cur.Bot), map[string]any{"retired_at": at, "retired_by": actor.ID, "retire_reason": reason},
			map[string]any{"ended_link_count": ended})
	})
	if err != nil {
		return RetireCommuneZaloBotOutput{}, fmt.Errorf("zalo: retire the commune bot: %w", err)
	}
	if out.Retired {
		uc.securityEvent(ctx, SecEventCommuneBotRetired, "retired", actor, retiredAccount)
	}
	return out, nil
}

// openLive reads the live own bot and opens its token. ErrCommuneZaloBotMissing when there is none.
func (uc *CommuneZaloBots) openLive(ctx context.Context) (docstore.CommuneBotSecrets, secret.Secret, error) {
	s, found, err := uc.bots.Sealed(ctx)
	if err != nil {
		return docstore.CommuneBotSecrets{}, nil, err
	}
	if !found {
		return docstore.CommuneBotSecrets{}, nil, ErrCommuneZaloBotMissing
	}
	if uc.envelope == nil {
		return docstore.CommuneBotSecrets{}, nil, crypto.ErrNotConfigured
	}
	token, err := uc.envelope.Open(ctx, s.TokenSealed, communeBotTokenAAD(ctx, s.Bot.ID))
	if err != nil {
		return docstore.CommuneBotSecrets{}, nil, fmt.Errorf("zalo: open the commune bot's token: %w", err)
	}
	return s, token, nil
}

// communeBotDelta is the audit view of a bot — NEVER a sealed column (0022: "before/after carrying set_at /
// set_by / bot_account_id / bot_name / chat_url / retired_* — NEVER a sealed column"). Nothing here is
// personal data: a bot's id, name and chat link are not a person's.
func communeBotDelta(b domain.CommuneZaloBot) map[string]any {
	return map[string]any{
		"bot_account_id": b.BotAccountID, "bot_name": b.BotName, "chat_url": b.ChatURL,
		"set_at": b.SetAt, "set_by": b.SetBy,
	}
}

func writeCommuneBotAudit(ctx context.Context, tx *store.ScopedTx, actor audit.Actor, action, subject string,
	at time.Time, before, after any, facts map[string]any) error {

	m := map[string]any{"truoc": before, "sau": after}
	for k, v := range facts {
		m[k] = v
	}
	delta, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("zalo: encode delta: %w", err)
	}
	return audit.Write(ctx, tx, audit.Entry{Actor: actor, Action: action, Subject: subject, At: at, Delta: delta})
}

// communeWebhookSecretMatches compares presented, in constant time, against the live own bot's secret in
// force AND its pending one (0022 pending-then-promote). FAIL CLOSED: no bot, no secret, no envelope or an
// unopenable seal answer "no match" or an error, never "match". Both slots are always compared.
//
// It returns the bot whose secret matched: everything the update then does is done through THAT bot, never
// a bot re-read later (a concurrent replace could have made another bot live meanwhile).
func communeWebhookSecretMatches(ctx context.Context, bots *docstore.ZaloCommuneBotStore, envelope *crypto.Envelope,
	presented []byte) (domain.CommuneZaloBot, bool, error) {

	if len(presented) == 0 {
		return domain.CommuneZaloBot{}, false, nil
	}
	s, found, err := bots.Sealed(ctx)
	if err != nil {
		return domain.CommuneZaloBot{}, false, err
	}
	if !found || (s.WebhookSecretSealed == nil && s.PendingSecretSealed == nil) {
		return domain.CommuneZaloBot{}, false, nil
	}
	if envelope == nil {
		return domain.CommuneZaloBot{}, false, crypto.ErrNotConfigured
	}
	aad := communeBotWebhookAAD(ctx, s.Bot.ID)
	match := 0
	for _, sealed := range [][]byte{s.WebhookSecretSealed, s.PendingSecretSealed} {
		if sealed == nil {
			continue
		}
		plain, err := envelope.Open(ctx, sealed, aad)
		if err != nil {
			return domain.CommuneZaloBot{}, false, fmt.Errorf("zalo: open a webhook secret: %w", err)
		}
		match |= subtle.ConstantTimeCompare(plain.Lo(), presented)
		clear(plain)
	}
	if match != 1 {
		return domain.CommuneZaloBot{}, false, nil
	}
	return s.Bot, true, nil
}
