package http

// NOTICE CAMPAIGNS (ADR 0099 #4–#9; migration 0032) — web-admin "Gửi tin ZNS / SMS" (/gui-thong-bao):
//
//	GET  /api/v1/notice-templates             notice.send  what a campaign can be composed from
//	POST /api/v1/notice-campaign-previews     notice.send  "Xem trước" — counts, content, estimate; writes nothing
//	POST /api/v1/notice-campaigns             notice.send  "Gửi" — campaign + every recipient row, one transaction
//	GET  /api/v1/notice-campaigns             notice.send  the "Đã gửi" history, newest first, keyset pages
//	GET  /api/v1/notice-campaigns/{id}        notice.send  one campaign (its progress while it sends)
//
// `notice.send` — "Gửi thông báo tới người dân, cán bộ" — seeded by service-identity migration 0028.
//
// THE COMMUNE IS THE CONTEXT'S (rule 1): TenantMiddleware resolved it from Host before any handler here;
// the use case reads and writes through db.For(ctx). No route takes a commune from the client.
//
// NO PHONE NUMBER LEAVES HERE, masked or not: a campaign reply carries counts, never recipients (ADR 0099
// "Lệch prototype": no "Ví dụ người nhận"). No reply names another commune: an id of another commune is
// the same 404 as an id that does not exist.

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/crypto"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// NoticeCampaignUseCase is the use case. *app.NoticeCampaigns satisfies it. The commune is the context's
// (rule 1); the send opens its own transaction and writes its audit entry inside it (rule 6).
type NoticeCampaignUseCase interface {
	Templates(ctx context.Context) (app.NoticeTemplates, error)
	Preview(ctx context.Context, in app.NoticeInput) (app.NoticePreview, error)
	Send(ctx context.Context, in app.NoticeInput, actor audit.Actor) (app.NoticeCampaignItem, error)
	List(ctx context.Context, cursor string) ([]app.NoticeCampaignItem, string, error)
	Get(ctx context.Context, id string) (app.NoticeCampaignItem, error)
}

// NoticeCampaignDeps is everything the five routes touch.
type NoticeCampaignDeps struct {
	Checker   authz.Checker
	Campaigns NoticeCampaignUseCase
	// Names is identity's ResolveStaffNames (*identityclient.Client) — `created_by_name`. Display only: when
	// identity cannot be asked the names are "" and the screen shows the business code alone.
	Names app.StaffNames
	Log   *slog.Logger
}

type noticeCampaignHandler struct{ d NoticeCampaignDeps }

// noticeBodyCap — 50 parameters of up to 500 characters fit with room; the shared 64 KiB cap may not.
const noticeBodyCap = 128 << 10

// ---- wire shapes --------------------------------------------------------------------------------------

// noticeTemplateParamOut is one parameter of a template. `field` is what the request's `params` object is
// keyed by: the Zalo parameter NAME for ZNS, the slot POSITION ("1", "2", …) for SMS — an eSMS template
// may repeat a name with different values, so its slots are filled by position. `max_length` 0 = Zalo
// declared no bound.
type noticeTemplateParamOut struct {
	Field     string `json:"field"`
	Name      string `json:"name"`
	MaxLength int    `json:"max_length"`
}

// znsNoticeTemplateOut is one ENABLE ZBS template of one live OA. `oa_needs_reauthorization`: the OA must
// be authorized again before anything goes out through it (a preview / send answers 409 source_unavailable).
// `unit_price`: the ESTIMATED price of one message, decimal text ("200.00", VAT excluded).
type znsNoticeTemplateOut struct {
	Channel                string                   `json:"channel"`
	OAID                   string                   `json:"oa_id"`
	OAName                 string                   `json:"oa_name"`
	OANeedsReauthorization bool                     `json:"oa_needs_reauthorization"`
	TemplateRef            string                   `json:"template_ref"`
	TemplateName           string                   `json:"template_name"`
	Tag                    string                   `json:"tag"`
	Params                 []noticeTemplateParamOut `json:"params"`
	UnitPrice              *string                  `json:"unit_price"`
}

// smsNoticeTemplateOut is one template TEXT of one live eSMS account. `unit_price` is always null here:
// eSMS prices a FILLED text, so the estimate comes with the preview.
type smsNoticeTemplateOut struct {
	Channel      string                   `json:"channel"`
	SmsAccountID string                   `json:"sms_account_id"`
	AccountLabel string                   `json:"account_label"`
	Brandname    string                   `json:"brandname"`
	TemplateRef  string                   `json:"template_ref"`
	Content      string                   `json:"content"`
	Params       []noticeTemplateParamOut `json:"params"`
	UnitPrice    *string                  `json:"unit_price"`
}

type noticeTemplatesOut struct {
	ZNS []znsNoticeTemplateOut `json:"zns"`
	SMS []smsNoticeTemplateOut `json:"sms"`
}

// noticeAudienceIn — `kind` is an ADR 0011 value: nguoi-dan-tat-ca · nguoi-dan-theo-nhom (ids = citizen
// group ids) · nguoi-dan-theo-nhan (tags) · can-bo-tat-ca · can-bo-theo-bo-phan (ids = org unit ids).
// Exactly the list the kind uses is non-empty; at most 50.
type noticeAudienceIn struct {
	Kind string   `json:"kind"`
	IDs  []string `json:"ids"`
	Tags []string `json:"tags"`
}

// noticePreviewIn is the body of POST notice-campaign-previews. `channel` zalo-zns needs `oa_id`, sms needs
// `sms_account_id` — never both. `template_ref` is the `template_ref` of GET notice-templates. `params` is
// keyed by the template parameters' `field`; an empty value counts as missing.
type noticePreviewIn struct {
	Channel      string            `json:"channel"`
	OAID         string            `json:"oa_id,omitempty"`
	SmsAccountID string            `json:"sms_account_id,omitempty"`
	TemplateRef  string            `json:"template_ref"`
	Audience     noticeAudienceIn  `json:"audience"`
	Params       map[string]string `json:"params"`
}

// noticeCampaignIn is the body of POST notice-campaigns: the preview's body plus `title` (≤ 200).
type noticeCampaignIn struct {
	Title        string            `json:"title"`
	Channel      string            `json:"channel"`
	OAID         string            `json:"oa_id,omitempty"`
	SmsAccountID string            `json:"sms_account_id,omitempty"`
	TemplateRef  string            `json:"template_ref"`
	Audience     noticeAudienceIn  `json:"audience"`
	Params       map[string]string `json:"params"`
}

// noticePreviewOut — `content`: the SMS text with the typed values filled and the missing slots left as
// written ("{P2,10}"); null for ZNS (Zalo holds the template body). `recipient_count`: messages that would
// be sent (deduplicated by number). `skipped_count`: directory rows not reached ("ngừng liên hệ", no
// number). `unit_price` / `estimated_cost`: decimal text, ESTIMATE, null when none could be had (SMS:
// while a parameter is missing, or when eSMS's estimate did not answer). `missing_params`: the keys still
// empty — a send answers 422 until it is empty.
type noticePreviewOut struct {
	Content        *string  `json:"content"`
	RecipientCount int      `json:"recipient_count"`
	SkippedCount   int      `json:"skipped_count"`
	UnitPrice      *string  `json:"unit_price"`
	EstimatedCost  *string  `json:"estimated_cost"`
	MissingParams  []string `json:"missing_params"`
}

// noticeCampaignOut is one campaign. `status`: dang-gui · da-gui · gui-mot-phan · that-bai. `source_name`:
// the OA's name (ZNS) or "label · Brandname" (SMS). `skipped_count`: "Bỏ qua" — identity's skipped rows at
// the send, plus (once finished) the rows the sender skipped. `sent_count` / `failed_count` are partial
// while `status` is dang-gui. "Đã gửi" means the PROVIDER ACCEPTED the message (ADR 0099 #9), not that it
// reached the handset. `created_by` is the staff business code; `created_by_name` "" when identity could not
// be asked.
type noticeCampaignOut struct {
	ID            string     `json:"id"`
	Title         string     `json:"title"`
	Channel       string     `json:"channel"`
	OAID          *string    `json:"oa_id"`
	SmsAccountID  *string    `json:"sms_account_id"`
	SourceName    string     `json:"source_name"`
	TemplateRef   string     `json:"template_ref"`
	TemplateName  string     `json:"template_name"`
	AudienceKind  string     `json:"audience_kind"`
	AudienceIDs   []string   `json:"audience_ids"`
	Recipients    int        `json:"recipient_count"`
	Skipped       int        `json:"skipped_count"`
	Sent          int        `json:"sent_count"`
	Failed        int        `json:"failed_count"`
	UnitPrice     *string    `json:"unit_price"`
	EstimatedCost *string    `json:"estimated_cost"`
	Status        string     `json:"status"`
	CreatedAt     time.Time  `json:"created_at"`
	CreatedBy     string     `json:"created_by"`
	CreatedByName string     `json:"created_by_name"`
	FinishedAt    *time.Time `json:"finished_at"`
}

// noticeCampaignListOut — `next_cursor` "" on the last page; pass it back as `?cursor=`.
type noticeCampaignListOut struct {
	Items      []noticeCampaignOut `json:"items"`
	NextCursor string              `json:"next_cursor"`
}

func optText(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func noticeCampaignToOut(it app.NoticeCampaignItem, names map[string]string) noticeCampaignOut {
	c := it.Campaign
	ids := c.Audience.IDs
	if ids == nil {
		ids = []string{}
	}
	out := noticeCampaignOut{ID: c.ID, Title: c.Title, Channel: c.Channel, OAID: optText(c.OARowID),
		SmsAccountID: optText(c.SmsAccountID), SourceName: it.SourceName, TemplateRef: c.TemplateRef,
		TemplateName: c.TemplateName, AudienceKind: c.Audience.Kind, AudienceIDs: ids, Recipients: c.RecipientCount,
		Skipped: it.Skipped, Sent: c.SentCount, Failed: c.FailedCount, UnitPrice: optText(c.UnitPrice),
		EstimatedCost: optText(c.EstimatedCost), Status: c.Status, CreatedAt: c.CreatedAt, CreatedBy: c.CreatedBy,
		CreatedByName: names[c.CreatedBy]}
	if !c.FinishedAt.IsZero() {
		f := c.FinishedAt
		out.FinishedAt = &f
	}
	return out
}

// ---- routes -------------------------------------------------------------------------------------------

// RegisterNoticeCampaigns mounts the five routes on the staff mux. Refuses incomplete wiring at
// construction.
func RegisterNoticeCampaigns(mux *http.ServeMux, d NoticeCampaignDeps) {
	switch {
	case d.Checker == nil:
		panic("comms/http: thiếu authz.Checker — các tuyến gửi đợt tin sẽ không kiểm được quyền")
	case d.Campaigns == nil:
		panic("comms/http: thiếu use case gửi đợt tin — /api/v1/notice-campaigns sẽ panic khi có người gọi")
	case d.Names == nil:
		panic("comms/http: thiếu tra tên cán bộ — created_by_name sẽ panic")
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	h := &noticeCampaignHandler{d: d}

	// NO idem.* DECLARATION: a GET. No audit entry: the commune's own configuration, no personal data.
	//
	// @summary  Mẫu để soạn đợt tin — mọi mẫu ZBS đang ENABLE của mọi OA đang nối của xã (kèm đơn giá ước tính theo loại mẫu) và mọi mẫu tin CSKH eSMS còn liệt kê của mọi tài khoản SMS đang dùng (mỗi nội dung một mục); mỗi mẫu kèm các tham số cần gõ và `field` để điền
	// @screen   ADR 0099 #4 — Gửi tin ZNS / SMS, ô chọn mẫu
	// @reply    200 noticeTemplatesOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/notice-templates",
		authz.RequirePermission(d.Checker, "notice.send")(
			http.HandlerFunc(h.Templates)))

	// idem.KhongCan: a preview writes nothing — asked twice, it answers twice.
	//
	//	400 notice_invalid              a field is malformed (channel, source, audience, a value too long)
	//	404 not_found                   no such live OA / SMS account / template in the commune
	//	409 template_not_sendable       the ZBS template is not ENABLE, or eSMS no longer lists the text
	//	409 source_unavailable          the OA needs re-authorization
	//	503 identity_unavailable        the recipient count could not be loaded
	//	503 encryption_not_configured   the platform cannot open the eSMS keys (SMS only)
	//
	// @summary  Xem trước đợt tin — số người nhận (đã khử trùng theo số), số bỏ qua (ngừng liên hệ, không có số), nội dung SMS đã điền (chỗ chưa gõ giữ nguyên), các tham số còn thiếu, đơn giá và chi phí dự kiến (ước tính, chưa VAT); không ghi gì, không vết, không lộ số điện thoại nào
	// @screen   ADR 0099 #7 — Gửi tin ZNS / SMS, nút "Xem trước"
	// @request  noticePreviewIn
	// @reply    200 noticePreviewOut
	// @reply    400 httpx.Error notice_invalid
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error not_found
	// @reply    409 httpx.Error template_not_sendable source_unavailable
	// @reply    503 httpx.Error identity_unavailable encryption_not_configured
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/notice-campaign-previews",
		authz.RequirePermission(d.Checker, "notice.send")(
			idem.KhongCan("xem trước chỉ đọc — không ghi gì, hỏi lại bao nhiêu lần cũng chỉ trả lại con số")(
				http.HandlerFunc(h.Preview))))

	// idem.Required(DongKhiHong): a repeated POST would write a SECOND campaign and send every message
	// twice — real money and a confused citizen. Closed when the idempotency store is down.
	//
	//	400 notice_invalid              a field is malformed, or the title is missing
	//	404 not_found                   no such live OA / SMS account / template in the commune
	//	409 no_recipients               the audience reaches nobody; nothing written
	//	409 template_not_sendable       the ZBS template is not ENABLE, or eSMS no longer lists the text
	//	409 source_unavailable          the OA needs re-authorization
	//	422 missing_params              a template parameter is still empty; nothing written
	//	503 identity_unavailable        the recipients could not be loaded; nothing written
	//	503 encryption_not_configured   the platform cannot open the eSMS keys (SMS only)
	//
	// @summary  Gửi đợt tin ZNS / SMS — ghi đợt (dang-gui) cùng MỌI dòng người nhận (mã dòng danh bạ + số đã che, không bao giờ số đầy đủ) và vết trong MỘT giao dịch; việc nền gửi theo lô sau đó; identity không trả lời thì không ghi gì; chi phí là ước tính, không chặn gửi
	// @screen   ADR 0099 #8 — Gửi tin ZNS / SMS, nút "Gửi"
	// @request  noticeCampaignIn
	// @reply    201 noticeCampaignOut
	// @reply    400 httpx.Error notice_invalid
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error not_found
	// @reply    409 httpx.Error no_recipients template_not_sendable source_unavailable
	// @reply    422 httpx.Error missing_params
	// @reply    503 httpx.Error identity_unavailable encryption_not_configured
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/notice-campaigns",
		authz.RequirePermission(d.Checker, "notice.send")(
			idem.Required(idem.DongKhiHong)(
				http.HandlerFunc(h.Send))))

	// NO idem.* DECLARATION: a GET. No audit entry: the commune's own history; no recipient in it.
	//
	// @summary  Lịch sử đợt tin đã gửi của xã — mới nhất trước, 50 đợt mỗi trang (con trỏ `cursor`); mỗi đợt: kênh, nguồn gửi, mẫu, đối tượng, số người nhận / đã gửi / thất bại / bỏ qua, chi phí ước tính, trạng thái, ai gửi lúc nào
	// @screen   ADR 0099 #8 — Gửi tin ZNS / SMS, tab "Đã gửi"
	// @reply    200 noticeCampaignListOut
	// @reply    400 httpx.Error notice_invalid
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/notice-campaigns",
		authz.RequirePermission(d.Checker, "notice.send")(
			http.HandlerFunc(h.List)))

	// NO idem.* DECLARATION: a GET.
	//
	// @summary  Một đợt tin của xã — tiến độ gửi (số đã gửi / thất bại tăng dần khi đang gửi), trạng thái, chi phí ước tính; mã đợt của xã khác trả 404 như mã không có
	// @screen   ADR 0099 #8 — Gửi tin ZNS / SMS, chi tiết đợt
	// @reply    200 noticeCampaignOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error not_found
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/notice-campaigns/{id}",
		authz.RequirePermission(d.Checker, "notice.send")(
			http.HandlerFunc(h.Get)))
}

// ---- handlers -----------------------------------------------------------------------------------------

// Templates — GET /api/v1/notice-templates
func (h *noticeCampaignHandler) Templates(w http.ResponseWriter, r *http.Request) {
	t, err := h.d.Campaigns.Templates(r.Context())
	if err != nil {
		h.fail(w, r, "đọc mẫu", err)
		return
	}
	out := noticeTemplatesOut{ZNS: make([]znsNoticeTemplateOut, 0, len(t.ZNS)), SMS: make([]smsNoticeTemplateOut, 0, len(t.SMS))}
	for _, z := range t.ZNS {
		params := make([]noticeTemplateParamOut, 0, len(z.Params))
		for _, p := range z.Params {
			params = append(params, noticeTemplateParamOut{Field: p.Name, Name: p.Name, MaxLength: p.MaxLength})
		}
		out.ZNS = append(out.ZNS, znsNoticeTemplateOut{Channel: domain.NoticeChannelZNS, OAID: z.OARowID, OAName: z.OAName,
			OANeedsReauthorization: z.OANeedsReauthorization, TemplateRef: z.TemplateID, TemplateName: z.TemplateName,
			Tag: z.Tag, Params: params, UnitPrice: optText(z.UnitPrice)})
	}
	for _, s := range t.SMS {
		params := make([]noticeTemplateParamOut, 0, len(s.Params))
		for i, p := range s.Params {
			params = append(params, noticeTemplateParamOut{Field: domain.SmsSlotKey(i), Name: p.Name, MaxLength: p.MaxLen})
		}
		out.SMS = append(out.SMS, smsNoticeTemplateOut{Channel: domain.NoticeChannelSMS, SmsAccountID: s.SmsAccountID,
			AccountLabel: s.AccountLabel, Brandname: s.Brandname, TemplateRef: s.TemplateRef, Content: s.Content, Params: params})
	}
	vietJSON(w, http.StatusOK, out)
}

func previewInput(in noticePreviewIn) app.NoticeInput {
	return app.NoticeInput{Channel: in.Channel, OARowID: in.OAID, SmsAccountID: in.SmsAccountID, TemplateRef: in.TemplateRef,
		AudienceKind: in.Audience.Kind, AudienceIDs: in.Audience.IDs, AudienceTags: in.Audience.Tags, Params: in.Params}
}

// Preview — POST /api/v1/notice-campaign-previews
func (h *noticeCampaignHandler) Preview(w http.ResponseWriter, r *http.Request) {
	var in noticePreviewIn
	if !readBodyCapped(w, r, &in, noticeBodyCap) {
		return
	}
	p, err := h.d.Campaigns.Preview(r.Context(), previewInput(in))
	if err != nil {
		h.fail(w, r, "xem trước", err)
		return
	}
	missing := p.MissingParams
	if missing == nil {
		missing = []string{}
	}
	vietJSON(w, http.StatusOK, noticePreviewOut{Content: p.Content, RecipientCount: p.RecipientCount, SkippedCount: p.SkippedCount,
		UnitPrice: optText(p.UnitPrice), EstimatedCost: optText(p.EstimatedCost), MissingParams: missing})
}

// Send — POST /api/v1/notice-campaigns
func (h *noticeCampaignHandler) Send(w http.ResponseWriter, r *http.Request) {
	var in noticeCampaignIn
	if !readBodyCapped(w, r, &in, noticeBodyCap) {
		return
	}
	actor, ok := nguoiThucHien(r)
	if !ok {
		h.d.Log.Error("tuyến gửi đợt tin chạy mà không có chủ thể mang mã cán bộ",
			"xa", string(tenant.MustFrom(r.Context())), "duong", r.URL.Path)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
		return
	}
	input := previewInput(noticePreviewIn{Channel: in.Channel, OAID: in.OAID, SmsAccountID: in.SmsAccountID,
		TemplateRef: in.TemplateRef, Audience: in.Audience, Params: in.Params})
	input.Title = in.Title
	it, err := h.d.Campaigns.Send(r.Context(), input, actor)
	if err != nil {
		h.fail(w, r, "gửi", err)
		return
	}
	vietJSON(w, http.StatusCreated, noticeCampaignToOut(it, h.staffNames(r.Context(), []string{it.Campaign.CreatedBy})))
}

// List — GET /api/v1/notice-campaigns
func (h *noticeCampaignHandler) List(w http.ResponseWriter, r *http.Request) {
	cursor := r.URL.Query().Get("cursor")
	// The commune is the context's (TenantMiddleware, from Host): the use case reads through db.For(ctx),
	// tenant_id = $1 — the cursor positions inside that commune's list, it names no commune.
	items, next, err := h.d.Campaigns.List(r.Context(), cursor)
	if err != nil {
		h.fail(w, r, "đọc lịch sử", err)
		return
	}
	codes := make([]string, 0, len(items))
	for _, it := range items {
		codes = append(codes, it.Campaign.CreatedBy)
	}
	names := h.staffNames(r.Context(), codes)
	out := noticeCampaignListOut{Items: make([]noticeCampaignOut, 0, len(items)), NextCursor: next}
	for _, it := range items {
		out.Items = append(out.Items, noticeCampaignToOut(it, names))
	}
	vietJSON(w, http.StatusOK, out)
}

// Get — GET /api/v1/notice-campaigns/{id}
func (h *noticeCampaignHandler) Get(w http.ResponseWriter, r *http.Request) {
	// The commune is the context's (TenantMiddleware, from Host): the use case reads through db.For(ctx),
	// tenant_id = $1 — another commune's id is the same 404 as an unknown one.
	it, err := h.d.Campaigns.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		h.fail(w, r, "đọc đợt", err)
		return
	}
	vietJSON(w, http.StatusOK, noticeCampaignToOut(it, h.staffNames(r.Context(), []string{it.Campaign.CreatedBy})))
}

// staffNames resolves business codes to names. Display only: identity failing leaves every name "" and
// the screen shows the code. Names are personal data — never logged.
func (h *noticeCampaignHandler) staffNames(ctx context.Context, codes []string) map[string]string {
	out := map[string]string{}
	var uniq []string
	seen := map[string]bool{}
	for _, c := range codes {
		if c != "" && !seen[c] {
			seen[c] = true
			uniq = append(uniq, c)
		}
	}
	for start := 0; start < len(uniq); start += identityclient.TranMaMotLo {
		end := min(start+identityclient.TranMaMotLo, len(uniq))
		got, err := h.d.Names.TenCanBoTheoMa(ctx, uniq[start:end])
		if err != nil {
			h.d.Log.Warn("gửi đợt tin: không tra được tên cán bộ — trả mã, tên để trống",
				"xa", string(tenant.MustFrom(ctx)), "err", err)
			return map[string]string{}
		}
		for k, v := range got {
			out[k] = v.HoTen
		}
	}
	return out
}

// noticeRefusals — the 400s, one fixed sentence per sentinel (the domain text never reaches the body).
var noticeRefusals = []refusal{
	{domain.ErrNoticeChannel, "Kênh gửi phải là Zalo ZNS hoặc SMS."},
	{domain.ErrNoticeSource, "Gửi ZNS cần chọn một OA; gửi SMS cần chọn một tài khoản SMS — không chọn cả hai."},
	{domain.ErrNoticeTemplate, "Cần chọn mẫu tin."},
	{domain.ErrNoticeAudience, "Đối tượng nhận không hợp lệ: chọn một kiểu gửi, và chọn nhóm / nhãn / bộ phận khi kiểu ấy cần (tối đa 50)."},
	{domain.ErrNoticeTitle, "Cần nhập tiêu đề đợt gửi (tối đa 200 ký tự)."},
	{domain.ErrNoticeParamLong, "Có giá trị tham số dài hơn mức mẫu cho phép."},
	{domain.ErrNoticeParams, "Giá trị tham số không hợp lệ: không xuống dòng, không chứa dấu ngoặc nhọn, tối đa 500 ký tự."},
	{domain.ErrNoticeCursor, "Con trỏ trang không hợp lệ. Hãy tải lại danh sách từ đầu."},
}

// fail maps a use-case error to its reply. The wrapped error never reaches the client (rule 3), and never
// names another commune.
func (h *noticeCampaignHandler) fail(w http.ResponseWriter, r *http.Request, op string, err error) {
	xa := string(tenant.MustFrom(r.Context()))
	var missing *app.NoticeMissingParamsError
	switch {
	case errors.As(err, &missing):
		// Parameter KEYS only (names / positions of the template), never a value.
		httpx.WriteError(w, http.StatusUnprocessableEntity, "missing_params",
			"Còn tham số chưa nhập: "+strings.Join(missing.Missing, ", ")+". Chưa có gì được gửi.", "")
	case errors.Is(err, app.ErrNoticeSourceMissing):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy OA hoặc tài khoản SMS đang dùng này của xã.", "")
	case errors.Is(err, app.ErrNoticeTemplateMissing):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy mẫu tin này của nguồn gửi đã chọn.", "")
	case errors.Is(err, app.ErrNoticeCampaignNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy đợt tin này của xã.", "")
	case errors.Is(err, app.ErrNoticeNoRecipients):
		httpx.WriteError(w, http.StatusConflict, "no_recipients",
			"Đối tượng đã chọn không có ai nhận được tin. Chưa có gì được gửi.", "")
	case errors.Is(err, app.ErrNoticeTemplateNotSendable):
		httpx.WriteError(w, http.StatusConflict, "template_not_sendable",
			"Mẫu này hiện không gửi được (chưa được duyệt, đã tắt, hoặc nhà cung cấp không còn liệt kê). Hãy nạp lại mẫu hoặc chọn mẫu khác.", "")
	case errors.Is(err, app.ErrNoticeSourceUnavailable):
		httpx.WriteError(w, http.StatusConflict, "source_unavailable",
			"OA này cần được uỷ quyền lại trước khi gửi. Vào Cấu hình → Kênh Zalo để uỷ quyền lại.", "")
	case errors.Is(err, app.ErrNoticeIdentityUnavailable):
		h.d.Log.Warn("gửi đợt tin: không hỏi được identity khi "+op, "xa", xa, "err", err)
		httpx.WriteError(w, http.StatusServiceUnavailable, "identity_unavailable",
			"Chưa tải được danh sách người nhận. Vui lòng thử lại sau ít phút. Chưa có gì được gửi.", "")
	case errors.Is(err, crypto.ErrNotConfigured):
		httpx.WriteError(w, http.StatusServiceUnavailable, "encryption_not_configured",
			"Nền tảng chưa cấu hình khoá mã hoá bí mật (SECRET_ENCRYPTION_KEYS), nên chưa mở được khoá eSMS của xã. "+
				"Chưa có gì được gửi. Hãy báo đơn vị vận hành hệ thống.", "")
	default:
		if msg, ok := refusalMessage(noticeRefusals, err); ok {
			h.d.Log.Info("gửi đợt tin: từ chối "+op, "xa", xa, "err", err)
			httpx.WriteError(w, http.StatusBadRequest, "notice_invalid", msg, "")
			return
		}
		h.d.Log.Error("gửi đợt tin: "+op+" lỗi hệ thống", "xa", xa, "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}
