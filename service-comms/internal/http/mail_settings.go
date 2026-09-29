package http

// The routes behind `Cấu hình → Máy chủ thư` (docs/ui-ux/14-cau-hinh.md §10):
// GET · PUT /api/v1/mail-settings, POST /api/v1/mail-settings/test-messages.
//
// `mail-settings` IS A VENDOR-CHOSEN NOUN: kb/00-foundation/ubiquitous-language.md §Tên tài nguyên
// trên URL has no row for it. §11's `/api/cau-hinh/may-chu-thu` sketch cannot ship (ADR 0011 puts
// path segments in English). `test-messages` is a sub-collection a POST adds to — a noun, not the
// verb `/gui-thu-thu` the sketch uses.
//
// THE PASSWORD NEVER LEAVES THIS SERVICE. mailSettingsOut has no field that could hold it — only
// `password_set`. mailSettingsIn carries it inward, and the handler turns it into a secret.Secret at
// once, so nothing that formats or logs the request value can print it.
//
// THE COMMUNE IS NEVER HANDLED HERE. Every call passes the request context; the store behind the use
// case reaches the database only through db.For(ctx) (rule 1, invariants 4 and 5).

import (
	"errors"
	"net/http"

	"github.com/vihat/vigov/core/crypto"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/secret"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
	"github.com/vihat/vigov/service-comms/internal/mail"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// mailSettingsOut is the commune's mail server as the screen reads it.
type mailSettingsOut struct {
	// Configured false is §10's orange warning: "Chưa có máy chủ thư nào…". The other fields then
	// hold §10's defaults (587, starttls) and empty strings.
	Configured bool   `json:"configured"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	// Security is `starttls` (port 587) or `tls` (port 465). There is no plaintext value.
	Security    string `json:"security"`
	Username    string `json:"username"`
	FromAddress string `json:"from_address"`
	FromName    string `json:"from_name"`
	// IsEnabled is `Dùng máy chủ thư này cho xã`.
	IsEnabled bool `json:"is_enabled"`
	// PasswordSet is the ONLY thing any response says about the password (§12.6).
	PasswordSet bool `json:"password_set"`
	// EncryptionConfigured false: the platform has no SECRET_ENCRYPTION_KEYS, so saving and
	// test-sending answer 503 until the operator configures it.
	EncryptionConfigured bool `json:"encryption_configured"`
}

func mailSettingsToOut(v app.MailSettingsView) mailSettingsOut {
	return mailSettingsOut{
		Configured: v.Configured, Host: v.Host, Port: v.Port, Security: v.Security,
		Username: v.Username, FromAddress: v.FromAddress, FromName: v.FromName,
		IsEnabled: v.IsEnabled, PasswordSet: v.PasswordSet,
		EncryptionConfigured: v.EncryptionConfigured,
	}
}

// mailSettingsIn is the body of PUT — the whole configuration.
//
// `password` IS WRITE-ONLY. Omitted or "" means KEEP the stored password — except when host, port or
// username changed, which is refused with 400 `password_required_for_new_host` (the old password is
// never sent to a destination it was not typed for).
type mailSettingsIn struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Security    string `json:"security"`
	Username    string `json:"username"`
	FromAddress string `json:"from_address"`
	FromName    string `json:"from_name,omitempty"`
	IsEnabled   bool   `json:"is_enabled,omitempty"`
	Password    string `json:"password,omitempty"`
}

// mailTestIn is the body of POST test-messages: `Gửi thư thử tới` [email].
type mailTestIn struct {
	Recipient string `json:"recipient"`
}

// mailTestOut — the server ACCEPTED the message. Whether it reaches the inbox is the receiving
// server's business; this is the most the protocol can say.
type mailTestOut struct {
	Sent bool `json:"sent"`
}

// GetMailSettings — GET /api/v1/mail-settings. No audit entry: the configuration read inside the
// commune it belongs to, and it holds no password to read.
func (h *Handler) GetMailSettings(w http.ResponseWriter, r *http.Request) {
	// Scoped in the use case: MailSettingsStore.Get reads through db.For(ctx).Query.
	v, err := h.d.MailSettings.Get(r.Context())
	if err != nil {
		h.writeMailSettingsError(w, r, "đọc", err)
		return
	}
	writeJSON(w, http.StatusOK, mailSettingsToOut(v))
}

// PutMailSettings — PUT /api/v1/mail-settings
func (h *Handler) PutMailSettings(w http.ResponseWriter, r *http.Request) {
	var in mailSettingsIn
	if !decodeBody(w, r, &in) {
		return
	}
	// The one conversion: from here on the password is a secret.Secret, which refuses to render.
	password := secret.Secret(in.Password)
	in.Password = ""

	actor, ok := actorFrom(r)
	if !ok {
		h.missingPrincipal(w, r)
		return
	}
	// Scoped in the use case: app.MailSettingsAdmin opens db.For(ctx).Tx.
	v, err := h.d.WriteMailSettings.Save(r.Context(), app.SaveMailSettingsRequest{
		Input: domain.MailSettingsInput{
			Host: in.Host, Port: in.Port, Security: in.Security, Username: in.Username,
			FromAddress: in.FromAddress, FromName: in.FromName, IsEnabled: in.IsEnabled,
		},
		Password: password,
	}, actor)
	clear(password)
	if err != nil {
		h.writeMailSettingsError(w, r, "lưu", err)
		return
	}
	writeJSON(w, http.StatusOK, mailSettingsToOut(v))
}

// SendTestMail — POST /api/v1/mail-settings/test-messages
func (h *Handler) SendTestMail(w http.ResponseWriter, r *http.Request) {
	var in mailTestIn
	if !decodeBody(w, r, &in) {
		return
	}
	actor, ok := actorFrom(r)
	if !ok {
		h.missingPrincipal(w, r)
		return
	}
	// Scoped in the use case: MailSettingsStore.Account reads through db.For(ctx).Query.
	if err := h.d.WriteMailSettings.SendTestMessage(r.Context(), in.Recipient, actor); err != nil {
		h.writeMailSettingsError(w, r, "gửi thử", err)
		return
	}
	writeJSON(w, http.StatusOK, mailTestOut{Sent: true})
}

// mailSendFailure is one SMTP failure category as the administrator reads it. The sentence says
// what to check; it never quotes the server (internal/mail drops the reply text).
type mailSendFailure struct {
	err     error
	code    string
	message string
}

var mailSendFailures = []mailSendFailure{
	{mail.ErrTimeout, "mail_timeout",
		"Máy chủ thư không trả lời kịp. Hãy kiểm tra tên máy chủ và cổng, hoặc thử lại sau."},
	{mail.ErrCertificate, "mail_certificate_invalid",
		"Chứng chỉ của máy chủ thư không xác minh được với tên máy chủ đã khai. Hệ thống không gửi thư qua kết nối chưa xác minh — hãy kiểm tra tên máy chủ, hoặc báo đơn vị quản lý máy chủ thư."},
	{mail.ErrStartTLSMissing, "mail_starttls_missing",
		"Máy chủ thư không hỗ trợ START TLS ở cổng này. Hãy thử chế độ TLS ngay từ đầu (thường là cổng 465)."},
	{mail.ErrTLS, "mail_tls_failed",
		"Không thiết lập được kết nối mã hoá tới máy chủ thư. Hãy kiểm tra chế độ bảo mật có khớp với cổng không (587 dùng START TLS, 465 dùng TLS ngay từ đầu)."},
	{mail.ErrPlaintextRefused, "mail_plaintext_refused",
		"Hệ thống không gửi thư qua kết nối không mã hoá."},
	{mail.ErrAuthUnsupported, "mail_auth_unsupported",
		"Máy chủ thư không cho đăng nhập bằng tài khoản và mật khẩu ở cổng này."},
	{mail.ErrAuthRejected, "mail_auth_rejected",
		"Máy chủ thư từ chối tài khoản hoặc mật khẩu. Hãy nhập lại mật khẩu và lưu cấu hình."},
	{mail.ErrRecipientRejected, "mail_rejected",
		"Máy chủ thư từ chối địa chỉ gửi hoặc địa chỉ nhận. Hãy kiểm tra Địa chỉ gửi có thuộc tài khoản đã khai không."},
	{mail.ErrConnect, "mail_connect_failed",
		"Không kết nối được tới máy chủ thư. Hãy kiểm tra tên máy chủ và cổng."},
	{mail.ErrProtocol, "mail_protocol_error",
		"Máy chủ thư trả lời không đúng giao thức SMTP. Hãy kiểm tra tên máy chủ và cổng."},
}

// writeMailSettingsError maps a use-case failure onto a status. 503 when the platform cannot seal,
// 502 when the commune's server refused, 409 for a state of the data, 400 for the request, 500 for
// everything not listed — never a default 400.
func (h *Handler) writeMailSettingsError(w http.ResponseWriter, r *http.Request, op string, err error) {
	switch {
	case errors.Is(err, crypto.ErrNotConfigured):
		// Named and honest: the cause is the platform, not the commune, and nothing was written.
		httpx.WriteError(w, http.StatusServiceUnavailable, "encryption_not_configured",
			"Nền tảng chưa cấu hình khoá mã hoá bí mật (SECRET_ENCRYPTION_KEYS), nên chưa lưu hay dùng "+
				"được mật khẩu máy chủ thư. Chưa có gì được ghi. Hãy báo đơn vị vận hành hệ thống.", "")
		return
	case errors.Is(err, app.ErrMailPasswordRequiredForNewDestination):
		httpx.WriteError(w, http.StatusBadRequest, "password_required_for_new_host",
			"Đã đổi máy chủ, cổng hoặc tài khoản thì phải nhập lại mật khẩu. Mật khẩu cũ không được "+
				"gửi tới một nơi chưa từng dùng nó.", "")
		return
	case errors.Is(err, app.ErrMailPasswordRequired):
		httpx.WriteError(w, http.StatusBadRequest, "password_required",
			"Lần lưu đầu tiên phải nhập mật khẩu của tài khoản thư.", "")
		return
	case errors.Is(err, commsstore.ErrMailSettingsNotFound):
		httpx.WriteError(w, http.StatusConflict, "mail_settings_missing",
			"Xã chưa lưu cấu hình máy chủ thư. Hãy lưu cấu hình trước khi gửi thử.", "")
		return
	}
	if msg, ok := refusalMessage(mailSettingsRefusals, err); ok {
		h.logRefusal(r, "máy chủ thư: từ chối "+op, err)
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", msg, "")
		return
	}
	for _, f := range mailSendFailures {
		if errors.Is(err, f.err) {
			// Logged: the category and the reply code only — internal/mail never returns the reply
			// text, so neither the account nor a server banner reaches this line.
			h.d.Log.Warn("máy chủ thư: gửi thử không thành",
				"xa", string(tenant.MustFrom(r.Context())), "loai", f.code, "err", err)
			httpx.WriteError(w, http.StatusBadGateway, f.code, f.message, "")
			return
		}
	}
	// The wrapped error never reaches the client (rule 3, forbidden #3). It never holds the password
	// either: nothing in the chain formats one.
	h.d.Log.Error("máy chủ thư: "+op+" lỗi hệ thống", "xa", string(tenant.MustFrom(r.Context())), "err", err)
	httpx.WriteError(w, http.StatusInternalServerError, "internal", "Đã xảy ra lỗi. Vui lòng thử lại.", "")
}

// mailSettingsRefusals — the 400s of the three mail-settings routes, one fixed sentence per sentinel
// (see `refusal` in map_field_schema.go for why the domain's own sentence never reaches the body).
// Field names are the labels of docs/ui-ux/14-cau-hinh.md §10.
//
// NO BOUND IS WRITTEN AS A NUMBER HERE. The domain keeps them unexported (mailFromNameMax,
// allowedMailPorts), and a number retyped into a sentence is a second copy that drifts. The port
// sentence names the two ports §10 itself recommends and does not claim they are the only ones.
var mailSettingsRefusals = []refusal{
	{domain.ErrMailHostEmpty, "Chưa nhập máy chủ SMTP."},
	{domain.ErrMailHostShape, "Máy chủ SMTP phải là tên máy (ví dụ smtp.xa.gov.vn) hoặc địa chỉ IP, " +
		"không kèm cổng hay giao thức."},
	{domain.ErrMailPortNotAllowed, "Cổng này không dùng được cho máy chủ thư. Thường dùng 587 với START TLS, " +
		"hoặc 465 với TLS ngay từ đầu."},
	{domain.ErrMailSecurityUnknown, "Hãy chọn START TLS hoặc TLS ngay từ đầu. Hệ thống không gửi thư qua " +
		"kết nối không mã hoá."},
	{domain.ErrMailUsernameEmpty, "Chưa nhập tài khoản."},
	{domain.ErrMailUsernameShape, "Tài khoản quá dài hoặc chứa ký tự không hợp lệ."},
	{domain.ErrMailFromAddress, "Địa chỉ gửi phải là một địa chỉ thư điện tử hợp lệ, không kèm tên hiển thị."},
	{domain.ErrMailFromName, "Tên hiển thị của người gửi quá dài, hoặc chứa ký tự xuống dòng hay ký tự " +
		"không hợp lệ."},
	{domain.ErrMailPasswordShape, "Mật khẩu quá dài hoặc chứa ký tự không hợp lệ."},
	{domain.ErrMailRecipient, "Địa chỉ nhận thư thử phải là một địa chỉ thư điện tử hợp lệ, không kèm tên " +
		"hiển thị."},
}
