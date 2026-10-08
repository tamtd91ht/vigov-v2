package app

// WHAT A CHAT SAYS TO A COMMUNE'S OWN ZALO BOT (ADR 0079 Q1 #2; migration 0022) — the commune bot's half
// of zalo_webhook.go, after internal/http has authenticated the update against THIS commune's webhook
// secret and parsed it.
//
// THE COMMUNE IS THE CONTEXT'S, from Host (rule 1 invariant 3; Zalo calls `https://<xã>/api/v1/zalo-bot-
// updates`). NOTHING HERE READS ANOTHER COMMUNE: the pairing code is looked up in this commune only, and
// the chat's link is this commune's — a commune bot's links live in its commune alone (0022). A code from
// another commune is "not found", the one refusal sentence.
//
// THE BOT IS THE ONE THAT AUTHENTICATED THE UPDATE. Authenticate returns a context carrying the bot whose
// secret matched; Handle and Reply act through THAT bot and never re-read "the live bot", which a
// concurrent replace may have changed between the two. A pairing through a bot retired meanwhile is then
// refused by 0022's insert trigger (its bot_ref is no longer live), and no reply goes out through a bot
// that is not the one Zalo called.
//
// The commands, the replies and the pairing steps are the shared bot's (pairInCommune, stopInCommune);
// only the bot differs: links carry this bot's bot_ref, replies go through this bot's token.
//
// NEVER LOGGED, NEVER IN AN ENTRY: chat_id, the text, the code, the secret header (rule 3; ADR 0074).

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/vihat/vigov/core/crypto"
	"github.com/vihat/vigov/core/ratelimit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-comms/internal/domain"
	docstore "github.com/vihat/vigov/service-comms/internal/store"
	"github.com/vihat/vigov/service-comms/internal/zalobot"
)

// CommuneZaloWebhook handles authenticated updates of the commune's own bot.
type CommuneZaloWebhook struct {
	db       *store.DB
	repo     *docstore.ZaloLinkStore
	bots     *docstore.ZaloCommuneBotStore
	envelope *crypto.Envelope // the COMMUNE envelope; nil = nothing opens, every update is refused
	limiter  ZaloPairingLimiter
	send     ZaloMessenger
	log      *slog.Logger

	newID   func() (string, error)
	now     func() time.Time
	replies chan struct{}
}

// NewCommuneZaloWebhook panics on a missing dependency.
func NewCommuneZaloWebhook(db *store.DB, repo *docstore.ZaloLinkStore, bots *docstore.ZaloCommuneBotStore,
	envelope *crypto.Envelope, limiter ZaloPairingLimiter, send ZaloMessenger, log *slog.Logger) *CommuneZaloWebhook {

	if db == nil || repo == nil || bots == nil || limiter == nil || send == nil {
		panic("app: NewCommuneZaloWebhook thiếu phụ thuộc")
	}
	if log == nil {
		log = slog.Default()
	}
	return &CommuneZaloWebhook{db: db, repo: repo, bots: bots, envelope: envelope, limiter: limiter, send: send,
		log: log, newID: ulid.Moi, now: time.Now, replies: make(chan struct{}, maxConcurrentReplies)}
}

func (w *CommuneZaloWebhook) clock() time.Time { return w.now().UTC().Truncate(time.Microsecond) }

// authenticatedBotKey carries the bot whose secret matched, from Authenticate to Handle and Reply.
type authenticatedBotKey struct{}

// authenticatedBot is the bot Authenticate put in ctx. ok=false: none — the caller refuses (fail closed).
func authenticatedBot(ctx context.Context) (domain.CommuneZaloBot, bool) {
	b, ok := ctx.Value(authenticatedBotKey{}).(domain.CommuneZaloBot)
	return b, ok && b.ID != "" && b.BotAccountID != ""
}

// Authenticate compares the X-Bot-Api-Secret-Token header with THIS commune's live bot's secrets — the
// one in force and the pending one — in constant time. No live own bot = no match. On a match the
// returned context carries THAT bot; the caller passes it to Handle and Reply.
func (w *CommuneZaloWebhook) Authenticate(ctx context.Context, presented string) (context.Context, bool, error) {
	bot, ok, err := communeWebhookSecretMatches(ctx, w.bots, w.envelope, []byte(presented))
	if err != nil || !ok {
		return ctx, false, err
	}
	return context.WithValue(ctx, authenticatedBotKey{}, bot), true, nil
}

// Handle decides what one update does and returns the reply to send ("" = none). Zalo is answered 200
// once authenticated, whatever happens here.
func (w *CommuneZaloWebhook) Handle(ctx context.Context, u zalobot.Update, clientIP string) string {
	bot, ok := authenticatedBot(ctx)
	if !ok {
		// Wiring fault: Handle without Authenticate's context. Nothing is done through an unknown bot.
		w.log.ErrorContext(ctx, "webhook bot riêng: thiếu bot đã xác thực trong ngữ cảnh — bỏ gói")
		return ""
	}
	switch u.EventName {
	case domain.ZaloEventUnsupportedReceived:
		return domain.ZaloReplyUnsupported
	case domain.ZaloEventTextReceived:
	default:
		return ""
	}
	switch domain.ParseZaloCommand(u.Text) {
	case domain.ZaloCommandHelp:
		return domain.ZaloReplyHelp
	case domain.ZaloCommandStop:
		return w.stop(ctx, bot, u.ChatID, clientIP)
	}
	if code, ok := domain.NormalizePairingCode(u.Text); ok {
		return w.pair(ctx, bot, u.ChatID, code, clientIP)
	}
	return domain.ZaloReplyHelp
}

func (w *CommuneZaloWebhook) pair(ctx context.Context, bot domain.CommuneZaloBot, chatID, code, clientIP string) string {
	// Per (commune, bot, chat): one commune's chats never spend another's budget.
	key, err := ratelimit.ZaloCommuneChatKey(ctx, bot.Ref(), chatID)
	if err != nil {
		return domain.ZaloReplyPairingRefused
	}
	allowed, _, err := w.limiter.Allow(ctx, key)
	if err != nil {
		w.log.WarnContext(ctx, "CẢNH BÁO HẠ TẦNG: không đếm được giới hạn ghép nối Zalo — từ chối (đóng kín)", "err", err)
		return domain.ZaloReplyUnavailable
	}
	if !allowed {
		w.log.WarnContext(ctx, "CẢNH BÁO BẢO MẬT: một chat Zalo vượt giới hạn thử mã ghép nối — từ chối",
			"event", "zalo_bot_pairing.rate_limited", "outcome", "refused", "xa", string(tenant.MustFrom(ctx)),
			"ip", clientIP)
		return domain.ZaloReplyPairingLimited
	}
	// THIS commune's open code, by hash — never another commune's (ADR 0079 Q1 #2).
	codeID, found, err := w.repo.OpenPairingCodeByHash(ctx, domain.PairingCodeHash(code))
	if err != nil {
		w.log.ErrorContext(ctx, "ghép nối Zalo (bot riêng): không tra được mã ghép", "err", err)
		return domain.ZaloReplyUnavailable
	}
	if !found {
		return domain.ZaloReplyPairingRefused
	}
	at := w.clock()
	// Through the AUTHENTICATED bot's ref: retired meanwhile, 0022's insert trigger refuses the link.
	err = w.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		return pairInCommune(ctx, tx, w.repo, codeID, chatID, bot.Ref(), clientIP, at, w.newID, nil)
	})
	switch {
	case errors.Is(err, errPairingRefused):
		return domain.ZaloReplyPairingRefused
	case err != nil:
		w.log.ErrorContext(ctx, "ghép nối Zalo (bot riêng): lỗi hệ thống", "err", err)
		return domain.ZaloReplyUnavailable
	}
	return domain.ZaloReplyPaired
}

func (w *CommuneZaloWebhook) stop(ctx context.Context, bot domain.CommuneZaloBot, chatID, clientIP string) string {
	reply, err := stopInCommune(ctx, w.db, w.repo, bot.Ref(), chatID, clientIP, w.clock())
	if err != nil {
		w.log.ErrorContext(ctx, "Zalo /dung (bot riêng): lỗi hệ thống", "err", err)
		return domain.ZaloReplyUnavailable
	}
	return reply
}

// Reply sends text to chatID through the AUTHENTICATED bot, AFTER the webhook has answered Zalo — in its
// own goroutine with its own deadline, bounded like the shared bot's. ctx is Authenticate's (commune +
// bot); it is detached from the request so Zalo hanging up does not cancel the reply.
func (w *CommuneZaloWebhook) Reply(ctx context.Context, chatID, text string) {
	if text == "" {
		return
	}
	rctx := context.WithoutCancel(ctx)
	select {
	case w.replies <- struct{}{}:
		go func() {
			defer func() { <-w.replies }()
			w.replyNow(rctx, chatID, text)
		}()
	default:
		w.replyNow(rctx, chatID, text)
	}
}

func (w *CommuneZaloWebhook) replyNow(ctx context.Context, chatID, text string) {
	ctx, cancel := context.WithTimeout(ctx, zalobot.DefaultTimeout)
	defer cancel()
	defer func() {
		if p := recover(); p != nil {
			w.log.Error("trả lời Zalo (bot riêng) bị panic", "panic", fmt.Sprint(p))
		}
	}()
	bot, ok := authenticatedBot(ctx)
	if !ok {
		w.log.ErrorContext(ctx, "webhook bot riêng: thiếu bot đã xác thực — không trả lời")
		return
	}
	s, found, err := w.bots.Sealed(ctx)
	if err != nil || !found || w.envelope == nil {
		w.log.WarnContext(ctx, "CẢNH BÁO: không trả lời được chat Zalo — bot riêng của xã không mở được token", "err", err)
		return
	}
	// Another bot is live now (replaced meanwhile): the chat wrote to the OLD one, and an answer from a
	// different bot would land in a chat the commune no longer serves. Say nothing.
	if s.Bot.ID != bot.ID {
		w.log.WarnContext(ctx, "webhook bot riêng: bot của xã đã đổi trong lúc xử lý — không trả lời qua bot khác")
		return
	}
	token, err := w.envelope.Open(ctx, s.TokenSealed, communeBotTokenAAD(ctx, s.Bot.ID))
	if err != nil {
		w.log.WarnContext(ctx, "CẢNH BÁO: không mở được token bot riêng của xã để trả lời", "err", err)
		return
	}
	defer clear(token)
	if o := w.send.SendMessage(ctx, token, chatID, text); o != zalobot.OutcomeOK {
		w.log.WarnContext(ctx, "CẢNH BÁO: Zalo không nhận câu trả lời của bot riêng", "ket_qua", o.String())
	}
}
