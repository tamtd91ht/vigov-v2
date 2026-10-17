package http

// A COMMUNE'S ZALO OAs (ADR 0098 #7, block A; migration 0031; SEVERAL per commune since ADR 0099 #1, #10
// and migration 0032) — Cấu hình → Kênh Zalo, "OA Zalo của xã" and "Mẫu tin ZBS":
//
//	GET    /api/v1/zalo-oas                          admin.zalo  every live OA of the commune
//	POST   /api/v1/zalo-oas/authorization-links      admin.zalo  "Kết nối OA" — a one-time Zalo link
//	DELETE /api/v1/zalo-oas/{id}                     admin.zalo  "Ngắt OA" — retire ONE OA, with a reason
//	POST   /api/v1/zalo-oas/{id}/check               admin.zalo  "Kiểm tra" — getoa, result recorded
//	GET    /api/v1/zalo-oas/{id}/templates           admin.zalo  the templates loaded from that OA
//	POST   /api/v1/zalo-oas/{id}/templates/sync      admin.zalo  "Nạp mẫu từ OA"
//	PATCH  /api/v1/zalo-oa-templates/{id}            admin.zalo  attach / detach a milestone
//
// The names are the owner's (ADR 0099 #10, owns_facts; ubiquitous-language.md "OA Zalo của xã").
// `zalo-oas/current/*` — one OA per commune — is GONE: a commune holds several OAs at once, so there is no
// "current" one, and every act names its OA by row id.
//
// `admin.zalo` — "Cấu hình kênh Zalo" — the Kênh Zalo tab's key (service-identity migration 0032), the
// same key as zalo-bots/current and zalo-groups. The commune comes from Host; the actor from the session.
//
// AN {id} THAT IS NOT THIS COMMUNE'S IS 404 not_found — the same sentence for "no such OA / template",
// "retired" and "another commune's": the scoped read cannot see another commune's row, and the reply must
// not tell them apart either (rule 1; rule 4 forbidden #2 in spirit).
//
// NOTHING SECRET LEAVES HERE. No reply type has a field for a token or a code verifier (ADR 0098 #5). The
// one sensitive value is the authorization link's `state` — inside the URL the OA admin must open — sent
// once, with Cache-Control: no-store.

import (
	"context"
	"encoding/json"
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
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// CommuneZaloOAAdmin is the use case. *app.CommuneZaloOAs satisfies it. Every write opens its own
// transaction and writes its audit entry inside it (rule 6, invariant 3).
type CommuneZaloOAAdmin interface {
	List(ctx context.Context) (app.CommuneZaloOAList, error)
	StartAuthorization(ctx context.Context, actor audit.Actor) (app.AuthorizationLink, error)
	Check(ctx context.Context, oaRowID string, actor audit.Actor) (app.ZaloOACheckOutput, error)
	Disconnect(ctx context.Context, oaRowID, reason string, actor audit.Actor) (app.DisconnectZaloOAOutput, error)
	ListTemplates(ctx context.Context, oaRowID string) ([]domain.ZaloOATemplate, error)
	SyncTemplates(ctx context.Context, oaRowID string, actor audit.Actor) (app.TemplateSyncResult, error)
	SetMilestone(ctx context.Context, templateRowID string, milestone *string, actor audit.Actor) (domain.ZaloOATemplate, error)
}

// ZaloCommuneOADeps is everything the seven routes touch.
type ZaloCommuneOADeps struct {
	Checker authz.Checker
	OAs     CommuneZaloOAAdmin
	// Names is identity's ResolveStaffNames (*identityclient.Client) — `authorized_by_name` and
	// `milestone_set_by_name`. Display only: when identity cannot be asked the names are "" and the
	// screen shows the business code alone (zalo-oa-form.ts `who`), never a refusal of the whole read.
	Names app.StaffNames
	Log   *slog.Logger
}

type zaloCommuneOAHandler struct{ d ZaloCommuneOADeps }

// ---- shapes (web-admin/src/lib/api/zalo-oa.ts) ------------------------------------------------------

// zaloOAReauthorizationOut — why the OA must be authorized again (ADR 0098 #6).
type zaloOAReauthorizationOut struct {
	Since  time.Time `json:"since"`
	Reason string    `json:"reason"`
}

// zaloOALastCheckOut — `result` is one of the seven ZaloCall* classes, never Zalo's words.
type zaloOALastCheckOut struct {
	At     time.Time `json:"at"`
	Result string    `json:"result"`
}

// communeZaloOAOut is one live OA as the screen sees it: names, ids, instants, classes, and its template
// counts. `id` is the row id every other route names. No token field exists on this shape, by contract.
type communeZaloOAOut struct {
	ID                   string                    `json:"id"`
	OAID                 string                    `json:"oa_id"`
	OAName               string                    `json:"oa_name"`
	AuthorizedAt         time.Time                 `json:"authorized_at"`
	AuthorizedBy         string                    `json:"authorized_by"`
	AuthorizedByName     string                    `json:"authorized_by_name"`
	AccessExpiresAt      time.Time                 `json:"access_expires_at"`
	TokenRefreshedAt     time.Time                 `json:"token_refreshed_at" apidoc:"bi-mat-co-chu-y:KHÔNG phải bí mật — chỉ là THỜI ĐIỂM token được ghi lần cuối (ADR 0098 §Hai cột thời điểm), không mang giá trị token nào; tên do hợp đồng web-admin TASK-A6 đặt"`
	NeedsReauthorization *zaloOAReauthorizationOut `json:"needs_reauthorization"`
	LastCheck            *zaloOALastCheckOut       `json:"last_check"`
	TemplateCount        int                       `json:"template_count"`
	AttachedCount        int                       `json:"attached_count"`
}

// communeZaloOAListOut is GET zalo-oas: every live OA, oldest first; `items` is [] when none.
// `configured`: the platform declared its shared Zalo App (ZALO_OA_* set) — "Kết nối OA" is usable.
type communeZaloOAListOut struct {
	Configured bool               `json:"configured"`
	Items      []communeZaloOAOut `json:"items"`
}

// zaloOAAuthorizationLinkOut is POST authorization-links, 201.
type zaloOAAuthorizationLinkOut struct {
	URL       string    `json:"url" apidoc:"bi-mat-co-chu-y:ADR 0098 #4 #7 — chủ dự án chốt 16/10/2026: link uỷ quyền của Zalo mang state CSPRNG một lần, sống 10 phút; trả MỘT lần cho cán bộ vừa bấm Kết nối OA để mở ngay, Cache-Control no-store, không lưu, không ghi log"`
	ExpiresAt time.Time `json:"expires_at"`
}

// zaloOACheckResultOut is POST check, 200. `oa_name` on success only.
type zaloOACheckResultOut struct {
	Result    string    `json:"result"`
	CheckedAt time.Time `json:"checked_at"`
	OAName    string    `json:"oa_name,omitempty"`
}

// communeZaloOADisconnectIn is the body of DELETE — rule 7: a soft delete carries its reason.
type communeZaloOADisconnectIn struct {
	Reason string `json:"reason"`
}

// communeZaloOADisconnectedOut is DELETE, 200. `disconnected` is always true on a 200 — an id that is not
// a live OA of the commune is 404 — and kept so the reply reads on its own.
type communeZaloOADisconnectedOut struct {
	Disconnected          bool   `json:"disconnected"`
	DetachedTemplateCount int    `json:"detached_template_count"`
	RevokeNotice          string `json:"revoke_notice"`
}

// zaloOATemplateParamOut is one parameter a template declares on Zalo's portal.
type zaloOATemplateParamOut struct {
	Name      string `json:"name"`
	Required  bool   `json:"required"`
	Type      string `json:"type"`
	MaxLength int    `json:"max_length"`
}

// zaloOATemplateOut is one template loaded from an OA. `oa_row_id` is the OA it belongs to (the `id` of
// GET zalo-oas). `milestone_set_by` is the attaching officer's business code.
type zaloOATemplateOut struct {
	ID                 string                   `json:"id"`
	OARowID            string                   `json:"oa_row_id"`
	TemplateID         string                   `json:"template_id"`
	TemplateName       string                   `json:"template_name"`
	ZaloStatus         string                   `json:"zalo_status"`
	ZaloQuality        string                   `json:"zalo_quality"`
	ZaloTag            string                   `json:"zalo_tag"`
	Params             []zaloOATemplateParamOut `json:"params"`
	ParamsMatch        bool                     `json:"params_match"`
	MissingParams      []string                 `json:"missing_params"`
	Milestone          *string                  `json:"milestone"`
	MilestoneSetAt     *time.Time               `json:"milestone_set_at"`
	MilestoneSetBy     *string                  `json:"milestone_set_by"`
	MilestoneSetByName *string                  `json:"milestone_set_by_name"`
}

// zaloOATemplateListOut is GET templates. `synced_at` null = never loaded from the OA.
type zaloOATemplateListOut struct {
	Items    []zaloOATemplateOut `json:"items"`
	SyncedAt *time.Time          `json:"synced_at"`
}

// zaloOASyncResultOut is POST templates/sync, 200. `changed`: templates new, or whose Zalo status moved.
type zaloOASyncResultOut struct {
	Loaded   int       `json:"loaded"`
	Enabled  int       `json:"enabled"`
	Matching int       `json:"matching"`
	Changed  int       `json:"changed"`
	SyncedAt time.Time `json:"synced_at"`
}

// zaloOATemplatePatchIn is the body of PATCH zalo-oa-templates/{id}: `milestone` is ALWAYS a key — null detaches.
// An absent key is refused (400), never read as "detach".
type zaloOATemplatePatchIn struct {
	Milestone *string `json:"milestone"`
	present   bool
}

// UnmarshalJSON tells `{"milestone": null}` (detach) from `{}` (refused), and refuses any other key.
func (p *zaloOATemplatePatchIn) UnmarshalJSON(b []byte) error {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	for k := range m {
		if k != "milestone" {
			return errors.New("unknown field")
		}
	}
	raw, ok := m["milestone"]
	if !ok {
		return nil
	}
	p.present = true
	return json.Unmarshal(raw, &p.Milestone)
}

// RegisterZaloCommuneOA mounts the seven routes on the staff mux. Refuses incomplete wiring at construction.
func RegisterZaloCommuneOA(mux *http.ServeMux, d ZaloCommuneOADeps) {
	switch {
	case d.Checker == nil:
		panic("comms/http: thiếu authz.Checker — các tuyến OA Zalo của xã sẽ không kiểm được quyền")
	case d.OAs == nil:
		panic("comms/http: thiếu use case OA Zalo của xã — /api/v1/zalo-oas sẽ panic khi có người gọi")
	case d.Names == nil:
		panic("comms/http: thiếu bộ tra tên cán bộ (identity) — authorized_by_name sẽ panic")
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	h := &zaloCommuneOAHandler{d: d}

	// NO idem.* DECLARATION: a GET. No audit entry: the configuration read in its own commune, no secret.
	//
	// @summary  Các OA Zalo của xã — nền tảng đã khai ứng dụng Zalo dùng chung chưa; mọi OA đang nối (nhiều OA cùng lúc): mã dòng, tên và mã OA, nối lúc nào bởi ai, token hết hạn lúc nào, có cần uỷ quyền lại không, lần kiểm gần nhất, số mẫu đã nạp và đã gắn mốc của từng OA; không bao giờ trả token
	// @screen   ADR 0099 #1 #10 — Cấu hình → tab Kênh Zalo, mục "OA Zalo của xã"
	// @reply    200 communeZaloOAListOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/zalo-oas",
		authz.RequirePermission(d.Checker, "admin.zalo")(
			http.HandlerFunc(h.GetList)))

	// idem.KhongCan: every call is a NEW link with its own state; a repeat leaves two open links, each
	// valid for its 10 minutes and each single-use — no state is ever handed out twice. Connecting a
	// second OA adds it beside the first (ADR 0099 #1); authorizing an OA already connected here renews it.
	//
	//	503 zalo_oa_not_configured      the platform has not declared its shared Zalo App (ZALO_OA_*)
	//	503 encryption_not_configured   the platform cannot seal the code verifier
	//
	// @summary  Tạo link uỷ quyền OA — sinh state CSPRNG một lần (chỉ lưu băm) và cặp PKCE (code verifier niêm bằng khoá của xã), trả link Zalo để quản trị viên OA mở và bấm Cho phép; OA mới được thêm cạnh các OA đã nối, OA đã nối thì được uỷ quyền lại; link sống 10 phút, có ghi vết; Cache-Control no-store
	// @screen   ADR 0098 #4 #7, ADR 0099 #1 — Cấu hình → tab Kênh Zalo, "Kết nối OA" / "Uỷ quyền lại"
	// @reply    201 zaloOAAuthorizationLinkOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    503 httpx.Error zalo_oa_not_configured encryption_not_configured
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/zalo-oas/authorization-links",
		authz.RequirePermission(d.Checker, "admin.zalo")(
			idem.KhongCan("mỗi lần gọi là một link mới với state riêng; gửi lại chỉ để thêm một link mở cũng sống 10 phút và chỉ dùng được một lần, không state nào bị trao hai lần")(
				http.HandlerFunc(h.PostAuthorizationLink))))

	// DELETE with a body: rule 7 — the retirement carries its reason. idem.KhongCan: a second DELETE of
	// the same id finds no live OA, writes nothing and answers 404.
	//
	//	404 not_found   the id is not a live OA of this commune (unknown, retired, or another commune's)
	//
	// @summary  Ngắt một OA của xã — xoá mềm dòng OA (giữ token đã niêm làm lịch sử, cần lý do), gỡ mọi mốc đã gắn mẫu của OA ấy trong cùng giao dịch, có ghi vết; các OA khác của xã giữ nguyên; câu trả lời nhắc xã thu hồi quyền của ứng dụng trong OA Manager vì ngắt ở đây không thu hồi gì ở Zalo
	// @screen   ADR 0098 #11, ADR 0099 #1 — Cấu hình → tab Kênh Zalo, "Ngắt OA"
	// @request  communeZaloOADisconnectIn
	// @reply    200 communeZaloOADisconnectedOut
	// @reply    400 httpx.Error invalid_request
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error not_found
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/zalo-oas/{id}",
		authz.RequirePermission(d.Checker, "admin.zalo")(
			idem.KhongCan("lần thứ hai cùng mã OA không còn gặp OA đang sống nên trả 404, không ghi gì và không có vết")(
				http.HandlerFunc(h.DeleteOA))))

	// idem.KhongCan: a check asks Zalo and records the latest answer; a second check is another check.
	//
	//	404 not_found                       the id is not a live OA of this commune
	//	409 zalo_oa_needs_reauthorization   the refresh token was refused or aged out
	//
	// @summary  Kiểm tra một OA của xã — gọi getoa bằng token đã lưu của OA ấy (làm mới trước nếu sắp hết hạn), ghi kết quả (lớp lỗi, không bao giờ chữ của Zalo), cập nhật tên OA và ghi vết
	// @screen   ADR 0098 #7, ADR 0099 #1 — Cấu hình → tab Kênh Zalo, "Kiểm tra kết nối"
	// @reply    200 zaloOACheckResultOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error not_found
	// @reply    409 httpx.Error zalo_oa_needs_reauthorization zalo_oa_changed
	// @reply    502 httpx.Error zalo_unavailable
	// @reply    503 httpx.Error zalo_oa_not_configured encryption_not_configured
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/zalo-oas/{id}/check",
		authz.RequirePermission(d.Checker, "admin.zalo")(
			idem.KhongCan("kiểm tra chỉ hỏi Zalo rồi ghi kết quả mới nhất; lần thứ hai là một lần kiểm khác, không đổi gì khác")(
				http.HandlerFunc(h.PostCheck))))

	// NO idem.* DECLARATION: a GET.
	//
	//	404 not_found   the id is not a live OA of this commune
	//
	// @summary  Mẫu tin ZBS đã nạp từ một OA của xã — mã và tên mẫu, trạng thái và chất lượng Zalo, tham số, có đủ ba tham số bắt buộc không (thiếu gì), mốc đã gắn, gắn lúc nào bởi ai; lần nạp gần nhất
	// @screen   ADR 0098 #8 #9, ADR 0099 #1 — Cấu hình → tab Kênh Zalo, mục "Mẫu tin ZBS"
	// @reply    200 zaloOATemplateListOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error not_found
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/zalo-oas/{id}/templates",
		authz.RequirePermission(d.Checker, "admin.zalo")(
			http.HandlerFunc(h.GetTemplates)))

	// idem.KhongCan: a sync rewrites Zalo's columns from Zalo's answer and keeps the commune's milestones;
	// a second sync is another read of the same OA.
	//
	//	404 not_found             the id is not a live OA of this commune
	//	422 zalo_token_rejected   Zalo refused the OA's token while listing; nothing written
	//	502 zalo_unavailable      Zalo could not be asked; nothing written
	//
	// @summary  Nạp mẫu từ một OA — gọi template/all rồi template/info từng mẫu, ghi mọi mẫu trong một giao dịch (mẫu Zalo không còn liệt kê thì đánh DELETE), giữ mốc đã gắn, có ghi vết
	// @screen   ADR 0098 #8, ADR 0099 #1 — Cấu hình → tab Kênh Zalo, "Nạp mẫu từ OA"
	// @reply    200 zaloOASyncResultOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error not_found
	// @reply    409 httpx.Error zalo_oa_needs_reauthorization zalo_oa_changed
	// @reply    422 httpx.Error zalo_token_rejected
	// @reply    502 httpx.Error zalo_unavailable
	// @reply    503 httpx.Error zalo_oa_not_configured encryption_not_configured
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/zalo-oas/{id}/templates/sync",
		authz.RequirePermission(d.Checker, "admin.zalo")(
			idem.KhongCan("nạp mẫu ghi lại cột của Zalo theo câu trả lời của Zalo và giữ mốc của xã; lần thứ hai là một lần đọc nữa của cùng OA")(
				http.HandlerFunc(h.PostSync))))

	// idem.KhongCan: the body names the end state — the same milestone again is a no-op with no entry,
	// and null on a detached template writes nothing.
	//
	//	400 invalid_request           no `milestone` key, or a milestone outside ADR 0098 #8
	//	404 not_found                 no such template of a live OA of this commune
	//	409 milestone_taken           another template of the commune (any OA) already announces it
	//	422 template_not_attachable   not ENABLE at Zalo, or a required parameter is missing (named)
	//
	// @summary  Gắn hoặc gỡ mốc cho một mẫu ZBS — chỉ mẫu ENABLE có đủ ba tham số ma_tra_cuu, moc_nhan, viec_tiep_theo mới gắn được (thiếu thì nói thiếu gì); một mốc một mẫu trong cả xã dù xã có nhiều OA, tin báo dân theo mốc đi từ OA của mẫu đã gắn; milestone null là gỡ; có ghi vết
	// @screen   ADR 0098 #8 #9, ADR 0099 #1 — Cấu hình → tab Kênh Zalo, mục "Mẫu tin ZBS"
	// @request  zaloOATemplatePatchIn
	// @reply    200 zaloOATemplateOut
	// @reply    400 httpx.Error invalid_request
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error not_found
	// @reply    409 httpx.Error milestone_taken zalo_oa_changed
	// @reply    422 httpx.Error template_not_attachable
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/zalo-oa-templates/{id}",
		authz.RequirePermission(d.Checker, "admin.zalo")(
			idem.KhongCan("thân nêu trạng thái đích: cùng mốc lần nữa thì không đổi gì và không có vết, gỡ một mẫu chưa gắn cũng không ghi gì")(
				http.HandlerFunc(h.PatchTemplate))))
}

// actor is the signed-in member of staff, by BUSINESS CODE (rule 6, invariant 8) — never a fallback.
func (h *zaloCommuneOAHandler) actor(w http.ResponseWriter, r *http.Request) (audit.Actor, bool) {
	a, ok := nguoiThucHien(r)
	if !ok {
		h.d.Log.Error("tuyến OA Zalo của xã chạy mà không có chủ thể mang mã cán bộ",
			"xa", string(tenant.MustFrom(r.Context())), "duong", r.URL.Path)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
	return a, ok
}

// staffNames resolves business codes to names. Display only: identity failing leaves every name "" and
// the screen shows the code (never a refusal of the read). Names are personal data — never logged.
func (h *zaloCommuneOAHandler) staffNames(ctx context.Context, codes []string) map[string]string {
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
			h.d.Log.Warn("OA Zalo của xã: không tra được tên cán bộ — trả mã, tên để trống",
				"xa", string(tenant.MustFrom(ctx)), "err", err)
			return map[string]string{}
		}
		for k, v := range got {
			out[k] = v.HoTen
		}
	}
	return out
}

func communeOAToOut(it app.CommuneZaloOAItem, byName string) communeZaloOAOut {
	o := it.OA
	out := communeZaloOAOut{ID: o.ID, OAID: o.OAID, OAName: o.OAName, AuthorizedAt: o.AuthorizedAt, AuthorizedBy: o.AuthorizedBy,
		AuthorizedByName: byName, AccessExpiresAt: o.AccessExpiresAt, TokenRefreshedAt: o.TokenRefreshedAt,
		TemplateCount: it.TemplateCount, AttachedCount: it.AttachedCount}
	if o.NeedsReauthorization() {
		out.NeedsReauthorization = &zaloOAReauthorizationOut{Since: o.NeedsReauthorizationSince, Reason: o.NeedsReauthorizationReason}
	}
	if !o.LastCheckAt.IsZero() {
		out.LastCheck = &zaloOALastCheckOut{At: o.LastCheckAt, Result: o.LastCheckResult}
	}
	return out
}

func templateToOut(t domain.ZaloOATemplate, names map[string]string) zaloOATemplateOut {
	out := zaloOATemplateOut{ID: t.ID, OARowID: t.OARowID, TemplateID: t.TemplateID, TemplateName: t.TemplateName,
		ZaloStatus: t.ZaloStatus, ZaloQuality: t.ZaloQuality, ZaloTag: t.ZaloTag, Params: make([]zaloOATemplateParamOut, 0, len(t.Params)),
		ParamsMatch: t.ParamsMatch, MissingParams: domain.MissingTemplateParams(t.Params)}
	if out.MissingParams == nil {
		out.MissingParams = []string{}
	}
	for _, p := range t.Params {
		out.Params = append(out.Params, zaloOATemplateParamOut{Name: p.Name, Required: p.Required, Type: p.Type, MaxLength: p.MaxLength})
	}
	if t.Attached() {
		m, at, by, name := t.Milestone, t.MilestoneSetAt, t.MilestoneSetBy, names[t.MilestoneSetBy]
		out.Milestone, out.MilestoneSetAt, out.MilestoneSetBy, out.MilestoneSetByName = &m, &at, &by, &name
	}
	return out
}

// GetList — GET /api/v1/zalo-oas
func (h *zaloCommuneOAHandler) GetList(w http.ResponseWriter, r *http.Request) {
	v, err := h.d.OAs.List(r.Context())
	if err != nil {
		h.fail(w, r, "đọc", err)
		return
	}
	codes := make([]string, 0, len(v.Items))
	for _, it := range v.Items {
		codes = append(codes, it.OA.AuthorizedBy)
	}
	names := h.staffNames(r.Context(), codes)
	out := communeZaloOAListOut{Configured: v.Configured, Items: make([]communeZaloOAOut, 0, len(v.Items))}
	for _, it := range v.Items {
		out.Items = append(out.Items, communeOAToOut(it, names[it.OA.AuthorizedBy]))
	}
	vietJSON(w, http.StatusOK, out)
}

// DeleteOA — DELETE /api/v1/zalo-oas/{id}
func (h *zaloCommuneOAHandler) DeleteOA(w http.ResponseWriter, r *http.Request) {
	var in communeZaloOADisconnectIn
	if !docThan(w, r, &in) {
		return
	}
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	res, err := h.d.OAs.Disconnect(r.Context(), r.PathValue("id"), in.Reason, actor)
	if err != nil {
		h.fail(w, r, "ngắt OA", err)
		return
	}
	vietJSON(w, http.StatusOK, communeZaloOADisconnectedOut{Disconnected: res.Retired,
		DetachedTemplateCount: res.DetachedTemplateCount, RevokeNotice: res.RevokeNotice})
}

// PostAuthorizationLink — POST /api/v1/zalo-oas/authorization-links
func (h *zaloCommuneOAHandler) PostAuthorizationLink(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	link, err := h.d.OAs.StartAuthorization(r.Context(), actor)
	if err != nil {
		h.fail(w, r, "tạo link uỷ quyền", err)
		return
	}
	// The URL carries a one-time state: never cached by anything between here and the browser.
	w.Header().Set("Cache-Control", "no-store")
	vietJSON(w, http.StatusCreated, zaloOAAuthorizationLinkOut{URL: link.URL, ExpiresAt: link.ExpiresAt})
}

// PostCheck — POST /api/v1/zalo-oas/{id}/check
func (h *zaloCommuneOAHandler) PostCheck(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	res, err := h.d.OAs.Check(r.Context(), r.PathValue("id"), actor)
	if err != nil {
		h.fail(w, r, "kiểm tra", err)
		return
	}
	vietJSON(w, http.StatusOK, zaloOACheckResultOut{Result: res.Outcome, CheckedAt: res.CheckedAt, OAName: res.OAName})
}

// GetTemplates — GET /api/v1/zalo-oas/{id}/templates
func (h *zaloCommuneOAHandler) GetTemplates(w http.ResponseWriter, r *http.Request) {
	list, err := h.d.OAs.ListTemplates(r.Context(), r.PathValue("id"))
	if err != nil {
		h.fail(w, r, "đọc mẫu", err)
		return
	}
	codes := make([]string, 0, len(list))
	for _, t := range list {
		if t.Attached() {
			codes = append(codes, t.MilestoneSetBy)
		}
	}
	names := h.staffNames(r.Context(), codes)
	out := zaloOATemplateListOut{Items: make([]zaloOATemplateOut, 0, len(list))}
	for _, t := range list {
		out.Items = append(out.Items, templateToOut(t, names))
		// The last load is the latest synced_at among the rows: every sync stamps every row it records.
		if !t.SyncedAt.IsZero() && (out.SyncedAt == nil || t.SyncedAt.After(*out.SyncedAt)) {
			at := t.SyncedAt
			out.SyncedAt = &at
		}
	}
	vietJSON(w, http.StatusOK, out)
}

// PostSync — POST /api/v1/zalo-oas/{id}/templates/sync
func (h *zaloCommuneOAHandler) PostSync(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	res, err := h.d.OAs.SyncTemplates(r.Context(), r.PathValue("id"), actor)
	if err != nil {
		h.fail(w, r, "nạp mẫu", err)
		return
	}
	vietJSON(w, http.StatusOK, zaloOASyncResultOut{Loaded: res.Loaded, Enabled: res.Enabled, Matching: res.Matching,
		Changed: len(res.ChangedTemplateIDs), SyncedAt: res.SyncedAt})
}

// PatchTemplate — PATCH /api/v1/zalo-oa-templates/{id}
func (h *zaloCommuneOAHandler) PatchTemplate(w http.ResponseWriter, r *http.Request) {
	var in zaloOATemplatePatchIn
	if !docThan(w, r, &in) {
		return
	}
	if !in.present {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Thiếu trường milestone (gửi null để gỡ mốc).", "")
		return
	}
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	t, err := h.d.OAs.SetMilestone(r.Context(), r.PathValue("id"), in.Milestone, actor)
	if err != nil {
		h.fail(w, r, "gắn mốc", err)
		return
	}
	var names map[string]string
	if t.Attached() {
		names = h.staffNames(r.Context(), []string{t.MilestoneSetBy})
	}
	vietJSON(w, http.StatusOK, templateToOut(t, names))
}

// fail maps a use-case error to its reply. The wrapped error never reaches the client (rule 3), and
// never names another commune.
func (h *zaloCommuneOAHandler) fail(w http.ResponseWriter, r *http.Request, op string, err error) {
	xa := string(tenant.MustFrom(r.Context()))
	var call *app.ZaloOACallError
	var missing *domain.TemplateParamsMissingError
	switch {
	case errors.Is(err, app.ErrZaloOANotConfigured):
		httpx.WriteError(w, http.StatusServiceUnavailable, "zalo_oa_not_configured",
			"Nền tảng chưa khai ứng dụng Zalo dùng chung nên chưa nối được OA. Hãy liên hệ ViHAT.", "")
	case errors.Is(err, crypto.ErrNotConfigured):
		httpx.WriteError(w, http.StatusServiceUnavailable, "encryption_not_configured",
			"Nền tảng chưa cấu hình khoá mã hoá bí mật (SECRET_ENCRYPTION_KEYS), nên chưa lưu hay dùng được uỷ quyền OA "+
				"của xã. Chưa có gì được ghi. Hãy báo đơn vị vận hành hệ thống.", "")
	case errors.Is(err, app.ErrCommuneZaloOAMissing):
		// One sentence for an unknown id, a retired OA and another commune's OA (ADR 0099 #1; rule 1).
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy OA này trong các OA đang nối của xã.", "")
	case errors.Is(err, app.ErrZaloOANeedsReauthorization):
		httpx.WriteError(w, http.StatusConflict, "zalo_oa_needs_reauthorization",
			"Zalo không còn nhận uỷ quyền OA của xã. Hãy bấm “Uỷ quyền lại”.", "")
	case errors.Is(err, app.ErrCommuneZaloOAChanged), errors.Is(err, commsstore.ErrZaloRowChanged):
		httpx.WriteError(w, http.StatusConflict, "zalo_oa_changed",
			"OA của xã vừa được thay đổi ở nơi khác. Vui lòng tải lại trang rồi thử lại.", "")
	case errors.Is(err, commsstore.ErrMilestoneTaken):
		httpx.WriteError(w, http.StatusConflict, "milestone_taken",
			"Mốc này đã được gắn cho một mẫu khác. Hãy gỡ mốc ở mẫu kia trước.", "")
	case errors.Is(err, app.ErrZaloOATemplateMissing):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy mẫu này trong các OA đang nối của xã.", "")
	case errors.As(err, &missing):
		// The parameter NAMES of a template — not personal data — named so the commune knows what to fix.
		httpx.WriteError(w, http.StatusUnprocessableEntity, "template_not_attachable",
			"Mẫu thiếu tham số bắt buộc: "+strings.Join(missing.Missing, ", ")+". "+
				"Mẫu đăng ký với Zalo phải có đúng ba tham số: "+strings.Join(domain.RequiredTemplateParams, ", ")+".", "")
	case errors.Is(err, domain.ErrZaloOATemplateNotEnabled):
		httpx.WriteError(w, http.StatusUnprocessableEntity, "template_not_attachable",
			"Chỉ mẫu đang ở trạng thái ENABLE trên Zalo mới gắn được mốc.", "")
	case errors.As(err, &call):
		// The class only — never Zalo's words (ADR 0098 #13).
		h.d.Log.Warn("OA Zalo của xã: Zalo không nhận khi "+op, "xa", xa, "buoc", call.Step, "ket_qua", call.Class)
		if call.Class == domain.ZaloCallTokenRejected {
			httpx.WriteError(w, http.StatusUnprocessableEntity, "zalo_token_rejected",
				"Zalo không nhận uỷ quyền OA của xã. Hãy bấm “Uỷ quyền lại”. Chưa có gì được ghi.", "")
			return
		}
		httpx.WriteError(w, http.StatusBadGateway, "zalo_unavailable",
			"Chưa hỏi được Zalo. Vui lòng thử lại sau ít phút. Chưa có gì được ghi.", "")
	default:
		if msg, ok := refusalMessage(communeOARefusals, err); ok {
			h.d.Log.Info("OA Zalo của xã: từ chối "+op, "xa", xa, "err", err)
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", msg, "")
			return
		}
		h.d.Log.Error("OA Zalo của xã: "+op+" lỗi hệ thống", "xa", xa, "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}

// communeOARefusals — the 400s, one fixed sentence per sentinel.
var communeOARefusals = []refusal{
	{domain.ErrCommuneZaloOARetireReason, "Cần nhập lý do ngắt OA (tối đa 200 ký tự)."},
	{domain.ErrZaloOAMilestoneUnknown, "Mốc không hợp lệ."},
}
