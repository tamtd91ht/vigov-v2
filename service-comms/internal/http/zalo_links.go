package http

// THE ZALO BOT CHANNEL, COMMUNE SIDE (ADR 0074 §"Tài nguyên URL", decided by the user 05/10/2026):
//
//	GET    /api/v1/zalo-links/current                AnyAuthenticated  the caller's own link
//	DELETE /api/v1/zalo-links/current                AnyAuthenticated  end it (soft, audited)
//	POST   /api/v1/zalo-links/current/pairing-codes  AnyAuthenticated  a one-time code (201)
//	POST   /api/v1/zalo-links/current/test-messages  AnyAuthenticated  one fixed test message
//	GET    /api/v1/zalo-links                        admin.lookup      who in the commune is linked
//	GET    /api/v1/zalo-channel-settings             admin.lookup      the commune's channel settings
//	PUT    /api/v1/zalo-channel-settings             admin.lookup      save them
//
// `zalo-links/current` HAS THE SHAPE OF `sessions/current` (ADR 0074): the person is the SESSION's
// business code and nothing else — no path segment, query or body field names a member of staff, so
// "every signed-in account" can touch exactly ONE link, its own (ADR 0074 #6, the bell's argument in
// routes_staff_notification.go). The `quyen` table has no key for "my own Zalo link" and none is invented
// (rule 5, 3c). `admin.lookup` is the key ADR 0074 #6 names for the Kênh Zalo tab — seeded at
// service-identity/migrations/0001_init.sql:281, the same as `mail-settings`.
//
// NO chat_id LEAVES HERE (ADR 0074: "không trả ra giao diện"): no type below has a field for it. The
// field names are web-admin/src/lib/api/zalo.ts's.

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// ZaloLinkReader is the READ half — no transaction. *app.ZaloLinks satisfies it.
type ZaloLinkReader interface {
	Current(ctx context.Context, actor audit.Actor) (app.ZaloLinkCurrentView, error)
	LinkedStaff(ctx context.Context) ([]app.LinkedStaffView, error)
	Settings(ctx context.Context) (domain.ZaloChannelSetting, error)
	// BotReady: some bot can serve the commune — its own live bot, or the shared bot with a token.
	BotReady(ctx context.Context) (bool, error)
}

// ZaloLinkWriter is the WRITE half: each method opens a transaction and writes its audit entry inside
// it (rule 6, invariant 3). *app.ZaloLinks satisfies it.
type ZaloLinkWriter interface {
	IssuePairingCode(ctx context.Context, actor audit.Actor) (app.PairingCodeView, error)
	Unlink(ctx context.Context, actor audit.Actor) error
	SendTestMessage(ctx context.Context, actor audit.Actor) error
	SaveSettings(ctx context.Context, in domain.ZaloChannelSetting, actor audit.Actor) (domain.ZaloChannelSetting, error)
}

// ZaloLinkDeps is everything the seven routes touch.
type ZaloLinkDeps struct {
	Checker authz.Checker
	Reader  ZaloLinkReader
	Writer  ZaloLinkWriter
	Log     *slog.Logger
}

type zaloLinkHandler struct{ d ZaloLinkDeps }

// zaloLinkCurrentOut — ZaloLinkCurrent in zalo.ts. bot_name / chat_url whenever the bot is configured
// (the person needs the chat link BEFORE pairing); linked_at only when linked.
type zaloLinkCurrentOut struct {
	Linked         bool       `json:"linked"`
	LinkedAt       *time.Time `json:"linked_at,omitempty"`
	BotName        string     `json:"bot_name,omitempty"`
	ChatURL        string     `json:"chat_url,omitempty"`
	ChannelEnabled bool       `json:"channel_enabled"`
}

// pairingCodeOut — ZaloPairingCode in zalo.ts. The code is shown ONCE; the server keeps only its hash.
type pairingCodeOut struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expires_at"`
	ChatURL   string    `json:"chat_url"`
}

// zaloLinkedStaffOut — ZaloLinkedStaff in zalo.ts. staff_name is "" when identity holds no name for the
// code any more (a removed record) — rendered as unknown, never invented.
type zaloLinkedStaffOut struct {
	StaffCode string    `json:"staff_code"`
	StaffName string    `json:"staff_name"`
	LinkedAt  time.Time `json:"linked_at"`
}

type zaloLinkedStaffList struct {
	Items []zaloLinkedStaffOut `json:"items"`
}

// zaloChannelSettingsOut — ZaloChannelSettings in zalo.ts. A commune that never saved gets is_enabled
// false with the defaults, and no updated_at / updated_by.
//
// `kinds` IS ALWAYS PER-DOMAIN (migration 0021): a row still holding one of the four old values is read
// as the per-domain kinds it means. `supported_events` is the list a commune may tick — 0021's twelve
// per-domain kinds and the weekly digest, in display order (spec Cấu hình 11 §2 "Chỉ hiện các sự kiện có
// trong supported_events"); the ten spec events with no producer yet are not in it (ADR 0079 Q3).
type zaloChannelSettingsOut struct {
	IsEnabled              bool       `json:"is_enabled"`
	Kinds                  []string   `json:"kinds"`
	QuietStart             string     `json:"quiet_start"`
	QuietEnd               string     `json:"quiet_end"`
	OverdueStartAfterDays  *int       `json:"overdue_start_after_days"`
	OverdueRepeatEveryDays *int       `json:"overdue_repeat_every_days"`
	UpdatedAt              *time.Time `json:"updated_at,omitempty"`
	UpdatedBy              string     `json:"updated_by,omitempty"`
	SupportedEvents        []string   `json:"supported_events"`
	// PlatformReady (GET only) is spec 11 §0's `platform_ready`: false = no bot can serve the commune —
	// neither its own live bot nor the shared bot with a token — so nothing can be sent yet.
	PlatformReady *bool `json:"platform_ready,omitempty"`
}

// zaloChannelSettingsIn — ZaloChannelSettingsChange in zalo.ts: the whole form. is_enabled, kinds and the
// two quiet times are required; the two overdue numbers are null unless a `*.qua-han` kind is chosen.
// `kinds` takes supported_events values; one of the four OLD values is still accepted and saved as the
// per-domain kinds it means (0021).
type zaloChannelSettingsIn struct {
	IsEnabled              *bool    `json:"is_enabled"`
	Kinds                  []string `json:"kinds"`
	QuietStart             *string  `json:"quiet_start"`
	QuietEnd               *string  `json:"quiet_end"`
	OverdueStartAfterDays  *int     `json:"overdue_start_after_days"`
	OverdueRepeatEveryDays *int     `json:"overdue_repeat_every_days"`
}

func settingsToOut(s domain.ZaloChannelSetting) zaloChannelSettingsOut {
	out := zaloChannelSettingsOut{
		IsEnabled: s.IsEnabled, Kinds: s.Kinds,
		QuietStart: domain.FormatClock(s.QuietStartMinute), QuietEnd: domain.FormatClock(s.QuietEndMinute),
		OverdueStartAfterDays: s.OverdueStartAfterDays, OverdueRepeatEveryDays: s.OverdueRepeatEveryDays,
	}
	if out.Kinds == nil {
		out.Kinds = []string{}
	}
	out.SupportedEvents = append([]string(nil), domain.ZaloReminderKinds...)
	if s.Saved {
		at := s.UpdatedAt
		out.UpdatedAt, out.UpdatedBy = &at, s.UpdatedBy
	}
	return out
}

// RegisterZaloLinks mounts the seven routes. Refuses incomplete wiring at construction.
func RegisterZaloLinks(mux *http.ServeMux, d ZaloLinkDeps) {
	switch {
	case d.Checker == nil:
		panic("comms/http: thiếu authz.Checker — các tuyến kênh Zalo sẽ không kiểm được quyền")
	case d.Reader == nil:
		panic("comms/http: thiếu bộ đọc kênh Zalo — GET /api/v1/zalo-links… sẽ panic khi có người gọi")
	case d.Writer == nil:
		panic("comms/http: thiếu use case ghi kênh Zalo — POST/PUT/DELETE /api/v1/zalo-… sẽ panic khi có người gọi")
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	h := &zaloLinkHandler{d: d}

	// NO idem.* DECLARATION: a GET changes no state. No audit entry: the person reads their own link.
	//
	// @summary  Trạng thái ghép nối Zalo của CHÍNH cán bộ đang đăng nhập — đã ghép chưa, từ lúc nào, tên và đường dẫn chat của bot dùng chung, xã đã bật kênh Zalo chưa; không bao giờ trả chat_id
	// @screen   ADR 0074 — trang Cá nhân (/ca-nhan)
	// @reply    200 zaloLinkCurrentOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/zalo-links/current",
		authz.AnyAuthenticated("liên kết Zalo là của CHÍNH cán bộ đang đăng nhập, lọc theo mã cán bộ của phiên — mỗi tài khoản chỉ đọc được liên kết của mình; ADR 0074 #6 chốt AnyAuthenticated và bảng quyen không có khoá nào cho việc này")(
			http.HandlerFunc(h.GetCurrentZaloLink)))

	// Unlink. idem.KhongCan: a second DELETE finds no live link, writes nothing and files nothing.
	//
	// @summary  Gỡ ghép nối Zalo của chính mình — kết thúc mềm liên kết (giữ làm lịch sử), có ghi vết; chưa ghép thì không ghi gì
	// @screen   ADR 0074 — trang Cá nhân (/ca-nhan)
	// @reply    204 -
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/zalo-links/current",
		authz.AnyAuthenticated("cán bộ gỡ ghép nối Zalo của chính mình; câu ghi lọc theo mã cán bộ của phiên nên không chạm được liên kết của người khác (ADR 0074 #6)")(
			idem.KhongCan("lần gỡ thứ hai không còn liên kết đang sống nên không ghi gì và không có vết")(
				http.HandlerFunc(h.UnlinkCurrentZalo))))

	// A one-time code. 201: a new resource. NO BODY: whose code it is comes from the session.
	//
	// idem.KhongCan, AND WHY NOT idem.Required: a replay would have to keep the reply — a LIVE CREDENTIAL —
	// in the cache for a day. A duplicate request instead issues a second code and cancels the first in
	// the same transaction (0018: at most one open code per member of staff), so the state after two
	// requests is still exactly one open code; the cost is one cancelled code and one more entry.
	//
	//	409 zalo_bot_not_configured  the shared bot has no token yet — a code for it would pair nothing
	//
	// @summary  Lấy mã ghép nối Zalo 8 ký tự cho chính mình — dùng một lần, sống 10 phút, mã cũ còn mở bị huỷ; máy chủ chỉ lưu bản băm; có ghi vết (không ghi mã)
	// @screen   ADR 0074 — trang Cá nhân (/ca-nhan)
	// @reply    201 pairingCodeOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error zalo_bot_not_configured
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/zalo-links/current/pairing-codes",
		authz.AnyAuthenticated("mã ghép nối Zalo được cấp cho CHÍNH cán bộ đang đăng nhập, theo mã cán bộ của phiên — không ai lấy được mã ghép vào tài khoản của người khác (ADR 0074 #6)")(
			idem.KhongCan("lần gửi thứ hai cấp mã mới và huỷ mã trước trong cùng giao dịch, nên sau hai lần vẫn đúng một mã đang mở; lưu câu trả lời để phát lại là giữ một mã đăng nhập còn sống trong bộ đệm")(
				http.HandlerFunc(h.CreatePairingCode))))

	// One fixed test message to the person's own chat. idem.Required(MoKhiHong): a double click must not
	// send two messages; a cache outage costs at worst a second test message to one's own chat.
	//
	//	409 zalo_channel_disabled    the commune has not switched its Zalo channel on
	//	409 zalo_bot_not_configured  the shared bot has no token
	//	409 zalo_not_linked          the person has not paired
	//	502 zalo_send_failed         Zalo refused or could not be reached (the class only, never Zalo's text)
	//
	// @summary  Gửi một tin thử cố định tới chat Zalo đã ghép của chính mình — có ghi vết và lưu kết quả gửi
	// @screen   ADR 0074 — trang Cá nhân (/ca-nhan)
	// @reply    200 -
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error zalo_channel_disabled zalo_bot_not_configured zalo_not_linked
	// @reply    502 httpx.Error zalo_send_failed
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/zalo-links/current/test-messages",
		authz.AnyAuthenticated("tin thử chỉ gửi tới chat Zalo đã ghép của CHÍNH cán bộ đang đăng nhập, tra theo mã cán bộ của phiên (ADR 0074 #6)")(
			idem.Required(idem.MoKhiHong)(
				http.HandlerFunc(h.SendZaloTestMessage))))

	// Who in the commune is linked: code, name, since when. No chat id. NO idem.* DECLARATION: a GET.
	//
	//	503 staff_names_unavailable  identity could not be asked for the names — never a page of blanks
	//
	// @summary  Danh sách cán bộ của xã đang ghép nối Zalo — mã cán bộ, họ tên, ghép từ lúc nào; không có chat_id
	// @screen   ADR 0074 — Cấu hình → tab Kênh Zalo
	// @reply    200 zaloLinkedStaffList
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	// @reply    503 httpx.Error staff_names_unavailable
	mux.Handle("GET /api/v1/zalo-links",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			http.HandlerFunc(h.ListZaloLinks)))

	// NO idem.* DECLARATION: a GET. No row = is_enabled false with the defaults.
	//
	// @summary  Cấu hình kênh nhắc việc Zalo của xã — bật/tắt, các loại nhắc theo từng phân hệ (supported_events là danh sách được chọn), giờ yên tĩnh (giờ Việt Nam), nhịp nhắc việc quá hạn, platform_ready (đã có bot phục vụ xã chưa); xã chưa lưu thì là tắt
	// @screen   ADR 0074 — Cấu hình → tab Kênh Zalo
	// @reply    200 zaloChannelSettingsOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/zalo-channel-settings",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			http.HandlerFunc(h.GetZaloChannelSettings)))

	// PUT AND NOT PATCH: one form saved whole. idem.KhongCan: a save equal to what is stored writes and
	// files nothing (app.ZaloLinks.SaveSettings), so the same request twice leaves one entry.
	//
	//	400 invalid_request  not JSON, or is_enabled / kinds / quiet_start / quiet_end missing
	//	422 <code>           a value 0018's CHECKs refuse — the code and sentence say which
	//
	// @summary  Lưu cấu hình kênh nhắc việc Zalo của xã — bật thì phải chọn ít nhất một loại; chọn quá hạn thì phải đặt đủ nhịp nhắc; giờ yên tĩnh HH:MM không trùng nhau; có ghi vết trước/sau
	// @screen   ADR 0074 — Cấu hình → tab Kênh Zalo
	// @request  zaloChannelSettingsIn
	// @reply    200 zaloChannelSettingsOut
	// @reply    400 httpx.Error invalid_request
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    422 httpx.Error unknown_kind enabled_without_kind invalid_quiet_time empty_quiet_window overdue_cadence_incomplete overdue_cadence_required overdue_start_out_of_range overdue_repeat_out_of_range
	// @reply    500 httpx.Error
	mux.Handle("PUT /api/v1/zalo-channel-settings",
		authz.RequirePermission(d.Checker, "admin.lookup")(
			idem.KhongCan("lưu là ghi đè cấu hình bằng đúng giá trị trong thân; trùng giá trị đang lưu thì không ghi và không có vết, nên lần gửi thứ hai để lại đúng một vết")(
				http.HandlerFunc(h.PutZaloChannelSettings))))
}

// owner returns the signed-in STAFF member as the actor — the bell's rule (inboxOwner): not staff 403,
// no business code 500, never a fallback to the internal id (rule 6, invariant 8).
func (h *zaloLinkHandler) owner(w http.ResponseWriter, r *http.Request) (audit.Actor, bool) {
	p, ok := authz.From(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized",
			"Phiên làm việc không hợp lệ hoặc đã kết thúc. Vui lòng đăng nhập lại.", "")
		return audit.Actor{}, false
	}
	if p.Kind != "staff" {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "Ghép nối Zalo chỉ dành cho tài khoản cán bộ.", "")
		return audit.Actor{}, false
	}
	if p.Ma == "" {
		h.d.Log.Error("kênh Zalo: chủ thể không có mã cán bộ", "xa", string(tenant.MustFrom(r.Context())))
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return audit.Actor{}, false
	}
	return audit.Actor{ID: p.Ma, Kind: p.Kind, IP: httpx.ClientIP(r)}, true
}

// GetCurrentZaloLink — GET /api/v1/zalo-links/current
func (h *zaloLinkHandler) GetCurrentZaloLink(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.owner(w, r)
	if !ok {
		return
	}
	v, err := h.d.Reader.Current(r.Context(), actor)
	if err != nil {
		h.fail(w, r, "đọc liên kết", err)
		return
	}
	out := zaloLinkCurrentOut{Linked: v.Linked, BotName: v.BotName, ChatURL: v.ChatURL, ChannelEnabled: v.ChannelEnabled}
	if v.Linked {
		at := v.LinkedAt
		out.LinkedAt = &at
	}
	vietJSON(w, http.StatusOK, out)
}

// UnlinkCurrentZalo — DELETE /api/v1/zalo-links/current
func (h *zaloLinkHandler) UnlinkCurrentZalo(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.owner(w, r)
	if !ok {
		return
	}
	if err := h.d.Writer.Unlink(r.Context(), actor); err != nil {
		h.fail(w, r, "gỡ ghép nối", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CreatePairingCode — POST /api/v1/zalo-links/current/pairing-codes
func (h *zaloLinkHandler) CreatePairingCode(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.owner(w, r)
	if !ok {
		return
	}
	v, err := h.d.Writer.IssuePairingCode(r.Context(), actor)
	if err != nil {
		h.fail(w, r, "cấp mã ghép", err)
		return
	}
	// A credential: never cached by anything between here and the browser.
	w.Header().Set("Cache-Control", "no-store")
	vietJSON(w, http.StatusCreated, pairingCodeOut{Code: v.Code, ExpiresAt: v.ExpiresAt, ChatURL: v.ChatURL})
}

// SendZaloTestMessage — POST /api/v1/zalo-links/current/test-messages
func (h *zaloLinkHandler) SendZaloTestMessage(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.owner(w, r)
	if !ok {
		return
	}
	if err := h.d.Writer.SendTestMessage(r.Context(), actor); err != nil {
		h.fail(w, r, "gửi tin thử", err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// ListZaloLinks — GET /api/v1/zalo-links
func (h *zaloLinkHandler) ListZaloLinks(w http.ResponseWriter, r *http.Request) {
	list, err := h.d.Reader.LinkedStaff(r.Context())
	if err != nil {
		h.fail(w, r, "liệt kê liên kết", err)
		return
	}
	out := zaloLinkedStaffList{Items: make([]zaloLinkedStaffOut, 0, len(list))}
	for _, l := range list {
		out.Items = append(out.Items, zaloLinkedStaffOut{StaffCode: l.StaffCode, StaffName: l.StaffName, LinkedAt: l.LinkedAt})
	}
	vietJSON(w, http.StatusOK, out)
}

// GetZaloChannelSettings — GET /api/v1/zalo-channel-settings
func (h *zaloLinkHandler) GetZaloChannelSettings(w http.ResponseWriter, r *http.Request) {
	s, err := h.d.Reader.Settings(r.Context())
	if err != nil {
		h.fail(w, r, "đọc cấu hình", err)
		return
	}
	ready, err := h.d.Reader.BotReady(r.Context())
	if err != nil {
		h.fail(w, r, "đọc trạng thái bot", err)
		return
	}
	out := settingsToOut(s)
	out.PlatformReady = &ready
	vietJSON(w, http.StatusOK, out)
}

// PutZaloChannelSettings — PUT /api/v1/zalo-channel-settings
func (h *zaloLinkHandler) PutZaloChannelSettings(w http.ResponseWriter, r *http.Request) {
	var in zaloChannelSettingsIn
	if !docThan(w, r, &in) {
		return
	}
	if in.IsEnabled == nil || in.Kinds == nil || in.QuietStart == nil || in.QuietEnd == nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Cần đủ các trường: bật/tắt (`is_enabled`), loại nhắc (`kinds`), giờ yên tĩnh (`quiet_start`, `quiet_end`).", "")
		return
	}
	qs, ok1 := domain.ParseClock(*in.QuietStart)
	qe, ok2 := domain.ParseClock(*in.QuietEnd)
	if !ok1 || !ok2 {
		httpx.WriteError(w, http.StatusUnprocessableEntity, "invalid_quiet_time",
			"Giờ yên tĩnh phải có dạng HH:MM, từ 00:00 đến 23:59.", "")
		return
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.d.Log.Error("tuyến lưu cấu hình kênh Zalo chạy mà không có chủ thể mang mã cán bộ",
			"xa", string(tenant.MustFrom(r.Context())))
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	s, err := h.d.Writer.SaveSettings(r.Context(), domain.ZaloChannelSetting{
		IsEnabled: *in.IsEnabled, Kinds: in.Kinds, QuietStartMinute: qs, QuietEndMinute: qe,
		OverdueStartAfterDays: in.OverdueStartAfterDays, OverdueRepeatEveryDays: in.OverdueRepeatEveryDays,
	}, actor)
	if err != nil {
		h.fail(w, r, "lưu cấu hình", err)
		return
	}
	vietJSON(w, http.StatusOK, settingsToOut(s))
}

// fail maps a use-case error to its reply. The wrapped error never reaches the client (rule 3).
func (h *zaloLinkHandler) fail(w http.ResponseWriter, r *http.Request, op string, err error) {
	var se *domain.ZaloSettingError
	var send *app.ZaloSendError
	switch {
	case errors.As(err, &se):
		httpx.WriteError(w, http.StatusUnprocessableEntity, se.Code, se.Message, "")
	case errors.Is(err, app.ErrZaloBotNotConfigured):
		httpx.WriteError(w, http.StatusConflict, "zalo_bot_not_configured",
			"Bot Zalo của nền tảng chưa được cấu hình. Vui lòng báo quản trị nền tảng.", "")
	case errors.Is(err, app.ErrZaloChannelOff):
		httpx.WriteError(w, http.StatusConflict, "zalo_channel_disabled",
			"Xã chưa bật kênh nhắc việc qua Zalo. Quản trị viên xã bật ở Cấu hình → Kênh Zalo.", "")
	case errors.Is(err, app.ErrZaloNotLinked):
		httpx.WriteError(w, http.StatusConflict, "zalo_not_linked", "Bạn chưa ghép nối Zalo.", "")
	case errors.As(err, &send):
		h.d.Log.Warn("kênh Zalo: "+op+" — Zalo không nhận", "xa", string(tenant.MustFrom(r.Context())), "ket_qua", send.Class)
		httpx.WriteError(w, http.StatusBadGateway, "zalo_send_failed",
			"Zalo chưa nhận tin thử. Vui lòng thử lại sau ít phút.", "")
	case errors.Is(err, app.ErrStaffNamesUnavailable):
		h.d.Log.Warn("kênh Zalo: "+op+" — không tra được tên cán bộ", "xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "staff_names_unavailable",
			"Tạm thời không tải được họ tên cán bộ. Vui lòng thử lại sau ít phút.", "")
	default:
		h.d.Log.Error("kênh Zalo: "+op+" lỗi hệ thống", "xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}
