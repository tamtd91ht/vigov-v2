package http

// A COMMUNE'S SMS ACCOUNTS (ADR 0099 #2, #3; migration 0032) — web-admin Cấu hình › Kênh SMS:
//
//	GET    /api/v1/sms-accounts                       admin.sms  the commune's live eSMS accounts
//	POST   /api/v1/sms-accounts                       admin.sms  add one — eSMS checks the keys first
//	PATCH  /api/v1/sms-accounts/{id}                  admin.sms  edit label / keys / Brandname / default
//	DELETE /api/v1/sms-accounts/{id}                  admin.sms  "Ngừng dùng" — soft delete, with a reason
//	POST   /api/v1/sms-accounts/{id}/check            admin.sms  "Kiểm tra kết nối" — result recorded
//	GET    /api/v1/sms-accounts/{id}/templates        admin.sms  the CSKH templates, one per text
//	POST   /api/v1/sms-accounts/{id}/templates/sync   admin.sms  "Nạp mẫu" from eSMS
//
// `admin.sms` — "Cấu hình kênh SMS" — seeded by service-identity migration 0035 (ADR 0099 #3), granted to
// every role that held `admin.zalo`.
//
// NOTHING SECRET LEAVES HERE. `api_key` / `secret_key` are WRITE-ONLY: no type below has a field for them
// on the way out — not even a hint of the last characters; `set_at` / `set_by` say WHEN and BY
// WHOM they were set, the commune bot's shape. No reply names another commune: an id of another commune
// is the same 404 as an id that does not exist.

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

// SmsAccountAdmin is the use case. *app.SmsAccounts satisfies it. Every write opens its own transaction
// and writes its audit entry inside it (rule 6, invariant 3). The commune is the context's (rule 1).
type SmsAccountAdmin interface {
	ListAccounts(ctx context.Context) ([]domain.SmsAccount, error)
	CreateAccount(ctx context.Context, in app.CreateSmsAccountInput, actor audit.Actor) (domain.SmsAccount, error)
	UpdateAccount(ctx context.Context, id string, in app.UpdateSmsAccountInput, actor audit.Actor) (domain.SmsAccount, error)
	RetireAccount(ctx context.Context, id, reason string, actor audit.Actor) error
	Check(ctx context.Context, id string, actor audit.Actor) (app.SmsCheckOutput, error)
	ListTemplates(ctx context.Context, accountID string) ([]app.SmsTemplateGroup, error)
	SyncTemplates(ctx context.Context, accountID string, actor audit.Actor) (app.SmsTemplateSyncOutput, error)
}

// SmsAccountDeps is everything the seven routes touch.
type SmsAccountDeps struct {
	Checker  authz.Checker
	Accounts SmsAccountAdmin
	Log      *slog.Logger
}

type smsAccountHandler struct{ d SmsAccountDeps }

// smsAccountOut is one account as the screen reads it. No key, no part of a key.
type smsAccountOut struct {
	ID        string `json:"id"`
	Provider  string `json:"provider"`
	Label     string `json:"label"`
	Brandname string `json:"brandname"`
	IsDefault bool   `json:"is_default"`
	// SetAt / SetBy: when and by whom (staff code) the keys and Brandname were last set.
	SetAt time.Time `json:"set_at"`
	SetBy string    `json:"set_by"`
	// LastCheck: null until the first check (a save records one: the keys were just checked).
	LastCheck *smsCheckOut `json:"last_check"`
	CreatedAt time.Time    `json:"created_at"`
}

// smsCheckOut — `result` is 0032's class: thanh-cong · sai-khoa · brandname-chua-kich-hoat ·
// khong-kha-dung · bi-tu-choi · phan-hoi-sai-dang. `balance` (eSMS's decimal as a string, "36693.00") only with
// thanh-cong, else null.
type smsCheckOut struct {
	At      time.Time `json:"at"`
	Result  string    `json:"result"`
	Balance *string   `json:"balance"`
}

type smsAccountListOut struct {
	Items []smsAccountOut `json:"items"`
}

// smsAccountCreateIn is the body of POST. `api_key` / `secret_key` ARE WRITE-ONLY.
type smsAccountCreateIn struct {
	Label     string `json:"label"`
	APIKey    string `json:"api_key"`
	SecretKey string `json:"secret_key"`
	Brandname string `json:"brandname"`
	IsDefault bool   `json:"is_default"`
}

// smsAccountUpdateIn is the body of PATCH. An absent field, or an empty key, keeps the stored value.
type smsAccountUpdateIn struct {
	Label     *string `json:"label,omitempty"`
	APIKey    string  `json:"api_key,omitempty"`
	SecretKey string  `json:"secret_key,omitempty"`
	Brandname *string `json:"brandname,omitempty"`
	IsDefault *bool   `json:"is_default,omitempty"`
}

// smsAccountRetireIn is the body of DELETE — rule 7: a soft delete carries its reason.
type smsAccountRetireIn struct {
	Reason string `json:"reason"`
}

// smsAccountRetiredOut is the reply of DELETE.
type smsAccountRetiredOut struct {
	Retired bool `json:"retired"`
}

// smsCheckResultOut is the reply of POST check — a failing check is still a 200 carrying its class.
type smsCheckResultOut struct {
	Result    string    `json:"result"`
	CheckedAt time.Time `json:"checked_at"`
	Balance   *string   `json:"balance"`
}

// smsTemplateParamOut is one slot of a template, IN ORDER: a value is typed per slot (names may repeat).
type smsTemplateParamOut struct {
	Name      string `json:"name"`
	MaxLength int    `json:"max_length"`
}

// smsTemplateNetworkOut is one network's registration: eSMS NetworkID (1 Viettel · 2 Mobi · 3 Vinaphone ·
// 4 Vietnamobile · 5 Gtel · 6 Itel · 7 Reddi) and its TempId.
type smsTemplateNetworkOut struct {
	NetworkID  int    `json:"network_id"`
	TemplateID string `json:"template_id"`
}

// smsTemplateOut is one template TEXT with the networks it is registered on.
type smsTemplateOut struct {
	Content  string                  `json:"content"`
	Params   []smsTemplateParamOut   `json:"params"`
	Networks []smsTemplateNetworkOut `json:"networks"`
	SyncedAt time.Time               `json:"synced_at"`
}

type smsTemplateListOut struct {
	Items []smsTemplateOut `json:"items"`
}

// smsTemplateSyncOut is the reply of POST templates/sync. `skipped`: templates eSMS listed that cannot be
// stored (counted, never quoted).
type smsTemplateSyncOut struct {
	Added    int              `json:"added"`
	Updated  int              `json:"updated"`
	Vanished int              `json:"vanished"`
	Skipped  int              `json:"skipped"`
	Items    []smsTemplateOut `json:"items"`
}

// balanceOut is eSMS's balance as DECIMAL TEXT ("36693.00") — exact, never a float — or null.
func balanceOut(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func smsAccountToOut(a domain.SmsAccount) smsAccountOut {
	out := smsAccountOut{ID: a.ID, Provider: a.Provider, Label: a.Label, Brandname: a.Brandname, IsDefault: a.IsDefault,
		SetAt: a.SetAt, SetBy: a.SetBy, CreatedAt: a.CreatedAt}
	if !a.LastCheckAt.IsZero() {
		out.LastCheck = &smsCheckOut{At: a.LastCheckAt, Result: a.LastCheckResult, Balance: balanceOut(a.LastBalance)}
	}
	return out
}

func smsTemplatesToOut(groups []app.SmsTemplateGroup) []smsTemplateOut {
	out := make([]smsTemplateOut, 0, len(groups))
	for _, g := range groups {
		t := smsTemplateOut{Content: g.Content, SyncedAt: g.SyncedAt, Params: make([]smsTemplateParamOut, 0, len(g.Params)),
			Networks: make([]smsTemplateNetworkOut, 0, len(g.Networks))}
		for _, p := range g.Params {
			t.Params = append(t.Params, smsTemplateParamOut{Name: p.Name, MaxLength: p.MaxLen})
		}
		for _, n := range g.Networks {
			t.Networks = append(t.Networks, smsTemplateNetworkOut{NetworkID: n.NetworkID, TemplateID: n.ProviderTemplateID})
		}
		out = append(out, t)
	}
	return out
}

// RegisterSmsAccounts mounts the seven routes on the staff mux. Refuses incomplete wiring at construction.
func RegisterSmsAccounts(mux *http.ServeMux, d SmsAccountDeps) {
	switch {
	case d.Checker == nil:
		panic("comms/http: thiếu authz.Checker — các tuyến tài khoản SMS sẽ không kiểm được quyền")
	case d.Accounts == nil:
		panic("comms/http: thiếu use case tài khoản SMS — /api/v1/sms-accounts sẽ panic khi có người gọi")
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	h := &smsAccountHandler{d: d}

	// NO idem.* DECLARATION: a GET. No audit entry: configuration read in its own commune, no secret in it.
	//
	// @summary  Danh sách tài khoản eSMS đang dùng của xã — tên gợi nhớ, Brandname, mặc định, khoá đặt lúc nào bởi ai, lần kiểm gần nhất và số dư; không bao giờ trả ApiKey hay SecretKey
	// @screen   ADR 0099 #2 — Cấu hình → tab Kênh SMS
	// @reply    200 smsAccountListOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/sms-accounts",
		authz.RequirePermission(d.Checker, "admin.sms")(
			http.HandlerFunc(h.List)))

	// idem.Required(DongKhiHong): a repeated POST would ask eSMS again and add a second account (the label
	// key would refuse a same-named one, but only after the keys travelled twice). Closed when the
	// idempotency store is down — configuration can wait; a duplicate credential row is worse.
	//
	//	400 sms_account_invalid         a field is malformed, or a key is missing
	//	409 sms_label_taken             another live account of the commune has that name
	//	422 sms_key_rejected            eSMS refused the ApiKey / SecretKey; nothing saved
	//	422 sms_brandname_inactive      the Brandname is not active on that account; nothing saved
	//	422 sms_rejected                eSMS refused for another reason (account locked, IP not allowed…)
	//	502 sms_unavailable             eSMS could not be asked, or answered unreadably; nothing saved
	//	503 encryption_not_configured   the platform cannot seal the keys; nothing saved
	//
	// @summary  Thêm tài khoản eSMS cho xã — tên gợi nhớ, ApiKey và SecretKey (chỉ ghi, niêm phong bằng khoá riêng của xã), Brandname, có làm mặc định không; hỏi eSMS (số dư, mẫu của Brandname) TRƯỚC khi lưu, khoá eSMS không nhận thì không lưu; có ghi vết
	// @screen   ADR 0099 #2 — Cấu hình → tab Kênh SMS "Thêm tài khoản"
	// @request  smsAccountCreateIn
	// @reply    201 smsAccountOut
	// @reply    400 httpx.Error sms_account_invalid
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    409 httpx.Error sms_label_taken sms_account_changed
	// @reply    422 httpx.Error sms_key_rejected sms_brandname_inactive sms_rejected
	// @reply    502 httpx.Error sms_unavailable
	// @reply    503 httpx.Error encryption_not_configured
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/sms-accounts",
		authz.RequirePermission(d.Checker, "admin.sms")(
			idem.Required(idem.DongKhiHong)(
				http.HandlerFunc(h.Create))))

	// idem.KhongCan: the same body twice leaves the same account — the second finds nothing changed, or
	// writes the same keys again with one more entry.
	//
	// @summary  Sửa tài khoản eSMS của xã — tên, ApiKey / SecretKey (để trống = giữ), Brandname, mặc định; đổi khoá hoặc Brandname thì hỏi eSMS trước, eSMS không nhận thì giữ nguyên; đổi mặc định bỏ mặc định cũ trong cùng giao dịch; có ghi vết
	// @screen   ADR 0099 #2 — Cấu hình → tab Kênh SMS "Sửa"
	// @request  smsAccountUpdateIn
	// @reply    200 smsAccountOut
	// @reply    400 httpx.Error sms_account_invalid
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error not_found
	// @reply    409 httpx.Error sms_label_taken sms_account_changed
	// @reply    422 httpx.Error sms_key_rejected sms_brandname_inactive sms_rejected
	// @reply    502 httpx.Error sms_unavailable
	// @reply    503 httpx.Error encryption_not_configured
	// @reply    500 httpx.Error
	mux.Handle("PATCH /api/v1/sms-accounts/{id}",
		authz.RequirePermission(d.Checker, "admin.sms")(
			idem.KhongCan("sửa là ghi đè bằng đúng giá trị trong thân; lần thứ hai không đổi gì nữa, hoặc ghi lại đúng khoá ấy kèm một vết")(
				http.HandlerFunc(h.Update))))

	// DELETE with a body: rule 7 — the retirement carries its reason. idem.KhongCan: a second DELETE finds
	// no live account and answers 404, writing nothing.
	//
	// @summary  Ngừng dùng tài khoản eSMS của xã — xoá mềm, giữ làm lịch sử (các đợt đã gửi vẫn trỏ tới), cần lý do; nếu là mặc định thì xã không còn tài khoản mặc định, hệ thống không tự chọn tài khoản khác; có ghi vết
	// @screen   ADR 0099 #2 — Cấu hình → tab Kênh SMS "Ngừng dùng"
	// @request  smsAccountRetireIn
	// @reply    200 smsAccountRetiredOut
	// @reply    400 httpx.Error sms_account_invalid
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error not_found
	// @reply    500 httpx.Error
	mux.Handle("DELETE /api/v1/sms-accounts/{id}",
		authz.RequirePermission(d.Checker, "admin.sms")(
			idem.KhongCan("lần thứ hai không còn tài khoản đang dùng nào mang mã ấy nên trả 404, không ghi gì")(
				http.HandlerFunc(h.Retire))))

	// idem.KhongCan: a check asks eSMS and records the latest answer; a second check is another check.
	//
	// @summary  Kiểm tra kết nối tài khoản eSMS — hỏi số dư và mẫu của Brandname bằng khoá đã lưu, ghi kết quả (lớp, không bao giờ chữ của eSMS), số dư khi thành công, và vết; kiểm tra không đạt vẫn trả 200 kèm lớp kết quả
	// @screen   ADR 0099 #2 — Cấu hình → tab Kênh SMS "Kiểm tra kết nối"
	// @reply    200 smsCheckResultOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error not_found
	// @reply    409 httpx.Error sms_account_changed
	// @reply    503 httpx.Error encryption_not_configured
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/sms-accounts/{id}/check",
		authz.RequirePermission(d.Checker, "admin.sms")(
			idem.KhongCan("kiểm tra chỉ hỏi eSMS rồi ghi kết quả mới nhất; lần thứ hai là một lần kiểm khác, không đổi gì khác")(
				http.HandlerFunc(h.Check))))

	// NO idem.* DECLARATION: a GET.
	//
	// @summary  Mẫu tin CSKH của một tài khoản eSMS — mỗi nội dung một mục, kèm các nhà mạng đã đăng ký (NetworkID, TempId) và các ô tham số theo thứ tự ({P1,20} → tên P1, tối đa 20 ký tự); mẫu eSMS không còn liệt kê thì không hiện
	// @screen   ADR 0099 #2 — Cấu hình → tab Kênh SMS "Mẫu tin"
	// @reply    200 smsTemplateListOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error not_found
	// @reply    500 httpx.Error
	mux.Handle("GET /api/v1/sms-accounts/{id}/templates",
		authz.RequirePermission(d.Checker, "admin.sms")(
			http.HandlerFunc(h.ListTemplates)))

	// idem.KhongCan: a sync rewrites the templates from eSMS's current list; a second sync of the same
	// list changes nothing but synced_at.
	//
	// @summary  Nạp mẫu tin CSKH từ eSMS cho một tài khoản — ghi mỗi (TempId, nhà mạng) một dòng, mẫu eSMS không còn liệt kê thì đánh dấu đã mất (giữ dòng), mẫu không lưu được thì đếm bỏ qua; eSMS không trả lời được thì không ghi gì; một vết cho cả lần nạp
	// @screen   ADR 0099 #2 — Cấu hình → tab Kênh SMS "Nạp mẫu"
	// @reply    200 smsTemplateSyncOut
	// @reply    401 httpx.Error
	// @reply    403 httpx.Error
	// @reply    404 httpx.Error not_found
	// @reply    409 httpx.Error sms_account_changed
	// @reply    422 httpx.Error sms_key_rejected sms_brandname_inactive sms_rejected
	// @reply    502 httpx.Error sms_unavailable
	// @reply    503 httpx.Error encryption_not_configured
	// @reply    500 httpx.Error
	mux.Handle("POST /api/v1/sms-accounts/{id}/templates/sync",
		authz.RequirePermission(d.Checker, "admin.sms")(
			idem.KhongCan("nạp lại ghi đè mẫu theo đúng danh sách eSMS đang có; lần thứ hai với cùng danh sách chỉ đổi thời điểm nạp")(
				http.HandlerFunc(h.SyncTemplates))))
}

// actor is the signed-in member of staff, by BUSINESS CODE (rule 6, invariant 8) — never a fallback.
func (h *smsAccountHandler) actor(w http.ResponseWriter, r *http.Request) (audit.Actor, bool) {
	a, ok := nguoiThucHien(r)
	if !ok {
		h.d.Log.Error("tuyến tài khoản SMS chạy mà không có chủ thể mang mã cán bộ",
			"xa", string(tenant.MustFrom(r.Context())), "duong", r.URL.Path)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
	return a, ok
}

// List — GET /api/v1/sms-accounts
func (h *smsAccountHandler) List(w http.ResponseWriter, r *http.Request) {
	accounts, err := h.d.Accounts.ListAccounts(r.Context())
	if err != nil {
		h.fail(w, r, "đọc", err)
		return
	}
	out := smsAccountListOut{Items: make([]smsAccountOut, 0, len(accounts))}
	for _, a := range accounts {
		out.Items = append(out.Items, smsAccountToOut(a))
	}
	vietJSON(w, http.StatusOK, out)
}

// Create — POST /api/v1/sms-accounts
func (h *smsAccountHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in smsAccountCreateIn
	if !docThan(w, r, &in) {
		return
	}
	// The one conversion: from here on the keys are secret.Secret, which refuse to render.
	api, sec := secret.Secret(in.APIKey), secret.Secret(in.SecretKey)
	in.APIKey, in.SecretKey = "", ""
	defer clear(api)
	defer clear(sec)
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	a, err := h.d.Accounts.CreateAccount(r.Context(), app.CreateSmsAccountInput{Label: in.Label, APIKey: api, SecretKey: sec,
		Brandname: in.Brandname, IsDefault: in.IsDefault}, actor)
	if err != nil {
		h.fail(w, r, "thêm", err)
		return
	}
	vietJSON(w, http.StatusCreated, smsAccountToOut(a))
}

// Update — PATCH /api/v1/sms-accounts/{id}
func (h *smsAccountHandler) Update(w http.ResponseWriter, r *http.Request) {
	var in smsAccountUpdateIn
	if !docThan(w, r, &in) {
		return
	}
	api, sec := secret.Secret(in.APIKey), secret.Secret(in.SecretKey)
	in.APIKey, in.SecretKey = "", ""
	defer clear(api)
	defer clear(sec)
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	a, err := h.d.Accounts.UpdateAccount(r.Context(), r.PathValue("id"), app.UpdateSmsAccountInput{Label: in.Label,
		Brandname: in.Brandname, APIKey: api, SecretKey: sec, IsDefault: in.IsDefault}, actor)
	if err != nil {
		h.fail(w, r, "sửa", err)
		return
	}
	vietJSON(w, http.StatusOK, smsAccountToOut(a))
}

// Retire — DELETE /api/v1/sms-accounts/{id}
func (h *smsAccountHandler) Retire(w http.ResponseWriter, r *http.Request) {
	var in smsAccountRetireIn
	if !docThan(w, r, &in) {
		return
	}
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	if err := h.d.Accounts.RetireAccount(r.Context(), r.PathValue("id"), in.Reason, actor); err != nil {
		h.fail(w, r, "ngừng dùng", err)
		return
	}
	vietJSON(w, http.StatusOK, smsAccountRetiredOut{Retired: true})
}

// Check — POST /api/v1/sms-accounts/{id}/check
func (h *smsAccountHandler) Check(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	res, err := h.d.Accounts.Check(r.Context(), r.PathValue("id"), actor)
	if err != nil {
		h.fail(w, r, "kiểm tra", err)
		return
	}
	vietJSON(w, http.StatusOK, smsCheckResultOut{Result: res.Result, CheckedAt: res.CheckedAt, Balance: balanceOut(res.Balance)})
}

// ListTemplates — GET /api/v1/sms-accounts/{id}/templates
func (h *smsAccountHandler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	groups, err := h.d.Accounts.ListTemplates(r.Context(), r.PathValue("id"))
	if err != nil {
		h.fail(w, r, "đọc mẫu", err)
		return
	}
	vietJSON(w, http.StatusOK, smsTemplateListOut{Items: smsTemplatesToOut(groups)})
}

// SyncTemplates — POST /api/v1/sms-accounts/{id}/templates/sync
func (h *smsAccountHandler) SyncTemplates(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actor(w, r)
	if !ok {
		return
	}
	res, err := h.d.Accounts.SyncTemplates(r.Context(), r.PathValue("id"), actor)
	if err != nil {
		h.fail(w, r, "nạp mẫu", err)
		return
	}
	vietJSON(w, http.StatusOK, smsTemplateSyncOut{Added: res.Added, Updated: res.Updated, Vanished: res.Vanished,
		Skipped: res.Skipped, Items: smsTemplatesToOut(res.Templates)})
}

// smsAccountRefusals — the 400s, one fixed sentence per sentinel (the domain text never reaches the body).
var smsAccountRefusals = []refusal{
	{domain.ErrSmsAccountLabel, "Tên gợi nhớ là bắt buộc, tối đa 100 ký tự, không có ký tự điều khiển."},
	{domain.ErrSmsBrandname, "Brandname chỉ gồm chữ không dấu, số, dấu cách, chấm, gạch dưới, gạch ngang; bắt đầu bằng chữ hoặc số; tối đa 32 ký tự."},
	{domain.ErrSmsKeyShape, "ApiKey và SecretKey phải chép nguyên từ trang quản trị eSMS: không dấu cách, không ký tự có dấu."},
	{domain.ErrSmsKeysRequired, "Tài khoản mới cần nhập cả ApiKey và SecretKey."},
	{domain.ErrSmsRetireReason, "Cần nhập lý do ngừng dùng (tối đa 200 ký tự)."},
}

// fail maps a use-case error to its reply. The wrapped error never reaches the client (rule 3), and never
// names another commune.
func (h *smsAccountHandler) fail(w http.ResponseWriter, r *http.Request, op string, err error) {
	xa := string(tenant.MustFrom(r.Context()))
	var pe *app.SmsProviderError
	switch {
	case errors.Is(err, crypto.ErrNotConfigured):
		httpx.WriteError(w, http.StatusServiceUnavailable, "encryption_not_configured",
			"Nền tảng chưa cấu hình khoá mã hoá bí mật (SECRET_ENCRYPTION_KEYS), nên chưa lưu hay dùng được khoá eSMS "+
				"của xã. Chưa có gì được ghi. Hãy báo đơn vị vận hành hệ thống.", "")
	case errors.As(err, &pe):
		// The class only — never eSMS's words.
		h.d.Log.Warn("tài khoản SMS: eSMS không nhận khi "+op, "xa", xa, "ket_qua", pe.Class)
		switch pe.Class {
		case domain.SmsCheckKeyRejected:
			httpx.WriteError(w, http.StatusUnprocessableEntity, "sms_key_rejected",
				"eSMS không nhận ApiKey / SecretKey này. Hãy chép lại khoá trong trang quản trị eSMS. Chưa có gì được lưu.", "")
		case domain.SmsCheckBrandnameInactive:
			httpx.WriteError(w, http.StatusUnprocessableEntity, "sms_brandname_inactive",
				"Brandname này chưa được kích hoạt trên tài khoản eSMS (hoặc viết chưa đúng). Hãy liên hệ eSMS. Chưa có gì được lưu.", "")
		case domain.SmsCheckRejected:
			httpx.WriteError(w, http.StatusUnprocessableEntity, "sms_rejected",
				"eSMS từ chối yêu cầu (tài khoản có thể đang bị khoá, hoặc máy chủ chưa được eSMS cho phép). Hãy liên hệ eSMS. "+
					"Chưa có gì được lưu.", "")
		default:
			httpx.WriteError(w, http.StatusBadGateway, "sms_unavailable",
				"Chưa hỏi được eSMS. Vui lòng thử lại sau ít phút. Chưa có gì được lưu.", "")
		}
	case errors.Is(err, app.ErrSmsAccountNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Không tìm thấy tài khoản SMS này của xã.", "")
	case errors.Is(err, commsstore.ErrSmsLabelTaken):
		httpx.WriteError(w, http.StatusConflict, "sms_label_taken",
			"Xã đã có một tài khoản SMS đang dùng mang tên này. Hãy chọn tên khác.", "")
	case errors.Is(err, commsstore.ErrSmsAccountChanged):
		httpx.WriteError(w, http.StatusConflict, "sms_account_changed",
			"Tài khoản SMS vừa được thay đổi ở nơi khác. Vui lòng tải lại trang rồi thử lại.", "")
	default:
		if msg, ok := refusalMessage(smsAccountRefusals, err); ok {
			h.d.Log.Info("tài khoản SMS: từ chối "+op, "xa", xa, "err", err)
			httpx.WriteError(w, http.StatusBadRequest, "sms_account_invalid", msg, "")
			return
		}
		h.d.Log.Error("tài khoản SMS: "+op+" lỗi hệ thống", "xa", xa, "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}
