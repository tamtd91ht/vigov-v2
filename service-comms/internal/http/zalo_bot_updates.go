package http

// THE ZALO BOT WEBHOOK — POST /api/v1/zalo-bot-updates (ADR 0074 #5; the owner-decided noun, after Zalo's
// own "update"). Zalo calls it; no person does. ONE ROUTE, TWO KINDS OF HOST:
//
//	ZALO_BOT_WEBHOOK_HOST   the SHARED bot (ADR 0074). No commune on this host: cmd/server mounts the
//	                        chain WITHOUT TenantMiddleware, and the commune of an update is found from the
//	                        pairing code or the link it concerns (internal/app/zalo_webhook.go).
//	a commune's own domain  that commune's OWN bot (ADR 0079 Q1 #2; migration 0022). cmd/server mounts the
//	                        SAME mux behind TenantMiddleware, so the commune comes from Host (rule 1,
//	                        invariant 3; unknown Host = 404 before this handler) and nothing crosses to
//	                        another commune (internal/app/zalo_commune_webhook.go).
//
// The handler tells the two apart by whether the context CARRIES a commune — which only TenantMiddleware
// can put there: both chains strip client tenant headers. Neither branch falls back to the other.
//
// No staff session on either: Zalo has none.
//
// THE ORDER, AND IT IS THE SECURITY OF THE ROUTE:
//
//	1. rate limit (ratelimit.ZaloBotWebhook)              BEFORE the secret is looked at: it bounds how
//	                                                      fast the secret can be guessed (rule 13 #7). Per
//	                                                      client network on the shared host; per (commune,
//	                                                      host, network) on a commune's host
//	2. X-Bot-Api-Secret-Token, compared in constant time  against the secret in force AND the pending one —
//	                                                      the shared bot's, or THIS commune's own bot's;
//	                                                      missing or wrong → 403 and THE BODY IS NOT READ
//	3. the body, capped at maxZaloUpdateBytes             then zalobot.ParseUpdate (flat or wrapped)
//	4. 200 to Zalo                                        for EVERY authenticated update — ignored events,
//	                                                      malformed payloads, refused codes included:
//	                                                      a non-2xx only makes Zalo redeliver it
//	5. the reply to the chat                              after the answer, never before (app.Reply)
//
// NEVER LOGGED: the header, the body, the chat id, the text (rule 3; ADR 0074).

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/ratelimit"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/zalobot"
)

// ZaloBotUpdatesPath is the webhook's path. Exported because cmd/server routes the webhook HOST's copy
// of this path to the webhook chain; the route below spells it as a LITERAL for tools/apidoc, and
// zalo_bot_updates_test.go asserts the two agree (and agree with domain.ZaloWebhookPath, the URL comms
// registers with Zalo).
const ZaloBotUpdatesPath = "/api/v1/zalo-bot-updates"

// ZaloSecretHeader is the header Zalo signs every update with (the secret_token given to setWebhook).
const ZaloSecretHeader = "X-Bot-Api-Secret-Token"

// maxZaloUpdateBytes caps the body read AFTER authentication. An update is a few hundred bytes.
const maxZaloUpdateBytes = 64 << 10

// zaloUpdateBudget bounds the work done for one update. Zalo is answered when it ends, whatever the
// outcome; the request's own context is not used, so Zalo hanging up does not roll back a pairing
// half-way through its transaction.
const zaloUpdateBudget = 10 * time.Second

// ZaloBotUpdates is the webhook's use case. *app.ZaloWebhook satisfies it.
type ZaloBotUpdates interface {
	Authenticate(ctx context.Context, presented string) (bool, error)
	Handle(ctx context.Context, u zalobot.Update, clientIP string) string
	Reply(chatID, text string)
}

// CommuneZaloBotUpdates is the commune bot's use case. *app.CommuneZaloWebhook satisfies it. Every method
// reads the commune from ctx. Authenticate returns the context carrying the bot whose secret matched;
// Handle and Reply MUST be given that context, so the update is acted on through the bot Zalo called —
// never one re-read after a concurrent replace. Reply detaches ctx from the request itself.
type CommuneZaloBotUpdates interface {
	Authenticate(ctx context.Context, presented string) (context.Context, bool, error)
	Handle(ctx context.Context, u zalobot.Update, clientIP string) string
	Reply(ctx context.Context, chatID, text string)
}

// ZaloBotUpdateDeps is everything the webhook touches.
type ZaloBotUpdateDeps struct {
	// Updates is the SHARED bot's use case (no commune in ctx).
	Updates ZaloBotUpdates
	// CommuneUpdates is a commune's OWN bot's use case (commune from Host).
	CommuneUpdates CommuneZaloBotUpdates
	// Limiter is ratelimit.ZaloBotWebhook — REQUIRED (rule 13, invariant 7).
	Limiter *ratelimit.Limiter
	Log     *slog.Logger
}

type zaloBotUpdateHandler struct{ d ZaloBotUpdateDeps }

// zaloUpdateAck is what Zalo is answered with once authenticated.
type zaloUpdateAck struct {
	OK bool `json:"ok"`
}

// RegisterZaloBotUpdates mounts the webhook onto ITS OWN mux.
func RegisterZaloBotUpdates(mux *http.ServeMux, d ZaloBotUpdateDeps) {
	switch {
	case d.Updates == nil:
		panic("comms/http: thiếu use case webhook Zalo Bot — POST /api/v1/zalo-bot-updates sẽ panic khi Zalo gọi")
	case d.CommuneUpdates == nil:
		panic("comms/http: thiếu use case webhook bot riêng của xã — POST /api/v1/zalo-bot-updates trên tên miền xã sẽ panic")
	case d.Limiter == nil:
		// Rule 13 invariant 7: an unauthenticated route without its rate limit is refused at startup.
		panic("comms/http: thiếu bộ giới hạn tần suất webhook Zalo Bot (ratelimit.ZaloBotWebhook)")
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	h := &zaloBotUpdateHandler{d: d}

	// PUBLIC — no staff session: the caller is Zalo's server. Authenticated instead by the webhook secret
	// (ADR 0074 #5; ADR 0079 Q1 #2 for a commune's own bot), compared in constant time BEFORE the body is
	// read. On the shared host there is no commune; on a commune's host the commune comes from Host.
	//
	// RATE LIMIT — ratelimit.ZaloBotWebhook, counted before the secret is looked at: per client network on
	// the shared host, per (commune, host, client network) on a commune's host. Over it: 429 +
	// Retry-After. Redis unavailable: 503 (closed). ITS NUMBER IS PROVISIONAL — a rule 13 stop condition
	// the owner has not answered (core/ratelimit ZaloBotWebhookLimit).
	//
	// idem.KhongCan: Zalo may redeliver an update, and sends no Idempotency-Key; every act behind it is
	// idempotent by its own state — a used code refuses, an ended link stays ended, help is help.
	//
	//	200   every authenticated update, acted on or not
	//	403   the secret header missing or wrong — the body was not read
	//	429   the rate limit
	//	503   the secrets could not be read, or the counter store is down
	//
	// @summary  Webhook của Zalo Bot — Zalo gọi khi cán bộ nhắn cho bot (mã ghép nối, /trogiup, /dung): bot dùng chung trên tên miền webhook của nền tảng, bot riêng của xã trên tên miền của xã (xã lấy từ Host); xác thực bằng X-Bot-Api-Secret-Token so thời gian hằng, sai khoá 403 không đọc thân
	// @screen   ADR 0074 #5 · ADR 0079 Q1 #2
	// @reply    200 zaloUpdateAck
	// @reply    403 httpx.Error
	// @reply    429 httpx.Error rate_limited
	// @reply    503 httpx.Error
	mux.Handle("POST /api/v1/zalo-bot-updates",
		authz.Public("Zalo gọi webhook khi cán bộ nhắn cho bot, không có phiên: bot dùng chung trên tên miền webhook của nền tảng, không có xã (ADR 0074 #5); bot riêng của xã trên chính tên miền của xã, xã lấy từ Host và không đọc chéo xã (ADR 0079 Q1 #2); xác thực bằng khoá X-Bot-Api-Secret-Token so thời gian hằng với khoá đang dùng và khoá chờ của đúng con bot ấy, sai khoá trả 403 không đọc thân; giới hạn tần suất theo mạng của máy gọi (kèm xã và tên miền khi có xã) trước khi xem khoá")(
			idem.KhongCan("Zalo có thể gửi lại một gói và không gửi Idempotency-Key; mỗi việc phía sau tự bất biến theo trạng thái: mã đã dùng thì từ chối, liên kết đã kết thúc thì vẫn kết thúc, trợ giúp vẫn là trợ giúp")(
				http.HandlerFunc(h.ReceiveZaloBotUpdate))))
}

// ReceiveZaloBotUpdate — POST /api/v1/zalo-bot-updates
func (h *zaloBotUpdateHandler) ReceiveZaloBotUpdate(w http.ResponseWriter, r *http.Request) {
	// A commune in the context was put there by TenantMiddleware from Host: this is a commune's own bot.
	if _, ok := tenant.From(r.Context()); ok {
		h.receiveCommuneUpdate(w, r)
		return
	}
	ip := httpx.ClientIP(r)
	if !ratelimit.Gate(w, r, h.d.Limiter, ratelimit.WebhookIPKey(ip), h.d.Log) {
		return
	}
	// The header, BEFORE the body. Never logged.
	ok, err := h.d.Updates.Authenticate(r.Context(), r.Header.Get(ZaloSecretHeader))
	if err != nil {
		h.d.Log.Warn("CẢNH BÁO: webhook Zalo Bot không kiểm được khoá — trả 503, không đọc thân", "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "unavailable",
			"Hệ thống tạm thời không xử lý được yêu cầu. Vui lòng thử lại sau.", "")
		return
	}
	if !ok {
		// A security event: somebody reached the webhook without the secret. The address the edge saw,
		// nothing else.
		h.d.Log.Warn("CẢNH BÁO BẢO MẬT: webhook Zalo Bot sai hoặc thiếu khoá — từ chối, không đọc thân",
			"event", "zalo_bot_webhook.secret_refused", "outcome", "refused", "ip", ip)
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "Không có quyền.", "")
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxZaloUpdateBytes))
	if err != nil {
		h.d.Log.Warn("webhook Zalo Bot: thân quá lớn hoặc đọc dở — bỏ gói")
		vietJSON(w, http.StatusOK, zaloUpdateAck{OK: true})
		return
	}
	u, err := zalobot.ParseUpdate(body)
	clear(body)
	if err != nil {
		// zalobot.ErrMalformedUpdate carries nothing of the payload.
		h.d.Log.Warn("webhook Zalo Bot: gói không đọc được — bỏ gói", "err", err)
		vietJSON(w, http.StatusOK, zaloUpdateAck{OK: true})
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), zaloUpdateBudget)
	reply := h.d.Updates.Handle(ctx, u, ip)
	cancel()
	vietJSON(w, http.StatusOK, zaloUpdateAck{OK: true})
	h.d.Updates.Reply(u.ChatID, reply)
}

// receiveCommuneUpdate is the same five steps for a commune's own bot, in the commune Host resolved.
func (h *zaloBotUpdateHandler) receiveCommuneUpdate(w http.ResponseWriter, r *http.Request) {
	ip := httpx.ClientIP(r)
	// Keyed by (commune, host, client network): one commune's webhook traffic never spends another's
	// budget. The host has already resolved to this commune (TenantMiddleware); the port is dropped so
	// two spellings of one name share a counter.
	host := r.Host
	if name, _, err := net.SplitHostPort(host); err == nil {
		host = name
	}
	key, err := ratelimit.PublicHostIPKey(r.Context(), true, host, ip)
	if err != nil {
		// No commune for a commune-scoped key: a wiring fault on the isolation path — refuse, never an
		// unscoped key.
		h.d.Log.Error("webhook bot riêng của xã: không dựng được khoá giới hạn tần suất theo xã", "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "unavailable",
			"Hệ thống tạm thời không xử lý được yêu cầu. Vui lòng thử lại sau.", "")
		return
	}
	if !ratelimit.Gate(w, r, h.d.Limiter, key, h.d.Log) {
		return
	}
	xa := string(tenant.MustFrom(r.Context()))
	// The header, BEFORE the body. Never logged.
	authCtx, ok, err := h.d.CommuneUpdates.Authenticate(r.Context(), r.Header.Get(ZaloSecretHeader))
	if err != nil {
		h.d.Log.Warn("CẢNH BÁO: webhook bot riêng của xã không kiểm được khoá — trả 503, không đọc thân", "xa", xa, "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "unavailable",
			"Hệ thống tạm thời không xử lý được yêu cầu. Vui lòng thử lại sau.", "")
		return
	}
	if !ok {
		h.d.Log.Warn("CẢNH BÁO BẢO MẬT: webhook bot riêng của xã sai hoặc thiếu khoá — từ chối, không đọc thân",
			"event", "zalo_commune_bot_webhook.secret_refused", "outcome", "refused", "xa", xa, "ip", ip)
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "Không có quyền.", "")
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxZaloUpdateBytes))
	if err != nil {
		h.d.Log.Warn("webhook bot riêng của xã: thân quá lớn hoặc đọc dở — bỏ gói", "xa", xa)
		vietJSON(w, http.StatusOK, zaloUpdateAck{OK: true})
		return
	}
	u, err := zalobot.ParseUpdate(body)
	clear(body)
	if err != nil {
		h.d.Log.Warn("webhook bot riêng của xã: gói không đọc được — bỏ gói", "xa", xa, "err", err)
		vietJSON(w, http.StatusOK, zaloUpdateAck{OK: true})
		return
	}
	// Detached from the request (Zalo hanging up must not roll back a pairing half-way) but carrying the
	// commune AND the authenticated bot: context.WithoutCancel keeps the values.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(authCtx), zaloUpdateBudget)
	reply := h.d.CommuneUpdates.Handle(ctx, u, ip)
	cancel()
	vietJSON(w, http.StatusOK, zaloUpdateAck{OK: true})
	h.d.CommuneUpdates.Reply(authCtx, u.ChatID, reply)
}
