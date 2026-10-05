package app

// WHAT A CHAT SAYS TO THE SHARED ZALO BOT (ADR 0074 #5) — after internal/http has authenticated the
// update against the webhook secret and parsed it (zalobot.ParseUpdate).
//
//	a pairing code      pair this chat with the code's member of staff, in the CODE's commune
//	/trogiup /help      the help text            /batdau /start   the same
//	/dung /stop         end this chat's link, in the LINK's commune
//	an unsupported msg  the explanatory reply
//	any other text      the help text (nothing counted: it is not code-shaped)
//
// THE COMMUNE: there is no commune Host on this path. It is found FROM the pairing code, or FROM the live
// link (crosstenant), and the work then runs in that commune's context through core/store. Nothing found
// = the one refusal sentence, never a guess (ADR 0074 #5: "không ra xã thì bỏ gói, không đoán").
//
// WHO: the member of staff the code or the link belongs to — the person who proved possession of the
// code, or whose chat it is — as actor kind "staff" with their business code (rule 6, invariant 8). The
// IP is the address the webhook request came from (Zalo's server): the one request this act had.
//
// NEVER LOGGED, NEVER IN AN ENTRY: chat_id, the text, the code (rule 3; ADR 0074).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/ratelimit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-comms/internal/domain"
	docstore "github.com/vihat/vigov/service-comms/internal/store"
	"github.com/vihat/vigov/service-comms/internal/store/crosstenant"
	"github.com/vihat/vigov/service-comms/internal/zalobot"
)

// ZaloChatResolver is the cross-commune half. *crosstenant.ZaloBot satisfies it.
type ZaloChatResolver interface {
	OpenPairingCodesByHash(ctx context.Context, hash []byte) ([]crosstenant.PairingCodeMatch, error)
	CommuneOfLiveChat(ctx context.Context, chatID string) (tenant.ID, bool, error)
	EndChatLinksInOtherCommunes(ctx context.Context, tx *store.ScopedTx, chatID string, at time.Time) (int, error)
}

// ZaloPairingLimiter is ratelimit.ZaloBotPairing's limiter. *ratelimit.Limiter satisfies it.
type ZaloPairingLimiter interface {
	Allow(ctx context.Context, key ratelimit.Key) (bool, time.Duration, error)
}

// errPairingRefused rolls a pairing back with the one refusal sentence as its answer.
var errPairingRefused = errors.New("zalo: pairing refused")

// maxConcurrentReplies bounds the replies in flight at once; past it a reply is sent in the request.
const maxConcurrentReplies = 32

// ZaloWebhook handles authenticated updates.
type ZaloWebhook struct {
	db       *store.DB
	repo     *docstore.ZaloLinkStore
	resolver ZaloChatResolver
	limiter  ZaloPairingLimiter
	bot      SharedZaloBot
	send     ZaloMessenger
	log      *slog.Logger

	newID   func() (string, error)
	now     func() time.Time
	replies chan struct{}
}

// NewZaloWebhook panics on a missing dependency.
func NewZaloWebhook(db *store.DB, repo *docstore.ZaloLinkStore, resolver ZaloChatResolver,
	limiter ZaloPairingLimiter, bot SharedZaloBot, send ZaloMessenger, log *slog.Logger) *ZaloWebhook {

	if db == nil || repo == nil || resolver == nil || limiter == nil || bot == nil || send == nil {
		panic("app: NewZaloWebhook thiếu phụ thuộc")
	}
	if log == nil {
		log = slog.Default()
	}
	return &ZaloWebhook{db: db, repo: repo, resolver: resolver, limiter: limiter, bot: bot, send: send, log: log,
		newID: ulid.Moi, now: time.Now, replies: make(chan struct{}, maxConcurrentReplies)}
}

// Authenticate compares the X-Bot-Api-Secret-Token header with the stored secrets, in constant time.
func (w *ZaloWebhook) Authenticate(ctx context.Context, presented string) (bool, error) {
	return w.bot.WebhookSecretMatches(ctx, []byte(presented))
}

// Handle decides what one update does and returns the reply to send ("" = none). It never fails the
// request: Zalo is answered 200 once authenticated, whatever happens here.
func (w *ZaloWebhook) Handle(ctx context.Context, u zalobot.Update, clientIP string) string {
	switch u.EventName {
	case domain.ZaloEventUnsupportedReceived:
		return domain.ZaloReplyUnsupported
	case domain.ZaloEventTextReceived:
	default:
		return "" // an event this bot does not act on: answered 200, nothing said
	}
	switch domain.ParseZaloCommand(u.Text) {
	case domain.ZaloCommandHelp:
		return domain.ZaloReplyHelp
	case domain.ZaloCommandStop:
		return w.stop(ctx, u.ChatID, clientIP)
	}
	if code, ok := domain.NormalizePairingCode(u.Text); ok {
		return w.pair(ctx, u.ChatID, code, clientIP)
	}
	return domain.ZaloReplyHelp
}

func (w *ZaloWebhook) clock() time.Time { return w.now().UTC().Truncate(time.Microsecond) }

// pair runs one pairing attempt (ADR 0074; 0018 zalo_pairing_code, zalo_link).
func (w *ZaloWebhook) pair(ctx context.Context, chatID, code, clientIP string) string {
	key, err := ratelimit.ZaloChatKey(chatID)
	if err != nil {
		return domain.ZaloReplyPairingRefused
	}
	allowed, _, err := w.limiter.Allow(ctx, key)
	if err != nil {
		// FAIL CLOSED (ratelimit.ZaloBotPairing): the guess bound is down, so no guess is checked.
		w.log.WarnContext(ctx, "CẢNH BÁO HẠ TẦNG: không đếm được giới hạn ghép nối Zalo — từ chối (đóng kín)", "err", err)
		return domain.ZaloReplyUnavailable
	}
	if !allowed {
		// A security event; the chat is not named — a digest would add nothing an operator can act on.
		w.log.WarnContext(ctx, "CẢNH BÁO BẢO MẬT: một chat Zalo vượt giới hạn thử mã ghép nối — từ chối",
			"event", "zalo_bot_pairing.rate_limited", "outcome", "refused", "ip", clientIP)
		return domain.ZaloReplyPairingLimited
	}

	matches, err := w.resolver.OpenPairingCodesByHash(ctx, domain.PairingCodeHash(code))
	if err != nil {
		w.log.ErrorContext(ctx, "ghép nối Zalo: không tra được mã ghép", "err", err)
		return domain.ZaloReplyUnavailable
	}
	if len(matches) != 1 {
		// None, or two communes holding the same live code: NO match (0018, fail closed).
		return domain.ZaloReplyPairingRefused
	}
	m := matches[0]
	cctx := tenant.Into(ctx, m.TenantID)
	at := w.clock()
	err = w.db.For(cctx).Tx(cctx, func(tx *store.ScopedTx) error {
		c, err := w.repo.LockOpenPairingCode(cctx, tx, m.CodeID)
		if errors.Is(err, docstore.ErrPairingCodeNotOpen) {
			return errPairingRefused
		}
		if err != nil {
			return err
		}
		if !at.Before(c.ExpiresAt) || c.FailedAttempts >= domain.PairingMaxFailedAttempts {
			return errPairingRefused
		}
		staff := audit.Actor{ID: c.StaffCode, Kind: "staff", IP: clientIP}
		if err := w.repo.MarkPairingCodeUsed(cctx, tx, c.ID, at); err != nil {
			return err
		}
		// "Một chat một tài khoản": the chat's live link in ANOTHER commune ends first (audited there, by
		// the system), then in THIS commune if it is somebody else's, then this person's previous link.
		elsewhere, err := w.resolver.EndChatLinksInOtherCommunes(cctx, tx, chatID, at)
		if err != nil {
			return err
		}
		if other, found, err := w.repo.LockLiveLinkOfChat(cctx, tx, chatID); err != nil {
			return err
		} else if found && other.StaffCode != c.StaffCode {
			if err := w.repo.EndLink(cctx, tx, other.ID, c.StaffCode, domain.ZaloLinkEndedByChatTaken, at); err != nil {
				return err
			}
			if err := writeLinkEnded(cctx, tx, staff, other, domain.ZaloLinkEndedByChatTaken, "ghep_cho_tai_khoan_khac", at); err != nil {
				return err
			}
		}
		repaired := false
		if own, found, err := w.repo.LockLiveLinkOfStaff(cctx, tx, c.StaffCode); err != nil {
			return err
		} else if found {
			if err := w.repo.EndLink(cctx, tx, own.ID, c.StaffCode, domain.ZaloLinkEndedByRepair, at); err != nil {
				return err
			}
			if err := writeLinkEnded(cctx, tx, staff, own, domain.ZaloLinkEndedByRepair, "ghep_lai", at); err != nil {
				return err
			}
			repaired = true
		}
		linkID, err := w.newID()
		if err != nil {
			return fmt.Errorf("zalo: new id: %w", err)
		}
		if err := w.repo.InsertLink(cctx, tx, linkID, c.StaffCode, chatID, at); err != nil {
			return err
		}
		delta, err := json.Marshal(map[string]any{
			"lien_ket_id": linkID, "ma_ghep_id": c.ID, "bot_ref": domain.SharedZaloBotRef, "kenh": "zalo_bot",
			"ghep_lai": repaired, "ket_thuc_o_xa_khac": elsewhere,
		})
		if err != nil {
			return fmt.Errorf("zalo: encode delta: %w", err)
		}
		return audit.Write(cctx, tx, audit.Entry{Actor: staff, Action: domain.ActionPairZaloLink,
			Subject: c.StaffCode, At: at, Delta: delta})
	})
	switch {
	case errors.Is(err, errPairingRefused):
		return domain.ZaloReplyPairingRefused
	case err != nil:
		w.log.ErrorContext(ctx, "ghép nối Zalo: lỗi hệ thống", "xa", string(m.TenantID), "err", err)
		return domain.ZaloReplyUnavailable
	}
	return domain.ZaloReplyPaired
}

// stop ends the chat's live link (/dung), in the link's commune, attributed to its owner.
func (w *ZaloWebhook) stop(ctx context.Context, chatID, clientIP string) string {
	commune, found, err := w.resolver.CommuneOfLiveChat(ctx, chatID)
	if err != nil {
		w.log.ErrorContext(ctx, "Zalo /dung: không tra được liên kết", "err", err)
		return domain.ZaloReplyUnavailable
	}
	if !found {
		return domain.ZaloReplyNotLinked
	}
	cctx := tenant.Into(ctx, commune)
	at := w.clock()
	ended := false
	err = w.db.For(cctx).Tx(cctx, func(tx *store.ScopedTx) error {
		link, found, err := w.repo.LockLiveLinkOfChat(cctx, tx, chatID)
		if err != nil || !found {
			return err
		}
		if err := w.repo.EndLink(cctx, tx, link.ID, link.StaffCode, domain.ZaloLinkEndedByStop, at); err != nil {
			return err
		}
		ended = true
		return writeLinkEnded(cctx, tx, audit.Actor{ID: link.StaffCode, Kind: "staff", IP: clientIP}, link,
			domain.ZaloLinkEndedByStop, "lenh_dung_trong_zalo", at)
	})
	if err != nil {
		w.log.ErrorContext(ctx, "Zalo /dung: lỗi hệ thống", "xa", string(commune), "err", err)
		return domain.ZaloReplyUnavailable
	}
	if !ended {
		return domain.ZaloReplyNotLinked
	}
	return domain.ZaloReplyStopped
}

// Reply sends text to chatID through the shared bot, AFTER the webhook has answered Zalo — in its own
// goroutine with its own deadline, bounded: past maxConcurrentReplies it is sent in the caller instead.
// A reply that fails is logged by class; the act it reports has already committed.
func (w *ZaloWebhook) Reply(chatID, text string) {
	if text == "" {
		return
	}
	select {
	case w.replies <- struct{}{}:
		go func() {
			defer func() { <-w.replies }()
			w.replyNow(chatID, text)
		}()
	default:
		w.replyNow(chatID, text)
	}
}

func (w *ZaloWebhook) replyNow(chatID, text string) {
	ctx, cancel := context.WithTimeout(context.Background(), zalobot.DefaultTimeout)
	defer cancel()
	defer func() {
		if p := recover(); p != nil {
			w.log.Error("trả lời Zalo bị panic", "panic", fmt.Sprint(p))
		}
	}()
	token, configured, err := w.bot.Token(ctx)
	if err != nil || !configured {
		w.log.WarnContext(ctx, "CẢNH BÁO: không trả lời được chat Zalo — bot dùng chung chưa có token mở được", "err", err)
		return
	}
	defer clear(token)
	if o := w.send.SendMessage(ctx, token, chatID, text); o != zalobot.OutcomeOK {
		w.log.WarnContext(ctx, "CẢNH BÁO: Zalo không nhận câu trả lời của bot", "ket_qua", o.String())
	}
}
