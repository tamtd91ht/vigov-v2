package app

// A GROUP CODE TYPED IN A GROUP CHAT (ADR 0092 #3, #9; transaction boundary gan_nhom_zalo_bang_ma) — both
// bots. Reached from outsidePrivateChat only for a chat whose type is GROUP; a chat of unknown type keeps
// the fixed personal-code refusal and is never looked up (fail closed).
//
//	limiter    the per-chat pairing budget, as a personal code: a guess bound (fail closed when down)
//	lookup     the GROUP table only — a personal code typed here is never read (ADR 0092 #4)
//	commune    shared bot: FROM the code (crosstenant, exactly one match); commune bot: from Host
//	own bot    a commune that uses its own bot links through IT — the shared bot refuses its code
//	chat live  a chat already live as a group (any commune for the shared bot) is refused; the code is
//	           not consumed — re-linking is "gỡ trên ViGov trước" (ADR 0092 #9, owner #7)
//	one tx     lock the code, check it, mark it used, insert the link with the bot that received it, one
//	           entry whose actor is the code's ISSUER (rule 6 inv 8 — whoever typed it is no ViGov identity)
//
// Every refusal of a code-shaped text is ONE sentence (domain.ZaloReplyGroupCodeRefused). NEVER LOGGED,
// NEVER IN AN ENTRY: chat_id, the text, the code.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/core/ratelimit"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	docstore "github.com/vihat/vigov/service-comms/internal/store"
)

var (
	// errGroupAlreadyLinked rolls a group linking back: the chat became live as a group meanwhile.
	errGroupAlreadyLinked = errors.New("zalo: the group is linked already")
	// errGroupPairedPersonally rolls it back: the chat is paired to a person (0029 zalo_chat_one_link_kind).
	errGroupPairedPersonally = errors.New("zalo: the group chat is paired to a person")
)

// OrgUnitNamer resolves unit names for the group confirmation. *identityclient.Client satisfies it.
type OrgUnitNamer interface {
	OrgUnitNames(ctx context.Context, ids []string) (map[string]identityclient.OrgUnitName, error)
}

// linkedGroup is what a committed linking tells the reply.
type linkedGroup struct {
	orgUnitID string
}

// linkGroupInCommune is one linking inside the commune's transaction — both webhooks.
func linkGroupInCommune(ctx context.Context, tx *store.ScopedTx, groups *docstore.ZaloGroupStore, codeID, chatID,
	botRef, clientIP string, at time.Time, newID func() (string, error)) (linkedGroup, error) {

	c, err := groups.LockOpenGroupPairingCode(ctx, tx, codeID)
	if errors.Is(err, docstore.ErrPairingCodeNotOpen) {
		return linkedGroup{}, errPairingRefused
	}
	if err != nil {
		return linkedGroup{}, err
	}
	if !at.Before(c.ExpiresAt) || c.FailedAttempts >= domain.PairingMaxFailedAttempts {
		return linkedGroup{}, errPairingRefused
	}
	if err := groups.MarkGroupPairingCodeUsed(ctx, tx, c.ID, at); err != nil {
		return linkedGroup{}, err
	}
	linkID, err := newID()
	if err != nil {
		return linkedGroup{}, fmt.Errorf("zalo: new id: %w", err)
	}
	switch err := groups.InsertGroupLink(ctx, tx, linkID, botRef, chatID, c, at); {
	case errors.Is(err, docstore.ErrZaloGroupChatLinked):
		return linkedGroup{}, errGroupAlreadyLinked
	case errors.Is(err, docstore.ErrZaloGroupChatPaired):
		return linkedGroup{}, errGroupPairedPersonally
	case errors.Is(err, docstore.ErrZaloGroupBotNotLive):
		return linkedGroup{}, errPairingRefused
	case err != nil:
		return linkedGroup{}, err
	}
	delta, err := json.Marshal(map[string]any{
		"lien_ket_id": linkID, "ma_gan_nhom_id": c.ID, "bot_ref": botRef, "kenh": "zalo_bot",
		"bo_phan": c.OrgUnitID, "loai": c.Kinds,
	})
	if err != nil {
		return linkedGroup{}, fmt.Errorf("zalo: encode delta: %w", err)
	}
	// The ISSUER is the actor: they hold the account and the permission and chose the scope.
	issuer := audit.Actor{ID: c.IssuedBy, Kind: "staff", IP: clientIP}
	if err := audit.Write(ctx, tx, audit.Entry{Actor: issuer, Action: domain.ActionLinkZaloGroup,
		Subject: domain.ZaloGroupSubject(linkID), At: at, Delta: delta}); err != nil {
		return linkedGroup{}, err
	}
	return linkedGroup{orgUnitID: c.OrgUnitID}, nil
}

// groupLinkReply maps a linking's outcome onto its sentence. ok=false: a system fault, logged by the caller.
func groupLinkReply(ctx context.Context, units OrgUnitNamer, g linkedGroup, err error) (string, bool) {
	switch {
	case errors.Is(err, errPairingRefused):
		return domain.ZaloReplyGroupCodeRefused, true
	case errors.Is(err, errGroupAlreadyLinked):
		return domain.ZaloReplyGroupAlreadyLinked, true
	case errors.Is(err, errGroupPairedPersonally):
		return domain.ZaloReplyGroupPairedPersonally, true
	case err != nil:
		return domain.ZaloReplyUnavailable, false
	}
	return domain.ZaloReplyGroupLinked(g.orgUnitID == "", unitName(ctx, units, g.orgUnitID)), true
}

// unitName is the confirmation's unit name — "" when it cannot be read: the link has committed, and the
// sentence then says "bộ phận đã chọn" rather than a guess.
func unitName(ctx context.Context, units OrgUnitNamer, id string) string {
	if id == "" || units == nil {
		return ""
	}
	names, err := units.OrgUnitNames(ctx, []string{id})
	if err != nil {
		return ""
	}
	return names[id].Name
}

// groupPair is the SHARED bot's group linking. The commune comes FROM the code.
func (w *ZaloWebhook) groupPair(ctx context.Context, chatID, code, clientIP string) string {
	key, err := ratelimit.ZaloChatKey(chatID)
	if err != nil {
		return domain.ZaloReplyGroupCodeRefused
	}
	if reply, ok := w.allowAttempt(ctx, key, clientIP); !ok {
		return reply
	}
	matches, err := w.resolver.OpenGroupPairingCodesByHash(ctx, domain.PairingCodeHash(code))
	if err != nil {
		w.log.ErrorContext(ctx, "gắn nhóm Zalo: không tra được mã gắn nhóm", "err", err)
		return domain.ZaloReplyUnavailable
	}
	if len(matches) != 1 {
		// None, or two communes holding the same live code: NO match (fail closed). A personal code typed
		// here lands here too, unread — the one sentence covers both.
		return domain.ZaloReplyGroupCodeRefused
	}
	m := matches[0]
	cctx := tenant.Into(ctx, m.TenantID)
	// A commune with its own bot links groups through IT (ADR 0079 Q1 #4); 0022's trigger is the floor.
	if _, ownBot, err := w.own.Live(cctx); err != nil {
		w.log.ErrorContext(ctx, "gắn nhóm Zalo: không đọc được bot riêng của xã", "xa", string(m.TenantID), "err", err)
		return domain.ZaloReplyUnavailable
	} else if ownBot {
		return domain.ZaloReplyGroupCodeRefused
	}
	// ADR 0092 #9: live as a group anywhere — refused before the code is touched (not consumed).
	if live, err := w.resolver.GroupChatLiveAnywhere(ctx, domain.SharedZaloBotRef, chatID); err != nil {
		w.log.ErrorContext(ctx, "gắn nhóm Zalo: không kiểm được nhóm đã gắn chưa", "err", err)
		return domain.ZaloReplyUnavailable
	} else if live {
		return domain.ZaloReplyGroupAlreadyLinked
	}
	var g linkedGroup
	err = w.db.For(cctx).Tx(cctx, func(tx *store.ScopedTx) error {
		var err error
		g, err = linkGroupInCommune(cctx, tx, w.groups, m.CodeID, chatID, domain.SharedZaloBotRef, clientIP, w.clock(), w.newID)
		return err
	})
	reply, ok := groupLinkReply(cctx, w.units, g, err)
	if !ok {
		w.log.ErrorContext(ctx, "gắn nhóm Zalo: lỗi hệ thống", "xa", string(m.TenantID), "err", err)
	}
	return reply
}

// allowAttempt spends one try of the chat's pairing budget. FAIL CLOSED when the counter is down.
func (w *ZaloWebhook) allowAttempt(ctx context.Context, key ratelimit.Key, clientIP string) (string, bool) {
	allowed, _, err := w.limiter.Allow(ctx, key)
	if err != nil {
		w.log.WarnContext(ctx, "CẢNH BÁO HẠ TẦNG: không đếm được giới hạn ghép nối Zalo — từ chối (đóng kín)", "err", err)
		return domain.ZaloReplyUnavailable, false
	}
	if !allowed {
		w.log.WarnContext(ctx, "CẢNH BÁO BẢO MẬT: một chat Zalo vượt giới hạn thử mã ghép nối — từ chối",
			"event", "zalo_bot_pairing.rate_limited", "outcome", "refused", "ip", clientIP)
		return domain.ZaloReplyPairingLimited, false
	}
	return "", true
}

// groupPair is a COMMUNE bot's group linking: the commune is Host's, the bot the authenticated one.
func (w *CommuneZaloWebhook) groupPair(ctx context.Context, bot domain.CommuneZaloBot, chatID, code, clientIP string) string {
	key, err := ratelimit.ZaloCommuneChatKey(ctx, bot.Ref(), chatID)
	if err != nil {
		return domain.ZaloReplyGroupCodeRefused
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
	// THIS commune's open group code, by hash — never another commune's.
	codeID, found, err := w.groups.OpenGroupPairingCodeByHash(ctx, domain.PairingCodeHash(code))
	if err != nil {
		w.log.ErrorContext(ctx, "gắn nhóm Zalo (bot riêng): không tra được mã gắn nhóm", "err", err)
		return domain.ZaloReplyUnavailable
	}
	if !found {
		return domain.ZaloReplyGroupCodeRefused
	}
	// A commune bot's group links live in its commune alone (0022 trigger): the scoped read is the whole answer.
	if live, err := w.groups.LiveGroupLinkOfChat(ctx, bot.Ref(), chatID); err != nil {
		w.log.ErrorContext(ctx, "gắn nhóm Zalo (bot riêng): không kiểm được nhóm đã gắn chưa", "err", err)
		return domain.ZaloReplyUnavailable
	} else if live {
		return domain.ZaloReplyGroupAlreadyLinked
	}
	var g linkedGroup
	err = w.db.For(ctx).Tx(ctx, func(tx *store.ScopedTx) error {
		var err error
		g, err = linkGroupInCommune(ctx, tx, w.groups, codeID, chatID, bot.Ref(), clientIP, w.clock(), w.newID)
		return err
	})
	reply, ok := groupLinkReply(ctx, w.units, g, err)
	if !ok {
		w.log.ErrorContext(ctx, "gắn nhóm Zalo (bot riêng): lỗi hệ thống", "err", err)
	}
	return reply
}
