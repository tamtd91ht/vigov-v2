package http

// The Zalo Bot screen of platform-admin (ADR 0074 §"Tài nguyên URL"): the ONE shared platform bot —
// its token, its webhook — and every commune's uptake of it. This service is a RELAY to comms'
// ZaloBotOperatorService (proto/vigov/comms/v1/zalo_bot_operator.proto, the spec of every answer
// below): comms holds the bot, seals the token, calls Zalo and writes THE trail of every act.
//
// TRUST (ADR 0074 #3, owner 05/10/2026): comms does not re-ask identity. The `ops.zalo_bot.manage`
// check made HERE, by opauth on this request, is the only authorisation the act gets; `actor_code` is
// the business code of the principal opauth resolved, never a value read from the request.
//
// WHERE THE BOT TOKEN GOES IN THIS PROCESS, AND WHERE IT DOES NOT — the whole list:
//
//	body bytes      read once by decodeBody (bounded by maxOperatorBody), never logged
//	decoded value   secretInput → secret.Secret ("***" in every rendering)
//	idempotency     idem.KhongCan: core/idem stores nothing
//	comms           one gRPC field, taken out with .Lo() at the line that builds the request. The
//	                request is NEVER logged: its generated String() prints the token
//	responses       never — the answer is metadata (set_at, set_by, has_token, bot_name, chat_url)
//	errors / logs   never — a failed call logs the RPC name and the gRPC code only; the display row
//	                (domain.ZaloBotAct) has no field that could hold it
//
// NO COMMUNE anywhere on these routes: the bot belongs to none (ADR 0074 #4), and the communes table
// reads every commune at once — a read across communes that comms and identity each audit on their
// side (rule 6 invariant 7), so platform adds no row for a read.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	commsv1 "github.com/vihat/vigov/core/gen/vigov/comms/v1"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/operatorclient"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/service-platform/internal/domain"
	"github.com/vihat/vigov/service-platform/internal/opauth"
	"github.com/vihat/vigov/service-platform/internal/store"
)

// ZaloBotActRecorder writes the display row of a Zalo Bot act comms answered (*store.ZaloBotDisplayLog).
type ZaloBotActRecorder interface {
	RecordZaloBotAct(ctx context.Context, act domain.ZaloBotAct, by domain.OperatorActor) error
}

// --- outcome classes -----------------------------------------------------------------------------------

// zaloOutcomeUnknown is what a class this build does not know becomes — UNSPECIFIED included. The
// console shows any value outside its list as "a failure of unknown cause", never as success
// (platform-admin zalo-bot-model.ts outcomeText; proto: "never as success").
const zaloOutcomeUnknown = "khong-ro"

// zaloOutcomes are the enum VALUES the console knows — kebab-case without diacritics, the values
// comms stores (ADR 0011; service-comms migration 0018).
var zaloOutcomes = map[commsv1.ZaloBotCallOutcome]string{
	commsv1.ZaloBotCallOutcome_ZALO_BOT_CALL_OUTCOME_OK:                 "thanh-cong",
	commsv1.ZaloBotCallOutcome_ZALO_BOT_CALL_OUTCOME_NOT_CONFIGURED:     "chua-cau-hinh",
	commsv1.ZaloBotCallOutcome_ZALO_BOT_CALL_OUTCOME_TOKEN_REJECTED:     "token-bi-tu-choi",
	commsv1.ZaloBotCallOutcome_ZALO_BOT_CALL_OUTCOME_RATE_LIMITED:       "gioi-han-tan-suat",
	commsv1.ZaloBotCallOutcome_ZALO_BOT_CALL_OUTCOME_UNAVAILABLE:        "khong-kha-dung",
	commsv1.ZaloBotCallOutcome_ZALO_BOT_CALL_OUTCOME_REJECTED:           "bi-tu-choi",
	commsv1.ZaloBotCallOutcome_ZALO_BOT_CALL_OUTCOME_MALFORMED_RESPONSE: "phan-hoi-sai-dang",
}

func zaloOutcome(o commsv1.ZaloBotCallOutcome) string {
	if s, ok := zaloOutcomes[o]; ok {
		return s
	}
	return zaloOutcomeUnknown
}

// --- shapes (platform-admin/src/lib/api.ts, "Zalo Bot (ADR 0074) wire shapes") ---------------------------

// sharedZaloBotBody — the token is a secretInput (operator_mini_app_secrets.go), never a string field.
type sharedZaloBotBody struct {
	Token   secretInput `json:"token"`
	BotName string      `json:"bot_name"`
	ChatURL string      `json:"chat_url"`
	// Absent on the first attempt; the count the operator confirmed on the second. A pointer only to
	// refuse a negative number with a clear code — absent is sent as 0, which the proto defines as
	// the real claim "no link will end" (against a bot with links it simply gets the dialog again).
	ExpectedRelinkCount *int64 `json:"expected_relink_count"`
}

// zaloBotStatusView is SharedZaloBotStatus — metadata about the live token, never the token.
type zaloBotStatusView struct {
	SetAt    time.Time `json:"set_at"`
	SetBy    string    `json:"set_by"`
	HasToken bool      `json:"has_token"`
	BotName  string    `json:"bot_name"`
	ChatURL  string    `json:"chat_url"`
}

type zaloBotCheckView struct {
	CheckedAt time.Time `json:"checked_at"`
	Outcome   string    `json:"outcome"`
}

// sharedZaloBotView answers GET and (with EndedLinkCount) PUT zalo-bots/shared.
type sharedZaloBotView struct {
	Configured     bool               `json:"configured"`
	Bot            *zaloBotStatusView `json:"bot,omitempty"`
	LastCheck      *zaloBotCheckView  `json:"last_check,omitempty"`
	EndedLinkCount *int32             `json:"ended_link_count,omitempty"`
}

type zaloBotCheckResultView struct {
	Outcome     string `json:"outcome"`
	AccountName string `json:"account_name,omitempty"`
}

type zaloBotWebhookView struct {
	Outcome       string     `json:"outcome"`
	URL           string     `json:"url"`
	URLMatches    bool       `json:"url_matches"`
	SecretSetAt   *time.Time `json:"secret_set_at,omitempty"`
	SecretSetBy   string     `json:"secret_set_by,omitempty"`
	ZaloUpdatedAt *time.Time `json:"zalo_updated_at,omitempty"`
}

type zaloBotWebhookSetView struct {
	Outcome string     `json:"outcome"`
	URL     string     `json:"url"`
	SetAt   *time.Time `json:"set_at,omitempty"`
	SetBy   string     `json:"set_by,omitempty"`
}

// zaloBotCommuneView is one row of the communes table — counts, a switch, which bot. Nothing about
// any person (ADR 0003).
type zaloBotCommuneView struct {
	TenantID       string `json:"tenant_id"`
	CommuneName    string `json:"commune_name"`
	ChannelEnabled bool   `json:"channel_enabled"`
	// The link count of THE BOT THE COMMUNE USES — own_bot_linked_staff_count when uses_own_bot,
	// linked_staff_count otherwise. Never the sum (zalo_bot_operator.proto, ZaloBotCommuneStats).
	LinkedStaffCount uint32 `json:"linked_staff_count"`
	UsesOwnBot       bool   `json:"uses_own_bot"`
	OwnBotName       string `json:"own_bot_name,omitempty"`
	// Active staff accounts (identity ListActiveStaffCounts) — the denominator of "Đã ghép nối".
	TotalStaffCount uint32 `json:"total_staff_count"`
}

type zaloBotCommuneListView struct {
	Items []zaloBotCommuneView `json:"items"`
}

// zaloBotRelinkView is httpx.Error plus the count the confirmation dialog shows (409
// `can-xac-nhan-ghep-lai`).
type zaloBotRelinkView struct {
	Code          string `json:"code"`
	Message       string `json:"message"`
	TraceID       string `json:"trace_id"`
	LiveLinkCount int32  `json:"live_link_count"`
}

// zaloBotTokenRefusedView is httpx.Error plus the class of Zalo's answer to the new token (422
// `token-khong-dung-duoc`). A class, never Zalo's text.
type zaloBotTokenRefusedView struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	TraceID string `json:"trace_id"`
	Outcome string `json:"outcome"`
}

const (
	codeRelinkRequired = "can-xac-nhan-ghep-lai"
	codeTokenRefused   = "token-khong-dung-duoc"

	msgZaloBotTokenInvalid  = "Token không đúng dạng: không có khoảng trắng, không chứa / ? # %, tối đa 256 ký tự."
	msgZaloBotNameInvalid   = "Tên bot không hợp lệ: cần 1–100 ký tự, không chứa ký tự điều khiển."
	msgZaloBotChatURL       = "Liên kết trò chuyện phải là địa chỉ đầy đủ bắt đầu bằng https://, tối đa 300 ký tự."
	msgZaloBotRelinkCount   = "Số cán bộ phải ghép nối lại không hợp lệ."
	msgZaloBotRelink        = "Token thuộc một tài khoản bot khác: cần xác nhận số cán bộ phải ghép nối lại."
	msgZaloBotTokenRefused  = "Zalo không chấp nhận token mới nên chưa lưu."
	msgZaloBotUnavailable   = "Hệ thống tạm thời không thao tác được với bot Zalo. Vui lòng thử lại sau."
	msgZaloBotKeyMissing    = "Hệ thống chưa cấu hình khoá dữ liệu cấp nền tảng cho bot Zalo. Báo bộ phận kỹ thuật."
	msgZaloBotRefusedShape  = "Yêu cầu tới bot Zalo không hợp lệ."
	msgStaffCountsUnavail   = "Hệ thống tạm thời không đếm được cán bộ của các xã. Vui lòng thử lại sau."
	msgCommuneListUnavail   = "Hệ thống tạm thời không đọc được danh sách xã. Vui lòng thử lại sau."
	maxZaloBotTokenBytes    = 256
	maxZaloBotNameRunes     = 100
	maxZaloChatURLBytes     = 300
	maxCommunePagesZaloStat = 1000 // 100 000 communes: a bound on a loop, never a business limit
)

// --- shape checks: the rules of SetSharedZaloBotRequest, answered field by field ------------------------
//
// Comms re-checks every one (INVALID_ARGUMENT), but its status cannot say WHICH field, and the screen
// puts the refusal under the field. These mirror the proto comments; comms stays the authority.

// validZaloBotToken: 1–256 bytes of printable ASCII, no whitespace, none of `/ ? # %` — the token is
// spliced into a URL path. Verbatim: never trimmed.
func validZaloBotToken(s secret.Secret) bool {
	b := s.Lo()
	if len(b) == 0 || len(b) > maxZaloBotTokenBytes {
		return false
	}
	for _, c := range b {
		if c < '!' || c > '~' || c == '/' || c == '?' || c == '#' || c == '%' {
			return false
		}
	}
	return true
}

// normalizeZaloBotName trims and bounds the display name; "" means refused.
func normalizeZaloBotName(s string) string {
	s = strings.TrimSpace(s)
	if n := utf8.RuneCountInString(s); n == 0 || n > maxZaloBotNameRunes || !utf8.ValidString(s) {
		return ""
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return ""
		}
	}
	return s
}

// normalizeZaloChatURL trims and checks the chat link: absolute https, a host, no user info, at most
// 300 bytes, no space or control character. "" means refused.
func normalizeZaloChatURL(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > maxZaloChatURLBytes {
		return ""
	}
	for _, r := range s {
		if unicode.IsControl(r) || r == ' ' {
			return ""
		}
	}
	u, err := url.Parse(s)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Opaque != "" {
		return ""
	}
	return s
}

// --- failures ------------------------------------------------------------------------------------------

// zaloBotCallFailed answers a comms call that returned a status. Logs the RPC and the gRPC code ONLY:
// never the request (its String() prints the token on Set), never the status message.
func (h *operatorHandlers) zaloBotCallFailed(w http.ResponseWriter, r *http.Request, rpc string, err error) {
	code := status.Code(err)
	h.d.Log.WarnContext(r.Context(), "khu vận hành: comms không thực hiện thao tác bot Zalo dùng chung",
		"event", "zalo_bot.call_failed", "rpc", rpc, "grpc_code", code.String())
	switch code {
	case codes.InvalidArgument:
		// The shape checks above ran first, so this is the two ends disagreeing on a rule.
		httpx.WriteError(w, http.StatusUnprocessableEntity, "invalid_request", msgZaloBotRefusedShape, "")
	case codes.FailedPrecondition:
		// Comms' platform data key is not configured (proto: "an operator-facing 503, not a 4xx").
		httpx.WriteError(w, http.StatusServiceUnavailable, "zalo_bot_key_not_configured", msgZaloBotKeyMissing, "")
	default:
		// Unreachable comms, deadline, its store down, an older comms (UNIMPLEMENTED): the act did
		// not happen as far as this answer knows. 503 — never "refused".
		httpx.WriteError(w, http.StatusServiceUnavailable, "zalo_bot_unavailable", msgZaloBotUnavailable, "")
	}
}

// zaloBotContractFault answers an answer comms gave outside its contract. Fail closed.
func (h *operatorHandlers) zaloBotContractFault(w http.ResponseWriter, r *http.Request, rpc, what string) {
	h.d.Log.ErrorContext(r.Context(), "CẢNH BÁO HỢP ĐỒNG: comms trả lời ngoài hợp đồng ZaloBotOperatorService",
		"event", "zalo_bot.contract_fault", "rpc", rpc, "what", what)
	httpx.WriteError(w, http.StatusServiceUnavailable, "zalo_bot_unavailable", msgZaloBotUnavailable, "")
}

// zaloBotReady answers 503 when comms is not wired (COMMS_GRPC_ADDR unset — dev only; staging/prod
// refuse to start without it, ADR 0057). false means the answer is written.
func (h *operatorHandlers) zaloBotReady(w http.ResponseWriter) bool {
	if h.d.ZaloBot != nil {
		return true
	}
	httpx.WriteError(w, http.StatusServiceUnavailable, "zalo_bot_unavailable", msgZaloBotUnavailable, "")
	return false
}

// recordZaloBotAct writes the display row AFTER comms answered — never before: a row written first
// could record an act that never happened. ITS FAILURE DOES NOT FAIL THE REQUEST: comms' own entry is
// the record, and answering 5xx now would tell the operator a token that is live was not saved
// (recordSecretForward's reasoning). Logged with the verb and the failure — the act has no secret.
func (h *operatorHandlers) recordZaloBotAct(r *http.Request, act domain.ZaloBotAct) {
	ctx := context.WithoutCancel(r.Context())
	if err := h.d.ZaloBotLog.RecordZaloBotAct(ctx, act, actorOf(r)); err != nil {
		h.d.Log.ErrorContext(ctx, "khu vận hành: comms đã thực hiện thao tác bot Zalo nhưng platform không ghi được dòng vết hiển thị",
			"event", "zalo_bot.display_row_missing", "action", act.Action, "outcome", act.Outcome, "err", err)
	}
}

func tsPtr(t *timestamppb.Timestamp) *time.Time {
	if t == nil {
		return nil
	}
	v := t.AsTime().UTC()
	return &v
}

// statusView maps SharedZaloBotStatus; false when it breaks the contract (absent, or no set_at/set_by).
func statusView(b *commsv1.SharedZaloBotStatus) (*zaloBotStatusView, bool) {
	if b == nil || b.GetSetAt() == nil || b.GetSetBy() == "" {
		return nil, false
	}
	return &zaloBotStatusView{SetAt: b.GetSetAt().AsTime().UTC(), SetBy: b.GetSetBy(), HasToken: b.GetHasToken(),
		BotName: b.GetBotName(), ChatURL: b.GetChatUrl()}, true
}

// sharedView maps GetSharedZaloBotResponse; false when it breaks the contract.
func sharedView(res *commsv1.GetSharedZaloBotResponse) (sharedZaloBotView, bool) {
	if !res.GetConfigured() {
		return sharedZaloBotView{Configured: false}, true
	}
	bot, ok := statusView(res.GetBot())
	if !ok {
		return sharedZaloBotView{}, false
	}
	v := sharedZaloBotView{Configured: true, Bot: bot}
	if c := res.GetLastCheck(); c != nil && c.GetCheckedAt() != nil {
		v.LastCheck = &zaloBotCheckView{CheckedAt: c.GetCheckedAt().AsTime().UTC(), Outcome: zaloOutcome(c.GetOutcome())}
	}
	return v, true
}

// --- handlers ------------------------------------------------------------------------------------------

func (h *operatorHandlers) getSharedZaloBot(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !h.zaloBotReady(w) {
		return
	}
	res, err := h.d.ZaloBot.GetSharedZaloBot(r.Context(), &commsv1.GetSharedZaloBotRequest{})
	if err != nil {
		h.zaloBotCallFailed(w, r, "GetSharedZaloBot", err)
		return
	}
	v, ok := sharedView(res)
	if !ok {
		h.zaloBotContractFault(w, r, "GetSharedZaloBot", "configured without complete bot metadata")
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// setSharedZaloBot forwards a new token with its display fields. See SetSharedZaloBot in the proto for
// the two steps and the relink confirmation.
func (h *operatorHandlers) setSharedZaloBot(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !h.zaloBotReady(w) {
		return
	}
	var b sharedZaloBotBody
	if !decodeBody(w, r, &b) {
		return
	}
	if !validZaloBotToken(b.Token.Secret) {
		httpx.WriteError(w, http.StatusUnprocessableEntity, "invalid_token", msgZaloBotTokenInvalid, "")
		return
	}
	name := normalizeZaloBotName(b.BotName)
	if name == "" {
		httpx.WriteError(w, http.StatusUnprocessableEntity, "invalid_bot_name", msgZaloBotNameInvalid, "")
		return
	}
	chatURL := normalizeZaloChatURL(b.ChatURL)
	if chatURL == "" {
		httpx.WriteError(w, http.StatusUnprocessableEntity, "invalid_chat_url", msgZaloBotChatURL, "")
		return
	}
	var expected int32
	if b.ExpectedRelinkCount != nil {
		n := *b.ExpectedRelinkCount
		if n < 0 || n > int64(^uint32(0)>>1) {
			httpx.WriteError(w, http.StatusUnprocessableEntity, "invalid_expected_relink_count", msgZaloBotRelinkCount, "")
			return
		}
		expected = int32(n)
	}
	actor := actorOf(r)
	res, err := h.d.ZaloBot.SetSharedZaloBot(r.Context(), &commsv1.SetSharedZaloBotRequest{
		Token:               string(b.Token.Lo()),
		BotName:             name,
		ChatUrl:             chatURL,
		ActorCode:           actor.Code,
		ActorIp:             actor.IP,
		ExpectedRelinkCount: expected,
	})
	if err != nil {
		h.zaloBotCallFailed(w, r, "SetSharedZaloBot", err)
		return
	}
	switch res.GetResult() {
	case commsv1.SetSharedZaloBotResult_SET_SHARED_ZALO_BOT_RESULT_TOKEN_CHECK_FAILED:
		writeJSON(w, http.StatusUnprocessableEntity, zaloBotTokenRefusedView{Code: codeTokenRefused,
			Message: msgZaloBotTokenRefused, Outcome: zaloOutcome(res.GetZaloOutcome())})
		return
	case commsv1.SetSharedZaloBotResult_SET_SHARED_ZALO_BOT_RESULT_RELINK_CONFIRMATION_REQUIRED:
		if res.GetLiveLinkCount() < 0 {
			h.zaloBotContractFault(w, r, "SetSharedZaloBot", "negative live_link_count")
			return
		}
		writeJSON(w, http.StatusConflict, zaloBotRelinkView{Code: codeRelinkRequired, Message: msgZaloBotRelink,
			LiveLinkCount: res.GetLiveLinkCount()})
		return
	case commsv1.SetSharedZaloBotResult_SET_SHARED_ZALO_BOT_RESULT_SAVED:
	default:
		// UNSPECIFIED: "treat it as not saved — fail closed" (proto).
		h.zaloBotContractFault(w, r, "SetSharedZaloBot", "result unspecified")
		return
	}

	ended := res.GetEndedLinkCount()
	h.recordZaloBotAct(r, domain.ZaloBotAct{Action: domain.ZaloBotActionTokenSet,
		Outcome: zaloOutcome(res.GetZaloOutcome()), BotName: name, ChatURL: chatURL, EndedLinkCount: ended})

	// SAVED: the token IS live, so nothing below may turn this answer into a failure. The answer is
	// re-read with GetSharedZaloBot so it carries the last check the save recorded (the Set response
	// has none); if that read fails, the Set response's own metadata is answered instead.
	v := sharedZaloBotView{Configured: true, EndedLinkCount: &ended}
	if bot, ok := statusView(res.GetBot()); ok {
		v.Bot = bot
	} else {
		h.d.Log.ErrorContext(r.Context(), "CẢNH BÁO HỢP ĐỒNG: comms trả SAVED không kèm siêu dữ liệu bot",
			"event", "zalo_bot.contract_fault", "rpc", "SetSharedZaloBot")
	}
	if fresh, err := h.d.ZaloBot.GetSharedZaloBot(context.WithoutCancel(r.Context()), &commsv1.GetSharedZaloBotRequest{}); err == nil {
		if fv, ok := sharedView(fresh); ok && fv.Configured {
			v.Bot, v.LastCheck = fv.Bot, fv.LastCheck
		}
	} else {
		h.d.Log.WarnContext(r.Context(), "khu vận hành: đã lưu token bot Zalo nhưng không đọc lại được trạng thái",
			"event", "zalo_bot.reread_failed", "grpc_code", status.Code(err).String())
	}
	writeJSON(w, http.StatusOK, v)
}

// checkSharedZaloBot asks comms to run getMe once with the live token. BODYLESS: no decodeBody.
func (h *operatorHandlers) checkSharedZaloBot(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !h.zaloBotReady(w) {
		return
	}
	actor := actorOf(r)
	res, err := h.d.ZaloBot.CheckSharedZaloBot(r.Context(), &commsv1.CheckSharedZaloBotRequest{
		ActorCode: actor.Code, ActorIp: actor.IP})
	if err != nil {
		h.zaloBotCallFailed(w, r, "CheckSharedZaloBot", err)
		return
	}
	out := zaloOutcome(res.GetOutcome())
	// A display row exactly when comms recorded the check (proto: `check` absent = nothing recorded,
	// which is NOT_CONFIGURED).
	if res.GetCheck() != nil {
		h.recordZaloBotAct(r, domain.ZaloBotAct{Action: domain.ZaloBotActionChecked, Outcome: out})
	}
	v := zaloBotCheckResultView{Outcome: out}
	if res.GetOutcome() == commsv1.ZaloBotCallOutcome_ZALO_BOT_CALL_OUTCOME_OK {
		v.AccountName = res.GetAccountName()
	}
	writeJSON(w, http.StatusOK, v)
}

func (h *operatorHandlers) getSharedZaloBotWebhook(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !h.zaloBotReady(w) {
		return
	}
	res, err := h.d.ZaloBot.GetSharedZaloBotWebhook(r.Context(), &commsv1.GetSharedZaloBotWebhookRequest{})
	if err != nil {
		h.zaloBotCallFailed(w, r, "GetSharedZaloBotWebhook", err)
		return
	}
	v := zaloBotWebhookView{Outcome: zaloOutcome(res.GetOutcome()), SecretSetAt: tsPtr(res.GetSecretSetAt()),
		SecretSetBy: res.GetSecretSetBy()}
	// "The fields below are set only when OK" (proto) — and only then shown.
	if res.GetOutcome() == commsv1.ZaloBotCallOutcome_ZALO_BOT_CALL_OUTCOME_OK {
		v.URL, v.URLMatches, v.ZaloUpdatedAt = res.GetUrl(), res.GetUrlMatches(), tsPtr(res.GetZaloUpdatedAt())
	}
	writeJSON(w, http.StatusOK, v)
}

// setSharedZaloBotWebhook asks comms to point the bot at itself. BODYLESS: comms builds the URL and a
// fresh secret_token (a caller-chosen URL could redirect every staff update). No decodeBody.
func (h *operatorHandlers) setSharedZaloBotWebhook(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !h.zaloBotReady(w) {
		return
	}
	actor := actorOf(r)
	res, err := h.d.ZaloBot.SetSharedZaloBotWebhook(r.Context(), &commsv1.SetSharedZaloBotWebhookRequest{
		ActorCode: actor.Code, ActorIp: actor.IP})
	if err != nil {
		h.zaloBotCallFailed(w, r, "SetSharedZaloBotWebhook", err)
		return
	}
	out := zaloOutcome(res.GetOutcome())
	v := zaloBotWebhookSetView{Outcome: out, URL: res.GetUrl()}
	if res.GetOutcome() == commsv1.ZaloBotCallOutcome_ZALO_BOT_CALL_OUTCOME_OK {
		v.SetAt, v.SetBy = tsPtr(res.GetSetAt()), res.GetSetBy()
	}
	// NOT_CONFIGURED: comms wrote nothing and called nobody, so there is no act to display.
	if res.GetOutcome() != commsv1.ZaloBotCallOutcome_ZALO_BOT_CALL_OUTCOME_NOT_CONFIGURED {
		h.recordZaloBotAct(r, domain.ZaloBotAct{Action: domain.ZaloBotActionWebhookSet, Outcome: out, WebhookURL: v.URL})
	}
	writeJSON(w, http.StatusOK, v)
}

// listZaloBotCommunes is the communes table: EVERY commune of the registry this service owns, joined
// with comms' uptake (absent = off, 0 linked, shared bot) and identity's active staff count (absent =
// 0). Both reads across communes are audited at their source; the registry list is metadata the user
// exempted (ADR 0048 §30/09 #5).
//
// Identity is asked with the operator's own `op1.` token — the realm the read belongs to — so a
// session that ended between opauth's check and this call is a 401, and a missing key a 403.
func (h *operatorHandlers) listZaloBotCommunes(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	if !h.zaloBotReady(w) {
		return
	}
	communes, err := h.allCommunes(r.Context())
	if err != nil {
		h.d.Log.ErrorContext(r.Context(), "khu vận hành: không đọc được danh sách xã cho bảng bot Zalo",
			"event", "zalo_bot.communes_failed", "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", msgCommuneListUnavail, "")
		return
	}

	tok, _ := opauth.TokenFrom(r)
	counts, err := h.d.Identity.ListActiveStaffCounts(r.Context(), tok, httpx.ClientIP(r))
	if err != nil {
		h.d.Log.WarnContext(r.Context(), "khu vận hành: identity không đếm được cán bộ đang hoạt động",
			"event", "zalo_bot.staff_counts_failed", "err_class", secretStatusErrClass(err))
		httpx.WriteError(w, http.StatusServiceUnavailable, "staff_counts_unavailable", msgStaffCountsUnavail, "")
		return
	}
	switch counts.Outcome {
	case operatorclient.OutcomeAccepted:
	case operatorclient.OutcomeSessionNotLive:
		opauth.ClearCookie(w)
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", msgSignInRefused, "")
		return
	case operatorclient.OutcomePermissionDenied:
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "Tài khoản vận hành của bạn không có quyền thực hiện thao tác này.", "")
		return
	default:
		httpx.WriteError(w, http.StatusServiceUnavailable, "staff_counts_unavailable", msgStaffCountsUnavail, "")
		return
	}

	actor := actorOf(r)
	stats, err := h.d.ZaloBot.ListZaloBotCommuneStats(r.Context(), &commsv1.ListZaloBotCommuneStatsRequest{
		ActorCode: actor.Code, ActorIp: actor.IP})
	if err != nil {
		h.zaloBotCallFailed(w, r, "ListZaloBotCommuneStats", err)
		return
	}
	byCommune := make(map[string]*commsv1.ZaloBotCommuneStats, len(stats.GetCommunes()))
	for _, s := range stats.GetCommunes() {
		if s.GetTenantId() == "" || byCommune[s.GetTenantId()] != nil {
			// Two rows for one commune: which count is true is not something to guess.
			h.zaloBotContractFault(w, r, "ListZaloBotCommuneStats", "empty or repeated tenant_id")
			return
		}
		byCommune[s.GetTenantId()] = s
	}

	out := zaloBotCommuneListView{Items: make([]zaloBotCommuneView, 0, len(communes))}
	for _, c := range communes {
		row := zaloBotCommuneView{TenantID: c.ID, CommuneName: c.Name, TotalStaffCount: counts.Counts[c.ID]}
		if s := byCommune[c.ID]; s != nil {
			row.ChannelEnabled = s.GetChannelEnabled()
			row.UsesOwnBot = s.GetUsesOwnBot()
			if row.UsesOwnBot {
				row.OwnBotName = s.GetOwnBotName()
				row.LinkedStaffCount = s.GetOwnBotLinkedStaffCount()
			} else {
				row.LinkedStaffCount = s.GetLinkedStaffCount()
			}
		}
		out.Items = append(out.Items, row)
	}
	writeJSON(w, http.StatusOK, out)
}

// allCommunes walks the registry list to its end, MaxLimit at a time, in its one order (creation
// time). Every commune, active or not: the table answers "which communes use the bot", and a commune
// switched off still has staff links comms keeps (rule 7).
func (h *operatorHandlers) allCommunes(ctx context.Context) ([]domain.Commune, error) {
	var out []domain.Commune
	cursor := ""
	for range maxCommunePagesZaloStat {
		req, err := page.New(store.CommuneOrder, "", "", strconv.Itoa(page.MaxLimit), cursor)
		if err != nil {
			return nil, fmt.Errorf("zalo bot communes: page: %w", err)
		}
		res, err := h.d.Registry.ListCommunes(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("zalo bot communes: %w", err)
		}
		out = append(out, res.Items...)
		if !res.HasMore {
			return out, nil
		}
		cursor = res.NextCursor
	}
	return nil, errors.New("zalo bot communes: registry list did not end")
}
