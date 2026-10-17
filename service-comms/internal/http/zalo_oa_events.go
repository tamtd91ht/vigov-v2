package http

// THE ZALO OA WEBHOOK — POST /api/v1/zalo-oa-events (owner decision 17/10/2026, TASK W-1). Zalo posts
// every event of every OA authorised to the platform's shared app here; comms verifies it and forwards the
// whole body to Kafka (internal/app/zalo_oa_events.go). No session, no commune Host: like the
// authorization callback it lives on the platform's callback host (ZALO_OA_CALLBACK_HOST), on the SAME
// chain WITHOUT TenantMiddleware. The commune comes from the event's OA, never from Host, a header or the
// query.
//
// THE ORDER, AND IT IS THE SECURITY OF THE ROUTE:
//
//	1. rate limit (ratelimit.ZaloOAWebhook, 600/min per network)  BEFORE the body is read (rule 13 #7)
//	2. the body, capped at maxZaloOAEventBytes                     the signature covers the body, so it
//	                                                               must be read before it can be checked
//	3. X-ZEvent-Signature + app_id                                 wrong/missing → 403, nothing published
//	4. the commune of the OA                                       none → 200, nothing published
//	5. publish, synchronously                                      200 only AFTER the broker acknowledged;
//	                                                               broker or lookup error → 503 (Zalo retries)
//
// NEVER LOGGED: the body, the header, the OA id, user ids, message text (rule 3). Only the event name,
// shape-checked, and the commune id once resolved.

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/ratelimit"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// ZaloOAEventsPath is the webhook's path. Exported because cmd/server routes the callback HOST's copy of
// this path to the callback chain; the route below spells it as a LITERAL for tools/apidoc, and
// zalo_oa_events_test.go asserts the two agree.
const ZaloOAEventsPath = "/api/v1/zalo-oa-events"

// maxZaloOAEventBytes caps the body read. A Zalo event is a few kilobytes; 1 MiB is the owner's bound.
const maxZaloOAEventBytes = 1 << 20

// zaloOAEventBudget bounds one event: one indexed lookup and one synchronous publish (whose own delivery
// timeout is core/events/kafka.DeliveryTimeout).
const zaloOAEventBudget = 15 * time.Second

// ZaloOAEventReceiver is the use case. *app.ZaloOAEvents satisfies it.
type ZaloOAEventReceiver interface {
	Receive(ctx context.Context, signature string, body []byte) (app.ZaloOAEventResult, error)
}

// ZaloOAEventDeps is everything the webhook touches.
type ZaloOAEventDeps struct {
	Events ZaloOAEventReceiver
	// Limiter is ratelimit.ZaloOAWebhook — REQUIRED (rule 13, invariant 7).
	Limiter *ratelimit.Limiter
	Log     *slog.Logger
}

type zaloOAEventHandler struct{ d ZaloOAEventDeps }

// zaloOAEventAck is what Zalo is answered with on 200.
type zaloOAEventAck struct {
	OK bool `json:"ok"`
}

// RegisterZaloOAEvents mounts the webhook onto the callback chain's mux (cmd/server: muxZaloOA).
func RegisterZaloOAEvents(mux *http.ServeMux, d ZaloOAEventDeps) {
	switch {
	case d.Events == nil:
		panic("comms/http: thiếu use case sự kiện OA Zalo — POST /api/v1/zalo-oa-events sẽ panic khi Zalo gọi")
	case d.Limiter == nil:
		// Rule 13 invariant 7: an unauthenticated route without its rate limit is refused at startup.
		panic("comms/http: thiếu bộ giới hạn tần suất webhook OA Zalo (ratelimit.ZaloOAWebhook)")
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	h := &zaloOAEventHandler{d: d}

	// PUBLIC — the caller is Zalo's server; it holds no ViGov session and the host is the platform's, not a
	// commune's. Authenticated instead by X-ZEvent-Signature (sha256 over app_id + body + timestamp + the
	// app's webhook OA Secret Key), compared in constant time, plus app_id = ZALO_OA_APP_ID.
	//
	// RATE LIMIT — ratelimit.ZaloOAWebhook, 600 per minute per client network (owner, 17/10/2026), counted
	// before the body is read. Over it: 429 + Retry-After. Redis unavailable: 503 (closed).
	//
	// idem.KhongCan: Zalo sends no Idempotency-Key and retries what it saw fail; a retry is published again
	// under a new envelope id — consumers deduplicate on the payload (app/zalo_oa_events.go).
	//
	//	200   published (broker acknowledged), or authenticated but naming no live OA (nothing published)
	//	403   signature missing or wrong, body unreadable, or another app's event — nothing published
	//	413   body over 1 MiB — nothing published
	//	429   the rate limit
	//	503   not configured (dev), the commune lookup or the broker failed, or the counter store is down
	//
	// @summary  Webhook sự kiện OA Zalo — Zalo gửi mọi sự kiện của OA các xã đã uỷ quyền cho app dùng chung; kiểm chữ ký X-ZEvent-Signature (so thời gian hằng) và app_id, tìm xã đang giữ OA ấy, rồi chuyển NGUYÊN thân sự kiện lên Kafka (comms.zalo_oa_event_received.v1, khoá = xã); OA không thuộc xã nào thì trả 200 và không chuyển gì
	// @screen   Chủ dự án chốt 17/10/2026 — TASK W-1 (ADR 0098: app Zalo dùng chung)
	// @reply    200 zaloOAEventAck
	// @reply    403 httpx.Error
	// @reply    413 httpx.Error
	// @reply    429 httpx.Error rate_limited
	// @reply    503 httpx.Error
	mux.Handle("POST /api/v1/zalo-oa-events",
		authz.Public("Zalo gọi webhook khi có sự kiện ở OA của một xã, không có phiên và không có Host của xã: xác thực bằng chữ ký X-ZEvent-Signature (sha256 của app_id + thân + timestamp + OA Secret Key của app dùng chung) so thời gian hằng cùng app_id của nền tảng, sai trả 403 và không chuyển gì; xã lấy từ OA đang hoạt động trong sự kiện, không từ Host; giới hạn 600 lượt/phút mỗi mạng máy gọi trước khi đọc thân")(
			idem.KhongCan("Zalo không gửi Idempotency-Key và gửi lại khi thấy lỗi; lần gửi lại được chuyển lên Kafka với mã phong bì mới, bên nhận chống trùng theo định danh trong thân sự kiện")(
				http.HandlerFunc(h.ReceiveZaloOAEvent))))
}

// ReceiveZaloOAEvent — POST /api/v1/zalo-oa-events
func (h *zaloOAEventHandler) ReceiveZaloOAEvent(w http.ResponseWriter, r *http.Request) {
	ip := httpx.ClientIP(r)
	if !ratelimit.Gate(w, r, h.d.Limiter, ratelimit.WebhookIPKey(ip), h.d.Log) {
		return
	}
	// Only TenantMiddleware puts a commune in the context, and the callback chain has none. A commune here
	// means the route was mounted behind a commune Host — refuse rather than let Host and the event's OA
	// disagree about which commune this is (rule 1).
	if _, ok := tenant.From(r.Context()); ok {
		h.d.Log.Error("webhook OA Zalo chạy trong chuỗi có xã — sai dây, từ chối", "event", "zalo_oa_webhook.miswired")
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy.", "")
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxZaloOAEventBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			h.d.Log.Warn("webhook OA Zalo: thân vượt 1 MiB — từ chối, không chuyển", "event", "zalo_oa_webhook.too_large", "ip", ip)
			httpx.WriteError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "Nội dung quá lớn.", "")
			return
		}
		// The connection broke mid-body: nothing to verify, nothing published; Zalo retries.
		h.d.Log.Warn("webhook OA Zalo: đọc thân dở — không chuyển", "event", "zalo_oa_webhook.read_failed", "ip", ip)
		httpx.WriteError(w, http.StatusServiceUnavailable, "unavailable",
			"Hệ thống tạm thời không xử lý được yêu cầu. Vui lòng thử lại sau.", "")
		return
	}

	// Detached from the request: Zalo hanging up must not cancel a publish half-way (it retries anyway).
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), zaloOAEventBudget)
	res, err := h.d.Events.Receive(ctx, r.Header.Get(domain.ZaloOAEventSignatureHeader), body)
	cancel()
	clear(body)

	switch {
	case errors.Is(err, app.ErrZaloOAEventForbidden):
		// A security event: somebody reached the webhook without a valid signature. The address only.
		h.d.Log.Warn("CẢNH BÁO BẢO MẬT: webhook OA Zalo sai hoặc thiếu chữ ký, hoặc không phải app của nền tảng — từ chối, không chuyển",
			"event", "zalo_oa_webhook.signature_refused", "outcome", "refused", "ip", ip)
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "Không có quyền.", "")
		return
	case errors.Is(err, app.ErrZaloOAEventsNotConfigured):
		h.d.Log.Warn("CẢNH BÁO CẤU HÌNH: sự kiện OA Zalo tới khi ZALO_OA_WEBHOOK_SECRET / ZALO_OA_APP_ID / KAFKA_EVENT_ADDRESS chưa đặt — 503",
			"event", "zalo_oa_webhook.not_configured")
		httpx.WriteError(w, http.StatusServiceUnavailable, "unavailable",
			"Hệ thống tạm thời không xử lý được yêu cầu. Vui lòng thử lại sau.", "")
		return
	case err != nil:
		// The lookup or the broker. err names the topic or the query, never the body (core/events/kafka).
		h.d.Log.Error("webhook OA Zalo: không chuyển được sự kiện — 503, Zalo sẽ gửi lại",
			"event", "zalo_oa_webhook.unavailable", "event_name", res.EventName, "xa", string(res.Tenant), "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "unavailable",
			"Hệ thống tạm thời không xử lý được yêu cầu. Vui lòng thử lại sau.", "")
		return
	}

	if res.Outcome != app.ZaloOAEventPublished {
		// Authenticated, but no live OA in any commune: answered 200 so Zalo stops retrying, published
		// NOTHING (fail closed). The event name only — never the OA id or the body.
		h.d.Log.Info("webhook OA Zalo: sự kiện không thuộc OA đang hoạt động của xã nào — không chuyển",
			"event", "zalo_oa_webhook.ignored", "event_name", res.EventName, "reason", string(res.Outcome))
	}
	vietJSON(w, http.StatusOK, zaloOAEventAck{OK: true})
}
