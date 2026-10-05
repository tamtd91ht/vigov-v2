package http

// THE SHARED ZALO BOT'S WEBHOOK — POST /api/v1/zalo-bot-updates (ADR 0074 #5; the owner-decided noun,
// after Zalo's own "update"). Zalo calls it; no person does.
//
// ITS OWN MUX AND ITS OWN EDGE CHAIN (cmd/server dungBienZaloBot): no TenantMiddleware — the webhook host
// (ZALO_BOT_WEBHOOK_HOST, bot.api.vigov.vn) maps to no commune — and no staff session. cmd/server routes
// ONLY that host's path here; any other Host never reaches this handler. The commune of an update is
// found from the pairing code or the link it concerns (internal/app/zalo_webhook.go), never from a header.
//
// THE ORDER, AND IT IS THE SECURITY OF THE ROUTE:
//
//	1. per-network rate limit (ratelimit.ZaloBotWebhook)  BEFORE the secret is looked at: it bounds how
//	                                                      fast the secret can be guessed (rule 13 #7)
//	2. X-Bot-Api-Secret-Token, compared in constant time  against the secret in force AND the pending one;
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
	"net/http"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/ratelimit"
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

// ZaloBotUpdateDeps is everything the webhook touches.
type ZaloBotUpdateDeps struct {
	Updates ZaloBotUpdates
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
	case d.Limiter == nil:
		// Rule 13 invariant 7: an unauthenticated route without its rate limit is refused at startup.
		panic("comms/http: thiếu bộ giới hạn tần suất webhook Zalo Bot (ratelimit.ZaloBotWebhook)")
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	h := &zaloBotUpdateHandler{d: d}

	// PUBLIC — no staff session and no commune: the caller is Zalo's server. Authenticated instead by the
	// webhook secret (ADR 0074 #5), compared in constant time BEFORE the body is read.
	//
	// RATE LIMIT — ratelimit.ZaloBotWebhook, per client network, counted before the secret is looked at.
	// Over it: 429 + Retry-After. Redis unavailable: 503 (closed). ITS NUMBER IS PROVISIONAL — a rule 13
	// stop condition the owner has not answered (core/ratelimit ZaloBotWebhookLimit).
	//
	// idem.KhongCan: Zalo may redeliver an update, and sends no Idempotency-Key; every act behind it is
	// idempotent by its own state — a used code refuses, an ended link stays ended, help is help.
	//
	//	200   every authenticated update, acted on or not
	//	403   the secret header missing or wrong — the body was not read
	//	429   the rate limit
	//	503   the secrets could not be read, or the counter store is down
	//
	// @summary  Webhook của Zalo Bot dùng chung — Zalo gọi khi cán bộ nhắn cho bot (mã ghép nối, /trogiup, /dung); xác thực bằng X-Bot-Api-Secret-Token so thời gian hằng, sai khoá 403 không đọc thân
	// @screen   ADR 0074 #5
	// @reply    200 zaloUpdateAck
	// @reply    403 httpx.Error
	// @reply    429 httpx.Error rate_limited
	// @reply    503 httpx.Error
	mux.Handle("POST /api/v1/zalo-bot-updates",
		authz.Public("Zalo gọi webhook của bot dùng chung khi cán bộ nhắn cho bot (ADR 0074 #5): không có phiên và không có xã ở tên miền này; xác thực bằng khoá X-Bot-Api-Secret-Token so thời gian hằng với khoá đang dùng và khoá chờ, sai khoá trả 403 không đọc thân; giới hạn tần suất theo mạng của máy gọi trước khi xem khoá")(
			idem.KhongCan("Zalo có thể gửi lại một gói và không gửi Idempotency-Key; mỗi việc phía sau tự bất biến theo trạng thái: mã đã dùng thì từ chối, liên kết đã kết thúc thì vẫn kết thúc, trợ giúp vẫn là trợ giúp")(
				http.HandlerFunc(h.ReceiveZaloBotUpdate))))
}

// ReceiveZaloBotUpdate — POST /api/v1/zalo-bot-updates
func (h *zaloBotUpdateHandler) ReceiveZaloBotUpdate(w http.ResponseWriter, r *http.Request) {
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
