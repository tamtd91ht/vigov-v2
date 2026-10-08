package http

// A COMMUNE'S OWN ZALO BOT (ADR 0079 Q1 #5, "zalo-bots/current … song song zalo-bots/shared"; migration
// 0022) — spec Cấu hình 11 §3 "Con bot của xã":
//
//	GET    /api/v1/zalo-bots/current           admin.lookup  the live own bot, or has_own_bot false
//	PUT    /api/v1/zalo-bots/current           admin.lookup  "Lưu con bot" — adopt, replace, or edit it
//	POST   /api/v1/zalo-bots/current/check     admin.lookup  "Kiểm tra kết nối" — getMe, result recorded
//	POST   /api/v1/zalo-bots/current/webhook   admin.lookup  "Đăng ký webhook" — the secret shown ONCE
//	DELETE /api/v1/zalo-bots/current           admin.lookup  "Quay về bot chung" — retire, with a reason
//
// `admin.lookup` is the key ADR 0074 #6 names for the Kênh Zalo tab (seeded at
// service-identity/migrations/0001_init.sql:281) — the same as zalo-channel-settings and mail-settings.
//
// NOTHING SECRET LEAVES HERE except the webhook secret, once, in the reply of the call that generated it
// (ADR 0079 Q1 #3), with Cache-Control: no-store. The token is write-only: no type below has a field for
// it on the way out. No reply names another commune — a bot account taken elsewhere is one sentence.

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/crypto"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// CommuneZaloBotAdmin is the use case. *app.CommuneZaloBots satisfies it. Every write opens its own
// transaction and writes its audit entry inside it (rule 6, invariant 3).
type CommuneZaloBotAdmin interface {
	Current(ctx context.Context) (app.CommuneZaloBotView, error)
	Set(ctx context.Context, in app.SetCommuneZaloBotInput, actor audit.Actor) (app.SetCommuneZaloBotOutput, error)
	Check(ctx context.Context, actor audit.Actor) (app.CheckOutput, error)
	RegisterWebhook(ctx context.Context, actor audit.Actor) (app.CommuneWebhookOutput, error)
	Retire(ctx context.Context, reason string, actor audit.Actor) (app.RetireCommuneZaloBotOutput, error)
}

// ZaloCommuneBotDeps is everything the five routes touch.
type ZaloCommuneBotDeps struct {
	Checker authz.Checker
	Bots    CommuneZaloBotAdmin
	Log     *slog.Logger
}

type zaloCommuneBotHandler struct{ d ZaloCommuneBotDeps }

// communeZaloBotOut is the own bot as the screen reads it. No token, no secret: `webhook_set_at` /
// `webhook_set_by` say THAT a secret is in force and since when, never what it is.
type communeZaloBotOut struct {
	BotAccountID string    `json:"bot_account_id"`
	BotName      string    `json:"bot_name"`
	ChatURL      string    `json:"chat_url"`
	SetAt        time.Time `json:"set_at"`
	SetBy        string    `json:"set_by"`
	// WebhookSetAt / WebhookSetBy: null until a webhook registration was confirmed by Zalo.
	WebhookSetAt *time.Time `json:"webhook_set_at"`
	WebhookSetBy *string    `json:"webhook_set_by"`
	// WebhookPending: a registration is unconfirmed — Zalo answered ambiguously; "Đăng ký webhook" again.
	WebhookPending bool `json:"webhook_pending"`
	// LastCheck: null until the first "Kiểm tra kết nối" (a save records one: the token was just checked).
	LastCheck *zaloBotCheckOut `json:"last_check"`
}

// zaloBotCheckOut — `result` is 0018's ZaloCallOutcome value (`thanh-cong`, `token-bi-tu-choi`, …).
type zaloBotCheckOut struct {
	At     time.Time `json:"at"`
	Result string    `json:"result"`
}

// communeZaloBotCurrentOut is GET zalo-bots/current.
type communeZaloBotCurrentOut struct {
	// HasOwnBot false: the commune uses the shared bot, and `bot` is null.
	HasOwnBot bool               `json:"has_own_bot"`
	Bot       *communeZaloBotOut `json:"bot"`
	// LiveLinkCount is how many staff are paired through the bot serving the commune now — the number the
	// confirmation of a switch shows (ADR 0079 Q1 #4): saving another bot, or going back to the shared
	// one, ends every one of these links.
	LiveLinkCount int `json:"live_link_count"`
}

// communeZaloBotIn is the body of PUT. `bot_token` IS WRITE-ONLY: omitted or "" keeps the live bot's
// token (refused when the commune has no own bot yet). The bot's account id is never sent: it is what
// Zalo answers for the token.
type communeZaloBotIn struct {
	BotToken string `json:"bot_token,omitempty"`
	BotName  string `json:"bot_name"`
	ChatURL  string `json:"chat_url"`
}

// communeZaloBotSavedOut is the reply of PUT.
type communeZaloBotSavedOut struct {
	Bot communeZaloBotOut `json:"bot"`
	// Adopted: a new bot is now live (from the shared bot, or replacing another own bot).
	Adopted bool `json:"adopted"`
	// EndedLinkCount: staff whose pairing ended with the switch — they pair again (ADR 0079 Q1 #4).
	EndedLinkCount int `json:"ended_link_count"`
	// RevokeNotice is set when another own bot was retired: the sentence telling the commune that the old
	// bot's credential is still valid at Zalo and must be revoked there (0022 "OWED BY GO").
	RevokeNotice string `json:"revoke_notice,omitempty"`
}

// zaloBotCheckResultOut is the reply of POST check. `account_name` only when `result` is `thanh-cong`.
type zaloBotCheckResultOut struct {
	Result      string    `json:"result"`
	CheckedAt   time.Time `json:"checked_at"`
	AccountName string    `json:"account_name,omitempty"`
}

// zaloBotWebhookOut is the reply of POST webhook. `secret` APPEARS ONCE — in the reply of the call that
// generated it, when the webhook now accepts it — for a commune that registers the webhook by hand
// (spec 11 §3). It is never readable again.
type zaloBotWebhookOut struct {
	Result string     `json:"result"`
	URL    string     `json:"url"`
	SetAt  *time.Time `json:"set_at,omitempty"`
	SetBy  string     `json:"set_by,omitempty"`
	Secret string     `json:"secret,omitempty" apidoc:"bi-mat-co-chu-y:ADR 0079 Q1 #3 — chủ dự án chốt 08/10/2026: secret webhook do hệ thống sinh, hiện MỘT lần trong câu trả lời của lần sinh ra nó để xã tự đăng ký webhook bằng tay nếu cần; không bao giờ đọc lại được"`
}

// communeZaloBotRetireIn is the body of DELETE — rule 7: a soft delete carries its reason.
type communeZaloBotRetireIn struct {
	Reason string `json:"reason"`
}

// communeZaloBotRetiredOut is the reply of DELETE. `retired` false: the commune had no own bot; nothing
// was written.
type communeZaloBotRetiredOut struct {
	Retired        bool `json:"retired"`
	EndedLinkCount int  `json:"ended_link_count"`
	// RevokeNotice: the retired bot's credential is still valid at Zalo — revoke it there.
	RevokeNotice string `json:"revoke_notice,omitempty"`
}

func communeBotToOut(b domain.CommuneZaloBot) communeZaloBotOut {
	out := communeZaloBotOut{BotAccountID: b.BotAccountID, BotName: b.BotName, ChatURL: b.ChatURL,
		SetAt: b.SetAt, SetBy: b.SetBy, WebhookPending: b.HasPendingWebhook}
	if !b.WebhookSetAt.IsZero() {
		at, by := b.WebhookSetAt, b.WebhookSetBy
		out.WebhookSetAt, out.WebhookSetBy = &at, &by
	}
	if !b.LastCheckAt.IsZero() {
		out.LastCheck = &zaloBotCheckOut{At: b.LastCheckAt, Result: b.LastCheckResult}
	}
	return out
}

// RegisterZaloCommuneBot mounts the five routes on the staff mux. Refuses incomplete wiring at construction.
func RegisterZaloCommuneBot(mux *http.ServeMux, d ZaloCommuneBotDeps) {
	switch {
	case d.Checker == nil:
		panic("comms/http: thiếu authz.Checker — các tuyến bot riêng của xã sẽ không kiểm được quyền")
	case d.Bots == nil:
		panic("comms/http: thiếu use case bot riêng của xã — /api/v1/zalo-bots/current sẽ panic khi có người gọi")
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	h := &zaloCommuneBotHandler{d: d}

	// NO idem.* DECLARATION: a GET. No audit entry: the configuration read in its own commune, and it
	// holds no secret.
	//
	// @summary  Bot Zalo riêng của xã — có hay không, tên, đường mở khung chat, mã tài khoản bot (Zalo trả), đặt lúc nào bởi ai, webhook đã đăng ký chưa, lần kiểm gần nhất, số cán bộ đang ghép nối; không bao giờ trả mã bot hay secret webhook
	// @screen   ADR 0079 Q1 #5 — Cấu hình → tab Kênh Zalo (spec 11 §3)
	// @reply    200 communeZaloBotCurrentOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/zalo-bots/current",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			http.HandlerFunc(h.GetCurrent)))

	// PUT: the commune's bot saved whole. idem.KhongCan: the same body twice leaves the same bot live —
	// a token for the account already live is an in-place update, never a second bot — at the cost of a
	// second entry recording the token set again.
	//
	//	400 invalid_request             a field the commune typed is malformed, or no token and no own bot
	//	409 zalo_bot_in_use             that bot account is live elsewhere on the platform (never says where)
	//	409 zalo_bot_changed            the commune's bot changed meanwhile — reload
	//	422 zalo_token_rejected         Zalo refused the token; nothing saved
	//	502 zalo_unavailable            Zalo could not be asked; nothing saved
	//	503 encryption_not_configured   the platform cannot seal the token; nothing saved
	//
	// @summary  Lưu bot Zalo riêng của xã — mã bot (chỉ ghi, niêm phong bằng khoá riêng của xã), tên bắt đầu bằng “Bot”, đường mở khung chat; mã tài khoản bot luôn lấy từ Zalo (getMe); đổi từ bot chung hoặc sang bot khác thì mọi ghép nối đang sống chấm dứt trong cùng giao dịch, có ghi vết
	// @screen   ADR 0079 Q1 #1 #4 — Cấu hình → tab Kênh Zalo (spec 11 §3 "Lưu con bot")
	// @request  communeZaloBotIn
	// @reply    200 communeZaloBotSavedOut
	// @reply    400 httpx.Error invalid_request
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error zalo_bot_in_use zalo_bot_changed
	// @reply    422 httpx.Error zalo_token_rejected
	// @reply    502 httpx.Error zalo_unavailable
	// @reply    503 httpx.Error encryption_not_configured
	// @reply    500 httpx.Error
	mux.Handle("PUT /api/v1/zalo-bots/current",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("lưu là ghi đè bot của xã bằng đúng giá trị trong thân; cùng mã bot thì vẫn là con bot đang sống, cập nhật tại chỗ, không sinh bot thứ hai — lần gửi lại chỉ thêm một vết ghi nhận đặt lại mã")(
				http.HandlerFunc(h.PutCurrent))))

	// idem.KhongCan: a check asks Zalo and records the latest answer; a second check is another check.
	//
	//	409 zalo_own_bot_missing        the commune has no own bot to check
	//
	// @summary  Kiểm tra kết nối bot Zalo riêng của xã — gọi getMe bằng mã đã lưu, ghi kết quả (lớp lỗi, không bao giờ chữ của Zalo) và vết
	// @screen   Cấu hình → tab Kênh Zalo (spec 11 §3 "Kiểm tra kết nối")
	// @reply    200 zaloBotCheckResultOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error zalo_own_bot_missing zalo_bot_changed
	// @reply    503 httpx.Error encryption_not_configured
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/zalo-bots/current/check",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("kiểm tra chỉ hỏi Zalo rồi ghi kết quả mới nhất; lần thứ hai là một lần kiểm khác, không đổi gì khác")(
				http.HandlerFunc(h.PostCheck))))

	// idem.KhongCan: pending-then-promote makes a repeat safe — a secret still pending is REUSED (and not
	// shown again), a confirmed one stays in force; no second secret is ever left half-registered.
	//
	//	409 zalo_own_bot_missing        the commune has no own bot
	//	503 commune_host_unavailable    the platform registry gave no host for the commune
	//
	// @summary  Đăng ký webhook cho bot Zalo riêng của xã — sinh secret ngẫu nhiên (CSPRNG), lưu chờ rồi gọi setWebhook tới https://<tên miền xã>/api/v1/zalo-bot-updates, Zalo xác nhận thì đưa vào dùng; secret hiện MỘT lần trong câu trả lời này
	// @screen   ADR 0079 Q1 #2 #3 — Cấu hình → tab Kênh Zalo (spec 11 §3 "Đăng ký webhook")
	// @reply    200 zaloBotWebhookOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error zalo_own_bot_missing zalo_bot_changed
	// @reply    503 httpx.Error encryption_not_configured commune_host_unavailable
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/zalo-bots/current/webhook",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("secret đang chờ thì dùng lại và không hiện lần nữa, secret đã xác nhận thì giữ nguyên; lần gửi thứ hai không bao giờ để lại một secret đăng ký dở")(
				http.HandlerFunc(h.PostWebhook))))

	// DELETE with a body: rule 7 — the retirement carries its reason (the map-asset-type delete's
	// argument: a free-text reason does not belong in a URL). idem.KhongCan: a second DELETE finds no own
	// bot, writes nothing, files nothing.
	//
	// @summary  Quay về bot chung — ngừng dùng bot riêng của xã (xoá mềm, giữ làm lịch sử, cần lý do), mọi ghép nối qua bot ấy chấm dứt trong cùng giao dịch, có ghi vết; câu trả lời nhắc xã thu hồi mã bot cũ trong Zalo Bot Creator
	// @screen   ADR 0079 Q1 #4 #5 — Cấu hình → tab Kênh Zalo (spec 11 §3 "Quay về bot chung")
	// @request  communeZaloBotRetireIn
	// @reply    200 communeZaloBotRetiredOut
	// @reply    400 httpx.Error invalid_request
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/zalo-bots/current",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("lần thứ hai không còn bot riêng nào đang sống nên không ghi gì và không có vết")(
				http.HandlerFunc(h.DeleteCurrent))))
}

// actor is the signed-in member of staff, by BUSINESS CODE (rule 6, invariant 8) — never a fallback.
func (h *zaloCommuneBotHandler) actor(w http.ResponseWriter, r *http.Request) (audit.Actor, bool) {
	a, ok := nguoiThucHien(r)
	if !ok {
		h.d.Log.Error("tuyến bot riêng của xã chạy mà không có chủ thể mang mã cán bộ",
			"xa", string(tenant.MustFrom(r.Context())), "duong", r.URL.Path)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
	return a, ok
}

// GetCurrent — GET /api/v1/zalo-bots/current
func (h *zaloCommuneBotHandler) GetCurrent(w http.ResponseWriter, r *http.Request) {
	v, err := h.d.Bots.Current(r.Context())
	if err != nil {
		h.fail(w, r, "đọc", err)
		return
	}
	out := communeZaloBotCurrentOut{HasOwnBot: v.HasOwnBot, LiveLinkCount: v.LiveLinkCount}
	if v.HasOwnBot {
		b := communeBotToOut(v.Bot)
		out.Bot = &b
	}
	vietJSON(w, http.StatusOK, out)
}

// PutCurrent — PUT /api/v1/zalo-bots/current
func (h *zaloCommuneBotHandler) PutCurrent(w http.ResponseWriter, r *http.Request) {
	var in communeZaloBotIn
	if !docThan(w, r, &in) {
		return
	}
	// The one conversion: from here on the token is a secret.Secret, which refuses to render.
	token := secret.Secret(in.BotToken)
	in.BotToken = ""
	defer clear(token)

	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	res, err := h.d.Bots.Set(r.Context(), app.SetCommuneZaloBotInput{Token: token, BotName: in.BotName, ChatURL: in.ChatURL}, actor)
	if err != nil {
		h.fail(w, r, "lưu", err)
		return
	}
	out := communeZaloBotSavedOut{Bot: communeBotToOut(res.Bot), Adopted: res.Adopted, EndedLinkCount: res.EndedLinkCount}
	if res.RetiredPrevious {
		out.RevokeNotice = domain.CommuneZaloBotRevokeNotice
	}
	vietJSON(w, http.StatusOK, out)
}

// PostCheck — POST /api/v1/zalo-bots/current/check
func (h *zaloCommuneBotHandler) PostCheck(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	res, err := h.d.Bots.Check(r.Context(), actor)
	if err != nil {
		h.fail(w, r, "kiểm tra", err)
		return
	}
	vietJSON(w, http.StatusOK, zaloBotCheckResultOut{Result: res.Outcome, CheckedAt: res.CheckedAt, AccountName: res.AccountName})
}

// PostWebhook — POST /api/v1/zalo-bots/current/webhook
func (h *zaloCommuneBotHandler) PostWebhook(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	res, err := h.d.Bots.RegisterWebhook(r.Context(), actor)
	if err != nil {
		h.fail(w, r, "đăng ký webhook", err)
		return
	}
	defer clear(res.Secret)
	out := zaloBotWebhookOut{Result: res.Outcome, URL: res.URL, SetBy: res.SetBy, Secret: string(res.Secret.Lo())}
	if !res.SetAt.IsZero() {
		at := res.SetAt
		out.SetAt = &at
	}
	// A credential may be in this body: never cached by anything between here and the browser.
	w.Header().Set("Cache-Control", "no-store")
	vietJSON(w, http.StatusOK, out)
}

// DeleteCurrent — DELETE /api/v1/zalo-bots/current
func (h *zaloCommuneBotHandler) DeleteCurrent(w http.ResponseWriter, r *http.Request) {
	var in communeZaloBotRetireIn
	if !docThan(w, r, &in) {
		return
	}
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	res, err := h.d.Bots.Retire(r.Context(), in.Reason, actor)
	if err != nil {
		h.fail(w, r, "quay về bot chung", err)
		return
	}
	out := communeZaloBotRetiredOut{Retired: res.Retired, EndedLinkCount: res.EndedLinkCount}
	if res.Retired {
		out.RevokeNotice = domain.CommuneZaloBotRevokeNotice
	}
	vietJSON(w, http.StatusOK, out)
}

// communeBotRefusals — the 400s, one fixed sentence per sentinel (the domain sentence after its package
// prefix never reaches the body: `refusal` in map_field_schema.go).
var communeBotRefusals = []refusal{
	{domain.ErrCommuneZaloBotToken, "Mã bot không hợp lệ. Hãy chép nguyên mã lấy trong Mini App “Zalo Bot Creator”."},
	{domain.ErrCommuneZaloBotTokenRequired, "Xã chưa có bot riêng nên phải nhập mã bot."},
	{domain.ErrCommuneZaloBotName, "Tên bot phải bắt đầu bằng “Bot”, không quá dài và không có ký tự điều khiển."},
	{domain.ErrCommuneZaloBotChatURL, "Đường mở khung chat phải là một liên kết https:// đầy đủ, ví dụ https://zalo.me/…"},
	{domain.ErrCommuneZaloBotRetireReason, "Cần nhập lý do quay về bot chung (tối đa 200 ký tự)."},
}

// fail maps a use-case error to its reply. The wrapped error never reaches the client (rule 3), and
// never names another commune.
func (h *zaloCommuneBotHandler) fail(w http.ResponseWriter, r *http.Request, op string, err error) {
	xa := string(tenant.MustFrom(r.Context()))
	var tc *app.ZaloTokenCheckError
	switch {
	case errors.Is(err, crypto.ErrNotConfigured):
		httpx.WriteError(w, http.StatusServiceUnavailable, "encryption_not_configured",
			"Nền tảng chưa cấu hình khoá mã hoá bí mật (SECRET_ENCRYPTION_KEYS), nên chưa lưu hay dùng được mã bot "+
				"của xã. Chưa có gì được ghi. Hãy báo đơn vị vận hành hệ thống.", "")
	case errors.As(err, &tc):
		// The class only — never Zalo's words: the token sits in the URL of the call.
		h.d.Log.Warn("bot riêng của xã: Zalo không nhận mã khi "+op, "xa", xa, "ket_qua", tc.Class)
		if tc.Class == domain.ZaloCallTokenRejected {
			httpx.WriteError(w, http.StatusUnprocessableEntity, "zalo_token_rejected",
				"Zalo không nhận mã bot này. Hãy kiểm tra lại mã trong Mini App “Zalo Bot Creator”. Chưa có gì được lưu.", "")
			return
		}
		httpx.WriteError(w, http.StatusBadGateway, "zalo_unavailable",
			"Chưa hỏi được Zalo để kiểm tra mã bot. Vui lòng thử lại sau ít phút. Chưa có gì được lưu.", "")
	case errors.Is(err, commsstore.ErrCommuneZaloBotAccountTaken):
		// ONE sentence for "another commune's bot" and "the platform's shared bot": which one it is would
		// tell this commune something about another (rule 1).
		httpx.WriteError(w, http.StatusConflict, "zalo_bot_in_use",
			"Con bot này đang được dùng ở nơi khác trên nền tảng nên xã không dùng được. Hãy tạo một bot riêng cho xã "+
				"trong Mini App “Zalo Bot Creator”.", "")
	case errors.Is(err, app.ErrCommuneZaloBotChanged), errors.Is(err, commsstore.ErrZaloRowChanged):
		httpx.WriteError(w, http.StatusConflict, "zalo_bot_changed",
			"Bot của xã vừa được thay đổi ở nơi khác. Vui lòng tải lại trang rồi thử lại.", "")
	case errors.Is(err, app.ErrCommuneZaloBotMissing):
		httpx.WriteError(w, http.StatusConflict, "zalo_own_bot_missing",
			"Xã đang dùng bot chung của nền tảng, chưa có bot riêng.", "")
	case errors.Is(err, app.ErrCommuneHostUnknown):
		h.d.Log.Error("bot riêng của xã: sổ xã của nền tảng không cho tên miền của xã", "xa", xa)
		httpx.WriteError(w, http.StatusServiceUnavailable, "commune_host_unavailable",
			"Chưa xác định được tên miền của xã để đăng ký webhook. Hãy báo đơn vị vận hành hệ thống.", "")
	default:
		if msg, ok := refusalMessage(communeBotRefusals, err); ok {
			h.d.Log.Info("bot riêng của xã: từ chối "+op, "xa", xa, "err", err)
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", msg, "")
			return
		}
		h.d.Log.Error("bot riêng của xã: "+op+" lỗi hệ thống", "xa", xa, "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}
