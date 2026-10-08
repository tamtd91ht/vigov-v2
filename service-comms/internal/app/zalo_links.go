package app

// THE STAFF SIDE OF THE ZALO BOT CHANNEL (ADR 0074 §"Tài nguyên URL"):
//
//	Current / IssuePairingCode / Unlink / SendTestMessage   the signed-in member of staff's OWN link
//	                                                        (AnyAuthenticated, ADR 0074 #6) — the person
//	                                                        is actor.ID, the session's business code, and
//	                                                        there is no parameter naming anybody else
//	LinkedStaff / Settings / SaveSettings                   the commune's administration (admin.lookup)
//
// Every write and its audit entry share ONE transaction (rule 6, invariant 3). No entry, no error, no log
// line carries a pairing code, a chat_id or a message text.

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/ulid"
	"github.com/vihat/vigov/service-comms/internal/domain"
	docstore "github.com/vihat/vigov/service-comms/internal/store"
	"github.com/vihat/vigov/service-comms/internal/zalobot"
)

var (
	// ErrZaloBotNotConfigured — the shared bot has no token (or no key to open it in this deployment).
	ErrZaloBotNotConfigured = errors.New("zalo: the shared bot is not configured")
	// ErrZaloChannelOff — the commune has not switched its Zalo channel on.
	ErrZaloChannelOff = errors.New("zalo: the commune's channel is off")
	// ErrZaloNotLinked — the member of staff has no live link.
	ErrZaloNotLinked = errors.New("zalo: not linked")
	// ErrStaffNamesUnavailable — identity could not be asked for names. Never rendered as "no name".
	ErrStaffNamesUnavailable = errors.New("zalo: staff names unavailable")
)

// ZaloSendError is a send Zalo refused or could not take: the CLASS only (domain.ZaloCall*), never text.
type ZaloSendError struct{ Class string }

func (e *ZaloSendError) Error() string { return "zalo: send failed: " + e.Class }

// StaffNames is identity's ResolveStaffNames. *identityclient.Client satisfies it.
type StaffNames interface {
	TenCanBoTheoMa(ctx context.Context, ma []string) (map[string]identityclient.TenCanBo, error) // vi-name-ok: the existing core/identityclient method this interface must match
}

// testDeliveryHold is how long a test message's row waits before the SENDER may pick it up — only if the
// direct send below never recorded an outcome (a crash between the two). Longer than any send.
const testDeliveryHold = 15 * time.Minute

// ZaloLinkCurrentView is GET zalo-links/current.
type ZaloLinkCurrentView struct {
	Linked         bool
	LinkedAt       time.Time
	BotName        string
	ChatURL        string
	ChannelEnabled bool
}

// PairingCodeView is the one-time code — returned ONCE, to its owner, and stored nowhere but as a hash.
type PairingCodeView struct {
	Code      string
	ExpiresAt time.Time
	ChatURL   string
}

// LinkedStaffView is one line of GET zalo-links.
type LinkedStaffView struct {
	StaffCode string
	StaffName string
	LinkedAt  time.Time
}

// ZaloLinks owns the staff side.
type ZaloLinks struct {
	db   *store.DB
	repo *docstore.ZaloLinkStore
	// bots answers which bot serves the commune: its own live bot (0022), else the shared one.
	bots  ZaloBotSource
	send  ZaloMessenger
	names StaffNames
	log   *slog.Logger

	newID func() (string, error)
	now   func() time.Time
	// drawCode is the CSPRNG draw of one code (rule 13 #2). Injected so a test can force a collision.
	drawCode func() (string, error)
}

// NewZaloLinks panics on a missing dependency — at construction, not on the first request.
func NewZaloLinks(db *store.DB, repo *docstore.ZaloLinkStore, bots ZaloBotSource, send ZaloMessenger,
	names StaffNames, log *slog.Logger) *ZaloLinks {

	if db == nil || repo == nil || bots == nil || send == nil || names == nil {
		panic("app: NewZaloLinks thiếu phụ thuộc")
	}
	if log == nil {
		log = slog.Default()
	}
	return &ZaloLinks{db: db, repo: repo, bots: bots, send: send, names: names, log: log,
		newID: ulid.Moi, now: time.Now, drawCode: drawPairingCode}
}

// drawPairingCode draws PairingCodeLength symbols uniformly from the alphabet with crypto/rand.
func drawPairingCode() (string, error) {
	n := big.NewInt(int64(len(domain.PairingCodeAlphabet)))
	b := make([]byte, domain.PairingCodeLength)
	for i := range b {
		k, err := rand.Int(rand.Reader, n)
		if err != nil {
			return "", fmt.Errorf("zalo: draw a pairing code: %w", err)
		}
		b[i] = domain.PairingCodeAlphabet[k.Int64()]
	}
	return string(b), nil
}

func (uc *ZaloLinks) clock() time.Time { return uc.now().UTC().Truncate(time.Microsecond) }

// Current is the member of staff's own state. The bot's name and chat link are shown whenever the bot is
// configured — the person needs the link BEFORE pairing, to open the chat at all.
func (uc *ZaloLinks) Current(ctx context.Context, actor audit.Actor) (ZaloLinkCurrentView, error) {
	if actor.ID == "" {
		return ZaloLinkCurrentView{}, ErrNoActor
	}
	var v ZaloLinkCurrentView
	link, linked, err := uc.repo.LiveLink(ctx, actor.ID)
	if err != nil {
		return v, fmt.Errorf("zalo: read the link: %w", err)
	}
	if linked {
		v.Linked, v.LinkedAt = true, link.LinkedAt
	}
	s, err := uc.repo.ChannelSetting(ctx)
	if err != nil {
		return v, fmt.Errorf("zalo: read the settings: %w", err)
	}
	v.ChannelEnabled = s.Saved && s.IsEnabled
	b, err := uc.bots.Active(ctx)
	if err != nil {
		return v, err
	}
	if b.Configured {
		v.BotName, v.ChatURL = b.BotName, b.ChatURL
	}
	return v, nil
}

// BotReady is spec Cấu hình 11 §0's `platform_ready`: some bot can serve the commune — its own live bot,
// or the shared bot with a token. false = no message can go out, whatever the settings say.
func (uc *ZaloLinks) BotReady(ctx context.Context) (bool, error) {
	b, err := uc.bots.Active(ctx)
	if err != nil {
		return false, err
	}
	return b.Configured, nil
}

// maxCodeDraws bounds the redraws on a hash collision in this commune's history. 31^8 codes make even one
// collision a once-in-a-lifetime event; five in a row means something else is wrong.
const maxCodeDraws = 5

// IssuePairingCode issues a new one-time code for the signed-in member of staff, cancelling any open one
// in the same transaction (0018: at most one open code per member of staff). Refused while the shared bot
// is not configured: a code for a bot nobody can reach is a dead credential.
func (uc *ZaloLinks) IssuePairingCode(ctx context.Context, actor audit.Actor) (PairingCodeView, error) {
	if actor.ID == "" {
		return PairingCodeView{}, ErrNoActor
	}
	b, err := uc.bots.Active(ctx)
	if err != nil {
		return PairingCodeView{}, err
	}
	if !b.Configured {
		return PairingCodeView{}, ErrZaloBotNotConfigured
	}
	at := uc.clock()
	expires := at.Add(domain.PairingCodeTTL)
	var code string
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		cancelled, err := uc.repo.CancelOpenPairingCodes(ctx, tx, actor.ID, at)
		if err != nil {
			return err
		}
		id, err := uc.newID()
		if err != nil {
			return fmt.Errorf("zalo: new id: %w", err)
		}
		for i := 0; ; i++ {
			if code, err = uc.drawCode(); err != nil {
				return err
			}
			err = uc.repo.InsertPairingCode(ctx, tx, id, actor.ID, domain.PairingCodeHash(code), expires, at)
			if err == nil {
				break
			}
			if !errors.Is(err, docstore.ErrPairingCodeTaken) || i+1 >= maxCodeDraws {
				return err
			}
		}
		// The code is NOT in the entry: the trail is kept for years and the code is a credential.
		delta, err := json.Marshal(map[string]any{"ma_ghep_id": id, "het_han": expires, "huy_ma_cu": cancelled})
		if err != nil {
			return fmt.Errorf("zalo: encode delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{Actor: actor, Action: domain.ActionIssueZaloPairingCode,
			Subject: actor.ID, At: at, Delta: delta})
	})
	if err != nil {
		return PairingCodeView{}, fmt.Errorf("zalo: issue a pairing code: %w", err)
	}
	return PairingCodeView{Code: code, ExpiresAt: expires, ChatURL: b.ChatURL}, nil
}

// Unlink ends the signed-in member of staff's live link. Not linked = nothing written, nothing recorded
// (the DELETE is idempotent).
func (uc *ZaloLinks) Unlink(ctx context.Context, actor audit.Actor) error {
	if actor.ID == "" {
		return ErrNoActor
	}
	at := uc.clock()
	err := uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		link, found, err := uc.repo.LockLiveLinkOfStaff(ctx, tx, actor.ID)
		if err != nil || !found {
			return err
		}
		if err := uc.repo.EndLink(ctx, tx, link.ID, actor.ID, domain.ZaloLinkEndedByStaff, at); err != nil {
			return err
		}
		return writeLinkEnded(ctx, tx, actor, link, domain.ZaloLinkEndedByStaff, "go_tren_vigov", at)
	})
	if err != nil {
		return fmt.Errorf("zalo: unlink: %w", err)
	}
	return nil
}

// writeLinkEnded is the ket_thuc_lien_ket_zalo entry — the same verb the operator's bot-account change
// writes (platformstore.EndAllLiveLinks), subject the link owner's business code.
func writeLinkEnded(ctx context.Context, tx *store.ScopedTx, actor audit.Actor, link domain.ZaloLink,
	reason, source string, at time.Time) error {

	delta, err := json.Marshal(map[string]any{
		"lien_ket_id": link.ID, "lien_ket_tu": link.LinkedAt, "ly_do": reason, "nguon": source,
		"bot_ref": link.BotRef,
	})
	if err != nil {
		return fmt.Errorf("zalo: encode delta: %w", err)
	}
	return audit.Write(ctx, tx, audit.Entry{Actor: actor, Action: domain.ActionEndZaloLink,
		Subject: link.StaffCode, At: at, Delta: delta})
}

// SendTestMessage sends the fixed test message to the member of staff's own chat, NOW, and records it in
// zalo_delivery (kind thu-nghiem): the row and its entry first, then the send outside any transaction, then
// the outcome with its entry. Refused — before anything is written — while the commune's channel is off,
// the bot is not configured, or the person is not linked.
func (uc *ZaloLinks) SendTestMessage(ctx context.Context, actor audit.Actor) error {
	if actor.ID == "" {
		return ErrNoActor
	}
	s, err := uc.repo.ChannelSetting(ctx)
	if err != nil {
		return fmt.Errorf("zalo: read the settings: %w", err)
	}
	if !s.Saved || !s.IsEnabled {
		return ErrZaloChannelOff
	}
	bot, token, err := uc.bots.ActiveToken(ctx)
	if err != nil {
		uc.log.WarnContext(ctx, "CẢNH BÁO: không mở được token Zalo Bot của xã — gửi thử bị từ chối", "err", err)
		return ErrZaloBotNotConfigured
	}
	if !bot.Configured {
		return ErrZaloBotNotConfigured
	}
	defer clear(token)

	at := uc.clock()
	id, err := uc.newID()
	if err != nil {
		return fmt.Errorf("zalo: new id: %w", err)
	}
	var chat docstore.LinkedChat
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		chats, err := uc.repo.LiveChatsOf(ctx, tx, bot.Ref, []string{actor.ID})
		if err != nil {
			return err
		}
		c, ok := chats[actor.ID]
		if !ok {
			return ErrZaloNotLinked
		}
		chat = c
		if err := uc.repo.InsertTestDelivery(ctx, tx, id, actor.ID, c.LinkID, at.Add(testDeliveryHold), at); err != nil {
			return err
		}
		if err := uc.repo.StartAttempt(ctx, tx, id, c.LinkID, at.Add(testDeliveryHold), at); err != nil {
			return err
		}
		return writeTestAudit(ctx, tx, actor, id, "", at)
	})
	if err != nil {
		if errors.Is(err, ErrZaloNotLinked) {
			return ErrZaloNotLinked
		}
		return fmt.Errorf("zalo: queue the test message: %w", err)
	}

	o := uc.send.SendMessage(ctx, token, chat.ChatID, domain.ZaloTestMessageText)
	result := outcomeValue(o)
	done := uc.clock()
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		if o == zalobot.OutcomeOK {
			if err := uc.repo.MarkDeliverySent(ctx, tx, id, done); err != nil {
				return err
			}
		} else if err := uc.repo.FailDelivery(ctx, tx, id, result, done); err != nil {
			return err
		}
		return writeTestAudit(ctx, tx, actor, id, result, done)
	})
	if err != nil {
		// Zalo answered; the record did not land. The row stays owed and the sender settles it after the
		// hold — the person may see the message twice, never a lost trail.
		return fmt.Errorf("zalo: record the test outcome: %w", err)
	}
	if o != zalobot.OutcomeOK {
		return &ZaloSendError{Class: result}
	}
	return nil
}

func writeTestAudit(ctx context.Context, tx *store.ScopedTx, actor audit.Actor, id, result string, at time.Time) error {
	m := map[string]any{"tin_id": id, "loai": domain.ZaloKindTest}
	if result != "" {
		m["ket_qua"] = result
	}
	delta, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("zalo: encode delta: %w", err)
	}
	return audit.Write(ctx, tx, audit.Entry{Actor: actor, Action: domain.ActionSendZaloTestMessage,
		Subject: actor.ID, At: at, Delta: delta})
}

// LinkedStaff lists the commune's live links with each person's name from identity. A name identity
// cannot give (a record since removed) is "" — rendered as unknown; an identity OUTAGE is an error, never
// a page of blanks (core/identityclient.TenCanBoTheoMa).
func (uc *ZaloLinks) LinkedStaff(ctx context.Context) ([]LinkedStaffView, error) {
	links, err := uc.repo.LiveLinks(ctx)
	if err != nil {
		return nil, fmt.Errorf("zalo: list links: %w", err)
	}
	names := map[string]identityclient.TenCanBo{}
	for start := 0; start < len(links); start += identityclient.TranMaMotLo {
		end := min(start+identityclient.TranMaMotLo, len(links))
		codes := make([]string, 0, end-start)
		for _, l := range links[start:end] {
			codes = append(codes, l.StaffCode)
		}
		got, err := uc.names.TenCanBoTheoMa(ctx, codes)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrStaffNamesUnavailable, err)
		}
		for k, v := range got {
			names[k] = v
		}
	}
	out := make([]LinkedStaffView, 0, len(links))
	for _, l := range links {
		out = append(out, LinkedStaffView{StaffCode: l.StaffCode, StaffName: names[l.StaffCode].HoTen, LinkedAt: l.LinkedAt})
	}
	return out, nil
}

// Settings reads the commune's channel settings; no row = off with the defaults.
func (uc *ZaloLinks) Settings(ctx context.Context) (domain.ZaloChannelSetting, error) {
	s, err := uc.repo.ChannelSetting(ctx)
	if err != nil {
		return domain.ZaloChannelSetting{}, fmt.Errorf("zalo: read the settings: %w", err)
	}
	return s, nil
}

// SaveSettings validates and saves the commune's channel settings, with before/after in the entry. A save
// that changes nothing (and a row already exists) writes and records nothing.
func (uc *ZaloLinks) SaveSettings(ctx context.Context, in domain.ZaloChannelSetting, actor audit.Actor) (
	domain.ZaloChannelSetting, error) {

	if actor.ID == "" {
		return domain.ZaloChannelSetting{}, ErrNoActor
	}
	v, err := domain.NormalizeZaloChannelSetting(in)
	if err != nil {
		return domain.ZaloChannelSetting{}, err
	}
	at := uc.clock()
	var out domain.ZaloChannelSetting
	err = uc.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		before, err := uc.repo.LockChannelSetting(ctx, tx)
		if err != nil {
			return err
		}
		// A row still holding an OLD kind (0021) is rewritten even when it reads the same: the save is
		// what moves it onto per-domain values.
		if before.Saved && before.Same(v) && !before.LegacyKindsStored {
			out = before
			return nil
		}
		if err := uc.repo.SaveChannelSetting(ctx, tx, v, actor.ID, at); err != nil {
			return err
		}
		out = v
		out.Saved, out.UpdatedAt, out.UpdatedBy = true, at, actor.ID
		var b any
		if before.Saved {
			b = settingDelta(before)
		}
		delta, err := json.Marshal(map[string]any{"truoc": b, "sau": settingDelta(out)})
		if err != nil {
			return fmt.Errorf("zalo: encode delta: %w", err)
		}
		return audit.Write(ctx, tx, audit.Entry{Actor: actor, Action: domain.ActionSaveZaloChannelSetting,
			Subject: "zalo-channel-settings", At: at, Delta: delta})
	})
	if err != nil {
		return domain.ZaloChannelSetting{}, fmt.Errorf("zalo: save the settings: %w", err)
	}
	return out, nil
}

// settingDelta — every field is configuration; none is personal data.
func settingDelta(s domain.ZaloChannelSetting) map[string]any {
	return map[string]any{
		"is_enabled": s.IsEnabled, "kinds": s.Kinds,
		"quiet_start": domain.FormatClock(s.QuietStartMinute), "quiet_end": domain.FormatClock(s.QuietEndMinute),
		"overdue_start_after_days": s.OverdueStartAfterDays, "overdue_repeat_every_days": s.OverdueRepeatEveryDays,
	}
}
